package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"adplatform/internal/platform/httpapi"
	"adplatform/internal/services/apigateway"
)

type publicSnapshotClient interface {
	Scoreboard(ctx context.Context) ([]apigateway.ScoreRowAlias, error)
	Attacks(ctx context.Context) (apigateway.AttackFeedPage, error)
	GameStatus(ctx context.Context) (apigateway.GameStatus, error)
	SchedulerEvents(ctx context.Context, limit int) (apigateway.GameSchedulerEventPage, error)
	CheckerRuns(ctx context.Context, limit int) (apigateway.GameCheckerRunPage, error)
}

type httpPublicSnapshotClient struct {
	baseURL    string
	adminToken string
	client     *http.Client
}

func newHTTPPublicSnapshotClient(baseURL, adminToken string) publicSnapshotClient {
	normalizedBaseURL, ok := httpapi.NormalizeInternalBaseURL(baseURL)
	if !ok {
		normalizedBaseURL = "http://127.0.0.1"
	}
	return &httpPublicSnapshotClient{
		baseURL:    normalizedBaseURL,
		adminToken: strings.TrimSpace(adminToken),
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *httpPublicSnapshotClient) Scoreboard(ctx context.Context) ([]apigateway.ScoreRowAlias, error) {
	return fetchSnapshot[[]apigateway.ScoreRowAlias](ctx, c.client, c.baseURL+"/api/v2/scoreboard")
}

func (c *httpPublicSnapshotClient) Attacks(ctx context.Context) (apigateway.AttackFeedPage, error) {
	return fetchSnapshot[apigateway.AttackFeedPage](ctx, c.client, c.baseURL+"/api/v2/attacks")
}

func (c *httpPublicSnapshotClient) GameStatus(ctx context.Context) (apigateway.GameStatus, error) {
	return fetchSnapshotWithToken[apigateway.GameStatus](ctx, c.client, c.baseURL+"/api/v2/admin/game/status", c.adminToken)
}

func (c *httpPublicSnapshotClient) SchedulerEvents(ctx context.Context, limit int) (apigateway.GameSchedulerEventPage, error) {
	url := c.baseURL + "/api/v2/admin/game/scheduler/events"
	if limit > 0 {
		url += "?limit=" + strconv.Itoa(limit)
	}
	return fetchSnapshotWithToken[apigateway.GameSchedulerEventPage](ctx, c.client, url, c.adminToken)
}

func (c *httpPublicSnapshotClient) CheckerRuns(ctx context.Context, limit int) (apigateway.GameCheckerRunPage, error) {
	url := c.baseURL + "/api/v2/admin/game/checker-runs"
	if limit > 0 {
		url += "?limit=" + strconv.Itoa(limit)
	}
	return fetchSnapshotWithToken[apigateway.GameCheckerRunPage](ctx, c.client, url, c.adminToken)
}

func fetchSnapshot[T any](ctx context.Context, client *http.Client, url string) (T, error) {
	return fetchSnapshotWithToken[T](ctx, client, url, "")
}

func fetchSnapshotWithToken[T any](ctx context.Context, client *http.Client, url, token string) (T, error) {
	var zero T

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil) // #nosec G704 -- snapshot URL is built from a validated base origin and fixed paths.
	if err != nil {
		return zero, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req) // #nosec G704 -- request targets a validated platform snapshot origin.
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return zero, fmt.Errorf("snapshot request failed with status %d", resp.StatusCode)
		}
		var problem httpapi.ProblemDetails
		if err := json.Unmarshal(data, &problem); err == nil {
			if trimmed := strings.TrimSpace(problem.Detail); trimmed != "" {
				return zero, fmt.Errorf("snapshot request failed: %s", trimmed)
			}
			if trimmed := strings.TrimSpace(problem.Title); trimmed != "" {
				return zero, fmt.Errorf("snapshot request failed: %s", trimmed)
			}
		}
		return zero, fmt.Errorf("snapshot request failed with status %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(&zero); err != nil {
		return zero, err
	}
	return zero, nil
}
