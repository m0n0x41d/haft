package typedmemoryvalidation

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

// Outcome is the sealed, effect-free result of validating one candidate
// against one resolved basis. Presentation adapters may project it to Response;
// persistence adapters may consume only ValidOutcome's opaque AdmissionBatch.
type Outcome interface {
	ContractVersion() string
	Verdict() typedmemory.ValidationVerdictKind
	BasisProjection() BasisProjection
	Diagnostics() []DiagnosticProjection
	outcomeVariant()
}

// ValidOutcome is the only outcome variant that carries an admission
// capability. Candidate and AdmissionBasis retain the exact request and basis
// correlated by AdmissionBatch; neither grants an effect on its own.
type ValidOutcome interface {
	Outcome
	Candidate() typedmemory.MemoryChangeSet
	AdmissionBatch() typedmemory.AdmissionBatch
	AdmissionBasis() typedmemory.AdmissionBasis
	SemanticChangeDigest() typedmemory.SHA256Digest
	validOutcomeVariant()
}

type InvalidOutcome interface {
	Outcome
	invalidOutcomeVariant()
}

type UnderdeterminedOutcome interface {
	Outcome
	underdeterminedOutcomeVariant()
}

type outcomeBase struct {
	contractVersion string
	verdict         typedmemory.ValidationVerdictKind
	basis           BasisProjection
	diagnostics     []DiagnosticProjection
}

func (outcome outcomeBase) ContractVersion() string {
	return outcome.contractVersion
}

func (outcome outcomeBase) Verdict() typedmemory.ValidationVerdictKind {
	return outcome.verdict
}

func (outcome outcomeBase) BasisProjection() BasisProjection { return outcome.basis }

func (outcome outcomeBase) Diagnostics() []DiagnosticProjection {
	return copyDiagnosticProjections(outcome.diagnostics)
}

type validOutcome struct {
	outcomeBase
	candidate typedmemory.MemoryChangeSet
	batch     typedmemory.AdmissionBatch
}

func (outcome validOutcome) Candidate() typedmemory.MemoryChangeSet {
	// MemoryChangeSet has no mutating surface and defensively copies Changes(),
	// so returning the value preserves the exact bound candidate without
	// exposing its backing slice.
	return outcome.candidate
}

func (outcome validOutcome) AdmissionBatch() typedmemory.AdmissionBatch {
	return outcome.batch
}

func (outcome validOutcome) AdmissionBasis() typedmemory.AdmissionBasis {
	return outcome.batch.Basis()
}

func (outcome validOutcome) SemanticChangeDigest() typedmemory.SHA256Digest {
	return outcome.batch.SemanticChangeDigest()
}

func (validOutcome) outcomeVariant() {}

func (validOutcome) validOutcomeVariant() {}

type invalidOutcome struct{ outcomeBase }

func (invalidOutcome) outcomeVariant() {}

func (invalidOutcome) invalidOutcomeVariant() {}

type underdeterminedOutcome struct{ outcomeBase }

func (underdeterminedOutcome) outcomeVariant() {}

func (underdeterminedOutcome) underdeterminedOutcomeVariant() {}

func newValidOutcome(
	contractVersion string,
	basis BasisProjection,
	candidate typedmemory.MemoryChangeSet,
	verdict typedmemory.Valid,
) (ValidOutcome, error) {
	if verdict == nil {
		return nil, fmt.Errorf("valid outcome requires a core Valid verdict")
	}
	batch := verdict.AdmissionBatch()
	if !batch.IsValid() {
		return nil, fmt.Errorf("valid outcome requires a self-validating AdmissionBatch")
	}
	candidateDigest, err := candidate.Digest()
	if err != nil {
		return nil, fmt.Errorf("valid outcome candidate digest: %w", err)
	}
	if candidateDigest != batch.RequestDigest() {
		return nil, fmt.Errorf("valid outcome candidate does not match AdmissionBatch request digest")
	}
	if verdict.SemanticChangeDigest() != batch.SemanticChangeDigest() {
		return nil, fmt.Errorf("valid outcome semantic digest does not match AdmissionBatch")
	}
	admissionBasis := batch.Basis()
	if admissionBasis == nil {
		return nil, fmt.Errorf("valid outcome AdmissionBatch has no exact basis")
	}
	projectedTypeEnv, hasTypeEnv := basis.TypeEnvRef()
	projectedRevision, hasRevision := basis.GraphRevision()
	admissionTypeEnv := admissionBasis.TypeEnv()
	admissionRevision := admissionBasis.GraphRevision()
	basisMatchesProjection := hasTypeEnv &&
		hasRevision &&
		admissionTypeEnv.String() == projectedTypeEnv &&
		admissionRevision.Value() == projectedRevision
	if !basisMatchesProjection {
		return nil, fmt.Errorf("valid outcome AdmissionBatch does not match the resolved basis")
	}
	base := outcomeBase{
		contractVersion: contractVersion,
		verdict:         typedmemory.ValidationValid,
		basis:           basis,
	}
	return validOutcome{
		outcomeBase: base,
		candidate:   candidate,
		batch:       batch,
	}, nil
}

func newInvalidOutcome(
	contractVersion string,
	basis BasisProjection,
	diagnostics []DiagnosticProjection,
) Outcome {
	retainedDiagnostics := copyDiagnosticProjections(diagnostics)
	base := outcomeBase{
		contractVersion: contractVersion,
		verdict:         typedmemory.ValidationInvalid,
		basis:           basis,
		diagnostics:     retainedDiagnostics,
	}
	return invalidOutcome{outcomeBase: base}
}

func newUnderdeterminedOutcome(
	contractVersion string,
	basis BasisProjection,
	diagnostics []DiagnosticProjection,
) Outcome {
	retainedDiagnostics := copyDiagnosticProjections(diagnostics)
	base := outcomeBase{
		contractVersion: contractVersion,
		verdict:         typedmemory.ValidationUnderdetermined,
		basis:           basis,
		diagnostics:     retainedDiagnostics,
	}
	return underdeterminedOutcome{outcomeBase: base}
}

func presentOutcome(outcome Outcome) Response {
	switch result := outcome.(type) {
	case ValidOutcome:
		contractVersion := result.ContractVersion()
		basis := result.BasisProjection()
		digest := result.SemanticChangeDigest()
		return newValidResponse(contractVersion, basis, digest)
	case InvalidOutcome:
		contractVersion := result.ContractVersion()
		basis := result.BasisProjection()
		diagnostics := result.Diagnostics()
		return newInvalidResponse(contractVersion, basis, diagnostics)
	case UnderdeterminedOutcome:
		contractVersion := result.ContractVersion()
		basis := result.BasisProjection()
		diagnostics := result.Diagnostics()
		return newUnderdeterminedResponse(contractVersion, basis, diagnostics)
	default:
		diagnostic := newInvalidDiagnostic(
			DiagnosticMalformedValidationRequest,
			"a sealed typed-memory validation outcome is required",
			"$",
			"sealed-validation-outcome",
			"missing-or-invalid-outcome",
			validationBindRule,
		)
		basis := BasisProjection{}
		diagnostics := []DiagnosticProjection{diagnostic}
		return newInvalidResponse(
			typedmemorywire.ContractVersionV1,
			basis,
			diagnostics,
		)
	}
}
