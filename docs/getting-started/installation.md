---
description: Install vlx from Go, release archives or Linux packages, and set up shell completion.
---

# Installation

vlx is a single static binary with no runtime dependencies.

| Platform | Supported |
| --- | --- |
| Linux | amd64, arm64, armv7, riscv64, ppc64le, s390x |
| macOS | amd64, arm64 (the demo service and any Varlink socket; macOS has no systemd) |
| systemd | v257+ serves most `io.systemd.*` APIs over Varlink; older releases have fewer |

## With Go

Needs Go 1.24 or newer.

```sh
go install github.com/rossedman/vlx/cmd/vlx@latest
```

## From a release

Every [release](https://github.com/rossedman/vlx/releases) has archives for
each platform, `.deb`, `.rpm` and `.apk` packages, and a `checksums.txt`.

```sh
VERSION=0.1.0
curl -LO https://github.com/rossedman/vlx/releases/download/v$VERSION/vlx_${VERSION}_linux_amd64.tar.gz
curl -LO https://github.com/rossedman/vlx/releases/download/v$VERSION/checksums.txt
sha256sum --ignore-missing -c checksums.txt
tar -xzf vlx_${VERSION}_linux_amd64.tar.gz
sudo install vlx /usr/local/bin/
```

Archives also contain shell completions (`completions/`) and man pages
(`manpages/`). The Linux packages install both for you.

## From source

```sh
git clone https://github.com/rossedman/vlx
cd vlx
make build        # writes ./bin/vlx
```

## Shell completion

Completion knows commands, flags and the names of discovered sockets.

=== "bash"

    ```sh
    vlx completion bash | sudo tee /etc/bash_completion.d/vlx >/dev/null
    ```

=== "zsh"

    ```sh
    vlx completion zsh > "${fpath[1]}/_vlx"
    ```

=== "fish"

    ```sh
    vlx completion fish > ~/.config/fish/completions/vlx.fish
    ```

## Verify

```sh
vlx version
vlx demo
```
