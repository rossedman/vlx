# Changelog

All notable changes to vlx are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and vlx uses
[semantic versioning](https://semver.org/).

## Unreleased

### Changed

- The explorer is now a drill-down with a command line: one page at a time
  (services → a service → a method) and a line along the bottom that is
  always there. Type to jump anywhere, or type a whole call —
  `resolve ResolveHostname name=example.com` — and press Enter. On a method
  page the parameter form and the command line show the same call and stay
  in sync. The three-pane layout and the Ctrl+K palette are gone; Ctrl+K now
  focuses the command line.
- Services on the home page are grouped (System, Network, Users & login, …)
  and described in plain language; recent calls are listed first.
- A method page lists earlier calls of that method, and keeps its parameters
  and result while vlx runs.

### Added

- `vlx call SERVICE METHOD NAME=VALUE...`: parameters as words, converted
  using the IDL, as an alternative to a JSON object.
- Parameter form on the call screen: one row per parameter with a fitting
  input (text, number, enum and bool pickers, JSON for complex types),
  inline docs and per-field errors. Ctrl+J switches to raw JSON and back.
- Preview: services are introspected (read-only) as the cursor rests on
  them, so their interfaces appear without opening them. `preview: false`
  or `--no-preview` turns it off.
- Search palette (Ctrl+K or `:`) across every service, interface, method,
  type and error, with background indexing and multi-word fuzzy matching.
- Boilerplate interfaces (`org.varlink.service`, `io.systemd.service`) start
  collapsed.

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
