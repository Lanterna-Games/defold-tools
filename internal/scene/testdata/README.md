# A scene the engine was asked about

The files under `main` were written for these tests: a collection with a parent and its
child, an object written in place, a tile map, and a collection inside a collection
inside it, each turned, scaled and moved differently. The three images they name were
plain rectangles: `ball.png` 32 by 16, `wall.png` 64 by 20 and `tiles.png` 32 by 16.

The game objects are kept as `.go.txt`, since a file that ends in `.go` is taken for Go
source by the tools that walk this repository. The tests read them under their names.

The numbers the tests expect are the engine's. The project was built by the build tool
of Defold 1.14.0 and run on its engine, where a script printed the world transform of
each object after a few frames, and the size of two sprites:

```
/parent        | 1.732051 -1.000000 0.000000 100.000000 | 1.000000 1.732051 0.000000 50.000000
/child         | 0.517638 -5.795555 0.000000 117.320511 | 1.931852 1.552915 0.000000 60.000000
/emb           | 1.409539 0.513030 0.000000 0.000000 | -0.513030 1.409539 -0.000000 -80.000000
/ground        | 1.000000 0.000000 0.000000 -50.000000 | 0.000000 1.000000 0.000000 -40.000000
/sub/leaf      | 0.000000 -0.500000 0.000000 -200.000000 | 0.500000 0.000000 0.000000 40.000000
/sub/tip       | 0.000000 -0.500000 0.000000 -215.000000 | 1.000000 0.000000 0.000000 40.000000
/sub/deep/leaf | 0.000000 1.500000 0.000000 -205.000000 | -1.500000 0.000000 0.000000 32.500000
/parent#sprite size 32.000 16.000 scale 2.000 1.000
/child#sprite size 64.000 20.000
```

Each line is the first two rows of the transform: what the object does to what it
holds, across and up, and where it is.

## A GUI the engine was asked about

`study.gui`, with `part.gui` and `inner.gui` as templates inside one another and
`ui.atlas`, was written for these tests as well: nodes with each kind of pivot, turned
about one axis and about all three, scaled unevenly, with children, a size taken from
an image, and template nodes whose fields are overridden, not overridden, and written
without being named as overridden. Its two images were plain rectangles, `wide.png` 48
by 16 and `tall.png` 16 by 40.

It was built by the build tool of Defold 1.14.0 and run on its engine without a window,
on a display of 960 by 640. `study.gui_script` printed, for each node, where
`gui.get_screen_position` puts it, its size, and how many of the points of the display
`gui.pick_node` says it covers, taking every second pixel from 1, with the middle of
those points:

```
plain              at  100.0000   50.0000 size 200.0000 100.0000 picked 5000 centre 100.000  50.000
turned             at  400.0000  300.0000 size 100.0000  60.0000 picked 1342 centre 475.346 362.595
turned_child       at  477.5058  361.9161 size  10.0000  10.0000 picked   22 centre 477.455 361.909
turned_flat_child  at  475.4194  362.6671 size  10.0000  10.0000 picked   22 centre 475.273 362.545
auto               at  700.0000  500.0000 size  48.0000  16.0000 picked  192 centre 700.000 500.000
stretched          at  800.0000  100.0000 size  90.0000  30.0000 picked  676 centre 755.846 117.464
no_size            at   10.0000   10.0000 size   0.0000   0.0000 picked    0 centre       -       -
text               at  300.0000  100.0000 size 200.0000  50.0000 picked 2500 centre 400.000  75.000
pie                at  150.0000  400.0000 size 120.0000  80.0000 picked 2400 centre  90.000 400.000
holder/root        at  560.0000  220.0000 size  50.0000  50.0000 picked 2500 centre 560.000 220.000
holder/leaf        at  542.0000  234.0000 size  20.0000  20.0000 picked  400 centre 542.000 234.000
holder/silent      at  480.0000  220.0000 size  20.0000  20.0000 picked  400 centre 480.000 220.000
holder/nest/dot    at  536.0000  182.0000 size  16.0000   4.0000 picked   64 centre 536.000 182.000
spun               at  200.0000  550.0000 size 100.0000  60.0000 picked 1346 centre 200.000 550.000
spun_child         at  230.7436  599.7850 size  10.0000  10.0000 picked   23 centre 230.739 599.783
spun_flat_child    at  228.6573  600.5359 size  10.0000  10.0000 picked   21 centre 228.714 600.619
z30                at  480.0000  320.0000 size  10.0000  10.0000 picked   24 centre 480.000 320.000
z30_child          at  566.6067  369.9798 size   2.0000   2.0000 picked    1 centre 567.000 371.000
z45                at  480.0000  320.0000 size  10.0000  10.0000 picked   24 centre 480.000 320.000
z45_child          at  550.7107  390.7107 size   2.0000   2.0000 picked    1 centre 551.000 391.000
z123               at  480.0000  320.0000 size  10.0000  10.0000 picked   24 centre 480.000 320.000
z123_child         at  424.9593  403.4789 size   2.0000   2.0000 picked    1 centre 425.000 403.000
```

What this settles:

- a node is placed in its parent by its position, then its turn, then its scale, and
  the turn is about X first, then Z, then Y;
- a position that is not written is 0, a size 0, a scale 1;
- the place of a node is its pivot, and its box lies around it as the pivot says;
- a size taken from an image is the size of the image;
- a node of a template takes from the scene that holds the template only the fields
  `overridden_fields` names, by the numbers they have in Defold's own description of a
  node, which were read from the editor of 1.14.0;
- the engine turns with a table of sines good to a thousandth. `z30_child` is 100
  across from a parent turned 30 degrees and stands at 566.6067, 369.9798, where exact
  sines give 566.6025, 370.0000. The tests allow a fifth of a pixel there.

## A game the engine was asked to draw

Some of what a scene says cannot be asked of the engine by a script: where the pivot of
an image puts a sprite, which way a flipped animation faces, which edges a sliced image
keeps, which way a pie is filled. For those the engine drew a picture. `things.go`,
`hud.gui`, `pics.atlas`, `pivots.atlas` and `flipped.tilesource` were written for it,
with three images: `quarters.png`, 32 by 32, red at the top left, green at the top
right, blue at the bottom left and yellow at the bottom right; `ramp.png`, 48 by 48,
whose red says how far across a pixel is and whose green how far down; and `dot.png`,
4 by 4. The tile source was called `tiles.tilesource` there.

The game was bundled for the web by the build tool of Defold 1.14.0, on a display of
400 by 300, and opened in a browser without a window, whose picture of the page was
read pixel by pixel:

```
sprite, as it is                           placed at (50, 250) covers x 34..66 y 234..266: top left red, top right green, bottom left blue, bottom right yellow
sprite, flipped across                     placed at (120, 250) covers x 104..136 y 234..266: top left green, top right red, bottom left yellow, bottom right blue
sprite, flipped up and down                placed at (190, 250) covers x 174..206 y 234..266: top left blue, top right yellow, bottom left red, bottom right green
sprite, image pivot 0.25, 1.0              placed at (280, 250) covers x 272..304 y 250..282: top left red, top right green, bottom left blue, bottom right yellow
GUI box, image pivot 0.25, 1.0             placed at (360, 250) covers x 352..384 y 250..282: top left red, top right green, bottom left blue, bottom right yellow
sprite of a tile animation flipped across  placed at (220, 140) covers x 204..236 y 124..156: top left green, top right red, bottom left yellow, bottom right blue
GUI box flipped across                     placed at (320, 50) covers x 304..336 y 34..66: top left green, top right red, bottom left yellow, bottom right blue
GUI box, pivot SW, image pivot 0.25, 1.0, placed at (150, 20) covers x 158..190 y 36..68
```

A sprite and a GUI box, both 120 by 80 with `slice9` 4, 8, 12, 16, showed these columns
of the ramp along a row, from the left, and these rows along a column, from the top:

```
across: [0, 1, 2, 3, 4, 4, 4, 5, 5, 5, 6, 6, 6, 6, 7, 7] ... [33, 34, 34, 34, 34, 35, 35, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47]
down:   [0, 1, 2, 3, 4, 5, 6, 7, 8, 8, 9, 9, 9, 10, 10, 11, 11, 12, 12, 12] ... [28, 29, 29, 30, 30, 30, 31, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47]
```

What this settles:

- the pivot of an image is counted from its left and from its top, and the image is
  placed so that the pivot is where the sprite is; a GUI box does the same, on top of
  what the pivot of the node does;
- a flipped animation is mirrored where it stands, from an atlas and from a tile
  source, on a sprite and on a GUI box;
- the four numbers of `slice9` are the left, top, right and bottom edges, which keep
  their size while the middle is stretched;
- a pie filled over 90 degrees covered the quarter to the right of and above its
  middle, and one filled over -120 degrees, with an inner radius, the part of a ring
  from the right down to the lower left: a positive fill turns counterclockwise from
  the right, a negative one clockwise.
