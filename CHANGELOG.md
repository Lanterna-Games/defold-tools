# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and
this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- The `check` command, which reads the data files and scripts of a Defold project and
  reports:
  - `unhandled-message`: a message posted to a script that does not handle it, to a
    component that takes engine messages only, or to an address that is not written
    down when no script in the project handles it;
  - `unposted-message`: a message a script handles that neither the project nor the
    engine posts;
  - `unknown-property`: a property named in `go.get`, `go.set`, `go.animate` or
    `go.cancel_animations` that its game object or component does not have;
  - `unbound-action`: an action `on_input` compares with that no input binding binds;
  - `unknown-collision-group`: a mask, or a group named in a script, that no collision
    object is in;
  - `unknown-gui-node`: a node asked for with `gui.get_node` that the scene does not
    have, templates included;
  - `unknown-animation`: a default animation, a GUI node texture or an animation played
    from a script that the atlas or tile source does not have;
  - `unknown-message-field`: a field `on_message` reads that the engine does not send or
    that no post of the message sets;
  - `undeclared-property`: a script property the script does not declare, set by a
    collection or named in `go.get` and its siblings;
  - `unassigned-self-field`: a field of `self` that a script reads and never assigns;
  - `missing-resource`: a file named in a data file that is not in the project, including
    files named inside embedded components;
  - `unresolved-url`: an address that points at a game object or component that does not
    exist: an address written in a script where the engine expects one, a `msg.url(...)`,
    or a script property of type URL set in a collection. An address may be a string,
    the `hash` of a path or a constant holding either, and may name another collection
    of the project;
  - `unknown-child`: a child that is not in the collection;
  - `unused-file`: a script, a game object, an atlas or another file of the game that no
    data file, no setting and no script names;
  - `unknown-setting` and `invalid-setting`: a section or a key of `game.project` that
    is almost a real one, and a value of the wrong kind, judged against the settings of
    the project's Defold version;
  - `unset-setting`: the collection to start from or the input binding is not set, and
    the file Defold falls back on is not there;
  - `removed-setting`: a setting Defold removed in the version the project targets, as
    1.14.0 removed `[script] shared_state`. A warning in an editor, with a fix that
    removes the line;
  - `syntax`: a data file that cannot be read.
- `defoldpls`, a second program: a language server that sends defold-vet's problems,
  generates annotations of the engine API for the Lua language server for every Defold
  version from 1.6.0 on, and serves the reference of the project's version from the
  machine, on a port the system assigns. `defoldpls docs`, `annotations` and `fetch`
  do the same from the command line.
- `defoldpls` completes, explains and finds the names of a project inside the strings of
  a script: addresses, message ids, properties, GUI nodes, animations, input actions and
  collision groups. A problem that says what was meant can be fixed from the editor.
- `defoldpls` shows what the reference says about a message or a property of the engine
  where it is hovered and where it is offered, with links to the offline reference and
  to defold.com, and completes the fields of a message after `message.` in `on_message`.
- A file or a directory is moved with every place that names it: `defoldpls move`, and
  the rename and the move of the VS Code explorer. `defoldpls uses`, Find All
  References on a path and Defold: Show What Uses This File list those places.
- `defoldpls new` makes eleven more kinds of file, a tile source, a tile map, a sprite,
  a factory and others from the file they point at, and VS Code has Defold: New File….
- In a data file, the files a field takes, the animations of a sprite and the children
  of a game object are completed, a path says what its file is, and a problem that says
  what was meant has its fix, in a component written in place too.
- Going to the definition of the path of a file opens the file, in a script and in any
  data file, a component written in place included: from a game object to the script
  of a component, from a collection to a game object, from an atlas to an image.
- `defoldpls` finds every place that writes a message id, an input action, a collision
  group, a GUI node, a component, a game object, an animation or a script property, and
  renames them all at once, in scripts and in data files. The edits are applied by the
  editor, and a rename that could not be complete is refused.
- `defoldpls build`, `run`, `reload` and `resolve` build the game, run it, reload a
  script in the running game and fetch libraries, with the Defold editor installed on
  the machine. The VS Code extension has a command for each and reloads on save. The
  game that runs takes a service port from the system, so that a reload reaches it and
  not another game that is running.
- `defoldpls bundle` builds the game for a platform and packs it, ready to hand out,
  apart from the build that runs. In VS Code, Defold: Bundle the Game.
- A project with native extensions runs and is debugged on the engine Defold's build
  tool has built for it, and an extension that does not compile is reported once.
- `defoldpls scene` lays out a GUI scene: every node in the display of the game, with
  the nodes of its templates and what the scene overrides in them, in the order the
  layers draw it. The VS Code extension draws it next to the text of a `.gui`.
- A sprite is placed by the pivot its image has in the atlas, mirrored when its
  animation is flipped, and stretched with the edges of its `slice9` kept.
- A collection placed inside another is found and renamed like the other names: its id
  where it is placed, and that part of every address that goes through it.
- The language server parses a file again only when it changed, which takes a third to
  a half off a check of a large project. What is reported is the same.
- `defoldpls clean` deletes what was built. The VS Code extension has Defold: Clean the
  Build and Defold: Stop the Game.
- The VS Code extension names the globals Defold gives a script for the Lua language
  server, as a default that writes nothing, so that a project is not reported from end
  to end before it has the annotations. A file nothing uses now comes to an editor as a
  warning.
- `defoldpls dap` is a debug adapter for the game: breakpoints, the stack, locals and
  what tables hold, expressions, steps and pauses, through the debugger the engine
  carries. In VS Code it is Defold: Build and Debug, or F5.
- `defoldpls new` makes a script, a module, a game object, a collection, a GUI scene or
  an atlas, and `defoldpls atlas` brings an atlas in line with the images on disk,
  changing nothing else in it.
- `defoldpls pack` has Defold's build tool pack one atlas, away from the game, and
  prints where each image landed. `defoldpls tiles` prints the tiles of a tile source
  and the cells of a tile map, which need no build. `defoldpls scene` prints what a
  game object or a collection places and where, as far as the text says: sprites, tile
  maps, labels and the shapes of collision objects, and a mark for the rest. In VS
  Code, Defold: Show What This File Draws draws each next to its text, and it opens
  with the file.
- Libraries a project depends on are read once they were fetched, from `.internal/lib`,
  so that a project with dependencies is judged in full.
- `defoldpls` completes and explains `game.project`: sections, the keys of a section
  that are not set yet, the values a setting takes, and the file a setting names.
- The VS Code extension is now Defold Tools and starts `defoldpls`.
- Documentation for defold-vet under `docs/defold-vet`: a guide, a reference of every
  check, how a project is read, strict mode, settings, editors and CI.
- `defold-vet lsp`, a language server that reports the problems of `check` as they are
  typed, and a VS Code extension without dependencies that starts it.
- Strict mode, on by default: `unverifiable` reports a name that is not written where a
  check would have read it, and the place where a script stops being readable as a
  whole. `strict = false` leaves it.
- Names are followed through constants: a local of the file, a field of a table of the
  file, and a field of the table a required module returns.
- Settings in `defold-vet.toml`: the Defold version the project targets, checks turned
  off, checks ignored in the files a pattern matches, each with a reason, and names the
  project vouches for where the reference has a gap.
- Defold from 1.6.0 to 1.14.0 is supported, and the reference of every version in that
  window is carried by the program. What a version's documentation lost between two
  versions that have it is put back. A version newer than the program is fetched from
  Defold's archive and kept in the user's cache directory; an older one is refused.
- `defold-vet check --defold <version>` checks against another Defold version than the
  one the project declares.
- Nothing is reported in a file `.defignore` names, with the patterns Defold 1.14.0
  brought for a project that targets it.
- `-- defold-vet:ignore <check> <reason>` silences one line of a script. An ignore
  without a reason, of something that is not a check, or of nothing, is reported.
- A Lua 5.1 parser that keeps the column of every node and returns what it understood
  from a script that is half typed.
- The reference of Defold, built in, which says where the engine expects a URL,
  which messages are the engine's own and which fields they carry, which properties
  each kind of component has and what the builtin materials and the builtin input
  binding hold.
- Problems are reported as `file:line:column`, with a suggestion when a name is close to
  one that exists.
- The `version` and `help` commands.

### Changed

- The licence is the WTFPL, with no warranty, where it was MIT. The contributing guide,
  the code of conduct and the security policy now say what the project offers, which is
  the code.

### Fixed

- Restarting the language server from VS Code, or changing one of its settings, left
  the old server running beside the new one, and every completion and fix was offered
  twice.
- A problem with a hashed name, such as an input action or a message id, underlined
  only the word `hash`. It underlines the whole of `hash("name")`, so pointing at the
  name shows what is wrong with it.
- `defoldpls atlas` added the frames of an animation again, as images of their own,
  when their files were in a directory the atlas takes images from.
- The `prototype` of a game object placed in a collection, and that of a factory, was
  offered collections as well as game objects. Each is now offered what it takes, and
  a collection factory is offered collections.

[Unreleased]: https://github.com/Lanterna-Games/defold-tools/commits/main
