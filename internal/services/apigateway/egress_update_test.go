package apigateway

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDeployedChallengeEgressChangeRequiresMaintenance(t *testing.T) {
	store := NewMemoryStore(101).(*memoryStore)
	ctx := context.Background()
	disabled := false
	update := adminUpdateChallengeRequest{Name: "banking", BaselineImage: "registry.local/banking:baseline", CheckerImage: "registry.local/banking-checker:latest", EgressEnabled: &disabled}
	if _, err := store.UpdateAdminChallenge(ctx, 1, update); !errors.Is(err, ErrInvalidRuntimeConfig) {
		t.Fatalf("live egress change accepted: %v", err)
	}
	if task, err := store.GetControllerRuntimeTask(ctx, 101, 1); err != nil || task.DisableExternalDNS {
		t.Fatalf("rejected change altered the active policy: %+v %v", task, err)
	}
	if _, err := store.SetChallengeMaintenance(ctx, 1, true, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpdateAdminChallenge(ctx, 1, update); err != nil {
		t.Fatalf("maintenance change failed: %v", err)
	}
	if task, err := store.GetControllerRuntimeTask(ctx, 101, 1); err != nil || !task.DisableExternalDNS {
		t.Fatalf("controller did not receive the restricted DNS policy: %+v %v", task, err)
	}
}

func TestServiceContainerNamesDistinguishEqualSlugs(t *testing.T) {
	first := serviceContainerName("alpha beta", 1, 101)
	second := serviceContainerName("alpha-beta", 2, 101)
	if first == second {
		t.Fatalf("different challenges share a Docker container name: %q", first)
	}
}
