package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math/big"
	mrand "math/rand/v2"
	"sync"
	"time"

	"adplatform/internal/services/apigateway"
)

const (
	missingStoredFlagMessage    = "confirmed missing stored flag; tick SLA forfeited and severe defense penalty"
	retentionCheckerUnavailable = "flag retention checker execution unavailable"
	retentionVerificationFailed = "stored flag failed retention verification"
)

func (s *gameCoreServer) lockRetentionTarget(t checkerTarget) func() {
	value, _ := s.retentionLocks.LoadOrStore([2]int{t.TeamID, t.ChallengeID}, &sync.Mutex{})
	lock := value.(*sync.Mutex)
	lock.Lock()
	return lock.Unlock
}
func (s *gameCoreServer) nextRetentionCheck() time.Time {
	low, high := 30*time.Second, 90*time.Second
	if interval := s.scheduler.Status().IntervalSeconds; interval > 0 {
		low = time.Duration(interval) * time.Second / 8
		high = time.Duration(interval) * time.Second / 3
	}
	// high > low for any positive interval (1/3 exceeds 1/8), and crypto/rand
	// never returns an error, so the jitter draw cannot fail.
	jitter, _ := rand.Int(rand.Reader, big.NewInt(int64(high-low)))
	return s.now().Add(low + time.Duration(jitter.Int64()))
}

// checkFlagRetention runs the checker under execCtx and persists under ctx.
// A tick deadline cancels execCtx only: the interrupted check has no verdict, so
// nothing is recorded and the tick keeps going. Cancelling ctx itself (shutdown,
// or a caller passing one context for both) is still returned as an error.
func (s *gameCoreServer) checkFlagRetention(ctx, execCtx context.Context, target checkerTarget, flag issuedFlagRecord) error {
	if flag.CheckerToken == "" || flag.CheckerToken != target.CheckerToken {
		return nil
	}
	timeout := time.Duration(s.checkerTimeout)*time.Second + 5*time.Second
	if timeout <= 5*time.Second {
		timeout = 20 * time.Second
	}
	checkCtx, cancel := context.WithTimeout(execCtx, timeout)
	defer cancel()
	result, execErr := s.checker.Execute(checkCtx, apigateway.CheckerExecutionRequest{
		ChallengeID: target.ChallengeID, TeamID: target.TeamID, TeamName: target.TeamName, ChallengeName: target.ChallengeName,
		CheckerImage: target.CheckerImage, Phase: "get", Target: target.Target, TargetHost: target.TargetHost, TargetIP: target.TargetIP, TargetPort: target.TargetPort,
		TickID: flag.IssuedTick, Flag: flag.Flag, Metadata: flag.Metadata, CheckerToken: target.CheckerToken, TimeoutSeconds: s.checkerTimeout,
	})
	if execCtx.Err() != nil {
		return ctx.Err()
	}

	_, err := s.recordFlagRetention(ctx, target, flag, apigateway.CheckerPhaseResult{
		Status: result.Status, ExitCode: result.ExitCode, Output: result.Output, CheckedAt: result.CheckedAt,
	}, execErr)
	return err
}

func (s *gameCoreServer) recordFlagRetention(ctx context.Context, target checkerTarget, flag issuedFlagRecord, result apigateway.CheckerPhaseResult, execErr error) (bool, error) {
	checkedAt, err := time.Parse(time.RFC3339, result.CheckedAt)
	if err != nil {
		checkedAt = s.now()
	}
	run := checkerRunRecord{TickID: flag.IssuedTick, TeamID: target.TeamID, TeamName: target.TeamName, ChallengeID: target.ChallengeID, ChallengeName: target.ChallengeName,
		CheckerImage: target.CheckerImage, Phase: "get", Target: target.Target, CheckedAt: checkedAt.UTC(), Status: normalizedCheckerRunStatus(result.Status), ExitCode: result.ExitCode}
	confirmed := execErr == nil && run.Status == "failed" && result.ExitCode == flagMissingExitCode
	if execErr != nil {
		run.Status = "failed"
		run.ExitCode = -1
		run.Message = retentionCheckerUnavailable
	} else if run.Status != "success" {
		run.Status = "failed"
		run.Message = retentionVerificationFailed
	} else {
		run.Output = result.Output
		run.Message = "stored flag passed retention verification"
	}
	if confirmed {
		run.Message = missingStoredFlagMessage
	}
	applied, err := s.store.RecordRetentionCheck(ctx, flag, run, confirmed)
	if err != nil {
		return false, err
	}
	if applied {
		s.scheduleScoreboardRecompute()
	}
	return applied, s.store.ScheduleRetentionCheck(ctx, flag.Flag, s.nextRetentionCheck())
}

// force is used before match completion. A pre-rotation check is also performed
// under the same target mutex before every new put, regardless of its due time.
func (s *gameCoreServer) verifyRetainedFlags(ctx context.Context, force bool) error {
	_, err := s.sweepRetainedFlags(ctx, ctx, force)
	return err
}

// retentionSweep reports how far a sweep got before execCtx ended.
type retentionSweep struct {
	Dispatched, Targets int
	Complete            bool
}

// sweepRetainedFlags runs checkers under execCtx and persists under ctx. When
// execCtx ends first the sweep stops dispatching, records nothing for checks it
// interrupted, and reports Complete=false instead of an error.
func (s *gameCoreServer) sweepRetainedFlags(ctx, execCtx context.Context, force bool) (retentionSweep, error) {
	var sweep retentionSweep
	status, err := s.store.GameStatus(ctx)
	if err != nil {
		return sweep, err
	}
	if status.CurrentTick == nil || (!force && status.CurrentTick.Status != "completed") {
		sweep.Complete = true
		return sweep, nil
	}
	targets, err := s.store.ListCheckerTargets(ctx)
	if err != nil {
		return sweep, err
	}
	if force {
		// A budget-limited final sweep must not always leave the same targets
		// (the highest team/challenge ids) unchecked.
		mrand.Shuffle(len(targets), func(i, j int) { targets[i], targets[j] = targets[j], targets[i] })
	}
	sweep.Targets = len(targets)
	parallel := s.checkerParallelism
	if parallel < 1 {
		parallel = 1
	}
	jobs := make(chan checkerTarget)
	var wg sync.WaitGroup
	var firstErr error
	var errorsMu sync.Mutex
	for i := 0; i < parallel; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Dispatch stops when execCtx ends; a job delivered on that
			// boundary finds execCtx expired, so its check records nothing.
			for target := range jobs {
				unlock := s.lockRetentionTarget(target)
				flag, err := s.store.LatestIssuedFlag(ctx, target.TeamID, target.ChallengeID)
				if errors.Is(err, sql.ErrNoRows) {
					err = nil
				} else if err == nil && flag.IssuedTick == status.CurrentTick.ID && flag.CheckerToken != "" && flag.CheckerToken == target.CheckerToken && (force || !flag.NextCheckAt.After(s.now())) {
					err = s.checkFlagRetention(ctx, execCtx, target, flag)
				}
				unlock()
				if err != nil {
					errorsMu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					errorsMu.Unlock()
				}
			}
		}()
	}
dispatch:
	for _, target := range targets {
		select {
		case jobs <- target:
			sweep.Dispatched++
		case <-execCtx.Done():
			break dispatch
		}
	}
	close(jobs)
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return sweep, err
	}
	sweep.Complete = execCtx.Err() == nil
	return sweep, firstErr
}

func (s *gameCoreServer) startRetentionMonitor() func() {
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		interval := s.retentionMonitorInterval
		if interval <= 0 {
			interval = 5 * time.Second
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			match, err := s.store.MatchStatus(ctx)
			if err != nil || match.State != "running" {
				continue
			}
			if err = s.verifyRetainedFlags(ctx, false); err != nil && ctx.Err() == nil {
				log.Printf("flag retention verification: %v", err)
			}
		}
	}()
	return func() { cancel(); wg.Wait() }
}

// finalRetentionSweepBudget caps the forced retention sweep run when a match is
// stopped. Its cost grows with targets x checker latency / parallelism, so on a
// large or degraded event it cannot always finish; the stop must still happen.
const defaultFinalRetentionSweepBudget = 3 * time.Minute

// matchStopTimeout bounds an organizer stop: the wait for an in-flight tick, the
// final retention sweep, and a margin for the recompute and state change.
func (s *gameCoreServer) matchStopTimeout() time.Duration {
	return max(s.tickTimeout, 0) + s.finalRetentionSweepBudget + 5*time.Minute
}

func (s *gameCoreServer) stopMatchWithRetention(ctx context.Context, now time.Time) (apigateway.GameMatchStatus, error) {
	s.tickMu.Lock()
	defer s.tickMu.Unlock()
	match, err := s.store.MatchStatus(ctx)
	if err != nil {
		return apigateway.GameMatchStatus{}, err
	}
	if match.State == "running" {
		sweepCtx, cancel := context.WithTimeout(ctx, s.finalRetentionSweepBudget)
		sweep, err := s.sweepRetainedFlags(ctx, sweepCtx, true)
		cancel()
		if err != nil {
			return apigateway.GameMatchStatus{}, fmt.Errorf("final flag retention verification: %w", err)
		}
		if !sweep.Complete {
			// Checks that finished are recorded. Stopping anyway beats a match
			// that can never end because every retry restarts the full sweep.
			message := fmt.Sprintf("final flag retention verification ran out of its %s budget after %d of %d targets; stopping the match anyway", s.finalRetentionSweepBudget, sweep.Dispatched, sweep.Targets)
			log.Print(message)
			if _, err := s.store.AppendSchedulerEvent(ctx, schedulerEventRecord{EventType: "final_retention_incomplete", Source: "match", State: match.State, Message: message, CreatedAt: s.now().UTC()}); err != nil {
				log.Printf("record incomplete final retention event: %v", err)
			}
		}
	}
	if _, err := s.store.RecomputeScoreboard(ctx); err != nil {
		return apigateway.GameMatchStatus{}, err
	}
	return s.store.StopMatch(ctx, now)
}
