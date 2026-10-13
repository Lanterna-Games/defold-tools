# 0017. A project just opened shows only mistakes

Status: accepted, 2026-10-11

## Context

The first thing the tools did in a project that had never seen them was to paint it.
Every `go`, `msg` and `hash` was underlined by the Lua language server as an undefined
global, and every `init` and `update` as a global in lowercase, dozens of times in a
small game, because nothing had told that server about Defold yet. And a module that
nothing required, or a script not yet attached, stood in the Problems panel as an
error.

None of that is a mistake in the game. The first is a setting nobody was asked about
yet, and 0013 keeps the extension from writing that setting unasked. The second is a
file that may be exactly where its author wants it.

## Decision

What a project shows before anything is set up is what is wrong in it, and nothing
else.

- **The checks say which of them report a mistake.** Two do not: `unverifiable`, a name
  that cannot be read, and `unused-file`, a file nothing uses. A language server sends
  those as warnings and every other problem as an error. The command line is unchanged:
  a problem is a problem, and the exit code is 1.
- **The VS Code extension names the globals Defold gives a script**, as a default of
  `Lua.diagnostics.globals`: the modules of the engine, `hash`, `hash_to_hex`, `pprint`
  and the functions the engine calls. A default is not written anywhere, so 0013 holds,
  and a project that sets the list itself wins over it.
- **The list is kept equal to the documentation by a test**, since the extension has no
  build step that could write it.
- **Accepting the annotations also says which Lua it is**, `Lua.runtime.version`, when
  neither the user nor the project says. It is written with the same question and into
  the same file as the library.

## Consequences

- A project opened for the first time shows no undefined global of Defold, whether the
  annotations are taken or not. Completion and types for the API still need them: the
  list silences, it does not describe.
- The default is the extension's, not the project's: with it installed, a Lua project
  that is not a Defold game is not told that `go` or `msg` is undefined either.
- The list is that of the newest Defold the program carries. A module of an older
  version that has since gone is not in it, and one that a project of an older version
  does not have is.
- This was verified with the Lua language server itself over a small game, given the
  list as its settings: no undefined global and no global in lowercase was left. VS
  Code takes a default from another extension for a setting of resource scope, which
  this one is; that was read in the VS Code it was written for, and not tried in one
  that was running.
