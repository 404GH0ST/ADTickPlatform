package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type seenRequest struct{ method, path, auth, ctype string }

func recordingServer(t *testing.T, status int, body string) (*httptest.Server, *[]seenRequest) {
	t.Helper()
	var seen []seenRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		seen = append(seen, seenRequest{r.Method, r.URL.Path, r.Header.Get("Authorization"), r.Header.Get("Content-Type")})
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server, &seen
}

func TestGameCoreScoringClientRoundTrips(t *testing.T) {
	if _, ok := newGameCoreScoringClient("", "t").(noopGameCoreScoringClient); !ok {
		t.Fatal("blank base URL must yield the noop client")
	}
	noop := noopGameCoreScoringClient{}
	if _, err := noop.Scoreboard(context.Background()); !errors.Is(err, errGameCoreScoringDisabled) {
		t.Fatal(err)
	}
	if _, err := noop.RecomputeScoring(context.Background()); !errors.Is(err, errGameCoreScoringDisabled) {
		t.Fatal(err)
	}
	if _, err := noop.AuditScoring(context.Background()); !errors.Is(err, errGameCoreScoringDisabled) {
		t.Fatal(err)
	}

	server, seen := recordingServer(t, http.StatusOK, `[]`)
	client := newGameCoreScoringClient(server.URL, "core")
	if _, err := client.Scoreboard(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := client.RecomputeScoring(context.Background()); err != nil {
		t.Fatal(err)
	}
	auditServer, auditSeen := recordingServer(t, http.StatusOK, `{}`)
	if _, err := newGameCoreScoringClient(auditServer.URL, "").AuditScoring(context.Background()); err != nil {
		t.Fatal(err)
	}
	if (*seen)[0] != (seenRequest{http.MethodGet, "/internal/v1/game/scoreboard", "Bearer core", ""}) {
		t.Fatalf("unexpected scoreboard request %+v", (*seen)[0])
	}
	if (*seen)[1] != (seenRequest{http.MethodPost, "/internal/v1/game/scoring/recompute", "Bearer core", "application/json"}) {
		t.Fatalf("unexpected recompute request %+v", (*seen)[1])
	}
	if (*auditSeen)[0] != (seenRequest{http.MethodGet, "/internal/v1/game/scoring/audit", "", ""}) {
		t.Fatalf("unexpected audit request %+v", (*auditSeen)[0])
	}
}

func TestGameCoreScoringClientErrors(t *testing.T) {
	cases := []struct{ name, body, want string }{
		{"detail", `{"detail":" recompute busy ","title":"t"}`, "recompute busy"},
		{"title", `{"title":" Upstream "}`, "Upstream"},
		{"message", `{"message":"legacy"}`, "legacy"},
		{"empty", `{}`, "status 503"},
		{"not json", `oops`, "status 503"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server, _ := recordingServer(t, http.StatusServiceUnavailable, tc.body)
			_, err := newGameCoreScoringClient(server.URL, "").Scoreboard(context.Background())
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q, got %v", tc.want, err)
			}
		})
	}
	server, _ := recordingServer(t, http.StatusOK, `{bad`)
	if _, err := newGameCoreScoringClient(server.URL, "").Scoreboard(context.Background()); err == nil {
		t.Fatal("malformed success must error")
	}
	if _, err := (&httpGameCoreScoringClient{baseURL: "http://127.0.0.1:1", client: http.DefaultClient}).Scoreboard(context.Background()); err == nil {
		t.Fatal("transport failure must error")
	}
	if _, err := requestGameCoreScoringJSON[int](context.Background(), &httpGameCoreScoringClient{baseURL: "http://x", client: http.DefaultClient}, http.MethodPost, "/", func() {}); err == nil {
		t.Fatal("unencodable body must error")
	}
	if _, err := requestGameCoreScoringJSON[int](context.Background(), &httpGameCoreScoringClient{baseURL: "://bad", client: http.DefaultClient}, http.MethodGet, "/"); err == nil {
		t.Fatal("invalid URL must error")
	}
	bodyServer, _ := recordingServer(t, http.StatusOK, `1`)
	if _, err := requestGameCoreScoringJSON[int](context.Background(), &httpGameCoreScoringClient{baseURL: bodyServer.URL, client: http.DefaultClient}, http.MethodPost, "/x", map[string]int{"a": 1}); err != nil {
		t.Fatal(err)
	}
}
