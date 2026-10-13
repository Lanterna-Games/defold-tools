# A tile source and a tile map

`tiles.tilesource` and `level.tilemap` were written for these tests. The image they
name was 78 by 58: twelve tiles of 16 by 16, four to a row, each with a margin of one
pixel around it and two pixels between one and the next.

What the tests expect was first settled against what the build tool of Defold 1.14.0
made of them. Each tile was found in the texture it wrote, pixel by pixel, where the
tests say it is in the image. Its texture set gave the animation written as tiles 2 to
4 the tiles 1, 2 and 3, counted from nothing. And the tile map it compiled keeps the
number of a cell as it is written, which the engine uses to pick a tile counted from
nothing.
