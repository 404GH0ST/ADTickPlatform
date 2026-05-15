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

	"adplatform/internal/platform/httpapi"
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

func (noopGameCoreScoringClient) AuditScoring(context.Context) (apigateway.ScoringAuditAlias, error) {
	return apigateway.ScoringAuditAlias{}, errGameCoreScoringDisabled
}

type httpGameCoreScoringClient struct {
	baseURL string
	token   string
	client  *http.Client
}

func newGameCoreScoringClient(baseURL, token string) scoringGameCoreClient {
	normalizedBaseURL, ok := httpapi.NormalizeInternalBaseURL(baseURL)
	if !ok {
		return noopGameCoreScoringClient{}
	}
	return &httpGameCoreScoringClient{
		baseURL: normalizedBaseURL,
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

func (c *httpGameCoreScoringClient) AuditScoring(ctx context.Context) (apigateway.ScoringAuditAlias, error) {
	return requestGameCoreScoringJSON[apigateway.ScoringAuditAlias](ctx, c, http.MethodGet, "/internal/v1/game/scoring/audit")
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

	resp, err := c.client.Do(req) // #nosec G704 -- request targets a validated internal service origin.
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if err := json.NewDecoder(resp.Body).Decode(&zero); err != nil {
			return zero, err
		}
		return zero, nil
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return zero, fmt.Errorf("game-core request failed with status %d", resp.StatusCode)
	}
	var problem httpapi.ProblemDetails
	if err := json.Unmarshal(data, &problem); err == nil {
		if trimmed := strings.TrimSpace(problem.Detail); trimmed != "" {
			return zero, fmt.Errorf("game-core request failed: %s", trimmed)
		}
		if trimmed := strings.TrimSpace(problem.Title); trimmed != "" {
			return zero, fmt.Errorf("game-core request failed: %s", trimmed)
		}
	}
	var payload struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return zero, fmt.Errorf("game-core request failed with status %d", resp.StatusCode)
	}
	if payload.Message != "" {
		return zero, fmt.Errorf("game-core request failed: %s", payload.Message)
	}
	return zero, fmt.Errorf("game-core request failed with status %d", resp.StatusCode)
}
