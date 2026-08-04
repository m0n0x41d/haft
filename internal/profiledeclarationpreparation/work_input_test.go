package profiledeclarationpreparation

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/profiledetector"
	"github.com/m0n0x41d/haft/internal/projectprofile"
)

func TestProfileOnboardingWorkInputBindsCurrentDetectorAndStableScope(t *testing.T) {
	root := canonicalWorkInputTestRoot(t)
	suggestion := workInputTestSuggestion(t, root, []string{
		"go.mod",
		"internal/kernel.go",
	})
	scope := suggestion.SuggestedScopes()[0]
	dto := profileOnboardingWorkInputJSON{
		Schema:            profileOnboardingWorkInputSchema,
		ProjectRoot:       root,
		SuggestionRef:     suggestion.SuggestionRef(),
		DetectorVersion:   suggestion.DetectorVersion(),
		PolicyVersion:     profiledetector.PolicyVersion,
		ObservationDigest: suggestion.Snapshot().ObservationDigest(),
		Scopes: []profileScopeDeclarationJSON{{
			ComponentCandidateRef: scope.ComponentCandidateRef(),
			ScopeID:               "product-software",
			RealizationKind:       string(profiledetector.SoftwareRealization),
			EntityRef:             "entity:product",
		}},
	}
	data := marshalProfileWorkInputTest(t, dto)

	input, err := DecodeProfileOnboardingWorkInput(data, suggestion)
	if err != nil {
		t.Fatal(err)
	}
	if !input.Valid() || input.ProjectRoot().String() != root {
		t.Fatalf("invalid sealed Work input: %#v", input)
	}
	if !strings.HasPrefix(input.Ref().String(), profileOnboardingWorkInputPrefix) {
		t.Fatalf("Work input ref = %q", input.Ref().String())
	}
	if !strings.HasPrefix(input.Digest().String(), "sha256:") {
		t.Fatalf("Work input digest = %q", input.Digest().String())
	}
	values := input.Payload().Scopes().Values()
	if len(values) != 1 || values[0].ScopeID().String() != "product-software" {
		t.Fatalf("payload scopes = %#v", values)
	}
	software, ok := values[0].(projectprofile.SoftwareRealization)
	if !ok {
		t.Fatalf("payload scope type = %T", values[0])
	}
	entity, ok := software.EntityReference().(projectprofile.ReferencedEntity)
	if !ok || entity.Ref().String() != "entity:product" {
		t.Fatalf("payload entity = %#v", software.EntityReference())
	}
}

func TestProfileEntityRelationChangeReviewPinsPredecessorAndOneDelta(
	t *testing.T,
) {
	root := canonicalWorkInputTestRoot(t)
	suggestion := workInputTestSuggestion(t, root, []string{
		"go.mod",
		"internal/kernel.go",
	})
	leftID, _ := projectprofile.NewScopeID("product-software")
	rightID, _ := projectprofile.NewScopeID("support-software")
	left, _ := projectprofile.NewSoftwareRealization(
		leftID,
		projectprofile.NoEntityReference{},
	)
	right, _ := projectprofile.NewSoftwareRealization(
		rightID,
		projectprofile.NoEntityReference{},
	)
	scopeSet, _ := projectprofile.NewScopeSet(
		[]projectprofile.RealizationScope{left, right},
	)
	current, _ := projectprofile.NewProfileDeclarationPayload(scopeSet)
	payloadDigest, _ := projectprofile.DigestProfileDeclarationPayload(current)
	admissionRef, _ := projectprofile.NewProfileDeclarationAdmissionRecordRef(
		"profile-admission:current",
	)
	admissionDigest, _ := projectprofile.NewContentDigest(
		"sha256:" + strings.Repeat("1", 64),
	)
	nextEntityRef, _ := projectprofile.NewEntityRef("entity:product")
	basis, err := NewProfileChangeBasis(
		admissionRef,
		admissionDigest,
		payloadDigest,
		projectprofile.NewLedgerRevision(4),
		leftID,
		"",
		nextEntityRef,
	)
	if err != nil {
		t.Fatal(err)
	}
	content, err := ProposeProfileEntityRelationChangeWorkInput(
		suggestion,
		current,
		basis,
	)
	if err != nil {
		t.Fatal(err)
	}
	input, err := DecodeProfileOnboardingWorkInput(content, suggestion)
	if err != nil {
		t.Fatal(err)
	}
	if err := input.ValidateProfileEntityRelationChange(current); err != nil {
		t.Fatalf("validate exact relation change: %v", err)
	}
	decodedBasis, ok := input.ProfileChangeBasis()
	if !ok || decodedBasis.LedgerRevision().Value() != 4 ||
		decodedBasis.ScopeID() != leftID {
		t.Fatalf("decoded change basis = %#v, present=%v", decodedBasis, ok)
	}
	dto := profileOnboardingWorkInputJSON{}
	if err := json.Unmarshal(content, &dto); err != nil {
		t.Fatal(err)
	}
	for index := range dto.Scopes {
		if dto.Scopes[index].ScopeID == rightID.String() {
			dto.Scopes[index].EntityRef = "entity:smuggled-delta"
		}
	}
	tampered := marshalProfileWorkInputTest(t, dto)
	tamperedInput, err := DecodeProfileOnboardingWorkInput(
		tampered,
		suggestion,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := tamperedInput.ValidateProfileEntityRelationChange(current); err == nil {
		t.Fatal("relation change accepted a second semantic delta")
	}
}

func TestProfileOnboardingWorkInputMapsEveryMixedCandidateExactlyOnce(t *testing.T) {
	root := canonicalWorkInputTestRoot(t)
	suggestion := workInputTestSuggestion(t, root, []string{
		"go.mod",
		"internal/kernel.go",
		"models/current.onnx",
	})
	scopes := suggestion.SuggestedScopes()
	if len(scopes) != 2 {
		t.Fatalf("mixed detector scopes = %d", len(scopes))
	}
	declarations := make([]profileScopeDeclarationJSON, 0, len(scopes))
	for _, scope := range scopes {
		declaration := profileScopeDeclarationJSON{
			ComponentCandidateRef: scope.ComponentCandidateRef(),
			ScopeID:               "scope-" + scope.Orientation(),
			RealizationKind:       string(scope.RealizationKind()),
		}
		if scope.RealizationKind() == profiledetector.NonSoftwareRealization {
			declaration.AdmittedKindRef = "U.Kind:model-artifact"
			declaration.GoverningPatternRefs = []string{"A.22.CGUS", "A.1"}
			declaration.ContractRefs = []string{"ModelSpec.current"}
		}
		declarations = append(declarations, declaration)
	}
	dto := profileWorkInputTestDTO(root, suggestion, declarations)
	input, err := DecodeProfileOnboardingWorkInput(
		marshalProfileWorkInputTest(t, dto),
		suggestion,
	)
	if err != nil {
		t.Fatal(err)
	}
	if input.Payload().Scopes().Len() != 2 {
		t.Fatalf("payload scopes = %d", input.Payload().Scopes().Len())
	}

	dto.Scopes = dto.Scopes[:1]
	_, err = DecodeProfileOnboardingWorkInput(
		marshalProfileWorkInputTest(t, dto),
		suggestion,
	)
	if err == nil || !strings.Contains(err.Error(), "maps 1 scope") {
		t.Fatalf("partial mixed declaration error = %v", err)
	}
}

func TestProfileOnboardingWorkInputRejectsStaleOrUnknownInput(t *testing.T) {
	root := canonicalWorkInputTestRoot(t)
	suggestion := workInputTestSuggestion(t, root, []string{
		"go.mod",
		"internal/kernel.go",
	})
	scope := suggestion.SuggestedScopes()[0]
	dto := profileWorkInputTestDTO(root, suggestion, []profileScopeDeclarationJSON{{
		ComponentCandidateRef: scope.ComponentCandidateRef(),
		ScopeID:               "software",
		RealizationKind:       string(profiledetector.SoftwareRealization),
	}})
	dto.ObservationDigest = "sha256:" + strings.Repeat("0", 64)
	_, err := DecodeProfileOnboardingWorkInput(
		marshalProfileWorkInputTest(t, dto),
		suggestion,
	)
	if err == nil || !strings.Contains(err.Error(), "observation_digest differs") {
		t.Fatalf("stale observation error = %v", err)
	}

	dto.ObservationDigest = suggestion.Snapshot().ObservationDigest()
	data := marshalProfileWorkInputTest(t, dto)
	data = append(data[:len(data)-1], []byte(`,"magic":true}`)...)
	_, err = DecodeProfileOnboardingWorkInput(data, suggestion)
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown field error = %v", err)
	}
}

func TestProfileOnboardingWorkInputRejectsInsufficientDetectorBasis(t *testing.T) {
	root := canonicalWorkInputTestRoot(t)
	suggestion := workInputTestSuggestion(t, root, []string{"README.md"})
	dto := profileWorkInputTestDTO(root, suggestion, []profileScopeDeclarationJSON{{
		ComponentCandidateRef: "component-candidate:invented",
		ScopeID:               "invented",
		RealizationKind:       string(profiledetector.NonSoftwareRealization),
	}})
	_, err := DecodeProfileOnboardingWorkInput(
		marshalProfileWorkInputTest(t, dto),
		suggestion,
	)
	if err == nil || !strings.Contains(err.Error(), "basis is insufficient") {
		t.Fatalf("insufficient detector error = %v", err)
	}
}

func TestProposeProfileOnboardingWorkInputProducesReadableRoundTrip(t *testing.T) {
	root := canonicalWorkInputTestRoot(t)
	suggestion := workInputTestSuggestion(t, root, []string{
		"go.mod",
		"internal/kernel.go",
		"models/current.onnx",
	})
	data, err := ProposeProfileOnboardingWorkInput(suggestion)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(data), "\n") || !strings.Contains(string(data), "\n  \"scopes\"") {
		t.Fatalf("proposal is not readable indented JSON:\n%s", data)
	}
	input, err := DecodeProfileOnboardingWorkInput(data, suggestion)
	if err != nil {
		t.Fatalf("decode proposed Work input: %v", err)
	}
	if input.Payload().Scopes().Len() != 2 {
		t.Fatalf("proposed scope count = %d", input.Payload().Scopes().Len())
	}
	got := []string{}
	for _, scope := range input.Payload().Scopes().Values() {
		got = append(got, scope.ScopeID().String())
	}
	if !strings.Contains(strings.Join(got, ","), "software") ||
		!strings.Contains(strings.Join(got, ","), "models") {
		t.Fatalf("proposed scope IDs = %#v", got)
	}
}

func TestProposeProfileOnboardingWorkInputRefusesInsufficientBasis(t *testing.T) {
	root := canonicalWorkInputTestRoot(t)
	suggestion := workInputTestSuggestion(t, root, []string{"README.md"})
	_, err := ProposeProfileOnboardingWorkInput(suggestion)
	if err == nil || !strings.Contains(err.Error(), "basis is insufficient") {
		t.Fatalf("proposal error = %v", err)
	}
}

func TestManualProfileFallbackCoversUnsupportedDocsAndEmptyProjects(
	t *testing.T,
) {
	testCases := []struct {
		name     string
		files    []string
		scope    ManualProfileScopeInput
		wantKind string
	}{
		{
			name:  "unsupported language",
			files: []string{"src/main.zig"},
			scope: ManualProfileScopeInput{
				ScopeID:         "software",
				Label:           "Zig application",
				RealizationKind: profiledetector.SoftwareRealization,
				EvidencePaths:   []string{"src/main.zig"},
			},
			wantKind: "software",
		},
		{
			name:  "small documentation repository",
			files: []string{"README.md"},
			scope: ManualProfileScopeInput{
				ScopeID:         "documents",
				Label:           "Project handbook",
				RealizationKind: profiledetector.NonSoftwareRealization,
				EvidencePaths:   []string{"README.md"},
			},
			wantKind: "non_software",
		},
		{
			name:  "empty repository",
			files: []string{},
			scope: ManualProfileScopeInput{
				ScopeID:         "software",
				Label:           "New application",
				RealizationKind: profiledetector.SoftwareRealization,
				EvidencePaths:   []string{},
			},
			wantKind: "software",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			root := canonicalWorkInputTestRoot(t)
			suggestion := workInputTestSuggestion(
				t,
				root,
				testCase.files,
			)
			if suggestion.Classification() !=
				profiledetector.InsufficientDetectorBasis {
				t.Fatalf(
					"fixture classification = %q",
					suggestion.Classification(),
				)
			}
			proposal, err :=
				ProposeManualProfileOnboardingWorkInput(
					suggestion,
					ManualProfileProposalInput{
						Basis: "Operator reviewed the repository purpose and supplied the missing scope.",
						Scopes: []ManualProfileScopeInput{
							testCase.scope,
						},
					},
				)
			if err != nil {
				t.Fatalf("manual proposal: %v", err)
			}
			if !strings.Contains(
				string(proposal),
				`"proposal_source": "manual_scope_proposal"`,
			) ||
				!strings.Contains(
					string(proposal),
					`"label": "`+testCase.scope.Label+`"`,
				) {
				t.Fatalf(
					"manual proposal omits readable basis:\n%s",
					proposal,
				)
			}
			input, err := DecodeProfileOnboardingWorkInput(
				proposal,
				suggestion,
			)
			if err != nil {
				t.Fatalf("decode manual proposal: %v", err)
			}
			if !input.UsesManualScopeBasis() ||
				input.ManualBasis() == "" {
				t.Fatalf(
					"manual input posture = manual:%t basis:%q",
					input.UsesManualScopeBasis(),
					input.ManualBasis(),
				)
			}
			values := input.Payload().Scopes().Values()
			if len(values) != 1 {
				t.Fatalf("manual payload scopes = %d", len(values))
			}
			switch testCase.wantKind {
			case "software":
				if _, ok := values[0].(projectprofile.SoftwareRealization); !ok {
					t.Fatalf("manual scope type = %T", values[0])
				}
			case "non_software":
				if _, ok := values[0].(projectprofile.NonSoftwareRealization); !ok {
					t.Fatalf("manual scope type = %T", values[0])
				}
			}
		})
	}
}

func TestManualProfileFallbackRejectsUnobservedEvidenceAndSupportedOverride(
	t *testing.T,
) {
	root := canonicalWorkInputTestRoot(t)
	insufficient := workInputTestSuggestion(
		t,
		root,
		[]string{"README.md"},
	)
	_, err := ProposeManualProfileOnboardingWorkInput(
		insufficient,
		ManualProfileProposalInput{
			Basis: "Operator reviewed the repository.",
			Scopes: []ManualProfileScopeInput{{
				ScopeID:         "documents",
				Label:           "Handbook",
				RealizationKind: profiledetector.NonSoftwareRealization,
				EvidencePaths:   []string{"missing.md"},
			}},
		},
	)
	if err == nil || !strings.Contains(
		err.Error(),
		"absent from the current repository observation",
	) {
		t.Fatalf("unobserved evidence error = %v", err)
	}

	_, err = ProposeManualProfileOnboardingWorkInput(
		insufficient,
		ManualProfileProposalInput{
			Basis: "Operator reviewed the repository.",
			Scopes: []ManualProfileScopeInput{{
				ScopeID:         "documents",
				Label:           "Handbook",
				RealizationKind: profiledetector.NonSoftwareRealization,
			}},
		},
	)
	if err == nil || !strings.Contains(
		err.Error(),
		"required for a non-empty repository observation",
	) {
		t.Fatalf("missing evidence error = %v", err)
	}

	supported := workInputTestSuggestion(
		t,
		root,
		[]string{"go.mod", "internal/kernel.go"},
	)
	_, err = ProposeManualProfileOnboardingWorkInput(
		supported,
		ManualProfileProposalInput{
			Basis: "Attempted detector override.",
			Scopes: []ManualProfileScopeInput{{
				ScopeID:         "documents",
				Label:           "Wrong override",
				RealizationKind: profiledetector.NonSoftwareRealization,
			}},
		},
	)
	if err == nil || !strings.Contains(
		err.Error(),
		"only when detector classification is insufficient",
	) {
		t.Fatalf("supported override error = %v", err)
	}
}

func TestManualProfileFallbackRejectsTruncatedAndTamperedCarriers(
	t *testing.T,
) {
	root := canonicalWorkInputTestRoot(t)
	truncatedSnapshot, err := profiledetector.NewSnapshot(
		root,
		[]string{"README.md"},
		2,
		true,
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ProposeManualProfileOnboardingWorkInput(
		profiledetector.Detect(truncatedSnapshot),
		ManualProfileProposalInput{
			Basis: "Incomplete scan must not be classified.",
			Scopes: []ManualProfileScopeInput{{
				ScopeID:         "documents",
				Label:           "Project handbook",
				RealizationKind: profiledetector.NonSoftwareRealization,
				EvidencePaths:   []string{"README.md"},
			}},
		},
	)
	if err == nil || !strings.Contains(
		err.Error(),
		"requires a complete repository observation",
	) {
		t.Fatalf("truncated observation error = %v", err)
	}

	suggestion := workInputTestSuggestion(
		t,
		root,
		[]string{"README.md", "docs/guide.md"},
	)
	proposal, err := ProposeManualProfileOnboardingWorkInput(
		suggestion,
		ManualProfileProposalInput{
			Basis: "The repository contains documentation.",
			Scopes: []ManualProfileScopeInput{{
				ScopeID:         "documents",
				Label:           "Project handbook",
				RealizationKind: profiledetector.NonSoftwareRealization,
				EvidencePaths: []string{
					"README.md",
					"docs/guide.md",
				},
			}},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	base := profileOnboardingWorkInputJSON{}
	if err := json.Unmarshal(proposal, &base); err != nil {
		t.Fatal(err)
	}
	testCases := []struct {
		name   string
		mutate func(*profileOnboardingWorkInputJSON)
	}{
		{
			name: "component ref",
			mutate: func(dto *profileOnboardingWorkInputJSON) {
				dto.Scopes[0].ComponentCandidateRef =
					"manual-component-candidate:sha256:" +
						strings.Repeat("0", 64)
			},
		},
		{
			name: "evidence order",
			mutate: func(dto *profileOnboardingWorkInputJSON) {
				dto.Scopes[0].EvidencePaths = []string{
					"docs/guide.md",
					"README.md",
				}
			},
		},
		{
			name: "classifier",
			mutate: func(dto *profileOnboardingWorkInputJSON) {
				dto.DetectorVersion = "unreviewed-classifier"
			},
		},
		{
			name: "observation detector",
			mutate: func(dto *profileOnboardingWorkInputJSON) {
				dto.ObservationDetectorVersion =
					"another-observation-detector"
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			tampered := base
			tampered.Scopes = append(
				[]profileScopeDeclarationJSON{},
				base.Scopes...,
			)
			testCase.mutate(&tampered)
			data, err := json.Marshal(tampered)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := DecodeProfileOnboardingWorkInput(
				data,
				suggestion,
			); err == nil {
				t.Fatal("tampered manual carrier was accepted")
			}
		})
	}
}

func profileWorkInputTestDTO(
	root string,
	suggestion profiledetector.Suggestion,
	scopes []profileScopeDeclarationJSON,
) profileOnboardingWorkInputJSON {
	return profileOnboardingWorkInputJSON{
		Schema:            profileOnboardingWorkInputSchema,
		ProjectRoot:       root,
		SuggestionRef:     suggestion.SuggestionRef(),
		DetectorVersion:   suggestion.DetectorVersion(),
		PolicyVersion:     profiledetector.PolicyVersion,
		ObservationDigest: suggestion.Snapshot().ObservationDigest(),
		Scopes:            scopes,
	}
}

func workInputTestSuggestion(
	t testing.TB,
	root string,
	files []string,
) profiledetector.Suggestion {
	t.Helper()
	snapshot, err := profiledetector.NewSnapshot(root, files, len(files), false)
	if err != nil {
		t.Fatal(err)
	}
	return profiledetector.Detect(snapshot)
}

func canonicalWorkInputTestRoot(t testing.TB) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func marshalProfileWorkInputTest(
	t testing.TB,
	value profileOnboardingWorkInputJSON,
) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
