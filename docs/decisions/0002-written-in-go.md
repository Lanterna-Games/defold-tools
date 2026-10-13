# 0002. Written in Go

Status: accepted, 2026-10-09

## Context

defold-vet has to run in three places: inside an editor as a language server, on a
developer's machine from a git hook, and in CI. Defold itself runs on Windows, macOS
and Linux, so the tool has to as well.

Three languages were considered.

- **Go.** One static binary with no runtime, cross-compiled from any machine. It has
  no Lua parser that fits: the one that exists keeps the line of a node but not its
  column, and gives nothing back when the code has a syntax error, which is the normal
  state of a file being typed.
- **Lua.** Every Defold user knows it, and a tolerant Lua parser written in Lua
  exists. But Lua has no runtime-free distribution: it needs an interpreter and native
  modules for the file system, which is harder to ship than a Go binary. It would also
  be a tool for strictness written in the language whose lack of it is the problem.
  Running as a plugin of the Lua language server was ruled out: its plugins can
  rewrite a file's text and syntax tree, but cannot report a problem.
- **TypeScript.** The conventional choice for an editor extension, with the reference
  language-server library and a ready parser. It ties the command-line tool to Node,
  which a Go or Defold project otherwise has no reason to install in CI.

## Decision

defold-vet is written in Go.

## Consequences

- It ships as one binary per platform with nothing else to install.
- The same binary is the language server and the command-line tool
  ([0003](0003-one-binary-for-editors-and-ci.md)).
- The project writes and maintains its own Lua parser: Lua 5.1, positions down to the
  column, and a partial tree out of broken code.
- An editor extension is a thin shell that starts the binary, and has to carry one
  binary per platform.
- Fewer people in the Defold community write Go than Lua.
