package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"adplatform/internal/services/apigateway"
)

type monitorTestScheduler struct {
	mu     sync.Mutex
	status apigateway.GameSchedulerStatus
	starts int
	stops  int
}

func (s *monitorTestScheduler) Status() apigateway.GameSchedulerStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.status.State == "" {
		s.status = apigateway.GameSchedulerStatus{State: "stopped", IntervalSeconds: 60}
	}
	return s.status
}

func (s *monitorTestScheduler) Start() (apigateway.GameSchedulerStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.starts++
	s.status.State = "running"
	s.status.IntervalSeconds = 60
	return s.status, nil
}

func (s *monitorTestScheduler) StartWithSource(string, string) (apigateway.GameSchedulerStatus, error) {
	return s.Start()
}

func (s *monitorTestScheduler) Stop() (apigateway.GameSchedulerStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stops++
	s.status.State = "stopped"
	s.status.NextRunAt = ""
	return s.status, nil
}

func (s *monitorTestScheduler) StopWithSource(string, string) (apigateway.GameSchedulerStatus, error) {
	return s.Stop()
}

func (s *monitorTestScheduler) Update(interval time.Duration) (apigateway.GameSchedulerStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.IntervalSeconds = int(interval / time.Second)
	return s.status, nil
}

func (s *monitorTestScheduler) Events(context.Context, apigateway.GameSchedulerEventQuery) (apigateway.GameSchedulerEventPage, error) {
	return apigateway.GameSchedulerEventPage{}, nil
}

func (s *monitorTestScheduler) Close() error {
	return nil
}

func TestMatchWindowMonitorStartsAndStopsScheduler(t *testing.T) {
	store := newMemoryGameStore()
	scheduler := &monitorTestScheduler{}
	server := newGameCoreServer("dev-admin-token", store, testCheckerClient{}, newFlagCodec("test-flag-secret"), scheduler, []string{"put", "get", "check"}, 15)

	base := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)
	start := base.Add(20 * time.Millisecond)
	end := base.Add(40 * time.Millisecond)
	server.matchStartAt = &start
	server.matchEndAt = &end

	var mu sync.Mutex
	now := base
	server.now = func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return now
	}

	monitor := newMatchWindowMonitor(server, scheduler, 2*time.Millisecond)
	defer monitor.Close()

	mu.Lock()
	now = start.Add(time.Millisecond)
	mu.Unlock()
	waitForCondition(t, 250*time.Millisecond, func() bool {
		return scheduler.starts > 0 && scheduler.Status().State == "running"
	})
	match, err := store.MatchStatus(context.Background())
	if err != nil {
		t.Fatalf("failed to load persisted running match state: %v", err)
	}
	if match.State != "running" || match.StartedAt != start.Format(time.RFC3339) {
		t.Fatalf("expected persisted running match at %s, got %+v", start.Format(time.RFC3339), match)
	}

	mu.Lock()
	now = end.Add(time.Millisecond)
	mu.Unlock()
	waitForCondition(t, 250*time.Millisecond, func() bool {
		return scheduler.stops > 0 && scheduler.Status().State == "stopped"
	})
	match, err = store.MatchStatus(context.Background())
	if err != nil {
		t.Fatalf("failed to load persisted finished match state: %v", err)
	}
	if match.State != "finished" || match.EndedAt != end.Format(time.RFC3339) {
		t.Fatalf("expected persisted finished match at %s, got %+v", end.Format(time.RFC3339), match)
	}
}

func TestMatchWindowMonitorDoesNotAutoStartWithoutConfiguredWindow(t *testing.T) {
	store := newMemoryGameStore()
	scheduler := &monitorTestScheduler{}
	server := newGameCoreServer("dev-admin-token", store, testCheckerClient{}, newFlagCodec("test-flag-secret"), scheduler, []string{"put", "get", "check"}, 15)

	base := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)
	server.now = func() time.Time { return base.Add(time.Minute) }

	if _, err := store.StartMatch(context.Background(), base); err != nil {
		t.Fatalf("failed to start match: %v", err)
	}

	monitor := newMatchWindowMonitor(server, scheduler, 2*time.Millisecond)
	defer monitor.Close()

	time.Sleep(40 * time.Millisecond)

	if scheduler.starts != 0 {
		t.Fatalf("expected no auto-start without configured match window, got %d starts", scheduler.starts)
	}
	if scheduler.Status().State != "stopped" {
		t.Fatalf("expected scheduler to remain stopped, got %+v", scheduler.Status())
	}
}

func waitForCondition(t *testing.T, timeout time.Duration, predicate func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if predicate() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition was not satisfied before timeout")
}
