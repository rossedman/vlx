package config

// Template is the commented file written by 'vlx config init'.
const Template = `# vlx configuration. Every key can also be set with a VLX_* environment
# variable (VLX_ALLOW_WRITES=1) or a flag (--allow-writes).
# Docs: https://rossedman.github.io/vlx/reference/configuration/

# Allow methods that change state. Even when true, vlx asks before every
# WRITE call and makes you type the name of every DANGER method.
allow_writes: false

# How long to wait for a connection or a single reply. Streams run until
# they end or you stop them.
timeout: 10s

# Largest message accepted from a service, in bytes.
max_frame_size: 16777216

# Introspect a service in the explorer as soon as the cursor rests on it,
# so its interfaces show without pressing Enter. Introspection only uses the
# read-only org.varlink.service methods. Set to false to connect only when
# you open a service (or pass --no-preview).
preview: true

# Most stream replies kept in the TUI's response pane.
max_replies: 1000

# Extra directories to search for sockets, one level deep.
search_paths: []

# Keys whose values are hidden in the TUI (case-insensitive globs).
# Press R on a response to reveal them.
redact:
  - "*password*"
  - "*secret*"
  - "*privatekey*"
  - "hashedpassword"
  - "privileged"
  - "*token*"

# Override how methods are classified, by qualified name (globs).
# The most severe matching class wins.
safety:
  read: []      # e.g. "com.example.Thing.Peek*"
  write: []
  danger: []    # e.g. "io.systemd.Unit.SetProperties"

# Limits for 'vlx mcp', the MCP server for AI agents. A stream stops at
# whichever cap it reaches first; an agent may ask for less, never more than
# 200 replies or 60 seconds. A tool result larger than max_result_bytes is
# cut, and the result says so.
mcp:
  max_stream_replies: 20
  max_stream_seconds: 10
  max_result_bytes: 262144
`
