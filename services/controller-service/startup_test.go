package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"adplatform/internal/services/apigateway"
)

type controllerStartupStoreStub struct {
	tasks              []apigateway.ControllerRuntimeTask
	policies           []apigateway.ControllerServiceAccessPolicy
	reconcileCalls     int
	listTaskCalls      int
	listPolicyCalls    int
	reconcileErr       error
	listTasksErr       error
	listPoliciesErr    error
}

func (s *controllerStartupStoreStub) ListControllerRuntimeTasks(context.Context) ([]apigateway.ControllerRuntimeTask, error) {
	s.listTaskCalls++
	return s.tasks, s.listTasksErr
}

func (s *controllerStartupStoreStub) ListControllerServiceAccessPolicies(context.Context) ([]apigateway.ControllerServiceAccessPolicy, error) {
	s.listPolicyCalls++
	return s.policies, s.listPoliciesErr
}

func (s *controllerStartupStoreStub) ReconcileDeployments(context.Context, time.Time) error {
	s.reconcileCalls++
	return s.reconcileErr
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
}

func (*serviceAccessExecutorStub) Status() apigateway.ControllerAccessStatus {
	return apigateway.ControllerAccessStatus{}
}

func (s *serviceAccessExecutorStub) Apply(_ context.Context, policies []apigateway.ControllerServiceAccessPolicy, now time.Time) (apigateway.ControllerAccessStatus, error) {
	s.policies = append([]apigateway.ControllerServiceAccessPolicy(nil), policies...)
	if s.err != nil {
		return apigateway.ControllerAccessStatus{State: "error", LastError: s.err.Error()}, s.err
	}
	return apigateway.ControllerAccessStatus{
		State:         "applied",
		Mode:          "host",
		PoliciesTotal: len(policies),
		AppliedAt:     now.UTC().Format(time.RFC3339),
	}, nil
}

func (s *serviceAccessExecutorStub) Teardown(_ context.Context) error {
	return nil
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
	now := func() time.Time { return time.Date(2026, time.March, 11, 9, 0, 0, 0, time.UTC) }

	if err := restoreControllerState(context.Background(), store, executor, access, now); err != nil {
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

	err := restoreControllerState(context.Background(), store, &runtimeExecutorStub{}, access, time.Now)
	if err == nil || err.Error() != "controller access restore failed: nft apply failed" {
		t.Fatalf("expected startup restore to fail on access error, got %v", err)
	}
}
