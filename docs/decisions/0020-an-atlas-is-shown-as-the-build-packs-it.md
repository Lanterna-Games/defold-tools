# 0020. An atlas is shown as the build packs it

Status: accepted, 2026-10-12

## Context

The Defold editor shows an atlas as a page with every image in its place. From a text
editor an atlas is a list of files: it has no coordinates, because the places are
worked out again at every build, by a packer, from the sizes of the images, the margin,
the borders and the padding the atlas sets. The picture is the one thing about an atlas
that cannot be read from its text.

Two ways lead to it. The packer can be written again here, from the source of Defold's
build tool, which is some two thousand lines with the trimming of sprites. Or the build
tool can be asked: it writes, next to each texture, a file that says where every image
landed.

A picture that differs from the game is worse than none, and a packer written again
differs the day Defold changes its own. The build tool's answer has one drawback: it
builds what a game reaches from the collection it starts with, and stops at the first
mistake anywhere. An atlas nothing uses yet would have no picture, and neither would
any atlas of a game that does not build.

## Decision

The places come from Defold's build tool, and the atlas is packed alone.

- **`defoldpls pack <atlas>`** has the build tool of the installed editor (0015) build
  a project made for that atlas: the atlas and its images, at the paths they have in
  the game, and one sprite that shows it. It then reads what the build wrote and gives
  each line of the atlas its place, as JSON.
- **That project lives in the cache, never in the game** (0014 is not called on:
  nothing is written). It is kept between runs, since the build tool builds again only
  what changed, and follows the atlas: an image that is no longer named is taken out.
- **Reading stays inside the project and writing inside that directory**, whatever
  path an atlas spells.
- **What is read is the texture set**, by the little of its encoding that is needed.
  How it counts was settled against the texture of the same build, pixel by pixel: the
  numbers go upwards from the bottom of the page, and an image the build turned lies a
  quarter turn clockwise.
- **The VS Code extension draws it**: the images of the game themselves, each in its
  place, in a view next to the text of the atlas. It opens with the atlas, beside the
  text or below it, whichever leaves the page larger, and only rearranges a window that
  holds just the two.

## Consequences

- The picture is the build's, for whatever version of Defold is installed, with no
  packer to keep in step.
- It needs the installed editor and takes a few seconds, most of them the start of the
  build tool. It is packed again when the atlas or one of its images is saved, not
  while typing.
- What is not saved is not shown.
- The texture profiles of the game do not go along, so what they change, such as the
  largest size of a texture, is not in the picture.
- The view draws the files of the images, not the texture: borders that the build
  extrudes around an image are left as gaps.
- The texture set is Defold's to change. A test reads one that Defold 1.14.0 wrote.
- This was tried with the build tool of 1.14.0 on Windows, and the page in a browser
  of the kind VS Code is built on. The view was not looked at in VS Code itself.
