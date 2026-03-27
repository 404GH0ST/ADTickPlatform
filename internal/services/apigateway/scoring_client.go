package apigateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var errScoringWorkerDisabled = errors.New("scoring worker is not configured")

type scoringClient interface {
	Scoreboard(ctx context.Context) ([]scoreRow, error)
	RecomputeScoring(ctx context.Context) ([]scoreRow, error)
}

type noopScoringClient struct{}

func (noopScoringClient) Scoreboard(context.Context) ([]scoreRow, error) {
	return nil, errScoringWorkerDisabled
}

func (noopScoringClient) RecomputeScoring(context.Context) ([]scoreRow, error) {
	return nil, errScoringWorkerDisabled
}

type httpScoringClient struct {
	baseURL string
	token   string
	client  *http.Client
}

func NewHTTPScoringClient(baseURL, token string) scoringClient {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return noopScoringClient{}
	}
	return &httpScoringClient{
		baseURL: baseURL,
		token:   strings.TrimSpace(token),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *httpScoringClient) Scoreboard(ctx context.Context) ([]scoreRow, error) {
	return requestScoringJSON[[]scoreRow](ctx, c, http.MethodGet, "/internal/v1/scoring/scoreboard")
}

func (c *httpScoringClient) RecomputeScoring(ctx context.Context) ([]scoreRow, error) {
	return requestScoringJSON[[]scoreRow](ctx, c, http.MethodPost, "/internal/v1/scoring/recompute")
}

func requestScoringJSON[T any](ctx context.Context, c *httpScoringClient, method, path string, body ...any) (T, error) {
	var zero T
	var requestBody io.Reader

	if len(body) > 0 && body[0] != nil {
		encodedBody, err := json.Marshal(body[0])
		if err != nil {
			return zero, err
		}
		requestBody = strings.NewReader(string(encodedBody))
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, requestBody)
	if err != nil {
		return zero, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if method != http.MethodGet {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var payload successEnvelope[T]
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return zero, err
		}
		return payload.Data, nil
	}

	var payload struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return zero, fmt.Errorf("scoring-worker request failed with status %d", resp.StatusCode)
	}
	if payload.Message != "" {
		return zero, fmt.Errorf("scoring-worker request failed: %s", payload.Message)
	}
	return zero, fmt.Errorf("scoring-worker request failed with status %d", resp.StatusCode)
}
