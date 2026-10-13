# Settings

A project needs no settings: without any, every check runs, strict mode included,
against the Defold version built into defold-vet.

When a project wants some, they go in a file named `defold-vet.toml`, next to
`game.project`. It is [TOML](https://toml.io).

```toml
defold = "1.14.0"
strict = false

[checks]
unknown-collision-group = false

[[ignore]]
check = "missing-resource"
path = "assets/generated/**"
reason = "written by the build"

[known]
messages = ["set_scale"]
properties = ["outline_width"]
```

A setting that is not understood stops the run, with exit code 2:

```console
$ defold-vet check
defold-vet: defold-vet.toml: "unresolved-ulr" is not a check
```

A misspelt check would otherwise stay on, or off, without a word.

## defold

The Defold version the project targets, as in `"1.14.0"`.

Which messages are the engine's, which properties a sprite has and where a function
expects an address all depend on the version. Without this setting the newest version
defold-vet was built with is used, which moves when defold-vet is updated. Declaring
it keeps the answers the same until the project decides to move, and raising it shows
what an engine update broke before the game is run.

| The version | |
|---|---|
| From 1.6.0 to the newest one defold-vet was built with | Supported. Its reference is carried by the program: no network is needed |
| Older than 1.6.0 | Refused |
| Newer than the program | Fetched from Defold's archive the first time, which needs the network, and kept in the user's cache directory (`~/.cache/defold-vet` on Linux). Updating defold-vet is the better answer |

A fetch is announced on the standard error:

```
defold-vet: fetching the reference of Defold 1.14.0, which this build does not carry
```

A version that was never published stops the run. Nothing falls back to another
version in silence.

To see what another version would say without touching the settings:

```sh
defold-vet check --defold 1.12.0
```

## strict

`strict = false` leaves [strict mode](strict-mode.md): nothing is reported as
`unverifiable`. It is the same as turning that check off under `[checks]`.

## [checks]

Turns a check off, by the name printed in brackets after each problem.

```toml
[checks]
unknown-collision-group = false
unposted-message = false
```

The names are listed in [checks](checks.md). `ignore`, the report about ignore comments
themselves, cannot be turned off.

## [[ignore]]

Leaves one check out of the files a pattern matches. It is the way to silence a problem
in a data file, which has no place for a comment since the Defold editor rewrites it.

```toml
[[ignore]]
check = "missing-resource"
path = "assets/generated/**"
reason = "written by the build"

[[ignore]]
check = "unverifiable"
path = "gui/*.gui_script"
reason = "the lists build their node ids from the row"
```

- `check` is one check.
- `path` is relative to the project, with `/` between directories. `*` matches within a
  directory and `**` across any number of them.
- `reason` is required. An ignore without one stops the run.

## [known]

Vouches for a name the engine accepts and its reference does not document.

```toml
[known]
messages = ["set_scale"]
properties = ["outline_width"]
```

A message listed here is treated as the engine's: it may be posted to any component,
and a handler for it is not reported as unposted. A property listed here is accepted
on any game object or component.

This is for gaps in the reference.

## Silencing one line

In a script, a comment silences a check where it stands:

```lua
msg.post("#shield", "enable") -- defold-vet:ignore unresolved-url the shield is added by the level loader

-- defold-vet:ignore unverifiable the skin is chosen in the shop
sprite.play_flipbook("#sprite", self.skin)
```

- A comment at the end of a line is for that line. A comment alone on its line is for
  the line below.
- Several checks are separated by commas, without spaces: `unresolved-url,unhandled-message`.
- Everything after the checks is the reason, and it is required.

An ignore is a claim that something is wrong on purpose, so one that is not in order is
itself reported, as [`ignore`](checks.md#ignore): one without a reason, one that names
something that is not a check, and one that silences nothing. The last keeps ignores
from outliving what they were written for.
