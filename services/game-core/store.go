package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"adplatform/internal/services/apigateway"
)

var (
	errTickInProgress    = errors.New("a game tick is already running")
	errContestNotStarted = errors.New("contest has not started yet.")
	errContestPaused     = errors.New("contest is temporarily paused.")
	errContestOver       = errors.New("contest is over.")
	// errFlagNoLongerValid is returned when accept-time revalidation fails
	// (match left running, or tick advanced past the flag expiry). Callers map
	// it to a per-flag "invalid" verdict rather than failing the whole batch.
	errFlagNoLongerValid = errors.New("flag is no longer valid")
)

// reapedTickMessage marks ticks that were left in the "running" state by a
// crash mid-tick and recovered on the next startup. Without this recovery,
// StartNextTick would refuse every future tick with errTickInProgress.
const reapedTickMessage = "reaped: incomplete tick recovered on startup"

const faustRecoveringValue = 0.5

type checkerTarget struct {
	TeamID        int
	TeamName      string
	ChallengeID   int
	ChallengeName string
	CheckerImage  string
	CheckerToken  string
	Target        string
	TargetHost    string
	TargetIP      string
	TargetPort    int
}

type checkerRunRecord struct {
	TickID               int
	TeamID               int
	TeamName             string
	ChallengeID          int
	ChallengeName        string
	Phase                string
	Target               string
	CheckerImage         string
	Status               string
	ExitCode             int
	Message              string
	Output               string
	ReportedServiceState string
	ReportedStateMessage string
	CheckedAt            time.Time
}

type issuedFlagRecord struct {
	Flag          string
	OwnerTeamID   int
	OwnerTeamName string
	ChallengeID   int
	ChallengeName string
	IssuedTick    int
	ExpiresTick   int
	CreatedAt     time.Time
}

type acceptedFlagSubmission struct {
	Flag           string
	SubmittingTeam int
	AttackerName   string
	VictimName     string
	ChallengeName  string
	SubmissionTick int
	// ExpiresTick is the last tick that may accept this flag. AcceptFlagSubmission
	// re-reads the live current tick under its lock/transaction and rejects the
	// flag when currentTick is 0 or greater than ExpiresTick.
	ExpiresTick int
	SubmittedAt time.Time
}

type schedulerEventRecord struct {
	EventType string
	Source    string
	State     string
	TickID    int
	Message   string
	CreatedAt time.Time
}

type checkerRunGroupKey struct {
	TickID      int
	TeamID      int
	ChallengeID int
}

type gameStore interface {
	ListCheckerTargets(ctx context.Context) ([]checkerTarget, error)
	IsChallengeInMaintenance(ctx context.Context, challengeID int) (bool, error)
	LookupTeamName(ctx context.Context, teamID int) (string, error)
	StartNextTick(ctx context.Context, now time.Time) (apigateway.GameTickStatus, error)
	RecordCheckerRun(ctx context.Context, run checkerRunRecord) (apigateway.GameCheckerRun, error)
	IssueFlag(ctx context.Context, flag issuedFlagRecord) error
	LookupIssuedFlag(ctx context.Context, flag string) (issuedFlagRecord, error)
	AcceptFlagSubmission(ctx context.Context, submission acceptedFlagSubmission) (bool, error)
	ListScoreboard(ctx context.Context) ([]apigateway.ScoreRowAlias, error)
	RecomputeScoreboard(ctx context.Context) ([]apigateway.ScoreRowAlias, error)
	AuditScoreboard(ctx context.Context) (apigateway.ScoringAuditAlias, error)
	ListAttackFeed(ctx context.Context, query apigateway.AttackFeedQuery) (apigateway.AttackFeedPage, error)
	MatchStatus(ctx context.Context) (apigateway.GameMatchStatus, error)
	StartMatch(ctx context.Context, now time.Time) (apigateway.GameMatchStatus, error)
	PauseMatch(ctx context.Context, now time.Time) (apigateway.GameMatchStatus, error)
	ResumeMatch(ctx context.Context, now time.Time) (apigateway.GameMatchStatus, error)
	StopMatch(ctx context.Context, now time.Time) (apigateway.GameMatchStatus, error)
	UpdateMatchSchedule(ctx context.Context, startAt, endAt *time.Time) (apigateway.GameMatchStatus, error)
	CompleteTick(ctx context.Context, tick apigateway.GameTickStatus) (apigateway.GameTickStatus, error)
	ReapRunningTicks(ctx context.Context) (int, error)
	GameStatus(ctx context.Context) (apigateway.GameStatus, error)
	ListCheckerRuns(ctx context.Context, query apigateway.GameCheckerRunQuery) (apigateway.GameCheckerRunPage, error)
	LoadSchedulerState(ctx context.Context, intervalSeconds int) (apigateway.GameSchedulerStatus, error)
	SaveSchedulerState(ctx context.Context, status apigateway.GameSchedulerStatus, now time.Time) error
	AppendSchedulerEvent(ctx context.Context, event schedulerEventRecord) (apigateway.GameSchedulerEvent, error)
	ListSchedulerEvents(ctx context.Context, query apigateway.GameSchedulerEventQuery) (apigateway.GameSchedulerEventPage, error)
	Close() error
}

type memoryGameStore struct {
	mu            sync.Mutex
	targets       []checkerTarget
	teamNames     map[int]string
	challenges    map[int]memoryChallenge
	ticks         []apigateway.GameTickStatus
	runs          []apigateway.GameCheckerRun
	serviceStates map[checkerRunGroupKey]apigateway.GameServiceStateSummary
	flags         map[string]issuedFlagRecord
	accepted      map[string]acceptedFlagSubmission
	attackFeed    []apigateway.AttackEventAlias
	scoreboard    []apigateway.ScoreRowAlias
	match         apigateway.GameMatchStatus
	scheduler     apigateway.GameSchedulerStatus
	events        []apigateway.GameSchedulerEvent
	nextRun       int64
	nextEvent     int64
}

type memoryChallenge struct {
	Name               string
	ServicePort        int
	ServiceSubnetOctet int
	Maintenance        bool
	PlayFromTick       int // 0 = no deferred gate
}

func newMemoryGameStore() gameStore {
	teams := []struct {
		id   int
		name string
	}{
		{101, "Team Alpha"},
		{102, "Team Delta"},
		{103, "Team Sigma"},
		{104, "Team Orchid"},
	}
	challenges := []struct {
		id           int
		name         string
		checkerImage string
		servicePort  int
		subnetOctet  int
	}{
		{1, "banking", "registry.local/banking-checker:latest", apigateway.DefaultServicePort(1), apigateway.DefaultServiceSubnetOctet(1)},
		{2, "chat", "registry.local/chat-checker:latest", apigateway.DefaultServicePort(2), apigateway.DefaultServiceSubnetOctet(2)},
		{3, "storage", "registry.local/storage-checker:latest", apigateway.DefaultServicePort(3), apigateway.DefaultServiceSubnetOctet(3)},
	}

	targets := make([]checkerTarget, 0, len(teams)*len(challenges))
	teamNames := make(map[int]string, len(teams))
	challengeMap := make(map[int]memoryChallenge, len(challenges))
	for _, team := range teams {
		teamNames[team.id] = team.name
	}
	for _, challenge := range challenges {
		challengeMap[challenge.id] = memoryChallenge{Name: challenge.name, ServicePort: challenge.servicePort, ServiceSubnetOctet: challenge.subnetOctet}
	}
	for _, team := range teams {
		for _, challenge := range challenges {
			targetHost := apigateway.ServiceIP(challenge.subnetOctet, team.id)
			targets = append(targets, checkerTarget{
				TeamID:        team.id,
				TeamName:      team.name,
				ChallengeID:   challenge.id,
				ChallengeName: challenge.name,
				CheckerImage:  challenge.checkerImage,
				CheckerToken:  fmt.Sprintf("dev-checker-token-%d-%d", team.id, challenge.id),
				Target:        apigateway.ServiceEndpointFor(challenge.subnetOctet, challenge.servicePort, team.id),
				TargetHost:    targetHost,
				TargetIP:      targetHost,
				TargetPort:    challenge.servicePort,
			})
		}
	}

	return &memoryGameStore{
		targets:       targets,
		teamNames:     teamNames,
		challenges:    challengeMap,
		ticks:         make([]apigateway.GameTickStatus, 0),
		runs:          make([]apigateway.GameCheckerRun, 0),
		serviceStates: make(map[checkerRunGroupKey]apigateway.GameServiceStateSummary),
		flags:         make(map[string]issuedFlagRecord),
		accepted:      make(map[string]acceptedFlagSubmission),
		attackFeed:    make([]apigateway.AttackEventAlias, 0),
		scoreboard:    make([]apigateway.ScoreRowAlias, 0),
		match: apigateway.GameMatchStatus{
			State:                "not_started",
			AcceptingSubmissions: false,
		},
		scheduler: apigateway.GameSchedulerStatus{State: "stopped", IntervalSeconds: 60},
		events:    make([]apigateway.GameSchedulerEvent, 0),
		nextRun:   1,
		nextEvent: 1,
	}
}

func (s *memoryGameStore) ListCheckerTargets(_ context.Context) ([]checkerTarget, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	currentTick := 0
	for _, tick := range s.ticks {
		if tick.ID > currentTick {
			currentTick = tick.ID
		}
	}

	result := make([]checkerTarget, 0, len(s.targets))
	for _, target := range s.targets {
		if challenge, ok := s.challenges[target.ChallengeID]; ok {
			if challenge.Maintenance {
				continue
			}
			if challenge.PlayFromTick > 0 && challenge.PlayFromTick > currentTick {
				continue
			}
		}
		result = append(result, target)
	}
	slices.SortFunc(result, func(a, b checkerTarget) int {
		if a.TeamID != b.TeamID {
			return a.TeamID - b.TeamID
		}
		return a.ChallengeID - b.ChallengeID
	})
	return result, nil
}

func (s *memoryGameStore) IsChallengeInMaintenance(_ context.Context, challengeID int) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	challenge, ok := s.challenges[challengeID]
	if !ok {
		return false, nil
	}
	if challenge.Maintenance {
		return true, nil
	}
	if challenge.PlayFromTick > 0 {
		currentTick := 0
		for _, tick := range s.ticks {
			if tick.ID > currentTick {
				currentTick = tick.ID
			}
		}
		if challenge.PlayFromTick > currentTick {
			return true, nil
		}
	}
	return false, nil
}

// setChallengeMaintenanceForTest is used by unit tests to toggle maintenance on
// the in-memory game store without a full admin plane.
func (s *memoryGameStore) setChallengeMaintenanceForTest(challengeID int, maintenance bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	challenge, ok := s.challenges[challengeID]
	if !ok {
		return
	}
	challenge.Maintenance = maintenance
	if maintenance {
		challenge.PlayFromTick = 0
	}
	s.challenges[challengeID] = challenge
}

func (s *memoryGameStore) setChallengePlayFromTickForTest(challengeID, tick int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	challenge, ok := s.challenges[challengeID]
	if !ok {
		return
	}
	challenge.PlayFromTick = tick
	s.challenges[challengeID] = challenge
}

func (s *memoryGameStore) LookupTeamName(_ context.Context, teamID int) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, target := range s.targets {
		if target.TeamID == teamID {
			return target.TeamName, nil
		}
	}
	return "", sql.ErrNoRows
}

func (s *memoryGameStore) StartNextTick(_ context.Context, now time.Time) (apigateway.GameTickStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.ticks) > 0 && s.ticks[len(s.ticks)-1].Status == "running" {
		return apigateway.GameTickStatus{}, errTickInProgress
	}

	tick := apigateway.GameTickStatus{
		ID:        len(s.ticks) + 1,
		Status:    "running",
		StartedAt: now.UTC().Format(time.RFC3339),
	}
	s.ticks = append(s.ticks, tick)
	return tick, nil
}

func (s *memoryGameStore) RecordCheckerRun(_ context.Context, run checkerRunRecord) (apigateway.GameCheckerRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	record := apigateway.GameCheckerRun{
		ID:            s.nextRun,
		TickID:        run.TickID,
		TeamID:        run.TeamID,
		TeamName:      run.TeamName,
		ChallengeID:   run.ChallengeID,
		ChallengeName: run.ChallengeName,
		Phase:         run.Phase,
		Target:        run.Target,
		CheckerImage:  run.CheckerImage,
		Status:        run.Status,
		ExitCode:      run.ExitCode,
		Message:       run.Message,
		Output:        run.Output,
		CheckedAt:     run.CheckedAt.UTC().Format(time.RFC3339),
	}
	s.nextRun++
	s.runs = append([]apigateway.GameCheckerRun{record}, s.runs...)
	s.refreshMemoryServiceState(run)
	return record, nil
}

func (s *memoryGameStore) IssueFlag(_ context.Context, flag issuedFlagRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.flags[flag.Flag] = flag
	return nil
}

func (s *memoryGameStore) LookupIssuedFlag(_ context.Context, flag string) (issuedFlagRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.flags[flag]
	if !ok {
		return issuedFlagRecord{}, sql.ErrNoRows
	}
	return record, nil
}

func (s *memoryGameStore) AcceptFlagSubmission(_ context.Context, submission acceptedFlagSubmission) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Re-check match + tick under the same lock that mutates accepted flags so
	// a concurrent pause/stop/tick advance cannot race a successful insert.
	if s.match.State != "running" || !s.match.AcceptingSubmissions {
		return false, errFlagNoLongerValid
	}
	currentTick := 0
	if len(s.ticks) > 0 {
		currentTick = s.ticks[len(s.ticks)-1].ID
	}
	if currentTick == 0 || currentTick > submission.ExpiresTick {
		return false, errFlagNoLongerValid
	}
	submission.SubmissionTick = currentTick

	key := acceptedSubmissionKey(submission.Flag, submission.SubmittingTeam)
	if _, ok := s.accepted[key]; ok {
		return false, nil
	}
	s.accepted[key] = submission
	s.attackFeed = append([]apigateway.AttackEventAlias{{
		ID:        acceptedFlagAttackEventID(submission),
		Attacker:  submission.AttackerName,
		Victim:    submission.VictimName,
		Service:   submission.ChallengeName,
		Tick:      submission.SubmissionTick,
		Verdict:   "first valid submission accepted",
		CreatedAt: submission.SubmittedAt.UTC(),
	}}, s.attackFeed...)
	s.recomputeScoreboardLocked()
	return true, nil
}

func (s *memoryGameStore) ListScoreboard(ctx context.Context) ([]apigateway.ScoreRowAlias, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]apigateway.ScoreRowAlias(nil), s.scoreboard...), nil
}

func (s *memoryGameStore) RecomputeScoreboard(_ context.Context) ([]apigateway.ScoreRowAlias, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rows := s.recomputeScoreboardLocked()
	return append([]apigateway.ScoreRowAlias(nil), rows...), nil
}

func (s *memoryGameStore) recomputeScoreboardLocked() []apigateway.ScoreRowAlias {
	previousRanks := make(map[string]int, len(s.scoreboard))
	for _, row := range s.scoreboard {
		previousRanks[row.Team] = row.Rank
	}

	type teamScore struct {
		Name     string
		Attack   float64
		Defense  float64
		SLA      float64
		Services map[int]*apigateway.ServiceScoreBreakdownAlias
	}

	challengeIDs := sortedChallengeIDs(s.challenges)
	scores := make(map[int]*teamScore, len(s.teamNames))
	for teamID, teamName := range s.teamNames {
		services := make(map[int]*apigateway.ServiceScoreBreakdownAlias, len(challengeIDs))
		for _, challengeID := range challengeIDs {
			challenge := s.challenges[challengeID]
			services[challengeID] = &apigateway.ServiceScoreBreakdownAlias{
				ChallengeID: challengeID,
				Service:     challenge.Name,
			}
		}
		scores[teamID] = &teamScore{
			Name:     teamName,
			Services: services,
		}
	}

	acceptedByFlag := make(map[string][]acceptedFlagSubmission)
	for _, submission := range s.accepted {
		acceptedByFlag[submission.Flag] = append(acceptedByFlag[submission.Flag], submission)
	}
	for flagValue, submissions := range acceptedByFlag {
		sortAcceptedSubmissions(submissions)
		flag := s.flags[flagValue]
		captureCount := len(submissions)
		for _, submission := range submissions {
			value := faustAttackValue(captureCount)
			scores[submission.SubmittingTeam].Attack += value
			scores[submission.SubmittingTeam].Services[flag.ChallengeID].Attack += value
		}
	}

	for _, flag := range s.flags {
		captureCount := len(acceptedByFlag[flag.Flag])
		if captureCount == 0 {
			continue
		}
		penalty := faustDefensePenalty(captureCount)
		scores[flag.OwnerTeamID].Defense -= penalty
		scores[flag.OwnerTeamID].Services[flag.ChallengeID].Defense -= penalty
	}

	for key, summary := range s.serviceStates {
		teamID := key.TeamID
		challengeID := key.ChallengeID
		value := faustSLAValueForStatus(summary.Status)
		scores[teamID].SLA += value
		scores[teamID].Services[challengeID].SLA += value
	}
	slaFactor := faustSLAFactor(len(s.teamNames))
	for _, score := range scores {
		score.SLA *= slaFactor
		for _, service := range score.Services {
			service.SLA *= slaFactor
		}
	}

	rows := make([]apigateway.ScoreRowAlias, 0, len(scores))
	for _, score := range scores {
		services := make([]apigateway.ServiceScoreBreakdownAlias, 0, len(challengeIDs))
		for _, challengeID := range challengeIDs {
			service := *score.Services[challengeID]
			service.Total = service.Attack + service.Defense + service.SLA
			services = append(services, service)
		}
		rows = append(rows, apigateway.ScoreRowAlias{
			Rank:     0,
			Team:     score.Name,
			Attack:   score.Attack,
			Defense:  score.Defense,
			SLA:      score.SLA,
			Total:    score.Attack + score.Defense + score.SLA,
			Services: services,
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Total != rows[j].Total {
			return rows[i].Total > rows[j].Total
		}
		if rows[i].Attack != rows[j].Attack {
			return rows[i].Attack > rows[j].Attack
		}
		if rows[i].Defense != rows[j].Defense {
			return rows[i].Defense > rows[j].Defense
		}
		return rows[i].Team < rows[j].Team
	})

	for index := range rows {
		rows[index].Rank = index + 1
		rows[index].Delta = rankDelta(previousRanks[rows[index].Team], rows[index].Rank)
	}

	s.scoreboard = append([]apigateway.ScoreRowAlias(nil), rows...)
	return rows
}

func (s *memoryGameStore) AuditScoreboard(ctx context.Context) (apigateway.ScoringAuditAlias, error) {
	s.mu.Lock()
	stored := append([]apigateway.ScoreRowAlias(nil), s.scoreboard...)
	s.mu.Unlock()

	replayed, err := s.RecomputeScoreboard(ctx)
	if err != nil {
		return apigateway.ScoringAuditAlias{}, err
	}

	s.mu.Lock()
	s.scoreboard = append([]apigateway.ScoreRowAlias(nil), stored...)
	s.mu.Unlock()

	return buildScoringAuditReport(stored, replayed), nil
}

func (s *memoryGameStore) ListAttackFeed(_ context.Context, query apigateway.AttackFeedQuery) (apigateway.AttackFeedPage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	filtered := filterAttackFeedRows(s.attackFeed, query)
	limit := query.Limit
	if limit <= 0 {
		limit = 12
	}
	offset := query.Offset
	if offset < 0 {
		offset = 0
	}
	page := apigateway.AttackFeedPage{
		Limit:      limit,
		Offset:     offset,
		TotalCount: len(filtered),
		HasPrev:    offset > 0,
	}
	if offset >= len(filtered) {
		page.Items = []apigateway.AttackEventAlias{}
		return page, nil
	}
	if limit > len(filtered)-offset {
		limit = len(filtered) - offset
	}
	page.HasNext = offset+limit < len(filtered)
	page.Items = append([]apigateway.AttackEventAlias(nil), filtered[offset:offset+limit]...)
	return page, nil
}

func (s *memoryGameStore) MatchStatus(_ context.Context) (apigateway.GameMatchStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.match, nil
}

func (s *memoryGameStore) StartMatch(_ context.Context, now time.Time) (apigateway.GameMatchStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch s.match.State {
	case "running":
		return s.match, nil
	case "finished":
		return apigateway.GameMatchStatus{}, errContestOver
	default:
		current := s.match
		s.match = apigateway.GameMatchStatus{
			State:                "running",
			StartedAt:            now.UTC().Format(time.RFC3339),
			ScheduledStartAt:     current.ScheduledStartAt,
			ScheduledEndAt:       current.ScheduledEndAt,
			ScheduleConfigured:   current.ScheduleConfigured,
			AcceptingSubmissions: true,
		}
		return s.match, nil
	}
}

func (s *memoryGameStore) PauseMatch(_ context.Context, now time.Time) (apigateway.GameMatchStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.match.State == "finished" {
		return apigateway.GameMatchStatus{}, errContestOver
	}
	if s.match.State == "not_started" {
		return apigateway.GameMatchStatus{}, errContestNotStarted
	}
	s.match.State = "paused"
	s.match.AcceptingSubmissions = false
	return s.match, nil
}

func (s *memoryGameStore) ResumeMatch(_ context.Context, now time.Time) (apigateway.GameMatchStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.match.State == "finished" {
		return apigateway.GameMatchStatus{}, errContestOver
	}
	if s.match.State != "paused" {
		return s.match, nil
	}
	s.match.State = "running"
	s.match.AcceptingSubmissions = true
	return s.match, nil
}

func (s *memoryGameStore) StopMatch(_ context.Context, now time.Time) (apigateway.GameMatchStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.match.State == "finished" {
		return s.match, nil
	}
	s.match.State = "finished"
	s.match.EndedAt = now.UTC().Format(time.RFC3339)
	s.match.AcceptingSubmissions = false
	return s.match, nil
}

func (s *memoryGameStore) UpdateMatchSchedule(_ context.Context, startAt, endAt *time.Time) (apigateway.GameMatchStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if startAt == nil {
		s.match.ScheduledStartAt = ""
	} else {
		s.match.ScheduledStartAt = startAt.UTC().Format(time.RFC3339)
	}
	if endAt == nil {
		s.match.ScheduledEndAt = ""
	} else {
		s.match.ScheduledEndAt = endAt.UTC().Format(time.RFC3339)
	}
	s.match.ScheduleConfigured = true
	return s.match, nil
}

func (s *memoryGameStore) CompleteTick(_ context.Context, tick apigateway.GameTickStatus) (apigateway.GameTickStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for index := range s.ticks {
		if s.ticks[index].ID != tick.ID {
			continue
		}
		s.ticks[index] = tick
		return tick, nil
	}
	return apigateway.GameTickStatus{}, fmt.Errorf("tick %d was not found", tick.ID)
}

func (s *memoryGameStore) ReapRunningTicks(_ context.Context) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	reaped := 0
	for index := range s.ticks {
		if s.ticks[index].Status != "running" {
			continue
		}
		s.ticks[index].Status = "failed"
		s.ticks[index].CompletedAt = time.Now().UTC().Format(time.RFC3339)
		s.ticks[index].Message = reapedTickMessage
		reaped++
	}
	return reaped, nil
}

func (s *memoryGameStore) GameStatus(_ context.Context) (apigateway.GameStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	status := apigateway.GameStatus{TotalTicks: len(s.ticks)}
	match := s.match
	status.Match = &match
	if len(s.ticks) > 0 {
		current := s.ticks[len(s.ticks)-1]
		status.CurrentTick = &current
	}
	for _, tick := range s.ticks {
		status.TotalCheckerRuns += tick.TotalCheckerRuns
		status.SuccessfulCheckerRuns += tick.SuccessfulCheckerRuns
		status.FailedCheckerRuns += tick.FailedCheckerRuns
		status.SkippedCheckerRuns += tick.SkippedCheckerRuns
	}
	return status, nil
}

func (s *memoryGameStore) ListCheckerRuns(_ context.Context, query apigateway.GameCheckerRunQuery) (apigateway.GameCheckerRunPage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	filtered := make([]apigateway.GameCheckerRun, 0, len(s.runs))
	for _, run := range s.runs {
		if query.TickID > 0 && run.TickID != query.TickID {
			continue
		}
		if query.TeamID > 0 && run.TeamID != query.TeamID {
			continue
		}
		if query.ChallengeID > 0 && run.ChallengeID != query.ChallengeID {
			continue
		}
		if query.Phase != "" && !strings.EqualFold(run.Phase, query.Phase) {
			continue
		}
		if query.Status != "" && !strings.EqualFold(run.Status, query.Status) {
			continue
		}
		if summary, ok := s.serviceStates[checkerRunKey(run)]; ok {
			applyCheckerRunSummary(&run, summary)
		}
		filtered = append(filtered, run)
	}

	offset := query.Offset
	if offset < 0 {
		offset = 0
	}
	totalCount := len(filtered)
	page := apigateway.GameCheckerRunPage{
		Limit:      query.Limit,
		Offset:     offset,
		TotalCount: totalCount,
		HasPrev:    offset > 0,
	}
	if page.Limit <= 0 {
		page.Limit = 25
	}
	if offset >= len(filtered) {
		page.Items = []apigateway.GameCheckerRun{}
		return page, nil
	}

	limit := page.Limit
	if limit > len(filtered)-offset {
		limit = len(filtered) - offset
	}
	page.HasNext = offset+limit < totalCount
	page.Items = append([]apigateway.GameCheckerRun(nil), filtered[offset:offset+limit]...)
	return page, nil
}

func (s *memoryGameStore) LoadSchedulerState(_ context.Context, intervalSeconds int) (apigateway.GameSchedulerStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	status := s.scheduler
	if intervalSeconds > 0 {
		status.IntervalSeconds = intervalSeconds
	}
	return status, nil
}

func (s *memoryGameStore) SaveSchedulerState(_ context.Context, status apigateway.GameSchedulerStatus, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scheduler = status
	return nil
}

func (s *memoryGameStore) AppendSchedulerEvent(_ context.Context, event schedulerEventRecord) (apigateway.GameSchedulerEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	record := apigateway.GameSchedulerEvent{
		ID:        s.nextEvent,
		EventType: event.EventType,
		Source:    event.Source,
		State:     event.State,
		TickID:    event.TickID,
		Message:   event.Message,
		CreatedAt: event.CreatedAt.UTC().Format(time.RFC3339),
	}
	s.nextEvent++
	s.events = append([]apigateway.GameSchedulerEvent{record}, s.events...)
	return record, nil
}

func (s *memoryGameStore) ListSchedulerEvents(_ context.Context, query apigateway.GameSchedulerEventQuery) (apigateway.GameSchedulerEventPage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	filtered := make([]apigateway.GameSchedulerEvent, 0, len(s.events))
	for _, event := range s.events {
		if query.EventType != "" && !strings.EqualFold(event.EventType, query.EventType) {
			continue
		}
		if query.Source != "" && !strings.EqualFold(event.Source, query.Source) {
			continue
		}
		if query.State != "" && !strings.EqualFold(event.State, query.State) {
			continue
		}
		filtered = append(filtered, event)
	}

	offset := query.Offset
	if offset < 0 {
		offset = 0
	}
	totalCount := len(filtered)
	page := apigateway.GameSchedulerEventPage{
		Limit:      query.Limit,
		Offset:     offset,
		TotalCount: totalCount,
		HasPrev:    offset > 0,
	}
	if page.Limit <= 0 {
		page.Limit = 25
	}
	if offset >= len(filtered) {
		page.Items = []apigateway.GameSchedulerEvent{}
		return page, nil
	}

	limit := page.Limit
	if limit > len(filtered)-offset {
		limit = len(filtered) - offset
	}
	page.HasNext = offset+limit < totalCount
	page.Items = append([]apigateway.GameSchedulerEvent(nil), filtered[offset:offset+limit]...)
	return page, nil
}

func (s *memoryGameStore) Close() error {
	return nil
}

type postgresGameStore struct {
	db *sql.DB
}

func newPostgresGameStore(db *sql.DB) gameStore {
	return &postgresGameStore{db: db}
}

func (s *postgresGameStore) ListCheckerTargets(ctx context.Context) ([]checkerTarget, error) {
	// play_from_tick gates post-maintenance resume: challenge only re-enters the
	// checker once the current max tick id has reached that value (set to next
	// tick on resume). StartNextTick runs before this query, so the new tick id
	// is already visible when the tick that should include the challenge starts.
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.id, t.name, c.id, c.name, c.checker_image, COALESCE(si.checker_token, ''), tss.endpoint, split_part(tss.endpoint, ':', 1), split_part(tss.endpoint, ':', 2)
		FROM teams t
		JOIN team_service_states tss ON tss.team_id = t.id
		JOIN challenges c ON c.id = tss.challenge_id
		LEFT JOIN service_instances si ON si.team_id = t.id AND si.challenge_id = c.id
		WHERE c.published = TRUE
		  AND c.maintenance = FALSE
		  AND (c.play_from_tick IS NULL OR c.play_from_tick <= COALESCE((SELECT MAX(id) FROM game_ticks), 0))
		  AND t.active = TRUE
		  AND (t.play_from_tick IS NULL OR t.play_from_tick <= COALESCE((SELECT MAX(id) FROM game_ticks), 0))
		  AND (si.runtime_status IS NULL OR si.runtime_status = 'ready')
		ORDER BY t.id, c.id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]checkerTarget, 0)
	for rows.Next() {
		var item checkerTarget
		var targetPortText string
		if err := rows.Scan(&item.TeamID, &item.TeamName, &item.ChallengeID, &item.ChallengeName, &item.CheckerImage, &item.CheckerToken, &item.Target, &item.TargetHost, &targetPortText); err != nil {
			return nil, err
		}
		item.TargetIP = item.TargetHost
		item.TargetPort, _ = strconv.Atoi(targetPortText)
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *postgresGameStore) IsChallengeInMaintenance(ctx context.Context, challengeID int) (bool, error) {
	// True when the challenge must not accept scoring (maintenance or deferred resume).
	var blocked bool
	err := s.db.QueryRowContext(ctx, `
		SELECT c.maintenance
		    OR (c.play_from_tick IS NOT NULL AND c.play_from_tick > COALESCE((SELECT MAX(id) FROM game_ticks), 0))
		FROM challenges c
		WHERE c.id = $1
	`, challengeID).Scan(&blocked)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return blocked, nil
}

func (s *postgresGameStore) LookupTeamName(ctx context.Context, teamID int) (string, error) {
	var name string
	if err := s.db.QueryRowContext(ctx, `SELECT name FROM teams WHERE id = $1`, teamID).Scan(&name); err != nil {
		return "", err
	}
	return name, nil
}

func (s *postgresGameStore) StartNextTick(ctx context.Context, now time.Time) (apigateway.GameTickStatus, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return apigateway.GameTickStatus{}, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `LOCK TABLE game_ticks IN EXCLUSIVE MODE`); err != nil {
		return apigateway.GameTickStatus{}, err
	}

	var lastStatus sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT status FROM game_ticks ORDER BY id DESC LIMIT 1`).Scan(&lastStatus); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return apigateway.GameTickStatus{}, err
	}
	if lastStatus.Valid && lastStatus.String == "running" {
		return apigateway.GameTickStatus{}, errTickInProgress
	}

	var nextID int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(id), 0) + 1 FROM game_ticks`).Scan(&nextID); err != nil {
		return apigateway.GameTickStatus{}, err
	}

	tick := apigateway.GameTickStatus{
		ID:        nextID,
		Status:    "running",
		StartedAt: now.UTC().Format(time.RFC3339),
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO game_ticks (id, status, started_at, message)
		VALUES ($1, $2, $3, '')
	`, tick.ID, tick.Status, now.UTC()); err != nil {
		return apigateway.GameTickStatus{}, err
	}

	if err := tx.Commit(); err != nil {
		return apigateway.GameTickStatus{}, err
	}
	return tick, nil
}

func (s *postgresGameStore) RecordCheckerRun(ctx context.Context, run checkerRunRecord) (apigateway.GameCheckerRun, error) {
	record := apigateway.GameCheckerRun{
		TickID:        run.TickID,
		TeamID:        run.TeamID,
		TeamName:      run.TeamName,
		ChallengeID:   run.ChallengeID,
		ChallengeName: run.ChallengeName,
		Phase:         run.Phase,
		Target:        run.Target,
		CheckerImage:  run.CheckerImage,
		Status:        run.Status,
		ExitCode:      run.ExitCode,
		Message:       run.Message,
		Output:        run.Output,
		CheckedAt:     run.CheckedAt.UTC().Format(time.RFC3339),
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return apigateway.GameCheckerRun{}, err
	}
	defer tx.Rollback()
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO checker_runs (
			tick_id, team_id, team_name, challenge_id, challenge_name, phase, target, checker_image, status, exit_code, message, output, checked_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id
	`, run.TickID, run.TeamID, run.TeamName, run.ChallengeID, run.ChallengeName, run.Phase, run.Target, run.CheckerImage, run.Status, run.ExitCode, run.Message, run.Output, run.CheckedAt.UTC()).Scan(&record.ID); err != nil {
		return apigateway.GameCheckerRun{}, err
	}
	if err := refreshPostgresServiceStateTx(ctx, tx, run); err != nil {
		return apigateway.GameCheckerRun{}, err
	}
	if err := tx.Commit(); err != nil {
		return apigateway.GameCheckerRun{}, err
	}
	return record, nil
}

func (s *postgresGameStore) IssueFlag(ctx context.Context, flag issuedFlagRecord) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO issued_flags (
			flag, owner_team_id, owner_team_name, challenge_id, challenge_name, issued_tick, expires_tick, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (flag) DO NOTHING
	`, flag.Flag, flag.OwnerTeamID, flag.OwnerTeamName, flag.ChallengeID, flag.ChallengeName, flag.IssuedTick, flag.ExpiresTick, flag.CreatedAt.UTC())
	return err
}

func (s *postgresGameStore) LookupIssuedFlag(ctx context.Context, flag string) (issuedFlagRecord, error) {
	var record issuedFlagRecord
	var createdAt time.Time
	err := s.db.QueryRowContext(ctx, `
		SELECT flag, owner_team_id, owner_team_name, challenge_id, challenge_name, issued_tick, expires_tick, created_at
		FROM issued_flags
		WHERE flag = $1
	`, flag).Scan(&record.Flag, &record.OwnerTeamID, &record.OwnerTeamName, &record.ChallengeID, &record.ChallengeName, &record.IssuedTick, &record.ExpiresTick, &createdAt)
	if err != nil {
		return issuedFlagRecord{}, err
	}
	record.CreatedAt = createdAt.UTC()
	return record, nil
}

func (s *postgresGameStore) AcceptFlagSubmission(ctx context.Context, submission acceptedFlagSubmission) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	// Same exclusive tick-table lock StartNextTick takes, so accept-time tick
	// revalidation cannot race a concurrent tick advance.
	if _, err := tx.ExecContext(ctx, `LOCK TABLE game_ticks IN EXCLUSIVE MODE`); err != nil {
		return false, err
	}

	// Serialize against match pause/stop/finish.
	var matchState string
	err = tx.QueryRowContext(ctx, `
		SELECT state
		FROM game_match_state
		WHERE singleton = TRUE
		FOR UPDATE
	`).Scan(&matchState)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return false, errFlagNoLongerValid
	case err != nil:
		return false, err
	}
	if matchState != "running" {
		return false, errFlagNoLongerValid
	}

	var currentTick int
	err = tx.QueryRowContext(ctx, `
		SELECT COALESCE((SELECT id FROM game_ticks ORDER BY id DESC LIMIT 1), 0)
	`).Scan(&currentTick)
	if err != nil {
		return false, err
	}
	if currentTick == 0 || currentTick > submission.ExpiresTick {
		return false, errFlagNoLongerValid
	}
	submission.SubmissionTick = currentTick

	var inserted string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO submitted_flags (flag, team_id, submitted_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (flag, team_id) DO NOTHING
		RETURNING flag
	`, submission.Flag, submission.SubmittingTeam, submission.SubmittedAt.UTC()).Scan(&inserted)
	switch {
	case err == nil:
	case errors.Is(err, sql.ErrNoRows):
		return false, nil
	default:
		return false, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO attack_events (id, attacker, victim, service, tick, verdict, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, acceptedFlagAttackEventID(submission), submission.AttackerName, submission.VictimName, submission.ChallengeName, submission.SubmissionTick, "first valid submission accepted", submission.SubmittedAt.UTC()); err != nil {
		return false, err
	}

	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (s *postgresGameStore) CompleteTick(ctx context.Context, tick apigateway.GameTickStatus) (apigateway.GameTickStatus, error) {
	var completedAt any
	if tick.CompletedAt != "" {
		parsed, err := time.Parse(time.RFC3339, tick.CompletedAt)
		if err != nil {
			return apigateway.GameTickStatus{}, err
		}
		completedAt = parsed.UTC()
	}
	if _, err := s.db.ExecContext(ctx, `
		UPDATE game_ticks
		SET status = $2,
		    total_checker_runs = $3,
		    successful_checker_runs = $4,
		    failed_checker_runs = $5,
		    skipped_checker_runs = $6,
		    completed_at = $7,
		    message = $8
		WHERE id = $1
	`, tick.ID, tick.Status, tick.TotalCheckerRuns, tick.SuccessfulCheckerRuns, tick.FailedCheckerRuns, tick.SkippedCheckerRuns, completedAt, tick.Message); err != nil {
		return apigateway.GameTickStatus{}, err
	}
	return tick, nil
}

func (s *postgresGameStore) ReapRunningTicks(ctx context.Context) (int, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE game_ticks
		SET status = 'failed',
		    completed_at = NOW(),
		    message = $1
		WHERE status = 'running'
	`, reapedTickMessage)
	if err != nil {
		return 0, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(affected), nil
}

func (s *postgresGameStore) GameStatus(ctx context.Context) (apigateway.GameStatus, error) {
	status := apigateway.GameStatus{}
	match, err := s.MatchStatus(ctx)
	if err != nil {
		return apigateway.GameStatus{}, err
	}
	status.Match = &match

	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(total_checker_runs), 0), COALESCE(SUM(successful_checker_runs), 0), COALESCE(SUM(failed_checker_runs), 0), COALESCE(SUM(skipped_checker_runs), 0)
		FROM game_ticks
	`).Scan(&status.TotalTicks, &status.TotalCheckerRuns, &status.SuccessfulCheckerRuns, &status.FailedCheckerRuns, &status.SkippedCheckerRuns); err != nil {
		return apigateway.GameStatus{}, err
	}

	var current apigateway.GameTickStatus
	var startedAt time.Time
	var completedAt sql.NullTime
	var message sql.NullString
	err = s.db.QueryRowContext(ctx, `
		SELECT id, status, total_checker_runs, successful_checker_runs, failed_checker_runs, skipped_checker_runs, started_at, completed_at, message
		FROM game_ticks
		ORDER BY id DESC
		LIMIT 1
	`).Scan(&current.ID, &current.Status, &current.TotalCheckerRuns, &current.SuccessfulCheckerRuns, &current.FailedCheckerRuns, &current.SkippedCheckerRuns, &startedAt, &completedAt, &message)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return status, nil
	case err != nil:
		return apigateway.GameStatus{}, err
	}
	current.StartedAt = startedAt.UTC().Format(time.RFC3339)
	if completedAt.Valid {
		current.CompletedAt = completedAt.Time.UTC().Format(time.RFC3339)
	}
	if message.Valid {
		current.Message = message.String
	}
	status.CurrentTick = &current
	return status, nil
}

func (s *postgresGameStore) ListCheckerRuns(ctx context.Context, query apigateway.GameCheckerRunQuery) (apigateway.GameCheckerRunPage, error) {
	limit := query.Limit
	if limit <= 0 {
		limit = 25
	}
	offset := query.Offset
	if offset < 0 {
		offset = 0
	}

	whereClause, args := buildCheckerRunFilterQuery(query)

	var totalCount int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM checker_runs
	`+whereClause, args...).Scan(&totalCount); err != nil {
		return apigateway.GameCheckerRunPage{}, err
	}

	statement := strings.Builder{}
	statement.WriteString(`
		SELECT id, tick_id, team_id, team_name, challenge_id, challenge_name, phase, target, checker_image, status, exit_code, message, output, checked_at
		FROM checker_runs
	`)
	statement.WriteString(whereClause)
	statement.WriteString(fmt.Sprintf(" ORDER BY tick_id DESC, id DESC LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2))
	queryArgs := append(append([]any(nil), args...), limit, offset)

	rows, err := s.db.QueryContext(ctx, statement.String(), queryArgs...)
	if err != nil {
		return apigateway.GameCheckerRunPage{}, err
	}
	defer rows.Close()

	result := make([]apigateway.GameCheckerRun, 0)
	for rows.Next() {
		var item apigateway.GameCheckerRun
		var checkedAt time.Time
		if err := rows.Scan(&item.ID, &item.TickID, &item.TeamID, &item.TeamName, &item.ChallengeID, &item.ChallengeName, &item.Phase, &item.Target, &item.CheckerImage, &item.Status, &item.ExitCode, &item.Message, &item.Output, &checkedAt); err != nil {
			return apigateway.GameCheckerRunPage{}, err
		}
		item.CheckedAt = checkedAt.UTC().Format(time.RFC3339)
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return apigateway.GameCheckerRunPage{}, err
	}
	summaries, err := s.loadCheckerRunSummaries(ctx, result)
	if err != nil {
		return apigateway.GameCheckerRunPage{}, err
	}
	for i := range result {
		applyCheckerRunSummary(&result[i], summaries[checkerRunKey(result[i])])
	}
	return apigateway.GameCheckerRunPage{
		Items:      result,
		Limit:      limit,
		Offset:     offset,
		TotalCount: totalCount,
		HasPrev:    offset > 0,
		HasNext:    offset+len(result) < totalCount,
	}, nil
}

func checkerRunKey(run apigateway.GameCheckerRun) checkerRunGroupKey {
	return checkerRunGroupKey{
		TickID:      run.TickID,
		TeamID:      run.TeamID,
		ChallengeID: run.ChallengeID,
	}
}

func checkerRunKeyFromIDs(tickID, teamID, challengeID int) checkerRunGroupKey {
	return checkerRunGroupKey{
		TickID:      tickID,
		TeamID:      teamID,
		ChallengeID: challengeID,
	}
}

func applyCheckerRunSummary(run *apigateway.GameCheckerRun, summary apigateway.GameServiceStateSummary) {
	run.ServiceState = summary.Status
	run.StatePhase = summary.Phase
	run.StateMessage = summary.Message
}

func (s *memoryGameStore) refreshMemoryServiceState(run checkerRunRecord) {
	tickID := run.TickID
	teamID := run.TeamID
	challengeID := run.ChallengeID
	key := checkerRunKeyFromIDs(tickID, teamID, challengeID)

	if existing, ok := s.serviceStates[key]; ok && run.Status == "skipped" && strings.TrimSpace(run.ReportedServiceState) == "" {
		s.serviceStates[key] = existing
		return
	}

	reportedState := run.ReportedServiceState
	reportedMessage := run.ReportedStateMessage
	if normalized := apigateway.NormalizeServiceStateStatus(reportedState); normalized != "" {
		s.serviceStates[key] = apigateway.GameServiceStateSummary{
			Status:  normalized,
			Phase:   inferReportedStatePhase(normalized),
			TickID:  tickID,
			Message: fallbackReportedStateMessage(normalized, reportedMessage),
		}
		return
	}

	group := make([]apigateway.GameCheckerRun, 0, 3)
	for _, run := range s.runs {
		if run.TickID != tickID || run.TeamID != teamID || run.ChallengeID != challengeID {
			continue
		}
		group = append(group, run)
	}
	s.serviceStates[key] = apigateway.SummarizeCheckerRunsForTick(group, tickID)
}

func (s *postgresGameStore) loadCheckerRunSummaries(ctx context.Context, runs []apigateway.GameCheckerRun) (map[checkerRunGroupKey]apigateway.GameServiceStateSummary, error) {
	if len(runs) == 0 {
		return map[checkerRunGroupKey]apigateway.GameServiceStateSummary{}, nil
	}

	keys := make([]checkerRunGroupKey, 0, len(runs))
	seen := make(map[checkerRunGroupKey]struct{}, len(runs))
	for _, run := range runs {
		key := checkerRunKey(run)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}

	args := make([]any, 0, len(keys)*3)
	clauses := make([]string, 0, len(keys))
	for _, key := range keys {
		args = append(args, key.TickID, key.TeamID, key.ChallengeID)
		base := len(args) - 2
		clauses = append(clauses, fmt.Sprintf("(tick_id = $%d AND team_id = $%d AND challenge_id = $%d)", base, base+1, base+2))
	}

	// #nosec G202 -- clauses are generated from fixed column predicates and positional placeholders only.
	query := `
		SELECT tick_id, team_id, challenge_id, service_state, state_phase, state_message
		FROM checker_service_states
		WHERE ` + strings.Join(clauses, " OR ") + `
	`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	summaries := make(map[checkerRunGroupKey]apigateway.GameServiceStateSummary, len(keys))
	for rows.Next() {
		var tickID, teamID, challengeID int
		var summary apigateway.GameServiceStateSummary
		if err := rows.Scan(&tickID, &teamID, &challengeID, &summary.Status, &summary.Phase, &summary.Message); err != nil {
			return nil, err
		}
		key := checkerRunGroupKey{TickID: tickID, TeamID: teamID, ChallengeID: challengeID}
		summary.TickID = tickID
		summaries[key] = summary
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	missing := make([]checkerRunGroupKey, 0)
	for _, key := range keys {
		if _, ok := summaries[key]; !ok {
			missing = append(missing, key)
		}
	}
	if len(missing) == 0 {
		return summaries, nil
	}

	fallbacks, err := s.loadCheckerRunSummariesFromRuns(ctx, missing)
	if err != nil {
		return nil, err
	}
	for key, summary := range fallbacks {
		summaries[key] = summary
	}
	return summaries, nil
}

func (s *postgresGameStore) loadCheckerRunSummariesFromRuns(ctx context.Context, keys []checkerRunGroupKey) (map[checkerRunGroupKey]apigateway.GameServiceStateSummary, error) {
	args := make([]any, 0, len(keys)*3)
	clauses := make([]string, 0, len(keys))
	for _, key := range keys {
		args = append(args, key.TickID, key.TeamID, key.ChallengeID)
		base := len(args) - 2
		clauses = append(clauses, fmt.Sprintf("(tick_id = $%d AND team_id = $%d AND challenge_id = $%d)", base, base+1, base+2))
	}

	// #nosec G202 -- clauses are generated from fixed column predicates and positional placeholders only.
	rows, err := s.db.QueryContext(ctx, `
		SELECT tick_id, team_id, challenge_id, phase, status, message, checked_at
		FROM checker_runs
		WHERE `+strings.Join(clauses, " OR ")+`
		ORDER BY tick_id DESC, id DESC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	grouped := make(map[checkerRunGroupKey][]apigateway.GameCheckerRun, len(keys))
	for rows.Next() {
		var run apigateway.GameCheckerRun
		var checkedAt time.Time
		if err := rows.Scan(&run.TickID, &run.TeamID, &run.ChallengeID, &run.Phase, &run.Status, &run.Message, &checkedAt); err != nil {
			return nil, err
		}
		run.CheckedAt = checkedAt.UTC().Format(time.RFC3339)
		key := checkerRunKey(run)
		grouped[key] = append(grouped[key], run)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	summaries := make(map[checkerRunGroupKey]apigateway.GameServiceStateSummary, len(keys))
	for _, key := range keys {
		summaries[key] = apigateway.SummarizeCheckerRunsForTick(grouped[key], key.TickID)
	}
	return summaries, nil
}

func refreshPostgresServiceStateTx(ctx context.Context, tx *sql.Tx, run checkerRunRecord) error {
	if normalized := apigateway.NormalizeServiceStateStatus(run.ReportedServiceState); normalized != "" {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO checker_service_states (
				tick_id, team_id, challenge_id, service_state, state_phase, state_message, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, NOW())
			ON CONFLICT (tick_id, team_id, challenge_id) DO UPDATE SET
				service_state = EXCLUDED.service_state,
				state_phase = EXCLUDED.state_phase,
				state_message = EXCLUDED.state_message,
				updated_at = EXCLUDED.updated_at
		`, run.TickID, run.TeamID, run.ChallengeID, normalized, inferReportedStatePhase(normalized), fallbackReportedStateMessage(normalized, run.ReportedStateMessage))
		return err
	}

	if run.Status == "skipped" {
		var existingState string
		err := tx.QueryRowContext(ctx, `
			SELECT service_state
			FROM checker_service_states
			WHERE tick_id = $1 AND team_id = $2 AND challenge_id = $3
		`, run.TickID, run.TeamID, run.ChallengeID).Scan(&existingState)
		switch {
		case err == nil && strings.TrimSpace(existingState) != "":
			return nil
		case errors.Is(err, sql.ErrNoRows):
		case err != nil:
			return err
		}
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT tick_id, team_id, challenge_id, phase, status, message, checked_at
		FROM checker_runs
		WHERE tick_id = $1 AND team_id = $2 AND challenge_id = $3
		ORDER BY id DESC
	`, run.TickID, run.TeamID, run.ChallengeID)
	if err != nil {
		return err
	}
	defer rows.Close()

	group := make([]apigateway.GameCheckerRun, 0, 3)
	for rows.Next() {
		var run apigateway.GameCheckerRun
		var checkedAt time.Time
		if err := rows.Scan(&run.TickID, &run.TeamID, &run.ChallengeID, &run.Phase, &run.Status, &run.Message, &checkedAt); err != nil {
			return err
		}
		run.CheckedAt = checkedAt.UTC().Format(time.RFC3339)
		group = append(group, run)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	summary := apigateway.SummarizeCheckerRunsForTick(group, run.TickID)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO checker_service_states (
			tick_id, team_id, challenge_id, service_state, state_phase, state_message, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, NOW())
		ON CONFLICT (tick_id, team_id, challenge_id) DO UPDATE SET
			service_state = EXCLUDED.service_state,
			state_phase = EXCLUDED.state_phase,
			state_message = EXCLUDED.state_message,
			updated_at = EXCLUDED.updated_at
	`, run.TickID, run.TeamID, run.ChallengeID, summary.Status, summary.Phase, summary.Message)
	return err
}

func inferReportedStatePhase(status string) string {
	switch status {
	case "recovering":
		return "put"
	case "flag_not_found":
		return "get"
	case "faulty", "ok", "down":
		return "check"
	default:
		return ""
	}
}

func fallbackReportedStateMessage(status, message string) string {
	message = strings.TrimSpace(message)
	if message != "" {
		return message
	}
	switch status {
	case "ok":
		return "checker reported the service healthy."
	case "recovering":
		return "checker reported a recovering service state."
	case "flag_not_found":
		return "checker reported the stored flag missing."
	case "faulty":
		return "checker reported the service functionality degraded."
	case "down":
		return "checker reported the service unavailable."
	default:
		return "checker reported the latest service state."
	}
}

func (s *postgresGameStore) LoadSchedulerState(ctx context.Context, intervalSeconds int) (apigateway.GameSchedulerStatus, error) {
	status := apigateway.GameSchedulerStatus{State: "stopped", IntervalSeconds: intervalSeconds}
	var lastRunAt sql.NullTime
	var nextRunAt sql.NullTime
	var lastTickID sql.NullInt64
	var lastError sql.NullString

	err := s.db.QueryRowContext(ctx, `
		SELECT state, interval_seconds, last_run_at, next_run_at, last_tick_id, last_error
		FROM game_scheduler_state
		WHERE singleton = TRUE
	`).Scan(&status.State, &status.IntervalSeconds, &lastRunAt, &nextRunAt, &lastTickID, &lastError)
	if errors.Is(err, sql.ErrNoRows) {
		return status, nil
	}
	if err != nil {
		return apigateway.GameSchedulerStatus{}, err
	}
	if intervalSeconds > 0 {
		status.IntervalSeconds = intervalSeconds
	}
	if lastRunAt.Valid {
		status.LastRunAt = lastRunAt.Time.UTC().Format(time.RFC3339)
	}
	if nextRunAt.Valid {
		status.NextRunAt = nextRunAt.Time.UTC().Format(time.RFC3339)
	}
	if lastTickID.Valid {
		status.LastTickID = int(lastTickID.Int64)
	}
	if lastError.Valid {
		status.LastError = lastError.String
	}
	return status, nil
}

func (s *postgresGameStore) SaveSchedulerState(ctx context.Context, status apigateway.GameSchedulerStatus, now time.Time) error {
	var lastRunAt any
	if status.LastRunAt != "" {
		parsed, err := time.Parse(time.RFC3339, status.LastRunAt)
		if err != nil {
			return err
		}
		lastRunAt = parsed.UTC()
	}

	var nextRunAt any
	if status.NextRunAt != "" {
		parsed, err := time.Parse(time.RFC3339, status.NextRunAt)
		if err != nil {
			return err
		}
		nextRunAt = parsed.UTC()
	}

	var lastTickID any
	if status.LastTickID > 0 {
		lastTickID = status.LastTickID
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO game_scheduler_state (singleton, state, interval_seconds, last_run_at, next_run_at, last_tick_id, last_error, updated_at)
		VALUES (TRUE, $1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (singleton) DO UPDATE SET
			state = EXCLUDED.state,
			interval_seconds = EXCLUDED.interval_seconds,
			last_run_at = EXCLUDED.last_run_at,
			next_run_at = EXCLUDED.next_run_at,
			last_tick_id = EXCLUDED.last_tick_id,
			last_error = EXCLUDED.last_error,
			updated_at = EXCLUDED.updated_at
	`, status.State, status.IntervalSeconds, lastRunAt, nextRunAt, lastTickID, status.LastError, now.UTC())
	return err
}

func (s *postgresGameStore) AppendSchedulerEvent(ctx context.Context, event schedulerEventRecord) (apigateway.GameSchedulerEvent, error) {
	record := apigateway.GameSchedulerEvent{
		EventType: event.EventType,
		Source:    event.Source,
		State:     event.State,
		TickID:    event.TickID,
		Message:   event.Message,
		CreatedAt: event.CreatedAt.UTC().Format(time.RFC3339),
	}

	var tickID any
	if event.TickID > 0 {
		tickID = event.TickID
	}

	if err := s.db.QueryRowContext(ctx, `
		INSERT INTO game_scheduler_events (event_type, source, state, tick_id, message, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`, event.EventType, event.Source, event.State, tickID, event.Message, event.CreatedAt.UTC()).Scan(&record.ID); err != nil {
		return apigateway.GameSchedulerEvent{}, err
	}
	return record, nil
}

func (s *postgresGameStore) ListSchedulerEvents(ctx context.Context, query apigateway.GameSchedulerEventQuery) (apigateway.GameSchedulerEventPage, error) {
	limit := query.Limit
	if limit <= 0 {
		limit = 25
	}
	offset := query.Offset
	if offset < 0 {
		offset = 0
	}

	whereClause, args := buildSchedulerEventFilterQuery(query)

	var totalCount int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM game_scheduler_events
	`+whereClause, args...).Scan(&totalCount); err != nil {
		return apigateway.GameSchedulerEventPage{}, err
	}

	statement := strings.Builder{}
	statement.WriteString(`
		SELECT id, event_type, source, state, tick_id, message, created_at
		FROM game_scheduler_events
	`)
	statement.WriteString(whereClause)
	statement.WriteString(fmt.Sprintf(" ORDER BY created_at DESC, id DESC LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2))
	queryArgs := append(append([]any(nil), args...), limit, offset)

	rows, err := s.db.QueryContext(ctx, statement.String(), queryArgs...)
	if err != nil {
		return apigateway.GameSchedulerEventPage{}, err
	}
	defer rows.Close()

	result := make([]apigateway.GameSchedulerEvent, 0)
	for rows.Next() {
		var item apigateway.GameSchedulerEvent
		var tickID sql.NullInt64
		var createdAt time.Time
		if err := rows.Scan(&item.ID, &item.EventType, &item.Source, &item.State, &tickID, &item.Message, &createdAt); err != nil {
			return apigateway.GameSchedulerEventPage{}, err
		}
		if tickID.Valid {
			item.TickID = int(tickID.Int64)
		}
		item.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return apigateway.GameSchedulerEventPage{}, err
	}
	return apigateway.GameSchedulerEventPage{
		Items:      result,
		Limit:      limit,
		Offset:     offset,
		TotalCount: totalCount,
		HasPrev:    offset > 0,
		HasNext:    offset+len(result) < totalCount,
	}, nil
}

func buildCheckerRunFilterQuery(query apigateway.GameCheckerRunQuery) (string, []any) {
	statement := strings.Builder{}
	statement.WriteString(" WHERE 1=1")
	args := make([]any, 0, 5)
	arg := 1
	if query.TickID > 0 {
		statement.WriteString(fmt.Sprintf(" AND tick_id = $%d", arg))
		args = append(args, query.TickID)
		arg++
	}
	if query.TeamID > 0 {
		statement.WriteString(fmt.Sprintf(" AND team_id = $%d", arg))
		args = append(args, query.TeamID)
		arg++
	}
	if query.ChallengeID > 0 {
		statement.WriteString(fmt.Sprintf(" AND challenge_id = $%d", arg))
		args = append(args, query.ChallengeID)
		arg++
	}
	if query.Phase != "" {
		statement.WriteString(fmt.Sprintf(" AND phase = $%d", arg))
		args = append(args, query.Phase)
		arg++
	}
	if query.Status != "" {
		statement.WriteString(fmt.Sprintf(" AND status = $%d", arg))
		args = append(args, query.Status)
	}
	return statement.String(), args
}

func buildSchedulerEventFilterQuery(query apigateway.GameSchedulerEventQuery) (string, []any) {
	statement := strings.Builder{}
	statement.WriteString(" WHERE 1=1")
	args := make([]any, 0, 3)
	arg := 1
	if query.EventType != "" {
		statement.WriteString(fmt.Sprintf(" AND event_type = $%d", arg))
		args = append(args, query.EventType)
		arg++
	}
	if query.Source != "" {
		statement.WriteString(fmt.Sprintf(" AND source = $%d", arg))
		args = append(args, query.Source)
		arg++
	}
	if query.State != "" {
		statement.WriteString(fmt.Sprintf(" AND state = $%d", arg))
		args = append(args, query.State)
	}
	return statement.String(), args
}

func (s *postgresGameStore) ListScoreboard(ctx context.Context) ([]apigateway.ScoreRowAlias, error) {
	return s.buildScoreboard(ctx, false)
}

func (s *postgresGameStore) RecomputeScoreboard(ctx context.Context) ([]apigateway.ScoreRowAlias, error) {
	return s.buildScoreboard(ctx, true)
}

func (s *postgresGameStore) AuditScoreboard(ctx context.Context) (apigateway.ScoringAuditAlias, error) {
	stored, err := s.listStoredScoreboard(ctx)
	if err != nil {
		return apigateway.ScoringAuditAlias{}, err
	}
	replayed, err := s.buildScoreboard(ctx, false)
	if err != nil {
		return apigateway.ScoringAuditAlias{}, err
	}
	return buildScoringAuditReport(stored, replayed), nil
}

func (s *postgresGameStore) buildScoreboard(ctx context.Context, persist bool) ([]apigateway.ScoreRowAlias, error) {
	type teamRecord struct {
		ID   int
		Name string
	}

	teams := make([]teamRecord, 0)
	// Deactivated teams are excluded from the leaderboard. Their historical
	// rows (flags, captures, checker runs) are left intact so reactivation
	// restores them cleanly; they simply do not surface while inactive.
	rows, err := s.db.QueryContext(ctx, `SELECT id, name FROM teams WHERE active = TRUE ORDER BY id`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var team teamRecord
		if err := rows.Scan(&team.ID, &team.Name); err != nil {
			rows.Close()
			return nil, err
		}
		teams = append(teams, team)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	previousRanks := make(map[int]int, len(teams))
	prevRows, err := s.db.QueryContext(ctx, `SELECT team_id, rank FROM scoreboard_entries`)
	if err != nil {
		return nil, err
	}
	for prevRows.Next() {
		var teamID, rank int
		if err := prevRows.Scan(&teamID, &rank); err != nil {
			prevRows.Close()
			return nil, err
		}
		previousRanks[teamID] = rank
	}
	if err := prevRows.Err(); err != nil {
		prevRows.Close()
		return nil, err
	}
	prevRows.Close()

	teamCount := len(teams)
	slaFactor := faustSLAFactor(teamCount)

	attackByTeam, err := s.sumFloatByTeam(ctx, `
		SELECT ranked.team_id,
		       COALESCE(SUM(1.0 + (1.0 / ranked.capture_count)), 0.0)
		FROM (
			SELECT
				sf.flag,
				sf.team_id,
				COUNT(*) OVER (PARTITION BY sf.flag) AS capture_count
			FROM submitted_flags sf
		) ranked
		GROUP BY ranked.team_id
	`)
	if err != nil {
		return nil, err
	}

	defenseByTeam, err := s.sumFloatByTeam(ctx, `
		SELECT
			f.owner_team_id,
			COALESCE(-SUM(CASE WHEN COALESCE(captures.capture_count, 0) > 0 THEN POWER(captures.capture_count::double precision, 0.75) ELSE 0.0 END), 0.0)
		FROM issued_flags f
		LEFT JOIN (
			SELECT flag, COUNT(*) AS capture_count
			FROM submitted_flags
			GROUP BY flag
		) captures ON captures.flag = f.flag
		GROUP BY f.owner_team_id
	`)
	if err != nil {
		return nil, err
	}

	slaByTeam, err := s.sumFloatByTeam(ctx, `
		SELECT states.team_id,
		       COALESCE(SUM(
			       CASE
			           WHEN states.service_state = 'ok' THEN 1.0
			           WHEN states.service_state = 'recovering' THEN 0.5
			           ELSE 0.0
			       END
		       ) * $1, 0.0)
		FROM checker_service_states states
		GROUP BY states.team_id
	`, slaFactor)
	if err != nil {
		return nil, err
	}

	attackByTeamService, err := s.sumFloatByTeamAndChallenge(ctx, `
		SELECT ranked.team_id,
		       f.challenge_id,
		       COALESCE(SUM(1.0 + (1.0 / ranked.capture_count)), 0.0)
		FROM (
			SELECT
				sf.flag,
				sf.team_id,
				COUNT(*) OVER (PARTITION BY sf.flag) AS capture_count
			FROM submitted_flags sf
		) ranked
		JOIN issued_flags f ON f.flag = ranked.flag
		GROUP BY ranked.team_id, f.challenge_id
	`)
	if err != nil {
		return nil, err
	}

	slaByTeamService, err := s.sumFloatByTeamAndChallenge(ctx, `
		SELECT states.team_id,
		       states.challenge_id,
		       COALESCE(SUM(
			       CASE
			           WHEN states.service_state = 'ok' THEN 1.0
			           WHEN states.service_state = 'recovering' THEN 0.5
			           ELSE 0.0
			       END
		       ) * $1, 0.0)
		FROM checker_service_states states
		GROUP BY states.team_id, states.challenge_id
	`, slaFactor)
	if err != nil {
		return nil, err
	}

	defenseByTeamService, err := s.sumFloatByTeamAndChallenge(ctx, `
		SELECT
			f.owner_team_id,
			f.challenge_id,
			COALESCE(-SUM(CASE WHEN COALESCE(captures.capture_count, 0) > 0 THEN POWER(captures.capture_count::double precision, 0.75) ELSE 0.0 END), 0.0)
		FROM issued_flags f
		LEFT JOIN (
			SELECT flag, COUNT(*) AS capture_count
			FROM submitted_flags
			GROUP BY flag
		) captures ON captures.flag = f.flag
		GROUP BY f.owner_team_id, f.challenge_id
	`)
	if err != nil {
		return nil, err
	}

	challengeRows, err := s.db.QueryContext(ctx, `SELECT id, name FROM challenges ORDER BY id`)
	if err != nil {
		return nil, err
	}
	type challengeRecord struct {
		ID   int
		Name string
	}
	challenges := make([]challengeRecord, 0)
	for challengeRows.Next() {
		var challenge challengeRecord
		if err := challengeRows.Scan(&challenge.ID, &challenge.Name); err != nil {
			challengeRows.Close()
			return nil, err
		}
		challenges = append(challenges, challenge)
	}
	if err := challengeRows.Err(); err != nil {
		challengeRows.Close()
		return nil, err
	}
	challengeRows.Close()

	scoreboard := make([]teamScoreSnapshot, 0, len(teams))
	for _, team := range teams {
		services := make([]apigateway.ServiceScoreBreakdownAlias, 0, len(challenges))
		for _, challenge := range challenges {
			service := apigateway.ServiceScoreBreakdownAlias{
				ChallengeID: challenge.ID,
				Service:     challenge.Name,
				Attack:      nestedFloat(attackByTeamService, team.ID, challenge.ID),
				Defense:     nestedFloat(defenseByTeamService, team.ID, challenge.ID),
				SLA:         nestedFloat(slaByTeamService, team.ID, challenge.ID),
			}
			service.Total = service.Attack + service.Defense + service.SLA
			services = append(services, service)
		}
		scoreboard = append(scoreboard, teamScoreSnapshot{
			TeamID:   team.ID,
			Team:     team.Name,
			Attack:   attackByTeam[team.ID],
			Defense:  defenseByTeam[team.ID],
			SLA:      slaByTeam[team.ID],
			Services: services,
		})
	}

	sort.Slice(scoreboard, func(i, j int) bool {
		if scoreboard[i].Total() != scoreboard[j].Total() {
			return scoreboard[i].Total() > scoreboard[j].Total()
		}
		if scoreboard[i].Attack != scoreboard[j].Attack {
			return scoreboard[i].Attack > scoreboard[j].Attack
		}
		if scoreboard[i].Defense != scoreboard[j].Defense {
			return scoreboard[i].Defense > scoreboard[j].Defense
		}
		return scoreboard[i].Team < scoreboard[j].Team
	})

	result := make([]apigateway.ScoreRowAlias, 0, len(scoreboard))
	for index, snapshot := range scoreboard {
		row := apigateway.ScoreRowAlias{
			Rank:     index + 1,
			Team:     snapshot.Team,
			Attack:   snapshot.Attack,
			Defense:  snapshot.Defense,
			SLA:      snapshot.SLA,
			Total:    snapshot.Total(),
			Delta:    rankDelta(previousRanks[snapshot.TeamID], index+1),
			Services: append([]apigateway.ServiceScoreBreakdownAlias(nil), snapshot.Services...),
		}
		result = append(result, row)
	}
	if !persist {
		return result, nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	for index, row := range result {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO scoreboard_entries (team_id, rank, team_name, attack_points, defense_points, sla_points, total_points, delta)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (team_id) DO UPDATE SET
				rank = EXCLUDED.rank,
				team_name = EXCLUDED.team_name,
				attack_points = EXCLUDED.attack_points,
				defense_points = EXCLUDED.defense_points,
				sla_points = EXCLUDED.sla_points,
				total_points = EXCLUDED.total_points,
				delta = EXCLUDED.delta
		`, scoreboard[index].TeamID, row.Rank, row.Team, row.Attack, row.Defense, row.SLA, row.Total, row.Delta); err != nil {
			return nil, err
		}
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM scoreboard_entries WHERE team_id NOT IN (SELECT id FROM teams)`); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *postgresGameStore) listStoredScoreboard(ctx context.Context) ([]apigateway.ScoreRowAlias, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT rank, team_name, attack_points, defense_points, sla_points, total_points, delta
		FROM scoreboard_entries
		ORDER BY rank ASC, team_name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]apigateway.ScoreRowAlias, 0)
	for rows.Next() {
		var row apigateway.ScoreRowAlias
		if err := rows.Scan(&row.Rank, &row.Team, &row.Attack, &row.Defense, &row.SLA, &row.Total, &row.Delta); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (s *postgresGameStore) ListAttackFeed(ctx context.Context, query apigateway.AttackFeedQuery) (apigateway.AttackFeedPage, error) {
	limit := query.Limit
	if limit <= 0 {
		limit = 12
	}
	offset := query.Offset
	if offset < 0 {
		offset = 0
	}

	conditions := []string{"1=1"}
	args := []any{}

	if query.Attacker != "" {
		args = append(args, "%"+query.Attacker+"%")
		conditions = append(conditions, fmt.Sprintf("attacker ILIKE $%d", len(args)))
	}
	if query.Victim != "" {
		args = append(args, "%"+query.Victim+"%")
		conditions = append(conditions, fmt.Sprintf("victim ILIKE $%d", len(args)))
	}
	if query.Service != "" {
		args = append(args, "%"+query.Service+"%")
		conditions = append(conditions, fmt.Sprintf("service ILIKE $%d", len(args)))
	}
	if query.TickFrom > 0 {
		args = append(args, query.TickFrom)
		conditions = append(conditions, fmt.Sprintf("tick >= $%d", len(args)))
	}
	if query.TickTo > 0 {
		args = append(args, query.TickTo)
		conditions = append(conditions, fmt.Sprintf("tick <= $%d", len(args)))
	}

	whereClause := strings.Join(conditions, " AND ")
	var totalCount int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM attack_events WHERE %s", whereClause) // #nosec G201 -- whereClause is assembled from fixed predicates and placeholders only.
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&totalCount); err != nil {
		return apigateway.AttackFeedPage{}, err
	}

	queryArgs := append(append([]any{}, args...), limit, offset)
	// #nosec G201 -- whereClause is assembled from fixed predicates and placeholders only.
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT id, attacker, victim, service, tick, verdict, created_at
		FROM attack_events
		WHERE %s
		ORDER BY created_at DESC, id DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, len(queryArgs)-1, len(queryArgs)), queryArgs...)
	if err != nil {
		return apigateway.AttackFeedPage{}, err
	}
	defer rows.Close()

	result := make([]apigateway.AttackEventAlias, 0)
	for rows.Next() {
		var item apigateway.AttackEventAlias
		if err := rows.Scan(&item.ID, &item.Attacker, &item.Victim, &item.Service, &item.Tick, &item.Verdict, &item.CreatedAt); err != nil {
			return apigateway.AttackFeedPage{}, err
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return apigateway.AttackFeedPage{}, err
	}
	return apigateway.AttackFeedPage{
		Items:      result,
		Limit:      limit,
		Offset:     offset,
		TotalCount: totalCount,
		HasPrev:    offset > 0,
		HasNext:    offset+len(result) < totalCount,
	}, nil
}

func filterAttackFeedRows(events []apigateway.AttackEventAlias, query apigateway.AttackFeedQuery) []apigateway.AttackEventAlias {
	if query.Attacker == "" && query.Victim == "" && query.Service == "" && query.TickFrom == 0 && query.TickTo == 0 {
		return events
	}

	filtered := make([]apigateway.AttackEventAlias, 0, len(events))
	for _, event := range events {
		if !attackFeedFieldMatches(event.Attacker, query.Attacker) {
			continue
		}
		if !attackFeedFieldMatches(event.Victim, query.Victim) {
			continue
		}
		if !attackFeedFieldMatches(event.Service, query.Service) {
			continue
		}
		if query.TickFrom > 0 && event.Tick < query.TickFrom {
			continue
		}
		if query.TickTo > 0 && event.Tick > query.TickTo {
			continue
		}
		filtered = append(filtered, event)
	}
	return filtered
}

func attackFeedFieldMatches(value string, filter string) bool {
	if filter == "" {
		return true
	}
	return strings.Contains(strings.ToLower(value), strings.ToLower(filter))
}

func (s *postgresGameStore) MatchStatus(ctx context.Context) (apigateway.GameMatchStatus, error) {
	status := apigateway.GameMatchStatus{
		State:                "not_started",
		AcceptingSubmissions: false,
	}
	var startedAt sql.NullTime
	var endedAt sql.NullTime
	var scheduledStartAt sql.NullTime
	var scheduledEndAt sql.NullTime
	var scheduleConfigured bool

	err := s.db.QueryRowContext(ctx, `
		SELECT state, started_at, ended_at, scheduled_start_at, scheduled_end_at, schedule_configured
		FROM game_match_state
		WHERE singleton = TRUE
	`).Scan(&status.State, &startedAt, &endedAt, &scheduledStartAt, &scheduledEndAt, &scheduleConfigured)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return status, nil
	case err != nil:
		return apigateway.GameMatchStatus{}, err
	}

	if startedAt.Valid {
		status.StartedAt = startedAt.Time.UTC().Format(time.RFC3339)
	}
	if endedAt.Valid {
		status.EndedAt = endedAt.Time.UTC().Format(time.RFC3339)
	}
	if scheduledStartAt.Valid {
		status.ScheduledStartAt = scheduledStartAt.Time.UTC().Format(time.RFC3339)
	}
	if scheduledEndAt.Valid {
		status.ScheduledEndAt = scheduledEndAt.Time.UTC().Format(time.RFC3339)
	}
	status.ScheduleConfigured = scheduleConfigured
	status.AcceptingSubmissions = status.State == "running"
	return status, nil
}

func (s *postgresGameStore) StartMatch(ctx context.Context, now time.Time) (apigateway.GameMatchStatus, error) {
	current, err := s.MatchStatus(ctx)
	if err != nil {
		return apigateway.GameMatchStatus{}, err
	}
	switch current.State {
	case "running":
		return current, nil
	case "finished":
		return apigateway.GameMatchStatus{}, errContestOver
	}

	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO game_match_state (singleton, state, started_at, ended_at, scheduled_start_at, scheduled_end_at, schedule_configured, updated_at)
		VALUES (TRUE, 'running', $1, NULL, $2, $3, $4, $1)
		ON CONFLICT (singleton) DO UPDATE SET
			state = EXCLUDED.state,
			started_at = EXCLUDED.started_at,
			ended_at = EXCLUDED.ended_at,
			updated_at = EXCLUDED.updated_at
	`, now.UTC(), nullTimeValue(current.ScheduledStartAt), nullTimeValue(current.ScheduledEndAt), current.ScheduleConfigured); err != nil {
		return apigateway.GameMatchStatus{}, err
	}
	return apigateway.GameMatchStatus{
		State:                "running",
		StartedAt:            now.UTC().Format(time.RFC3339),
		ScheduledStartAt:     current.ScheduledStartAt,
		ScheduledEndAt:       current.ScheduledEndAt,
		ScheduleConfigured:   current.ScheduleConfigured,
		AcceptingSubmissions: true,
	}, nil
}

func (s *postgresGameStore) PauseMatch(ctx context.Context, now time.Time) (apigateway.GameMatchStatus, error) {
	current, err := s.MatchStatus(ctx)
	if err != nil {
		return apigateway.GameMatchStatus{}, err
	}
	if current.State == "finished" {
		return apigateway.GameMatchStatus{}, errContestOver
	}
	if current.State == "not_started" {
		return apigateway.GameMatchStatus{}, errContestNotStarted
	}
	if current.State == "paused" {
		return current, nil
	}

	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO game_match_state (singleton, state, started_at, ended_at, scheduled_start_at, scheduled_end_at, schedule_configured, updated_at)
		VALUES (TRUE, 'paused', $1, NULL, $2, $3, $4, $5)
		ON CONFLICT (singleton) DO UPDATE SET
			state = EXCLUDED.state,
			updated_at = EXCLUDED.updated_at
	`, nullTimeValue(current.StartedAt), nullTimeValue(current.ScheduledStartAt), nullTimeValue(current.ScheduledEndAt), current.ScheduleConfigured, now.UTC()); err != nil {
		return apigateway.GameMatchStatus{}, err
	}
	current.State = "paused"
	current.AcceptingSubmissions = false
	return current, nil
}

func (s *postgresGameStore) ResumeMatch(ctx context.Context, now time.Time) (apigateway.GameMatchStatus, error) {
	current, err := s.MatchStatus(ctx)
	if err != nil {
		return apigateway.GameMatchStatus{}, err
	}
	if current.State == "finished" {
		return apigateway.GameMatchStatus{}, errContestOver
	}
	if current.State != "paused" {
		return current, nil
	}

	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO game_match_state (singleton, state, started_at, ended_at, scheduled_start_at, scheduled_end_at, schedule_configured, updated_at)
		VALUES (TRUE, 'running', $1, NULL, $2, $3, $4, $5)
		ON CONFLICT (singleton) DO UPDATE SET
			state = EXCLUDED.state,
			updated_at = EXCLUDED.updated_at
	`, nullTimeValue(current.StartedAt), nullTimeValue(current.ScheduledStartAt), nullTimeValue(current.ScheduledEndAt), current.ScheduleConfigured, now.UTC()); err != nil {
		return apigateway.GameMatchStatus{}, err
	}
	current.State = "running"
	current.AcceptingSubmissions = true
	return current, nil
}

func (s *postgresGameStore) StopMatch(ctx context.Context, now time.Time) (apigateway.GameMatchStatus, error) {
	current, err := s.MatchStatus(ctx)
	if err != nil {
		return apigateway.GameMatchStatus{}, err
	}
	if current.State == "finished" {
		return current, nil
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO game_match_state (singleton, state, started_at, ended_at, scheduled_start_at, scheduled_end_at, schedule_configured, updated_at)
		VALUES (TRUE, 'finished', $1, $2, $3, $4, $5, $2)
		ON CONFLICT (singleton) DO UPDATE SET
			state = EXCLUDED.state,
			ended_at = EXCLUDED.ended_at,
			updated_at = EXCLUDED.updated_at
	`, nullTimeValue(current.StartedAt), now.UTC(), nullTimeValue(current.ScheduledStartAt), nullTimeValue(current.ScheduledEndAt), current.ScheduleConfigured); err != nil {
		return apigateway.GameMatchStatus{}, err
	}
	current.State = "finished"
	current.EndedAt = now.UTC().Format(time.RFC3339)
	current.AcceptingSubmissions = false
	return current, nil
}

func (s *postgresGameStore) UpdateMatchSchedule(ctx context.Context, startAt, endAt *time.Time) (apigateway.GameMatchStatus, error) {
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO game_match_state (singleton, state, scheduled_start_at, scheduled_end_at, schedule_configured, updated_at)
		VALUES (TRUE, 'not_started', $1, $2, TRUE, NOW())
		ON CONFLICT (singleton) DO UPDATE SET
			scheduled_start_at = EXCLUDED.scheduled_start_at,
			scheduled_end_at = EXCLUDED.scheduled_end_at,
			schedule_configured = EXCLUDED.schedule_configured,
			updated_at = EXCLUDED.updated_at
	`, startAt, endAt); err != nil {
		return apigateway.GameMatchStatus{}, err
	}
	return s.MatchStatus(ctx)
}

func (s *postgresGameStore) Close() error {
	return s.db.Close()
}

func nullTimeValue(raw string) any {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil
	}
	return parsed.UTC()
}

type teamScoreSnapshot struct {
	TeamID   int
	Team     string
	Attack   float64
	Defense  float64
	SLA      float64
	Services []apigateway.ServiceScoreBreakdownAlias
}

func (s teamScoreSnapshot) Total() float64 {
	return s.Attack + s.Defense + s.SLA
}

func (s *postgresGameStore) sumFloatByTeam(ctx context.Context, query string, args ...any) (map[int]float64, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int]float64)
	for rows.Next() {
		var teamID int
		var value float64
		if err := rows.Scan(&teamID, &value); err != nil {
			return nil, err
		}
		result[teamID] = value
	}
	return result, rows.Err()
}

func (s *postgresGameStore) sumFloatByTeamAndChallenge(ctx context.Context, query string, args ...any) (map[int]map[int]float64, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int]map[int]float64)
	for rows.Next() {
		var teamID, challengeID int
		var value float64
		if err := rows.Scan(&teamID, &challengeID, &value); err != nil {
			return nil, err
		}
		if _, ok := result[teamID]; !ok {
			result[teamID] = make(map[int]float64)
		}
		result[teamID][challengeID] = value
	}
	return result, rows.Err()
}

func acceptedSubmissionKey(flag string, teamID int) string {
	return fmt.Sprintf("%s:%d", flag, teamID)
}

func acceptedFlagAttackEventID(submission acceptedFlagSubmission) string {
	sum := sha256.Sum256([]byte(acceptedSubmissionKey(submission.Flag, submission.SubmittingTeam)))
	return fmt.Sprintf("atk-submit-%d-%d-%s", submission.SubmissionTick, submission.SubmittingTeam, hex.EncodeToString(sum[:8]))
}

func sortAcceptedSubmissions(submissions []acceptedFlagSubmission) {
	sort.Slice(submissions, func(i, j int) bool {
		if !submissions[i].SubmittedAt.Equal(submissions[j].SubmittedAt) {
			return submissions[i].SubmittedAt.Before(submissions[j].SubmittedAt)
		}
		if submissions[i].SubmittingTeam != submissions[j].SubmittingTeam {
			return submissions[i].SubmittingTeam < submissions[j].SubmittingTeam
		}
		return submissions[i].Flag < submissions[j].Flag
	})
}

func sortedChallengeIDs(challenges map[int]memoryChallenge) []int {
	result := make([]int, 0, len(challenges))
	for challengeID := range challenges {
		result = append(result, challengeID)
	}
	slices.Sort(result)
	return result
}

func nestedFloat(values map[int]map[int]float64, teamID, challengeID int) float64 {
	if byChallenge, ok := values[teamID]; ok {
		return byChallenge[challengeID]
	}
	return 0
}

func faustAttackValue(captureCount int) float64 {
	if captureCount <= 0 {
		return 0
	}
	return 1.0 + (1.0 / float64(captureCount))
}

func faustDefensePenalty(captureCount int) float64 {
	if captureCount <= 0 {
		return 0
	}
	return math.Pow(float64(captureCount), 0.75)
}

func faustSLAValue(putOK, getOK, checkOK bool) float64 {
	switch {
	case putOK && getOK && checkOK:
		return 1.0
	case !putOK && getOK && checkOK:
		return faustRecoveringValue
	default:
		return 0.0
	}
}

func faustSLAValueForStatus(status string) float64 {
	switch strings.TrimSpace(strings.ToLower(status)) {
	case "ok":
		return 1.0
	case "recovering":
		return faustRecoveringValue
	default:
		return 0.0
	}
}

func faustSLAFactor(teamCount int) float64 {
	if teamCount <= 0 {
		return 0
	}
	return math.Sqrt(float64(teamCount))
}

func buildScoringAuditReport(stored, replayed []apigateway.ScoreRowAlias) apigateway.ScoringAuditAlias {
	const scoreTolerance = 0.000001

	report := apigateway.ScoringAuditAlias{
		Status:       "ok",
		StoredRows:   len(stored),
		ReplayedRows: len(replayed),
		Mismatches:   make([]apigateway.ScoringAuditMismatchAlias, 0),
	}

	storedByTeam := make(map[string]apigateway.ScoreRowAlias, len(stored))
	for _, row := range stored {
		storedByTeam[row.Team] = row
	}
	replayedByTeam := make(map[string]apigateway.ScoreRowAlias, len(replayed))
	for _, row := range replayed {
		replayedByTeam[row.Team] = row
	}

	for _, row := range replayed {
		storedRow, ok := storedByTeam[row.Team]
		if !ok {
			report.Mismatches = append(report.Mismatches, apigateway.ScoringAuditMismatchAlias{
				Team:   row.Team,
				Field:  "row",
				Detail: "replayed row is missing from stored scoreboard",
			})
			continue
		}
		appendScoreMismatch := func(field string, storedValue, replayedValue float64) {
			delta := replayedValue - storedValue
			if math.Abs(delta) <= scoreTolerance {
				return
			}
			report.Mismatches = append(report.Mismatches, apigateway.ScoringAuditMismatchAlias{
				Team:     row.Team,
				Field:    field,
				Stored:   storedValue,
				Replayed: replayedValue,
				Delta:    delta,
			})
		}
		appendScoreMismatch("rank", float64(storedRow.Rank), float64(row.Rank))
		appendScoreMismatch("attack", storedRow.Attack, row.Attack)
		appendScoreMismatch("defense", storedRow.Defense, row.Defense)
		appendScoreMismatch("sla", storedRow.SLA, row.SLA)
		appendScoreMismatch("total", storedRow.Total, row.Total)
	}
	for _, row := range stored {
		if _, ok := replayedByTeam[row.Team]; ok {
			continue
		}
		report.Mismatches = append(report.Mismatches, apigateway.ScoringAuditMismatchAlias{
			Team:   row.Team,
			Field:  "row",
			Detail: "stored row is missing from replayed scoreboard",
		})
	}

	report.MismatchCount = len(report.Mismatches)
	if report.MismatchCount > 0 {
		report.Status = "mismatch"
	}
	return report
}

func rankDelta(previousRank, currentRank int) string {
	if previousRank == 0 {
		return "new"
	}
	diff := previousRank - currentRank
	switch {
	case diff > 0:
		return fmt.Sprintf("+%d", diff)
	case diff < 0:
		return fmt.Sprintf("%d", diff)
	default:
		return "0"
	}
}
