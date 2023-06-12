# av-stack-branch-commit

## NAME

av-stack-branch-commit - Create a new branch in the stack with the staged changes

## SYNOPSIS

```synopsis
av stack branch-commit [-a | --all] [-b <branch_name> | --branch-name <branch_name>]
    [-m <msg>| --message=<msg>]`
```

## DESCRIPTION

Create a new branch that is stacked on the current branch and commit all
staged changes with the specified arguments. One of `-b` or `-m` flags are
required to create the branch.

## OPTIONS

`-a, --all`
: Automatically stage all modified/deleted files, (same as `git commit --all`)

`-b <branch_name>, --branch-name <branch_name>`
: The branch name to create. If empty, automatically generate from the message.

`-m <msg>, --message=<msg>`
: Use the given `<msg>` as the commit message.
