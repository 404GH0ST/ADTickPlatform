-- Scoreboard freeze window. During a freeze, the participant-facing scoreboard
-- serves a snapshot captured when the window first became active, while
-- organizers keep seeing the live board. A single-row table (id = 1) holds the
-- window bounds and the captured snapshot; an empty window (NULL freeze_at)
-- means the board is live.
CREATE TABLE IF NOT EXISTS scoreboard_freeze (
    id INTEGER PRIMARY KEY,
    freeze_at TIMESTAMPTZ NULL,
    unfreeze_at TIMESTAMPTZ NULL,
    snapshot JSONB NULL,
    snapshot_taken_at TIMESTAMPTZ NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO scoreboard_freeze (id) VALUES (1) ON CONFLICT (id) DO NOTHING;
