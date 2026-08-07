package typedmemory

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type DiagnosticCode string

const (
	DiagnosticSignatureNotActive        DiagnosticCode = "signature_not_active"
	DiagnosticKindUnavailableInContext  DiagnosticCode = "kind_unavailable_in_context"
	DiagnosticContextNotActive          DiagnosticCode = "context_not_active"
	DiagnosticContextBridgeMissing      DiagnosticCode = "context_bridge_missing"
	DiagnosticCodecUnavailable          DiagnosticCode = "codec_unavailable"
	DiagnosticValueBindingNotActive     DiagnosticCode = "value_binding_not_active"
	DiagnosticTypeRuleUnavailable       DiagnosticCode = "type_rule_unavailable"
	DiagnosticReferenceUnresolved       DiagnosticCode = "reference_unresolved"
	DiagnosticCompilerCoverageMissing   DiagnosticCode = "compiler_coverage_missing"
	DiagnosticUnknownSlot               DiagnosticCode = "unknown_slot"
	DiagnosticMissingSlot               DiagnosticCode = "missing_slot"
	DiagnosticDuplicateSlot             DiagnosticCode = "duplicate_slot"
	DiagnosticCardinalityMismatch       DiagnosticCode = "cardinality_mismatch"
	DiagnosticReferenceModeMismatch     DiagnosticCode = "reference_mode_mismatch"
	DiagnosticReferenceKindMismatch     DiagnosticCode = "reference_kind_mismatch"
	DiagnosticValueKindMismatch         DiagnosticCode = "value_kind_mismatch"
	DiagnosticValueShapeMismatch        DiagnosticCode = "value_shape_mismatch"
	DiagnosticCodecRefMismatch          DiagnosticCode = "codec_ref_mismatch"
	DiagnosticMalformedValue            DiagnosticCode = "malformed_value"
	DiagnosticTypedValueDigestMismatch  DiagnosticCode = "typed_value_digest_mismatch"
	DiagnosticClaimGraphDuplicateNode   DiagnosticCode = "claim_graph_duplicate_node"
	DiagnosticClaimGraphDanglingEdge    DiagnosticCode = "claim_graph_dangling_endpoint"
	DiagnosticAliasAmbiguous            DiagnosticCode = "alias_ambiguous"
	DiagnosticAliasAlreadyBound         DiagnosticCode = "alias_already_bound"
	DiagnosticAliasNotBound             DiagnosticCode = "alias_not_bound"
	DiagnosticIdentityBasisMissing      DiagnosticCode = "identity_basis_missing"
	DiagnosticEntityAlreadyExists       DiagnosticCode = "entity_already_exists"
	DiagnosticEntityKindMismatch        DiagnosticCode = "entity_kind_mismatch"
	DiagnosticSignatureContextMismatch  DiagnosticCode = "signature_context_mismatch"
	DiagnosticAssertionAlreadyExists    DiagnosticCode = "assertion_already_exists"
	DiagnosticAssertionAlreadyRetracted DiagnosticCode = "assertion_already_retracted"
	DiagnosticAssertionNotFound         DiagnosticCode = "assertion_not_found"
)

func (code DiagnosticCode) valid() bool {
	return strings.TrimSpace(string(code)) != ""
}

type DiagnosticPath struct {
	value string
}

func NewDiagnosticPath(raw string) (DiagnosticPath, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return DiagnosticPath{}, fmt.Errorf("diagnostic path is required")
	}
	return DiagnosticPath{value: value}, nil
}

func (path DiagnosticPath) String() string { return path.value }

func (path DiagnosticPath) valid() bool { return path.value != "" }

type DiagnosticPosture string

const (
	DiagnosticInvalid         DiagnosticPosture = "invalid"
	DiagnosticUnderdetermined DiagnosticPosture = "underdetermined"
)

// DiagnosticDatum is a closed, transport-neutral semantic atom. Values are
// retained by the validator instead of being recovered from human messages.
type DiagnosticDatumKind string

const (
	DiagnosticDatumText      DiagnosticDatumKind = "text"
	DiagnosticDatumReference DiagnosticDatumKind = "reference"
	DiagnosticDatumCount     DiagnosticDatumKind = "count"
	DiagnosticDatumSet       DiagnosticDatumKind = "set"
	DiagnosticDatumState     DiagnosticDatumKind = "state"
	DiagnosticDatumUnknown   DiagnosticDatumKind = "unknown"
)

func (kind DiagnosticDatumKind) valid() bool {
	switch kind {
	case DiagnosticDatumText,
		DiagnosticDatumReference,
		DiagnosticDatumCount,
		DiagnosticDatumSet,
		DiagnosticDatumState,
		DiagnosticDatumUnknown:
		return true
	default:
		return false
	}
}

type DiagnosticDatum struct {
	kind   DiagnosticDatumKind
	values []string
}

func NewDiagnosticTextDatum(value string) (DiagnosticDatum, error) {
	return newSingleDiagnosticDatum(DiagnosticDatumText, value)
}

func NewDiagnosticReferenceDatum(value string) (DiagnosticDatum, error) {
	return newSingleDiagnosticDatum(DiagnosticDatumReference, value)
}

func NewDiagnosticCountDatum(value uint64) DiagnosticDatum {
	return DiagnosticDatum{
		kind:   DiagnosticDatumCount,
		values: []string{strconv.FormatUint(value, 10)},
	}
}

func NewDiagnosticSetDatum(values []string) (DiagnosticDatum, error) {
	owned := append([]string(nil), values...)
	for index, value := range owned {
		owned[index] = strings.TrimSpace(value)
		if owned[index] == "" {
			return DiagnosticDatum{}, fmt.Errorf("diagnostic set value %d is empty", index)
		}
	}
	if len(owned) == 0 {
		return DiagnosticDatum{}, fmt.Errorf("diagnostic set requires at least one value")
	}
	sort.Strings(owned)
	for index := 1; index < len(owned); index++ {
		if owned[index] == owned[index-1] {
			return DiagnosticDatum{}, fmt.Errorf("duplicate diagnostic set value %q", owned[index])
		}
	}
	return DiagnosticDatum{kind: DiagnosticDatumSet, values: owned}, nil
}

func NewDiagnosticStateDatum(value string) (DiagnosticDatum, error) {
	return newSingleDiagnosticDatum(DiagnosticDatumState, value)
}

func NewUnknownDiagnosticDatum(reason string) (DiagnosticDatum, error) {
	return newSingleDiagnosticDatum(DiagnosticDatumUnknown, reason)
}

func newSingleDiagnosticDatum(
	kind DiagnosticDatumKind,
	value string,
) (DiagnosticDatum, error) {
	trimmed := strings.TrimSpace(value)
	if !kind.valid() || kind == DiagnosticDatumSet || kind == DiagnosticDatumCount {
		return DiagnosticDatum{}, fmt.Errorf("single diagnostic datum kind is invalid")
	}
	if trimmed == "" {
		return DiagnosticDatum{}, fmt.Errorf("diagnostic datum value is required")
	}
	return DiagnosticDatum{kind: kind, values: []string{trimmed}}, nil
}

func (datum DiagnosticDatum) Kind() DiagnosticDatumKind { return datum.kind }

func (datum DiagnosticDatum) Values() []string {
	return append([]string(nil), datum.values...)
}

func (datum DiagnosticDatum) Scalar() (string, bool) {
	if !datum.valid() || datum.kind == DiagnosticDatumCount || datum.kind == DiagnosticDatumSet {
		return "", false
	}
	return datum.values[0], true
}

func (datum DiagnosticDatum) Count() (uint64, bool) {
	if !datum.valid() || datum.kind != DiagnosticDatumCount {
		return 0, false
	}
	value, err := strconv.ParseUint(datum.values[0], 10, 64)
	return value, err == nil
}

func (datum DiagnosticDatum) SetValues() ([]string, bool) {
	if !datum.valid() || datum.kind != DiagnosticDatumSet {
		return nil, false
	}
	return append([]string(nil), datum.values...), true
}

func (datum DiagnosticDatum) valid() bool {
	if !datum.kind.valid() || len(datum.values) == 0 {
		return false
	}
	for _, value := range datum.values {
		if strings.TrimSpace(value) == "" {
			return false
		}
	}
	if datum.kind != DiagnosticDatumSet && len(datum.values) != 1 {
		return false
	}
	return true
}

// DiagnosticWitness is closed. Both variants project to expected/actual; a
// missing basis is represented as an explicit unknown observation, never by
// omitting or inventing a value.
type DiagnosticWitness interface {
	Expected() DiagnosticDatum
	Actual() DiagnosticDatum
	diagnosticWitnessVariant()
}

type ExpectedActualWitness struct {
	expected DiagnosticDatum
	actual   DiagnosticDatum
}

func NewExpectedActualWitness(
	expected DiagnosticDatum,
	actual DiagnosticDatum,
) (ExpectedActualWitness, error) {
	if !expected.valid() || !actual.valid() {
		return ExpectedActualWitness{}, fmt.Errorf("expected and actual diagnostic data are required")
	}
	return ExpectedActualWitness{
		expected: copyDiagnosticDatum(expected),
		actual:   copyDiagnosticDatum(actual),
	}, nil
}

func (witness ExpectedActualWitness) Expected() DiagnosticDatum {
	return copyDiagnosticDatum(witness.expected)
}

func (witness ExpectedActualWitness) Actual() DiagnosticDatum {
	return copyDiagnosticDatum(witness.actual)
}

func (ExpectedActualWitness) diagnosticWitnessVariant() {}

type MissingBasisWitness struct {
	required DiagnosticDatum
	actual   DiagnosticDatum
}

func NewMissingBasisWitness(
	required DiagnosticDatum,
	unknownReason string,
) (MissingBasisWitness, error) {
	if !required.valid() {
		return MissingBasisWitness{}, fmt.Errorf("required diagnostic datum is invalid")
	}
	unknown, err := NewUnknownDiagnosticDatum(unknownReason)
	if err != nil {
		return MissingBasisWitness{}, err
	}
	return NewMissingBasisWitnessWithActual(required, unknown)
}

func NewMissingBasisWitnessWithActual(
	required DiagnosticDatum,
	actual DiagnosticDatum,
) (MissingBasisWitness, error) {
	if !required.valid() || !actual.valid() {
		return MissingBasisWitness{}, fmt.Errorf("required and actual diagnostic data are required")
	}
	return MissingBasisWitness{
		required: copyDiagnosticDatum(required),
		actual:   copyDiagnosticDatum(actual),
	}, nil
}

func (witness MissingBasisWitness) Expected() DiagnosticDatum {
	return copyDiagnosticDatum(witness.required)
}

func (witness MissingBasisWitness) Actual() DiagnosticDatum {
	return copyDiagnosticDatum(witness.actual)
}

func (MissingBasisWitness) diagnosticWitnessVariant() {}

func validDiagnosticWitness(witness DiagnosticWitness) bool {
	switch value := witness.(type) {
	case ExpectedActualWitness:
		return value.expected.valid() && value.actual.valid()
	case MissingBasisWitness:
		return value.required.valid() &&
			value.actual.valid()
	default:
		return false
	}
}

func copyDiagnosticDatum(datum DiagnosticDatum) DiagnosticDatum {
	return DiagnosticDatum{
		kind:   datum.kind,
		values: append([]string(nil), datum.values...),
	}
}

// DiagnosticGoverningBasis keeps exact authored/compiler provenance distinct
// from validator rules, snapshot rules, and genuinely missing declarations.
type DiagnosticBasisKind string

const (
	DiagnosticBasisKnownDeclaration DiagnosticBasisKind = "known_declaration"
	DiagnosticBasisCoreValidator    DiagnosticBasisKind = "core_validator"
	DiagnosticBasisSnapshotRule     DiagnosticBasisKind = "snapshot_rule"
	DiagnosticBasisMissingTypeEnv   DiagnosticBasisKind = "missing_typeenv_declaration"
	DiagnosticBasisMissingRuntime   DiagnosticBasisKind = "missing_runtime_basis"
)

type DiagnosticGoverningBasis interface {
	Kind() DiagnosticBasisKind
	diagnosticGoverningBasisVariant()
}

type KnownDeclarationBasis struct {
	provenance DeclarationProvenance
}

func NewKnownDeclarationBasis(
	provenance DeclarationProvenance,
) (KnownDeclarationBasis, error) {
	if !validDeclarationProvenance(provenance) {
		return KnownDeclarationBasis{}, fmt.Errorf("known declaration provenance is required")
	}
	return KnownDeclarationBasis{provenance: provenance}, nil
}

func (KnownDeclarationBasis) Kind() DiagnosticBasisKind {
	return DiagnosticBasisKnownDeclaration
}

func (basis KnownDeclarationBasis) Provenance() DeclarationProvenance {
	return basis.provenance
}

func (KnownDeclarationBasis) diagnosticGoverningBasisVariant() {}

type CoreValidatorBasis struct {
	rule RuleRef
}

func NewCoreValidatorBasis(rule RuleRef) (CoreValidatorBasis, error) {
	if !rule.valid() {
		return CoreValidatorBasis{}, fmt.Errorf("core validator rule is required")
	}
	return CoreValidatorBasis{rule: rule}, nil
}

func (CoreValidatorBasis) Kind() DiagnosticBasisKind { return DiagnosticBasisCoreValidator }

func (basis CoreValidatorBasis) Rule() RuleRef { return basis.rule }

func (CoreValidatorBasis) diagnosticGoverningBasisVariant() {}

type SnapshotRuleBasis struct {
	rule RuleRef
}

func NewSnapshotRuleBasis(rule RuleRef) (SnapshotRuleBasis, error) {
	if !rule.valid() {
		return SnapshotRuleBasis{}, fmt.Errorf("snapshot rule is required")
	}
	return SnapshotRuleBasis{rule: rule}, nil
}

func (SnapshotRuleBasis) Kind() DiagnosticBasisKind { return DiagnosticBasisSnapshotRule }

func (basis SnapshotRuleBasis) Rule() RuleRef { return basis.rule }

func (SnapshotRuleBasis) diagnosticGoverningBasisVariant() {}

type MissingTypeEnvDeclarationBasis struct {
	typeEnv     TypeEnvRef
	subject     DiagnosticDatum
	coverage    CoverageEntry
	hasCoverage bool
}

func NewMissingTypeEnvDeclarationBasis(
	typeEnv TypeEnvRef,
	subject DiagnosticDatum,
) (MissingTypeEnvDeclarationBasis, error) {
	return newMissingTypeEnvDeclarationBasis(typeEnv, subject, CoverageEntry{}, false)
}

func NewSourceOnlyTypeEnvDeclarationBasis(
	typeEnv TypeEnvRef,
	subject DiagnosticDatum,
	coverage CoverageEntry,
) (MissingTypeEnvDeclarationBasis, error) {
	return newMissingTypeEnvDeclarationBasis(typeEnv, subject, coverage, true)
}

func newMissingTypeEnvDeclarationBasis(
	typeEnv TypeEnvRef,
	subject DiagnosticDatum,
	coverage CoverageEntry,
	hasCoverage bool,
) (MissingTypeEnvDeclarationBasis, error) {
	if !typeEnv.valid() {
		return MissingTypeEnvDeclarationBasis{}, fmt.Errorf("active TypeEnv is required")
	}
	if !subject.valid() {
		return MissingTypeEnvDeclarationBasis{}, fmt.Errorf("missing declaration subject is required")
	}
	if hasCoverage && !coverage.valid() {
		return MissingTypeEnvDeclarationBasis{}, fmt.Errorf("source-only coverage evidence is invalid")
	}
	return MissingTypeEnvDeclarationBasis{
		typeEnv:     typeEnv,
		subject:     copyDiagnosticDatum(subject),
		coverage:    coverage,
		hasCoverage: hasCoverage,
	}, nil
}

func (MissingTypeEnvDeclarationBasis) Kind() DiagnosticBasisKind {
	return DiagnosticBasisMissingTypeEnv
}

func (basis MissingTypeEnvDeclarationBasis) TypeEnv() TypeEnvRef { return basis.typeEnv }

func (basis MissingTypeEnvDeclarationBasis) Subject() DiagnosticDatum {
	return copyDiagnosticDatum(basis.subject)
}

func (basis MissingTypeEnvDeclarationBasis) Coverage() (CoverageEntry, bool) {
	return basis.coverage, basis.hasCoverage
}

func (MissingTypeEnvDeclarationBasis) diagnosticGoverningBasisVariant() {}

type MissingRuntimeBasisKind string

const (
	MissingRuntimeActiveTypeEnv MissingRuntimeBasisKind = "active_typeenv"
	MissingRuntimeSnapshot      MissingRuntimeBasisKind = "snapshot"
	MissingRuntimeDeclaration   MissingRuntimeBasisKind = "declaration"
	MissingRuntimeResolution    MissingRuntimeBasisKind = "resolution"
	MissingRuntimeCodec         MissingRuntimeBasisKind = "codec"
	MissingRuntimeCoverage      MissingRuntimeBasisKind = "coverage"
	MissingRuntimeValidator     MissingRuntimeBasisKind = "validator"
)

func (kind MissingRuntimeBasisKind) valid() bool {
	switch kind {
	case MissingRuntimeActiveTypeEnv,
		MissingRuntimeSnapshot,
		MissingRuntimeDeclaration,
		MissingRuntimeResolution,
		MissingRuntimeCodec,
		MissingRuntimeCoverage,
		MissingRuntimeValidator:
		return true
	default:
		return false
	}
}

type MissingRuntimeBasis struct {
	kind     MissingRuntimeBasisKind
	required DiagnosticDatum
}

func NewMissingRuntimeBasis(
	kind MissingRuntimeBasisKind,
	required DiagnosticDatum,
) (MissingRuntimeBasis, error) {
	if !kind.valid() {
		return MissingRuntimeBasis{}, fmt.Errorf("missing runtime basis kind is invalid")
	}
	if !required.valid() {
		return MissingRuntimeBasis{}, fmt.Errorf("missing runtime basis requirement is invalid")
	}
	return MissingRuntimeBasis{kind: kind, required: copyDiagnosticDatum(required)}, nil
}

func (MissingRuntimeBasis) Kind() DiagnosticBasisKind { return DiagnosticBasisMissingRuntime }

func (basis MissingRuntimeBasis) MissingKind() MissingRuntimeBasisKind { return basis.kind }

func (basis MissingRuntimeBasis) Required() DiagnosticDatum {
	return copyDiagnosticDatum(basis.required)
}

func (MissingRuntimeBasis) diagnosticGoverningBasisVariant() {}

func validDiagnosticGoverningBasis(basis DiagnosticGoverningBasis) bool {
	switch value := basis.(type) {
	case KnownDeclarationBasis:
		return validDeclarationProvenance(value.provenance)
	case CoreValidatorBasis:
		return value.rule.valid()
	case SnapshotRuleBasis:
		return value.rule.valid()
	case MissingTypeEnvDeclarationBasis:
		if !value.typeEnv.valid() || !value.subject.valid() {
			return false
		}
		return !value.hasCoverage || value.coverage.valid()
	case MissingRuntimeBasis:
		return value.kind.valid() && value.required.valid()
	default:
		return false
	}
}

func knownDiagnosticBasis(basis DiagnosticGoverningBasis) bool {
	switch basis.(type) {
	case KnownDeclarationBasis, CoreValidatorBasis, SnapshotRuleBasis:
		return true
	default:
		return false
	}
}

func missingDiagnosticBasis(basis DiagnosticGoverningBasis) bool {
	switch basis.(type) {
	case MissingTypeEnvDeclarationBasis, MissingRuntimeBasis:
		return true
	default:
		return false
	}
}

type RepairKind string

const (
	RepairChangeInput     RepairKind = "change_input"
	RepairInspectBasis    RepairKind = "inspect_basis"
	RepairExtendTypeEnv   RepairKind = "extend_typeenv"
	RepairRefreshSnapshot RepairKind = "refresh_snapshot"
	RepairResolveIdentity RepairKind = "resolve_identity"
)

func (kind RepairKind) valid() bool {
	switch kind {
	case RepairChangeInput,
		RepairInspectBasis,
		RepairExtendTypeEnv,
		RepairRefreshSnapshot,
		RepairResolveIdentity:
		return true
	default:
		return false
	}
}

type HumanChoiceRequirement string

const (
	HumanChoiceNotClaimed HumanChoiceRequirement = "not_claimed"
	HumanChoiceRequired   HumanChoiceRequirement = "required"
)

func (requirement HumanChoiceRequirement) valid() bool {
	return requirement == HumanChoiceNotClaimed || requirement == HumanChoiceRequired
}

type RepairCandidate struct {
	kind        RepairKind
	pointer     RepairPointer
	target      DiagnosticDatum
	humanChoice HumanChoiceRequirement
}

func NewRepairCandidate(
	kind RepairKind,
	pointer RepairPointer,
	target DiagnosticDatum,
	humanChoice HumanChoiceRequirement,
) (RepairCandidate, error) {
	if !kind.valid() || !pointer.valid() || !target.valid() || !humanChoice.valid() {
		return RepairCandidate{}, fmt.Errorf("repair kind, pointer, target, and human-choice marker are required")
	}
	return RepairCandidate{
		kind:        kind,
		pointer:     pointer,
		target:      copyDiagnosticDatum(target),
		humanChoice: humanChoice,
	}, nil
}

func (candidate RepairCandidate) Kind() RepairKind { return candidate.kind }

func (candidate RepairCandidate) Pointer() RepairPointer { return candidate.pointer }

func (candidate RepairCandidate) Target() DiagnosticDatum {
	return copyDiagnosticDatum(candidate.target)
}

func (candidate RepairCandidate) HumanChoiceRequirement() HumanChoiceRequirement {
	return candidate.humanChoice
}

func (candidate RepairCandidate) valid() bool {
	return candidate.kind.valid() &&
		candidate.pointer.valid() &&
		candidate.target.valid() &&
		candidate.humanChoice.valid()
}

type Diagnostic struct {
	posture       DiagnosticPosture
	code          DiagnosticCode
	message       string
	path          DiagnosticPath
	witness       DiagnosticWitness
	basis         DiagnosticGoverningBasis
	missingRepair RepairPointer
	repairs       []RepairCandidate
}

func NewInvalidDiagnosticWithDetails(
	code DiagnosticCode,
	message string,
	path DiagnosticPath,
	witness ExpectedActualWitness,
	basis DiagnosticGoverningBasis,
	repairs []RepairCandidate,
) (Diagnostic, error) {
	if !code.valid() {
		return Diagnostic{}, fmt.Errorf("invalid diagnostic code is required")
	}
	if strings.TrimSpace(message) == "" {
		return Diagnostic{}, fmt.Errorf("invalid diagnostic message is required")
	}
	if !path.valid() {
		return Diagnostic{}, fmt.Errorf("invalid diagnostic path is required")
	}
	if !validDiagnosticWitness(witness) {
		return Diagnostic{}, fmt.Errorf("invalid diagnostic witness is required")
	}
	if !validDiagnosticGoverningBasis(basis) || !knownDiagnosticBasis(basis) {
		return Diagnostic{}, fmt.Errorf("invalid diagnostic must cite a known governing basis")
	}
	ownedRepairs, err := validateRepairCandidates(repairs)
	if err != nil {
		return Diagnostic{}, err
	}
	return Diagnostic{
		posture: DiagnosticInvalid,
		code:    code,
		message: strings.TrimSpace(message),
		path:    path,
		witness: witness,
		basis:   basis,
		repairs: ownedRepairs,
	}, nil
}

func NewUnderdeterminedDiagnosticWithDetails(
	code DiagnosticCode,
	message string,
	path DiagnosticPath,
	witness MissingBasisWitness,
	basis DiagnosticGoverningBasis,
	missingRepair RepairPointer,
	repairs []RepairCandidate,
) (Diagnostic, error) {
	if !code.valid() {
		return Diagnostic{}, fmt.Errorf("underdetermined diagnostic code is required")
	}
	if strings.TrimSpace(message) == "" {
		return Diagnostic{}, fmt.Errorf("underdetermined diagnostic message is required")
	}
	if !path.valid() {
		return Diagnostic{}, fmt.Errorf("underdetermined diagnostic path is required")
	}
	if !validDiagnosticWitness(witness) {
		return Diagnostic{}, fmt.Errorf("underdetermined diagnostic witness is required")
	}
	if !validDiagnosticGoverningBasis(basis) || !missingDiagnosticBasis(basis) {
		return Diagnostic{}, fmt.Errorf("underdetermined diagnostic must name a missing governing basis")
	}
	if !missingRepair.valid() {
		return Diagnostic{}, fmt.Errorf("underdetermined diagnostic must name a missing-basis repair pointer")
	}
	ownedRepairs, err := validateRepairCandidates(repairs)
	if err != nil {
		return Diagnostic{}, err
	}
	return Diagnostic{
		posture:       DiagnosticUnderdetermined,
		code:          code,
		message:       strings.TrimSpace(message),
		path:          path,
		witness:       witness,
		basis:         basis,
		missingRepair: missingRepair,
		repairs:       ownedRepairs,
	}, nil
}

func validateRepairCandidates(values []RepairCandidate) ([]RepairCandidate, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("diagnostic requires at least one unselected repair candidate")
	}
	owned := append([]RepairCandidate(nil), values...)
	for index, candidate := range owned {
		if !candidate.valid() {
			return nil, fmt.Errorf("repair candidate %d is invalid", index)
		}
	}
	return owned, nil
}

func (diagnostic Diagnostic) Posture() DiagnosticPosture { return diagnostic.posture }

func (diagnostic Diagnostic) Code() DiagnosticCode { return diagnostic.code }

func (diagnostic Diagnostic) Message() string { return diagnostic.message }

func (diagnostic Diagnostic) Path() DiagnosticPath { return diagnostic.path }

func (diagnostic Diagnostic) Witness() DiagnosticWitness { return diagnostic.witness }

func (diagnostic Diagnostic) GoverningBasis() DiagnosticGoverningBasis {
	return diagnostic.basis
}

func (diagnostic Diagnostic) Rule() (RuleRef, bool) {
	switch basis := diagnostic.basis.(type) {
	case KnownDeclarationBasis:
		return RuleRef{value: basis.provenance.Reference().String()}, true
	case CoreValidatorBasis:
		return basis.rule, basis.rule.valid()
	case SnapshotRuleBasis:
		return basis.rule, basis.rule.valid()
	default:
		return RuleRef{}, false
	}
}

func (diagnostic Diagnostic) Repair() (RepairPointer, bool) {
	return diagnostic.missingRepair, diagnostic.missingRepair.valid()
}

func (diagnostic Diagnostic) RepairCandidates() []RepairCandidate {
	return append([]RepairCandidate(nil), diagnostic.repairs...)
}

func (diagnostic Diagnostic) valid() bool {
	if !diagnostic.code.valid() ||
		diagnostic.message == "" ||
		!diagnostic.path.valid() ||
		!validDiagnosticWitness(diagnostic.witness) ||
		!validDiagnosticGoverningBasis(diagnostic.basis) {
		return false
	}
	if _, err := validateRepairCandidates(diagnostic.repairs); err != nil {
		return false
	}
	if diagnostic.posture == DiagnosticInvalid {
		_, exactWitness := diagnostic.witness.(ExpectedActualWitness)
		return exactWitness && knownDiagnosticBasis(diagnostic.basis) && !diagnostic.missingRepair.valid()
	}
	if diagnostic.posture == DiagnosticUnderdetermined {
		_, missingWitness := diagnostic.witness.(MissingBasisWitness)
		return missingWitness && missingDiagnosticBasis(diagnostic.basis) && diagnostic.missingRepair.valid()
	}
	return false
}

func copyDiagnostics(values []Diagnostic) []Diagnostic {
	return append([]Diagnostic(nil), values...)
}

func unknownExpectedActualWitness(reason string) ExpectedActualWitness {
	expected, _ := NewUnknownDiagnosticDatum("expected value was not retained: " + reason)
	actual, _ := NewUnknownDiagnosticDatum("actual value was not retained: " + reason)
	witness, _ := NewExpectedActualWitness(expected, actual)
	return witness
}

func diagnosticText(value string) DiagnosticDatum {
	datum, err := NewDiagnosticTextDatum(value)
	if err == nil {
		return datum
	}
	return diagnosticUnknown("text datum was unavailable: " + err.Error())
}

func diagnosticReference(value string) DiagnosticDatum {
	datum, err := NewDiagnosticReferenceDatum(value)
	if err == nil {
		return datum
	}
	return diagnosticUnknown("reference datum was unavailable: " + err.Error())
}

func diagnosticSet(values []string) DiagnosticDatum {
	datum, err := NewDiagnosticSetDatum(values)
	if err == nil {
		return datum
	}
	return diagnosticUnknown("set datum was unavailable: " + err.Error())
}

func diagnosticState(value string) DiagnosticDatum {
	datum, err := NewDiagnosticStateDatum(value)
	if err == nil {
		return datum
	}
	return diagnosticUnknown("state datum was unavailable: " + err.Error())
}

func diagnosticUnknown(reason string) DiagnosticDatum {
	datum, _ := NewUnknownDiagnosticDatum(reason)
	return datum
}

func genericRequiredDatum(code DiagnosticCode) DiagnosticDatum {
	state, _ := NewDiagnosticStateDatum("required basis for " + string(code))
	return state
}

func missingRuntimeKind(code DiagnosticCode) MissingRuntimeBasisKind {
	switch code {
	case DiagnosticCodecUnavailable:
		return MissingRuntimeCodec
	case DiagnosticCompilerCoverageMissing:
		return MissingRuntimeCoverage
	case DiagnosticReferenceUnresolved,
		DiagnosticIdentityBasisMissing,
		DiagnosticAssertionNotFound:
		return MissingRuntimeResolution
	case DiagnosticTypeRuleUnavailable,
		DiagnosticSignatureNotActive,
		DiagnosticKindUnavailableInContext,
		DiagnosticContextNotActive,
		DiagnosticContextBridgeMissing,
		DiagnosticValueBindingNotActive:
		return MissingRuntimeDeclaration
	default:
		return MissingRuntimeSnapshot
	}
}

func defaultInvalidRepair(
	code DiagnosticCode,
	path DiagnosticPath,
	target DiagnosticDatum,
) RepairCandidate {
	pointer, _ := NewRepairPointer("change-candidate-at:" + path.String())
	candidate, _ := NewRepairCandidate(
		RepairChangeInput,
		pointer,
		target,
		humanChoiceForDiagnostic(code),
	)
	return candidate
}

func defaultMissingBasisRepair(
	code DiagnosticCode,
	basis DiagnosticGoverningBasis,
	pointer RepairPointer,
	target DiagnosticDatum,
) RepairCandidate {
	kind, humanChoice := repairDispositionForMissingBasis(code, basis)
	candidate, _ := NewRepairCandidate(
		kind,
		pointer,
		target,
		humanChoice,
	)
	return candidate
}

func repairDispositionForMissingBasis(
	code DiagnosticCode,
	basis DiagnosticGoverningBasis,
) (RepairKind, HumanChoiceRequirement) {
	switch value := basis.(type) {
	case MissingTypeEnvDeclarationBasis:
		return RepairExtendTypeEnv, HumanChoiceRequired
	case MissingRuntimeBasis:
		switch value.MissingKind() {
		case MissingRuntimeSnapshot:
			return RepairRefreshSnapshot, HumanChoiceNotClaimed
		case MissingRuntimeResolution:
			if code == DiagnosticIdentityBasisMissing {
				return RepairResolveIdentity, HumanChoiceRequired
			}
			return RepairRefreshSnapshot, HumanChoiceNotClaimed
		case MissingRuntimeCodec, MissingRuntimeValidator:
			return RepairInspectBasis, HumanChoiceNotClaimed
		case MissingRuntimeActiveTypeEnv,
			MissingRuntimeDeclaration,
			MissingRuntimeCoverage:
			return RepairInspectBasis, HumanChoiceRequired
		}
	}
	return RepairInspectBasis, HumanChoiceNotClaimed
}

func humanChoiceForDiagnostic(code DiagnosticCode) HumanChoiceRequirement {
	switch code {
	case DiagnosticSignatureNotActive,
		DiagnosticKindUnavailableInContext,
		DiagnosticContextNotActive,
		DiagnosticContextBridgeMissing,
		DiagnosticValueBindingNotActive,
		DiagnosticTypeRuleUnavailable,
		DiagnosticCompilerCoverageMissing,
		DiagnosticAliasAmbiguous,
		DiagnosticAliasAlreadyBound,
		DiagnosticAliasNotBound,
		DiagnosticIdentityBasisMissing,
		DiagnosticEntityAlreadyExists,
		DiagnosticEntityKindMismatch:
		return HumanChoiceRequired
	default:
		return HumanChoiceNotClaimed
	}
}
