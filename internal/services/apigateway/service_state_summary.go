package apigateway

import "strings"

type GameServiceStateSummary struct {
	Status  string
	Phase   string
	TickID  int
	Message string
}

type tickPhaseState struct {
	run     *GameCheckerRun
	success bool
	failed  bool
}

func SummarizeCheckerRunsForTick(runs []GameCheckerRun, tickID int) GameServiceStateSummary {
	if len(runs) == 0 {
		return GameServiceStateSummary{
			Status:  "unknown",
			TickID:  tickID,
			Message: "awaiting first checker run",
		}
	}

	phases := collectTickPhaseStates(runs)
	putOK := phases["put"].success
	getOK := phases["get"].success
	checkOK := phases["check"].success

	summary := GameServiceStateSummary{TickID: tickID}
	switch {
	case putOK && getOK && checkOK:
		summary.Status = "ok"
		summary.Phase = "check"
		summary.Message = "service passed storage, retrieval, and functionality checks"
	case !putOK && getOK && checkOK:
		summary.Status = "recovering"
		summary.Phase = "put"
		summary.Message = phaseMessage(phases["put"], "flag storage failed but retrieval and functionality still passed")
	case putOK && !getOK:
		summary.Status = "flag_not_found"
		summary.Phase = "get"
		summary.Message = phaseMessage(phases["get"], "checker could not retrieve the stored flag")
	case putOK && getOK && !checkOK:
		summary.Status = "faulty"
		summary.Phase = "check"
		summary.Message = phaseMessage(phases["check"], "service functionality check failed")
	default:
		summary.Status = "down"
		summary.Phase = dominantFailurePhase(phases)
		summary.Message = dominantFailureMessage(phases)
	}
	if strings.TrimSpace(summary.Message) == "" {
		summary.Message = "service state unavailable for the latest checker tick"
	}
	return summary
}

func collectTickPhaseStates(runs []GameCheckerRun) map[string]tickPhaseState {
	phases := map[string]tickPhaseState{}
	for i := range runs {
		run := &runs[i]
		phase := strings.ToLower(strings.TrimSpace(run.Phase))
		if phase == "" {
			continue
		}
		state := phases[phase]
		if shouldReplacePhaseRun(state.run, run) {
			state.run = run
		}
		switch strings.ToLower(strings.TrimSpace(run.Status)) {
		case "success":
			state.success = true
		case "failed":
			state.failed = true
		default:
			if state.run == nil {
				state.run = run
			}
		}
		phases[phase] = state
	}
	return phases
}

func shouldReplacePhaseRun(current, candidate *GameCheckerRun) bool {
	if candidate == nil {
		return false
	}
	if current == nil {
		return true
	}
	return statusRank(candidate.Status) > statusRank(current.Status)
}

func statusRank(status string) int {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "failed":
		return 3
	case "skipped":
		return 2
	case "success":
		return 1
	default:
		return 0
	}
}

func phaseMessage(state tickPhaseState, fallback string) string {
	if state.run != nil && strings.TrimSpace(state.run.Message) != "" {
		return strings.TrimSpace(state.run.Message)
	}
	return fallback
}

func dominantFailurePhase(phases map[string]tickPhaseState) string {
	for _, phase := range []string{"put", "get", "check"} {
		state := phases[phase]
		if state.failed || (state.run != nil && !state.success) {
			return phase
		}
	}
	for _, phase := range []string{"put", "get", "check"} {
		if phases[phase].run != nil {
			return phase
		}
	}
	return ""
}

func dominantFailureMessage(phases map[string]tickPhaseState) string {
	for _, phase := range []string{"put", "get", "check"} {
		state := phases[phase]
		if state.failed || (state.run != nil && !state.success) {
			return phaseMessage(state, "service did not complete the latest checker cycle")
		}
	}
	return "service did not complete the latest checker cycle"
}
