package apigateway

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWireGuardServerEndpointDerivesFromPublicBaseURL(t *testing.T) {
	t.Setenv("WIREGUARD_SERVER_ENDPOINT", "vpn.adplatform.local:51820")
	t.Setenv("AD_PLATFORM_PUBLIC_BASE_URL", "http://10.70.0.1")
	t.Setenv("WIREGUARD_SERVER_LISTEN_PORT", "51820")

	if endpoint := wireGuardServerEndpoint(); endpoint != "10.70.0.1:51820" {
		t.Fatalf("expected derived wireguard endpoint, got %q", endpoint)
	}
}

func TestAdminPlayerWireGuardConfigRefreshesServerMetadata(t *testing.T) {
	t.Setenv("WIREGUARD_SERVER_ENDPOINT", "vpn.adplatform.local:51820")
	t.Setenv("AD_PLATFORM_PUBLIC_BASE_URL", "http://10.70.0.1")
	t.Setenv("WIREGUARD_SERVER_PUBLIC_KEY", "initial-server-public")

	mux := newTestMux()
	adminAuth := "Bearer dev-admin-token"

	createRequest := httptest.NewRequest(http.MethodPost, "/api/v2/admin/players", bytes.NewBufferString(`{"team_id":101,"display_name":"WireGuard Member","email":"wg.member@example.com","password":"wg-member-secret","role":"member"}`))
	createRequest.Header.Set("Authorization", adminAuth)
	createResponse := httptest.NewRecorder()
	mux.ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusOK {
		t.Fatalf("expected player create 200, got %d", createResponse.Code)
	}

	playerPayload := decodeCompat[adminPlayer](t, createResponse.Body.Bytes())

	getRequest := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v2/admin/players/%d/wireguard", playerPayload.ID), nil)
	getRequest.Header.Set("Authorization", adminAuth)
	getResponse := httptest.NewRecorder()
	mux.ServeHTTP(getResponse, getRequest)
	if getResponse.Code != http.StatusOK {
		t.Fatalf("expected wireguard get 200, got %d", getResponse.Code)
	}

	initialPayload := decodeCompat[adminWireGuardPeer](t, getResponse.Body.Bytes())
	if initialPayload.ServerEndpoint != "10.70.0.1:51820" {
		t.Fatalf("expected derived endpoint, got %q", initialPayload.ServerEndpoint)
	}
	if initialPayload.ServerPublicKey != "initial-server-public" {
		t.Fatalf("expected initial server public key, got %q", initialPayload.ServerPublicKey)
	}
	if !bytes.Contains([]byte(initialPayload.Config), []byte("Endpoint = 10.70.0.1:51820")) {
		t.Fatalf("expected derived endpoint in config, got %q", initialPayload.Config)
	}

	t.Setenv("WIREGUARD_SERVER_ENDPOINT", "192.168.23.30:51820")
	t.Setenv("WIREGUARD_SERVER_PUBLIC_KEY", "rotated-server-public")

	refreshRequest := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v2/admin/players/%d/wireguard", playerPayload.ID), nil)
	refreshRequest.Header.Set("Authorization", adminAuth)
	refreshResponse := httptest.NewRecorder()
	mux.ServeHTTP(refreshResponse, refreshRequest)
	if refreshResponse.Code != http.StatusOK {
		t.Fatalf("expected refreshed wireguard get 200, got %d", refreshResponse.Code)
	}

	refreshedPayload := decodeCompat[adminWireGuardPeer](t, refreshResponse.Body.Bytes())
	if refreshedPayload.ServerEndpoint != "192.168.23.30:51820" {
		t.Fatalf("expected refreshed endpoint, got %q", refreshedPayload.ServerEndpoint)
	}
	if refreshedPayload.ServerPublicKey != "rotated-server-public" {
		t.Fatalf("expected refreshed server public key, got %q", refreshedPayload.ServerPublicKey)
	}
	if refreshedPayload.ClientPublicKey != initialPayload.ClientPublicKey {
		t.Fatalf("expected client public key to be preserved, got %q then %q", initialPayload.ClientPublicKey, refreshedPayload.ClientPublicKey)
	}
	if refreshedPayload.Config == initialPayload.Config {
		t.Fatal("expected config to be refreshed when server metadata changes")
	}
	if !bytes.Contains([]byte(refreshedPayload.Config), []byte("Endpoint = 192.168.23.30:51820")) {
		t.Fatalf("expected refreshed endpoint in config, got %q", refreshedPayload.Config)
	}
	if !bytes.Contains([]byte(refreshedPayload.Config), []byte("PublicKey = rotated-server-public")) {
		t.Fatalf("expected refreshed public key in config, got %q", refreshedPayload.Config)
	}
}
