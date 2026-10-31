---
description: Let an AI agent such as Claude Code explore and call the Varlink services on a host through vlx's MCP server.
---

# AI agents (MCP)

`vlx mcp` runs a [Model Context Protocol](https://modelcontextprotocol.io)
server on stdin and stdout. An agent that speaks MCP can then find the
Varlink services on the host, read their interfaces and call their READ
methods, with the same discovery, safety classes and redaction as the rest
of vlx.

## Set it up

For Claude Code:

```sh
claude mcp add vlx -- vlx mcp
```

Any other MCP client works the same way: run `vlx mcp` as a stdio server.
Global flags apply, so `vlx mcp --search DIR` adds a directory to search
and `vlx mcp --timeout 5s` shortens the per-call timeout.

To try it without systemd, run the demo service in one terminal and point
the server at it:

```sh
vlx demo --serve
claude mcp add vlx-demo -- vlx mcp
```

## What the agent gets

Four tools, the same for every service:

| Tool | What it does |
| --- | --- |
| `list_services` | The sockets on the host, with `probe` for vendor, product, version and interfaces. |
| `describe` | A service's interfaces: each method's doc, safety class, streaming mode, signature and whether it can be called. |
| `method_schema` | JSON Schema for a method's parameters and reply, an example parameter object and the optional fields. |
| `call` | Calls a method. Parameters are checked against the IDL first and every problem is reported at once. |

Each interface's IDL is also a resource at
`varlink://<service>/<interface>`, and `resources/list` lists one per
interface of every service the agent can reach.

Failed calls come back as tool errors the agent can read, with a `kind`
(`varlink`, `errno`, `refused`, `invalid_params`, `not_found`, `access`,
`limit` or `unavailable`), a message, a hint and whether retrying can help.

## What it will not do

- Only READ methods are sent. WRITE and DANGER methods are refused, with or
  without `--allow-writes`, because this version cannot ask you to confirm
  them. The refusal says so and says nothing was sent.
- Oneway calls are never offered, and methods that take file descriptors
  are refused.
- The safety classes come from vlx's own table and your `safety:` config.
  Tool annotations sent to the client are hints only; the server decides.

## Limits

A stream stops after `mcp.max_stream_replies` replies (default 20) or
`mcp.max_stream_seconds` seconds (default 10). The agent can ask for up to
200 replies and 60 seconds per call with the `stream` argument. One result
is at most `mcp.max_result_bytes` (default 256 KiB); a stream that would go
over stops early and says so. Set these in the config file:

```yaml
mcp:
  max_stream_replies: 20
  max_stream_seconds: 10
  max_result_bytes: 262144
```

## Untrusted content

Everything a service sends (interface names, doc comments, reply values) is
treated as untrusted. Terminal escapes and control characters are stripped,
values under secret-looking keys are replaced with `«redacted»` (see
`redact` in [Configuration](configuration.md)), and every result carries a
notice that the content is data from the service, not instructions.
