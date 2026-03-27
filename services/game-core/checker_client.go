package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

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
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return noopCheckerClient{}
	}
	return &httpCheckerClient{
		baseURL: baseURL,
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

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/v1/checkers/execute", strings.NewReader(string(reqBody)))
	if err != nil {
		return apigateway.CheckerExecutionResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return apigateway.CheckerExecutionResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var payload successEnvelope[apigateway.CheckerExecutionResult]
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return apigateway.CheckerExecutionResult{}, err
		}
		return payload.Data, nil
	}

	var payload struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return apigateway.CheckerExecutionResult{}, fmt.Errorf("checker-runner request failed with status %d", resp.StatusCode)
	}
	if payload.Message != "" {
		return apigateway.CheckerExecutionResult{}, fmt.Errorf("checker-runner request failed: %s", payload.Message)
	}
	return apigateway.CheckerExecutionResult{}, fmt.Errorf("checker-runner request failed with status %d", resp.StatusCode)
}
