# av-create

## NAME

av-commit - Record changes to the repository with commits

## SYNOPSIS

```synopsis
av commit [-m <msg>| --message=<msg>] [-a | --all] [--amend] [--edit]
    [-b | --branch] [-A | --all-changes] [--branch-name <name>]
    [--parent <parent_branch>]
```

## DESCRIPTION

Create a new commit containing the current contents of the index and the given
log message describing the changes, then run **av restack** on all
subsequent child branches with the new commit.

Previous to running **av commit**, add changes to the index via
git-add(1) to incrementally "add" changes to the index.

## OPTIONS

`-m <msg>, --message=<msg>`
: Use the given `<msg>` as the commit message.

`-a, --all`
: Automatically stage modified/deleted files, but new files you have not told
  Git about are not affected. (Same as git commit --all)

`-A, --all-changes`
: Automatically stage all files, including untracked files

`--amend`
: Amend the last commit, using the same message as last commit by default

`--edit`
: When amending a commit, open the default `$GIT_EDITOR` for modifying the
  commit message

`-b, --branch`
: Create a new branch with an automatically generated name and commit to it

`--branch-name <name>`
: Create a new branch with the given name and commit to it

`--parent <parent_branch>`
: Instead of creating a new branch from current branch, create it from
  specified `<parent_branch>`
