package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type snapshotRoute struct {
	status int
	body   string
}

func TestHTTPPublicSnapshotClientRoutesAndTokens(t *testing.T) {
	type seen struct{ path, query, auth string }
	var requests []seen
	routes := map[string]snapshotRoute{
		"/api/v2/scoreboard":                  {http.StatusOK, `[{"team_id":1}]`},
		"/api/v2/admin/game/scoreboard":       {http.StatusOK, `[{"team_id":1},{"team_id":2}]`},
		"/api/v2/attacks":                     {http.StatusOK, `{"items":[],"limit":12}`},
		"/api/v2/admin/game/status":           {http.StatusOK, `{"total_ticks":4}`},
		"/api/v2/admin/game/scheduler/events": {http.StatusOK, `{"items":[],"limit":12}`},
		"/api/v2/admin/game/checker-runs":     {http.StatusOK, `{"items":[],"limit":18}`},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, seen{r.URL.Path, r.URL.RawQuery, r.Header.Get("Authorization")})
		route := routes[r.URL.Path]
		w.WriteHeader(route.status)
		_, _ = w.Write([]byte(route.body))
	}))
	defer server.Close()

	client := newHTTPPublicSnapshotClient(server.URL, " admin-token ")
	ctx := context.Background()
	if rows, err := client.Scoreboard(ctx); err != nil || len(rows) != 1 {
		t.Fatalf("scoreboard %v %v", rows, err)
	}
	if rows, err := client.ScoreboardLive(ctx); err != nil || len(rows) != 2 {
		t.Fatalf("live scoreboard %v %v", rows, err)
	}
	if _, err := client.Attacks(ctx); err != nil {
		t.Fatal(err)
	}
	if status, err := client.GameStatus(ctx); err != nil || status.TotalTicks != 4 {
		t.Fatalf("status %+v %v", status, err)
	}
	if _, err := client.SchedulerEvents(ctx, 12); err != nil {
		t.Fatal(err)
	}
	if _, err := client.SchedulerEvents(ctx, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := client.CheckerRuns(ctx, 18); err != nil {
		t.Fatal(err)
	}
	if _, err := client.CheckerRuns(ctx, 0); err != nil {
		t.Fatal(err)
	}

	want := []seen{
		// Public feeds must never carry the admin token, or the freeze-aware
		// public board could be swapped for the live organizer board.
		{"/api/v2/scoreboard", "", ""},
		{"/api/v2/admin/game/scoreboard", "", "Bearer admin-token"},
		{"/api/v2/attacks", "", ""},
		{"/api/v2/admin/game/status", "", "Bearer admin-token"},
		{"/api/v2/admin/game/scheduler/events", "limit=12", "Bearer admin-token"},
		{"/api/v2/admin/game/scheduler/events", "", "Bearer admin-token"},
		{"/api/v2/admin/game/checker-runs", "limit=18", "Bearer admin-token"},
		{"/api/v2/admin/game/checker-runs", "", "Bearer admin-token"},
	}
	if len(requests) != len(want) {
		t.Fatalf("got %d requests, want %d: %+v", len(requests), len(want), requests)
	}
	for i := range want {
		if requests[i] != want[i] {
			t.Fatalf("request %d = %+v, want %+v", i, requests[i], want[i])
		}
	}
}

func TestHTTPPublicSnapshotClientInvalidBaseURLFallsBack(t *testing.T) {
	client := newHTTPPublicSnapshotClient("not a url", "").(*httpPublicSnapshotClient)
	if client.baseURL != "http://127.0.0.1" {
		t.Fatalf("invalid base URL must fall back to loopback, got %q", client.baseURL)
	}
}

func TestFetchSnapshotErrors(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"problem detail", http.StatusForbidden, `{"title":"Forbidden","detail":" bad token "}`, "bad token"},
		{"problem title", http.StatusBadGateway, `{"title":" Upstream "}`, "Upstream"},
		{"empty problem", http.StatusBadGateway, `{}`, "status 502"},
		{"not json", http.StatusBadGateway, `oops`, "status 502"},
		{"bad success body", http.StatusOK, `{bad`, "invalid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			_, err := fetchSnapshot[map[string]any](context.Background(), http.DefaultClient, server.URL)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want error containing %q, got %v", tc.want, err)
			}
		})
	}
	if _, err := fetchSnapshot[int](context.Background(), http.DefaultClient, "://bad"); err == nil {
		t.Fatal("invalid URL must error")
	}
	if _, err := fetchSnapshot[int](context.Background(), http.DefaultClient, "http://127.0.0.1:1"); err == nil {
		t.Fatal("transport failure must error")
	}
}
