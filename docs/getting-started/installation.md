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

Needs Go 1.26 or newer.

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

## Verify a download

Releases after v0.2.1 are signed keylessly with [Sigstore](https://www.sigstore.dev/)
from the release workflow: `checksums.txt` has a cosign signature bundle,
`checksums.txt.sigstore.json`, and covers every archive and package. Each
release also has a GitHub build provenance attestation. To check a download,
put it in a directory with both files and run (with
[cosign](https://docs.sigstore.dev/cosign/system_config/installation/) v2.4 or
newer, and `VERSION` set to the release tag, e.g. `v0.3.0`):

```sh
cosign verify-blob \
  --certificate-identity "https://github.com/rossedman/vlx/.github/workflows/release.yml@refs/tags/${VERSION}" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --bundle checksums.txt.sigstore.json \
  checksums.txt
sha256sum --check --ignore-missing checksums.txt
```

Or check the provenance with the GitHub CLI:

```sh
gh attestation verify vlx_*.tar.gz --repo rossedman/vlx
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
