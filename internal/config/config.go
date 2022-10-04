package config

import (
	"emperror.dev/errors"
	"github.com/spf13/viper"
	"os"
)

type GitHub struct {
	Token   string
	BaseUrl string
}

type PullRequest struct {
	Draft       bool
	OpenBrowser bool
	// If true, the pull request will be converted to a draft if the base branch
	// needs to be changed after the pull request has been changed. This avoids
	// accidentally adding lots of unnecessary auto-added reviewers (via GitHub's
	// CODEOWNERS feature) to the pull request while the PR is in a transient
	// state.
	// If not set, the value should be considered true iff there is a CODEOWNERS
	// file in the repository.
	RebaseWithDraft *bool
}

var Av = struct {
	PullRequest PullRequest
	GitHub      GitHub
}{
	PullRequest: PullRequest{
		OpenBrowser: true,
	},
	GitHub: GitHub{
		BaseUrl: "https://github.com",
	},
}

// Load initializes the configuration values.
// It may optionally be called with a list of additional paths to check for the
// config file.
// Returns a boolean indicating whether or not a config file was loaded and an
// error if one occurred.
func Load(paths []string) (bool, error) {
	loaded, err := loadFromFile(paths)
	loadFromEnv()
	return loaded, err
}

func loadFromFile(paths []string) (bool, error) {
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

	if err := config.Unmarshal(&Av); err != nil {
		return true, errors.Wrap(err, "failed to read av configs")
	}

	return false, nil
}

func loadFromEnv() {
	// TODO: integrate this better with cobra/viper/whatever
	if githubToken := os.Getenv("AV_GITHUB_TOKEN"); githubToken != "" {
		Av.GitHub.Token = githubToken
	} else if githubToken := os.Getenv("GITHUB_TOKEN"); githubToken != "" {
		Av.GitHub.Token = githubToken
	}
}
