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
`
