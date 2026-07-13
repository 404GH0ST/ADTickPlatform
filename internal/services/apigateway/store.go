package apigateway

import (
	"context"
	"crypto/hmac"
	"crypto/md5" // #nosec G501 -- read-only compatibility for legacy stored tokens.
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var (
	ErrChallengeNotFound    = errors.New("challenge not found")
	ErrChallengeMaintenance = errors.New("challenge under maintenance")
	// ErrChallengeDeferred is returned when a challenge was resumed but only
	// re-enters play starting at play_from_tick (usually the next tick).
	ErrChallengeDeferred = errors.New("challenge resumes next tick")
	// ErrMatchNotStarted is returned when participant play is requested before the
	// match leaves not_started (pre-game warm deploy must stay dark).
	ErrMatchNotStarted = errors.New("contest has not started yet")
	// ErrMatchPaused is returned when participant play is requested while the
	// match is temporarily paused (submissions, unlock, reset, SSH, source).
	ErrMatchPaused           = errors.New("contest is temporarily paused")
	ErrDeploymentActive      = errors.New("deployment job is still active")
	ErrDeploymentNotFound    = errors.New("deployment job not found")
	ErrServiceLocked         = errors.New("service locked")
	ErrServiceUnavailable    = errors.New("service unavailable")
	ErrTeamNotFound          = errors.New("team not found")
	ErrPlayerNotFound        = errors.New("player not found")
	ErrDuplicateResource     = errors.New("duplicate resource")
	ErrInvalidCredentials    = errors.New("invalid credentials")
	ErrAccountDeactivated    = errors.New("account deactivated")
	ErrInvalidRuntimeConfig  = errors.New("invalid runtime config")
	ErrSubmissionUnavailable = errors.New("authoritative submission backend unavailable")
	ErrTeamMemberLimit       = errors.New("team member limit reached")
	ErrTeamAddressPool       = errors.New("team network address pool exhausted")
	ErrPlayerAddressPool     = errors.New("player network address pool exhausted")
	ErrResourceNotFound      = errors.New("resource not found")
)

type authenticatedPlayer struct {
	PlayerID         int
	TeamID           int
	TeamName         string
	TeamContactEmail string
	DisplayName      string
	Email            string
	Role             string
	SessionVersion   int
}

type Store interface {
	AuthenticatePlayer(ctx context.Context, email, password string) (authenticatedPlayer, error)
	RegisterPlayer(ctx context.Context, input participantRegisterRequest, now time.Time) (authenticatedPlayer, error)
	ValidatePlayerSession(ctx context.Context, playerID, teamID int, role string, sessionVersion int) (authenticatedPlayer, error)
	UpdateParticipantProfile(ctx context.Context, playerID int, input participantUpdateProfileRequest) (authenticatedPlayer, error)
	ChangeParticipantPassword(ctx context.Context, playerID int, currentPassword, newPassword string) (int, error)
	ListAnnouncements(ctx context.Context) ([]matchAnnouncement, error)
	CreateAnnouncement(ctx context.Context, body, createdBy string, now time.Time) (matchAnnouncement, error)
	DeleteAnnouncement(ctx context.Context, id int) error
	ImportAdminTeams(ctx context.Context, teams []adminBulkImportTeam, now time.Time) (adminBulkImportResult, error)
	ListChallenges(ctx context.Context) ([]challenge, error)
	ListPublicServices(ctx context.Context) (map[string]map[string][]string, error)
	ListScoreboard(ctx context.Context) ([]scoreRow, error)
	GetScoreboardFreeze(ctx context.Context) (scoreboardFreezeWindow, error)
	SetScoreboardFreezeWindow(ctx context.Context, freezeAt, unfreezeAt *time.Time, now time.Time) (scoreboardFreezeWindow, error)
	ClearScoreboardFreeze(ctx context.Context, now time.Time) (scoreboardFreezeWindow, error)
	SaveFrozenScoreboardSnapshot(ctx context.Context, rows []scoreRow, takenAt time.Time) (scoreboardFreezeWindow, error)
	ListAttackFeed(ctx context.Context) ([]attackEvent, error)
	ListTeamServices(ctx context.Context, teamID int) ([]serviceState, error)
	SubmitFlags(ctx context.Context, teamID int, flags []string) ([]submissionVerdict, error)
	ValidateServiceAction(ctx context.Context, teamID, challengeID int) error
	UnlockService(ctx context.Context, teamID, challengeID int) (unlockData, error)
	CreateSSHSession(ctx context.Context, teamID, challengeID int, now time.Time) (sshSessionData, error)
	MarkSSHSessionApplyFailure(ctx context.Context, teamID, challengeID int) error
	PrepareFactoryResetService(ctx context.Context, teamID, challengeID int) (resetData, error)
	CompleteFactoryResetService(ctx context.Context, teamID, challengeID int) (resetData, error)
	MarkFactoryResetFailure(ctx context.Context, teamID, challengeID int) error
	RestartService(ctx context.Context, teamID, challengeID int) (resetData, error)
	ListAdminTeams(ctx context.Context) ([]adminTeam, error)
	CreateAdminTeam(ctx context.Context, input adminCreateTeamRequest) (adminTeam, error)
	UpdateAdminTeam(ctx context.Context, teamID int, input adminUpdateTeamRequest) (adminTeam, error)
	DeleteAdminTeam(ctx context.Context, teamID int) error
	SetTeamActive(ctx context.Context, teamID int, active bool, now time.Time) (adminTeam, error)
	RequeueTeamServices(ctx context.Context, teamID int) error
	SetChallengeMaintenance(ctx context.Context, challengeID int, maintenance bool, now time.Time) (adminChallenge, error)
	// ChallengePlayBlocked is true while maintenance is set or play_from_tick is
	// still in the future (warm redeploy window before the challenge re-enters play).
	ChallengePlayBlocked(ctx context.Context, challengeID int) (bool, error)
	// GetChallengeUnlockProofEpoch returns the current unlock proof epoch (>= 1).
	GetChallengeUnlockProofEpoch(ctx context.Context, challengeID int) (int, error)
	// RotateChallengeUnlockProof bumps unlock_proof_epoch and clears unlocks so
	// previously extracted proofs no longer verify after instances are redeployed.
	RotateChallengeUnlockProof(ctx context.Context, challengeID int, now time.Time) (adminChallenge, error)
	RequeueChallengeServices(ctx context.Context, challengeID int) error
	ListAdminPlayers(ctx context.Context) ([]adminPlayer, error)
	CreateAdminPlayer(ctx context.Context, input adminCreatePlayerRequest, now time.Time) (adminPlayer, error)
	UpdateAdminPlayer(ctx context.Context, playerID int, input adminUpdatePlayerRequest) (adminPlayer, error)
	DeleteAdminPlayer(ctx context.Context, playerID int) error
	SetPlayerActive(ctx context.Context, playerID int, active bool, now time.Time) (adminPlayer, error)
	GetAdminPlayerWireGuardConfig(ctx context.Context, playerID int) (adminWireGuardPeer, error)
	RotateAdminPlayerWireGuardConfig(ctx context.Context, playerID int, now time.Time) (adminWireGuardPeer, error)
	RevokeAdminPlayerWireGuardConfig(ctx context.Context, playerID int, now time.Time) (adminWireGuardPeer, error)
	ListWireGuardGatewayPeers(ctx context.Context) ([]WireGuardGatewayPeer, error)
	IsMatchPaused(ctx context.Context) (bool, error)
	// IsMatchStarted is true once the match has left not_started (running,
	// paused, or finished). Before that, challenges/public services stay hidden
	// and network access stays closed.
	IsMatchStarted(ctx context.Context) (bool, error)
	ListAdminChallenges(ctx context.Context) ([]adminChallenge, error)
	CreateAdminChallenge(ctx context.Context, input adminCreateChallengeRequest, now time.Time) (adminChallenge, error)
	UpdateAdminChallenge(ctx context.Context, challengeID int, input adminUpdateChallengeRequest) (adminChallenge, error)
	DeleteAdminChallenge(ctx context.Context, challengeID int) error
	DeployAdminChallenge(ctx context.Context, challengeID int) (adminDeployment, error)
	SaveAdminChallengeValidation(ctx context.Context, result ChallengeValidationResult) error
	ListAdminDeployments(ctx context.Context) ([]adminDeploymentJob, error)
	DeleteAdminDeployment(ctx context.Context, deploymentID int) error
	JoinExistingPlayerTeam(ctx context.Context, playerID int, teamKey string, now time.Time) (authenticatedPlayer, error)
	ListAdminAuditLogs(ctx context.Context, query adminAuditLogQuery) (adminAuditLogPage, error)
	AppendAdminAuditLog(ctx context.Context, entry adminAuditLogEntry) error
	ListControllerRuntimeTasks(ctx context.Context) ([]ControllerRuntimeTask, error)
	GetControllerRuntimeTask(ctx context.Context, teamID, challengeID int) (ControllerRuntimeTask, error)
	ListControllerServiceAccessPolicies(ctx context.Context) ([]ControllerServiceAccessPolicy, error)
	GetControllerServiceAccessPolicy(ctx context.Context, teamID, challengeID int) (ControllerServiceAccessPolicy, error)
	ReconcileAdminDeployments(ctx context.Context, now time.Time) (adminReconcileResult, error)
	GetPlatformSettings(ctx context.Context) (adminPlatformSettings, error)
	UpdatePlatformSettings(ctx context.Context, input adminUpdatePlatformSettingsRequest, actor string, now time.Time) (adminPlatformSettings, error)
	SetActiveFlagFormat(ctx context.Context, format string, now time.Time) (adminPlatformSettings, error)
	Close() error
}

func NewTeamJoinKey() (string, error) {
	var raw [12]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf(
		"TEAM-%s-%s-%s",
		strings.ToUpper(hex.EncodeToString(raw[:4])),
		strings.ToUpper(hex.EncodeToString(raw[4:8])),
		strings.ToUpper(hex.EncodeToString(raw[8:])),
	), nil
}

func DefaultServicePort(challengeID int) int {
	return 10000 + challengeID
}

func DefaultServiceSubnetOctet(challengeID int) int {
	if challengeID <= 0 {
		return 1
	}
	return challengeID
}

func ServiceIP(subnetOctet, teamID int) string {
	return fmt.Sprintf("10.80.%d.%d", subnetOctet, teamServiceOctet(teamID))
}

const (
	minimumTeamNetworkID         = 101
	maximumTeamNetworkID         = 344
	wireGuardPeerAddressPoolSize = 50800
)

func ServiceEndpointFor(subnetOctet, servicePort, teamID int) string {
	return fmt.Sprintf("%s:%d", ServiceIP(subnetOctet, teamID), servicePort)
}

func ParseEndpoint(endpoint string) (string, int) {
	host, portText, ok := strings.Cut(strings.TrimSpace(endpoint), ":")
	if !ok {
		return strings.TrimSpace(endpoint), 0
	}
	port, err := strconv.Atoi(strings.TrimSpace(portText))
	if err != nil {
		return strings.TrimSpace(host), 0
	}
	return strings.TrimSpace(host), port
}

func validateChallengeRuntimeConfig(servicePort, serviceSubnetOctet int) error {
	if servicePort <= 0 || servicePort > 65535 {
		return fmt.Errorf("%w: service_port must be between 1 and 65535", ErrInvalidRuntimeConfig)
	}
	if serviceSubnetOctet <= 0 || serviceSubnetOctet > 254 {
		return fmt.Errorf("%w: service_subnet_octet must be between 1 and 254", ErrInvalidRuntimeConfig)
	}
	return nil
}

func challengePort(challengeID int) int {
	return DefaultServicePort(challengeID)
}

func serviceEndpoint(challengeID, teamID int) string {
	return ServiceEndpointFor(DefaultServiceSubnetOctet(challengeID), DefaultServicePort(challengeID), teamID)
}

func sshHost(challengeID, teamID int) string {
	return ServiceIP(DefaultServiceSubnetOctet(challengeID), teamID)
}

func sshUsername() string {
	return "root"
}

func sshConnectionHint(challengeID, teamID int) string {
	return fmt.Sprintf("ssh %s@%s", sshUsername(), sshHost(challengeID, teamID))
}

func stableRootPassword(secret string, teamID, challengeID int) string {
	key := []byte(strings.TrimSpace(secret))
	if len(key) == 0 {
		panic("stableRootPassword requires a non-empty secret")
	}
	mac := hmac.New(sha256.New, key)
	fmt.Fprintf(mac, "ssh-root:%d:%d", teamID, challengeID)
	encoded := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("Adp-%s-Aa1!", encoded[:18])
}

func issueOneTimeRootPassword() (string, error) {
	buffer := make([]byte, 18)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func hashSecret(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func legacyMD5Secret(value string) string {
	// #nosec G401,G501 -- read-only compatibility for legacy stored tokens.
	sum := md5.Sum([]byte(value))
	return hex.EncodeToString(sum[:])
}

const (
	passwordHashScheme     = "pbkdf2-sha256"
	passwordHashIterations = 210000
	passwordHashSaltBytes  = 16
	passwordHashKeyBytes   = 32
	minimumPasswordLength  = 8
)

var invalidCredentialsPasswordHash = mustHashPassword("invalid-credentials-dummy-password")

func validPassword(value string) bool {
	return len(value) >= minimumPasswordLength && strings.TrimSpace(value) != ""
}

func validatePlayerAuthentication(storedHash, password, role string, playerActive, teamActive bool) error {
	if !passwordMatches(storedHash, password) {
		return ErrInvalidCredentials
	}
	if !playerActive || (!strings.EqualFold(strings.TrimSpace(role), "organizer") && !teamActive) {
		return ErrAccountDeactivated
	}
	return nil
}

func consumeUnknownPlayerPassword(password string) error {
	_ = passwordMatches(invalidCredentialsPasswordHash, password)
	return ErrInvalidCredentials
}

func hashPassword(value string) (string, error) {
	salt := make([]byte, passwordHashSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := pbkdf2.Key(sha256.New, value, salt, passwordHashIterations, passwordHashKeyBytes)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(
		"%s$i=%d$s=%s$h=%s",
		passwordHashScheme,
		passwordHashIterations,
		base64.RawURLEncoding.EncodeToString(salt),
		base64.RawURLEncoding.EncodeToString(key),
	), nil
}

func mustHashPassword(value string) string {
	hashed, err := hashPassword(value)
	if err != nil {
		panic(err)
	}
	return hashed
}

func passwordMatches(storedHash, password string) bool {
	trimmed := strings.TrimSpace(storedHash)
	if strings.HasPrefix(trimmed, passwordHashScheme+"$") {
		return verifyPBKDF2Password(trimmed, password)
	}
	return trimmed == hashSecret(password) || trimmed == legacyMD5Secret(password)
}

func passwordHashNeedsUpgrade(storedHash string) bool {
	return !strings.HasPrefix(strings.TrimSpace(storedHash), passwordHashScheme+"$")
}

func verifyPBKDF2Password(storedHash, password string) bool {
	fields := strings.Split(storedHash, "$")
	if len(fields) != 4 || fields[0] != passwordHashScheme {
		return false
	}
	iterationsText, ok := strings.CutPrefix(fields[1], "i=")
	if !ok {
		return false
	}
	iterations, err := strconv.Atoi(iterationsText)
	if err != nil || iterations <= 0 {
		return false
	}
	saltText, ok := strings.CutPrefix(fields[2], "s=")
	if !ok {
		return false
	}
	hashText, ok := strings.CutPrefix(fields[3], "h=")
	if !ok {
		return false
	}
	salt, err := base64.RawURLEncoding.DecodeString(saltText)
	if err != nil {
		return false
	}
	expected, err := base64.RawURLEncoding.DecodeString(hashText)
	if err != nil {
		return false
	}
	actual, err := pbkdf2.Key(sha256.New, password, salt, iterations, len(expected))
	if err != nil {
		return false
	}
	return hmac.Equal(actual, expected)
}

func wireguardPeerName(teamID, playerID int) string {
	if teamID == 0 {
		return fmt.Sprintf("organizer-player-%d", playerID)
	}
	return fmt.Sprintf("team-%d-player-%d", teamID, playerID)
}

func newSubmissionResult(items []submissionVerdict) submissionResult {
	result := submissionResult{
		Results:       items,
		RejectedCount: len(items),
	}
	for _, item := range items {
		if item.Status == "accepted" {
			result.AcceptedCount++
			result.RejectedCount--
		}
	}
	return result
}

func normalizedRole(role string) string {
	trimmed := strings.ToLower(strings.TrimSpace(role))
	if trimmed == "organizer" || trimmed == "admin" {
		return "organizer"
	}
	if trimmed == "captain" {
		return "captain"
	}
	return "member"
}

func defaultChallengeImages(name string) (string, string) {
	slug := slugName(name)
	return fmt.Sprintf("registry.local/%s:baseline", slug), fmt.Sprintf("registry.local/%s-checker:latest", slug)
}

func serviceContainerName(challengeName string, teamID int) string {
	return fmt.Sprintf("svc-%s-team-%d", slugName(challengeName), teamID)
}

func serviceStateVolumeName(challengeName string, teamID int) string {
	return fmt.Sprintf("svc-%s-team-%d-state", slugName(challengeName), teamID)
}

func slugName(value string) string {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" {
		return "service"
	}

	var builder strings.Builder
	lastDash := false
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastDash = false
		case !lastDash:
			builder.WriteByte('-')
			lastDash = true
		}
	}

	slug := strings.Trim(builder.String(), "-")
	if slug == "" {
		return "service"
	}
	return slug
}

func teamServiceOctet(teamID int) int {
	if teamID >= minimumTeamNetworkID && teamID <= maximumTeamNetworkID {
		return teamID - 90
	}
	if teamID == 0 {
		return 1
	}
	return 0
}

func splitCSVList(value string) []string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	parts := strings.Split(trimmed, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		result = append(result, part)
	}
	return result
}

func demoteStatus(current string) string {
	if current == "degraded" {
		return "warming"
	}
	return current
}

func defaultServiceState(challengeID, teamID int, challengeName string) *serviceState {
	return defaultServiceStateForConfig(challengeID, teamID, challengeName, DefaultServicePort(challengeID), DefaultServiceSubnetOctet(challengeID))
}

func defaultServiceStateForConfig(challengeID, teamID int, challengeName string, servicePort, serviceSubnetOctet int) *serviceState {
	return &serviceState{
		ChallengeID:   challengeID,
		TeamID:        teamID,
		Name:          challengeName,
		Endpoint:      ServiceEndpointFor(serviceSubnetOctet, servicePort, teamID),
		Status:        "stable",
		Checker:       "passing",
		Unlocked:      false,
		SSHHint:       "solve service to generate SSH credential",
		LastEvent:     "no patch applied yet",
		ResetCooldown: "ready",
		SLAStatus:     "ok",
		SLAMessage:    "checker passing; service state detail unavailable",
	}
}

func sanitizeSourceBundlePath(value string) string {
	return strings.TrimSpace(value)
}

func challengeRuntimeStatus(published bool, totalTeams, readyTeams, queuedTeams int) string {
	switch {
	case !published:
		return "draft"
	case queuedTeams > 0:
		return "deploying"
	case totalTeams > 0 && readyTeams >= totalTeams:
		return "ready"
	default:
		return "partial"
	}
}
