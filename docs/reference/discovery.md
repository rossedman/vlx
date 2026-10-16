---
description: Where vlx looks for Varlink sockets, and how SERVICE arguments resolve to a socket.
---

# Discovery

## Where vlx looks

vlx lists these directories, **one level deep**, and keeps only Unix
sockets:

| Directory | What lives there |
| --- | --- |
| `/run/varlink/registry` | systemd v258+'s registry of public Varlink services |
| `/run/systemd` | most `io.systemd.*` services, PID 1's `io.systemd.Manager` |
| `/run/systemd/resolve` | `io.systemd.Resolve`, `io.systemd.Resolve.Monitor` |
| `/run/systemd/userdb` | `io.systemd.Multiplexer` and user database providers |
| `/run/systemd/machine` | `io.systemd.Machine`, `io.systemd.MachineImage` |
| `/run/systemd/netif` | `io.systemd.Network` |
| `/run/systemd/journal` | `io.systemd.journal` |
| `/run/systemd/oom` | `io.systemd.ManagedOOM` |
| `/run/systemd/report` | `io.systemd.Metrics` providers |
| `/run/udev` | `io.systemd.Udev` |
| `$XDG_RUNTIME_DIR/systemd` | your user manager's services |
| `$XDG_RUNTIME_DIR/vlx` | the `vlx demo` service |

Add more with `--search DIR` or `search_paths` in the
[config](../guides/configuration.md). Sockets reached through several paths
(registry symlinks) are listed once.

Discovery only reads directories. It never connects to a socket, and it
does not follow symlinked directories. `vlx list --probe` additionally sends
each socket a read-only `GetInfo`.

## How SERVICE resolves

Wherever a command takes SERVICE, vlx tries, in order:

1. **A path** — anything containing `/`, such as
   `/run/systemd/io.systemd.Hostname` or `./my.sock`.
2. **A Varlink address** — `unix:/run/systemd/io.systemd.Hostname`
   (options after `;` are ignored).
3. **An exact socket name** — `io.systemd.Hostname`.
4. **A short name, any case** — `hostname`, `Resolve.Monitor`.
5. **A unique fragment, any case** — `multiplex` matches
   `io.systemd.Multiplexer`.

If a fragment matches more than one socket, vlx lists them and asks you to
be more specific (exit code `5`).
