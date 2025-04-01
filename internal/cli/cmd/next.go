package cmd

import (
	"github.com/charmbracelet/log"
	"github.com/matjam/smoothpaper/internal/ipc"
	"github.com/spf13/cobra"
)

func NewNextCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "next",
		Short: "Switch to the next wallpaper",
		Run: func(cmd *cobra.Command, args []string) {
			if err := ipc.SendNext(); err != nil {
				log.Fatalf("Failed to send 'next' command: %v", err)
			}
			log.Info("Next wallpaper command sent")
		},
	}
}
