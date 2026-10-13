# 0026. A check reads again only what changed

Status: accepted, 2026-10-13

## Context

The language server checks the project a fifth of a second after the typing pauses,
and every check started from nothing: each file read from disk and parsed, then the
checks run over all of it. That is the simplest thing that is always right, and for a
small game it takes a few hundredths of a second.

Measured on the sample projects, the largest real ones, with 250 to 380 scripts and
data files, take 0.1 to 0.5 seconds for a whole check. A third to a half of that is
parsing: Lua where the project is mostly code, Defold's text format where it is mostly
scenes. Between two checks one file changed, the one being typed in.

Checking only what a change can affect was looked at and left. What a script means
depends on where it runs, on the modules it requires and on what the data files say,
so a change to one file can change what is said about any other, and finding which
ones is the whole analysis again. Getting that wrong shows as a problem that stays
after it was fixed, or one that never appears.

## Decision

The server keeps what it parsed from each file between checks, and parses a file again
only when its bytes are not the ones it parsed. Everything after the parsing is done
in full, every time.

- **What is kept is what depends on one file alone**: the tree of a script, and the
  messages of a data file with the errors of its syntax. Nothing that was worked out
  by looking at another file is kept.
- **A file is the same when its bytes are.** Not its date, not its size: the content
  as the editor holds it, or as it is on disk.
- **A file that is gone is forgotten** at the end of the check that did not find it.
- **Only the server remembers.** `defold-vet check` and the commands of `defoldpls`
  read the project once and have nothing to keep.
- **It costs no memory to speak of**: the server already holds the project of the last
  check, for completion and hover, and what is kept is part of that.

## Consequences

- With everything kept, a check of those projects takes 30 to 45 percent less: 207 to
  144 milliseconds for one, 480 to 313 for the one that is mostly Lua, 216 to 120 and
  115 to 68 for two others.
- The answer is the one a check that remembers nothing gives. A test holds that for a
  change, a removal and a return, and another, run over the 393 sample projects,
  checks each three times and compares.
- The checks themselves are not faster, and neither is finding the files and reading
  them from disk. A synthetic project of 38,000 files goes from 4.3 seconds to 2.6,
  and that is what is left. The largest real project of the samples has a hundredth
  of its files.
- A file whose bytes are the same is not read differently because another changed.
  That holds as long as parsing a file looks at nothing else, which is what the tests
  above would catch.
- Listing the files of one extension, done once for every missing file that is given a
  suggestion, sorted the names of the whole project each time. It is done once now.
