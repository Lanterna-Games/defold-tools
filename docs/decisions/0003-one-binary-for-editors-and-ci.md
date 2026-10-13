# 0003. One binary for editors and CI

Status: accepted, 2026-10-09. Superseded in part by
[0011](0011-two-programs-one-repository.md)

## Context

The same mistakes need to be caught while typing and before a merge. If the editor
and the pipeline ran different tools they would disagree sooner or later, and a check
that passes in one place and fails in the other is trusted in neither.

## Decision

There is one analysis and one binary. It speaks the Language Server Protocol to
editors and has a `check` command for the command line. Both report the same problems
from the same code.

It runs beside the Lua language server. It reports only what is specific to Defold and
leaves the Lua language itself to the server that already does that.

## Consequences

- Any editor with a language-server client can use it, not only VS Code.
- A problem shown in the editor is exactly the one that fails CI.
- The editor extension holds no logic, so it rarely needs to change.
- Features that only make sense in an editor, such as completion, are built on the
  same project index as the checks.
