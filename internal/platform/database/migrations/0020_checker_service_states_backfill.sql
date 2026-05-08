INSERT INTO checker_service_states (
    tick_id,
    team_id,
    challenge_id,
    service_state,
    state_phase,
    state_message,
    updated_at
)
WITH grouped AS (
    SELECT
        tick_id,
        team_id,
        challenge_id,
        MAX(CASE WHEN phase = 'put' AND status = 'success' THEN 1 ELSE 0 END) AS put_ok,
        MAX(CASE WHEN phase = 'get' AND status = 'success' THEN 1 ELSE 0 END) AS get_ok,
        MAX(CASE WHEN phase = 'check' AND status = 'success' THEN 1 ELSE 0 END) AS check_ok,
        MAX(CASE WHEN phase = 'put' AND status <> 'success' THEN 1 ELSE 0 END) AS put_failed,
        MAX(CASE WHEN phase = 'get' AND status <> 'success' THEN 1 ELSE 0 END) AS get_failed,
        MAX(CASE WHEN phase = 'check' AND status <> 'success' THEN 1 ELSE 0 END) AS check_failed,
        MAX(CASE WHEN phase = 'put' THEN NULLIF(message, '') END) AS put_message,
        MAX(CASE WHEN phase = 'get' THEN NULLIF(message, '') END) AS get_message,
        MAX(CASE WHEN phase = 'check' THEN NULLIF(message, '') END) AS check_message,
        MAX(checked_at) AS updated_at
    FROM checker_runs
    GROUP BY tick_id, team_id, challenge_id
)
SELECT
    tick_id,
    team_id,
    challenge_id,
    CASE
        WHEN put_ok = 1 AND get_ok = 1 AND check_ok = 1 THEN 'ok'
        WHEN put_ok = 0 AND get_ok = 1 AND check_ok = 1 THEN 'recovering'
        WHEN put_ok = 1 AND get_ok = 0 THEN 'flag_not_found'
        WHEN put_ok = 1 AND get_ok = 1 AND check_ok = 0 THEN 'faulty'
        ELSE 'down'
    END AS service_state,
    CASE
        WHEN put_ok = 1 AND get_ok = 1 AND check_ok = 1 THEN 'check'
        WHEN put_ok = 0 AND get_ok = 1 AND check_ok = 1 THEN 'put'
        WHEN put_ok = 1 AND get_ok = 0 THEN 'get'
        WHEN put_ok = 1 AND get_ok = 1 AND check_ok = 0 THEN 'check'
        WHEN put_failed = 1 THEN 'put'
        WHEN get_failed = 1 THEN 'get'
        WHEN check_failed = 1 THEN 'check'
        ELSE ''
    END AS state_phase,
    CASE
        WHEN put_ok = 1 AND get_ok = 1 AND check_ok = 1 THEN 'service passed storage, retrieval, and functionality checks'
        WHEN put_ok = 0 AND get_ok = 1 AND check_ok = 1 THEN COALESCE(put_message, 'flag storage failed but retrieval and functionality still passed')
        WHEN put_ok = 1 AND get_ok = 0 THEN COALESCE(get_message, 'checker could not retrieve the stored flag')
        WHEN put_ok = 1 AND get_ok = 1 AND check_ok = 0 THEN COALESCE(check_message, 'service functionality check failed')
        WHEN put_failed = 1 THEN COALESCE(put_message, 'service did not complete the latest checker cycle')
        WHEN get_failed = 1 THEN COALESCE(get_message, 'service did not complete the latest checker cycle')
        WHEN check_failed = 1 THEN COALESCE(check_message, 'service did not complete the latest checker cycle')
        ELSE 'service did not complete the latest checker cycle'
    END AS state_message,
    updated_at
FROM grouped
ON CONFLICT (tick_id, team_id, challenge_id) DO NOTHING;
