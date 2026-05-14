package apigateway

import "time"

type authenticateRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type submitRequest struct {
	Flags []string `json:"flags"`
}

type unlockRequest struct {
	Proof string `json:"proof"`
}

type challenge struct {
	ID                int    `json:"id"`
	Name              string `json:"name"`
	HasSourceDownload bool   `json:"has_source_download"`
}

type authenticateResponse struct {
	Token     string `json:"token"`
	TokenType string `json:"token_type"`
}

type SubmissionVerdictAlias struct {
	Flag   string `json:"flag"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

type submissionVerdict = SubmissionVerdictAlias

type submissionResult struct {
	Results       []submissionVerdict `json:"results"`
	AcceptedCount int                 `json:"accepted_count"`
	RejectedCount int                 `json:"rejected_count"`
}

type unlockData struct {
	ChallengeID int  `json:"challenge_id"`
	TeamID      int  `json:"team_id"`
	Unlocked    bool `json:"unlocked"`
}

type sshSessionData struct {
	ChallengeID    int    `json:"challenge_id"`
	Host           string `json:"host"`
	Port           int    `json:"port"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	PasswordMode   string `json:"password_mode,omitempty"`
	ConnectionHint string `json:"connection_hint"`
}

type resetData struct {
	ChallengeID     int    `json:"challenge_id"`
	TeamID          int    `json:"team_id"`
	Action          string `json:"action"`
	UnlockPreserved bool   `json:"unlock_preserved,omitempty"`
}

type serviceState struct {
	ChallengeID   int    `json:"challenge_id"`
	TeamID        int    `json:"team_id"`
	Name          string `json:"name"`
	Endpoint      string `json:"endpoint"`
	Status        string `json:"status"`
	Checker       string `json:"checker"`
	Unlocked      bool   `json:"unlocked"`
	SSHHint       string `json:"ssh_hint"`
	LastEvent     string `json:"last_event"`
	ResetCooldown string `json:"reset_cooldown"`
	SLAStatus     string `json:"sla_status,omitempty"`
	SLAPhase      string `json:"sla_phase,omitempty"`
	SLATickID     int    `json:"sla_tick_id,omitempty"`
	SLAMessage    string `json:"sla_message,omitempty"`
}

type ScoreRowAlias struct {
	Rank     int                          `json:"rank"`
	Team     string                       `json:"team"`
	Attack   float64                      `json:"attack"`
	Defense  float64                      `json:"defense"`
	SLA      float64                      `json:"sla"`
	Total    float64                      `json:"total"`
	Delta    string                       `json:"delta"`
	Services []ServiceScoreBreakdownAlias `json:"services,omitempty"`
}

type ServiceScoreBreakdownAlias struct {
	ChallengeID int     `json:"challenge_id"`
	Service     string  `json:"service"`
	Attack      float64 `json:"attack"`
	Defense     float64 `json:"defense"`
	SLA         float64 `json:"sla"`
	Total       float64 `json:"total"`
}

type ScoringAuditAlias struct {
	Status        string                      `json:"status"`
	StoredRows    int                         `json:"stored_rows"`
	ReplayedRows  int                         `json:"replayed_rows"`
	MismatchCount int                         `json:"mismatch_count"`
	Mismatches    []ScoringAuditMismatchAlias `json:"mismatches"`
}

type ScoringAuditMismatchAlias struct {
	Team     string  `json:"team"`
	Field    string  `json:"field"`
	Stored   float64 `json:"stored"`
	Replayed float64 `json:"replayed"`
	Delta    float64 `json:"delta"`
	Detail   string  `json:"detail,omitempty"`
}

type AttackEventAlias struct {
	ID        string    `json:"id"`
	Attacker  string    `json:"attacker"`
	Victim    string    `json:"victim"`
	Service   string    `json:"service"`
	Tick      int       `json:"tick"`
	Verdict   string    `json:"verdict"`
	CreatedAt time.Time `json:"-"`
}

type AttackFeedQuery struct {
	Limit    int
	Offset   int
	Attacker string
	Victim   string
	Service  string
	TickFrom int
	TickTo   int
}

type AttackFeedPage struct {
	Items      []AttackEventAlias `json:"items"`
	Limit      int                `json:"limit"`
	Offset     int                `json:"offset"`
	TotalCount int                `json:"total_count"`
	HasPrev    bool               `json:"has_prev"`
	HasNext    bool               `json:"has_next"`
}

type scoreRow = ScoreRowAlias
type attackEvent = AttackEventAlias
