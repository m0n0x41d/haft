package typedmemorywire

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const testDigest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
const alternateTestDigest = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

func TestValidateRequestProofDistinguishesDecodedValueFromZeroValue(t *testing.T) {
	zero := ValidateRequest{}
	if IsDecodedValidateRequest(zero) {
		t.Fatal("zero ValidateRequest carries decoder proof")
	}
	if _, err := zero.BindChangeSet(testTypeEnvRef(t)); err == nil {
		t.Fatal("zero ValidateRequest lowered a change set")
	}

	decoded, err := DecodeValidateRequest(validRelationPayload("Unknown.RelationSignature"))
	if err != nil {
		t.Fatalf("DecodeValidateRequest() error = %v", err)
	}
	if !IsDecodedValidateRequest(decoded) {
		t.Fatal("strict decoder did not attach ValidateRequest proof")
	}
}

func TestDecodeValidateRequestBuildsClosedTypedMemoryChangeSet(t *testing.T) {
	payload := []byte(fmt.Sprintf(`{
  "contract_version": %q,
  "action": "validate",
  "basis": {
	"kind": "exact_project",
    "type_env_digest": %q,
    "graph_revision": 17
  },
  "change_set": {
    "changes": [
      {
        "kind": "declare_entity",
        "entity_id": "entity-auth-service",
        "local_ref": "local-auth-service",
        "context": "haft-v9",
        "label": "Authentication service",
        "provenance": "operator:stream-design"
      },
      {
        "kind": "identity_change",
        "change": {
          "kind": "admit_alias",
          "entity_id": "entity-auth-service",
          "alias": "auth-service",
          "context": "haft-v9",
          "provenance": "operator:stream-design"
        }
      },
      {
        "kind": "instantiate_relation",
        "assertion_id": "assertion-auth-concern",
        "signature_id": "Local.EntityOfConcern",
        "context_slice": %s,
        "bindings": [
          {
            "slot_kind": "Local.EntityOfConcernSlot",
            "fillers": [
              {
                "kind": "by_reference",
                "reference": {
                  "kind": "local",
                  "ref_kind": "U.EntityRef",
                  "local_ref": "local-auth-service"
                }
              }
            ]
          },
          {
            "slot_kind": "Local.DescriptionSlot",
            "fillers": [
              {
                "kind": "by_value",
                "value": {
                  "value_kind": "U.ClaimGraph",
                  "value_shape": {
                    "id": "Haft.ClaimGraph",
                    "digest": %q
                  },
                  "codec": {
                    "id": "Haft.ClaimGraphCodec",
                    "version": "v1",
                    "specification_digest": %q
                  },
                  "input_base64": "e30=",
                  "asserted_digest": {"kind": "none"}
                }
              }
            ]
          }
        ],
        "provenance": "agent:typed-memory-wire-test"
      },
      {
        "kind": "retract_assertion",
        "assertion_id": "assertion-obsolete",
        "reason": "Superseded by the exact concern relation",
        "provenance": "operator:stream-design"
      }
    ]
  }
}`, ContractVersion, testDigest, testContextSliceJSON("haft-v9"), testDigest, testDigest))

	request, err := DecodeValidateRequest(payload)
	if err != nil {
		t.Fatalf("DecodeValidateRequest() error = %v", err)
	}
	if request.ContractVersion() != ContractVersion {
		t.Fatalf("ContractVersion() = %q", request.ContractVersion())
	}
	if request.Action() != ActionValidate {
		t.Fatalf("Action() = %q", request.Action())
	}
	basis, ok := request.Basis().(ExactProjectSelector)
	if !ok {
		t.Fatalf("basis = %T", request.Basis())
	}
	digest := basis.RequestedTypeEnvDigest()
	if digest.String() != testDigest {
		t.Fatalf("requested TypeEnv digest = %q", digest.String())
	}
	if basis.RequestedGraphRevision().Value() != 17 {
		t.Fatalf("requested GraphRevision() = %d", basis.RequestedGraphRevision().Value())
	}

	typeEnv := testTypeEnvRef(t)
	changeSet, err := request.BindChangeSet(typeEnv)
	if err != nil {
		t.Fatalf("BindChangeSet() error = %v", err)
	}
	changes := changeSet.Changes()
	if len(changes) != 4 {
		t.Fatalf("changes = %d, want 4", len(changes))
	}
	if _, ok := changes[0].(typedmemory.DeclareEntity); !ok {
		t.Fatalf("change[0] = %T", changes[0])
	}
	if _, ok := changes[1].(typedmemory.ApplyIdentityChange); !ok {
		t.Fatalf("change[1] = %T", changes[1])
	}
	relationChange, ok := changes[2].(typedmemory.InstantiateRelation)
	if !ok {
		t.Fatalf("change[2] = %T", changes[2])
	}
	relation := relationChange.Relation()
	if len(relation.Bindings()) != 2 {
		t.Fatalf("relation bindings = %d", len(relation.Bindings()))
	}
	if relation.Context().String() != "haft-v9" {
		t.Fatalf("relation context = %q", relation.Context().String())
	}
	if relation.Slice().Ref().String() == "" {
		t.Fatal("relation ContextSlice has no content-addressed ref")
	}
	if _, ok := changes[3].(typedmemory.RetractAssertion); !ok {
		t.Fatalf("change[3] = %T", changes[3])
	}
}

func TestDecodeValidateRequestDoesNotPerformSemanticValidation(t *testing.T) {
	payload := validRelationPayload("Unknown.RelationSignature")
	request, err := DecodeValidateRequest(payload)
	if err != nil {
		t.Fatalf("wire decoder performed semantic validation: %v", err)
	}
	typeEnv := testTypeEnvRef(t)
	changeSet, err := request.BindChangeSet(typeEnv)
	if err != nil {
		t.Fatalf("BindChangeSet() error = %v", err)
	}
	changes := changeSet.Changes()
	relationChange, ok := changes[0].(typedmemory.InstantiateRelation)
	if !ok {
		t.Fatalf("decoded change = %T", changes[0])
	}
	relation := relationChange.Relation()
	signature := relation.Signature()
	if signature.ID().String() != "Unknown.RelationSignature" {
		t.Fatalf("signature ID = %q", signature.ID().String())
	}
}

func TestDecodeRelationRequiresCompleteContextSliceAndRejectsLegacyContext(t *testing.T) {
	legacy := []byte(fmt.Sprintf(`{"contract_version":%q,"action":"validate","basis":{"kind":"project_current"},"change_set":{"changes":[{"kind":"instantiate_relation","assertion_id":"assertion-1","signature_id":"Local.Relation","context":"project","bindings":[{"slot_kind":"Local.EntitySlot","fillers":[{"kind":"by_reference","reference":{"kind":"persisted","ref_kind":"U.EntityRef","id":"entity-1"}}]}],"provenance":"p"}]}}`, ContractVersion))
	_, err := DecodeValidateRequest(legacy)
	assertDecodeErrorCode(t, err, ErrorInvalidContract)

	missingGamma := `{"context":"project","standard_pins":[],"environment_selectors":[],"vocabulary_pins":[],"role_set_pins":[]}`
	payload := relationPayloadWithContextSlice("Local.Relation", missingGamma)
	_, err = DecodeValidateRequest(payload)
	assertDecodeErrorCode(t, err, ErrorInvalidContract)

	unknownField := strings.Replace(
		testContextSliceJSON("project"),
		`"gamma_time":`,
		`"authority":true,"gamma_time":`,
		1,
	)
	_, err = DecodeValidateRequest(relationPayloadWithContextSlice("Local.Relation", unknownField))
	assertDecodeErrorCode(t, err, ErrorInvalidContract)

	implicitPin := fmt.Sprintf(`{"context":"project","standard_pins":[{"reference":"standard:api","edition":"latest","digest":%q}],"environment_selectors":[],"vocabulary_pins":[],"role_set_pins":[],"gamma_time":{"kind":"point","at":"2026-07-16T08:00:00Z"}}`, testDigest)
	_, err = DecodeValidateRequest(relationPayloadWithContextSlice("Local.Relation", implicitPin))
	assertDecodeErrorCode(t, err, ErrorInvalidContract)
}

func TestDecodeRelationRetainsExactContextSliceAlgebra(t *testing.T) {
	sliceJSON := fmt.Sprintf(`{
  "context":"project",
  "standard_pins":[{"reference":"standard:api","edition":"v2","digest":%q}],
  "environment_selectors":[{"key":"platform","value":"linux-arm64","source_digest":%q}],
  "vocabulary_pins":[{"reference":"vocabulary:domain","edition":"2026-07","digest":%q}],
  "role_set_pins":[{"reference":"roles:delivery","edition":"v4","digest":%q}],
  "gamma_time":{"kind":"window","start":"2026-07-16T08:00:00Z","end":"2026-07-16T09:00:00Z","start_boundary":"inclusive","end_boundary":"exclusive"}
}`, testDigest, alternateTestDigest, testDigest, alternateTestDigest)
	payload := relationPayloadWithContextSlice("Local.Relation", sliceJSON)
	request, err := DecodeValidateRequest(payload)
	if err != nil {
		t.Fatalf("DecodeValidateRequest() error = %v", err)
	}
	changeSet, err := request.BindChangeSet(testTypeEnvRef(t))
	if err != nil {
		t.Fatalf("BindChangeSet() error = %v", err)
	}
	relation := changeSet.Changes()[0].(typedmemory.InstantiateRelation).Relation()
	slice := relation.Slice()
	if relation.Context() != slice.Context() || relation.Context().String() != "project" {
		t.Fatal("relation context was not derived from its ContextSlice")
	}
	if len(slice.StandardPins()) != 1 ||
		len(slice.EnvironmentSelectors()) != 1 ||
		len(slice.VocabularyPins()) != 1 ||
		len(slice.RoleSetPins()) != 1 {
		t.Fatal("wire decoding lost a ContextSlice position")
	}
	window, ok := slice.GammaTime().(typedmemory.GammaWindow)
	if !ok {
		t.Fatalf("GammaTime() = %T; want GammaWindow", slice.GammaTime())
	}
	if window.StartBoundary() != typedmemory.GammaBoundaryInclusive ||
		window.EndBoundary() != typedmemory.GammaBoundaryExclusive {
		t.Fatal("wire decoding changed Gamma window boundary semantics")
	}
}

func TestDecodeRelationSupportsResolvedGammaPolicyApplication(t *testing.T) {
	sliceJSON := fmt.Sprintf(`{
  "context":"project",
  "standard_pins":[],
  "environment_selectors":[],
  "vocabulary_pins":[],
  "role_set_pins":[],
  "gamma_time":{
    "kind":"policy_application",
    "policy_ref":"policy:rolling-window",
    "policy_edition":"2026-07",
    "policy_digest":%q,
    "evaluation_anchor":{"kind":"point","at":"2026-07-16T08:00:00Z"},
    "resolved":{"kind":"point","at":"2026-07-15T08:00:00Z"}
  }
}`, testDigest)
	payload := relationPayloadWithContextSlice("Local.Relation", sliceJSON)
	request, err := DecodeValidateRequest(payload)
	if err != nil {
		t.Fatalf("DecodeValidateRequest() error = %v", err)
	}
	changeSet, err := request.BindChangeSet(testTypeEnvRef(t))
	if err != nil {
		t.Fatalf("BindChangeSet() error = %v", err)
	}
	relation := changeSet.Changes()[0].(typedmemory.InstantiateRelation).Relation()
	application, ok := relation.Slice().GammaTime().(typedmemory.GammaPolicyApplication)
	if !ok {
		t.Fatalf("GammaTime() = %T; want GammaPolicyApplication", relation.Slice().GammaTime())
	}
	if application.PolicyRef().String() != "policy:rolling-window" {
		t.Fatalf("policy ref = %q", application.PolicyRef().String())
	}
	if _, ok := application.Resolved().(typedmemory.GammaPoint); !ok {
		t.Fatalf("resolved Gamma_time = %T; want GammaPoint", application.Resolved())
	}

	nested := strings.Replace(
		sliceJSON,
		`"resolved":{"kind":"point","at":"2026-07-15T08:00:00Z"}`,
		`"resolved":{"kind":"policy_application","policy_ref":"policy:nested","policy_edition":"v1","policy_digest":"`+testDigest+`","evaluation_anchor":{"kind":"point","at":"2026-07-16T08:00:00Z"},"resolved":{"kind":"point","at":"2026-07-15T08:00:00Z"}}`,
		1,
	)
	_, err = DecodeValidateRequest(relationPayloadWithContextSlice("Local.Relation", nested))
	assertDecodeErrorCode(t, err, ErrorInvalidContract)
}

func TestDecodeValidateRequestSupportsEveryClosedIdentityVariant(t *testing.T) {
	tests := []struct {
		name       string
		changeJSON string
		assertType func(*testing.T, typedmemory.IdentityChange)
	}{
		{
			name:       "admit alias",
			changeJSON: `{"kind":"admit_alias","entity_id":"e1","alias":"service","context":"project","provenance":"p"}`,
			assertType: func(t *testing.T, change typedmemory.IdentityChange) {
				t.Helper()
				if _, ok := change.(typedmemory.AdmitAlias); !ok {
					t.Fatalf("identity change = %T", change)
				}
			},
		},
		{
			name:       "supersede alias",
			changeJSON: `{"kind":"supersede_alias","entity_id":"e1","old_alias":"old","replacement":"new","context":"project","provenance":"p"}`,
			assertType: func(t *testing.T, change typedmemory.IdentityChange) {
				t.Helper()
				if _, ok := change.(typedmemory.SupersedeAlias); !ok {
					t.Fatalf("identity change = %T", change)
				}
			},
		},
		{
			name:       "merge entities",
			changeJSON: `{"kind":"merge_entities","survivor":"e1","merged":["e2"],"context":"project","basis":"review:1"}`,
			assertType: func(t *testing.T, change typedmemory.IdentityChange) {
				t.Helper()
				if _, ok := change.(typedmemory.MergeEntities); !ok {
					t.Fatalf("identity change = %T", change)
				}
			},
		},
		{
			name:       "split entity",
			changeJSON: `{"kind":"split_entity","source":"e1","targets":["e2","e3"],"context":"project","basis":"review:1"}`,
			assertType: func(t *testing.T, change typedmemory.IdentityChange) {
				t.Helper()
				if _, ok := change.(typedmemory.SplitEntity); !ok {
					t.Fatalf("identity change = %T", change)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			payload := requestWithIdentityChange(test.changeJSON)
			request, err := DecodeValidateRequest(payload)
			if err != nil {
				t.Fatalf("DecodeValidateRequest() error = %v", err)
			}
			changeSet, err := request.BindChangeSet(testTypeEnvRef(t))
			if err != nil {
				t.Fatalf("BindChangeSet() error = %v", err)
			}
			changes := changeSet.Changes()
			identityMemoryChange := changes[0].(typedmemory.ApplyIdentityChange)
			test.assertType(t, identityMemoryChange.Change())
		})
	}
}

func TestBasisSelectorsRemainUntrustedUntilServiceResolution(t *testing.T) {
	tests := []struct {
		name      string
		basisJSON string
		wantKind  BasisKind
	}{
		{name: "bundled open-world candidate", basisJSON: `{"kind":"bundled_candidate_open_world"}`, wantKind: BasisBundledCandidateOpenWorld},
		{name: "project current", basisJSON: `{"kind":"project_current"}`, wantKind: BasisProjectCurrent},
		{name: "exact project", basisJSON: `{"kind":"exact_project","type_env_digest":"` + testDigest + `","graph_revision":7}`, wantKind: BasisExactProject},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			payload := requestWithBasis(test.basisJSON)
			request, err := DecodeValidateRequest(payload)
			if err != nil {
				t.Fatalf("DecodeValidateRequest() error = %v", err)
			}
			if request.Basis().Kind() != test.wantKind {
				t.Fatalf("basis kind = %q, want %q", request.Basis().Kind(), test.wantKind)
			}
		})
	}

	badBundled := requestWithBasis(`{"kind":"bundled_candidate_open_world","graph_revision":7}`)
	_, err := DecodeValidateRequest(badBundled)
	assertDecodeErrorCode(t, err, ErrorInvalidContract)

	badCurrent := requestWithBasis(`{"kind":"project_current","type_env_digest":"` + testDigest + `"}`)
	_, err = DecodeValidateRequest(badCurrent)
	assertDecodeErrorCode(t, err, ErrorInvalidContract)
}

func TestBindChangeSetUsesOnlyServerResolvedTypeEnv(t *testing.T) {
	payload := validRelationExactPayload("Local.Relation")
	request, err := DecodeValidateRequest(payload)
	if err != nil {
		t.Fatalf("DecodeValidateRequest() error = %v", err)
	}
	requested := request.Basis().(ExactProjectSelector)
	if requested.RequestedTypeEnvDigest().String() != testDigest {
		t.Fatalf("requested digest = %q", requested.RequestedTypeEnvDigest().String())
	}

	resolvedDigest, err := typedmemory.NewSHA256Digest(alternateTestDigest)
	if err != nil {
		t.Fatalf("NewSHA256Digest(): %v", err)
	}
	resolvedRef, err := typedmemory.NewTypeEnvRef(resolvedDigest)
	if err != nil {
		t.Fatalf("NewTypeEnvRef(): %v", err)
	}
	changeSet, err := request.BindChangeSet(resolvedRef)
	if err != nil {
		t.Fatalf("BindChangeSet() error = %v", err)
	}
	changes := changeSet.Changes()
	relationChange := changes[0].(typedmemory.InstantiateRelation)
	relation := relationChange.Relation()
	actualRef := relation.Signature().TypeEnv()
	actualDigest := actualRef.Digest()
	if actualDigest.String() != alternateTestDigest {
		t.Fatalf("bound digest = %q; request digest was trusted", actualDigest.String())
	}
}

func TestDecodeValidateRequestRejectsProtocolAndJSONAttacks(t *testing.T) {
	valid := validDeclarePayload()
	exact := validDeclareExactPayload()
	tests := []struct {
		name    string
		payload []byte
		code    ErrorCode
	}{
		{name: "empty", payload: nil, code: ErrorMalformedJSON},
		{name: "trailing value", payload: append(append([]byte{}, valid...), []byte(` {}`)...), code: ErrorMalformedJSON},
		{name: "duplicate top field", payload: []byte(fmt.Sprintf(`{"contract_version":%q,"contract_version":%q,"action":"validate","basis":{"kind":"project_current"},"change_set":{"changes":[]}}`, ContractVersion, ContractVersion)), code: ErrorInvalidContract},
		{name: "duplicate nested field", payload: []byte(fmt.Sprintf(`{"contract_version":%q,"action":"validate","basis":{"kind":"exact_project","type_env_digest":%q,"graph_revision":0,"graph_revision":1},"change_set":{"changes":[]}}`, ContractVersion, testDigest)), code: ErrorInvalidContract},
		{name: "null", payload: []byte(fmt.Sprintf(`{"contract_version":%q,"action":"validate","basis":null,"change_set":{"changes":[]}}`, ContractVersion)), code: ErrorInvalidContract},
		{name: "unknown top field", payload: insertBeforeFinalBrace(valid, `,"write":true`), code: ErrorInvalidContract},
		{name: "wrong version", payload: bytes.Replace(valid, []byte(ContractVersion), []byte("haft.memory.v999"), 1), code: ErrorInvalidContract},
		{name: "wrong action", payload: bytes.Replace(valid, []byte(`"validate"`), []byte(`"admit"`), 1), code: ErrorInvalidContract},
		{name: "missing graph revision", payload: bytes.Replace(exact, []byte(`,"graph_revision":0`), nil, 1), code: ErrorInvalidContract},
		{name: "negative graph revision", payload: bytes.Replace(exact, []byte(`"graph_revision":0`), []byte(`"graph_revision":-1`), 1), code: ErrorInvalidContract},
		{name: "fractional graph revision", payload: bytes.Replace(exact, []byte(`"graph_revision":0`), []byte(`"graph_revision":1.5`), 1), code: ErrorInvalidContract},
		{name: "unknown change kind", payload: bytes.Replace(valid, []byte(`"declare_entity"`), []byte(`"declare_kind"`), 1), code: ErrorInvalidContract},
		{name: "unknown nested field", payload: bytes.Replace(valid, []byte(`"provenance":"p"`), []byte(`"provenance":"p","authority":true`), 1), code: ErrorInvalidContract},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := DecodeValidateRequest(test.payload)
			assertDecodeErrorCode(t, err, test.code)
		})
	}
}

func TestDecodeValidateRequestRejectsResourceExhaustionBeforeConstruction(t *testing.T) {
	tooLarge := bytes.Repeat([]byte("x"), MaximumRequestBytes+1)
	tooDeep := []byte(strings.Repeat("[", MaximumJSONDepth+1) + strings.Repeat("]", MaximumJSONDepth+1))
	tooManyChanges := requestWithArray("changes", MaximumChanges+1)
	tooManyFields := objectWithFields(MaximumObjectFields + 1)

	tests := []struct {
		name    string
		payload []byte
	}{
		{name: "request bytes", payload: tooLarge},
		{name: "nesting depth", payload: tooDeep},
		{name: "change count", payload: tooManyChanges},
		{name: "object fields", payload: tooManyFields},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := DecodeValidateRequest(test.payload)
			assertDecodeErrorCode(t, err, ErrorResourceLimit)
		})
	}
}

func TestDecodeValidateRequestRejectsValueAndReferenceSmuggling(t *testing.T) {
	payload := validRelationPayload("Local.Relation")
	tests := []struct {
		name string
		from string
		to   string
	}{
		{name: "unknown filler field", from: `"reference":{"kind":"persisted"`, to: `"reference":{"authority":"operator","kind":"persisted"`},
		{name: "unknown reference kind", from: `"kind":"persisted","ref_kind"`, to: `"kind":"weak","ref_kind"`},
		{name: "invalid qualified RefKind", from: `"ref_kind":"U.EntityRef"`, to: `"ref_kind":"U EntityRef"`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mutated := bytes.Replace(payload, []byte(test.from), []byte(test.to), 1)
			_, err := DecodeValidateRequest(mutated)
			assertDecodeErrorCode(t, err, ErrorInvalidContract)
		})
	}

	valuePayload := validValueRelationPayload()
	valueTests := []struct {
		name string
		from string
		to   string
	}{
		{name: "noncanonical base64", from: `"input_base64":"e30="`, to: `"input_base64":"e30"`},
		{name: "implicit digest posture", from: `,"asserted_digest":{"kind":"none"}`, to: ``},
		{name: "digest on none posture", from: `"asserted_digest":{"kind":"none"}`, to: `"asserted_digest":{"kind":"none","digest":"` + testDigest + `"}`},
		{name: "unknown codec field", from: `"specification_digest":"` + testDigest + `"`, to: `"specification_digest":"` + testDigest + `","command":"cat /etc/passwd"`},
	}
	for _, test := range valueTests {
		t.Run(test.name, func(t *testing.T) {
			mutated := bytes.Replace(valuePayload, []byte(test.from), []byte(test.to), 1)
			_, err := DecodeValidateRequest(mutated)
			assertDecodeErrorCode(t, err, ErrorInvalidContract)
		})
	}
}

func TestDecodeValidateRequestRejectsOversizedDecodedTypedValue(t *testing.T) {
	input := bytes.Repeat([]byte("x"), MaximumTypedValueBytes+1)
	encoded := make([]byte, ((len(input)+2)/3)*4)
	base64.StdEncoding.Encode(encoded, input)
	payload := validValueRelationPayload()
	payload = bytes.Replace(payload, []byte("e30="), encoded, 1)
	_, err := DecodeValidateRequest(payload)
	assertDecodeErrorCode(t, err, ErrorResourceLimit)
}

func TestDecodeValidateRequestAcceptsExactTypedValueByteLimit(t *testing.T) {
	input := bytes.Repeat([]byte("x"), MaximumTypedValueBytes)
	encoded := make([]byte, base64.StdEncoding.EncodedLen(len(input)))
	base64.StdEncoding.Encode(encoded, input)
	payload := validValueRelationPayload()
	payload = bytes.Replace(payload, []byte("e30="), encoded, 1)
	request, err := DecodeValidateRequest(payload)
	if err != nil {
		t.Fatalf("DecodeValidateRequest() error = %v", err)
	}
	changeSet, err := request.BindChangeSet(testTypeEnvRef(t))
	if err != nil {
		t.Fatalf("BindChangeSet() error = %v", err)
	}
	changes := changeSet.Changes()
	relationChange := changes[0].(typedmemory.InstantiateRelation)
	relation := relationChange.Relation()
	bindings := relation.Bindings()
	fillers := bindings[0].Fillers()
	valueFiller := fillers[0].(typedmemory.ByValueCandidate)
	value := valueFiller.Value()
	if len(value.InputBytes()) != MaximumTypedValueBytes {
		t.Fatalf("input bytes = %d", len(value.InputBytes()))
	}
}

func TestDecodeValidateRequestRejectsMoreThanSixtyFourSlotsOrFillers(t *testing.T) {
	base := baseRequestMap(t)
	changes := base["change_set"].(map[string]any)["changes"].([]any)
	relation := changes[0].(map[string]any)

	bindings := make([]any, MaximumSlotBindings+1)
	for index := range bindings {
		bindings[index] = map[string]any{
			"slot_kind": fmt.Sprintf("Local.Slot%d", index),
			"fillers": []any{map[string]any{
				"kind": "by_reference",
				"reference": map[string]any{
					"kind": "persisted", "ref_kind": "U.EntityRef", "id": fmt.Sprintf("entity-%d", index),
				},
			}},
		}
	}
	relation["bindings"] = bindings
	payload, err := json.Marshal(base)
	if err != nil {
		t.Fatalf("json.Marshal(): %v", err)
	}
	_, err = DecodeValidateRequest(payload)
	assertDecodeErrorCode(t, err, ErrorResourceLimit)

	base = baseRequestMap(t)
	changes = base["change_set"].(map[string]any)["changes"].([]any)
	relation = changes[0].(map[string]any)
	bindings = relation["bindings"].([]any)
	binding := bindings[0].(map[string]any)
	fillers := make([]any, MaximumFillersPerSlot+1)
	for index := range fillers {
		fillers[index] = map[string]any{
			"kind": "by_reference",
			"reference": map[string]any{
				"kind": "persisted", "ref_kind": "U.EntityRef", "id": fmt.Sprintf("entity-%d", index),
			},
		}
	}
	binding["fillers"] = fillers
	payload, err = json.Marshal(base)
	if err != nil {
		t.Fatalf("json.Marshal(): %v", err)
	}
	_, err = DecodeValidateRequest(payload)
	assertDecodeErrorCode(t, err, ErrorResourceLimit)
}

func TestDecodeValidateRequestRejectsDuplicateStructuralEffectsBeforeBasisResolution(t *testing.T) {
	base := baseRequestMap(t)
	changeSet := base["change_set"].(map[string]any)
	changes := changeSet["changes"].([]any)
	relation := changes[0].(map[string]any)
	bindings := relation["bindings"].([]any)
	relation["bindings"] = append(bindings, bindings[0])
	payload, err := json.Marshal(base)
	if err != nil {
		t.Fatalf("json.Marshal(): %v", err)
	}
	_, err = DecodeValidateRequest(payload)
	assertDecodeErrorCode(t, err, ErrorInvalidContract)

	base = baseRequestMap(t)
	changeSet = base["change_set"].(map[string]any)
	changes = changeSet["changes"].([]any)
	changeSet["changes"] = append(changes, changes[0])
	payload, err = json.Marshal(base)
	if err != nil {
		t.Fatalf("json.Marshal(): %v", err)
	}
	_, err = DecodeValidateRequest(payload)
	assertDecodeErrorCode(t, err, ErrorInvalidContract)

	first := `{"kind":"identity_change","change":{"kind":"admit_alias","entity_id":"e1","alias":"service","context":"project","provenance":"p1"}}`
	second := `{"kind":"identity_change","change":{"kind":"admit_alias","entity_id":"e2","alias":"service","context":"project","provenance":"p2"}}`
	payload = []byte(fmt.Sprintf(`{"contract_version":%q,"action":"validate","basis":{"kind":"project_current"},"change_set":{"changes":[%s,%s]}}`, ContractVersion, first, second))
	_, err = DecodeValidateRequest(payload)
	assertDecodeErrorCode(t, err, ErrorInvalidContract)
}

func assertDecodeErrorCode(t *testing.T, err error, code ErrorCode) {
	t.Helper()
	if err == nil {
		t.Fatalf("DecodeValidateRequest() error = nil; want %s", code)
	}
	decodeError := &DecodeError{}
	if !errors.As(err, &decodeError) {
		t.Fatalf("error = %T %v; want *DecodeError", err, err)
	}
	if decodeError.Code() != code {
		t.Fatalf("error code = %q, want %q; error=%v", decodeError.Code(), code, err)
	}
}

func validDeclarePayload() []byte {
	return []byte(fmt.Sprintf(`{"contract_version":%q,"action":"validate","basis":{"kind":"project_current"},"change_set":{"changes":[{"kind":"declare_entity","entity_id":"entity-1","local_ref":"local-1","context":"project","label":"Entity one","provenance":"p"}]}}`, ContractVersion))
}

func validDeclareExactPayload() []byte {
	return []byte(fmt.Sprintf(`{"contract_version":%q,"action":"validate","basis":{"kind":"exact_project","type_env_digest":%q,"graph_revision":0},"change_set":{"changes":[{"kind":"declare_entity","entity_id":"entity-1","local_ref":"local-1","context":"project","label":"Entity one","provenance":"p"}]}}`, ContractVersion, testDigest))
}

func validRelationPayload(signature string) []byte {
	return relationPayloadWithContextSlice(signature, testContextSliceJSON("project"))
}

func relationPayloadWithContextSlice(signature string, contextSliceJSON string) []byte {
	return []byte(fmt.Sprintf(`{"contract_version":%q,"action":"validate","basis":{"kind":"project_current"},"change_set":{"changes":[{"kind":"instantiate_relation","assertion_id":"assertion-1","signature_id":%q,"context_slice":%s,"bindings":[{"slot_kind":"Local.EntitySlot","fillers":[{"kind":"by_reference","reference":{"kind":"persisted","ref_kind":"U.EntityRef","id":"entity-1"}}]}],"provenance":"p"}]}}`, ContractVersion, signature, contextSliceJSON))
}

func validRelationExactPayload(signature string) []byte {
	payload := validRelationPayload(signature)
	current := []byte(`{"kind":"project_current"}`)
	exact := []byte(`{"kind":"exact_project","type_env_digest":"` + testDigest + `","graph_revision":0}`)
	return bytes.Replace(payload, current, exact, 1)
}

func validValueRelationPayload() []byte {
	return []byte(fmt.Sprintf(`{"contract_version":%q,"action":"validate","basis":{"kind":"project_current"},"change_set":{"changes":[{"kind":"instantiate_relation","assertion_id":"assertion-1","signature_id":"Local.Relation","context_slice":%s,"bindings":[{"slot_kind":"Local.ValueSlot","fillers":[{"kind":"by_value","value":{"value_kind":"U.ClaimGraph","value_shape":{"id":"Haft.ClaimGraph","digest":%q},"codec":{"id":"Haft.ClaimGraphCodec","version":"v1","specification_digest":%q},"input_base64":"e30=","asserted_digest":{"kind":"none"}}}]}],"provenance":"p"}]}}`, ContractVersion, testContextSliceJSON("project"), testDigest, testDigest))
}

func testContextSliceJSON(context string) string {
	return fmt.Sprintf(
		`{"context":%q,"standard_pins":[],"environment_selectors":[],"vocabulary_pins":[],"role_set_pins":[],"gamma_time":{"kind":"point","at":"2026-07-16T08:00:00Z"}}`,
		context,
	)
}

func insertBeforeFinalBrace(payload []byte, insertion string) []byte {
	trimmed := bytes.TrimSpace(payload)
	result := append([]byte{}, trimmed[:len(trimmed)-1]...)
	result = append(result, []byte(insertion)...)
	return append(result, '}')
}

func requestWithArray(field string, count int) []byte {
	items := make([]string, count)
	for index := range items {
		items[index] = `{}`
	}
	return []byte(fmt.Sprintf(`{"contract_version":%q,"action":"validate","basis":{"kind":"project_current"},"change_set":{"%s":[%s]}}`, ContractVersion, field, strings.Join(items, ",")))
}

func requestWithBasis(basisJSON string) []byte {
	return []byte(fmt.Sprintf(`{"contract_version":%q,"action":"validate","basis":%s,"change_set":{"changes":[{"kind":"declare_entity","entity_id":"entity-1","local_ref":"local-1","context":"project","label":"Entity one","provenance":"p"}]}}`, ContractVersion, basisJSON))
}

func requestWithIdentityChange(changeJSON string) []byte {
	return []byte(fmt.Sprintf(`{"contract_version":%q,"action":"validate","basis":{"kind":"project_current"},"change_set":{"changes":[{"kind":"identity_change","change":%s}]}}`, ContractVersion, changeJSON))
}

func objectWithFields(count int) []byte {
	fields := make([]string, count)
	for index := range fields {
		fields[index] = fmt.Sprintf(`"field_%d":%d`, index, index)
	}
	return []byte("{" + strings.Join(fields, ",") + "}")
}

func baseRequestMap(t *testing.T) map[string]any {
	t.Helper()
	payload := validRelationPayload("Local.Relation")
	request := map[string]any{}
	if err := json.Unmarshal(payload, &request); err != nil {
		t.Fatalf("json.Unmarshal(): %v", err)
	}
	return request
}

func testTypeEnvRef(t *testing.T) typedmemory.TypeEnvRef {
	t.Helper()
	digest, err := typedmemory.NewSHA256Digest(testDigest)
	if err != nil {
		t.Fatalf("NewSHA256Digest(): %v", err)
	}
	ref, err := typedmemory.NewTypeEnvRef(digest)
	if err != nil {
		t.Fatalf("NewTypeEnvRef(): %v", err)
	}
	return ref
}
