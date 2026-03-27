CREATE TABLE IF NOT EXISTS wireguard_peers (
    player_id INTEGER PRIMARY KEY REFERENCES players(id) ON DELETE CASCADE,
    address TEXT NOT NULL UNIQUE,
    private_key TEXT NOT NULL,
    public_key TEXT NOT NULL,
    preshared_key TEXT NOT NULL,
    server_public_key TEXT NOT NULL,
    server_endpoint TEXT NOT NULL,
    dns TEXT NOT NULL,
    allowed_ips TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    config TEXT NOT NULL,
    issued_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ NULL,
    updated_at TIMESTAMPTZ NOT NULL
);
