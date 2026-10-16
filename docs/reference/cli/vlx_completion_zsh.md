---
description: Reference for the 'vlx completion zsh' command.
---

## vlx completion zsh

Generate the autocompletion script for zsh

### Synopsis

Generate the autocompletion script for the zsh shell.

If shell completion is not already enabled in your environment you will need
to enable it.  You can execute the following once:

	echo "autoload -U compinit; compinit" >> ~/.zshrc

To load completions in your current shell session:

	source <(vlx completion zsh)

To load completions for every new session, execute once:

#### Linux:

	vlx completion zsh > "${fpath[1]}/_vlx"

#### macOS:

	vlx completion zsh > $(brew --prefix)/share/zsh/site-functions/_vlx

You will need to start a new shell for this setup to take effect.


```
vlx completion zsh [flags]
```

### Options

```
  -h, --help              help for zsh
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

