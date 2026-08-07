package decisionbinding

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
)

const testDecisionRef = "dec-20260715-typed-binding-a1b2c3d4"

func TestDecisionBindingContentBindsExactPreparedDecision(t *testing.T) {
	root := t.TempDir()
	prepared := mustPreparedDecision(t, root, testDecisionRef, decisionInputFixture())
	content, err := NewDecisionBindingContent(prepared)
	if err != nil {
		t.Fatalf("NewDecisionBindingContent: %v", err)
	}

	gotRoot, rootOK := content.ProjectRoot()
	gotRef, refOK := content.DecisionRef()
	gotInput, inputOK := content.ResolvedInput()
	gotPrepared, preparedOK := content.PreparedDecision()
	contentDigest, digestOK := content.Digest()
	contentBytes, bytesOK := content.CanonicalBytes()
	preparedBytes, _ := prepared.CanonicalBytes()
	preparedDigest, _ := prepared.Digest()
	if !rootOK || !refOK || !inputOK || !preparedOK || !digestOK || !bytesOK {
		t.Fatal("strong content accessors rejected a freshly constructed value")
	}
	if gotRoot.String() != root || gotRef != testDecisionRef {
		t.Fatalf("identity = (%q, %q), want (%q, %q)", gotRoot.String(), gotRef, root, testDecisionRef)
	}
	if gotInput.SelectedTitle != decisionInputFixture().SelectedTitle {
		t.Fatalf("selected title = %q", gotInput.SelectedTitle)
	}
	gotPreparedBytes, _ := gotPrepared.CanonicalBytes()
	if !bytes.Equal(gotPreparedBytes, preparedBytes) {
		t.Fatal("content changed the PreparedDecision snapshot")
	}
	if !bytes.Contains(contentBytes, preparedBytes) ||
		!bytes.Contains(contentBytes, []byte(preparedDigest.String())) {
		t.Fatal("binding carrier does not contain the exact PreparedDecision and digest")
	}
	if contentDigest.String() == preparedDigest.String() {
		t.Fatal("domain binding carrier digest collapsed into its prepared-content digest")
	}
}

func TestDecisionBindingContentDigestIsProjectAndDecisionSpecific(t *testing.T) {
	input := decisionInputFixture()
	first := mustDecisionBindingContent(t, t.TempDir(), testDecisionRef, input)
	second := mustDecisionBindingContent(
		t,
		t.TempDir(),
		testDecisionRef,
		input,
	)
	third := mustDecisionBindingContent(
		t,
		mustProjectRoot(t, first),
		"dec-20260715-typed-binding-b2c3d4e5",
		input,
	)

	firstDigest, _ := first.Digest()
	secondDigest, _ := second.Digest()
	thirdDigest, _ := third.Digest()
	if firstDigest.String() == secondDigest.String() {
		t.Fatal("project root did not affect decision-binding content digest")
	}
	if firstDigest.String() == thirdDigest.String() {
		t.Fatal("reserved decision identity did not affect decision-binding content digest")
	}
}

func TestDecisionBindingContentReturnsDefensiveViews(t *testing.T) {
	content := mustDecisionBindingContent(
		t,
		t.TempDir(),
		testDecisionRef,
		decisionInputFixture(),
	)
	firstBytes, _ := content.CanonicalBytes()
	firstBytes[0] = '!'
	secondBytes, secondBytesOK := content.CanonicalBytes()
	if !secondBytesOK || secondBytes[0] == '!' {
		t.Fatal("caller mutation escaped through canonical binding bytes")
	}
	firstValue, _ := content.ResolvedInput()
	firstValue.AffectedFiles[0] = "mutated.go"
	firstValue.ChoiceResult.OptionSet[0] = "mutated option"
	secondValue, secondValueOK := content.ResolvedInput()
	if !secondValueOK || secondValue.AffectedFiles[0] == "mutated.go" ||
		secondValue.ChoiceResult.OptionSet[0] == "mutated option" {
		t.Fatal("caller mutation escaped through resolved decision input")
	}
}

func TestDecisionBindingContentRejectsMissingOrRootlessPreparation(t *testing.T) {
	if _, err := NewDecisionBindingContent(artifact.PreparedDecision{}); err == nil {
		t.Fatal("zero PreparedDecision became binding content")
	}
	store := decisionBindingTestStore(t)
	reservation, err := artifact.NewDecisionReservation(testDecisionRef)
	if err != nil {
		t.Fatal(err)
	}
	_, err = artifact.PrepareDecision(
		context.Background(),
		store,
		t.TempDir(),
		reservation,
		decisionInputFixture(),
	)
	if err == nil {
		t.Fatal("PrepareDecision accepted a rootless non-.haft location")
	}
}

func decisionInputFixture() artifact.DecideInput {
	selected := "Use typed decision binding"
	problemRef := "prob-20260715-hidden-aabbccdd"
	portfolioRef := "sol-20260715-hidden-bbccddee"
	whySelected := "It separates readable review, the human act, and the later institutional effect."
	selectionPolicy := "Choose the smallest design that cannot cross-bind another authority act."
	return artifact.DecideInput{
		ProblemRef:       problemRef,
		ProblemStatement: "Agents can currently turn model-supplied decision data into a binding record without a distinct human decision act.",
		PortfolioRef:     portfolioRef,
		SelectedTitle:    selected,
		WhySelected:      whySelected,
		SelectionPolicy:  selectionPolicy,
		CounterArgument:  "The extra types add implementation cost.",
		WeakestLink:      "The final PreparedDecision snapshot is a later integration slice.",
		WhyNotOthers: []artifact.RejectionReason{{
			Variant: "sol-20260715-hidden-ccddeeff",
			Reason:  "A command-only gate cannot prove a human SpeechAct.",
		}},
		Invariants:     []string{"A model-generated field is never an approval receipt."},
		PreConditions:  []string{"The exact decision draft is canonical."},
		PostConditions: []string{"The reviewed choice has a decision-specific SpeechAct intent."},
		EvidenceReqs:   []string{"Cross-binding tests remain green."},
		Rollback: &artifact.RollbackSpec{
			Triggers:    []string{"The adapter can accept a foreign context policy."},
			Steps:       []string{"Supersede the decision and replace the adapter."},
			BlastRadius: "Decision creation only.",
		},
		RefreshTriggers:    []string{"The generic SpeechAct source contract changes."},
		Context:            "Haft v9 manual institutional acts",
		TaskContext:        "decision binding",
		Mode:               string(artifact.ModeDeep),
		SectionRefs:        []string{"SS.authority-boundary"},
		AffectedFiles:      []string{"internal/decisionbinding/content.go"},
		DecisionSubjectRef: "entity-of-concern:manual-decision-binding",
		BindingTargets: []artifact.BindingTarget{{
			Kind:       artifact.BindingTargetSymbol,
			TargetRef:  "binding:secret-ref",
			AnchorID:   "anchor:secret-ref",
			FilePath:   "internal/decisionbinding/content.go",
			SymbolName: "NewDecisionBindingContent",
			BodyHash:   "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		}},
		GovernanceTargets: []artifact.GovernanceTarget{{
			Kind: "spec_section",
			Ref:  "SS.secret-governance-ref",
		}},
		DriftWatchTargets: []artifact.DriftWatchTarget{{
			TargetRef: "drift:secret-target",
			Trigger:   "decision adapter changes",
		}},
		SpecBindingPreflight: &artifact.SpecBindingPreflight{
			State:               artifact.SpecBindingStateProvidedRefsValid,
			SelectedSectionRefs: []string{"SS.authority-boundary"},
			DecisionDraftDigest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		ChoiceResult: &artifact.ChoiceResult{
			SubjectRef:      "operator-role:secret-ref",
			OptionSet:       []string{selected, "Keep command-only gating"},
			ComparisonBasis: []string{"human authority", "cross-domain isolation"},
			ChoiceRule:      selectionPolicy,
			NextMove:        artifact.ChoiceNextMoveChooseNow,
			VariantRef:      selected,
			ProblemRefs:     []string{problemRef},
			PortfolioRef:    portfolioRef,
			Reason:          whySelected,
			Reversibility:   "Supersede with another DecisionRecord.",
			ReopenCondition: "The generic source cannot support the adapter without domain leakage.",
		},
	}
}

func mustDecisionBindingContent(
	t *testing.T,
	root string,
	ref string,
	input artifact.DecideInput,
) DecisionBindingContent {
	t.Helper()
	prepared := mustPreparedDecision(t, root, ref, input)
	content, err := NewDecisionBindingContent(prepared)
	if err != nil {
		t.Fatalf("NewDecisionBindingContent: %v", err)
	}
	return content
}

func mustPreparedDecision(
	t *testing.T,
	root string,
	ref string,
	input artifact.DecideInput,
) artifact.PreparedDecision {
	t.Helper()
	reservation, err := artifact.NewDecisionReservation(ref)
	if err != nil {
		t.Fatalf("NewDecisionReservation: %v", err)
	}
	prepared, err := artifact.PrepareDecision(
		context.Background(),
		decisionBindingTestStore(t),
		filepath.Join(root, ".haft"),
		reservation,
		input,
	)
	if err != nil {
		t.Fatalf("PrepareDecision: %v", err)
	}
	return prepared
}

func decisionBindingTestStore(t *testing.T) artifact.ArtifactStore {
	t.Helper()
	return decisionBindingReadStore{}
}

type decisionBindingReadStore struct {
	artifact.ArtifactStore
}

func (decisionBindingReadStore) Get(
	_ context.Context,
	id string,
) (*artifact.Artifact, error) {
	return nil, fmt.Errorf("artifact %s is not present in the test source set", id)
}

func (decisionBindingReadStore) ListByKind(
	_ context.Context,
	_ artifact.Kind,
	_ int,
) ([]*artifact.Artifact, error) {
	return nil, nil
}

func mustProjectRoot(t *testing.T, content DecisionBindingContent) string {
	t.Helper()
	root, ok := content.ProjectRoot()
	if !ok {
		t.Fatal("content has no project root")
	}
	return root.String()
}
