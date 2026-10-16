# Contributing to vlx

Thanks for helping! vlx aims to be the friendliest way to explore Varlink, and
the safest. Both matter in every change.

## Getting set up

You need Go 1.24 or newer. Nothing else: the tests run against vlx's built-in
demo service, so you do not need systemd (macOS works too).

```sh
git clone https://github.com/rossedman/vlx
cd vlx
make test        # unit and end-to-end tests
make run-demo    # build and open the explorer on the demo service
```

Run `make help` for every task.

## Layout

```text
cmd/vlx/            entry point
internal/cli/       one Cobra command per file (call.go, list.go, ...)
internal/tui/       the Bubble Tea explorer
internal/safety/    READ / WRITE / DANGER classification and policy
internal/schema/    parameter skeletons and validation
internal/...        discovery, client, render, sanitize, config, demo
docs/               MkDocs site (docs/project/spec.md is the design)
hack/               scripts for screenshots, diagrams and completions
```

See [Architecture](https://rossedman.github.io/vlx/project/architecture/).

## Making changes

- **Behaviour changes start in the spec.** Update `docs/project/spec.md` in
  the same pull request.
- **New commands** go in their own file under `internal/cli/`, registered in
  `root.go`, with `Short`, `Long` and `Example` text. Regenerate the reference
  with `make docs-reference`.
- **Safety first.** Anything that sends a message must go through
  `safety.Classifier` and `safety.Policy`. Anything that prints data from a
  service must go through `sanitize` or `render`. New systemd methods need a
  rule in `internal/safety/builtin.go`; `make test` tells you when one is
  missing (`hack/gen-safety-test.sh` refreshes the list after an sdvarlink
  upgrade).
- **TUI changes** should keep the view inside 80×24 and add a test in
  `internal/tui/tui_test.go`. `make screenshots` refreshes the docs images.

## Before you push

```sh
make lint test
```

CI runs the tests with the race detector on Linux (amd64, arm64) and macOS,
golangci-lint, and a GoReleaser snapshot build for every platform.

## Commit messages

Use [Conventional Commits](https://www.conventionalcommits.org/): `feat:`,
`fix:`, `safety:`, `docs:`, `test:`, `ci:`, `chore:`. The release notes are
grouped by these prefixes.

## Releasing

Tag `vX.Y.Z` on `main` and push the tag. The Release workflow tests, builds
every platform with GoReleaser and publishes the GitHub release with archives,
packages and checksums.
