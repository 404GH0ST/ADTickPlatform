ALTER TABLE challenges ADD COLUMN IF NOT EXISTS service_port INTEGER NOT NULL DEFAULT 0;
ALTER TABLE challenges ADD COLUMN IF NOT EXISTS service_subnet_octet INTEGER NOT NULL DEFAULT 0;

UPDATE challenges
SET service_port = 10000 + id
WHERE service_port <= 0;

UPDATE challenges
SET service_subnet_octet = id
WHERE service_subnet_octet <= 0;
