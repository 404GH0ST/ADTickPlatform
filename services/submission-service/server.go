package main

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"adplatform/internal/platform/httpapi"
	"adplatform/internal/services/apigateway"
)

type successEnvelope[T any] struct {
	Status string `json:"status"`
	Data   T      `json:"data"`
}

type submissionServiceServer struct {
	adminToken string
	gameCore   submissionGameCoreClient
}

type submissionGameCoreClient interface {
	SubmitFlags(ctx context.Context, teamID int, flags []string) ([]apigateway.SubmissionVerdictAlias, error)
	AttackFeed(ctx context.Context, query apigateway.AttackFeedQuery) (apigateway.AttackFeedPage, error)
}

func newSubmissionServiceServer(adminToken string, gameCore submissionGameCoreClient) *submissionServiceServer {
	if gameCore == nil {
		gameCore = noopGameCoreSubmissionClient{}
	}
	return &submissionServiceServer{
		adminToken: strings.TrimSpace(adminToken),
		gameCore:   gameCore,
	}
}

func (s *submissionServiceServer) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /internal/v1/submissions/submit", s.handleSubmit)
	mux.HandleFunc("GET /internal/v1/submissions/attacks", s.handleAttackFeed)
}

func (s *submissionServiceServer) handleSubmit(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}

	var request apigateway.GameSubmitFlagsRequest
	if err := httpapi.DecodeJSON(r, &request); err != nil || request.TeamID <= 0 || len(request.Flags) == 0 {
		httpapi.WriteJSON(w, http.StatusBadRequest, httpapi.ErrorEnvelope{Status: "failed", Message: "submission request is invalid."})
		return
	}

	results, err := s.gameCore.SubmitFlags(r.Context(), request.TeamID, request.Flags)
	if err != nil {
		writeSubmissionServiceFailure(w, err, "game-core flag submission failed.")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, successEnvelope[[]apigateway.SubmissionVerdictAlias]{Status: "success", Data: results})
}

func (s *submissionServiceServer) handleAttackFeed(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}

	page, err := s.gameCore.AttackFeed(r.Context(), apigateway.AttackFeedQuery{
		Limit:    parsePositiveQueryInt(r, "limit", 12, 200),
		Offset:   parseNonNegativeQueryInt(r, "offset"),
		Attacker: strings.TrimSpace(r.URL.Query().Get("attacker")),
		Victim:   strings.TrimSpace(r.URL.Query().Get("victim")),
		Service:  strings.TrimSpace(r.URL.Query().Get("service")),
		TickFrom: parseNonNegativeQueryInt(r, "tick_from"),
		TickTo:   parseNonNegativeQueryInt(r, "tick_to"),
	})
	if err != nil {
		writeSubmissionServiceFailure(w, err, "game-core attack feed failed.")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, successEnvelope[apigateway.AttackFeedPage]{Status: "success", Data: page})
}

func (s *submissionServiceServer) requireAdminAuth(w http.ResponseWriter, r *http.Request) bool {
	token, ok := httpapi.BearerToken(r)
	if !ok || token != s.adminToken {
		httpapi.WriteJSON(w, http.StatusForbidden, httpapi.ErrorEnvelope{Status: "forbidden", Message: "please authenticate before accessing submission-service endpoints."})
		return false
	}
	return true
}

func writeSubmissionServiceFailure(w http.ResponseWriter, err error, fallback string) {
	message := strings.TrimSpace(fallback)
	if message == "" {
		message = "submission-service request failed."
	}
	if errors.Is(err, errGameCoreSubmissionDisabled) {
		message = "submission-service is not connected to game-core."
	}
	httpapi.WriteJSON(w, http.StatusBadGateway, httpapi.ErrorEnvelope{Status: "failed", Message: message})
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
