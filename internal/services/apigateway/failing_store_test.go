package apigateway

// Code generated for tests from the Store interface; every method fails.

import (
	"context"
	"errors"
	"time"
)

var errStoreDown = errors.New("simulated store outage")

// failingStore implements Store with every method returning errStoreDown, so
// handler tests can drive each store-failure branch without a database.
type failingStore struct{}

var _ Store = failingStore{}

func (failingStore) AuthenticatePlayer(ctx context.Context, email, password string) (r0 authenticatedPlayer, err error) {
	err = errStoreDown
	return
}

func (failingStore) RegisterPlayer(ctx context.Context, input participantRegisterRequest, now time.Time) (r0 authenticatedPlayer, err error) {
	err = errStoreDown
	return
}

func (failingStore) ValidatePlayerSession(ctx context.Context, playerID, teamID int, role string, sessionVersion int) (r0 authenticatedPlayer, err error) {
	err = errStoreDown
	return
}

func (failingStore) UpdateParticipantProfile(ctx context.Context, playerID int, input participantUpdateProfileRequest) (r0 authenticatedPlayer, err error) {
	err = errStoreDown
	return
}

func (failingStore) ChangeParticipantPassword(ctx context.Context, playerID int, currentPassword, newPassword string) (r0 int, err error) {
	err = errStoreDown
	return
}

func (failingStore) ListAnnouncements(ctx context.Context) (r0 []matchAnnouncement, err error) {
	err = errStoreDown
	return
}

func (failingStore) CreateAnnouncement(ctx context.Context, body, createdBy string, now time.Time) (r0 matchAnnouncement, err error) {
	err = errStoreDown
	return
}

func (failingStore) DeleteAnnouncement(ctx context.Context, id int) (err error) {
	err = errStoreDown
	return
}

func (failingStore) ImportAdminTeams(ctx context.Context, teams []adminBulkImportTeam, now time.Time) (r0 adminBulkImportResult, err error) {
	err = errStoreDown
	return
}

func (failingStore) ListChallenges(ctx context.Context) (r0 []challenge, err error) {
	err = errStoreDown
	return
}

func (failingStore) ListPublicServices(ctx context.Context) (r0 map[string]map[string][]string, err error) {
	err = errStoreDown
	return
}

func (failingStore) ListScoreboard(ctx context.Context) (r0 []scoreRow, err error) {
	err = errStoreDown
	return
}

func (failingStore) GetScoreboardFreeze(ctx context.Context) (r0 scoreboardFreezeWindow, err error) {
	err = errStoreDown
	return
}

func (failingStore) SetScoreboardFreezeWindow(ctx context.Context, freezeAt, unfreezeAt *time.Time, now time.Time) (r0 scoreboardFreezeWindow, err error) {
	err = errStoreDown
	return
}

func (failingStore) ClearScoreboardFreeze(ctx context.Context, now time.Time) (r0 scoreboardFreezeWindow, err error) {
	err = errStoreDown
	return
}

func (failingStore) SaveFrozenScoreboardSnapshot(ctx context.Context, rows []scoreRow, takenAt time.Time) (r0 scoreboardFreezeWindow, err error) {
	err = errStoreDown
	return
}

func (failingStore) ListAttackFeed(ctx context.Context) (r0 []attackEvent, err error) {
	err = errStoreDown
	return
}

func (failingStore) ListTeamServices(ctx context.Context, teamID int) (r0 []serviceState, err error) {
	err = errStoreDown
	return
}

func (failingStore) SubmitFlags(ctx context.Context, teamID int, flags []string) (r0 []submissionVerdict, err error) {
	err = errStoreDown
	return
}

func (failingStore) ValidateServiceAction(ctx context.Context, teamID, challengeID int) (err error) {
	err = errStoreDown
	return
}

func (failingStore) UnlockService(ctx context.Context, teamID, challengeID int) (r0 unlockData, err error) {
	err = errStoreDown
	return
}

func (failingStore) CreateSSHSession(ctx context.Context, teamID, challengeID int, now time.Time) (r0 sshSessionData, err error) {
	err = errStoreDown
	return
}

func (failingStore) MarkSSHSessionApplyFailure(ctx context.Context, teamID, challengeID int) (err error) {
	err = errStoreDown
	return
}

func (failingStore) LockServiceLifecycle(ctx context.Context, teamID, challengeID int) (r0 func(), err error) {
	err = errStoreDown
	return
}

func (failingStore) PrepareFactoryResetService(ctx context.Context, teamID, challengeID int) (r0 resetData, err error) {
	err = errStoreDown
	return
}

func (failingStore) AbortFactoryResetService(ctx context.Context, prepared resetData) (err error) {
	err = errStoreDown
	return
}

func (failingStore) CompleteFactoryResetService(ctx context.Context, teamID, challengeID int) (r0 resetData, err error) {
	err = errStoreDown
	return
}

func (failingStore) MarkFactoryResetFailure(ctx context.Context, teamID, challengeID int) (err error) {
	err = errStoreDown
	return
}

func (failingStore) MarkFactoryResetCompletionFailure(ctx context.Context, teamID, challengeID int) (err error) {
	err = errStoreDown
	return
}

func (failingStore) ValidateRestartService(ctx context.Context, teamID, challengeID int) (err error) {
	err = errStoreDown
	return
}

func (failingStore) RestartService(ctx context.Context, teamID, challengeID int) (r0 resetData, err error) {
	err = errStoreDown
	return
}

func (failingStore) ListAdminTeams(ctx context.Context) (r0 []adminTeam, err error) {
	err = errStoreDown
	return
}

func (failingStore) CreateAdminTeam(ctx context.Context, input adminCreateTeamRequest) (r0 adminTeam, err error) {
	err = errStoreDown
	return
}

func (failingStore) UpdateAdminTeam(ctx context.Context, teamID int, input adminUpdateTeamRequest) (r0 adminTeam, err error) {
	err = errStoreDown
	return
}

func (failingStore) DeleteAdminTeam(ctx context.Context, teamID int) (err error) {
	err = errStoreDown
	return
}

func (failingStore) SetTeamActive(ctx context.Context, teamID int, active bool, now time.Time) (r0 adminTeam, err error) {
	err = errStoreDown
	return
}

func (failingStore) RequeueTeamServices(ctx context.Context, teamID int) (err error) {
	err = errStoreDown
	return
}

func (failingStore) SetChallengeMaintenance(ctx context.Context, challengeID int, maintenance bool, now time.Time) (r0 adminChallenge, err error) {
	err = errStoreDown
	return
}

func (failingStore) ChallengePlayBlocked(ctx context.Context, challengeID int) (r0 bool, err error) {
	err = errStoreDown
	return
}

func (failingStore) RequeueChallengeServices(ctx context.Context, challengeID int) (err error) {
	err = errStoreDown
	return
}

func (failingStore) ListAdminPlayers(ctx context.Context) (r0 []adminPlayer, err error) {
	err = errStoreDown
	return
}

func (failingStore) CreateAdminPlayer(ctx context.Context, input adminCreatePlayerRequest, now time.Time) (r0 adminPlayer, err error) {
	err = errStoreDown
	return
}

func (failingStore) UpdateAdminPlayer(ctx context.Context, playerID int, input adminUpdatePlayerRequest) (r0 adminPlayer, err error) {
	err = errStoreDown
	return
}

func (failingStore) DeleteAdminPlayer(ctx context.Context, playerID int) (err error) {
	err = errStoreDown
	return
}

func (failingStore) SetPlayerActive(ctx context.Context, playerID int, active bool, now time.Time) (r0 adminPlayer, err error) {
	err = errStoreDown
	return
}

func (failingStore) GetAdminPlayerWireGuardConfig(ctx context.Context, playerID int) (r0 adminWireGuardPeer, err error) {
	err = errStoreDown
	return
}

func (failingStore) RotateAdminPlayerWireGuardConfig(ctx context.Context, playerID int, now time.Time) (r0 adminWireGuardPeer, err error) {
	err = errStoreDown
	return
}

func (failingStore) RevokeAdminPlayerWireGuardConfig(ctx context.Context, playerID int, now time.Time) (r0 adminWireGuardPeer, err error) {
	err = errStoreDown
	return
}

func (failingStore) ListWireGuardGatewayPeers(ctx context.Context) (r0 []WireGuardGatewayPeer, err error) {
	err = errStoreDown
	return
}

func (failingStore) IsMatchPaused(ctx context.Context) (r0 bool, err error) {
	err = errStoreDown
	return
}

func (failingStore) IsMatchStarted(ctx context.Context) (r0 bool, err error) {
	err = errStoreDown
	return
}

func (failingStore) ListAdminChallenges(ctx context.Context) (r0 []adminChallenge, err error) {
	err = errStoreDown
	return
}

func (failingStore) CreateAdminChallenge(ctx context.Context, input adminCreateChallengeRequest, now time.Time) (r0 adminChallenge, err error) {
	err = errStoreDown
	return
}

func (failingStore) UpdateAdminChallenge(ctx context.Context, challengeID int, input adminUpdateChallengeRequest) (r0 adminChallenge, err error) {
	err = errStoreDown
	return
}

func (failingStore) DeleteAdminChallenge(ctx context.Context, challengeID int) (err error) {
	err = errStoreDown
	return
}

func (failingStore) DeployAdminChallenge(ctx context.Context, challengeID int) (r0 adminDeployment, err error) {
	err = errStoreDown
	return
}

func (failingStore) SaveAdminChallengeValidation(ctx context.Context, result ChallengeValidationResult) (err error) {
	err = errStoreDown
	return
}

func (failingStore) ListAdminDeployments(ctx context.Context) (r0 []adminDeploymentJob, err error) {
	err = errStoreDown
	return
}

func (failingStore) DeleteAdminDeployment(ctx context.Context, deploymentID int) (err error) {
	err = errStoreDown
	return
}

func (failingStore) JoinExistingPlayerTeam(ctx context.Context, playerID int, teamKey string, now time.Time) (r0 authenticatedPlayer, err error) {
	err = errStoreDown
	return
}

func (failingStore) ListTeamMembers(ctx context.Context, teamID int) (r0 []teamMember, err error) {
	err = errStoreDown
	return
}

func (failingStore) TransferTeamCaptain(ctx context.Context, captainPlayerID, targetPlayerID int) (r0 authenticatedPlayer, err error) {
	err = errStoreDown
	return
}

func (failingStore) ListAdminAuditLogs(ctx context.Context, query adminAuditLogQuery) (r0 adminAuditLogPage, err error) {
	err = errStoreDown
	return
}

func (failingStore) AppendAdminAuditLog(ctx context.Context, entry adminAuditLogEntry) (err error) {
	err = errStoreDown
	return
}

func (failingStore) ListControllerRuntimeTasks(ctx context.Context) (r0 []ControllerRuntimeTask, err error) {
	err = errStoreDown
	return
}

func (failingStore) GetControllerRuntimeTask(ctx context.Context, teamID, challengeID int) (r0 ControllerRuntimeTask, err error) {
	err = errStoreDown
	return
}

func (failingStore) ListControllerServiceAccessPolicies(ctx context.Context) (r0 []ControllerServiceAccessPolicy, err error) {
	err = errStoreDown
	return
}

func (failingStore) GetControllerServiceAccessPolicy(ctx context.Context, teamID, challengeID int) (r0 ControllerServiceAccessPolicy, err error) {
	err = errStoreDown
	return
}

func (failingStore) ReconcileAdminDeployments(ctx context.Context, now time.Time) (r0 adminReconcileResult, err error) {
	err = errStoreDown
	return
}

func (failingStore) GetPlatformSettings(ctx context.Context) (r0 adminPlatformSettings, err error) {
	err = errStoreDown
	return
}

func (failingStore) UpdatePlatformSettings(ctx context.Context, input adminUpdatePlatformSettingsRequest, actor string, now time.Time) (r0 adminPlatformSettings, err error) {
	err = errStoreDown
	return
}

func (failingStore) SetActiveFlagFormat(ctx context.Context, format string, now time.Time) (r0 adminPlatformSettings, err error) {
	err = errStoreDown
	return
}

func (failingStore) GetPlatformAppearance(ctx context.Context) (r0 platformAppearance, err error) {
	err = errStoreDown
	return
}

func (failingStore) SetPlatformTheme(ctx context.Context, themeID string, actor string, now time.Time) (r0 platformAppearance, err error) {
	err = errStoreDown
	return
}

func (failingStore) Close() (err error) {
	err = errStoreDown
	return
}
