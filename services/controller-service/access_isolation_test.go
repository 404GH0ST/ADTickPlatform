package main

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"adplatform/internal/services/apigateway"
)

func TestGameNetworkCIDRs(t *testing.T) {
	policies := []apigateway.ControllerServiceAccessPolicy{
		{ServiceIP: "10.80.1.101"}, {ServiceIP: "10.80.1.102"}, {ServiceIP: "10.81.7.9"}, {ServiceIP: "invalid"},
	}
	for name, tc := range map[string]struct {
		subnet string
		want   []string
	}{
		"covered services add nothing": {"10.80.0.0/16", []string{"10.80.0.0/16", "10.81.7.0/24"}},
		"empty uses the default":       {"", []string{"10.80.0.0/16", "10.81.7.0/24"}},
		"narrow subnet is widened":     {"10.80.1.0/25", []string{"10.80.1.0/24", "10.81.7.0/24"}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("AD_PLATFORM_GAME_NETWORK_SUBNET", tc.subnet)
			got, err := gameNetworkCIDRs(policies)
			if err != nil || !slices.Equal(got, tc.want) {
				t.Fatalf("got %v, %v; want %v", got, err, tc.want)
			}
		})
	}
	for _, invalid := range []string{"not-a-cidr", "fd00::/64"} {
		t.Setenv("AD_PLATFORM_GAME_NETWORK_SUBNET", invalid)
		if _, err := gameNetworkCIDRs(policies); err == nil {
			t.Fatalf("%q must be rejected", invalid)
		}
	}
}

func TestRenderGameIsolationRules(t *testing.T) {
	rules := renderGameIsolationRules("access_isolation", "wg0", []string{"10.80.0.0/16"}, []apigateway.ControllerServiceAccessPolicy{
		{ServiceIP: "10.80.1.101", ServicePort: 8080},
		{ServiceIP: "10.80.1.102", ServicePort: 8080, EgressEnabled: true},
		{ServiceIP: "10.80.1.103", ServicePort: 8080, NetworkClosed: true},
		{ServiceIP: "invalid", ServicePort: 8080},
	})
	for _, want := range []string{
		"add table inet access_isolation\ndelete table inet access_isolation\nadd table bridge access_isolation\ndelete table bridge access_isolation\n",
		"elements = { 10.80.0.0/16 }",
		"elements = { 10.80.1.101, 10.80.1.103 }",
		"elements = { 10.80.1.101, 10.80.1.102, 10.80.1.103 }",
		"elements = { 10.80.1.101 . 8080, 10.80.1.102 . 8080 }",
		"ip saddr @game_networks ct status dnat drop",
		"ip saddr 10.79.0.0/16 drop",
		"ip saddr 10.79.0.0/16 ip daddr != @game_networks drop",
		"type nat hook postrouting priority srcnat; policy accept;",
		"ip saddr 10.79.0.0/16 ip daddr @game_networks snat ip prefix to ip saddr map { 10.79.0.0/16 : 10.70.0.0/16 }",
		`ip saddr @egress_denied oifname != "wg0" ip daddr != @game_networks drop`,
	} {
		if !strings.Contains(rules, want) {
			t.Fatalf("missing %q:\n%s", want, rules)
		}
	}
	if strings.Contains(rules, "invalid") {
		t.Fatalf("invalid addresses must be omitted:\n%s", rules)
	}
	input := rules[strings.Index(rules, "chain input"):strings.Index(rules, "chain forward")]
	if strings.Index(input, "established,related accept") > strings.Index(input, "ip saddr @game_networks drop") {
		t.Fatalf("host replies must be accepted before the game-network drop:\n%s", input)
	}
	bridge := rules[strings.Index(rules, "table bridge access_isolation {"):]
	order := []string{"ether type != ip accept", "ip saddr != @service_addresses accept", "ip daddr != @service_addresses accept", "ip protocol != tcp drop", "tcp flags & (syn | ack) != syn accept", "ip daddr . tcp dport @open_services accept", "    drop\n"}
	last := -1
	for _, rule := range order {
		index := strings.Index(bridge, rule)
		if index <= last {
			t.Fatalf("bridge rule %q is out of order:\n%s", rule, bridge)
		}
		last = index
	}

	withoutVPN := renderGameIsolationRules("access_isolation", "", []string{"10.80.0.0/16"}, nil)
	if strings.Contains(withoutVPN, "oifname") || !strings.Contains(withoutVPN, "ip saddr @egress_denied ip daddr != @game_networks drop") || strings.Contains(withoutVPN, "elements = { 10.80.1") {
		t.Fatalf("no VPN interface or services must render no related rules:\n%s", withoutVPN)
	}
}

func TestHostAccessAppliesIsolationFirstWithEveryBackend(t *testing.T) {
	for _, backend := range []string{"nftables", "iptables"} {
		runner := &faultRunner{}
		e := hostAccess(t, runner, backend, "wg0", "eth0")
		if _, err := e.Apply(context.Background(), accessPolicies(), time.Now()); err != nil {
			t.Fatalf("%s: %v", backend, err)
		}
		if want := "nft -f " + e.paths.rulesPath + ".isolation"; runner.calls[0] != want {
			t.Fatalf("%s: first command %q, want %q", backend, runner.calls[0], want)
		}
		body, err := os.ReadFile(e.paths.rulesPath + ".isolation")
		if err != nil || !strings.Contains(string(body), "table bridge adp_access_isolation {") {
			t.Fatalf("%s: isolation rules: %v\n%s", backend, err, body)
		}
		if err := e.Teardown(context.Background()); err != nil {
			t.Fatal(err)
		}
		joined := strings.Join(runner.calls, "\n")
		for _, want := range []string{"nft delete table inet adp_access_isolation", "nft delete table bridge adp_access_isolation"} {
			if !strings.Contains(joined, want) {
				t.Fatalf("%s: teardown missing %q", backend, want)
			}
		}
		if _, err := os.Stat(e.paths.rulesPath + ".isolation"); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s: teardown must remove the isolation artifact: %v", backend, err)
		}
	}
}

func TestHostAccessLetsCheckersReachServicePortsOnly(t *testing.T) {
	runner := &recordingControllerCommandRunner{}
	if _, err := hostAccess(t, runner, "iptables", "wg0", "eth0").Apply(context.Background(), []apigateway.ControllerServiceAccessPolicy{
		{ServiceIP: "10.80.1.101", ServicePort: 8080, SSHPort: 22, NetworkClosed: true},
	}, time.Now()); err != nil {
		t.Fatal(err)
	}
	accept := slices.Index(runner.rules, "-A ADPLATFORM-WG-SERVICES -i adpchk0 -d 10.80.1.101/32 -p tcp --dport 8080 -j ACCEPT")
	closed := slices.Index(runner.rules, "-A ADPLATFORM-WG-SERVICES -i wg0 -d 10.80.1.101/32 -j DROP")
	if accept < 0 || closed < accept {
		t.Fatalf("warmup checkers must reach closed service ports: %v", runner.rules)
	}
	if !slices.Contains(runner.rules, "-A ADPLATFORM-WG-RAW -i adpchk0 -d 10.80.1.101/32 -j ACCEPT") {
		t.Fatalf("Docker's raw guard must allow the checker bridge: %v", runner.rules)
	}
	for _, rule := range runner.rules {
		if strings.Contains(rule, "adpchk0") && strings.Contains(rule, "--dport 22") {
			t.Fatalf("checkers must not get SSH access: %s", rule)
		}
	}
}

func TestHostAccessStopsWhenIsolationCannotApply(t *testing.T) {
	runner := &faultRunner{reject: func(call string) bool { return strings.HasSuffix(call, ".isolation") }}
	status, err := hostAccess(t, runner, "iptables", "wg0", "eth0").Apply(context.Background(), accessPolicies(), time.Now())
	if err == nil || status.State != "error" || len(runner.calls) != 1 {
		t.Fatalf("apply must stop at the failed boundary: %+v %v %v", status, err, runner.calls)
	}

	t.Setenv("AD_PLATFORM_GAME_NETWORK_SUBNET", "invalid")
	runner = &faultRunner{}
	status, err = hostAccess(t, runner, "iptables", "wg0", "eth0").Apply(context.Background(), accessPolicies(), time.Now())
	if err == nil || status.State != "error" || len(runner.calls) != 0 {
		t.Fatalf("an invalid game subnet must fail before any firewall change: %+v %v %v", status, err, runner.calls)
	}
}
