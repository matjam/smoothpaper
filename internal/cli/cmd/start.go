package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/matjam/smoothpaper/internal/cli/cmd/utils"
	"github.com/matjam/smoothpaper/internal/ipc"
	"github.com/spf13/viper"
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

	log.Info("Searching for images ...")

	wallpapers, err := os.ReadDir(utils.CanonicalPath(viper.GetString("wallpapers")))
	if err != nil {
		log.Fatalf("Error reading wallpapers directory: %v", err)
	}

	if len(wallpapers) == 0 {
		log.Fatal("No wallpapers found in the specified directory.")
	}

	wallpaperPaths := make([]string, 0)
	for _, wallpaper := range wallpapers {
		if wallpaper.IsDir() {
			continue
		}

		name := strings.ToLower(wallpaper.Name())
		if strings.HasSuffix(name, ".png") ||
			strings.HasSuffix(name, ".jpg") ||
			strings.HasSuffix(name, ".jpeg") ||
			strings.HasSuffix(name, ".gif") {
			wallpaperPaths = append(wallpaperPaths, filepath.Join(utils.CanonicalPath(viper.GetString("wallpapers")), wallpaper.Name()))
		}
	}

	if len(wallpaperPaths) == 0 {
		log.Fatal("No valid wallpapers found in the specified directory.")
	}

	log.Infof("Found %d wallpapers in %s", len(wallpaperPaths), viper.GetString("wallpapers"))
	log.Infof("First wallpaper: %s", wallpapers[0].Name())
	log.Infof("Shuffle: %v", viper.GetBool("shuffle"))

	manager := ipc.NewManager(wallpaperPaths)
	if viper.GetBool("shuffle") {
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

func setupRotatingLogger() {
	home := os.Getenv("HOME")
	logDir := filepath.Join(home, ".local", "share", "smoothpaper")
	logPath := filepath.Join(logDir, "smoothpaper.log")

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
