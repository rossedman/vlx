---
description: Reference for the 'vlx completion fish' command.
---

## vlx completion fish

Generate the autocompletion script for fish

### Synopsis

Generate the autocompletion script for the fish shell.

To load completions in your current shell session:

	vlx completion fish | source

To load completions for every new session, execute once:

	vlx completion fish > ~/.config/fish/completions/vlx.fish

You will need to start a new shell for this setup to take effect.


```
vlx completion fish [flags]
```

### Options

```
  -h, --help              help for fish
      --no-descriptions   disable completion descriptions
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

* [vlx completion](vlx_completion.md)	 - Generate the autocompletion script for the specified shell

