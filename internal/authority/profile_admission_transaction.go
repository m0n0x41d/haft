package authority

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

var profileAdmissionCurrentDenialCodes = map[DenialCode]struct{}{
	DenialOutsideAuthorizationWindow: {},
	DenialResolutionNotEffective:     {},
	DenialResolutionExpired:          {},
}

var profileAdmissionIgnoredDenialCodes = map[DenialCode]struct{}{
	DenialSingleUseAlreadyConsumed: {},
}

// LoadProfileAdmissionAuthorityForTransaction reads and validates the exact
// canonical authority snapshot at kernel time. It does not begin, commit, or
// roll back a transaction; consume a single-use key; or write any profile row.
// The caller must supply the opaque capability belonging to its already-open
// canonical admission transaction.
func LoadProfileAdmissionAuthorityForTransaction(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	request ResolveRequest,
) (ProfileAdmissionAuthorityCheck, error) {
	judgementTime := canonicalAuthorityTime(time.Now())
	return loadProfileAdmissionAuthorityForTransactionAt(
		ctx,
		transaction,
		request,
		judgementTime,
	)
}

func loadProfileAdmissionAuthorityForTransactionAt(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	request ResolveRequest,
	judgementTime time.Time,
) (ProfileAdmissionAuthorityCheck, error) {
	if ctx == nil {
		return ProfileAdmissionAuthorityCheck{}, fmt.Errorf("profile-admission authority context is required")
	}
	if err := transaction.RequireActive(); err != nil {
		return ProfileAdmissionAuthorityCheck{}, fmt.Errorf(
			"profile-admission authority transaction is required: %w",
			err,
		)
	}
	if judgementTime.IsZero() {
		return ProfileAdmissionAuthorityCheck{}, fmt.Errorf("profile-admission authority judgement time is required")
	}
	if err := validateResolveRequest(request); err != nil {
		return profileAdmissionAuthorityDenied(
			DenialInvalidRequest,
			err.Error(),
		), nil
	}
	durableRequest, found, err := loadProfileDeclarationResolveRequestInTransaction(
		ctx,
		transaction,
		request.authorityResolutionID,
	)
	if err != nil {
		return profileAdmissionAuthorityDenied(
			DenialCanonicalRecordInvalid,
			err.Error(),
		), nil
	}
	if !found {
		return profileAdmissionAuthorityDenied(
			DenialCanonicalRecordMissing,
			"no exact v38 profile-declaration authority basis exists",
		), nil
	}
	if durableRequest != request {
		return profileAdmissionAuthorityDenied(
			DenialCanonicalRecordInvalid,
			"profile-declaration gate request differs from the exact v38 authority basis",
		), nil
	}
	record, err := loadAuthorityRecordForTransaction(ctx, transaction, request)
	if errors.Is(err, sql.ErrNoRows) {
		return profileAdmissionAuthorityDenied(
			DenialCanonicalRecordMissing,
			"no exact canonical presentation and authority-resolution record exists",
		), nil
	}
	if err != nil {
		return ProfileAdmissionAuthorityCheck{}, fmt.Errorf("load profile-admission authority record: %w", err)
	}
	presentation, resolution, err := parseAuthorityRecord(record)
	if err != nil {
		return profileAdmissionAuthorityDenied(
			DenialCanonicalRecordInvalid,
			err.Error(),
		), nil
	}
	reasons := compareAuthorityRecord(
		request,
		presentation,
		resolution,
		record,
		judgementTime,
	)
	blockingDenials, currentDenials := partitionProfileAdmissionAuthorityDenials(reasons)
	if presentation.envelope.actionKind.String() != profileDeclarationActionKind {
		blockingDenials = append(
			blockingDenials,
			Denial{
				code:   DenialEnvelopeMismatch,
				detail: "authority action is not profile declaration from an onboarding candidate",
			},
		)
	}
	if len(blockingDenials) > 0 {
		return profileAdmissionAuthorityDeniedMany(blockingDenials), nil
	}
	recordedUse, err := loadProfileAdmissionRecordedUse(
		ctx,
		transaction,
		resolution.id,
		presentation.envelope.singleUseKey,
	)
	if err != nil {
		return ProfileAdmissionAuthorityCheck{}, fmt.Errorf("load recorded profile-admission authority use: %w", err)
	}
	used := recordedUse != nil
	if !authorityUseFlagsMatchRecord(record, used) {
		return profileAdmissionAuthorityDenied(
			DenialCanonicalRecordInvalid,
			"authority-use indexes disagree with the exact recorded use",
		), nil
	}
	if recordedUse != nil {
		err = validateProfileDeclarationRecordedUse(
			*recordedUse,
			presentation,
			resolution,
			judgementTime,
		)
		if err != nil {
			return profileAdmissionAuthorityDenied(
				DenialCanonicalRecordInvalid,
				err.Error(),
			), nil
		}
	}
	state := profileAdmissionAuthoritySnapshotState{
		presentation:   Presentation{value: presentation},
		resolution:     resolution,
		judgementTime:  canonicalAuthorityTime(judgementTime),
		currentDenials: cloneDenials(currentDenials),
		recordedUse:    recordedUse,
	}
	snapshot := ProfileAdmissionAuthoritySnapshot{state: &state}
	if !snapshot.valid() {
		return ProfileAdmissionAuthorityCheck{}, fmt.Errorf("loaded profile-admission authority snapshot is invalid")
	}
	return ProfileAdmissionAuthorityCheck{snapshot: &snapshot}, nil
}

func partitionProfileAdmissionAuthorityDenials(
	values []Denial,
) ([]Denial, []Denial) {
	blocking := cloneDenials(values)
	blocking = slices.DeleteFunc(blocking, isNonBlockingProfileAdmissionDenial)
	current := cloneDenials(values)
	current = slices.DeleteFunc(current, isNotCurrentProfileAdmissionDenial)
	return blocking, current
}

func isNonBlockingProfileAdmissionDenial(value Denial) bool {
	_, current := profileAdmissionCurrentDenialCodes[value.code]
	_, ignored := profileAdmissionIgnoredDenialCodes[value.code]
	return current || ignored
}

func isNotCurrentProfileAdmissionDenial(value Denial) bool {
	_, current := profileAdmissionCurrentDenialCodes[value.code]
	return !current
}

func authorityUseFlagsMatchRecord(record authorityRecordRow, used bool) bool {
	resolutionUsed := record.resolutionUsed == 1
	singleUseKeyUsed := record.singleUseKeyUsed == 1
	flagsAreBoolean := (record.resolutionUsed == 0 || resolutionUsed) &&
		(record.singleUseKeyUsed == 0 || singleUseKeyUsed)
	return flagsAreBoolean && resolutionUsed == used && singleUseKeyUsed == used
}

func profileAdmissionAuthorityDenied(
	code DenialCode,
	detail string,
) ProfileAdmissionAuthorityCheck {
	denial := newProfileAdmissionNotAdmitted(code, detail)
	return ProfileAdmissionAuthorityCheck{notAdmitted: &denial}
}

func profileAdmissionAuthorityDeniedMany(
	reasons []Denial,
) ProfileAdmissionAuthorityCheck {
	denial := NotAdmitted{reasons: cloneDenials(reasons)}
	return ProfileAdmissionAuthorityCheck{notAdmitted: &denial}
}

const loadProfileAdmissionRecordedUseSQL = `SELECT
	u.use_id,
	u.authority_resolution_ref,
	u.authority_resolution_digest,
	u.single_use_key,
	u.action_kind,
	u.project_root,
	u.project_binding_digest,
	u.envelope_digest,
	u.authority_record_ref,
	u.authority_record_digest,
	u.admission_request_digest,
	u.verifier_identity,
	u.verifier_version,
	u.committed_result_ref,
	u.committed_result_digest,
	u.consumed_at,
	(
		SELECT COUNT(*)
		FROM authority_uses candidate
		WHERE candidate.authority_resolution_ref = ?
			OR candidate.single_use_key = ?
	)
FROM authority_uses u
WHERE u.authority_resolution_ref = ?
	OR u.single_use_key = ?
ORDER BY u.use_id
LIMIT 1`

func loadProfileAdmissionRecordedUse(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	resolutionRef AuthorityResolutionID,
	singleUseKey SingleUseKey,
) (*ProfileAdmissionRecordedUse, error) {
	raw := profileAdmissionRecordedUseRaw{}
	arguments := []any{
		resolutionRef.String(),
		singleUseKey.String(),
		resolutionRef.String(),
		singleUseKey.String(),
	}
	destinations := raw.scanTargets()
	err := transaction.ScanOne(
		ctx,
		loadProfileAdmissionRecordedUseSQL,
		arguments,
		destinations,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if raw.matchCount != 1 {
		return nil, fmt.Errorf(
			"authority resolution and single-use key match %d use rows; expected exactly one",
			raw.matchCount,
		)
	}
	value, err := raw.parse()
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func loadAuthorityRecordForTransaction(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	request ResolveRequest,
) (authorityRecordRow, error) {
	record := authorityRecordRow{}
	arguments := []any{
		request.presentationID.String(),
		request.authorityResolutionID.String(),
	}
	destinations := record.scanTargets()
	err := transaction.ScanOne(
		ctx,
		loadAuthorityRecordSQL,
		arguments,
		destinations,
	)
	return record, err
}

type profileAdmissionRecordedUseRaw struct {
	useRef                    string
	authorityResolutionRef    string
	authorityResolutionDigest string
	singleUseKey              string
	actionKind                string
	projectRoot               string
	projectBindingDigest      string
	envelopeDigest            string
	authorityRecordRef        string
	authorityRecordDigest     string
	admissionRequestDigest    string
	verifierIdentity          string
	verifierVersion           string
	committedResultRef        string
	committedResultDigest     string
	consumedAt                string
	matchCount                int
}

func (row *profileAdmissionRecordedUseRaw) scanTargets() []any {
	return []any{
		&row.useRef,
		&row.authorityResolutionRef,
		&row.authorityResolutionDigest,
		&row.singleUseKey,
		&row.actionKind,
		&row.projectRoot,
		&row.projectBindingDigest,
		&row.envelopeDigest,
		&row.authorityRecordRef,
		&row.authorityRecordDigest,
		&row.admissionRequestDigest,
		&row.verifierIdentity,
		&row.verifierVersion,
		&row.committedResultRef,
		&row.committedResultDigest,
		&row.consumedAt,
		&row.matchCount,
	}
}

func (row profileAdmissionRecordedUseRaw) parse() (ProfileAdmissionRecordedUse, error) {
	useRef, err := NewAuthorityUseRecordRef(row.useRef)
	if err != nil {
		return ProfileAdmissionRecordedUse{}, err
	}
	resolutionRef, err := NewAuthorityResolutionID(row.authorityResolutionRef)
	if err != nil {
		return ProfileAdmissionRecordedUse{}, err
	}
	resolutionDigest, err := NewDigest(row.authorityResolutionDigest)
	if err != nil {
		return ProfileAdmissionRecordedUse{}, err
	}
	singleUseKey, err := NewSingleUseKey(row.singleUseKey)
	if err != nil {
		return ProfileAdmissionRecordedUse{}, err
	}
	actionKind, err := NewActionKind(row.actionKind)
	if err != nil {
		return ProfileAdmissionRecordedUse{}, err
	}
	projectRoot, err := NewProjectRoot(row.projectRoot)
	if err != nil {
		return ProfileAdmissionRecordedUse{}, err
	}
	projectBindingDigest, err := NewDigest(row.projectBindingDigest)
	if err != nil {
		return ProfileAdmissionRecordedUse{}, err
	}
	envelopeDigest, err := NewDigest(row.envelopeDigest)
	if err != nil {
		return ProfileAdmissionRecordedUse{}, err
	}
	authorityRecordRef, err := NewPresentationID(row.authorityRecordRef)
	if err != nil {
		return ProfileAdmissionRecordedUse{}, err
	}
	authorityRecordDigest, err := NewDigest(row.authorityRecordDigest)
	if err != nil {
		return ProfileAdmissionRecordedUse{}, err
	}
	admissionRequestDigest, err := NewDigest(row.admissionRequestDigest)
	if err != nil {
		return ProfileAdmissionRecordedUse{}, err
	}
	verifierIdentity, err := NewVerifierIdentity(row.verifierIdentity)
	if err != nil {
		return ProfileAdmissionRecordedUse{}, err
	}
	verifierVersion, err := NewVerifierVersion(row.verifierVersion)
	if err != nil {
		return ProfileAdmissionRecordedUse{}, err
	}
	committedResultRef, err := NewProfileDeclarationAdmissionRecordRef(row.committedResultRef)
	if err != nil {
		return ProfileAdmissionRecordedUse{}, err
	}
	committedResultDigest, err := NewDigest(row.committedResultDigest)
	if err != nil {
		return ProfileAdmissionRecordedUse{}, err
	}
	consumedAt, err := parseAuthorityTime(row.consumedAt)
	if err != nil {
		return ProfileAdmissionRecordedUse{}, err
	}
	return ProfileAdmissionRecordedUse{
		useRef:                    useRef,
		authorityResolutionRef:    resolutionRef,
		authorityResolutionDigest: resolutionDigest,
		singleUseKey:              singleUseKey,
		actionKind:                actionKind,
		projectRoot:               projectRoot,
		projectBindingDigest:      projectBindingDigest,
		envelopeDigest:            envelopeDigest,
		authorityRecordRef:        authorityRecordRef,
		authorityRecordDigest:     authorityRecordDigest,
		admissionRequestDigest:    admissionRequestDigest,
		verifierIdentity:          verifierIdentity,
		verifierVersion:           verifierVersion,
		committedResultRef:        committedResultRef,
		committedResultDigest:     committedResultDigest,
		consumedAt:                consumedAt,
	}, nil
}
