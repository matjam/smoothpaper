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

	wallpaperPaths := make([]string, 0)
	paths := make([]string, 0)

	if !viper.IsSet("wallpapers") {
		log.Fatal("No wallpapers directory specified. Please set the 'wallpapers' configuration.")
	}

	wallpapersConfig := viper.Get("wallpapers")

	switch v := wallpapersConfig.(type) {
	case string:
		paths = append(paths, utils.CanonicalPath(v))
	case []any:
		for _, wallpaperEntry := range v {
			wallpaper, ok := wallpaperEntry.(string)
			if !ok {
				log.Fatalf("Invalid type for wallpaper entry: %T", wallpaperEntry)
			}
			paths = append(paths, utils.CanonicalPath(wallpaper))
		}
	default:
		log.Fatalf("Invalid type for wallpapers configuration: %T", v)
	}

	log.Infof("Wallpaper directories: %v", paths)

	for _, path := range paths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			log.Fatalf("Wallpaper directory does not exist: %s", path)
		}

		dirEntries, err := os.ReadDir(path)
		if err != nil {
			log.Fatalf("Error reading wallpapers directory: %v", err)
		}

		for _, entry := range dirEntries {
			if entry.IsDir() {
				continue
			}
			name := strings.ToLower(entry.Name())
			if strings.HasSuffix(name, ".png") ||
				strings.HasSuffix(name, ".jpg") ||
				strings.HasSuffix(name, ".jpeg") ||
				strings.HasSuffix(name, ".gif") {
				wallpaperPaths = append(wallpaperPaths, filepath.Join(path, entry.Name()))
			}
		}
	}

	if len(wallpaperPaths) == 0 {
		log.Fatal("No valid wallpapers found in the specified directories.")
	}

	log.Infof("Found %d wallpapers in %s", len(wallpaperPaths), viper.GetString("wallpapers"))
	log.Infof("First wallpaper: %s", wallpaperPaths[0])
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
