package main

// Code generated for tests from the gameStore interface: delegates to inner,
// failing the call its counter is armed for.

import (
	"context"
	"sync"
	"time"

	"adplatform/internal/services/apigateway"
)

type callFaults struct {
	mu     sync.Mutex
	count  int
	failAt int
}

func (f *callFaults) arm(n int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.count, f.failAt = 0, n
}

func (f *callFaults) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.count
}

func (f *callFaults) fail() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.count++
	return f.failAt > 0 && f.count == f.failAt
}

type faultingGameStore struct {
	inner  gameStore
	faults *callFaults
}

var _ gameStore = faultingGameStore{}

func (s faultingGameStore) ListCheckerTargets(ctx context.Context) (r0 []checkerTarget, err error) {
	if s.faults.fail() {
		err = errGameStoreDown
		return
	}
	return s.inner.ListCheckerTargets(ctx)
}

func (s faultingGameStore) LockCheckerTarget(ctx context.Context, teamID, challengeID int) (r0 func(), err error) {
	if s.faults.fail() {
		err = errGameStoreDown
		return
	}
	return s.inner.LockCheckerTarget(ctx, teamID, challengeID)
}

func (s faultingGameStore) CheckerTargetCurrent(ctx context.Context, target checkerTarget) (r0 bool, err error) {
	if s.faults.fail() {
		err = errGameStoreDown
		return
	}
	return s.inner.CheckerTargetCurrent(ctx, target)
}

func (s faultingGameStore) CheckerTargetConcurrencyLimit() (r0 int) {
	return s.inner.CheckerTargetConcurrencyLimit()
}

func (s faultingGameStore) IsChallengeInMaintenance(ctx context.Context, challengeID int) (r0 bool, err error) {
	if s.faults.fail() {
		err = errGameStoreDown
		return
	}
	return s.inner.IsChallengeInMaintenance(ctx, challengeID)
}

func (s faultingGameStore) LookupTeamName(ctx context.Context, teamID int) (r0 string, err error) {
	if s.faults.fail() {
		err = errGameStoreDown
		return
	}
	return s.inner.LookupTeamName(ctx, teamID)
}

func (s faultingGameStore) StartNextTick(ctx context.Context, now time.Time) (r0 apigateway.GameTickStatus, err error) {
	if s.faults.fail() {
		err = errGameStoreDown
		return
	}
	return s.inner.StartNextTick(ctx, now)
}

func (s faultingGameStore) RecordCheckerRun(ctx context.Context, run checkerRunRecord) (r0 apigateway.GameCheckerRun, err error) {
	if s.faults.fail() {
		err = errGameStoreDown
		return
	}
	return s.inner.RecordCheckerRun(ctx, run)
}

func (s faultingGameStore) IssueFlag(ctx context.Context, flag issuedFlagRecord) (err error) {
	if s.faults.fail() {
		err = errGameStoreDown
		return
	}
	return s.inner.IssueFlag(ctx, flag)
}

func (s faultingGameStore) LookupIssuedFlag(ctx context.Context, flag string) (r0 issuedFlagRecord, err error) {
	if s.faults.fail() {
		err = errGameStoreDown
		return
	}
	return s.inner.LookupIssuedFlag(ctx, flag)
}

func (s faultingGameStore) LatestIssuedFlag(a0 context.Context, a1 int, a2 int) (r0 issuedFlagRecord, err error) {
	if s.faults.fail() {
		err = errGameStoreDown
		return
	}
	return s.inner.LatestIssuedFlag(a0, a1, a2)
}

func (s faultingGameStore) ScheduleRetentionCheck(a0 context.Context, a1 string, a2 time.Time) (err error) {
	if s.faults.fail() {
		err = errGameStoreDown
		return
	}
	return s.inner.ScheduleRetentionCheck(a0, a1, a2)
}

func (s faultingGameStore) RecordRetentionCheck(a0 context.Context, a1 issuedFlagRecord, a2 checkerRunRecord, a3 bool) (r0 bool, err error) {
	if s.faults.fail() {
		err = errGameStoreDown
		return
	}
	return s.inner.RecordRetentionCheck(a0, a1, a2, a3)
}

func (s faultingGameStore) AcceptFlagSubmission(ctx context.Context, submission acceptedFlagSubmission) (r0 bool, err error) {
	if s.faults.fail() {
		err = errGameStoreDown
		return
	}
	return s.inner.AcceptFlagSubmission(ctx, submission)
}

func (s faultingGameStore) RecordSubmissions(a0 context.Context, a1 ...apigateway.AdminSubmission) (err error) {
	if s.faults.fail() {
		err = errGameStoreDown
		return
	}
	return s.inner.RecordSubmissions(a0, a1...)
}

func (s faultingGameStore) PruneRejectedSubmissions(ctx context.Context, cutoff time.Time) error {
	if s.faults.fail() {
		return errGameStoreDown
	}
	return s.inner.PruneRejectedSubmissions(ctx, cutoff)
}

func (s faultingGameStore) ListSubmissions(a0 context.Context, a1 apigateway.SubmissionQuery) (r0 apigateway.SubmissionPage, err error) {
	if s.faults.fail() {
		err = errGameStoreDown
		return
	}
	return s.inner.ListSubmissions(a0, a1)
}

func (s faultingGameStore) DeleteSubmission(a0 context.Context, a1 int64) (r0 apigateway.SubmissionDeletion, err error) {
	if s.faults.fail() {
		err = errGameStoreDown
		return
	}
	return s.inner.DeleteSubmission(a0, a1)
}

func (s faultingGameStore) ListScoreboard(ctx context.Context) (r0 []apigateway.ScoreRowAlias, err error) {
	if s.faults.fail() {
		err = errGameStoreDown
		return
	}
	return s.inner.ListScoreboard(ctx)
}

func (s faultingGameStore) RecomputeScoreboard(ctx context.Context) (r0 []apigateway.ScoreRowAlias, err error) {
	if s.faults.fail() {
		err = errGameStoreDown
		return
	}
	return s.inner.RecomputeScoreboard(ctx)
}

func (s faultingGameStore) AuditScoreboard(ctx context.Context) (r0 apigateway.ScoringAuditAlias, err error) {
	if s.faults.fail() {
		err = errGameStoreDown
		return
	}
	return s.inner.AuditScoreboard(ctx)
}

func (s faultingGameStore) ListAttackFeed(ctx context.Context, query apigateway.AttackFeedQuery) (r0 apigateway.AttackFeedPage, err error) {
	if s.faults.fail() {
		err = errGameStoreDown
		return
	}
	return s.inner.ListAttackFeed(ctx, query)
}

func (s faultingGameStore) MatchStatus(ctx context.Context) (r0 apigateway.GameMatchStatus, err error) {
	if s.faults.fail() {
		err = errGameStoreDown
		return
	}
	return s.inner.MatchStatus(ctx)
}

func (s faultingGameStore) StartMatch(ctx context.Context, now time.Time) (r0 apigateway.GameMatchStatus, err error) {
	if s.faults.fail() {
		err = errGameStoreDown
		return
	}
	return s.inner.StartMatch(ctx, now)
}

func (s faultingGameStore) RestartMatch(ctx context.Context, now time.Time) (r0 apigateway.GameMatchStatus, err error) {
	if s.faults.fail() {
		err = errGameStoreDown
		return
	}
	return s.inner.RestartMatch(ctx, now)
}

func (s faultingGameStore) PauseMatch(ctx context.Context, now time.Time) (r0 apigateway.GameMatchStatus, err error) {
	if s.faults.fail() {
		err = errGameStoreDown
		return
	}
	return s.inner.PauseMatch(ctx, now)
}

func (s faultingGameStore) ResumeMatch(ctx context.Context, now time.Time) (r0 apigateway.GameMatchStatus, err error) {
	if s.faults.fail() {
		err = errGameStoreDown
		return
	}
	return s.inner.ResumeMatch(ctx, now)
}

func (s faultingGameStore) StopMatch(ctx context.Context, now time.Time) (r0 apigateway.GameMatchStatus, err error) {
	if s.faults.fail() {
		err = errGameStoreDown
		return
	}
	return s.inner.StopMatch(ctx, now)
}

func (s faultingGameStore) UpdateMatchSchedule(ctx context.Context, startAt, endAt *time.Time) (r0 apigateway.GameMatchStatus, err error) {
	if s.faults.fail() {
		err = errGameStoreDown
		return
	}
	return s.inner.UpdateMatchSchedule(ctx, startAt, endAt)
}

func (s faultingGameStore) CompleteTick(ctx context.Context, tick apigateway.GameTickStatus) (r0 apigateway.GameTickStatus, err error) {
	if s.faults.fail() {
		err = errGameStoreDown
		return
	}
	return s.inner.CompleteTick(ctx, tick)
}

func (s faultingGameStore) ReapRunningTicks(ctx context.Context) (r0 int, err error) {
	if s.faults.fail() {
		err = errGameStoreDown
		return
	}
	return s.inner.ReapRunningTicks(ctx)
}

func (s faultingGameStore) GameStatus(ctx context.Context) (r0 apigateway.GameStatus, err error) {
	if s.faults.fail() {
		err = errGameStoreDown
		return
	}
	return s.inner.GameStatus(ctx)
}

func (s faultingGameStore) ListCheckerRuns(ctx context.Context, query apigateway.GameCheckerRunQuery) (r0 apigateway.GameCheckerRunPage, err error) {
	if s.faults.fail() {
		err = errGameStoreDown
		return
	}
	return s.inner.ListCheckerRuns(ctx, query)
}

func (s faultingGameStore) LoadSchedulerState(ctx context.Context, intervalSeconds int) (r0 apigateway.GameSchedulerStatus, err error) {
	if s.faults.fail() {
		err = errGameStoreDown
		return
	}
	return s.inner.LoadSchedulerState(ctx, intervalSeconds)
}

func (s faultingGameStore) SaveSchedulerState(ctx context.Context, status apigateway.GameSchedulerStatus, now time.Time) (err error) {
	if s.faults.fail() {
		err = errGameStoreDown
		return
	}
	return s.inner.SaveSchedulerState(ctx, status, now)
}

func (s faultingGameStore) AppendSchedulerEvent(ctx context.Context, event schedulerEventRecord) (r0 apigateway.GameSchedulerEvent, err error) {
	if s.faults.fail() {
		err = errGameStoreDown
		return
	}
	return s.inner.AppendSchedulerEvent(ctx, event)
}

func (s faultingGameStore) ListSchedulerEvents(ctx context.Context, query apigateway.GameSchedulerEventQuery) (r0 apigateway.GameSchedulerEventPage, err error) {
	if s.faults.fail() {
		err = errGameStoreDown
		return
	}
	return s.inner.ListSchedulerEvents(ctx, query)
}

func (s faultingGameStore) Close() (err error) {
	if s.faults.fail() {
		err = errGameStoreDown
		return
	}
	return s.inner.Close()
}
