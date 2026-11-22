package apigateway

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql/driver"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"adplatform/internal/platform/httpapi"
)

// signedToken signs raw (already encoded) header and payload segments, so a
// forged token passes the signature check and must be caught by what follows.
func signedToken(secret, headerPart, payloadPart string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(headerPart + "." + payloadPart))
	return headerPart + "." + payloadPart + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func TestVerifyTeamJWTRejectsTamperedTokens(t *testing.T) {
	const secret = "jwt-secret"
	now := time.Now()
	b64 := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
	goodHeader := b64(`{"alg":"HS256","typ":"JWT"}`)
	claims := func(extra string) string {
		return b64(fmt.Sprintf(`{"team_id":101,"player_id":7,"session_version":1,"exp":%d%s}`, now.Add(time.Hour).Unix(), extra))
	}
	valid := signedToken(secret, goodHeader, claims(""))
	if _, err := verifyTeamJWT(secret, valid, now); err != nil {
		t.Fatalf("the reference token must verify: %v", err)
	}
	cases := map[string]struct{ secret, token string }{
		"empty secret":         {"", valid},
		"two segments":         {secret, goodHeader + "." + claims("")},
		"wrong key":            {"other-secret", valid},
		"signature not base64": {secret, goodHeader + "." + claims("") + ".***"},
		"header not base64":    {secret, signedToken(secret, "***", claims(""))},
		"header not json":      {secret, signedToken(secret, b64("not json"), claims(""))},
		"alg none":             {secret, signedToken(secret, b64(`{"alg":"none","typ":"JWT"}`), claims(""))},
		"alg HS512":            {secret, signedToken(secret, b64(`{"alg":"HS512","typ":"JWT"}`), claims(""))},
		"typ missing":          {secret, signedToken(secret, b64(`{"alg":"HS256"}`), claims(""))},
		"payload not base64":   {secret, signedToken(secret, goodHeader, "***")},
		"payload not json":     {secret, signedToken(secret, goodHeader, b64("[1,2]"))},
		"expired":              {secret, signedToken(secret, goodHeader, b64(fmt.Sprintf(`{"player_id":7,"session_version":1,"exp":%d}`, now.Add(-time.Second).Unix())))},
		"no player":            {secret, signedToken(secret, goodHeader, b64(fmt.Sprintf(`{"session_version":1,"exp":%d}`, now.Add(time.Hour).Unix())))},
		"organizer with team":  {secret, signedToken(secret, goodHeader, claims(`,"role":"organizer"`))},
	}
	for name, tc := range cases {
		if _, err := verifyTeamJWT(tc.secret, tc.token, now); !errors.Is(err, ErrInvalidTeamToken) {
			t.Errorf("%s: must be rejected, got %v", name, err)
		}
	}
	organizer := signedToken(secret, goodHeader, b64(fmt.Sprintf(`{"team_id":0,"player_id":1,"role":"Organizer","session_version":1,"exp":%d}`, now.Add(time.Hour).Unix())))
	if _, err := verifyTeamJWT(secret, organizer, now); err != nil {
		t.Errorf("a teamless organizer token is valid: %v", err)
	}
	if _, err := issueTeamJWT("  ", authenticatedPlayer{PlayerID: 1}, now); err == nil {
		t.Error("tokens cannot be issued without a secret")
	}
}

func TestPasswordHashParsingFailsClosed(t *testing.T) {
	good := hashPassword("correct horse")
	if !passwordMatches(good, "correct horse") || passwordMatches(good, "wrong") {
		t.Fatal("the reference hash must verify only its password")
	}
	parts := strings.Split(good, "$")
	salt, hash := parts[2], parts[3]
	for name, stored := range map[string]string{
		"too few fields":  "pbkdf2-sha256$i=1$" + salt,
		"wrong scheme":    "pbkdf2-sha1$i=1$" + salt + "$" + hash,
		"no iterations":   "pbkdf2-sha256$n=1$" + salt + "$" + hash,
		"bad iterations":  "pbkdf2-sha256$i=x$" + salt + "$" + hash,
		"zero iterations": "pbkdf2-sha256$i=0$" + salt + "$" + hash,
		"no salt":         "pbkdf2-sha256$i=1$x=1$" + hash,
		"no hash":         "pbkdf2-sha256$i=1$" + salt + "$x=1",
		"salt not base64": "pbkdf2-sha256$i=1$s=***$" + hash,
		"hash not base64": "pbkdf2-sha256$i=1$" + salt + "$h=***",
		"empty key":       "pbkdf2-sha256$i=1$" + salt + "$h=",
	} {
		if verifyPBKDF2Password(stored, "correct horse") {
			t.Errorf("%s: a malformed hash must never verify", name)
		}
	}
	if !passwordMatches(hashSecret("legacy"), "legacy") || !passwordMatches(legacyMD5Secret("legacy"), "legacy") || !passwordHashNeedsUpgrade(hashSecret("legacy")) {
		t.Error("legacy hashes verify and are flagged for upgrade")
	}
}

func TestStoreHelpers(t *testing.T) {
	if DefaultServiceSubnetOctet(0) != 1 || DefaultServiceSubnetOctet(4) != 4 {
		t.Error("subnet octet")
	}
	for raw, want := range map[string]string{"host": "host:0", "host:abc": "host:0", " 10.0.0.1 : 22 ": "10.0.0.1:22"} {
		if host, port := ParseEndpoint(raw); fmt.Sprintf("%s:%d", host, port) != want {
			t.Errorf("ParseEndpoint(%q) = %s:%d", raw, host, port)
		}
	}
	for _, bad := range [][2]int{{0, 1}, {70000, 1}, {80, 0}, {80, 255}} {
		if err := validateChallengeRuntimeConfig(bad[0], bad[1]); !errors.Is(err, ErrInvalidRuntimeConfig) {
			t.Errorf("runtime config %v must be invalid: %v", bad, err)
		}
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Error("a root password needs a secret")
			}
		}()
		stableRootPassword(" ", 1, 1, "instance")
	}()
	for raw, want := range map[string]string{"": "service", "!!!": "service", "Web App 2": "web-app-2"} {
		if got := slugName(raw); got != want {
			t.Errorf("slugName(%q) = %q", raw, got)
		}
	}
	if teamServiceOctet(0) != 1 || teamServiceOctet(5) != 0 || teamServiceOctet(101) != 11 {
		t.Error("team service octet")
	}
	if got := splitCSVList(" a, ,b "); strings.Join(got, "|") != "a|b" || splitCSVList(" ") != nil {
		t.Errorf("splitCSVList: %q", got)
	}
	if demoteStatus("degraded") != "warming" || demoteStatus("stable") != "stable" {
		t.Error("demoteStatus")
	}
	if len(NewTeamJoinKey()) != len("TEAM-XXXXXXXX-XXXXXXXX-XXXXXXXX") || newCheckerToken() == newCheckerToken() {
		t.Error("random identifiers")
	}
	if marshalAuditMetadata(map[string]any{"bad": make(chan int)}) != "{}" || auditChallengeTarget(3, " ") != "challenge:3" {
		t.Error("audit helpers")
	}
	past, future := time.Now().Add(-time.Hour), time.Now().Add(time.Hour)
	if (scoreboardFreezeWindow{FreezeAt: &past, UnfreezeAt: &past}).activeAt(time.Now()) || !(scoreboardFreezeWindow{FreezeAt: &past, UnfreezeAt: &future}).activeAt(time.Now()) {
		t.Error("a freeze ends at its unfreeze time")
	}
}

func TestServiceStateSummaryHelpers(t *testing.T) {
	if summary := SummarizeCheckerRunsForTick(nil, 3); summary.Status != "unknown" || summary.TickID != 3 {
		t.Errorf("no runs: %+v", summary)
	}
	if summary := SummarizeCheckerRunsForTick([]GameCheckerRun{{Phase: " ", Status: "success"}}, 3); summary.Status != "unknown" || summary.Phase != "put" {
		t.Errorf("phaseless runs leave the cycle running: %+v", summary)
	}
	for raw, want := range map[string]string{"OK": "ok", "recovering": "recovering", "flag-not-found": "flag_not_found", "faulty": "faulty", "down": "down", "unknown": "unknown", "odd": ""} {
		if got := NormalizeServiceStateStatus(raw); got != want {
			t.Errorf("NormalizeServiceStateStatus(%q) = %q", raw, got)
		}
	}
	for status, want := range map[string]int{"failed": 3, "Skipped": 2, "success": 1, "running": 0} {
		if got := statusRank(status); got != want {
			t.Errorf("statusRank(%q) = %d", status, got)
		}
	}
	run := &GameCheckerRun{Status: "success"}
	full := map[string]tickPhaseState{"put": {run: run, success: true}, "get": {run: run, success: true}, "check": {run: run, success: true}}
	if nextMissingCheckerPhase(full) != "" || dominantFailurePhase(full) != "put" || dominantFailurePhase(nil) != "" {
		t.Error("phase helpers on a complete cycle")
	}
	if !strings.Contains(dominantFailureMessage(full), "did not complete") || phaseMessage(tickPhaseState{run: run}, "fallback") != "fallback" {
		t.Error("message helpers")
	}
}

func TestNoopGameCoreClientIsDisabled(t *testing.T) {
	ctx := context.Background()
	client := noopGameCoreClient{}
	calls := []func() error{
		func() error { _, err := client.MatchStatus(ctx); return err },
		func() error { _, err := client.StartMatch(ctx); return err },
		func() error { _, err := client.PauseMatch(ctx); return err },
		func() error { _, err := client.ResumeMatch(ctx); return err },
		func() error { _, err := client.StopMatch(ctx); return err },
		func() error { _, err := client.UpdateMatchSchedule(ctx, UpdateMatchScheduleRequest{}); return err },
		func() error { _, err := client.AdvanceTick(ctx); return err },
		func() error { _, err := client.SchedulerStatus(ctx); return err },
		func() error { _, err := client.SchedulerEvents(ctx, GameSchedulerEventQuery{}); return err },
		func() error { _, err := client.StartScheduler(ctx); return err },
		func() error { _, err := client.StopScheduler(ctx); return err },
		func() error { _, err := client.UpdateScheduler(ctx, UpdateSchedulerRequest{}); return err },
		func() error { _, err := client.RecomputeScoring(ctx); return err },
		func() error { _, err := client.AuditScoring(ctx); return err },
		func() error { _, err := client.FlagFormat(ctx); return err },
		func() error { _, err := client.RefreshFlagFormat(ctx, "X"); return err },
		func() error { _, err := client.Submissions(ctx, SubmissionQuery{}); return err },
		func() error { _, err := client.DeleteSubmission(ctx, 1); return err },
	}
	for i, call := range calls {
		if err := call(); !errors.Is(err, errGameCoreDisabled) {
			t.Errorf("noop call %d: %v", i, err)
		}
	}
}

func TestInternalClientTransportFailures(t *testing.T) {
	ctx := context.Background()
	game := &httpGameCoreClient{baseURL: "http://bad host", client: http.DefaultClient}
	if _, err := requestGameCoreJSON[FlagFormatStatus](ctx, game, http.MethodPost, "/x", make(chan int)); err == nil {
		t.Error("an unencodable game-core body must fail")
	}
	if _, err := requestGameCoreJSON[FlagFormatStatus](ctx, game, http.MethodGet, "/x"); err == nil {
		t.Error("an unbuildable game-core request must fail")
	}
	game.baseURL = closedServerURL(t)
	if _, err := requestGameCoreJSON[FlagFormatStatus](ctx, game, http.MethodGet, "/x"); err == nil {
		t.Error("an unreachable game-core must fail")
	}
	truncated := truncatedBodyServer(t, http.StatusBadGateway).URL
	game.baseURL = truncated
	var gameErr gameCoreHTTPError
	if _, err := requestGameCoreJSON[FlagFormatStatus](ctx, game, http.MethodGet, "/x"); !errors.As(err, &gameErr) || gameErr.Message != "" {
		t.Errorf("an unreadable game-core error body has no message: %v", err)
	}
	if _, err := NewHTTPScoringClient(truncated, "").Scoreboard(ctx); err == nil || !strings.Contains(err.Error(), "status 502") {
		t.Errorf("an unreadable scoring error body reports the status: %v", err)
	}
	if _, err := NewHTTPSubmissionClient(truncated, "").SubmitFlags(ctx, 1, []string{"x"}); err == nil {
		t.Error("an unreadable submission error body must fail")
	}
}

// silentListener accepts TCP connections at the kernel level but never reads,
// so a large enough write blocks until its deadline.
func silentListener(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	return listener.Addr().String()
}

func TestRateLimiterResidualBranches(t *testing.T) {
	ctx := context.Background()
	huge := strings.Repeat("x", 32<<20)
	for name, limiter := range map[string]*redisRateLimiter{
		"auth write":    {addr: silentListener(t), password: huge, dialTimeout: 200 * time.Millisecond},
		"command write": {addr: silentListener(t), dialTimeout: 200 * time.Millisecond},
	} {
		if _, err := limiter.Allow(ctx, huge, authRateLimitPolicy); err == nil {
			t.Errorf("%s: a stalled Redis write must fail", name)
		}
	}

	// A fast-refilling bucket still keeps its key for the five-second floor.
	server := newFakeRedis(t, "*2\r\n:1\r\n:0\r\n")
	fast := &redisRateLimiter{addr: server.addr(), dialTimeout: time.Second}
	if _, err := fast.Allow(ctx, "k", rateLimitPolicy{capacity: 1, refillPerSecond: 1000}); err != nil {
		t.Fatal(err)
	}
	if eval := server.recorded()[0]; eval[8] != "5000" {
		t.Errorf("idle ttl floor: %q", eval[8])
	}

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("X-Forwarded-For", " , 203.0.113.9")
	request.RemoteAddr = "198.51.100.1:4000"
	if got := forwardedRateLimitKey(request); got != "198.51.100.1" {
		t.Errorf("a blank first hop falls back to the peer: %q", got)
	}
	request.RemoteAddr = "@unix"
	if got := remoteRateLimitKey(request); got != "@unix" {
		t.Errorf("a portless peer is used verbatim: %q", got)
	}
	if endpointFromKey("nocolon") != "nocolon" || retryAfterSeconds(rateLimitDecision{}) != 1 {
		t.Error("key and retry helpers")
	}
	server2 := &Server{rateLimiter: &redisRateLimiter{addr: closedServerURL(t)[len("http://"):], dialTimeout: 100 * time.Millisecond}}
	server2.refundRateLimit(ctx, "k", authRateLimitPolicy) // logs and keeps the cooldown
}

// emptyWireGuardStore hands out a peer whose config was never rendered.
type emptyWireGuardStore struct{ Store }

func (emptyWireGuardStore) GetAdminPlayerWireGuardConfig(context.Context, int) (adminWireGuardPeer, error) {
	return adminWireGuardPeer{}, nil
}

func TestParticipantWireGuardDownloadEdges(t *testing.T) {
	store := NewMemoryStore(101)
	env, token := residualMux(t, emptyWireGuardStore{Store: store}, nil)
	if response := env.do(http.MethodGet, "/api/v2/me/wireguard", token, nil); response.Code != http.StatusNotFound {
		t.Errorf("an unrendered config: %d", response.Code)
	}
	limited := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "api-gateway", Version: "test", Addr: ":0"})
	NewWithDeps("dev-team-token", "dev-admin-token", 101, store, storeBackedControllerClient{store: store}, noopWireGuardClient{}).
		WithRateLimiter(prefixLimiter{prefix: "wireguard-download:"}).RegisterRoutes(limited)
	env.mux = limited
	if response := env.do(http.MethodGet, "/api/v2/me/wireguard", token, nil); response.Code != http.StatusTooManyRequests {
		t.Errorf("a rate-limited download: %d", response.Code)
	}
}

func TestOperationsAlertHelpers(t *testing.T) {
	now := time.Now()
	ids := func(alerts []AdminOperationsAlert) string {
		names := make([]string, 0, len(alerts))
		for _, alert := range alerts {
			names = append(names, alert.ID)
		}
		return strings.Join(names, ",")
	}
	running := &GameMatchStatus{State: "running"}
	if got := ids(evaluateGameOperationsAlerts(GameStatus{Match: running}, now)); !strings.Contains(got, "scheduler-missing") {
		t.Errorf("a running match with no scheduler: %q", got)
	}
	if got := ids(evaluateGameOperationsAlerts(GameStatus{Match: running, Scheduler: &GameSchedulerStatus{State: "stopped"}}, now)); !strings.Contains(got, "scheduler-stopped") {
		t.Errorf("a running match with a stopped scheduler: %q", got)
	}
	if got := ids(evaluateGameOperationsAlerts(GameStatus{Match: &GameMatchStatus{State: "finished"}, Scheduler: &GameSchedulerStatus{State: "running"}}, now)); !strings.Contains(got, "scheduler-running-after-finish") {
		t.Errorf("a finished match with a running scheduler: %q", got)
	}
	stale := now.Add(-2 * time.Hour).Format(time.RFC3339)
	deployments := []adminDeploymentJob{{Status: "completed"}, {Status: "queued", CreatedAt: "not a time"}, {Status: "running", CreatedAt: stale}}
	if got := evaluateDeploymentOperationsAlerts(deployments, now); len(got) != 1 || !strings.Contains(got[0].Detail, "2h00m") {
		t.Errorf("a two-hour-old job stalls the queue: %+v", got)
	}
	if got := ids(evaluateWireGuardOperationsAlerts(WireGuardGatewayStatus{State: "error"}, nil)); got != "wireguard-not-applied" {
		t.Errorf("an unapplied gateway: %q", got)
	}
	if got := ids(evaluateAccessOperationsAlerts(ControllerAccessStatus{State: "error"}, nil)); !strings.Contains(got, "access-not-applied") {
		t.Errorf("unapplied access policies: %q", got)
	}
	if severityRank("info") != 2 || humanDuration(0) != "1s" || maxDuration(time.Second, time.Minute) != time.Minute || maxDuration(time.Minute, time.Second) != time.Minute {
		t.Error("ranking and duration helpers")
	}
	if active, revoked := expectedWireGuardPeerCounts([]WireGuardGatewayPeer{{Status: "active"}, {Status: " Revoked "}}); active != 1 || revoked != 1 {
		t.Errorf("peer counts: %d active %d revoked", active, revoked)
	}
	running2 := &GameCheckerRun{Status: "running"}
	if noCheckerPhaseRan(map[string]tickPhaseState{"put": {run: running2}}) {
		t.Error("a phase that is still running has not been skipped")
	}
}

func TestRetryBadConnection(t *testing.T) {
	bad := errors.New("write tcp: bad connection")
	calls := 0
	value, err := retryBadConnection(func() (int, error) {
		calls++
		if calls == 1 {
			return 0, driver.ErrBadConn
		}
		return 7, nil
	})
	if err != nil || value != 7 || calls != 2 {
		t.Fatalf("one retry after a bad connection: %d %v (%d calls)", value, err, calls)
	}
	calls = 0
	if _, err := retryBadConnection(func() (int, error) { calls++; return 0, bad }); !errors.Is(err, bad) || calls != 2 {
		t.Fatalf("a connection that stays bad fails after the retry: %v (%d calls)", err, calls)
	}
	calls = 0
	if _, err := retryBadConnection(func() (int, error) { calls++; return 0, errStoreDown }); !errors.Is(err, errStoreDown) || calls != 1 {
		t.Fatalf("other errors are not retried: %v (%d calls)", err, calls)
	}
}
