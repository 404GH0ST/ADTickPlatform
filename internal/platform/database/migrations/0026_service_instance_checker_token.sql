ALTER TABLE service_instances
    ADD COLUMN IF NOT EXISTS checker_token TEXT NOT NULL DEFAULT '';

UPDATE service_instances
SET checker_token = md5(random()::text || clock_timestamp()::text || team_id::text || ':' || challenge_id::text)
WHERE checker_token = '';
