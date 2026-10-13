# 0013. The engine API is handed to the Lua language server, and generated here

Status: accepted, 2026-10-10

## Context

A Defold project is edited with completion for `go.animate` and its hundreds of
siblings. Something has to know that API. The Lua language server, which most editors
already run for Lua, learns an API from annotation files, and does the rest better than
a second implementation would: completion, hover, signatures, type checking.

Defold publishes such files itself, inside the reference archive of each release. They
are there from 1.11.0, and carry a link to the reference from 1.13. The documentation
they are made from is in the same archive from 1.6.0, which is where the supported
window starts.

## Decision

defoldpls does not complete or describe the engine API. It generates annotation files
from the documentation of the project's Defold version and lets the Lua language
server use them.

The files are generated here, for every version of the window, and Defold's own are not
passed on. Every function ends with two links: one to the reference kept on the
machine, one to defold.com.

The reference kept on the machine is served by defoldpls over HTTP, on the loopback
address only and on a port the system assigns. The port is remembered between runs so
that links stay valid, and servers started by several editors share one.

The documentation of the newest version is carried by the program. Other versions are
fetched once and kept, and can be fetched ahead of time.

The annotations are written to the user's cache directory. Pointing the Lua language
server at them means a setting in the editor, which the VS Code extension writes into
the project's folder settings only after asking. [0004](0004-read-only.md) is about the
project's data and holds; an editor's settings file is the editor's, and is still not
touched without consent.

## Consequences

- defoldpls is useless for the API without the Lua language server, and says so.
- A project on 1.6.0 gets the same completion and the same links as one on 1.13.2.
- The generator has to follow the documentation as its shape changes. Eleven versions
  from 1.6.0 to 1.13.2 were generated and passed the Lua language server's own check.
- The standard library and the editor API are not annotated.
- The port changes if another program takes the remembered one, and the annotations are
  then written again with the new address.
- What only this project can know stays here: the names of a game's own objects,
  messages and properties, which are the next thing defoldpls is to complete.
