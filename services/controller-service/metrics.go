package main

import (
	"fmt"
	"io"
	"sync"
	"time"

	"adplatform/internal/services/apigateway"
)

const (
	controllerOperationDeploymentReconcile = "deployment_reconcile"
	controllerOperationChallengeValidate   = "challenge_validate"
	controllerOperationAccessTeardown      = "access_teardown"
	controllerOperationSSHCredential       = "ssh_credential"
	controllerOperationFactoryReset        = "factory_reset"
	controllerOperationRestart             = "restart"
	controllerOperationRemoveService       = "remove_service"
	controllerOperationRemoveTeam          = "remove_team"
	controllerOperationRemoveChallenge     = "remove_challenge"

	controllerAccessScopeGlobal  = "global"
	controllerAccessScopeService = "service"
)

var controllerMetricOperations = []string{
	controllerOperationDeploymentReconcile,
	controllerOperationChallengeValidate,
	"access_reconcile",
	"service_access_reconcile",
	controllerOperationAccessTeardown,
	controllerOperationSSHCredential,
	controllerOperationFactoryReset,
	controllerOperationRestart,
	controllerOperationRemoveService,
	controllerOperationRemoveTeam,
	controllerOperationRemoveChallenge,
}

var controllerAccessScopes = []string{
	controllerAccessScopeGlobal,
	controllerAccessScopeService,
}

type controllerOperationAggregate struct {
	requestsTotal        uint64
	failuresTotal        uint64
	durationSecondsSum   float64
	durationSecondsCount uint64
}

type controllerServiceMetrics struct {
	mu sync.Mutex

	operations                 map[string]controllerOperationAggregate
	deploymentTasksEnsured     uint64
	accessPoliciesAppliedTotal map[string]uint64
}

func newControllerServiceMetrics() controllerServiceMetrics {
	metrics := controllerServiceMetrics{
		operations:                 make(map[string]controllerOperationAggregate, len(controllerMetricOperations)),
		accessPoliciesAppliedTotal: make(map[string]uint64, len(controllerAccessScopes)),
	}
	for _, operation := range controllerMetricOperations {
		metrics.operations[operation] = controllerOperationAggregate{}
	}
	for _, scope := range controllerAccessScopes {
		metrics.accessPoliciesAppliedTotal[scope] = 0
	}
	return metrics
}

func (m *controllerServiceMetrics) recordDeploymentReconcile(duration time.Duration, ensured int, failed bool) {
	m.recordOperation(controllerOperationDeploymentReconcile, duration, failed)
	if ensured <= 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deploymentTasksEnsured += uint64(ensured)
}

func (m *controllerServiceMetrics) recordAccessReconcile(scope string, duration time.Duration, policiesApplied int, failed bool) {
	operation := "access_reconcile"
	if scope == controllerAccessScopeService {
		operation = "service_access_reconcile"
	}
	m.recordOperation(operation, duration, failed)
	if policiesApplied <= 0 || (scope != controllerAccessScopeGlobal && scope != controllerAccessScopeService) {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensureMapsLocked()
	m.accessPoliciesAppliedTotal[scope] += uint64(policiesApplied)
}

func (m *controllerServiceMetrics) recordOperation(operation string, duration time.Duration, failed bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensureMapsLocked()
	entry := m.operations[operation]
	entry.requestsTotal++
	if failed {
		entry.failuresTotal++
	}
	entry.durationSecondsSum += duration.Seconds()
	entry.durationSecondsCount++
	m.operations[operation] = entry
}

func (m *controllerServiceMetrics) ensureMapsLocked() {
	if m.operations == nil {
		m.operations = make(map[string]controllerOperationAggregate, len(controllerMetricOperations))
	}
	for _, operation := range controllerMetricOperations {
		if _, ok := m.operations[operation]; !ok {
			m.operations[operation] = controllerOperationAggregate{}
		}
	}
	if m.accessPoliciesAppliedTotal == nil {
		m.accessPoliciesAppliedTotal = make(map[string]uint64, len(controllerAccessScopes))
	}
	for _, scope := range controllerAccessScopes {
		if _, ok := m.accessPoliciesAppliedTotal[scope]; !ok {
			m.accessPoliciesAppliedTotal[scope] = 0
		}
	}
}

func (s *controllerServer) WritePrometheusMetrics(w io.Writer) {
	status := apigateway.ControllerAccessStatus{}
	if s.access != nil {
		status = s.access.Status()
	}

	s.metrics.mu.Lock()
	s.metrics.ensureMapsLocked()
	operations := make(map[string]controllerOperationAggregate, len(s.metrics.operations))
	for key, value := range s.metrics.operations {
		operations[key] = value
	}
	accessPoliciesApplied := make(map[string]uint64, len(s.metrics.accessPoliciesAppliedTotal))
	for key, value := range s.metrics.accessPoliciesAppliedTotal {
		accessPoliciesApplied[key] = value
	}
	deploymentTasksEnsured := s.metrics.deploymentTasksEnsured
	s.metrics.mu.Unlock()

	fmt.Fprintln(w, "# HELP adplatform_controller_service_metrics_collection_success Whether controller-service metrics collection succeeded.")
	fmt.Fprintln(w, "# TYPE adplatform_controller_service_metrics_collection_success gauge")
	fmt.Fprintln(w, "adplatform_controller_service_metrics_collection_success 1")

	fmt.Fprintln(w, "# HELP adplatform_controller_service_operation_requests_total Total controller-service requests grouped by operation.")
	fmt.Fprintln(w, "# TYPE adplatform_controller_service_operation_requests_total counter")
	for _, operation := range controllerMetricOperations {
		fmt.Fprintf(w, "adplatform_controller_service_operation_requests_total{operation=%q} %d\n", operation, operations[operation].requestsTotal)
	}

	fmt.Fprintln(w, "# HELP adplatform_controller_service_operation_failures_total Total failed controller-service requests grouped by operation.")
	fmt.Fprintln(w, "# TYPE adplatform_controller_service_operation_failures_total counter")
	for _, operation := range controllerMetricOperations {
		fmt.Fprintf(w, "adplatform_controller_service_operation_failures_total{operation=%q} %d\n", operation, operations[operation].failuresTotal)
	}

	fmt.Fprintln(w, "# HELP adplatform_controller_service_operation_duration_seconds_sum Total controller-service operation duration in seconds.")
	fmt.Fprintln(w, "# TYPE adplatform_controller_service_operation_duration_seconds_sum counter")
	for _, operation := range controllerMetricOperations {
		fmt.Fprintf(w, "adplatform_controller_service_operation_duration_seconds_sum{operation=%q} %.6f\n", operation, operations[operation].durationSecondsSum)
	}

	fmt.Fprintln(w, "# HELP adplatform_controller_service_operation_duration_seconds_count Total observed controller-service operations for duration accounting.")
	fmt.Fprintln(w, "# TYPE adplatform_controller_service_operation_duration_seconds_count counter")
	for _, operation := range controllerMetricOperations {
		fmt.Fprintf(w, "adplatform_controller_service_operation_duration_seconds_count{operation=%q} %d\n", operation, operations[operation].durationSecondsCount)
	}

	fmt.Fprintln(w, "# HELP adplatform_controller_service_deployment_tasks_ensured_total Total runtime tasks ensured during deployment reconcile operations.")
	fmt.Fprintln(w, "# TYPE adplatform_controller_service_deployment_tasks_ensured_total counter")
	fmt.Fprintf(w, "adplatform_controller_service_deployment_tasks_ensured_total %d\n", deploymentTasksEnsured)

	fmt.Fprintln(w, "# HELP adplatform_controller_service_access_policies_applied_total Total access policies applied by reconcile scope.")
	fmt.Fprintln(w, "# TYPE adplatform_controller_service_access_policies_applied_total counter")
	for _, scope := range controllerAccessScopes {
		fmt.Fprintf(w, "adplatform_controller_service_access_policies_applied_total{scope=%q} %d\n", scope, accessPoliciesApplied[scope])
	}

	fmt.Fprintln(w, "# HELP adplatform_controller_service_access_state Whether the controller access layer currently reports the given state.")
	fmt.Fprintln(w, "# TYPE adplatform_controller_service_access_state gauge")
	for _, state := range []string{"idle", "applied", "error"} {
		fmt.Fprintf(w, "adplatform_controller_service_access_state{state=%q} %.0f\n", state, metricBool(status.State == state))
	}

	fmt.Fprintln(w, "# HELP adplatform_controller_service_access_policies_total Current number of access policies tracked by the controller.")
	fmt.Fprintln(w, "# TYPE adplatform_controller_service_access_policies_total gauge")
	fmt.Fprintf(w, "adplatform_controller_service_access_policies_total %d\n", status.PoliciesTotal)

	fmt.Fprintln(w, "# HELP adplatform_controller_service_access_ssh_open_services Current number of services with SSH access open.")
	fmt.Fprintln(w, "# TYPE adplatform_controller_service_access_ssh_open_services gauge")
	fmt.Fprintf(w, "adplatform_controller_service_access_ssh_open_services %d\n", status.SSHOpenServices)

	fmt.Fprintln(w, "# HELP adplatform_controller_service_access_ssh_locked_services Current number of services with SSH access locked.")
	fmt.Fprintln(w, "# TYPE adplatform_controller_service_access_ssh_locked_services gauge")
	fmt.Fprintf(w, "adplatform_controller_service_access_ssh_locked_services %d\n", status.SSHLockedServices)

	fmt.Fprintln(w, "# HELP adplatform_controller_service_access_allowed_peers_total Current number of allowed SSH peers tracked by the controller.")
	fmt.Fprintln(w, "# TYPE adplatform_controller_service_access_allowed_peers_total gauge")
	fmt.Fprintf(w, "adplatform_controller_service_access_allowed_peers_total %d\n", status.AllowedPeersTotal)

	fmt.Fprintln(w, "# HELP adplatform_controller_service_access_last_apply_success Whether the controller access layer last reported a successful apply.")
	fmt.Fprintln(w, "# TYPE adplatform_controller_service_access_last_apply_success gauge")
	fmt.Fprintf(w, "adplatform_controller_service_access_last_apply_success %.0f\n", metricBool(status.State == "applied" && status.LastError == ""))

	fmt.Fprintln(w, "# HELP adplatform_controller_service_access_last_error Whether the controller access layer currently reports a last_error value.")
	fmt.Fprintln(w, "# TYPE adplatform_controller_service_access_last_error gauge")
	fmt.Fprintf(w, "adplatform_controller_service_access_last_error %.0f\n", metricBool(status.LastError != ""))

	if appliedAt := metricParseTime(status.AppliedAt); !appliedAt.IsZero() {
		fmt.Fprintln(w, "# HELP adplatform_controller_service_access_applied_at_unixtime Last access policy apply time as a Unix timestamp.")
		fmt.Fprintln(w, "# TYPE adplatform_controller_service_access_applied_at_unixtime gauge")
		fmt.Fprintf(w, "adplatform_controller_service_access_applied_at_unixtime %.0f\n", float64(appliedAt.Unix()))
	}
}

var _ interface{ WritePrometheusMetrics(io.Writer) } = (*controllerServer)(nil)

func metricBool(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

func metricParseTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return parsed.UTC()
}
