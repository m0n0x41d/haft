package typedmemoryvalidation

import (
	"encoding/json"

	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

type PersistenceMode string

const PersistenceValidationOnlyNoWrite PersistenceMode = "validation_only_no_write"

type PersistenceDisposition struct{}

func (PersistenceDisposition) Mode() PersistenceMode {
	return PersistenceValidationOnlyNoWrite
}

func (PersistenceDisposition) RowsWritten() uint64 { return 0 }

func (PersistenceDisposition) AuthorityGranted() bool { return false }

func (disposition PersistenceDisposition) MarshalJSON() ([]byte, error) {
	payload := struct {
		Mode             PersistenceMode `json:"mode"`
		RowsWritten      uint64          `json:"rows_written"`
		AuthorityGranted bool            `json:"authority_granted"`
	}{
		Mode:             disposition.Mode(),
		RowsWritten:      disposition.RowsWritten(),
		AuthorityGranted: disposition.AuthorityGranted(),
	}
	return json.Marshal(payload)
}

type BasisProjection struct {
	requestedKind          typedmemorywire.BasisKind
	resolutionKind         BasisResolutionKind
	typeEnvRef             string
	graphRevision          uint64
	hasGraphRevision       bool
	requestedTypeEnvDigest string
	requestedGraphRevision uint64
	hasExactRequest        bool
}

func (projection BasisProjection) RequestedKind() typedmemorywire.BasisKind {
	return projection.requestedKind
}

func (projection BasisProjection) ResolutionKind() BasisResolutionKind {
	return projection.resolutionKind
}

func (projection BasisProjection) TypeEnvRef() (string, bool) {
	return projection.typeEnvRef, projection.typeEnvRef != ""
}

func (projection BasisProjection) GraphRevision() (uint64, bool) {
	return projection.graphRevision, projection.hasGraphRevision
}

func (projection BasisProjection) RequestedTypeEnvDigest() (string, bool) {
	return projection.requestedTypeEnvDigest, projection.hasExactRequest
}

func (projection BasisProjection) RequestedGraphRevision() (uint64, bool) {
	return projection.requestedGraphRevision, projection.hasExactRequest
}

func (projection BasisProjection) MarshalJSON() ([]byte, error) {
	payload := struct {
		RequestedKind          typedmemorywire.BasisKind `json:"requested_kind"`
		ResolutionKind         BasisResolutionKind       `json:"resolution_kind"`
		TypeEnvRef             string                    `json:"type_env_ref,omitempty"`
		GraphRevision          *uint64                   `json:"graph_revision,omitempty"`
		RequestedTypeEnvDigest string                    `json:"requested_type_env_digest,omitempty"`
		RequestedGraphRevision *uint64                   `json:"requested_graph_revision,omitempty"`
	}{
		RequestedKind:          projection.requestedKind,
		ResolutionKind:         projection.resolutionKind,
		TypeEnvRef:             projection.typeEnvRef,
		RequestedTypeEnvDigest: projection.requestedTypeEnvDigest,
	}
	if projection.hasGraphRevision {
		value := projection.graphRevision
		payload.GraphRevision = &value
	}
	if projection.hasExactRequest {
		value := projection.requestedGraphRevision
		payload.RequestedGraphRevision = &value
	}
	return json.Marshal(payload)
}

type Response interface {
	ContractVersion() string
	Action() string
	Verdict() typedmemory.ValidationVerdictKind
	Basis() BasisProjection
	PersistenceDisposition() PersistenceDisposition
	Diagnostics() []DiagnosticProjection
	responseVariant()
}

type ValidResponse interface {
	Response
	// NormalizedDigest is the admitted semantic change-set digest. Validation
	// never projects the request digest, AdmissionBatch, basis bytes, or
	// admission-envelope identity.
	NormalizedDigest() typedmemory.SHA256Digest
	validResponseVariant()
}

type InvalidResponse interface {
	Response
	invalidResponseVariant()
}

type UnderdeterminedResponse interface {
	Response
	underdeterminedResponseVariant()
}

type responseBase struct {
	contractVersion string
	verdict         typedmemory.ValidationVerdictKind
	basis           BasisProjection
	diagnostics     []DiagnosticProjection
}

func (response responseBase) ContractVersion() string { return response.contractVersion }

func (response responseBase) Action() string { return typedmemorywire.ActionValidate }

func (response responseBase) Verdict() typedmemory.ValidationVerdictKind { return response.verdict }

func (response responseBase) Basis() BasisProjection { return response.basis }

func (response responseBase) PersistenceDisposition() PersistenceDisposition {
	return PersistenceDisposition{}
}

func (response responseBase) Diagnostics() []DiagnosticProjection {
	return copyDiagnosticProjections(response.diagnostics)
}

type validResponse struct {
	responseBase
	digest typedmemory.SHA256Digest
}

func (response validResponse) NormalizedDigest() typedmemory.SHA256Digest {
	return response.digest
}

func (validResponse) responseVariant() {}

func (validResponse) validResponseVariant() {}

func (response validResponse) MarshalJSON() ([]byte, error) {
	return marshalResponse(response.responseBase, response.digest.String())
}

type invalidResponse struct{ responseBase }

func (invalidResponse) responseVariant() {}

func (invalidResponse) invalidResponseVariant() {}

func (response invalidResponse) MarshalJSON() ([]byte, error) {
	return marshalResponse(response.responseBase, "")
}

type underdeterminedResponse struct{ responseBase }

func (underdeterminedResponse) responseVariant() {}

func (underdeterminedResponse) underdeterminedResponseVariant() {}

func (response underdeterminedResponse) MarshalJSON() ([]byte, error) {
	return marshalResponse(response.responseBase, "")
}

func marshalResponse(base responseBase, digest string) ([]byte, error) {
	payload := struct {
		ContractVersion        string                            `json:"contract_version"`
		Action                 string                            `json:"action"`
		Verdict                typedmemory.ValidationVerdictKind `json:"verdict"`
		Basis                  BasisProjection                   `json:"basis"`
		PersistenceDisposition PersistenceDisposition            `json:"persistence_disposition"`
		Diagnostics            []DiagnosticProjection            `json:"diagnostics"`
		NormalizedDigest       string                            `json:"normalized_digest,omitempty"`
	}{
		ContractVersion:        base.contractVersion,
		Action:                 typedmemorywire.ActionValidate,
		Verdict:                base.verdict,
		Basis:                  base.basis,
		PersistenceDisposition: PersistenceDisposition{},
		Diagnostics:            copyDiagnosticProjections(base.diagnostics),
		NormalizedDigest:       digest,
	}
	return json.Marshal(payload)
}
