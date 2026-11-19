package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"adplatform/internal/services/apigateway"
)

type seenRequest struct {
	method, path, auth, ctype, body string
	query                           url.Values
}

func recordingServer(t *testing.T, status int, body string) (*httptest.Server, *[]seenRequest) {
	t.Helper()
	var seen []seenRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		seen = append(seen, seenRequest{r.Method, r.URL.Path, r.Header.Get("Authorization"), r.Header.Get("Content-Type"), string(data), r.URL.Query()})
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server, &seen
}

func TestGameCoreSubmissionClientRoundTrips(t *testing.T) {
	if _, ok := newGameCoreSubmissionClient("", "t").(noopGameCoreSubmissionClient); !ok {
		t.Fatal("blank base URL must yield the noop client")
	}
	if _, err := (noopGameCoreSubmissionClient{}).SubmitFlags(context.Background(), 1, nil); !errors.Is(err, errGameCoreSubmissionDisabled) {
		t.Fatal(err)
	}
	if _, err := (noopGameCoreSubmissionClient{}).AttackFeed(context.Background(), apigateway.AttackFeedQuery{}); !errors.Is(err, errGameCoreSubmissionDisabled) {
		t.Fatal(err)
	}

	server, seen := recordingServer(t, http.StatusOK, `[{"flag":"F","status":"accepted","detail":"ok"}]`)
	client := newGameCoreSubmissionClient(server.URL, " core-token ")
	verdicts, err := client.SubmitFlags(context.Background(), 7, []string{"F"})
	if err != nil || len(verdicts) != 1 || verdicts[0].Status != "accepted" {
		t.Fatalf("unexpected verdicts %+v %v", verdicts, err)
	}
	got := (*seen)[0]
	if got.method != http.MethodPost || got.path != "/internal/v1/flags/submit" || got.auth != "Bearer core-token" || got.ctype != "application/json" {
		t.Fatalf("unexpected submit request %+v", got)
	}
	var sent apigateway.GameSubmitFlagsRequest
	if err := json.Unmarshal([]byte(got.body), &sent); err != nil || sent.TeamID != 7 {
		t.Fatalf("unexpected body %q", got.body)
	}

	feedServer, feedSeen := recordingServer(t, http.StatusOK, `{"items":[],"limit":3}`)
	feed := newGameCoreSubmissionClient(feedServer.URL, "")
	if _, err := feed.AttackFeed(context.Background(), apigateway.AttackFeedQuery{}); err != nil {
		t.Fatal(err)
	}
	if first := (*feedSeen)[0]; len(first.query) != 0 || first.auth != "" || first.path != "/internal/v1/game/attacks" {
		t.Fatalf("unexpected empty feed request %+v", first)
	}
	if _, err := feed.AttackFeed(context.Background(), apigateway.AttackFeedQuery{Limit: 3, Offset: 6, Attacker: "a", Victim: "v", Service: "s", TickFrom: 1, TickTo: 2}); err != nil {
		t.Fatal(err)
	}
	q := (*feedSeen)[1].query
	if q.Get("limit") != "3" || q.Get("offset") != "6" || q.Get("attacker") != "a" || q.Get("victim") != "v" || q.Get("service") != "s" || q.Get("tick_from") != "1" || q.Get("tick_to") != "2" {
		t.Fatalf("unexpected feed query %v", q)
	}
}

func TestGameCoreSubmissionClientErrors(t *testing.T) {
	cases := []struct{ name, body, want string }{
		{"detail", `{"detail":"bad flag batch","message":"m","title":"t"}`, "bad flag batch"},
		{"message", `{"message":"legacy","title":"t"}`, "legacy"},
		{"title", `{"title":"Upstream"}`, "Upstream"},
		{"empty", `{}`, "status 502"},
		{"not json", `oops`, "status 502"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server, _ := recordingServer(t, http.StatusBadGateway, tc.body)
			_, err := newGameCoreSubmissionClient(server.URL, "").SubmitFlags(context.Background(), 1, []string{"x"})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q, got %v", tc.want, err)
			}
		})
	}
	server, _ := recordingServer(t, http.StatusOK, `{bad`)
	if _, err := newGameCoreSubmissionClient(server.URL, "").SubmitFlags(context.Background(), 1, nil); err == nil {
		t.Fatal("malformed success must error")
	}
	if _, err := (&httpGameCoreSubmissionClient{baseURL: "http://127.0.0.1:1", client: http.DefaultClient}).SubmitFlags(context.Background(), 1, nil); err == nil {
		t.Fatal("transport failure must error")
	}
	if _, err := requestGameCoreSubmissionJSON[int](context.Background(), &httpGameCoreSubmissionClient{baseURL: "http://x", client: http.DefaultClient}, http.MethodPost, "/", func() {}); err == nil {
		t.Fatal("unencodable body must error")
	}
	if _, err := requestGameCoreSubmissionJSON[int](context.Background(), &httpGameCoreSubmissionClient{baseURL: "://bad", client: http.DefaultClient}, http.MethodGet, "/"); err == nil {
		t.Fatal("invalid URL must error")
	}
}
