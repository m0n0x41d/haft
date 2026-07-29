package cli

type decisionBindingUnavailableError struct{}

func (decisionBindingUnavailableError) Error() string {
	return "operator_confirmation_required: MCP and generic internal-helper calls cannot institute a DecisionRecord; explicitly invoke h-decide and use `haft artifact create decision.decide --input-file <file>`; default explicit_h_decide trusts that external invocation by project policy, but the kernel neither observes it nor records a durable authorization receipt; strict_cli_speech_act adds a durable controlling-terminal SpeechAct on /dev/tty"
}

var errDecisionBindingUnavailable = decisionBindingUnavailableError{}

func rejectNonManualDecisionBinding() error {
	return errDecisionBindingUnavailable
}
