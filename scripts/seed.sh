#!/bin/sh
# Create task-alpha, task-beta, and COUNT fake objects (default 5)
# through the running API server (make start). Existing objects keep their status.
set -eu
cd "$(dirname "$0")/.."

count=${1:-5}
case "$count" in
  ''|*[!0-9]*)
    echo 'Usage: scripts/seed.sh [count]' >&2
    exit 2
    ;;
esac

ids='task-alpha task-beta'
i=1
while [ "$i" -le "$count" ]; do
  ids="$ids fake-$i"
  i=$((i + 1))
done

go build -o bin/mdo ./cmd/mdo
for id in $ids; do
  ./bin/mdo create "$id"
done
