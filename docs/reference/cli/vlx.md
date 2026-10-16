---
description: Reference for the 'vlx' command.
---

## vlx

Explore and call Varlink services

### Synopsis

vlx is a terminal explorer for Varlink services, such as systemd's
io.systemd.* APIs. Run it with no arguments to find the sockets on this host
and browse their interfaces, methods, types and errors.

vlx is read-only by default: methods that change state are refused unless you
pass --allow-writes, and even then vlx asks before sending them.

SERVICE is a socket path, a "unix:" address, or the name (or a unique part
of the name) of a discovered socket, e.g. "hostname".

```
vlx [SERVICE] [flags]
```

### Examples

```
  vlx                         # discover sockets and explore
  vlx hostname                # open io.systemd.Hostname directly
  vlx demo                    # try vlx with a built-in sample service
  vlx call hostname io.systemd.Hostname.Describe
```

### Options

```
      --allow-writes         allow methods that change state (each still needs confirmation)
      --config string        config file (default $XDG_CONFIG_HOME/vlx/config.yaml)
  -h, --help                 help for vlx
      --max-frame-size int   largest message accepted from a service, in bytes (default 16777216)
      --no-color             disable colour (also NO_COLOR=1)
  -o, --output string        output format: text or json (default "text")
      --search strings       extra directory to search for sockets (repeatable)
      --timeout duration     timeout for connecting and for single replies (default 10s)
```

### SEE ALSO

* [vlx call](vlx_call.md)	 - Call a method and print the reply
* [vlx completion](vlx_completion.md)	 - Generate the autocompletion script for the specified shell
* [vlx config](vlx_config.md)	 - Inspect or create the configuration file
* [vlx demo](vlx_demo.md)	 - Explore a built-in sample service (no systemd needed)
* [vlx describe](vlx_describe.md)	 - Print the IDL of a service's interfaces
* [vlx explore](vlx_explore.md)	 - Open the interactive explorer (the default command)
* [vlx info](vlx_info.md)	 - Show a service's vendor, product, version and interfaces
* [vlx list](vlx_list.md)	 - List the Varlink sockets on this host
* [vlx template](vlx_template.md)	 - Print a JSON parameter template for a method
* [vlx version](vlx_version.md)	 - Print version and build information

