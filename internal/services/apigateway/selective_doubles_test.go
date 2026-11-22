package apigateway

// Code generated for tests: each sel* double delegates to inner unless its
// fail set names the method, which then returns that error.

import (
	"context"
	"time"
)

// failSet maps method names to the error they return.
type failSet map[string]error

type selStore struct {
	inner Store
	fail  failSet
}

var _ Store = selStore{}

func (s selStore) AuthenticatePlayer(ctx context.Context, email, password string) (r0 authenticatedPlayer, err error) {
	if err = s.fail["AuthenticatePlayer"]; err != nil {
		return
	}
	return s.inner.AuthenticatePlayer(ctx, email, password)
}

func (s selStore) RegisterPlayer(ctx context.Context, input participantRegisterRequest, now time.Time) (r0 authenticatedPlayer, err error) {
	if err = s.fail["RegisterPlayer"]; err != nil {
		return
	}
	return s.inner.RegisterPlayer(ctx, input, now)
}

func (s selStore) ValidatePlayerSession(ctx context.Context, playerID, teamID int, role string, sessionVersion int) (r0 authenticatedPlayer, err error) {
	if err = s.fail["ValidatePlayerSession"]; err != nil {
		return
	}
	return s.inner.ValidatePlayerSession(ctx, playerID, teamID, role, sessionVersion)
}

func (s selStore) IsPlayerSessionRevoked(ctx context.Context, sessionID string) (bool, error) {
	if err := s.fail["IsPlayerSessionRevoked"]; err != nil {
		return false, err
	}
	return s.inner.IsPlayerSessionRevoked(ctx, sessionID)
}

func (s selStore) RevokePlayerSession(ctx context.Context, sessionID string, expires, now time.Time) error {
	if err := s.fail["RevokePlayerSession"]; err != nil {
		return err
	}
	return s.inner.RevokePlayerSession(ctx, sessionID, expires, now)
}

func (s selStore) UpdateParticipantProfile(ctx context.Context, playerID int, input participantUpdateProfileRequest) (r0 authenticatedPlayer, err error) {
	if err = s.fail["UpdateParticipantProfile"]; err != nil {
		return
	}
	return s.inner.UpdateParticipantProfile(ctx, playerID, input)
}

func (s selStore) ChangeParticipantPassword(ctx context.Context, playerID int, currentPassword, newPassword string) (r0 int, err error) {
	if err = s.fail["ChangeParticipantPassword"]; err != nil {
		return
	}
	return s.inner.ChangeParticipantPassword(ctx, playerID, currentPassword, newPassword)
}

func (s selStore) ListAnnouncements(ctx context.Context) (r0 []matchAnnouncement, err error) {
	if err = s.fail["ListAnnouncements"]; err != nil {
		return
	}
	return s.inner.ListAnnouncements(ctx)
}

func (s selStore) CreateAnnouncement(ctx context.Context, body, createdBy string, now time.Time) (r0 matchAnnouncement, err error) {
	if err = s.fail["CreateAnnouncement"]; err != nil {
		return
	}
	return s.inner.CreateAnnouncement(ctx, body, createdBy, now)
}

func (s selStore) DeleteAnnouncement(ctx context.Context, id int) (err error) {
	if err = s.fail["DeleteAnnouncement"]; err != nil {
		return
	}
	return s.inner.DeleteAnnouncement(ctx, id)
}

func (s selStore) ImportAdminTeams(ctx context.Context, teams []adminBulkImportTeam, now time.Time) (r0 adminBulkImportResult, err error) {
	if err = s.fail["ImportAdminTeams"]; err != nil {
		return
	}
	return s.inner.ImportAdminTeams(ctx, teams, now)
}

func (s selStore) ListChallenges(ctx context.Context) (r0 []challenge, err error) {
	if err = s.fail["ListChallenges"]; err != nil {
		return
	}
	return s.inner.ListChallenges(ctx)
}

func (s selStore) ListPublicServices(ctx context.Context) (r0 map[string]map[string][]string, err error) {
	if err = s.fail["ListPublicServices"]; err != nil {
		return
	}
	return s.inner.ListPublicServices(ctx)
}

func (s selStore) ListScoreboard(ctx context.Context) (r0 []scoreRow, err error) {
	if err = s.fail["ListScoreboard"]; err != nil {
		return
	}
	return s.inner.ListScoreboard(ctx)
}

func (s selStore) GetScoreboardFreeze(ctx context.Context) (r0 scoreboardFreezeWindow, err error) {
	if err = s.fail["GetScoreboardFreeze"]; err != nil {
		return
	}
	return s.inner.GetScoreboardFreeze(ctx)
}

func (s selStore) SetScoreboardFreezeWindow(ctx context.Context, freezeAt, unfreezeAt *time.Time, now time.Time) (r0 scoreboardFreezeWindow, err error) {
	if err = s.fail["SetScoreboardFreezeWindow"]; err != nil {
		return
	}
	return s.inner.SetScoreboardFreezeWindow(ctx, freezeAt, unfreezeAt, now)
}

func (s selStore) ClearScoreboardFreeze(ctx context.Context, now time.Time) (r0 scoreboardFreezeWindow, err error) {
	if err = s.fail["ClearScoreboardFreeze"]; err != nil {
		return
	}
	return s.inner.ClearScoreboardFreeze(ctx, now)
}

func (s selStore) SaveFrozenScoreboardSnapshot(ctx context.Context, rows []scoreRow, takenAt time.Time) (r0 scoreboardFreezeWindow, err error) {
	if err = s.fail["SaveFrozenScoreboardSnapshot"]; err != nil {
		return
	}
	return s.inner.SaveFrozenScoreboardSnapshot(ctx, rows, takenAt)
}

func (s selStore) ListAttackFeed(ctx context.Context) (r0 []attackEvent, err error) {
	if err = s.fail["ListAttackFeed"]; err != nil {
		return
	}
	return s.inner.ListAttackFeed(ctx)
}

func (s selStore) ListTeamServices(ctx context.Context, teamID int) (r0 []serviceState, err error) {
	if err = s.fail["ListTeamServices"]; err != nil {
		return
	}
	return s.inner.ListTeamServices(ctx, teamID)
}

func (s selStore) SubmitFlags(ctx context.Context, teamID int, flags []string) (r0 []submissionVerdict, err error) {
	if err = s.fail["SubmitFlags"]; err != nil {
		return
	}
	return s.inner.SubmitFlags(ctx, teamID, flags)
}

func (s selStore) ValidateServiceAction(ctx context.Context, teamID, challengeID int) (err error) {
	if err = s.fail["ValidateServiceAction"]; err != nil {
		return
	}
	return s.inner.ValidateServiceAction(ctx, teamID, challengeID)
}

func (s selStore) UnlockService(ctx context.Context, teamID, challengeID int) (r0 unlockData, err error) {
	if err = s.fail["UnlockService"]; err != nil {
		return
	}
	return s.inner.UnlockService(ctx, teamID, challengeID)
}

func (s selStore) CreateSSHSession(ctx context.Context, teamID, challengeID int, now time.Time) (r0 sshSessionData, err error) {
	if err = s.fail["CreateSSHSession"]; err != nil {
		return
	}
	return s.inner.CreateSSHSession(ctx, teamID, challengeID, now)
}

func (s selStore) MarkSSHSessionApplyFailure(ctx context.Context, teamID, challengeID int) (err error) {
	if err = s.fail["MarkSSHSessionApplyFailure"]; err != nil {
		return
	}
	return s.inner.MarkSSHSessionApplyFailure(ctx, teamID, challengeID)
}

func (s selStore) LockServiceLifecycle(ctx context.Context, teamID, challengeID int) (r0 func(), err error) {
	if err = s.fail["LockServiceLifecycle"]; err != nil {
		return
	}
	return s.inner.LockServiceLifecycle(ctx, teamID, challengeID)
}

func (s selStore) PrepareFactoryResetService(ctx context.Context, teamID, challengeID int) (r0 resetData, err error) {
	if err = s.fail["PrepareFactoryResetService"]; err != nil {
		return
	}
	return s.inner.PrepareFactoryResetService(ctx, teamID, challengeID)
}

func (s selStore) AbortFactoryResetService(ctx context.Context, prepared resetData) (err error) {
	if err = s.fail["AbortFactoryResetService"]; err != nil {
		return
	}
	return s.inner.AbortFactoryResetService(ctx, prepared)
}

func (s selStore) CompleteFactoryResetService(ctx context.Context, teamID, challengeID int) (r0 resetData, err error) {
	if err = s.fail["CompleteFactoryResetService"]; err != nil {
		return
	}
	return s.inner.CompleteFactoryResetService(ctx, teamID, challengeID)
}

func (s selStore) MarkFactoryResetFailure(ctx context.Context, teamID, challengeID int) (err error) {
	if err = s.fail["MarkFactoryResetFailure"]; err != nil {
		return
	}
	return s.inner.MarkFactoryResetFailure(ctx, teamID, challengeID)
}

func (s selStore) MarkFactoryResetCompletionFailure(ctx context.Context, teamID, challengeID int) (err error) {
	if err = s.fail["MarkFactoryResetCompletionFailure"]; err != nil {
		return
	}
	return s.inner.MarkFactoryResetCompletionFailure(ctx, teamID, challengeID)
}

func (s selStore) ValidateRestartService(ctx context.Context, teamID, challengeID int) (err error) {
	if err = s.fail["ValidateRestartService"]; err != nil {
		return
	}
	return s.inner.ValidateRestartService(ctx, teamID, challengeID)
}

func (s selStore) RestartService(ctx context.Context, teamID, challengeID int) (r0 resetData, err error) {
	if err = s.fail["RestartService"]; err != nil {
		return
	}
	return s.inner.RestartService(ctx, teamID, challengeID)
}

func (s selStore) ListAdminTeams(ctx context.Context) (r0 []adminTeam, err error) {
	if err = s.fail["ListAdminTeams"]; err != nil {
		return
	}
	return s.inner.ListAdminTeams(ctx)
}

func (s selStore) CreateAdminTeam(ctx context.Context, input adminCreateTeamRequest) (r0 adminTeam, err error) {
	if err = s.fail["CreateAdminTeam"]; err != nil {
		return
	}
	return s.inner.CreateAdminTeam(ctx, input)
}

func (s selStore) UpdateAdminTeam(ctx context.Context, teamID int, input adminUpdateTeamRequest) (r0 adminTeam, err error) {
	if err = s.fail["UpdateAdminTeam"]; err != nil {
		return
	}
	return s.inner.UpdateAdminTeam(ctx, teamID, input)
}

func (s selStore) RotateAdminTeamJoinKey(ctx context.Context, teamID int) (string, error) {
	if err := s.fail["RotateAdminTeamJoinKey"]; err != nil {
		return "", err
	}
	return s.inner.RotateAdminTeamJoinKey(ctx, teamID)
}

func (s selStore) DeleteAdminTeam(ctx context.Context, teamID int) (err error) {
	if err = s.fail["DeleteAdminTeam"]; err != nil {
		return
	}
	return s.inner.DeleteAdminTeam(ctx, teamID)
}

func (s selStore) SetTeamActive(ctx context.Context, teamID int, active bool, now time.Time) (r0 adminTeam, err error) {
	if err = s.fail["SetTeamActive"]; err != nil {
		return
	}
	return s.inner.SetTeamActive(ctx, teamID, active, now)
}

func (s selStore) RequeueTeamServices(ctx context.Context, teamID int) (err error) {
	if err = s.fail["RequeueTeamServices"]; err != nil {
		return
	}
	return s.inner.RequeueTeamServices(ctx, teamID)
}

func (s selStore) SetChallengeMaintenance(ctx context.Context, challengeID int, maintenance bool, now time.Time) (r0 adminChallenge, err error) {
	if err = s.fail["SetChallengeMaintenance"]; err != nil {
		return
	}
	return s.inner.SetChallengeMaintenance(ctx, challengeID, maintenance, now)
}

func (s selStore) ChallengePlayBlocked(ctx context.Context, challengeID int) (r0 bool, err error) {
	if err = s.fail["ChallengePlayBlocked"]; err != nil {
		return
	}
	return s.inner.ChallengePlayBlocked(ctx, challengeID)
}

func (s selStore) RequeueChallengeServices(ctx context.Context, challengeID int) (err error) {
	if err = s.fail["RequeueChallengeServices"]; err != nil {
		return
	}
	return s.inner.RequeueChallengeServices(ctx, challengeID)
}

func (s selStore) ListAdminPlayers(ctx context.Context) (r0 []adminPlayer, err error) {
	if err = s.fail["ListAdminPlayers"]; err != nil {
		return
	}
	return s.inner.ListAdminPlayers(ctx)
}

func (s selStore) CreateAdminPlayer(ctx context.Context, input adminCreatePlayerRequest, now time.Time) (r0 adminPlayer, err error) {
	if err = s.fail["CreateAdminPlayer"]; err != nil {
		return
	}
	return s.inner.CreateAdminPlayer(ctx, input, now)
}

func (s selStore) UpdateAdminPlayer(ctx context.Context, playerID int, input adminUpdatePlayerRequest) (r0 adminPlayer, err error) {
	if err = s.fail["UpdateAdminPlayer"]; err != nil {
		return
	}
	return s.inner.UpdateAdminPlayer(ctx, playerID, input)
}

func (s selStore) DeleteAdminPlayer(ctx context.Context, playerID int) (err error) {
	if err = s.fail["DeleteAdminPlayer"]; err != nil {
		return
	}
	return s.inner.DeleteAdminPlayer(ctx, playerID)
}

func (s selStore) SetPlayerActive(ctx context.Context, playerID int, active bool, now time.Time) (r0 adminPlayer, err error) {
	if err = s.fail["SetPlayerActive"]; err != nil {
		return
	}
	return s.inner.SetPlayerActive(ctx, playerID, active, now)
}

func (s selStore) GetAdminPlayerWireGuardConfig(ctx context.Context, playerID int) (r0 adminWireGuardPeer, err error) {
	if err = s.fail["GetAdminPlayerWireGuardConfig"]; err != nil {
		return
	}
	return s.inner.GetAdminPlayerWireGuardConfig(ctx, playerID)
}

func (s selStore) RotateAdminPlayerWireGuardConfig(ctx context.Context, playerID int, now time.Time) (r0 adminWireGuardPeer, err error) {
	if err = s.fail["RotateAdminPlayerWireGuardConfig"]; err != nil {
		return
	}
	return s.inner.RotateAdminPlayerWireGuardConfig(ctx, playerID, now)
}

func (s selStore) RevokeAdminPlayerWireGuardConfig(ctx context.Context, playerID int, now time.Time) (r0 adminWireGuardPeer, err error) {
	if err = s.fail["RevokeAdminPlayerWireGuardConfig"]; err != nil {
		return
	}
	return s.inner.RevokeAdminPlayerWireGuardConfig(ctx, playerID, now)
}

func (s selStore) ListWireGuardGatewayPeers(ctx context.Context) (r0 []WireGuardGatewayPeer, err error) {
	if err = s.fail["ListWireGuardGatewayPeers"]; err != nil {
		return
	}
	return s.inner.ListWireGuardGatewayPeers(ctx)
}

func (s selStore) IsMatchPaused(ctx context.Context) (r0 bool, err error) {
	if err = s.fail["IsMatchPaused"]; err != nil {
		return
	}
	return s.inner.IsMatchPaused(ctx)
}

func (s selStore) IsMatchStarted(ctx context.Context) (r0 bool, err error) {
	if err = s.fail["IsMatchStarted"]; err != nil {
		return
	}
	return s.inner.IsMatchStarted(ctx)
}

func (s selStore) ListAdminChallenges(ctx context.Context) (r0 []adminChallenge, err error) {
	if err = s.fail["ListAdminChallenges"]; err != nil {
		return
	}
	return s.inner.ListAdminChallenges(ctx)
}

func (s selStore) CreateAdminChallenge(ctx context.Context, input adminCreateChallengeRequest, now time.Time) (r0 adminChallenge, err error) {
	if err = s.fail["CreateAdminChallenge"]; err != nil {
		return
	}
	return s.inner.CreateAdminChallenge(ctx, input, now)
}

func (s selStore) UpdateAdminChallenge(ctx context.Context, challengeID int, input adminUpdateChallengeRequest) (r0 adminChallenge, err error) {
	if err = s.fail["UpdateAdminChallenge"]; err != nil {
		return
	}
	return s.inner.UpdateAdminChallenge(ctx, challengeID, input)
}

func (s selStore) DeleteAdminChallenge(ctx context.Context, challengeID int) (err error) {
	if err = s.fail["DeleteAdminChallenge"]; err != nil {
		return
	}
	return s.inner.DeleteAdminChallenge(ctx, challengeID)
}

func (s selStore) DeployAdminChallenge(ctx context.Context, challengeID int) (r0 adminDeployment, err error) {
	if err = s.fail["DeployAdminChallenge"]; err != nil {
		return
	}
	return s.inner.DeployAdminChallenge(ctx, challengeID)
}

func (s selStore) SaveAdminChallengeValidation(ctx context.Context, result ChallengeValidationResult) (err error) {
	if err = s.fail["SaveAdminChallengeValidation"]; err != nil {
		return
	}
	return s.inner.SaveAdminChallengeValidation(ctx, result)
}

func (s selStore) ListAdminDeployments(ctx context.Context) (r0 []adminDeploymentJob, err error) {
	if err = s.fail["ListAdminDeployments"]; err != nil {
		return
	}
	return s.inner.ListAdminDeployments(ctx)
}

func (s selStore) DeleteAdminDeployment(ctx context.Context, deploymentID int) (err error) {
	if err = s.fail["DeleteAdminDeployment"]; err != nil {
		return
	}
	return s.inner.DeleteAdminDeployment(ctx, deploymentID)
}

func (s selStore) JoinExistingPlayerTeam(ctx context.Context, playerID int, teamKey string, now time.Time) (r0 authenticatedPlayer, err error) {
	if err = s.fail["JoinExistingPlayerTeam"]; err != nil {
		return
	}
	return s.inner.JoinExistingPlayerTeam(ctx, playerID, teamKey, now)
}

func (s selStore) ListTeamMembers(ctx context.Context, teamID int) (r0 []teamMember, err error) {
	if err = s.fail["ListTeamMembers"]; err != nil {
		return
	}
	return s.inner.ListTeamMembers(ctx, teamID)
}

func (s selStore) TransferTeamCaptain(ctx context.Context, captainPlayerID, targetPlayerID int) (r0 authenticatedPlayer, err error) {
	if err = s.fail["TransferTeamCaptain"]; err != nil {
		return
	}
	return s.inner.TransferTeamCaptain(ctx, captainPlayerID, targetPlayerID)
}

func (s selStore) ListAdminAuditLogs(ctx context.Context, query adminAuditLogQuery) (r0 adminAuditLogPage, err error) {
	if err = s.fail["ListAdminAuditLogs"]; err != nil {
		return
	}
	return s.inner.ListAdminAuditLogs(ctx, query)
}

func (s selStore) AppendAdminAuditLog(ctx context.Context, entry adminAuditLogEntry) (err error) {
	if err = s.fail["AppendAdminAuditLog"]; err != nil {
		return
	}
	return s.inner.AppendAdminAuditLog(ctx, entry)
}

func (s selStore) ListControllerRuntimeTasks(ctx context.Context) (r0 []ControllerRuntimeTask, err error) {
	if err = s.fail["ListControllerRuntimeTasks"]; err != nil {
		return
	}
	return s.inner.ListControllerRuntimeTasks(ctx)
}

func (s selStore) ListControllerTeamStateVolumes(ctx context.Context, teamID int) ([]string, error) {
	if err := s.fail["ListControllerTeamStateVolumes"]; err != nil {
		return nil, err
	}
	return s.inner.ListControllerTeamStateVolumes(ctx, teamID)
}

func (s selStore) GetControllerRuntimeTask(ctx context.Context, teamID, challengeID int) (r0 ControllerRuntimeTask, err error) {
	if err = s.fail["GetControllerRuntimeTask"]; err != nil {
		return
	}
	return s.inner.GetControllerRuntimeTask(ctx, teamID, challengeID)
}

func (s selStore) ListControllerServiceAccessPolicies(ctx context.Context) (r0 []ControllerServiceAccessPolicy, err error) {
	if err = s.fail["ListControllerServiceAccessPolicies"]; err != nil {
		return
	}
	return s.inner.ListControllerServiceAccessPolicies(ctx)
}

func (s selStore) GetControllerServiceAccessPolicy(ctx context.Context, teamID, challengeID int) (r0 ControllerServiceAccessPolicy, err error) {
	if err = s.fail["GetControllerServiceAccessPolicy"]; err != nil {
		return
	}
	return s.inner.GetControllerServiceAccessPolicy(ctx, teamID, challengeID)
}

func (s selStore) ReconcileAdminDeployments(ctx context.Context, now time.Time) (r0 adminReconcileResult, err error) {
	if err = s.fail["ReconcileAdminDeployments"]; err != nil {
		return
	}
	return s.inner.ReconcileAdminDeployments(ctx, now)
}

func (s selStore) GetPlatformSettings(ctx context.Context) (r0 adminPlatformSettings, err error) {
	if err = s.fail["GetPlatformSettings"]; err != nil {
		return
	}
	return s.inner.GetPlatformSettings(ctx)
}

func (s selStore) UpdatePlatformSettings(ctx context.Context, input adminUpdatePlatformSettingsRequest, actor string, now time.Time) (r0 adminPlatformSettings, err error) {
	if err = s.fail["UpdatePlatformSettings"]; err != nil {
		return
	}
	return s.inner.UpdatePlatformSettings(ctx, input, actor, now)
}

func (s selStore) SetActiveFlagFormat(ctx context.Context, format string, now time.Time) (r0 adminPlatformSettings, err error) {
	if err = s.fail["SetActiveFlagFormat"]; err != nil {
		return
	}
	return s.inner.SetActiveFlagFormat(ctx, format, now)
}

func (s selStore) GetPlatformAppearance(ctx context.Context) (r0 platformAppearance, err error) {
	if err = s.fail["GetPlatformAppearance"]; err != nil {
		return
	}
	return s.inner.GetPlatformAppearance(ctx)
}

func (s selStore) SetPlatformTheme(ctx context.Context, themeID string, actor string, now time.Time) (r0 platformAppearance, err error) {
	if err = s.fail["SetPlatformTheme"]; err != nil {
		return
	}
	return s.inner.SetPlatformTheme(ctx, themeID, actor, now)
}

func (s selStore) Close() (err error) {
	if err = s.fail["Close"]; err != nil {
		return
	}
	return s.inner.Close()
}

type selController struct {
	inner controllerClient
	fail  failSet
}

var _ controllerClient = selController{}

func (s selController) ReconcileDeployments(ctx context.Context) (r0 adminReconcileResult, err error) {
	if err = s.fail["ReconcileDeployments"]; err != nil {
		return
	}
	return s.inner.ReconcileDeployments(ctx)
}

func (s selController) FactoryResetService(ctx context.Context, teamID, challengeID int) (err error) {
	if err = s.fail["FactoryResetService"]; err != nil {
		return
	}
	return s.inner.FactoryResetService(ctx, teamID, challengeID)
}

func (s selController) RestartService(ctx context.Context, teamID, challengeID int) (err error) {
	if err = s.fail["RestartService"]; err != nil {
		return
	}
	return s.inner.RestartService(ctx, teamID, challengeID)
}

func (s selController) ReconcileServiceAccess(ctx context.Context, teamID, challengeID int) (err error) {
	if err = s.fail["ReconcileServiceAccess"]; err != nil {
		return
	}
	return s.inner.ReconcileServiceAccess(ctx, teamID, challengeID)
}

func (s selController) ApplySSHCredential(ctx context.Context, teamID, challengeID int, credential ControllerSSHCredential) (err error) {
	if err = s.fail["ApplySSHCredential"]; err != nil {
		return
	}
	return s.inner.ApplySSHCredential(ctx, teamID, challengeID, credential)
}

func (s selController) ValidateChallengeRuntime(ctx context.Context, request ChallengeValidationRequest) (r0 ChallengeValidationResult, err error) {
	if err = s.fail["ValidateChallengeRuntime"]; err != nil {
		return
	}
	return s.inner.ValidateChallengeRuntime(ctx, request)
}

func (s selController) AccessStatus(ctx context.Context) (r0 ControllerAccessStatus, err error) {
	if err = s.fail["AccessStatus"]; err != nil {
		return
	}
	return s.inner.AccessStatus(ctx)
}

func (s selController) ReconcileAccessPolicies(ctx context.Context) (r0 ControllerAccessStatus, err error) {
	if err = s.fail["ReconcileAccessPolicies"]; err != nil {
		return
	}
	return s.inner.ReconcileAccessPolicies(ctx)
}

func (s selController) TeardownAccessPolicies(ctx context.Context) (err error) {
	if err = s.fail["TeardownAccessPolicies"]; err != nil {
		return
	}
	return s.inner.TeardownAccessPolicies(ctx)
}

func (s selController) RemoveService(ctx context.Context, teamID, challengeID int) (err error) {
	if err = s.fail["RemoveService"]; err != nil {
		return
	}
	return s.inner.RemoveService(ctx, teamID, challengeID)
}

func (s selController) RemoveTeamServices(ctx context.Context, teamID int) (err error) {
	if err = s.fail["RemoveTeamServices"]; err != nil {
		return
	}
	return s.inner.RemoveTeamServices(ctx, teamID)
}

func (s selController) RemoveChallengeServices(ctx context.Context, challengeID int) (err error) {
	if err = s.fail["RemoveChallengeServices"]; err != nil {
		return
	}
	return s.inner.RemoveChallengeServices(ctx, challengeID)
}

type selWireGuard struct {
	inner wireGuardClient
	fail  failSet
}

var _ wireGuardClient = selWireGuard{}

func (s selWireGuard) Status(ctx context.Context) (r0 WireGuardGatewayStatus, err error) {
	if err = s.fail["Status"]; err != nil {
		return
	}
	return s.inner.Status(ctx)
}

func (s selWireGuard) Reconcile(ctx context.Context) (r0 WireGuardGatewayStatus, err error) {
	if err = s.fail["Reconcile"]; err != nil {
		return
	}
	return s.inner.Reconcile(ctx)
}

func (s selWireGuard) Teardown(ctx context.Context) (err error) {
	if err = s.fail["Teardown"]; err != nil {
		return
	}
	return s.inner.Teardown(ctx)
}

type selGameCore struct {
	inner gameCoreClient
	fail  failSet
}

var _ gameCoreClient = selGameCore{}

func (s selGameCore) Submissions(a0 context.Context, a1 SubmissionQuery) (r0 SubmissionPage, err error) {
	if err = s.fail["Submissions"]; err != nil {
		return
	}
	return s.inner.Submissions(a0, a1)
}

func (s selGameCore) DeleteSubmission(a0 context.Context, a1 int64) (r0 SubmissionDeletion, err error) {
	if err = s.fail["DeleteSubmission"]; err != nil {
		return
	}
	return s.inner.DeleteSubmission(a0, a1)
}

func (s selGameCore) Status(ctx context.Context) (r0 GameStatus, err error) {
	if err = s.fail["Status"]; err != nil {
		return
	}
	return s.inner.Status(ctx)
}

func (s selGameCore) MatchStatus(ctx context.Context) (r0 GameMatchStatus, err error) {
	if err = s.fail["MatchStatus"]; err != nil {
		return
	}
	return s.inner.MatchStatus(ctx)
}

func (s selGameCore) StartMatch(ctx context.Context) (r0 GameMatchStatus, err error) {
	if err = s.fail["StartMatch"]; err != nil {
		return
	}
	return s.inner.StartMatch(ctx)
}

func (s selGameCore) PauseMatch(ctx context.Context) (r0 GameMatchStatus, err error) {
	if err = s.fail["PauseMatch"]; err != nil {
		return
	}
	return s.inner.PauseMatch(ctx)
}

func (s selGameCore) ResumeMatch(ctx context.Context) (r0 GameMatchStatus, err error) {
	if err = s.fail["ResumeMatch"]; err != nil {
		return
	}
	return s.inner.ResumeMatch(ctx)
}

func (s selGameCore) StopMatch(ctx context.Context) (r0 GameMatchStatus, err error) {
	if err = s.fail["StopMatch"]; err != nil {
		return
	}
	return s.inner.StopMatch(ctx)
}

func (s selGameCore) UpdateMatchSchedule(ctx context.Context, req UpdateMatchScheduleRequest) (r0 GameMatchStatus, err error) {
	if err = s.fail["UpdateMatchSchedule"]; err != nil {
		return
	}
	return s.inner.UpdateMatchSchedule(ctx, req)
}

func (s selGameCore) AdvanceTick(ctx context.Context) (r0 GameTickStatus, err error) {
	if err = s.fail["AdvanceTick"]; err != nil {
		return
	}
	return s.inner.AdvanceTick(ctx)
}

func (s selGameCore) CheckerRuns(ctx context.Context, query GameCheckerRunQuery) (r0 GameCheckerRunPage, err error) {
	if err = s.fail["CheckerRuns"]; err != nil {
		return
	}
	return s.inner.CheckerRuns(ctx, query)
}

func (s selGameCore) SchedulerStatus(ctx context.Context) (r0 GameSchedulerStatus, err error) {
	if err = s.fail["SchedulerStatus"]; err != nil {
		return
	}
	return s.inner.SchedulerStatus(ctx)
}

func (s selGameCore) SchedulerEvents(ctx context.Context, query GameSchedulerEventQuery) (r0 GameSchedulerEventPage, err error) {
	if err = s.fail["SchedulerEvents"]; err != nil {
		return
	}
	return s.inner.SchedulerEvents(ctx, query)
}

func (s selGameCore) StartScheduler(ctx context.Context) (r0 GameSchedulerStatus, err error) {
	if err = s.fail["StartScheduler"]; err != nil {
		return
	}
	return s.inner.StartScheduler(ctx)
}

func (s selGameCore) StopScheduler(ctx context.Context) (r0 GameSchedulerStatus, err error) {
	if err = s.fail["StopScheduler"]; err != nil {
		return
	}
	return s.inner.StopScheduler(ctx)
}

func (s selGameCore) UpdateScheduler(ctx context.Context, req UpdateSchedulerRequest) (r0 GameSchedulerStatus, err error) {
	if err = s.fail["UpdateScheduler"]; err != nil {
		return
	}
	return s.inner.UpdateScheduler(ctx, req)
}

func (s selGameCore) SubmitFlags(ctx context.Context, teamID int, flags []string) (r0 []submissionVerdict, err error) {
	if err = s.fail["SubmitFlags"]; err != nil {
		return
	}
	return s.inner.SubmitFlags(ctx, teamID, flags)
}

func (s selGameCore) Scoreboard(ctx context.Context) (r0 []scoreRow, err error) {
	if err = s.fail["Scoreboard"]; err != nil {
		return
	}
	return s.inner.Scoreboard(ctx)
}

func (s selGameCore) AttackFeed(ctx context.Context, query AttackFeedQuery) (r0 AttackFeedPage, err error) {
	if err = s.fail["AttackFeed"]; err != nil {
		return
	}
	return s.inner.AttackFeed(ctx, query)
}

func (s selGameCore) RecomputeScoring(ctx context.Context) (r0 []scoreRow, err error) {
	if err = s.fail["RecomputeScoring"]; err != nil {
		return
	}
	return s.inner.RecomputeScoring(ctx)
}

func (s selGameCore) AuditScoring(ctx context.Context) (r0 ScoringAuditAlias, err error) {
	if err = s.fail["AuditScoring"]; err != nil {
		return
	}
	return s.inner.AuditScoring(ctx)
}

func (s selGameCore) FlagFormat(ctx context.Context) (r0 FlagFormatStatus, err error) {
	if err = s.fail["FlagFormat"]; err != nil {
		return
	}
	return s.inner.FlagFormat(ctx)
}

func (s selGameCore) RefreshFlagFormat(ctx context.Context, prefix string) (r0 FlagFormatStatus, err error) {
	if err = s.fail["RefreshFlagFormat"]; err != nil {
		return
	}
	return s.inner.RefreshFlagFormat(ctx, prefix)
}

type selScoring struct {
	inner scoringClient
	fail  failSet
}

var _ scoringClient = selScoring{}

func (s selScoring) Scoreboard(ctx context.Context) (r0 []scoreRow, err error) {
	if err = s.fail["Scoreboard"]; err != nil {
		return
	}
	return s.inner.Scoreboard(ctx)
}

func (s selScoring) RecomputeScoring(ctx context.Context) (r0 []scoreRow, err error) {
	if err = s.fail["RecomputeScoring"]; err != nil {
		return
	}
	return s.inner.RecomputeScoring(ctx)
}

func (s selScoring) AuditScoring(ctx context.Context) (r0 ScoringAuditAlias, err error) {
	if err = s.fail["AuditScoring"]; err != nil {
		return
	}
	return s.inner.AuditScoring(ctx)
}
