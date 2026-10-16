---
description: Use vlx in scripts and CI — JSON output, NDJSON streams, exit codes and non-interactive confirmations.
---

# Scripting

vlx is built to be scripted as well as explored. The rules:

- **stdout** carries data, **stderr** carries diagnostics.
- When stdout is not a terminal, or with `-o json`, every reply is one
  compact JSON object per line (NDJSON), ready for `jq`.
- Exit codes are stable: see [Exit codes](../reference/exit-codes.md).
- Colour is only used on terminals, and never with `NO_COLOR` or `--no-color`.

## Examples

```sh
# The machine's hostname
vlx call hostname Describe -o json | jq -r .Hostname

# Every user name known to systemd-userdbd
vlx call multiplexer GetUserRecord '{"service":"io.systemd.Multiplexer"}' --more \
  | jq -r '.record.userName'

# All units, as a table
vlx call manager io.systemd.Unit.List --more | jq -r '.context.ID'

# Which sockets answer, and what are they?
vlx list --probe -o json | jq -r '.[] | select(.product) | "\(.name)\t\(.product)"'

# Fail a CI step if systemd-resolved is unhappy
if ! vlx call resolve ResolveHostname '{"name":"example.com"}' >/dev/null; then
  echo "DNS is broken" >&2; exit 1
fi
```

## Errors as JSON

With `-o json`, a Varlink error reply is also written to stdout as one line,
so a pipeline sees it:

```console
$ vlx call demo GetItem '{"id":42}' -o json; echo "exit $?"
{"error":"io.vlx.demo.Inventory.NoSuchItem","parameters":{"id":42}}
exit 3
```

## Writes in scripts

Scripts cannot answer confirmation prompts, so WRITE and DANGER calls need
both `--allow-writes` and `--yes`:

```sh
vlx call --allow-writes --yes resolve io.systemd.service.SetLogLevel '{"level":7}'
```

Without `--yes`, vlx refuses (exit `4`) rather than guessing. Think twice
before putting `--yes` on a DANGER method in automation.

## Environment variables

Every setting can come from the environment, which suits CI:

```sh
export VLX_TIMEOUT=3s
export VLX_OUTPUT=json
```

See [Configuration](configuration.md).
