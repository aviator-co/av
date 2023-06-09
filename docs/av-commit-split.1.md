# av-commit-split

## NAME

av-commit-split - Split a commit into multiple commits

## DESCRIPTION

Split the currently checked out commit into multiple commits. When invoked, it
prompts you which diff chunks should be included in the first commit and asks
you for the commit message. The process repeats until all diff chunks are
distributed to the commits.
