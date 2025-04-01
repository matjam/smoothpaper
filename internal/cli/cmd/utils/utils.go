package utils

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/matjam/smoothpaper"
	"github.com/tidwall/pretty"
)

func CanonicalPath(path string) string {
	if path == "" {
		return ""
	}

	if path == "~" {
		return os.Getenv("HOME")
	}

	if strings.HasPrefix(path, "~/") {
		homeDir := os.Getenv("HOME")
		return strings.Replace(path, "~", homeDir, 1)
	}

	return path
}

func PrintJSONColored(data interface{}) {
	j, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Errorf("Error marshalling JSON: %v", err)
		return
	}

	jPretty := pretty.Color(j, nil)
	log.Info(string(jPretty))
}

func InstallDefaultConfig() {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		configDir = filepath.Join(os.Getenv("HOME"), ".config")
	}

	configPath := filepath.Join(configDir, "smoothpaper", "smoothpaper.toml")

	if _, err := os.Stat(configPath); err == nil {
		log.Warnf("Config file already exists at %v", configPath)
		return
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		log.Fatalf("Error creating config directory: %v", err)
	}

	if err := os.WriteFile(configPath, []byte(smoothpaper.DefaultConfig), 0644); err != nil {
		log.Fatalf("Error writing config file: %v", err)
	}

	log.Infof("Installed default config file at %v", configPath)
}
