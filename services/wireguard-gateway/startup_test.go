package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"adplatform/internal/services/apigateway"
)

type wireGuardStartupStoreStub struct {
	peers    []apigateway.WireGuardGatewayPeer
	err      error
	pauseErr error
}

func (s wireGuardStartupStoreStub) ListWireGuardGatewayPeers(context.Context) ([]apigateway.WireGuardGatewayPeer, error) {
	return s.peers, s.err
}

func (s wireGuardStartupStoreStub) IsMatchPaused(context.Context) (bool, error) {
	return false, s.pauseErr
}

func TestRestoreWireGuardStateFailsClosedWhenPauseStateUnavailable(t *testing.T) {
	t.Setenv("WIREGUARD_GATEWAY_RECONCILE_ON_STARTUP", "true")
	applier := &wireGuardApplierStub{}
	server := newWireGuardGatewayServer("admin-token", nil, applier)

	err := restoreWireGuardState(context.Background(), wireGuardStartupStoreStub{pauseErr: errors.New("database unavailable")}, server)
	if err != nil {
		t.Fatalf("expected fail-closed restore to apply, got %v", err)
	}
	if !applier.snapshot.MatchPaused {
		t.Fatal("expected unavailable pause state to produce a paused snapshot")
	}
}

func (s wireGuardStartupStoreStub) IsMatchStarted(context.Context) (bool, error) {
	return true, nil
}

type wireGuardApplierStub struct {
	status   apigateway.WireGuardGatewayStatus
	err      error
	snapshot wireGuardGatewaySnapshot
}

func (s *wireGuardApplierStub) Status() apigateway.WireGuardGatewayStatus {
	return apigateway.WireGuardGatewayStatus{State: "idle", Mode: "host"}
}

func (s *wireGuardApplierStub) Apply(_ context.Context, snapshot wireGuardGatewaySnapshot) (apigateway.WireGuardGatewayStatus, error) {
	s.snapshot = snapshot
	if s.err != nil {
		return apigateway.WireGuardGatewayStatus{State: "error", Mode: "host", LastError: s.err.Error()}, s.err
	}
	if s.status.Mode == "" {
		s.status = apigateway.WireGuardGatewayStatus{State: "applied", Mode: "host", PeersTotal: snapshot.PeersTotal, PeersActive: snapshot.PeersActive}
	}
	return s.status, nil
}

func (s *wireGuardApplierStub) Teardown(_ context.Context) error {
	return nil
}

func TestRestoreWireGuardStateAppliesSnapshotOnStartup(t *testing.T) {
	t.Setenv("WIREGUARD_GATEWAY_RECONCILE_ON_STARTUP", "true")

	applier := &wireGuardApplierStub{}
	server := newWireGuardGatewayServer("admin-token", nil, applier)
	server.now = func() time.Time { return time.Date(2026, time.March, 11, 9, 15, 0, 0, time.UTC) }

	err := restoreWireGuardState(context.Background(), wireGuardStartupStoreStub{
		peers: []apigateway.WireGuardGatewayPeer{
			{WireGuardPeer: "team-101-player-1", Address: "10.70.11.20", Status: "active", ClientPublicKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="},
			{WireGuardPeer: "team-102-player-2", Address: "10.70.12.21", Status: "revoked", ClientPublicKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="},
		},
	}, server)
	if err != nil {
		t.Fatalf("expected startup restore to succeed, got %v", err)
	}
	if applier.snapshot.PeersTotal != 2 || applier.snapshot.PeersActive != 1 {
		t.Fatalf("unexpected startup snapshot %+v", applier.snapshot)
	}
	if server.status().Mode != "host" || server.status().State != "applied" {
		t.Fatalf("unexpected remembered status %+v", server.status())
	}
}

func TestRestoreWireGuardStateFailsFastOnApplyError(t *testing.T) {
	t.Setenv("WIREGUARD_GATEWAY_RECONCILE_ON_STARTUP", "true")

	applier := &wireGuardApplierStub{err: errors.New("wg syncconf failed")}
	server := newWireGuardGatewayServer("admin-token", nil, applier)

	err := restoreWireGuardState(context.Background(), wireGuardStartupStoreStub{}, server)
	if err == nil || err.Error() != "wireguard startup restore failed: wg syncconf failed" {
		t.Fatalf("expected startup restore failure, got %v", err)
	}
	if server.status().State != "error" {
		t.Fatalf("expected server to remember error status, got %+v", server.status())
	}
}
