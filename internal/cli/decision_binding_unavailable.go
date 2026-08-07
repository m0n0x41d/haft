package cli

type decisionBindingUnavailableError struct{}

func (decisionBindingUnavailableError) Error() string {
	return "operator_confirmation_required: MCP and generic internal-helper calls cannot institute a DecisionRecord because they cannot verify conversational provenance; a supported host must recognize one direct, unambiguous operator request and route the exact reviewed payload to `haft artifact create decision.decide --input-file <file>` with host_routed_operator_request provenance"
}

var errDecisionBindingUnavailable = decisionBindingUnavailableError{}

func rejectUnverifiedMCPDecisionBinding() error {
	return errDecisionBindingUnavailable
}
