---
description: Reference for the 'vlx list' command.
---

## vlx list

List the Varlink sockets on this host

### Synopsis

List the Varlink sockets found in the standard locations (/run/varlink/registry,
/run/systemd and its subdirectories, /run/udev and $XDG_RUNTIME_DIR/systemd),
plus any --search directories and search_paths from the config.

Listing only reads directories. With --probe, vlx also connects to each socket
and asks for its GetInfo, which is read-only.

```
vlx list [flags]
```

### Examples

```
  vlx list
  vlx list --probe
  vlx list -o json | jq -r '.[].path'
```

### Options

```
  -h, --help    help for list
      --probe   connect to each socket and ask for its info
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

