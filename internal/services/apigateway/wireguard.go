package apigateway

import (
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"adplatform/internal/platform/config"
)

type wireGuardPeerState struct {
	PlayerID         int
	TeamID           int
	TeamName         string
	DisplayName      string
	WireGuardPeer    string
	Address          string
	Status           string
	ServerEndpoint   string
	ServerPublicKey  string
	ClientPrivateKey string
	ClientPublicKey  string
	PresharedKey     string
	AllowedIPs       string
	DNS              string
	Config           string
	IssuedAt         string
	RevokedAt        string
}

type wireGuardServerProfile struct {
	Endpoint   string
	PublicKey  string
	AllowedIPs string
	DNS        string
}

func newWireGuardPeerState(playerID, teamID int, teamName, displayName, wireGuardPeer, address string, now time.Time) (wireGuardPeerState, error) {
	if strings.TrimSpace(address) == "" {
		return wireGuardPeerState{}, ErrPlayerAddressPool
	}
	clientPrivateKey, clientPublicKey, err := generateX25519Keypair()
	if err != nil {
		return wireGuardPeerState{}, err
	}

	presharedKey, err := generateWireGuardSecret()
	if err != nil {
		return wireGuardPeerState{}, err
	}

	serverPublicKey, err := wireGuardServerPublicKey()
	if err != nil {
		return wireGuardPeerState{}, err
	}

	state := wireGuardPeerState{
		PlayerID:         playerID,
		TeamID:           teamID,
		TeamName:         teamName,
		DisplayName:      displayName,
		WireGuardPeer:    wireGuardPeer,
		Address:          address,
		Status:           "active",
		ServerEndpoint:   wireGuardServerEndpoint(),
		ServerPublicKey:  serverPublicKey,
		ClientPrivateKey: clientPrivateKey,
		ClientPublicKey:  clientPublicKey,
		PresharedKey:     presharedKey,
		AllowedIPs:       wireGuardAllowedIPs(),
		DNS:              wireGuardDNS(),
		IssuedAt:         now.UTC().Format(time.RFC3339),
	}
	state.Config = renderWireGuardConfig(state)
	return state, nil
}

func currentWireGuardServerProfile() (wireGuardServerProfile, error) {
	serverPublicKey, err := wireGuardServerPublicKey()
	if err != nil {
		return wireGuardServerProfile{}, err
	}
	return wireGuardServerProfile{
		Endpoint:   wireGuardServerEndpoint(),
		PublicKey:  serverPublicKey,
		AllowedIPs: wireGuardAllowedIPs(),
		DNS:        wireGuardDNS(),
	}, nil
}

func refreshWireGuardPeerState(state wireGuardPeerState) (wireGuardPeerState, bool, error) {
	profile, err := currentWireGuardServerProfile()
	if err != nil {
		return wireGuardPeerState{}, false, err
	}
	updated := state
	changed := false

	if updated.ServerEndpoint != profile.Endpoint {
		updated.ServerEndpoint = profile.Endpoint
		changed = true
	}
	if updated.ServerPublicKey != profile.PublicKey {
		updated.ServerPublicKey = profile.PublicKey
		changed = true
	}
	if updated.AllowedIPs != profile.AllowedIPs {
		updated.AllowedIPs = profile.AllowedIPs
		changed = true
	}
	if updated.DNS != profile.DNS {
		updated.DNS = profile.DNS
		changed = true
	}

	renderedConfig := renderWireGuardConfig(updated)
	if updated.Config != renderedConfig {
		updated.Config = renderedConfig
		changed = true
	}
	return updated, changed, nil
}

func renderWireGuardConfig(state wireGuardPeerState) string {
	return fmt.Sprintf(
		"# AD Platform WireGuard peer: %s\n# Player: %s (%s)\n[Interface]\nPrivateKey = %s\nAddress = %s/32\nDNS = %s\n\n[Peer]\nPublicKey = %s\nPresharedKey = %s\nAllowedIPs = %s\nEndpoint = %s\nPersistentKeepalive = 25\n",
		state.WireGuardPeer,
		state.DisplayName,
		state.TeamName,
		state.ClientPrivateKey,
		state.Address,
		state.DNS,
		state.ServerPublicKey,
		state.PresharedKey,
		state.AllowedIPs,
		state.ServerEndpoint,
	)
}

func wireGuardDownloadName(usernameSource, fallback string) string {
	username := strings.TrimSpace(usernameSource)
	if beforeAt, _, found := strings.Cut(username, "@"); found {
		username = beforeAt
	}
	stem := sanitizeWireGuardDownloadStem(username)
	if stem == "" {
		stem = sanitizeWireGuardDownloadStem(fallback)
	}
	if stem == "" {
		stem = "wireguard"
	}
	return fmt.Sprintf("%s.conf", stem)
}

func sanitizeWireGuardDownloadStem(value string) string {
	var builder strings.Builder
	previousSeparator := false
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
			previousSeparator = false
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
			previousSeparator = false
		case r == '-' || r == '_' || r == '.':
			if builder.Len() > 0 && !previousSeparator {
				builder.WriteRune(r)
				previousSeparator = true
			}
		default:
			if builder.Len() > 0 && !previousSeparator {
				builder.WriteRune('-')
				previousSeparator = true
			}
		}
	}
	return strings.Trim(builder.String(), "-_.")
}

func wireGuardPeerAddress(_ int, playerID int) string {
	return wireGuardPeerAddressAt(max(playerID-1, 0))
}

func wireGuardPeerAddressAt(index int) string {
	return fmt.Sprintf("10.70.%d.%d", (index/200)+1, (index%200)+20)
}

func wireGuardServerEndpoint() string {
	explicitEndpoint := strings.TrimSpace(config.String("WIREGUARD_SERVER_ENDPOINT", ""))
	if explicitEndpoint != "" && !isExampleWireGuardEndpoint(explicitEndpoint) {
		return normalizeWireGuardEndpoint(explicitEndpoint)
	}

	if derivedEndpoint := deriveWireGuardServerEndpoint(); derivedEndpoint != "" {
		return derivedEndpoint
	}

	if explicitEndpoint != "" {
		return normalizeWireGuardEndpoint(explicitEndpoint)
	}
	return net.JoinHostPort("vpn.adplatform.local", wireGuardListenPort())
}

func normalizeWireGuardEndpoint(endpoint string) string {
	trimmed := strings.TrimSpace(endpoint)
	if trimmed == "" {
		return ""
	}

	if strings.Contains(trimmed, "://") {
		parsed, err := url.Parse(trimmed)
		if err == nil && parsed.Hostname() != "" {
			port := parsed.Port()
			if port == "" {
				port = wireGuardListenPort()
			}
			return net.JoinHostPort(parsed.Hostname(), port)
		}
		return trimmed
	}

	if host, port, err := net.SplitHostPort(trimmed); err == nil && strings.TrimSpace(host) != "" && strings.TrimSpace(port) != "" {
		return net.JoinHostPort(host, port)
	}

	if strings.Count(trimmed, ":") == 1 {
		host, port, found := strings.Cut(trimmed, ":")
		if found && strings.TrimSpace(host) != "" && strings.TrimSpace(port) != "" {
			return net.JoinHostPort(strings.TrimSpace(host), strings.TrimSpace(port))
		}
	}

	return net.JoinHostPort(trimmed, wireGuardListenPort())
}

func wireGuardAllowedIPs() string {
	return strings.TrimSpace(config.String("WIREGUARD_SERVER_ALLOWED_IPS", "10.70.0.0/16,10.80.0.0/16"))
}

func wireGuardDNS() string {
	return strings.TrimSpace(config.String("WIREGUARD_SERVER_DNS", "1.1.1.1"))
}

func wireGuardListenPort() string {
	return strings.TrimSpace(config.String("WIREGUARD_SERVER_LISTEN_PORT", "51820"))
}

func wireGuardServerPublicKey() (string, error) {
	if publicKey := strings.TrimSpace(config.String("WIREGUARD_SERVER_PUBLIC_KEY", "")); publicKey != "" {
		return publicKey, nil
	}
	if privateKey := strings.TrimSpace(config.String("WIREGUARD_SERVER_PRIVATE_KEY", "")); privateKey != "" {
		publicKey, err := x25519PublicKeyFromBase64(privateKey)
		if err == nil {
			return publicKey, nil
		}
		return "", fmt.Errorf("WIREGUARD_SERVER_PRIVATE_KEY is invalid: %w", err)
	}
	if !allowDevWireGuardServerKeyFallback() {
		return "", fmt.Errorf("WIREGUARD_SERVER_PRIVATE_KEY or WIREGUARD_SERVER_PUBLIC_KEY must be set outside dry-run memory mode")
	}
	_, publicKey := wireGuardDevKeypair()
	return publicKey, nil
}

func allowDevWireGuardServerKeyFallback() bool {
	apiBackend := strings.ToLower(strings.TrimSpace(config.String("API_GATEWAY_STATE_BACKEND", "memory")))
	wgBackend := strings.ToLower(strings.TrimSpace(config.String("WIREGUARD_GATEWAY_STATE_BACKEND", apiBackend)))
	mode := strings.ToLower(strings.TrimSpace(config.String("WIREGUARD_GATEWAY_MODE", "dry-run")))
	return apiBackend != "postgres" && wgBackend != "postgres" && mode == "dry-run"
}

func wireGuardDevKeypair() (string, string) {
	curve := ecdh.X25519()
	seed := sha256.Sum256([]byte("adplatform-dev-wireguard-server"))
	privateKey, err := curve.NewPrivateKey(seed[:])
	if err != nil {
		encodedSeed := base64.StdEncoding.EncodeToString(seed[:])
		return encodedSeed, encodedSeed
	}
	return base64.StdEncoding.EncodeToString(privateKey.Bytes()), base64.StdEncoding.EncodeToString(privateKey.PublicKey().Bytes())
}

func generateX25519Keypair() (string, string, error) {
	curve := ecdh.X25519()
	privateKey, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", err
	}
	return base64.StdEncoding.EncodeToString(privateKey.Bytes()), base64.StdEncoding.EncodeToString(privateKey.PublicKey().Bytes()), nil
}

func generateWireGuardSecret() (string, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(secret), nil
}

func x25519PublicKeyFromBase64(privateKey string) (string, error) {
	decodedKey, err := base64.StdEncoding.DecodeString(strings.TrimSpace(privateKey))
	if err != nil {
		return "", err
	}
	key, err := ecdh.X25519().NewPrivateKey(decodedKey)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key.PublicKey().Bytes()), nil
}

func deriveWireGuardServerEndpoint() string {
	for _, source := range []string{
		strings.TrimSpace(config.String("AD_PLATFORM_PUBLIC_BASE_URL", "")),
		strings.TrimSpace(config.String("EDGE_SITE_ADDRESS", "")),
	} {
		host := endpointHost(source)
		if host == "" {
			continue
		}
		return net.JoinHostPort(host, wireGuardListenPort())
	}
	return ""
}

func endpointHost(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if !strings.Contains(trimmed, "://") {
		trimmed = "http://" + trimmed
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}

func isExampleWireGuardEndpoint(endpoint string) bool {
	host := endpointHost(endpoint)
	return host == "vpn.adplatform.local" || host == "vpn.example.com"
}

func wireGuardAdminView(state wireGuardPeerState, email string) adminWireGuardPeer {
	return adminWireGuardPeer{
		PlayerID:        state.PlayerID,
		TeamID:          state.TeamID,
		TeamName:        state.TeamName,
		DisplayName:     state.DisplayName,
		Email:           email,
		WireGuardPeer:   state.WireGuardPeer,
		Address:         state.Address,
		Status:          state.Status,
		ServerEndpoint:  state.ServerEndpoint,
		ServerPublicKey: state.ServerPublicKey,
		ClientPublicKey: state.ClientPublicKey,
		AllowedIPs:      state.AllowedIPs,
		DNS:             state.DNS,
		Config:          state.Config,
		DownloadName:    wireGuardDownloadName(email, state.WireGuardPeer),
		IssuedAt:        state.IssuedAt,
		RevokedAt:       state.RevokedAt,
	}
}
