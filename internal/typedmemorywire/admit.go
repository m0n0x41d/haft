package typedmemorywire

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	AuthorityClassNonBindingSemanticAssertion = "non_binding_semantic_assertion"
	MaximumAdmissionIdempotencyKeyBytes       = 512
)

// Request is the sealed top-level wire request union. Decoding one variant
// never broadens the fields accepted by another variant.
type Request interface {
	ContractVersion() string
	Action() string
	requestVariant()
}

// AdmitRequest is an opaque decoder-owned non-binding admission request. It
// carries an exact-project validation request plus effect coordinates, but no
// project identity, TypeEnv-head mutation, schema mutation, decision,
// commission, or spec-lifecycle authority.
type AdmitRequest struct {
	proof             *admitRequestProof
	validation        ValidateRequest
	idempotencyKey    string
	requestProvenance typedmemory.ProvenanceRef
}

type admitRequestProof struct{}

var decodedAdmitRequestProof = &admitRequestProof{}

func (request AdmitRequest) ContractVersion() string {
	if !IsDecodedAdmitRequest(request) {
		return ""
	}
	return request.validation.ContractVersion()
}

func (AdmitRequest) Action() string { return ActionAdmit }

func (AdmitRequest) AuthorityClass() string {
	return AuthorityClassNonBindingSemanticAssertion
}

func (AdmitRequest) requestVariant() {}

func (request AdmitRequest) ValidationRequest() ValidateRequest {
	if !IsDecodedAdmitRequest(request) {
		return ValidateRequest{}
	}
	return request.validation
}

func (request AdmitRequest) Basis() ExactProjectSelector {
	if !IsDecodedAdmitRequest(request) {
		return ExactProjectSelector{}
	}
	selector, _ := request.validation.Basis().(ExactProjectSelector)
	return selector
}

func (request AdmitRequest) IdempotencyKey() string {
	if !IsDecodedAdmitRequest(request) {
		return ""
	}
	return request.idempotencyKey
}

func (request AdmitRequest) RequestProvenance() typedmemory.ProvenanceRef {
	if !IsDecodedAdmitRequest(request) {
		return typedmemory.ProvenanceRef{}
	}
	return request.requestProvenance
}

func IsDecodedAdmitRequest(request AdmitRequest) bool {
	if request.proof != decodedAdmitRequestProof {
		return false
	}
	if !IsDecodedValidateRequest(request.validation) {
		return false
	}
	if _, exact := request.validation.Basis().(ExactProjectSelector); !exact {
		return false
	}
	if request.idempotencyKey == "" ||
		request.requestProvenance.String() == "" {
		return false
	}
	return true
}

type admitRequestWire struct {
	ContractVersion      string          `json:"contract_version"`
	Action               string          `json:"action"`
	Basis                json.RawMessage `json:"basis"`
	AuthorityClass       string          `json:"authority_class"`
	IdempotencyKey       string          `json:"idempotency_key"`
	RequestProvenanceRef string          `json:"request_provenance_ref"`
	ChangeSet            json.RawMessage `json:"change_set"`
}

func DecodeRequest(payload []byte) (Request, error) {
	if err := scanStrictJSON(payload); err != nil {
		return nil, err
	}
	discriminator := struct {
		Action string `json:"action"`
	}{}
	if err := json.Unmarshal(payload, &discriminator); err != nil {
		return nil, invalidContract("$", "request must be a JSON object")
	}
	if err := requireIdentifier(discriminator.Action, "$.action"); err != nil {
		return nil, err
	}
	switch discriminator.Action {
	case ActionValidate:
		return DecodeValidateRequest(payload)
	case ActionAdmit:
		return DecodeAdmitRequest(payload)
	case ActionResolve:
		return DecodeResolveReadRequest(payload)
	case ActionNeighborhood:
		return DecodeNeighborhoodReadRequest(payload)
	case ActionRecall:
		return DecodeRecallReadRequest(payload)
	default:
		message := fmt.Sprintf(
			"must equal one of %q, %q, %q, %q, or %q",
			ActionValidate,
			ActionAdmit,
			ActionResolve,
			ActionNeighborhood,
			ActionRecall,
		)
		return nil, invalidContract("$.action", message)
	}
}

func DecodeAdmitRequest(payload []byte) (AdmitRequest, error) {
	if err := scanStrictJSON(payload); err != nil {
		return AdmitRequest{}, err
	}
	wire := admitRequestWire{}
	if err := decodeStrict(payload, &wire, "$", "admission request"); err != nil {
		return AdmitRequest{}, err
	}
	if err := requireIdentifier(
		wire.ContractVersion,
		"$.contract_version",
	); err != nil {
		return AdmitRequest{}, err
	}
	if !validValidateContractVersion(wire.ContractVersion) {
		message := fmt.Sprintf(
			"must equal %q or %q",
			ContractVersionV1,
			ContractVersionV2,
		)
		return AdmitRequest{}, invalidContract("$.contract_version", message)
	}
	if err := requireIdentifier(wire.Action, "$.action"); err != nil {
		return AdmitRequest{}, err
	}
	if wire.Action != ActionAdmit {
		message := fmt.Sprintf("must equal %q", ActionAdmit)
		return AdmitRequest{}, invalidContract("$.action", message)
	}
	if len(wire.Basis) == 0 {
		return AdmitRequest{}, invalidContract("$.basis", "basis is required")
	}
	basis, err := decodeBasis(wire.Basis)
	if err != nil {
		return AdmitRequest{}, err
	}
	exact, ok := basis.(ExactProjectSelector)
	if !ok {
		return AdmitRequest{}, invalidContract(
			"$.basis.kind",
			"generic admission requires exact_project",
		)
	}
	if err := requireIdentifier(
		wire.AuthorityClass,
		"$.authority_class",
	); err != nil {
		return AdmitRequest{}, err
	}
	if wire.AuthorityClass != AuthorityClassNonBindingSemanticAssertion {
		message := fmt.Sprintf(
			"must equal %q",
			AuthorityClassNonBindingSemanticAssertion,
		)
		return AdmitRequest{}, invalidContract("$.authority_class", message)
	}
	idempotencyKey, err := decodeAdmissionIdempotencyKey(wire.IdempotencyKey)
	if err != nil {
		return AdmitRequest{}, err
	}
	provenance, err := parseProvenance(
		wire.RequestProvenanceRef,
		"$.request_provenance_ref",
	)
	if err != nil {
		return AdmitRequest{}, err
	}
	if len(wire.ChangeSet) == 0 {
		return AdmitRequest{}, invalidContract(
			"$.change_set",
			"change_set is required",
		)
	}
	changeSet, err := decodeChangeSet(wire.ChangeSet, wire.ContractVersion)
	if err != nil {
		return AdmitRequest{}, err
	}
	validation := ValidateRequest{
		proof:           decodedValidateRequestProof,
		contractVersion: wire.ContractVersion,
		basis:           exact,
		changeSet:       changeSet,
	}
	return AdmitRequest{
		proof:             decodedAdmitRequestProof,
		validation:        validation,
		idempotencyKey:    idempotencyKey,
		requestProvenance: provenance,
	}, nil
}

func decodeAdmissionIdempotencyKey(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" || raw != strings.TrimSpace(raw) {
		return "", invalidContract(
			"$.idempotency_key",
			"must be non-empty canonical text",
		)
	}
	if len(raw) > MaximumAdmissionIdempotencyKeyBytes {
		return "", resourceLimit(
			"$.idempotency_key",
			fmt.Sprintf(
				"idempotency key exceeds %d bytes",
				MaximumAdmissionIdempotencyKeyBytes,
			),
		)
	}
	return raw, nil
}
