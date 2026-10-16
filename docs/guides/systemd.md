---
description: Useful vlx calls against real systemd services — hostname, DNS, users, units, machines and more.
---

# systemd recipes

systemd v257 and newer serve most of their APIs over Varlink. These recipes
are all <span class="vlx-read">READ</span> calls, so they work in vlx's
default read-only mode. Some services only talk to root; prefix with `sudo`
if you see *permission denied*.

!!! tip
    Every recipe also works in the explorer: open the service, select the
    method, and paste the parameters.

## The host

```sh
vlx call hostname io.systemd.Hostname.Describe
```

## DNS (systemd-resolved)

```sh
# Resolve a name
vlx call resolve io.systemd.Resolve.ResolveHostname '{"name":"example.com"}'

# Reverse lookup (family 2 = AF_INET)
vlx call resolve io.systemd.Resolve.ResolveAddress \
  '{"family":2,"address":[93,184,215,14]}'

# Current DNS configuration
vlx call resolve io.systemd.Resolve.DumpDNSConfiguration

# Watch every query as it happens (root)
sudo vlx call io.systemd.Resolve.Monitor \
  io.systemd.Resolve.Monitor.SubscribeQueryResults --more
```

## Users and groups (systemd-userdbd)

```sh
# Every user, one record per line
vlx call multiplexer io.systemd.UserDatabase.GetUserRecord \
  '{"service":"io.systemd.Multiplexer"}' --more

# One user
vlx call multiplexer io.systemd.UserDatabase.GetUserRecord \
  '{"service":"io.systemd.Multiplexer","userName":"root"}'
```

User records can contain password hashes; vlx's explorer
[redacts](explorer.md#redaction) them on screen.

## Units (PID 1, v258+)

```sh
# Every unit
sudo vlx call manager io.systemd.Unit.List --more

# One unit
sudo vlx call manager io.systemd.Unit.List '{"name":"systemd-resolved.service"}'

# The manager itself
sudo vlx call manager io.systemd.Manager.Describe
```

## Machines and images (systemd-machined)

```sh
vlx call io.systemd.Machine io.systemd.Machine.List --more
vlx call io.systemd.MachineImage io.systemd.MachineImage.List --more
```

## Network (systemd-networkd)

```sh
vlx call io.systemd.Network io.systemd.Network.Describe
vlx call io.systemd.Network io.systemd.Network.GetStates
```

## Any service

Every systemd Varlink service also implements `io.systemd.service`:

```sh
vlx call resolve io.systemd.service.Ping
vlx call resolve io.systemd.service.GetEnvironment
```

## Is it Varlink yet?

```sh
vlx list --probe
```

lists every socket with its product and version. For the full list of
interfaces and their default sockets, see sdvarlink's
[socket reference](https://rossedman.github.io/sdvarlink/reference/sockets/).
