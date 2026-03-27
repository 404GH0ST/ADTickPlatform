package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"adplatform/internal/platform/config"
	"adplatform/internal/services/apigateway"
)

type controllerStartupStore interface {
	ListControllerRuntimeTasks(context.Context) ([]apigateway.ControllerRuntimeTask, error)
	ListControllerServiceAccessPolicies(context.Context) ([]apigateway.ControllerServiceAccessPolicy, error)
	ReconcileDeployments(context.Context, time.Time) error
}

type controllerStartupStoreAdapter struct {
	store apigateway.Store
}

func (a controllerStartupStoreAdapter) ListControllerRuntimeTasks(ctx context.Context) ([]apigateway.ControllerRuntimeTask, error) {
	return a.store.ListControllerRuntimeTasks(ctx)
}

func (a controllerStartupStoreAdapter) ListControllerServiceAccessPolicies(ctx context.Context) ([]apigateway.ControllerServiceAccessPolicy, error) {
	return a.store.ListControllerServiceAccessPolicies(ctx)
}

func (a controllerStartupStoreAdapter) ReconcileDeployments(ctx context.Context, now time.Time) error {
	_, err := a.store.ReconcileAdminDeployments(ctx, now)
	return err
}

func restoreControllerState(ctx context.Context, store controllerStartupStore, executor runtimeExecutor, access serviceAccessExecutor, now func() time.Time) error {
	if config.Bool("CONTROLLER_RECONCILE_DEPLOYMENTS_ON_STARTUP", true) {
		tasks, err := store.ListControllerRuntimeTasks(ctx)
		if err != nil {
			return err
		}
		for _, task := range tasks {
			if err := executor.EnsureService(ctx, task); err != nil {
				return fmt.Errorf("controller deployment restore failed for team %d challenge %d: %w", task.TeamID, task.ChallengeID, err)
			}
		}
		if err := store.ReconcileDeployments(ctx, now()); err != nil {
			return fmt.Errorf("controller deployment state reconcile failed: %w", err)
		}
		log.Printf("controller startup restore applied %d queued runtime task(s)", len(tasks))
	}

	if config.Bool("CONTROLLER_RECONCILE_ACCESS_ON_STARTUP", true) {
		policies, err := store.ListControllerServiceAccessPolicies(ctx)
		if err != nil {
			return err
		}
		status, err := access.Apply(ctx, policies, now())
		if err != nil {
			return fmt.Errorf("controller access restore failed: %w", err)
		}
		log.Printf("controller startup restore applied %d service access policie(s) in %s mode", status.PoliciesTotal, status.Mode)
	}

	return nil
}
