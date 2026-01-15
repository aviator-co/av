# av-reorder

## NAME

av-reorder - Interactively reorder the stack

## SYNOPSIS

```synopsis
av reorder [--continue | --abort]
```

## DESCRIPTION

This is analogous to git rebase --interactive but operates across all branches
in the stack.

Branches can be re-arranged within the stack and commits can be edited,
squashed, dropped, or moved within the stack, even across the branches.

## OPTIONS

`--continue`
: Continue an in-progress reorder.

`--abort`
: Abort an in-progress reorder.
