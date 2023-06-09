# av-stack-prev

## NAME

av-stack-prev - Checkout a previous branch in the stack

## SYNOPSIS

```synopsis
av stack prev [<n> | --first]
```

## DESCRIPTION

Checkout a previous branch in the stack. Without any options, this will default
to checking out the previous branch in the stack.

## OPTIONS

`<n>`
: Checkout to the branch that is `<n>` branches before the current branch in the
  stack.

`--first`
: Checkout to the first branch in the stack.

## SEE ALSO

`av-pr-next`(1)
