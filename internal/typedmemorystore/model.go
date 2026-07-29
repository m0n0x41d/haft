package typedmemorystore

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

var (
	ErrDatabaseRequired          = errors.New("typed-memory database is required")
	ErrTypeEnvLoaderRequired     = errors.New("typed-memory TypeEnv loader is required")
	ErrClockRequired             = errors.New("typed-memory clock is required")
	ErrSnapshotConflict          = errors.New("typed-memory TypeEnv snapshot conflicts with immutable history")
	ErrProjectNotInitialized     = errors.New("typed-memory graph is not initialized for this project")
	ErrInvalidAdmissionBatch     = errors.New("typed-memory admission batch is invalid")
	ErrUnsupportedBatch          = errors.New("typed-memory change is not supported by generic storage")
	ErrAdmissionBasisMismatch    = errors.New("typed-memory admission batch basis does not match the requested transaction basis")
	ErrStaleGraphRevision        = errors.New("typed-memory graph revision is stale")
	ErrActiveTypeEnvMismatch     = errors.New("typed-memory active TypeEnv does not match the requested basis")
	ErrIdempotencyConflict       = errors.New("typed-memory idempotency key was already consumed by another canonical change set")
	ErrRevalidationRejected      = errors.New("typed-memory transaction-time revalidation did not return Valid")
	ErrRevisionOverflow          = errors.New("typed-memory graph revision exceeds SQLite signed INTEGER range")
	ErrCommitOutcomeUnknown      = errors.New("typed-memory commit outcome is unknown")
	ErrAdmissionBatchRequired    = errors.New("typed-memory generic commit requires a sealed admission batch")
	ErrRequestDigestMismatch     = errors.New("typed-memory request bytes do not match the sealed admission batch")
	ErrAdmissionEnvelopeMismatch = errors.New(
		"typed-memory admission envelope does not match transaction-time revalidation",
	)
	ErrObservableInputBlobRequired = errors.New(
		"typed-memory observable input blob is unavailable for the exact admission basis",
	)
	ErrManualIdentityReconciliationRequired = errors.New(
		"typed-memory merge and split require the manual identity reconciliation service",
	)
	ErrStorageGenerationUnavailable = errors.New(
		"typed-memory generic admission storage generation is unavailable",
	)
	ErrStoredAdmissionIntegrity = errors.New(
		"typed-memory stored admission failed exact integrity verification",
	)
	ErrLegacyAdmissionReplayOnly = errors.New(
		"typed-memory v1 admission is replay-only",
	)
)

const BaseTypeEnvSnapshotFormat = "base-typeenv-artifact-payload.v1"

const (
	admissionContractVersionV1 = "haft.memory.v1"
	admissionContractVersionV2 = "haft.memory.v2"
)

// AdmissionContractVersion is the store-owned closed admission protocol
// version. Its zero value and arbitrary strings are not valid versions, so a
// caller cannot silently route an admission through a storage generation by
// inventing a tag.
type AdmissionContractVersion struct {
	value string
}

func AdmissionContractV1() AdmissionContractVersion {
	return AdmissionContractVersion{value: admissionContractVersionV1}
}

func AdmissionContractV2() AdmissionContractVersion {
	return AdmissionContractVersion{value: admissionContractVersionV2}
}

func ParseAdmissionContractVersion(raw string) (AdmissionContractVersion, error) {
	switch raw {
	case admissionContractVersionV1:
		return AdmissionContractV1(), nil
	case admissionContractVersionV2:
		return AdmissionContractV2(), nil
	default:
		return AdmissionContractVersion{}, fmt.Errorf(
			"typed-memory admission contract version %q is unsupported",
			raw,
		)
	}
}

func (version AdmissionContractVersion) String() string { return version.value }

func (version AdmissionContractVersion) IsV1() bool {
	return version == AdmissionContractV1()
}

func (version AdmissionContractVersion) IsV2() bool {
	return version == AdmissionContractV2()
}

func (version AdmissionContractVersion) valid() bool {
	return version.IsV1() || version.IsV2()
}

type SnapshotFormat struct {
	value string
}

func NewSnapshotFormat(raw string) (SnapshotFormat, error) {
	value := strings.TrimSpace(raw)
	if value == "" || value != raw {
		return SnapshotFormat{}, fmt.Errorf("snapshot format must be non-empty canonical text")
	}
	return SnapshotFormat{value: value}, nil
}

func (format SnapshotFormat) String() string { return format.value }

type TypeEnvSnapshot struct {
	ref                   typedmemory.TypeEnvRef
	artifactDigest        typedmemory.SHA256Digest
	format                SnapshotFormat
	canonicalBytes        []byte
	sourceRevision        typedmemory.SourceRevision
	compilerSchemaVersion typedmemory.CompilerSchemaVersion
}

type TypeEnvSnapshotBuilder struct {
	value TypeEnvSnapshot
	err   error
}

func NewTypeEnvSnapshotBuilder(
	ref typedmemory.TypeEnvRef,
) *TypeEnvSnapshotBuilder {
	return &TypeEnvSnapshotBuilder{value: TypeEnvSnapshot{ref: ref}}
}

func (builder *TypeEnvSnapshotBuilder) SetFormat(
	format SnapshotFormat,
) *TypeEnvSnapshotBuilder {
	builder.value.format = format
	return builder
}

func (builder *TypeEnvSnapshotBuilder) SetCanonicalBytes(
	canonical []byte,
) *TypeEnvSnapshotBuilder {
	builder.value.canonicalBytes = append([]byte(nil), canonical...)
	return builder
}

func (builder *TypeEnvSnapshotBuilder) SetSourceRevision(
	revision typedmemory.SourceRevision,
) *TypeEnvSnapshotBuilder {
	builder.value.sourceRevision = revision
	return builder
}

func (builder *TypeEnvSnapshotBuilder) SetCompilerSchemaVersion(
	version typedmemory.CompilerSchemaVersion,
) *TypeEnvSnapshotBuilder {
	builder.value.compilerSchemaVersion = version
	return builder
}

func (builder *TypeEnvSnapshotBuilder) Build() (TypeEnvSnapshot, error) {
	if builder == nil {
		return TypeEnvSnapshot{}, fmt.Errorf("TypeEnv snapshot builder is required")
	}
	if builder.err != nil {
		return TypeEnvSnapshot{}, builder.err
	}
	value := builder.value
	if len(value.canonicalBytes) == 0 {
		return TypeEnvSnapshot{}, fmt.Errorf("TypeEnv snapshot canonical bytes are required")
	}
	digest, err := digestBytes(value.canonicalBytes)
	if err != nil {
		return TypeEnvSnapshot{}, err
	}
	if value.ref.String() != "typeenv:"+digest.String() {
		return TypeEnvSnapshot{}, fmt.Errorf("TypeEnv snapshot ref does not match canonical bytes")
	}
	if value.format.String() == "" ||
		value.sourceRevision.String() == "" ||
		value.compilerSchemaVersion.String() == "" {
		return TypeEnvSnapshot{}, fmt.Errorf("TypeEnv snapshot format, source revision, and compiler version are required")
	}
	value.artifactDigest = digest
	return cloneSnapshot(value), nil
}

func (snapshot TypeEnvSnapshot) Ref() typedmemory.TypeEnvRef { return snapshot.ref }

func (snapshot TypeEnvSnapshot) ArtifactDigest() typedmemory.SHA256Digest {
	return snapshot.artifactDigest
}

func (snapshot TypeEnvSnapshot) Format() SnapshotFormat { return snapshot.format }

func (snapshot TypeEnvSnapshot) CanonicalBytes() []byte {
	return append([]byte(nil), snapshot.canonicalBytes...)
}

func (snapshot TypeEnvSnapshot) SourceRevision() typedmemory.SourceRevision {
	return snapshot.sourceRevision
}

func (snapshot TypeEnvSnapshot) CompilerSchemaVersion() typedmemory.CompilerSchemaVersion {
	return snapshot.compilerSchemaVersion
}

func cloneSnapshot(snapshot TypeEnvSnapshot) TypeEnvSnapshot {
	result := snapshot
	result.canonicalBytes = append([]byte(nil), snapshot.canonicalBytes...)
	return result
}

type IdempotencyKey struct {
	value string
}

func NewIdempotencyKey(raw string) (IdempotencyKey, error) {
	value := strings.TrimSpace(raw)
	if value == "" || value != raw || len(value) > 512 {
		return IdempotencyKey{}, fmt.Errorf("idempotency key must be 1-512 bytes of canonical text")
	}
	return IdempotencyKey{value: value}, nil
}

func (key IdempotencyKey) String() string { return key.value }

// ReplayRequest is the minimal exact identity supplied before semantic
// revalidation. It can recover only an already-committed byte-identical
// admission. It carries no AdmissionBatch and therefore cannot create work.
type ReplayRequest struct {
	contractVersion   AdmissionContractVersion
	project           projectledger.ProjectID
	expectedRevision  typedmemory.GraphRevision
	expectedTypeEnv   typedmemory.TypeEnvRef
	idempotencyKey    IdempotencyKey
	requestProvenance typedmemory.ProvenanceRef
	candidate         typedmemory.MemoryChangeSet
}

type ReplayRequestBuilder struct {
	value               ReplayRequest
	expectedRevisionSet bool
	contractVersionSet  bool
}

func NewReplayRequestBuilder() *ReplayRequestBuilder {
	return &ReplayRequestBuilder{}
}

func (builder *ReplayRequestBuilder) SetContractVersion(
	version AdmissionContractVersion,
) *ReplayRequestBuilder {
	builder.value.contractVersion = version
	builder.contractVersionSet = true
	return builder
}

func (builder *ReplayRequestBuilder) SetProject(
	project projectledger.ProjectID,
) *ReplayRequestBuilder {
	builder.value.project = project
	return builder
}

func (builder *ReplayRequestBuilder) SetExpectedRevision(
	revision typedmemory.GraphRevision,
) *ReplayRequestBuilder {
	builder.value.expectedRevision = revision
	builder.expectedRevisionSet = true
	return builder
}

func (builder *ReplayRequestBuilder) SetExpectedTypeEnv(
	environment typedmemory.TypeEnvRef,
) *ReplayRequestBuilder {
	builder.value.expectedTypeEnv = environment
	return builder
}

func (builder *ReplayRequestBuilder) SetIdempotencyKey(
	key IdempotencyKey,
) *ReplayRequestBuilder {
	builder.value.idempotencyKey = key
	return builder
}

func (builder *ReplayRequestBuilder) SetRequestProvenance(
	provenance typedmemory.ProvenanceRef,
) *ReplayRequestBuilder {
	builder.value.requestProvenance = provenance
	return builder
}

func (builder *ReplayRequestBuilder) SetCandidate(
	candidate typedmemory.MemoryChangeSet,
) *ReplayRequestBuilder {
	builder.value.candidate = candidate
	return builder
}

func (builder *ReplayRequestBuilder) Build() (ReplayRequest, error) {
	if builder == nil {
		return ReplayRequest{}, fmt.Errorf(
			"replay request builder and admission contract version must be explicit",
		)
	}
	if !builder.contractVersionSet || !builder.value.contractVersion.valid() {
		return ReplayRequest{}, fmt.Errorf(
			"replay request admission contract version must be explicit",
		)
	}
	if !builder.expectedRevisionSet {
		return ReplayRequest{}, fmt.Errorf(
			"replay request expected graph revision must be explicit",
		)
	}
	value := builder.value
	project, err := projectledger.ParseProjectID(value.project.String())
	if err != nil || project != value.project {
		return ReplayRequest{}, fmt.Errorf("replay request project is invalid")
	}
	if value.expectedRevision.Value() > math.MaxInt64-1 {
		return ReplayRequest{}, ErrRevisionOverflow
	}
	if value.expectedTypeEnv.String() == "" ||
		value.idempotencyKey.String() == "" ||
		value.requestProvenance.String() == "" {
		return ReplayRequest{}, fmt.Errorf(
			"replay request exact TypeEnv, idempotency key, and provenance are required",
		)
	}
	if len(value.candidate.Changes()) == 0 {
		return ReplayRequest{}, fmt.Errorf(
			"replay request candidate change set is required",
		)
	}
	return value, nil
}

func (request ReplayRequest) ContractVersion() AdmissionContractVersion {
	return request.contractVersion
}

type CommitRequest struct {
	contractVersion   AdmissionContractVersion
	project           projectledger.ProjectID
	expectedRevision  typedmemory.GraphRevision
	expectedTypeEnv   typedmemory.TypeEnvRef
	idempotencyKey    IdempotencyKey
	requestProvenance typedmemory.ProvenanceRef
	candidate         typedmemory.MemoryChangeSet
	admissionBatch    typedmemory.AdmissionBatch
}

type CommitRequestBuilder struct {
	value               CommitRequest
	expectedRevisionSet bool
	contractVersionSet  bool
}

func NewCommitRequestBuilder() *CommitRequestBuilder { return &CommitRequestBuilder{} }

func (builder *CommitRequestBuilder) SetContractVersion(
	version AdmissionContractVersion,
) *CommitRequestBuilder {
	builder.value.contractVersion = version
	builder.contractVersionSet = true
	return builder
}

func (builder *CommitRequestBuilder) SetProject(
	project projectledger.ProjectID,
) *CommitRequestBuilder {
	builder.value.project = project
	return builder
}

func (builder *CommitRequestBuilder) SetExpectedRevision(
	revision typedmemory.GraphRevision,
) *CommitRequestBuilder {
	builder.value.expectedRevision = revision
	builder.expectedRevisionSet = true
	return builder
}

func (builder *CommitRequestBuilder) SetExpectedTypeEnv(
	ref typedmemory.TypeEnvRef,
) *CommitRequestBuilder {
	builder.value.expectedTypeEnv = ref
	return builder
}

func (builder *CommitRequestBuilder) SetIdempotencyKey(
	key IdempotencyKey,
) *CommitRequestBuilder {
	builder.value.idempotencyKey = key
	return builder
}

// SetRequestProvenance supplies the trusted application-level provenance for
// this admission request. It is distinct from per-change provenance and is
// never inferred from candidate content or an admission envelope.
func (builder *CommitRequestBuilder) SetRequestProvenance(
	provenance typedmemory.ProvenanceRef,
) *CommitRequestBuilder {
	builder.value.requestProvenance = provenance
	return builder
}

func (builder *CommitRequestBuilder) SetCandidate(
	changeSet typedmemory.MemoryChangeSet,
) *CommitRequestBuilder {
	builder.value.candidate = changeSet
	return builder
}

// SetAdmissionBatch supplies the sealed result of the read-only validation
// pass. Generic admission requires it so transaction-time validation can
// recheck the exact observations, membership judgements, and reference
// resolutions that produced the candidate result. The bounded legacy
// CommitDeclareEntity compatibility path may omit it.
func (builder *CommitRequestBuilder) SetAdmissionBatch(
	batch typedmemory.AdmissionBatch,
) *CommitRequestBuilder {
	builder.value.admissionBatch = batch
	return builder
}

func (builder *CommitRequestBuilder) Build() (CommitRequest, error) {
	if builder == nil {
		return CommitRequest{}, fmt.Errorf("commit request builder is required")
	}
	value := builder.value
	if !builder.contractVersionSet || !value.contractVersion.valid() {
		return CommitRequest{}, fmt.Errorf(
			"commit request admission contract version must be explicit",
		)
	}
	if !builder.expectedRevisionSet {
		return CommitRequest{}, fmt.Errorf("commit request expected graph revision must be explicit")
	}
	project, err := projectledger.ParseProjectID(value.project.String())
	if err != nil || project != value.project {
		return CommitRequest{}, fmt.Errorf("commit request project is invalid")
	}
	if value.expectedRevision.Value() > math.MaxInt64-1 {
		return CommitRequest{}, ErrRevisionOverflow
	}
	if value.expectedTypeEnv.String() == "" || value.idempotencyKey.String() == "" {
		return CommitRequest{}, fmt.Errorf("commit request exact TypeEnv and idempotency key are required")
	}
	if value.requestProvenance.String() == "" {
		return CommitRequest{}, fmt.Errorf("commit request provenance is required")
	}
	if len(value.candidate.Changes()) == 0 {
		return CommitRequest{}, fmt.Errorf("commit request candidate change set is required")
	}
	return value, nil
}

func (request CommitRequest) ContractVersion() AdmissionContractVersion {
	return request.contractVersion
}

type CommitDisposition string

const (
	CommitApplied   CommitDisposition = "applied"
	CommitReplay    CommitDisposition = "replay"
	CommitRecovered CommitDisposition = "recovered"
)

type CommitReceipt struct {
	disposition CommitDisposition
	eventRef    string
	commitRef   string
	revision    typedmemory.GraphRevision
	digest      typedmemory.SHA256Digest
}

func (receipt CommitReceipt) Disposition() CommitDisposition { return receipt.disposition }

func (receipt CommitReceipt) EventRef() string { return receipt.eventRef }

func (receipt CommitReceipt) CommitRef() string { return receipt.commitRef }

func (receipt CommitReceipt) GraphRevision() typedmemory.GraphRevision {
	return receipt.revision
}

func (receipt CommitReceipt) ResultDigest() typedmemory.SHA256Digest {
	return receipt.digest
}

type GraphHead struct {
	project       projectledger.ProjectID
	revision      typedmemory.GraphRevision
	activeTypeEnv typedmemory.TypeEnvRef
	lastEventRef  string
	lastCommitRef string
}

func (head GraphHead) Project() projectledger.ProjectID { return head.project }

func (head GraphHead) Revision() typedmemory.GraphRevision { return head.revision }

func (head GraphHead) ActiveTypeEnv() typedmemory.TypeEnvRef { return head.activeTypeEnv }

func (head GraphHead) LastEventRef() string { return head.lastEventRef }

func (head GraphHead) LastCommitRef() string { return head.lastCommitRef }

type StoredEntity struct {
	project    projectledger.ProjectID
	entity     typedmemory.EntityID
	context    typedmemory.BoundedContextRef
	label      typedmemory.EntityLabel
	provenance typedmemory.ProvenanceRef
	eventRef   string
	revision   typedmemory.GraphRevision
}

func (entity StoredEntity) Project() projectledger.ProjectID { return entity.project }

func (entity StoredEntity) Entity() typedmemory.EntityID { return entity.entity }

func (entity StoredEntity) Context() typedmemory.BoundedContextRef { return entity.context }

func (entity StoredEntity) Label() typedmemory.EntityLabel { return entity.label }

func (entity StoredEntity) Provenance() typedmemory.ProvenanceRef { return entity.provenance }

func (entity StoredEntity) EventRef() string { return entity.eventRef }

func (entity StoredEntity) Revision() typedmemory.GraphRevision { return entity.revision }

type Clock interface {
	Now() time.Time
}

type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now() }

func digestBytes(value []byte) (typedmemory.SHA256Digest, error) {
	sum := sha256.Sum256(value)
	encoded := hex.EncodeToString(sum[:])
	return typedmemory.NewSHA256Digest("sha256:" + encoded)
}
