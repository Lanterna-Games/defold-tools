# Defold API reference data

The files here are generated. Do not edit them.

There is one per Defold version defold-vet supports, from 1.6.0 on, gzipped. Each is
extracted from what the Defold Foundation publishes for that release at
`d.defold.com`: from the reference documentation, the names of the Lua functions of
the engine, the names and types of their parameters, the engine's messages and their
fields, and the properties of each kind of component; from `builtins.zip`, the
constants, attributes and samplers of the builtin materials and the actions of the
builtin input binding. Defold and its documentation are covered by the
[Defold License](https://defold.com/license/), a copy of which is in
[`licenses`](../../../licenses/Defold-License-1.0.txt). The [notice](../../../NOTICE)
of this repository says what that asks of whoever passes these files on.

To read one:

```sh
zcat internal/engine/reference/1.13.2.json.gz | jq .
```

## Regenerating

After a Defold release, or after a change to what is extracted:

```sh
go run ./internal/engine/generate
```

It lists the tags of the Defold repository with `git`, fetches the two archives of
every version in the window, and writes all the files again. The archives are kept in
the user's cache directory, so a second run fetches only what is new.

All the versions are written together because each is read against its neighbours:
something documented before and after a version and missing from it is taken to be a
hole in that version's documentation, and is put back. The `filled` list of a file
says what was. This is why adding a version can change the file of the one before it.
