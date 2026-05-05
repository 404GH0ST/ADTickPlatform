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

	"adplatform/internal/platform/httpapi"
)

var errControllerDisabled = errors.New("controller is not configured")

type controllerProblemError struct {
	statusCode int
	title      string
	detail     string
}

func (e *controllerProblemError) Error() string {
	if trimmed := strings.TrimSpace(e.detail); trimmed != "" {
		return trimmed
	}
	if trimmed := strings.TrimSpace(e.title); trimmed != "" {
		return trimmed
	}
	return fmt.Sprintf("controller request failed with status %d", e.statusCode)
}

func (e *controllerProblemError) StatusCode() int {
	return e.statusCode
}

func (e *controllerProblemError) Title() string {
	if trimmed := strings.TrimSpace(e.title); trimmed != "" {
		return trimmed
	}
	return "Controller request failed"
}

func (e *controllerProblemError) Detail() string {
	if trimmed := strings.TrimSpace(e.detail); trimmed != "" {
		return trimmed
	}
	return e.Title()
}

type controllerClient interface {
	ReconcileDeployments(ctx context.Context) (adminReconcileResult, error)
	FactoryResetService(ctx context.Context, teamID, challengeID int) error
	RestartService(ctx context.Context, teamID, challengeID int) error
	ReconcileServiceAccess(ctx context.Context, teamID, challengeID int) error
	ApplySSHCredential(ctx context.Context, teamID, challengeID int, credential ControllerSSHCredential) error
	ValidateChallengeRuntime(ctx context.Context, request ChallengeValidationRequest) (ChallengeValidationResult, error)
	AccessStatus(ctx context.Context) (ControllerAccessStatus, error)
	ReconcileAccessPolicies(ctx context.Context) (ControllerAccessStatus, error)
	TeardownAccessPolicies(ctx context.Context) error
	RemoveService(ctx context.Context, teamID, challengeID int) error
	RemoveTeamServices(ctx context.Context, teamID int) error
	RemoveChallengeServices(ctx context.Context, challengeID int) error
}

type noopControllerClient struct{}

func (noopControllerClient) ReconcileDeployments(context.Context) (adminReconcileResult, error) {
	return adminReconcileResult{}, errControllerDisabled
}
func (noopControllerClient) FactoryResetService(context.Context, int, int) error { return nil }
func (noopControllerClient) RestartService(context.Context, int, int) error      { return nil }
func (noopControllerClient) ReconcileServiceAccess(context.Context, int, int) error {
	return nil
}
func (noopControllerClient) ApplySSHCredential(context.Context, int, int, ControllerSSHCredential) error {
	return nil
}
func (noopControllerClient) ValidateChallengeRuntime(_ context.Context, request ChallengeValidationRequest) (ChallengeValidationResult, error) {
	return ChallengeValidationResult{
		ChallengeID:           request.ChallengeID,
		Name:                  request.Name,
		BaselineImage:         request.BaselineImage,
		CheckerImage:          request.CheckerImage,
		Status:                "unavailable",
		BaselineSSHContractOK: false,
		CheckerContractOK:     false,
		CheckedAt:             time.Now().UTC().Format(time.RFC3339),
		Message:               "controller runtime validation is not configured.",
	}, nil
}
func (noopControllerClient) AccessStatus(context.Context) (ControllerAccessStatus, error) {
	return ControllerAccessStatus{State: "disabled", Mode: "disabled"}, nil
}
func (noopControllerClient) ReconcileAccessPolicies(context.Context) (ControllerAccessStatus, error) {
	return ControllerAccessStatus{State: "disabled", Mode: "disabled"}, nil
}
func (noopControllerClient) TeardownAccessPolicies(context.Context) error  { return nil }
func (noopControllerClient) RemoveService(context.Context, int, int) error { return nil }
func (noopControllerClient) RemoveTeamServices(context.Context, int) error { return nil }
func (noopControllerClient) RemoveChallengeServices(context.Context, int) error {
	return nil
}

type httpControllerClient struct {
	baseURL string
	token   string
	client  *http.Client
}

func NewHTTPControllerClient(baseURL, token string) controllerClient {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return noopControllerClient{}
	}
	return &httpControllerClient{
		baseURL: baseURL,
		token:   strings.TrimSpace(token),
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *httpControllerClient) FactoryResetService(ctx context.Context, teamID, challengeID int) error {
	return c.post(ctx, fmt.Sprintf("/internal/v1/teams/%d/services/%d/reset/factory", teamID, challengeID))
}

func (c *httpControllerClient) ReconcileDeployments(ctx context.Context) (adminReconcileResult, error) {
	return requestControllerJSON[adminReconcileResult](ctx, c, http.MethodPost, "/internal/v1/deployments/reconcile", nil)
}

func (c *httpControllerClient) RestartService(ctx context.Context, teamID, challengeID int) error {
	return c.post(ctx, fmt.Sprintf("/internal/v1/teams/%d/services/%d/restart", teamID, challengeID))
}

func (c *httpControllerClient) ReconcileServiceAccess(ctx context.Context, teamID, challengeID int) error {
	return c.post(ctx, fmt.Sprintf("/internal/v1/teams/%d/services/%d/access/reconcile", teamID, challengeID))
}

func (c *httpControllerClient) ApplySSHCredential(ctx context.Context, teamID, challengeID int, credential ControllerSSHCredential) error {
	return c.postJSON(ctx, fmt.Sprintf("/internal/v1/teams/%d/services/%d/ssh-credential", teamID, challengeID), credential)
}

func (c *httpControllerClient) ValidateChallengeRuntime(ctx context.Context, request ChallengeValidationRequest) (ChallengeValidationResult, error) {
	return requestControllerJSON[ChallengeValidationResult](ctx, c, http.MethodPost, "/internal/v1/challenges/validate", request)
}

func (c *httpControllerClient) AccessStatus(ctx context.Context) (ControllerAccessStatus, error) {
	return requestControllerJSON[ControllerAccessStatus](ctx, c, http.MethodGet, "/internal/v1/access/status", nil)
}

func (c *httpControllerClient) ReconcileAccessPolicies(ctx context.Context) (ControllerAccessStatus, error) {
	return requestControllerJSON[ControllerAccessStatus](ctx, c, http.MethodPost, "/internal/v1/access/reconcile", nil)
}

func (c *httpControllerClient) TeardownAccessPolicies(ctx context.Context) error {
	return c.post(ctx, "/internal/v1/access/teardown")
}

func (c *httpControllerClient) RemoveService(ctx context.Context, teamID, challengeID int) error {
	return c.post(ctx, fmt.Sprintf("/internal/v1/teams/%d/services/%d/remove", teamID, challengeID))
}

func (c *httpControllerClient) RemoveTeamServices(ctx context.Context, teamID int) error {
	return c.post(ctx, fmt.Sprintf("/internal/v1/teams/%d/remove", teamID))
}

func (c *httpControllerClient) RemoveChallengeServices(ctx context.Context, challengeID int) error {
	return c.post(ctx, fmt.Sprintf("/internal/v1/challenges/%d/remove", challengeID))
}

func (c *httpControllerClient) post(ctx context.Context, path string) error {
	return c.postJSON(ctx, path, nil)
}

func (c *httpControllerClient) postJSON(ctx context.Context, path string, body any) error {
	var requestBody io.Reader
	if body != nil {
		encodedBody, err := json.Marshal(body)
		if err != nil {
			return err
		}
		requestBody = strings.NewReader(string(encodedBody))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, requestBody)
	if err != nil {
		return err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if resp.StatusCode == http.StatusNoContent {
			return nil
		}
		return nil
	}
	problem, err := decodeProblemDetails(resp.Body)
	if err != nil {
		return fmt.Errorf("controller request failed with status %d", resp.StatusCode)
	}
	if resp.StatusCode == http.StatusNotFound {
		return ErrChallengeNotFound
	}
	return &controllerProblemError{
		statusCode: resp.StatusCode,
		title:      problem.Title,
		detail:     problem.Detail,
	}
}

func requestControllerJSON[T any](ctx context.Context, c *httpControllerClient, method, path string, body any) (T, error) {
	var zero T
	var requestBody io.Reader
	if body != nil {
		encodedBody, err := json.Marshal(body)
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
	if method != http.MethodGet || body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.client.Do(req)
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
	problem, err := decodeProblemDetails(resp.Body)
	if err != nil {
		return zero, fmt.Errorf("controller request failed with status %d", resp.StatusCode)
	}
	return zero, &controllerProblemError{
		statusCode: resp.StatusCode,
		title:      problem.Title,
		detail:     problem.Detail,
	}
}

func decodeProblemDetails(body io.Reader) (httpapi.ProblemDetails, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return httpapi.ProblemDetails{}, err
	}
	var problem httpapi.ProblemDetails
	if err := json.Unmarshal(data, &problem); err == nil {
		return problem, nil
	}
	var payload struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return httpapi.ProblemDetails{}, err
	}
	return httpapi.ProblemDetails{
		Title:  "Controller request failed",
		Detail: strings.TrimSpace(payload.Message),
	}, nil
}
