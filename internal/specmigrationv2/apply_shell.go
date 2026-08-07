package specmigrationv2

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	profileadmissionsqlite "github.com/m0n0x41d/haft/internal/profileadmission/sqlite"
	"github.com/m0n0x41d/haft/internal/projectprofile"
)

const migrationEffectFilePrefix = "spec-migration-v2"

type effectPoint string

const (
	effectJournalPreparedFileSync      effectPoint = "journal_prepared.file_fsync"
	effectJournalPreparedRename        effectPoint = "journal_prepared.rename"
	effectJournalPreparedDirectorySync effectPoint = "journal_prepared.directory_fsync"
	effectRecoveryPreparedRename       effectPoint = "journal_prepared.recovery_rename"
	effectRecoveryPreparedDirSync      effectPoint = "journal_prepared.recovery_directory_fsync"
	effectTargetStageFileSync          effectPoint = "target_stage.file_fsync"
	effectTargetStageDirectorySync     effectPoint = "target_stage.directory_fsync"
	effectTargetInstallRename          effectPoint = "target_install.rename"
	effectTargetStageRemovalDirSync    effectPoint = "target_install.stage_directory_fsync"
	effectTargetInstallDirectorySync   effectPoint = "target_install.directory_fsync"
	effectJournalTargetFileSync        effectPoint = "journal_target.file_fsync"
	effectJournalTargetRename          effectPoint = "journal_target.rename"
	effectJournalTargetDirectorySync   effectPoint = "journal_target.directory_fsync"
	effectSourceArchiveRename          effectPoint = "source_archive.rename"
	effectArchiveParentCreate          effectPoint = "source_archive.parent_create"
	effectArchiveParentParentSync      effectPoint = "source_archive.parent_parent_fsync"
	effectSourceDirectorySync          effectPoint = "source_archive.source_directory_fsync"
	effectArchiveDirectorySync         effectPoint = "source_archive.archive_directory_fsync"
	effectJournalArchiveFileSync       effectPoint = "journal_archive.file_fsync"
	effectJournalArchiveRename         effectPoint = "journal_archive.rename"
	effectJournalArchiveDirectorySync  effectPoint = "journal_archive.directory_fsync"
	effectLineageFileSync              effectPoint = "lineage.file_fsync"
	effectLineageRename                effectPoint = "lineage.rename"
	effectLineageDirectorySync         effectPoint = "lineage.directory_fsync"
	effectJournalLineageFileSync       effectPoint = "journal_lineage.file_fsync"
	effectJournalLineageRename         effectPoint = "journal_lineage.rename"
	effectJournalLineageDirectorySync  effectPoint = "journal_lineage.directory_fsync"
	effectReceiptFileSync              effectPoint = "receipt.file_fsync"
	effectReceiptRename                effectPoint = "receipt.rename"
	effectReceiptDirectorySync         effectPoint = "receipt.directory_fsync"
	effectJournalReceiptFileSync       effectPoint = "journal_receipt.file_fsync"
	effectJournalReceiptRename         effectPoint = "journal_receipt.rename"
	effectJournalReceiptDirectorySync  effectPoint = "journal_receipt.directory_fsync"
	effectJournalCompleteFileSync      effectPoint = "journal_complete.file_fsync"
	effectJournalCompleteRename        effectPoint = "journal_complete.rename"
	effectJournalCompleteDirectorySync effectPoint = "journal_complete.directory_fsync"
)

type effectFailureHook func(effectPoint) error

type migrationEffectPaths struct {
	lock        string
	journal     string
	targetStage string
	lineage     string
	receipt     string
	source      string
	target      string
	archive     string
}

type migrationEffectPlan struct {
	request      ApplyRequest
	journal      migrationJournal
	paths        migrationEffectPaths
	targetBytes  []byte
	lineageBytes []byte
	receiptBytes []byte
}

type sagaProgress string

const (
	progressPrepared        sagaProgress = "prepared"
	progressTargetInstalled sagaProgress = "target_installed"
	progressSourceArchived  sagaProgress = "source_archived"
	progressLineageWritten  sagaProgress = "lineage_written"
	progressReceiptWritten  sagaProgress = "receipt_written"
	progressCompleted       sagaProgress = "completed"
)

func ApplyMigration(
	ctx context.Context,
	profileService profileadmissionsqlite.Service,
	request ApplyRequest,
) MigrationApplyResult {
	validated, err := revalidateApplyRequest(request)
	if err != nil {
		return applyRejectedFromError(err)
	}
	if err := validateApplyContext(ctx); err != nil {
		return applyRejected{reason: err.Error()}
	}
	paths := effectPaths(validated)
	lock, err := acquireApplyLock(validated, paths)
	if err != nil {
		return recoveryForRequest(validated, JournalPrepared, err)
	}
	defer func() { _ = lock.close() }()
	existing, found, err := loadJournal(paths.journal)
	if err != nil {
		return recoveryForRequest(validated, JournalPrepared, err)
	}
	if found {
		if profileErr := validateHistoricalProfileForEffect(
			ctx,
			profileService,
			validated,
		); profileErr != nil {
			return recoveryForRequest(validated, existing.phase, profileErr)
		}
		if witnessErr := validateGitWitnessAgainstProvenance(
			existing.gitWitness,
			validated.projectRoot,
			validated.analysis.sourceProvenance,
		); witnessErr != nil {
			return recoveryForRequest(validated, existing.phase, witnessErr)
		}
		plan, buildErr := buildMigrationEffectPlan(validated, existing.gitWitness, existing.startedAt)
		if buildErr != nil {
			return recoveryForRequest(validated, existing.phase, buildErr)
		}
		if reviewErr := verifyReviewCarrierBytes(plan); reviewErr != nil {
			return recoveryForPlan(plan, existing.phase, reviewErr)
		}
		if topologyErr := verifyEffectPathTopology(plan); topologyErr != nil {
			return recoveryForPlan(plan, existing.phase, topologyErr)
		}
		return resultForExistingJournal(plan, existing)
	}
	temporaryJournalExists, err := safePathExists(paths.journal + ".tmp")
	if err != nil {
		return recoveryForRequest(validated, JournalPrepared, err)
	}
	if temporaryJournalExists {
		return recoveryForRequest(
			validated,
			JournalPrepared,
			fmt.Errorf("prepared migration journal requires explicit recovery"),
		)
	}
	witnessResult, err := VerifyGitSourceProvenance(
		ctx,
		validated.projectRoot,
		validated.analysis.sourceProvenance,
	)
	if err != nil {
		return applyRejected{reason: err.Error()}
	}
	witness, ok := witnessResult.(gitSourceProvenanceWitness)
	if !ok {
		return applyRejected{reason: "Git provenance witness is not package-owned"}
	}
	plan, err := buildMigrationEffectPlan(validated, witness, validated.requestedAt)
	if err != nil {
		return applyRejected{reason: err.Error()}
	}
	if err := verifyCurrentReviewBasis(ctx, plan); err != nil {
		return applyRejected{reason: err.Error()}
	}
	if err := verifyInitialFilesystemState(plan); err != nil {
		return applyRejected{reason: err.Error()}
	}
	if profileErr := validateCurrentProfileForEffect(
		ctx,
		profileService,
		validated,
	); profileErr != nil {
		return applyRejectedFromError(profileErr)
	}
	// SQLite profile admission and the filesystem saga cannot share one atomic
	// transaction. This is the final current-head gate immediately before the
	// first write. A later concurrent profile revision does not rewrite the
	// journal's exact reliance binding or invalidate recovery of an already-
	// started saga.
	return startMigrationSaga(plan, nil)
}

// RecoverMigration is the only automatic continuation of a partial saga. It
// is sealed behind the journal-bound historical canonical applicability proof
// and admitted semantic review used by ApplyMigration. It derives progress from
// verified filesystem hashes rather than trusting a journal phase that may
// lag a completed rename after a crash.
func RecoverMigration(
	ctx context.Context,
	profileService profileadmissionsqlite.Service,
	reviewService ReviewAdmissionService,
	request RecoveryRequest,
) MigrationApplyResult {
	validatedRecovery, err := NewRecoveryRequest(RecoveryRequestInput{
		ProjectRoot: request.projectRoot,
		Structural:  request.structural,
	})
	if err != nil {
		return applyRejectedFromError(err)
	}
	if err := validateApplyContext(ctx); err != nil {
		return applyRejected{reason: err.Error()}
	}
	provisional := ApplyRequest{
		projectRoot: validatedRecovery.projectRoot,
		structural:  validatedRecovery.structural,
		analysis:    validatedRecovery.analysis,
	}
	paths := effectPaths(provisional)
	lock, err := acquireApplyLock(provisional, paths)
	if err != nil {
		return recoveryForRequest(provisional, JournalPrepared, err)
	}
	defer func() { _ = lock.close() }()
	existing, found, err := loadJournal(paths.journal)
	if err != nil {
		return recoveryForRequest(provisional, JournalPrepared, err)
	}
	validated := ApplyRequest{}
	profileResolved := false
	if !found {
		temporary, temporaryFound, inspectErr := inspectPreparedJournalTemp(paths.journal)
		if inspectErr != nil {
			return recoveryForRequest(provisional, JournalPrepared, inspectErr)
		}
		if temporaryFound {
			validated, err = rehydrateRecoveryApplyRequest(
				ctx,
				profileService,
				reviewService,
				validatedRecovery,
				temporary,
			)
			if err != nil {
				return recoveryForRequest(provisional, temporary.phase, err)
			}
			preflight, buildErr := buildMigrationEffectPlan(
				validated,
				temporary.gitWitness,
				temporary.startedAt,
			)
			if buildErr != nil {
				return recoveryForRequest(validated, temporary.phase, buildErr)
			}
			if planErr := validateJournalForPlan(temporary, preflight); planErr != nil {
				return recoveryForPlan(preflight, temporary.phase, planErr)
			}
			profileResolved = true
		}
		if promoteErr := promotePreparedJournalTemp(paths.journal, nil); promoteErr != nil {
			return recoveryForRequest(provisional, JournalPrepared, promoteErr)
		}
		existing, found, err = loadJournal(paths.journal)
		if err != nil {
			return recoveryForRequest(provisional, JournalPrepared, err)
		}
	}
	if !found {
		return applyRejected{reason: "no durable migration journal exists to recover"}
	}
	if !profileResolved {
		validated, err = rehydrateRecoveryApplyRequest(
			ctx,
			profileService,
			reviewService,
			validatedRecovery,
			existing,
		)
		if err != nil {
			return recoveryForRequest(provisional, existing.phase, err)
		}
	}
	if err := validateJournalForRequest(existing, validated); err != nil {
		return recoveryForRequest(validated, existing.phase, err)
	}
	if err := validateGitWitnessAgainstProvenance(
		existing.gitWitness,
		validated.projectRoot,
		validated.analysis.sourceProvenance,
	); err != nil {
		return recoveryForRequest(validated, existing.phase, err)
	}
	plan, err := buildMigrationEffectPlan(validated, existing.gitWitness, existing.startedAt)
	if err != nil {
		return recoveryForRequest(validated, existing.phase, err)
	}
	if err := validateJournalForPlan(existing, plan); err != nil {
		return recoveryForPlan(plan, existing.phase, err)
	}
	if err := verifyReviewCarrierBytes(plan); err != nil {
		return recoveryForPlan(plan, existing.phase, err)
	}
	if err := verifyEffectPathTopology(plan); err != nil {
		return recoveryForPlan(plan, existing.phase, err)
	}
	progress, err := inspectSagaProgress(plan)
	if err != nil {
		return recoveryForPlan(plan, existing.phase, err)
	}
	if progress == progressCompleted && existing.phase == JournalCompleted {
		if err := resynchronizeObservedPrefix(plan, progressCompleted, nil); err != nil {
			return recoveryForPlan(plan, existing.phase, err)
		}
		receipt, err := verifyCompletedState(plan)
		if err != nil {
			return recoveryForPlan(plan, existing.phase, err)
		}
		receiptCarrier, err := migrationEffectReceiptCarrierForPlan(plan)
		if err != nil {
			return recoveryForPlan(plan, existing.phase, err)
		}
		return replayed{receipt: receipt, receiptCarrier: receiptCarrier}
	}
	if progress == progressCompleted {
		progress = progressReceiptWritten
	}
	if err := resynchronizeObservedPrefix(plan, progress, nil); err != nil {
		return recoveryForPlan(plan, existing.phase, err)
	}
	aligned, err := alignJournalToProgress(plan, existing, progress, nil)
	if err != nil {
		return recoveryForPlan(plan, existing.phase, err)
	}
	return continueMigrationSaga(plan, aligned, progress, nil)
}

func rehydrateRecoveryApplyRequest(
	ctx context.Context,
	profileService profileadmissionsqlite.Service,
	reviewService ReviewAdmissionService,
	request RecoveryRequest,
	journal migrationJournal,
) (ApplyRequest, error) {
	admissionRef, err := projectprofile.NewProfileDeclarationAdmissionRecordRef(
		journal.profileAdmissionRef,
	)
	if err != nil {
		return ApplyRequest{}, fmt.Errorf("recovery journal profile admission ref is invalid: %w", err)
	}
	admissionDigest, err := projectprofile.NewContentDigest(journal.profileAdmissionHash)
	if err != nil {
		return ApplyRequest{}, fmt.Errorf("recovery journal profile admission digest is invalid: %w", err)
	}
	projectRoot, err := projectprofile.NewProjectRootV1(request.projectRoot.String())
	if err != nil {
		return ApplyRequest{}, fmt.Errorf("recovery project root is invalid: %w", err)
	}
	applicability := profileService.ResolveHistoricalSoftwareSystemSpecMigration(
		ctx,
		projectRoot,
		admissionRef,
		admissionDigest,
	)
	required, ok := applicability.Required()
	if !ok {
		return ApplyRequest{}, ProfileApplicabilityPreconditionError{
			precondition: ProfileApplicabilityUnderdetermined,
		}
	}
	reviewResult, err := reviewService.resolveHistorical(
		ctx,
		request.projectRoot,
		journal.semanticReviewRef,
		journal.semanticAdmissionHash,
	)
	if err != nil {
		return ApplyRequest{}, fmt.Errorf("resolve journal-bound historical semantic review: %w", err)
	}
	input := ApplyRequestInput{
		ProjectRoot:          request.projectRoot,
		Structural:           request.structural,
		ProfileApplicability: required,
		Review:               reviewResult,
		RequestedAt:          journal.startedAt,
	}
	return newApplyRequestShape(input)
}

func validateApplyContext(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("migration apply context is required")
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("migration apply context is not active: %w", err)
	}
	return nil
}

func applyRejectedFromError(err error) applyRejected {
	var profileError ProfileApplicabilityPreconditionError
	if !errors.As(err, &profileError) {
		return applyRejected{code: ApplyRejectionInvalidRequest, reason: err.Error()}
	}
	code := ApplyRejectionProfileProofInvalid
	switch profileError.Precondition() {
	case ProfileApplicabilityNotCurrent:
		code = ApplyRejectionProfileApplicabilityNotCurrent
	case ProfileApplicabilityNotApplicable:
		code = ApplyRejectionProfileNotApplicable
	case ProfileApplicabilityUnderdetermined:
		code = ApplyRejectionProfileApplicabilityUndetermined
	}
	return applyRejected{code: code, reason: err.Error()}
}

func promotePreparedJournalTemp(path string, hook effectFailureHook) error {
	exists, err := safePathExists(path)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	_, found, err := inspectPreparedJournalTemp(path)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	temporary := path + ".tmp"
	if err := renameNoReplace(temporary, path); err != nil {
		return err
	}
	if err := runEffectHook(hook, effectRecoveryPreparedRename); err != nil {
		return err
	}
	parent := filepath.Dir(path)
	if err := syncDirectoryNoFollow(parent); err != nil {
		return err
	}
	return runEffectHook(hook, effectRecoveryPreparedDirSync)
}

func inspectPreparedJournalTemp(
	path string,
) (migrationJournal, bool, error) {
	temporary := path + ".tmp"
	content, err := readRegularFileNoFollow(temporary)
	if errors.Is(err, os.ErrNotExist) {
		return migrationJournal{}, false, nil
	}
	if err != nil {
		return migrationJournal{}, false, err
	}
	journal, err := decodeJournal(content)
	if err != nil {
		return migrationJournal{}, false, fmt.Errorf("temporary migration journal is invalid: %w", err)
	}
	if journal.phase != JournalPrepared {
		return migrationJournal{}, false, fmt.Errorf("orphan temporary journal is not the initial prepared record")
	}
	return journal, true, nil
}

func revalidateApplyRequest(request ApplyRequest) (ApplyRequest, error) {
	input := ApplyRequestInput{
		ProjectRoot:          request.projectRoot,
		Structural:           request.structural,
		ProfileApplicability: request.profileApplicability,
		Review:               request.review,
		RequestedAt:          request.requestedAt,
	}
	return newApplyRequestShape(input)
}

func validateCurrentProfileForEffect(
	ctx context.Context,
	profileService profileadmissionsqlite.Service,
	request ApplyRequest,
) error {
	validation := profileService.ValidateCurrentSoftwareSystemSpecMigrationRequired(
		ctx,
		request.profileApplicability,
	)
	return profileValidationError(validation)
}

func validateHistoricalProfileForEffect(
	ctx context.Context,
	profileService profileadmissionsqlite.Service,
	request ApplyRequest,
) error {
	validation := profileService.ValidateHistoricalSoftwareSystemSpecMigrationRequired(
		ctx,
		request.profileApplicability,
	)
	return profileValidationError(validation)
}

func acquireApplyLock(
	request ApplyRequest,
	paths migrationEffectPaths,
) (migrationLock, error) {
	root := request.projectRoot.String()
	haftDirectory := filepath.Join(root, ".haft")
	info, err := os.Lstat(haftDirectory)
	if err != nil {
		return migrationLock{}, fmt.Errorf("migration apply requires an existing .haft directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return migrationLock{}, fmt.Errorf("migration apply .haft carrier must be a real directory")
	}
	return acquireMigrationLock(paths.lock)
}

func buildMigrationEffectPlan(
	request ApplyRequest,
	witness gitSourceProvenanceWitness,
	startedAt time.Time,
) (migrationEffectPlan, error) {
	if err := validateGitWitnessAgainstProvenance(
		witness,
		request.projectRoot,
		request.analysis.sourceProvenance,
	); err != nil {
		return migrationEffectPlan{}, err
	}
	if !witness.digest.valid() || startedAt.IsZero() {
		return migrationEffectPlan{}, fmt.Errorf("migration effect witness and start time are required")
	}
	reviewDigest, err := semanticReviewDigest(request.review)
	if err != nil {
		return migrationEffectPlan{}, err
	}
	lineageBytes, err := encodeLineageRecord(
		request.analysis.packetID,
		request.analysis.packetDigest,
		request.analysis.lineagePolicy,
		request.analysis.lineageDigest,
	)
	if err != nil {
		return migrationEffectPlan{}, err
	}
	profileBinding := request.profileBinding
	if err := validateOpaqueProfileBindingShape(
		profileBinding.ref,
		profileBinding.digest,
		profileBinding.ledgerRevision,
	); err != nil {
		return migrationEffectPlan{}, fmt.Errorf("migration profile proof binding is unavailable: %w", err)
	}
	journal := migrationJournal{
		migrationID:           request.analysis.packetID,
		packetDigest:          request.analysis.packetDigest,
		projectRoot:           request.projectRoot,
		sourceCarrier:         request.analysis.sourceCarrier,
		sourceDigest:          request.analysis.sourceDigest,
		targetCarrier:         request.analysis.targetCarrier,
		targetDigest:          request.analysis.targetDigest,
		archiveCarrier:        request.analysis.archiveCarrier,
		lineageDigest:         request.analysis.lineageDigest,
		lineageRecordDigest:   DigestBytes(lineageBytes),
		profileAdmissionRef:   profileBinding.ref,
		profileAdmissionHash:  profileBinding.digest,
		profileLedgerRevision: profileBinding.ledgerRevision,
		semanticReviewRef:     request.review.reviewRef,
		semanticAdmissionHash: request.review.admissionDigest,
		semanticReviewDigest:  reviewDigest,
		gitWitness:            witness,
		gitWitnessDigest:      witness.digest,
		phase:                 JournalPrepared,
		startedAt:             startedAt.UTC(),
		updatedAt:             startedAt.UTC(),
	}
	receipt := receiptFromJournal(journal)
	receiptBytes, err := encodeReceipt(receipt)
	if err != nil {
		return migrationEffectPlan{}, err
	}
	journal.receiptDigest = DigestBytes(receiptBytes)
	if err := validateJournal(journal); err != nil {
		return migrationEffectPlan{}, err
	}
	return migrationEffectPlan{
		request:      request,
		journal:      journal,
		paths:        effectPaths(request),
		targetBytes:  request.structural.target.Bytes(),
		lineageBytes: lineageBytes,
		receiptBytes: receiptBytes,
	}, nil
}

func receiptFromJournal(journal migrationJournal) MigrationEffectReceipt {
	return MigrationEffectReceipt{
		migrationID:                   journal.migrationID,
		packetDigest:                  journal.packetDigest,
		sourceDigest:                  journal.sourceDigest,
		targetDigest:                  journal.targetDigest,
		lineageDigest:                 journal.lineageDigest,
		profileAdmissionRef:           journal.profileAdmissionRef,
		profileAdmissionHash:          journal.profileAdmissionHash,
		profileLedgerRevision:         journal.profileLedgerRevision,
		semanticReviewRef:             journal.semanticReviewRef,
		semanticReviewAdmissionDigest: journal.semanticAdmissionHash,
		semanticReviewDigest:          journal.semanticReviewDigest,
		gitWitnessDigest:              journal.gitWitnessDigest,
		appliedAt:                     journal.startedAt,
	}
}

func effectPaths(request ApplyRequest) migrationEffectPaths {
	root := request.projectRoot.String()
	packetID := request.analysis.packetID.String()
	packetIDBytes := []byte(packetID)
	keyDigest := DigestBytes(packetIDBytes)
	keyDigestText := keyDigest.String()
	key := strings.TrimPrefix(keyDigestText, "sha256:")
	base := filepath.Join(root, ".haft")
	stem := migrationEffectFilePrefix + "." + key
	sourceCarrier := request.analysis.sourceCarrier.String()
	targetCarrier := request.analysis.targetCarrier.String()
	archiveCarrier := request.analysis.archiveCarrier.String()
	sourcePath := filepath.FromSlash(sourceCarrier)
	targetPath := filepath.FromSlash(targetCarrier)
	archivePath := filepath.FromSlash(archiveCarrier)
	return migrationEffectPaths{
		lock:        filepath.Join(root, ".haft"),
		journal:     filepath.Join(base, stem+".journal.json"),
		targetStage: filepath.Join(base, stem+".target.stage"),
		lineage:     filepath.Join(base, stem+".lineage.json"),
		receipt:     filepath.Join(base, stem+".receipt.json"),
		source:      filepath.Join(root, sourcePath),
		target:      filepath.Join(root, targetPath),
		archive:     filepath.Join(root, archivePath),
	}
}

func migrationEffectReceiptCarrierForPlan(
	plan migrationEffectPlan,
) (MigrationEffectReceiptCarrier, error) {
	return newMigrationEffectReceiptCarrier(
		plan.request.projectRoot,
		plan.paths.receipt,
		plan.journal.receiptDigest,
	)
}

func verifyReviewCarrierBytes(plan migrationEffectPlan) error {
	for _, binding := range plan.request.review.targetCarrierDigests.Values() {
		root := plan.request.projectRoot.String()
		carrier := binding.carrier.String()
		carrierPath := filepath.FromSlash(carrier)
		path := filepath.Join(root, carrierPath)
		if err := verifyConfinedPathComponents(root, path); err != nil {
			return err
		}
		content, err := readRegularFileNoFollow(path)
		if err != nil {
			return fmt.Errorf("read reviewed carrier %s: %w", carrier, err)
		}
		observedDigest := DigestBytes(content)
		observedDigestText := observedDigest.String()
		expectedDigestText := binding.digest.String()
		if observedDigestText != expectedDigestText {
			return fmt.Errorf("reviewed carrier %s drifted from admitted bytes", carrier)
		}
	}
	return nil
}

// verifyCurrentReviewBasis is a fresh-apply gate. Recovery deliberately uses
// the historical admission binding and the narrower immutable-carrier check,
// because already-started Work must not be recast as a new semantic choice.
func verifyCurrentReviewBasis(
	ctx context.Context,
	plan migrationEffectPlan,
) error {
	if err := verifyReviewCarrierBytes(plan); err != nil {
		return err
	}
	review := plan.request.review
	semantic := review.semanticZeroPass
	binding := ReviewCarrierDigest{
		role:    ReviewCarrierRole("semantic_zero_pass"),
		carrier: semantic.Carrier(),
		digest:  semantic.Digest(),
	}
	if err := validateExactReviewCarrierBytes(plan.request.projectRoot, binding); err != nil {
		return err
	}
	return validateCurrentFPFRevision(
		ctx,
		plan.request.projectRoot,
		review.fpfRevision,
	)
}

func verifyInitialFilesystemState(plan migrationEffectPlan) error {
	if err := verifyEffectPathTopology(plan); err != nil {
		return err
	}
	sourceBytes, err := readRegularFileNoFollow(plan.paths.source)
	if err != nil {
		return fmt.Errorf("read migration source: %w", err)
	}
	observedSourceDigest := SourceDigestOf(sourceBytes)
	if !observedSourceDigest.Equal(plan.journal.sourceDigest) {
		return fmt.Errorf("migration source drifted after structural review")
	}
	for _, path := range []string{
		plan.paths.journal,
		plan.paths.journal + ".tmp",
		plan.paths.targetStage,
		plan.paths.target,
		plan.paths.archive,
		plan.paths.lineage,
		plan.paths.lineage + ".tmp",
		plan.paths.receipt,
		plan.paths.receipt + ".tmp",
	} {
		exists, err := safePathExists(path)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("migration destination collision at %s", path)
		}
	}
	return nil
}

func startMigrationSaga(
	plan migrationEffectPlan,
	hook effectFailureHook,
) MigrationApplyResult {
	journal := plan.journal
	if err := persistJournal(plan.paths.journal, journal, effectJournalPreparedFileSync, effectJournalPreparedRename, effectJournalPreparedDirectorySync, hook); err != nil {
		return recoveryForPlan(plan, JournalPrepared, err)
	}
	return continueMigrationSaga(plan, journal, progressPrepared, hook)
}

func continueMigrationSaga(
	plan migrationEffectPlan,
	journal migrationJournal,
	progress sagaProgress,
	hook effectFailureHook,
) MigrationApplyResult {
	var err error
	if progress == progressPrepared {
		err = installReviewedTarget(plan, hook)
		if err != nil {
			return recoveryForPlan(plan, journal.phase, err)
		}
		journal, err = advanceJournal(plan, journal, JournalTargetInstalled, hook)
		if err != nil {
			return recoveryForPlan(plan, JournalTargetInstalled, err)
		}
		progress = progressTargetInstalled
	}
	if progress == progressTargetInstalled {
		err = archiveDesignatedSource(plan, hook)
		if err != nil {
			return recoveryForPlan(plan, journal.phase, err)
		}
		journal, err = advanceJournal(plan, journal, JournalSourceArchived, hook)
		if err != nil {
			return recoveryForPlan(plan, JournalSourceArchived, err)
		}
		progress = progressSourceArchived
	}
	if progress == progressSourceArchived {
		err = persistArtifact(plan.paths.lineage, plan.lineageBytes, 0o600, false, effectLineageFileSync, effectLineageRename, effectLineageDirectorySync, hook)
		if err != nil {
			return recoveryForPlan(plan, journal.phase, err)
		}
		journal, err = advanceJournal(plan, journal, JournalLineageWritten, hook)
		if err != nil {
			return recoveryForPlan(plan, JournalLineageWritten, err)
		}
		progress = progressLineageWritten
	}
	if progress == progressLineageWritten {
		if err := verifyEffectEvidenceBeforeReceipt(plan); err != nil {
			return recoveryForPlan(plan, journal.phase, err)
		}
		err = persistArtifact(plan.paths.receipt, plan.receiptBytes, 0o600, false, effectReceiptFileSync, effectReceiptRename, effectReceiptDirectorySync, hook)
		if err != nil {
			return recoveryForPlan(plan, journal.phase, err)
		}
		journal, err = advanceJournal(plan, journal, JournalReceiptWritten, hook)
		if err != nil {
			return recoveryForPlan(plan, JournalReceiptWritten, err)
		}
		progress = progressReceiptWritten
	}
	if progress == progressReceiptWritten {
		journal, err = advanceJournal(plan, journal, JournalCompleted, hook)
		if err != nil {
			return recoveryForPlan(plan, JournalCompleted, err)
		}
		progress = progressCompleted
	}
	if progress != progressCompleted {
		return recoveryForPlan(plan, journal.phase, fmt.Errorf("migration saga stopped at unknown progress %q", progress))
	}
	receipt, err := verifyCompletedState(plan)
	if err != nil {
		return recoveryForPlan(plan, journal.phase, err)
	}
	receiptCarrier, err := migrationEffectReceiptCarrierForPlan(plan)
	if err != nil {
		return recoveryForPlan(plan, journal.phase, err)
	}
	return applied{receipt: receipt, receiptCarrier: receiptCarrier}
}

func verifyEffectEvidenceBeforeReceipt(plan migrationEffectPlan) error {
	if err := verifyReviewCarrierBytes(plan); err != nil {
		return err
	}
	progress, err := inspectSagaProgress(plan)
	if err != nil {
		return err
	}
	if progress != progressLineageWritten {
		return fmt.Errorf("migration effects are not exact before receipt issuance")
	}
	return nil
}

func installReviewedTarget(plan migrationEffectPlan, hook effectFailureHook) error {
	if err := writeDurableStage(plan.paths.targetStage, plan.targetBytes, hook); err != nil {
		return err
	}
	if err := verifyReviewedTargetAtPath(plan.paths.targetStage, plan.journal.targetDigest); err != nil {
		return err
	}
	if err := renameNoReplace(plan.paths.targetStage, plan.paths.target); err != nil {
		return fmt.Errorf("install reviewed target: %w", err)
	}
	if err := runEffectHook(hook, effectTargetInstallRename); err != nil {
		return err
	}
	stageParent := filepath.Dir(plan.paths.targetStage)
	if err := syncDirectoryNoFollow(stageParent); err != nil {
		return err
	}
	if err := runEffectHook(hook, effectTargetStageRemovalDirSync); err != nil {
		return err
	}
	targetParent := filepath.Dir(plan.paths.target)
	if err := syncDirectoryNoFollow(targetParent); err != nil {
		return err
	}
	if err := runEffectHook(hook, effectTargetInstallDirectorySync); err != nil {
		return err
	}
	return verifyReviewedTargetAtPath(plan.paths.target, plan.journal.targetDigest)
}

func verifyReviewedTargetAtPath(path string, expected TargetDigest) error {
	content, err := readRegularFileNoFollow(path)
	if err != nil {
		return fmt.Errorf("recheck reviewed target at %s: %w", path, err)
	}
	observed := TargetDigestOf(content)
	if !observed.Equal(expected) {
		return fmt.Errorf("reviewed target bytes drifted at %s", path)
	}
	return nil
}

func archiveDesignatedSource(plan migrationEffectPlan, hook effectFailureHook) error {
	if err := prepareArchiveParent(plan, hook); err != nil {
		return err
	}
	if err := verifySourceReadyForArchive(plan); err != nil {
		return err
	}
	if err := renameNoReplace(plan.paths.source, plan.paths.archive); err != nil {
		return fmt.Errorf("archive designated source: %w", err)
	}
	if err := runEffectHook(hook, effectSourceArchiveRename); err != nil {
		return err
	}
	sourceParent := filepath.Dir(plan.paths.source)
	if err := syncDirectoryNoFollow(sourceParent); err != nil {
		return err
	}
	if err := runEffectHook(hook, effectSourceDirectorySync); err != nil {
		return err
	}
	archiveParent := filepath.Dir(plan.paths.archive)
	if sourceParent == archiveParent {
		return nil
	}
	if err := syncDirectoryNoFollow(archiveParent); err != nil {
		return err
	}
	return runEffectHook(hook, effectArchiveDirectorySync)
}

func verifySourceReadyForArchive(plan migrationEffectPlan) error {
	content, err := readRegularFileNoFollow(plan.paths.source)
	if err != nil {
		return fmt.Errorf("recheck designated source before archive: %w", err)
	}
	observed := SourceDigestOf(content)
	if !observed.Equal(plan.journal.sourceDigest) {
		return fmt.Errorf("designated source drifted before archive install")
	}
	return nil
}

func prepareArchiveParent(plan migrationEffectPlan, hook effectFailureHook) error {
	root := plan.request.projectRoot.String()
	archiveParent := filepath.Dir(plan.paths.archive)
	chain, err := archiveDirectoryChain(root, archiveParent)
	if err != nil {
		return err
	}
	for _, directory := range chain {
		exists, err := safePathExists(directory)
		if err != nil {
			return err
		}
		if !exists {
			if err := os.Mkdir(directory, 0o700); err != nil {
				return fmt.Errorf("create migration archive directory: %w", err)
			}
			if err := runEffectHook(hook, effectArchiveParentCreate); err != nil {
				return err
			}
		}
		if err := requireRealDirectory(directory); err != nil {
			return err
		}
		needsParentSync := !exists || directory == archiveParent
		if !needsParentSync {
			continue
		}
		parent := filepath.Dir(directory)
		if err := syncDirectoryNoFollow(parent); err != nil {
			return err
		}
		if err := runEffectHook(hook, effectArchiveParentParentSync); err != nil {
			return err
		}
	}
	sourceParent := filepath.Dir(plan.paths.source)
	paths := []string{sourceParent, archiveParent}
	return sameFilesystem(paths)
}

func advanceJournal(
	plan migrationEffectPlan,
	journal migrationJournal,
	phase JournalPhase,
	hook effectFailureHook,
) (migrationJournal, error) {
	journal.phase = phase
	journal.updatedAt = plan.request.requestedAt.UTC()
	fileSync, rename, directorySync := journalEffectPoints(phase)
	err := persistJournal(plan.paths.journal, journal, fileSync, rename, directorySync, hook)
	return journal, err
}

func journalEffectPoints(phase JournalPhase) (effectPoint, effectPoint, effectPoint) {
	switch phase {
	case JournalTargetInstalled:
		return effectJournalTargetFileSync, effectJournalTargetRename, effectJournalTargetDirectorySync
	case JournalSourceArchived:
		return effectJournalArchiveFileSync, effectJournalArchiveRename, effectJournalArchiveDirectorySync
	case JournalLineageWritten:
		return effectJournalLineageFileSync, effectJournalLineageRename, effectJournalLineageDirectorySync
	case JournalReceiptWritten:
		return effectJournalReceiptFileSync, effectJournalReceiptRename, effectJournalReceiptDirectorySync
	case JournalCompleted:
		return effectJournalCompleteFileSync, effectJournalCompleteRename, effectJournalCompleteDirectorySync
	default:
		return effectJournalPreparedFileSync, effectJournalPreparedRename, effectJournalPreparedDirectorySync
	}
}

func persistJournal(
	path string,
	journal migrationJournal,
	fileSync effectPoint,
	rename effectPoint,
	directorySync effectPoint,
	hook effectFailureHook,
) error {
	encoded, err := encodeJournal(journal)
	if err != nil {
		return err
	}
	replaceExisting := journal.phase != JournalPrepared
	return persistArtifact(path, encoded, 0o600, replaceExisting, fileSync, rename, directorySync, hook)
}

func persistArtifact(
	path string,
	content []byte,
	mode os.FileMode,
	replaceExisting bool,
	fileSync effectPoint,
	rename effectPoint,
	directorySync effectPoint,
	hook effectFailureHook,
) error {
	parent := filepath.Dir(path)
	if err := requireRealDirectory(parent); err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err := writeExclusiveOrVerify(temporary, content, mode); err != nil {
		return err
	}
	if err := runEffectHook(hook, fileSync); err != nil {
		return err
	}
	if err := installPersistedArtifact(temporary, path, replaceExisting); err != nil {
		return err
	}
	if err := runEffectHook(hook, rename); err != nil {
		return err
	}
	if err := syncDirectoryNoFollow(parent); err != nil {
		return err
	}
	return runEffectHook(hook, directorySync)
}

func writeDurableStage(path string, content []byte, hook effectFailureHook) error {
	parent := filepath.Dir(path)
	if err := requireRealDirectory(parent); err != nil {
		return err
	}
	if err := writeExclusiveOrVerify(path, content, 0o644); err != nil {
		return err
	}
	if err := runEffectHook(hook, effectTargetStageFileSync); err != nil {
		return err
	}
	if err := syncDirectoryNoFollow(parent); err != nil {
		return err
	}
	return runEffectHook(hook, effectTargetStageDirectorySync)
}

func writeExclusiveOrVerify(path string, content []byte, mode os.FileMode) error {
	file, err := openExclusiveNoFollow(path, mode)
	if err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	if errors.Is(err, os.ErrExist) {
		existing, readErr := readRegularFileNoFollow(path)
		if readErr != nil {
			return readErr
		}
		if !bytes.Equal(existing, content) {
			existingDigest := DigestBytes(existing)
			expectedDigest := DigestBytes(content)
			return fmt.Errorf(
				"migration temporary-file collision at %s: observed %s, expected %s",
				path,
				existingDigest.String(),
				expectedDigest.String(),
			)
		}
		return nil
	}
	writeErr := writeAndSync(file, content)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func installPersistedArtifact(source string, target string, replaceExisting bool) error {
	if replaceExisting {
		if err := os.Rename(source, target); err != nil {
			return fmt.Errorf("replace migration journal: %w", err)
		}
		return nil
	}
	return renameNoReplace(source, target)
}

func writeAndSync(file *os.File, content []byte) error {
	written, err := file.Write(content)
	if err != nil {
		return err
	}
	if written != len(content) {
		return io.ErrShortWrite
	}
	return file.Sync()
}

func runEffectHook(hook effectFailureHook, point effectPoint) error {
	if hook == nil {
		return nil
	}
	if err := hook(point); err != nil {
		return fmt.Errorf("injected effect failure after %s: %w", point, err)
	}
	return nil
}

func loadJournal(path string) (migrationJournal, bool, error) {
	content, err := readRegularFileNoFollow(path)
	if errors.Is(err, os.ErrNotExist) {
		return migrationJournal{}, false, nil
	}
	if err != nil {
		return migrationJournal{}, false, err
	}
	journal, err := decodeJournal(content)
	return journal, true, err
}

func resultForExistingJournal(
	plan migrationEffectPlan,
	journal migrationJournal,
) MigrationApplyResult {
	if err := validateJournalForPlan(journal, plan); err != nil {
		return recoveryForPlan(plan, journal.phase, err)
	}
	if journal.phase != JournalCompleted {
		return recoveryForPlan(plan, journal.phase, fmt.Errorf("partial migration journal requires explicit recovery"))
	}
	if err := resynchronizeObservedPrefix(plan, progressCompleted, nil); err != nil {
		return recoveryForPlan(plan, journal.phase, err)
	}
	receipt, err := verifyCompletedState(plan)
	if err != nil {
		return recoveryForPlan(plan, journal.phase, err)
	}
	receiptCarrier, err := migrationEffectReceiptCarrierForPlan(plan)
	if err != nil {
		return recoveryForPlan(plan, journal.phase, err)
	}
	return replayed{receipt: receipt, receiptCarrier: receiptCarrier}
}

func resynchronizeObservedPrefix(
	plan migrationEffectPlan,
	progress sagaProgress,
	hook effectFailureHook,
) error {
	directories := durabilityDirectoriesForProgress(plan, progress)
	for _, directory := range directories {
		if err := syncDirectoryNoFollow(directory); err != nil {
			return fmt.Errorf("reconcile migration directory durability at %s: %w", directory, err)
		}
		point, err := recoveryDirectoryEffectPoint(plan, directory)
		if err != nil {
			return err
		}
		if err := runEffectHook(hook, point); err != nil {
			return err
		}
	}
	return nil
}

func recoveryDirectoryEffectPoint(
	plan migrationEffectPlan,
	directory string,
) (effectPoint, error) {
	root := plan.request.projectRoot.String()
	relative, err := confinedRelativePath(root, directory)
	if err != nil {
		return "", err
	}
	slashed := filepath.ToSlash(relative)
	return effectPoint("recovery_observed_directory_fsync:" + slashed), nil
}

func durabilityDirectoriesForProgress(
	plan migrationEffectPlan,
	progress sagaProgress,
) []string {
	journalParent := filepath.Dir(plan.paths.journal)
	stageParent := filepath.Dir(plan.paths.targetStage)
	targetParent := filepath.Dir(plan.paths.target)
	sourceParent := filepath.Dir(plan.paths.source)
	archiveParent := filepath.Dir(plan.paths.archive)
	lineageParent := filepath.Dir(plan.paths.lineage)
	receiptParent := filepath.Dir(plan.paths.receipt)
	directories := []string{journalParent}
	rank := sagaProgressRank(progress)
	targetRank := sagaProgressRank(progressTargetInstalled)
	archiveRank := sagaProgressRank(progressSourceArchived)
	lineageRank := sagaProgressRank(progressLineageWritten)
	receiptRank := sagaProgressRank(progressReceiptWritten)
	if rank >= targetRank {
		directories = append(
			directories,
			stageParent,
			targetParent,
		)
	}
	if rank >= archiveRank {
		directories = append(
			directories,
			sourceParent,
			archiveParent,
		)
	}
	if rank >= lineageRank {
		directories = append(directories, lineageParent)
	}
	if rank >= receiptRank {
		directories = append(directories, receiptParent)
	}
	return uniquePaths(directories)
}

func sagaProgressRank(progress sagaProgress) int {
	switch progress {
	case progressPrepared:
		return 0
	case progressTargetInstalled:
		return 1
	case progressSourceArchived:
		return 2
	case progressLineageWritten:
		return 3
	case progressReceiptWritten:
		return 4
	case progressCompleted:
		return 5
	default:
		return -1
	}
}

func alignJournalToProgress(
	plan migrationEffectPlan,
	journal migrationJournal,
	progress sagaProgress,
	hook effectFailureHook,
) (migrationJournal, error) {
	desired := journalPhaseForProgress(progress)
	phases := []JournalPhase{
		JournalPrepared,
		JournalTargetInstalled,
		JournalSourceArchived,
		JournalLineageWritten,
		JournalReceiptWritten,
		JournalCompleted,
	}
	currentRank := slices.Index(phases, journal.phase)
	desiredRank := slices.Index(phases, desired)
	if currentRank < 0 || desiredRank < 0 || currentRank > desiredRank {
		return migrationJournal{}, fmt.Errorf("journal phase cannot align with observed migration progress")
	}
	var err error
	for _, phase := range phases[currentRank+1 : desiredRank+1] {
		journal, err = advanceJournal(plan, journal, phase, hook)
		if err != nil {
			return migrationJournal{}, err
		}
	}
	return journal, nil
}

func journalPhaseForProgress(progress sagaProgress) JournalPhase {
	switch progress {
	case progressPrepared:
		return JournalPrepared
	case progressTargetInstalled:
		return JournalTargetInstalled
	case progressSourceArchived:
		return JournalSourceArchived
	case progressLineageWritten:
		return JournalLineageWritten
	case progressReceiptWritten:
		return JournalReceiptWritten
	case progressCompleted:
		return JournalCompleted
	default:
		return ""
	}
}

func uniquePaths(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, found := seen[value]; found {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func validateJournalForRequest(journal migrationJournal, request ApplyRequest) error {
	journalMigrationID := journal.migrationID.String()
	requestMigrationID := request.analysis.packetID.String()
	if journalMigrationID != requestMigrationID ||
		!journal.packetDigest.Equal(request.analysis.packetDigest) {
		return fmt.Errorf("journal migration identity does not match the requested packet")
	}
	journalRoot := journal.projectRoot.String()
	requestRoot := request.projectRoot.String()
	if journalRoot != requestRoot {
		return fmt.Errorf("journal project root does not match the current project")
	}
	return nil
}

func validateJournalForPlan(journal migrationJournal, plan migrationEffectPlan) error {
	expected := plan.journal
	journalMigrationID := journal.migrationID.String()
	expectedMigrationID := expected.migrationID.String()
	journalRoot := journal.projectRoot.String()
	expectedRoot := expected.projectRoot.String()
	journalSource := journal.sourceCarrier.String()
	expectedSource := expected.sourceCarrier.String()
	journalTarget := journal.targetCarrier.String()
	expectedTarget := expected.targetCarrier.String()
	journalArchive := journal.archiveCarrier.String()
	expectedArchive := expected.archiveCarrier.String()
	journalLineage := journal.lineageDigest.String()
	expectedLineage := expected.lineageDigest.String()
	journalReview := journal.semanticReviewRef.String()
	expectedReview := expected.semanticReviewRef.String()
	checks := []struct {
		matches bool
		name    string
	}{
		{matches: journalMigrationID == expectedMigrationID, name: "migration ID"},
		{matches: journal.packetDigest.Equal(expected.packetDigest), name: "packet digest"},
		{matches: journalRoot == expectedRoot, name: "project root"},
		{matches: journalSource == expectedSource, name: "source carrier"},
		{matches: journal.sourceDigest.Equal(expected.sourceDigest), name: "source digest"},
		{matches: journalTarget == expectedTarget, name: "target carrier"},
		{matches: journal.targetDigest.Equal(expected.targetDigest), name: "target digest"},
		{matches: journalArchive == expectedArchive, name: "archive carrier"},
		{matches: journalLineage == expectedLineage, name: "lineage digest"},
		{matches: journal.lineageRecordDigest.Equal(expected.lineageRecordDigest), name: "lineage record digest"},
		{matches: journal.profileAdmissionRef == expected.profileAdmissionRef, name: "profile admission ref"},
		{matches: journal.profileAdmissionHash == expected.profileAdmissionHash, name: "profile admission digest"},
		{matches: journal.profileLedgerRevision == expected.profileLedgerRevision, name: "profile ledger revision"},
		{matches: journalReview == expectedReview, name: "semantic review ref"},
		{matches: journal.semanticAdmissionHash.Equal(expected.semanticAdmissionHash), name: "semantic review admission digest"},
		{matches: journal.semanticReviewDigest.Equal(expected.semanticReviewDigest), name: "semantic review digest"},
		{matches: journal.gitWitnessDigest.Equal(expected.gitWitnessDigest), name: "Git witness digest"},
		{matches: journal.receiptDigest.Equal(expected.receiptDigest), name: "receipt digest"},
		{matches: journal.startedAt.Equal(expected.startedAt), name: "start time"},
	}
	for _, check := range checks {
		if !check.matches {
			return fmt.Errorf("journal %s does not match the exact apply plan", check.name)
		}
	}
	return nil
}

func inspectSagaProgress(plan migrationEffectPlan) (sagaProgress, error) {
	targetDigest := plan.journal.targetDigest.String()
	sourceDigest := plan.journal.sourceDigest.String()
	lineageDigest := plan.journal.lineageRecordDigest.String()
	receiptDigest := plan.journal.receiptDigest.String()
	target, targetExact, err := exactFileState(plan.paths.target, targetDigest)
	if err != nil {
		return "", err
	}
	source, sourceExact, err := exactFileState(plan.paths.source, sourceDigest)
	if err != nil {
		return "", err
	}
	archive, archiveExact, err := exactFileState(plan.paths.archive, sourceDigest)
	if err != nil {
		return "", err
	}
	lineage, lineageExact, err := exactFileState(plan.paths.lineage, lineageDigest)
	if err != nil {
		return "", err
	}
	receipt, receiptExact, err := exactFileState(plan.paths.receipt, receiptDigest)
	if err != nil {
		return "", err
	}
	if !target && source && !archive && !lineage && !receipt {
		if !sourceExact {
			return "", fmt.Errorf("source bytes drifted before recovery")
		}
		return progressPrepared, nil
	}
	if target && source && !archive && !lineage && !receipt {
		if !targetExact || !sourceExact {
			return "", fmt.Errorf("target or source bytes drifted before archive recovery")
		}
		return progressTargetInstalled, nil
	}
	if target && !source && archive && !lineage && !receipt {
		if !targetExact || !archiveExact {
			return "", fmt.Errorf("target or archive bytes drifted before lineage recovery")
		}
		return progressSourceArchived, nil
	}
	if target && !source && archive && lineage && !receipt {
		if !targetExact || !archiveExact || !lineageExact {
			return "", fmt.Errorf("migration artifacts drifted before receipt recovery")
		}
		return progressLineageWritten, nil
	}
	if target && !source && archive && lineage && receipt {
		if !targetExact || !archiveExact || !lineageExact || !receiptExact {
			return "", fmt.Errorf("completed migration artifacts have digest drift")
		}
		return progressCompleted, nil
	}
	return "", fmt.Errorf("filesystem state is not a recoverable migration prefix")
}

func verifyCompletedState(plan migrationEffectPlan) (MigrationEffectReceipt, error) {
	progress, err := inspectSagaProgress(plan)
	if err != nil {
		return MigrationEffectReceipt{}, err
	}
	if progress != progressCompleted {
		return MigrationEffectReceipt{}, fmt.Errorf("migration final filesystem state is incomplete")
	}
	journal, found, err := loadJournal(plan.paths.journal)
	if err != nil || !found {
		return MigrationEffectReceipt{}, fmt.Errorf("completed migration journal is missing or invalid: %w", err)
	}
	if journal.phase != JournalCompleted {
		return MigrationEffectReceipt{}, fmt.Errorf("migration journal is not completed")
	}
	if err := validateJournalForPlan(journal, plan); err != nil {
		return MigrationEffectReceipt{}, err
	}
	content, err := readRegularFileNoFollow(plan.paths.receipt)
	if err != nil {
		return MigrationEffectReceipt{}, err
	}
	receipt, err := decodeReceipt(content)
	if err != nil {
		return MigrationEffectReceipt{}, err
	}
	return receipt, nil
}

func exactFileState(path string, expectedDigest string) (bool, bool, error) {
	content, err := readRegularFileNoFollow(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	digest := DigestBytes(content)
	observedDigest := digest.String()
	return true, observedDigest == expectedDigest, nil
}

func recoveryForRequest(
	request ApplyRequest,
	phase JournalPhase,
	err error,
) RecoveryRequired {
	return recoveryRequired{
		migrationID: request.analysis.packetID,
		packet:      request.analysis.packetDigest,
		phase:       phase,
		reason:      err.Error(),
	}
}

func recoveryForPlan(
	plan migrationEffectPlan,
	phase JournalPhase,
	err error,
) RecoveryRequired {
	return recoveryForRequest(plan.request, phase, err)
}
