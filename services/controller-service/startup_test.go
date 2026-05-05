package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"adplatform/internal/services/apigateway"
)

type controllerStartupStoreStub struct {
	tasks           []apigateway.ControllerRuntimeTask
	policies        []apigateway.ControllerServiceAccessPolicy
	reconcileCalls  int
	reconcileResult controllerDeploymentReconcileState
	listTaskCalls   int
	listPolicyCalls int
	reconcileErr    error
	listTasksErr    error
	listPoliciesErr error
}

func (s *controllerStartupStoreStub) ListControllerRuntimeTasks(context.Context) ([]apigateway.ControllerRuntimeTask, error) {
	s.listTaskCalls++
	return s.tasks, s.listTasksErr
}

func (s *controllerStartupStoreStub) ListControllerServiceAccessPolicies(context.Context) ([]apigateway.ControllerServiceAccessPolicy, error) {
	s.listPolicyCalls++
	return s.policies, s.listPoliciesErr
}

func (s *controllerStartupStoreStub) ReconcileDeployments(context.Context, time.Time) (controllerDeploymentReconcileState, error) {
	s.reconcileCalls++
	if s.reconcileErr != nil {
		return controllerDeploymentReconcileState{}, s.reconcileErr
	}
	return s.reconcileResult, nil
}

type runtimeExecutorStub struct {
	ensured []apigateway.ControllerRuntimeTask
	err     error
}

func (s *runtimeExecutorStub) EnsureService(_ context.Context, task apigateway.ControllerRuntimeTask) error {
	s.ensured = append(s.ensured, task)
	return s.err
}

func (*runtimeExecutorStub) FactoryResetService(context.Context, apigateway.ControllerRuntimeTask) error {
	return nil
}

func (*runtimeExecutorStub) RestartService(context.Context, apigateway.ControllerRuntimeTask) error {
	return nil
}

func (*runtimeExecutorStub) ApplySSHCredential(context.Context, apigateway.ControllerRuntimeTask, apigateway.ControllerSSHCredential) error {
	return nil
}

func (*runtimeExecutorStub) ValidateChallengeRuntime(context.Context, apigateway.ChallengeValidationRequest) (apigateway.ChallengeValidationResult, error) {
	return apigateway.ChallengeValidationResult{}, nil
}

func (*runtimeExecutorStub) RemoveService(context.Context, int, int) error {
	return nil
}

func (*runtimeExecutorStub) RemoveTeamServices(context.Context, int) error {
	return nil
}

func (*runtimeExecutorStub) RemoveChallengeServices(context.Context, int) error {
	return nil
}

type serviceAccessExecutorStub struct {
	policies []apigateway.ControllerServiceAccessPolicy
	err      error
	mode     string
}

func (*serviceAccessExecutorStub) Status() apigateway.ControllerAccessStatus {
	return apigateway.ControllerAccessStatus{}
}

func (s *serviceAccessExecutorStub) Apply(_ context.Context, policies []apigateway.ControllerServiceAccessPolicy, now time.Time) (apigateway.ControllerAccessStatus, error) {
	s.policies = append([]apigateway.ControllerServiceAccessPolicy(nil), policies...)
	if s.err != nil {
		return apigateway.ControllerAccessStatus{State: "error", LastError: s.err.Error()}, s.err
	}
	mode := s.mode
	if mode == "" {
		mode = "host"
	}
	return apigateway.ControllerAccessStatus{
		State:         "applied",
		Mode:          mode,
		PoliciesTotal: len(policies),
		AppliedAt:     now.UTC().Format(time.RFC3339),
	}, nil
}

func (s *serviceAccessExecutorStub) Teardown(_ context.Context) error {
	return nil
}

type controllerWireGuardReconcilerStub struct {
	status apigateway.WireGuardGatewayStatus
	err    error
	calls  int
}

func (s *controllerWireGuardReconcilerStub) Reconcile(context.Context) (apigateway.WireGuardGatewayStatus, error) {
	s.calls++
	if s.err != nil {
		return apigateway.WireGuardGatewayStatus{}, s.err
	}
	if s.status == (apigateway.WireGuardGatewayStatus{}) {
		return apigateway.WireGuardGatewayStatus{State: "applied", Mode: "host"}, nil
	}
	return s.status, nil
}

func TestRestoreControllerStateAppliesQueuedDeploymentsAndAccessPolicies(t *testing.T) {
	t.Setenv("CONTROLLER_RECONCILE_DEPLOYMENTS_ON_STARTUP", "true")
	t.Setenv("CONTROLLER_RECONCILE_ACCESS_ON_STARTUP", "true")

	store := &controllerStartupStoreStub{
		tasks: []apigateway.ControllerRuntimeTask{
			{TeamID: 101, ChallengeID: 3, ContainerName: "svc-storage-team-101", BaselineImage: "registry.local/storage:baseline"},
		},
		policies: []apigateway.ControllerServiceAccessPolicy{
			{TeamID: 101, ChallengeID: 3, ServiceIP: "10.80.3.11", ServicePort: 10003, SSHPort: 22, SSHUnlocked: true, AllowedPeerAddresses: []string{"10.70.11.20"}},
		},
	}
	executor := &runtimeExecutorStub{}
	access := &serviceAccessExecutorStub{}
	wireGuard := &controllerWireGuardReconcilerStub{}
	now := func() time.Time { return time.Date(2026, time.March, 11, 9, 0, 0, 0, time.UTC) }

	if err := restoreControllerState(context.Background(), store, executor, access, wireGuard, now); err != nil {
		t.Fatalf("expected startup restore to succeed, got %v", err)
	}
	if len(executor.ensured) != 1 {
		t.Fatalf("expected 1 ensured runtime task, got %d", len(executor.ensured))
	}
	if store.reconcileCalls != 1 {
		t.Fatalf("expected deployment reconcile to run once, got %d", store.reconcileCalls)
	}
	if len(access.policies) != 1 {
		t.Fatalf("expected access apply to receive 1 policy, got %d", len(access.policies))
	}
	if wireGuard.calls != 1 {
		t.Fatalf("expected wireguard reconcile to run once, got %d", wireGuard.calls)
	}
}

func TestRestoreControllerStateFailsFastOnAccessRestoreError(t *testing.T) {
	t.Setenv("CONTROLLER_RECONCILE_DEPLOYMENTS_ON_STARTUP", "false")
	t.Setenv("CONTROLLER_RECONCILE_ACCESS_ON_STARTUP", "true")

	store := &controllerStartupStoreStub{
		policies: []apigateway.ControllerServiceAccessPolicy{
			{TeamID: 101, ChallengeID: 3, ServiceIP: "10.80.3.11", ServicePort: 10003, SSHPort: 22},
		},
	}
	access := &serviceAccessExecutorStub{err: errors.New("nft apply failed")}

	err := restoreControllerState(context.Background(), store, &runtimeExecutorStub{}, access, &controllerWireGuardReconcilerStub{}, time.Now)
	if err == nil || !strings.Contains(err.Error(), controllerAccessTruthUnknownDetail) {
		t.Fatalf("expected startup restore to fail on access error, got %v", err)
	}
}

func TestRestoreControllerStateSkipsWireGuardOutsideHostMode(t *testing.T) {
	t.Setenv("CONTROLLER_RECONCILE_DEPLOYMENTS_ON_STARTUP", "true")
	t.Setenv("CONTROLLER_RECONCILE_ACCESS_ON_STARTUP", "true")

	store := &controllerStartupStoreStub{
		reconcileResult: controllerDeploymentReconcileState{ProcessedJobs: 1, ProcessedInstances: 2, CompletedJobs: 1},
	}
	access := &serviceAccessExecutorStub{mode: "dry-run"}
	wireGuard := &controllerWireGuardReconcilerStub{}

	if err := restoreControllerState(context.Background(), store, &runtimeExecutorStub{}, access, wireGuard, time.Now); err != nil {
		t.Fatalf("expected startup restore to succeed, got %v", err)
	}
	if wireGuard.calls != 0 {
		t.Fatalf("expected no wireguard reconcile call outside host mode, got %d", wireGuard.calls)
	}
}

func TestRestoreControllerStateFailsWhenWireGuardTruthUnknown(t *testing.T) {
	t.Setenv("CONTROLLER_RECONCILE_DEPLOYMENTS_ON_STARTUP", "true")
	t.Setenv("CONTROLLER_RECONCILE_ACCESS_ON_STARTUP", "true")

	store := &controllerStartupStoreStub{
		reconcileResult: controllerDeploymentReconcileState{ProcessedJobs: 1, ProcessedInstances: 2, CompletedJobs: 1},
	}
	wireGuard := &controllerWireGuardReconcilerStub{err: errors.New("wg sync failed")}

	err := restoreControllerState(context.Background(), store, &runtimeExecutorStub{}, &serviceAccessExecutorStub{}, wireGuard, time.Now)
	if err == nil || !strings.Contains(err.Error(), controllerWireGuardTruthUnknownDetail) {
		t.Fatalf("expected startup restore to fail on wireguard error, got %v", err)
	}
}
