---
description: Reference for the 'vlx template' command.
---

## vlx template

Print a JSON parameter template for a method

### Synopsis

Print a JSON object with every required parameter of METHOD set to a typed
placeholder, ready to edit and pass to 'vlx call'. Optional parameters are
listed on stderr.

METHOD may be qualified (io.systemd.Hostname.Describe) or bare (Describe)
when only one interface of the service has it.

```
vlx template SERVICE METHOD [flags]
```

### Examples

```
  vlx template resolve ResolveHostname > params.json
  vlx call resolve ResolveHostname -f params.json
```

### Options

```
  -h, --help   help for template
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

