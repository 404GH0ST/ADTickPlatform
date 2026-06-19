package main

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"adplatform/internal/platform/httpapi"
	"adplatform/internal/services/apigateway"
)

type scoringWorkerStatus struct {
	State            string `json:"state"`
	LastRecomputedAt string `json:"last_recomputed_at,omitempty"`
	LastScoreRows    int    `json:"last_score_rows,omitempty"`
	LastAuditAt      string `json:"last_audit_at,omitempty"`
	LastAuditStatus  string `json:"last_audit_status,omitempty"`
	LastMismatches   int    `json:"last_mismatches,omitempty"`
	LastError        string `json:"last_error,omitempty"`
}

type scoringWorkerServer struct {
	adminToken string
	gameCore   scoringGameCoreClient
	now        func() time.Time

	mu     sync.RWMutex
	status scoringWorkerStatus
}

type scoringGameCoreClient interface {
	Scoreboard(ctx context.Context) ([]apigateway.ScoreRowAlias, error)
	RecomputeScoring(ctx context.Context) ([]apigateway.ScoreRowAlias, error)
	AuditScoring(ctx context.Context) (apigateway.ScoringAuditAlias, error)
}

func newScoringWorkerServer(adminToken string, gameCore scoringGameCoreClient) *scoringWorkerServer {
	if gameCore == nil {
		gameCore = noopGameCoreScoringClient{}
	}
	return &scoringWorkerServer{
		adminToken: strings.TrimSpace(adminToken),
		gameCore:   gameCore,
		now:        time.Now,
		status: scoringWorkerStatus{
			State: "ready",
		},
	}
}

func (s *scoringWorkerServer) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /internal/v1/scoring/status", s.handleStatus)
	mux.HandleFunc("GET /internal/v1/scoring/scoreboard", s.handleScoreboard)
	mux.HandleFunc("POST /internal/v1/scoring/recompute", s.handleRecompute)
	mux.HandleFunc("GET /internal/v1/scoring/audit", s.handleAudit)
}

func (s *scoringWorkerServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, s.snapshotStatus())
}

func (s *scoringWorkerServer) handleScoreboard(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	rows, err := s.gameCore.Scoreboard(r.Context())
	if err != nil {
		s.recordFailure(err)
		writeScoringWorkerFailure(w, err, "game-core scoreboard failed.")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, rows)
}

func (s *scoringWorkerServer) handleRecompute(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	rows, err := s.gameCore.RecomputeScoring(r.Context())
	if err != nil {
		s.recordFailure(err)
		writeScoringWorkerFailure(w, err, "game-core score recompute failed.")
		return
	}
	s.recordSuccess(len(rows))
	httpapi.WriteJSON(w, http.StatusOK, rows)
}

func (s *scoringWorkerServer) handleAudit(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	report, err := s.gameCore.AuditScoring(r.Context())
	if err != nil {
		s.recordFailure(err)
		writeScoringWorkerFailure(w, err, "game-core scoring audit failed.")
		return
	}
	s.recordAudit(report)
	httpapi.WriteJSON(w, http.StatusOK, report)
}

func (s *scoringWorkerServer) requireAdminAuth(w http.ResponseWriter, r *http.Request) bool {
	token, ok := httpapi.BearerToken(r)
	if !ok || subtle.ConstantTimeCompare([]byte(token), []byte(s.adminToken)) != 1 {
		httpapi.WriteProblem(w, http.StatusForbidden, httpapi.ProblemDetails{
			Title:  "Forbidden",
			Detail: "please authenticate before accessing scoring-worker endpoints.",
		})
		return false
	}
	return true
}

func (s *scoringWorkerServer) snapshotStatus() scoringWorkerStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

func (s *scoringWorkerServer) recordSuccess(rowCount int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.State = "ready"
	s.status.LastError = ""
	s.status.LastScoreRows = rowCount
	s.status.LastRecomputedAt = s.now().UTC().Format(time.RFC3339)
}

func (s *scoringWorkerServer) recordAudit(report apigateway.ScoringAuditAlias) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.State = "ready"
	s.status.LastError = ""
	s.status.LastAuditAt = s.now().UTC().Format(time.RFC3339)
	s.status.LastAuditStatus = report.Status
	s.status.LastMismatches = report.MismatchCount
	if report.MismatchCount > 0 {
		log.Printf("scoring audit mismatch: mismatches=%d stored_rows=%d replayed_rows=%d", report.MismatchCount, report.StoredRows, report.ReplayedRows)
	}
}

func (s *scoringWorkerServer) recordFailure(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.State = "degraded"
	s.status.LastError = strings.TrimSpace(err.Error())
}

func writeScoringWorkerFailure(w http.ResponseWriter, err error, fallback string) {
	message := strings.TrimSpace(fallback)
	if message == "" {
		message = "scoring-worker request failed."
	}
	if errors.Is(err, errGameCoreScoringDisabled) {
		message = "scoring-worker is not connected to game-core."
	}
	httpapi.WriteProblem(w, http.StatusBadGateway, httpapi.ProblemDetails{
		Title:  "Upstream unavailable",
		Detail: message,
	})
}

func (s *scoringWorkerServer) WritePrometheusMetrics(w io.Writer) {
	status := s.snapshotStatus()

	fmt.Fprintln(w, "# HELP adplatform_scoring_worker_last_score_rows Last recomputed scoreboard row count.")
	fmt.Fprintln(w, "# TYPE adplatform_scoring_worker_last_score_rows gauge")
	fmt.Fprintf(w, "adplatform_scoring_worker_last_score_rows %d\n", status.LastScoreRows)

	fmt.Fprintln(w, "# HELP adplatform_scoring_worker_last_audit_mismatches Last scoring replay audit mismatch count.")
	fmt.Fprintln(w, "# TYPE adplatform_scoring_worker_last_audit_mismatches gauge")
	fmt.Fprintf(w, "adplatform_scoring_worker_last_audit_mismatches %d\n", status.LastMismatches)

	fmt.Fprintln(w, "# HELP adplatform_scoring_worker_last_audit_ok Whether the last scoring replay audit reported ok.")
	fmt.Fprintln(w, "# TYPE adplatform_scoring_worker_last_audit_ok gauge")
	fmt.Fprintf(w, "adplatform_scoring_worker_last_audit_ok %.0f\n", boolMetric(status.LastAuditStatus == "ok"))

	fmt.Fprintln(w, "# HELP adplatform_scoring_worker_degraded Whether scoring-worker currently reports a degraded state.")
	fmt.Fprintln(w, "# TYPE adplatform_scoring_worker_degraded gauge")
	fmt.Fprintf(w, "adplatform_scoring_worker_degraded %.0f\n", boolMetric(status.State == "degraded"))
}

func boolMetric(value bool) float64 {
	if value {
		return 1
	}
	return 0
}
