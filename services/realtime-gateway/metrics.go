package main

import (
	"fmt"
	"io"
	"time"
)

func (g *realtimeGateway) WritePrometheusMetrics(w io.Writer) {
	g.mu.RLock()
	subscriberCounts := map[streamKind]int{
		streamScoreboard:      len(g.subscribers[streamScoreboard]),
		streamAttacks:         len(g.subscribers[streamAttacks]),
		streamGameStatus:      len(g.subscribers[streamGameStatus]),
		streamSchedulerEvents: len(g.subscribers[streamSchedulerEvents]),
		streamCheckerRuns:     len(g.subscribers[streamCheckerRuns]),
	}
	snapshotBytes := map[streamKind]int{
		streamScoreboard:      len(g.scoreboard),
		streamAttacks:         len(g.attacks),
		streamGameStatus:      len(g.gameStatus),
		streamSchedulerEvents: len(g.schedulerEvents),
		streamCheckerRuns:     len(g.checkerRuns),
	}
	lastSyncAt := g.lastSyncAt
	lastSyncSuccessful := g.lastSyncSuccessful
	syncErrorsTotal := g.syncErrorsTotal
	g.mu.RUnlock()

	fmt.Fprintln(w, "# HELP adplatform_realtime_gateway_poll_interval_seconds Configured realtime poll interval in seconds.")
	fmt.Fprintln(w, "# TYPE adplatform_realtime_gateway_poll_interval_seconds gauge")
	fmt.Fprintf(w, "adplatform_realtime_gateway_poll_interval_seconds %.6f\n", g.pollInterval.Seconds())

	fmt.Fprintln(w, "# HELP adplatform_realtime_gateway_subscribers Current SSE subscribers by stream.")
	fmt.Fprintln(w, "# TYPE adplatform_realtime_gateway_subscribers gauge")
	for _, kind := range []streamKind{streamScoreboard, streamAttacks, streamGameStatus, streamSchedulerEvents, streamCheckerRuns} {
		fmt.Fprintf(w, "adplatform_realtime_gateway_subscribers{stream=%q} %d\n", string(kind), subscriberCounts[kind])
	}

	fmt.Fprintln(w, "# HELP adplatform_realtime_gateway_snapshot_bytes Current cached snapshot size in bytes by stream.")
	fmt.Fprintln(w, "# TYPE adplatform_realtime_gateway_snapshot_bytes gauge")
	for _, kind := range []streamKind{streamScoreboard, streamAttacks, streamGameStatus, streamSchedulerEvents, streamCheckerRuns} {
		fmt.Fprintf(w, "adplatform_realtime_gateway_snapshot_bytes{stream=%q} %d\n", string(kind), snapshotBytes[kind])
	}

	fmt.Fprintln(w, "# HELP adplatform_realtime_gateway_last_sync_success Whether the last poll cycle succeeded.")
	fmt.Fprintln(w, "# TYPE adplatform_realtime_gateway_last_sync_success gauge")
	if lastSyncSuccessful {
		fmt.Fprintln(w, "adplatform_realtime_gateway_last_sync_success 1")
	} else {
		fmt.Fprintln(w, "adplatform_realtime_gateway_last_sync_success 0")
	}

	fmt.Fprintln(w, "# HELP adplatform_realtime_gateway_sync_errors_total Total failed poll cycles.")
	fmt.Fprintln(w, "# TYPE adplatform_realtime_gateway_sync_errors_total counter")
	fmt.Fprintf(w, "adplatform_realtime_gateway_sync_errors_total %d\n", syncErrorsTotal)

	if !lastSyncAt.IsZero() {
		fmt.Fprintln(w, "# HELP adplatform_realtime_gateway_last_sync_unixtime Last poll cycle time as a Unix timestamp.")
		fmt.Fprintln(w, "# TYPE adplatform_realtime_gateway_last_sync_unixtime gauge")
		fmt.Fprintf(w, "adplatform_realtime_gateway_last_sync_unixtime %.0f\n", float64(lastSyncAt.Unix()))

		fmt.Fprintln(w, "# HELP adplatform_realtime_gateway_seconds_since_last_sync Seconds since the most recent poll cycle.")
		fmt.Fprintln(w, "# TYPE adplatform_realtime_gateway_seconds_since_last_sync gauge")
		fmt.Fprintf(w, "adplatform_realtime_gateway_seconds_since_last_sync %.6f\n", time.Since(lastSyncAt).Seconds())
	}
}
