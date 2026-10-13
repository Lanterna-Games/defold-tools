# 0018. A setting Defold removed is reported

Status: accepted, 2026-10-12

## Context

When the settings of `game.project` began to be checked, a key that the project's
Defold version does not have and another supported version has was left alone. A
project file gathers keys over the years, and saying something about each of them
would have been noise.

Defold 1.14.0 made one of those keys matter. It always shares one Lua state between
scripts, GUI scripts and render scripts, and took `[script] shared_state` out of the
schema. A project that had it at `0` behaves differently after the update and is told
nothing: Defold skips a key it does not know. And the project templates wrote the key
for years, so the line is in almost every project: in 389 of the 393 this was measured
on, 388 of them with the value the engine now always uses.

## Decision

A setting that Defold removed is reported, by a check of its own, `removed-setting`,
in a project that targets a version without it.

- **Removed means gone for good.** An earlier version carried has it and no later one
  does. The references say so for each version, with the version that removed it. A
  setting missing from one version and back in the next is a hole in a schema and is
  left alone, and so is one that a later version brings.
- **It is upkeep, not a mistake.** A language server sends it as a warning, as it does
  a file nothing uses (0017), and offers a fix that removes the line. The fix is an
  edit the editor applies when asked (0014).
- **The command line reports it like any problem**, and the exit code is 1. There is
  one kind of problem there, and a line the engine no longer reads is one.
- **It does not say what the removal changes.** Whether the value written was the one
  the engine now uses is known for one setting of one release, by reading its notes. A
  table of such cases would have to be kept by hand at every release, and is not.

## Consequences

- Nearly every existing project reports once after it moves to Defold 1.14.0, until
  the line is taken out or the check is turned off in `defold-vet.toml`. For all but
  the few that had the setting at `0`, taking the line out changes nothing.
- A project that declares the version it targets is told when it moves, not before.
- What Defold removed before 1.6.0 is not known, since no reference that old is
  carried.
- Each Defold release can add to what is reported, with no change to the program
  besides the references.
