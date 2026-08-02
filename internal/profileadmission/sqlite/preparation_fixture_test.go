package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/operatorrequest"
	"github.com/m0n0x41d/haft/internal/profileadmission"
	"github.com/m0n0x41d/haft/internal/profiledeclarationpreparation"
	profiledeclarationpreparationsqlite "github.com/m0n0x41d/haft/internal/profiledeclarationpreparation/sqlite"
	"github.com/m0n0x41d/haft/internal/profiledetector"
	"github.com/m0n0x41d/haft/internal/projectprofile"
)

const testProfileWorkInputSchema = "haft.profile-onboarding.work-input/v1"

type testProfileWorkInputJSON struct {
	Schema            string                      `json:"schema"`
	ProjectRoot       string                      `json:"project_root"`
	SuggestionRef     string                      `json:"suggestion_ref"`
	DetectorVersion   string                      `json:"detector_version"`
	PolicyVersion     string                      `json:"policy_version"`
	ObservationDigest string                      `json:"observation_digest"`
	Scopes            []testProfileScopeInputJSON `json:"scopes"`
}

type testProfileScopeInputJSON struct {
	ComponentCandidateRef string   `json:"component_candidate_ref"`
	ScopeID               string   `json:"scope_id"`
	RealizationKind       string   `json:"realization_kind"`
	EntityRef             string   `json:"entity_ref,omitempty"`
	AdmittedKindRef       string   `json:"admitted_kind_ref,omitempty"`
	GoverningPatternRefs  []string `json:"governing_pattern_refs,omitempty"`
	ContractRefs          []string `json:"contract_refs,omitempty"`
}

func prepareV3AdmissionRequest(
	t testing.TB,
	database *sql.DB,
	root projectprofile.ProjectRootV1,
	payload projectprofile.ProfileDeclarationPayload,
	suffix string,
) profileadmission.ProfileDeclarationAdmissionRequest {
	t.Helper()
	prepared := prepareV3ProfileDeclaration(t, database, root, payload, suffix)
	candidate, ok := prepared.Candidate()
	if !ok {
		t.Fatal("v3 preparation omitted its exact candidate")
	}
	request, err := profileadmission.NewProfileDeclarationAdmissionRequest(candidate)
	if err != nil {
		t.Fatalf("NewProfileDeclarationAdmissionRequest: %v", err)
	}
	return request
}

func prepareV3ProfileDeclaration(
	t testing.TB,
	database *sql.DB,
	root projectprofile.ProjectRootV1,
	payload projectprofile.ProfileDeclarationPayload,
	suffix string,
) profiledeclarationpreparationsqlite.Prepared {
	t.Helper()
	input := newV3TestWorkInput(t, root, payload)
	request, err := operatorrequest.New(
		operatorrequest.ProfileDeclaration,
		"profile-fixture:"+suffix,
		input.CanonicalJSON(),
	)
	if err != nil {
		t.Fatalf("new operator request: %v", err)
	}
	policy, err := profiledeclarationpreparation.NewHostRoutedOperatorRequestPolicy(request)
	if err != nil {
		t.Fatalf("new host-routed policy: %v", err)
	}
	outcome, err := profiledeclarationpreparationsqlite.PrepareBeforeAdmission(
		context.Background(),
		database,
		root.String(),
		input,
		policy,
		func() time.Time { return time.Now().UTC().Round(0) },
		func(context.Context) error { return nil },
	)
	if err != nil {
		t.Fatalf("PrepareBeforeAdmission: %v", err)
	}
	prepared, ok := outcome.Prepared()
	if !ok {
		detail, _ := outcome.ConflictDetail()
		t.Fatalf("profile preparation = %q: %s", outcome.Kind(), detail)
	}
	return prepared
}

func newV3TestWorkInput(
	t testing.TB,
	root projectprofile.ProjectRootV1,
	payload projectprofile.ProfileDeclarationPayload,
) profiledeclarationpreparation.ProfileOnboardingWorkInput {
	t.Helper()
	scopes := payload.Scopes().Values()
	files := testDetectorFilesForScopes(t, scopes)
	snapshot, err := profiledetector.NewSnapshot(
		root.String(),
		files,
		len(files),
		false,
	)
	if err != nil {
		t.Fatalf("NewSnapshot: %v", err)
	}
	suggestion := profiledetector.Detect(snapshot)
	declarations := testScopeDeclarations(t, scopes, suggestion.SuggestedScopes())
	document := testProfileWorkInputJSON{
		Schema:            testProfileWorkInputSchema,
		ProjectRoot:       root.String(),
		SuggestionRef:     suggestion.SuggestionRef(),
		DetectorVersion:   suggestion.DetectorVersion(),
		PolicyVersion:     profiledetector.PolicyVersion,
		ObservationDigest: snapshot.ObservationDigest(),
		Scopes:            declarations,
	}
	data, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("marshal profile Work input: %v", err)
	}
	input, err := profiledeclarationpreparation.DecodeProfileOnboardingWorkInput(
		data,
		suggestion,
	)
	if err != nil {
		t.Fatalf("DecodeProfileOnboardingWorkInput: %v", err)
	}
	return input
}

func testDetectorFilesForScopes(
	t testing.TB,
	scopes []projectprofile.RealizationScope,
) []string {
	t.Helper()
	software := 0
	nonSoftware := 0
	for _, scope := range scopes {
		switch scope.(type) {
		case projectprofile.SoftwareRealization:
			software++
		case projectprofile.NonSoftwareRealization:
			nonSoftware++
		default:
			t.Fatalf("unsupported realization scope %T", scope)
		}
	}
	if software > 1 || nonSoftware > 2 {
		t.Fatalf(
			"v3 detector input supports one software and two non-software components; got %d and %d",
			software,
			nonSoftware,
		)
	}
	files := []string{}
	if software == 1 {
		files = append(files, "go.mod", "internal/fixture.go")
	}
	if nonSoftware >= 1 {
		files = append(
			files,
			"docs/index.md",
			"docs/operations.md",
			"docs/product.md",
		)
	}
	if nonSoftware == 2 {
		files = append(files, "models/fixture.onnx")
	}
	return files
}

func testScopeDeclarations(
	t testing.TB,
	scopes []projectprofile.RealizationScope,
	candidates []profiledetector.SuggestedScope,
) []testProfileScopeInputJSON {
	t.Helper()
	if len(scopes) != len(candidates) {
		t.Fatalf("%d payload scopes cannot map to %d detector candidates", len(scopes), len(candidates))
	}
	used := make([]bool, len(candidates))
	result := make([]testProfileScopeInputJSON, len(scopes))
	for index, scope := range scopes {
		kind := testRealizationKind(t, scope)
		candidateIndex := testCandidateIndex(candidates, used, kind)
		if candidateIndex < 0 {
			t.Fatalf("detector omitted a %q component candidate", kind)
		}
		used[candidateIndex] = true
		result[index] = testScopeDeclaration(
			t,
			candidates[candidateIndex].ComponentCandidateRef(),
			scope,
		)
	}
	return result
}

func testCandidateIndex(
	candidates []profiledetector.SuggestedScope,
	used []bool,
	kind profiledetector.RealizationKind,
) int {
	for index, candidate := range candidates {
		if !used[index] && candidate.RealizationKind() == kind {
			return index
		}
	}
	return -1
}

func testScopeDeclaration(
	t testing.TB,
	component string,
	scope projectprofile.RealizationScope,
) testProfileScopeInputJSON {
	t.Helper()
	declaration := testProfileScopeInputJSON{
		ComponentCandidateRef: component,
		ScopeID:               scope.ScopeID().String(),
		RealizationKind:       string(testRealizationKind(t, scope)),
	}
	switch value := scope.(type) {
	case projectprofile.SoftwareRealization:
		declaration.EntityRef = testEntityReference(t, value.EntityReference())
	case projectprofile.NonSoftwareRealization:
		declaration.EntityRef = testEntityReference(t, value.EntityReference())
		declaration.AdmittedKindRef = testKindOrientation(t, value.KindOrientation())
		declaration.GoverningPatternRefs = testSourceUnitRefs(value.GoverningPatternRefs())
		declaration.ContractRefs = testSpecSectionRefs(value.ContractRefs())
	default:
		t.Fatalf("unsupported realization scope %T", scope)
	}
	return declaration
}

func testRealizationKind(
	t testing.TB,
	scope projectprofile.RealizationScope,
) profiledetector.RealizationKind {
	t.Helper()
	switch scope.(type) {
	case projectprofile.SoftwareRealization:
		return profiledetector.SoftwareRealization
	case projectprofile.NonSoftwareRealization:
		return profiledetector.NonSoftwareRealization
	default:
		t.Fatalf("unsupported realization scope %T", scope)
		return ""
	}
}

func testEntityReference(
	t testing.TB,
	reference projectprofile.EntityReference,
) string {
	t.Helper()
	switch value := reference.(type) {
	case projectprofile.NoEntityReference:
		return ""
	case projectprofile.ReferencedEntity:
		return value.Ref().String()
	default:
		t.Fatalf("unsupported entity reference %T", reference)
		return ""
	}
}

func testKindOrientation(
	t testing.TB,
	orientation projectprofile.KindOrientation,
) string {
	t.Helper()
	switch value := orientation.(type) {
	case projectprofile.UnspecifiedKindOrientation:
		return ""
	case projectprofile.ReferencedKindOrientation:
		return value.Ref().String()
	default:
		t.Fatalf("unsupported kind orientation %T", orientation)
		return ""
	}
}

func testSourceUnitRefs(values []projectprofile.SourceUnitRef) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = value.String()
	}
	return result
}

func testSpecSectionRefs(values []projectprofile.SpecSectionRef) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = value.String()
	}
	return result
}
