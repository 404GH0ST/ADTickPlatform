package main

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"adplatform/internal/platform/httpapi"
)

type faultFixture struct {
	mem      gameStore
	faults   *callFaults
	server   *gameCoreServer
	mux      *http.ServeMux
	attacker int
	flag     string
}

// newFaultFixture builds a running match with one clean tick behind it, then
// routes a fresh server through a store that can fail any single call.
func newFaultFixture(t *testing.T) faultFixture {
	t.Helper()
	ctx := context.Background()
	mem := newMemoryGameStore()
	clean := newTickServer(mem)
	if _, err := mem.StartMatch(ctx, clean.now()); err != nil {
		t.Fatal(err)
	}
	if _, err := clean.advanceTick(ctx); err != nil {
		t.Fatal(err)
	}
	targets, err := mem.ListCheckerTargets(ctx)
	if err != nil || len(targets) < 2 {
		t.Fatalf("targets: %v %v", targets, err)
	}
	victim := targets[0]
	issued, err := mem.LatestIssuedFlag(ctx, victim.TeamID, victim.ChallengeID)
	if err != nil {
		t.Fatal(err)
	}
	attacker := targets[len(targets)-1].TeamID
	faults := &callFaults{}
	server := newTickServer(faultingGameStore{inner: mem, faults: faults}).WithScoringDebounce(0)
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "game-core", Version: "test", Addr: ":0"})
	server.RegisterRoutes(mux)
	return faultFixture{mem: mem, faults: faults, server: server, mux: mux, attacker: attacker, flag: issued.Flag}
}

// sweepServerFaults fails each store call an operation makes, on a fresh
// fixture per position. With mustRecover set, a clean retry must then succeed.
func sweepServerFaults(t *testing.T, name string, mustRecover bool, op func(f faultFixture) error) {
	t.Helper()
	counting := newFaultFixture(t)
	counting.faults.arm(0)
	if err := op(counting); err != nil {
		t.Fatalf("%s: clean run failed: %v", name, err)
	}
	total := counting.faults.calls()
	if total == 0 {
		t.Fatalf("%s: made no store calls", name)
	}
	for position := 1; position <= total; position++ {
		fixture := newFaultFixture(t)
		fixture.faults.arm(position)
		func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Errorf("%s: panic with fault at call %d/%d: %v", name, position, total, recovered)
				}
			}()
			_ = op(fixture)
		}()
		if !mustRecover {
			continue
		}
		fixture.faults.arm(0)
		if err := op(fixture); err != nil {
			t.Errorf("%s: no recovery after fault at call %d/%d: %v", name, position, total, err)
		}
	}
}

func httpOp(method, path, body string) func(f faultFixture) error {
	return func(f faultFixture) error {
		response := gameCall(f.mux, method, path, body)
		if response.Code >= 400 {
			return fmt.Errorf("%s %s: %d %s", method, path, response.Code, response.Body.String())
		}
		return nil
	}
}

func TestServerOperationsSurviveStoreFaults(t *testing.T) {
	ctx := context.Background()
	sweepServerFaults(t, "advance tick", true, func(f faultFixture) error {
		_, err := f.server.advanceTick(ctx)
		return err
	})
	sweepServerFaults(t, "submit flags", true, func(f faultFixture) error {
		_, err := f.server.submitFlags(ctx, f.attacker, []string{f.flag, "PLAYIT{junk}", " "})
		return err
	})
	sweepServerFaults(t, "warmup", false, func(f faultFixture) error {
		_, err := f.server.runWarmup(ctx)
		return err
	})
	sweepServerFaults(t, "retention sweep", false, func(f faultFixture) error {
		_, err := f.server.sweepRetainedFlags(ctx, ctx, true)
		return err
	})
	sweepServerFaults(t, "stop with retention", false, func(f faultFixture) error {
		_, err := f.server.stopMatchWithRetention(ctx, time.Now())
		return err
	})
	for _, route := range []struct{ method, path, body string }{
		{http.MethodGet, "/internal/v1/game/status", ""},
		{http.MethodGet, "/internal/v1/game/match", ""},
		{http.MethodPost, "/internal/v1/game/match/pause", ""},
		{http.MethodPost, "/internal/v1/game/match/stop", ""},
		{http.MethodPut, "/internal/v1/game/match/schedule", `{"scheduled_end_at":"2099-01-01T00:00:00Z"}`},
		{http.MethodGet, "/internal/v1/game/checker-runs?tick_id=1&phase=put", ""},
		{http.MethodGet, "/internal/v1/game/scoreboard", ""},
		{http.MethodGet, "/internal/v1/game/attacks?limit=5", ""},
		{http.MethodGet, "/internal/v1/game/submissions?limit=5", ""},
		{http.MethodPost, "/internal/v1/game/scoring/recompute", ""},
		{http.MethodGet, "/internal/v1/game/scoring/audit", ""},
		{http.MethodPost, "/internal/v1/game/ticks/advance", ""},
		{http.MethodPost, "/internal/v1/game/warmup", ""},
	} {
		sweepServerFaults(t, route.method+" "+route.path, false, httpOp(route.method, route.path, route.body))
	}
	sweepServerFaults(t, "resume", false, func(f faultFixture) error {
		// Pause through the underlying store so the fault counter only sees the resume.
		if _, err := f.mem.PauseMatch(ctx, time.Now()); err != nil {
			return err
		}
		return httpOp(http.MethodPost, "/internal/v1/game/match/resume", "")(f)
	})
}
