# Security policy

## Supported versions

| Version | Supported |
|---|---|
| The one Lanterna Games is using this week | Yes |
| Every other one | Thoughts and prayers |

There is no release. Security fixes go into whatever was built last, when it was
Lanterna Games that got bitten.

## Reporting a vulnerability

You may report one. Here is the whole process, so that nobody is kept waiting:

1. Nothing happens.

There is no security team, no embargo, no CVE, no bounty and no responsible disclosure
window, since nobody here is responsible for disclosing anything to anyone. If it
bothers you, the fix is yours to write, in a fork that is yours to keep.

## What you are actually running

Honesty is the one security feature that shipped complete:

- `defold-vet` reads the files of the project it is pointed at. It does not write to
  them and does not run the project's code.
- `defoldpls` is less shy. When asked, it moves and creates files in the project,
  starts the build tool and the engine of the Defold editor installed on the machine,
  lets that tool send native extensions to Defold's build server, and serves the
  reference on a port of `127.0.0.1`.
- Project files are untrusted input, read by a program that nobody has audited.

It comes with no warranty. The [licence](LICENSE) says so in capital letters, which is
how lawyers raise their voice.

## What counts

Nothing counts. A check that reports a mistake that is not there, or misses one that
is, is not a vulnerability either. It is a bug, and bugs have a policy of their own,
in [CONTRIBUTING.md](CONTRIBUTING.md). It is the same policy.
