package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"adplatform/internal/platform/config"
	"adplatform/internal/platform/database"
	"adplatform/internal/platform/httpapi"
	"adplatform/internal/services/apigateway"
)

type successEnvelope[T any] struct {
	Status string `json:"status"`
	Data   T      `json:"data"`
}

type controllerServer struct {
	adminToken string
	store      apigateway.Store
	executor   runtimeExecutor
	access     serviceAccessExecutor
	metrics    controllerServiceMetrics
	now        func() time.Time
}

func main() {
	ctx := context.Background()
	info := httpapi.ServiceInfo{Name: "controller-service", Version: "dev", Addr: config.String("CONTROLLER_SERVICE_ADDR", ":8084")}

	store, err := buildStore(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	server := &controllerServer{
		adminToken: config.String("ADMIN_API_TOKEN", "dev-admin-token"),
		store:      store,
		executor:   newRuntimeExecutor(),
		access:     newServiceAccessExecutor(),
		metrics:    newControllerServiceMetrics(),
		now:        time.Now,
	}
	httpapi.RegisterMetricsSource(info.Name, server)
	if err := restoreControllerState(ctx, controllerStartupStoreAdapter{store: store}, server.executor, server.access, server.now); err != nil {
		log.Fatal(err)
	}

	mux := httpapi.NewBaseMux(info)
	server.RegisterRoutes(mux)

	log.Printf("starting %s on %s", info.Name, info.Addr)
	if err := httpapi.RunServer(ctx, info, mux); err != nil {
		log.Fatal(err)
	}
}

func buildStore(ctx context.Context) (apigateway.Store, error) {
	backend := config.String("CONTROLLER_STATE_BACKEND", config.String("API_GATEWAY_STATE_BACKEND", "memory"))
	if backend != "postgres" {
		return apigateway.NewMemoryStore(config.Int("API_GATEWAY_TEAM_ID", 101)), nil
	}

	db, err := database.OpenPostgres(ctx, config.String("POSTGRES_DSN", "postgres://adplatform:adplatform@localhost:5432/adplatform?sslmode=disable"))
	if err != nil {
		return nil, err
	}
	if config.Bool("CONTROLLER_AUTO_MIGRATE", config.Bool("API_GATEWAY_AUTO_MIGRATE", true)) {
		if err := database.RunEmbeddedMigrationsWithOptions(ctx, db, database.MigrationOptions{
			IncludeSeeds: config.Bool("DATABASE_INCLUDE_SEEDS", config.Bool("API_GATEWAY_AUTO_SEED", false)),
		}); err != nil {
			db.Close()
			return nil, err
		}
	}
	return apigateway.NewPostgresStore(db), nil
}

func (s *controllerServer) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /internal/v1/deployments", s.handleListDeployments)
	mux.HandleFunc("POST /internal/v1/deployments/reconcile", s.handleReconcileDeployments)
	mux.HandleFunc("POST /internal/v1/challenges/validate", s.handleValidateChallengeRuntime)
	mux.HandleFunc("GET /internal/v1/access/status", s.handleAccessStatus)
	mux.HandleFunc("POST /internal/v1/access/reconcile", s.handleReconcileAccessPolicies)
	mux.HandleFunc("POST /internal/v1/access/teardown", s.handleTeardownAccessPolicies)
	mux.HandleFunc("POST /internal/v1/teams/{team_id}/services/{challenge_id}/access/reconcile", s.handleReconcileServiceAccess)
	mux.HandleFunc("POST /internal/v1/teams/{team_id}/services/{challenge_id}/ssh-credential", s.handleApplySSHCredential)
	mux.HandleFunc("POST /internal/v1/teams/{team_id}/services/{challenge_id}/reset/factory", s.handleFactoryResetService)
	mux.HandleFunc("POST /internal/v1/teams/{team_id}/services/{challenge_id}/restart", s.handleRestartService)
	mux.HandleFunc("POST /internal/v1/teams/{team_id}/services/{challenge_id}/remove", s.handleRemoveService)
	mux.HandleFunc("POST /internal/v1/teams/{team_id}/remove", s.handleRemoveTeamServices)
	mux.HandleFunc("POST /internal/v1/challenges/{challenge_id}/remove", s.handleRemoveChallengeServices)
}

func (s *controllerServer) handleListDeployments(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	deployments, err := s.store.ListAdminDeployments(r.Context())
	if err != nil {
		httpapi.WriteJSON(w, http.StatusInternalServerError, httpapi.ErrorEnvelope{Status: "failed", Message: err.Error()})
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, successEnvelope[any]{Status: "success", Data: deployments})
}

func (s *controllerServer) handleReconcileDeployments(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	started := time.Now()
	tasks, err := s.store.ListControllerRuntimeTasks(r.Context())
	if err != nil {
		s.metrics.recordDeploymentReconcile(time.Since(started), 0, true)
		httpapi.WriteJSON(w, http.StatusInternalServerError, httpapi.ErrorEnvelope{Status: "failed", Message: err.Error()})
		return
	}
	ensured := 0
	for _, task := range tasks {
		if err := s.executor.EnsureService(r.Context(), task); err != nil {
			s.metrics.recordDeploymentReconcile(time.Since(started), ensured, true)
			httpapi.WriteJSON(w, http.StatusInternalServerError, httpapi.ErrorEnvelope{Status: "failed", Message: err.Error()})
			return
		}
		ensured++
	}
	result, err := s.store.ReconcileAdminDeployments(r.Context(), s.now())
	if err != nil {
		s.metrics.recordDeploymentReconcile(time.Since(started), ensured, true)
		httpapi.WriteJSON(w, http.StatusInternalServerError, httpapi.ErrorEnvelope{Status: "failed", Message: err.Error()})
		return
	}
	s.metrics.recordDeploymentReconcile(time.Since(started), ensured, false)
	httpapi.WriteJSON(w, http.StatusOK, successEnvelope[any]{Status: "success", Data: result})
}

func (s *controllerServer) handleValidateChallengeRuntime(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	started := time.Now()
	var request apigateway.ChallengeValidationRequest
	if err := httpapi.DecodeJSON(r, &request); err != nil || request.ChallengeID <= 0 || request.BaselineImage == "" {
		s.metrics.recordOperation(controllerOperationChallengeValidate, time.Since(started), true)
		httpapi.WriteJSON(w, http.StatusBadRequest, httpapi.ErrorEnvelope{Status: "failed", Message: "challenge validation request is invalid."})
		return
	}
	result, err := s.executor.ValidateChallengeRuntime(r.Context(), request)
	if err != nil {
		s.metrics.recordOperation(controllerOperationChallengeValidate, time.Since(started), true)
		httpapi.WriteJSON(w, http.StatusInternalServerError, httpapi.ErrorEnvelope{Status: "failed", Message: err.Error()})
		return
	}
	s.metrics.recordOperation(controllerOperationChallengeValidate, time.Since(started), false)
	httpapi.WriteJSON(w, http.StatusOK, successEnvelope[apigateway.ChallengeValidationResult]{Status: "success", Data: result})
}

func (s *controllerServer) handleAccessStatus(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, successEnvelope[any]{Status: "success", Data: s.access.Status()})
}

func (s *controllerServer) handleReconcileAccessPolicies(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	started := time.Now()
	policies, err := s.store.ListControllerServiceAccessPolicies(r.Context())
	if err != nil {
		s.metrics.recordAccessReconcile(controllerAccessScopeGlobal, time.Since(started), 0, true)
		httpapi.WriteJSON(w, http.StatusInternalServerError, httpapi.ErrorEnvelope{Status: "failed", Message: err.Error()})
		return
	}
	status, err := s.access.Apply(r.Context(), policies, s.now())
	if err != nil {
		s.metrics.recordAccessReconcile(controllerAccessScopeGlobal, time.Since(started), 0, true)
		httpapi.WriteJSON(w, http.StatusInternalServerError, httpapi.ErrorEnvelope{Status: "failed", Message: status.LastError})
		return
	}
	s.metrics.recordAccessReconcile(controllerAccessScopeGlobal, time.Since(started), status.PoliciesTotal, false)
	httpapi.WriteJSON(w, http.StatusOK, successEnvelope[any]{Status: "success", Data: status})
}

func (s *controllerServer) handleTeardownAccessPolicies(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	started := time.Now()
	if err := s.access.Teardown(r.Context()); err != nil {
		s.metrics.recordOperation(controllerOperationAccessTeardown, time.Since(started), true)
		httpapi.WriteJSON(w, http.StatusInternalServerError, httpapi.ErrorEnvelope{Status: "failed", Message: err.Error()})
		return
	}
	s.metrics.recordOperation(controllerOperationAccessTeardown, time.Since(started), false)
	httpapi.WriteJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

func (s *controllerServer) handleReconcileServiceAccess(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	started := time.Now()
	teamID, err := strconv.Atoi(r.PathValue("team_id"))
	if err != nil || teamID <= 0 {
		s.metrics.recordAccessReconcile(controllerAccessScopeService, time.Since(started), 0, true)
		httpapi.WriteJSON(w, http.StatusBadRequest, httpapi.ErrorEnvelope{Status: "failed", Message: "team id is invalid."})
		return
	}
	challengeID, err := strconv.Atoi(r.PathValue("challenge_id"))
	if err != nil || challengeID <= 0 {
		s.metrics.recordAccessReconcile(controllerAccessScopeService, time.Since(started), 0, true)
		httpapi.WriteJSON(w, http.StatusBadRequest, httpapi.ErrorEnvelope{Status: "failed", Message: "challenge id is invalid."})
		return
	}
	if _, err := s.store.GetControllerServiceAccessPolicy(r.Context(), teamID, challengeID); err != nil {
		s.metrics.recordAccessReconcile(controllerAccessScopeService, time.Since(started), 0, true)
		status := http.StatusInternalServerError
		message := err.Error()
		if errors.Is(err, apigateway.ErrChallengeNotFound) {
			status = http.StatusNotFound
			message = "service access policy was not found."
		}
		httpapi.WriteJSON(w, status, httpapi.ErrorEnvelope{Status: "failed", Message: message})
		return
	}
	policies, err := s.store.ListControllerServiceAccessPolicies(r.Context())
	if err != nil {
		s.metrics.recordAccessReconcile(controllerAccessScopeService, time.Since(started), 0, true)
		httpapi.WriteJSON(w, http.StatusInternalServerError, httpapi.ErrorEnvelope{Status: "failed", Message: err.Error()})
		return
	}
	status, err := s.access.Apply(r.Context(), policies, s.now())
	if err != nil {
		s.metrics.recordAccessReconcile(controllerAccessScopeService, time.Since(started), 0, true)
		httpapi.WriteJSON(w, http.StatusInternalServerError, httpapi.ErrorEnvelope{Status: "failed", Message: status.LastError})
		return
	}
	s.metrics.recordAccessReconcile(controllerAccessScopeService, time.Since(started), status.PoliciesTotal, false)
	httpapi.WriteJSON(w, http.StatusOK, successEnvelope[any]{Status: "success", Data: status})
}

func (s *controllerServer) handleApplySSHCredential(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	started := time.Now()
	task, ok := s.parseRuntimeTask(w, r)
	if !ok {
		s.metrics.recordOperation(controllerOperationSSHCredential, time.Since(started), true)
		return
	}
	var credential apigateway.ControllerSSHCredential
	if err := httpapi.DecodeJSON(r, &credential); err != nil || credential.Password == "" || credential.ExpiresAt == "" {
		s.metrics.recordOperation(controllerOperationSSHCredential, time.Since(started), true)
		httpapi.WriteJSON(w, http.StatusBadRequest, httpapi.ErrorEnvelope{Status: "failed", Message: "ssh credential request is invalid."})
		return
	}
	if err := s.executor.ApplySSHCredential(r.Context(), task, credential); err != nil {
		s.metrics.recordOperation(controllerOperationSSHCredential, time.Since(started), true)
		httpapi.WriteJSON(w, http.StatusInternalServerError, httpapi.ErrorEnvelope{Status: "failed", Message: err.Error()})
		return
	}
	s.metrics.recordOperation(controllerOperationSSHCredential, time.Since(started), false)
	httpapi.WriteJSON(w, http.StatusOK, successEnvelope[any]{Status: "success", Data: map[string]any{
		"team_id": task.TeamID, "challenge_id": task.ChallengeID, "action": "apply_ssh_credential_runtime",
	}})
}

func (s *controllerServer) handleFactoryResetService(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	started := time.Now()
	task, ok := s.parseRuntimeTask(w, r)
	if !ok {
		s.metrics.recordOperation(controllerOperationFactoryReset, time.Since(started), true)
		return
	}
	if err := s.executor.FactoryResetService(r.Context(), task); err != nil {
		s.metrics.recordOperation(controllerOperationFactoryReset, time.Since(started), true)
		httpapi.WriteJSON(w, http.StatusInternalServerError, httpapi.ErrorEnvelope{Status: "failed", Message: err.Error()})
		return
	}
	s.metrics.recordOperation(controllerOperationFactoryReset, time.Since(started), false)
	httpapi.WriteJSON(w, http.StatusOK, successEnvelope[any]{Status: "success", Data: map[string]any{
		"team_id": task.TeamID, "challenge_id": task.ChallengeID, "action": "factory_reset_runtime",
	}})
}

func (s *controllerServer) handleRestartService(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	started := time.Now()
	task, ok := s.parseRuntimeTask(w, r)
	if !ok {
		s.metrics.recordOperation(controllerOperationRestart, time.Since(started), true)
		return
	}
	if err := s.executor.RestartService(r.Context(), task); err != nil {
		s.metrics.recordOperation(controllerOperationRestart, time.Since(started), true)
		httpapi.WriteJSON(w, http.StatusInternalServerError, httpapi.ErrorEnvelope{Status: "failed", Message: err.Error()})
		return
	}
	s.metrics.recordOperation(controllerOperationRestart, time.Since(started), false)
	httpapi.WriteJSON(w, http.StatusOK, successEnvelope[any]{Status: "success", Data: map[string]any{
		"team_id": task.TeamID, "challenge_id": task.ChallengeID, "action": "restart_runtime",
	}})
}

func (s *controllerServer) handleRemoveService(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	started := time.Now()
	teamID, err := strconv.Atoi(r.PathValue("team_id"))
	if err != nil || teamID <= 0 {
		s.metrics.recordOperation(controllerOperationRemoveService, time.Since(started), true)
		httpapi.WriteJSON(w, http.StatusBadRequest, httpapi.ErrorEnvelope{Status: "failed", Message: "team id is invalid."})
		return
	}
	challengeID, err := strconv.Atoi(r.PathValue("challenge_id"))
	if err != nil || challengeID <= 0 {
		s.metrics.recordOperation(controllerOperationRemoveService, time.Since(started), true)
		httpapi.WriteJSON(w, http.StatusBadRequest, httpapi.ErrorEnvelope{Status: "failed", Message: "challenge id is invalid."})
		return
	}
	if err := s.executor.RemoveService(r.Context(), teamID, challengeID); err != nil {
		s.metrics.recordOperation(controllerOperationRemoveService, time.Since(started), true)
		httpapi.WriteJSON(w, http.StatusInternalServerError, httpapi.ErrorEnvelope{Status: "failed", Message: err.Error()})
		return
	}
	s.metrics.recordOperation(controllerOperationRemoveService, time.Since(started), false)
	httpapi.WriteJSON(w, http.StatusOK, successEnvelope[any]{Status: "success", Data: map[string]any{
		"team_id": teamID, "challenge_id": challengeID, "action": "remove_runtime",
	}})
}

func (s *controllerServer) handleRemoveTeamServices(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	started := time.Now()
	teamID, err := strconv.Atoi(r.PathValue("team_id"))
	if err != nil || teamID <= 0 {
		s.metrics.recordOperation(controllerOperationRemoveTeam, time.Since(started), true)
		httpapi.WriteJSON(w, http.StatusBadRequest, httpapi.ErrorEnvelope{Status: "failed", Message: "team id is invalid."})
		return
	}
	if err := s.executor.RemoveTeamServices(r.Context(), teamID); err != nil {
		s.metrics.recordOperation(controllerOperationRemoveTeam, time.Since(started), true)
		httpapi.WriteJSON(w, http.StatusInternalServerError, httpapi.ErrorEnvelope{Status: "failed", Message: err.Error()})
		return
	}
	s.metrics.recordOperation(controllerOperationRemoveTeam, time.Since(started), false)
	httpapi.WriteJSON(w, http.StatusOK, successEnvelope[any]{Status: "success", Data: map[string]any{
		"team_id": teamID, "action": "remove_team_runtimes",
	}})
}

func (s *controllerServer) handleRemoveChallengeServices(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	started := time.Now()
	challengeID, err := strconv.Atoi(r.PathValue("challenge_id"))
	if err != nil || challengeID <= 0 {
		s.metrics.recordOperation(controllerOperationRemoveChallenge, time.Since(started), true)
		httpapi.WriteJSON(w, http.StatusBadRequest, httpapi.ErrorEnvelope{Status: "failed", Message: "challenge id is invalid."})
		return
	}
	if err := s.executor.RemoveChallengeServices(r.Context(), challengeID); err != nil {
		s.metrics.recordOperation(controllerOperationRemoveChallenge, time.Since(started), true)
		httpapi.WriteJSON(w, http.StatusInternalServerError, httpapi.ErrorEnvelope{Status: "failed", Message: err.Error()})
		return
	}
	s.metrics.recordOperation(controllerOperationRemoveChallenge, time.Since(started), false)
	httpapi.WriteJSON(w, http.StatusOK, successEnvelope[any]{Status: "success", Data: map[string]any{
		"challenge_id": challengeID, "action": "remove_challenge_runtimes",
	}})
}

func (s *controllerServer) parseRuntimeTask(w http.ResponseWriter, r *http.Request) (apigateway.ControllerRuntimeTask, bool) {
	teamID, err := strconv.Atoi(r.PathValue("team_id"))
	if err != nil || teamID <= 0 {
		httpapi.WriteJSON(w, http.StatusBadRequest, httpapi.ErrorEnvelope{Status: "failed", Message: "team id is invalid."})
		return apigateway.ControllerRuntimeTask{}, false
	}
	challengeID, err := strconv.Atoi(r.PathValue("challenge_id"))
	if err != nil || challengeID <= 0 {
		httpapi.WriteJSON(w, http.StatusBadRequest, httpapi.ErrorEnvelope{Status: "failed", Message: "challenge id is invalid."})
		return apigateway.ControllerRuntimeTask{}, false
	}
	task, err := s.store.GetControllerRuntimeTask(r.Context(), teamID, challengeID)
	if err != nil {
		status := http.StatusInternalServerError
		message := err.Error()
		if errors.Is(err, apigateway.ErrChallengeNotFound) {
			status = http.StatusNotFound
			message = "service runtime was not found."
		}
		httpapi.WriteJSON(w, status, httpapi.ErrorEnvelope{Status: "failed", Message: message})
		return apigateway.ControllerRuntimeTask{}, false
	}
	return task, true
}

func (s *controllerServer) requireAdminAuth(w http.ResponseWriter, r *http.Request) bool {
	token, ok := httpapi.BearerToken(r)
	if !ok || token != s.adminToken {
		httpapi.WriteJSON(w, http.StatusForbidden, httpapi.ErrorEnvelope{Status: "forbidden", Message: "please authenticate before accessing controller endpoints."})
		return false
	}
	return true
}
