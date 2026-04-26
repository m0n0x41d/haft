package workcommission

import "strings"

type State string

const (
	StateDraft                       State = "draft"
	StateQueued                      State = "queued"
	StateReady                       State = "ready"
	StatePreflighting                State = "preflighting"
	StateRunning                     State = "running"
	StateBlockedStale                State = "blocked_stale"
	StateBlockedPolicy               State = "blocked_policy"
	StateBlockedConflict             State = "blocked_conflict"
	StateNeedsHumanReview            State = "needs_human_review"
	StateCompleted                   State = "completed"
	StateCompletedWithProjectionDebt State = "completed_with_projection_debt"
	StateFailed                      State = "failed"
	StateCancelled                   State = "cancelled"
	StateExpired                     State = "expired"
)

var knownStates = map[State]struct{}{
	StateDraft:                       {},
	StateQueued:                      {},
	StateReady:                       {},
	StatePreflighting:                {},
	StateRunning:                     {},
	StateBlockedStale:                {},
	StateBlockedPolicy:               {},
	StateBlockedConflict:             {},
	StateNeedsHumanReview:            {},
	StateCompleted:                   {},
	StateCompletedWithProjectionDebt: {},
	StateFailed:                      {},
	StateCancelled:                   {},
	StateExpired:                     {},
}

var runnableStates = map[State]struct{}{
	StateQueued: {},
	StateReady:  {},
}

var executingStates = map[State]struct{}{
	StatePreflighting: {},
	StateRunning:      {},
}

var completionStates = map[State]struct{}{
	StateCompleted:                   {},
	StateCompletedWithProjectionDebt: {},
}

var terminalStates = map[State]struct{}{
	StateCompleted:                   {},
	StateCompletedWithProjectionDebt: {},
	StateCancelled:                   {},
	StateExpired:                     {},
}

var recoverableStates = map[State]struct{}{
	StateQueued:           {},
	StateReady:            {},
	StatePreflighting:     {},
	StateRunning:          {},
	StateBlockedStale:     {},
	StateBlockedPolicy:    {},
	StateBlockedConflict:  {},
	StateNeedsHumanReview: {},
	StateFailed:           {},
}

var operatorDecisionStates = map[State]struct{}{
	StateBlockedStale:     {},
	StateBlockedPolicy:    {},
	StateBlockedConflict:  {},
	StateNeedsHumanReview: {},
	StateFailed:           {},
}

func NormalizeState(value string) State {
	return State(strings.TrimSpace(value))
}

func IsKnownState(value string) bool {
	state := NormalizeState(value)
	return stateIn(state, knownStates)
}

func IsRunnableState(value string) bool {
	state := NormalizeState(value)
	return stateIn(state, runnableStates)
}

func IsExecutingState(value string) bool {
	state := NormalizeState(value)
	return stateIn(state, executingStates)
}

func IsCompletionState(value string) bool {
	state := NormalizeState(value)
	return stateIn(state, completionStates)
}

func IsTerminalState(value string) bool {
	state := NormalizeState(value)
	return stateIn(state, terminalStates)
}

func IsRecoverableState(value string) bool {
	state := NormalizeState(value)
	return stateIn(state, recoverableStates)
}

func RequiresOperatorDecisionState(value string) bool {
	state := NormalizeState(value)
	return stateIn(state, operatorDecisionStates)
}

func SatisfiesDependencyState(value string) bool {
	return IsCompletionState(value)
}

func RecoverableStateValues() []string {
	return stateValues([]State{
		StateQueued,
		StateReady,
		StatePreflighting,
		StateRunning,
		StateBlockedStale,
		StateBlockedPolicy,
		StateBlockedConflict,
		StateNeedsHumanReview,
		StateFailed,
	})
}

func CancellableStateValues() []string {
	return stateValues([]State{
		StateDraft,
		StateQueued,
		StateReady,
		StatePreflighting,
		StateRunning,
		StateBlockedStale,
		StateBlockedPolicy,
		StateBlockedConflict,
		StateNeedsHumanReview,
		StateFailed,
	})
}

func stateValues(states []State) []string {
	values := make([]string, 0, len(states))

	for _, state := range states {
		values = append(values, string(state))
	}

	return values
}

func stateIn(state State, states map[State]struct{}) bool {
	_, ok := states[state]
	return ok
}
