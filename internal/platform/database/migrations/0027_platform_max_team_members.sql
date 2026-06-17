ALTER TABLE platform_settings
    ADD COLUMN IF NOT EXISTS max_team_members INTEGER NOT NULL DEFAULT 0;
