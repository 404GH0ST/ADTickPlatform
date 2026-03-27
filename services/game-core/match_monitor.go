package main

import (
	"context"
	"time"

	"adplatform/internal/services/apigateway"
)

type matchWindowMonitor struct {
	cancel context.CancelFunc
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

			status, err := reconcileMatchWindowState(server)
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

func reconcileMatchWindowState(server *gameCoreServer) (apigateway.GameMatchStatus, error) {
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
	return nil
}
