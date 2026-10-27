# E2E Test Scripts

Tests use Go's [testscript](https://pkg.go.dev/github.com/rogpeppe/go-internal/testscript) framework. Each `.txtar` file is an independent test with a fresh git repo, bare remote, and mock GitHub GraphQL server.

## Directory layout at runtime

- `$WORK/repo` (working directory) — the git repo where commands run
- `$WORK/remote` — bare remote (origin)
- `$WORK/*.txt` etc. — files from the `-- filename --` archive section at the bottom of the `.txtar` file

Use `cp $WORK/fixture.txt target.txt` to copy archive files into the repo.

## Custom commands (defined in `testscript_test.go`)

| Command | Usage | Description |
|---------|-------|-------------|
| `commit-file` | `commit-file [--amend] <file> <content> [msg]` | Write file, `git add`, `git commit`. Content supports `\n`/`\t` escapes. Default message: `Write <file>`. |
| `branch-parent` | `branch-parent <branch> <expected>` | Assert av metadata parent name. Supports `!` negation. |
| `branch-parent-hash` | `branch-parent-hash <branch> <ref>` | Assert branching point commit hash matches resolved ref. |
| `branch-children` | `branch-children <branch> [children...]` | Assert child branch names. |
| `set-branch-pr` | `set-branch-pr <branch> <id> <number> <state>` | Set PR metadata in av database. |
| `set-branch-merge-commit` | `set-branch-merge-commit <branch> <ref>` | Set MergeCommit field; ref is resolved via `git rev-parse`. |
| `mock-pull` | `mock-pull <head> <number> <state> [mergeCommitRef]` | Add a mock PR to the GitHub server. |
| `set-branch-prefix` | `set-branch-prefix <prefix>` | Update `branchNamePrefix` in av config. |

## Conventions

- Start with a comment describing the scenario; use ASCII diagrams for branch structure.
- `exec av ...` runs the av CLI (stdin is `/dev/null` to avoid hangs).
- `! exec ...` expects failure. `stdout 'pattern'` / `! stdout 'pattern'` asserts on the last command's stdout.
- Place file fixtures in the archive section (`-- name --`) at the end.
