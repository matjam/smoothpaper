package cmd

import (
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

// NewGenManCmd returns a cobra command to generate man pages
func NewGenManCmd(rootCmd *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:   "genman [output-dir]",
		Short: "Generate man pages for smoothpaper CLI",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := args[0]
			header := &doc.GenManHeader{
				Title:   "SMOOTHPAPER",
				Section: "1",
			}
			return doc.GenManTree(rootCmd, header, filepath.Clean(dir))
		},
	}
}
