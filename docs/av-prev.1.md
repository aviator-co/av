# av-prev

## NAME

av-prev - Checkout the previous branch in the stack

## SYNOPSIS

```synopsis
av prev [<n> | --first]
```

## DESCRIPTION

Checkout a previous branch in the stack. Without any options, this will default
to checking out the previous branch in the stack.

## OPTIONS

`<n>`
: Checkout the branch that is `<n>` branches before the current branch in the
  stack.

`--first`
: Checkout the first branch in the stack.

## SEE ALSO

`av-next`(1)
