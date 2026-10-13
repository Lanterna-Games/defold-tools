# defold-vet

defold-vet reads a [Defold](https://defold.com) project and reports the names in it
that point at nothing: a component that is not there, a message nobody handles, a
property the target does not have, an input action that was never bound.

Defold names almost everything by string, and a string is not checked until the line
that uses it runs. Some are never checked at all: `message_id == hash("colision_response")`
is simply false, for ever. Everything needed to catch these is already in the project,
in the collections, game objects, GUI scenes, atlases and input bindings. defold-vet
reads those files and the scripts together and compares one with the other.

| | |
|---|---|
| [Checks](checks.md) | Every check: what it reports, an example, when it stays quiet |
| [How it reads a project](how-it-reads.md) | What counts as written, how a shared script is judged, what it cannot see |
| [Strict mode](strict-mode.md) | Reporting what cannot be verified |
| [Settings](settings.md) | `defold-vet.toml`, and silencing one line |
| [In the editor](editors.md) | The language server and the VS Code extension |
| [In CI and git hooks](ci.md) | Running it before a merge and before a commit |

## Installing

defold-vet is one program with no runtime to install beside it. It needs Go 1.27 or
newer to build.

```sh
go install github.com/Lanterna-Games/defold-tools/cmd/defold-vet@latest
```

Or from a clone, which installs into `~/.local/bin` (set `PREFIX` for another place):

```sh
git clone https://github.com/Lanterna-Games/defold-tools.git
cd defold-tools
make install
```

## Running it

Run it in the directory that holds `game.project`, or name that directory:

```console
$ defold-vet check
main/ball.script:5:14: "#bounce_sond": "/ball" has no component "bounce_sond" (did you mean "bounce_sound"?) [unresolved-url]
main/ball.script:13:28: "bal_out" is posted to "/game#game", but /main/game.script does not handle it (did you mean "ball_out"?) [unhandled-message]
main/ball.script:29:21: "p3_up" is not bound in /input/game.input_binding (did you mean "p1_up"?) [unbound-action]
main/main.collection:10:14: property "game" of "/ball#ball" points at "/game#gaem", but "/game" has no component "gaem" (did you mean "game"?) [unresolved-url]
```

Each line is one problem:

```
file:line:column: what is wrong [check]
```

The file is relative to the directory that was checked. The line and the column point
at the name itself, also when it sits inside a component embedded in a game object or
a collection. The name in brackets is the check, which is what
[settings](settings.md) and ignore comments refer to.

A project needs no setup for this. With no settings file every check runs, strict
mode included, against the newest Defold version defold-vet knows. Defold from 1.6.0
on is supported, and [the `defold` setting](settings.md#defold) says which version a
project targets.

### Exit codes

| Code | Meaning |
|---|---|
| 0 | Nothing was found |
| 1 | Problems were found, and printed on the standard output |
| 2 | It could not run: the directory is not a Defold project, the settings were not understood, or the declared Defold version is not one it supports. The reason is on the standard error |

### What is read

Every file of the project, except what is under a directory whose name starts with a
dot and the `build` directory at the top. Libraries are not fetched: a project that
depends on one is checked as far as its own files allow, as
[how it reads a project](how-it-reads.md#code-out-of-sight) explains.

defold-vet never writes to the project. Data files belong to the Defold editor.

## What it does not do

- It does not check Lua itself: syntax, undefined variables, types. The
  [Lua language server](https://luals.github.io) does that, and defold-vet is meant to
  run beside it.
- It does not run the game. A name built while the game runs cannot be compared with
  anything, and [strict mode](strict-mode.md) is how those places are made visible.
- Native extensions and shaders are out of scope.

## The other commands

| Command | |
|---|---|
| `defold-vet lsp` | Serve an editor over the Language Server Protocol. See [in the editor](editors.md) |
| `defold-vet version` | Print the version |
| `defold-vet help` | Print the usage |
