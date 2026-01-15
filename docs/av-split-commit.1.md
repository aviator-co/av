# av-split-commit

## NAME

av-split-commit - Split a commit into multiple commits

## SYNOPSIS

```synopsis
av split-commit
```

## DESCRIPTION

Split the currently checked out commit into multiple commits. When invoked, it
prompts you which diff chunks should be included in the first commit and asks
you for the commit message. The process repeats until all diff chunks are
distributed to the commits.
