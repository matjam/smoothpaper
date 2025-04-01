package cmd

import (
	"github.com/charmbracelet/log"
	"github.com/matjam/smoothpaper/internal/ipc"
	"github.com/spf13/cobra"
)

func NewLoadCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "load [wallpaper1.jpg] [wallpaper2.png] ...",
		Short: "Load a new list of wallpapers into the daemon",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if err := ipc.SendLoad(args); err != nil {
				log.Fatalf("Failed to send 'load' command: %v", err)
			}
			log.Infof("Loaded %d wallpapers", len(args))
		},
	}
}
