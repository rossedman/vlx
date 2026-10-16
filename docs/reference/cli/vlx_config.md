---
description: Reference for the 'vlx config' command.
---

## vlx config

Inspect or create the configuration file

### Synopsis

vlx reads settings from flags, then VLX_* environment variables, then the
config file, then built-in defaults. These commands show where the file is,
what the effective settings are, and write a commented starter file.

### Options

```
  -h, --help   help for config
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
* [vlx config init](vlx_config_init.md)	 - Write a commented starter config file
* [vlx config path](vlx_config_path.md)	 - Print the config file path
* [vlx config show](vlx_config_show.md)	 - Print the effective configuration

