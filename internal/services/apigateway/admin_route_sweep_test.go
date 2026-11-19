package apigateway

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"

	"adplatform/internal/platform/httpapi"
)

type registeredRoute struct {
	method  string
	pattern string
}

var routeRegistration = regexp.MustCompile(`mux\.HandleFunc\("([A-Z]+) (/[^"]+)"`)

// registeredGatewayRoutes reads the route table straight from the sources, so a
// route added later is swept automatically and cannot silently skip auth.
func registeredGatewayRoutes(t *testing.T) []registeredRoute {
	t.Helper()
	var routes []registeredRoute
	for _, file := range []string{"server.go", "ops_features.go"} {
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		for _, match := range routeRegistration.FindAllStringSubmatch(string(source), -1) {
			routes = append(routes, registeredRoute{method: match[1], pattern: match[2]})
		}
	}
	if len(routes) < 80 {
		t.Fatalf("route table parse looks wrong: found only %d routes", len(routes))
	}
	return routes
}

var pathParameter = regexp.MustCompile(`\{[a-z_]+\}`)

func concretePath(pattern, value string) string {
	return pathParameter.ReplaceAllString(pattern, value)
}

func sweepRequest(method, path, token string) *http.Request {
	var body *bytes.Buffer
	if method == http.MethodGet || method == http.MethodDelete {
		body = &bytes.Buffer{}
	} else {
		body = bytes.NewBufferString(`{}`)
	}
	request := httptest.NewRequest(method, path, body)
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	return request
}

func serve(mux *http.ServeMux, request *http.Request) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	return recorder
}

func failingBackendMux() *http.ServeMux {
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "api-gateway", Version: "dev", Addr: ":0"})
	server := NewWithDeps("dev-team-token", "dev-admin-token", 101, failingStore{}, failingControllerClient{}, failingWireGuardClient{}, failingGameCoreClient{})
	server.WithSubmissionClient(testSubmissionClient{err: errUpstreamDown})
	server.WithScoringClient(testScoringClient{err: errUpstreamDown})
	server.RegisterRoutes(mux)
	return mux
}

func TestEveryAdminRouteRequiresTheAdminToken(t *testing.T) {
	mux := newTestMux()
	participantToken := ""
	{
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		setTestTeamAuthHeader(t, request)
		participantToken = strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer ")
	}
	swept := 0
	for _, route := range registeredGatewayRoutes(t) {
		if !strings.HasPrefix(route.pattern, "/api/v2/admin/") {
			continue
		}
		swept++
		path := concretePath(route.pattern, "1")
		for name, token := range map[string]string{"anonymous": "", "wrong token": "not-the-admin-token", "participant token": participantToken} {
			response := serve(mux, sweepRequest(route.method, path, token))
			if response.Code != http.StatusForbidden {
				t.Errorf("%s %s with %s: want 403, got %d: %s", route.method, path, name, response.Code, response.Body.String())
			}
		}
	}
	if swept < 60 {
		t.Fatalf("expected to sweep the admin surface, swept only %d routes", swept)
	}
}

func TestAdminRoutesRejectMalformedPathIDs(t *testing.T) {
	mux := newTestMux()
	for _, route := range registeredGatewayRoutes(t) {
		if !strings.HasPrefix(route.pattern, "/api/v2/admin/") || !pathParameter.MatchString(route.pattern) {
			continue
		}
		for _, bad := range []string{"abc", "0", "-4"} {
			request := sweepRequest(route.method, concretePath(route.pattern, bad), "dev-admin-token")
			response := serve(mux, request)
			if response.Code != http.StatusBadRequest {
				t.Errorf("%s %s: want 400 for id %q, got %d: %s", route.method, request.URL.Path, bad, response.Code, response.Body.String())
			}
		}
	}
}

// With every backend failing, an admin route must report an error and never a
// success the organizer could act on.
func TestAdminRoutesSurfaceBackendFailures(t *testing.T) {
	mux := failingBackendMux()
	for _, route := range registeredGatewayRoutes(t) {
		if !strings.HasPrefix(route.pattern, "/api/v2/admin/") {
			continue
		}
		request := sweepRequest(route.method, concretePath(route.pattern, "1"), "dev-admin-token")
		response := serve(mux, request)
		if route.pattern == "/api/v2/admin/operations/status" {
			// The health report itself must stay readable during an outage and
			// say it is unhealthy instead of failing.
			status := decodeCompat[map[string]any](t, response.Body.Bytes())
			if response.Code != http.StatusOK || status["healthy"] != false {
				t.Errorf("operations status during outage: want 200 healthy=false, got %d %v", response.Code, status)
			}
			continue
		}
		if response.Code < 400 || response.Code > 599 {
			t.Errorf("%s %s with failing backends: want an error status, got %d: %s", route.method, request.URL.Path, response.Code, response.Body.String())
		}
	}
}

// sessionOnlyStore validates participant sessions against a real store while
// every other operation fails, so participant handlers reach their own
// failure branches instead of stopping at authentication.
type sessionOnlyStore struct {
	failingStore
	sessions Store
}

func (s sessionOnlyStore) ValidatePlayerSession(ctx context.Context, playerID, teamID int, role string, sessionVersion int) (authenticatedPlayer, error) {
	return s.sessions.ValidatePlayerSession(ctx, playerID, teamID, role, sessionVersion)
}

func TestParticipantRoutesSurfaceBackendFailures(t *testing.T) {
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "api-gateway", Version: "dev", Addr: ":0"})
	server := NewWithDeps("dev-team-token", "dev-admin-token", 101, sessionOnlyStore{sessions: NewMemoryStore(101)}, failingControllerClient{}, failingWireGuardClient{}, failingGameCoreClient{})
	server.WithSubmissionClient(testSubmissionClient{err: errUpstreamDown})
	server.WithScoringClient(testScoringClient{err: errUpstreamDown})
	server.RegisterRoutes(mux)

	bodies := map[string]string{
		"POST /api/v2/submit":                 `{"flags":["PLAYIT{x}"]}`,
		"POST /api/v2/me/team":                `{"team_key":"TEAM-KEY"}`,
		"POST /api/v2/me/team/captain":        `{"player_id":2}`,
		"PUT /api/v2/me/profile":              `{"display_name":"Alice","email":"alice@example.test"}`,
		"PUT /api/v2/me/password":             `{"current_password":"old-password","new_password":"new-password"}`,
		"POST /api/v2/authenticate":           `{"email":"a@example.test","password":"password1"}`,
		"POST /api/v2/register":               `{"display_name":"Alice","email":"a@example.test","password":"password1"}`,
		"GET /api/v2/session":                 ``,
		"GET /api/v2/platform/appearance":     ``,
		"GET /api/v2/scoreboard/freeze":       ``,
		"GET /api/v2/me/team/members":         ``,
		"GET /api/v2/me/wireguard":            ``,
		"GET /api/v2/challenges/1/source":     ``,
		"GET /api/v2/challenges":              ``,
		"GET /api/v2/services":                ``,
		"GET /api/v2/scoreboard":              ``,
		"GET /api/v2/game/status":             ``,
		"GET /api/v2/attacks":                 ``,
		"GET /api/v2/team/services":           ``,
		"GET /api/v2/announcements":           ``,
		"POST /api/v2/services/1/ssh-session": ``,
	}
	// Session and appearance reads are served from the validated session or
	// have documented defaults; every other route must fail loudly.
	mayStillSucceed := map[string]bool{
		"GET /api/v2/session":             true,
		"GET /api/v2/platform/appearance": true,
		"GET /api/v2/team/services":       false,
	}
	for key, body := range bodies {
		method, path, _ := strings.Cut(key, " ")
		request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		setTestTeamAuthHeader(t, request)
		response := serve(mux, request)
		if mayStillSucceed[key] {
			continue
		}
		if response.Code < 400 || response.Code > 599 {
			t.Errorf("%s with failing backends: want an error status, got %d: %s", key, response.Code, response.Body.String())
		}
	}
}
