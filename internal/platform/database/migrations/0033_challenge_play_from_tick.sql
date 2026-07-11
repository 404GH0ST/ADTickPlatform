-- After maintenance resume, a challenge re-enters checker/scoring/participant
-- play only once game_ticks.id reaches play_from_tick (typically next tick).
-- NULL means no deferred gate (normal live challenge).
ALTER TABLE challenges
    ADD COLUMN IF NOT EXISTS play_from_tick INTEGER NULL;
