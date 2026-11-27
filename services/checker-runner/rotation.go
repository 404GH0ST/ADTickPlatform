package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"

	"adplatform/internal/platform/gamenet"
	"adplatform/internal/services/apigateway"
)

func (s *checkerRunnerServer) handleRotateChecker(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdminAuth(w, r) {
		return
	}
	executor, ok := s.executor.(*dockerCheckerExecutor)
	if !ok {
		http.Error(w, "checker rotation unavailable", http.StatusNotImplemented)
		return
	}
	controller := http.NewResponseController(w)
	if err := controller.EnableFullDuplex(); err != nil {
		http.Error(w, "checker rotation unavailable", http.StatusNotImplemented)
		return
	}
	reader := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	var request apigateway.CheckerRotationRequest
	if err := reader.Decode(&request); err != nil || request.TeamID <= 0 || request.ChallengeID <= 0 ||
		request.PreviousTickID <= 0 || request.PreviousTickID >= request.TickID || request.PreviousFlag == "" ||
		strings.TrimSpace(request.CheckerImage) == "" || strings.TrimSpace(request.Target) == "" ||
		request.TimeoutSeconds < 0 || request.TimeoutSeconds > 120 {
		http.Error(w, "invalid checker rotation", http.StatusBadRequest)
		return
	}
	perPhase := executor.timeout
	if request.TimeoutSeconds > 0 {
		perPhase = time.Duration(request.TimeoutSeconds) * time.Second
	}
	if err := controller.SetReadDeadline(time.Now().Add(2 * perPhase)); err != nil {
		http.Error(w, "checker rotation unavailable", http.StatusNotImplemented)
		return
	}
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.WriteHeader(http.StatusOK)
	if err := controller.Flush(); err != nil {
		return
	}
	writer := json.NewEncoder(w)
	send := func(kind string, result apigateway.CheckerPhaseResult) error {
		if err := writer.Encode(apigateway.CheckerRotationEvent{Kind: kind, Result: result}); err != nil {
			return err
		}
		return controller.Flush()
	}
	acknowledge := func(previous apigateway.CheckerPhaseResult) error {
		if err := send("retention", previous); err != nil {
			return err
		}
		var allow bool
		if err := reader.Decode(&allow); err != nil || !allow {
			return errors.New("previous-flag verdict was not acknowledged")
		}
		return nil
	}
	if err := executor.rotateChecker(r.Context(), request, perPhase, acknowledge, func(result apigateway.CheckerPhaseResult) error {
		return send("phase", result)
	}); err == nil {
		_ = send("done", apigateway.CheckerPhaseResult{})
	}
}

func (e *dockerCheckerExecutor) rotateChecker(
	ctx context.Context, request apigateway.CheckerRotationRequest, perPhase time.Duration,
	acknowledge func(apigateway.CheckerPhaseResult) error,
	report func(apigateway.CheckerPhaseResult) error,
) error {
	runCtx, cancel := context.WithTimeout(ctx, 2*perPhase)
	defer cancel()
	targetHost, _ := apigateway.ParseEndpoint(request.Target)
	plan, err := gamenet.PlanDockerNetwork(e.network, e.networkLayout, firstNonEmpty(request.TargetIP, request.TargetHost, targetHost))
	if err != nil {
		return err
	}
	network, source := e.resolveRunNetwork(runCtx, plan.Name, request.SourceIP)
	request.SourceIP = source
	request.Phases = []string{"put", "get"}
	name := dockerRunName("adchkr", request.ChallengeID, request.TeamID, request.TickID)
	nonce := rand.Text()
	env := checkerRunEnv(request.Flag, request.Metadata, request.CheckerToken, nonce)
	env = append(env, "AD_PREVIOUS_FLAG="+request.PreviousFlag, "AD_PREVIOUS_META="+request.PreviousMetadata,
		"AD_PREVIOUS_TICK="+strconv.Itoa(request.PreviousTickID))
	args := buildDockerCheckerBatchExecuteArgs(network, name, e.security, request.CheckerBatchExecutionRequest)
	args = slices.Insert(args, 1, "-i")
	args = slices.Insert(args, slices.Index(args, "--entrypoint"), "-e", "AD_PREVIOUS_FLAG", "-e", "AD_PREVIOUS_META", "-e", "AD_PREVIOUS_TICK")
	args[len(args)-1] = checkerRotationScript()

	cmd := exec.CommandContext(runCtx, e.binary, args...) // #nosec G204 -- same operator-controlled Docker invocation as execute-batch.
	cmd.Env = append(cmd.Environ(), env...)
	input, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		_ = input.Close()
		return err
	}
	if err = cmd.Start(); err != nil {
		_ = input.Close()
		_ = output.Close()
		return err
	}
	waited := false
	defer func() {
		_ = input.Close()
		if !waited {
			cancel()
			_ = cmd.Wait()
		}
		if runCtx.Err() != nil {
			e.forceRemove(name)
		}
	}()

	reader := bufio.NewReader(output)
	readPhase := func(expected string) (apigateway.CheckerPhaseResult, error) {
		phase, frame, err := readCheckerBatchFrame(reader)
		if err != nil || phase != expected {
			return apigateway.CheckerPhaseResult{}, errors.New("incomplete checker rotation phase")
		}
		result := apigateway.CheckerPhaseResult{Phase: phase, ExitCode: frame.exitCode, CheckedAt: time.Now().UTC().Format(time.RFC3339Nano)}
		if frame.exitCode == 0 {
			result.Status = "success"
			result.Message = "checker phase completed successfully."
			filtered, _, _ := parseCheckerServiceStateOutput([]byte(frame.output), false, nonce)
			result.Output = truncateOutput(filtered, 65536)
		} else {
			result.Status = "failed"
			result.Message = apigateway.CheckerExitFailureMessage(frame.exitCode)
		}
		return result, nil
	}

	retentionDeadline := time.AfterFunc(perPhase, cancel)
	previous, err := readPhase("get")
	retentionDeadline.Stop()
	if err != nil {
		return err
	}
	if err := acknowledge(previous); err != nil {
		return err
	}
	if _, err := io.WriteString(input, "continue\n"); err != nil {
		return err
	}
	_ = input.Close()
	for _, phase := range request.Phases {
		result, err := readPhase(phase)
		if err != nil {
			return err
		}
		if err := report(result); err != nil {
			return err
		}
		if result.Status != "success" {
			break
		}
	}
	// Consume the expected EOF before Wait closes the pipe. Anything else is a
	// protocol error and cancels the container.
	if _, err := reader.ReadByte(); err != io.EOF {
		return errors.New("extra checker rotation output")
	}
	err = cmd.Wait()
	waited = true
	return err
}

func checkerRotationScript() string {
	// Reuse the same entrypoint resolution and framing as ordinary batches. The
	// child checker never inherits the control stream used for the permit.
	return checkerBatchEntrypoint + `
(
  out=$(AD_PHASE=get AD_FLAG="$AD_PREVIOUS_FLAG" AD_METADATA="$AD_PREVIOUS_META" AD_TICK_ID="$AD_PREVIOUS_TICK" run_phase get </dev/null 2>&1)
  code=$?
  byte_count=$(printf '%s' "$out" | wc -c)
  printf '__ADP_PHASE_FRAME__ get %s %s\n%s\n' "$code" "$byte_count" "$out"
)
IFS= read -r permit && [ "$permit" = continue ] || exit 0
unset AD_PREVIOUS_FLAG AD_PREVIOUS_META AD_PREVIOUS_TICK
` + checkerBatchPhases
}
