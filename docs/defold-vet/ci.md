# In CI and git hooks

`defold-vet check` is made to be a gate: it prints one problem per line and its exit
code says whether there were any.

| Code | Meaning |
|---|---|
| 0 | Nothing was found |
| 1 | Problems were found |
| 2 | It could not run |

It needs no Defold editor and no build of the game: it reads the project's files.

## GitLab CI

```yaml
defold-vet:
  image: golang:1.27
  script:
    - go install github.com/Lanterna-Games/defold-tools/cmd/defold-vet@latest
    - defold-vet check
```

## GitHub Actions

```yaml
jobs:
  defold-vet:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.27"
      - run: go install github.com/Lanterna-Games/defold-tools/cmd/defold-vet@latest
      - run: defold-vet check
```

## Before a commit

With [lefthook](https://lefthook.dev):

```yaml
pre-commit:
  commands:
    defold-vet:
      run: defold-vet check
```

Or as a plain git hook, in `.git/hooks/pre-commit`:

```sh
#!/bin/sh
exec defold-vet check
```

## Things to settle first

**Pin what you run.** `@latest` follows defold-vet, and a new check can turn a green
pipeline red on a day nobody touched the game. Install a fixed version, or a fixed
commit while there are no releases, and raise it on purpose.

**Declare the Defold version.** Without [`defold`](settings.md#defold) in
`defold-vet.toml`, the engine's messages and properties are those of the version built
into the defold-vet you installed, so updating defold-vet can change the answers. With
it, they change when the project says so.

**Keep defold-vet at least as new as the Defold you declare.** Every version from
1.6.0 to the newest one it was built with is carried by the program and needs no
network. A version newer than the program is fetched on each run of a job that starts
with an empty cache directory (`$XDG_CACHE_HOME/defold-vet`, or `~/.cache/defold-vet`,
on Linux).

**Libraries are read, not fetched.** A project that depends on libraries is
[judged less](how-it-reads.md#code-out-of-sight) while their code is not there. Running
`bob resolve` first puts them in `.internal/lib`, where defold-vet reads them, and the
project is then judged in full. Without that step the job still passes or fails on what
can be judged from the project's own files.

**Decide what strict mode is for.** [Strict mode](strict-mode.md) is on by default and
fails the job for every name that cannot be read. A project that is not ready for that
can set `strict = false` and keep the other checks as the gate.
