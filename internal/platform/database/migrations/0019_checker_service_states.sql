CREATE TABLE IF NOT EXISTS checker_service_states (
    tick_id INTEGER NOT NULL REFERENCES game_ticks(id) ON DELETE CASCADE,
    team_id INTEGER NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    challenge_id INTEGER NOT NULL REFERENCES challenges(id) ON DELETE CASCADE,
    service_state TEXT NOT NULL,
    state_phase TEXT NOT NULL DEFAULT '',
    state_message TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tick_id, team_id, challenge_id)
);

CREATE INDEX IF NOT EXISTS checker_service_states_team_challenge_idx
    ON checker_service_states (team_id, challenge_id, tick_id DESC);
