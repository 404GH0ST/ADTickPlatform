package main

// Code generated for tests from the gameStore interface; every method fails.

import (
	"context"
	"errors"
	"time"

	"adplatform/internal/services/apigateway"
)

var errGameStoreDown = errors.New("simulated game store outage")

type failingGameStore struct{}

var _ gameStore = failingGameStore{}

func (failingGameStore) ListCheckerTargets(ctx context.Context) (r0 []checkerTarget, err error) {
	err = errGameStoreDown
	return
}

func (failingGameStore) LockCheckerTarget(ctx context.Context, teamID, challengeID int) (r0 func(), err error) {
	err = errGameStoreDown
	return
}

func (failingGameStore) CheckerTargetCurrent(ctx context.Context, target checkerTarget) (r0 bool, err error) {
	err = errGameStoreDown
	return
}

func (failingGameStore) CheckerTargetConcurrencyLimit() (r0 int) {
	return
}

func (failingGameStore) IsChallengeInMaintenance(ctx context.Context, challengeID int) (r0 bool, err error) {
	err = errGameStoreDown
	return
}

func (failingGameStore) LookupTeamName(ctx context.Context, teamID int) (r0 string, err error) {
	err = errGameStoreDown
	return
}

func (failingGameStore) StartNextTick(ctx context.Context, now time.Time) (r0 apigateway.GameTickStatus, err error) {
	err = errGameStoreDown
	return
}

func (failingGameStore) RecordCheckerRun(ctx context.Context, run checkerRunRecord) (r0 apigateway.GameCheckerRun, err error) {
	err = errGameStoreDown
	return
}

func (failingGameStore) IssueFlag(ctx context.Context, flag issuedFlagRecord) (err error) {
	err = errGameStoreDown
	return
}

func (failingGameStore) LookupIssuedFlag(ctx context.Context, flag string) (r0 issuedFlagRecord, err error) {
	err = errGameStoreDown
	return
}

func (failingGameStore) LatestIssuedFlag(context.Context, int, int) (r0 issuedFlagRecord, err error) {
	err = errGameStoreDown
	return
}

func (failingGameStore) ScheduleRetentionCheck(context.Context, string, time.Time) (err error) {
	err = errGameStoreDown
	return
}

func (failingGameStore) RecordRetentionCheck(context.Context, issuedFlagRecord, checkerRunRecord, bool) (r0 bool, err error) {
	err = errGameStoreDown
	return
}

func (failingGameStore) AcceptFlagSubmission(ctx context.Context, submission acceptedFlagSubmission) (r0 bool, err error) {
	err = errGameStoreDown
	return
}

func (failingGameStore) RecordSubmission(context.Context, apigateway.AdminSubmission) (err error) {
	err = errGameStoreDown
	return
}

func (failingGameStore) ListSubmissions(context.Context, apigateway.SubmissionQuery) (r0 apigateway.SubmissionPage, err error) {
	err = errGameStoreDown
	return
}

func (failingGameStore) DeleteSubmission(context.Context, int64) (r0 apigateway.SubmissionDeletion, err error) {
	err = errGameStoreDown
	return
}

func (failingGameStore) ListScoreboard(ctx context.Context) (r0 []apigateway.ScoreRowAlias, err error) {
	err = errGameStoreDown
	return
}

func (failingGameStore) RecomputeScoreboard(ctx context.Context) (r0 []apigateway.ScoreRowAlias, err error) {
	err = errGameStoreDown
	return
}

func (failingGameStore) AuditScoreboard(ctx context.Context) (r0 apigateway.ScoringAuditAlias, err error) {
	err = errGameStoreDown
	return
}

func (failingGameStore) ListAttackFeed(ctx context.Context, query apigateway.AttackFeedQuery) (r0 apigateway.AttackFeedPage, err error) {
	err = errGameStoreDown
	return
}

func (failingGameStore) MatchStatus(ctx context.Context) (r0 apigateway.GameMatchStatus, err error) {
	err = errGameStoreDown
	return
}

func (failingGameStore) StartMatch(ctx context.Context, now time.Time) (r0 apigateway.GameMatchStatus, err error) {
	err = errGameStoreDown
	return
}

func (failingGameStore) PauseMatch(ctx context.Context, now time.Time) (r0 apigateway.GameMatchStatus, err error) {
	err = errGameStoreDown
	return
}

func (failingGameStore) ResumeMatch(ctx context.Context, now time.Time) (r0 apigateway.GameMatchStatus, err error) {
	err = errGameStoreDown
	return
}

func (failingGameStore) StopMatch(ctx context.Context, now time.Time) (r0 apigateway.GameMatchStatus, err error) {
	err = errGameStoreDown
	return
}

func (failingGameStore) UpdateMatchSchedule(ctx context.Context, startAt, endAt *time.Time) (r0 apigateway.GameMatchStatus, err error) {
	err = errGameStoreDown
	return
}

func (failingGameStore) CompleteTick(ctx context.Context, tick apigateway.GameTickStatus) (r0 apigateway.GameTickStatus, err error) {
	err = errGameStoreDown
	return
}

func (failingGameStore) ReapRunningTicks(ctx context.Context) (r0 int, err error) {
	err = errGameStoreDown
	return
}

func (failingGameStore) GameStatus(ctx context.Context) (r0 apigateway.GameStatus, err error) {
	err = errGameStoreDown
	return
}

func (failingGameStore) ListCheckerRuns(ctx context.Context, query apigateway.GameCheckerRunQuery) (r0 apigateway.GameCheckerRunPage, err error) {
	err = errGameStoreDown
	return
}

func (failingGameStore) LoadSchedulerState(ctx context.Context, intervalSeconds int) (r0 apigateway.GameSchedulerStatus, err error) {
	err = errGameStoreDown
	return
}

func (failingGameStore) SaveSchedulerState(ctx context.Context, status apigateway.GameSchedulerStatus, now time.Time) (err error) {
	err = errGameStoreDown
	return
}

func (failingGameStore) AppendSchedulerEvent(ctx context.Context, event schedulerEventRecord) (r0 apigateway.GameSchedulerEvent, err error) {
	err = errGameStoreDown
	return
}

func (failingGameStore) ListSchedulerEvents(ctx context.Context, query apigateway.GameSchedulerEventQuery) (r0 apigateway.GameSchedulerEventPage, err error) {
	err = errGameStoreDown
	return
}

func (failingGameStore) Close() (err error) {
	err = errGameStoreDown
	return
}
