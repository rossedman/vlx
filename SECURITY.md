# Security policy

vlx is a low-level tool that talks to privileged system services, often as
root. We take its safety guarantees seriously:

- it sends nothing but READ methods unless writes are explicitly enabled;
- WRITE and DANGER methods are always confirmed;
- data from services cannot inject terminal escape sequences;
- it never escalates privileges, spawns processes or keeps history.

A way around any of these is a security bug.

## Reporting a vulnerability

Please **do not open a public issue**. Report it privately through
[GitHub security advisories](https://github.com/rossedman/vlx/security/advisories/new).
Include the vlx version (`vlx version`), your OS and systemd version, and
steps to reproduce.

You will get an answer within a week. Fixes are released as soon as they are
ready, and credited unless you prefer otherwise.

## Supported versions

Security fixes go into the latest release.

## Scope

In scope: vlx itself — the safety gate and classifications, sanitisation,
redaction, config handling, discovery and the demo service.

Out of scope: vulnerabilities in the services vlx talks to (report those to
systemd or the service's authors), and in sdvarlink (report those to
[sdvarlink](https://github.com/rossedman/sdvarlink/security)).
