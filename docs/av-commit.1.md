# av-create

## NAME

av-create - Record changes to the repository with commits

## SYNOPSIS

```synopsis
av commit [-m <msg>| --message=<msg>] [-a | --all] [--amend] [--edit]
    [-b | -b <branch_name> | --branch-name <branch_name>] [-A | --all-changes]
```

## DESCRIPTION

Create a new commit containing the current contents of the index and the given
log message describing the changes, then run **av stack restack** on all
subsequent child branches with the new commit.

Previous to running **av commit**, add changes to the index via
git-add(1) to incrementally "add" changes to the index.

## OPTIONS

`-m <msg>, --message=<msg>`
: Use the given `<msg>` as the commit message.

`-a, --all`
: Automatically stage modified/deleted files, but new files you have not told
  Git about are not affected. (Same as git commit --all)

`--amend`
: Amend the last commit, using the same message as last commit by default

`--edit`
: When amending a commit, open the default git `$EDITOR` for modifying the
  commit message

`-b, -b <branch_name>, --branch-name <branch_name>`
: The branch name to create. If empty, automatically generate from the message.
