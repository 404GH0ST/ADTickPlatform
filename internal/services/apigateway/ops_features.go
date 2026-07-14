package apigateway

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"adplatform/internal/platform/httpapi"
)

type participantChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type matchAnnouncement struct {
	ID        int    `json:"id"`
	Body      string `json:"body"`
	CreatedBy string `json:"created_by,omitempty"`
	CreatedAt string `json:"created_at"`
}

type createAnnouncementRequest struct {
	Body string `json:"body"`
}

type adminBulkImportPlayer struct {
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Password    string `json:"password"`
	Role        string `json:"role"`
}

type adminBulkImportTeam struct {
	Name         string                  `json:"name"`
	ContactEmail string                  `json:"contact_email"`
	Players      []adminBulkImportPlayer `json:"players"`
}

type adminBulkImportRequest struct {
	Teams []adminBulkImportTeam `json:"teams"`
}

type adminBulkImportResult struct {
	TeamsCreated   int           `json:"teams_created"`
	PlayersCreated int           `json:"players_created"`
	Errors         []string      `json:"errors,omitempty"`
	Teams          []adminTeam   `json:"teams,omitempty"`
	Players        []adminPlayer `json:"players,omitempty"`
}

func (s *Server) registerOpsFeatureRoutes(mux *http.ServeMux) {
	mux.HandleFunc("PUT /api/v2/me/password", s.handleChangeParticipantPassword)
	mux.HandleFunc("GET /api/v2/announcements", s.handleListAnnouncements)
	mux.HandleFunc("GET /api/v2/admin/announcements", s.handleAdminListAnnouncements)
	mux.HandleFunc("POST /api/v2/admin/announcements", s.handleAdminCreateAnnouncement)
	mux.HandleFunc("DELETE /api/v2/admin/announcements/{announcement_id}", s.handleAdminDeleteAnnouncement)
	mux.HandleFunc("POST /api/v2/admin/import/teams", s.handleAdminBulkImportTeams)
}

func (s *Server) handleChangeParticipantPassword(w http.ResponseWriter, r *http.Request) {
	player, ok := s.requirePlayerAuth(w, r, "please authenticate before password change.")
	if !ok {
		return
	}
	if strings.EqualFold(strings.TrimSpace(player.Role), "organizer") {
		writeProblem(w, http.StatusForbidden, "Authentication required", "please authenticate as a participant.")
		return
	}

	var req participantChangePasswordRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		writeProblem(w, http.StatusBadRequest, "Invalid request", "password change request is invalid.")
		return
	}
	current := req.CurrentPassword
	next := req.NewPassword
	if current == "" || next == "" {
		writeProblem(w, http.StatusBadRequest, "Invalid request", "current_password and new_password are required.")
		return
	}
	if !validPassword(next) {
		writeProblem(w, http.StatusBadRequest, "Invalid request", "new_password must be at least 8 characters.")
		return
	}
	decision, allowed := s.allowRateLimit(r.Context(), rateLimitClientKey("password-change", clientRateLimitKey(r)), passwordChangeRateLimitPolicy)
	if !allowed {
		writeRateLimitFailure(w, decision, defaultRateLimit429Message)
		return
	}
	decision, allowed = s.allowRateLimit(r.Context(), rateLimitUserKey("password-change", player.PlayerID), passwordChangeRateLimitPolicy)
	if !allowed {
		writeRateLimitFailure(w, decision, defaultRateLimit429Message)
		return
	}

	sessionVersion, err := s.store.ChangeParticipantPassword(r.Context(), player.PlayerID, current, next)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidCredentials):
			writeProblem(w, http.StatusForbidden, "Password change failed", "current password is wrong.")
		default:
			writeStoreFailure(w, err)
		}
		return
	}
	player.SessionVersion = sessionVersion
	token, err := issueTeamJWT(s.teamTokenSecret, player, s.now())
	if err != nil {
		writeStoreFailure(w, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"updated": true, "token": token, "token_type": "Bearer"})
}

func (s *Server) handleListAnnouncements(w http.ResponseWriter, r *http.Request) {
	decision, allowed := s.allowRateLimit(r.Context(), rateLimitClientKey("announcements", clientRateLimitKey(r)), scoreboardRateLimitPolicy)
	if !allowed {
		writeRateLimitFailure(w, decision, defaultRateLimit429Message)
		return
	}
	items, err := s.store.ListAnnouncements(r.Context())
	if err != nil {
		writeStoreFailure(w, err)
		return
	}
	writeData(w, http.StatusOK, items)
}

func (s *Server) handleAdminListAnnouncements(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	items, err := s.store.ListAnnouncements(r.Context())
	if err != nil {
		writeStoreFailure(w, err)
		return
	}
	writeData(w, http.StatusOK, items)
}

func (s *Server) handleAdminCreateAnnouncement(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	var req createAnnouncementRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		writeProblem(w, http.StatusBadRequest, "Invalid request", "announcement request is invalid.")
		return
	}
	body := strings.TrimSpace(req.Body)
	if body == "" || len(body) > 4000 {
		writeProblem(w, http.StatusBadRequest, "Invalid request", "body must be 1-4000 characters.")
		return
	}
	item, err := s.store.CreateAnnouncement(r.Context(), body, "organizer", s.now())
	if err != nil {
		writeStoreFailure(w, err)
		return
	}
	s.recordAdminAudit(r.Context(), "announcement.create", "announcement", fmt.Sprintf("announcement:%d", item.ID), "created match announcement", map[string]any{
		"announcement_id": item.ID,
	})
	writeData(w, http.StatusOK, item)
}

func (s *Server) handleAdminDeleteAnnouncement(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	id, err := strconv.Atoi(r.PathValue("announcement_id"))
	if err != nil || id <= 0 {
		writeProblem(w, http.StatusBadRequest, "Invalid request", "announcement_id is invalid.")
		return
	}
	if err := s.store.DeleteAnnouncement(r.Context(), id); err != nil {
		if errors.Is(err, ErrResourceNotFound) {
			writeProblem(w, http.StatusNotFound, "Not found", "announcement was not found.")
			return
		}
		writeStoreFailure(w, err)
		return
	}
	s.recordAdminAudit(r.Context(), "announcement.delete", "announcement", fmt.Sprintf("announcement:%d", id), "deleted match announcement", map[string]any{
		"announcement_id": id,
	})
	writeData(w, http.StatusOK, map[string]any{"deleted": true})
}

func (s *Server) handleAdminBulkImportTeams(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	var req adminBulkImportRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil || len(req.Teams) == 0 {
		writeProblem(w, http.StatusBadRequest, "Invalid request", "teams array is required.")
		return
	}
	if len(req.Teams) > 200 {
		writeProblem(w, http.StatusBadRequest, "Invalid request", "import at most 200 teams per request.")
		return
	}

	result, err := s.store.ImportAdminTeams(r.Context(), req.Teams, s.now())
	if err != nil {
		writeStoreFailure(w, err)
		return
	}

	s.recordAdminAudit(r.Context(), "import.teams", "registry", "bulk-import", "bulk imported teams and players", map[string]any{
		"teams_created":   result.TeamsCreated,
		"players_created": result.PlayersCreated,
		"error_count":     len(result.Errors),
	})

	// Contract: valid body always returns structured result.
	// 422 when nothing was created (all rows failed validation/persist).
	// 200 when at least one team landed (partial success includes errors[]).
	if result.TeamsCreated == 0 {
		writeData(w, http.StatusUnprocessableEntity, result)
		return
	}
	writeData(w, http.StatusOK, result)
}
