# How defold-vet reads a project

The checks share a few rules. Knowing them explains both what is reported and what is
left alone.

## What counts as written

A name can be compared with the project only when it can be read from the source. It
is written when it is:

- a string: `"ball_out"`;
- the hash of a string: `hash("ball_out")`;
- a constant that holds one of those.

A constant is a name that stands for one value through the whole file:

```lua
local BALL_OUT = hash("ball_out")           -- a local, declared once, never assigned again

local IDS = { SERVE = hash("serve") }       -- a field of a table of the file
IDS.STOP = hash("stop")                     -- set once, at the top level

local ids = require("main.ids")             -- a field of what a required module returns
msg.post(url, ids.BALL_OUT)
```

In the module, the field has to be set once and at the top level, in the table the
file returns:

```lua
local M = {}
M.BALL_OUT = hash("ball_out")
return M
```

A local assigned twice, a field set inside a function, and a module that is not in the
project are not constants, and what they hold is not read.

An address is written in the same ways, and also as a `msg.url(...)` of strings, in one
part or three:

```lua
msg.url("/hud#hud")
msg.url("main", "/hud", "hud")
msg.url(nil, nil, "sprite")
hash("/hud")                                -- a hash is an address only for a whole path
```

A problem with an address is reported where its text is. A constant used in ten calls
is reported once, at its declaration, which may be in another file.

## A script is judged where it runs

The same script can be attached to several game objects, and `"#sprite"` means a
different component in each. defold-vet finds every game object that uses the script
and judges the name against each of them. It is reported only when it is wrong for
**all** of them. The same holds for a collection nested in several others, and for a
GUI script run by several scenes.

A script that no game object uses is not judged on what depends on its owner. A Lua
module, which any script may call, is judged against the whole project instead: the
node it asks for has to be in some scene, the animation it plays in some atlas.

## Where an address leads

| The address | Is looked up in |
|---|---|
| `"#sprite"`, `"."`, `"#"` | the game object that runs the script |
| `"paddle"`, `"paddle#sprite"` | the collection the game object sits in, next to it |
| `"/hud#hud"` | the root collection the game object ends up in, however deeply nested |
| `"main:/loader#loader"` | the root collection named `main`, if the project has one |
| `"@render:"`, a socket that is no collection of the project | nowhere: it is left alone |

Three places cannot be followed, and addresses that leave them are not judged:

- **What a factory spawns.** A game object or a collection that is the prototype of a
  factory lands somewhere that is not written down. Only the addresses that stay inside
  it are judged.
- **A collection from a library.** Nothing is said about a path under the id it was
  given.
- **An address the script asks about.** After `go.exists("/frame")`, `"/frame"` is not
  assumed to exist anywhere in that script.

## Code out of sight

Some of what a project runs is not in its files. defold-vet says so to itself and
judges less, instead of guessing.

A project **depends on a library** (`dependencies` in `game.project`), **is a library**
(`include_dirs`), or **has a native extension** (`ext.manifest`). In such a project:

- a message is judged only where the script that receives it is one of the project's;
- a handler is never reported as waiting for a message nobody posts;
- a property on an address that is not written, or on a component drawn with a
  material, is not judged;
- collision groups are not judged;
- a Lua module is not judged for the nodes it asks for or the animations it plays.

A library stops being out of sight once it is fetched. The Defold editor and
`bob resolve` both keep what they fetch in `.internal/lib`, inside the project, and
defold-vet reads it from there: the directories each library shares are read as part of
the project, where Defold mounts them. When every dependency is there, and none of them
brings a native extension, the project is judged in full. Problems inside a library are
not reported, since they are not the project's to fix, and a rename that would have to
change a library is refused.

A **script that does not parse** is read as far as it could be. Its own mistakes are
for the Lua language server; defold-vet only refrains from concluding anything that
would need the part it could not read.

## What Defold is told to ignore

A project can keep files that are not the game's next to the ones that are: the Go
source of a server, whose `.go` files Defold would take for game objects, or what a
tool exports. `.defignore`, in the project's directory, lists what Defold is to leave
alone, and defold-vet reports nothing in those files:

```
/server
/assets/raw
/**/generated
/assets/temp_??.png
```

- A line names a path from the project's directory and has to start with `/`. It
  covers that path and everything under it. Any other line is skipped.
- From Defold 1.14.0 on, `*` stands for any characters within one name, `?` for one,
  and `**` for any number of whole directories. For a project that
  [targets](settings.md#defold) an older version, a line is read as the path it spells,
  which is how that version reads it.
- Nothing is reported in a file that is ignored: not what is wrong inside it, and not
  that [nothing uses it](checks.md#unused-file).
- An ignored file that the game names is still part of the game. Defold's build tool
  builds a game object that a collection places whether `.defignore` names it or not,
  so it is not called missing, and what it uses is not called unused.

`.defunload` looks the same and is another matter. It keeps Defold's editor from
loading part of a large project, and the build still takes all of it, so those files
are read and judged like the rest.

## The engine's side

Which functions take an address, which messages are the engine's and what they carry,
which properties each kind of component has: all of that comes from the reference
documentation Defold publishes for each release, and from the builtin materials and
the builtin input binding of the same release. defold-vet carries the reference of
every version from 1.6.0 on, and [settings](settings.md#defold) say which one a
project uses.

The reference has gaps, and two things are done about them. What a version documents
before and after another one is taken to exist in between: the two camera focus
messages, for instance, are missing from the documentation of every release from 1.8.1
to 1.12.4 and never left the engine. And the gaps that are in every version are filled
by hand: `texture0` on anything drawn with a material, `group` on collision messages,
the material of a model. When a project meets another, it can
[vouch for the name](settings.md#known).

## Why it would rather stay quiet

A report that is wrong costs more than one that is missing: it teaches people to stop
reading the output. Each check therefore reports only what it can stand behind, and the
[decisions](../decisions/README.md) record where each line was drawn. What the checks
leave alone for this reason is exactly what [strict mode](strict-mode.md) lists.
