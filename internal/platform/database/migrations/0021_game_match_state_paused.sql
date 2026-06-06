ALTER TABLE game_match_state DROP CONSTRAINT IF EXISTS game_match_state_valid_state;
ALTER TABLE game_match_state ADD CONSTRAINT game_match_state_valid_state CHECK (state IN ('not_started', 'running', 'paused', 'finished'));
