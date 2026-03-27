package main

import (
	"context"
	"fmt"
	"log"

	"adplatform/internal/platform/config"
	"adplatform/internal/services/apigateway"
)

type wireGuardStartupStore interface {
	ListWireGuardGatewayPeers(context.Context) ([]apigateway.WireGuardGatewayPeer, error)
}

func restoreWireGuardState(ctx context.Context, store wireGuardStartupStore, server *wireGuardGatewayServer) error {
	if !config.Bool("WIREGUARD_GATEWAY_RECONCILE_ON_STARTUP", true) {
		return nil
	}
	peers, err := store.ListWireGuardGatewayPeers(ctx)
	if err != nil {
		return err
	}
	snapshot := buildWireGuardGatewaySnapshot(peers, server.now())
	status, err := server.applier.Apply(ctx, snapshot)
	if err != nil {
		server.rememberStatus(status)
		return fmt.Errorf("wireguard startup restore failed: %w", err)
	}
	server.rememberStatus(status)
	log.Printf("wireguard startup restore applied %d active peer(s) in %s mode", status.PeersActive, status.Mode)
	return nil
}
