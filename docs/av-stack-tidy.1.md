# av-commit-create 1 "" av-cli "Aviator CLI User Manual"

# NAME

av-stack-tidy - Record changes to the repository with commits

# SYNOPSIS

`av stack tidy`

# DESCRIPTION

Tidy stacked branches by removing deleted or merged branches.

This command detects which branches are deleted or merged and re-parents
children of merged branches. This operates on only av's internal metadata and
does not delete Git branches.
