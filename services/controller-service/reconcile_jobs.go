package main

import (
	"context"
	"crypto/rand"
	"log"
	"net/http"
	"sync"
	"time"

	"adplatform/internal/services/apigateway"
)

type deploymentReconcileJobs struct {
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.Mutex
	job    apigateway.ControllerDeploymentReconcileJob
	done   chan struct{}
}

func (j *deploymentReconcileJobs) shutdown() {
	j.cancel()
	j.mu.Lock()
	done := j.done
	j.mu.Unlock()
	if done != nil {
		<-done
	}
}

func (s *controllerServer) handleDeploymentReconcileJob(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	jobs := &s.reconcileJobs
	jobs.mu.Lock()
	job := jobs.job
	if r.Method == http.MethodGet {
		jobs.mu.Unlock()
		if job.Status == "" {
			job.Status = "idle"
		}
		writeData(w, http.StatusOK, job)
		return
	}
	if job.Status == "running" {
		jobs.mu.Unlock()
		writeData(w, http.StatusAccepted, job)
		return
	}
	if jobs.ctx == nil || jobs.ctx.Err() != nil {
		jobs.mu.Unlock()
		writeProblem(w, http.StatusServiceUnavailable, "Deployment reconcile unavailable", "The controller is shutting down.")
		return
	}
	if !s.reconcileMu.TryLock() {
		jobs.mu.Unlock()
		writeProblem(w, http.StatusConflict, "Deployment reconcile already running", "Wait for the current deployment reconcile to finish.")
		return
	}
	job = apigateway.ControllerDeploymentReconcileJob{ID: rand.Text(), Status: "running", Phase: "starting"}
	jobs.job = job
	jobs.done = make(chan struct{})
	jobs.mu.Unlock()
	go s.runDeploymentReconcileJob(job.ID)
	writeData(w, http.StatusAccepted, job)
}

func (s *controllerServer) runDeploymentReconcileJob(id string) {
	jobs := &s.reconcileJobs
	result, err := s.reconcileDeployments(jobs.ctx, func(phase string, ensured, total int) {
		jobs.mu.Lock()
		jobs.job.Phase, jobs.job.Ensured, jobs.job.Total = phase, ensured, total
		jobs.mu.Unlock()
	})
	if err != nil {
		log.Printf("deployment reconcile job %s failed: %v", id, err)
	} else {
		log.Printf("deployment reconcile job %s completed %d jobs and processed %d instances", id, result.CompletedJobs, result.ProcessedInstances)
	}
	jobs.mu.Lock()
	jobs.job.Result = &result
	if err != nil {
		jobs.job.Status = "failed"
		jobs.job.Error = err.Error()
	} else {
		jobs.job.Status = "completed"
	}
	close(jobs.done)
	s.reconcileMu.Unlock()
	jobs.mu.Unlock()
}

func (s *controllerServer) reconcileDeployments(ctx context.Context, onProgress func(string, int, int)) (controllerDeploymentReconcileState, error) {
	started := time.Now()
	reconciler := newControllerTrustedReconciler(controllerReconcileStoreAdapter{store: s.store}, s.executor, s.access, s.wireGuard, s.now)
	reconciler.accessMu = &s.accessMu
	reconciler.onProgress = onProgress
	result, err := reconciler.Reconcile(ctx)
	s.metrics.recordDeploymentReconcile(time.Since(started), result.ProcessedInstances, err != nil)
	return result, err
}
