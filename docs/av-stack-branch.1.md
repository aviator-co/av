# av-stack-branch 1 "" av-cli "Aviator CLI User Manual"

# NAME

av-stack-branch - Create or rename a branch in the stack

# SYNOPSIS

`av stack branch [-m | --rename] [--parent <parent_branch>] <branch-name>`

# DESCRIPTION

Create a new branch that is stacked on the current branch by default

If the --rename/-m flag is given, the current branch is renamed to the name
instead of creating a new branch. Branches should only be renamed
with this command (not with git branch -m ...) because av needs to update
internal tracking metadata that defines the order of branches within a stack.

# OPTIONS

`--parent <parent_branch>`
: Instead of creating a new branch from current branch, create it from
  specified `<parent_branch>`

`-m, --rename`
: Rename the current branch to the provided `<branch_name>` instead of
  creating a new one.
