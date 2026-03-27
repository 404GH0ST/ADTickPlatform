UPDATE team_service_states
SET ssh_hint = CASE
    WHEN unlocked THEN 'ssh root@' || split_part(endpoint, ':', 1)
    ELSE ssh_hint
END;
