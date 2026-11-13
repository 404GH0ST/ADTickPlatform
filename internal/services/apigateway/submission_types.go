package apigateway

import "time"

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
	ID                int64  `json:"id"`
	Status            string `json:"status"`
	Service           string `json:"service"`
	FirstBloodCleared bool   `json:"first_blood_cleared"`
}
