# av-git-interaction

## NAME

av-git-interaction - av CLI Git Interaction

## BRANCH CREATION AND ADOPTION

Typically in Git, a branch is created with `git branch`, `git switch`, or `git
checkout`. Since `av` needs to keep track of the extra information about
branches such as the parent branch, we have `av-stack-branch`(1) to create a new
branch and track the necessary information. This metadata is stored in
`.git/av/av.db`. We call the branches that `av` has metadata for as "managed
branches", and the branches that av doesn't have metadata for as "unmanaged
branches".

There is a case where you created a branch without going through
`av-stack-branch`(1). In this case, you can attach the branch metadata by using
`av-stack-adopt`(1). The opposite can be done with `av-stack-orphan`(1).

## BRANCH DELETION

When you merge a branch, `av-sync`(1) will prompt you to delete the merged
branches. However, there can be a case where you want to delete a branch that is
not merged yet. In this case, you can delete the branch with `git branch -d|-D`.

When you delete a branch with `git branch -d|-D`, the branch metadata is still
kept in `.git/av/av.db`. Next time you run `av` command, it'll be automatically
removed.

If a deleted branch has a child branch, the child branch will be orphaned. This
means that the child branch still exists in the Git repository, but `av` will
not manage it. In order to add it back to `av`, you can use `av-stack-adopt`(1).
