package apigateway

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"adplatform/internal/platform/httpapi"
)

// actionController fails chosen runtime actions; by default each succeeds.
type actionController struct {
	storeBackedControllerClient
	resetErr   error
	restartErr error
	accessErr  map[int]error
	sshErr     map[int]error
}

func (c actionController) FactoryResetService(context.Context, int, int) error { return c.resetErr }
func (c actionController) RestartService(context.Context, int, int) error      { return c.restartErr }
func (c actionController) ReconcileServiceAccess(_ context.Context, _, challengeID int) error {
	return c.accessErr[challengeID]
}
func (c actionController) ApplySSHCredential(_ context.Context, _, challengeID int, _ ControllerSSHCredential) error {
	return c.sshErr[challengeID]
}

// actionStore fails chosen lifecycle writes on top of a real memory store.
type actionStore struct {
	Store
	abortErr     error
	markErr      error
	completeErr  error
	unlockErr    map[int]error
	mu           *sync.Mutex
	markCalls    *int
	markFailures int // MarkFactoryResetCompletionFailure fails this many times
}

func (s actionStore) AbortFactoryResetService(ctx context.Context, prepared resetData) error {
	if s.abortErr != nil {
		return s.abortErr
	}
	return s.Store.AbortFactoryResetService(ctx, prepared)
}

func (s actionStore) MarkFactoryResetFailure(ctx context.Context, teamID, challengeID int) error {
	if s.markErr != nil {
		return s.markErr
	}
	return s.Store.MarkFactoryResetFailure(ctx, teamID, challengeID)
}

func (s actionStore) CompleteFactoryResetService(ctx context.Context, teamID, challengeID int) (resetData, error) {
	if s.completeErr != nil {
		return resetData{}, s.completeErr
	}
	return s.Store.CompleteFactoryResetService(ctx, teamID, challengeID)
}

func (s actionStore) MarkFactoryResetCompletionFailure(ctx context.Context, teamID, challengeID int) error {
	if s.markCalls != nil {
		s.mu.Lock()
		*s.markCalls++
		failing := *s.markCalls <= s.markFailures
		s.mu.Unlock()
		if failing {
			return errStoreDown
		}
	}
	return s.Store.MarkFactoryResetCompletionFailure(ctx, teamID, challengeID)
}

func (s actionStore) UnlockService(ctx context.Context, teamID, challengeID int) (unlockData, error) {
	if err := s.unlockErr[challengeID]; err != nil {
		return unlockData{}, err
	}
	return s.Store.UnlockService(ctx, teamID, challengeID)
}

type actionCase struct {
	name       string
	controller func(actionController) actionController
	store      func(actionStore) actionStore
	limiter    func(teamID int) rateLimiter
	submission submissionClient
	method     string
	path       string
	body       any
	want       int
}

// runActionCase serves one request from a fresh participant whose services
// are deployed and unlocked, against the configured doubles.
func runActionCase(t *testing.T, tc actionCase) (*sweepEnv, actionStore) {
	t.Helper()
	memory := NewMemoryStore(101)
	base := actionStore{Store: memory, mu: &sync.Mutex{}}
	if tc.store != nil {
		base = tc.store(base)
	}
	controller := actionController{storeBackedControllerClient: storeBackedControllerClient{store: memory}}
	if tc.controller != nil {
		controller = tc.controller(controller)
	}
	limiter := &switchLimiter{inner: testRateLimiter{}}
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "api-gateway", Version: "test", Addr: ":0"})
	server := NewWithDeps("dev-team-token", "dev-admin-token", 101, base, controller, noopWireGuardClient{}).
		WithSSHCredentialSecret("action-ssh-secret").
		WithRateLimiter(limiter)
	if tc.submission != nil {
		server.WithSubmissionClient(tc.submission)
	}
	server.RegisterRoutes(mux)
	env := &sweepEnv{t: t, mux: mux, faults: &storeFaults{}}
	token, _, teamID := env.register(0, "action")
	if _, err := memory.ReconcileAdminDeployments(t.Context(), time.Now()); err != nil {
		t.Fatal(err)
	}
	for id := 1; id <= 3; id++ {
		if _, err := memory.UnlockService(t.Context(), teamID, id); err != nil {
			t.Fatal(err)
		}
	}
	if tc.limiter != nil {
		limiter.set(tc.limiter(teamID))
	}
	if response := env.do(tc.method, tc.path, token, tc.body); response.Code != tc.want {
		t.Errorf("%s: want %d, got %d %s", tc.name, tc.want, response.Code, response.Body.String())
	}
	return env, base
}

// switchLimiter lets a case install its limiter after setup has registered.
type switchLimiter struct {
	mu    sync.Mutex
	inner rateLimiter
}

func (l *switchLimiter) set(inner rateLimiter) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.inner = inner
}

func (l *switchLimiter) Allow(ctx context.Context, key string, policy rateLimitPolicy) (rateLimitDecision, error) {
	l.mu.Lock()
	inner := l.inner
	l.mu.Unlock()
	return inner.Allow(ctx, key, policy)
}

func denyKey(format string) func(int) rateLimiter {
	return func(teamID int) rateLimiter {
		return testRateLimiter{denyKeys: map[string]bool{fmt.Sprintf(format, teamID): true}}
	}
}

func TestServiceActionRuntimeFailures(t *testing.T) {
	reset := "/api/v2/services/3/reset/factory"
	notFound := fmt.Errorf("%w: %w", errResetPreflight, ErrChallengeNotFound)
	cases := []actionCase{
		{name: "ssh bad id", method: http.MethodPost, path: "/api/v2/services/abc/ssh-session", want: http.StatusBadRequest},
		{name: "reset bad id", method: http.MethodPost, path: "/api/v2/services/abc/reset/factory", want: http.StatusBadRequest},
		{name: "restart bad id", method: http.MethodPost, path: "/api/v2/services/abc/reset/restart", want: http.StatusBadRequest},
		{name: "ssh rate limited", method: http.MethodPost, path: "/api/v2/services/1/ssh-session", limiter: func(teamID int) rateLimiter {
			return testRateLimiter{denyKeys: map[string]bool{rateLimitTeamChallengeKey("ssh-session", teamID, 1): true}}
		}, want: http.StatusTooManyRequests},
		{name: "reset cooldown", method: http.MethodPost, path: reset, limiter: func(teamID int) rateLimiter {
			return testRateLimiter{denyKeys: map[string]bool{rateLimitTeamChallengeKey("factory-reset", teamID, 3): true}}
		}, want: http.StatusTooManyRequests},
		{name: "restart cooldown", method: http.MethodPost, path: "/api/v2/services/2/reset/restart", limiter: func(teamID int) rateLimiter {
			return testRateLimiter{denyKeys: map[string]bool{rateLimitTeamChallengeKey("restart", teamID, 2): true}}
		}, want: http.StatusTooManyRequests},
		{name: "ssh access reconcile fails", method: http.MethodPost, path: "/api/v2/services/1/ssh-session",
			controller: func(c actionController) actionController { c.accessErr = map[int]error{1: errUpstreamDown}; return c },
			want:       http.StatusBadGateway},
		{name: "ssh credential apply fails", method: http.MethodPost, path: "/api/v2/services/1/ssh-session",
			controller: func(c actionController) actionController { c.sshErr = map[int]error{1: errUpstreamDown}; return c },
			want:       http.StatusBadGateway},
		{name: "reset preflight rejects missing challenge", method: http.MethodPost, path: reset,
			controller: func(c actionController) actionController { c.resetErr = notFound; return c },
			want:       http.StatusBadRequest},
		{name: "reset preflight rollback fails", method: http.MethodPost, path: reset,
			controller: func(c actionController) actionController { c.resetErr = errResetPreflight; return c },
			store:      func(s actionStore) actionStore { s.abortErr = errStoreDown; return s },
			want:       http.StatusInternalServerError},
		{name: "reset fails before volume removal", method: http.MethodPost, path: reset,
			controller: func(c actionController) actionController { c.resetErr = errResetBeforeVolume; return c },
			want:       http.StatusBadGateway},
		{name: "reset runtime fails and marking fails", method: http.MethodPost, path: reset,
			controller: func(c actionController) actionController { c.resetErr = errUpstreamDown; return c },
			store:      func(s actionStore) actionStore { s.markErr = errStoreDown; return s },
			want:       http.StatusBadGateway},
		{name: "restart runtime rejects missing challenge", method: http.MethodPost, path: "/api/v2/services/2/reset/restart",
			controller: func(c actionController) actionController { c.restartErr = ErrChallengeNotFound; return c },
			want:       http.StatusBadRequest},
		{name: "restart runtime fails", method: http.MethodPost, path: "/api/v2/services/2/reset/restart",
			controller: func(c actionController) actionController { c.restartErr = errUpstreamDown; return c },
			want:       http.StatusBadGateway},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { runActionCase(t, tc) })
	}
}

func TestFactoryResetCompletionFailureIsRecoveredInBackground(t *testing.T) {
	calls := 0
	_, store := runActionCase(t, actionCase{
		name: "completion fails", method: http.MethodPost, path: "/api/v2/services/3/reset/factory",
		store: func(s actionStore) actionStore {
			s.completeErr, s.markCalls, s.markFailures = errStoreDown, &calls, 1
			return s
		},
		want: http.StatusInternalServerError,
	})
	deadline := time.Now().Add(5 * time.Second)
	for {
		store.mu.Lock()
		done := calls >= 2
		store.mu.Unlock()
		if done {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the background recovery must retry the completion-failure mark")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestCompletionRecoveryStopsWithContext(t *testing.T) {
	calls := 0
	store := actionStore{Store: NewMemoryStore(101), mu: &sync.Mutex{}, markCalls: &calls, markFailures: 1 << 30}
	server := &Server{store: store}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	server.retryFactoryResetCompletionFailure(ctx, 101, 1, 5*time.Millisecond)
	if calls < 2 {
		t.Fatalf("recovery must retry until its context ends, made %d attempts", calls)
	}
	if sleepContext(ctx, time.Hour) {
		t.Fatal("an ended context cuts the sleep short")
	}
}

func TestAcceptedFlagAccessFailuresKeepVerdicts(t *testing.T) {
	accepted := []submissionVerdict{
		{Flag: "a", Status: "accepted", ChallengeID: 1},
		{Flag: "b", Status: "accepted", ChallengeID: 2},
		{Flag: "c", Status: "accepted", ChallengeID: 3},
		{Flag: "d", Status: "accepted", ChallengeID: 3},
		{Flag: "e", Status: "invalid"},
	}
	runActionCase(t, actionCase{
		name: "unlock, reconcile and apply each fail", method: http.MethodPost, path: "/api/v2/submit",
		body:       submitRequest{Flags: []string{"a", "b", "c", "d", "e"}},
		submission: testSubmissionClient{submit: accepted},
		store: func(s actionStore) actionStore {
			s.unlockErr = map[int]error{1: errStoreDown}
			return s
		},
		controller: func(c actionController) actionController {
			c.accessErr, c.sshErr = map[int]error{2: errUpstreamDown}, map[int]error{3: errUpstreamDown}
			return c
		},
		want: http.StatusOK,
	})
}

func TestSubmitSecondaryRateLimits(t *testing.T) {
	runActionCase(t, actionCase{
		name: "per-player submit limit", method: http.MethodPost, path: "/api/v2/submit",
		body: submitRequest{Flags: []string{"x"}},
		limiter: func(int) rateLimiter {
			return prefixLimiter{prefix: "submit:user:"}
		},
		want: http.StatusTooManyRequests,
	})
}

// prefixLimiter denies every key with the prefix.
type prefixLimiter struct{ prefix string }

func (l prefixLimiter) Allow(_ context.Context, key string, _ rateLimitPolicy) (rateLimitDecision, error) {
	if len(key) >= len(l.prefix) && key[:len(l.prefix)] == l.prefix {
		return rateLimitDecision{allowed: false, retryAfter: time.Second}, nil
	}
	return rateLimitDecision{allowed: true}, nil
}

func TestEnrichServiceStatesWithSLADetailsPaging(t *testing.T) {
	server := &Server{}
	if got := server.enrichServiceStatesWithSLADetails(context.Background(), 101, nil); len(got) != 0 {
		t.Fatal("no states stay no states")
	}
	// Newest first: 30 runs of challenge 2 on tick 9 span the first page, a
	// run for an unlisted challenge is skipped, and the older tick closes the
	// latest tick's summary for both challenges.
	runs := []GameCheckerRun{}
	for i := 0; i < 30; i++ {
		runs = append(runs, GameCheckerRun{ID: int64(100 - i), TickID: 9, TeamID: 101, ChallengeID: 2, Phase: "check", Status: "success"})
	}
	runs = append(runs,
		GameCheckerRun{ID: 60, TickID: 9, TeamID: 101, ChallengeID: 7, Phase: "check", Status: "success"},
		GameCheckerRun{ID: 59, TickID: 9, TeamID: 101, ChallengeID: 1, Phase: "put", Status: "success"},
		GameCheckerRun{ID: 58, TickID: 8, TeamID: 101, ChallengeID: 1, Phase: "put", Status: "failed"},
		GameCheckerRun{ID: 57, TickID: 8, TeamID: 101, ChallengeID: 2, Phase: "put", Status: "failed"},
	)
	server.gameCore = testGameCoreClient{runs: runs}
	states := server.enrichServiceStatesWithSLADetails(context.Background(), 101, []serviceState{{ChallengeID: 1}, {ChallengeID: 2}})
	for _, state := range states {
		// Tick 9 is partial for both (no get for 1, no put for 2), so the
		// summary names the missing phase rather than tick 8's failures.
		want := map[int]string{1: "get", 2: "put"}[state.ChallengeID]
		if state.SLAPhase != want {
			t.Errorf("challenge %d must be summarized from tick 9 (phase %q), got %+v", state.ChallengeID, want, state)
		}
	}
}

func TestWriteDomainFailureMapsEverySentinel(t *testing.T) {
	cases := map[error]int{
		ErrServiceLocked:          http.StatusBadRequest,
		ErrServiceUnavailable:     http.StatusBadRequest,
		ErrChallengeMaintenance:   http.StatusServiceUnavailable,
		ErrChallengeDeferred:      http.StatusServiceUnavailable,
		ErrMatchNotStarted:        http.StatusServiceUnavailable,
		ErrMatchPaused:            http.StatusServiceUnavailable,
		ErrChallengeNotFound:      http.StatusBadRequest,
		ErrTeamNotFound:           http.StatusBadRequest,
		ErrPlayerNotFound:         http.StatusBadRequest,
		ErrDeploymentNotFound:     http.StatusBadRequest,
		ErrDeploymentActive:       http.StatusBadRequest,
		ErrDuplicateResource:      http.StatusConflict,
		ErrTeamMemberLimit:        http.StatusBadRequest,
		ErrNotTeamCaptain:         http.StatusForbidden,
		ErrInvalidCaptainTransfer: http.StatusBadRequest,
		ErrTeamAddressPool:        http.StatusConflict,
		ErrPlayerAddressPool:      http.StatusConflict,
		ErrInvalidRuntimeConfig:   http.StatusBadRequest,
		errStoreDown:              http.StatusInternalServerError,
	}
	for err, want := range cases {
		recorder := httptest.NewRecorder()
		writeDomainFailure(recorder, fmt.Errorf("wrapped: %w", err))
		if recorder.Code != want {
			t.Errorf("%v: want %d, got %d", err, want, recorder.Code)
		}
	}
}
