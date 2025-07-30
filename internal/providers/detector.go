package providers

import (
	"context"
	"net/url"
	"strings"

	"emperror.dev/errors"
	"github.com/aviator-co/av/internal/git"
)

// Provider represents the Git hosting provider type.
type Provider string

const (
	ProviderGitHub Provider = "github"
	ProviderGitLab Provider = "gitlab"
)

// String returns the string representation of the provider.
func (p Provider) String() string {
	return string(p)
}

// DetectionResult contains information about the detected provider.
type DetectionResult struct {
	Provider Provider
	BaseURL  string // Base URL for the provider (e.g., "https://gitlab.example.com")
	RepoSlug string // Repository slug (e.g., "owner/repo")
}

// DetectProvider determines the Git hosting provider for the given repository.
func DetectProvider(ctx context.Context, repo *git.Repo) (*DetectionResult, error) {
	origin, err := repo.Origin(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get repository origin")
	}

	return DetectProviderFromURL(origin.URL, origin.RepoSlug)
}

// DetectProviderFromURL determines the provider from a Git remote URL.
func DetectProviderFromURL(remoteURL *url.URL, repoSlug string) (*DetectionResult, error) {
	if remoteURL == nil {
		return nil, errors.New("remote URL is nil")
	}

	hostname := strings.ToLower(remoteURL.Hostname())
	
	// Detect GitHub
	if hostname == "github.com" || strings.HasSuffix(hostname, ".github.com") {
		return &DetectionResult{
			Provider: ProviderGitHub,
			BaseURL:  getBaseURL(remoteURL),
			RepoSlug: repoSlug,
		}, nil
	}

	// Detect GitLab (both GitLab.com and self-hosted)
	if hostname == "gitlab.com" || strings.HasSuffix(hostname, ".gitlab.com") {
		return &DetectionResult{
			Provider: ProviderGitLab,
			BaseURL:  getBaseURL(remoteURL),
			RepoSlug: repoSlug,
		}, nil
	}

	// For other hosts, we need to make a best guess
	// Check common GitLab patterns in hostname
	if strings.Contains(hostname, "gitlab") {
		return &DetectionResult{
			Provider: ProviderGitLab,
			BaseURL:  getBaseURL(remoteURL),
			RepoSlug: repoSlug,
		}, nil
	}

	// Default to GitHub for unknown providers
	// This maintains backward compatibility with existing behavior
	return &DetectionResult{
		Provider: ProviderGitHub,
		BaseURL:  getBaseURL(remoteURL),
		RepoSlug: repoSlug,
	}, nil
}

// getBaseURL extracts the base URL from a Git remote URL.
func getBaseURL(remoteURL *url.URL) string {
	if remoteURL.Scheme == "" {
		// Handle SSH URLs like git@github.com:owner/repo.git
		return "https://" + remoteURL.Hostname()
	}
	return remoteURL.Scheme + "://" + remoteURL.Host
}