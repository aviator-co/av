# av-commit-amend

## NAME

av-commit-amend - Amend a commit

## SYNOPSIS

```synopsis
av commit amend [-m <msg>| --message=<msg>] [--no-edit]
```

## DESCRIPTION

The tip of the current branch can be replaced by creating a new commit. During
this process, the recorded tree is prepared as per usual. In cases where no
other message is specified via the command-line option -m, the message from the
original commit is utilized as the initial point rather than an empty message.

## OPTIONS

`-m <msg>, --message=<msg>`
: Use the given `<msg>` as the commit message.

`--no-edit`
: Amends a commit without changing its commit message. (Same as git commit
  --amend --no-edit)
