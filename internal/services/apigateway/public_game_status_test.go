package apigateway

import (
	"strings"
	"testing"
)

// The public game status is unauthenticated and routed straight through the
// edge proxy. Scheduler and tick diagnostics carry raw internal err.Error()
// strings (database errors, upstream failures), which belong only on the
// organizer endpoint.
func TestSanitizePublicGameStatusStripsInternalDiagnostics(t *testing.T) {
	status := GameStatus{
		CurrentTick: &GameTickStatus{ID: 7, Status: "failed", Message: `pq: relation "checker_targets" does not exist at 10.0.3.4:5432`},
		Scheduler: &GameSchedulerStatus{
			State:           "running",
			IntervalSeconds: 60,
			NextRunAt:       "2026-11-19T10:00:00Z",
			LastTickID:      6,
			LastError:       "flag issuance persistence failed: dial tcp 10.0.3.4:5432: connect: connection refused",
		},
	}

	public := sanitizePublicGameStatus(status)

	if public.Scheduler == nil || public.Scheduler.LastError != "" {
		t.Fatalf("public scheduler must drop last_error, got %+v", public.Scheduler)
	}
	if public.Scheduler.State != "running" || public.Scheduler.IntervalSeconds != 60 || public.Scheduler.NextRunAt == "" || public.Scheduler.LastTickID != 6 {
		t.Fatalf("public scheduler must keep timing fields participants use, got %+v", public.Scheduler)
	}
	if public.CurrentTick == nil || public.CurrentTick.Message != "" {
		t.Fatalf("public tick must drop its internal message, got %+v", public.CurrentTick)
	}
	if public.CurrentTick.ID != 7 || public.CurrentTick.Status != "failed" {
		t.Fatalf("public tick must keep id and status, got %+v", public.CurrentTick)
	}
	if status.Scheduler.LastError == "" || status.CurrentTick.Message == "" {
		t.Fatal("sanitizePublicGameStatus mutated the organizer status")
	}
}

func TestSanitizePublicGameStatusHandlesEmptyStatus(t *testing.T) {
	public := sanitizePublicGameStatus(GameStatus{TotalTicks: 3})
	if public.Scheduler != nil || public.CurrentTick != nil || public.Match != nil || public.TotalTicks != 3 {
		t.Fatalf("empty status should pass through unchanged, got %+v", public)
	}
}

// A warmup that failed on the database must not expose the raw error (an
// internal address, SQL text) on the public status endpoint.
func TestPublicWarmupMessageNeverCarriesRawErrors(t *testing.T) {
	raw := "failed to list checker targets: dial tcp 10.0.0.5:5432: connect: connection refused"
	for _, status := range []string{"failed", "passed", "running"} {
		public := sanitizePublicGameStatus(GameStatus{Match: &GameMatchStatus{State: "running", Warmup: &GameWarmupResult{Status: status, Message: raw}}})
		if message := public.Match.Warmup.Message; strings.Contains(message, "5432") || strings.Contains(message, "dial tcp") {
			t.Errorf("%s warmup leaked %q", status, message)
		}
	}
}
