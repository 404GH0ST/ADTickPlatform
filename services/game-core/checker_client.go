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
	ExecuteBatch(ctx context.Context, request apigateway.CheckerBatchExecutionRequest) (apigateway.CheckerBatchExecutionResult, error)
}

// executeBatchViaSingle runs the phases of a batch request one Execute call at a
// time, chaining each successful phase's output into the next as metadata and
// halting on the first non-success phase. It backs checker clients that have no
// native batch path (dry-run, tests) so they keep identical phase semantics to a
// real single-container batch.
func executeBatchViaSingle(
	ctx context.Context,
	exec func(context.Context, apigateway.CheckerExecutionRequest) (apigateway.CheckerExecutionResult, error),
	request apigateway.CheckerBatchExecutionRequest,
) (apigateway.CheckerBatchExecutionResult, error) {
	result := apigateway.CheckerBatchExecutionResult{
		ChallengeID: request.ChallengeID,
		TeamID:      request.TeamID,
	}
	metadata := request.Metadata
	for _, phase := range request.Phases {
		res, err := exec(ctx, apigateway.CheckerExecutionRequest{
			ChallengeID:    request.ChallengeID,
			TeamID:         request.TeamID,
			TeamName:       request.TeamName,
			ChallengeName:  request.ChallengeName,
			CheckerImage:   request.CheckerImage,
			Phase:          phase,
			Target:         request.Target,
			TargetHost:     request.TargetHost,
			TargetIP:       request.TargetIP,
			TargetPort:     request.TargetPort,
			TickID:         request.TickID,
			Flag:           request.Flag,
			Metadata:       metadata,
			CheckerToken:   request.CheckerToken,
			TimeoutSeconds: request.TimeoutSeconds,
		})
		if err != nil {
			return result, err
		}
		result.Phases = append(result.Phases, apigateway.CheckerPhaseResult{
			Phase:        phase,
			Status:       res.Status,
			ExitCode:     res.ExitCode,
			CheckedAt:    res.CheckedAt,
			Message:      res.Message,
			Output:       res.Output,
			ServiceState: res.ServiceState,
			StateMessage: res.StateMessage,
		})
		if normalizedCheckerRunStatus(res.Status) != "success" {
			break
		}
		if res.Output != "" {
			metadata = res.Output
		}
	}
	return result, nil
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

func (c noopCheckerClient) ExecuteBatch(ctx context.Context, request apigateway.CheckerBatchExecutionRequest) (apigateway.CheckerBatchExecutionResult, error) {
	return executeBatchViaSingle(ctx, c.Execute, request)
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
			// A batch call runs every phase in one container, so the HTTP deadline
			// must cover the summed per-phase checker timeouts plus overhead.
			Timeout: 150 * time.Second,
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

func (c *httpCheckerClient) ExecuteBatch(ctx context.Context, request apigateway.CheckerBatchExecutionRequest) (apigateway.CheckerBatchExecutionResult, error) {
	reqBody, err := json.Marshal(request)
	if err != nil {
		return apigateway.CheckerBatchExecutionResult{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/v1/checkers/execute-batch", strings.NewReader(string(reqBody))) // #nosec G704 -- base URL is validated service configuration.
	if err != nil {
		return apigateway.CheckerBatchExecutionResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.client.Do(req) // #nosec G704 -- request targets a validated internal checker service origin.
	if err != nil {
		return apigateway.CheckerBatchExecutionResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var payload apigateway.CheckerBatchExecutionResult
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return apigateway.CheckerBatchExecutionResult{}, err
		}
		return payload, nil
	}
	message, err := decodeCheckerRunnerError(resp.Body)
	if err != nil {
		return apigateway.CheckerBatchExecutionResult{}, fmt.Errorf("checker-runner request failed with status %d", resp.StatusCode)
	}
	if message != "" {
		return apigateway.CheckerBatchExecutionResult{}, fmt.Errorf("checker-runner request failed: %s", message)
	}
	return apigateway.CheckerBatchExecutionResult{}, fmt.Errorf("checker-runner request failed with status %d", resp.StatusCode)
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
