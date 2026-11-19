package apigateway

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeRedis is a minimal RESP server: it records each command it receives and
// answers with the next scripted raw reply.
type fakeRedis struct {
	listener net.Listener
	mu       sync.Mutex
	commands [][]string
	replies  []string
}

func newFakeRedis(t *testing.T, replies ...string) *fakeRedis {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := &fakeRedis{listener: listener, replies: replies}
	t.Cleanup(func() { listener.Close() })
	go server.serve()
	return server
}

func (f *fakeRedis) addr() string { return f.listener.Addr().String() }

func (f *fakeRedis) serve() {
	for {
		conn, err := f.listener.Accept()
		if err != nil {
			return
		}
		go f.handle(conn)
	}
}

func (f *fakeRedis) handle(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	for {
		command, err := readRESPCommand(reader)
		if err != nil {
			return
		}
		f.mu.Lock()
		f.commands = append(f.commands, command)
		reply := "-ERR no scripted reply\r\n"
		if len(f.replies) > 0 {
			reply = f.replies[0]
			f.replies = f.replies[1:]
		}
		f.mu.Unlock()
		if _, err := io.WriteString(conn, reply); err != nil {
			return
		}
	}
}

func (f *fakeRedis) recorded() [][]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([][]string(nil), f.commands...)
}

func readRESPCommand(reader *bufio.Reader) ([]string, error) {
	header, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(header, "*") {
		return nil, fmt.Errorf("expected array, got %q", header)
	}
	count, err := strconv.Atoi(strings.TrimSpace(header[1:]))
	if err != nil {
		return nil, err
	}
	parts := make([]string, 0, count)
	for range count {
		lengthLine, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		length, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(lengthLine, "$")))
		if err != nil {
			return nil, err
		}
		buffer := make([]byte, length+2)
		if _, err := io.ReadFull(reader, buffer); err != nil {
			return nil, err
		}
		parts = append(parts, string(buffer[:length]))
	}
	return parts, nil
}

func TestNewRateLimiterSelectsBackend(t *testing.T) {
	if _, ok := NewRateLimiter("  ", "").(noopRateLimiter); !ok {
		t.Fatal("blank redis address must select the noop limiter")
	}
	limiter, ok := NewRateLimiter(" 127.0.0.1:6379 ", " secret ").(*redisRateLimiter)
	if !ok {
		t.Fatal("redis address must select the redis limiter")
	}
	if limiter.addr != "127.0.0.1:6379" || limiter.password != "secret" || limiter.dialTimeout <= 0 {
		t.Fatalf("unexpected redis limiter config: %+v", limiter)
	}
	if decision, err := (noopRateLimiter{}).Allow(context.Background(), "k", authRateLimitPolicy); err != nil || !decision.allowed {
		t.Fatalf("noop limiter must always allow, got %+v %v", decision, err)
	}
}

func TestRedisRateLimiterAllowEncodesTokenBucketCall(t *testing.T) {
	server := newFakeRedis(t, "+OK\r\n", "*2\r\n:1\r\n:0\r\n")
	limiter := &redisRateLimiter{addr: server.addr(), password: "hunter2", dialTimeout: time.Second}

	// A key carrying CR/LF must travel as one length-prefixed bulk string and
	// never split into extra Redis commands.
	key := "auth:email:evil\r\nFLUSHALL\r\n:ip:203.0.113.9"
	decision, err := limiter.Allow(context.Background(), key, authRateLimitPolicy)
	if err != nil || !decision.allowed || decision.retryAfter != 0 {
		t.Fatalf("expected allowed decision, got %+v %v", decision, err)
	}

	commands := server.recorded()
	if len(commands) != 2 {
		t.Fatalf("expected AUTH then EVAL, got %q", commands)
	}
	if strings.Join(commands[0], " ") != "AUTH hunter2" {
		t.Fatalf("unexpected auth command %q", commands[0])
	}
	eval := commands[1]
	if len(eval) != 9 || eval[0] != "EVAL" || eval[1] != redisTokenBucketScript || eval[2] != "1" {
		t.Fatalf("unexpected eval shape %q", eval)
	}
	if eval[3] != "ratelimit:"+key {
		t.Fatalf("key was not sent verbatim as one argument: %q", eval[3])
	}
	if eval[4] != strconv.Itoa(authRateLimitPolicy.capacity) || eval[5] != formatRefillPerMillisecond(authRateLimitPolicy.refillPerSecond) || eval[7] != "1" {
		t.Fatalf("unexpected policy arguments %q", eval[4:])
	}
	if ttl, err := strconv.ParseInt(eval[8], 10, 64); err != nil || ttl < 5000 {
		t.Fatalf("idle ttl must be at least 5s, got %q", eval[8])
	}
}

func TestRedisRateLimiterAllowReportsDenialAndRetryAfter(t *testing.T) {
	server := newFakeRedis(t, "*2\r\n:0\r\n:2500\r\n")
	limiter := &redisRateLimiter{addr: server.addr(), dialTimeout: time.Second}
	decision, err := limiter.Allow(context.Background(), "submit:team:1", submitRateLimitPolicy)
	if err != nil || decision.allowed || decision.retryAfter != 2500*time.Millisecond {
		t.Fatalf("expected denied decision with retry-after, got %+v %v", decision, err)
	}
	if commands := server.recorded(); len(commands) != 1 || commands[0][0] != "EVAL" {
		t.Fatalf("no AUTH expected without a password, got %q", commands)
	}
}

func TestRedisRateLimiterAllowSkipsDisabledPolicies(t *testing.T) {
	limiter := &redisRateLimiter{addr: "127.0.0.1:1", dialTimeout: 10 * time.Millisecond}
	for _, policy := range []rateLimitPolicy{{capacity: 0, refillPerSecond: 1}, {capacity: 1, refillPerSecond: 0}} {
		decision, err := limiter.Allow(context.Background(), "k", policy)
		if err != nil || !decision.allowed {
			t.Fatalf("disabled policy %+v must allow without contacting redis, got %+v %v", policy, decision, err)
		}
		if err := limiter.Refund(context.Background(), "k", policy); err != nil {
			t.Fatalf("disabled policy %+v refund must be a no-op, got %v", policy, err)
		}
	}
}

func TestRedisRateLimiterAllowRejectsMalformedReplies(t *testing.T) {
	cases := map[string]string{
		"error reply":        "-ERR script failed\r\n",
		"not an array":       ":1\r\n",
		"short array":        "*1\r\n:1\r\n",
		"non-integer allow":  "*2\r\n+yes\r\n:0\r\n",
		"non-integer retry":  "*2\r\n:1\r\n+soon\r\n",
		"bad integer":        ":abc\r\n",
		"unknown prefix":     "!weird\r\n",
		"truncated":          "*2\r\n:1\r\n",
		"bad array length":   "*x\r\n",
		"bad element":        "*2\r\n:x\r\n:0\r\n",
		"nested error":       "*2\r\n-ERR inner\r\n:0\r\n",
		"truncated line":     "+no newline",
		"bad bulk length":    "$x\r\n",
		"truncated bulk":     "$10\r\nabc",
		"error line missing": "-",
		"int line missing":   ":",
		"bulk line missing":  "$",
		"array line missing": "*",
	}
	for name, reply := range cases {
		t.Run(name, func(t *testing.T) {
			server := newFakeRedis(t, reply)
			limiter := &redisRateLimiter{addr: server.addr(), dialTimeout: 200 * time.Millisecond}
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()
			if decision, err := limiter.Allow(ctx, "k", authRateLimitPolicy); err == nil {
				t.Fatalf("malformed reply %q must surface an error, got %+v", reply, decision)
			}
		})
	}
}

func TestRedisRateLimiterAuthFailureIsAnError(t *testing.T) {
	server := newFakeRedis(t, "-WRONGPASS invalid password\r\n")
	limiter := &redisRateLimiter{addr: server.addr(), password: "wrong", dialTimeout: time.Second}
	if _, err := limiter.Allow(context.Background(), "k", authRateLimitPolicy); err == nil || !strings.Contains(err.Error(), "WRONGPASS") {
		t.Fatalf("expected auth error, got %v", err)
	}
}

func TestRedisRateLimiterDialFailureIsAnError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	listener.Close()
	limiter := &redisRateLimiter{addr: addr, dialTimeout: 200 * time.Millisecond}
	if _, err := limiter.Allow(context.Background(), "k", authRateLimitPolicy); err == nil {
		t.Fatal("expected dial error")
	}
}

func TestRedisRateLimiterRefundSendsRefundScript(t *testing.T) {
	server := newFakeRedis(t, ":1\r\n")
	limiter := &redisRateLimiter{addr: server.addr(), dialTimeout: time.Second}
	if err := limiter.Refund(context.Background(), "factory-reset:team:1:challenge:2", factoryResetRateLimitPolicy); err != nil {
		t.Fatalf("refund: %v", err)
	}
	commands := server.recorded()
	if len(commands) != 1 || len(commands[0]) != 5 || commands[0][1] != redisTokenRefundScript || commands[0][3] != "ratelimit:factory-reset:team:1:challenge:2" || commands[0][4] != "1" {
		t.Fatalf("unexpected refund command %q", commands)
	}
}

func TestReadRedisValueDecodesReplyTypes(t *testing.T) {
	cases := []struct {
		raw  string
		want any
	}{
		{"+OK\r\n", "OK"},
		{":42\r\n", int64(42)},
		{"$5\r\nhello\r\n", "hello"},
		{"$-1\r\n", ""},
		{"*-1\r\n", []any(nil)},
		{"*2\r\n$1\r\na\r\n:7\r\n", []any{"a", int64(7)}},
	}
	for _, tc := range cases {
		got, err := readRedisValue(bufio.NewReader(strings.NewReader(tc.raw)))
		if err != nil {
			t.Fatalf("decode %q: %v", tc.raw, err)
		}
		if fmt.Sprint(got) != fmt.Sprint(tc.want) {
			t.Fatalf("decode %q = %#v, want %#v", tc.raw, got, tc.want)
		}
	}
	if _, err := readRedisValue(bufio.NewReader(strings.NewReader(""))); err == nil {
		t.Fatal("empty input must error")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestWriteRedisCommandFramesBulkStrings(t *testing.T) {
	var buffer bytes.Buffer
	if err := writeRedisCommand(&buffer, "SET", "a\r\nb", ""); err != nil {
		t.Fatal(err)
	}
	if got := buffer.String(); got != "*3\r\n$3\r\nSET\r\n$4\r\na\r\nb\r\n$0\r\n\r\n" {
		t.Fatalf("unexpected framing %q", got)
	}
	if err := writeRedisCommand(failingWriter{}, "PING"); err == nil {
		t.Fatal("write error must propagate")
	}
}

func TestRateLimitMetricsWritePrometheus(t *testing.T) {
	var nilMetrics *rateLimitMetrics
	nilMetrics.recordHit("auth")
	nilMetrics.WritePrometheus(io.Discard)

	metrics := newRateLimitMetrics()
	metrics.recordHit("")
	metrics.recordHit("auth")
	metrics.recordHit("auth")
	metrics.recordHit("submit")
	metrics.hits.Store(42, "ignored non-string key")
	metrics.hits.Store("broken", "ignored non-counter value")
	var buffer bytes.Buffer
	metrics.WritePrometheus(&buffer)
	out := buffer.String()
	for _, want := range []string{
		"# TYPE adplatform_rate_limit_exceeded_total counter",
		`adplatform_rate_limit_exceeded_total{endpoint="auth"} 2`,
		`adplatform_rate_limit_exceeded_total{endpoint="submit"} 1`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("metrics output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "broken") {
		t.Fatalf("non-counter values must be skipped:\n%s", out)
	}
}

// TestRedisTokenBucketScriptAgainstRealRedis runs the actual Lua scripts, so
// the brute-force throttle semantics are verified, not just the protocol.
// Set TEST_REDIS_ADDR (and optionally TEST_REDIS_PASSWORD) to enable it.
func TestRedisTokenBucketScriptAgainstRealRedis(t *testing.T) {
	addr := strings.TrimSpace(os.Getenv("TEST_REDIS_ADDR"))
	if addr == "" {
		t.Skip("TEST_REDIS_ADDR is not set")
	}
	limiter := newRateLimiter(addr, os.Getenv("TEST_REDIS_PASSWORD")).(*redisRateLimiter)
	ctx := context.Background()
	key := fmt.Sprintf("test:%d", time.Now().UnixNano())
	policy := rateLimitPolicy{capacity: 3, refillPerSecond: 3.0 / 3600.0, failClosedOnError: true}

	for attempt := 1; attempt <= 3; attempt++ {
		decision, err := limiter.Allow(ctx, key, policy)
		if err != nil || !decision.allowed {
			t.Fatalf("attempt %d within capacity must be allowed, got %+v %v", attempt, decision, err)
		}
	}
	denied, err := limiter.Allow(ctx, key, policy)
	if err != nil || denied.allowed {
		t.Fatalf("attempt over capacity must be denied, got %+v %v", denied, err)
	}
	if denied.retryAfter < 15*time.Minute {
		t.Fatalf("slow policy must report a long retry-after, got %s", denied.retryAfter)
	}

	if err := limiter.Refund(ctx, key, policy); err != nil {
		t.Fatalf("refund: %v", err)
	}
	if decision, err := limiter.Allow(ctx, key, policy); err != nil || !decision.allowed {
		t.Fatalf("a refunded token must be usable once, got %+v %v", decision, err)
	}
	if decision, err := limiter.Allow(ctx, key, policy); err != nil || decision.allowed {
		t.Fatalf("refund must return exactly one token, got %+v %v", decision, err)
	}

	missingKey := key + ":missing"
	if err := limiter.Refund(ctx, missingKey, policy); err != nil {
		t.Fatalf("refunding a missing bucket must succeed: %v", err)
	}
	for attempt := 1; attempt <= 3; attempt++ {
		if decision, err := limiter.Allow(ctx, missingKey, policy); err != nil || !decision.allowed {
			t.Fatalf("refund on a missing bucket must not grant extra tokens (attempt %d): %+v %v", attempt, decision, err)
		}
	}
	if decision, err := limiter.Allow(ctx, missingKey, policy); err != nil || decision.allowed {
		t.Fatalf("missing-bucket refund overfilled the bucket: %+v %v", decision, err)
	}
}
