# 0015. The game is built and run with the editor that is installed

Status: accepted, 2026-10-10

## Context

Working from a text editor still meant going back to the Defold editor to see the game
run. Building a Defold game is the job of Bob, Defold's build tool, which is a Java
program; running it is the job of the engine, a binary for each platform. Neither can
be rewritten here, and neither should be: a second builder would build a different
game.

Someone who makes Defold games has the Defold editor installed, and the editor carries
all three things: a Java to run on, a jar with Bob in it, and the debug engine of every
desktop platform, inside the same jar. It also writes down where it was installed.

The other source is Defold's archive, from which Bob and an engine can be fetched for
any version. That needs a Java on the machine and a download of several hundred
megabytes before the first build.

## Decision

`defoldpls build`, `run`, `reload` and `resolve` use the Defold editor installed on the
machine: its Java, its jar, and the engine taken out of that jar once and kept in the
cache. The editor is found where it says it was installed, or where `DEFOLD_HOME` or
`--defold-home` says.

- `build` runs Bob and prints what someone can act on: an error becomes
  `file:line:column: message`, and the progress, the traces and the warnings of Java
  are left out.
- `run` builds, starts the engine on what was built, and prints what the game logs
  until it is stopped.
- `reload` builds again and asks the running game, over the address its engine serves
  on this machine, to load the changed files again. Which game is running is
  remembered in the cache for as long as it runs.
- `resolve` has Bob fetch the libraries, into `.internal/lib`, where
  [the checks read them](../defold-vet/how-it-reads.md#code-out-of-sight).

They are commands of defoldpls and not a third program: they serve someone who is
editing, and an editor extension already knows where defoldpls is.

## Consequences

- Nothing is downloaded and no Java has to be installed. The game is built by the
  same Bob the editor would use.
- The game is built with the version of the installed editor. When the project
  declares another one, that is said, and the build goes on.
- Without an installed editor these commands do not work. Fetching Bob and an engine
  from Defold's archive, for a machine that only builds, is not written.
- A reload costs a build, and Bob takes a few seconds to start even when little
  changed: about four on the machine this was written on. The Defold editor reloads
  faster because it never stops running.
- A project with native extensions needs an engine built for it. `run` starts the
  editor's plain engine, which does not carry them, and such a project was not tried.
