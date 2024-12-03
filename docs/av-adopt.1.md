# av-adopt

## NAME

av-adopt - Adopt branches that are not managed by `av`

## SYNOPSIS

```synopsis
av adopt [--parent=<parent>]
```

## DESCRIPTION

`av adopt` is a command to adopt a branch or an entire stack that is not
managed by `av`. This command is useful when you have branches that are created
outside of `av` and want to manage them with `av`.

When you run this command, it looks into the commits to figure out the branch
structure in the following way:

* For all non-adopted branches, find the nearest `av` managed branch, other
  non-adopted branch, or the commits that are included by the trunk branch.
* If a merge commit exists while finding the nearest branch, the command will
  not adopt that branch and its children.

After this process, it prompts you to choose which branch to adopt. By default,
it adopts all the branches it finds. If you want to adopt only a specific
branch, you can unspecify the branches you don't want to adopt.

If a branch contains a merge commit or if there are multiple possible parents,
the command will exit with an error. In this case, you need to manually rebase
branches so that they form a tree structure.

## ADOPTING A SINGLE BRANCH

If you want to adopt the current branch and specify the parent branch, you can
do so by running the command with `--parent`.

## OPTIONS

`--parent=<parent>`
: Force specify the parent branch.

## SEE ALSO

`av-orphan`(1) for orphaning a branch.
