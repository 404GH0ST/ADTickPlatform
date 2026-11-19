package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"

	"adplatform/internal/platform/httpapi"
)

var gameRoutePattern = regexp.MustCompile(`mux\.HandleFunc\("([A-Z]+) (/[^"]+)"`)

func gameCoreRoutes(t *testing.T) [][2]string {
	t.Helper()
	source, err := os.ReadFile("server.go")
	if err != nil {
		t.Fatal(err)
	}
	var routes [][2]string
	for _, match := range gameRoutePattern.FindAllStringSubmatch(string(source), -1) {
		routes = append(routes, [2]string{match[1], match[2]})
	}
	if len(routes) < 25 {
		t.Fatalf("route parse looks wrong: %v", routes)
	}
	return routes
}

func gameMux(store gameStore) *http.ServeMux {
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "game-core", Version: "test", Addr: ":0"})
	newTickServer(store).RegisterRoutes(mux)
	return mux
}

func gameCall(mux *http.ServeMux, method, path, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Authorization", "Bearer dev-admin-token")
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	return recorder
}

// With every store call failing, store-backed routes must report an error, and
// every response must still be well-formed JSON.
func TestGameCoreRoutesSurfaceStoreFailures(t *testing.T) {
	mux := gameMux(failingGameStore{})
	bodies := map[string]string{
		"/internal/v1/game/match/schedule":      `{"scheduled_start_at":"2026-11-19T10:00:00Z"}`,
		"/internal/v1/game/scheduler/interval":  `{"interval_seconds":30}`,
		"/internal/v1/flags/submit":             `{"team_id":101,"flags":["PLAYIT{x}"]}`,
		"/internal/v1/game/flag/format/refresh": `{"prefix":"CTF"}`,
	}
	// These are served from in-process state, not the store.
	inProcess := map[string]bool{
		"GET /internal/v1/game/scheduler":            true,
		"GET /internal/v1/game/scheduler/events":     true,
		"POST /internal/v1/game/scheduler/start":     true,
		"POST /internal/v1/game/scheduler/stop":      true,
		"PUT /internal/v1/game/scheduler/interval":   true,
		"GET /internal/v1/game/flag/format":          true,
		"POST /internal/v1/game/flag/format/refresh": true,
		"GET /internal/v1/game/warmup":               true,
	}
	for _, route := range gameCoreRoutes(t) {
		path := strings.ReplaceAll(route[1], "{submission_id}", "1")
		response := gameCall(mux, route[0], path, bodies[route[1]])
		if !json.Valid(response.Body.Bytes()) {
			t.Errorf("%s %s: response is not JSON: %q", route[0], path, response.Body.String())
		}
		if inProcess[route[0]+" "+route[1]] {
			continue
		}
		if response.Code < 400 {
			t.Errorf("%s %s with a failing store: want an error, got %d %s", route[0], path, response.Code, response.Body.String())
		}
	}
}

func TestGameCoreRoutesRejectMalformedBodies(t *testing.T) {
	mux := gameMux(newMemoryGameStore())
	for _, route := range gameCoreRoutes(t) {
		if route[0] == http.MethodGet || strings.Contains(route[1], "{") {
			continue
		}
		response := gameCall(mux, route[0], route[1], `{"not":"expected", trailing`)
		if response.Code >= 500 {
			t.Errorf("%s %s: malformed input must not be a server error, got %d", route[0], route[1], response.Code)
		}
	}
}
