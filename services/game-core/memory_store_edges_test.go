package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"adplatform/internal/services/apigateway"
)

func TestScoreRanksBeforeBreaksEveryTie(t *testing.T) {
	cases := []struct {
		a, b scoreRankKey
		want bool
	}{
		{scoreRankKey{10, 0, 0, "b"}, scoreRankKey{9, 9, 9, "a"}, true},  // higher total first
		{scoreRankKey{10, 5, 0, "b"}, scoreRankKey{10, 4, 9, "a"}, true}, // then attack
		{scoreRankKey{10, 5, 2, "b"}, scoreRankKey{10, 5, 1, "a"}, true}, // then defense
		{scoreRankKey{10, 5, 2, "a"}, scoreRankKey{10, 5, 2, "b"}, true}, // then name
		{scoreRankKey{10, 5, 2, "b"}, scoreRankKey{10, 5, 2, "a"}, false},
	}
	for _, tc := range cases {
		if got := scoreRanksBefore(tc.a, tc.b); got != tc.want {
			t.Errorf("scoreRanksBefore(%+v, %+v) = %v", tc.a, tc.b, got)
		}
	}
}

func TestMemoryMatchTransitionsGuardState(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	s := newMemoryGameStore()
	if _, err := s.PauseMatch(ctx, now); !errors.Is(err, errContestNotStarted) {
		t.Fatalf("pause before start: %v", err)
	}
	if status, err := s.ResumeMatch(ctx, now); err != nil || status.State != "not_started" {
		t.Fatalf("resume before start is a no-op: %+v %v", status, err)
	}
	if _, err := s.StartMatch(ctx, now); err != nil {
		t.Fatal(err)
	}
	if status, err := s.StartMatch(ctx, now); err != nil || status.State != "running" {
		t.Fatalf("starting a running match is idempotent: %+v %v", status, err)
	}
	if _, err := s.StopMatch(ctx, now); err != nil {
		t.Fatal(err)
	}
	if status, err := s.StopMatch(ctx, now); err != nil || status.State != "finished" {
		t.Fatalf("stopping a finished match is idempotent: %+v %v", status, err)
	}
	for name, op := range map[string]func() error{
		"start":  func() error { _, err := s.StartMatch(ctx, now); return err },
		"pause":  func() error { _, err := s.PauseMatch(ctx, now); return err },
		"resume": func() error { _, err := s.ResumeMatch(ctx, now); return err },
	} {
		if err := op(); !errors.Is(err, errContestOver) {
			t.Errorf("%s after the match finished: %v", name, err)
		}
	}
	end := now.Add(time.Hour)
	if status, _ := s.UpdateMatchSchedule(ctx, nil, &end); status.ScheduledEndAt == "" {
		t.Fatal("end schedule not stored")
	}
	if status, _ := s.UpdateMatchSchedule(ctx, nil, nil); status.ScheduledEndAt != "" || status.ScheduledStartAt != "" {
		t.Fatalf("nil bounds must clear the schedule: %+v", status)
	}
	if _, err := s.CompleteTick(ctx, apigateway.GameTickStatus{ID: 999}); err == nil {
		t.Fatal("completing an unknown tick must fail")
	}
}

func TestMemoryPlayGateAndTargets(t *testing.T) {
	ctx := context.Background()
	s := newMemoryGameStore().(*memoryGameStore)
	targets, _ := s.ListCheckerTargets(ctx)
	challenge := targets[0].ChallengeID
	s.setChallengePlayFromTickForTest(challenge, 3)
	if blocked, _ := s.IsChallengeInMaintenance(ctx, challenge); !blocked {
		t.Fatal("a challenge deferred to a future tick is blocked")
	}
	s.setChallengePlayFromTickForTest(999, 1)   // unknown challenge: no-op
	s.setChallengeMaintenanceForTest(999, true) // unknown challenge: no-op
	if blocked, _ := s.IsChallengeInMaintenance(ctx, 999); blocked {
		t.Fatal("unknown challenges are not blocked")
	}
	s.setChallengePlayFromTickForTest(challenge, 0)
	if s.CheckerTargetConcurrencyLimit() != 0 {
		t.Fatal("the memory store imposes no concurrency limit")
	}
	if accepted, err := s.AcceptFlagSubmission(ctx, acceptedFlagSubmission{Flag: "x", SubmittingTeam: 1, ExpiresTick: 0}); accepted || !errors.Is(err, errFlagNoLongerValid) {
		t.Fatalf("an expired capture must be rejected: %v %v", accepted, err)
	}
}

func TestMemoryListingsPaginateAndFilter(t *testing.T) {
	ctx := context.Background()
	s := newMemoryGameStore()
	server := newTickServer(s)
	if _, err := s.StartMatch(ctx, server.now()); err != nil {
		t.Fatal(err)
	}
	if _, err := server.advanceTick(ctx); err != nil {
		t.Fatal(err)
	}
	for _, query := range []apigateway.GameCheckerRunQuery{
		{TickID: 99}, {TeamID: 99999}, {ChallengeID: 99999}, {Phase: "nope"}, {Status: "nope"},
	} {
		if page, _ := s.ListCheckerRuns(ctx, query); page.TotalCount != 0 {
			t.Errorf("filter %+v leaked runs", query)
		}
	}
	if page, _ := s.ListCheckerRuns(ctx, apigateway.GameCheckerRunQuery{Offset: -3}); page.Offset != 0 {
		t.Fatalf("negative offsets clamp to zero: %+v", page)
	}
	if page, _ := s.ListCheckerRuns(ctx, apigateway.GameCheckerRunQuery{Offset: 100000}); len(page.Items) != 0 {
		t.Fatal("an offset past the end is empty")
	}
	for _, event := range []schedulerEventRecord{{EventType: "a", Source: "x", State: "s"}, {EventType: "b", Source: "y", State: "t"}} {
		if _, err := s.AppendSchedulerEvent(ctx, event); err != nil {
			t.Fatal(err)
		}
	}
	for _, query := range []apigateway.GameSchedulerEventQuery{{EventType: "zzz"}, {Source: "zzz"}, {State: "zzz"}} {
		if page, _ := s.ListSchedulerEvents(ctx, query); page.TotalCount != 0 {
			t.Errorf("event filter %+v leaked events", query)
		}
	}
	page, _ := s.ListSchedulerEvents(ctx, apigateway.GameSchedulerEventQuery{Offset: -1})
	if page.Limit != 25 || page.Offset != 0 || len(page.Items) != 2 {
		t.Fatalf("default paging: %+v", page)
	}
	if page, _ := s.ListSchedulerEvents(ctx, apigateway.GameSchedulerEventQuery{Offset: 50}); len(page.Items) != 0 {
		t.Fatal("an offset past the end is empty")
	}
	feed, _ := s.ListAttackFeed(ctx, apigateway.AttackFeedQuery{Limit: -1, Offset: -1})
	if feed.Limit != 12 || feed.Offset != 0 {
		t.Fatalf("attack feed defaults: %+v", feed)
	}
}
