package apigateway

type ControllerRuntimeTask struct {
	DeploymentJobID int    `json:"deployment_job_id"`
	TeamID          int    `json:"team_id"`
	ChallengeID     int    `json:"challenge_id"`
	ChallengeName   string `json:"challenge_name"`
	RuntimeKind     string `json:"runtime_kind"`
	ContainerName   string `json:"container_name"`
	StateVolume     string `json:"state_volume"`
	BaselineImage   string `json:"baseline_image"`
	CheckerToken    string `json:"checker_token,omitempty"`
	Endpoint        string `json:"endpoint"`
	SSHHost         string `json:"ssh_host"`
	ServicePort     int    `json:"service_port"`
	// UnlockProofEpoch is mixed into AD_PLATFORM_UNLOCK_PROOF. Bumping it
	// invalidates previously stolen proofs after a security-fix redeploy.
	UnlockProofEpoch int `json:"unlock_proof_epoch"`
}

type ControllerServiceAccessPolicy struct {
	TeamID               int      `json:"team_id"`
	TeamName             string   `json:"team_name"`
	ChallengeID          int      `json:"challenge_id"`
	ChallengeName        string   `json:"challenge_name"`
	ServiceIP            string   `json:"service_ip"`
	ServicePort          int      `json:"service_port"`
	SSHPort              int      `json:"ssh_port"`
	SSHUnlocked          bool     `json:"ssh_unlocked"`
	AllowedPeerAddresses []string `json:"allowed_peer_addresses"`
	EgressEnabled        bool     `json:"egress_enabled"`
	// NetworkClosed blocks participant traffic to the service IP. True while the
	// match is not started or paused, the challenge is under maintenance, the
	// team is inactive or deferred (play_from_tick). Organizer WireGuard peers
	// listed in AllowedPeerAddresses still reach service + SSH for ops checks.
	NetworkClosed bool `json:"network_closed"`
}

type ControllerAccessStatus struct {
	State                  string `json:"state"`
	Mode                   string `json:"mode"`
	Interface              string `json:"interface,omitempty"`
	InternetInterface      string `json:"internet_interface,omitempty"`
	FirewallBackend        string `json:"firewall_backend,omitempty"`
	RulesPath              string `json:"rules_path,omitempty"`
	StatusPath             string `json:"status_path,omitempty"`
	PoliciesTotal          int    `json:"policies_total"`
	SSHOpenServices        int    `json:"ssh_open_services"`
	SSHLockedServices      int    `json:"ssh_locked_services"`
	EgressDisabledServices int    `json:"egress_disabled_services"`
	AllowedPeersTotal      int    `json:"allowed_peers_total"`
	Revision               string `json:"revision,omitempty"`
	AppliedAt              string `json:"applied_at,omitempty"`
	LastError              string `json:"last_error,omitempty"`
}

type ControllerSSHCredential struct {
	Password string `json:"password"`
}
