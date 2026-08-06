package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/profiledetector"
	"github.com/m0n0x41d/haft/internal/profileonboarding"
)

func TestProfileInspectAndProposeCommandsAreReadOnlyPublicSurfaces(t *testing.T) {
	t.Parallel()

	commands := []*struct {
		name    string
		command commandContract
	}{
		{
			name: "inspect",
			command: commandContract{
				value: profileInspectCmd,
				use:   profileInspectCmd.Use,
				long:  profileInspectCmd.Long,
				args:  profileInspectCmd.Args,
				json:  profileInspectCmd.Flags().Lookup("json") != nil,
				full:  profileInspectCmd.Flags().Lookup("full-evidence") != nil,
			},
		},
		{
			name: "propose",
			command: commandContract{
				value: profileProposeCmd,
				use:   profileProposeCmd.Use,
				long:  profileProposeCmd.Long,
				args:  profileProposeCmd.Args,
				json:  profileProposeCmd.Flags().Lookup("json") != nil,
				full:  profileProposeCmd.Flags().Lookup("full-evidence") != nil,
			},
		},
	}
	for _, fixture := range commands {
		t.Run(fixture.name, func(t *testing.T) {
			if fixture.command.use != fixture.name {
				t.Fatalf("command use = %q", fixture.command.use)
			}
			if err := fixture.command.args(fixture.command.value, []string{"payload"}); err == nil {
				t.Fatal("read-only profile surface accepted positional mutation input")
			}
			if !fixture.command.json {
				t.Fatal("read-only profile surface omitted --json")
			}
			if !fixture.command.full {
				t.Fatal("read-only profile surface omitted --full-evidence")
			}
			for _, want := range []string{"orientation", "mutat"} {
				if !strings.Contains(strings.ToLower(fixture.command.long), want) {
					t.Fatalf("command help omitted %q:\n%s", want, fixture.command.long)
				}
			}
		})
	}
	if !strings.Contains(strings.ToLower(profileProposeCmd.Long), "non-binding") {
		t.Fatalf("profile propose help omitted non-binding boundary:\n%s", profileProposeCmd.Long)
	}
}

type commandContract struct {
	value *cobra.Command
	use   string
	long  string
	args  func(*cobra.Command, []string) error
	json  bool
	full  bool
}

func TestExecuteProfileInspectionReportsAutoAndEvidenceWithoutWrites(t *testing.T) {
	fixture := newCLIProfileOnboardLedgerFixture(t)
	writeProfileInspectionFixture(t, fixture.root, "go.mod")
	writeProfileInspectionFixture(t, fixture.root, "internal/kernel.go")

	response, err := executeProfileInspection(context.Background(), fixture.root, false)
	if err != nil {
		t.Fatal(err)
	}
	if response.Kind != profileInspectionRecordKind {
		t.Fatalf("kind = %q", response.Kind)
	}
	if response.CanonicalProfile.Kind != "auto" {
		t.Fatalf("canonical profile = %#v", response.CanonicalProfile)
	}
	if response.Suggestion.Kind != "suggested_scopes" {
		t.Fatalf("suggestion = %#v", response.Suggestion)
	}
	if response.Suggestion.Classification != string(profiledetector.SoftwareSignals) {
		t.Fatalf("classification = %q", response.Suggestion.Classification)
	}
	if len(response.Suggestion.SuggestedScopes) != 1 {
		t.Fatalf("suggested scopes = %#v", response.Suggestion.SuggestedScopes)
	}
	if response.Suggestion.SuggestedScopes[0].Orientation != "software" {
		t.Fatalf("software suggestion = %#v", response.Suggestion.SuggestedScopes[0])
	}
	if !strings.HasPrefix(response.Suggestion.ObservationDigest, "sha256:") {
		t.Fatalf("observation digest = %q", response.Suggestion.ObservationDigest)
	}
	if response.Relation.Kind != "not_declared" {
		t.Fatalf("relation = %#v", response.Relation)
	}
	if response.Relation.DetectorMayMutate || response.Relation.DetectorMaySatisfyGate {
		t.Fatalf("detector crossed authority boundary: %#v", response.Relation)
	}
	assertCLIProfileOnboardMutationCounts(t, fixture.root, 0)
	projectionPath := filepath.Join(fixture.root, ".haft", "project-profile.yaml")
	if _, statErr := os.Stat(projectionPath); !os.IsNotExist(statErr) {
		t.Fatalf("read-only inspection wrote profile projection: %v", statErr)
	}
}

func TestMixedProposalKeepsPartialComponentsButMintsNoScopeIdentity(t *testing.T) {
	t.Parallel()

	root := mustCLIProfileOnboardPhysicalPath(t, t.TempDir())
	files := []string{"go.mod", "internal/kernel.go", "models/current.onnx"}
	snapshot, err := profiledetector.NewSnapshot(root, files, len(files), false)
	if err != nil {
		t.Fatal(err)
	}
	view := profileSuggestionViewFromDomain(profiledetector.Detect(snapshot), false)
	if view.Kind != "underdetermined" {
		t.Fatalf("mixed proposal kind = %q", view.Kind)
	}
	if !reflect.DeepEqual(view.MissingBasis, []string{"stable_scope_identity"}) {
		t.Fatalf("missing basis = %#v", view.MissingBasis)
	}
	if len(view.SuggestedScopes) != 0 || len(view.PartialSuggestions) != 2 {
		t.Fatalf("mixed suggestions = %#v / %#v", view.SuggestedScopes, view.PartialSuggestions)
	}
	encoded, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, []byte(`"scope_id"`)) {
		t.Fatalf("read-only proposal minted stable ScopeID: %s", encoded)
	}
	if bytes.Contains(encoded, []byte(`"entity_ref"`)) ||
		bytes.Contains(encoded, []byte(`"admitted_kind_ref"`)) {
		t.Fatalf("read-only proposal fabricated admitted identity/kind: %s", encoded)
	}
}

func TestProfileSuggestionEvidenceIsBoundedByDefaultAndExactOnRequest(t *testing.T) {
	t.Parallel()

	root := mustCLIProfileOnboardPhysicalPath(t, t.TempDir())
	files := []string{"go.mod"}
	for index := 0; index < profileEvidenceWindowLimit+3; index++ {
		files = append(files, filepath.ToSlash(filepath.Join("internal", fmt.Sprintf("unit-%02d.go", index))))
	}
	snapshot, err := profiledetector.NewSnapshot(root, files, len(files), false)
	if err != nil {
		t.Fatal(err)
	}
	suggestion := profiledetector.Detect(snapshot)
	bounded := profileSuggestionViewFromDomain(suggestion, false)
	if bounded.Scan.ObservedFileCount != len(files) {
		t.Fatalf("observed file count = %d, want %d", bounded.Scan.ObservedFileCount, len(files))
	}
	if len(bounded.Scan.ObservedFiles) != profileEvidenceWindowLimit || !bounded.Scan.ObservedFilesTruncated {
		t.Fatalf("bounded observed files = %d, truncated = %v", len(bounded.Scan.ObservedFiles), bounded.Scan.ObservedFilesTruncated)
	}
	if len(bounded.SuggestedScopes) != 1 {
		t.Fatalf("bounded scopes = %#v", bounded.SuggestedScopes)
	}
	software := bounded.SuggestedScopes[0]
	if software.PositiveSignalCount != len(files) {
		t.Fatalf("positive signal count = %d, want %d", software.PositiveSignalCount, len(files))
	}
	if len(software.PositiveSignals) != profileEvidenceWindowLimit || !software.PositiveSignalsTruncated {
		t.Fatalf("bounded signals = %d, truncated = %v", len(software.PositiveSignals), software.PositiveSignalsTruncated)
	}
	full := profileSuggestionViewFromDomain(suggestion, true)
	if len(full.Scan.ObservedFiles) != len(files) || full.Scan.ObservedFilesTruncated {
		t.Fatalf("full observed files = %d, truncated = %v", len(full.Scan.ObservedFiles), full.Scan.ObservedFilesTruncated)
	}
	if len(full.SuggestedScopes[0].PositiveSignals) != len(files) || full.SuggestedScopes[0].PositiveSignalsTruncated {
		t.Fatalf(
			"full signals = %d, truncated = %v",
			len(full.SuggestedScopes[0].PositiveSignals),
			full.SuggestedScopes[0].PositiveSignalsTruncated,
		)
	}
}

func TestDeclaredProfileWinsWhenDetectorClassesConflict(t *testing.T) {
	t.Parallel()

	canonical := canonicalProfileView{
		Kind:         "declared",
		SemanticRole: "canonical_admitted_profile",
		Scopes: []canonicalProfileScopeView{{
			ScopeID:         "publication",
			RealizationKind: string(profiledetector.NonSoftwareRealization),
		}},
	}
	suggestion := profileSuggestionView{
		Kind:           "suggested_scopes",
		Classification: string(profiledetector.SoftwareSignals),
		SuggestedScopes: []profileSuggestedScopeView{{
			ComponentCandidateRef: "profile-component-suggestion:sha256:test",
			RealizationKind:       string(profiledetector.SoftwareRealization),
			Orientation:           "software",
		}},
	}
	relation := compareCanonicalProfileAndSuggestion(canonical, suggestion)
	if relation.Kind != "conflicts_with_declared" {
		t.Fatalf("relation = %#v", relation)
	}
	if relation.BindingSource != "sqlite_profile_admission_ledger" {
		t.Fatalf("binding source = %q", relation.BindingSource)
	}
	if relation.DetectorMayMutate || relation.DetectorMaySatisfyGate {
		t.Fatalf("detector displaced declared authority: %#v", relation)
	}
}

func TestWriteProfileProposalResponseStatesNonBindingBoundary(t *testing.T) {
	t.Parallel()

	response := profileProposalResponse{
		Kind:         profileProposalRecordKind,
		ProjectRoot:  "/tmp/profile-proposal",
		ProjectID:    "qnt_profile",
		SemanticRole: "non_binding_orientation",
		Suggestion: profileSuggestionView{
			Kind:              "suggested_scopes",
			Classification:    string(profiledetector.NonSoftwareSignals),
			ConfidencePosture: string(profiledetector.SupportedConfidence),
			SuggestedScopes: []profileSuggestedScopeView{{
				ComponentCandidateRef: "profile-component-suggestion:sha256:test",
				RealizationKind:       string(profiledetector.NonSoftwareRealization),
				Orientation:           "documents",
			}},
		},
		ReviewCandidate: profileReviewCandidate{
			Path:  ".haft/profile-declaration-review.json",
			State: "created",
		},
		MutationPerformed:  true,
		AdmissionPerformed: false,
	}
	output := &bytes.Buffer{}
	if err := writeProfileProposalResponse(output, response, false); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Project profile proposal (non-binding)",
		"documents [non_software]",
		"Review candidate: .haft/profile-declaration-review.json (created).",
		"No profile declaration or admission was performed.",
		"haft profile declare",
	} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("proposal output omitted %q:\n%s", want, output.String())
		}
	}
}

func TestPrepareProfileReviewCandidateCreatesReadableNoClobberCarrier(t *testing.T) {
	t.Parallel()

	root := mustCLIProfileOnboardPhysicalPath(t, t.TempDir())
	if err := os.MkdirAll(filepath.Join(root, ".haft"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeProfileInspectionFixture(t, root, "go.mod")
	writeProfileInspectionFixture(t, root, "internal/kernel.go")
	suggestion, err := profiledetector.Inspect(root)
	if err != nil {
		t.Fatal(err)
	}
	created, err := prepareProfileReviewCandidate(root, suggestion)
	if err != nil {
		t.Fatal(err)
	}
	if created.State != "created" || created.Path != profileDeclarationReviewRelativePath() {
		t.Fatalf("created review candidate = %#v", created)
	}
	path := profileDeclarationReviewPath(root)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("review carrier mode = %o", info.Mode().Perm())
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := profileonboarding.DecodeProfileOnboardingWorkInput(content, suggestion); err != nil {
		t.Fatalf("decode installed review candidate: %v", err)
	}
	reused, err := prepareProfileReviewCandidate(root, suggestion)
	if err != nil {
		t.Fatal(err)
	}
	if reused.State != "reused" {
		t.Fatalf("reused review candidate = %#v", reused)
	}
	if err := os.WriteFile(path, []byte("operator edit"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareProfileReviewCandidate(root, suggestion); err == nil {
		t.Fatal("proposal silently replaced an existing different review carrier")
	}
	observed, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(observed) != "operator edit" {
		t.Fatalf("different review carrier was overwritten: %q", observed)
	}
}

func writeProfileInspectionFixture(t *testing.T, root string, relative string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("fixture"), 0o644); err != nil {
		t.Fatal(err)
	}
}
