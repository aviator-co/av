# av-stack-sync

## NAME

av-stack-sync - Synchronize stacked branches

## SYNOPSIS

```synopsis
av stack sync [--all | --current] [--no-push] [--no-fetch] [--prune]
              [--trunk] [--continue | --abort | --skip] [--parent=<parent>]
```

## DESCRIPTION

Over the time, branches get unsynchronized in many ways. Some branches are
merged. The upstream branch is moved. A parent branch get their commit amended.
`av stack sync` synchronizes the branches following the changes.

For each branch, this command does the following

* Rebase onto the parent branch. By default, if the parent is the trunk branch
  (e.g. `main`), this step is skipped. If `--trunk` is used, it fetches the
  trunk branch from the remote and rebase onto it.

* Push to the remote branch. With Git's default config, the push updates the
  same name branch on the remote.

Note that currently, this overwrites the remote with force. This can overwrite
any changes happen on GitHub. To avoid this, pull or manually cherry-pick the
changes on the remote.

By default, this command will sync all branches starting at the root of the
stack and repeatedly executes the above steps. If the --current flag is given,
this command will sync the branches up to the current one, and the rest of the
branches are not synced. This allows you to make changes to the current branch
before syncing the rest of the stack. If the --all flag is given, it will sync
all branches in the repository.

If --prune option is given, it deletes the merged branches at the end of sync.

## REBASE CONFLICT

Rebasing can cause a conflict. When a conflict happens, it prompts you to
resolve the conflict, and continue with `av stack sync --continue`. This is
similar to `git rebase --continue`, but it continues with syncing the rest of
the branches.

## CHANGE PARENT

If you want to change the parent, use `--parent=<parent>` to specify the new
parent. This rebases the current branch onto the new parent and runs the sync
operations on the children.

## ADOPTING BRANCHES

If you want to adopt a Git branch that is created outside of `av`, you can run
`av stack sync --parent=<parent>` or `av stack sync --parent=<parent> --trunk`
to adopt a branch to `av`. If the parent is a trunk branch (e.g. main), use
`--trunk`.

## OPTIONS

`--all`
: Synchronize all branches.

`--current`
: Only sync changes to the current branch. (Don't recurse into descendant
  branches.)

`--no-push`
: Do not force-push updated branches to GitHub.

`--no-fetch`
: Do not fetch latest PR information from GitHub.

`--prune`
: Delete the merged branches.

`--trunk`
: Synchronize the trunk into the stack.

`--continue`
: Continue an in-progress sync.

`--abort`
: Abort an in-progress sync.

`--skip`
: Skip the current commit and continue an in-progress sync.

`--parent=<parent>`
: Parent branch to rebase onto.
