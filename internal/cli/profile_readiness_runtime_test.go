package cli

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/testsupport/profileadmissionfixture"
)

func TestCanonicalProjectReadinessKeepsNonSoftwareHarnessFreeOfSWEGate(
	t *testing.T,
) {
	root := t.TempDir()
	harness := profileadmissionfixture.New(t, root)
	harness.AdmitNonSoftwareRevision(t, "readiness-nonsoftware")
	readiness, err := inspectCanonicalProjectReadiness(
		context.Background(),
		root,
		automaticProjectSpecificationScopeRequest(),
	)
	if err != nil {
		t.Fatal(err)
	}
	gate := harnessRunReadinessGateForCanonicalProfile(readiness, "")
	if gate.Kind != harnessRunReadinessAdmissible {
		t.Fatalf(
			"non-software gate = %#v, want admissible without tactical waiver",
			gate,
		)
	}
}

func TestCanonicalProjectReadinessRetainsSoftwareHarnessGate(
	t *testing.T,
) {
	root := t.TempDir()
	harness := profileadmissionfixture.New(t, root)
	harness.AdmitSoftwareRevision(t, "readiness-software")
	readiness, err := inspectCanonicalProjectReadiness(
		context.Background(),
		root,
		automaticProjectSpecificationScopeRequest(),
	)
	if err != nil {
		t.Fatal(err)
	}
	gate := harnessRunReadinessGateForCanonicalProfile(readiness, "")
	if gate.Kind != harnessRunReadinessBlocked {
		t.Fatalf("software gate = %#v, want blocked", gate)
	}
	if !strings.Contains(gate.BlockReason, "ProjectSpecificationSet") {
		t.Fatalf(
			"software block reason = %q, want specification boundary",
			gate.BlockReason,
		)
	}
}

func TestProfileAwareReadinessReminderUsesOneNeutralProfileCue(
	t *testing.T,
) {
	root := t.TempDir()
	profileadmissionfixture.New(t, root)
	result := applyProfileAwareReadinessReminder(
		context.Background(),
		"ProblemCard framed",
		"haft_problem",
		filepath.Join(root, ".haft"),
		map[string]any{},
	)
	if strings.Count(result, "── Project profile") != 1 {
		t.Fatalf("profile cue count in %q, want one", result)
	}
	if strings.Contains(result, "SoftwareSystemSpec") ||
		strings.Contains(result, "force-skip-specs") {
		t.Fatalf("neutral profile cue gained SWE pressure: %q", result)
	}
}

func TestCanonicalProjectReadinessPreservesExactScopeMiss(t *testing.T) {
	root := t.TempDir()
	harness := profileadmissionfixture.New(t, root)
	harness.AdmitNonSoftwareRevision(t, "readiness-exact-miss")
	missingScope, err := projectprofile.NewScopeID("missing-scope")
	if err != nil {
		t.Fatal(err)
	}
	request, err := exactProjectSpecificationScopeRequest(missingScope)
	if err != nil {
		t.Fatal(err)
	}
	readiness, err := inspectCanonicalProjectReadiness(
		context.Background(),
		root,
		request,
	)
	if err != nil {
		t.Fatal(err)
	}
	if readiness.resolution.Kind() != projectSpecificationRequestedScopeNotFound {
		t.Fatalf("resolution kind = %q", readiness.resolution.Kind())
	}
	if readiness.facts.Status != project.ReadinessNeedsOnboard {
		t.Fatalf("readiness status = %q", readiness.facts.Status)
	}
	cue := readiness.profileCue()
	if !strings.Contains(cue, `Requested project scope "missing-scope" is absent`) ||
		!strings.Contains(cue, "non-software-readiness-exact-miss") {
		t.Fatalf("exact-scope cue = %q", cue)
	}
	if strings.Count(statusProfilePrefix(readiness, false), cue) != 1 {
		t.Fatalf("exact-scope cue was not rendered exactly once")
	}
}

func TestProfileAwareReadinessReminderOmitsSWENagForNonSoftware(
	t *testing.T,
) {
	root := t.TempDir()
	harness := profileadmissionfixture.New(t, root)
	harness.AdmitNonSoftwareRevision(t, "reminder-nonsoftware")
	result := applyProfileAwareReadinessReminder(
		context.Background(),
		"Note recorded",
		"haft_note",
		filepath.Join(root, ".haft"),
		map[string]any{},
	)
	if result != "Note recorded" {
		t.Fatalf(
			"non-software reminder = %q, want original result",
			result,
		)
	}
}

func TestProfileAwareReadinessReminderKeepsSoftwareSpecCue(
	t *testing.T,
) {
	root := t.TempDir()
	harness := profileadmissionfixture.New(t, root)
	harness.AdmitSoftwareRevision(t, "reminder-software")
	result := applyProfileAwareReadinessReminder(
		context.Background(),
		"ProblemCard framed",
		"haft_problem",
		filepath.Join(root, ".haft"),
		map[string]any{},
	)
	if !strings.Contains(result, "Project readiness") ||
		!strings.Contains(result, "needs_onboard") {
		t.Fatalf("software reminder = %q, want readiness cue", result)
	}
}

func TestStatusProfilePrefixOmitsResolvedProfileUntilFullView(
	t *testing.T,
) {
	root := t.TempDir()
	harness := profileadmissionfixture.New(t, root)
	admission := harness.AdmitNonSoftwareRevision(t, "status-nonsoftware")
	readiness, err := inspectCanonicalProjectReadiness(
		context.Background(),
		root,
		automaticProjectSpecificationScopeRequest(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if compact := statusProfilePrefix(readiness, false); compact != "" {
		t.Fatalf("compact resolved profile = %q, want omitted", compact)
	}
	full := statusProfilePrefix(readiness, true)
	for _, marker := range []string{
		"scope_id=non-software-status-nonsoftware",
		"admission_record_ref=" + admission.AdmissionRecordRef().String(),
		"admission_record_digest=" + admission.AdmissionRecordDigest().String(),
		"profile_payload_digest=" + admission.PayloadDigest().String(),
		"Capability applicability (authority=canonical_profile_capability_matrix.v1)",
		"capability=software_system_spec; applicability=not_applicable",
		"capability=target_system_spec; applicability=underdetermined",
		"missing_basis=admitted_target_system_relation",
	} {
		if strings.Contains(full, marker) {
			continue
		}
		t.Fatalf("full profile basis = %q", full)
	}
}

func TestProfileAwareCoverageOmitsCodeIndexForNonSoftware(
	t *testing.T,
) {
	root := t.TempDir()
	harness := profileadmissionfixture.New(t, root)
	harness.AdmitNonSoftwareRevision(t, "coverage-nonsoftware")
	readiness, err := inspectCanonicalProjectReadiness(
		context.Background(),
		root,
		automaticProjectSpecificationScopeRequest(),
	)
	if err != nil {
		t.Fatal(err)
	}
	required, err := profileCodeCoverageRequired(readiness)
	if err != nil {
		t.Fatal(err)
	}
	if required {
		t.Fatal("non-software profile required code coverage")
	}
	response, err := profileAwareCoverageResponse(
		context.Background(),
		artifact.NewStore(harness.Database()),
		root,
		readiness,
		0,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(response, "not applicable") ||
		!strings.Contains(response, "No code index or SWE coverage debt") {
		t.Fatalf("non-software coverage response = %q", response)
	}
}

func TestProfileAwareCoverageRetainsSoftwareCapability(
	t *testing.T,
) {
	root := t.TempDir()
	harness := profileadmissionfixture.New(t, root)
	harness.AdmitSoftwareRevision(t, "coverage-software")
	readiness, err := inspectCanonicalProjectReadiness(
		context.Background(),
		root,
		automaticProjectSpecificationScopeRequest(),
	)
	if err != nil {
		t.Fatal(err)
	}
	required, err := profileCodeCoverageRequired(readiness)
	if err != nil {
		t.Fatal(err)
	}
	if !required {
		t.Fatal("software profile omitted code coverage")
	}
}

func TestProfileAwareCoverageKeepsMissingProfileNeutral(
	t *testing.T,
) {
	root := t.TempDir()
	harness := profileadmissionfixture.New(t, root)
	readiness, err := inspectCanonicalProjectReadiness(
		context.Background(),
		root,
		automaticProjectSpecificationScopeRequest(),
	)
	if err != nil {
		t.Fatal(err)
	}
	response, err := profileAwareCoverageResponse(
		context.Background(),
		artifact.NewStore(harness.Database()),
		root,
		readiness,
		0,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(response, "profile is underdetermined") ||
		!strings.Contains(response, "Code coverage was not evaluated") {
		t.Fatalf("underdetermined coverage response = %q", response)
	}
}
