# av

## NAME

av - Aviator CLI

## DESCRIPTION

**av** allows you to manage stacked pull requests with Aviator.

## SUBCOMMANDS

- av-auth-status(1): Show info about the logged in user
- av-commit-amend(1): Amend a commit
- av-commit-create(1): Record changes to the repository with commits
- av-commit-split(1): Split a commit into multiple commits
- av-fetch(1): Fetch latest repository state from GitHub
- av-init(1): Initialize the repository for `av`
- av-pr-create(1): Create a pull request for the current branch
- av-pr-queue(1): Queue an existing pull request for the current branch
- av-pr-status(1): Get the status of the associated pull request
- av-stack-adopt(1): Adopt branches that are not managed by `av`
- av-stack-branch-commit(1): Create a new branch in the stack with the staged changes
- av-stack-branch(1): Create or rename a branch in the stack
- av-stack-diff(1): Show the diff between working tree and parent branch
- av-stack-next(1): Checkout the next branch in the stack
- av-stack-orphan(1): Orphan branches that are managed by `av`
- av-stack-prev(1): Checkout the previous branch in the stack
- av-stack-reorder(1): Interactively reorder the stack
- av-stack-reparent(1): Change the parent of the current branch
- av-stack-restack(1): Rebase the stacked branches
- av-stack-submit(1): Create pull requests for every branch in the stack
- av-stack-switch(1): Interactively switch to a different branch
- av-stack-tidy(1): Tidy stacked branches
- av-stack-tree(1): Show the tree of stacked branches
- av-sync(1): Synchronize stacked branches with GitHub

## FURTHER DOCUMENTATION

See [Aviator documentation](https://docs.aviator.co) for the help document
beyond Aviator CLI.
