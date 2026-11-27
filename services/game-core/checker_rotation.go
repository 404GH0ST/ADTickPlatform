package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"

	"adplatform/internal/services/apigateway"
)

var errRotationUnavailable = errors.New("checker rotation unavailable before write authorization")

const checkerHealthTimeoutSeconds = 30

type rotationCheckerClient interface {
	Rotate(context.Context, apigateway.CheckerRotationRequest, func(apigateway.CheckerPhaseResult) error) (apigateway.CheckerBatchExecutionResult, error)
}

func (c *httpCheckerClient) Rotate(
	ctx context.Context, request apigateway.CheckerRotationRequest, persist func(apigateway.CheckerPhaseResult) error,
) (result apigateway.CheckerBatchExecutionResult, err error) {
	// Before sending the permit, the checker can only perform a read. An
	// unavailable runner can safely fall back to the older checker execution.
	// After sending it, never replay a write with an uncertain outcome.
	admitted := false
	defer func() {
		if err != nil && !admitted {
			err = errors.Join(errRotationUnavailable, err)
		}
	}()
	payload, err := json.Marshal(request)
	if err != nil {
		return result, err
	}
	input, permit := io.Pipe()
	defer permit.Close()
	defer input.Close()
	body := struct {
		io.Reader
		io.Closer
	}{io.MultiReader(bytes.NewReader(append(payload, '\n')), input), input}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/v1/checkers/rotate", body) // #nosec G704 -- base URL is validated service configuration.
	if err != nil {
		return result, err
	}
	req.Header.Set("Content-Type", "application/x-ndjson")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.client.Do(req) // #nosec G704 -- request targets a validated internal checker service origin.
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return result, errors.New("checker runner rejected the rotation")
	}

	result.TeamID, result.ChallengeID = request.TeamID, request.ChallengeID
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 2<<20))
	phases := []string{"put", "get"}
	for {
		var event apigateway.CheckerRotationEvent
		if err := decoder.Decode(&event); err != nil {
			return result, err
		}
		switch event.Kind {
		case "retention":
			if admitted || event.Result.Phase != "get" || !validRotationResult(event.Result) {
				return result, errors.New("unexpected previous-flag verdict")
			}
			if err := persist(event.Result); err != nil {
				return result, err
			}
			admitted = true
			if _, err := io.WriteString(permit, "true\n"); err != nil {
				return result, err
			}
			_ = permit.Close()
		case "phase":
			if !admitted || len(phases) == 0 || event.Result.Phase != phases[0] || !validRotationResult(event.Result) {
				return result, errors.New("unexpected checker rotation phase")
			}
			result.Phases = append(result.Phases, event.Result)
			phases = phases[1:]
			if event.Result.Status == "failed" {
				phases = nil
			}
		case "done":
			if !admitted || len(phases) != 0 {
				return result, errors.New("incomplete checker rotation")
			}
			return result, nil
		default:
			return result, errors.New("invalid checker rotation event")
		}
	}
}

func validRotationResult(result apigateway.CheckerPhaseResult) bool {
	return (result.Status == "success" && result.ExitCode == 0) ||
		(result.Status == "failed" && result.ExitCode > 0 && result.ExitCode <= 255)
}

func (c *playerSourceChecker) Rotate(ctx context.Context, request apigateway.CheckerRotationRequest, persist func(apigateway.CheckerPhaseResult) error) (apigateway.CheckerBatchExecutionResult, error) {
	inner, ok := c.inner.(rotationCheckerClient)
	if !ok {
		return apigateway.CheckerBatchExecutionResult{}, errRotationUnavailable
	}
	source, release := c.acquire(ctx)
	defer release()
	request.SourceIP = source
	return inner.Rotate(ctx, request, persist)
}

func (s *gameCoreServer) rotationFirst() bool {
	_, supportsRotation := s.checker.(rotationCheckerClient)
	return supportsRotation && slices.Equal(s.checkerPhases, []string{"put", "get", "check"})
}

// The caller holds the target lock until the new flag and its read-back verdict
// have been recorded. Release it before queuing health so resets need not wait
// for an unrelated target's health check to begin.
func (s *gameCoreServer) rotateCheckerTarget(
	ctx, execCtx context.Context, target checkerTarget, previous issuedFlagRecord,
	request apigateway.CheckerBatchExecutionRequest,
) (checkerTargetTickResult, error) {
	var rotation apigateway.CheckerBatchExecutionResult
	var execErr error
	var storeErr error
	phases := request.Phases[:2]
	rotationRequest := request
	rotationRequest.Phases = phases
	if previous.CheckerToken == "" {
		rotation, execErr = s.checker.ExecuteBatch(execCtx, rotationRequest)
	} else {
		var staleVerdict bool
		rotation, execErr = s.checker.(rotationCheckerClient).Rotate(execCtx, apigateway.CheckerRotationRequest{
			CheckerBatchExecutionRequest: rotationRequest,
			PreviousTickID:               previous.IssuedTick,
			PreviousFlag:                 previous.Flag,
			PreviousMetadata:             previous.Metadata,
		}, func(result apigateway.CheckerPhaseResult) error {
			var applied bool
			applied, storeErr = s.recordFlagRetention(ctx, target, previous, result, nil)
			if storeErr != nil {
				return storeErr
			}
			if !applied {
				staleVerdict = true
				return errors.New("previous flag is no longer current")
			}
			return execCtx.Err()
		})
		if storeErr != nil {
			return checkerTargetTickResult{}, storeErr
		}
		if errors.Is(execErr, errRotationUnavailable) && !staleVerdict && execCtx.Err() == nil {
			if err := s.checkFlagRetention(ctx, execCtx, target, previous); err != nil {
				return checkerTargetTickResult{}, err
			}
			batch, err := s.checker.ExecuteBatch(execCtx, request)
			return s.recordCheckerPhases(ctx, execCtx, request.TickID, target, request.Flag, request.Phases, batch, err)
		}
	}

	var health *apigateway.CheckerExecutionRequest
	if len(rotation.Phases) == 2 && rotation.Phases[0].Status == "success" && rotation.Phases[1].Status == "success" {
		health = &apigateway.CheckerExecutionRequest{
			ChallengeID: request.ChallengeID, TeamID: request.TeamID,
			TeamName: request.TeamName, ChallengeName: request.ChallengeName,
			CheckerImage: request.CheckerImage, Phase: "check", Target: request.Target,
			TargetHost: request.TargetHost, TargetIP: request.TargetIP, TargetPort: request.TargetPort,
			TickID: request.TickID, Flag: request.Flag, CheckerToken: request.CheckerToken,
			Metadata:       rotation.Phases[1].Output,
			TimeoutSeconds: max(checkerHealthTimeoutSeconds, request.TimeoutSeconds),
		}
	} else {
		phases = request.Phases // Record the skipped health phase with the failed rotation.
	}
	result, err := s.recordCheckerPhases(ctx, execCtx, request.TickID, target, request.Flag, phases, rotation, execErr)
	if err == nil {
		result.health = health
	}
	return result, err
}

func (s *gameCoreServer) runCheckerHealth(ctx, execCtx context.Context, target checkerTarget, request apigateway.CheckerExecutionRequest) (checkerTargetTickResult, error) {
	record := func(result apigateway.CheckerExecutionResult, execErr error) (checkerTargetTickResult, error) {
		var batch apigateway.CheckerBatchExecutionResult
		if execErr == nil {
			batch.Phases = []apigateway.CheckerPhaseResult{{
				Phase: request.Phase, Status: result.Status, ExitCode: result.ExitCode,
				CheckedAt: result.CheckedAt, Message: result.Message, Output: result.Output,
				ServiceState: result.ServiceState, StateMessage: result.StateMessage,
			}}
		}
		return s.recordCheckerPhases(ctx, execCtx, request.TickID, target, request.Flag, []string{request.Phase}, batch, execErr)
	}
	if err := execCtx.Err(); err != nil {
		return record(apigateway.CheckerExecutionResult{}, err)
	}
	unlock, err := s.store.LockCheckerTarget(ctx, target.TeamID, target.ChallengeID)
	if err != nil {
		return checkerTargetTickResult{}, err
	}
	defer unlock()
	current, err := s.store.CheckerTargetCurrent(ctx, target)
	if err != nil {
		return checkerTargetTickResult{}, err
	}
	if !current {
		return record(apigateway.CheckerExecutionResult{Status: "failed", ExitCode: -1, Message: "checker target changed or unavailable during tick"}, nil)
	}
	result, err := s.checker.Execute(execCtx, request)
	return record(result, err)
}
