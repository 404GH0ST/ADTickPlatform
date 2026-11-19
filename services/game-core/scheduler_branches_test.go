package main

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"adplatform/internal/services/apigateway"
)

func fastRestoreRetries(t *testing.T, attempts int) {
	t.Helper()
	delay, count := schedulerRestoreRetryDelay, schedulerRestoreAttempts
	schedulerRestoreRetryDelay, schedulerRestoreAttempts = 5*time.Millisecond, attempts
	t.Cleanup(func() { schedulerRestoreRetryDelay, schedulerRestoreAttempts = delay, count })
}

// scriptedMatch returns the queued results in order, repeating the last.
type scriptedMatch struct {
	mu      sync.Mutex
	results []error
	state   string
}

func (m *scriptedMatch) status(context.Context) (apigateway.GameMatchStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	err := m.results[0]
	if len(m.results) > 1 {
		m.results = m.results[1:]
	}
	return apigateway.GameMatchStatus{State: m.state}, err
}

func runningState(t *testing.T) gameStore {
	t.Helper()
	store := newMemoryGameStore()
	if err := store.SaveSchedulerState(context.Background(), apigateway.GameSchedulerStatus{State: "running", IntervalSeconds: 1}, time.Now()); err != nil {
		t.Fatal(err)
	}
	return store
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func noTick(context.Context) (apigateway.GameTickStatus, error) {
	return apigateway.GameTickStatus{}, nil
}

func TestSchedulerRestoreOutcomes(t *testing.T) {
	fastRestoreRetries(t, 3)

	restored := newIntervalGameScheduler(runningState(t), time.Hour, false, noTick, (&scriptedMatch{results: []error{nil}, state: "running"}).status)
	defer restored.Close()
	if restored.Status().State != "running" {
		t.Fatalf("a running scheduler is restored: %+v", restored.Status())
	}

	transient := newIntervalGameScheduler(runningState(t), time.Hour, false, noTick, (&scriptedMatch{results: []error{errors.New("db down"), nil}, state: "running"}).status)
	defer transient.Close()
	waitFor(t, "a transient restore failure to recover", func() bool { return transient.Status().State == "running" })

	notRunning := newIntervalGameScheduler(runningState(t), time.Hour, false, noTick, (&scriptedMatch{results: []error{nil}, state: "paused"}).status)
	if status := notRunning.Status(); status.State != "stopped" || status.LastError == "" {
		t.Fatalf("a paused match stops the restored scheduler: %+v", status)
	}

	becomesPaused := &scriptedMatch{results: []error{errors.New("db down")}, state: "running"}
	later := newIntervalGameScheduler(runningState(t), time.Hour, false, noTick, becomesPaused.status)
	becomesPaused.mu.Lock()
	becomesPaused.results, becomesPaused.state = []error{nil}, "finished"
	becomesPaused.mu.Unlock()
	waitFor(t, "a retry that finds the match over to stop", func() bool {
		s := later.(*intervalGameScheduler)
		s.mu.Lock()
		defer s.mu.Unlock()
		return !s.restoring
	})

	exhausted := newIntervalGameScheduler(runningState(t), time.Hour, false, noTick, (&scriptedMatch{results: []error{errors.New("db down")}, state: "running"}).status)
	waitFor(t, "retries to give up", func() bool {
		s := exhausted.(*intervalGameScheduler)
		s.mu.Lock()
		defer s.mu.Unlock()
		return !s.restoring
	})

	cancelled := newIntervalGameScheduler(runningState(t), time.Hour, false, noTick, (&scriptedMatch{results: []error{errors.New("db down")}, state: "running"}).status)
	if _, err := cancelled.Stop(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(30 * time.Millisecond) // the retry loop notices it was cancelled
}

func TestSchedulerRunsTicksAndRecordsOutcomes(t *testing.T) {
	store := newMemoryGameStore()
	var calls atomic.Int32
	advance := func(context.Context) (apigateway.GameTickStatus, error) {
		if calls.Add(1)%2 == 0 {
			return apigateway.GameTickStatus{}, errors.New("tick failed")
		}
		return apigateway.GameTickStatus{ID: int(calls.Load())}, nil
	}
	scheduler := newIntervalGameScheduler(store, 10*time.Millisecond, true, advance, (&scriptedMatch{results: []error{nil}, state: "running"}).status)
	waitFor(t, "ticks to run", func() bool { return calls.Load() >= 3 })
	if _, err := scheduler.Update(time.Second); err != nil {
		t.Fatal(err)
	}
	if status, _ := scheduler.Update(time.Millisecond); status.IntervalSeconds != 1 {
		t.Fatalf("sub-second intervals are ignored: %+v", status)
	}
	if _, err := scheduler.(*intervalGameScheduler).StopWithSource("test", "stopped by test"); err != nil {
		t.Fatal(err)
	}
	if _, err := scheduler.(*intervalGameScheduler).StartWithSource("test", "started by test"); err != nil {
		t.Fatal(err)
	}
	scheduler.Close()
	page, err := scheduler.Events(context.Background(), apigateway.GameSchedulerEventQuery{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, event := range page.Items {
		seen[event.EventType] = true
	}
	for _, want := range []string{"tick_completed", "tick_failed", "updated"} {
		if !seen[want] {
			t.Fatalf("missing %q event in %+v", want, page.Items)
		}
	}

	stopped := newIntervalGameScheduler(nil, time.Hour, false, noTick, nil)
	if status, err := stopped.Update(2 * time.Second); err != nil || status.IntervalSeconds != 2 {
		t.Fatalf("updating a stopped scheduler: %+v %v", status, err)
	}
	if page, err := stopped.Events(context.Background(), apigateway.GameSchedulerEventQuery{}); err != nil || len(page.Items) != 0 {
		t.Fatal("a store-less scheduler has no events")
	}
	if _, err := stopped.Start(); err != nil {
		t.Fatal(err)
	}
	if status, err := stopped.Start(); err != nil || status.State != "running" {
		t.Fatalf("starting twice is idempotent: %+v %v", status, err)
	}
	stopped.Close()

	over := newIntervalGameScheduler(nil, time.Hour, false, noTick, (&scriptedMatch{results: []error{nil}, state: "finished"}).status)
	if _, err := over.Start(); !errors.Is(err, errContestOver) {
		t.Fatalf("a finished match cannot be scheduled: %v", err)
	}
	failing := newIntervalGameScheduler(nil, time.Hour, false, noTick, (&scriptedMatch{results: []error{errors.New("x")}, state: "running"}).status)
	if _, err := failing.Start(); err == nil {
		t.Fatal("an unknown match state must not start the scheduler")
	}
}

func TestSchedulerLoopExitsWhenStoppedMidTick(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{}, 1)
	advance := func(context.Context) (apigateway.GameTickStatus, error) {
		select {
		case started <- struct{}{}:
		default:
		}
		<-release
		return apigateway.GameTickStatus{ID: 1}, nil
	}
	scheduler := newIntervalGameScheduler(newMemoryGameStore(), 5*time.Millisecond, true, advance, (&scriptedMatch{results: []error{nil}, state: "running"}).status)
	<-started
	if _, err := scheduler.Stop(); err != nil {
		t.Fatal(err)
	}
	close(release) // the tick finishes after the stop and must not be recorded
	time.Sleep(20 * time.Millisecond)
	if status := scheduler.Status(); status.LastTickID != 0 {
		t.Fatalf("a tick finishing after stop must not update status: %+v", status)
	}
}

func TestNoopScheduler(t *testing.T) {
	var s noopGameScheduler
	if s.Status().State != "stopped" || s.Close() != nil {
		t.Fatal("noop status")
	}
	if status, _ := s.Start(); status.State != "running" {
		t.Fatal("noop start")
	}
	if status, _ := s.Stop(); status.State != "stopped" {
		t.Fatal("noop stop")
	}
	if status, _ := s.Update(time.Second); status.State != "stopped" {
		t.Fatal("noop update")
	}
	if page, _ := s.Events(context.Background(), apigateway.GameSchedulerEventQuery{}); len(page.Items) != 0 {
		t.Fatal("noop events")
	}
}
