# av-stack-orphan

## NAME

av-stack-orphan - Orphan branches that are managed by `av`

## SYNOPSIS

```synopsis
av stack orphan
```

## DESCRIPTION

`av stack orphan` is a command to orphan branches that are managed by `av`.
This is an opposite command to `av stack adopt`.

When you run this command, the current and child branches are no longer managed
by `av`. The branches are still kept in the repository, but `av` will not keep
track of them.
