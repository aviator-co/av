# av-stack-branch

## NAME

av-stack-branch - Create or rename a branch in the stack

## SYNOPSIS

`av stack branch [-m | --rename] [--force] [--parent <parent_branch>] <branch-name>`

## DESCRIPTION

Create a new branch that is stacked on the current branch by default

If the --rename/-m flag is given, the current branch is renamed to the name
instead of creating a new branch. Branches should only be renamed with this
command (not with `git branch -m ...`) because av needs to update internal
tracking metadata that defines the order of branches within a stack. If you
renamed a branch with `git branch -m`, you can retroactively update the internal
metadata with `av stack branch --rename <old-branch-name>:<new-branch-name>`.

## OPTIONS

`--parent <parent_branch>`
: Instead of creating a new branch from current branch, create it from
  specified `<parent_branch>`

`-m, --rename`
: Rename the current branch to the provided `<branch_name>` instead of
  creating a new one, only if a pull request does not exist.

`--force`
: Force rename the branch, even if a pull request exists.
