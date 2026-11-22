package main

import (
	"fmt"
	"net"
	"slices"
	"sort"
	"strings"

	"adplatform/internal/platform/config"
	"adplatform/internal/platform/gamenet"
	"adplatform/internal/services/apigateway"
)

const defaultGameNetworkSubnet = "10.80.0.0/16"

func gameIsolationTableName(accessTable string) string {
	return accessTable + "_isolation"
}

// gameNetworkCIDRs returns the configured game subnet plus the /24 of any
// service outside it. Service addresses are allocated independently of the
// setting, so a narrower value must not leave a running service unguarded.
func gameNetworkCIDRs(policies []apigateway.ControllerServiceAccessPolicy) ([]string, error) {
	raw := strings.TrimSpace(config.String("AD_PLATFORM_GAME_NETWORK_SUBNET", defaultGameNetworkSubnet))
	if raw == "" {
		raw = defaultGameNetworkSubnet
	}
	_, configured, err := net.ParseCIDR(raw)
	if err != nil || configured.IP.To4() == nil {
		return nil, fmt.Errorf("AD_PLATFORM_GAME_NETWORK_SUBNET must be an IPv4 CIDR, got %q", raw)
	}

	networks := []*net.IPNet{configured}
	mask := net.CIDRMask(24, 32)
	for _, policy := range policies {
		if ip := net.ParseIP(strings.TrimSpace(policy.ServiceIP)).To4(); ip != nil {
			networks = append(networks, &net.IPNet{IP: ip.Mask(mask), Mask: mask})
		}
	}
	// Broadest first, so a covered network is dropped. nftables rejects
	// overlapping elements in an interval set.
	sort.SliceStable(networks, func(i, j int) bool {
		left, _ := networks[i].Mask.Size()
		right, _ := networks[j].Mask.Size()
		return left < right
	})
	var kept []*net.IPNet
	for _, network := range networks {
		covered := slices.ContainsFunc(kept, func(parent *net.IPNet) bool { return parent.Contains(network.IP) })
		if !covered {
			kept = append(kept, network)
		}
	}
	cidrs := make([]string, 0, len(kept))
	for _, network := range kept {
		cidrs = append(cidrs, network.String())
	}
	return cidrs, nil
}

// renderGameIsolationRules is the default-deny boundary around game containers.
// It is applied with either access backend, because the routed rules cannot
// see traffic switched inside a Docker bridge unless br_netfilter is loaded.
//
//   - input: game networks cannot open connections to the host, which holds the
//     controller and WireGuard APIs on its Docker bridge address. The forward
//     chain applies the same limit to ports Docker publishes on the host.
//   - forward: egress-disabled services can reach only game networks and VPN
//     clients, so reverse shells to players work on every challenge.
//   - checker_source: checkers reach services from the player address range, so
//     a service cannot keep its SLA by trusting only the checker network.
//   - bridge: one service instance can open a connection to another only on an
//     open service port. Checkers are not service addresses, so warmup and SLA
//     checks are unaffected. The bridge hook sees every Docker network on the
//     host, so only traffic between known service addresses is filtered.
func renderGameIsolationRules(tableName, wireGuardInterface string, gameCIDRs []string, policies []apigateway.ControllerServiceAccessPolicy) string {
	var serviceIPs, egressDenied, openServices []string
	for _, policy := range policies {
		ip := net.ParseIP(strings.TrimSpace(policy.ServiceIP)).To4()
		if ip == nil {
			continue
		}
		address := ip.String()
		serviceIPs = append(serviceIPs, address)
		if !policy.EgressEnabled {
			egressDenied = append(egressDenied, address)
		}
		if !policy.NetworkClosed && policy.ServicePort > 0 {
			openServices = append(openServices, fmt.Sprintf("%s . %d", address, policy.ServicePort))
		}
	}

	var rules strings.Builder
	fmt.Fprintf(&rules, "add table inet %[1]s\ndelete table inet %[1]s\nadd table bridge %[1]s\ndelete table bridge %[1]s\n", tableName)

	fmt.Fprintf(&rules, "table inet %s {\n", tableName)
	writeNftSet(&rules, "game_networks", "ipv4_addr", "flags interval", gameCIDRs)
	writeNftSet(&rules, "egress_denied", "ipv4_addr", "", egressDenied)
	rules.WriteString("  chain input {\n    type filter hook input priority filter; policy accept;\n")
	rules.WriteString("    ct state established,related accept\n")
	rules.WriteString("    ip saddr @game_networks drop\n")
	fmt.Fprintf(&rules, "    ip saddr %s drop\n  }\n", gamenet.CheckerNetworkSubnet)
	// Checkers reach services with a player-range source address, keeping the
	// host part so each run has one stable address.
	rules.WriteString("  chain checker_source {\n    type nat hook postrouting priority srcnat; policy accept;\n")
	fmt.Fprintf(&rules, "    ip saddr %[1]s ip daddr @game_networks snat ip prefix to ip saddr map { %[1]s : %[2]s }\n  }\n", gamenet.CheckerNetworkSubnet, gamenet.PlayerNetworkSubnet)
	rules.WriteString("  chain forward {\n    type filter hook forward priority filter; policy accept;\n")
	rules.WriteString("    ct state established,related accept\n")
	// Docker DNATs published host ports before the input hook, so a connection
	// to any host address arrives here instead.
	rules.WriteString("    ip saddr @game_networks ct status dnat drop\n")
	fmt.Fprintf(&rules, "    ip saddr %s ip daddr != @game_networks drop\n", gamenet.CheckerNetworkSubnet)
	// The VPN is part of the game, so player callbacks survive disabled egress.
	if wireGuardInterface != "" {
		fmt.Fprintf(&rules, "    ip saddr @egress_denied oifname != %q ip daddr != @game_networks drop\n  }\n}\n", wireGuardInterface)
	} else {
		rules.WriteString("    ip saddr @egress_denied ip daddr != @game_networks drop\n  }\n}\n")
	}

	fmt.Fprintf(&rules, "table bridge %s {\n", tableName)
	writeNftSet(&rules, "service_addresses", "ipv4_addr", "", serviceIPs)
	writeNftSet(&rules, "open_services", "ipv4_addr . inet_service", "", openServices)
	rules.WriteString("  chain forward {\n    type filter hook forward priority filter; policy accept;\n")
	rules.WriteString("    ether type != ip accept\n")
	rules.WriteString("    ip saddr != @service_addresses accept\n")
	rules.WriteString("    ip daddr != @service_addresses accept\n")
	rules.WriteString("    ip protocol != tcp drop\n")
	// Only a bare SYN opens a connection. Stateless matching on the destination
	// keeps replies working without letting a service use its own port as a
	// source-port bypass.
	rules.WriteString("    tcp flags & (syn | ack) != syn accept\n")
	rules.WriteString("    ip daddr . tcp dport @open_services accept\n")
	rules.WriteString("    drop\n  }\n}\n")
	return rules.String()
}

func writeNftSet(rules *strings.Builder, name, elementType, flags string, elements []string) {
	fmt.Fprintf(rules, "  set %s {\n    type %s\n", name, elementType)
	if flags != "" {
		fmt.Fprintf(rules, "    %s\n", flags)
	}
	unique := slices.Clone(elements)
	slices.Sort(unique)
	unique = slices.Compact(unique)
	if len(unique) > 0 {
		fmt.Fprintf(rules, "    elements = { %s }\n", strings.Join(unique, ", "))
	}
	rules.WriteString("  }\n")
}
