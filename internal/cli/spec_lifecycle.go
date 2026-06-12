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

func runSpecStatus(cmd *cobra.Command, _ []string) error {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a haft project: %w", err)
	}

	projection, err := buildSpecLifecycleProjection(projectRoot)
	if err != nil {
		return err
	}

	if specStatusJSON {
		return writeSpecLifecycleJSON(cmd.OutOrStdout(), projection)
	}

	return writeSpecLifecycleSummary(cmd.OutOrStdout(), projection)
}

func runSpecNext(cmd *cobra.Command, _ []string) error {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a haft project: %w", err)
	}

	projection, err := buildSpecLifecycleProjection(projectRoot)
	if err != nil {
		return err
	}

	if specNextJSON {
		return writeSpecLifecycleJSON(cmd.OutOrStdout(), projection)
	}

	return writeSpecLifecycleSummary(cmd.OutOrStdout(), projection)
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

func buildSpecLifecycleProjection(projectRoot string) (specflow.SpecLifecycleProjection, error) {
	specSet, err := project.LoadProjectSpecificationSet(projectRoot)
	if err != nil {
		return specflow.SpecLifecycleProjection{}, err
	}

	store, projectID, closeFn, err := projectBaseline(projectRoot)
	defer closeFn()
	if err != nil {
		return specflow.SpecLifecycleProjection{}, err
	}

	state := specflow.DeriveStateWithBaselines(specSet, store, projectID)
	return specflow.ProjectLifecycle(state), nil
}

func writeSpecLifecycleJSON(w io.Writer, projection specflow.SpecLifecycleProjection) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(projection)
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
