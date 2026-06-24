-- Mid-match deactivation support for teams and players.
-- A deactivated team drops off the leaderboard, becomes non-targetable by the
-- checker, and has its instances torn down; a deactivated player can no longer
-- authenticate. Defaults keep every existing row live, so applying this
-- migration changes no behavior until an organizer toggles a flag.
ALTER TABLE teams
    ADD COLUMN IF NOT EXISTS active BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE teams
    ADD COLUMN IF NOT EXISTS deactivated_at TIMESTAMPTZ NULL;

ALTER TABLE players
    ADD COLUMN IF NOT EXISTS active BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE players
    ADD COLUMN IF NOT EXISTS deactivated_at TIMESTAMPTZ NULL;
