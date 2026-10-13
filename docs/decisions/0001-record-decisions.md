# 0001. Record decisions

Status: accepted, 2026-10-09

## Context

The code is meant to explain itself, and by [0007](0007-no-comments-in-source.md) it
carries no comments. That leaves no place in the source for the reasons behind a
choice, and reasons are what a contributor needs before changing something that looks
odd.

## Decision

Every decision that shapes the project is written down in this folder, in the merge
request that makes it: the language, a dependency, the scope, a rule for the codebase.

A record is short. It says what forced the choice, what was chosen, and what follows
from it, including what it costs.

## Consequences

- A contributor can find out why without asking.
- A decision is reviewed like code, and changing one takes a new record.
- It is one more thing to write. Records that only restate the code are not worth it:
  the test is whether someone would otherwise ask.
