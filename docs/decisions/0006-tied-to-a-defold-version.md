# 0006. Tied to a Defold version

Status: accepted, 2026-10-09

## Context

Most of what defold-vet checks comes from the project. Two lists do not: the messages
and the properties of the engine. They change between Defold releases, and a Defold
project does not record which release it targets.

Defold publishes its reference documentation for every release, and ships the same
documentation inside the editor. Both are generated from the engine's source.

## Decision

The engine's messages and properties are read from the reference documentation of one
specific Defold version. That version is resolved in this order:

1. the version the project declares in its defold-vet configuration;
2. the version of the Defold editor installed on the machine;
3. the newest version defold-vet was built with.

## Consequences

- Raising the declared version shows what an update broke before the game is run.
- CI needs no Defold editor: the documentation for the declared version is fetched
  once and cached.
- An id that exists in the engine but is missing from its documentation is reported
  wrongly. A project can list such ids as known, and the gap is worth reporting to
  Defold.
- When the declared version and the installed editor differ, defold-vet says so.
