# av-stack-next 1 "" av-cli "Aviator CLI User Manual"

# NAME

av-stack-next - Checkout a later branch in the stack.

# SYNOPSIS

`av stack next [<n> | --last]`

# DESCRIPTION

Checkout a later branch in the stack. Without any options, this will default to checking out the next branch in the stack.

# OPTIONS

`<n>`
: Checkout to the branch that is `<n>` branches after the current branch in the stack.

`--first`
: Checkout to the last branch in the stack.

# SEE ALSO

`av-pr-prev`(1)
