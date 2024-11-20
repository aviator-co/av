<div align="center">
  <img src="resources/isotype-color.svg" width="100">
  <p><strong>av:</strong> CLI to manage Stacked PRs</p>
</div>

---

`av` is a command-line tool that helps you manage your stacked PRs on GitHub. It
allows you to create a PR stacked on top of another PR, and it will
automatically update the dependent PR when the base PR is updated. Read more at
[Rethinking code reviews with stacked
PRs](https://www.aviator.co/blog/rethinking-code-reviews-with-stacked-prs/).

# Community
Join our discord community: [https://discord.gg/TFgtZtN8](https://discord.gg/NFsYWNzXcH)

# Features

- Create a PR that is stacked on another PR.
- Visualize the entire stack of PRs.
- Interactively navigate through the PR stack.
- Rebase the dependent PR when the base PR is updated.
- Remove the merged PRs from the stack.
- Split a PR into multiple PRs.
- Split a commit into multiple commits.
- Reorder the PRs and commits in the stack.

# Demo

![Demo](resources/demo.gif)

# Usage

> [!TIP]
> Complete documentation is available at [docs.aviator.co](https://docs.aviator.co/aviator-cli).

Create a new branch and make some changes:

```sh
$ av stack branch feature-1
$ echo "Hello, world!" > hello.txt
$ git add hello.txt
$ git commit -m "Add hello.txt"
```

Create a PR:

```sh
$ av pr create
```

Create a new branch and make some changes. Create another PR that depends on the
previous PR:

```sh
$ av stack branch feature-2
$ echo "Another feature" >> hello.txt
$ git add hello.txt
$ git commit -m "Update hello.txt"
$ av pr create
```

Visualize the PR stack:

```sh
$ av stack tree
  * feature-2 (HEAD)
  │ https://github.com/octocat/Hello-World/pull/2
  │
  * feature-1
  │ https://github.com/octocat/Hello-World/pull/1
  │
  * master
```

Merge the first PR:

```sh
$ gh pr merge feature-1
```

Sync the stack:

```sh
$ av stack sync

  ✓ GitHub fetch is done
  ✓ Restack is done

    * ✓ feature-2 f9d85fe
    │
    * master 7fd1a60

  ✓ Pushed to GitHub

    Following branches do not need a push.

      feature-1: PR is already merged.

    Following branches are pushed.

      feature-2
        Remote: dbae4bd Update hello.txt 2024-06-11 16:41:18 -0700 -0700 (2 minutes ago)
        Local:  f9d85fe Update hello.txt 2024-06-11 16:43:41 -0700 -0700 (7 seconds ago)
        PR:     https://github.com/octocat/Hello-World/pull/2

  ✓ Deleted the merged branches

    Following merged branches are deleted.

      feature-1: f2335eec783b54226a7ab90f4af1c9b8309f8b61

```

# Installation & Upgrade

`av` is available for macOS and Linux. You can install and upgrade it using the following methods:

## macOS

Install via Homebrew:
```sh
brew install gh aviator-co/tap/av
```

Upgrade:
```sh
brew upgrade av
```

## Arch Linux (AUR)

Install via AUR (published as [`av-cli-bin`](https://aur.archlinux.org/packages/av-cli-bin)):
```sh
yay -S av-cli-bin
```

Upgrade:
```sh
yay -S av-cli-bin
```

## Debian/Ubuntu

Download the `.deb` file from the [releases page](https://github.com/aviator-co/av/releases):
```sh
# Install
sudo dpkg -i ./av_$VERSION_linux_$ARCH.deb

# Upgrade
av upgrade   # or use dpkg -i with the new version
```

## RPM-based systems

Download the `.rpm` file from the [releases page](https://github.com/aviator-co/av/releases):
```sh
# Install
sudo rpm -i ./av_$VERSION_linux_$ARCH.rpm

# Upgrade
av upgrade   # or use rpm -U with the new version
```

## Binary installation

1. Download the binary for your system from the [releases page](https://github.com/aviator-co/av/releases)
2. Extract and install the binary:
```sh
# Download and install
curl -L -o av.tar.gz "https://github.com/aviator-co/av/releases/latest/download/av_$(uname -s | tr '[:upper:]' '[:lower:]')_$(uname -m).tar.gz"
sudo tar xzf av.tar.gz -C /usr/local/bin

# Upgrade
av upgrade   # or repeat the installation steps with the new version
```

## Automatic upgrades

Once installed, you can upgrade `av` using the built-in upgrade command:
```sh
av upgrade
```

This command will automatically detect how `av` was installed and perform the appropriate upgrade.

# Setup

1. Set up the GitHub CLI for GitHub authentication:

   ```sh
   gh auth login
   ```

   Or you can create a Personal Access Token as described in the
   [Configuration](https://docs.aviator.co/aviator-cli/configuration#github-personal-access-token)
   section.

2. Set up the `av` CLI autocompletion:

   ```sh
   # Bash
   source <(av completion bash)
   # Zsh
   source <(av completion zsh)
   ```

3. Initialize the repository:

   ```sh
   av init
   ```

# Example commands

| Command               | Description                                                |
| --------------------- | ---------------------------------------------------------- |
| `av stack branch`     | Create a new child branch from the current branch.         |
| `av stack restack`    | Rebase the branches to their parents.                      |
| `av pr create`        | Create or update a PR.                                     |
| `av stack tree`       | Visualize the PRs.                                         |
| `av stack sync --all` | Fetch and rebase all branches.                             |
| `av stack adopt`      | Adopt a branch that is not created from `av stack branch`. |
| `av stack reparent`   | Change the parent of the current branch.                   |
| `av stack switch`     | Check out branches interactively.                          |
| `av stack reorder`    | Reorder the branches.                                      |
| `av commit amend`     | Amend the last commit and rebase the children.             |
| `av commit split`     | Split the last commit.                                     |

# How it works

`av` internally keeps tracks of the PRs, their associated branches and their dependent branches.
For each branch, it remembers where the branch started (the base commit of the branch). When
the base branch is updated, `av` rebases the dependent branches on top of the
new base branch using the remembered starting point as the merge base.

# Learn more

* [Rethinking code reviews with stacked PRs](https://www.aviator.co/blog/rethinking-code-reviews-with-stacked-prs/)
* [Issue Tracker](https://github.com/aviator-co/av/issues)
* [Changelog](https://github.com/aviator-co/av/releases)
