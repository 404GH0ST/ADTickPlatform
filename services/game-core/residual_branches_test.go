package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"adplatform/internal/platform/database/dbtest"
	"adplatform/internal/services/apigateway"
)

// startScript makes the match look not started but fails the start itself.
type startScript struct {
	gameStore
	startErr error
}

func (s startScript) StartMatch(context.Context, time.Time) (apigateway.GameMatchStatus, error) {
	return apigateway.GameMatchStatus{}, s.startErr
}

func TestStartMatchHandlerOutcomes(t *testing.T) {
	ctx := context.Background()
	running := newMemoryGameStore()
	if _, err := running.StartMatch(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	if response := call(serverMux(newTickServer(running)), http.MethodPost, "/internal/v1/game/match/start", ""); response.Code != http.StatusOK {
		t.Fatalf("starting a running match is a no-op: %d", response.Code)
	}
	for err, want := range map[error]int{errContestOver: http.StatusConflict, errScripted: http.StatusInternalServerError} {
		server := newTickServer(startScript{gameStore: newMemoryGameStore(), startErr: err})
		if response := call(serverMux(server), http.MethodPost, "/internal/v1/game/match/start", ""); response.Code != want {
			t.Errorf("start failing with %v: want %d, got %d", err, want, response.Code)
		}
	}
	listing := newTickServer(&targetsFailAfterStatus{gameStore: newMemoryGameStore()}).WithWarmupRequired(true)
	if response := call(serverMux(listing), http.MethodPost, "/internal/v1/game/match/start", ""); response.Code != http.StatusInternalServerError {
		t.Fatalf("a warmup that cannot list targets: %d", response.Code)
	}
	ticking := newTickServer(newMemoryGameStore())
	ticking.autoTickOnMatchStart = true
	ticking.tickMu.Lock() // the auto tick finds one in progress and only logs
	response := call(serverMux(ticking), http.MethodPost, "/internal/v1/game/match/start", "")
	ticking.tickMu.Unlock()
	if response.Code != http.StatusOK {
		t.Fatalf("a failed auto tick does not fail the start: %d", response.Code)
	}
}

type noTargets struct{ gameStore }

func (noTargets) ListCheckerTargets(context.Context) ([]checkerTarget, error) { return nil, nil }

type concurrencyLimited struct {
	gameStore
	limit int
}

func (c concurrencyLimited) CheckerTargetConcurrencyLimit() int { return c.limit }

// lateSuccessChecker succeeds after the warmup deadline, ignoring cancellation.
type lateSuccessChecker struct{ delay time.Duration }

func (c lateSuccessChecker) Execute(context.Context, apigateway.CheckerExecutionRequest) (apigateway.CheckerExecutionResult, error) {
	time.Sleep(c.delay)
	return apigateway.CheckerExecutionResult{Status: "success"}, nil
}
func (c lateSuccessChecker) ExecuteBatch(ctx context.Context, r apigateway.CheckerBatchExecutionRequest) (apigateway.CheckerBatchExecutionResult, error) {
	return executeBatchViaSingle(ctx, c.Execute, r)
}

func TestWarmupEdgeCounts(t *testing.T) {
	ctx := context.Background()
	if result, err := newTickServer(noTargets{gameStore: newMemoryGameStore()}).runWarmup(ctx); err != nil || result.Status != "passed" || result.SuccessRate != 1 {
		t.Fatalf("no targets passes vacuously: %+v %v", result, err)
	}
	wide := newTickServer(newMemoryGameStore()).WithCheckerParallelism(1000)
	if result, err := wide.runWarmup(ctx); err != nil || result.Status != "passed" {
		t.Fatalf("more workers than targets: %+v %v", result, err)
	}
	late := newGameCoreServer("dev-admin-token", newMemoryGameStore(), lateSuccessChecker{delay: 40 * time.Millisecond}, newFlagCodec("s", "PLAYIT"), &testGameScheduler{}, []string{"put"}, 15).
		WithWarmupTimeout(10 * time.Millisecond).WithWarmupPutRetries(0)
	result, _ := late.runWarmup(ctx)
	if result.Status == "passed" || len(result.Failures) != 1 || !strings.Contains(result.Failures[0].Error, "aborted") {
		t.Fatalf("an abort with no explicit failure reports the abort: %+v", result)
	}
	// A cancelled warmup may hand a worker a job after the deadline; that job
	// is skipped. Repeat so the scheduler's random choice reaches it.
	parallel := newGameCoreServer("dev-admin-token", newMemoryGameStore(), blockingChecker{}, newFlagCodec("s", "PLAYIT"), &testGameScheduler{}, []string{"put"}, 15).
		WithCheckerParallelism(2).WithWarmupPutRetries(0)
	for range 25 {
		result, _ := parallel.runWarmup(cancelledContext())
		if result.Status == "passed" {
			t.Fatal("a cancelled warmup never passes")
		}
	}
}

func TestCheckerParallelismBounds(t *testing.T) {
	ctx := context.Background()
	for name, setup := range map[string]func() *gameCoreServer{
		"zero": func() *gameCoreServer { s := newTickServer(newMemoryGameStore()); s.checkerParallelism = 0; return s },
		"store limit": func() *gameCoreServer {
			return newTickServer(concurrencyLimited{gameStore: newMemoryGameStore(), limit: 2}).WithCheckerParallelism(10)
		},
		"above targets": func() *gameCoreServer { return newTickServer(newMemoryGameStore()).WithCheckerParallelism(1000) },
	} {
		server := setup()
		if _, err := server.store.StartMatch(ctx, time.Now()); err != nil {
			t.Fatal(err)
		}
		if _, err := server.advanceTick(ctx); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

// partialBatch reports only the put phase, as a halted batch does.
type partialBatch struct{}

func (partialBatch) Execute(context.Context, apigateway.CheckerExecutionRequest) (apigateway.CheckerExecutionResult, error) {
	return apigateway.CheckerExecutionResult{Status: "success"}, nil
}
func (partialBatch) ExecuteBatch(_ context.Context, r apigateway.CheckerBatchExecutionRequest) (apigateway.CheckerBatchExecutionResult, error) {
	return apigateway.CheckerBatchExecutionResult{Phases: []apigateway.CheckerPhaseResult{{Phase: "put", Status: "success"}}}, nil
}

// recordFails fails checker run persistence; lookupFails fails issued-flag
// lookups; retentionFails fails retention records.
type recordFails struct{ gameStore }

func (recordFails) RecordCheckerRun(context.Context, checkerRunRecord) (apigateway.GameCheckerRun, error) {
	return apigateway.GameCheckerRun{}, errScripted
}

type lookupFails struct{ gameStore }

func (lookupFails) LookupIssuedFlag(context.Context, string) (issuedFlagRecord, error) {
	return issuedFlagRecord{}, errScripted
}

type retentionFails struct{ gameStore }

func (retentionFails) RecordRetentionCheck(context.Context, issuedFlagRecord, checkerRunRecord, bool) (bool, error) {
	return false, errScripted
}

type staleAndRecordFails struct{ recordFails }

func (staleAndRecordFails) CheckerTargetCurrent(context.Context, checkerTarget) (bool, error) {
	return false, nil
}

func TestTickPhaseHandling(t *testing.T) {
	ctx := context.Background()
	tick := func(store gameStore, checker checkerClient) error {
		server := newGameCoreServer("dev-admin-token", store, checker, newFlagCodec("s", "PLAYIT"), &testGameScheduler{}, []string{"put", "get", "check"}, 15).
			WithWarmupRequired(false).WithAutoTickOnMatchStart(false)
		if _, err := store.StartMatch(ctx, time.Now()); err != nil {
			return err
		}
		_, err := server.advanceTick(ctx)
		return err
	}
	if err := tick(newMemoryGameStore(), partialBatch{}); err != nil {
		t.Fatalf("absent phases after a failure are skipped: %v", err)
	}
	if err := tick(staleAndRecordFails{recordFails{newMemoryGameStore()}}, testCheckerClient{}); err == nil {
		t.Fatal("a failed record of a changed target fails the tick")
	}
	if err := tick(lookupFails{newMemoryGameStore()}, missingFlagChecker{}); err == nil {
		t.Fatal("a failed flag lookup during a confirmed-missing get fails the tick")
	}
	if err := tick(retentionFails{newMemoryGameStore()}, missingFlagChecker{}); err == nil {
		t.Fatal("a failed retention record during a tick fails the tick")
	}
}

func TestSubmitFlagEdges(t *testing.T) {
	ctx := context.Background()
	server, store := tickedServer(t, testCheckerClient{})
	forged := server.issueFlag(101, 1, 1, 99) // validly signed, never issued
	results, err := server.submitFlags(ctx, 102, []string{forged})
	if err != nil || results[0].Status != "invalid" {
		t.Fatalf("a signed but never-issued flag is invalid: %+v %v", results, err)
	}
	statusFails := newTickServer(&statusFailsAfterMatch{gameStore: store})
	if _, err := statusFails.submitFlags(ctx, 102, []string{"x"}); err == nil {
		t.Fatal("an unreadable game status fails the submission")
	}
}

// rescheduling triggers a new recompute request while one is running.
type rescheduling struct {
	gameStore
	server *gameCoreServer
	err    error
	once   sync.Once
}

func (r *rescheduling) RecomputeScoreboard(ctx context.Context) ([]apigateway.ScoreRowAlias, error) {
	r.once.Do(func() { r.server.scheduleScoreboardRecompute() })
	return nil, r.err
}

func TestRecomputeRestartsWhenRequestedMidRun(t *testing.T) {
	for _, err := range []error{nil, errScripted} {
		store := &rescheduling{gameStore: newMemoryGameStore(), err: err}
		server := newTickServer(store).WithScoringDebounce(time.Hour).WithScoringRetryDelay(time.Hour)
		store.server = server
		server.runScheduledScoreboardRecompute()
		server.scoringMu.Lock()
		if server.scoringTimer == nil {
			t.Errorf("err=%v: a mid-run request must leave a follow-up scheduled", err)
		} else {
			server.scoringTimer.Stop()
		}
		server.scoringMu.Unlock()
	}
}

func TestMemoryStoreGates(t *testing.T) {
	ctx := context.Background()
	server, store := tickedServer(t, testCheckerClient{})
	_ = server
	mem := store.(*memoryGameStore)
	targets, _ := mem.ListCheckerTargets(ctx)
	maintained, deferred := targets[0].ChallengeID, targets[len(targets)-1].ChallengeID
	mem.setChallengeMaintenanceForTest(maintained, true)
	if maintained != deferred {
		mem.setChallengePlayFromTickForTest(deferred, 99)
	}
	remaining, _ := mem.ListCheckerTargets(ctx)
	for _, target := range remaining {
		if target.ChallengeID == maintained || target.ChallengeID == deferred {
			t.Fatalf("maintained or deferred challenges are not checked: %+v", target)
		}
	}
	if blocked, _ := mem.IsChallengeInMaintenance(ctx, deferred); !blocked {
		t.Fatal("a challenge deferred past the current tick is blocked")
	}
	mem.setChallengeMaintenanceForTest(maintained, false)
	mem.setChallengePlayFromTickForTest(maintained, 99)
	if accepted, err := mem.AcceptFlagSubmission(ctx, acceptedFlagSubmission{Flag: "x", SubmittingTeam: 1, ChallengeID: maintained, ExpiresTick: 50}); accepted || !errors.Is(err, errFlagNoLongerValid) {
		t.Fatalf("captures on a deferred challenge are rejected: %v %v", accepted, err)
	}
	if limit, offset := submissionPageBounds(apigateway.SubmissionQuery{AttackFeedQuery: apigateway.AttackFeedQuery{Limit: 5000, Offset: -4}}); limit != 200 || offset != 0 {
		t.Fatalf("submission paging bounds: %d %d", limit, offset)
	}
	at := time.Now()
	for _, flag := range []string{"a", "b"} {
		if err := mem.RecordSubmission(ctx, apigateway.AdminSubmission{TeamID: 1, Flag: flag, Status: "invalid", SubmittedAt: at}); err != nil {
			t.Fatal(err)
		}
	}
	page, _ := mem.ListSubmissions(ctx, apigateway.SubmissionQuery{AttackFeedQuery: apigateway.AttackFeedQuery{Limit: 10}, Status: "invalid"})
	if len(page.Items) < 2 || page.Items[0].ID < page.Items[1].ID {
		t.Fatalf("equal timestamps sort newest id first: %+v", page.Items)
	}
	if response := call(gameMux(mem), http.MethodDelete, "/internal/v1/game/submissions/abc", ""); response.Code != http.StatusBadRequest {
		t.Fatalf("a malformed submission id: %d", response.Code)
	}
	if nullTimeValue("not-a-time") != nil {
		t.Fatal("unparseable times store as NULL")
	}
}

func TestPostgresStartStopAndFeedEdges(t *testing.T) {
	ctx := context.Background()
	db := migratedGameDB(t, "gc_start_stop", false)
	store := newPostgresGameStore(db)
	if _, err := store.DeleteSubmission(ctx, 424242); !errors.Is(err, errSubmissionNotFound) {
		t.Fatalf("deleting an unknown submission: %v", err)
	}
	if _, err := store.RecordRetentionCheck(ctx, issuedFlagRecord{IssuedTick: 1}, checkerRunRecord{Phase: "put"}, false); err == nil {
		t.Fatal("a run for another flag must be rejected")
	}
	mustExec(t, db, `DELETE FROM game_scheduler_state`)
	if status, err := store.LoadSchedulerState(ctx, 9); err != nil || status.IntervalSeconds != 9 {
		t.Fatalf("a missing scheduler row uses defaults: %+v %v", status, err)
	}
	mustExec(t, db, `INSERT INTO attack_events(id, attacker, victim, service, tick, verdict, created_at) VALUES ('a1', 'Alpha', 'Bravo', 'web', 3, 'ok', NOW())`)
	for _, query := range []apigateway.AttackFeedQuery{{Victim: "Zulu"}, {Service: "pwn"}, {TickFrom: 9}, {TickTo: 1}} {
		page, err := store.ListAttackFeed(ctx, query)
		if err != nil || page.TotalCount != 0 {
			t.Errorf("filter %+v leaked the event: %+v %v", query, page, err)
		}
	}

	faulty, faults := faultyGameStore(t, db)
	for name, op := range map[string]func() error{
		"start": func() error { _, err := faulty.StartMatch(ctx, time.Now()); return err },
		"stop":  func() error { _, err := faulty.StopMatch(ctx, time.Now()); return err },
	} {
		for _, position := range []int{1, 2} {
			mustExec(t, db, `DELETE FROM game_match_state`)
			if name == "stop" {
				mustExec(t, db, `INSERT INTO game_match_state(singleton, state) VALUES (TRUE, 'running')`)
			}
			faults.Arm(position)
			if err := op(); err == nil {
				t.Errorf("%s with fault at round trip %d must fail", name, position)
			}
			faults.Arm(0)
		}
	}
}

func faultyGameStore(t *testing.T, db *sql.DB) (gameStore, *dbtest.Faults) {
	t.Helper()
	faultyDB, faults := dbtest.FaultInjectingDB(t, "SUBMISSION_TEST_POSTGRES_DSN", db)
	return newPostgresGameStore(faultyDB), faults
}

type recordSubmissionFails struct{ gameStore }

func (recordSubmissionFails) RecordSubmission(context.Context, apigateway.AdminSubmission) error {
	return errScripted
}

type acceptRevalidationFails struct{ gameStore }

func (acceptRevalidationFails) AcceptFlagSubmission(context.Context, acceptedFlagSubmission) (bool, error) {
	return false, errFlagNoLongerValid
}

func TestSubmissionGateAndRevalidation(t *testing.T) {
	ctx := context.Background()
	notStarted := newTickServer(recordSubmissionFails{gameStore: newMemoryGameStore()})
	if _, err := notStarted.submitFlags(ctx, 101, []string{"x"}); err == nil || errors.Is(err, errContestNotStarted) {
		t.Fatalf("a failure to record rejected attempts must surface: %v", err)
	}
	server, store := tickedServer(t, testCheckerClient{})
	targets, _ := store.ListCheckerTargets(ctx)
	flag, _ := store.LatestIssuedFlag(ctx, targets[0].TeamID, targets[0].ChallengeID)
	revalidating := newTickServer(acceptRevalidationFails{gameStore: store})
	revalidating.flags = server.flags
	results, err := revalidating.submitFlags(ctx, targets[len(targets)-1].TeamID, []string{flag.Flag})
	if err != nil || results[0].Status != "invalid" {
		t.Fatalf("a capture invalidated at accept time is invalid: %+v %v", results, err)
	}
}

// Cancelling while a worker is busy leaves the producer racing a job send
// against the deadline; any job that lands after it must be skipped.
func TestCancelledWorkersSkipLateJobs(t *testing.T) {
	ctx := context.Background()
	warmup := newGameCoreServer("dev-admin-token", newMemoryGameStore(), blockingChecker{}, newFlagCodec("s", "PLAYIT"), &testGameScheduler{}, []string{"put"}, 15).
		WithCheckerParallelism(1).WithWarmupPutRetries(0).WithWarmupTimeout(5 * time.Millisecond)
	for range 20 {
		if result, _ := warmup.runWarmup(ctx); result.Status == "passed" {
			t.Fatal("a timed-out warmup never passes")
		}
	}
	server, _ := tickedServer(t, testCheckerClient{})
	server.checker = blockingChecker{}
	server.checkerParallelism = 1
	for range 20 {
		execCtx, cancel := context.WithTimeout(ctx, 5*time.Millisecond)
		if sweep, err := server.sweepRetainedFlags(ctx, execCtx, true); err != nil || sweep.Complete {
			t.Fatalf("a timed-out sweep is incomplete: %+v %v", sweep, err)
		}
		cancel()
	}
}

func TestMemoryAttackFeedFilters(t *testing.T) {
	ctx := context.Background()
	server, store := tickedServer(t, testCheckerClient{})
	if _, err := server.advanceTick(ctx); err != nil { // capture on tick 2
		t.Fatal(err)
	}
	targets, _ := store.ListCheckerTargets(ctx)
	flag, _ := store.LatestIssuedFlag(ctx, targets[0].TeamID, targets[0].ChallengeID)
	if results, err := server.submitFlags(ctx, targets[len(targets)-1].TeamID, []string{flag.Flag}); err != nil || results[0].Status != "accepted" {
		t.Fatalf("capture: %+v %v", results, err)
	}
	for _, query := range []apigateway.AttackFeedQuery{{Victim: "nobody"}, {Service: "nothing"}, {TickFrom: 99}, {TickTo: 1}, {Attacker: "nobody"}} {
		if page, _ := store.ListAttackFeed(ctx, query); page.TotalCount != 0 {
			t.Errorf("filter %+v leaked the capture", query)
		}
	}
	if page, _ := store.ListAttackFeed(ctx, apigateway.AttackFeedQuery{TickTo: 0, TickFrom: 1}); page.TotalCount != 1 {
		t.Fatalf("a matching range keeps the capture: %+v", page)
	}
}
