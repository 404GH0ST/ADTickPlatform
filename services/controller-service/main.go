package main

import (
	"context"
	"crypto/subtle"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"adplatform/internal/platform/config"
	"adplatform/internal/platform/database"
	"adplatform/internal/platform/httpapi"
	"adplatform/internal/services/apigateway"
)

type controllerServer struct {
	adminToken string
	store      apigateway.Store
	executor   runtimeExecutor
	access     serviceAccessExecutor
	accessMu   sync.Mutex
	wireGuard  controllerWireGuardReconciler
	metrics    *controllerServiceMetrics
	now        func() time.Time
}

// fatal reports a startup failure and exits; tests replace it.
var fatal = log.Fatal

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil {
		fatal(err)
	}
}

// run wires the service and serves until ctx is cancelled.
func run(ctx context.Context) error {
	info := httpapi.ServiceInfo{Name: "controller-service", Version: "dev", Addr: config.String("CONTROLLER_SERVICE_ADDR", ":8084")}

	store, err := buildStore(ctx)
	if err != nil {
		return err
	}
	defer store.Close()

	server := &controllerServer{
		adminToken: config.Secret("CONTROLLER_INTERNAL_TOKEN", "ADMIN_API_TOKEN"),
		store:      store,
		executor:   newRuntimeExecutor(),
		access:     newServiceAccessExecutor(),
		wireGuard:  newControllerWireGuardReconciler(),
		metrics:    newControllerServiceMetrics(),
		now:        time.Now,
	}
	if docker, ok := server.executor.(*dockerCLIExecutor); ok {
		if err := docker.validateCaptureConfig(); err != nil {
			return err
		}
	}
	httpapi.RegisterMetricsSource(info.Name, server)
	if err := restoreControllerState(ctx, controllerReconcileStoreAdapter{store: store}, server.executor, server.access, server.wireGuard, server.now); err != nil {
		return err
	}

	if docker, ok := server.executor.(*dockerCLIExecutor); ok {
		stopCapture := docker.startCaptureMonitor()
		defer stopCapture()
	}

	mux := httpapi.NewBaseMux(info)
	server.RegisterRoutes(mux)

	log.Printf("starting %s on %s", info.Name, info.Addr)
	return httpapi.RunServer(ctx, info, mux)
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
		writeProblem(w, http.StatusInternalServerError, "Internal state unavailable", err.Error())
		return
	}
	writeData(w, http.StatusOK, deployments)
}

func (s *controllerServer) handleReconcileDeployments(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	started := time.Now()
	reconciler := newControllerTrustedReconciler(
		controllerReconcileStoreAdapter{store: s.store},
		s.executor,
		s.access,
		s.wireGuard,
		s.now,
	)
	reconciler.accessMu = &s.accessMu
	result, err := reconciler.Reconcile(r.Context())
	if err != nil {
		s.metrics.recordDeploymentReconcile(time.Since(started), result.ProcessedInstances, true)
		var reconcileErr *controllerReconcilePhaseError
		if errors.As(err, &reconcileErr) {
			writeProblem(w, reconcileErr.StatusCode(), reconcileErr.Title(), reconcileErr.Detail())
			return
		}
		writeProblem(w, http.StatusInternalServerError, "Runtime reconcile failed", err.Error())
		return
	}
	s.metrics.recordDeploymentReconcile(time.Since(started), result.ProcessedInstances, false)
	writeData(w, http.StatusOK, result)
}

func (s *controllerServer) handleValidateChallengeRuntime(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	started := time.Now()
	var request apigateway.ChallengeValidationRequest
	if err := httpapi.DecodeJSON(r, &request); err != nil || request.ChallengeID <= 0 || request.BaselineImage == "" {
		s.metrics.recordOperation(controllerOperationChallengeValidate, time.Since(started), true)
		writeProblem(w, http.StatusBadRequest, "Invalid request", "challenge validation request is invalid.")
		return
	}
	result, err := s.executor.ValidateChallengeRuntime(r.Context(), request)
	if err != nil {
		s.metrics.recordOperation(controllerOperationChallengeValidate, time.Since(started), true)
		writeProblem(w, http.StatusInternalServerError, "Challenge validation failed", err.Error())
		return
	}
	s.metrics.recordOperation(controllerOperationChallengeValidate, time.Since(started), false)
	writeData(w, http.StatusOK, result)
}

func (s *controllerServer) handleAccessStatus(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	writeData(w, http.StatusOK, s.access.Status())
}

func (s *controllerServer) handleReconcileAccessPolicies(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	started := time.Now()
	s.accessMu.Lock()
	defer s.accessMu.Unlock()
	policies, err := s.store.ListControllerServiceAccessPolicies(r.Context())
	if err != nil {
		s.metrics.recordAccessReconcile(controllerAccessScopeGlobal, time.Since(started), 0, true)
		writeProblem(w, http.StatusInternalServerError, "Internal state unavailable", err.Error())
		return
	}
	status, err := s.access.Apply(r.Context(), policies, s.now())
	if err != nil {
		s.metrics.recordAccessReconcile(controllerAccessScopeGlobal, time.Since(started), 0, true)
		writeProblem(w, http.StatusInternalServerError, "Access reconcile failed", status.LastError)
		return
	}
	// Converge WireGuard after policy apply so tick-opened (play_from_tick) services
	// get host access + peer path together. The caller must not observe success until
	// both enforcement layers report an applied state.
	if requiresWireGuardConverge(status) {
		wireGuardStatus, wgErr := s.wireGuard.Reconcile(r.Context())
		if wgErr == nil {
			wgErr = validateWireGuardAppliedStatus(wireGuardStatus)
		}
		if wgErr != nil {
			log.Printf("controller: wireguard reconcile after access apply failed: %v", wgErr)
			s.metrics.recordAccessReconcile(controllerAccessScopeGlobal, time.Since(started), status.PoliciesTotal, true)
			writeProblem(w, http.StatusBadGateway, "Access reconcile failed", controllerWireGuardTruthUnknownDetail)
			return
		}
	}
	s.metrics.recordAccessReconcile(controllerAccessScopeGlobal, time.Since(started), status.PoliciesTotal, false)
	writeData(w, http.StatusOK, status)
}

func (s *controllerServer) handleTeardownAccessPolicies(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	started := time.Now()
	s.accessMu.Lock()
	defer s.accessMu.Unlock()
	if err := s.access.Teardown(r.Context()); err != nil {
		s.metrics.recordOperation(controllerOperationAccessTeardown, time.Since(started), true)
		writeProblem(w, http.StatusInternalServerError, "Access teardown failed", err.Error())
		return
	}
	s.metrics.recordOperation(controllerOperationAccessTeardown, time.Since(started), false)
	w.WriteHeader(http.StatusNoContent)
}

func (s *controllerServer) handleReconcileServiceAccess(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	started := time.Now()
	teamID, err := strconv.Atoi(r.PathValue("team_id"))
	if err != nil || teamID <= 0 {
		s.metrics.recordAccessReconcile(controllerAccessScopeService, time.Since(started), 0, true)
		writeProblem(w, http.StatusBadRequest, "Invalid request", "team id is invalid.")
		return
	}
	challengeID, err := strconv.Atoi(r.PathValue("challenge_id"))
	if err != nil || challengeID <= 0 {
		s.metrics.recordAccessReconcile(controllerAccessScopeService, time.Since(started), 0, true)
		writeProblem(w, http.StatusBadRequest, "Invalid request", "challenge id is invalid.")
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
		writeProblem(w, status, "Request failed", message)
		return
	}
	s.accessMu.Lock()
	defer s.accessMu.Unlock()
	policies, err := s.store.ListControllerServiceAccessPolicies(r.Context())
	if err != nil {
		s.metrics.recordAccessReconcile(controllerAccessScopeService, time.Since(started), 0, true)
		writeProblem(w, http.StatusInternalServerError, "Internal state unavailable", err.Error())
		return
	}
	status, err := s.access.Apply(r.Context(), policies, s.now())
	if err != nil {
		s.metrics.recordAccessReconcile(controllerAccessScopeService, time.Since(started), 0, true)
		writeProblem(w, http.StatusInternalServerError, "Access reconcile failed", status.LastError)
		return
	}
	s.metrics.recordAccessReconcile(controllerAccessScopeService, time.Since(started), status.PoliciesTotal, false)
	writeData(w, http.StatusOK, status)
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
	if err := httpapi.DecodeJSON(r, &credential); err != nil || credential.Password == "" {
		s.metrics.recordOperation(controllerOperationSSHCredential, time.Since(started), true)
		writeProblem(w, http.StatusBadRequest, "Invalid request", "ssh credential request is invalid.")
		return
	}
	if err := s.executor.ApplySSHCredential(r.Context(), task, credential); err != nil {
		s.metrics.recordOperation(controllerOperationSSHCredential, time.Since(started), true)
		writeProblem(w, http.StatusInternalServerError, "SSH credential apply failed", err.Error())
		return
	}
	s.metrics.recordOperation(controllerOperationSSHCredential, time.Since(started), false)
	writeData(w, http.StatusOK, map[string]any{
		"team_id": task.TeamID, "challenge_id": task.ChallengeID, "action": "apply_ssh_credential_runtime",
	})
}

func (s *controllerServer) handleFactoryResetService(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	started := time.Now()
	w.Header().Set("X-ADPlatform-Reset-Predelete", "clean")
	task, ok := s.parseRuntimeTask(w, r)
	if !ok {
		s.metrics.recordOperation(controllerOperationFactoryReset, time.Since(started), true)
		return
	}
	w.Header().Del("X-ADPlatform-Reset-Predelete")
	if err := s.executor.FactoryResetService(r.Context(), task); err != nil {
		s.metrics.recordOperation(controllerOperationFactoryReset, time.Since(started), true)
		var preflight resetPreflightError
		if errors.As(err, &preflight) {
			stage := "clean"
			if preflight.runtimeMayHaveChanged {
				stage = "runtime-changed"
			}
			w.Header().Set("X-ADPlatform-Reset-Predelete", stage)
		}
		writeProblem(w, http.StatusInternalServerError, "Factory reset failed", err.Error())
		return
	}
	s.metrics.recordOperation(controllerOperationFactoryReset, time.Since(started), false)
	writeData(w, http.StatusOK, map[string]any{
		"team_id": task.TeamID, "challenge_id": task.ChallengeID, "action": "factory_reset_runtime",
	})
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
		writeProblem(w, http.StatusInternalServerError, "Restart failed", err.Error())
		return
	}
	s.metrics.recordOperation(controllerOperationRestart, time.Since(started), false)
	writeData(w, http.StatusOK, map[string]any{
		"team_id": task.TeamID, "challenge_id": task.ChallengeID, "action": "restart_runtime",
	})
}

func (s *controllerServer) handleRemoveService(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	started := time.Now()
	teamID, err := strconv.Atoi(r.PathValue("team_id"))
	if err != nil || teamID <= 0 {
		s.metrics.recordOperation(controllerOperationRemoveService, time.Since(started), true)
		writeProblem(w, http.StatusBadRequest, "Invalid request", "team id is invalid.")
		return
	}
	challengeID, err := strconv.Atoi(r.PathValue("challenge_id"))
	if err != nil || challengeID <= 0 {
		s.metrics.recordOperation(controllerOperationRemoveService, time.Since(started), true)
		writeProblem(w, http.StatusBadRequest, "Invalid request", "challenge id is invalid.")
		return
	}
	if err := s.executor.RemoveService(r.Context(), teamID, challengeID); err != nil {
		s.metrics.recordOperation(controllerOperationRemoveService, time.Since(started), true)
		writeProblem(w, http.StatusInternalServerError, "Remove service failed", err.Error())
		return
	}
	s.metrics.recordOperation(controllerOperationRemoveService, time.Since(started), false)
	writeData(w, http.StatusOK, map[string]any{
		"team_id": teamID, "challenge_id": challengeID, "action": "remove_runtime",
	})
}

func (s *controllerServer) handleRemoveTeamServices(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	started := time.Now()
	teamID, err := strconv.Atoi(r.PathValue("team_id"))
	if err != nil || teamID <= 0 {
		s.metrics.recordOperation(controllerOperationRemoveTeam, time.Since(started), true)
		writeProblem(w, http.StatusBadRequest, "Invalid request", "team id is invalid.")
		return
	}
	volumes, err := s.store.ListControllerTeamStateVolumes(r.Context(), teamID)
	if err != nil {
		s.metrics.recordOperation(controllerOperationRemoveTeam, time.Since(started), true)
		writeProblem(w, http.StatusInternalServerError, "Remove team services failed", "team state volumes could not be listed.")
		return
	}
	if err := s.executor.RemoveTeamServices(r.Context(), teamID, volumes...); err != nil {
		s.metrics.recordOperation(controllerOperationRemoveTeam, time.Since(started), true)
		writeProblem(w, http.StatusInternalServerError, "Remove team services failed", err.Error())
		return
	}
	s.metrics.recordOperation(controllerOperationRemoveTeam, time.Since(started), false)
	writeData(w, http.StatusOK, map[string]any{
		"team_id": teamID, "action": "remove_team_runtimes",
	})
}

func (s *controllerServer) handleRemoveChallengeServices(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	started := time.Now()
	challengeID, err := strconv.Atoi(r.PathValue("challenge_id"))
	if err != nil || challengeID <= 0 {
		s.metrics.recordOperation(controllerOperationRemoveChallenge, time.Since(started), true)
		writeProblem(w, http.StatusBadRequest, "Invalid request", "challenge id is invalid.")
		return
	}
	if err := s.executor.RemoveChallengeServices(r.Context(), challengeID); err != nil {
		s.metrics.recordOperation(controllerOperationRemoveChallenge, time.Since(started), true)
		writeProblem(w, http.StatusInternalServerError, "Remove challenge services failed", err.Error())
		return
	}
	s.metrics.recordOperation(controllerOperationRemoveChallenge, time.Since(started), false)
	writeData(w, http.StatusOK, map[string]any{
		"challenge_id": challengeID, "action": "remove_challenge_runtimes",
	})
}

func (s *controllerServer) parseRuntimeTask(w http.ResponseWriter, r *http.Request) (apigateway.ControllerRuntimeTask, bool) {
	teamID, err := strconv.Atoi(r.PathValue("team_id"))
	if err != nil || teamID <= 0 {
		writeProblem(w, http.StatusBadRequest, "Invalid request", "team id is invalid.")
		return apigateway.ControllerRuntimeTask{}, false
	}
	challengeID, err := strconv.Atoi(r.PathValue("challenge_id"))
	if err != nil || challengeID <= 0 {
		writeProblem(w, http.StatusBadRequest, "Invalid request", "challenge id is invalid.")
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
		writeProblem(w, status, "Request failed", message)
		return apigateway.ControllerRuntimeTask{}, false
	}
	return task, true
}

func (s *controllerServer) requireAdminAuth(w http.ResponseWriter, r *http.Request) bool {
	token, ok := httpapi.BearerToken(r)
	if !ok || subtle.ConstantTimeCompare([]byte(token), []byte(s.adminToken)) != 1 {
		writeProblem(w, http.StatusForbidden, "Forbidden", "please authenticate before accessing controller endpoints.")
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
