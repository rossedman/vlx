---
description: Reference for the 'vlx mcp' command.
---

## vlx mcp

Serve vlx to an AI agent over the Model Context Protocol

### Synopsis

Run an MCP server on stdin and stdout, so an AI agent such as Claude Code can
discover, inspect and call the Varlink services on this host.

The server offers four tools: list_services, describe, method_schema and call.
Each interface's IDL is also a resource, varlink://<service>/<interface>.
Everything a service returns is sanitised, redacted and labelled as data before
the agent sees it.

Only READ methods are sent. WRITE and DANGER methods are refused, with or
without --allow-writes, because this version cannot ask you to confirm them.
Streams stop after mcp.max_stream_replies replies or mcp.max_stream_seconds
seconds, and one result is at most mcp.max_result_bytes; see 'vlx config show'.

Register it with your agent, for example:

  claude mcp add vlx -- vlx mcp

```
vlx mcp [flags]
```

### Examples

```
  vlx mcp
  claude mcp add vlx -- vlx mcp
  claude mcp add vlx-demo -- vlx mcp --search "$XDG_RUNTIME_DIR/vlx"
```

### Options

```
  -h, --help   help for mcp
```

### Options inherited from parent commands

```
      --allow-writes         allow methods that change state (each still needs confirmation)
      --config string        config file (default $XDG_CONFIG_HOME/vlx/config.yaml)
      --max-frame-size int   largest message accepted from a service, in bytes (default 16777216)
      --no-color             disable colour (also NO_COLOR=1)
      --no-preview           in the explorer, connect to a service only when you open it
  -o, --output string        output format: text or json (default "text")
      --search strings       extra directory to search for sockets (repeatable)
      --timeout duration     timeout for connecting and for single replies (default 10s)
```

### SEE ALSO

* [vlx](vlx.md)	 - Explore and call Varlink services

