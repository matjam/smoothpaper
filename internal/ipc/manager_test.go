package ipc

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/log"
	"github.com/matjam/smoothpaper/internal/config"
	"github.com/matjam/smoothpaper/internal/types"
	"github.com/matjam/smoothpaper/internal/wallpapers"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// fakeRenderer records the setter calls, so a test can see what reload() applied.
type fakeRenderer struct {
	scaleMode types.ScalingMode
	easing    types.EasingMode
	framerate int
	calls     []string
}

func (f *fakeRenderer) SetScaleMode(m types.ScalingMode) {
	f.scaleMode = m
	f.calls = append(f.calls, "SetScaleMode")
}

func (f *fakeRenderer) SetEasingMode(m types.EasingMode) {
	f.easing = m
	f.calls = append(f.calls, "SetEasingMode")
}

func (f *fakeRenderer) SetFrameRateLimit(n int) {
	f.framerate = n
	f.calls = append(f.calls, "SetFrameRateLimit")
}

func (f *fakeRenderer) SetImage(image.Image) error                  { return nil }
func (f *fakeRenderer) Transition(image.Image, time.Duration) error { return nil }
func (f *fakeRenderer) Render() error                               { return nil }
func (f *fakeRenderer) Cleanup()                                    {}
func (f *fakeRenderer) GetSize() (int, int)                         { return 0, 0 }
func (f *fakeRenderer) IsDisplayRunning() bool                      { return true }
func (f *fakeRenderer) TryReconnect() error                         { return nil }

// reloadTest is a Manager with a fake renderer and a real config file. A test
// changes the file with write() and then calls m.reload(), like the daemon.
type reloadTest struct {
	t        *testing.T
	m        *Manager
	renderer *fakeRenderer
	path     string
}

func newReloadTest(t *testing.T, toml string) *reloadTest {
	t.Helper()

	viper.Reset()
	t.Cleanup(viper.Reset)

	rt := &reloadTest{t: t, path: filepath.Join(t.TempDir(), "smoothpaper.toml")}
	rt.write(toml)

	viper.SetConfigFile(rt.path)
	if err := viper.ReadInConfig(); err != nil {
		t.Fatalf("read start config: %v", err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load start config: %v", err)
	}
	paths, err := wallpapers.ScanWallpapers(cfg.Directories)
	if err != nil {
		t.Fatalf("scan start directories: %v", err)
	}

	rt.renderer = &fakeRenderer{scaleMode: cfg.ScaleMode, easing: cfg.Easing, framerate: cfg.FramerateLimit}
	rt.m = &Manager{wallpapers: paths, renderer: rt.renderer, cmds: make(chan Command, 1), cfg: cfg}
	return rt
}

func (rt *reloadTest) write(toml string) {
	rt.t.Helper()
	if err := os.WriteFile(rt.path, []byte(toml), 0o644); err != nil {
		rt.t.Fatal(err)
	}
}

// wallpaperDir makes a directory with n empty .png files.
func wallpaperDir(t *testing.T, n int) string {
	t.Helper()
	dir := t.TempDir()
	for i := range n {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%02d.png", i)), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// configTOML returns a valid config. Each entry in overrides replaces a key.
func configTOML(dirs []string, overrides map[string]string) string {
	quoted := make([]string, len(dirs))
	for i, dir := range dirs {
		quoted[i] = fmt.Sprintf("%q", dir)
	}
	values := map[string]string{
		"wallpapers":      "[" + strings.Join(quoted, ", ") + "]",
		"shuffle":         "false",
		"scale_mode":      `"center"`,
		"easing":          `"linear"`,
		"fade_speed":      "1",
		"delay":           "0",
		"framerate_limit": "60",
		"debug":           "false",
	}
	for key, value := range overrides {
		values[key] = value
	}
	var b strings.Builder
	for key, value := range values {
		fmt.Fprintf(&b, "%s = %s\n", key, value)
	}
	return b.String()
}

// A valid reload must reach every consumer: the renderer through the setters,
// the wallpaper list through a new scan of all directories, and c.cfg for the
// values that Run() and Next() read.
func TestReloadAppliesChanges(t *testing.T) {
	dirA := wallpaperDir(t, 2)
	dirB := wallpaperDir(t, 1)
	rt := newReloadTest(t, configTOML([]string{dirA}, nil))

	rt.write(configTOML([]string{dirA, dirB}, map[string]string{
		"scale_mode":      `"stretched"`,
		"easing":          `"ease-in"`,
		"framerate_limit": "30",
		"fade_speed":      "2.5",
		"delay":           "10",
	}))

	runNext, err := rt.m.reload()
	if err != nil {
		t.Fatalf("reload() error: %v", err)
	}
	if !runNext {
		t.Error("reload() did not report the new scan")
	}

	r := rt.renderer
	if r.scaleMode != types.ScalingModeStretch || r.easing != types.EasingEaseIn || r.framerate != 30 {
		t.Errorf("renderer = %s/%s/%d, want stretched/ease-in/30", r.scaleMode, r.easing, r.framerate)
	}

	want := []string{filepath.Join(dirA, "00.png"), filepath.Join(dirA, "01.png"), filepath.Join(dirB, "00.png")}
	if got := rt.m.GetWallpapers(); !slices.Equal(got, want) {
		t.Errorf("wallpapers = %q, want %q", got, want)
	}

	if rt.m.cfg.Delay != 10 || rt.m.cfg.FadeDuration() != 2500*time.Millisecond {
		t.Errorf("cfg delay = %d, fade = %v, want 10 and 2.5s", rt.m.cfg.Delay, rt.m.cfg.FadeDuration())
	}
}

// A failed reload must change nothing. Each broken config also changes the
// scale mode, so a partial apply shows up as a setter call.
func TestReloadKeepsOldStateOnError(t *testing.T) {
	dir := wallpaperDir(t, 2)
	newScale := map[string]string{"scale_mode": `"stretched"`}

	tests := []struct {
		name    string
		toml    string
		wantErr string
	}{
		{
			name:    "invalid value",
			toml:    configTOML([]string{dir}, map[string]string{"scale_mode": `"stretched"`, "delay": "-5"}),
			wantErr: "delay must be 0 or greater",
		},
		{
			name:    "wrong type",
			toml:    configTOML([]string{dir}, map[string]string{"scale_mode": `"stretched"`, "delay": `"abc"`}),
			wantErr: "delay must be a whole number",
		},
		{
			name:    "non-finite fade_speed",
			toml:    configTOML([]string{dir}, map[string]string{"scale_mode": `"stretched"`, "fade_speed": "nan"}),
			wantErr: "fade_speed must be a finite number",
		},
		{
			name:    "TOML syntax error",
			toml:    configTOML([]string{dir}, newScale) + "wallpapers = [\n",
			wantErr: "failed to read config file",
		},
		{
			name:    "missing directory",
			toml:    configTOML([]string{dir, filepath.Join(dir, "missing")}, newScale),
			wantErr: "wallpaper directory does not exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := newReloadTest(t, configTOML([]string{dir}, nil))
			oldCfg := rt.m.cfg
			oldWallpapers := slices.Clone(rt.m.GetWallpapers())

			rt.write(tt.toml)
			_, err := rt.m.reload()

			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("reload() error = %v, want it to contain %q", err, tt.wantErr)
			}
			if rt.m.cfg != oldCfg {
				t.Error("c.cfg was replaced")
			}
			if len(rt.renderer.calls) > 0 {
				t.Errorf("renderer setters were called: %v", rt.renderer.calls)
			}
			if got := rt.m.GetWallpapers(); !slices.Equal(got, oldWallpapers) {
				t.Errorf("wallpapers = %q, want %q", got, oldWallpapers)
			}
		})
	}
}

// A reload of an unchanged file must not touch the renderer or shuffle the
// list again. 20 files make an unchanged order by chance practically impossible.
func TestReloadWithoutChangesKeepsOrder(t *testing.T) {
	dir := wallpaperDir(t, 20)
	toml := configTOML([]string{dir}, map[string]string{"shuffle": "true"})
	rt := newReloadTest(t, toml)
	rt.m.Shuffle()
	before := slices.Clone(rt.m.GetWallpapers())

	runNext, err := rt.m.reload()
	if err != nil {
		t.Fatalf("reload() error: %v", err)
	}
	if runNext {
		t.Error("reload() reported a new scan without changes")
	}

	if got := rt.m.GetWallpapers(); !slices.Equal(got, before) {
		t.Error("the wallpaper order changed after a reload without changes")
	}
	if len(rt.renderer.calls) > 0 {
		t.Errorf("renderer setters were called: %v", rt.renderer.calls)
	}
}

// Many quick commands must not stop the daemon. The buffer holds one command,
// so the other senders wait. Run() needs the mutex in NextWallpaper() before it
// takes the next command, so a sender that waits with the mutex blocks Run().
func TestQuickCommandsDoNotDeadlock(t *testing.T) {
	dir := wallpaperDir(t, 2)
	rt := newReloadTest(t, configTOML([]string{dir}, nil))

	stopped := make(chan struct{})
	go func() {
		rt.m.Run()
		close(stopped)
	}()

	go func() {
		var wg sync.WaitGroup
		for range 5 {
			wg.Go(func() { rt.m.EnqueueCommand(Command{Type: CommandNext}) })
		}
		wg.Wait()
		rt.m.EnqueueCommand(Command{Type: CommandStop})
	}()

	select {
	case <-stopped:
	case <-time.After(10 * time.Second):
		t.Fatal("Run() did not stop: the command queue is blocked")
	}
}

// The --debug flag wins over the config file, also at a reload. The level
// starts at info, so the test fails if reload() ignores the debug setting or
// uses only the value from the file.
func TestReloadKeepsDebugFlag(t *testing.T) {
	dir := wallpaperDir(t, 1)
	rt := newReloadTest(t, configTOML([]string{dir}, nil))

	root := &cobra.Command{}
	root.Flags().Bool("debug", false, "")
	if err := viper.BindPFlag("debug", root.Flags().Lookup("debug")); err != nil {
		t.Fatal(err)
	}
	if err := root.Flags().Set("debug", "true"); err != nil {
		t.Fatal(err)
	}
	log.SetLevel(log.InfoLevel)
	t.Cleanup(func() { log.SetLevel(log.InfoLevel) })

	rt.write(configTOML([]string{dir}, map[string]string{"debug": "false"}))
	if _, err := rt.m.reload(); err != nil {
		t.Fatalf("reload() error: %v", err)
	}

	if got := log.GetLevel(); got != log.DebugLevel {
		t.Errorf("log level = %s, want debug", got)
	}
}
