# av-adopt

## NAME

av-adopt - Adopt branches that are not managed by `av`

## SYNOPSIS

```synopsis
av adopt [--dry-run] [--parent=<parent> | --remote=<branch> [--include=ancestors]]
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
adopt. Pass `--include=ancestors` to skip the picker and non-interactively adopt
the named branch together with its ancestor branches up to the trunk. This is
useful for scripts and agents that want to mirror a remote stack locally without
a prompt. Combine with `--dry-run` to print the adoption plan without applying it.

## OPTIONS

`--parent=<parent>`
: Force specify the parent branch.

`--dry-run`
: Show what branches would be adopted without actually adopting them.

`--remote=<branch>`
: Specify a remote branch to adopt from.

`--include=<set>`
: With `--remote`, non-interactively adopt the named branch and a related set of
branches instead of opening the picker. The only supported value is `ancestors`,
which adopts the named branch and its ancestors up to the trunk.

## SEE ALSO

`av-orphan`(1) for orphaning a branch.
