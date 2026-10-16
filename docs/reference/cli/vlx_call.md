---
description: Reference for the 'vlx call' command.
---

## vlx call

Call a method and print the reply

### Synopsis

Call METHOD on SERVICE. Give parameters as NAME=VALUE pairs (the explorer's
command-line syntax: strings are taken literally, other values are JSON) or as
one JSON object ('-' reads stdin, -f reads a file, and nothing means {}).
Parameters are checked against the method's IDL before anything is sent.

Safety: methods are classified READ, WRITE or DANGER. vlx only sends READ
methods unless you pass --allow-writes. WRITE calls then ask for confirmation
and DANGER calls ask you to type the method name; in scripts, --yes confirms.

Output: each reply is printed as JSON, pretty on a terminal and one compact
object per line otherwise (or with -o json). Streams (--more) print every
reply as it arrives. Exit status is 3 when the service returns an error,
4 when vlx refuses a call, 5 when the service cannot be found.

```
vlx call SERVICE METHOD [PARAMS | NAME=VALUE...] [flags]
```

### Examples

```
  vlx call hostname io.systemd.Hostname.Describe
  vlx call resolve ResolveHostname name=example.com family=2
  vlx call resolve ResolveHostname '{"name":"example.com"}'
  echo '{"name":"example.com"}' | vlx call resolve ResolveHostname -
  vlx call multiplexer GetUserRecord '{"service":"io.systemd.Multiplexer"}' --more
  vlx call demo SetDefaultLanguage '{"language":"german"}' --allow-writes --yes
```

### Options

```
      --dry-run           validate and print the message without sending it
  -f, --file string       read PARAMS from a file ('-' for stdin)
  -h, --help              help for call
      --max-replies int   stop a stream after N replies (0: no limit; implies --more when supported)
      --more              ask for a stream of replies (required by some methods)
      --no-validate       skip introspection and parameter checks (needs a qualified METHOD; safety still applies)
      --oneway            send without waiting for a reply
  -y, --yes               confirm WRITE and DANGER calls without prompting
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

