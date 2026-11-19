package apigateway

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type recordedRequest struct {
	method string
	path   string
	query  url.Values
	auth   string
	ctype  string
	body   string
}

// newRecordingServer answers every request with status/body and records it.
func newRecordingServer(t *testing.T, status int, body string) (*httptest.Server, *[]recordedRequest) {
	t.Helper()
	var requests []recordedRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := new(strings.Builder)
		if r.Body != nil {
			buf := make([]byte, 4096)
			for {
				n, err := r.Body.Read(buf)
				data.Write(buf[:n])
				if err != nil {
					break
				}
			}
		}
		requests = append(requests, recordedRequest{
			method: r.Method,
			path:   r.URL.Path,
			query:  r.URL.Query(),
			auth:   r.Header.Get("Authorization"),
			ctype:  r.Header.Get("Content-Type"),
			body:   data.String(),
		})
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server, &requests
}

// upstreamErrorCases covers every error-body shape the internal clients parse.
var upstreamErrorCases = []struct {
	name string
	body string
	want string
}{
	{"problem detail", `{"title":"Bad","detail":"  flag storm  "}`, "flag storm"},
	{"problem title", `{"title":" Upstream broke "}`, "Upstream broke"},
	{"message payload", `{"message":"legacy failure"}`, "legacy failure"},
	{"empty json", `{}`, "status 502"},
	{"not json", `<html>gateway</html>`, "status 502"},
}

func TestHTTPSubmissionClientRoundTrips(t *testing.T) {
	if _, ok := NewHTTPSubmissionClient("", "token").(noopSubmissionClient); !ok {
		t.Fatal("blank base URL must yield the noop client")
	}
	if _, err := (noopSubmissionClient{}).SubmitFlags(context.Background(), 1, nil); !errors.Is(err, errSubmissionServiceDisabled) {
		t.Fatalf("noop submit must report disabled, got %v", err)
	}
	if _, err := (noopSubmissionClient{}).AttackFeed(context.Background(), AttackFeedQuery{}); !errors.Is(err, errSubmissionServiceDisabled) {
		t.Fatalf("noop feed must report disabled, got %v", err)
	}

	server, requests := newRecordingServer(t, http.StatusOK, `[{"flag":"F","status":"accepted","detail":"ok","challenge_id":3}]`)
	client := NewHTTPSubmissionClient(server.URL, " internal-token ")
	verdicts, err := client.SubmitFlags(context.Background(), 101, []string{"F"})
	if err != nil || len(verdicts) != 1 || verdicts[0].Status != "accepted" || verdicts[0].ChallengeID != 3 {
		t.Fatalf("unexpected verdicts %+v %v", verdicts, err)
	}
	got := (*requests)[0]
	if got.method != http.MethodPost || got.path != "/internal/v1/submissions/submit" || got.auth != "Bearer internal-token" || got.ctype != "application/json" {
		t.Fatalf("unexpected submit request %+v", got)
	}
	var sent GameSubmitFlagsRequest
	if err := json.Unmarshal([]byte(got.body), &sent); err != nil || sent.TeamID != 101 || len(sent.Flags) != 1 {
		t.Fatalf("unexpected submit body %q (%v)", got.body, err)
	}
}

func TestHTTPSubmissionClientAttackFeedQuery(t *testing.T) {
	server, requests := newRecordingServer(t, http.StatusOK, `{"items":[],"limit":5,"offset":10,"total_count":0}`)
	client := NewHTTPSubmissionClient(server.URL, "")

	if _, err := client.AttackFeed(context.Background(), AttackFeedQuery{}); err != nil {
		t.Fatal(err)
	}
	if first := (*requests)[0]; len(first.query) != 0 || first.auth != "" || first.ctype != "" {
		t.Fatalf("empty query must send no params, auth, or content type: %+v", first)
	}

	page, err := client.AttackFeed(context.Background(), AttackFeedQuery{Limit: 5, Offset: 10, Attacker: "A&B", Victim: "V", Service: "web", TickFrom: 2, TickTo: 9})
	if err != nil || page.Limit != 5 || page.Offset != 10 {
		t.Fatalf("unexpected page %+v %v", page, err)
	}
	q := (*requests)[1].query
	if q.Get("limit") != "5" || q.Get("offset") != "10" || q.Get("attacker") != "A&B" || q.Get("victim") != "V" || q.Get("service") != "web" || q.Get("tick_from") != "2" || q.Get("tick_to") != "9" {
		t.Fatalf("unexpected encoded query %v", q)
	}
}

func TestHTTPSubmissionClientErrors(t *testing.T) {
	for _, tc := range upstreamErrorCases {
		t.Run(tc.name, func(t *testing.T) {
			server, _ := newRecordingServer(t, http.StatusBadGateway, tc.body)
			_, err := NewHTTPSubmissionClient(server.URL, "t").SubmitFlags(context.Background(), 1, []string{"x"})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want error containing %q, got %v", tc.want, err)
			}
		})
	}
	server, _ := newRecordingServer(t, http.StatusOK, `not json`)
	if _, err := NewHTTPSubmissionClient(server.URL, "t").SubmitFlags(context.Background(), 1, []string{"x"}); err == nil {
		t.Fatal("malformed success body must error")
	}
	if _, err := (&httpSubmissionClient{baseURL: "http://127.0.0.1:1", client: http.DefaultClient}).SubmitFlags(context.Background(), 1, nil); err == nil {
		t.Fatal("transport failure must error")
	}
	if _, err := requestSubmissionJSON[int](context.Background(), &httpSubmissionClient{baseURL: "http://x", client: http.DefaultClient}, http.MethodPost, "/", func() {}); err == nil {
		t.Fatal("unencodable body must error")
	}
	if _, err := requestSubmissionJSON[int](context.Background(), &httpSubmissionClient{baseURL: "://bad", client: http.DefaultClient}, http.MethodGet, "/"); err == nil {
		t.Fatal("invalid URL must error")
	}
}

func TestHTTPScoringClientRoundTrips(t *testing.T) {
	if _, ok := NewHTTPScoringClient("", "token").(noopScoringClient); !ok {
		t.Fatal("blank base URL must yield the noop client")
	}
	noop := noopScoringClient{}
	if _, err := noop.Scoreboard(context.Background()); !errors.Is(err, errScoringWorkerDisabled) {
		t.Fatal(err)
	}
	if _, err := noop.RecomputeScoring(context.Background()); !errors.Is(err, errScoringWorkerDisabled) {
		t.Fatal(err)
	}
	if _, err := noop.AuditScoring(context.Background()); !errors.Is(err, errScoringWorkerDisabled) {
		t.Fatal(err)
	}

	server, requests := newRecordingServer(t, http.StatusOK, `[{"team_id":101,"team":"Alpha","total":12.5}]`)
	client := NewHTTPScoringClient(server.URL, "score-token")
	rows, err := client.Scoreboard(context.Background())
	if err != nil || len(rows) != 1 {
		t.Fatalf("scoreboard %+v %v", rows, err)
	}
	if _, err := client.RecomputeScoring(context.Background()); err != nil {
		t.Fatal(err)
	}
	auditServer, auditRequests := newRecordingServer(t, http.StatusOK, `{}`)
	if _, err := NewHTTPScoringClient(auditServer.URL, "score-token").AuditScoring(context.Background()); err != nil {
		t.Fatal(err)
	}
	if (*requests)[0].method != http.MethodGet || (*requests)[0].path != "/internal/v1/scoring/scoreboard" || (*requests)[0].auth != "Bearer score-token" {
		t.Fatalf("unexpected scoreboard request %+v", (*requests)[0])
	}
	if (*requests)[1].method != http.MethodPost || (*requests)[1].path != "/internal/v1/scoring/recompute" || (*requests)[1].ctype != "application/json" {
		t.Fatalf("unexpected recompute request %+v", (*requests)[1])
	}
	if (*auditRequests)[0].path != "/internal/v1/scoring/audit" {
		t.Fatalf("unexpected audit request %+v", (*auditRequests)[0])
	}
}

func TestHTTPScoringClientErrors(t *testing.T) {
	for _, tc := range upstreamErrorCases {
		t.Run(tc.name, func(t *testing.T) {
			server, _ := newRecordingServer(t, http.StatusBadGateway, tc.body)
			_, err := NewHTTPScoringClient(server.URL, "").Scoreboard(context.Background())
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want error containing %q, got %v", tc.want, err)
			}
		})
	}
	server, _ := newRecordingServer(t, http.StatusOK, `{bad`)
	if _, err := NewHTTPScoringClient(server.URL, "").Scoreboard(context.Background()); err == nil {
		t.Fatal("malformed success body must error")
	}
	if _, err := (&httpScoringClient{baseURL: "http://127.0.0.1:1", client: http.DefaultClient}).Scoreboard(context.Background()); err == nil {
		t.Fatal("transport failure must error")
	}
	if _, err := requestScoringJSON[int](context.Background(), &httpScoringClient{baseURL: "http://x", client: http.DefaultClient}, http.MethodPost, "/", func() {}); err == nil {
		t.Fatal("unencodable body must error")
	}
	if _, err := requestScoringJSON[int](context.Background(), &httpScoringClient{baseURL: "://bad", client: http.DefaultClient}, http.MethodGet, "/"); err == nil {
		t.Fatal("invalid URL must error")
	}
	bodyServer, bodyRequests := newRecordingServer(t, http.StatusOK, `1`)
	if _, err := requestScoringJSON[int](context.Background(), &httpScoringClient{baseURL: bodyServer.URL, client: http.DefaultClient}, http.MethodPost, "/x", map[string]int{"a": 1}); err != nil {
		t.Fatal(err)
	}
	if (*bodyRequests)[0].body != `{"a":1}` {
		t.Fatalf("request body not sent: %q", (*bodyRequests)[0].body)
	}
}
