package neighborhood_test

import (
	"testing"

	"github.com/m0n0x41d/haft/internal/projectmemory/neighborhood"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestFacetCoverageKeepsFivePosturesDistinct(t *testing.T) {
	complete := neighborhood.NewCompleteCoverage(0)
	if complete.Kind() != neighborhood.CoverageComplete ||
		complete.Included() != 0 {
		t.Fatal("known-empty Complete coverage is not preserved")
	}

	profile := mustProfile(t, "agent_orientation.v1")
	typeEnv := testTypeEnvRef(t, "c")
	cursor, err := neighborhood.NewSnapshotCursor(
		typedmemory.NewGraphRevision(7),
		typeEnv,
		profile,
		neighborhood.FacetDecisions,
		4,
	)
	if err != nil {
		t.Fatal(err)
	}
	partial, err := neighborhood.NewPartialCoverage(3, 1, cursor)
	if err != nil {
		t.Fatal(err)
	}
	if partial.Kind() != neighborhood.CoveragePartial ||
		partial.OmittedAtLeast() != 1 ||
		!partial.Cursor().Valid() {
		t.Fatal("partial coverage lost omission or cursor basis")
	}

	applicability, err := neighborhood.NewApplicabilityBasisRef(
		"profile:implementation-not-requested",
	)
	if err != nil {
		t.Fatal(err)
	}
	notApplicable, err := neighborhood.NewNotApplicableCoverage(applicability)
	if err != nil {
		t.Fatal(err)
	}
	missing, err := neighborhood.NewMissingBasisRef(
		"basis:correspondence-manifest-absent",
	)
	if err != nil {
		t.Fatal(err)
	}
	unavailable, err := neighborhood.NewUnavailableCoverage(missing)
	if err != nil {
		t.Fatal(err)
	}
	retry, err := neighborhood.NewRetryBasisRef(
		"retry:code-projection-epoch-12",
	)
	if err != nil {
		t.Fatal(err)
	}
	stale, err := neighborhood.NewStaleCoverage(retry)
	if err != nil {
		t.Fatal(err)
	}
	kinds := []neighborhood.FacetCoverageKind{
		complete.Kind(),
		partial.Kind(),
		notApplicable.Kind(),
		unavailable.Kind(),
		stale.Kind(),
	}
	if !allCoverageKindsUnique(kinds) {
		t.Fatalf("coverage kinds collapsed: %#v", kinds)
	}
}

func TestSnapshotCursorBindsEveryProjectionCoordinate(t *testing.T) {
	profile := mustProfile(t, "agent_orientation.v1")
	typeEnv := testTypeEnvRef(t, "d")
	first, err := neighborhood.NewSnapshotCursor(
		typedmemory.NewGraphRevision(4),
		typeEnv,
		profile,
		neighborhood.FacetProblems,
		3,
	)
	if err != nil {
		t.Fatal(err)
	}
	second, err := neighborhood.NewSnapshotCursor(
		typedmemory.NewGraphRevision(5),
		typeEnv,
		profile,
		neighborhood.FacetProblems,
		3,
	)
	if err != nil {
		t.Fatal(err)
	}
	if first.Digest() == second.Digest() {
		t.Fatal("cursor digest ignored graph revision")
	}
	otherFacet, err := neighborhood.NewSnapshotCursor(
		typedmemory.NewGraphRevision(4),
		typeEnv,
		profile,
		neighborhood.FacetDecisions,
		3,
	)
	if err != nil {
		t.Fatal(err)
	}
	if first.Digest() == otherFacet.Digest() {
		t.Fatal("cursor digest ignored facet")
	}
}

func TestPartialCoverageRejectsImplicitContinuation(t *testing.T) {
	if _, err := neighborhood.NewPartialCoverage(
		3,
		1,
		neighborhood.SnapshotCursor{},
	); err == nil {
		t.Fatal("partial coverage accepted an implicit cursor")
	}
	profile := mustProfile(t, "agent_orientation.v1")
	cursor, err := neighborhood.NewSnapshotCursor(
		typedmemory.NewGraphRevision(2),
		testTypeEnvRef(t, "e"),
		profile,
		neighborhood.FacetEvidence,
		1,
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := neighborhood.NewPartialCoverage(1, 0, cursor); err == nil {
		t.Fatal("partial coverage accepted zero omission")
	}
}

func allCoverageKindsUnique(values []neighborhood.FacetCoverageKind) bool {
	seen := make(map[neighborhood.FacetCoverageKind]struct{}, len(values))
	for _, value := range values {
		if _, found := seen[value]; found {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}
