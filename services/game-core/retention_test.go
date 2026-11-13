package main

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

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
