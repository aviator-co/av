# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`av` is a command-line tool for managing stacked PRs on GitHub. It allows developers to create dependent pull requests and automatically manages rebasing when base PRs are updated.

## Documentation Sources

This project maintains documentation in three equally important locations:

- **README.md**: Repository overview, quick start guide, and installation instructions in the root directory
- **https://docs.aviator.co/aviator-cli**: Comprehensive online documentation covering all features and workflows
- **Man pages**: Detailed command reference documentation in markdown format within the `docs/` directory, converted to man page format

## Command Structure Philosophy

The CLI follows a flat command structure where commands are added as root-level commands rather than nested subcommands:

- **Preferred**: Root commands with flags (e.g., `av commit`, `av restack`)
- **Historical**: The project has migrated away from layered commands (e.g., `av stack commit` → `av commit`)
- **Guidance for new commands**: Add new functionality as root commands in `cmd/av/`, not as subcommands

Some legacy subcommands remain (e.g., `av stack foreach`, `av pr status`) but new features should follow the flat structure.

## Build and Development Commands

### Running the CLI

```bash
go run ./cmd/av [subcommand/flags...]
```

### Building

```bash
go build -v ./...
```

### Testing

```bash
# Run all tests with verbose output and vet checks
go test -v --vet=all ./...

# Run specific test packages
go test ./internal/git/...
go test ./e2e_tests/...
```

### Linting

The project uses golangci-lint for code quality checks:

```bash
golangci-lint run
```

### CLI Smoke Test

```bash
go run ./cmd/av --help
```

## Architecture Overview

### Core Packages

- **`cmd/av/`**: Main CLI entry point with command implementations using Cobra framework. Each command is typically implemented in its own file.
- **`internal/git/`**: Git operations wrapper providing high-level Git functionality
  - `gittest/`: Test utilities for creating and managing temporary Git repositories in tests
  - `gitui/`: Bubbletea-based interactive UI components for Git operations
- **`internal/gh/`**: GitHub API client using GraphQL for PR management
  - `ghui/`: Bubbletea-based interactive UI components for GitHub operations
- **`internal/avgql/`**: Custom GraphQL client wrapper layer that abstracts GitHub API interactions
- **`internal/meta/`**: Metadata storage system with transaction-based database interface for tracking branch relationships and stack state
  - `jsonfiledb/`: JSON file-based database implementation for persisting metadata
- **`internal/config/`**: Configuration management using Viper for user settings and repository state
- **`internal/actions/`**: High-level business logic for PR operations and workflows, orchestrating git, GitHub, and metadata operations
- **`internal/sequencer/`**: System for planning and executing complex multi-step operations with rollback support
  - `planner/`: Planning logic for determining operation sequences
  - `sequencerui/`: Interactive UI components for displaying and executing sequences
- **`internal/editor/`**: Editor integration for launching and managing external text editors for interactive operations
- **`internal/reorder/`**: Branch reordering functionality for reorganizing stacks
- **`internal/treedetector/`**: Stack and tree structure detection for analyzing branch relationships
- **`internal/utils/`**: Comprehensive utility packages providing common functionality (browser launching, color handling, error utilities, execution helpers, logging, string manipulation, UI helpers, and more)

### Key Architecture Patterns

- **Transaction-based metadata**: Uses `ReadTx`/`WriteTx` pattern for consistent data access
- **Command pattern**: Each CLI command is implemented as a separate file in `cmd/av/`
- **Repository pattern**: Git operations are abstracted through the `Repo` interface
- **Client wrapper**: GitHub API calls are centralized through the `gh.Client`
- **UI component organization**: Interactive Bubbletea components are organized by domain (`gitui/`, `ghui/`, `sequencerui/`) rather than centralized, keeping UI logic close to the operations they represent

### Dependencies

- Built with Go 1.24
- **Cobra**: CLI framework for command structure and flag parsing
- **githubv4**: GitHub GraphQL API client for PR and repository operations
- **go-git**: Git operations library, supplemented with direct git command execution for complex operations
- **Bubbletea**: Terminal UI framework for interactive components and prompts
- **Viper**: Configuration management for loading and persisting user settings
- **Glamour**: Markdown rendering for terminal display
- **Lipgloss**: Terminal styling and layout for consistent UI presentation
- **oauth2**: OAuth authentication flow for GitHub API access

### Key Data Flow

1. CLI commands parse user input and flags
2. Commands use `git.Repo` for Git operations
3. Branch metadata is stored/retrieved via `meta.DB` transactions
4. GitHub operations go through `gh.Client` for PR management
5. Interactive flows use Bubbletea for user prompts

### Testing Structure

- **Unit tests**: Alongside source files (`*_test.go`)
- **E2E tests**: In `e2e_tests/` directory for integration testing with mock GitHub GraphQL server
- **Test utilities**: `internal/git/gittest/` provides test repository setup helpers

### Build Configuration

The project uses several configuration files for build, release, and code quality:

- **`.goreleaser.yaml`**: GoReleaser configuration for building multi-platform release binaries
- **`.golangci.yaml`**: golangci-lint configuration defining enabled linters and code quality rules
- **`.pre-commit-config.yaml`**: Pre-commit hooks for automated checks before commits
- **No Makefile**: All build and test operations use `go` commands directly (see Build and Development Commands section)

### CI/CD

GitHub Actions workflows in `.github/workflows/`:

- **`go.yml`**: Main CI pipeline (build, test, lint, smoke test)
- **`release.yml`**: Automated release builds and distribution
- **`pre-commit.yml`**: Pre-commit hook validation

The codebase emphasizes transaction safety for metadata operations and provides extensive Git workflow automation for stacked PR management.
