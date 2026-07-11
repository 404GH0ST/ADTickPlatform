package apigateway

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (s *postgresStore) ChangeParticipantPassword(ctx context.Context, playerID int, currentPassword, newPassword string) error {
	var role string
	var storedHash string
	err := s.queryRowScan(ctx, `
		SELECT role, password_hash
		FROM players
		WHERE id = $1
	`, []any{playerID}, &role, &storedHash)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrInvalidCredentials
	}
	if err != nil {
		return err
	}
	if strings.EqualFold(strings.TrimSpace(role), "organizer") {
		return ErrInvalidCredentials
	}
	if !passwordMatches(storedHash, currentPassword) {
		return ErrInvalidCredentials
	}
	hashed, err := hashPassword(newPassword)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `UPDATE players SET password_hash = $2 WHERE id = $1`, playerID, hashed)
	return err
}

func (s *postgresStore) ListAnnouncements(ctx context.Context) ([]matchAnnouncement, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, body, created_by, created_at
		FROM match_announcements
		ORDER BY created_at DESC, id DESC
		LIMIT 100
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]matchAnnouncement, 0)
	for rows.Next() {
		var item matchAnnouncement
		var createdAt time.Time
		if err := rows.Scan(&item.ID, &item.Body, &item.CreatedBy, &createdAt); err != nil {
			return nil, err
		}
		item.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *postgresStore) CreateAnnouncement(ctx context.Context, body, createdBy string, now time.Time) (matchAnnouncement, error) {
	var item matchAnnouncement
	var createdAt time.Time
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO match_announcements (body, created_by, created_at)
		VALUES ($1, $2, $3)
		RETURNING id, body, created_by, created_at
	`, body, createdBy, now.UTC()).Scan(&item.ID, &item.Body, &item.CreatedBy, &createdAt)
	if err != nil {
		return matchAnnouncement{}, err
	}
	item.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	return item, nil
}

func (s *postgresStore) DeleteAnnouncement(ctx context.Context, id int) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM match_announcements WHERE id = $1`, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrResourceNotFound
	}
	return nil
}

// ImportAdminTeams creates each team atomically with its players.
// A player failure rolls back that team only; other teams still commit.
func (s *postgresStore) ImportAdminTeams(ctx context.Context, teams []adminBulkImportTeam, now time.Time) (adminBulkImportResult, error) {
	result := adminBulkImportResult{
		Teams:   make([]adminTeam, 0),
		Players: make([]adminPlayer, 0),
		Errors:  make([]string, 0),
	}

	for index, teamInput := range teams {
		name := strings.TrimSpace(teamInput.Name)
		email := strings.TrimSpace(teamInput.ContactEmail)
		if name == "" || email == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("row %d: team name and contact_email are required", index+1))
			continue
		}

		players := make([]adminCreatePlayerRequest, 0, len(teamInput.Players))
		playerFailed := false
		for playerIndex, playerInput := range teamInput.Players {
			displayName := strings.TrimSpace(playerInput.DisplayName)
			playerEmail := strings.TrimSpace(playerInput.Email)
			password := strings.TrimSpace(playerInput.Password)
			role := strings.TrimSpace(playerInput.Role)
			if role == "" {
				role = "member"
			}
			if displayName == "" || playerEmail == "" || password == "" {
				result.Errors = append(result.Errors, fmt.Sprintf("row %d player %d: display_name, email, and password are required", index+1, playerIndex+1))
				playerFailed = true
				break
			}
			players = append(players, adminCreatePlayerRequest{
				DisplayName: displayName,
				Email:       playerEmail,
				Password:    password,
				Role:        role,
			})
		}
		if playerFailed {
			continue
		}

		team, createdPlayers, err := s.importOneTeam(ctx, name, email, players, now)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("row %d (%s): %v", index+1, name, err))
			continue
		}
		result.TeamsCreated++
		result.PlayersCreated += len(createdPlayers)
		result.Teams = append(result.Teams, team)
		result.Players = append(result.Players, createdPlayers...)
	}

	return result, nil
}

func (s *postgresStore) importOneTeam(
	ctx context.Context,
	name, contactEmail string,
	players []adminCreatePlayerRequest,
	now time.Time,
) (adminTeam, []adminPlayer, error) {
	// CreateAdminTeam already uses a transaction for team + scoreboard + instances.
	team, err := s.CreateAdminTeam(ctx, adminCreateTeamRequest{
		Name:         name,
		ContactEmail: contactEmail,
	})
	if err != nil {
		return adminTeam{}, nil, err
	}

	created := make([]adminPlayer, 0, len(players))
	for _, playerInput := range players {
		playerInput.TeamID = team.ID
		player, err := s.CreateAdminPlayer(ctx, playerInput, now)
		if err != nil {
			// Best-effort rollback of the orphaned team so this row is atomic.
			_ = s.DeleteAdminTeam(ctx, team.ID)
			return adminTeam{}, nil, err
		}
		created = append(created, player)
	}
	return team, created, nil
}

func (s *memoryStore) ChangeParticipantPassword(_ context.Context, playerID int, currentPassword, newPassword string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.players[playerID]
	if !ok || strings.EqualFold(strings.TrimSpace(record.Player.Role), "organizer") {
		return ErrInvalidCredentials
	}
	if !passwordMatches(record.PasswordHash, currentPassword) {
		return ErrInvalidCredentials
	}
	record.PasswordHash = mustHashPassword(newPassword)
	return nil
}

func (s *memoryStore) ListAnnouncements(_ context.Context) ([]matchAnnouncement, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]matchAnnouncement, 0, len(s.announcements))
	for i := len(s.announcements) - 1; i >= 0; i-- {
		result = append(result, s.announcements[i])
	}
	return result, nil
}

func (s *memoryStore) CreateAnnouncement(_ context.Context, body, createdBy string, now time.Time) (matchAnnouncement, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextAnnouncementID++
	item := matchAnnouncement{
		ID:        s.nextAnnouncementID,
		Body:      body,
		CreatedBy: createdBy,
		CreatedAt: now.UTC().Format(time.RFC3339),
	}
	s.announcements = append(s.announcements, item)
	return item, nil
}

func (s *memoryStore) DeleteAnnouncement(_ context.Context, id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, item := range s.announcements {
		if item.ID == id {
			s.announcements = append(s.announcements[:i], s.announcements[i+1:]...)
			return nil
		}
	}
	return ErrResourceNotFound
}

func (s *memoryStore) ImportAdminTeams(ctx context.Context, teams []adminBulkImportTeam, now time.Time) (adminBulkImportResult, error) {
	result := adminBulkImportResult{
		Teams:   make([]adminTeam, 0),
		Players: make([]adminPlayer, 0),
		Errors:  make([]string, 0),
	}

	for index, teamInput := range teams {
		name := strings.TrimSpace(teamInput.Name)
		email := strings.TrimSpace(teamInput.ContactEmail)
		if name == "" || email == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("row %d: team name and contact_email are required", index+1))
			continue
		}

		players := make([]adminCreatePlayerRequest, 0, len(teamInput.Players))
		playerFailed := false
		for playerIndex, playerInput := range teamInput.Players {
			displayName := strings.TrimSpace(playerInput.DisplayName)
			playerEmail := strings.TrimSpace(playerInput.Email)
			password := strings.TrimSpace(playerInput.Password)
			role := strings.TrimSpace(playerInput.Role)
			if role == "" {
				role = "member"
			}
			if displayName == "" || playerEmail == "" || password == "" {
				result.Errors = append(result.Errors, fmt.Sprintf("row %d player %d: display_name, email, and password are required", index+1, playerIndex+1))
				playerFailed = true
				break
			}
			players = append(players, adminCreatePlayerRequest{
				DisplayName: displayName,
				Email:       playerEmail,
				Password:    password,
				Role:        role,
			})
		}
		if playerFailed {
			continue
		}

		team, err := s.CreateAdminTeam(ctx, adminCreateTeamRequest{Name: name, ContactEmail: email})
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("row %d (%s): %v", index+1, name, err))
			continue
		}

		createdPlayers := make([]adminPlayer, 0, len(players))
		failed := false
		for _, playerInput := range players {
			playerInput.TeamID = team.ID
			player, err := s.CreateAdminPlayer(ctx, playerInput, now)
			if err != nil {
				_ = s.DeleteAdminTeam(ctx, team.ID)
				result.Errors = append(result.Errors, fmt.Sprintf("row %d (%s): %v", index+1, name, err))
				failed = true
				break
			}
			createdPlayers = append(createdPlayers, player)
		}
		if failed {
			continue
		}

		result.TeamsCreated++
		result.PlayersCreated += len(createdPlayers)
		result.Teams = append(result.Teams, team)
		result.Players = append(result.Players, createdPlayers...)
	}

	return result, nil
}
