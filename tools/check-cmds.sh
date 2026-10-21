#!/usr/bin/env sh
# Enforces one Cobra command per file in internal/cli (vlx keeps its
# commands there rather than in cmd/). A file may define no command (shared
# helpers such as app.go) or exactly one; a file with a new...Command
# constructor must define exactly one.
#
#   tools/check-cmds.sh [DIR]     # DIR defaults to internal/cli
dir=${1:-internal/cli}
fail=0
for f in "$dir"/*.go; do
  case "$f" in *_test.go) continue ;; esac
  n=$(grep -c '&cobra.Command{' "$f")
  if [ "$n" -gt 1 ]; then
    echo "$f defines $n commands; split it into one file per command"
    fail=1
  elif grep -q '^func new[A-Za-z]*Command(' "$f" && [ "$n" -ne 1 ]; then
    echo "$f has a command constructor but defines $n commands"
    fail=1
  fi
done
exit $fail
