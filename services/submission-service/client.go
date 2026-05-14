package main

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
	"adplatform/internal/services/apigateway"
)

var errGameCoreSubmissionDisabled = errors.New("game core is not configured")

type noopGameCoreSubmissionClient struct{}

func (noopGameCoreSubmissionClient) SubmitFlags(context.Context, int, []string) ([]apigateway.SubmissionVerdictAlias, error) {
	return nil, errGameCoreSubmissionDisabled
}

func (noopGameCoreSubmissionClient) AttackFeed(context.Context, apigateway.AttackFeedQuery) (apigateway.AttackFeedPage, error) {
	return apigateway.AttackFeedPage{}, errGameCoreSubmissionDisabled
}

type httpGameCoreSubmissionClient struct {
	baseURL string
	token   string
	client  *http.Client
}

func newGameCoreSubmissionClient(baseURL, token string) submissionGameCoreClient {
	normalizedBaseURL, ok := httpapi.NormalizeInternalBaseURL(baseURL)
	if !ok {
		return noopGameCoreSubmissionClient{}
	}
	return &httpGameCoreSubmissionClient{
		baseURL: normalizedBaseURL,
		token:   strings.TrimSpace(token),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *httpGameCoreSubmissionClient) SubmitFlags(ctx context.Context, teamID int, flags []string) ([]apigateway.SubmissionVerdictAlias, error) {
	return requestGameCoreSubmissionJSON[[]apigateway.SubmissionVerdictAlias](ctx, c, http.MethodPost, "/internal/v1/flags/submit", apigateway.GameSubmitFlagsRequest{
		TeamID: teamID,
		Flags:  flags,
	})
}

func (c *httpGameCoreSubmissionClient) AttackFeed(ctx context.Context, query apigateway.AttackFeedQuery) (apigateway.AttackFeedPage, error) {
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
	return requestGameCoreSubmissionJSON[apigateway.AttackFeedPage](ctx, c, http.MethodGet, path)
}

func requestGameCoreSubmissionJSON[T any](ctx context.Context, c *httpGameCoreSubmissionClient, method, path string, body ...any) (T, error) {
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

	var payload struct {
		Detail  string `json:"detail"`
		Title   string `json:"title"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return zero, fmt.Errorf("game-core request failed with status %d", resp.StatusCode)
	}
	if strings.TrimSpace(payload.Detail) != "" {
		return zero, fmt.Errorf("game-core request failed: %s", payload.Detail)
	}
	if strings.TrimSpace(payload.Message) != "" {
		return zero, fmt.Errorf("game-core request failed: %s", payload.Message)
	}
	if strings.TrimSpace(payload.Title) != "" {
		return zero, fmt.Errorf("game-core request failed: %s", payload.Title)
	}
	return zero, fmt.Errorf("game-core request failed with status %d", resp.StatusCode)
}
