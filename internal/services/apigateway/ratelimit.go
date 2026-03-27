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
	"strconv"
	"strings"
	"time"

	"adplatform/internal/platform/httpapi"
)

type rateLimitPolicy struct {
	capacity        int
	refillPerSecond float64
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
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}

func (s *Server) allowRateLimit(ctx context.Context, key string, policy rateLimitPolicy) (rateLimitDecision, bool) {
	decision, err := s.rateLimiter.Allow(ctx, key, policy)
	if err != nil {
		log.Printf("rate limiter failed open for key %q: %v", key, err)
		return rateLimitDecision{allowed: true}, true
	}
	return decision, decision.allowed
}

func writeRateLimitFailure(w http.ResponseWriter, decision rateLimitDecision, message string) {
	retryAfter := int(math.Ceil(decision.retryAfter.Seconds()))
	if retryAfter <= 0 {
		retryAfter = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
	httpapi.WriteJSON(w, http.StatusTooManyRequests, httpapi.ErrorEnvelope{
		Status:  "too many request",
		Message: message,
	})
}

func rateLimitTeamKey(prefix string, teamID int) string {
	return fmt.Sprintf("%s:team:%d", prefix, teamID)
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

var (
	authRateLimitPolicy         = rateLimitPolicy{capacity: 5, refillPerSecond: 5.0 / 60.0}
	challengesRateLimitPolicy   = rateLimitPolicy{capacity: 4, refillPerSecond: 2}
	servicesReadRateLimitPolicy = rateLimitPolicy{capacity: 6, refillPerSecond: 2}
	scoreboardRateLimitPolicy   = rateLimitPolicy{capacity: 6, refillPerSecond: 3}
	attacksReadRateLimitPolicy  = rateLimitPolicy{capacity: 6, refillPerSecond: 3}
	teamServicesRateLimitPolicy = rateLimitPolicy{capacity: 6, refillPerSecond: 2}
	submitRateLimitPolicy       = rateLimitPolicy{capacity: 30, refillPerSecond: 10}
	unlockRateLimitPolicy       = rateLimitPolicy{capacity: 10, refillPerSecond: 10.0 / 60.0}
	sshSessionRateLimitPolicy   = rateLimitPolicy{capacity: 6, refillPerSecond: 6.0 / 60.0}
	factoryResetRateLimitPolicy = rateLimitPolicy{capacity: 3, refillPerSecond: 3.0 / 60.0}
	restartRateLimitPolicy      = rateLimitPolicy{capacity: 6, refillPerSecond: 6.0 / 60.0}
	defaultRateLimit429Message  = "no bruteforce needed, calm down a little bit."
)
