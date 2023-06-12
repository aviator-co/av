# av-stack-submit

## NAME

av-stack-submit - Create pull requests for every branch in the stack

## SYNOPSIS

```synopsis
av stack submit
```

## DESCRIPTION

Create pull requests for every branch in the current stack. This ensures that
every pull request has the correct base branch and includes the correct metadata
in the pull request description.

If a branch has an existing pull request, it will be modified with the correct
base branch and metadata (if necessary).

## SEE ALSO

`av-pr-create`(1)
