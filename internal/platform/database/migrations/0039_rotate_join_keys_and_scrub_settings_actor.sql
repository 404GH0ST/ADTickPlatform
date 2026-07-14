-- The original join-key backfill derived credentials from public team data.
-- Rotate every team so upgraded installations cannot retain a predictable key.
UPDATE teams
SET join_key = 'TEAM-' || UPPER(REPLACE(gen_random_uuid()::TEXT, '-', ''));

-- Older API versions stored ADMIN_API_TOKEN as the settings actor. The column
-- is display-only metadata, so replace any non-system value with a safe label.
UPDATE platform_settings
SET updated_by = 'organizer'
WHERE TRIM(updated_by) <> ''
  AND updated_by <> 'system';
