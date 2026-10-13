# Checks

Every problem defold-vet prints ends with the name of the check that found it, in
brackets. That name is what [settings](settings.md) turn off and what an ignore comment
names.

| Check | In one line |
|---|---|
| [`unresolved-url`](#unresolved-url) | An address that leads to no game object or component |
| [`unhandled-message`](#unhandled-message) | A message posted to something that does not handle it |
| [`unposted-message`](#unposted-message) | A handler waiting for a message nothing posts |
| [`unknown-message-field`](#unknown-message-field) | A field read from a message that does not carry it |
| [`unknown-property`](#unknown-property) | A property its game object or component does not have |
| [`undeclared-property`](#undeclared-property) | A script property the script does not declare |
| [`unassigned-self-field`](#unassigned-self-field) | A field of `self` that is read and never assigned |
| [`unbound-action`](#unbound-action) | An input action no binding binds |
| [`unknown-collision-group`](#unknown-collision-group) | A collision group nothing is in |
| [`unknown-gui-node`](#unknown-gui-node) | A GUI node the scene does not have |
| [`unknown-animation`](#unknown-animation) | An animation the atlas or tile source does not have |
| [`missing-resource`](#missing-resource) | A file named in a data file that is not in the project |
| [`unknown-child`](#unknown-child) | A child that is not in the collection |
| [`unused-file`](#unused-file) | A file nothing in the project uses |
| [`unknown-setting`](#unknown-setting) | A section or a key of `game.project` that is almost a real one |
| [`invalid-setting`](#invalid-setting) | A value of `game.project` of the wrong kind |
| [`unset-setting`](#unset-setting) | A setting a build needs that is not set and has nothing to fall back on |
| [`removed-setting`](#removed-setting) | A setting Defold removed in the version the project targets |
| [`syntax`](#syntax) | A data file that cannot be read |
| [`unverifiable`](#unverifiable) | Strict mode: a name that cannot be read |
| [`ignore`](#ignore) | An ignore comment that is not in order |

The rules the checks share (what counts as written, how a shared script is judged, what
a library changes) are in [how it reads a project](how-it-reads.md). Each section below
only adds what is particular to the check.

---

## unresolved-url

An address that leads to a game object or a component that does not exist.

```lua
msg.post("#bounce_sond", "play_sound")
```

```
main/ball.script:5:14: "#bounce_sond": "/ball" has no component "bounce_sond" (did you mean "bounce_sound"?) [unresolved-url]
```

It is judged in three places:

- in a script, where the engine expects an address: the receiver of `msg.post`, the
  first argument of `go.get`, `sprite.play_flipbook`, `factory.create` and the rest, as
  the reference of the Defold version says;
- every `msg.url(...)`, wherever it stands, since a URL kept in `self` is as wrong as
  one passed along;
- in a collection, a script property of type URL:

```
main/main.collection:10:14: property "game" of "/ball#ball" points at "/game#gaem", but "/game" has no component "gaem" (did you mean "game"?) [unresolved-url]
```

**Stays quiet** for an address that is not written, for one that leaves a game object
or collection a factory spawns, for a path under a collection from a library, for a
socket that is no collection of the project, and for an address the script passes to
`go.exists`. See [where an address leads](how-it-reads.md#where-an-address-leads).

## unhandled-message

A message of the project posted to something that does not handle it.

```lua
msg.post("/game#game", "bal_out", { side = 1 })
```

```
main/ball.script:13:28: "bal_out" is posted to "/game#game", but /main/game.script does not handle it (did you mean "ball_out"?) [unhandled-message]
```

Where the receiver is written, the message is followed to it:

- a script, or the script of a GUI scene, has to handle it;
- a component that is not a script takes engine messages only;
- a game object needs one component that handles it.

Where the receiver is not written (`msg.post(self.target, "tick")`), the message is
reported only if no script in the whole project handles it.

A script **handles** a message when its `on_message` compares the message id with it in
the condition of an `if`, or returns early on every other id:

```lua
if message_id == hash("ball_out") then ... end
if message_id ~= BALL_OUT then return end
```

A handler that does anything else with the id is taken to handle **every** message:
passing it to a function, looking it up in a table, comparing it with a value that is
not written, an `else` after a test of the id, a body that never looks at the id. This
is the usual shape of a handler that forwards to a library, and it is why such a script
is never reported here. [Decision 0008](../decisions/0008-how-messages-are-matched.md)
has the full rules.

**Stays quiet** for engine messages, and, in a project with
[code out of sight](how-it-reads.md#code-out-of-sight), for everything but a post
whose receiver is a script of the project.

## unposted-message

A handler waiting for a message that neither the project nor the engine posts.

```lua
if message_id == hash("colision_response") then
```

```
main/ball.script:19:22: "colision_response" is handled here, but nothing in the project posts it (did you mean "collision_response"?) [unposted-message]
```

This is where a misspelt engine message shows up, and where the other end of a
misspelt message of your own does: the post is reported as `unhandled-message`, the
handler here.

**Stays quiet** unless every `msg.post` in the project has an id that can be read, since
one post that hides its id could be the one. A mention of `msg.post` that is not a call
(`local post = msg.post`) counts as one that hides it. Also quiet in a project with
code out of sight, and for the render script, which the engine and the cameras talk to
in messages of their own.

## unknown-message-field

A field read from the message in `on_message` that the message does not carry.

```lua
elseif message_id == hash("trigger_response") then
    if message.other_group == hash("wal") or message.exit then
```

```
main/ball.script:22:58: "exit" is not a field of "trigger_response" [unknown-message-field]
```

A message only the engine sends has the fields its reference documents. Any other has
the fields its posts set:

```lua
msg.post("/game#game", "serve", { speed = self.speed })   -- in one script
self.speed = message.sped                                 -- in the handler of another
```

```
main/game.script:7:30: "sped" is read from "serve", but nothing that posts it sets "sped" (did you mean "speed"?) [unknown-message-field]
```

A read is judged where the handler has settled which message it holds: inside
`if message_id == A`, inside `A or B` (the field has to belong to one of them), and
after `if message_id ~= A then return end`.

**Stays quiet** for a message of the project when one of its posts passes a table built
elsewhere, when some post in the project hides its id, when nothing posts it, and in a
project with code out of sight. A field the handler assigns to the message itself is
not expected from the post. A message passed on to a helper is not followed.

## unknown-property

A property named in `go.get`, `go.set`, `go.animate` or `go.cancel_animations` that
what the address leads to does not have.

```lua
go.set(".", "postion.x", 0)
go.set("#sprite", "tnit.w", 0.5)
```

```
main/ball.script:6:17: "postion.x": "." is a game object, which has no property "postion" (did you mean "position"?) [unknown-property]
main/ball.script:7:23: "tnit.w": "#sprite" is a sprite component, which has no property "tnit" (did you mean "tint"?) [unknown-property]
```

| The address leads to | Its properties |
|---|---|
| a game object | `position`, `rotation`, `euler`, `scale` |
| a component | what the reference documents for its kind |
| a component drawn with a material | also the constants, attributes and samplers of the builtin materials and of the project's |
| nothing that is written | the name has to be a property of something in the project |

`position.x` is read as `position`.

**Stays quiet** for a kind of component the reference does not document, such as one a
native extension adds. A script as the target is the next check.

## undeclared-property

A script property the script does not declare with `go.property`.

Set in a collection:

```
main/main.collection:14:11: "/ball#ball" sets the property "sped", but /main/ball.script does not declare it (did you mean "speed"?) [undeclared-property]
```

Or named in `go.get` and its siblings, with a script as the target:

```lua
self.best = go.get("#game", "scroe")
```

```
main/game.script:8:37: "scroe": /main/game.script does not declare the property "scroe" (did you mean "score"?) [undeclared-property]
```

**Stays quiet** for a script that declares a property under a name that cannot be read,
and for one that is not in the project.

## unassigned-self-field

A field of `self` that a script reads and never assigns.

```lua
sprite.play_flipbook("#sprite", self.skin)
```

```
main/ball.script:15:42: self.skin is read, but this script never assigns it [unassigned-self-field]
```

A field counts as assigned when the script assigns it anywhere, or declares it with
`go.property`. The instance is followed from the functions the engine calls (`init`,
`update`, `on_message` and the rest) into the helpers that receive `self`.

**Stays quiet** inside a method of a class the script defines (`function Enemy:hit()`),
where `self` is something else, and for a script that hands its instance to code that
cannot be followed.

## unbound-action

An action `on_input` compares its `action_id` with and no input binding binds.

```lua
if action_id == hash("p3_up") then
```

```
main/ball.script:29:21: "p3_up" is not bound in /input/game.input_binding (did you mean "p1_up"?) [unbound-action]
```

The binding is the one `game.project` names as `game_binding`, the builtin
`/builtins/input/all.input_binding` included. An action bound in any other binding of
the project counts as bound too.

**Stays quiet** when the game binding comes from a library. A script that calls its own
`on_input` makes up input: the actions it passes count as bound, and if one of them
cannot be read the script is not judged.

## unknown-collision-group

A collision group that no collision object is in.

In a collision object's mask:

```
main/ball.go:27:10: mask "wal" is not the group of any collision object (did you mean "wall"?) [unknown-collision-group]
```

Or in a script, where the engine means a group:

```lua
if message.other_group == hash("wal") or message.exit then
```

```
main/ball.script:22:35: group "wal" is not the group of any collision object (did you mean "wall"?) [unknown-collision-group]
```

That is: the groups of `physics.raycast`, `physics.set_group`, `set_maskbit` and
`get_maskbit`; a comparison with `other_group` or `own_group`; and a comparison with
`group` of the message `on_message` receives. The groups a tile source defines count.

**Stays quiet** for a field called `group` anywhere else, for a project in which nothing
declares a group, and for a project with code out of sight.

A mask that matches nothing is usually left over from a group that was renamed, and
does no harm. It is reported because it is the only trace of the rename.

## unknown-gui-node

A node asked for with `gui.get_node` that the scene running the script does not have.

```lua
self.score = gui.get_node("scroe")
```

```
main/hud.gui_script:2:31: "scroe": /main/hud.gui has no node "scroe" (did you mean "score"?) [unknown-gui-node]
```

The nodes of a scene are its own and those of its templates, under the id of the node
that holds each template (`restart/label`), to any depth.

**Stays quiet** for an id under a template that is not in the project, and for an id the
script gives to a node itself with `gui.set_id`. A script that names nodes with ids that
cannot be read is not judged.

## unknown-animation

An animation the atlas or tile source does not have.

The default animation of a sprite, and the texture of a GUI node:

```
main/ball.go:8:29: default animation "bal" is not in /main/ball.atlas (did you mean "ball"?) [unknown-animation]
main/hud.gui:13:12: texture "ui/bal": /main/ball.atlas has no animation "bal" (did you mean "ball"?) [unknown-animation]
```

Or one played from a script, with `sprite.play_flipbook`, `gui.play_flipbook` or the
`play_animation` message:

```lua
sprite.play_flipbook("#sprite", "rol")
```

```
main/ball.script:8:37: "rol": "#sprite" plays the animations of /main/ball.atlas, which has no "rol" (did you mean "roll"?) [unknown-animation]
```

The animations of an atlas are its animation groups and one per single image, named
after the file and renamed by the atlas's rename patterns.

**Stays quiet** for a texture that is not in the project. When a script of the project
sets the `image` of a sprite, a played animation only has to be in some atlas of the
project. When one builds an atlas while the game runs (`resource.create_atlas`,
`resource.set_atlas`), animations played from scripts are not judged.

## missing-resource

A file named in a data file that is not in the project.

```
main/ball.go:19:17: "/main/bounce.wav" does not exist [missing-resource]
```

Every data file is read, and so is every component embedded in one, and the settings in
`game.project` that take a file.

**Stays quiet** for a path whose top directory is not in the project, such as
`/builtins/...` and the directories of libraries, and for a path pattern, as a texture
profile has.

## unknown-child

A game object listed as a child that is not in the collection.

```
main/main.collection:5:13: "ball" lists the child "trail", but collection "main" has no game object with that id [unknown-child]
```

## unused-file

A file of the game that nothing in the project uses.

```
main/old_enemy.go:1:1: nothing in the project uses this file [unused-file]
```

In an editor it is a warning, not an error: the file may be a module that is about to be
required, or what a tool has just written.

It is about the files a game is made of: scripts, Lua modules, game objects, GUI
scenes, atlases, tile sources and tile maps, the component files, fonts, materials,
render files, input bindings and the profile files. A file is used when a data file
names it, when the settings do, or when a script writes its path. A game object that
only an unused collection places is used: what is reported is the end of the thread,
the file that nothing at all leads to.

**Stays quiet** for:

- a collection: one that nothing loads is a place the game can start from;
- anything a script names in a string, whatever the string is for: the path of a
  prototype given to `factory.set_prototype`, an atlas held by a script property, a
  module required behind `pcall`;
- Lua modules, all of them, once one `require` in the project builds the name it loads;
- what the settings name, in `game.project` or in any other `.project` or `.settings`
  file, and everything under a directory they list, as `custom_resources` does;
- a project that is a library, whose files are for the projects that depend on it;
- every other kind of file: images, sounds, text, anything the game may load by itself.

## unknown-setting

A section or a key of `game.project` that the engine does not have and that is close to
one it has.

```
game.project:5:1: "widht" is not a setting of [display] (did you mean "width"?) [unknown-setting]
game.project:8:2: "dispaly" is not a section of the settings (did you mean "display"?) [unknown-setting]
```

What the engine has comes from the schema Defold's build tool carries for the project's
[version](settings.md#defold).

A project file gathers keys over the years, and none of that is a mistake. **Stays
quiet** for a section the engine does not have, which is how an extension gets its
settings; for a key that is not close to a real one; for a key a later version of
Defold brings; and for an icon or a launch image of a size Defold no longer asks for.
A key Defold had and took out has a check of its own,
[`removed-setting`](#removed-setting).

## invalid-setting

A value of `game.project` that is not of the kind its setting takes.

```
game.project:6:12: "yes" is not 0 or 1, which is what display.high_dpi takes [invalid-setting]
game.project:12:23: "9" is below the minimum of 21, which is what android.minimum_sdk_version takes [invalid-setting]
```

Whole numbers, numbers and switches are judged, with the bounds the schema gives. A
setting left empty is not. A file a setting names is judged by
[`missing-resource`](#missing-resource), under its own name or its compiled one
(`/main/main.collectionc`).

## unset-setting

A setting the build cannot do without that is not set, when the file it falls back on is
not there either.

```
game.project:1:1: input.game_binding is not set, and "/input/game.input_binding", which it falls back on, does not exist [unset-setting]
```

Two settings are judged: the collection the game starts from and the input binding.
Neither has to be written, since each has a path Defold looks at instead, but a build
stops when that file is missing too.

## removed-setting

A setting that Defold removed, in a project that targets a version without it.

```
game.project:8:1: "shared_state" is no longer a setting of [script]: Defold 1.14.0 removed it [removed-setting]
```

The line does nothing any more. Often that is all there is to it: Defold 1.14.0 always
shares one Lua state, and `shared_state = 1`, which its project templates wrote for
years, asks for what now happens anyway. Sometimes the game changes with it: a project
that had the same setting at `0` now shares what it kept apart.

What was removed, and in which version, comes from the schemas of the versions
defold-vet [carries](settings.md#defold): a setting an earlier version had and no later
one has. In an editor it is a warning, with a fix that removes the line.

**Stays quiet** for:

- a project that targets a version that still has the setting;
- a setting that a later version brings and the targeted one does not have yet;
- a setting that one version lacks and the next has again, which is a hole in that
  version's schema;
- what Defold removed before 1.6.0, the oldest version carried.

## syntax

A data file that cannot be read.

```
main/ball.go:4:1: missing "}" [syntax]
```

The rest of the file is still read as far as it makes sense, so that one broken file
does not hide the problems of the others. Lua syntax is not reported: that is the Lua
language server's.

## unverifiable

Strict mode. A name that is not written where a check would have read it, and the place
where a script stops being readable as a whole.

```
main/ball.script:15:37: the animation played here cannot be read [unverifiable]
```

It has [a page of its own](strict-mode.md).

## ignore

An ignore comment that is not in order: it gives no reason, names something that is not
a check, or silences nothing.

```
main/ball.script:2:35: this ignore gives no reason: write why after "unresolved-url" [ignore]
main/ball.script:3:35: "unresolved-ulr" is not a check (did you mean "unresolved-url"?) [ignore]
main/ball.script:4:5: nothing is reported here for "unknown-animation": this ignore can go [ignore]
```

It cannot be turned off or ignored. See [silencing one line](settings.md#silencing-one-line).
