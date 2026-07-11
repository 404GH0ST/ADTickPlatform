-- Per-challenge maintenance mode. When maintenance is true, participant
-- actions and scoring for that challenge are blocked, and organizers tear down
-- team containers until resume requeues them.
ALTER TABLE challenges
    ADD COLUMN IF NOT EXISTS maintenance BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE challenges
    ADD COLUMN IF NOT EXISTS maintenance_at TIMESTAMPTZ NULL;
