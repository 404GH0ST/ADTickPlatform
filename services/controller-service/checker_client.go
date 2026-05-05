package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"adplatform/internal/platform/config"
	"adplatform/internal/platform/httpapi"
	"adplatform/internal/services/apigateway"
)

type checkerValidationClient interface {
	Validate(ctx context.Context, request apigateway.CheckerValidationRequest) (apigateway.CheckerValidationResult, error)
}

type httpCheckerValidationClient struct {
	baseURL string
	token   string
	client  *http.Client
}

func newCheckerValidationClient() checkerValidationClient {
	baseURL := strings.TrimRight(strings.TrimSpace(config.String("CHECKER_RUNNER_INTERNAL_URL", "")), "/")
	if baseURL == "" {
		return nil
	}
	return &httpCheckerValidationClient{
		baseURL: baseURL,
		token:   strings.TrimSpace(config.String("CHECKER_RUNNER_INTERNAL_TOKEN", config.String("ADMIN_API_TOKEN", "dev-admin-token"))),
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *httpCheckerValidationClient) Validate(ctx context.Context, request apigateway.CheckerValidationRequest) (apigateway.CheckerValidationResult, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return apigateway.CheckerValidationResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/v1/checkers/validate", strings.NewReader(string(body)))
	if err != nil {
		return apigateway.CheckerValidationResult{}, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return apigateway.CheckerValidationResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var payload apigateway.CheckerValidationResult
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return apigateway.CheckerValidationResult{}, err
		}
		return payload, nil
	}

	var problem httpapi.ProblemDetails
	if err := json.NewDecoder(resp.Body).Decode(&problem); err != nil {
		return apigateway.CheckerValidationResult{}, fmt.Errorf("checker-runner request failed with status %d", resp.StatusCode)
	}
	if strings.TrimSpace(problem.Detail) != "" {
		return apigateway.CheckerValidationResult{}, fmt.Errorf("checker-runner request failed: %s", strings.TrimSpace(problem.Detail))
	}
	if strings.TrimSpace(problem.Title) != "" {
		return apigateway.CheckerValidationResult{}, fmt.Errorf("checker-runner request failed: %s", strings.TrimSpace(problem.Title))
	}
	return apigateway.CheckerValidationResult{}, fmt.Errorf("checker-runner request failed with status %d", resp.StatusCode)
}

func cloneCheckerValidationClient(baseURL, token string, client *http.Client) *httpCheckerValidationClient {
	return &httpCheckerValidationClient{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		token:   strings.TrimSpace(token),
		client:  client,
	}
}

func decodeCheckerValidationRequest(body io.Reader) (apigateway.CheckerValidationRequest, error) {
	var request apigateway.CheckerValidationRequest
	if err := json.NewDecoder(body).Decode(&request); err != nil {
		return apigateway.CheckerValidationRequest{}, err
	}
	return request, nil
}
