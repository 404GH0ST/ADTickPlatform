package main

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"adplatform/internal/platform/httpapi"
	"adplatform/internal/services/apigateway"
)

type gameCoreServer struct {
	adminToken           string
	store                gameStore
	checker              checkerClient
	accessReconcile      accessReconcileClient
	flags                flagCodec
	flagsMu              sync.RWMutex
	scheduler            gameScheduler
	checkerPhases        []string
	checkerTimeout       int
	checkerParallelism   int
	tickTimeout          time.Duration
	scoringDebounce      time.Duration
	scoringRetryDelay    time.Duration
	scoringTimeout       time.Duration
	scoringMu            sync.Mutex
	scoringTimer         *time.Timer
	scoringStatus        scoringRecomputeStatus
	matchStartAt         *time.Time
	matchEndAt           *time.Time
	warmupRequired       bool
	warmupTimeout        time.Duration
	warmupMinSuccess     float64
	warmupPutRetries     int
	warmupPutRetryDelay  time.Duration
	warmupMu             sync.RWMutex
	warmupInFlight       bool
	lastWarmup           apigateway.GameWarmupResult
	autoTickOnMatchStart bool
	now                  func() time.Time
}

// accessReconcileClient re-applies host access policies after a tick so deferred
// (play_from_tick) challenges open to players when their gate is reached.
type accessReconcileClient interface {
	ReconcileAccess(ctx context.Context) error
}

type scoringRecomputeStatus struct {
	Pending       bool
	InFlight      bool
	LastSuccessAt time.Time
	LastFailureAt time.Time
	LastError     string
	FailureCount  uint64
}

func newGameCoreServer(adminToken string, store gameStore, checker checkerClient, flags flagCodec, scheduler gameScheduler, checkerPhases []string, checkerTimeout int) *gameCoreServer {
	if scheduler == nil {
		scheduler = noopGameScheduler{}
	}
	return &gameCoreServer{
		adminToken:           strings.TrimSpace(adminToken),
		store:                store,
		checker:              checker,
		flags:                flags,
		scheduler:            scheduler,
		checkerPhases:        checkerPhases,
		checkerTimeout:       checkerTimeout,
		checkerParallelism:   1,
		tickTimeout:          180 * time.Second,
		scoringDebounce:      time.Second,
		scoringRetryDelay:    5 * time.Second,
		scoringTimeout:       30 * time.Second,
		warmupRequired:       true,
		warmupTimeout:        60 * time.Second,
		warmupMinSuccess:     1.0,
		warmupPutRetries:     2,
		warmupPutRetryDelay:  2 * time.Second,
		autoTickOnMatchStart: true,
		now:                  time.Now,
	}
}

func (s *gameCoreServer) currentFlagFormat() string {
	s.flagsMu.RLock()
	defer s.flagsMu.RUnlock()
	return s.flags.format()
}

func (s *gameCoreServer) reloadFlagFormat(prefix string) string {
	s.flagsMu.Lock()
	defer s.flagsMu.Unlock()
	s.flags = s.flags.withPrefix(prefix)
	return s.flags.format()
}

func (s *gameCoreServer) issueFlag(ownerTeamID, challengeID, issuedTick, expiresTick int) string {
	s.flagsMu.RLock()
	codec := s.flags
	s.flagsMu.RUnlock()
	return codec.Issue(ownerTeamID, challengeID, issuedTick, expiresTick)
}

func (s *gameCoreServer) parseFlag(value string) (flagClaims, bool) {
	s.flagsMu.RLock()
	codec := s.flags
	s.flagsMu.RUnlock()
	return codec.Parse(value)
}

func (s *gameCoreServer) WithCheckerParallelism(value int) *gameCoreServer {
	if value > 0 {
		s.checkerParallelism = value
	}
	return s
}

// WithTickTimeout bounds the wall time of a single tick's checker run. If
// exceeded, the in-flight checker calls are cancelled and the tick is marked
// failed, so a saturated host can't drag a tick indefinitely or overlap the next
// one. Must exceed the worst-case tick: ceil(targets/parallelism) * checkerTimeout
// * len(phases). 0 disables the bound.
func (s *gameCoreServer) WithTickTimeout(value time.Duration) *gameCoreServer {
	if value >= 0 {
		s.tickTimeout = value
	}
	return s
}

func (s *gameCoreServer) WithScoringDebounce(value time.Duration) *gameCoreServer {
	if value >= 0 {
		s.scoringDebounce = value
	}
	return s
}

func (s *gameCoreServer) WithScoringRetryDelay(value time.Duration) *gameCoreServer {
	if value >= 0 {
		s.scoringRetryDelay = value
	}
	return s
}

func (s *gameCoreServer) WithScoringTimeout(value time.Duration) *gameCoreServer {
	if value >= 0 {
		s.scoringTimeout = value
	}
	return s
}

func (s *gameCoreServer) WithWarmupRequired(value bool) *gameCoreServer {
	s.warmupRequired = value
	return s
}

func (s *gameCoreServer) WithWarmupTimeout(value time.Duration) *gameCoreServer {
	if value > 0 {
		s.warmupTimeout = value
	}
	return s
}

func (s *gameCoreServer) WithWarmupMinSuccessRate(value float64) *gameCoreServer {
	if value < 0 {
		value = 0
	}
	if value > 1 {
		value = 1
	}
	s.warmupMinSuccess = value
	return s
}

// WithWarmupPutRetries sets how many extra times a failed warmup put is retried
// before it counts as a failure. 0 disables retries (single attempt).
func (s *gameCoreServer) WithWarmupPutRetries(value int) *gameCoreServer {
	if value >= 0 {
		s.warmupPutRetries = value
	}
	return s
}

// WithWarmupPutRetryDelay sets the backoff slept between warmup put attempts.
func (s *gameCoreServer) WithWarmupPutRetryDelay(value time.Duration) *gameCoreServer {
	if value >= 0 {
		s.warmupPutRetryDelay = value
	}
	return s
}

func (s *gameCoreServer) WithAutoTickOnMatchStart(value bool) *gameCoreServer {
	s.autoTickOnMatchStart = value
	return s
}

func (s *gameCoreServer) matchStatus(ctx context.Context) (apigateway.GameMatchStatus, error) {
	status, err := s.store.MatchStatus(ctx)
	if err != nil {
		return apigateway.GameMatchStatus{}, err
	}
	status = s.applyMatchWindow(status)
	s.warmupMu.RLock()
	warmup := s.lastWarmup
	s.warmupMu.RUnlock()
	if warmup.Status != "" {
		warmupCopy := warmup
		status.Warmup = &warmupCopy
	}
	return status, nil
}

func (s *gameCoreServer) applyMatchWindow(status apigateway.GameMatchStatus) apigateway.GameMatchStatus {
	now := s.now().UTC()
	var startAt *time.Time
	var endAt *time.Time
	if !status.ScheduleConfigured {
		startAt = s.matchStartAt
		endAt = s.matchEndAt
	}
	if parsed := parseOptionalRFC3339(status.ScheduledStartAt); parsed != nil {
		startAt = parsed
	}
	if parsed := parseOptionalRFC3339(status.ScheduledEndAt); parsed != nil {
		endAt = parsed
	}
	if startAt != nil {
		status.ScheduledStartAt = startAt.UTC().Format(time.RFC3339)
	}
	if endAt != nil {
		status.ScheduledEndAt = endAt.UTC().Format(time.RFC3339)
	}
	if status.State == "finished" {
		status.AcceptingSubmissions = false
		if status.EndedAt == "" && endAt != nil && !now.Before(endAt.UTC()) {
			status.EndedAt = endAt.UTC().Format(time.RFC3339)
		}
		if status.StartedAt == "" && startAt != nil && !now.Before(startAt.UTC()) {
			status.StartedAt = startAt.UTC().Format(time.RFC3339)
		}
		return status
	}

	if status.State == "paused" {
		status.AcceptingSubmissions = false
		return status
	}

	if endAt != nil && !now.Before(endAt.UTC()) {
		status.State = "finished"
		status.AcceptingSubmissions = false
		if status.EndedAt == "" {
			status.EndedAt = endAt.UTC().Format(time.RFC3339)
		}
		if status.StartedAt == "" && startAt != nil && !endAt.UTC().Before(startAt.UTC()) {
			status.StartedAt = startAt.UTC().Format(time.RFC3339)
		}
		return status
	}

	if status.State == "running" {
		status.AcceptingSubmissions = true
		return status
	}

	if startAt != nil && !now.Before(startAt.UTC()) {
		status.State = "running"
		status.AcceptingSubmissions = true
		if status.StartedAt == "" {
			status.StartedAt = startAt.UTC().Format(time.RFC3339)
		}
		return status
	}

	status.State = "not_started"
	status.AcceptingSubmissions = false
	return status
}

func (s *gameCoreServer) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /internal/v1/game/status", s.handleGameStatus)
	mux.HandleFunc("GET /internal/v1/game/match", s.handleMatchStatus)
	mux.HandleFunc("POST /internal/v1/game/match/start", s.handleStartMatch)
	mux.HandleFunc("POST /internal/v1/game/match/pause", s.handlePauseMatch)
	mux.HandleFunc("POST /internal/v1/game/match/resume", s.handleResumeMatch)
	mux.HandleFunc("POST /internal/v1/game/match/stop", s.handleStopMatch)
	mux.HandleFunc("PUT /internal/v1/game/match/schedule", s.handleUpdateMatchSchedule)
	mux.HandleFunc("POST /internal/v1/game/ticks/advance", s.handleAdvanceTick)
	mux.HandleFunc("GET /internal/v1/game/checker-runs", s.handleCheckerRuns)
	mux.HandleFunc("GET /internal/v1/game/scoreboard", s.handleScoreboard)
	mux.HandleFunc("GET /internal/v1/game/attacks", s.handleAttackFeed)
	mux.HandleFunc("POST /internal/v1/game/scoring/recompute", s.handleRecomputeScoring)
	mux.HandleFunc("GET /internal/v1/game/scoring/audit", s.handleAuditScoring)
	mux.HandleFunc("GET /internal/v1/game/scheduler", s.handleSchedulerStatus)
	mux.HandleFunc("GET /internal/v1/game/scheduler/events", s.handleSchedulerEvents)
	mux.HandleFunc("POST /internal/v1/game/scheduler/start", s.handleStartScheduler)
	mux.HandleFunc("POST /internal/v1/game/scheduler/stop", s.handleStopScheduler)
	mux.HandleFunc("PUT /internal/v1/game/scheduler/interval", s.handleUpdateScheduler)
	mux.HandleFunc("POST /internal/v1/flags/submit", s.handleSubmitFlags)
	mux.HandleFunc("GET /internal/v1/game/flag/format", s.handleGetFlagFormat)
	mux.HandleFunc("POST /internal/v1/game/flag/format/refresh", s.handleRefreshFlagFormat)
	mux.HandleFunc("POST /internal/v1/game/warmup", s.handleRunWarmup)
	mux.HandleFunc("GET /internal/v1/game/warmup", s.handleGetWarmup)
}

func (s *gameCoreServer) handleGameStatus(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	status, err := s.store.GameStatus(r.Context())
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "Internal state unavailable", err.Error())
		return
	}
	if s.scheduler != nil {
		schedulerStatus := s.scheduler.Status()
		status.Scheduler = &schedulerStatus
	}
	matchStatus, err := s.matchStatus(r.Context())
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "Internal state unavailable", err.Error())
		return
	}
	status.Match = &matchStatus
	writeData(w, http.StatusOK, status)
}

func (s *gameCoreServer) handleAdvanceTick(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	tick, err := s.advanceTick(r.Context())
	if err != nil {
		statusCode := http.StatusInternalServerError
		switch {
		case err == errTickInProgress:
			statusCode = http.StatusConflict
		case errors.Is(err, errContestNotStarted), errors.Is(err, errContestOver), errors.Is(err, errContestPaused):
			statusCode = http.StatusBadRequest
		}
		writeProblem(w, statusCode, "Tick advance failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, tick)
}

func (s *gameCoreServer) handleMatchStatus(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	status, err := s.matchStatus(r.Context())
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "Internal state unavailable", err.Error())
		return
	}
	writeData(w, http.StatusOK, status)
}

func (s *gameCoreServer) handleRunWarmup(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	result, err := s.runWarmup(r.Context())
	statusCode := http.StatusOK
	switch {
	case err != nil && result.Status == "failed":
		statusCode = http.StatusUnprocessableEntity
	case err != nil:
		statusCode = http.StatusInternalServerError
	case result.Status != "passed":
		statusCode = http.StatusUnprocessableEntity
	}
	writeData(w, statusCode, result)
}

func (s *gameCoreServer) handleGetWarmup(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	s.warmupMu.RLock()
	current := s.lastWarmup
	s.warmupMu.RUnlock()
	writeData(w, http.StatusOK, current)
}

func (s *gameCoreServer) handleStartMatch(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	current, err := s.matchStatus(r.Context())
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "Internal state unavailable", err.Error())
		return
	}
	if current.State == "running" {
		writeData(w, http.StatusOK, current)
		return
	}
	if current.State == "finished" {
		writeProblem(w, http.StatusConflict, "Request rejected", errContestOver.Error())
		return
	}

	if s.warmupRequired {
		warmup, warmupErr := s.runWarmup(r.Context())
		if warmupErr != nil {
			writeProblem(w, http.StatusInternalServerError, "Warmup failed", warmupErr.Error())
			return
		}
		if warmup.Status != "passed" {
			writeData(w, http.StatusConflict, warmup)
			return
		}
	}

	status, err := s.store.StartMatch(r.Context(), s.now())
	if err != nil {
		statusCode := http.StatusInternalServerError
		if errors.Is(err, errContestOver) {
			statusCode = http.StatusConflict
		}
		writeProblem(w, statusCode, "Match start failed", err.Error())
		return
	}
	status = s.applyMatchWindow(status)

	// Open network for warm-deployed services (closed while match was not_started).
	if s.accessReconcile != nil {
		if recErr := s.accessReconcile.ReconcileAccess(r.Context()); recErr != nil {
			log.Printf("game-core: access reconcile on match start failed: %v", recErr)
		}
	}

	// Best-effort: fire the first tick immediately so a scoreable tick-1 flag
	// is in every container by the time this response returns. The scheduler's
	// 5-minute interval would otherwise leave a window of zero scoreable
	// flags between match start and the first scheduled tick. We use a
	// detached context so a client disconnect does not abort the tick, and
	// we log-and-continue on failure: the match is already "running", and
	// the scheduler (or a manual POST /ticks/advance) will produce tick 2
	// on the configured cadence.
	if s.autoTickOnMatchStart {
		tickCtx, cancel := context.WithCancel(context.Background())
		if _, tickErr := s.advanceTick(tickCtx); tickErr != nil {
			log.Printf("game-core: auto-tick on match start failed: %v", tickErr)
		}
		cancel()
	}

	writeData(w, http.StatusOK, status)
}

func (s *gameCoreServer) handlePauseMatch(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	current, err := s.matchStatus(r.Context())
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "Internal state unavailable", err.Error())
		return
	}
	if current.State == "finished" {
		writeProblem(w, http.StatusConflict, "Request rejected", errContestOver.Error())
		return
	}
	if current.State != "paused" {
		status, err := s.store.PauseMatch(r.Context(), s.now())
		if err != nil {
			writeProblem(w, http.StatusInternalServerError, "Match pause failed", err.Error())
			return
		}
		current = s.applyMatchWindow(status)
	}
	if s.scheduler != nil && s.scheduler.Status().State == "running" {
		_, _ = s.scheduler.Stop()
	}
	writeData(w, http.StatusOK, current)
}

func (s *gameCoreServer) handleResumeMatch(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	current, err := s.matchStatus(r.Context())
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "Internal state unavailable", err.Error())
		return
	}
	if current.State == "finished" {
		writeProblem(w, http.StatusConflict, "Request rejected", errContestOver.Error())
		return
	}
	if current.State == "paused" {
		status, err := s.store.ResumeMatch(r.Context(), s.now())
		if err != nil {
			writeProblem(w, http.StatusInternalServerError, "Match resume failed", err.Error())
			return
		}
		current = s.applyMatchWindow(status)
	}
	writeData(w, http.StatusOK, current)
}

func (s *gameCoreServer) handleStopMatch(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	current, err := s.matchStatus(r.Context())
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "Internal state unavailable", err.Error())
		return
	}
	if current.State != "finished" {
		status, err := s.store.StopMatch(r.Context(), s.now())
		if err != nil {
			writeProblem(w, http.StatusInternalServerError, "Match stop failed", err.Error())
			return
		}
		current = s.applyMatchWindow(status)
	}
	if s.scheduler != nil && s.scheduler.Status().State == "running" {
		_, _ = s.scheduler.Stop()
	}
	writeData(w, http.StatusOK, current)
}

func (s *gameCoreServer) handleUpdateMatchSchedule(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}

	var req apigateway.UpdateMatchScheduleRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		writeProblem(w, http.StatusBadRequest, "Invalid request", "could not parse match schedule payload.")
		return
	}

	startAt, err := parseScheduleValue(req.ScheduledStartAt)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "Invalid request", "scheduled_start_at must be RFC3339.")
		return
	}
	endAt, err := parseScheduleValue(req.ScheduledEndAt)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "Invalid request", "scheduled_end_at must be RFC3339.")
		return
	}
	if startAt != nil && endAt != nil && endAt.Before(*startAt) {
		writeProblem(w, http.StatusBadRequest, "Invalid request", "scheduled_end_at must not be earlier than scheduled_start_at.")
		return
	}

	status, err := s.store.UpdateMatchSchedule(r.Context(), startAt, endAt)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "Match schedule update failed", err.Error())
		return
	}
	status = s.applyMatchWindow(status)
	writeData(w, http.StatusOK, status)
}

func (s *gameCoreServer) handleCheckerRuns(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	query := parseCheckerRunQuery(r)
	runs, err := s.store.ListCheckerRuns(r.Context(), query)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "Internal state unavailable", err.Error())
		return
	}
	writeData(w, http.StatusOK, runs)
}

func (s *gameCoreServer) handleScoreboard(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	rows, err := s.store.ListScoreboard(r.Context())
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "Scoreboard unavailable", err.Error())
		return
	}
	writeData(w, http.StatusOK, rows)
}

func (s *gameCoreServer) handleAttackFeed(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	query := apigateway.AttackFeedQuery{
		Limit:    parsePositiveQueryInt(r, "limit", 12, 200),
		Offset:   parseNonNegativeQueryInt(r, "offset"),
		Attacker: strings.TrimSpace(r.URL.Query().Get("attacker")),
		Victim:   strings.TrimSpace(r.URL.Query().Get("victim")),
		Service:  strings.TrimSpace(r.URL.Query().Get("service")),
		TickFrom: parseNonNegativeQueryInt(r, "tick_from"),
		TickTo:   parseNonNegativeQueryInt(r, "tick_to"),
	}
	rows, err := s.store.ListAttackFeed(r.Context(), query)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "Attack feed unavailable", err.Error())
		return
	}
	writeData(w, http.StatusOK, rows)
}

func (s *gameCoreServer) handleRecomputeScoring(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	rows, err := s.store.RecomputeScoreboard(r.Context())
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "Scoring recompute failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, rows)
}

func (s *gameCoreServer) handleAuditScoring(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	report, err := s.store.AuditScoreboard(r.Context())
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "Scoring audit failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, report)
}

func (s *gameCoreServer) handleSchedulerStatus(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	status := s.scheduler.Status()
	writeData(w, http.StatusOK, status)
}

func (s *gameCoreServer) handleStartScheduler(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	status, err := s.scheduler.Start()
	if err != nil {
		statusCode := http.StatusInternalServerError
		switch {
		case errors.Is(err, errContestNotStarted), errors.Is(err, errContestOver), errors.Is(err, errContestPaused):
			statusCode = http.StatusBadRequest
		}
		writeProblem(w, statusCode, "Scheduler start failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, status)
}

func (s *gameCoreServer) handleSchedulerEvents(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	query := parseSchedulerEventQuery(r)
	events, err := s.scheduler.Events(r.Context(), query)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "Scheduler events unavailable", err.Error())
		return
	}
	writeData(w, http.StatusOK, events)
}

func (s *gameCoreServer) handleStopScheduler(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	status, err := s.scheduler.Stop()
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "Scheduler stop failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, status)
}

func (s *gameCoreServer) handleUpdateScheduler(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}

	var req apigateway.UpdateSchedulerRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		writeProblem(w, http.StatusBadRequest, "Invalid request", "could not parse interval payload.")
		return
	}

	if req.IntervalSeconds < 1 {
		writeProblem(w, http.StatusBadRequest, "Invalid request", "interval must be at least 1 second.")
		return
	}

	status, err := s.scheduler.Update(time.Duration(req.IntervalSeconds) * time.Second)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "Scheduler update failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, status)
}

func (s *gameCoreServer) handleSubmitFlags(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	var request apigateway.GameSubmitFlagsRequest
	if err := httpapi.DecodeJSON(r, &request); err != nil || request.TeamID <= 0 || len(request.Flags) == 0 {
		writeProblem(w, http.StatusBadRequest, "Invalid request", "flag submission request is invalid.")
		return
	}
	results, err := s.submitFlags(r.Context(), request.TeamID, request.Flags)
	if err != nil {
		statusCode := http.StatusInternalServerError
		switch {
		case errors.Is(err, errContestNotStarted), errors.Is(err, errContestOver), errors.Is(err, errContestPaused):
			statusCode = http.StatusBadRequest
		}
		writeProblem(w, statusCode, "Flag submission failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, toSubmissionVerdictAliases(results))
}

func (s *gameCoreServer) handleGetFlagFormat(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	writeData(w, http.StatusOK, apigateway.FlagFormatStatus{
		Format:    s.currentFlagFormat(),
		UpdatedAt: s.now().UTC().Format(time.RFC3339),
	})
}

func (s *gameCoreServer) handleRefreshFlagFormat(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	var req apigateway.UpdateFlagFormatRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		writeProblem(w, http.StatusBadRequest, "Invalid request", "could not parse flag format payload.")
		return
	}
	if strings.TrimSpace(req.Prefix) == "" {
		writeProblem(w, http.StatusBadRequest, "Invalid request", "prefix must not be empty.")
		return
	}
	active := s.reloadFlagFormat(req.Prefix)
	writeData(w, http.StatusOK, apigateway.FlagFormatStatus{
		Format:    active,
		UpdatedAt: s.now().UTC().Format(time.RFC3339),
	})
}

func (s *gameCoreServer) requireAdminAuth(w http.ResponseWriter, r *http.Request) bool {
	token, ok := httpapi.BearerToken(r)
	if !ok || subtle.ConstantTimeCompare([]byte(token), []byte(s.adminToken)) != 1 {
		writeProblem(w, http.StatusForbidden, "Forbidden", "please authenticate before accessing game-core endpoints.")
		return false
	}
	return true
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

func parseCheckerRunQuery(r *http.Request) apigateway.GameCheckerRunQuery {
	return apigateway.GameCheckerRunQuery{
		Limit:       parsePositiveQueryInt(r, "limit", 25, 200),
		Offset:      parseNonNegativeQueryInt(r, "offset"),
		TickID:      parsePositiveQueryInt(r, "tick_id", 0, 0),
		TeamID:      parsePositiveQueryInt(r, "team_id", 0, 0),
		ChallengeID: parsePositiveQueryInt(r, "challenge_id", 0, 0),
		Phase:       strings.ToLower(strings.TrimSpace(r.URL.Query().Get("phase"))),
		Status:      strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status"))),
	}
}

func parseSchedulerEventQuery(r *http.Request) apigateway.GameSchedulerEventQuery {
	return apigateway.GameSchedulerEventQuery{
		Limit:     parsePositiveQueryInt(r, "limit", 25, 200),
		Offset:    parseNonNegativeQueryInt(r, "offset"),
		EventType: strings.ToLower(strings.TrimSpace(r.URL.Query().Get("event_type"))),
		Source:    strings.ToLower(strings.TrimSpace(r.URL.Query().Get("source"))),
		State:     strings.ToLower(strings.TrimSpace(r.URL.Query().Get("state"))),
	}
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

func parseOptionalRFC3339(raw string) *time.Time {
	parsed, err := parseScheduleValue(raw)
	if err != nil {
		return nil
	}
	return parsed
}

func parseScheduleValue(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, err
	}
	utc := parsed.UTC()
	return &utc, nil
}

func (s *gameCoreServer) advanceTick(ctx context.Context) (apigateway.GameTickStatus, error) {
	match, err := s.matchStatus(ctx)
	if err != nil {
		return apigateway.GameTickStatus{}, err
	}
	switch match.State {
	case "finished":
		return apigateway.GameTickStatus{}, errContestOver
	case "paused":
		return apigateway.GameTickStatus{}, errContestPaused
	case "running":
	default:
		return apigateway.GameTickStatus{}, errContestNotStarted
	}

	startedAt := s.now().UTC()
	tick, err := s.store.StartNextTick(ctx, startedAt)
	if err != nil {
		return apigateway.GameTickStatus{}, err
	}

	targets, err := s.store.ListCheckerTargets(ctx)
	if err != nil {
		tick.Status = "failed"
		tick.CompletedAt = s.now().UTC().Format(time.RFC3339)
		tick.Message = err.Error()
		_, _ = s.store.CompleteTick(ctx, tick)
		return apigateway.GameTickStatus{}, err
	}
	// Rotate target order by tick id so a hard timeout does not permanently
	// prefer lower team/challenge ids from the SQL ORDER BY.
	targets = rotateCheckerTargets(targets, tick.ID)

	runCtx := ctx
	if s.tickTimeout > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, s.tickTimeout)
		defer cancel()
	}
	results, err := s.runCheckerTargets(runCtx, tick.ID, targets)
	if err != nil {
		tick.Status = "failed"
		tick.CompletedAt = s.now().UTC().Format(time.RFC3339)
		tick.Message = err.Error()
		_, _ = s.store.CompleteTick(ctx, tick)
		return apigateway.GameTickStatus{}, err
	}
	coveredTargets := 0
	for _, result := range results {
		tick.TotalCheckerRuns += result.total
		tick.SuccessfulCheckerRuns += result.success
		tick.SkippedCheckerRuns += result.skipped
		tick.FailedCheckerRuns += result.failed
		if result.total > 0 {
			coveredTargets++
		}
	}

	tick.Status = "completed"
	tick.CompletedAt = s.now().UTC().Format(time.RFC3339)
	if s.tickTimeout > 0 && errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		tick.Message = fmt.Sprintf(
			"tick timed out after covering %d of %d targets (%d phases configured)",
			coveredTargets,
			len(targets),
			len(s.checkerPhases),
		)
	} else {
		tick.Message = fmt.Sprintf(
			"advanced tick across %d targets and %d phases",
			len(targets),
			len(s.checkerPhases),
		)
	}
	completed, err := s.store.CompleteTick(ctx, tick)
	if err != nil {
		return apigateway.GameTickStatus{}, err
	}
	if _, err := s.store.RecomputeScoreboard(ctx); err != nil {
		return apigateway.GameTickStatus{}, err
	}
	// Best-effort: open network (+ WG converge via controller) for challenges whose
	// play_from_tick was this tick (post-maintenance resume). Failures must not
	// fail the tick — access can be re-applied by admin reconcile.
	if s.accessReconcile != nil {
		if recErr := s.accessReconcile.ReconcileAccess(ctx); recErr != nil {
			log.Printf("game-core: access/WG reconcile after tick %d failed: %v", completed.ID, recErr)
		}
	}
	return completed, nil
}

// runWarmup executes a pre-match warmup: it lists every checker target and runs
// only the "put" phase of the checker against each, with the same parallelism
// and per-call timeout as a real tick. Unlike advanceTick it does NOT call
// StartNextTick / CompleteTick, does NOT persist the planted flag to
// issued_flags, and does NOT recompute the scoreboard — the goal is purely to
// verify that every (team, challenge) target can accept a flag put before the
// match is allowed to start. The last result (pass/fail per target) is held
// in memory on the server so a subsequent handleStartMatch can gate the
// transition into "running" on a passing warmup.
func (s *gameCoreServer) runWarmup(ctx context.Context) (apigateway.GameWarmupResult, error) {
	s.warmupMu.Lock()
	if s.warmupInFlight {
		current := s.lastWarmup
		s.warmupMu.Unlock()
		return current, nil
	}
	s.warmupInFlight = true
	s.warmupMu.Unlock()
	defer func() {
		s.warmupMu.Lock()
		s.warmupInFlight = false
		s.warmupMu.Unlock()
	}()

	startedAt := s.now().UTC()
	result := apigateway.GameWarmupResult{
		Status:    "running",
		StartedAt: startedAt.Format(time.RFC3339),
	}
	s.warmupMu.Lock()
	s.lastWarmup = result
	s.warmupMu.Unlock()

	runCtx := ctx
	if s.warmupTimeout > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, s.warmupTimeout)
		defer cancel()
	}

	targets, err := s.store.ListCheckerTargets(runCtx)
	if err != nil {
		failed := s.recordWarmupFailure(result, fmt.Errorf("failed to list checker targets: %w", err))
		return failed, err
	}

	result.TotalTargets = len(targets)
	if result.TotalTargets == 0 {
		completedAt := s.now().UTC()
		result.CompletedAt = completedAt.Format(time.RFC3339)
		result.Status = "passed"
		result.SuccessRate = 1.0
		result.Message = "no checker targets configured; warmup passes vacuously"
		s.warmupMu.Lock()
		s.lastWarmup = result
		s.warmupMu.Unlock()
		return result, nil
	}

	parallelism := s.checkerParallelism
	if parallelism <= 0 {
		parallelism = 1
	}
	if parallelism > result.TotalTargets {
		parallelism = result.TotalTargets
	}

	type warmupJob struct{ index int }
	jobs := make(chan warmupJob)
	var failures []apigateway.GameWarmupFailure
	var failuresMu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < parallelism; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				if runCtx.Err() != nil {
					continue
				}
				target := targets[j.index]
				if putErr := s.runWarmupForTarget(runCtx, target); putErr != nil {
					failuresMu.Lock()
					failures = append(failures, apigateway.GameWarmupFailure{
						TeamID:        target.TeamID,
						ChallengeID:   target.ChallengeID,
						ChallengeName: target.ChallengeName,
						Error:         putErr.Error(),
					})
					failuresMu.Unlock()
				}
			}
		}()
	}

producer:
	for i := range targets {
		select {
		case <-runCtx.Done():
			break producer
		case jobs <- warmupJob{index: i}:
		}
	}
	close(jobs)
	wg.Wait()

	completedAt := s.now().UTC()
	result.CompletedAt = completedAt.Format(time.RFC3339)
	result.PutFailed = len(failures)
	result.PutSuccess = result.TotalTargets - result.PutFailed
	if result.TotalTargets > 0 {
		result.SuccessRate = float64(result.PutSuccess) / float64(result.TotalTargets)
	} else {
		result.SuccessRate = 1.0
	}
	if runCtx.Err() != nil && len(failures) == 0 {
		result.Failures = []apigateway.GameWarmupFailure{{
			Error: fmt.Sprintf("warmup aborted before completion: %v", runCtx.Err()),
		}}
		result.PutFailed = result.TotalTargets
		result.PutSuccess = 0
		result.SuccessRate = 0
	} else {
		result.Failures = failures
	}

	if result.SuccessRate >= s.warmupMinSuccess {
		result.Status = "passed"
	} else {
		result.Status = "failed"
		if result.Message == "" && result.PutFailed > 0 {
			result.Message = fmt.Sprintf("%d of %d targets failed put phase (required success rate: %.0f%%)",
				result.PutFailed, result.TotalTargets, s.warmupMinSuccess*100)
		}
	}

	s.warmupMu.Lock()
	s.lastWarmup = result
	s.warmupMu.Unlock()
	return result, nil
}

func (s *gameCoreServer) runWarmupForTarget(ctx context.Context, target checkerTarget) error {
	// Mint a real flag value with tickID=0 so the checker behaviour is identical
	// to a real tick's put phase. The flag will not appear in issued_flags (we
	// skip persistence), and tickID=0 will never satisfy the scorer check
	// (currentTick > claims.ExpiresTick) so attackers cannot exploit the warmup
	// flag for scoring.
	flagValue := s.issueFlag(target.TeamID, target.ChallengeID, 0, 0)

	// At match start a slow service may not have bound its checker gate yet, so
	// retry the put a few times with backoff before counting it as a failure.
	// Each attempt is a real checker execution against the target; only an
	// exhausted retry budget fails the target.
	attempts := s.warmupPutRetries + 1
	if attempts < 1 {
		attempts = 1
	}
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(s.warmupPutRetryDelay):
			}
		}
		if lastErr = s.attemptWarmupPut(ctx, target, flagValue); lastErr == nil {
			return nil
		}
	}
	return lastErr
}

func (s *gameCoreServer) attemptWarmupPut(ctx context.Context, target checkerTarget, flagValue string) error {
	result, execErr := s.checker.Execute(ctx, apigateway.CheckerExecutionRequest{
		ChallengeID:    target.ChallengeID,
		TeamID:         target.TeamID,
		TeamName:       target.TeamName,
		ChallengeName:  target.ChallengeName,
		CheckerImage:   target.CheckerImage,
		Phase:          "put",
		Target:         target.Target,
		TargetHost:     target.TargetHost,
		TargetIP:       target.TargetIP,
		TargetPort:     target.TargetPort,
		TickID:         0,
		Flag:           flagValue,
		CheckerToken:   target.CheckerToken,
		TimeoutSeconds: s.checkerTimeout,
	})
	if execErr != nil {
		return fmt.Errorf("checker execution failed: %w", execErr)
	}

	status := normalizedCheckerRunStatus(result.Status)
	if status != "success" {
		return fmt.Errorf("put phase did not succeed: status=%s message=%s",
			status, strings.TrimSpace(result.Message))
	}
	return nil
}

func (s *gameCoreServer) recordWarmupFailure(base apigateway.GameWarmupResult, runErr error) apigateway.GameWarmupResult {
	completedAt := s.now().UTC()
	result := base
	result.CompletedAt = completedAt.Format(time.RFC3339)
	result.Status = "failed"
	result.Message = strings.TrimSpace(runErr.Error())
	if result.TotalTargets == 0 {
		result.SuccessRate = 0
	}
	s.warmupMu.Lock()
	s.lastWarmup = result
	s.warmupMu.Unlock()
	return result
}

type checkerTargetTickResult struct {
	total   int
	success int
	skipped int
	failed  int
}

// rotateCheckerTargets rotates the target list by tickID so hard timeouts do
// not always starve the same high team/challenge ids from SQL ORDER BY.
func rotateCheckerTargets(targets []checkerTarget, tickID int) []checkerTarget {
	if len(targets) <= 1 {
		return targets
	}
	offset := tickID % len(targets)
	if offset < 0 {
		offset = -offset
	}
	if offset == 0 {
		return targets
	}
	rotated := make([]checkerTarget, 0, len(targets))
	rotated = append(rotated, targets[offset:]...)
	rotated = append(rotated, targets[:offset]...)
	return rotated
}

func (s *gameCoreServer) runCheckerTargets(ctx context.Context, tickID int, targets []checkerTarget) ([]checkerTargetTickResult, error) {
	parallelism := s.checkerParallelism
	if parallelism <= 0 {
		parallelism = 1
	}
	if parallelism > len(targets) && len(targets) > 0 {
		parallelism = len(targets)
	}

	results := make([]checkerTargetTickResult, len(targets))
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	jobs := make(chan int)
	var firstErr error
	var errMu sync.Mutex

	for range parallelism {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				if workerCtx.Err() != nil {
					continue
				}
				result, err := s.runCheckerTarget(workerCtx, tickID, targets[index])
				results[index] = result
				if err != nil {
					errMu.Lock()
					if firstErr == nil {
						firstErr = err
						cancel()
					}
					errMu.Unlock()
				}
			}
		}()
	}

	for index := range targets {
		if workerCtx.Err() != nil {
			break
		}
		jobs <- index
	}
	close(jobs)
	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}
	return results, nil
}

func (s *gameCoreServer) runCheckerTarget(ctx context.Context, tickID int, target checkerTarget) (checkerTargetTickResult, error) {
	var counts checkerTargetTickResult
	flagValue := s.issueFlag(target.TeamID, target.ChallengeID, tickID, tickID)

	// All phases for this target run inside a single checker container, which
	// chains each successful phase's output into the next and halts on the first
	// failure. This collapses the per-phase container cold-starts of a tick.
	batch, execErr := s.checker.ExecuteBatch(ctx, apigateway.CheckerBatchExecutionRequest{
		ChallengeID:    target.ChallengeID,
		TeamID:         target.TeamID,
		TeamName:       target.TeamName,
		ChallengeName:  target.ChallengeName,
		CheckerImage:   target.CheckerImage,
		Phases:         s.checkerPhases,
		Target:         target.Target,
		TargetHost:     target.TargetHost,
		TargetIP:       target.TargetIP,
		TargetPort:     target.TargetPort,
		TickID:         tickID,
		Flag:           flagValue,
		CheckerToken:   target.CheckerToken,
		TimeoutSeconds: s.checkerTimeout,
	})

	phaseResults := make(map[string]apigateway.CheckerPhaseResult, len(batch.Phases))
	for _, pr := range batch.Phases {
		phaseResults[pr.Phase] = pr
	}

	halted := false
	for _, phase := range s.checkerPhases {
		runTime := s.now().UTC()
		run := checkerRunRecord{
			TickID:        tickID,
			TeamID:        target.TeamID,
			TeamName:      target.TeamName,
			ChallengeID:   target.ChallengeID,
			ChallengeName: target.ChallengeName,
			Phase:         phase,
			Target:        target.Target,
			CheckerImage:  target.CheckerImage,
			CheckedAt:     runTime,
			ExitCode:      -1,
		}

		switch {
		case halted:
			run.Status = "skipped"
			run.Message = "phase skipped after previous checker failure"
		case execErr != nil:
			// Batch transport/infra failure (docker, network, timeout): no phase
			// ran. Attribute the failure to the first phase and skip the rest.
			run.Status = "failed"
			run.Message = execErr.Error()
			halted = true
		default:
			result, ok := phaseResults[phase]
			if !ok {
				// Phase absent: a prior phase failed and the batch halted before it.
				run.Status = "skipped"
				run.Message = "phase skipped after previous checker failure"
				halted = true
				break
			}
			run.Status = normalizedCheckerRunStatus(result.Status)
			run.ExitCode = result.ExitCode
			run.Message = strings.TrimSpace(result.Message)
			run.Output = strings.TrimSpace(result.Output)
			run.ReportedServiceState = apigateway.NormalizeServiceStateStatus(result.ServiceState)
			run.ReportedStateMessage = strings.TrimSpace(result.StateMessage)
			if parsed, err := time.Parse(time.RFC3339, result.CheckedAt); err == nil {
				run.CheckedAt = parsed.UTC()
			}
			if run.Status == "failed" {
				halted = true
			}
		}

		if _, err := s.store.RecordCheckerRun(ctx, run); err != nil {
			return counts, fmt.Errorf("checker run persistence failed: %w", err)
		}

		if phase == "put" && run.Status == "success" {
			flag := issuedFlagRecord{
				Flag:          flagValue,
				OwnerTeamID:   target.TeamID,
				OwnerTeamName: target.TeamName,
				ChallengeID:   target.ChallengeID,
				ChallengeName: target.ChallengeName,
				IssuedTick:    tickID,
				ExpiresTick:   tickID,
				CreatedAt:     run.CheckedAt,
			}
			if err := s.store.IssueFlag(ctx, flag); err != nil {
				return counts, fmt.Errorf("flag issuance persistence failed: %w", err)
			}
		}

		counts.total++
		switch run.Status {
		case "success":
			counts.success++
		case "skipped":
			counts.skipped++
		default:
			counts.failed++
		}
	}
	return counts, nil
}

func (s *gameCoreServer) submitFlags(ctx context.Context, teamID int, flags []string) ([]submissionVerdictAlias, error) {
	match, err := s.matchStatus(ctx)
	if err != nil {
		return nil, err
	}
	switch match.State {
	case "finished":
		return nil, errContestOver
	case "paused":
		return nil, errContestPaused
	case "running":
	default:
		return nil, errContestNotStarted
	}

	status, err := s.store.GameStatus(ctx)
	if err != nil {
		return nil, err
	}

	currentTick := 0
	attackerName, err := s.store.LookupTeamName(ctx, teamID)
	if err != nil {
		attackerName = fmt.Sprintf("Team %d", teamID)
	}
	if status.CurrentTick != nil {
		currentTick = status.CurrentTick.ID
	}

	results := make([]submissionVerdictAlias, 0, len(flags))
	seen := make(map[string]struct{}, len(flags))
	acceptedAny := false
	for _, flagValue := range flags {
		trimmed := strings.TrimSpace(flagValue)
		if trimmed == "" {
			results = append(results, submissionVerdictAlias{Flag: flagValue, Status: "invalid", Detail: "flag is wrong or expired."})
			continue
		}
		if _, ok := seen[trimmed]; ok {
			results = append(results, submissionVerdictAlias{Flag: trimmed, Status: "duplicate", Detail: "flag already submitted."})
			continue
		}
		seen[trimmed] = struct{}{}

		claims, ok := s.parseFlag(trimmed)
		if !ok || currentTick == 0 || currentTick > claims.ExpiresTick || claims.OwnerTeamID == teamID {
			results = append(results, submissionVerdictAlias{Flag: trimmed, Status: "invalid", Detail: "flag is wrong or expired."})
			continue
		}

		issued, err := s.store.LookupIssuedFlag(ctx, trimmed)
		if err != nil || issued.OwnerTeamID != claims.OwnerTeamID || issued.ChallengeID != claims.ChallengeID || issued.IssuedTick != claims.IssuedTick || issued.ExpiresTick != claims.ExpiresTick {
			results = append(results, submissionVerdictAlias{Flag: trimmed, Status: "invalid", Detail: "flag is wrong or expired."})
			continue
		}

		inMaintenance, err := s.store.IsChallengeInMaintenance(ctx, issued.ChallengeID)
		if err != nil {
			return nil, err
		}
		if inMaintenance {
			results = append(results, submissionVerdictAlias{Flag: trimmed, Status: "invalid", Detail: "challenge is under maintenance."})
			continue
		}

		accepted, err := s.store.AcceptFlagSubmission(ctx, acceptedFlagSubmission{
			Flag:           trimmed,
			SubmittingTeam: teamID,
			AttackerName:   attackerName,
			VictimName:     issued.OwnerTeamName,
			ChallengeName:  issued.ChallengeName,
			SubmissionTick: currentTick,
			ExpiresTick:    claims.ExpiresTick,
			SubmittedAt:    s.now().UTC(),
		})
		if err != nil {
			if errors.Is(err, errFlagNoLongerValid) {
				results = append(results, submissionVerdictAlias{Flag: trimmed, Status: "invalid", Detail: "flag is wrong or expired."})
				continue
			}
			return nil, err
		}
		if !accepted {
			results = append(results, submissionVerdictAlias{Flag: trimmed, Status: "duplicate", Detail: "flag already submitted."})
			continue
		}

		results = append(results, submissionVerdictAlias{Flag: trimmed, Status: "accepted", Detail: "flag is correct."})
		acceptedAny = true
	}
	if acceptedAny {
		s.scheduleScoreboardRecompute()
	}
	return results, nil
}

func (s *gameCoreServer) scheduleScoreboardRecompute() {
	s.scoringMu.Lock()
	defer s.scoringMu.Unlock()

	s.scoringStatus.Pending = true
	if s.scoringStatus.InFlight {
		return
	}
	if s.scoringTimer != nil {
		s.scoringTimer.Stop()
	}
	s.scoringTimer = time.AfterFunc(nonNegativeDuration(s.scoringDebounce), s.runScheduledScoreboardRecompute)
}

func (s *gameCoreServer) runScheduledScoreboardRecompute() {
	s.scoringMu.Lock()
	if s.scoringStatus.InFlight {
		s.scoringStatus.Pending = true
		s.scoringMu.Unlock()
		return
	}
	s.scoringStatus.Pending = false
	s.scoringStatus.InFlight = true
	s.scoringTimer = nil
	s.scoringMu.Unlock()

	ctx := context.Background()
	cancel := func() {}
	if s.scoringTimeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, s.scoringTimeout)
	}
	defer cancel()

	_, err := s.store.RecomputeScoreboard(ctx)

	s.scoringMu.Lock()
	defer s.scoringMu.Unlock()
	s.scoringStatus.InFlight = false
	if err != nil {
		s.scoringStatus.Pending = true
		s.scoringStatus.LastFailureAt = s.now().UTC()
		s.scoringStatus.LastError = err.Error()
		s.scoringStatus.FailureCount++
		log.Printf("scoreboard recompute failed after submission: %v", err)
		if s.scoringTimer != nil {
			s.scoringTimer.Stop()
		}
		s.scoringTimer = time.AfterFunc(nonNegativeDuration(s.scoringRetryDelay), s.runScheduledScoreboardRecompute)
		return
	}

	s.scoringStatus.LastSuccessAt = s.now().UTC()
	s.scoringStatus.LastError = ""
	if s.scoringStatus.Pending {
		if s.scoringTimer != nil {
			s.scoringTimer.Stop()
		}
		s.scoringTimer = time.AfterFunc(nonNegativeDuration(s.scoringDebounce), s.runScheduledScoreboardRecompute)
	}
}

func (s *gameCoreServer) snapshotScoringStatus() scoringRecomputeStatus {
	s.scoringMu.Lock()
	defer s.scoringMu.Unlock()
	return s.scoringStatus
}

func nonNegativeDuration(value time.Duration) time.Duration {
	if value < 0 {
		return 0
	}
	return value
}

func normalizedCheckerRunStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "success", "passed", "ok":
		return "success"
	case "skipped":
		return "skipped"
	default:
		return "failed"
	}
}

type submissionVerdictAlias struct {
	Flag   string `json:"flag"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

func toSubmissionVerdictAliases(values []submissionVerdictAlias) []apigateway.SubmissionVerdictAlias {
	result := make([]apigateway.SubmissionVerdictAlias, 0, len(values))
	for _, value := range values {
		result = append(result, apigateway.SubmissionVerdictAlias{Flag: value.Flag, Status: value.Status, Detail: value.Detail})
	}
	return result
}
