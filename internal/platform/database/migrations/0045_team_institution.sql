-- Institution (university, school, company) a team represents. Display-only:
-- shown beside the team name on the scoreboard, attack globe and first-blood
-- banner. Empty string means "not set".
ALTER TABLE teams
    ADD COLUMN IF NOT EXISTS institution TEXT NOT NULL DEFAULT '';
