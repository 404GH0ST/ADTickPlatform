package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"adplatform/internal/platform/httpapi"
	"adplatform/internal/services/apigateway"
)

type testScoringGameCoreClient struct {
	rows []apigateway.ScoreRowAlias
	err  error
}

func (c testScoringGameCoreClient) Scoreboard(_ context.Context) ([]apigateway.ScoreRowAlias, error) {
	if c.err != nil {
		return nil, c.err
	}
	if len(c.rows) == 0 {
		return []apigateway.ScoreRowAlias{{Rank: 1, Team: "Team Alpha", Attack: 10, Defense: 9, SLA: 8, Total: 27, Delta: "new"}}, nil
	}
	return c.rows, nil
}

func (c testScoringGameCoreClient) RecomputeScoring(_ context.Context) ([]apigateway.ScoreRowAlias, error) {
	return c.Scoreboard(context.Background())
}

func TestScoringWorkerRecompute(t *testing.T) {
	server := newScoringWorkerServer("dev-admin-token", testScoringGameCoreClient{})
	server.now = func() time.Time { return time.Date(2026, 3, 11, 2, 0, 0, 0, time.UTC) }
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "scoring-worker", Version: "dev", Addr: ":0"})
	server.RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodPost, "/internal/v1/scoring/recompute", nil)
	request.Header.Set("Authorization", "Bearer dev-admin-token")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}

	statusRequest := httptest.NewRequest(http.MethodGet, "/internal/v1/scoring/status", nil)
	statusRequest.Header.Set("Authorization", "Bearer dev-admin-token")
	statusResponse := httptest.NewRecorder()
	mux.ServeHTTP(statusResponse, statusRequest)

	if statusResponse.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", statusResponse.Code)
	}

	var payload struct {
		Status string              `json:"status"`
		Data   scoringWorkerStatus `json:"data"`
	}
	if err := json.Unmarshal(statusResponse.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode scoring status: %v", err)
	}
	if payload.Data.LastScoreRows != 1 || payload.Data.LastRecomputedAt == "" || payload.Data.State != "ready" {
		t.Fatalf("unexpected scoring status %+v", payload.Data)
	}
}

func TestScoringWorkerFailureMarksStatusDegraded(t *testing.T) {
	server := newScoringWorkerServer("dev-admin-token", testScoringGameCoreClient{err: errors.New("boom")})
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "scoring-worker", Version: "dev", Addr: ":0"})
	server.RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodPost, "/internal/v1/scoring/recompute", nil)
	request.Header.Set("Authorization", "Bearer dev-admin-token")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", response.Code)
	}

	status := server.snapshotStatus()
	if status.State != "degraded" || status.LastError == "" {
		t.Fatalf("unexpected worker status %+v", status)
	}
}
