# 0012. A window of Defold versions is supported, and carried

Status: accepted, 2026-10-10. Narrows [0010](0010-where-a-reference-comes-from.md).

## Context

0010 made any published version usable: its reference is fetched and kept. That works,
and promises nothing. The first run needs the network and the Defold repository's tag
list, a CI job fetches every time, and nobody had looked at what a reference of
another version does to the checks.

Looking showed three things. The two archives exist without a gap from 1.2.185 on. A
reference carries what the checks need from 1.6.0 at least. And the documentation has
holes that come and go: the two camera focus messages are documented in 1.6.0 and in
1.13.2 and in no version between 1.8.1 and 1.12.4, and one release, 1.12.3, lacks the
whole editor API. A project that declared such a version would be told that a message
of the engine is not one.

## Decision

defold-vet supports Defold from **1.6.0** to the newest version it was built with, and
carries the reference of every version in between.

- A version in the window is read from the program. No network, no cache.
- A version older than the window is refused, by name.
- A version newer than the program is fetched and kept as 0010 describes, since it may
  be the release of last week. It is used as published, without the next point.

A reference is read against its neighbours when the files are generated. What is
documented before a version and after it, and missing from it, is taken to be a hole in
that version's documentation and is put back: a message, a field of a message, a
property of a kind of component, a function. Each file lists what was put back. What a
version stops documenting for good stays gone, and what a later version adds does not
reach back.

The names written by hand where the reference has a gap in every version carry the
version they hold from.

## Consequences

- The program grows by about a third of a megabyte for 33 versions.
- Every Defold release means regenerating the files, and the file of the release
  before it can change, because it has gained a neighbour.
- The newest version has no later neighbour, so a hole in it is not found until the
  next release. `[known]` in the settings remains the way out.
- The window can be opened further back: the archives are there from 1.2.185. Nobody
  asked, and each older version is one more to answer for.
- Run over the 393 sample projects, the references of 1.6.0, 1.9.0 and 1.11.0 give the
  same reports as that of 1.13.2 except one, an address the older ones do not know to
  be an address.
