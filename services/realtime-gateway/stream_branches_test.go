package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"adplatform/internal/services/apigateway"
)

var errSnapshotDown = errors.New("snapshot source down")

type failingSnapshotClient struct{}

func (failingSnapshotClient) Scoreboard(context.Context) ([]apigateway.ScoreRowAlias, error) {
	return nil, errSnapshotDown
}
func (failingSnapshotClient) ScoreboardLive(context.Context) ([]apigateway.ScoreRowAlias, error) {
	return nil, errSnapshotDown
}
func (failingSnapshotClient) Attacks(context.Context) (apigateway.AttackFeedPage, error) {
	return apigateway.AttackFeedPage{}, errSnapshotDown
}
func (failingSnapshotClient) GameStatus(context.Context) (apigateway.GameStatus, error) {
	return apigateway.GameStatus{}, errSnapshotDown
}
func (failingSnapshotClient) SchedulerEvents(context.Context, int) (apigateway.GameSchedulerEventPage, error) {
	return apigateway.GameSchedulerEventPage{}, errSnapshotDown
}
func (failingSnapshotClient) CheckerRuns(context.Context, int) (apigateway.GameCheckerRunPage, error) {
	return apigateway.GameCheckerRunPage{}, errSnapshotDown
}

func TestSyncKindSurfacesEveryUpstreamFailure(t *testing.T) {
	g := newRealtimeGateway(failingSnapshotClient{}, time.Hour, "admin")
	for _, kind := range []streamKind{streamScoreboard, streamAdminScoreboard, streamAttacks, streamGameStatus, streamSchedulerEvents, streamCheckerRuns} {
		if err := g.syncKind(context.Background(), kind); !errors.Is(err, errSnapshotDown) {
			t.Fatalf("%s: want upstream error, got %v", kind, err)
		}
	}
	if err := g.syncKind(context.Background(), streamKind("unknown")); err != nil {
		t.Fatalf("unknown kinds are ignored, got %v", err)
	}
	_ = g.syncOnce(context.Background())
	var metrics bytes.Buffer
	g.WritePrometheusMetrics(&metrics)
	if !strings.Contains(metrics.String(), "adplatform_realtime_gateway_last_sync_success 0") {
		t.Fatalf("a failed sync must be reported:\n%s", metrics.String())
	}
}

func TestPublishIfChangedSkipsDuplicatesAndFullSubscribers(t *testing.T) {
	g := newRealtimeGateway(&testSnapshotClient{}, time.Hour, "admin")
	_, updates, unsubscribe, ok := g.subscribe(streamAttacks, "client", false)
	if !ok {
		t.Fatal("subscribe")
	}
	defer unsubscribe()
	for _, kind := range []streamKind{streamScoreboard, streamAdminScoreboard, streamAttacks, streamGameStatus, streamSchedulerEvents, streamCheckerRuns} {
		g.publishIfChanged(kind, []byte(`{"v":1}`))
		g.publishIfChanged(kind, []byte(`{"v":1}`)) // unchanged: no second broadcast
	}
	g.publishIfChanged(streamKind("unknown"), []byte(`{}`))
	if g.current(streamKind("unknown")) != nil {
		t.Fatal("unknown kinds have no snapshot")
	}
	if len(updates) != 1 {
		t.Fatalf("a duplicate payload must not be re-broadcast, got %d updates", len(updates))
	}
	// Overfill the subscriber buffer: extra updates are dropped, never block.
	for i := 0; i < 20; i++ {
		g.publishIfChanged(streamAttacks, []byte{byte('a' + i)})
	}
	if len(updates) != cap(updates) {
		t.Fatalf("buffer should be full without blocking, got %d/%d", len(updates), cap(updates))
	}
}

func TestClientKeyAndAdminAuthBranches(t *testing.T) {
	g := newRealtimeGateway(&testSnapshotClient{}, time.Hour, "").withSubscriberLimits(1, 1, 1, true)
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("X-Forwarded-For", " 203.0.113.9 , 10.0.0.1")
	if got := g.clientKey(request); got != "203.0.113.9" {
		t.Fatalf("trusted proxy key: %q", got)
	}
	request.Header.Del("X-Forwarded-For")
	request.RemoteAddr = "no-port"
	if got := g.clientKey(request); got != "no-port" {
		t.Fatalf("bare remote addr: %q", got)
	}
	recorder := httptest.NewRecorder()
	if g.requireAdminAuth(recorder, request) || recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured admin token must be 503, got %d", recorder.Code)
	}
}

type nonFlushingWriter struct{ header http.Header }

func (w *nonFlushingWriter) Header() http.Header         { return w.header }
func (w *nonFlushingWriter) Write(b []byte) (int, error) { return len(b), nil }
func (w *nonFlushingWriter) WriteHeader(int)             {}

// brokenStreamWriter flushes but fails every write after the first n.
type brokenStreamWriter struct {
	mu     sync.Mutex
	header http.Header
	okLeft int
}

func (w *brokenStreamWriter) Header() http.Header { return w.header }
func (w *brokenStreamWriter) WriteHeader(int)     {}
func (w *brokenStreamWriter) Flush()              {}
func (w *brokenStreamWriter) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.okLeft <= 0 {
		return 0, errors.New("client went away")
	}
	w.okLeft--
	return len(b), nil
}

func TestHandleStreamBranches(t *testing.T) {
	client := &testSnapshotClient{scoreboard: []apigateway.ScoreRowAlias{{Team: "A"}}}
	g := newRealtimeGateway(client, time.Hour, "admin")
	request := httptest.NewRequest(http.MethodGet, "/public/v1/scoreboard/stream", nil)

	plain := &nonFlushingWriter{header: http.Header{}}
	g.handleStream(plain, request, streamScoreboard, false) // returns 500 without streaming

	// Snapshot write fails immediately.
	g.handleStream(&brokenStreamWriter{header: http.Header{}}, request, streamScoreboard, false)

	// The first snapshot succeeds; the next update write fails.
	done := make(chan struct{})
	go func() {
		defer close(done)
		g.handleStream(&brokenStreamWriter{header: http.Header{}, okLeft: 1}, request, streamScoreboard, false)
	}()
	waitForSubscribers(t, g, streamScoreboard, 1)
	g.publishIfChanged(streamScoreboard, []byte(`[{"team":"B"}]`))
	<-done

	// Keepalive write fails once the stream is idle.
	g.keepaliveInterval = 10 * time.Millisecond
	g.handleStream(&brokenStreamWriter{header: http.Header{}, okLeft: 1}, request, streamScoreboard, false)

	// A closed update channel ends the stream.
	done = make(chan struct{})
	go func() {
		defer close(done)
		g.keepaliveInterval = time.Hour
		g.handleStream(&brokenStreamWriter{header: http.Header{}, okLeft: 10}, request, streamScoreboard, false)
	}()
	waitForSubscribers(t, g, streamScoreboard, 1)
	g.mu.Lock()
	for id, subscriber := range g.subscribers[streamScoreboard] {
		close(subscriber.updates)
		delete(g.subscribers[streamScoreboard], id)
		g.publicPool.total--
		g.publicPool.byClient[subscriber.clientKey]--
	}
	g.mu.Unlock()
	<-done

	// Keepalives are written while idle until the client disconnects.
	g.keepaliveInterval = 10 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	recorder := newSafeRecorder()
	g.handleStream(recorder, request.WithContext(ctx), streamScoreboard, false)
	if !strings.Contains(recorder.BodyString(), ": keepalive") {
		t.Fatalf("idle stream must send keepalives, got %q", recorder.BodyString())
	}

	limited := newRealtimeGateway(client, time.Hour, "admin").withSubscriberLimits(1, 1, 1, false)
	_, _, unsubscribe, _ := limited.subscribe(streamScoreboard, "192.0.2.1", false)
	defer unsubscribe()
	overflow := httptest.NewRecorder()
	limited.handleStream(overflow, request, streamScoreboard, false)
	if overflow.Code != http.StatusTooManyRequests || overflow.Header().Get("Retry-After") != "5" {
		t.Fatalf("subscriber limit must answer 429, got %d", overflow.Code)
	}
}

func waitForSubscribers(t *testing.T, g *realtimeGateway, kind streamKind, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		g.mu.RLock()
		count := len(g.subscribers[kind])
		g.mu.RUnlock()
		if count >= want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d %s subscribers", want, kind)
}

func TestRunPollsUntilCancelled(t *testing.T) {
	client := &testSnapshotClient{scoreboard: []apigateway.ScoreRowAlias{{Team: "A"}}}
	g := newRealtimeGateway(client, 10*time.Millisecond, "")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()
	g.Run(ctx)
	if len(g.current(streamScoreboard)) == 0 {
		t.Fatal("polling must populate the scoreboard snapshot")
	}
}

func TestSnapshotErrorBodyReadFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("short"))
	}))
	defer server.Close()
	if _, err := fetchSnapshot[int](context.Background(), http.DefaultClient, server.URL); err == nil || !strings.Contains(err.Error(), "status 502") {
		t.Fatalf("a truncated error body must fall back to the status, got %v", err)
	}
}

func TestSyncKindSurfacesEncodingFailures(t *testing.T) {
	original := marshalJSON
	t.Cleanup(func() { marshalJSON = original })
	marshalJSON = func(any) ([]byte, error) { return nil, errors.New("encode failed") }
	g := newRealtimeGateway(&testSnapshotClient{}, time.Hour, "admin")
	for _, kind := range []streamKind{streamScoreboard, streamAdminScoreboard, streamAttacks, streamGameStatus, streamSchedulerEvents, streamCheckerRuns} {
		if err := g.syncKind(context.Background(), kind); err == nil {
			t.Fatalf("%s: encoding failure must surface", kind)
		}
	}
}

func TestRejectedSubscriptionReturnsHarmlessUnsubscribe(t *testing.T) {
	g := newRealtimeGateway(&testSnapshotClient{}, time.Hour, "admin").withSubscriberLimits(1, 1, 1, false)
	_, _, first, _ := g.subscribe(streamAttacks, "c", false)
	defer first()
	_, updates, unsubscribe, ok := g.subscribe(streamAttacks, "c", false)
	if ok || updates != nil {
		t.Fatal("over-limit subscription must be rejected")
	}
	unsubscribe() // must be safe to call
	if g.publicPool.total != 1 {
		t.Fatalf("a rejected unsubscribe must not change counts, got %d", g.publicPool.total)
	}
}
