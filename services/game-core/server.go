package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"adplatform/internal/platform/httpapi"
	"adplatform/internal/services/apigateway"
)

type gameCoreServer struct {
	adminToken     string
	store          gameStore
	checker        checkerClient
	flags          flagCodec
	scheduler      gameScheduler
	checkerPhases  []string
	checkerTimeout int
	matchStartAt   *time.Time
	matchEndAt     *time.Time
	now            func() time.Time
}

func newGameCoreServer(adminToken string, store gameStore, checker checkerClient, flags flagCodec, scheduler gameScheduler, checkerPhases []string, checkerTimeout int) *gameCoreServer {
	if scheduler == nil {
		scheduler = noopGameScheduler{}
	}
	return &gameCoreServer{
		adminToken:     strings.TrimSpace(adminToken),
		store:          store,
		checker:        checker,
		flags:          flags,
		scheduler:      scheduler,
		checkerPhases:  checkerPhases,
		checkerTimeout: checkerTimeout,
		now:            time.Now,
	}
}

func (s *gameCoreServer) matchStatus(ctx context.Context) (apigateway.GameMatchStatus, error) {
	status, err := s.store.MatchStatus(ctx)
	if err != nil {
		return apigateway.GameMatchStatus{}, err
	}
	return s.applyMatchWindow(status), nil
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
	mux.HandleFunc("POST /internal/v1/game/match/stop", s.handleStopMatch)
	mux.HandleFunc("PUT /internal/v1/game/match/schedule", s.handleUpdateMatchSchedule)
	mux.HandleFunc("POST /internal/v1/game/ticks/advance", s.handleAdvanceTick)
	mux.HandleFunc("GET /internal/v1/game/checker-runs", s.handleCheckerRuns)
	mux.HandleFunc("GET /internal/v1/game/scoreboard", s.handleScoreboard)
	mux.HandleFunc("GET /internal/v1/game/attacks", s.handleAttackFeed)
	mux.HandleFunc("POST /internal/v1/game/scoring/recompute", s.handleRecomputeScoring)
	mux.HandleFunc("GET /internal/v1/game/scheduler", s.handleSchedulerStatus)
	mux.HandleFunc("GET /internal/v1/game/scheduler/events", s.handleSchedulerEvents)
	mux.HandleFunc("POST /internal/v1/game/scheduler/start", s.handleStartScheduler)
	mux.HandleFunc("POST /internal/v1/game/scheduler/stop", s.handleStopScheduler)
	mux.HandleFunc("PUT /internal/v1/game/scheduler/interval", s.handleUpdateScheduler)
	mux.HandleFunc("POST /internal/v1/flags/submit", s.handleSubmitFlags)
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
		case errors.Is(err, errContestNotStarted), errors.Is(err, errContestOver):
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
	writeData(w, http.StatusOK, status)
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
		case errors.Is(err, errContestNotStarted), errors.Is(err, errContestOver):
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
		case errors.Is(err, errContestNotStarted), errors.Is(err, errContestOver):
			statusCode = http.StatusBadRequest
		}
		writeProblem(w, statusCode, "Flag submission failed", err.Error())
		return
	}
	writeData(w, http.StatusOK, toSubmissionVerdictAliases(results))
}

func (s *gameCoreServer) requireAdminAuth(w http.ResponseWriter, r *http.Request) bool {
	token, ok := httpapi.BearerToken(r)
	if !ok || token != s.adminToken {
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

	for _, target := range targets {
		flag := s.flags.Issue(target.TeamID, target.ChallengeID, tick.ID, tick.ID)
		metadata := ""
		halted := false

		for _, phase := range s.checkerPhases {
			runTime := s.now().UTC()
			run := checkerRunRecord{
				TickID:        tick.ID,
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

			if halted {
				run.Status = "skipped"
				run.Message = "phase skipped after previous checker failure"
			} else {
				result, execErr := s.checker.Execute(ctx, apigateway.CheckerExecutionRequest{
					ChallengeID:    target.ChallengeID,
					TeamID:         target.TeamID,
					TeamName:       target.TeamName,
					ChallengeName:  target.ChallengeName,
					CheckerImage:   target.CheckerImage,
					Phase:          phase,
					Target:         target.Target,
					TargetHost:     target.TargetHost,
					TargetIP:       target.TargetIP,
					TargetPort:     target.TargetPort,
					TickID:         tick.ID,
					Flag:           flag,
					Metadata:       metadata,
					TimeoutSeconds: s.checkerTimeout,
				})
				if execErr != nil {
					run.Status = "failed"
					run.Message = execErr.Error()
				} else {
					run.Status = normalizedCheckerRunStatus(result.Status)
					run.ExitCode = result.ExitCode
					run.Message = strings.TrimSpace(result.Message)
					run.Output = strings.TrimSpace(result.Output)
					if parsed, err := time.Parse(time.RFC3339, result.CheckedAt); err == nil {
						run.CheckedAt = parsed.UTC()
					}
				}

				if run.Status == "success" && run.Output != "" {
					metadata = run.Output
				}
				if run.Status == "failed" {
					halted = true
				}
			}

			if _, err := s.store.RecordCheckerRun(ctx, run); err != nil {
				tick.Status = "failed"
				tick.CompletedAt = s.now().UTC().Format(time.RFC3339)
				tick.Message = fmt.Sprintf("checker run persistence failed: %v", err)
				_, _ = s.store.CompleteTick(ctx, tick)
				return apigateway.GameTickStatus{}, err
			}

			if phase == "put" && run.Status == "success" {
				flag := issuedFlagRecord{
					Flag:          flag,
					OwnerTeamID:   target.TeamID,
					OwnerTeamName: target.TeamName,
					ChallengeID:   target.ChallengeID,
					ChallengeName: target.ChallengeName,
					IssuedTick:    tick.ID,
					ExpiresTick:   tick.ID,
					CreatedAt:     run.CheckedAt,
				}
				if err := s.store.IssueFlag(ctx, flag); err != nil {
					tick.Status = "failed"
					tick.CompletedAt = s.now().UTC().Format(time.RFC3339)
					tick.Message = fmt.Sprintf("flag issuance persistence failed: %v", err)
					_, _ = s.store.CompleteTick(ctx, tick)
					return apigateway.GameTickStatus{}, err
				}
			}

			tick.TotalCheckerRuns++
			switch run.Status {
			case "success":
				tick.SuccessfulCheckerRuns++
			case "skipped":
				tick.SkippedCheckerRuns++
			default:
				tick.FailedCheckerRuns++
			}
		}
	}

	tick.Status = "completed"
	tick.CompletedAt = s.now().UTC().Format(time.RFC3339)
	tick.Message = fmt.Sprintf(
		"advanced tick across %d targets and %d phases",
		len(targets),
		len(s.checkerPhases),
	)
	return s.store.CompleteTick(ctx, tick)
}

func (s *gameCoreServer) submitFlags(ctx context.Context, teamID int, flags []string) ([]submissionVerdictAlias, error) {
	match, err := s.matchStatus(ctx)
	if err != nil {
		return nil, err
	}
	switch match.State {
	case "finished":
		return nil, errContestOver
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

		claims, ok := s.flags.Parse(trimmed)
		if !ok || currentTick == 0 || currentTick > claims.ExpiresTick || claims.OwnerTeamID == teamID {
			results = append(results, submissionVerdictAlias{Flag: trimmed, Status: "invalid", Detail: "flag is wrong or expired."})
			continue
		}

		issued, err := s.store.LookupIssuedFlag(ctx, trimmed)
		if err != nil || issued.OwnerTeamID != claims.OwnerTeamID || issued.ChallengeID != claims.ChallengeID || issued.IssuedTick != claims.IssuedTick || issued.ExpiresTick != claims.ExpiresTick {
			results = append(results, submissionVerdictAlias{Flag: trimmed, Status: "invalid", Detail: "flag is wrong or expired."})
			continue
		}

		accepted, err := s.store.AcceptFlagSubmission(ctx, acceptedFlagSubmission{
			Flag:           trimmed,
			SubmittingTeam: teamID,
			AttackerName:   attackerName,
			VictimName:     issued.OwnerTeamName,
			ChallengeName:  issued.ChallengeName,
			SubmissionTick: currentTick,
			SubmittedAt:    s.now().UTC(),
		})
		if err != nil {
			return nil, err
		}
		if !accepted {
			results = append(results, submissionVerdictAlias{Flag: trimmed, Status: "duplicate", Detail: "flag already submitted."})
			continue
		}

		results = append(results, submissionVerdictAlias{Flag: trimmed, Status: "accepted", Detail: "flag is correct."})
	}
	return results, nil
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
