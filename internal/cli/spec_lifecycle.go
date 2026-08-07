package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/project/specflow"
)

func runSpecStatus(cmd *cobra.Command, _ []string) error {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a haft project: %w", err)
	}

	request, err := projectSpecificationScopeRequestFromFlag(specStatusScopeID)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	result, err := buildPublicSpecLifecycle(ctx, projectRoot, request)
	if err != nil {
		return err
	}

	if specStatusJSON {
		return writePublicSpecLifecycleJSON(cmd.OutOrStdout(), result)
	}

	if result.SpecLifecycleProjection != nil {
		return writePublicSpecLifecycleSummary(cmd.OutOrStdout(), result)
	}
	return writeProjectSpecificationApplicabilityCue(
		cmd.OutOrStdout(),
		"haft spec status",
		result.ProfileApplicability,
	)
}

func runSpecNext(cmd *cobra.Command, _ []string) error {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a haft project: %w", err)
	}

	request, err := projectSpecificationScopeRequestFromFlag(specNextScopeID)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	result, err := buildPublicSpecLifecycle(ctx, projectRoot, request)
	if err != nil {
		return err
	}

	if specNextJSON {
		return writePublicSpecLifecycleJSON(cmd.OutOrStdout(), result)
	}
	if result.SpecLifecycleProjection != nil {
		return writePublicSpecLifecycleSummary(cmd.OutOrStdout(), result)
	}
	return writeProjectSpecificationApplicabilityCue(
		cmd.OutOrStdout(),
		"haft spec next",
		result.ProfileApplicability,
	)
}

func runSpecApprove(cmd *cobra.Command, args []string) error {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a haft project: %w", err)
	}

	sectionID := strings.TrimSpace(args[0])
	mutationArgs := map[string]any{
		"action":      "approve",
		"section_id":  sectionID,
		"approved_by": specApproveApprovedBy,
	}

	return runSpecOnboardMutation(cmd, projectRoot, "approve", sectionID, mutationArgs, specApproveJSON)
}

func runSpecRebaseline(cmd *cobra.Command, args []string) error {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a haft project: %w", err)
	}

	sectionID := strings.TrimSpace(args[0])
	mutationArgs := map[string]any{
		"action":      "rebaseline",
		"section_id":  sectionID,
		"approved_by": specRebaselineBy,
		"reason":      specRebaselineReason,
	}

	return runSpecOnboardMutation(cmd, projectRoot, "rebaseline", sectionID, mutationArgs, specRebaselineJSON)
}

func runSpecReopen(cmd *cobra.Command, args []string) error {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a haft project: %w", err)
	}

	sectionID := strings.TrimSpace(args[0])
	mutationArgs := map[string]any{
		"action":     "reopen",
		"section_id": sectionID,
		"reason":     specReopenReason,
	}

	return runSpecOnboardMutation(cmd, projectRoot, "reopen", sectionID, mutationArgs, specReopenJSON)
}

func writeSpecLifecycleSummary(w io.Writer, projection specflow.SpecLifecycleProjection) error {
	var b strings.Builder

	fmt.Fprintf(&b, "Spec status: %s\n", projection.State)
	fmt.Fprintf(&b, "Next action: %s\n", projection.Action)
	if projection.Object != "" {
		fmt.Fprintf(&b, "Object:      %s\n", projection.Object)
	}
	if projection.Phase != "" {
		fmt.Fprintf(&b, "Phase:       %s\n", projection.Phase)
	}
	if projection.SectionKind != "" {
		fmt.Fprintf(&b, "Section:     %s\n", projection.SectionKind)
	}
	if projection.SectionID != "" {
		fmt.Fprintf(&b, "Section id:  %s\n", projection.SectionID)
	}
	if projection.Carrier != "" {
		fmt.Fprintf(&b, "Carrier:     %s\n", projection.Carrier)
	}
	if projection.Why != "" {
		fmt.Fprintf(&b, "\nWhy:\n%s\n", projection.Why)
	}
	if projection.HumanGate != "" {
		fmt.Fprintf(&b, "\nHuman gate:\n%s\n", projection.HumanGate)
	}
	if len(projection.AllowedCommands) > 0 {
		fmt.Fprintf(&b, "\nAllowed next steps:\n")
		for _, command := range projection.AllowedCommands {
			fmt.Fprintf(&b, "  - %s\n", command)
		}
	}
	if len(projection.BlockedCommands) > 0 {
		fmt.Fprintf(&b, "\nBlocked until current action closes:\n")
		for _, command := range projection.BlockedCommands {
			fmt.Fprintf(&b, "  - %s\n", command)
		}
	}
	if len(projection.BlockingFindings) > 0 {
		fmt.Fprintf(&b, "\nBlocking findings:\n")
		for _, finding := range projection.BlockingFindings {
			fmt.Fprintf(&b, "  - [%s/%s] %s", finding.Level, finding.Code, finding.Message)
			if finding.NextAction != "" {
				fmt.Fprintf(&b, " — %s", finding.NextAction)
			}
			fmt.Fprintln(&b)
		}
	}

	_, err := fmt.Fprint(w, b.String())
	return err
}

func writePublicSpecLifecycleSummary(
	w io.Writer,
	result publicSpecLifecycleResult,
) error {
	if result.SpecLifecycleProjection == nil {
		return fmt.Errorf("spec lifecycle projection is unavailable")
	}
	var output strings.Builder
	if err := writeSpecLifecycleSummary(&output, *result.SpecLifecycleProjection); err != nil {
		return err
	}
	if result.Health != nil {
		fmt.Fprintf(&output, "\nSpec health: %s (%d finding(s); %d error(s), %d warning(s))\n",
			result.Health.State,
			result.Health.TotalFindings,
			result.Health.ErrorFindings,
			result.Health.WarningFindings,
		)
		fmt.Fprintf(&output, "Health check: %s\n", result.Health.CheckCommand)
		if result.Health.TotalFindings > 0 {
			fmt.Fprintln(&output, "Workflow readiness does not clear these health findings.")
		}
	}
	_, err := io.WriteString(w, output.String())
	return err
}
