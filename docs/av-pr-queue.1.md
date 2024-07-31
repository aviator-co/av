# av-pr-queue

## NAME

av-pr-queue - Queue an existing pull request for the current branch

## SYNOPSIS

```synopsis
av pr queue [--skip-line] [--targets=<targets>]
```

## DESCRIPTION

Attempt to add an existing pull request for the current branch to the queue.
If the current branch does not have an open pull request it will need to be
created first. `av pr create` can accomplish this. 

## OPTIONS

`--skip-line`
: Skip in front of the existing pull requests, merge this pull request right
  now.

`--targets`
: Additional targets affected by this pull request.

## SEE ALSO

`av-pr-create`(1)
