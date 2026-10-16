---
description: Reference for the 'vlx version' command.
---

## vlx version

Print version and build information

```
vlx version [flags]
```

### Options

```
  -h, --help   help for version
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

