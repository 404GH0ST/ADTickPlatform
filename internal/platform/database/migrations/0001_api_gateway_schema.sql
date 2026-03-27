CREATE TABLE IF NOT EXISTS teams (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS challenges (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS team_service_states (
    team_id INTEGER NOT NULL REFERENCES teams(id),
    challenge_id INTEGER NOT NULL REFERENCES challenges(id),
    endpoint TEXT NOT NULL,
    status TEXT NOT NULL,
    checker TEXT NOT NULL,
    unlocked BOOLEAN NOT NULL DEFAULT FALSE,
    ssh_hint TEXT NOT NULL,
    last_event TEXT NOT NULL,
    reset_cooldown TEXT NOT NULL,
    PRIMARY KEY (team_id, challenge_id)
);

CREATE TABLE IF NOT EXISTS scoreboard_entries (
    team_id INTEGER PRIMARY KEY REFERENCES teams(id),
    rank INTEGER NOT NULL,
    team_name TEXT NOT NULL,
    attack_points INTEGER NOT NULL,
    defense_points INTEGER NOT NULL,
    sla_points INTEGER NOT NULL,
    total_points INTEGER NOT NULL,
    delta TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS attack_events (
    id TEXT PRIMARY KEY,
    attacker TEXT NOT NULL,
    victim TEXT NOT NULL,
    service TEXT NOT NULL,
    tick INTEGER NOT NULL,
    verdict TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS submitted_flags (
    flag TEXT PRIMARY KEY,
    team_id INTEGER NOT NULL REFERENCES teams(id),
    submitted_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
