package main

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"adplatform/internal/platform/httpapi"
)

// An open SSE stream must not hold graceful shutdown past its deadline: the
// stream ends as soon as shutdown begins.
func TestGracefulShutdownEndsOpenStreams(t *testing.T) {
	stopping := make(chan struct{})
	gateway := newRealtimeGateway(&testSnapshotClient{}, time.Hour, "dev-admin-token").withShutdown(stopping)
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "realtime-gateway", Version: "test", Addr: ":0"})
	gateway.RegisterRoutes(mux)
	server := httptest.NewUnstartedServer(mux)
	server.Start()
	defer server.Close()

	response, err := http.Get(server.URL + "/public/v1/scoreboard/stream")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("stream status %d", response.StatusCode)
	}
	streamEnded := make(chan struct{})
	go func() {
		defer close(streamEnded)
		reader := bufio.NewReader(response.Body)
		for {
			if _, err := reader.ReadString('\n'); err != nil {
				return
			}
		}
	}()

	close(stopping)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := server.Config.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown must not wait out the open stream: %v", err)
	}
	select {
	case <-streamEnded:
	case <-time.After(2 * time.Second):
		t.Fatal("the client's stream must end")
	}
}
