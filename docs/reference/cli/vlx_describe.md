---
description: Reference for the 'vlx describe' command.
---

## vlx describe

Print the IDL of a service's interfaces

### Synopsis

Print the Varlink IDL of every interface a service implements, or of one
interface, as the service describes it. Output is highlighted on a terminal.

```
vlx describe SERVICE [INTERFACE] [flags]
```

### Examples

```
  vlx describe hostname
  vlx describe resolve io.systemd.Resolve
  vlx describe hostname -o json
```

### Options

```
  -h, --help   help for describe
```

### Options inherited from parent commands

```
      --allow-writes         allow methods that change state (each still needs confirmation)
      --config string        config file (default $XDG_CONFIG_HOME/vlx/config.yaml)
      --max-frame-size int   largest message accepted from a service, in bytes (default 16777216)
      --no-color             disable colour (also NO_COLOR=1)
  -o, --output string        output format: text or json (default "text")
      --search strings       extra directory to search for sockets (repeatable)
      --timeout duration     timeout for connecting and for single replies (default 10s)
```

### SEE ALSO

* [vlx](vlx.md)	 - Explore and call Varlink services

