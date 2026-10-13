# 0010. A reference is bundled, or fetched by version and kept

Status: accepted, 2026-10-10. Narrows [0006](0006-tied-to-a-defold-version.md).

## Context

0006 resolves the Defold version in three steps: the one the project declares, the one
of the editor installed on the machine, the newest one bundled. It also says the
reference of a declared version is fetched once and cached.

The middle step has no dependable way to be carried out. The editor has no fixed place
on Linux or Windows, and on a machine where the checker runs in one system and the
editor in another, as under WSL, it is not on the same file system at all. A guess that
is sometimes wrong would change which names are known from one machine to the next.

## Decision

The version is the one `defold-vet.toml` declares, and otherwise the one bundled.
The installed editor is not looked for.

A declared version that is not the bundled one is fetched: its tag in the Defold
repository gives the commit, and the commit gives the two archives Defold publishes for
it, the reference documentation and the builtins. The reference built from them is
kept in the user's cache directory and read from there afterwards. Fetching is said on
the standard error, once.

A version that was never published, or was published without those archives, stops the
run, as does a reference that documents no function. Nothing falls back to the bundled
version in silence.

A project can vouch for a message or a property the reference leaves out, under
`[known]` in its settings.

## Consequences

- The first run against a new version needs the network, and CI needs it unless the
  cache directory is kept between runs.
- Two machines with the same settings know the same names.
- Versions old enough to be published under another layout cannot be used. 1.6.0 and
  1.9.0 were tried and work; 1.2.150 does not.
- A project that declares nothing follows the bundled version, which moves when
  defold-vet is updated.
