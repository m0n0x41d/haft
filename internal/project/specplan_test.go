package project

import "testing"

func TestBuildSpecPlan_GroupsByKindDependencyAndAffectedArea(t *testing.T) {
	report := SpecCoverageReport{
		Sections: []SpecCoverageSection{
			specPlanTestSection("TS.checkout.001", "acceptance", SpecCoverageUncovered, []string{"TS.role.001"}, nil),
			specPlanTestSection("TS.checkout.002", "acceptance", SpecCoverageStale, []string{"TS.role.001"}, nil),
			specPlanTestSection("TS.checkout.003", "acceptance", SpecCoverageUncovered, []string{"TS.boundary.001"}, nil),
			specPlanTestSection("TS.search.001", "acceptance", SpecCoverageUncovered, []string{"TS.role.001"}, nil),
			specPlanTestSection("TS.checkout.004", "interfaces", SpecCoverageUncovered, []string{"TS.role.001"}, nil),
			specPlanTestSection("TS.checkout.005", "acceptance", SpecCoverageVerified, []string{"TS.role.001"}, nil),
		},
	}

	plan := BuildSpecPlan(report)

	if plan.Summary.TotalCandidates != 5 {
		t.Fatalf("total_candidates = %d, want 5", plan.Summary.TotalCandidates)
	}
	if len(plan.Proposals) != 4 {
		t.Fatalf("proposals = %#v, want 4 grouped proposals", plan.Proposals)
	}

	group := specPlanTestProposal(plan, "acceptance", "checkout", []string{"TS.role.001"})
	if got := group.SectionRefs; !sameStrings(got, []string{"TS.checkout.001", "TS.checkout.002"}) {
		t.Fatalf("checkout role section_refs = %#v, want grouped stale+uncovered refs", got)
	}
	if got := group.States; !sameCoverageStates(got, []SpecCoverageState{SpecCoverageStale, SpecCoverageUncovered}) {
		t.Fatalf("checkout role states = %#v, want stale+uncovered", got)
	}
}

func TestBuildSpecPlan_UsesTargetRefsAsAffectedArea(t *testing.T) {
	report := SpecCoverageReport{
		Sections: []SpecCoverageSection{
			specPlanTestSection("ES.tests.001", "test-strategy", SpecCoverageUncovered, nil, []string{"TS.checkout.001"}),
			specPlanTestSection("ES.tests.002", "test-strategy", SpecCoverageUncovered, nil, []string{"TS.checkout.002"}),
		},
	}

	plan := BuildSpecPlan(report)

	if len(plan.Proposals) != 1 {
		t.Fatalf("proposals = %#v, want one target-area group", plan.Proposals)
	}
	if got := plan.Proposals[0].AffectedArea; got != "checkout" {
		t.Fatalf("affected_area = %q, want checkout", got)
	}
	if got := plan.Proposals[0].SectionRefs; !sameStrings(got, []string{"ES.tests.001", "ES.tests.002"}) {
		t.Fatalf("section_refs = %#v, want both enabling sections", got)
	}
}

func TestBuildSpecPlan_ExposesNeutralReviewActionsAndDraftAuthority(t *testing.T) {
	plan := BuildSpecPlan(SpecCoverageReport{
		Sections: []SpecCoverageSection{
			specPlanTestSection("TS.checkout.001", "acceptance", SpecCoverageUncovered, nil, nil),
		},
	})

	if plan.Authority != SpecPlanAuthorityNotice {
		t.Fatalf("authority = %q, want proposal authority notice", plan.Authority)
	}
	if got := specPlanTestReviewActionKinds(plan.ReviewActions); !sameStrings(got, []string{"accept", "discard", "merge", "split"}) {
		t.Fatalf("review action kinds = %#v, want accept/merge/split/discard", got)
	}
	if len(plan.Proposals) != 1 {
		t.Fatalf("proposals = %#v, want one proposal", plan.Proposals)
	}
	if got := plan.Proposals[0].DecisionRecordDraft.Kind; got != "DecisionRecord" {
		t.Fatalf("draft kind = %q, want DecisionRecord", got)
	}
	if got := plan.Proposals[0].DecisionRecordDraft.SectionRefs; !sameStrings(got, []string{"TS.checkout.001"}) {
		t.Fatalf("draft section refs = %#v, want exact section refs", got)
	}
	if plan.Proposals[0].DecisionRecordDraft.CounterArgument == "" {
		t.Fatalf("draft counterargument is empty: %#v", plan.Proposals[0].DecisionRecordDraft)
	}
	if len(plan.Proposals[0].DecisionRecordDraft.WhyNotOthers) == 0 {
		t.Fatalf("draft rejected alternatives are empty: %#v", plan.Proposals[0].DecisionRecordDraft)
	}
}

func TestSpecPlanReviewActions_OnlyAcceptIsExecutable(t *testing.T) {
	plan := BuildSpecPlan(SpecCoverageReport{
		Sections: []SpecCoverageSection{
			specPlanTestSection("TS.checkout.001", "acceptance", SpecCoverageUncovered, nil, nil),
		},
	})

	actions := specPlanTestReviewActionsByKind(plan.ReviewActions)
	for _, kind := range []SpecPlanActionKind{
		SpecPlanActionAccept,
		SpecPlanActionMerge,
		SpecPlanActionSplit,
		SpecPlanActionDiscard,
	} {
		if _, ok := actions[kind]; !ok {
			t.Fatalf("missing action %s in %#v", kind, plan.ReviewActions)
		}
	}

	if !actions[SpecPlanActionAccept].Executable {
		t.Fatalf("accept action = %#v, want executable", actions[SpecPlanActionAccept])
	}

	for _, kind := range []SpecPlanActionKind{
		SpecPlanActionMerge,
		SpecPlanActionSplit,
		SpecPlanActionDiscard,
	} {
		action := actions[kind]
		if action.Executable {
			t.Fatalf("%s action = %#v, want non-executable", kind, action)
		}
		if action.CommandGap == "" {
			t.Fatalf("%s action has empty command gap: %#v", kind, action)
		}
	}
}

func TestParseSpecPlanActionKindRejectsUnknownAction(t *testing.T) {
	kind, err := ParseSpecPlanActionKind("accept")
	if err != nil {
		t.Fatalf("parse accept: %v", err)
	}
	if kind != SpecPlanActionAccept {
		t.Fatalf("kind = %q, want accept", kind)
	}

	_, err = ParseSpecPlanActionKind("approve")
	if err == nil {
		t.Fatal("ParseSpecPlanActionKind returned nil error for unknown action")
	}
}

func specPlanTestSection(
	id string,
	specKind string,
	state SpecCoverageState,
	dependsOn []string,
	targetRefs []string,
) SpecCoverageSection {
	return SpecCoverageSection{
		SectionID:    id,
		Title:        id,
		DocumentKind: "target-system",
		SpecKind:     specKind,
		Path:         ".haft/specs/target-system.md",
		DependsOn:    dependsOn,
		TargetRefs:   targetRefs,
		State:        state,
		Why:          []string{"coverage fixture"},
		NextAction:   "review fixture",
	}
}

func specPlanTestProposal(
	report SpecPlanReport,
	specKind string,
	affectedArea string,
	dependencyRefs []string,
) SpecPlanProposal {
	for _, proposal := range report.Proposals {
		if proposal.SpecKind != specKind {
			continue
		}
		if proposal.AffectedArea != affectedArea {
			continue
		}
		if !sameStrings(proposal.DependencyRefs, dependencyRefs) {
			continue
		}

		return proposal
	}

	return SpecPlanProposal{}
}

func specPlanTestReviewActionKinds(actions []SpecPlanReviewAction) []string {
	kinds := make([]string, 0, len(actions))

	for _, action := range actions {
		kinds = append(kinds, string(action.Kind))
	}

	return sortedUniqueStrings(kinds)
}

func specPlanTestReviewActionsByKind(actions []SpecPlanReviewAction) map[SpecPlanActionKind]SpecPlanReviewAction {
	actionsByKind := map[SpecPlanActionKind]SpecPlanReviewAction{}

	for _, action := range actions {
		actionsByKind[action.Kind] = action
	}

	return actionsByKind
}

func sameStrings(left []string, right []string) bool {
	left = sortedUniqueStrings(left)
	right = sortedUniqueStrings(right)
	if len(left) != len(right) {
		return false
	}

	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}

	return true
}

func sameCoverageStates(left []SpecCoverageState, right []SpecCoverageState) bool {
	leftStrings := make([]string, 0, len(left))
	rightStrings := make([]string, 0, len(right))

	for _, state := range left {
		leftStrings = append(leftStrings, string(state))
	}
	for _, state := range right {
		rightStrings = append(rightStrings, string(state))
	}

	return sameStrings(leftStrings, rightStrings)
}
