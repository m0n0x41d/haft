package typedmemorywire

import (
	"bytes"
	"errors"
	"testing"
)

func TestDecodeRequestKeepsValidateAndAdmitAsSealedVariants(t *testing.T) {
	admitPayload := admissionRequestFixture()
	decoded, err := DecodeRequest(admitPayload)
	if err != nil {
		t.Fatalf("DecodeRequest(admit) error = %v", err)
	}
	admit, ok := decoded.(AdmitRequest)
	if !ok {
		t.Fatalf("DecodeRequest(admit) = %T, want AdmitRequest", decoded)
	}
	if !IsDecodedAdmitRequest(admit) {
		t.Fatal("decoded admission request is not sealed")
	}
	if admit.Action() != ActionAdmit ||
		admit.AuthorityClass() != AuthorityClassNonBindingSemanticAssertion ||
		admit.IdempotencyKey() != "typed-memory-admit-fixture" ||
		admit.RequestProvenance().String() != "provenance:typed-memory-admit-fixture" {
		t.Fatalf("decoded admission coordinates = %#v", admit)
	}
	selector := admit.Basis()
	if selector.RequestedGraphRevision().Value() != 17 ||
		selector.RequestedTypeEnvDigest().String() !=
			"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("decoded exact basis = %#v", selector)
	}
	validation := admit.ValidationRequest()
	if !IsDecodedValidateRequest(validation) ||
		validation.Action() != ActionValidate ||
		validation.ChangeCount() != 1 {
		t.Fatalf("derived validation request = %#v", validation)
	}

	validatePayload := bytes.Replace(
		admitPayload,
		[]byte(`"action":"admit",`+
			`"basis":{"kind":"exact_project","type_env_digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","graph_revision":17},`+
			`"authority_class":"non_binding_semantic_assertion",`+
			`"idempotency_key":"typed-memory-admit-fixture",`+
			`"request_provenance_ref":"provenance:typed-memory-admit-fixture",`),
		[]byte(`"action":"validate","basis":{"kind":"project_current"},`),
		1,
	)
	decoded, err = DecodeRequest(validatePayload)
	if err != nil {
		t.Fatalf("DecodeRequest(validate) error = %v", err)
	}
	if _, ok := decoded.(ValidateRequest); !ok {
		t.Fatalf("DecodeRequest(validate) = %T, want ValidateRequest", decoded)
	}
}

func TestDecodeAdmitRequestRejectsAuthorityOrBasisBroadening(t *testing.T) {
	cases := []struct {
		name string
		from string
		to   string
		path string
	}{
		{
			name: "current project basis",
			from: `"kind":"exact_project","type_env_digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","graph_revision":17`,
			to:   `"kind":"project_current"`,
			path: "$.basis.kind",
		},
		{
			name: "binding authority",
			from: `"authority_class":"non_binding_semantic_assertion"`,
			to:   `"authority_class":"decision_binding"`,
			path: "$.authority_class",
		},
		{
			name: "non canonical idempotency key",
			from: `"idempotency_key":"typed-memory-admit-fixture"`,
			to:   `"idempotency_key":" typed-memory-admit-fixture"`,
			path: "$.idempotency_key",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			payload := bytes.Replace(
				admissionRequestFixture(),
				[]byte(testCase.from),
				[]byte(testCase.to),
				1,
			)
			_, err := DecodeAdmitRequest(payload)
			var decoded *DecodeError
			if !errors.As(err, &decoded) {
				t.Fatalf("error = %T %v, want DecodeError", err, err)
			}
			if decoded.Path() != testCase.path {
				t.Fatalf("error path = %q, want %q", decoded.Path(), testCase.path)
			}
		})
	}
}

func TestDecodeAdmitRequestRejectsUnknownOrValidateOnlyShapes(t *testing.T) {
	unknown := bytes.Replace(
		admissionRequestFixture(),
		[]byte(`"change_set":`),
		[]byte(`"operator_confirmed":true,"change_set":`),
		1,
	)
	if _, err := DecodeAdmitRequest(unknown); err == nil {
		t.Fatal("unknown admission authority field was accepted")
	}

	validateWithAdmissionFields := bytes.Replace(
		admissionRequestFixture(),
		[]byte(`"action":"admit"`),
		[]byte(`"action":"validate"`),
		1,
	)
	if _, err := DecodeValidateRequest(validateWithAdmissionFields); err == nil {
		t.Fatal("validate request accepted admission-only fields")
	}
}

func admissionRequestFixture() []byte {
	return []byte(`{"contract_version":"haft.memory.v1","action":"admit","basis":{"kind":"exact_project","type_env_digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","graph_revision":17},"authority_class":"non_binding_semantic_assertion","idempotency_key":"typed-memory-admit-fixture","request_provenance_ref":"provenance:typed-memory-admit-fixture","change_set":{"changes":[{"kind":"declare_entity","entity_id":"entity:typed-memory-admit-fixture","local_ref":"local:typed-memory-admit-fixture","context":"haft-project","label":"Typed-memory admission fixture","provenance":"provenance:typed-memory-admit-fixture-change"}]}}`)
}
