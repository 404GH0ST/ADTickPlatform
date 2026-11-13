-- Persist the winner at acceptance time; feed filters never determine first blood.
ALTER TABLE attack_events ADD COLUMN challenge_id INTEGER;
ALTER TABLE attack_events ADD COLUMN first_blood BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE attack_events a SET challenge_id = c.id FROM challenges c WHERE c.name = a.service;
WITH winners AS (
    SELECT DISTINCT ON (service) id FROM attack_events ORDER BY service, created_at ASC, id ASC
)
UPDATE attack_events SET first_blood = TRUE WHERE id IN (SELECT id FROM winners);

-- AcceptFlagSubmission holds the exclusive game_ticks lock through this insert.
-- This constraint also protects against writers outside that path.
CREATE UNIQUE INDEX attack_events_first_blood_challenge_idx
    ON attack_events (challenge_id) WHERE first_blood AND challenge_id IS NOT NULL;
