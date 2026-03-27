CREATE TABLE IF NOT EXISTS issued_flags (
    flag TEXT PRIMARY KEY,
    owner_team_id INTEGER NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    owner_team_name TEXT NOT NULL,
    challenge_id INTEGER NOT NULL REFERENCES challenges(id) ON DELETE CASCADE,
    challenge_name TEXT NOT NULL,
    issued_tick INTEGER NOT NULL REFERENCES game_ticks(id) ON DELETE CASCADE,
    expires_tick INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS issued_flags_owner_idx ON issued_flags (owner_team_id, challenge_id, issued_tick DESC);
