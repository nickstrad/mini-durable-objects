-- Requests in one-second buckets: throughput, errors, latency, and queue
-- depth over time. Use it to find the moment when a run became slow.
.print '== timeline: one row for each second =='
WITH t AS (
    SELECT *, (start_at - min(start_at) OVER (PARTITION BY run_id)) // 1000000000 AS second
    FROM req
)
SELECT
    run_id,
    second,
    count(*)                                   AS n,
    count(*) FILTER (kind <> 'ok')             AS errors,
    round(quantile_cont(total_ms, 0.95), 3)    AS p95_total_ms,
    max(depth)                                 AS max_depth,
    count(*) FILTER (cold)                     AS cold
FROM t
GROUP BY run_id, second
ORDER BY run_id, second;
