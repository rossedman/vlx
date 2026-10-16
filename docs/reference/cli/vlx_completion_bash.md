---
description: Reference for the 'vlx completion bash' command.
---

## vlx completion bash

Generate the autocompletion script for bash

### Synopsis

Generate the autocompletion script for the bash shell.

This script depends on the 'bash-completion' package.
If it is not installed already, you can install it via your OS's package manager.

To load completions in your current shell session:

	source <(vlx completion bash)

To load completions for every new session, execute once:

#### Linux:

	vlx completion bash > /etc/bash_completion.d/vlx

#### macOS:

	vlx completion bash > $(brew --prefix)/etc/bash_completion.d/vlx

You will need to start a new shell for this setup to take effect.


```
vlx completion bash
```

### Options

```
  -h, --help              help for bash
      --no-descriptions   disable completion descriptions
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

* [vlx completion](vlx_completion.md)	 - Generate the autocompletion script for the specified shell

