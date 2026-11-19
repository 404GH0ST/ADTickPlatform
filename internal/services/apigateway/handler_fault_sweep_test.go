package apigateway

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"adplatform/internal/platform/httpapi"
)

type sweepEnv struct {
	t       *testing.T
	mux     *http.ServeMux
	faults  *storeFaults
	captain string
}

func (e *sweepEnv) do(method, path, token string, body any) *httptest.ResponseRecorder {
	var payload bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&payload).Encode(body)
	}
	request := httptest.NewRequest(method, path, &payload)
	request.RemoteAddr = "198.51.100.77:5000"
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	e.mux.ServeHTTP(recorder, request)
	return recorder
}

// register creates a fresh participant on a fresh team (so member limits
// never interfere) and returns its token, player ID and team ID. Faults are
// disarmed during setup and re-armed with a zeroed counter afterwards.
func (e *sweepEnv) register(run int, label string) (string, int, int) {
	e.t.Helper()
	armed := e.faults.failAt
	e.faults.arm(0)
	defer e.faults.arm(armed)
	team := decodeCompat[map[string]any](e.t, e.do(http.MethodPost, "/api/v2/admin/teams", "dev-admin-token", adminCreateTeamRequest{Name: fmt.Sprintf("%s team %d", label, run), ContactEmail: fmt.Sprintf("%s-team-%d@sweep.test", label, run)}).Body.Bytes())
	key, _ := team["join_key"].(string)
	teamID, _ := team["id"].(float64)
	email := fmt.Sprintf("%s-%d@sweep.test", label, run)
	auth := decodeCompat[map[string]any](e.t, e.do(http.MethodPost, "/api/v2/register", "", participantRegisterRequest{DisplayName: label, Email: email, Password: "sweep-password"}).Body.Bytes())
	token, _ := auth["token"].(string)
	joined := decodeCompat[map[string]any](e.t, e.do(http.MethodPost, "/api/v2/me/team", token, participantJoinExistingTeamRequest{TeamKey: key}).Body.Bytes())
	token, _ = joined["token"].(string)
	session := decodeCompat[map[string]any](e.t, e.do(http.MethodGet, "/api/v2/session", token, nil).Body.Bytes())
	id, _ := session["player_id"].(float64)
	if token == "" || id == 0 {
		e.t.Fatalf("sweep setup could not register %s-%d", label, run)
	}
	return token, int(id), int(teamID)
}

var errMalformedResponse = errors.New("malformed response")

// check fails a malformed body always, and any non-2xx status on a clean run
// so an operation cannot silently stop short of the code it means to sweep.
func (e *sweepEnv) check(response *httptest.ResponseRecorder) error {
	body := strings.TrimSpace(response.Body.String())
	if body != "" && !json.Valid([]byte(body)) && !strings.HasPrefix(response.Header().Get("Content-Type"), "text/plain") {
		return fmt.Errorf("%w: %q", errMalformedResponse, body)
	}
	if e.faults.failAt == 0 && response.Code >= 400 {
		return fmt.Errorf("status %d: %s", response.Code, body)
	}
	return nil
}

func TestHandlersSurviveStoreFaults(t *testing.T) {
	store := NewMemoryStore(101)
	faults := &storeFaults{}
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "api-gateway", Version: "test", Addr: ":0"})
	NewWithDeps("dev-team-token", "dev-admin-token", 101, faultingStore{inner: store, faults: faults}, storeBackedControllerClient{store: store}, noopWireGuardClient{}, testGameCoreClient{match: GameMatchStatus{State: "not_started"}}).
		WithSSHCredentialSecret("sweep-ssh-secret").
		// Cooldowns would turn every repeat of an action into a 429 before it
		// reaches the store, so the sweep runs without them.
		WithRateLimiter(testRateLimiter{}).
		RegisterRoutes(mux)
	env := &sweepEnv{t: t, mux: mux, faults: faults}
	const admin = "dev-admin-token"
	for id := 1; id <= 3; id++ {
		if _, err := store.UnlockService(t.Context(), 101, id); err != nil {
			t.Fatal(err)
		}
	}

	ops := map[string]func(run int) error{
		"register": func(run int) error {
			return env.check(env.do(http.MethodPost, "/api/v2/register", "", participantRegisterRequest{DisplayName: "R", Email: fmt.Sprintf("reg-%d@sweep.test", run), Password: "sweep-password"}))
		},
		"authenticate": func(run int) error {
			env.register(run, "auth")
			return env.check(env.do(http.MethodPost, "/api/v2/authenticate", "", authenticateRequest{Email: fmt.Sprintf("auth-%d@sweep.test", run), Password: "sweep-password"}))
		},
		"participant reads": func(run int) error {
			token, _, _ := env.register(run, "reader")
			for _, path := range []string{"/api/v2/session", "/api/v2/me/team/members", "/api/v2/services", "/api/v2/team/services", "/api/v2/me/wireguard", "/api/v2/challenges", "/api/v2/scoreboard", "/api/v2/scoreboard/freeze", "/api/v2/attacks", "/api/v2/announcements", "/api/v2/platform/appearance", "/api/v2/game/status"} {
				if err := env.check(env.do(http.MethodGet, path, token, nil)); err != nil {
					return fmt.Errorf("%s: %w", path, err)
				}
			}
			return nil
		},
		"profile and password": func(run int) error {
			token, _, _ := env.register(run, "profile")
			email := fmt.Sprintf("profile-%d@sweep.test", run)
			if err := env.check(env.do(http.MethodPut, "/api/v2/me/profile", token, participantUpdateProfileRequest{DisplayName: "Renamed", Email: email, TeamName: fmt.Sprintf("profile team %d renamed", run), TeamContactEmail: fmt.Sprintf("profile-team-%d@sweep.test", run)})); err != nil {
				return err
			}
			return env.check(env.do(http.MethodPut, "/api/v2/me/password", token, participantChangePasswordRequest{CurrentPassword: "sweep-password", NewPassword: "sweep-password-2"}))
		},
		"captain transfer": func(run int) error {
			auth := decodeCompat[map[string]any](t, env.do(http.MethodPost, "/api/v2/admin/teams", admin, adminCreateTeamRequest{Name: fmt.Sprintf("Sweep %d", run), ContactEmail: fmt.Sprintf("sweep-team-%d@sweep.test", run)}).Body.Bytes())
			key, _ := auth["join_key"].(string)
			lead, member := "", 0
			for i, label := range []string{"lead", "mate"} {
				reg := decodeCompat[map[string]any](t, env.do(http.MethodPost, "/api/v2/register", "", participantRegisterRequest{DisplayName: label, Email: fmt.Sprintf("%s-%d@sweep.test", label, run), Password: "sweep-password"}).Body.Bytes())
				token, _ := reg["token"].(string)
				joined := decodeCompat[map[string]any](t, env.do(http.MethodPost, "/api/v2/me/team", token, participantJoinExistingTeamRequest{TeamKey: key}).Body.Bytes())
				token, _ = joined["token"].(string)
				if i == 0 {
					lead = token
				} else {
					session := decodeCompat[map[string]any](t, env.do(http.MethodGet, "/api/v2/session", token, nil).Body.Bytes())
					id, _ := session["player_id"].(float64)
					member = int(id)
				}
			}
			return env.check(env.do(http.MethodPost, "/api/v2/me/team/captain", lead, transferTeamCaptainRequest{PlayerID: member}))
		},
		"service actions": func(run int) error {
			token, _, teamID := env.register(run, "operator")
			// A new team's services are queued until the controller reconciles.
			if _, err := store.ReconcileAdminDeployments(t.Context(), time.Now()); err != nil {
				return err
			}
			for id := 1; id <= 3; id++ {
				if _, err := store.UnlockService(t.Context(), teamID, id); err != nil {
					return err
				}
			}
			for _, path := range []string{"/api/v2/services/1/ssh-session", "/api/v2/services/2/reset/restart", "/api/v2/services/3/reset/factory"} {
				if err := env.check(env.do(http.MethodPost, path, token, nil)); err != nil {
					return fmt.Errorf("%s: %w", path, err)
				}
			}
			return nil
		},
		"admin registry": func(run int) error {
			team := decodeCompat[map[string]any](t, env.do(http.MethodPost, "/api/v2/admin/teams", admin, adminCreateTeamRequest{Name: fmt.Sprintf("Reg %d", run), ContactEmail: fmt.Sprintf("reg-team-%d@sweep.test", run)}).Body.Bytes())
			teamID, _ := team["id"].(float64)
			player := decodeCompat[map[string]any](t, env.do(http.MethodPost, "/api/v2/admin/players", admin, adminCreatePlayerRequest{TeamID: int(teamID), DisplayName: "P", Email: fmt.Sprintf("reg-player-%d@sweep.test", run), Password: "sweep-password"}).Body.Bytes())
			playerID, _ := player["id"].(float64)
			steps := []struct {
				method, path string
				body         any
			}{
				{http.MethodGet, "/api/v2/admin/teams", nil},
				{http.MethodGet, "/api/v2/admin/players", nil},
				{http.MethodPut, fmt.Sprintf("/api/v2/admin/teams/%d", int(teamID)), adminUpdateTeamRequest{Name: fmt.Sprintf("Reg %d b", run), ContactEmail: fmt.Sprintf("reg-team-%d-b@sweep.test", run)}},
				{http.MethodPut, fmt.Sprintf("/api/v2/admin/players/%d", int(playerID)), adminUpdatePlayerRequest{DisplayName: "Q", Email: fmt.Sprintf("reg-player-%d@sweep.test", run), Role: "captain"}},
				{http.MethodGet, fmt.Sprintf("/api/v2/admin/players/%d/wireguard", int(playerID)), nil},
				{http.MethodPost, fmt.Sprintf("/api/v2/admin/players/%d/wireguard/rotate", int(playerID)), nil},
				{http.MethodPost, fmt.Sprintf("/api/v2/admin/players/%d/wireguard/revoke", int(playerID)), nil},
				{http.MethodPost, fmt.Sprintf("/api/v2/admin/players/%d/deactivate", int(playerID)), nil},
				{http.MethodPost, fmt.Sprintf("/api/v2/admin/players/%d/reactivate", int(playerID)), nil},
				{http.MethodPost, fmt.Sprintf("/api/v2/admin/teams/%d/deactivate", int(teamID)), nil},
				{http.MethodPost, fmt.Sprintf("/api/v2/admin/teams/%d/reactivate", int(teamID)), nil},
				{http.MethodDelete, fmt.Sprintf("/api/v2/admin/players/%d", int(playerID)), nil},
				{http.MethodDelete, fmt.Sprintf("/api/v2/admin/teams/%d", int(teamID)), nil},
			}
			for _, step := range steps {
				if err := env.check(env.do(step.method, step.path, admin, step.body)); err != nil {
					return fmt.Errorf("%s %s: %w", step.method, step.path, err)
				}
			}
			return nil
		},
		"admin challenges": func(run int) error {
			challenge := decodeCompat[map[string]any](t, env.do(http.MethodPost, "/api/v2/admin/challenges", admin, adminCreateChallengeRequest{Name: fmt.Sprintf("chal-%d", run), BaselineImage: "img", CheckerImage: "chk"}).Body.Bytes())
			created, ok := challenge["id"].(float64)
			if !ok {
				return fmt.Errorf("challenge create failed: %v", challenge)
			}
			id := int(created)
			for _, step := range []struct {
				method, path string
				body         any
			}{
				{http.MethodGet, "/api/v2/admin/challenges", nil},
				{http.MethodPut, fmt.Sprintf("/api/v2/admin/challenges/%d", id), adminUpdateChallengeRequest{Name: fmt.Sprintf("chal-%d", run), BaselineImage: "img2", CheckerImage: "chk"}},
				{http.MethodPost, fmt.Sprintf("/api/v2/admin/challenges/%d/validate", id), nil},
				{http.MethodPost, fmt.Sprintf("/api/v2/admin/challenges/%d/deploy", id), nil},
				{http.MethodGet, "/api/v2/admin/deployments", nil},
				{http.MethodPost, "/api/v2/admin/deployments/reconcile", nil},
				{http.MethodPost, fmt.Sprintf("/api/v2/admin/challenges/%d/maintenance", id), nil},
				{http.MethodPost, fmt.Sprintf("/api/v2/admin/challenges/%d/resume", id), nil},
				{http.MethodDelete, fmt.Sprintf("/api/v2/admin/challenges/%d", id), nil},
			} {
				if err := env.check(env.do(step.method, step.path, admin, step.body)); err != nil {
					return fmt.Errorf("%s %s: %w", step.method, step.path, err)
				}
			}
			return nil
		},
		"admin platform": func(run int) error {
			for _, step := range []struct {
				method, path string
				body         any
			}{
				{http.MethodGet, "/api/v2/admin/platform/settings", nil},
				{http.MethodPut, "/api/v2/admin/platform/settings", adminUpdatePlatformSettingsRequest{FlagFormatPrefix: "PLAYIT"}},
				{http.MethodPut, "/api/v2/admin/platform/appearance", adminUpdatePlatformAppearanceRequest{ThemeID: "neubrutalism"}},
				{http.MethodPost, "/api/v2/admin/game/scoreboard/freeze", scoreboardFreezeRequest{FreezeAt: "2099-01-01T00:00:00Z"}},
				{http.MethodGet, "/api/v2/admin/game/scoreboard/freeze", nil},
				{http.MethodPost, "/api/v2/admin/game/scoreboard/unfreeze", nil},
				{http.MethodGet, "/api/v2/admin/audit-logs?limit=5", nil},
				{http.MethodPost, "/api/v2/admin/announcements", createAnnouncementRequest{Body: fmt.Sprintf("note %d", run)}},
				{http.MethodGet, "/api/v2/admin/announcements", nil},
				{http.MethodPost, "/api/v2/admin/import/teams", adminBulkImportRequest{Teams: []adminBulkImportTeam{{Name: fmt.Sprintf("Imp %d", run), ContactEmail: fmt.Sprintf("imp-%d@sweep.test", run)}}}},
				{http.MethodGet, "/api/v2/admin/operations/status", nil},
				{http.MethodGet, "/api/v2/admin/access/status", nil},
			} {
				if err := env.check(env.do(step.method, step.path, admin, step.body)); err != nil {
					return fmt.Errorf("%s %s: %w", step.method, step.path, err)
				}
			}
			return nil
		},
	}
	names := make([]string, 0, len(ops))
	for name := range ops {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			op := ops[name]
			run := 0
			faults.arm(0)
			if err := op(run); err != nil {
				t.Fatalf("clean run: %v", err)
			}
			total := faults.calls()
			for position := 1; position <= total; position++ {
				run++
				faults.arm(position)
				func() {
					defer func() {
						if recovered := recover(); recovered != nil {
							t.Errorf("panic with fault at store call %d/%d: %v", position, total, recovered)
						}
					}()
					// Under a fault, 5xx is expected; only a panic or a malformed
					// body is a defect here.
					if err := op(run); errors.Is(err, errMalformedResponse) {
						t.Errorf("fault at store call %d/%d: %v", position, total, err)
					}
				}()
				run++
				faults.arm(0)
				if err := op(run); err != nil {
					t.Errorf("no recovery after fault at store call %d/%d: %v", position, total, err)
				}
			}
		})
	}
}
