package main

import (
	"context"
	"log"
	"sync"
	"time"

	"adplatform/internal/services/apigateway"
)

type matchWindowMonitor struct {
	cancel     context.CancelFunc
	autoTickWg sync.WaitGroup
}

func newMatchWindowMonitor(server *gameCoreServer, scheduler gameScheduler, interval time.Duration) *matchWindowMonitor {
	ctx, cancel := context.WithCancel(context.Background())
	monitor := &matchWindowMonitor{cancel: cancel}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}

			status, err := reconcileMatchWindowState(server, monitor)
			if err != nil {
				continue
			}

			schedulerStatus := scheduler.Status()
			switch {
			case status.State == "running" && matchWindowConfigured(status) && schedulerStatus.State != "running":
				if controller, ok := scheduler.(interface {
					StartWithSource(string, string) (apigateway.GameSchedulerStatus, error)
				}); ok {
					_, _ = controller.StartWithSource("schedule", "scheduler started by configured match window")
					continue
				}
				_, _ = scheduler.Start()
			case status.State != "running" && schedulerStatus.State == "running":
				if controller, ok := scheduler.(interface {
					StopWithSource(string, string) (apigateway.GameSchedulerStatus, error)
				}); ok {
					_, _ = controller.StopWithSource("schedule", "scheduler stopped by configured match window")
					continue
				}
				_, _ = scheduler.Stop()
			}
		}
	}()

	return monitor
}

func matchWindowConfigured(status apigateway.GameMatchStatus) bool {
	return status.ScheduleConfigured || status.ScheduledStartAt != "" || status.ScheduledEndAt != ""
}

func reconcileMatchWindowState(server *gameCoreServer, m *matchWindowMonitor) (apigateway.GameMatchStatus, error) {
	ctx := context.Background()

	current, err := server.store.MatchStatus(ctx)
	if err != nil {
		return apigateway.GameMatchStatus{}, err
	}
	desired := server.applyMatchWindow(current)

	switch {
	case current.State == desired.State:
	case desired.State == "running":
		startAt := parseOptionalRFC3339(desired.StartedAt)
		if startAt == nil {
			now := server.now().UTC()
			startAt = &now
		}
		if _, err := server.store.StartMatch(ctx, startAt.UTC()); err != nil && err != errContestOver {
			return apigateway.GameMatchStatus{}, err
		}
		// Best-effort auto-tick for scheduled matches: same rationale as the
		// one in handleStartMatch — without this, a match whose state flips
		// from not_started to running because ScheduledStartAt was reached
		// would still wait for the scheduler's first interval (default 5
		// min) before any tick-1 flag is plantable. We run the tick in a
		// tracked goroutine instead of synchronously because the monitor
		// loop polls every 1 s and a sync call here would block the
		// scheduler auto-start (the next monitor iteration) for the full
		// tick duration. The WaitGroup in matchWindowMonitor lets Close()
		// drain in-flight auto-ticks on shutdown.
		if server.autoTickOnMatchStart {
			m.autoTickWg.Add(1)
			go func() {
				defer m.autoTickWg.Done()
				tickCtx, cancel := context.WithCancel(context.Background())
				defer cancel()
				if _, tickErr := server.advanceTick(tickCtx); tickErr != nil {
					log.Printf("game-core: monitor auto-tick on scheduled start failed: %v", tickErr)
				}
			}()
		}
	case desired.State == "finished":
		if current.State == "not_started" {
			startAt := parseOptionalRFC3339(desired.StartedAt)
			if startAt != nil {
				if _, err := server.store.StartMatch(ctx, startAt.UTC()); err != nil && err != errContestOver {
					return apigateway.GameMatchStatus{}, err
				}
			}
		}

		endAt := parseOptionalRFC3339(desired.EndedAt)
		if endAt == nil {
			now := server.now().UTC()
			endAt = &now
		}
		if _, err := server.store.StopMatch(ctx, endAt.UTC()); err != nil {
			return apigateway.GameMatchStatus{}, err
		}
	}

	return server.matchStatus(ctx)
}

func (m *matchWindowMonitor) Close() error {
	if m.cancel != nil {
		m.cancel()
	}
	m.autoTickWg.Wait()
	return nil
}
