#!/usr/bin/env bash
# Regenerates internal/safety/systemd_idl_test.go, which imports the IDL of
# every systemd release sdvarlink ships bindings for, so that
# TestBuiltinCoversSystemd fails when a new systemd method lacks a rule.
set -euo pipefail
cd "$(dirname "$0")/.."
dir=$(go list -m -f '{{.Dir}}' github.com/rossedman/sdvarlink)
out=internal/safety/systemd_idl_test.go
pkgs=()
for v in "$dir"/systemd/v*/; do
  v=$(basename "$v")
  for p in "$dir/systemd/$v"/*/; do
    pkgs+=("$v/$(basename "$p")")
  done
done
{
  echo 'package safety'
  echo
  echo "// Code generated for tests by listing sdvarlink's systemd packages. DO NOT EDIT by hand;"
  echo '// regenerate with hack/gen-safety-test.sh when sdvarlink adds a systemd release.'
  echo
  echo 'import ('
  for p in "${pkgs[@]}"; do echo "	${p%%/*}${p##*/} \"github.com/rossedman/sdvarlink/systemd/$p\""; done
  echo ')'
  echo
  echo 'var systemdIDL = []string{'
  for p in "${pkgs[@]}"; do echo "	${p%%/*}${p##*/}.InterfaceDescription,"; done
  echo '}'
} > "$out"
gofmt -w "$out"
echo "wrote $out (${#pkgs[@]} interfaces)"
