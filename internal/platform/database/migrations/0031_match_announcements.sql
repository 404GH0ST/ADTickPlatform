CREATE TABLE IF NOT EXISTS match_announcements (
    id BIGSERIAL PRIMARY KEY,
    body TEXT NOT NULL,
    created_by TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS match_announcements_created_idx
    ON match_announcements (created_at DESC);
