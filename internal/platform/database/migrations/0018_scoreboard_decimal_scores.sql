ALTER TABLE scoreboard_entries
    ALTER COLUMN attack_points TYPE DOUBLE PRECISION USING attack_points::double precision,
    ALTER COLUMN defense_points TYPE DOUBLE PRECISION USING defense_points::double precision,
    ALTER COLUMN sla_points TYPE DOUBLE PRECISION USING sla_points::double precision,
    ALTER COLUMN total_points TYPE DOUBLE PRECISION USING total_points::double precision;
