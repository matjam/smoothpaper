/*
Copyright © 2025 Nathan Ollerenshaw <chrome@stupendous.net>
*/
package cli

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/matjam/smoothpaper"
	"github.com/matjam/smoothpaper/internal/cli/cmd"
	"github.com/matjam/smoothpaper/internal/cli/cmd/utils"
	"github.com/matjam/smoothpaper/internal/ipc"
	"github.com/sevlyar/go-daemon"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// rootCmd represents the base command when called without any subcommands.
// It handles the main program execution flow including daemon management,
// configuration display, and version information.
var rootCmd = &cobra.Command{
	Use:   "smoothpaper",
	Short: "A hardware accelerated wallpaper changer",
	Long: `smoothpaper is a hardware-accelerated wallpaper daemon for X11 window managers.

	It provides smooth, GPU-powered transitions between wallpapers using OpenGL, and is
	intended for use as a long-running background process. You can launch it in daemon mode
	using the -b or --background flag.
	
	Once running, smoothpaper exposes a local UNIX domain socket API for control. You can
	send commands such as:
	
	  • status — check if the daemon is running and inspect the current wallpaper
	  • next   — immediately transition to the next wallpaper
	  • stop   — gracefully shut down the background daemon
	  • load   — load a new list of wallpaper file paths
	
	Wallpapers are shuffled by default (unless configured otherwise), and transitions can
	be customized via the configuration file.
	
	The default configuration file is located at:
	
	  $XDG_CONFIG_HOME/smoothpaper/smoothpaper.toml
	
	Logs are written to:
	
	  $HOME/.local/share/smoothpaper/smoothpaper.log
	
	and are automatically rotated to prevent log growth over time.
	
	You can generate man pages and shell completion scripts for smoothpaper using the built-in
	'genman' and 'completion' subcommands. See their help output for usage.
	
	smoothpaper is designed to be lightweight, fast, and aesthetically pleasing for users
	who want animated wallpaper transitions without compromising system performance.`,
	Run: func(cmdObj *cobra.Command, args []string) {
		// DAEMON MODE: Handle background flag by forking the process
		// We handle this first to ensure proper daemonization before any other operations
		if v, err := cmdObj.Flags().GetBool("background"); err == nil && v {
			// Check if daemon is already running
			if _, err := ipc.SendStatus(); err == nil {
				log.Infof("smoothpaper is already running, exiting")
				os.Exit(0)
			}

			// Create data directory if it doesn't exist
			home := os.Getenv("HOME")
			dataDir := filepath.Join(home, ".local", "share", "smoothpaper")
			_ = os.MkdirAll(dataDir, 0755)

			// Filter out background flags to prevent recursive daemonization
			filteredArgs := []string{}
			for _, arg := range os.Args[1:] {
				if arg != "-b" && arg != "--background" {
					filteredArgs = append(filteredArgs, arg)
				}
			}

			// Configure daemon context
			ctx := &daemon.Context{
				PidFileName: filepath.Join(dataDir, "smoothpaper.pid"),
				PidFilePerm: 0644,
				LogFileName: "", // Logging is configured separately in cmd.StartManager()
				WorkDir:     "./",
				Umask:       027,
				Args:        append([]string{os.Args[0]}, filteredArgs...),
			}

			// Fork the process
			d, err := ctx.Reborn()
			if err != nil {
				log.Fatalf("Failed to daemonize: %v", err)
			}
			if d != nil {
				// Parent process exits after successful fork
				log.Infof("Parent exiting, child PID: %d", d.Pid)
				os.Exit(0)
			}

			// Child process continues
			defer ctx.Release()
			log.Infof("Daemon process started, PID: %d", os.Getpid())
			os.Setenv("BACKGROUND_PROCESS", "1")
		}

		// CONFIGURATION: Display all configuration settings when requested
		if v, err := cmdObj.Flags().GetBool("show-config"); err == nil && v {
			allSettings := viper.AllSettings()
			log.Infof("Using config file: %v", viper.ConfigFileUsed())
			log.Infof("All settings:")
			cmd.PrintJSONColored(allSettings)
			return
		}

		// VERSION: Print version information and exit
		if v, err := cmdObj.Flags().GetBool("version"); err == nil && v {
			// Display version from the embedded version.txt file
			log.Infof("smoothpaper version %s", strings.TrimSpace(smoothpaper.Version))
			return
		}

		// CONFIG INSTALLATION: Create default config file when requested
		if v, err := cmdObj.Flags().GetBool("installconfig"); err == nil && v {
			utils.InstallDefaultConfig()
			return
		}

		// MAIN EXECUTION: Start the wallpaper manager service if no other flags were processed
		cmd.StartManager()
	},
}

// Execute adds all child commands to the root command and sets up the configuration.
// It's the main entry point for the CLI application and is called directly from main.go.
// This function handles command registration, flag parsing, and error reporting.
func Execute() {
	RegisterFlags(rootCmd)

	// Register subcommands
	rootCmd.AddCommand(cmd.NewStatusCmd())
	rootCmd.AddCommand(cmd.NewNextCmd())
	rootCmd.AddCommand(cmd.NewStopCmd())
	rootCmd.AddCommand(cmd.NewLoadCmd())
	rootCmd.AddCommand(cmd.NewGenManCmd(rootCmd))

	// Initialize configuration before command execution
	cobra.OnInitialize(InitConfig)

	// Execute the root command and handle any errors
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
