package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"adplatform/internal/services/apigateway"
)

func (s *gameCoreServer) handleSubmissions(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	query := apigateway.SubmissionQuery{AttackFeedQuery: apigateway.AttackFeedQuery{Limit: parsePositiveQueryInt(r, "limit", 25, 200), Offset: parseNonNegativeQueryInt(r, "offset"), Attacker: strings.TrimSpace(r.URL.Query().Get("attacker")), Victim: strings.TrimSpace(r.URL.Query().Get("victim")), Service: strings.TrimSpace(r.URL.Query().Get("service")), TickFrom: parseNonNegativeQueryInt(r, "tick_from"), TickTo: parseNonNegativeQueryInt(r, "tick_to")}, Status: strings.TrimSpace(r.URL.Query().Get("status")), Flag: strings.TrimSpace(r.URL.Query().Get("flag"))}
	page, err := s.store.ListSubmissions(r.Context(), query)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "Submissions unavailable", "could not load submission history.")
		return
	}
	writeData(w, http.StatusOK, page)
}
func (s *gameCoreServer) handleDeleteSubmission(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("submission_id"), 10, 64)
	if err != nil || id <= 0 {
		writeProblem(w, http.StatusBadRequest, "Invalid submission", "submission id must be positive.")
		return
	}
	result, err := s.store.DeleteSubmission(r.Context(), id)
	if errors.Is(err, errSubmissionNotFound) {
		writeProblem(w, http.StatusNotFound, "Submission not found", "this submission has already been removed or does not exist.")
		return
	}
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "Delete failed", "could not delete the submission.")
		return
	}
	if result.Status == "accepted" {
		s.scheduleScoreboardRecompute()
	}
	if result.SSHGrantRevoked && s.accessReconcile != nil {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 15*time.Second)
		defer cancel()
		if err := s.accessReconcile.ReconcileAccess(ctx); err != nil {
			log.Printf("access reconcile after removing submission %d: %v", id, err)
			s.accessPending.Store(true)
			result.AccessReconcilePending = true
		}
	}
	writeData(w, http.StatusOK, result)
}

func (s *gameCoreServer) recordRejectedSubmissions(ctx context.Context, teamID int, attacker string, tick int, verdicts []submissionVerdictAlias) error {
	var rejected []apigateway.AdminSubmission
	for _, verdict := range verdicts {
		if verdict.Status == "accepted" {
			continue
		}
		row := apigateway.AdminSubmission{TeamID: teamID, Attacker: attacker, Flag: verdict.Flag, Tick: tick, Status: verdict.Status, Detail: verdict.Detail, SubmittedAt: s.now().UTC()}
		if issued, err := s.store.LookupIssuedFlag(ctx, strings.TrimSpace(verdict.Flag)); err == nil {
			row.Victim, row.ChallengeID, row.Service = issued.OwnerTeamName, issued.ChallengeID, issued.ChallengeName
		}
		rejected = append(rejected, row)
	}
	if len(rejected) == 0 {
		return nil
	}
	return s.store.RecordSubmissions(ctx, rejected...)
}
