package cmd

import (
	"github.com/charmbracelet/log"
	"github.com/matjam/smoothpaper/internal/ipc"
	"github.com/spf13/cobra"
)

func NewStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the smoothpaper daemon",
		Run: func(cmd *cobra.Command, args []string) {
			if err := ipc.SendStop(); err != nil {
				log.Fatalf("Failed to send 'stop' command: %v", err)
			}
			log.Info("Stop command sent")
		},
	}
}
