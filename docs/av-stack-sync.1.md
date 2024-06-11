# av-stack-sync

## NAME

av-stack-sync - Synchronize stacked branches with GitHub

## SYNOPSIS

```synopsis
av stack sync [--all | --current] [--push=(yes|no|ask)] [--prune=(yes|no|ask)]
              [--continue | --abort | --skip]
```

## DESCRIPTION

`av stack sync` is a command to fetch and push the changes to the remote GitHub
repository. This command fetches from the remote, restacks the branches, and
pushes the changes back to the remote.

Note that currently, this overwrites the remote with force. This can overwrite
any changes happen on GitHub. To avoid this, pull or manually cherry-pick the
changes on the remote.

When a branch is merged, the child branches are restacked to the new parent. The
command prompts you if the merged branches should be deleted.

## REBASE CONFLICT

Rebasing can cause a conflict. When a conflict happens, it prompts you to
resolve the conflict, and continue with `av stack sync --continue`. This is
similar to `git rebase --continue`, but it continues with syncing the rest of
the branches.

## OPTIONS

`--all`
: Synchronize all branches.

`--current`
: Only sync changes to the current branch. (Don't recurse into descendant
branches.)

`--push=(yes|no|ask)`
: Push the changes to the remote. If `ask`, it prompts to you when push is
needed. Default is `ask`.

`--prune=(yes|no|ask)`
: Delete the merged branches. If `ask`, it prompts to you when there's a merged
branch to delete. Default is `ask`.

`--continue`
: Continue an in-progress sync.

`--abort`
: Abort an in-progress sync.

`--skip`
: Skip the current commit and continue an in-progress sync.

## SEE ALSO

`av-stack-restack`(1) for restacking the branches locally.
`av-stack-adopt`(1) for adopting a new branch.
`av-stack-reparent`(1) for changing the parent of a branch.
