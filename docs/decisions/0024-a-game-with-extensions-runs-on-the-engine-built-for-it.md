# 0024. A game with extensions runs on the engine built for it

Status: accepted, 2026-10-13

## Context

`run` and the debug adapter started the engine the editor carries in its jar ([decision
0015](0015-the-game-is-built-by-the-installed-editor.md)). That engine has no native
extension in it. A project that uses one built without complaint and then failed where
a script called what the extension adds, which reads as a mistake in the script.

Defold's build tool already does the work. When a project has native extensions, a
build sends them to Defold's build server and writes the engine that comes back next
to the build, in a directory named after the platform. With Bob 1.14.0 and a small
extension of our own:

- the engine is `build/x86_64-win32/dmengine.exe` or `build/x86_64-linux/dmengine`,
  under that name for the debug variant and the headless one alike;
- a build of the same project with the extension taken out removes the directory;
- started as the plain engine is, it answered the call the extension adds, where the
  plain engine of the same version did not know it; a breakpoint stopped it and the
  call was evaluated at the stop.

## Decision

When the build left an engine for the platform of this machine, that is the engine
`run` and the debug adapter start. Otherwise it is the editor's, as before.

- **That the file is there is the whole test.** The build tool keeps it there while
  the project has extensions and removes it when it has none, so nothing has to read
  the project to know which engine goes with the last build.
- **An engine that is named wins**, on the command line or in the launch request.
- **It is said**, on a line of its own, which engine runs when it is not the editor's.
- **Nothing is built here.** The extensions go to Defold's build server, sent by
  Defold's own tool, as they are from the Defold editor.
- **What the build tool logs about the request is cut down** to where it asked, and a
  compiler error is passed on once with the line of source under it. The tool repeats
  each error three times and mixes in warnings about Defold's own headers.

## Consequences

- A project with native extensions runs and is debugged from the editor like any other.
- The first build, and each one after an extension changed, waits for the server, and
  needs the network. The tool keeps what it got, so the builds between do not wait.
- With `--built`, the engine is the one the last build left, which is the one that goes
  with what is built.
- A build made for another platform leaves an engine in another directory, and it is
  not taken for this machine's.
- This was seen with 1.14.0 only. A version that named the engine otherwise would run
  on the editor's engine, as before this decision.
