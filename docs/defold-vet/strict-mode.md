# Strict mode

The [checks](checks.md) judge what is written. What is not written they leave alone,
and a mistake can sit there unseen. Strict mode reports those places, under the check
`unverifiable`. It is on unless a project leaves it.

```lua
sprite.play_flipbook("#sprite", self.skin)
```

```
main/ball.script:15:37: the animation played here cannot be read [unverifiable]
```

Nothing is known to be wrong on that line. What the report says is that nobody checked.

## What is reported

A **name** that is not written where a check would have read it:

| | The report |
|---|---|
| The id of a message that is posted | `the id of the message posted here cannot be read` |
| The name of a property | `the name of the property cannot be read` |
| An animation | `the animation played here cannot be read` |
| A collision group | `the collision group named here cannot be read` |
| A GUI node | `the id of the node asked for here cannot be read` |
| An input action | `the action compared here cannot be read` |
| An address put together from text, as in `"/enemy" .. n` | `this address is put together while the game runs` |

And, once per script, the place where the script stops being readable as a whole. The
report says what is no longer judged because of it:

```lua
function on_message(self, message_id, message, sender)
    dispatcher.on_message(message_id, message)
end
```

```
main/ball.script:13:27: the message id is used here for something other than a comparison, so this script counts as handling every message [unverifiable]
```

The others of this kind:

- `msg.post is used here as a value, so what is posted through it cannot be read`
- `the id given to this node cannot be read, so the nodes this script asks for are not judged`
- `the action passed to on_input here cannot be read, so the actions of this script are not judged`
- `the name of the property declared here cannot be read, so the properties of this script are not judged`

[What counts as written](how-it-reads.md#what-counts-as-written) is the same as for
the other checks: a string, a hash of one, or a constant that holds one, a constant of
a required module included.

## What is not reported

An address held in a value: a URL in a variable, a field of `self`, `sender`, what
`factory.create` returned.

```lua
msg.post(sender, "ack")
go.delete(self.bullet)
```

That is how the engine hands addresses out, and there is no literal to move them into.
Only an address made of text at run time is reported, because that one was spelt by
hand.

## Living with it

Most of what strict mode reports goes away by giving the id a name:

```lua
local ids = require("main.ids")
msg.post(self.game, ids.BALL_OUT)            -- read: a constant of a required module

local ROLL = hash("roll")
sprite.play_flipbook("#sprite", ROLL)        -- read: a constant of the file
```

Where the choice is between a few names, a branch keeps each of them readable, at the
price of saying the call twice:

```lua
if self.team == RED_TEAM then
    sprite.play_flipbook("#sprite", BALL_RED)
else
    sprite.play_flipbook("#sprite", BALL_BLUE)
end
```

Whereas these are computed while the game runs, and are reported:

```lua
sprite.play_flipbook("#sprite", self.team == RED_TEAM and BALL_RED or BALL_BLUE)
sprite.play_flipbook("#sprite", SKINS[self.team])
gui.get_node("row_" .. index)
```

Some code picks names at run time and should. For that there are three ways out, from
the narrowest to the widest:

1. Say why, on the line, and keep the rest strict:

   ```lua
   -- defold-vet:ignore unverifiable the skin is chosen in the shop
   sprite.play_flipbook("#sprite", self.skin)
   ```

2. Leave it out of the files where it is the rule, in `defold-vet.toml`:

   ```toml
   [[ignore]]
   check = "unverifiable"
   path = "gui/**"
   reason = "the lists build their node ids from the row"
   ```

3. Leave strict mode:

   ```toml
   strict = false
   ```

## What to expect

Run over 393 open-source Defold projects, strict mode reports nothing in 221 of them
and about 1570 places in the other 172, next to 250 problems from the other checks.
The largest shares are GUI nodes whose id is built from an index and animations picked
from a value. A project written with ids in constants from the start sees few.

[Decision 0009](../decisions/0009-what-strict-mode-reports.md) records why strict mode
reports names and not values.
