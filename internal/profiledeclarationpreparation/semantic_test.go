package profiledeclarationpreparation

import (
	"slices"
	"strings"
	"testing"
	"time"
)

func TestPlanBuildsOnlyCoherentV2ProfileDeclarationWork(t *testing.T) {
	root := canonicalWorkInputTestRoot(t)
	suggestion := workInputTestSuggestion(t, root, []string{
		"go.mod",
		"internal/kernel.go",
	})
	proposal, err := ProposeProfileOnboardingWorkInput(suggestion)
	if err != nil {
		t.Fatal(err)
	}
	input, err := DecodeProfileOnboardingWorkInput(proposal, suggestion)
	if err != nil {
		t.Fatal(err)
	}
	policy, err := NewPolicy(
		ModeExplicitHOnboard,
		".haft/config.yaml",
		[]byte("authority:\n  profile_declaration_mode: explicit_h_onboard\n"),
	)
	if err != nil {
		t.Fatal(err)
	}
	checkedAt := time.Now().UTC().Round(0)
	plan, err := NewPlan(root, input, policy, checkedAt)
	if err != nil {
		t.Fatal(err)
	}
	times, err := NewOccurrenceTimes(
		checkedAt.Add(time.Second),
		checkedAt.Add(2*time.Second),
		checkedAt.Add(3*time.Second),
		checkedAt.Add(4*time.Second),
	)
	if err != nil {
		t.Fatal(err)
	}
	values, err := plan.BuildValueSet(times)
	if err != nil {
		t.Fatal(err)
	}
	work := values.WorkRecord()
	if work.MethodDescriptionRef() != values.MethodDescription().Ref() {
		t.Fatal("v2 Work does not enact the exact v2 MethodDescription")
	}
	if work.MethodContractRef() != values.MethodContract().Ref() {
		t.Fatal("v2 Work does not bind the exact v2 MethodContract")
	}
	inputRefs := work.InputRefs()
	if len(inputRefs) != 2 || !slices.Contains(inputRefs, input.Ref()) {
		t.Fatalf("v2 Work inputs = %#v", inputRefs)
	}
	exactInput, ok := work.ProfileOnboardingWorkInputRefV2()
	if !ok || exactInput != input.Ref() {
		t.Fatalf("typed v2 Work input = %q, present = %t", exactInput.String(), ok)
	}
	basisRef, err := plan.AuthorityBasisRef()
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := plan.Candidate(values, basisRef)
	if err != nil {
		t.Fatal(err)
	}
	if candidate.Provenance().WorkRecordRef() != work.RecordRef() {
		t.Fatal("candidate does not bind the performed v2 Work")
	}
}

func TestPlanRejectsStrictProfileAuthorityBeforeAnyWork(t *testing.T) {
	root := canonicalWorkInputTestRoot(t)
	suggestion := workInputTestSuggestion(t, root, []string{"go.mod", "main.go"})
	proposal, err := ProposeProfileOnboardingWorkInput(suggestion)
	if err != nil {
		t.Fatal(err)
	}
	input, err := DecodeProfileOnboardingWorkInput(proposal, suggestion)
	if err != nil {
		t.Fatal(err)
	}
	policy, err := NewPolicy(ModeStrictSpeechAct, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewPlan(root, input, policy, time.Now().UTC())
	if err == nil || !strings.Contains(err.Error(), "strict_profile_authority_not_available") {
		t.Fatalf("strict preparation error = %v", err)
	}
}

func TestManualPlanKeepsObservationDetectorAndScopeClassifierDistinct(
	t *testing.T,
) {
	root := canonicalWorkInputTestRoot(t)
	suggestion := workInputTestSuggestion(
		t,
		root,
		[]string{"README.md"},
	)
	proposal, err := ProposeManualProfileOnboardingWorkInput(
		suggestion,
		ManualProfileProposalInput{
			Basis: "The repository is a documentation product.",
			Scopes: []ManualProfileScopeInput{{
				ScopeID:         "documents",
				Label:           "Project handbook",
				RealizationKind: "non_software",
				EvidencePaths:   []string{"README.md"},
			}},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	input, err := DecodeProfileOnboardingWorkInput(
		proposal,
		suggestion,
	)
	if err != nil {
		t.Fatal(err)
	}
	reloaded, err := DecodeCanonicalProfileOnboardingWorkInput(
		input.CanonicalJSON(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !reloaded.UsesManualScopeBasis() ||
		reloaded.ClassifierVersion() !=
			profileManualClassifierVersion {
		t.Fatalf(
			"manual durable reload source=%t classifier=%q",
			reloaded.UsesManualScopeBasis(),
			reloaded.ClassifierVersion(),
		)
	}
	policy, err := NewPolicy(
		ModeExplicitHOnboard,
		".haft/config.yaml",
		[]byte(
			"authority:\n  profile_declaration_mode: explicit_h_onboard\n",
		),
	)
	if err != nil {
		t.Fatal(err)
	}
	checkedAt := time.Now().UTC().Round(0)
	plan, err := NewPlan(root, reloaded, policy, checkedAt)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Support().ClassifierVersion().String() !=
		profileManualClassifierVersion {
		t.Fatalf(
			"authority classifier = %q",
			plan.Support().ClassifierVersion().String(),
		)
	}
	if plan.Support().PolicyVersion().String() !=
		profileManualPolicyVersion {
		t.Fatalf(
			"authority policy = %q",
			plan.Support().PolicyVersion().String(),
		)
	}
	times, err := NewOccurrenceTimes(
		checkedAt.Add(time.Second),
		checkedAt.Add(2*time.Second),
		checkedAt.Add(3*time.Second),
		checkedAt.Add(4*time.Second),
	)
	if err != nil {
		t.Fatal(err)
	}
	values, err := plan.BuildValueSet(times)
	if err != nil {
		t.Fatal(err)
	}
	basis := values.ObservedBasis()
	if basis.DetectorVersion().String() !=
		suggestion.DetectorVersion() {
		t.Fatalf(
			"observation detector = %q",
			basis.DetectorVersion().String(),
		)
	}
	if basis.ClassifierVersion().String() !=
		profileManualClassifierVersion {
		t.Fatalf(
			"basis classifier = %q",
			basis.ClassifierVersion().String(),
		)
	}
	signals := basis.Signals()
	if len(signals) != 1 ||
		!strings.Contains(
			signals[0].Value().String(),
			"manual scope proposal",
		) ||
		strings.Contains(
			signals[0].Value().String(),
			"detector observation",
		) {
		t.Fatalf("manual basis signals = %#v", signals)
	}
	basisRef, err := plan.AuthorityBasisRef()
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := plan.Candidate(values, basisRef)
	if err != nil {
		t.Fatal(err)
	}
	if candidate.Provenance().ClassifierVersion().String() !=
		profileManualClassifierVersion {
		t.Fatalf(
			"candidate classifier = %q",
			candidate.Provenance().ClassifierVersion().String(),
		)
	}
	if candidate.Provenance().PolicyVersion().String() !=
		profileManualPolicyVersion {
		t.Fatalf(
			"candidate policy = %q",
			candidate.Provenance().PolicyVersion().String(),
		)
	}
}
