/*
Copyright © 2025 Nathan Ollerenshaw <chrome@stupendous.net>
*/
package cli

import (
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"
	"github.com/matjam/smoothpaper/internal/cli/cmd"
	"github.com/matjam/smoothpaper/internal/cli/cmd/utils"
	"github.com/matjam/smoothpaper/internal/ipc"
	"github.com/sevlyar/go-daemon"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

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
		// Check for --background and fork early
		if v, err := cmdObj.Flags().GetBool("background"); err == nil && v {
			if _, err := ipc.SendStatus(); err == nil {
				log.Infof("smoothpaper is already running, exiting")
				os.Exit(0)
			}

			home := os.Getenv("HOME")
			dataDir := filepath.Join(home, ".local", "share", "smoothpaper")
			_ = os.MkdirAll(dataDir, 0755)

			// Strip -b from args so the child doesn't daemonize again
			filteredArgs := []string{}
			for _, arg := range os.Args[1:] {
				if arg != "-b" && arg != "--background" {
					filteredArgs = append(filteredArgs, arg)
				}
			}

			ctx := &daemon.Context{
				PidFileName: filepath.Join(dataDir, "smoothpaper.pid"),
				PidFilePerm: 0644,
				LogFileName: "", // we'll configure logging ourselves
				WorkDir:     "./",
				Umask:       027,
				Args:        append([]string{os.Args[0]}, filteredArgs...),
			}

			d, err := ctx.Reborn()
			if err != nil {
				log.Fatalf("Failed to daemonize: %v", err)
			}
			if d != nil {
				log.Infof("Parent exiting, child PID: %d", d.Pid)
				os.Exit(0)
			}

			defer ctx.Release()
			log.Infof("Daemon process started, PID: %d", os.Getpid())
			os.Setenv("BACKGROUND_PROCESS", "1")
		}

		if v, err := cmdObj.Flags().GetBool("show-config"); err == nil && v {
			allSettings := viper.AllSettings()
			log.Infof("Using config file: %v", viper.ConfigFileUsed())
			log.Infof("All settings:")
			cmd.PrintJSONColored(allSettings)
			return
		}

		if v, err := cmdObj.Flags().GetBool("version"); err == nil && v {
			// version printing stuff
			return
		}

		if v, err := cmdObj.Flags().GetBool("installconfig"); err == nil && v {
			utils.InstallDefaultConfig()
			return
		}

		// ✅ Only call into the daemon/manager here:
		cmd.StartManager()
	},
}

func Execute() {
	RegisterFlags(rootCmd)
	rootCmd.AddCommand(cmd.NewStatusCmd())
	rootCmd.AddCommand(cmd.NewNextCmd())
	rootCmd.AddCommand(cmd.NewStopCmd())
	rootCmd.AddCommand(cmd.NewLoadCmd())
	rootCmd.AddCommand(cmd.NewGenManCmd(rootCmd))
	cobra.OnInitialize(InitConfig)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
