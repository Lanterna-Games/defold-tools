# 0011. Two programs in one repository

Status: accepted, 2026-10-10. Supersedes [0003](0003-one-binary-for-editors-and-ci.md)
in part.

## Context

0003 decided on one binary that both checks a project from the command line and serves
editors. That holds while serving an editor means sending the problems `check` finds,
which is what `defold-vet lsp` does.

A language server that completes names, goes to definitions and fixes a name from its
suggestion is a different program. It has other things to answer for, it is released
when editors need it and not when a check changes, and a tool called "vet" is the
wrong place to look for it. What the two share is not the checks: it is the index of
the project, which says what exists and where.

The Go tools made the same split. `go vet` and `gopls` are separate programs built on
the same analysis packages, kept in one repository.

## Decision

The repository, renamed `defold-tools`, holds more than one program in one Go module:

- `defold-vet` judges a project: the `check` command, its exit code, and `lsp` for the
  problems alone;
- `defoldpls`, when it is written, serves editors with everything else.

Both are built on the same packages, which stay internal to the module. They are not
published as a library: an index that changes with every check would have to promise
an interface it cannot keep yet.

What 0003 was for still holds. There is one analysis, and a problem shown in an editor
is the one that fails CI, whichever program shows it.

## Consequences

- A change to the index and to what is built on it is one change, in one place.
- The module path names the repository, not a program:
  `gitlab.com/lanterna-games/defold-tools/cmd/defold-vet`.
- Someone who wants the index as a library has to ask. If that happens, the packages
  that are stable by then can be moved out from under `internal`.
- An editor extension may come to start two programs. Until `defoldpls` exists, the
  one for VS Code starts `defold-vet lsp`.
