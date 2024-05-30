# av-stack-reparent

## NAME

av-stack-reparent - Reparent a branch to a new parent

## SYNOPSIS

```synopsis
av stack reparent [--parent=<parent>]
```

## DESCRIPTION

This rebases the current branch onto the new parent and runs the restack
operations on the children.

## OPTIONS

`--parent=<parent>`
: Parent branch to rebase onto.
