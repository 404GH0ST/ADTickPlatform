CREATE TABLE IF NOT EXISTS game_match_state (
    singleton BOOLEAN PRIMARY KEY DEFAULT TRUE,
    state TEXT NOT NULL DEFAULT 'not_started',
    started_at TIMESTAMPTZ NULL,
    ended_at TIMESTAMPTZ NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT game_match_state_valid_state CHECK (state IN ('not_started', 'running', 'finished'))
);
