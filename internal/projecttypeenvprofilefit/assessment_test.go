package projecttypeenvprofilefit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilebasis"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestAssessExactAbsenceIsUnderdetermined(t *testing.T) {
	basis, err := projecttypeenvprofilebasis.NewNoCanonicalProjectProfile(
		fitProjectRoot(t),
	)
	if err != nil {
		t.Fatal(err)
	}
	result, err := assessVerifiedEnvironment(
		basis,
		fitTypeEnvRef(t, "1"),
		fitDigest(t, "2"),
		fitEnvironment(t, nil, nil),
		CurrentRuleEdition(),
	)
	if err != nil {
		t.Fatalf("assessVerifiedEnvironment(): %v", err)
	}
	if _, ok := result.(Underdetermined); !ok {
		t.Fatalf("result = %T, want Underdetermined", result)
	}
	grounds := result.Grounds()
	if len(grounds) != 1 || grounds[0].Kind() != GroundNoCanonicalProfile {
		t.Fatalf("grounds = %#v", grounds)
	}
	if err := result.Verify(); err != nil {
		t.Fatalf("Verify(): %v", err)
	}
}

func TestAssessSoftwareOnlyDeclaredProfileIsCompatible(t *testing.T) {
	basis := fitDeclaredBasis(t, fitSoftwarePayload(t))
	result, err := assessVerifiedEnvironment(
		basis,
		fitTypeEnvRef(t, "1"),
		fitDigest(t, "2"),
		fitEnvironment(t, nil, nil),
		CurrentRuleEdition(),
	)
	if err != nil {
		t.Fatalf("assessVerifiedEnvironment(): %v", err)
	}
	if _, ok := result.(Compatible); !ok {
		t.Fatalf("result = %T, want Compatible", result)
	}
	if result.Grounds()[0].Kind() != GroundSoftwareScope {
		t.Fatalf("ground = %s", result.Grounds()[0].String())
	}
}

func TestAssessDeclaredKindWithoutContextIsIncompatible(t *testing.T) {
	kind := fitKindDefinition(t, "U.System")
	basis := fitDeclaredBasis(
		t,
		fitNonSoftwarePayload(t, "U.System", nil, nil),
	)
	result, err := assessVerifiedEnvironment(
		basis,
		fitTypeEnvRef(t, "1"),
		fitDigest(t, "2"),
		fitEnvironment(t, []typedmemory.KindDefinition{kind}, nil),
		CurrentRuleEdition(),
	)
	if err != nil {
		t.Fatalf("assessVerifiedEnvironment(): %v", err)
	}
	if _, ok := result.(Incompatible); !ok {
		t.Fatalf("result = %T, want Incompatible", result)
	}
	if result.Grounds()[0].Kind() != GroundKindDefinitionWithoutContext {
		t.Fatalf("ground = %s", result.Grounds()[0].String())
	}
}

func TestAssessCompiledPatternButUnindexedContractIsUnderdetermined(t *testing.T) {
	patternRef := "C.28"
	contractRef := "spec-section:system-contract"
	basis := fitDeclaredBasis(
		t,
		fitNonSoftwarePayload(t, "", []string{patternRef}, []string{contractRef}),
	)
	coverage := []typedmemory.CoverageEntry{
		fitCoverageEntry(t, typedmemory.CoverageCompiled, patternRef),
	}
	result, err := assessVerifiedEnvironment(
		basis,
		fitTypeEnvRef(t, "1"),
		fitDigest(t, "2"),
		fitEnvironment(t, nil, coverage),
		CurrentRuleEdition(),
	)
	if err != nil {
		t.Fatalf("assessVerifiedEnvironment(): %v", err)
	}
	if _, ok := result.(Underdetermined); !ok {
		t.Fatalf("result = %T, want Underdetermined", result)
	}
	kinds := map[GroundKind]bool{}
	for _, ground := range result.Grounds() {
		kinds[ground.Kind()] = true
	}
	if !kinds[GroundPatternCompiled] ||
		!kinds[GroundContractIndexUnavailable] ||
		!kinds[GroundKindOrientationUnspecified] {
		t.Fatalf("ground kinds = %#v", kinds)
	}
}

func TestAssessMixedPatternCoverageDoesNotOverclaimCompiledFit(t *testing.T) {
	patternRef := "C.28"
	basis := fitDeclaredBasis(
		t,
		fitNonSoftwarePayload(t, "", []string{patternRef}, nil),
	)
	coverage := []typedmemory.CoverageEntry{
		fitCoverageEntryForUnit(
			t,
			typedmemory.CoverageCompiled,
			patternRef,
			"source-unit:C.28-compiled",
		),
		fitCoverageEntryForUnit(
			t,
			typedmemory.CoverageSourceOnly,
			patternRef,
			"source-unit:C.28-source-only",
		),
	}
	result, err := assessVerifiedEnvironment(
		basis,
		fitTypeEnvRef(t, "1"),
		fitDigest(t, "2"),
		fitEnvironment(t, nil, coverage),
		CurrentRuleEdition(),
	)
	if err != nil {
		t.Fatalf("assessVerifiedEnvironment(): %v", err)
	}
	if _, ok := result.(Underdetermined); !ok {
		t.Fatalf("result = %T, want Underdetermined", result)
	}
	kinds := map[GroundKind]bool{}
	for _, ground := range result.Grounds() {
		kinds[ground.Kind()] = true
	}
	if !kinds[GroundPatternSourceOnly] || kinds[GroundPatternCompiled] {
		t.Fatalf("ground kinds = %#v", kinds)
	}
}

func TestAssessUnknownEditionIsUnavailableAndBindsTargetIdentity(t *testing.T) {
	basis := fitDeclaredBasis(t, fitSoftwarePayload(t))
	edition, err := NewRuleEdition("future-rules/v9")
	if err != nil {
		t.Fatal(err)
	}
	left, err := assessVerifiedEnvironment(
		basis,
		fitTypeEnvRef(t, "1"),
		fitDigest(t, "2"),
		fitEnvironment(t, nil, nil),
		edition,
	)
	if err != nil {
		t.Fatal(err)
	}
	right, err := assessVerifiedEnvironment(
		basis,
		fitTypeEnvRef(t, "1"),
		fitDigest(t, "3"),
		fitEnvironment(t, nil, nil),
		edition,
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := left.(Unavailable); !ok {
		t.Fatalf("result = %T, want Unavailable", left)
	}
	if left.Digest() == right.Digest() {
		t.Fatal("target snapshot digest did not affect assessment identity")
	}
}

func fitEnvironment(
	t *testing.T,
	kinds []typedmemory.KindDefinition,
	extraCoverage []typedmemory.CoverageEntry,
) typedmemory.TypeEnv {
	t.Helper()
	baseCoverage := fitCoverageEntry(t, typedmemory.CoverageCompiled, "A.1")
	coverageEntries := append([]typedmemory.CoverageEntry{baseCoverage}, extraCoverage...)
	coverage, err := typedmemory.NewCoverageManifest(coverageEntries)
	if err != nil {
		t.Fatal(err)
	}
	revision, _ := typedmemory.NewSourceRevision("fpf-test-revision")
	compiler, _ := typedmemory.NewCompilerSchemaVersion("profile-fit-test/v1")
	contextRef, _ := typedmemory.NewBoundedContextRef("ctx:test")
	context, err := typedmemory.NewBoundedContext(contextRef, fitProvenance(t, "A.1"))
	if err != nil {
		t.Fatal(err)
	}
	builder := typedmemory.NewTypeEnvBuilder(fitTypeEnvRef(t, "1")).
		SetSourceRevision(revision).
		SetCompilerSchemaVersion(compiler).
		SetCoverageManifest(coverage).
		AddBoundedContext(context)
	for _, kind := range kinds {
		builder = builder.AddKindDefinition(kind)
	}
	environment, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	return environment
}

func fitCoverageEntry(
	t *testing.T,
	posture typedmemory.CoveragePosture,
	pattern string,
) typedmemory.CoverageEntry {
	t.Helper()
	return fitCoverageEntryForUnit(
		t,
		posture,
		pattern,
		"source-unit:"+pattern,
	)
}

func fitCoverageEntryForUnit(
	t *testing.T,
	posture typedmemory.CoveragePosture,
	pattern string,
	unitID string,
) typedmemory.CoverageEntry {
	t.Helper()
	location := fitSourceLocationForUnit(t, pattern, unitID)
	subject, err := typedmemory.SourceUnitCoverage(location.UnitID())
	if err != nil {
		t.Fatal(err)
	}
	switch posture {
	case typedmemory.CoverageCompiled:
		value, err := typedmemory.NewCompiledCoverageEntry(subject, location)
		if err != nil {
			t.Fatal(err)
		}
		return value
	case typedmemory.CoverageSourceOnly:
		value, err := typedmemory.NewSourceOnlyCoverageEntry(subject, location, "not_compiled")
		if err != nil {
			t.Fatal(err)
		}
		return value
	case typedmemory.CoverageUnsupported:
		value, err := typedmemory.NewUnsupportedCoverageEntry(subject, location, "unsupported")
		if err != nil {
			t.Fatal(err)
		}
		return value
	default:
		t.Fatalf("unsupported test coverage posture %v", posture)
		return typedmemory.CoverageEntry{}
	}
}

func fitKindDefinition(t *testing.T, raw string) typedmemory.KindDefinition {
	t.Helper()
	id, err := typedmemory.NewKindID(raw)
	if err != nil {
		t.Fatal(err)
	}
	value, err := typedmemory.NewKindDefinition(id, fitProvenance(t, "A.1"))
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func fitProvenance(t *testing.T, pattern string) typedmemory.FPFSourceProvenance {
	t.Helper()
	ref, _ := typedmemory.NewProvenanceRef("provenance:" + pattern)
	rule, _ := typedmemory.NewCompilerRuleID("profile-fit-test.rule.v1")
	value, err := typedmemory.NewFPFSourceProvenance(
		ref,
		fitSourceLocation(t, pattern),
		rule,
	)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func fitSourceLocation(t *testing.T, pattern string) typedmemory.SourceLocation {
	t.Helper()
	return fitSourceLocationForUnit(t, pattern, "source-unit:"+pattern)
}

func fitSourceLocationForUnit(
	t *testing.T,
	pattern string,
	unitID string,
) typedmemory.SourceLocation {
	t.Helper()
	unit, _ := typedmemory.NewSourceUnitID(unitID)
	revision, _ := typedmemory.NewSourceRevision("fpf-test-revision")
	lines, _ := typedmemory.NewSourceLineRange(1, 2)
	patternID, _ := typedmemory.NewPatternID(pattern)
	value, err := typedmemory.NewPatternedSourceLocation(
		unit,
		revision,
		fitDigest(t, "a"),
		lines,
		patternID,
	)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func fitDeclaredBasis(
	t *testing.T,
	payload projectprofile.ProfileDeclarationPayload,
) projecttypeenvprofilebasis.DeclaredCanonicalProjectProfile {
	t.Helper()
	input := projecttypeenvprofilebasis.DeclaredProjectProfileBasisInput{
		ProjectRoot: fitProjectRoot(t), LedgerRevision: projectprofile.NewLedgerRevision(1),
		Payload:                           payload,
		AdmissionRecordRef:                fitAdmissionRef(t, "profile-admission:test"),
		AdmissionRecordDigest:             fitContentDigest(t, "1"),
		AdmissionRecordCanonicalJSON:      []byte(`{"schema":"test-admission/v1"}`),
		ReceiptDigest:                     fitContentDigest(t, "2"),
		ReceiptCanonicalJSON:              []byte(`{"schema":"test-receipt/v1"}`),
		CandidateProvenanceDigest:         fitContentDigest(t, "3"),
		WorkRecordRef:                     fitWorkRef(t, "work:test"),
		WorkRecordDigest:                  fitContentDigest(t, "4"),
		AuthorityBasisRef:                 fitAuthorityBasisRef(t, "authority-basis:test"),
		AuthorityBasisDigest:              fitContentDigest(t, "5"),
		AuthorityResolutionRef:            fitAuthorityResolutionRef(t, "authority-resolution:test"),
		AuthorityResolutionDigest:         fitContentDigest(t, "6"),
		ProfileAuthorRoleAssignmentRef:    fitRoleAssignmentRef(t, "role-assignment:test"),
		ProfileAuthorRoleAssignmentDigest: fitContentDigest(t, "7"),
		ObservedProjectBasisRef:           fitObservedBasisRef(t, "observed-basis:test"),
		ObservedProjectBasisDigest:        fitContentDigest(t, "8"),
		OutcomeAssessmentRef:              fitOutcomeAssessmentRef(t, "outcome:test"),
		OutcomeAssessmentDigest:           fitContentDigest(t, "9"),
	}
	value, err := projecttypeenvprofilebasis.NewDeclaredCanonicalProjectProfile(input)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func fitSoftwarePayload(t *testing.T) projectprofile.ProfileDeclarationPayload {
	t.Helper()
	scopeID, _ := projectprofile.NewScopeID("software")
	scope, _ := projectprofile.NewSoftwareRealization(
		scopeID,
		projectprofile.NoEntityReference{},
	)
	return fitPayload(t, []projectprofile.RealizationScope{scope})
}

func fitNonSoftwarePayload(
	t *testing.T,
	kind string,
	patterns []string,
	contracts []string,
) projectprofile.ProfileDeclarationPayload {
	t.Helper()
	scopeID, _ := projectprofile.NewScopeID("non-software")
	orientation := projectprofile.KindOrientation(projectprofile.UnspecifiedKindOrientation{})
	if kind != "" {
		ref, _ := projectprofile.NewKindRef(kind)
		orientation = projectprofile.NewReferencedKindOrientation(ref)
	}
	patternRefs := make([]projectprofile.SourceUnitRef, 0, len(patterns))
	for _, raw := range patterns {
		ref, _ := projectprofile.NewSourceUnitRef(raw)
		patternRefs = append(patternRefs, ref)
	}
	contractRefs := make([]projectprofile.SpecSectionRef, 0, len(contracts))
	for _, raw := range contracts {
		ref, _ := projectprofile.NewSpecSectionRef(raw)
		contractRefs = append(contractRefs, ref)
	}
	scope, err := projectprofile.NewNonSoftwareRealization(
		scopeID,
		projectprofile.NoEntityReference{},
		orientation,
		patternRefs,
		contractRefs,
	)
	if err != nil {
		t.Fatal(err)
	}
	return fitPayload(t, []projectprofile.RealizationScope{scope})
}

func fitPayload(
	t *testing.T,
	scopes []projectprofile.RealizationScope,
) projectprofile.ProfileDeclarationPayload {
	t.Helper()
	set, err := projectprofile.NewScopeSet(scopes)
	if err != nil {
		t.Fatal(err)
	}
	value, err := projectprofile.NewProfileDeclarationPayload(set)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func fitProjectRoot(t *testing.T) projectprofile.ProjectRootV1 {
	t.Helper()
	value, err := projectprofile.NewProjectRootV1("/tmp/haft-profile-fit")
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func fitDigest(t *testing.T, fill string) typedmemory.SHA256Digest {
	t.Helper()
	value, err := typedmemory.NewSHA256Digest("sha256:" + strings.Repeat(fill, 64))
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func fitTypeEnvRef(t *testing.T, fill string) typedmemory.TypeEnvRef {
	t.Helper()
	value, err := typedmemory.NewTypeEnvRef(fitDigest(t, fill))
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func fitContentDigest(t *testing.T, fill string) projectprofile.ContentDigest {
	t.Helper()
	value, err := projectprofile.NewContentDigest("sha256:" + strings.Repeat(fill, 64))
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func fitAdmissionRef(
	t *testing.T,
	raw string,
) projectprofile.ProfileDeclarationAdmissionRecordRef {
	t.Helper()
	value, err := projectprofile.NewProfileDeclarationAdmissionRecordRef(raw)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func fitWorkRef(t *testing.T, raw string) projectprofile.ProfileOnboardingWorkRecordRef {
	t.Helper()
	value, err := projectprofile.NewProfileOnboardingWorkRecordRef(raw)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func fitAuthorityBasisRef(
	t *testing.T,
	raw string,
) projectprofile.ProfileDeclarationAuthorityBasisRef {
	t.Helper()
	value, err := projectprofile.NewProfileDeclarationAuthorityBasisRef(raw)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func fitAuthorityResolutionRef(
	t *testing.T,
	raw string,
) projectprofile.AuthorityResolutionRecordRef {
	t.Helper()
	value, err := projectprofile.NewAuthorityResolutionRecordRef(raw)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func fitRoleAssignmentRef(t *testing.T, raw string) projectprofile.RoleAssignmentRef {
	t.Helper()
	value, err := projectprofile.NewRoleAssignmentRef(raw)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func fitObservedBasisRef(
	t *testing.T,
	raw string,
) projectprofile.ObservedProjectBasisRefV1 {
	t.Helper()
	value, err := projectprofile.NewObservedProjectBasisRefV1(raw)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func fitOutcomeAssessmentRef(
	t *testing.T,
	raw string,
) projectprofile.ProfileOnboardingOutcomeAssessmentRefV1 {
	t.Helper()
	value, err := projectprofile.NewProfileOnboardingOutcomeAssessmentRefV1(raw)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestGroundStringIsDeterministicPresentationOnly(t *testing.T) {
	ground := newGround(
		GroundKindAvailable,
		GroundSatisfied,
		"scope",
		"U.System",
		[]string{"ctx:b", "ctx:a"},
	)
	if got, want := ground.String(), "satisfied;kind_available;scope=scope;coordinate=U.System;contexts=ctx:a,ctx:b"; got != want {
		t.Fatalf("ground.String() = %q, want %q", got, want)
	}
	_ = fmt.Sprintf("%s", ground.String())
}

func TestDecodeCanonicalAssessmentRoundTripsEveryClosedVariant(t *testing.T) {
	environment := fitEnvironment(t, nil, nil)
	targetRef := fitTypeEnvRef(t, "1")
	targetDigest := fitDigest(t, "2")
	absence, err := projecttypeenvprofilebasis.NewNoCanonicalProjectProfile(
		fitProjectRoot(t),
	)
	if err != nil {
		t.Fatal(err)
	}
	underdetermined, err := assessVerifiedEnvironment(
		absence,
		targetRef,
		targetDigest,
		environment,
		CurrentRuleEdition(),
	)
	if err != nil {
		t.Fatal(err)
	}
	compatible, err := assessVerifiedEnvironment(
		fitDeclaredBasis(t, fitSoftwarePayload(t)),
		targetRef,
		targetDigest,
		environment,
		CurrentRuleEdition(),
	)
	if err != nil {
		t.Fatal(err)
	}
	incompatible, err := assessVerifiedEnvironment(
		fitDeclaredBasis(t, fitNonSoftwarePayload(t, "U.System", nil, nil)),
		targetRef,
		targetDigest,
		fitEnvironment(t, []typedmemory.KindDefinition{fitKindDefinition(t, "U.System")}, nil),
		CurrentRuleEdition(),
	)
	if err != nil {
		t.Fatal(err)
	}
	futureEdition, _ := NewRuleEdition("future-rules/v9")
	unavailable, err := assessVerifiedEnvironment(
		fitDeclaredBasis(t, fitSoftwarePayload(t)),
		targetRef,
		targetDigest,
		environment,
		futureEdition,
	)
	if err != nil {
		t.Fatal(err)
	}
	values := []Assessment{compatible, incompatible, underdetermined, unavailable}
	for _, value := range values {
		t.Run(fmt.Sprintf("%T", value), func(t *testing.T) {
			decoded, err := DecodeCanonicalAssessment(value.CanonicalBytes())
			if err != nil {
				t.Fatalf("DecodeCanonicalAssessment(): %v", err)
			}
			if fmt.Sprintf("%T", decoded) != fmt.Sprintf("%T", value) {
				t.Fatalf("decoded = %T, want %T", decoded, value)
			}
			if decoded.Digest() != value.Digest() ||
				decoded.FitRef() != value.FitRef() ||
				decoded.BasisRef() != value.BasisRef() ||
				decoded.BasisDigest() != value.BasisDigest() ||
				!bytes.Equal(decoded.CanonicalBytes(), value.CanonicalBytes()) {
				t.Fatal("decoded assessment differs from exact persisted assessment")
			}
		})
	}
}

func TestDecodeCanonicalAssessmentRejectsNonCanonicalUnknownAndTrailingInput(t *testing.T) {
	value, err := assessVerifiedEnvironment(
		fitDeclaredBasis(t, fitSoftwarePayload(t)),
		fitTypeEnvRef(t, "1"),
		fitDigest(t, "2"),
		fitEnvironment(t, nil, nil),
		CurrentRuleEdition(),
	)
	if err != nil {
		t.Fatal(err)
	}
	canonical := value.CanonicalBytes()
	dto := map[string]any{}
	if err := json.Unmarshal(canonical, &dto); err != nil {
		t.Fatal(err)
	}
	unknownVariant := cloneAssessmentMap(t, dto)
	unknownVariant["variant"] = "maybe"
	unknownField := cloneAssessmentMap(t, dto)
	unknownField["recommended"] = true
	basisMismatch := cloneAssessmentMap(t, dto)
	basisMismatch["basis_digest"] = fitDigest(t, "f").String()
	cases := map[string][]byte{
		"unknown variant": mustJSON(t, unknownVariant),
		"unknown field":   mustJSON(t, unknownField),
		"basis mismatch":  mustJSON(t, basisMismatch),
		"trailing JSON":   append(append([]byte(nil), canonical...), []byte(`{}`)...),
		"trailing space":  append(append([]byte(nil), canonical...), ' '),
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeCanonicalAssessment(raw); err == nil {
				t.Fatalf("%s assessment was accepted", name)
			}
		})
	}
}

func cloneAssessmentMap(t *testing.T, value map[string]any) map[string]any {
	t.Helper()
	raw := mustJSON(t, value)
	result := map[string]any{}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
