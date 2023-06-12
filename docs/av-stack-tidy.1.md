# av-stack-tidy

## NAME

av-stack-tidy - Tidy stacked branches by removing deleted or merged branches

## SYNOPSIS

```synopsis
av stack tidy
```

## DESCRIPTION

Tidy stacked branches by removing deleted or merged branches.

This command detects which branches are deleted or merged and re-parents
children of merged branches. This operates on only av's internal metadata and
does not delete Git branches.
