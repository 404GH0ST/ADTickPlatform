package main

import (
	"context"
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
}

func newIntervalGameScheduler(store gameStore, interval time.Duration, autoStart bool, advance tickAdvanceFunc, match matchStatusFunc) gameScheduler {
	scheduler := &intervalGameScheduler{
		store:    store,
		interval: interval,
		advance:  advance,
		match:    match,
		now:      time.Now,
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

	switch {
	case scheduler.status.State == "running":
		_, _ = scheduler.startLocked("restore", "scheduler state restored on process start")
	case autoStart:
		_, _ = scheduler.startLocked("config", "scheduler started from configuration")
	}

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
	return s.startLocked("organizer", "scheduler started by organizer")
}

func (s *intervalGameScheduler) StartWithSource(source, message string) (apigateway.GameSchedulerStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.startLocked(source, message)
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
	go s.run(ctx)
	return s.status, nil
}

func (s *intervalGameScheduler) Stop() (apigateway.GameSchedulerStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stopLocked("organizer", "scheduler stopped by organizer")
}

func (s *intervalGameScheduler) StopWithSource(source, message string) (apigateway.GameSchedulerStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
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
		go s.run(ctx)
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
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	return nil
}

func (s *intervalGameScheduler) run(ctx context.Context) {
	timer := time.NewTimer(s.interval)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}

		tick, err := s.advance(context.Background())
		now := s.now().UTC()

		s.mu.Lock()
		if ctx.Err() != nil || s.status.State != "running" {
			s.mu.Unlock()
			return
		}
		s.status.LastRunAt = now.Format(time.RFC3339)
		s.status.NextRunAt = now.Add(s.interval).Format(time.RFC3339)
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

		timer.Reset(s.interval)
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
