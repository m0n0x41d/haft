package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/project/specflow"
)

var specReviewJSON bool

type specReviewPacket struct {
	specflow.ReviewPacket
	SpecEditionCarrierDelta specEditionCarrierDelta `json:"spec_edition_carrier_delta"`
}

var specReviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Run read-only semantic review over active SpecSections",
	Long: `Run a deterministic read-only semantic review over active SpecSections.

The review emits advisory findings and FPF hints. It does not approve,
rebaseline, reopen, create evidence, create decisions, or act as
SpecUseAdmission / GateDecision. It does not create claim truth, global truth,
or publication authority.`,
	RunE: runSpecReview,
}

func runSpecReview(cmd *cobra.Command, _ []string) error {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a haft project: %w", err)
	}

	packet, err := buildSpecReviewPacket(projectRoot)
	if err != nil {
		return err
	}

	output := cmd.OutOrStdout()
	if specReviewJSON {
		return writeSpecReviewJSON(output, packet)
	}

	return writeSpecReviewSummary(output, packet)
}

func buildSpecReviewPacket(projectRoot string) (specReviewPacket, error) {
	specSet, delta, err := loadProjectSpecificationSetSQLFirstWithCarrierDelta(
		projectRoot,
	)
	if err != nil {
		return specReviewPacket{}, err
	}

	return specReviewPacket{
		ReviewPacket:            specflow.ReviewSpecificationSet(specSet),
		SpecEditionCarrierDelta: delta,
	}, nil
}

func writeSpecReviewJSON(w io.Writer, packet specReviewPacket) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")

	return encoder.Encode(packet)
}

func writeSpecReviewSummary(w io.Writer, packet specReviewPacket) error {
	builder := strings.Builder{}

	builder.WriteString(fmt.Sprintf(
		"haft spec review: %s %s over %d active section(s)\n",
		packet.ReviewKind,
		packet.Authority,
		packet.Summary.ActiveSections,
	))
	builder.WriteString(fmt.Sprintf(
		"findings: %d (warn=%d abstain=%d blocked_for_stronger_use=%d)\n",
		packet.Summary.TotalFindings,
		packet.Summary.WarnFindings,
		packet.Summary.AbstainFindings,
		packet.Summary.BlockedForStrongerUse,
	))
	if packet.Summary.ExplicitClaims > 0 {
		builder.WriteString(fmt.Sprintf(
			"claims: explicit=%d declared=%d mixed_unresolved=%d unclassified=%d missing_support=%d\n",
			packet.Summary.ExplicitClaims,
			packet.Summary.DeclaredClaims,
			packet.Summary.MixedUnresolvedClaims,
			packet.Summary.UnclassifiedClaims,
			packet.Summary.MissingSupportClaims,
		))
	}
	builder.WriteString(formatSpecReviewProfile(packet.Profile))
	builder.WriteString(fmt.Sprintf(
		"authority: advisory_only; evidence=%s approval=%s rebaseline=%s gate_decision=%s spec_use_admission=%s claim_truth=%s global_truth=%s publication=%s\n",
		packet.AuthorityBoundary.Evidence,
		packet.AuthorityBoundary.Approval,
		packet.AuthorityBoundary.Rebaseline,
		packet.AuthorityBoundary.GateDecision,
		packet.AuthorityBoundary.SpecUseAdmission,
		packet.AuthorityBoundary.ClaimTruth,
		packet.AuthorityBoundary.GlobalTruth,
		packet.AuthorityBoundary.Publication,
	))
	builder.WriteString("state_readings: per-section profile names bearer, frame, use, and reopen_condition; not global ready/pass/current\n")
	builder.WriteString(formatSpecEditionCarrierDelta(
		packet.SpecEditionCarrierDelta,
	))

	if len(packet.Findings) > 0 {
		builder.WriteString("\nFindings:\n")
	}

	for _, finding := range packet.Findings {
		builder.WriteString(formatSpecReviewFinding(finding))
	}

	_, err := io.WriteString(w, builder.String())

	return err
}

func formatSpecEditionCarrierDelta(delta specEditionCarrierDelta) string {
	builder := strings.Builder{}
	builder.WriteString(fmt.Sprintf(
		"spec_edition_carrier_delta: %s sql_sections=%d carrier_sections=%d sql_claims=%d sql_stored_claims=%d carrier_claims=%d claim_count_basis=%s judgment=%s\n",
		delta.Posture,
		delta.SQLSectionCount,
		delta.CarrierSectionCount,
		delta.SQLClaimCount,
		delta.SQLStoredClaimCount,
		delta.CarrierClaimCount,
		delta.ClaimCountBasis,
		delta.Judgment,
	))
	builder.WriteString(fmt.Sprintf(
		"carrier_observation: %s findings=%d\n",
		delta.CarrierObservation.Posture,
		delta.CarrierObservation.FindingCount,
	))
	for _, section := range delta.Sections {
		builder.WriteString(fmt.Sprintf(
			"  - %s sql_hash=%s carrier_hash=%s carrier_added_claims=%s carrier_removed_claims=%s carrier_changed_claims=%s non_claim_fields_changed=%t claim_order_changed=%t\n",
			section.SectionID,
			section.SQLSemanticHash,
			section.CarrierSemanticHash,
			strings.Join(section.CarrierAddedClaimIDs, ","),
			strings.Join(section.CarrierRemovedClaimIDs, ","),
			strings.Join(section.CarrierChangedClaimIDs, ","),
			section.NonClaimFieldsChanged,
			section.ClaimOrderChanged,
		))
	}
	return builder.String()
}

func formatSpecReviewProfile(profile specflow.ReviewProfile) string {
	id := strings.TrimSpace(profile.ID)
	if id == "" {
		return ""
	}

	return fmt.Sprintf(
		"profile: %s; value_slice=%s\n",
		id,
		reviewModelDisposition(profile, "value_slice"),
	)
}

func reviewModelDisposition(profile specflow.ReviewProfile, name string) string {
	needle := strings.TrimSpace(name)
	for _, input := range profile.ModelInputs {
		if strings.TrimSpace(input.Name) != needle {
			continue
		}

		return strings.TrimSpace(input.Disposition)
	}

	return "unknown"
}

func formatSpecReviewFinding(finding specflow.ReviewFinding) string {
	location := finding.Source.Path
	if finding.Source.Line > 0 {
		location = fmt.Sprintf("%s:%d", finding.Source.Path, finding.Source.Line)
	}

	section := ""
	if strings.TrimSpace(finding.SectionID) != "" {
		section = " section=" + finding.SectionID
	}

	line := fmt.Sprintf(
		"- [%s] %s %s%s - %s\n",
		finding.Severity,
		finding.RuleID,
		location,
		section,
		finding.Finding,
	)
	line += fmt.Sprintf("  fpf: %s\n", finding.FPFHint.Principle)
	line += fmt.Sprintf("  agent_action: %s\n", finding.FPFHint.AgentAction)
	line += fmt.Sprintf("  stronger_use: %s\n", finding.FPFHint.StrongerUse)

	return line
}
