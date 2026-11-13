package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"time"

	"adplatform/internal/services/apigateway"
)

type flagRetentionLoss struct {
	ConfirmedMissing bool
	Penalty          float64
}

// At most 2 attack points per opponent and n^0.75 saved defense points
// can be denied by hiding this flag. Exceed that combined upper bound.
func retentionPenalty(opponents int) float64 {
	if opponents < 0 {
		opponents = 0
	}
	return math.Ceil(math.Max(10, 2*float64(opponents)+faustDefensePenalty(opponents)+1))
}

const flagMissingExitCode = 42

func validateRetentionRun(f issuedFlagRecord, run checkerRunRecord) error {
	if run.TickID != f.IssuedTick || run.TeamID != f.OwnerTeamID || run.ChallengeID != f.ChallengeID || run.Phase != "get" {
		return fmt.Errorf("retention check identity does not match issued flag")
	}
	return nil
}

type flagScanner interface{ Scan(...any) error }

func scanRetentionFlag(row flagScanner) (issuedFlagRecord, error) {
	var f issuedFlagRecord
	var next sql.NullTime
	err := row.Scan(&f.Flag, &f.OwnerTeamID, &f.OwnerTeamName, &f.ChallengeID, &f.ChallengeName, &f.IssuedTick, &f.ExpiresTick, &f.CreatedAt, &f.Metadata, &f.CheckerToken, &next)
	if next.Valid {
		f.NextCheckAt = next.Time.UTC()
	}
	return f, err
}

const retentionFlagColumns = `flag, owner_team_id, owner_team_name, challenge_id, challenge_name, issued_tick, expires_tick, created_at, checker_metadata, checker_token_snapshot, retention_check_at`

func nullableCheckTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.UTC()
}

func (s *memoryGameStore) latestFlagLocked(team, challenge int) (issuedFlagRecord, error) {
	var latest issuedFlagRecord
	for _, f := range s.flags {
		if f.OwnerTeamID == team && f.ChallengeID == challenge && f.IssuedTick > latest.IssuedTick {
			latest = f
		}
	}
	if latest.Flag == "" {
		return latest, sql.ErrNoRows
	}
	return latest, nil
}
func (s *memoryGameStore) LatestIssuedFlag(_ context.Context, team, challenge int) (issuedFlagRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.latestFlagLocked(team, challenge)
}
func (s *postgresGameStore) LatestIssuedFlag(ctx context.Context, team, challenge int) (issuedFlagRecord, error) {
	return scanRetentionFlag(s.db.QueryRowContext(ctx, `SELECT `+retentionFlagColumns+` FROM issued_flags WHERE owner_team_id=$1 AND challenge_id=$2 ORDER BY issued_tick DESC, created_at DESC, flag DESC LIMIT 1`, team, challenge))
}
func (s *memoryGameStore) ScheduleRetentionCheck(_ context.Context, flag string, next time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.flags[flag]
	if !ok {
		return sql.ErrNoRows
	}
	f.NextCheckAt = next
	s.flags[flag] = f
	return nil
}
func (s *postgresGameStore) ScheduleRetentionCheck(ctx context.Context, flag string, next time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE issued_flags SET retention_check_at=$2 WHERE flag=$1`, flag, next.UTC())
	return err
}

func (s *memoryGameStore) RecordRetentionCheck(ctx context.Context, f issuedFlagRecord, run checkerRunRecord, confirmed bool) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if err := validateRetentionRun(f, run); err != nil {
		return false, err
	}
	confirmed = confirmed && run.Status == "failed" && run.ExitCode == flagMissingExitCode

	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.latestFlagLocked(f.OwnerTeamID, f.ChallengeID)
	if err != nil || current.Flag != f.Flag || f.CheckerToken == "" || s.match.State != "running" {
		return false, nil
	}
	challenge, ok := s.challenges[f.ChallengeID]
	if !ok || challenge.Maintenance || (challenge.PlayFromTick > 0 && (len(s.ticks) == 0 || challenge.PlayFromTick > s.ticks[len(s.ticks)-1].ID)) {
		return false, nil
	}
	eligible := false
	for _, target := range s.targets {
		if target.TeamID == f.OwnerTeamID && target.ChallengeID == f.ChallengeID && target.CheckerToken == f.CheckerToken {
			eligible = true
			break
		}
	}
	if !eligible {
		return false, nil
	}
	key := checkerRunKeyFromIDs(f.IssuedTick, f.OwnerTeamID, f.ChallengeID)
	if run.Status != "success" {
		if s.retentionFailures == nil {
			s.retentionFailures = map[checkerRunGroupKey]flagRetentionLoss{}
		}
		loss := s.retentionFailures[key]
		if confirmed && !loss.ConfirmedMissing {
			loss.ConfirmedMissing = true
			loss.Penalty = retentionPenalty(len(s.teamNames) - 1)
		}
		s.retentionFailures[key] = loss
		if confirmed && loss.ConfirmedMissing {
			run.Message = fmt.Sprintf("confirmed missing stored flag; tick SLA forfeited and %.0f-point defense penalty", loss.Penalty)
		}
	}
	record := apigateway.GameCheckerRun{ID: s.nextRun, TickID: run.TickID, TeamID: run.TeamID, TeamName: run.TeamName, ChallengeID: run.ChallengeID, ChallengeName: run.ChallengeName, Phase: run.Phase, Target: run.Target, CheckerImage: run.CheckerImage, Status: run.Status, ExitCode: run.ExitCode, Message: run.Message, Output: run.Output, CheckedAt: run.CheckedAt.UTC().Format(time.RFC3339)}
	s.nextRun++
	s.runs = append([]apigateway.GameCheckerRun{record}, s.runs...)
	s.refreshMemoryServiceState(run)
	for i := range s.ticks {
		if s.ticks[i].ID == run.TickID {
			s.ticks[i].TotalCheckerRuns++
			if run.Status == "success" {
				s.ticks[i].SuccessfulCheckerRuns++
			} else {
				s.ticks[i].FailedCheckerRuns++
			}
		}
	}
	return true, nil
}

func (s *postgresGameStore) RecordRetentionCheck(ctx context.Context, f issuedFlagRecord, run checkerRunRecord, confirmed bool) (bool, error) {
	if err := validateRetentionRun(f, run); err != nil {
		return false, err
	}
	confirmed = confirmed && run.Status == "failed" && run.ExitCode == flagMissingExitCode

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	// Recheck eligibility after the checker returns. Reset preparation rotates
	// the DB token before changing runtime storage. Avoid parent-row locks here:
	// submission acceptance takes the tick-table lock before the match row.
	var token string
	var active, published, maintenance bool
	var runtime, match string
	err = tx.QueryRowContext(ctx, `SELECT si.checker_token,t.active AND (t.play_from_tick IS NULL OR t.play_from_tick<=COALESCE((SELECT MAX(id) FROM game_ticks),0)),c.published AND (c.play_from_tick IS NULL OR c.play_from_tick<=COALESCE((SELECT MAX(id) FROM game_ticks),0)),c.maintenance,si.runtime_status,m.state
 FROM service_instances si JOIN teams t ON t.id=si.team_id JOIN challenges c ON c.id=si.challenge_id CROSS JOIN game_match_state m
 WHERE si.team_id=$1 AND si.challenge_id=$2 AND m.singleton=TRUE`, f.OwnerTeamID, f.ChallengeID).Scan(&token, &active, &published, &maintenance, &runtime, &match)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if f.CheckerToken == "" || token != f.CheckerToken || !active || !published || maintenance || runtime != "ready" || match != "running" {
		return false, nil
	}
	var latest string
	if err = tx.QueryRowContext(ctx, `SELECT flag FROM issued_flags WHERE owner_team_id=$1 AND challenge_id=$2 ORDER BY issued_tick DESC, created_at DESC, flag DESC LIMIT 1`, f.OwnerTeamID, f.ChallengeID).Scan(&latest); err != nil {
		return false, err
	}
	if latest != f.Flag {
		return false, nil
	}

	if run.Status != "success" {
		penalty := 0.0
		if confirmed {
			var opponents int
			if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM teams WHERE id<>$1 AND active=TRUE AND (play_from_tick IS NULL OR play_from_tick<=COALESCE((SELECT MAX(id) FROM game_ticks),0))`, f.OwnerTeamID).Scan(&opponents); err != nil {
				return false, err
			}
			penalty = retentionPenalty(opponents)
		}
		err = tx.QueryRowContext(ctx, `INSERT INTO flag_retention_failures(tick_id,team_id,challenge_id,flag,confirmed_missing,penalty_points,checked_at)
 VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(tick_id,team_id,challenge_id) DO UPDATE SET
 penalty_points=CASE WHEN flag_retention_failures.confirmed_missing THEN flag_retention_failures.penalty_points ELSE EXCLUDED.penalty_points END,
 confirmed_missing=flag_retention_failures.confirmed_missing OR EXCLUDED.confirmed_missing,checked_at=EXCLUDED.checked_at,
 reset_attempt_token=NULL,reset_created=FALSE,reset_prior_confirmed=NULL,reset_prior_penalty=NULL
 RETURNING penalty_points`, f.IssuedTick, f.OwnerTeamID, f.ChallengeID, f.Flag, confirmed, penalty, run.CheckedAt.UTC()).Scan(&penalty)
		if err != nil {
			return false, err
		}
		if confirmed && penalty > 0 {
			run.Message = fmt.Sprintf("confirmed missing stored flag; tick SLA forfeited and %.0f-point defense penalty", penalty)
		}
	}

	_, err = tx.ExecContext(ctx, `INSERT INTO checker_runs(tick_id,team_id,team_name,challenge_id,challenge_name,phase,target,checker_image,status,exit_code,message,output,checked_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, run.TickID, run.TeamID, run.TeamName, run.ChallengeID, run.ChallengeName, run.Phase, run.Target, run.CheckerImage, run.Status, run.ExitCode, run.Message, run.Output, run.CheckedAt.UTC())
	if err != nil {
		return false, err
	}
	if err = refreshPostgresServiceStateTx(ctx, tx, run); err != nil {
		return false, err
	}
	success, failed := 0, 0
	if run.Status == "success" {
		success = 1
	} else {
		failed = 1
	}
	_, err = tx.ExecContext(ctx, `UPDATE game_ticks SET total_checker_runs=total_checker_runs+1,successful_checker_runs=successful_checker_runs+$2,failed_checker_runs=failed_checker_runs+$3 WHERE id=$1`, run.TickID, success, failed)
	if err != nil {
		return false, err
	}
	return true, tx.Commit()
}
