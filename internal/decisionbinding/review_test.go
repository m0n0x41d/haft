package decisionbinding

import (
	"strings"
	"testing"
)

func TestDecisionReviewCardShowsSemanticsWithoutMachineCopyWork(t *testing.T) {
	root := t.TempDir()
	input := decisionInputFixture()
	content := mustDecisionBindingContent(t, root, testDecisionRef, input)
	card, err := content.ReviewCard()
	if err != nil {
		t.Fatalf("ReviewCard: %v", err)
	}
	review, reviewOK := card.Text()
	if !reviewOK {
		t.Fatal("fresh review card has no canonical text")
	}

	required := []string{
		"Selected choice\n" + input.SelectedTitle,
		"Problem\n" + input.ProblemStatement,
		"Project: " + root,
		"Affected code object: NewDecisionBindingContent in internal/decisionbinding/content.go",
		"Project memory will bind the selected choice as a DecisionRecord.",
		"It does not grant execution authority or create a WorkCommission.",
		"Supersede with another DecisionRecord.",
		"Canceling creates no DecisionRecord, decision authority, WorkCommission, code change, specification change, or project-profile change.",
		"An inert prepared review carrier may remain for audit or exact retry; its existence alone cannot satisfy a decision gate or institute any effect.",
	}
	for _, fragment := range required {
		if !strings.Contains(review, fragment) {
			t.Errorf("review omits %q\n%s", fragment, review)
		}
	}

	prepared, _ := content.PreparedDecision()
	inputDigest, _ := prepared.Digest()
	contentDigest, _ := content.Digest()
	forbidden := []string{
		testDecisionRef,
		input.ProblemRef,
		input.PortfolioRef,
		input.SectionRefs[0],
		input.DecisionSubjectRef,
		input.BindingTargets[0].TargetRef,
		input.BindingTargets[0].AnchorID,
		input.BindingTargets[0].BodyHash,
		input.GovernanceTargets[0].Ref,
		input.DriftWatchTargets[0].TargetRef,
		input.SpecBindingPreflight.DecisionDraftDigest,
		input.ChoiceResult.SubjectRef,
		inputDigest.String(),
		contentDigest.String(),
		"sha256:",
	}
	for _, fragment := range forbidden {
		if strings.Contains(review, fragment) {
			t.Errorf("review leaked machine identity/digest %q\n%s", fragment, review)
		}
	}
}

func TestDecisionReviewCardReturnsDefensiveSlices(t *testing.T) {
	content := mustDecisionBindingContent(
		t,
		t.TempDir(),
		testDecisionRef,
		decisionInputFixture(),
	)
	card, err := content.ReviewCard()
	if err != nil {
		t.Fatalf("ReviewCard: %v", err)
	}
	first, firstOK := card.Scope()
	if !firstOK || len(first) == 0 {
		t.Fatal("review scope is absent")
	}
	first[0] = "mutated"
	second, secondOK := card.Scope()
	if !secondOK || len(second) == 0 {
		t.Fatal("review card became invalid")
	}
	if second[0] == "mutated" {
		t.Fatal("caller mutation escaped through review scope")
	}
}

func TestDecisionReviewCardRejectsTextIncompatibleWithManualSpeechActSource(t *testing.T) {
	input := decisionInputFixture()
	input.ProblemStatement = "problem" + string(rune(0)) + "with unsupported control"
	content := mustDecisionBindingContent(t, t.TempDir(), testDecisionRef, input)
	_, err := content.ReviewCard()
	if err == nil {
		t.Fatal("review accepted unsupported control text")
	}
}
