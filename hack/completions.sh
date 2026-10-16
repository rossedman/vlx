#!/usr/bin/env bash
# Generates shell completions and man pages for release archives and
# packages. GoReleaser runs it before building.
set -euo pipefail
cd "$(dirname "$0")/.."
rm -rf completions manpages
mkdir -p completions manpages
for sh in bash zsh fish; do
  go run ./cmd/vlx completion "$sh" > "completions/vlx.$sh"
done
go run ./cmd/vlx man manpages
gzip -9n manpages/*.1
