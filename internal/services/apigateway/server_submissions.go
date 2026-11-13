package apigateway

import (
	"net/http"
	"strconv"
	"strings"
)

func (s *Server) handleAdminSubmissions(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	query := SubmissionQuery{AttackFeedQuery: parseAttackFeedQuery(r), Status: strings.TrimSpace(r.URL.Query().Get("status")), Flag: strings.TrimSpace(r.URL.Query().Get("flag"))}
	if r.URL.Query().Get("limit") == "" {
		query.Limit = 25
	}
	page, err := s.gameCore.Submissions(r.Context(), query)
	if err != nil {
		writeGameCoreFailure(w, err, "could not load submissions.")
		return
	}
	writeData(w, http.StatusOK, page)
}
func (s *Server) handleAdminDeleteSubmission(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("submission_id"), 10, 64)
	if err != nil || id <= 0 {
		writeProblem(w, http.StatusBadRequest, "Invalid submission", "submission id must be positive.")
		return
	}
	result, err := s.gameCore.DeleteSubmission(r.Context(), id)
	if err != nil {
		writeGameCoreFailure(w, err, "could not delete submission.")
		return
	}
	s.recordAdminAudit(r.Context(), "submission.delete", "submission", strconv.FormatInt(id, 10), "deleted submission", map[string]any{"status": result.Status, "service": result.Service, "first_blood_cleared": result.FirstBloodCleared})
	writeData(w, http.StatusOK, result)
}
