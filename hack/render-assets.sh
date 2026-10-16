#!/usr/bin/env bash
# Re-renders the banner and diagrams in docs/assets/ from their HTML sources
# in docs/assets/src/. Needs node and Playwright's Chromium.
set -euo pipefail
cd "$(dirname "$0")/.."
export NODE_PATH="${NODE_PATH:-$(npm root -g)}"
node hack/render.cjs docs/assets/src/banner.html docs/assets/banner.png 1280 400
node hack/render.cjs docs/assets/src/architecture.html docs/assets/architecture.png 1200 620
node hack/render.cjs docs/assets/src/safety.html docs/assets/safety.png 1200 640
node hack/render.cjs docs/assets/src/quickstart.html docs/assets/quickstart.png 1200 330
echo "rendered docs/assets/*.png"
