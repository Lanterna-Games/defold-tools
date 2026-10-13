# 0004. Read project files, never write them

Status: accepted, 2026-10-09. Superseded in part by [0014](0014-defoldpls-may-change-a-project.md): it holds for defold-vet.

## Context

Checking a reference means knowing what exists, and that is declared in the project's
data files: collections, game objects, GUI scenes, atlases, input bindings. So
defold-vet has to read all of them, not only the scripts.

Those files belong to the Defold editor, which rewrites them in its own format. A
second tool writing to them is a way to corrupt a project.

Some mistakes also live in the data files and not in a script: a property override in
a collection that points at an object that was renamed.

## Decision

defold-vet reads every file of the project and writes none. It reports problems on
data files as well as on scripts, when that is where the mistake is.

## Consequences

- Running it can never damage a project.
- There are no automatic fixes to data files.
- The readers have to tolerate fields they do not know, because the formats grow with
  Defold.
