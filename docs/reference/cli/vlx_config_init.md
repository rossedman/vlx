---
description: Reference for the 'vlx config init' command.
---

## vlx config init

Write a commented starter config file

### Synopsis

Write a commented config file with the default settings to the config path
(or --config). The file is created with mode 0600: vlx refuses config files
that others can write, because the config can enable writes.

```
vlx config init [flags]
```

### Options

```
      --force   overwrite an existing file
  -h, --help    help for init
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

* [vlx config](vlx_config.md)	 - Inspect or create the configuration file

