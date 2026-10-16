---
description: Reference for the 'vlx demo' command.
---

## vlx demo

Explore a built-in sample service (no systemd needed)

### Synopsis

Start vlx's demo Varlink service and open the explorer on it. The service
listens on $XDG_RUNTIME_DIR/vlx/io.vlx.demo, so other vlx commands find it as
"demo", or on a private temporary socket when that is not possible. The demo has two small interfaces, io.vlx.demo.Greeter and
io.vlx.demo.Inventory, with streaming methods, typed errors, a redacted secret
and methods of every safety class. It keeps all state in memory.

Add --allow-writes to try the confirmation dialogs. Use --serve to only run
the service, for example to try 'vlx call demo ...' from another terminal.

```
vlx demo [flags]
```

### Examples

```
  vlx demo
  vlx demo --allow-writes
  vlx demo --serve
```

### Options

```
  -h, --help            help for demo
      --serve           only run the service and print its socket path
      --socket string   listen on this path instead of a temporary one
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

