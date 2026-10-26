package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/log"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/matjam/smoothpaper/internal/config"
	"github.com/matjam/smoothpaper/internal/ipc"
	"github.com/matjam/smoothpaper/internal/wallpapers"
)

func StartManager() {
	log.Infof("StartManager() started in PID: %d", os.Getpid())

	if os.Getenv("BACKGROUND_PROCESS") == "1" {
		setupRotatingLogger()
	}

	if _, err := ipc.SendStatus(); err == nil {
		log.Infof("smoothpaper is already running, exiting")
		os.Exit(0)
	}

	cfg, paths := LoadConfigAndWallpapers()

	log.Infof("Shuffle: %v", cfg.Shuffle)

	manager := ipc.NewManager(paths, cfg)
	if cfg.Shuffle {
		manager.Shuffle()
	}

	go func() {
		log.Infof("Starting socket server")
		ipc.Start(manager)
	}()

	log.Infof("Running with %d wallpapers", len(manager.GetWallpapers()))
	manager.Run()

	sockDir := os.Getenv("XDG_RUNTIME_DIR")
	if sockDir == "" {
		sockDir = os.TempDir()
	}
	os.Remove(sockDir + "/smoothpaper.sock")
	log.Infof("smoothpaper exited")
}

// LoadConfigAndWallpapers loads the config and scans the wallpaper directories.
// It logs a warning for each invalid value, and it stops the program if the
// scan fails. With -b, the parent also calls it before the fork, these
// messages show in the terminal and not only in the log file
func LoadConfigAndWallpapers() (*config.Config, []string) {
	cfg, problems := config.LoadWithDefaults()
	for _, err := range problems {
		log.Warnf("Invalid config value, using default value: %v", err)
	}

	if cfg.Debug {
		log.SetLevel(log.DebugLevel)
	}

	paths, err := wallpapers.ScanWallpapers(cfg.Directories)
	if err != nil {
		log.Fatalf("Failed to scan wallpapers: %v", err)
	}
	return cfg, paths
}

// How long the parent waits for the daemon to answer
var startTimeout = 10 * time.Second

// WaitForDaemon waits until the daemon answers on the socket. The daemon logs
// its errors only to the log file, so the parent must find out if the daemon
// stopped before it exits with code 0. After the timeout, it kills the child,
// so an error always means that no daemon runs.
func WaitForDaemon(child *os.Process) error {
	exited := make(chan struct{})
	go func() {
		child.Wait()
		close(exited)
	}()

	timeout := time.After(startTimeout)
	for {
		select {
		case <-exited:
			return errors.New("the daemon stopped during startup")
		case <-timeout:
			child.Kill()
			<-exited
			return fmt.Errorf("the daemon did not answer in %v, killed the process", startTimeout)
		case <-time.After(100 * time.Millisecond):
			if _, err := ipc.SendStatus(); err == nil {
				return nil
			}
		}
	}
}

func LogPath() string {
	return filepath.Join(os.Getenv("HOME"), ".local", "share", "smoothpaper", "smoothpaper.log")
}

func setupRotatingLogger() {
	logPath := LogPath()

	writer, err := rotatelogs.New(
		logPath+".%Y%m%d%H%M",
		rotatelogs.WithLinkName(logPath),
		rotatelogs.WithMaxAge(7*24*time.Hour),
		rotatelogs.WithRotationSize(10*1024*1024),
		rotatelogs.WithRotationTime(24*time.Hour),
	)
	if err != nil {
		log.Fatalf("failed to configure log rotation: %v", err)
	}

	log.SetOutput(writer)
	log.SetLevel(log.InfoLevel)
}
