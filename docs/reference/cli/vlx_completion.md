---
description: Reference for the 'vlx completion' command.
---

## vlx completion

Generate the autocompletion script for the specified shell

### Synopsis

Generate the autocompletion script for vlx for the specified shell.
See each sub-command's help for details on how to use the generated script.


### Options

```
  -h, --help   help for completion
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
* [vlx completion bash](vlx_completion_bash.md)	 - Generate the autocompletion script for bash
* [vlx completion fish](vlx_completion_fish.md)	 - Generate the autocompletion script for fish
* [vlx completion powershell](vlx_completion_powershell.md)	 - Generate the autocompletion script for powershell
* [vlx completion zsh](vlx_completion_zsh.md)	 - Generate the autocompletion script for zsh

