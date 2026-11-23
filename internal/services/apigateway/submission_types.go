package apigateway

import (
	"fmt"
	"strings"
	"time"
)

const (
	maxSubmitFlagsPerRequest = 128
	MaxSubmittedFlagBytes    = 512
	MaxFlagFormatPrefixBytes = 256
)

func ValidateSubmittedFlags(flags []string) error {
	if len(flags) == 0 {
		return fmt.Errorf("at least one flag is required.")
	}
	if len(flags) > maxSubmitFlagsPerRequest {
		return fmt.Errorf("too many flags in one request.")
	}
	for _, flag := range flags {
		if len(flag) > MaxSubmittedFlagBytes {
			return fmt.Errorf("each flag must be at most %d bytes.", MaxSubmittedFlagBytes)
		}
		// PostgreSQL text cannot store NUL, so one such value would otherwise
		// fail the whole rejected-attempt history write for its batch.
		if strings.ContainsRune(flag, 0) {
			return fmt.Errorf("flags must not contain NUL bytes.")
		}
	}
	return nil
}

type AdminSubmission struct {
	ID          int64     `json:"id"`
	TeamID      int       `json:"team_id"`
	Attacker    string    `json:"attacker"`
	Victim      string    `json:"victim"`
	ChallengeID int       `json:"challenge_id"`
	Service     string    `json:"service"`
	Flag        string    `json:"flag"`
	Tick        int       `json:"tick"`
	Status      string    `json:"status"`
	Detail      string    `json:"detail"`
	FirstBlood  bool      `json:"first_blood"`
	SubmittedAt time.Time `json:"submitted_at"`
	AttackID    string    `json:"-"`
}
type SubmissionQuery struct {
	AttackFeedQuery
	Status string
	Flag   string
}
type SubmissionPage struct {
	Items      []AdminSubmission `json:"items"`
	Limit      int               `json:"limit"`
	Offset     int               `json:"offset"`
	TotalCount int               `json:"total_count"`
	HasPrev    bool              `json:"has_prev"`
	HasNext    bool              `json:"has_next"`
}
type SubmissionDeletion struct {
	ID                     int64  `json:"id"`
	Status                 string `json:"status"`
	Service                string `json:"service"`
	FirstBloodCleared      bool   `json:"first_blood_cleared"`
	TeamID                 int    `json:"team_id,omitempty"`
	ChallengeID            int    `json:"challenge_id,omitempty"`
	SSHGrantRevoked        bool   `json:"ssh_grant_revoked,omitempty"`
	RevokedThrough         int64  `json:"revoked_through,omitempty"`
	AccessReconcilePending bool   `json:"access_reconcile_pending,omitempty"`
}
