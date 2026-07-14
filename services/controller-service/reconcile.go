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

	"adplatform/internal/platform/config"
	"adplatform/internal/platform/httpapi"
	"adplatform/internal/services/apigateway"
)

const (
	controllerReconcileUnavailableDetail  = "controller deployment reconcile is not configured."
	controllerAccessTruthUnknownDetail    = "runtime converge completed but controller access reconcile failed, so host access truth was not established."
	controllerWireGuardTruthUnknownDetail = "runtime and controller access converged but WireGuard converge/verification failed, so host access truth was not established."
)

type controllerDeploymentReconcileState struct {
	ProcessedJobs      int `json:"processed_jobs"`
	ProcessedInstances int `json:"processed_instances"`
	CompletedJobs      int `json:"completed_jobs"`
}

type controllerReconcileStore interface {
	ListControllerRuntimeTasks(context.Context) ([]apigateway.ControllerRuntimeTask, error)
	ListControllerServiceAccessPolicies(context.Context) ([]apigateway.ControllerServiceAccessPolicy, error)
	ReconcileDeployments(context.Context, time.Time) (controllerDeploymentReconcileState, error)
}

type controllerReconcileStoreAdapter struct {
	store apigateway.Store
}

func (a controllerReconcileStoreAdapter) ListControllerRuntimeTasks(ctx context.Context) ([]apigateway.ControllerRuntimeTask, error) {
	return a.store.ListControllerRuntimeTasks(ctx)
}

func (a controllerReconcileStoreAdapter) ListControllerServiceAccessPolicies(ctx context.Context) ([]apigateway.ControllerServiceAccessPolicy, error) {
	return a.store.ListControllerServiceAccessPolicies(ctx)
}

func (a controllerReconcileStoreAdapter) ReconcileDeployments(ctx context.Context, now time.Time) (controllerDeploymentReconcileState, error) {
	result, err := a.store.ReconcileAdminDeployments(ctx, now)
	if err != nil {
		return controllerDeploymentReconcileState{}, err
	}
	return controllerDeploymentReconcileState{
		ProcessedJobs:      result.ProcessedJobs,
		ProcessedInstances: result.ProcessedInstances,
		CompletedJobs:      result.CompletedJobs,
	}, nil
}

type controllerWireGuardReconciler interface {
	Reconcile(context.Context) (apigateway.WireGuardGatewayStatus, error)
}

type noopControllerWireGuardReconciler struct{}

func (noopControllerWireGuardReconciler) Reconcile(context.Context) (apigateway.WireGuardGatewayStatus, error) {
	return apigateway.WireGuardGatewayStatus{}, errWireGuardGatewayUnavailable
}

type httpControllerWireGuardReconciler struct {
	baseURL string
	token   string
	client  *http.Client
}

type controllerReconcilePhaseError struct {
	statusCode int
	title      string
	detail     string
	cause      error
}

func (e *controllerReconcilePhaseError) Error() string {
	return e.detail
}

func (e *controllerReconcilePhaseError) Unwrap() error {
	return e.cause
}

func (e *controllerReconcilePhaseError) StatusCode() int {
	return e.statusCode
}

func (e *controllerReconcilePhaseError) Title() string {
	return e.title
}

func (e *controllerReconcilePhaseError) Detail() string {
	return e.detail
}

type controllerTrustedReconciler struct {
	store     controllerReconcileStore
	executor  runtimeExecutor
	access    serviceAccessExecutor
	wireGuard controllerWireGuardReconciler
	now       func() time.Time
}

type controllerTrustedReconcileOptions struct {
	includeRuntime   bool
	includeAccess    bool
	includeWireGuard bool
}

func newControllerWireGuardReconciler() controllerWireGuardReconciler {
	baseURL, ok := httpapi.NormalizeInternalBaseURL(config.String("WIREGUARD_GATEWAY_INTERNAL_URL", ""))
	if !ok {
		return noopControllerWireGuardReconciler{}
	}
	return &httpControllerWireGuardReconciler{
		baseURL: baseURL,
		token:   strings.TrimSpace(config.Secret("WIREGUARD_GATEWAY_INTERNAL_TOKEN", "ADMIN_API_TOKEN")),
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *httpControllerWireGuardReconciler) Reconcile(ctx context.Context) (apigateway.WireGuardGatewayStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/v1/wireguard/reconcile", nil) // #nosec G704 -- base URL is validated service configuration.
	if err != nil {
		return apigateway.WireGuardGatewayStatus{}, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req) // #nosec G704 -- request targets a validated internal wireguard service origin.
	if err != nil {
		return apigateway.WireGuardGatewayStatus{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var payload apigateway.WireGuardGatewayStatus
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return apigateway.WireGuardGatewayStatus{}, err
		}
		return payload, nil
	}
	message, err := decodeControllerProblemBody(resp.Body)
	if err != nil {
		return apigateway.WireGuardGatewayStatus{}, fmt.Errorf("wireguard gateway request failed with status %d", resp.StatusCode)
	}
	if message == "" {
		return apigateway.WireGuardGatewayStatus{}, fmt.Errorf("wireguard gateway request failed with status %d", resp.StatusCode)
	}
	return apigateway.WireGuardGatewayStatus{}, fmt.Errorf("wireguard gateway request failed: %s", message)
}

var errWireGuardGatewayUnavailable = errors.New("wireguard gateway is not configured")

func newControllerTrustedReconciler(
	store controllerReconcileStore,
	executor runtimeExecutor,
	access serviceAccessExecutor,
	wireGuard controllerWireGuardReconciler,
	now func() time.Time,
) controllerTrustedReconciler {
	if wireGuard == nil {
		wireGuard = noopControllerWireGuardReconciler{}
	}
	if now == nil {
		now = time.Now
	}
	return controllerTrustedReconciler{
		store:     store,
		executor:  executor,
		access:    access,
		wireGuard: wireGuard,
		now:       now,
	}
}

func (r controllerTrustedReconciler) Reconcile(ctx context.Context) (controllerDeploymentReconcileState, error) {
	return r.ReconcileWithOptions(ctx, controllerTrustedReconcileOptions{
		includeRuntime:   true,
		includeAccess:    true,
		includeWireGuard: true,
	})
}

func (r controllerTrustedReconciler) ReconcileWithOptions(ctx context.Context, options controllerTrustedReconcileOptions) (controllerDeploymentReconcileState, error) {
	result := controllerDeploymentReconcileState{}

	if options.includeRuntime {
		tasks, err := r.store.ListControllerRuntimeTasks(ctx)
		if err != nil {
			return controllerDeploymentReconcileState{}, err
		}
		for _, task := range tasks {
			if err := r.executor.EnsureService(ctx, task); err != nil {
				return controllerDeploymentReconcileState{}, fmt.Errorf("runtime reconcile failed for team %d challenge %d: %w", task.TeamID, task.ChallengeID, err)
			}
		}
		reconciled, err := r.store.ReconcileDeployments(ctx, r.now())
		if err != nil {
			return controllerDeploymentReconcileState{}, err
		}
		result = reconciled
	}

	if !options.includeAccess {
		return result, nil
	}

	policies, err := r.store.ListControllerServiceAccessPolicies(ctx)
	if err != nil {
		return controllerDeploymentReconcileState{}, err
	}
	accessStatus, err := r.access.Apply(ctx, policies, r.now())
	if err != nil {
		return result, wrapControllerReconcilePhaseError(http.StatusBadGateway, "Deployment reconcile failed", controllerAccessTruthUnknownDetail, err)
	}

	if !options.includeWireGuard || !requiresWireGuardConverge(accessStatus) {
		return result, nil
	}

	wireGuardStatus, err := r.wireGuard.Reconcile(ctx)
	if err != nil {
		return result, wrapControllerReconcilePhaseError(http.StatusBadGateway, "Deployment reconcile failed", controllerWireGuardTruthUnknownDetail, err)
	}
	if wireGuardErr := validateWireGuardAppliedStatus(wireGuardStatus); wireGuardErr != nil {
		return result, wrapControllerReconcilePhaseError(http.StatusBadGateway, "Deployment reconcile failed", controllerWireGuardTruthUnknownDetail, wireGuardErr)
	}

	return result, nil
}

func wrapControllerReconcilePhaseError(statusCode int, title, detail string, cause error) error {
	if cause == nil {
		return &controllerReconcilePhaseError{
			statusCode: statusCode,
			title:      title,
			detail:     detail,
		}
	}
	return &controllerReconcilePhaseError{
		statusCode: statusCode,
		title:      title,
		detail:     fmt.Sprintf("%s Cause: %s", detail, cause.Error()),
		cause:      cause,
	}
}

func requiresWireGuardConverge(status apigateway.ControllerAccessStatus) bool {
	return strings.EqualFold(strings.TrimSpace(status.Mode), "host")
}

func validateWireGuardAppliedStatus(status apigateway.WireGuardGatewayStatus) error {
	if trimmed := strings.TrimSpace(status.LastError); trimmed != "" {
		return errors.New(trimmed)
	}
	if !strings.EqualFold(strings.TrimSpace(status.State), "applied") {
		return errors.New("wireguard gateway did not report an applied state")
	}
	return nil
}

func decodeControllerProblemBody(body io.Reader) (string, error) {
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
