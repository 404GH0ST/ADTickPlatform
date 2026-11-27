ALTER TABLE checker_runs
    ADD COLUMN IF NOT EXISTS retention_check BOOLEAN NOT NULL DEFAULT FALSE;

-- Earlier retention observations used fixed messages without a separate type.
-- Mark them while keeping the original checker results and deduction records.
UPDATE checker_runs
SET retention_check = TRUE
WHERE phase = 'get'
  AND (
    message IN (
      'flag retention checker execution unavailable',
      'stored flag failed retention verification',
      'stored flag passed retention verification'
    )
    OR message LIKE 'confirmed missing stored flag; tick SLA forfeited and %'
  )
  AND NOT retention_check;

-- A legacy delayed GET could replace the original service verdict with DOWN.
-- Restore credit only where all three ordinary tick phases passed. Confirmed
-- losses and ticks with an ordinary phase failure keep their prior scoring.
WITH passed_ticks AS (
    SELECT s.tick_id, s.team_id, s.challenge_id
    FROM checker_service_states s
    JOIN flag_retention_failures f USING (tick_id, team_id, challenge_id)
    JOIN checker_runs r USING (tick_id, team_id, challenge_id)
    WHERE NOT f.confirmed_missing
      AND s.service_state = 'down'
      AND s.state_phase = 'get'
      AND s.state_message = 'stored flag failed retention verification'
      AND NOT r.retention_check
      AND r.phase IN ('put', 'get', 'check')
    GROUP BY s.tick_id, s.team_id, s.challenge_id
    HAVING COUNT(DISTINCT r.phase) = 3 AND BOOL_AND(r.status = 'success')
)
UPDATE checker_service_states s
SET service_state = 'ok',
    state_phase = 'check',
    state_message = 'service passed storage, retrieval, and functionality checks',
    updated_at = NOW()
FROM passed_ticks p
WHERE (s.tick_id, s.team_id, s.challenge_id) = (p.tick_id, p.team_id, p.challenge_id);
