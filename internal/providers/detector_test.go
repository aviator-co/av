package providers

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDetectProviderFromURL(t *testing.T) {
	tests := []struct {
		name        string
		urlString   string
		repoSlug    string
		expected    Provider
		expectedURL string
	}{
		{
			name:        "GitHub.com HTTPS",
			urlString:   "https://github.com/owner/repo.git",
			repoSlug:    "owner/repo",
			expected:    ProviderGitHub,
			expectedURL: "https://github.com",
		},
		{
			name:        "GitHub.com SSH",
			urlString:   "ssh://git@github.com/owner/repo.git",
			repoSlug:    "owner/repo",
			expected:    ProviderGitHub,
			expectedURL: "ssh://github.com",
		},
		{
			name:        "GitHub Enterprise",
			urlString:   "https://github.company.com/owner/repo.git",
			repoSlug:    "owner/repo",
			expected:    ProviderGitHub,
			expectedURL: "https://github.company.com",
		},
		{
			name:        "GitLab.com HTTPS",
			urlString:   "https://gitlab.com/owner/repo.git",
			repoSlug:    "owner/repo",
			expected:    ProviderGitLab,
			expectedURL: "https://gitlab.com",
		},
		{
			name:        "GitLab.com SSH",
			urlString:   "ssh://git@gitlab.com/owner/repo.git",
			repoSlug:    "owner/repo",
			expected:    ProviderGitLab,
			expectedURL: "ssh://gitlab.com",
		},
		{
			name:        "Self-hosted GitLab",
			urlString:   "https://gitlab.company.com/owner/repo.git",
			repoSlug:    "owner/repo",
			expected:    ProviderGitLab,
			expectedURL: "https://gitlab.company.com",
		},
		{
			name:        "Custom GitLab hostname",
			urlString:   "https://code.company.com/owner/repo.git",
			repoSlug:    "owner/repo",
			expected:    ProviderGitHub, // Default to GitHub for unknown
			expectedURL: "https://code.company.com",
		},
		{
			name:        "GitLab with keyword in hostname",
			urlString:   "https://mygitlab.company.com/owner/repo.git",
			repoSlug:    "owner/repo",
			expected:    ProviderGitLab,
			expectedURL: "https://mygitlab.company.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsedURL, err := url.Parse(tt.urlString)
			require.NoError(t, err, "Failed to parse URL: %s", tt.urlString)

			result, err := DetectProviderFromURL(parsedURL, tt.repoSlug)
			require.NoError(t, err)
			require.Equal(t, tt.expected, result.Provider)
			require.Equal(t, tt.expectedURL, result.BaseURL)
			require.Equal(t, tt.repoSlug, result.RepoSlug)
		})
	}
}

func TestDetectProviderFromURL_Errors(t *testing.T) {
	t.Run("nil URL", func(t *testing.T) {
		_, err := DetectProviderFromURL(nil, "owner/repo")
		require.Error(t, err)
		require.Contains(t, err.Error(), "remote URL is nil")
	})
}

func TestProviderString(t *testing.T) {
	require.Equal(t, "github", ProviderGitHub.String())
	require.Equal(t, "gitlab", ProviderGitLab.String())
}