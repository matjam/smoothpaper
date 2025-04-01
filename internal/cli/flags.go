package cli

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

func RegisterFlags(rootCmd *cobra.Command) {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/smoothpaper/smoothpaper.toml)")
	viper.BindPFlag("config", rootCmd.PersistentFlags().Lookup("config"))

	rootCmd.PersistentFlags().BoolP("installconfig", "i", false, "Install a default config file")
	rootCmd.PersistentFlags().Bool("show-config", false, "Dump resolved config")
	rootCmd.PersistentFlags().BoolP("background", "b", false, "Run as a daemon")
	rootCmd.PersistentFlags().BoolP("debug", "d", false, "Enable debug logging")
	rootCmd.PersistentFlags().BoolP("version", "v", false, "Print version")
	rootCmd.PersistentFlags().BoolP("help", "h", false, "Print usage")

	// You can add more shared flags here if needed
}
