package config

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"slices"
	"time"

	"github.com/matjam/smoothpaper/internal/types"
	"github.com/spf13/viper"
)

type Config struct {
	Directories    []string          `mapstructure:"wallpapers"`
	Shuffle        bool              `mapstructure:"shuffle"`
	ScaleMode      types.ScalingMode `mapstructure:"scale_mode"`
	FadeSpeed      float64           `mapstructure:"fade_speed"`
	Easing         types.EasingMode  `mapstructure:"easing"`
	Delay          int               `mapstructure:"delay"`
	FramerateLimit int               `mapstructure:"framerate_limit"`
	Debug          bool              `mapstructure:"debug"`
}

// maxSeconds is the largest number of whole seconds that a time.Duration can
// hold. A larger delay or fade_speed overflows in Run() or in FadeDuration().
const maxSeconds = math.MaxInt64 / int64(time.Second)

// Default returns the value of each key that the config file does not set.
func Default() *Config {
	return &Config{
		Directories:    []string{"~/Pictures/wallpapers"},
		Shuffle:        true,
		ScaleMode:      types.ScalingModeFitVertical,
		FadeSpeed:      1.0,
		Easing:         types.EasingEaseInOut,
		Delay:          300,
		FramerateLimit: 60,
		Debug:          false,
	}
}

// SetDefaults gives the defaults to viper, so --show-config also shows the
// keys that the config file does not set.
func SetDefaults() {
	d := Default()
	viper.SetDefault("wallpapers", d.Directories)
	viper.SetDefault("shuffle", d.Shuffle)
	viper.SetDefault("scale_mode", d.ScaleMode)
	viper.SetDefault("easing", d.Easing)
	viper.SetDefault("fade_speed", d.FadeSpeed)
	viper.SetDefault("delay", d.Delay)
	viper.SetDefault("framerate_limit", d.FramerateLimit)
	viper.SetDefault("debug", d.Debug)
}

// Load decodes and checks the config. Any invalid value is an error, so a
// reload with a broken file changes nothing. The error lists all invalid values.
func Load() (*Config, error) {
	cfg, problems := load()
	if err := errors.Join(problems...); err != nil {
		return nil, err
	}
	return cfg, nil
}

// LoadWithDefaults decodes and checks the config like Load(), but it uses the
// default for each invalid value. It returns one problem for each of these
// values. Used at startup, because there is no old config to keep.
func LoadWithDefaults() (*Config, []error) {
	return load()
}

// load decodes each key on its own, so an invalid value replaces only its own
// key with the default.
func load() (*Config, []error) {
	cfg := Default()
	problems := []error{
		decodeKey("wallpapers", &cfg.Directories, func(dirs []string) error {
			if len(dirs) == 0 {
				return errors.New("wallpapers must contain at least one directory")
			}
			return nil
		}),
		decodeKey("shuffle", &cfg.Shuffle, nil),
		decodeKey("scale_mode", &cfg.ScaleMode, func(mode types.ScalingMode) error {
			if !mode.Valid() {
				return fmt.Errorf("scale_mode must be %s, %s, %s or %s, got %q",
					types.ScalingModeCenter, types.ScalingModeStretch, types.ScalingModeFitHorizontal, types.ScalingModeFitVertical, mode)
			}
			return nil
		}),
		decodeKey("fade_speed", &cfg.FadeSpeed, func(speed float64) error {
			if math.IsNaN(speed) || math.IsInf(speed, 0) {
				return fmt.Errorf("fade_speed must be a finite number, got %v", speed)
			}
			if speed < 0 {
				return fmt.Errorf("fade_speed must be 0 or greater, got %v", speed)
			}
			if speed > float64(maxSeconds) {
				return fmt.Errorf("fade_speed must be %d or less, got %v", maxSeconds, speed)
			}
			return nil
		}),
		decodeKey("easing", &cfg.Easing, func(easing types.EasingMode) error {
			if !easing.Valid() {
				return fmt.Errorf("easing must be %s, %s, %s or %s, got %q",
					types.EasingLinear, types.EasingEaseIn, types.EasingEaseOut, types.EasingEaseInOut, easing)
			}
			return nil
		}),
		decodeKey("delay", &cfg.Delay, func(delay int) error {
			if delay < 0 {
				return fmt.Errorf("delay must be 0 or greater, got %d", delay)
			}
			if int64(delay) > maxSeconds {
				return fmt.Errorf("delay must be %d or less, got %d", maxSeconds, delay)
			}
			return nil
		}),
		decodeKey("framerate_limit", &cfg.FramerateLimit, func(framerate int) error {
			if framerate <= 0 {
				return fmt.Errorf("framerate_limit must be greater than 0, got %d", framerate)
			}
			return nil
		}),
		decodeKey("debug", &cfg.Debug, nil),
	}
	return cfg, slices.DeleteFunc(problems, func(err error) bool { return err == nil })
}

// decodeKey decodes the value of key into dst and checks it. On an error, dst
// keeps its value, which is the default. A key without a value also keeps the
// default.
func decodeKey[T any](key string, dst *T, check func(T) error) error {
	if !viper.IsSet(key) {
		return nil
	}

	var value T
	if err := viper.UnmarshalKey(key, &value, viper.DecodeHook(stringToSingleItemSlice)); err != nil {
		return fmt.Errorf("%s must be %s, got %q", key, typeName(value), fmt.Sprint(viper.Get(key)))
	}
	if check != nil {
		if err := check(value); err != nil {
			return err
		}
	}

	*dst = value
	return nil
}

// helper for formatting error messages
func typeName(v any) string {
	switch v.(type) {
	case bool:
		return "true or false"
	case int:
		return "a whole number"
	case float64:
		return "a number"
	case []string:
		return "a string or a list of strings"
	default:
		return "a string"
	}
}

// stringToSingleItemSlice lets a key hold one string or a list of strings.
// Viper's default hook splits a string on commas, which breaks a path that
// contains a comma. Weird edge case, but technically a valid name for a directory
func stringToSingleItemSlice(from, to reflect.Type, data any) (any, error) {
	s, ok := data.(string)
	if !ok || from.Kind() != reflect.String || to.Kind() != reflect.Slice {
		return data, nil
	}
	if s == "" {
		return []string{}, nil
	}
	return []string{s}, nil
}

// FadeDuration converts FadeSpeed from seconds to a time.Duration. Multiply
// before the conversion, because time.Duration(f) truncates 1.5 to 1 nanosecond.
func (c *Config) FadeDuration() time.Duration {
	return time.Duration(c.FadeSpeed * float64(time.Second))
}
