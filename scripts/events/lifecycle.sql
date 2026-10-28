-- Activation and eviction of objects. The latency query shows the cost of a
-- cold request: one that opened the database file. "Cold" does not mean that
-- the data came from disk: the OS can have the file in its page cache.
.print '== lifecycle: activations and evictions =='
SELECT
    run_id,
    count(*) FILTER (event = 'activated')                  AS activations,
    count(*) FILTER (event = 'evicted')                    AS evictions,
    round(quantile_cont(open_ns / 1e6, 0.50) FILTER (event = 'activated'), 3) AS p50_open_ms,
    round(max(open_ns / 1e6) FILTER (event = 'activated'), 3)  AS max_open_ms,
    round(avg(lifetime_ns / 1e9) FILTER (event = 'evicted'), 2) AS avg_lifetime_s,
    round(min(idle_ns / 1e9) FILTER (event = 'evicted'), 2)    AS min_idle_s,
    round(avg(requests) FILTER (event = 'evicted'), 1)         AS avg_requests_per_activation
FROM objects
GROUP BY run_id
ORDER BY run_id;
