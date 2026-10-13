# 0014. defoldpls may change a project, when asked

Status: accepted, 2026-10-10. Supersedes [0004](0004-read-only.md) in part.

## Context

0004 decided that project files are read and never written. It was written for
defold-vet, a program that judges a project and gates a pipeline: such a program must
not be able to damage what it judges.

defoldpls is a different program. It serves someone who is editing, and some of what
an editor is asked to do cannot be done without writing: renaming a component means
changing the game object file that declares it together with every script that
addresses it. Today that is done by hand in two tools, the Defold editor for the data
file and a text editor for the scripts, and nothing checks that the two agree.

What 0004 was afraid of still holds. Data files are written by the Defold editor in its
own format, and a second tool that rewrites them can corrupt them.

## Decision

defoldpls may change the files of a project, data files included. defold-vet still
writes nothing.

- **Only when asked.** A change is the answer to a request that names it: a rename, a
  fix, a command. Reading and checking never write.
- **Through the editor where there is one.** The server answers with edits and the
  editor applies them, so that a change can be seen before it is accepted and undone
  after.
- **In place.** A data file is changed by replacing the text of a value where it
  stands. It is not read into a structure and written out again, so everything the
  change does not touch stays as the Defold editor left it, whatever its format.
- **All or nothing.** Every edit is compared with the text it replaces before any is
  offered. If one of them does not find what it expects, or the new name would collide
  with one that exists, the whole change is refused and says why.
- **Held by tests.** Whatever changes a project is tested by reading the project again
  after the change: the same problems are found, and the name is where the old one was.

## Consequences

- Renaming across scripts and data files, and fixes in data files, become possible.
- A change is only as complete as what can be read. A name built at run time is not
  renamed, and [strict mode](../defold-vet/strict-mode.md) is what says where those
  are.
- Files that have to be created whole, a new game object for instance, cannot be made by
  replacing text. They are written in the format the Defold editor writes, which the
  tests hold against real files: every atlas of the sample projects comes back byte for
  byte when its last image is taken out and put back, and what `defoldpls new` makes
  was built by Defold's own build tool.
- The promise of 0004, that running the tool can never damage a project, now holds for
  defold-vet and for everything in defoldpls that was not asked to change something.
