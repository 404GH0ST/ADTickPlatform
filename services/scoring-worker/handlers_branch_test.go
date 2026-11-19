package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"adplatform/internal/services/apigateway"
)

type scriptedScoringCore struct {
	err   error
	audit apigateway.ScoringAuditAlias
}

func (c scriptedScoringCore) Scoreboard(context.Context) ([]apigateway.ScoreRowAlias, error) {
	return []apigateway.ScoreRowAlias{{Team: "A"}}, c.err
}

func (c scriptedScoringCore) RecomputeScoring(context.Context) ([]apigateway.ScoreRowAlias, error) {
	return []apigateway.ScoreRowAlias{{Team: "A"}}, c.err
}

func (c scriptedScoringCore) AuditScoring(context.Context) (apigateway.ScoringAuditAlias, error) {
	return c.audit, c.err
}

func scoringCall(server *scoringWorkerServer, method, path string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)
	request := httptest.NewRequest(method, path, nil)
	request.Header.Set("Authorization", "Bearer worker-token")
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	return recorder
}

func TestScoringWorkerHandlerBranches(t *testing.T) {
	if _, ok := newScoringWorkerServer("t", nil).gameCore.(noopGameCoreScoringClient); !ok {
		t.Fatal("a nil game-core client must default to the noop client")
	}
	healthy := newScoringWorkerServer("worker-token", scriptedScoringCore{audit: apigateway.ScoringAuditAlias{Status: "mismatch", MismatchCount: 2}})
	if response := scoringCall(healthy, http.MethodGet, "/internal/v1/scoring/scoreboard"); response.Code != http.StatusOK {
		t.Fatalf("scoreboard: %d", response.Code)
	}
	if response := scoringCall(healthy, http.MethodGet, "/internal/v1/scoring/audit"); response.Code != http.StatusOK {
		t.Fatalf("audit: %d", response.Code)
	}
	if status := healthy.snapshotStatus(); status.LastMismatches != 2 || status.LastAuditStatus != "mismatch" {
		t.Fatalf("audit mismatches must be recorded, got %+v", status)
	}

	failing := newScoringWorkerServer("worker-token", scriptedScoringCore{err: errors.New("core down")})
	for _, path := range []string{"/internal/v1/scoring/scoreboard", "/internal/v1/scoring/audit"} {
		if response := scoringCall(failing, http.MethodGet, path); response.Code != http.StatusBadGateway {
			t.Fatalf("%s with a failing core: %d", path, response.Code)
		}
	}
	if status := failing.snapshotStatus(); status.State != "degraded" || status.LastError != "core down" {
		t.Fatalf("failures must degrade status, got %+v", status)
	}

	recorder := httptest.NewRecorder()
	writeScoringWorkerFailure(recorder, errGameCoreScoringDisabled, "")
	if !strings.Contains(recorder.Body.String(), "not connected to game-core") {
		t.Fatalf("disabled core message: %s", recorder.Body.String())
	}
	recorder = httptest.NewRecorder()
	writeScoringWorkerFailure(recorder, errors.New("x"), "  ")
	if !strings.Contains(recorder.Body.String(), "scoring-worker request failed.") {
		t.Fatalf("blank fallback must use the generic message: %s", recorder.Body.String())
	}
}

func TestScoringClientErrorBodyReadFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("short"))
	}))
	defer server.Close()
	_, err := newGameCoreScoringClient(server.URL, "").Scoreboard(context.Background())
	if err == nil || !strings.Contains(err.Error(), "status 502") {
		t.Fatalf("a truncated error body must fall back to the status, got %v", err)
	}
}
