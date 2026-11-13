package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"

	"adplatform/internal/services/apigateway"
)

var errSubmissionNotFound = errors.New("submission not found")

func (s *memoryGameStore) RecordSubmission(_ context.Context, row apigateway.AdminSubmission) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextSubmission++
	row.ID = s.nextSubmission
	s.submissions = append(s.submissions, row)
	return nil
}

func (s *memoryGameStore) ListSubmissions(_ context.Context, query apigateway.SubmissionQuery) (apigateway.SubmissionPage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows := make([]apigateway.AdminSubmission, 0)
	for _, row := range s.submissions {
		if !attackFeedFieldMatches(row.Attacker, query.Attacker) || !attackFeedFieldMatches(row.Victim, query.Victim) || !attackFeedFieldMatches(row.Service, query.Service) || (query.Status != "" && row.Status != query.Status) || (query.Flag != "" && !strings.Contains(strings.ToLower(row.Flag), strings.ToLower(query.Flag))) || (query.TickFrom > 0 && row.Tick < query.TickFrom) || (query.TickTo > 0 && row.Tick > query.TickTo) {
			continue
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].SubmittedAt.Equal(rows[j].SubmittedAt) {
			return rows[i].ID > rows[j].ID
		}
		return rows[i].SubmittedAt.After(rows[j].SubmittedAt)
	})
	limit, offset := submissionPageBounds(query)
	page := apigateway.SubmissionPage{Items: []apigateway.AdminSubmission{}, Limit: limit, Offset: offset, TotalCount: len(rows), HasPrev: offset > 0}
	if offset < len(rows) {
		end := min(offset+limit, len(rows))
		page.Items = rows[offset:end]
		page.HasNext = end < len(rows)
	}
	return page, nil
}

func (s *memoryGameStore) DeleteSubmission(_ context.Context, id int64) (apigateway.SubmissionDeletion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, row := range s.submissions {
		if row.ID != id {
			continue
		}
		result := apigateway.SubmissionDeletion{ID: id, Status: row.Status, Service: row.Service, FirstBloodCleared: row.FirstBlood}
		if row.Status == "accepted" {
			delete(s.accepted, acceptedSubmissionKey(row.Flag, row.TeamID))
			for j, event := range s.attackFeed {
				if event.ID == row.AttackID {
					s.attackFeed = append(s.attackFeed[:j], s.attackFeed[j+1:]...)
					break
				}
			}
			if row.FirstBlood {
				delete(s.firstBloodChallenges, row.ChallengeID)
			}
			s.attackFeedRevision++
			s.recomputeScoreboardLocked()
		}
		s.submissions = append(s.submissions[:i], s.submissions[i+1:]...)
		return result, nil
	}
	return apigateway.SubmissionDeletion{}, errSubmissionNotFound
}

func (s *postgresGameStore) RecordSubmission(ctx context.Context, row apigateway.AdminSubmission) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO submission_events (team_id, attacker, victim, challenge_id, service, flag, tick, status, detail, submitted_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, row.TeamID, row.Attacker, row.Victim, row.ChallengeID, row.Service, row.Flag, row.Tick, row.Status, row.Detail, row.SubmittedAt.UTC())
	return err
}

func submissionPageBounds(query apigateway.SubmissionQuery) (int, int) {
	limit := query.Limit
	if limit <= 0 {
		limit = 25
	}
	if limit > 200 {
		limit = 200
	}
	return limit, max(0, query.Offset)
}

func (s *postgresGameStore) ListSubmissions(ctx context.Context, query apigateway.SubmissionQuery) (apigateway.SubmissionPage, error) {
	limit, offset := submissionPageBounds(query)
	conditions, args := []string{"TRUE"}, []any{}
	for _, filter := range []struct{ column, value string }{{"s.attacker", query.Attacker}, {"s.victim", query.Victim}, {"s.service", query.Service}, {"s.flag", query.Flag}} {
		if filter.value != "" {
			args = append(args, filter.value)
			conditions = append(conditions, fmt.Sprintf("strpos(lower(%s), lower($%d)) > 0", filter.column, len(args)))
		}
	}
	if query.Status != "" {
		args = append(args, query.Status)
		conditions = append(conditions, fmt.Sprintf("s.status = $%d", len(args)))
	}
	if query.TickFrom > 0 {
		args = append(args, query.TickFrom)
		conditions = append(conditions, fmt.Sprintf("s.tick >= $%d", len(args)))
	}
	if query.TickTo > 0 {
		args = append(args, query.TickTo)
		conditions = append(conditions, fmt.Sprintf("s.tick <= $%d", len(args)))
	}
	where := strings.Join(conditions, " AND ")
	page := apigateway.SubmissionPage{Items: []apigateway.AdminSubmission{}, Limit: limit, Offset: offset, HasPrev: offset > 0}
	// #nosec G201 -- Predicates and column names are fixed above; filter values use placeholders.
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM submission_events s WHERE `+where, args...).Scan(&page.TotalCount); err != nil {
		return page, err
	}
	args = append(args, limit, offset)
	// #nosec G201 -- where uses only fixed predicates and placeholders.
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`SELECT s.id, s.team_id, s.attacker, s.victim, s.challenge_id, s.service, s.flag, s.tick, s.status, s.detail, s.submitted_at, COALESCE(a.first_blood,FALSE) FROM submission_events s LEFT JOIN attack_events a ON a.id = s.attack_event_id WHERE %s ORDER BY s.submitted_at DESC, s.id DESC LIMIT $%d OFFSET $%d`, where, len(args)-1, len(args)), args...)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var row apigateway.AdminSubmission
		if err := rows.Scan(&row.ID, &row.TeamID, &row.Attacker, &row.Victim, &row.ChallengeID, &row.Service, &row.Flag, &row.Tick, &row.Status, &row.Detail, &row.SubmittedAt, &row.FirstBlood); err != nil {
			return page, err
		}
		page.Items = append(page.Items, row)
	}
	page.HasNext = offset+len(page.Items) < page.TotalCount
	return page, rows.Err()
}

func (s *postgresGameStore) DeleteSubmission(ctx context.Context, id int64) (apigateway.SubmissionDeletion, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return apigateway.SubmissionDeletion{}, err
	}
	defer tx.Rollback()
	// Same lock as acceptance and tick advance: deletion cannot race a new winner.
	if _, err := tx.ExecContext(ctx, `LOCK TABLE game_ticks IN EXCLUSIVE MODE`); err != nil {
		return apigateway.SubmissionDeletion{}, err
	}
	result := apigateway.SubmissionDeletion{ID: id}
	var flag string
	var teamID int
	var attackID sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT flag, team_id, status, service, attack_event_id FROM submission_events WHERE id=$1 FOR UPDATE`, id).Scan(&flag, &teamID, &result.Status, &result.Service, &attackID)
	if errors.Is(err, sql.ErrNoRows) {
		return result, errSubmissionNotFound
	}
	if err != nil {
		return result, err
	}
	if result.Status == "accepted" {
		if attackID.Valid {
			err = tx.QueryRowContext(ctx, `DELETE FROM attack_events WHERE id=$1 RETURNING first_blood`, attackID.String).Scan(&result.FirstBloodCleared)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return result, err
			}
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM submitted_flags WHERE flag=$1 AND team_id=$2`, flag, teamID); err != nil {
			return result, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE attack_feed_state SET revision=revision+1 WHERE singleton=TRUE`); err != nil {
			return result, err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM submission_events WHERE id=$1`, id); err != nil {
		return result, err
	}
	return result, tx.Commit()
}
