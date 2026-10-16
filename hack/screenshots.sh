#!/usr/bin/env bash
# Regenerates the TUI screenshots in docs/assets/screens/ from the TUI tests,
# which drive the real model against the demo service.
# Needs python3, node and Playwright's Chromium.
set -euo pipefail
cd "$(dirname "$0")/.."
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
VLX_SCREENSHOT_DIR="$tmp" go test -count=1 ./internal/tui/ >/dev/null
mkdir -p docs/assets/screens
export NODE_PATH="${NODE_PATH:-$(npm root -g)}"
for f in "$tmp"/*.ans; do
  name=$(basename "$f" .ans)
  python3 hack/ansi2html.py "$f" "vlx demo" > "$tmp/$name.html"
  node hack/shot.cjs "$tmp/$name.html" "docs/assets/screens/$name.png"
  echo "docs/assets/screens/$name.png"
done
