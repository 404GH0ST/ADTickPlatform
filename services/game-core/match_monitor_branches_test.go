package main

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"adplatform/internal/services/apigateway"
)

func windowServer(store gameStore, start, end *time.Time) *gameCoreServer {
	server := newTickServer(store)
	server.matchStartAt, server.matchEndAt = start, end
	return server
}

func ago(d time.Duration) *time.Time   { at := time.Now().Add(-d); return &at }
func ahead(d time.Duration) *time.Time { at := time.Now().Add(d); return &at }

func TestScheduledStartRunsAndAutoTicks(t *testing.T) {
	store := newMemoryGameStore()
	server := windowServer(store, ago(time.Minute), ahead(time.Hour))
	server.autoTickOnMatchStart = true
	server.tickMu.Lock() // the auto tick finds a tick in progress and only logs
	monitor := &matchWindowMonitor{}
	status, err := reconcileMatchWindowState(server, monitor)
	if err != nil || status.State != "running" {
		t.Fatalf("a reached start time starts the match: %+v %v", status, err)
	}
	monitor.autoTickWg.Wait()
	server.tickMu.Unlock()

	// A corrupted stored start time falls back to now.
	corrupt := newMemoryGameStore().(*memoryGameStore)
	corrupt.match.StartedAt = "not-a-time"
	if status, err := reconcileMatchWindowState(windowServer(corrupt, ago(time.Minute), nil), &matchWindowMonitor{}); err != nil || status.State != "running" {
		t.Fatalf("corrupted start time: %+v %v", status, err)
	}
}

func TestScheduledEndFinishesTheMatch(t *testing.T) {
	// Not started and already past the end: start, then finish with retention.
	store := newMemoryGameStore()
	status, err := reconcileMatchWindowState(windowServer(store, ago(2*time.Hour), ago(time.Hour)), &matchWindowMonitor{})
	if err != nil || status.State != "finished" || status.StartedAt == "" {
		t.Fatalf("a passed window finishes the match: %+v %v", status, err)
	}
	corrupt := newMemoryGameStore().(*memoryGameStore)
	if _, err := corrupt.StartMatch(context.Background(), time.Now()); err != nil {
		t.Fatal(err)
	}
	corrupt.match.EndedAt = "not-a-time"
	if status, err := reconcileMatchWindowState(windowServer(corrupt, nil, ago(time.Minute)), &matchWindowMonitor{}); err != nil || status.State != "finished" {
		t.Fatalf("corrupted end time: %+v %v", status, err)
	}
}

func TestScheduledTransitionsSurfaceFailures(t *testing.T) {
	cases := map[string]struct {
		start, end *time.Time
		failAt     int
		access     bool
		warmup     bool
	}{
		"match status":        {ago(time.Minute), ahead(time.Hour), 1, false, false},
		"start":               {ago(time.Minute), ahead(time.Hour), 2, false, false},
		"warmup":              {ago(time.Minute), ahead(time.Hour), 2, false, true},
		"start before finish": {ago(2 * time.Hour), ago(time.Hour), 2, false, false},
		"finish":              {nil, ago(time.Hour), 2, false, false},
		"access after start":  {ago(time.Minute), ahead(time.Hour), 0, true, false},
		"access after finish": {nil, ago(time.Hour), 0, true, false},
	}
	for name, tc := range cases {
		mem := newMemoryGameStore()
		if name == "finish" || name == "access after finish" {
			if _, err := mem.StartMatch(context.Background(), time.Now()); err != nil {
				t.Fatal(err)
			}
		}
		faults := &callFaults{}
		server := windowServer(faultingGameStore{inner: mem, faults: faults}, tc.start, tc.end)
		server.warmupRequired = tc.warmup
		if tc.access {
			server.accessReconcile = &failingAccess{}
		}
		faults.arm(tc.failAt)
		if _, err := reconcileMatchWindowState(server, &matchWindowMonitor{}); err == nil {
			t.Errorf("%s: the failure must surface", name)
		}
	}

	// A steady running window with a failing access reconcile keeps retrying it.
	mem := newMemoryGameStore()
	if _, err := mem.StartMatch(context.Background(), time.Now()); err != nil {
		t.Fatal(err)
	}
	steady := windowServer(mem, ago(time.Minute), ahead(time.Hour))
	steady.accessReconcile = &failingAccess{}
	if _, err := reconcileMatchWindowState(steady, &matchWindowMonitor{}); err == nil {
		t.Fatal("a pending access reconcile failure must surface")
	}
}

func TestScheduledWarmupGate(t *testing.T) {
	server := windowServer(newMemoryGameStore(), ago(time.Minute), ahead(time.Hour))
	server.warmupRequired = true
	monitor := &matchWindowMonitor{lastWarmupAttemptAt: time.Now()}
	if err := monitor.passScheduledWarmup(context.Background(), server); err == nil {
		t.Fatal("a recent failed attempt delays the next warmup")
	}
	failing := windowServer(failingGameStore{}, nil, nil)
	if err := (&matchWindowMonitor{}).passScheduledWarmup(context.Background(), failing); err == nil {
		t.Fatal("a warmup error blocks the scheduled start")
	}
}

// plainScheduler has no *WithSource methods, so the monitor uses Start/Stop.
type plainScheduler struct {
	mu            sync.Mutex
	state         string
	starts, stops int
}

func (s *plainScheduler) Status() apigateway.GameSchedulerStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return apigateway.GameSchedulerStatus{State: s.state}
}
func (s *plainScheduler) Start() (apigateway.GameSchedulerStatus, error) {
	s.mu.Lock()
	s.starts++
	s.state = "running"
	s.mu.Unlock()
	return s.Status(), nil
}
func (s *plainScheduler) Stop() (apigateway.GameSchedulerStatus, error) {
	s.mu.Lock()
	s.stops++
	s.state = "stopped"
	s.mu.Unlock()
	return s.Status(), nil
}
func (s *plainScheduler) counts() (int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.starts, s.stops
}
func (s *plainScheduler) Update(time.Duration) (apigateway.GameSchedulerStatus, error) {
	return s.Status(), nil
}
func (s *plainScheduler) Events(context.Context, apigateway.GameSchedulerEventQuery) (apigateway.GameSchedulerEventPage, error) {
	return apigateway.GameSchedulerEventPage{}, nil
}
func (s *plainScheduler) Close() error { return nil }

func TestMatchMonitorLoopDrivesPlainSchedulers(t *testing.T) {
	store := newMemoryGameStore()
	server := windowServer(store, ago(time.Minute), ahead(time.Hour))
	scheduler := &plainScheduler{state: "stopped"}
	monitor := newMatchWindowMonitor(server, scheduler, 5*time.Millisecond)
	waitFor(t, "the window to start the scheduler", func() bool { starts, _ := scheduler.counts(); return starts > 0 })
	monitor.Close()

	if _, err := store.StopMatch(context.Background(), time.Now()); err != nil {
		t.Fatal(err)
	}
	stopper := &plainScheduler{state: "running"}
	monitor = newMatchWindowMonitor(windowServer(store, nil, nil), stopper, 5*time.Millisecond)
	waitFor(t, "a finished match to stop the scheduler", func() bool { _, stops := stopper.counts(); return stops > 0 })
	monitor.Close()

	broken := newMatchWindowMonitor(windowServer(failingGameStore{}, nil, nil), &plainScheduler{}, 5*time.Millisecond)
	time.Sleep(30 * time.Millisecond) // failures are logged at most every 30s
	broken.Close()
	m := &matchWindowMonitor{}
	m.logAccessReconcileError(errors.New("first"))
	first := m.lastAccessErrorLogAt
	m.logAccessReconcileError(errors.New("suppressed"))
	if !m.lastAccessErrorLogAt.Equal(first) {
		t.Fatal("repeated failures within 30s are not re-logged")
	}
}
