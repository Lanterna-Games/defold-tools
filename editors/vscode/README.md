<!-- markdownlint-disable -->

<p align="center">
  <img src="icon.png" alt="logo" width="120">
</p>

<h1 align="center">Defold Tools</h1>

<p align="center"><em>Write Defold games from Visual Studio Code</em></p>

<p align="center">
  <a href="https://github.com/Lanterna-Games/defold-tools"><img src="https://img.shields.io/badge/Defold-1.6.0%20and%20newer-FF7A1A?style=for-the-badge&colorA=263238&colorB=FF7A1A" alt="Defold 1.6.0 and newer"></a>
  <a href="https://github.com/Lanterna-Games/defold-tools"><img src="https://img.shields.io/badge/Dependencies-none-FF7A1A?style=for-the-badge&colorA=263238&colorB=FFB21E" alt="No dependencies"></a>
  <a href="https://github.com/Lanterna-Games/defold-tools/blob/main/LICENSE"><img src="https://img.shields.io/badge/License-WTFPL-FF7A1A?style=for-the-badge&colorA=263238&colorB=FFD215" alt="WTFPL"></a>
</p>

<br />

<p align="center">A Defold project names almost everything by text: addresses, messages, properties, nodes, animations.<br>Defold Tools reads the whole project and treats those names the way a compiler would.</p>

<br />

### Problems

<details><summary>🔎 <b>Show the twenty-one kinds of problem it finds</b></summary><br/>

| Check | In one line |
|---|---|
| `unresolved-url` | An address that leads to no game object or component |
| `unhandled-message` | A message posted to something that does not handle it |
| `unposted-message` | A handler waiting for a message nothing posts |
| `unknown-message-field` | A field read from a message that does not carry it |
| `unknown-property` | A property its game object or component does not have |
| `undeclared-property` | A script property the script does not declare |
| `unassigned-self-field` | A field of `self` that is read and never assigned |
| `unbound-action` | An input action no binding binds |
| `unknown-collision-group` | A collision group nothing is in |
| `unknown-gui-node` | A GUI node the scene does not have |
| `unknown-animation` | An animation the atlas or tile source does not have |
| `missing-resource` | A file named in a data file or a setting that is not in the project |
| `unknown-child` | A child that is not in the collection |
| `unused-file` | A file nothing in the project uses |
| `unknown-setting` | A section or a key of `game.project` that is almost a real one |
| `invalid-setting` | A value of `game.project` of the wrong kind |
| `unset-setting` | A setting a build needs that is not set and has nothing to fall back on |
| `removed-setting` | A setting Defold removed in the version the project targets |
| `syntax` | A data file that cannot be read |
| `unverifiable` | Strict mode: a name that cannot be read |
| `ignore` | An ignore comment that is not in order |

</details>

### Names

<details><summary>🔤 <b>Show the names it completes, explains, finds and renames</b></summary><br/>

| Name | Where |
|---|---|
| Addresses | `msg.post("#sprite", ...)`, `go.get("/hud#score", ...)`, `msg.url(...)` |
| Message ids | Where they are posted and where `on_message` compares them |
| Fields of a message | After `message.` in `on_message` |
| Properties | `go.get`, `go.set`, `go.animate`, `go.property` and `self.name` |
| GUI nodes | `gui.get_node("restart/label")`, through templates |
| Animations | `sprite.play_flipbook`, `gui.play_flipbook`, `play_animation` |
| Input actions | Where `on_input` compares them |
| Collision groups | Masks, ray casts and collision messages |
| Game objects, and collections placed in collections | The paths that lead to them and through them: `"/area/gate#script"` |
| Settings | Sections, keys and values of `game.project` |

</details>

<br />

## Table of Contents

- [Features](#features)
- [Getting Started](#getting-started)
- [Customization](#customization)
- [Commands](#commands)
- [Debugging](#debugging)
- [How it works](#how-it-works)
- [Not there yet](#not-there-yet)
- [Support](#support)
- [Related](#related)

## Features

- Problems while you type, in the editor and in the Problems panel, with a quick fix when a name is almost right
- Completion, hover and go to definition for the names of your project, inside strings
- In game objects, collections and the other data files: completion of files and names, hover and go to definition on a path, and the same quick fixes
- Find All References and Rename Symbol, across scripts and data files at once
- Renaming or moving a file in the explorer rewrites every place that names it
- The engine API for the Lua language server, for the Defold version your project targets
- The reference of that version, kept on the machine and opened without a network
- Build, run and hot reload, and a debugger with breakpoints, the stack and variables
- Atlases, tiles, game objects, collections and GUI scenes drawn next to their text
- No dependencies: the extension starts one program and shows what it sends back

## Getting Started

1. **Install `defoldpls`**<br>
  It is the program that does the work. With Go installed: `go install github.com/Lanterna-Games/defold-tools/cmd/defoldpls@latest`. If it is not in the `PATH` of the editor, [say where it is](#the-program-to-run). The [README of the repository](https://github.com/Lanterna-Games/defold-tools#installing-the-programs) goes through it step by step, for Linux, macOS and Windows.

2. **Install the extension**<br>
  From a clone of [defold-tools](https://github.com/Lanterna-Games/defold-tools): `make vsix`, then `code --install-extension build/defold-tools.vsix`.

3. **Open a Defold project**<br>
  Any folder with a `game.project`. Problems appear as soon as it is read.

4. **Let the Lua language server know the engine**<br>
  The first time, the extension offers to point the [Lua language server](https://luals.github.io) at the API of your Defold version. Accepting writes `Lua.workspace.library` in the settings of that folder, and `Lua.runtime.version` when nothing says which Lua it is. Nothing is written without the question, and until it is answered `go`, `msg`, `hash` and the rest are [not called undefined](#the-globals-of-defold).

5. **Press F5**<br>
  The game is built, started and stopped at your breakpoints. This uses the Defold editor installed on the machine: its Java, its build tool and its engine.

## Customization

### The program to run

`defoldpls` is looked up in the `PATH`. To use one somewhere else:

```json
"defoldTools.server.path": "C:\\Tools\\defold-tools\\defoldpls.exe"
```

### The Defold editor to build with

Building, running and debugging use the Defold editor that was launched last. To use another one:

```json
"defoldTools.defoldHome": "C:\\Tools\\defold"
```

### The view of what a file draws

Opening one of these shows it as a picture next to its text:

| File | What is drawn |
|---|---|
| `.atlas` | The page, with every image where Defold's build puts it |
| `.tilesource` | The image under its grid, each tile with its number, the tiles of an animation outlined |
| `.tilemap` | The cells, layer by layer in the order of their depth, mirrored and turned as written |
| `.go` | The components of the game object where it places them |
| `.collection` | Every game object where the collection, its parents and the collections it came through place it |
| `.gui` | Every node in the display of the game, where its parents and its pivot place it, the nodes of templates included |

A game object and a collection are shown as a layout, not as the game: a sprite is the image of its animation, placed by its pivot, flipped and stretched as written, a tile map its cells, a label its box, a collision object its shapes, and anything else, a script, a model, a particle effect, is a mark where it is. Materials, fonts and what only exists while the game runs stay with Defold.

A GUI scene is a layout as well: a box is its image, stretched as the node says with the edges it keeps, or its colour when it has no image; a text is its words at the size of its font; a pie is its shape; and what the game would not show, a node that is not enabled or not visible, is a dashed outline. It is the default layout at the size of the display in `game.project`, in the order the layers and the tree draw it.

Hovering says what is under the pointer; clicking selects the line that writes it, and what the line under the cursor writes is outlined. The wheel zooms, dragging moves the picture, and a double click fits it again. To have the view only when asked, with **Defold: Show What This File Draws**:

```json
"defoldTools.showWithTheFile": false
```

- The view goes beside the text or below it, whichever leaves the picture larger. A window that holds more than the two is left as you arranged it.
- An atlas is packed by the Defold editor installed on the machine, away from your project, which takes a few seconds. It is packed again when the atlas or one of its images is saved.
- The others need no build and follow what you type.
- A `.go` file that is Go source opens nothing.

### Reload on save

While the game runs from **Defold: Build and Run**, a script that is saved is loaded again in it. To reload only when asked:

```json
"defoldTools.reloadOnSave": false
```

### Suggestions inside strings

Names are completed inside strings, where VS Code does not suggest by default. The extension turns that on for Lua files. To ask with `Ctrl+Space` instead:

```json
"[lua]": {
  "editor.quickSuggestions": { "strings": "off" }
}
```

- That default reaches `.script`, `.gui_script` and `.render_script` files once `files.associations` says they are Lua, which the Lua language server needs as well.
- A quote, `#`, `/`, `:` or `.` brings the list either way.

### The globals of Defold

The Lua language server reports a global it does not know. So that a project is not underlined from end to end before it has the annotations, the extension names the globals Defold gives a script as the default of `Lua.diagnostics.globals`: the modules of the engine, `hash`, `hash_to_hex`, `pprint`, and the functions the engine calls, from `init` to `on_reload`.

- A default is written nowhere. Setting `Lua.diagnostics.globals` yourself, for a user or for a project, replaces it.
- It holds in every Lua workspace while the extension is installed, a Defold game or not.

### What is checked

Which checks run, the Defold version the project targets and what is ignored are set in the project, in [`defold-vet.toml`](https://github.com/Lanterna-Games/defold-tools/blob/main/docs/defold-vet/settings.md), so that the editor and a pipeline agree:

```toml
defold = "1.14.0"
strict = false

[checks]
unused-file = false
```

## Commands

Press `Ctrl-Shift-P` to open the command palette and type `Defold`.

### Command List

| Command | Description |
|---|---|
| **Build** | Build the game with the Defold editor installed on the machine. |
| **Build and Run** | Build the game and run it, with what it logs in the terminal. |
| **Build and Debug** | Build the game and run it under the debugger. The same as `F5`. |
| **Hot Reload This File** | Save the file and have the running game take it again. |
| **Stop the Game** | Stop the game, whether it runs from Build and Run or under the debugger. |
| **Clean the Build** | Delete what was built, so that the next build starts from nothing. |
| **Fetch Libraries** | Fetch the libraries the project depends on. |
| **Bundle the Game** | Build the game for a platform and pack it into `dist/<platform>`, ready to hand out. |
| **Bring This Atlas in Line with the Images on Disk** | Add the images next to those the atlas has, and take out those that are gone. |
| **Show What This File Draws** | Draw an atlas, tiles, a game object, a collection or a GUI scene next to its text. Also the button at the top right of such a file. |
| **New File…** | Make a file of the game: pick its kind, name it, and say what it is made from. Also on a folder of the explorer. |
| **Show What Uses This File** | List every place of the project that names the file that is open. |
| **Open the Offline Reference** | Open the reference of the project's Defold version in the browser. |
| **Use the API Annotations in This Workspace** | Point the Lua language server at the annotations, replacing the ones Defold Kit may have left. |
| **Restart the Language Server** | Start `defoldpls` again, for instance after updating it. |

Build errors, and the script errors the running game logs, go to the Problems panel.

## Debugging

`F5` needs no `launch.json`. With one, the configuration is:

```json
{
  "type": "defold",
  "request": "launch",
  "name": "Defold: Build and Debug"
}
```

A project that was set up with Defold Kit has a `launch.json` that starts the game through it, and `F5` fails once Defold Kit is gone. The extension offers to replace that configuration with the one above. Nothing is written without the question.

- Breakpoints in scripts and Lua modules, set before the game starts or while it runs
- The stack where the game stopped, with the locals of each function and what `self` holds
- Expressions evaluated where it stopped, in the debug console or on hover
- Step over, into and out, continue and pause

## How it works

The extension holds no logic about a game. It starts `defoldpls` in each workspace folder that has a `game.project`, tells it what is typed and what changes on disk, and shows what it sends back. It runs beside the Lua language server, which covers Lua itself, and needs it for the engine API.

A rename is answered with edits that VS Code applies, so it can be previewed first and undone after. A data file is changed by replacing the name where it stands, and nothing else in it is touched.

## Not there yet

- A reload costs a build, which takes a few seconds even when one file changed.

## Support

There is none. Defold Tools was made to solve the development problems of Lanterna Games, and it is public for whoever finds it useful as it is. Nothing here is a promise of fixes, answers or releases.

If it does not do what you need, [fork it](https://github.com/Lanterna-Games/defold-tools): the licence lets you do what you want, and says so in those words.

## Related

- [`defold-vet`](https://github.com/Lanterna-Games/defold-tools/blob/main/docs/defold-vet/README.md), the same checks from the command line and in a pipeline
- [`defoldpls`](https://github.com/Lanterna-Games/defold-tools/blob/main/docs/defoldpls/README.md), the program behind this extension, for any editor
- [Lua language server](https://luals.github.io)

<br />

<p align="center"><sub>Defold Tools is not affiliated with the Defold Foundation. Defold is a trademark of the Defold Foundation.<br>The icon is not the Defold logo.</sub></p>
