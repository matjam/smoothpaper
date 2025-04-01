/*
Copyright © 2025 Nathan Ollerenshaw <chrome@stupendous.net>
*/
package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/matjam/smoothpaper"
	"github.com/matjam/smoothpaper/internal/command"
	"github.com/matjam/smoothpaper/internal/wallpaper"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tidwall/pretty"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "smoothpaper",
	Short: "A hardware accelerated wallpaper changer",
	Long: `Smoothpaper is a wallpaper changer with smooth transitions for 
X11 Window Managers using OpenGL for hardware acceleration.`,
	Run: func(cmd *cobra.Command, args []string) {
		if v, err := cmd.Flags().GetBool("show-config"); err == nil && v {
			allSettings := viper.AllSettings()

			log.Infof("Using config file: %v", viper.ConfigFileUsed())
			log.Infof("All settings:")
			printJSONColored(allSettings)
			return
		}

		babyBlue := lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
		yellow := lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
		green := lipgloss.NewStyle().Foreground(lipgloss.Color("76"))
		if v, err := cmd.Flags().GetBool("version"); err == nil && v {
			log.Infof("%v version %v © 2025 %v",
				babyBlue.Render("smoothpaper "),
				green.Render(strings.Trim(smoothpaper.Version, "\n\r ")),
				yellow.Render("Nathan Ollerenshaw"))
			return
		}

		if v, err := cmd.Flags().GetBool("installconfig"); err == nil && v {
			configDir := os.Getenv("XDG_CONFIG_HOME")
			if configDir == "" {
				configDir = os.Getenv("HOME") + "/.config"
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
			return
		}

		startManager()
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Get smoothpaper status",
	Long:  `Returns the current status of the smoothpaper process.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := command.GetStatus(); err != nil {
			log.Fatalf("Error pinging socket: %v", err)
		}
	},
}

func canonicalPath(path string) string {
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

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

var cfgFile string

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/smoothpaper/smoothpaper.toml)")
	viper.BindPFlag("config", rootCmd.PersistentFlags().Lookup("config"))
	rootCmd.PersistentFlags().BoolP("installconfig", "i", false, "Install a default config file")
	rootCmd.PersistentFlags().Bool("show-config", false, "Dump resolved config")
	rootCmd.PersistentFlags().BoolP("background", "b", false, "Run as a daemon")
	rootCmd.PersistentFlags().BoolP("debug", "d", false, "Enable debug logging")
	rootCmd.PersistentFlags().BoolP("version", "v", false, "Print version")
	rootCmd.PersistentFlags().BoolP("help", "h", false, "Print usage")

	rootCmd.AddCommand(statusCmd)
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("smoothpaper")
		viper.SetConfigType("toml")
		if viper.GetString("config") != "" {
			viper.SetConfigFile(viper.GetString("config"))
		} else {
			viper.AddConfigPath("$HOME/.config/smoothpaper")
			viper.AddConfigPath("/etc/xdg/smoothpaper")
		}
	}

	viper.SetDefault("wallpapers", "~/Pictures/wallpapers")
	viper.SetDefault("shuffle", true)
	viper.SetDefault("scale_mode", "vertical")
	viper.SetDefault("easing", "ease-in-out")
	viper.SetDefault("fade_speed", 1.0)
	viper.SetDefault("delay", 300)
	viper.SetDefault("framerate_limit", 60)
	viper.SetDefault("debug", false)

	viper.AutomaticEnv() // read environment variables that match

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	err := viper.ReadInConfig()
	cobra.CheckErr(err)
}

func printJSONColored(data interface{}) {
	j, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Errorf("Error marshalling JSON: %v", err)
		return
	}

	jPretty := pretty.Color(j, nil)
	log.Info(string(jPretty))
}

func startManager() {
	err := command.GetStatus()
	if err == nil {
		log.Infof("smoothpaper is already running, exiting")
		os.Exit(0)
	}

	log.Info("Searching for images ...")

	wallpapers, err := os.ReadDir(canonicalPath(viper.GetString("wallpapers")))
	if err != nil {
		log.Fatalf("Error reading wallpapers directory: %v", err)
	}

	if len(wallpapers) == 0 {
		log.Fatal("No wallpapers found in the specified directory.")
	}

	wallpaperPaths := make([]string, 0)
	for i, wallpaper := range wallpapers {
		if wallpaper.IsDir() {
			continue
		}

		name := strings.ToLower(wallpaper.Name())
		if strings.HasSuffix(name, ".png") ||
			strings.HasSuffix(name, ".jpg") ||
			strings.HasSuffix(name, ".jpeg") ||
			strings.HasSuffix(name, ".gif") {
			wallpaperPaths = append(wallpaperPaths, filepath.Join(canonicalPath(viper.GetString("wallpapers")), wallpaper.Name()))

			log.Infof("Found wallpaper: %s", wallpaperPaths[i])

			continue
		}
	}

	if len(wallpaperPaths) == 0 {
		log.Fatal("No valid wallpapers found in the specified directory.")
	}

	log.Infof("Found %d wallpapers in %s", len(wallpaperPaths), viper.GetString("wallpapers"))
	log.Infof("First wallpaper: %s", wallpapers[0].Name())
	log.Infof("Shuffle: %v", viper.GetBool("shuffle"))

	manager := wallpaper.NewManager(wallpaperPaths)
	if viper.GetBool("shuffle") {
		manager.Shuffle()
	}

	go func() {
		log.Infof("Starting socket server")
		wallpaper.StartSocketServer(manager)
	}()

	log.Infof("Running with %d wallpapers", len(manager.GetWallpapers()))
	manager.Run()

	// unlink the socket
	sockDir := os.Getenv("XDG_RUNTIME_DIR")
	if sockDir == "" {
		sockDir = os.TempDir()
	}
	sockPath := sockDir + "/smoothpaper.sock"

	os.Remove(sockPath)

	log.Infof("smoothpaper exited")
}
