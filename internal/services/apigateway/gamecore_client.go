package apigateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"adplatform/internal/platform/httpapi"
)

var errGameCoreDisabled = errors.New("game core is not configured")

type gameCoreClient interface {
	Status(ctx context.Context) (GameStatus, error)
	MatchStatus(ctx context.Context) (GameMatchStatus, error)
	StartMatch(ctx context.Context) (GameMatchStatus, error)
	StopMatch(ctx context.Context) (GameMatchStatus, error)
	UpdateMatchSchedule(ctx context.Context, req UpdateMatchScheduleRequest) (GameMatchStatus, error)
	AdvanceTick(ctx context.Context) (GameTickStatus, error)
	CheckerRuns(ctx context.Context, query GameCheckerRunQuery) (GameCheckerRunPage, error)
	SchedulerStatus(ctx context.Context) (GameSchedulerStatus, error)
	SchedulerEvents(ctx context.Context, query GameSchedulerEventQuery) (GameSchedulerEventPage, error)
	StartScheduler(ctx context.Context) (GameSchedulerStatus, error)
	StopScheduler(ctx context.Context) (GameSchedulerStatus, error)
	UpdateScheduler(ctx context.Context, req UpdateSchedulerRequest) (GameSchedulerStatus, error)
	SubmitFlags(ctx context.Context, teamID int, flags []string) ([]submissionVerdict, error)
	Scoreboard(ctx context.Context) ([]scoreRow, error)
	AttackFeed(ctx context.Context, query AttackFeedQuery) (AttackFeedPage, error)
	RecomputeScoring(ctx context.Context) ([]scoreRow, error)
	AuditScoring(ctx context.Context) (ScoringAuditAlias, error)
}

type noopGameCoreClient struct{}

func (noopGameCoreClient) Status(context.Context) (GameStatus, error) {
	return GameStatus{}, errGameCoreDisabled
}

func (noopGameCoreClient) MatchStatus(context.Context) (GameMatchStatus, error) {
	return GameMatchStatus{}, errGameCoreDisabled
}

func (noopGameCoreClient) StartMatch(context.Context) (GameMatchStatus, error) {
	return GameMatchStatus{}, errGameCoreDisabled
}

func (noopGameCoreClient) StopMatch(context.Context) (GameMatchStatus, error) {
	return GameMatchStatus{}, errGameCoreDisabled
}

func (noopGameCoreClient) UpdateMatchSchedule(context.Context, UpdateMatchScheduleRequest) (GameMatchStatus, error) {
	return GameMatchStatus{}, errGameCoreDisabled
}

func (noopGameCoreClient) AdvanceTick(context.Context) (GameTickStatus, error) {
	return GameTickStatus{}, errGameCoreDisabled
}

func (noopGameCoreClient) CheckerRuns(context.Context, GameCheckerRunQuery) (GameCheckerRunPage, error) {
	return GameCheckerRunPage{}, errGameCoreDisabled
}

func (noopGameCoreClient) SchedulerStatus(context.Context) (GameSchedulerStatus, error) {
	return GameSchedulerStatus{}, errGameCoreDisabled
}

func (noopGameCoreClient) SchedulerEvents(context.Context, GameSchedulerEventQuery) (GameSchedulerEventPage, error) {
	return GameSchedulerEventPage{}, errGameCoreDisabled
}

func (noopGameCoreClient) StartScheduler(context.Context) (GameSchedulerStatus, error) {
	return GameSchedulerStatus{}, errGameCoreDisabled
}

func (noopGameCoreClient) StopScheduler(context.Context) (GameSchedulerStatus, error) {
	return GameSchedulerStatus{}, errGameCoreDisabled
}

func (noopGameCoreClient) UpdateScheduler(context.Context, UpdateSchedulerRequest) (GameSchedulerStatus, error) {
	return GameSchedulerStatus{}, errGameCoreDisabled
}

func (noopGameCoreClient) SubmitFlags(context.Context, int, []string) ([]submissionVerdict, error) {
	return nil, errGameCoreDisabled
}

func (noopGameCoreClient) Scoreboard(context.Context) ([]scoreRow, error) {
	return nil, errGameCoreDisabled
}

func (noopGameCoreClient) AttackFeed(context.Context, AttackFeedQuery) (AttackFeedPage, error) {
	return AttackFeedPage{}, errGameCoreDisabled
}

func (noopGameCoreClient) RecomputeScoring(context.Context) ([]scoreRow, error) {
	return nil, errGameCoreDisabled
}

func (noopGameCoreClient) AuditScoring(context.Context) (ScoringAuditAlias, error) {
	return ScoringAuditAlias{}, errGameCoreDisabled
}

type httpGameCoreClient struct {
	baseURL string
	token   string
	client  *http.Client
}

type gameCoreHTTPError struct {
	StatusCode int
	Message    string
}

func (e gameCoreHTTPError) Error() string {
	if strings.TrimSpace(e.Message) != "" {
		return e.Message
	}
	return fmt.Sprintf("game core request failed with status %d", e.StatusCode)
}

func NewHTTPGameCoreClient(baseURL, token string) gameCoreClient {
	return NewHTTPGameCoreClientWithTimeout(baseURL, token, 30*time.Second)
}

func NewHTTPGameCoreClientWithTimeout(baseURL, token string, timeout time.Duration) gameCoreClient {
	normalizedBaseURL, ok := httpapi.NormalizeInternalBaseURL(baseURL)
	if !ok {
		return noopGameCoreClient{}
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &httpGameCoreClient{
		baseURL: normalizedBaseURL,
		token:   strings.TrimSpace(token),
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *httpGameCoreClient) Status(ctx context.Context) (GameStatus, error) {
	return requestGameCoreJSON[GameStatus](ctx, c, http.MethodGet, "/internal/v1/game/status")
}

func (c *httpGameCoreClient) MatchStatus(ctx context.Context) (GameMatchStatus, error) {
	return requestGameCoreJSON[GameMatchStatus](ctx, c, http.MethodGet, "/internal/v1/game/match")
}

func (c *httpGameCoreClient) StartMatch(ctx context.Context) (GameMatchStatus, error) {
	return requestGameCoreJSON[GameMatchStatus](ctx, c, http.MethodPost, "/internal/v1/game/match/start")
}

func (c *httpGameCoreClient) StopMatch(ctx context.Context) (GameMatchStatus, error) {
	return requestGameCoreJSON[GameMatchStatus](ctx, c, http.MethodPost, "/internal/v1/game/match/stop")
}

func (c *httpGameCoreClient) UpdateMatchSchedule(ctx context.Context, req UpdateMatchScheduleRequest) (GameMatchStatus, error) {
	return requestGameCoreJSON[GameMatchStatus](ctx, c, http.MethodPut, "/internal/v1/game/match/schedule", req)
}

func (c *httpGameCoreClient) AdvanceTick(ctx context.Context) (GameTickStatus, error) {
	return requestGameCoreJSON[GameTickStatus](ctx, c, http.MethodPost, "/internal/v1/game/ticks/advance")
}

func (c *httpGameCoreClient) CheckerRuns(ctx context.Context, query GameCheckerRunQuery) (GameCheckerRunPage, error) {
	path := "/internal/v1/game/checker-runs"
	values := url.Values{}
	if query.Limit > 0 {
		values.Set("limit", strconv.Itoa(query.Limit))
	}
	if query.Offset > 0 {
		values.Set("offset", strconv.Itoa(query.Offset))
	}
	if query.TickID > 0 {
		values.Set("tick_id", strconv.Itoa(query.TickID))
	}
	if query.TeamID > 0 {
		values.Set("team_id", strconv.Itoa(query.TeamID))
	}
	if query.ChallengeID > 0 {
		values.Set("challenge_id", strconv.Itoa(query.ChallengeID))
	}
	if query.Phase != "" {
		values.Set("phase", query.Phase)
	}
	if query.Status != "" {
		values.Set("status", query.Status)
	}
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}
	return requestGameCoreJSON[GameCheckerRunPage](ctx, c, http.MethodGet, path)
}

func (c *httpGameCoreClient) SchedulerStatus(ctx context.Context) (GameSchedulerStatus, error) {
	return requestGameCoreJSON[GameSchedulerStatus](ctx, c, http.MethodGet, "/internal/v1/game/scheduler")
}

func (c *httpGameCoreClient) SchedulerEvents(ctx context.Context, query GameSchedulerEventQuery) (GameSchedulerEventPage, error) {
	path := "/internal/v1/game/scheduler/events"
	values := url.Values{}
	if query.Limit > 0 {
		values.Set("limit", strconv.Itoa(query.Limit))
	}
	if query.Offset > 0 {
		values.Set("offset", strconv.Itoa(query.Offset))
	}
	if query.EventType != "" {
		values.Set("event_type", query.EventType)
	}
	if query.Source != "" {
		values.Set("source", query.Source)
	}
	if query.State != "" {
		values.Set("state", query.State)
	}
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}
	return requestGameCoreJSON[GameSchedulerEventPage](ctx, c, http.MethodGet, path)
}

func (c *httpGameCoreClient) StartScheduler(ctx context.Context) (GameSchedulerStatus, error) {
	return requestGameCoreJSON[GameSchedulerStatus](ctx, c, http.MethodPost, "/internal/v1/game/scheduler/start")
}

func (c *httpGameCoreClient) StopScheduler(ctx context.Context) (GameSchedulerStatus, error) {
	return requestGameCoreJSON[GameSchedulerStatus](ctx, c, http.MethodPost, "/internal/v1/game/scheduler/stop")
}

func (c *httpGameCoreClient) UpdateScheduler(ctx context.Context, req UpdateSchedulerRequest) (GameSchedulerStatus, error) {
	return requestGameCoreJSON[GameSchedulerStatus](ctx, c, http.MethodPut, "/internal/v1/game/scheduler/interval", req)
}

func (c *httpGameCoreClient) SubmitFlags(ctx context.Context, teamID int, flags []string) ([]submissionVerdict, error) {
	return requestGameCoreJSON[[]submissionVerdict](ctx, c, http.MethodPost, "/internal/v1/flags/submit", GameSubmitFlagsRequest{
		TeamID: teamID,
		Flags:  flags,
	})
}

func (c *httpGameCoreClient) Scoreboard(ctx context.Context) ([]scoreRow, error) {
	return requestGameCoreJSON[[]scoreRow](ctx, c, http.MethodGet, "/internal/v1/game/scoreboard")
}

func (c *httpGameCoreClient) AttackFeed(ctx context.Context, query AttackFeedQuery) (AttackFeedPage, error) {
	path := "/internal/v1/game/attacks"
	values := url.Values{}
	if query.Limit > 0 {
		values.Set("limit", strconv.Itoa(query.Limit))
	}
	if query.Offset > 0 {
		values.Set("offset", strconv.Itoa(query.Offset))
	}
	if query.Attacker != "" {
		values.Set("attacker", query.Attacker)
	}
	if query.Victim != "" {
		values.Set("victim", query.Victim)
	}
	if query.Service != "" {
		values.Set("service", query.Service)
	}
	if query.TickFrom > 0 {
		values.Set("tick_from", strconv.Itoa(query.TickFrom))
	}
	if query.TickTo > 0 {
		values.Set("tick_to", strconv.Itoa(query.TickTo))
	}
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}
	return requestGameCoreJSON[AttackFeedPage](ctx, c, http.MethodGet, path)
}

func (c *httpGameCoreClient) RecomputeScoring(ctx context.Context) ([]scoreRow, error) {
	return requestGameCoreJSON[[]scoreRow](ctx, c, http.MethodPost, "/internal/v1/game/scoring/recompute")
}

func (c *httpGameCoreClient) AuditScoring(ctx context.Context) (ScoringAuditAlias, error) {
	return requestGameCoreJSON[ScoringAuditAlias](ctx, c, http.MethodGet, "/internal/v1/game/scoring/audit")
}

func requestGameCoreJSON[T any](ctx context.Context, c *httpGameCoreClient, method, path string, body ...any) (T, error) {
	var zero T
	var requestBody io.Reader

	if len(body) > 0 && body[0] != nil {
		encodedBody, err := json.Marshal(body[0])
		if err != nil {
			return zero, err
		}
		requestBody = strings.NewReader(string(encodedBody))
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, requestBody) // #nosec G704 -- base URL is validated service configuration.
	if err != nil {
		return zero, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if method != http.MethodGet {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.client.Do(req) // #nosec G704 -- request targets a validated internal game-core service origin.
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return decodeSuccessPayload[T](resp.Body)
	}

	message := decodeErrorMessage(resp.Body)
	return zero, gameCoreHTTPError{StatusCode: resp.StatusCode, Message: message}
}

func decodeSuccessPayload[T any](body io.Reader) (T, error) {
	var zero T
	if err := json.NewDecoder(body).Decode(&zero); err != nil {
		return zero, err
	}
	return zero, nil
}

func decodeErrorMessage(body io.Reader) string {
	data, err := io.ReadAll(body)
	if err != nil {
		return ""
	}
	var problem httpapi.ProblemDetails
	if err := json.Unmarshal(data, &problem); err == nil {
		if trimmed := strings.TrimSpace(problem.Detail); trimmed != "" {
			return trimmed
		}
		if trimmed := strings.TrimSpace(problem.Title); trimmed != "" {
			return trimmed
		}
	}
	var payload struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(data, &payload); err == nil {
		return strings.TrimSpace(payload.Message)
	}
	return ""
}
