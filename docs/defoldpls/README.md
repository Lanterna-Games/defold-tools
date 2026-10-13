# defoldpls

defoldpls is the language server of defold-tools. It serves an editor that works on a
[Defold](https://defold.com) project with seven things:

- **The problems [defold-vet](../defold-vet/README.md) finds**, as they are typed.
- **The settings file**, `game.project`: completion and hover from the schema of the
  project's Defold version.
- **The names of the project**: completion, hover and going to a definition for the
  addresses, message ids, properties, GUI nodes, animations, input actions and
  collision groups a script writes as strings, for the fields of the message a handler
  receives, and a fix for a name that is almost right. Every one of those names can be
  found wherever it is written, and renamed there.
- **Building, running and debugging the game**, and reloading a script in the running
  game, with the Defold editor installed on the machine.
- **Making the files of a game**, and keeping an atlas in line with a directory of
  images.
- **The engine API for the [Lua language server](https://luals.github.io)**: completion,
  hover, signatures and types for `go.animate`, `msg.post` and the rest, for the Defold
  version the project targets.
- **The reference of that version, kept on the machine**, served to the browser without
  a network, and linked from every function.

It runs beside the Lua language server and does not replace it. Lua itself is that
server's; what is particular to Defold is this one's.

The Defold editor runs a Lua language server of its own for the project it has open.
From Defold 1.14.0 it can be turned off there, with **Enable built-in Lua language
server** in the editor's preferences, which saves its memory when the code is written
in another editor.

> **State.** The seven things above work.

## Installing

```sh
go install github.com/Lanterna-Games/defold-tools/cmd/defoldpls@latest
```

Or `make install` from a clone, which installs it next to `defold-vet`.

In VS Code, the extension in [`editors/vscode`](../../editors/vscode/README.md) starts
it. Any other editor with a language-server client can start `defoldpls` with no
arguments in the project's directory.

## The names of a project

Inside a string of a script, defoldpls knows what the string is for from where it
stands: the first argument of `msg.post` is an address, the second a message id, the
other side of `message_id ==` a message that is handled, the argument of
`gui.get_node` a node. The same holds inside `hash("...")`. Outside a string it knows
one more place: after the dot of the message `on_message` receives.

| Where the cursor is | What is offered |
|---|---|
| An address | The components of the game object that runs the script, as `#sprite`; the game objects of its collection, next to it and from the root; after a `#`, the components of the game object named before it |
| The id of a message that is posted | What the script at the address handles, and the engine's messages if the address is a component that takes them. When the address is not written, what any script handles |
| The id of a message that is handled | What the scripts of the project post, and the engine's messages |
| A property, in `go.get`, `go.set`, `go.animate` | The properties of what the address names: the engine's for its kind of component, and the ones its script declares |
| A GUI node | The nodes of the scenes that run the script, templates included |
| An animation | The animations of the atlas or tile source of the component played, or of the textures of the scene for a node |
| An input action | The actions the game's input binding binds |
| A collision group | The groups some collision object is in |
| A field, after `message.` in `on_message` | The fields of the message handled where the cursor is: those the engine sends with it, with their types, and those the posts of the project set |

VS Code does not suggest inside strings unless told to. The extension turns that on for
Lua, as a default that your own settings override; any of `"`, `'`, `#`, `/`, `:` and
`.` also asks for the list.

Which message is handled where the cursor is comes from the tests around it:

```lua
function on_message(self, message_id, message, sender)
    if message_id == hash("collision_response") then
        message.                -- other_id, other_position, other_group, own_group
    elseif message_id == SERVE then
        message.                -- what the posts of "serve" set
    end
end
```

A handler that leaves early, with `if message_id ~= hash("hit") then return end`, is
read the same way for what follows. Where no test says which message it is, nothing is
offered.

**Hover** says what the name is: the kind and file of a component with what its script
handles and declares, the game object with its components, where a message is handled
and where it is posted from, with the fields each post sets, and the file and line
that declare a node, an animation, an action or a group. A name that leads nowhere
says so.

A message or a property of the engine comes with what the reference of the project's
Defold version says about it: its description, the fields of the message with their
types, its examples, and the two links the engine API has, to the offline reference
and to defold.com. A name that several kinds of component have is explained for the
kind the address names: `scale` on `"#sprite"` is the sprite's, and on `"."` the game
object's. Where the address cannot be read, each kind is listed with its links. The
same text comes with each name in the completion list, without the examples.

A field under the cursor says its type and what it holds, or where the project sets it.

**Go to definition** leads to the declaration:

| From | To |
|---|---|
| An address | The line of the collection or game object file that declares the game object or the component |
| A message that is posted | The handlers of it in the script at the address |
| A message that is handled | Every place that posts it |
| A property | The `go.property` of the script at the address |
| A node, an animation, an action, a group | The line of the `.gui`, the atlas or tile source, the input binding or the collision object that declares it |
| A field of a message of the project | The posts that set it |
| The path of a file, in a script or in any data file | The file |

A data file is mostly a list of other files: a game object names the script of a
component, a collection the game objects it places, an atlas its images. With the cursor
on such a path, going to the definition opens the file it names:

```
components {
  id: "script"
  component: "/pickups/feedback.script"
}
```

This works where the path is written plainly and where it is inside a component or an
object written in place, with its quotes escaped. It leads only to a file that is in the
project and on disk: a file of the engine, under `/builtins`, or of a library that was
fetched, has no place to open.

**A fix** is offered on a problem that says what was meant. It replaces the part of the
text that is wrong and leaves the rest: in `"/game#gaem"`, only `gaem`.

### Finding and renaming a name

**Find all references** lists every place that writes a name, and **Rename** changes all
of them at once, data files as well as scripts:

| The name | Where it is found and changed |
|---|---|
| A message id | Its posts and its handlers |
| An input action | The comparisons in `on_input` and the triggers that bind it |
| A collision group | The group and the masks of collision objects and tile sources, and the scripts that name it |
| A GUI node | Its declaration, the nodes it is the parent of, the layouts, the ids a template gives its children in other scenes, and `gui.get_node` |
| A component | Its declaration in the game object, the addresses that lead to it, what a collection sets on it, and addresses given as script properties |
| A game object | Its declaration in the collection, the `children` that list it, the paths that lead to it |
| A collection placed in another | Its id in the collection that places it, and that part of every path that goes through it: `area` in `"/area/gate#script"` and in `"area/yard/tree"` |
| An animation | Its declaration in the atlas or tile source, the default animation of sprites, the textures of GUI nodes, particle emitters, and where it is played |
| A script property | `go.property`, `self.name` in that script, `go.get`, `go.set` and `go.animate` on it, and what collections and game objects set |

```
main/hero.script            if action_id == hash("left") then      ->  hash("move_left")
input/game.input_binding      action: "left"                       ->  action: "move_left"
```

A name is often a part of a longer text, and only the part is changed: `body` in
`"/enemy#body"`, `label` in `"restart/label"`, `idle` in `texture: "hero/idle"`,
`speed` in `"speed.x"`. A name kept in a constant is changed where its text is, in the
module that declares it if that is where it lives.

The same text can name different things. Two game objects can each have a component
called `body`, two atlases an animation called `idle`, two scripts a property called
`speed`. What is renamed is the thing, found by following each address to the game
object it leads to from every place its script runs. `"#body"` in the enemy's script
and `"/enemy#body"` in the hero's are the same component; `"#body"` in the hero's is
another, and is left alone.

defoldpls does not write the files. It answers with the edits and the editor applies
them, so a rename can be previewed first and undone after. A data file is changed by
replacing the name where it stands, and nothing else in it is touched. [Decision
0014](../decisions/0014-defoldpls-may-change-a-project.md) records the rules.

A rename is refused, with the reason, when it could not be complete or would break
something:

| The rename | Why it is refused |
|---|---|
| Of a message of the engine, or of an action the engine's own binding binds | It is not the project's to rename |
| Of an action no binding of the project binds | The script would be left comparing with a name nothing sends |
| Of an animation that is an image of an atlas | It takes its name from the file |
| Onto a name that exists | Two things would become one |
| Of a text that means one thing where its script runs for one owner and another thing for another: `"#sprite"` in a script that several game objects use, each with its own sprite | Renaming one would break the others |
| Of a text that also serves as something else, a constant used as a message id and as a node id for instance | Renaming one would rename the other |
| When the checks read a use the rename cannot find | Some place would keep the old name |
| When a file changed since the project was last read | The edit would land on other text |
| To a name that would not survive where it is written: a quote in any name, a `#` or a `/` in a component, a game object or a collection, a space in a script property | The text around it would be read differently |

A name can also reach its use where nothing says what it is for: the action a paddle
answers to, set as a script property in a collection; a relative address in a Lua
module, which runs for no game object; `self.speed` in a module that is handed the
instance; the properties given to `factory.create`. Such places are not changed, and
after the rename a warning lists each one that holds the same text.

Renaming where there is no name of the project is left to the Lua language server, so
a local variable is still its to rename.

A script is read [where it runs](../defold-vet/how-it-reads.md#a-script-is-judged-where-it-runs).
One that several game objects use is offered what belongs to any of them, and a script
that nothing uses is offered nothing that depends on its owner. The names are those of
the last check, a fifth of a second behind the keyboard.

## Data files

A game object, a collection, an atlas and the other data files of Defold are text, and
writing one by hand is mostly writing the names of other things. Inside the quotes of a
value, defoldpls offers what the field takes:

| The field | What is offered |
|---|---|
| `component`, `prototype`, `collection`, `image`, `tile_set`, `texture`, `material`, `font`, `script` and the other fields that name a file | The files of the project of the kinds that field takes |
| `default_animation` | The animations of the atlas or tile source the same component names |
| `children` | The game objects of the collection being written |
| Any other field, once the value starts with `/` | Every file of the project |

A choice replaces the whole value. It works the same inside a component or an object
written in place, where the quotes are escaped.

**Hover** over the path of a file says what the file is: the components of a game
object, what a collection holds, the animations of an atlas, the properties of a script.
**Go to definition** opens it, and **a fix** is offered on a problem that says what was
meant, as in a script.

## The settings file

`game.project` is a form in the Defold editor. As text, defoldpls gives it what the
form gives:

- **completion** of sections after a `[`, of the keys of the section that are not set
  yet, and of the values of a setting that takes one of a few;
- **hover** on a key, with what it is for, its kind and its default;
- **go to definition** from a file a setting names, compiled name or not;
- the problems [defold-vet](../defold-vet/checks.md#unknown-setting) finds there, with
  the fix when a key is almost a real one.

All of it follows the schema of the project's Defold version, the one Defold's build
tool carries.

## Building and running the game

```sh
defoldpls build      # build the game
defoldpls run        # build it and run it, printing what it logs
defoldpls reload main/ball.script
defoldpls resolve    # fetch the libraries the project depends on
defoldpls clean      # delete what was built
defoldpls bundle --platform wasm-web
```

These use the Defold editor installed on the machine: its Java, the build tool inside
it, and its engine. Nothing is downloaded. The editor is found where it says it was
installed; `DEFOLD_HOME`, or `--defold-home`, names another one. [Decision
0015](../decisions/0015-the-game-is-built-by-the-installed-editor.md) says why.

A build prints what can be acted on, in the form editors read:

```
main/ball.script:27:1: token recognition error at: '?'
defoldpls: the build failed
```

`bundle` builds the game for one platform and packs it, ready to hand out:

```
bundled for wasm-web into dist/wasm-web
```

- `--platform` is one of Defold's: `x86_64-win32`, `x86_64-linux`, `arm64-macos`,
  `x86_64-macos`, `wasm-web`, `js-web`, `arm64-android`, `armv7-android`, `arm64-ios`.
  Without it, the platform of this machine.
- `--variant` is `release`, the default, or `debug`.
- `--output` is where the bundle goes, `dist/<platform>` in the project by default.
  Defold's build tool refuses a directory under `build`. A project that keeps bundles
  inside it does well to name that directory in
  [`.defignore`](../defold-vet/how-it-reads.md#what-defold-is-told-to-ignore).
- What comes after `--` goes to Defold's build tool as it is: `-- --bundle-format aab`,
  a keystore, an identity for iOS.

The bundle is built apart from the build that runs, under `build/bundling`, so that a
release for another platform does not replace what the next run needs.

`run` keeps printing what the game logs until it is stopped. While it runs, `reload`
builds again and has the game take the named files again, without restarting it:

```
INFO:RESOURCE: /main/ball.scriptc was successfully reloaded.
```

A reload costs a build, which takes a few seconds even when one file changed.

In VS Code these are **Defold: Build**, **Defold: Build and Run**, **Defold: Hot Reload
This File** and **Defold: Fetch Libraries**. Build errors and the script errors the
running game logs go to the Problems panel, and a script saved while the game runs is
reloaded, unless `defoldTools.reloadOnSave` is off.

A project with native extensions needs an engine of its own, with the extensions in
it. Defold's build tool sends them to Defold's build server, as the editor does, and
leaves the engine that comes back in `build/<platform>`. A build that has to wait for
one says where it asked:

```
INFO: Sending async build request to https://build.defold.com/build_async/x86_64-win32/a81dbb0478e8b73931dc2d671563204f6ac0ef82
```

`run` and the debug adapter start that engine when it is there, and say that they do.
The build tool removes it once the project has no extension left, and the game is back
on the engine of the editor. An extension that does not compile is a build that
failed, said once however many times the compiler repeats it:

```
myext/ext.manifest:1:1: 2 warnings and 1 error generated.
myext/src/myext.cpp:8:1: use of undeclared identifier 'nothing_here
    8 |     return 1 + nothing_here;'
```

[Decision 0024](../decisions/0024-a-game-with-extensions-runs-on-the-engine-built-for-it.md)
says what this rests on.

### Debugging

`defoldpls dap` is a debug adapter: it speaks the Debug Adapter Protocol on standard
input and output, builds the game, runs it and attaches to the debugger the engine
itself carries. In VS Code it is **Defold: Build and Debug**, or F5 with no
`launch.json` at all.

- **Breakpoints** in scripts and Lua modules, set before the game starts or while it
  runs.
- **The stack** where the game stopped, with the locals of each function and what the
  tables among them hold, `self` included.
- **Expressions** evaluated where it stopped, in the debug console or on hover.
- **Step over, into and out**, continue and pause.
- What the game logs, in the debug console.

Values are read three levels deep: what lies deeper is shown as text. [Decision
0016](../decisions/0016-the-game-is-debugged-by-its-own-engine.md) says how it is done
and what it rests on.

For another editor, the adapter takes these in its `launch` request:

| | |
|---|---|
| `root` | The directory of the project |
| `defoldHome` | Where the Defold editor is installed, when it is not the one launched last |
| `built` | `true` to debug what is already built, without building |
| `engine` | An engine to run in place of the one built for the project, or the editor's |

## Making files

```sh
defoldpls new main/enemy.script
defoldpls new --script main/enemy.go        # a game object, with its script attached
defoldpls new --script main/hud.gui         # a GUI scene, with its script
defoldpls new main/physics.lua
defoldpls new levels/cave.collection
defoldpls new --from main/images main/sprites.atlas
defoldpls new --from main/tiles.png main/tiles.tilesource
defoldpls new --from main/tiles.tilesource main/level.tilemap
defoldpls new --from main/bullet.go main/bullets.factory
```

The kind of file is its extension. What is made is printed, and nothing is ever written
over a file that is there. Data files are written the way the Defold editor writes
them; a project made of nothing but these was built by Defold's own build tool.

| Made as it is | Made from another file, named with `--from` |
|---|---|
| `.script`, `.gui_script`, `.render_script`, `.lua` | `.atlas` from a directory of images |
| `.go` and `.gui`, with `--script` for their script | `.tilesource` from an image |
| `.collection` | `.tilemap` from a tile source |
| `.label` | `.sprite` from an atlas or a tile source |
| `.collisionobject` | `.factory` from a game object |
| `.input_binding` | `.collectionfactory` and `.collectionproxy` from a collection |
| | `.sound` from a `.wav` or an `.ogg`, `.font` from a `.ttf` or an `.otf` |

Without `--from`, a file of the second column is made with the path left empty, to be
[filled in](#data-files). `defoldpls new --kinds` prints the list as JSON, for an editor
to offer. In VS Code this is **Defold: New File…**, also on a folder of the explorer.

```sh
defoldpls atlas main/sprites.atlas
```

brings an atlas in line with the images on disk: those in the directories its images
come from are added, and those whose file is gone are taken out. An image that one of
its animations already has as a frame is in the atlas, and is not added again.

```
- /main/images/old_wall.png
+ /main/images/net.png
```

An image is added after the last one and taken out with its block, and nothing else in
the file is touched: animations, margins and what was set on other images stay as they
were. `--from <directory>` looks there, and below, instead. In VS Code this is
**Defold: Bring This Atlas in Line with the Images on Disk**.

## Moving a file

A file is named by its path in every file that uses it, so moving one by hand breaks
all of them.

```sh
defoldpls move main/ball.go objects/ball.go
defoldpls move main/images assets/pictures
```

moves a file, or a directory with all it holds, and writes the new path wherever the
old one was:

```
~ main/main.collection
main/ball.go -> objects/ball.go
```

- A path is found where it is written plainly, inside a component or an object written
  in place, under the compiled name `game.project` uses (`/main/main.collectionc`), and
  in the strings and comments of scripts.
- A Lua module is also followed by the dotted name scripts `require` it by:
  `require("main.ids")` becomes `require("shared.names")`.
- Nothing is rewritten and nothing moves when the target is there already, or when a
  file changed between being read and being written.
- An image gives its name to its animation in an atlas. Giving an image another name
  therefore renames the animation, which is said and not followed into what plays it;
  moving an image to another directory under its own name changes nothing.
- A path a script builds while the game runs cannot be found, and is left as it is.

In an editor nothing has to be asked for: when a file is renamed or moved in the
explorer, the editor asks defoldpls what else has to change and applies the edits with
the move.

```sh
defoldpls uses main/sprites.atlas
```

prints every place that names a file, as `file:line:column`. In VS Code it is **Find All
References** on a path, and **Defold: Show What Uses This File** for the file that is
open.

## Looking at an atlas

An atlas lists images and says nothing of where they go. Defold packs them into a page
each time it builds, and only the build knows the result.

```sh
defoldpls pack main/sprites.atlas
```

has Defold's build tool pack the atlas and prints where each image landed, as JSON:

```json
{
  "atlas": "/main/sprites.atlas",
  "root": "/home/you/game",
  "defold": "1.14.0",
  "width": 128,
  "height": 64,
  "pages": 1,
  "images": [
    {
      "image": "/main/images/house.png",
      "name": "house",
      "line": 2,
      "page": 0,
      "x": 2,
      "y": 2,
      "width": 64,
      "height": 48,
      "rotated": false
    },
    {
      "image": "/main/images/coin.png",
      "name": "coin",
      "line": 5,
      "page": 0,
      "x": 68,
      "y": 2,
      "width": 16,
      "height": 16,
      "rotated": false
    }
  ]
}
```

- `x` and `y` are counted from the top left of the page, and `width` and `height` are
  what the image takes there. An image the build turned to make it fit is `rotated`: it
  lies a quarter turn clockwise, so its width is the height of its file.
- `line` is the line of the atlas that names the image, and `animation` the animation
  it is a frame of, when it is one.
- `pages` is more than one for an atlas too big for a page; every page has the same
  size.

The atlas is packed away from the game, in a project of its own in the cache that holds
the atlas, its images and nothing else. So an atlas nothing uses yet is packed all the
same, a mistake elsewhere in the game does not stand in the way, and nothing is written
in the game. It takes the editor that is [installed](#building-and-running-the-game),
and a few seconds. [Decision
0020](../decisions/0020-an-atlas-is-shown-as-the-build-packs-it.md) says why it is not
worked out here instead.

In VS Code this is **Defold: Show What This File Draws**, which draws the page next to
the text of the atlas.

## Looking at tiles

A tile source and a tile map need no build to be drawn: everything is in their text.

```sh
defoldpls tiles main/ground.tilesource
defoldpls tiles main/level.tilemap
```

A tile source cuts one image into a grid, by the size of a tile, the margin around each
one and the spacing between them. `tiles` prints the image, its size and where each
tile lies in it, counted as Defold counts them:

```json
{
  "kind": "tilesource",
  "file": "/main/ground.tilesource",
  "root": "/home/you/game",
  "source": {
    "file": "/main/ground.tilesource",
    "image": "/main/ground.png",
    "width": 32,
    "height": 16,
    "tileWidth": 16,
    "tileHeight": 16,
    "columns": 2,
    "rows": 1,
    "tiles": [
      [0, 0],
      [16, 0]
    ],
    "animations": [
      {
        "id": "water",
        "first": 0,
        "last": 1,
        "line": 4,
        "lastLine": 8
      }
    ]
  }
}
```

For a tile map it prints the same `source`, read from the tile source the map names,
and then its layers and how far its cells reach:

```json
{"layers": [{"id": "ground", "z": 0, "visible": true, "line": 2, "cells": [[0, 0, 1, 1, 6, 12]]}], "bounds": {"minX": 0, "minY": 0, "maxX": 0, "maxY": 0}}
```

- Tiles are counted from nothing here, in `tiles`, in `first` and `last`, and in a
  cell. The text of a tile source counts the tiles of an animation from one, and the
  text of a tile map counts the tile of a cell from nothing, which is how Defold reads
  each.
- A cell is six numbers: its place across and up, its tile, what was done to the tile,
  and the first and last line that write it. What was done is a sum: 1 for mirrored
  across, 2 for mirrored up and down, 4 for a quarter turn, which Defold makes
  clockwise after the mirroring.
- A cell that names a tile the source does not have is left out and counted in
  `cellsWithoutATile`.
- `--stdin` reads the file from standard input, as an editor holds it before it is
  saved. The tile source and the image a map leads to are still read from the project,
  from a library it fetched if that is where they are; an image that is not a file on
  disk comes whole, as `imageData`.

[Decision 0021](../decisions/0021-tiles-are-drawn-from-their-text.md) says how each of
these was settled. In VS Code the same command as for an atlas draws them, and follows
what is typed.

## Looking at a game object or a collection

Where a game object ends up is written across several files: a collection places it, a
parent moves, turns and scales it, a collection inside another does the same to all it
holds, and the object places its own components.

```sh
defoldpls scene main/hero.go
defoldpls scene main/main.collection
```

reads all of it and prints every component with the place it ends up in:

```json
{
  "kind": "scene",
  "file": "/main/small.collection",
  "root": "/home/you/game",
  "images": [
    {
      "path": "/main/wall.png"
    }
  ],
  "maps": [],
  "items": [
    {
      "kind": "sprite",
      "owner": "/wall",
      "component": "sprite",
      "type": "sprite",
      "across": [2, 0, 0, 2, 40, 10],
      "depth": 0.5,
      "line": 2,
      "lastLine": 15,
      "image": 0,
      "from": [0, 0, 64, 20],
      "width": 64,
      "height": 20,
      "map": -1,
      "alpha": 1
    }
  ],
  "uses": [
    "/main/prop.go",
    "/main/sprites.atlas",
    "/main/wall.png"
  ],
  "objects": 1,
  "cutShort": false,
  "minX": -24,
  "minY": -10,
  "maxX": 104,
  "maxY": 30
}
```

- `across` places what an item draws: a point `x, y` of the item, counted from its
  middle with `y` upwards, lands at `a·x + c·y + e, b·x + d·y + f` for the six numbers
  `a` to `f`. `depth` is how far to the front it is, and the items come from the back.
- What is drawn is what the text gives:

  | `kind` | From | Drawn as |
  |---|---|---|
  | `sprite` | a sprite | the part `from` of the image `image`, `width` by `height`: the image of its animation, or its first frame, placed by the pivot the atlas gives the image, mirrored across when `flip` has 1 and up and down when it has 2, and with the edges `slice9` names kept when the sprite is stretched: left, top, right, bottom |
  | `tilemap` | a tile map | the cells of `maps[map]`, in the form [`tiles`](#looking-at-tiles) prints |
  | `label` | a label | its box, with its `text` |
  | `box`, `sphere`, `capsule` | a collision object | one for each plain shape it has |
  | `marker` | anything else | a mark where it is, with its `type` |

- `owner` is the game object, named as the engine names it through the collections it
  came by, and `line` and `lastLine` are the lines of the file asked about that brought
  the item there.
- `uses` are the files the scene was read from, for an editor to watch.
- A scene stops at 20,000 game objects, or a million cells of tile maps, and says so
  with `cutShort`.
- `--stdin` reads the file from standard input, as for [tiles](#looking-at-tiles).

It is a layout and not the game: materials, tints, fonts, particles, models and what is
made while the game runs are Defold's to show. [Decision
0022](../decisions/0022-a-scene-is-laid-out-from-its-text.md) says where the line is
drawn, and how the places were checked against the engine.

## Looking at a GUI scene

A GUI scene is laid out by the same command:

```sh
defoldpls scene main/hud.gui
```

Each node comes as an item, placed in the display of the game:

```json
{
  "kind": "scene",
  "file": "/main/hud.gui",
  "root": "/home/you/game",
  "images": [
    {
      "path": "/main/wall.png"
    }
  ],
  "maps": [],
  "items": [
    {
      "kind": "display",
      "owner": "",
      "component": "",
      "type": "display",
      "across": [1, 0, 0, 1, 640, 360],
      "depth": -1,
      "line": 0,
      "lastLine": 0,
      "image": -1,
      "from": [0, 0, 0, 0],
      "width": 1280,
      "height": 720,
      "map": -1,
      "alpha": 1
    },
    {
      "kind": "sprite",
      "owner": "bar",
      "component": "",
      "type": "box",
      "across": [1, 0, 0, 1, 640, 50],
      "depth": 0,
      "line": 5,
      "lastLine": 15,
      "image": 0,
      "from": [0, 0, 64, 20],
      "width": 64,
      "height": 20,
      "map": -1,
      "alpha": 1,
      "tint": "#ffffff"
    }
  ],
  "uses": [
    "/game.project",
    "/main/sprites.atlas",
    "/main/wall.png"
  ],
  "objects": 1,
  "display": [1280, 720],
  "cutShort": false,
  "minX": 0,
  "minY": 0,
  "maxX": 1280,
  "maxY": 720
}
```

- The first item is the `display` the game asks for in `game.project`, 960 by 640 when
  it asks for none, with its lower left corner at 0, 0. `display` gives its size again.
- A node is placed in its parent by its position, its rotation and its scale, and its
  box lies around that place as its pivot says: the node above stands on the middle of
  its lower edge, at 640, 40, so the middle of its box is at 640, 50. `owner` is the id
  of the node.

  | `kind` | From | Drawn as |
  |---|---|---|
  | `sprite` | a box node with a texture | the part `from` of the image, stretched to `width` by `height`, with the edges `slice9` names kept as they are: left, top, right, bottom. The pivot of the image moves it and `flip` mirrors it, as for a sprite |
  | `panel` | a box node without one | its box, in its colour |
  | `label` | a text node | its `text`, `textSize` high when the font is a file of the project, leaning to the side `leanX` and `leanY` name |
  | `pie` | a pie node | an ellipse, or a rectangle when `square`, open in the middle by `inner` and filled over `sweep` degrees |
  | `marker` | a particle effect, or a node an extension adds | a mark where it is, with its `type` |

- `tint` is the colour of the node and `alpha` how solid it is, with what it takes from
  its parent when it inherits. `off` marks a node the game does not show: one that is
  not enabled, or is under one that is not, or is not visible.
- The items come in the order they are drawn: layer by layer in the order the scene
  lists its layers, `layer` being the one a node is in or inherits, and inside a layer
  in the order of the tree. `depth` counts the layers.
- A template node brings the nodes of the scene it names, under its id. Each takes from
  the scene that holds the template the fields that scene overrides, and the rest from
  the template, as the build does. `line` is where the node is written in the file
  asked about, or the line of the template node when it is not written there.
- A scene stops at 20,000 nodes, and says so with `cutShort`.

The layout is the default one, at the size of the display, where anchors and adjust
modes move nothing. Fonts, materials, clipping, particles and the other layouts of the
scene are Defold's to show. [Decision
0025](../decisions/0025-a-gui-is-laid-out-from-its-text.md) says how the places were
checked against the engine.

## The engine API

Defold publishes the documentation of every release as data. defoldpls turns the
documentation of the project's version into annotation files the Lua language server
reads, and tells the editor where they are. Hovering `go.animate` then shows what it
does, its parameters with their types, its examples, and two links:

```
[Offline reference](http://127.0.0.1:34751/1.14.0/go#go.animate) · [defold.com](https://defold.com/ref/go-lua#go.animate:url-property-playback-to-easing-duration-delay-complete_function)
```

The files are written to the user's cache directory, under
`defold-vet/annotations/<version>`, and the Lua language server has to be pointed at
that directory with its `Lua.workspace.library` setting. The VS Code extension offers to
do that; nothing is written into a project without the question. Outside VS Code:

```sh
defoldpls annotations            # writes them and prints the directory
defoldpls annotations --output api --defold 1.12.0
```

The annotations are generated for every version from 1.6.0 on. Defold's own, which
exist from 1.11.0, are not used: generating them here gives every supported version the
same files and the same links.

Two parts of the documentation are left out. The Lua standard library, which the Lua
language server knows already, and the editor API, which is for editor scripts and not
for the game.

## The offline reference

```sh
defoldpls docs
```

prints an address such as `http://127.0.0.1:34751/1.14.0/` and serves the API reference
there until it is interrupted. When defoldpls serves an editor it does the same on its
own, and the links in the annotations lead to it.

- **It listens on this machine only**, on a port the system hands out. No fixed port is
  taken, so it cannot be one of the ports other programs are used to. The port is
  remembered, so that a link written yesterday still works today.
- **Two editors share one reference.** The second finds the first and uses it, and takes
  the port up when the first one leaves.
- **Every version of the window is there.** The page has a version switch.

The documentation of the newest Defold version is carried by the program. That of
another version is fetched from Defold's archive the first time it is asked for and
kept in the cache. To have it before going somewhere without a network:

```sh
defoldpls fetch 1.12.0 1.9.8
```

## Commands

| Command | |
|---|---|
| `defoldpls` | Serve an editor over the Language Server Protocol, on standard input and output |
| `defoldpls docs [directory]` | Serve the offline reference and print its address |
| `defoldpls annotations [directory]` | Write the annotations and print where they are |
| `defoldpls fetch <version>...` | Fetch the documentation of Defold versions |
| `defoldpls build [directory]` | Build the game |
| `defoldpls run [directory]` | Build the game and run it |
| `defoldpls reload <file>...` | Build again and have the running game take the files again |
| `defoldpls resolve [directory]` | Fetch the libraries the project depends on |
| `defoldpls bundle [directory]` | Build the game for a platform and pack it, ready to hand out |
| `defoldpls clean [directory]` | Delete what was built, so that the next build starts from nothing |
| `defoldpls dap` | Serve an editor that debugs the game, over the Debug Adapter Protocol |
| `defoldpls new <file>` | Make a file of a game, of one of seventeen kinds |
| `defoldpls move <from> <to>` | Move a file or a directory, and write its new path wherever it is named |
| `defoldpls uses <file>` | Print every place that names a file |
| `defoldpls atlas <file>` | Add the images on disk to an atlas, and take out those that are gone |
| `defoldpls pack <file>` | Pack an atlas as the build does and print where each image lands |
| `defoldpls tiles <file>` | Print the tiles of a tile source, or the cells of a tile map |
| `defoldpls scene <file>` | Print what a game object, a collection or a GUI scene places, and where |
| `defoldpls version`, `defoldpls help` | |

`docs` and `annotations` take `--defold <version>` to use that version instead of the
one the project declares in [`defold-vet.toml`](../defold-vet/settings.md#defold).

## For editors

Besides the standard notifications, defoldpls sends one of its own when the annotations
and the reference are ready:

```json
{
  "method": "defold/annotations",
  "params": {
    "defold": "1.14.0",
    "directory": "/home/you/.cache/defold-vet/annotations/1.14.0",
    "reference": "http://127.0.0.1:34751/1.14.0/"
  }
}
```

A client that does nothing with it loses nothing but the convenience: the same
directory can be set by hand, and the same address is printed by `defoldpls docs`.

It answers `textDocument/completion`, `definition`, `hover`, `codeAction`, `references`,
`prepareRename` and `rename`, for scripts and for `game.project`. For a data file it
answers `completion`, `definition`, `hover`, `codeAction` and `references`, and `null`
to the rest. `references` on the path of a file gives every place that names the file.

It also answers `workspace/willRenameFiles`, with the edits that follow a file or a
directory to its new path, and a request of its own, `defold/uses`, which takes
`{"textDocument": {"uri": …}}` and answers the locations that name that file. `prepareRename` answers `null` where
there is no name of the project, and an error where there is one that cannot be renamed. A
completion item carries the range it replaces, since a name may hold characters an
editor takes for the end of a word, and its documentation as Markdown.

The documentation and the annotations are those of the version the project declared
when the editor opened it. After changing `defold` in `defold-vet.toml`, start the
server again: until then the problems follow the new version, and the hovers leave out
what the reference says rather than quote another version.
