---
description: Configure vlx with flags, VLX_* environment variables or a YAML config file.
---

# Configuration

vlx works with no configuration at all. When you want to change something,
settings come from four places; the first that sets a value wins:

1. **Flags** — `--timeout 3s`
2. **Environment** — `VLX_TIMEOUT=3s`
3. **Config file** — `timeout: 3s`
4. **Defaults**

## The config file

The file lives at `$XDG_CONFIG_HOME/vlx/config.yaml`, or
`~/.config/vlx/config.yaml` when `XDG_CONFIG_HOME` is unset — on macOS too;
`--config FILE` uses another (`vlx config init --config FILE` creates it). Create a
commented starter file with:

```sh
vlx config init
```

See what vlx is actually using, and from where:

```sh
vlx config show
vlx config path
```

!!! warning "Permissions"
    Because the config can enable writes, vlx refuses to load a config file
    that is writable by group or others, or owned by anyone other than you or
    root. `vlx config init` creates it with mode `0600`.

## Settings

| Key | Flag | Environment | Default | Meaning |
| --- | --- | --- | --- | --- |
| `allow_writes` | `--allow-writes` | `VLX_ALLOW_WRITES` | `false` | Allow WRITE and DANGER methods, after confirmation. |
| `timeout` | `--timeout` | `VLX_TIMEOUT` | `10s` | Connect and single-reply timeout. |
| `max_frame_size` | `--max-frame-size` | `VLX_MAX_FRAME_SIZE` | `16777216` | Largest message accepted, in bytes. |
| `max_replies` | | `VLX_MAX_REPLIES` | `1000` | Stream replies kept in the TUI. |
| `preview` | `--no-preview` | `VLX_PREVIEW` | `true` | Introspect services as the cursor moves, and for search. |
| `search_paths` | `--search` | | `[]` | Extra directories to search for sockets. |
| `redact` | | | see below | Keys whose values are hidden in the TUI. |
| `safety` | | | empty | Classification overrides. |
| `output` | `-o`, `--output` | `VLX_OUTPUT` | `text` | `text` or `json`. |
| `no_color` | `--no-color` | `VLX_NO_COLOR`, `NO_COLOR` | `false` | Disable colour. |

## Example

```yaml title="~/.config/vlx/config.yaml"
timeout: 5s

# My own services live here.
search_paths:
  - /run/myapp

redact:
  - "*password*"
  - "*secret*"
  - "*privatekey*"
  - "hashedpassword"
  - "privileged"
  - "*token*"
  - "*credential*"

safety:
  read:
    - "com.example.Thing.Peek*"
  danger:
    - "com.example.*.Purge*"
    - "io.systemd.Unit.SetProperties"
```

## Safety overrides

`safety.read`, `safety.write` and `safety.danger` are lists of glob
patterns matched against qualified method names, such as
`io.systemd.Unit.SetProperties` or `com.example.*`. When several match, the
most severe class wins, and overrides beat vlx's built-in rules. Use them to
mark your own services' read methods as READ, or to make a method you are
nervous about DANGER.
