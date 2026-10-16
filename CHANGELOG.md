# Changelog

All notable changes to vlx are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and vlx uses
[semantic versioning](https://semver.org/).

## Unreleased

### Added

- The explorer: discover Varlink sockets, browse interfaces, methods, types
  and errors with their documentation, view raw IDL, filter everything.
- The call screen: parameters pre-filled from the IDL and validated as you
  type, live streaming replies, typed errors with docs and hints, redaction,
  and copyable `vlx call` / `varlinkctl call` commands.
- Commands: `explore`, `list`, `info`, `describe`, `template`, `call`,
  `demo`, `config`, `version` and shell completion.
- Safety: READ / WRITE / DANGER classification with a built-in rule for
  every `io.systemd.*` method in systemd v257–v260, read-only by default,
  confirmations for writes, typed confirmation for dangerous methods,
  terminal escape sanitisation, config permission checks.
- `vlx demo`: a built-in sample service for learning and testing.
- A 90% test-coverage minimum, enforced in CI and by `make cover`.
- Releases for Linux and macOS on six architectures, with `.deb`, `.rpm` and
  `.apk` packages, completions and man pages.
