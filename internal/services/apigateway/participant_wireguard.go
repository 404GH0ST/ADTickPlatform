package apigateway

import (
	"fmt"
	"net/http"
	"strings"
)

func (s *Server) handleParticipantWireGuardConfig(w http.ResponseWriter, r *http.Request) {
	player, ok := s.requirePlayerAuth(w, r, "please authenticate before VPN config download.")
	if !ok {
		return
	}

	decision, allowed := s.allowRateLimit(r.Context(), rateLimitTeamKey("wireguard-download", player.TeamID), challengeSourceRateLimitPolicy)
	if !allowed {
		writeRateLimitFailure(w, decision, defaultRateLimit429Message)
		return
	}

	peer, err := s.store.GetAdminPlayerWireGuardConfig(r.Context(), player.PlayerID)
	if err != nil {
		writeDomainFailure(w, err)
		return
	}

	config := strings.TrimSpace(peer.Config)
	if config == "" {
		writeProblem(w, http.StatusNotFound, "VPN config unavailable", "wireguard config is not available yet.")
		return
	}

	s.recordTeamAudit(r.Context(), player.TeamID, "wireguard.download", "player",
		fmt.Sprintf("player:%d", player.PlayerID),
		"downloaded own wireguard config",
		map[string]any{
			"player_id": player.PlayerID,
			"team_id":   player.TeamID,
			"peer":      peer.WireGuardPeer,
			"status":    peer.Status,
		})

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", peer.DownloadName))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(peer.Config)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(peer.Config))
}
