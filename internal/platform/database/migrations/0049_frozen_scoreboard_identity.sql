ALTER TABLE teams ADD COLUMN score_identity UUID NOT NULL UNIQUE DEFAULT gen_random_uuid();

-- Attach identities to existing frozen rows whose team can still be identified
-- by its saved name. Unmatched rows retain their scores and remain ineligible.
UPDATE scoreboard_freeze AS f
SET snapshot = (
    SELECT COALESCE(jsonb_agg(
        CASE WHEN t.score_identity IS NOT NULL
            THEN saved.row || jsonb_build_object('team_identity', t.score_identity)
            ELSE saved.row END
        ORDER BY saved.position
    ), '[]'::jsonb)
    FROM jsonb_array_elements(f.snapshot) WITH ORDINALITY AS saved(row, position)
    LEFT JOIN teams t ON t.name = saved.row->>'team'
)
WHERE jsonb_typeof(snapshot) = 'array';
