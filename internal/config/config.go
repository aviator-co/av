package config

import (
	"emperror.dev/errors"
	"github.com/spf13/viper"
)

var GitHub = struct {
	Token   string
	BaseUrl string
}{
	BaseUrl: "https://github.com",
}

// Load initializes the configuration values.
// It may optionally be called with a list of additional paths to check for the
// config file.
// Returns a boolean indicating whether or not a config file was loaded and an
// error if one occurred.
func Load(paths []string) (bool, error) {
	config := viper.New()

	// Viper has support for various formats, so it supports kson, toml, yaml,
	// and more (https://github.com/spf13/viper#reading-config-files).
	config.SetConfigName("config")

	// Reasonable places to look for config files.
	config.AddConfigPath("$XDG_CONFIG_HOME/av")
	config.AddConfigPath("$HOME/.config/av")
	config.AddConfigPath("$HOME/.av")
	config.AddConfigPath("$AV_HOME")
	// Add additional custom paths.
	// The primary use case for this is adding repository-specific
	// configuration (e.g., $REPO/.git/av/config.json).
	for _, path := range paths {
		config.AddConfigPath(path)
	}

	if err := config.ReadInConfig(); err != nil {
		if errors.As(err, &viper.ConfigFileNotFoundError{}) {
			return false, nil
		}
		return false, err
	}

	if err := config.UnmarshalKey("github", &GitHub); err != nil {
		return true, errors.Wrap(err, "failed to read github config")
	}

	return true, nil
}
