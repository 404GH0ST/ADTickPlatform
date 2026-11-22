package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"adplatform/internal/services/apigateway"
)

// playerPathDocker records each invocation and fails player-network runs when asked.
func playerPathDocker(t *testing.T, script string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls")
	binary := filepath.Join(dir, "docker")
	body := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> '" + logPath + "'\n" + script + "\n"
	if err := os.WriteFile(binary, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	return binary, logPath
}

func dockerCalls(t *testing.T, logPath string) []string {
	t.Helper()
	body, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(body)), "\n")
}

func playerSourceRequest() apigateway.CheckerExecutionRequest {
	return apigateway.CheckerExecutionRequest{ChallengeID: 1, TeamID: 101, CheckerImage: "checker", Phase: "check", Target: "10.80.1.101:8080", TargetIP: "10.80.1.101", TargetPort: 8080, TickID: 2, SourceIP: "10.70.1.44"}
}

func TestCheckerUsesPlayerNetworkAndCreatesItOnce(t *testing.T) {
	binary, logPath := playerPathDocker(t, `case "$1 $2" in "network inspect") [ -f "$0.ready" ] || exit 1;; "network create") : > "$0.ready";; esac; exit 0`)
	e := &dockerCheckerExecutor{binary: binary, network: "adplatform_prod_game", networkLayout: "per-service", timeout: 5 * time.Second}
	for range 2 {
		if result, err := e.ExecuteChecker(context.Background(), playerSourceRequest()); err != nil || result.Status != "success" {
			t.Fatalf("%+v %v", result, err)
		}
	}
	calls := dockerCalls(t, logPath)
	creates := 0
	runs := 0
	for _, call := range calls {
		if strings.HasPrefix(call, "network create") {
			creates++
			for _, want := range []string{"--subnet 10.79.0.0/16", "com.docker.network.bridge.name=adpchk0", "enable_ip_masquerade=false", "driver.mtu=1420", "adplatform_prod_game_checker"} {
				if !strings.Contains(call, want) {
					t.Fatalf("network create missing %q: %s", want, call)
				}
			}
		}
		if strings.HasPrefix(call, "run ") {
			runs++
			if !strings.Contains(call, "--network adplatform_prod_game_checker --ip 10.79.1.44") {
				t.Fatalf("checker must run on the player-like path: %s", call)
			}
		}
	}
	if creates != 1 || runs != 2 {
		t.Fatalf("creates=%d runs=%d: %v", creates, runs, calls)
	}
}

func TestCheckerFallsBackWhenPlayerPathUnavailable(t *testing.T) {
	for name, script := range map[string]string{
		"network cannot be created":  `case "$1" in network) exit 1;; esac; exit 0`,
		"docker rejects the address": `case "$*" in *--ip*) exit 125;; esac; exit 0`,
	} {
		t.Run(name, func(t *testing.T) {
			binary, logPath := playerPathDocker(t, script)
			e := &dockerCheckerExecutor{binary: binary, network: "adplatform_prod_game", networkLayout: "per-service", timeout: 5 * time.Second}
			if result, err := e.ExecuteChecker(context.Background(), playerSourceRequest()); err != nil || result.Status != "success" {
				t.Fatalf("a platform fault must not fail the service: %+v %v", result, err)
			}
			batch := apigateway.CheckerBatchExecutionRequest{ChallengeID: 1, TeamID: 101, CheckerImage: "checker", Phases: []string{"check"}, Target: "10.80.1.101:8080", TargetIP: "10.80.1.101", TargetPort: 8080, TickID: 2, SourceIP: "10.70.1.44"}
			if _, err := e.ExecuteCheckerBatch(context.Background(), batch); err != nil {
				t.Fatal(err)
			}
			calls := dockerCalls(t, logPath)
			runs := slices.DeleteFunc(slices.Clone(calls), func(call string) bool { return !strings.HasPrefix(call, "run ") })
			if len(runs) == 0 || !strings.Contains(runs[len(runs)-1], "--network adplatform_prod_game_svc_001") || strings.Contains(runs[len(runs)-1], "--ip") {
				t.Fatalf("expected a direct service-network run: %v", runs)
			}
		})
	}
}

func TestCheckerWithoutSourceStaysOnServiceNetwork(t *testing.T) {
	binary, logPath := playerPathDocker(t, `exit 0`)
	e := &dockerCheckerExecutor{binary: binary, network: "adplatform_prod_game", networkLayout: "per-service", timeout: 5 * time.Second}
	request := playerSourceRequest()
	request.SourceIP = ""
	if _, err := e.ExecuteChecker(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	calls := dockerCalls(t, logPath)
	if !strings.HasPrefix(calls[0], "run ") || !strings.Contains(calls[0], "--network adplatform_prod_game_svc_001") || strings.Contains(calls[0], "--ip") {
		t.Fatalf("unexpected run: %s", calls[0])
	}
	for _, call := range calls {
		if strings.HasPrefix(call, "network ") {
			t.Fatalf("a direct run must not touch the checker network: %s", call)
		}
	}
}
