package config

import (
	"os"
	"path/filepath"

	"emperror.dev/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type GitHub struct {
	// The GitHub API token to use for authenticating to the GitHub API.
	Token string
	// The base URL of the GitHub instance to use.
	// This should only be set for GitHub Enterprise Server (GHES) instances.
	// For example, "https://github.mycompany.com/" (without a "/api/v3" or
	// "/api/graphql" suffix).
	BaseURL string
}

type GitLab struct {
	// The GitLab API token to use for authenticating to the GitLab API.
	Token string
	// The base URL of the GitLab instance to use.
	// For example, "https://gitlab.com" or "https://gitlab.mycompany.com"
	// (without a "/api/graphql" suffix).
	BaseURL string
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

	// By default, when the pull request title contains "WIP", it automatically sets the PR as
	// a draft PR. Setting this to true suppresses this behavior.
	NoWIPDetection bool

	// Branch prefix to use for creating new branches.
	BranchNamePrefix string

	// If true, the CLI will automatically add/update a comment to all PRs linking other PRs in the stack.
	// False by default, since Aviator's MergeQueue also adds a similar comment.
	WriteStack bool
}

type Aviator struct {
	// The base URL of the Aviator API to use.
	// By default, this is https://aviator.co, but for on-prem installations
	// this can be changed (e.g., https://aviator.mycompany.com).
	// It should not include a trailing slash or any path components.
	APIHost string
	// The API token to use for authenticating to the Aviator API.
	APIToken string
}

var Av = struct {
	PullRequest             PullRequest
	GitHub                  GitHub
	GitLab                  GitLab
	Aviator                 Aviator
	AdditionalTrunkBranches []string
	Remote                  string
}{
	Aviator: Aviator{
		APIHost: "https://api.aviator.co",
	},
	PullRequest: PullRequest{
		OpenBrowser: true,
	},
	GitHub:                  GitHub{},
	GitLab: GitLab{
		BaseURL: "https://gitlab.com",
	},
	AdditionalTrunkBranches: []string{},
	Remote:                  "",
}

// Load initializes the configuration values.
//
// This takes an optional repository config directory, which, when exists, overrides the default
// config.
func Load(repoConfigDir string) error {
	if err := loadFromFile(repoConfigDir); err != nil {
		return err
	}
	if err := loadFromEnv(); err != nil {
		return err
	}
	return nil
}

func loadFromFile(repoConfigDir string) error {
	config := viper.New()
	// The base filename of the config files.
	config.SetConfigName("config")
	// With config.ReadInConfig, Viper looks for a file with `config.$EXT` where $EXT is
	// viper.SupportedExts. It tries to find the file in the following directories in this
	// order (e.g. $XDG_CONFIG_HOME/av/config.yaml first).
	//
	// Note that Viper will find only one file in these directories, so if there are multiple,
	// only one is read.
	config.AddConfigPath("$XDG_CONFIG_HOME/av")
	config.AddConfigPath("$HOME/.config/av")
	config.AddConfigPath("$HOME/.av")
	config.AddConfigPath("$AV_HOME")
	if err := config.ReadInConfig(); err != nil {
		// We can ignore config file not exist case.
		if !errors.As(err, &viper.ConfigFileNotFoundError{}) {
			return err
		}
	} else {
		logrus.WithField("config_file", config.ConfigFileUsed()).Debug("loaded config file")
	}

	// As stated above, Viper will read only one file from the above paths. However, we want to
	// support per-repo configuration that overrides the global configuration. Here, we mimic
	// the behavior of Viper by looking for the per-repo config file and merge it.
	for _, ext := range viper.SupportedExts {
		fp := filepath.Join(repoConfigDir, "config."+ext)
		if stat, err := os.Stat(fp); err == nil {
			if !stat.IsDir() {
				config.SetConfigFile(fp)
				config.SetConfigType(ext)
				if err := config.MergeInConfig(); err != nil {
					return errors.Wrapf(err, "failed to read %s", fp)
				}
				logrus.WithField("config_file", fp).Debug("loaded config file")
				break
			}
		}
	}

	if err := config.Unmarshal(&Av); err != nil {
		return errors.Wrap(err, "failed to read av configs")
	}
	return nil
}

func loadFromEnv() error {
	// TODO: integrate this better with cobra/viper/whatever
	if githubToken := os.Getenv("AV_GITHUB_TOKEN"); githubToken != "" {
		Av.GitHub.Token = githubToken
	} else if githubToken := os.Getenv("GITHUB_TOKEN"); githubToken != "" {
		Av.GitHub.Token = githubToken
	}

	if gitlabToken := os.Getenv("AV_GITLAB_TOKEN"); gitlabToken != "" {
		Av.GitLab.Token = gitlabToken
	} else if gitlabToken := os.Getenv("GITLAB_TOKEN"); gitlabToken != "" {
		Av.GitLab.Token = gitlabToken
	}

	if apiToken := os.Getenv("AV_API_TOKEN"); apiToken != "" {
		Av.Aviator.APIToken = apiToken
	}
	if apiHost := os.Getenv("AV_API_HOST"); apiHost != "" {
		Av.Aviator.APIHost = apiHost
	}

	return nil
}
