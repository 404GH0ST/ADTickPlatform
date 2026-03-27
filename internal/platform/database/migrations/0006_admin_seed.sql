UPDATE challenges SET baseline_image = 'registry.local/banking:baseline', checker_image = 'registry.local/banking-checker:latest', weight = 1, published = TRUE WHERE id = 1;
UPDATE challenges SET baseline_image = 'registry.local/chat:baseline', checker_image = 'registry.local/chat-checker:latest', weight = 1, published = TRUE WHERE id = 2;
UPDATE challenges SET baseline_image = 'registry.local/storage:baseline', checker_image = 'registry.local/storage-checker:latest', weight = 1, published = TRUE WHERE id = 3;

INSERT INTO players (id, team_id, display_name, email, password_hash, role, wireguard_peer, created_at) VALUES
    (1, 101, 'Alpha Captain', 'alpha.captain@example.com', md5('alpha-secret'), 'captain', 'team-101-player-1', NOW() - INTERVAL '72 hours'),
    (2, 102, 'Delta Captain', 'delta.captain@example.com', md5('delta-secret'), 'captain', 'team-102-player-2', NOW() - INTERVAL '71 hours'),
    (3, 103, 'Sigma Captain', 'sigma.captain@example.com', md5('sigma-secret'), 'captain', 'team-103-player-3', NOW() - INTERVAL '70 hours'),
    (4, 104, 'Orchid Captain', 'orchid.captain@example.com', md5('orchid-secret'), 'captain', 'team-104-player-4', NOW() - INTERVAL '69 hours')
ON CONFLICT (id) DO NOTHING;
