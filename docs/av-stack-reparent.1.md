# av-stack-reparent

## NAME

av-stack-reparent - Reparent a branch to a new parent

## SYNOPSIS

```synopsis
av stack sync [--continue | --abort | --skip | --parent=<parent>]
```

## DESCRIPTION

This rebases the current branch onto the new parent and runs the restack
operations on the children.

## OPTIONS

`--continue`
: Continue an in-progress rebase.

`--abort`
: Abort an in-progress rebase.

`--skip`
: Skip the current commit and continue an in-progress rebase.

`--parent=<parent>`
: Parent branch to rebase onto.
