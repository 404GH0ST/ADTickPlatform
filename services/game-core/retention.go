package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math/big"
	"sync"
	"time"

	"adplatform/internal/services/apigateway"
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
	if high <= low {
		high = low + time.Millisecond
	}
	jitter, err := rand.Int(rand.Reader, big.NewInt(int64(high-low)))
	if err != nil {
		return s.now().Add(low)
	}
	return s.now().Add(low + time.Duration(jitter.Int64()))
}

func (s *gameCoreServer) checkFlagRetention(ctx context.Context, target checkerTarget, flag issuedFlagRecord) error {
	if flag.CheckerToken == "" || flag.CheckerToken != target.CheckerToken {
		return nil
	}
	timeout := time.Duration(s.checkerTimeout)*time.Second + 5*time.Second
	if timeout <= 5*time.Second {
		timeout = 20 * time.Second
	}
	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	result, execErr := s.checker.Execute(checkCtx, apigateway.CheckerExecutionRequest{
		ChallengeID: target.ChallengeID, TeamID: target.TeamID, TeamName: target.TeamName, ChallengeName: target.ChallengeName,
		CheckerImage: target.CheckerImage, Phase: "get", Target: target.Target, TargetHost: target.TargetHost, TargetIP: target.TargetIP, TargetPort: target.TargetPort,
		TickID: flag.IssuedTick, Flag: flag.Flag, Metadata: flag.Metadata, CheckerToken: target.CheckerToken, TimeoutSeconds: s.checkerTimeout,
	})
	if ctx.Err() != nil {
		return ctx.Err()
	}

	run := checkerRunRecord{TickID: flag.IssuedTick, TeamID: target.TeamID, TeamName: target.TeamName, ChallengeID: target.ChallengeID, ChallengeName: target.ChallengeName,
		CheckerImage: target.CheckerImage, Phase: "get", Target: target.Target, CheckedAt: s.now().UTC(), Status: normalizedCheckerRunStatus(result.Status), ExitCode: result.ExitCode, Output: result.Output, Message: "stored flag passed retention verification"}
	confirmed := execErr == nil && run.Status == "failed" && result.ExitCode == flagMissingExitCode
	if execErr != nil {
		run.Status = "failed"
		run.ExitCode = -1
		run.Message = "flag retention checker execution unavailable"
	} else if run.Status != "success" {
		run.Status = "failed"
		run.Message = "stored flag failed retention verification"
	}
	if confirmed {
		run.Message = "confirmed missing stored flag; tick SLA forfeited and severe defense penalty"
	}
	applied, err := s.store.RecordRetentionCheck(ctx, flag, run, confirmed)
	if err != nil {
		return err
	}
	if applied {
		s.scheduleScoreboardRecompute()
	}
	return s.store.ScheduleRetentionCheck(ctx, flag.Flag, s.nextRetentionCheck())
}

// force is used before match completion. A pre-rotation check is also performed
// under the same target mutex before every new put, regardless of its due time.
func (s *gameCoreServer) verifyRetainedFlags(ctx context.Context, force bool) error {
	status, err := s.store.GameStatus(ctx)
	if err != nil {
		return err
	}
	if status.CurrentTick == nil {
		return nil
	}
	if !force && status.CurrentTick.Status != "completed" {
		return nil
	}
	targets, err := s.store.ListCheckerTargets(ctx)
	if err != nil {
		return err
	}
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
			for target := range jobs {
				unlock := s.lockRetentionTarget(target)
				flag, err := s.store.LatestIssuedFlag(ctx, target.TeamID, target.ChallengeID)
				if errors.Is(err, sql.ErrNoRows) {
					err = nil
				} else if err == nil && flag.IssuedTick == status.CurrentTick.ID && flag.CheckerToken != "" && flag.CheckerToken == target.CheckerToken && (force || !flag.NextCheckAt.After(s.now())) {
					err = s.checkFlagRetention(ctx, target, flag)
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
	for _, target := range targets {
		select {
		case jobs <- target:
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return ctx.Err()
		}
	}
	close(jobs)
	wg.Wait()
	return firstErr
}

func (s *gameCoreServer) startRetentionMonitor() func() {
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(5 * time.Second)
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
func (s *gameCoreServer) stopMatchWithRetention(ctx context.Context, now time.Time) (apigateway.GameMatchStatus, error) {
	s.tickMu.Lock()
	defer s.tickMu.Unlock()
	match, err := s.store.MatchStatus(ctx)
	if err != nil {
		return apigateway.GameMatchStatus{}, err
	}
	if match.State == "running" {
		if err := s.verifyRetainedFlags(ctx, true); err != nil {
			return apigateway.GameMatchStatus{}, fmt.Errorf("final flag retention verification: %w", err)
		}
	}
	if _, err := s.store.RecomputeScoreboard(ctx); err != nil {
		return apigateway.GameMatchStatus{}, err
	}
	return s.store.StopMatch(ctx, now)
}
