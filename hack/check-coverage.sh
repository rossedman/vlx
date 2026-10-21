#!/usr/bin/env bash
# Fails when total statement coverage in a Go cover profile is below a
# minimum. Coverage is measured across packages (-coverpkg=./...), so code
# exercised by another package's tests counts. Only hand-written code is
# counted: generated files (tools/generated) and examples/ are dropped first.
#
#   hack/check-coverage.sh coverage.out [MIN]     # MIN defaults to 90
set -euo pipefail
profile=${1:?usage: hack/check-coverage.sh PROFILE [MIN]}
min=${2:-90}
hand="${profile%.out}.hand.out"
{
  head -1 "$profile"
  grep -v '^mode:' "$profile" |
    grep -vFf <(go run ./tools/generated ./...; echo '/nonexistent/') |
    grep -v '/examples/' || true
} >"$hand"
profile=$hand
total=$(go tool cover -func="$profile" | awk '/^total:/ {gsub("%", "", $3); print $3}')
if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
  echo "### Coverage: ${total}% (minimum ${min}%)" >> "$GITHUB_STEP_SUMMARY"
fi
if awk -v t="$total" -v m="$min" 'BEGIN { exit !(t + 0 < m + 0) }'; then
  echo "coverage ${total}% is below the ${min}% minimum" >&2
  echo "lowest-covered functions:" >&2
  go tool cover -func="$profile" | grep -v '^total:' | sort -k3 -n | head -15 >&2
  exit 1
fi
echo "coverage ${total}% (minimum ${min}%)"
