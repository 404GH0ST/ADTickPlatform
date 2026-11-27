package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"adplatform/internal/services/apigateway"
)

func TestCheckerRotationTransportFailureAfterPermitKeepsPut(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		controller := http.NewResponseController(w)
		if err := controller.EnableFullDuplex(); err != nil {
			t.Error(err)
			return
		}
		reader := json.NewDecoder(r.Body)
		var request apigateway.CheckerRotationRequest
		if err := reader.Decode(&request); err != nil {
			t.Error(err)
			return
		}
		writer := json.NewEncoder(w)
		old := apigateway.CheckerPhaseResult{Phase: "get", ExitCode: 42, Status: "failed"}
		if err := writer.Encode(apigateway.CheckerRotationEvent{Kind: "retention", Result: old}); err != nil {
			t.Error(err)
			return
		}
		if err := controller.Flush(); err != nil {
			t.Error(err)
			return
		}
		var permit bool
		if err := reader.Decode(&permit); err != nil || !permit {
			t.Errorf("missing write permit: %v, %v", permit, err)
			return
		}
		put := apigateway.CheckerPhaseResult{Phase: "put", Status: "success"}
		if err := writer.Encode(apigateway.CheckerRotationEvent{Kind: "phase", Result: put}); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	client := &httpCheckerClient{baseURL: server.URL, client: &http.Client{Timeout: 3 * time.Second}}
	result, err := client.Rotate(context.Background(), apigateway.CheckerRotationRequest{}, func(old apigateway.CheckerPhaseResult) error {
		if old.ExitCode != 42 {
			t.Errorf("unexpected previous-flag verdict: %+v", old)
		}
		return nil
	})
	if !errors.Is(err, io.EOF) || errors.Is(err, errRotationUnavailable) || len(result.Phases) != 1 || result.Phases[0].Phase != "put" {
		t.Fatalf("rotation lost completed put or allowed a replay: %+v, %v", result, err)
	}
}

type testRotationChecker struct {
	testCheckerClient
	targets int
	started atomic.Int32
	writes  atomic.Int32
}

func (c *testRotationChecker) Rotate(_ context.Context, _ apigateway.CheckerRotationRequest, persist func(apigateway.CheckerPhaseResult) error) (apigateway.CheckerBatchExecutionResult, error) {
	c.started.Add(1)
	if err := persist(apigateway.CheckerPhaseResult{Phase: "get", Status: "success"}); err != nil {
		return apigateway.CheckerBatchExecutionResult{}, err
	}
	c.writes.Add(1)
	return apigateway.CheckerBatchExecutionResult{Phases: []apigateway.CheckerPhaseResult{
		{Phase: "put", Status: "success", Output: "new flag metadata"},
		{Phase: "get", Status: "success", Output: "read-back metadata"},
	}}, nil
}

func (c *testRotationChecker) Execute(ctx context.Context, request apigateway.CheckerExecutionRequest) (apigateway.CheckerExecutionResult, error) {
	if request.TickID == 2 && request.Phase == "check" {
		if c.started.Load() != int32(c.targets) || request.Metadata != "read-back metadata" || request.TimeoutSeconds != 30 {
			return apigateway.CheckerExecutionResult{}, fmt.Errorf("health ran before rotation or lost its read-back metadata or deadline")
		}
	}
	return c.testCheckerClient.Execute(ctx, request)
}

type failingRotationRetentionStore struct {
	gameStore
	err error
}

func (s failingRotationRetentionStore) RecordRetentionCheck(context.Context, issuedFlagRecord, checkerRunRecord, bool) (bool, error) {
	return false, s.err
}

func TestRotationPrioritizesFlagsAndRequiresDurableRetention(t *testing.T) {
	ctx := context.Background()
	for _, storeFails := range []bool{false, true} {
		name := "all rotations precede health"
		if storeFails {
			name = "persistence failure blocks overwrite"
		}
		t.Run(name, func(t *testing.T) {
			store := newMemoryGameStore()
			targets, err := store.ListCheckerTargets(ctx)
			if err != nil {
				t.Fatal(err)
			}
			checker := &testRotationChecker{targets: len(targets)}
			server := newGameCoreServer("admin", store, checker, newFlagCodec("test-secret", "PLAYIT"), &testGameScheduler{}, []string{"put", "get", "check"}, 15).
				WithCheckerParallelism(1).
				WithWarmupRequired(false).
				WithAutoTickOnMatchStart(false)
			if _, err := store.StartMatch(ctx, time.Now()); err != nil {
				t.Fatal(err)
			}
			if tick, err := server.advanceTick(ctx); err != nil || tick.SuccessfulCheckerRuns != len(targets)*3 {
				t.Fatalf("seed tick: %+v, %v", tick, err)
			}
			expectedErr := errors.New("retention write failed")
			if storeFails {
				server.store = failingRotationRetentionStore{gameStore: store, err: expectedErr}
			}
			tick, err := server.advanceTick(ctx)
			if storeFails {
				if !errors.Is(err, expectedErr) || checker.writes.Load() != 0 {
					t.Fatalf("persistence error did not block rotation: %v, writes=%d", err, checker.writes.Load())
				}
				return
			}
			if err != nil || tick.SuccessfulCheckerRuns != len(targets)*3 || checker.writes.Load() != int32(len(targets)) {
				t.Fatalf("rotation tick: %+v, %v, writes=%d", tick, err, checker.writes.Load())
			}
		})
	}
}
