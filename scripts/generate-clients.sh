#!/bin/sh
set -eu
cd "$(dirname "$0")/.."

output=internal/apiclient/client.gen.go
case "${1:-}" in
  '')
    go tool oapi-codegen -config api/client.yaml -o "$output" api/openapi.yaml
    ;;
  --check)
    generated=$(mktemp)
    trap 'rm -f "$generated"' EXIT HUP INT TERM
    go tool oapi-codegen -config api/client.yaml -o "$generated" api/openapi.yaml
    if ! cmp -s "$generated" "$output"; then
      echo 'Generated client is stale. Run make generate and commit the result.' >&2
      exit 1
    fi
    ;;
  *)
    echo 'Usage: scripts/generate-clients.sh [--check]' >&2
    exit 2
    ;;
esac
