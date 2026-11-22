package main

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"adplatform/internal/services/apigateway"
)

// missingFlagChecker reports every get as a confirmed missing flag.
type missingFlagChecker struct{}

func (missingFlagChecker) Execute(_ context.Context, request apigateway.CheckerExecutionRequest) (apigateway.CheckerExecutionResult, error) {
	if request.Phase == "get" {
		return apigateway.CheckerExecutionResult{Status: "failed", ExitCode: flagMissingExitCode}, nil
	}
	return apigateway.CheckerExecutionResult{Status: "success"}, nil
}

func (c missingFlagChecker) ExecuteBatch(ctx context.Context, r apigateway.CheckerBatchExecutionRequest) (apigateway.CheckerBatchExecutionResult, error) {
	return executeBatchViaSingle(ctx, c.Execute, r)
}

func tickedServer(t *testing.T, checker checkerClient) (*gameCoreServer, gameStore) {
	t.Helper()
	ctx := context.Background()
	store := newMemoryGameStore()
	server := newGameCoreServer("dev-admin-token", store, checker, newFlagCodec("s", "PLAYIT"), &testGameScheduler{}, []string{"put", "get", "check"}, 15).
		WithWarmupRequired(false).WithAutoTickOnMatchStart(false).WithScoringDebounce(time.Hour)
	if _, err := store.StartMatch(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := server.advanceTick(ctx); err != nil {
		t.Fatal(err)
	}
	return server, store
}

func TestConfirmedMissingFlagIsPenalized(t *testing.T) {
	ctx := context.Background()
	server, store := tickedServer(t, missingFlagChecker{})
	server.checkerTimeout = 0 // exercises the default retention timeout
	if err := server.verifyRetainedFlags(ctx, true); err != nil {
		t.Fatal(err)
	}
	page, _ := store.ListCheckerRuns(ctx, apigateway.GameCheckerRunQuery{Limit: 500, Phase: "get"})
	confirmed := 0
	for _, run := range page.Items {
		if strings.Contains(run.Message, "confirmed missing") {
			confirmed++
		}
	}
	if confirmed == 0 {
		t.Fatalf("a confirmed missing flag must be recorded as such: %+v", page.Items)
	}
}

func TestRetentionCheckBranches(t *testing.T) {
	ctx := context.Background()
	server, store := tickedServer(t, testCheckerClient{})
	targets, _ := store.ListCheckerTargets(ctx)
	flag, err := store.LatestIssuedFlag(ctx, targets[0].TeamID, targets[0].ChallengeID)
	if err != nil {
		t.Fatal(err)
	}
	mismatch := targets[0]
	mismatch.CheckerToken = "other"
	if err := server.checkFlagRetention(ctx, ctx, mismatch, flag); err != nil {
		t.Fatal("a flag issued under another checker token is skipped")
	}
	unavailable := newGameCoreServer("dev-admin-token", store, erroringChecker{}, newFlagCodec("s", "PLAYIT"), &testGameScheduler{}, []string{"get"}, 15)
	if err := unavailable.checkFlagRetention(ctx, ctx, targets[0], flag); err != nil {
		t.Fatalf("an unavailable checker is recorded, not fatal: %v", err)
	}

	server.checkerParallelism = 0
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if sweep, err := server.sweepRetainedFlags(ctx, cancelled, true); err != nil || sweep.Complete {
		t.Fatalf("a cancelled sweep is incomplete, not an error: %+v %v", sweep, err)
	}
	if _, err := server.sweepRetainedFlags(cancelled, cancelled, true); err == nil {
		t.Fatal("cancelling the persistence context is an error")
	}
	fresh := newTickServer(newMemoryGameStore())
	if sweep, err := fresh.sweepRetainedFlags(ctx, ctx, false); err != nil || !sweep.Complete {
		t.Fatalf("no tick yet: nothing to verify: %+v %v", sweep, err)
	}
	if _, err := newTickServer(failingGameStore{}).sweepRetainedFlags(ctx, ctx, true); err == nil {
		t.Fatal("an unreadable game status fails the sweep")
	}
	listing := &targetsFailAfterStatus{gameStore: store}
	if _, err := newTickServer(listing).sweepRetainedFlags(ctx, ctx, true); err == nil {
		t.Fatal("a target listing failure fails the sweep")
	}
	noFlags := &noIssuedFlags{gameStore: store}
	if sweep, err := newTickServer(noFlags).sweepRetainedFlags(ctx, ctx, true); err != nil || !sweep.Complete {
		t.Fatalf("targets without flags are skipped: %+v %v", sweep, err)
	}
}

type targetsFailAfterStatus struct{ gameStore }

func (targetsFailAfterStatus) ListCheckerTargets(context.Context) ([]checkerTarget, error) {
	return nil, errScripted
}

type noIssuedFlags struct{ gameStore }

func (noIssuedFlags) LatestIssuedFlag(context.Context, int, int) (issuedFlagRecord, error) {
	return issuedFlagRecord{}, sql.ErrNoRows
}

// appendFails fails scheduler event writes only.
type appendFails struct{ gameStore }

func (appendFails) AppendSchedulerEvent(context.Context, schedulerEventRecord) (apigateway.GameSchedulerEvent, error) {
	return apigateway.GameSchedulerEvent{}, errScripted
}

func TestStopWithRetentionBudgetExhausted(t *testing.T) {
	ctx := context.Background()
	server, store := tickedServer(t, testCheckerClient{})
	server.checker = blockingChecker{} // only the final retention sweep blocks
	server.store = appendFails{gameStore: store}
	server.finalRetentionSweepBudget = 20 * time.Millisecond
	status, err := server.stopMatchWithRetention(ctx, time.Now())
	if err != nil || status.State != "finished" {
		t.Fatalf("a match stops even when the final sweep runs out of budget: %+v %v", status, err)
	}
	failing := newTickServer(failingGameStore{})
	if _, err := failing.stopMatchWithRetention(ctx, time.Now()); err == nil {
		t.Fatal("an unreadable match state fails the stop")
	}
	recompute := &recomputeFails{gameStore: newMemoryGameStore()}
	if _, err := newTickServer(recompute).stopMatchWithRetention(ctx, time.Now()); err == nil {
		t.Fatal("a failed final recompute fails the stop")
	}
}

type recomputeFails struct{ gameStore }

func (recomputeFails) RecomputeScoreboard(context.Context) ([]apigateway.ScoreRowAlias, error) {
	return nil, errScripted
}

func TestRetentionMonitorLoop(t *testing.T) {
	server, store := tickedServer(t, testCheckerClient{})
	server.retentionMonitorInterval = 5 * time.Millisecond
	stop := server.startRetentionMonitor()
	time.Sleep(40 * time.Millisecond)
	stop()

	failing := newTickServer(&statusFailsAfterMatch{gameStore: store})
	failing.retentionMonitorInterval = 5 * time.Millisecond
	stop = failing.startRetentionMonitor()
	time.Sleep(30 * time.Millisecond)
	stop()

	idle := newTickServer(newMemoryGameStore())
	idle.retentionMonitorInterval = 5 * time.Millisecond
	stop = idle.startRetentionMonitor()
	time.Sleep(20 * time.Millisecond)
	stop()
	newTickServer(newMemoryGameStore()).startRetentionMonitor()() // default interval
}

type statusFailsAfterMatch struct{ gameStore }

func (statusFailsAfterMatch) GameStatus(context.Context) (apigateway.GameStatus, error) {
	return apigateway.GameStatus{}, errScripted
}

func TestMemoryRetentionStoreGuards(t *testing.T) {
	ctx := context.Background()
	_, store := tickedServer(t, testCheckerClient{})
	mem := store.(*memoryGameStore)
	if err := mem.ScheduleRetentionCheck(ctx, "PLAYIT{unknown}", time.Now()); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("unknown flags cannot be scheduled: %v", err)
	}
	targets, _ := mem.ListCheckerTargets(ctx)
	flag, _ := mem.LatestIssuedFlag(ctx, targets[0].TeamID, targets[0].ChallengeID)
	run := checkerRunRecord{TickID: flag.IssuedTick, TeamID: flag.OwnerTeamID, ChallengeID: flag.ChallengeID, Phase: "get", Status: "failed"}
	if _, err := mem.RecordRetentionCheck(cancelledContext(), flag, run, false); err == nil {
		t.Fatal("a cancelled context must stop the record")
	}
	if _, err := mem.RecordRetentionCheck(ctx, flag, checkerRunRecord{Phase: "put"}, false); err == nil {
		t.Fatal("a run for another flag must be rejected")
	}
	stale := flag
	stale.Flag = "PLAYIT{superseded}"
	if recorded, err := mem.RecordRetentionCheck(ctx, stale, run, false); err != nil || recorded {
		t.Fatalf("a superseded flag is ignored: %v %v", recorded, err)
	}
}

// A tick whose scoreboard recompute fails is reported as failed, even though
// the tick itself is already stored.
func TestTickFailsWhenScoreboardRecomputeFails(t *testing.T) {
	ctx := context.Background()
	store := recomputeFails{gameStore: newMemoryGameStore()}
	if _, err := store.StartMatch(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := newTickServer(store).advanceTick(ctx); !errors.Is(err, errScripted) {
		t.Fatalf("want the recompute failure, got %v", err)
	}
}
