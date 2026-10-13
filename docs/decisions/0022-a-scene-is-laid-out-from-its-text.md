# 0022. A scene is laid out from its text, and has an end

Status: accepted, 2026-10-12

## Context

After atlases (0020) and tiles (0021), the files left that the Defold editor shows as a
picture are the game object and the collection. Its picture of them is the game itself:
every component rendered with its material, in three dimensions where there are any.

That cannot be had from a text editor without being the engine. What can be had is
where things are. The position, rotation and scale of every object and component are
in the text, as are the children of an object and the collections placed inside a
collection, and for some components so is what they look like, near enough: a sprite
is an image, a tile map its cells, a label a box with a text, a collision object its
shapes.

## Decision

A game object and a collection are shown as a layout: what the text places, where the
text places it, and a mark for what the text cannot show.

- **`defoldpls scene`** follows a collection through its objects, their children, the
  objects written in place and the collections inside it, multiplying what each one
  does to what it holds, and gives every component the transform it ends up with.
- **Drawn**: a sprite as the image of its animation at the size of that image, a tile
  map as in 0021, a label as its box and text, a collision object as its boxes,
  spheres and capsules.
- **Marked, not drawn**: everything else. A script, a factory or a sound has no
  picture. A model, a mesh, a particle effect, a GUI or a Spine model has one that
  only the engine can make. The mark says what is there and where.
- **Not attempted**: materials, tints, blend modes, the font of a label, what a render
  script does, what a camera frames, what a factory makes while the game runs.
- **The places are the engine's.** A fixture with a parent and a child, an object
  written in place and a collection inside a collection inside it, each turned and
  scaled differently, was built and run on the engine of Defold 1.14.0, where a script
  printed the world transform of every object and the size of two sprites. The tests
  hold the scene to those numbers.
- **A scene has an end.** A collection inside another is laid out again each time it
  is placed, so a few files can stand for more objects than a machine has memory for.
  A tile map is kept once however often it is placed, a scene stops at 20,000 objects
  or a million cells and says it was cut short, and a collection is not laid out
  inside itself.
- **A file that ends in `.go` and is Go source is refused**, and the VS Code extension
  asks before it opens a view for such a file, so that the source of a server kept
  next to a game opens nothing.

## Consequences

- Where everything is can be seen and followed back to the line that put it there,
  while typing, with no editor of Defold installed.
- It is not what the game looks like, and the view says what it left as marks.
- The bounds were learnt the hard way: the first version, run over a project made to
  stress Defold, of 134 thousand objects, took all the memory of the machine it was
  run on, twice. Whatever is run over many projects is now run with a cap on memory
  and on time.
- The pivot an atlas can give an image, the slices of a sliced sprite and a sprite
  mirrored by its animation are not read: such a sprite is drawn plain, around its
  middle.
- The numbers were the engine's for Defold 1.14.0. How a component is placed on its
  object was not asked of the engine, which has no word for it; it is the same
  arithmetic one level down.
