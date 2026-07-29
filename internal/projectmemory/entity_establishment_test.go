package projectmemory

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

func TestDecodeEntityEstablishmentRequestBuildsCanonicalSameBatchIntent(
	t *testing.T,
) {
	t.Parallel()

	request, err := DecodeEntityEstablishmentRequest([]byte(`{
		"action":"establish",
		"entity_id":"service:auth",
		"label":"Authentication service",
		"bounded_context_ref":"context:software-system",
		"aliases":["auth","auth-service"],
		"persistence_reason":"named_receiving_use",
		"request_provenance_ref":"task:release-readiness",
		"idempotency_key":"entity:service:auth:v1"
	}`))
	if err != nil {
		t.Fatalf("DecodeEntityEstablishmentRequest() error = %v", err)
	}
	aliases := request.Aliases()
	if len(aliases) != 2 ||
		aliases[0].String() != "auth" ||
		aliases[1].String() != "auth-service" {
		t.Fatalf("canonical aliases = %#v", aliases)
	}
	if request.PersistenceReason() != EntityPersistenceNamedReceivingUse {
		t.Fatalf("persistence reason = %q", request.PersistenceReason())
	}

	candidate, err := request.Candidate()
	if err != nil {
		t.Fatalf("Candidate() error = %v", err)
	}
	changes := candidate.Changes()
	if len(changes) != 3 {
		t.Fatalf("candidate changes = %d, want declaration + two aliases", len(changes))
	}
	declaration, ok := changes[0].(typedmemory.DeclareEntity)
	if !ok {
		t.Fatalf("first change = %T, want DeclareEntity", changes[0])
	}
	if declaration.Entity().String() != "service:auth" ||
		declaration.Context().String() != "context:software-system" ||
		declaration.Label().String() != "Authentication service" {
		t.Fatalf("declaration = %#v", declaration)
	}
	for index, raw := range changes[1:] {
		effect, effectOK := raw.(typedmemory.ApplyIdentityChange)
		if !effectOK {
			t.Fatalf("change %d = %T, want ApplyIdentityChange", index+1, raw)
		}
		alias, aliasOK := effect.Change().(typedmemory.AdmitAlias)
		if !aliasOK || alias.Entity() != declaration.Entity() {
			t.Fatalf("alias change %d = %#v", index+1, effect.Change())
		}
	}
}

func TestDecodeEntityEstablishmentRequestRejectsLowLevelAndAmbiguousInputs(
	t *testing.T,
) {
	t.Parallel()

	base := `{
		"action":"establish",
		"entity_id":"service:auth",
		"label":"Authentication service",
		"bounded_context_ref":"context:software-system",
		"aliases":[],
		"persistence_reason":"explicit_operator_request",
		"request_provenance_ref":"operator:chat",
		"idempotency_key":"entity:service:auth:v1"
	}`
	tests := []struct {
		name    string
		payload string
	}{
		{
			name: "TypeEnv is not agent input",
			payload: strings.Replace(
				base,
				`"action":"establish",`,
				`"action":"establish","type_env_digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",`,
				1,
			),
		},
		{
			name: "duplicate field",
			payload: strings.Replace(
				base,
				`"action":"establish",`,
				`"action":"establish","action":"establish",`,
				1,
			),
		},
		{
			name: "duplicate alias",
			payload: strings.Replace(
				base,
				`"aliases":[]`,
				`"aliases":["auth","auth"]`,
				1,
			),
		},
		{
			name: "unsorted aliases",
			payload: strings.Replace(
				base,
				`"aliases":[]`,
				`"aliases":["auth-service","auth"]`,
				1,
			),
		},
		{
			name: "invented persistence authority",
			payload: strings.Replace(
				base,
				`"persistence_reason":"explicit_operator_request"`,
				`"persistence_reason":"known_absent"`,
				1,
			),
		},
		{
			name: "noncanonical identifier",
			payload: strings.Replace(
				base,
				`"entity_id":"service:auth"`,
				`"entity_id":" service:auth "`,
				1,
			),
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := DecodeEntityEstablishmentRequest(
				[]byte(test.payload),
			); err == nil {
				t.Fatal("invalid request was accepted")
			}
		})
	}
}

func TestEntityEstablishedResponseCarriesVerbatimCanonicalEntityRef(
	t *testing.T,
) {
	t.Parallel()

	request := mustEntityEstablishmentRequest(t)
	result, err := NewAlreadyExactEntityEstablished(request)
	if err != nil {
		t.Fatalf("NewAlreadyExactEntityEstablished() error = %v", err)
	}
	payload, err := MarshalEntityEstablishmentResult(result)
	if err != nil {
		t.Fatalf("MarshalEntityEstablishmentResult() error = %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["result"] != "established" ||
		decoded["delivery_kind"] != "already_exact" {
		t.Fatalf("established response = %s", payload)
	}
	reference, ok := decoded["entity_ref"].(map[string]any)
	if !ok ||
		reference["ref_kind_id"] != "U.EntityRef" ||
		reference["reference_id"] != "service:auth" {
		t.Fatalf("entity_ref = %#v", decoded["entity_ref"])
	}
	nextRead, ok := decoded["next_read"].(map[string]any)
	if !ok || nextRead["tool"] != "haft_query" {
		t.Fatalf("next_read = %#v", decoded["next_read"])
	}
	arguments, ok := nextRead["arguments"].(map[string]any)
	if !ok {
		t.Fatalf("next_read.arguments = %#v", nextRead["arguments"])
	}
	argumentsPayload, err := json.Marshal(arguments)
	if err != nil {
		t.Fatal(err)
	}
	neighborhood, err := typedmemorywire.DecodeQueryReadRequest(
		argumentsPayload,
	)
	if err != nil {
		t.Fatalf("next_read round-trip decode: %v\n%s", err, argumentsPayload)
	}
	exactNeighborhood, ok := neighborhood.(typedmemorywire.NeighborhoodReadRequest)
	if !ok ||
		exactNeighborhood.Entity().RefKindID().String() != "U.EntityRef" ||
		exactNeighborhood.Entity().ReferenceID().String() != "service:auth" {
		t.Fatalf("next_read neighborhood = %#v", neighborhood)
	}
	for _, forbidden := range []string{
		"type_env",
		"graph_revision",
		"memory_change_set",
		"authority_class",
		"ref_kind\"",
	} {
		if strings.Contains(string(payload), forbidden) {
			t.Fatalf("response leaked %q: %s", forbidden, payload)
		}
	}
}

func TestEntityRecoveryResultCarriesClosedNextActionWithoutWrite(t *testing.T) {
	t.Parallel()

	result, err := NewEntityEnablementChoiceRequired(
		"Typed project memory is not enabled for this project.",
	)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := MarshalEntityEstablishmentResult(result)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Result      string `json:"result"`
		NextAction  string `json:"next_action"`
		Persistence struct {
			Performed        bool `json:"performed"`
			AuthorityGranted bool `json:"authority_granted"`
		} `json:"persistence"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Result != "enablement_choice_required" ||
		decoded.NextAction == "" ||
		decoded.Persistence.Performed ||
		decoded.Persistence.AuthorityGranted {
		t.Fatalf("recovery response = %s", payload)
	}
}

func mustEntityEstablishmentRequest(
	t *testing.T,
) EntityEstablishmentRequest {
	t.Helper()
	request, err := DecodeEntityEstablishmentRequest([]byte(`{
		"action":"establish",
		"entity_id":"service:auth",
		"label":"Authentication service",
		"bounded_context_ref":"context:software-system",
		"aliases":["auth"],
		"persistence_reason":"explicit_operator_request",
		"request_provenance_ref":"operator:chat",
		"idempotency_key":"entity:service:auth:v1"
	}`))
	if err != nil {
		t.Fatal(err)
	}
	return request
}
