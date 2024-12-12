# av-orphan

## NAME

av-orphan - Orphan branches that are managed by `av`

## SYNOPSIS

```synopsis
av orphan
```

## DESCRIPTION

`av orphan` is a command to orphan branches that are managed by `av`.
This is an opposite command to `av adopt`.

When you run this command, the current and child branches are no longer managed
by `av`. The branches are still kept in the repository, but `av` will not keep
track of them.

## SEE ALSO

`av-adopt`(1) for adopting a new branch.
