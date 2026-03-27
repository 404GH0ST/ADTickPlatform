package apigateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"adplatform/internal/platform/httpapi"
	"adplatform/internal/platform/unlockproof"
)

type testControllerClient struct {
	deployReconcileErr    error
	deployReconcileResult adminReconcileResult
	sshApplyErr           error
	validationErr         error
	validationResult      ChallengeValidationResult
	accessStatus          ControllerAccessStatus
	accessErr             error
}

type testWireGuardClient struct {
	status WireGuardGatewayStatus
	err    error
}

type testGameCoreClient struct {
	status    GameStatus
	tick      GameTickStatus
	runs      []GameCheckerRun
	match     GameMatchStatus
	scheduler GameSchedulerStatus
	events    []GameSchedulerEvent
	submit    []SubmissionVerdictAlias
	scores    []ScoreRowAlias
	attacks   []AttackEventAlias
	err       error
}

type testSubmissionClient struct {
	submit  []SubmissionVerdictAlias
	attacks []AttackEventAlias
	err     error
}

type testScoringClient struct {
	scores []ScoreRowAlias
	err    error
}

type testRateLimiter struct {
	denyKeys   map[string]bool
	retryAfter time.Duration
}

func (l testRateLimiter) Allow(_ context.Context, key string, _ rateLimitPolicy) (rateLimitDecision, error) {
	if l.denyKeys != nil && l.denyKeys[key] {
		retryAfter := l.retryAfter
		if retryAfter <= 0 {
			retryAfter = 2 * time.Second
		}
		return rateLimitDecision{
			allowed:    false,
			retryAfter: retryAfter,
		}, nil
	}
	return rateLimitDecision{allowed: true}, nil
}

func (c testControllerClient) ReconcileDeployments(_ context.Context) (adminReconcileResult, error) {
	if c.deployReconcileErr != nil {
		return adminReconcileResult{}, c.deployReconcileErr
	}
	if c.deployReconcileResult == (adminReconcileResult{}) {
		return adminReconcileResult{}, errControllerDisabled
	}
	return c.deployReconcileResult, nil
}

func (c testControllerClient) FactoryResetService(_ context.Context, _, _ int) error {
	return nil
}

func (c testControllerClient) RestartService(_ context.Context, _, _ int) error {
	return nil
}

func (c testControllerClient) ReconcileServiceAccess(_ context.Context, _, _ int) error {
	return nil
}

func (c testControllerClient) ApplySSHCredential(_ context.Context, _, _ int, _ ControllerSSHCredential) error {
	return c.sshApplyErr
}

func (c testControllerClient) ValidateChallengeRuntime(_ context.Context, request ChallengeValidationRequest) (ChallengeValidationResult, error) {
	if c.validationErr != nil {
		return ChallengeValidationResult{}, c.validationErr
	}
	if c.validationResult.Status == "" {
		return ChallengeValidationResult{
			ChallengeID:           request.ChallengeID,
			Name:                  request.Name,
			BaselineImage:         request.BaselineImage,
			CheckerImage:          request.CheckerImage,
			Status:                "valid",
			BaselineSSHContractOK: true,
			CheckerContractOK:     true,
			CheckedAt:             "2026-03-10T10:00:00Z",
			Message:               "challenge package validated by test controller",
		}, nil
	}
	result := c.validationResult
	if result.ChallengeID == 0 {
		result.ChallengeID = request.ChallengeID
	}
	if result.Name == "" {
		result.Name = request.Name
	}
	if result.BaselineImage == "" {
		result.BaselineImage = request.BaselineImage
	}
	if result.CheckerImage == "" {
		result.CheckerImage = request.CheckerImage
	}
	if result.CheckedAt == "" {
		result.CheckedAt = "2026-03-10T10:00:00Z"
	}
	return result, nil
}

func (c testControllerClient) AccessStatus(_ context.Context) (ControllerAccessStatus, error) {
	if c.accessErr != nil {
		return ControllerAccessStatus{}, c.accessErr
	}
	if c.accessStatus != (ControllerAccessStatus{}) {
		return c.accessStatus, nil
	}
	return ControllerAccessStatus{State: "disabled", Mode: "disabled"}, nil
}

func (c testControllerClient) ReconcileAccessPolicies(_ context.Context) (ControllerAccessStatus, error) {
	if c.accessErr != nil {
		return ControllerAccessStatus{}, c.accessErr
	}
	if c.accessStatus != (ControllerAccessStatus{}) {
		return c.accessStatus, nil
	}
	return ControllerAccessStatus{State: "disabled", Mode: "disabled"}, nil
}

func (c testControllerClient) TeardownAccessPolicies(_ context.Context) error {
	return nil
}

func (c testControllerClient) RemoveService(_ context.Context, _, _ int) error {
	return nil
}

func (c testControllerClient) RemoveTeamServices(_ context.Context, _ int) error {
	return nil
}

func (c testControllerClient) RemoveChallengeServices(_ context.Context, _ int) error {
	return nil
}

func (c testWireGuardClient) Status(_ context.Context) (WireGuardGatewayStatus, error) {
	if c.err != nil {
		return WireGuardGatewayStatus{}, c.err
	}
	if c.status != (WireGuardGatewayStatus{}) {
		return c.status, nil
	}
	return WireGuardGatewayStatus{State: "disabled", Mode: "disabled"}, nil
}

func (c testWireGuardClient) Reconcile(_ context.Context) (WireGuardGatewayStatus, error) {
	return c.Status(context.Background())
}

func (c testWireGuardClient) Teardown(_ context.Context) error {
	return nil
}

func (c testGameCoreClient) Status(_ context.Context) (GameStatus, error) {
	if c.err != nil {
		return GameStatus{}, c.err
	}
	if c.status.TotalTicks == 0 && c.status.CurrentTick == nil && c.status.Match == nil && c.match.State == "" {
		return GameStatus{
			Match: &GameMatchStatus{
				State:                "running",
				StartedAt:            "2026-03-10T10:00:00Z",
				AcceptingSubmissions: true,
			},
			CurrentTick: &GameTickStatus{
				ID:                    1,
				Status:                "completed",
				TotalCheckerRuns:      36,
				SuccessfulCheckerRuns: 36,
				FailedCheckerRuns:     0,
				SkippedCheckerRuns:    0,
				StartedAt:             "2026-03-10T10:00:00Z",
				CompletedAt:           "2026-03-10T10:00:10Z",
				Message:               "manual tick advanced in test",
			},
			Scheduler: &GameSchedulerStatus{
				State:           "stopped",
				IntervalSeconds: 60,
			},
			TotalTicks:            1,
			TotalCheckerRuns:      36,
			SuccessfulCheckerRuns: 36,
			FailedCheckerRuns:     0,
			SkippedCheckerRuns:    0,
		}, nil
	}
	if c.status.Match == nil {
		status := c.status
		if c.match.State != "" {
			match := c.match
			status.Match = &match
		}
		return status, nil
	}
	return c.status, nil
}

func (c testGameCoreClient) MatchStatus(_ context.Context) (GameMatchStatus, error) {
	if c.err != nil {
		return GameMatchStatus{}, c.err
	}
	if c.match.State == "" {
		return GameMatchStatus{
			State:                "running",
			StartedAt:            "2026-03-10T10:00:00Z",
			AcceptingSubmissions: true,
		}, nil
	}
	return c.match, nil
}

func (c testGameCoreClient) StartMatch(_ context.Context) (GameMatchStatus, error) {
	if c.err != nil {
		return GameMatchStatus{}, c.err
	}
	if c.match.State == "" {
		return GameMatchStatus{
			State:                "running",
			StartedAt:            "2026-03-10T10:00:00Z",
			AcceptingSubmissions: true,
		}, nil
	}
	match := c.match
	match.State = "running"
	match.AcceptingSubmissions = true
	if match.StartedAt == "" {
		match.StartedAt = "2026-03-10T10:00:00Z"
	}
	match.EndedAt = ""
	return match, nil
}

func (c testGameCoreClient) StopMatch(_ context.Context) (GameMatchStatus, error) {
	if c.err != nil {
		return GameMatchStatus{}, c.err
	}
	match := c.match
	if match.State == "" {
		match = GameMatchStatus{State: "finished", StartedAt: "2026-03-10T10:00:00Z", EndedAt: "2026-03-10T12:00:00Z"}
	}
	match.State = "finished"
	match.AcceptingSubmissions = false
	if match.EndedAt == "" {
		match.EndedAt = "2026-03-10T12:00:00Z"
	}
	return match, nil
}

func (c testGameCoreClient) UpdateMatchSchedule(_ context.Context, req UpdateMatchScheduleRequest) (GameMatchStatus, error) {
	if c.err != nil {
		return GameMatchStatus{}, c.err
	}
	match, err := c.MatchStatus(context.Background())
	if err != nil {
		return GameMatchStatus{}, err
	}
	match.ScheduledStartAt = req.ScheduledStartAt
	match.ScheduledEndAt = req.ScheduledEndAt
	return match, nil
}

func (c testGameCoreClient) AdvanceTick(_ context.Context) (GameTickStatus, error) {
	if c.err != nil {
		return GameTickStatus{}, c.err
	}
	if c.tick.ID == 0 {
		return GameTickStatus{
			ID:                    2,
			Status:                "completed",
			TotalCheckerRuns:      36,
			SuccessfulCheckerRuns: 30,
			FailedCheckerRuns:     3,
			SkippedCheckerRuns:    3,
			StartedAt:             "2026-03-10T10:01:00Z",
			CompletedAt:           "2026-03-10T10:01:12Z",
			Message:               "advanced tick across 12 targets and 3 phases",
		}, nil
	}
	return c.tick, nil
}

func (c testGameCoreClient) CheckerRuns(_ context.Context, query GameCheckerRunQuery) (GameCheckerRunPage, error) {
	if c.err != nil {
		return GameCheckerRunPage{}, c.err
	}
	if len(c.runs) == 0 {
		runs := []GameCheckerRun{
			{ID: 1, TickID: 2, TeamID: 101, TeamName: "Team Alpha", ChallengeID: 1, ChallengeName: "banking", Phase: "put", Target: "10.80.1.11:10001", CheckerImage: "registry.local/banking-checker:latest", Status: "success", ExitCode: 0, CheckedAt: "2026-03-10T10:01:01Z"},
			{ID: 2, TickID: 2, TeamID: 101, TeamName: "Team Alpha", ChallengeID: 1, ChallengeName: "banking", Phase: "get", Target: "10.80.1.11:10001", CheckerImage: "registry.local/banking-checker:latest", Status: "failed", ExitCode: 1, Message: "checker returned non-zero", CheckedAt: "2026-03-10T10:01:02Z"},
		}
		return pageTestCheckerRuns(filterTestCheckerRuns(runs, query), query), nil
	}
	return pageTestCheckerRuns(filterTestCheckerRuns(c.runs, query), query), nil
}

func (c testGameCoreClient) SchedulerStatus(_ context.Context) (GameSchedulerStatus, error) {
	if c.err != nil {
		return GameSchedulerStatus{}, c.err
	}
	if c.scheduler.State == "" {
		return GameSchedulerStatus{State: "stopped", IntervalSeconds: 60}, nil
	}
	return c.scheduler, nil
}

func (c testGameCoreClient) SchedulerEvents(_ context.Context, query GameSchedulerEventQuery) (GameSchedulerEventPage, error) {
	if c.err != nil {
		return GameSchedulerEventPage{}, c.err
	}
	if len(c.events) == 0 {
		events := []GameSchedulerEvent{
			{ID: 2, EventType: "tick_completed", Source: "scheduler", State: "running", TickID: 2, Message: "scheduled tick completed successfully", CreatedAt: "2026-03-10T10:02:12Z"},
			{ID: 1, EventType: "started", Source: "organizer", State: "running", Message: "scheduler started by organizer", CreatedAt: "2026-03-10T10:02:00Z"},
		}
		return pageTestSchedulerEvents(filterTestSchedulerEvents(events, query), query), nil
	}
	return pageTestSchedulerEvents(filterTestSchedulerEvents(c.events, query), query), nil
}

func filterTestCheckerRuns(runs []GameCheckerRun, query GameCheckerRunQuery) []GameCheckerRun {
	filtered := make([]GameCheckerRun, 0, len(runs))
	for _, run := range runs {
		if query.TickID > 0 && run.TickID != query.TickID {
			continue
		}
		if query.TeamID > 0 && run.TeamID != query.TeamID {
			continue
		}
		if query.ChallengeID > 0 && run.ChallengeID != query.ChallengeID {
			continue
		}
		if query.Phase != "" && !strings.EqualFold(run.Phase, query.Phase) {
			continue
		}
		if query.Status != "" && !strings.EqualFold(run.Status, query.Status) {
			continue
		}
		filtered = append(filtered, run)
	}
	return filtered
}

func filterTestSchedulerEvents(events []GameSchedulerEvent, query GameSchedulerEventQuery) []GameSchedulerEvent {
	filtered := make([]GameSchedulerEvent, 0, len(events))
	for _, event := range events {
		if query.EventType != "" && !strings.EqualFold(event.EventType, query.EventType) {
			continue
		}
		if query.Source != "" && !strings.EqualFold(event.Source, query.Source) {
			continue
		}
		if query.State != "" && !strings.EqualFold(event.State, query.State) {
			continue
		}
		filtered = append(filtered, event)
	}
	return filtered
}

func pageTestCheckerRuns(runs []GameCheckerRun, query GameCheckerRunQuery) GameCheckerRunPage {
	limit := query.Limit
	if limit <= 0 {
		limit = 25
	}
	offset := query.Offset
	if offset < 0 {
		offset = 0
	}
	page := GameCheckerRunPage{
		Limit:      limit,
		Offset:     offset,
		TotalCount: len(runs),
		HasPrev:    offset > 0,
	}
	if offset >= len(runs) {
		page.Items = []GameCheckerRun{}
		return page
	}
	if limit > len(runs)-offset {
		limit = len(runs) - offset
	}
	page.HasNext = offset+limit < len(runs)
	page.Items = append([]GameCheckerRun(nil), runs[offset:offset+limit]...)
	return page
}

func pageTestSchedulerEvents(events []GameSchedulerEvent, query GameSchedulerEventQuery) GameSchedulerEventPage {
	limit := query.Limit
	if limit <= 0 {
		limit = 25
	}
	offset := query.Offset
	if offset < 0 {
		offset = 0
	}
	page := GameSchedulerEventPage{
		Limit:      limit,
		Offset:     offset,
		TotalCount: len(events),
		HasPrev:    offset > 0,
	}
	if offset >= len(events) {
		page.Items = []GameSchedulerEvent{}
		return page
	}
	if limit > len(events)-offset {
		limit = len(events) - offset
	}
	page.HasNext = offset+limit < len(events)
	page.Items = append([]GameSchedulerEvent(nil), events[offset:offset+limit]...)
	return page
}

func (c testGameCoreClient) StartScheduler(_ context.Context) (GameSchedulerStatus, error) {
	if c.err != nil {
		return GameSchedulerStatus{}, c.err
	}
	status := c.scheduler
	if status.State == "" {
		status = GameSchedulerStatus{State: "running", IntervalSeconds: 60}
	} else {
		status.State = "running"
	}
	if status.NextRunAt == "" {
		status.NextRunAt = "2026-03-10T10:03:00Z"
	}
	return status, nil
}

func (c testGameCoreClient) StopScheduler(_ context.Context) (GameSchedulerStatus, error) {
	if c.err != nil {
		return GameSchedulerStatus{}, c.err
	}
	status := c.scheduler
	if status.State == "" {
		status = GameSchedulerStatus{State: "stopped", IntervalSeconds: 60}
	} else {
		status.State = "stopped"
	}
	status.NextRunAt = ""
	return status, nil
}

func (c testGameCoreClient) UpdateScheduler(_ context.Context, req UpdateSchedulerRequest) (GameSchedulerStatus, error) {
	if c.err != nil {
		return GameSchedulerStatus{}, c.err
	}
	status := c.scheduler
	status.IntervalSeconds = req.IntervalSeconds
	return status, nil
}

func (c testGameCoreClient) SubmitFlags(_ context.Context, _ int, flags []string) ([]submissionVerdict, error) {
	if c.err != nil {
		return nil, c.err
	}
	if len(c.submit) == 0 {
		result := make([]submissionVerdict, 0, len(flags))
		for _, flag := range flags {
			result = append(result, submissionVerdict{Flag: flag, Verdict: "flag is correct."})
		}
		return result, nil
	}
	result := make([]submissionVerdict, 0, len(c.submit))
	for _, item := range c.submit {
		result = append(result, submissionVerdict(item))
	}
	return result, nil
}

func (c testGameCoreClient) Scoreboard(_ context.Context) ([]scoreRow, error) {
	if c.err != nil {
		return nil, c.err
	}
	if len(c.scores) == 0 {
		return []scoreRow{
			{Rank: 1, Team: "Team Alpha", Attack: 40, Defense: 30, SLA: 28, Total: 98, Delta: "+1"},
			{Rank: 2, Team: "Team Delta", Attack: 30, Defense: 20, SLA: 24, Total: 74, Delta: "-1"},
		}, nil
	}
	result := make([]scoreRow, 0, len(c.scores))
	for _, item := range c.scores {
		result = append(result, scoreRow(item))
	}
	return result, nil
}

func (c testGameCoreClient) AttackFeed(_ context.Context, query AttackFeedQuery) (AttackFeedPage, error) {
	if c.err != nil {
		return AttackFeedPage{}, c.err
	}
	if len(c.attacks) == 0 {
		limit := query.Limit
		if limit <= 0 {
			limit = 12
		}
		return AttackFeedPage{
			Items: []attackEvent{
				{ID: "atk-1", Attacker: "Team Alpha", Victim: "Team Delta", Service: "banking", Tick: 2, Verdict: "first valid submission accepted"},
			},
			Limit:      limit,
			Offset:     query.Offset,
			TotalCount: 1,
		}, nil
	}
	result := make([]attackEvent, 0, len(c.attacks))
	for _, item := range c.attacks {
		result = append(result, attackEvent(item))
	}
	return pageTestAttackEvents(result, query), nil
}

func pageTestAttackEvents(events []attackEvent, query AttackFeedQuery) AttackFeedPage {
	return paginateAttackFeed(events, query)
}

func (c testGameCoreClient) RecomputeScoring(_ context.Context) ([]scoreRow, error) {
	return c.Scoreboard(context.Background())
}

func (c testSubmissionClient) SubmitFlags(_ context.Context, _ int, flags []string) ([]submissionVerdict, error) {
	if c.err != nil {
		return nil, c.err
	}
	if len(c.submit) == 0 {
		result := make([]submissionVerdict, 0, len(flags))
		for _, flag := range flags {
			result = append(result, submissionVerdict{Flag: flag, Verdict: "submission-service accepted the flag."})
		}
		return result, nil
	}
	result := make([]submissionVerdict, 0, len(c.submit))
	for _, item := range c.submit {
		result = append(result, submissionVerdict(item))
	}
	return result, nil
}

func (c testSubmissionClient) AttackFeed(_ context.Context, query AttackFeedQuery) (AttackFeedPage, error) {
	if c.err != nil {
		return AttackFeedPage{}, c.err
	}
	events := make([]attackEvent, 0, len(c.attacks))
	if len(c.attacks) == 0 {
		events = append(events, attackEvent{
			ID:       "atk-submission-service",
			Attacker: "Team Submission",
			Victim:   "Team Target",
			Service:  "banking",
			Tick:     7,
			Verdict:  "submission-service attack feed",
		})
	} else {
		for _, item := range c.attacks {
			events = append(events, attackEvent(item))
		}
	}
	return pageTestAttackEvents(events, query), nil
}

func (c testScoringClient) Scoreboard(_ context.Context) ([]scoreRow, error) {
	if c.err != nil {
		return nil, c.err
	}
	if len(c.scores) == 0 {
		return []scoreRow{{Rank: 1, Team: "Team Worker", Attack: 12, Defense: 10, SLA: 9, Total: 31, Delta: "new"}}, nil
	}
	result := make([]scoreRow, 0, len(c.scores))
	for _, item := range c.scores {
		result = append(result, scoreRow(item))
	}
	return result, nil
}

func (c testScoringClient) RecomputeScoring(_ context.Context) ([]scoreRow, error) {
	return c.Scoreboard(context.Background())
}

func newTestMux() *http.ServeMux {
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "api-gateway", Version: "dev", Addr: ":0"})
	NewWithDeps("dev-team-token", "dev-admin-token", 101, NewMemoryStore(101), testControllerClient{}, noopWireGuardClient{}).RegisterRoutes(mux)
	return mux
}

func newTestMuxWithGameCore(game gameCoreClient) *http.ServeMux {
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "api-gateway", Version: "dev", Addr: ":0"})
	NewWithDeps("dev-team-token", "dev-admin-token", 101, NewMemoryStore(101), testControllerClient{}, noopWireGuardClient{}, game).RegisterRoutes(mux)
	return mux
}

func newTestMuxWithOps(game gameCoreClient, controller controllerClient, wireGuard wireGuardClient) *http.ServeMux {
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "api-gateway", Version: "dev", Addr: ":0"})
	NewWithDeps("dev-team-token", "dev-admin-token", 101, NewMemoryStore(101), controller, wireGuard, game).RegisterRoutes(mux)
	return mux
}

func newTestMuxWithWorkers(game gameCoreClient, submission submissionClient, scoring scoringClient) *http.ServeMux {
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "api-gateway", Version: "dev", Addr: ":0"})
	server := NewWithDeps("dev-team-token", "dev-admin-token", 101, NewMemoryStore(101), testControllerClient{}, noopWireGuardClient{}, game)
	server.WithSubmissionClient(submission)
	server.WithScoringClient(scoring)
	server.RegisterRoutes(mux)
	return mux
}

func newTestMuxWithLimiter(limiter rateLimiter) *http.ServeMux {
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "api-gateway", Version: "dev", Addr: ":0"})
	server := NewWithDeps("dev-team-token", "dev-admin-token", 101, NewMemoryStore(101), testControllerClient{}, noopWireGuardClient{})
	server.WithRateLimiter(limiter)
	server.RegisterRoutes(mux)
	return mux
}

func testUnlockProof(teamID, challengeID int) string {
	return unlockproof.Issue("dev-team-token", teamID, challengeID)
}

func TestAuthenticate(t *testing.T) {
	mux := newTestMux()
	body := bytes.NewBufferString(`{"email":"alpha.captain@example.com","password":"alpha-secret"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v2/authenticate", body)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}

	var payload struct {
		Status string `json:"status"`
		Data   string `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode auth response: %v", err)
	}
	if payload.Data == "" || bytes.Count([]byte(payload.Data), []byte(".")) != 2 {
		t.Fatalf("expected jwt-like token, got %q", payload.Data)
	}
}

func TestAuthenticateReturnsRateLimit429(t *testing.T) {
	mux := newTestMuxWithLimiter(testRateLimiter{
		denyKeys: map[string]bool{
			rateLimitAuthKey("alpha.captain@example.com", "203.0.113.10"): true,
		},
		retryAfter: 4 * time.Second,
	})
	body := bytes.NewBufferString(`{"email":"alpha.captain@example.com","password":"alpha-secret"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v2/authenticate", body)
	request.RemoteAddr = "203.0.113.10:51234"
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", response.Code)
	}
	if got := response.Header().Get("Retry-After"); got != "4" {
		t.Fatalf("expected Retry-After 4, got %q", got)
	}

	var payload httpapi.ErrorEnvelope
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode 429 response: %v", err)
	}
	if payload.Status != "too many request" {
		t.Fatalf("unexpected status %q", payload.Status)
	}
	if payload.Message != defaultRateLimit429Message {
		t.Fatalf("unexpected rate-limit message %q", payload.Message)
	}
}

func TestSubmitRequiresAuth(t *testing.T) {
	mux := newTestMux()
	body := bytes.NewBufferString(`{"flags":["FLAGv1.demo"]}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v2/submit", body)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", response.Code)
	}
}

func TestSubmitReturnsRateLimit429(t *testing.T) {
	mux := newTestMuxWithLimiter(testRateLimiter{
		denyKeys: map[string]bool{
			rateLimitTeamKey("submit", 101): true,
		},
		retryAfter: 3 * time.Second,
	})
	body := bytes.NewBufferString(`{"flags":["FLAGv1.demo"]}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v2/submit", body)
	request.Header.Set("Authorization", "Bearer dev-team-token")
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", response.Code)
	}
	if got := response.Header().Get("Retry-After"); got != "3" {
		t.Fatalf("expected Retry-After 3, got %q", got)
	}

	var payload httpapi.ErrorEnvelope
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode 429 response: %v", err)
	}
	if payload.Status != "too many request" {
		t.Fatalf("unexpected status %q", payload.Status)
	}
	if payload.Message != defaultRateLimit429Message {
		t.Fatalf("unexpected rate-limit message %q", payload.Message)
	}
}

func TestSubmitReturnsDuplicateVerdict(t *testing.T) {
	mux := newTestMuxWithGameCore(testGameCoreClient{
		submit: []SubmissionVerdictAlias{
			{Flag: "FLAGv1.demo", Verdict: "flag is correct."},
			{Flag: "FLAGv1.demo", Verdict: "flag already submitted."},
			{Flag: "bad", Verdict: "flag is wrong or expired."},
		},
	})
	body := bytes.NewBufferString(`{"flags":["FLAGv1.demo","FLAGv1.demo","bad"]}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v2/submit", body)
	request.Header.Set("Authorization", "Bearer dev-team-token")
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}

	var payload struct {
		Status string `json:"status"`
		Data   []struct {
			Flag    string `json:"flag"`
			Verdict string `json:"verdict"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(payload.Data) != 3 {
		t.Fatalf("expected 3 verdicts, got %d", len(payload.Data))
	}
	if payload.Data[0].Verdict != "flag is correct." {
		t.Fatalf("unexpected first verdict: %s", payload.Data[0].Verdict)
	}
	if payload.Data[1].Verdict != "flag already submitted." {
		t.Fatalf("unexpected duplicate verdict: %s", payload.Data[1].Verdict)
	}
}

func TestSubmitReturnsServiceUnavailableWithoutAuthoritativeBackend(t *testing.T) {
	mux := newTestMux()
	body := bytes.NewBufferString(`{"flags":["FLAGv1.demo"]}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v2/submit", body)
	request.Header.Set("Authorization", "Bearer dev-team-token")
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", response.Code)
	}
}

func TestScoreboardEndpoint(t *testing.T) {
	mux := newTestMux()
	request := httptest.NewRequest(http.MethodGet, "/api/v2/scoreboard", nil)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}

	var payload struct {
		Status string     `json:"status"`
		Data   []scoreRow `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(payload.Data) == 0 {
		t.Fatal("expected non-empty scoreboard")
	}
}

func TestChallengesReturnsRateLimit429(t *testing.T) {
	mux := newTestMuxWithLimiter(testRateLimiter{
		denyKeys: map[string]bool{
			rateLimitClientKey("challenges", "203.0.113.20"): true,
		},
		retryAfter: 2 * time.Second,
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v2/challenges", nil)
	request.RemoteAddr = "203.0.113.20:40123"
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", response.Code)
	}
	if got := response.Header().Get("Retry-After"); got != "2" {
		t.Fatalf("expected Retry-After 2, got %q", got)
	}
}

func TestServicesReturnsRateLimit429(t *testing.T) {
	mux := newTestMuxWithLimiter(testRateLimiter{
		denyKeys: map[string]bool{
			rateLimitTeamKey("services", 101): true,
		},
		retryAfter: 2 * time.Second,
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v2/services", nil)
	request.Header.Set("Authorization", "Bearer dev-team-token")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", response.Code)
	}
	if got := response.Header().Get("Retry-After"); got != "2" {
		t.Fatalf("expected Retry-After 2, got %q", got)
	}
}

func TestScoreboardReturnsRateLimit429(t *testing.T) {
	mux := newTestMuxWithLimiter(testRateLimiter{
		denyKeys: map[string]bool{
			rateLimitClientKey("scoreboard", "203.0.113.21"): true,
		},
		retryAfter: 2 * time.Second,
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v2/scoreboard", nil)
	request.RemoteAddr = "203.0.113.21:40124"
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", response.Code)
	}
	if got := response.Header().Get("Retry-After"); got != "2" {
		t.Fatalf("expected Retry-After 2, got %q", got)
	}
}

func TestScoreboardPrefersGameCoreWhenConfigured(t *testing.T) {
	mux := newTestMuxWithGameCore(testGameCoreClient{
		scores: []ScoreRowAlias{
			{Rank: 1, Team: "Team Omega", Attack: 90, Defense: 40, SLA: 28, Total: 158, Delta: "+2"},
		},
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v2/scoreboard", nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}

	var payload struct {
		Status string     `json:"status"`
		Data   []scoreRow `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode scoreboard response: %v", err)
	}
	if len(payload.Data) != 1 || payload.Data[0].Team != "Team Omega" {
		t.Fatalf("expected authoritative game-core scoreboard, got %+v", payload.Data)
	}
}

func TestScoreboardPrefersScoringWorkerWhenConfigured(t *testing.T) {
	mux := newTestMuxWithWorkers(
		testGameCoreClient{
			scores: []ScoreRowAlias{
				{Rank: 1, Team: "Team Core", Attack: 50, Defense: 30, SLA: 20, Total: 100, Delta: "+1"},
			},
		},
		noopSubmissionClient{},
		testScoringClient{
			scores: []ScoreRowAlias{
				{Rank: 1, Team: "Team Worker", Attack: 12, Defense: 10, SLA: 9, Total: 31, Delta: "new"},
			},
		},
	)

	request := httptest.NewRequest(http.MethodGet, "/api/v2/scoreboard", nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}

	var payload struct {
		Status string     `json:"status"`
		Data   []scoreRow `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode scoreboard response: %v", err)
	}
	if len(payload.Data) != 1 || payload.Data[0].Team != "Team Worker" {
		t.Fatalf("expected scoring-worker scoreboard, got %+v", payload.Data)
	}
}

func TestAttacksPrefersGameCoreWhenConfigured(t *testing.T) {
	mux := newTestMuxWithGameCore(testGameCoreClient{
		attacks: []AttackEventAlias{
			{ID: "atk-core-1", Attacker: "Team Omega", Victim: "Team Delta", Service: "banking", Tick: 9, Verdict: "first valid submission accepted"},
		},
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v2/attacks", nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}

	var payload struct {
		Status string         `json:"status"`
		Data   AttackFeedPage `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode attack feed response: %v", err)
	}
	if len(payload.Data.Items) != 1 || payload.Data.Items[0].ID != "atk-core-1" {
		t.Fatalf("expected authoritative game-core attack feed, got %+v", payload.Data)
	}
	if payload.Data.TotalCount != 1 || payload.Data.HasNext || payload.Data.HasPrev {
		t.Fatalf("unexpected attack feed page metadata %+v", payload.Data)
	}
}

func TestAttacksPrefersSubmissionServiceWhenConfigured(t *testing.T) {
	mux := newTestMuxWithWorkers(
		testGameCoreClient{
			attacks: []AttackEventAlias{
				{ID: "atk-core-1", Attacker: "Team Core", Victim: "Team Delta", Service: "banking", Tick: 9, Verdict: "game-core"},
			},
		},
		testSubmissionClient{
			attacks: []AttackEventAlias{
				{ID: "atk-sub-1", Attacker: "Team Submission", Victim: "Team Delta", Service: "banking", Tick: 7, Verdict: "submission-service"},
			},
		},
		noopScoringClient{},
	)

	request := httptest.NewRequest(http.MethodGet, "/api/v2/attacks", nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}

	var payload struct {
		Status string         `json:"status"`
		Data   AttackFeedPage `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode attack feed response: %v", err)
	}
	if len(payload.Data.Items) != 1 || payload.Data.Items[0].ID != "atk-sub-1" {
		t.Fatalf("expected submission-service attack feed, got %+v", payload.Data)
	}
}

func TestAttacksSupportsTextFilters(t *testing.T) {
	mux := newTestMuxWithGameCore(testGameCoreClient{
		attacks: []AttackEventAlias{
			{ID: "atk-core-1", Attacker: "Team Alpha", Victim: "Team Delta", Service: "banking", Tick: 9, Verdict: "first valid submission accepted"},
			{ID: "atk-core-2", Attacker: "Team Sigma", Victim: "Team Alpha", Service: "chat", Tick: 8, Verdict: "first valid submission accepted"},
		},
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v2/attacks?attacker=sigma&service=hat", nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}

	var payload struct {
		Status string         `json:"status"`
		Data   AttackFeedPage `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode filtered attack feed response: %v", err)
	}
	if len(payload.Data.Items) != 1 || payload.Data.Items[0].ID != "atk-core-2" {
		t.Fatalf("expected filtered attack feed, got %+v", payload.Data)
	}
	if payload.Data.TotalCount != 1 || payload.Data.HasNext || payload.Data.HasPrev {
		t.Fatalf("unexpected filtered attack feed page metadata %+v", payload.Data)
	}
}

func TestAttacksReturnsRateLimit429(t *testing.T) {
	mux := newTestMuxWithLimiter(testRateLimiter{
		denyKeys: map[string]bool{
			rateLimitClientKey("attacks", "203.0.113.22"): true,
		},
		retryAfter: 2 * time.Second,
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v2/attacks", nil)
	request.RemoteAddr = "203.0.113.22:40125"
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", response.Code)
	}
	if got := response.Header().Get("Retry-After"); got != "2" {
		t.Fatalf("expected Retry-After 2, got %q", got)
	}
}

func TestAttacksSupportsTickRangeFilters(t *testing.T) {
	mux := newTestMuxWithGameCore(testGameCoreClient{
		attacks: []AttackEventAlias{
			{ID: "atk-core-1", Attacker: "Team Alpha", Victim: "Team Delta", Service: "banking", Tick: 11, Verdict: "first valid submission accepted"},
			{ID: "atk-core-2", Attacker: "Team Sigma", Victim: "Team Alpha", Service: "chat", Tick: 9, Verdict: "first valid submission accepted"},
		},
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v2/attacks?tick_from=10&tick_to=11", nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}

	var payload struct {
		Status string         `json:"status"`
		Data   AttackFeedPage `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode tick-filtered attack feed response: %v", err)
	}
	if len(payload.Data.Items) != 1 || payload.Data.Items[0].ID != "atk-core-1" {
		t.Fatalf("expected tick-filtered attack feed, got %+v", payload.Data)
	}
	if payload.Data.TotalCount != 1 || payload.Data.HasNext || payload.Data.HasPrev {
		t.Fatalf("unexpected tick-filtered attack feed page metadata %+v", payload.Data)
	}
}

func TestUnlockRejectsInvalidProof(t *testing.T) {
	mux := newTestMux()

	request := httptest.NewRequest(http.MethodPost, "/api/v2/services/2/unlock", bytes.NewBufferString(`{"proof":"wrong-proof"}`))
	request.Header.Set("Authorization", "Bearer dev-team-token")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", response.Code)
	}

	var payload struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode unlock response: %v", err)
	}
	if payload.Message != "unlock proof is invalid." {
		t.Fatalf("unexpected unlock message %q", payload.Message)
	}
}

func TestTeamServicesReflectUnlockAndResetState(t *testing.T) {
	mux := newTestMux()

	unlockRequest := httptest.NewRequest(http.MethodPost, "/api/v2/services/2/unlock", bytes.NewBufferString(`{"proof":"`+testUnlockProof(101, 2)+`"}`))
	unlockRequest.Header.Set("Authorization", "Bearer dev-team-token")
	unlockResponse := httptest.NewRecorder()
	mux.ServeHTTP(unlockResponse, unlockRequest)
	if unlockResponse.Code != http.StatusOK {
		t.Fatalf("expected unlock 200, got %d", unlockResponse.Code)
	}

	resetRequest := httptest.NewRequest(http.MethodPost, "/api/v2/services/2/reset/factory", nil)
	resetRequest.Header.Set("Authorization", "Bearer dev-team-token")
	resetResponse := httptest.NewRecorder()
	mux.ServeHTTP(resetResponse, resetRequest)
	if resetResponse.Code != http.StatusOK {
		t.Fatalf("expected reset 200, got %d", resetResponse.Code)
	}

	servicesRequest := httptest.NewRequest(http.MethodGet, "/api/v2/team/services", nil)
	servicesRequest.Header.Set("Authorization", "Bearer dev-team-token")
	servicesResponse := httptest.NewRecorder()
	mux.ServeHTTP(servicesResponse, servicesRequest)
	if servicesResponse.Code != http.StatusOK {
		t.Fatalf("expected services 200, got %d", servicesResponse.Code)
	}

	var payload struct {
		Status string         `json:"status"`
		Data   []serviceState `json:"data"`
	}
	if err := json.Unmarshal(servicesResponse.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode services response: %v", err)
	}

	var chatState *serviceState
	for idx := range payload.Data {
		if payload.Data[idx].ChallengeID == 2 {
			chatState = &payload.Data[idx]
			break
		}
	}
	if chatState == nil {
		t.Fatal("expected chat service state")
	}
	if !chatState.Unlocked {
		t.Fatal("expected unlock to be preserved in readable service state")
	}
	if chatState.Status != "warming" {
		t.Fatalf("expected warming status after reset, got %s", chatState.Status)
	}
}

func TestTeamServicesReturnsRateLimit429(t *testing.T) {
	mux := newTestMuxWithLimiter(testRateLimiter{
		denyKeys: map[string]bool{
			rateLimitTeamKey("team-services", 101): true,
		},
		retryAfter: 2 * time.Second,
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v2/team/services", nil)
	request.Header.Set("Authorization", "Bearer dev-team-token")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", response.Code)
	}
	if got := response.Header().Get("Retry-After"); got != "2" {
		t.Fatalf("expected Retry-After 2, got %q", got)
	}
}

func TestUnlockSurvivesFactoryResetForSSH(t *testing.T) {
	mux := newTestMux()
	unlockRequest := httptest.NewRequest(http.MethodPost, "/api/v2/services/1/unlock", bytes.NewBufferString(`{"proof":"`+testUnlockProof(101, 1)+`"}`))
	unlockRequest.Header.Set("Authorization", "Bearer dev-team-token")
	unlockResponse := httptest.NewRecorder()
	mux.ServeHTTP(unlockResponse, unlockRequest)
	if unlockResponse.Code != http.StatusOK {
		t.Fatalf("expected unlock 200, got %d", unlockResponse.Code)
	}

	resetRequest := httptest.NewRequest(http.MethodPost, "/api/v2/services/1/reset/factory", nil)
	resetRequest.Header.Set("Authorization", "Bearer dev-team-token")
	resetResponse := httptest.NewRecorder()
	mux.ServeHTTP(resetResponse, resetRequest)
	if resetResponse.Code != http.StatusOK {
		t.Fatalf("expected reset 200, got %d", resetResponse.Code)
	}

	sshRequest := httptest.NewRequest(http.MethodPost, "/api/v2/services/1/ssh-session", nil)
	sshRequest.Header.Set("Authorization", "Bearer dev-team-token")
	sshResponse := httptest.NewRecorder()
	mux.ServeHTTP(sshResponse, sshRequest)
	if sshResponse.Code != http.StatusOK {
		t.Fatalf("expected ssh session 200 after reset, got %d", sshResponse.Code)
	}

	var payload struct {
		Status string         `json:"status"`
		Data   sshSessionData `json:"data"`
	}
	if err := json.Unmarshal(sshResponse.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode ssh response: %v", err)
	}
	if payload.Data.Host != "10.80.1.11" {
		t.Fatalf("expected same service ip host, got %s", payload.Data.Host)
	}
	if payload.Data.Username != "root" {
		t.Fatalf("expected root user, got %s", payload.Data.Username)
	}
	if payload.Data.Password == "" {
		t.Fatal("expected one-time root password to be issued")
	}
	if payload.Data.ConnectionHint != "ssh root@10.80.1.11" {
		t.Fatalf("unexpected connection hint: %s", payload.Data.ConnectionHint)
	}
}

func TestSSHSessionReturnsBadGatewayWhenRuntimeApplyFails(t *testing.T) {
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "api-gateway", Version: "dev", Addr: ":0"})
	store := NewMemoryStore(101)
	NewWithDeps("dev-team-token", "dev-admin-token", 101, store, testControllerClient{sshApplyErr: fmt.Errorf("runtime apply failed")}, noopWireGuardClient{}).RegisterRoutes(mux)

	unlockRequest := httptest.NewRequest(http.MethodPost, "/api/v2/services/1/unlock", bytes.NewBufferString(`{"proof":"`+testUnlockProof(101, 1)+`"}`))
	unlockRequest.Header.Set("Authorization", "Bearer dev-team-token")
	unlockResponse := httptest.NewRecorder()
	mux.ServeHTTP(unlockResponse, unlockRequest)
	if unlockResponse.Code != http.StatusOK {
		t.Fatalf("expected unlock 200, got %d", unlockResponse.Code)
	}

	sshRequest := httptest.NewRequest(http.MethodPost, "/api/v2/services/1/ssh-session", nil)
	sshRequest.Header.Set("Authorization", "Bearer dev-team-token")
	sshResponse := httptest.NewRecorder()
	mux.ServeHTTP(sshResponse, sshRequest)
	if sshResponse.Code != http.StatusBadGateway {
		t.Fatalf("expected ssh session 502, got %d", sshResponse.Code)
	}

	servicesRequest := httptest.NewRequest(http.MethodGet, "/api/v2/team/services", nil)
	servicesRequest.Header.Set("Authorization", "Bearer dev-team-token")
	servicesResponse := httptest.NewRecorder()
	mux.ServeHTTP(servicesResponse, servicesRequest)
	if servicesResponse.Code != http.StatusOK {
		t.Fatalf("expected services 200, got %d", servicesResponse.Code)
	}

	var payload struct {
		Status string         `json:"status"`
		Data   []serviceState `json:"data"`
	}
	if err := json.Unmarshal(servicesResponse.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode services response: %v", err)
	}
	for _, state := range payload.Data {
		if state.ChallengeID == 1 {
			if state.SSHHint != "credential apply failed; request a fresh one-time root password to retry" {
				t.Fatalf("unexpected ssh hint %q", state.SSHHint)
			}
			if state.LastEvent != "ssh credential apply failed" {
				t.Fatalf("unexpected last event %q", state.LastEvent)
			}
			return
		}
	}
	t.Fatal("expected service state for challenge 1")
}

func TestAdminAuditLogCapturesParticipantServiceActions(t *testing.T) {
	mux := newTestMux()

	unlockRequest := httptest.NewRequest(http.MethodPost, "/api/v2/services/1/unlock", bytes.NewBufferString(`{"proof":"`+testUnlockProof(101, 1)+`"}`))
	unlockRequest.Header.Set("Authorization", "Bearer dev-team-token")
	unlockResponse := httptest.NewRecorder()
	mux.ServeHTTP(unlockResponse, unlockRequest)
	if unlockResponse.Code != http.StatusOK {
		t.Fatalf("expected unlock 200, got %d", unlockResponse.Code)
	}

	resetRequest := httptest.NewRequest(http.MethodPost, "/api/v2/services/1/reset/factory", nil)
	resetRequest.Header.Set("Authorization", "Bearer dev-team-token")
	resetResponse := httptest.NewRecorder()
	mux.ServeHTTP(resetResponse, resetRequest)
	if resetResponse.Code != http.StatusOK {
		t.Fatalf("expected reset 200, got %d", resetResponse.Code)
	}

	sshRequest := httptest.NewRequest(http.MethodPost, "/api/v2/services/1/ssh-session", nil)
	sshRequest.Header.Set("Authorization", "Bearer dev-team-token")
	sshResponse := httptest.NewRecorder()
	mux.ServeHTTP(sshResponse, sshRequest)
	if sshResponse.Code != http.StatusOK {
		t.Fatalf("expected ssh 200, got %d", sshResponse.Code)
	}

	auditRequest := httptest.NewRequest(http.MethodGet, "/api/v2/admin/audit-logs?actor_type=team&limit=10", nil)
	auditRequest.Header.Set("Authorization", "Bearer dev-admin-token")
	auditResponse := httptest.NewRecorder()
	mux.ServeHTTP(auditResponse, auditRequest)
	if auditResponse.Code != http.StatusOK {
		t.Fatalf("expected audit 200, got %d", auditResponse.Code)
	}

	var payload struct {
		Status string            `json:"status"`
		Data   adminAuditLogPage `json:"data"`
	}
	if err := json.Unmarshal(auditResponse.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode audit response: %v", err)
	}
	if payload.Data.TotalCount < 3 {
		t.Fatalf("expected at least 3 audit entries, got %d", payload.Data.TotalCount)
	}

	joinedActions := make([]string, 0, len(payload.Data.Items))
	for _, item := range payload.Data.Items {
		joinedActions = append(joinedActions, item.Action)
	}
	actionSet := strings.Join(joinedActions, ",")
	if !strings.Contains(actionSet, "service.unlock") {
		t.Fatalf("expected unlock audit action, got %v", joinedActions)
	}
	if !strings.Contains(actionSet, "service.factory_reset") {
		t.Fatalf("expected factory reset audit action, got %v", joinedActions)
	}
	if !strings.Contains(actionSet, "service.ssh_session") {
		t.Fatalf("expected ssh audit action, got %v", joinedActions)
	}
}

func TestAdminAuditLogCanFilterChallengeCreate(t *testing.T) {
	mux := newTestMux()

	body := bytes.NewBufferString(`{"name":"audit-demo","baseline_image":"registry.local/audit-demo:baseline","checker_image":"registry.local/audit-demo-checker:latest","weight":1,"service_port":31010,"service_subnet_octet":10}`)
	createRequest := httptest.NewRequest(http.MethodPost, "/api/v2/admin/challenges", body)
	createRequest.Header.Set("Authorization", "Bearer dev-admin-token")
	createResponse := httptest.NewRecorder()
	mux.ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusOK {
		t.Fatalf("expected create challenge 200, got %d", createResponse.Code)
	}

	auditRequest := httptest.NewRequest(http.MethodGet, "/api/v2/admin/audit-logs?action=challenge.create&target_type=challenge", nil)
	auditRequest.Header.Set("Authorization", "Bearer dev-admin-token")
	auditResponse := httptest.NewRecorder()
	mux.ServeHTTP(auditResponse, auditRequest)
	if auditResponse.Code != http.StatusOK {
		t.Fatalf("expected audit 200, got %d", auditResponse.Code)
	}

	var payload struct {
		Status string            `json:"status"`
		Data   adminAuditLogPage `json:"data"`
	}
	if err := json.Unmarshal(auditResponse.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode audit response: %v", err)
	}
	if payload.Data.TotalCount == 0 {
		t.Fatal("expected at least one challenge.create audit entry")
	}
	if payload.Data.Items[0].Action != "challenge.create" {
		t.Fatalf("expected challenge.create action, got %s", payload.Data.Items[0].Action)
	}
}

func TestAdminRequiresAuth(t *testing.T) {
	mux := newTestMux()
	request := httptest.NewRequest(http.MethodGet, "/api/v2/admin/teams", nil)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", response.Code)
	}
}

func TestAdminCanCreateTeamPlayerAndDeployChallenge(t *testing.T) {
	mux := newTestMux()
	adminAuth := "Bearer dev-admin-token"

	teamRequest := httptest.NewRequest(http.MethodPost, "/api/v2/admin/teams", bytes.NewBufferString(`{"name":"Team Nova","contact_email":"team-nova@example.com"}`))
	teamRequest.Header.Set("Authorization", adminAuth)
	teamResponse := httptest.NewRecorder()
	mux.ServeHTTP(teamResponse, teamRequest)
	if teamResponse.Code != http.StatusOK {
		t.Fatalf("expected team create 200, got %d", teamResponse.Code)
	}

	var teamPayload struct {
		Status string    `json:"status"`
		Data   adminTeam `json:"data"`
	}
	if err := json.Unmarshal(teamResponse.Body.Bytes(), &teamPayload); err != nil {
		t.Fatalf("failed to decode team response: %v", err)
	}

	playerRequest := httptest.NewRequest(http.MethodPost, "/api/v2/admin/players", bytes.NewBufferString(fmt.Sprintf(`{"team_id":%d,"display_name":"Nova Captain","email":"nova.captain@example.com","password":"nova-secret","role":"captain"}`, teamPayload.Data.ID)))
	playerRequest.Header.Set("Authorization", adminAuth)
	playerResponse := httptest.NewRecorder()
	mux.ServeHTTP(playerResponse, playerRequest)
	if playerResponse.Code != http.StatusOK {
		t.Fatalf("expected player create 200, got %d", playerResponse.Code)
	}

	challengeRequest := httptest.NewRequest(http.MethodPost, "/api/v2/admin/challenges", bytes.NewBufferString(`{"name":"proxy","baseline_image":"registry.local/proxy:baseline","checker_image":"registry.local/proxy-checker:latest","weight":2}`))
	challengeRequest.Header.Set("Authorization", adminAuth)
	challengeResponse := httptest.NewRecorder()
	mux.ServeHTTP(challengeResponse, challengeRequest)
	if challengeResponse.Code != http.StatusOK {
		t.Fatalf("expected challenge create 200, got %d", challengeResponse.Code)
	}

	var challengePayload struct {
		Status string         `json:"status"`
		Data   adminChallenge `json:"data"`
	}
	if err := json.Unmarshal(challengeResponse.Body.Bytes(), &challengePayload); err != nil {
		t.Fatalf("failed to decode challenge response: %v", err)
	}

	deployRequest := httptest.NewRequest(http.MethodPost, "/api/v2/admin/challenges/"+fmt.Sprintf("%d", challengePayload.Data.ID)+"/deploy", nil)
	deployRequest.Header.Set("Authorization", adminAuth)
	deployResponse := httptest.NewRecorder()
	mux.ServeHTTP(deployResponse, deployRequest)
	if deployResponse.Code != http.StatusOK {
		t.Fatalf("expected deploy 200, got %d", deployResponse.Code)
	}
	var deployPayload struct {
		Status string          `json:"status"`
		Data   adminDeployment `json:"data"`
	}
	if err := json.Unmarshal(deployResponse.Body.Bytes(), &deployPayload); err != nil {
		t.Fatalf("failed to decode deploy response: %v", err)
	}
	if deployPayload.Data.Status != "queued" {
		t.Fatalf("expected queued deploy status, got %s", deployPayload.Data.Status)
	}
	if deployPayload.Data.JobID == 0 {
		t.Fatal("expected deployment job id")
	}

	servicesRequest := httptest.NewRequest(http.MethodGet, "/api/v2/services", nil)
	servicesRequest.Header.Set("Authorization", "Bearer dev-team-token")
	servicesResponse := httptest.NewRecorder()
	mux.ServeHTTP(servicesResponse, servicesRequest)
	if servicesResponse.Code != http.StatusOK {
		t.Fatalf("expected services 200, got %d", servicesResponse.Code)
	}

	var servicesPayload struct {
		Status string                         `json:"status"`
		Data   map[string]map[string][]string `json:"data"`
	}
	if err := json.Unmarshal(servicesResponse.Body.Bytes(), &servicesPayload); err != nil {
		t.Fatalf("failed to decode services response: %v", err)
	}
	challengeKey := fmt.Sprintf("%d", challengePayload.Data.ID)
	if _, ok := servicesPayload.Data[challengeKey]; !ok {
		t.Fatalf("expected deployed challenge %s in public service map", challengeKey)
	}

	deploymentsRequest := httptest.NewRequest(http.MethodGet, "/api/v2/admin/deployments", nil)
	deploymentsRequest.Header.Set("Authorization", adminAuth)
	deploymentsResponse := httptest.NewRecorder()
	mux.ServeHTTP(deploymentsResponse, deploymentsRequest)
	if deploymentsResponse.Code != http.StatusOK {
		t.Fatalf("expected deployment list 200, got %d", deploymentsResponse.Code)
	}

	reconcileRequest := httptest.NewRequest(http.MethodPost, "/api/v2/admin/deployments/reconcile", nil)
	reconcileRequest.Header.Set("Authorization", adminAuth)
	reconcileResponse := httptest.NewRecorder()
	mux.ServeHTTP(reconcileResponse, reconcileRequest)
	if reconcileResponse.Code != http.StatusOK {
		t.Fatalf("expected deployment reconcile 200, got %d", reconcileResponse.Code)
	}

	teamServicesRequest := httptest.NewRequest(http.MethodGet, "/api/v2/team/services", nil)
	teamServicesRequest.Header.Set("Authorization", "Bearer dev-team-token")
	teamServicesResponse := httptest.NewRecorder()
	mux.ServeHTTP(teamServicesResponse, teamServicesRequest)
	if teamServicesResponse.Code != http.StatusOK {
		t.Fatalf("expected team services 200, got %d", teamServicesResponse.Code)
	}

	var teamServicesPayload struct {
		Status string         `json:"status"`
		Data   []serviceState `json:"data"`
	}
	if err := json.Unmarshal(teamServicesResponse.Body.Bytes(), &teamServicesPayload); err != nil {
		t.Fatalf("failed to decode team services response: %v", err)
	}
	foundReady := false
	for _, service := range teamServicesPayload.Data {
		if service.ChallengeID == challengePayload.Data.ID {
			foundReady = service.Status == "stable"
			break
		}
	}
	if !foundReady {
		t.Fatal("expected reconciled challenge to reach stable service state")
	}
}

func TestAdminDeployUsesConfiguredServiceSubnetAndPort(t *testing.T) {
	mux := newTestMux()
	adminAuth := "Bearer dev-admin-token"

	challengeRequest := httptest.NewRequest(http.MethodPost, "/api/v2/admin/challenges", bytes.NewBufferString(`{"name":"proxy-custom","baseline_image":"registry.local/proxy-custom:baseline","checker_image":"registry.local/proxy-custom-checker:latest","weight":2,"service_port":31337,"service_subnet_octet":77}`))
	challengeRequest.Header.Set("Authorization", adminAuth)
	challengeResponse := httptest.NewRecorder()
	mux.ServeHTTP(challengeResponse, challengeRequest)
	if challengeResponse.Code != http.StatusOK {
		t.Fatalf("expected challenge create 200, got %d", challengeResponse.Code)
	}

	var challengePayload struct {
		Status string         `json:"status"`
		Data   adminChallenge `json:"data"`
	}
	if err := json.Unmarshal(challengeResponse.Body.Bytes(), &challengePayload); err != nil {
		t.Fatalf("failed to decode challenge response: %v", err)
	}
	if challengePayload.Data.ServicePort != 31337 || challengePayload.Data.ServiceSubnetOctet != 77 {
		t.Fatalf("unexpected runtime spec %+v", challengePayload.Data)
	}

	deployRequest := httptest.NewRequest(http.MethodPost, "/api/v2/admin/challenges/"+fmt.Sprintf("%d", challengePayload.Data.ID)+"/deploy", nil)
	deployRequest.Header.Set("Authorization", adminAuth)
	deployResponse := httptest.NewRecorder()
	mux.ServeHTTP(deployResponse, deployRequest)
	if deployResponse.Code != http.StatusOK {
		t.Fatalf("expected deploy 200, got %d", deployResponse.Code)
	}

	reconcileRequest := httptest.NewRequest(http.MethodPost, "/api/v2/admin/deployments/reconcile", nil)
	reconcileRequest.Header.Set("Authorization", adminAuth)
	reconcileResponse := httptest.NewRecorder()
	mux.ServeHTTP(reconcileResponse, reconcileRequest)
	if reconcileResponse.Code != http.StatusOK {
		t.Fatalf("expected deployment reconcile 200, got %d", reconcileResponse.Code)
	}

	teamServicesRequest := httptest.NewRequest(http.MethodGet, "/api/v2/team/services", nil)
	teamServicesRequest.Header.Set("Authorization", "Bearer dev-team-token")
	teamServicesResponse := httptest.NewRecorder()
	mux.ServeHTTP(teamServicesResponse, teamServicesRequest)
	if teamServicesResponse.Code != http.StatusOK {
		t.Fatalf("expected team services 200, got %d", teamServicesResponse.Code)
	}

	var teamServicesPayload struct {
		Status string         `json:"status"`
		Data   []serviceState `json:"data"`
	}
	if err := json.Unmarshal(teamServicesResponse.Body.Bytes(), &teamServicesPayload); err != nil {
		t.Fatalf("failed to decode team services response: %v", err)
	}

	expectedEndpoint := "10.80.77.11:31337"
	for _, service := range teamServicesPayload.Data {
		if service.ChallengeID == challengePayload.Data.ID {
			if service.Endpoint != expectedEndpoint {
				t.Fatalf("expected endpoint %s, got %s", expectedEndpoint, service.Endpoint)
			}
			return
		}
	}
	t.Fatalf("expected deployed service for challenge %d", challengePayload.Data.ID)
}

func TestAdminReconcileDeploymentsUsesControllerWhenConfigured(t *testing.T) {
	store := NewMemoryStore(101)
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "api-gateway", Version: "dev", Addr: ":0"})
	NewWithDeps(
		"dev-team-token",
		"dev-admin-token",
		101,
		store,
		testControllerClient{deployReconcileResult: adminReconcileResult{
			ProcessedJobs:      7,
			ProcessedInstances: 21,
			CompletedJobs:      3,
		}},
		noopWireGuardClient{},
	).RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodPost, "/api/v2/admin/deployments/reconcile", nil)
	request.Header.Set("Authorization", "Bearer dev-admin-token")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected reconcile 200, got %d", response.Code)
	}

	var payload struct {
		Status string               `json:"status"`
		Data   adminReconcileResult `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode reconcile response: %v", err)
	}
	if payload.Data.ProcessedJobs != 7 || payload.Data.ProcessedInstances != 21 || payload.Data.CompletedJobs != 3 {
		t.Fatalf("unexpected reconcile payload %+v", payload.Data)
	}
}

func TestAdminCanDeleteCompletedDeploymentJob(t *testing.T) {
	mux := newTestMux()
	adminAuth := "Bearer dev-admin-token"

	challengeRequest := httptest.NewRequest(http.MethodPost, "/api/v2/admin/challenges", bytes.NewBufferString(`{"name":"cleanup","baseline_image":"registry.local/cleanup:baseline","checker_image":"registry.local/cleanup-checker:latest","weight":1}`))
	challengeRequest.Header.Set("Authorization", adminAuth)
	challengeResponse := httptest.NewRecorder()
	mux.ServeHTTP(challengeResponse, challengeRequest)
	if challengeResponse.Code != http.StatusOK {
		t.Fatalf("expected challenge create 200, got %d", challengeResponse.Code)
	}

	var challengePayload struct {
		Status string         `json:"status"`
		Data   adminChallenge `json:"data"`
	}
	if err := json.Unmarshal(challengeResponse.Body.Bytes(), &challengePayload); err != nil {
		t.Fatalf("failed to decode challenge response: %v", err)
	}

	deployRequest := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v2/admin/challenges/%d/deploy", challengePayload.Data.ID), nil)
	deployRequest.Header.Set("Authorization", adminAuth)
	deployResponse := httptest.NewRecorder()
	mux.ServeHTTP(deployResponse, deployRequest)
	if deployResponse.Code != http.StatusOK {
		t.Fatalf("expected deploy 200, got %d", deployResponse.Code)
	}

	var deployPayload struct {
		Status string          `json:"status"`
		Data   adminDeployment `json:"data"`
	}
	if err := json.Unmarshal(deployResponse.Body.Bytes(), &deployPayload); err != nil {
		t.Fatalf("failed to decode deploy response: %v", err)
	}

	reconcileRequest := httptest.NewRequest(http.MethodPost, "/api/v2/admin/deployments/reconcile", nil)
	reconcileRequest.Header.Set("Authorization", adminAuth)
	reconcileResponse := httptest.NewRecorder()
	mux.ServeHTTP(reconcileResponse, reconcileRequest)
	if reconcileResponse.Code != http.StatusOK {
		t.Fatalf("expected reconcile 200, got %d", reconcileResponse.Code)
	}

	deleteRequest := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v2/admin/deployments/%d", deployPayload.Data.JobID), nil)
	deleteRequest.Header.Set("Authorization", adminAuth)
	deleteResponse := httptest.NewRecorder()
	mux.ServeHTTP(deleteResponse, deleteRequest)
	if deleteResponse.Code != http.StatusOK {
		t.Fatalf("expected deployment delete 200, got %d", deleteResponse.Code)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v2/admin/deployments", nil)
	listRequest.Header.Set("Authorization", adminAuth)
	listResponse := httptest.NewRecorder()
	mux.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("expected deployment list 200, got %d", listResponse.Code)
	}

	var listPayload struct {
		Status string               `json:"status"`
		Data   []adminDeploymentJob `json:"data"`
	}
	if err := json.Unmarshal(listResponse.Body.Bytes(), &listPayload); err != nil {
		t.Fatalf("failed to decode deployment list: %v", err)
	}
	if len(listPayload.Data) != 0 {
		t.Fatalf("expected deleted deployment job to be removed, got %+v", listPayload.Data)
	}
}

func TestAdminRedeploySupersedesOlderQueuedJob(t *testing.T) {
	mux := newTestMux()
	adminAuth := "Bearer dev-admin-token"

	challengeRequest := httptest.NewRequest(http.MethodPost, "/api/v2/admin/challenges", bytes.NewBufferString(`{"name":"redeploy","baseline_image":"registry.local/redeploy:baseline","checker_image":"registry.local/redeploy-checker:latest","weight":1}`))
	challengeRequest.Header.Set("Authorization", adminAuth)
	challengeResponse := httptest.NewRecorder()
	mux.ServeHTTP(challengeResponse, challengeRequest)
	if challengeResponse.Code != http.StatusOK {
		t.Fatalf("expected challenge create 200, got %d", challengeResponse.Code)
	}

	var challengePayload struct {
		Status string         `json:"status"`
		Data   adminChallenge `json:"data"`
	}
	if err := json.Unmarshal(challengeResponse.Body.Bytes(), &challengePayload); err != nil {
		t.Fatalf("failed to decode challenge response: %v", err)
	}

	firstDeployRequest := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v2/admin/challenges/%d/deploy", challengePayload.Data.ID), nil)
	firstDeployRequest.Header.Set("Authorization", adminAuth)
	firstDeployResponse := httptest.NewRecorder()
	mux.ServeHTTP(firstDeployResponse, firstDeployRequest)
	if firstDeployResponse.Code != http.StatusOK {
		t.Fatalf("expected first deploy 200, got %d", firstDeployResponse.Code)
	}

	secondDeployRequest := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v2/admin/challenges/%d/deploy", challengePayload.Data.ID), nil)
	secondDeployRequest.Header.Set("Authorization", adminAuth)
	secondDeployResponse := httptest.NewRecorder()
	mux.ServeHTTP(secondDeployResponse, secondDeployRequest)
	if secondDeployResponse.Code != http.StatusOK {
		t.Fatalf("expected second deploy 200, got %d", secondDeployResponse.Code)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v2/admin/deployments", nil)
	listRequest.Header.Set("Authorization", adminAuth)
	listResponse := httptest.NewRecorder()
	mux.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("expected deployment list 200, got %d", listResponse.Code)
	}

	var listPayload struct {
		Status string               `json:"status"`
		Data   []adminDeploymentJob `json:"data"`
	}
	if err := json.Unmarshal(listResponse.Body.Bytes(), &listPayload); err != nil {
		t.Fatalf("failed to decode deployment list: %v", err)
	}
	if len(listPayload.Data) != 2 {
		t.Fatalf("expected two deployment jobs after redeploy, got %+v", listPayload.Data)
	}
	if listPayload.Data[0].Status != "queued" {
		t.Fatalf("expected newest deployment job to stay queued, got %+v", listPayload.Data[0])
	}
	if listPayload.Data[1].Status != "superseded" {
		t.Fatalf("expected older deployment job to be superseded, got %+v", listPayload.Data[1])
	}
	if listPayload.Data[1].QueuedTeamCount != 0 {
		t.Fatalf("expected superseded deployment job queue to be cleared, got %+v", listPayload.Data[1])
	}
	if listPayload.Data[1].CompletedAt == "" {
		t.Fatalf("expected superseded deployment job completion time, got %+v", listPayload.Data[1])
	}
}

func TestAdminCannotDeleteActiveDeploymentJob(t *testing.T) {
	mux := newTestMux()
	adminAuth := "Bearer dev-admin-token"

	challengeRequest := httptest.NewRequest(http.MethodPost, "/api/v2/admin/challenges", bytes.NewBufferString(`{"name":"active-job","baseline_image":"registry.local/active-job:baseline","checker_image":"registry.local/active-job-checker:latest","weight":1}`))
	challengeRequest.Header.Set("Authorization", adminAuth)
	challengeResponse := httptest.NewRecorder()
	mux.ServeHTTP(challengeResponse, challengeRequest)
	if challengeResponse.Code != http.StatusOK {
		t.Fatalf("expected challenge create 200, got %d", challengeResponse.Code)
	}

	var challengePayload struct {
		Status string         `json:"status"`
		Data   adminChallenge `json:"data"`
	}
	if err := json.Unmarshal(challengeResponse.Body.Bytes(), &challengePayload); err != nil {
		t.Fatalf("failed to decode challenge response: %v", err)
	}

	deployRequest := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v2/admin/challenges/%d/deploy", challengePayload.Data.ID), nil)
	deployRequest.Header.Set("Authorization", adminAuth)
	deployResponse := httptest.NewRecorder()
	mux.ServeHTTP(deployResponse, deployRequest)
	if deployResponse.Code != http.StatusOK {
		t.Fatalf("expected deploy 200, got %d", deployResponse.Code)
	}

	var deployPayload struct {
		Status string          `json:"status"`
		Data   adminDeployment `json:"data"`
	}
	if err := json.Unmarshal(deployResponse.Body.Bytes(), &deployPayload); err != nil {
		t.Fatalf("failed to decode deploy response: %v", err)
	}

	deleteRequest := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v2/admin/deployments/%d", deployPayload.Data.JobID), nil)
	deleteRequest.Header.Set("Authorization", adminAuth)
	deleteResponse := httptest.NewRecorder()
	mux.ServeHTTP(deleteResponse, deleteRequest)
	if deleteResponse.Code != http.StatusBadRequest {
		t.Fatalf("expected deployment delete 400, got %d", deleteResponse.Code)
	}

	if !strings.Contains(deleteResponse.Body.String(), "deployment job is still active") {
		t.Fatalf("unexpected deployment delete error: %s", deleteResponse.Body.String())
	}
}

func TestAdminCanValidateChallengeBeforeDeploy(t *testing.T) {
	mux := newTestMux()
	adminAuth := "Bearer dev-admin-token"

	request := httptest.NewRequest(http.MethodPost, "/api/v2/admin/challenges/1/validate", nil)
	request.Header.Set("Authorization", adminAuth)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected validate 200, got %d", response.Code)
	}

	var payload struct {
		Status string                    `json:"status"`
		Data   ChallengeValidationResult `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode validation response: %v", err)
	}
	if payload.Data.Status != "valid" || !payload.Data.BaselineSSHContractOK || !payload.Data.CheckerContractOK {
		t.Fatalf("unexpected validation result %+v", payload.Data)
	}
}

func TestAdminDeployRejectsInvalidChallengeRuntime(t *testing.T) {
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "api-gateway", Version: "dev", Addr: ":0"})
	store := NewMemoryStore(101)
	NewWithDeps("dev-team-token", "dev-admin-token", 101, store, testControllerClient{
		validationResult: ChallengeValidationResult{
			Status:                "invalid",
			BaselineSSHContractOK: false,
			CheckerContractOK:     false,
			Message:               "missing ssh daemon binary inside image",
		},
	}, noopWireGuardClient{}).RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodPost, "/api/v2/admin/challenges/1/deploy", nil)
	request.Header.Set("Authorization", "Bearer dev-admin-token")
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected deploy rejection 400, got %d", response.Code)
	}

	var payload struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode deploy rejection: %v", err)
	}
	if payload.Message != "missing ssh daemon binary inside image" {
		t.Fatalf("unexpected deploy rejection message %q", payload.Message)
	}
}

func TestAdminDeployRejectsInvalidCheckerRuntime(t *testing.T) {
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "api-gateway", Version: "dev", Addr: ":0"})
	store := NewMemoryStore(101)
	NewWithDeps("dev-team-token", "dev-admin-token", 101, store, testControllerClient{
		validationResult: ChallengeValidationResult{
			Status:                "invalid",
			BaselineSSHContractOK: true,
			CheckerContractOK:     false,
			Message:               "missing standard checker entrypoint inside image",
		},
	}, noopWireGuardClient{}).RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodPost, "/api/v2/admin/challenges/1/deploy", nil)
	request.Header.Set("Authorization", "Bearer dev-admin-token")
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected deploy rejection 400, got %d", response.Code)
	}

	var payload struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode deploy rejection: %v", err)
	}
	if payload.Message != "missing standard checker entrypoint inside image" {
		t.Fatalf("unexpected deploy rejection message %q", payload.Message)
	}
}

func TestPlayerJWTScopesRequestsToAuthenticatedTeam(t *testing.T) {
	mux := newTestMux()
	adminAuth := "Bearer dev-admin-token"

	teamRequest := httptest.NewRequest(http.MethodPost, "/api/v2/admin/teams", bytes.NewBufferString(`{"name":"Team Nova","contact_email":"team-nova@example.com"}`))
	teamRequest.Header.Set("Authorization", adminAuth)
	teamResponse := httptest.NewRecorder()
	mux.ServeHTTP(teamResponse, teamRequest)
	if teamResponse.Code != http.StatusOK {
		t.Fatalf("expected team create 200, got %d", teamResponse.Code)
	}

	var teamPayload struct {
		Status string    `json:"status"`
		Data   adminTeam `json:"data"`
	}
	if err := json.Unmarshal(teamResponse.Body.Bytes(), &teamPayload); err != nil {
		t.Fatalf("failed to decode team response: %v", err)
	}

	playerRequest := httptest.NewRequest(http.MethodPost, "/api/v2/admin/players", bytes.NewBufferString(fmt.Sprintf(`{"team_id":%d,"display_name":"Nova Captain","email":"nova.captain@example.com","password":"nova-secret","role":"captain"}`, teamPayload.Data.ID)))
	playerRequest.Header.Set("Authorization", adminAuth)
	playerResponse := httptest.NewRecorder()
	mux.ServeHTTP(playerResponse, playerRequest)
	if playerResponse.Code != http.StatusOK {
		t.Fatalf("expected player create 200, got %d", playerResponse.Code)
	}

	authRequest := httptest.NewRequest(http.MethodPost, "/api/v2/authenticate", bytes.NewBufferString(`{"email":"nova.captain@example.com","password":"nova-secret"}`))
	authResponse := httptest.NewRecorder()
	mux.ServeHTTP(authResponse, authRequest)
	if authResponse.Code != http.StatusOK {
		t.Fatalf("expected auth 200, got %d", authResponse.Code)
	}

	var authPayload struct {
		Status string `json:"status"`
		Data   string `json:"data"`
	}
	if err := json.Unmarshal(authResponse.Body.Bytes(), &authPayload); err != nil {
		t.Fatalf("failed to decode auth response: %v", err)
	}

	servicesRequest := httptest.NewRequest(http.MethodGet, "/api/v2/team/services", nil)
	servicesRequest.Header.Set("Authorization", "Bearer "+authPayload.Data)
	servicesResponse := httptest.NewRecorder()
	mux.ServeHTTP(servicesResponse, servicesRequest)
	if servicesResponse.Code != http.StatusOK {
		t.Fatalf("expected services 200, got %d", servicesResponse.Code)
	}

	var servicesPayload struct {
		Status string         `json:"status"`
		Data   []serviceState `json:"data"`
	}
	if err := json.Unmarshal(servicesResponse.Body.Bytes(), &servicesPayload); err != nil {
		t.Fatalf("failed to decode services response: %v", err)
	}
	if len(servicesPayload.Data) == 0 {
		t.Fatal("expected team services for authenticated player")
	}
	for _, state := range servicesPayload.Data {
		if state.TeamID != teamPayload.Data.ID {
			t.Fatalf("expected team id %d, got %d", teamPayload.Data.ID, state.TeamID)
		}
	}
}

func TestAdminPlayerWireGuardLifecycle(t *testing.T) {
	mux := newTestMux()
	adminAuth := "Bearer dev-admin-token"

	createRequest := httptest.NewRequest(http.MethodPost, "/api/v2/admin/players", bytes.NewBufferString(`{"team_id":101,"display_name":"Alpha Member","email":"alpha.member@example.com","password":"alpha-member-secret","role":"member"}`))
	createRequest.Header.Set("Authorization", adminAuth)
	createResponse := httptest.NewRecorder()
	mux.ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusOK {
		t.Fatalf("expected player create 200, got %d", createResponse.Code)
	}

	var playerPayload struct {
		Status string      `json:"status"`
		Data   adminPlayer `json:"data"`
	}
	if err := json.Unmarshal(createResponse.Body.Bytes(), &playerPayload); err != nil {
		t.Fatalf("failed to decode player response: %v", err)
	}

	getRequest := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v2/admin/players/%d/wireguard", playerPayload.Data.ID), nil)
	getRequest.Header.Set("Authorization", adminAuth)
	getResponse := httptest.NewRecorder()
	mux.ServeHTTP(getResponse, getRequest)
	if getResponse.Code != http.StatusOK {
		t.Fatalf("expected wireguard get 200, got %d", getResponse.Code)
	}

	var getPayload struct {
		Status string             `json:"status"`
		Data   adminWireGuardPeer `json:"data"`
	}
	if err := json.Unmarshal(getResponse.Body.Bytes(), &getPayload); err != nil {
		t.Fatalf("failed to decode wireguard response: %v", err)
	}
	if getPayload.Data.Status != "active" {
		t.Fatalf("expected active peer, got %s", getPayload.Data.Status)
	}
	if !bytes.Contains([]byte(getPayload.Data.Config), []byte("[Interface]")) {
		t.Fatalf("expected wireguard config body, got %q", getPayload.Data.Config)
	}

	revokeRequest := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v2/admin/players/%d/wireguard/revoke", playerPayload.Data.ID), nil)
	revokeRequest.Header.Set("Authorization", adminAuth)
	revokeResponse := httptest.NewRecorder()
	mux.ServeHTTP(revokeResponse, revokeRequest)
	if revokeResponse.Code != http.StatusOK {
		t.Fatalf("expected revoke 200, got %d", revokeResponse.Code)
	}

	var revokePayload struct {
		Status string             `json:"status"`
		Data   adminWireGuardPeer `json:"data"`
	}
	if err := json.Unmarshal(revokeResponse.Body.Bytes(), &revokePayload); err != nil {
		t.Fatalf("failed to decode revoke response: %v", err)
	}
	if revokePayload.Data.Status != "revoked" || revokePayload.Data.RevokedAt == "" {
		t.Fatalf("expected revoked peer with timestamp, got %+v", revokePayload.Data)
	}

	rotateRequest := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v2/admin/players/%d/wireguard/rotate", playerPayload.Data.ID), nil)
	rotateRequest.Header.Set("Authorization", adminAuth)
	rotateResponse := httptest.NewRecorder()
	mux.ServeHTTP(rotateResponse, rotateRequest)
	if rotateResponse.Code != http.StatusOK {
		t.Fatalf("expected rotate 200, got %d", rotateResponse.Code)
	}

	var rotatePayload struct {
		Status string             `json:"status"`
		Data   adminWireGuardPeer `json:"data"`
	}
	if err := json.Unmarshal(rotateResponse.Body.Bytes(), &rotatePayload); err != nil {
		t.Fatalf("failed to decode rotate response: %v", err)
	}
	if rotatePayload.Data.Status != "active" || rotatePayload.Data.RevokedAt != "" {
		t.Fatalf("expected active peer after rotate, got %+v", rotatePayload.Data)
	}
	if rotatePayload.Data.Config == getPayload.Data.Config {
		t.Fatal("expected rotate to issue a new config")
	}
}

func TestAdminWireGuardGatewayStatusDisabledByDefault(t *testing.T) {
	mux := http.NewServeMux()
	New("dev-team-token", "dev-admin-token", 101).RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodGet, "/api/v2/admin/wireguard/status", nil)
	request.Header.Set("Authorization", "Bearer dev-admin-token")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", response.Code)
	}

	var payload struct {
		Status string                 `json:"status"`
		Data   WireGuardGatewayStatus `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode status response: %v", err)
	}
	if payload.Data.State != "disabled" || payload.Data.Mode != "disabled" {
		t.Fatalf("unexpected gateway status %+v", payload.Data)
	}
}

func TestAdminWireGuardGatewayReconcileDisabledByDefault(t *testing.T) {
	mux := http.NewServeMux()
	New("dev-team-token", "dev-admin-token", 101).RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodPost, "/api/v2/admin/wireguard/reconcile", nil)
	request.Header.Set("Authorization", "Bearer dev-admin-token")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected reconcile 503, got %d", response.Code)
	}
}

func TestAdminOperationsStatusHealthyWhenRuntimeMatchesStore(t *testing.T) {
	mux := newTestMuxWithOps(
		testGameCoreClient{
			status: GameStatus{
				Match: &GameMatchStatus{State: "stopped", AcceptingSubmissions: false},
				Scheduler: &GameSchedulerStatus{State: "stopped", IntervalSeconds: 60},
			},
		},
		testControllerClient{
			accessStatus: ControllerAccessStatus{
				State:             "applied",
				Mode:              "host",
				PoliciesTotal:     12,
				SSHOpenServices:   0,
				SSHLockedServices: 12,
				AllowedPeersTotal: 0,
			},
		},
		testWireGuardClient{
			status: WireGuardGatewayStatus{
				State:       "applied",
				Mode:        "host",
				PeersTotal:  4,
				PeersActive: 4,
			},
		},
	)

	request := httptest.NewRequest(http.MethodGet, "/api/v2/admin/operations/status", nil)
	request.Header.Set("Authorization", "Bearer dev-admin-token")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", response.Code)
	}

	var payload struct {
		Status string                `json:"status"`
		Data   AdminOperationsStatus `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode operations response: %v", err)
	}
	if !payload.Data.Healthy {
		t.Fatalf("expected healthy operations status, got %+v", payload.Data)
	}
	if len(payload.Data.Alerts) != 0 {
		t.Fatalf("expected no alerts, got %+v", payload.Data.Alerts)
	}
}

func TestAdminOperationsStatusFlagsOperationalDrift(t *testing.T) {
	now := time.Date(2026, 3, 27, 15, 30, 0, 0, time.UTC)
	store := NewMemoryStore(101).(*memoryStore)
	store.deployments[91] = &adminDeploymentJob{
		ID:              91,
		ChallengeID:     1,
		ChallengeName:   "banking",
		Status:          "queued",
		TargetTeamCount: 4,
		QueuedTeamCount: 4,
		ReadyTeamCount:  0,
		FailedTeamCount: 0,
		CreatedAt:       "2026-03-27T15:00:00Z",
	}
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "api-gateway", Version: "dev", Addr: ":0"})
	server := NewWithDeps(
		"dev-team-token",
		"dev-admin-token",
		101,
		store,
		testControllerClient{
			accessStatus: ControllerAccessStatus{
				State:             "applied",
				Mode:              "host",
				PoliciesTotal:     2,
				SSHOpenServices:   1,
				SSHLockedServices: 1,
				AllowedPeersTotal: 1,
				LastError:         "iptables apply failed",
			},
		},
		testWireGuardClient{
			status: WireGuardGatewayStatus{
				State:       "applied",
				Mode:        "host",
				PeersTotal:  1,
				PeersActive: 1,
				LastError:   "wg sync failed",
			},
		},
		testGameCoreClient{
			status: GameStatus{
				Match: &GameMatchStatus{
					State:                "running",
					StartedAt:            "2026-03-27T15:00:00Z",
					AcceptingSubmissions: true,
				},
				CurrentTick: &GameTickStatus{
					ID:        9,
					Status:    "running",
					StartedAt: "2026-03-27T15:20:00Z",
				},
				Scheduler: &GameSchedulerStatus{
					State:           "running",
					IntervalSeconds: 30,
					NextRunAt:       "2026-03-27T15:22:00Z",
					LastError:       "tick 8 failed",
				},
			},
		},
	)
	server.now = func() time.Time { return now }
	server.RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodGet, "/api/v2/admin/operations/status", nil)
	request.Header.Set("Authorization", "Bearer dev-admin-token")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", response.Code)
	}

	var payload struct {
		Status string                `json:"status"`
		Data   AdminOperationsStatus `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode operations response: %v", err)
	}
	if payload.Data.Healthy {
		t.Fatalf("expected degraded operations status, got %+v", payload.Data)
	}

	alertIDs := make(map[string]bool, len(payload.Data.Alerts))
	for _, alert := range payload.Data.Alerts {
		alertIDs[alert.ID] = true
	}
	for _, required := range []string{
		"scheduler-overdue",
		"checker-stalled",
		"scheduler-last-error",
		"deployment-queue-stalled",
		"wireguard-peer-drift",
		"wireguard-last-error",
		"access-policy-drift",
		"access-last-error",
	} {
		if !alertIDs[required] {
			t.Fatalf("expected alert %q in %+v", required, payload.Data.Alerts)
		}
	}
}

func TestEvaluateGameOperationsAlertsSchedulerOverdueBoundary(t *testing.T) {
	now := time.Date(2026, 3, 27, 15, 30, 0, 0, time.UTC)
	status := GameStatus{
		Match: &GameMatchStatus{
			State:                "running",
			StartedAt:            "2026-03-27T15:00:00Z",
			AcceptingSubmissions: true,
		},
		Scheduler: &GameSchedulerStatus{
			State:           "running",
			IntervalSeconds: 30,
			NextRunAt:       "2026-03-27T15:29:30Z",
		},
	}

	alerts := evaluateGameOperationsAlerts(status, now)
	for _, alert := range alerts {
		if alert.ID == "scheduler-overdue" {
			t.Fatalf("did not expect scheduler-overdue alert at grace boundary: %+v", alerts)
		}
	}

	alerts = evaluateGameOperationsAlerts(status, now.Add(time.Second))
	found := false
	for _, alert := range alerts {
		if alert.ID == "scheduler-overdue" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected scheduler-overdue alert once grace boundary is exceeded, got %+v", alerts)
	}
}

func TestAdminGameCoreRoutesProxyInternalState(t *testing.T) {
	mux := newTestMuxWithGameCore(testGameCoreClient{})
	adminAuth := "Bearer dev-admin-token"

	matchRequest := httptest.NewRequest(http.MethodGet, "/api/v2/admin/game/match", nil)
	matchRequest.Header.Set("Authorization", adminAuth)
	matchResponse := httptest.NewRecorder()
	mux.ServeHTTP(matchResponse, matchRequest)
	if matchResponse.Code != http.StatusOK {
		t.Fatalf("expected match status 200, got %d", matchResponse.Code)
	}

	var matchPayload struct {
		Status string          `json:"status"`
		Data   GameMatchStatus `json:"data"`
	}
	if err := json.Unmarshal(matchResponse.Body.Bytes(), &matchPayload); err != nil {
		t.Fatalf("failed to decode match status response: %v", err)
	}
	if matchPayload.Data.State != "running" || !matchPayload.Data.AcceptingSubmissions {
		t.Fatalf("unexpected match payload %+v", matchPayload.Data)
	}

	updateMatchScheduleRequest := httptest.NewRequest(
		http.MethodPut,
		"/api/v2/admin/game/match/schedule",
		bytes.NewBufferString(`{"scheduled_start_at":"2026-03-10T11:00:00Z","scheduled_end_at":"2026-03-10T13:00:00Z"}`),
	)
	updateMatchScheduleRequest.Header.Set("Authorization", adminAuth)
	updateMatchScheduleRequest.Header.Set("Content-Type", "application/json")
	updateMatchScheduleResponse := httptest.NewRecorder()
	mux.ServeHTTP(updateMatchScheduleResponse, updateMatchScheduleRequest)
	if updateMatchScheduleResponse.Code != http.StatusOK {
		t.Fatalf("expected match schedule update 200, got %d", updateMatchScheduleResponse.Code)
	}

	var updateMatchSchedulePayload struct {
		Status string          `json:"status"`
		Data   GameMatchStatus `json:"data"`
	}
	if err := json.Unmarshal(updateMatchScheduleResponse.Body.Bytes(), &updateMatchSchedulePayload); err != nil {
		t.Fatalf("failed to decode match schedule update response: %v", err)
	}
	if updateMatchSchedulePayload.Data.ScheduledStartAt != "2026-03-10T11:00:00Z" || updateMatchSchedulePayload.Data.ScheduledEndAt != "2026-03-10T13:00:00Z" {
		t.Fatalf("unexpected updated match schedule payload %+v", updateMatchSchedulePayload.Data)
	}

	statusRequest := httptest.NewRequest(http.MethodGet, "/api/v2/admin/game/status", nil)
	statusRequest.Header.Set("Authorization", adminAuth)
	statusResponse := httptest.NewRecorder()
	mux.ServeHTTP(statusResponse, statusRequest)
	if statusResponse.Code != http.StatusOK {
		t.Fatalf("expected game status 200, got %d", statusResponse.Code)
	}

	var statusPayload struct {
		Status string     `json:"status"`
		Data   GameStatus `json:"data"`
	}
	if err := json.Unmarshal(statusResponse.Body.Bytes(), &statusPayload); err != nil {
		t.Fatalf("failed to decode game status response: %v", err)
	}
	if statusPayload.Data.CurrentTick == nil || statusPayload.Data.CurrentTick.ID != 1 {
		t.Fatalf("unexpected current tick %+v", statusPayload.Data.CurrentTick)
	}

	publicStatusRequest := httptest.NewRequest(http.MethodGet, "/api/v2/game/status", nil)
	publicStatusResponse := httptest.NewRecorder()
	mux.ServeHTTP(publicStatusResponse, publicStatusRequest)
	if publicStatusResponse.Code != http.StatusOK {
		t.Fatalf("expected public game status 200, got %d", publicStatusResponse.Code)
	}

	var publicStatusPayload struct {
		Status string     `json:"status"`
		Data   GameStatus `json:"data"`
	}
	if err := json.Unmarshal(publicStatusResponse.Body.Bytes(), &publicStatusPayload); err != nil {
		t.Fatalf("failed to decode public game status response: %v", err)
	}
	if publicStatusPayload.Data.Match == nil || publicStatusPayload.Data.Match.State != "running" {
		t.Fatalf("unexpected public game match %+v", publicStatusPayload.Data.Match)
	}

	advanceRequest := httptest.NewRequest(http.MethodPost, "/api/v2/admin/game/ticks/advance", nil)
	advanceRequest.Header.Set("Authorization", adminAuth)
	advanceResponse := httptest.NewRecorder()
	mux.ServeHTTP(advanceResponse, advanceRequest)
	if advanceResponse.Code != http.StatusOK {
		t.Fatalf("expected advance 200, got %d", advanceResponse.Code)
	}

	var advancePayload struct {
		Status string         `json:"status"`
		Data   GameTickStatus `json:"data"`
	}
	if err := json.Unmarshal(advanceResponse.Body.Bytes(), &advancePayload); err != nil {
		t.Fatalf("failed to decode advance response: %v", err)
	}
	if advancePayload.Data.ID != 2 || advancePayload.Data.FailedCheckerRuns != 3 {
		t.Fatalf("unexpected advance payload %+v", advancePayload.Data)
	}

	runsRequest := httptest.NewRequest(http.MethodGet, "/api/v2/admin/game/checker-runs?limit=1", nil)
	runsRequest.Header.Set("Authorization", adminAuth)
	runsResponse := httptest.NewRecorder()
	mux.ServeHTTP(runsResponse, runsRequest)
	if runsResponse.Code != http.StatusOK {
		t.Fatalf("expected checker runs 200, got %d", runsResponse.Code)
	}

	var runsPayload struct {
		Status string             `json:"status"`
		Data   GameCheckerRunPage `json:"data"`
	}
	if err := json.Unmarshal(runsResponse.Body.Bytes(), &runsPayload); err != nil {
		t.Fatalf("failed to decode checker runs response: %v", err)
	}
	if len(runsPayload.Data.Items) != 1 || runsPayload.Data.Items[0].Phase != "put" {
		t.Fatalf("unexpected checker runs payload %+v", runsPayload.Data)
	}
	if runsPayload.Data.TotalCount != 2 || !runsPayload.Data.HasNext {
		t.Fatalf("unexpected checker run page metadata %+v", runsPayload.Data)
	}

	schedulerRequest := httptest.NewRequest(http.MethodGet, "/api/v2/admin/game/scheduler", nil)
	schedulerRequest.Header.Set("Authorization", adminAuth)
	schedulerResponse := httptest.NewRecorder()
	mux.ServeHTTP(schedulerResponse, schedulerRequest)
	if schedulerResponse.Code != http.StatusOK {
		t.Fatalf("expected scheduler status 200, got %d", schedulerResponse.Code)
	}

	var schedulerPayload struct {
		Status string              `json:"status"`
		Data   GameSchedulerStatus `json:"data"`
	}
	if err := json.Unmarshal(schedulerResponse.Body.Bytes(), &schedulerPayload); err != nil {
		t.Fatalf("failed to decode scheduler response: %v", err)
	}
	if schedulerPayload.Data.State != "stopped" {
		t.Fatalf("unexpected scheduler payload %+v", schedulerPayload.Data)
	}

	eventsRequest := httptest.NewRequest(http.MethodGet, "/api/v2/admin/game/scheduler/events?limit=1", nil)
	eventsRequest.Header.Set("Authorization", adminAuth)
	eventsResponse := httptest.NewRecorder()
	mux.ServeHTTP(eventsResponse, eventsRequest)
	if eventsResponse.Code != http.StatusOK {
		t.Fatalf("expected scheduler events 200, got %d", eventsResponse.Code)
	}

	var eventsPayload struct {
		Status string                 `json:"status"`
		Data   GameSchedulerEventPage `json:"data"`
	}
	if err := json.Unmarshal(eventsResponse.Body.Bytes(), &eventsPayload); err != nil {
		t.Fatalf("failed to decode scheduler events response: %v", err)
	}
	if len(eventsPayload.Data.Items) != 1 || eventsPayload.Data.Items[0].EventType != "tick_completed" {
		t.Fatalf("unexpected scheduler events payload %+v", eventsPayload.Data)
	}
	if eventsPayload.Data.TotalCount != 2 || !eventsPayload.Data.HasNext {
		t.Fatalf("unexpected scheduler event page metadata %+v", eventsPayload.Data)
	}

	startSchedulerRequest := httptest.NewRequest(http.MethodPost, "/api/v2/admin/game/scheduler/start", nil)
	startSchedulerRequest.Header.Set("Authorization", adminAuth)
	startSchedulerResponse := httptest.NewRecorder()
	mux.ServeHTTP(startSchedulerResponse, startSchedulerRequest)
	if startSchedulerResponse.Code != http.StatusOK {
		t.Fatalf("expected scheduler start 200, got %d", startSchedulerResponse.Code)
	}

	stopSchedulerRequest := httptest.NewRequest(http.MethodPost, "/api/v2/admin/game/scheduler/stop", nil)
	stopSchedulerRequest.Header.Set("Authorization", adminAuth)
	stopSchedulerResponse := httptest.NewRecorder()
	mux.ServeHTTP(stopSchedulerResponse, stopSchedulerRequest)
	if stopSchedulerResponse.Code != http.StatusOK {
		t.Fatalf("expected scheduler stop 200, got %d", stopSchedulerResponse.Code)
	}

	startMatchRequest := httptest.NewRequest(http.MethodPost, "/api/v2/admin/game/match/start", nil)
	startMatchRequest.Header.Set("Authorization", adminAuth)
	startMatchResponse := httptest.NewRecorder()
	mux.ServeHTTP(startMatchResponse, startMatchRequest)
	if startMatchResponse.Code != http.StatusOK {
		t.Fatalf("expected match start 200, got %d", startMatchResponse.Code)
	}

	stopMatchRequest := httptest.NewRequest(http.MethodPost, "/api/v2/admin/game/match/stop", nil)
	stopMatchRequest.Header.Set("Authorization", adminAuth)
	stopMatchResponse := httptest.NewRecorder()
	mux.ServeHTTP(stopMatchResponse, stopMatchRequest)
	if stopMatchResponse.Code != http.StatusOK {
		t.Fatalf("expected match stop 200, got %d", stopMatchResponse.Code)
	}

	scoreboardRequest := httptest.NewRequest(http.MethodGet, "/api/v2/admin/game/scoreboard", nil)
	scoreboardRequest.Header.Set("Authorization", adminAuth)
	scoreboardResponse := httptest.NewRecorder()
	mux.ServeHTTP(scoreboardResponse, scoreboardRequest)
	if scoreboardResponse.Code != http.StatusOK {
		t.Fatalf("expected scoreboard 200, got %d", scoreboardResponse.Code)
	}

	var scoreboardPayload struct {
		Status string          `json:"status"`
		Data   []ScoreRowAlias `json:"data"`
	}
	if err := json.Unmarshal(scoreboardResponse.Body.Bytes(), &scoreboardPayload); err != nil {
		t.Fatalf("failed to decode scoreboard response: %v", err)
	}
	if len(scoreboardPayload.Data) == 0 || scoreboardPayload.Data[0].Rank != 1 {
		t.Fatalf("unexpected scoreboard payload %+v", scoreboardPayload.Data)
	}

	recomputeRequest := httptest.NewRequest(http.MethodPost, "/api/v2/admin/game/scoring/recompute", nil)
	recomputeRequest.Header.Set("Authorization", adminAuth)
	recomputeResponse := httptest.NewRecorder()
	mux.ServeHTTP(recomputeResponse, recomputeRequest)
	if recomputeResponse.Code != http.StatusOK {
		t.Fatalf("expected score recompute 200, got %d", recomputeResponse.Code)
	}

	var recomputePayload struct {
		Status string          `json:"status"`
		Data   []ScoreRowAlias `json:"data"`
	}
	if err := json.Unmarshal(recomputeResponse.Body.Bytes(), &recomputePayload); err != nil {
		t.Fatalf("failed to decode score recompute response: %v", err)
	}
	if len(recomputePayload.Data) == 0 || recomputePayload.Data[0].Team == "" {
		t.Fatalf("unexpected recompute payload %+v", recomputePayload.Data)
	}
}

func TestAdminGameCoreRoutesApplyHistoryFiltersAndOffset(t *testing.T) {
	mux := newTestMuxWithGameCore(testGameCoreClient{
		runs: []GameCheckerRun{
			{ID: 31, TickID: 4, TeamID: 101, TeamName: "Team Alpha", ChallengeID: 2, ChallengeName: "chat", Phase: "check", Status: "skipped", CheckedAt: "2026-03-10T10:04:03Z"},
			{ID: 30, TickID: 4, TeamID: 101, TeamName: "Team Alpha", ChallengeID: 2, ChallengeName: "chat", Phase: "get", Status: "failed", CheckedAt: "2026-03-10T10:04:02Z"},
			{ID: 29, TickID: 3, TeamID: 101, TeamName: "Team Alpha", ChallengeID: 1, ChallengeName: "banking", Phase: "put", Status: "success", CheckedAt: "2026-03-10T10:03:01Z"},
			{ID: 28, TickID: 4, TeamID: 102, TeamName: "Team Delta", ChallengeID: 2, ChallengeName: "chat", Phase: "check", Status: "skipped", CheckedAt: "2026-03-10T10:04:01Z"},
		},
		events: []GameSchedulerEvent{
			{ID: 11, EventType: "tick_failed", Source: "scheduler", State: "running", TickID: 4, CreatedAt: "2026-03-10T10:04:11Z"},
			{ID: 10, EventType: "tick_completed", Source: "scheduler", State: "running", TickID: 3, CreatedAt: "2026-03-10T10:03:11Z"},
			{ID: 9, EventType: "started", Source: "organizer", State: "running", CreatedAt: "2026-03-10T10:03:00Z"},
		},
	})
	adminAuth := "Bearer dev-admin-token"

	runsRequest := httptest.NewRequest(http.MethodGet, "/api/v2/admin/game/checker-runs?team_id=101&challenge_id=2&status=skipped&limit=1", nil)
	runsRequest.Header.Set("Authorization", adminAuth)
	runsResponse := httptest.NewRecorder()
	mux.ServeHTTP(runsResponse, runsRequest)
	if runsResponse.Code != http.StatusOK {
		t.Fatalf("expected checker runs 200, got %d", runsResponse.Code)
	}

	var runsPayload struct {
		Status string             `json:"status"`
		Data   GameCheckerRunPage `json:"data"`
	}
	if err := json.Unmarshal(runsResponse.Body.Bytes(), &runsPayload); err != nil {
		t.Fatalf("failed to decode filtered checker runs response: %v", err)
	}
	if len(runsPayload.Data.Items) != 1 || runsPayload.Data.Items[0].ID != 31 {
		t.Fatalf("unexpected filtered checker runs payload %+v", runsPayload.Data)
	}
	if runsPayload.Data.TotalCount != 1 || runsPayload.Data.HasNext || runsPayload.Data.HasPrev {
		t.Fatalf("unexpected filtered checker run page metadata %+v", runsPayload.Data)
	}

	runsOffsetRequest := httptest.NewRequest(http.MethodGet, "/api/v2/admin/game/checker-runs?team_id=101&offset=1&limit=1", nil)
	runsOffsetRequest.Header.Set("Authorization", adminAuth)
	runsOffsetResponse := httptest.NewRecorder()
	mux.ServeHTTP(runsOffsetResponse, runsOffsetRequest)
	if runsOffsetResponse.Code != http.StatusOK {
		t.Fatalf("expected checker runs offset 200, got %d", runsOffsetResponse.Code)
	}
	if err := json.Unmarshal(runsOffsetResponse.Body.Bytes(), &runsPayload); err != nil {
		t.Fatalf("failed to decode offset checker runs response: %v", err)
	}
	if len(runsPayload.Data.Items) != 1 || runsPayload.Data.Items[0].ID != 30 {
		t.Fatalf("unexpected checker runs offset payload %+v", runsPayload.Data)
	}
	if !runsPayload.Data.HasPrev || !runsPayload.Data.HasNext || runsPayload.Data.TotalCount != 3 {
		t.Fatalf("unexpected checker runs offset metadata %+v", runsPayload.Data)
	}

	eventsRequest := httptest.NewRequest(http.MethodGet, "/api/v2/admin/game/scheduler/events?source=scheduler&state=running&event_type=tick_failed&limit=1", nil)
	eventsRequest.Header.Set("Authorization", adminAuth)
	eventsResponse := httptest.NewRecorder()
	mux.ServeHTTP(eventsResponse, eventsRequest)
	if eventsResponse.Code != http.StatusOK {
		t.Fatalf("expected filtered scheduler events 200, got %d", eventsResponse.Code)
	}

	var eventsPayload struct {
		Status string                 `json:"status"`
		Data   GameSchedulerEventPage `json:"data"`
	}
	if err := json.Unmarshal(eventsResponse.Body.Bytes(), &eventsPayload); err != nil {
		t.Fatalf("failed to decode filtered scheduler events response: %v", err)
	}
	if len(eventsPayload.Data.Items) != 1 || eventsPayload.Data.Items[0].ID != 11 {
		t.Fatalf("unexpected filtered scheduler events payload %+v", eventsPayload.Data)
	}
	if eventsPayload.Data.TotalCount != 1 || eventsPayload.Data.HasNext || eventsPayload.Data.HasPrev {
		t.Fatalf("unexpected filtered scheduler page metadata %+v", eventsPayload.Data)
	}

	eventsOffsetRequest := httptest.NewRequest(http.MethodGet, "/api/v2/admin/game/scheduler/events?source=scheduler&offset=1&limit=1", nil)
	eventsOffsetRequest.Header.Set("Authorization", adminAuth)
	eventsOffsetResponse := httptest.NewRecorder()
	mux.ServeHTTP(eventsOffsetResponse, eventsOffsetRequest)
	if eventsOffsetResponse.Code != http.StatusOK {
		t.Fatalf("expected scheduler events offset 200, got %d", eventsOffsetResponse.Code)
	}
	if err := json.Unmarshal(eventsOffsetResponse.Body.Bytes(), &eventsPayload); err != nil {
		t.Fatalf("failed to decode offset scheduler events response: %v", err)
	}
	if len(eventsPayload.Data.Items) != 1 || eventsPayload.Data.Items[0].ID != 10 {
		t.Fatalf("unexpected scheduler events offset payload %+v", eventsPayload.Data)
	}
	if !eventsPayload.Data.HasPrev || eventsPayload.Data.HasNext || eventsPayload.Data.TotalCount != 2 {
		t.Fatalf("unexpected scheduler events offset metadata %+v", eventsPayload.Data)
	}
}

func TestSubmitPrefersGameCoreWhenConfigured(t *testing.T) {
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "api-gateway", Version: "dev", Addr: ":0"})
	NewWithDeps(
		"dev-team-token",
		"dev-admin-token",
		101,
		NewMemoryStore(101),
		testControllerClient{},
		noopWireGuardClient{},
		testGameCoreClient{
			submit: []SubmissionVerdictAlias{
				{Flag: "FLAGv1.authoritative", Verdict: "flag is correct."},
				{Flag: "FLAGv1.dupe", Verdict: "flag already submitted."},
			},
		},
	).RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodPost, "/api/v2/submit", bytes.NewBufferString(`{"flags":["FLAGv1.authoritative","FLAGv1.dupe"]}`))
	request.Header.Set("Authorization", "Bearer dev-team-token")
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}

	var payload struct {
		Status string                   `json:"status"`
		Data   []SubmissionVerdictAlias `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode submit response: %v", err)
	}
	if len(payload.Data) != 2 || payload.Data[1].Verdict != "flag already submitted." {
		t.Fatalf("unexpected submit payload %+v", payload.Data)
	}
}

func TestSubmitRejectsWhenContestHasNotStarted(t *testing.T) {
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "api-gateway", Version: "dev", Addr: ":0"})
	NewWithDeps(
		"dev-team-token",
		"dev-admin-token",
		101,
		NewMemoryStore(101),
		testControllerClient{},
		noopWireGuardClient{},
		testGameCoreClient{
			status: GameStatus{
				Match: &GameMatchStatus{
					State:                "not_started",
					AcceptingSubmissions: false,
				},
			},
		},
	).RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodPost, "/api/v2/submit", bytes.NewBufferString(`{"flags":["FLAGv1.authoritative"]}`))
	request.Header.Set("Authorization", "Bearer dev-team-token")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", response.Code)
	}

	var payload struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode pre-start submit response: %v", err)
	}
	if payload.Message != "contest has not started yet." {
		t.Fatalf("unexpected message %q", payload.Message)
	}
}

func TestSubmitPrefersSubmissionServiceWhenConfigured(t *testing.T) {
	mux := newTestMuxWithWorkers(
		testGameCoreClient{
			submit: []SubmissionVerdictAlias{
				{Flag: "FLAGv1.core", Verdict: "flag is correct."},
			},
		},
		testSubmissionClient{
			submit: []SubmissionVerdictAlias{
				{Flag: "FLAGv1.worker", Verdict: "submission-service accepted the flag."},
			},
		},
		noopScoringClient{},
	)

	request := httptest.NewRequest(http.MethodPost, "/api/v2/submit", bytes.NewBufferString(`{"flags":["FLAGv1.worker"]}`))
	request.Header.Set("Authorization", "Bearer dev-team-token")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}

	var payload struct {
		Status string                   `json:"status"`
		Data   []SubmissionVerdictAlias `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode submit response: %v", err)
	}
	if len(payload.Data) != 1 || payload.Data[0].Verdict != "submission-service accepted the flag." {
		t.Fatalf("unexpected submit payload %+v", payload.Data)
	}
}

func TestSubmitRejectsWhenContestIsOver(t *testing.T) {
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "api-gateway", Version: "dev", Addr: ":0"})
	NewWithDeps(
		"dev-team-token",
		"dev-admin-token",
		101,
		NewMemoryStore(101),
		testControllerClient{},
		noopWireGuardClient{},
		testGameCoreClient{
			status: GameStatus{
				Match: &GameMatchStatus{
					State:                "finished",
					StartedAt:            "2026-03-10T10:00:00Z",
					EndedAt:              "2026-03-10T12:00:00Z",
					AcceptingSubmissions: false,
				},
			},
		},
	).RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodPost, "/api/v2/submit", bytes.NewBufferString(`{"flags":["FLAGv1.authoritative"]}`))
	request.Header.Set("Authorization", "Bearer dev-team-token")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", response.Code)
	}

	var payload struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode finished submit response: %v", err)
	}
	if payload.Message != "contest is over." {
		t.Fatalf("unexpected message %q", payload.Message)
	}
}
