package apigateway

// Code generated for tests from the Store interface: delegates to inner,
// failing the call its counter is armed for.

import (
	"context"
	"sync"
	"time"
)

type storeFaults struct {
	mu     sync.Mutex
	count  int
	failAt int
}

func (f *storeFaults) arm(n int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.count, f.failAt = 0, n
}

func (f *storeFaults) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.count
}

func (f *storeFaults) fail() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.count++
	return f.failAt > 0 && f.count == f.failAt
}

type faultingStore struct {
	inner  Store
	faults *storeFaults
}

var _ Store = faultingStore{}

func (s faultingStore) AuthenticatePlayer(ctx context.Context, email, password string) (r0 authenticatedPlayer, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.AuthenticatePlayer(ctx, email, password)
}

func (s faultingStore) RegisterPlayer(ctx context.Context, input participantRegisterRequest, now time.Time) (r0 authenticatedPlayer, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.RegisterPlayer(ctx, input, now)
}

func (s faultingStore) ValidatePlayerSession(ctx context.Context, playerID, teamID int, role string, sessionVersion int) (r0 authenticatedPlayer, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.ValidatePlayerSession(ctx, playerID, teamID, role, sessionVersion)
}

func (s faultingStore) UpdateParticipantProfile(ctx context.Context, playerID int, input participantUpdateProfileRequest) (r0 authenticatedPlayer, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.UpdateParticipantProfile(ctx, playerID, input)
}

func (s faultingStore) ChangeParticipantPassword(ctx context.Context, playerID int, currentPassword, newPassword string) (r0 int, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.ChangeParticipantPassword(ctx, playerID, currentPassword, newPassword)
}

func (s faultingStore) ListAnnouncements(ctx context.Context) (r0 []matchAnnouncement, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.ListAnnouncements(ctx)
}

func (s faultingStore) CreateAnnouncement(ctx context.Context, body, createdBy string, now time.Time) (r0 matchAnnouncement, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.CreateAnnouncement(ctx, body, createdBy, now)
}

func (s faultingStore) DeleteAnnouncement(ctx context.Context, id int) (err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.DeleteAnnouncement(ctx, id)
}

func (s faultingStore) ImportAdminTeams(ctx context.Context, teams []adminBulkImportTeam, now time.Time) (r0 adminBulkImportResult, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.ImportAdminTeams(ctx, teams, now)
}

func (s faultingStore) ListChallenges(ctx context.Context) (r0 []challenge, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.ListChallenges(ctx)
}

func (s faultingStore) ListPublicServices(ctx context.Context) (r0 map[string]map[string][]string, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.ListPublicServices(ctx)
}

func (s faultingStore) ListScoreboard(ctx context.Context) (r0 []scoreRow, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.ListScoreboard(ctx)
}

func (s faultingStore) GetScoreboardFreeze(ctx context.Context) (r0 scoreboardFreezeWindow, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.GetScoreboardFreeze(ctx)
}

func (s faultingStore) SetScoreboardFreezeWindow(ctx context.Context, freezeAt, unfreezeAt *time.Time, now time.Time) (r0 scoreboardFreezeWindow, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.SetScoreboardFreezeWindow(ctx, freezeAt, unfreezeAt, now)
}

func (s faultingStore) ClearScoreboardFreeze(ctx context.Context, now time.Time) (r0 scoreboardFreezeWindow, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.ClearScoreboardFreeze(ctx, now)
}

func (s faultingStore) SaveFrozenScoreboardSnapshot(ctx context.Context, rows []scoreRow, takenAt time.Time) (r0 scoreboardFreezeWindow, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.SaveFrozenScoreboardSnapshot(ctx, rows, takenAt)
}

func (s faultingStore) ListAttackFeed(ctx context.Context) (r0 []attackEvent, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.ListAttackFeed(ctx)
}

func (s faultingStore) ListTeamServices(ctx context.Context, teamID int) (r0 []serviceState, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.ListTeamServices(ctx, teamID)
}

func (s faultingStore) SubmitFlags(ctx context.Context, teamID int, flags []string) (r0 []submissionVerdict, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.SubmitFlags(ctx, teamID, flags)
}

func (s faultingStore) ValidateServiceAction(ctx context.Context, teamID, challengeID int) (err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.ValidateServiceAction(ctx, teamID, challengeID)
}

func (s faultingStore) UnlockService(ctx context.Context, teamID, challengeID int) (r0 unlockData, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.UnlockService(ctx, teamID, challengeID)
}

func (s faultingStore) CreateSSHSession(ctx context.Context, teamID, challengeID int, now time.Time) (r0 sshSessionData, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.CreateSSHSession(ctx, teamID, challengeID, now)
}

func (s faultingStore) MarkSSHSessionApplyFailure(ctx context.Context, teamID, challengeID int) (err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.MarkSSHSessionApplyFailure(ctx, teamID, challengeID)
}

func (s faultingStore) LockServiceLifecycle(ctx context.Context, teamID, challengeID int) (r0 func(), err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.LockServiceLifecycle(ctx, teamID, challengeID)
}

func (s faultingStore) PrepareFactoryResetService(ctx context.Context, teamID, challengeID int) (r0 resetData, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.PrepareFactoryResetService(ctx, teamID, challengeID)
}

func (s faultingStore) AbortFactoryResetService(ctx context.Context, prepared resetData) (err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.AbortFactoryResetService(ctx, prepared)
}

func (s faultingStore) CompleteFactoryResetService(ctx context.Context, teamID, challengeID int) (r0 resetData, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.CompleteFactoryResetService(ctx, teamID, challengeID)
}

func (s faultingStore) MarkFactoryResetFailure(ctx context.Context, teamID, challengeID int) (err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.MarkFactoryResetFailure(ctx, teamID, challengeID)
}

func (s faultingStore) MarkFactoryResetCompletionFailure(ctx context.Context, teamID, challengeID int) (err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.MarkFactoryResetCompletionFailure(ctx, teamID, challengeID)
}

func (s faultingStore) ValidateRestartService(ctx context.Context, teamID, challengeID int) (err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.ValidateRestartService(ctx, teamID, challengeID)
}

func (s faultingStore) RestartService(ctx context.Context, teamID, challengeID int) (r0 resetData, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.RestartService(ctx, teamID, challengeID)
}

func (s faultingStore) ListAdminTeams(ctx context.Context) (r0 []adminTeam, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.ListAdminTeams(ctx)
}

func (s faultingStore) CreateAdminTeam(ctx context.Context, input adminCreateTeamRequest) (r0 adminTeam, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.CreateAdminTeam(ctx, input)
}

func (s faultingStore) UpdateAdminTeam(ctx context.Context, teamID int, input adminUpdateTeamRequest) (r0 adminTeam, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.UpdateAdminTeam(ctx, teamID, input)
}

func (s faultingStore) DeleteAdminTeam(ctx context.Context, teamID int) (err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.DeleteAdminTeam(ctx, teamID)
}

func (s faultingStore) SetTeamActive(ctx context.Context, teamID int, active bool, now time.Time) (r0 adminTeam, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.SetTeamActive(ctx, teamID, active, now)
}

func (s faultingStore) RequeueTeamServices(ctx context.Context, teamID int) (err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.RequeueTeamServices(ctx, teamID)
}

func (s faultingStore) SetChallengeMaintenance(ctx context.Context, challengeID int, maintenance bool, now time.Time) (r0 adminChallenge, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.SetChallengeMaintenance(ctx, challengeID, maintenance, now)
}

func (s faultingStore) ChallengePlayBlocked(ctx context.Context, challengeID int) (r0 bool, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.ChallengePlayBlocked(ctx, challengeID)
}

func (s faultingStore) RequeueChallengeServices(ctx context.Context, challengeID int) (err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.RequeueChallengeServices(ctx, challengeID)
}

func (s faultingStore) ListAdminPlayers(ctx context.Context) (r0 []adminPlayer, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.ListAdminPlayers(ctx)
}

func (s faultingStore) CreateAdminPlayer(ctx context.Context, input adminCreatePlayerRequest, now time.Time) (r0 adminPlayer, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.CreateAdminPlayer(ctx, input, now)
}

func (s faultingStore) UpdateAdminPlayer(ctx context.Context, playerID int, input adminUpdatePlayerRequest) (r0 adminPlayer, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.UpdateAdminPlayer(ctx, playerID, input)
}

func (s faultingStore) DeleteAdminPlayer(ctx context.Context, playerID int) (err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.DeleteAdminPlayer(ctx, playerID)
}

func (s faultingStore) SetPlayerActive(ctx context.Context, playerID int, active bool, now time.Time) (r0 adminPlayer, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.SetPlayerActive(ctx, playerID, active, now)
}

func (s faultingStore) GetAdminPlayerWireGuardConfig(ctx context.Context, playerID int) (r0 adminWireGuardPeer, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.GetAdminPlayerWireGuardConfig(ctx, playerID)
}

func (s faultingStore) RotateAdminPlayerWireGuardConfig(ctx context.Context, playerID int, now time.Time) (r0 adminWireGuardPeer, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.RotateAdminPlayerWireGuardConfig(ctx, playerID, now)
}

func (s faultingStore) RevokeAdminPlayerWireGuardConfig(ctx context.Context, playerID int, now time.Time) (r0 adminWireGuardPeer, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.RevokeAdminPlayerWireGuardConfig(ctx, playerID, now)
}

func (s faultingStore) ListWireGuardGatewayPeers(ctx context.Context) (r0 []WireGuardGatewayPeer, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.ListWireGuardGatewayPeers(ctx)
}

func (s faultingStore) IsMatchPaused(ctx context.Context) (r0 bool, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.IsMatchPaused(ctx)
}

func (s faultingStore) IsMatchStarted(ctx context.Context) (r0 bool, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.IsMatchStarted(ctx)
}

func (s faultingStore) ListAdminChallenges(ctx context.Context) (r0 []adminChallenge, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.ListAdminChallenges(ctx)
}

func (s faultingStore) CreateAdminChallenge(ctx context.Context, input adminCreateChallengeRequest, now time.Time) (r0 adminChallenge, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.CreateAdminChallenge(ctx, input, now)
}

func (s faultingStore) UpdateAdminChallenge(ctx context.Context, challengeID int, input adminUpdateChallengeRequest) (r0 adminChallenge, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.UpdateAdminChallenge(ctx, challengeID, input)
}

func (s faultingStore) DeleteAdminChallenge(ctx context.Context, challengeID int) (err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.DeleteAdminChallenge(ctx, challengeID)
}

func (s faultingStore) DeployAdminChallenge(ctx context.Context, challengeID int) (r0 adminDeployment, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.DeployAdminChallenge(ctx, challengeID)
}

func (s faultingStore) SaveAdminChallengeValidation(ctx context.Context, result ChallengeValidationResult) (err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.SaveAdminChallengeValidation(ctx, result)
}

func (s faultingStore) ListAdminDeployments(ctx context.Context) (r0 []adminDeploymentJob, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.ListAdminDeployments(ctx)
}

func (s faultingStore) DeleteAdminDeployment(ctx context.Context, deploymentID int) (err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.DeleteAdminDeployment(ctx, deploymentID)
}

func (s faultingStore) JoinExistingPlayerTeam(ctx context.Context, playerID int, teamKey string, now time.Time) (r0 authenticatedPlayer, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.JoinExistingPlayerTeam(ctx, playerID, teamKey, now)
}

func (s faultingStore) ListTeamMembers(ctx context.Context, teamID int) (r0 []teamMember, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.ListTeamMembers(ctx, teamID)
}

func (s faultingStore) TransferTeamCaptain(ctx context.Context, captainPlayerID, targetPlayerID int) (r0 authenticatedPlayer, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.TransferTeamCaptain(ctx, captainPlayerID, targetPlayerID)
}

func (s faultingStore) ListAdminAuditLogs(ctx context.Context, query adminAuditLogQuery) (r0 adminAuditLogPage, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.ListAdminAuditLogs(ctx, query)
}

func (s faultingStore) AppendAdminAuditLog(ctx context.Context, entry adminAuditLogEntry) (err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.AppendAdminAuditLog(ctx, entry)
}

func (s faultingStore) ListControllerRuntimeTasks(ctx context.Context) (r0 []ControllerRuntimeTask, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.ListControllerRuntimeTasks(ctx)
}

func (s faultingStore) GetControllerRuntimeTask(ctx context.Context, teamID, challengeID int) (r0 ControllerRuntimeTask, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.GetControllerRuntimeTask(ctx, teamID, challengeID)
}

func (s faultingStore) ListControllerServiceAccessPolicies(ctx context.Context) (r0 []ControllerServiceAccessPolicy, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.ListControllerServiceAccessPolicies(ctx)
}

func (s faultingStore) GetControllerServiceAccessPolicy(ctx context.Context, teamID, challengeID int) (r0 ControllerServiceAccessPolicy, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.GetControllerServiceAccessPolicy(ctx, teamID, challengeID)
}

func (s faultingStore) ReconcileAdminDeployments(ctx context.Context, now time.Time) (r0 adminReconcileResult, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.ReconcileAdminDeployments(ctx, now)
}

func (s faultingStore) GetPlatformSettings(ctx context.Context) (r0 adminPlatformSettings, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.GetPlatformSettings(ctx)
}

func (s faultingStore) UpdatePlatformSettings(ctx context.Context, input adminUpdatePlatformSettingsRequest, actor string, now time.Time) (r0 adminPlatformSettings, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.UpdatePlatformSettings(ctx, input, actor, now)
}

func (s faultingStore) SetActiveFlagFormat(ctx context.Context, format string, now time.Time) (r0 adminPlatformSettings, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.SetActiveFlagFormat(ctx, format, now)
}

func (s faultingStore) GetPlatformAppearance(ctx context.Context) (r0 platformAppearance, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.GetPlatformAppearance(ctx)
}

func (s faultingStore) SetPlatformTheme(ctx context.Context, themeID string, actor string, now time.Time) (r0 platformAppearance, err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.SetPlatformTheme(ctx, themeID, actor, now)
}

func (s faultingStore) Close() (err error) {
	if s.faults.fail() {
		err = errStoreDown
		return
	}
	return s.inner.Close()
}
