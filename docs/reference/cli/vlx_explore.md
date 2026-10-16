---
description: Reference for the 'vlx explore' command.
---

## vlx explore

Open the interactive explorer (the default command)

### Synopsis

Open the interactive explorer. With SERVICE, vlx connects to it straight
away; without, it lists the Varlink sockets it finds and lets you pick one.

Press ? inside the explorer for every key binding.

```
vlx explore [SERVICE] [flags]
```

### Examples

```
  vlx explore
  vlx explore /run/systemd/io.systemd.Hostname
  vlx explore resolve --allow-writes
```

### Options

```
  -h, --help   help for explore
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

