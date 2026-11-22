package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"adplatform/internal/platform/gamenet"
)

// resolveRunNetwork selects the player-like checker network. Any failure falls
// back to the service network so a platform fault cannot cost a team SLA.
func (e *dockerCheckerExecutor) resolveRunNetwork(ctx context.Context, serviceNetwork, sourceIP string) (string, string) {
	if strings.TrimSpace(sourceIP) == "" {
		return serviceNetwork, ""
	}
	if _, err := gamenet.CheckerContainerIP(sourceIP); err != nil {
		log.Printf("checker source rejected; using the service network: %v", err)
		return serviceNetwork, ""
	}
	name := gamenet.CheckerNetworkName(e.network)
	if err := e.ensureCheckerNetwork(ctx, name); err != nil {
		log.Printf("checker network unavailable; using the service network: %v", err)
		return serviceNetwork, ""
	}
	return name, sourceIP
}

func (e *dockerCheckerExecutor) ensureCheckerNetwork(ctx context.Context, name string) error {
	e.checkerNetworkMu.Lock()
	defer e.checkerNetworkMu.Unlock()
	if _, _, err := e.execDocker(ctx, "network", "inspect", name); err == nil {
		return nil
	}
	_, _, err := e.execDocker(ctx,
		"network", "create",
		"--driver", "bridge",
		"--subnet", gamenet.CheckerNetworkSubnet,
		"--gateway", gamenet.CheckerNetworkGateway,
		"--label", "adplatform.game_network=true",
		"--label", "adplatform.checker_network=true",
		"-o", "com.docker.network.bridge.name="+gamenet.CheckerBridgeName,
		"-o", "com.docker.network.bridge.enable_ip_masquerade=false",
		"-o", fmt.Sprintf("com.docker.network.driver.mtu=%d", gamenet.CheckerNetworkMTU),
		name,
	)
	return err
}

func appendCheckerNetworkArgs(args []string, network, sourceIP string) []string {
	if strings.TrimSpace(network) == "" {
		return args
	}
	args = append(args, "--network", network)
	if containerIP, err := gamenet.CheckerContainerIP(sourceIP); err == nil {
		args = append(args, "--ip", containerIP)
	}
	return args
}

// retryOnServiceNetwork reports a Docker failure on the player-like path, such
// as an address still held by a container that could not be removed.
func retryOnServiceNetwork(ctx context.Context, exitCode int, sourceIP string) bool {
	return exitCode == 125 && sourceIP != "" && ctx.Err() == nil
}
