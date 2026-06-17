CREATE TABLE IF NOT EXISTS platform_settings (
    id INTEGER PRIMARY KEY,
    flag_format_prefix TEXT NOT NULL DEFAULT 'PLAYIT',
    flag_format_active TEXT NOT NULL DEFAULT 'PLAYIT',
    max_team_members INTEGER NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by TEXT NOT NULL DEFAULT '',
    CONSTRAINT platform_settings_singleton CHECK (id = 1)
);

INSERT INTO platform_settings (id) VALUES (1)
ON CONFLICT (id) DO NOTHING;

ALTER TABLE platform_settings
    ADD COLUMN IF NOT EXISTS max_team_members INTEGER NOT NULL DEFAULT 0;
