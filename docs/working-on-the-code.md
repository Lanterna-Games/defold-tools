# Working on the code

What the code holds itself to. It is written down for whoever works on it and for
whoever forks it.

## Setting up

You need Go 1.27 or newer. Everything else is pinned in `mise.toml`:

```sh
mise install     # Go, golangci-lint, lefthook, shellcheck, gitleaks
mise run hooks   # optional: run the checks before each commit and push
make check       # every check there is
```

`make help` lists the other targets. Without mise, install the tools at the versions
in `mise.toml`.

One test needs real projects and is skipped without them. With `DEFOLD_TOOLS_SAMPLES`
set to a directory that holds Defold projects, at any depth, it renames names in each
of them in memory and checks that the project reads the same afterwards:

```sh
DEFOLD_TOOLS_SAMPLES=~/defold-samples go test ./internal/insight/ -run RealProjects
```

Anything that changes a project file has to pass it. The projects are not copied into
the repository.

## How the code is written

**Source files carry no comments.** Names and structure have to say what the code
does. Why it does it goes where it will not drift: a decision record, a test name, or
the commit message. Three kinds of comment remain, because something reads them:

- directives for the toolchain and the linter, such as `//go:build` and `//nolint:`;
- the package comment of a command, in its `doc.go`;
- comments in test files, where they say why a case exists.

A test enforces this (`internal/conventions`), and
[decision 0007](decisions/0007-no-comments-in-source.md) gives the reasoning.

**A silenced linter says why.** `//nolint:` names the linter and carries the reason on
the same line.

**Errors are wrapped, not restated.** `fmt.Errorf("reading %s: %w", path, err)`.

**No dependency without a reason.** The standard library is the default. A new module
is a decision and gets a record.

## How a check is written

A check is only as good as its silence.

- It has a test where it fires on a real mistake and a test where it stays quiet on
  valid code that looks similar.
- Test projects live in `testdata` and are small enough to read in one screen.
- A check that reports a problem that is not there is a bug, and outranks a check that
  misses one.
- defold-vet reads project files and never writes them.

## Commits

Commits follow [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/):

```text
feat: check relative urls against the components of the game object
fix(parser): keep the column after a tab
docs: explain how the Defold version is resolved
```

The types are `build`, `chore`, `ci`, `docs`, `feat`, `fix`, `perf`, `refactor`,
`revert` and `test`. The subject is in the imperative, has no final period and fits in
72 characters. The body says why, since the code does not. `make check` checks the
subjects of the commits that are not on `main` yet.

## A change

1. Branch from `main`.
2. Keep the change small enough to read and to revert on its own.
3. Add an entry to `CHANGELOG.md` under *Unreleased* if a user would notice the change.
4. `make check` has to pass. Nothing else runs it for you: there is no pipeline.

A decision that outlives the change, such as a new dependency, a change of scope or a
rule for the codebase, is written down in [`docs/decisions`](decisions/README.md)
along with it.

## Releases

Versions follow [Semantic Versioning](https://semver.org/). A release is a tag on
`main`, made by a maintainer, with the *Unreleased* section of the changelog renamed
to the version.
