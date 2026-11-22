package apigateway

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"adplatform/internal/platform/database"
	"adplatform/internal/platform/database/dbtest"
	"adplatform/internal/platform/httpapi"
)

type e2eClient struct {
	t   *testing.T
	mux *http.ServeMux
}

func (c e2eClient) do(method, path, token string, body any) *httptest.ResponseRecorder {
	c.t.Helper()
	var payload bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&payload).Encode(body); err != nil {
			c.t.Fatal(err)
		}
	}
	request := httptest.NewRequest(method, path, &payload)
	request.RemoteAddr = "198.51.100.10:4000"
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	c.mux.ServeHTTP(recorder, request)
	return recorder
}

// expect asserts the status and decodes a JSON object body when present.
func (c e2eClient) expect(want int, method, path, token string, body any) map[string]any {
	c.t.Helper()
	response := c.do(method, path, token, body)
	if response.Code != want {
		c.t.Fatalf("%s %s: want %d, got %d: %s", method, path, want, response.Code, response.Body.String())
	}
	decoded := map[string]any{}
	if strings.HasPrefix(strings.TrimSpace(response.Body.String()), "{") {
		_ = json.Unmarshal(response.Body.Bytes(), &decoded)
	}
	return decoded
}

func (c e2eClient) list(method, path, token string) []map[string]any {
	c.t.Helper()
	response := c.do(method, path, token, nil)
	if response.Code != http.StatusOK {
		c.t.Fatalf("%s %s: want 200, got %d: %s", method, path, response.Code, response.Body.String())
	}
	var items []map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &items); err != nil {
		c.t.Fatalf("%s %s: decode list: %v (%s)", method, path, err, response.Body.String())
	}
	return items
}

func newPostgresE2E(t *testing.T) (e2eClient, *sql.DB, Store) {
	t.Helper()
	db := dbtest.IsolatedSchema(t, "TEST_POSTGRES_DSN", "gateway_e2e_test")
	t.Setenv("WIREGUARD_SERVER_PUBLIC_KEY", "e2e-server-public-key")
	if err := database.RunEmbeddedMigrationsWithOptions(context.Background(), db, database.MigrationOptions{IncludeSeeds: false}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	store := NewPostgresStore(db)
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "api-gateway", Version: "test", Addr: ":0"})
	// Destructive deletes require game-core to confirm no match is active.
	NewWithDeps("e2e-team-secret", "e2e-admin-token", 101, store, storeBackedControllerClient{store: store}, noopWireGuardClient{}, testGameCoreClient{match: GameMatchStatus{State: "not_started"}}).
		WithSSHCredentialSecret("e2e-ssh-secret").
		RegisterRoutes(mux)
	return e2eClient{t: t, mux: mux}, db, store
}

// e2eBackend adapts the lifecycle tests to a store backend: the few steps the
// HTTP API cannot drive (publishing, match state, capture grants) go here.
type e2eBackend struct {
	client         e2eClient
	store          Store
	publishAll     func()
	preMatch       func()
	startMatch     func()
	unlock         func(teamID, challengeID int)
	challengeCount func() int
}

func postgresBackend(t *testing.T) e2eBackend {
	t.Helper()
	c, db, store := newPostgresE2E(t)
	exec := func(statement string, args ...any) {
		t.Helper()
		if _, err := db.Exec(statement, args...); err != nil {
			t.Fatal(err)
		}
	}
	return e2eBackend{
		client: c, store: store,
		publishAll: func() { exec(`UPDATE challenges SET published = TRUE`) },
		preMatch:   func() {},
		startMatch: func() {
			exec(`INSERT INTO game_match_state(singleton, state) VALUES (TRUE, 'running') ON CONFLICT (singleton) DO UPDATE SET state = 'running'; INSERT INTO game_ticks(id, status) VALUES (1, 'completed')`)
		},
		unlock: func(teamID, _ int) { exec(`UPDATE team_service_states SET unlocked = TRUE WHERE team_id = $1`, teamID) },
		challengeCount: func() int {
			var count int
			if err := db.QueryRow(`SELECT COUNT(*) FROM challenges`).Scan(&count); err != nil {
				t.Fatal(err)
			}
			return count
		},
	}
}

func memoryBackend(t *testing.T) e2eBackend {
	t.Helper()
	store := NewMemoryStore(101).(*memoryStore)
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "api-gateway", Version: "test", Addr: ":0"})
	NewWithDeps("e2e-team-secret", "e2e-admin-token", 101, store, storeBackedControllerClient{store: store}, noopWireGuardClient{}, testGameCoreClient{match: GameMatchStatus{State: "not_started"}}).
		WithSSHCredentialSecret("e2e-ssh-secret").
		RegisterRoutes(mux)
	return e2eBackend{
		client: e2eClient{t: t, mux: mux}, store: store,
		publishAll: func() {},
		preMatch:   func() { store.setMatchStartedForTest(false) },
		startMatch: func() { store.setMatchStartedForTest(true) },
		unlock: func(teamID, challengeID int) {
			if _, err := store.UnlockService(context.Background(), teamID, challengeID); err != nil {
				t.Fatal(err)
			}
		},
		challengeCount: func() int {
			challenges, err := store.ListAdminChallenges(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			return len(challenges)
		},
	}
}

func TestGatewayLifecyclesOnBothStores(t *testing.T) {
	for name, backend := range map[string]func(*testing.T) e2eBackend{"memory": memoryBackend, "postgres": postgresBackend} {
		t.Run(name+"/admin", func(t *testing.T) { gatewayAdminLifecycle(t, backend(t)) })
		t.Run(name+"/participant", func(t *testing.T) { gatewayParticipantLifecycle(t, backend(t)) })
	}
}

func idOf(t *testing.T, value map[string]any) int {
	t.Helper()
	id, ok := value["id"].(float64)
	if !ok || id <= 0 {
		t.Fatalf("missing id in %v", value)
	}
	return int(id)
}

func gatewayAdminLifecycle(t *testing.T, b e2eBackend) {
	c := b.client
	const admin = "e2e-admin-token"
	baseTeams := len(c.list(http.MethodGet, "/api/v2/admin/teams", admin))
	baseChallenges := b.challengeCount()
	basePlayers := len(c.list(http.MethodGet, "/api/v2/admin/players", admin))
	baseAnnouncements := len(c.list(http.MethodGet, "/api/v2/announcements", ""))

	settings := c.expect(http.StatusOK, http.MethodGet, "/api/v2/admin/platform/settings", admin, nil)
	if settings["flag_format_prefix"] == nil {
		t.Fatalf("settings missing prefix: %v", settings)
	}
	four := 4
	c.expect(http.StatusOK, http.MethodPut, "/api/v2/admin/platform/settings", admin, adminUpdatePlatformSettingsRequest{FlagFormatPrefix: "CTF", MaxTeamMembers: &four})
	c.expect(http.StatusBadRequest, http.MethodPut, "/api/v2/admin/platform/settings", admin, adminUpdatePlatformSettingsRequest{FlagFormatPrefix: "   "})
	negative := -1
	c.expect(http.StatusBadRequest, http.MethodPut, "/api/v2/admin/platform/settings", admin, adminUpdatePlatformSettingsRequest{FlagFormatPrefix: "CTF", MaxTeamMembers: &negative})
	c.expect(http.StatusOK, http.MethodPut, "/api/v2/admin/platform/appearance", admin, adminUpdatePlatformAppearanceRequest{ThemeID: "neubrutalism"})
	if appearance := c.expect(http.StatusOK, http.MethodGet, "/api/v2/platform/appearance", "", nil); appearance["theme_id"] != "neubrutalism" {
		t.Fatalf("appearance not persisted: %v", appearance)
	}

	alpha := c.expect(http.StatusOK, http.MethodPost, "/api/v2/admin/teams", admin, adminCreateTeamRequest{Name: "Alpha", ContactEmail: "alpha@example.test", Institution: "Institut Teknologi Bandung"})
	if alpha["institution"] != "Institut Teknologi Bandung" {
		t.Fatalf("create must persist institution: %v", alpha)
	}
	bravo := c.expect(http.StatusOK, http.MethodPost, "/api/v2/admin/teams", admin, adminCreateTeamRequest{Name: "Bravo", ContactEmail: "bravo@example.test"})
	alphaID, bravoID := idOf(t, alpha), idOf(t, bravo)
	c.expect(http.StatusConflict, http.MethodPost, "/api/v2/admin/teams", admin, adminCreateTeamRequest{Name: "Alpha", ContactEmail: "other@example.test"})
	// Omitting institution on update keeps the stored value.
	if renamed := c.expect(http.StatusOK, http.MethodPut, fmt.Sprintf("/api/v2/admin/teams/%d", alphaID), admin, adminUpdateTeamRequest{Name: "Alpha Prime", ContactEmail: "alpha@example.test"}); renamed["institution"] != "Institut Teknologi Bandung" {
		t.Fatalf("update without institution must keep it: %v", renamed)
	}
	institution := "Universitas Indonesia"
	if moved := c.expect(http.StatusOK, http.MethodPut, fmt.Sprintf("/api/v2/admin/teams/%d", bravoID), admin, adminUpdateTeamRequest{Name: "Bravo", ContactEmail: "bravo@example.test", Institution: &institution}); moved["institution"] != institution {
		t.Fatalf("update must set institution: %v", moved)
	}
	institutionByTeam := map[string]any{}
	for _, row := range c.list(http.MethodGet, "/api/v2/admin/teams", admin) {
		institutionByTeam[fmt.Sprint(row["name"])] = row["institution"]
	}
	if institutionByTeam["Alpha Prime"] != "Institut Teknologi Bandung" || institutionByTeam["Bravo"] != institution {
		t.Fatalf("listed institutions = %v", institutionByTeam)
	}
	if teams := c.list(http.MethodGet, "/api/v2/admin/teams", admin); len(teams) != baseTeams+2 {
		t.Fatalf("want 2 teams, got %v", teams)
	}

	challenge := c.expect(http.StatusOK, http.MethodPost, "/api/v2/admin/challenges", admin, adminCreateChallengeRequest{Name: "web", BaselineImage: "img/web:1", CheckerImage: "img/web-checker:1", ServicePort: 8080, ServiceSubnetOctet: 7, Category: "pwn"})
	challengeID := idOf(t, challenge)
	if challenge["category"] != "pwn" || challenge["protocol"] != "tcp" {
		t.Fatalf("create must store the category and default to tcp, got %v/%v", challenge["category"], challenge["protocol"])
	}
	c.expect(http.StatusBadRequest, http.MethodPost, "/api/v2/admin/challenges", admin, adminCreateChallengeRequest{Name: "bad", BaselineImage: "img", ServicePort: 70000})
	c.expect(http.StatusBadRequest, http.MethodPost, "/api/v2/admin/challenges", admin, adminCreateChallengeRequest{Name: "bad-category", Category: "forensics"})
	// Omitting category/protocol keeps them; sending them replaces them.
	kept := c.expect(http.StatusOK, http.MethodPut, fmt.Sprintf("/api/v2/admin/challenges/%d", challengeID), admin, adminUpdateChallengeRequest{Name: "web", BaselineImage: "img/web:2", CheckerImage: "img/web-checker:2"})
	if kept["category"] != "pwn" || kept["protocol"] != "tcp" {
		t.Fatalf("update without metadata must keep it, got %v/%v", kept["category"], kept["protocol"])
	}
	webCategory, httpProtocol := "web", "http"
	updated := c.expect(http.StatusOK, http.MethodPut, fmt.Sprintf("/api/v2/admin/challenges/%d", challengeID), admin, adminUpdateChallengeRequest{Name: "web", BaselineImage: "img/web:2", CheckerImage: "img/web-checker:2", Category: &webCategory, Protocol: &httpProtocol})
	if updated["category"] != "web" || updated["protocol"] != "http" {
		t.Fatalf("update must store web/http, got %v/%v", updated["category"], updated["protocol"])
	}
	c.expect(http.StatusOK, http.MethodPost, fmt.Sprintf("/api/v2/admin/challenges/%d/deploy", challengeID), admin, nil)
	if deployments := c.list(http.MethodGet, "/api/v2/admin/deployments", admin); len(deployments) == 0 {
		t.Fatal("deploy must enqueue deployment jobs")
	}
	if challenges := c.list(http.MethodGet, "/api/v2/admin/challenges", admin); len(challenges) != baseChallenges+1 {
		t.Fatalf("want 1 challenge, got %v", challenges)
	}
	c.expect(http.StatusOK, http.MethodPost, fmt.Sprintf("/api/v2/admin/challenges/%d/maintenance", challengeID), admin, nil)
	c.expect(http.StatusOK, http.MethodPost, fmt.Sprintf("/api/v2/admin/challenges/%d/resume", challengeID), admin, nil)

	captain := c.expect(http.StatusOK, http.MethodPost, "/api/v2/admin/players", admin, adminCreatePlayerRequest{TeamID: alphaID, DisplayName: "Ada", Email: "ada@example.test", Password: "ada-password", Role: "member"})
	if captain["role"] != "captain" {
		t.Fatalf("first player on a team must be promoted to captain, got %v", captain)
	}
	member := c.expect(http.StatusOK, http.MethodPost, "/api/v2/admin/players", admin, adminCreatePlayerRequest{TeamID: alphaID, DisplayName: "Bob", Email: "bob@example.test", Password: "bob-password"})
	memberID := idOf(t, member)
	organizer := c.expect(http.StatusOK, http.MethodPost, "/api/v2/admin/players", admin, adminCreatePlayerRequest{DisplayName: "Org", Email: "org@example.test", Password: "org-password", Role: "organizer"})
	c.expect(http.StatusConflict, http.MethodPost, "/api/v2/admin/players", admin, adminCreatePlayerRequest{TeamID: alphaID, DisplayName: "Dup", Email: "ADA@example.test", Password: "dup-password"})
	c.expect(http.StatusOK, http.MethodPut, fmt.Sprintf("/api/v2/admin/players/%d", memberID), admin, adminUpdatePlayerRequest{DisplayName: "Bobby", Email: "bob@example.test", Role: "member"})
	if players := c.list(http.MethodGet, "/api/v2/admin/players", admin); len(players) != basePlayers+3 {
		t.Fatalf("want 3 players, got %d", len(players))
	}

	peer := c.expect(http.StatusOK, http.MethodGet, fmt.Sprintf("/api/v2/admin/players/%d/wireguard", memberID), admin, nil)
	rotated := c.expect(http.StatusOK, http.MethodPost, fmt.Sprintf("/api/v2/admin/players/%d/wireguard/rotate", memberID), admin, nil)
	if peer["client_public_key"] == rotated["client_public_key"] {
		t.Fatal("rotation must issue a new client key")
	}
	revoked := c.expect(http.StatusOK, http.MethodPost, fmt.Sprintf("/api/v2/admin/players/%d/wireguard/revoke", memberID), admin, nil)
	if revoked["status"] != "revoked" {
		t.Fatalf("want revoked peer, got %v", revoked["status"])
	}
	c.expect(http.StatusOK, http.MethodGet, fmt.Sprintf("/api/v2/admin/players/%d/wireguard", idOf(t, organizer)), admin, nil)

	c.expect(http.StatusOK, http.MethodPost, fmt.Sprintf("/api/v2/admin/players/%d/deactivate", memberID), admin, nil)
	c.expect(http.StatusOK, http.MethodPost, fmt.Sprintf("/api/v2/admin/players/%d/reactivate", memberID), admin, nil)
	c.expect(http.StatusOK, http.MethodPost, fmt.Sprintf("/api/v2/admin/teams/%d/deactivate", bravoID), admin, nil)
	c.expect(http.StatusOK, http.MethodPost, fmt.Sprintf("/api/v2/admin/teams/%d/reactivate", bravoID), admin, nil)

	freezeAt := time.Now().UTC().Add(-time.Minute).Format(time.RFC3339)
	unfreezeAt := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	c.expect(http.StatusOK, http.MethodPost, "/api/v2/admin/game/scoreboard/freeze", admin, scoreboardFreezeRequest{FreezeAt: freezeAt, UnfreezeAt: unfreezeAt})
	c.expect(http.StatusBadRequest, http.MethodPost, "/api/v2/admin/game/scoreboard/freeze", admin, scoreboardFreezeRequest{FreezeAt: "not-a-time"})
	if status := c.expect(http.StatusOK, http.MethodGet, "/api/v2/admin/game/scoreboard/freeze", admin, nil); status["frozen"] != true {
		t.Fatalf("freeze not active: %v", status)
	}
	if public := c.expect(http.StatusOK, http.MethodGet, "/api/v2/scoreboard/freeze", "", nil); public["frozen"] != true {
		t.Fatalf("public freeze status not active: %v", public)
	}
	c.list(http.MethodGet, "/api/v2/scoreboard", "")
	c.expect(http.StatusOK, http.MethodPost, "/api/v2/admin/game/scoreboard/unfreeze", admin, nil)

	announcement := c.expect(http.StatusOK, http.MethodPost, "/api/v2/admin/announcements", admin, createAnnouncementRequest{Body: "Tick interval is now 60s"})
	if items := c.list(http.MethodGet, "/api/v2/announcements", ""); len(items) != baseAnnouncements+1 {
		t.Fatalf("want one public announcement, got %v", items)
	}
	c.list(http.MethodGet, "/api/v2/admin/announcements", admin)
	c.expect(http.StatusOK, http.MethodDelete, fmt.Sprintf("/api/v2/admin/announcements/%d", idOf(t, announcement)), admin, nil)
	c.expect(http.StatusNotFound, http.MethodDelete, fmt.Sprintf("/api/v2/admin/announcements/%d", idOf(t, announcement)), admin, nil)

	logs := c.expect(http.StatusOK, http.MethodGet, "/api/v2/admin/audit-logs?limit=5&actor_type=admin", admin, nil)
	if total, _ := logs["total_count"].(float64); total == 0 {
		t.Fatalf("admin actions must be audited, got %v", logs)
	}

	// Deletions succeed once, then report the resource as gone.
	c.expect(http.StatusNoContent, http.MethodDelete, fmt.Sprintf("/api/v2/admin/players/%d", memberID), admin, nil)
	c.expect(http.StatusBadRequest, http.MethodDelete, fmt.Sprintf("/api/v2/admin/players/%d", memberID), admin, nil)
	c.expect(http.StatusNoContent, http.MethodDelete, fmt.Sprintf("/api/v2/admin/teams/%d", bravoID), admin, nil)
	c.expect(http.StatusNoContent, http.MethodDelete, fmt.Sprintf("/api/v2/admin/challenges/%d", challengeID), admin, nil)
	c.expect(http.StatusBadRequest, http.MethodDelete, fmt.Sprintf("/api/v2/admin/challenges/%d", challengeID), admin, nil)
	if remaining := b.challengeCount(); remaining != baseChallenges {
		t.Fatalf("challenge must be gone, count=%d want %d", remaining, baseChallenges)
	}
}

func gatewayParticipantLifecycle(t *testing.T, b e2eBackend) {
	c := b.client
	const admin = "e2e-admin-token"
	b.preMatch()

	team := c.expect(http.StatusOK, http.MethodPost, "/api/v2/admin/teams", admin, adminCreateTeamRequest{Name: "Gamma", ContactEmail: "gamma@example.test"})
	teamID := idOf(t, team)
	joinKey, _ := team["join_key"].(string)
	challenge := c.expect(http.StatusOK, http.MethodPost, "/api/v2/admin/challenges", admin, adminCreateChallengeRequest{Name: "pwn", BaselineImage: "img/pwn:1", CheckerImage: "img/pwn-checker:1"})
	challengeID := idOf(t, challenge)
	c.expect(http.StatusOK, http.MethodPost, fmt.Sprintf("/api/v2/admin/challenges/%d/deploy", challengeID), admin, nil)
	c.expect(http.StatusOK, http.MethodPost, "/api/v2/admin/deployments/reconcile", admin, nil)
	b.publishAll()

	register := func(name, email string) string {
		auth := c.expect(http.StatusOK, http.MethodPost, "/api/v2/register", "", participantRegisterRequest{DisplayName: name, Email: email, Password: "participant-password"})
		token, _ := auth["token"].(string)
		return token
	}
	first := register("Cleo", "cleo@example.test")
	c.expect(http.StatusBadRequest, http.MethodPost, "/api/v2/register", "", participantRegisterRequest{DisplayName: "Dup", Email: "CLEO@example.test", Password: "participant-password"})
	c.expect(http.StatusForbidden, http.MethodGet, "/api/v2/team/services", first, nil)

	c.expect(http.StatusForbidden, http.MethodPost, "/api/v2/me/team", first, participantJoinExistingTeamRequest{TeamKey: "TEAM-WRONG"})
	joined := c.expect(http.StatusOK, http.MethodPost, "/api/v2/me/team", first, participantJoinExistingTeamRequest{TeamKey: joinKey})
	captainToken, _ := joined["token"].(string)
	c.expect(http.StatusForbidden, http.MethodGet, "/api/v2/session", first, nil) // the pre-join token is stale
	session := c.expect(http.StatusOK, http.MethodGet, "/api/v2/session", captainToken, nil)
	if session["role"] != "captain" || int(session["team_id"].(float64)) != teamID {
		t.Fatalf("first joiner must captain the team, got %v", session)
	}

	second := register("Dax", "dax@example.test")
	memberAuth := c.expect(http.StatusOK, http.MethodPost, "/api/v2/me/team", second, participantJoinExistingTeamRequest{TeamKey: joinKey})
	memberToken, _ := memberAuth["token"].(string)
	members := c.list(http.MethodGet, "/api/v2/me/team/members", memberToken)
	if len(members) != 2 {
		t.Fatalf("want 2 members, got %v", members)
	}
	var memberID int
	for _, m := range members {
		if m["role"] == "member" {
			memberID = int(m["player_id"].(float64))
		}
	}

	// Only the captain may rename the team; a member's team fields are ignored.
	c.expect(http.StatusOK, http.MethodPut, "/api/v2/me/profile", memberToken, participantUpdateProfileRequest{DisplayName: "Dax II", Email: "dax@example.test", TeamName: "Hijacked"})
	renamed := c.expect(http.StatusOK, http.MethodPut, "/api/v2/me/profile", captainToken, participantUpdateProfileRequest{DisplayName: "Cleo", Email: "cleo@example.test", TeamName: "Gamma Prime", TeamContactEmail: "gamma@example.test"})
	captainToken, _ = renamed["token"].(string)
	if s := c.expect(http.StatusOK, http.MethodGet, "/api/v2/session", captainToken, nil); s["team_name"] != "Gamma Prime" {
		t.Fatalf("captain rename not applied: %v", s)
	}
	c.expect(http.StatusBadRequest, http.MethodPut, "/api/v2/me/profile", captainToken, participantUpdateProfileRequest{DisplayName: "Cleo\nInjected", Email: "cleo@example.test"})

	c.expect(http.StatusForbidden, http.MethodPut, "/api/v2/me/password", captainToken, participantChangePasswordRequest{CurrentPassword: "wrong-password", NewPassword: "new-participant-password"})
	changed := c.expect(http.StatusOK, http.MethodPut, "/api/v2/me/password", captainToken, participantChangePasswordRequest{CurrentPassword: "participant-password", NewPassword: "new-participant-password"})
	c.expect(http.StatusForbidden, http.MethodGet, "/api/v2/session", captainToken, nil) // password change revokes old sessions
	captainToken, _ = changed["token"].(string)
	c.expect(http.StatusOK, http.MethodPost, "/api/v2/authenticate", "", authenticateRequest{Email: "cleo@example.test", Password: "new-participant-password"})

	c.expect(http.StatusForbidden, http.MethodPost, "/api/v2/me/team/captain", memberToken, transferTeamCaptainRequest{PlayerID: memberID})
	transferred := c.expect(http.StatusOK, http.MethodPost, "/api/v2/me/team/captain", captainToken, transferTeamCaptainRequest{PlayerID: memberID})
	formerCaptain, _ := transferred["token"].(string)
	if s := c.expect(http.StatusOK, http.MethodGet, "/api/v2/session", formerCaptain, nil); s["role"] != "member" {
		t.Fatalf("former captain must be demoted, got %v", s)
	}
	c.expect(http.StatusForbidden, http.MethodGet, "/api/v2/session", memberToken, nil) // promotion revokes the old member token

	// Pre-match the catalog stays dark; once running, services open up.
	if items := c.list(http.MethodGet, "/api/v2/challenges", ""); len(items) != 0 {
		t.Fatalf("challenges must stay hidden before the match starts, got %v", items)
	}
	b.startMatch()
	if items := c.list(http.MethodGet, "/api/v2/challenges", ""); !containsID(items, "id", challengeID) {
		t.Fatalf("want the published challenge once running, got %v", items)
	}
	for _, item := range c.list(http.MethodGet, "/api/v2/challenges", "") {
		// Created without metadata: uncategorized, reached over raw TCP.
		if fmt.Sprint(item["id"]) == fmt.Sprint(challengeID) && (item["category"] != "" || item["protocol"] != "tcp") {
			t.Fatalf("public catalog must carry the default category and protocol, got %v", item)
		}
	}
	c.expect(http.StatusOK, http.MethodGet, "/api/v2/services", formerCaptain, nil)
	services := c.list(http.MethodGet, "/api/v2/team/services", formerCaptain)
	if !containsID(services, "challenge_id", challengeID) {
		t.Fatalf("the team must have the deployed service, got %v", services)
	}

	sshPath := fmt.Sprintf("/api/v2/services/%d/ssh-session", challengeID)
	c.expect(http.StatusBadRequest, http.MethodPost, sshPath, formerCaptain, nil) // locked until a capture
	b.unlock(teamID, challengeID)
	ssh := c.expect(http.StatusOK, http.MethodPost, sshPath, formerCaptain, nil)
	task, err := b.store.GetControllerRuntimeTask(context.Background(), teamID, challengeID)
	if err != nil {
		t.Fatal(err)
	}
	if ssh["password"] != stableRootPassword("e2e-ssh-secret", teamID, challengeID, task.StateVolume) {
		t.Fatalf("ssh credential must derive from the ssh secret, got %v", ssh["password"])
	}
	c.expect(http.StatusOK, http.MethodPost, fmt.Sprintf("/api/v2/services/%d/reset/restart", challengeID), formerCaptain, nil)

	// Deactivating the team cuts every member off immediately.
	c.expect(http.StatusOK, http.MethodPost, fmt.Sprintf("/api/v2/admin/teams/%d/deactivate", teamID), admin, nil)
	if denied := c.do(http.MethodGet, "/api/v2/session", formerCaptain, nil); denied.Code != http.StatusForbidden || !strings.Contains(denied.Body.String(), "deactivated") {
		t.Fatalf("deactivated team session must be refused, got %d %s", denied.Code, denied.Body.String())
	}
	c.expect(http.StatusForbidden, http.MethodPost, "/api/v2/authenticate", "", authenticateRequest{Email: "cleo@example.test", Password: "new-participant-password"})
	c.expect(http.StatusOK, http.MethodPost, fmt.Sprintf("/api/v2/admin/teams/%d/reactivate", teamID), admin, nil)
	c.expect(http.StatusForbidden, http.MethodGet, "/api/v2/session", formerCaptain, nil)
	login := c.expect(http.StatusOK, http.MethodPost, "/api/v2/authenticate", "", authenticateRequest{Email: "cleo@example.test", Password: "new-participant-password"})
	reactivated, _ := login["token"].(string)
	c.expect(http.StatusOK, http.MethodGet, "/api/v2/session", reactivated, nil)
	secondLogin := c.expect(http.StatusOK, http.MethodPost, "/api/v2/authenticate", "", authenticateRequest{Email: "cleo@example.test", Password: "new-participant-password"})
	otherSession, _ := secondLogin["token"].(string)
	profile := c.expect(http.StatusOK, http.MethodPut, "/api/v2/me/profile", reactivated, participantUpdateProfileRequest{DisplayName: "Cleo Updated", Email: "cleo@example.test"})
	renewed, _ := profile["token"].(string)
	c.expect(http.StatusNoContent, http.MethodPost, "/api/v2/session/logout", reactivated, nil)
	c.expect(http.StatusForbidden, http.MethodGet, "/api/v2/session", renewed, nil)
	c.expect(http.StatusForbidden, http.MethodGet, "/api/v2/session", reactivated, nil)
	c.expect(http.StatusOK, http.MethodGet, "/api/v2/session", otherSession, nil)
}

func containsID(items []map[string]any, key string, id int) bool {
	for _, item := range items {
		if value, ok := item[key].(float64); ok && int(value) == id {
			return true
		}
	}
	return false
}
