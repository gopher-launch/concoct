package activework

import (
	"fmt"
	"time"

	"github.com/gopher-launch/concoct/internal/dispatch"
)

// ConfigureDispatch records resolved policy once for this run. Reopening the
// store or claiming a selected task retains the same limits and profiles.
func (o *Owner) ConfigureDispatch(policy dispatch.State) error {
	if err := policy.Validate(); err != nil {
		return err
	}
	st, err := o.currentState()
	if err != nil {
		return err
	}
	if st.Dispatch != nil {
		return fmt.Errorf("dispatch policy already configured")
	}
	copy, err := policy.Clone()
	if err != nil {
		return err
	}
	if len(copy.Attempts) != 0 || len(copy.Assessments) != 0 || copy.Pending != nil {
		return fmt.Errorf("new dispatch policy has prior consumption")
	}
	st.Dispatch = &copy
	st.Revision++
	return writeJSON(o.store.statePath(st.RunID), st)
}

func (o *Owner) dispatchState() (state, dispatch.State, error) {
	st, err := o.currentState()
	if err != nil {
		return state{}, dispatch.State{}, err
	}
	if st.Dispatch == nil {
		return state{}, dispatch.State{}, fmt.Errorf("dispatch policy is not configured")
	}
	copy, err := st.Dispatch.Clone()
	return st, copy, err
}

func (o *Owner) saveDispatch(st state, policy dispatch.State) error {
	st.Dispatch = &policy
	st.Revision++
	return writeJSON(o.store.statePath(st.RunID), st)
}

// ReserveDispatch persists a turn attempt or an authorization wait before the
// caller can send a provider request. A plain retry never grants a pending gate.
func (o *Owner) ReserveDispatch(id string, binding dispatch.Binding, now time.Time) (dispatch.Decision, error) {
	st, policy, err := o.dispatchState()
	if err != nil {
		return dispatch.Decision{}, err
	}
	if binding.TaskID != st.TaskID {
		return dispatch.Decision{}, fmt.Errorf("dispatch task differs from active binding")
	}
	decision, err := policy.Reserve(id, binding, now)
	if err != nil {
		return dispatch.Decision{}, err
	}
	if err := o.saveDispatch(st, policy); err != nil {
		return dispatch.Decision{}, err
	}
	return decision, nil
}

func (o *Owner) AuthorizeDispatch(id string) error {
	st, policy, err := o.dispatchState()
	if err != nil {
		return err
	}
	if err := policy.Authorize(id); err != nil {
		return err
	}
	return o.saveDispatch(st, policy)
}

func (o *Owner) FinishDispatch(id, status string, tokens *dispatch.TokenReceipt) error {
	st, policy, err := o.dispatchState()
	if err != nil {
		return err
	}
	if err := policy.Finish(id, status, tokens); err != nil {
		return err
	}
	return o.saveDispatch(st, policy)
}

func (o *Owner) ReconcileDispatch(id, evidence string) error {
	st, policy, err := o.dispatchState()
	if err != nil {
		return err
	}
	if err := policy.Reconcile(id, evidence); err != nil {
		return err
	}
	return o.saveDispatch(st, policy)
}

func (o *Owner) RecordAssessment(id, attemptID string) error {
	st, policy, err := o.dispatchState()
	if err != nil {
		return err
	}
	if err := policy.RecordAssessment(id, attemptID); err != nil {
		return err
	}
	return o.saveDispatch(st, policy)
}

func (o *Owner) ExtendDispatchLimits(next dispatch.Limits) error {
	st, policy, err := o.dispatchState()
	if err != nil {
		return err
	}
	if err := policy.Extend(next); err != nil {
		return err
	}
	return o.saveDispatch(st, policy)
}

func (o *Owner) ObserveQuota(value dispatch.QuotaObservation, now time.Time) error {
	st, policy, err := o.dispatchState()
	if err != nil {
		return err
	}
	if err := policy.ObserveQuota(value, now); err != nil {
		return err
	}
	return o.saveDispatch(st, policy)
}
