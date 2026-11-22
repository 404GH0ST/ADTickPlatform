package apigateway

import (
	"slices"
	"testing"
)

func TestUnassignedPlayerAddresses(t *testing.T) {
	free := UnassignedPlayerAddresses([]string{"10.70.1.20", " 10.70.1.21 "})
	if len(free) != 198 || slices.Contains(free, "10.70.1.20") || slices.Contains(free, "10.70.1.21") || !slices.Contains(free, "10.70.1.219") || slices.Contains(free, "10.70.2.20") {
		t.Fatalf("free addresses must stay in the players' block: %d %v", len(free), free[:2])
	}
	if free := UnassignedPlayerAddresses([]string{"10.70.2.20"}); len(free) != 399 || !slices.Contains(free, "10.70.1.20") {
		t.Fatalf("every block holding players is eligible: %d", len(free))
	}
	full := UnassignedPlayerAddresses(nil)
	if free := UnassignedPlayerAddresses(full); len(free) != 200 || free[0] != "10.70.2.20" {
		t.Fatalf("a full block must open the next one: %d", len(free))
	}
}
