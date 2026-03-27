INSERT INTO teams (id, name, email) VALUES
    (101, 'Team Alpha', 'team-alpha@example.com'),
    (102, 'Team Delta', 'team-delta@example.com'),
    (103, 'Team Sigma', 'team-sigma@example.com'),
    (104, 'Team Orchid', 'team-orchid@example.com')
ON CONFLICT (id) DO NOTHING;

INSERT INTO challenges (id, name) VALUES
    (1, 'banking'),
    (2, 'chat'),
    (3, 'storage')
ON CONFLICT (id) DO NOTHING;

INSERT INTO team_service_states (team_id, challenge_id, endpoint, status, checker, unlocked, ssh_hint, last_event, reset_cooldown) VALUES
    (101, 1, '10.80.1.11:10001', 'stable', 'passing', FALSE, 'solve service to generate SSH credential', 'no patch applied yet', 'ready'),
    (101, 2, '10.80.2.11:10002', 'stable', 'passing', FALSE, 'solve service to generate SSH credential', 'no patch applied yet', 'ready'),
    (101, 3, '10.80.3.11:10003', 'stable', 'passing', FALSE, 'solve service to generate SSH credential', 'no patch applied yet', 'ready'),
    (102, 1, '10.80.1.12:10001', 'stable', 'passing', FALSE, 'solve service to generate SSH credential', 'no patch applied yet', 'ready'),
    (102, 2, '10.80.2.12:10002', 'stable', 'passing', FALSE, 'solve service to generate SSH credential', 'no patch applied yet', 'ready'),
    (102, 3, '10.80.3.12:10003', 'stable', 'passing', FALSE, 'solve service to generate SSH credential', 'no patch applied yet', 'ready'),
    (103, 1, '10.80.1.13:10001', 'stable', 'passing', FALSE, 'solve service to generate SSH credential', 'no patch applied yet', 'ready'),
    (103, 2, '10.80.2.13:10002', 'stable', 'passing', FALSE, 'solve service to generate SSH credential', 'no patch applied yet', 'ready'),
    (103, 3, '10.80.3.13:10003', 'stable', 'passing', FALSE, 'solve service to generate SSH credential', 'no patch applied yet', 'ready'),
    (104, 1, '10.80.1.14:10001', 'stable', 'passing', FALSE, 'solve service to generate SSH credential', 'no patch applied yet', 'ready'),
    (104, 2, '10.80.2.14:10002', 'stable', 'passing', FALSE, 'solve service to generate SSH credential', 'no patch applied yet', 'ready'),
    (104, 3, '10.80.3.14:10003', 'stable', 'passing', FALSE, 'solve service to generate SSH credential', 'no patch applied yet', 'ready')
ON CONFLICT (team_id, challenge_id) DO NOTHING;

INSERT INTO scoreboard_entries (team_id, rank, team_name, attack_points, defense_points, sla_points, total_points, delta) VALUES
    (101, 1, 'Team Alpha', 0, 0, 0, 0, '0'),
    (102, 2, 'Team Delta', 0, 0, 0, 0, '0'),
    (103, 3, 'Team Sigma', 0, 0, 0, 0, '0'),
    (104, 4, 'Team Orchid', 0, 0, 0, 0, '0')
ON CONFLICT (team_id) DO NOTHING;
