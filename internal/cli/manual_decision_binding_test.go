package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/operatorrequest"
)

type fakeHostRoutedDecisionBinder struct {
	input  artifact.DecideInput
	result decisionBindingOutcome
	err    error
}

func stubHostRoutedDecisionBinder(
	t *testing.T,
	binder *fakeHostRoutedDecisionBinder,
) {
	t.Helper()
	previous := bindDecisionFromHostRequest
	bindDecisionFromHostRequest = func(
		_ context.Context,
		_ string,
		input artifact.DecideInput,
	) (decisionBindingOutcome, error) {
		binder.input = input
		return binder.result, binder.err
	}
	t.Cleanup(func() {
		bindDecisionFromHostRequest = previous
	})
}

func TestDecisionArtifactCreateUsesHostRoutedEffectSink(t *testing.T) {
	_ = newArtifactCLITestProject(t)
	binder := &fakeHostRoutedDecisionBinder{
		result: decisionBindingOutcome{
			DecisionRef: "dec-20260801-readable-a1b2c3d4",
			Title:       "Use the host-routed decision sink",
			FilePath:    ".haft/decisions/host-routed.md",
		},
	}
	stubHostRoutedDecisionBinder(t, binder)
	input := []byte(`{
		"selected_title":"Use the host-routed decision sink",
		"problem_statement":"The public host must route the operator request.",
		"why_selected":"It preserves the authority boundary.",
		"selection_policy":"Require a direct unambiguous operator request.",
		"counterargument":"Host classification can be wrong.",
		"weakest_link":"Quoted text must remain non-authoritative."
	}`)
	command := &cobra.Command{}
	output := bytes.Buffer{}
	command.SetOut(&output)
	previousJSON := artifactCreateJSON
	artifactCreateJSON = false
	t.Cleanup(func() { artifactCreateJSON = previousJSON })

	if err := runDecisionArtifactCreate(command, input); err != nil {
		t.Fatalf("runDecisionArtifactCreate: %v", err)
	}
	if binder.input.SelectedTitle != "Use the host-routed decision sink" {
		t.Fatalf("binding input title = %q", binder.input.SelectedTitle)
	}
	for _, want := range []string{
		"dec-20260801-readable-a1b2c3d4",
		"Use the host-routed decision sink",
	} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("result omitted %q:\n%s", want, output.String())
		}
	}
}

func TestDecisionOperatorRequestUsesHonestProvenance(t *testing.T) {
	request, err := decisionOperatorRequest(artifact.DecideInput{
		DecisionSubjectRef: "subject:authority",
		SelectedTitle:      "Host routing",
	})
	if err != nil {
		t.Fatalf("decisionOperatorRequest: %v", err)
	}
	if request.Provenance() != operatorrequest.HostRoutedOperatorRequest {
		t.Fatalf("provenance = %q", request.Provenance())
	}
	if request.SubjectRef() != "subject:authority" {
		t.Fatalf("subject = %q", request.SubjectRef())
	}
}

func TestArtifactCommandHasNoDecisionResumeSurface(t *testing.T) {
	for _, command := range artifactCmd.Commands() {
		if command.Name() == "resume-decision" {
			t.Fatal("resume-decision remains registered")
		}
	}
}
