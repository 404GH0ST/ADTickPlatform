package apigateway

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type postgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) Store {
	return &postgresStore{db: db}
}

func (s *postgresStore) beginTx(ctx context.Context) (*sql.Tx, error) {
	var (
		tx  *sql.Tx
		err error
	)
	for range 2 {
		tx, err = s.db.BeginTx(ctx, nil)
		if err == nil {
			return tx, nil
		}
		if !isBadConnectionError(err) {
			return nil, err
		}
	}
	return nil, err
}

func (s *postgresStore) queryRowScan(ctx context.Context, query string, args []any, dest ...any) error {
	var err error
	for range 2 {
		err = s.db.QueryRowContext(ctx, query, args...).Scan(dest...)
		if err == nil || !isBadConnectionError(err) {
			return err
		}
	}
	return err
}

func retryBadConnection[T any](fn func() (T, error)) (T, error) {
	var (
		value T
		err   error
	)
	for range 2 {
		value, err = fn()
		if err == nil || !isBadConnectionError(err) {
			return value, err
		}
	}
	return value, err
}

func isBadConnectionError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, driver.ErrBadConn) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "bad connection")
}

func (s *postgresStore) AuthenticatePlayer(ctx context.Context, email, password string) (authenticatedPlayer, error) {
	var player authenticatedPlayer
	var passwordHash string
	var teamID sql.NullInt64
	var teamName sql.NullString
	if err := s.db.QueryRowContext(ctx, `
		SELECT p.id, p.team_id, t.name, p.display_name, p.email, p.role, p.password_hash
		FROM players p
		LEFT JOIN teams t ON t.id = p.team_id
		WHERE LOWER(p.email) = LOWER($1)
	`, strings.TrimSpace(strings.ToLower(email))).Scan(
		&player.PlayerID,
		&teamID,
		&teamName,
		&player.DisplayName,
		&player.Email,
		&player.Role,
		&passwordHash,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return authenticatedPlayer{}, ErrInvalidCredentials
		}
		return authenticatedPlayer{}, err
	}
	if teamID.Valid {
		player.TeamID = int(teamID.Int64)
	}
	if teamName.Valid {
		player.TeamName = teamName.String
	} else if strings.EqualFold(strings.TrimSpace(player.Role), "organizer") {
		player.TeamName = "Organizer"
	}

	if !passwordMatches(passwordHash, password) {
		return authenticatedPlayer{}, ErrInvalidCredentials
	}
	if passwordHashNeedsUpgrade(passwordHash) {
		if upgradedHash, hashErr := hashPassword(password); hashErr == nil {
			_, _ = s.db.ExecContext(ctx, `UPDATE players SET password_hash = $2 WHERE id = $1`, player.PlayerID, upgradedHash)
		}
	}
	return player, nil
}

func (s *postgresStore) RegisterPlayer(ctx context.Context, input participantRegisterRequest, now time.Time) (authenticatedPlayer, error) {
	displayName := strings.TrimSpace(input.DisplayName)
	email := strings.TrimSpace(strings.ToLower(input.Email))
	password := input.Password
	if displayName == "" || email == "" || strings.TrimSpace(password) == "" {
		return authenticatedPlayer{}, ErrDuplicateResource
	}

	tx, err := s.beginTx(ctx)
	if err != nil {
		return authenticatedPlayer{}, err
	}
	defer tx.Rollback()

	var duplicate bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM players WHERE LOWER(email) = LOWER($1))`, email).Scan(&duplicate); err != nil {
		return authenticatedPlayer{}, err
	}
	if duplicate {
		return authenticatedPlayer{}, ErrDuplicateResource
	}

	playerID, err := s.nextIDTx(ctx, tx, `SELECT COALESCE(MAX(id), 0) + 1 FROM players`)
	if err != nil {
		return authenticatedPlayer{}, err
	}
	passwordHash, err := hashPassword(password)
	if err != nil {
		return authenticatedPlayer{}, err
	}
	wireGuardPeer := wireguardPeerName(0, playerID)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO players (id, team_id, display_name, email, password_hash, role, wireguard_peer, created_at)
		VALUES ($1, NULL, $2, $3, $4, 'member', $5, $6)
	`, playerID, displayName, email, passwordHash, wireGuardPeer, now.UTC()); err != nil {
		return authenticatedPlayer{}, err
	}
	if err := tx.Commit(); err != nil {
		return authenticatedPlayer{}, err
	}
	return authenticatedPlayer{
		PlayerID:    playerID,
		TeamID:      0,
		TeamName:    "",
		DisplayName: displayName,
		Email:       email,
		Role:        "member",
	}, nil
}

func (s *postgresStore) ValidatePlayerSession(ctx context.Context, playerID, teamID int, role string) (authenticatedPlayer, error) {
	var player authenticatedPlayer
	var currentTeamID sql.NullInt64
	var currentTeamName sql.NullString
	if err := s.db.QueryRowContext(ctx, `
		SELECT p.id, p.team_id, t.name, p.display_name, p.email, p.role
		FROM players p
		LEFT JOIN teams t ON t.id = p.team_id
		WHERE p.id = $1
	`, playerID).Scan(&player.PlayerID, &currentTeamID, &currentTeamName, &player.DisplayName, &player.Email, &player.Role); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return authenticatedPlayer{}, ErrInvalidCredentials
		}
		return authenticatedPlayer{}, err
	}
	if strings.TrimSpace(player.Role) != strings.TrimSpace(role) {
		return authenticatedPlayer{}, ErrInvalidCredentials
	}

	normalizedRole := strings.TrimSpace(strings.ToLower(player.Role))
	if normalizedRole == "organizer" {
		if teamID != 0 {
			return authenticatedPlayer{}, ErrInvalidCredentials
		}
		player.TeamID = 0
		player.TeamName = "Organizer"
		return player, nil
	}

	if !currentTeamID.Valid {
		if teamID != 0 {
			return authenticatedPlayer{}, ErrInvalidCredentials
		}
		player.TeamID = 0
		player.TeamName = ""
		return player, nil
	}
	if int(currentTeamID.Int64) != teamID {
		return authenticatedPlayer{}, ErrInvalidCredentials
	}
	player.TeamID = teamID
	if currentTeamName.Valid {
		player.TeamName = currentTeamName.String
	}
	return player, nil
}

func (s *postgresStore) ListChallenges(ctx context.Context) ([]challenge, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, COALESCE(source_bundle_path, '') <> '' FROM challenges WHERE published = TRUE ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]challenge, 0)
	for rows.Next() {
		var item challenge
		if err := rows.Scan(&item.ID, &item.Name, &item.HasSourceDownload); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *postgresStore) ListPublicServices(ctx context.Context) (map[string]map[string][]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT tss.challenge_id, tss.team_id, tss.endpoint
		FROM team_service_states tss
		JOIN challenges c ON c.id = tss.challenge_id
		WHERE c.published = TRUE
		ORDER BY tss.challenge_id, tss.team_id, tss.endpoint
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	services := make(map[string]map[string][]string)
	for rows.Next() {
		var challengeID int
		var teamID int
		var endpoint string
		if err := rows.Scan(&challengeID, &teamID, &endpoint); err != nil {
			return nil, err
		}
		challengeKey := fmt.Sprintf("%d", challengeID)
		if _, ok := services[challengeKey]; !ok {
			services[challengeKey] = make(map[string][]string)
		}
		services[challengeKey][fmt.Sprintf("%d", teamID)] = []string{endpoint}
	}
	return services, rows.Err()
}

func (s *postgresStore) ListScoreboard(ctx context.Context) ([]scoreRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT rank, team_name, attack_points, defense_points, sla_points, total_points, delta
		FROM scoreboard_entries
		ORDER BY rank
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]scoreRow, 0)
	for rows.Next() {
		var row scoreRow
		if err := rows.Scan(&row.Rank, &row.Team, &row.Attack, &row.Defense, &row.SLA, &row.Total, &row.Delta); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (s *postgresStore) ListAttackFeed(ctx context.Context) ([]attackEvent, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, attacker, victim, service, tick, verdict, created_at
		FROM attack_events
		ORDER BY created_at DESC, id DESC
		LIMIT 12
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]attackEvent, 0)
	for rows.Next() {
		var event attackEvent
		if err := rows.Scan(&event.ID, &event.Attacker, &event.Victim, &event.Service, &event.Tick, &event.Verdict, &event.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, event)
	}
	return result, rows.Err()
}

func (s *postgresStore) ListTeamServices(ctx context.Context, teamID int) ([]serviceState, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT tss.challenge_id, tss.team_id, c.name, tss.endpoint, tss.status, tss.checker, tss.unlocked, tss.ssh_hint, tss.last_event, tss.reset_cooldown
		FROM team_service_states tss
		JOIN challenges c ON c.id = tss.challenge_id
		WHERE tss.team_id = $1 AND c.published = TRUE
		ORDER BY tss.challenge_id
	`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]serviceState, 0)
	for rows.Next() {
		var state serviceState
		if err := rows.Scan(&state.ChallengeID, &state.TeamID, &state.Name, &state.Endpoint, &state.Status, &state.Checker, &state.Unlocked, &state.SSHHint, &state.LastEvent, &state.ResetCooldown); err != nil {
			return nil, err
		}
		result = append(result, state)
	}
	return result, rows.Err()
}

func (s *postgresStore) SubmitFlags(ctx context.Context, teamID int, flags []string) ([]submissionVerdict, error) {
	return nil, ErrSubmissionUnavailable
}

func (s *postgresStore) ValidateServiceAction(ctx context.Context, teamID, challengeID int) error {
	var published bool
	var runtimeStatus sql.NullString
	if err := s.db.QueryRowContext(ctx, `
		SELECT c.published, si.runtime_status
		FROM team_service_states tss
		JOIN challenges c ON c.id = tss.challenge_id
		LEFT JOIN service_instances si ON si.team_id = tss.team_id AND si.challenge_id = tss.challenge_id
		WHERE tss.team_id = $1 AND tss.challenge_id = $2
	`, teamID, challengeID).Scan(&published, &runtimeStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrChallengeNotFound
		}
		return err
	}
	if !published {
		return ErrChallengeNotFound
	}
	if !runtimeStatus.Valid {
		return ErrChallengeNotFound
	}
	if strings.TrimSpace(runtimeStatus.String) != "ready" {
		return ErrServiceUnavailable
	}
	return nil
}

func (s *postgresStore) UnlockService(ctx context.Context, teamID, challengeID int) (unlockData, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE team_service_states
		SET unlocked = TRUE,
		    status = CASE WHEN status = 'degraded' THEN 'warming' ELSE status END,
		    ssh_hint = 'unlock accepted; use SSH Access to view the team credential',
		    last_event = 'unlock granted via participant API'
		WHERE team_id = $1 AND challenge_id = $2
		  AND EXISTS (
			SELECT 1
			FROM challenges c
			JOIN service_instances si ON si.team_id = team_service_states.team_id AND si.challenge_id = team_service_states.challenge_id
			WHERE c.id = team_service_states.challenge_id
			  AND c.published = TRUE
			  AND si.runtime_status = 'ready'
		  )
	`, teamID, challengeID)
	if err != nil {
		return unlockData{}, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return unlockData{}, ErrChallengeNotFound
	}
	return unlockData{ChallengeID: challengeID, TeamID: teamID, Unlocked: true}, nil
}

func (s *postgresStore) CreateSSHSession(ctx context.Context, teamID, challengeID int, _ time.Time) (sshSessionData, error) {
	tx, err := s.beginTx(ctx)
	if err != nil {
		return sshSessionData{}, err
	}
	defer tx.Rollback()

	var unlocked bool
	var endpoint string
	if err := tx.QueryRowContext(ctx, `
		SELECT unlocked, endpoint
		FROM team_service_states
		WHERE team_id = $1 AND challenge_id = $2
		  AND EXISTS (
			SELECT 1
			FROM challenges c
			JOIN service_instances si ON si.team_id = team_service_states.team_id AND si.challenge_id = team_service_states.challenge_id
			WHERE c.id = team_service_states.challenge_id
			  AND c.published = TRUE
			  AND si.runtime_status = 'ready'
		  )
		FOR UPDATE
	`, teamID, challengeID).Scan(&unlocked, &endpoint); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sshSessionData{}, ErrChallengeNotFound
		}
		return sshSessionData{}, err
	}
	if !unlocked {
		return sshSessionData{}, ErrServiceLocked
	}

	host, _ := ParseEndpoint(endpoint)
	connectionHint := fmt.Sprintf("ssh %s@%s", sshUsername(), host)
	if _, err := tx.ExecContext(ctx, `
		UPDATE team_service_states
		SET ssh_hint = $3,
		    last_event = 'team ssh credential retrieved'
		WHERE team_id = $1 AND challenge_id = $2
	`, teamID, challengeID, connectionHint); err != nil {
		return sshSessionData{}, err
	}
	if err := tx.Commit(); err != nil {
		return sshSessionData{}, err
	}

	return sshSessionData{ChallengeID: challengeID, Host: host, Port: 22, Username: sshUsername(), PasswordMode: "stable", ConnectionHint: connectionHint}, nil
}

func (s *postgresStore) MarkSSHSessionApplyFailure(ctx context.Context, teamID, challengeID int) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE team_service_states
		SET ssh_hint = 'credential apply failed; open SSH Access to retry applying the team credential',
		    last_event = 'ssh credential apply failed'
		WHERE team_id = $1 AND challenge_id = $2
	`, teamID, challengeID)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return ErrChallengeNotFound
	}
	return nil
}

func (s *postgresStore) PrepareFactoryResetService(ctx context.Context, teamID, challengeID int) (resetData, error) {
	checkerToken, err := newCheckerToken()
	if err != nil {
		return resetData{}, err
	}
	var unlocked bool
	if err := s.db.QueryRowContext(ctx, `
		WITH updated_instance AS (
			UPDATE service_instances si
			SET checker_token = $3,
			    updated_at = NOW()
			FROM challenges c
			WHERE si.team_id = $1
			  AND si.challenge_id = $2
			  AND c.id = si.challenge_id
			  AND c.published = TRUE
			  AND si.runtime_status = 'ready'
			RETURNING si.team_id, si.challenge_id
		)
		UPDATE team_service_states
		SET status = 'resetting',
		    checker = 'warning',
		    last_event = 'factory reset started via participant API',
		    reset_cooldown = 'resetting'
		WHERE team_id = $1 AND challenge_id = $2
		  AND EXISTS (
			SELECT 1
			FROM updated_instance ui
			WHERE ui.team_id = team_service_states.team_id
			  AND ui.challenge_id = team_service_states.challenge_id
		  )
		RETURNING unlocked
	`, teamID, challengeID, checkerToken).Scan(&unlocked); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return resetData{}, ErrChallengeNotFound
		}
		return resetData{}, err
	}
	return resetData{ChallengeID: challengeID, TeamID: teamID, Action: "factory_reset", UnlockPreserved: unlocked}, nil
}

func (s *postgresStore) CompleteFactoryResetService(ctx context.Context, teamID, challengeID int) (resetData, error) {
	var unlocked bool
	if err := s.db.QueryRowContext(ctx, `
		UPDATE team_service_states
		SET status = 'warming',
		    checker = 'warning',
		    last_event = 'factory reset completed; waiting for checker verification',
		    reset_cooldown = 'cooldown: 90s',
		    ssh_hint = CASE WHEN unlocked THEN 'unlock preserved; open SSH Access to reapply the team credential' ELSE ssh_hint END
		WHERE team_id = $1 AND challenge_id = $2
		RETURNING unlocked
	`, teamID, challengeID).Scan(&unlocked); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return resetData{}, ErrChallengeNotFound
		}
		return resetData{}, err
	}
	return resetData{ChallengeID: challengeID, TeamID: teamID, Action: "factory_reset", UnlockPreserved: unlocked}, nil
}

func (s *postgresStore) MarkFactoryResetFailure(ctx context.Context, teamID, challengeID int) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE team_service_states
		SET status = 'degraded',
		    checker = 'warning',
		    last_event = 'factory reset runtime failed',
		    reset_cooldown = 'ready'
		WHERE team_id = $1 AND challenge_id = $2
	`, teamID, challengeID)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return ErrChallengeNotFound
	}
	return nil
}

func (s *postgresStore) RestartService(ctx context.Context, teamID, challengeID int) (resetData, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE team_service_states
		SET status = 'warming',
		    checker = 'warning',
		    last_event = 'service restart completed; waiting for checker verification',
		    reset_cooldown = 'cooldown: 30s'
		WHERE team_id = $1 AND challenge_id = $2
		  AND EXISTS (
			SELECT 1
			FROM challenges c
			JOIN service_instances si ON si.team_id = team_service_states.team_id AND si.challenge_id = team_service_states.challenge_id
			WHERE c.id = team_service_states.challenge_id
			  AND c.published = TRUE
			  AND si.runtime_status = 'ready'
		  )
	`, teamID, challengeID)
	if err != nil {
		return resetData{}, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return resetData{}, ErrChallengeNotFound
	}
	return resetData{ChallengeID: challengeID, TeamID: teamID, Action: "restart"}, nil
}

func (s *postgresStore) ListAdminTeams(ctx context.Context) ([]adminTeam, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.id, t.name, t.email, COALESCE(t.join_key, ''), COUNT(DISTINCT p.id) AS player_count, COUNT(DISTINCT tss.challenge_id) AS deployed_challenges
		FROM teams t
		LEFT JOIN players p ON p.team_id = t.id
		LEFT JOIN team_service_states tss ON tss.team_id = t.id
		GROUP BY t.id, t.name, t.email, t.join_key
		ORDER BY t.id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]adminTeam, 0)
	for rows.Next() {
		var team adminTeam
		if err := rows.Scan(&team.ID, &team.Name, &team.ContactEmail, &team.JoinKey, &team.PlayerCount, &team.DeployedChallenges); err != nil {
			return nil, err
		}
		result = append(result, team)
	}
	return result, rows.Err()
}

func (s *postgresStore) CreateAdminTeam(ctx context.Context, input adminCreateTeamRequest) (adminTeam, error) {
	return retryBadConnection(func() (adminTeam, error) {
		name := strings.TrimSpace(input.Name)
		contactEmail := strings.TrimSpace(strings.ToLower(input.ContactEmail))
		if name == "" || contactEmail == "" {
			return adminTeam{}, ErrDuplicateResource
		}

		duplicate, err := s.exists(ctx, `SELECT EXISTS(SELECT 1 FROM teams WHERE LOWER(name) = LOWER($1) OR LOWER(email) = LOWER($2))`, name, contactEmail)
		if err != nil {
			return adminTeam{}, fmt.Errorf("check team duplicate: %w", err)
		}
		if duplicate {
			return adminTeam{}, ErrDuplicateResource
		}

		tx, err := s.beginTx(ctx)
		if err != nil {
			return adminTeam{}, fmt.Errorf("begin create team tx: %w", err)
		}
		defer tx.Rollback()

		teamID, err := s.nextIDTx(ctx, tx, `SELECT COALESCE(MAX(id), 100) + 1 FROM teams`)
		if err != nil {
			return adminTeam{}, fmt.Errorf("allocate team id: %w", err)
		}
		joinKey, err := NewTeamJoinKey()
		if err != nil {
			return adminTeam{}, fmt.Errorf("generate team join key: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO teams (id, name, email, join_key) VALUES ($1, $2, $3, $4)`, teamID, name, contactEmail, joinKey); err != nil {
			return adminTeam{}, fmt.Errorf("insert team: %w", err)
		}

		rank, err := s.nextIDTx(ctx, tx, `SELECT COALESCE(MAX(rank), 0) + 1 FROM scoreboard_entries`)
		if err != nil {
			return adminTeam{}, fmt.Errorf("allocate scoreboard rank: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO scoreboard_entries (team_id, rank, team_name, attack_points, defense_points, sla_points, total_points, delta)
			VALUES ($1, $2, $3, 0, 0, 0, 0, 'new')
		`, teamID, rank, name); err != nil {
			return adminTeam{}, fmt.Errorf("insert scoreboard row: %w", err)
		}

		challengeRows, err := tx.QueryContext(ctx, `
			SELECT id,
			       name,
			       COALESCE(baseline_image, ''),
			       COALESCE(checker_image, ''),
			       COALESCE(NULLIF(service_port, 0), 10000 + id),
			       COALESCE(NULLIF(service_subnet_octet, 0), id)
			FROM challenges
			WHERE published = TRUE
			ORDER BY id
		`)
		if err != nil {
			return adminTeam{}, fmt.Errorf("query published challenges for new team: %w", err)
		}
		type publishedChallengeConfig struct {
			ID                 int
			Name               string
			BaselineImage      string
			CheckerImage       string
			ServicePort        int
			ServiceSubnetOctet int
		}
		challenges := make([]publishedChallengeConfig, 0)
		for challengeRows.Next() {
			var challengeConfig publishedChallengeConfig
			if err := challengeRows.Scan(
				&challengeConfig.ID,
				&challengeConfig.Name,
				&challengeConfig.BaselineImage,
				&challengeConfig.CheckerImage,
				&challengeConfig.ServicePort,
				&challengeConfig.ServiceSubnetOctet,
			); err != nil {
				challengeRows.Close()
				return adminTeam{}, fmt.Errorf("scan published challenge for new team: %w", err)
			}
			challenges = append(challenges, challengeConfig)
		}
		if err := challengeRows.Err(); err != nil {
			challengeRows.Close()
			return adminTeam{}, fmt.Errorf("iterate published challenges for new team: %w", err)
		}
		challengeRows.Close()

		deployedChallenges := 0
		for _, challengeConfig := range challenges {
			checkerToken, err := newCheckerToken()
			if err != nil {
				return adminTeam{}, err
			}
			state := defaultServiceStateForConfig(challengeConfig.ID, teamID, challengeConfig.Name, challengeConfig.ServicePort, challengeConfig.ServiceSubnetOctet)
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO team_service_states (team_id, challenge_id, endpoint, status, checker, unlocked, ssh_hint, last_event, reset_cooldown)
				VALUES ($1, $2, $3, 'provisioning', 'pending', FALSE, 'deployment queued; wait for controller reconciliation before unlock', 'baseline deployment queued by team join', 'deploying')
			`, teamID, challengeConfig.ID, state.Endpoint); err != nil {
				return adminTeam{}, fmt.Errorf("insert team service state for challenge %d: %w", challengeConfig.ID, err)
			}
			if _, err := tx.ExecContext(ctx, `
					INSERT INTO service_instances (team_id, challenge_id, deployment_job_id, runtime_kind, runtime_status, container_name, state_volume, baseline_image, checker_image, checker_token, endpoint, ssh_host, created_at, updated_at)
					VALUES ($1, $2, NULL, 'docker', 'queued', $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
				`, teamID, challengeConfig.ID, serviceContainerName(challengeConfig.Name, teamID), serviceStateVolumeName(challengeConfig.Name, teamID), challengeConfig.BaselineImage, challengeConfig.CheckerImage, checkerToken, state.Endpoint, ServiceIP(challengeConfig.ServiceSubnetOctet, teamID)); err != nil {
				return adminTeam{}, fmt.Errorf("insert service instance for challenge %d: %w", challengeConfig.ID, err)
			}
			deployedChallenges++
		}
		if err := tx.Commit(); err != nil {
			return adminTeam{}, fmt.Errorf("commit create team tx: %w", err)
		}

		return adminTeam{ID: teamID, Name: name, ContactEmail: contactEmail, JoinKey: joinKey, PlayerCount: 0, DeployedChallenges: deployedChallenges}, nil
	})
}

func (s *postgresStore) ListAdminPlayers(ctx context.Context) ([]adminPlayer, error) {
	if err := s.ensureAllWireGuardPeers(ctx, time.Now().UTC()); err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT p.id, p.team_id, t.name, p.display_name, p.email, p.role, p.wireguard_peer, p.created_at,
		       wp.address, wp.status, wp.issued_at, wp.revoked_at
		FROM players p
		LEFT JOIN teams t ON t.id = p.team_id
		LEFT JOIN wireguard_peers wp ON wp.player_id = p.id
		ORDER BY p.id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]adminPlayer, 0)
	for rows.Next() {
		var player adminPlayer
		var teamID sql.NullInt64
		var teamName sql.NullString
		var wgAddress sql.NullString
		var wgStatus sql.NullString
		var createdAt time.Time
		var issuedAt sql.NullTime
		var revokedAt sql.NullTime
		if err := rows.Scan(
			&player.ID,
			&teamID,
			&teamName,
			&player.DisplayName,
			&player.Email,
			&player.Role,
			&player.WireGuardPeer,
			&createdAt,
			&wgAddress,
			&wgStatus,
			&issuedAt,
			&revokedAt,
		); err != nil {
			return nil, err
		}
		if teamID.Valid {
			player.TeamID = int(teamID.Int64)
		}
		if teamName.Valid {
			player.TeamName = teamName.String
		} else {
			player.TeamName = "Organizer"
		}
		if wgAddress.Valid {
			player.WireGuardAddress = wgAddress.String
		}
		if wgStatus.Valid {
			player.WireGuardStatus = wgStatus.String
		}
		player.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		if issuedAt.Valid {
			player.WireGuardIssuedAt = issuedAt.Time.UTC().Format(time.RFC3339)
		}
		if revokedAt.Valid {
			player.WireGuardRevokedAt = revokedAt.Time.UTC().Format(time.RFC3339)
		}
		result = append(result, player)
	}
	return result, rows.Err()
}

func (s *postgresStore) CreateAdminPlayer(ctx context.Context, input adminCreatePlayerRequest, now time.Time) (adminPlayer, error) {
	var teamName string
	if input.TeamID == 0 {
		teamName = "Organizer"
	} else {
		var err error
		teamName, err = s.lookupTeamName(ctx, input.TeamID)
		if err != nil {
			return adminPlayer{}, err
		}
	}
	email := strings.TrimSpace(strings.ToLower(input.Email))
	duplicate, err := s.exists(ctx, `SELECT EXISTS(SELECT 1 FROM players WHERE LOWER(email) = LOWER($1))`, email)
	if err != nil {
		return adminPlayer{}, err
	}
	if duplicate {
		return adminPlayer{}, ErrDuplicateResource
	}

	tx, err := s.beginTx(ctx)
	if err != nil {
		return adminPlayer{}, err
	}
	defer tx.Rollback()

	if input.TeamID > 0 {
		limited, err := teamMemberLimitReachedTx(ctx, tx, input.TeamID)
		if err != nil {
			return adminPlayer{}, err
		}
		if limited {
			return adminPlayer{}, ErrTeamMemberLimit
		}
	}

	playerID, err := s.nextIDTx(ctx, tx, `SELECT COALESCE(MAX(id), 0) + 1 FROM players`)
	if err != nil {
		return adminPlayer{}, err
	}
	player := adminPlayer{
		ID:            playerID,
		TeamID:        input.TeamID,
		TeamName:      teamName,
		DisplayName:   strings.TrimSpace(input.DisplayName),
		Email:         email,
		Role:          normalizedRole(input.Role),
		WireGuardPeer: wireguardPeerName(input.TeamID, playerID),
		CreatedAt:     now.UTC().Format(time.RFC3339),
	}
	var dbTeamID sql.NullInt64
	if player.TeamID > 0 {
		dbTeamID = sql.NullInt64{Int64: int64(player.TeamID), Valid: true}
	}
	passwordHash, err := hashPassword(input.Password)
	if err != nil {
		return adminPlayer{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO players (id, team_id, display_name, email, password_hash, role, wireguard_peer, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, player.ID, dbTeamID, player.DisplayName, player.Email, passwordHash, player.Role, player.WireGuardPeer, now.UTC()); err != nil {
		return adminPlayer{}, err
	}
	wireGuardState, err := newWireGuardPeerState(player.ID, player.TeamID, player.TeamName, player.DisplayName, player.WireGuardPeer, now)
	if err != nil {
		return adminPlayer{}, err
	}
	if err := insertWireGuardPeerTx(ctx, tx, wireGuardState); err != nil {
		return adminPlayer{}, err
	}
	player.WireGuardAddress = wireGuardState.Address
	player.WireGuardStatus = wireGuardState.Status
	player.WireGuardIssuedAt = wireGuardState.IssuedAt
	if err := tx.Commit(); err != nil {
		return adminPlayer{}, err
	}
	return player, nil
}

func (s *postgresStore) JoinExistingPlayerTeam(ctx context.Context, playerID int, teamKey string, now time.Time) (authenticatedPlayer, error) {
	key := strings.TrimSpace(teamKey)
	if key == "" {
		return authenticatedPlayer{}, ErrInvalidCredentials
	}

	tx, err := s.beginTx(ctx)
	if err != nil {
		return authenticatedPlayer{}, err
	}
	defer tx.Rollback()

	var player authenticatedPlayer
	var currentTeamID sql.NullInt64
	if err := tx.QueryRowContext(ctx, `
		SELECT id, team_id, display_name, email, role
		FROM players
		WHERE id = $1
		FOR UPDATE
	`, playerID).Scan(
		&player.PlayerID,
		&currentTeamID,
		&player.DisplayName,
		&player.Email,
		&player.Role,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return authenticatedPlayer{}, ErrInvalidCredentials
		}
		return authenticatedPlayer{}, err
	}
	if strings.EqualFold(strings.TrimSpace(player.Role), "organizer") || (currentTeamID.Valid && currentTeamID.Int64 > 0) {
		return authenticatedPlayer{}, ErrDuplicateResource
	}

	var teamID int
	var teamName string
	if err := tx.QueryRowContext(ctx, `
		SELECT id, name
		FROM teams
		WHERE join_key = $1
	`, key).Scan(&teamID, &teamName); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return authenticatedPlayer{}, ErrInvalidCredentials
		}
		return authenticatedPlayer{}, err
	}
	limited, err := teamMemberLimitReachedTx(ctx, tx, teamID)
	if err != nil {
		return authenticatedPlayer{}, err
	}
	if limited {
		return authenticatedPlayer{}, ErrTeamMemberLimit
	}

	wireGuardPeer := wireguardPeerName(teamID, playerID)
	if _, err := tx.ExecContext(ctx, `
		UPDATE players
		SET team_id = $2, wireguard_peer = $3
		WHERE id = $1
	`, playerID, teamID, wireGuardPeer); err != nil {
		return authenticatedPlayer{}, err
	}

	wireGuardState, err := newWireGuardPeerState(playerID, teamID, teamName, player.DisplayName, wireGuardPeer, now)
	if err != nil {
		return authenticatedPlayer{}, err
	}
	if err := upsertWireGuardPeerTx(ctx, tx, wireGuardState); err != nil {
		return authenticatedPlayer{}, err
	}
	if err := tx.Commit(); err != nil {
		return authenticatedPlayer{}, err
	}

	player.TeamID = teamID
	player.TeamName = teamName
	return player, nil
}

func teamMemberLimitReachedTx(ctx context.Context, tx *sql.Tx, teamID int) (bool, error) {
	if teamID <= 0 {
		return false, nil
	}
	var limit int
	if err := tx.QueryRowContext(ctx, `
		SELECT max_team_members
		FROM platform_settings
		WHERE id = 1
	`).Scan(&limit); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	if limit <= 0 {
		return false, nil
	}
	var count int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM players
		WHERE team_id = $1
	`, teamID).Scan(&count); err != nil {
		return false, err
	}
	return count >= limit, nil
}

func (s *postgresStore) GetAdminPlayerWireGuardConfig(ctx context.Context, playerID int) (adminWireGuardPeer, error) {
	if err := s.ensureWireGuardPeer(ctx, playerID, time.Now().UTC()); err != nil {
		return adminWireGuardPeer{}, err
	}
	return s.loadAdminWireGuardPeer(ctx, playerID)
}

func (s *postgresStore) RotateAdminPlayerWireGuardConfig(ctx context.Context, playerID int, now time.Time) (adminWireGuardPeer, error) {
	tx, err := s.beginTx(ctx)
	if err != nil {
		return adminWireGuardPeer{}, err
	}
	defer tx.Rollback()

	identity, err := loadWireGuardProvisionIdentityTx(ctx, tx, playerID)
	if err != nil {
		return adminWireGuardPeer{}, err
	}
	wireGuardState, err := newWireGuardPeerState(identity.PlayerID, identity.TeamID, identity.TeamName, identity.DisplayName, identity.WireGuardPeer, now)
	if err != nil {
		return adminWireGuardPeer{}, err
	}
	if err := upsertWireGuardPeerTx(ctx, tx, wireGuardState); err != nil {
		return adminWireGuardPeer{}, err
	}
	if err := tx.Commit(); err != nil {
		return adminWireGuardPeer{}, err
	}
	return wireGuardAdminView(wireGuardState, identity.Email), nil
}

func (s *postgresStore) RevokeAdminPlayerWireGuardConfig(ctx context.Context, playerID int, now time.Time) (adminWireGuardPeer, error) {
	if err := s.ensureWireGuardPeer(ctx, playerID, now); err != nil {
		return adminWireGuardPeer{}, err
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE wireguard_peers
		SET status = 'revoked',
		    revoked_at = $2,
		    updated_at = $2
		WHERE player_id = $1
	`, playerID, now.UTC())
	if err != nil {
		return adminWireGuardPeer{}, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return adminWireGuardPeer{}, ErrPlayerNotFound
	}
	return s.loadAdminWireGuardPeer(ctx, playerID)
}

func (s *postgresStore) ListWireGuardGatewayPeers(ctx context.Context) ([]WireGuardGatewayPeer, error) {
	if err := s.ensureAllWireGuardPeers(ctx, time.Now().UTC()); err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT p.id, p.team_id, t.name, p.display_name, p.wireguard_peer,
		       wp.address, wp.status, wp.public_key, wp.preshared_key
		FROM players p
		LEFT JOIN teams t ON t.id = p.team_id
		JOIN wireguard_peers wp ON wp.player_id = p.id
		ORDER BY p.id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	peers := make([]WireGuardGatewayPeer, 0)
	for rows.Next() {
		var peer WireGuardGatewayPeer
		var teamID sql.NullInt64
		var teamName sql.NullString
		if err := rows.Scan(
			&peer.PlayerID,
			&teamID,
			&teamName,
			&peer.DisplayName,
			&peer.WireGuardPeer,
			&peer.Address,
			&peer.Status,
			&peer.ClientPublicKey,
			&peer.PresharedKey,
		); err != nil {
			return nil, err
		}
		if teamID.Valid {
			peer.TeamID = int(teamID.Int64)
		}
		if teamName.Valid {
			peer.TeamName = teamName.String
		} else {
			peer.TeamName = "Organizer"
		}
		peers = append(peers, peer)
	}
	return peers, rows.Err()
}

func (s *postgresStore) IsMatchPaused(ctx context.Context) (bool, error) {
	var state string
	err := s.db.QueryRowContext(ctx, `SELECT state FROM game_match_state LIMIT 1`).Scan(&state)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return state == "paused", nil
}

func (s *postgresStore) ListAdminChallenges(ctx context.Context) ([]adminChallenge, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id,
		       c.name,
		       COALESCE(c.baseline_image, ''),
		       COALESCE(c.checker_image, ''),
		       COALESCE(c.source_bundle_path, ''),
		       COALESCE(NULLIF(c.service_port, 0), 10000 + c.id),
		       COALESCE(NULLIF(c.service_subnet_octet, 0), c.id),
		       c.egress_enabled,
		       c.published,
		       c.created_at,
		       c.last_validation_status,
		       c.last_validation_baseline_ssh_contract_ok,
		       c.last_validation_checker_contract_ok,
		       c.last_validation_service_state_contract_ok,
		       c.last_validation_checked_at,
		       COALESCE(c.last_validation_message, ''),
		       COUNT(DISTINCT si.team_id) AS deployed_teams,
		       COUNT(DISTINCT CASE WHEN si.runtime_status = 'queued' THEN si.team_id END) AS queued_teams,
		       COUNT(DISTINCT CASE WHEN si.runtime_status = 'ready' THEN si.team_id END) AS ready_teams,
		       (SELECT COUNT(*) FROM teams) AS total_teams
		FROM challenges c
		LEFT JOIN service_instances si ON si.challenge_id = c.id
		GROUP BY c.id, c.name, c.baseline_image, c.checker_image, c.source_bundle_path, c.service_port, c.service_subnet_octet, c.egress_enabled, c.published, c.created_at, c.last_validation_status, c.last_validation_baseline_ssh_contract_ok, c.last_validation_checker_contract_ok, c.last_validation_service_state_contract_ok, c.last_validation_checked_at, c.last_validation_message
		ORDER BY c.id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]adminChallenge, 0)
	for rows.Next() {
		var challenge adminChallenge
		var createdAt time.Time
		var validationStatus sql.NullString
		var validationBaselineOK sql.NullBool
		var validationCheckerOK sql.NullBool
		var validationServiceStateOK sql.NullBool
		var validationCheckedAt sql.NullTime
		var validationMessage string
		if err := rows.Scan(
			&challenge.ID,
			&challenge.Name,
			&challenge.BaselineImage,
			&challenge.CheckerImage,
			&challenge.SourceBundlePath,
			&challenge.ServicePort,
			&challenge.ServiceSubnetOctet,
			&challenge.EgressEnabled,
			&challenge.Published,
			&createdAt,
			&validationStatus,
			&validationBaselineOK,
			&validationCheckerOK,
			&validationServiceStateOK,
			&validationCheckedAt,
			&validationMessage,
			&challenge.DeployedTeams,
			&challenge.QueuedTeams,
			&challenge.ReadyTeams,
			&challenge.TotalTeams,
		); err != nil {
			return nil, err
		}
		challenge.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		if validationStatus.Valid && validationCheckedAt.Valid {
			challenge.LastValidation = &ChallengeValidationResult{
				ChallengeID:            challenge.ID,
				Name:                   challenge.Name,
				BaselineImage:          challenge.BaselineImage,
				CheckerImage:           challenge.CheckerImage,
				Status:                 validationStatus.String,
				BaselineSSHContractOK:  validationBaselineOK.Valid && validationBaselineOK.Bool,
				CheckerContractOK:      validationCheckerOK.Valid && validationCheckerOK.Bool,
				ServiceStateContractOK: validationServiceStateOK.Valid && validationServiceStateOK.Bool,
				CheckedAt:              validationCheckedAt.Time.UTC().Format(time.RFC3339),
				Message:                validationMessage,
			}
		}
		challenge.RuntimeStatus = challengeRuntimeStatus(challenge.Published, challenge.TotalTeams, challenge.ReadyTeams, challenge.QueuedTeams)
		result = append(result, challenge)
	}
	return result, rows.Err()
}

func (s *postgresStore) CreateAdminChallenge(ctx context.Context, input adminCreateChallengeRequest, now time.Time) (adminChallenge, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return adminChallenge{}, ErrDuplicateResource
	}
	duplicate, err := s.exists(ctx, `SELECT EXISTS(SELECT 1 FROM challenges WHERE LOWER(name) = LOWER($1))`, name)
	if err != nil {
		return adminChallenge{}, err
	}
	if duplicate {
		return adminChallenge{}, ErrDuplicateResource
	}

	baselineImage := strings.TrimSpace(input.BaselineImage)
	checkerImage := strings.TrimSpace(input.CheckerImage)
	if baselineImage == "" || checkerImage == "" {
		defaultBaseline, defaultChecker := defaultChallengeImages(name)
		if baselineImage == "" {
			baselineImage = defaultBaseline
		}
		if checkerImage == "" {
			checkerImage = defaultChecker
		}
	}
	sourceBundlePath := sanitizeSourceBundlePath(input.SourceBundlePath)

	challengeID, err := s.nextID(ctx, `SELECT COALESCE(MAX(id), 0) + 1 FROM challenges`)
	if err != nil {
		return adminChallenge{}, err
	}
	servicePort := input.ServicePort
	if servicePort <= 0 {
		servicePort = DefaultServicePort(challengeID)
	}
	serviceSubnetOctet := input.ServiceSubnetOctet
	if serviceSubnetOctet <= 0 {
		serviceSubnetOctet = DefaultServiceSubnetOctet(challengeID)
	}
	if err := validateChallengeRuntimeConfig(servicePort, serviceSubnetOctet); err != nil {
		return adminChallenge{}, err
	}
	subnetTaken, err := s.exists(ctx, `SELECT EXISTS(SELECT 1 FROM challenges WHERE service_subnet_octet = $1)`, serviceSubnetOctet)
	if err != nil {
		return adminChallenge{}, err
	}
	if subnetTaken {
		return adminChallenge{}, fmt.Errorf("%w: service_subnet_octet %d is already assigned to another challenge", ErrInvalidRuntimeConfig, serviceSubnetOctet)
	}
	egressEnabled := true
	if input.EgressEnabled != nil {
		egressEnabled = *input.EgressEnabled
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO challenges (id, name, baseline_image, checker_image, source_bundle_path, service_port, service_subnet_octet, egress_enabled, published, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, FALSE, $9)
	`, challengeID, name, baselineImage, checkerImage, sourceBundlePath, servicePort, serviceSubnetOctet, egressEnabled, now.UTC()); err != nil {
		return adminChallenge{}, err
	}
	totalTeams, err := s.countTeams(ctx)
	if err != nil {
		return adminChallenge{}, err
	}
	return adminChallenge{
		ID:                 challengeID,
		Name:               name,
		BaselineImage:      baselineImage,
		CheckerImage:       checkerImage,
		SourceBundlePath:   sourceBundlePath,
		ServicePort:        servicePort,
		ServiceSubnetOctet: serviceSubnetOctet,
		EgressEnabled:      egressEnabled,
		Published:          false,
		DeployedTeams:      0,
		TotalTeams:         totalTeams,
		RuntimeStatus:      "draft",
		CreatedAt:          now.UTC().Format(time.RFC3339),
	}, nil
}

func (s *postgresStore) UpdateAdminTeam(ctx context.Context, teamID int, input adminUpdateTeamRequest) (adminTeam, error) {
	name := strings.TrimSpace(input.Name)
	contactEmail := strings.TrimSpace(strings.ToLower(input.ContactEmail))
	if name == "" || contactEmail == "" {
		return adminTeam{}, fmt.Errorf("%w: name and contact_email are required", ErrDuplicateResource)
	}
	duplicate, err := s.exists(ctx, `SELECT EXISTS(SELECT 1 FROM teams WHERE (LOWER(name) = LOWER($1) OR LOWER(email) = LOWER($2)) AND id != $3)`, name, contactEmail, teamID)
	if err != nil {
		return adminTeam{}, fmt.Errorf("check team duplicate: %w", err)
	}
	if duplicate {
		return adminTeam{}, ErrDuplicateResource
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE teams SET name = $2, email = $3 WHERE id = $1`, teamID, name, contactEmail); err != nil {
		return adminTeam{}, fmt.Errorf("update team: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE scoreboard_entries SET team_name = $2 WHERE team_id = $1`, teamID, name); err != nil {
		return adminTeam{}, fmt.Errorf("update scoreboard team name: %w", err)
	}
	var team adminTeam
	if err := s.db.QueryRowContext(ctx, `
		SELECT t.id, t.name, t.email, COALESCE(t.join_key, ''),
		       COALESCE((SELECT COUNT(*) FROM players p WHERE p.team_id = t.id), 0),
		       COALESCE((SELECT COUNT(DISTINCT si.challenge_id) FROM service_instances si WHERE si.team_id = t.id AND si.runtime_status = 'ready'), 0)
		FROM teams t WHERE t.id = $1
	`, teamID).Scan(&team.ID, &team.Name, &team.ContactEmail, &team.JoinKey, &team.PlayerCount, &team.DeployedChallenges); err != nil {
		return adminTeam{}, fmt.Errorf("read updated team: %w", err)
	}
	return team, nil
}

func (s *postgresStore) UpdateAdminPlayer(ctx context.Context, playerID int, input adminUpdatePlayerRequest) (adminPlayer, error) {
	displayName := strings.TrimSpace(input.DisplayName)
	email := strings.TrimSpace(strings.ToLower(input.Email))
	role := normalizedRole(input.Role)
	if displayName == "" || email == "" {
		return adminPlayer{}, fmt.Errorf("%w: display_name and email are required", ErrDuplicateResource)
	}
	duplicate, err := s.exists(ctx, `SELECT EXISTS(SELECT 1 FROM players WHERE LOWER(email) = LOWER($1) AND id != $2)`, email, playerID)
	if err != nil {
		return adminPlayer{}, fmt.Errorf("check player duplicate: %w", err)
	}
	if duplicate {
		return adminPlayer{}, ErrDuplicateResource
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE players SET display_name = $2, email = $3, role = $4 WHERE id = $1`, playerID, displayName, email, role); err != nil {
		return adminPlayer{}, fmt.Errorf("update player: %w", err)
	}
	players, err := s.ListAdminPlayers(ctx)
	if err != nil {
		return adminPlayer{}, err
	}
	for _, p := range players {
		if p.ID == playerID {
			return p, nil
		}
	}
	return adminPlayer{}, ErrPlayerNotFound
}

func (s *postgresStore) UpdateAdminChallenge(ctx context.Context, challengeID int, input adminUpdateChallengeRequest) (adminChallenge, error) {
	name := strings.TrimSpace(input.Name)
	baselineImage := strings.TrimSpace(input.BaselineImage)
	checkerImage := strings.TrimSpace(input.CheckerImage)
	sourceBundlePath := sanitizeSourceBundlePath(input.SourceBundlePath)
	if name == "" {
		return adminChallenge{}, fmt.Errorf("%w: name is required", ErrDuplicateResource)
	}
	duplicate, err := s.exists(ctx, `SELECT EXISTS(SELECT 1 FROM challenges WHERE LOWER(name) = LOWER($1) AND id != $2)`, name, challengeID)
	if err != nil {
		return adminChallenge{}, fmt.Errorf("check challenge duplicate: %w", err)
	}
	if duplicate {
		return adminChallenge{}, ErrDuplicateResource
	}
	if _, err := s.db.ExecContext(ctx, `
		UPDATE challenges
		SET name = $2,
		    baseline_image = $3,
		    checker_image = $4,
		    source_bundle_path = $5,
		    egress_enabled = COALESCE($6, egress_enabled),
		    last_validation_status = NULL,
		    last_validation_baseline_ssh_contract_ok = NULL,
		    last_validation_checker_contract_ok = NULL,
		    last_validation_service_state_contract_ok = NULL,
		    last_validation_checked_at = NULL,
		    last_validation_message = NULL
		WHERE id = $1
	`, challengeID, name, baselineImage, checkerImage, sourceBundlePath, input.EgressEnabled); err != nil {
		return adminChallenge{}, fmt.Errorf("update challenge: %w", err)
	}
	challenges, err := s.ListAdminChallenges(ctx)
	if err != nil {
		return adminChallenge{}, err
	}
	for _, c := range challenges {
		if c.ID == challengeID {
			return c, nil
		}
	}
	return adminChallenge{}, ErrChallengeNotFound
}

func (s *postgresStore) DeployAdminChallenge(ctx context.Context, challengeID int) (adminDeployment, error) {
	tx, err := s.beginTx(ctx)
	if err != nil {
		return adminDeployment{}, err
	}
	defer tx.Rollback()

	var challengeName string
	var baselineImage string
	var checkerImage string
	var servicePort int
	var serviceSubnetOctet int
	if err := tx.QueryRowContext(ctx, `
		SELECT name,
		       COALESCE(baseline_image, ''),
		       COALESCE(checker_image, ''),
		       COALESCE(NULLIF(service_port, 0), 10000 + id),
		       COALESCE(NULLIF(service_subnet_octet, 0), id)
		FROM challenges
		WHERE id = $1
	`, challengeID).Scan(&challengeName, &baselineImage, &checkerImage, &servicePort, &serviceSubnetOctet); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return adminDeployment{}, ErrChallengeNotFound
		}
		return adminDeployment{}, err
	}

	rows, err := tx.QueryContext(ctx, `SELECT id FROM teams ORDER BY id`)
	if err != nil {
		return adminDeployment{}, err
	}
	teamIDs := make([]int, 0)
	for rows.Next() {
		var teamID int
		if err := rows.Scan(&teamID); err != nil {
			rows.Close()
			return adminDeployment{}, err
		}
		teamIDs = append(teamIDs, teamID)
	}
	rows.Close()

	existingRows, err := tx.QueryContext(ctx, `
		SELECT team_id, runtime_status
		FROM service_instances
		WHERE challenge_id = $1
		FOR UPDATE
	`, challengeID)
	if err != nil {
		return adminDeployment{}, err
	}
	existingStatuses := make(map[int]string)
	for existingRows.Next() {
		var teamID int
		var runtimeStatus string
		if err := existingRows.Scan(&teamID, &runtimeStatus); err != nil {
			existingRows.Close()
			return adminDeployment{}, err
		}
		existingStatuses[teamID] = runtimeStatus
	}
	existingRows.Close()

	readyCount := 0
	for _, runtimeStatus := range existingStatuses {
		if runtimeStatus == "ready" {
			readyCount++
		}
	}

	jobID := 0
	queuedCount := 0
	createdAt := time.Now().UTC()
	for _, teamID := range teamIDs {
		if existingStatuses[teamID] == "ready" {
			state := defaultServiceStateForConfig(challengeID, teamID, challengeName, servicePort, serviceSubnetOctet)
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO team_service_states (team_id, challenge_id, endpoint, status, checker, unlocked, ssh_hint, last_event, reset_cooldown)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
				ON CONFLICT (team_id, challenge_id) DO NOTHING
			`, teamID, challengeID, state.Endpoint, state.Status, state.Checker, state.Unlocked, state.SSHHint, state.LastEvent, state.ResetCooldown); err != nil {
				return adminDeployment{}, err
			}
			continue
		}
		if jobID == 0 {
			jobID, err = s.nextIDTx(ctx, tx, `SELECT COALESCE(MAX(id), 0) + 1 FROM deployment_jobs`)
			if err != nil {
				return adminDeployment{}, err
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO deployment_jobs (id, challenge_id, challenge_name, status, target_team_count, queued_team_count, ready_team_count, failed_team_count, created_at)
				VALUES ($1, $2, $3, 'queued', $4, 0, $5, 0, $6)
			`, jobID, challengeID, challengeName, len(teamIDs), readyCount, createdAt); err != nil {
				return adminDeployment{}, err
			}
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO team_service_states (team_id, challenge_id, endpoint, status, checker, unlocked, ssh_hint, last_event, reset_cooldown)
			VALUES ($1, $2, $3, 'provisioning', 'pending', FALSE, 'deployment queued; wait for controller reconciliation before unlock', 'baseline deployment queued by organizer', 'deploying')
			ON CONFLICT (team_id, challenge_id) DO UPDATE SET
				endpoint = EXCLUDED.endpoint,
				status = EXCLUDED.status,
				checker = EXCLUDED.checker,
				unlocked = FALSE,
				ssh_hint = EXCLUDED.ssh_hint,
				last_event = EXCLUDED.last_event,
				reset_cooldown = EXCLUDED.reset_cooldown
		`, teamID, challengeID, ServiceEndpointFor(serviceSubnetOctet, servicePort, teamID)); err != nil {
			return adminDeployment{}, err
		}

		checkerToken, err := newCheckerToken()
		if err != nil {
			return adminDeployment{}, err
		}
		if _, err := tx.ExecContext(ctx, `
				INSERT INTO service_instances (
					team_id, challenge_id, deployment_job_id, runtime_kind, runtime_status, container_name,
					state_volume, baseline_image, checker_image, checker_token, endpoint, ssh_host, created_at, updated_at
				)
				VALUES ($1, $2, $3, 'docker', 'queued', $4, $5, $6, $7, $8, $9, $10, $11, $11)
				ON CONFLICT (team_id, challenge_id) DO UPDATE SET
					deployment_job_id = EXCLUDED.deployment_job_id,
					runtime_kind = EXCLUDED.runtime_kind,
					runtime_status = EXCLUDED.runtime_status,
					container_name = EXCLUDED.container_name,
					state_volume = EXCLUDED.state_volume,
					baseline_image = EXCLUDED.baseline_image,
					checker_image = EXCLUDED.checker_image,
					checker_token = EXCLUDED.checker_token,
					endpoint = EXCLUDED.endpoint,
					ssh_host = EXCLUDED.ssh_host,
					updated_at = EXCLUDED.updated_at
			`, teamID, challengeID, jobID, serviceContainerName(challengeName, teamID), serviceStateVolumeName(challengeName, teamID), baselineImage, checkerImage, checkerToken, ServiceEndpointFor(serviceSubnetOctet, servicePort, teamID), ServiceIP(serviceSubnetOctet, teamID), createdAt); err != nil {
			return adminDeployment{}, err
		}
		queuedCount++
	}

	if jobID != 0 {
		if _, err := tx.ExecContext(ctx, `
			UPDATE deployment_jobs
			SET queued_team_count = $2,
				ready_team_count = $3
			WHERE id = $1
		`, jobID, queuedCount, readyCount); err != nil {
			return adminDeployment{}, err
		}
	}

	if jobID != 0 {
		if _, err := tx.ExecContext(ctx, `
			UPDATE deployment_jobs
			SET status = 'superseded',
			    queued_team_count = 0,
			    completed_at = $3
			WHERE challenge_id = $1
			  AND id <> $2
			  AND status IN ('queued', 'running')
		`, challengeID, jobID, createdAt); err != nil {
			return adminDeployment{}, err
		}
	} else {
		if _, err := tx.ExecContext(ctx, `
			UPDATE deployment_jobs
			SET status = 'superseded',
			    queued_team_count = 0,
			    completed_at = $2
			WHERE challenge_id = $1
			  AND status IN ('queued', 'running')
		`, challengeID, createdAt); err != nil {
			return adminDeployment{}, err
		}
	}

	if _, err := tx.ExecContext(ctx, `UPDATE challenges SET published = TRUE WHERE id = $1`, challengeID); err != nil {
		return adminDeployment{}, err
	}
	if err := tx.Commit(); err != nil {
		return adminDeployment{}, err
	}

	status := "completed"
	if queuedCount > 0 {
		status = "queued"
	}

	return adminDeployment{
		JobID:             jobID,
		ChallengeID:       challengeID,
		ChallengeName:     challengeName,
		Status:            status,
		Published:         true,
		DeployedTeamCount: queuedCount,
		TotalTeamCount:    len(teamIDs),
		QueuedTeamCount:   queuedCount,
		ReadyTeamCount:    readyCount,
		CreatedAt:         createdAt.Format(time.RFC3339),
	}, nil
}

func (s *postgresStore) ListAdminDeployments(ctx context.Context) ([]adminDeploymentJob, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, challenge_id, challenge_name, status, target_team_count, queued_team_count, ready_team_count, failed_team_count, created_at, completed_at
		FROM deployment_jobs
		ORDER BY id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]adminDeploymentJob, 0)
	for rows.Next() {
		var job adminDeploymentJob
		var createdAt time.Time
		var completedAt sql.NullTime
		if err := rows.Scan(
			&job.ID,
			&job.ChallengeID,
			&job.ChallengeName,
			&job.Status,
			&job.TargetTeamCount,
			&job.QueuedTeamCount,
			&job.ReadyTeamCount,
			&job.FailedTeamCount,
			&createdAt,
			&completedAt,
		); err != nil {
			return nil, err
		}
		job.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		if completedAt.Valid {
			job.CompletedAt = completedAt.Time.UTC().Format(time.RFC3339)
		}
		result = append(result, job)
	}
	return result, rows.Err()
}

func (s *postgresStore) DeleteAdminDeployment(ctx context.Context, deploymentID int) error {
	tx, err := s.beginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = tx.QueryRowContext(ctx, `
		SELECT 1
		FROM deployment_jobs
		WHERE id = $1
		FOR UPDATE
	`, deploymentID).Scan(new(int))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrDeploymentNotFound
		}
		return err
	}

	var queuedInstances int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM service_instances
		WHERE deployment_job_id = $1
		  AND runtime_status = 'queued'
	`, deploymentID).Scan(&queuedInstances); err != nil {
		return err
	}
	if queuedInstances > 0 {
		return ErrDeploymentActive
	}

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM deployment_jobs
		WHERE id = $1
	`, deploymentID); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *postgresStore) ListAdminAuditLogs(ctx context.Context, query adminAuditLogQuery) (adminAuditLogPage, error) {
	limit := query.Limit
	if limit <= 0 {
		limit = 25
	}
	offset := query.Offset
	if offset < 0 {
		offset = 0
	}

	conditions := make([]string, 0, 4)
	args := make([]any, 0, 6)
	argIndex := 1

	if trimmed := strings.TrimSpace(query.ActorType); trimmed != "" {
		conditions = append(conditions, fmt.Sprintf("actor_type = $%d", argIndex))
		args = append(args, trimmed)
		argIndex++
	}
	if trimmed := strings.TrimSpace(query.Action); trimmed != "" {
		conditions = append(conditions, fmt.Sprintf("action ILIKE $%d", argIndex))
		args = append(args, "%"+trimmed+"%")
		argIndex++
	}
	if trimmed := strings.TrimSpace(query.TargetType); trimmed != "" {
		conditions = append(conditions, fmt.Sprintf("target_type = $%d", argIndex))
		args = append(args, trimmed)
		argIndex++
	}
	if trimmed := strings.TrimSpace(query.Status); trimmed != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIndex))
		args = append(args, trimmed)
		argIndex++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " WHERE " + strings.Join(conditions, " AND ")
	}

	countQuery := "SELECT COUNT(*) FROM audit_logs" + whereClause
	var totalCount int
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&totalCount); err != nil {
		return adminAuditLogPage{}, err
	}

	selectArgs := append(append([]any(nil), args...), limit, offset)
	// #nosec G202 -- whereClause is assembled from fixed predicates and placeholders only.
	selectQuery := `
		SELECT id, actor_type, actor, action, target_type, target, status, message, metadata::text, created_at
		FROM audit_logs` + whereClause + fmt.Sprintf(`
		ORDER BY id DESC
		LIMIT $%d OFFSET $%d`, argIndex, argIndex+1)

	rows, err := s.db.QueryContext(ctx, selectQuery, selectArgs...)
	if err != nil {
		return adminAuditLogPage{}, err
	}
	defer rows.Close()

	page := adminAuditLogPage{
		Items:      make([]adminAuditLog, 0),
		Limit:      limit,
		Offset:     offset,
		TotalCount: totalCount,
		HasPrev:    offset > 0,
		HasNext:    offset+limit < totalCount,
	}
	for rows.Next() {
		var entry adminAuditLog
		var createdAt time.Time
		if err := rows.Scan(
			&entry.ID,
			&entry.ActorType,
			&entry.Actor,
			&entry.Action,
			&entry.TargetType,
			&entry.Target,
			&entry.Status,
			&entry.Message,
			&entry.Metadata,
			&createdAt,
		); err != nil {
			return adminAuditLogPage{}, err
		}
		entry.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		page.Items = append(page.Items, entry)
	}
	return page, rows.Err()
}

func (s *postgresStore) AppendAdminAuditLog(ctx context.Context, entry adminAuditLogEntry) error {
	metadata := strings.TrimSpace(entry.Metadata)
	if metadata == "" {
		metadata = "{}"
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO audit_logs (
			actor_type,
			actor,
			action,
			target_type,
			target,
			status,
			message,
			metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb)
	`,
		strings.TrimSpace(entry.ActorType),
		strings.TrimSpace(entry.Actor),
		strings.TrimSpace(entry.Action),
		strings.TrimSpace(entry.TargetType),
		strings.TrimSpace(entry.Target),
		strings.TrimSpace(entry.Status),
		strings.TrimSpace(entry.Message),
		metadata,
	)
	return err
}

func (s *postgresStore) GetPlatformSettings(ctx context.Context) (adminPlatformSettings, error) {
	var settings adminPlatformSettings
	var updatedAt time.Time
	if err := s.db.QueryRowContext(ctx, `
		SELECT flag_format_prefix, flag_format_active, max_team_members, updated_at, updated_by
		FROM platform_settings
		WHERE id = 1
	`).Scan(
		&settings.FlagFormatPrefix,
		&settings.FlagFormatActive,
		&settings.MaxTeamMembers,
		&updatedAt,
		&settings.UpdatedBy,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return adminPlatformSettings{FlagFormatPrefix: "PLAYIT", FlagFormatActive: "PLAYIT", MaxTeamMembers: 0, UpdatedBy: "system"}, nil
		}
		return adminPlatformSettings{}, err
	}
	settings.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
	return settings, nil
}

func (s *postgresStore) UpdatePlatformSettings(ctx context.Context, input adminUpdatePlatformSettingsRequest, actor string, now time.Time) (adminPlatformSettings, error) {
	prefix := strings.TrimSpace(input.FlagFormatPrefix)
	if prefix == "" {
		return adminPlatformSettings{}, fmt.Errorf("flag_format_prefix must not be empty")
	}
	if input.MaxTeamMembers != nil && *input.MaxTeamMembers < 0 {
		return adminPlatformSettings{}, fmt.Errorf("max_team_members must not be negative")
	}
	maxTeamMembers := 0
	if input.MaxTeamMembers != nil {
		maxTeamMembers = *input.MaxTeamMembers
	} else if current, err := s.GetPlatformSettings(ctx); err == nil {
		maxTeamMembers = current.MaxTeamMembers
	} else {
		return adminPlatformSettings{}, err
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO platform_settings (id, flag_format_prefix, flag_format_active, max_team_members, updated_at, updated_by)
		VALUES (1, $1, COALESCE((SELECT flag_format_active FROM platform_settings WHERE id = 1), $1), $2, $3, $4)
		ON CONFLICT (id) DO UPDATE SET
			flag_format_prefix = EXCLUDED.flag_format_prefix,
			max_team_members = EXCLUDED.max_team_members,
			updated_at = EXCLUDED.updated_at,
			updated_by = EXCLUDED.updated_by
	`, prefix, maxTeamMembers, now.UTC(), strings.TrimSpace(actor)); err != nil {
		return adminPlatformSettings{}, err
	}
	return s.GetPlatformSettings(ctx)
}

func (s *postgresStore) SetActiveFlagFormat(ctx context.Context, format string, now time.Time) (adminPlatformSettings, error) {
	active := strings.TrimSpace(format)
	if active == "" {
		return adminPlatformSettings{}, fmt.Errorf("active flag format must not be empty")
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO platform_settings (id, flag_format_prefix, flag_format_active, updated_at, updated_by)
		VALUES (1, $1, $2, $3, '')
		ON CONFLICT (id) DO UPDATE SET
			flag_format_active = EXCLUDED.flag_format_active,
			updated_at = EXCLUDED.updated_at
	`, active, active, now.UTC()); err != nil {
		return adminPlatformSettings{}, err
	}
	return s.GetPlatformSettings(ctx)
}

func (s *postgresStore) ListControllerRuntimeTasks(ctx context.Context) ([]ControllerRuntimeTask, error) {
	rows, err := s.db.QueryContext(ctx, `
			SELECT COALESCE(si.deployment_job_id, 0), si.team_id, si.challenge_id, c.name, si.runtime_kind, si.container_name, si.state_volume, si.baseline_image, si.checker_token, si.endpoint, si.ssh_host, c.service_port
		FROM service_instances si
		JOIN challenges c ON c.id = si.challenge_id
		LEFT JOIN deployment_jobs dj ON dj.id = si.deployment_job_id
		WHERE si.runtime_status = 'queued' AND COALESCE(dj.status, 'queued') IN ('queued', 'running')
		ORDER BY si.deployment_job_id, si.challenge_id, si.team_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]ControllerRuntimeTask, 0)
	for rows.Next() {
		var task ControllerRuntimeTask
		if err := rows.Scan(
			&task.DeploymentJobID,
			&task.TeamID,
			&task.ChallengeID,
			&task.ChallengeName,
			&task.RuntimeKind,
			&task.ContainerName,
			&task.StateVolume,
			&task.BaselineImage,
			&task.CheckerToken,
			&task.Endpoint,
			&task.SSHHost,
			&task.ServicePort,
		); err != nil {
			return nil, err
		}
		result = append(result, task)
	}
	return result, rows.Err()
}

func (s *postgresStore) GetControllerRuntimeTask(ctx context.Context, teamID, challengeID int) (ControllerRuntimeTask, error) {
	var task ControllerRuntimeTask
	if err := s.db.QueryRowContext(ctx, `
			SELECT COALESCE(si.deployment_job_id, 0), si.team_id, si.challenge_id, c.name, si.runtime_kind, si.container_name, si.state_volume, si.baseline_image, si.checker_token, si.endpoint, si.ssh_host, c.service_port
		FROM service_instances si
		JOIN challenges c ON c.id = si.challenge_id
		WHERE si.team_id = $1 AND si.challenge_id = $2
	`, teamID, challengeID).Scan(
		&task.DeploymentJobID,
		&task.TeamID,
		&task.ChallengeID,
		&task.ChallengeName,
		&task.RuntimeKind,
		&task.ContainerName,
		&task.StateVolume,
		&task.BaselineImage,
		&task.CheckerToken,
		&task.Endpoint,
		&task.SSHHost,
		&task.ServicePort,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ControllerRuntimeTask{}, ErrChallengeNotFound
		}
		return ControllerRuntimeTask{}, err
	}
	return task, nil
}

func (s *postgresStore) ListControllerServiceAccessPolicies(ctx context.Context) ([]ControllerServiceAccessPolicy, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			tss.team_id,
			t.name,
			tss.challenge_id,
			c.name,
			split_part(tss.endpoint, ':', 1),
			split_part(tss.endpoint, ':', 2),
			tss.unlocked,
			c.egress_enabled,
			COALESCE((
				SELECT string_agg(wp.address, ',')
				FROM players p
				JOIN wireguard_peers wp ON wp.player_id = p.id
				WHERE ((p.team_id = tss.team_id AND tss.unlocked = TRUE) OR p.role = 'organizer')
				  AND wp.status <> 'revoked'
			), '')
		FROM team_service_states tss
		JOIN teams t ON t.id = tss.team_id
		JOIN challenges c ON c.id = tss.challenge_id
		ORDER BY tss.challenge_id, tss.team_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]ControllerServiceAccessPolicy, 0)
	for rows.Next() {
		var policy ControllerServiceAccessPolicy
		var allowedPeersCSV string
		var servicePortText string
		if err := rows.Scan(
			&policy.TeamID,
			&policy.TeamName,
			&policy.ChallengeID,
			&policy.ChallengeName,
			&policy.ServiceIP,
			&servicePortText,
			&policy.SSHUnlocked,
			&policy.EgressEnabled,
			&allowedPeersCSV,
		); err != nil {
			return nil, err
		}
		policy.ServicePort, _ = strconv.Atoi(servicePortText)
		policy.SSHPort = 22
		policy.AllowedPeerAddresses = splitCSVList(allowedPeersCSV)
		result = append(result, policy)
	}
	return result, rows.Err()
}

func (s *postgresStore) GetControllerServiceAccessPolicy(ctx context.Context, teamID, challengeID int) (ControllerServiceAccessPolicy, error) {
	var policy ControllerServiceAccessPolicy
	var allowedPeersCSV string
	var servicePortText string
	if err := s.db.QueryRowContext(ctx, `
		SELECT
			tss.team_id,
			t.name,
			tss.challenge_id,
			c.name,
			split_part(tss.endpoint, ':', 1),
			split_part(tss.endpoint, ':', 2),
			tss.unlocked,
			c.egress_enabled,
			COALESCE((
				SELECT string_agg(wp.address, ',')
				FROM players p
				JOIN wireguard_peers wp ON wp.player_id = p.id
				WHERE ((p.team_id = tss.team_id AND tss.unlocked = TRUE) OR p.role = 'organizer')
				  AND wp.status <> 'revoked'
			), '')
		FROM team_service_states tss
		JOIN teams t ON t.id = tss.team_id
		JOIN challenges c ON c.id = tss.challenge_id
		WHERE tss.team_id = $1 AND tss.challenge_id = $2
	`, teamID, challengeID).Scan(
		&policy.TeamID,
		&policy.TeamName,
		&policy.ChallengeID,
		&policy.ChallengeName,
		&policy.ServiceIP,
		&servicePortText,
		&policy.SSHUnlocked,
		&policy.EgressEnabled,
		&allowedPeersCSV,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ControllerServiceAccessPolicy{}, ErrChallengeNotFound
		}
		return ControllerServiceAccessPolicy{}, err
	}
	policy.ServicePort, _ = strconv.Atoi(servicePortText)
	policy.SSHPort = 22
	policy.AllowedPeerAddresses = splitCSVList(allowedPeersCSV)
	return policy, nil
}

func (s *postgresStore) ReconcileAdminDeployments(ctx context.Context, now time.Time) (adminReconcileResult, error) {
	tx, err := s.beginTx(ctx)
	if err != nil {
		return adminReconcileResult{}, err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, `
		SELECT id, challenge_id
		FROM deployment_jobs
		WHERE status IN ('queued', 'running')
		ORDER BY id
		FOR UPDATE
	`)
	if err != nil {
		return adminReconcileResult{}, err
	}
	type deploymentRef struct {
		ID          int
		ChallengeID int
	}
	deployments := make([]deploymentRef, 0)
	for rows.Next() {
		var ref deploymentRef
		if err := rows.Scan(&ref.ID, &ref.ChallengeID); err != nil {
			rows.Close()
			return adminReconcileResult{}, err
		}
		deployments = append(deployments, ref)
	}
	rows.Close()

	result := adminReconcileResult{}
	for _, deployment := range deployments {
		result.ProcessedJobs++
		if _, err := tx.ExecContext(ctx, `
			UPDATE deployment_jobs
			SET status = 'running'
			WHERE id = $1
		`, deployment.ID); err != nil {
			return adminReconcileResult{}, err
		}

		instanceRows, err := tx.QueryContext(ctx, `
			UPDATE service_instances
			SET runtime_status = 'ready',
			    updated_at = $3
			WHERE challenge_id = $1 AND deployment_job_id = $2 AND runtime_status = 'queued'
			RETURNING team_id
		`, deployment.ChallengeID, deployment.ID, now.UTC())
		if err != nil {
			return adminReconcileResult{}, err
		}

		teamIDs := make([]int, 0)
		for instanceRows.Next() {
			var teamID int
			if err := instanceRows.Scan(&teamID); err != nil {
				instanceRows.Close()
				return adminReconcileResult{}, err
			}
			teamIDs = append(teamIDs, teamID)
		}
		instanceRows.Close()

		for _, teamID := range teamIDs {
			if _, err := tx.ExecContext(ctx, `
				UPDATE team_service_states
				SET status = 'stable',
				    checker = 'passing',
				    ssh_hint = 'solve service to generate SSH credential',
				    last_event = 'baseline deployed by controller reconcile',
				    reset_cooldown = 'ready'
				WHERE team_id = $1 AND challenge_id = $2
			`, teamID, deployment.ChallengeID); err != nil {
				return adminReconcileResult{}, err
			}
		}

		if _, err := tx.ExecContext(ctx, `
			UPDATE deployment_jobs
			SET status = 'completed',
			    queued_team_count = 0,
			    ready_team_count = ready_team_count + $2,
			    completed_at = $3
			WHERE id = $1
		`, deployment.ID, len(teamIDs), now.UTC()); err != nil {
			return adminReconcileResult{}, err
		}

		result.ProcessedInstances += len(teamIDs)
		result.CompletedJobs++
	}

	// Handle "job-less" queued instances (e.g. from new team joining)
	instanceRows, err := tx.QueryContext(ctx, `
		UPDATE service_instances
		SET runtime_status = 'ready',
		    updated_at = $1
		WHERE deployment_job_id IS NULL AND runtime_status = 'queued'
		RETURNING team_id, challenge_id
	`, now.UTC())
	if err != nil {
		return adminReconcileResult{}, err
	}
	type instanceRef struct {
		TeamID      int
		ChallengeID int
	}
	standaloneInstances := make([]instanceRef, 0)
	for instanceRows.Next() {
		var ref instanceRef
		if err := instanceRows.Scan(&ref.TeamID, &ref.ChallengeID); err != nil {
			instanceRows.Close()
			return adminReconcileResult{}, err
		}
		standaloneInstances = append(standaloneInstances, ref)
	}
	instanceRows.Close()

	for _, ref := range standaloneInstances {
		result.ProcessedInstances++
		if _, err := tx.ExecContext(ctx, `
			UPDATE team_service_states
			SET status = 'stable',
			    checker = 'passing',
			    ssh_hint = 'solve service to generate SSH credential',
			    last_event = 'baseline deployed by controller reconcile',
			    reset_cooldown = 'ready'
			WHERE team_id = $1 AND challenge_id = $2
		`, ref.TeamID, ref.ChallengeID); err != nil {
			return adminReconcileResult{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return adminReconcileResult{}, err
	}
	return result, nil
}

func (s *postgresStore) DeleteAdminPlayer(ctx context.Context, playerID int) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM players WHERE id = $1`, playerID)
	return err
}

func (s *postgresStore) DeleteAdminTeam(ctx context.Context, teamID int) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM teams WHERE id = $1`, teamID)
	return err
}

func (s *postgresStore) DeleteAdminChallenge(ctx context.Context, challengeID int) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM challenges WHERE id = $1`, challengeID)
	return err
}

func (s *postgresStore) SaveAdminChallengeValidation(ctx context.Context, result ChallengeValidationResult) error {
	checkedAt, err := time.Parse(time.RFC3339, strings.TrimSpace(result.CheckedAt))
	if err != nil {
		checkedAt = time.Now().UTC()
	}
	command, err := s.db.ExecContext(ctx, `
		UPDATE challenges
		SET last_validation_status = $2,
		    last_validation_baseline_ssh_contract_ok = $3,
		    last_validation_checker_contract_ok = $4,
		    last_validation_service_state_contract_ok = $5,
		    last_validation_checked_at = $6,
		    last_validation_message = $7
		WHERE id = $1
	`,
		result.ChallengeID,
		strings.TrimSpace(result.Status),
		result.BaselineSSHContractOK,
		result.CheckerContractOK,
		result.ServiceStateContractOK,
		checkedAt.UTC(),
		strings.TrimSpace(result.Message),
	)
	if err != nil {
		return err
	}
	affected, err := command.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrChallengeNotFound
	}
	return nil
}

func (s *postgresStore) Close() error {
	return s.db.Close()
}

func (s *postgresStore) exists(ctx context.Context, query string, args ...any) (bool, error) {
	var exists bool
	if err := s.queryRowScan(ctx, query, args, &exists); err != nil {
		return false, err
	}
	return exists, nil
}

func (s *postgresStore) nextID(ctx context.Context, query string, args ...any) (int, error) {
	var id int
	if err := s.queryRowScan(ctx, query, args, &id); err != nil {
		return 0, err
	}
	return id, nil
}

func (s *postgresStore) nextIDTx(ctx context.Context, tx *sql.Tx, query string, args ...any) (int, error) {
	var id int
	if err := tx.QueryRowContext(ctx, query, args...).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

func (s *postgresStore) lookupTeamName(ctx context.Context, teamID int) (string, error) {
	if teamID == 0 {
		return "Organizer", nil
	}
	var name string
	if err := s.queryRowScan(ctx, `SELECT name FROM teams WHERE id = $1`, []any{teamID}, &name); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrTeamNotFound
		}
		return "", err
	}
	return name, nil
}

func (s *postgresStore) countTeams(ctx context.Context) (int, error) {
	var count int
	if err := s.queryRowScan(ctx, `SELECT COUNT(*) FROM teams`, nil, &count); err != nil {
		return 0, err
	}
	return count, nil
}

type wireGuardProvisionIdentity struct {
	PlayerID      int
	TeamID        int
	TeamName      string
	DisplayName   string
	Email         string
	WireGuardPeer string
}

func (s *postgresStore) ensureAllWireGuardPeers(ctx context.Context, now time.Time) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT p.id
		FROM players p
		LEFT JOIN wireguard_peers wp ON wp.player_id = p.id
		WHERE wp.player_id IS NULL
		ORDER BY p.id
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	playerIDs := make([]int, 0)
	for rows.Next() {
		var playerID int
		if err := rows.Scan(&playerID); err != nil {
			return err
		}
		playerIDs = append(playerIDs, playerID)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, playerID := range playerIDs {
		if err := s.ensureWireGuardPeer(ctx, playerID, now); err != nil {
			return err
		}
	}
	return nil
}

func (s *postgresStore) ensureWireGuardPeer(ctx context.Context, playerID int, now time.Time) error {
	tx, err := s.beginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	identity, err := loadWireGuardProvisionIdentityTx(ctx, tx, playerID)
	if err != nil {
		return err
	}

	wireGuardState, found, err := loadWireGuardPeerStateTx(ctx, tx, identity)
	if err != nil {
		return err
	}
	if !found {
		wireGuardState, err = newWireGuardPeerState(identity.PlayerID, identity.TeamID, identity.TeamName, identity.DisplayName, identity.WireGuardPeer, now)
		if err != nil {
			return err
		}
		if err := insertWireGuardPeerTx(ctx, tx, wireGuardState); err != nil {
			return err
		}
		return tx.Commit()
	}

	refreshedState, changed, err := refreshWireGuardPeerState(wireGuardState)
	if err != nil {
		return err
	}
	if changed {
		if err := updateWireGuardPeerServerConfigTx(ctx, tx, refreshedState, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *postgresStore) loadAdminWireGuardPeer(ctx context.Context, playerID int) (adminWireGuardPeer, error) {
	var peer adminWireGuardPeer
	var teamID sql.NullInt64
	var teamName sql.NullString
	var issuedAt time.Time
	var revokedAt sql.NullTime
	if err := s.db.QueryRowContext(ctx, `
		SELECT p.id, p.team_id, t.name, p.display_name, p.email, p.wireguard_peer,
		       wp.address, wp.status, wp.server_endpoint, wp.server_public_key,
		       wp.public_key, wp.allowed_ips, wp.dns, wp.config, wp.issued_at, wp.revoked_at
		FROM players p
		LEFT JOIN teams t ON t.id = p.team_id
		JOIN wireguard_peers wp ON wp.player_id = p.id
		WHERE p.id = $1
	`, playerID).Scan(
		&peer.PlayerID,
		&teamID,
		&teamName,
		&peer.DisplayName,
		&peer.Email,
		&peer.WireGuardPeer,
		&peer.Address,
		&peer.Status,
		&peer.ServerEndpoint,
		&peer.ServerPublicKey,
		&peer.ClientPublicKey,
		&peer.AllowedIPs,
		&peer.DNS,
		&peer.Config,
		&issuedAt,
		&revokedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return adminWireGuardPeer{}, ErrPlayerNotFound
		}
		return adminWireGuardPeer{}, err
	}
	if teamID.Valid {
		peer.TeamID = int(teamID.Int64)
	}
	if teamName.Valid {
		peer.TeamName = teamName.String
	} else {
		peer.TeamName = "Organizer"
	}
	peer.DownloadName = wireGuardDownloadName(peer.Email, peer.WireGuardPeer)
	peer.IssuedAt = issuedAt.UTC().Format(time.RFC3339)
	if revokedAt.Valid {
		peer.RevokedAt = revokedAt.Time.UTC().Format(time.RFC3339)
	}
	return peer, nil
}

func loadWireGuardProvisionIdentityTx(ctx context.Context, tx *sql.Tx, playerID int) (wireGuardProvisionIdentity, error) {
	var identity wireGuardProvisionIdentity
	var teamID sql.NullInt64
	var teamName sql.NullString
	if err := tx.QueryRowContext(ctx, `
		SELECT p.id, p.team_id, t.name, p.display_name, p.email, p.wireguard_peer
		FROM players p
		LEFT JOIN teams t ON t.id = p.team_id
		WHERE p.id = $1
		FOR UPDATE OF p
	`, playerID).Scan(
		&identity.PlayerID,
		&teamID,
		&teamName,
		&identity.DisplayName,
		&identity.Email,
		&identity.WireGuardPeer,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return wireGuardProvisionIdentity{}, ErrPlayerNotFound
		}
		return wireGuardProvisionIdentity{}, err
	}
	if teamID.Valid {
		identity.TeamID = int(teamID.Int64)
	}
	if teamName.Valid {
		identity.TeamName = teamName.String
	} else {
		identity.TeamName = "Organizer"
	}
	return identity, nil
}

func loadWireGuardPeerStateTx(ctx context.Context, tx *sql.Tx, identity wireGuardProvisionIdentity) (wireGuardPeerState, bool, error) {
	var (
		state     wireGuardPeerState
		issuedAt  time.Time
		revokedAt sql.NullTime
	)
	err := tx.QueryRowContext(ctx, `
		SELECT address, status, server_endpoint, server_public_key,
		       private_key, public_key, preshared_key, allowed_ips, dns, config, issued_at, revoked_at
		FROM wireguard_peers
		WHERE player_id = $1
		FOR UPDATE
	`, identity.PlayerID).Scan(
		&state.Address,
		&state.Status,
		&state.ServerEndpoint,
		&state.ServerPublicKey,
		&state.ClientPrivateKey,
		&state.ClientPublicKey,
		&state.PresharedKey,
		&state.AllowedIPs,
		&state.DNS,
		&state.Config,
		&issuedAt,
		&revokedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return wireGuardPeerState{}, false, nil
	}
	if err != nil {
		return wireGuardPeerState{}, false, err
	}

	state.PlayerID = identity.PlayerID
	state.TeamID = identity.TeamID
	state.TeamName = identity.TeamName
	state.DisplayName = identity.DisplayName
	state.WireGuardPeer = identity.WireGuardPeer
	state.IssuedAt = issuedAt.UTC().Format(time.RFC3339)
	if revokedAt.Valid {
		state.RevokedAt = revokedAt.Time.UTC().Format(time.RFC3339)
	}
	return state, true, nil
}

func insertWireGuardPeerTx(ctx context.Context, tx *sql.Tx, state wireGuardPeerState) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO wireguard_peers (
			player_id, address, private_key, public_key, preshared_key,
			server_public_key, server_endpoint, dns, allowed_ips, status,
			config, issued_at, revoked_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NULL, $12)
	`, state.PlayerID, state.Address, state.ClientPrivateKey, state.ClientPublicKey, state.PresharedKey, state.ServerPublicKey, state.ServerEndpoint, state.DNS, state.AllowedIPs, state.Status, state.Config, state.IssuedAt)
	return err
}

func updateWireGuardPeerServerConfigTx(ctx context.Context, tx *sql.Tx, state wireGuardPeerState, now time.Time) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE wireguard_peers
		SET server_public_key = $2,
		    server_endpoint = $3,
		    dns = $4,
		    allowed_ips = $5,
		    config = $6,
		    updated_at = $7
		WHERE player_id = $1
	`, state.PlayerID, state.ServerPublicKey, state.ServerEndpoint, state.DNS, state.AllowedIPs, state.Config, now.UTC())
	return err
}

func upsertWireGuardPeerTx(ctx context.Context, tx *sql.Tx, state wireGuardPeerState) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO wireguard_peers (
			player_id, address, private_key, public_key, preshared_key,
			server_public_key, server_endpoint, dns, allowed_ips, status,
			config, issued_at, revoked_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NULL, $12)
		ON CONFLICT (player_id) DO UPDATE SET
			address = EXCLUDED.address,
			private_key = EXCLUDED.private_key,
			public_key = EXCLUDED.public_key,
			preshared_key = EXCLUDED.preshared_key,
			server_public_key = EXCLUDED.server_public_key,
			server_endpoint = EXCLUDED.server_endpoint,
			dns = EXCLUDED.dns,
			allowed_ips = EXCLUDED.allowed_ips,
			status = EXCLUDED.status,
			config = EXCLUDED.config,
			issued_at = EXCLUDED.issued_at,
			revoked_at = NULL,
			updated_at = EXCLUDED.updated_at
	`, state.PlayerID, state.Address, state.ClientPrivateKey, state.ClientPublicKey, state.PresharedKey, state.ServerPublicKey, state.ServerEndpoint, state.DNS, state.AllowedIPs, state.Status, state.Config, state.IssuedAt)
	return err
}
