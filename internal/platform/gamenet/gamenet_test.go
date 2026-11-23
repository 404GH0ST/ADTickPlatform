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

func TestSharedNetworkNamesAreCanonical(t *testing.T) {
	for _, tt := range []struct{ name, want string }{
		{"  Game--Net  ", "game_net"},
		{"", "adplatform_game"},
		{"---", "adplatform_game"},
	} {
		plan, err := PlanDockerNetwork(tt.name, " SHARED ", "")
		if err != nil || plan.Name != tt.want || plan.Layout != LayoutShared {
			t.Errorf("PlanDockerNetwork(%q) = %+v, %v; want %q", tt.name, plan, err, tt.want)
		}
	}
}

func TestCheckerContainerIP(t *testing.T) {
	if got, err := CheckerContainerIP(" 10.70.3.44 "); err != nil || got != "10.79.3.44" {
		t.Fatalf("got %q, %v", got, err)
	}
	for _, invalid := range []string{"", "bad", "10.80.1.2", "10.70.0.1", "10.70.1.0", "10.70.1.255", "fd00::1"} {
		if _, err := CheckerContainerIP(invalid); err == nil {
			t.Fatalf("%q must be rejected", invalid)
		}
	}
	if got := CheckerNetworkName("adplatform_prod_game"); got != "adplatform_prod_game_checker" {
		t.Fatalf("network name: %q", got)
	}
}
