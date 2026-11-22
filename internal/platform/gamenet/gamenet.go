package gamenet

import (
	"fmt"
	"net"
	"strings"
)

const (
	LayoutShared     = "shared"
	LayoutPerService = "per-service"

	// Checkers run on their own routed network and leave it with a source
	// address from the player VPN range. Matching the VPN MTU gives them the
	// same TCP MSS as players, and the routed hop gives them the same TTL.
	PlayerNetworkSubnet   = "10.70.0.0/16"
	CheckerNetworkSubnet  = "10.79.0.0/16"
	CheckerNetworkGateway = "10.79.0.1"
	CheckerBridgeName     = "adpchk0"
	CheckerNetworkMTU     = 1420
)

// CheckerNetworkName returns the Docker network for player-like checker runs.
func CheckerNetworkName(baseName string) string {
	return sanitizeDockerName(baseName) + "_checker"
}

// CheckerContainerIP maps a player-range source address to the checker network
// address that the host translates back to it.
func CheckerContainerIP(sourceIP string) (string, error) {
	ip := net.ParseIP(strings.TrimSpace(sourceIP)).To4()
	if ip == nil || ip[0] != 10 || ip[1] != 70 || ip[2] == 0 || ip[3] == 0 || ip[3] == 255 {
		return "", fmt.Errorf("invalid checker source ip: %q", sourceIP)
	}
	return fmt.Sprintf("10.79.%d.%d", ip[2], ip[3]), nil
}

type DockerNetworkPlan struct {
	Name   string
	Subnet string
	Layout string
}

func NormalizeLayout(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case LayoutShared:
		return LayoutShared
	default:
		return LayoutPerService
	}
}

func PlanDockerNetwork(baseName, layout, serviceIP string) (DockerNetworkPlan, error) {
	layout = NormalizeLayout(layout)
	baseName = sanitizeDockerName(baseName)
	if layout == LayoutShared {
		return DockerNetworkPlan{Name: baseName, Layout: layout}, nil
	}

	ip := net.ParseIP(strings.TrimSpace(serviceIP)).To4()
	if ip == nil {
		return DockerNetworkPlan{}, fmt.Errorf("invalid service ip for per-service network plan: %q", serviceIP)
	}

	return DockerNetworkPlan{
		Name:   fmt.Sprintf("%s_svc_%03d", baseName, int(ip[2])),
		Subnet: fmt.Sprintf("%d.%d.%d.0/24", int(ip[0]), int(ip[1]), int(ip[2])),
		Layout: layout,
	}, nil
}

func sanitizeDockerName(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "adplatform_game"
	}
	var builder strings.Builder
	lastSep := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			lastSep = false
			continue
		}
		if !lastSep {
			builder.WriteRune('_')
			lastSep = true
		}
	}
	result := strings.Trim(builder.String(), "_")
	if result == "" {
		return "adplatform_game"
	}
	return result
}
