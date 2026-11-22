package main

import (
	"context"
	"log"
	"strings"
	"sync"

	"adplatform/internal/services/apigateway"
)

type playerAddressLister interface {
	ListPlayerVPNAddresses(ctx context.Context) ([]string, error)
}

// playerSourceChecker gives each checker run a random unused player address, so
// a service cannot keep its SLA by trusting only the checker's network. If no
// address is available the run still happens directly on the service network:
// a platform fault must never cost a team SLA.
type playerSourceChecker struct {
	inner     checkerClient
	addresses playerAddressLister

	mu    sync.Mutex
	inUse map[string]struct{}
}

func withPlayerCheckerSource(client checkerClient, store gameStore, mode string) checkerClient {
	if !strings.EqualFold(strings.TrimSpace(mode), "player") {
		return client
	}
	addresses, ok := store.(playerAddressLister)
	if !ok {
		log.Printf("game-core: player checker source needs the postgres state backend; using the service network")
		return client
	}
	return &playerSourceChecker{inner: client, addresses: addresses, inUse: make(map[string]struct{})}
}

func (c *playerSourceChecker) Execute(ctx context.Context, request apigateway.CheckerExecutionRequest) (apigateway.CheckerExecutionResult, error) {
	source, release := c.acquire(ctx)
	defer release()
	request.SourceIP = source
	return c.inner.Execute(ctx, request)
}

func (c *playerSourceChecker) ExecuteBatch(ctx context.Context, request apigateway.CheckerBatchExecutionRequest) (apigateway.CheckerBatchExecutionResult, error) {
	source, release := c.acquire(ctx)
	defer release()
	request.SourceIP = source
	return c.inner.ExecuteBatch(ctx, request)
}

func (c *playerSourceChecker) acquire(ctx context.Context) (string, func()) {
	assigned, err := c.addresses.ListPlayerVPNAddresses(ctx)
	if err != nil {
		log.Printf("game-core: checker source lookup failed; using the service network: %v", err)
		return "", func() {}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	used := make(map[string]struct{}, len(assigned)+len(c.inUse))
	for _, address := range assigned {
		used[strings.TrimSpace(address)] = struct{}{}
	}
	for address := range c.inUse {
		used[address] = struct{}{}
	}
	address, ok := apigateway.RandomUnassignedPlayerAddress(used)
	if !ok {
		log.Printf("game-core: no free checker source address; using the service network")
		return "", func() {}
	}
	c.inUse[address] = struct{}{}
	return address, func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		delete(c.inUse, address)
	}
}

func (s *postgresGameStore) ListPlayerVPNAddresses(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT address FROM wireguard_peers`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	addresses := make([]string, 0)
	for rows.Next() {
		var address string
		if err := rows.Scan(&address); err != nil {
			return nil, err
		}
		addresses = append(addresses, address)
	}
	return addresses, rows.Err()
}
