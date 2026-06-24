package main

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"adplatform/internal/services/apigateway"
)

func TestFaustDefensePenalty(t *testing.T) {
	tests := []struct {
		name         string
		captureCount int
		want         float64
	}{
		{name: "uncaptured flag has no penalty", captureCount: 0, want: 0},
		{name: "single capture subtracts one point", captureCount: 1, want: 1},
		{name: "multiple captures use exponent", captureCount: 2, want: math.Pow(2, 0.75)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := faustDefensePenalty(tt.captureCount); math.Abs(got-tt.want) > 0.000001 {
				t.Fatalf("faustDefensePenalty(%d) = %f, want %f", tt.captureCount, got, tt.want)
			}
		})
	}
}

func TestStartupRecomputeHonorsTimeout(t *testing.T) {
	err := recomputeScoreboardWithTimeout(context.Background(), time.Nanosecond, func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected startup recompute timeout, got %v", err)
	}
}

func TestFaustAttackValueUsesCaptureCount(t *testing.T) {
	tests := []struct {
		name         string
		captureCount int
		want         float64
	}{
		{name: "single capture gets full bonus", captureCount: 1, want: 2.0},
		{name: "second attacker still gets one and a half", captureCount: 2, want: 1.5},
		{name: "third attacker gets one and a third", captureCount: 3, want: 1.0 + (1.0 / 3.0)},
		{name: "invalid capture count stays zero", captureCount: 0, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := faustAttackValue(tt.captureCount); math.Abs(got-tt.want) > 0.000001 {
				t.Fatalf("faustAttackValue(%d) = %f, want %f", tt.captureCount, got, tt.want)
			}
		})
	}
}

func TestFaustSLAValueMapping(t *testing.T) {
	tests := []struct {
		name    string
		putOK   bool
		getOK   bool
		checkOK bool
		want    float64
	}{
		{name: "all phases successful is ok", putOK: true, getOK: true, checkOK: true, want: 1.0},
		{name: "get and check only is recovering", putOK: false, getOK: true, checkOK: true, want: 0.5},
		{name: "missing get is down", putOK: true, getOK: false, checkOK: true, want: 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := faustSLAValue(tt.putOK, tt.getOK, tt.checkOK); math.Abs(got-tt.want) > 0.000001 {
				t.Fatalf("faustSLAValue(%t, %t, %t) = %f, want %f", tt.putOK, tt.getOK, tt.checkOK, got, tt.want)
			}
		})
	}
}

func TestFaustSLAValueForStatus(t *testing.T) {
	tests := []struct {
		status string
		want   float64
	}{
		{status: "ok", want: 1.0},
		{status: "recovering", want: 0.5},
		{status: "flag_not_found", want: 0.0},
		{status: "faulty", want: 0.0},
		{status: "down", want: 0.0},
		{status: "unknown", want: 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			if got := faustSLAValueForStatus(tt.status); math.Abs(got-tt.want) > 0.000001 {
				t.Fatalf("faustSLAValueForStatus(%q) = %f, want %f", tt.status, got, tt.want)
			}
		})
	}
}

func TestMemoryStorePersistsCheckerServiceState(t *testing.T) {
	store, ok := newMemoryGameStore().(*memoryGameStore)
	if !ok {
		t.Fatal("expected concrete memory game store")
	}
	ctx := context.Background()

	runs := []checkerRunRecord{
		{
			TickID:        1,
			TeamID:        101,
			TeamName:      "Alpha",
			ChallengeID:   1,
			ChallengeName: "banking",
			Phase:         "put",
			Status:        "success",
		},
		{
			TickID:        1,
			TeamID:        101,
			TeamName:      "Alpha",
			ChallengeID:   1,
			ChallengeName: "banking",
			Phase:         "get",
			Status:        "failed",
			Message:       "flag retrieval failed",
		},
		{
			TickID:        1,
			TeamID:        101,
			TeamName:      "Alpha",
			ChallengeID:   1,
			ChallengeName: "banking",
			Phase:         "check",
			Status:        "skipped",
		},
	}
	for _, run := range runs {
		if _, err := store.RecordCheckerRun(ctx, run); err != nil {
			t.Fatalf("RecordCheckerRun: %v", err)
		}
	}

	summary := store.serviceStates[checkerRunGroupKey{TickID: 1, TeamID: 101, ChallengeID: 1}]
	if summary.Status != "flag_not_found" {
		t.Fatalf("expected stored service state flag_not_found, got %+v", summary)
	}

	page, err := store.ListCheckerRuns(ctx, apigateway.GameCheckerRunQuery{TeamID: 101, ChallengeID: 1})
	if err != nil {
		t.Fatalf("ListCheckerRuns: %v", err)
	}
	if len(page.Items) != 3 {
		t.Fatalf("expected 3 checker runs, got %d", len(page.Items))
	}
	if page.Items[0].ServiceState != "flag_not_found" {
		t.Fatalf("expected checker run page to expose stored service state, got %+v", page.Items[0])
	}
}

func TestMemoryStoreReapRunningTicks(t *testing.T) {
	store, ok := newMemoryGameStore().(*memoryGameStore)
	if !ok {
		t.Fatal("expected concrete memory game store")
	}
	ctx := context.Background()

	first, err := store.StartNextTick(ctx, time.Now())
	if err != nil {
		t.Fatalf("StartNextTick: %v", err)
	}
	first.Status = "completed"
	first.CompletedAt = time.Now().UTC().Format(time.RFC3339)
	if _, err := store.CompleteTick(ctx, first); err != nil {
		t.Fatalf("CompleteTick: %v", err)
	}

	// Second tick is left "running" to simulate a crash mid-tick.
	if _, err := store.StartNextTick(ctx, time.Now()); err != nil {
		t.Fatalf("StartNextTick (stuck): %v", err)
	}

	// A new tick must be refused while the stuck one is still running.
	if _, err := store.StartNextTick(ctx, time.Now()); !errors.Is(err, errTickInProgress) {
		t.Fatalf("expected errTickInProgress before reap, got %v", err)
	}

	reaped, err := store.ReapRunningTicks(ctx)
	if err != nil {
		t.Fatalf("ReapRunningTicks: %v", err)
	}
	if reaped != 1 {
		t.Fatalf("expected to reap 1 tick, got %d", reaped)
	}
	if store.ticks[1].Status != "failed" || store.ticks[1].Message != reapedTickMessage {
		t.Fatalf("expected stuck tick reaped to failed, got %+v", store.ticks[1])
	}
	if store.ticks[0].Status != "completed" {
		t.Fatalf("expected completed tick untouched, got %+v", store.ticks[0])
	}

	// Reaping again is a no-op, and a fresh tick is now allowed.
	if reaped, err := store.ReapRunningTicks(ctx); err != nil || reaped != 0 {
		t.Fatalf("expected idempotent reap, got reaped=%d err=%v", reaped, err)
	}
	if _, err := store.StartNextTick(ctx, time.Now()); err != nil {
		t.Fatalf("expected tick allowed after reap, got %v", err)
	}
}

func TestMemoryStoreAcceptFlagSubmissionRefreshesScoreboard(t *testing.T) {
	store, ok := newMemoryGameStore().(*memoryGameStore)
	if !ok {
		t.Fatal("expected concrete memory game store")
	}
	ctx := context.Background()
	issued := issuedFlagRecord{
		Flag:          "flag-1",
		OwnerTeamID:   102,
		OwnerTeamName: "Team Delta",
		ChallengeID:   1,
		ChallengeName: "banking",
		IssuedTick:    1,
		ExpiresTick:   1,
	}
	if err := store.IssueFlag(ctx, issued); err != nil {
		t.Fatalf("IssueFlag: %v", err)
	}
	accepted, err := store.AcceptFlagSubmission(ctx, acceptedFlagSubmission{
		Flag:           issued.Flag,
		SubmittingTeam: 101,
		AttackerName:   "Team Alpha",
		VictimName:     issued.OwnerTeamName,
		ChallengeName:  issued.ChallengeName,
		SubmissionTick: 1,
	})
	if err != nil {
		t.Fatalf("AcceptFlagSubmission: %v", err)
	}
	if !accepted {
		t.Fatal("expected accepted submission")
	}
	before, err := store.ListScoreboard(ctx)
	if err != nil {
		t.Fatalf("ListScoreboard after accept: %v", err)
	}
	rows := scoreRowsByTeam(before)
	if !approxScore(rows["Team Alpha"].Attack, 2) {
		t.Fatalf("expected accepted attack to refresh scoreboard, got %+v", rows["Team Alpha"])
	}
	if !approxScore(rows["Team Delta"].Defense, -1) {
		t.Fatalf("expected accepted attack to refresh defense penalty, got %+v", rows["Team Delta"])
	}

	recomputed, err := store.RecomputeScoreboard(ctx)
	if err != nil {
		t.Fatalf("RecomputeScoreboard: %v", err)
	}
	rows = scoreRowsByTeam(recomputed)
	if !approxScore(rows["Team Alpha"].Attack, 2) {
		t.Fatalf("expected recovered attack score, got %+v", rows["Team Alpha"])
	}
	if !approxScore(rows["Team Delta"].Defense, -1) {
		t.Fatalf("expected recovered defense penalty, got %+v", rows["Team Delta"])
	}
}
