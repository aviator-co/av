# Aviator CLI manual pages

The Markdown files that end with `.\d.md` in this directory are manual pages for
Aviator CLI.

## Online manual documentation pages

Run `man man` in your system to see the overview of the online manual pages. The
number in the filename is the manual page sections. Typically, the manual pages
for shell commands are in section 1. You will likely write a page under this
section. In the manual pages, other pages are referenced like `git(1)`, where
the number after the page name is the section number. The section number is used
to distinguish pages with the same name. For example, `man 1 exit` will show the
help document for the shell command `exit`, and `man 3 exit` will show the help
for the C POSIX API's `void exit(int)`.

Git's reference pages are good references for the manual pages. Try `man 1 git`
or `man 1 git-commit`.

## Converting the Markdown files

Go script `convert-manpages.go` has two modes.

1. Preview mode: Run `go run ./convert-manpages.go FILE`, and you'll see the
   preview of the converted man page.
2. Conversion mode: Run `go run ./convert-manpages.go --output-dir=DIR`, and
   you'll get all files in the directory converted to the manual pages in `DIR`.

The conversion output directory can be used as a `MANPATH`. For example, run
`MANPATH=$PWD/DIR man 1 av` will search for the page for `av(1)` in the
directory. You can use this for checking the final output.
