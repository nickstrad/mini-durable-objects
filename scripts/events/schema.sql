-- Typed tables for the event CSVs (see "## Events" in CODING_STYLE.md), and
-- views with derived columns. scripts/events.sh loads the CSVs into these
-- tables. A table stays empty when its files do not exist, for example the
-- runs table after the server was killed.

CREATE TABLE requests (
    run_id VARCHAR, seq BIGINT, start_at BIGINT, object VARCHAR, op VARCHAR,
    kind VARCHAR, ran BOOLEAN, depth INTEGER, lock_ns BIGINT, open_ns BIGINT,
    queue_ns BIGINT, exec_at BIGINT, exec_ns BIGINT, total_ns BIGINT, "rows" BIGINT
);

CREATE TABLE objects (
    run_id VARCHAR, seq BIGINT, object VARCHAR, "at" BIGINT, event VARCHAR,
    open_ns BIGINT, idle_ns BIGINT, lifetime_ns BIGINT, requests BIGINT
);

CREATE TABLE runs (
    run_id VARCHAR, started_at BIGINT, stopped_at BIGINT, written BIGINT,
    dropped BIGINT, max_buffered BIGINT
);

-- One row for each request, with times in milliseconds and the end of the
-- exec interval. Time points are nanoseconds since the run started.
-- exec_at is NULL for a request that did not run.
CREATE VIEW req AS
SELECT
    *,
    exec_at + exec_ns                                AS exec_end_at,
    open_ns > 0                                      AS cold,
    lock_ns / 1e6                                    AS lock_ms,
    open_ns / 1e6                                    AS open_ms,
    queue_ns / 1e6                                   AS queue_ms,
    exec_ns / 1e6                                    AS exec_ms,
    total_ns / 1e6                                   AS total_ms,
    -- Time in the store method that is not one of the measured steps: the
    -- check that the database file exists, the error wrap, and Go scheduling.
    (total_ns - lock_ns - open_ns - queue_ns - exec_ns) / 1e6 AS other_ms
FROM requests;
