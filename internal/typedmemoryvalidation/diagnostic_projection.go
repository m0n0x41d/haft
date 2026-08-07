package typedmemoryvalidation

import (
	"encoding/json"
	"fmt"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

type DiagnosticWitnessKind string

const (
	DiagnosticWitnessExpectedActual DiagnosticWitnessKind = "expected_actual"
	DiagnosticWitnessMissingBasis   DiagnosticWitnessKind = "missing_basis"
)

type DiagnosticDatumProjection struct {
	kind      typedmemory.DiagnosticDatumKind
	scalar    string
	count     uint64
	hasCount  bool
	setValues []string
}

func (projection DiagnosticDatumProjection) Kind() typedmemory.DiagnosticDatumKind {
	return projection.kind
}

func (projection DiagnosticDatumProjection) Scalar() (string, bool) {
	switch projection.kind {
	case typedmemory.DiagnosticDatumText,
		typedmemory.DiagnosticDatumReference,
		typedmemory.DiagnosticDatumState,
		typedmemory.DiagnosticDatumUnknown:
		return projection.scalar, projection.scalar != ""
	default:
		return "", false
	}
}

func (projection DiagnosticDatumProjection) Count() (uint64, bool) {
	return projection.count, projection.kind == typedmemory.DiagnosticDatumCount && projection.hasCount
}

func (projection DiagnosticDatumProjection) SetValues() ([]string, bool) {
	if projection.kind != typedmemory.DiagnosticDatumSet || len(projection.setValues) == 0 {
		return nil, false
	}
	return append([]string(nil), projection.setValues...), true
}

func (projection DiagnosticDatumProjection) MarshalJSON() ([]byte, error) {
	payload := struct {
		Kind      typedmemory.DiagnosticDatumKind `json:"kind"`
		Scalar    string                          `json:"scalar,omitempty"`
		Count     *uint64                         `json:"count,omitempty"`
		SetValues []string                        `json:"set_values,omitempty"`
	}{
		Kind: projection.kind,
	}
	switch projection.kind {
	case typedmemory.DiagnosticDatumText,
		typedmemory.DiagnosticDatumReference,
		typedmemory.DiagnosticDatumState,
		typedmemory.DiagnosticDatumUnknown:
		if projection.scalar == "" {
			return nil, fmt.Errorf("diagnostic datum %q requires scalar", projection.kind)
		}
		payload.Scalar = projection.scalar
	case typedmemory.DiagnosticDatumCount:
		if !projection.hasCount {
			return nil, fmt.Errorf("diagnostic count datum is absent")
		}
		value := projection.count
		payload.Count = &value
	case typedmemory.DiagnosticDatumSet:
		if len(projection.setValues) == 0 {
			return nil, fmt.Errorf("diagnostic set datum is empty")
		}
		payload.SetValues = append([]string(nil), projection.setValues...)
	default:
		return nil, fmt.Errorf("unknown diagnostic datum kind %q", projection.kind)
	}
	return json.Marshal(payload)
}

type DiagnosticWitnessProjection struct {
	kind     DiagnosticWitnessKind
	expected DiagnosticDatumProjection
	actual   DiagnosticDatumProjection
}

func (projection DiagnosticWitnessProjection) Kind() DiagnosticWitnessKind {
	return projection.kind
}

func (projection DiagnosticWitnessProjection) Expected() DiagnosticDatumProjection {
	return copyDatumProjection(projection.expected)
}

func (projection DiagnosticWitnessProjection) Actual() DiagnosticDatumProjection {
	return copyDatumProjection(projection.actual)
}

func (projection DiagnosticWitnessProjection) MarshalJSON() ([]byte, error) {
	payload := struct {
		Kind     DiagnosticWitnessKind     `json:"kind"`
		Expected DiagnosticDatumProjection `json:"expected"`
		Actual   DiagnosticDatumProjection `json:"actual"`
	}{
		Kind:     projection.kind,
		Expected: projection.expected,
		Actual:   projection.actual,
	}
	return json.Marshal(payload)
}

type SourceLocationProjection struct {
	unitID      string
	revision    string
	contentHash string
	startLine   uint64
	endLine     uint64
	patternID   string
}

func (projection SourceLocationProjection) MarshalJSON() ([]byte, error) {
	payload := struct {
		UnitID      string `json:"unit_id"`
		Revision    string `json:"revision"`
		ContentHash string `json:"content_hash"`
		StartLine   uint64 `json:"start_line"`
		EndLine     uint64 `json:"end_line"`
		PatternID   string `json:"pattern_id,omitempty"`
	}{
		UnitID:      projection.unitID,
		Revision:    projection.revision,
		ContentHash: projection.contentHash,
		StartLine:   projection.startLine,
		EndLine:     projection.endLine,
		PatternID:   projection.patternID,
	}
	return json.Marshal(payload)
}

type CoverageProjection struct {
	subject   string
	posture   typedmemory.CoveragePosture
	source    SourceLocationProjection
	rationale string
}

func (projection CoverageProjection) MarshalJSON() ([]byte, error) {
	payload := struct {
		Subject   string                      `json:"subject"`
		Posture   typedmemory.CoveragePosture `json:"posture"`
		Source    SourceLocationProjection    `json:"source"`
		Rationale string                      `json:"rationale,omitempty"`
	}{
		Subject:   projection.subject,
		Posture:   projection.posture,
		Source:    projection.source,
		Rationale: projection.rationale,
	}
	return json.Marshal(payload)
}

type DeclarationProvenanceKind string

const (
	DeclarationProvenanceFPFSource       DeclarationProvenanceKind = "fpf_source"
	DeclarationProvenanceCompilerDerived DeclarationProvenanceKind = "compiler_derived"
	DeclarationProvenanceProjectSource   DeclarationProvenanceKind = "project_source"
)

type DeclarationProvenanceProjection struct {
	kind              DeclarationProvenanceKind
	reference         string
	compilerRuleID    string
	sources           []SourceLocationProjection
	carrier           string
	edition           string
	contentHash       string
	startLine         uint64
	endLine           uint64
	boundedContext    string
	baseTypeEnv       string
	signatureBlockRow string
	manifestRef       string
	manifestDirection string
	manifestSymbol    string
}

func (projection DeclarationProvenanceProjection) MarshalJSON() ([]byte, error) {
	payload := struct {
		Kind              DeclarationProvenanceKind  `json:"kind"`
		Reference         string                     `json:"reference"`
		CompilerRuleID    string                     `json:"compiler_rule_id"`
		Sources           []SourceLocationProjection `json:"sources,omitempty"`
		Carrier           string                     `json:"carrier,omitempty"`
		Edition           string                     `json:"edition,omitempty"`
		ContentHash       string                     `json:"content_hash,omitempty"`
		StartLine         uint64                     `json:"start_line,omitempty"`
		EndLine           uint64                     `json:"end_line,omitempty"`
		BoundedContext    string                     `json:"bounded_context,omitempty"`
		BaseTypeEnv       string                     `json:"base_type_env,omitempty"`
		SignatureBlockRow string                     `json:"signature_block_row,omitempty"`
		ManifestRef       string                     `json:"manifest_ref,omitempty"`
		ManifestDirection string                     `json:"manifest_direction,omitempty"`
		ManifestSymbol    string                     `json:"manifest_symbol,omitempty"`
	}{
		Kind:              projection.kind,
		Reference:         projection.reference,
		CompilerRuleID:    projection.compilerRuleID,
		Sources:           append([]SourceLocationProjection(nil), projection.sources...),
		Carrier:           projection.carrier,
		Edition:           projection.edition,
		ContentHash:       projection.contentHash,
		StartLine:         projection.startLine,
		EndLine:           projection.endLine,
		BoundedContext:    projection.boundedContext,
		BaseTypeEnv:       projection.baseTypeEnv,
		SignatureBlockRow: projection.signatureBlockRow,
		ManifestRef:       projection.manifestRef,
		ManifestDirection: projection.manifestDirection,
		ManifestSymbol:    projection.manifestSymbol,
	}
	return json.Marshal(payload)
}

type GoverningBasisProjection struct {
	kind               typedmemory.DiagnosticBasisKind
	provenance         *DeclarationProvenanceProjection
	rule               string
	typeEnvRef         string
	subject            *DiagnosticDatumProjection
	coverage           *CoverageProjection
	missingRuntimeKind typedmemory.MissingRuntimeBasisKind
	required           *DiagnosticDatumProjection
}

func (projection GoverningBasisProjection) Kind() typedmemory.DiagnosticBasisKind {
	return projection.kind
}

func (projection GoverningBasisProjection) MarshalJSON() ([]byte, error) {
	payload := struct {
		Kind               typedmemory.DiagnosticBasisKind     `json:"kind"`
		Provenance         *DeclarationProvenanceProjection    `json:"provenance,omitempty"`
		Rule               string                              `json:"rule,omitempty"`
		TypeEnvRef         string                              `json:"type_env_ref,omitempty"`
		Subject            *DiagnosticDatumProjection          `json:"subject,omitempty"`
		Coverage           *CoverageProjection                 `json:"coverage,omitempty"`
		MissingRuntimeKind typedmemory.MissingRuntimeBasisKind `json:"missing_runtime_kind,omitempty"`
		Required           *DiagnosticDatumProjection          `json:"required,omitempty"`
	}{
		Kind:               projection.kind,
		Provenance:         projection.provenance,
		Rule:               projection.rule,
		TypeEnvRef:         projection.typeEnvRef,
		Subject:            projection.subject,
		Coverage:           projection.coverage,
		MissingRuntimeKind: projection.missingRuntimeKind,
		Required:           projection.required,
	}
	return json.Marshal(payload)
}

type RepairProjection struct {
	kind        typedmemory.RepairKind
	pointer     string
	target      DiagnosticDatumProjection
	humanChoice typedmemory.HumanChoiceRequirement
}

func (projection RepairProjection) Kind() typedmemory.RepairKind { return projection.kind }

func (projection RepairProjection) Pointer() string { return projection.pointer }

func (projection RepairProjection) Target() DiagnosticDatumProjection {
	return copyDatumProjection(projection.target)
}

func (projection RepairProjection) HumanChoiceRequirement() typedmemory.HumanChoiceRequirement {
	return projection.humanChoice
}

func (projection RepairProjection) MarshalJSON() ([]byte, error) {
	payload := struct {
		Kind        typedmemory.RepairKind             `json:"kind"`
		Pointer     string                             `json:"pointer"`
		Target      DiagnosticDatumProjection          `json:"target"`
		HumanChoice typedmemory.HumanChoiceRequirement `json:"human_choice_requirement"`
	}{
		Kind:        projection.kind,
		Pointer:     projection.pointer,
		Target:      projection.target,
		HumanChoice: projection.humanChoice,
	}
	return json.Marshal(payload)
}

type DiagnosticProjection struct {
	posture  typedmemory.DiagnosticPosture
	code     string
	message  string
	path     string
	pathKind DiagnosticPathKind
	witness  DiagnosticWitnessProjection
	basis    GoverningBasisProjection
	repairs  []RepairProjection
}

func (projection DiagnosticProjection) Posture() typedmemory.DiagnosticPosture {
	return projection.posture
}

func (projection DiagnosticProjection) Code() string { return projection.code }

func (projection DiagnosticProjection) Message() string { return projection.message }

func (projection DiagnosticProjection) Path() string { return projection.path }

// PathKind distinguishes an exact coordinate in the strict request from a
// typed semantic coordinate that has no input location. JSON compatibility is
// preserved by keeping the existing string-valued path field; semantic paths
// are also visibly prefixed with "typed-memory-semantic:".
func (projection DiagnosticProjection) PathKind() DiagnosticPathKind {
	return projection.pathKind
}

func (projection DiagnosticProjection) Witness() DiagnosticWitnessProjection {
	return copyWitnessProjection(projection.witness)
}

func (projection DiagnosticProjection) GoverningBasis() GoverningBasisProjection {
	return copyGoverningBasisProjection(projection.basis)
}

func (projection DiagnosticProjection) RepairCandidates() []RepairProjection {
	return copyRepairProjections(projection.repairs)
}

func (projection DiagnosticProjection) MarshalJSON() ([]byte, error) {
	payload := struct {
		Posture typedmemory.DiagnosticPosture `json:"posture"`
		Code    string                        `json:"code"`
		Message string                        `json:"message"`
		Path    string                        `json:"path"`
		Witness DiagnosticWitnessProjection   `json:"witness"`
		Basis   GoverningBasisProjection      `json:"governing_basis"`
		Repairs []RepairProjection            `json:"repair_candidates"`
	}{
		Posture: projection.posture,
		Code:    projection.code,
		Message: projection.message,
		Path:    projection.path,
		Witness: projection.witness,
		Basis:   projection.basis,
		Repairs: copyRepairProjections(projection.repairs),
	}
	return json.Marshal(payload)
}

func copyDatumProjection(value DiagnosticDatumProjection) DiagnosticDatumProjection {
	return DiagnosticDatumProjection{
		kind:      value.kind,
		scalar:    value.scalar,
		count:     value.count,
		hasCount:  value.hasCount,
		setValues: append([]string(nil), value.setValues...),
	}
}

func copyWitnessProjection(value DiagnosticWitnessProjection) DiagnosticWitnessProjection {
	return DiagnosticWitnessProjection{
		kind:     value.kind,
		expected: copyDatumProjection(value.expected),
		actual:   copyDatumProjection(value.actual),
	}
}

func copyGoverningBasisProjection(value GoverningBasisProjection) GoverningBasisProjection {
	copyValue := value
	if value.subject != nil {
		subject := copyDatumProjection(*value.subject)
		copyValue.subject = &subject
	}
	if value.required != nil {
		required := copyDatumProjection(*value.required)
		copyValue.required = &required
	}
	if value.provenance != nil {
		provenance := *value.provenance
		provenance.sources = append([]SourceLocationProjection(nil), value.provenance.sources...)
		copyValue.provenance = &provenance
	}
	if value.coverage != nil {
		coverage := *value.coverage
		copyValue.coverage = &coverage
	}
	return copyValue
}

func copyRepairProjections(values []RepairProjection) []RepairProjection {
	result := make([]RepairProjection, 0, len(values))
	for _, value := range values {
		copyValue := value
		copyValue.target = copyDatumProjection(value.target)
		result = append(result, copyValue)
	}
	return result
}
