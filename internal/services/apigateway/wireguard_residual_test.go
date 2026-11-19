package apigateway

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"
)

func clearWireGuardEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"WIREGUARD_SERVER_ENDPOINT", "WIREGUARD_SERVER_PUBLIC_KEY", "WIREGUARD_SERVER_PRIVATE_KEY",
		"WIREGUARD_SERVER_ALLOWED_IPS", "WIREGUARD_SERVER_DNS", "WIREGUARD_SERVER_LISTEN_PORT",
		"API_GATEWAY_STATE_BACKEND", "WIREGUARD_GATEWAY_STATE_BACKEND", "WIREGUARD_GATEWAY_MODE",
		"AD_PLATFORM_PUBLIC_BASE_URL", "EDGE_SITE_ADDRESS",
	} {
		t.Setenv(key, "")
	}
}

func TestWireGuardServerKeyConfiguration(t *testing.T) {
	clearWireGuardEnv(t)
	devPrivate, devPublic := wireGuardDevKeypair()
	if key, err := wireGuardServerPublicKey(); err != nil || key != devPublic {
		t.Fatalf("dry-run memory mode uses the dev key: %q %v", key, err)
	}

	t.Setenv("WIREGUARD_SERVER_PRIVATE_KEY", devPrivate)
	if key, err := wireGuardServerPublicKey(); err != nil || key != devPublic {
		t.Fatalf("a private key yields its public key: %q %v", key, err)
	}
	for _, bad := range []string{"not base64!", base64.StdEncoding.EncodeToString([]byte("short"))} {
		t.Setenv("WIREGUARD_SERVER_PRIVATE_KEY", bad)
		if _, err := wireGuardServerPublicKey(); err == nil || !strings.Contains(err.Error(), "WIREGUARD_SERVER_PRIVATE_KEY is invalid") {
			t.Errorf("%q must be rejected: %v", bad, err)
		}
	}

	// Outside dry-run memory mode a key is mandatory, and every builder fails.
	t.Setenv("WIREGUARD_SERVER_PRIVATE_KEY", "")
	t.Setenv("API_GATEWAY_STATE_BACKEND", "postgres")
	if _, err := wireGuardServerPublicKey(); err == nil {
		t.Fatal("postgres mode without a server key must fail")
	}
	if _, err := newWireGuardPeerState(1, 101, "T", "P", "peer", "10.70.1.20", time.Now()); err == nil {
		t.Error("a peer cannot be issued without a server key")
	}
	if _, _, err := refreshWireGuardPeerState(wireGuardPeerState{}); err == nil {
		t.Error("a peer cannot be refreshed without a server key")
	}

	func() {
		defer func() {
			if recover() == nil {
				t.Error("a wrong-length seed is a programming error")
			}
		}()
		mustX25519Key([]byte("short"))
	}()
	private, public := generateX25519Keypair()
	if derived, err := x25519PublicKeyFromBase64(private); err != nil || derived != public {
		t.Fatalf("a generated keypair must be consistent: %v", err)
	}
	if generateWireGuardSecret() == generateWireGuardSecret() {
		t.Fatal("preshared keys must be random")
	}
}

func TestWireGuardPeerStateLifecycle(t *testing.T) {
	clearWireGuardEnv(t)
	if _, err := newWireGuardPeerState(1, 101, "T", "P", "peer", " ", time.Now()); !errors.Is(err, ErrPlayerAddressPool) {
		t.Fatalf("no address means the pool is exhausted: %v", err)
	}
	state, err := newWireGuardPeerState(1, 101, "Team", "Player", "peer-1", "10.70.1.20", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, changed, err := refreshWireGuardPeerState(state); err != nil || changed {
		t.Fatalf("an up-to-date peer is unchanged: %v %v", changed, err)
	}
	t.Setenv("WIREGUARD_SERVER_ALLOWED_IPS", "10.99.0.0/16")
	t.Setenv("WIREGUARD_SERVER_DNS", "9.9.9.9")
	updated, changed, err := refreshWireGuardPeerState(state)
	if err != nil || !changed || updated.AllowedIPs != "10.99.0.0/16" || updated.DNS != "9.9.9.9" || !strings.Contains(updated.Config, "DNS = 9.9.9.9") {
		t.Fatalf("new routes and DNS must be applied: %+v %v %v", updated, changed, err)
	}
}

func TestWireGuardNamesAndEndpoints(t *testing.T) {
	clearWireGuardEnv(t)
	for _, tc := range []struct{ user, fallback, want string }{
		{"alice@example.test", "x", "alice.conf"},
		{"!!!", "Team Blue", "team-blue.conf"},
		{"", "", "wireguard.conf"},
		{"a..b  c", "", "a.b-c.conf"},
	} {
		if got := wireGuardDownloadName(tc.user, tc.fallback); got != tc.want {
			t.Errorf("wireGuardDownloadName(%q, %q) = %q, want %q", tc.user, tc.fallback, got, tc.want)
		}
	}

	for raw, want := range map[string]string{
		"":                        "",
		"udp://vpn.ctf.test":      "vpn.ctf.test:51820",
		"udp://vpn.ctf.test:4000": "vpn.ctf.test:4000",
		"udp://":                  "udp://",
		"vpn.ctf.test:4000":       "vpn.ctf.test:4000",
		"[vpn.ctf.test:4000":      "[vpn.ctf.test:4000",
		"vpn.ctf.test":            "vpn.ctf.test:51820",
		"2001:db8::1":             "[2001:db8::1]:51820",
	} {
		if got := normalizeWireGuardEndpoint(raw); got != want {
			t.Errorf("normalizeWireGuardEndpoint(%q) = %q, want %q", raw, got, want)
		}
	}

	if got := wireGuardServerEndpoint(); got != "vpn.adplatform.local:51820" {
		t.Errorf("default endpoint: %q", got)
	}
	t.Setenv("WIREGUARD_SERVER_ENDPOINT", "vpn.example.com")
	if got := wireGuardServerEndpoint(); got != "vpn.example.com:51820" {
		t.Errorf("an example endpoint with nothing to derive from is kept: %q", got)
	}
	t.Setenv("AD_PLATFORM_PUBLIC_BASE_URL", "http://[broken")
	t.Setenv("EDGE_SITE_ADDRESS", "ctf.example.org")
	if got := wireGuardServerEndpoint(); got != "ctf.example.org:51820" {
		t.Errorf("an example endpoint yields to the edge address: %q", got)
	}
	t.Setenv("WIREGUARD_SERVER_ENDPOINT", "wg.ctf.test:7000")
	if got := wireGuardServerEndpoint(); got != "wg.ctf.test:7000" {
		t.Errorf("an explicit endpoint wins: %q", got)
	}
}
