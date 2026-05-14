package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"adplatform/internal/platform/httpapi"
	"adplatform/internal/services/apigateway"
)

type checkerClient interface {
	Execute(ctx context.Context, request apigateway.CheckerExecutionRequest) (apigateway.CheckerExecutionResult, error)
}

type noopCheckerClient struct{}

func (noopCheckerClient) Execute(_ context.Context, request apigateway.CheckerExecutionRequest) (apigateway.CheckerExecutionResult, error) {
	return apigateway.CheckerExecutionResult{
		ChallengeID: request.ChallengeID,
		TeamID:      request.TeamID,
		Phase:       request.Phase,
		Status:      "success",
		ExitCode:    0,
		CheckedAt:   time.Now().UTC().Format(time.RFC3339),
		Message:     "checker execution assumed in game-core dry-run mode.",
		Output:      "dry-run checker execution",
	}, nil
}

type httpCheckerClient struct {
	baseURL string
	token   string
	client  *http.Client
}

func newCheckerClient(baseURL, token string) checkerClient {
	normalizedBaseURL, ok := httpapi.NormalizeInternalBaseURL(baseURL)
	if !ok {
		return noopCheckerClient{}
	}
	return &httpCheckerClient{
		baseURL: normalizedBaseURL,
		token:   strings.TrimSpace(token),
		client: &http.Client{
			Timeout: 45 * time.Second,
		},
	}
}

func (c *httpCheckerClient) Execute(ctx context.Context, request apigateway.CheckerExecutionRequest) (apigateway.CheckerExecutionResult, error) {
	reqBody, err := json.Marshal(request)
	if err != nil {
		return apigateway.CheckerExecutionResult{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/v1/checkers/execute", strings.NewReader(string(reqBody))) // #nosec G704 -- base URL is validated service configuration.
	if err != nil {
		return apigateway.CheckerExecutionResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.client.Do(req) // #nosec G704 -- request targets a validated internal checker service origin.
	if err != nil {
		return apigateway.CheckerExecutionResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var payload apigateway.CheckerExecutionResult
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return apigateway.CheckerExecutionResult{}, err
		}
		return payload, nil
	}
	message, err := decodeCheckerRunnerError(resp.Body)
	if err != nil {
		return apigateway.CheckerExecutionResult{}, fmt.Errorf("checker-runner request failed with status %d", resp.StatusCode)
	}
	if message != "" {
		return apigateway.CheckerExecutionResult{}, fmt.Errorf("checker-runner request failed: %s", message)
	}
	return apigateway.CheckerExecutionResult{}, fmt.Errorf("checker-runner request failed with status %d", resp.StatusCode)
}

func decodeCheckerRunnerError(body io.Reader) (string, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return "", err
	}
	var problem httpapi.ProblemDetails
	if err := json.Unmarshal(data, &problem); err == nil {
		if trimmed := strings.TrimSpace(problem.Detail); trimmed != "" {
			return trimmed, nil
		}
		if trimmed := strings.TrimSpace(problem.Title); trimmed != "" {
			return trimmed, nil
		}
	}
	var payload struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", err
	}
	return strings.TrimSpace(payload.Message), nil
}
