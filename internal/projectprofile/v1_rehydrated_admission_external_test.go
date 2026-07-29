package projectprofile_test

import (
	"bytes"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectprofile"
)

type foreignDurableProfileAdmissionTupleV1 struct {
	projectprofile.DurableProfileAdmissionTupleV1
}

type foreignRehydratedProfileAdmissionV1 struct {
	projectprofile.RehydratedProfileAdmissionV1
}

func TestRehydrateProfileAdmissionV1ReturnsSealedCommittedSemantics(t *testing.T) {
	fixture := mustV1Fixture(t)
	prepared := mustPreparedProfileAdmissionV1(t, fixture)
	tentative := mustTentativeProfileAdmissionV1(t, fixture, prepared)
	durable := mustDurableProfileAdmissionTupleV1(t, prepared, tentative)

	rehydrated, err := projectprofile.RehydrateProfileAdmissionV1(
		prepared,
		tentative,
		durable,
	)
	if err != nil {
		t.Fatalf("RehydrateProfileAdmissionV1: %v", err)
	}
	if err := projectprofile.ValidateRehydratedProfileAdmissionV1(rehydrated); err != nil {
		t.Fatalf("ValidateRehydratedProfileAdmissionV1: %v", err)
	}
	record := rehydrated.AdmissionRecord()
	if rehydrated.AdmissionRecordDigest() != tentative.TentativeAdmissionRecordDigest() {
		t.Fatal("rehydrated admission-record digest differs from tentative material")
	}
	if rehydrated.LedgerRevision() != record.CommittedLedgerRevision() {
		t.Fatal("rehydrated ledger revision differs from admission record")
	}
	if rehydrated.Receipt().LedgerRevision() != rehydrated.LedgerRevision() {
		t.Fatal("rehydrated receipt differs from committed ledger revision")
	}
	if rehydrated.DeclaredProfile().Receipt().LedgerRevision() != rehydrated.LedgerRevision() {
		t.Fatal("Declared profile does not carry the rehydrated receipt")
	}
	payloadDigest, err := projectprofile.DigestProfileDeclarationPayload(
		rehydrated.DeclaredProfile().Payload(),
	)
	if err != nil {
		t.Fatalf("DigestProfileDeclarationPayload: %v", err)
	}
	if payloadDigest != prepared.ProfilePayloadDigest() {
		t.Fatal("Declared profile does not carry the exact prepared payload")
	}
	rawDurable := any(durable)
	if _, ok := rawDurable.(projectprofile.RehydratedProfileAdmissionV1); ok {
		t.Fatal("raw durable tuple implements RehydratedProfileAdmissionV1")
	}
	mutated := durable.AdmissionRecordCanonicalJSON()
	mutated[0] = '['
	if bytes.Equal(mutated, durable.AdmissionRecordCanonicalJSON()) {
		t.Fatal("durable tuple leaked mutable admission bytes")
	}
}

func TestValidateDurableProfileAdmissionRecordV1ChecksCanonicalProvenance(t *testing.T) {
	fixture := mustV1Fixture(t)
	prepared := mustPreparedProfileAdmissionV1(t, fixture)
	tentative := mustTentativeProfileAdmissionV1(t, fixture, prepared)
	durable := mustDurableProfileAdmissionTupleV1(t, prepared, tentative)
	err := projectprofile.ValidateDurableProfileAdmissionRecordV1(
		durable,
		prepared.CandidateProvenanceCanonicalJSON(),
		prepared.CandidateProvenanceDigest(),
	)
	if err != nil {
		t.Fatalf("ValidateDurableProfileAdmissionRecordV1: %v", err)
	}
	corrupted := prepared.CandidateProvenanceCanonicalJSON()
	corrupted[0] = '['
	err = projectprofile.ValidateDurableProfileAdmissionRecordV1(
		durable,
		corrupted,
		prepared.CandidateProvenanceDigest(),
	)
	if err == nil {
		t.Fatal("validator accepted corrupted candidate-provenance JSON")
	}
}

func TestRehydrateProfileAdmissionV1RejectsEveryDurableTupleMismatch(t *testing.T) {
	fixture := mustV1Fixture(t)
	prepared := mustPreparedProfileAdmissionV1(t, fixture)
	tentative := mustTentativeProfileAdmissionV1(t, fixture, prepared)
	base := durableTupleFieldsV1{
		admissionJSON:   tentative.TentativeAdmissionRecordCanonicalJSON(),
		admissionDigest: tentative.TentativeAdmissionRecordDigest(),
		receiptJSON:     tentative.TentativeReceiptCanonicalJSON(),
		receiptDigest:   tentative.TentativeReceiptDigest(),
		payloadJSON:     prepared.ProfilePayloadCanonicalJSON(),
		payloadDigest:   prepared.ProfilePayloadDigest(),
		ledgerRevision:  mustNextLedgerRevisionV1(t, prepared.ExpectedLedgerRevision()),
	}
	tests := []struct {
		name   string
		mutate func(durableTupleFieldsV1) durableTupleFieldsV1
	}{
		{name: "admission JSON", mutate: func(value durableTupleFieldsV1) durableTupleFieldsV1 {
			value.admissionJSON = append([]byte(" "), value.admissionJSON...)
			return value
		}},
		{name: "admission digest", mutate: func(value durableTupleFieldsV1) durableTupleFieldsV1 {
			value.admissionDigest = digestOfTB(t, "different-admission")
			return value
		}},
		{name: "receipt JSON", mutate: func(value durableTupleFieldsV1) durableTupleFieldsV1 {
			value.receiptJSON = append([]byte(" "), value.receiptJSON...)
			return value
		}},
		{name: "receipt digest", mutate: func(value durableTupleFieldsV1) durableTupleFieldsV1 {
			value.receiptDigest = digestOfTB(t, "different-receipt")
			return value
		}},
		{name: "payload JSON", mutate: func(value durableTupleFieldsV1) durableTupleFieldsV1 {
			value.payloadJSON = append([]byte(" "), value.payloadJSON...)
			return value
		}},
		{name: "payload digest", mutate: func(value durableTupleFieldsV1) durableTupleFieldsV1 {
			value.payloadDigest = digestOfTB(t, "different-payload")
			return value
		}},
		{name: "ledger revision", mutate: func(value durableTupleFieldsV1) durableTupleFieldsV1 {
			value.ledgerRevision = projectprofile.NewLedgerRevision(value.ledgerRevision.Value() + 1)
			return value
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			durable := buildDurableProfileAdmissionTupleV1(t, test.mutate(base))
			_, err := projectprofile.RehydrateProfileAdmissionV1(prepared, tentative, durable)
			if err == nil {
				t.Fatalf("rehydration accepted mismatched %s", test.name)
			}
		})
	}
}

func TestRehydrateProfileAdmissionV1RejectsForeignAndMismatchedSealedInputs(t *testing.T) {
	fixture := mustV1Fixture(t)
	prepared := mustPreparedProfileAdmissionV1(t, fixture)
	tentative := mustTentativeProfileAdmissionV1(t, fixture, prepared)
	durable := mustDurableProfileAdmissionTupleV1(t, prepared, tentative)
	forgedPrepared := foreignPreparedProfileAdmissionV1{
		PreparedProfileAdmissionV1: prepared,
	}
	if _, err := projectprofile.RehydrateProfileAdmissionV1(forgedPrepared, tentative, durable); err == nil {
		t.Fatal("rehydration accepted foreign Prepared input")
	}
	forgedTentative := foreignTentativeProfileAdmissionV1{
		TentativeProfileAdmissionTransactionMaterialV1: tentative,
	}
	if _, err := projectprofile.RehydrateProfileAdmissionV1(prepared, forgedTentative, durable); err == nil {
		t.Fatal("rehydration accepted foreign Tentative input")
	}
	forgedDurable := foreignDurableProfileAdmissionTupleV1{
		DurableProfileAdmissionTupleV1: durable,
	}
	if _, err := projectprofile.RehydrateProfileAdmissionV1(prepared, tentative, forgedDurable); err == nil {
		t.Fatal("rehydration accepted foreign durable tuple")
	}
	if _, err := projectprofile.RehydrateProfileAdmissionV1(nil, tentative, durable); err == nil {
		t.Fatal("rehydration accepted nil Prepared input")
	}
	if _, err := projectprofile.RehydrateProfileAdmissionV1(prepared, nil, durable); err == nil {
		t.Fatal("rehydration accepted nil Tentative input")
	}
	if _, err := projectprofile.RehydrateProfileAdmissionV1(prepared, tentative, nil); err == nil {
		t.Fatal("rehydration accepted nil durable tuple")
	}
	rehydrated, err := projectprofile.RehydrateProfileAdmissionV1(prepared, tentative, durable)
	if err != nil {
		t.Fatalf("RehydrateProfileAdmissionV1(valid): %v", err)
	}
	foreignRehydrated := foreignRehydratedProfileAdmissionV1{
		RehydratedProfileAdmissionV1: rehydrated,
	}
	if err := projectprofile.ValidateRehydratedProfileAdmissionV1(foreignRehydrated); err == nil {
		t.Fatal("rehydrated validator accepted foreign embedded implementation")
	}
	if err := projectprofile.ValidateRehydratedProfileAdmissionV1(nil); err == nil {
		t.Fatal("rehydrated validator accepted nil")
	}

	nextExpected := mustNextLedgerRevisionV1(t, prepared.ExpectedLedgerRevision())
	nextInputs, err := projectprofile.NewProfileDeclarationAdmissionInputs(fixture.candidate, nextExpected)
	if err != nil {
		t.Fatalf("NewProfileDeclarationAdmissionInputs(next): %v", err)
	}
	nextPlan, err := projectprofile.NewProfileDeclarationCommitPlan(
		nextInputs,
		fixture.commitPlan.AuthorityResolutionRecordRef(),
		fixture.commitPlan.AuthorityResolutionRecordDigest(),
		fixture.commitPlan.SingleUseKey(),
	)
	if err != nil {
		t.Fatalf("NewProfileDeclarationCommitPlan(next): %v", err)
	}
	provenance := fixture.candidate.Provenance()
	projectRoot := provenance.ProjectRoot()
	builder := profileAdmissionPreparationBuilderV1(fixture, nextPlan, fixture.assessment, projectRoot)
	nextPrepared, err := builder.Build()
	if err != nil {
		t.Fatalf("ProfileAdmissionPreparationV1Builder.Build(next): %v", err)
	}
	if nextPrepared.AdmissionRequestDigest() != prepared.AdmissionRequestDigest() {
		t.Fatal("test setup changed stable admission intent")
	}
	if _, err := projectprofile.RehydrateProfileAdmissionV1(nextPrepared, tentative, durable); err == nil {
		t.Fatal("rehydration accepted Tentative material from another CAS-bound Prepared value")
	}
}

type durableTupleFieldsV1 struct {
	admissionJSON   []byte
	admissionDigest projectprofile.ContentDigest
	receiptJSON     []byte
	receiptDigest   projectprofile.ContentDigest
	payloadJSON     []byte
	payloadDigest   projectprofile.ContentDigest
	ledgerRevision  projectprofile.LedgerRevision
}

func mustDurableProfileAdmissionTupleV1(
	t testing.TB,
	prepared projectprofile.PreparedProfileAdmissionV1,
	tentative projectprofile.TentativeProfileAdmissionTransactionMaterialV1,
) projectprofile.DurableProfileAdmissionTupleV1 {
	t.Helper()
	return buildDurableProfileAdmissionTupleV1(t, durableTupleFieldsV1{
		admissionJSON:   tentative.TentativeAdmissionRecordCanonicalJSON(),
		admissionDigest: tentative.TentativeAdmissionRecordDigest(),
		receiptJSON:     tentative.TentativeReceiptCanonicalJSON(),
		receiptDigest:   tentative.TentativeReceiptDigest(),
		payloadJSON:     prepared.ProfilePayloadCanonicalJSON(),
		payloadDigest:   prepared.ProfilePayloadDigest(),
		ledgerRevision:  mustNextLedgerRevisionV1(t, prepared.ExpectedLedgerRevision()),
	})
}

func buildDurableProfileAdmissionTupleV1(
	t testing.TB,
	fields durableTupleFieldsV1,
) projectprofile.DurableProfileAdmissionTupleV1 {
	t.Helper()
	builder := projectprofile.NewDurableProfileAdmissionTupleV1Builder(
		fields.admissionJSON,
		fields.admissionDigest,
	)
	builder = builder.WithReceipt(fields.receiptJSON, fields.receiptDigest)
	builder = builder.WithPayload(fields.payloadJSON, fields.payloadDigest)
	builder = builder.AtLedgerRevision(fields.ledgerRevision)
	value, err := builder.Build()
	if err != nil {
		t.Fatalf("DurableProfileAdmissionTupleV1Builder.Build: %v", err)
	}
	return value
}
