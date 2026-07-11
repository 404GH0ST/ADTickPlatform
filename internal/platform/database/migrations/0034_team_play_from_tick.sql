-- After mid-match team reactivation, the team re-enters checker/scoring/network
-- play only once game_ticks.id reaches play_from_tick (typically next tick).
-- Warm redeploy can finish earlier without leaking placeholder flags.
-- NULL means no deferred gate (normal live team).
ALTER TABLE teams
    ADD COLUMN IF NOT EXISTS play_from_tick INTEGER NULL;
