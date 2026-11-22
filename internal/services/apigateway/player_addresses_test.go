package apigateway

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestRandomUnassignedPlayerAddressUsesWholePool(t *testing.T) {
	blocks := map[string]struct{}{}
	for range 200 {
		address, ok := RandomUnassignedPlayerAddress(map[string]struct{}{"10.70.1.20": {}})
		if !ok || address == "10.70.1.20" {
			t.Fatalf("unexpected address %q %v", address, ok)
		}
		var block, host int
		if _, err := fmt.Sscanf(address, "10.70.%d.%d", &block, &host); err != nil || block < 1 || block > 254 || host < 20 || host > 219 {
			t.Fatalf("address outside the player pool: %q", address)
		}
		blocks[fmt.Sprint(block)] = struct{}{}
	}
	if len(blocks) < 50 {
		t.Fatalf("addresses must spread across the pool, got %d blocks", len(blocks))
	}
}

func TestRandomUnassignedPlayerAddressHandlesNearlyFullPool(t *testing.T) {
	used := make(map[string]struct{}, wireGuardPeerAddressPoolSize)
	for index := range wireGuardPeerAddressPoolSize {
		used[wireGuardPeerAddressAt(index)] = struct{}{}
	}
	if address, ok := RandomUnassignedPlayerAddress(used); ok {
		t.Fatalf("a full pool returned %q", address)
	}
	last := wireGuardPeerAddressAt(wireGuardPeerAddressPoolSize - 1)
	delete(used, last)
	if address, ok := RandomUnassignedPlayerAddress(used); !ok || address != last {
		t.Fatalf("must find the only free address: %q %v", address, ok)
	}
}

func TestStoreContractPlayerAddressesAreRandomAndRotate(t *testing.T) {
	forEachStore(t, func(t *testing.T, s contractStore) {
		ctx := context.Background()
		now := time.Now()
		w := buildWorld(t, s)
		player, err := s.CreateAdminPlayer(ctx, adminCreatePlayerRequest{TeamID: w.teamB.ID, DisplayName: "Random Peer", Email: "random-peer@example.test", Password: "random-peer-password", Role: "member"}, now)
		if err != nil {
			t.Fatal(err)
		}
		seen := map[string]struct{}{player.WireGuardAddress: {}}
		for range 3 {
			peer, err := s.RotateAdminPlayerWireGuardConfig(ctx, player.ID, now)
			if err != nil {
				t.Fatal(err)
			}
			if _, repeated := seen[peer.Address]; repeated {
				t.Fatalf("rotation must assign a new address, repeated %q", peer.Address)
			}
			if peer.Config == "" || !strings.Contains(peer.Config, "Address = "+peer.Address+"/32") {
				t.Fatalf("config must use the rotated address %q", peer.Address)
			}
			seen[peer.Address] = struct{}{}
		}
	})
}
