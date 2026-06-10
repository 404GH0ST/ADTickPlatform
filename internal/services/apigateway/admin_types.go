package apigateway

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
	Weight             int    `json:"weight"`
	ServicePort        int    `json:"service_port,omitempty"`
	ServiceSubnetOctet int    `json:"service_subnet_octet,omitempty"`
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
	Weight           int    `json:"weight"`
}

type adminTeam struct {
	ID                 int    `json:"id"`
	Name               string `json:"name"`
	ContactEmail       string `json:"contact_email"`
	PlayerCount        int    `json:"player_count"`
	DeployedChallenges int    `json:"deployed_challenges"`
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
}

type adminWireGuardPeer struct {
	PlayerID        int    `json:"player_id"`
	TeamID          int    `json:"team_id"`
	TeamName        string `json:"team_name"`
	DisplayName     string `json:"display_name"`
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
	Weight             int                        `json:"weight"`
	ServicePort        int                        `json:"service_port"`
	ServiceSubnetOctet int                        `json:"service_subnet_octet"`
	Published          bool                       `json:"published"`
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
