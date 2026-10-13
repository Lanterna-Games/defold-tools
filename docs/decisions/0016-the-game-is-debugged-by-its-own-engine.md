# 0016. The game is debugged with the debugger its engine carries

Status: accepted, 2026-10-10

## Context

Stopping a game at a line, looking at what it holds and going on step by step was the
last thing that needed the Defold editor. Editors speak the Debug Adapter Protocol to a
program, the adapter, which has to speak whatever the game speaks.

A Defold game needs nothing added to be debugged. The engine's builtins hold a Lua
debugger, the one the Defold editor uses, and a running engine takes a script to run
over the same address it takes a reload on. The editor starts a session by having the
engine run one line that starts that debugger on a port, and then connects to it.

Two things stand in the way from outside the editor. The debugger module is only in a
build that names it, and the editor names it through a script of its own that a
project does not have. And the protocol has corners that only show against the real
engine: a pause is never answered, a run can end without saying where, and a step from
a breakpoint first lands on the same line.

## Decision

`defoldpls dap` is a debug adapter. It builds the game, runs it and attaches to the
debugger the engine carries.

- The build is given one more setting, from a file kept in the cache and never in the
  project, that names the engine's debugger module as the script to run at start. The
  module only defines itself, so nothing changes in the game, and it is then in the
  build for the engine to load.
- The engine is asked to run one line that starts its debugger, on a port the system
  hands out, and the adapter connects to it.
- What the debugger sends is read by one listener that tells answers from stops, and
  the adapter mirrors the editor where the protocol is odd.

## Consequences

- Breakpoints, the stack, locals and what tables hold, expressions, steps and pauses
  work from any editor that has a debugger, with the engine that is installed.
- It was built against the real engine without a window, on Linux, and every exchange
  the tests replay was first seen there. It was not tried in a real editor.
- Values are read three levels deep. What lies deeper shows as text and cannot be
  opened.
- It rests on what 0015 rests on: the installed editor. And on the debugger the engine
  ships, which is Defold's to change.
