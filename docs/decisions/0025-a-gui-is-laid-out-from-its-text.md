# 0025. A GUI is laid out from its text

Status: accepted, 2026-10-13

## Context

[Decision 0022](0022-a-scene-is-laid-out-from-its-text.md) left the GUI scene as a mark
where a game object holds one. It is the file a text editor shows worst: a list of
nodes, each with a position inside a parent that is named somewhere else in the list,
and a box that lies around that position according to a pivot. Of the 1,512 GUI scenes
of the sample projects, 196 also bring nodes in from other scenes, as templates, and
say which of their fields they change.

All of it is in the text, as it is for a collection: positions, rotations, scales,
sizes, pivots, parents, the texture and the text of a node, its colour, its layer.

## Decision

`defoldpls scene` takes a GUI scene and gives every node the place it ends up in, in
the display of the game, and the VS Code extension draws it next to the text.

- **The place is the engine's.** A node is placed in its parent by its position, then
  its rotation, about X, then Z, then Y, then its scale; a position that is not
  written is 0, a size 0, a scale 1; the box lies around the place as the pivot says.
  This was read off the engine of 1.14.0: a scene written for the purpose was built and
  run without a window, and a script printed where `gui.get_screen_position` puts each
  node and which points of the display `gui.pick_node` says it covers. The tests hold
  the layout to those numbers.
- **A template is followed as the build follows it.** Its nodes come under the id of
  the template node. A field is taken from the scene that holds the template only when
  that scene names it in `overridden_fields`, by the number the field has in Defold's
  description of a node; anything else it writes for the node is what the editor last
  saw in the template, and is not used. The numbers were read from the editor of
  1.14.0, and the study has a node that proves each half.
- **Drawn**: a box with a texture as the image of its animation, stretched to the size
  of the node with the edges of its `slice9` kept; a box without one in its colour; a
  text as its words, at the size its font file gives; a pie as its shape. Each with its
  alpha, inherited when the node says so.
- **Outlined, not drawn**: a node the game does not show, which is one that is not
  enabled, anything under it, and one that is not visible. Someone laying out a screen
  wants to see where it is.
- **In the order it is drawn**: by layer, in the order the scene lists its layers, a
  node without one going with its parent, then in the order of the tree.
- **In a frame**: the display `game.project` asks for, or Defold's 960 by 640.
- **It has an end**: 20,000 nodes, and a template that holds itself is followed once.

## Consequences

- A GUI scene can be laid out from a text editor, and a click on a node finds the
  lines that write it, a node of a template included.
- Only the default layout is shown, at the size of the display. Anchors and adjust
  modes act when the window is another size, and that is the game's to show.
- The engine turns a node with a table of sines that is good to a thousandth. Exact
  sines are used here, so a node 100 from a parent turned 30 degrees is drawn a fiftieth
  of a pixel from where the engine has it, and the node of the study under a parent
  turned about three axes at once, 0.15 of a pixel.
- A font is not rendered: the words are set in the font of the editor at the size the
  font file names, so the width of a line is not the game's. A font that is not a file
  of the project, such as the one Defold carries, gives no size, and the words are
  fitted to the box.
- A texture is the whole image of the animation's first frame, without the tint of the
  node. Clipping, materials, particle effects and the nodes extensions add are not
  shown, the last two as a mark.
- What no call of the engine tells was read off a picture it drew, of a game bundled
  for the web and opened in a browser without a window: the edges a box keeps are left,
  top, right, bottom; a pie is filled from the right, counterclockwise when the fill is
  positive; a box is moved by the pivot its image has in the atlas, on top of the pivot
  of the node, and mirrored when its animation is flipped.
- Seen with 1.14.0 only. The 1,512 scenes of the sample projects are laid out, none
  refused and none cut short, in 16 seconds together.
