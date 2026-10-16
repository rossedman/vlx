---
description: Call Varlink methods with vlx call — parameters, method names, streams, oneway calls, dry runs and errors.
---

# Calling methods

`vlx call` calls one method and prints the replies.

```sh
vlx call SERVICE METHOD [PARAMS | NAME=VALUE ...]
```

```console
$ vlx call hostname io.systemd.Hostname.Describe
{
  "Hostname": "atlas",
  "StaticHostname": "atlas",
  ...
}
```

## Naming the service

SERVICE is a socket path, a `unix:` address, or the name — or a unique part
of the name — of a socket that [discovery](../reference/discovery.md) finds:

```sh
vlx call /run/systemd/io.systemd.Hostname ...
vlx call io.systemd.Hostname ...
vlx call hostname ...
```

An ambiguous name is an error that lists the candidates.

## Naming the method

Use the qualified name (`io.systemd.Hostname.Describe`), or just the method
(`Describe`) when only one interface on that socket has it.

## Parameters

Give parameters as `NAME=VALUE` pairs — the same form the explorer's command
line uses — or as one JSON object. Leave them out for `{}`.

```sh
vlx call resolve ResolveHostname name=example.com family=2
vlx call demo GetGreeting name='Ada Lovelace' language=german
vlx call resolve ResolveHostname '{"name": "example.com"}'
echo '{"name": "example.com"}' | vlx call resolve ResolveHostname -
vlx call resolve ResolveHostname -f params.json
```

With `NAME=VALUE`, vlx converts each value using the method's IDL: strings
and enum values are taken as typed (`name=123` stays a string for a string
parameter), and everything else — numbers, booleans, arrays, structs — is
JSON (`ids=[1,2]`, `where='{"x":1}'`). An unknown or repeated name is a usage
error that lists the method's parameters. `NAME=VALUE` cannot be combined
with `-f` or `--no-validate`.

Not sure what a method takes? Ask for a template:

```console
$ vlx template resolve ResolveHostname
{
  "name": ""
}
# optional parameters:
#   ifindex: int
#   family: int
#   flags: int
```

vlx checks parameters against the method's IDL **before** connecting to
send anything, and explains exactly what is wrong:

```console
$ vlx call demo GetItem '{"id": "4"}'
vlx: parameters for io.vlx.demo.Inventory.GetItem: .id: expected int, got string
```

Unknown fields get a *did you mean* suggestion when only the case differs.
Pass `--no-validate` to skip introspection for services that do not describe
themselves (the safety checks still apply).

## Streams

Methods marked `[Supports 'more' flag]` can return many replies, and
`[Requires 'more' flag]` methods always do. Add `--more`:

```sh
vlx call multiplexer io.systemd.UserDatabase.GetUserRecord \
  '{"service": "io.systemd.Multiplexer"}' --more
```

vlx turns `--more` on by itself for methods that require it, and
`--max-replies N` stops after N replies (and implies `--more`). Each reply is
printed as it arrives; ++ctrl+c++ ends the stream cleanly.

## Oneway

`--oneway` sends the call without waiting for, or receiving, a reply. Because
nothing about it can be observed, vlx treats every oneway call as at least a
<span class="vlx-write">WRITE</span>.

## Dry runs

`--dry-run` validates the parameters and prints the exact message vlx would
send, plus its safety class, without connecting:

```console
$ vlx call demo RemoveItem '{"id": 1}' --dry-run
dry run: io.vlx.demo.Inventory.RemoveItem is DANGER (name starts with "Remove"); nothing was sent to /run/user/1000/vlx/io.vlx.demo
{
  "method": "io.vlx.demo.Inventory.RemoveItem",
  "parameters": {
    "id": 1
  }
}
```

## Errors

When the service answers with an error, vlx prints it and exits with code
`3`:

```console
$ vlx call demo GetItem '{"id": 42}'
vlx: io.vlx.demo.Inventory.GetItem returned io.vlx.demo.Inventory.NoSuchItem {"id":42}
```

Common failures come with a hint — permission denied suggests `sudo`, a
missing method suggests the systemd version is too old, and so on. See
[Exit codes](../reference/exit-codes.md) for the full list.

## File descriptors

Some systemd methods return file descriptors. vlx closes every descriptor it
receives immediately and tells you how many arrived; it never sends any.
