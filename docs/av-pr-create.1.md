# av-pr-create

## NAME

av-pr-create - Create a pull request for the current branch

## SYNOPSIS

```synopsis
av pr create [-t <title>| --title=<title>] [-b <body>| --body=<body>]
    [--draft] [--edit] [--force] [--no-push]
```

## DESCRIPTION

Push the current branch and create a pull request if not exist. If the branch
has a parent branch, you need to make a pull-request for the parent first. If
title and body are not provided, `$EDITOR` pops up and you are asked to provide
them.

When there's already a pull-request, the command just pushes the branch to
remote, and you are not asked to provide the title and the body. If you want to
edit the pull-request description for the existing pull-request, use `--edit`.

## OPTIONS

`-t <title>, --title=<title>`
: Use the given `<title>` as the title for the pull request.

`-b <body>, --body=<body>`
: Use the given `<body>` as the body for the pull request.

`--draft`
: Open the pull request as a draft.

`--edit`
: Edit the pull request title and description before submitting even if the
  pull request already exists.

`--force`
: Force creation of a pull request even if there is already a pull request
  associated with this branch.

`--no-push`
: Do not push the branch to the remote repository before creating the pull
  request.

## EXAMPLES

Create a pull request, specifying the body of the PR from standard input:

```bash
$ av pr create --title "Implement fancy feature" --body - <<EOF
> Implement my very fancy feature.
> Can you please review it?
> EOF
```

## SEE ALSO

`av-stack-submit`(1)
