package artifact

func completeDecision(input DecideInput) DecideInput {
	if input.SelectionPolicy == "" {
		input.SelectionPolicy = "Prefer the option that best satisfies the active acceptance criteria with the least avoidable complexity."
	}
	if input.CounterArgument == "" {
		input.CounterArgument = "The chosen option could fail if the assumptions behind the current comparison do not survive real workload conditions."
	}
	if len(input.WhyNotOthers) == 0 {
		input.WhyNotOthers = []RejectionReason{{
			Variant: "Fallback alternative",
			Reason:  "It adds more cost or complexity without enough compensating value for the current scope.",
		}}
	}
	if input.WeakestLink == "" {
		input.WeakestLink = "Operational confidence still depends on limited production-grade evidence."
	}
	if input.Rollback == nil {
		input.Rollback = &RollbackSpec{}
	}
	if len(input.Rollback.Triggers) == 0 {
		input.Rollback.Triggers = []string{"Primary acceptance check regresses after rollout"}
	}

	return input
}
