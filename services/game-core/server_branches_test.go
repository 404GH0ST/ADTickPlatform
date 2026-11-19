package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"adplatform/internal/platform/httpapi"
	"adplatform/internal/services/apigateway"
)

var errScripted = errors.New("scripted failure")

// failingScheduler reports errors from every mutating call.
type failingScheduler struct{ noopGameScheduler }

func (failingScheduler) Stop() (apigateway.GameSchedulerStatus, error) {
	return apigateway.GameSchedulerStatus{}, errScripted
}
func (failingScheduler) Update(time.Duration) (apigateway.GameSchedulerStatus, error) {
	return apigateway.GameSchedulerStatus{}, errScripted
}
func (failingScheduler) Events(context.Context, apigateway.GameSchedulerEventQuery) (apigateway.GameSchedulerEventPage, error) {
	return apigateway.GameSchedulerEventPage{}, errScripted
}

// runningScheduler reports running so match stops must stop it.
type runningScheduler struct {
	noopGameScheduler
	stops int
}

func (s *runningScheduler) Status() apigateway.GameSchedulerStatus {
	return apigateway.GameSchedulerStatus{State: "running"}
}
func (s *runningScheduler) Stop() (apigateway.GameSchedulerStatus, error) {
	s.stops++
	return apigateway.GameSchedulerStatus{State: "stopped"}, nil
}

type erroringChecker struct{}

func (erroringChecker) Execute(context.Context, apigateway.CheckerExecutionRequest) (apigateway.CheckerExecutionResult, error) {
	return apigateway.CheckerExecutionResult{}, errScripted
}
func (erroringChecker) ExecuteBatch(context.Context, apigateway.CheckerBatchExecutionRequest) (apigateway.CheckerBatchExecutionResult, error) {
	return apigateway.CheckerBatchExecutionResult{}, errScripted
}

// blockingChecker waits until the call is cancelled.
type blockingChecker struct{}

func (blockingChecker) Execute(ctx context.Context, _ apigateway.CheckerExecutionRequest) (apigateway.CheckerExecutionResult, error) {
	<-ctx.Done()
	return apigateway.CheckerExecutionResult{Status: "failed"}, nil
}
func (b blockingChecker) ExecuteBatch(ctx context.Context, r apigateway.CheckerBatchExecutionRequest) (apigateway.CheckerBatchExecutionResult, error) {
	return executeBatchViaSingle(ctx, b.Execute, r)
}

type failingAccess struct{ calls int }

func (f *failingAccess) ReconcileAccess(context.Context) error {
	f.calls++
	return errScripted
}

func serverMux(server *gameCoreServer) *http.ServeMux {
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "game-core", Version: "test", Addr: ":0"})
	server.RegisterRoutes(mux)
	return mux
}

func call(mux *http.ServeMux, method, path, body string) *httptest.ResponseRecorder {
	return gameCall(mux, method, path, body)
}

func TestEveryGameCoreRouteRequiresTheInternalToken(t *testing.T) {
	mux := gameMux(newMemoryGameStore())
	for _, route := range gameCoreRoutes(t) {
		path := strings.ReplaceAll(route[1], "{submission_id}", "1")
		for _, token := range []string{"", "Bearer wrong"} {
			request := httptest.NewRequest(route[0], path, bytes.NewBufferString(`{}`))
			if token != "" {
				request.Header.Set("Authorization", token)
			}
			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusForbidden {
				t.Errorf("%s %s with %q: want 403, got %d", route[0], path, token, recorder.Code)
			}
		}
	}
}

func TestServerConfigurationClamps(t *testing.T) {
	server := newTickServer(newMemoryGameStore())
	if server.WithWarmupMinSuccessRate(-1).warmupMinSuccess != 0 || server.WithWarmupMinSuccessRate(7).warmupMinSuccess != 1 {
		t.Fatal("warmup success rates are clamped to [0,1]")
	}
	server.WithScoringRetryDelay(-1).WithScoringTimeout(-1).WithWarmupTimeout(0)
	if nonNegativeDuration(-time.Second) != 0 || normalizedCheckerRunStatus("Skipped") != "skipped" {
		t.Fatal("duration and status helpers")
	}
	if rotated := rotateCheckerTargets([]checkerTarget{{TeamID: 1}, {TeamID: 2}, {TeamID: 3}}, -1); len(rotated) != 3 {
		t.Fatal("negative tick ids rotate safely")
	}
	if parseOptionalRFC3339("") != nil || parseOptionalRFC3339("nope") != nil {
		t.Fatal("invalid optional times parse to nil")
	}
	request := httptest.NewRequest(http.MethodGet, "/?limit=abc&offset=-1", nil)
	if parsePositiveQueryInt(request, "limit", 7, 10) != 7 || parseNonNegativeQueryInt(request, "offset") != 0 {
		t.Fatal("invalid query ints fall back")
	}
}

func TestWarmupOutcomes(t *testing.T) {
	ctx := context.Background()
	// No targets: the warmup passes vacuously.
	empty := newMemoryGameStore().(*memoryGameStore)
	empty.challenges = map[int]memoryChallenge{}
	if result, err := newTickServer(empty).runWarmup(ctx); err != nil || result.Status != "passed" {
		t.Fatalf("vacuous warmup: %+v %v", result, err)
	}

	// A second warmup during one in flight returns the last result.
	server := newTickServer(newMemoryGameStore())
	server.warmupInFlight = true
	server.lastWarmup = apigateway.GameWarmupResult{Status: "running"}
	if result, err := server.runWarmup(ctx); err != nil || result.Status != "running" {
		t.Fatalf("concurrent warmup: %+v %v", result, err)
	}

	// Transport errors fail every target after the retry budget.
	failing := newGameCoreServer("dev-admin-token", newMemoryGameStore(), erroringChecker{}, newFlagCodec("s", "PLAYIT"), &testGameScheduler{}, []string{"put"}, 15).
		WithCheckerParallelism(64)
	failing.checkerParallelism = 0
	failing.warmupPutRetries = -1 // the setter rejects negatives; exercise the clamp directly
	if result, _ := failing.runWarmup(ctx); result.Status != "failed" || result.PutFailed != result.TotalTargets {
		t.Fatalf("checker transport errors fail the warmup: %+v", result)
	}

	// An aborted warmup reports every target as failed.
	aborted := newGameCoreServer("dev-admin-token", newMemoryGameStore(), blockingChecker{}, newFlagCodec("s", "PLAYIT"), &testGameScheduler{}, []string{"put"}, 15).
		WithWarmupTimeout(30 * time.Millisecond).WithWarmupPutRetries(3).WithWarmupPutRetryDelay(time.Hour)
	if result, _ := aborted.runWarmup(ctx); result.PutSuccess != 0 || result.SuccessRate != 0 {
		t.Fatalf("an aborted warmup is a total failure: %+v", result)
	}
	if err := aborted.runWarmupForTarget(cancelledContext(), checkerTarget{TeamID: 1, ChallengeID: 1}); err == nil {
		t.Fatal("a cancelled retry wait must stop")
	}

	listing := newTickServer(failingGameStore{})
	if _, err := listing.runWarmup(ctx); err == nil {
		t.Fatal("a target listing failure fails the warmup")
	}
}

func cancelledContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func TestWarmupAndMatchHandlers(t *testing.T) {
	ctx := context.Background()
	store := newMemoryGameStore()
	server := newTickServer(store).WithWarmupRequired(true)
	mux := serverMux(server)

	if response := call(mux, http.MethodPost, "/internal/v1/game/warmup", ""); response.Code != http.StatusOK {
		t.Fatalf("pre-match warmup: %d %s", response.Code, response.Body.String())
	}
	if response := call(mux, http.MethodGet, "/internal/v1/game/warmup", ""); response.Code != http.StatusOK {
		t.Fatalf("read warmup: %d", response.Code)
	}
	access := &failingAccess{}
	server.accessReconcile = access
	server.autoTickOnMatchStart = true
	if response := call(mux, http.MethodPost, "/internal/v1/game/match/start", ""); response.Code != http.StatusOK {
		t.Fatalf("start: %d %s", response.Code, response.Body.String())
	}
	waitFor(t, "the auto tick and its access reconcile", func() bool {
		status, _ := store.GameStatus(ctx)
		return status.TotalTicks >= 1 && access.calls >= 2
	})
	if response := call(mux, http.MethodPost, "/internal/v1/game/warmup", ""); response.Code != http.StatusOK {
		t.Fatalf("a warmup during the match returns the last result: %d", response.Code)
	}
	if response := call(mux, http.MethodPost, "/internal/v1/game/match/stop", ""); response.Code != http.StatusOK {
		t.Fatalf("stop: %d", response.Code)
	}
	for _, path := range []string{"/internal/v1/game/match/pause", "/internal/v1/game/match/resume", "/internal/v1/game/match/start"} {
		if response := call(mux, http.MethodPost, path, ""); response.Code != http.StatusConflict {
			t.Errorf("%s after the match ended: want 409, got %d", path, response.Code)
		}
	}

	failingStart := newGameCoreServer("dev-admin-token", faultingGameStore{inner: newMemoryGameStore(), faults: &callFaults{}}, erroringChecker{}, newFlagCodec("s", "PLAYIT"), &testGameScheduler{}, []string{"put"}, 15).WithWarmupPutRetries(0)
	if response := call(serverMux(failingStart), http.MethodPost, "/internal/v1/game/match/start", ""); response.Code < 400 {
		t.Fatalf("a failed required warmup blocks the start: %d", response.Code)
	}

	// A tick already in progress is a conflict; contest errors are bad requests.
	busy := newTickServer(newMemoryGameStore())
	busy.tickMu.Lock()
	if response := call(serverMux(busy), http.MethodPost, "/internal/v1/game/ticks/advance", ""); response.Code != http.StatusConflict {
		t.Fatalf("a concurrent tick: %d", response.Code)
	}
	busy.tickMu.Unlock()
	if response := call(serverMux(busy), http.MethodPost, "/internal/v1/game/ticks/advance", ""); response.Code != http.StatusBadRequest {
		t.Fatalf("a tick before the match: %d", response.Code)
	}
	if response := call(serverMux(newTickServer(failingGameStore{})), http.MethodPost, "/internal/v1/game/warmup", ""); response.Code != http.StatusUnprocessableEntity && response.Code != http.StatusInternalServerError {
		t.Fatalf("a failing warmup: %d", response.Code)
	}
}

func TestSchedulerAndScheduleHandlers(t *testing.T) {
	failing := newGameCoreServer("dev-admin-token", newMemoryGameStore(), testCheckerClient{}, newFlagCodec("s", "PLAYIT"), failingScheduler{}, []string{"put"}, 15)
	mux := serverMux(failing)
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, "/internal/v1/game/scheduler/events", ""},
		{http.MethodPost, "/internal/v1/game/scheduler/stop", ""},
		{http.MethodPut, "/internal/v1/game/scheduler/interval", `{"interval_seconds":30}`},
	} {
		if response := call(mux, tc.method, tc.path, tc.body); response.Code != http.StatusInternalServerError {
			t.Errorf("%s %s: want 500, got %d", tc.method, tc.path, response.Code)
		}
	}
	mux = gameMux(newMemoryGameStore())
	for _, body := range []string{`{"interval_seconds":0}`} {
		if response := call(mux, http.MethodPut, "/internal/v1/game/scheduler/interval", body); response.Code != http.StatusBadRequest {
			t.Errorf("interval %s: %d", body, response.Code)
		}
	}
	for _, body := range []string{`{"scheduled_start_at":"soon"}`, `{"scheduled_end_at":"later"}`, `{"scheduled_start_at":"2026-11-19T18:00:00Z","scheduled_end_at":"2026-11-19T10:00:00Z"}`} {
		if response := call(mux, http.MethodPut, "/internal/v1/game/match/schedule", body); response.Code != http.StatusBadRequest {
			t.Errorf("schedule %s: %d", body, response.Code)
		}
	}
	scheduler := &runningScheduler{}
	stopper := newGameCoreServer("dev-admin-token", newMemoryGameStore(), testCheckerClient{}, newFlagCodec("s", "PLAYIT"), scheduler, []string{"put"}, 15).WithWarmupRequired(false).WithAutoTickOnMatchStart(false)
	stopMux := serverMux(stopper)
	call(stopMux, http.MethodPost, "/internal/v1/game/match/start", "")
	if response := call(stopMux, http.MethodPost, "/internal/v1/game/match/stop", ""); response.Code != http.StatusOK || scheduler.stops == 0 {
		t.Fatalf("stopping the match stops a running scheduler: %d %d", response.Code, scheduler.stops)
	}
}

func TestMatchWindowFillsFinishedTimes(t *testing.T) {
	ctx := context.Background()
	store := newMemoryGameStore()
	server := newTickServer(store)
	start, end := time.Now().Add(-2*time.Hour), time.Now().Add(-time.Hour)
	server.matchStartAt, server.matchEndAt = &start, &end
	if _, err := store.StartMatch(ctx, start); err != nil {
		t.Fatal(err)
	}
	if _, err := store.StopMatch(ctx, end); err != nil {
		t.Fatal(err)
	}
	mem := store.(*memoryGameStore)
	mem.match.StartedAt, mem.match.EndedAt = "", ""
	status, err := server.matchStatus(ctx)
	if err != nil || status.StartedAt == "" || status.EndedAt == "" || status.AcceptingSubmissions {
		t.Fatalf("a finished scheduled match reports its window: %+v %v", status, err)
	}
	if _, err := newTickServer(failingGameStore{}).submitFlags(ctx, 1, []string{"x"}); err == nil {
		t.Fatal("submissions fail when the match state is unreadable")
	}
}

func TestTickAccessReconcileFailureIsNotFatal(t *testing.T) {
	ctx := context.Background()
	store := newMemoryGameStore()
	server := newTickServer(store)
	access := &failingAccess{}
	server.accessReconcile = access
	if _, err := store.StartMatch(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := server.advanceTick(ctx); err != nil || access.calls != 1 {
		t.Fatalf("an access reconcile failure must not fail the tick: %v %d", err, access.calls)
	}
}

func TestAccessReconcileClient(t *testing.T) {
	if newAccessReconcileClient("", "t") != nil {
		t.Fatal("no URL means no client")
	}
	var mu sync.Mutex
	var auth string
	status := http.StatusOK
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		auth = r.Header.Get("Authorization")
		w.WriteHeader(status)
	}))
	defer server.Close()
	client := newAccessReconcileClient(server.URL+"/", " ctl-token ")
	if err := client.ReconcileAccess(context.Background()); err != nil || auth != "Bearer ctl-token" {
		t.Fatalf("success: %v %q", err, auth)
	}
	status = http.StatusBadGateway
	if err := client.ReconcileAccess(context.Background()); err == nil {
		t.Fatal("a non-2xx answer must fail")
	}
	if err := newAccessReconcileClient("http://127.0.0.1:1", "").ReconcileAccess(context.Background()); err == nil {
		t.Fatal("transport failure must fail")
	}
	if err := (&httpAccessReconcileClient{baseURL: "://bad", client: http.DefaultClient}).ReconcileAccess(context.Background()); err == nil {
		t.Fatal("invalid URL must fail")
	}
}

func TestGameCoreMetricsIncludeOptionalFields(t *testing.T) {
	ctx := context.Background()
	store := newMemoryGameStore()
	server := newTickServer(store)
	if _, err := store.StartMatch(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := store.StopMatch(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	server.scoringStatus.LastSuccessAt = time.Now()
	server.scoringStatus.LastFailureAt = time.Now()
	var out bytes.Buffer
	server.WritePrometheusMetrics(&out)
	for _, want := range []string{"scoreboard_recompute_last_success_unixtime", "scoreboard_recompute_last_failure_unixtime", "match_ended_at_unixtime"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %s:\n%s", want, out.String())
		}
	}
	out.Reset()
	newTickServer(failingGameStore{}).WritePrometheusMetrics(&out)
	if !strings.Contains(out.String(), "metrics_collection_success 0") {
		t.Fatalf("a failing store must report unsuccessful collection:\n%s", out.String())
	}
	if !parseMetricTime("bogus").IsZero() {
		t.Fatal("bad metric time")
	}
}

func TestScoringRecomputeTimers(t *testing.T) {
	server := newTickServer(newMemoryGameStore()).WithScoringDebounce(time.Hour).WithScoringTimeout(0)
	server.scheduleScoreboardRecompute()
	server.scheduleScoreboardRecompute() // replaces the pending timer
	server.runScheduledScoreboardRecompute()
	failing := newTickServer(failingGameStore{}).WithScoringRetryDelay(time.Hour)
	failing.runScheduledScoreboardRecompute()
	failing.runScheduledScoreboardRecompute() // a retry timer is already pending
	if status := failing.snapshotScoringStatus(); status.FailureCount != 2 || status.LastError == "" {
		t.Fatalf("failures are counted: %+v", status)
	}
	server.scoringMu.Lock()
	if server.scoringTimer != nil {
		server.scoringTimer.Stop()
	}
	server.scoringMu.Unlock()
	failing.scoringMu.Lock()
	if failing.scoringTimer != nil {
		failing.scoringTimer.Stop()
	}
	failing.scoringMu.Unlock()
}
