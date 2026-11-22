package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"adplatform/internal/services/apigateway"
)

func TestSubmissionHistoryDeletionAndFirstBloodReopens(t *testing.T) {
	mux, store := newTestGameCoreMuxWithStore(testCheckerClient{})
	startTestMatch(t, mux)
	call := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer dev-admin-token")
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, req)
		return response
	}
	if res := call(http.MethodPost, "/internal/v1/game/ticks/advance", ""); res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	flag := issuedTestFlag(t, store, 102, 1, 1)
	other := issuedTestFlag(t, store, 104, 1, 1)
	if res := call(http.MethodPost, "/internal/v1/flags/submit", fmt.Sprintf(`{"team_id":101,"flags":[%q,%q,"garbage"]}`, flag, flag)); res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	if res := call(http.MethodPost, "/internal/v1/flags/submit", fmt.Sprintf(`{"team_id":103,"flags":[%q]}`, other)); res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	history := func() apigateway.SubmissionPage {
		t.Helper()
		res := call(http.MethodGet, "/internal/v1/game/submissions?limit=100", "")
		if res.Code != 200 {
			t.Fatal(res.Body.String())
		}
		return decodeResponse[apigateway.SubmissionPage](t, res.Body.Bytes())
	}
	page := history()
	if page.TotalCount != 4 {
		t.Fatalf("expected all four attempts: %+v", page)
	}
	var first, duplicate int64
	for _, row := range page.Items {
		if row.FirstBlood {
			first = row.ID
		}
		if row.Status == "duplicate" {
			duplicate = row.ID
		}
	}
	if first == 0 || duplicate == 0 {
		t.Fatal("missing first blood or duplicate history")
	}
	unauthorized := httptest.NewRecorder()
	mux.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/internal/v1/game/submissions/%d", first), nil))
	if unauthorized.Code != 403 {
		t.Fatalf("unauthorized deletion: %d", unauthorized.Code)
	}
	if res := call(http.MethodDelete, fmt.Sprintf("/internal/v1/game/submissions/%d", duplicate), ""); res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	if res := call(http.MethodPost, "/internal/v1/flags/submit", fmt.Sprintf(`{"team_id":101,"flags":[%q]}`, flag)); res.Code != 200 {
		t.Fatal(res.Body.String())
	} else if verdict := decodeResponse[[]apigateway.SubmissionVerdictAlias](t, res.Body.Bytes()); verdict[0].Status != "duplicate" {
		t.Fatalf("deleting duplicate changed the capture: %+v", verdict)
	}
	res := call(http.MethodDelete, fmt.Sprintf("/internal/v1/game/submissions/%d", first), "")
	if res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	deleted := decodeResponse[apigateway.SubmissionDeletion](t, res.Body.Bytes())
	if !deleted.FirstBloodCleared {
		t.Fatalf("first blood not cleared: %+v", deleted)
	}
	for _, row := range history().Items {
		if row.FirstBlood {
			t.Fatal("an older normal capture was promoted")
		}
	}
	scores := decodeResponse[[]apigateway.ScoreRowAlias](t, call(http.MethodGet, "/internal/v1/game/scoreboard", "").Body.Bytes())
	for _, row := range scores {
		if row.Team == "Team Alpha" && row.Attack != 0 {
			t.Fatalf("deleted capture is still scored: %+v", row)
		}
	}
	if res := call(http.MethodPost, "/internal/v1/flags/submit", fmt.Sprintf(`{"team_id":101,"flags":[%q]}`, flag)); res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	winners := 0
	for _, row := range history().Items {
		if row.FirstBlood {
			winners++
			if row.ID == first {
				t.Fatal("removed submission ID reused")
			}
		}
	}
	if winners != 1 {
		t.Fatalf("next capture did not earn exactly one first blood: %d", winners)
	}
	if res := call(http.MethodDelete, fmt.Sprintf("/internal/v1/game/submissions/%d", first), ""); res.Code != 404 {
		t.Fatalf("repeat delete should be 404, got %d", res.Code)
	}
	invalid := decodeResponse[apigateway.SubmissionPage](t, call(http.MethodGet, "/internal/v1/game/submissions?status=invalid&flag=garbage", "").Body.Bytes())
	if invalid.TotalCount != 1 {
		t.Fatalf("invalid filter: %+v", invalid)
	}
	attacks := decodeResponse[apigateway.AttackFeedPage](t, call(http.MethodGet, "/internal/v1/game/attacks", "").Body.Bytes())
	if attacks.Revision != 1 || attacks.TotalCount != 2 {
		t.Fatalf("attack feed not invalidated correctly: %+v", attacks)
	}
}

func TestSubmissionHistoryRecordsClosedMatchRejections(t *testing.T) {
	mux := newTestGameCoreMux(testCheckerClient{})
	req := httptest.NewRequest(http.MethodPost, "/internal/v1/flags/submit", bytes.NewBufferString(`{"team_id":101,"flags":["closed-match-attempt"]}`))
	req.Header.Set("Authorization", "Bearer dev-admin-token")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, req)
	if response.Code == 200 {
		t.Fatal("closed match accepted submission")
	}
	req = httptest.NewRequest(http.MethodGet, "/internal/v1/game/submissions?status=invalid", nil)
	req.Header.Set("Authorization", "Bearer dev-admin-token")
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, req)
	page := decodeResponse[apigateway.SubmissionPage](t, response.Body.Bytes())
	if page.TotalCount != 1 || page.Items[0].Flag != "closed-match-attempt" {
		t.Fatalf("closed-match attempt not retained: %+v", page)
	}
}

type failingHistoryStore struct {
	*memoryGameStore
}

func (failingHistoryStore) RecordSubmissions(context.Context, ...apigateway.AdminSubmission) error {
	return errors.New("transient submission history failure")
}

// A capture committed earlier in the batch must still be reported and must
// still schedule a recompute when writing a rejected attempt to the history
// fails afterwards.
func TestHistoryWriteFailureKeepsCommittedCapture(t *testing.T) {
	store := newMemoryGameStore().(*memoryGameStore)
	srv := newGameCoreServer("test", failingHistoryStore{store}, testCheckerClient{}, newFlagCodec("test-secret", "PLAYIT"), &testGameScheduler{}, []string{"put", "get", "check"}, 1).
		WithWarmupRequired(false).WithAutoTickOnMatchStart(false)
	srv.scoringDebounce = time.Hour
	ctx := context.Background()
	if _, err := store.StartMatch(ctx, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.advanceTick(ctx); err != nil {
		t.Fatal(err)
	}
	var victim issuedFlagRecord
	store.mu.Lock()
	for _, flag := range store.flags {
		victim = flag
		break
	}
	store.mu.Unlock()
	attacker := 0
	for id := range store.teamNames {
		if id != victim.OwnerTeamID {
			attacker = id
			break
		}
	}

	results, err := srv.submitFlags(ctx, attacker, []string{victim.Flag, "PLAYIT{garbage}"})
	if err != nil {
		t.Fatalf("committed capture reported as a failed submission: %v", err)
	}
	if len(results) != 2 || results[0].Status != "accepted" || results[1].Status != "invalid" {
		t.Fatalf("unexpected verdicts: %+v", results)
	}
	if !srv.snapshotScoringStatus().Pending {
		t.Fatal("scoreboard recompute not scheduled after accepted capture")
	}

	// With nothing accepted there is no committed state, so the error surfaces.
	if _, err := srv.submitFlags(ctx, attacker, []string{"PLAYIT{garbage}"}); err == nil {
		t.Fatal("history failure with no accepted capture should be returned")
	}
}
