package main

import (
	"context"
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
	for count.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if count.Load() == 0 {
		t.Fatal("expected scheduler to advance at least one tick")
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
