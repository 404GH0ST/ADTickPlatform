package apigateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
	factoryResetErr       error
	validationErr         error
	validationResult      ChallengeValidationResult
	accessStatus          ControllerAccessStatus
	accessErr             error
}

type storeBackedControllerClient struct {
	store Store
	now   func() time.Time
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

type recordingResetControllerClient struct {
	store Store
	token string
	err   error
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

func (c storeBackedControllerClient) ReconcileDeployments(ctx context.Context) (adminReconcileResult, error) {
	now := time.Now
	if c.now != nil {
		now = c.now
	}
	return c.store.ReconcileAdminDeployments(ctx, now())
}

func (storeBackedControllerClient) FactoryResetService(_ context.Context, _, _ int) error {
	return nil
}

func (storeBackedControllerClient) RestartService(_ context.Context, _, _ int) error {
	return nil
}

func (storeBackedControllerClient) ReconcileServiceAccess(_ context.Context, _, _ int) error {
	return nil
}

func (storeBackedControllerClient) ApplySSHCredential(_ context.Context, _, _ int, _ ControllerSSHCredential) error {
	return nil
}

func (storeBackedControllerClient) ValidateChallengeRuntime(_ context.Context, request ChallengeValidationRequest) (ChallengeValidationResult, error) {
	return ChallengeValidationResult{
		ChallengeID:            request.ChallengeID,
		Name:                   request.Name,
		BaselineImage:          request.BaselineImage,
		CheckerImage:           request.CheckerImage,
		Status:                 "valid",
		BaselineSSHContractOK:  true,
		CheckerContractOK:      true,
		ServiceStateContractOK: true,
		CheckedAt:              "2026-03-10T10:00:00Z",
		Message:                "challenge package validated by store-backed test controller",
	}, nil
}

func (storeBackedControllerClient) AccessStatus(_ context.Context) (ControllerAccessStatus, error) {
	return ControllerAccessStatus{State: "applied", Mode: "dry-run"}, nil
}

func (storeBackedControllerClient) ReconcileAccessPolicies(_ context.Context) (ControllerAccessStatus, error) {
	return ControllerAccessStatus{State: "applied", Mode: "dry-run"}, nil
}

func (storeBackedControllerClient) TeardownAccessPolicies(_ context.Context) error {
	return nil
}

func (storeBackedControllerClient) RemoveService(_ context.Context, _, _ int) error {
	return nil
}

func (storeBackedControllerClient) RemoveTeamServices(_ context.Context, _ int) error {
	return nil
}

func (storeBackedControllerClient) RemoveChallengeServices(_ context.Context, _ int) error {
	return nil
}

func (c testControllerClient) FactoryResetService(_ context.Context, _, _ int) error {
	return c.factoryResetErr
}

func (c testControllerClient) RestartService(_ context.Context, _, _ int) error {
	return nil
}

func (c *recordingResetControllerClient) ReconcileDeployments(_ context.Context) (adminReconcileResult, error) {
	return adminReconcileResult{}, errControllerDisabled
}

func (c *recordingResetControllerClient) FactoryResetService(ctx context.Context, teamID, challengeID int) error {
	task, err := c.store.GetControllerRuntimeTask(ctx, teamID, challengeID)
	if err != nil {
		return err
	}
	c.token = task.CheckerToken
	return c.err
}

func (*recordingResetControllerClient) RestartService(_ context.Context, _, _ int) error {
	return nil
}

func (*recordingResetControllerClient) ReconcileServiceAccess(_ context.Context, _, _ int) error {
	return nil
}

func (*recordingResetControllerClient) ApplySSHCredential(_ context.Context, _, _ int, _ ControllerSSHCredential) error {
	return nil
}

func (*recordingResetControllerClient) ValidateChallengeRuntime(_ context.Context, request ChallengeValidationRequest) (ChallengeValidationResult, error) {
	return ChallengeValidationResult{ChallengeID: request.ChallengeID, Name: request.Name, Status: "valid", BaselineSSHContractOK: true, CheckerContractOK: true, ServiceStateContractOK: true}, nil
}

func (*recordingResetControllerClient) ReconcileServiceAccessPolicies(_ context.Context) error {
	return nil
}

func (*recordingResetControllerClient) ReconcileAccessPolicies(_ context.Context) (ControllerAccessStatus, error) {
	return ControllerAccessStatus{State: "ready"}, nil
}

func (*recordingResetControllerClient) AccessStatus(_ context.Context) (ControllerAccessStatus, error) {
	return ControllerAccessStatus{State: "ready"}, nil
}

func (*recordingResetControllerClient) TeardownAccessPolicies(_ context.Context) error {
	return nil
}

func (*recordingResetControllerClient) RemoveService(_ context.Context, _, _ int) error {
	return nil
}

func (*recordingResetControllerClient) RemoveTeamServices(_ context.Context, _ int) error {
	return nil
}

func (*recordingResetControllerClient) RemoveChallengeServices(_ context.Context, _ int) error {
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
			ChallengeID:            request.ChallengeID,
			Name:                   request.Name,
			BaselineImage:          request.BaselineImage,
			CheckerImage:           request.CheckerImage,
			Status:                 "valid",
			BaselineSSHContractOK:  true,
			CheckerContractOK:      true,
			ServiceStateContractOK: true,
			CheckedAt:              "2026-03-10T10:00:00Z",
			Message:                "challenge package validated by test controller",
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

func (c testGameCoreClient) PauseMatch(_ context.Context) (GameMatchStatus, error) {
	if c.err != nil {
		return GameMatchStatus{}, c.err
	}
	match := c.match
	if match.State == "" {
		match = GameMatchStatus{
			State:                "paused",
			StartedAt:            "2026-03-10T10:00:00Z",
			AcceptingSubmissions: false,
		}
	} else {
		match.State = "paused"
		match.AcceptingSubmissions = false
	}
	return match, nil
}

func (c testGameCoreClient) ResumeMatch(_ context.Context) (GameMatchStatus, error) {
	if c.err != nil {
		return GameMatchStatus{}, c.err
	}
	match := c.match
	if match.State == "" {
		match = GameMatchStatus{
			State:                "running",
			StartedAt:            "2026-03-10T10:00:00Z",
			AcceptingSubmissions: true,
		}
	} else {
		match.State = "running"
		match.AcceptingSubmissions = true
	}
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
			result = append(result, submissionVerdict{Flag: flag, Status: "accepted", Detail: "flag is correct."})
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

func (c testGameCoreClient) AuditScoring(_ context.Context) (ScoringAuditAlias, error) {
	if c.err != nil {
		return ScoringAuditAlias{}, c.err
	}
	return ScoringAuditAlias{Status: "ok", StoredRows: 2, ReplayedRows: 2}, nil
}

func (c testGameCoreClient) FlagFormat(_ context.Context) (FlagFormatStatus, error) {
	if c.err != nil {
		return FlagFormatStatus{}, c.err
	}
	return FlagFormatStatus{Format: "PLAYIT"}, nil
}

func (c testGameCoreClient) RefreshFlagFormat(_ context.Context, prefix string) (FlagFormatStatus, error) {
	if c.err != nil {
		return FlagFormatStatus{}, c.err
	}
	format := strings.TrimSpace(prefix)
	if format == "" {
		format = "PLAYIT"
	}
	return FlagFormatStatus{Format: format}, nil
}

func (c testSubmissionClient) SubmitFlags(_ context.Context, _ int, flags []string) ([]submissionVerdict, error) {
	if c.err != nil {
		return nil, c.err
	}
	if len(c.submit) == 0 {
		result := make([]submissionVerdict, 0, len(flags))
		for _, flag := range flags {
			result = append(result, submissionVerdict{Flag: flag, Status: "accepted", Detail: "submission-service accepted the flag."})
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

func (c testScoringClient) AuditScoring(_ context.Context) (ScoringAuditAlias, error) {
	if c.err != nil {
		return ScoringAuditAlias{}, c.err
	}
	return ScoringAuditAlias{Status: "ok", StoredRows: 1, ReplayedRows: 1}, nil
}

func newTestMux() *http.ServeMux {
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "api-gateway", Version: "dev", Addr: ":0"})
	store := NewMemoryStore(101)
	NewWithDeps("dev-team-token", "dev-admin-token", 101, store, storeBackedControllerClient{store: store}, noopWireGuardClient{}).RegisterRoutes(mux)
	return mux
}

func newTestMuxWithGameCore(game gameCoreClient) *http.ServeMux {
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "api-gateway", Version: "dev", Addr: ":0"})
	store := NewMemoryStore(101)
	NewWithDeps("dev-team-token", "dev-admin-token", 101, store, storeBackedControllerClient{store: store}, noopWireGuardClient{}, game).RegisterRoutes(mux)
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

func findServiceStateForTest(t *testing.T, services []serviceState, challengeID int) serviceState {
	t.Helper()
	for _, service := range services {
		if service.ChallengeID == challengeID {
			return service
		}
	}
	t.Fatalf("service state for challenge %d not found", challengeID)
	return serviceState{}
}

func testTeamBearerToken(t *testing.T, teamID int) string {
	t.Helper()

	player := authenticatedPlayer{
		PlayerID:    1,
		TeamID:      101,
		TeamName:    "Team Alpha",
		DisplayName: "Alpha Captain",
		Email:       "alpha.captain@example.com",
		Role:        "captain",
	}
	if teamID != 101 {
		player = authenticatedPlayer{
			PlayerID:    teamID,
			TeamID:      teamID,
			TeamName:    fmt.Sprintf("Team %d", teamID),
			DisplayName: fmt.Sprintf("Captain %d", teamID),
			Email:       fmt.Sprintf("captain.%d@example.com", teamID),
			Role:        "captain",
		}
	}
	token, err := issueTeamJWT("dev-team-token", player, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("issue team jwt: %v", err)
	}
	return "Bearer " + token
}

func setTestTeamAuthHeader(t *testing.T, request *http.Request) {
	t.Helper()
	request.Header.Set("Authorization", testTeamBearerToken(t, 101))
}

func setTestAdminAuthHeader(request *http.Request) {
	request.Header.Set("Authorization", "Bearer dev-admin-token")
}

func decodeCompat[T any](t *testing.T, body []byte) T {
	t.Helper()

	var value T
	if err := json.Unmarshal(body, &value); err == nil {
		return value
	}

	t.Fatalf("failed to decode response body %s", string(body))
	var zero T
	return zero
}

func decodeProblemCompat(t *testing.T, body []byte) httpapi.ProblemDetails {
	t.Helper()

	var problem httpapi.ProblemDetails
	if err := json.Unmarshal(body, &problem); err == nil && (problem.Detail != "" || problem.Title != "") {
		return problem
	}

	t.Fatalf("failed to decode problem body %s", string(body))
	return httpapi.ProblemDetails{}
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
		Token     string `json:"token"`
		TokenType string `json:"token_type"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode auth response: %v", err)
	}
	if payload.TokenType != "Bearer" {
		t.Fatalf("expected bearer token type, got %q", payload.TokenType)
	}
	if payload.Token == "" || bytes.Count([]byte(payload.Token), []byte(".")) != 2 {
		t.Fatalf("expected jwt-like token, got %q", payload.Token)
	}
}

func TestDeletedPlayerTokenIsRejected(t *testing.T) {
	store := NewMemoryStore(101)
	player, err := store.AuthenticatePlayer(context.Background(), "alpha.captain@example.com", "alpha-secret")
	if err != nil {
		t.Fatalf("authenticate player: %v", err)
	}
	token, err := issueTeamJWT("dev-team-token", player, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("issue jwt: %v", err)
	}
	if err := store.DeleteAdminPlayer(context.Background(), player.PlayerID); err != nil {
		t.Fatalf("delete player: %v", err)
	}

	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "api-gateway", Version: "dev", Addr: ":0"})
	NewWithDeps("dev-team-token", "dev-admin-token", 101, store, testControllerClient{}, noopWireGuardClient{}).RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodGet, "/api/v2/team/services", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", response.Code)
	}
	problem := decodeProblemCompat(t, response.Body.Bytes())
	if problem.Detail != "please authenticate before access." {
		t.Fatalf("unexpected detail: %q", problem.Detail)
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

	payload := decodeProblemCompat(t, response.Body.Bytes())
	if payload.Status != http.StatusTooManyRequests {
		t.Fatalf("unexpected status %d", payload.Status)
	}
	if payload.Title != "Too many requests" {
		t.Fatalf("unexpected title %q", payload.Title)
	}
	if payload.Detail != defaultRateLimit429Message {
		t.Fatalf("unexpected rate-limit message %q", payload.Detail)
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

func TestSubmitRejectsRawTeamSecretBearer(t *testing.T) {
	mux := newTestMux()
	body := bytes.NewBufferString(`{"flags":["FLAGv1.demo"]}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v2/submit", body)
	request.Header.Set("Authorization", "Bearer dev-team-token")
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", response.Code)
	}
}

func TestAdminTokenCannotSubmit(t *testing.T) {
	mux := newTestMux()
	body := bytes.NewBufferString(`{"flags":["FLAGv1.demo"]}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v2/submit", body)
	setTestAdminAuthHeader(request)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", response.Code)
	}
	problem := decodeProblemCompat(t, response.Body.Bytes())
	if problem.Detail != "please authenticate before submit." {
		t.Fatalf("unexpected detail %q", problem.Detail)
	}
}

func TestOrganizerTokenCannotSubmit(t *testing.T) {
	mux := newTestMux()
	token, err := issueTeamJWT("dev-team-token", authenticatedPlayer{
		PlayerID:    99,
		TeamID:      101,
		TeamName:    "Team Alpha",
		DisplayName: "Organizer",
		Email:       "organizer@example.com",
		Role:        "organizer",
	}, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("issue organizer token: %v", err)
	}

	body := bytes.NewBufferString(`{"flags":["FLAGv1.demo"]}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v2/submit", body)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", response.Code)
	}
	problem := decodeProblemCompat(t, response.Body.Bytes())
	if problem.Detail != "please authenticate before submit." {
		t.Fatalf("unexpected detail %q", problem.Detail)
	}
}

func TestDeletedParticipantTokenCannotSubmit(t *testing.T) {
	store := NewMemoryStore(101)
	player, err := store.AuthenticatePlayer(context.Background(), "alpha.captain@example.com", "alpha-secret")
	if err != nil {
		t.Fatalf("authenticate player: %v", err)
	}
	token, err := issueTeamJWT("dev-team-token", player, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("issue team token: %v", err)
	}
	if err := store.DeleteAdminPlayer(context.Background(), player.PlayerID); err != nil {
		t.Fatalf("delete player: %v", err)
	}

	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "api-gateway", Version: "dev", Addr: ":0"})
	NewWithDeps("dev-team-token", "dev-admin-token", 101, store, testControllerClient{}, noopWireGuardClient{}).RegisterRoutes(mux)

	body := bytes.NewBufferString(`{"flags":["FLAGv1.demo"]}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v2/submit", body)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", response.Code)
	}
	problem := decodeProblemCompat(t, response.Body.Bytes())
	if problem.Detail != "please authenticate before submit." {
		t.Fatalf("unexpected detail %q", problem.Detail)
	}
}

func TestParticipantTokenCannotAccessAdminRoutes(t *testing.T) {
	mux := newTestMux()
	request := httptest.NewRequest(http.MethodGet, "/api/v2/admin/teams", nil)
	setTestTeamAuthHeader(t, request)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), "please authenticate as organizer") {
		t.Fatalf("expected organizer auth failure, got %s", response.Body.String())
	}
}

func TestAdminTeamListIncludesJoinKeys(t *testing.T) {
	mux := newTestMux()
	request := httptest.NewRequest(http.MethodGet, "/api/v2/admin/teams", nil)
	request.Header.Set("Authorization", "Bearer dev-admin-token")
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected teams 200, got %d", response.Code)
	}
	payload := decodeCompat[[]adminTeam](t, response.Body.Bytes())
	if len(payload) == 0 || strings.TrimSpace(payload[0].JoinKey) == "" {
		t.Fatalf("expected seeded team join key, got %+v", payload)
	}
}

func TestParticipantCanJoinTeamWithKey(t *testing.T) {
	mux := newTestMux()

	body := `{"team_key":"TEAM-ALPHA-JOIN","display_name":"New Member","email":"new.member@example.com","password":"member-secret"}`
	joinRequest := httptest.NewRequest(http.MethodPost, "/api/v2/team/join", bytes.NewBufferString(body))
	joinResponse := httptest.NewRecorder()
	mux.ServeHTTP(joinResponse, joinRequest)
	if joinResponse.Code != http.StatusOK {
		t.Fatalf("expected join 200, got %d: %s", joinResponse.Code, joinResponse.Body.String())
	}
	authPayload := decodeCompat[authenticateResponse](t, joinResponse.Body.Bytes())
	if authPayload.Token == "" || authPayload.TokenType != "Bearer" {
		t.Fatalf("unexpected join auth payload %+v", authPayload)
	}

	loginRequest := httptest.NewRequest(http.MethodPost, "/api/v2/authenticate", bytes.NewBufferString(`{"email":"new.member@example.com","password":"member-secret"}`))
	loginResponse := httptest.NewRecorder()
	mux.ServeHTTP(loginResponse, loginRequest)
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("expected login 200 after join, got %d: %s", loginResponse.Code, loginResponse.Body.String())
	}
}

func TestParticipantJoinRejectsInvalidTeamKey(t *testing.T) {
	mux := newTestMux()

	body := `{"team_key":"wrong","display_name":"New Member","email":"bad.member@example.com","password":"member-secret"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v2/team/join", bytes.NewBufferString(body))
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("expected join 403, got %d: %s", response.Code, response.Body.String())
	}
}

func TestParticipantCanRegisterWithoutTeam(t *testing.T) {
	mux := newTestMux()

	body := `{"display_name":"Pending Player","email":"pending.player@example.com","password":"pending-secret"}`
	registerRequest := httptest.NewRequest(http.MethodPost, "/api/v2/register", bytes.NewBufferString(body))
	registerResponse := httptest.NewRecorder()
	mux.ServeHTTP(registerResponse, registerRequest)
	if registerResponse.Code != http.StatusOK {
		t.Fatalf("expected register 200, got %d: %s", registerResponse.Code, registerResponse.Body.String())
	}
	authPayload := decodeCompat[authenticateResponse](t, registerResponse.Body.Bytes())
	if authPayload.Token == "" || authPayload.TokenType != "Bearer" {
		t.Fatalf("unexpected register auth payload %+v", authPayload)
	}

	loginRequest := httptest.NewRequest(http.MethodPost, "/api/v2/authenticate", bytes.NewBufferString(`{"email":"pending.player@example.com","password":"pending-secret"}`))
	loginResponse := httptest.NewRecorder()
	mux.ServeHTTP(loginResponse, loginRequest)
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("expected login 200 after register, got %d: %s", loginResponse.Code, loginResponse.Body.String())
	}
}

func TestTeamlessParticipantCannotAccessEventOrVPN(t *testing.T) {
	mux := newTestMux()

	body := `{"display_name":"Pending Player","email":"pending.blocked@example.com","password":"pending-secret"}`
	registerRequest := httptest.NewRequest(http.MethodPost, "/api/v2/register", bytes.NewBufferString(body))
	registerResponse := httptest.NewRecorder()
	mux.ServeHTTP(registerResponse, registerRequest)
	if registerResponse.Code != http.StatusOK {
		t.Fatalf("expected register 200, got %d: %s", registerResponse.Code, registerResponse.Body.String())
	}
	authPayload := decodeCompat[authenticateResponse](t, registerResponse.Body.Bytes())

	eventRequest := httptest.NewRequest(http.MethodGet, "/api/v2/team/services", nil)
	eventRequest.Header.Set("Authorization", "Bearer "+authPayload.Token)
	eventResponse := httptest.NewRecorder()
	mux.ServeHTTP(eventResponse, eventRequest)
	if eventResponse.Code != http.StatusForbidden {
		t.Fatalf("expected team services 403, got %d: %s", eventResponse.Code, eventResponse.Body.String())
	}

	vpnRequest := httptest.NewRequest(http.MethodGet, "/api/v2/me/wireguard", nil)
	vpnRequest.Header.Set("Authorization", "Bearer "+authPayload.Token)
	vpnResponse := httptest.NewRecorder()
	mux.ServeHTTP(vpnResponse, vpnRequest)
	if vpnResponse.Code != http.StatusForbidden {
		t.Fatalf("expected vpn 403, got %d: %s", vpnResponse.Code, vpnResponse.Body.String())
	}
}

func TestRegisteredParticipantCanJoinExistingAccountToTeam(t *testing.T) {
	mux := newTestMux()

	body := `{"display_name":"Pending Player","email":"pending.join@example.com","password":"pending-secret"}`
	registerRequest := httptest.NewRequest(http.MethodPost, "/api/v2/register", bytes.NewBufferString(body))
	registerResponse := httptest.NewRecorder()
	mux.ServeHTTP(registerResponse, registerRequest)
	if registerResponse.Code != http.StatusOK {
		t.Fatalf("expected register 200, got %d: %s", registerResponse.Code, registerResponse.Body.String())
	}
	registerPayload := decodeCompat[authenticateResponse](t, registerResponse.Body.Bytes())

	joinRequest := httptest.NewRequest(http.MethodPost, "/api/v2/me/team", bytes.NewBufferString(`{"team_key":"TEAM-ALPHA-JOIN"}`))
	joinRequest.Header.Set("Authorization", "Bearer "+registerPayload.Token)
	joinResponse := httptest.NewRecorder()
	mux.ServeHTTP(joinResponse, joinRequest)
	if joinResponse.Code != http.StatusOK {
		t.Fatalf("expected existing join 200, got %d: %s", joinResponse.Code, joinResponse.Body.String())
	}
	joinPayload := decodeCompat[authenticateResponse](t, joinResponse.Body.Bytes())

	eventRequest := httptest.NewRequest(http.MethodGet, "/api/v2/team/services", nil)
	eventRequest.Header.Set("Authorization", "Bearer "+joinPayload.Token)
	eventResponse := httptest.NewRecorder()
	mux.ServeHTTP(eventResponse, eventRequest)
	if eventResponse.Code != http.StatusOK {
		t.Fatalf("expected team services 200 after join, got %d: %s", eventResponse.Code, eventResponse.Body.String())
	}

	vpnRequest := httptest.NewRequest(http.MethodGet, "/api/v2/me/wireguard", nil)
	vpnRequest.Header.Set("Authorization", "Bearer "+joinPayload.Token)
	vpnResponse := httptest.NewRecorder()
	mux.ServeHTTP(vpnResponse, vpnRequest)
	if vpnResponse.Code != http.StatusOK {
		t.Fatalf("expected vpn 200 after join, got %d: %s", vpnResponse.Code, vpnResponse.Body.String())
	}
}

func TestParticipantJoinRespectsMaxTeamMembers(t *testing.T) {
	store := NewMemoryStore(101)
	maxTeamMembers := 1
	if _, err := store.UpdatePlatformSettings(context.Background(), adminUpdatePlatformSettingsRequest{
		FlagFormatPrefix: "PLAYIT",
		MaxTeamMembers:   &maxTeamMembers,
	}, "test", time.Now().UTC()); err != nil {
		t.Fatalf("update platform settings: %v", err)
	}
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "api-gateway", Version: "dev", Addr: ":0"})
	NewWithDeps("dev-team-token", "dev-admin-token", 101, store, storeBackedControllerClient{store: store}, noopWireGuardClient{}).RegisterRoutes(mux)

	body := `{"team_key":"TEAM-ALPHA-JOIN","display_name":"Overflow Member","email":"overflow.member@example.com","password":"member-secret"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v2/team/join", bytes.NewBufferString(body))
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected join 400 for full team, got %d: %s", response.Code, response.Body.String())
	}
}

func TestAllAdminRoutesRejectUnauthenticatedAndParticipantCallers(t *testing.T) {
	mux := newTestMux()
	cases := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/v2/admin/teams", ""},
		{http.MethodPost, "/api/v2/admin/teams", `{}`},
		{http.MethodPut, "/api/v2/admin/teams/101", `{}`},
		{http.MethodDelete, "/api/v2/admin/teams/101", ""},
		{http.MethodGet, "/api/v2/admin/players", ""},
		{http.MethodPost, "/api/v2/admin/players", `{}`},
		{http.MethodPut, "/api/v2/admin/players/1001", `{}`},
		{http.MethodDelete, "/api/v2/admin/players/1001", ""},
		{http.MethodGet, "/api/v2/admin/players/1001/wireguard", ""},
		{http.MethodPost, "/api/v2/admin/players/1001/wireguard/rotate", `{}`},
		{http.MethodPost, "/api/v2/admin/players/1001/wireguard/revoke", `{}`},
		{http.MethodGet, "/api/v2/admin/wireguard/status", ""},
		{http.MethodPost, "/api/v2/admin/wireguard/reconcile", `{}`},
		{http.MethodPost, "/api/v2/admin/wireguard/teardown", `{}`},
		{http.MethodGet, "/api/v2/admin/access/status", ""},
		{http.MethodPost, "/api/v2/admin/access/reconcile", `{}`},
		{http.MethodPost, "/api/v2/admin/access/teardown", `{}`},
		{http.MethodGet, "/api/v2/admin/challenges", ""},
		{http.MethodPost, "/api/v2/admin/challenges", `{}`},
		{http.MethodPut, "/api/v2/admin/challenges/1", `{}`},
		{http.MethodDelete, "/api/v2/admin/challenges/1", ""},
		{http.MethodPost, "/api/v2/admin/challenges/1/validate", `{}`},
		{http.MethodPost, "/api/v2/admin/challenges/1/deploy", `{}`},
		{http.MethodGet, "/api/v2/admin/deployments", ""},
		{http.MethodDelete, "/api/v2/admin/deployments/77", ""},
		{http.MethodPost, "/api/v2/admin/deployments/reconcile", `{}`},
		{http.MethodGet, "/api/v2/admin/audit-logs", ""},
		{http.MethodGet, "/api/v2/admin/operations/status", ""},
		{http.MethodGet, "/api/v2/admin/game/status", ""},
		{http.MethodGet, "/api/v2/admin/game/match", ""},
		{http.MethodPost, "/api/v2/admin/game/match/start", `{}`},
		{http.MethodPost, "/api/v2/admin/game/match/stop", `{}`},
		{http.MethodPut, "/api/v2/admin/game/match/schedule", `{}`},
		{http.MethodPost, "/api/v2/admin/game/ticks/advance", `{}`},
		{http.MethodGet, "/api/v2/admin/game/checker-runs", ""},
		{http.MethodGet, "/api/v2/admin/game/scheduler", ""},
		{http.MethodGet, "/api/v2/admin/game/scheduler/events", ""},
		{http.MethodPost, "/api/v2/admin/game/scheduler/start", `{}`},
		{http.MethodPost, "/api/v2/admin/game/scheduler/stop", `{}`},
		{http.MethodPut, "/api/v2/admin/game/scheduler/interval", `{}`},
		{http.MethodGet, "/api/v2/admin/game/scoreboard", ""},
		{http.MethodPost, "/api/v2/admin/game/scoring/recompute", `{}`},
		{http.MethodGet, "/api/v2/admin/game/scoring/audit", ""},
	}

	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path+" unauthenticated", func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			response := httptest.NewRecorder()

			mux.ServeHTTP(response, request)

			if response.Code != http.StatusForbidden {
				t.Fatalf("expected 403, got %d: %s", response.Code, response.Body.String())
			}
		})

		t.Run(tc.method+" "+tc.path+" participant", func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			setTestTeamAuthHeader(t, request)
			response := httptest.NewRecorder()

			mux.ServeHTTP(response, request)

			if response.Code != http.StatusForbidden {
				t.Fatalf("expected 403, got %d: %s", response.Code, response.Body.String())
			}
			if !strings.Contains(response.Body.String(), "please authenticate as organizer") {
				t.Fatalf("expected organizer auth failure, got %s", response.Body.String())
			}
		})
	}
}

func TestAdminTokenCannotAccessParticipantRoutes(t *testing.T) {
	mux := newTestMux()
	request := httptest.NewRequest(http.MethodGet, "/api/v2/team/services", nil)
	setTestAdminAuthHeader(request)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), "please authenticate before access") {
		t.Fatalf("expected participant auth failure, got %s", response.Body.String())
	}
}

func TestTeamRoutesRejectUnauthenticatedAndAdminCallers(t *testing.T) {
	mux := newTestMux()
	cases := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/v2/challenges/1/source", ""},
		{http.MethodGet, "/api/v2/services", ""},
		{http.MethodGet, "/api/v2/team/services", ""},
		{http.MethodPost, "/api/v2/submit", `{"flags":["FLAGv1.demo"]}`},
		{http.MethodPost, "/api/v2/services/1/unlock", `{"proof":"bad"}`},
		{http.MethodPost, "/api/v2/services/1/ssh-session", `{}`},
		{http.MethodPost, "/api/v2/services/1/reset/factory", `{}`},
		{http.MethodPost, "/api/v2/services/1/reset/restart", `{}`},
	}

	token, err := issueTeamJWT("dev-team-token", authenticatedPlayer{
		PlayerID:    99,
		TeamID:      101,
		TeamName:    "Team Alpha",
		DisplayName: "Organizer",
		Email:       "organizer@example.com",
		Role:        "organizer",
	}, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("issue organizer token: %v", err)
	}

	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path+" unauthenticated", func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			response := httptest.NewRecorder()

			mux.ServeHTTP(response, request)

			if response.Code != http.StatusForbidden {
				t.Fatalf("expected 403, got %d: %s", response.Code, response.Body.String())
			}
		})

		t.Run(tc.method+" "+tc.path+" admin", func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			setTestAdminAuthHeader(request)
			response := httptest.NewRecorder()

			mux.ServeHTTP(response, request)

			if response.Code != http.StatusForbidden {
				t.Fatalf("expected 403, got %d: %s", response.Code, response.Body.String())
			}
		})

		t.Run(tc.method+" "+tc.path+" organizer", func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			request.Header.Set("Authorization", "Bearer "+token)
			response := httptest.NewRecorder()

			mux.ServeHTTP(response, request)

			if response.Code != http.StatusForbidden {
				t.Fatalf("expected 403, got %d: %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestOrganizerBearerTokenCannotAccessParticipantRoutes(t *testing.T) {
	mux := newTestMux()
	token, err := issueTeamJWT("dev-team-token", authenticatedPlayer{
		PlayerID:    99,
		TeamID:      101,
		TeamName:    "Team Alpha",
		DisplayName: "Organizer",
		Email:       "organizer@example.com",
		Role:        "organizer",
	}, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("issue organizer token: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/v2/team/services", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), "please authenticate before access") {
		t.Fatalf("expected participant auth failure, got %s", response.Body.String())
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
	setTestTeamAuthHeader(t, request)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", response.Code)
	}
	if got := response.Header().Get("Retry-After"); got != "3" {
		t.Fatalf("expected Retry-After 3, got %q", got)
	}

	payload := decodeProblemCompat(t, response.Body.Bytes())
	if payload.Status != http.StatusTooManyRequests {
		t.Fatalf("unexpected status %d", payload.Status)
	}
	if payload.Title != "Too many requests" {
		t.Fatalf("unexpected title %q", payload.Title)
	}
	if payload.Detail != defaultRateLimit429Message {
		t.Fatalf("unexpected rate-limit message %q", payload.Detail)
	}
}

func TestSubmitRejectsOversizedBatch(t *testing.T) {
	mux := newTestMux()
	flags := make([]string, 0, maxSubmitFlagsPerRequest+1)
	for i := 0; i < maxSubmitFlagsPerRequest+1; i++ {
		flags = append(flags, fmt.Sprintf("FLAGv1.oversized.%d", i))
	}
	bodyBytes, err := json.Marshal(submitRequest{Flags: flags})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/v2/submit", bytes.NewReader(bodyBytes))
	setTestTeamAuthHeader(t, request)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", response.Code)
	}
	problem := decodeProblemCompat(t, response.Body.Bytes())
	if problem.Detail != "too many flags in one request." {
		t.Fatalf("unexpected detail %q", problem.Detail)
	}
}

func TestSubmitReturnsDuplicateVerdict(t *testing.T) {
	mux := newTestMuxWithGameCore(testGameCoreClient{
		submit: []SubmissionVerdictAlias{
			{Flag: "FLAGv1.demo", Status: "accepted", Detail: "flag is correct."},
			{Flag: "FLAGv1.demo", Status: "duplicate", Detail: "flag already submitted."},
			{Flag: "bad", Status: "invalid", Detail: "flag is wrong or expired."},
		},
	})
	body := bytes.NewBufferString(`{"flags":["FLAGv1.demo","FLAGv1.demo","bad"]}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v2/submit", body)
	setTestTeamAuthHeader(t, request)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}

	var payload submissionResult
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(payload.Results) != 3 {
		t.Fatalf("expected 3 verdicts, got %d", len(payload.Results))
	}
	if payload.AcceptedCount != 1 || payload.RejectedCount != 2 {
		t.Fatalf("unexpected counts %+v", payload)
	}
	if payload.Results[0].Status != "accepted" || payload.Results[0].Detail != "flag is correct." {
		t.Fatalf("unexpected first verdict: %+v", payload.Results[0])
	}
	if payload.Results[1].Status != "duplicate" || payload.Results[1].Detail != "flag already submitted." {
		t.Fatalf("unexpected duplicate verdict: %+v", payload.Results[1])
	}
	if payload.Results[2].Status != "invalid" || payload.Results[2].Detail != "flag is wrong or expired." {
		t.Fatalf("unexpected invalid verdict: %+v", payload.Results[2])
	}
}

func TestSubmitReturnsServiceUnavailableWithoutAuthoritativeBackend(t *testing.T) {
	mux := newTestMux()
	body := bytes.NewBufferString(`{"flags":["FLAGv1.demo"]}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v2/submit", body)
	setTestTeamAuthHeader(t, request)
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

	var payload []scoreRow
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(payload) == 0 {
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
	setTestTeamAuthHeader(t, request)
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

	var payload []scoreRow
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode scoreboard response: %v", err)
	}
	if len(payload) != 1 || payload[0].Team != "Team Omega" {
		t.Fatalf("expected authoritative game-core scoreboard, got %+v", payload)
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

	var payload []scoreRow
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode scoreboard response: %v", err)
	}
	if len(payload) != 1 || payload[0].Team != "Team Worker" {
		t.Fatalf("expected scoring-worker scoreboard, got %+v", payload)
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

	var payload AttackFeedPage
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode attack feed response: %v", err)
	}
	if len(payload.Items) != 1 || payload.Items[0].ID != "atk-core-1" {
		t.Fatalf("expected authoritative game-core attack feed, got %+v", payload)
	}
	if payload.TotalCount != 1 || payload.HasNext || payload.HasPrev {
		t.Fatalf("unexpected attack feed page metadata %+v", payload)
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

	var payload AttackFeedPage
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode attack feed response: %v", err)
	}
	if len(payload.Items) != 1 || payload.Items[0].ID != "atk-sub-1" {
		t.Fatalf("expected submission-service attack feed, got %+v", payload)
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

	var payload AttackFeedPage
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode filtered attack feed response: %v", err)
	}
	if len(payload.Items) != 1 || payload.Items[0].ID != "atk-core-2" {
		t.Fatalf("expected filtered attack feed, got %+v", payload)
	}
	if payload.TotalCount != 1 || payload.HasNext || payload.HasPrev {
		t.Fatalf("unexpected filtered attack feed page metadata %+v", payload)
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

	var payload AttackFeedPage
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode tick-filtered attack feed response: %v", err)
	}
	if len(payload.Items) != 1 || payload.Items[0].ID != "atk-core-1" {
		t.Fatalf("expected tick-filtered attack feed, got %+v", payload)
	}
	if payload.TotalCount != 1 || payload.HasNext || payload.HasPrev {
		t.Fatalf("unexpected tick-filtered attack feed page metadata %+v", payload)
	}
}

func TestUnlockRejectsInvalidProof(t *testing.T) {
	mux := newTestMux()

	request := httptest.NewRequest(http.MethodPost, "/api/v2/services/2/unlock", bytes.NewBufferString(`{"proof":"wrong-proof"}`))
	setTestTeamAuthHeader(t, request)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", response.Code)
	}

	var payload httpapi.ProblemDetails
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode unlock response: %v", err)
	}
	if payload.Detail != "unlock proof is invalid." {
		t.Fatalf("unexpected unlock message %q", payload.Detail)
	}
}

func TestTeamServicesReflectUnlockAndResetState(t *testing.T) {
	mux := newTestMux()

	unlockRequest := httptest.NewRequest(http.MethodPost, "/api/v2/services/2/unlock", bytes.NewBufferString(`{"proof":"`+testUnlockProof(101, 2)+`"}`))
	setTestTeamAuthHeader(t, unlockRequest)
	unlockResponse := httptest.NewRecorder()
	mux.ServeHTTP(unlockResponse, unlockRequest)
	if unlockResponse.Code != http.StatusOK {
		t.Fatalf("expected unlock 200, got %d", unlockResponse.Code)
	}

	resetRequest := httptest.NewRequest(http.MethodPost, "/api/v2/services/2/reset/factory", nil)
	setTestTeamAuthHeader(t, resetRequest)
	resetResponse := httptest.NewRecorder()
	mux.ServeHTTP(resetResponse, resetRequest)
	if resetResponse.Code != http.StatusOK {
		t.Fatalf("expected reset 200, got %d", resetResponse.Code)
	}

	servicesRequest := httptest.NewRequest(http.MethodGet, "/api/v2/team/services", nil)
	setTestTeamAuthHeader(t, servicesRequest)
	servicesResponse := httptest.NewRecorder()
	mux.ServeHTTP(servicesResponse, servicesRequest)
	if servicesResponse.Code != http.StatusOK {
		t.Fatalf("expected services 200, got %d", servicesResponse.Code)
	}

	var payload []serviceState
	if err := json.Unmarshal(servicesResponse.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode services response: %v", err)
	}

	var chatState *serviceState
	for idx := range payload {
		if payload[idx].ChallengeID == 2 {
			chatState = &payload[idx]
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

func TestTeamServicesExposeLatestSLAFailureDetails(t *testing.T) {
	mux := newTestMuxWithGameCore(testGameCoreClient{
		runs: []GameCheckerRun{
			{ID: 33, TickID: 12, TeamID: 101, TeamName: "Team Alpha", ChallengeID: 1, ChallengeName: "college-http", Phase: "check", Status: "skipped", Message: "phase skipped after previous checker failure", CheckedAt: "2026-03-10T10:12:03Z"},
			{ID: 32, TickID: 12, TeamID: 101, TeamName: "Team Alpha", ChallengeID: 1, ChallengeName: "college-http", Phase: "get", Status: "failed", Message: "flag retrieval failed from service endpoint", CheckedAt: "2026-03-10T10:12:02Z"},
			{ID: 31, TickID: 12, TeamID: 101, TeamName: "Team Alpha", ChallengeID: 1, ChallengeName: "college-http", Phase: "put", Status: "success", CheckedAt: "2026-03-10T10:12:01Z"},
		},
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v2/team/services", nil)
	setTestTeamAuthHeader(t, request)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected services 200, got %d", response.Code)
	}

	var payload []serviceState
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode services response: %v", err)
	}

	for _, state := range payload {
		if state.ChallengeID != 1 {
			continue
		}
		if state.SLAStatus != "flag_not_found" {
			t.Fatalf("expected flag_not_found SLA status, got %q", state.SLAStatus)
		}
		if state.SLAPhase != "get" {
			t.Fatalf("expected get SLA phase, got %q", state.SLAPhase)
		}
		if state.SLATickID != 12 {
			t.Fatalf("expected SLA tick 12, got %d", state.SLATickID)
		}
		if state.SLAMessage != "flag retrieval failed from service endpoint" {
			t.Fatalf("unexpected SLA message %q", state.SLAMessage)
		}
		return
	}
	t.Fatal("expected service state for challenge 1")
}

func TestSummarizeSLARunsMapsFaustServiceStates(t *testing.T) {
	testCases := []struct {
		name    string
		runs    []GameCheckerRun
		status  string
		phase   string
		message string
	}{
		{
			name: "ok",
			runs: []GameCheckerRun{
				{Phase: "put", Status: "success"},
				{Phase: "get", Status: "success"},
				{Phase: "check", Status: "success"},
			},
			status:  "ok",
			phase:   "check",
			message: "service passed storage, retrieval, and functionality checks",
		},
		{
			name: "recovering",
			runs: []GameCheckerRun{
				{Phase: "put", Status: "failed", Message: "flag write failed"},
				{Phase: "get", Status: "success"},
				{Phase: "check", Status: "success"},
			},
			status:  "recovering",
			phase:   "put",
			message: "flag write failed",
		},
		{
			name: "flag not found",
			runs: []GameCheckerRun{
				{Phase: "put", Status: "success"},
				{Phase: "get", Status: "failed", Message: "flag retrieval failed"},
				{Phase: "check", Status: "success"},
			},
			status:  "flag_not_found",
			phase:   "get",
			message: "flag retrieval failed",
		},
		{
			name: "faulty",
			runs: []GameCheckerRun{
				{Phase: "put", Status: "success"},
				{Phase: "get", Status: "success"},
				{Phase: "check", Status: "failed", Message: "health endpoint timeout"},
			},
			status:  "faulty",
			phase:   "check",
			message: "health endpoint timeout",
		},
		{
			name: "down",
			runs: []GameCheckerRun{
				{Phase: "put", Status: "failed", Message: "target unreachable"},
				{Phase: "get", Status: "skipped"},
				{Phase: "check", Status: "skipped"},
			},
			status:  "down",
			phase:   "put",
			message: "target unreachable",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			summary := summarizeSLARuns(tc.runs, 44)
			if summary.Status != tc.status {
				t.Fatalf("expected status %q, got %q", tc.status, summary.Status)
			}
			if summary.Phase != tc.phase {
				t.Fatalf("expected phase %q, got %q", tc.phase, summary.Phase)
			}
			if summary.TickID != 44 {
				t.Fatalf("expected tick 44, got %d", summary.TickID)
			}
			if summary.Message != tc.message {
				t.Fatalf("expected message %q, got %q", tc.message, summary.Message)
			}
		})
	}
}

func TestParticipantCanDownloadChallengeSourceBundle(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Setenv("AD_CHALLENGE_SOURCE_ROOT", filepath.Clean(filepath.Join(wd, "..", "..", "..")))

	request := httptest.NewRequest(http.MethodGet, "/api/v2/challenges/1/source", nil)
	setTestTeamAuthHeader(t, request)
	response := httptest.NewRecorder()

	newTestMux().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected source download 200, got %d", response.Code)
	}
	if got := response.Header().Get("Content-Disposition"); !strings.Contains(got, "banking-source.tar.gz") {
		t.Fatalf("unexpected content disposition %q", got)
	}
	if got := response.Header().Get("Content-Type"); !strings.Contains(got, "application/gzip") {
		t.Fatalf("unexpected content type %q", got)
	}
	if response.Body.Len() == 0 {
		t.Fatal("expected non-empty source bundle response body")
	}
}

func TestParticipantCannotDownloadDraftChallengeSourceBundle(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Setenv("AD_CHALLENGE_SOURCE_ROOT", filepath.Clean(filepath.Join(wd, "..", "..", "..")))

	mux := newTestMux()

	createRequest := httptest.NewRequest(http.MethodPost, "/api/v2/admin/challenges", bytes.NewBufferString(`{
		"name":"draft-only",
		"baseline_image":"registry.local/draft:baseline",
		"checker_image":"registry.local/draft-checker:latest",
		"source_bundle_path":"examples/sample-lfi-challenge"
	}`))
	setTestAdminAuthHeader(createRequest)
	createRequest.Header.Set("Content-Type", "application/json")
	createResponse := httptest.NewRecorder()
	mux.ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusOK {
		t.Fatalf("expected challenge create 200, got %d: %s", createResponse.Code, createResponse.Body.String())
	}

	request := httptest.NewRequest(http.MethodGet, "/api/v2/challenges/4/source", nil)
	setTestTeamAuthHeader(t, request)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected draft source download 404, got %d", response.Code)
	}
}

func TestChallengeSourceRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0o600); err != nil {
		t.Fatalf("write outside source: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatalf("create symlink: %v", err)
	}
	t.Setenv("AD_CHALLENGE_SOURCE_ROOT", root)

	_, _, err := resolveChallengeSourcePath(filepath.Join("escape", "secret.txt"))
	if !errors.Is(err, errChallengeSourceUnavailable) {
		t.Fatalf("expected symlink escape to be unavailable, got %v", err)
	}
}

func TestParticipantServiceActionsRejectQueuedDeployment(t *testing.T) {
	mux := newTestMux()

	createRequest := httptest.NewRequest(http.MethodPost, "/api/v2/admin/challenges", bytes.NewBufferString(`{
		"name":"queued-only",
		"baseline_image":"registry.local/queued:baseline",
		"checker_image":"registry.local/queued-checker:latest"
	}`))
	setTestAdminAuthHeader(createRequest)
	createRequest.Header.Set("Content-Type", "application/json")
	createResponse := httptest.NewRecorder()
	mux.ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusOK {
		t.Fatalf("expected challenge create 200, got %d: %s", createResponse.Code, createResponse.Body.String())
	}

	var created adminChallenge
	if err := json.Unmarshal(createResponse.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to decode created challenge: %v", err)
	}

	deployRequest := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v2/admin/challenges/%d/deploy", created.ID), nil)
	setTestAdminAuthHeader(deployRequest)
	deployResponse := httptest.NewRecorder()
	mux.ServeHTTP(deployResponse, deployRequest)
	if deployResponse.Code != http.StatusOK {
		t.Fatalf("expected challenge deploy 200, got %d: %s", deployResponse.Code, deployResponse.Body.String())
	}

	for _, tc := range []struct {
		name string
		req  *http.Request
	}{
		{
			name: "unlock",
			req:  httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v2/services/%d/unlock", created.ID), bytes.NewBufferString(`{"proof":"`+testUnlockProof(101, created.ID)+`"}`)),
		},
		{
			name: "ssh-session",
			req:  httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v2/services/%d/ssh-session", created.ID), nil),
		},
		{
			name: "factory-reset",
			req:  httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v2/services/%d/reset/factory", created.ID), nil),
		},
		{
			name: "restart",
			req:  httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v2/services/%d/reset/restart", created.ID), nil),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setTestTeamAuthHeader(t, tc.req)
			if tc.name == "unlock" {
				tc.req.Header.Set("Content-Type", "application/json")
			}
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, tc.req)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", response.Code, response.Body.String())
			}

			var payload httpapi.ProblemDetails
			if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if payload.Detail != "service is not available yet." {
				t.Fatalf("unexpected detail %q", payload.Detail)
			}
		})
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
	setTestTeamAuthHeader(t, request)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", response.Code)
	}
	if got := response.Header().Get("Retry-After"); got != "2" {
		t.Fatalf("expected Retry-After 2, got %q", got)
	}
}

func TestUnlockReturnsRateLimit429(t *testing.T) {
	mux := newTestMuxWithLimiter(testRateLimiter{
		denyKeys: map[string]bool{
			rateLimitTeamChallengeKey("unlock", 101, 1): true,
		},
		retryAfter: 2 * time.Second,
	})

	request := httptest.NewRequest(http.MethodPost, "/api/v2/services/1/unlock", bytes.NewBufferString(`{"proof":"`+testUnlockProof(101, 1)+`"}`))
	setTestTeamAuthHeader(t, request)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", response.Code)
	}
	if got := response.Header().Get("Retry-After"); got != "2" {
		t.Fatalf("expected Retry-After 2, got %q", got)
	}
}

func TestSSHSessionReturnsRateLimit429(t *testing.T) {
	mux := newTestMuxWithLimiter(testRateLimiter{
		denyKeys: map[string]bool{
			rateLimitTeamChallengeKey("ssh-session", 101, 1): true,
		},
		retryAfter: 2 * time.Second,
	})

	request := httptest.NewRequest(http.MethodPost, "/api/v2/services/1/ssh-session", nil)
	setTestTeamAuthHeader(t, request)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", response.Code)
	}
	if got := response.Header().Get("Retry-After"); got != "2" {
		t.Fatalf("expected Retry-After 2, got %q", got)
	}
}

func TestFactoryResetReturnsRateLimit429(t *testing.T) {
	mux := newTestMuxWithLimiter(testRateLimiter{
		denyKeys: map[string]bool{
			rateLimitTeamChallengeKey("factory-reset", 101, 1): true,
		},
		retryAfter: 2 * time.Second,
	})

	request := httptest.NewRequest(http.MethodPost, "/api/v2/services/1/reset/factory", nil)
	setTestTeamAuthHeader(t, request)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", response.Code)
	}
	if got := response.Header().Get("Retry-After"); got != "2" {
		t.Fatalf("expected Retry-After 2, got %q", got)
	}
}

func TestRestartReturnsRateLimit429(t *testing.T) {
	mux := newTestMuxWithLimiter(testRateLimiter{
		denyKeys: map[string]bool{
			rateLimitTeamChallengeKey("restart", 101, 1): true,
		},
		retryAfter: 2 * time.Second,
	})

	request := httptest.NewRequest(http.MethodPost, "/api/v2/services/1/reset/restart", nil)
	setTestTeamAuthHeader(t, request)
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
	setTestTeamAuthHeader(t, unlockRequest)
	unlockResponse := httptest.NewRecorder()
	mux.ServeHTTP(unlockResponse, unlockRequest)
	if unlockResponse.Code != http.StatusOK {
		t.Fatalf("expected unlock 200, got %d", unlockResponse.Code)
	}

	resetRequest := httptest.NewRequest(http.MethodPost, "/api/v2/services/1/reset/factory", nil)
	setTestTeamAuthHeader(t, resetRequest)
	resetResponse := httptest.NewRecorder()
	mux.ServeHTTP(resetResponse, resetRequest)
	if resetResponse.Code != http.StatusOK {
		t.Fatalf("expected reset 200, got %d", resetResponse.Code)
	}

	sshRequest := httptest.NewRequest(http.MethodPost, "/api/v2/services/1/ssh-session", nil)
	setTestTeamAuthHeader(t, sshRequest)
	sshResponse := httptest.NewRecorder()
	mux.ServeHTTP(sshResponse, sshRequest)
	if sshResponse.Code != http.StatusOK {
		t.Fatalf("expected ssh session 200 after reset, got %d", sshResponse.Code)
	}

	var payload sshSessionData
	if err := json.Unmarshal(sshResponse.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode ssh response: %v", err)
	}
	if payload.Host != "10.80.1.11" {
		t.Fatalf("expected same service ip host, got %s", payload.Host)
	}
	if payload.Username != "root" {
		t.Fatalf("expected root user, got %s", payload.Username)
	}
	if payload.Password == "" {
		t.Fatal("expected stable root password to be returned")
	}
	if payload.PasswordMode != "stable" {
		t.Fatalf("expected stable password mode, got %q", payload.PasswordMode)
	}
	if payload.ConnectionHint != "ssh root@10.80.1.11" {
		t.Fatalf("unexpected connection hint: %s", payload.ConnectionHint)
	}
}

func TestFactoryResetRotatesCheckerTokenBeforeControllerReset(t *testing.T) {
	store := NewMemoryStore(101)
	before, err := store.GetControllerRuntimeTask(context.Background(), 101, 1)
	if err != nil {
		t.Fatalf("get initial runtime task: %v", err)
	}
	controller := &recordingResetControllerClient{store: store}
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "api-gateway", Version: "dev", Addr: ":0"})
	NewWithDeps("dev-team-token", "dev-admin-token", 101, store, controller, noopWireGuardClient{}).RegisterRoutes(mux)

	resetRequest := httptest.NewRequest(http.MethodPost, "/api/v2/services/1/reset/factory", nil)
	setTestTeamAuthHeader(t, resetRequest)
	resetResponse := httptest.NewRecorder()
	mux.ServeHTTP(resetResponse, resetRequest)
	if resetResponse.Code != http.StatusOK {
		t.Fatalf("expected reset 200, got %d: %s", resetResponse.Code, resetResponse.Body.String())
	}

	after, err := store.GetControllerRuntimeTask(context.Background(), 101, 1)
	if err != nil {
		t.Fatalf("get rotated runtime task: %v", err)
	}
	if before.CheckerToken == "" || after.CheckerToken == "" {
		t.Fatalf("expected non-empty checker tokens before=%q after=%q", before.CheckerToken, after.CheckerToken)
	}
	if before.CheckerToken == after.CheckerToken {
		t.Fatalf("expected checker token rotation, still %q", after.CheckerToken)
	}
	if controller.token != after.CheckerToken {
		t.Fatalf("controller saw token %q, expected rotated token %q", controller.token, after.CheckerToken)
	}
}

func TestFactoryResetFailureMarksServiceDegraded(t *testing.T) {
	store := NewMemoryStore(101)
	controller := &recordingResetControllerClient{store: store, err: errors.New("runtime reset failed")}
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "api-gateway", Version: "dev", Addr: ":0"})
	NewWithDeps("dev-team-token", "dev-admin-token", 101, store, controller, noopWireGuardClient{}).RegisterRoutes(mux)

	resetRequest := httptest.NewRequest(http.MethodPost, "/api/v2/services/1/reset/factory", nil)
	setTestTeamAuthHeader(t, resetRequest)
	resetResponse := httptest.NewRecorder()
	mux.ServeHTTP(resetResponse, resetRequest)
	if resetResponse.Code != http.StatusBadGateway {
		t.Fatalf("expected reset 502, got %d", resetResponse.Code)
	}

	services, err := store.ListTeamServices(context.Background(), 101)
	if err != nil {
		t.Fatalf("list services: %v", err)
	}
	state := findServiceStateForTest(t, services, 1)
	if state.Status != "degraded" || state.Checker != "warning" {
		t.Fatalf("expected degraded warning state after failed reset, got %+v", state)
	}
	if state.LastEvent != "factory reset runtime failed" {
		t.Fatalf("unexpected last event %q", state.LastEvent)
	}
}

func TestRestartWaitsForCheckerVerification(t *testing.T) {
	store := NewMemoryStore(101)
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "api-gateway", Version: "dev", Addr: ":0"})
	NewWithDeps("dev-team-token", "dev-admin-token", 101, store, storeBackedControllerClient{store: store}, noopWireGuardClient{}).RegisterRoutes(mux)

	restartRequest := httptest.NewRequest(http.MethodPost, "/api/v2/services/1/reset/restart", nil)
	setTestTeamAuthHeader(t, restartRequest)
	restartResponse := httptest.NewRecorder()
	mux.ServeHTTP(restartResponse, restartRequest)
	if restartResponse.Code != http.StatusOK {
		t.Fatalf("expected restart 200, got %d", restartResponse.Code)
	}

	services, err := store.ListTeamServices(context.Background(), 101)
	if err != nil {
		t.Fatalf("list services: %v", err)
	}
	state := findServiceStateForTest(t, services, 1)
	if state.Status != "warming" || state.Checker != "warning" {
		t.Fatalf("expected restart to wait for checker verification, got %+v", state)
	}
}

func TestSSHSessionReturnsBadGatewayWhenRuntimeApplyFails(t *testing.T) {
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "api-gateway", Version: "dev", Addr: ":0"})
	store := NewMemoryStore(101)
	NewWithDeps("dev-team-token", "dev-admin-token", 101, store, testControllerClient{sshApplyErr: fmt.Errorf("runtime apply failed")}, noopWireGuardClient{}).RegisterRoutes(mux)

	unlockRequest := httptest.NewRequest(http.MethodPost, "/api/v2/services/1/unlock", bytes.NewBufferString(`{"proof":"`+testUnlockProof(101, 1)+`"}`))
	setTestTeamAuthHeader(t, unlockRequest)
	unlockResponse := httptest.NewRecorder()
	mux.ServeHTTP(unlockResponse, unlockRequest)
	if unlockResponse.Code != http.StatusOK {
		t.Fatalf("expected unlock 200, got %d", unlockResponse.Code)
	}

	sshRequest := httptest.NewRequest(http.MethodPost, "/api/v2/services/1/ssh-session", nil)
	setTestTeamAuthHeader(t, sshRequest)
	sshResponse := httptest.NewRecorder()
	mux.ServeHTTP(sshResponse, sshRequest)
	if sshResponse.Code != http.StatusBadGateway {
		t.Fatalf("expected ssh session 502, got %d", sshResponse.Code)
	}

	servicesRequest := httptest.NewRequest(http.MethodGet, "/api/v2/team/services", nil)
	setTestTeamAuthHeader(t, servicesRequest)
	servicesResponse := httptest.NewRecorder()
	mux.ServeHTTP(servicesResponse, servicesRequest)
	if servicesResponse.Code != http.StatusOK {
		t.Fatalf("expected services 200, got %d", servicesResponse.Code)
	}

	var payload []serviceState
	if err := json.Unmarshal(servicesResponse.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode services response: %v", err)
	}
	for _, state := range payload {
		if state.ChallengeID == 1 {
			if state.SSHHint != "credential apply failed; open SSH Access to retry applying the team credential" {
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
	setTestTeamAuthHeader(t, unlockRequest)
	unlockResponse := httptest.NewRecorder()
	mux.ServeHTTP(unlockResponse, unlockRequest)
	if unlockResponse.Code != http.StatusOK {
		t.Fatalf("expected unlock 200, got %d", unlockResponse.Code)
	}

	resetRequest := httptest.NewRequest(http.MethodPost, "/api/v2/services/1/reset/factory", nil)
	setTestTeamAuthHeader(t, resetRequest)
	resetResponse := httptest.NewRecorder()
	mux.ServeHTTP(resetResponse, resetRequest)
	if resetResponse.Code != http.StatusOK {
		t.Fatalf("expected reset 200, got %d", resetResponse.Code)
	}

	sshRequest := httptest.NewRequest(http.MethodPost, "/api/v2/services/1/ssh-session", nil)
	setTestTeamAuthHeader(t, sshRequest)
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

	payload := decodeCompat[adminAuditLogPage](t, auditResponse.Body.Bytes())
	if payload.TotalCount < 3 {
		t.Fatalf("expected at least 3 audit entries, got %d", payload.TotalCount)
	}

	joinedActions := make([]string, 0, len(payload.Items))
	for _, item := range payload.Items {
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

	body := bytes.NewBufferString(`{"name":"audit-demo","baseline_image":"registry.local/audit-demo:baseline","checker_image":"registry.local/audit-demo-checker:latest","service_port":31010,"service_subnet_octet":10}`)
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

	payload := decodeCompat[adminAuditLogPage](t, auditResponse.Body.Bytes())
	if payload.TotalCount == 0 {
		t.Fatal("expected at least one challenge.create audit entry")
	}
	if payload.Items[0].Action != "challenge.create" {
		t.Fatalf("expected challenge.create action, got %s", payload.Items[0].Action)
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

	teamPayload := decodeCompat[adminTeam](t, teamResponse.Body.Bytes())

	playerRequest := httptest.NewRequest(http.MethodPost, "/api/v2/admin/players", bytes.NewBufferString(fmt.Sprintf(`{"team_id":%d,"display_name":"Nova Captain","email":"nova.captain@example.com","password":"nova-secret","role":"captain"}`, teamPayload.ID)))
	playerRequest.Header.Set("Authorization", adminAuth)
	playerResponse := httptest.NewRecorder()
	mux.ServeHTTP(playerResponse, playerRequest)
	if playerResponse.Code != http.StatusOK {
		t.Fatalf("expected player create 200, got %d", playerResponse.Code)
	}

	challengeRequest := httptest.NewRequest(http.MethodPost, "/api/v2/admin/challenges", bytes.NewBufferString(`{"name":"proxy","baseline_image":"registry.local/proxy:baseline","checker_image":"registry.local/proxy-checker:latest"}`))
	challengeRequest.Header.Set("Authorization", adminAuth)
	challengeResponse := httptest.NewRecorder()
	mux.ServeHTTP(challengeResponse, challengeRequest)
	if challengeResponse.Code != http.StatusOK {
		t.Fatalf("expected challenge create 200, got %d", challengeResponse.Code)
	}

	challengePayload := decodeCompat[adminChallenge](t, challengeResponse.Body.Bytes())

	deployRequest := httptest.NewRequest(http.MethodPost, "/api/v2/admin/challenges/"+fmt.Sprintf("%d", challengePayload.ID)+"/deploy", nil)
	deployRequest.Header.Set("Authorization", adminAuth)
	deployResponse := httptest.NewRecorder()
	mux.ServeHTTP(deployResponse, deployRequest)
	if deployResponse.Code != http.StatusOK {
		t.Fatalf("expected deploy 200, got %d", deployResponse.Code)
	}
	deployPayload := decodeCompat[adminDeployment](t, deployResponse.Body.Bytes())
	if deployPayload.Status != "queued" {
		t.Fatalf("expected queued deploy status, got %s", deployPayload.Status)
	}
	if deployPayload.JobID == 0 {
		t.Fatal("expected deployment job id")
	}

	servicesRequest := httptest.NewRequest(http.MethodGet, "/api/v2/services", nil)
	setTestTeamAuthHeader(t, servicesRequest)
	servicesResponse := httptest.NewRecorder()
	mux.ServeHTTP(servicesResponse, servicesRequest)
	if servicesResponse.Code != http.StatusOK {
		t.Fatalf("expected services 200, got %d", servicesResponse.Code)
	}

	var servicesPayload map[string]map[string][]string
	if err := json.Unmarshal(servicesResponse.Body.Bytes(), &servicesPayload); err != nil {
		t.Fatalf("failed to decode services response: %v", err)
	}
	challengeKey := fmt.Sprintf("%d", challengePayload.ID)
	if _, ok := servicesPayload[challengeKey]; !ok {
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
	setTestTeamAuthHeader(t, teamServicesRequest)
	teamServicesResponse := httptest.NewRecorder()
	mux.ServeHTTP(teamServicesResponse, teamServicesRequest)
	if teamServicesResponse.Code != http.StatusOK {
		t.Fatalf("expected team services 200, got %d", teamServicesResponse.Code)
	}

	var teamServicesPayload []serviceState
	if err := json.Unmarshal(teamServicesResponse.Body.Bytes(), &teamServicesPayload); err != nil {
		t.Fatalf("failed to decode team services response: %v", err)
	}
	foundReady := false
	for _, service := range teamServicesPayload {
		if service.ChallengeID == challengePayload.ID {
			foundReady = service.Status == "stable"
			break
		}
	}
	if !foundReady {
		t.Fatal("expected reconciled challenge to reach stable service state")
	}
}

func TestAdminCreateChallengeRejectsInvalidSourceBundlePath(t *testing.T) {
	t.Setenv("AD_CHALLENGE_SOURCE_ROOT", t.TempDir())

	mux := newTestMux()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v2/admin/challenges",
		bytes.NewBufferString(`{"name":"proxy","baseline_image":"registry.local/proxy:baseline","checker_image":"registry.local/proxy-checker:latest","source_bundle_path":"missing-source"}`),
	)
	request.Header.Set("Authorization", "Bearer dev-admin-token")
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected challenge create 400, got %d", response.Code)
	}
	problem := decodeProblemCompat(t, response.Body.Bytes())
	if problem.Detail != "challenge source path is invalid." {
		t.Fatalf("unexpected problem payload %+v", problem)
	}
}

func TestAdminCreateChallengeAcceptsValidSourceBundlePath(t *testing.T) {
	root := t.TempDir()
	t.Setenv("AD_CHALLENGE_SOURCE_ROOT", root)
	if err := os.WriteFile(filepath.Join(root, "sample-lfi-challenge.tar.gz"), []byte("placeholder"), 0o644); err != nil {
		t.Fatalf("write source bundle: %v", err)
	}

	mux := newTestMux()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v2/admin/challenges",
		bytes.NewBufferString(`{"name":"proxy","baseline_image":"registry.local/proxy:baseline","checker_image":"registry.local/proxy-checker:latest","source_bundle_path":"sample-lfi-challenge.tar.gz"}`),
	)
	request.Header.Set("Authorization", "Bearer dev-admin-token")
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected challenge create 200, got %d", response.Code)
	}
	challenge := decodeCompat[adminChallenge](t, response.Body.Bytes())
	if challenge.SourceBundlePath != "sample-lfi-challenge.tar.gz" {
		t.Fatalf("unexpected source bundle path %+v", challenge)
	}
}

func TestAdminUpdateChallengeRejectsInvalidSourceBundlePath(t *testing.T) {
	t.Setenv("AD_CHALLENGE_SOURCE_ROOT", t.TempDir())

	mux := newTestMux()
	updateRequest := httptest.NewRequest(
		http.MethodPut,
		"/api/v2/admin/challenges/1",
		bytes.NewBufferString(`{"name":"banking","baseline_image":"registry.local/banking:baseline","checker_image":"registry.local/banking-checker:latest","source_bundle_path":"missing-source"}`),
	)
	updateRequest.Header.Set("Authorization", "Bearer dev-admin-token")
	updateResponse := httptest.NewRecorder()

	mux.ServeHTTP(updateResponse, updateRequest)

	if updateResponse.Code != http.StatusBadRequest {
		t.Fatalf("expected challenge update 400, got %d", updateResponse.Code)
	}
	problem := decodeProblemCompat(t, updateResponse.Body.Bytes())
	if problem.Detail != "challenge source path is invalid." {
		t.Fatalf("unexpected problem payload %+v", problem)
	}
}

func TestAdminDeployUsesConfiguredServiceSubnetAndPort(t *testing.T) {
	mux := newTestMux()
	adminAuth := "Bearer dev-admin-token"

	challengeRequest := httptest.NewRequest(http.MethodPost, "/api/v2/admin/challenges", bytes.NewBufferString(`{"name":"proxy-custom","baseline_image":"registry.local/proxy-custom:baseline","checker_image":"registry.local/proxy-custom-checker:latest","service_port":31337,"service_subnet_octet":77}`))
	challengeRequest.Header.Set("Authorization", adminAuth)
	challengeResponse := httptest.NewRecorder()
	mux.ServeHTTP(challengeResponse, challengeRequest)
	if challengeResponse.Code != http.StatusOK {
		t.Fatalf("expected challenge create 200, got %d", challengeResponse.Code)
	}

	challengePayload := decodeCompat[adminChallenge](t, challengeResponse.Body.Bytes())
	if challengePayload.ServicePort != 31337 || challengePayload.ServiceSubnetOctet != 77 {
		t.Fatalf("unexpected runtime spec %+v", challengePayload)
	}

	deployRequest := httptest.NewRequest(http.MethodPost, "/api/v2/admin/challenges/"+fmt.Sprintf("%d", challengePayload.ID)+"/deploy", nil)
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
	setTestTeamAuthHeader(t, teamServicesRequest)
	teamServicesResponse := httptest.NewRecorder()
	mux.ServeHTTP(teamServicesResponse, teamServicesRequest)
	if teamServicesResponse.Code != http.StatusOK {
		t.Fatalf("expected team services 200, got %d", teamServicesResponse.Code)
	}

	var teamServicesPayload []serviceState
	if err := json.Unmarshal(teamServicesResponse.Body.Bytes(), &teamServicesPayload); err != nil {
		t.Fatalf("failed to decode team services response: %v", err)
	}

	expectedEndpoint := "10.80.77.11:31337"
	for _, service := range teamServicesPayload {
		if service.ChallengeID == challengePayload.ID {
			if service.Endpoint != expectedEndpoint {
				t.Fatalf("expected endpoint %s, got %s", expectedEndpoint, service.Endpoint)
			}
			return
		}
	}
	t.Fatalf("expected deployed service for challenge %d", challengePayload.ID)
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

	payload := decodeCompat[adminReconcileResult](t, response.Body.Bytes())
	if payload.ProcessedJobs != 7 || payload.ProcessedInstances != 21 || payload.CompletedJobs != 3 {
		t.Fatalf("unexpected reconcile payload %+v", payload)
	}
}

func TestAdminReconcileDeploymentsReturnsServiceUnavailableWhenControllerDisabled(t *testing.T) {
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "api-gateway", Version: "dev", Addr: ":0"})
	NewWithDeps(
		"dev-team-token",
		"dev-admin-token",
		101,
		NewMemoryStore(101),
		noopControllerClient{},
		noopWireGuardClient{},
	).RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodPost, "/api/v2/admin/deployments/reconcile", nil)
	request.Header.Set("Authorization", "Bearer dev-admin-token")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected reconcile 503, got %d", response.Code)
	}
	problem := decodeProblemCompat(t, response.Body.Bytes())
	if problem.Title != "Deployment reconcile unavailable" || problem.Detail != "controller deployment reconcile is not configured." {
		t.Fatalf("unexpected problem payload %+v", problem)
	}
}

func TestAdminReconcileDeploymentsReturnsControllerProblemStatus(t *testing.T) {
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "api-gateway", Version: "dev", Addr: ":0"})
	NewWithDeps(
		"dev-team-token",
		"dev-admin-token",
		101,
		NewMemoryStore(101),
		testControllerClient{deployReconcileErr: &controllerProblemError{
			statusCode: http.StatusBadGateway,
			title:      "Deployment reconcile failed",
			detail:     "runtime and controller access converged but WireGuard converge/verification failed, so host access truth was not established.",
		}},
		noopWireGuardClient{},
	).RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodPost, "/api/v2/admin/deployments/reconcile", nil)
	request.Header.Set("Authorization", "Bearer dev-admin-token")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadGateway {
		t.Fatalf("expected reconcile 502, got %d", response.Code)
	}
	problem := decodeProblemCompat(t, response.Body.Bytes())
	if problem.Title != "Deployment reconcile failed" || problem.Detail != "runtime and controller access converged but WireGuard converge/verification failed, so host access truth was not established." {
		t.Fatalf("unexpected problem payload %+v", problem)
	}
}

func TestAdminCanDeleteCompletedDeploymentJob(t *testing.T) {
	mux := newTestMux()
	adminAuth := "Bearer dev-admin-token"

	challengeRequest := httptest.NewRequest(http.MethodPost, "/api/v2/admin/challenges", bytes.NewBufferString(`{"name":"cleanup","baseline_image":"registry.local/cleanup:baseline","checker_image":"registry.local/cleanup-checker:latest"}`))
	challengeRequest.Header.Set("Authorization", adminAuth)
	challengeResponse := httptest.NewRecorder()
	mux.ServeHTTP(challengeResponse, challengeRequest)
	if challengeResponse.Code != http.StatusOK {
		t.Fatalf("expected challenge create 200, got %d", challengeResponse.Code)
	}

	challengePayload := decodeCompat[adminChallenge](t, challengeResponse.Body.Bytes())

	deployRequest := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v2/admin/challenges/%d/deploy", challengePayload.ID), nil)
	deployRequest.Header.Set("Authorization", adminAuth)
	deployResponse := httptest.NewRecorder()
	mux.ServeHTTP(deployResponse, deployRequest)
	if deployResponse.Code != http.StatusOK {
		t.Fatalf("expected deploy 200, got %d", deployResponse.Code)
	}

	deployPayload := decodeCompat[adminDeployment](t, deployResponse.Body.Bytes())

	reconcileRequest := httptest.NewRequest(http.MethodPost, "/api/v2/admin/deployments/reconcile", nil)
	reconcileRequest.Header.Set("Authorization", adminAuth)
	reconcileResponse := httptest.NewRecorder()
	mux.ServeHTTP(reconcileResponse, reconcileRequest)
	if reconcileResponse.Code != http.StatusOK {
		t.Fatalf("expected reconcile 200, got %d", reconcileResponse.Code)
	}

	deleteRequest := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v2/admin/deployments/%d", deployPayload.JobID), nil)
	deleteRequest.Header.Set("Authorization", adminAuth)
	deleteResponse := httptest.NewRecorder()
	mux.ServeHTTP(deleteResponse, deleteRequest)
	if deleteResponse.Code != http.StatusNoContent {
		t.Fatalf("expected deployment delete 204, got %d", deleteResponse.Code)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v2/admin/deployments", nil)
	listRequest.Header.Set("Authorization", adminAuth)
	listResponse := httptest.NewRecorder()
	mux.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("expected deployment list 200, got %d", listResponse.Code)
	}

	listPayload := decodeCompat[[]adminDeploymentJob](t, listResponse.Body.Bytes())
	if len(listPayload) != 0 {
		t.Fatalf("expected deleted deployment job to be removed, got %+v", listPayload)
	}
}

func TestAdminRedeploySupersedesOlderQueuedJob(t *testing.T) {
	mux := newTestMux()
	adminAuth := "Bearer dev-admin-token"

	challengeRequest := httptest.NewRequest(http.MethodPost, "/api/v2/admin/challenges", bytes.NewBufferString(`{"name":"redeploy","baseline_image":"registry.local/redeploy:baseline","checker_image":"registry.local/redeploy-checker:latest"}`))
	challengeRequest.Header.Set("Authorization", adminAuth)
	challengeResponse := httptest.NewRecorder()
	mux.ServeHTTP(challengeResponse, challengeRequest)
	if challengeResponse.Code != http.StatusOK {
		t.Fatalf("expected challenge create 200, got %d", challengeResponse.Code)
	}

	challengePayload := decodeCompat[adminChallenge](t, challengeResponse.Body.Bytes())

	firstDeployRequest := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v2/admin/challenges/%d/deploy", challengePayload.ID), nil)
	firstDeployRequest.Header.Set("Authorization", adminAuth)
	firstDeployResponse := httptest.NewRecorder()
	mux.ServeHTTP(firstDeployResponse, firstDeployRequest)
	if firstDeployResponse.Code != http.StatusOK {
		t.Fatalf("expected first deploy 200, got %d", firstDeployResponse.Code)
	}

	secondDeployRequest := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v2/admin/challenges/%d/deploy", challengePayload.ID), nil)
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

	listPayload := decodeCompat[[]adminDeploymentJob](t, listResponse.Body.Bytes())
	if len(listPayload) != 2 {
		t.Fatalf("expected two deployment jobs after redeploy, got %+v", listPayload)
	}
	if listPayload[0].Status != "queued" {
		t.Fatalf("expected newest deployment job to stay queued, got %+v", listPayload[0])
	}
	if listPayload[1].Status != "superseded" {
		t.Fatalf("expected older deployment job to be superseded, got %+v", listPayload[1])
	}
	if listPayload[1].QueuedTeamCount != 0 {
		t.Fatalf("expected superseded deployment job queue to be cleared, got %+v", listPayload[1])
	}
	if listPayload[1].CompletedAt == "" {
		t.Fatalf("expected superseded deployment job completion time, got %+v", listPayload[1])
	}
}

func TestAdminCannotDeleteActiveDeploymentJob(t *testing.T) {
	mux := newTestMux()
	adminAuth := "Bearer dev-admin-token"

	challengeRequest := httptest.NewRequest(http.MethodPost, "/api/v2/admin/challenges", bytes.NewBufferString(`{"name":"active-job","baseline_image":"registry.local/active-job:baseline","checker_image":"registry.local/active-job-checker:latest"}`))
	challengeRequest.Header.Set("Authorization", adminAuth)
	challengeResponse := httptest.NewRecorder()
	mux.ServeHTTP(challengeResponse, challengeRequest)
	if challengeResponse.Code != http.StatusOK {
		t.Fatalf("expected challenge create 200, got %d", challengeResponse.Code)
	}

	challengePayload := decodeCompat[adminChallenge](t, challengeResponse.Body.Bytes())

	deployRequest := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v2/admin/challenges/%d/deploy", challengePayload.ID), nil)
	deployRequest.Header.Set("Authorization", adminAuth)
	deployResponse := httptest.NewRecorder()
	mux.ServeHTTP(deployResponse, deployRequest)
	if deployResponse.Code != http.StatusOK {
		t.Fatalf("expected deploy 200, got %d", deployResponse.Code)
	}

	deployPayload := decodeCompat[adminDeployment](t, deployResponse.Body.Bytes())

	deleteRequest := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v2/admin/deployments/%d", deployPayload.JobID), nil)
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

	payload := decodeCompat[ChallengeValidationResult](t, response.Body.Bytes())
	if payload.Status != "valid" || !payload.BaselineSSHContractOK || !payload.CheckerContractOK || !payload.ServiceStateContractOK {
		t.Fatalf("unexpected validation result %+v", payload)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v2/admin/challenges", nil)
	listRequest.Header.Set("Authorization", adminAuth)
	listResponse := httptest.NewRecorder()
	mux.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("expected list 200, got %d", listResponse.Code)
	}
	challenges := decodeCompat[[]adminChallenge](t, listResponse.Body.Bytes())
	if len(challenges) == 0 || challenges[0].LastValidation == nil {
		t.Fatalf("expected persisted validation in challenge list, got %+v", challenges)
	}
	if challenges[0].LastValidation.Status != "valid" {
		t.Fatalf("unexpected persisted validation %+v", challenges[0].LastValidation)
	}
}

func TestAdminDeployRejectsInvalidChallengeRuntime(t *testing.T) {
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "api-gateway", Version: "dev", Addr: ":0"})
	store := NewMemoryStore(101)
	NewWithDeps("dev-team-token", "dev-admin-token", 101, store, testControllerClient{
		validationResult: ChallengeValidationResult{
			Status:                 "invalid",
			BaselineSSHContractOK:  false,
			CheckerContractOK:      false,
			ServiceStateContractOK: false,
			Message:                "missing ssh daemon binary inside image",
		},
	}, noopWireGuardClient{}).RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodPost, "/api/v2/admin/challenges/1/deploy", nil)
	request.Header.Set("Authorization", "Bearer dev-admin-token")
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected deploy rejection 400, got %d", response.Code)
	}

	payload := decodeProblemCompat(t, response.Body.Bytes())
	if payload.Detail != "missing ssh daemon binary inside image" {
		t.Fatalf("unexpected deploy rejection message %q", payload.Detail)
	}
}

func TestAdminDeployRejectsInvalidCheckerRuntime(t *testing.T) {
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "api-gateway", Version: "dev", Addr: ":0"})
	store := NewMemoryStore(101)
	NewWithDeps("dev-team-token", "dev-admin-token", 101, store, testControllerClient{
		validationResult: ChallengeValidationResult{
			Status:                 "invalid",
			BaselineSSHContractOK:  true,
			CheckerContractOK:      false,
			ServiceStateContractOK: false,
			Message:                "missing standard checker entrypoint inside image",
		},
	}, noopWireGuardClient{}).RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodPost, "/api/v2/admin/challenges/1/deploy", nil)
	request.Header.Set("Authorization", "Bearer dev-admin-token")
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected deploy rejection 400, got %d", response.Code)
	}

	payload := decodeProblemCompat(t, response.Body.Bytes())
	if payload.Detail != "missing standard checker entrypoint inside image" {
		t.Fatalf("unexpected deploy rejection message %q", payload.Detail)
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

	teamPayload := decodeCompat[adminTeam](t, teamResponse.Body.Bytes())

	playerRequest := httptest.NewRequest(http.MethodPost, "/api/v2/admin/players", bytes.NewBufferString(fmt.Sprintf(`{"team_id":%d,"display_name":"Nova Captain","email":"nova.captain@example.com","password":"nova-secret","role":"captain"}`, teamPayload.ID)))
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
		Token string `json:"token"`
	}
	if err := json.Unmarshal(authResponse.Body.Bytes(), &authPayload); err != nil {
		t.Fatalf("failed to decode auth response: %v", err)
	}

	servicesRequest := httptest.NewRequest(http.MethodGet, "/api/v2/team/services", nil)
	servicesRequest.Header.Set("Authorization", "Bearer "+authPayload.Token)
	servicesResponse := httptest.NewRecorder()
	mux.ServeHTTP(servicesResponse, servicesRequest)
	if servicesResponse.Code != http.StatusOK {
		t.Fatalf("expected services 200, got %d", servicesResponse.Code)
	}

	var servicesPayload []serviceState
	if err := json.Unmarshal(servicesResponse.Body.Bytes(), &servicesPayload); err != nil {
		t.Fatalf("failed to decode services response: %v", err)
	}
	if len(servicesPayload) == 0 {
		t.Fatal("expected team services for authenticated player")
	}
	for _, state := range servicesPayload {
		if state.TeamID != teamPayload.ID {
			t.Fatalf("expected team id %d, got %d", teamPayload.ID, state.TeamID)
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

	playerPayload := decodeCompat[adminPlayer](t, createResponse.Body.Bytes())

	getRequest := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v2/admin/players/%d/wireguard", playerPayload.ID), nil)
	getRequest.Header.Set("Authorization", adminAuth)
	getResponse := httptest.NewRecorder()
	mux.ServeHTTP(getResponse, getRequest)
	if getResponse.Code != http.StatusOK {
		t.Fatalf("expected wireguard get 200, got %d", getResponse.Code)
	}

	getPayload := decodeCompat[adminWireGuardPeer](t, getResponse.Body.Bytes())
	if getPayload.Status != "active" {
		t.Fatalf("expected active peer, got %s", getPayload.Status)
	}
	if !bytes.Contains([]byte(getPayload.Config), []byte("[Interface]")) {
		t.Fatalf("expected wireguard config body, got %q", getPayload.Config)
	}

	revokeRequest := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v2/admin/players/%d/wireguard/revoke", playerPayload.ID), nil)
	revokeRequest.Header.Set("Authorization", adminAuth)
	revokeResponse := httptest.NewRecorder()
	mux.ServeHTTP(revokeResponse, revokeRequest)
	if revokeResponse.Code != http.StatusOK {
		t.Fatalf("expected revoke 200, got %d", revokeResponse.Code)
	}

	revokePayload := decodeCompat[adminWireGuardPeer](t, revokeResponse.Body.Bytes())
	if revokePayload.Status != "revoked" || revokePayload.RevokedAt == "" {
		t.Fatalf("expected revoked peer with timestamp, got %+v", revokePayload)
	}

	rotateRequest := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v2/admin/players/%d/wireguard/rotate", playerPayload.ID), nil)
	rotateRequest.Header.Set("Authorization", adminAuth)
	rotateResponse := httptest.NewRecorder()
	mux.ServeHTTP(rotateResponse, rotateRequest)
	if rotateResponse.Code != http.StatusOK {
		t.Fatalf("expected rotate 200, got %d", rotateResponse.Code)
	}

	rotatePayload := decodeCompat[adminWireGuardPeer](t, rotateResponse.Body.Bytes())
	if rotatePayload.Status != "active" || rotatePayload.RevokedAt != "" {
		t.Fatalf("expected active peer after rotate, got %+v", rotatePayload)
	}
	if rotatePayload.Config == getPayload.Config {
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

	payload := decodeCompat[WireGuardGatewayStatus](t, response.Body.Bytes())
	if payload.State != "disabled" || payload.Mode != "disabled" {
		t.Fatalf("unexpected gateway status %+v", payload)
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
				Match:     &GameMatchStatus{State: "stopped", AcceptingSubmissions: false},
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

	payload := decodeCompat[AdminOperationsStatus](t, response.Body.Bytes())
	if !payload.Healthy {
		t.Fatalf("expected healthy operations status, got %+v", payload)
	}
	if len(payload.Alerts) != 0 {
		t.Fatalf("expected no alerts, got %+v", payload.Alerts)
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

	payload := decodeCompat[AdminOperationsStatus](t, response.Body.Bytes())
	if payload.Healthy {
		t.Fatalf("expected degraded operations status, got %+v", payload)
	}

	alertIDs := make(map[string]bool, len(payload.Alerts))
	for _, alert := range payload.Alerts {
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
			t.Fatalf("expected alert %q in %+v", required, payload.Alerts)
		}
	}
}

func TestEvaluateAccessOperationsAlertsDeduplicatesSharedPeersAcrossPolicies(t *testing.T) {
	status := ControllerAccessStatus{
		State:             "applied",
		Mode:              "host",
		PoliciesTotal:     2,
		SSHOpenServices:   2,
		SSHLockedServices: 0,
		AllowedPeersTotal: 4,
	}
	policies := []ControllerServiceAccessPolicy{
		{
			TeamID:               101,
			ChallengeID:          1,
			SSHUnlocked:          true,
			AllowedPeerAddresses: []string{"10.70.11.20", "10.70.11.21", "10.70.12.22"},
		},
		{
			TeamID:               102,
			ChallengeID:          1,
			SSHUnlocked:          true,
			AllowedPeerAddresses: []string{"10.70.11.20", "10.70.12.23"},
		},
	}

	alerts := evaluateAccessOperationsAlerts(status, policies)
	for _, alert := range alerts {
		if alert.ID == "access-policy-drift" {
			t.Fatalf("did not expect false drift alert for shared peers across policies: %+v", alerts)
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

	matchPayload := decodeCompat[GameMatchStatus](t, matchResponse.Body.Bytes())
	if matchPayload.State != "running" || !matchPayload.AcceptingSubmissions {
		t.Fatalf("unexpected match payload %+v", matchPayload)
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

	updateMatchSchedulePayload := decodeCompat[GameMatchStatus](t, updateMatchScheduleResponse.Body.Bytes())
	if updateMatchSchedulePayload.ScheduledStartAt != "2026-03-10T11:00:00Z" || updateMatchSchedulePayload.ScheduledEndAt != "2026-03-10T13:00:00Z" {
		t.Fatalf("unexpected updated match schedule payload %+v", updateMatchSchedulePayload)
	}

	statusRequest := httptest.NewRequest(http.MethodGet, "/api/v2/admin/game/status", nil)
	statusRequest.Header.Set("Authorization", adminAuth)
	statusResponse := httptest.NewRecorder()
	mux.ServeHTTP(statusResponse, statusRequest)
	if statusResponse.Code != http.StatusOK {
		t.Fatalf("expected game status 200, got %d", statusResponse.Code)
	}

	statusPayload := decodeCompat[GameStatus](t, statusResponse.Body.Bytes())
	if statusPayload.CurrentTick == nil || statusPayload.CurrentTick.ID != 1 {
		t.Fatalf("unexpected current tick %+v", statusPayload.CurrentTick)
	}

	publicStatusRequest := httptest.NewRequest(http.MethodGet, "/api/v2/game/status", nil)
	publicStatusResponse := httptest.NewRecorder()
	mux.ServeHTTP(publicStatusResponse, publicStatusRequest)
	if publicStatusResponse.Code != http.StatusOK {
		t.Fatalf("expected public game status 200, got %d", publicStatusResponse.Code)
	}

	var publicStatusPayload GameStatus
	if err := json.Unmarshal(publicStatusResponse.Body.Bytes(), &publicStatusPayload); err != nil {
		t.Fatalf("failed to decode public game status response: %v", err)
	}
	if publicStatusPayload.Match == nil || publicStatusPayload.Match.State != "running" {
		t.Fatalf("unexpected public game match %+v", publicStatusPayload.Match)
	}

	advanceRequest := httptest.NewRequest(http.MethodPost, "/api/v2/admin/game/ticks/advance", nil)
	advanceRequest.Header.Set("Authorization", adminAuth)
	advanceResponse := httptest.NewRecorder()
	mux.ServeHTTP(advanceResponse, advanceRequest)
	if advanceResponse.Code != http.StatusOK {
		t.Fatalf("expected advance 200, got %d", advanceResponse.Code)
	}

	advancePayload := decodeCompat[GameTickStatus](t, advanceResponse.Body.Bytes())
	if advancePayload.ID != 2 || advancePayload.FailedCheckerRuns != 3 {
		t.Fatalf("unexpected advance payload %+v", advancePayload)
	}

	runsRequest := httptest.NewRequest(http.MethodGet, "/api/v2/admin/game/checker-runs?limit=1", nil)
	runsRequest.Header.Set("Authorization", adminAuth)
	runsResponse := httptest.NewRecorder()
	mux.ServeHTTP(runsResponse, runsRequest)
	if runsResponse.Code != http.StatusOK {
		t.Fatalf("expected checker runs 200, got %d", runsResponse.Code)
	}

	runsPayload := decodeCompat[GameCheckerRunPage](t, runsResponse.Body.Bytes())
	if len(runsPayload.Items) != 1 || runsPayload.Items[0].Phase != "put" {
		t.Fatalf("unexpected checker runs payload %+v", runsPayload)
	}
	if runsPayload.TotalCount != 2 || !runsPayload.HasNext {
		t.Fatalf("unexpected checker run page metadata %+v", runsPayload)
	}

	schedulerRequest := httptest.NewRequest(http.MethodGet, "/api/v2/admin/game/scheduler", nil)
	schedulerRequest.Header.Set("Authorization", adminAuth)
	schedulerResponse := httptest.NewRecorder()
	mux.ServeHTTP(schedulerResponse, schedulerRequest)
	if schedulerResponse.Code != http.StatusOK {
		t.Fatalf("expected scheduler status 200, got %d", schedulerResponse.Code)
	}

	schedulerPayload := decodeCompat[GameSchedulerStatus](t, schedulerResponse.Body.Bytes())
	if schedulerPayload.State != "stopped" {
		t.Fatalf("unexpected scheduler payload %+v", schedulerPayload)
	}

	eventsRequest := httptest.NewRequest(http.MethodGet, "/api/v2/admin/game/scheduler/events?limit=1", nil)
	eventsRequest.Header.Set("Authorization", adminAuth)
	eventsResponse := httptest.NewRecorder()
	mux.ServeHTTP(eventsResponse, eventsRequest)
	if eventsResponse.Code != http.StatusOK {
		t.Fatalf("expected scheduler events 200, got %d", eventsResponse.Code)
	}

	eventsPayload := decodeCompat[GameSchedulerEventPage](t, eventsResponse.Body.Bytes())
	if len(eventsPayload.Items) != 1 || eventsPayload.Items[0].EventType != "tick_completed" {
		t.Fatalf("unexpected scheduler events payload %+v", eventsPayload)
	}
	if eventsPayload.TotalCount != 2 || !eventsPayload.HasNext {
		t.Fatalf("unexpected scheduler event page metadata %+v", eventsPayload)
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

	scoreboardPayload := decodeCompat[[]ScoreRowAlias](t, scoreboardResponse.Body.Bytes())
	if len(scoreboardPayload) == 0 || scoreboardPayload[0].Rank != 1 {
		t.Fatalf("unexpected scoreboard payload %+v", scoreboardPayload)
	}

	recomputeRequest := httptest.NewRequest(http.MethodPost, "/api/v2/admin/game/scoring/recompute", nil)
	recomputeRequest.Header.Set("Authorization", adminAuth)
	recomputeResponse := httptest.NewRecorder()
	mux.ServeHTTP(recomputeResponse, recomputeRequest)
	if recomputeResponse.Code != http.StatusOK {
		t.Fatalf("expected score recompute 200, got %d", recomputeResponse.Code)
	}

	recomputePayload := decodeCompat[[]ScoreRowAlias](t, recomputeResponse.Body.Bytes())
	if len(recomputePayload) == 0 || recomputePayload[0].Team == "" {
		t.Fatalf("unexpected recompute payload %+v", recomputePayload)
	}

	auditRequest := httptest.NewRequest(http.MethodGet, "/api/v2/admin/game/scoring/audit", nil)
	auditRequest.Header.Set("Authorization", adminAuth)
	auditResponse := httptest.NewRecorder()
	mux.ServeHTTP(auditResponse, auditRequest)
	if auditResponse.Code != http.StatusOK {
		t.Fatalf("expected score audit 200, got %d", auditResponse.Code)
	}

	auditPayload := decodeCompat[ScoringAuditAlias](t, auditResponse.Body.Bytes())
	if auditPayload.Status != "ok" || auditPayload.StoredRows == 0 || auditPayload.ReplayedRows == 0 {
		t.Fatalf("unexpected scoring audit payload %+v", auditPayload)
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

	runsPayload := decodeCompat[GameCheckerRunPage](t, runsResponse.Body.Bytes())
	if len(runsPayload.Items) != 1 || runsPayload.Items[0].ID != 31 {
		t.Fatalf("unexpected filtered checker runs payload %+v", runsPayload)
	}
	if runsPayload.TotalCount != 1 || runsPayload.HasNext || runsPayload.HasPrev {
		t.Fatalf("unexpected filtered checker run page metadata %+v", runsPayload)
	}

	runsOffsetRequest := httptest.NewRequest(http.MethodGet, "/api/v2/admin/game/checker-runs?team_id=101&offset=1&limit=1", nil)
	runsOffsetRequest.Header.Set("Authorization", adminAuth)
	runsOffsetResponse := httptest.NewRecorder()
	mux.ServeHTTP(runsOffsetResponse, runsOffsetRequest)
	if runsOffsetResponse.Code != http.StatusOK {
		t.Fatalf("expected checker runs offset 200, got %d", runsOffsetResponse.Code)
	}
	runsPayload = decodeCompat[GameCheckerRunPage](t, runsOffsetResponse.Body.Bytes())
	if len(runsPayload.Items) != 1 || runsPayload.Items[0].ID != 30 {
		t.Fatalf("unexpected checker runs offset payload %+v", runsPayload)
	}
	if !runsPayload.HasPrev || !runsPayload.HasNext || runsPayload.TotalCount != 3 {
		t.Fatalf("unexpected checker runs offset metadata %+v", runsPayload)
	}

	eventsRequest := httptest.NewRequest(http.MethodGet, "/api/v2/admin/game/scheduler/events?source=scheduler&state=running&event_type=tick_failed&limit=1", nil)
	eventsRequest.Header.Set("Authorization", adminAuth)
	eventsResponse := httptest.NewRecorder()
	mux.ServeHTTP(eventsResponse, eventsRequest)
	if eventsResponse.Code != http.StatusOK {
		t.Fatalf("expected filtered scheduler events 200, got %d", eventsResponse.Code)
	}

	eventsPayload := decodeCompat[GameSchedulerEventPage](t, eventsResponse.Body.Bytes())
	if len(eventsPayload.Items) != 1 || eventsPayload.Items[0].ID != 11 {
		t.Fatalf("unexpected filtered scheduler events payload %+v", eventsPayload)
	}
	if eventsPayload.TotalCount != 1 || eventsPayload.HasNext || eventsPayload.HasPrev {
		t.Fatalf("unexpected filtered scheduler page metadata %+v", eventsPayload)
	}

	eventsOffsetRequest := httptest.NewRequest(http.MethodGet, "/api/v2/admin/game/scheduler/events?source=scheduler&offset=1&limit=1", nil)
	eventsOffsetRequest.Header.Set("Authorization", adminAuth)
	eventsOffsetResponse := httptest.NewRecorder()
	mux.ServeHTTP(eventsOffsetResponse, eventsOffsetRequest)
	if eventsOffsetResponse.Code != http.StatusOK {
		t.Fatalf("expected scheduler events offset 200, got %d", eventsOffsetResponse.Code)
	}
	eventsPayload = decodeCompat[GameSchedulerEventPage](t, eventsOffsetResponse.Body.Bytes())
	if len(eventsPayload.Items) != 1 || eventsPayload.Items[0].ID != 10 {
		t.Fatalf("unexpected scheduler events offset payload %+v", eventsPayload)
	}
	if !eventsPayload.HasPrev || eventsPayload.HasNext || eventsPayload.TotalCount != 2 {
		t.Fatalf("unexpected scheduler events offset metadata %+v", eventsPayload)
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
				{Flag: "FLAGv1.authoritative", Status: "accepted", Detail: "flag is correct."},
				{Flag: "FLAGv1.dupe", Status: "duplicate", Detail: "flag already submitted."},
			},
		},
	).RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodPost, "/api/v2/submit", bytes.NewBufferString(`{"flags":["FLAGv1.authoritative","FLAGv1.dupe"]}`))
	setTestTeamAuthHeader(t, request)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}

	var payload submissionResult
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode submit response: %v", err)
	}
	if len(payload.Results) != 2 || payload.Results[1].Status != "duplicate" || payload.Results[1].Detail != "flag already submitted." {
		t.Fatalf("unexpected submit payload %+v", payload.Results)
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
	setTestTeamAuthHeader(t, request)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", response.Code)
	}

	var payload httpapi.ProblemDetails
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode pre-start submit response: %v", err)
	}
	if payload.Detail != "contest has not started yet." {
		t.Fatalf("unexpected message %q", payload.Detail)
	}
}

func TestSubmitPrefersSubmissionServiceWhenConfigured(t *testing.T) {
	mux := newTestMuxWithWorkers(
		testGameCoreClient{
			submit: []SubmissionVerdictAlias{
				{Flag: "FLAGv1.core", Status: "accepted", Detail: "flag is correct."},
			},
		},
		testSubmissionClient{
			submit: []SubmissionVerdictAlias{
				{Flag: "FLAGv1.worker", Status: "accepted", Detail: "submission-service accepted the flag."},
			},
		},
		noopScoringClient{},
	)

	request := httptest.NewRequest(http.MethodPost, "/api/v2/submit", bytes.NewBufferString(`{"flags":["FLAGv1.worker"]}`))
	setTestTeamAuthHeader(t, request)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}

	var payload submissionResult
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode submit response: %v", err)
	}
	if len(payload.Results) != 1 || payload.Results[0].Status != "accepted" || payload.Results[0].Detail != "submission-service accepted the flag." {
		t.Fatalf("unexpected submit payload %+v", payload.Results)
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
	setTestTeamAuthHeader(t, request)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", response.Code)
	}

	var payload httpapi.ProblemDetails
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode finished submit response: %v", err)
	}
	if payload.Detail != "contest is over." {
		t.Fatalf("unexpected message %q", payload.Detail)
	}
}
