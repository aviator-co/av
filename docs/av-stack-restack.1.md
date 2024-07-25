# av-stack-restack

## NAME

av-stack-restack - Rebase the stacked branches

## SYNOPSIS

```synopsis
av stack restack [--dry-run] [--continue | --abort | --skip]
```

## DESCRIPTION

`av stack restack` is a command to re-align the stacked branches. When a parent
branch is amended or has a new commit, the children branches need to be rebased
on the new parent. This command does the rebase operation for all the branches
in the current stack. This command does not push the changes to the remote.

## REBASE CONFLICT

Rebasing can cause a conflict. When a conflict happens, it prompts you to
resolve the conflict, and continue with `av stack restack --continue`. This is
similar to `git rebase --continue`, but it continues with syncing the rest of
the branches.

## OPTIONS

`--all`
: Rebase all branches.

`--current`
: Only rebase up to the current branch. (Don't recurse into descendant
  branches.)

`--continue`
: Continue an in-progress rebase.

`--abort`
: Abort an in-progress rebase.

`--skip`
: Skip the current commit and continue an in-progress rebase.

`--dry-run`
: Show the list of branches that will be rebased without actually rebasing.

## SEE ALSO

`av-stack-sync`(1) for syncing with the remote repository.
