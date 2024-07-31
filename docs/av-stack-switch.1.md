# av-stack-switch

## NAME

av-stack-switch - Interactively switch to a different branch

## SYNOPSIS

```synopsis
av stack switch [<branch> | <url>]
```

## DESCRIPTION

If no branch or URL is provided, this command will show a list of branches and
you can interactively switch to a different branch. This command shows only
branches adopted using av-cli.

If a branch is provided, this command will switch to the provided branch. This
is the same as running `git switch <branch>`.

If a pull request URL is provided, this command will switch to the branch that
is corresponding to the pull request.
