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
	IsMatchPaused(context.Context) (bool, error)
}

func restoreWireGuardState(ctx context.Context, store wireGuardStartupStore, server *wireGuardGatewayServer) error {
	if !config.Bool("WIREGUARD_GATEWAY_RECONCILE_ON_STARTUP", true) {
		return nil
	}
	peers, err := store.ListWireGuardGatewayPeers(ctx)
	if err != nil {
		return err
	}
	paused, err := store.IsMatchPaused(ctx)
	if err != nil {
		log.Printf("warning: could not determine match pause state on startup, assuming paused: %v", err)
		paused = true
	}
	snapshot, err := buildWireGuardGatewaySnapshot(peers, server.now())
	if err != nil {
		return err
	}
	snapshot.MatchPaused = paused
	status, err := server.applier.Apply(ctx, snapshot)
	if err != nil {
		server.rememberStatus(status)
		return fmt.Errorf("wireguard startup restore failed: %w", err)
	}
	server.rememberStatus(status)
	log.Printf("wireguard startup restore applied %d active peer(s) in %s mode (paused: %v)", status.PeersActive, status.Mode, paused)
	return nil
}
