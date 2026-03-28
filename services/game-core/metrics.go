package main

import (
	"context"
	"fmt"
	"io"
	"time"
)

func (s *gameCoreServer) WritePrometheusMetrics(w io.Writer) {
	status, err := s.store.GameStatus(context.Background())
	if err != nil {
		fmt.Fprintln(w, "# HELP adplatform_game_core_metrics_collection_success Whether game-core metrics collection succeeded.")
		fmt.Fprintln(w, "# TYPE adplatform_game_core_metrics_collection_success gauge")
		fmt.Fprintln(w, "adplatform_game_core_metrics_collection_success 0")
		return
	}

	matchStatus, err := s.matchStatus(context.Background())
	if err == nil {
		status.Match = &matchStatus
	}
	if s.scheduler != nil {
		schedulerStatus := s.scheduler.Status()
		status.Scheduler = &schedulerStatus
	}

	fmt.Fprintln(w, "# HELP adplatform_game_core_metrics_collection_success Whether game-core metrics collection succeeded.")
	fmt.Fprintln(w, "# TYPE adplatform_game_core_metrics_collection_success gauge")
	fmt.Fprintln(w, "adplatform_game_core_metrics_collection_success 1")

	fmt.Fprintln(w, "# HELP adplatform_game_core_total_ticks Total ticks recorded by game-core.")
	fmt.Fprintln(w, "# TYPE adplatform_game_core_total_ticks gauge")
	fmt.Fprintf(w, "adplatform_game_core_total_ticks %d\n", status.TotalTicks)

	fmt.Fprintln(w, "# HELP adplatform_game_core_checker_runs_total Total checker runs grouped by outcome.")
	fmt.Fprintln(w, "# TYPE adplatform_game_core_checker_runs_total gauge")
	fmt.Fprintf(w, "adplatform_game_core_checker_runs_total{status=%q} %d\n", "all", status.TotalCheckerRuns)
	fmt.Fprintf(w, "adplatform_game_core_checker_runs_total{status=%q} %d\n", "successful", status.SuccessfulCheckerRuns)
	fmt.Fprintf(w, "adplatform_game_core_checker_runs_total{status=%q} %d\n", "failed", status.FailedCheckerRuns)
	fmt.Fprintf(w, "adplatform_game_core_checker_runs_total{status=%q} %d\n", "skipped", status.SkippedCheckerRuns)

	if status.Match != nil {
		fmt.Fprintln(w, "# HELP adplatform_game_core_match_accepting_submissions Whether the match is accepting submissions.")
		fmt.Fprintln(w, "# TYPE adplatform_game_core_match_accepting_submissions gauge")
		fmt.Fprintf(w, "adplatform_game_core_match_accepting_submissions %.0f\n", boolMetric(status.Match.AcceptingSubmissions))

		fmt.Fprintln(w, "# HELP adplatform_game_core_match_state Whether the match is currently in the given state.")
		fmt.Fprintln(w, "# TYPE adplatform_game_core_match_state gauge")
		for _, state := range []string{"not_started", "running", "finished"} {
			fmt.Fprintf(w, "adplatform_game_core_match_state{state=%q} %.0f\n", state, boolMetric(status.Match.State == state))
		}

		if startedAt := parseMetricTime(status.Match.StartedAt); !startedAt.IsZero() {
			fmt.Fprintln(w, "# HELP adplatform_game_core_match_started_at_unixtime Match start time as a Unix timestamp.")
			fmt.Fprintln(w, "# TYPE adplatform_game_core_match_started_at_unixtime gauge")
			fmt.Fprintf(w, "adplatform_game_core_match_started_at_unixtime %.0f\n", float64(startedAt.Unix()))
		}
		if endedAt := parseMetricTime(status.Match.EndedAt); !endedAt.IsZero() {
			fmt.Fprintln(w, "# HELP adplatform_game_core_match_ended_at_unixtime Match end time as a Unix timestamp.")
			fmt.Fprintln(w, "# TYPE adplatform_game_core_match_ended_at_unixtime gauge")
			fmt.Fprintf(w, "adplatform_game_core_match_ended_at_unixtime %.0f\n", float64(endedAt.Unix()))
		}
	}

	if status.CurrentTick != nil {
		fmt.Fprintln(w, "# HELP adplatform_game_core_current_tick_id Current tick identifier.")
		fmt.Fprintln(w, "# TYPE adplatform_game_core_current_tick_id gauge")
		fmt.Fprintf(w, "adplatform_game_core_current_tick_id %d\n", status.CurrentTick.ID)

		fmt.Fprintln(w, "# HELP adplatform_game_core_current_tick_running Whether the current tick is still running.")
		fmt.Fprintln(w, "# TYPE adplatform_game_core_current_tick_running gauge")
		fmt.Fprintf(w, "adplatform_game_core_current_tick_running %.0f\n", boolMetric(status.CurrentTick.Status == "running"))
	}

	if status.Scheduler != nil {
		fmt.Fprintln(w, "# HELP adplatform_game_core_scheduler_running Whether the scheduler is running.")
		fmt.Fprintln(w, "# TYPE adplatform_game_core_scheduler_running gauge")
		fmt.Fprintf(w, "adplatform_game_core_scheduler_running %.0f\n", boolMetric(status.Scheduler.State == "running"))

		fmt.Fprintln(w, "# HELP adplatform_game_core_scheduler_interval_seconds Configured scheduler interval in seconds.")
		fmt.Fprintln(w, "# TYPE adplatform_game_core_scheduler_interval_seconds gauge")
		fmt.Fprintf(w, "adplatform_game_core_scheduler_interval_seconds %d\n", status.Scheduler.IntervalSeconds)

		fmt.Fprintln(w, "# HELP adplatform_game_core_scheduler_last_tick_id Most recent tick completed by the scheduler.")
		fmt.Fprintln(w, "# TYPE adplatform_game_core_scheduler_last_tick_id gauge")
		fmt.Fprintf(w, "adplatform_game_core_scheduler_last_tick_id %d\n", status.Scheduler.LastTickID)

		if lastRunAt := parseMetricTime(status.Scheduler.LastRunAt); !lastRunAt.IsZero() {
			fmt.Fprintln(w, "# HELP adplatform_game_core_scheduler_last_run_unixtime Last scheduler run time as a Unix timestamp.")
			fmt.Fprintln(w, "# TYPE adplatform_game_core_scheduler_last_run_unixtime gauge")
			fmt.Fprintf(w, "adplatform_game_core_scheduler_last_run_unixtime %.0f\n", float64(lastRunAt.Unix()))
		}
		if nextRunAt := parseMetricTime(status.Scheduler.NextRunAt); !nextRunAt.IsZero() {
			now := time.Now().UTC()
			fmt.Fprintln(w, "# HELP adplatform_game_core_scheduler_next_run_unixtime Next scheduled run time as a Unix timestamp.")
			fmt.Fprintln(w, "# TYPE adplatform_game_core_scheduler_next_run_unixtime gauge")
			fmt.Fprintf(w, "adplatform_game_core_scheduler_next_run_unixtime %.0f\n", float64(nextRunAt.Unix()))

			fmt.Fprintln(w, "# HELP adplatform_game_core_scheduler_seconds_until_next_run Seconds until the next scheduled run; negative values mean overdue.")
			fmt.Fprintln(w, "# TYPE adplatform_game_core_scheduler_seconds_until_next_run gauge")
			fmt.Fprintf(w, "adplatform_game_core_scheduler_seconds_until_next_run %.6f\n", nextRunAt.Sub(now).Seconds())
		}

		fmt.Fprintln(w, "# HELP adplatform_game_core_scheduler_last_error Whether the scheduler currently reports a last_error value.")
		fmt.Fprintln(w, "# TYPE adplatform_game_core_scheduler_last_error gauge")
		fmt.Fprintf(w, "adplatform_game_core_scheduler_last_error %.0f\n", boolMetric(status.Scheduler.LastError != ""))
	}
}

func boolMetric(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

func parseMetricTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return parsed.UTC()
}

var _ interface{ WritePrometheusMetrics(io.Writer) } = (*gameCoreServer)(nil)
