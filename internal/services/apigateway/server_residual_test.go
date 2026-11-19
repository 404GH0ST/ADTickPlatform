package apigateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"adplatform/internal/platform/httpapi"
)

type denyAllLimiter struct{}

func (denyAllLimiter) Allow(context.Context, string, rateLimitPolicy) (rateLimitDecision, error) {
	return rateLimitDecision{allowed: false, retryAfter: time.Second}, nil
}

func residualMux(t *testing.T, store Store, configure func(*Server)) (*sweepEnv, string) {
	t.Helper()
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "api-gateway", Version: "test", Addr: ":0"})
	server := NewWithDeps("dev-team-token", "dev-admin-token", 101, store, storeBackedControllerClient{store: store}, noopWireGuardClient{}, testGameCoreClient{match: GameMatchStatus{State: "running", AcceptingSubmissions: true}})
	if configure != nil {
		configure(server)
	}
	server.RegisterRoutes(mux)
	env := &sweepEnv{t: t, mux: mux, faults: &storeFaults{}}
	token, _ := registerOn(t, env, "resident")
	return env, token
}

// registerOn creates a participant on team 101 directly against the mux.
func registerOn(t *testing.T, env *sweepEnv, label string) (string, int) {
	t.Helper()
	auth := decodeCompat[map[string]any](t, env.do(http.MethodPost, "/api/v2/register", "", participantRegisterRequest{DisplayName: label, Email: label + "@residual.test", Password: "residual-password"}).Body.Bytes())
	token, _ := auth["token"].(string)
	joined := decodeCompat[map[string]any](t, env.do(http.MethodPost, "/api/v2/me/team", token, participantJoinExistingTeamRequest{TeamKey: "TEAM-ALPHA-JOIN"}).Body.Bytes())
	token, _ = joined["token"].(string)
	session := decodeCompat[map[string]any](t, env.do(http.MethodGet, "/api/v2/session", token, nil).Body.Bytes())
	id, _ := session["player_id"].(float64)
	return token, int(id)
}

func TestRateLimitedRoutesRejectWhenDenied(t *testing.T) {
	store := NewMemoryStore(101)
	env, token := residualMux(t, store, nil)
	denying := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "api-gateway", Version: "test", Addr: ":0"})
	NewWithDeps("dev-team-token", "dev-admin-token", 101, store, storeBackedControllerClient{store: store}, noopWireGuardClient{}).
		WithRateLimiter(denyAllLimiter{}).RegisterRoutes(denying)
	env.mux = denying
	routes := []struct {
		method, path string
		body         any
		token        string
	}{
		{http.MethodPost, "/api/v2/authenticate", authenticateRequest{Email: "a@b.c", Password: "password1"}, ""},
		{http.MethodPost, "/api/v2/register", participantRegisterRequest{DisplayName: "X", Email: "x@y.z", Password: "password1"}, ""},
		{http.MethodPost, "/api/v2/me/team", participantJoinExistingTeamRequest{TeamKey: "k"}, token},
		{http.MethodGet, "/api/v2/challenges", nil, ""},
		{http.MethodGet, "/api/v2/services", nil, token},
		{http.MethodGet, "/api/v2/scoreboard", nil, ""},
		{http.MethodGet, "/api/v2/scoreboard/freeze", nil, ""},
		{http.MethodGet, "/api/v2/game/status", nil, ""},
		{http.MethodGet, "/api/v2/attacks", nil, ""},
		{http.MethodGet, "/api/v2/team/services", nil, token},
		{http.MethodPost, "/api/v2/submit", submitRequest{Flags: []string{"x"}}, token},
		{http.MethodPost, "/api/v2/services/1/ssh-session", nil, token},
		{http.MethodGet, "/api/v2/challenges/1/source", nil, token},
		{http.MethodGet, "/api/v2/announcements", nil, ""},
		{http.MethodPut, "/api/v2/me/password", participantChangePasswordRequest{CurrentPassword: "residual-password", NewPassword: "another-password"}, token},
	}
	for _, route := range routes {
		if response := env.do(route.method, route.path, route.token, route.body); response.Code != http.StatusTooManyRequests {
			t.Errorf("%s %s: want 429, got %d %s", route.method, route.path, response.Code, response.Body.String())
		}
	}
}

func TestTokenIssuanceFailuresAreServerErrors(t *testing.T) {
	store := NewMemoryStore(101)
	env, token := residualMux(t, store, nil)
	captainEnv, captainToken, mateID := captainFixture(t)
	original := issueToken
	t.Cleanup(func() { issueToken = original })
	issueToken = func(string, authenticatedPlayer, time.Time) (string, error) { return "", errors.New("signing failed") }

	for _, tc := range []struct {
		env          *sweepEnv
		method, path string
		token        string
		body         any
	}{
		{env, http.MethodPost, "/api/v2/authenticate", "", authenticateRequest{Email: "resident@residual.test", Password: "residual-password"}},
		{env, http.MethodPost, "/api/v2/register", "", participantRegisterRequest{DisplayName: "N", Email: "new@residual.test", Password: "residual-password"}},
		{env, http.MethodPut, "/api/v2/me/profile", token, participantUpdateProfileRequest{DisplayName: "Res", Email: "resident@residual.test"}},
		{env, http.MethodPut, "/api/v2/me/password", token, participantChangePasswordRequest{CurrentPassword: "residual-password", NewPassword: "residual-password-2"}},
		{captainEnv, http.MethodPost, "/api/v2/me/team/captain", captainToken, transferTeamCaptainRequest{PlayerID: mateID}},
	} {
		if response := tc.env.do(tc.method, tc.path, tc.token, tc.body); response.Code != http.StatusInternalServerError {
			t.Errorf("%s %s: want 500, got %d %s", tc.method, tc.path, response.Code, response.Body.String())
		}
	}
	// Joining needs a fresh teamless player.
	issueToken = original
	fresh := decodeCompat[map[string]any](t, env.do(http.MethodPost, "/api/v2/register", "", participantRegisterRequest{DisplayName: "J", Email: "join@residual.test", Password: "residual-password"}).Body.Bytes())
	freshToken, _ := fresh["token"].(string)
	issueToken = func(string, authenticatedPlayer, time.Time) (string, error) { return "", errors.New("signing failed") }
	if response := env.do(http.MethodPost, "/api/v2/me/team", freshToken, participantJoinExistingTeamRequest{TeamKey: "TEAM-ALPHA-JOIN"}); response.Code != http.StatusInternalServerError {
		t.Errorf("join with failed signing: %d", response.Code)
	}
}

func captainFixture(t *testing.T) (*sweepEnv, string, int) {
	t.Helper()
	store := NewMemoryStore(101)
	env, _ := residualMux(t, store, nil)
	team := decodeCompat[map[string]any](t, env.do(http.MethodPost, "/api/v2/admin/teams", "dev-admin-token", adminCreateTeamRequest{Name: "Cap Team", ContactEmail: "cap-team@residual.test"}).Body.Bytes())
	key, _ := team["join_key"].(string)
	join := func(label string) (string, int) {
		reg := decodeCompat[map[string]any](t, env.do(http.MethodPost, "/api/v2/register", "", participantRegisterRequest{DisplayName: label, Email: label + "@cap.test", Password: "residual-password"}).Body.Bytes())
		token, _ := reg["token"].(string)
		joined := decodeCompat[map[string]any](t, env.do(http.MethodPost, "/api/v2/me/team", token, participantJoinExistingTeamRequest{TeamKey: key}).Body.Bytes())
		token, _ = joined["token"].(string)
		session := decodeCompat[map[string]any](t, env.do(http.MethodGet, "/api/v2/session", token, nil).Body.Bytes())
		id, _ := session["player_id"].(float64)
		return token, int(id)
	}
	captain, _ := join("captain")
	_, mate := join("mate")
	return env, captain, mate
}

// submitStore lets a submission reach the local store path.
type submitStore struct {
	Store
	results []submissionVerdict
	err     error
}

func (s submitStore) SubmitFlags(context.Context, int, []string) ([]submissionVerdict, error) {
	return s.results, s.err
}

func TestUpstreamFallbackFailures(t *testing.T) {
	store := NewMemoryStore(101)
	cases := []struct {
		name       string
		game       gameCoreClient
		submission submissionClient
		scoring    scoringClient
		store      Store
		method     string
		path       string
		body       any
		want       int
	}{
		{"scoreboard via scoring", failingGameCoreClient{}, noopSubmissionClient{}, testScoringClient{err: errUpstreamDown}, store, http.MethodGet, "/api/v2/scoreboard", nil, http.StatusBadGateway},
		{"scoreboard via game-core", failingGameCoreClient{}, noopSubmissionClient{}, noopScoringClient{}, store, http.MethodGet, "/api/v2/scoreboard", nil, http.StatusBadGateway},
		{"scoreboard via store", noopGameCoreClient{}, noopSubmissionClient{}, noopScoringClient{}, failingStoreExceptSessions(store), http.MethodGet, "/api/v2/scoreboard", nil, http.StatusInternalServerError},
		{"attacks via game-core", failingGameCoreClient{}, noopSubmissionClient{}, noopScoringClient{}, store, http.MethodGet, "/api/v2/attacks", nil, http.StatusBadGateway},
		{"attacks via store", noopGameCoreClient{}, noopSubmissionClient{}, noopScoringClient{}, store, http.MethodGet, "/api/v2/attacks?limit=-1&offset=-1&attacker=zz&victim=zz&service=zz", nil, http.StatusOK},
		{"attacks store failure", noopGameCoreClient{}, noopSubmissionClient{}, noopScoringClient{}, failingStoreExceptSessions(store), http.MethodGet, "/api/v2/attacks", nil, http.StatusInternalServerError},
		{"submit via submission", noopGameCoreClient{}, testSubmissionClient{err: errUpstreamDown}, noopScoringClient{}, store, http.MethodPost, "/api/v2/submit", submitRequest{Flags: []string{"x"}}, http.StatusBadGateway},
		{"submit via game-core", testGameCoreClientRunningButFailing{}, noopSubmissionClient{}, noopScoringClient{}, store, http.MethodPost, "/api/v2/submit", submitRequest{Flags: []string{"x"}}, http.StatusBadGateway},
		{"submit store failure", noopGameCoreClient{}, noopSubmissionClient{}, noopScoringClient{}, submitStore{Store: store, err: errStoreDown}, http.MethodPost, "/api/v2/submit", submitRequest{Flags: []string{"x"}}, http.StatusInternalServerError},
		{"submit store success", noopGameCoreClient{}, noopSubmissionClient{}, noopScoringClient{}, submitStore{Store: store, results: []submissionVerdict{{Flag: "x", Status: "invalid"}}}, http.MethodPost, "/api/v2/submit", submitRequest{Flags: []string{"x"}}, http.StatusOK},
		{"submit paused", testGameCoreClient{match: GameMatchStatus{State: "paused"}}, noopSubmissionClient{}, noopScoringClient{}, store, http.MethodPost, "/api/v2/submit", submitRequest{Flags: []string{"x"}}, http.StatusBadRequest},
		{"submit bad body", noopGameCoreClient{}, noopSubmissionClient{}, noopScoringClient{}, store, http.MethodPost, "/api/v2/submit", map[string]any{"flags": []string{}}, http.StatusBadRequest},
	}
	for _, tc := range cases {
		mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "api-gateway", Version: "test", Addr: ":0"})
		server := NewWithDeps("dev-team-token", "dev-admin-token", 101, tc.store, storeBackedControllerClient{store: store}, noopWireGuardClient{}, tc.game)
		server.WithSubmissionClient(tc.submission).WithScoringClient(tc.scoring).RegisterRoutes(mux)
		var payload bytes.Buffer
		if tc.body != nil {
			_ = json.NewEncoder(&payload).Encode(tc.body)
		}
		request := httptest.NewRequest(tc.method, tc.path, &payload)
		setTestTeamAuthHeader(t, request)
		response := serve(mux, request)
		if response.Code != tc.want {
			t.Errorf("%s: want %d, got %d %s", tc.name, tc.want, response.Code, response.Body.String())
		}
	}
}

// testGameCoreClientRunningButFailing reports a running match but fails submits.
type testGameCoreClientRunningButFailing struct{ failingGameCoreClient }

func (testGameCoreClientRunningButFailing) Status(context.Context) (GameStatus, error) {
	return GameStatus{Match: &GameMatchStatus{State: "running", AcceptingSubmissions: true}}, nil
}

func failingStoreExceptSessions(sessions Store) Store {
	return sessionOnlyStore{sessions: sessions}
}

func TestParticipantHandlerEdges(t *testing.T) {
	store := NewMemoryStore(101)
	env, token := residualMux(t, store, nil)
	organizer := decodeCompat[map[string]any](t, env.do(http.MethodPost, "/api/v2/admin/players", "dev-admin-token", adminCreatePlayerRequest{DisplayName: "Org", Email: "org@residual.test", Password: "residual-password", Role: "organizer"}).Body.Bytes())
	_ = organizer
	orgAuth := decodeCompat[map[string]any](t, env.do(http.MethodPost, "/api/v2/authenticate", "", authenticateRequest{Email: "org@residual.test", Password: "residual-password"}).Body.Bytes())
	orgToken, _ := orgAuth["token"].(string)
	teamless := decodeCompat[map[string]any](t, env.do(http.MethodPost, "/api/v2/register", "", participantRegisterRequest{DisplayName: "Solo", Email: "solo@residual.test", Password: "residual-password"}).Body.Bytes())
	soloToken, _ := teamless["token"].(string)
	for _, tc := range []struct {
		method, path, token string
		body                any
		want                int
	}{
		{http.MethodPost, "/api/v2/authenticate", "", "not json", http.StatusForbidden},
		{http.MethodPost, "/api/v2/authenticate", "", authenticateRequest{Email: " ", Password: "x"}, http.StatusForbidden},
		{http.MethodPost, "/api/v2/register", "", "not json", http.StatusBadRequest},
		{http.MethodPost, "/api/v2/register", "", participantRegisterRequest{DisplayName: "Bad\nName", Email: "b@c.d", Password: "residual-password"}, http.StatusBadRequest},
		{http.MethodPost, "/api/v2/me/team", token, participantJoinExistingTeamRequest{TeamKey: "TEAM-ALPHA-JOIN"}, http.StatusBadRequest},
		{http.MethodGet, "/api/v2/me/team/members", soloToken, nil, http.StatusForbidden},
		{http.MethodPut, "/api/v2/me/profile", orgToken, participantUpdateProfileRequest{DisplayName: "O", Email: "org@residual.test"}, http.StatusForbidden},
		{http.MethodPut, "/api/v2/me/profile", token, "not json", http.StatusBadRequest},
		{http.MethodPut, "/api/v2/me/profile", token, participantUpdateProfileRequest{DisplayName: "R", Email: "org@residual.test"}, http.StatusBadRequest},
		{http.MethodPost, "/api/v2/submit", orgToken, submitRequest{Flags: []string{"x"}}, http.StatusForbidden},
	} {
		if response := env.do(tc.method, tc.path, tc.token, tc.body); response.Code != tc.want {
			t.Errorf("%s %s: want %d, got %d %s", tc.method, tc.path, tc.want, response.Code, response.Body.String())
		}
	}
	noTeam := sessionWithTeamGone{Store: NewMemoryStore(101)}
	envGone, goneToken := residualMux(t, noTeam, nil)
	if response := envGone.do(http.MethodPut, "/api/v2/me/profile", goneToken, participantUpdateProfileRequest{DisplayName: "R", Email: "resident@residual.test"}); response.Code != http.StatusBadRequest {
		t.Errorf("a profile edit whose team vanished: %d", response.Code)
	}
}

type sessionWithTeamGone struct{ Store }

func (sessionWithTeamGone) UpdateParticipantProfile(context.Context, int, participantUpdateProfileRequest) (authenticatedPlayer, error) {
	return authenticatedPlayer{}, ErrTeamNotFound
}

func TestParticipantSLAHelpers(t *testing.T) {
	for phase, want := range map[string]int{"put": 0, " GET ": 1, "check": 2, "other": 99} {
		if got := phaseOrder(phase); got != want {
			t.Errorf("phaseOrder(%q) = %d", phase, got)
		}
	}
	if fallbackSLAStatus("warning") != "down" || fallbackSLAStatus("") != "unknown" || fallbackSLAStatus("passing") != "ok" {
		t.Fatal("fallback SLA status")
	}
	if !strings.Contains(fallbackSLAMessage("Warning"), "warning") || !strings.Contains(fallbackSLAMessage(""), "awaiting") {
		t.Fatal("fallback SLA message")
	}
	for _, summary := range []GameServiceStateSummary{
		{Phase: "put"}, {Phase: "get"}, {Phase: "check"},
		{Status: "ok"}, {Status: "recovering"}, {Status: "flag_not_found"}, {Status: "faulty"}, {Status: "down"}, {Status: "odd"},
		{Status: "ok", Message: "leaked AD_FLAG=PLAYIT{x}"},
	} {
		message := sanitizeParticipantSLAMessage(summary)
		if message == "" || strings.Contains(message, "AD_FLAG") {
			t.Errorf("summary %+v sanitized to %q", summary, message)
		}
	}
	if matchSubmissionState(nil) != "running" || matchSubmissionState(&GameMatchStatus{State: "paused"}) != "paused" || matchSubmissionState(&GameMatchStatus{State: "running"}) != "not_started" {
		t.Fatal("match submission state")
	}
	var nilServer *Server
	if len(nilServer.sensitiveCheckerMarkers(context.Background())) != 0 {
		t.Fatal("a nil server has no markers")
	}
	if markers := (&Server{store: failingStore{}}).sensitiveCheckerMarkers(context.Background()); len(markers) != 0 {
		t.Fatal("unreadable settings mean no extra markers")
	}
	store := NewMemoryStore(101)
	if _, err := store.UpdatePlatformSettings(context.Background(), adminUpdatePlatformSettingsRequest{FlagFormatPrefix: "CTF"}, "t", time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetActiveFlagFormat(context.Background(), "", time.Now()); err != nil {
		t.Fatal(err)
	}
	markers := (&Server{store: store}).sensitiveCheckerMarkers(context.Background())
	if len(markers) == 0 {
		t.Fatal("a custom flag prefix becomes a redaction marker")
	}
	blank := blankActiveFormatStore{Store: store}
	if markers := (&Server{store: blank}).sensitiveCheckerMarkers(context.Background()); len(markers) == 0 {
		t.Fatal("with no active format the prefix is the marker")
	}
	states := []serviceState{{ChallengeID: 1, SLAMessage: "flag CTF{abc} leaked"}}
	(&Server{store: store}).sanitizeParticipantServiceStates(context.Background(), states)
	if strings.Contains(states[0].SLAMessage, "CTF{") {
		t.Fatalf("custom-prefix flags must be redacted: %q", states[0].SLAMessage)
	}
	page := paginateAttackFeed([]attackEvent{{Tick: 1}, {Tick: 2}}, AttackFeedQuery{Limit: 0, Offset: -1, TickFrom: 2})
	if page.Limit != 12 || page.Offset != 0 || page.TotalCount != 1 {
		t.Fatalf("attack paging defaults: %+v", page)
	}
	if empty := paginateAttackFeed([]attackEvent{{Tick: 1}}, AttackFeedQuery{Limit: 5, Offset: 9}); len(empty.Items) != 0 {
		t.Fatal("an offset past the end is empty")
	}
	if len(filterAttackFeed([]attackEvent{{Tick: 5, Attacker: "a", Victim: "b", Service: "c"}}, AttackFeedQuery{TickTo: 3})) != 0 {
		t.Fatal("tick_to filter")
	}
	if parsePositiveIntWithDefault("abc", 7, 10) != 7 || parsePositiveIntWithDefault("99", 7, 10) != 10 || parseNonNegativeInt("-1") != 0 {
		t.Fatal("query int parsing")
	}
	if NewWithStore("a", "b", 101, store) == nil {
		t.Fatal("NewWithStore")
	}
}

type blankActiveFormatStore struct{ Store }

func (s blankActiveFormatStore) GetPlatformSettings(ctx context.Context) (adminPlatformSettings, error) {
	settings, err := s.Store.GetPlatformSettings(ctx)
	settings.FlagFormatActive = ""
	return settings, err
}

// scoreboardDownStore fails only the scoreboard read.
type scoreboardDownStore struct{ Store }

func (scoreboardDownStore) ListScoreboard(context.Context) ([]scoreRow, error) {
	return nil, errStoreDown
}

func TestServerRemainingBranches(t *testing.T) {
	store := NewMemoryStore(101)
	env, _ := residualMux(t, store, nil)
	if response := env.do(http.MethodPost, "/api/v2/authenticate", "", authenticateRequest{Email: "resident@residual.test", Password: "wrong-password"}); response.Code != http.StatusForbidden {
		t.Errorf("a wrong password: %d", response.Code)
	}
	if validParticipantName(strings.Repeat("n", 97)) {
		t.Error("names over 96 characters are rejected")
	}
	if response := env.do(http.MethodGet, "/internal/v1/rate-limit/metrics", "", nil); response.Code != http.StatusOK || !strings.HasPrefix(response.Header().Get("Content-Type"), "text/plain") {
		t.Errorf("metrics: %d %q", response.Code, response.Header().Get("Content-Type"))
	}
	if err := (&Server{}).reconcileWireGuardGateway(context.Background()); err != nil {
		t.Errorf("no WireGuard client means nothing to reconcile: %v", err)
	}

	// The per-email registration limit applies after the per-IP one.
	emailLimited, _ := residualMux(t, NewMemoryStore(101), func(s *Server) { s.WithRateLimiter(prefixLimiter{prefix: "register:email:"}) })
	if response := emailLimited.do(http.MethodPost, "/api/v2/register", "", participantRegisterRequest{DisplayName: "L", Email: "limited@residual.test", Password: "residual-password"}); response.Code != http.StatusTooManyRequests {
		t.Errorf("per-email registration limit: %d", response.Code)
	}

	scoreboardDown := scoreboardDownStore{Store: NewMemoryStore(101)}
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "api-gateway", Version: "test", Addr: ":0"})
	NewWithDeps("dev-team-token", "dev-admin-token", 101, scoreboardDown, storeBackedControllerClient{store: scoreboardDown}, noopWireGuardClient{}, noopGameCoreClient{}).RegisterRoutes(mux)
	if response := (&sweepEnv{t: t, mux: mux}).do(http.MethodGet, "/api/v2/scoreboard", "", nil); response.Code != http.StatusInternalServerError {
		t.Errorf("a store scoreboard failure: %d", response.Code)
	}

	// Organizers have no team, so team routes refuse them.
	if response := env.do(http.MethodPost, "/api/v2/admin/players", "dev-admin-token", adminCreatePlayerRequest{DisplayName: "Org", Email: "org2@residual.test", Password: "residual-password", Role: "organizer"}); response.Code >= 300 {
		t.Fatalf("organizer setup: %d %s", response.Code, response.Body.String())
	}
	orgAuth := decodeCompat[map[string]any](t, env.do(http.MethodPost, "/api/v2/authenticate", "", authenticateRequest{Email: "org2@residual.test", Password: "residual-password"}).Body.Bytes())
	orgToken, _ := orgAuth["token"].(string)
	if response := env.do(http.MethodPost, "/api/v2/services/1/ssh-session", orgToken, nil); response.Code != http.StatusForbidden {
		t.Errorf("an organizer on a team route: %d", response.Code)
	}

	// A sensitive warmup message is dropped unless the warmup failed.
	status := sanitizePublicGameStatus(GameStatus{Match: &GameMatchStatus{State: "running", Warmup: &GameWarmupResult{Status: "running", Message: "leaked AD_FLAG=PLAYIT{x}"}}})
	if status.Match.Warmup.Message != "" {
		t.Errorf("a sensitive in-progress warmup message must be cleared: %q", status.Match.Warmup.Message)
	}

	events := []attackEvent{{Tick: 1, Attacker: "a", Victim: "b", Service: "c"}}
	for _, query := range []AttackFeedQuery{{Victim: "zz"}, {Service: "zz"}} {
		if len(filterAttackFeed(events, query)) != 0 {
			t.Errorf("query %+v must filter the event out", query)
		}
	}
}
