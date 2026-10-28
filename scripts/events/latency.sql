-- Where the time of a request goes, step by step across the integration
-- points: the registry lock, the database open, the object inbox, and SQLite.
-- Only requests that the object goroutine ran (ran = true).
.print '== latency: percentiles of total_ms =='
SELECT
    op, kind, cold,
    count(*)                                   AS n,
    round(quantile_cont(total_ms, 0.50), 3)    AS p50_ms,
    round(quantile_cont(total_ms, 0.95), 3)    AS p95_ms,
    round(quantile_cont(total_ms, 0.99), 3)    AS p99_ms,
    round(max(total_ms), 3)                    AS max_ms
FROM req
WHERE ran
GROUP BY ALL
ORDER BY op, kind, cold;

-- The share of each step in the sum of all request time. If queue_pct is
-- large, requests wait for their object more than they run.
.print '== latency: share of each step (percent of total time) =='
SELECT
    op,
    count(*)                                         AS n,
    round(100 * sum(lock_ns)  / sum(total_ns), 1)    AS lock_pct,
    round(100 * sum(open_ns)  / sum(total_ns), 1)    AS open_pct,
    round(100 * sum(queue_ns) / sum(total_ns), 1)    AS queue_pct,
    round(100 * sum(exec_ns)  / sum(total_ns), 1)    AS exec_pct,
    round(100 * sum(other_ms) / sum(total_ms), 1)    AS other_pct
FROM req
WHERE ran
GROUP BY op
ORDER BY op;
