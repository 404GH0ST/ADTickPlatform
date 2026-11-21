-- Challenge category (what kind of target it is) and connection protocol (how
-- participants reach it). Both are display metadata for participants and
-- attack scripts: they never change deployment, scoring or networking.
--
-- category: '' means uncategorized (challenges created before this column).
-- protocol: 'tcp' is the neutral default, since every service is reachable as
-- raw TCP; 'http' tells participants to open it as a web endpoint.
ALTER TABLE challenges
    ADD COLUMN IF NOT EXISTS category TEXT NOT NULL DEFAULT '';

ALTER TABLE challenges
    ADD COLUMN IF NOT EXISTS protocol TEXT NOT NULL DEFAULT 'tcp';

ALTER TABLE challenges
    ADD CONSTRAINT challenges_category_known
    CHECK (category IN ('', 'pwn', 'web', 'crypto'));

ALTER TABLE challenges
    ADD CONSTRAINT challenges_protocol_known
    CHECK (protocol IN ('tcp', 'http'));
