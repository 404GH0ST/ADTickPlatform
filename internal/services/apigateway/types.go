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
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type successEnvelope[T any] struct {
	Status string `json:"status"`
	Data   T      `json:"data"`
}

type SubmissionVerdictAlias struct {
	Flag    string `json:"flag"`
	Verdict string `json:"verdict"`
}

type submissionVerdict = SubmissionVerdictAlias

type unlockData struct {
	ChallengeID             int  `json:"challenge_id"`
	TeamID                  int  `json:"team_id"`
	Unlocked                bool `json:"unlocked"`
	SSHCredentialTTLSeconds int  `json:"ssh_credential_ttl_seconds"`
}

type sshSessionData struct {
	ChallengeID    int    `json:"challenge_id"`
	Host           string `json:"host"`
	Port           int    `json:"port"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	ExpiresAt      string `json:"expires_at"`
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
}

type ScoreRowAlias struct {
	Rank    int    `json:"rank"`
	Team    string `json:"team"`
	Attack  int    `json:"attack"`
	Defense int    `json:"defense"`
	SLA     int    `json:"sla"`
	Total   int    `json:"total"`
	Delta   string `json:"delta"`
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
