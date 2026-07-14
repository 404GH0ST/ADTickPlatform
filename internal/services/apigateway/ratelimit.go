package apigateway

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type rateLimitPolicy struct {
	capacity          int
	refillPerSecond   float64
	failClosedOnError bool
}

type rateLimitDecision struct {
	allowed    bool
	retryAfter time.Duration
}

type rateLimiter interface {
	Allow(ctx context.Context, key string, policy rateLimitPolicy) (rateLimitDecision, error)
}

type noopRateLimiter struct{}

type redisRateLimiter struct {
	addr        string
	password    string
	dialTimeout time.Duration
}

var redisTokenBucketScript = strings.TrimSpace(`
local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local refill_per_ms = tonumber(ARGV[2])
local now_ms = tonumber(ARGV[3])
local requested = tonumber(ARGV[4])
local ttl_ms = tonumber(ARGV[5])

local state = redis.call('HMGET', key, 'tokens', 'ts')
local tokens = tonumber(state[1])
local ts = tonumber(state[2])

if not tokens then
  tokens = capacity
end
if not ts then
  ts = now_ms
end

local delta = now_ms - ts
if delta < 0 then
  delta = 0
end

tokens = math.min(capacity, tokens + (delta * refill_per_ms))

local allowed = 0
local retry_after_ms = 0
if tokens >= requested then
  tokens = tokens - requested
  allowed = 1
elseif refill_per_ms > 0 then
  retry_after_ms = math.ceil((requested - tokens) / refill_per_ms)
end

redis.call('HMSET', key, 'tokens', tostring(tokens), 'ts', tostring(now_ms))
redis.call('PEXPIRE', key, ttl_ms)

return {allowed, retry_after_ms}
`)

func newRateLimiter(redisAddr, redisPassword string) rateLimiter {
	if strings.TrimSpace(redisAddr) == "" {
		return noopRateLimiter{}
	}
	return &redisRateLimiter{
		addr:        strings.TrimSpace(redisAddr),
		password:    strings.TrimSpace(redisPassword),
		dialTimeout: 2 * time.Second,
	}
}

func NewRateLimiter(redisAddr, redisPassword string) rateLimiter {
	return newRateLimiter(redisAddr, redisPassword)
}

func (noopRateLimiter) Allow(_ context.Context, _ string, _ rateLimitPolicy) (rateLimitDecision, error) {
	return rateLimitDecision{allowed: true}, nil
}

func (l *redisRateLimiter) Allow(ctx context.Context, key string, policy rateLimitPolicy) (rateLimitDecision, error) {
	if policy.capacity <= 0 || policy.refillPerSecond <= 0 {
		return rateLimitDecision{allowed: true}, nil
	}

	dialer := net.Dialer{Timeout: l.dialTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", l.addr)
	if err != nil {
		return rateLimitDecision{}, err
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else {
		_ = conn.SetDeadline(time.Now().Add(l.dialTimeout))
	}

	reader := bufio.NewReader(conn)
	if l.password != "" {
		if err := writeRedisCommand(conn, "AUTH", l.password); err != nil {
			return rateLimitDecision{}, err
		}
		if _, err := readRedisValue(reader); err != nil {
			return rateLimitDecision{}, err
		}
	}

	idleTTL := time.Duration(math.Ceil((float64(policy.capacity)/policy.refillPerSecond)*2*1000)) * time.Millisecond
	if idleTTL < 5*time.Second {
		idleTTL = 5 * time.Second
	}

	nowMillis := time.Now().UTC().UnixMilli()
	if err := writeRedisCommand(
		conn,
		"EVAL",
		redisTokenBucketScript,
		"1",
		"ratelimit:"+key,
		strconv.Itoa(policy.capacity),
		strconv.FormatFloat(policy.refillPerSecond/1000, 'f', 6, 64),
		strconv.FormatInt(nowMillis, 10),
		"1",
		strconv.FormatInt(idleTTL.Milliseconds(), 10),
	); err != nil {
		return rateLimitDecision{}, err
	}

	reply, err := readRedisValue(reader)
	if err != nil {
		return rateLimitDecision{}, err
	}

	values, ok := reply.([]any)
	if !ok || len(values) != 2 {
		return rateLimitDecision{}, fmt.Errorf("unexpected redis rate-limit response %T", reply)
	}

	allowed, ok := values[0].(int64)
	if !ok {
		return rateLimitDecision{}, fmt.Errorf("unexpected redis allow type %T", values[0])
	}
	retryAfterMillis, ok := values[1].(int64)
	if !ok {
		return rateLimitDecision{}, fmt.Errorf("unexpected redis retry-after type %T", values[1])
	}

	return rateLimitDecision{
		allowed:    allowed == 1,
		retryAfter: time.Duration(retryAfterMillis) * time.Millisecond,
	}, nil
}

func writeRedisCommand(w io.Writer, parts ...string) error {
	var builder bytes.Buffer
	builder.WriteString(fmt.Sprintf("*%d\r\n", len(parts)))
	for _, part := range parts {
		builder.WriteString(fmt.Sprintf("$%d\r\n%s\r\n", len(part), part))
	}
	_, err := w.Write(builder.Bytes())
	return err
}

func readRedisValue(reader *bufio.Reader) (any, error) {
	prefix, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}

	switch prefix {
	case '+':
		return readRedisLine(reader)
	case '-':
		line, lineErr := readRedisLine(reader)
		if lineErr != nil {
			return nil, lineErr
		}
		return nil, errors.New(line)
	case ':':
		line, lineErr := readRedisLine(reader)
		if lineErr != nil {
			return nil, lineErr
		}
		value, parseErr := strconv.ParseInt(line, 10, 64)
		if parseErr != nil {
			return nil, parseErr
		}
		return value, nil
	case '$':
		line, lineErr := readRedisLine(reader)
		if lineErr != nil {
			return nil, lineErr
		}
		length, parseErr := strconv.Atoi(line)
		if parseErr != nil {
			return nil, parseErr
		}
		if length < 0 {
			return "", nil
		}
		buffer := make([]byte, length+2)
		if _, err := io.ReadFull(reader, buffer); err != nil {
			return nil, err
		}
		return string(buffer[:length]), nil
	case '*':
		line, lineErr := readRedisLine(reader)
		if lineErr != nil {
			return nil, lineErr
		}
		length, parseErr := strconv.Atoi(line)
		if parseErr != nil {
			return nil, parseErr
		}
		if length < 0 {
			return []any(nil), nil
		}
		items := make([]any, 0, length)
		for range length {
			item, itemErr := readRedisValue(reader)
			if itemErr != nil {
				return nil, itemErr
			}
			items = append(items, item)
		}
		return items, nil
	default:
		return nil, fmt.Errorf("unsupported redis response prefix %q", string(prefix))
	}
}

func readRedisLine(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r"), nil
}

func clientRateLimitKey(r *http.Request) string {
	if trustProxyRateLimitHeaders() {
		return forwardedRateLimitKey(r)
	}
	return remoteRateLimitKey(r)
}

func trustProxyRateLimitHeaders() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("API_GATEWAY_TRUST_PROXY_HEADERS"))) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}

func forwardedRateLimitKey(r *http.Request) string {
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if len(parts) > 0 {
			if client := strings.TrimSpace(parts[0]); client != "" {
				return client
			}
		}
	}
	return remoteRateLimitKey(r)
}

func remoteRateLimitKey(r *http.Request) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}

func (s *Server) allowRateLimit(ctx context.Context, key string, policy rateLimitPolicy) (rateLimitDecision, bool) {
	decision, err := s.rateLimiter.Allow(ctx, key, policy)
	if err != nil {
		if policy.failClosedOnError {
			log.Printf("rate limiter failed closed for key %q: %v", key, err)
			s.rateLimitMetrics.recordHit(endpointFromKey(key))
			return rateLimitDecision{allowed: false, retryAfter: 5 * time.Second}, false
		}
		log.Printf("rate limiter failed open for key %q: %v", key, err)
		return rateLimitDecision{allowed: true}, true
	}
	if !decision.allowed {
		s.rateLimitMetrics.recordHit(endpointFromKey(key))
	}
	return decision, decision.allowed
}

func endpointFromKey(key string) string {
	if i := strings.IndexByte(key, ':'); i > 0 {
		return key[:i]
	}
	return key
}

type rateLimitMetrics struct {
	hits sync.Map // map[string]*atomic.Uint64
}

func newRateLimitMetrics() *rateLimitMetrics {
	return &rateLimitMetrics{}
}

func (m *rateLimitMetrics) recordHit(endpoint string) {
	if m == nil || endpoint == "" {
		return
	}
	actual, _ := m.hits.LoadOrStore(endpoint, &atomic.Uint64{})
	actual.(*atomic.Uint64).Add(1)
}

func (m *rateLimitMetrics) WritePrometheus(w io.Writer) {
	if m == nil {
		return
	}
	fmt.Fprintln(w, "# HELP adplatform_rate_limit_exceeded_total Requests rejected by the rate limiter, labelled by endpoint.")
	fmt.Fprintln(w, "# TYPE adplatform_rate_limit_exceeded_total counter")
	m.hits.Range(func(key, value any) bool {
		endpoint, ok := key.(string)
		if !ok {
			return true
		}
		count, ok := value.(*atomic.Uint64)
		if !ok {
			return true
		}
		fmt.Fprintf(w, "adplatform_rate_limit_exceeded_total{endpoint=%q} %d\n", endpoint, count.Load())
		return true
	})
}

func writeRateLimitFailure(w http.ResponseWriter, decision rateLimitDecision, message string) {
	retryAfter := int(math.Ceil(decision.retryAfter.Seconds()))
	if retryAfter <= 0 {
		retryAfter = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
	writeProblem(w, http.StatusTooManyRequests, "Too many requests", message)
}

func rateLimitTeamKey(prefix string, teamID int) string {
	return fmt.Sprintf("%s:team:%d", prefix, teamID)
}

func rateLimitUserKey(prefix string, userID int) string {
	return fmt.Sprintf("%s:user:%d", prefix, userID)
}

func rateLimitClientKey(prefix, clientIP string) string {
	return fmt.Sprintf("%s:client:%s", prefix, clientIP)
}

func rateLimitTeamChallengeKey(prefix string, teamID, challengeID int) string {
	return fmt.Sprintf("%s:team:%d:challenge:%d", prefix, teamID, challengeID)
}

func rateLimitAuthKey(email, clientIP string) string {
	normalizedEmail := strings.TrimSpace(strings.ToLower(email))
	if normalizedEmail == "" {
		normalizedEmail = "unknown"
	}
	return fmt.Sprintf("auth:email:%s:ip:%s", normalizedEmail, clientIP)
}

func rateLimitRegistrationEmailKey(email string) string {
	normalizedEmail := strings.TrimSpace(strings.ToLower(email))
	if normalizedEmail == "" {
		normalizedEmail = "unknown"
	}
	return fmt.Sprintf("register:email:%s", normalizedEmail)
}

var (
	maxSubmitFlagsPerRequest         = 128
	authRateLimitPolicy              = rateLimitPolicy{capacity: 5, refillPerSecond: 5.0 / 60.0, failClosedOnError: true}
	registrationIPRateLimitPolicy    = rateLimitPolicy{capacity: 5, refillPerSecond: 5.0 / 60.0, failClosedOnError: true}
	registrationEmailRateLimitPolicy = rateLimitPolicy{capacity: 3, refillPerSecond: 3.0 / 3600.0, failClosedOnError: true}
	challengesRateLimitPolicy        = rateLimitPolicy{capacity: 4, refillPerSecond: 2, failClosedOnError: false}
	servicesReadRateLimitPolicy      = rateLimitPolicy{capacity: 6, refillPerSecond: 3, failClosedOnError: false}
	scoreboardRateLimitPolicy        = rateLimitPolicy{capacity: 6, refillPerSecond: 3, failClosedOnError: false}
	attacksReadRateLimitPolicy       = rateLimitPolicy{capacity: 6, refillPerSecond: 3, failClosedOnError: false}
	teamServicesRateLimitPolicy      = rateLimitPolicy{capacity: 6, refillPerSecond: 3, failClosedOnError: false}
	challengeSourceRateLimitPolicy   = rateLimitPolicy{capacity: 3, refillPerSecond: 1, failClosedOnError: true}
	submitRateLimitPolicy            = rateLimitPolicy{capacity: 45, refillPerSecond: 15, failClosedOnError: true}
	submitUserRateLimitPolicy        = rateLimitPolicy{capacity: 15, refillPerSecond: 5, failClosedOnError: true}
	unlockRateLimitPolicy            = rateLimitPolicy{capacity: 10, refillPerSecond: 10.0 / 60.0, failClosedOnError: true}
	sshSessionRateLimitPolicy        = rateLimitPolicy{capacity: 6, refillPerSecond: 6.0 / 60.0, failClosedOnError: true}
	factoryResetRateLimitPolicy      = rateLimitPolicy{capacity: 3, refillPerSecond: 3.0 / 60.0, failClosedOnError: true}
	restartRateLimitPolicy           = rateLimitPolicy{capacity: 6, refillPerSecond: 6.0 / 60.0, failClosedOnError: true}
	defaultRateLimit429Message       = "no bruteforce needed, calm down a little bit."
)
