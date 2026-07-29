package profileadmissionfixture

import (
	"encoding/json"
	"testing"

	"github.com/m0n0x41d/haft/internal/profiledetector"
	"github.com/m0n0x41d/haft/internal/profileonboarding"
	"github.com/m0n0x41d/haft/internal/projectprofile"
)

const fixtureProfileWorkInputSchema = "haft.profile-onboarding.work-input/v1"

type fixtureProfileWorkInputJSON struct {
	Schema            string                           `json:"schema"`
	ProjectRoot       string                           `json:"project_root"`
	SuggestionRef     string                           `json:"suggestion_ref"`
	DetectorVersion   string                           `json:"detector_version"`
	PolicyVersion     string                           `json:"policy_version"`
	ObservationDigest string                           `json:"observation_digest"`
	Scopes            []fixtureProfileScopeDeclaration `json:"scopes"`
}

type fixtureProfileScopeDeclaration struct {
	ComponentCandidateRef string   `json:"component_candidate_ref"`
	ScopeID               string   `json:"scope_id"`
	RealizationKind       string   `json:"realization_kind"`
	EntityRef             string   `json:"entity_ref,omitempty"`
	AdmittedKindRef       string   `json:"admitted_kind_ref,omitempty"`
	GoverningPatternRefs  []string `json:"governing_pattern_refs,omitempty"`
	ContractRefs          []string `json:"contract_refs,omitempty"`
}

func newFixtureProfileWorkInput(
	t testing.TB,
	root projectprofile.ProjectRootV1,
	payload projectprofile.ProfileDeclarationPayload,
) profileonboarding.ProfileOnboardingWorkInput {
	t.Helper()
	scopes := payload.Scopes().Values()
	suggestion := fixtureProfileSuggestion(t, root, scopes)
	declarations := fixtureProfileScopeDeclarations(t, suggestion, scopes)
	snapshot := suggestion.Snapshot()
	document := fixtureProfileWorkInputJSON{
		Schema:            fixtureProfileWorkInputSchema,
		ProjectRoot:       snapshot.ProjectRoot(),
		SuggestionRef:     suggestion.SuggestionRef(),
		DetectorVersion:   suggestion.DetectorVersion(),
		PolicyVersion:     profiledetector.PolicyVersion,
		ObservationDigest: snapshot.ObservationDigest(),
		Scopes:            declarations,
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("marshal fixture profile WorkInput: %v", err)
	}
	input, err := profileonboarding.DecodeProfileOnboardingWorkInput(
		encoded,
		suggestion,
	)
	if err != nil {
		t.Fatalf("decode fixture profile WorkInput: %v", err)
	}
	return input
}

func fixtureProfileSuggestion(
	t testing.TB,
	root projectprofile.ProjectRootV1,
	scopes []projectprofile.RealizationScope,
) profiledetector.Suggestion {
	t.Helper()
	softwareCount, nonSoftwareCount := fixtureScopeKindCounts(t, scopes)
	files := []string{}
	if softwareCount > 0 {
		files = append(files, "go.mod", "internal/fixture.go")
	}
	if nonSoftwareCount > 0 {
		files = append(files, "book.toml", "docs/index.md")
	}
	if nonSoftwareCount > 1 {
		files = append(files, "models/fixture.onnx")
	}
	if softwareCount > 1 || nonSoftwareCount > 2 {
		t.Fatalf(
			"fixture detector supports at most one software and two non-software component candidates; got %d and %d",
			softwareCount,
			nonSoftwareCount,
		)
	}
	snapshot, err := profiledetector.NewSnapshot(
		root.String(),
		files,
		len(files),
		false,
	)
	if err != nil {
		t.Fatalf("build fixture profile detector snapshot: %v", err)
	}
	suggestion := profiledetector.Detect(snapshot)
	if len(suggestion.SuggestedScopes()) != len(scopes) {
		t.Fatalf(
			"fixture detector produced %d candidates for %d declared scopes",
			len(suggestion.SuggestedScopes()),
			len(scopes),
		)
	}
	return suggestion
}

func fixtureScopeKindCounts(
	t testing.TB,
	scopes []projectprofile.RealizationScope,
) (int, int) {
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
			t.Fatalf("fixture payload contains unsupported realization scope %T", scope)
		}
	}
	return software, nonSoftware
}

func fixtureProfileScopeDeclarations(
	t testing.TB,
	suggestion profiledetector.Suggestion,
	scopes []projectprofile.RealizationScope,
) []fixtureProfileScopeDeclaration {
	t.Helper()
	candidates := suggestion.SuggestedScopes()
	used := make([]bool, len(candidates))
	result := make([]fixtureProfileScopeDeclaration, len(scopes))
	for index, scope := range scopes {
		kind := fixtureRealizationKind(t, scope)
		candidateIndex := fixtureCandidateIndex(candidates, used, kind)
		if candidateIndex < 0 {
			t.Fatalf("fixture detector omitted a %q candidate", kind)
		}
		used[candidateIndex] = true
		result[index] = fixtureScopeDeclaration(
			t,
			candidates[candidateIndex].ComponentCandidateRef(),
			scope,
		)
	}
	return result
}

func fixtureCandidateIndex(
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

func fixtureScopeDeclaration(
	t testing.TB,
	component string,
	scope projectprofile.RealizationScope,
) fixtureProfileScopeDeclaration {
	t.Helper()
	declaration := fixtureProfileScopeDeclaration{
		ComponentCandidateRef: component,
		ScopeID:               scope.ScopeID().String(),
		RealizationKind:       string(fixtureRealizationKind(t, scope)),
	}
	switch value := scope.(type) {
	case projectprofile.SoftwareRealization:
		declaration.EntityRef = fixtureEntityReference(t, value.EntityReference())
	case projectprofile.NonSoftwareRealization:
		declaration.EntityRef = fixtureEntityReference(t, value.EntityReference())
		declaration.AdmittedKindRef = fixtureKindOrientation(t, value.KindOrientation())
		declaration.GoverningPatternRefs = fixturePatternRefs(value.GoverningPatternRefs())
		declaration.ContractRefs = fixtureContractRefs(value.ContractRefs())
	default:
		t.Fatalf("fixture payload contains unsupported realization scope %T", scope)
	}
	return declaration
}

func fixtureRealizationKind(
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
		t.Fatalf("fixture payload contains unsupported realization scope %T", scope)
		return ""
	}
}

func fixtureEntityReference(
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
		t.Fatalf("fixture payload contains unsupported EntityReference %T", reference)
		return ""
	}
}

func fixtureKindOrientation(
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
		t.Fatalf("fixture payload contains unsupported KindOrientation %T", orientation)
		return ""
	}
}

func fixturePatternRefs(values []projectprofile.SourceUnitRef) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = value.String()
	}
	return result
}

func fixtureContractRefs(values []projectprofile.SpecSectionRef) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = value.String()
	}
	return result
}
