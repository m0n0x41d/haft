package neighborhood_test

import (
	"testing"

	"github.com/m0n0x41d/haft/internal/projectmemory/neighborhood"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestWholeSnapshotStalenessCannotBecomeExactData(t *testing.T) {
	observedTypeEnv := testTypeEnvRef(t, "4")
	requiredTypeEnv := testTypeEnvRef(t, "5")
	observed := testSnapshotBasis(t, 7, observedTypeEnv)
	required := testSnapshotBasis(t, 8, requiredTypeEnv)
	cause, err := neighborhood.NewStaleSnapshotCause(observed, required)
	if err != nil {
		t.Fatal(err)
	}
	result, err := neighborhood.NewRetryRequiredResult(cause, required)
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind() != neighborhood.ResultRetryRequired ||
		result.RetryOperation() != neighborhood.RetryReloadSnapshot ||
		result.Interpretation().Structure() != neighborhood.StructureUnavailable {
		t.Fatal("stale whole snapshot did not fail as RetryRequired")
	}
}

func TestNoAdmissibleFacetAbstentionNeedsTypedIssueAndInspectedSource(t *testing.T) {
	required, err := neighborhood.NewRequiredBasisRef(
		"typeenv:missing-kind:Haft.CodeAnchor",
	)
	if err != nil {
		t.Fatal(err)
	}
	issue, err := neighborhood.NewMissingTypeBasisIssue(
		neighborhood.FacetImplementation,
		required,
	)
	if err != nil {
		t.Fatal(err)
	}
	basis, err := neighborhood.NewNoAdmissibleFacetBasis(
		[]neighborhood.FacetBasisIssue{issue},
	)
	if err != nil {
		t.Fatal(err)
	}
	source, err := neighborhood.NewInspectedSourceRef(
		"canonical:typed-memory@revision:8",
	)
	if err != nil {
		t.Fatal(err)
	}
	result, err := neighborhood.NewAbstainedResult(
		basis,
		[]neighborhood.InspectedSourceRef{source},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind() != neighborhood.ResultAbstained ||
		result.Interpretation().Authority() != neighborhood.AuthorityNotGranted {
		t.Fatal("abstention contract weakened authority boundary")
	}
	if _, err := neighborhood.NewNoAdmissibleFacetBasis(nil); err == nil {
		t.Fatal("empty no-admissible-facet basis was accepted")
	}
}

func TestExplicitBridgeIssueKeepsUnknownAndKnownBridgeDistinct(t *testing.T) {
	source, err := typedmemory.NewBoundedContextRef("context:source")
	if err != nil {
		t.Fatal(err)
	}
	target, err := typedmemory.NewBoundedContextRef("context:target")
	if err != nil {
		t.Fatal(err)
	}
	unknown, err := neighborhood.NewExplicitBridgeRequiredIssue(
		neighborhood.FacetSpecifications,
		source,
		target,
		neighborhood.UnknownBridge{},
	)
	if err != nil {
		t.Fatal(err)
	}
	bridgeRef, err := neighborhood.NewContextBridgeRef("bridge:source-target")
	if err != nil {
		t.Fatal(err)
	}
	knownBridge, err := neighborhood.NewKnownBridge(bridgeRef)
	if err != nil {
		t.Fatal(err)
	}
	known, err := neighborhood.NewExplicitBridgeRequiredIssue(
		neighborhood.FacetSpecifications,
		source,
		target,
		knownBridge,
	)
	if err != nil {
		t.Fatal(err)
	}
	if unknown.Bridge().Kind() != neighborhood.BridgeUnknown ||
		known.Bridge().Kind() != neighborhood.BridgeKnown {
		t.Fatal("bridge knowledge collapsed unknown and known")
	}
	if _, err := neighborhood.NewExplicitBridgeRequiredIssue(
		neighborhood.FacetSpecifications,
		source,
		source,
		neighborhood.UnknownBridge{},
	); err == nil {
		t.Fatal("same-context relation was mislabeled as a bridge requirement")
	}
}

func TestWholeReadRetryCauseAlgebraIsClosedAndOperationSpecific(
	t *testing.T,
) {
	observedTypeEnv := testTypeEnvRef(t, "6")
	requiredTypeEnv := testTypeEnvRef(t, "7")
	observed := testSnapshotBasis(t, 11, observedTypeEnv)
	required := testSnapshotBasis(t, 12, requiredTypeEnv)
	profile := mustProfile(t, "agent_orientation.v1")
	cursor, err := neighborhood.NewSnapshotCursor(
		observed.GraphRevision(),
		observed.TypeEnv(),
		profile,
		neighborhood.FacetEvidence,
		3,
	)
	if err != nil {
		t.Fatal(err)
	}
	projection, err := neighborhood.NewProjectionRef(
		"projection:code-index:v2",
	)
	if err != nil {
		t.Fatal(err)
	}
	staleSnapshot, err := neighborhood.NewStaleSnapshotCause(
		observed,
		required,
	)
	if err != nil {
		t.Fatal(err)
	}
	staleCursor, err := neighborhood.NewStaleCursorCause(cursor, required)
	if err != nil {
		t.Fatal(err)
	}
	rebuild, err := neighborhood.NewProjectionRebuildRequiredCause(
		projection,
		1,
		2,
	)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		cause     neighborhood.WholeReadRetryCause
		kind      neighborhood.WholeReadRetryCauseKind
		operation neighborhood.RetryOperation
	}{
		{
			cause:     staleSnapshot,
			kind:      neighborhood.RetryStaleSnapshot,
			operation: neighborhood.RetryReloadSnapshot,
		},
		{
			cause:     staleCursor,
			kind:      neighborhood.RetryStaleCursor,
			operation: neighborhood.RetryRestartFromCursor,
		},
		{
			cause:     rebuild,
			kind:      neighborhood.RetryProjectionRebuildRequired,
			operation: neighborhood.RetryRebuildProjection,
		},
	}
	seen := make(map[neighborhood.WholeReadRetryCauseKind]struct{}, len(cases))
	for _, testCase := range cases {
		result, resultErr := neighborhood.NewRetryRequiredResult(
			testCase.cause,
			required,
		)
		if resultErr != nil {
			t.Fatal(resultErr)
		}
		if testCase.cause.Kind() != testCase.kind ||
			result.RetryOperation() != testCase.operation ||
			result.Interpretation().Authority() !=
				neighborhood.AuthorityNotGranted ||
			result.Interpretation().WorkOrder() !=
				neighborhood.WorkOrderNotImplied {
			t.Fatalf("retry variant %q lost its closed semantics", testCase.kind)
		}
		if _, found := seen[testCase.kind]; found {
			t.Fatalf("retry kind %q is not unique", testCase.kind)
		}
		seen[testCase.kind] = struct{}{}
	}
}

func TestFacetBasisAndReadAbstentionAlgebrasExposeEveryClosedVariant(
	t *testing.T,
) {
	requiredType, err := neighborhood.NewRequiredBasisRef(
		"typeenv:missing-kind:Haft.CodeAnchor",
	)
	if err != nil {
		t.Fatal(err)
	}
	correspondence, err :=
		neighborhood.NewProjectionCorrespondenceManifestRef(
			"correspondence:code-anchor:v1",
		)
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := neighborhood.NewLegacyRecordRef("legacy:decision:42")
	if err != nil {
		t.Fatal(err)
	}
	resolution, err := neighborhood.NewIdentityResolutionRef(
		"identity-resolution:decision:42",
	)
	if err != nil {
		t.Fatal(err)
	}
	projection, err := neighborhood.NewProjectionRef(
		"projection:spec-index",
	)
	if err != nil {
		t.Fatal(err)
	}
	observedVersion, err := neighborhood.NewProjectionVersion(
		"projection-version:1",
	)
	if err != nil {
		t.Fatal(err)
	}
	requiredVersion, err := neighborhood.NewProjectionVersion(
		"projection-version:2",
	)
	if err != nil {
		t.Fatal(err)
	}
	sourceContext, err := typedmemory.NewBoundedContextRef("context:source")
	if err != nil {
		t.Fatal(err)
	}
	targetContext, err := typedmemory.NewBoundedContextRef("context:target")
	if err != nil {
		t.Fatal(err)
	}
	missingType, err := neighborhood.NewMissingTypeBasisIssue(
		neighborhood.FacetImplementation,
		requiredType,
	)
	if err != nil {
		t.Fatal(err)
	}
	missingCorrespondence, err :=
		neighborhood.NewMissingCorrespondenceBasisIssue(
			neighborhood.FacetImplementation,
			correspondence,
		)
	if err != nil {
		t.Fatal(err)
	}
	unresolvedIdentity, err :=
		neighborhood.NewUnresolvedLegacyIdentityIssue(
			neighborhood.FacetDecisions,
			legacy,
			resolution,
		)
	if err != nil {
		t.Fatal(err)
	}
	staleProjection, err := neighborhood.NewStaleDerivedProjectionIssue(
		neighborhood.FacetSpecifications,
		projection,
		observedVersion,
		requiredVersion,
	)
	if err != nil {
		t.Fatal(err)
	}
	explicitBridge, err := neighborhood.NewExplicitBridgeRequiredIssue(
		neighborhood.FacetSpecifications,
		sourceContext,
		targetContext,
		neighborhood.UnknownBridge{},
	)
	if err != nil {
		t.Fatal(err)
	}
	issues := []neighborhood.FacetBasisIssue{
		missingType,
		missingCorrespondence,
		unresolvedIdentity,
		staleProjection,
		explicitBridge,
	}
	wantKinds := []neighborhood.FacetBasisIssueKind{
		neighborhood.IssueMissingTypeBasis,
		neighborhood.IssueMissingCorrespondenceBasis,
		neighborhood.IssueUnresolvedLegacyIdentity,
		neighborhood.IssueStaleDerivedProjection,
		neighborhood.IssueExplicitBridgeRequired,
	}
	for index, issue := range issues {
		if issue.Kind() != wantKinds[index] {
			t.Fatalf(
				"facet issue %d kind = %q, want %q",
				index,
				issue.Kind(),
				wantKinds[index],
			)
		}
	}
	noFacet, err := neighborhood.NewNoAdmissibleFacetBasis(issues)
	if err != nil {
		t.Fatal(err)
	}

	typeEnv := testTypeEnvRef(t, "8")
	entity := testPersistedRef(t, typeEnv, "entity:unresolved")
	context, err := typedmemory.NewBoundedContextRef("context:unresolved")
	if err != nil {
		t.Fatal(err)
	}
	snapshot := testSnapshotBasis(t, 13, typeEnv)
	missingEntity, err := neighborhood.NewEntityOrContextNotFoundBasis(
		entity,
		context,
		snapshot,
	)
	if err != nil {
		t.Fatal(err)
	}
	source, err := neighborhood.NewInspectedSourceRef(
		"canonical:typed-memory@revision:13",
	)
	if err != nil {
		t.Fatal(err)
	}
	abstentions := []neighborhood.ReadAbstentionBasis{
		missingEntity,
		noFacet,
	}
	wantAbstentions := []neighborhood.ReadAbstentionBasisKind{
		neighborhood.AbstainEntityOrContextNotFound,
		neighborhood.AbstainNoAdmissibleFacet,
	}
	for index, basis := range abstentions {
		result, resultErr := neighborhood.NewAbstainedResult(
			basis,
			[]neighborhood.InspectedSourceRef{source},
		)
		if resultErr != nil {
			t.Fatal(resultErr)
		}
		if result.Basis().Kind() != wantAbstentions[index] ||
			result.Interpretation().Authority() !=
				neighborhood.AuthorityNotGranted ||
			result.Interpretation().WorkOrder() !=
				neighborhood.WorkOrderNotImplied {
			t.Fatalf(
				"abstention variant %q lost its closed semantics",
				wantAbstentions[index],
			)
		}
	}
}

func testSnapshotBasis(
	t *testing.T,
	revision uint64,
	typeEnv typedmemory.TypeEnvRef,
) neighborhood.SnapshotBasis {
	t.Helper()
	basis, err := neighborhood.NewSnapshotBasis(
		typedmemory.NewGraphRevision(revision),
		typeEnv,
		typeEnv.Digest(),
	)
	if err != nil {
		t.Fatal(err)
	}
	return basis
}
