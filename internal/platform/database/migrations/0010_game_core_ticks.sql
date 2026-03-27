CREATE TABLE IF NOT EXISTS game_ticks (
    id INTEGER PRIMARY KEY,
    status TEXT NOT NULL,
    total_checker_runs INTEGER NOT NULL DEFAULT 0,
    successful_checker_runs INTEGER NOT NULL DEFAULT 0,
    failed_checker_runs INTEGER NOT NULL DEFAULT 0,
    skipped_checker_runs INTEGER NOT NULL DEFAULT 0,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ NULL,
    message TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS checker_runs (
    id BIGSERIAL PRIMARY KEY,
    tick_id INTEGER NOT NULL REFERENCES game_ticks(id) ON DELETE CASCADE,
    team_id INTEGER NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    team_name TEXT NOT NULL,
    challenge_id INTEGER NOT NULL REFERENCES challenges(id) ON DELETE CASCADE,
    challenge_name TEXT NOT NULL,
    phase TEXT NOT NULL,
    target TEXT NOT NULL,
    checker_image TEXT NOT NULL,
    status TEXT NOT NULL,
    exit_code INTEGER NOT NULL,
    message TEXT NOT NULL DEFAULT '',
    output TEXT NOT NULL DEFAULT '',
    checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS checker_runs_tick_id_idx ON checker_runs (tick_id DESC, id DESC);
CREATE INDEX IF NOT EXISTS checker_runs_team_id_idx ON checker_runs (team_id, challenge_id, phase);
