package neighborhood

import (
	"testing"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestExactInterpretationIsDerivedFromIndependentItemPostures(t *testing.T) {
	current, valid := NewItemPostures(
		SemanticTypedActive,
		LifecycleActive,
		EvidenceCurrent,
		ProjectionCurrent,
	)
	if !valid {
		t.Fatal("current posture is invalid")
	}
	exact := interpretationForExactNeighborhood(
		[]ItemPostures{current},
		[]RelationalRecordItemPosture{
			legacyUnqualifiedAssertionItemPosture(),
		},
	)
	if !exact.Valid() ||
		exact.Structure() != StructureExactAtSnapshot ||
		exact.Identity() != IdentityExact ||
		exact.RelationalRecords() !=
			RelationalRecordsLegacyUnqualifiedAssertions ||
		exact.Ranking() != RankingNotApplicable ||
		exact.Completeness() != CompletenessFacetLocal ||
		exact.HydrateBeforeReliance() {
		t.Fatalf("exact interpretation = %#v", exact)
	}

	unknownEvidence, valid := NewItemPostures(
		SemanticTypedActive,
		LifecycleActive,
		EvidenceUnknown,
		ProjectionCurrent,
	)
	if !valid {
		t.Fatal("unknown-evidence posture is invalid")
	}
	requiresHydration := interpretationForExactNeighborhood(
		[]ItemPostures{unknownEvidence},
		[]RelationalRecordItemPosture{
			legacyUnqualifiedAssertionItemPosture(),
		},
	)
	if !requiresHydration.Valid() ||
		!requiresHydration.HydrateBeforeReliance() {
		t.Fatal("exact structure hid an evidence reliance gap")
	}
	if requiresHydration.RelationalRecords() !=
		RelationalRecordsLegacyUnqualifiedAssertions {
		t.Fatal("evidence uncertainty rewrote the legacy relational-record posture")
	}
}

func TestRetryAndAbstentionCannotImplyTruthAuthorityOrWorkOrder(t *testing.T) {
	contract := interpretationForRetryOrAbstention()
	if !contract.Valid() {
		t.Fatal("retry/abstention contract is invalid")
	}
	if contract.Structure() != StructureUnavailable ||
		contract.RelationalRecords() != RelationalRecordsUnavailable ||
		contract.Completeness() != CompletenessUnavailable ||
		contract.Truth() != TruthNotImplied ||
		contract.Applicability() != ApplicabilityNotImplied ||
		contract.Authority() != AuthorityNotGranted ||
		contract.WorkOrder() != WorkOrderNotImplied ||
		!contract.HydrateBeforeReliance() {
		t.Fatalf("retry/abstention interpretation = %#v", contract)
	}
}

func TestIndependentPosturesRejectCollapsedOrOpenEndedValues(t *testing.T) {
	if _, valid := NewItemPostures(
		SemanticPosture("fresh"),
		LifecycleActive,
		EvidenceCurrent,
		ProjectionCurrent,
	); valid {
		t.Fatal("collapsed freshness was accepted as semantic posture")
	}
	if _, valid := NewItemPostures(
		SemanticTypedActive,
		LifecyclePosture("ready"),
		EvidenceCurrent,
		ProjectionCurrent,
	); valid {
		t.Fatal("open-ended lifecycle posture was accepted")
	}
}

func TestScopedAndUnresolvedCandidateInterpretationsStayDistinct(t *testing.T) {
	scoped := InterpretationForScopedCandidates()
	unresolved := InterpretationForEntityCandidates()
	if !scoped.Valid() || !unresolved.Valid() {
		t.Fatal("candidate interpretation is invalid")
	}
	if scoped.Identity() != IdentityExact ||
		unresolved.Identity() != IdentityUnresolved {
		t.Fatal("exact and unresolved candidate identity collapsed")
	}
	if scoped.RelationalRecords() != RelationalRecordsCandidateAssertions ||
		unresolved.RelationalRecords() != RelationalRecordsUnavailable {
		t.Fatal("identity candidates were confused with relation candidates")
	}
	for _, contract := range []InterpretationContract{scoped, unresolved} {
		if contract.Ranking() != RankingDiscoveryOnly ||
			contract.Authority() != AuthorityNotGranted ||
			contract.WorkOrder() != WorkOrderNotImplied ||
			!contract.HydrateBeforeReliance() {
			t.Fatal("candidate interpretation exceeded discovery authority")
		}
	}
}

func TestExactResolutionAndKnownAbsentDoNotImplyGlobalStructure(t *testing.T) {
	exact := InterpretationForExactEntityResolution()
	absent := InterpretationForKnownAbsent()
	if !exact.Valid() || !absent.Valid() {
		t.Fatal("resolution interpretation is invalid")
	}
	if exact.Identity() != IdentityExact ||
		absent.Identity() != IdentityUnresolved ||
		exact.RelationalRecords() != RelationalRecordsUnavailable ||
		absent.RelationalRecords() != RelationalRecordsUnavailable {
		t.Fatal("resolution identity or relation boundary collapsed")
	}
}

func TestRelationalRecordInterpretationsAreClosedAndUnambiguous(t *testing.T) {
	values := []RelationalRecordsInterpretation{
		RelationalRecordsAssertionsExactAtSnapshot,
		RelationalRecordsOccurrencesExactAtSnapshot,
		RelationalRecordsLegacyUnqualifiedAssertions,
		RelationalRecordsCandidateAssertions,
		RelationalRecordsHeterogeneous,
		RelationalRecordsUnavailable,
	}
	for _, value := range values {
		contract := InterpretationContract{
			structure:             StructureDiscoveryOnly,
			identity:              IdentityExact,
			relationalRecords:     value,
			ranking:               RankingNotApplicable,
			truth:                 TruthNotImplied,
			applicability:         ApplicabilityNotImplied,
			authority:             AuthorityNotGranted,
			workOrder:             WorkOrderNotImplied,
			completeness:          CompletenessCandidateSet,
			hydrateBeforeReliance: true,
		}
		if !contract.Valid() {
			t.Fatalf("closed relational-record posture %q is invalid", value)
		}
	}

	contract := InterpretationContract{
		structure:             StructureDiscoveryOnly,
		identity:              IdentityExact,
		relationalRecords:     RelationalRecordsInterpretation("exact_at_snapshot"),
		ranking:               RankingNotApplicable,
		truth:                 TruthNotImplied,
		applicability:         ApplicabilityNotImplied,
		authority:             AuthorityNotGranted,
		workOrder:             WorkOrderNotImplied,
		completeness:          CompletenessCandidateSet,
		hydrateBeforeReliance: true,
	}
	if contract.Valid() {
		t.Fatal("ambiguous exact_at_snapshot relational-record posture was accepted")
	}
}

func TestRelationalRecordAggregatePreservesItemPostureDifferences(t *testing.T) {
	legacy := legacyUnqualifiedAssertionItemPosture()
	assertion := exactAssertionItemPosture(
		typedmemory.AssertionModalityAffirmsObtaining,
	)
	occurrence := RelationalRecordItemPosture{
		kind: RelationalRecordItemOccurrenceExact,
	}
	candidate := RelationalRecordItemPosture{
		kind: RelationalRecordItemCandidateAssertion,
	}

	tests := []struct {
		name     string
		postures []RelationalRecordItemPosture
		want     RelationalRecordsInterpretation
	}{
		{
			name:     "no emitted relational records",
			postures: nil,
			want:     RelationalRecordsUnavailable,
		},
		{
			name:     "legacy assertions",
			postures: []RelationalRecordItemPosture{legacy, legacy},
			want:     RelationalRecordsLegacyUnqualifiedAssertions,
		},
		{
			name:     "exact assertions",
			postures: []RelationalRecordItemPosture{assertion},
			want:     RelationalRecordsAssertionsExactAtSnapshot,
		},
		{
			name:     "exact occurrences",
			postures: []RelationalRecordItemPosture{occurrence},
			want:     RelationalRecordsOccurrencesExactAtSnapshot,
		},
		{
			name:     "candidate assertions",
			postures: []RelationalRecordItemPosture{candidate},
			want:     RelationalRecordsCandidateAssertions,
		},
		{
			name:     "heterogeneous",
			postures: []RelationalRecordItemPosture{legacy, assertion},
			want:     RelationalRecordsHeterogeneous,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := relationalRecordsInterpretationForItems(test.postures)
			if got != test.want {
				t.Fatalf("aggregate = %q, want %q", got, test.want)
			}
		})
	}
}
