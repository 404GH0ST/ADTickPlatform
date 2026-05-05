package apigateway

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"adplatform/internal/platform/httpapi"
)

func (s *Server) handleAdminListTeams(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	teams, err := s.store.ListAdminTeams(r.Context())
	if err != nil {
		writeStoreFailure(w, err)
		return
	}
	writeData(w, http.StatusOK, teams)
}

func (s *Server) handleAdminCreateTeam(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	var req adminCreateTeamRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil || strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.ContactEmail) == "" {
		writeProblem(w, http.StatusBadRequest, "Invalid request", "team request is invalid.")
		return
	}
	team, err := s.store.CreateAdminTeam(r.Context(), req)
	if err != nil {
		writeDomainFailure(w, err)
		return
	}
	s.recordAdminAudit(r.Context(), "team.create", "team", fmt.Sprintf("team:%d %s", team.ID, team.Name), "created team", map[string]any{
		"team_id":       team.ID,
		"contact_email": team.ContactEmail,
	})
	writeData(w, http.StatusOK, team)
}

func (s *Server) handleAdminDeleteTeam(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	teamID, ok := parseTeamID(w, r)
	if !ok {
		return
	}
	if err := s.store.DeleteAdminTeam(r.Context(), teamID); err != nil {
		writeStoreFailure(w, err)
		return
	}
	// Try to remove services from controller, ignore error if controller is disabled
	_ = s.controller.RemoveTeamServices(r.Context(), teamID)
	// Refresh firewall rules
	_, _ = s.controller.ReconcileAccessPolicies(r.Context())
	_, _ = s.wireGuard.Reconcile(r.Context())

	s.recordAdminAudit(r.Context(), "team.delete", "team", fmt.Sprintf("team:%d", teamID), "deleted team", map[string]any{
		"team_id": teamID,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAdminUpdateTeam(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	teamID, ok := parseTeamID(w, r)
	if !ok {
		return
	}
	var req adminUpdateTeamRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil || strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.ContactEmail) == "" {
		writeProblem(w, http.StatusBadRequest, "Invalid request", "team update request is invalid.")
		return
	}
	team, err := s.store.UpdateAdminTeam(r.Context(), teamID, req)
	if err != nil {
		writeDomainFailure(w, err)
		return
	}
	s.recordAdminAudit(r.Context(), "team.update", "team", fmt.Sprintf("team:%d %s", team.ID, team.Name), "updated team", map[string]any{
		"team_id":       team.ID,
		"name":          team.Name,
		"contact_email": team.ContactEmail,
	})
	writeData(w, http.StatusOK, team)
}

func (s *Server) handleAdminListPlayers(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	players, err := s.store.ListAdminPlayers(r.Context())
	if err != nil {
		writeStoreFailure(w, err)
		return
	}
	writeData(w, http.StatusOK, players)
}

func (s *Server) handleAdminCreatePlayer(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	var req adminCreatePlayerRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil || req.TeamID < 0 || strings.TrimSpace(req.DisplayName) == "" || strings.TrimSpace(req.Email) == "" || strings.TrimSpace(req.Password) == "" {
		writeProblem(w, http.StatusBadRequest, "Invalid request", "player request is invalid.")
		return
	}
	player, err := s.store.CreateAdminPlayer(r.Context(), req, s.now())
	if err != nil {
		writeDomainFailure(w, err)
		return
	}
	s.recordAdminAudit(r.Context(), "player.create", "player", fmt.Sprintf("player:%d %s", player.ID, player.DisplayName), "created player", map[string]any{
		"player_id":   player.ID,
		"team_id":     player.TeamID,
		"email":       player.Email,
		"role":        player.Role,
		"peer":        player.WireGuardPeer,
		"peer_status": player.WireGuardStatus,
	})
	writeData(w, http.StatusOK, player)
}

func (s *Server) handleAdminDeletePlayer(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	playerID, ok := parsePlayerID(w, r)
	if !ok {
		return
	}
	if err := s.store.DeleteAdminPlayer(r.Context(), playerID); err != nil {
		writeStoreFailure(w, err)
		return
	}
	// Refresh firewall rules
	_, _ = s.controller.ReconcileAccessPolicies(r.Context())
	_, _ = s.wireGuard.Reconcile(r.Context())

	s.recordAdminAudit(r.Context(), "player.delete", "player", fmt.Sprintf("player:%d", playerID), "deleted player", map[string]any{
		"player_id": playerID,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAdminUpdatePlayer(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	playerID, ok := parsePlayerID(w, r)
	if !ok {
		return
	}
	var req adminUpdatePlayerRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil || strings.TrimSpace(req.DisplayName) == "" || strings.TrimSpace(req.Email) == "" {
		writeProblem(w, http.StatusBadRequest, "Invalid request", "player update request is invalid.")
		return
	}
	player, err := s.store.UpdateAdminPlayer(r.Context(), playerID, req)
	if err != nil {
		writeDomainFailure(w, err)
		return
	}
	s.recordAdminAudit(r.Context(), "player.update", "player", fmt.Sprintf("player:%d %s", player.ID, player.DisplayName), "updated player", map[string]any{
		"player_id":    player.ID,
		"display_name": player.DisplayName,
		"email":        player.Email,
		"role":         player.Role,
	})
	writeData(w, http.StatusOK, player)
}

func (s *Server) handleAdminGetPlayerWireGuard(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	playerID, ok := parsePlayerID(w, r)
	if !ok {
		return
	}
	peer, err := s.store.GetAdminPlayerWireGuardConfig(r.Context(), playerID)
	if err != nil {
		writeDomainFailure(w, err)
		return
	}
	s.recordAdminAudit(r.Context(), "wireguard.inspect", "player", fmt.Sprintf("player:%d %s", peer.PlayerID, peer.DisplayName), "read WireGuard peer configuration", map[string]any{
		"player_id": peer.PlayerID,
		"team_id":   peer.TeamID,
		"peer":      peer.WireGuardPeer,
		"status":    peer.Status,
	})
	writeData(w, http.StatusOK, peer)
}

func (s *Server) handleAdminRotatePlayerWireGuard(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	playerID, ok := parsePlayerID(w, r)
	if !ok {
		return
	}
	peer, err := s.store.RotateAdminPlayerWireGuardConfig(r.Context(), playerID, s.now())
	if err != nil {
		writeDomainFailure(w, err)
		return
	}
	s.recordAdminAudit(r.Context(), "wireguard.rotate", "player", fmt.Sprintf("player:%d %s", peer.PlayerID, peer.DisplayName), "rotated WireGuard peer", map[string]any{
		"player_id": peer.PlayerID,
		"team_id":   peer.TeamID,
		"peer":      peer.WireGuardPeer,
		"status":    peer.Status,
	})
	writeData(w, http.StatusOK, peer)
}

func (s *Server) handleAdminRevokePlayerWireGuard(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	playerID, ok := parsePlayerID(w, r)
	if !ok {
		return
	}
	peer, err := s.store.RevokeAdminPlayerWireGuardConfig(r.Context(), playerID, s.now())
	if err != nil {
		writeDomainFailure(w, err)
		return
	}
	s.recordAdminAudit(r.Context(), "wireguard.revoke", "player", fmt.Sprintf("player:%d %s", peer.PlayerID, peer.DisplayName), "revoked WireGuard peer", map[string]any{
		"player_id": peer.PlayerID,
		"team_id":   peer.TeamID,
		"peer":      peer.WireGuardPeer,
		"status":    peer.Status,
	})
	writeData(w, http.StatusOK, peer)
}

func (s *Server) handleAdminWireGuardGatewayStatus(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	status, err := s.wireGuard.Status(r.Context())
	if err != nil {
		writeProblem(w, http.StatusBadGateway, "WireGuard unavailable", "wireguard gateway status could not be read.")
		return
	}
	writeData(w, http.StatusOK, status)
}

func (s *Server) handleAdminWireGuardGatewayReconcile(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	status, err := s.wireGuard.Reconcile(r.Context())
	if err != nil {
		message := "wireguard gateway reconcile failed."
		statusCode := http.StatusBadGateway
		if errors.Is(err, errWireGuardGatewayDisabled) {
			statusCode = http.StatusServiceUnavailable
			message = "wireguard gateway is not configured."
		}
		writeProblem(w, statusCode, "WireGuard unavailable", message)
		return
	}
	s.recordAdminAudit(r.Context(), "wireguard.reconcile", "wireguard_gateway", "wireguard-gateway", "reconciled WireGuard gateway state", map[string]any{
		"mode":         status.Mode,
		"state":        status.State,
		"peers_total":  status.PeersTotal,
		"peers_active": status.PeersActive,
	})
	writeData(w, http.StatusOK, status)
}

func (s *Server) handleAdminWireGuardGatewayTeardown(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	if err := s.wireGuard.Teardown(r.Context()); err != nil {
		writeProblem(w, http.StatusBadGateway, "WireGuard unavailable", "wireguard gateway teardown failed.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAdminAccessStatus(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	status, err := s.controller.AccessStatus(r.Context())
	if err != nil {
		writeProblem(w, http.StatusBadGateway, "Controller access unavailable", "controller access status could not be read.")
		return
	}
	writeData(w, http.StatusOK, status)
}

func (s *Server) handleAdminAccessReconcile(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	status, err := s.controller.ReconcileAccessPolicies(r.Context())
	if err != nil {
		writeProblem(w, http.StatusBadGateway, "Controller access unavailable", "controller access reconcile failed.")
		return
	}
	s.recordAdminAudit(r.Context(), "access.reconcile", "controller_access", "controller-access", "reconciled controller access policies", map[string]any{
		"mode":                status.Mode,
		"policies_total":      status.PoliciesTotal,
		"ssh_open_services":   status.SSHOpenServices,
		"ssh_locked_services": status.SSHLockedServices,
		"allowed_peers_total": status.AllowedPeersTotal,
	})
	writeData(w, http.StatusOK, status)
}

func (s *Server) handleAdminAccessTeardown(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	if err := s.controller.TeardownAccessPolicies(r.Context()); err != nil {
		writeProblem(w, http.StatusBadGateway, "Controller access unavailable", "controller access teardown failed.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAdminListChallenges(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	challenges, err := s.store.ListAdminChallenges(r.Context())
	if err != nil {
		writeStoreFailure(w, err)
		return
	}
	writeData(w, http.StatusOK, challenges)
}

func (s *Server) handleAdminCreateChallenge(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	var req adminCreateChallengeRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil || strings.TrimSpace(req.Name) == "" {
		writeProblem(w, http.StatusBadRequest, "Invalid request", "challenge request is invalid.")
		return
	}
	if err := validateChallengeSourceReference(req.SourceBundlePath); err != nil {
		writeProblem(w, http.StatusBadRequest, "Invalid source path", "challenge source path is invalid.")
		return
	}
	challenge, err := s.store.CreateAdminChallenge(r.Context(), req, s.now())
	if err != nil {
		writeDomainFailure(w, err)
		return
	}
	s.recordAdminAudit(r.Context(), "challenge.create", "challenge", auditChallengeTarget(challenge.ID, challenge.Name), "created challenge", map[string]any{
		"challenge_id":         challenge.ID,
		"baseline_image":       challenge.BaselineImage,
		"checker_image":        challenge.CheckerImage,
		"service_port":         challenge.ServicePort,
		"service_subnet_octet": challenge.ServiceSubnetOctet,
		"weight":               challenge.Weight,
	})
	writeData(w, http.StatusOK, challenge)
}

func (s *Server) handleAdminDeleteChallenge(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	challengeID, ok := parseChallengeID(w, r)
	if !ok {
		return
	}
	if err := s.store.DeleteAdminChallenge(r.Context(), challengeID); err != nil {
		writeStoreFailure(w, err)
		return
	}
	// Try to remove services from controller, ignore error if controller is disabled
	_ = s.controller.RemoveChallengeServices(r.Context(), challengeID)
	// Refresh firewall rules
	_, _ = s.controller.ReconcileAccessPolicies(r.Context())

	s.recordAdminAudit(r.Context(), "challenge.delete", "challenge", fmt.Sprintf("challenge:%d", challengeID), "deleted challenge", map[string]any{
		"challenge_id": challengeID,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAdminUpdateChallenge(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	challengeID, ok := parseChallengeID(w, r)
	if !ok {
		return
	}
	var req adminUpdateChallengeRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil || strings.TrimSpace(req.Name) == "" {
		writeProblem(w, http.StatusBadRequest, "Invalid request", "challenge update request is invalid.")
		return
	}
	if err := validateChallengeSourceReference(req.SourceBundlePath); err != nil {
		writeProblem(w, http.StatusBadRequest, "Invalid source path", "challenge source path is invalid.")
		return
	}
	challenge, err := s.store.UpdateAdminChallenge(r.Context(), challengeID, req)
	if err != nil {
		writeDomainFailure(w, err)
		return
	}
	s.recordAdminAudit(r.Context(), "challenge.update", "challenge", auditChallengeTarget(challenge.ID, challenge.Name), "updated challenge", map[string]any{
		"challenge_id":   challenge.ID,
		"name":           challenge.Name,
		"baseline_image": challenge.BaselineImage,
		"checker_image":  challenge.CheckerImage,
		"weight":         challenge.Weight,
	})
	writeData(w, http.StatusOK, challenge)
}

func (s *Server) handleAdminValidateChallenge(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	challengeID, ok := parseChallengeID(w, r)
	if !ok {
		return
	}
	result, err := s.validateChallengeRuntime(r.Context(), challengeID)
	if err != nil {
		if errors.Is(err, ErrChallengeNotFound) {
			writeDomainFailure(w, err)
			return
		}
		writeProblem(w, http.StatusBadGateway, "Challenge validation unavailable", "challenge runtime validation failed.")
		return
	}
	s.recordAdminAudit(r.Context(), "challenge.validate", "challenge", auditChallengeTarget(result.ChallengeID, result.Name), "validated challenge runtime", map[string]any{
		"challenge_id":             result.ChallengeID,
		"status":                   result.Status,
		"baseline_ssh_contract_ok": result.BaselineSSHContractOK,
		"checker_contract_ok":      result.CheckerContractOK,
	})
	writeData(w, http.StatusOK, result)
}

func (s *Server) handleAdminDeployChallenge(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	challengeID, ok := parseChallengeID(w, r)
	if !ok {
		return
	}
	validation, err := s.validateChallengeRuntime(r.Context(), challengeID)
	if err != nil {
		if errors.Is(err, ErrChallengeNotFound) {
			writeDomainFailure(w, err)
			return
		}
		writeProblem(w, http.StatusBadGateway, "Challenge deployment unavailable", "challenge runtime validation failed.")
		return
	}
	if validation.Status != "valid" || !validation.BaselineSSHContractOK || !validation.CheckerContractOK {
		message := strings.TrimSpace(validation.Message)
		if message == "" {
			message = "challenge package failed runtime validation."
		}
		writeProblem(w, http.StatusBadRequest, "Invalid runtime configuration", message)
		return
	}
	deployment, err := s.store.DeployAdminChallenge(r.Context(), challengeID)
	if err != nil {
		writeDomainFailure(w, err)
		return
	}
	s.recordAdminAudit(r.Context(), "challenge.deploy", "challenge", auditChallengeTarget(deployment.ChallengeID, deployment.ChallengeName), "queued challenge deployment", map[string]any{
		"job_id":            deployment.JobID,
		"queued_team_count": deployment.QueuedTeamCount,
		"ready_team_count":  deployment.ReadyTeamCount,
		"total_team_count":  deployment.TotalTeamCount,
	})
	writeData(w, http.StatusOK, deployment)
}

func (s *Server) handleAdminListDeployments(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	deployments, err := s.store.ListAdminDeployments(r.Context())
	if err != nil {
		writeStoreFailure(w, err)
		return
	}
	writeData(w, http.StatusOK, deployments)
}

func (s *Server) handleAdminDeleteDeployment(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	deploymentID, ok := parseDeploymentID(w, r)
	if !ok {
		return
	}
	if err := s.store.DeleteAdminDeployment(r.Context(), deploymentID); err != nil {
		writeDomainFailure(w, err)
		return
	}
	s.recordAdminAudit(r.Context(), "deployment.delete", "deployment", fmt.Sprintf("deployment-job:%d", deploymentID), "deleted deployment job", map[string]any{
		"deployment_job_id": deploymentID,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAdminListAuditLogs(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	page, err := s.store.ListAdminAuditLogs(r.Context(), parseAdminAuditLogQuery(r))
	if err != nil {
		writeStoreFailure(w, err)
		return
	}
	writeData(w, http.StatusOK, page)
}

func (s *Server) handleAdminReconcileDeployments(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	result, err := s.controller.ReconcileDeployments(r.Context())
	if err != nil {
		if errors.Is(err, errControllerDisabled) {
			writeProblem(w, http.StatusServiceUnavailable, "Deployment reconcile unavailable", "controller deployment reconcile is not configured.")
			return
		}
		var problemErr *controllerProblemError
		if errors.As(err, &problemErr) {
			writeProblem(w, problemErr.StatusCode(), problemErr.Title(), problemErr.Detail())
			return
		}
		writeProblem(w, http.StatusBadGateway, "Deployment reconcile unavailable", "controller deployment reconcile failed.")
		return
	}
	s.recordAdminAudit(r.Context(), "deployment.reconcile", "deployment", "deployment-jobs", "reconciled deployment jobs", map[string]any{
		"processed_jobs":      result.ProcessedJobs,
		"processed_instances": result.ProcessedInstances,
		"completed_jobs":      result.CompletedJobs,
	})
	writeData(w, http.StatusOK, result)
}

func (s *Server) handleAdminGameStatus(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	status, err := s.gameCore.Status(r.Context())
	if err != nil {
		writeGameCoreFailure(w, err, "game-core status could not be read.")
		return
	}
	writeData(w, http.StatusOK, status)
}

func (s *Server) handleAdminGameMatchStatus(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	status, err := s.gameCore.MatchStatus(r.Context())
	if err != nil {
		writeGameCoreFailure(w, err, "game-core match status could not be read.")
		return
	}
	writeData(w, http.StatusOK, status)
}

func (s *Server) handleAdminStartGameMatch(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	status, err := s.gameCore.StartMatch(r.Context())
	if err != nil {
		writeGameCoreFailure(w, err, "game-core match start failed.")
		return
	}
	s.recordAdminAudit(r.Context(), "match.start", "match", "primary-match", "started match", map[string]any{
		"state":                 status.State,
		"accepting_submissions": status.AcceptingSubmissions,
	})
	writeData(w, http.StatusOK, status)
}

func (s *Server) handleAdminStopGameMatch(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	status, err := s.gameCore.StopMatch(r.Context())
	if err != nil {
		writeGameCoreFailure(w, err, "game-core match stop failed.")
		return
	}
	s.recordAdminAudit(r.Context(), "match.stop", "match", "primary-match", "stopped match", map[string]any{
		"state":                 status.State,
		"accepting_submissions": status.AcceptingSubmissions,
	})
	writeData(w, http.StatusOK, status)
}

func (s *Server) handleAdminUpdateGameMatchSchedule(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}

	var req UpdateMatchScheduleRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		writeProblem(w, http.StatusBadRequest, "Invalid request", "could not parse match schedule payload.")
		return
	}

	status, err := s.gameCore.UpdateMatchSchedule(r.Context(), req)
	if err != nil {
		writeGameCoreFailure(w, err, "game-core match schedule update failed.")
		return
	}

	s.recordAdminAudit(r.Context(), "match.schedule.update", "match", "primary-match", "updated match schedule", map[string]any{
		"scheduled_start_at":    status.ScheduledStartAt,
		"scheduled_end_at":      status.ScheduledEndAt,
		"state":                 status.State,
		"accepting_submissions": status.AcceptingSubmissions,
	})
	writeData(w, http.StatusOK, status)
}

func (s *Server) handleAdminAdvanceGameTick(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	status, err := s.gameCore.AdvanceTick(r.Context())
	if err != nil {
		writeGameCoreFailure(w, err, "game-core tick advance failed.")
		return
	}
	s.recordAdminAudit(r.Context(), "tick.advance", "tick", fmt.Sprintf("tick:%d", status.ID), "advanced authoritative tick", map[string]any{
		"tick_id":                 status.ID,
		"status":                  status.Status,
		"total_checker_runs":      status.TotalCheckerRuns,
		"successful_checker_runs": status.SuccessfulCheckerRuns,
		"failed_checker_runs":     status.FailedCheckerRuns,
		"skipped_checker_runs":    status.SkippedCheckerRuns,
	})
	writeData(w, http.StatusOK, status)
}

func (s *Server) handleAdminListCheckerRuns(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	query := parseAdminCheckerRunQuery(r)
	runs, err := s.gameCore.CheckerRuns(r.Context(), query)
	if err != nil {
		writeGameCoreFailure(w, err, "game-core checker runs could not be read.")
		return
	}
	writeData(w, http.StatusOK, runs)
}

func (s *Server) handleAdminGameSchedulerStatus(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	status, err := s.gameCore.SchedulerStatus(r.Context())
	if err != nil {
		writeGameCoreFailure(w, err, "game-core scheduler status could not be read.")
		return
	}
	writeData(w, http.StatusOK, status)
}

func (s *Server) handleAdminGameSchedulerEvents(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	query := parseAdminSchedulerEventQuery(r)
	events, err := s.gameCore.SchedulerEvents(r.Context(), query)
	if err != nil {
		writeGameCoreFailure(w, err, "game-core scheduler events could not be read.")
		return
	}
	writeData(w, http.StatusOK, events)
}

func (s *Server) handleAdminStartGameScheduler(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	status, err := s.gameCore.StartScheduler(r.Context())
	if err != nil {
		writeGameCoreFailure(w, err, "game-core scheduler start failed.")
		return
	}
	s.recordAdminAudit(r.Context(), "scheduler.start", "scheduler", "game-scheduler", "started scheduler", map[string]any{
		"state":            status.State,
		"interval_seconds": status.IntervalSeconds,
		"next_run_at":      status.NextRunAt,
	})
	writeData(w, http.StatusOK, status)
}

func (s *Server) handleAdminStopGameScheduler(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	status, err := s.gameCore.StopScheduler(r.Context())
	if err != nil {
		writeGameCoreFailure(w, err, "game-core scheduler stop failed.")
		return
	}
	s.recordAdminAudit(r.Context(), "scheduler.stop", "scheduler", "game-scheduler", "stopped scheduler", map[string]any{
		"state":            status.State,
		"interval_seconds": status.IntervalSeconds,
		"last_run_at":      status.LastRunAt,
	})
	writeData(w, http.StatusOK, status)
}

func (s *Server) handleAdminUpdateGameScheduler(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}

	var req UpdateSchedulerRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		writeProblem(w, http.StatusBadRequest, "Invalid request", "could not parse interval payload.")
		return
	}

	status, err := s.gameCore.UpdateScheduler(r.Context(), req)
	if err != nil {
		writeGameCoreFailure(w, err, "game-core scheduler update failed.")
		return
	}

	s.recordAdminAudit(r.Context(), "scheduler.update", "scheduler", "game-scheduler", "updated scheduler interval", map[string]any{
		"interval_seconds": status.IntervalSeconds,
		"state":            status.State,
	})
	writeData(w, http.StatusOK, status)
}

func (s *Server) handleAdminGameScoreboard(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	if s.scoring != nil {
		if rows, err := s.scoring.Scoreboard(r.Context()); err == nil {
			writeData(w, http.StatusOK, rows)
			return
		} else if !errors.Is(err, errScoringWorkerDisabled) {
			writeProblem(w, http.StatusBadGateway, "Scoreboard unavailable", "scoring-worker scoreboard could not be read.")
			return
		}
	}
	rows, err := s.gameCore.Scoreboard(r.Context())
	if err != nil {
		writeGameCoreFailure(w, err, "game-core scoreboard could not be read.")
		return
	}
	writeData(w, http.StatusOK, rows)
}

func (s *Server) handleAdminRecomputeScoring(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	if s.scoring != nil {
		if rows, err := s.scoring.RecomputeScoring(r.Context()); err == nil {
			s.recordAdminAudit(r.Context(), "scoring.recompute", "scoreboard", "public-scoreboard", "recomputed scoreboard", map[string]any{
				"rows": len(rows),
			})
			writeData(w, http.StatusOK, rows)
			return
		} else if !errors.Is(err, errScoringWorkerDisabled) {
			writeProblem(w, http.StatusBadGateway, "Scoreboard unavailable", "scoring-worker score recompute failed.")
			return
		}
	}
	rows, err := s.gameCore.RecomputeScoring(r.Context())
	if err != nil {
		writeGameCoreFailure(w, err, "game-core score recompute failed.")
		return
	}
	s.recordAdminAudit(r.Context(), "scoring.recompute", "scoreboard", "public-scoreboard", "recomputed scoreboard", map[string]any{
		"rows": len(rows),
	})
	writeData(w, http.StatusOK, rows)
}

func parseAdminCheckerRunQuery(r *http.Request) GameCheckerRunQuery {
	return GameCheckerRunQuery{
		Limit:       parseAdminPositiveQueryInt(r, "limit", 25, 200),
		Offset:      parseAdminNonNegativeQueryInt(r, "offset"),
		TickID:      parseAdminPositiveQueryInt(r, "tick_id", 0, 0),
		TeamID:      parseAdminPositiveQueryInt(r, "team_id", 0, 0),
		ChallengeID: parseAdminPositiveQueryInt(r, "challenge_id", 0, 0),
		Phase:       strings.ToLower(strings.TrimSpace(r.URL.Query().Get("phase"))),
		Status:      strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status"))),
	}
}

func parseAdminAuditLogQuery(r *http.Request) adminAuditLogQuery {
	return adminAuditLogQuery{
		Limit:      parseAdminPositiveQueryInt(r, "limit", 25, 200),
		Offset:     parseAdminNonNegativeQueryInt(r, "offset"),
		ActorType:  strings.ToLower(strings.TrimSpace(r.URL.Query().Get("actor_type"))),
		Action:     strings.ToLower(strings.TrimSpace(r.URL.Query().Get("action"))),
		TargetType: strings.ToLower(strings.TrimSpace(r.URL.Query().Get("target_type"))),
		Status:     strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status"))),
	}
}

func parseAdminSchedulerEventQuery(r *http.Request) GameSchedulerEventQuery {
	return GameSchedulerEventQuery{
		Limit:     parseAdminPositiveQueryInt(r, "limit", 25, 200),
		Offset:    parseAdminNonNegativeQueryInt(r, "offset"),
		EventType: strings.ToLower(strings.TrimSpace(r.URL.Query().Get("event_type"))),
		Source:    strings.ToLower(strings.TrimSpace(r.URL.Query().Get("source"))),
		State:     strings.ToLower(strings.TrimSpace(r.URL.Query().Get("state"))),
	}
}

func parseAdminPositiveQueryInt(r *http.Request, key string, fallback int, max int) int {
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

func parseAdminNonNegativeQueryInt(r *http.Request, key string) int {
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

func (s *Server) validateChallengeRuntime(ctx context.Context, challengeID int) (ChallengeValidationResult, error) {
	request, err := s.buildChallengeValidationRequest(ctx, challengeID)
	if err != nil {
		return ChallengeValidationResult{}, err
	}
	return s.controller.ValidateChallengeRuntime(ctx, request)
}

func (s *Server) buildChallengeValidationRequest(ctx context.Context, challengeID int) (ChallengeValidationRequest, error) {
	challenges, err := s.store.ListAdminChallenges(ctx)
	if err != nil {
		return ChallengeValidationRequest{}, err
	}
	for _, challenge := range challenges {
		if challenge.ID != challengeID {
			continue
		}
		return ChallengeValidationRequest{
			ChallengeID:   challenge.ID,
			Name:          challenge.Name,
			BaselineImage: challenge.BaselineImage,
			CheckerImage:  challenge.CheckerImage,
		}, nil
	}
	return ChallengeValidationRequest{}, ErrChallengeNotFound
}

func writeGameCoreFailure(w http.ResponseWriter, err error, fallback string) {
	statusCode := http.StatusBadGateway
	message := fallback
	if errors.Is(err, errGameCoreDisabled) {
		statusCode = http.StatusServiceUnavailable
		message = "game-core is not configured."
	} else {
		var httpErr gameCoreHTTPError
		if errors.As(err, &httpErr) {
			if httpErr.StatusCode > 0 {
				statusCode = httpErr.StatusCode
			}
			if trimmed := strings.TrimSpace(httpErr.Message); trimmed != "" {
				message = trimmed
			}
		} else if trimmed := strings.TrimSpace(err.Error()); trimmed != "" {
			message = trimmed
		}
	}
	writeProblem(w, statusCode, "Game-core unavailable", message)
}
