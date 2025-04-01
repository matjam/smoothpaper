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
	Long: `Smoothpaper is a wallpaper changer with smooth transitions for 
X11 Window Managers using OpenGL for hardware acceleration.`,
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
