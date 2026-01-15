# av-squash

## NAME

av-squash - Squash commits of the current branch into a single commit

## SYNOPSIS

```synopsis
av squash
```

## DESCRIPTION

Squash all commits on the current branch into a single commit. This command
performs a soft reset to the first commit on the branch (relative to the parent
branch) and then amends that commit with all subsequent changes, effectively
combining all commits into one.

This command requires a clean working directory and will not work on branches
that have already been merged. The branch must have at least two commits to
squash.

After squashing, **av squash** automatically runs **av restack** to rebase
any child branches on the newly squashed commit.

## EXAMPLES

Squash all commits on the current branch:

```bash
$ av squash
```

## SEE ALSO

`av-commit`(1), `av-restack`(1)
