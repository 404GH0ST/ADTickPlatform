package main

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"adplatform/internal/services/apigateway"
)

type gameScheduler interface {
	Status() apigateway.GameSchedulerStatus
	Start() (apigateway.GameSchedulerStatus, error)
	Stop() (apigateway.GameSchedulerStatus, error)
	Update(interval time.Duration) (apigateway.GameSchedulerStatus, error)
	Events(ctx context.Context, query apigateway.GameSchedulerEventQuery) (apigateway.GameSchedulerEventPage, error)
	Close() error
}

type tickAdvanceFunc func(context.Context) (apigateway.GameTickStatus, error)
type matchStatusFunc func(context.Context) (apigateway.GameMatchStatus, error)

type intervalGameScheduler struct {
	mu       sync.Mutex
	store    gameStore
	interval time.Duration
	advance  tickAdvanceFunc
	match    matchStatusFunc
	now      func() time.Time
	cancel   context.CancelFunc
	status   apigateway.GameSchedulerStatus
	// restoring is true while a background retry of a failed start-up restore
	// is pending. Any explicit Start, Stop, or Close cancels the retry.
	restoring bool
	// retryDelay and retryAttempts are copied from the package defaults when
	// the scheduler is built, so a running retry loop never reads shared state.
	retryDelay    time.Duration
	retryAttempts int
}

// Restoring a running scheduler reads the match state; if that read fails
// transiently at boot, retry in the background instead of leaving a scheduler
// that reports "running" with no tick loop behind it.
var (
	schedulerRestoreRetryDelay = 5 * time.Second
	schedulerRestoreAttempts   = 24
)

func newIntervalGameScheduler(store gameStore, interval time.Duration, autoStart bool, advance tickAdvanceFunc, match matchStatusFunc) gameScheduler {
	scheduler := &intervalGameScheduler{
		store:    store,
		interval: interval,
		advance:  advance,
		match:    match,
		now:      time.Now,
		// Copied so a background retry never reads the mutable package defaults.
		retryDelay:    schedulerRestoreRetryDelay,
		retryAttempts: schedulerRestoreAttempts,
		status: apigateway.GameSchedulerStatus{
			State:           "stopped",
			IntervalSeconds: int(interval / time.Second),
		},
	}

	if store != nil {
		if persisted, err := store.LoadSchedulerState(context.Background(), int(interval/time.Second)); err == nil {
			scheduler.status = persisted
		}
	}

	scheduler.mu.Lock()
	switch {
	case scheduler.status.State == "running":
		if _, err := scheduler.startLocked("restore", "scheduler state restored on process start"); err != nil {
			scheduler.restoreFailedLocked(err)
		}
	case autoStart:
		_, _ = scheduler.startLocked("config", "scheduler started from configuration")
	}
	scheduler.mu.Unlock()

	return scheduler
}

func (s *intervalGameScheduler) Status() apigateway.GameSchedulerStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

func (s *intervalGameScheduler) Start() (apigateway.GameSchedulerStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.restoring = false
	return s.startLocked("organizer", "scheduler started by organizer")
}

func (s *intervalGameScheduler) StartWithSource(source, message string) (apigateway.GameSchedulerStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.restoring = false
	return s.startLocked(source, message)
}

func isMatchStateError(err error) bool {
	return errors.Is(err, errContestOver) || errors.Is(err, errContestPaused) || errors.Is(err, errContestNotStarted)
}

// restoreFailedLocked handles a failed start-up restore. startLocked started
// no loop, so the scheduler must not keep reporting "running": the match
// monitor only starts a scheduler that reports otherwise.
func (s *intervalGameScheduler) restoreFailedLocked(err error) {
	s.status.State = "stopped"
	s.status.NextRunAt = ""
	s.status.LastError = "scheduler restore failed: " + strings.TrimSpace(err.Error())
	if isMatchStateError(err) {
		// The match is not running, so the scheduler genuinely should not run.
		s.recordRestoreStoppedLocked()
		return
	}
	// Transient failure: leave the persisted "running" state so a restart
	// still restores, and retry in the background.
	s.restoring = true
	go s.retryRestore()
}

func (s *intervalGameScheduler) recordRestoreStoppedLocked() {
	now := s.now().UTC()
	s.persistLocked(now)
	s.appendEventLocked(now, schedulerEventRecord{
		EventType: "stopped",
		Source:    "restore",
		State:     s.status.State,
		Message:   s.status.LastError,
	})
}

func (s *intervalGameScheduler) retryRestore() {
	for attempt := 0; attempt < s.retryAttempts; attempt++ {
		time.Sleep(s.retryDelay)
		s.mu.Lock()
		if !s.restoring {
			s.mu.Unlock()
			return
		}
		_, err := s.startLocked("restore", "scheduler state restored after retry")
		if err == nil {
			s.restoring = false
			s.mu.Unlock()
			return
		}
		s.status.LastError = "scheduler restore failed: " + strings.TrimSpace(err.Error())
		if isMatchStateError(err) {
			s.restoring = false
			s.recordRestoreStoppedLocked()
			s.mu.Unlock()
			return
		}
		s.mu.Unlock()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.restoring {
		s.restoring = false
		s.recordRestoreStoppedLocked()
	}
}

func (s *intervalGameScheduler) startLocked(source, message string) (apigateway.GameSchedulerStatus, error) {
	if s.status.State == "running" && s.cancel != nil {
		return s.status, nil
	}
	if s.match != nil {
		match, err := s.match(context.Background())
		if err != nil {
			return s.status, err
		}
		switch match.State {
		case "finished":
			return s.status, errContestOver
		case "paused":
			return s.status, errContestPaused
		case "running":
		default:
			return s.status, errContestNotStarted
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	now := s.now().UTC()
	s.status.State = "running"
	s.status.IntervalSeconds = int(s.interval / time.Second)
	s.status.NextRunAt = now.Add(s.interval).Format(time.RFC3339)
	s.status.LastError = ""
	s.persistLocked(now)
	s.appendEventLocked(now, schedulerEventRecord{
		EventType: "started",
		Source:    source,
		State:     s.status.State,
		Message:   message,
	})
	go s.run(ctx, s.interval)
	return s.status, nil
}

func (s *intervalGameScheduler) Stop() (apigateway.GameSchedulerStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.restoring = false
	return s.stopLocked("organizer", "scheduler stopped by organizer")
}

func (s *intervalGameScheduler) StopWithSource(source, message string) (apigateway.GameSchedulerStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.restoring = false
	return s.stopLocked(source, message)
}

func (s *intervalGameScheduler) stopLocked(source, message string) (apigateway.GameSchedulerStatus, error) {
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	now := s.now().UTC()
	s.status.State = "stopped"
	s.status.IntervalSeconds = int(s.interval / time.Second)
	s.status.NextRunAt = ""
	s.persistLocked(now)
	s.appendEventLocked(now, schedulerEventRecord{
		EventType: "stopped",
		Source:    source,
		State:     s.status.State,
		Message:   message,
	})
	return s.status, nil
}

func (s *intervalGameScheduler) Update(interval time.Duration) (apigateway.GameSchedulerStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if interval < time.Second {
		return s.status, nil
	}

	s.interval = interval
	s.status.IntervalSeconds = int(s.interval / time.Second)

	now := s.now().UTC()

	// If it's running, update the NextRunAt and restart the internal timer
	if s.status.State == "running" {
		if s.cancel != nil {
			s.cancel()
		}
		s.status.NextRunAt = now.Add(s.interval).Format(time.RFC3339)
		ctx, cancel := context.WithCancel(context.Background())
		s.cancel = cancel
		go s.run(ctx, s.interval)
	}

	s.persistLocked(now)
	s.appendEventLocked(now, schedulerEventRecord{
		EventType: "updated",
		Source:    "organizer",
		State:     s.status.State,
		Message:   "scheduler interval updated",
	})

	return s.status, nil
}

func (s *intervalGameScheduler) Events(ctx context.Context, query apigateway.GameSchedulerEventQuery) (apigateway.GameSchedulerEventPage, error) {
	if s.store == nil {
		return apigateway.GameSchedulerEventPage{}, nil
	}
	return s.store.ListSchedulerEvents(ctx, query)
}

func (s *intervalGameScheduler) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.restoring = false
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	return nil
}

// run ticks every interval until ctx ends. The interval is captured under the
// lock when the loop starts: Update replaces the loop rather than changing
// s.interval underneath it, which would race with this goroutine.
func (s *intervalGameScheduler) run(ctx context.Context, interval time.Duration) {
	timer := time.NewTimer(interval)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}

		tickStart := s.now().UTC()
		tickCtx, tickCancel := context.WithCancel(ctx)
		tick, err := s.advance(tickCtx)
		tickCancel()
		now := s.now().UTC()

		elapsed := now.Sub(tickStart)
		nextWait := interval - elapsed
		if nextWait < 0 {
			nextWait = 0
		}

		s.mu.Lock()
		if ctx.Err() != nil || s.status.State != "running" {
			s.mu.Unlock()
			return
		}
		s.status.LastRunAt = now.Format(time.RFC3339)
		s.status.NextRunAt = now.Add(nextWait).Format(time.RFC3339)
		s.status.IntervalSeconds = int(s.interval / time.Second)
		if err != nil {
			s.status.LastError = strings.TrimSpace(err.Error())
			s.persistLocked(now)
			s.appendEventLocked(now, schedulerEventRecord{
				EventType: "tick_failed",
				Source:    "scheduler",
				State:     s.status.State,
				Message:   s.status.LastError,
			})
		} else {
			s.status.LastTickID = tick.ID
			s.status.LastError = ""
			s.persistLocked(now)
			s.appendEventLocked(now, schedulerEventRecord{
				EventType: "tick_completed",
				Source:    "scheduler",
				State:     s.status.State,
				TickID:    tick.ID,
				Message:   "scheduled tick completed successfully",
			})
		}
		s.mu.Unlock()

		timer.Reset(nextWait)
	}
}

func (s *intervalGameScheduler) persistLocked(now time.Time) {
	if s.store == nil {
		return
	}
	_ = s.store.SaveSchedulerState(context.Background(), s.status, now)
}

func (s *intervalGameScheduler) appendEventLocked(now time.Time, event schedulerEventRecord) {
	if s.store == nil {
		return
	}
	event.CreatedAt = now
	_, _ = s.store.AppendSchedulerEvent(context.Background(), event)
}
