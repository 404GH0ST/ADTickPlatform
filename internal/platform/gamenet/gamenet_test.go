package gamenet

import "testing"

func TestPlanDockerNetworkShared(t *testing.T) {
	plan, err := PlanDockerNetwork("adplatform_prod_game", "shared", "10.80.50.11")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.Name != "adplatform_prod_game" || plan.Subnet != "" || plan.Layout != LayoutShared {
		t.Fatalf("unexpected shared plan %+v", plan)
	}
}

func TestPlanDockerNetworkPerService(t *testing.T) {
	plan, err := PlanDockerNetwork("adplatform_prod_game", "per-service", "10.80.50.11")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.Name != "adplatform_prod_game_svc_050" {
		t.Fatalf("unexpected plan name %q", plan.Name)
	}
	if plan.Subnet != "10.80.50.0/24" || plan.Layout != LayoutPerService {
		t.Fatalf("unexpected per-service plan %+v", plan)
	}
}

func TestPlanDockerNetworkRejectsInvalidIP(t *testing.T) {
	if _, err := PlanDockerNetwork("adplatform_prod_game", "per-service", "bad"); err == nil {
		t.Fatal("expected invalid ip error")
	}
}
