# 0021. Tiles are drawn from their text

Status: accepted, 2026-10-12

## Context

After an atlas (0020), the two other files a text editor shows as text and the Defold
editor as a picture are the tile source and the tile map. An atlas has to be asked of
the build, because a packer decides where its images go. These two are different: a
tile source cuts one image into a grid whose measures it writes down, and a tile map
lists its cells one by one. Nothing is decided at build time.

What the text does not say is how to read it. Where the first tile starts when there
is a margin, whether tiles are counted from nothing or from one, which way up a map
grows, and what a turned and mirrored cell looks like are conventions of Defold.

## Decision

A tile source and a tile map are drawn from their text, with no build, and each
convention was settled against Defold before it was written down.

- **The grid** is counted as Defold's build tool and editor count it, by one function
  they share, and a tile lies a margin in from its slot. Every tile of a fixture was
  then found, pixel by pixel, in the texture the build tool of 1.14.0 wrote, where
  that puts it in the image.
- **An animation counts its tiles from one, both ends included, and a cell counts its
  tile from nothing.** The texture set of the same build gave the tiles 1, 2 and 3 for
  an animation written from 2 to 4, and the engine picks a tile by the number of the
  cell as it is. The sample projects agree: no animation starts below one, and 102 of
  their maps have a cell of tile nothing.
- **A cell grows upwards**, as the engine places it, and **what is done to its tile**
  is read from the table of corners the engine draws by, against the corners the build
  wrote for a tile: the tile is mirrored first and then turned a quarter clockwise.
- **`defoldpls tiles`** prints what follows from these, and the VS Code extension
  draws it: the image under its grid, or the cells of a map layer by layer in the
  order of their depth, without the layers that are hidden.
- **It takes the file as the editor holds it**, since nothing has to be built: the
  picture follows what is typed.

## Consequences

- A tile source and a tile map are shown at once, with no editor of Defold installed.
- The shapes a tile source gets for collisions are not shown. Defold works them out at
  build time from the collision image, and they are not in the text.
- The conventions are Defold's to change. The turning of a cell rests on reading the
  engine's source, not on seeing the engine draw; the rest was seen in what the build
  wrote.
- A map is drawn as one picture. A very large one is drawn smaller than it is, to stay
  within what a browser will draw.
- This is a picture, not an editor: a map is still painted in Defold.
