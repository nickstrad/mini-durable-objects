#!/usr/bin/env bash
# Flakiness check: run the registry tests N times under the race detector on a
# temp copy of the repo. The working tree is never modified.
#
#   agent_work/validation/flaky.sh [N] [REPO]
#     N     repetitions (default 20)
#     REPO  repo root (default: the repo containing this script)
set -euo pipefail

N="${1:-20}"
REPO="${2:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
# Keep these excludes in sync with skip in vlib/vlib.go.
rsync -a --exclude .git --exclude data --exclude bin --exclude agent_work "$REPO/" "$TMP/"

cd "$TMP"
echo "repo copy: $TMP"
echo "go test -race -count=$N -run '^TestRegistry' ./internal/store/"
go test -race -count="$N" -timeout 600s -run '^TestRegistry' ./internal/store/
