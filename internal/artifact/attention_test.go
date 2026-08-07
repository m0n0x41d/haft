package artifact

import "testing"

func TestBuildBlockedUseAttentionItemPreservesObjectAndExactSourceReturn(t *testing.T) {
	item := BuildBlockedUseAttentionItem(BlockedUseAttentionInput{
		BearerRef:             " dec-1 ",
		EntityOrSubjectLabel:  " Spec semantic review decision ",
		FindingKind:           " evidence_freshness_drift ",
		BlockedUse:            " release reliance ",
		SourceRefs:            []string{" dec-1 ", "", " evid-1 "},
		ExactRecordNeeded:     " current EvidencePath ",
		NextAdmissibleActions: []string{" recover_exact_source_record ", " refresh_evidence "},
		ValidUntil:            " 2026-07-19 ",
	})

	if item.Object.BearerRef != "dec-1" {
		t.Fatalf("bearer_ref = %q", item.Object.BearerRef)
	}
	if item.Object.EntityOrSubjectLabel != "Spec semantic review decision" {
		t.Fatalf("label = %q", item.Object.EntityOrSubjectLabel)
	}
	if item.SourceReturn.Status != BlockedUseExactRecordNeeded {
		t.Fatalf("source status = %q", item.SourceReturn.Status)
	}
	if item.SourceReturn.ExactRecordNeeded != "current EvidencePath" {
		t.Fatalf("exact record needed = %q", item.SourceReturn.ExactRecordNeeded)
	}
	if !blockedUseAttentionHasSource(item, "dec-1") || !blockedUseAttentionHasSource(item, "evid-1") {
		t.Fatalf("source refs = %#v", item.SourceReturn.SourceRefs)
	}
	if !blockedUseAttentionHasNextAction(item, "recover_exact_source_record") {
		t.Fatalf("next actions = %#v", item.NextAdmissibleActions)
	}
}

func TestBuildBlockedUseAttentionItemMissingSourceFailsToBlockedSourceReturn(t *testing.T) {
	item := BuildBlockedUseAttentionItem(BlockedUseAttentionInput{
		BearerRef:   "prob-1",
		FindingKind: "state_assertion_drift",
		BlockedUse:  "commission preflight",
	})

	if item.SourceReturn.Status != BlockedUseMissingSourceReturn {
		t.Fatalf("source status = %q", item.SourceReturn.Status)
	}
	if !blockedUseAttentionHasNextAction(item, "recover_exact_source_record") {
		t.Fatalf("next actions = %#v", item.NextAdmissibleActions)
	}
}

func TestBuildBlockedUseAttentionItemCannotMasqueradeAsWorkOrAuthority(t *testing.T) {
	item := BuildBlockedUseAttentionItem(BlockedUseAttentionInput{
		BearerRef:         "dec-1",
		FindingKind:       "decision_basis_drift",
		BlockedUse:        "merge gate",
		SourceRefs:        []string{"dec-1"},
		ExactRecordNeeded: "",
	})

	if item.Authority != BlockedUseAttentionAuthority {
		t.Fatalf("authority = %q", item.Authority)
	}
	if item.AuthorityBoundary.WorkPlan != BlockedUseBoundaryNotWorkPlan {
		t.Fatalf("work plan boundary = %+v", item.AuthorityBoundary)
	}
	if item.AuthorityBoundary.GateDecision != BlockedUseBoundaryNotGateDecision {
		t.Fatalf("gate boundary = %+v", item.AuthorityBoundary)
	}
	if item.AuthorityBoundary.ClaimTruth != BlockedUseBoundaryNotClaimTruth {
		t.Fatalf("claim truth boundary = %+v", item.AuthorityBoundary)
	}
	if item.AuthorityBoundary.GlobalTruth != BlockedUseBoundaryNotGlobalTruth {
		t.Fatalf("global truth boundary = %+v", item.AuthorityBoundary)
	}
	if item.AuthorityBoundary.Publication != BlockedUseBoundaryNotPublication {
		t.Fatalf("publication boundary = %+v", item.AuthorityBoundary)
	}
}

func blockedUseAttentionHasSource(item BlockedUseAttentionItem, sourceRef string) bool {
	for _, candidate := range item.SourceReturn.SourceRefs {
		if candidate == sourceRef {
			return true
		}
	}

	return false
}

func blockedUseAttentionHasNextAction(item BlockedUseAttentionItem, action string) bool {
	for _, candidate := range item.NextAdmissibleActions {
		if candidate == action {
			return true
		}
	}

	return false
}
