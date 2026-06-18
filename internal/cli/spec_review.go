package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/project/specflow"
)

var specReviewJSON bool

var specReviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Run read-only semantic review over active SpecSections",
	Long: `Run a deterministic read-only semantic review over active SpecSections.

The review emits advisory findings and FPF hints. It does not approve,
rebaseline, reopen, create evidence, create decisions, or act as
SpecUseAdmission / GateDecision.`,
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

func buildSpecReviewPacket(projectRoot string) (specflow.ReviewPacket, error) {
	specSet, err := project.LoadProjectSpecificationSet(projectRoot)
	if err != nil {
		return specflow.ReviewPacket{}, err
	}

	return specflow.ReviewSpecificationSet(specSet), nil
}

func writeSpecReviewJSON(w io.Writer, packet specflow.ReviewPacket) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")

	return encoder.Encode(packet)
}

func writeSpecReviewSummary(w io.Writer, packet specflow.ReviewPacket) error {
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
	builder.WriteString("authority: advisory_only; not evidence, approval, rebaseline, GateDecision, or SpecUseAdmission\n")
	builder.WriteString("state_readings: per-section profile names bearer, frame, use, and reopen_condition; not global ready/pass/current\n")

	if len(packet.Findings) > 0 {
		builder.WriteString("\nFindings:\n")
	}

	for _, finding := range packet.Findings {
		builder.WriteString(formatSpecReviewFinding(finding))
	}

	_, err := io.WriteString(w, builder.String())

	return err
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
