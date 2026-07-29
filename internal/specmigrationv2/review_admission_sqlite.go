package specmigrationv2

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

var ErrNoCurrentSemanticReviewAdmission = errors.New("no admitted semantic review exists for the exact project root, packet-carrier digest, and partition audit")

// NoDurableMigrationReviewSpeechActSourceError is the only condition under
// which a caller may initiate a new terminal capture. Every other Resume
// failure is a fail-closed conflict, corruption, or mutable-environment error.
type NoDurableMigrationReviewSpeechActSourceError struct {
	speechActRef string
}

func (failure *NoDurableMigrationReviewSpeechActSourceError) Error() string {
	return "durable migration-review SpeechAct source is unavailable for " + failure.speechActRef
}

func (failure *NoDurableMigrationReviewSpeechActSourceError) SpeechActRef() string {
	if failure == nil {
		return ""
	}
	return failure.speechActRef
}

// ReviewAdmissionService institutes one migration-specific admission from one
// durable generic SpeechAct source. It owns no terminal capture policy and no
// alternate SpeechAct representation.
type ReviewAdmissionService struct {
	database *sql.DB
	writer   *authority.SpeechActSourceWriter
	store    migrationReviewAdmissionStore
	now      func() time.Time
}

func NewReviewAdmissionService(
	database *sql.DB,
) (ReviewAdmissionService, error) {
	if database == nil {
		return ReviewAdmissionService{}, fmt.Errorf("semantic-review admission database is required")
	}
	if err := database.Ping(); err != nil {
		return ReviewAdmissionService{}, fmt.Errorf("ping semantic-review admission database: %w", err)
	}
	store, err := openMigrationReviewAdmissionStore(database)
	if err != nil {
		return ReviewAdmissionService{}, err
	}
	writer, err := authority.OpenSpeechActSourceWriter(database)
	if err != nil {
		return ReviewAdmissionService{}, err
	}
	return ReviewAdmissionService{
		database: database,
		writer:   writer,
		store:    store,
		now:      time.Now,
	}, nil
}

func (service ReviewAdmissionService) Admit(
	ctx context.Context,
	prepared PreparedMigrationReviewAdmission,
	source authority.VerifiedSpeechActSource,
) (AdmittedMigrationReview, error) {
	if err := service.RecordSource(ctx, prepared, source); err != nil {
		return nil, err
	}
	return service.Resume(ctx, prepared)
}

// RecordSource closes phase one immediately after the terminal SpeechAct. It
// deliberately performs no mutable migration-environment validation: once the
// verified act occurred, its generic source must remain durable even when a
// later carrier/profile/FPF check prevents the domain effect.
func (service ReviewAdmissionService) RecordSource(
	ctx context.Context,
	prepared PreparedMigrationReviewAdmission,
	source authority.VerifiedSpeechActSource,
) error {
	if err := validateReviewAdmissionContext(ctx); err != nil {
		return err
	}
	if err := service.validateOpen(); err != nil {
		return err
	}
	if err := validatePreparedMigrationReviewAdmission(prepared); err != nil {
		return err
	}
	if _, err := exactVerifiedMigrationReviewSource(prepared, source); err != nil {
		return err
	}
	recorded, err := service.writer.Record(ctx, source)
	if err != nil {
		return fmt.Errorf("record migration-review SpeechAct source: %w", err)
	}
	if _, err := exactRecordedMigrationReviewSource(prepared, recorded); err != nil {
		return fmt.Errorf("verify durable migration-review SpeechAct source: %w", err)
	}
	return nil
}

// Resume institutes the migration-specific effect from an already durable
// exact SpeechAct source. It performs no terminal interaction and creates no
// replacement capture when phase one committed before phase two failed.
func (service ReviewAdmissionService) Resume(
	ctx context.Context,
	prepared PreparedMigrationReviewAdmission,
) (AdmittedMigrationReview, error) {
	if err := validateReviewAdmissionContext(ctx); err != nil {
		return nil, err
	}
	if err := service.validateOpen(); err != nil {
		return nil, err
	}
	if err := validatePreparedMigrationReviewAdmission(prepared); err != nil {
		return nil, err
	}
	recorded, found, err := authority.ResolveRecordedSpeechActSource(
		ctx,
		service.database,
		prepared.state.speechActRef,
	)
	if err != nil {
		return nil, fmt.Errorf("resolve durable migration-review SpeechAct source: %w", err)
	}
	if !found {
		return nil, &NoDurableMigrationReviewSpeechActSourceError{
			speechActRef: prepared.state.speechActRef.String(),
		}
	}
	return service.instituteRecordedReview(ctx, prepared, recorded)
}

func (service ReviewAdmissionService) instituteRecordedReview(
	ctx context.Context,
	prepared PreparedMigrationReviewAdmission,
	recorded authority.RecordedSpeechActSource,
) (AdmittedMigrationReview, error) {
	content := prepared.state.content
	bindings, err := exactRecordedMigrationReviewSource(prepared, recorded)
	if err != nil {
		return nil, err
	}
	record, err := newMigrationReviewAdmissionRecordV2(prepared, recorded)
	if err != nil {
		return nil, err
	}
	if err := validateCurrentReviewEnvironment(ctx, content.root, content.carrier); err != nil {
		return nil, err
	}
	transaction, err := sqlitetransaction.BeginImmediate(ctx, service.database)
	if err != nil {
		return nil, fmt.Errorf("begin migration-review effect transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = transaction.Rollback(context.Background()).Err()
		}
	}()
	existing, found, err := service.store.loadInTransactionByStableRef(
		ctx,
		transaction,
		prepared.state.admissionRef,
	)
	if err != nil {
		return nil, err
	}
	if found {
		finish := transaction.Rollback(context.Background())
		committed = true
		if finish.Err() != nil {
			return nil, fmt.Errorf("close migration-review replay transaction: %w", finish.Err())
		}
		return service.resolveExactReplay(ctx, prepared, bindings, existing)
	}
	recordedAt := canonicalReviewTime(service.now())
	if recordedAt.IsZero() || recordedAt.Before(record.admittedAt) {
		return nil, fmt.Errorf("migration-review durable record time precedes the SpeechAct occurrence")
	}
	if err := service.store.insert(ctx, transaction, record, recordedAt); err != nil {
		return nil, err
	}
	reread, rereadFound, err := service.store.loadInTransactionByStableRef(
		ctx,
		transaction,
		record.reviewRef,
	)
	if err != nil {
		return nil, err
	}
	if !rereadFound || !sameMigrationReviewAdmissionV2(record, reread) {
		return nil, fmt.Errorf("staged migration-review admission failed exact closure reread")
	}
	finish := transaction.Commit(ctx)
	committed = true
	if !finish.Succeeded() {
		recovered, recoveryErr := service.loadExactRecord(ctx, record.reviewRef, record.admissionDigest)
		if recoveryErr == nil && sameMigrationReviewAdmissionV2(record, recovered) {
			return recovered.admittedMigrationReview, nil
		}
		return nil, errors.Join(
			fmt.Errorf("commit migration-review admission: %w", finish.Err()),
			recoveryErr,
		)
	}
	durable, err := service.loadExactRecord(ctx, record.reviewRef, record.admissionDigest)
	if err != nil {
		return nil, err
	}
	if !sameMigrationReviewAdmissionV2(record, durable) {
		return nil, fmt.Errorf("committed migration-review admission failed exact durable reread")
	}
	return durable.admittedMigrationReview, nil
}

func (service ReviewAdmissionService) ResolveCurrentForAudit(
	ctx context.Context,
	carrier FinalCandidatePacketCarrier,
	audit PacketPartitionAudit,
) (AdmittedMigrationReview, error) {
	if err := validateReviewAdmissionContext(ctx); err != nil {
		return nil, err
	}
	if err := service.validateOpen(); err != nil {
		return nil, err
	}
	root, err := validateMigrationReviewPreparationBasis(carrier, audit)
	if err != nil {
		return nil, err
	}
	if err := validateCurrentReviewEnvironment(ctx, root, carrier); err != nil {
		return nil, err
	}
	record, found, err := service.store.loadCurrent(
		ctx,
		root,
		carrier.CarrierDigest(),
		audit.Binding(),
	)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrNoCurrentSemanticReviewAdmission
	}
	if err := validateMigrationReviewAgainstCurrent(record, carrier, audit); err != nil {
		return nil, err
	}
	return record.admittedMigrationReview, nil
}

// resolveHistorical skips mutable carrier freshness only for recovery of an
// already journaled effect. It still requires the exact v2 admission closure.
func (service ReviewAdmissionService) resolveHistorical(
	ctx context.Context,
	projectRoot ApplyProjectRoot,
	admissionRef ReviewRef,
	admissionDigest SHA256,
) (AdmittedMigrationReview, error) {
	if err := validateReviewAdmissionContext(ctx); err != nil {
		return nil, err
	}
	if err := service.validateOpen(); err != nil {
		return nil, err
	}
	if !projectRoot.valid() || admissionRef.String() == "" || !admissionDigest.valid() {
		return nil, fmt.Errorf("historical migration-review admission binding is invalid")
	}
	record, err := service.loadExactRecord(ctx, admissionRef, admissionDigest)
	if err != nil {
		return nil, err
	}
	if record.projectRoot.String() != projectRoot.String() {
		return nil, fmt.Errorf("historical migration-review admission belongs to another project root")
	}
	return record.admittedMigrationReview, nil
}

func (service ReviewAdmissionService) validateOpen() error {
	if service.database == nil || service.writer == nil || service.now == nil {
		return fmt.Errorf("semantic-review admission service is not open")
	}
	return nil
}

func (service ReviewAdmissionService) loadExactRecord(
	ctx context.Context,
	ref ReviewRef,
	digest SHA256,
) (migrationReviewAdmissionRecordV2, error) {
	record, found, err := service.store.loadByRef(ctx, ref, digest)
	if err != nil {
		return migrationReviewAdmissionRecordV2{}, err
	}
	if !found {
		return migrationReviewAdmissionRecordV2{}, fmt.Errorf("migration-review admission does not resolve by exact v2 ref and digest")
	}
	return record, nil
}

func (service ReviewAdmissionService) resolveExactReplay(
	ctx context.Context,
	prepared PreparedMigrationReviewAdmission,
	bindings recordedMigrationReviewSourceBindings,
	existing migrationReviewAdmissionRecordV2,
) (AdmittedMigrationReview, error) {
	durable, err := service.loadExactRecord(
		ctx,
		existing.reviewRef,
		existing.admissionDigest,
	)
	if err != nil {
		return nil, err
	}
	if err := validateMigrationReviewReplay(prepared, bindings, durable); err != nil {
		return nil, err
	}
	return durable.admittedMigrationReview, nil
}

func exactVerifiedMigrationReviewSource(
	prepared PreparedMigrationReviewAdmission,
	source authority.VerifiedSpeechActSource,
) (recordedMigrationReviewSourceBindings, error) {
	projectRoot, rootOK := source.ProjectRoot()
	speechActRef, refOK := source.SpeechActRef()
	speechActDigest, actDigestOK := source.SpeechActDigest()
	captureRef, captureRefOK := source.TerminalCaptureRef()
	captureDigest, captureDigestOK := source.TerminalCaptureDigest()
	reviewDigest, reviewDigestOK := source.ReviewDigest()
	reviewText, reviewTextOK := source.ReviewText()
	subjectRef, subjectRefOK := source.ReviewSubjectRef()
	subjectDigest, subjectDigestOK := source.ReviewSubjectDigest()
	occurredAt, occurredAtOK := source.OccurredAt()
	complete := rootOK && refOK && actDigestOK && captureRefOK && captureDigestOK
	complete = complete && reviewDigestOK && reviewTextOK && subjectRefOK && subjectDigestOK && occurredAtOK
	if !complete {
		return recordedMigrationReviewSourceBindings{}, fmt.Errorf("migration-review admission requires a package-verified generic SpeechAct source")
	}
	bindings := recordedMigrationReviewSourceBindings{
		projectRoot:     projectRoot.String(),
		speechActRef:    speechActRef.String(),
		speechActDigest: speechActDigest.String(),
		captureRef:      captureRef.String(),
		captureDigest:   captureDigest.String(),
		reviewDigest:    reviewDigest.String(),
		reviewText:      reviewText,
		subjectRef:      subjectRef.String(),
		subjectDigest:   subjectDigest.String(),
		occurredAt:      occurredAt,
	}
	if err := validateRecordedMigrationReviewBindings(prepared, bindings); err != nil {
		return recordedMigrationReviewSourceBindings{}, err
	}
	return bindings, nil
}

func validateMigrationReviewReplay(
	prepared PreparedMigrationReviewAdmission,
	bindings recordedMigrationReviewSourceBindings,
	record migrationReviewAdmissionRecordV2,
) error {
	state := prepared.state
	reviewText, textOK := state.manualSource.ReviewText()
	if !textOK {
		return fmt.Errorf("prepared migration-review manual SpeechAct source is invalid")
	}
	expectedProtocol, err := canonicalMigrationReviewProtocolPins()
	if err != nil {
		return err
	}
	checks := []struct {
		matches bool
		name    string
	}{
		{matches: record.reviewRef.String() == state.admissionRef.String(), name: "admission ref"},
		{matches: record.content.ref == state.content.ref, name: "content ref"},
		{matches: record.content.digest.Equal(state.content.digest), name: "content digest"},
		{matches: record.speechActRef.String() == bindings.speechActRef, name: "SpeechAct ref"},
		{matches: record.speechActDigest.String() == bindings.speechActDigest, name: "SpeechAct digest"},
		{matches: record.captureRef == bindings.captureRef, name: "capture ref"},
		{matches: record.captureDigest.String() == bindings.captureDigest, name: "capture digest"},
		{matches: record.reviewDigest.String() == bindings.reviewDigest, name: "review digest"},
		{matches: record.reviewText == bindings.reviewText, name: "review text"},
		{matches: record.reviewText == reviewText, name: "canonical review text"},
		{matches: sameMigrationReviewProtocolPins(record.protocol, expectedProtocol), name: "sealed protocol"},
		{matches: record.admittedAt.Equal(canonicalReviewTime(bindings.occurredAt)), name: "SpeechAct occurrence"},
	}
	return firstMigrationReviewMismatch(checks, "exact replay")
}

func sameMigrationReviewAdmissionV2(
	left migrationReviewAdmissionRecordV2,
	right migrationReviewAdmissionRecordV2,
) bool {
	return left.reviewRef.String() == right.reviewRef.String() &&
		left.admissionDigest.Equal(right.admissionDigest) &&
		left.content.ref == right.content.ref &&
		left.content.digest.Equal(right.content.digest) &&
		left.speechActRef.String() == right.speechActRef.String() &&
		left.speechActDigest.Equal(right.speechActDigest) &&
		left.captureRef == right.captureRef &&
		left.captureDigest.Equal(right.captureDigest) &&
		left.reviewText == right.reviewText &&
		left.reviewDigest.Equal(right.reviewDigest) &&
		sameMigrationReviewProtocolPins(left.protocol, right.protocol) &&
		left.effectDigest.Equal(right.effectDigest) &&
		bytes.Equal(left.canonical, right.canonical) &&
		bytes.Equal(left.effectCanonical, right.effectCanonical)
}

func validateMigrationReviewAgainstCurrent(
	record migrationReviewAdmissionRecordV2,
	carrier FinalCandidatePacketCarrier,
	audit PacketPartitionAudit,
) error {
	root, err := reviewCarrierProjectRoot(carrier)
	if err != nil {
		return err
	}
	packet := carrier.Packet()
	basis := carrier.ReviewBasis()
	carrierLeft, err := marshalMigrationReviewFragment(
		canonicalReviewCarrierDTOs(record.targetCarrierDigests),
	)
	if err != nil {
		return err
	}
	carrierRight, err := marshalMigrationReviewFragment(
		canonicalReviewCarrierDTOs(basis.CarrierDigests()),
	)
	if err != nil {
		return err
	}
	lifecycleLeft, err := marshalMigrationReviewFragment(
		canonicalLifecycleIntentDTOs(record.lifecycleIntent),
	)
	if err != nil {
		return err
	}
	lifecycleRight, err := marshalMigrationReviewFragment(
		canonicalLifecycleIntentDTOs(basis.LifecycleIntent()),
	)
	if err != nil {
		return err
	}
	binding := audit.Binding()
	checks := []struct {
		matches bool
		name    string
	}{
		{matches: record.projectRoot.String() == root.String(), name: "project root"},
		{matches: record.packetDigest.Equal(carrier.PacketDigest()), name: "packet digest"},
		{matches: record.packetCarrierDigest.Equal(carrier.CarrierDigest()), name: "packet-carrier digest"},
		{matches: record.partitionAudit.Schema() == binding.Schema(), name: "partition-audit schema"},
		{matches: record.partitionAudit.Status() == binding.Status(), name: "partition-audit status"},
		{matches: record.partitionAudit.Digest().Equal(binding.Digest()), name: "partition-audit digest"},
		{matches: record.sourceCarrier.String() == packet.Source().Carrier().String(), name: "source carrier"},
		{matches: record.sourceDigest.Equal(packet.Source().Digest()), name: "source digest"},
		{matches: bytes.Equal(carrierLeft, carrierRight), name: "review carrier bindings"},
		{matches: record.fpfRevision.String() == basis.FPFRevision().String(), name: "FPF revision"},
		{matches: record.semanticZeroPass.Carrier().String() == basis.SemanticZeroPass().Carrier().String(), name: "semantic zero-pass carrier"},
		{matches: record.semanticZeroPass.Digest().Equal(basis.SemanticZeroPass().Digest()), name: "semantic zero-pass digest"},
		{matches: bytes.Equal(lifecycleLeft, lifecycleRight), name: "lifecycle intent"},
	}
	return firstMigrationReviewMismatch(checks, "current admission")
}

func packetPartitionAuditBindingFromRow(
	schema string,
	status string,
	digestRaw string,
) (PacketPartitionAuditBinding, error) {
	digest, err := NewSHA256(digestRaw)
	if err != nil {
		return PacketPartitionAuditBinding{}, err
	}
	binding := PacketPartitionAuditBinding{
		schema: schema,
		status: PacketPartitionAuditStatus(status),
		digest: PacketPartitionAuditDigest{value: digest},
	}
	if !binding.valid() {
		return PacketPartitionAuditBinding{}, fmt.Errorf("migration-review partition-audit binding is invalid")
	}
	return binding, nil
}

func validateCurrentReviewEnvironment(
	ctx context.Context,
	root ApplyProjectRoot,
	carrier FinalCandidatePacketCarrier,
) error {
	if err := validatePacketCarrierForReviewAdmission(carrier); err != nil {
		return err
	}
	packet := carrier.Packet()
	source := packet.Source()
	sourceBytes, err := readCarrier(root, source.Carrier())
	if err != nil {
		return fmt.Errorf("revalidate semantic-review source carrier: %w", err)
	}
	if !source.Digest().equalBytes(sourceBytes) || uint64(len(sourceBytes)) != source.ByteLength().Value() {
		return fmt.Errorf("semantic-review source carrier no longer matches the final-candidate packet")
	}
	bindings := carrier.ReviewBasis().CarrierDigests().Values()
	for _, binding := range bindings {
		if err := validateExactReviewCarrierBytes(root, binding); err != nil {
			return err
		}
	}
	semantic := carrier.ReviewBasis().SemanticZeroPass()
	semanticBinding := ReviewCarrierDigest{
		role:    ReviewCarrierRole("semantic_zero_pass"),
		carrier: semantic.Carrier(),
		digest:  semantic.Digest(),
	}
	if err := validateExactReviewCarrierBytes(root, semanticBinding); err != nil {
		return err
	}
	return validateCurrentFPFRevision(ctx, root, carrier.ReviewBasis().FPFRevision())
}

func validateExactReviewCarrierBytes(
	root ApplyProjectRoot,
	binding ReviewCarrierDigest,
) error {
	carrier, err := NewSourceCarrierID(binding.carrier.String())
	if err != nil {
		return err
	}
	content, err := readCarrier(root, carrier)
	if err != nil {
		return fmt.Errorf("revalidate %s review carrier: %w", binding.role, err)
	}
	if !DigestBytes(content).Equal(binding.digest) {
		return fmt.Errorf("%s review carrier no longer matches its exact digest", binding.role)
	}
	return nil
}

func validateCurrentFPFRevision(
	ctx context.Context,
	root ApplyProjectRoot,
	revision FPFRevision,
) error {
	fpfRoot := filepath.Join(root.String(), "data", "FPF")
	info, err := os.Lstat(fpfRoot)
	if err != nil {
		return fmt.Errorf("inspect FPF source root: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("FPF source root must be a real directory")
	}
	repositoryRoot, err := runGit(ctx, fpfRoot, "rev-parse", "--show-toplevel")
	if err != nil {
		return fmt.Errorf("resolve FPF repository root: %w", err)
	}
	observedRoot := strings.TrimSpace(string(repositoryRoot))
	expectedRoot, err := filepath.EvalSymlinks(fpfRoot)
	if err != nil {
		return fmt.Errorf("resolve FPF source root: %w", err)
	}
	resolvedObserved, err := filepath.EvalSymlinks(observedRoot)
	if err != nil {
		return fmt.Errorf("resolve observed FPF repository root: %w", err)
	}
	if resolvedObserved != expectedRoot {
		return fmt.Errorf("FPF repository root does not match data/FPF")
	}
	head, err := runGit(ctx, fpfRoot, "rev-parse", "--verify", "HEAD")
	if err != nil {
		return fmt.Errorf("resolve FPF revision: %w", err)
	}
	if strings.TrimSpace(string(head)) != revision.String() {
		return fmt.Errorf("FPF revision no longer matches the reviewed final candidate")
	}
	status, err := runGit(
		ctx,
		fpfRoot,
		"status",
		"--porcelain=v1",
		"--untracked-files=all",
	)
	if err != nil {
		return fmt.Errorf("inspect FPF worktree: %w", err)
	}
	if len(bytes.TrimSpace(status)) != 0 {
		return fmt.Errorf("FPF worktree is not clean")
	}
	return nil
}

func validateReviewAdmissionContext(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("semantic-review admission context is required")
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("semantic-review admission context is not active: %w", err)
	}
	return nil
}
