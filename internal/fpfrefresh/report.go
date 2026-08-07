// Package fpfrefresh contains the pure compatibility model used before an FPF
// source refresh performs any repository mutation.
package fpfrefresh

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	// ReportSchemaV1 is the canonical compatibility-report schema.
	ReportSchemaV1 = "haft.fpf-refresh-report/v1"

	maxCoordinateText = 4096
	maxMessageText    = 8192
)

var (
	// ErrDuplicateDelta means one delta family described the same subject more
	// than once.
	ErrDuplicateDelta = errors.New("duplicate refresh delta subject")
	// ErrDuplicateDiagnostic means the report repeated one exact diagnostic.
	ErrDuplicateDiagnostic = errors.New("duplicate refresh diagnostic")
	// ErrDuplicateStageTiming means one execution stage supplied two durations.
	ErrDuplicateStageTiming = errors.New("duplicate refresh stage timing")
	// ErrIncompleteCandidate means a non-rejected result lacks verified derived
	// candidate coordinates.
	ErrIncompleteCandidate = errors.New("non-rejected refresh candidate requires derived coordinates")
	// ErrMissingLocalPracticeAssessment means a complete candidate report omitted
	// the required closed Local-Practice compatibility observation.
	ErrMissingLocalPracticeAssessment = errors.New("complete refresh candidate requires Local-Practice compatibility assessment")
	// ErrUnexplainedCandidateChange means complete predecessor and candidate
	// coordinates differ without a classified delta or diagnostic.
	ErrUnexplainedCandidateChange = errors.New("changed refresh candidate requires a classified delta or diagnostic")
)

// SourceCoordinates identify the exact source bytes used by one snapshot.
type SourceCoordinates struct {
	revision     typedmemory.SourceRevision
	readmeDigest typedmemory.SHA256Digest
	specDigest   typedmemory.SHA256Digest
}

// NewSourceCoordinates constructs exact source coordinates.
func NewSourceCoordinates(
	revision typedmemory.SourceRevision,
	readmeDigest typedmemory.SHA256Digest,
	specDigest typedmemory.SHA256Digest,
) (SourceCoordinates, error) {
	coordinates := SourceCoordinates{
		revision:     revision,
		readmeDigest: readmeDigest,
		specDigest:   specDigest,
	}
	if err := coordinates.validate(); err != nil {
		return SourceCoordinates{}, err
	}
	return coordinates, nil
}

// Revision returns the exact source revision.
func (coordinates SourceCoordinates) Revision() typedmemory.SourceRevision {
	return coordinates.revision
}

// ReadmeDigest returns the digest of the standalone README snapshot.
func (coordinates SourceCoordinates) ReadmeDigest() typedmemory.SHA256Digest {
	return coordinates.readmeDigest
}

// SpecDigest returns the digest of the canonical FPF specification snapshot.
func (coordinates SourceCoordinates) SpecDigest() typedmemory.SHA256Digest {
	return coordinates.specDigest
}

func (coordinates SourceCoordinates) validate() error {
	revision, err := typedmemory.NewSourceRevision(coordinates.revision.String())
	if err != nil || revision != coordinates.revision {
		return fmt.Errorf("source coordinates require an exact source revision")
	}
	if err := requireDigest("README digest", coordinates.readmeDigest); err != nil {
		return err
	}
	if err := requireDigest("FPF specification digest", coordinates.specDigest); err != nil {
		return err
	}
	return nil
}

// DerivedCoordinates identify the verified artifacts derived from source
// coordinates. They are absent only when a candidate is rejected before those
// artifacts can be completed.
type DerivedCoordinates struct {
	sourceUnitCount    uint64
	baseTypeEnvRef     typedmemory.TypeEnvRef
	baseTypeEnvDigest  typedmemory.SHA256Digest
	compilerEdition    typedmemory.CompilerSchemaVersion
	databaseDigest     typedmemory.SHA256Digest
	indexSchemaVersion uint64
}

// NewDerivedCoordinates constructs verified derived-artifact coordinates.
func NewDerivedCoordinates(
	sourceUnitCount uint64,
	baseTypeEnvRef typedmemory.TypeEnvRef,
	baseTypeEnvDigest typedmemory.SHA256Digest,
	compilerEdition typedmemory.CompilerSchemaVersion,
	databaseDigest typedmemory.SHA256Digest,
	indexSchemaVersion uint64,
) (DerivedCoordinates, error) {
	coordinates := DerivedCoordinates{
		sourceUnitCount:    sourceUnitCount,
		baseTypeEnvRef:     baseTypeEnvRef,
		baseTypeEnvDigest:  baseTypeEnvDigest,
		compilerEdition:    compilerEdition,
		databaseDigest:     databaseDigest,
		indexSchemaVersion: indexSchemaVersion,
	}
	if err := coordinates.validate(); err != nil {
		return DerivedCoordinates{}, err
	}
	return coordinates, nil
}

// SourceUnitCount returns the number of projected source units.
func (coordinates DerivedCoordinates) SourceUnitCount() uint64 {
	return coordinates.sourceUnitCount
}

// BaseTypeEnvRef returns the exact Base TypeEnv reference.
func (coordinates DerivedCoordinates) BaseTypeEnvRef() typedmemory.TypeEnvRef {
	return coordinates.baseTypeEnvRef
}

// BaseTypeEnvDigest returns the exact Base TypeEnv digest.
func (coordinates DerivedCoordinates) BaseTypeEnvDigest() typedmemory.SHA256Digest {
	return coordinates.baseTypeEnvDigest
}

// CompilerEdition returns the compiler schema edition.
func (coordinates DerivedCoordinates) CompilerEdition() typedmemory.CompilerSchemaVersion {
	return coordinates.compilerEdition
}

// DatabaseDigest returns the candidate SQLite database digest.
func (coordinates DerivedCoordinates) DatabaseDigest() typedmemory.SHA256Digest {
	return coordinates.databaseDigest
}

// IndexSchemaVersion returns the candidate index schema version.
func (coordinates DerivedCoordinates) IndexSchemaVersion() uint64 {
	return coordinates.indexSchemaVersion
}

func (coordinates DerivedCoordinates) validate() error {
	if coordinates.sourceUnitCount == 0 {
		return fmt.Errorf("derived coordinates require a positive source-unit count")
	}
	typeEnvRef, err := typedmemory.ParseTypeEnvRef(coordinates.baseTypeEnvRef.String())
	if err != nil || typeEnvRef != coordinates.baseTypeEnvRef {
		return fmt.Errorf("derived coordinates require an exact Base TypeEnv reference")
	}
	if err := requireDigest("Base TypeEnv digest", coordinates.baseTypeEnvDigest); err != nil {
		return err
	}
	if coordinates.baseTypeEnvRef.Digest() != coordinates.baseTypeEnvDigest {
		return fmt.Errorf(
			"derived coordinates require matching Base TypeEnv reference and digest",
		)
	}
	compiler, err := typedmemory.NewCompilerSchemaVersion(coordinates.compilerEdition.String())
	if err != nil || compiler != coordinates.compilerEdition {
		return fmt.Errorf("derived coordinates require an exact compiler edition")
	}
	if err := requireDigest("database digest", coordinates.databaseDigest); err != nil {
		return err
	}
	if coordinates.indexSchemaVersion == 0 {
		return fmt.Errorf("derived coordinates require a positive index schema version")
	}
	return nil
}

// PredecessorSnapshot is always a complete, last-known-good source/artifact
// pair.
type PredecessorSnapshot struct {
	source  SourceCoordinates
	derived DerivedCoordinates
}

// NewPredecessorSnapshot constructs a complete predecessor snapshot.
func NewPredecessorSnapshot(
	source SourceCoordinates,
	derived DerivedCoordinates,
) (PredecessorSnapshot, error) {
	if err := source.validate(); err != nil {
		return PredecessorSnapshot{}, fmt.Errorf("predecessor: %w", err)
	}
	if err := derived.validate(); err != nil {
		return PredecessorSnapshot{}, fmt.Errorf("predecessor: %w", err)
	}
	return PredecessorSnapshot{source: source, derived: derived}, nil
}

// Source returns the predecessor source coordinates.
func (snapshot PredecessorSnapshot) Source() SourceCoordinates {
	return snapshot.source
}

// Derived returns the predecessor derived coordinates.
func (snapshot PredecessorSnapshot) Derived() DerivedCoordinates {
	return snapshot.derived
}

func (snapshot PredecessorSnapshot) validate() error {
	if err := snapshot.source.validate(); err != nil {
		return fmt.Errorf("predecessor: %w", err)
	}
	if err := snapshot.derived.validate(); err != nil {
		return fmt.Errorf("predecessor: %w", err)
	}
	return nil
}

// CandidateSnapshot always has exact source coordinates. Derived coordinates
// are present after a successful candidate build and may be absent only for a
// CandidateRejected result.
type CandidateSnapshot struct {
	source         SourceCoordinates
	sourceComplete bool
	derived        *DerivedCoordinates
}

// LocalPracticeCompatibilityResult is the closed technical posture between
// the latest repo-owned Local-Practice candidate and an exact candidate FPF
// Base TypeEnv. It says nothing about activation, admission, approval, or
// semantic binding.
type LocalPracticeCompatibilityResult uint8

const (
	LocalPracticeExact LocalPracticeCompatibilityResult = iota + 1
	LocalPracticeCompatibleSuccessorCandidatePossible
	LocalPracticeSemanticReviewRequired
	LocalPracticeCompilerGap
)

func (result LocalPracticeCompatibilityResult) String() string {
	switch result {
	case LocalPracticeExact:
		return "exact"
	case LocalPracticeCompatibleSuccessorCandidatePossible:
		return "compatible_successor_candidate_possible"
	case LocalPracticeSemanticReviewRequired:
		return "semantic_review_required"
	case LocalPracticeCompilerGap:
		return "compiler_gap"
	default:
		return ""
	}
}

// LocalPracticeCompatibilityAssessment records the exact TypeEnv coordinates
// used to derive one closed Local-Practice compatibility result. A compatible
// successor remains only a possible non-binding candidate.
type LocalPracticeCompatibilityAssessment struct {
	carrierBase     typedmemory.TypeEnvRef
	predecessorBase typedmemory.TypeEnvRef
	candidateBase   typedmemory.TypeEnvRef
	result          LocalPracticeCompatibilityResult
}

// NewLocalPracticeCompatibilityAssessment constructs a technical assessment
// without granting activation, admission, approval, or binding authority.
func NewLocalPracticeCompatibilityAssessment(
	carrierBase typedmemory.TypeEnvRef,
	predecessorBase typedmemory.TypeEnvRef,
	candidateBase typedmemory.TypeEnvRef,
	result LocalPracticeCompatibilityResult,
) (LocalPracticeCompatibilityAssessment, error) {
	assessment := LocalPracticeCompatibilityAssessment{
		carrierBase:     carrierBase,
		predecessorBase: predecessorBase,
		candidateBase:   candidateBase,
		result:          result,
	}
	if err := assessment.validate(); err != nil {
		return LocalPracticeCompatibilityAssessment{}, err
	}
	return assessment, nil
}

// CarrierBase returns the Base TypeEnv pinned by the Local-Practice carrier.
func (assessment LocalPracticeCompatibilityAssessment) CarrierBase() typedmemory.TypeEnvRef {
	return assessment.carrierBase
}

// PredecessorBase returns the exact predecessor FPF Base TypeEnv.
func (assessment LocalPracticeCompatibilityAssessment) PredecessorBase() typedmemory.TypeEnvRef {
	return assessment.predecessorBase
}

// CandidateBase returns the exact candidate FPF Base TypeEnv.
func (assessment LocalPracticeCompatibilityAssessment) CandidateBase() typedmemory.TypeEnvRef {
	return assessment.candidateBase
}

// Result returns the closed technical compatibility posture.
func (assessment LocalPracticeCompatibilityAssessment) Result() LocalPracticeCompatibilityResult {
	return assessment.result
}

func (assessment LocalPracticeCompatibilityAssessment) validate() error {
	for _, coordinate := range []struct {
		label     string
		reference typedmemory.TypeEnvRef
	}{
		{
			label:     "Local-Practice carrier Base TypeEnv",
			reference: assessment.carrierBase,
		},
		{label: "predecessor Base TypeEnv", reference: assessment.predecessorBase},
		{label: "candidate Base TypeEnv", reference: assessment.candidateBase},
	} {
		label := coordinate.label
		reference := coordinate.reference
		canonical, err := typedmemory.ParseTypeEnvRef(reference.String())
		if err != nil || canonical != reference {
			return fmt.Errorf("%s reference is not exact", label)
		}
	}
	if assessment.result.String() == "" {
		return fmt.Errorf("Local-Practice compatibility result is invalid")
	}
	switch assessment.result {
	case LocalPracticeExact:
		if assessment.carrierBase != assessment.candidateBase {
			return fmt.Errorf("exact Local-Practice compatibility requires the candidate Base TypeEnv")
		}
	case LocalPracticeCompatibleSuccessorCandidatePossible,
		LocalPracticeSemanticReviewRequired:
		if assessment.carrierBase != assessment.predecessorBase ||
			assessment.carrierBase == assessment.candidateBase {
			return fmt.Errorf(
				"Local-Practice successor assessment requires a distinct candidate from the carrier predecessor",
			)
		}
	case LocalPracticeCompilerGap:
		if assessment.carrierBase == assessment.candidateBase {
			return fmt.Errorf("compiler gap cannot classify an exact Local-Practice basis")
		}
	}
	return nil
}

// NewCandidateSourceSnapshot constructs a source-only candidate for a
// fail-closed rejection that occurred before derived artifacts were completed.
func NewCandidateSourceSnapshot(source SourceCoordinates) (CandidateSnapshot, error) {
	if err := source.validate(); err != nil {
		return CandidateSnapshot{}, fmt.Errorf("candidate: %w", err)
	}
	return CandidateSnapshot{source: source, sourceComplete: true}, nil
}

// NewCandidateRevisionSnapshot constructs the bounded source observation used
// when a resolved candidate commit is missing or malforms a required
// publication. Document digests are deliberately unavailable rather than
// synthesized from non-existent bytes.
func NewCandidateRevisionSnapshot(
	revision typedmemory.SourceRevision,
) (CandidateSnapshot, error) {
	canonical, err := typedmemory.NewSourceRevision(revision.String())
	if err != nil || canonical != revision {
		return CandidateSnapshot{}, fmt.Errorf("candidate revision must be exact")
	}
	return CandidateSnapshot{
		source:         SourceCoordinates{revision: revision},
		sourceComplete: false,
	}, nil
}

// NewCandidateSnapshot constructs a complete verified candidate snapshot.
func NewCandidateSnapshot(
	source SourceCoordinates,
	derived DerivedCoordinates,
) (CandidateSnapshot, error) {
	if err := source.validate(); err != nil {
		return CandidateSnapshot{}, fmt.Errorf("candidate: %w", err)
	}
	if err := derived.validate(); err != nil {
		return CandidateSnapshot{}, fmt.Errorf("candidate: %w", err)
	}
	owned := derived
	return CandidateSnapshot{
		source:         source,
		sourceComplete: true,
		derived:        &owned,
	}, nil
}

// Source returns the candidate source coordinates.
func (snapshot CandidateSnapshot) Source() SourceCoordinates {
	return snapshot.source
}

// SourceComplete reports whether both required publication digests are known.
func (snapshot CandidateSnapshot) SourceComplete() bool {
	return snapshot.sourceComplete
}

// Derived returns candidate derived coordinates and whether they are present.
func (snapshot CandidateSnapshot) Derived() (DerivedCoordinates, bool) {
	if snapshot.derived == nil {
		return DerivedCoordinates{}, false
	}
	return *snapshot.derived, true
}

// Complete reports whether all derived candidate coordinates are present.
func (snapshot CandidateSnapshot) Complete() bool {
	return snapshot.derived != nil
}

func (snapshot CandidateSnapshot) validate() error {
	if !snapshot.sourceComplete {
		revision, err := typedmemory.NewSourceRevision(snapshot.source.revision.String())
		if err != nil || revision != snapshot.source.revision {
			return fmt.Errorf("candidate: source revision is required and must be exact")
		}
		if snapshot.source.readmeDigest.String() != "" ||
			snapshot.source.specDigest.String() != "" ||
			snapshot.derived != nil {
			return fmt.Errorf(
				"candidate: incomplete source cannot carry document or derived coordinates",
			)
		}
		return nil
	}
	if err := snapshot.source.validate(); err != nil {
		return fmt.Errorf("candidate: %w", err)
	}
	if snapshot.derived == nil {
		return nil
	}
	if err := snapshot.derived.validate(); err != nil {
		return fmt.Errorf("candidate: %w", err)
	}
	return nil
}

// DeltaFamily is the closed set of compatibility-report change families.
type DeltaFamily uint8

const (
	DeltaSourceContent DeltaFamily = iota + 1
	DeltaSourceIdentity
	DeltaPublicationGrammar
	DeltaPracticalUseCards
	DeltaPatternIDs
	DeltaToCRelations
	DeltaPracticalCardDirectRefs
	DeltaSourceRoles
	DeltaBaseTypeEnv
	DeltaQueryBehavior
	DeltaTokenBudgetCorpus
	DeltaLocalPracticeBasis
	DeltaSpecCarrierReferences
	DeltaDerivationToolchain
)

func (family DeltaFamily) String() string {
	switch family {
	case DeltaSourceContent:
		return "source_content"
	case DeltaSourceIdentity:
		return "source_identity"
	case DeltaPublicationGrammar:
		return "publication_grammar"
	case DeltaPracticalUseCards:
		return "practical_use_cards"
	case DeltaPatternIDs:
		return "pattern_ids"
	case DeltaToCRelations:
		return "toc_relation_projection"
	case DeltaPracticalCardDirectRefs:
		return "direct_practical_card_refs"
	case DeltaSourceRoles:
		return "source_roles"
	case DeltaBaseTypeEnv:
		return "base_type_env"
	case DeltaQueryBehavior:
		return "query_behavior_fixtures"
	case DeltaTokenBudgetCorpus:
		return "token_budget_corpus"
	case DeltaLocalPracticeBasis:
		return "local_practice_base_type_env_pins"
	case DeltaSpecCarrierReferences:
		return "spec_carrier_source_references"
	case DeltaDerivationToolchain:
		return "derivation_toolchain"
	default:
		return ""
	}
}

// DeltaKind is the closed set of exact change classifications.
type DeltaKind uint8

const (
	DeltaContentOnlyCompatible DeltaKind = iota + 1
	DeltaSourceIdentityChanged
	DeltaPublicationGrammarExtended
	DeltaPracticalUseCardAdded
	DeltaPracticalUseCardRemoved
	DeltaPracticalUseCardChanged
	DeltaPracticalUseCardSplit
	DeltaPatternIDAdded
	DeltaPatternIDRemoved
	DeltaToCRelationChanged
	DeltaPracticalCardDirectRefsChanged
	DeltaSourceRoleChanged
	DeltaTypeEnvChanged
	DeltaTypeEnvAdditive
	DeltaTypeEnvNarrowed
	DeltaTypeEnvRemoved
	DeltaTypeEnvCompilerGap
	DeltaQueryExpectationChanged
	DeltaTokenBudgetCorpusChanged
	DeltaLocalPracticeBasisChanged
	DeltaSpecSemanticReviewRequired
	DeltaDerivationToolChanged
)

func (kind DeltaKind) String() string {
	switch kind {
	case DeltaContentOnlyCompatible:
		return "content_only_compatible"
	case DeltaSourceIdentityChanged:
		return "source_identity_changed"
	case DeltaPublicationGrammarExtended:
		return "publication_grammar_extended"
	case DeltaPracticalUseCardAdded:
		return "practical_use_card_added"
	case DeltaPracticalUseCardRemoved:
		return "practical_use_card_removed"
	case DeltaPracticalUseCardChanged:
		return "practical_use_card_changed"
	case DeltaPracticalUseCardSplit:
		return "practical_use_card_split"
	case DeltaPatternIDAdded:
		return "pattern_id_added"
	case DeltaPatternIDRemoved:
		return "pattern_id_removed"
	case DeltaToCRelationChanged:
		return "toc_relation_projection_changed"
	case DeltaPracticalCardDirectRefsChanged:
		return "direct_practical_card_refs_changed"
	case DeltaSourceRoleChanged:
		return "source_role_changed"
	case DeltaTypeEnvChanged:
		return "typeenv_changed"
	case DeltaTypeEnvAdditive:
		return "typeenv_additive"
	case DeltaTypeEnvNarrowed:
		return "typeenv_narrowed"
	case DeltaTypeEnvRemoved:
		return "typeenv_removed"
	case DeltaTypeEnvCompilerGap:
		return "typeenv_compiler_gap"
	case DeltaQueryExpectationChanged:
		return "query_expectation_changed"
	case DeltaTokenBudgetCorpusChanged:
		return "token_budget_corpus_changed"
	case DeltaLocalPracticeBasisChanged:
		return "local_practice_basis_changed"
	case DeltaSpecSemanticReviewRequired:
		return "spec_semantic_review_required"
	case DeltaDerivationToolChanged:
		return "derivation_tool_changed"
	default:
		return ""
	}
}

// Delta is one exact before/after observation in a declared family.
type Delta struct {
	family    DeltaFamily
	kind      DeltaKind
	subject   string
	before    string
	after     string
	sourceRef string
}

// NewDelta constructs one exact compatibility delta.
func NewDelta(
	family DeltaFamily,
	kind DeltaKind,
	subject string,
	before string,
	after string,
	sourceRef string,
) (Delta, error) {
	delta := Delta{
		family:    family,
		kind:      kind,
		subject:   subject,
		before:    before,
		after:     after,
		sourceRef: sourceRef,
	}
	if _, err := delta.disposition(); err != nil {
		return Delta{}, err
	}
	if err := requireLine("delta subject", delta.subject, maxCoordinateText, true); err != nil {
		return Delta{}, err
	}
	if err := requireLine("delta before", delta.before, maxCoordinateText, false); err != nil {
		return Delta{}, err
	}
	if err := requireLine("delta after", delta.after, maxCoordinateText, false); err != nil {
		return Delta{}, err
	}
	if delta.before == delta.after {
		return Delta{}, fmt.Errorf("delta before and after values must differ")
	}
	if delta.before == "" && delta.after == "" {
		return Delta{}, fmt.Errorf("delta requires a before or after value")
	}
	if err := requireLine("delta source reference", delta.sourceRef, maxCoordinateText, false); err != nil {
		return Delta{}, err
	}
	return delta, nil
}

// Family returns the delta family.
func (delta Delta) Family() DeltaFamily { return delta.family }

// Kind returns the exact delta classification.
func (delta Delta) Kind() DeltaKind { return delta.kind }

// Subject returns the exact changed subject.
func (delta Delta) Subject() string { return delta.subject }

// Before returns the predecessor value, or an empty string for an addition.
func (delta Delta) Before() string { return delta.before }

// After returns the candidate value, or an empty string for a removal.
func (delta Delta) After() string { return delta.after }

// SourceRef returns the optional exact source trace.
func (delta Delta) SourceRef() string { return delta.sourceRef }

type resultDisposition uint8

const (
	dispositionApply resultDisposition = iota + 1
	dispositionReview
	dispositionReject
)

func (delta Delta) disposition() (resultDisposition, error) {
	switch delta.family {
	case DeltaSourceContent:
		if delta.kind == DeltaContentOnlyCompatible {
			return dispositionApply, nil
		}
	case DeltaSourceIdentity:
		if delta.kind == DeltaSourceIdentityChanged {
			return dispositionApply, nil
		}
	case DeltaPublicationGrammar:
		if delta.kind == DeltaPublicationGrammarExtended {
			return dispositionReview, nil
		}
	case DeltaPracticalUseCards:
		switch delta.kind {
		case DeltaPracticalUseCardAdded,
			DeltaPracticalUseCardRemoved,
			DeltaPracticalUseCardChanged,
			DeltaPracticalUseCardSplit:
			return dispositionReview, nil
		}
	case DeltaPatternIDs:
		switch delta.kind {
		case DeltaPatternIDAdded, DeltaPatternIDRemoved:
			return dispositionReview, nil
		}
	case DeltaToCRelations:
		if delta.kind == DeltaToCRelationChanged {
			return dispositionReview, nil
		}
	case DeltaPracticalCardDirectRefs:
		if delta.kind == DeltaPracticalCardDirectRefsChanged {
			return dispositionReview, nil
		}
	case DeltaSourceRoles:
		if delta.kind == DeltaSourceRoleChanged {
			return dispositionReview, nil
		}
	case DeltaBaseTypeEnv:
		switch delta.kind {
		case DeltaTypeEnvChanged,
			DeltaTypeEnvAdditive,
			DeltaTypeEnvNarrowed,
			DeltaTypeEnvRemoved,
			DeltaTypeEnvCompilerGap:
			return dispositionReview, nil
		}
	case DeltaQueryBehavior:
		if delta.kind == DeltaQueryExpectationChanged {
			return dispositionReview, nil
		}
	case DeltaTokenBudgetCorpus:
		if delta.kind == DeltaTokenBudgetCorpusChanged {
			return dispositionReview, nil
		}
	case DeltaLocalPracticeBasis:
		if delta.kind == DeltaLocalPracticeBasisChanged {
			return dispositionReview, nil
		}
	case DeltaSpecCarrierReferences:
		if delta.kind == DeltaSpecSemanticReviewRequired {
			return dispositionReview, nil
		}
	case DeltaDerivationToolchain:
		if delta.kind == DeltaDerivationToolChanged {
			return dispositionApply, nil
		}
	}
	return 0, fmt.Errorf(
		"unsupported refresh delta family/kind pair %q/%q",
		delta.family.String(),
		delta.kind.String(),
	)
}

// DiagnosticCode is the closed set of compatibility-check diagnostics. Apply
// interruption is intentionally absent: it belongs to the apply/re-entry
// RecoveryRequired algebra, not this compatibility report.
type DiagnosticCode uint8

const (
	DiagnosticSourcePublicationMalformed DiagnosticCode = iota + 1
	DiagnosticAdapterGrammarUnsupported
	DiagnosticSourceProjectionDegraded
	DiagnosticSourceStructureCollapse
	DiagnosticSourceReferenceUnresolved
	DiagnosticTypeEnvSemanticRejection
	DiagnosticTypeEnvCompatibilityReviewRequired
	DiagnosticTypeEnvCompilerGap
	DiagnosticQueryContractRegression
	DiagnosticSnapshotPinStale
	DiagnosticLocalPracticeRebaseRequired
	DiagnosticCandidateVerificationFailed
	DiagnosticTokenGateFailed
)

func (code DiagnosticCode) String() string {
	switch code {
	case DiagnosticSourcePublicationMalformed:
		return "source_publication_malformed"
	case DiagnosticAdapterGrammarUnsupported:
		return "adapter_grammar_unsupported"
	case DiagnosticSourceProjectionDegraded:
		return "source_projection_degraded"
	case DiagnosticSourceStructureCollapse:
		return "source_structure_collapse"
	case DiagnosticSourceReferenceUnresolved:
		return "source_reference_unresolved"
	case DiagnosticTypeEnvSemanticRejection:
		return "typeenv_semantic_rejection"
	case DiagnosticTypeEnvCompatibilityReviewRequired:
		return "typeenv_compatibility_review_required"
	case DiagnosticTypeEnvCompilerGap:
		return "typeenv_compiler_gap"
	case DiagnosticQueryContractRegression:
		return "query_contract_regression"
	case DiagnosticSnapshotPinStale:
		return "snapshot_pin_stale"
	case DiagnosticLocalPracticeRebaseRequired:
		return "local_practice_rebase_required"
	case DiagnosticCandidateVerificationFailed:
		return "candidate_verification_failed"
	case DiagnosticTokenGateFailed:
		return "token_gate_failed"
	default:
		return ""
	}
}

func (code DiagnosticCode) disposition() (resultDisposition, error) {
	switch code {
	case DiagnosticTypeEnvCompatibilityReviewRequired,
		DiagnosticAdapterGrammarUnsupported,
		DiagnosticSourceProjectionDegraded,
		DiagnosticTypeEnvCompilerGap,
		DiagnosticQueryContractRegression,
		DiagnosticSnapshotPinStale,
		DiagnosticLocalPracticeRebaseRequired,
		DiagnosticTokenGateFailed:
		return dispositionReview, nil
	case DiagnosticSourcePublicationMalformed,
		DiagnosticSourceStructureCollapse,
		DiagnosticSourceReferenceUnresolved,
		DiagnosticTypeEnvSemanticRejection,
		DiagnosticCandidateVerificationFailed:
		return dispositionReject, nil
	default:
		return 0, fmt.Errorf("unsupported refresh diagnostic code %d", code)
	}
}

// Diagnostic describes one exact compatibility-check failure or review need.
type Diagnostic struct {
	code             DiagnosticCode
	subject          string
	message          string
	sourceRef        string
	reproduceCommand string
}

// NewDiagnostic constructs one exact compatibility diagnostic.
func NewDiagnostic(
	code DiagnosticCode,
	subject string,
	message string,
	sourceRef string,
	reproduceCommand string,
) (Diagnostic, error) {
	if _, err := code.disposition(); err != nil {
		return Diagnostic{}, err
	}
	diagnostic := Diagnostic{
		code:             code,
		subject:          subject,
		message:          message,
		sourceRef:        sourceRef,
		reproduceCommand: reproduceCommand,
	}
	if err := requireLine("diagnostic subject", diagnostic.subject, maxCoordinateText, true); err != nil {
		return Diagnostic{}, err
	}
	if err := requireLine("diagnostic message", diagnostic.message, maxMessageText, true); err != nil {
		return Diagnostic{}, err
	}
	if err := requireLine("diagnostic source reference", diagnostic.sourceRef, maxCoordinateText, false); err != nil {
		return Diagnostic{}, err
	}
	if err := requireLine("diagnostic reproduce command", diagnostic.reproduceCommand, maxMessageText, false); err != nil {
		return Diagnostic{}, err
	}
	return diagnostic, nil
}

// Code returns the stable diagnostic code.
func (diagnostic Diagnostic) Code() DiagnosticCode { return diagnostic.code }

// Subject returns the exact affected subject.
func (diagnostic Diagnostic) Subject() string { return diagnostic.subject }

// Message returns the readable diagnostic.
func (diagnostic Diagnostic) Message() string { return diagnostic.message }

// SourceRef returns the optional source trace.
func (diagnostic Diagnostic) SourceRef() string { return diagnostic.sourceRef }

// ReproduceCommand returns the optional focused reproduction command.
func (diagnostic Diagnostic) ReproduceCommand() string {
	return diagnostic.reproduceCommand
}

// Stage identifies one measured refresh stage.
type Stage uint8

const (
	StageFetch Stage = iota + 1
	StageSourceObjectRead
	StageAtlasParse
	StageSourceUnitProjection
	StageTypeEnvCompile
	StageCompatibilityComparison
	StageSQLiteBuild
	StageVerification
	StageQuerySmoke
	StageTokenGate
	StageApply
)

func (stage Stage) String() string {
	switch stage {
	case StageFetch:
		return "fetch"
	case StageSourceObjectRead:
		return "source_object_read"
	case StageAtlasParse:
		return "atlas_parse"
	case StageSourceUnitProjection:
		return "source_unit_projection"
	case StageTypeEnvCompile:
		return "typeenv_compile"
	case StageCompatibilityComparison:
		return "compatibility_comparison"
	case StageSQLiteBuild:
		return "sqlite_build"
	case StageVerification:
		return "verification"
	case StageQuerySmoke:
		return "query_smoke"
	case StageTokenGate:
		return "token_gate"
	case StageApply:
		return "apply"
	default:
		return ""
	}
}

// StageTiming is one non-negative measured duration.
type StageTiming struct {
	stage    Stage
	duration time.Duration
}

// NewStageTiming constructs one measured stage duration.
func NewStageTiming(stage Stage, duration time.Duration) (StageTiming, error) {
	if stage.String() == "" {
		return StageTiming{}, fmt.Errorf("refresh timing stage is unsupported")
	}
	if duration < 0 {
		return StageTiming{}, fmt.Errorf("refresh timing duration must be non-negative")
	}
	return StageTiming{stage: stage, duration: duration}, nil
}

// Stage returns the measured stage.
func (timing StageTiming) Stage() Stage { return timing.stage }

// Duration returns the exact measured duration.
func (timing StageTiming) Duration() time.Duration { return timing.duration }

// CheckState is the closed compatibility-check state vocabulary.
type CheckState uint8

const (
	StateNoChange CheckState = iota + 1
	StateCandidateRejected
	StateReviewReady
	StateApplyReady
)

func (state CheckState) String() string {
	switch state {
	case StateNoChange:
		return "no_change"
	case StateCandidateRejected:
		return "candidate_rejected"
	case StateReviewReady:
		return "review_ready"
	case StateApplyReady:
		return "apply_ready"
	default:
		return ""
	}
}

// CheckOutcome is sealed to the four compatibility-check variants below.
// RecoveryRequired is deliberately not a variant: it belongs only to the
// apply/re-entry boundary.
type CheckOutcome interface {
	State() CheckState
	checkOutcomeVariant()
}

// NoChange means the exact complete predecessor and candidate snapshots agree.
type NoChange struct{}

// State returns StateNoChange.
func (NoChange) State() CheckState    { return StateNoChange }
func (NoChange) checkOutcomeVariant() {}

// CandidateRejected means the source could not produce one complete,
// structurally supported, deterministically verified candidate publication.
type CandidateRejected struct{}

// State returns StateCandidateRejected.
func (CandidateRejected) State() CheckState    { return StateCandidateRejected }
func (CandidateRejected) checkOutcomeVariant() {}

// ReviewReady means a complete verified candidate has non-blocking parser,
// semantic, query-behavior, token-budget, or expectation findings to review.
// It does not mean approval or authorization, and it does not veto
// source-current apply.
type ReviewReady struct{}

// State returns StateReviewReady.
func (ReviewReady) State() CheckState    { return StateReviewReady }
func (ReviewReady) checkOutcomeVariant() {}

// ApplyReady means the predeclared technical compatibility policy found only
// apply-safe delta classes. It grants no permission to perform effects.
type ApplyReady struct{}

// State returns StateApplyReady.
func (ApplyReady) State() CheckState    { return StateApplyReady }
func (ApplyReady) checkOutcomeVariant() {}

// Report is an immutable, canonical compatibility-check result. Its closed
// schema contains observations and timings only; it has no authority-bearing
// or semantic-selection fields.
type Report struct {
	toolRevision               typedmemory.SourceRevision
	predecessor                PredecessorSnapshot
	candidate                  CandidateSnapshot
	localPracticeCompatibility *LocalPracticeCompatibilityAssessment
	outcome                    CheckOutcome
	deltas                     []Delta
	diagnostics                []Diagnostic
	timings                    []StageTiming
	canonicalBytes             []byte
	digest                     typedmemory.SHA256Digest
}

// NewReport validates and canonically orders observations for a fail-closed
// candidate rejection that occurred before derived coordinates and the
// Local-Practice assessment were available. Complete candidate reports are
// constructed through BuildCompatibilityReport, which supplies the mandatory
// closed Local-Practice assessment.
func NewReport(
	toolRevision typedmemory.SourceRevision,
	predecessor PredecessorSnapshot,
	candidate CandidateSnapshot,
	deltas []Delta,
	diagnostics []Diagnostic,
	timings []StageTiming,
) (Report, error) {
	return newReport(
		toolRevision,
		predecessor,
		candidate,
		nil,
		deltas,
		diagnostics,
		timings,
	)
}

func newReport(
	toolRevision typedmemory.SourceRevision,
	predecessor PredecessorSnapshot,
	candidate CandidateSnapshot,
	localPracticeCompatibility *LocalPracticeCompatibilityAssessment,
	deltas []Delta,
	diagnostics []Diagnostic,
	timings []StageTiming,
) (Report, error) {
	revision, err := typedmemory.NewSourceRevision(toolRevision.String())
	if err != nil || revision != toolRevision {
		return Report{}, fmt.Errorf("refresh report requires an exact tool revision")
	}
	if err := predecessor.validate(); err != nil {
		return Report{}, err
	}
	if err := candidate.validate(); err != nil {
		return Report{}, err
	}
	if candidate.derived != nil && localPracticeCompatibility == nil {
		return Report{}, ErrMissingLocalPracticeAssessment
	}
	if localPracticeCompatibility != nil {
		if err := localPracticeCompatibility.validate(); err != nil {
			return Report{}, err
		}
		if localPracticeCompatibility.predecessorBase != predecessor.derived.baseTypeEnvRef {
			return Report{}, fmt.Errorf(
				"Local-Practice assessment predecessor Base TypeEnv differs from report predecessor",
			)
		}
		if candidate.derived == nil ||
			localPracticeCompatibility.candidateBase != candidate.derived.baseTypeEnvRef {
			return Report{}, fmt.Errorf(
				"Local-Practice assessment candidate Base TypeEnv differs from report candidate",
			)
		}
	}
	normalizedDeltas, err := normalizeDeltas(deltas)
	if err != nil {
		return Report{}, err
	}
	normalizedDiagnostics, err := normalizeDiagnostics(diagnostics)
	if err != nil {
		return Report{}, err
	}
	normalizedTimings, err := normalizeTimings(timings)
	if err != nil {
		return Report{}, err
	}
	outcome, err := deriveOutcome(
		predecessor,
		candidate,
		normalizedDeltas,
		normalizedDiagnostics,
	)
	if err != nil {
		return Report{}, err
	}
	canonical, err := json.Marshal(reportDTOOf(
		toolRevision,
		predecessor,
		candidate,
		localPracticeCompatibility,
		outcome,
		normalizedDeltas,
		normalizedDiagnostics,
		normalizedTimings,
	))
	if err != nil {
		return Report{}, fmt.Errorf("encode canonical FPF refresh report: %w", err)
	}
	digest, err := digestBytes(canonical)
	if err != nil {
		return Report{}, err
	}
	return Report{
		toolRevision:               toolRevision,
		predecessor:                predecessor,
		candidate:                  candidate,
		localPracticeCompatibility: copyLocalPracticeCompatibility(localPracticeCompatibility),
		outcome:                    outcome,
		deltas:                     normalizedDeltas,
		diagnostics:                normalizedDiagnostics,
		timings:                    normalizedTimings,
		canonicalBytes:             canonical,
		digest:                     digest,
	}, nil
}

// Schema returns the canonical schema identifier.
func (Report) Schema() string { return ReportSchemaV1 }

// ToolRevision returns the report-generator source revision.
func (report Report) ToolRevision() typedmemory.SourceRevision {
	return report.toolRevision
}

// Predecessor returns the last-known-good predecessor snapshot.
func (report Report) Predecessor() PredecessorSnapshot {
	return report.predecessor
}

// Candidate returns the candidate snapshot.
func (report Report) Candidate() CandidateSnapshot {
	return copyCandidate(report.candidate)
}

// LocalPracticeCompatibility returns the closed technical assessment when a
// repo-owned Local-Practice candidate was part of this report's inputs.
func (report Report) LocalPracticeCompatibility() (
	LocalPracticeCompatibilityAssessment,
	bool,
) {
	if report.localPracticeCompatibility == nil {
		return LocalPracticeCompatibilityAssessment{}, false
	}
	return *report.localPracticeCompatibility, true
}

// Outcome returns the sealed compatibility-check outcome.
func (report Report) Outcome() CheckOutcome { return report.outcome }

// Deltas returns a copy of the canonically ordered deltas.
func (report Report) Deltas() []Delta {
	return append([]Delta(nil), report.deltas...)
}

// Diagnostics returns a copy of the canonically ordered diagnostics.
func (report Report) Diagnostics() []Diagnostic {
	return append([]Diagnostic(nil), report.diagnostics...)
}

// Timings returns a copy of the timings in the fixed stage order.
func (report Report) Timings() []StageTiming {
	return append([]StageTiming(nil), report.timings...)
}

// CanonicalBytes returns a copy of the deterministic report payload.
func (report Report) CanonicalBytes() []byte {
	return append([]byte(nil), report.canonicalBytes...)
}

// Digest returns the SHA-256 digest of CanonicalBytes.
func (report Report) Digest() typedmemory.SHA256Digest { return report.digest }

// MarshalJSON emits the same canonical bytes returned by CanonicalBytes.
func (report Report) MarshalJSON() ([]byte, error) {
	if err := report.Verify(); err != nil {
		return nil, err
	}
	return report.CanonicalBytes(), nil
}

// Verify reconstructs the report and rejects internal drift.
func (report Report) Verify() error {
	rebuilt, err := newReport(
		report.toolRevision,
		report.predecessor,
		report.candidate,
		report.localPracticeCompatibility,
		report.deltas,
		report.diagnostics,
		report.timings,
	)
	if err != nil {
		return err
	}
	if !bytes.Equal(rebuilt.canonicalBytes, report.canonicalBytes) ||
		rebuilt.digest != report.digest ||
		rebuilt.outcome.State() != report.outcome.State() {
		return fmt.Errorf("FPF refresh report state is inconsistent")
	}
	return nil
}

// Readable returns a deterministic terminal-oriented rendering.
func (report Report) Readable() string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "schema: %s\n", ReportSchemaV1)
	fmt.Fprintf(&builder, "tool_revision: %s\n", report.toolRevision.String())
	fmt.Fprintf(&builder, "result: %s\n", report.outcome.State().String())
	writeReadableSnapshot(
		&builder,
		"predecessor",
		report.predecessor.source,
		true,
		&report.predecessor.derived,
	)
	writeReadableSnapshot(
		&builder,
		"candidate",
		report.candidate.source,
		report.candidate.sourceComplete,
		report.candidate.derived,
	)
	if assessment := report.localPracticeCompatibility; assessment != nil {
		fmt.Fprintf(
			&builder,
			"local_practice_compatibility: %s (carrier=%s predecessor=%s candidate=%s)\n",
			assessment.result.String(),
			assessment.carrierBase.String(),
			assessment.predecessorBase.String(),
			assessment.candidateBase.String(),
		)
	}
	fmt.Fprintf(&builder, "deltas: %d\n", len(report.deltas))
	for _, delta := range report.deltas {
		fmt.Fprintf(
			&builder,
			"  - %s/%s %s: %q -> %q",
			delta.family.String(),
			delta.kind.String(),
			delta.subject,
			delta.before,
			delta.after,
		)
		if delta.sourceRef != "" {
			fmt.Fprintf(&builder, " [%s]", delta.sourceRef)
		}
		builder.WriteByte('\n')
	}
	fmt.Fprintf(&builder, "diagnostics: %d\n", len(report.diagnostics))
	for _, diagnostic := range report.diagnostics {
		fmt.Fprintf(
			&builder,
			"  - %s %s: %s",
			diagnostic.code.String(),
			diagnostic.subject,
			diagnostic.message,
		)
		if diagnostic.sourceRef != "" {
			fmt.Fprintf(&builder, " [%s]", diagnostic.sourceRef)
		}
		if diagnostic.reproduceCommand != "" {
			fmt.Fprintf(&builder, " (reproduce: %s)", diagnostic.reproduceCommand)
		}
		builder.WriteByte('\n')
	}
	fmt.Fprintf(&builder, "timings: %d\n", len(report.timings))
	for _, timing := range report.timings {
		fmt.Fprintf(
			&builder,
			"  - %s: %s\n",
			timing.stage.String(),
			timing.duration.String(),
		)
	}
	return builder.String()
}

func deriveOutcome(
	predecessor PredecessorSnapshot,
	candidate CandidateSnapshot,
	deltas []Delta,
	diagnostics []Diagnostic,
) (CheckOutcome, error) {
	hasReview := false
	hasReject := false
	hasRejectDiagnostic := false
	for _, delta := range deltas {
		disposition, err := delta.disposition()
		if err != nil {
			return nil, err
		}
		switch disposition {
		case dispositionReview:
			hasReview = true
		case dispositionReject:
			hasReject = true
		}
	}
	for _, diagnostic := range diagnostics {
		disposition, err := diagnostic.code.disposition()
		if err != nil {
			return nil, err
		}
		switch disposition {
		case dispositionReview:
			hasReview = true
		case dispositionReject:
			hasReject = true
			hasRejectDiagnostic = true
		}
	}
	if hasReject {
		if !hasRejectDiagnostic {
			return nil, fmt.Errorf("candidate rejection requires an exact rejecting diagnostic")
		}
		return CandidateRejected{}, nil
	}
	if !candidate.Complete() {
		return nil, ErrIncompleteCandidate
	}
	if hasReview {
		return ReviewReady{}, nil
	}
	if sameCompleteSnapshot(predecessor, candidate) {
		if len(deltas) == 0 && len(diagnostics) == 0 {
			return NoChange{}, nil
		}
		// Generated-lock inputs such as the exact derivation tool identity can
		// change while all source/DB coordinates remain byte-identical.
		return ApplyReady{}, nil
	}
	if len(deltas) == 0 && len(diagnostics) == 0 {
		return nil, ErrUnexplainedCandidateChange
	}
	return ApplyReady{}, nil
}

func sameCompleteSnapshot(
	predecessor PredecessorSnapshot,
	candidate CandidateSnapshot,
) bool {
	if candidate.derived == nil {
		return false
	}
	return candidate.sourceComplete &&
		predecessor.source == candidate.source &&
		predecessor.derived == *candidate.derived
}

func normalizeDeltas(values []Delta) ([]Delta, error) {
	result := append([]Delta(nil), values...)
	for index, delta := range result {
		rebuilt, err := NewDelta(
			delta.family,
			delta.kind,
			delta.subject,
			delta.before,
			delta.after,
			delta.sourceRef,
		)
		if err != nil || rebuilt != delta {
			if err == nil {
				err = fmt.Errorf("delta is not canonical")
			}
			return nil, fmt.Errorf("delta %d: %w", index, err)
		}
	}
	sort.Slice(result, func(left, right int) bool {
		return deltaSortKey(result[left]) < deltaSortKey(result[right])
	})
	seen := map[string]struct{}{}
	for _, delta := range result {
		key := delta.family.String() + "\x00" + delta.subject
		if _, exists := seen[key]; exists {
			return nil, fmt.Errorf(
				"%w: %s/%s",
				ErrDuplicateDelta,
				delta.family.String(),
				delta.subject,
			)
		}
		seen[key] = struct{}{}
	}
	return result, nil
}

func deltaSortKey(delta Delta) string {
	return strings.Join([]string{
		delta.family.String(),
		delta.kind.String(),
		delta.subject,
		delta.before,
		delta.after,
		delta.sourceRef,
	}, "\x00")
}

func normalizeDiagnostics(values []Diagnostic) ([]Diagnostic, error) {
	result := append([]Diagnostic(nil), values...)
	for index, diagnostic := range result {
		rebuilt, err := NewDiagnostic(
			diagnostic.code,
			diagnostic.subject,
			diagnostic.message,
			diagnostic.sourceRef,
			diagnostic.reproduceCommand,
		)
		if err != nil || rebuilt != diagnostic {
			if err == nil {
				err = fmt.Errorf("diagnostic is not canonical")
			}
			return nil, fmt.Errorf("diagnostic %d: %w", index, err)
		}
	}
	sort.Slice(result, func(left, right int) bool {
		return diagnosticSortKey(result[left]) < diagnosticSortKey(result[right])
	})
	seen := map[string]struct{}{}
	for _, diagnostic := range result {
		key := diagnosticSortKey(diagnostic)
		if _, exists := seen[key]; exists {
			return nil, fmt.Errorf(
				"%w: %s/%s",
				ErrDuplicateDiagnostic,
				diagnostic.code.String(),
				diagnostic.subject,
			)
		}
		seen[key] = struct{}{}
	}
	return result, nil
}

func diagnosticSortKey(diagnostic Diagnostic) string {
	return strings.Join([]string{
		diagnostic.code.String(),
		diagnostic.subject,
		diagnostic.sourceRef,
		diagnostic.message,
		diagnostic.reproduceCommand,
	}, "\x00")
}

func normalizeTimings(values []StageTiming) ([]StageTiming, error) {
	result := append([]StageTiming(nil), values...)
	for index, timing := range result {
		rebuilt, err := NewStageTiming(timing.stage, timing.duration)
		if err != nil || rebuilt != timing {
			if err == nil {
				err = fmt.Errorf("stage timing is not canonical")
			}
			return nil, fmt.Errorf("stage timing %d: %w", index, err)
		}
	}
	sort.Slice(result, func(left, right int) bool {
		return result[left].stage < result[right].stage
	})
	seen := map[Stage]struct{}{}
	for _, timing := range result {
		if _, exists := seen[timing.stage]; exists {
			return nil, fmt.Errorf(
				"%w: %s",
				ErrDuplicateStageTiming,
				timing.stage.String(),
			)
		}
		seen[timing.stage] = struct{}{}
	}
	return result, nil
}

func copyCandidate(candidate CandidateSnapshot) CandidateSnapshot {
	if candidate.derived == nil {
		return candidate
	}
	derived := *candidate.derived
	candidate.derived = &derived
	return candidate
}

func copyLocalPracticeCompatibility(
	assessment *LocalPracticeCompatibilityAssessment,
) *LocalPracticeCompatibilityAssessment {
	if assessment == nil {
		return nil
	}
	copy := *assessment
	return &copy
}

func requireDigest(label string, digest typedmemory.SHA256Digest) error {
	canonical, err := typedmemory.NewSHA256Digest(digest.String())
	if err != nil || canonical != digest {
		return fmt.Errorf("%s is required and must be canonical", label)
	}
	return nil
}

func requireLine(label, value string, maximum int, required bool) error {
	if required && value == "" {
		return fmt.Errorf("%s is required", label)
	}
	if value != strings.TrimSpace(value) {
		return fmt.Errorf("%s must not have surrounding whitespace", label)
	}
	if strings.ContainsAny(value, "\r\n\x00") {
		return fmt.Errorf("%s must be one line without NUL", label)
	}
	if len(value) > maximum {
		return fmt.Errorf("%s exceeds %d bytes", label, maximum)
	}
	return nil
}

func digestBytes(value []byte) (typedmemory.SHA256Digest, error) {
	sum := sha256.Sum256(value)
	digest, err := typedmemory.NewSHA256Digest("sha256:" + hex.EncodeToString(sum[:]))
	if err != nil {
		return typedmemory.SHA256Digest{}, fmt.Errorf("build refresh report digest: %w", err)
	}
	return digest, nil
}

type sourceCoordinatesDTO struct {
	Revision     string `json:"revision"`
	ReadmeDigest string `json:"readme_digest,omitempty"`
	SpecDigest   string `json:"spec_digest,omitempty"`
}

type derivedCoordinatesDTO struct {
	SourceUnitCount    uint64 `json:"source_unit_count"`
	BaseTypeEnvRef     string `json:"base_type_env_ref"`
	BaseTypeEnvDigest  string `json:"base_type_env_digest"`
	CompilerEdition    string `json:"compiler_edition"`
	DatabaseDigest     string `json:"database_digest"`
	IndexSchemaVersion uint64 `json:"index_schema_version"`
}

type snapshotDTO struct {
	Source  sourceCoordinatesDTO   `json:"source"`
	Derived *derivedCoordinatesDTO `json:"derived,omitempty"`
}

type resultDTO struct {
	State string `json:"state"`
}

type localPracticeCompatibilityDTO struct {
	Result             string `json:"result"`
	CarrierBaseRef     string `json:"carrier_base_type_env_ref"`
	PredecessorBaseRef string `json:"predecessor_base_type_env_ref"`
	CandidateBaseRef   string `json:"candidate_base_type_env_ref"`
}

type deltaDTO struct {
	Family    string `json:"family"`
	Kind      string `json:"kind"`
	Subject   string `json:"subject"`
	Before    string `json:"before,omitempty"`
	After     string `json:"after,omitempty"`
	SourceRef string `json:"source_ref,omitempty"`
}

type diagnosticDTO struct {
	Code             string `json:"code"`
	Subject          string `json:"subject"`
	Message          string `json:"message"`
	SourceRef        string `json:"source_ref,omitempty"`
	ReproduceCommand string `json:"reproduce_command,omitempty"`
}

type stageTimingDTO struct {
	Stage         string `json:"stage"`
	DurationNanos int64  `json:"duration_nanos"`
}

type reportDTO struct {
	Schema                     string                         `json:"schema"`
	ToolRevision               string                         `json:"tool_revision"`
	Result                     resultDTO                      `json:"result"`
	Predecessor                snapshotDTO                    `json:"predecessor"`
	Candidate                  snapshotDTO                    `json:"candidate"`
	LocalPracticeCompatibility *localPracticeCompatibilityDTO `json:"local_practice_compatibility,omitempty"`
	Deltas                     []deltaDTO                     `json:"deltas"`
	Diagnostics                []diagnosticDTO                `json:"diagnostics"`
	Timings                    []stageTimingDTO               `json:"timings"`
}

func reportDTOOf(
	toolRevision typedmemory.SourceRevision,
	predecessor PredecessorSnapshot,
	candidate CandidateSnapshot,
	localPracticeCompatibility *LocalPracticeCompatibilityAssessment,
	outcome CheckOutcome,
	deltas []Delta,
	diagnostics []Diagnostic,
	timings []StageTiming,
) reportDTO {
	result := reportDTO{
		Schema:       ReportSchemaV1,
		ToolRevision: toolRevision.String(),
		Result:       resultDTO{State: outcome.State().String()},
		Predecessor: snapshotDTOOf(
			predecessor.source,
			true,
			&predecessor.derived,
		),
		Candidate: snapshotDTOOf(
			candidate.source,
			candidate.sourceComplete,
			candidate.derived,
		),
		Deltas:      deltaDTOs(deltas),
		Diagnostics: diagnosticDTOs(diagnostics),
		Timings:     stageTimingDTOs(timings),
	}
	if localPracticeCompatibility != nil {
		result.LocalPracticeCompatibility = &localPracticeCompatibilityDTO{
			Result:             localPracticeCompatibility.result.String(),
			CarrierBaseRef:     localPracticeCompatibility.carrierBase.String(),
			PredecessorBaseRef: localPracticeCompatibility.predecessorBase.String(),
			CandidateBaseRef:   localPracticeCompatibility.candidateBase.String(),
		}
	}
	return result
}

func snapshotDTOOf(
	source SourceCoordinates,
	sourceComplete bool,
	derived *DerivedCoordinates,
) snapshotDTO {
	result := snapshotDTO{
		Source: sourceCoordinatesDTO{
			Revision: source.revision.String(),
		},
	}
	if sourceComplete {
		result.Source.ReadmeDigest = source.readmeDigest.String()
		result.Source.SpecDigest = source.specDigest.String()
	}
	if derived != nil {
		result.Derived = &derivedCoordinatesDTO{
			SourceUnitCount:    derived.sourceUnitCount,
			BaseTypeEnvRef:     derived.baseTypeEnvRef.String(),
			BaseTypeEnvDigest:  derived.baseTypeEnvDigest.String(),
			CompilerEdition:    derived.compilerEdition.String(),
			DatabaseDigest:     derived.databaseDigest.String(),
			IndexSchemaVersion: derived.indexSchemaVersion,
		}
	}
	return result
}

func deltaDTOs(values []Delta) []deltaDTO {
	result := make([]deltaDTO, 0, len(values))
	for _, delta := range values {
		result = append(result, deltaDTO{
			Family:    delta.family.String(),
			Kind:      delta.kind.String(),
			Subject:   delta.subject,
			Before:    delta.before,
			After:     delta.after,
			SourceRef: delta.sourceRef,
		})
	}
	return result
}

func diagnosticDTOs(values []Diagnostic) []diagnosticDTO {
	result := make([]diagnosticDTO, 0, len(values))
	for _, diagnostic := range values {
		result = append(result, diagnosticDTO{
			Code:             diagnostic.code.String(),
			Subject:          diagnostic.subject,
			Message:          diagnostic.message,
			SourceRef:        diagnostic.sourceRef,
			ReproduceCommand: diagnostic.reproduceCommand,
		})
	}
	return result
}

func stageTimingDTOs(values []StageTiming) []stageTimingDTO {
	result := make([]stageTimingDTO, 0, len(values))
	for _, timing := range values {
		result = append(result, stageTimingDTO{
			Stage:         timing.stage.String(),
			DurationNanos: timing.duration.Nanoseconds(),
		})
	}
	return result
}

func writeReadableSnapshot(
	builder *strings.Builder,
	label string,
	source SourceCoordinates,
	sourceComplete bool,
	derived *DerivedCoordinates,
) {
	fmt.Fprintf(builder, "%s:\n", label)
	fmt.Fprintf(builder, "  source_revision: %s\n", source.revision.String())
	if sourceComplete {
		fmt.Fprintf(builder, "  readme_digest: %s\n", source.readmeDigest.String())
		fmt.Fprintf(builder, "  spec_digest: %s\n", source.specDigest.String())
	} else {
		builder.WriteString("  publication_digests: unavailable\n")
	}
	if derived == nil {
		builder.WriteString("  derived: unavailable\n")
		return
	}
	fmt.Fprintf(builder, "  source_unit_count: %d\n", derived.sourceUnitCount)
	fmt.Fprintf(builder, "  base_type_env_ref: %s\n", derived.baseTypeEnvRef.String())
	fmt.Fprintf(builder, "  base_type_env_digest: %s\n", derived.baseTypeEnvDigest.String())
	fmt.Fprintf(builder, "  compiler_edition: %s\n", derived.compilerEdition.String())
	fmt.Fprintf(builder, "  database_digest: %s\n", derived.databaseDigest.String())
	fmt.Fprintf(builder, "  index_schema_version: %d\n", derived.indexSchemaVersion)
}
