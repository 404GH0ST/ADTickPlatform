package apigateway

import (
	"context"
	"crypto/subtle"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"adplatform/internal/platform/httpapi"
	"adplatform/internal/platform/unlockproof"
)

type Server struct {
	teamTokenSecret     string
	adminToken          string
	sshCredentialSecret string
	store               Store
	controller          controllerClient
	wireGuard           wireGuardClient
	submission          submissionClient
	scoring             scoringClient
	gameCore            gameCoreClient
	rateLimiter         rateLimiter
	rateLimitMetrics    *rateLimitMetrics
	unlockProofSecret   string
	now                 func() time.Time
}

func New(teamToken, adminToken string, teamID int) *Server {
	return NewWithDeps(teamToken, adminToken, teamID, NewMemoryStore(teamID), noopControllerClient{}, noopWireGuardClient{})
}

func NewWithStore(teamToken, adminToken string, teamID int, store Store) *Server {
	return NewWithDeps(teamToken, adminToken, teamID, store, noopControllerClient{}, noopWireGuardClient{})
}

func NewWithDeps(teamToken, adminToken string, teamID int, store Store, controller controllerClient, wireGuard wireGuardClient, gameCore ...gameCoreClient) *Server {
	game := gameCoreClient(noopGameCoreClient{})
	if len(gameCore) > 0 && gameCore[0] != nil {
		game = gameCore[0]
	}
	return &Server{
		teamTokenSecret:     teamToken,
		adminToken:          adminToken,
		sshCredentialSecret: teamToken,
		store:               store,
		controller:          controller,
		wireGuard:           wireGuard,
		submission:          noopSubmissionClient{},
		scoring:             noopScoringClient{},
		gameCore:            game,
		rateLimiter:         noopRateLimiter{},
		rateLimitMetrics:    newRateLimitMetrics(),
		unlockProofSecret:   teamToken,
		now:                 time.Now,
	}
}

func (s *Server) WithSubmissionClient(client submissionClient) *Server {
	if client != nil {
		s.submission = client
	}
	return s
}

func (s *Server) WithScoringClient(client scoringClient) *Server {
	if client != nil {
		s.scoring = client
	}
	return s
}

func (s *Server) WithUnlockProofSecret(secret string) *Server {
	trimmed := strings.TrimSpace(secret)
	if trimmed != "" {
		s.unlockProofSecret = trimmed
	}
	return s
}

func (s *Server) WithSSHCredentialSecret(secret string) *Server {
	trimmed := strings.TrimSpace(secret)
	if trimmed != "" {
		s.sshCredentialSecret = trimmed
	}
	return s
}

func (s *Server) WithRateLimiter(limiter rateLimiter) *Server {
	if limiter != nil {
		s.rateLimiter = limiter
	}
	return s
}

func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v2/authenticate", s.handleAuthenticate)
	mux.HandleFunc("POST /api/v2/register", s.handleRegisterPlayer)
	mux.HandleFunc("POST /api/v2/me/team", s.handleJoinExistingTeam)
	mux.HandleFunc("PUT /api/v2/me/profile", s.handleUpdateParticipantProfile)
	mux.HandleFunc("GET /api/v2/session", s.handleSession)
	s.registerOpsFeatureRoutes(mux)
	mux.HandleFunc("GET /api/v2/me/wireguard", s.handleParticipantWireGuardConfig)
	mux.HandleFunc("GET /api/v2/challenges", s.handleChallenges)
	mux.HandleFunc("GET /api/v2/challenges/{challenge_id}/source", s.handleChallengeSourceDownload)
	mux.HandleFunc("GET /api/v2/services", s.handleServices)
	mux.HandleFunc("GET /api/v2/scoreboard", s.handleScoreboard)
	mux.HandleFunc("GET /api/v2/scoreboard/freeze", s.handleScoreboardFreezeStatus)
	mux.HandleFunc("GET /api/v2/game/status", s.handleGameStatus)
	mux.HandleFunc("GET /api/v2/attacks", s.handleAttacks)
	mux.HandleFunc("GET /api/v2/team/services", s.handleTeamServices)
	mux.HandleFunc("POST /api/v2/submit", s.handleSubmit)
	mux.HandleFunc("POST /api/v2/services/{challenge_id}/unlock", s.handleUnlock)
	mux.HandleFunc("POST /api/v2/services/{challenge_id}/ssh-session", s.handleSSHSession)
	mux.HandleFunc("POST /api/v2/services/{challenge_id}/reset/factory", s.handleFactoryReset)
	mux.HandleFunc("POST /api/v2/services/{challenge_id}/reset/restart", s.handleRestart)
	mux.HandleFunc("GET /api/v2/admin/teams", s.handleAdminListTeams)
	mux.HandleFunc("POST /api/v2/admin/teams", s.handleAdminCreateTeam)
	mux.HandleFunc("PUT /api/v2/admin/teams/{team_id}", s.handleAdminUpdateTeam)
	mux.HandleFunc("DELETE /api/v2/admin/teams/{team_id}", s.handleAdminDeleteTeam)
	mux.HandleFunc("POST /api/v2/admin/teams/{team_id}/deactivate", s.handleAdminDeactivateTeam)
	mux.HandleFunc("POST /api/v2/admin/teams/{team_id}/reactivate", s.handleAdminReactivateTeam)
	mux.HandleFunc("GET /api/v2/admin/players", s.handleAdminListPlayers)
	mux.HandleFunc("POST /api/v2/admin/players", s.handleAdminCreatePlayer)
	mux.HandleFunc("PUT /api/v2/admin/players/{player_id}", s.handleAdminUpdatePlayer)
	mux.HandleFunc("DELETE /api/v2/admin/players/{player_id}", s.handleAdminDeletePlayer)
	mux.HandleFunc("POST /api/v2/admin/players/{player_id}/deactivate", s.handleAdminDeactivatePlayer)
	mux.HandleFunc("POST /api/v2/admin/players/{player_id}/reactivate", s.handleAdminReactivatePlayer)
	mux.HandleFunc("GET /api/v2/admin/players/{player_id}/wireguard", s.handleAdminGetPlayerWireGuard)
	mux.HandleFunc("POST /api/v2/admin/players/{player_id}/wireguard/rotate", s.handleAdminRotatePlayerWireGuard)
	mux.HandleFunc("POST /api/v2/admin/players/{player_id}/wireguard/revoke", s.handleAdminRevokePlayerWireGuard)
	mux.HandleFunc("GET /api/v2/admin/wireguard/status", s.handleAdminWireGuardGatewayStatus)
	mux.HandleFunc("POST /api/v2/admin/wireguard/reconcile", s.handleAdminWireGuardGatewayReconcile)
	mux.HandleFunc("POST /api/v2/admin/wireguard/teardown", s.handleAdminWireGuardGatewayTeardown)
	mux.HandleFunc("GET /api/v2/admin/access/status", s.handleAdminAccessStatus)
	mux.HandleFunc("POST /api/v2/admin/access/reconcile", s.handleAdminAccessReconcile)
	mux.HandleFunc("POST /api/v2/admin/access/teardown", s.handleAdminAccessTeardown)
	mux.HandleFunc("GET /api/v2/admin/challenges", s.handleAdminListChallenges)
	mux.HandleFunc("POST /api/v2/admin/challenges", s.handleAdminCreateChallenge)
	mux.HandleFunc("PUT /api/v2/admin/challenges/{challenge_id}", s.handleAdminUpdateChallenge)
	mux.HandleFunc("DELETE /api/v2/admin/challenges/{challenge_id}", s.handleAdminDeleteChallenge)
	mux.HandleFunc("POST /api/v2/admin/challenges/{challenge_id}/validate", s.handleAdminValidateChallenge)
	mux.HandleFunc("POST /api/v2/admin/challenges/{challenge_id}/deploy", s.handleAdminDeployChallenge)
	mux.HandleFunc("POST /api/v2/admin/challenges/{challenge_id}/maintenance", s.handleAdminChallengeMaintenance)
	mux.HandleFunc("POST /api/v2/admin/challenges/{challenge_id}/rotate-unlock-proof", s.handleAdminRotateChallengeUnlockProof)
	mux.HandleFunc("POST /api/v2/admin/challenges/{challenge_id}/resume", s.handleAdminChallengeResume)
	mux.HandleFunc("GET /api/v2/admin/deployments", s.handleAdminListDeployments)
	mux.HandleFunc("DELETE /api/v2/admin/deployments/{deployment_id}", s.handleAdminDeleteDeployment)
	mux.HandleFunc("POST /api/v2/admin/deployments/reconcile", s.handleAdminReconcileDeployments)
	mux.HandleFunc("GET /api/v2/admin/audit-logs", s.handleAdminListAuditLogs)
	mux.HandleFunc("GET /api/v2/admin/operations/status", s.handleAdminOperationsStatus)
	mux.HandleFunc("GET /api/v2/admin/game/status", s.handleAdminGameStatus)
	mux.HandleFunc("GET /api/v2/admin/game/match", s.handleAdminGameMatchStatus)
	mux.HandleFunc("POST /api/v2/admin/game/match/start", s.handleAdminStartGameMatch)
	mux.HandleFunc("POST /api/v2/admin/game/match/pause", s.handleAdminPauseGameMatch)
	mux.HandleFunc("POST /api/v2/admin/game/match/resume", s.handleAdminResumeGameMatch)
	mux.HandleFunc("POST /api/v2/admin/game/match/stop", s.handleAdminStopGameMatch)
	mux.HandleFunc("PUT /api/v2/admin/game/match/schedule", s.handleAdminUpdateGameMatchSchedule)
	mux.HandleFunc("POST /api/v2/admin/game/ticks/advance", s.handleAdminAdvanceGameTick)
	mux.HandleFunc("GET /api/v2/admin/game/checker-runs", s.handleAdminListCheckerRuns)
	mux.HandleFunc("GET /api/v2/admin/game/scheduler", s.handleAdminGameSchedulerStatus)
	mux.HandleFunc("GET /api/v2/admin/game/scheduler/events", s.handleAdminGameSchedulerEvents)
	mux.HandleFunc("POST /api/v2/admin/game/scheduler/start", s.handleAdminStartGameScheduler)
	mux.HandleFunc("POST /api/v2/admin/game/scheduler/stop", s.handleAdminStopGameScheduler)
	mux.HandleFunc("PUT /api/v2/admin/game/scheduler/interval", s.handleAdminUpdateGameScheduler)
	mux.HandleFunc("GET /api/v2/admin/game/scoreboard", s.handleAdminGameScoreboard)
	mux.HandleFunc("GET /api/v2/admin/game/scoreboard/freeze", s.handleAdminScoreboardFreezeStatus)
	mux.HandleFunc("POST /api/v2/admin/game/scoreboard/freeze", s.handleAdminSetScoreboardFreeze)
	mux.HandleFunc("POST /api/v2/admin/game/scoreboard/unfreeze", s.handleAdminClearScoreboardFreeze)
	mux.HandleFunc("POST /api/v2/admin/game/scoring/recompute", s.handleAdminRecomputeScoring)
	mux.HandleFunc("GET /api/v2/admin/game/scoring/audit", s.handleAdminAuditScoring)
	mux.HandleFunc("GET /internal/v1/rate-limit/metrics", s.handleRateLimitMetrics)
	mux.HandleFunc("GET /api/v2/admin/platform/settings", s.handleAdminGetPlatformSettings)
	mux.HandleFunc("PUT /api/v2/admin/platform/settings", s.handleAdminUpdatePlatformSettings)
	mux.HandleFunc("POST /api/v2/admin/platform/settings/reload", s.handleAdminReloadFlagFormat)
}

func (s *Server) handleAuthenticate(w http.ResponseWriter, r *http.Request) {
	var req authenticateRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		writeProblem(w, http.StatusForbidden, "Authentication failed", "email or password is wrong.")
		return
	}

	decision, allowed := s.allowRateLimit(r.Context(), rateLimitClientKey("auth", clientRateLimitKey(r)), authClientRateLimitPolicy)
	if !allowed {
		writeRateLimitFailure(w, decision, defaultRateLimit429Message)
		return
	}
	decision, allowed = s.allowRateLimit(r.Context(), rateLimitAuthEmailKey(req.Email), authRateLimitPolicy)
	if !allowed {
		writeRateLimitFailure(w, decision, defaultRateLimit429Message)
		return
	}

	if strings.TrimSpace(req.Email) == "" || strings.TrimSpace(req.Password) == "" {
		writeProblem(w, http.StatusForbidden, "Authentication failed", "email or password is wrong.")
		return
	}

	player, err := s.store.AuthenticatePlayer(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, ErrAccountDeactivated) {
			writeProblem(w, http.StatusForbidden, "Access deactivated", "Your account or team has been deactivated by the organizers.")
			return
		}
		if errors.Is(err, ErrInvalidCredentials) {
			writeProblem(w, http.StatusForbidden, "Authentication failed", "email or password is wrong.")
			return
		}
		writeStoreFailure(w, err)
		return
	}

	token, err := issueTeamJWT(s.teamTokenSecret, player, s.now())
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "Authentication failed", "team authentication token could not be issued.")
		return
	}

	writeData(w, http.StatusOK, authenticateResponse{Token: token, TokenType: "Bearer"})
}

// validParticipantName rejects display/team names that contain control
// characters (notably CR/LF) or exceed the rendered length budget. These values
// flow into WireGuard peer config comments; an unfiltered newline could inject
// rogue [Peer]/AllowedIPs directives applied by wg syncconf.
func validParticipantName(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || len(trimmed) > 96 {
		return false
	}
	for _, r := range trimmed {
		if r == '\n' || r == '\r' || unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func (s *Server) handleRegisterPlayer(w http.ResponseWriter, r *http.Request) {
	var req participantRegisterRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		writeProblem(w, http.StatusBadRequest, "Invalid request", "player registration request is invalid.")
		return
	}

	if !validParticipantName(req.DisplayName) {
		writeProblem(w, http.StatusBadRequest, "Invalid request", "display name must be 1-96 characters with no control characters.")
		return
	}
	if !validPassword(req.Password) {
		writeProblem(w, http.StatusBadRequest, "Invalid request", "password must be at least 8 characters.")
		return
	}

	decision, allowed := s.allowRateLimit(r.Context(), rateLimitClientKey("register", clientRateLimitKey(r)), registrationIPRateLimitPolicy)
	if !allowed {
		writeRateLimitFailure(w, decision, defaultRateLimit429Message)
		return
	}
	decision, allowed = s.allowRateLimit(r.Context(), rateLimitRegistrationEmailKey(req.Email), registrationEmailRateLimitPolicy)
	if !allowed {
		writeRateLimitFailure(w, decision, defaultRateLimit429Message)
		return
	}

	player, err := s.store.RegisterPlayer(r.Context(), req, s.now())
	if err != nil {
		switch {
		case errors.Is(err, ErrDuplicateResource):
			writeProblem(w, http.StatusBadRequest, "Registration failed", "display name, email, password, and a unique email are required.")
		default:
			writeStoreFailure(w, err)
		}
		return
	}

	token, err := issueTeamJWT(s.teamTokenSecret, player, s.now())
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "Registration failed", "team authentication token could not be issued.")
		return
	}

	writeData(w, http.StatusOK, authenticateResponse{Token: token, TokenType: "Bearer"})
}

func (s *Server) handleJoinExistingTeam(w http.ResponseWriter, r *http.Request) {
	player, ok := s.requirePlayerAuth(w, r, "please authenticate before joining a team.")
	if !ok {
		return
	}
	var req participantJoinExistingTeamRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil || strings.TrimSpace(req.TeamKey) == "" {
		writeProblem(w, http.StatusBadRequest, "Invalid request", "team key is required.")
		return
	}

	decision, allowed := s.allowRateLimit(r.Context(), rateLimitAuthKey(player.Email, clientRateLimitKey(r)), authRateLimitPolicy)
	if !allowed {
		writeRateLimitFailure(w, decision, defaultRateLimit429Message)
		return
	}

	updatedPlayer, err := s.store.JoinExistingPlayerTeam(r.Context(), player.PlayerID, req.TeamKey, s.now())
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidCredentials):
			writeProblem(w, http.StatusForbidden, "Team join failed", "team key is invalid.")
		case errors.Is(err, ErrTeamMemberLimit):
			writeProblem(w, http.StatusBadRequest, "Team join failed", "team has reached the maximum member count.")
		case errors.Is(err, ErrDuplicateResource):
			writeProblem(w, http.StatusBadRequest, "Team join failed", "player has already joined a team.")
		default:
			writeStoreFailure(w, err)
		}
		return
	}
	s.reconcileWireGuardGatewayBestEffort(r.Context(), "participant team join")

	token, err := issueTeamJWT(s.teamTokenSecret, updatedPlayer, s.now())
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "Team join failed", "team authentication token could not be issued.")
		return
	}

	writeData(w, http.StatusOK, authenticateResponse{Token: token, TokenType: "Bearer"})
}

func (s *Server) handleChallenges(w http.ResponseWriter, r *http.Request) {
	decision, allowed := s.allowRateLimit(r.Context(), rateLimitClientKey("challenges", clientRateLimitKey(r)), challengesRateLimitPolicy)
	if !allowed {
		writeRateLimitFailure(w, decision, defaultRateLimit429Message)
		return
	}

	challenges, err := s.store.ListChallenges(r.Context())
	if err != nil {
		writeStoreFailure(w, err)
		return
	}
	writeData(w, http.StatusOK, challenges)
}

func (s *Server) handleServices(w http.ResponseWriter, r *http.Request) {
	teamID, ok := s.requireTeamAuth(w, r, "please authenticate before access.")
	if !ok {
		return
	}
	decision, allowed := s.allowRateLimit(r.Context(), rateLimitTeamKey("services", teamID), servicesReadRateLimitPolicy)
	if !allowed {
		writeRateLimitFailure(w, decision, defaultRateLimit429Message)
		return
	}
	services, err := s.store.ListPublicServices(r.Context())
	if err != nil {
		writeStoreFailure(w, err)
		return
	}
	writeData(w, http.StatusOK, services)
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	player, ok := s.requirePlayerAuth(w, r, "please authenticate before access.")
	if !ok {
		return
	}
	writeData(w, http.StatusOK, map[string]any{
		"player_id":          player.PlayerID,
		"team_id":            player.TeamID,
		"team_name":          player.TeamName,
		"team_contact_email": player.TeamContactEmail,
		"display_name":       player.DisplayName,
		"email":              player.Email,
		"role":               player.Role,
	})
}

func (s *Server) handleUpdateParticipantProfile(w http.ResponseWriter, r *http.Request) {
	player, ok := s.requirePlayerAuth(w, r, "please authenticate before profile update.")
	if !ok {
		return
	}
	if strings.EqualFold(strings.TrimSpace(player.Role), "organizer") {
		writeProblem(w, http.StatusForbidden, "Authentication required", "please authenticate as a participant.")
		return
	}

	var req participantUpdateProfileRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		writeProblem(w, http.StatusBadRequest, "Invalid request", "profile update request is invalid.")
		return
	}

	if !validParticipantName(req.DisplayName) || (strings.TrimSpace(req.TeamName) != "" && !validParticipantName(req.TeamName)) {
		writeProblem(w, http.StatusBadRequest, "Invalid request", "display name and team name must be 1-96 characters with no control characters.")
		return
	}

	updatedPlayer, err := s.store.UpdateParticipantProfile(r.Context(), player.PlayerID, req)
	if err != nil {
		switch {
		case errors.Is(err, ErrDuplicateResource):
			writeProblem(w, http.StatusBadRequest, "Profile update failed", "display name, email, team name, team email, and unique names/emails are required.")
		case errors.Is(err, ErrTeamNotFound):
			writeProblem(w, http.StatusBadRequest, "Profile update failed", "team membership is required before editing team information.")
		default:
			writeStoreFailure(w, err)
		}
		return
	}

	token, err := issueTeamJWT(s.teamTokenSecret, updatedPlayer, s.now())
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "Profile update failed", "team authentication token could not be issued.")
		return
	}
	writeData(w, http.StatusOK, authenticateResponse{Token: token, TokenType: "Bearer"})
}

func (s *Server) handleScoreboard(w http.ResponseWriter, r *http.Request) {
	decision, allowed := s.allowRateLimit(r.Context(), rateLimitClientKey("scoreboard", clientRateLimitKey(r)), scoreboardRateLimitPolicy)
	if !allowed {
		writeRateLimitFailure(w, decision, defaultRateLimit429Message)
		return
	}

	// During a freeze the participant-facing board serves the snapshot captured
	// when the window first became active. Organizers keep hitting the live board
	// via the admin endpoint, which never consults the freeze window.
	now := s.now()
	freeze, err := s.store.GetScoreboardFreeze(r.Context())
	if err != nil {
		writeStoreFailure(w, err)
		return
	}
	frozen := freeze.activeAt(now)
	if frozen && freeze.Snapshot != nil {
		writeData(w, http.StatusOK, freeze.Snapshot)
		return
	}

	rows, ok := s.fetchLiveScoreboard(w, r)
	if !ok {
		return
	}
	if frozen {
		// First read after freeze_at: capture the live board as the snapshot.
		// SaveFrozenScoreboardSnapshot only writes when none exists yet, so
		// concurrent first-reads converge on one snapshot.
		saved, err := s.store.SaveFrozenScoreboardSnapshot(r.Context(), rows, now)
		if err != nil {
			writeStoreFailure(w, err)
			return
		}
		if saved.Snapshot == nil {
			writeStoreFailure(w, errors.New("scoreboard freeze snapshot was not persisted"))
			return
		}
		rows = saved.Snapshot
	}
	writeData(w, http.StatusOK, rows)
}

// fetchLiveScoreboard returns the current live board, preferring scoring-worker,
// then game-core, then the local store. On a hard upstream failure it writes the
// problem response and returns ok=false.
func (s *Server) fetchLiveScoreboard(w http.ResponseWriter, r *http.Request) ([]scoreRow, bool) {
	if s.scoring != nil {
		if rows, err := s.scoring.Scoreboard(r.Context()); err == nil {
			return rows, true
		} else if !errors.Is(err, errScoringWorkerDisabled) {
			writeProblem(w, http.StatusBadGateway, "Scoreboard unavailable", "scoring-worker scoreboard failed.")
			return nil, false
		}
	}
	if s.gameCore != nil {
		if rows, err := s.gameCore.Scoreboard(r.Context()); err == nil {
			return rows, true
		} else if !errors.Is(err, errGameCoreDisabled) {
			writeProblem(w, http.StatusBadGateway, "Scoreboard unavailable", "game-core scoreboard failed.")
			return nil, false
		}
	}
	rows, err := s.store.ListScoreboard(r.Context())
	if err != nil {
		writeStoreFailure(w, err)
		return nil, false
	}
	return rows, true
}

func (s *Server) handleScoreboardFreezeStatus(w http.ResponseWriter, r *http.Request) {
	decision, allowed := s.allowRateLimit(r.Context(), rateLimitClientKey("scoreboard", clientRateLimitKey(r)), scoreboardRateLimitPolicy)
	if !allowed {
		writeRateLimitFailure(w, decision, defaultRateLimit429Message)
		return
	}
	freeze, err := s.store.GetScoreboardFreeze(r.Context())
	if err != nil {
		writeStoreFailure(w, err)
		return
	}
	writeData(w, http.StatusOK, scoreboardFreezeStatusFrom(freeze, s.now()))
}

func (s *Server) handleGameStatus(w http.ResponseWriter, r *http.Request) {
	decision, allowed := s.allowRateLimit(r.Context(), rateLimitClientKey("game-status", clientRateLimitKey(r)), scoreboardRateLimitPolicy)
	if !allowed {
		writeRateLimitFailure(w, decision, defaultRateLimit429Message)
		return
	}

	status, err := s.gameCore.Status(r.Context())
	if err != nil {
		writeGameCoreFailure(w, err, "game-core status could not be read.")
		return
	}
	// Public surface must not expose per-target warmup errors: those strings can
	// include checker argv material (flags / checker tokens) from failed puts.
	writeData(w, http.StatusOK, sanitizePublicGameStatus(status))
}

// sanitizePublicGameStatus strips diagnostics that belong only on organizer
// endpoints. Summary counters stay so participants can see match readiness.
func sanitizePublicGameStatus(status GameStatus) GameStatus {
	if status.Match == nil || status.Match.Warmup == nil {
		return status
	}
	public := status
	matchCopy := *status.Match
	warmupCopy := *status.Match.Warmup
	warmupCopy.Failures = nil
	if containsSensitiveCheckerDetail(warmupCopy.Message) {
		if strings.EqualFold(strings.TrimSpace(warmupCopy.Status), "failed") {
			warmupCopy.Message = "warmup put phase failed for one or more targets"
		} else {
			warmupCopy.Message = ""
		}
	}
	matchCopy.Warmup = &warmupCopy
	public.Match = &matchCopy
	return public
}

func (s *Server) handleAttacks(w http.ResponseWriter, r *http.Request) {
	decision, allowed := s.allowRateLimit(r.Context(), rateLimitClientKey("attacks", clientRateLimitKey(r)), attacksReadRateLimitPolicy)
	if !allowed {
		writeRateLimitFailure(w, decision, defaultRateLimit429Message)
		return
	}

	query := parseAttackFeedQuery(r)
	if s.submission != nil {
		if events, err := s.submission.AttackFeed(r.Context(), query); err == nil {
			writeData(w, http.StatusOK, events)
			return
		} else if !errors.Is(err, errSubmissionServiceDisabled) {
			writeProblem(w, http.StatusBadGateway, "Attack feed unavailable", "submission-service attack feed failed.")
			return
		}
	}
	if s.gameCore != nil {
		if events, err := s.gameCore.AttackFeed(r.Context(), query); err == nil {
			writeData(w, http.StatusOK, events)
			return
		} else if !errors.Is(err, errGameCoreDisabled) {
			writeProblem(w, http.StatusBadGateway, "Attack feed unavailable", "game-core attack feed failed.")
			return
		}
	}
	events, err := s.store.ListAttackFeed(r.Context())
	if err != nil {
		writeStoreFailure(w, err)
		return
	}
	writeData(w, http.StatusOK, paginateAttackFeed(events, query))
}

func parseAttackFeedQuery(r *http.Request) AttackFeedQuery {
	return AttackFeedQuery{
		Limit:    parsePositiveIntWithDefault(strings.TrimSpace(r.URL.Query().Get("limit")), 12, 200),
		Offset:   parseNonNegativeInt(strings.TrimSpace(r.URL.Query().Get("offset"))),
		Attacker: strings.TrimSpace(r.URL.Query().Get("attacker")),
		Victim:   strings.TrimSpace(r.URL.Query().Get("victim")),
		Service:  strings.TrimSpace(r.URL.Query().Get("service")),
		TickFrom: parseNonNegativeInt(strings.TrimSpace(r.URL.Query().Get("tick_from"))),
		TickTo:   parseNonNegativeInt(strings.TrimSpace(r.URL.Query().Get("tick_to"))),
	}
}

func paginateAttackFeed(events []attackEvent, query AttackFeedQuery) AttackFeedPage {
	filtered := filterAttackFeed(events, query)
	limit := query.Limit
	if limit <= 0 {
		limit = 12
	}
	offset := query.Offset
	if offset < 0 {
		offset = 0
	}

	page := AttackFeedPage{
		Limit:      limit,
		Offset:     offset,
		TotalCount: len(filtered),
		HasPrev:    offset > 0,
	}
	if offset >= len(filtered) {
		page.Items = []attackEvent{}
		return page
	}
	if limit > len(filtered)-offset {
		limit = len(filtered) - offset
	}
	page.HasNext = offset+limit < len(filtered)
	page.Items = append([]attackEvent(nil), filtered[offset:offset+limit]...)
	return page
}

func filterAttackFeed(events []attackEvent, query AttackFeedQuery) []attackEvent {
	if query.Attacker == "" && query.Victim == "" && query.Service == "" && query.TickFrom == 0 && query.TickTo == 0 {
		return events
	}

	filtered := make([]attackEvent, 0, len(events))
	for _, event := range events {
		if !attackFeedTextMatch(event.Attacker, query.Attacker) {
			continue
		}
		if !attackFeedTextMatch(event.Victim, query.Victim) {
			continue
		}
		if !attackFeedTextMatch(event.Service, query.Service) {
			continue
		}
		if query.TickFrom > 0 && event.Tick < query.TickFrom {
			continue
		}
		if query.TickTo > 0 && event.Tick > query.TickTo {
			continue
		}
		filtered = append(filtered, event)
	}
	return filtered
}

func attackFeedTextMatch(value string, filter string) bool {
	if filter == "" {
		return true
	}
	return strings.Contains(strings.ToLower(value), strings.ToLower(filter))
}

func parsePositiveIntWithDefault(raw string, fallback int, max int) int {
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

func parseNonNegativeInt(raw string) int {
	if raw == "" {
		return 0
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed < 0 {
		return 0
	}
	return parsed
}

func (s *Server) handleTeamServices(w http.ResponseWriter, r *http.Request) {
	teamID, ok := s.requireTeamAuth(w, r, "please authenticate before access.")
	if !ok {
		return
	}
	decision, allowed := s.allowRateLimit(r.Context(), rateLimitTeamKey("team-services", teamID), teamServicesRateLimitPolicy)
	if !allowed {
		writeRateLimitFailure(w, decision, defaultRateLimit429Message)
		return
	}
	states, err := s.store.ListTeamServices(r.Context(), teamID)
	if err != nil {
		writeStoreFailure(w, err)
		return
	}
	states = s.enrichServiceStatesWithSLADetails(r.Context(), teamID, states)
	s.sanitizeParticipantServiceStates(r.Context(), states)
	writeData(w, http.StatusOK, states)
}

func (s *Server) handleSubmit(w http.ResponseWriter, r *http.Request) {
	player, ok := s.requirePlayerAuth(w, r, "please authenticate before submit.")
	if !ok {
		return
	}
	if strings.EqualFold(strings.TrimSpace(player.Role), "organizer") {
		writeProblem(w, http.StatusForbidden, "Authentication required", "please authenticate before submit.")
		return
	}
	if player.TeamID <= 0 {
		writeProblem(w, http.StatusForbidden, "Team membership required", "please join a team before submitting flags.")
		return
	}
	teamID := player.TeamID

	decision, allowed := s.allowRateLimit(r.Context(), rateLimitTeamKey("submit", teamID), submitRateLimitPolicy)
	if !allowed {
		writeRateLimitFailure(w, decision, defaultRateLimit429Message)
		return
	}
	decision, allowed = s.allowRateLimit(r.Context(), rateLimitUserKey("submit", player.PlayerID), submitUserRateLimitPolicy)
	if !allowed {
		writeRateLimitFailure(w, decision, defaultRateLimit429Message)
		return
	}

	var req submitRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil || len(req.Flags) == 0 {
		writeProblem(w, http.StatusBadRequest, "Invalid request", "request body is invalid.")
		return
	}
	if len(req.Flags) > maxSubmitFlagsPerRequest {
		writeProblem(w, http.StatusBadRequest, "Submission rejected", "too many flags in one request.")
		return
	}

	if s.gameCore != nil {
		if status, statusErr := s.gameCore.Status(r.Context()); statusErr == nil {
			switch matchSubmissionState(status.Match) {
			case "not_started":
				writeProblem(w, http.StatusBadRequest, "Submission rejected", "contest has not started yet.")
				return
			case "paused":
				writeProblem(w, http.StatusBadRequest, "Submission rejected", "contest is temporarily paused.")
				return
			case "finished":
				writeProblem(w, http.StatusBadRequest, "Submission rejected", "contest is over.")
				return
			}
		} else if !errors.Is(statusErr, errGameCoreDisabled) {
			writeProblem(w, http.StatusBadGateway, "Submission unavailable", "game-core status could not be read.")
			return
		}

	}

	if s.submission != nil {
		if results, submissionErr := s.submission.SubmitFlags(r.Context(), teamID, req.Flags); submissionErr == nil {
			writeData(w, http.StatusOK, newSubmissionResult(results))
			return
		} else if !errors.Is(submissionErr, errSubmissionServiceDisabled) {
			writeProblem(w, http.StatusBadGateway, "Submission unavailable", "submission-service flag submission failed.")
			return
		}
	}

	if s.gameCore != nil {
		if gameResults, gameErr := s.gameCore.SubmitFlags(r.Context(), teamID, req.Flags); gameErr == nil {
			writeData(w, http.StatusOK, newSubmissionResult(gameResults))
			return
		} else if !errors.Is(gameErr, errGameCoreDisabled) {
			writeProblem(w, http.StatusBadGateway, "Submission unavailable", "game-core flag submission failed.")
			return
		}
	}
	results, err := s.store.SubmitFlags(r.Context(), teamID, req.Flags)
	if err != nil {
		if errors.Is(err, ErrSubmissionUnavailable) {
			writeProblem(w, http.StatusServiceUnavailable, "Submission unavailable", "authoritative flag submission backend is unavailable.")
			return
		}
		writeStoreFailure(w, err)
		return
	}
	writeData(w, http.StatusOK, newSubmissionResult(results))
}

func (s *Server) enrichServiceStatesWithSLADetails(ctx context.Context, teamID int, states []serviceState) []serviceState {
	if len(states) == 0 {
		return states
	}

	remaining := make(map[int]struct{}, len(states))
	for i := range states {
		if strings.TrimSpace(states[i].SLAStatus) == "" {
			states[i].SLAStatus = fallbackSLAStatus(states[i].Checker)
		}
		if strings.TrimSpace(states[i].SLAMessage) == "" {
			states[i].SLAMessage = fallbackSLAMessage(states[i].Checker)
		}
		remaining[states[i].ChallengeID] = struct{}{}
	}

	type latestTickRuns struct {
		tickID int
		runs   []GameCheckerRun
	}

	accumulators := make(map[int]*latestTickRuns, len(states))
	limit := len(states) * 3
	if limit < 25 {
		limit = 25
	}
	offset := 0

	for len(remaining) > 0 {
		page, err := s.gameCore.CheckerRuns(ctx, GameCheckerRunQuery{
			TeamID: teamID,
			Limit:  limit,
			Offset: offset,
		})
		if err != nil {
			return states
		}
		if len(page.Items) == 0 {
			break
		}

		for _, run := range page.Items {
			if _, ok := remaining[run.ChallengeID]; !ok {
				continue
			}
			accumulator := accumulators[run.ChallengeID]
			if accumulator == nil {
				accumulator = &latestTickRuns{tickID: run.TickID}
				accumulators[run.ChallengeID] = accumulator
			}
			if run.TickID == accumulator.tickID {
				accumulator.runs = append(accumulator.runs, run)
				continue
			}
			applySLASummary(states, run.ChallengeID, summarizeSLARuns(accumulator.runs, accumulator.tickID))
			delete(remaining, run.ChallengeID)
		}

		if !page.HasNext {
			break
		}
		offset += len(page.Items)
	}

	for challengeID := range remaining {
		if accumulator := accumulators[challengeID]; accumulator != nil && len(accumulator.runs) > 0 {
			applySLASummary(states, challengeID, summarizeSLARuns(accumulator.runs, accumulator.tickID))
		}
	}
	return states
}

func applySLASummary(states []serviceState, challengeID int, summary GameServiceStateSummary, extraMarkers ...string) {
	for i := range states {
		if states[i].ChallengeID != challengeID {
			continue
		}
		states[i].SLAStatus = summary.Status
		states[i].SLAPhase = summary.Phase
		states[i].SLATickID = summary.TickID
		states[i].SLAMessage = sanitizeParticipantSLAMessage(summary, extraMarkers...)

		switch strings.ToLower(strings.TrimSpace(summary.Status)) {
		case "ok", "recovering":
			states[i].Status = "stable"
			states[i].Checker = "passing"
		case "down", "faulty", "flag_not_found":
			states[i].Status = "degraded"
			states[i].Checker = "warning"
		}
		return
	}
}

func summarizeSLARuns(runs []GameCheckerRun, tickID int) GameServiceStateSummary {
	for i := range runs {
		run := runs[i]
		if strings.TrimSpace(run.ServiceState) == "" {
			continue
		}
		return GameServiceStateSummary{
			Status:  strings.TrimSpace(run.ServiceState),
			Phase:   strings.TrimSpace(run.StatePhase),
			TickID:  tickID,
			Message: strings.TrimSpace(run.StateMessage),
		}
	}
	return SummarizeCheckerRunsForTick(runs, tickID)
}

func phaseOrder(phase string) int {
	switch strings.ToLower(strings.TrimSpace(phase)) {
	case "put":
		return 0
	case "get":
		return 1
	case "check":
		return 2
	default:
		return 99
	}
}

func fallbackSLAStatus(checker string) string {
	if strings.EqualFold(strings.TrimSpace(checker), "warning") {
		return "down"
	}
	if strings.TrimSpace(checker) == "" {
		return "unknown"
	}
	return "ok"
}

func fallbackSLAMessage(checker string) string {
	if strings.EqualFold(strings.TrimSpace(checker), "warning") {
		return "checker warning; service state detail unavailable"
	}
	if strings.TrimSpace(checker) == "" {
		return "awaiting first checker run"
	}
	return "checker passing; service state detail unavailable"
}

func (s *Server) sanitizeParticipantServiceStates(ctx context.Context, states []serviceState) {
	markers := s.sensitiveCheckerMarkers(ctx)
	for i := range states {
		if !containsSensitiveCheckerDetail(states[i].SLAMessage, markers...) {
			continue
		}
		states[i].SLAMessage = sanitizeParticipantSLAMessage(GameServiceStateSummary{
			Status:  states[i].SLAStatus,
			Phase:   states[i].SLAPhase,
			TickID:  states[i].SLATickID,
			Message: states[i].SLAMessage,
		}, markers...)
	}
}

func sanitizeParticipantSLAMessage(summary GameServiceStateSummary, extraMarkers ...string) string {
	message := strings.TrimSpace(summary.Message)
	if message != "" && !containsSensitiveCheckerDetail(message, extraMarkers...) {
		return message
	}
	switch strings.ToLower(strings.TrimSpace(summary.Phase)) {
	case "put":
		return "checker could not store the flag in the service"
	case "get":
		return "checker could not retrieve the stored flag"
	case "check":
		return "service functionality check failed"
	}
	switch strings.ToLower(strings.TrimSpace(summary.Status)) {
	case "ok":
		return "service passed storage, retrieval, and functionality checks"
	case "recovering":
		return "flag storage failed but retrieval and functionality still passed"
	case "flag_not_found":
		return "checker could not retrieve the stored flag"
	case "faulty":
		return "service functionality check failed"
	case "down":
		return "service did not complete the latest checker cycle"
	}
	return "service state detail unavailable"
}

func (s *Server) sensitiveCheckerMarkers(ctx context.Context) []string {
	markers := []string{}
	if s == nil || s.store == nil {
		return markers
	}
	settings, err := s.store.GetPlatformSettings(ctx)
	if err != nil {
		return markers
	}
	active := strings.ToLower(strings.TrimSpace(settings.FlagFormatActive))
	if active == "" {
		active = strings.ToLower(strings.TrimSpace(settings.FlagFormatPrefix))
	}
	if active != "" && active != "playit" {
		markers = append(markers, active+"{")
	}
	return markers
}

func containsSensitiveCheckerDetail(message string, extraMarkers ...string) bool {
	lowered := strings.ToLower(message)
	markers := []string{
		"ad_flag=",
		"ad_checker_token=",
		"ad_metadata=",
		"docker run",
		"playit{",
		"--entrypoint",
		"checker_token",
	}
	for _, marker := range extraMarkers {
		trimmed := strings.ToLower(strings.TrimSpace(marker))
		if trimmed != "" {
			markers = append(markers, trimmed)
		}
	}
	for _, marker := range markers {
		if strings.Contains(lowered, marker) {
			return true
		}
	}
	return false
}

func matchSubmissionState(match *GameMatchStatus) string {
	if match == nil {
		return "running"
	}
	switch strings.TrimSpace(match.State) {
	case "finished":
		return "finished"
	case "paused":
		return "paused"
	case "running":
		if match.AcceptingSubmissions {
			return "running"
		}
	}
	return "not_started"
}

func (s *Server) handleUnlock(w http.ResponseWriter, r *http.Request) {
	teamID, ok := s.requireTeamAuth(w, r, "please authenticate before unlock.")
	if !ok {
		return
	}
	challengeID, ok := parseChallengeID(w, r)
	if !ok {
		return
	}
	if err := s.store.ValidateServiceAction(r.Context(), teamID, challengeID); err != nil {
		writeDomainFailure(w, err)
		return
	}

	decision, allowed := s.allowRateLimit(r.Context(), rateLimitTeamChallengeKey("unlock", teamID, challengeID), unlockRateLimitPolicy)
	if !allowed {
		writeRateLimitFailure(w, decision, defaultRateLimit429Message)
		return
	}

	var req unlockRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil || strings.TrimSpace(req.Proof) == "" {
		writeProblem(w, http.StatusBadRequest, "Invalid request", "unlock proof is invalid.")
		return
	}
	epoch, epochErr := s.store.GetChallengeUnlockProofEpoch(r.Context(), challengeID)
	if epochErr != nil {
		writeDomainFailure(w, epochErr)
		return
	}
	if !unlockproof.Verify(s.unlockProofSecret, teamID, challengeID, epoch, req.Proof) {
		writeProblem(w, http.StatusBadRequest, "Unlock rejected", "unlock proof is invalid.")
		return
	}

	data, err := s.store.UnlockService(r.Context(), teamID, challengeID)
	if err != nil {
		writeDomainFailure(w, err)
		return
	}
	if err := s.controller.ReconcileServiceAccess(r.Context(), teamID, challengeID); err != nil {
		writeProblem(w, http.StatusBadGateway, "Unlock unavailable", "service unlock access reconcile failed.")
		return
	}
	password := stableRootPassword(s.sshCredentialSecret, teamID, challengeID)
	if err := s.controller.ApplySSHCredential(r.Context(), teamID, challengeID, ControllerSSHCredential{
		Password: password,
	}); err != nil {
		_ = s.store.MarkSSHSessionApplyFailure(r.Context(), teamID, challengeID)
		writeProblem(w, http.StatusBadGateway, "SSH access unavailable", "ssh credential runtime apply failed.")
		return
	}
	s.recordTeamAudit(r.Context(), teamID, "service.unlock", "service", auditServiceTarget(teamID, challengeID), "unlocked service", map[string]any{
		"challenge_id": challengeID,
	})
	writeData(w, http.StatusOK, data)
}

func (s *Server) handleSSHSession(w http.ResponseWriter, r *http.Request) {
	teamID, ok := s.requireTeamAuth(w, r, "please authenticate before requesting ssh access.")
	if !ok {
		return
	}
	challengeID, ok := parseChallengeID(w, r)
	if !ok {
		return
	}
	decision, allowed := s.allowRateLimit(r.Context(), rateLimitTeamChallengeKey("ssh-session", teamID, challengeID), sshSessionRateLimitPolicy)
	if !allowed {
		writeRateLimitFailure(w, decision, defaultRateLimit429Message)
		return
	}
	if err := s.store.ValidateServiceAction(r.Context(), teamID, challengeID); err != nil {
		writeDomainFailure(w, err)
		return
	}
	data, err := s.store.CreateSSHSession(r.Context(), teamID, challengeID, s.now())
	if err != nil {
		writeDomainFailure(w, err)
		return
	}
	data.Password = stableRootPassword(s.sshCredentialSecret, teamID, challengeID)
	data.PasswordMode = "stable"
	if err := s.controller.ApplySSHCredential(r.Context(), teamID, challengeID, ControllerSSHCredential{
		Password: data.Password,
	}); err != nil {
		_ = s.store.MarkSSHSessionApplyFailure(r.Context(), teamID, challengeID)
		writeProblem(w, http.StatusBadGateway, "SSH access unavailable", "ssh credential runtime apply failed.")
		return
	}
	s.recordTeamAudit(r.Context(), teamID, "service.ssh_session", "service", auditServiceTarget(teamID, challengeID), "retrieved stable team ssh credential", map[string]any{
		"challenge_id": challengeID,
		"host":         data.Host,
		"port":         data.Port,
	})
	writeData(w, http.StatusOK, data)
}

func (s *Server) handleFactoryReset(w http.ResponseWriter, r *http.Request) {
	teamID, ok := s.requireTeamAuth(w, r, "please authenticate before reset.")
	if !ok {
		return
	}
	challengeID, ok := parseChallengeID(w, r)
	if !ok {
		return
	}
	decision, allowed := s.allowRateLimit(r.Context(), rateLimitTeamChallengeKey("factory-reset", teamID, challengeID), factoryResetRateLimitPolicy)
	if !allowed {
		writeRateLimitFailure(w, decision, defaultRateLimit429Message)
		return
	}
	if err := s.store.ValidateServiceAction(r.Context(), teamID, challengeID); err != nil {
		writeDomainFailure(w, err)
		return
	}
	prepared, err := s.store.PrepareFactoryResetService(r.Context(), teamID, challengeID)
	if err != nil {
		writeDomainFailure(w, err)
		return
	}
	if err := s.controller.FactoryResetService(r.Context(), teamID, challengeID); err != nil {
		_ = s.store.MarkFactoryResetFailure(r.Context(), teamID, challengeID)
		if errors.Is(err, ErrChallengeNotFound) {
			writeDomainFailure(w, err)
			return
		}
		writeProblem(w, http.StatusBadGateway, "Reset unavailable", "factory reset runtime failed.")
		return
	}
	data, err := s.store.CompleteFactoryResetService(r.Context(), teamID, challengeID)
	if err != nil {
		writeDomainFailure(w, err)
		return
	}
	s.recordTeamAudit(r.Context(), teamID, "service.factory_reset", "service", auditServiceTarget(teamID, challengeID), "triggered factory reset", map[string]any{
		"challenge_id":     challengeID,
		"unlock_preserved": prepared.UnlockPreserved,
	})
	writeData(w, http.StatusOK, data)
}

func (s *Server) handleRestart(w http.ResponseWriter, r *http.Request) {
	teamID, ok := s.requireTeamAuth(w, r, "please authenticate before restart.")
	if !ok {
		return
	}
	challengeID, ok := parseChallengeID(w, r)
	if !ok {
		return
	}
	decision, allowed := s.allowRateLimit(r.Context(), rateLimitTeamChallengeKey("restart", teamID, challengeID), restartRateLimitPolicy)
	if !allowed {
		writeRateLimitFailure(w, decision, defaultRateLimit429Message)
		return
	}
	if err := s.store.ValidateServiceAction(r.Context(), teamID, challengeID); err != nil {
		writeDomainFailure(w, err)
		return
	}
	if err := s.controller.RestartService(r.Context(), teamID, challengeID); err != nil {
		if errors.Is(err, ErrChallengeNotFound) {
			writeDomainFailure(w, err)
			return
		}
		writeProblem(w, http.StatusBadGateway, "Restart unavailable", "service restart runtime failed.")
		return
	}
	data, err := s.store.RestartService(r.Context(), teamID, challengeID)
	if err != nil {
		writeDomainFailure(w, err)
		return
	}
	s.recordTeamAudit(r.Context(), teamID, "service.restart", "service", auditServiceTarget(teamID, challengeID), "triggered service restart", map[string]any{
		"challenge_id": challengeID,
	})
	writeData(w, http.StatusOK, data)
}

func (s *Server) requireTeamAuth(w http.ResponseWriter, r *http.Request, message string) (int, bool) {
	player, ok := s.requirePlayerAuth(w, r, message)
	if !ok {
		return 0, false
	}
	if strings.EqualFold(strings.TrimSpace(player.Role), "organizer") {
		writeProblem(w, http.StatusForbidden, "Authentication required", message)
		return 0, false
	}
	if player.TeamID <= 0 {
		writeProblem(w, http.StatusForbidden, "Team membership required", "please join a team before accessing event resources.")
		return 0, false
	}
	return player.TeamID, true
}

func (s *Server) requirePlayerAuth(w http.ResponseWriter, r *http.Request, message string) (authenticatedPlayer, bool) {
	token, ok := httpapi.BearerToken(r)
	if !ok {
		writeProblem(w, http.StatusForbidden, "Authentication required", message)
		return authenticatedPlayer{}, false
	}
	claims, err := verifyTeamJWT(s.teamTokenSecret, token, s.now())
	if err != nil {
		writeProblem(w, http.StatusForbidden, "Authentication required", message)
		return authenticatedPlayer{}, false
	}
	player, err := s.store.ValidatePlayerSession(r.Context(), claims.PlayerID, claims.TeamID, claims.Role, claims.SessionVersion)
	if err != nil {
		if errors.Is(err, ErrAccountDeactivated) {
			writeProblem(w, http.StatusForbidden, "Access deactivated", "Your account or team has been deactivated by the organizers. Contact them if you believe this is a mistake.")
			return authenticatedPlayer{}, false
		}
		writeProblem(w, http.StatusForbidden, "Authentication required", message)
		return authenticatedPlayer{}, false
	}
	return player, true
}

func (s *Server) requireAdminAuth(w http.ResponseWriter, r *http.Request) bool {
	token, ok := httpapi.BearerToken(r)
	if !ok || subtle.ConstantTimeCompare([]byte(token), []byte(s.adminToken)) != 1 {
		writeProblem(w, http.StatusForbidden, "Authentication required", "please authenticate as organizer.")
		return false
	}
	return true
}

func parseChallengeID(w http.ResponseWriter, r *http.Request) (int, bool) {
	challengeID, err := strconv.Atoi(r.PathValue("challenge_id"))
	if err != nil || challengeID <= 0 {
		writeProblem(w, http.StatusBadRequest, "Invalid request", "challenge id is invalid.")
		return 0, false
	}
	return challengeID, true
}

func parseTeamID(w http.ResponseWriter, r *http.Request) (int, bool) {
	teamID, err := strconv.Atoi(r.PathValue("team_id"))
	if err != nil || teamID <= 0 {
		writeProblem(w, http.StatusBadRequest, "Invalid request", "team id is invalid.")
		return 0, false
	}
	return teamID, true
}

func parsePlayerID(w http.ResponseWriter, r *http.Request) (int, bool) {
	playerID, err := strconv.Atoi(r.PathValue("player_id"))
	if err != nil || playerID <= 0 {
		writeProblem(w, http.StatusBadRequest, "Invalid request", "player id is invalid.")
		return 0, false
	}
	return playerID, true
}

func parseDeploymentID(w http.ResponseWriter, r *http.Request) (int, bool) {
	deploymentID, err := strconv.Atoi(r.PathValue("deployment_id"))
	if err != nil || deploymentID <= 0 {
		writeProblem(w, http.StatusBadRequest, "Invalid request", "deployment job id is invalid.")
		return 0, false
	}
	return deploymentID, true
}

func writeDomainFailure(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrServiceLocked):
		writeProblem(w, http.StatusBadRequest, "Request rejected", "service is not unlocked yet.")
	case errors.Is(err, ErrServiceUnavailable):
		writeProblem(w, http.StatusBadRequest, "Request rejected", "service is not available yet.")
	case errors.Is(err, ErrChallengeMaintenance):
		writeProblem(w, http.StatusServiceUnavailable, "Challenge maintenance", "challenge is under maintenance.")
	case errors.Is(err, ErrChallengeDeferred):
		writeProblem(w, http.StatusServiceUnavailable, "Challenge deferred", "challenge resumes on the next tick.")
	case errors.Is(err, ErrMatchNotStarted):
		writeProblem(w, http.StatusServiceUnavailable, "Match not started", "contest has not started yet.")
	case errors.Is(err, ErrMatchPaused):
		writeProblem(w, http.StatusServiceUnavailable, "Match paused", "contest is temporarily paused.")
	case errors.Is(err, ErrChallengeNotFound):
		writeProblem(w, http.StatusBadRequest, "Invalid request", "challenge id is invalid.")
	case errors.Is(err, ErrTeamNotFound):
		writeProblem(w, http.StatusBadRequest, "Invalid request", "team id is invalid.")
	case errors.Is(err, ErrPlayerNotFound):
		writeProblem(w, http.StatusBadRequest, "Invalid request", "player id is invalid.")
	case errors.Is(err, ErrDeploymentNotFound):
		writeProblem(w, http.StatusBadRequest, "Invalid request", "deployment job id is invalid.")
	case errors.Is(err, ErrDeploymentActive):
		writeProblem(w, http.StatusBadRequest, "Request rejected", "deployment job is still active.")
	case errors.Is(err, ErrDuplicateResource):
		writeProblem(w, http.StatusConflict, "Duplicate resource", "resource already exists.")
	case errors.Is(err, ErrTeamMemberLimit):
		writeProblem(w, http.StatusBadRequest, "Request rejected", "team has reached the maximum member count.")
	case errors.Is(err, ErrTeamAddressPool), errors.Is(err, ErrPlayerAddressPool):
		writeProblem(w, http.StatusConflict, "Address pool exhausted", "the configured private network address pool has no remaining capacity.")
	case errors.Is(err, ErrInvalidRuntimeConfig):
		writeProblem(w, http.StatusBadRequest, "Invalid runtime configuration", err.Error())
	default:
		writeStoreFailure(w, err)
	}
}

func writeStoreFailure(w http.ResponseWriter, err error) {
	if err != nil {
		log.Printf("store failure: %v", err)
	}
	writeProblem(w, http.StatusInternalServerError, "Internal state unavailable", "internal platform state is unavailable.")
}

func (s *Server) reconcileWireGuardGateway(ctx context.Context) error {
	if s.wireGuard == nil {
		return nil
	}
	_, err := s.wireGuard.Reconcile(ctx)
	if errors.Is(err, errWireGuardGatewayDisabled) {
		return nil
	}
	return err
}

func (s *Server) reconcileWireGuardGatewayBestEffort(ctx context.Context, reason string) {
	if err := s.reconcileWireGuardGateway(ctx); err != nil {
		log.Printf("wireguard gateway reconcile after %s failed: %v", reason, err)
	}
}

func (s *Server) handleRateLimitMetrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	s.rateLimitMetrics.WritePrometheus(w)
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
