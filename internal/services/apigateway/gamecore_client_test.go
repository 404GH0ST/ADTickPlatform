package apigateway

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGameCoreClientRequests(t *testing.T) {
	ctx := context.Background()
	schedule := UpdateMatchScheduleRequest{ScheduledStartAt: "2027-03-01T12:00:00Z", ScheduledEndAt: "2027-03-01T14:00:00Z"}
	interval := UpdateSchedulerRequest{IntervalSeconds: 30}
	attackFilter := AttackFeedQuery{Attacker: "Team A", Victim: "Team B", Service: "Mail", Limit: 12, Offset: 3, TickFrom: 2, TickTo: 9}
	client := &httpGameCoreClient{}
	checks := []struct {
		method, path, response string
		body                   any
		call                   func() error
	}{
		{"GET", "/game/status", `{}`, nil, gameCoreCall(ctx, client.Status)},
		{"GET", "/game/match", `{}`, nil, gameCoreCall(ctx, client.MatchStatus)},
		{"POST", "/game/match/start", `{}`, nil, gameCoreCall(ctx, client.StartMatch)},
		{"POST", "/game/match/pause", `{}`, nil, gameCoreCall(ctx, client.PauseMatch)},
		{"POST", "/game/match/resume", `{}`, nil, gameCoreCall(ctx, client.ResumeMatch)},
		{"POST", "/game/match/stop", `{}`, nil, gameCoreCall(ctx, client.StopMatch)},
		{"PUT", "/game/match/schedule", `{}`, schedule, func() error {
			_, err := client.UpdateMatchSchedule(ctx, schedule)
			return err
		}},
		{"POST", "/game/ticks/advance", `{}`, nil, gameCoreCall(ctx, client.AdvanceTick)},
		{"GET", "/game/scheduler", `{}`, nil, gameCoreCall(ctx, client.SchedulerStatus)},
		{"POST", "/game/scheduler/start", `{}`, nil, gameCoreCall(ctx, client.StartScheduler)},
		{"POST", "/game/scheduler/stop", `{}`, nil, gameCoreCall(ctx, client.StopScheduler)},
		{"PUT", "/game/scheduler/interval", `{}`, interval, func() error {
			_, err := client.UpdateScheduler(ctx, interval)
			return err
		}},
		{"GET", "/game/scheduler/events?event_type=tick&limit=25&offset=5&source=manual&state=running", `{}`, nil, func() error {
			_, err := client.SchedulerEvents(ctx, GameSchedulerEventQuery{EventType: "tick", Limit: 25, Offset: 5, Source: "manual", State: "running"})
			return err
		}},
		{"GET", "/game/checker-runs?challenge_id=3&limit=10&offset=2&phase=check&status=up&team_id=101&tick_id=12", `{}`, nil, func() error {
			_, err := client.CheckerRuns(ctx, GameCheckerRunQuery{ChallengeID: 3, Limit: 10, Offset: 2, Phase: "check", Status: "up", TeamID: 101, TickID: 12})
			return err
		}},
		{"GET", "/game/scoreboard", `[]`, nil, gameCoreCall(ctx, client.Scoreboard)},
		{"POST", "/game/scoring/recompute", `[]`, nil, gameCoreCall(ctx, client.RecomputeScoring)},
		{"GET", "/game/scoring/audit", `{}`, nil, gameCoreCall(ctx, client.AuditScoring)},
		{"GET", "/game/flag/format", `{}`, nil, gameCoreCall(ctx, client.FlagFormat)},
		{"POST", "/game/flag/format/refresh", `{}`, UpdateFlagFormatRequest{Prefix: "TEST"}, func() error {
			_, err := client.RefreshFlagFormat(ctx, "TEST")
			return err
		}},
		{"POST", "/flags/submit", `[]`, GameSubmitFlagsRequest{TeamID: 101, Flags: []string{"fixture-flag"}}, func() error {
			_, err := client.SubmitFlags(ctx, 101, []string{"fixture-flag"})
			return err
		}},
		{"GET", "/game/attacks?attacker=Team+A&limit=12&offset=3&service=Mail&tick_from=2&tick_to=9&victim=Team+B", `{}`, nil, func() error {
			_, err := client.AttackFeed(ctx, attackFilter)
			return err
		}},
		{"GET", "/game/submissions?attacker=Team+A&flag=fixture&limit=12&offset=3&service=Mail&status=accepted&tick_from=2&tick_to=9&victim=Team+B", `{}`, nil, func() error {
			_, err := client.Submissions(ctx, SubmissionQuery{AttackFeedQuery: attackFilter, Status: "accepted", Flag: "fixture"})
			return err
		}},
		{"DELETE", "/game/submissions/27", `{}`, nil, func() error {
			_, err := client.DeleteSubmission(ctx, 27)
			return err
		}},
	}
	for _, tt := range checks {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			service := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got := r.Method + " " + r.URL.RequestURI(); got != tt.method+" /internal/v1"+tt.path {
					t.Errorf("unexpected game-core request %s", got)
				}
				if got := r.Header.Get("Authorization"); got != "Bearer game-core-test-token" {
					t.Error("game-core request did not use the configured bearer token")
				}
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Errorf("read game-core request: %v", err)
				} else if tt.body == nil {
					if len(body) != 0 {
						t.Errorf("unexpected request body: %s", body)
					}
				} else {
					want, err := json.Marshal(tt.body)
					if err != nil || string(body) != string(want) || r.Header.Get("Content-Type") != "application/json" {
						t.Errorf("request body = %s, want JSON %s (%v)", body, want, err)
					}
				}
				_, _ = io.WriteString(w, tt.response)
			}))
			defer service.Close()
			*client = *NewHTTPGameCoreClientWithTimeout(service.URL+"/", " game-core-test-token ", time.Second).(*httpGameCoreClient)
			if err := tt.call(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func gameCoreCall[T any](ctx context.Context, call func(context.Context) (T, error)) func() error {
	return func() error {
		_, err := call(ctx)
		return err
	}
}

func TestGameCoreClientReportsFailures(t *testing.T) {
	ctx := context.Background()
	if _, err := NewHTTPGameCoreClient("invalid-url", "").Status(ctx); !errors.Is(err, errGameCoreDisabled) {
		t.Fatalf("disabled client status = %v", err)
	}
	for _, tt := range []struct {
		status int
		body   string
		want   string
	}{
		{http.StatusConflict, `{"detail":"The match is already active.","title":"Conflict"}`, "The match is already active."},
		{http.StatusConflict, `{"title":"Conflict"}`, "Conflict"},
		{http.StatusServiceUnavailable, `{"message":"  Maintenance  "}`, "Maintenance"},
		{http.StatusBadGateway, `bad gateway`, "game core request failed with status 502"},
		{http.StatusOK, ``, "EOF"},
	} {
		client := NewHTTPGameCoreClientWithTimeout("http://core.internal", "", 0).(*httpGameCoreClient)
		client.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.Header.Get("Authorization") != "" {
				t.Error("unexpected authorization on an unconfigured client")
			}
			return &http.Response{StatusCode: tt.status, Body: io.NopCloser(strings.NewReader(tt.body))}, nil
		})
		_, err := client.Status(ctx)
		if err == nil || err.Error() != tt.want {
			t.Errorf("status %d returned %v, want %q", tt.status, err, tt.want)
		}
		var upstream gameCoreHTTPError
		if tt.status != http.StatusOK && (!errors.As(err, &upstream) || upstream.StatusCode != tt.status) {
			t.Errorf("missing upstream status %d in %v", tt.status, err)
		}
	}
}
