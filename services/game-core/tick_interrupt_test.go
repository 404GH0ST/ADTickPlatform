package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"adplatform/internal/platform/httpapi"
	"adplatform/internal/services/apigateway"
)

// cancellingChecker cancels the tick from inside the first checker call, the
// way a match pause lands while a checker container is running.
type cancellingChecker struct{ cancel context.CancelFunc }

func (c cancellingChecker) Execute(ctx context.Context, _ apigateway.CheckerExecutionRequest) (apigateway.CheckerExecutionResult, error) {
	c.cancel()
	<-ctx.Done()
	return apigateway.CheckerExecutionResult{}, ctx.Err()
}

func (c cancellingChecker) ExecuteBatch(ctx context.Context, r apigateway.CheckerBatchExecutionRequest) (apigateway.CheckerBatchExecutionResult, error) {
	return executeBatchViaSingle(ctx, c.Execute, r)
}

// ctxAwareChecker fails any call whose context is already done and otherwise
// behaves like the healthy test checker.
type ctxAwareChecker struct{}

func (ctxAwareChecker) Execute(ctx context.Context, r apigateway.CheckerExecutionRequest) (apigateway.CheckerExecutionResult, error) {
	if err := ctx.Err(); err != nil {
		return apigateway.CheckerExecutionResult{}, err
	}
	return testCheckerClient{}.Execute(ctx, r)
}

func (c ctxAwareChecker) ExecuteBatch(ctx context.Context, r apigateway.CheckerBatchExecutionRequest) (apigateway.CheckerBatchExecutionResult, error) {
	return executeBatchViaSingle(ctx, c.Execute, r)
}

// A target whose checker is cut off by a pause is recorded as skipped in every
// phase and its service state is unknown, not down.
func TestInterruptedTickDoesNotMarkServiceDown(t *testing.T) {
	store := newMemoryGameStore().(*memoryGameStore)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv := newGameCoreServer("test", store, cancellingChecker{cancel}, newFlagCodec("test-secret", "PLAYIT"), &testGameScheduler{}, []string{"put", "get", "check"}, 1).
		WithWarmupRequired(false).WithAutoTickOnMatchStart(false)
	srv.scoringDebounce = time.Hour
	if _, err := store.StartMatch(context.Background(), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}

	tick, err := srv.advanceTick(ctx)
	if err != nil {
		t.Fatalf("interrupted tick returned an error: %v", err)
	}
	if tick.SkippedCheckerRuns != 3 || tick.FailedCheckerRuns != 0 {
		t.Fatalf("interrupted target should record 3 skipped and 0 failed runs: %+v", tick)
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	runs := 0
	for _, run := range store.runs {
		if run.TickID != tick.ID {
			continue
		}
		runs++
		if run.Status != "skipped" || run.Message != "checker interrupted: tick cancelled before this target finished" {
			t.Fatalf("phase %s recorded as %q (%q)", run.Phase, run.Status, run.Message)
		}
	}
	if runs != 3 {
		t.Fatalf("expected 3 recorded runs, got %d", runs)
	}
	for key, state := range store.serviceStates {
		if key.TickID == tick.ID && state.Status != "unknown" {
			t.Fatalf("interrupted target service state = %q, want unknown", state.Status)
		}
	}
}

// A manual tick advance must not be interrupted when the HTTP caller goes
// away; the tick timeout, not the caller's deadline, bounds the checkers.
func TestManualTickAdvanceSurvivesCallerDisconnect(t *testing.T) {
	store := newMemoryGameStore().(*memoryGameStore)
	srv := newGameCoreServer("test", store, ctxAwareChecker{}, newFlagCodec("test-secret", "PLAYIT"), &testGameScheduler{}, []string{"put", "get", "check"}, 1).
		WithWarmupRequired(false).WithAutoTickOnMatchStart(false)
	srv.scoringDebounce = time.Hour
	if _, err := store.StartMatch(context.Background(), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "game-core", Version: "dev", Addr: ":0"})
	srv.RegisterRoutes(mux)

	gone, cancel := context.WithCancel(context.Background())
	cancel()
	request := httptest.NewRequest(http.MethodPost, "/internal/v1/game/ticks/advance", nil).WithContext(gone)
	request.Header.Set("Authorization", "Bearer test")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("advance: %d %s", response.Code, response.Body.String())
	}

	status, err := store.GameStatus(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	tick := status.CurrentTick
	if tick == nil || tick.Status != "completed" || tick.SkippedCheckerRuns != 0 || tick.SuccessfulCheckerRuns == 0 {
		t.Fatalf("caller disconnect interrupted the tick: %+v", tick)
	}
}
