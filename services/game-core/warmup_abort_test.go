package main

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"adplatform/internal/services/apigateway"
)

// firstPutHangs blocks the first put until cancelled and fails it; any later
// put succeeds immediately.
type firstPutHangs struct{ calls atomic.Int32 }

func (c *firstPutHangs) Execute(ctx context.Context, _ apigateway.CheckerExecutionRequest) (apigateway.CheckerExecutionResult, error) {
	if c.calls.Add(1) == 1 {
		<-ctx.Done()
		return apigateway.CheckerExecutionResult{Status: "failed"}, nil
	}
	return apigateway.CheckerExecutionResult{Status: "success"}, nil
}

func (c *firstPutHangs) ExecuteBatch(ctx context.Context, r apigateway.CheckerBatchExecutionRequest) (apigateway.CheckerBatchExecutionResult, error) {
	return executeBatchViaSingle(ctx, c.Execute, r)
}

// A warmup that times out must not count the targets it never reached as
// successful puts: with a success threshold below 100% that would let the match
// start while most services were never verified.
func TestTimedOutWarmupDoesNotCountUnattemptedTargets(t *testing.T) {
	checker := &firstPutHangs{}
	server := newGameCoreServer("dev-admin-token", newMemoryGameStore(), checker, newFlagCodec("s", "PLAYIT"), &testGameScheduler{}, []string{"put"}, 15).
		WithCheckerParallelism(1).
		WithWarmupPutRetries(0).
		WithWarmupMinSuccessRate(0.5).
		WithWarmupTimeout(50 * time.Millisecond)
	result, _ := server.runWarmup(context.Background())
	if result.TotalTargets < 2 {
		t.Fatalf("fixture needs several targets, got %d", result.TotalTargets)
	}
	attempted := int(checker.calls.Load())
	if result.PutSuccess > attempted-1 {
		t.Fatalf("%d puts were attempted (one failed) but %d were reported successful", attempted, result.PutSuccess)
	}
	if result.PutSuccess+result.PutFailed != result.TotalTargets {
		t.Fatalf("every target must be accounted for: %+v", result)
	}
	if result.Status == "passed" {
		t.Fatalf("an aborted warmup that verified %d/%d targets must not pass: %+v", result.PutSuccess, result.TotalTargets, result)
	}
}
