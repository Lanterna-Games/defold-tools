# A texture set as Defold writes it

`props.a.texturesetc` is what the build tool of Defold 1.14.0 wrote for `props.atlas`,
next to it. Nothing here is taken from a game: the atlas and its six images were made
for this test, the images as plain rectangles of these sizes.

| Image | Width | Height |
|---|---|---|
| `house.png` | 64 | 48 |
| `tree.png` | 32 | 80 |
| `coin.png` | 16 | 16 |
| `sign.png` | 100 | 30 |
| `walk_1.png`, `walk_2.png` | 24 | 24 |

Each image had a mark at its top left corner and a line along its bottom, and the place
and the turn the test expects for each were first read off the texture the build tool
wrote, pixel by pixel. The house is the one it turned.
