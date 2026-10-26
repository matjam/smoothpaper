package wallpapers

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/matjam/smoothpaper/internal/cli/cmd/utils"
)

func ScanWallpapers(directories []string) ([]string, error) {
	paths := make([]string, 0)
	wallpaperPaths := make([]string, 0)

	if len(directories) == 0 {
		return nil, errors.New("no wallpapers directory specified")
	}

	for _, wallpaper := range directories {
		paths = append(paths, utils.CanonicalPath(wallpaper))
	}

	log.Infof("Wallpaper directories: %v", paths)

	for _, path := range paths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return nil, fmt.Errorf("wallpaper directory does not exist: %s", path)
		}

		dirEntries, err := os.ReadDir(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read wallpaper directory: %w", err)
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
		return nil, errors.New("no valid wallpapers found in the specified directories")
	}

	log.Infof("Found %d wallpapers in %s", len(wallpaperPaths), directories)
	log.Infof("First wallpaper: %s", wallpaperPaths[0])

	return wallpaperPaths, nil
}
