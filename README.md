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
$ av branch feature-1
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
$ av branch feature-2
$ echo "Another feature" >> hello.txt
$ git add hello.txt
$ git commit -m "Update hello.txt"
$ av pr create
```

Visualize the PR stack:

```sh
$ av tree
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
$ av sync

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

# Installation

`av` is available for macOS and Linux. In order to interact with GitHub, `av`
uses the GitHub API token. If you have [GitHub CLI](https://cli.github.com/)
installed, `av` will use the token automatically from the GitHub CLI. It is
recommended to install both.

## macOS

```sh
brew install gh aviator-co/tap/av
```

## Arch Linux (AUR)

Published as [`av-cli-bin`](https://aur.archlinux.org/packages/av-cli-bin) in
AUR.

```sh
yay av-cli
```

## Debian/Ubuntu

Add Aviator to your APT repositories.

```sh
echo "deb [trusted=yes] https://apt.fury.io/aviator/ /" > \
/etc/apt/sources.list.d/fury.list
```

And then apt install.

```sh
sudo apt update
sudo apt install av
```

### Alternatively

If you'd prefer you can download the `.deb` file from the [releases page](https://github.com/aviator-co/av/releases).

```sh
apt install ./av_$VERSION_linux_$ARCH.deb
```

## RPM-based systems

Add the following file `/etc/yum.repos.d/fury.repo`.

```conf
[fury]
name=Gemfury Private Repo
baseurl=https://yum.fury.io/aviator/
enabled=1
gpgcheck=0
```

Run the following command to confirm the configuration is working.

```sh
yum --disablerepo=* --enablerepo=fury list available
```

And then run yum install.

```sh
sudo yum install av
```

### Alternatively

If you'd prefer you can download the `.rpm` file from the [releases page](https://github.com/aviator-co/av/releases).

```sh
rpm -i ./av_$VERSION_linux_$ARCH.rpm
```

## Binary download

Download the binary from the [releases page](https://github.com/aviator-co/av/releases).

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

| Command            | Description                                          |
| ------------------ | ---------------------------------------------------- |
| `av branch`        | Create a new child branch from the current branch.   |
| `av commit amend`  | Amend the last commit and rebase the children.       |
| `av commit split`  | Split the last commit.                               |
| `av pr create`     | Create or update a PR.                               |
| `av reorder`       | Reorder the branches.                                |
| `av reparent`      | Change the parent of the current branch.             |
| `av stack adopt`   | Adopt a branch that is not created from `av branch`. |
| `av stack restack` | Rebase the branches to their parents.                |
| `av switch`        | Check out branches interactively.                    |
| `av sync --all`    | Fetch and rebase all branches.                       |
| `av tree`          | Visualize the PRs.                                   |

# How it works

`av` internally keeps tracks of the PRs, their associated branches and their dependent branches.
For each branch, it remembers where the branch started (the base commit of the branch). When
the base branch is updated, `av` rebases the dependent branches on top of the
new base branch using the remembered starting point as the merge base.

# Learn more

- [Rethinking code reviews with stacked PRs](https://www.aviator.co/blog/rethinking-code-reviews-with-stacked-prs/)
- [Issue Tracker](https://github.com/aviator-co/av/issues)
- [Changelog](https://github.com/aviator-co/av/releases)
