-- Team IDs double as service-network host slots (101..344), so a monotonic
-- sequence would exhaust a lifetime counter even after teams were deleted.
-- Allocation is serialized and selected from currently unused IDs in the API.
ALTER TABLE teams ALTER COLUMN id DROP DEFAULT;
DROP SEQUENCE IF EXISTS teams_id_seq;
