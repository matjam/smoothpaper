/*
Copyright © 2025 Nathan Ollerenshaw <chrome@stupendous.net>
*/
package cli

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "smoothpaper",
	Short: "A hardware accelerated wallpaper changer",
	Long: `Smoothpaper is a wallpaper changer with smooth transitions for 
X11 Window Managers using OpenGL for hardware acceleration.`,
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
	rootCmd.PersistentFlags().StringP("author", "a", "YOUR NAME", "author name for copyright attribution")
	rootCmd.PersistentFlags().Bool("viper", true, "use Viper for configuration")
	viper.BindPFlag("author", rootCmd.PersistentFlags().Lookup("author"))
	viper.BindPFlag("useViper", rootCmd.PersistentFlags().Lookup("viper"))
	viper.SetDefault("author", "NAME HERE <EMAIL ADDRESS>")
	viper.SetDefault("license", "apache")
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("smoothpaper")
		viper.SetConfigType("toml")
		viper.AddConfigPath("$HOME/.config/smoothpaper")
		viper.AddConfigPath("/etc/xdg/smoothpaper")

		viper.SetDefault("wallpapers", "~/Pictures/wallpapers")
		viper.SetDefault("shuffle", true)
		viper.SetDefault("scale_mode", "vertical")
		viper.SetDefault("easing", "ease-in-out")
		viper.SetDefault("fade_speed", 1.0)
		viper.SetDefault("delay", 300)
		viper.SetDefault("framerate_limit", 60)
		viper.SetDefault("debug", false)

	}

	viper.AutomaticEnv() // read environment variables that match

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.cli.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	err := viper.ReadInConfig()
	cobra.CheckErr(err)

}
