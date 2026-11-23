package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"adplatform/internal/services/apigateway"
)

type fakePlayerAddresses struct {
	addresses []string
	err       error
}

func (f fakePlayerAddresses) ListPlayerVPNAddresses(context.Context) ([]string, error) {
	return f.addresses, f.err
}

type recordingSourceClient struct {
	mu      sync.Mutex
	sources []string
	hold    chan struct{}
	entered chan struct{}
}

func (c *recordingSourceClient) record(source string) {
	c.mu.Lock()
	c.sources = append(c.sources, source)
	c.mu.Unlock()
	if c.hold != nil {
		c.entered <- struct{}{}
		<-c.hold
	}
}

func (c *recordingSourceClient) Execute(_ context.Context, request apigateway.CheckerExecutionRequest) (apigateway.CheckerExecutionResult, error) {
	c.record(request.SourceIP)
	return apigateway.CheckerExecutionResult{Status: "success"}, nil
}

func (c *recordingSourceClient) ExecuteBatch(_ context.Context, request apigateway.CheckerBatchExecutionRequest) (apigateway.CheckerBatchExecutionResult, error) {
	c.record(request.SourceIP)
	return apigateway.CheckerBatchExecutionResult{}, nil
}

func TestPlayerSourceCheckerUsesUnassignedPlayerAddresses(t *testing.T) {
	inner := &recordingSourceClient{}
	client := &playerSourceChecker{inner: inner, addresses: fakePlayerAddresses{addresses: []string{"10.70.1.20", "10.70.1.21"}}, inUse: map[string]struct{}{}}
	for range 50 {
		if _, err := client.Execute(context.Background(), apigateway.CheckerExecutionRequest{}); err != nil {
			t.Fatal(err)
		}
		if _, err := client.ExecuteBatch(context.Background(), apigateway.CheckerBatchExecutionRequest{}); err != nil {
			t.Fatal(err)
		}
	}
	distinct := map[string]struct{}{}
	for _, source := range inner.sources {
		if !strings.HasPrefix(source, "10.70.") || source == "10.70.1.20" || source == "10.70.1.21" {
			t.Fatalf("checker source %q must be a free player-pool address", source)
		}
		distinct[source] = struct{}{}
	}
	if len(distinct) < 2 {
		t.Fatalf("checker sources must rotate, got %v", distinct)
	}
	if len(client.inUse) != 0 {
		t.Fatalf("finished runs must release their address: %v", client.inUse)
	}
}

func TestPlayerSourceCheckerNeverSharesAnAddress(t *testing.T) {
	// Leave exactly two free addresses so a shared address would be detected.
	assigned := make([]string, 0, 50798)
	for block := 1; block <= 254; block++ {
		for host := 20; host <= 219; host++ {
			if block == 7 && (host == 33 || host == 34) {
				continue
			}
			assigned = append(assigned, fmt.Sprintf("10.70.%d.%d", block, host))
		}
	}
	inner := &recordingSourceClient{hold: make(chan struct{}), entered: make(chan struct{}, 2)}
	client := &playerSourceChecker{inner: inner, addresses: fakePlayerAddresses{addresses: assigned}, inUse: map[string]struct{}{}}
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = client.Execute(context.Background(), apigateway.CheckerExecutionRequest{})
		}()
	}
	<-inner.entered
	<-inner.entered
	close(inner.hold)
	wg.Wait()
	if len(inner.sources) != 2 || inner.sources[0] == inner.sources[1] || inner.sources[0] == "" || inner.sources[1] == "" {
		t.Fatalf("concurrent runs need distinct sources: %v", inner.sources)
	}
}

func TestPlayerSourceCheckerFallsBackWithoutAddress(t *testing.T) {
	inner := &recordingSourceClient{}
	client := &playerSourceChecker{inner: inner, addresses: fakePlayerAddresses{err: errors.New("database unavailable")}, inUse: map[string]struct{}{}}
	if _, err := client.Execute(context.Background(), apigateway.CheckerExecutionRequest{}); err != nil {
		t.Fatal(err)
	}
	if len(inner.sources) != 1 || inner.sources[0] != "" {
		t.Fatalf("a lookup failure must still run the checker directly: %v", inner.sources)
	}
}

func TestWithPlayerCheckerSourceRequiresPlayerMode(t *testing.T) {
	inner := &recordingSourceClient{}
	if got := withPlayerCheckerSource(inner, newMemoryGameStore(), "service"); got != checkerClient(inner) {
		t.Fatal("service mode must leave the checker client unchanged")
	}
	if got := withPlayerCheckerSource(inner, newMemoryGameStore(), "player"); got != checkerClient(inner) {
		t.Fatal("a store without player addresses must leave the checker client unchanged")
	}
	if _, ok := withPlayerCheckerSource(inner, &postgresGameStore{}, " PLAYER ").(*playerSourceChecker); !ok {
		t.Fatal("player mode must wrap a postgres-backed checker client")
	}
}
