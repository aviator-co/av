# av-tidy

## NAME

av-tidy - Tidy stacked branches

## SYNOPSIS

```synopsis
av tidy
```

## DESCRIPTION

Tidy stacked branches by removing deleted or merged branches.

This command detects which branches are deleted or merged and re-parents
children of merged branches. This operates on only av's internal metadata and
does not delete Git branches.
