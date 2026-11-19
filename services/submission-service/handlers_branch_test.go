package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"adplatform/internal/services/apigateway"
)

type scriptedSubmissionCore struct{ err error }

func (c scriptedSubmissionCore) SubmitFlags(context.Context, int, []string) ([]apigateway.SubmissionVerdictAlias, error) {
	return []apigateway.SubmissionVerdictAlias{{Status: "accepted", Detail: "something odd"}}, c.err
}

func (c scriptedSubmissionCore) AttackFeed(_ context.Context, query apigateway.AttackFeedQuery) (apigateway.AttackFeedPage, error) {
	return apigateway.AttackFeedPage{Limit: query.Limit, Offset: query.Offset}, c.err
}

func submissionCall(server *submissionServiceServer, method, path, body string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Authorization", "Bearer submit-token")
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	return recorder
}

func TestSubmissionServiceHandlerBranches(t *testing.T) {
	if _, ok := newSubmissionServiceServer("t", nil).gameCore.(noopGameCoreSubmissionClient); !ok {
		t.Fatal("a nil game-core client must default to the noop client")
	}
	ok := newSubmissionServiceServer("submit-token", scriptedSubmissionCore{})
	for _, body := range []string{`not json`, `{"team_id":0,"flags":["x"]}`, `{"team_id":1,"flags":[]}`} {
		if response := submissionCall(ok, http.MethodPost, "/internal/v1/submissions/submit", body); response.Code != http.StatusBadRequest {
			t.Fatalf("body %s: want 400, got %d", body, response.Code)
		}
	}
	if response := submissionCall(ok, http.MethodPost, "/internal/v1/submissions/submit", `{"team_id":1,"flags":["x"]}`); response.Code != http.StatusOK {
		t.Fatalf("submit: %d", response.Code)
	}
	cases := map[string][2]int{
		"":                                {12, 0},
		"?limit=abc&offset=-3":            {12, 0},
		"?limit=0&offset=x":               {12, 0},
		"?limit=999&offset=7":             {200, 7},
		"?limit=5&attacker=a&tick_from=2": {5, 0},
	}
	for query, want := range cases {
		response := submissionCall(ok, http.MethodGet, "/internal/v1/submissions/attacks"+query, "")
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"limit":`+strconv.Itoa(want[0])) || !strings.Contains(response.Body.String(), `"offset":`+strconv.Itoa(want[1])) {
			t.Fatalf("query %q: want limit %d offset %d, got %d %s", query, want[0], want[1], response.Code, response.Body.String())
		}
	}

	failing := newSubmissionServiceServer("submit-token", scriptedSubmissionCore{err: errors.New("core down")})
	if response := submissionCall(failing, http.MethodPost, "/internal/v1/submissions/submit", `{"team_id":1,"flags":["x"]}`); response.Code != http.StatusBadGateway {
		t.Fatalf("submit failure: %d", response.Code)
	}
	if response := submissionCall(failing, http.MethodGet, "/internal/v1/submissions/attacks", ""); response.Code != http.StatusBadGateway {
		t.Fatalf("feed failure: %d", response.Code)
	}
	var metrics bytes.Buffer
	failing.WritePrometheusMetrics(&metrics)
	if !strings.Contains(metrics.String(), "adplatform_submission_service_submit_failures_total 1") || !strings.Contains(metrics.String(), "adplatform_submission_service_attack_feed_failures_total 1") {
		t.Fatalf("failures must be counted:\n%s", metrics.String())
	}

	recorder := httptest.NewRecorder()
	writeSubmissionServiceFailure(recorder, errGameCoreSubmissionDisabled, "")
	if !strings.Contains(recorder.Body.String(), "not connected to game-core") {
		t.Fatal(recorder.Body.String())
	}
	recorder = httptest.NewRecorder()
	writeSubmissionServiceFailure(recorder, errors.New("x"), " ")
	if !strings.Contains(recorder.Body.String(), "submission-service request failed.") {
		t.Fatal(recorder.Body.String())
	}
	if nonNegativeUint64(-5) != 0 || classifySubmissionVerdict("¯\\_(ツ)_/¯") != "unknown" {
		t.Fatal("helper edge cases")
	}
}
