package legacyimport

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestImportPlanPreservesOpaqueHistoryDeterministically(t *testing.T) {
	firstCarrier := testCarrier(t, "opaque-a")
	secondCarrier := testCarrier(t, "opaque-b")
	firstSubject := mustSubject(t, "legacy-subject:opaque-a")
	secondSubject := mustSubject(t, "legacy-subject:opaque-b")
	firstObservation := testCarrierObservation(t, firstSubject, firstCarrier)
	secondObservation := mustAssociationObservation(
		t,
		secondSubject,
		secondCarrier,
		"legacy:source",
		"legacy:target",
		"dependsOnProjected",
	)
	firstClassification, err := NewCarrierOnly(
		firstSubject,
		[]CarrierObservation{firstObservation},
	)
	if err != nil {
		t.Fatalf("NewCarrierOnly() error = %v", err)
	}
	secondClassification, err := NewLegacyUnbound(
		secondSubject,
		[]AssociationObservation{secondObservation},
	)
	if err != nil {
		t.Fatalf("NewLegacyUnbound() error = %v", err)
	}
	firstReport := mustDryRunReport(
		t,
		[]CarrierSnapshot{firstCarrier, secondCarrier},
		[]SubjectObservation{firstObservation, secondObservation},
		[]SubjectClassification{firstClassification, secondClassification},
	)
	secondReport := mustDryRunReport(
		t,
		[]CarrierSnapshot{secondCarrier, firstCarrier},
		[]SubjectObservation{secondObservation, firstObservation},
		[]SubjectClassification{secondClassification, firstClassification},
	)

	firstPlan, err := NewImportPlan(firstReport)
	if err != nil {
		t.Fatalf("NewImportPlan(first) error = %v", err)
	}
	secondPlan, err := NewImportPlan(secondReport)
	if err != nil {
		t.Fatalf("NewImportPlan(second) error = %v", err)
	}
	if err := firstPlan.Verify(); err != nil {
		t.Fatalf("Verify(first) error = %v", err)
	}
	if err := (ImportPlan{}).Verify(); err == nil {
		t.Fatal("Verify() accepted a zero import plan")
	}

	if !bytes.Equal(firstPlan.CanonicalBytes(), secondPlan.CanonicalBytes()) {
		t.Fatalf(
			"import plan changed under source permutation:\n%s\n%s",
			firstPlan.CanonicalBytes(),
			secondPlan.CanonicalBytes(),
		)
	}
	if firstPlan.Digest() != secondPlan.Digest() {
		t.Fatalf(
			"import plan digest changed under permutation: %s != %s",
			firstPlan.Digest().String(),
			secondPlan.Digest().String(),
		)
	}
	if firstPlan.Posture() != ImportPlanPosture {
		t.Fatalf("Posture() = %q, want %q", firstPlan.Posture(), ImportPlanPosture)
	}
	if firstPlan.DryRunReportDigest() != firstReport.Digest() {
		t.Fatal("plan lost exact dry-run report digest")
	}
	if firstPlan.SourceSnapshotDigest() != firstReport.SourceSnapshotDigest() {
		t.Fatal("plan lost exact source snapshot digest")
	}

	histories := firstPlan.CarrierHistories()
	if len(histories) != 2 {
		t.Fatalf("CarrierHistories() count = %d, want 2", len(histories))
	}
	historyBytes := [][]byte{histories[0].ExactBytes(), histories[1].ExactBytes()}
	if !containsExactBytes(historyBytes, firstCarrier.ExactBytes()) {
		t.Fatal("plan lost first exact legacy carrier bytes")
	}
	if !containsExactBytes(historyBytes, secondCarrier.ExactBytes()) {
		t.Fatal("plan lost second exact legacy carrier bytes")
	}

	dispositions := firstPlan.SubjectDispositions()
	if len(dispositions) != 2 {
		t.Fatalf("SubjectDispositions() count = %d, want 2", len(dispositions))
	}
	observedKinds := map[ClassificationKind]bool{}
	for _, disposition := range dispositions {
		observedKinds[disposition.Kind()] = true
	}
	if !observedKinds[ClassificationCarrierOnly] {
		t.Fatal("plan lost carrier_only posture")
	}
	if !observedKinds[ClassificationLegacyUnbound] {
		t.Fatal("plan lost legacy_unbound posture")
	}
}

func TestImportPlanDoesNotFabricateTypedSemanticsOrAuthority(t *testing.T) {
	carrier := testCarrier(t, "no-fabrication")
	subject := mustSubject(t, "legacy-subject:no-fabrication")
	observation := mustAssociationObservation(
		t,
		subject,
		carrier,
		"legacy:source",
		"legacy:target",
		"approvedBy",
	)
	reason, err := NewUnresolvedReason("association label is not semantic evidence")
	if err != nil {
		t.Fatalf("NewUnresolvedReason() error = %v", err)
	}
	classification, err := NewUnresolved(
		subject,
		reason,
		[]SubjectObservation{observation},
	)
	if err != nil {
		t.Fatalf("NewUnresolved() error = %v", err)
	}
	report := mustDryRunReport(
		t,
		[]CarrierSnapshot{carrier},
		[]SubjectObservation{observation},
		[]SubjectClassification{classification},
	)
	plan, err := NewImportPlan(report)
	if err != nil {
		t.Fatalf("NewImportPlan() error = %v", err)
	}

	var body map[string]any
	if err := json.Unmarshal(plan.CanonicalBytes(), &body); err != nil {
		t.Fatalf("decode import plan: %v", err)
	}
	forbiddenKeys := []string{
		"entity_id",
		"bounded_context",
		"kind_id",
		"member_of",
		"scope_ref",
		"authority_receipt",
		"permission_ref",
		"work_ref",
	}
	assertJSONKeysAbsent(t, body, forbiddenKeys)

	disposition := plan.SubjectDispositions()[0]
	if disposition.Kind() != ClassificationUnresolved {
		t.Fatalf("Kind() = %q, want unresolved", disposition.Kind())
	}
	observedReason, exists := disposition.UnresolvedReason()
	if !exists {
		t.Fatal("unresolved plan item lost its explicit basis reason")
	}
	if observedReason != reason {
		t.Fatalf(
			"UnresolvedReason() = %q, want %q",
			observedReason.String(),
			reason.String(),
		)
	}
	if disposition.Subject().String() != subject.String() {
		t.Fatal("opaque subject was silently remapped to another identity")
	}
}

func TestImportPlanDefensivelyOwnsCarrierAndReportBytes(t *testing.T) {
	carrier := testCarrier(t, "owned-plan")
	subject := mustSubject(t, "legacy-subject:owned-plan")
	observation := testCarrierObservation(t, subject, carrier)
	classification, err := NewCarrierOnly(
		subject,
		[]CarrierObservation{observation},
	)
	if err != nil {
		t.Fatalf("NewCarrierOnly() error = %v", err)
	}
	report := mustDryRunReport(
		t,
		[]CarrierSnapshot{carrier},
		[]SubjectObservation{observation},
		[]SubjectClassification{classification},
	)
	plan, err := NewImportPlan(report)
	if err != nil {
		t.Fatalf("NewImportPlan() error = %v", err)
	}
	expectedDigest := plan.Digest()
	carrierBytes := plan.CarrierHistories()[0].ExactBytes()
	reportBytes := plan.DryRunReportCanonicalBytes()
	canonical := plan.CanonicalBytes()
	carrierBytes[0] ^= 0xff
	reportBytes[0] ^= 0xff
	canonical[0] ^= 0xff

	if plan.Digest() != expectedDigest {
		t.Fatal("plan digest changed through returned byte slices")
	}
	if !bytes.Equal(
		plan.CarrierHistories()[0].ExactBytes(),
		carrier.ExactBytes(),
	) {
		t.Fatal("carrier history changed through returned byte slice")
	}
	if !bytes.Equal(plan.DryRunReportCanonicalBytes(), report.CanonicalBytes()) {
		t.Fatal("dry-run report changed through returned byte slice")
	}
}

func containsExactBytes(values [][]byte, expected []byte) bool {
	for _, value := range values {
		if bytes.Equal(value, expected) {
			return true
		}
	}
	return false
}

func assertJSONKeysAbsent(
	t *testing.T,
	value any,
	forbidden []string,
) {
	t.Helper()
	switch current := value.(type) {
	case map[string]any:
		for key, nested := range current {
			for _, forbiddenKey := range forbidden {
				if strings.EqualFold(key, forbiddenKey) {
					t.Fatalf("legacy import plan fabricates forbidden key %q", key)
				}
			}
			assertJSONKeysAbsent(t, nested, forbidden)
		}
	case []any:
		for _, nested := range current {
			assertJSONKeysAbsent(t, nested, forbidden)
		}
	}
}
