package activework

import (
	"fmt"
	"slices"
)

func (o *Owner) BeginCandidateFreeze(intent CandidateFreezeIntent) error {
	st, err := o.currentState()
	if err != nil {
		return err
	}
	if st.TaskID == "" || (st.Phase != "engineering" && st.Phase != "correcting") || st.Hold != "" || intent.TurnID == "" || !isDigest(intent.CheckpointSHA256) || len(intent.ChangedPaths) == 0 {
		return fmt.Errorf("candidate freeze intent is not ready")
	}
	checkpoint, err := o.Checkpoint()
	if err != nil {
		return err
	}
	if digest([]byte(checkpoint)) != intent.CheckpointSHA256 {
		return fmt.Errorf("candidate freeze checkpoint changed")
	}
	if st.Candidate != nil && st.Candidate.Commit != intent.PriorCommit {
		return fmt.Errorf("candidate freeze prior revision changed")
	}
	if st.Candidate == nil && intent.PriorCommit != "" {
		return fmt.Errorf("candidate freeze invented prior revision")
	}
	if st.FreezeIntent != nil {
		prior := st.FreezeIntent
		if prior.TurnID == intent.TurnID && prior.CheckpointSHA256 == intent.CheckpointSHA256 && prior.PriorCommit == intent.PriorCommit && slices.Equal(prior.ChangedPaths, intent.ChangedPaths) && slices.Equal(prior.GeneratedOutputs, intent.GeneratedOutputs) {
			return nil
		}
		return fmt.Errorf("candidate freeze has different pending intent")
	}
	st.FreezeIntent = &intent
	st.Revision++
	return writeJSON(o.store.statePath(st.RunID), st)
}
