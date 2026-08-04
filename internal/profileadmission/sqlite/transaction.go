package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/m0n0x41d/haft/internal/profileadmission"
	"github.com/m0n0x41d/haft/internal/profileauthority"
	profileauthoritysqlite "github.com/m0n0x41d/haft/internal/profileauthority/sqlite"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	projectprofilesqlite "github.com/m0n0x41d/haft/internal/projectprofile/sqlite"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

const selectRecordedV2UseByBasisSQL = `WITH recorded AS (
	SELECT committed_admission_ref, committed_admission_digest,
		admission_request_digest, authority_basis_ref
	FROM profile_declaration_authority_uses_v2
	UNION ALL
	SELECT committed_admission_ref, committed_admission_digest,
		admission_request_digest, authority_basis_ref
	FROM profile_declaration_authority_uses_v3
	UNION ALL
	SELECT committed_admission_ref, committed_admission_digest,
		admission_request_digest, authority_basis_ref
	FROM profile_declaration_authority_uses_v4
	UNION ALL
	SELECT committed_admission_ref, committed_admission_digest,
		admission_request_digest, authority_basis_ref
	FROM profile_declaration_authority_uses_v5
)
SELECT committed_admission_ref, committed_admission_digest, admission_request_digest
FROM recorded
WHERE authority_basis_ref = ?`

type recordedV2Use struct {
	admissionRef         string
	admissionDigest      string
	admissionRequestHash string
}

const selectRecordedV3UseByBasisSQL = `SELECT
	committed_admission_ref,
	committed_admission_digest,
	admission_request_digest
FROM profile_declaration_authority_uses_v3
WHERE authority_basis_ref = ?`

type recordedV3Use struct {
	admissionRef         string
	admissionDigest      string
	admissionRequestHash string
}

type admissionHeadExpectation struct {
	revision projectprofile.LedgerRevision
	present  bool
}

func admissionHeadExpectationFromRequest(
	request profileadmission.ProfileDeclarationAdmissionRequest,
) admissionHeadExpectation {
	revision, present := request.ExpectedLedgerRevision()
	return admissionHeadExpectation{
		revision: revision,
		present:  present,
	}
}

func validateAdmissionHeadExpectation(
	expectation admissionHeadExpectation,
	current projectprofile.LedgerRevision,
) error {
	if !expectation.present || expectation.revision == current {
		return nil
	}
	return fmt.Errorf(
		"profile change expected ledger revision %d but current revision is %d",
		expectation.revision.Value(),
		current.Value(),
	)
}

func (adapter adapter) rejectUnexpectedAdmissionHead(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	candidate projectprofile.ProfileDeclarationCandidateV1,
	expectation admissionHeadExpectation,
) (adapterOutcome, bool) {
	if !expectation.present {
		return adapterOutcome{}, false
	}
	ledgerRevision, err := loadExactLedgerHead(
		ctx,
		transaction,
		candidate.Provenance().ProjectRoot(),
	)
	if err != nil {
		return adapter.rollbackFailure(
			transaction,
			failureStageLedgerHeadIntegrity,
		), true
	}
	if err := validateAdmissionHeadExpectation(
		expectation,
		ledgerRevision,
	); err != nil {
		outcome := denied("ledger_revision_conflict", err.Error())
		return adapter.rollbackOutcome(transaction, outcome), true
	}
	return adapterOutcome{}, false
}

const selectRecordedV4UseByBasisSQL = `SELECT
	committed_admission_ref,
	committed_admission_digest,
	admission_request_digest
FROM profile_declaration_authority_uses_v4
WHERE authority_basis_ref = ?`

const selectRecordedV5UseByBasisSQL = `SELECT
	committed_admission_ref,
	committed_admission_digest,
	admission_request_digest
FROM profile_declaration_authority_uses_v5
WHERE authority_basis_ref = ?`

func loadRecordedV5UseByBasis(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	basisRef string,
) (recordedV3Use, bool, error) {
	row := recordedV3Use{}
	err := transaction.ScanOne(
		ctx,
		selectRecordedV5UseByBasisSQL,
		[]any{basisRef},
		[]any{&row.admissionRef, &row.admissionDigest, &row.admissionRequestHash},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return recordedV3Use{}, false, nil
	}
	if err != nil {
		return recordedV3Use{}, false, fmt.Errorf("load recorded v5 authority use: %w", err)
	}
	return row, true, nil
}

func loadRecordedV4UseByBasis(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	basisRef string,
) (recordedV3Use, bool, error) {
	row := recordedV3Use{}
	err := transaction.ScanOne(
		ctx,
		selectRecordedV4UseByBasisSQL,
		[]any{basisRef},
		[]any{&row.admissionRef, &row.admissionDigest, &row.admissionRequestHash},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return recordedV3Use{}, false, nil
	}
	if err != nil {
		return recordedV3Use{}, false, fmt.Errorf("load recorded v4 authority use: %w", err)
	}
	return row, true, nil
}

func loadRecordedV3UseByBasis(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	basisRef string,
) (recordedV3Use, bool, error) {
	row := recordedV3Use{}
	err := transaction.ScanOne(
		ctx,
		selectRecordedV3UseByBasisSQL,
		[]any{basisRef},
		[]any{&row.admissionRef, &row.admissionDigest, &row.admissionRequestHash},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return recordedV3Use{}, false, nil
	}
	if err != nil {
		return recordedV3Use{}, false, fmt.Errorf("load recorded v3 authority use: %w", err)
	}
	return row, true, nil
}

func loadRecordedV2UseByBasis(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	basisRef profileauthority.BasisRef,
) (recordedV2Use, bool, error) {
	row := recordedV2Use{}
	arguments := []any{basisRef.String()}
	destinations := []any{
		&row.admissionRef,
		&row.admissionDigest,
		&row.admissionRequestHash,
	}
	err := transaction.ScanOne(
		ctx,
		selectRecordedV2UseByBasisSQL,
		arguments,
		destinations,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return recordedV2Use{}, false, nil
	}
	if err != nil {
		return recordedV2Use{}, false, fmt.Errorf("load recorded v2 authority use: %w", err)
	}
	return row, true, nil
}

func resolveReplayCandidate(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	candidate projectprofile.ProfileDeclarationCandidateV1,
	recorded profileauthority.AuthorityUseRecord,
) (canonicalAdmissionMaterial, error) {
	committedRef, committedDigest, committedOK := recorded.CommittedAdmission()
	requestDigest, requestOK := recorded.AdmissionRequestDigest()
	if !committedOK || !requestOK {
		return canonicalAdmissionMaterial{}, fmt.Errorf("recorded authority use is incomplete")
	}
	admissionRef, err := projectprofile.NewProfileDeclarationAdmissionRecordRef(
		committedRef.String(),
	)
	if err != nil {
		return canonicalAdmissionMaterial{}, err
	}
	admissionDigest, err := projectprofile.NewContentDigest(committedDigest.String())
	if err != nil {
		return canonicalAdmissionMaterial{}, err
	}
	projectRoot := candidate.Provenance().ProjectRoot()
	material, err := resolveCanonicalByReferenceOnConnection(
		ctx,
		transaction,
		projectRoot,
		admissionRef,
		admissionDigest,
	)
	if err != nil {
		return canonicalAdmissionMaterial{}, err
	}
	incomingPayloadDigest, err := projectprofile.DigestProfileDeclarationPayload(
		candidate.Payload(),
	)
	if err != nil {
		return canonicalAdmissionMaterial{}, err
	}
	incomingProvenanceDigest, err := projectprofile.DigestCandidateProvenanceV1(
		candidate.Provenance(),
	)
	if err != nil {
		return canonicalAdmissionMaterial{}, err
	}
	storedPayloadDigest, err := projectprofile.DigestProfileDeclarationPayload(
		material.candidate.Payload(),
	)
	if err != nil {
		return canonicalAdmissionMaterial{}, err
	}
	checks := []struct {
		matches bool
		name    string
	}{
		{matches: incomingPayloadDigest == storedPayloadDigest, name: "replay payload digest"},
		{matches: incomingProvenanceDigest == material.provenanceDigest, name: "replay provenance digest"},
		{matches: requestDigest.String() != "", name: "recorded admission-request digest"},
	}
	err = firstMismatch(checks, "recorded profile admission replay")
	if err != nil {
		return canonicalAdmissionMaterial{}, err
	}
	return material, nil
}

func resolveReplayCandidateV3(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	candidate projectprofile.ProfileDeclarationCandidateV1,
	recorded recordedV3Use,
) (canonicalAdmissionMaterial, error) {
	admissionRef, err := projectprofile.NewProfileDeclarationAdmissionRecordRef(
		recorded.admissionRef,
	)
	if err != nil {
		return canonicalAdmissionMaterial{}, err
	}
	admissionDigest, err := projectprofile.NewContentDigest(recorded.admissionDigest)
	if err != nil {
		return canonicalAdmissionMaterial{}, err
	}
	material, err := resolveCanonicalByReferenceOnConnection(
		ctx,
		transaction,
		candidate.Provenance().ProjectRoot(),
		admissionRef,
		admissionDigest,
	)
	if err != nil {
		return canonicalAdmissionMaterial{}, err
	}
	incomingPayloadDigest, err := projectprofile.DigestProfileDeclarationPayload(
		candidate.Payload(),
	)
	if err != nil {
		return canonicalAdmissionMaterial{}, err
	}
	incomingProvenanceDigest, err := projectprofile.DigestCandidateProvenanceV1(
		candidate.Provenance(),
	)
	if err != nil {
		return canonicalAdmissionMaterial{}, err
	}
	storedPayloadDigest, err := projectprofile.DigestProfileDeclarationPayload(
		material.candidate.Payload(),
	)
	if err != nil {
		return canonicalAdmissionMaterial{}, err
	}
	checks := []struct {
		matches bool
		name    string
	}{
		{matches: incomingPayloadDigest == storedPayloadDigest, name: "replay payload digest"},
		{matches: incomingProvenanceDigest == material.provenanceDigest, name: "replay provenance digest"},
		{matches: recorded.admissionRequestHash != "", name: "recorded admission-request digest"},
	}
	if err := firstMismatch(checks, "recorded v3 profile admission replay"); err != nil {
		return canonicalAdmissionMaterial{}, err
	}
	return material, nil
}

// Admit performs the whole authority-use and profile-revision effect on one
// dedicated SQLite connection. No Prepared value or transaction handle crosses
// this boundary.
func (adapter adapter) Admit(
	ctx context.Context,
	request profileadmission.ProfileDeclarationAdmissionRequest,
) adapterOutcome {
	if ctx == nil {
		return denied(
			"invalid_request",
			"profile-admission context is required",
		)
	}
	if adapter.starter == nil || adapter.finisher == nil || adapter.database == nil ||
		adapter.authorityGate == nil {
		return writeFailure(
			AdmissionDefinitelyNotCommitted,
			failureStageAdapterContract,
		)
	}
	candidate := request.Candidate()
	expectation := admissionHeadExpectationFromRequest(request)
	_, err := projectprofile.NewProfileDeclarationCandidateV1(
		candidate.Payload(),
		candidate.Provenance(),
	)
	if err != nil {
		return denied(
			"invalid_request",
			fmt.Sprintf("profile declaration candidate is invalid: %v", err),
		)
	}
	provenance := candidate.Provenance()
	authorityBasisRef, err := profileauthority.NewBasisRef(
		provenance.AuthorityBasisRef().String(),
	)
	if err != nil {
		return denied("invalid_authority_basis", err.Error())
	}
	v5Found, err := discoverV5Basis(
		ctx,
		adapter.database,
		authorityBasisRef.String(),
	)
	if err != nil {
		return denied("authority_closure_unavailable", err.Error())
	}
	if v5Found {
		return adapter.admitV5(ctx, candidate, expectation)
	}
	v4Found, err := discoverV4Basis(
		ctx,
		adapter.database,
		authorityBasisRef.String(),
	)
	if err != nil {
		return denied("authority_closure_unavailable", err.Error())
	}
	if v4Found {
		return adapter.admitV4(ctx, candidate, expectation)
	}
	v3Discovery, v3Found, err := discoverV3Basis(
		ctx,
		adapter.database,
		authorityBasisRef.String(),
	)
	if err != nil {
		return denied("authority_closure_unavailable", err.Error())
	}
	if v3Found {
		return adapter.admitV3(
			ctx,
			candidate,
			v3Discovery,
			expectation,
		)
	}
	snapshot, err := adapter.authorityGate.PrepareClosureSnapshotForBasis(
		ctx,
		authorityBasisRef,
	)
	if err != nil {
		return denied("authority_closure_unavailable", err.Error())
	}
	transaction, err := adapter.starter.BeginImmediate(ctx, adapter.database)
	if err != nil {
		return writeFailure(
			AdmissionDefinitelyNotCommitted,
			failureStageBeginImmediate,
		)
	}
	if expectation.present {
		ledgerRevision, headErr := loadExactLedgerHead(
			ctx,
			transaction,
			candidate.Provenance().ProjectRoot(),
		)
		if headErr != nil {
			return adapter.rollbackFailure(
				transaction,
				failureStageLedgerHeadIntegrity,
			)
		}
		if headErr := validateAdmissionHeadExpectation(
			expectation,
			ledgerRevision,
		); headErr != nil {
			outcome := denied("ledger_revision_conflict", headErr.Error())
			return adapter.rollbackOutcome(transaction, outcome)
		}
	}
	replay, replayFound, err := adapter.authorityGate.ResolveAuthorityUseForBasisInTransaction(
		ctx,
		transaction,
		snapshot,
	)
	if err != nil {
		return adapter.rollbackFailure(transaction, failureStageReplayReread)
	}
	if replayFound {
		result, loadErr := resolveReplayCandidate(
			ctx,
			transaction,
			candidate,
			replay,
		)
		if loadErr != nil {
			outcome := denied("single_use_already_consumed", loadErr.Error())
			return adapter.rollbackOutcome(transaction, outcome)
		}
		if loadErr = validateAuthorityUseAgainstMaterial(replay, result); loadErr != nil {
			return adapter.rollbackFailure(transaction, failureStageReplayReread)
		}
		return adapter.commitAndDeliver(
			ctx,
			transaction,
			result,
			CanonicalAdmissionReplayed,
		)
	}
	admittedUse, denials, err := adapter.authorityGate.ResolveForAdmission(
		ctx,
		transaction,
		snapshot,
	)
	if err != nil {
		if errors.Is(err, profileauthoritysqlite.ErrAuthorityResolutionRequired) {
			outcome := denied("authority_resolution_required", err.Error())
			return adapter.rollbackOutcome(transaction, outcome)
		}
		if errors.Is(err, profileauthoritysqlite.ErrAuthorityAlreadyConsumed) {
			return adapter.rollbackFailure(transaction, failureStageAuthorityUseContract)
		}
		return adapter.rollbackFailure(transaction, failureStageAuthorityRead)
	}
	if len(denials) > 0 {
		outcome := profileAuthorityReasonsOutcome(denials)
		return adapter.rollbackOutcome(transaction, outcome)
	}
	valueSnapshot, err := projectprofilesqlite.ResolveProfileAdmissionValueSnapshotV1(
		ctx,
		transaction,
		candidate,
	)
	if err != nil {
		outcome := denied(
			"durable_support_invalid",
			fmt.Sprintf("durable profile-onboarding support is invalid: %v", err),
		)
		return adapter.rollbackOutcome(transaction, outcome)
	}
	values, ok := valueSnapshot.Values()
	if !ok {
		return adapter.rollbackFailure(transaction, failureStageSupportContract)
	}
	authorityValue, err := materializeAuthority(admittedUse)
	if err != nil {
		return adapter.rollbackFailure(transaction, failureStageAuthorityContract)
	}
	projectRoot := provenance.ProjectRoot()
	ledgerRevision, err := loadExactLedgerHead(ctx, transaction, projectRoot)
	if err != nil {
		return adapter.rollbackFailure(transaction, failureStageLedgerHeadIntegrity)
	}
	if err := validateAdmissionHeadExpectation(expectation, ledgerRevision); err != nil {
		outcome := denied("ledger_revision_conflict", err.Error())
		return adapter.rollbackOutcome(transaction, outcome)
	}
	prepared, err := prepareAdmission(
		candidate,
		values,
		authorityValue,
		ledgerRevision,
	)
	if err != nil {
		outcome := denied(
			"candidate_authority_mismatch",
			err.Error(),
		)
		return adapter.rollbackOutcome(transaction, outcome)
	}
	return adapter.writeCommitAndDeliver(
		ctx,
		transaction,
		snapshot,
		prepared,
		authorityValue,
	)
}

func (adapter adapter) admitV5(
	ctx context.Context,
	candidate projectprofile.ProfileDeclarationCandidateV1,
	expectation admissionHeadExpectation,
) adapterOutcome {
	transaction, err := adapter.starter.BeginImmediate(ctx, adapter.database)
	if err != nil {
		return writeFailure(
			AdmissionDefinitelyNotCommitted,
			failureStageBeginImmediate,
		)
	}
	if outcome, rejected := adapter.rejectUnexpectedAdmissionHead(
		ctx,
		transaction,
		candidate,
		expectation,
	); rejected {
		return outcome
	}
	basisRef := candidate.Provenance().AuthorityBasisRef().String()
	recorded, found, err := loadRecordedV5UseByBasis(ctx, transaction, basisRef)
	if err != nil {
		return adapter.rollbackFailure(transaction, failureStageReplayReread)
	}
	if found {
		material, loadErr := resolveReplayCandidateV3(
			ctx,
			transaction,
			candidate,
			recorded,
		)
		if loadErr != nil {
			outcome := denied("single_use_already_consumed", loadErr.Error())
			return adapter.rollbackOutcome(transaction, outcome)
		}
		return adapter.commitAndDeliver(
			ctx,
			transaction,
			material,
			CanonicalAdmissionReplayed,
		)
	}
	closure, err := loadV5AuthorityClosure(ctx, transaction, basisRef)
	if err != nil {
		outcome := denied("authority_closure_unavailable", err.Error())
		return adapter.rollbackOutcome(transaction, outcome)
	}
	valueSnapshot, err := projectprofilesqlite.ResolveProfileAdmissionValueSnapshotV1(
		ctx,
		transaction,
		candidate,
	)
	if err != nil {
		outcome := denied(
			"durable_support_invalid",
			fmt.Sprintf("durable profile-onboarding support is invalid: %v", err),
		)
		return adapter.rollbackOutcome(transaction, outcome)
	}
	values, ok := valueSnapshot.Values()
	if !ok {
		return adapter.rollbackFailure(transaction, failureStageSupportContract)
	}
	authorityValue, err := materializeV5Authority(
		closure,
		values,
		adapter.now(),
	)
	if err != nil {
		outcome := denied("candidate_authority_mismatch", err.Error())
		return adapter.rollbackOutcome(transaction, outcome)
	}
	projectRoot := candidate.Provenance().ProjectRoot()
	ledgerRevision, err := loadExactLedgerHead(ctx, transaction, projectRoot)
	if err != nil {
		return adapter.rollbackFailure(transaction, failureStageLedgerHeadIntegrity)
	}
	if err := validateAdmissionHeadExpectation(expectation, ledgerRevision); err != nil {
		outcome := denied("ledger_revision_conflict", err.Error())
		return adapter.rollbackOutcome(transaction, outcome)
	}
	prepared, err := prepareAdmission(
		candidate,
		values,
		authorityValue,
		ledgerRevision,
	)
	if err != nil {
		outcome := denied("candidate_authority_mismatch", err.Error())
		return adapter.rollbackOutcome(transaction, outcome)
	}
	return adapter.writeCommitAndDeliverV5(
		ctx,
		transaction,
		prepared,
		authorityValue,
	)
}

func (adapter adapter) admitV4(
	ctx context.Context,
	candidate projectprofile.ProfileDeclarationCandidateV1,
	expectation admissionHeadExpectation,
) adapterOutcome {
	transaction, err := adapter.starter.BeginImmediate(ctx, adapter.database)
	if err != nil {
		return writeFailure(
			AdmissionDefinitelyNotCommitted,
			failureStageBeginImmediate,
		)
	}
	if outcome, rejected := adapter.rejectUnexpectedAdmissionHead(
		ctx,
		transaction,
		candidate,
		expectation,
	); rejected {
		return outcome
	}
	basisRef := candidate.Provenance().AuthorityBasisRef().String()
	recorded, found, err := loadRecordedV4UseByBasis(ctx, transaction, basisRef)
	if err != nil {
		return adapter.rollbackFailure(transaction, failureStageReplayReread)
	}
	if found {
		material, loadErr := resolveReplayCandidateV3(
			ctx,
			transaction,
			candidate,
			recorded,
		)
		if loadErr != nil {
			outcome := denied("single_use_already_consumed", loadErr.Error())
			return adapter.rollbackOutcome(transaction, outcome)
		}
		return adapter.commitAndDeliver(
			ctx,
			transaction,
			material,
			CanonicalAdmissionReplayed,
		)
	}
	closure, err := loadV4AuthorityClosure(ctx, transaction, basisRef)
	if err != nil {
		outcome := denied("authority_closure_unavailable", err.Error())
		return adapter.rollbackOutcome(transaction, outcome)
	}
	valueSnapshot, err := projectprofilesqlite.ResolveProfileAdmissionValueSnapshotV1(
		ctx,
		transaction,
		candidate,
	)
	if err != nil {
		outcome := denied(
			"durable_support_invalid",
			fmt.Sprintf("durable profile-onboarding support is invalid: %v", err),
		)
		return adapter.rollbackOutcome(transaction, outcome)
	}
	values, ok := valueSnapshot.Values()
	if !ok {
		return adapter.rollbackFailure(transaction, failureStageSupportContract)
	}
	authorityValue, err := materializeV4Authority(
		closure,
		values,
		adapter.now(),
	)
	if err != nil {
		outcome := denied("candidate_authority_mismatch", err.Error())
		return adapter.rollbackOutcome(transaction, outcome)
	}
	projectRoot := candidate.Provenance().ProjectRoot()
	ledgerRevision, err := loadExactLedgerHead(ctx, transaction, projectRoot)
	if err != nil {
		return adapter.rollbackFailure(transaction, failureStageLedgerHeadIntegrity)
	}
	if err := validateAdmissionHeadExpectation(expectation, ledgerRevision); err != nil {
		outcome := denied("ledger_revision_conflict", err.Error())
		return adapter.rollbackOutcome(transaction, outcome)
	}
	prepared, err := prepareAdmission(
		candidate,
		values,
		authorityValue,
		ledgerRevision,
	)
	if err != nil {
		outcome := denied("candidate_authority_mismatch", err.Error())
		return adapter.rollbackOutcome(transaction, outcome)
	}
	return adapter.writeCommitAndDeliverV4(
		ctx,
		transaction,
		prepared,
		authorityValue,
	)
}

func (adapter adapter) admitV3(
	ctx context.Context,
	candidate projectprofile.ProfileDeclarationCandidateV1,
	discovery v3BasisDiscovery,
	expectation admissionHeadExpectation,
) adapterOutcome {
	strictSnapshot := profileauthoritysqlite.ClosureSnapshot{}
	if discovery.strict() {
		strictBasisRef, err := profileauthority.NewBasisRef(discovery.strictBasisRef)
		if err != nil {
			return denied("invalid_strict_authority_basis", err.Error())
		}
		strictSnapshot, err = adapter.authorityGate.PrepareClosureSnapshotForBasis(
			ctx,
			strictBasisRef,
		)
		if err != nil {
			return denied("strict_authority_closure_unavailable", err.Error())
		}
	}
	transaction, err := adapter.starter.BeginImmediate(ctx, adapter.database)
	if err != nil {
		return writeFailure(
			AdmissionDefinitelyNotCommitted,
			failureStageBeginImmediate,
		)
	}
	if outcome, rejected := adapter.rejectUnexpectedAdmissionHead(
		ctx,
		transaction,
		candidate,
		expectation,
	); rejected {
		return outcome
	}
	basisRef := candidate.Provenance().AuthorityBasisRef().String()
	recorded, found, err := loadRecordedV3UseByBasis(ctx, transaction, basisRef)
	if err != nil {
		return adapter.rollbackFailure(transaction, failureStageReplayReread)
	}
	if found {
		material, loadErr := resolveReplayCandidateV3(
			ctx,
			transaction,
			candidate,
			recorded,
		)
		if loadErr != nil {
			outcome := denied("single_use_already_consumed", loadErr.Error())
			return adapter.rollbackOutcome(transaction, outcome)
		}
		return adapter.commitAndDeliver(
			ctx,
			transaction,
			material,
			CanonicalAdmissionReplayed,
		)
	}
	closure, err := loadV3AuthorityClosure(ctx, transaction, basisRef)
	if err != nil {
		if errors.Is(err, profileauthoritysqlite.ErrAuthorityResolutionRequired) {
			outcome := denied("authority_resolution_required", err.Error())
			return adapter.rollbackOutcome(transaction, outcome)
		}
		outcome := denied("authority_closure_unavailable", err.Error())
		return adapter.rollbackOutcome(transaction, outcome)
	}
	valueSnapshot, err := projectprofilesqlite.ResolveProfileAdmissionValueSnapshotV1(
		ctx,
		transaction,
		candidate,
	)
	if err != nil {
		outcome := denied(
			"durable_support_invalid",
			fmt.Sprintf("durable profile-onboarding support is invalid: %v", err),
		)
		return adapter.rollbackOutcome(transaction, outcome)
	}
	values, ok := valueSnapshot.Values()
	if !ok {
		return adapter.rollbackFailure(transaction, failureStageSupportContract)
	}
	strictUse := profileauthority.AdmittedUse{}
	if discovery.strict() {
		resolved, denials, resolveErr := adapter.authorityGate.ResolveForAdmission(
			ctx,
			transaction,
			strictSnapshot,
		)
		if resolveErr != nil {
			return adapter.rollbackFailure(transaction, failureStageAuthorityRead)
		}
		if len(denials) > 0 {
			return adapter.rollbackOutcome(
				transaction,
				profileAuthorityReasonsOutcome(denials),
			)
		}
		strictUse = resolved
	}
	authorityValue, err := materializeV3Authority(
		closure,
		values,
		strictUse,
		adapter.now(),
	)
	if err != nil {
		outcome := denied("candidate_authority_mismatch", err.Error())
		return adapter.rollbackOutcome(transaction, outcome)
	}
	projectRoot := candidate.Provenance().ProjectRoot()
	ledgerRevision, err := loadExactLedgerHead(ctx, transaction, projectRoot)
	if err != nil {
		return adapter.rollbackFailure(transaction, failureStageLedgerHeadIntegrity)
	}
	if err := validateAdmissionHeadExpectation(expectation, ledgerRevision); err != nil {
		outcome := denied("ledger_revision_conflict", err.Error())
		return adapter.rollbackOutcome(transaction, outcome)
	}
	prepared, err := prepareAdmission(
		candidate,
		values,
		authorityValue,
		ledgerRevision,
	)
	if err != nil {
		outcome := denied("candidate_authority_mismatch", err.Error())
		return adapter.rollbackOutcome(transaction, outcome)
	}
	return adapter.writeCommitAndDeliverV3(
		ctx,
		transaction,
		prepared,
		authorityValue,
	)
}

func (adapter adapter) writeCommitAndDeliver(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	snapshot profileauthoritysqlite.ClosureSnapshot,
	prepared projectprofile.PreparedProfileAdmissionV1,
	authorityValue authorityMaterial,
) adapterOutcome {
	material, err := newWriteMaterial(prepared, authorityValue)
	if err != nil {
		outcome := denied(
			"admission_material_invalid",
			err.Error(),
		)
		return adapter.rollbackOutcome(transaction, outcome)
	}
	err = persistAdmission(ctx, transaction, material)
	if err != nil {
		return adapter.rollbackFailure(transaction, failureStageAdmissionWrite)
	}
	err = persistAuthorityUse(ctx, transaction, material)
	if err != nil {
		return adapter.rollbackFailure(transaction, failureStageAuthorityUseWrite)
	}
	durableUse, found, err := adapter.authorityGate.ResolveAuthorityUseForBasisInTransaction(
		ctx,
		transaction,
		snapshot,
	)
	if err != nil || !found || !sameAuthorityUseRecord(durableUse, material.authorityUse) {
		return adapter.rollbackFailure(transaction, failureStageAuthorityUseWrite)
	}
	err = persistRevision(ctx, transaction, material)
	if err != nil {
		return adapter.rollbackFailure(transaction, failureStageRevisionWrite)
	}
	tentativeDigest := material.tentative.TentativeAdmissionRecordDigest()
	precommit, err := resolveCanonicalByReferenceOnConnection(
		ctx,
		transaction,
		prepared.ProjectRoot(),
		material.admissionRef,
		tentativeDigest,
	)
	if err != nil {
		return adapter.rollbackFailure(transaction, failureStagePrecommitReread)
	}
	return adapter.commitAndDeliver(
		ctx,
		transaction,
		precommit,
		CanonicalAdmissionFresh,
	)
}

func (adapter adapter) writeCommitAndDeliverV3(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	prepared projectprofile.PreparedProfileAdmissionV1,
	authorityValue authorityMaterial,
) adapterOutcome {
	material, err := newWriteMaterialV3(prepared, authorityValue)
	if err != nil {
		outcome := denied("admission_material_invalid", err.Error())
		return adapter.rollbackOutcome(transaction, outcome)
	}
	if err := persistAdmissionV3(ctx, transaction, material); err != nil {
		return adapter.rollbackFailure(transaction, failureStageAdmissionWrite)
	}
	if err := persistAuthorityUseV3(ctx, transaction, material); err != nil {
		return adapter.rollbackFailure(transaction, failureStageAuthorityUseWrite)
	}
	recorded, found, err := loadRecordedV3UseByBasis(
		ctx,
		transaction,
		authorityValue.authorityBasisRef.String(),
	)
	if err != nil || !found ||
		recorded.admissionRef != material.admissionRef.String() ||
		recorded.admissionDigest != material.tentative.TentativeAdmissionRecordDigest().String() ||
		recorded.admissionRequestHash != prepared.AdmissionRequestDigest().String() {
		return adapter.rollbackFailure(transaction, failureStageAuthorityUseWrite)
	}
	if err := persistRevisionV3(ctx, transaction, material); err != nil {
		return adapter.rollbackFailure(transaction, failureStageRevisionWrite)
	}
	tentativeDigest := material.tentative.TentativeAdmissionRecordDigest()
	precommit, err := resolveCanonicalByReferenceOnConnection(
		ctx,
		transaction,
		prepared.ProjectRoot(),
		material.admissionRef,
		tentativeDigest,
	)
	if err != nil {
		return adapter.rollbackFailure(transaction, failureStagePrecommitReread)
	}
	return adapter.commitAndDeliver(
		ctx,
		transaction,
		precommit,
		CanonicalAdmissionFresh,
	)
}

func (adapter adapter) writeCommitAndDeliverV4(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	prepared projectprofile.PreparedProfileAdmissionV1,
	authorityValue authorityMaterial,
) adapterOutcome {
	material, err := newWriteMaterialV4(prepared, authorityValue)
	if err != nil {
		outcome := denied("admission_material_invalid", err.Error())
		return adapter.rollbackOutcome(transaction, outcome)
	}
	if err := persistAdmissionV4(ctx, transaction, material); err != nil {
		return adapter.rollbackFailure(transaction, failureStageAdmissionWrite)
	}
	if err := persistAuthorityUseV4(ctx, transaction, material); err != nil {
		return adapter.rollbackFailure(transaction, failureStageAuthorityUseWrite)
	}
	recorded, found, err := loadRecordedV4UseByBasis(
		ctx,
		transaction,
		authorityValue.authorityBasisRef.String(),
	)
	if err != nil || !found ||
		recorded.admissionRef != material.admissionRef.String() ||
		recorded.admissionDigest != material.tentative.TentativeAdmissionRecordDigest().String() ||
		recorded.admissionRequestHash != prepared.AdmissionRequestDigest().String() {
		return adapter.rollbackFailure(transaction, failureStageAuthorityUseWrite)
	}
	if err := persistRevisionV4(ctx, transaction, material); err != nil {
		return adapter.rollbackFailure(transaction, failureStageRevisionWrite)
	}
	tentativeDigest := material.tentative.TentativeAdmissionRecordDigest()
	precommit, err := resolveCanonicalByReferenceOnConnection(
		ctx,
		transaction,
		prepared.ProjectRoot(),
		material.admissionRef,
		tentativeDigest,
	)
	if err != nil {
		return adapter.rollbackFailure(transaction, failureStagePrecommitReread)
	}
	return adapter.commitAndDeliver(
		ctx,
		transaction,
		precommit,
		CanonicalAdmissionFresh,
	)
}

func (adapter adapter) writeCommitAndDeliverV5(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	prepared projectprofile.PreparedProfileAdmissionV1,
	authorityValue authorityMaterial,
) adapterOutcome {
	material, err := newWriteMaterialV5(prepared, authorityValue)
	if err != nil {
		outcome := denied("admission_material_invalid", err.Error())
		return adapter.rollbackOutcome(transaction, outcome)
	}
	if err := persistAdmissionV5(ctx, transaction, material); err != nil {
		return adapter.rollbackFailure(transaction, failureStageAdmissionWrite)
	}
	if err := persistAuthorityUseV5(ctx, transaction, material); err != nil {
		return adapter.rollbackFailure(transaction, failureStageAuthorityUseWrite)
	}
	recorded, found, err := loadRecordedV5UseByBasis(
		ctx,
		transaction,
		authorityValue.authorityBasisRef.String(),
	)
	if err != nil || !found ||
		recorded.admissionRef != material.admissionRef.String() ||
		recorded.admissionDigest != material.tentative.TentativeAdmissionRecordDigest().String() ||
		recorded.admissionRequestHash != prepared.AdmissionRequestDigest().String() {
		return adapter.rollbackFailure(transaction, failureStageAuthorityUseWrite)
	}
	if err := persistRevisionV5(ctx, transaction, material); err != nil {
		return adapter.rollbackFailure(transaction, failureStageRevisionWrite)
	}
	tentativeDigest := material.tentative.TentativeAdmissionRecordDigest()
	precommit, err := resolveCanonicalByReferenceOnConnection(
		ctx,
		transaction,
		prepared.ProjectRoot(),
		material.admissionRef,
		tentativeDigest,
	)
	if err != nil {
		return adapter.rollbackFailure(transaction, failureStagePrecommitReread)
	}
	return adapter.commitAndDeliver(
		ctx,
		transaction,
		precommit,
		CanonicalAdmissionFresh,
	)
}

func sameAuthorityUseRecord(
	left profileauthority.AuthorityUseRecord,
	right profileauthority.AuthorityUseRecord,
) bool {
	leftDigest, leftDigestOK := left.Digest()
	rightDigest, rightDigestOK := right.Digest()
	leftCanonical, leftCanonicalOK := left.CanonicalBytes()
	rightCanonical, rightCanonicalOK := right.CanonicalBytes()
	return leftDigestOK && rightDigestOK && leftCanonicalOK && rightCanonicalOK &&
		leftDigest.String() == rightDigest.String() &&
		string(leftCanonical) == string(rightCanonical)
}

func (adapter adapter) commitAndDeliver(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	precommit canonicalAdmissionMaterial,
	successPosture CanonicalAdmissionDelivery,
) adapterOutcome {
	if ctx.Err() != nil {
		return adapter.rollbackFailure(transaction, failureStageContextBeforeCommit)
	}
	finish := adapter.finisher.Commit(ctx, transaction)
	commitErr := finish.statementErr
	closeErr := finish.closeErr
	readCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	postcommit, rereadErr := adapter.readCommitted(
		readCtx,
		precommit.projectRoot,
		precommit.admissionRef.String(),
		precommit.admissionDigest.String(),
	)
	if commitErr != nil {
		if rereadErr == nil {
			return admitted(postcommit, CanonicalAdmissionRecovered)
		}
		posture := failedCommitPosture(finish)
		return writeFailure(
			posture,
			failureStageAmbiguousCommit,
		)
	}
	if finish.cleanupErr != nil {
		if rereadErr == nil {
			return admitted(postcommit, successPosture)
		}
		return writeFailure(
			AdmissionCommitOutcomeUnknown,
			failureStageCommitCleanup,
		)
	}
	if closeErr != nil {
		if rereadErr == nil {
			return admitted(postcommit, successPosture)
		}
		return writeFailure(
			AdmissionCommitOutcomeUnknown,
			failureStagePostcommitClose,
		)
	}
	if rereadErr != nil {
		return writeFailure(
			AdmissionCommitOutcomeUnknown,
			failureStagePostcommitReread,
		)
	}
	return admitted(postcommit, successPosture)
}

func (adapter adapter) readCommitted(
	ctx context.Context,
	projectRoot projectprofile.ProjectRootV1,
	admissionRef string,
	expectedAdmissionDigest string,
) (canonicalAdmissionMaterial, error) {
	transaction, err := adapter.starter.BeginRead(ctx, adapter.database)
	if err != nil {
		return canonicalAdmissionMaterial{}, fmt.Errorf("begin post-COMMIT reread: %w", err)
	}
	ref, parseErr := projectprofile.NewProfileDeclarationAdmissionRecordRef(admissionRef)
	if parseErr != nil {
		return canonicalAdmissionMaterial{}, parseErr
	}
	digest, parseErr := projectprofile.NewContentDigest(expectedAdmissionDigest)
	if parseErr != nil {
		return canonicalAdmissionMaterial{}, parseErr
	}
	result, readErr := resolveCanonicalByReferenceOnConnection(
		ctx,
		transaction,
		projectRoot,
		ref,
		digest,
	)
	finish := adapter.rollbackTransaction(transaction)
	rollbackErr := finishError(finish)
	if readErr != nil {
		return canonicalAdmissionMaterial{}, readErr
	}
	if rollbackErr != nil {
		return canonicalAdmissionMaterial{}, fmt.Errorf("finish post-COMMIT reread: %w", rollbackErr)
	}
	if err := adapter.validateHistoricalAuthorityMaterial(ctx, result); err != nil {
		return canonicalAdmissionMaterial{}, err
	}
	return result, nil
}

func (adapter adapter) rollbackOutcome(
	transaction *sqlitetransaction.Transaction,
	outcome adapterOutcome,
) adapterOutcome {
	finish := adapter.rollbackTransaction(transaction)
	if !rollbackProvesNotCommitted(finish) {
		return writeFailure(
			AdmissionDefinitelyNotCommitted,
			failureStageRollback,
		)
	}
	return outcome
}

func (adapter adapter) rollbackFailure(
	transaction *sqlitetransaction.Transaction,
	stage effectFailureStage,
) adapterOutcome {
	finish := adapter.rollbackTransaction(transaction)
	if !rollbackProvesNotCommitted(finish) {
		return writeFailure(
			AdmissionDefinitelyNotCommitted,
			rollbackStage(stage),
		)
	}
	return writeFailure(AdmissionDefinitelyNotCommitted, stage)
}

func (adapter adapter) rollbackTransaction(
	transaction *sqlitetransaction.Transaction,
) transactionFinishEvidence {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return adapter.finisher.Rollback(ctx, transaction)
}

func finishError(evidence transactionFinishEvidence) error {
	return errors.Join(
		evidence.statementErr,
		evidence.cleanupErr,
		evidence.closeErr,
	)
}

func failedCommitPosture(
	evidence transactionFinishEvidence,
) AdmissionCommitPosture {
	if evidence.cleanupErr == nil {
		return AdmissionDefinitelyNotCommitted
	}
	return AdmissionCommitOutcomeUnknown
}

func rollbackProvesNotCommitted(evidence transactionFinishEvidence) bool {
	statementSucceeded := evidence.statementErr == nil
	cleanupSucceeded := evidence.statementErr != nil && evidence.cleanupErr == nil
	return statementSucceeded || cleanupSucceeded
}

func profileAuthorityReasonsOutcome(
	reasons []profileauthority.ResolutionDenial,
) adapterOutcome {
	values, err := convertProfileAuthorityReasons(
		reasons,
		0,
		[]AdmissionDenial{},
	)
	if err != nil {
		return writeFailure(
			AdmissionDefinitelyNotCommitted,
			failureStageAuthorityDenialContract,
		)
	}
	if len(values) == 0 {
		return writeFailure(
			AdmissionDefinitelyNotCommitted,
			failureStageAuthorityDenialContract,
		)
	}
	return newAdapterDenied(values)
}

func convertProfileAuthorityReasons(
	reasons []profileauthority.ResolutionDenial,
	index int,
	values []AdmissionDenial,
) ([]AdmissionDenial, error) {
	if index >= len(reasons) {
		return values, nil
	}
	reason := reasons[index]
	code := string(reason.Code())
	detail := reason.Detail()
	if code == "" || detail == "" {
		return nil, fmt.Errorf("authority denial is incomplete")
	}
	values = append(values, AdmissionDenial{code: code, detail: detail})
	return convertProfileAuthorityReasons(reasons, index+1, values)
}
