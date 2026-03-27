package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"adplatform/internal/services/apigateway"
)

var errGameCoreScoringDisabled = errors.New("game core is not configured")

type noopGameCoreScoringClient struct{}

func (noopGameCoreScoringClient) Scoreboard(context.Context) ([]apigateway.ScoreRowAlias, error) {
	return nil, errGameCoreScoringDisabled
}

func (noopGameCoreScoringClient) RecomputeScoring(context.Context) ([]apigateway.ScoreRowAlias, error) {
	return nil, errGameCoreScoringDisabled
}

type httpGameCoreScoringClient struct {
	baseURL string
	token   string
	client  *http.Client
}

func newGameCoreScoringClient(baseURL, token string) scoringGameCoreClient {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return noopGameCoreScoringClient{}
	}
	return &httpGameCoreScoringClient{
		baseURL: baseURL,
		token:   strings.TrimSpace(token),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *httpGameCoreScoringClient) Scoreboard(ctx context.Context) ([]apigateway.ScoreRowAlias, error) {
	return requestGameCoreScoringJSON[[]apigateway.ScoreRowAlias](ctx, c, http.MethodGet, "/internal/v1/game/scoreboard")
}

func (c *httpGameCoreScoringClient) RecomputeScoring(ctx context.Context) ([]apigateway.ScoreRowAlias, error) {
	return requestGameCoreScoringJSON[[]apigateway.ScoreRowAlias](ctx, c, http.MethodPost, "/internal/v1/game/scoring/recompute")
}

func requestGameCoreScoringJSON[T any](ctx context.Context, c *httpGameCoreScoringClient, method, path string, body ...any) (T, error) {
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
		return zero, fmt.Errorf("game-core request failed with status %d", resp.StatusCode)
	}
	if payload.Message != "" {
		return zero, fmt.Errorf("game-core request failed: %s", payload.Message)
	}
	return zero, fmt.Errorf("game-core request failed with status %d", resp.StatusCode)
}
