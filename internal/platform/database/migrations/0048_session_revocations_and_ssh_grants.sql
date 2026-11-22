CREATE TABLE revoked_player_sessions (
    session_id TEXT PRIMARY KEY,
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX revoked_player_sessions_expiry ON revoked_player_sessions (expires_at);

ALTER TABLE team_service_states ADD COLUMN ssh_grant_start_id BIGINT;
CREATE INDEX submission_events_accepted_grants
    ON submission_events (team_id, challenge_id, id) WHERE status = 'accepted';

-- Use the currently eligible captures as the basis for existing grants.
WITH proofs AS (
    SELECT e.team_id, e.challenge_id, MIN(e.id) AS first_id
    FROM submission_events e
    JOIN submitted_flags f ON f.team_id = e.team_id AND f.flag = e.flag
    JOIN teams t ON t.id = e.team_id
    JOIN challenges c ON c.id = e.challenge_id
    WHERE e.status = 'accepted'
      AND e.tick >= GREATEST(COALESCE(t.play_from_tick, 0), COALESCE(c.play_from_tick, 0))
      AND NOT c.maintenance
    GROUP BY e.team_id, e.challenge_id
)
UPDATE team_service_states s SET ssh_grant_start_id = p.first_id
FROM proofs p WHERE s.team_id = p.team_id AND s.challenge_id = p.challenge_id AND s.unlocked;

UPDATE team_service_states
SET unlocked = FALSE, ssh_hint = 'capture a flag to unlock SSH', last_event = 'SSH access requires a qualifying capture'
WHERE unlocked AND ssh_grant_start_id IS NULL;
