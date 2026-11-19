package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"adplatform/internal/platform/database"
	"adplatform/internal/platform/database/dbtest"
	"adplatform/internal/platform/httpapi"
	"adplatform/internal/services/apigateway"
)

// TestPostgresGameLifecycleEndToEnd drives the real game-core server against
// Postgres through a whole match: ticks, a capture, every filtered listing,
// scoring, and the pause/resume/stop transitions.
func TestPostgresGameLifecycleEndToEnd(t *testing.T) {
	db := dbtest.IsolatedSchema(t, "SUBMISSION_TEST_POSTGRES_DSN", "lifecycle_test")
	ctx := context.Background()
	if err := database.RunEmbeddedMigrationsWithOptions(ctx, db, database.MigrationOptions{IncludeSeeds: true}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// Seeded challenges ship unpublished; publish them so checkers target them.
	if _, err := db.ExecContext(ctx, `UPDATE challenges SET published = TRUE`); err != nil {
		t.Fatal(err)
	}

	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "game-core", Version: "test", Addr: ":0"})
	newGameCoreServer("dev-admin-token", newPostgresGameStore(db), testCheckerClient{}, newFlagCodec("lifecycle-secret", "PLAYIT"), &testGameScheduler{}, []string{"put", "get", "check"}, 15).
		WithWarmupRequired(false).
		WithAutoTickOnMatchStart(false).
		WithScoringDebounce(0).
		RegisterRoutes(mux)

	call := func(method, path string, body any) *httptest.ResponseRecorder {
		t.Helper()
		var payload bytes.Buffer
		if body != nil {
			if err := json.NewEncoder(&payload).Encode(body); err != nil {
				t.Fatal(err)
			}
		}
		request := httptest.NewRequest(method, path, &payload)
		request.Header.Set("Authorization", "Bearer dev-admin-token")
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, request)
		return recorder
	}
	mustOK := func(method, path string, body any) []byte {
		t.Helper()
		response := call(method, path, body)
		if response.Code != http.StatusOK {
			t.Fatalf("%s %s: want 200, got %d: %s", method, path, response.Code, response.Body.String())
		}
		return response.Body.Bytes()
	}

	// Before the match starts, flags are rejected and no tick can advance.
	if response := call(http.MethodPost, "/internal/v1/game/ticks/advance", nil); response.Code == http.StatusOK {
		t.Fatalf("tick advanced before match start: %s", response.Body.String())
	}

	mustOK(http.MethodPost, "/internal/v1/game/match/start", nil)
	for range 2 {
		mustOK(http.MethodPost, "/internal/v1/game/ticks/advance", nil)
	}

	status := decodeResponse[apigateway.GameStatus](t, mustOK(http.MethodGet, "/internal/v1/game/status", nil))
	if status.CurrentTick == nil || status.CurrentTick.ID != 2 || status.TotalTicks != 2 || status.TotalCheckerRuns == 0 {
		t.Fatalf("unexpected status after two ticks: %+v", status)
	}

	runs := decodeResponse[apigateway.GameCheckerRunPage](t, mustOK(http.MethodGet, "/internal/v1/game/checker-runs?limit=500", nil))
	if runs.TotalCount == 0 || len(runs.Items) == 0 {
		t.Fatalf("expected checker runs, got %+v", runs)
	}
	sample := runs.Items[0]
	filtered := decodeResponse[apigateway.GameCheckerRunPage](t, mustOK(http.MethodGet,
		"/internal/v1/game/checker-runs?tick_id=2&team_id="+strconv.Itoa(sample.TeamID)+"&challenge_id="+strconv.Itoa(sample.ChallengeID)+"&phase=put&status=success&limit=5&offset=0", nil))
	for _, run := range filtered.Items {
		if run.TickID != 2 || run.TeamID != sample.TeamID || run.ChallengeID != sample.ChallengeID || run.Phase != "put" {
			t.Fatalf("filter returned a non-matching run: %+v", run)
		}
	}
	if filtered.TotalCount != 1 {
		t.Fatalf("expected exactly one put run for one target in tick 2, got %+v", filtered)
	}
	empty := decodeResponse[apigateway.GameCheckerRunPage](t, mustOK(http.MethodGet, "/internal/v1/game/checker-runs?status=failed", nil))
	if empty.TotalCount != 0 {
		t.Fatalf("passing checkers must not list failed runs, got %+v", empty)
	}

	// Steal a flag owned by another team and submit it.
	var flag string
	var owner, challengeID int
	if err := db.QueryRowContext(ctx, `SELECT flag, owner_team_id, challenge_id FROM issued_flags WHERE issued_tick = 2 ORDER BY owner_team_id LIMIT 1`).Scan(&flag, &owner, &challengeID); err != nil {
		t.Fatalf("load issued flag: %v", err)
	}
	var attacker int
	if err := db.QueryRowContext(ctx, `SELECT id FROM teams WHERE id <> $1 AND active ORDER BY id LIMIT 1`, owner).Scan(&attacker); err != nil {
		t.Fatal(err)
	}
	verdicts := decodeResponse[[]submissionVerdictAlias](t, mustOK(http.MethodPost, "/internal/v1/flags/submit", map[string]any{"team_id": attacker, "flags": []string{flag, flag, "PLAYIT{forged.deadbeef}", " "}}))
	if len(verdicts) != 4 || verdicts[0].Status != "accepted" || verdicts[1].Status != "duplicate" || verdicts[2].Status != "invalid" || verdicts[3].Status != "invalid" {
		t.Fatalf("unexpected verdicts %+v", verdicts)
	}
	own := decodeResponse[[]submissionVerdictAlias](t, mustOK(http.MethodPost, "/internal/v1/flags/submit", map[string]any{"team_id": owner, "flags": []string{flag}}))
	if own[0].Status != "invalid" {
		t.Fatalf("a team must not score its own flag, got %+v", own)
	}
	again := decodeResponse[[]submissionVerdictAlias](t, mustOK(http.MethodPost, "/internal/v1/flags/submit", map[string]any{"team_id": attacker, "flags": []string{flag}}))
	if again[0].Status != "duplicate" {
		t.Fatalf("resubmitting a capture must be a duplicate, got %+v", again)
	}

	attacks := decodeResponse[apigateway.AttackFeedPage](t, mustOK(http.MethodGet, "/internal/v1/game/attacks?limit=10&tick_from=1&tick_to=5", nil))
	if attacks.TotalCount != 1 || len(attacks.Items) != 1 || !attacks.Items[0].FirstBlood {
		t.Fatalf("expected one first-blood attack, got %+v", attacks)
	}
	victimName := attacks.Items[0].Victim
	byVictim := decodeResponse[apigateway.AttackFeedPage](t, mustOK(http.MethodGet, "/internal/v1/game/attacks?victim="+url.QueryEscape(victimName)+"&attacker="+url.QueryEscape(attacks.Items[0].Attacker)+"&service="+url.QueryEscape(attacks.Items[0].Service), nil))
	if byVictim.TotalCount != 1 {
		t.Fatalf("attack filters dropped the capture: %+v", byVictim)
	}
	if none := decodeResponse[apigateway.AttackFeedPage](t, mustOK(http.MethodGet, "/internal/v1/game/attacks?tick_from=50", nil)); none.TotalCount != 0 {
		t.Fatalf("tick filter leaked attacks: %+v", none)
	}

	submissions := decodeResponse[apigateway.SubmissionPage](t, mustOK(http.MethodGet, "/internal/v1/game/submissions?limit=50", nil))
	if submissions.TotalCount < 4 {
		t.Fatalf("expected accepted and rejected attempts in history, got %+v", submissions)
	}

	scores := decodeResponse[[]apigateway.ScoreRowAlias](t, mustOK(http.MethodPost, "/internal/v1/game/scoring/recompute", nil))
	var attackerAttack float64
	for _, row := range scores {
		if row.Team == attacks.Items[0].Attacker {
			attackerAttack = row.Attack
		}
	}
	if attackerAttack <= 0 {
		t.Fatalf("attacker must earn attack points, scoreboard %+v", scores)
	}
	audit := decodeResponse[apigateway.ScoringAuditAlias](t, mustOK(http.MethodGet, "/internal/v1/game/scoring/audit", nil))
	if audit.MismatchCount != 0 {
		t.Fatalf("stored and replayed scoreboards diverge: %+v", audit)
	}
	mustOK(http.MethodGet, "/internal/v1/game/scoreboard", nil)

	// Pause blocks submissions and ticks; resume and stop are persisted.
	paused := decodeResponse[apigateway.GameMatchStatus](t, mustOK(http.MethodPost, "/internal/v1/game/match/pause", nil))
	if paused.State != "paused" || paused.AcceptingSubmissions {
		t.Fatalf("unexpected paused state %+v", paused)
	}
	if response := call(http.MethodPost, "/internal/v1/flags/submit", map[string]any{"team_id": attacker, "flags": []string{flag}}); response.Code == http.StatusOK {
		t.Fatalf("submission accepted while paused: %s", response.Body.String())
	}
	if response := call(http.MethodPost, "/internal/v1/game/ticks/advance", nil); response.Code == http.StatusOK {
		t.Fatalf("tick advanced while paused: %s", response.Body.String())
	}
	resumed := decodeResponse[apigateway.GameMatchStatus](t, mustOK(http.MethodPost, "/internal/v1/game/match/resume", nil))
	if resumed.State != "running" {
		t.Fatalf("unexpected resumed state %+v", resumed)
	}
	mustOK(http.MethodGet, "/internal/v1/game/match", nil)
	stopped := decodeResponse[apigateway.GameMatchStatus](t, mustOK(http.MethodPost, "/internal/v1/game/match/stop", nil))
	if stopped.State != "finished" {
		t.Fatalf("unexpected stopped state %+v", stopped)
	}
	if response := call(http.MethodPost, "/internal/v1/flags/submit", map[string]any{"team_id": attacker, "flags": []string{flag}}); response.Code == http.StatusOK {
		t.Fatalf("submission accepted after the match finished: %s", response.Body.String())
	}

	events := decodeResponse[apigateway.GameSchedulerEventPage](t, mustOK(http.MethodGet, "/internal/v1/game/scheduler/events?limit=200", nil))
	_ = events
	filteredEvents := decodeResponse[apigateway.GameSchedulerEventPage](t, mustOK(http.MethodGet, "/internal/v1/game/scheduler/events?event_type=match_started&source=admin&state=running&limit=5&offset=0", nil))
	for _, event := range filteredEvents.Items {
		if event.EventType != "match_started" {
			t.Fatalf("event filter returned %+v", event)
		}
	}
}
