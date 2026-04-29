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

func runSpecOnboard(cmd *cobra.Command, _ []string) error {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a haft project: %w", err)
	}

	if mutationFlagCount() > 1 {
		return fmt.Errorf("specify at most one of --approve / --rebaseline / --reopen per invocation")
	}

	if action, sectionID, args := specOnboardMutationArgs(); action != "" {
		return runSpecOnboardMutation(cmd, projectRoot, action, sectionID, args)
	}

	specSet, err := project.LoadProjectSpecificationSet(projectRoot)
	if err != nil {
		return err
	}

	store, projectID, closeFn, _ := projectBaseline(projectRoot)
	defer closeFn()

	intent := specflow.NextStep(specflow.DeriveStateWithBaselines(specSet, store, projectID))

	output := cmd.OutOrStdout()
	if specOnboardJSON {
		return writeSpecOnboardJSON(output, intent)
	}

	return writeSpecOnboardSummary(output, intent)
}

func mutationFlagCount() int {
	count := 0
	for _, value := range []string{specOnboardApproveID, specOnboardRebaseline, specOnboardReopenID} {
		if strings.TrimSpace(value) != "" {
			count++
		}
	}
	return count
}

func specOnboardMutationArgs() (action, sectionID string, args map[string]any) {
	switch {
	case strings.TrimSpace(specOnboardApproveID) != "":
		return "approve", specOnboardApproveID, map[string]any{
			"action":      "approve",
			"section_id":  specOnboardApproveID,
			"approved_by": specOnboardApprovedBy,
		}
	case strings.TrimSpace(specOnboardRebaseline) != "":
		return "rebaseline", specOnboardRebaseline, map[string]any{
			"action":      "rebaseline",
			"section_id":  specOnboardRebaseline,
			"approved_by": specOnboardApprovedBy,
			"reason":      specOnboardReason,
		}
	case strings.TrimSpace(specOnboardReopenID) != "":
		return "reopen", specOnboardReopenID, map[string]any{
			"action":     "reopen",
			"section_id": specOnboardReopenID,
			"reason":     specOnboardReason,
		}
	}
	return "", "", nil
}

func runSpecOnboardMutation(cmd *cobra.Command, projectRoot, action, sectionID string, args map[string]any) error {
	var (
		raw string
		err error
	)
	switch action {
	case "approve":
		raw, err = handleSpecSectionApprove(projectRoot, args)
	case "rebaseline":
		raw, err = handleSpecSectionRebaseline(projectRoot, args)
	case "reopen":
		raw, err = handleSpecSectionReopen(projectRoot, args)
	default:
		return fmt.Errorf("unknown mutation action: %s", action)
	}
	if err != nil {
		return err
	}

	var result SpecSectionBaselineResult
	if jsonErr := json.Unmarshal([]byte(raw), &result); jsonErr != nil {
		return fmt.Errorf("decode baseline result: %w", jsonErr)
	}

	output := cmd.OutOrStdout()
	if specOnboardJSON {
		_, err := fmt.Fprintln(output, raw)
		return err
	}

	return writeSpecOnboardBaselineSummary(output, result, sectionID)
}

func writeSpecOnboardBaselineSummary(w io.Writer, result SpecSectionBaselineResult, sectionID string) error {
	var b strings.Builder

	fmt.Fprintf(&b, "Action:     %s\n", result.Action)
	fmt.Fprintf(&b, "Section:    %s\n", sectionID)
	if result.Hash != "" {
		fmt.Fprintf(&b, "Hash:       %s\n", result.Hash)
	}
	if result.CapturedAt != "" {
		fmt.Fprintf(&b, "Captured:   %s\n", result.CapturedAt)
	}
	if result.ApprovedBy != "" {
		fmt.Fprintf(&b, "Approved by: %s\n", result.ApprovedBy)
	}
	if result.Reason != "" {
		fmt.Fprintf(&b, "Reason:     %s\n", result.Reason)
	}
	if result.Message != "" {
		fmt.Fprintf(&b, "\n%s\n", result.Message)
	}

	_, err := fmt.Fprint(w, b.String())
	return err
}

func writeSpecOnboardJSON(w io.Writer, intent specflow.WorkflowIntent) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(intent)
}

func writeSpecOnboardSummary(w io.Writer, intent specflow.WorkflowIntent) error {
	if intent.Terminal {
		_, err := fmt.Fprintf(w, "Spec onboarding: terminal — %s\n", intent.Reason)
		return err
	}

	var b strings.Builder

	fmt.Fprintf(&b, "Phase:    %s\n", intent.Phase)
	fmt.Fprintf(&b, "Audience: %s\n", intent.Audience)
	fmt.Fprintf(&b, "Document: %s\n", intent.DocumentKind)
	fmt.Fprintf(&b, "Section:  %s\n", intent.SectionKind)

	if intent.PromptForUser != "" {
		fmt.Fprintf(&b, "\nFor the operator:\n%s\n", intent.PromptForUser)
	}

	if intent.ContextForAgent != "" {
		fmt.Fprintf(&b, "\nFor the host agent:\n%s\n", intent.ContextForAgent)
	}

	if len(intent.ExpectedFields) > 0 {
		fmt.Fprintf(&b, "\nExpected YAML fields: %s\n", strings.Join(intent.ExpectedFields, ", "))
	}

	if len(intent.Checks) > 0 {
		fmt.Fprintf(&b, "Structural checks:    %s\n", strings.Join(intent.Checks, ", "))
	}

	if len(intent.BlockingFindings) > 0 {
		fmt.Fprintf(&b, "\nBlocking findings:\n")
		for _, finding := range intent.BlockingFindings {
			fmt.Fprintf(&b, "  - [%s/%s] %s", finding.Level, finding.Code, finding.Message)
			if finding.NextAction != "" {
				fmt.Fprintf(&b, " — %s", finding.NextAction)
			}
			fmt.Fprintln(&b)
		}
	}

	if intent.Reason != "" {
		fmt.Fprintf(&b, "\nReason: %s\n", intent.Reason)
	}

	_, err := fmt.Fprint(w, b.String())
	return err
}
