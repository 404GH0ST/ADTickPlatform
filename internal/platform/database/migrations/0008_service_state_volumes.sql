ALTER TABLE service_instances ADD COLUMN IF NOT EXISTS state_volume TEXT NOT NULL DEFAULT '';

UPDATE service_instances AS si
SET state_volume = format('svc-%s-team-%s-state', regexp_replace(lower(c.name), '[^a-z0-9]+', '-', 'g'), si.team_id)
FROM challenges c
WHERE c.id = si.challenge_id
  AND (si.state_volume = '' OR si.state_volume IS NULL);
