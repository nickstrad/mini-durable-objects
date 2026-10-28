-- Contention for the two shared resources: the goroutine of each object
-- (depth, queue_ns) and the registry lock that all objects share (lock_ns).

-- Busy objects first. A high p95 queue with a low p95 exec means that the
-- requests wait for their object, not for SQLite.
.print '== contention: by object =='
SELECT
    object,
    count(*)                                  AS n,
    max(depth)                                AS max_depth,
    round(avg(depth), 2)                      AS avg_depth,
    round(quantile_cont(queue_ms, 0.95), 3)   AS p95_queue_ms,
    round(quantile_cont(exec_ms, 0.95), 3)    AS p95_exec_ms,
    round(100 * sum(queue_ns) / sum(total_ns), 1) AS queue_pct
FROM req
WHERE ran
GROUP BY object
ORDER BY sum(queue_ns) DESC
LIMIT 20;

-- The wait in the inbox for each depth. If the object runs one request at a
-- time, the queue time increases about linearly with depth, by about one
-- exec time for each request in front.
.print '== contention: queue time by depth =='
SELECT
    depth,
    count(*)                                  AS n,
    round(avg(queue_ms), 3)                   AS avg_queue_ms,
    round(avg(exec_ms), 3)                    AS avg_exec_ms,
    round(avg(queue_ms) / nullif(depth * avg(exec_ms), 0), 2) AS queue_per_exec_ahead
FROM req
WHERE ran
GROUP BY depth
ORDER BY depth;

-- The registry lock is shared by all objects. Its wait must stay small
-- compared to the other steps. If it does not, a request for one object
-- slows down the requests for all other objects.
.print '== contention: registry lock =='
SELECT
    count(*)                                  AS n,
    round(quantile_cont(lock_ms, 0.50), 4)    AS p50_lock_ms,
    round(quantile_cont(lock_ms, 0.95), 4)    AS p95_lock_ms,
    round(max(lock_ms), 4)                    AS max_lock_ms,
    count(*) FILTER (lock_ms > 1)             AS over_1ms
FROM req
WHERE ran;
