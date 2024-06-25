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
