-- The rules from "Rules that the events must obey" in CODING_STYLE.md, as
-- queries. Each row counts the violations of one rule: 0 is correct. Rule 3
-- (each committed change has an event) needs the value in the database, so it
-- is not here.
.print '== invariants: violations (0 is correct) =='

-- Rule 1: one request at a time for each object. exec_at and exec_ns come
-- from the same monotonic clock, so the check needs no tolerance.
SELECT '1 exec intervals overlap for one object' AS rule, count(*) AS violations
FROM req a
JOIN req b
  ON a.run_id = b.run_id AND a.object = b.object AND a.seq < b.seq
 AND a.ran AND b.ran
 AND a.exec_at < b.exec_end_at AND b.exec_at < a.exec_end_at

UNION ALL
-- Rule 2: the steps fit in the total.
SELECT '2 steps longer than total', count(*)
FROM requests
WHERE lock_ns + open_ns + queue_ns + exec_ns > total_ns
   OR least(lock_ns, open_ns, queue_ns, exec_ns, total_ns) < 0

UNION ALL
-- Rule 4a: for each object, activated and evicted alternate, and the first
-- event is activated.
SELECT '4a object events out of sequence', count(*)
FROM (
    SELECT event, lag(event) OVER (PARTITION BY run_id, object ORDER BY seq) AS prev
    FROM objects
)
WHERE event = prev OR (prev IS NULL AND event <> 'activated')

UNION ALL
-- Rule 4b: the requests count in an evicted row is equal to the requests
-- that ran in that activation. A request belongs to the activation in which
-- it started to run.
SELECT '4b evicted.requests does not match', count(*)
FROM (
    SELECT
        e.requests,
        (SELECT count(*) FROM req r
          WHERE r.run_id = e.run_id AND r.object = e.object AND r.ran
            AND r.exec_at BETWEEN a.at AND e.at) AS counted
    FROM objects e
    JOIN LATERAL (
        SELECT max("at") AS "at" FROM objects a
        WHERE a.run_id = e.run_id AND a.object = e.object
          AND a.event = 'activated' AND a.seq < e.seq
    ) a ON true
    WHERE e.event = 'evicted'
)
WHERE requests <> counted

UNION ALL
-- Rule 5a: after a clean stop, no event was dropped.
SELECT '5a dropped events', coalesce(sum(dropped), 0)
FROM runs

UNION ALL
-- Rule 5b: seq has no gaps, and written is equal to the number of rows.
SELECT '5b seq gaps or written mismatch', count(*)
FROM (
    SELECT run_id, count(*) AS n, max(seq) - min(seq) + 1 AS span
    FROM (SELECT run_id, seq FROM requests UNION ALL SELECT run_id, seq FROM objects)
    GROUP BY run_id
) s
LEFT JOIN runs u USING (run_id)
WHERE s.n <> s.span OR (u.run_id IS NOT NULL AND u.written <> s.n);
