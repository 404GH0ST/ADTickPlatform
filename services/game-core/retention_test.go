package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"adplatform/internal/platform/httpapi"
	"adplatform/internal/services/apigateway"
)

type retentionTestChecker struct {
	mu          sync.Mutex
	value       string
	unavailable bool
	requests    []apigateway.CheckerExecutionRequest
	beforeGet   func()
}

func (c *retentionTestChecker) Execute(_ context.Context, r apigateway.CheckerExecutionRequest) (apigateway.CheckerExecutionResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.requests = append(c.requests, r)
	result := apigateway.CheckerExecutionResult{Status: "success", Phase: r.Phase, ExitCode: 0}
	switch r.Phase {
	case "put":
		c.value = r.Flag
		result.Output = `{"path":"flag.txt","private_slot":"opaque"}`
	case "get":
		if c.beforeGet != nil {
			c.beforeGet()
			c.beforeGet = nil
		}
		if c.unavailable {
			return result, errors.New("checker network unavailable")
		}
		if r.Metadata != `{"path":"flag.txt","private_slot":"opaque"}` {
			return apigateway.CheckerExecutionResult{Status: "failed", ExitCode: 1}, nil
		}
		if c.value != r.Flag {
			result.Status = "failed"
			result.ExitCode = flagMissingExitCode
		}
	}
	return result, nil
}
func (c *retentionTestChecker) ExecuteBatch(ctx context.Context, r apigateway.CheckerBatchExecutionRequest) (apigateway.CheckerBatchExecutionResult, error) {
	return executeBatchViaSingle(ctx, c.Execute, r)
}
func setupRetentionTest(t *testing.T) (*gameCoreServer, *memoryGameStore, *retentionTestChecker, *time.Time) {
	t.Helper()
	store := newMemoryGameStore().(*memoryGameStore)
	store.targets = store.targets[:1]
	checker := &retentionTestChecker{}
	now := time.Now().UTC()
	server := newGameCoreServer("test", store, checker, newFlagCodec("test-secret", "PLAYIT"), &testGameScheduler{}, []string{"put", "get", "check"}, 1)
	server.scoringDebounce = time.Hour
	server.now = func() time.Time { return now }
	if _, err := store.StartMatch(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		server.scoringMu.Lock()
		if server.scoringTimer != nil {
			server.scoringTimer.Stop()
		}
		server.scoringMu.Unlock()
	})
	return server, store, checker, &now
}
func retentionTeamScore(t *testing.T, store gameStore) apigateway.ScoreRowAlias {
	t.Helper()
	rows, err := store.RecomputeScoreboard(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.Team == "Team Alpha" {
			return row
		}
	}
	t.Fatal("missing team")
	return apigateway.ScoreRowAlias{}
}
func TestRetentionOverwriteDetectedDuringTickAndPenaltyIsSticky(t *testing.T) {
	s, store, checker, now := setupRetentionTest(t)
	ctx := context.Background()
	if _, err := s.advanceTick(ctx); err != nil {
		t.Fatal(err)
	}
	flag, err := store.LatestIssuedFlag(ctx, 101, 1)
	if err != nil {
		t.Fatal(err)
	}
	checker.value = "FAKE{poisoned}"
	*now = flag.NextCheckAt.Add(time.Second)
	if err = s.verifyRetainedFlags(ctx, false); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err = s.verifyRetainedFlags(ctx, true); err != nil {
			t.Fatal(err)
		}
	}
	row := retentionTeamScore(t, store)
	if row.Defense != -10 || row.SLA != 0 || row.Total != -10 {
		t.Fatalf("wrong retention penalty: %+v", row)
	}
	if len(store.retentionFailures) != 1 {
		t.Fatalf("penalty duplicated: %v", store.retentionFailures)
	}
	checker.value = flag.Flag
	if err = s.verifyRetainedFlags(ctx, true); err != nil {
		t.Fatal(err)
	}
	if row = retentionTeamScore(t, store); row.Defense != -10 || row.SLA != 0 {
		t.Fatalf("recovery erased penalty: %+v", row)
	}
	// Poisoning service storage never changes the authoritative accepted value.
	results, err := s.submitFlags(ctx, 102, []string{flag.Flag, "FAKE{poisoned}"})
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Status != "accepted" || results[1].Status != "invalid" {
		t.Fatalf("registry was changed: %+v", results)
	}
}
func TestRetentionChecksOldFlagBeforeRotationEvenWhenNotDue(t *testing.T) {
	s, store, checker, _ := setupRetentionTest(t)
	ctx := context.Background()
	if _, err := s.advanceTick(ctx); err != nil {
		t.Fatal(err)
	}
	first, _ := store.LatestIssuedFlag(ctx, 101, 1)
	checker.value = "replacement"
	before := len(checker.requests)
	if _, err := s.advanceTick(ctx); err != nil {
		t.Fatal(err)
	}
	requests := checker.requests[before:]
	if len(requests) != 4 || requests[0].Phase != "get" || requests[0].Flag != first.Flag || requests[1].Phase != "put" {
		t.Fatalf("old flag not verified before put: %+v", requests)
	}
	score := retentionTeamScore(t, store)
	if score.Defense != -10 || score.SLA != 2 {
		t.Fatalf("old tick not forfeited independently of new tick: %+v", score)
	}
}
func TestRetentionTransportFailureDoesNotReceiveSeverePenalty(t *testing.T) {
	s, store, checker, _ := setupRetentionTest(t)
	ctx := context.Background()
	if _, err := s.advanceTick(ctx); err != nil {
		t.Fatal(err)
	}
	checker.unavailable = true
	if err := s.verifyRetainedFlags(ctx, true); err != nil {
		t.Fatal(err)
	}
	if row := retentionTeamScore(t, store); row.Defense != 0 || row.SLA != 0 {
		t.Fatalf("transport failure treated as confirmed replacement: %+v", row)
	}
	checker.unavailable = false
	checker.value = "replaced"
	if err := s.verifyRetainedFlags(ctx, true); err != nil {
		t.Fatal(err)
	}
	if row := retentionTeamScore(t, store); row.Defense != -10 {
		t.Fatalf("later confirmed loss not penalized: %+v", row)
	}
}
func TestRetentionSkipsOrganizerResetOrMaintenanceRaces(t *testing.T) {
	for _, action := range []string{"reset", "maintenance"} {
		t.Run(action, func(t *testing.T) {
			s, store, checker, _ := setupRetentionTest(t)
			ctx := context.Background()
			if _, err := s.advanceTick(ctx); err != nil {
				t.Fatal(err)
			}
			checker.value = "cleared by organizer"
			checker.beforeGet = func() {
				store.mu.Lock()
				defer store.mu.Unlock()
				if action == "reset" {
					store.targets[0].CheckerToken = "rotated-token"
				} else {
					challenge := store.challenges[1]
					challenge.Maintenance = true
					store.challenges[1] = challenge
				}
			}
			if err := s.verifyRetainedFlags(ctx, true); err != nil {
				t.Fatal(err)
			}
			if row := retentionTeamScore(t, store); row.Defense != 0 || row.SLA != 2 {
				t.Fatalf("organizer action penalized: %+v", row)
			}
		})
	}
}
func TestStopMatchChecksFinalFlagRetention(t *testing.T) {
	s, store, checker, now := setupRetentionTest(t)
	ctx := context.Background()
	if _, err := s.advanceTick(ctx); err != nil {
		t.Fatal(err)
	}
	checker.value = "last tick replacement"
	status, err := s.stopMatchWithRetention(ctx, *now)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "finished" {
		t.Fatalf("not stopped: %+v", status)
	}
	if row := retentionTeamScore(t, store); row.Defense != -10 || row.SLA != 0 {
		t.Fatalf("final tick escaped verification: %+v", row)
	}
}

func TestRetentionPenaltyOutweighsSavedDefenseAndDeniedAttackPoints(t *testing.T) {
	for _, opponents := range []int{0, 1, 3, 7, 49, 99, 1000} {
		penalty := retentionPenalty(opponents)
		bound := 2*float64(opponents) + faustDefensePenalty(opponents)
		if penalty < 10 || penalty <= bound {
			t.Fatalf("penalty %.0f does not exceed avoided costs %.2f for %d opponents", penalty, bound, opponents)
		}
	}
	if retentionPenalty(7) != 20 || retentionPenalty(49) != 118 {
		t.Fatal("unexpected penalty scale")
	}
}
func TestRetentionScaledPenaltyIsFrozenAtFirstConfirmation(t *testing.T) {
	s, store, checker, _ := setupRetentionTest(t)
	ctx := context.Background()
	// Eight eligible teams means seven potential opposing capturers.
	for id := 105; id <= 108; id++ {
		store.teamNames[id] = "Additional team"
	}
	if _, err := s.advanceTick(ctx); err != nil {
		t.Fatal(err)
	}
	checker.value = "replaced"
	if err := s.verifyRetainedFlags(ctx, true); err != nil {
		t.Fatal(err)
	}
	if row := retentionTeamScore(t, store); row.Defense != -20 {
		t.Fatalf("flat penalty remains: %+v", row)
	}
	for id := 109; id <= 130; id++ {
		store.teamNames[id] = "Later joiner"
	}
	if err := s.verifyRetainedFlags(ctx, true); err != nil {
		t.Fatal(err)
	}
	if row := retentionTeamScore(t, store); row.Defense != -20 {
		t.Fatalf("existing penalty changed with later joiners: %+v", row)
	}
}

func TestImmediateFlagReplacementIsPenalizedWithoutDuplicateHistory(t *testing.T) {
	s, store, checker, _ := setupRetentionTest(t)
	ctx := context.Background()
	checker.beforeGet = func() { checker.value = "changed immediately after put" }
	tick, err := s.advanceTick(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if tick.TotalCheckerRuns != 3 || tick.FailedCheckerRuns != 1 || tick.SkippedCheckerRuns != 1 {
		t.Fatalf("incorrect phase accounting: %+v", tick)
	}
	if len(store.runs) != 3 {
		t.Fatalf("get recorded twice: %d", len(store.runs))
	}
	if row := retentionTeamScore(t, store); row.Defense != -10 || row.SLA != 0 {
		t.Fatalf("immediate replacement not penalized: %+v", row)
	}
}

// gatedTickChecker holds tick 2's put until released and honours cancellation.
type gatedTickChecker struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (c *gatedTickChecker) Execute(ctx context.Context, r apigateway.CheckerExecutionRequest) (apigateway.CheckerExecutionResult, error) {
	if r.TickID == 2 && r.Phase == "put" {
		c.once.Do(func() { close(c.entered) })
		<-c.release
	}
	if err := ctx.Err(); err != nil {
		return apigateway.CheckerExecutionResult{}, err
	}
	return testCheckerClient{}.Execute(ctx, r)
}
func (c *gatedTickChecker) ExecuteBatch(ctx context.Context, r apigateway.CheckerBatchExecutionRequest) (apigateway.CheckerBatchExecutionResult, error) {
	return executeBatchViaSingle(ctx, c.Execute, r)
}

// An organizer stop waits for the running tick and then verifies retained
// flags. If the HTTP caller gives up meanwhile, the stop must still finish.
func TestStopMatchCompletesAfterCallerDisconnects(t *testing.T) {
	store := newMemoryGameStore().(*memoryGameStore)
	store.targets = store.targets[:1]
	checker := &gatedTickChecker{entered: make(chan struct{}), release: make(chan struct{})}
	srv := newGameCoreServer("test", store, checker, newFlagCodec("test-secret", "PLAYIT"), &testGameScheduler{}, []string{"put", "get", "check"}, 1).
		WithWarmupRequired(false).WithAutoTickOnMatchStart(false)
	srv.scoringDebounce = time.Hour
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "game-core", Version: "dev", Addr: ":0"})
	srv.RegisterRoutes(mux)
	bg := context.Background()
	if _, err := store.StartMatch(bg, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.advanceTick(bg); err != nil {
		t.Fatal(err)
	}
	tickDone := make(chan struct{})
	go func() { _, _ = srv.advanceTick(bg); close(tickDone) }()
	<-checker.entered

	callerCtx, cancel := context.WithTimeout(bg, 50*time.Millisecond)
	defer cancel()
	request := httptest.NewRequest(http.MethodPost, "/internal/v1/game/match/stop", nil).WithContext(callerCtx)
	request.Header.Set("Authorization", "Bearer test")
	response := httptest.NewRecorder()
	stopDone := make(chan struct{})
	go func() { mux.ServeHTTP(response, request); close(stopDone) }()

	<-callerCtx.Done() // the gateway has given up while the tick is still running
	close(checker.release)
	<-tickDone
	<-stopDone

	match, err := store.MatchStatus(bg)
	if err != nil {
		t.Fatal(err)
	}
	if match.State != "finished" {
		t.Fatalf("stop aborted after caller disconnect; match state %q, response %d %s", match.State, response.Code, response.Body.String())
	}
}

// The forced final sweep can outgrow any fixed budget on a large or degraded
// event. When it runs out, the match must still stop rather than fail (and fail
// again on every retry, since force restarts the whole sweep).
func TestStopMatchFinishesWhenFinalRetentionSweepRunsOutOfBudget(t *testing.T) {
	store := newMemoryGameStore().(*memoryGameStore)
	checker := &hangAfterFirstTickChecker{}
	srv := newGameCoreServer("test", store, checker, newFlagCodec("test-secret", "PLAYIT"), &testGameScheduler{}, []string{"put", "get", "check"}, 15).
		WithWarmupRequired(false).WithAutoTickOnMatchStart(false).WithCheckerParallelism(2)
	srv.scoringDebounce = time.Hour
	srv.finalRetentionSweepBudget = 100 * time.Millisecond
	ctx := context.Background()
	if _, err := store.StartMatch(ctx, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.advanceTick(ctx); err != nil {
		t.Fatal(err)
	}
	checker.mu.Lock()
	checker.hang = true // every retention get now outlasts the sweep budget
	checker.mu.Unlock()

	status, err := srv.stopMatchWithRetention(ctx, time.Now().UTC())
	if err != nil {
		t.Fatalf("stop failed when the final sweep ran out of budget: %v", err)
	}
	if status.State != "finished" {
		t.Fatalf("match not stopped: %+v", status)
	}
	store.mu.Lock()
	losses := len(store.retentionFailures)
	store.mu.Unlock()
	if losses != 0 {
		t.Fatalf("interrupted final checks were recorded as %d retention losses", losses)
	}
	events, err := store.ListSchedulerEvents(ctx, apigateway.GameSchedulerEventQuery{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events.Items {
		if event.EventType == "final_retention_incomplete" {
			return
		}
	}
	t.Fatalf("incomplete final sweep was not reported: %+v", events.Items)
}
