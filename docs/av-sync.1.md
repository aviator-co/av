# av-sync

## NAME

av-sync - Synchronize stacked branches with GitHub

## SYNOPSIS

```synopsis
av sync [--all | --current] [--push=(yes|no|ask)] [--prune=(yes|no|ask)]
        [--rebase-to-trunk] [--continue | --abort | --skip]
```

## DESCRIPTION

`av sync` is a command to fetch and push the changes to the remote GitHub
repository. This command fetches from the remote, restacks the branches, and
pushes the changes back to the remote.

Note that currently, this overwrites the remote with force. This can overwrite
any changes happen on GitHub. To avoid this, pull or manually cherry-pick the
changes on the remote.

When a branch is merged, the child branches are restacked to the new parent. The
command prompts you if the merged branches should be deleted.

## REBASE CONFLICT

Rebasing can cause a conflict. When a conflict happens, it prompts you to
resolve the conflict, and continue with `av sync --continue`. This is similar
to `git rebase --continue`, but it continues with syncing the rest of
the branches.

## REBASING THE STACK ROOT TO TRUNK

By default, the branches are conditionally rebased if needed:

- If a part of the stack is merged, the rest of the stack is rebased to the
  latest trunk commit.
- If a branch is a stack root (the first topic branch next to trunk), it's
  rebased if `--rebase-to-trunk` option is specified.
- If a branch is not a stack root, it's rebased to the parent branch.

While you are developing in a topic branch, it's possible that the trunk branch
is updated by somebody else. In some cases, you may need to rebase onto that
latest trunk branch to resolve the conflicts. For example, if somebody else
updates the same file you are working on, you need to rebase your branch onto
the latest trunk branch. In this case, you can use `--rebase-to-trunk` option to
rebase the stacks to the latest trunk branch.

## OPTIONS

`--all`
: Synchronize all branches.

`--current`
: Only sync changes to the current branch. (Don't recurse into descendant
branches.)

`--rebase-to-trunk`
: Rebase the branches to trunk.

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

`av-restack`(1) for rebasing the branches locally.
`av-adopt`(1) for adopting a new branch.
`av-reparent`(1) for changing the parent of a branch.
