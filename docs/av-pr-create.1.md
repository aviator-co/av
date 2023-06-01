# av-pr-create 1 "" av-cli "Aviator CLI User Manual"

# NAME

av-pr-create - Create a pull request for the current branch

# SYNOPSIS

`` av pr create [-t <title>| --title=<title>] [-b <body>| --body=<body>]
    [--draft] [--edit] [--force] [--no-push] ``

# DESCRIPTION

Create a pull request for the current branch.

# OPTIONS

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

# EXAMPLES

Create a pull request, specifying the body of the PR from standard input:

    $ av pr create --title "Implement fancy feature" --body - <<EOF
    > Implement my very fancy feature.
    > Can you please review it?
    > EOF

# SEE ALSO

`av-stack-submit`(1)
