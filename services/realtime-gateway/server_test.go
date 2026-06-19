package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"adplatform/internal/platform/httpapi"
	"adplatform/internal/services/apigateway"
)

type testSnapshotClient struct {
	mu         sync.Mutex
	scoreboard []apigateway.ScoreRowAlias
	attacks    []apigateway.AttackEventAlias
	status     apigateway.GameStatus
	events     []apigateway.GameSchedulerEvent
	runs       []apigateway.GameCheckerRun
}

func (c *testSnapshotClient) Scoreboard(context.Context) ([]apigateway.ScoreRowAlias, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]apigateway.ScoreRowAlias(nil), c.scoreboard...), nil
}

func (c *testSnapshotClient) Attacks(context.Context) (apigateway.AttackFeedPage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return apigateway.AttackFeedPage{
		Items:      append([]apigateway.AttackEventAlias(nil), c.attacks...),
		Limit:      len(c.attacks),
		Offset:     0,
		TotalCount: len(c.attacks),
	}, nil
}

func (c *testSnapshotClient) GameStatus(context.Context) (apigateway.GameStatus, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.status, nil
}

func (c *testSnapshotClient) SchedulerEvents(_ context.Context, limit int) (apigateway.GameSchedulerEventPage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	page := apigateway.GameSchedulerEventPage{
		Limit:      limit,
		Offset:     0,
		TotalCount: len(c.events),
	}
	if page.Limit <= 0 {
		page.Limit = len(c.events)
	}
	if limit > 0 && limit < len(c.events) {
		page.HasNext = true
		page.Items = append([]apigateway.GameSchedulerEvent(nil), c.events[:limit]...)
		return page, nil
	}
	page.Items = append([]apigateway.GameSchedulerEvent(nil), c.events...)
	return page, nil
}

func (c *testSnapshotClient) CheckerRuns(_ context.Context, limit int) (apigateway.GameCheckerRunPage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	page := apigateway.GameCheckerRunPage{
		Limit:      limit,
		Offset:     0,
		TotalCount: len(c.runs),
	}
	if page.Limit <= 0 {
		page.Limit = len(c.runs)
	}
	if limit > 0 && limit < len(c.runs) {
		page.HasNext = true
		page.Items = append([]apigateway.GameCheckerRun(nil), c.runs[:limit]...)
		return page, nil
	}
	page.Items = append([]apigateway.GameCheckerRun(nil), c.runs...)
	return page, nil
}

func TestScoreboardStreamEmitsInitialAndUpdatedSnapshots(t *testing.T) {
	client := &testSnapshotClient{
		scoreboard: []apigateway.ScoreRowAlias{
			{Rank: 1, Team: "Team Alpha", Attack: 40, Defense: 30, SLA: 28, Total: 98, Delta: "+1"},
		},
		attacks: []apigateway.AttackEventAlias{
			{ID: "atk-1", Attacker: "Team Alpha", Victim: "Team Delta", Service: "banking", Tick: 1, Verdict: "first valid submission accepted"},
		},
		status: apigateway.GameStatus{TotalTicks: 1},
		events: []apigateway.GameSchedulerEvent{
			{ID: 1, EventType: "started", Source: "organizer", State: "running", CreatedAt: "2026-03-10T10:00:00Z"},
		},
		runs: []apigateway.GameCheckerRun{
			{ID: 1, TickID: 1, TeamID: 101, TeamName: "Team Alpha", ChallengeID: 1, ChallengeName: "banking", Phase: "put", Target: "10.80.1.11:10001", Status: "success", ExitCode: 0, CheckedAt: "2026-03-10T10:00:01Z"},
		},
	}

	gateway := newRealtimeGateway(client, 25*time.Millisecond, "dev-admin-token")
	if err := gateway.syncOnce(context.Background()); err != nil {
		t.Fatalf("failed to prime gateway: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	request := httptest.NewRequest("GET", "/public/v1/scoreboard/stream", nil).WithContext(ctx)
	response := newSafeRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		gateway.handleScoreboardStream(response, request)
	}()

	waitForBodyContains(t, response, `"team":"Team Alpha"`)

	client.mu.Lock()
	client.scoreboard = []apigateway.ScoreRowAlias{
		{Rank: 1, Team: "Team Sigma", Attack: 60, Defense: 40, SLA: 30, Total: 130, Delta: "+2"},
	}
	client.mu.Unlock()
	if err := gateway.syncOnce(context.Background()); err != nil {
		t.Fatalf("failed to resync gateway: %v", err)
	}

	waitForBodyContains(t, response, `"team":"Team Sigma"`)
	cancel()
	<-done
}

func TestAttackStreamEmitsInitialSnapshot(t *testing.T) {
	client := &testSnapshotClient{
		scoreboard: []apigateway.ScoreRowAlias{
			{Rank: 1, Team: "Team Alpha", Attack: 40, Defense: 30, SLA: 28, Total: 98, Delta: "+1"},
		},
		attacks: []apigateway.AttackEventAlias{
			{ID: "atk-1", Attacker: "Team Alpha", Victim: "Team Delta", Service: "banking", Tick: 1, Verdict: "first valid submission accepted"},
		},
		status: apigateway.GameStatus{TotalTicks: 1},
		events: []apigateway.GameSchedulerEvent{
			{ID: 1, EventType: "started", Source: "organizer", State: "running", CreatedAt: "2026-03-10T10:00:00Z"},
		},
		runs: []apigateway.GameCheckerRun{
			{ID: 1, TickID: 1, TeamID: 101, TeamName: "Team Alpha", ChallengeID: 1, ChallengeName: "banking", Phase: "put", Target: "10.80.1.11:10001", Status: "success", ExitCode: 0, CheckedAt: "2026-03-10T10:00:01Z"},
		},
	}

	gateway := newRealtimeGateway(client, 25*time.Millisecond, "dev-admin-token")
	if err := gateway.syncOnce(context.Background()); err != nil {
		t.Fatalf("failed to prime gateway: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	request := httptest.NewRequest("GET", "/public/v1/attacks/stream", nil).WithContext(ctx)
	response := newSafeRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		gateway.handleAttackStream(response, request)
	}()

	waitForBodyContains(t, response, `"id":"atk-1"`)
	cancel()
	<-done
}

func TestPublicScoreboardStreamWorksThroughInstrumentedMux(t *testing.T) {
	info := httpapi.ServiceInfo{Name: "realtime-gateway-stream-" + strings.ToLower(strings.ReplaceAll(t.Name(), "/", "-")), Version: "test", Addr: ":0"}
	client := &testSnapshotClient{
		scoreboard: []apigateway.ScoreRowAlias{
			{Rank: 1, Team: "Team Alpha", Attack: 40, Defense: 30, SLA: 28, Total: 98, Delta: "+1"},
		},
	}

	gateway := newRealtimeGateway(client, 25*time.Millisecond, "dev-admin-token")
	if err := gateway.syncOnce(context.Background()); err != nil {
		t.Fatalf("failed to prime gateway: %v", err)
	}

	mux := httpapi.NewBaseMux(info)
	gateway.RegisterRoutes(mux)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	request := httptest.NewRequest(http.MethodGet, "/public/v1/scoreboard/stream", nil).WithContext(ctx)
	response := newSafeRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		mux.ServeHTTP(response, request)
	}()

	waitForBodyContains(t, response, `"team":"Team Alpha"`)
	cancel()
	<-done

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}
}

func TestAdminStreamsRequireAdminAuth(t *testing.T) {
	gateway := newRealtimeGateway(&testSnapshotClient{}, 25*time.Millisecond, "dev-admin-token")
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "realtime-gateway", Version: "dev", Addr: ":0"})
	gateway.RegisterRoutes(mux)
	cases := []struct {
		path string
	}{
		{"/admin/v1/game/scoreboard/stream"},
		{"/admin/v1/game/status/stream"},
		{"/admin/v1/game/checker-runs/stream"},
		{"/admin/v1/game/scheduler/events/stream"},
	}

	for _, tc := range cases {
		t.Run(tc.path+" unauthenticated", func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, tc.path, nil)
			response := newSafeRecorder()

			mux.ServeHTTP(response, request)

			if response.Code != http.StatusForbidden {
				t.Fatalf("expected 403, got %d: %s", response.Code, response.Body.String())
			}
		})

		t.Run(tc.path+" wrong token", func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, tc.path, nil)
			request.Header.Set("Authorization", "Bearer wrong-token")
			response := newSafeRecorder()

			mux.ServeHTTP(response, request)

			if response.Code != http.StatusForbidden {
				t.Fatalf("expected 403, got %d: %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestAdminScoreboardStreamEmitsSnapshot(t *testing.T) {
	client := &testSnapshotClient{
		scoreboard: []apigateway.ScoreRowAlias{
			{Rank: 1, Team: "Team Alpha", Attack: 40, Defense: 30, SLA: 28, Total: 98, Delta: "+1"},
		},
	}

	gateway := newRealtimeGateway(client, 25*time.Millisecond, "dev-admin-token")
	if err := gateway.syncKind(context.Background(), streamScoreboard); err != nil {
		t.Fatalf("failed to prime admin scoreboard stream: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	request := httptest.NewRequest("GET", "/admin/v1/game/scoreboard/stream", nil).WithContext(ctx)
	request.Header.Set("Authorization", "Bearer dev-admin-token")
	response := newSafeRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		gateway.handleAdminScoreboardStream(response, request)
	}()

	waitForBodyContains(t, response, `"team":"Team Alpha"`)
	cancel()
	<-done
}

func TestAdminGameStatusStreamEmitsInitialAndUpdatedSnapshots(t *testing.T) {
	client := &testSnapshotClient{
		status: apigateway.GameStatus{
			CurrentTick: &apigateway.GameTickStatus{ID: 1, Status: "completed"},
			TotalTicks:  1,
		},
	}

	gateway := newRealtimeGateway(client, 25*time.Millisecond, "dev-admin-token")
	if err := gateway.syncKind(context.Background(), streamGameStatus); err != nil {
		t.Fatalf("failed to prime game status stream: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	request := httptest.NewRequest("GET", "/admin/v1/game/status/stream", nil).WithContext(ctx)
	request.Header.Set("Authorization", "Bearer dev-admin-token")
	response := newSafeRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		gateway.handleAdminGameStatusStream(response, request)
	}()

	waitForBodyContains(t, response, `"total_ticks":1`)

	client.mu.Lock()
	client.status = apigateway.GameStatus{
		CurrentTick: &apigateway.GameTickStatus{ID: 2, Status: "completed"},
		TotalTicks:  2,
	}
	client.mu.Unlock()
	if err := gateway.syncKind(context.Background(), streamGameStatus); err != nil {
		t.Fatalf("failed to resync game status stream: %v", err)
	}

	waitForBodyContains(t, response, `"total_ticks":2`)
	cancel()
	<-done
}

func TestAdminSchedulerEventsStreamEmitsSnapshot(t *testing.T) {
	client := &testSnapshotClient{
		events: []apigateway.GameSchedulerEvent{
			{ID: 2, EventType: "tick_completed", Source: "scheduler", State: "running", TickID: 9, CreatedAt: "2026-03-10T10:09:00Z"},
			{ID: 1, EventType: "started", Source: "restore", State: "running", CreatedAt: "2026-03-10T10:08:00Z"},
		},
	}

	gateway := newRealtimeGateway(client, 25*time.Millisecond, "dev-admin-token")
	if err := gateway.syncKind(context.Background(), streamSchedulerEvents); err != nil {
		t.Fatalf("failed to prime scheduler events stream: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	request := httptest.NewRequest("GET", "/admin/v1/game/scheduler/events/stream", nil).WithContext(ctx)
	request.Header.Set("Authorization", "Bearer dev-admin-token")
	response := newSafeRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		gateway.handleAdminSchedulerEventsStream(response, request)
	}()

	waitForBodyContains(t, response, `"event_type":"tick_completed"`)
	cancel()
	<-done
}

func TestAdminCheckerRunsStreamEmitsSnapshot(t *testing.T) {
	client := &testSnapshotClient{
		runs: []apigateway.GameCheckerRun{
			{ID: 7, TickID: 9, TeamID: 101, TeamName: "Team Alpha", ChallengeID: 1, ChallengeName: "banking", Phase: "get", Target: "10.80.1.11:10001", Status: "failed", ExitCode: 1, CheckedAt: "2026-03-10T10:09:03Z"},
		},
	}

	gateway := newRealtimeGateway(client, 25*time.Millisecond, "dev-admin-token")
	if err := gateway.syncKind(context.Background(), streamCheckerRuns); err != nil {
		t.Fatalf("failed to prime checker runs stream: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	request := httptest.NewRequest("GET", "/admin/v1/game/checker-runs/stream", nil).WithContext(ctx)
	request.Header.Set("Authorization", "Bearer dev-admin-token")
	response := newSafeRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		gateway.handleAdminCheckerRunsStream(response, request)
	}()

	waitForBodyContains(t, response, `"phase":"get"`)
	cancel()
	<-done
}

func waitForBodyContains(t *testing.T, response *safeRecorder, want string) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(response.BodyString(), want) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for response body to contain %q; body=%q", want, response.BodyString())
}

func TestMetricsEndpointIncludesRealtimeGatewayMetrics(t *testing.T) {
	info := httpapi.ServiceInfo{Name: "realtime-gateway-metrics-" + strings.ToLower(strings.ReplaceAll(t.Name(), "/", "-")), Version: "test", Addr: ":0"}
	client := &testSnapshotClient{
		scoreboard: []apigateway.ScoreRowAlias{
			{Rank: 1, Team: "Team Alpha", Attack: 40, Defense: 30, SLA: 28, Total: 98, Delta: "+1"},
		},
		attacks: []apigateway.AttackEventAlias{
			{ID: "atk-1", Attacker: "Team Alpha", Victim: "Team Delta", Service: "banking", Tick: 1, Verdict: "first valid submission accepted"},
		},
		status: apigateway.GameStatus{TotalTicks: 1},
		events: []apigateway.GameSchedulerEvent{
			{ID: 1, EventType: "started", Source: "organizer", State: "running", CreatedAt: "2026-03-10T10:00:00Z"},
		},
		runs: []apigateway.GameCheckerRun{
			{ID: 1, TickID: 1, TeamID: 101, TeamName: "Team Alpha", ChallengeID: 1, ChallengeName: "banking", Phase: "put", Target: "10.80.1.11:10001", Status: "success", ExitCode: 0, CheckedAt: "2026-03-10T10:00:01Z"},
		},
	}

	gateway := newRealtimeGateway(client, 25*time.Millisecond, "dev-admin-token")
	httpapi.RegisterMetricsSource(info.Name, gateway)
	if err := gateway.syncOnce(context.Background()); err != nil {
		t.Fatalf("failed to prime gateway metrics: %v", err)
	}

	mux := httpapi.NewBaseMux(info)
	gateway.RegisterRoutes(mux)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request := httptest.NewRequest(http.MethodGet, "/public/v1/scoreboard/stream", nil).WithContext(ctx)
	response := newSafeRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		gateway.handleScoreboardStream(response, request)
	}()
	waitForBodyContains(t, response, `"team":"Team Alpha"`)
	cancel()
	<-done

	metricsRequest := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	metricsResponse := httptest.NewRecorder()
	mux.ServeHTTP(metricsResponse, metricsRequest)

	body := metricsResponse.Body.String()
	for _, fragment := range []string{
		"adplatform_realtime_gateway_poll_interval_seconds 0.025000",
		`adplatform_realtime_gateway_last_sync_success 1`,
		`adplatform_realtime_gateway_subscribers{stream="scoreboard"} 0`,
		`adplatform_realtime_gateway_snapshot_bytes{stream="scoreboard"} `,
	} {
		if !strings.Contains(body, fragment) {
			t.Fatalf("expected metrics output to contain %q, got:\n%s", fragment, body)
		}
	}
}

type safeRecorder struct {
	*httptest.ResponseRecorder
	mu sync.Mutex
}

func newSafeRecorder() *safeRecorder {
	return &safeRecorder{
		ResponseRecorder: httptest.NewRecorder(),
	}
}

func (r *safeRecorder) Write(b []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ResponseRecorder.Write(b)
}

func (r *safeRecorder) BodyString() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ResponseRecorder.Body.String()
}
