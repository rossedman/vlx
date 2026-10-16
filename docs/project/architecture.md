---
description: How vlx is put together — packages, data flow and the libraries it builds on.
---

# Architecture

![vlx architecture](../assets/architecture.png){ .vlx-diagram }

## Packages

```text
cmd/vlx/main.go            entry point; maps errors to exit codes
internal/cli/              one Cobra command per file
internal/config/           Viper setup, defaults, config permission checks
internal/discovery/        socket discovery and SERVICE resolution
internal/client/           sdvarlink wrapper: introspect, call, stream, hints
internal/schema/           IDL → JSON skeletons; JSON → IDL validation
internal/safety/           READ/WRITE/DANGER classification and policy
internal/sanitize/         control-character stripping for the terminal
internal/render/           JSON and IDL highlighting, redaction, palette
internal/tui/              Bubble Tea model: browse, call, help screens
internal/demo/             the built-in demo Varlink service
internal/version/          build metadata set by GoReleaser
```

Dependencies point one way: `cli → tui → client · schema · safety · render →
sdvarlink`. Only `client` and `demo` open sockets.

## A call, end to end

1. **Discovery** lists sockets (`discovery.Discover`); nothing is sent.
2. Opening a service runs `client.Introspect` in a `tea.Cmd`: one
   connection, `GetInfo`, then `GetInterfaceDescription` for each interface,
   parsed with sdvarlink's `idl` package. A broken interface does not hide the
   others.
3. Opening a method builds a **skeleton** from its input type
   (`schema.Skeleton`) and validates every edit (`schema.Validate`).
4. Sending passes the **safety gate**: `safety.Classifier` gives a class,
   `safety.Policy` decides whether to refuse, confirm or send.
5. `client.Call` dials, sends with sdvarlink's `Conn.CallReply`, `Conn.Stream`
   or `Conn.Oneway`, closes any received file descriptors and hands each reply
   back over a channel. The UI drains it with `tea.Cmd`s, so it never blocks.
6. Replies are rendered by `render.JSON`: key order preserved, strings
   sanitised, secrets redacted.

## Libraries

| Library | Used for |
| --- | --- |
| [sdvarlink](https://github.com/rossedman/sdvarlink) | Varlink wire protocol, calls and streams, IDL parsing |
| [Bubble Tea](https://github.com/charmbracelet/bubbletea) | the TUI's model–update–view loop |
| [Bubbles](https://github.com/charmbracelet/bubbles) | textarea, textinput, viewport, spinner, help |
| [Lip Gloss](https://github.com/charmbracelet/lipgloss) | styles, layout and the adaptive palette |
| [Cobra](https://github.com/spf13/cobra) | commands, flags, completion, man pages |
| [Viper](https://github.com/spf13/viper) | layered configuration |

## Testing

- Unit tests cover every package; `TestBuiltinCoversSystemd` imports the IDL
  of every systemd release sdvarlink ships and fails when a method lacks a
  safety rule.
- The CLI tests run real commands against the demo service over a real
  socket, checking output and exit codes.
- The TUI tests drive the Bubble Tea model with key messages and assert on
  state and rendered views — including that the view fits 80×24 and that
  escape sequences from a service never reach the terminal. The same tests
  produce the screenshots in these docs (`hack/screenshots.sh`).
