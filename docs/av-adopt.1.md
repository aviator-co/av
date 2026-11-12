# av-adopt

## NAME

av-adopt - Adopt branches that are not managed by `av`

## SYNOPSIS

```synopsis
av adopt [--dry-run] [--parent=<parent> | --remote=<branch> [--yes]]
```

## DESCRIPTION

`av adopt` is a command to adopt a branch or an entire stack that is not
managed by `av`. This command is useful when you have branches that are created
outside of `av` and want to manage them with `av`.

When you run this command, it looks into the commits to figure out the branch
structure in the following way:

* For all non-adopted branches, find the nearest `av` managed branch, other
  non-adopted branch, or the commits that are included by the trunk branch.
* If a merge commit exists while finding the nearest branch, the command will
  not adopt that branch and its children.

After this process, it prompts you to choose which branch to adopt. By default,
it adopts all the branches it finds. If you want to adopt only a specific
branch, you can unspecify the branches you don't want to adopt.

If a branch contains a merge commit or if there are multiple possible parents,
the command will exit with an error. In this case, you need to manually rebase
branches so that they form a tree structure.

## ADOPTING A SINGLE BRANCH

If you want to adopt the current branch and specify the parent branch, you can
do so by running the command with `--parent`.

## ADOPTING FROM A REMOTE REPOSITORY

You can also adopt branches from a remote repository by using the
`--remote` option. This option fetches the specified remote branch and adopts
it along with its parent branches.

By default `--remote` opens an interactive picker to choose which branches to
adopt. Pass `--yes` to skip the picker and adopt what it selects by default:
the named branch and each open, not-yet-adopted ancestor pull request found by
following av's PR metadata. Merged or closed ancestors are skipped and their
child is reparented onto the next open ancestor or the trunk. After adopting, av
switches to the top adopted branch, unless the working tree prevents the switch.
This is useful for scripts and agents that want to mirror a remote stack locally
without a prompt. Combine with `--dry-run` to print the adoption plan without
applying it.

## OPTIONS

`--parent=<parent>`
: Force specify the parent branch.

`--dry-run`
: Show what branches would be adopted without actually adopting them.

`--remote=<branch>`
: Specify a remote branch to adopt from.

`--yes`
: With `--remote`, adopt the named branch and its open ancestors without opening
the picker. Fails if the named branch's pull request is not open, or if a pull
request in the stack has no av metadata while its base branch has its own open
pull request.

## SEE ALSO

`av-orphan`(1) for orphaning a branch.
