# 0023. A file is moved with the places that name it

Status: accepted, 2026-10-13

## Context

Defold links its files by path: a collection names its game objects, a game object its
scripts, an atlas its images, the settings the collection to start from. The Defold
editor keeps those paths right when a file is renamed or moved. A text editor moves
the file and nothing else, and every file that named it is broken until each path is
corrected by hand. It was the one everyday task that still needed the Defold editor.

Names were already renamed here through what the checks know of a project (0014). A
path is simpler than a name and written in more places than the checks read: in any
field of any data file, inside components written in place with their quotes escaped
once or twice, under a compiled name in the settings, in the strings of scripts, and,
for a Lua module, as a dotted name.

## Decision

A move is followed by text, not by the model: every file of the project that can name
another is read once, and each path written in it is compared with what moved.

- **A path is a path wherever it stands.** It is taken between the marks that hold it,
  a quote, an escaped quote or the sign of a setting, and it has to be the whole of
  the old path, or lie under the directory that moved. A name that only begins like it
  is another name.
- **The compiled name and the module name move with it**: `/main/main.collectionc`
  for the settings, and `main.ids` for a `require`.
- **All or nothing** (0014): the edits are worked out first, each is checked against
  the text it replaces, and a target that is there already stops the move.
- **The editor applies it where there is one.** It asks before it renames or moves a
  file, and is answered with the edits, which it applies with the move. From the
  command line, `defoldpls move` writes them and then moves the file.
- **What a new name changes besides the path is said, not followed.** An image gives
  its name to its animation, so an image under another name is an animation under
  another name. The move says which, and leaves what plays it alone.
- **The same reading answers what uses a file**, which is every place that names it.

## Consequences

- A file or a directory can be moved from the text editor without breaking the game.
- A path in a comment of a script is rewritten too, which is usually what is wanted.
- A path a script builds while the game runs is not found. Neither is a path in a
  kind of file that is not read for paths, such as a file of the game's own format.
- In each of the 393 sample projects a file of every kind that something names was
  given another name in memory and the project read again: 2127 files, followed by
  11421 edits, and the checks said what they had said before in every one. Another
  1019 were images an atlas names, set aside since a new name renames their animation.
- A copy of a real game had a game object, a script, a directory of images, a module
  and the directory holding nearly everything moved one after the other, and was then
  built by the build tool of Defold 1.14.0 as before.
