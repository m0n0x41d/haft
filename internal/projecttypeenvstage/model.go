package projecttypeenvstage

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	verificationRecordCanonicalSchema = "haft.fpf.projecttypeenv.composite-verification/v1"
	executableSnapshotCanonicalSchema = projecttypeenv.ProjectTypeEnvExecutableSnapshotSchema
)

var (
	ErrContextRequired = errors.New("project TypeEnv Stage store context is required")
	ErrStoreRequired   = errors.New("project TypeEnv Stage store is required")
	ErrStageNotFound   = errors.New("project TypeEnv Stage is not found")
	ErrStageConflict   = errors.New("project TypeEnv Stage coordinate conflicts with stored bytes")
	ErrStageIntegrity  = errors.New("project TypeEnv Stage integrity check failed")
)

// PersistedStage is the data-only pair returned by plain storage reads.
// Possessing it does not recreate final-lowerer or selection capability.
type PersistedStage struct {
	stage        projecttypeenvselection.ProjectTypeEnvStage
	verification projecttypeenv.ProjectTypeEnvCompositeVerificationRecord
	snapshot     projecttypeenv.ProjectTypeEnvExecutableSnapshotRecord
}

func (value PersistedStage) Stage() projecttypeenvselection.ProjectTypeEnvStage {
	return value.stage
}

func (value PersistedStage) VerificationRecord() projecttypeenv.ProjectTypeEnvCompositeVerificationRecord {
	return value.verification
}

func (value PersistedStage) ExecutableSnapshotRecord() projecttypeenv.ProjectTypeEnvExecutableSnapshotRecord {
	return value.snapshot
}

func (value PersistedStage) Verify() error {
	_, _, _, err := preparePersistedStage(
		value.stage,
		value.verification,
		value.snapshot,
	)
	return err
}

type selectionReadyStageCapability struct{}

// SelectionReadyStage is a non-serializable in-process result minted only
// after the exact persisted closure has been reloaded and final lowering has
// been replayed. It performs no head mutation or selection effect.
type SelectionReadyStage struct {
	persisted    PersistedStage
	verification projecttypeenv.ProjectTypeEnvCompositeVerification
	snapshot     projecttypeenv.ProjectTypeEnvExecutableSnapshot
	capability   *selectionReadyStageCapability
}

func (value SelectionReadyStage) Stage() projecttypeenvselection.ProjectTypeEnvStage {
	return value.persisted.Stage()
}

func (value SelectionReadyStage) VerificationRecord() projecttypeenv.ProjectTypeEnvCompositeVerificationRecord {
	return value.persisted.VerificationRecord()
}

func (value SelectionReadyStage) FinalLowererVerification() projecttypeenv.ProjectTypeEnvCompositeVerification {
	return value.verification
}

func (value SelectionReadyStage) ExecutableSnapshot() projecttypeenv.ProjectTypeEnvExecutableSnapshot {
	return value.snapshot
}

func (value SelectionReadyStage) ExecutableTypeEnv() typedmemory.TypeEnv {
	return value.snapshot.Environment()
}

func (value SelectionReadyStage) Verify() error {
	if value.capability == nil {
		return fmt.Errorf("selection-ready Stage capability was not minted by reload service")
	}
	if err := value.persisted.Verify(); err != nil {
		return err
	}
	if err := value.verification.Verify(); err != nil {
		return fmt.Errorf("verify restored final-lowerer capability: %w", err)
	}
	if err := value.snapshot.Verify(); err != nil {
		return fmt.Errorf("verify restored executable snapshot: %w", err)
	}
	record := value.persisted.VerificationRecord()
	if value.verification.Ref() != record.Ref() ||
		!bytes.Equal(value.verification.CanonicalBytes(), record.CanonicalBytes()) {
		return fmt.Errorf("restored final-lowerer capability differs from persisted record")
	}
	snapshotRecord := value.persisted.ExecutableSnapshotRecord()
	if value.snapshot.TypeEnvRef() != snapshotRecord.TypeEnvRef() ||
		value.snapshot.Digest() != snapshotRecord.Digest() ||
		!bytes.Equal(
			value.snapshot.Record().CanonicalBytes(),
			snapshotRecord.CanonicalBytes(),
		) {
		return fmt.Errorf("restored executable snapshot differs from persisted record")
	}
	return nil
}

type stageRecord struct {
	ref             string
	digest          string
	project         string
	verificationRef string
	executableRef   string
	canonicalSchema string
	canonical       []byte
}

func (record stageRecord) clone() stageRecord {
	result := record
	result.canonical = append([]byte(nil), record.canonical...)
	return result
}

func (record stageRecord) exactEqual(other stageRecord) bool {
	return record.ref == other.ref &&
		record.digest == other.digest &&
		record.project == other.project &&
		record.verificationRef == other.verificationRef &&
		record.executableRef == other.executableRef &&
		record.canonicalSchema == other.canonicalSchema &&
		bytes.Equal(record.canonical, other.canonical)
}

type verificationRecord struct {
	ref             string
	digest          string
	lowererSchema   string
	canonicalSchema string
	canonical       []byte
}

func (record verificationRecord) clone() verificationRecord {
	result := record
	result.canonical = append([]byte(nil), record.canonical...)
	return result
}

func (record verificationRecord) exactEqual(other verificationRecord) bool {
	return record.ref == other.ref &&
		record.digest == other.digest &&
		record.lowererSchema == other.lowererSchema &&
		record.canonicalSchema == other.canonicalSchema &&
		bytes.Equal(record.canonical, other.canonical)
}

type executableSnapshotRecord struct {
	typeEnvRef      string
	snapshotDigest  string
	loweredDigest   string
	sourceRevision  string
	compilerSchema  string
	lowererSchema   string
	verificationRef string
	canonicalSchema string
	canonical       []byte
}

func (record executableSnapshotRecord) clone() executableSnapshotRecord {
	result := record
	result.canonical = append([]byte(nil), record.canonical...)
	return result
}

func (record executableSnapshotRecord) exactEqual(other executableSnapshotRecord) bool {
	return record.typeEnvRef == other.typeEnvRef &&
		record.snapshotDigest == other.snapshotDigest &&
		record.loweredDigest == other.loweredDigest &&
		record.sourceRevision == other.sourceRevision &&
		record.compilerSchema == other.compilerSchema &&
		record.lowererSchema == other.lowererSchema &&
		record.verificationRef == other.verificationRef &&
		record.canonicalSchema == other.canonicalSchema &&
		bytes.Equal(record.canonical, other.canonical)
}

func preparePersistedStage(
	stage projecttypeenvselection.ProjectTypeEnvStage,
	verification projecttypeenv.ProjectTypeEnvCompositeVerificationRecord,
	snapshot projecttypeenv.ProjectTypeEnvExecutableSnapshotRecord,
) (stageRecord, verificationRecord, executableSnapshotRecord, error) {
	if err := stage.Verify(); err != nil {
		return stageRecord{}, verificationRecord{}, executableSnapshotRecord{}, integrityError(
			stage.Ref().String(),
			fmt.Errorf("verify Stage: %w", err),
		)
	}
	if err := verification.Verify(); err != nil {
		return stageRecord{}, verificationRecord{}, executableSnapshotRecord{}, integrityError(
			stage.Ref().String(),
			fmt.Errorf("verify final-lowerer record: %w", err),
		)
	}
	if err := snapshot.Verify(); err != nil {
		return stageRecord{}, verificationRecord{}, executableSnapshotRecord{}, integrityError(
			stage.Ref().String(),
			fmt.Errorf("verify executable snapshot record: %w", err),
		)
	}
	if err := verifyStageVerificationCoordinates(stage, verification); err != nil {
		return stageRecord{}, verificationRecord{}, executableSnapshotRecord{},
			integrityError(stage.Ref().String(), err)
	}
	if err := verifyStageSnapshotCoordinates(stage, verification, snapshot); err != nil {
		return stageRecord{}, verificationRecord{}, executableSnapshotRecord{},
			integrityError(stage.Ref().String(), err)
	}
	stageRow := stageRecord{
		ref:             stage.Ref().String(),
		digest:          stage.Ref().Digest().String(),
		project:         stage.Project().String(),
		verificationRef: verification.Ref().String(),
		executableRef:   snapshot.TypeEnvRef().String(),
		canonicalSchema: stage.SchemaEdition(),
		canonical:       stage.CanonicalBytes(),
	}
	verificationRow := verificationRecord{
		ref:             verification.Ref().String(),
		digest:          verification.Digest().String(),
		lowererSchema:   verification.LowererSchemaVersion(),
		canonicalSchema: verificationRecordCanonicalSchema,
		canonical:       verification.CanonicalBytes(),
	}
	snapshotRow := executableSnapshotRecord{
		typeEnvRef:      snapshot.TypeEnvRef().String(),
		snapshotDigest:  snapshot.Digest().String(),
		loweredDigest:   snapshot.LoweredEnvironmentDigest().String(),
		sourceRevision:  snapshot.SourceRevision().String(),
		compilerSchema:  snapshot.CompilerSchemaVersion().String(),
		lowererSchema:   snapshot.LowererSchemaVersion(),
		verificationRef: snapshot.VerificationRef().String(),
		canonicalSchema: executableSnapshotCanonicalSchema,
		canonical:       snapshot.CanonicalBytes(),
	}
	return stageRow, verificationRow, snapshotRow, nil
}

func verifyStageVerificationCoordinates(
	stage projecttypeenvselection.ProjectTypeEnvStage,
	verification projecttypeenv.ProjectTypeEnvCompositeVerificationRecord,
) error {
	if stage.CompositeVerificationRef() != verification.Ref() ||
		stage.CompositeVerificationDigest() != verification.Digest() {
		return fmt.Errorf("Stage composite-verification identity mismatch")
	}
	if stage.Base() != verification.BaseTypeEnvRef() {
		return fmt.Errorf("Stage B differs from final-lowerer record")
	}
	if !orderedExtensionRefsEqual(stage.OrderedExtensions(), verification.ExtensionRefs()) {
		return fmt.Errorf("Stage ordered E DAG differs from final-lowerer record")
	}
	if stage.RuntimeBasis() != verification.RuntimeEvaluationBasisRef() {
		return fmt.Errorf("Stage X differs from final-lowerer record")
	}
	if stage.VerifiedComposite() != verification.CompositeRef() {
		return fmt.Errorf("Stage C differs from final-lowerer record")
	}
	return nil
}

func verifyStageSnapshotCoordinates(
	stage projecttypeenvselection.ProjectTypeEnvStage,
	verification projecttypeenv.ProjectTypeEnvCompositeVerificationRecord,
	snapshot projecttypeenv.ProjectTypeEnvExecutableSnapshotRecord,
) error {
	if stage.VerifiedComposite() != snapshot.TypeEnvRef() {
		return fmt.Errorf("Stage C differs from executable snapshot TypeEnvRef")
	}
	if stage.Base() != snapshot.BaseTypeEnvRef() {
		return fmt.Errorf("Stage B differs from executable snapshot")
	}
	if !orderedExtensionRefsEqual(stage.OrderedExtensions(), snapshot.ExtensionRefs()) {
		return fmt.Errorf("Stage ordered E DAG differs from executable snapshot")
	}
	if stage.RuntimeBasis() != snapshot.RuntimeEvaluationBasisRef() {
		return fmt.Errorf("Stage X differs from executable snapshot")
	}
	if verification.Ref() != snapshot.VerificationRef() {
		return fmt.Errorf("Stage verification differs from executable snapshot")
	}
	if verification.LoweredEnvironmentDigest() != snapshot.LoweredEnvironmentDigest() {
		return fmt.Errorf("final-lowerer digest differs from executable snapshot")
	}
	if verification.LowererSchemaVersion() != snapshot.LowererSchemaVersion() {
		return fmt.Errorf("final-lowerer edition differs from executable snapshot")
	}
	return nil
}

func decodePersistedStage(
	stageRow stageRecord,
	verificationRow verificationRecord,
	snapshotRow executableSnapshotRecord,
) (PersistedStage, error) {
	stageRef, err := projecttypeenvselection.ParseProjectTypeEnvStageRef(stageRow.ref)
	if err != nil {
		return PersistedStage{}, integrityError(stageRow.ref, err)
	}
	stage, err := projecttypeenvselection.VerifyProjectTypeEnvStage(
		stageRef,
		stageRow.canonical,
	)
	if err != nil {
		return PersistedStage{}, integrityError(stageRow.ref, err)
	}
	verificationRef, err := projecttypeenv.ParseProjectTypeEnvCompositeVerificationRef(
		verificationRow.ref,
	)
	if err != nil {
		return PersistedStage{}, integrityError(stageRow.ref, err)
	}
	verification, err := projecttypeenv.VerifyProjectTypeEnvCompositeVerificationRecord(
		verificationRef,
		verificationRow.canonical,
	)
	if err != nil {
		return PersistedStage{}, integrityError(stageRow.ref, err)
	}
	snapshot, err := projecttypeenv.DecodeProjectTypeEnvExecutableSnapshotRecord(
		snapshotRow.canonical,
	)
	if err != nil {
		return PersistedStage{}, integrityError(stageRow.ref, err)
	}
	expectedStage, expectedVerification, expectedSnapshot, err := preparePersistedStage(
		stage,
		verification,
		snapshot,
	)
	if err != nil {
		return PersistedStage{}, err
	}
	if !stageRecordMatchesCanonical(expectedStage, stageRow, stage.SchemaEdition()) {
		return PersistedStage{}, integrityError(
			stageRow.ref,
			fmt.Errorf("stored Stage metadata does not match canonical bytes"),
		)
	}
	if !expectedVerification.exactEqual(verificationRow) {
		return PersistedStage{}, integrityError(
			stageRow.ref,
			fmt.Errorf("stored verification metadata does not match canonical bytes"),
		)
	}
	if !expectedSnapshot.exactEqual(snapshotRow) {
		return PersistedStage{}, integrityError(
			stageRow.ref,
			fmt.Errorf("stored executable snapshot metadata does not match canonical bytes"),
		)
	}
	return PersistedStage{
		stage:        stage,
		verification: verification,
		snapshot:     snapshot,
	}, nil
}

func stageRecordMatchesCanonical(
	expected stageRecord,
	stored stageRecord,
	decodedSchema string,
) bool {
	if expected.exactEqual(stored) {
		return true
	}
	if decodedSchema != projecttypeenvselection.ProjectTypeEnvStageSchemaEditionV3 ||
		stored.canonicalSchema != projecttypeenvselection.ProjectTypeEnvStageSchemaEditionV2 {
		return false
	}
	legacy := stored.clone()
	legacy.canonicalSchema = projecttypeenvselection.ProjectTypeEnvStageSchemaEditionV3
	return expected.exactEqual(legacy)
}

func orderedExtensionRefsEqual(
	left []typedmemory.TypeEnvExtensionRef,
	right []typedmemory.TypeEnvExtensionRef,
) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].String() != right[index].String() {
			return false
		}
	}
	return true
}

func integrityError(ref string, err error) error {
	return fmt.Errorf("%w for %q: %v", ErrStageIntegrity, ref, err)
}
