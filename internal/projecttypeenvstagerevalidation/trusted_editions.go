package projecttypeenvstagerevalidation

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"sort"

	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
)

const trustedStageEditionCatalogDomain = "haft.projecttypeenv.stage-trusted-editions.v1"

// TrustedStageEditionCoordinate is the closed set of static implementation
// editions compared before a Stage can be considered for selection. Target-X
// callable and registration coordinates belong to the separate runtime
// registry derivation.
type TrustedStageEditionCoordinate uint8

const (
	TrustedStageSchemaEdition TrustedStageEditionCoordinate = iota + 1
	TrustedStageCompilerEdition
	TrustedBaseCompilerSchemaEdition
	TrustedStageProducerEdition
	TrustedStageRevalidatorEdition
	TrustedCompositeLowererEdition
)

func (coordinate TrustedStageEditionCoordinate) String() string {
	switch coordinate {
	case TrustedStageSchemaEdition:
		return "stage_schema"
	case TrustedStageCompilerEdition:
		return "stage_compiler"
	case TrustedBaseCompilerSchemaEdition:
		return "base_compiler_schema"
	case TrustedStageProducerEdition:
		return "stage_producer"
	case TrustedStageRevalidatorEdition:
		return "stage_revalidator"
	case TrustedCompositeLowererEdition:
		return "composite_lowerer"
	default:
		return ""
	}
}

// TrustedStageEditionCatalogDigest is the content identity of the compiled,
// package-owned static edition catalog. It is not a TypeEnvRef, StageRef,
// evidence result, or selection authority.
type TrustedStageEditionCatalogDigest struct {
	value [sha256.Size]byte
}

func (digest TrustedStageEditionCatalogDigest) String() string {
	encoded := hex.EncodeToString(digest.value[:])
	return "sha256:" + encoded
}

// TrustedStageEditionObservationDigest identifies the exact static edition
// coordinates observed in one verified Stage plus executable snapshot.
type TrustedStageEditionObservationDigest struct {
	value [sha256.Size]byte
}

func (digest TrustedStageEditionObservationDigest) String() string {
	encoded := hex.EncodeToString(digest.value[:])
	return "sha256:" + encoded
}

// TrustedStageEditionCatalog is immutable package-owned policy. Callers can
// inspect it, but cannot construct a different catalog from arbitrary strings.
// Matching this catalog is only one revalidation input.
type TrustedStageEditionCatalog struct {
	stageSchema      string
	stageCompiler    projecttypeenvselection.StageCompilerEdition
	baseCompiler     string
	stageProducer    projecttypeenvselection.StageProducerEdition
	stageRevalidator projecttypeenvselection.StageRevalidatorEdition
	compositeLowerer string
	canonical        []byte
	digest           TrustedStageEditionCatalogDigest
}

// CurrentTrustedStageEditionCatalog retains the already-sealed Genesis/v2
// policy. Compiler-v3 candidates use a distinct catalog selected from their
// exact executable observation; this function must not relabel old Stages.
func CurrentTrustedStageEditionCatalog() TrustedStageEditionCatalog {
	return newTrustedStageEditionCatalog(
		projecttypeenvselection.ProjectTypeEnvStageSchemaEditionV4,
		projecttypeenvselection.StageCompilerEditionV4(),
		typeenv.BaseTypeEnvCompilerSchemaV2,
		projecttypeenvselection.StageProducerEditionV4(),
		projecttypeenvselection.StageRevalidatorEditionV4(),
		projecttypeenv.ProjectTypeEnvCompositeLowererSchemaV1,
	)
}

func currentBaseV3GenesisTrustedStageEditionCatalog() TrustedStageEditionCatalog {
	return newTrustedStageEditionCatalog(
		projecttypeenvselection.ProjectTypeEnvStageSchemaEditionV4,
		projecttypeenvselection.StageCompilerEditionV4(),
		typeenv.BaseTypeEnvCompilerSchemaV3,
		projecttypeenvselection.StageProducerEditionV4(),
		projecttypeenvselection.StageRevalidatorEditionV4(),
		projecttypeenv.ProjectTypeEnvCompositeLowererSchemaV1,
	)
}

func currentBaseV4GenesisTrustedStageEditionCatalog() TrustedStageEditionCatalog {
	return newTrustedStageEditionCatalog(
		projecttypeenvselection.ProjectTypeEnvStageSchemaEditionV4,
		projecttypeenvselection.StageCompilerEditionV4(),
		typeenv.BaseTypeEnvCompilerSchemaV4,
		projecttypeenvselection.StageProducerEditionV4(),
		projecttypeenvselection.StageRevalidatorEditionV4(),
		projecttypeenv.ProjectTypeEnvCompositeLowererSchemaV2,
	)
}

func currentTransitionTrustedStageEditionCatalog() TrustedStageEditionCatalog {
	return newTrustedStageEditionCatalog(
		projecttypeenvselection.ProjectTypeEnvStageSchemaEditionV5,
		projecttypeenvselection.StageCompilerEditionV5(),
		typeenv.BaseTypeEnvCompilerSchemaV3,
		projecttypeenvselection.StageProducerEditionV5(),
		projecttypeenvselection.StageRevalidatorEditionV5(),
		projecttypeenv.ProjectTypeEnvCompositeLowererSchemaV1,
	)
}

func currentBaseV4TransitionTrustedStageEditionCatalog() TrustedStageEditionCatalog {
	return newTrustedStageEditionCatalog(
		projecttypeenvselection.ProjectTypeEnvStageSchemaEditionV5,
		projecttypeenvselection.StageCompilerEditionV5(),
		typeenv.BaseTypeEnvCompilerSchemaV4,
		projecttypeenvselection.StageProducerEditionV5(),
		projecttypeenvselection.StageRevalidatorEditionV5(),
		projecttypeenv.ProjectTypeEnvCompositeLowererSchemaV2,
	)
}

func newTrustedStageEditionCatalog(
	stageSchema string,
	stageCompiler projecttypeenvselection.StageCompilerEdition,
	baseCompiler string,
	stageProducer projecttypeenvselection.StageProducerEdition,
	stageRevalidator projecttypeenvselection.StageRevalidatorEdition,
	compositeLowerer string,
) TrustedStageEditionCatalog {
	catalog := TrustedStageEditionCatalog{
		stageSchema:      stageSchema,
		stageCompiler:    stageCompiler,
		baseCompiler:     baseCompiler,
		stageProducer:    stageProducer,
		stageRevalidator: stageRevalidator,
		compositeLowerer: compositeLowerer,
	}
	catalog.canonical = trustedStageEditionCatalogCanonical(catalog)
	catalog.digest = digestTrustedStageEditionCatalog(catalog.canonical)
	return catalog
}

func (catalog TrustedStageEditionCatalog) StageSchemaEdition() string {
	return catalog.stageSchema
}

func (catalog TrustedStageEditionCatalog) StageCompilerEdition() projecttypeenvselection.StageCompilerEdition {
	return catalog.stageCompiler
}

func (catalog TrustedStageEditionCatalog) BaseCompilerSchemaEdition() string {
	return catalog.baseCompiler
}

func (catalog TrustedStageEditionCatalog) StageProducerEdition() projecttypeenvselection.StageProducerEdition {
	return catalog.stageProducer
}

func (catalog TrustedStageEditionCatalog) StageRevalidatorEdition() projecttypeenvselection.StageRevalidatorEdition {
	return catalog.stageRevalidator
}

func (catalog TrustedStageEditionCatalog) CompositeLowererEdition() string {
	return catalog.compositeLowerer
}

func (catalog TrustedStageEditionCatalog) CanonicalBytes() []byte {
	return append([]byte(nil), catalog.canonical...)
}

func (catalog TrustedStageEditionCatalog) Digest() TrustedStageEditionCatalogDigest {
	return catalog.digest
}

// TrustedStageEditionInputIssueCode distinguishes malformed strong inputs from
// a well-formed but unsupported edition.
type TrustedStageEditionInputIssueCode uint8

const (
	TrustedEditionStageInvalid TrustedStageEditionInputIssueCode = iota + 1
	TrustedEditionExecutableSnapshotInvalid
	TrustedEditionTargetRuntimeBasisMismatch
)

func (code TrustedStageEditionInputIssueCode) String() string {
	switch code {
	case TrustedEditionStageInvalid:
		return "stage_invalid"
	case TrustedEditionExecutableSnapshotInvalid:
		return "executable_snapshot_invalid"
	case TrustedEditionTargetRuntimeBasisMismatch:
		return "target_runtime_basis_mismatch"
	default:
		return ""
	}
}

type TrustedStageEditionInputIssue struct {
	code   TrustedStageEditionInputIssueCode
	actual string
	repair string
}

func (issue TrustedStageEditionInputIssue) Code() TrustedStageEditionInputIssueCode {
	return issue.code
}

func (issue TrustedStageEditionInputIssue) Actual() string { return issue.actual }

func (issue TrustedStageEditionInputIssue) Repair() string { return issue.repair }

type UnsupportedTrustedStageEdition struct {
	coordinate TrustedStageEditionCoordinate
	expected   string
	actual     string
	repair     string
}

func (issue UnsupportedTrustedStageEdition) Coordinate() TrustedStageEditionCoordinate {
	return issue.coordinate
}

func (issue UnsupportedTrustedStageEdition) Expected() string { return issue.expected }

func (issue UnsupportedTrustedStageEdition) Actual() string { return issue.actual }

func (issue UnsupportedTrustedStageEdition) Repair() string { return issue.repair }

// TrustedStageEditionComparison is a closed pure result. None of its variants
// authorizes head selection or proves mutable Stage bases current.
type TrustedStageEditionComparison interface {
	trustedStageEditionComparisonVariant()
}

type InvalidTrustedStageEditionInput struct {
	issues []TrustedStageEditionInputIssue
}

func (result InvalidTrustedStageEditionInput) Issues() []TrustedStageEditionInputIssue {
	return append([]TrustedStageEditionInputIssue(nil), result.issues...)
}

func (InvalidTrustedStageEditionInput) trustedStageEditionComparisonVariant() {}

type UnsupportedTrustedStageEditions struct {
	catalogDigest     TrustedStageEditionCatalogDigest
	observationDigest TrustedStageEditionObservationDigest
	issues            []UnsupportedTrustedStageEdition
}

func (result UnsupportedTrustedStageEditions) CatalogDigest() TrustedStageEditionCatalogDigest {
	return result.catalogDigest
}

func (result UnsupportedTrustedStageEditions) ObservationDigest() TrustedStageEditionObservationDigest {
	return result.observationDigest
}

func (result UnsupportedTrustedStageEditions) Issues() []UnsupportedTrustedStageEdition {
	return append([]UnsupportedTrustedStageEdition(nil), result.issues...)
}

func (UnsupportedTrustedStageEditions) trustedStageEditionComparisonVariant() {}

// TargetRuntimeRegistryRequirement binds the exact X that a separate
// transaction-local runtime-registry service must resolve. It is a request for
// a stronger derived input, not a registry observation or trust capability.
type TargetRuntimeRegistryRequirement struct {
	target  projecttypeenv.RuntimeEvaluationBasisRef
	catalog TrustedStageEditionCatalogDigest
}

func (requirement TargetRuntimeRegistryRequirement) TargetRuntimeBasis() projecttypeenv.RuntimeEvaluationBasisRef {
	return requirement.target
}

func (requirement TargetRuntimeRegistryRequirement) CatalogDigest() TrustedStageEditionCatalogDigest {
	return requirement.catalog
}

type StaticTrustedStageEditionsMatched struct {
	catalogDigest      TrustedStageEditionCatalogDigest
	observationDigest  TrustedStageEditionObservationDigest
	runtimeRequirement TargetRuntimeRegistryRequirement
}

func (result StaticTrustedStageEditionsMatched) CatalogDigest() TrustedStageEditionCatalogDigest {
	return result.catalogDigest
}

func (result StaticTrustedStageEditionsMatched) ObservationDigest() TrustedStageEditionObservationDigest {
	return result.observationDigest
}

func (result StaticTrustedStageEditionsMatched) RuntimeRegistryRequirement() TargetRuntimeRegistryRequirement {
	return result.runtimeRequirement
}

func (StaticTrustedStageEditionsMatched) trustedStageEditionComparisonVariant() {}

type staticStageEditionObservation struct {
	stageSchema      string
	stageCompiler    string
	baseCompiler     string
	stageProducer    string
	stageRevalidator string
	compositeLowerer string
	targetRuntime    projecttypeenv.RuntimeEvaluationBasisRef
}

// CompareCurrentTrustedStageEditions verifies strong inputs, compares their
// exact static implementation editions with the compiled catalog, and returns
// the exact target-X requirement for the separate runtime-registry derivation.
func CompareCurrentTrustedStageEditions(
	stage projecttypeenvselection.ProjectTypeEnvStage,
	executable projecttypeenv.ProjectTypeEnvExecutableSnapshot,
) TrustedStageEditionComparison {
	inputIssues := trustedStageEditionInputIssues(stage, executable)
	if len(inputIssues) > 0 {
		return InvalidTrustedStageEditionInput{issues: inputIssues}
	}
	observation := observeStaticStageEditions(stage, executable)
	catalog := trustedStageEditionCatalogForStage(stage, observation)
	observationCanonical := staticStageEditionObservationCanonical(observation)
	observationDigest := digestTrustedStageEditionObservation(observationCanonical)
	unsupported := compareStaticStageEditionObservation(catalog, observation)
	if len(unsupported) > 0 {
		return UnsupportedTrustedStageEditions{
			catalogDigest:     catalog.Digest(),
			observationDigest: observationDigest,
			issues:            unsupported,
		}
	}
	requirement := TargetRuntimeRegistryRequirement{
		target:  observation.targetRuntime,
		catalog: catalog.Digest(),
	}
	return StaticTrustedStageEditionsMatched{
		catalogDigest:      catalog.Digest(),
		observationDigest:  observationDigest,
		runtimeRequirement: requirement,
	}
}

func trustedStageEditionCatalogForStage(
	stage projecttypeenvselection.ProjectTypeEnvStage,
	observation staticStageEditionObservation,
) TrustedStageEditionCatalog {
	if _, transition := stage.Predecessor().(projecttypeenvselection.TransitionStagePredecessor); transition {
		if observation.baseCompiler == typeenv.BaseTypeEnvCompilerSchemaV4 {
			return currentBaseV4TransitionTrustedStageEditionCatalog()
		}
		return currentTransitionTrustedStageEditionCatalog()
	}
	if observation.baseCompiler == typeenv.BaseTypeEnvCompilerSchemaV4 {
		return currentBaseV4GenesisTrustedStageEditionCatalog()
	}
	if observation.baseCompiler == typeenv.BaseTypeEnvCompilerSchemaV3 {
		return currentBaseV3GenesisTrustedStageEditionCatalog()
	}
	return CurrentTrustedStageEditionCatalog()
}

func trustedStageEditionInputIssues(
	stage projecttypeenvselection.ProjectTypeEnvStage,
	executable projecttypeenv.ProjectTypeEnvExecutableSnapshot,
) []TrustedStageEditionInputIssue {
	issues := make([]TrustedStageEditionInputIssue, 0, 3)
	stageErr := stage.Verify()
	if stageErr != nil {
		issues = append(issues, TrustedStageEditionInputIssue{
			code:   TrustedEditionStageInvalid,
			actual: stageErr.Error(),
			repair: "reload and verify the exact immutable Stage",
		})
	}
	executableErr := executable.Verify()
	if executableErr != nil {
		issues = append(issues, TrustedStageEditionInputIssue{
			code:   TrustedEditionExecutableSnapshotInvalid,
			actual: executableErr.Error(),
			repair: "restore the exact executable snapshot by replaying final lowering",
		})
	}
	if stageErr == nil && executableErr == nil {
		record := executable.Record()
		if stage.RuntimeBasis() != record.RuntimeEvaluationBasisRef() {
			issues = append(issues, TrustedStageEditionInputIssue{
				code:   TrustedEditionTargetRuntimeBasisMismatch,
				actual: record.RuntimeEvaluationBasisRef().String(),
				repair: "reload the executable snapshot bound to the Stage target X",
			})
		}
	}
	sort.Slice(issues, func(left int, right int) bool {
		return issues[left].code < issues[right].code
	})
	return issues
}

func observeStaticStageEditions(
	stage projecttypeenvselection.ProjectTypeEnvStage,
	executable projecttypeenv.ProjectTypeEnvExecutableSnapshot,
) staticStageEditionObservation {
	record := executable.Record()
	return staticStageEditionObservation{
		stageSchema:      stage.SchemaEdition(),
		stageCompiler:    stage.CompilerEdition().String(),
		baseCompiler:     record.CompilerSchemaVersion().String(),
		stageProducer:    stage.ProducerEdition().String(),
		stageRevalidator: stage.RevalidatorEdition().String(),
		compositeLowerer: record.LowererSchemaVersion(),
		targetRuntime:    record.RuntimeEvaluationBasisRef(),
	}
}

func compareStaticStageEditionObservation(
	catalog TrustedStageEditionCatalog,
	observation staticStageEditionObservation,
) []UnsupportedTrustedStageEdition {
	issues := make([]UnsupportedTrustedStageEdition, 0, 6)
	issues = appendUnsupportedEdition(
		issues,
		TrustedStageSchemaEdition,
		catalog.StageSchemaEdition(),
		observation.stageSchema,
	)
	issues = appendUnsupportedEdition(
		issues,
		TrustedStageCompilerEdition,
		catalog.StageCompilerEdition().String(),
		observation.stageCompiler,
	)
	issues = appendUnsupportedEdition(
		issues,
		TrustedBaseCompilerSchemaEdition,
		catalog.BaseCompilerSchemaEdition(),
		observation.baseCompiler,
	)
	issues = appendUnsupportedEdition(
		issues,
		TrustedStageProducerEdition,
		catalog.StageProducerEdition().String(),
		observation.stageProducer,
	)
	issues = appendUnsupportedEdition(
		issues,
		TrustedStageRevalidatorEdition,
		catalog.StageRevalidatorEdition().String(),
		observation.stageRevalidator,
	)
	issues = appendUnsupportedEdition(
		issues,
		TrustedCompositeLowererEdition,
		catalog.CompositeLowererEdition(),
		observation.compositeLowerer,
	)
	return issues
}

func appendUnsupportedEdition(
	issues []UnsupportedTrustedStageEdition,
	coordinate TrustedStageEditionCoordinate,
	expected string,
	actual string,
) []UnsupportedTrustedStageEdition {
	if expected == actual {
		return issues
	}
	return append(issues, UnsupportedTrustedStageEdition{
		coordinate: coordinate,
		expected:   expected,
		actual:     actual,
		repair:     unsupportedEditionRepair(coordinate),
	})
}

func unsupportedEditionRepair(
	coordinate TrustedStageEditionCoordinate,
) string {
	switch coordinate {
	case TrustedBaseCompilerSchemaEdition:
		return "recompile B with the current Base-TypeEnv compiler and rebuild B/E/X/C plus Stage"
	case TrustedCompositeLowererEdition:
		return "rerun final lowering with the current composite lowerer and rebuild the Stage"
	case TrustedStageSchemaEdition:
		return "rebuild and review the Stage under the supported Stage schema"
	case TrustedStageCompilerEdition:
		return "rebuild and review the Stage with the current Stage compiler"
	case TrustedStageProducerEdition:
		return "rebuild and review the Stage with the current Stage producer"
	case TrustedStageRevalidatorEdition:
		return "rebuild and review the Stage under the current Stage revalidator"
	default:
		return "rebuild and review the Stage under the current implementation editions"
	}
}

func trustedStageEditionCatalogCanonical(
	catalog TrustedStageEditionCatalog,
) []byte {
	values := []string{
		trustedStageEditionCatalogDomain,
		TrustedStageSchemaEdition.String(),
		catalog.stageSchema,
		TrustedStageCompilerEdition.String(),
		catalog.stageCompiler.String(),
		TrustedBaseCompilerSchemaEdition.String(),
		catalog.baseCompiler,
		TrustedStageProducerEdition.String(),
		catalog.stageProducer.String(),
		TrustedStageRevalidatorEdition.String(),
		catalog.stageRevalidator.String(),
		TrustedCompositeLowererEdition.String(),
		catalog.compositeLowerer,
	}
	return canonicalTrustedEditionStrings(values)
}

func staticStageEditionObservationCanonical(
	observation staticStageEditionObservation,
) []byte {
	values := []string{
		trustedStageEditionCatalogDomain + ".observation",
		TrustedStageSchemaEdition.String(),
		observation.stageSchema,
		TrustedStageCompilerEdition.String(),
		observation.stageCompiler,
		TrustedBaseCompilerSchemaEdition.String(),
		observation.baseCompiler,
		TrustedStageProducerEdition.String(),
		observation.stageProducer,
		TrustedStageRevalidatorEdition.String(),
		observation.stageRevalidator,
		TrustedCompositeLowererEdition.String(),
		observation.compositeLowerer,
		"target_runtime_basis",
		observation.targetRuntime.String(),
	}
	return canonicalTrustedEditionStrings(values)
}

func canonicalTrustedEditionStrings(values []string) []byte {
	result := make([]byte, 0)
	lengthBuffer := make([]byte, 8)
	for _, value := range values {
		binary.BigEndian.PutUint64(lengthBuffer, uint64(len(value)))
		result = append(result, lengthBuffer...)
		result = append(result, value...)
	}
	return result
}

func digestTrustedStageEditionCatalog(
	canonical []byte,
) TrustedStageEditionCatalogDigest {
	sum := sha256.Sum256(canonical)
	return TrustedStageEditionCatalogDigest{value: sum}
}

func digestTrustedStageEditionObservation(
	canonical []byte,
) TrustedStageEditionObservationDigest {
	sum := sha256.Sum256(canonical)
	return TrustedStageEditionObservationDigest{value: sum}
}
