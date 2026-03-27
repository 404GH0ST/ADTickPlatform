package apigateway

type GameTickStatus struct {
	ID                    int    `json:"id"`
	Status                string `json:"status"`
	TotalCheckerRuns      int    `json:"total_checker_runs"`
	SuccessfulCheckerRuns int    `json:"successful_checker_runs"`
	FailedCheckerRuns     int    `json:"failed_checker_runs"`
	SkippedCheckerRuns    int    `json:"skipped_checker_runs"`
	StartedAt             string `json:"started_at"`
	CompletedAt           string `json:"completed_at,omitempty"`
	Message               string `json:"message,omitempty"`
}

type GameSchedulerStatus struct {
	State           string `json:"state"`
	IntervalSeconds int    `json:"interval_seconds"`
	LastRunAt       string `json:"last_run_at,omitempty"`
	NextRunAt       string `json:"next_run_at,omitempty"`
	LastTickID      int    `json:"last_tick_id,omitempty"`
	LastError       string `json:"last_error,omitempty"`
}

type UpdateSchedulerRequest struct {
	IntervalSeconds int `json:"interval_seconds"`
}

type UpdateMatchScheduleRequest struct {
	ScheduledStartAt string `json:"scheduled_start_at,omitempty"`
	ScheduledEndAt   string `json:"scheduled_end_at,omitempty"`
}

type GameSchedulerEvent struct {
	ID        int64  `json:"id"`
	EventType string `json:"event_type"`
	Source    string `json:"source"`
	State     string `json:"state"`
	TickID    int    `json:"tick_id,omitempty"`
	Message   string `json:"message,omitempty"`
	CreatedAt string `json:"created_at"`
}

type GameSchedulerEventPage struct {
	Items      []GameSchedulerEvent `json:"items"`
	Limit      int                  `json:"limit"`
	Offset     int                  `json:"offset"`
	TotalCount int                  `json:"total_count"`
	HasPrev    bool                 `json:"has_prev"`
	HasNext    bool                 `json:"has_next"`
}

type GameSchedulerEventQuery struct {
	Limit     int
	Offset    int
	EventType string
	Source    string
	State     string
}

type GameMatchStatus struct {
	State                string `json:"state"`
	StartedAt            string `json:"started_at,omitempty"`
	EndedAt              string `json:"ended_at,omitempty"`
	ScheduledStartAt     string `json:"scheduled_start_at,omitempty"`
	ScheduledEndAt       string `json:"scheduled_end_at,omitempty"`
	ScheduleConfigured   bool   `json:"schedule_configured,omitempty"`
	AcceptingSubmissions bool   `json:"accepting_submissions"`
}

type GameStatus struct {
	Match                 *GameMatchStatus     `json:"match,omitempty"`
	CurrentTick           *GameTickStatus      `json:"current_tick,omitempty"`
	Scheduler             *GameSchedulerStatus `json:"scheduler,omitempty"`
	TotalTicks            int                  `json:"total_ticks"`
	TotalCheckerRuns      int                  `json:"total_checker_runs"`
	SuccessfulCheckerRuns int                  `json:"successful_checker_runs"`
	FailedCheckerRuns     int                  `json:"failed_checker_runs"`
	SkippedCheckerRuns    int                  `json:"skipped_checker_runs"`
}

type GameCheckerRun struct {
	ID            int64  `json:"id"`
	TickID        int    `json:"tick_id"`
	TeamID        int    `json:"team_id"`
	TeamName      string `json:"team_name"`
	ChallengeID   int    `json:"challenge_id"`
	ChallengeName string `json:"challenge_name"`
	Phase         string `json:"phase"`
	Target        string `json:"target"`
	CheckerImage  string `json:"checker_image"`
	Status        string `json:"status"`
	ExitCode      int    `json:"exit_code"`
	Message       string `json:"message,omitempty"`
	Output        string `json:"output,omitempty"`
	CheckedAt     string `json:"checked_at"`
}

type GameCheckerRunPage struct {
	Items      []GameCheckerRun `json:"items"`
	Limit      int              `json:"limit"`
	Offset     int              `json:"offset"`
	TotalCount int              `json:"total_count"`
	HasPrev    bool             `json:"has_prev"`
	HasNext    bool             `json:"has_next"`
}

type GameCheckerRunQuery struct {
	Limit       int
	Offset      int
	TickID      int
	TeamID      int
	ChallengeID int
	Phase       string
	Status      string
}

type GameSubmitFlagsRequest struct {
	TeamID int      `json:"team_id"`
	Flags  []string `json:"flags"`
}
