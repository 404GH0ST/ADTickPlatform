package apigateway

import (
	"testing"
	"time"
)

func TestIssueAndVerifyTeamJWT(t *testing.T) {
	now := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	player := authenticatedPlayer{
		PlayerID:    1,
		TeamID:      101,
		TeamName:    "Team Alpha",
		DisplayName: "Alpha Captain",
		Email:       "alpha.captain@example.com",
		Role:        "captain",
	}

	token, err := issueTeamJWT("dev-team-token", player, now)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	claims, err := verifyTeamJWT("dev-team-token", token, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("verify token: %v", err)
	}
	if claims.TeamID != player.TeamID || claims.PlayerID != player.PlayerID || claims.Email != player.Email {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestVerifyTeamJWTRejectsExpiredToken(t *testing.T) {
	now := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	token, err := issueTeamJWT("dev-team-token", authenticatedPlayer{
		PlayerID:    1,
		TeamID:      101,
		TeamName:    "Team Alpha",
		DisplayName: "Alpha Captain",
		Email:       "alpha.captain@example.com",
		Role:        "captain",
	}, now.Add(-25*time.Hour))
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	if _, err := verifyTeamJWT("dev-team-token", token, now); err == nil {
		t.Fatal("expected expired token to be rejected")
	}
}

func TestVerifyTeamJWTAcceptsOrganizerWithoutTeam(t *testing.T) {
	now := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	token, err := issueTeamJWT("dev-team-token", authenticatedPlayer{
		PlayerID:    7,
		TeamID:      0,
		TeamName:    "Organizer",
		DisplayName: "System Admin",
		Email:       "admin@example.com",
		Role:        "organizer",
	}, now)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	claims, err := verifyTeamJWT("dev-team-token", token, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("verify organizer token: %v", err)
	}
	if claims.TeamID != 0 || claims.Role != "organizer" {
		t.Fatalf("unexpected organizer claims: %+v", claims)
	}
}

func TestVerifyTeamJWTAcceptsRegisteredParticipantWithoutTeam(t *testing.T) {
	now := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	token, err := issueTeamJWT("dev-team-token", authenticatedPlayer{
		PlayerID:    8,
		TeamID:      0,
		TeamName:    "",
		DisplayName: "Pending Player",
		Email:       "pending@example.com",
		Role:        "member",
	}, now)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	claims, err := verifyTeamJWT("dev-team-token", token, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("verify registered participant token: %v", err)
	}
	if claims.TeamID != 0 || claims.Role != "member" {
		t.Fatalf("unexpected registered participant claims: %+v", claims)
	}
}
