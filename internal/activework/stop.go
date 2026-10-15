package activework

import (
	"fmt"

	"github.com/gopher-launch/concoct/internal/stop"
)

// BeginStop durably records the owned role and attempt before interrupting
// provider execution or cleaning its background terminals.
func (o *Owner) BeginStop(role, threadID, turnID, attemptID string) error {
	st, err := o.currentState()
	if err != nil {
		return err
	}
	if (role != "reagent" && role != "iragent") || threadID == "" || st.RoleIDs[role] != threadID || st.Dispatch == nil || st.Stop != nil {
		return fmt.Errorf("stop has no unique owned role binding")
	}
	bound := false
	for _, attempt := range st.Dispatch.Attempts {
		if attempt.ID == attemptID && string(attempt.Binding.Role) == role && (attempt.Status == "reserved" || attempt.Status == "uncertain") {
			bound = true
		}
	}
	if !bound {
		return fmt.Errorf("stop attempt is not active or uncertain")
	}
	st.Stop = &StopRecord{Role: role, ThreadID: threadID, TurnID: turnID, AttemptID: attemptID, Status: "requested"}
	st.Revision++
	return writeJSON(o.store.statePath(st.RunID), st)
}

func (o *Owner) FinishStop(result stop.Result) error {
	st, err := o.currentState()
	if err != nil {
		return err
	}
	if st.Stop == nil || st.Stop.Status != "requested" || (result.Status != "stopped_safely" && result.Status != "stopped_abnormally") {
		return fmt.Errorf("no pending stop to finalize")
	}
	if len(result.Problem) > 1024 {
		result.Problem = result.Problem[:1024]
	}
	if len(result.Turn.FinalText) > 64*1024 {
		result.Status = "stopped_abnormally"
		result.Problem = "observed provider result exceeds stop evidence limit"
		result.Turn.FinalText = ""
	}
	st.Stop.Status, st.Stop.Problem = result.Status, result.Problem
	st.Stop.ObservedStatus, st.Stop.ObservedFinalText = result.Turn.Status, result.Turn.FinalText
	st.Stop.Owned = append([]stop.Identity(nil), result.Owned...)
	st.Stop.Handles = append([]string(nil), result.Handles...)
	st.Hold = "recovery-required"
	st.PendingRequest = "reconcile stopped attempt " + st.Stop.AttemptID + " before any new dispatch"
	if result.Status == "stopped_abnormally" {
		st.Hold = "abnormal-stop"
		st.PendingRequest = "verify cessation and reconcile stopped attempt " + st.Stop.AttemptID
	}
	st.Revision++
	return writeJSON(o.store.statePath(st.RunID), st)
}

func (o *Owner) CompleteStopReconciliation(attemptID string) error {
	st, err := o.currentState()
	if err != nil {
		return err
	}
	if st.Stop == nil || st.Stop.AttemptID != attemptID || st.Stop.Status != "stopped_safely" || st.Dispatch == nil {
		return fmt.Errorf("stop is not safely reconcilable")
	}
	resolved := false
	for _, attempt := range st.Dispatch.Attempts {
		if attempt.ID == attemptID && (attempt.Status == "completed" || attempt.Status == "reconciled" || attempt.Status == "failed") {
			resolved = true
		}
	}
	if !resolved || len(st.StopHistory) >= st.Dispatch.Limits.Dispatches {
		return fmt.Errorf("stopped attempt is not reconciled within bounds")
	}
	st.StopHistory = append(st.StopHistory, *st.Stop)
	st.Stop = nil
	if st.Hold == "recovery-required" {
		st.Hold, st.PendingRequest = "", ""
	}
	st.Revision++
	return writeJSON(o.store.statePath(st.RunID), st)
}
