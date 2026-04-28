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

	specSet, err := project.LoadProjectSpecificationSet(projectRoot)
	if err != nil {
		return err
	}

	intent := specflow.NextStep(specflow.DeriveState(specSet))

	output := cmd.OutOrStdout()
	if specOnboardJSON {
		return writeSpecOnboardJSON(output, intent)
	}

	return writeSpecOnboardSummary(output, intent)
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
