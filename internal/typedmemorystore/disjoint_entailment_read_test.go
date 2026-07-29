package typedmemorystore

import (
	"context"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
)

func TestDecodeActualDisjointEntailmentSemanticRowBindsExactContent(t *testing.T) {
	constraintBytes := []byte("constraint-canonical")
	counterQueryBytes := []byte("counter-query-canonical")
	useBytes := []byte("entailment-use-canonical")
	fillerDigest := mustDigest(t, []byte("filler"))
	constraintDigest := mustDigest(t, constraintBytes)
	counterQueryDigest := mustDigest(t, counterQueryBytes)
	useDigest := mustDigest(t, useBytes)
	fields := []string{
		projectionHexString("0"),
		projectionHexString("assertion:test"),
		projectionHexString("1"),
		projectionHexString("2"),
		projectionHexString(fillerDigest.String()),
		projectionHexString("constraint:test"),
		projectionHexString(constraintDigest.String()),
		projectionHexBytes(constraintBytes),
		projectionHexString("U.System"),
		projectionHexString("U.Episteme"),
		projectionHexString("kind-ref:counter"),
		projectionHexString(counterQueryDigest.String()),
		projectionHexBytes(counterQueryBytes),
		projectionHexString("evaluation:support"),
		projectionHexString(useDigest.String()),
		projectionHexBytes(useBytes),
	}

	got, err := decodeActualDisjointEntailmentSemanticRow(
		strings.Join(fields, ","),
		legacyRelationStorageFamily.disjointnessUseRowKind,
	)
	if err != nil {
		t.Fatalf("decodeActualDisjointEntailmentSemanticRow: %v", err)
	}
	want := newExpectedSemanticRowIdentity(
		"relation_filler_disjoint_entailment_use",
		[]string{
			"0",
			"assertion:test",
			"1",
			"2",
			fillerDigest.String(),
			"constraint:test",
			constraintDigest.String(),
			string(constraintBytes),
			"U.System",
			"U.Episteme",
			"kind-ref:counter",
			counterQueryDigest.String(),
			string(counterQueryBytes),
			"evaluation:support",
		},
		useDigest,
		useBytes,
		false,
	)
	if string(got) != string(want.canonicalBytes) {
		t.Fatalf("semantic row = %q; want %q", got, want.canonicalBytes)
	}
}

func TestDecodeActualDisjointEntailmentSemanticRowRejectsTampering(t *testing.T) {
	constraintBytes := []byte("constraint-canonical")
	counterQueryBytes := []byte("counter-query-canonical")
	useBytes := []byte("entailment-use-canonical")
	fields := []string{
		projectionHexString("0"),
		projectionHexString("assertion:test"),
		projectionHexString("1"),
		projectionHexString("2"),
		projectionHexString(mustDigest(t, []byte("filler")).String()),
		projectionHexString("constraint:test"),
		projectionHexString(mustDigest(t, constraintBytes).String()),
		projectionHexBytes(constraintBytes),
		projectionHexString("U.System"),
		projectionHexString("U.Episteme"),
		projectionHexString("kind-ref:counter"),
		projectionHexString(mustDigest(t, counterQueryBytes).String()),
		projectionHexBytes(counterQueryBytes),
		projectionHexString("evaluation:support"),
		projectionHexString(mustDigest(t, useBytes).String()),
		projectionHexBytes(useBytes),
	}
	cases := []struct {
		name   string
		mutate func([]string) []string
	}{
		{
			name: "constraint digest",
			mutate: func(values []string) []string {
				values[6] = projectionHexString(mustDigest(t, []byte("other")).String())
				return values
			},
		},
		{
			name: "counter query digest",
			mutate: func(values []string) []string {
				values[11] = projectionHexString(mustDigest(t, []byte("other")).String())
				return values
			},
		},
		{
			name: "use digest",
			mutate: func(values []string) []string {
				values[14] = projectionHexString(mustDigest(t, []byte("other")).String())
				return values
			},
		},
		{
			name: "identical operands",
			mutate: func(values []string) []string {
				values[9] = values[8]
				return values
			},
		},
		{
			name: "missing field",
			mutate: func(values []string) []string {
				return values[:len(values)-1]
			},
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			mutated := append([]string(nil), fields...)
			mutated = test.mutate(mutated)
			_, err := decodeActualDisjointEntailmentSemanticRow(
				strings.Join(mutated, ","),
				legacyRelationStorageFamily.disjointnessUseRowKind,
			)
			if !errors.Is(err, ErrStoredAdmissionIntegrity) {
				t.Fatalf("tampered row error = %v; want ErrStoredAdmissionIntegrity", err)
			}
		})
	}
}

func TestDecodeStoredV46DigestRowAcceptsDisjointEntailmentUse(t *testing.T) {
	canonical := []byte("entailment-use-canonical")
	digest := mustDigest(t, canonical)
	rowDigest := "disjoint-entailment-use:" + digest.String()
	encoded := rowDigest + "," + strings.ToUpper(hex.EncodeToString(canonical))

	got, err := decodeStoredV46DigestRow(encoded)
	if err != nil {
		t.Fatalf("decodeStoredV46DigestRow: %v", err)
	}
	if got != rowDigest {
		t.Fatalf("row digest = %q; want %q", got, rowDigest)
	}

	tampered := encoded + projectionHexString("tampered")
	_, err = decodeStoredV46DigestRow(tampered)
	if !errors.Is(err, ErrStoredAdmissionIntegrity) {
		t.Fatalf("tampered row error = %v; want ErrStoredAdmissionIntegrity", err)
	}
}

func TestExpectedDisjointEntailmentRowsUseTheAdmissionContractStorageFamily(
	t *testing.T,
) {
	legacy := newLegacyDisjointEntailmentWriteFixture(t)
	v2 := newDisjointEntailmentWriteFixture(t)

	legacyRows, err := expectedDisjointEntailmentProjectionRows(
		legacy.entailedPrepared,
		AdmissionContractV1(),
	)
	if err != nil {
		t.Fatalf("legacy expected rows: %v", err)
	}
	v2Rows, err := expectedDisjointEntailmentProjectionRows(
		v2.entailedPrepared,
		AdmissionContractV2(),
	)
	if err != nil {
		t.Fatalf("v2 expected rows: %v", err)
	}
	if len(legacyRows) != 1 || len(v2Rows) != 1 {
		t.Fatalf(
			"expected row counts = legacy:%d v2:%d; want one each",
			len(legacyRows),
			len(v2Rows),
		)
	}
	_, err = expectedDisjointEntailmentProjectionRows(
		legacy.entailedPrepared,
		AdmissionContractV2(),
	)
	if !errors.Is(err, ErrInvalidAdmissionBatch) {
		t.Fatalf("legacy manifest under v2 error = %v; want ErrInvalidAdmissionBatch", err)
	}
	_, err = expectedDisjointEntailmentProjectionRows(
		v2.entailedPrepared,
		AdmissionContractV1(),
	)
	if !errors.Is(err, ErrInvalidAdmissionBatch) {
		t.Fatalf("v2 manifest under v1 error = %v; want ErrInvalidAdmissionBatch", err)
	}
}

func TestDisjointEntailmentProjectionReplayKeepsDirectOnlyIdentity(t *testing.T) {
	fixture := newLegacyDisjointEntailmentWriteFixture(t)
	rows, err := expectedDisjointEntailmentProjectionRows(
		fixture.directPrepared,
		AdmissionContractV1(),
	)
	if err != nil {
		t.Fatalf("expectedDisjointEntailmentProjectionRows: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("direct-only expected rows = %d; want 0", len(rows))
	}

	project := mustProjectID(t, "qnt_d1ce7001")
	scanner := fixedDisjointEntailmentScanner{}
	err = verifyStoredV46DisjointEntailmentProjections(
		context.Background(),
		scanner,
		project,
		"event:direct-only",
		fixture.directPrepared,
		AdmissionContractV1(),
	)
	if err != nil {
		t.Fatalf("empty direct-only projection: %v", err)
	}

	scanner = fixedDisjointEntailmentScanner{
		count:  1,
		joined: "unexpected-row",
	}
	err = verifyStoredV46DisjointEntailmentProjections(
		context.Background(),
		scanner,
		project,
		"event:direct-only",
		fixture.directPrepared,
		AdmissionContractV1(),
	)
	if !errors.Is(err, ErrStoredAdmissionIntegrity) {
		t.Fatalf("extra entailment row error = %v; want ErrStoredAdmissionIntegrity", err)
	}
}

func TestDisjointEntailmentProjectionReplayRequiresExactStoredRows(t *testing.T) {
	fixture := newLegacyDisjointEntailmentWriteFixture(t)
	expected, err := expectedDisjointEntailmentProjectionRows(
		fixture.entailedPrepared,
		AdmissionContractV1(),
	)
	if err != nil {
		t.Fatalf("expectedDisjointEntailmentProjectionRows: %v", err)
	}
	if len(expected) != 1 {
		t.Fatalf("expected entailment rows = %d; want 1", len(expected))
	}
	fields := strings.Split(expected[0], ",")
	if len(fields) != 16 {
		t.Fatalf("expected entailment row fields = %d; want 16", len(fields))
	}
	required := fixture.admissionUse.RequiredMembership()
	wantSupportingRef := derivedRef(
		"typed-memory-memberof-evaluation",
		required.Digest().String(),
	)
	wantSupportingField := projectionHexString(wantSupportingRef)
	if fields[13] != wantSupportingField {
		t.Fatalf(
			"supporting evaluation = %q; want required positive evaluation %q",
			fields[13],
			wantSupportingField,
		)
	}

	project := mustProjectID(t, "qnt_d1ce7002")
	cases := []struct {
		name    string
		scanner fixedDisjointEntailmentScanner
		wantErr bool
	}{
		{
			name: "exact",
			scanner: fixedDisjointEntailmentScanner{
				count:  1,
				joined: expected[0],
			},
		},
		{
			name:    "missing",
			scanner: fixedDisjointEntailmentScanner{},
			wantErr: true,
		},
		{
			name: "extra",
			scanner: fixedDisjointEntailmentScanner{
				count:  2,
				joined: expected[0] + "|" + expected[0],
			},
			wantErr: true,
		},
		{
			name: "tampered",
			scanner: fixedDisjointEntailmentScanner{
				count:  1,
				joined: expected[0] + projectionHexString("tampered"),
			},
			wantErr: true,
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			err := verifyStoredV46DisjointEntailmentProjections(
				context.Background(),
				test.scanner,
				project,
				"event:entailed",
				fixture.entailedPrepared,
				AdmissionContractV1(),
			)
			if test.wantErr && !errors.Is(err, ErrStoredAdmissionIntegrity) {
				t.Fatalf("replay error = %v; want ErrStoredAdmissionIntegrity", err)
			}
			if !test.wantErr && err != nil {
				t.Fatalf("exact replay: %v", err)
			}
		})
	}
}

func TestCurrentProjectSnapshotRejectsExactDisjointEntailmentWitnessDrift(
	t *testing.T,
) {
	cases := []struct {
		name   string
		mutate func(*testing.T, disjointEntailmentWriteFixture, CommitReceipt)
	}{
		{
			name: "supporting evaluation reference",
			mutate: func(
				t *testing.T,
				fixture disjointEntailmentWriteFixture,
				receipt CommitReceipt,
			) {
				execDisjointEntailmentWitnessMutation(
					t,
					fixture,
					receipt,
					`UPDATE typed_memory_relational_assertion_disjointness_uses_v3
					SET supporting_evaluation_ref = ?
					WHERE project_id = ? AND event_ref = ?`,
					"evaluation:tampered-support",
				)
			},
		},
		{
			name: "self-consistent constraint carrier replacement",
			mutate: func(
				t *testing.T,
				fixture disjointEntailmentWriteFixture,
				receipt CommitReceipt,
			) {
				canonical := []byte("tampered-constraint-canonical")
				digest := mustDigest(t, canonical)
				execDisjointEntailmentWitnessMutation(
					t,
					fixture,
					receipt,
					`UPDATE typed_memory_relational_assertion_disjointness_uses_v3
					SET constraint_digest = ?, canonical_constraint_bytes = ?
					WHERE project_id = ? AND event_ref = ?`,
					digest.String(),
					canonical,
				)
			},
		},
		{
			name: "swapped disjoint operands",
			mutate: func(
				t *testing.T,
				fixture disjointEntailmentWriteFixture,
				receipt CommitReceipt,
			) {
				execDisjointEntailmentWitnessMutation(
					t,
					fixture,
					receipt,
					`UPDATE typed_memory_relational_assertion_disjointness_uses_v3
					SET matched_operand_kind_id = ?, excluded_operand_kind_id = ?
					WHERE project_id = ? AND event_ref = ?`,
					fixture.entailment.ExcludedOperand().String(),
					fixture.entailment.MatchedOperand().String(),
				)
			},
		},
		{
			name: "self-consistent counter-query carrier replacement",
			mutate: func(
				t *testing.T,
				fixture disjointEntailmentWriteFixture,
				receipt CommitReceipt,
			) {
				canonical := []byte("tampered-counter-query-canonical")
				digest := mustDigest(t, canonical)
				execDisjointEntailmentWitnessMutation(
					t,
					fixture,
					receipt,
					`UPDATE typed_memory_relational_assertion_disjointness_uses_v3
					SET counter_query_digest = ?, canonical_counter_query_bytes = ?
					WHERE project_id = ? AND event_ref = ?`,
					digest.String(),
					canonical,
				)
			},
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			keySuffix := strings.ReplaceAll(test.name, " ", "-")
			key := "snapshot-disjoint-entailment-" + keySuffix
			fixture, adapter, receipt := commitDisjointEntailmentFixture(
				t,
				key,
			)
			test.mutate(t, fixture, receipt)

			_, err := adapter.LoadCurrentProjectSnapshot(
				context.Background(),
				fixture.base.base.project,
			)
			if !errors.Is(err, ErrStoredAdmissionIntegrity) {
				t.Fatalf(
					"snapshot witness drift error = %v; want ErrStoredAdmissionIntegrity",
					err,
				)
			}
		})
	}
}

func execDisjointEntailmentWitnessMutation(
	t *testing.T,
	fixture disjointEntailmentWriteFixture,
	receipt CommitReceipt,
	query string,
	values ...any,
) {
	t.Helper()
	ctx := context.Background()
	database := fixture.base.base.database
	connection, err := database.Conn(ctx)
	if err != nil {
		t.Fatalf("open disjoint-entailment corruption fixture: %v", err)
	}
	closed := false
	defer func() {
		if closed {
			return
		}
		_, _ = connection.ExecContext(ctx, "PRAGMA foreign_keys = ON")
		_ = connection.Close()
	}()
	if _, err := connection.ExecContext(
		ctx,
		"PRAGMA foreign_keys = OFF",
	); err != nil {
		t.Fatalf("disable fixture foreign keys: %v", err)
	}
	if _, err := connection.ExecContext(
		ctx,
		"DROP TRIGGER typed_memory_relational_assertion_disjointness_uses_v3_v53_no_update",
	); err != nil {
		t.Fatalf("drop disjoint-entailment update guard: %v", err)
	}
	arguments := append([]any(nil), values...)
	project := fixture.base.base.project.String()
	eventRef := receipt.EventRef()
	arguments = append(
		arguments,
		project,
		eventRef,
	)
	result, err := connection.ExecContext(ctx, query, arguments...)
	if err != nil {
		t.Fatalf("inject disjoint-entailment witness drift: %v", err)
	}
	assertExactBasisRowsAffected(t, result, 1, "disjoint-entailment witness")
	if _, err := connection.ExecContext(
		ctx,
		"PRAGMA foreign_keys = ON",
	); err != nil {
		t.Fatalf("restore fixture foreign keys: %v", err)
	}
	if err := connection.Close(); err != nil {
		t.Fatalf("close disjoint-entailment corruption fixture: %v", err)
	}
	closed = true
}

type fixedDisjointEntailmentScanner struct {
	count  int64
	joined string
}

func (source fixedDisjointEntailmentScanner) ScanOne(
	_ context.Context,
	_ string,
	_ []any,
	destinations []any,
) error {
	*destinations[0].(*int64) = source.count
	*destinations[1].(*string) = source.joined
	return nil
}

var _ scanner = fixedDisjointEntailmentScanner{}
