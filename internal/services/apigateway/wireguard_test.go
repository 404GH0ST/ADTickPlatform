package apigateway

import (
	"bytes"
	"encoding/json"
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

	var playerPayload struct {
		Status string      `json:"status"`
		Data   adminPlayer `json:"data"`
	}
	if err := json.Unmarshal(createResponse.Body.Bytes(), &playerPayload); err != nil {
		t.Fatalf("failed to decode player response: %v", err)
	}

	getRequest := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v2/admin/players/%d/wireguard", playerPayload.Data.ID), nil)
	getRequest.Header.Set("Authorization", adminAuth)
	getResponse := httptest.NewRecorder()
	mux.ServeHTTP(getResponse, getRequest)
	if getResponse.Code != http.StatusOK {
		t.Fatalf("expected wireguard get 200, got %d", getResponse.Code)
	}

	var initialPayload struct {
		Status string             `json:"status"`
		Data   adminWireGuardPeer `json:"data"`
	}
	if err := json.Unmarshal(getResponse.Body.Bytes(), &initialPayload); err != nil {
		t.Fatalf("failed to decode wireguard response: %v", err)
	}
	if initialPayload.Data.ServerEndpoint != "10.70.0.1:51820" {
		t.Fatalf("expected derived endpoint, got %q", initialPayload.Data.ServerEndpoint)
	}
	if initialPayload.Data.ServerPublicKey != "initial-server-public" {
		t.Fatalf("expected initial server public key, got %q", initialPayload.Data.ServerPublicKey)
	}
	if !bytes.Contains([]byte(initialPayload.Data.Config), []byte("Endpoint = 10.70.0.1:51820")) {
		t.Fatalf("expected derived endpoint in config, got %q", initialPayload.Data.Config)
	}

	t.Setenv("WIREGUARD_SERVER_ENDPOINT", "192.168.23.30:51820")
	t.Setenv("WIREGUARD_SERVER_PUBLIC_KEY", "rotated-server-public")

	refreshRequest := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v2/admin/players/%d/wireguard", playerPayload.Data.ID), nil)
	refreshRequest.Header.Set("Authorization", adminAuth)
	refreshResponse := httptest.NewRecorder()
	mux.ServeHTTP(refreshResponse, refreshRequest)
	if refreshResponse.Code != http.StatusOK {
		t.Fatalf("expected refreshed wireguard get 200, got %d", refreshResponse.Code)
	}

	var refreshedPayload struct {
		Status string             `json:"status"`
		Data   adminWireGuardPeer `json:"data"`
	}
	if err := json.Unmarshal(refreshResponse.Body.Bytes(), &refreshedPayload); err != nil {
		t.Fatalf("failed to decode refreshed wireguard response: %v", err)
	}
	if refreshedPayload.Data.ServerEndpoint != "192.168.23.30:51820" {
		t.Fatalf("expected refreshed endpoint, got %q", refreshedPayload.Data.ServerEndpoint)
	}
	if refreshedPayload.Data.ServerPublicKey != "rotated-server-public" {
		t.Fatalf("expected refreshed server public key, got %q", refreshedPayload.Data.ServerPublicKey)
	}
	if refreshedPayload.Data.ClientPublicKey != initialPayload.Data.ClientPublicKey {
		t.Fatalf("expected client public key to be preserved, got %q then %q", initialPayload.Data.ClientPublicKey, refreshedPayload.Data.ClientPublicKey)
	}
	if refreshedPayload.Data.Config == initialPayload.Data.Config {
		t.Fatal("expected config to be refreshed when server metadata changes")
	}
	if !bytes.Contains([]byte(refreshedPayload.Data.Config), []byte("Endpoint = 192.168.23.30:51820")) {
		t.Fatalf("expected refreshed endpoint in config, got %q", refreshedPayload.Data.Config)
	}
	if !bytes.Contains([]byte(refreshedPayload.Data.Config), []byte("PublicKey = rotated-server-public")) {
		t.Fatalf("expected refreshed public key in config, got %q", refreshedPayload.Data.Config)
	}
}
