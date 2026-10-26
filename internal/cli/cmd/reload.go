package cmd

import (
	"github.com/charmbracelet/log"
	"github.com/matjam/smoothpaper/internal/ipc"
	"github.com/spf13/cobra"
)

func NewReloadCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reload",
		Short: "Reload the active config file",
		Run: func(cmd *cobra.Command, args []string) {
			if err := ipc.SendReload(); err != nil {
				log.Fatalf("Failed to reload config: %v", err)
			}
			log.Info("Config reloaded")
		},
	}
}
