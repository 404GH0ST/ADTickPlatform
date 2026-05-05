package main

import (
	"context"
	"log"
	"time"

	"adplatform/internal/platform/config"
)

func restoreControllerState(ctx context.Context, store controllerReconcileStore, executor runtimeExecutor, access serviceAccessExecutor, wireGuard controllerWireGuardReconciler, now func() time.Time) error {
	includeRuntime := config.Bool("CONTROLLER_RECONCILE_DEPLOYMENTS_ON_STARTUP", true)
	includeAccess := config.Bool("CONTROLLER_RECONCILE_ACCESS_ON_STARTUP", true)
	if !includeRuntime && !includeAccess {
		return nil
	}

	result, err := newControllerTrustedReconciler(store, executor, access, wireGuard, now).ReconcileWithOptions(ctx, controllerTrustedReconcileOptions{
		includeRuntime:   includeRuntime,
		includeAccess:    includeAccess,
		includeWireGuard: includeAccess,
	})
	if err != nil {
		return err
	}
	log.Printf(
		"controller startup restore reconciled %d deployment job(s), %d instance(s), and %d completed job(s)",
		result.ProcessedJobs,
		result.ProcessedInstances,
		result.CompletedJobs,
	)
	return nil
}
