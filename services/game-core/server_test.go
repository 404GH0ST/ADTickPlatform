package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"adplatform/internal/platform/httpapi"
	"adplatform/internal/services/apigateway"
)

type testCheckerClient struct {
	failPhase string
}

type capturingCheckerClient struct {
	requests []apigateway.CheckerExecutionRequest
}

type testGameScheduler struct {
	status apigateway.GameSchedulerStatus
	events []apigateway.GameSchedulerEvent
}

func (s *testGameScheduler) Status() apigateway.GameSchedulerStatus {
	if s.status.State == "" {
		s.status = apigateway.GameSchedulerStatus{State: "stopped", IntervalSeconds: 60}
	}
	return s.status
}

func (s *testGameScheduler) Start() (apigateway.GameSchedulerStatus, error) {
	status := s.Status()
	status.State = "running"
	status.NextRunAt = "2026-03-10T10:02:00Z"
	s.status = status
	return status, nil
}

func (s *testGameScheduler) Stop() (apigateway.GameSchedulerStatus, error) {
	status := s.Status()
	status.State = "stopped"
	status.NextRunAt = ""
	s.status = status
	return status, nil
}

func (s *testGameScheduler) Update(interval time.Duration) (apigateway.GameSchedulerStatus, error) {
	status := s.Status()
	status.IntervalSeconds = int(interval / time.Second)
	s.status = status
	return status, nil
}

func (s *testGameScheduler) Events(_ context.Context, query apigateway.GameSchedulerEventQuery) (apigateway.GameSchedulerEventPage, error) {
	if len(s.events) == 0 {
		s.events = []apigateway.GameSchedulerEvent{
			{ID: 2, EventType: "tick_completed", Source: "scheduler", State: "running", TickID: 1, Message: "scheduled tick completed successfully", CreatedAt: "2026-03-10T10:01:12Z"},
			{ID: 1, EventType: "started", Source: "organizer", State: "running", Message: "scheduler started by organizer", CreatedAt: "2026-03-10T10:01:00Z"},
		}
	}
	filtered := make([]apigateway.GameSchedulerEvent, 0, len(s.events))
	for _, event := range s.events {
		if query.EventType != "" && event.EventType != query.EventType {
			continue
		}
		if query.Source != "" && event.Source != query.Source {
			continue
		}
		if query.State != "" && event.State != query.State {
			continue
		}
		filtered = append(filtered, event)
	}
	offset := query.Offset
	if offset < 0 {
		offset = 0
	}
	if offset >= len(filtered) {
		limit := query.Limit
		if limit <= 0 {
			limit = 25
		}
		return apigateway.GameSchedulerEventPage{
			Items:      []apigateway.GameSchedulerEvent{},
			Limit:      limit,
			Offset:     offset,
			TotalCount: len(filtered),
			HasPrev:    offset > 0,
		}, nil
	}
	limit := query.Limit
	if limit <= 0 || limit > len(filtered)-offset {
		limit = len(filtered) - offset
	}
	return apigateway.GameSchedulerEventPage{
		Items:      filtered[offset : offset+limit],
		Limit:      limit,
		Offset:     offset,
		TotalCount: len(filtered),
		HasPrev:    offset > 0,
		HasNext:    offset+limit < len(filtered),
	}, nil
}

func (s *testGameScheduler) Close() error {
	return nil
}

func (c testCheckerClient) Execute(_ context.Context, request apigateway.CheckerExecutionRequest) (apigateway.CheckerExecutionResult, error) {
	result := apigateway.CheckerExecutionResult{
		ChallengeID: request.ChallengeID,
		TeamID:      request.TeamID,
		Phase:       request.Phase,
		CheckedAt:   "2026-03-10T10:01:00Z",
		Output:      request.Phase + "-output",
	}
	if request.Phase == c.failPhase {
		result.Status = "failed"
		result.ExitCode = 1
		result.Message = "checker phase failed in test"
		return result, nil
	}
	result.Status = "success"
	result.ExitCode = 0
	result.Message = "checker phase passed in test"
	return result, nil
}

func (c *capturingCheckerClient) Execute(_ context.Context, request apigateway.CheckerExecutionRequest) (apigateway.CheckerExecutionResult, error) {
	c.requests = append(c.requests, request)
	return apigateway.CheckerExecutionResult{
		ChallengeID: request.ChallengeID,
		TeamID:      request.TeamID,
		Phase:       request.Phase,
		Status:      "success",
		ExitCode:    0,
		CheckedAt:   "2026-03-10T10:01:00Z",
		Output:      request.Phase + "-output",
		Message:     "captured",
	}, nil
}

func newTestGameCoreMux(checker checkerClient) *http.ServeMux {
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "game-core", Version: "dev", Addr: ":0"})
	newGameCoreServer("dev-admin-token", newMemoryGameStore(), checker, newFlagCodec("test-flag-secret"), &testGameScheduler{}, []string{"put", "get", "check"}, 15).RegisterRoutes(mux)
	return mux
}

func newTestGameCoreMuxWithScheduler(checker checkerClient, scheduler gameScheduler) *http.ServeMux {
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "game-core", Version: "dev", Addr: ":0"})
	newGameCoreServer("dev-admin-token", newMemoryGameStore(), checker, newFlagCodec("test-flag-secret"), scheduler, []string{"put", "get", "check"}, 15).RegisterRoutes(mux)
	return mux
}

func decodeResponse[T any](t *testing.T, body []byte) T {
	t.Helper()

	var payload T
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	return payload
}

func decodeProblem(t *testing.T, body []byte) httpapi.ProblemDetails {
	t.Helper()

	var payload httpapi.ProblemDetails
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("failed to decode problem response: %v", err)
	}
	return payload
}

func startTestMatch(t *testing.T, mux *http.ServeMux) {
	t.Helper()

	request := httptest.NewRequest(http.MethodPost, "/internal/v1/game/match/start", nil)
	request.Header.Set("Authorization", "Bearer dev-admin-token")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected match start 200, got %d", response.Code)
	}
}

func TestAdvanceTickPassesStructuredCheckerTargetMetadata(t *testing.T) {
	checker := &capturingCheckerClient{}
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "game-core", Version: "dev", Addr: ":0"})
	newGameCoreServer("dev-admin-token", newMemoryGameStore(), checker, newFlagCodec("test-flag-secret"), &testGameScheduler{}, []string{"put"}, 15).RegisterRoutes(mux)

	startTestMatch(t, mux)

	request := httptest.NewRequest(http.MethodPost, "/internal/v1/game/ticks/advance", nil)
	request.Header.Set("Authorization", "Bearer dev-admin-token")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}
	if len(checker.requests) == 0 {
		t.Fatal("expected checker execution requests to be captured")
	}

	first := checker.requests[0]
	if first.Target != "10.80.1.11:10001" {
		t.Fatalf("unexpected target %q", first.Target)
	}
	if first.TargetHost != "10.80.1.11" || first.TargetIP != "10.80.1.11" || first.TargetPort != 10001 {
		t.Fatalf("unexpected structured target metadata %+v", first)
	}
	if first.TeamName == "" || first.ChallengeName == "" {
		t.Fatalf("expected team and challenge names in checker request %+v", first)
	}
}

func TestAdvanceTickPersistsRunsAndStatus(t *testing.T) {
	mux := newTestGameCoreMux(testCheckerClient{failPhase: "get"})
	startTestMatch(t, mux)
	request := httptest.NewRequest(http.MethodPost, "/internal/v1/game/ticks/advance", nil)
	request.Header.Set("Authorization", "Bearer dev-admin-token")
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected advance 200, got %d", response.Code)
	}

	payload := decodeResponse[apigateway.GameTickStatus](t, response.Body.Bytes())
	if payload.TotalCheckerRuns != 36 {
		t.Fatalf("expected 36 checker runs, got %d", payload.TotalCheckerRuns)
	}
	if payload.SuccessfulCheckerRuns != 12 || payload.FailedCheckerRuns != 12 || payload.SkippedCheckerRuns != 12 {
		t.Fatalf("unexpected tick counters %+v", payload)
	}

	statusRequest := httptest.NewRequest(http.MethodGet, "/internal/v1/game/status", nil)
	statusRequest.Header.Set("Authorization", "Bearer dev-admin-token")
	statusResponse := httptest.NewRecorder()
	mux.ServeHTTP(statusResponse, statusRequest)
	if statusResponse.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", statusResponse.Code)
	}

	statusPayload := decodeResponse[apigateway.GameStatus](t, statusResponse.Body.Bytes())
	if statusPayload.CurrentTick == nil || statusPayload.CurrentTick.ID != 1 {
		t.Fatalf("unexpected game status %+v", statusPayload)
	}
	if statusPayload.TotalCheckerRuns != 36 {
		t.Fatalf("expected aggregate total 36, got %d", statusPayload.TotalCheckerRuns)
	}
}

func TestCheckerRunsEndpointHonorsLimit(t *testing.T) {
	mux := newTestGameCoreMux(testCheckerClient{})
	startTestMatch(t, mux)

	advanceRequest := httptest.NewRequest(http.MethodPost, "/internal/v1/game/ticks/advance", nil)
	advanceRequest.Header.Set("Authorization", "Bearer dev-admin-token")
	advanceResponse := httptest.NewRecorder()
	mux.ServeHTTP(advanceResponse, advanceRequest)
	if advanceResponse.Code != http.StatusOK {
		t.Fatalf("expected advance 200, got %d", advanceResponse.Code)
	}

	runsRequest := httptest.NewRequest(http.MethodGet, "/internal/v1/game/checker-runs?limit=5", nil)
	runsRequest.Header.Set("Authorization", "Bearer dev-admin-token")
	runsResponse := httptest.NewRecorder()
	mux.ServeHTTP(runsResponse, runsRequest)
	if runsResponse.Code != http.StatusOK {
		t.Fatalf("expected runs 200, got %d", runsResponse.Code)
	}

	payload := decodeResponse[apigateway.GameCheckerRunPage](t, runsResponse.Body.Bytes())
	if len(payload.Items) != 5 {
		t.Fatalf("expected 5 checker runs, got %d", len(payload.Items))
	}
	if payload.TotalCount != 36 || !payload.HasNext {
		t.Fatalf("unexpected checker run page metadata %+v", payload)
	}
	if payload.Items[0].TickID != 1 {
		t.Fatalf("expected latest runs from tick 1, got tick %d", payload.Items[0].TickID)
	}
}

func TestMetricsEndpointIncludesGameCoreMetrics(t *testing.T) {
	info := httpapi.ServiceInfo{Name: "game-core-metrics-" + strings.ToLower(strings.ReplaceAll(t.Name(), "/", "-")), Version: "test", Addr: ":0"}
	store := newMemoryGameStore()
	scheduler := &testGameScheduler{
		status: apigateway.GameSchedulerStatus{
			State:           "running",
			IntervalSeconds: 45,
			LastRunAt:       "2026-03-10T10:01:00Z",
			NextRunAt:       "2099-03-10T10:02:00Z",
			LastTickID:      1,
		},
	}
	server := newGameCoreServer("dev-admin-token", store, testCheckerClient{}, newFlagCodec("test-flag-secret"), scheduler, []string{"put", "get", "check"}, 15)
	httpapi.RegisterMetricsSource(info.Name, server)

	mux := httpapi.NewBaseMux(info)
	server.RegisterRoutes(mux)
	startTestMatch(t, mux)

	advanceRequest := httptest.NewRequest(http.MethodPost, "/internal/v1/game/ticks/advance", nil)
	advanceRequest.Header.Set("Authorization", "Bearer dev-admin-token")
	advanceResponse := httptest.NewRecorder()
	mux.ServeHTTP(advanceResponse, advanceRequest)
	if advanceResponse.Code != http.StatusOK {
		t.Fatalf("expected advance 200, got %d", advanceResponse.Code)
	}

	metricsRequest := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	metricsResponse := httptest.NewRecorder()
	mux.ServeHTTP(metricsResponse, metricsRequest)

	body := metricsResponse.Body.String()
	for _, fragment := range []string{
		"adplatform_game_core_metrics_collection_success 1",
		"adplatform_game_core_total_ticks 1",
		`adplatform_game_core_checker_runs_total{status="all"} 36`,
		"adplatform_game_core_scheduler_running 1",
		"adplatform_game_core_match_accepting_submissions 1",
		"adplatform_game_core_current_tick_id 1",
	} {
		if !strings.Contains(body, fragment) {
			t.Fatalf("expected metrics output to contain %q, got:\n%s", fragment, body)
		}
	}
}

func TestCheckerRunsEndpointSupportsFiltersAndOffset(t *testing.T) {
	mux := newTestGameCoreMux(testCheckerClient{failPhase: "get"})
	startTestMatch(t, mux)

	advanceRequest := httptest.NewRequest(http.MethodPost, "/internal/v1/game/ticks/advance", nil)
	advanceRequest.Header.Set("Authorization", "Bearer dev-admin-token")
	advanceResponse := httptest.NewRecorder()
	mux.ServeHTTP(advanceResponse, advanceRequest)
	if advanceResponse.Code != http.StatusOK {
		t.Fatalf("expected advance 200, got %d", advanceResponse.Code)
	}

	filteredRequest := httptest.NewRequest(http.MethodGet, "/internal/v1/game/checker-runs?team_id=101&challenge_id=1&phase=get&status=failed&limit=1", nil)
	filteredRequest.Header.Set("Authorization", "Bearer dev-admin-token")
	filteredResponse := httptest.NewRecorder()
	mux.ServeHTTP(filteredResponse, filteredRequest)
	if filteredResponse.Code != http.StatusOK {
		t.Fatalf("expected filtered checker runs 200, got %d", filteredResponse.Code)
	}

	filteredPayload := decodeResponse[apigateway.GameCheckerRunPage](t, filteredResponse.Body.Bytes())
	if len(filteredPayload.Items) != 1 || filteredPayload.Items[0].Phase != "get" || filteredPayload.Items[0].Status != "failed" {
		t.Fatalf("unexpected filtered checker runs payload %+v", filteredPayload)
	}
	if filteredPayload.TotalCount != 1 || filteredPayload.HasNext || filteredPayload.HasPrev {
		t.Fatalf("unexpected filtered checker run page metadata %+v", filteredPayload)
	}

	offsetRequest := httptest.NewRequest(http.MethodGet, "/internal/v1/game/checker-runs?team_id=101&challenge_id=1&offset=1&limit=1", nil)
	offsetRequest.Header.Set("Authorization", "Bearer dev-admin-token")
	offsetResponse := httptest.NewRecorder()
	mux.ServeHTTP(offsetResponse, offsetRequest)
	if offsetResponse.Code != http.StatusOK {
		t.Fatalf("expected checker runs offset 200, got %d", offsetResponse.Code)
	}

	filteredPayload = decodeResponse[apigateway.GameCheckerRunPage](t, offsetResponse.Body.Bytes())
	if len(filteredPayload.Items) != 1 || filteredPayload.Items[0].Phase != "get" {
		t.Fatalf("unexpected checker runs offset payload %+v", filteredPayload)
	}
	if !filteredPayload.HasPrev {
		t.Fatalf("expected checker run offset page to expose previous page %+v", filteredPayload)
	}
}

func TestSchedulerEndpointsExposeStatusAndControl(t *testing.T) {
	store := newMemoryGameStore()
	scheduler := newIntervalGameScheduler(store, time.Minute, false, func(context.Context) (apigateway.GameTickStatus, error) {
		return apigateway.GameTickStatus{ID: 1, Status: "completed"}, nil
	}, store.MatchStatus)
	defer scheduler.Close()

	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "game-core", Version: "dev", Addr: ":0"})
	newGameCoreServer("dev-admin-token", store, testCheckerClient{}, newFlagCodec("test-flag-secret"), scheduler, []string{"put", "get", "check"}, 15).RegisterRoutes(mux)

	statusRequest := httptest.NewRequest(http.MethodGet, "/internal/v1/game/scheduler", nil)
	statusRequest.Header.Set("Authorization", "Bearer dev-admin-token")
	statusResponse := httptest.NewRecorder()
	mux.ServeHTTP(statusResponse, statusRequest)
	if statusResponse.Code != http.StatusOK {
		t.Fatalf("expected scheduler status 200, got %d", statusResponse.Code)
	}

	statusPayload := decodeResponse[apigateway.GameSchedulerStatus](t, statusResponse.Body.Bytes())
	if statusPayload.State != "stopped" || statusPayload.IntervalSeconds != 60 {
		t.Fatalf("unexpected scheduler status %+v", statusPayload)
	}

	startRequest := httptest.NewRequest(http.MethodPost, "/internal/v1/game/scheduler/start", nil)
	startRequest.Header.Set("Authorization", "Bearer dev-admin-token")
	startResponse := httptest.NewRecorder()
	mux.ServeHTTP(startResponse, startRequest)
	if startResponse.Code != http.StatusBadRequest {
		t.Fatalf("expected scheduler start 400 before match start, got %d", startResponse.Code)
	}

	startTestMatch(t, mux)

	startRequest = httptest.NewRequest(http.MethodPost, "/internal/v1/game/scheduler/start", nil)
	startRequest.Header.Set("Authorization", "Bearer dev-admin-token")
	startResponse = httptest.NewRecorder()
	mux.ServeHTTP(startResponse, startRequest)
	if startResponse.Code != http.StatusOK {
		t.Fatalf("expected scheduler start 200, got %d", startResponse.Code)
	}

	stopRequest := httptest.NewRequest(http.MethodPost, "/internal/v1/game/scheduler/stop", nil)
	stopRequest.Header.Set("Authorization", "Bearer dev-admin-token")
	stopResponse := httptest.NewRecorder()
	mux.ServeHTTP(stopResponse, stopRequest)
	if stopResponse.Code != http.StatusOK {
		t.Fatalf("expected scheduler stop 200, got %d", stopResponse.Code)
	}
}

func TestMatchEndpointsExposeStatusAndControl(t *testing.T) {
	mux := newTestGameCoreMux(testCheckerClient{})

	statusRequest := httptest.NewRequest(http.MethodGet, "/internal/v1/game/match", nil)
	statusRequest.Header.Set("Authorization", "Bearer dev-admin-token")
	statusResponse := httptest.NewRecorder()
	mux.ServeHTTP(statusResponse, statusRequest)
	if statusResponse.Code != http.StatusOK {
		t.Fatalf("expected match status 200, got %d", statusResponse.Code)
	}

	statusPayload := decodeResponse[apigateway.GameMatchStatus](t, statusResponse.Body.Bytes())
	if statusPayload.State != "not_started" || statusPayload.AcceptingSubmissions {
		t.Fatalf("unexpected match status %+v", statusPayload)
	}

	updateRequest := httptest.NewRequest(
		http.MethodPut,
		"/internal/v1/game/match/schedule",
		strings.NewReader(`{"scheduled_start_at":"2099-03-10T09:00:00Z","scheduled_end_at":"2099-03-10T13:00:00Z"}`),
	)
	updateRequest.Header.Set("Authorization", "Bearer dev-admin-token")
	updateRequest.Header.Set("Content-Type", "application/json")
	updateResponse := httptest.NewRecorder()
	mux.ServeHTTP(updateResponse, updateRequest)
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("expected match schedule update 200, got %d", updateResponse.Code)
	}

	updatePayload := decodeResponse[apigateway.GameMatchStatus](t, updateResponse.Body.Bytes())
	if updatePayload.ScheduledStartAt != "2099-03-10T09:00:00Z" || updatePayload.ScheduledEndAt != "2099-03-10T13:00:00Z" {
		t.Fatalf("unexpected match schedule %+v", updatePayload)
	}

	startRequest := httptest.NewRequest(http.MethodPost, "/internal/v1/game/match/start", nil)
	startRequest.Header.Set("Authorization", "Bearer dev-admin-token")
	startResponse := httptest.NewRecorder()
	mux.ServeHTTP(startResponse, startRequest)
	if startResponse.Code != http.StatusOK {
		t.Fatalf("expected match start 200, got %d", startResponse.Code)
	}

	stopRequest := httptest.NewRequest(http.MethodPost, "/internal/v1/game/match/stop", nil)
	stopRequest.Header.Set("Authorization", "Bearer dev-admin-token")
	stopResponse := httptest.NewRecorder()
	mux.ServeHTTP(stopResponse, stopRequest)
	if stopResponse.Code != http.StatusOK {
		t.Fatalf("expected match stop 200, got %d", stopResponse.Code)
	}
}

func TestMatchStatusUsesConfiguredWindow(t *testing.T) {
	start := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)
	end := start.Add(4 * time.Hour)
	server := newGameCoreServer("dev-admin-token", newMemoryGameStore(), testCheckerClient{}, newFlagCodec("test-flag-secret"), &testGameScheduler{}, []string{"put", "get", "check"}, 15)
	server.matchStartAt = &start
	server.matchEndAt = &end
	server.now = func() time.Time {
		return start.Add(30 * time.Minute)
	}

	status, err := server.matchStatus(context.Background())
	if err != nil {
		t.Fatalf("failed to resolve scheduled match status: %v", err)
	}
	if status.State != "running" || !status.AcceptingSubmissions {
		t.Fatalf("expected scheduled running match, got %+v", status)
	}
	if status.StartedAt != start.Format(time.RFC3339) || status.ScheduledEndAt != end.Format(time.RFC3339) {
		t.Fatalf("unexpected scheduled window metadata %+v", status)
	}

	server.now = func() time.Time {
		return end.Add(time.Minute)
	}
	status, err = server.matchStatus(context.Background())
	if err != nil {
		t.Fatalf("failed to resolve finished scheduled match status: %v", err)
	}
	if status.State != "finished" || status.AcceptingSubmissions {
		t.Fatalf("expected scheduled finished match, got %+v", status)
	}
	if status.EndedAt != end.Format(time.RFC3339) {
		t.Fatalf("expected scheduled end time %s, got %+v", end.Format(time.RFC3339), status)
	}
}

func TestAdvanceTickRequiresRunningMatch(t *testing.T) {
	mux := newTestGameCoreMux(testCheckerClient{})

	request := httptest.NewRequest(http.MethodPost, "/internal/v1/game/ticks/advance", nil)
	request.Header.Set("Authorization", "Bearer dev-admin-token")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected pre-start advance 400, got %d", response.Code)
	}

	startTestMatch(t, mux)

	request = httptest.NewRequest(http.MethodPost, "/internal/v1/game/ticks/advance", nil)
	request.Header.Set("Authorization", "Bearer dev-admin-token")
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected running advance 200, got %d", response.Code)
	}

	stopRequest := httptest.NewRequest(http.MethodPost, "/internal/v1/game/match/stop", nil)
	stopRequest.Header.Set("Authorization", "Bearer dev-admin-token")
	stopResponse := httptest.NewRecorder()
	mux.ServeHTTP(stopResponse, stopRequest)
	if stopResponse.Code != http.StatusOK {
		t.Fatalf("expected stop match 200, got %d", stopResponse.Code)
	}

	request = httptest.NewRequest(http.MethodPost, "/internal/v1/game/ticks/advance", nil)
	request.Header.Set("Authorization", "Bearer dev-admin-token")
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected finished advance 400, got %d", response.Code)
	}
}

func TestSchedulerEventsEndpointHonorsLimit(t *testing.T) {
	mux := newTestGameCoreMux(testCheckerClient{})

	request := httptest.NewRequest(http.MethodGet, "/internal/v1/game/scheduler/events?limit=1", nil)
	request.Header.Set("Authorization", "Bearer dev-admin-token")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected scheduler events 200, got %d", response.Code)
	}

	payload := decodeResponse[apigateway.GameSchedulerEventPage](t, response.Body.Bytes())
	if len(payload.Items) != 1 || payload.Items[0].EventType != "tick_completed" {
		t.Fatalf("unexpected scheduler events payload %+v", payload)
	}
	if payload.TotalCount != 2 || payload.HasNext != true {
		t.Fatalf("unexpected scheduler page metadata %+v", payload)
	}
}

func TestSchedulerEventsEndpointSupportsFiltersAndOffset(t *testing.T) {
	scheduler := &testGameScheduler{
		events: []apigateway.GameSchedulerEvent{
			{ID: 3, EventType: "tick_failed", Source: "scheduler", State: "running", TickID: 2, Message: "tick 2 failed", CreatedAt: "2026-03-10T10:02:12Z"},
			{ID: 2, EventType: "tick_completed", Source: "scheduler", State: "running", TickID: 1, Message: "tick 1 completed", CreatedAt: "2026-03-10T10:01:12Z"},
			{ID: 1, EventType: "started", Source: "organizer", State: "running", Message: "scheduler started", CreatedAt: "2026-03-10T10:01:00Z"},
		},
	}
	mux := newTestGameCoreMuxWithScheduler(testCheckerClient{}, scheduler)

	filteredRequest := httptest.NewRequest(http.MethodGet, "/internal/v1/game/scheduler/events?source=scheduler&state=running&event_type=tick_failed&limit=1", nil)
	filteredRequest.Header.Set("Authorization", "Bearer dev-admin-token")
	filteredResponse := httptest.NewRecorder()
	mux.ServeHTTP(filteredResponse, filteredRequest)
	if filteredResponse.Code != http.StatusOK {
		t.Fatalf("expected filtered scheduler events 200, got %d", filteredResponse.Code)
	}

	filteredPayload := decodeResponse[apigateway.GameSchedulerEventPage](t, filteredResponse.Body.Bytes())
	if len(filteredPayload.Items) != 1 || filteredPayload.Items[0].ID != 3 {
		t.Fatalf("unexpected filtered scheduler events payload %+v", filteredPayload)
	}
	if filteredPayload.TotalCount != 1 || filteredPayload.HasNext || filteredPayload.HasPrev {
		t.Fatalf("unexpected filtered scheduler event page metadata %+v", filteredPayload)
	}

	offsetRequest := httptest.NewRequest(http.MethodGet, "/internal/v1/game/scheduler/events?source=scheduler&offset=1&limit=1", nil)
	offsetRequest.Header.Set("Authorization", "Bearer dev-admin-token")
	offsetResponse := httptest.NewRecorder()
	mux.ServeHTTP(offsetResponse, offsetRequest)
	if offsetResponse.Code != http.StatusOK {
		t.Fatalf("expected scheduler events offset 200, got %d", offsetResponse.Code)
	}

	filteredPayload = decodeResponse[apigateway.GameSchedulerEventPage](t, offsetResponse.Body.Bytes())
	if len(filteredPayload.Items) != 1 || filteredPayload.Items[0].ID != 2 {
		t.Fatalf("unexpected scheduler events offset payload %+v", filteredPayload)
	}
	if !filteredPayload.HasPrev {
		t.Fatalf("expected scheduler event offset page to expose previous page %+v", filteredPayload)
	}
}

func TestSubmitFlagsAcceptsIssuedEnemyFlag(t *testing.T) {
	mux := newTestGameCoreMux(testCheckerClient{})
	startTestMatch(t, mux)

	advanceRequest := httptest.NewRequest(http.MethodPost, "/internal/v1/game/ticks/advance", nil)
	advanceRequest.Header.Set("Authorization", "Bearer dev-admin-token")
	advanceResponse := httptest.NewRecorder()
	mux.ServeHTTP(advanceResponse, advanceRequest)
	if advanceResponse.Code != http.StatusOK {
		t.Fatalf("expected advance 200, got %d", advanceResponse.Code)
	}

	flag := newFlagCodec("test-flag-secret").Issue(102, 1, 1, 1)
	submitRequest := httptest.NewRequest(http.MethodPost, "/internal/v1/flags/submit", bytes.NewBufferString(`{"team_id":101,"flags":["`+flag+`","`+flag+`"]}`))
	submitRequest.Header.Set("Authorization", "Bearer dev-admin-token")
	submitResponse := httptest.NewRecorder()
	mux.ServeHTTP(submitResponse, submitRequest)
	if submitResponse.Code != http.StatusOK {
		t.Fatalf("expected submit 200, got %d", submitResponse.Code)
	}

	payload := decodeResponse[[]apigateway.SubmissionVerdictAlias](t, submitResponse.Body.Bytes())
	if len(payload) != 2 {
		t.Fatalf("expected 2 verdicts, got %d", len(payload))
	}
	if payload[0].Detail != "flag is correct." || payload[1].Detail != "flag already submitted." {
		t.Fatalf("unexpected submit verdicts %+v", payload)
	}
}

func TestSubmitFlagsAcceptsSameFlagFromMultipleAttackers(t *testing.T) {
	mux := newTestGameCoreMux(testCheckerClient{})
	startTestMatch(t, mux)

	advanceRequest := httptest.NewRequest(http.MethodPost, "/internal/v1/game/ticks/advance", nil)
	advanceRequest.Header.Set("Authorization", "Bearer dev-admin-token")
	advanceResponse := httptest.NewRecorder()
	mux.ServeHTTP(advanceResponse, advanceRequest)
	if advanceResponse.Code != http.StatusOK {
		t.Fatalf("expected advance 200, got %d", advanceResponse.Code)
	}

	flag := newFlagCodec("test-flag-secret").Issue(102, 1, 1, 1)
	for _, teamID := range []int{101, 103} {
		submitRequest := httptest.NewRequest(http.MethodPost, "/internal/v1/flags/submit", bytes.NewBufferString(fmt.Sprintf(`{"team_id":%d,"flags":["%s"]}`, teamID, flag)))
		submitRequest.Header.Set("Authorization", "Bearer dev-admin-token")
		submitResponse := httptest.NewRecorder()
		mux.ServeHTTP(submitResponse, submitRequest)
		if submitResponse.Code != http.StatusOK {
			t.Fatalf("expected submit 200 for team %d, got %d", teamID, submitResponse.Code)
		}

		payload := decodeResponse[[]apigateway.SubmissionVerdictAlias](t, submitResponse.Body.Bytes())
		if len(payload) != 1 || payload[0].Detail != "flag is correct." {
			t.Fatalf("unexpected submit verdict for team %d: %+v", teamID, payload)
		}
	}

	scoreboardRequest := httptest.NewRequest(http.MethodPost, "/internal/v1/game/scoring/recompute", nil)
	scoreboardRequest.Header.Set("Authorization", "Bearer dev-admin-token")
	scoreboardResponse := httptest.NewRecorder()
	mux.ServeHTTP(scoreboardResponse, scoreboardRequest)
	if scoreboardResponse.Code != http.StatusOK {
		t.Fatalf("expected score recompute 200, got %d", scoreboardResponse.Code)
	}

	scoreboard := decodeResponse[[]apigateway.ScoreRowAlias](t, scoreboardResponse.Body.Bytes())
	rows := scoreRowsByTeam(scoreboard)
	if rows["Team Alpha"].Attack != 10 {
		t.Fatalf("expected first capture to be worth 10, got %+v", rows["Team Alpha"])
	}
	if rows["Team Sigma"].Attack != 5 {
		t.Fatalf("expected second capture to be worth 5, got %+v", rows["Team Sigma"])
	}
	if rows["Team Delta"].Defense != 700 {
		t.Fatalf("expected victim defense to drop once for the stolen flag, got %+v", rows["Team Delta"])
	}
}

func scoreRowsByTeam(rows []apigateway.ScoreRowAlias) map[string]apigateway.ScoreRowAlias {
	result := make(map[string]apigateway.ScoreRowAlias, len(rows))
	for _, row := range rows {
		result[row.Team] = row
	}
	return result
}

func TestSubmitFlagsRequiresRunningMatch(t *testing.T) {
	mux := newTestGameCoreMux(testCheckerClient{})

	request := httptest.NewRequest(http.MethodPost, "/internal/v1/flags/submit", bytes.NewBufferString(`{"team_id":101,"flags":["FLAGv1.demo"]}`))
	request.Header.Set("Authorization", "Bearer dev-admin-token")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected submit 400 before start, got %d", response.Code)
	}

	payload := decodeProblem(t, response.Body.Bytes())
	if payload.Detail != "contest has not started yet." {
		t.Fatalf("unexpected pre-start message %q", payload.Detail)
	}

	startTestMatch(t, mux)

	stopRequest := httptest.NewRequest(http.MethodPost, "/internal/v1/game/match/stop", nil)
	stopRequest.Header.Set("Authorization", "Bearer dev-admin-token")
	stopResponse := httptest.NewRecorder()
	mux.ServeHTTP(stopResponse, stopRequest)
	if stopResponse.Code != http.StatusOK {
		t.Fatalf("expected match stop 200, got %d", stopResponse.Code)
	}

	finishedRequest := httptest.NewRequest(http.MethodPost, "/internal/v1/flags/submit", bytes.NewBufferString(`{"team_id":101,"flags":["FLAGv1.demo"]}`))
	finishedRequest.Header.Set("Authorization", "Bearer dev-admin-token")
	finishedResponse := httptest.NewRecorder()
	mux.ServeHTTP(finishedResponse, finishedRequest)
	if finishedResponse.Code != http.StatusBadRequest {
		t.Fatalf("expected submit 400 after finish, got %d", finishedResponse.Code)
	}
	payload = decodeProblem(t, finishedResponse.Body.Bytes())
	if payload.Detail != "contest is over." {
		t.Fatalf("unexpected finished message %q", payload.Detail)
	}
}

func TestSubmitFlagsRejectsOwnAndExpiredFlags(t *testing.T) {
	mux := newTestGameCoreMux(testCheckerClient{})
	startTestMatch(t, mux)

	for idx := 0; idx < 4; idx++ {
		advanceRequest := httptest.NewRequest(http.MethodPost, "/internal/v1/game/ticks/advance", nil)
		advanceRequest.Header.Set("Authorization", "Bearer dev-admin-token")
		advanceResponse := httptest.NewRecorder()
		mux.ServeHTTP(advanceResponse, advanceRequest)
		if advanceResponse.Code != http.StatusOK {
			t.Fatalf("expected advance 200, got %d", advanceResponse.Code)
		}
	}

	codec := newFlagCodec("test-flag-secret")
	ownFlag := codec.Issue(101, 1, 4, 6)
	expiredFlag := codec.Issue(102, 1, 1, 1)

	submitRequest := httptest.NewRequest(http.MethodPost, "/internal/v1/flags/submit", bytes.NewBufferString(`{"team_id":101,"flags":["`+ownFlag+`","`+expiredFlag+`"]}`))
	submitRequest.Header.Set("Authorization", "Bearer dev-admin-token")
	submitResponse := httptest.NewRecorder()
	mux.ServeHTTP(submitResponse, submitRequest)
	if submitResponse.Code != http.StatusOK {
		t.Fatalf("expected submit 200, got %d", submitResponse.Code)
	}

	payload := decodeResponse[[]apigateway.SubmissionVerdictAlias](t, submitResponse.Body.Bytes())
	if payload[0].Detail != "flag is wrong or expired." || payload[1].Detail != "flag is wrong or expired." {
		t.Fatalf("unexpected submit verdicts %+v", payload)
	}
}

func TestScoreboardRecomputeReflectsCheckerAndSubmissionState(t *testing.T) {
	mux := newTestGameCoreMux(testCheckerClient{})
	startTestMatch(t, mux)

	advanceRequest := httptest.NewRequest(http.MethodPost, "/internal/v1/game/ticks/advance", nil)
	advanceRequest.Header.Set("Authorization", "Bearer dev-admin-token")
	advanceResponse := httptest.NewRecorder()
	mux.ServeHTTP(advanceResponse, advanceRequest)
	if advanceResponse.Code != http.StatusOK {
		t.Fatalf("expected advance 200, got %d", advanceResponse.Code)
	}

	flag := newFlagCodec("test-flag-secret").Issue(102, 1, 1, 1)
	submitRequest := httptest.NewRequest(http.MethodPost, "/internal/v1/flags/submit", bytes.NewBufferString(`{"team_id":101,"flags":["`+flag+`"]}`))
	submitRequest.Header.Set("Authorization", "Bearer dev-admin-token")
	submitResponse := httptest.NewRecorder()
	mux.ServeHTTP(submitResponse, submitRequest)
	if submitResponse.Code != http.StatusOK {
		t.Fatalf("expected submit 200, got %d", submitResponse.Code)
	}

	scoreboardRequest := httptest.NewRequest(http.MethodPost, "/internal/v1/game/scoring/recompute", nil)
	scoreboardRequest.Header.Set("Authorization", "Bearer dev-admin-token")
	scoreboardResponse := httptest.NewRecorder()
	mux.ServeHTTP(scoreboardResponse, scoreboardRequest)
	if scoreboardResponse.Code != http.StatusOK {
		t.Fatalf("expected score recompute 200, got %d", scoreboardResponse.Code)
	}

	payload := decodeResponse[[]apigateway.ScoreRowAlias](t, scoreboardResponse.Body.Bytes())
	if len(payload) != 4 {
		t.Fatalf("expected 4 score rows, got %d", len(payload))
	}
	if payload[0].Team != "Team Alpha" || payload[0].Attack != 10 || payload[0].Defense != 1000 || payload[0].SLA != 33 || payload[0].Total != 1043 {
		t.Fatalf("unexpected leading score row %+v", payload[0])
	}
	if len(payload[0].Services) != 3 {
		t.Fatalf("expected service breakdown on leading row, got %+v", payload[0].Services)
	}
	if payload[0].Services[0].Service != "banking" || payload[0].Services[0].Attack != 10 {
		t.Fatalf("unexpected leading service breakdown %+v", payload[0].Services[0])
	}
}

func TestAttackFeedEndpointReflectsAcceptedSubmissions(t *testing.T) {
	mux := newTestGameCoreMux(testCheckerClient{})
	startTestMatch(t, mux)

	advanceRequest := httptest.NewRequest(http.MethodPost, "/internal/v1/game/ticks/advance", nil)
	advanceRequest.Header.Set("Authorization", "Bearer dev-admin-token")
	advanceResponse := httptest.NewRecorder()
	mux.ServeHTTP(advanceResponse, advanceRequest)
	if advanceResponse.Code != http.StatusOK {
		t.Fatalf("expected advance 200, got %d", advanceResponse.Code)
	}

	flag := newFlagCodec("test-flag-secret").Issue(102, 1, 1, 1)
	submitRequest := httptest.NewRequest(http.MethodPost, "/internal/v1/flags/submit", bytes.NewBufferString(`{"team_id":101,"flags":["`+flag+`"]}`))
	submitRequest.Header.Set("Authorization", "Bearer dev-admin-token")
	submitResponse := httptest.NewRecorder()
	mux.ServeHTTP(submitResponse, submitRequest)
	if submitResponse.Code != http.StatusOK {
		t.Fatalf("expected submit 200, got %d", submitResponse.Code)
	}

	attackRequest := httptest.NewRequest(http.MethodGet, "/internal/v1/game/attacks?limit=1", nil)
	attackRequest.Header.Set("Authorization", "Bearer dev-admin-token")
	attackResponse := httptest.NewRecorder()
	mux.ServeHTTP(attackResponse, attackRequest)
	if attackResponse.Code != http.StatusOK {
		t.Fatalf("expected attacks 200, got %d", attackResponse.Code)
	}

	payload := decodeResponse[apigateway.AttackFeedPage](t, attackResponse.Body.Bytes())
	if len(payload.Items) != 1 {
		t.Fatalf("expected 1 attack event, got %d", len(payload.Items))
	}
	if payload.Items[0].Attacker != "Team Alpha" || payload.Items[0].Victim != "Team Delta" || payload.Items[0].Tick != 1 {
		t.Fatalf("unexpected attack event %+v", payload.Items[0])
	}
	if payload.TotalCount != 1 || payload.HasNext || payload.HasPrev {
		t.Fatalf("unexpected attack page metadata %+v", payload)
	}
}

func TestAttackFeedEndpointSupportsOffset(t *testing.T) {
	mux := newTestGameCoreMux(testCheckerClient{})
	startTestMatch(t, mux)

	advanceRequest := httptest.NewRequest(http.MethodPost, "/internal/v1/game/ticks/advance", nil)
	advanceRequest.Header.Set("Authorization", "Bearer dev-admin-token")
	advanceResponse := httptest.NewRecorder()
	mux.ServeHTTP(advanceResponse, advanceRequest)
	if advanceResponse.Code != http.StatusOK {
		t.Fatalf("expected advance 200, got %d", advanceResponse.Code)
	}

	codec := newFlagCodec("test-flag-secret")
	firstSubmit := httptest.NewRequest(http.MethodPost, "/internal/v1/flags/submit", bytes.NewBufferString(`{"team_id":101,"flags":["`+codec.Issue(102, 1, 1, 1)+`"]}`))
	firstSubmit.Header.Set("Authorization", "Bearer dev-admin-token")
	firstSubmitResponse := httptest.NewRecorder()
	mux.ServeHTTP(firstSubmitResponse, firstSubmit)
	if firstSubmitResponse.Code != http.StatusOK {
		t.Fatalf("expected first submit 200, got %d", firstSubmitResponse.Code)
	}

	secondSubmit := httptest.NewRequest(http.MethodPost, "/internal/v1/flags/submit", bytes.NewBufferString(`{"team_id":101,"flags":["`+codec.Issue(103, 2, 1, 1)+`"]}`))
	secondSubmit.Header.Set("Authorization", "Bearer dev-admin-token")
	secondSubmitResponse := httptest.NewRecorder()
	mux.ServeHTTP(secondSubmitResponse, secondSubmit)
	if secondSubmitResponse.Code != http.StatusOK {
		t.Fatalf("expected second submit 200, got %d", secondSubmitResponse.Code)
	}

	attackRequest := httptest.NewRequest(http.MethodGet, "/internal/v1/game/attacks?limit=1&offset=1", nil)
	attackRequest.Header.Set("Authorization", "Bearer dev-admin-token")
	attackResponse := httptest.NewRecorder()
	mux.ServeHTTP(attackResponse, attackRequest)
	if attackResponse.Code != http.StatusOK {
		t.Fatalf("expected attacks 200, got %d", attackResponse.Code)
	}

	payload := decodeResponse[apigateway.AttackFeedPage](t, attackResponse.Body.Bytes())
	if len(payload.Items) != 1 || payload.Items[0].Victim != "Team Delta" {
		t.Fatalf("unexpected paged attack feed %+v", payload)
	}
	if payload.TotalCount != 2 || !payload.HasPrev || payload.HasNext {
		t.Fatalf("unexpected paged attack metadata %+v", payload)
	}
}

func TestAttackFeedEndpointSupportsTextFilters(t *testing.T) {
	mux := newTestGameCoreMux(testCheckerClient{})
	startTestMatch(t, mux)

	advanceRequest := httptest.NewRequest(http.MethodPost, "/internal/v1/game/ticks/advance", nil)
	advanceRequest.Header.Set("Authorization", "Bearer dev-admin-token")
	advanceResponse := httptest.NewRecorder()
	mux.ServeHTTP(advanceResponse, advanceRequest)
	if advanceResponse.Code != http.StatusOK {
		t.Fatalf("expected advance 200, got %d", advanceResponse.Code)
	}

	codec := newFlagCodec("test-flag-secret")
	firstSubmit := httptest.NewRequest(http.MethodPost, "/internal/v1/flags/submit", bytes.NewBufferString(`{"team_id":101,"flags":["`+codec.Issue(102, 1, 1, 1)+`"]}`))
	firstSubmit.Header.Set("Authorization", "Bearer dev-admin-token")
	firstSubmitResponse := httptest.NewRecorder()
	mux.ServeHTTP(firstSubmitResponse, firstSubmit)
	if firstSubmitResponse.Code != http.StatusOK {
		t.Fatalf("expected first submit 200, got %d", firstSubmitResponse.Code)
	}

	secondSubmit := httptest.NewRequest(http.MethodPost, "/internal/v1/flags/submit", bytes.NewBufferString(`{"team_id":101,"flags":["`+codec.Issue(103, 2, 1, 1)+`"]}`))
	secondSubmit.Header.Set("Authorization", "Bearer dev-admin-token")
	secondSubmitResponse := httptest.NewRecorder()
	mux.ServeHTTP(secondSubmitResponse, secondSubmit)
	if secondSubmitResponse.Code != http.StatusOK {
		t.Fatalf("expected second submit 200, got %d", secondSubmitResponse.Code)
	}

	attackRequest := httptest.NewRequest(http.MethodGet, "/internal/v1/game/attacks?service=hat", nil)
	attackRequest.Header.Set("Authorization", "Bearer dev-admin-token")
	attackResponse := httptest.NewRecorder()
	mux.ServeHTTP(attackResponse, attackRequest)
	if attackResponse.Code != http.StatusOK {
		t.Fatalf("expected attacks 200, got %d", attackResponse.Code)
	}

	payload := decodeResponse[apigateway.AttackFeedPage](t, attackResponse.Body.Bytes())
	if len(payload.Items) != 1 || payload.Items[0].Service != "chat" {
		t.Fatalf("unexpected filtered attack feed %+v", payload)
	}
	if payload.TotalCount != 1 || payload.HasPrev || payload.HasNext {
		t.Fatalf("unexpected filtered attack metadata %+v", payload)
	}
}

func TestAttackFeedEndpointSupportsTickRange(t *testing.T) {
	mux := newTestGameCoreMux(testCheckerClient{})
	startTestMatch(t, mux)

	advanceRequest := httptest.NewRequest(http.MethodPost, "/internal/v1/game/ticks/advance", nil)
	advanceRequest.Header.Set("Authorization", "Bearer dev-admin-token")
	advanceResponse := httptest.NewRecorder()
	mux.ServeHTTP(advanceResponse, advanceRequest)
	if advanceResponse.Code != http.StatusOK {
		t.Fatalf("expected advance 200, got %d", advanceResponse.Code)
	}

	codec := newFlagCodec("test-flag-secret")
	firstSubmit := httptest.NewRequest(http.MethodPost, "/internal/v1/flags/submit", bytes.NewBufferString(`{"team_id":101,"flags":["`+codec.Issue(102, 1, 1, 1)+`"]}`))
	firstSubmit.Header.Set("Authorization", "Bearer dev-admin-token")
	firstSubmitResponse := httptest.NewRecorder()
	mux.ServeHTTP(firstSubmitResponse, firstSubmit)
	if firstSubmitResponse.Code != http.StatusOK {
		t.Fatalf("expected first submit 200, got %d", firstSubmitResponse.Code)
	}

	secondTickAdvance := httptest.NewRequest(http.MethodPost, "/internal/v1/game/ticks/advance", nil)
	secondTickAdvance.Header.Set("Authorization", "Bearer dev-admin-token")
	secondTickResponse := httptest.NewRecorder()
	mux.ServeHTTP(secondTickResponse, secondTickAdvance)
	if secondTickResponse.Code != http.StatusOK {
		t.Fatalf("expected second advance 200, got %d", secondTickResponse.Code)
	}

	secondSubmit := httptest.NewRequest(http.MethodPost, "/internal/v1/flags/submit", bytes.NewBufferString(`{"team_id":101,"flags":["`+codec.Issue(103, 2, 2, 2)+`"]}`))
	secondSubmit.Header.Set("Authorization", "Bearer dev-admin-token")
	secondSubmitResponse := httptest.NewRecorder()
	mux.ServeHTTP(secondSubmitResponse, secondSubmit)
	if secondSubmitResponse.Code != http.StatusOK {
		t.Fatalf("expected second submit 200, got %d", secondSubmitResponse.Code)
	}

	attackRequest := httptest.NewRequest(http.MethodGet, "/internal/v1/game/attacks?tick_from=2&tick_to=2", nil)
	attackRequest.Header.Set("Authorization", "Bearer dev-admin-token")
	attackResponse := httptest.NewRecorder()
	mux.ServeHTTP(attackResponse, attackRequest)
	if attackResponse.Code != http.StatusOK {
		t.Fatalf("expected attacks 200, got %d", attackResponse.Code)
	}

	payload := decodeResponse[apigateway.AttackFeedPage](t, attackResponse.Body.Bytes())
	if len(payload.Items) != 1 || payload.Items[0].Tick != 2 {
		t.Fatalf("unexpected tick-filtered attack feed %+v", payload)
	}
	if payload.TotalCount != 1 || payload.HasPrev || payload.HasNext {
		t.Fatalf("unexpected tick-filtered attack metadata %+v", payload)
	}
}
