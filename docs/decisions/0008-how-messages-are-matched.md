# 0008. A message is matched by what `on_message` compares

Status: accepted, 2026-10-10

## Context

Nothing in Defold declares which messages a script accepts. The only evidence is the
body of `on_message`, and the only evidence that a message is sent is a call to
`msg.post`. Both are ordinary Lua, so both can be written in ways that say nothing to
a reader that does not run the code: a handler that passes the id to a dispatcher, a
post whose id arrives in a parameter, a library that does either from code that is not
in the project.

[0005](0005-literals-and-strict-mode.md) settles that an unhandled message is reported
even though it is an inference. This record settles what the inference rests on.

## Decision

A script handles a message when its `on_message` compares the message id with it in the
condition of an `if`: `==` against a string, a `hash("...")` or a constant of the file
that holds one, or `~=` in a clause that does nothing but return.

Any other use of the id makes the script one that handles everything: passing it on,
indexing a table with it, comparing it with a name that is not a constant of the file,
an `else` after a test of the id, a handler that has a body and never looks at the id.
So does a script that did not parse.

A post is judged against the script it is addressed to when the address is written in
the call. Otherwise it is judged against every script in the project.

A handler is reported as never posted to only when every `msg.post` in the project has
an id that can be read. A mention of `msg.post` that is not a call counts as one that
cannot.

A project that declares a library or contains a native extension runs code that is not
read. There, only a post addressed to a script that is read is judged.

The engine's own messages come from the reference of the Defold version and are never
the project's to post or handle. The render script is not asked who posts to it.

## Consequences

- The common way of writing a handler, a chain of `if message_id == ...`, is checked in
  full. A project that routes messages through a dispatcher is checked less, and one
  dispatcher is enough to silence the project-wide half of the check.
- One misspelt id is usually reported twice: where it is posted, and where the handler
  waits for the right spelling.
- A message the engine accepts and does not document is reported when posted to a
  component that is not a script.
- Strict mode is where the cases that are skipped here will be reported as
  unverifiable.
