package apigateway

// Code generated for tests from client interfaces; every method fails.

import (
	"context"
	"errors"
)

var errUpstreamDown = errors.New("simulated upstream outage")

type failingGameCoreClient struct{}

var _ gameCoreClient = failingGameCoreClient{}

func (failingGameCoreClient) Submissions(a0 context.Context, a1 SubmissionQuery) (r0 SubmissionPage, err error) {
	err = errUpstreamDown
	return
}

func (failingGameCoreClient) DeleteSubmission(a0 context.Context, a1 int64) (r0 SubmissionDeletion, err error) {
	err = errUpstreamDown
	return
}

func (failingGameCoreClient) Status(ctx context.Context) (r0 GameStatus, err error) {
	err = errUpstreamDown
	return
}

func (failingGameCoreClient) MatchStatus(ctx context.Context) (r0 GameMatchStatus, err error) {
	err = errUpstreamDown
	return
}

func (failingGameCoreClient) StartMatch(ctx context.Context) (r0 GameMatchStatus, err error) {
	err = errUpstreamDown
	return
}

func (failingGameCoreClient) PauseMatch(ctx context.Context) (r0 GameMatchStatus, err error) {
	err = errUpstreamDown
	return
}

func (failingGameCoreClient) ResumeMatch(ctx context.Context) (r0 GameMatchStatus, err error) {
	err = errUpstreamDown
	return
}

func (failingGameCoreClient) StopMatch(ctx context.Context) (r0 GameMatchStatus, err error) {
	err = errUpstreamDown
	return
}

func (failingGameCoreClient) UpdateMatchSchedule(ctx context.Context, req UpdateMatchScheduleRequest) (r0 GameMatchStatus, err error) {
	err = errUpstreamDown
	return
}

func (failingGameCoreClient) AdvanceTick(ctx context.Context) (r0 GameTickStatus, err error) {
	err = errUpstreamDown
	return
}

func (failingGameCoreClient) CheckerRuns(ctx context.Context, query GameCheckerRunQuery) (r0 GameCheckerRunPage, err error) {
	err = errUpstreamDown
	return
}

func (failingGameCoreClient) SchedulerStatus(ctx context.Context) (r0 GameSchedulerStatus, err error) {
	err = errUpstreamDown
	return
}

func (failingGameCoreClient) SchedulerEvents(ctx context.Context, query GameSchedulerEventQuery) (r0 GameSchedulerEventPage, err error) {
	err = errUpstreamDown
	return
}

func (failingGameCoreClient) StartScheduler(ctx context.Context) (r0 GameSchedulerStatus, err error) {
	err = errUpstreamDown
	return
}

func (failingGameCoreClient) StopScheduler(ctx context.Context) (r0 GameSchedulerStatus, err error) {
	err = errUpstreamDown
	return
}

func (failingGameCoreClient) UpdateScheduler(ctx context.Context, req UpdateSchedulerRequest) (r0 GameSchedulerStatus, err error) {
	err = errUpstreamDown
	return
}

func (failingGameCoreClient) SubmitFlags(ctx context.Context, teamID int, flags []string) (r0 []submissionVerdict, err error) {
	err = errUpstreamDown
	return
}

func (failingGameCoreClient) Scoreboard(ctx context.Context) (r0 []scoreRow, err error) {
	err = errUpstreamDown
	return
}

func (failingGameCoreClient) AttackFeed(ctx context.Context, query AttackFeedQuery) (r0 AttackFeedPage, err error) {
	err = errUpstreamDown
	return
}

func (failingGameCoreClient) RecomputeScoring(ctx context.Context) (r0 []scoreRow, err error) {
	err = errUpstreamDown
	return
}

func (failingGameCoreClient) AuditScoring(ctx context.Context) (r0 ScoringAuditAlias, err error) {
	err = errUpstreamDown
	return
}

func (failingGameCoreClient) FlagFormat(ctx context.Context) (r0 FlagFormatStatus, err error) {
	err = errUpstreamDown
	return
}

func (failingGameCoreClient) RefreshFlagFormat(ctx context.Context, prefix string) (r0 FlagFormatStatus, err error) {
	err = errUpstreamDown
	return
}

type failingControllerClient struct{}

var _ controllerClient = failingControllerClient{}

func (failingControllerClient) DeploymentReconcileJob(context.Context, string) (ControllerDeploymentReconcileJob, error) {
	return ControllerDeploymentReconcileJob{}, errUpstreamDown
}

func (failingControllerClient) ReconcileDeployments(ctx context.Context) (r0 adminReconcileResult, err error) {
	err = errUpstreamDown
	return
}

func (failingControllerClient) FactoryResetService(ctx context.Context, teamID, challengeID int) (err error) {
	err = errUpstreamDown
	return
}

func (failingControllerClient) RestartService(ctx context.Context, teamID, challengeID int) (err error) {
	err = errUpstreamDown
	return
}

func (failingControllerClient) ReconcileServiceAccess(ctx context.Context, teamID, challengeID int) (err error) {
	err = errUpstreamDown
	return
}

func (failingControllerClient) ApplySSHCredential(ctx context.Context, teamID, challengeID int, credential ControllerSSHCredential) (err error) {
	err = errUpstreamDown
	return
}

func (failingControllerClient) ValidateChallengeRuntime(ctx context.Context, request ChallengeValidationRequest) (r0 ChallengeValidationResult, err error) {
	err = errUpstreamDown
	return
}

func (failingControllerClient) AccessStatus(ctx context.Context) (r0 ControllerAccessStatus, err error) {
	err = errUpstreamDown
	return
}

func (failingControllerClient) ReconcileAccessPolicies(ctx context.Context) (r0 ControllerAccessStatus, err error) {
	err = errUpstreamDown
	return
}

func (failingControllerClient) TeardownAccessPolicies(ctx context.Context) (err error) {
	err = errUpstreamDown
	return
}

func (failingControllerClient) RemoveService(ctx context.Context, teamID, challengeID int) (err error) {
	err = errUpstreamDown
	return
}

func (failingControllerClient) RemoveTeamServices(ctx context.Context, teamID int) (err error) {
	err = errUpstreamDown
	return
}

func (failingControllerClient) RemoveChallengeServices(ctx context.Context, challengeID int) (err error) {
	err = errUpstreamDown
	return
}

type failingWireGuardClient struct{}

var _ wireGuardClient = failingWireGuardClient{}

func (failingWireGuardClient) Status(ctx context.Context) (r0 WireGuardGatewayStatus, err error) {
	err = errUpstreamDown
	return
}

func (failingWireGuardClient) Reconcile(ctx context.Context) (r0 WireGuardGatewayStatus, err error) {
	err = errUpstreamDown
	return
}

func (failingWireGuardClient) Teardown(ctx context.Context) (err error) {
	err = errUpstreamDown
	return
}
