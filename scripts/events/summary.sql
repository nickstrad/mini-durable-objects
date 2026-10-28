-- One row for each run: how much work, how many errors, and the quality of
-- the event data. clean_stop is false when the runs row is missing, which
-- means the server stopped without SIGINT or SIGTERM.
.print '== summary: one row for each run =='
SELECT
    r.run_id,
    count(*)                                          AS requests,
    count(*) FILTER (kind <> 'ok')                    AS errors,
    round(100.0 * count(*) FILTER (kind <> 'ok') / count(*), 1) AS error_pct,
    count(DISTINCT object)                            AS objects,
    round((max(r.start_at) - min(r.start_at)) / 1e9, 2) AS span_s,
    round(count(*) / greatest((max(r.start_at) - min(r.start_at)) / 1e9, 1e-9), 1) AS req_per_s,
    any_value(u.run_id) IS NOT NULL                   AS clean_stop,
    any_value(u.dropped)                              AS dropped,
    any_value(u.max_buffered)                         AS max_buffered
FROM requests r
LEFT JOIN runs u USING (run_id)
GROUP BY r.run_id
ORDER BY r.run_id;

.print '== summary: errors by kind =='
SELECT op, kind, count(*) AS n
FROM requests
GROUP BY ALL
ORDER BY op, n DESC;
