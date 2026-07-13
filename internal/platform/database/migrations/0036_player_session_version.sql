ALTER TABLE players
    ADD COLUMN IF NOT EXISTS session_version INTEGER NOT NULL DEFAULT 1;

UPDATE players
SET session_version = 1
WHERE session_version < 1;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'players_session_version_positive'
    ) THEN
        ALTER TABLE players
            ADD CONSTRAINT players_session_version_positive CHECK (session_version > 0);
    END IF;
END $$;
