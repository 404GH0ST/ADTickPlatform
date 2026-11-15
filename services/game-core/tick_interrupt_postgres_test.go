package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"adplatform/internal/platform/database"
	"adplatform/internal/platform/httpapi"
	"adplatform/internal/services/apigateway"
)

// blockingPutChecker blocks every put until its ctx is cancelled, like a real
// checker container killed by context cancellation.
type blockingPutChecker struct {
	entered chan struct{}
	once    sync.Once
}

func (c *blockingPutChecker) Execute(ctx context.Context, r apigateway.CheckerExecutionRequest) (apigateway.CheckerExecutionResult, error) {
	if r.Phase == "put" && r.TickID == 1 {
		c.once.Do(func() { close(c.entered) })
		<-ctx.Done()
		return apigateway.CheckerExecutionResult{}, ctx.Err()
	}
	return testCheckerClient{}.Execute(ctx, r)
}

func (c *blockingPutChecker) ExecuteBatch(ctx context.Context, r apigateway.CheckerBatchExecutionRequest) (apigateway.CheckerBatchExecutionResult, error) {
	return executeBatchViaSingle(ctx, c.Execute, r)
}

// Pausing the match cancels the scheduler's context while a tick is running.
// The tick must still be finalized: a row left 'running' made StartNextTick
// refuse every later tick until game-core restarted.
func TestPostgresPauseDuringTickDoesNotWedgeTicks(t *testing.T) {
	db := isolatedSubmissionDB(t)
	ctx := context.Background()
	if err := database.RunEmbeddedMigrationsWithOptions(ctx, db, database.MigrationOptions{IncludeSeeds: false}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO teams(id,name,email,join_key) VALUES(101,'Team Alpha','a@test.local','a');
		INSERT INTO challenges(id,name,published,checker_image) VALUES(1,'svc',TRUE,'checker');
		INSERT INTO service_instances(team_id,challenge_id,runtime_kind,runtime_status,container_name,baseline_image,checker_image,endpoint,ssh_host,checker_token)
		VALUES(101,1,'docker','ready','svc-101','test','checker','10.80.1.11:8080','10.80.1.11','tok');
		INSERT INTO team_service_states(team_id,challenge_id,endpoint,status,checker,unlocked,ssh_hint,last_event,reset_cooldown)
		VALUES(101,1,'10.80.1.11:8080','healthy','ok',FALSE,'','','ready');
		INSERT INTO game_match_state(singleton,state) VALUES(TRUE,'running')
		ON CONFLICT (singleton) DO UPDATE SET state='running';
	`); err != nil {
		t.Fatal(err)
	}
	store := &postgresGameStore{db: db}
	checker := &blockingPutChecker{entered: make(chan struct{})}
	srv := newGameCoreServer("test", store, checker, newFlagCodec("test-secret", "PLAYIT"), nil, []string{"put", "get", "check"}, 1).
		WithWarmupRequired(false).WithAutoTickOnMatchStart(false)
	srv.scoringDebounce = time.Hour
	scheduler := newIntervalGameScheduler(store, 20*time.Millisecond, false, srv.advanceTick, srv.matchStatus)
	srv.scheduler = scheduler
	defer scheduler.Close()
	if _, err := scheduler.Start(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-checker.entered:
	case <-time.After(10 * time.Second):
		t.Fatal("scheduled tick never reached the checker")
	}

	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "game-core", Version: "dev", Addr: ":0"})
	srv.RegisterRoutes(mux)
	post := func(path string) {
		t.Helper()
		request := httptest.NewRequest(http.MethodPost, path, nil)
		request.Header.Set("Authorization", "Bearer test")
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", path, response.Code, response.Body.String())
		}
	}
	post("/internal/v1/game/match/pause")

	// The tick holds tickMu until it returns; wait for it to unwind.
	unwound := false
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if srv.tickMu.TryLock() {
			srv.tickMu.Unlock()
			unwound = true
			break
		}
	}
	if !unwound {
		t.Fatal("interrupted tick did not return")
	}

	var status, message string
	if err := db.QueryRowContext(ctx, `SELECT status, message FROM game_ticks WHERE id=1`).Scan(&status, &message); err != nil {
		t.Fatal(err)
	}
	if status == "running" {
		t.Fatalf("interrupted tick left in 'running' state")
	}
	t.Logf("interrupted tick: status=%s message=%q", status, message)

	var putStatus string
	if err := db.QueryRowContext(ctx, `SELECT status FROM checker_runs WHERE tick_id=1 AND phase='put'`).Scan(&putStatus); err != nil {
		t.Fatal(err)
	}
	if putStatus != "skipped" {
		t.Fatalf("interrupted put recorded as %q; a pause must not be charged as a checker failure", putStatus)
	}
	var serviceState string
	if err := db.QueryRowContext(ctx, `SELECT service_state FROM checker_service_states WHERE tick_id=1 AND team_id=101 AND challenge_id=1`).Scan(&serviceState); err != nil {
		t.Fatal(err)
	}
	if serviceState != "unknown" {
		t.Fatalf("interrupted target service state = %q; a pause must not mark the service down", serviceState)
	}

	post("/internal/v1/game/match/resume")
	next, err := srv.advanceTick(ctx)
	if err != nil {
		t.Fatalf("tick after pause/resume: %v", err)
	}
	if next.ID != 2 || next.Status != "completed" {
		t.Fatalf("unexpected next tick: %+v", next)
	}
}
