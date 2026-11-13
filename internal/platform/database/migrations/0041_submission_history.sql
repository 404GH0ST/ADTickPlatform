CREATE TABLE submission_events (
    id BIGSERIAL PRIMARY KEY,
    team_id INTEGER NOT NULL,
    attacker TEXT NOT NULL,
    victim TEXT NOT NULL DEFAULT '',
    challenge_id INTEGER NOT NULL DEFAULT 0,
    service TEXT NOT NULL DEFAULT '',
    flag TEXT NOT NULL,
    tick INTEGER NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('accepted', 'duplicate', 'invalid')),
    detail TEXT NOT NULL DEFAULT '',
    attack_event_id TEXT UNIQUE,
    submitted_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX submission_events_time_idx ON submission_events (submitted_at DESC, id DESC);
CREATE INDEX submission_events_status_time_idx ON submission_events (status, submitted_at DESC, id DESC);

-- Reconstruct accepted attempts. Rejections and duplicates were not retained before
-- this migration. Old attack IDs contain the first eight bytes of SHA256(flag:team).
INSERT INTO submission_events (team_id, attacker, victim, challenge_id, service, flag, tick, status, detail, attack_event_id, submitted_at)
SELECT sf.team_id, COALESCE(a.attacker, t.name, 'Team ' || sf.team_id),
       COALESCE(f.owner_team_name, a.victim, ''), COALESCE(f.challenge_id, a.challenge_id, 0),
       COALESCE(f.challenge_name, a.service, ''), sf.flag, COALESCE(a.tick, f.issued_tick, 0),
       'accepted', 'flag is correct.', a.id, sf.submitted_at
FROM submitted_flags sf
LEFT JOIN teams t ON t.id = sf.team_id
LEFT JOIN issued_flags f ON f.flag = sf.flag
LEFT JOIN LATERAL (
    SELECT ae.* FROM attack_events ae
    WHERE ae.id LIKE 'atk-submit-%-' || sf.team_id || '-' ||
        substr(encode(sha256(convert_to(sf.flag || ':' || sf.team_id, 'UTF8')), 'hex'), 1, 16)
    ORDER BY ae.created_at DESC, ae.id DESC LIMIT 1
) a ON TRUE;

-- A revision change tells live dashboards to reload their full attack slice,
-- including rows beyond the short SSE snapshot, after a capture is removed.
CREATE TABLE attack_feed_state (
    singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
    revision BIGINT NOT NULL DEFAULT 0
);
INSERT INTO attack_feed_state (singleton) VALUES (TRUE);
