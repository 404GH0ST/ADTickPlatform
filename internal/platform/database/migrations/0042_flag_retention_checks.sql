ALTER TABLE issued_flags
    ADD COLUMN checker_metadata TEXT NOT NULL DEFAULT '',
    ADD COLUMN checker_token_snapshot TEXT NOT NULL DEFAULT '',
    ADD COLUMN retention_check_at TIMESTAMPTZ;

-- Sticky per-instance/tick losses survive retries, recovery, and process restarts.
CREATE TABLE flag_retention_failures (
    tick_id INTEGER NOT NULL REFERENCES game_ticks(id) ON DELETE CASCADE,
    team_id INTEGER NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    challenge_id INTEGER NOT NULL REFERENCES challenges(id) ON DELETE CASCADE,
    flag TEXT NOT NULL REFERENCES issued_flags(flag) ON DELETE CASCADE,
    confirmed_missing BOOLEAN NOT NULL DEFAULT FALSE,
    penalty_points DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (penalty_points >= 0),
    checked_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (tick_id, team_id, challenge_id)
);
