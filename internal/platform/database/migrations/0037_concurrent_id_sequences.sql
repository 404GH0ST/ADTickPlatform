CREATE SEQUENCE IF NOT EXISTS players_id_seq AS INTEGER OWNED BY players.id;
SELECT setval('players_id_seq', COALESCE(MAX(id), 1), COUNT(*) > 0) FROM players;
ALTER TABLE players ALTER COLUMN id SET DEFAULT nextval('players_id_seq');

CREATE SEQUENCE IF NOT EXISTS challenges_id_seq AS INTEGER OWNED BY challenges.id;
SELECT setval('challenges_id_seq', COALESCE(MAX(id), 1), COUNT(*) > 0) FROM challenges;
ALTER TABLE challenges ALTER COLUMN id SET DEFAULT nextval('challenges_id_seq');

CREATE SEQUENCE IF NOT EXISTS deployment_jobs_id_seq AS INTEGER OWNED BY deployment_jobs.id;
SELECT setval('deployment_jobs_id_seq', COALESCE(MAX(id), 1), COUNT(*) > 0) FROM deployment_jobs;
ALTER TABLE deployment_jobs ALTER COLUMN id SET DEFAULT nextval('deployment_jobs_id_seq');

CREATE SEQUENCE IF NOT EXISTS scoreboard_rank_seq AS INTEGER;
SELECT setval('scoreboard_rank_seq', COALESCE(MAX(rank), 1), COUNT(*) > 0) FROM scoreboard_entries;
