DELETE FROM submission_events
WHERE status <> 'accepted' AND submitted_at < NOW() - INTERVAL '7 days';

DELETE FROM submission_events AS event
USING (
    SELECT id, row_number() OVER (PARTITION BY team_id ORDER BY id DESC) AS position
    FROM submission_events
    WHERE status <> 'accepted'
) AS rejected
WHERE event.id = rejected.id AND rejected.position > 4096;

UPDATE submission_events
SET flag = left(flag, 128)
WHERE status <> 'accepted' AND octet_length(flag) > 512;

ALTER TABLE submission_events ADD CONSTRAINT submission_events_rejected_flag_size
    CHECK (status = 'accepted' OR octet_length(flag) <= 512);

CREATE INDEX submission_events_rejected_team_idx ON submission_events (team_id, id DESC)
    WHERE status <> 'accepted';
CREATE INDEX submission_events_rejected_age_idx ON submission_events (submitted_at)
    WHERE status <> 'accepted';
