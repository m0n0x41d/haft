package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/profileauthority"
	profileauthoritysqlite "github.com/m0n0x41d/haft/internal/profileauthority/sqlite"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	projectprofilesqlite "github.com/m0n0x41d/haft/internal/projectprofile/sqlite"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

var errNoCurrentCanonicalAdmission = errors.New("no current canonical profile admission")
var errNoCommittedAuthorityBasis = errors.New("no committed admission for authority basis")
var errAmbiguousCommittedPayload = errors.New("multiple committed admissions match the payload")

const selectCommittedV2ByPayloadSQL = `WITH committed AS (
	SELECT admission.admission_id, admission.admission_digest,
		admission.project_root, admission.profile_payload_digest
	FROM project_profile_admissions_v2 admission
	JOIN profile_declaration_authority_uses_v2 authority_use
		ON authority_use.committed_admission_ref = admission.admission_id
		AND authority_use.committed_admission_digest = admission.admission_digest
	JOIN project_profile_revisions_v2 revision
		ON revision.admission_id = admission.admission_id
		AND revision.admission_digest = admission.admission_digest
	UNION ALL
	SELECT admission.admission_id, admission.admission_digest,
		admission.project_root, admission.profile_payload_digest
	FROM project_profile_admissions_v3 admission
	JOIN profile_declaration_authority_uses_v3 authority_use
		ON authority_use.committed_admission_ref = admission.admission_id
		AND authority_use.committed_admission_digest = admission.admission_digest
	JOIN project_profile_revisions_v3 revision
		ON revision.admission_id = admission.admission_id
		AND revision.admission_digest = admission.admission_digest
	UNION ALL
	SELECT admission.admission_id, admission.admission_digest,
		admission.project_root, admission.profile_payload_digest
	FROM project_profile_admissions_v4 admission
	JOIN profile_declaration_authority_uses_v4 authority_use
		ON authority_use.committed_admission_ref = admission.admission_id
		AND authority_use.committed_admission_digest = admission.admission_digest
	JOIN project_profile_revisions_v4 revision
		ON revision.admission_id = admission.admission_id
		AND revision.admission_digest = admission.admission_digest
	UNION ALL
	SELECT admission.admission_id, admission.admission_digest,
		admission.project_root, admission.profile_payload_digest
	FROM project_profile_admissions_v5 admission
	JOIN profile_declaration_authority_uses_v5 authority_use
		ON authority_use.committed_admission_ref = admission.admission_id
		AND authority_use.committed_admission_digest = admission.admission_digest
	JOIN project_profile_revisions_v5 revision
		ON revision.admission_id = admission.admission_id
		AND revision.admission_digest = admission.admission_digest
)
SELECT
	COUNT(*),
	COALESCE(SUM(CASE WHEN current.admission_id IS NOT NULL THEN 1 ELSE 0 END), 0),
	COALESCE(MAX(CASE WHEN current.admission_id IS NOT NULL THEN committed.admission_id END), ''),
	COALESCE(MAX(CASE WHEN current.admission_id IS NOT NULL THEN committed.admission_digest END), ''),
	COALESCE(MIN(committed.admission_id), ''),
	COALESCE(MIN(committed.admission_digest), '')
FROM committed
LEFT JOIN current_project_profiles current
	ON current.admission_id = committed.admission_id
	AND current.admission_digest = committed.admission_digest
WHERE committed.project_root = ?
	AND committed.profile_payload_digest = ?`

type committedPayloadEvidence struct {
	matchCount    int64
	currentCount  int64
	currentRef    string
	currentDigest string
	firstRef      string
	firstDigest   string
}

const selectCurrentAdmissionIdentitySQL = `SELECT
	admission_id,
	admission_digest
FROM current_project_profiles
WHERE project_root = ?
	AND ledger_revision = ?
	AND configured_profile_kind = 'Declared'`

type canonicalAdmissionMaterial struct {
	storageGeneration                 string
	origin                            projectprofile.ProfileAdmissionOrigin
	projectRoot                       projectprofile.ProjectRootV1
	candidate                         projectprofile.ProfileDeclarationCandidateV1
	payloadJSON                       []byte
	provenanceJSON                    []byte
	provenanceDigest                  projectprofile.ContentDigest
	admissionRef                      projectprofile.ProfileDeclarationAdmissionRecordRef
	admissionDigest                   projectprofile.ContentDigest
	admissionJSON                     []byte
	receiptJSON                       []byte
	receiptDigest                     projectprofile.ContentDigest
	workRecordRef                     projectprofile.ProfileOnboardingWorkRecordRef
	workRecordDigest                  projectprofile.ContentDigest
	authorityBasisRef                 projectprofile.ProfileDeclarationAuthorityBasisRef
	authorityBasisDigest              projectprofile.ContentDigest
	authorityResolutionRef            projectprofile.AuthorityResolutionRecordRef
	authorityResolutionDigest         projectprofile.ContentDigest
	authorityUseRef                   string
	authorityUseDigest                projectprofile.ContentDigest
	profileAuthorRoleAssignmentRef    projectprofile.RoleAssignmentRef
	profileAuthorRoleAssignmentDigest projectprofile.ContentDigest
	observedProjectBasisRef           projectprofile.ObservedProjectBasisRefV1
	observedProjectBasisDigest        projectprofile.ContentDigest
	outcomeAssessmentRef              projectprofile.ProfileOnboardingOutcomeAssessmentRefV1
	outcomeAssessmentDigest           projectprofile.ContentDigest
	expectedLedgerRevision            projectprofile.LedgerRevision
	ledgerRevision                    projectprofile.LedgerRevision
	recordedAt                        time.Time
}

type currentAdmissionIdentity struct {
	ref    projectprofile.ProfileDeclarationAdmissionRecordRef
	digest projectprofile.ContentDigest
}

type candidateCanonicalEnvelope struct {
	Schema     string          `json:"schema"`
	Payload    json.RawMessage `json:"payload"`
	Provenance json.RawMessage `json:"provenance"`
}

type admissionRecordIdentityProjection struct {
	Schema                          string                    `json:"schema"`
	AdmissionRecordRef              string                    `json:"admission_record_ref"`
	ClassificationWorkRecordRef     string                    `json:"classification_work_record_ref"`
	AuthorityBasisRef               string                    `json:"authority_basis_ref"`
	AuthorityResolutionRecordRef    string                    `json:"authority_resolution_record_ref"`
	AuthorityResolutionRecordDigest string                    `json:"authority_resolution_record_digest"`
	Receipt                         receiptIdentityProjection `json:"receipt"`
	ExpectedLedgerRevision          uint64                    `json:"expected_ledger_revision"`
	CommittedLedgerRevision         uint64                    `json:"committed_ledger_revision"`
	CommittedAt                     string                    `json:"committed_at"`
}

type receiptIdentityProjection struct {
	Schema                          string `json:"schema"`
	AuthorityResolutionRecordRef    string `json:"authority_resolution_record_ref"`
	AuthorityResolutionRecordDigest string `json:"authority_resolution_record_digest"`
	AuthorityBasisRef               string `json:"authority_basis_ref"`
	WorkRecordRef                   string `json:"work_record_ref"`
	CandidateProvenanceDigest       string `json:"candidate_provenance_digest"`
	PayloadDigest                   string `json:"payload_digest"`
	ObservedBasisDigest             string `json:"observed_basis_digest"`
	LedgerRevision                  uint64 `json:"ledger_revision"`
	RecordedAt                      string `json:"recorded_at"`
}

func (adapter adapter) resolveCurrentCanonical(
	ctx context.Context,
	projectRoot projectprofile.ProjectRootV1,
) (canonicalAdmissionMaterial, error) {
	transaction, err := adapter.starter.BeginRead(ctx, adapter.database)
	if err != nil {
		return canonicalAdmissionMaterial{}, fmt.Errorf("begin current profile-admission resolution: %w", err)
	}
	material, readErr := resolveCurrentCanonicalOnConnection(
		ctx,
		transaction,
		projectRoot,
	)
	finish := adapter.rollbackTransaction(transaction)
	finishErr := finishError(finish)
	if readErr != nil {
		return canonicalAdmissionMaterial{}, readErr
	}
	if finishErr != nil {
		return canonicalAdmissionMaterial{}, fmt.Errorf("finish current profile-admission resolution: %w", finishErr)
	}
	if err := adapter.validateHistoricalAuthorityMaterial(ctx, material); err != nil {
		return canonicalAdmissionMaterial{}, err
	}
	return material, nil
}

func (adapter adapter) resolveCanonicalByReference(
	ctx context.Context,
	projectRoot projectprofile.ProjectRootV1,
	admissionRef projectprofile.ProfileDeclarationAdmissionRecordRef,
	expectedDigest projectprofile.ContentDigest,
) (canonicalAdmissionMaterial, error) {
	transaction, err := adapter.starter.BeginRead(ctx, adapter.database)
	if err != nil {
		return canonicalAdmissionMaterial{}, fmt.Errorf("begin exact profile-admission resolution: %w", err)
	}
	material, readErr := resolveCanonicalByReferenceOnConnection(
		ctx,
		transaction,
		projectRoot,
		admissionRef,
		expectedDigest,
	)
	finish := adapter.rollbackTransaction(transaction)
	finishErr := finishError(finish)
	if readErr != nil {
		return canonicalAdmissionMaterial{}, readErr
	}
	if finishErr != nil {
		return canonicalAdmissionMaterial{}, fmt.Errorf("finish exact profile-admission resolution: %w", finishErr)
	}
	if err := adapter.validateHistoricalAuthorityMaterial(ctx, material); err != nil {
		return canonicalAdmissionMaterial{}, err
	}
	return material, nil
}

func (adapter adapter) resolveCommittedForAuthorityBasis(
	ctx context.Context,
	projectRoot projectprofile.ProjectRootV1,
	basisRef projectprofile.ProfileDeclarationAuthorityBasisRef,
) (canonicalAdmissionMaterial, error) {
	transaction, err := adapter.starter.BeginRead(ctx, adapter.database)
	if err != nil {
		return canonicalAdmissionMaterial{}, fmt.Errorf("begin authority-basis admission resolution: %w", err)
	}
	profileBasisRef, parseErr := profileauthority.NewBasisRef(basisRef.String())
	if parseErr != nil {
		adapter.rollbackTransaction(transaction)
		return canonicalAdmissionMaterial{}, parseErr
	}
	recorded, found, readErr := loadRecordedV2UseByBasis(
		ctx,
		transaction,
		profileBasisRef,
	)
	if readErr == nil && !found {
		readErr = errNoCommittedAuthorityBasis
	}
	material := canonicalAdmissionMaterial{}
	if readErr == nil {
		admissionRef, refErr := projectprofile.NewProfileDeclarationAdmissionRecordRef(
			recorded.admissionRef,
		)
		admissionDigest, digestErr := projectprofile.NewContentDigest(
			recorded.admissionDigest,
		)
		readErr = errors.Join(refErr, digestErr)
		if readErr == nil {
			material, readErr = resolveCanonicalByReferenceOnConnection(
				ctx,
				transaction,
				projectRoot,
				admissionRef,
				admissionDigest,
			)
		}
	}
	finish := adapter.rollbackTransaction(transaction)
	finishErr := finishError(finish)
	if readErr != nil {
		return canonicalAdmissionMaterial{}, readErr
	}
	if finishErr != nil {
		return canonicalAdmissionMaterial{}, fmt.Errorf("finish authority-basis admission resolution: %w", finishErr)
	}
	if err := adapter.validateHistoricalAuthorityMaterial(ctx, material); err != nil {
		return canonicalAdmissionMaterial{}, err
	}
	return material, nil
}

func (adapter adapter) resolveCommittedForPayload(
	ctx context.Context,
	projectRoot projectprofile.ProjectRootV1,
	payloadDigest projectprofile.ContentDigest,
) (canonicalAdmissionMaterial, error) {
	transaction, err := adapter.starter.BeginRead(ctx, adapter.database)
	if err != nil {
		return canonicalAdmissionMaterial{}, fmt.Errorf("begin payload admission resolution: %w", err)
	}
	evidence := committedPayloadEvidence{}
	arguments := []any{projectRoot.String(), payloadDigest.String()}
	destinations := []any{
		&evidence.matchCount,
		&evidence.currentCount,
		&evidence.currentRef,
		&evidence.currentDigest,
		&evidence.firstRef,
		&evidence.firstDigest,
	}
	readErr := transaction.ScanOne(
		ctx,
		selectCommittedV2ByPayloadSQL,
		arguments,
		destinations,
	)
	refRaw, digestRaw, selectionErr := selectCommittedPayloadIdentity(evidence)
	readErr = errors.Join(readErr, selectionErr)
	material := canonicalAdmissionMaterial{}
	if readErr == nil {
		admissionRef, refErr := projectprofile.NewProfileDeclarationAdmissionRecordRef(refRaw)
		admissionDigest, digestErr := projectprofile.NewContentDigest(digestRaw)
		readErr = errors.Join(refErr, digestErr)
		if readErr == nil {
			material, readErr = resolveCanonicalByReferenceOnConnection(
				ctx,
				transaction,
				projectRoot,
				admissionRef,
				admissionDigest,
			)
		}
	}
	finish := adapter.rollbackTransaction(transaction)
	finishErr := finishError(finish)
	if readErr != nil {
		return canonicalAdmissionMaterial{}, readErr
	}
	if finishErr != nil {
		return canonicalAdmissionMaterial{}, fmt.Errorf("finish payload admission resolution: %w", finishErr)
	}
	if err := adapter.validateHistoricalAuthorityMaterial(ctx, material); err != nil {
		return canonicalAdmissionMaterial{}, err
	}
	return material, nil
}

func (adapter adapter) validateHistoricalAuthorityMaterial(
	ctx context.Context,
	material canonicalAdmissionMaterial,
) error {
	if material.storageGeneration == "v1" {
		return nil
	}
	if material.storageGeneration == "v3" {
		transaction, err := adapter.starter.BeginRead(ctx, adapter.database)
		if err != nil {
			return fmt.Errorf("begin historical v3 authority validation: %w", err)
		}
		validateErr := validateV3HistoricalMaterialInTransaction(
			ctx,
			transaction,
			material,
		)
		finish := adapter.rollbackTransaction(transaction)
		if validateErr != nil {
			return validateErr
		}
		if err := finishError(finish); err != nil {
			return fmt.Errorf("finish historical v3 authority validation: %w", err)
		}
		return nil
	}
	if material.storageGeneration == "v4" {
		transaction, err := adapter.starter.BeginRead(ctx, adapter.database)
		if err != nil {
			return fmt.Errorf("begin historical v4 authority validation: %w", err)
		}
		validateErr := validateV4HistoricalMaterialInTransaction(
			ctx,
			transaction,
			material,
		)
		finish := adapter.rollbackTransaction(transaction)
		if validateErr != nil {
			return validateErr
		}
		if err := finishError(finish); err != nil {
			return fmt.Errorf("finish historical v4 authority validation: %w", err)
		}
		return nil
	}
	if material.storageGeneration == "v5" {
		transaction, err := adapter.starter.BeginRead(ctx, adapter.database)
		if err != nil {
			return fmt.Errorf("begin historical v5 authority validation: %w", err)
		}
		validateErr := validateV5HistoricalMaterialInTransaction(
			ctx,
			transaction,
			material,
		)
		finish := adapter.rollbackTransaction(transaction)
		if validateErr != nil {
			return validateErr
		}
		if err := finishError(finish); err != nil {
			return fmt.Errorf("finish historical v5 authority validation: %w", err)
		}
		return nil
	}
	if material.storageGeneration != "v2" {
		return fmt.Errorf("canonical admission has an unknown storage generation")
	}
	useRef, err := profileauthority.NewProfileDeclarationAuthorityUseRef(
		material.authorityUseRef,
	)
	if err != nil {
		return fmt.Errorf("parse historical v2 authority-use ref: %w", err)
	}
	useDigest, err := authority.NewDigest(material.authorityUseDigest.String())
	if err != nil {
		return fmt.Errorf("parse historical v2 authority-use digest: %w", err)
	}
	use, err := profileauthoritysqlite.LoadAuthorityUseRecord(
		ctx,
		adapter.database,
		useRef,
		useDigest,
	)
	if err != nil {
		return fmt.Errorf("strict-load historical v2 authority use: %w", err)
	}
	return validateAuthorityUseAgainstMaterial(use, material)
}

func validateAuthorityUseAgainstMaterial(
	use profileauthority.AuthorityUseRecord,
	material canonicalAdmissionMaterial,
) error {
	resolutionRef, resolutionDigest, resolutionOK := use.Resolution()
	basisRef, basisDigest, basisOK := use.Basis()
	projectRoot, _, _, projectOK := use.ProjectBinding()
	committedRef, committedDigest, committedOK := use.CommittedAdmission()
	consumedAt, consumedOK := use.ConsumedAt()
	if !resolutionOK || !basisOK || !projectOK || !committedOK || !consumedOK {
		return fmt.Errorf("historical v2 authority use is incomplete")
	}
	checks := []struct {
		matches bool
		name    string
	}{
		{matches: resolutionRef.String() == material.authorityResolutionRef.String(), name: "authority-resolution ref"},
		{matches: resolutionDigest.String() == material.authorityResolutionDigest.String(), name: "authority-resolution digest"},
		{matches: basisRef.String() == material.authorityBasisRef.String(), name: "authority-basis ref"},
		{matches: basisDigest.String() == material.authorityBasisDigest.String(), name: "authority-basis digest"},
		{matches: projectRoot.String() == material.projectRoot.String(), name: "project root"},
		{matches: committedRef.String() == material.admissionRef.String(), name: "committed admission ref"},
		{matches: committedDigest.String() == material.admissionDigest.String(), name: "committed admission digest"},
		{matches: consumedAt.Equal(material.recordedAt), name: "authority-use consumption time"},
	}
	return firstMismatch(checks, "historical v2 authority use")
}

func selectCommittedPayloadIdentity(
	evidence committedPayloadEvidence,
) (string, string, error) {
	if evidence.matchCount == 0 {
		return "", "", errNoCommittedAuthorityBasis
	}
	if evidence.currentCount == 1 {
		return evidence.currentRef, evidence.currentDigest, nil
	}
	if evidence.currentCount > 1 {
		return "", "", fmt.Errorf("current profile projection is ambiguous")
	}
	if evidence.matchCount == 1 {
		return evidence.firstRef, evidence.firstDigest, nil
	}
	return "", "", errAmbiguousCommittedPayload
}

func resolveCurrentCanonicalOnConnection(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	projectRoot projectprofile.ProjectRootV1,
) (canonicalAdmissionMaterial, error) {
	canonicalRoot, err := parseProjectRoot(projectRoot)
	if err != nil {
		return canonicalAdmissionMaterial{}, err
	}
	head, err := loadExactLedgerHead(ctx, transaction, canonicalRoot)
	if err != nil {
		return canonicalAdmissionMaterial{}, err
	}
	headValue := head.Value()
	if headValue == 0 {
		return canonicalAdmissionMaterial{}, errNoCurrentCanonicalAdmission
	}
	identity, err := loadCurrentAdmissionIdentity(
		ctx,
		transaction,
		canonicalRoot,
		head,
	)
	if err != nil {
		return canonicalAdmissionMaterial{}, err
	}
	return resolveCanonicalByReferenceOnConnection(
		ctx,
		transaction,
		canonicalRoot,
		identity.ref,
		identity.digest,
	)
}

func resolveCanonicalByReferenceOnConnection(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	projectRoot projectprofile.ProjectRootV1,
	admissionRef projectprofile.ProfileDeclarationAdmissionRecordRef,
	expectedDigest projectprofile.ContentDigest,
) (canonicalAdmissionMaterial, error) {
	canonicalRoot, err := parseProjectRoot(projectRoot)
	if err != nil {
		return canonicalAdmissionMaterial{}, err
	}
	head, err := loadExactLedgerHead(ctx, transaction, canonicalRoot)
	if err != nil {
		return canonicalAdmissionMaterial{}, err
	}
	headValue := head.Value()
	if headValue == 0 {
		return canonicalAdmissionMaterial{}, errNoCurrentCanonicalAdmission
	}
	admissionRefRaw := admissionRef.String()
	parsedRef, err := projectprofile.NewProfileDeclarationAdmissionRecordRef(
		admissionRefRaw,
	)
	if err != nil {
		return canonicalAdmissionMaterial{}, fmt.Errorf("validate exact admission ref: %w", err)
	}
	expectedDigestRaw := expectedDigest.String()
	parsedDigest, err := projectprofile.NewContentDigest(expectedDigestRaw)
	if err != nil {
		return canonicalAdmissionMaterial{}, fmt.Errorf("validate exact admission digest: %w", err)
	}
	parsedRefRaw := parsedRef.String()
	row, err := loadDurableAdmissionRow(ctx, transaction, parsedRefRaw)
	if err != nil {
		return canonicalAdmissionMaterial{}, err
	}
	parsedDigestRaw := parsedDigest.String()
	if row.admissionDigest != parsedDigestRaw {
		return canonicalAdmissionMaterial{}, fmt.Errorf("durable admission digest differs from exact expected digest")
	}
	canonicalRootRaw := canonicalRoot.String()
	if row.projectRoot != canonicalRootRaw {
		return canonicalAdmissionMaterial{}, fmt.Errorf("durable admission belongs to another project root")
	}
	headRevision, err := profileLedgerRevisionSQLiteValue(
		"exact ledger head revision",
		headValue,
	)
	if err != nil {
		return canonicalAdmissionMaterial{}, err
	}
	if row.ledgerRevision > headRevision {
		return canonicalAdmissionMaterial{}, fmt.Errorf("durable admission revision exceeds the exact ledger head")
	}
	return reconstructCanonicalAdmissionMaterial(ctx, transaction, canonicalRoot, row)
}

func loadCurrentAdmissionIdentity(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	projectRoot projectprofile.ProjectRootV1,
	revision projectprofile.LedgerRevision,
) (currentAdmissionIdentity, error) {
	var refRaw string
	var digestRaw string
	projectRootRaw := projectRoot.String()
	revisionValue := revision.Value()
	revisionRaw, err := profileLedgerRevisionSQLiteValue(
		"current admission revision",
		revisionValue,
	)
	if err != nil {
		return currentAdmissionIdentity{}, err
	}
	arguments := []any{projectRootRaw, revisionRaw}
	destinations := []any{&refRaw, &digestRaw}
	err = transaction.ScanOne(
		ctx,
		selectCurrentAdmissionIdentitySQL,
		arguments,
		destinations,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return currentAdmissionIdentity{}, fmt.Errorf("exact profile ledger head has no current Declared admission")
	}
	if err != nil {
		return currentAdmissionIdentity{}, fmt.Errorf("load current profile-admission identity: %w", err)
	}
	ref, err := projectprofile.NewProfileDeclarationAdmissionRecordRef(refRaw)
	if err != nil {
		return currentAdmissionIdentity{}, fmt.Errorf("parse current admission ref: %w", err)
	}
	digest, err := projectprofile.NewContentDigest(digestRaw)
	if err != nil {
		return currentAdmissionIdentity{}, fmt.Errorf("parse current admission digest: %w", err)
	}
	return currentAdmissionIdentity{ref: ref, digest: digest}, nil
}

func reconstructCanonicalAdmissionMaterial(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	projectRoot projectprofile.ProjectRootV1,
	row durableAdmissionRow,
) (canonicalAdmissionMaterial, error) {
	err := validateRequestFreeDurableRow(row)
	if err != nil {
		return canonicalAdmissionMaterial{}, err
	}
	candidate, err := decodeCandidateFromCanonicalParts(
		[]byte(row.payloadJSON),
		[]byte(row.candidateProvenanceJSON),
	)
	if err != nil {
		return canonicalAdmissionMaterial{}, fmt.Errorf("reconstruct durable profile candidate: %w", err)
	}
	provenance := candidate.Provenance()
	if provenance.ProjectRoot() != projectRoot {
		return canonicalAdmissionMaterial{}, fmt.Errorf("durable candidate belongs to another project root")
	}
	snapshot, err := projectprofilesqlite.ResolveProfileAdmissionValueSnapshotV1(
		ctx,
		transaction,
		candidate,
	)
	if err != nil {
		return canonicalAdmissionMaterial{}, fmt.Errorf("resolve durable profile-admission support DAG: %w", err)
	}
	values, ok := snapshot.Values()
	if !ok {
		return canonicalAdmissionMaterial{}, fmt.Errorf("durable profile-admission support DAG is unusable")
	}
	err = validateCandidateAgainstDurableValues(candidate, values)
	if err != nil {
		return canonicalAdmissionMaterial{}, err
	}
	if row.storageGeneration == "v1" {
		authorityAdmissionRef, parseErr := authority.NewProfileDeclarationAdmissionRecordRef(row.admissionID)
		if parseErr != nil {
			return canonicalAdmissionMaterial{}, fmt.Errorf("parse historical authority admission ref: %w", parseErr)
		}
		authorityProof, proofErr := authority.ResolveHistoricalProfileAdmissionAuthorityV1(
			ctx,
			transaction,
			authorityAdmissionRef,
		)
		if proofErr != nil {
			return canonicalAdmissionMaterial{}, fmt.Errorf("resolve historical profile-admission authority: %w", proofErr)
		}
		if proofErr = validateHistoricalAuthorityProof(row, authorityProof); proofErr != nil {
			return canonicalAdmissionMaterial{}, proofErr
		}
	}
	material, err := parseCanonicalAdmissionMaterial(projectRoot, candidate, row)
	if err != nil {
		return canonicalAdmissionMaterial{}, err
	}
	err = validateCanonicalAdmissionMaterial(material)
	if err != nil {
		return canonicalAdmissionMaterial{}, err
	}
	return material, nil
}

func validateCandidateAgainstDurableValues(
	candidate projectprofile.ProfileDeclarationCandidateV1,
	values projectprofilesqlite.ProfileOnboardingValueSetV1,
) error {
	support, err := projectprofile.CarryProfileAuthorAssignmentSupportV1(
		values.SystemAdmission(),
		values.RoleAdmission(),
		values.AssignmentJustification(),
		values.AssignmentProvenance(),
	)
	if err != nil {
		return fmt.Errorf("reconstruct durable ProfileAuthor assignment support: %w", err)
	}
	err = validateDurableCandidateAgainstMethodEdition(candidate, values, support)
	if err != nil {
		return fmt.Errorf("validate durable candidate against support DAG: %w", err)
	}
	return nil
}

func validateDurableCandidateAgainstMethodEdition(
	candidate projectprofile.ProfileDeclarationCandidateV1,
	values projectprofilesqlite.ProfileOnboardingValueSetV1,
	support projectprofile.ProfileAuthorAssignmentSupportCarrierV1,
) error {
	work := values.WorkRecord()
	assignment := values.RoleAssignment()
	basis := values.ObservedBasis()
	effect := values.Effect()
	assessment := values.Assessment()
	switch description := values.MethodDescriptionEdition().(type) {
	case projectprofile.ProfileOnboardingMethodDescriptionV1:
		contract, ok := values.MethodContractEdition().(projectprofile.ProfileOnboardingMethodContractV1)
		if !ok {
			return fmt.Errorf("durable profile-onboarding method editions differ")
		}
		return projectprofile.ValidateProfileDeclarationCandidateV1AgainstSupports(
			candidate,
			work,
			description,
			contract,
			assignment,
			support,
			basis,
			effect,
			assessment,
		)
	case projectprofile.ProfileOnboardingMethodDescriptionV2:
		contract, ok := values.MethodContractEdition().(projectprofile.ProfileOnboardingMethodContractV2)
		if !ok {
			return fmt.Errorf("durable profile-onboarding method editions differ")
		}
		workInputRef, ok := work.ProfileOnboardingWorkInputRefV2()
		if !ok {
			return fmt.Errorf("durable profile-onboarding Work v2 omits its exact WorkInput ref")
		}
		return projectprofile.ValidateProfileDeclarationCandidateV1AgainstSupportsV2(
			candidate,
			work,
			description,
			contract,
			assignment,
			support,
			basis,
			effect,
			assessment,
			workInputRef,
		)
	default:
		return fmt.Errorf("durable profile-onboarding method edition is unsupported")
	}
}

func validateHistoricalAuthorityProof(
	row durableAdmissionRow,
	proof authority.HistoricalProfileAdmissionAuthorityProofV1,
) error {
	useRef, useRefOK := proof.AuthorityUseRecordRef()
	resolutionID, resolutionIDOK := proof.AuthorityResolutionID()
	resolutionDigest, resolutionDigestOK := proof.AuthorityResolutionDigest()
	basisRef, basisRefOK := proof.AuthorityBasisRef()
	basisDigest, basisDigestOK := proof.AuthorityBasisDigest()
	actionKind, actionKindOK := proof.ActionKind()
	projectRoot, projectRootOK := proof.ProjectRoot()
	projectBindingDigest, bindingOK := proof.ProjectBindingDigest()
	envelopeDigest, envelopeOK := proof.EnvelopeDigest()
	singleUseKey, singleUseOK := proof.SingleUseKey()
	requestDigest, requestOK := proof.AdmissionRequestDigest()
	verifierIdentity, verifierIdentityOK := proof.VerifierIdentity()
	verifierVersion, verifierVersionOK := proof.VerifierVersion()
	committedRef, committedRefOK := proof.CommittedResultRef()
	committedDigest, committedDigestOK := proof.CommittedResultDigest()
	consumedAt, consumedAtOK := proof.ConsumedAt()
	allPresent := useRefOK && resolutionIDOK && resolutionDigestOK &&
		basisRefOK && basisDigestOK && actionKindOK && projectRootOK &&
		bindingOK && envelopeOK && singleUseOK && requestOK &&
		verifierIdentityOK && verifierVersionOK && committedRefOK &&
		committedDigestOK && consumedAtOK
	if !allPresent {
		return fmt.Errorf("historical profile-admission authority proof is incomplete")
	}
	useRefRaw := useRef.String()
	resolutionIDRaw := resolutionID.String()
	resolutionDigestRaw := resolutionDigest.String()
	basisRefRaw := basisRef.String()
	basisDigestRaw := basisDigest.String()
	actionKindRaw := actionKind.String()
	projectRootRaw := projectRoot.String()
	projectBindingDigestRaw := projectBindingDigest.String()
	envelopeDigestRaw := envelopeDigest.String()
	singleUseKeyRaw := singleUseKey.String()
	requestDigestRaw := requestDigest.String()
	verifierIdentityRaw := verifierIdentity.String()
	verifierVersionRaw := verifierVersion.String()
	committedRefRaw := committedRef.String()
	committedDigestRaw := committedDigest.String()
	consumedAtRaw := formatTime(consumedAt)
	checks := []struct {
		matches bool
		name    string
	}{
		{matches: useRefRaw == row.useID, name: "authority-use ref"},
		{matches: resolutionIDRaw == row.authorityResolutionRef, name: "authority-resolution ref"},
		{matches: resolutionDigestRaw == row.authorityResolutionDigest, name: "authority-resolution digest"},
		{matches: basisRefRaw == row.authorityBasisRef, name: "authority-basis ref"},
		{matches: basisDigestRaw == row.authorityBasisDigest, name: "authority-basis digest"},
		{matches: actionKindRaw == row.actionKind, name: "authority action kind"},
		{matches: projectRootRaw == row.projectRoot, name: "authority project root"},
		{matches: projectBindingDigestRaw == row.projectBindingDigest, name: "authority project-binding digest"},
		{matches: envelopeDigestRaw == row.useEnvelopeDigest, name: "authority envelope digest"},
		{matches: singleUseKeyRaw == row.singleUseKey, name: "authority single-use key"},
		{matches: requestDigestRaw == row.admissionRequestDigest, name: "authority admission-request digest"},
		{matches: verifierIdentityRaw == row.useVerifierIdentity, name: "authority verifier identity"},
		{matches: verifierVersionRaw == row.useVerifierVersion, name: "authority verifier version"},
		{matches: committedRefRaw == row.admissionID, name: "authority committed-result ref"},
		{matches: committedDigestRaw == row.admissionDigest, name: "authority committed-result digest"},
		{matches: consumedAtRaw == row.consumedAt, name: "authority consumption time"},
	}
	err := firstMismatch(checks, "historical profile-admission authority")
	if err != nil {
		return err
	}
	return nil
}

func parseCanonicalAdmissionMaterial(
	projectRoot projectprofile.ProjectRootV1,
	candidate projectprofile.ProfileDeclarationCandidateV1,
	row durableAdmissionRow,
) (canonicalAdmissionMaterial, error) {
	admissionRef, err := projectprofile.NewProfileDeclarationAdmissionRecordRef(row.admissionID)
	if err != nil {
		return canonicalAdmissionMaterial{}, fmt.Errorf("parse durable admission ref: %w", err)
	}
	admissionDigest, err := projectprofile.NewContentDigest(row.admissionDigest)
	if err != nil {
		return canonicalAdmissionMaterial{}, fmt.Errorf("parse durable admission digest: %w", err)
	}
	receiptDigest, err := projectprofile.NewContentDigest(row.receiptDigest)
	if err != nil {
		return canonicalAdmissionMaterial{}, fmt.Errorf("parse durable receipt digest: %w", err)
	}
	provenanceDigest, err := projectprofile.NewContentDigest(row.candidateProvenanceDigest)
	if err != nil {
		return canonicalAdmissionMaterial{}, fmt.Errorf("parse durable provenance digest: %w", err)
	}
	workRecordRef, err := projectprofile.NewProfileOnboardingWorkRecordRef(row.workRecordRef)
	if err != nil {
		return canonicalAdmissionMaterial{}, fmt.Errorf("parse durable Work-record ref: %w", err)
	}
	workRecordDigest, err := projectprofile.NewContentDigest(row.workRecordDigest)
	if err != nil {
		return canonicalAdmissionMaterial{}, fmt.Errorf("parse durable Work-record digest: %w", err)
	}
	authorityBasisRef, err := projectprofile.NewProfileDeclarationAuthorityBasisRef(row.authorityBasisRef)
	if err != nil {
		return canonicalAdmissionMaterial{}, fmt.Errorf("parse durable authority-basis ref: %w", err)
	}
	authorityBasisDigest, err := projectprofile.NewContentDigest(row.authorityBasisDigest)
	if err != nil {
		return canonicalAdmissionMaterial{}, fmt.Errorf("parse durable authority-basis digest: %w", err)
	}
	authorityResolutionRef, err := projectprofile.NewAuthorityResolutionRecordRef(row.authorityResolutionRef)
	if err != nil {
		return canonicalAdmissionMaterial{}, fmt.Errorf("parse durable authority-resolution ref: %w", err)
	}
	authorityResolutionDigest, err := projectprofile.NewContentDigest(row.authorityResolutionDigest)
	if err != nil {
		return canonicalAdmissionMaterial{}, fmt.Errorf("parse durable authority-resolution digest: %w", err)
	}
	authorityUseDigest := projectprofile.ContentDigest{}
	if row.storageGeneration == "v2" ||
		row.storageGeneration == "v3" ||
		row.storageGeneration == "v4" ||
		row.storageGeneration == "v5" {
		authorityUseDigest, err = projectprofile.NewContentDigest(row.useDigest)
		if err != nil {
			return canonicalAdmissionMaterial{}, fmt.Errorf("parse durable %s authority-use digest: %w", row.storageGeneration, err)
		}
	}
	profileAuthorRoleAssignmentRef, err := projectprofile.NewRoleAssignmentRef(row.roleAssignmentRef)
	if err != nil {
		return canonicalAdmissionMaterial{}, fmt.Errorf("parse durable ProfileAuthorRoleAssignment ref: %w", err)
	}
	profileAuthorRoleAssignmentDigest, err := projectprofile.NewContentDigest(row.roleAssignmentDigest)
	if err != nil {
		return canonicalAdmissionMaterial{}, fmt.Errorf("parse durable ProfileAuthorRoleAssignment digest: %w", err)
	}
	observedProjectBasisRef, err := projectprofile.NewObservedProjectBasisRefV1(row.observedBasisRef)
	if err != nil {
		return canonicalAdmissionMaterial{}, fmt.Errorf("parse durable ObservedProjectBasis ref: %w", err)
	}
	observedProjectBasisDigest, err := projectprofile.NewContentDigest(row.observedBasisDigest)
	if err != nil {
		return canonicalAdmissionMaterial{}, fmt.Errorf("parse durable ObservedProjectBasis digest: %w", err)
	}
	outcomeAssessmentRef, err := projectprofile.NewProfileOnboardingOutcomeAssessmentRefV1(row.outcomeAssessmentRef)
	if err != nil {
		return canonicalAdmissionMaterial{}, fmt.Errorf("parse durable outcome-assessment ref: %w", err)
	}
	outcomeAssessmentDigest, err := projectprofile.NewContentDigest(row.outcomeAssessmentDigest)
	if err != nil {
		return canonicalAdmissionMaterial{}, fmt.Errorf("parse durable outcome-assessment digest: %w", err)
	}
	expectedLedgerRevision, err := parseLedgerRevision(row.expectedLedgerRevision, true)
	if err != nil {
		return canonicalAdmissionMaterial{}, err
	}
	ledgerRevision, err := parseLedgerRevision(row.ledgerRevision, false)
	if err != nil {
		return canonicalAdmissionMaterial{}, err
	}
	recordedAt, err := parseCanonicalTime(row.recordedAt)
	if err != nil {
		return canonicalAdmissionMaterial{}, err
	}
	origin, err := profileAdmissionOriginForStorageGeneration(
		row.storageGeneration,
	)
	if err != nil {
		return canonicalAdmissionMaterial{}, err
	}
	return canonicalAdmissionMaterial{
		storageGeneration:                 row.storageGeneration,
		origin:                            origin,
		projectRoot:                       projectRoot,
		candidate:                         candidate,
		payloadJSON:                       []byte(row.payloadJSON),
		provenanceJSON:                    []byte(row.candidateProvenanceJSON),
		provenanceDigest:                  provenanceDigest,
		admissionRef:                      admissionRef,
		admissionDigest:                   admissionDigest,
		admissionJSON:                     []byte(row.admissionJSON),
		receiptJSON:                       []byte(row.receiptJSON),
		receiptDigest:                     receiptDigest,
		workRecordRef:                     workRecordRef,
		workRecordDigest:                  workRecordDigest,
		authorityBasisRef:                 authorityBasisRef,
		authorityBasisDigest:              authorityBasisDigest,
		authorityResolutionRef:            authorityResolutionRef,
		authorityResolutionDigest:         authorityResolutionDigest,
		authorityUseRef:                   row.useID,
		authorityUseDigest:                authorityUseDigest,
		profileAuthorRoleAssignmentRef:    profileAuthorRoleAssignmentRef,
		profileAuthorRoleAssignmentDigest: profileAuthorRoleAssignmentDigest,
		observedProjectBasisRef:           observedProjectBasisRef,
		observedProjectBasisDigest:        observedProjectBasisDigest,
		outcomeAssessmentRef:              outcomeAssessmentRef,
		outcomeAssessmentDigest:           outcomeAssessmentDigest,
		expectedLedgerRevision:            expectedLedgerRevision,
		ledgerRevision:                    ledgerRevision,
		recordedAt:                        recordedAt,
	}, nil
}

func validateCanonicalAdmissionMaterial(material canonicalAdmissionMaterial) error {
	if _, ok := projectprofile.ParseProfileAdmissionOrigin(
		string(material.origin),
	); !ok {
		return fmt.Errorf("canonical profile admission origin is invalid")
	}
	materialPayload := material.candidate.Payload()
	materialProvenance := material.candidate.Provenance()
	candidate, err := projectprofile.NewProfileDeclarationCandidateV1(
		materialPayload,
		materialProvenance,
	)
	if err != nil {
		return fmt.Errorf("validate canonical profile candidate: %w", err)
	}
	candidatePayload := candidate.Payload()
	payloadJSON, err := projectprofile.EncodeProfileDeclarationPayloadCanonicalJSON(candidatePayload)
	if err != nil {
		return err
	}
	if !equalCanonicalBytes(payloadJSON, material.payloadJSON) {
		return fmt.Errorf("canonical profile admission has mismatched payload bytes")
	}
	provenance := candidate.Provenance()
	provenanceDigest, err := projectprofile.DigestCandidateProvenanceV1(provenance)
	if err != nil {
		return err
	}
	provenanceProjectRoot := provenance.ProjectRoot()
	provenanceWorkRef := provenance.WorkRecordRef()
	provenanceWorkDigest := provenance.WorkRecordDigest()
	provenanceAuthorityBasisRef := provenance.AuthorityBasisRef()
	provenanceAssignmentRef := provenance.ProfileAuthorRoleAssignmentRef()
	provenanceAssignmentDigest := provenance.ProfileAuthorRoleAssignmentDigest()
	provenanceObservedBasisRef := provenance.ObservedProjectBasisRef()
	provenanceObservedBasisDigest := provenance.ObservedProjectBasisDigest()
	provenanceAssessmentRef := provenance.OutcomeAssessmentRef()
	provenanceAssessmentDigest := provenance.OutcomeAssessmentDigest()
	checks := []struct {
		matches bool
		name    string
	}{
		{matches: provenanceProjectRoot == material.projectRoot, name: "project root"},
		{matches: provenanceDigest == material.provenanceDigest, name: "candidate provenance digest"},
		{matches: provenanceWorkRef == material.workRecordRef, name: "Work-record ref"},
		{matches: provenanceWorkDigest == material.workRecordDigest, name: "Work-record digest"},
		{matches: provenanceAuthorityBasisRef == material.authorityBasisRef, name: "authority-basis ref"},
		{matches: provenanceAssignmentRef == material.profileAuthorRoleAssignmentRef, name: "ProfileAuthorRoleAssignment ref"},
		{matches: provenanceAssignmentDigest == material.profileAuthorRoleAssignmentDigest, name: "ProfileAuthorRoleAssignment digest"},
		{matches: provenanceObservedBasisRef == material.observedProjectBasisRef, name: "ObservedProjectBasis ref"},
		{matches: provenanceObservedBasisDigest == material.observedProjectBasisDigest, name: "ObservedProjectBasis digest"},
		{matches: provenanceAssessmentRef == material.outcomeAssessmentRef, name: "outcome-assessment ref"},
		{matches: provenanceAssessmentDigest == material.outcomeAssessmentDigest, name: "outcome-assessment digest"},
	}
	err = firstMismatch(checks, "canonical profile admission")
	if err != nil {
		return err
	}
	admissionIdentity := admissionRecordIdentityProjection{}
	err = json.Unmarshal(material.admissionJSON, &admissionIdentity)
	if err != nil {
		return fmt.Errorf("decode durable admission identity: %w", err)
	}
	admissionRefRaw := material.admissionRef.String()
	if admissionIdentity.AdmissionRecordRef != admissionRefRaw {
		return fmt.Errorf("canonical profile admission has a mismatched admission ref")
	}
	payloadDigest, err := projectprofile.DigestProfileDeclarationPayload(candidatePayload)
	if err != nil {
		return err
	}
	workRecordRefRaw := material.workRecordRef.String()
	authorityBasisRefRaw := material.authorityBasisRef.String()
	authorityResolutionRefRaw := material.authorityResolutionRef.String()
	authorityResolutionDigestRaw := material.authorityResolutionDigest.String()
	expectedRevisionValue := material.expectedLedgerRevision.Value()
	ledgerRevisionValue := material.ledgerRevision.Value()
	recordedAtRaw := formatTime(material.recordedAt)
	provenanceDigestRaw := material.provenanceDigest.String()
	payloadDigestRaw := payloadDigest.String()
	observedBasisDigestRaw := material.observedProjectBasisDigest.String()
	identityChecks := []struct {
		matches bool
		name    string
	}{
		{matches: admissionIdentity.Schema == "haft.project-profile.admission-record/v1", name: "admission schema"},
		{matches: admissionIdentity.ClassificationWorkRecordRef == workRecordRefRaw, name: "admission Work ref"},
		{matches: admissionIdentity.AuthorityBasisRef == authorityBasisRefRaw, name: "admission authority-basis ref"},
		{matches: admissionIdentity.AuthorityResolutionRecordRef == authorityResolutionRefRaw, name: "admission authority-resolution ref"},
		{matches: admissionIdentity.AuthorityResolutionRecordDigest == authorityResolutionDigestRaw, name: "admission authority-resolution digest"},
		{matches: admissionIdentity.ExpectedLedgerRevision == expectedRevisionValue, name: "admission expected revision"},
		{matches: admissionIdentity.CommittedLedgerRevision == ledgerRevisionValue, name: "admission committed revision"},
		{matches: admissionIdentity.CommittedAt == recordedAtRaw, name: "admission recording time"},
		{matches: admissionIdentity.Receipt.Schema == "haft.project-profile.declaration-receipt/v1", name: "receipt schema"},
		{matches: admissionIdentity.Receipt.AuthorityResolutionRecordRef == authorityResolutionRefRaw, name: "receipt authority-resolution ref"},
		{matches: admissionIdentity.Receipt.AuthorityResolutionRecordDigest == authorityResolutionDigestRaw, name: "receipt authority-resolution digest"},
		{matches: admissionIdentity.Receipt.AuthorityBasisRef == authorityBasisRefRaw, name: "receipt authority-basis ref"},
		{matches: admissionIdentity.Receipt.WorkRecordRef == workRecordRefRaw, name: "receipt Work ref"},
		{matches: admissionIdentity.Receipt.CandidateProvenanceDigest == provenanceDigestRaw, name: "receipt provenance digest"},
		{matches: admissionIdentity.Receipt.PayloadDigest == payloadDigestRaw, name: "receipt payload digest"},
		{matches: admissionIdentity.Receipt.ObservedBasisDigest == observedBasisDigestRaw, name: "receipt observed-basis digest"},
		{matches: admissionIdentity.Receipt.LedgerRevision == ledgerRevisionValue, name: "receipt ledger revision"},
		{matches: admissionIdentity.Receipt.RecordedAt == recordedAtRaw, name: "receipt recording time"},
	}
	err = firstMismatch(identityChecks, "canonical profile admission")
	if err != nil {
		return err
	}
	nextRevision, err := material.expectedLedgerRevision.Next()
	if err != nil {
		return err
	}
	if nextRevision != material.ledgerRevision {
		return fmt.Errorf("canonical profile admission has mismatched expected and committed revisions")
	}
	if material.recordedAt.IsZero() {
		return fmt.Errorf("canonical profile admission has no recording time")
	}
	durable, err := buildCanonicalDurableTuple(material)
	if err != nil {
		return err
	}
	err = projectprofile.ValidateDurableProfileAdmissionRecordV1(
		durable,
		material.provenanceJSON,
		material.provenanceDigest,
	)
	if err != nil {
		return fmt.Errorf("validate canonical durable profile admission: %w", err)
	}
	return nil
}

func profileAdmissionOriginForStorageGeneration(
	generation string,
) (projectprofile.ProfileAdmissionOrigin, error) {
	switch generation {
	case "v1", "v2":
		return projectprofile.ProfileAdmissionOriginLegacyUnknown, nil
	case "v3":
		return projectprofile.ProfileAdmissionOriginExplicitOperator, nil
	case "v4":
		return projectprofile.ProfileAdmissionOriginDetectorDefault, nil
	case "v5":
		return projectprofile.ProfileAdmissionOriginHostRoutedOperatorRequest, nil
	default:
		return "", fmt.Errorf(
			"canonical admission has unknown storage generation %q",
			generation,
		)
	}
}

func buildCanonicalDurableTuple(
	material canonicalAdmissionMaterial,
) (projectprofile.DurableProfileAdmissionTupleV1, error) {
	payload := material.candidate.Payload()
	payloadDigest, err := projectprofile.DigestProfileDeclarationPayload(payload)
	if err != nil {
		return nil, err
	}
	builder := projectprofile.NewDurableProfileAdmissionTupleV1Builder(
		material.admissionJSON,
		material.admissionDigest,
	)
	builder = builder.WithReceipt(material.receiptJSON, material.receiptDigest)
	builder = builder.WithPayload(material.payloadJSON, payloadDigest)
	builder = builder.AtLedgerRevision(material.ledgerRevision)
	return builder.Build()
}

func decodeCandidateFromCanonicalParts(
	payloadJSON []byte,
	provenanceJSON []byte,
) (projectprofile.ProfileDeclarationCandidateV1, error) {
	envelope := candidateCanonicalEnvelope{
		Schema:     "haft.project-profile.declaration-candidate/v1",
		Payload:    append(json.RawMessage{}, payloadJSON...),
		Provenance: append(json.RawMessage{}, provenanceJSON...),
	}
	canonicalJSON, err := json.Marshal(envelope)
	if err != nil {
		return projectprofile.ProfileDeclarationCandidateV1{}, err
	}
	return projectprofile.DecodeProfileDeclarationCandidateV1CanonicalJSON(canonicalJSON)
}

func validateRequestFreeDurableRow(row durableAdmissionRow) error {
	if row.storageGeneration != "v1" && row.storageGeneration != "v2" &&
		row.storageGeneration != "v3" && row.storageGeneration != "v4" &&
		row.storageGeneration != "v5" {
		return fmt.Errorf("durable profile admission has an unknown storage generation")
	}
	if row.storageGeneration == "v1" && row.useDigest != "" {
		return fmt.Errorf("legacy durable profile admission has a v2 authority-use digest")
	}
	if row.storageGeneration == "v2" || row.storageGeneration == "v3" ||
		row.storageGeneration == "v4" || row.storageGeneration == "v5" {
		_, err := projectprofile.NewContentDigest(row.useDigest)
		if err != nil {
			return fmt.Errorf("parse %s authority-use digest: %w", row.storageGeneration, err)
		}
	}
	expectedRevision, err := parseLedgerRevision(row.expectedLedgerRevision, true)
	if err != nil {
		return err
	}
	committedRevision, err := expectedRevision.Next()
	if err != nil {
		return err
	}
	committedRevisionValue := committedRevision.Value()
	committedRevisionRaw, err := profileLedgerRevisionSQLiteValue(
		"committed admission revision",
		committedRevisionValue,
	)
	if err != nil {
		return err
	}
	expectedActionKind := "profile.declare.from_onboarding_candidate"
	if row.storageGeneration == "v4" {
		expectedActionKind = v4ActionKind
	}
	checks := []struct {
		matches bool
		name    string
	}{
		{matches: row.actionKind == expectedActionKind, name: "action kind"},
		{matches: row.ledgerRevision == committedRevisionRaw, name: "ledger revision"},
		{matches: row.useAuthorityResolutionRef == row.authorityResolutionRef, name: "use authority-resolution ref"},
		{matches: row.useAuthorityResolutionHash == row.authorityResolutionDigest, name: "use authority-resolution digest"},
		{matches: row.useSingleUseKey == row.singleUseKey, name: "use single-use key"},
		{matches: row.useActionKind == row.actionKind, name: "use action kind"},
		{matches: row.useProjectRoot == row.projectRoot, name: "use project root"},
		{matches: row.useProjectBindingDigest == row.projectBindingDigest, name: "use project-binding digest"},
		{matches: row.useAuthorityRecordRef == row.authorityBasisRef, name: "use authority-record ref"},
		{matches: row.useAuthorityRecordDigest == row.authorityBasisDigest, name: "use authority-record digest"},
		{matches: row.useAdmissionRequestDigest == row.admissionRequestDigest, name: "use admission-request digest"},
		{matches: row.useCommittedResultRef == row.admissionID, name: "use committed-result ref"},
		{matches: row.useCommittedResultDigest == row.admissionDigest, name: "use committed-result digest"},
		{matches: row.consumedAt == row.recordedAt, name: "use consumption time"},
		{matches: row.revisionProjectRoot == row.projectRoot, name: "revision project root"},
		{matches: row.revisionLedgerRevision == row.ledgerRevision, name: "revision ledger revision"},
		{matches: row.revisionConfiguredKind == "Declared", name: "revision configured kind"},
		{matches: row.revisionPayloadJSON == row.payloadJSON, name: "revision payload JSON"},
		{matches: row.revisionPayloadDigest == row.payloadDigest, name: "revision payload digest"},
		{matches: row.revisionReceiptJSON == row.receiptJSON, name: "revision receipt JSON"},
		{matches: row.revisionReceiptDigest == row.receiptDigest, name: "revision receipt digest"},
		{matches: row.revisionAdmissionID == row.admissionID, name: "revision admission ref"},
		{matches: row.revisionAdmissionDigest == row.admissionDigest, name: "revision admission digest"},
		{matches: row.revisionRecordedAt == row.recordedAt, name: "revision recording time"},
	}
	return firstMismatch(checks, "request-free durable profile admission")
}

func parseProjectRoot(
	projectRoot projectprofile.ProjectRootV1,
) (projectprofile.ProjectRootV1, error) {
	projectRootRaw := projectRoot.String()
	parsed, err := projectprofile.NewProjectRootV1(projectRootRaw)
	if err != nil {
		return projectprofile.ProjectRootV1{}, fmt.Errorf("validate project root: %w", err)
	}
	return parsed, nil
}

func profileLedgerRevisionSQLiteValue(
	label string,
	value uint64,
) (int64, error) {
	if value > math.MaxInt64 {
		return 0, fmt.Errorf("%s exceeds SQLite integer range", label)
	}
	return int64(value), nil
}

func isNoCurrentAdmission(err error) bool {
	return errors.Is(err, errNoCurrentCanonicalAdmission)
}
