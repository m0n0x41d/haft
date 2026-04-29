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

type DeliveryPolicy string

const (
	DeliveryPolicyWorkspacePatchManual     DeliveryPolicy = "workspace_patch_manual"
	DeliveryPolicyWorkspacePatchAutoOnPass DeliveryPolicy = "workspace_patch_auto_on_pass"
)

type DeliveryVerdict string

const (
	DeliveryVerdictPass    DeliveryVerdict = "pass"
	DeliveryVerdictNonPass DeliveryVerdict = "non_pass"
)

type DeliveryGate string

const (
	DeliveryGateAllowed DeliveryGate = "allowed"
	DeliveryGateBlocked DeliveryGate = "blocked"
	DeliveryGateMissing DeliveryGate = "missing"
)

type DeliveryAction string

const (
	DeliveryActionAutoApply   DeliveryAction = "auto_apply"
	DeliveryActionManualApply DeliveryAction = "manual_apply"
)

type DeliveryDecision struct {
	Action    DeliveryAction
	AutoApply bool
	Reason    string
}

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

func NormalizeDeliveryPolicy(value string) DeliveryPolicy {
	switch DeliveryPolicy(strings.TrimSpace(value)) {
	case DeliveryPolicyWorkspacePatchAutoOnPass:
		return DeliveryPolicyWorkspacePatchAutoOnPass
	default:
		return DeliveryPolicyWorkspacePatchManual
	}
}

func NormalizeDeliveryVerdict(value string) DeliveryVerdict {
	switch strings.TrimSpace(value) {
	case "pass", "completed":
		return DeliveryVerdictPass
	default:
		return DeliveryVerdictNonPass
	}
}

func NormalizeDeliveryGate(value string) DeliveryGate {
	switch DeliveryGate(strings.TrimSpace(value)) {
	case DeliveryGateAllowed:
		return DeliveryGateAllowed
	case DeliveryGateBlocked:
		return DeliveryGateBlocked
	default:
		return DeliveryGateMissing
	}
}

func DeliveryAfterLocalEvidence(
	policy DeliveryPolicy,
	verdict DeliveryVerdict,
	gate DeliveryGate,
) DeliveryDecision {
	if verdict != DeliveryVerdictPass {
		return manualDeliveryDecision("verdict_not_pass")
	}
	if policy != DeliveryPolicyWorkspacePatchAutoOnPass {
		return manualDeliveryDecision("delivery_policy_manual")
	}
	// V3 invariant (dec-20260428-harness-drain-v3-16bf21f3):
	// AutonomyEnvelope evaluates at WorkCommission creation, preflight, and
	// execute — unchanged. This decision adds NO envelope evaluation at apply.
	// Reaching terminal+pass means envelope did not block at the earlier
	// phases; the apply gate is purely (policy + verdict). An explicitly
	// blocked envelope still keeps the manual path because it represents a
	// concrete operator decision, not a missing snapshot.
	if gate == DeliveryGateBlocked {
		return manualDeliveryDecision(deliveryGateManualReason(gate))
	}

	return DeliveryDecision{
		Action:    DeliveryActionAutoApply,
		AutoApply: true,
		Reason:    "policy_auto_on_pass_and_verdict_pass",
	}
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

func manualDeliveryDecision(reason string) DeliveryDecision {
	return DeliveryDecision{
		Action:    DeliveryActionManualApply,
		AutoApply: false,
		Reason:    reason,
	}
}

func deliveryGateManualReason(gate DeliveryGate) string {
	switch gate {
	case DeliveryGateBlocked:
		return "autonomy_envelope_blocked"
	case DeliveryGateMissing:
		return "autonomy_envelope_missing"
	default:
		return "autonomy_envelope_not_allowed"
	}
}
