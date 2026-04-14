package artifact

import (
	"context"
	"fmt"
	"strings"
)

// VerificationPassInput captures the facts recorded after a successful
// post-execution verification pass.
type VerificationPassInput struct {
	DecisionRef   string   `json:"decision_ref"`
	AffectedFiles []string `json:"affected_files,omitempty"`
	CarrierRef    string   `json:"carrier_ref,omitempty"`
	Summary       string   `json:"summary,omitempty"`
}

// VerificationPassResult returns the baseline snapshot and linked evidence item.
type VerificationPassResult struct {
	Baseline []AffectedFile `json:"baseline"`
	Evidence *EvidenceItem  `json:"evidence"`
}

// RecordVerificationPass baselines the decision's affected files in the current
// worktree and records a same-context verification evidence item.
func RecordVerificationPass(
	ctx context.Context,
	store ArtifactStore,
	projectRoot string,
	input VerificationPassInput,
) (*VerificationPassResult, error) {
	decisionRef := strings.TrimSpace(input.DecisionRef)
	if decisionRef == "" {
		return nil, fmt.Errorf("decision_ref is required")
	}

	projectRoot = strings.TrimSpace(projectRoot)
	if projectRoot == "" {
		return nil, fmt.Errorf("project_root is required")
	}

	decision, err := store.Get(ctx, decisionRef)
	if err != nil {
		return nil, fmt.Errorf("decision %s not found: %w", decisionRef, err)
	}
	if decision.Meta.Kind != KindDecisionRecord {
		return nil, fmt.Errorf("%s is %s, not DecisionRecord", decisionRef, decision.Meta.Kind)
	}

	baseline, err := Baseline(ctx, store, projectRoot, BaselineInput{
		DecisionRef:   decisionRef,
		AffectedFiles: input.AffectedFiles,
	})
	if err != nil {
		return nil, err
	}

	evidence, err := AttachEvidence(ctx, store, EvidenceInput{
		ArtifactRef:     decisionRef,
		Content:         verificationPassEvidenceContent(decision, baseline, input.Summary),
		Type:            "audit",
		Verdict:         "supports",
		CarrierRef:      strings.TrimSpace(input.CarrierRef),
		CongruenceLevel: 3,
		FormalityLevel:  2,
		ValidUntil:      decision.Meta.ValidUntil,
	})
	if err != nil {
		return nil, err
	}

	return &VerificationPassResult{
		Baseline: baseline,
		Evidence: evidence,
	}, nil
}

func verificationPassEvidenceContent(decision *Artifact, baseline []AffectedFile, summary string) string {
	paths := make([]string, 0, len(baseline))

	for _, file := range baseline {
		paths = append(paths, file.Path)
	}

	lines := []string{
		"Desktop post-execution verification pass recorded.",
		fmt.Sprintf("Decision: %s", decision.Meta.ID),
		fmt.Sprintf("Baselined files (%d): %s", len(paths), strings.Join(paths, ", ")),
	}

	summary = strings.TrimSpace(summary)
	if summary != "" {
		lines = append(lines, summary)
	}

	return strings.Join(lines, "\n")
}
