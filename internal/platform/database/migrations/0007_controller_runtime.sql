CREATE TABLE IF NOT EXISTS deployment_jobs (
    id INTEGER PRIMARY KEY,
    challenge_id INTEGER NOT NULL REFERENCES challenges(id) ON DELETE CASCADE,
    challenge_name TEXT NOT NULL,
    status TEXT NOT NULL,
    target_team_count INTEGER NOT NULL,
    queued_team_count INTEGER NOT NULL,
    ready_team_count INTEGER NOT NULL DEFAULT 0,
    failed_team_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS service_instances (
    team_id INTEGER NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    challenge_id INTEGER NOT NULL REFERENCES challenges(id) ON DELETE CASCADE,
    deployment_job_id INTEGER REFERENCES deployment_jobs(id) ON DELETE SET NULL,
    runtime_kind TEXT NOT NULL,
    runtime_status TEXT NOT NULL,
    container_name TEXT NOT NULL,
    baseline_image TEXT NOT NULL,
    checker_image TEXT NOT NULL,
    endpoint TEXT NOT NULL,
    ssh_host TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (team_id, challenge_id)
);

INSERT INTO service_instances (
    team_id,
    challenge_id,
    deployment_job_id,
    runtime_kind,
    runtime_status,
    container_name,
    baseline_image,
    checker_image,
    endpoint,
    ssh_host,
    created_at,
    updated_at
)
SELECT
    tss.team_id,
    tss.challenge_id,
    NULL,
    'docker',
    'ready',
    format('svc-%s-team-%s', regexp_replace(lower(c.name), '[^a-z0-9]+', '-', 'g'), tss.team_id),
    c.baseline_image,
    c.checker_image,
    tss.endpoint,
    split_part(tss.endpoint, ':', 1),
    NOW(),
    NOW()
FROM team_service_states tss
JOIN challenges c ON c.id = tss.challenge_id
ON CONFLICT (team_id, challenge_id) DO NOTHING;
