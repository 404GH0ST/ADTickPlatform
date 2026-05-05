ALTER TABLE submitted_flags DROP CONSTRAINT IF EXISTS submitted_flags_pkey;

ALTER TABLE submitted_flags
    ADD PRIMARY KEY (flag, team_id);
