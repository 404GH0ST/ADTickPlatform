-- Add ON DELETE CASCADE to team_service_states
ALTER TABLE team_service_states DROP CONSTRAINT IF EXISTS team_service_states_team_id_fkey;
ALTER TABLE team_service_states ADD CONSTRAINT team_service_states_team_id_fkey FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE;

ALTER TABLE team_service_states DROP CONSTRAINT IF EXISTS team_service_states_challenge_id_fkey;
ALTER TABLE team_service_states ADD CONSTRAINT team_service_states_challenge_id_fkey FOREIGN KEY (challenge_id) REFERENCES challenges(id) ON DELETE CASCADE;

-- Add ON DELETE CASCADE to scoreboard_entries
ALTER TABLE scoreboard_entries DROP CONSTRAINT IF EXISTS scoreboard_entries_team_id_fkey;
ALTER TABLE scoreboard_entries ADD CONSTRAINT scoreboard_entries_team_id_fkey FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE;

-- Add ON DELETE CASCADE to submitted_flags
ALTER TABLE submitted_flags DROP CONSTRAINT IF EXISTS submitted_flags_team_id_fkey;
ALTER TABLE submitted_flags ADD CONSTRAINT submitted_flags_team_id_fkey FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE;
