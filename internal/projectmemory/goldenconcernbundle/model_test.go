package goldenconcernbundle

import (
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestItemRoleGrammarIsClosedAndRoundTrips(t *testing.T) {
	roles := []ItemRole{
		ItemEntityOfConcern,
		ItemProblemCard,
		ItemSolutionOption,
		ItemSolutionPortfolio,
		ItemPortfolioComparison,
		ItemDecisionRecord,
		ItemSpecSection,
		ItemProjectClaim,
		ItemEvidenceRecord,
		ItemSupportingEpistemeRecord,
		ItemWorkRecord,
		ItemPerformedWorkOccurrence,
		ItemCodeAnchor,
	}
	seen := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		token := role.String()
		if token == "" {
			t.Fatalf("role %d has no canonical token", role)
		}
		if _, found := seen[token]; found {
			t.Fatalf("role token %q is duplicated", token)
		}
		seen[token] = struct{}{}
		parsed, err := parseItemRole(token)
		if err != nil {
			t.Fatalf("parse role %q: %v", token, err)
		}
		if parsed != role {
			t.Fatalf("parse role %q = %d, want %d", token, parsed, role)
		}
	}
	if _, err := parseItemRole("next_action"); err == nil {
		t.Fatal("closed GoldenConcernBundle role grammar accepted next_action")
	}
}

func TestSnapshotCoordinateAndItemSpecRejectImplicitBasis(t *testing.T) {
	typeEnv := goldenTestTypeEnvRef(t)
	contextRef, err := typedmemory.NewBoundedContextRef("haft-project")
	if err != nil {
		t.Fatalf("NewBoundedContextRef: %v", err)
	}
	observedAt := time.Date(2026, 7, 18, 15, 0, 0, 0, time.UTC)
	_, err = NewSnapshotCoordinate(
		contextRef,
		typeEnv,
		typedmemory.NewGraphRevision(0),
		observedAt,
	)
	if err == nil {
		t.Fatal("snapshot accepted an implicit revision-zero current state")
	}
	coordinate, err := NewSnapshotCoordinate(
		contextRef,
		typeEnv,
		typedmemory.NewGraphRevision(7),
		observedAt,
	)
	if err != nil {
		t.Fatalf("NewSnapshotCoordinate: %v", err)
	}
	if coordinate.GraphRevision().Value() != 7 ||
		!coordinate.ObservedAt().Equal(observedAt) {
		t.Fatal("snapshot coordinate lost its exact revision or observation time")
	}

	refKindID, err := typedmemory.NewRefKindID("Haft.ProjectRecordRef")
	if err != nil {
		t.Fatalf("NewRefKindID: %v", err)
	}
	refKind, err := typedmemory.NewRefKindRef(typeEnv, refKindID)
	if err != nil {
		t.Fatalf("NewRefKindRef: %v", err)
	}
	referenceID, err := typedmemory.NewReferenceID("record:test")
	if err != nil {
		t.Fatalf("NewReferenceID: %v", err)
	}
	reference, err := typedmemory.NewPersistedRef(refKind, referenceID)
	if err != nil {
		t.Fatalf("NewPersistedRef: %v", err)
	}
	if _, err := NewItemSpec(
		ItemProblemCard,
		reference,
		"",
	); err == nil {
		t.Fatal("item accepted an implicit admission event")
	}
	if _, err := NewItemSpec(
		ItemRole(255),
		reference,
		"event:test",
	); err == nil {
		t.Fatal("item accepted an open-ended role")
	}
}

func TestInterpretationBoundaryDoesNotDefineAWorkOrder(t *testing.T) {
	bundle := Bundle{}
	canonical, err := encodeBundleCanonical(bundle)
	if err != nil {
		t.Fatalf("encode boundary-only bundle: %v", err)
	}
	required := []string{
		"canonical_order_is_not_causal_temporal_method_or_work_order",
		"relation_paths_are_exact_inclusion_witnesses_not_applicability_or_recommendation",
		"bundle_contains_no_capability_continuation_global_phase_or_next_action",
	}
	for _, statement := range required {
		if !strings.Contains(string(canonical), statement) {
			t.Fatalf("canonical boundary omitted %q", statement)
		}
	}
}

func goldenTestTypeEnvRef(t *testing.T) typedmemory.TypeEnvRef {
	t.Helper()
	digest, err := typedmemory.NewSHA256Digest(
		"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	)
	if err != nil {
		t.Fatalf("NewSHA256Digest: %v", err)
	}
	ref, err := typedmemory.NewTypeEnvRef(digest)
	if err != nil {
		t.Fatalf("NewTypeEnvRef: %v", err)
	}
	return ref
}
