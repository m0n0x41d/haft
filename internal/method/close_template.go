package method

type CloseTemplate struct {
	Action       string                    `json:"action"`
	PullID       string                    `json:"pull_id"`
	ChangedFiles []string                  `json:"changed_files"`
	GateResults  []CloseTemplateGateResult `json:"gate_results"`
	Verification CloseTemplateVerification `json:"verification"`
	Waivers      []Waiver                  `json:"waivers"`
}

type CloseTemplateGateResult struct {
	GateID       string   `json:"gate_id"`
	Status       string   `json:"status"`
	EvidenceRefs []string `json:"evidence_refs"`
}

type CloseTemplateVerification struct {
	Commands  []string `json:"commands"`
	Result    string   `json:"result"`
	OutputRef string   `json:"output_ref"`
}

func BuildCloseTemplate(run MethodRun) CloseTemplate {
	return CloseTemplate{
		Action:       "close",
		PullID:       run.ID,
		ChangedFiles: []string{"<changed-file>"},
		GateResults:  closeTemplateGateResults(run),
		Verification: CloseTemplateVerification{
			Commands:  []string{"<verification-command>"},
			Result:    "<pass|partial|failed>",
			OutputRef: "<optional-output-ref>",
		},
		Waivers: []Waiver{},
	}
}

func closeTemplateGateResults(run MethodRun) []CloseTemplateGateResult {
	var results []CloseTemplateGateResult
	for _, card := range run.Methods {
		for _, gate := range card.HardGates {
			results = append(results, CloseTemplateGateResult{
				GateID:       gate.ID,
				Status:       "satisfied",
				EvidenceRefs: closeTemplateEvidenceRefs(gate),
			})
		}
	}
	return results
}

func closeTemplateEvidenceRefs(gate Gate) []string {
	if len(gate.RequiredEvidence) == 0 {
		return []string{}
	}
	return []string{"<evidence-ref>"}
}

func CloseInputShapeHint() string {
	return `expected gate_results[] shape: {"gate_id":"<hard-gate-id>","status":"satisfied","evidence_refs":["<evidence-ref>"]}; waiver shape: {"gate_id":"<hard-gate-id>","reason":"<why waived>"}`
}
