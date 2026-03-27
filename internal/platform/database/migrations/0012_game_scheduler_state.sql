CREATE TABLE IF NOT EXISTS game_scheduler_state (
    singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
    state TEXT NOT NULL DEFAULT 'stopped',
    interval_seconds INTEGER NOT NULL DEFAULT 60,
    last_run_at TIMESTAMPTZ NULL,
    next_run_at TIMESTAMPTZ NULL,
    last_tick_id INTEGER NULL REFERENCES game_ticks(id) ON DELETE SET NULL,
    last_error TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO game_scheduler_state (singleton, state, interval_seconds, last_error)
VALUES (TRUE, 'stopped', 60, '')
ON CONFLICT (singleton) DO NOTHING;

CREATE TABLE IF NOT EXISTS game_scheduler_events (
    id BIGSERIAL PRIMARY KEY,
    event_type TEXT NOT NULL,
    source TEXT NOT NULL,
    state TEXT NOT NULL,
    tick_id INTEGER NULL REFERENCES game_ticks(id) ON DELETE SET NULL,
    message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS game_scheduler_events_created_at_idx ON game_scheduler_events (created_at DESC, id DESC);
