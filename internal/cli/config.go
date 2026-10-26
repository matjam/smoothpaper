package cli

import (
	"fmt"
	"math"

	"github.com/matjam/smoothpaper/internal/config"
	"github.com/spf13/viper"
)

// InitConfig runs one time at startup. A reload calls viper.ReadInConfig()
// instead: a second InitConfig() call panics on the "toggle" flag
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

	config.SetDefaults()

	viper.AutomaticEnv() // read environment variables that match

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

}

// jsonSafe replaces each NaN or infinite float with its string form. JSON has
// no value for these numbers, so json.Marshal fails on them. A TOML table
// gives a nested map and a TOML array gives a slice, so jsonSafe also looks
// into these.
func jsonSafe(v any) any {
	switch v := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, value := range v {
			out[key] = jsonSafe(value)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, value := range v {
			out[i] = jsonSafe(value)
		}
		return out
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return fmt.Sprint(v)
		}
	}
	return v
}
