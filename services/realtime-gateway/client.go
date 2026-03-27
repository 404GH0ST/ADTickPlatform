package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

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

type successEnvelope[T any] struct {
	Status string `json:"status"`
	Data   T      `json:"data"`
}

func newHTTPPublicSnapshotClient(baseURL, adminToken string) publicSnapshotClient {
	return &httpPublicSnapshotClient{
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
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

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return zero, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return zero, fmt.Errorf("snapshot request failed with status %d", resp.StatusCode)
	}

	var payload successEnvelope[T]
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return zero, err
	}
	return payload.Data, nil
}
