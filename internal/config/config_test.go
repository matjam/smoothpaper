package config

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/matjam/smoothpaper/internal/types"
	"github.com/spf13/viper"
)

// readTOML gives viper TOML text instead of a file. Each entry in overrides
// replaces the value of that key in an otherwise valid config.
func readTOML(t *testing.T, overrides map[string]string) {
	t.Helper()

	values := map[string]string{
		"wallpapers":      `"~/Pictures"`,
		"shuffle":         "true",
		"scale_mode":      `"vertical"`,
		"easing":          `"linear"`,
		"fade_speed":      "1",
		"delay":           "300",
		"framerate_limit": "60",
		"debug":           "false",
	}
	maps.Copy(values, overrides)

	var toml strings.Builder
	for key, value := range values {
		fmt.Fprintf(&toml, "%s = %s\n", key, value)
	}

	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.SetConfigType("toml")
	if err := viper.ReadConfig(strings.NewReader(toml.String())); err != nil {
		t.Fatalf("parse TOML: %v", err)
	}
}

// loadTOML runs Load() on the config from readTOML().
func loadTOML(t *testing.T, overrides map[string]string) (*Config, error) {
	t.Helper()
	readTOML(t, overrides)
	return Load()
}

// The wallpapers key accepts one string or a list. The scan in start.go before
// Load() kept a single string as one path, so a comma must not split it. An
// empty string must not become a list with one empty path.
func TestLoadWallpapersAcceptsStringOrList(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    []string
		wantErr bool
	}{
		{name: "single string", value: `"~/Pictures"`, want: []string{"~/Pictures"}},
		{name: "list", value: `["~/Pictures", "/srv/walls"]`, want: []string{"~/Pictures", "/srv/walls"}},
		{name: "comma in a single string", value: `"~/Pictures/red,blue"`, want: []string{"~/Pictures/red,blue"}},
		{name: "empty string", value: `""`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := loadTOML(t, map[string]string{"wallpapers": tt.value})
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Load() returned no error, Directories = %q", cfg.Directories)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() error: %v", err)
			}
			if !slices.Equal(cfg.Directories, tt.want) {
				t.Errorf("Directories = %q, want %q", cfg.Directories, tt.want)
			}
		})
	}
}

// fade_speed is in seconds and can have a fraction. A TOML integer must also
// decode into the float64 field.
func TestFadeDurationKeepsFraction(t *testing.T) {
	tests := []struct {
		value string
		want  time.Duration
	}{
		{value: "1.5", want: 1500 * time.Millisecond},
		{value: "0.25", want: 250 * time.Millisecond},
		{value: "5", want: 5 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			cfg, err := loadTOML(t, map[string]string{"fade_speed": tt.value})
			if err != nil {
				t.Fatalf("Load() error: %v", err)
			}
			if got := cfg.FadeDuration(); got != tt.want {
				t.Errorf("FadeDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}

// These values pass a simple range check, but they stop the daemon. NaN passes
// "fade_speed < 0", and a NaN or infinite fade never ends, so Run() takes no
// more commands. A delay or fade_speed above maxSeconds overflows
// time.Duration. An accepted fade_speed must give a positive FadeDuration().
func TestLoadRejectsValuesThatBreakTheDaemon(t *testing.T) {
	tests := []struct {
		key     string
		value   string
		wantErr string
	}{
		{key: "fade_speed", value: "nan", wantErr: "fade_speed must be a finite number"},
		{key: "fade_speed", value: "inf", wantErr: "fade_speed must be a finite number"},
		{key: "fade_speed", value: fmt.Sprint(maxSeconds), wantErr: ""},
		{key: "fade_speed", value: fmt.Sprint(maxSeconds + 1), wantErr: "fade_speed must be 9223372036 or less"},
		{key: "delay", value: fmt.Sprint(maxSeconds), wantErr: ""},
		{key: "delay", value: fmt.Sprint(maxSeconds + 1), wantErr: "delay must be 9223372036 or less"},
	}

	for _, tt := range tests {
		t.Run(tt.key+" = "+tt.value, func(t *testing.T) {
			cfg, err := loadTOML(t, map[string]string{tt.key: tt.value})
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Load() error: %v", err)
				}
				if got := cfg.FadeDuration(); got <= 0 {
					t.Errorf("FadeDuration() = %v, want a positive duration", got)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Load() error = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

// At startup, an invalid value must replace only its own key with the default.
// The other keys keep the values from the file. A value of the wrong type is
// invalid too, and Load() must still reject the same file.
func TestLoadWithDefaultsReplacesOnlyInvalidValues(t *testing.T) {
	readTOML(t, map[string]string{
		"delay":      "-5",
		"shuffle":    `"yes"`,
		"fade_speed": "nan",
		"easing":     `"ease-out"`,
		"scale_mode": `"center"`,
	})

	cfg, problems := LoadWithDefaults()

	def := Default()
	if cfg.Delay != def.Delay || cfg.Shuffle != def.Shuffle || cfg.FadeSpeed != def.FadeSpeed {
		t.Errorf("delay/shuffle/fade_speed = %d/%v/%v, want the defaults %d/%v/%v",
			cfg.Delay, cfg.Shuffle, cfg.FadeSpeed, def.Delay, def.Shuffle, def.FadeSpeed)
	}
	if cfg.Easing != types.EasingEaseOut || cfg.ScaleMode != types.ScalingModeCenter {
		t.Errorf("easing/scale_mode = %s/%s, want ease-out/center from the file", cfg.Easing, cfg.ScaleMode)
	}
	if len(problems) != 3 {
		t.Errorf("got %d problems, want 3: %v", len(problems), problems)
	}

	if _, err := Load(); err == nil {
		t.Error("Load() accepted the same file")
	}
}
