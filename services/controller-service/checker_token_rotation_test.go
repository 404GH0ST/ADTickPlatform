package main

import (
	"testing"

	"adplatform/internal/services/apigateway"
)

// A redeploy rotates the database checker token, but a guarded container bakes
// the token into its environment at creation. The deploy warm-reconcile path
// must recreate such a container, not keep it, or the in-container gate rejects
// every checker put.
func TestCheckerTokenCurrentForcesRecreateOnRotation(t *testing.T) {
	info := captureInspect{}
	info.Config.Env = []string{"PATH=/usr/bin", "AD_CHECKER_TOKEN=old-token", "PORT=31337"}

	if got := capturedCheckerToken(info); got != "old-token" {
		t.Fatalf("capturedCheckerToken = %q, want old-token", got)
	}
	if !checkerTokenCurrent(info, apigateway.ControllerRuntimeTask{CheckerToken: "old-token"}) {
		t.Fatal("a matching token must be treated as current (warm reconcile kept)")
	}
	if checkerTokenCurrent(info, apigateway.ControllerRuntimeTask{CheckerToken: "new-token"}) {
		t.Fatal("a rotated token must not be current (container must be recreated)")
	}
	// No managed token for this task: nothing to compare, keep the container.
	if !checkerTokenCurrent(info, apigateway.ControllerRuntimeTask{CheckerToken: ""}) {
		t.Fatal("an empty task token must be treated as current")
	}
	// Container without the variable, task expects one: recreate.
	if checkerTokenCurrent(captureInspect{}, apigateway.ControllerRuntimeTask{CheckerToken: "new-token"}) {
		t.Fatal("a missing baked token must not match a managed task token")
	}
}
