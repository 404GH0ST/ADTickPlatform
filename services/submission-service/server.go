package main

import (
	"context"
	"crypto/subtle"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"adplatform/internal/platform/httpapi"
	"adplatform/internal/services/apigateway"
)

type submissionServiceServer struct {
	adminToken string
	gameCore   submissionGameCoreClient
	metrics    submissionServiceMetrics
}

type submissionServiceMetrics struct {
	mu sync.Mutex

	submitRequestsTotal            uint64
	submitFlagsTotal               uint64
	submitFailuresTotal            uint64
	submitDurationSecondsSum       float64
	submitDurationSecondsCount     uint64
	submitVerdictClassTotals       map[string]uint64
	attackFeedRequestsTotal        uint64
	attackFeedItemsTotal           uint64
	attackFeedFailuresTotal        uint64
	attackFeedDurationSecondsSum   float64
	attackFeedDurationSecondsCount uint64
}

type submissionGameCoreClient interface {
	SubmitFlags(ctx context.Context, teamID int, flags []string) ([]apigateway.SubmissionVerdictAlias, error)
	AttackFeed(ctx context.Context, query apigateway.AttackFeedQuery) (apigateway.AttackFeedPage, error)
}

func newSubmissionServiceServer(adminToken string, gameCore submissionGameCoreClient) *submissionServiceServer {
	if gameCore == nil {
		gameCore = noopGameCoreSubmissionClient{}
	}
	return &submissionServiceServer{
		adminToken: strings.TrimSpace(adminToken),
		gameCore:   gameCore,
		metrics: submissionServiceMetrics{
			submitVerdictClassTotals: make(map[string]uint64),
		},
	}
}

func (s *submissionServiceServer) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /internal/v1/submissions/submit", s.handleSubmit)
	mux.HandleFunc("GET /internal/v1/submissions/attacks", s.handleAttackFeed)
}

func (s *submissionServiceServer) handleSubmit(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	started := time.Now()

	var request apigateway.GameSubmitFlagsRequest
	if err := httpapi.DecodeJSON(r, &request); err != nil || request.TeamID <= 0 || len(request.Flags) == 0 {
		writeProblem(w, http.StatusBadRequest, "Invalid request", "submission request is invalid.")
		return
	}

	results, err := s.gameCore.SubmitFlags(r.Context(), request.TeamID, request.Flags)
	if err != nil {
		s.metrics.recordSubmitFailure(time.Since(started), len(request.Flags))
		writeSubmissionServiceFailure(w, err, "game-core flag submission failed.")
		return
	}
	s.metrics.recordSubmitSuccess(time.Since(started), len(request.Flags), results)
	writeData(w, http.StatusOK, results)
}

func (s *submissionServiceServer) handleAttackFeed(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	started := time.Now()

	page, err := s.gameCore.AttackFeed(r.Context(), apigateway.AttackFeedQuery{
		Limit:    parsePositiveQueryInt(r, "limit", 12, 200),
		Offset:   parseNonNegativeQueryInt(r, "offset"),
		Attacker: strings.TrimSpace(r.URL.Query().Get("attacker")),
		Victim:   strings.TrimSpace(r.URL.Query().Get("victim")),
		Service:  strings.TrimSpace(r.URL.Query().Get("service")),
		TickFrom: parseNonNegativeQueryInt(r, "tick_from"),
		TickTo:   parseNonNegativeQueryInt(r, "tick_to"),
	})
	if err != nil {
		s.metrics.recordAttackFeedFailure(time.Since(started))
		writeSubmissionServiceFailure(w, err, "game-core attack feed failed.")
		return
	}
	s.metrics.recordAttackFeedSuccess(time.Since(started), len(page.Items))
	writeData(w, http.StatusOK, page)
}

func (s *submissionServiceServer) requireAdminAuth(w http.ResponseWriter, r *http.Request) bool {
	token, ok := httpapi.BearerToken(r)
	if !ok || subtle.ConstantTimeCompare([]byte(token), []byte(s.adminToken)) != 1 {
		writeProblem(w, http.StatusForbidden, "Forbidden", "please authenticate before accessing submission-service endpoints.")
		return false
	}
	return true
}

func writeSubmissionServiceFailure(w http.ResponseWriter, err error, fallback string) {
	message := strings.TrimSpace(fallback)
	if message == "" {
		message = "submission-service request failed."
	}
	if errors.Is(err, errGameCoreSubmissionDisabled) {
		message = "submission-service is not connected to game-core."
	}
	writeProblem(w, http.StatusBadGateway, "Upstream unavailable", message)
}

func parsePositiveQueryInt(r *http.Request, key string, fallback int, max int) int {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= 0 {
		return fallback
	}
	if max > 0 && parsed > max {
		return max
	}
	return parsed
}

func parseNonNegativeQueryInt(r *http.Request, key string) int {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return 0
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed < 0 {
		return 0
	}
	return parsed
}

func (s *submissionServiceServer) WritePrometheusMetrics(w io.Writer) {
	s.metrics.writePrometheus(w)
}

func (m *submissionServiceMetrics) recordSubmitSuccess(duration time.Duration, flags int, verdicts []apigateway.SubmissionVerdictAlias) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.submitRequestsTotal++
	m.submitFlagsTotal += nonNegativeUint64(flags)
	m.submitDurationSecondsSum += duration.Seconds()
	m.submitDurationSecondsCount++
	for _, verdict := range verdicts {
		m.submitVerdictClassTotals[classifySubmissionVerdict(verdict.Detail)]++
	}
}

func writeData(w http.ResponseWriter, statusCode int, value any) {
	httpapi.WriteJSON(w, statusCode, value)
}

func writeProblem(w http.ResponseWriter, statusCode int, title, detail string) {
	httpapi.WriteProblem(w, statusCode, httpapi.ProblemDetails{
		Title:  title,
		Status: statusCode,
		Detail: detail,
	})
}

func (m *submissionServiceMetrics) recordSubmitFailure(duration time.Duration, flags int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.submitRequestsTotal++
	m.submitFlagsTotal += nonNegativeUint64(flags)
	m.submitFailuresTotal++
	m.submitDurationSecondsSum += duration.Seconds()
	m.submitDurationSecondsCount++
}

func (m *submissionServiceMetrics) recordAttackFeedSuccess(duration time.Duration, items int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.attackFeedRequestsTotal++
	m.attackFeedItemsTotal += nonNegativeUint64(items)
	m.attackFeedDurationSecondsSum += duration.Seconds()
	m.attackFeedDurationSecondsCount++
}

func (m *submissionServiceMetrics) recordAttackFeedFailure(duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.attackFeedRequestsTotal++
	m.attackFeedFailuresTotal++
	m.attackFeedDurationSecondsSum += duration.Seconds()
	m.attackFeedDurationSecondsCount++
}

func nonNegativeUint64(value int) uint64 {
	if value <= 0 {
		return 0
	}
	return uint64(value)
}

func (m *submissionServiceMetrics) writePrometheus(w io.Writer) {
	m.mu.Lock()
	defer m.mu.Unlock()

	io.WriteString(w, "# HELP adplatform_submission_service_submit_requests_total Total submit requests handled by submission-service.\n")
	io.WriteString(w, "# TYPE adplatform_submission_service_submit_requests_total counter\n")
	io.WriteString(w, "adplatform_submission_service_submit_requests_total ")
	io.WriteString(w, strconv.FormatUint(m.submitRequestsTotal, 10))
	io.WriteString(w, "\n")

	io.WriteString(w, "# HELP adplatform_submission_service_submit_flags_total Total flags processed by submission-service submit requests.\n")
	io.WriteString(w, "# TYPE adplatform_submission_service_submit_flags_total counter\n")
	io.WriteString(w, "adplatform_submission_service_submit_flags_total ")
	io.WriteString(w, strconv.FormatUint(m.submitFlagsTotal, 10))
	io.WriteString(w, "\n")

	io.WriteString(w, "# HELP adplatform_submission_service_submit_failures_total Total failed submit requests.\n")
	io.WriteString(w, "# TYPE adplatform_submission_service_submit_failures_total counter\n")
	io.WriteString(w, "adplatform_submission_service_submit_failures_total ")
	io.WriteString(w, strconv.FormatUint(m.submitFailuresTotal, 10))
	io.WriteString(w, "\n")

	io.WriteString(w, "# HELP adplatform_submission_service_submit_duration_seconds_sum Total duration of submit requests in seconds.\n")
	io.WriteString(w, "# TYPE adplatform_submission_service_submit_duration_seconds_sum counter\n")
	io.WriteString(w, "adplatform_submission_service_submit_duration_seconds_sum ")
	io.WriteString(w, strconv.FormatFloat(m.submitDurationSecondsSum, 'f', 6, 64))
	io.WriteString(w, "\n")

	io.WriteString(w, "# HELP adplatform_submission_service_submit_duration_seconds_count Total observed submit requests for duration accounting.\n")
	io.WriteString(w, "# TYPE adplatform_submission_service_submit_duration_seconds_count counter\n")
	io.WriteString(w, "adplatform_submission_service_submit_duration_seconds_count ")
	io.WriteString(w, strconv.FormatUint(m.submitDurationSecondsCount, 10))
	io.WriteString(w, "\n")

	io.WriteString(w, "# HELP adplatform_submission_service_submit_verdicts_total Total submission verdicts grouped by class.\n")
	io.WriteString(w, "# TYPE adplatform_submission_service_submit_verdicts_total counter\n")
	for _, class := range []string{"correct", "duplicate", "invalid", "unknown"} {
		io.WriteString(w, "adplatform_submission_service_submit_verdicts_total{class=\"")
		io.WriteString(w, class)
		io.WriteString(w, "\"} ")
		io.WriteString(w, strconv.FormatUint(m.submitVerdictClassTotals[class], 10))
		io.WriteString(w, "\n")
	}

	io.WriteString(w, "# HELP adplatform_submission_service_attack_feed_requests_total Total attack-feed requests handled by submission-service.\n")
	io.WriteString(w, "# TYPE adplatform_submission_service_attack_feed_requests_total counter\n")
	io.WriteString(w, "adplatform_submission_service_attack_feed_requests_total ")
	io.WriteString(w, strconv.FormatUint(m.attackFeedRequestsTotal, 10))
	io.WriteString(w, "\n")

	io.WriteString(w, "# HELP adplatform_submission_service_attack_feed_items_total Total attack-feed items returned by submission-service.\n")
	io.WriteString(w, "# TYPE adplatform_submission_service_attack_feed_items_total counter\n")
	io.WriteString(w, "adplatform_submission_service_attack_feed_items_total ")
	io.WriteString(w, strconv.FormatUint(m.attackFeedItemsTotal, 10))
	io.WriteString(w, "\n")

	io.WriteString(w, "# HELP adplatform_submission_service_attack_feed_failures_total Total failed attack-feed requests.\n")
	io.WriteString(w, "# TYPE adplatform_submission_service_attack_feed_failures_total counter\n")
	io.WriteString(w, "adplatform_submission_service_attack_feed_failures_total ")
	io.WriteString(w, strconv.FormatUint(m.attackFeedFailuresTotal, 10))
	io.WriteString(w, "\n")

	io.WriteString(w, "# HELP adplatform_submission_service_attack_feed_duration_seconds_sum Total duration of attack-feed requests in seconds.\n")
	io.WriteString(w, "# TYPE adplatform_submission_service_attack_feed_duration_seconds_sum counter\n")
	io.WriteString(w, "adplatform_submission_service_attack_feed_duration_seconds_sum ")
	io.WriteString(w, strconv.FormatFloat(m.attackFeedDurationSecondsSum, 'f', 6, 64))
	io.WriteString(w, "\n")

	io.WriteString(w, "# HELP adplatform_submission_service_attack_feed_duration_seconds_count Total observed attack-feed requests for duration accounting.\n")
	io.WriteString(w, "# TYPE adplatform_submission_service_attack_feed_duration_seconds_count counter\n")
	io.WriteString(w, "adplatform_submission_service_attack_feed_duration_seconds_count ")
	io.WriteString(w, strconv.FormatUint(m.attackFeedDurationSecondsCount, 10))
	io.WriteString(w, "\n")
}

func classifySubmissionVerdict(verdict string) string {
	normalized := strings.ToLower(strings.TrimSpace(verdict))
	switch {
	case strings.Contains(normalized, "correct"):
		return "correct"
	case strings.Contains(normalized, "already submitted"):
		return "duplicate"
	case strings.Contains(normalized, "wrong"), strings.Contains(normalized, "expired"):
		return "invalid"
	default:
		return "unknown"
	}
}
