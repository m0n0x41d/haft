package typedmemorywire

import (
	"bytes"
	"encoding/json"
	"fmt"
)

const QueryActionMemory = "memory"

type queryReadMCPEnvelopeWire struct {
	Action        string          `json:"action"`
	MemoryRequest json.RawMessage `json:"memory_request"`
}

type queryReadDiscriminatorWire struct {
	Action string `json:"action"`
	Mode   string `json:"mode"`
}

type queryResolveReadRequestWire struct {
	ContractVersion string          `json:"contract_version"`
	Action          string          `json:"action"`
	Mode            string          `json:"mode"`
	Basis           json.RawMessage `json:"basis"`
	Query           string          `json:"query"`
	BoundedContext  *string         `json:"bounded_context_ref,omitempty"`
	MaxCandidates   *uint32         `json:"max_candidates"`
}

type queryNeighborhoodReadRequestWire struct {
	ContractVersion string          `json:"contract_version"`
	Action          string          `json:"action"`
	Mode            string          `json:"mode"`
	Basis           json.RawMessage `json:"basis"`
	Entity          json.RawMessage `json:"entity_ref"`
	Context         string          `json:"bounded_context_ref"`
	View            json.RawMessage `json:"view"`
	ReadBudget      json.RawMessage `json:"read_budget"`
}

type queryRecallReadRequestWire struct {
	ContractVersion string          `json:"contract_version"`
	Action          string          `json:"action"`
	Mode            string          `json:"mode"`
	Basis           json.RawMessage `json:"basis"`
	Entity          json.RawMessage `json:"entity_ref"`
	Context         string          `json:"bounded_context_ref"`
	View            json.RawMessage `json:"view"`
	ReadBudget      json.RawMessage `json:"read_budget"`
	Query           string          `json:"query"`
	CandidateBudget json.RawMessage `json:"candidate_budget"`
}

// DecodeQueryReadRequest translates the public
// haft_query(action="memory", memory_request={...}) envelope into the existing
// sealed flat project-memory read union. Both envelope and nested request bytes
// are scanned before any map decode, so duplicate, unknown, legacy-flat, and
// cross-variant fields remain observable and fail closed.
func DecodeQueryReadRequest(payload []byte) (Request, error) {
	if err := scanStrictJSON(payload); err != nil {
		return nil, err
	}
	envelope := queryReadMCPEnvelopeWire{}
	if err := decodeStrict(
		payload,
		&envelope,
		"$",
		"haft_query memory envelope",
	); err != nil {
		return nil, err
	}
	if envelope.Action != QueryActionMemory {
		return nil, invalidContract(
			"$.action",
			fmt.Sprintf("must equal %q", QueryActionMemory),
		)
	}
	flat, err := flattenQueryReadMCPEnvelope(envelope.MemoryRequest)
	if err != nil {
		return nil, err
	}
	return decodeFlatQueryReadRequest(flat)
}

func flattenQueryReadMCPEnvelope(
	request json.RawMessage,
) ([]byte, error) {
	trimmed := bytes.TrimSpace(request)
	if len(trimmed) < 2 ||
		trimmed[0] != '{' ||
		trimmed[len(trimmed)-1] != '}' {
		return nil, invalidContract(
			"$.memory_request",
			"memory_request must be a JSON object",
		)
	}
	body := bytes.TrimSpace(trimmed[1 : len(trimmed)-1])
	result := make([]byte, 0, len(body)+24)
	result = append(result, `{"action":"memory"`...)
	if len(body) > 0 {
		result = append(result, ',')
		result = append(result, body...)
	}
	result = append(result, '}')
	return result, nil
}

func decodeFlatQueryReadRequest(payload []byte) (Request, error) {
	if err := scanStrictJSON(payload); err != nil {
		return nil, err
	}
	discriminator := queryReadDiscriminatorWire{}
	if err := json.Unmarshal(payload, &discriminator); err != nil {
		return nil, invalidContract("$", "query request must be a JSON object")
	}
	if err := requireQueryReadHeader(
		discriminator.Action,
		discriminator.Mode,
	); err != nil {
		return nil, err
	}
	switch discriminator.Mode {
	case ActionResolve:
		return decodeQueryResolveReadRequest(payload)
	case ActionNeighborhood:
		return decodeQueryNeighborhoodReadRequest(payload)
	case ActionRecall:
		return decodeQueryRecallReadRequest(payload)
	default:
		return nil, invalidContract(
			"$.mode",
			fmt.Sprintf(
				"must equal one of %q, %q, or %q",
				ActionResolve,
				ActionNeighborhood,
				ActionRecall,
			),
		)
	}
}

func decodeQueryResolveReadRequest(payload []byte) (Request, error) {
	wire := queryResolveReadRequestWire{}
	if err := decodeStrict(
		payload,
		&wire,
		"$",
		"EntityOfConcern resolution query",
	); err != nil {
		return nil, err
	}
	if err := requireQueryReadHeader(wire.Action, wire.Mode); err != nil {
		return nil, err
	}
	translated := resolveReadRequestWire{
		ContractVersion: wire.ContractVersion,
		Action:          ActionResolve,
		Basis:           wire.Basis,
		Query:           wire.Query,
		BoundedContext:  wire.BoundedContext,
		MaxCandidates:   wire.MaxCandidates,
	}
	return decodeTranslatedResolveReadRequest(translated)
}

func decodeQueryNeighborhoodReadRequest(payload []byte) (Request, error) {
	wire := queryNeighborhoodReadRequestWire{}
	if err := decodeStrict(
		payload,
		&wire,
		"$",
		"EntityOfConcern neighborhood query",
	); err != nil {
		return nil, err
	}
	if err := requireQueryReadHeader(wire.Action, wire.Mode); err != nil {
		return nil, err
	}
	translated := neighborhoodReadRequestWire{
		ContractVersion: wire.ContractVersion,
		Action:          ActionNeighborhood,
		Basis:           wire.Basis,
		Entity:          wire.Entity,
		Context:         wire.Context,
		View:            wire.View,
		ReadBudget:      wire.ReadBudget,
	}
	return decodeTranslatedNeighborhoodReadRequest(translated)
}

func decodeQueryRecallReadRequest(payload []byte) (Request, error) {
	wire := queryRecallReadRequestWire{}
	if err := decodeStrict(
		payload,
		&wire,
		"$",
		"EntityOfConcern recall query",
	); err != nil {
		return nil, err
	}
	if err := requireQueryReadHeader(wire.Action, wire.Mode); err != nil {
		return nil, err
	}
	translated := recallReadRequestWire{
		ContractVersion: wire.ContractVersion,
		Action:          ActionRecall,
		Basis:           wire.Basis,
		Entity:          wire.Entity,
		Context:         wire.Context,
		View:            wire.View,
		ReadBudget:      wire.ReadBudget,
		Query:           wire.Query,
		CandidateBudget: wire.CandidateBudget,
	}
	return decodeTranslatedRecallReadRequest(translated)
}

func decodeTranslatedResolveReadRequest(
	wire resolveReadRequestWire,
) (Request, error) {
	payload, err := json.Marshal(wire)
	if err != nil {
		return nil, fmt.Errorf("encode translated resolve request: %w", err)
	}
	return DecodeResolveReadRequest(payload)
}

func decodeTranslatedNeighborhoodReadRequest(
	wire neighborhoodReadRequestWire,
) (Request, error) {
	payload, err := json.Marshal(wire)
	if err != nil {
		return nil, fmt.Errorf(
			"encode translated neighborhood request: %w",
			err,
		)
	}
	return DecodeNeighborhoodReadRequest(payload)
}

func decodeTranslatedRecallReadRequest(
	wire recallReadRequestWire,
) (Request, error) {
	payload, err := json.Marshal(wire)
	if err != nil {
		return nil, fmt.Errorf("encode translated recall request: %w", err)
	}
	return DecodeRecallReadRequest(payload)
}

func requireQueryReadHeader(action string, mode string) error {
	if err := requireIdentifier(action, "$.action"); err != nil {
		return err
	}
	if action != QueryActionMemory {
		return invalidContract(
			"$.action",
			fmt.Sprintf("must equal %q", QueryActionMemory),
		)
	}
	if err := requireIdentifier(mode, "$.mode"); err != nil {
		return err
	}
	return nil
}
