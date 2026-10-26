package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/matjam/smoothpaper"
	"github.com/matjam/smoothpaper/internal/config"
	"github.com/spf13/viper"
)

// Each config file must pass the validation in config.Load() together with the
// defaults from InitConfig(). "smoothpaper -i" installs the embedded default
// config. A file with only the wallpapers key makes sure that each other key
// has a valid default, so a missing key never stops the startup.
// InitConfig() runs one time only, because a second call panics on the
// "toggle" flag.
func TestConfigFilesLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "smoothpaper.toml")
	cfgFile = path
	t.Cleanup(func() {
		cfgFile = ""
		viper.Reset()
	})
	InitConfig()

	tests := []struct {
		name string
		toml string
	}{
		{name: "embedded default config", toml: smoothpaper.DefaultConfig},
		{name: "only wallpapers", toml: `wallpapers = "~/Pictures"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(tt.toml), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := viper.ReadInConfig(); err != nil {
				t.Fatalf("config does not parse: %v", err)
			}
			if _, err := config.Load(); err != nil {
				t.Fatalf("config does not load: %v", err)
			}
		})
	}
}

// --show-config prints viper.AllSettings() as JSON, and TOML accepts nan and
// inf. The test uses the types that viper really returns for a TOML table and
// a TOML array, so a missed type shows up as a json.Marshal error.
func TestShowConfigAcceptsNonFiniteNumbers(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.SetConfigType("toml")
	toml := "fade_speed = nan\n[table]\nlist = [inf, 1.5]\n"
	if err := viper.ReadConfig(strings.NewReader(toml)); err != nil {
		t.Fatalf("parse TOML: %v", err)
	}

	j, err := json.Marshal(jsonSafe(viper.AllSettings()))
	if err != nil {
		t.Fatalf("json.Marshal() error: %v", err)
	}
	want := `{"fade_speed":"NaN","table":{"list":["+Inf",1.5]}}`
	if string(j) != want {
		t.Errorf("JSON = %s, want %s", j, want)
	}
}
