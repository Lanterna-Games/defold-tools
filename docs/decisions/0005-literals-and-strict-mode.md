# 0005. Check literals, and be strict by default

Status: accepted, 2026-10-09

## Context

A string written in the source can be compared with what the project declares. A
string built while the game runs cannot: `hash("enemy_" .. n)`, a URL received in a
message, the id of an object a factory spawned.

That leaves a choice for the references that cannot be verified. Staying silent keeps
the output clean and lets exactly the mistakes this tool exists for slip through
wherever an id is computed. Reporting them asks more of the code.

## Decision

defold-vet verifies literals, and constants that hold them. In strict mode, which is
on by default, a reference it cannot verify is reported as unverifiable.

Strict mode can be turned off for a project, and a single line can be silenced with a
comment that says why.

Whether anyone handles a message is inferred from the whole project. It is reported,
because an unhandled message is usually a typo, and it can be silenced, because it is
an inference and not a proof.

## Consequences

- Ids move into literals and named constants, where they can be checked.
- A project that builds ids at run time has to silence those lines or turn strict mode
  off.
- A false positive is treated as a bug in defold-vet, not as the user's problem.
