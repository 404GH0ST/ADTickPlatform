ALTER TABLE teams ADD COLUMN IF NOT EXISTS join_key TEXT;

UPDATE teams
SET join_key = 'TEAM-' || UPPER(REPLACE(name, ' ', '-')) || '-' || id::TEXT
WHERE join_key IS NULL OR TRIM(join_key) = '';

ALTER TABLE teams ALTER COLUMN join_key SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS teams_join_key_key ON teams (join_key);
