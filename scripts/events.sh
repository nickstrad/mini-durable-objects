#!/bin/sh
# Analyze the server's event CSVs with DuckDB.
#
#   scripts/events.sh [-d DIR] [QUERY...]
#
# DIR defaults to data/events. QUERY is one of the files in scripts/events/
# without .sql (summary, latency, contention, lifecycle, timeline, invariants).
# With no QUERY, all of them run. Start the server with -events DIR to record.
set -eu
cd "$(dirname "$0")/.."

dir=data/events
if [ "${1:-}" = -d ]; then
  dir=$2
  shift 2
fi
queries=${*:-summary latency contention lifecycle timeline invariants}

command -v duckdb >/dev/null || { echo 'duckdb not found; see https://duckdb.org/docs/installation' >&2; exit 2; }
ls "$dir"/requests-*.csv >/dev/null 2>&1 || { echo "no events in $dir; start the server with -events $dir" >&2; exit 2; }

sql=$(mktemp)
trap 'rm -f "$sql"' EXIT HUP INT TERM
{
  echo "SET VARIABLE events_dir = '$dir';"
  cat scripts/events/schema.sql
  # Load only the tables that have files: a killed run has no runs file.
  for table in requests objects runs; do
    if ls "$dir"/$table-*.csv >/dev/null 2>&1; then
      echo "INSERT INTO $table BY NAME SELECT * FROM read_csv(getvariable('events_dir') || '/$table-*.csv', header = true);"
    fi
  done
  for q in $queries; do
    [ -f "scripts/events/$q.sql" ] || { echo "unknown query: $q" >&2; exit 2; }
    cat "scripts/events/$q.sql"
  done
} > "$sql"
duckdb -c ".read $sql"
