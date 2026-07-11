package apigateway

import "time"

type adminCreateTeamRequest struct {
	Name         string `json:"name"`
	ContactEmail string `json:"contact_email"`
}

type adminCreatePlayerRequest struct {
	TeamID      int    `json:"team_id"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Password    string `json:"password"`
	Role        string `json:"role"`
}

type adminCreateChallengeRequest struct {
	Name               string `json:"name"`
	BaselineImage      string `json:"baseline_image"`
	CheckerImage       string `json:"checker_image"`
	SourceBundlePath   string `json:"source_bundle_path"`
	ServicePort        int    `json:"service_port,omitempty"`
	ServiceSubnetOctet int    `json:"service_subnet_octet,omitempty"`
	EgressEnabled      *bool  `json:"egress_enabled,omitempty"`
}

type adminUpdateTeamRequest struct {
	Name         string `json:"name"`
	ContactEmail string `json:"contact_email"`
}

type adminUpdatePlayerRequest struct {
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Role        string `json:"role"`
}

type adminUpdateChallengeRequest struct {
	Name             string `json:"name"`
	BaselineImage    string `json:"baseline_image"`
	CheckerImage     string `json:"checker_image"`
	SourceBundlePath string `json:"source_bundle_path"`
	EgressEnabled    *bool  `json:"egress_enabled,omitempty"`
}

type adminTeam struct {
	ID                 int    `json:"id"`
	Name               string `json:"name"`
	ContactEmail       string `json:"contact_email"`
	JoinKey            string `json:"join_key"`
	PlayerCount        int    `json:"player_count"`
	DeployedChallenges int    `json:"deployed_challenges"`
	Active             bool   `json:"active"`
	DeactivatedAt      string `json:"deactivated_at,omitempty"`
}

type adminPlayer struct {
	ID                 int    `json:"id"`
	TeamID             int    `json:"team_id"`
	TeamName           string `json:"team_name"`
	DisplayName        string `json:"display_name"`
	Email              string `json:"email"`
	Role               string `json:"role"`
	WireGuardPeer      string `json:"wireguard_peer"`
	WireGuardAddress   string `json:"wireguard_address"`
	WireGuardStatus    string `json:"wireguard_status"`
	WireGuardIssuedAt  string `json:"wireguard_issued_at"`
	WireGuardRevokedAt string `json:"wireguard_revoked_at,omitempty"`
	CreatedAt          string `json:"created_at"`
	Active             bool   `json:"active"`
	DeactivatedAt      string `json:"deactivated_at,omitempty"`
}

type participantRegisterRequest struct {
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Password    string `json:"password"`
}

type participantJoinExistingTeamRequest struct {
	TeamKey string `json:"team_key"`
}

type participantUpdateProfileRequest struct {
	DisplayName      string `json:"display_name"`
	Email            string `json:"email"`
	TeamName         string `json:"team_name"`
	TeamContactEmail string `json:"team_contact_email"`
}

type adminWireGuardPeer struct {
	PlayerID        int    `json:"player_id"`
	TeamID          int    `json:"team_id"`
	TeamName        string `json:"team_name"`
	DisplayName     string `json:"display_name"`
	Email           string `json:"email,omitempty"`
	WireGuardPeer   string `json:"wireguard_peer"`
	Address         string `json:"address"`
	Status          string `json:"status"`
	ServerEndpoint  string `json:"server_endpoint"`
	ServerPublicKey string `json:"server_public_key"`
	ClientPublicKey string `json:"client_public_key"`
	AllowedIPs      string `json:"allowed_ips"`
	DNS             string `json:"dns"`
	Config          string `json:"config"`
	DownloadName    string `json:"download_name"`
	IssuedAt        string `json:"issued_at"`
	RevokedAt       string `json:"revoked_at,omitempty"`
}

type WireGuardGatewayPeer struct {
	PlayerID        int    `json:"player_id"`
	TeamID          int    `json:"team_id"`
	TeamName        string `json:"team_name"`
	DisplayName     string `json:"display_name"`
	WireGuardPeer   string `json:"wireguard_peer"`
	Address         string `json:"address"`
	Status          string `json:"status"`
	ClientPublicKey string `json:"client_public_key"`
	PresharedKey    string `json:"preshared_key"`
}

type WireGuardGatewayStatus struct {
	State           string `json:"state"`
	Mode            string `json:"mode"`
	Interface       string `json:"interface,omitempty"`
	FirewallBackend string `json:"firewall_backend,omitempty"`
	ConfigPath      string `json:"config_path,omitempty"`
	PeersPath       string `json:"peers_path,omitempty"`
	RulesPath       string `json:"rules_path,omitempty"`
	StatusPath      string `json:"status_path,omitempty"`
	PeersTotal      int    `json:"peers_total"`
	PeersActive     int    `json:"peers_active"`
	PeersRevoked    int    `json:"peers_revoked"`
	Revision        string `json:"revision,omitempty"`
	AppliedAt       string `json:"applied_at,omitempty"`
	LastError       string `json:"last_error,omitempty"`
}

type adminChallenge struct {
	ID                 int                        `json:"id"`
	Name               string                     `json:"name"`
	BaselineImage      string                     `json:"baseline_image"`
	CheckerImage       string                     `json:"checker_image"`
	SourceBundlePath   string                     `json:"source_bundle_path"`
	ServicePort        int                        `json:"service_port"`
	ServiceSubnetOctet int                        `json:"service_subnet_octet"`
	EgressEnabled      bool                       `json:"egress_enabled"`
	Published          bool                       `json:"published"`
	Maintenance        bool                       `json:"maintenance"`
	MaintenanceAt      string                     `json:"maintenance_at,omitempty"`
	// PlayFromTick, when set, defers checker/scoring/participant access until
	// that tick id exists (set on maintenance resume to "next tick").
	PlayFromTick       int                        `json:"play_from_tick,omitempty"`
	DeployedTeams      int                        `json:"deployed_teams"`
	TotalTeams         int                        `json:"total_teams"`
	RuntimeStatus      string                     `json:"runtime_status"`
	QueuedTeams        int                        `json:"queued_teams"`
	ReadyTeams         int                        `json:"ready_teams"`
	CreatedAt          string                     `json:"created_at"`
	LastValidation     *ChallengeValidationResult `json:"last_validation,omitempty"`
}

type adminDeployment struct {
	JobID             int    `json:"job_id"`
	ChallengeID       int    `json:"challenge_id"`
	ChallengeName     string `json:"challenge_name"`
	Status            string `json:"status"`
	Published         bool   `json:"published"`
	DeployedTeamCount int    `json:"deployed_team_count"`
	TotalTeamCount    int    `json:"total_team_count"`
	QueuedTeamCount   int    `json:"queued_team_count"`
	ReadyTeamCount    int    `json:"ready_team_count"`
	CreatedAt         string `json:"created_at"`
	CompletedAt       string `json:"completed_at,omitempty"`
}

type adminDeploymentJob struct {
	ID              int    `json:"id"`
	ChallengeID     int    `json:"challenge_id"`
	ChallengeName   string `json:"challenge_name"`
	Status          string `json:"status"`
	TargetTeamCount int    `json:"target_team_count"`
	QueuedTeamCount int    `json:"queued_team_count"`
	ReadyTeamCount  int    `json:"ready_team_count"`
	FailedTeamCount int    `json:"failed_team_count"`
	CreatedAt       string `json:"created_at"`
	CompletedAt     string `json:"completed_at,omitempty"`
}

type adminReconcileResult struct {
	ProcessedJobs      int `json:"processed_jobs"`
	ProcessedInstances int `json:"processed_instances"`
	CompletedJobs      int `json:"completed_jobs"`
}

type adminAuditLog struct {
	ID         int    `json:"id"`
	ActorType  string `json:"actor_type"`
	Actor      string `json:"actor"`
	Action     string `json:"action"`
	TargetType string `json:"target_type"`
	Target     string `json:"target"`
	Status     string `json:"status"`
	Message    string `json:"message"`
	Metadata   string `json:"metadata"`
	CreatedAt  string `json:"created_at"`
}

type adminAuditLogEntry struct {
	ActorType  string
	Actor      string
	Action     string
	TargetType string
	Target     string
	Status     string
	Message    string
	Metadata   string
}

type adminAuditLogQuery struct {
	Limit      int
	Offset     int
	ActorType  string
	Action     string
	TargetType string
	Status     string
}

type adminAuditLogPage struct {
	Items      []adminAuditLog `json:"items"`
	Limit      int             `json:"limit"`
	Offset     int             `json:"offset"`
	TotalCount int             `json:"total_count"`
	HasPrev    bool            `json:"has_prev"`
	HasNext    bool            `json:"has_next"`
}

type ChallengeValidationRequest struct {
	ChallengeID   int    `json:"challenge_id"`
	Name          string `json:"name"`
	BaselineImage string `json:"baseline_image"`
	CheckerImage  string `json:"checker_image"`
}

type ChallengeValidationResult struct {
	ChallengeID            int    `json:"challenge_id"`
	Name                   string `json:"name"`
	BaselineImage          string `json:"baseline_image"`
	CheckerImage           string `json:"checker_image"`
	Status                 string `json:"status"`
	BaselineSSHContractOK  bool   `json:"baseline_ssh_contract_ok"`
	CheckerContractOK      bool   `json:"checker_contract_ok"`
	ServiceStateContractOK bool   `json:"service_state_contract_ok"`
	CheckedAt              string `json:"checked_at"`
	Message                string `json:"message,omitempty"`
}

type adminPlatformSettings struct {
	FlagFormatPrefix string `json:"flag_format_prefix"`
	FlagFormatActive string `json:"flag_format_active,omitempty"`
	MaxTeamMembers   int    `json:"max_team_members"`
	UpdatedAt        string `json:"updated_at,omitempty"`
	UpdatedBy        string `json:"updated_by,omitempty"`
}

type adminUpdatePlatformSettingsRequest struct {
	FlagFormatPrefix string `json:"flag_format_prefix"`
	MaxTeamMembers   *int   `json:"max_team_members,omitempty"`
}

type scoreboardFreezeStatus struct {
	Frozen          bool   `json:"frozen"`
	Configured      bool   `json:"configured"`
	FreezeAt        string `json:"freeze_at,omitempty"`
	UnfreezeAt      string `json:"unfreeze_at,omitempty"`
	SnapshotTakenAt string `json:"snapshot_taken_at,omitempty"`
}

type scoreboardFreezeRequest struct {
	FreezeAt   string `json:"freeze_at"`
	UnfreezeAt string `json:"unfreeze_at,omitempty"`
}

func scoreboardFreezeStatusFrom(window scoreboardFreezeWindow, now time.Time) scoreboardFreezeStatus {
	status := scoreboardFreezeStatus{
		Frozen:     window.activeAt(now),
		Configured: window.FreezeAt != nil,
	}
	if window.FreezeAt != nil {
		status.FreezeAt = window.FreezeAt.UTC().Format(time.RFC3339)
	}
	if window.UnfreezeAt != nil {
		status.UnfreezeAt = window.UnfreezeAt.UTC().Format(time.RFC3339)
	}
	if window.SnapshotTakenAt != nil {
		status.SnapshotTakenAt = window.SnapshotTakenAt.UTC().Format(time.RFC3339)
	}
	return status
}
