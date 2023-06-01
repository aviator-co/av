# av-stack-submit 1 "" av-cli "Aviator CLI User Manual"

# NAME

av-stack-submit - Create pull requests for every branch in the stack

# SYNOPSIS

`av stack submit`

# DESCRIPTION

Create pull requests for every branch in the current stack. This ensures that
every pull request has the correct base branch and includes the correct metadata
in the pull request description.

If a branch has an existing pull request, it will be modified with the correct
base branch and metadata (if necessary).
