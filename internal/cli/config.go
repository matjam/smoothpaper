package cli

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func InitConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("smoothpaper")
		viper.SetConfigType("toml")
		if viper.GetString("config") != "" {
			viper.SetConfigFile(viper.GetString("config"))
		} else {
			viper.AddConfigPath("$HOME/.config/smoothpaper")
			viper.AddConfigPath("/etc/xdg/smoothpaper")
		}
	}

	viper.SetDefault("wallpapers", "~/Pictures/wallpapers")
	viper.SetDefault("shuffle", true)
	viper.SetDefault("scale_mode", "vertical")
	viper.SetDefault("easing", "ease-in-out")
	viper.SetDefault("fade_speed", 1.0)
	viper.SetDefault("delay", 300)
	viper.SetDefault("framerate_limit", 60)
	viper.SetDefault("debug", false)

	viper.AutomaticEnv() // read environment variables that match

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	err := viper.ReadInConfig()
	cobra.CheckErr(err)
}
