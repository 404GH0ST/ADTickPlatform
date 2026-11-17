ALTER TABLE platform_settings
    ADD COLUMN IF NOT EXISTS theme_id TEXT NOT NULL DEFAULT 'graphite';
