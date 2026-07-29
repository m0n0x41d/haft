package typedmemorywire

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestDecodeAssertRelationBuildsExplicitSealedModality(t *testing.T) {
	testCases := []struct {
		wireKind string
		want     typedmemory.AssertionModalityKind
	}{
		{
			wireKind: "affirms_obtaining",
			want:     typedmemory.AssertionModalityAffirmsObtaining,
		},
		{
			wireKind: "denies_obtaining",
			want:     typedmemory.AssertionModalityDeniesObtaining,
		},
		{
			wireKind: "obtaining_unknown",
			want:     typedmemory.AssertionModalityObtainingUnknown,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.wireKind, func(t *testing.T) {
			request, err := DecodeValidateRequest(
				validAssertRelationPayload(testCase.wireKind),
			)
			if err != nil {
				t.Fatalf("DecodeValidateRequest(): %v", err)
			}
			changeSet, err := request.BindChangeSet(testTypeEnvRef(t))
			if err != nil {
				t.Fatalf("BindChangeSet(): %v", err)
			}
			changes := changeSet.Changes()
			change, ok := changes[0].(typedmemory.AssertRelation)
			if !ok {
				t.Fatalf("decoded change = %T; want AssertRelation", changes[0])
			}
			assertion := change.Assertion()
			if assertion.Modality().Kind() != testCase.want {
				t.Fatalf(
					"modality = %q; want %q",
					assertion.Modality().Kind(),
					testCase.want,
				)
			}
			if positive, ok := assertion.Modality().(typedmemory.AffirmsObtaining); ok &&
				positive.HasOccurrenceDesignation() {
				t.Fatal("wire positive assertion inferred an occurrence designation")
			}

			coordinates := request.DiagnosticCoordinates()
			kind, found := coordinates.ChangeKind(0)
			if !found || kind != DiagnosticChangeAssertRelation {
				t.Fatalf("diagnostic change kind = %d, %v; want AssertRelation", kind, found)
			}
			bindings := assertion.Bindings()
			ordinal, found := coordinates.BindingOrdinal(0, bindings[0].Name())
			if !found || ordinal != 0 {
				t.Fatalf("binding ordinal = %d, %v; want 0", ordinal, found)
			}
		})
	}
}

func TestDecodeAssertRelationRejectsMissingUnknownOrOccurrenceBearingModality(
	t *testing.T,
) {
	valid := validAssertRelationPayload("affirms_obtaining")
	testCases := []struct {
		name    string
		payload []byte
	}{
		{
			name: "missing modality",
			payload: bytes.Replace(
				valid,
				[]byte(`"modality":{"kind":"affirms_obtaining"},`),
				nil,
				1,
			),
		},
		{
			name: "unknown modality",
			payload: bytes.Replace(
				valid,
				[]byte(`"affirms_obtaining"`),
				[]byte(`"probably_obtains"`),
				1,
			),
		},
		{
			name: "positive occurrence designation is not yet representable",
			payload: bytes.Replace(
				valid,
				[]byte(`"modality":{"kind":"affirms_obtaining"}`),
				[]byte(`"modality":{"kind":"affirms_obtaining","occurrence_designation":{"kind":"none"}}`),
				1,
			),
		},
		{
			name: "negative occurrence designation",
			payload: bytes.Replace(
				valid,
				[]byte(`"modality":{"kind":"affirms_obtaining"}`),
				[]byte(`"modality":{"kind":"denies_obtaining","occurrence_designation":{"kind":"none"}}`),
				1,
			),
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := DecodeValidateRequest(testCase.payload)
			assertDecodeErrorCode(t, err, ErrorInvalidContract)
		})
	}
}

func TestDecodeAdmitRequestPreservesAssertRelationV2(t *testing.T) {
	payload := []byte(fmt.Sprintf(`{
  "contract_version":%q,
  "action":"admit",
  "basis":{"kind":"exact_project","type_env_digest":%q,"graph_revision":17},
  "authority_class":"non_binding_semantic_assertion",
  "idempotency_key":"assert-relation-v3-validate-only",
  "request_provenance_ref":"test:assert-relation-v3",
  "change_set":{"changes":[%s]}
}`,
		ContractVersionV2,
		testDigest,
		assertRelationChangeJSON("affirms_obtaining"),
	))
	request, err := DecodeAdmitRequest(payload)
	if err != nil {
		t.Fatalf("DecodeAdmitRequest(v2 assertion): %v", err)
	}
	if request.ContractVersion() != ContractVersionV2 {
		t.Fatalf("admit version = %q; want v2", request.ContractVersion())
	}
	if request.ValidationRequest().ContractVersion() != ContractVersionV2 {
		t.Fatalf(
			"derived validation version = %q; want v2",
			request.ValidationRequest().ContractVersion(),
		)
	}
	changeSet, err := request.ValidationRequest().BindChangeSet(testTypeEnvRef(t))
	if err != nil {
		t.Fatalf("BindChangeSet(v2 assertion): %v", err)
	}
	if _, ok := changeSet.Changes()[0].(typedmemory.AssertRelation); !ok {
		t.Fatalf("admit change = %T; want AssertRelation", changeSet.Changes()[0])
	}
}

func TestRelationWriteKindsAreDisjointAcrossWireContractVersions(t *testing.T) {
	v1Assert := bytes.Replace(
		validAssertRelationPayload("affirms_obtaining"),
		[]byte(ContractVersionV2),
		[]byte(ContractVersionV1),
		1,
	)
	_, err := DecodeValidateRequest(v1Assert)
	assertDecodeErrorCode(t, err, ErrorInvalidContract)

	v2Instantiation := bytes.Replace(
		validRelationPayload("Local.Relation"),
		[]byte(ContractVersionV1),
		[]byte(ContractVersionV2),
		1,
	)
	_, err = DecodeValidateRequest(v2Instantiation)
	assertDecodeErrorCode(t, err, ErrorInvalidContract)

	v1Request, err := DecodeValidateRequest(validRelationPayload("Local.Relation"))
	if err != nil {
		t.Fatalf("DecodeValidateRequest(v1 legacy): %v", err)
	}
	if v1Request.ContractVersion() != ContractVersionV1 {
		t.Fatalf("legacy request version = %q; want v1", v1Request.ContractVersion())
	}
	v2Request, err := DecodeValidateRequest(
		validAssertRelationPayload("affirms_obtaining"),
	)
	if err != nil {
		t.Fatalf("DecodeValidateRequest(v2 assertion): %v", err)
	}
	if v2Request.ContractVersion() != ContractVersionV2 {
		t.Fatalf("assertion request version = %q; want v2", v2Request.ContractVersion())
	}
}

func TestDecodeAdmitRequestPreservesExistingV1RelationWrite(t *testing.T) {
	payload := []byte(fmt.Sprintf(`{
  "contract_version":%q,
  "action":"admit",
  "basis":{"kind":"exact_project","type_env_digest":%q,"graph_revision":17},
  "authority_class":"non_binding_semantic_assertion",
		"idempotency_key":"legacy-relation-write-preserved",
  "request_provenance_ref":"test:legacy-relation-write",
  "change_set":{"changes":[%s]}
}`,
		ContractVersionV1,
		testDigest,
		string(extractOnlyChange(t, validRelationPayload("Local.Relation"))),
	))
	request, err := DecodeAdmitRequest(payload)
	if err != nil {
		t.Fatalf("DecodeAdmitRequest(v1 legacy): %v", err)
	}
	if request.ContractVersion() != ContractVersionV1 {
		t.Fatalf("legacy admit version = %q; want v1", request.ContractVersion())
	}
	if request.ValidationRequest().ContractVersion() != ContractVersionV1 {
		t.Fatalf(
			"legacy validation version = %q; want v1",
			request.ValidationRequest().ContractVersion(),
		)
	}
}

func validAssertRelationPayload(modality string) []byte {
	return []byte(fmt.Sprintf(`{
  "contract_version":%q,
  "action":"validate",
  "basis":{"kind":"project_current"},
  "change_set":{"changes":[%s]}
}`,
		ContractVersionV2,
		assertRelationChangeJSON(modality),
	))
}

func assertRelationChangeJSON(modality string) string {
	return fmt.Sprintf(`{
  "kind":"assert_relation",
  "assertion_id":"assertion-v3",
  "signature_id":"Local.Relation",
  "context_slice":%s,
  "modality":{"kind":%q},
  "bindings":[{
    "slot_kind":"Local.EntitySlot",
    "fillers":[{
      "kind":"by_reference",
      "reference":{"kind":"persisted","ref_kind":"U.EntityRef","id":"entity-1"}
    }]
  }],
  "provenance":"test:assert-relation-v3"
}`,
		testContextSliceJSON("project"),
		modality,
	)
}

func extractOnlyChange(t *testing.T, payload []byte) []byte {
	t.Helper()
	prefix := []byte(`"changes":[`)
	start := bytes.Index(payload, prefix)
	if start < 0 {
		t.Fatal("fixture has no changes array")
	}
	start += len(prefix)
	end := bytes.LastIndex(payload, []byte(`]}`))
	if end < start {
		t.Fatal("fixture has no closing changes array")
	}
	return append([]byte(nil), payload[start:end]...)
}
