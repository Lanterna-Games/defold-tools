# 0009. Strict mode reports names, not values

Status: accepted, 2026-10-10

## Context

[0005](0005-literals-and-strict-mode.md) says that in strict mode a reference that
cannot be verified is reported as unverifiable, and counts among those a URL received
in a message and the id of an object a factory spawned.

Taken literally that reports most of the calls in a Defold project. `msg.post(sender,
...)`, `go.delete(id)` and `go.set(self.target, ...)` pass an address the engine handed
over, and there is no literal to move it into. A report that cannot be acted on, on
most lines, would be turned off, and the cases that can be acted on would go with it.

Run over 393 open-source projects, the rule below adds about 1570 reports to the 250 of
the other checks, in 172 of the projects.

## Decision

Strict mode reports, under the check `unverifiable`, a **name** that is not written
where a check would have read it:

- the id of a message that is posted;
- the name of a property, an animation, a collision group, a GUI node, an input action;
- an address **put together from text** while the game runs, as in `"/enemy" .. n`.

It reports, once per script, the place where the script stops being readable as a
whole: a message handler that does more with the id than compare it, a node named with
an id that cannot be read, an action passed to `on_input` that cannot be read, a
property declared under a name that cannot be read. The report says what is no longer
judged because of it.

It does not report an address held in a value: a URL in a variable, a field of `self`,
`sender`, the result of `factory.create`. That is how the engine hands addresses out.

A name counts as written when it is a string, a `hash` of one, or a constant that holds
one: a local of the file, a field of a table of the file, or a field of the table a
required module returns, set once and at the top level.

Strict mode is on by default. `strict = false` leaves it, and since `unverifiable` is a
check like the others, it can also be ignored on a line or in the files a pattern
matches, with a reason.

## Consequences

- Ids kept in a module of constants are verifiable, so the usual way of organising
  them is not punished.
- A project that picks animations or GUI nodes by name at run time has many lines to
  silence, or leaves strict mode. Each silenced line carries its reason.
- A script that hands the message id to a library, as the common GUI libraries ask, is
  reported once: it does count as handling every message, and saying so is the point.
- An address in a variable is checked only where the value was written, if it was.
