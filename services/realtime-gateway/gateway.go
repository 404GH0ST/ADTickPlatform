package main

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"adplatform/internal/platform/httpapi"
)

type streamKind string

const (
	streamScoreboard      streamKind = "scoreboard"
	streamAdminScoreboard streamKind = "admin_scoreboard"
	streamAttacks         streamKind = "attacks"
	streamGameStatus      streamKind = "game_status"
	streamSchedulerEvents streamKind = "scheduler_events"
	streamCheckerRuns     streamKind = "checker_runs"
)

type realtimeGateway struct {
	client       publicSnapshotClient
	pollInterval time.Duration
	adminToken   string

	mu                  sync.RWMutex
	scoreboard          []byte
	scoreHash           [32]byte
	adminScoreboard     []byte
	adminScoreHash      [32]byte
	attacks             []byte
	attackHash          [32]byte
	gameStatus          []byte
	gameStatusHash      [32]byte
	schedulerEvents     []byte
	schedulerHash       [32]byte
	checkerRuns         []byte
	checkerRunsHash     [32]byte
	subscribers         map[streamKind]map[int]realtimeSubscriber
	nextSubscriberID    int
	maxSubscribers      int
	maxPerClient        int
	trustProxyHeaders   bool
	subscriberTotal     int
	subscribersByClient map[string]int
	lastSyncAt          time.Time
	lastSyncSuccessful  bool
	syncErrorsTotal     uint64
}

type realtimeSubscriber struct {
	updates   chan []byte
	clientKey string
}

func newRealtimeGateway(client publicSnapshotClient, pollInterval time.Duration, adminToken string) *realtimeGateway {
	return &realtimeGateway{
		client:              client,
		pollInterval:        pollInterval,
		adminToken:          strings.TrimSpace(adminToken),
		maxSubscribers:      2000,
		maxPerClient:        12,
		subscribersByClient: make(map[string]int),
		subscribers: map[streamKind]map[int]realtimeSubscriber{
			streamScoreboard:      make(map[int]realtimeSubscriber),
			streamAdminScoreboard: make(map[int]realtimeSubscriber),
			streamAttacks:         make(map[int]realtimeSubscriber),
			streamGameStatus:      make(map[int]realtimeSubscriber),
			streamSchedulerEvents: make(map[int]realtimeSubscriber),
			streamCheckerRuns:     make(map[int]realtimeSubscriber),
		},
	}
}

func (g *realtimeGateway) withSubscriberLimits(total, perClient int, trustProxyHeaders bool) *realtimeGateway {
	if total > 0 {
		g.maxSubscribers = total
	}
	if perClient > 0 {
		g.maxPerClient = perClient
	}
	g.trustProxyHeaders = trustProxyHeaders
	return g
}

func (g *realtimeGateway) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /public/v1/scoreboard/stream", g.handleScoreboardStream)
	mux.HandleFunc("GET /public/v1/attacks/stream", g.handleAttackStream)
	mux.HandleFunc("GET /admin/v1/game/scoreboard/stream", g.handleAdminScoreboardStream)
	mux.HandleFunc("GET /admin/v1/game/status/stream", g.handleAdminGameStatusStream)
	mux.HandleFunc("GET /admin/v1/game/checker-runs/stream", g.handleAdminCheckerRunsStream)
	mux.HandleFunc("GET /admin/v1/game/scheduler/events/stream", g.handleAdminSchedulerEventsStream)
}

func (g *realtimeGateway) Run(ctx context.Context) {
	_ = g.syncOnce(ctx)

	ticker := time.NewTicker(g.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = g.syncOnce(ctx)
		}
	}
}

func (g *realtimeGateway) syncOnce(ctx context.Context) error {
	var errs []string

	if err := g.syncKind(ctx, streamScoreboard); err != nil {
		errs = append(errs, "scoreboard sync failed: "+err.Error())
	}
	if err := g.syncKind(ctx, streamAttacks); err != nil {
		errs = append(errs, "attack sync failed: "+err.Error())
	}
	if g.adminToken != "" {
		if err := g.syncKind(ctx, streamAdminScoreboard); err != nil {
			errs = append(errs, "admin scoreboard sync failed: "+err.Error())
		}
		if err := g.syncKind(ctx, streamGameStatus); err != nil {
			errs = append(errs, "game status sync failed: "+err.Error())
		}
		if err := g.syncKind(ctx, streamSchedulerEvents); err != nil {
			errs = append(errs, "scheduler events sync failed: "+err.Error())
		}
		if err := g.syncKind(ctx, streamCheckerRuns); err != nil {
			errs = append(errs, "checker runs sync failed: "+err.Error())
		}
	}

	if len(errs) > 0 {
		g.mu.Lock()
		g.lastSyncAt = time.Now().UTC()
		g.lastSyncSuccessful = false
		g.syncErrorsTotal++
		g.mu.Unlock()
		return errors.New(strings.Join(errs, "; "))
	}
	g.mu.Lock()
	g.lastSyncAt = time.Now().UTC()
	g.lastSyncSuccessful = true
	g.mu.Unlock()
	return nil
}

func (g *realtimeGateway) handleScoreboardStream(w http.ResponseWriter, r *http.Request) {
	g.handleStream(w, r, streamScoreboard)
}

func (g *realtimeGateway) handleAttackStream(w http.ResponseWriter, r *http.Request) {
	g.handleStream(w, r, streamAttacks)
}

func (g *realtimeGateway) handleAdminScoreboardStream(w http.ResponseWriter, r *http.Request) {
	if !g.requireAdminAuth(w, r) {
		return
	}
	// Organizers get the live board, which ignores the freeze window, so their
	// stream keeps updating while participants see the frozen snapshot.
	g.handleStream(w, r, streamAdminScoreboard)
}

func (g *realtimeGateway) handleAdminGameStatusStream(w http.ResponseWriter, r *http.Request) {
	if !g.requireAdminAuth(w, r) {
		return
	}
	g.handleStream(w, r, streamGameStatus)
}

func (g *realtimeGateway) handleAdminCheckerRunsStream(w http.ResponseWriter, r *http.Request) {
	if !g.requireAdminAuth(w, r) {
		return
	}
	g.handleStream(w, r, streamCheckerRuns)
}

func (g *realtimeGateway) handleAdminSchedulerEventsStream(w http.ResponseWriter, r *http.Request) {
	if !g.requireAdminAuth(w, r) {
		return
	}
	g.handleStream(w, r, streamSchedulerEvents)
}

func (g *realtimeGateway) handleStream(w http.ResponseWriter, r *http.Request, kind streamKind) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	_, updates, unsubscribe, subscribed := g.subscribe(kind, g.clientKey(r))
	if !subscribed {
		w.Header().Set("Retry-After", "5")
		httpapi.WriteProblem(w, http.StatusTooManyRequests, httpapi.ProblemDetails{
			Title:  "Too many realtime connections",
			Detail: "close an existing realtime stream before opening another.",
		})
		return
	}
	defer unsubscribe()

	if len(g.current(kind)) == 0 {
		_ = g.syncKind(r.Context(), kind)
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if snapshot := g.current(kind); len(snapshot) > 0 {
		if err := writeSSE(w, snapshot); err != nil {
			return
		}
		flusher.Flush()
	}

	keepalive := time.NewTicker(15 * time.Second)
	defer keepalive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case payload, ok := <-updates:
			if !ok {
				return
			}
			if err := writeSSE(w, payload); err != nil {
				return
			}
			flusher.Flush()
		case <-keepalive.C:
			if _, err := w.Write([]byte(": keepalive\n\n")); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func writeSSE(w http.ResponseWriter, payload []byte) error {
	_, err := fmt.Fprintf(w, "data: %s\n\n", payload)
	return err
}

func (g *realtimeGateway) publishIfChanged(kind streamKind, payload []byte) {
	hash := sha256.Sum256(payload)

	g.mu.Lock()
	defer g.mu.Unlock()

	switch kind {
	case streamScoreboard:
		if hash == g.scoreHash {
			return
		}
		g.scoreHash = hash
		g.scoreboard = append([]byte(nil), payload...)
	case streamAdminScoreboard:
		if hash == g.adminScoreHash {
			return
		}
		g.adminScoreHash = hash
		g.adminScoreboard = append([]byte(nil), payload...)
	case streamAttacks:
		if hash == g.attackHash {
			return
		}
		g.attackHash = hash
		g.attacks = append([]byte(nil), payload...)
	case streamGameStatus:
		if hash == g.gameStatusHash {
			return
		}
		g.gameStatusHash = hash
		g.gameStatus = append([]byte(nil), payload...)
	case streamSchedulerEvents:
		if hash == g.schedulerHash {
			return
		}
		g.schedulerHash = hash
		g.schedulerEvents = append([]byte(nil), payload...)
	case streamCheckerRuns:
		if hash == g.checkerRunsHash {
			return
		}
		g.checkerRunsHash = hash
		g.checkerRuns = append([]byte(nil), payload...)
	default:
		return
	}

	for _, subscriber := range g.subscribers[kind] {
		select {
		case subscriber.updates <- append([]byte(nil), payload...):
		default:
		}
	}
}

func (g *realtimeGateway) current(kind streamKind) []byte {
	g.mu.RLock()
	defer g.mu.RUnlock()

	switch kind {
	case streamScoreboard:
		return append([]byte(nil), g.scoreboard...)
	case streamAdminScoreboard:
		return append([]byte(nil), g.adminScoreboard...)
	case streamAttacks:
		return append([]byte(nil), g.attacks...)
	case streamGameStatus:
		return append([]byte(nil), g.gameStatus...)
	case streamSchedulerEvents:
		return append([]byte(nil), g.schedulerEvents...)
	case streamCheckerRuns:
		return append([]byte(nil), g.checkerRuns...)
	default:
		return nil
	}
}

func (g *realtimeGateway) subscribe(kind streamKind, clientKey string) (int, <-chan []byte, func(), bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.subscriberTotal >= g.maxSubscribers || g.subscribersByClient[clientKey] >= g.maxPerClient {
		return 0, nil, func() {}, false
	}

	id := g.nextSubscriberID
	g.nextSubscriberID++
	ch := make(chan []byte, 8)
	g.subscribers[kind][id] = realtimeSubscriber{updates: ch, clientKey: clientKey}
	g.subscriberTotal++
	g.subscribersByClient[clientKey]++

	return id, ch, func() {
		g.mu.Lock()
		defer g.mu.Unlock()
		if subscriber, ok := g.subscribers[kind][id]; ok {
			delete(g.subscribers[kind], id)
			close(subscriber.updates)
			g.subscriberTotal--
			g.subscribersByClient[subscriber.clientKey]--
			if g.subscribersByClient[subscriber.clientKey] == 0 {
				delete(g.subscribersByClient, subscriber.clientKey)
			}
		}
	}, true
}

func (g *realtimeGateway) clientKey(r *http.Request) string {
	if g.trustProxyHeaders {
		if forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0]); forwarded != "" {
			return forwarded
		}
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}

func (g *realtimeGateway) requireAdminAuth(w http.ResponseWriter, r *http.Request) bool {
	if g.adminToken == "" {
		httpapi.WriteProblem(w, http.StatusServiceUnavailable, httpapi.ProblemDetails{
			Title:  "Service unavailable",
			Detail: "admin realtime stream is not configured.",
		})
		return false
	}
	token, ok := httpapi.BearerToken(r)
	if !ok || subtle.ConstantTimeCompare([]byte(token), []byte(g.adminToken)) != 1 {
		httpapi.WriteProblem(w, http.StatusForbidden, httpapi.ProblemDetails{
			Title:  "Forbidden",
			Detail: "please authenticate before accessing admin realtime streams.",
		})
		return false
	}
	return true
}

func (g *realtimeGateway) syncKind(ctx context.Context, kind streamKind) error {
	switch kind {
	case streamScoreboard:
		rows, err := g.client.Scoreboard(ctx)
		if err != nil {
			return err
		}
		payload, err := json.Marshal(rows)
		if err != nil {
			return err
		}
		g.publishIfChanged(kind, payload)
	case streamAdminScoreboard:
		rows, err := g.client.ScoreboardLive(ctx)
		if err != nil {
			return err
		}
		payload, err := json.Marshal(rows)
		if err != nil {
			return err
		}
		g.publishIfChanged(kind, payload)
	case streamAttacks:
		rows, err := g.client.Attacks(ctx)
		if err != nil {
			return err
		}
		payload, err := json.Marshal(rows)
		if err != nil {
			return err
		}
		g.publishIfChanged(kind, payload)
	case streamGameStatus:
		status, err := g.client.GameStatus(ctx)
		if err != nil {
			return err
		}
		payload, err := json.Marshal(status)
		if err != nil {
			return err
		}
		g.publishIfChanged(kind, payload)
	case streamSchedulerEvents:
		events, err := g.client.SchedulerEvents(ctx, 12)
		if err != nil {
			return err
		}
		payload, err := json.Marshal(events)
		if err != nil {
			return err
		}
		g.publishIfChanged(kind, payload)
	case streamCheckerRuns:
		runs, err := g.client.CheckerRuns(ctx, 18)
		if err != nil {
			return err
		}
		payload, err := json.Marshal(runs)
		if err != nil {
			return err
		}
		g.publishIfChanged(kind, payload)
	}
	return nil
}
