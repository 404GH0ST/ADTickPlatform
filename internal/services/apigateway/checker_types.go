package apigateway

type CheckerValidationRequest struct {
	ChallengeID  int    `json:"challenge_id"`
	Name         string `json:"name"`
	CheckerImage string `json:"checker_image"`
}

type CheckerValidationResult struct {
	ChallengeID            int    `json:"challenge_id"`
	Name                   string `json:"name"`
	CheckerImage           string `json:"checker_image"`
	Status                 string `json:"status"`
	ContractOK             bool   `json:"contract_ok"`
	ServiceStateContractOK bool   `json:"service_state_contract_ok"`
	CheckedAt              string `json:"checked_at"`
	Message                string `json:"message,omitempty"`
}

type CheckerExecutionRequest struct {
	ChallengeID    int    `json:"challenge_id"`
	TeamID         int    `json:"team_id"`
	TeamName       string `json:"team_name,omitempty"`
	ChallengeName  string `json:"challenge_name,omitempty"`
	CheckerImage   string `json:"checker_image"`
	Phase          string `json:"phase"`
	Target         string `json:"target"`
	TargetHost     string `json:"target_host,omitempty"`
	TargetIP       string `json:"target_ip,omitempty"`
	TargetPort     int    `json:"target_port,omitempty"`
	TickID         int    `json:"tick_id"`
	Flag           string `json:"flag,omitempty"`
	Metadata       string `json:"metadata,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

type CheckerExecutionResult struct {
	ChallengeID  int    `json:"challenge_id"`
	TeamID       int    `json:"team_id"`
	Phase        string `json:"phase"`
	Status       string `json:"status"`
	ExitCode     int    `json:"exit_code"`
	CheckedAt    string `json:"checked_at"`
	Message      string `json:"message,omitempty"`
	Output       string `json:"output,omitempty"`
	ServiceState string `json:"service_state,omitempty"`
	StateMessage string `json:"state_message,omitempty"`
}
