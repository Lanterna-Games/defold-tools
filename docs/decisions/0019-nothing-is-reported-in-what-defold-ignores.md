# 0019. Nothing is reported in what Defold is told to ignore

Status: accepted, 2026-10-12

## Context

Every file under a project's directory was read and judged, except those under a
directory whose name starts with a dot and under `build`. Defold looks at less.
`.defignore` lists paths that its editor and its build tool leave alone, and its manual
gives the reason: a project that keeps other sources next to the game, such as the Go
code of a server, whose `.go` files Defold would take for game objects.

A project that relied on it got reports about files that are not part of it: a Go
source that is no game object, an export that nothing uses. Fourteen of the 393 sample
projects have the file. Defold 1.14.0 then gave it patterns, with `*`, `?` and `**`,
and a second file, `.defunload`, takes the same patterns for another purpose.

The first reading of this was that an ignored file does not exist. The build tool of
1.14.0 says otherwise. With a path, a file or a pattern in `.defignore` that names a
game object a collection places, it builds the game object all the same, from a clean
build. What `.defignore` stops is the file being found when the tool looks through the
project; a file the game names is built whether it is ignored or not.

## Decision

Nothing is reported in a file that `.defignore` names, and the file is otherwise read
as before.

- **The rules are the build tool's**, read from its source: only lines that start with
  `/`, a path covers itself and what is under it, and the patterns match as its one
  matcher does. The table of examples Defold published with the change is a test.
- **The patterns hold from the version that has them.** For a project that targets a
  Defold before 1.14.0 a line is the path it spells.
- **It happens after the checks**, on what they found, for the command line and the
  language server alike. What is wrong inside an ignored file is dropped, and so is the
  report that nothing uses it.
- **An ignored file the game names stays part of the game.** It is not called missing
  where it is named, and what it uses is not called unused, since the build has it.
- **`.defunload` changes nothing.** It is for Defold's editor, the build takes those
  files, and they are judged.

## Consequences

- A project is no longer told about files it asked Defold to leave alone.
- Ignoring a path hides its mistakes with it, even for a file the game uses. That is
  what the file asks for.
- An ignored file still counts for what the others are judged against: a message an
  ignored script posts is a message that is posted. That errs on the quiet side.
- A rename still changes a name where an ignored file writes it.
- What the build tool does with a named file that is ignored was seen with 1.14.0
  only. Before 1.14.0 it also matched in ways that changed between versions at the
  edges, such as where a name merely begins like a line. Those are not reproduced: a
  line covers its path and what is under it, which is what the manual says.
