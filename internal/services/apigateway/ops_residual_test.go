package apigateway

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestOpsFeatureHandlerGuards(t *testing.T) {
	store := NewMemoryStore(101)
	env, token := residualMux(t, store, nil)
	const admin = "dev-admin-token"
	if response := env.do(http.MethodPost, "/api/v2/admin/players", admin, adminCreatePlayerRequest{DisplayName: "Org", Email: "org-ops@residual.test", Password: "residual-password", Role: "organizer"}); response.Code >= 300 {
		t.Fatalf("organizer setup: %d", response.Code)
	}
	orgAuth := decodeCompat[map[string]any](t, env.do(http.MethodPost, "/api/v2/authenticate", "", authenticateRequest{Email: "org-ops@residual.test", Password: "residual-password"}).Body.Bytes())
	orgToken, _ := orgAuth["token"].(string)

	tooMany := adminBulkImportRequest{Teams: make([]adminBulkImportTeam, 201)}
	for _, tc := range []struct {
		method, path, token string
		body                any
		want                int
	}{
		{http.MethodPut, "/api/v2/me/password", orgToken, participantChangePasswordRequest{CurrentPassword: "residual-password", NewPassword: "another-password"}, http.StatusForbidden},
		{http.MethodPut, "/api/v2/me/password", token, "nope", http.StatusBadRequest},
		{http.MethodPut, "/api/v2/me/password", token, participantChangePasswordRequest{CurrentPassword: "residual-password"}, http.StatusBadRequest},
		{http.MethodPut, "/api/v2/me/password", token, participantChangePasswordRequest{CurrentPassword: "residual-password", NewPassword: "short"}, http.StatusBadRequest},
		{http.MethodPost, "/api/v2/admin/announcements", admin, "nope", http.StatusBadRequest},
		{http.MethodPost, "/api/v2/admin/import/teams", admin, tooMany, http.StatusBadRequest},
	} {
		if response := env.do(tc.method, tc.path, tc.token, tc.body); response.Code != tc.want {
			t.Errorf("%s %s: want %d, got %d %s", tc.method, tc.path, tc.want, response.Code, response.Body.String())
		}
	}

	// The per-player limit applies after the per-client one.
	limited, limitedToken := residualMux(t, NewMemoryStore(101), func(s *Server) { s.WithRateLimiter(prefixLimiter{prefix: "password-change:user:"}) })
	if response := limited.do(http.MethodPut, "/api/v2/me/password", limitedToken, participantChangePasswordRequest{CurrentPassword: "residual-password", NewPassword: "another-password"}); response.Code != http.StatusTooManyRequests {
		t.Errorf("per-player password limit: %d", response.Code)
	}
}

func TestOpsFeatureStoreRulesOnBothStores(t *testing.T) {
	for name, backend := range map[string]func(*testing.T) e2eBackend{"memory": memoryBackend, "postgres": postgresBackend} {
		t.Run(name, func(t *testing.T) {
			store := backend(t).store
			ctx := context.Background()
			now := time.Now()
			team, err := store.CreateAdminTeam(ctx, adminCreateTeamRequest{Name: "Ops Team", ContactEmail: "ops-team@example.test"})
			if err != nil {
				t.Fatal(err)
			}
			player, err := store.CreateAdminPlayer(ctx, adminCreatePlayerRequest{TeamID: team.ID, DisplayName: "Ops", Email: "ops@example.test", Password: "ops-password"}, now)
			if err != nil {
				t.Fatal(err)
			}
			organizer, err := store.CreateAdminPlayer(ctx, adminCreatePlayerRequest{DisplayName: "Org", Email: "ops-org@example.test", Password: "ops-password", Role: "organizer"}, now)
			if err != nil {
				t.Fatal(err)
			}
			for label, attempt := range map[string]func() (int, error){
				"short new password": func() (int, error) { return store.ChangeParticipantPassword(ctx, player.ID, "ops-password", "short") },
				"unknown player": func() (int, error) {
					return store.ChangeParticipantPassword(ctx, 999999, "ops-password", "changed-password")
				},
				"organizer": func() (int, error) {
					return store.ChangeParticipantPassword(ctx, organizer.ID, "ops-password", "changed-password")
				},
				"wrong current": func() (int, error) {
					return store.ChangeParticipantPassword(ctx, player.ID, "not-it", "changed-password")
				},
			} {
				if _, err := attempt(); !errors.Is(err, ErrInvalidCredentials) {
					t.Errorf("%s: want invalid credentials, got %v", label, err)
				}
			}

			result, err := store.ImportAdminTeams(ctx, []adminBulkImportTeam{
				{Name: " ", ContactEmail: "blank@example.test"},
				{Name: "Ops Team", ContactEmail: "dup-team@example.test"},
				{Name: "Clashing", ContactEmail: "clash@example.test", Players: []adminBulkImportPlayer{{DisplayName: "Dup", Email: "ops@example.test", Password: "import-password"}}},
				{Name: "Imported", ContactEmail: "imported@example.test", Players: []adminBulkImportPlayer{{DisplayName: "New", Email: "imported-player@example.test", Password: "import-password"}}},
			}, now)
			if err != nil {
				t.Fatal(err)
			}
			if result.TeamsCreated != 1 || result.PlayersCreated != 1 || len(result.Errors) != 3 {
				t.Fatalf("only the clean row imports: %+v", result)
			}
			for i, want := range []string{"row 1:", "row 2 (Ops Team)", "row 3 (Clashing)"} {
				if !strings.HasPrefix(result.Errors[i], want) {
					t.Errorf("error %d = %q, want prefix %q", i, result.Errors[i], want)
				}
			}
			teams, err := store.ListAdminTeams(ctx)
			if err != nil {
				t.Fatal(err)
			}
			for _, listed := range teams {
				if listed.Name == "Clashing" {
					t.Fatal("a row whose player fails must roll its team back")
				}
			}
		})
	}
}
