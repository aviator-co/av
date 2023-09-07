# Aviator Command Line Tool
Aviator CLI is an open-source CLI tool to create, update, review and merge stacked PRs on GitHub.

## What are Stacked PRs
Stacked pull requests make smaller, iterative changes and are stacked on top of each other instead of bundling large monolith changes in a single pull request. Each PR in the stack focuses on one logical change only, making the review process more manageable and less time-consuming.

Read more about stacked PRs in our blog: [Rethinking code reviews with stacked PRs](https://www.aviator.co/blog/rethinking-code-reviews-with-stacked-prs/).

# Installing the CLI

### For Mac
```
brew install aviator-co/tap/av
```

### For other platforms
Download the latest av executable from the GitHub releases page on the av repository. Extract the archive and add the executable to your PATH.


See rest of the instructions on
[Aviator Stacked PRs quickstart](https://docs.aviator.co/aviator-cli).

# Development setup

Install the latest version of Go from https://go.dev/doc/install.

To run the command line:

```
go run ./cmd/av [subcommand/flags...]
```

# Release

To create a release, create a tag with the desired version and push to GitHub.

```
# Change the version as appropriate
TAG="v0.0.0"

git tag "$TAG"
git push origin tags/"$TAG"
```

This will automatically trigger [Goreleaser](https://goreleaser.com/) (as part
of the
[`release.yml` workflow](https://github.com/aviator-co/av/blob/master/.github/workflows/release.yml))
which will create a GitHub release and build and publish binaries to Homebrew
and Scoop.
