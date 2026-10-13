# In the editor

`defold-vet lsp` is a language server. It reports the same problems as
`defold-vet check`, from the same code, while the project is being edited:

- what is open and not yet saved is read in place of what is on disk;
- a problem that is fixed disappears, and one that a change causes in another file
  appears there;
- what [cannot be verified](strict-mode.md), a [file nothing
  uses](checks.md#unused-file) and a [setting Defold
  removed](checks.md#removed-setting) come as warnings, everything else as an error;
- settings that cannot be read are shown as an error message, not passed over.

It runs beside the [Lua language server](https://luals.github.io), which covers Lua
itself. The two do not overlap.

Today the server sends problems and nothing else: no completion, no going to a
definition, no fixing a name from its suggestion.

[`defoldpls`](../defoldpls/README.md) sends the same problems and more: completion,
hover and definitions for the names of the project, a fix from a suggestion, the engine
API for the Lua language server and the offline reference. An editor needs one of the
two, not both.

## VS Code

The extension is in [`editors/vscode`](../../editors/vscode/README.md). It holds no
logic: it starts `defoldpls` in each workspace folder that has a `game.project`, tells
it what is typed and what changes on disk, and shows what it sends back in the editor
and in the Problems panel.

It has no dependencies and no build step. To package and install it from a clone:

```sh
make vsix
code --install-extension build/defold-tools.vsix
```

If `defoldpls` is not in the `PATH` of the editor, say where it is:

```json
"defoldTools.server.path": "C:\\Tools\\defold-tools\\defoldpls.exe"
```

**Defold: Restart the Language Server**, in the command palette, starts the program
again, for instance after updating it.

### Windows and WSL

The program has to run on the same side as the extension, because the two exchange
file paths. A project opened in a Windows window needs the Windows build of defold-vet,
and one opened in a WSL window needs the Linux build installed in WSL, with the
extension installed in that window too. Building for Windows from Linux is one line:

```sh
GOOS=windows GOARCH=amd64 go build -o . ./cmd/...
```

### Without the extension

A task can run `check` and feed the Problems panel. The output matches this pattern:

```json
"problemMatcher": {
  "owner": "defold-vet",
  "fileLocation": ["relative", "${workspaceFolder}"],
  "pattern": {
    "regexp": "^(.+?):(\\d+):(\\d+): (.*) \\[([a-z-]+)\\]$",
    "file": 1, "line": 2, "column": 3, "message": 4, "code": 5
  }
}
```

## Other editors

Any editor with a language-server client can use it. What the client needs to know:

| | |
|---|---|
| Command | `defold-vet lsp`, or `defoldpls`, speaking on standard input and output |
| Root | the directory that holds `game.project`, sent as `rootUri` |
| Documents | sent whole on every change (`textDocumentSync` is *full*) |
| Files | `.script`, `.gui_script`, `.render_script`, `.lua`, and the data files if the editor opens them |
| Changes on disk | a `workspace/didChangeWatchedFiles` notification makes it read the project again; its contents are not looked at |

`defold-vet lsp` answers `initialize` and `shutdown`, and refuses every other request as
not found. What `defoldpls` answers besides is [listed with it](../defoldpls/README.md#for-editors).

## How fast it is

Every change checks the whole project again, after a pause of a fifth of a second
without typing. On a small game the check itself is not noticed. On the largest
project it was tried on, a synthetic one with 25 thousand game objects, it takes
several seconds.
