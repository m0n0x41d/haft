package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/testsupport/profileadmissionfixture"
)

func TestComposeServerInstructionsContainsOnlyGlobalInvariants(t *testing.T) {
	t.Parallel()

	got := composeServerInstructionsForUnavailableProfile(nil)
	for _, want := range []string{
		"# Haft project memory",
		"## Conditional memory orientation",
		"context-heavy",
		"select exact identity",
		"## Persistence gate",
		"explicit operator",
		"agent-inferred from current Work",
		"without asking for separate permission",
		"`known_absent`",
		"## Status is not authority",
		"read-only attention surface",
		"## Manual decision and commission authority",
		"Binding a decision or commissioning Work",
		"# Haft MethodPack",
		`haft_method(action="pull")`,
		`haft_method(action="close")`,
		"# Haft code preflight",
		"explore",
		"code_context",
		"impact",
		"may be material",
		"not safety claims",
		"not_applicable",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("initialize doctrine missing %q:\n%s", want, got)
		}
	}
	for _, forbidden := range []string{
		"MANDATORY FIRST ACTION",
		"CRITICAL, NON-NEGOTIABLE",
		"FIRST thing you do in EVERY fresh session",
		"Run it, READ it, and only THEN begin work",
		"periodically during long work",
		"Consult it BEFORE writing or editing code",
		"ProjectTypeEnvHead",
		"TypeEnv",
		"haft_memory(action=",
		"haft_entity(action=",
		"haft_onboard(action=",
		"restart_required",
		"Project Workflow",
		".haft/workflow.md",
		"Path policies:",
		"Project profile applicability",
		"profile applicability",
		"profile-applicability",
		"project-profile",
		"canonical profile",
		"profile_applicability",
		"selected project profile",
		"Autonomous maintenance",
		"Trust resolved results",
	} {
		if strings.Contains(got, forbidden) {
			t.Errorf("initialize doctrine retained internal or universal wording %q:\n%s", forbidden, got)
		}
	}
	if len(got) > 3200 {
		t.Fatalf("always-on initialize doctrine is %d bytes, want <= 3200", len(got))
	}
	if strings.Index(got, "Haft project memory") > strings.Index(got, "Haft code preflight") {
		t.Errorf("general orientation should precede the code-graph doctrine in the prompt:\n%s", got)
	}

	const localWorkflowText = "LOCAL_WORKFLOW_MUST_NOT_ENTER_INITIALIZE"
	w := &project.Workflow{Intent: localWorkflowText}
	if !strings.Contains(w.PromptPrefix(), localWorkflowText) {
		t.Fatal("workflow fixture did not expose its project-local prompt")
	}
	withWf := composeServerInstructionsForUnavailableProfile(w)
	if withWf != got || strings.Contains(withWf, localWorkflowText) {
		t.Errorf(
			"project-local workflow changed initialize instructions:\n%s",
			withWf,
		)
	}
}

func TestComposeServerInstructionsForUnavailableProfileOmitsProjectLocalState(
	t *testing.T,
) {
	t.Parallel()

	const localWorkflowText = "LOCAL_UNAVAILABLE_PROFILE_WORKFLOW"
	workflow := &project.Workflow{Intent: localWorkflowText}
	instructions := composeServerInstructionsForUnavailableProfile(workflow)

	if instructions != composeServerInstructionsForUnavailableProfile(nil) ||
		strings.Contains(instructions, localWorkflowText) ||
		strings.Contains(instructions, "Project profile applicability") {
		t.Fatalf(
			"unavailable profile or workflow changed global initialize bytes:\n%s",
			instructions,
		)
	}
}

func TestComposeServerInstructions_AlwaysIncludesMethodPackProtocol(t *testing.T) {
	t.Parallel()

	got := composeServerInstructionsForUnavailableProfile(nil)

	for _, want := range []string{
		"Haft MethodPack",
		`haft_method(action="pull")`,
		"`pull_id`",
		`haft_method(action="close")`,
		"gate evidence",
		"verification",
		"explicit waivers",
		"Recover an open run",
		"does not block otherwise",
		"Mechanical edits need no manufactured ceremony",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("methodpack server instructions missing %q:\n%s", want, got)
		}
	}

	if strings.Index(got, "Haft project memory") > strings.Index(got, "Haft MethodPack") {
		t.Errorf("general orientation should precede MethodPack instructions:\n%s", got)
	}
	if strings.Index(got, "Haft MethodPack") > strings.Index(got, "Haft code preflight") {
		t.Errorf("MethodPack instructions should precede code-graph doctrine:\n%s", got)
	}
}

func TestComposeServerInstructionPartsBeforeInitializationKeepsGlobalRoutingInvariants(
	t *testing.T,
) {
	t.Parallel()

	got := composeServerInstructionParts(nil, nil)

	for _, want := range []string{
		"Haft project memory",
		"Haft MethodPack",
		"When the selected scope requires the SWE MethodPack",
		"Haft code preflight",
		"governance may be material",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf(
				"pre-initialization instructions omit %q:\n%s",
				want,
				got,
			)
		}
	}
}

func TestComposeServerInstructionsForProfileResolutionUsesCanonicalNonSoftwareScope(
	t *testing.T,
) {
	root := t.TempDir()
	harness := profileadmissionfixture.New(t, root)
	harness.AdmitNonSoftwareRevision(t, "server-instructions-nonsoftware")
	const localWorkflowText = "LOCAL_NONSOFTWARE_WORKFLOW_MUST_NOT_ENTER_INITIALIZE"
	workflowPath := project.WorkflowPath(filepath.Join(root, ".haft"))
	workflowContent := strings.Join([]string{
		"# Workflow",
		"",
		"## Intent",
		"",
		localWorkflowText,
		"",
		"## Defaults",
		"",
		"```yaml",
		"mode: standard",
		"require_decision: true",
		"require_verify: true",
		"allow_autonomy: false",
		"```",
	}, "\n")
	if err := os.WriteFile(
		workflowPath,
		[]byte(workflowContent),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	workflow, err := project.LoadWorkflow(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(workflow.PromptPrefix(), localWorkflowText) {
		t.Fatal("project workflow fixture did not expose its local prompt")
	}
	resolution, err := resolveCanonicalProjectSpecificationApplicability(
		context.Background(),
		root,
		automaticProjectSpecificationScopeRequest(),
	)
	if err != nil {
		t.Fatal(err)
	}
	instructions, err := composeServerInstructionsForProfileResolution(
		workflow,
		resolution,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(instructions, "Haft project memory") {
		t.Fatalf("non-software instructions omitted project memory:\n%s", instructions)
	}
	for _, required := range []string{
		"Haft MethodPack",
		"When the selected scope requires the SWE MethodPack",
		"Haft code preflight",
		"governance may be material",
	} {
		if !strings.Contains(instructions, required) {
			t.Fatalf(
				"canonical non-software instructions omit conditional global invariant %q:\n%s",
				required,
				instructions,
			)
		}
	}
	if instructions != composeServerInstructionsForUnavailableProfile(nil) ||
		strings.Contains(instructions, localWorkflowText) {
		t.Fatalf(
			"admitted non-software profile or local workflow changed initialize bytes:\n%s",
			instructions,
		)
	}
}

func TestComposeServerInstructionsForProfileResolutionUsesCanonicalSoftwareScope(
	t *testing.T,
) {
	root := t.TempDir()
	harness := profileadmissionfixture.New(t, root)
	harness.AdmitSoftwareRevision(t, "server-instructions-software")
	resolution, err := resolveCanonicalProjectSpecificationApplicability(
		context.Background(),
		root,
		automaticProjectSpecificationScopeRequest(),
	)
	if err != nil {
		t.Fatal(err)
	}
	instructions, err := composeServerInstructionsForProfileResolution(
		nil,
		resolution,
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"Haft MethodPack", "Haft code preflight"} {
		if !strings.Contains(instructions, required) {
			t.Fatalf(
				"canonical software instructions omit %q:\n%s",
				required,
				instructions,
			)
		}
	}
	if instructions != composeServerInstructionsForUnavailableProfile(nil) {
		t.Fatalf(
			"admitted software profile changed global initialize bytes:\n%s",
			instructions,
		)
	}
}

func TestComposeServerInstructionsForProfileResolutionOmitsUnderdeterminedCue(
	t *testing.T,
) {
	root := t.TempDir()
	profileadmissionfixture.New(t, root)
	resolution, err := resolveCanonicalProjectSpecificationApplicability(
		context.Background(),
		root,
		automaticProjectSpecificationScopeRequest(),
	)
	if err != nil {
		t.Fatal(err)
	}
	readiness := canonicalProjectReadiness{
		facts:            project.ReadinessFacts{},
		profileEvaluated: true,
		resolution:       resolution,
	}
	if cue := readiness.profileCue(); !strings.Contains(
		cue,
		"Project profile is underdetermined",
	) {
		t.Fatalf("dedicated profile cue surface was removed: %q", cue)
	}
	instructions, err := composeServerInstructionsForProfileResolution(
		nil,
		resolution,
	)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(instructions, "Project profile is underdetermined") ||
		strings.Contains(instructions, "Project profile applicability") ||
		instructions != composeServerInstructionsForUnavailableProfile(nil) {
		t.Fatalf(
			"underdetermined profile leaked into global initialize bytes:\n%s",
			instructions,
		)
	}
	for _, required := range []string{
		"Haft MethodPack",
		"When the selected scope requires the SWE MethodPack",
		"Haft code preflight",
		"governance may be material",
	} {
		if !strings.Contains(instructions, required) {
			t.Fatalf(
				"underdetermined instructions omit conditional global invariant %q:\n%s",
				required,
				instructions,
			)
		}
	}
}
