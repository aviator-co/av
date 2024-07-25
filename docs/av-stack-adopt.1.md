# av-stack-adopt

## NAME

av-stack-adopt - Adopt branches that are not managed by `av`

## SYNOPSIS

```synopsis
av stack adopt [--parent=<parent>]
```

## DESCRIPTION

`av stack adopt` is a command to adopt a branch or an entire stack that is not
managed by `av`. This command is useful when you have branches that are created
outside of `av` and want to manage them with `av`.

When you run this command, it lists the branches that are not managed by `av`,
automatically detecting the tree structure of the branches. You can choose which
branch to adopt.

If a branch contains a merge commit or if there are multiple possible parents,
the command will exit with an error. In this case, you need to manually rebase
branches so that they form a tree structure.

## OPTIONS

`--parent=<parent>`
: Force specify the parent branch.
