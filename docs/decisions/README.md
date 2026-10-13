# Decisions

Source files in this project carry no comments, so the reasoning has to live
somewhere. It lives here: one short record per decision that someone will later ask
"why?" about.

| | Decision |
|---|---|
| [0001](0001-record-decisions.md) | Record decisions |
| [0002](0002-written-in-go.md) | Written in Go |
| [0003](0003-one-binary-for-editors-and-ci.md) | One binary for editors and CI |
| [0004](0004-read-only.md) | Read project files, never write them |
| [0005](0005-literals-and-strict-mode.md) | Check literals, and be strict by default |
| [0006](0006-tied-to-a-defold-version.md) | Tied to a Defold version |
| [0007](0007-no-comments-in-source.md) | No comments in source files |
| [0008](0008-how-messages-are-matched.md) | A message is matched by what `on_message` compares |
| [0009](0009-what-strict-mode-reports.md) | Strict mode reports names, not values |
| [0010](0010-where-a-reference-comes-from.md) | A reference is bundled, or fetched by version and kept |
| [0011](0011-two-programs-one-repository.md) | Two programs in one repository |
| [0012](0012-a-window-of-defold-versions.md) | A window of Defold versions is supported, and carried |
| [0013](0013-the-api-is-handed-to-the-lua-language-server.md) | The engine API is handed to the Lua language server, and generated here |
| [0014](0014-defoldpls-may-change-a-project.md) | defoldpls may change a project, when asked |
| [0015](0015-the-game-is-built-by-the-installed-editor.md) | The game is built and run with the editor that is installed |
| [0016](0016-the-game-is-debugged-by-its-own-engine.md) | The game is debugged with the debugger its engine carries |
| [0017](0017-a-project-just-opened-shows-only-mistakes.md) | A project just opened shows only mistakes |
| [0018](0018-a-setting-defold-removed-is-reported.md) | A setting Defold removed is reported |
| [0019](0019-nothing-is-reported-in-what-defold-ignores.md) | Nothing is reported in what Defold is told to ignore |
| [0020](0020-an-atlas-is-shown-as-the-build-packs-it.md) | An atlas is shown as the build packs it |
| [0021](0021-tiles-are-drawn-from-their-text.md) | Tiles are drawn from their text |
| [0022](0022-a-scene-is-laid-out-from-its-text.md) | A scene is laid out from its text, and has an end |
| [0023](0023-a-file-is-moved-with-the-places-that-name-it.md) | A file is moved with the places that name it |
| [0024](0024-a-game-with-extensions-runs-on-the-engine-built-for-it.md) | A game with extensions runs on the engine built for it |
| [0025](0025-a-gui-is-laid-out-from-its-text.md) | A GUI is laid out from its text |
| [0026](0026-a-check-reads-again-only-what-changed.md) | A check reads again only what changed |

## Adding one

Copy the shape of an existing record: **Context**, **Decision**, **Consequences**.
Number it next in line and add it to the table. A record is not edited once it is
accepted; a later decision that changes it says so and links back, and the old one
gets `Status: superseded by NNNN`.
