# 0007. No comments in source files

Status: accepted, 2026-10-09

## Context

A comment is a second description of the code, kept next to it and checked by nothing.
When the code changes and the comment does not, the reader is told something false by
the one line they were most likely to trust.

Most comments also make up for something the code could have said: a name that is too
short, a function that does two things, a constant with no name.

## Decision

A source file that is not a test carries no comments. What the code does is said by
its names and its structure. Why it does it is said in a decision record, in a test
name, or in the commit message.

Three kinds of comment remain, because something reads them:

- directives for the toolchain and the linter (`//go:build`, `//nolint:` with the
  linter named and the reason given);
- the package comment of a command, kept in a `doc.go`, which is what `go doc` and the
  package index show;
- comments in test files, where they say why a case exists.

A test in `internal/conventions` fails the build when any other comment appears, and
the linters that demand a comment on every exported name are switched off.

## Consequences

- Naming is held to a higher standard, because nothing else will explain the code.
- The reasons for a design are in one place and are reviewed when they change.
- Packages under `internal` have no generated documentation. They are not an API.
- If defold-vet ever exposes a public Go package, its exported names will need
  documentation, and this decision will need a new record.
