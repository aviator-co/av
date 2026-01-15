# av-sync-exclude

## NAME

av-sync-exclude - Toggle branch exclusion from sync --all operations

## SYNOPSIS

```synopsis
av sync-exclude [<branch>]
av sync-exclude --list
```

## DESCRIPTION

`av sync-exclude` allows you to toggle whether a branch is excluded from
`av sync --all` operations. This is useful for temporarily excluding
work-in-progress branches or experimental work from your regular sync workflow.

When a branch is excluded, it and all its descendant branches will be skipped
during `av sync --all`. The branches can still be synced by running `av sync`
from within that branch or its stack, or by syncing the stack explicitly.

Running this command on a branch toggles its exclusion state:

- If the branch is currently excluded, it will be included
- If the branch is currently included, it will be excluded

When excluding a branch that has descendant branches, the command will inform
you how many descendants will also be affected by the exclusion.

## CASCADING BEHAVIOR

Exclusion cascades to all descendant branches. For example, if you have a stack:

```
main -> feature-root -> feature-child-1 -> feature-grandchild
                     -> feature-child-2
```

Excluding `feature-child-1` will also exclude `feature-grandchild` from
`av sync --all`, but `feature-child-2` will remain unaffected.

This cascading is automatic and does not require marking each descendant
individually. When you include the branch again, all descendants are
automatically included as well.

## OPTIONS

`--list`
: List all branches that are currently excluded from sync --all operations.
The list is displayed in alphabetical order.

## EXAMPLES

Exclude a branch from sync --all:

```sh
$ av sync-exclude experimental-feature
Branch "experimental-feature" is now excluded from sync --all
Note: 2 descendant branch(es) will also be excluded
```

Include a previously excluded branch:

```sh
$ av sync-exclude experimental-feature
Branch "experimental-feature" is now included in sync --all
Note: 2 descendant branch(es) will also be included
```

List all excluded branches:

```sh
$ av sync-exclude --list
Branches excluded from sync --all:
  - experimental-feature
  - old-prototype
```

## SEE ALSO

`av-sync`(1) for synchronizing stacked branches with GitHub.
`av-tree`(1) for visualizing excluded branches in the stack.
