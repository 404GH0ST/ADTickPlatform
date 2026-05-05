package apigateway

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"
)

const (
	operationsDeploymentStallThreshold = 10 * time.Minute
	operationsCheckerStallThreshold    = 2 * time.Minute
	operationsSchedulerOverdueFloor    = 30 * time.Second
)

func (s *Server) handleAdminOperationsStatus(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	writeData(w, http.StatusOK, s.buildAdminOperationsStatus(r.Context()))
}

func (s *Server) buildAdminOperationsStatus(ctx context.Context) AdminOperationsStatus {
	now := s.now().UTC()
	alerts := make([]AdminOperationsAlert, 0, 8)

	gameStatus, err := s.gameCore.Status(ctx)
	if err != nil {
		alerts = append(alerts, AdminOperationsAlert{
			ID:       "game-status-unavailable",
			Severity: "critical",
			Source:   "game",
			Summary:  "Authoritative game status is unavailable.",
			Detail:   strings.TrimSpace(err.Error()),
		})
	} else {
		alerts = append(alerts, evaluateGameOperationsAlerts(gameStatus, now)...)
	}

	deployments, err := s.store.ListAdminDeployments(ctx)
	if err != nil {
		alerts = append(alerts, AdminOperationsAlert{
			ID:       "deployments-unavailable",
			Severity: "warning",
			Source:   "deployments",
			Summary:  "Deployment queue state is unavailable.",
			Detail:   strings.TrimSpace(err.Error()),
		})
	} else {
		alerts = append(alerts, evaluateDeploymentOperationsAlerts(deployments, now)...)
	}

	wireGuardStatus, err := s.wireGuard.Status(ctx)
	if err != nil {
		alerts = append(alerts, AdminOperationsAlert{
			ID:       "wireguard-status-unavailable",
			Severity: "warning",
			Source:   "wireguard",
			Summary:  "WireGuard gateway status is unavailable.",
			Detail:   strings.TrimSpace(err.Error()),
		})
	} else {
		peers, peersErr := s.store.ListWireGuardGatewayPeers(ctx)
		if peersErr != nil {
			alerts = append(alerts, AdminOperationsAlert{
				ID:       "wireguard-peers-unavailable",
				Severity: "warning",
				Source:   "wireguard",
				Summary:  "Expected WireGuard peer state is unavailable.",
				Detail:   strings.TrimSpace(peersErr.Error()),
			})
		}
		alerts = append(alerts, evaluateWireGuardOperationsAlerts(wireGuardStatus, peers)...)
	}

	accessStatus, err := s.controller.AccessStatus(ctx)
	if err != nil {
		alerts = append(alerts, AdminOperationsAlert{
			ID:       "access-status-unavailable",
			Severity: "warning",
			Source:   "access",
			Summary:  "Controller access status is unavailable.",
			Detail:   strings.TrimSpace(err.Error()),
		})
	} else {
		policies, policiesErr := s.store.ListControllerServiceAccessPolicies(ctx)
		if policiesErr != nil {
			alerts = append(alerts, AdminOperationsAlert{
				ID:       "access-policies-unavailable",
				Severity: "warning",
				Source:   "access",
				Summary:  "Expected access policy state is unavailable.",
				Detail:   strings.TrimSpace(policiesErr.Error()),
			})
		}
		alerts = append(alerts, evaluateAccessOperationsAlerts(accessStatus, policies)...)
	}

	slices.SortFunc(alerts, compareOperationsAlerts)
	return AdminOperationsStatus{
		Healthy:     len(alerts) == 0,
		GeneratedAt: now.Format(time.RFC3339),
		Alerts:      alerts,
	}
}

func evaluateGameOperationsAlerts(status GameStatus, now time.Time) []AdminOperationsAlert {
	alerts := make([]AdminOperationsAlert, 0, 4)
	match := status.Match
	scheduler := status.Scheduler
	currentTick := status.CurrentTick
	interval := schedulerInterval(scheduler)

	if match != nil && match.State == "running" {
		switch {
		case scheduler == nil:
			alerts = append(alerts, AdminOperationsAlert{
				ID:       "scheduler-missing",
				Severity: "critical",
				Source:   "scheduler",
				Summary:  "Scheduler state is missing while the match is running.",
			})
		case scheduler.State != "running":
			alerts = append(alerts, AdminOperationsAlert{
				ID:       "scheduler-stopped",
				Severity: "critical",
				Source:   "scheduler",
				Summary:  "Scheduler is stopped while the match is running.",
			})
		}
	}

	if match != nil && match.State == "finished" && scheduler != nil && scheduler.State == "running" {
		alerts = append(alerts, AdminOperationsAlert{
			ID:       "scheduler-running-after-finish",
			Severity: "warning",
			Source:   "scheduler",
			Summary:  "Scheduler is still running after the match finished.",
		})
	}

	if scheduler != nil {
		if strings.TrimSpace(scheduler.LastError) != "" {
			alerts = append(alerts, AdminOperationsAlert{
				ID:       "scheduler-last-error",
				Severity: "warning",
				Source:   "scheduler",
				Summary:  "Scheduler reported a recent error.",
				Detail:   strings.TrimSpace(scheduler.LastError),
			})
		}
		if scheduler.State == "running" {
			if nextRunAt, ok := parseRFC3339UTC(scheduler.NextRunAt); ok {
				overdueGrace := maxDuration(interval, operationsSchedulerOverdueFloor)
				if now.After(nextRunAt.Add(overdueGrace)) {
					alerts = append(alerts, AdminOperationsAlert{
						ID:       "scheduler-overdue",
						Severity: "critical",
						Source:   "scheduler",
						Summary:  "Scheduler next run is overdue.",
						Detail: fmt.Sprintf(
							"Expected the next run around %s with a %s interval.",
							nextRunAt.Format(time.RFC3339),
							humanDuration(interval),
						),
					})
				}
			}
		}
	}

	if currentTick != nil && strings.EqualFold(currentTick.Status, "running") {
		if startedAt, ok := parseRFC3339UTC(currentTick.StartedAt); ok {
			stallThreshold := maxDuration(2*interval, operationsCheckerStallThreshold)
			if now.After(startedAt.Add(stallThreshold)) {
				alerts = append(alerts, AdminOperationsAlert{
					ID:       "checker-stalled",
					Severity: "warning",
					Source:   "checker",
					Summary:  "Checker execution appears stalled on the current tick.",
					Detail: fmt.Sprintf(
						"Tick %d has been running for %s.",
						currentTick.ID,
						humanDuration(now.Sub(startedAt)),
					),
				})
			}
		}
	}

	return alerts
}

func evaluateDeploymentOperationsAlerts(deployments []adminDeploymentJob, now time.Time) []AdminOperationsAlert {
	activeCount := 0
	oldestAge := time.Duration(0)
	oldestStatus := ""
	for _, deployment := range deployments {
		if isTerminalDeploymentStatus(deployment.Status) {
			continue
		}
		activeCount++
		createdAt, ok := parseRFC3339UTC(deployment.CreatedAt)
		if !ok {
			continue
		}
		age := now.Sub(createdAt)
		if age > oldestAge {
			oldestAge = age
			oldestStatus = deployment.Status
		}
	}
	if activeCount == 0 || oldestAge < operationsDeploymentStallThreshold {
		return nil
	}
	return []AdminOperationsAlert{{
		ID:       "deployment-queue-stalled",
		Severity: "warning",
		Source:   "deployments",
		Summary:  fmt.Sprintf("%d deployment job(s) are still active.", activeCount),
		Detail: fmt.Sprintf(
			"The oldest active job has been %s in status %s.",
			humanDuration(oldestAge),
			oldestStatus,
		),
	}}
}

func evaluateWireGuardOperationsAlerts(status WireGuardGatewayStatus, peers []WireGuardGatewayPeer) []AdminOperationsAlert {
	alerts := make([]AdminOperationsAlert, 0, 3)
	if status.State != "" && status.State != "applied" && status.State != "disabled" {
		alerts = append(alerts, AdminOperationsAlert{
			ID:       "wireguard-not-applied",
			Severity: "critical",
			Source:   "wireguard",
			Summary:  "WireGuard gateway is not in an applied state.",
			Detail:   fmt.Sprintf("Current state is %s.", status.State),
		})
	}
	if strings.TrimSpace(status.LastError) != "" {
		alerts = append(alerts, AdminOperationsAlert{
			ID:       "wireguard-last-error",
			Severity: "warning",
			Source:   "wireguard",
			Summary:  "WireGuard gateway reported a recent error.",
			Detail:   strings.TrimSpace(status.LastError),
		})
	}
	if status.State == "applied" && status.Mode != "disabled" && peers != nil {
		expectedActive, expectedRevoked := expectedWireGuardPeerCounts(peers)
		if status.PeersTotal != len(peers) || status.PeersActive != expectedActive || status.PeersRevoked != expectedRevoked {
			alerts = append(alerts, AdminOperationsAlert{
				ID:       "wireguard-peer-drift",
				Severity: "warning",
				Source:   "wireguard",
				Summary:  "WireGuard gateway peer counts differ from the stored peer manifest.",
				Detail: fmt.Sprintf(
					"Applied=%d total/%d active/%d revoked, expected=%d total/%d active/%d revoked.",
					status.PeersTotal,
					status.PeersActive,
					status.PeersRevoked,
					len(peers),
					expectedActive,
					expectedRevoked,
				),
			})
		}
	}
	return alerts
}

func evaluateAccessOperationsAlerts(status ControllerAccessStatus, policies []ControllerServiceAccessPolicy) []AdminOperationsAlert {
	alerts := make([]AdminOperationsAlert, 0, 3)
	if status.State != "" && status.State != "applied" && status.State != "disabled" {
		alerts = append(alerts, AdminOperationsAlert{
			ID:       "access-not-applied",
			Severity: "critical",
			Source:   "access",
			Summary:  "Controller access policies are not in an applied state.",
			Detail:   fmt.Sprintf("Current state is %s.", status.State),
		})
	}
	if strings.TrimSpace(status.LastError) != "" {
		alerts = append(alerts, AdminOperationsAlert{
			ID:       "access-last-error",
			Severity: "warning",
			Source:   "access",
			Summary:  "Controller access policy reconciliation reported a recent error.",
			Detail:   strings.TrimSpace(status.LastError),
		})
	}
	if status.State == "applied" && status.Mode != "disabled" && policies != nil {
		expectedOpen, expectedLocked, expectedAllowed := expectedAccessPolicyCounts(policies)
		if status.PoliciesTotal != len(policies) ||
			status.SSHOpenServices != expectedOpen ||
			status.SSHLockedServices != expectedLocked ||
			status.AllowedPeersTotal != expectedAllowed {
			alerts = append(alerts, AdminOperationsAlert{
				ID:       "access-policy-drift",
				Severity: "warning",
				Source:   "access",
				Summary:  "Applied access policy counts differ from the stored service policy manifest.",
				Detail: fmt.Sprintf(
					"Applied=%d policies/%d SSH-open/%d SSH-locked/%d allowed peers, expected=%d/%d/%d/%d.",
					status.PoliciesTotal,
					status.SSHOpenServices,
					status.SSHLockedServices,
					status.AllowedPeersTotal,
					len(policies),
					expectedOpen,
					expectedLocked,
					expectedAllowed,
				),
			})
		}
	}
	return alerts
}

func compareOperationsAlerts(a, b AdminOperationsAlert) int {
	if severityRank(a.Severity) != severityRank(b.Severity) {
		return severityRank(a.Severity) - severityRank(b.Severity)
	}
	if a.Source != b.Source {
		return strings.Compare(a.Source, b.Source)
	}
	return strings.Compare(a.ID, b.ID)
}

func severityRank(value string) int {
	switch value {
	case "critical":
		return 0
	case "warning":
		return 1
	default:
		return 2
	}
}

func schedulerInterval(status *GameSchedulerStatus) time.Duration {
	if status == nil || status.IntervalSeconds <= 0 {
		return time.Minute
	}
	return time.Duration(status.IntervalSeconds) * time.Second
}

func parseRFC3339UTC(value string) (time.Time, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return time.Time{}, false
	}
	return parsed.UTC(), true
}

func humanDuration(value time.Duration) string {
	if value < time.Minute {
		seconds := int(value.Round(time.Second) / time.Second)
		if seconds < 1 {
			seconds = 1
		}
		return fmt.Sprintf("%ds", seconds)
	}
	if value < time.Hour {
		return fmt.Sprintf("%dm", int(value.Round(time.Minute)/time.Minute))
	}
	hours := int(value / time.Hour)
	minutes := int(value.Round(time.Minute)/time.Minute) % 60
	return fmt.Sprintf("%dh%02dm", hours, minutes)
}

func maxDuration(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}

func isTerminalDeploymentStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "failed", "superseded":
		return true
	default:
		return false
	}
}

func expectedWireGuardPeerCounts(peers []WireGuardGatewayPeer) (active int, revoked int) {
	for _, peer := range peers {
		if strings.EqualFold(strings.TrimSpace(peer.Status), "revoked") {
			revoked++
			continue
		}
		active++
	}
	return active, revoked
}

func expectedAccessPolicyCounts(policies []ControllerServiceAccessPolicy) (open int, locked int, allowed int) {
	for _, policy := range policies {
		if policy.SSHUnlocked {
			open++
		} else {
			locked++
		}
		allowed += len(policy.AllowedPeerAddresses)
	}
	return open, locked, allowed
}
