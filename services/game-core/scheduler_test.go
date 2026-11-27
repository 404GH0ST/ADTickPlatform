package main

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"adplatform/internal/services/apigateway"
)

func TestIntervalGameSchedulerRunsAndStops(t *testing.T) {
	var count atomic.Int32
	store := newMemoryGameStore()
	if _, err := store.StartMatch(context.Background(), time.Now()); err != nil {
		t.Fatalf("failed to start match: %v", err)
	}

	scheduler := newIntervalGameScheduler(store, 5*time.Millisecond, false, func(context.Context) (apigateway.GameTickStatus, error) {
		id := count.Add(1)
		return apigateway.GameTickStatus{ID: int(id), Status: "completed"}, nil
	}, store.MatchStatus)
	defer scheduler.Close()

	status, err := scheduler.Start()
	if err != nil {
		t.Fatalf("failed to start scheduler: %v", err)
	}
	if status.State != "running" {
		t.Fatalf("expected running state, got %+v", status)
	}

	deadline := time.Now().Add(250 * time.Millisecond)
	for scheduler.Status().LastTickID == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	status = scheduler.Status()
	if status.LastTickID == 0 || status.LastRunAt == "" {
		t.Fatalf("expected scheduler status to record runs, got %+v", status)
	}

	_, err = scheduler.Stop()
	if err != nil {
		t.Fatalf("failed to stop scheduler: %v", err)
	}

	stopped := scheduler.Status()
	if stopped.State != "stopped" || stopped.NextRunAt != "" {
		t.Fatalf("expected stopped scheduler status, got %+v", stopped)
	}
}

func TestIntervalGameSchedulerRestoresRunningState(t *testing.T) {
	store := newMemoryGameStore()
	if _, err := store.StartMatch(context.Background(), time.Now()); err != nil {
		t.Fatalf("failed to start match: %v", err)
	}
	var firstCount atomic.Int32

	scheduler := newIntervalGameScheduler(store, 5*time.Millisecond, false, func(context.Context) (apigateway.GameTickStatus, error) {
		id := firstCount.Add(1)
		return apigateway.GameTickStatus{ID: int(id), Status: "completed"}, nil
	}, store.MatchStatus)
	status, err := scheduler.Start()
	if err != nil {
		t.Fatalf("failed to start scheduler: %v", err)
	}
	if status.State != "running" {
		t.Fatalf("expected running state, got %+v", status)
	}

	scheduler.Close()

	var restoreCount atomic.Int32
	restored := newIntervalGameScheduler(store, 5*time.Millisecond, false, func(context.Context) (apigateway.GameTickStatus, error) {
		id := restoreCount.Add(1)
		return apigateway.GameTickStatus{ID: int(id), Status: "completed"}, nil
	}, store.MatchStatus)
	defer restored.Close()

	deadline := time.Now().Add(250 * time.Millisecond)
	for restoreCount.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if restoreCount.Load() == 0 {
		t.Fatal("expected restored scheduler to resume running ticks")
	}

	events, err := restored.Events(context.Background(), apigateway.GameSchedulerEventQuery{Limit: 4})
	if err != nil {
		t.Fatalf("failed to load scheduler events: %v", err)
	}
	if len(events.Items) == 0 {
		t.Fatal("expected persisted scheduler events")
	}
	if events.TotalCount == 0 {
		t.Fatalf("expected scheduler event page metadata, got %+v", events)
	}
	if events.Items[0].EventType != "tick_completed" && events.Items[0].EventType != "started" {
		t.Fatalf("unexpected latest scheduler event %+v", events.Items[0])
	}
	foundRestore := false
	for _, event := range events.Items {
		if event.EventType == "started" && event.Source == "restore" {
			foundRestore = true
			break
		}
	}
	if !foundRestore {
		t.Fatalf("expected restore start event, got %+v", events)
	}
}

func TestIntervalGameSchedulerRequiresRunningMatch(t *testing.T) {
	store := newMemoryGameStore()
	scheduler := newIntervalGameScheduler(store, 5*time.Millisecond, false, func(context.Context) (apigateway.GameTickStatus, error) {
		return apigateway.GameTickStatus{ID: 1, Status: "completed"}, nil
	}, store.MatchStatus)
	defer scheduler.Close()

	if _, err := scheduler.Start(); err != errContestNotStarted {
		t.Fatalf("expected errContestNotStarted, got %v", err)
	}
}

// If reading the match state fails once while restoring a running scheduler
// at boot, the scheduler must not report "running" with no loop behind it;
// it retries and resumes ticking.
func TestIntervalGameSchedulerRetriesFailedRestore(t *testing.T) {
	previousDelay := schedulerRestoreRetryDelay
	schedulerRestoreRetryDelay = 10 * time.Millisecond
	defer func() { schedulerRestoreRetryDelay = previousDelay }()

	store := newMemoryGameStore()
	if err := store.SaveSchedulerState(context.Background(), apigateway.GameSchedulerStatus{State: "running", IntervalSeconds: 1}, time.Now()); err != nil {
		t.Fatal(err)
	}
	var matchCalls atomic.Int32
	ticked := make(chan struct{}, 1)
	scheduler := newIntervalGameScheduler(store, 5*time.Millisecond, false,
		func(context.Context) (apigateway.GameTickStatus, error) {
			select {
			case ticked <- struct{}{}:
			default:
			}
			return apigateway.GameTickStatus{ID: 1}, nil
		},
		func(context.Context) (apigateway.GameMatchStatus, error) {
			if matchCalls.Add(1) == 1 {
				return apigateway.GameMatchStatus{}, errors.New("connection reset by peer")
			}
			return apigateway.GameMatchStatus{State: "running"}, nil
		})
	defer scheduler.Close()

	if got := scheduler.Status().State; got != "stopped" {
		t.Fatalf("failed restore reported state %q, want stopped until retried", got)
	}
	select {
	case <-ticked:
	case <-time.After(2 * time.Second):
		t.Fatal("scheduler never resumed ticking after a transient restore failure")
	}
	if got := scheduler.Status().State; got != "running" {
		t.Fatalf("state after successful retry = %q", got)
	}
}

// A restore refused because the match is not running stops the scheduler
// outright instead of reporting "running" with no loop.
func TestIntervalGameSchedulerRestoreStopsWhenMatchPaused(t *testing.T) {
	store := newMemoryGameStore()
	if err := store.SaveSchedulerState(context.Background(), apigateway.GameSchedulerStatus{State: "running", IntervalSeconds: 1}, time.Now()); err != nil {
		t.Fatal(err)
	}
	scheduler := newIntervalGameScheduler(store, 5*time.Millisecond, false,
		func(context.Context) (apigateway.GameTickStatus, error) {
			t.Error("no tick may run while the match is paused")
			return apigateway.GameTickStatus{}, nil
		},
		func(context.Context) (apigateway.GameMatchStatus, error) {
			return apigateway.GameMatchStatus{State: "paused"}, nil
		})
	defer scheduler.Close()
	if got := scheduler.Status().State; got != "stopped" {
		t.Fatalf("state = %q, want stopped", got)
	}
	persisted, err := store.LoadSchedulerState(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.State != "stopped" {
		t.Fatalf("persisted state = %q, want stopped", persisted.State)
	}
}

// The scheduler stops itself when its tick ends the match (a scheduled end
// passing) or a tick is refused because the match is over, without waiting for
// the match monitor.
func TestIntervalGameSchedulerStopsWhenTheMatchFinishes(t *testing.T) {
	for _, refuse := range []bool{false, true} {
		store := newMemoryGameStore()
		if _, err := store.StartMatch(context.Background(), time.Now()); err != nil {
			t.Fatal(err)
		}
		var ticks atomic.Int32
		scheduler := newIntervalGameScheduler(store, 5*time.Millisecond, false, func(ctx context.Context) (apigateway.GameTickStatus, error) {
			id := ticks.Add(1)
			if id == 2 {
				if _, err := store.StopMatch(ctx, time.Now()); err != nil {
					return apigateway.GameTickStatus{}, err
				}
				if refuse {
					return apigateway.GameTickStatus{}, errContestOver
				}
			}
			return apigateway.GameTickStatus{ID: int(id), Status: "completed"}, nil
		}, store.MatchStatus)

		if _, err := scheduler.Start(); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(time.Second)
		for scheduler.Status().State == "running" && time.Now().Before(deadline) {
			time.Sleep(5 * time.Millisecond)
		}
		status := scheduler.Status()
		if status.State != "stopped" || status.NextRunAt != "" || status.LastError != "" {
			t.Fatalf("refuse=%v: the scheduler stops cleanly once the match finishes: %+v", refuse, status)
		}
		time.Sleep(30 * time.Millisecond)
		if got := ticks.Load(); got != 2 {
			t.Fatalf("refuse=%v: no tick runs after the match finished, got %d", refuse, got)
		}
		scheduler.Close()
	}
}
