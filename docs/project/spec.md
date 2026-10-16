---
description: The design specification for vlx, the Varlink explorer — goals, user experience, commands, the safety model and architecture.
---

# Specification

!!! info "Status"
    This is the living design document for vlx. It describes v0.1 and records
    the decisions behind it. Changes to behaviour start as a change to this page.

## 1. Summary

**vlx** (*Varlink explorer*) is a terminal UI and command-line tool for
discovering, reading and calling [Varlink](https://varlink.org/) services —
above all systemd's `io.systemd.*` APIs. It is to Varlink what `d-feet` and
`busctl` are to D-Bus: point it at a host and browse every service, interface,
method, type and error, read the documentation that ships in the IDL, and call
methods with a parameter editor that knows the schema.

vlx is built on [sdvarlink](https://github.com/rossedman/sdvarlink) for the
wire protocol, introspection and IDL parsing, on Cobra and Viper for the CLI
and configuration, and on Charm's Bubble Tea, Bubbles and Lip Gloss for the
interface.

## 2. Goals

1. **Zero to useful in one keystroke.** `vlx` with no arguments finds the
   Varlink sockets on the host and opens the explorer. No flags, no config.
2. **Discoverable.** Every screen shows the keys that work on it. Anything the
   TUI can do, it can also print as the equivalent `vlx` and `varlinkctl`
   command, so the TUI teaches the CLI.
3. **Safe by default.** vlx starts read-only. Methods that change the system
   need an explicit opt-in *and* a confirmation; destructive ones need the
   method name typed back. See [§6](#6-safety-model).
4. **Schema-aware.** Parameters are pre-filled from the IDL and validated
   locally before anything is sent.
5. **Scriptable.** Every TUI capability has a non-interactive command with
   stable JSON output and meaningful exit codes.
6. **Beautiful and accessible.** Adaptive light/dark colours, honours
   `NO_COLOR`, works in 80×24, never relies on colour alone to convey meaning.

### Non-goals (v0.1)

- Implementing Varlink services (vlx is a client; `vlx demo` is the one
  exception and serves a fixed, harmless interface).
- Remote transports (`ssh:`, `exec:`, TCP). vlx speaks to Unix sockets only.
  Spawning processes is out of scope by design (see [§6.5](#65-attack-surface)).
- Passing file descriptors *to* services. Received descriptors are closed and
  reported.
- Persisting call history (it may contain secrets).

## 3. Users and scenarios

| Who | Scenario | What vlx gives them |
| --- | --- | --- |
| systemd user / sysadmin | "What can I ask `systemd-resolved` over Varlink?" | Browse `io.systemd.Resolve`, read docs, run `ResolveHostname` read-only. |
| Developer on sdvarlink or a Varlink service | "Does my service answer `Describe` correctly?" | Point vlx at the socket, call the method, inspect raw JSON and errors. |
| Script author | "I need the hostname as JSON in CI." | `vlx call hostname io.systemd.Hostname.Describe -o json`. |
| Learner | "What is Varlink?" | `vlx demo` serves a sample service anywhere, including macOS. |

## 4. User experience

### 4.1 Launch

```text
vlx                         # discover sockets, open the explorer
vlx explore hostname        # open straight at a service (name, path or unix:path)
vlx demo                    # start the built-in demo service and explore it
```

Services are named by socket path. Wherever a SERVICE argument is accepted,
vlx also accepts:

- an absolute or relative path (`/run/systemd/io.systemd.Hostname`),
- the `unix:` form that `varlinkctl` uses (`unix:/run/systemd/io.systemd.Hostname`),
- a discovered socket's base name (`io.systemd.Hostname`), or a
  case-insensitive unique fragment of it (`hostname`).

Ambiguous fragments are an error that lists the candidates.

### 4.2 Explorer layout

```text
╭ vlx ─ /run/systemd/io.systemd.Hostname ───────────── READ-ONLY ─ uid 1000 ╮
│ Services          │ Interfaces            │ io.systemd.Hostname.Describe  │
│ ● Hostname        │ ▾ io.systemd.Hostname │ READ · single reply           │
│   Manager         │   ƒ Describe     READ │                               │
│   Resolve         │   ⌁ Monitor      READ │ method Describe () -> (       │
│   userdb          │ ▸ io.systemd.service  │   Hostname: string,           │
│   …               │ ▸ org.varlink.service │   …                           │
├───────────────────┴───────────────────────┴───────────────────────────────┤
│ ↑↓ move  ⇥ pane  ⏎ open  c call  i IDL  / filter  ? help  q quit          │
╰───────────────────────────────────────────────────────────────────────────╯
```

Three panes, left to right — **Services**, **Interfaces** (a tree of
interfaces and their methods, types and errors) and **Detail** — with a
header and a context-sensitive key bar.

- The **header** shows the current socket, the safety mode badge
  (`READ-ONLY` green, `WRITES ENABLED` amber) and, when running as root, a red
  `ROOT` badge.
- **Services** lists discovered sockets with a status dot: grey not yet
  probed, green answered `GetInfo`, red failed (the error is shown in Detail).
- **Preview.** When the cursor rests on a service for 250 ms, vlx introspects
  it in the background (only `org.varlink.service.GetInfo` and
  `GetInterfaceDescription`, which are read-only) and shows its tree in the
  Interfaces pane without moving focus. Results are cached per socket, so
  moving back is instant. Sockets the user cannot access are not previewed.
  `preview: false` (or `--no-preview`) restores connect-on-Enter.
- **Boilerplate interfaces** (`org.varlink.service`, `io.systemd.service`,
  `io.systemd`) start collapsed so the service's own API is what you see.
- **Interfaces** marks members with glyphs *and* letters so meaning never
  depends on colour: `ƒ` method, `⌁` streaming method, `τ` type, `!` error,
  and a safety tag `READ` / `WRITE` / `DANGER`.
- **Detail** renders the selected item: signature with syntax highlighting,
  the doc comment, input and output parameters as a table with named types
  expanded one level, streaming behaviour and safety classification with the
  reason (e.g. *"name starts with Get"*).

### 4.3 Keys

| Key | Action |
| --- | --- |
| `↑`/`↓`, `k`/`j` | Move |
| `←`/`→`, `h`/`l`, `Tab`/`Shift+Tab` | Change pane |
| `Enter` | Open: connect to service / expand interface / call method |
| `Ctrl+K`, `:` | Search everything (see [§4.5](#45-search-palette)) |
| `c` | Call the selected method |
| `i` | Toggle the raw IDL of the selected interface |
| `/` | Filter the focused list (`Esc` clears) |
| `r` | Refresh: re-discover sockets, re-introspect the service |
| `y` | Copy the equivalent `vlx call` command (OSC 52) |
| `?` | Full help |
| `q`, `Ctrl+C` | Quit (`Esc` goes back one level first) |

### 4.4 Calling a method

`Enter` or `c` on a method opens the **call screen**:

1. A **parameter form**: one row per top-level parameter showing its name,
   IDL type and an input that fits the type —
   text for `string`, a number field for `int`/`float`, a `‹ choice ›` for
   `bool` and enums (←/→, space, or type the first letters), and a one-line
   JSON field for structs, arrays, maps, `object` and `any` (pre-filled with a
   skeleton). The focused row shows the parameter's doc comment and how to
   edit it. Optional (`?T`) parameters are omitted until given a value; for
   choices the first option is `(unset)`. ↑/↓ or Enter move between rows.
2. **Live validation** against the IDL on every keystroke. Problems appear
   under the field they belong to, including nested ones
   (`where`: `.x: expected int`). Sending is disabled while the parameters are
   invalid.
3. **Ctrl+J** switches to a raw JSON editor holding the same parameters, and
   back. JSON the form cannot represent (unknown keys, an empty optional
   string) keeps you in JSON mode with an explanation. Methods without
   parameters say so and can be sent straight away.
4. Flags shown as toggles: `more` (enabled and forced on for methods that
   require it, unavailable for methods that do not support it) and `oneway`.
5. `Ctrl+S` sends. Non-read methods go through the confirmation flow in
   [§6.2](#62-policy).
6. The **response pane** shows each reply as pretty, colourised JSON with a
   counter, elapsed time and, for streams, a live tail. Errors show the Varlink
   error name, its parameters, and the IDL doc for that error if the interface
   declares it. `Esc` stops a stream (which closes the connection, as the
   protocol requires).
7. `y` copies the response; `Ctrl+Y` copies the equivalent `vlx call` and
   `varlinkctl call` commands.

### 4.5 Search palette

`Ctrl+K` or `:` opens a palette that searches every service, interface,
method, type and error at once. Opening it introspects every accessible
service in the background (with preview enabled), and results update as they
arrive. Matching is token-based: every word must match a name, the qualified
name or the service's short name — best at the start of a word or a CamelCase
hump, otherwise as a subsequence — so `resolve host` finds
`io.systemd.Resolve.ResolveHostname`. Methods rank above services, interfaces,
types and errors; an exact name wins. With an empty query the palette lists
services, so it doubles as a service switcher. Enter jumps there: it selects
the service, expands the interface and puts the cursor on the entry.

### 4.6 Visual language

| Token | Use |
| --- | --- |
| Violet `#A78BFA` / `#7C3AED` | Brand, focus, selection |
| Green `#30D475` | `READ`, success, connected |
| Amber `#F5B83D` | `WRITE`, warnings, writes-enabled badge |
| Red `#F2545B` | `DANGER`, errors, root badge |
| Cyan `#22D3EE` | Types and keys in JSON |

Colours are `lipgloss.AdaptiveColor` pairs for light and dark terminals.
`NO_COLOR` or `--no-color` turns all styling off; every state also has a text
or glyph form.

## 5. Command-line interface

One Cobra command per file under `internal/cli/`.

| Command | Purpose |
| --- | --- |
| `vlx` / `vlx explore [SERVICE]` | Open the TUI. |
| `vlx list` (`ls`) | List discovered sockets; `--probe` asks each for `GetInfo`. |
| `vlx info SERVICE` | Vendor, product, version, URL and interfaces. |
| `vlx describe SERVICE [INTERFACE]` (`introspect`) | Print IDL, highlighted on a TTY. |
| `vlx template SERVICE METHOD` | Print a JSON parameter skeleton for a method. |
| `vlx call SERVICE METHOD [PARAMS]` | Call a method. |
| `vlx demo` | Serve the demo interface and explore it (`--serve` to only serve). The socket lives in `$XDG_RUNTIME_DIR/vlx/` so other commands find it as `demo`. |
| `vlx config path\|show\|init` | Inspect or create the config file. |
| `vlx version` | Version, commit, build date, Go version. |
| `vlx completion SHELL` | Shell completion (Cobra built-in). |

`vlx call` details:

- `PARAMS` is a JSON object, `-` for stdin, or `-f FILE`. Omitted means `{}`.
- `--more` streams (newline-delimited JSON on stdout, one reply per line),
  and is turned on automatically for methods that require it;
  `--oneway` sends without waiting; `--max-replies N` caps streams (and
  implies `--more` where supported).
- `--no-validate` skips introspection for services that cannot describe
  themselves; the safety policy still applies.
- With `-o json`, a Varlink error reply is also printed to stdout as
  `{"error": NAME, "parameters": {...}}`.
- `--dry-run` validates the parameters and prints the message that would be
  sent, without connecting.
- `--allow-writes` / `--yes`: see [§6.2](#62-policy).
- Method names may be qualified (`io.systemd.Hostname.Describe`) or, when the
  service implements exactly one interface with that method, bare (`Describe`).

Global flags: `--config`, `--timeout` (default `10s`), `--allow-writes`,
`-o/--output text|json`, `--no-color`, `--max-frame-size`.

**Exit codes:** `0` success; `1` generic failure; `2` usage error; `3` the
service returned a Varlink error; `4` refused by the safety policy; `5` service
not found or not reachable; `130` interrupted.

**Output:** human output goes to stdout, diagnostics to stderr. `-o json`
output is stable and documented. Colour only when stdout is a TTY.

## 6. Safety model

vlx talks directly to privileged system services, often as root. A slip of a
key must not reboot a machine, wipe a disk or extend a TPM PCR.

### 6.1 Classification

Every method is classified as one of:

| Class | Meaning | Examples |
| --- | --- | --- |
| `READ` | No side effects expected. | `Get*`, `List*`, `Describe`, `Resolve*`, `Subscribe*`, `Dump*`, `Can*`, `Ping` |
| `WRITE` | Changes state. Also the default for anything unknown. | `SetLogLevel`, `Unit.SetProperties`, `Credentials.Decrypt` |
| `DANGER` | Hard to undo, disruptive or security-sensitive. | `Manager.Reexecute`, `Machine.Kill`, `PCRExtend.Extend`, `Repart.Run`, `MachineImage.Remove` |

Classification is decided in this order, first match wins:

1. User overrides in config (`safety.read`, `safety.write`, `safety.danger`:
   lists of glob patterns over qualified method names).
2. A built-in table covering every `io.systemd.*` method in systemd v257–v260
   and `org.varlink.service`.
3. Name heuristics: a danger verb (`Kill`, `Terminate`, `Remove`, `Delete`,
   `Reset`, `Reboot`, `PowerOff`, `Wipe`, `Format`, …) → `DANGER`; a read verb
   (`Get`, `List`, `Describe`, `Resolve`, `Query`, `Lookup`, `Dump`, `Watch`,
   `Subscribe`, `Monitor`, `Can`, `Is`, `Has`, `Ping`, …) → `READ`.
4. Otherwise `WRITE` — unknown is never assumed safe.

`oneway` calls are at least `WRITE`, since nothing can be observed of them.

### 6.2 Policy

- **Default mode is read-only.** `WRITE` and `DANGER` methods can be browsed
  and their parameters prepared and validated, but sending them is refused
  with a message that explains how to enable writes.
- **`--allow-writes`** (or `allow_writes: true` in config, or
  `VLX_ALLOW_WRITES=1`) enables writes for that session. The header badge
  turns amber.
- With writes enabled, a `WRITE` call shows a confirmation dialog with the
  method, socket and parameters; `DANGER` additionally requires typing the
  method name.
- `vlx call` non-interactive: `WRITE` needs `--allow-writes`; it then prompts
  on a TTY and requires `--yes` otherwise. `DANGER` needs `--allow-writes`
  and either a typed confirmation on a TTY or `--yes`. Refusals exit with
  code `4`.

### 6.3 Resource limits

| Limit | Default | Why |
| --- | --- | --- |
| Call timeout (`--timeout`) | 10 s | A hung service must not hang vlx. Streams are exempt but cancellable. |
| Introspection timeout | 5 s per socket | Discovery stays responsive with stuck services. |
| Max frame size | 16 MiB | Bounded memory per message (sdvarlink `WithMaxFrameSize`). |
| Max stream replies (TUI) | 1000 kept | The response pane drops the oldest and says so. |
| Received file descriptors | closed immediately | Never leak descriptors; their count is reported. |

### 6.4 Terminal and data hygiene

- **Escape-sequence injection:** every string that comes from a service — IDL
  doc comments, interface names, JSON values, error names — is stripped of
  control characters before it reaches the terminal, so a malicious or buggy
  service cannot rewrite the screen, set the window title or abuse OSC 52.
- **Redaction:** fields whose names match `redact` patterns (default:
  `*password*`, `*secret*`, `*privateKey*`, `hashedPassword`, `privileged`,
  `*token*`) are shown as `«redacted»` in the TUI. `R` reveals them for the
  current response. `vlx call` output is not redacted (it is for scripts), but
  vlx never writes responses anywhere other than stdout.
- **No persistence:** vlx keeps no history and writes no files except
  `vlx config init`.

### 6.5 Attack surface

- Unix sockets only. No `exec:`/`ssh:` transports, no shelling out, no
  plugins.
- Discovery is read-only: it lists a fixed set of directories one level deep
  (plus configured `search_paths`), does not follow symlinked directories and
  only reports sockets. `vlx list` never connects unless `--probe` is given.
  In the explorer, preview and the palette introspect services with the
  read-only `org.varlink.service` methods only; `preview: false` turns that
  off so nothing is sent until a service is opened.
- The config file can enable writes, so vlx refuses to load a config file that
  is group- or world-writable or not owned by the current user or root.
- vlx never needs and never requests elevated privileges. Running as root is
  allowed but flagged in the header and in `vlx call` diagnostics.

## 7. Discovery

Default search locations (each listed one level deep, sockets only):

```text
/run/varlink/registry            # systemd ≥ 258 registry of public services
/run/systemd                     # io.systemd.* and friends
/run/systemd/{resolve,userdb,machine,netif,journal,oom,report}
/run/udev
$XDG_RUNTIME_DIR/systemd         # per-user services
$XDG_RUNTIME_DIR/vlx             # the vlx demo service
```

plus `search_paths` from config and `--search` flags. Results are de-duplicated
by resolved path and sorted by name. A socket is reported with its path, base
name and permissions (so `EACCES` is predictable before connecting).

## 8. Configuration

Viper, layered (highest wins): flags → environment (`VLX_*`, `-` → `_`) →
config file → defaults. The file is
`$XDG_CONFIG_HOME/vlx/config.yaml` (`~/.config/vlx/config.yaml`), overridable
with `--config`.

```yaml
# ~/.config/vlx/config.yaml
allow_writes: false
timeout: 10s
max_frame_size: 16777216
max_replies: 1000
preview: true
search_paths: []
redact: ["*password*", "*secret*", "*privateKey*", "hashedPassword", "privileged", "*token*"]
safety:
  read: []      # globs, e.g. "com.example.Thing.Peek*"
  write: []
  danger: []
```

## 9. Architecture

```text
cmd/vlx/main.go            entry point; exit-code mapping
internal/cli/              one Cobra command per file
internal/config/           Viper setup, defaults, file permission checks
internal/discovery/        socket discovery and SERVICE resolution
internal/client/           sdvarlink wrapper: introspect, call, stream
internal/schema/           IDL → JSON skeleton; JSON → IDL validation
internal/safety/           classification and policy
internal/sanitize/         control-character stripping
internal/render/           JSON and IDL highlighting
internal/tui/              Bubble Tea model, panes, styles, keys
internal/demo/             built-in demo Varlink service
internal/version/          build metadata
```

Dependencies flow one way: `cli → tui → client/schema/safety/render →
sdvarlink`. Only `client` and `demo` touch sockets. All network work in the
TUI runs in `tea.Cmd`s with contexts, so the UI never blocks.

## 10. Quality

- Unit tests for discovery, resolution, schema skeletons and validation,
  safety classification (including a test that every built-in systemd method
  has an explicit class), sanitisation and rendering.
- End-to-end tests run the CLI against the demo service over a real socket —
  no systemd needed, so they run on macOS too.
- TUI tests drive the Bubble Tea model with messages and assert on state and
  rendered views.
- CI: tests with `-race` on Linux amd64/arm64 and macOS, golangci-lint,
  GoReleaser snapshot on every push.

## 11. Distribution

GoReleaser builds static (`CGO_ENABLED=0`) binaries for linux
amd64/arm64/armv7/riscv64/ppc64le/s390x and darwin amd64/arm64, with
`tar.gz` archives that include shell completions and man pages, `.deb`,
`.rpm` and `.apk` packages, and SHA-256 checksums. Tags `v*` publish a GitHub
release; every branch push builds a snapshot artifact. Docs are MkDocs
Material, published to GitHub Pages.

## 12. Future work

- `ssh:` transport behind an explicit flag, mirroring `varlinkctl --host`.
- Sending file descriptors from files the user picks.
- A request "scratchpad" with saved (opt-in, redacted) calls.
- Bubble Tea v2 migration.
- Recording sessions as `varlinktest` golden files for sdvarlink tests.
