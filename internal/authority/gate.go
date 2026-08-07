package authority

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"time"
)

const (
	authorityPresentationDigestDomain = "haft.authority.presentation/v1"
	authorityResolutionDigestDomain   = "haft.authority.resolution/v1"
	projectBindingDigestDomain        = "haft.authority.project-binding/v1"
	permissionModalityMay             = "MAY"
)

type AuthorityBasisExpectation struct {
	speechActRef               SpeechActRef
	speechActDigest            Digest
	authorizationContentRef    AuthorizationContentRef
	authorizationContentDigest Digest
	permissionRef              PermissionRef
	permissionDigest           Digest
	permissionPredicateRef     ProfileAdmissionPredicateRef
	contextPolicyRef           ContextPolicyRef
	contextPolicyDigest        Digest
}

type AuthorityBasisExpectationBuilder struct {
	value AuthorityBasisExpectation
}

func NewAuthorityBasisExpectationBuilder() AuthorityBasisExpectationBuilder {
	return AuthorityBasisExpectationBuilder{}
}

func (builder AuthorityBasisExpectationBuilder) FromSpeechAct(
	ref SpeechActRef,
	digest Digest,
) AuthorityBasisExpectationBuilder {
	builder.value.speechActRef = ref
	builder.value.speechActDigest = digest
	return builder
}

func (builder AuthorityBasisExpectationBuilder) DescribedBy(
	ref AuthorizationContentRef,
	digest Digest,
) AuthorityBasisExpectationBuilder {
	builder.value.authorizationContentRef = ref
	builder.value.authorizationContentDigest = digest
	return builder
}

func (builder AuthorityBasisExpectationBuilder) InstitutesPermission(
	ref PermissionRef,
	digest Digest,
) AuthorityBasisExpectationBuilder {
	builder.value.permissionRef = ref
	builder.value.permissionDigest = digest
	return builder
}

func (builder AuthorityBasisExpectationBuilder) ScopedBy(
	ref ProfileAdmissionPredicateRef,
) AuthorityBasisExpectationBuilder {
	builder.value.permissionPredicateRef = ref
	return builder
}

func (builder AuthorityBasisExpectationBuilder) UnderContextPolicy(
	ref ContextPolicyRef,
	digest Digest,
) AuthorityBasisExpectationBuilder {
	builder.value.contextPolicyRef = ref
	builder.value.contextPolicyDigest = digest
	return builder
}

func (builder AuthorityBasisExpectationBuilder) Build() (AuthorityBasisExpectation, error) {
	if err := validateAuthorityBasisExpectation(builder.value); err != nil {
		return AuthorityBasisExpectation{}, err
	}
	return builder.value, nil
}

type ResolveRequest struct {
	presentationID        PresentationID
	authorityResolutionID AuthorityResolutionID
	basis                 AuthorityBasisExpectation
	envelope              AuthorizationEnvelope
}

type ResolveRequestBuilder struct {
	value ResolveRequest
}

func NewResolveRequestBuilder(
	presentationID PresentationID,
	authorityResolutionID AuthorityResolutionID,
) ResolveRequestBuilder {
	return ResolveRequestBuilder{
		value: ResolveRequest{
			presentationID:        presentationID,
			authorityResolutionID: authorityResolutionID,
		},
	}
}

func (builder ResolveRequestBuilder) ExpectBasis(
	basis AuthorityBasisExpectation,
) ResolveRequestBuilder {
	builder.value.basis = basis
	return builder
}

func (builder ResolveRequestBuilder) ExpectEnvelope(
	envelope AuthorizationEnvelope,
) ResolveRequestBuilder {
	builder.value.envelope = envelope
	return builder
}

func (builder ResolveRequestBuilder) Build() (ResolveRequest, error) {
	if err := validateResolveRequest(builder.value); err != nil {
		return ResolveRequest{}, err
	}
	return builder.value, nil
}

type Presentation struct {
	value canonicalPresentation
}

func (value Presentation) ID() PresentationID {
	return value.value.id
}

func (value Presentation) SpeechActRef() SpeechActRef {
	return value.value.basis.speechActRef
}

func (value Presentation) SpeechActDigest() Digest {
	return value.value.basis.speechActDigest
}

func (value Presentation) AuthorizationContentRef() AuthorizationContentRef {
	return value.value.basis.authorizationContentRef
}

func (value Presentation) AuthorizationContentDigest() Digest {
	return value.value.basis.authorizationContentDigest
}

func (value Presentation) PermissionRef() PermissionRef {
	return value.value.basis.permissionRef
}

func (value Presentation) PermissionDigest() Digest {
	return value.value.basis.permissionDigest
}

func (value Presentation) ProfileAdmissionPredicateRef() ProfileAdmissionPredicateRef {
	return value.value.basis.permissionPredicateRef
}

func (value Presentation) ContextPolicyRef() ContextPolicyRef {
	return value.value.basis.contextPolicyRef
}

func (value Presentation) ContextPolicyDigest() Digest {
	return value.value.basis.contextPolicyDigest
}

func (value Presentation) Envelope() AuthorizationEnvelope {
	return value.value.envelope
}

func (value Presentation) Digest() Digest {
	return value.value.digest
}

func (value Presentation) valid() bool {
	return validateCanonicalPresentation(value.value) == nil
}

type ResolutionKind string

const (
	ResolutionAdmitted    ResolutionKind = "admitted"
	ResolutionNotAdmitted ResolutionKind = "not_admitted"
	ResolutionInvalid     ResolutionKind = "invalid"
)

type DenialCode string

const (
	DenialInvalidRequest               DenialCode = "invalid_request"
	DenialCanonicalRecordMissing       DenialCode = "canonical_record_missing"
	DenialCanonicalRecordInvalid       DenialCode = "canonical_record_invalid"
	DenialSpeechActMismatch            DenialCode = "speech_act_mismatch"
	DenialAuthorizationContentMismatch DenialCode = "authorization_content_mismatch"
	DenialPermissionMismatch           DenialCode = "permission_mismatch"
	DenialContextPolicyMismatch        DenialCode = "context_policy_mismatch"
	DenialEnvelopeMismatch             DenialCode = "authorization_envelope_mismatch"
	DenialOutsideAuthorizationWindow   DenialCode = "outside_authorization_window"
	DenialResolutionNotEffective       DenialCode = "authority_resolution_not_effective"
	DenialResolutionExpired            DenialCode = "authority_resolution_expired"
	DenialSingleUseAlreadyConsumed     DenialCode = "single_use_already_consumed"
)

type Denial struct {
	code   DenialCode
	detail string
}

func (value Denial) Code() DenialCode { return value.code }
func (value Denial) Detail() string   { return value.detail }

type NotAdmitted struct {
	reasons []Denial
}

func (value NotAdmitted) Reasons() []Denial {
	return append([]Denial{}, value.reasons...)
}

func (value NotAdmitted) valid() bool {
	return len(value.reasons) > 0
}

// admittedUse is intentionally package-private. Resolve may carry it as an
// orientation snapshot, but no external caller can construct or name it. A
// later effect boundary must re-load and re-check the canonical resolution in its
// own consumption transaction; it must not trust a Resolve snapshot.
type admittedUse struct {
	presentation Presentation
	resolution   canonicalAuthorityResolution
}

func (value admittedUse) valid() bool {
	return value.presentation.valid() &&
		validateCanonicalAuthorityResolution(value.resolution, value.presentation.value) == nil
}

type Resolution struct {
	admitted    *admittedUse
	notAdmitted *NotAdmitted
}

func (value Resolution) Kind() ResolutionKind {
	if value.admitted != nil && value.admitted.valid() && value.notAdmitted == nil {
		return ResolutionAdmitted
	}
	if value.notAdmitted != nil && value.notAdmitted.valid() && value.admitted == nil {
		return ResolutionNotAdmitted
	}
	return ResolutionInvalid
}

func (value Resolution) Presentation() (Presentation, bool) {
	if value.Kind() != ResolutionAdmitted {
		return Presentation{}, false
	}
	return value.admitted.presentation, true
}

func (value Resolution) AuthorityResolutionID() (AuthorityResolutionID, bool) {
	if value.Kind() != ResolutionAdmitted {
		return AuthorityResolutionID{}, false
	}
	return value.admitted.resolution.id, true
}

func (value Resolution) AuthorityResolutionDigest() (Digest, bool) {
	if value.Kind() != ResolutionAdmitted {
		return Digest{}, false
	}
	return value.admitted.resolution.digest, true
}

func (value Resolution) VerifierIdentity() (VerifierIdentity, bool) {
	if value.Kind() != ResolutionAdmitted {
		return VerifierIdentity{}, false
	}
	return value.admitted.resolution.verifierIdentity, true
}

func (value Resolution) VerifierVersion() (VerifierVersion, bool) {
	if value.Kind() != ResolutionAdmitted {
		return VerifierVersion{}, false
	}
	return value.admitted.resolution.verifierVersion, true
}

func (value Resolution) NotAdmitted() (NotAdmitted, bool) {
	if value.Kind() != ResolutionNotAdmitted {
		return NotAdmitted{}, false
	}
	return *value.notAdmitted, true
}

type KernelGate struct {
	database *sql.DB
}

// OpenKernelGate opens the only production authority gate. It accepts neither
// a caller-supplied verifier nor a clock. This boundary protects the API from
// model/caller fields; it does not claim resistance to hostile same-process
// code with raw write access to the canonical SQLite database.
func OpenKernelGate(database *sql.DB) (*KernelGate, error) {
	if database == nil {
		return nil, fmt.Errorf("authority kernel gate requires a database")
	}
	if err := verifyAuthoritySchema(database); err != nil {
		return nil, err
	}
	return &KernelGate{database: database}, nil
}

// Resolve is read-only. It never consumes the single-use key and never writes
// an audit row. No trustworthy production presentation writer exists in P0PA;
// absent canonical records, resolution fails closed as NotAdmitted.
func (gate *KernelGate) Resolve(
	ctx context.Context,
	request ResolveRequest,
) (Resolution, error) {
	return gate.resolveAt(ctx, request, time.Now().UTC())
}

func (gate *KernelGate) resolveAt(
	ctx context.Context,
	request ResolveRequest,
	now time.Time,
) (Resolution, error) {
	if gate == nil || gate.database == nil {
		return Resolution{}, fmt.Errorf("authority kernel gate is not open")
	}
	return resolveFromCanonicalLedgerAt(
		ctx,
		gate.database,
		request,
		now,
	)
}

type canonicalAuthorityReader interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func resolveFromCanonicalLedgerAt(
	ctx context.Context,
	reader canonicalAuthorityReader,
	request ResolveRequest,
	now time.Time,
) (Resolution, error) {
	if err := validateResolveRequest(request); err != nil {
		return denied(DenialInvalidRequest, err.Error()), nil
	}
	record, err := loadAuthorityRecord(ctx, reader, request)
	if errors.Is(err, sql.ErrNoRows) {
		return denied(DenialCanonicalRecordMissing, "no exact canonical presentation and authority-resolution record exists"), nil
	}
	if err != nil {
		return Resolution{}, fmt.Errorf("load canonical authority record: %w", err)
	}
	presentation, authorityResolution, parseErr := parseAuthorityRecord(record)
	if parseErr != nil {
		return denied(DenialCanonicalRecordInvalid, parseErr.Error()), nil
	}
	reasons := compareAuthorityRecord(request, presentation, authorityResolution, record, now)
	if len(reasons) > 0 {
		return deniedMany(reasons), nil
	}
	witness := admittedUse{
		presentation: Presentation{value: presentation},
		resolution:   authorityResolution,
	}
	return Resolution{admitted: &witness}, nil
}

type canonicalPresentation struct {
	id       PresentationID
	basis    AuthorityBasisExpectation
	envelope AuthorizationEnvelope
	digest   Digest
}

type canonicalAuthorityResolution struct {
	id                       AuthorityResolutionID
	presentationID           PresentationID
	presentationDigest       Digest
	profileAuthorRef         RoleAssignmentRef
	profileAuthorDigest      Digest
	methodDescriptionRef     MethodDescriptionRef
	methodDescriptionDigest  Digest
	methodContractRef        MethodContractRef
	methodContractDigest     Digest
	verifierIdentity         VerifierIdentity
	verifierVersion          VerifierVersion
	verificationPolicyRef    VerificationPolicyRef
	verificationPolicyDigest Digest
	resolvedAt               time.Time
	validUntil               time.Time
	digest                   Digest
}

type authorityRecordRow struct {
	presentationID                    string
	speechActRef                      string
	speechActDigest                   string
	authorizationContentRef           string
	authorizationContentDigest        string
	permissionRef                     string
	permissionDigest                  string
	permissionModality                string
	permissionSourceRef               string
	permissionSubjectRef              string
	permissionContentRef              string
	permissionActionKind              string
	permissionProjectRoot             string
	permissionMethodRef               string
	permissionValidFrom               string
	permissionValidUntil              string
	permissionSingleUseKey            string
	permissionPredicateRef            string
	permissionContextPolicyRef        string
	contextPolicyRef                  string
	contextPolicyDigest               string
	actionKind                        string
	projectRoot                       string
	profileAuthorRef                  string
	profileAuthorDigest               string
	methodDescriptionRef              string
	methodDescriptionDigest           string
	methodContractRef                 string
	methodContractDigest              string
	classifierVersion                 string
	policyVersion                     string
	sessionRef                        string
	allowedWorkFrom                   string
	allowedWorkUntil                  string
	basisObservationFrom              string
	basisObservationUntil             string
	validFrom                         string
	validUntil                        string
	singleUseKey                      string
	projectBindingDigest              string
	envelopeDigest                    string
	presentationDigest                string
	authorityResolutionID             string
	resolutionPresentationDigest      string
	resolutionProfileAuthorRef        string
	resolutionProfileAuthorDigest     string
	resolutionMethodDescriptionRef    string
	resolutionMethodDescriptionDigest string
	resolutionMethodContractRef       string
	resolutionMethodContractDigest    string
	verifierIdentity                  string
	verifierVersion                   string
	verificationPolicyRef             string
	verificationPolicyDigest          string
	resolvedAt                        string
	resolutionValidUntil              string
	authorityResolutionDigest         string
	resolutionUsed                    int
	singleUseKeyUsed                  int
}

func (record *authorityRecordRow) scanTargets() []any {
	return []any{
		&record.presentationID,
		&record.speechActRef,
		&record.speechActDigest,
		&record.authorizationContentRef,
		&record.authorizationContentDigest,
		&record.permissionRef,
		&record.permissionDigest,
		&record.permissionModality,
		&record.permissionSourceRef,
		&record.permissionSubjectRef,
		&record.permissionContentRef,
		&record.permissionActionKind,
		&record.permissionProjectRoot,
		&record.permissionMethodRef,
		&record.permissionValidFrom,
		&record.permissionValidUntil,
		&record.permissionSingleUseKey,
		&record.permissionPredicateRef,
		&record.permissionContextPolicyRef,
		&record.contextPolicyRef,
		&record.contextPolicyDigest,
		&record.actionKind,
		&record.projectRoot,
		&record.profileAuthorRef,
		&record.profileAuthorDigest,
		&record.methodDescriptionRef,
		&record.methodDescriptionDigest,
		&record.methodContractRef,
		&record.methodContractDigest,
		&record.classifierVersion,
		&record.policyVersion,
		&record.sessionRef,
		&record.allowedWorkFrom,
		&record.allowedWorkUntil,
		&record.basisObservationFrom,
		&record.basisObservationUntil,
		&record.validFrom,
		&record.validUntil,
		&record.singleUseKey,
		&record.projectBindingDigest,
		&record.envelopeDigest,
		&record.presentationDigest,
		&record.authorityResolutionID,
		&record.resolutionPresentationDigest,
		&record.resolutionProfileAuthorRef,
		&record.resolutionProfileAuthorDigest,
		&record.resolutionMethodDescriptionRef,
		&record.resolutionMethodDescriptionDigest,
		&record.resolutionMethodContractRef,
		&record.resolutionMethodContractDigest,
		&record.verifierIdentity,
		&record.verifierVersion,
		&record.verificationPolicyRef,
		&record.verificationPolicyDigest,
		&record.resolvedAt,
		&record.resolutionValidUntil,
		&record.authorityResolutionDigest,
		&record.resolutionUsed,
		&record.singleUseKeyUsed,
	}
}

const loadAuthorityRecordSQL = `
SELECT
	p.presentation_id,
	p.speech_act_ref,
	p.speech_act_digest,
	p.authorization_content_ref,
	p.authorization_content_digest,
	p.permission_ref,
	p.permission_digest,
	p.permission_modality,
	p.permission_source_speech_act_ref,
	p.permission_subject_role_assignment_ref,
	p.permission_authorization_content_ref,
	p.permission_action_kind,
	p.permission_project_root,
	p.permission_method_description_ref,
	p.permission_valid_from,
	p.permission_valid_until,
	p.permission_single_use_key,
	p.permission_profile_admission_predicate_ref,
	p.permission_context_policy_ref,
	p.context_policy_ref,
	p.context_policy_digest,
	p.action_kind,
	p.project_root,
	p.profile_author_role_assignment_ref,
	p.profile_author_role_assignment_digest,
	p.method_description_ref,
	p.method_description_digest,
	p.method_contract_ref,
	p.method_contract_digest,
	p.classifier_version,
	p.policy_version,
	p.session_ref,
	p.allowed_work_from,
	p.allowed_work_until,
	p.basis_observation_from,
	p.basis_observation_until,
	p.valid_from,
	p.valid_until,
	p.single_use_key,
	p.project_binding_digest,
	p.envelope_digest,
	p.presentation_digest,
	r.authority_resolution_id,
	r.presentation_digest,
	r.profile_author_role_assignment_ref,
	r.profile_author_role_assignment_digest,
	r.method_description_ref,
	r.method_description_digest,
	r.method_contract_ref,
	r.method_contract_digest,
	r.verifier_identity,
	r.verifier_version,
	r.verification_policy_ref,
	r.verification_policy_digest,
	r.resolved_at,
	r.valid_until,
	r.authority_resolution_digest,
	EXISTS(SELECT 1 FROM authority_uses u WHERE u.authority_resolution_ref = r.authority_resolution_id),
	EXISTS(SELECT 1 FROM authority_uses u WHERE u.single_use_key = p.single_use_key)
FROM authority_presentations p
JOIN authority_resolution_records r ON r.presentation_id = p.presentation_id
WHERE p.presentation_id = ? AND r.authority_resolution_id = ?`

func loadAuthorityRecord(
	ctx context.Context,
	reader canonicalAuthorityReader,
	request ResolveRequest,
) (authorityRecordRow, error) {
	row := reader.QueryRowContext(
		ctx,
		loadAuthorityRecordSQL,
		request.presentationID.String(),
		request.authorityResolutionID.String(),
	)
	record := authorityRecordRow{}
	err := row.Scan(record.scanTargets()...)
	return record, err
}

func parseAuthorityRecord(
	record authorityRecordRow,
) (canonicalPresentation, canonicalAuthorityResolution, error) {
	basis, err := parseAuthorityBasis(record)
	if err != nil {
		return canonicalPresentation{}, canonicalAuthorityResolution{}, err
	}
	envelope, err := parseAuthorizationEnvelope(record)
	if err != nil {
		return canonicalPresentation{}, canonicalAuthorityResolution{}, err
	}
	if err := validatePermissionEnvelope(record, basis, envelope); err != nil {
		return canonicalPresentation{}, canonicalAuthorityResolution{}, err
	}
	presentation, err := parseCanonicalPresentation(record, basis, envelope)
	if err != nil {
		return canonicalPresentation{}, canonicalAuthorityResolution{}, err
	}
	authorityResolution, err := parseCanonicalAuthorityResolution(record)
	if err != nil {
		return canonicalPresentation{}, canonicalAuthorityResolution{}, err
	}
	if err := validateCanonicalAuthorityResolution(authorityResolution, presentation); err != nil {
		return canonicalPresentation{}, canonicalAuthorityResolution{}, err
	}
	return presentation, authorityResolution, nil
}

func parseAuthorityBasis(record authorityRecordRow) (AuthorityBasisExpectation, error) {
	if record.permissionModality != permissionModalityMay {
		return AuthorityBasisExpectation{}, fmt.Errorf("canonical permission modality must be MAY")
	}
	if record.permissionSourceRef != record.speechActRef {
		return AuthorityBasisExpectation{}, fmt.Errorf("canonical permission source must be the exact SpeechAct ref")
	}
	speechActRef, err := NewSpeechActRef(record.speechActRef)
	if err != nil {
		return AuthorityBasisExpectation{}, err
	}
	speechActDigest, err := NewDigest(record.speechActDigest)
	if err != nil {
		return AuthorityBasisExpectation{}, err
	}
	contentRef, err := NewAuthorizationContentRef(record.authorizationContentRef)
	if err != nil {
		return AuthorityBasisExpectation{}, err
	}
	contentDigest, err := NewDigest(record.authorizationContentDigest)
	if err != nil {
		return AuthorityBasisExpectation{}, err
	}
	permissionRef, err := NewPermissionRef(record.permissionRef)
	if err != nil {
		return AuthorityBasisExpectation{}, err
	}
	permissionDigest, err := NewDigest(record.permissionDigest)
	if err != nil {
		return AuthorityBasisExpectation{}, err
	}
	permissionPredicateRef, err := NewProfileAdmissionPredicateRef(record.permissionPredicateRef)
	if err != nil {
		return AuthorityBasisExpectation{}, err
	}
	contextPolicyRef, err := NewContextPolicyRef(record.contextPolicyRef)
	if err != nil {
		return AuthorityBasisExpectation{}, err
	}
	contextPolicyDigest, err := NewDigest(record.contextPolicyDigest)
	if err != nil {
		return AuthorityBasisExpectation{}, err
	}
	return NewAuthorityBasisExpectationBuilder().
		FromSpeechAct(speechActRef, speechActDigest).
		DescribedBy(contentRef, contentDigest).
		InstitutesPermission(permissionRef, permissionDigest).
		ScopedBy(permissionPredicateRef).
		UnderContextPolicy(contextPolicyRef, contextPolicyDigest).
		Build()
}

func validatePermissionEnvelope(
	record authorityRecordRow,
	basis AuthorityBasisExpectation,
	envelope AuthorizationEnvelope,
) error {
	permissionFrom, err := parseAuthorityTime(record.permissionValidFrom)
	if err != nil {
		return err
	}
	permissionUntil, err := parseAuthorityTime(record.permissionValidUntil)
	if err != nil {
		return err
	}
	checks := []struct {
		matches bool
		detail  string
	}{
		{matches: record.permissionSubjectRef == envelope.profileAuthor.String(), detail: "permission subject does not match ProfileAuthor RoleAssignment"},
		{matches: record.permissionContentRef == basis.authorizationContentRef.String(), detail: "permission referent does not match authorization content"},
		{matches: record.permissionActionKind == envelope.actionKind.String(), detail: "permission action does not match authorization content"},
		{matches: record.permissionProjectRoot == envelope.projectRoot.String(), detail: "permission project scope does not match authorization content"},
		{matches: record.permissionMethodRef == envelope.methodDescription.String(), detail: "permission MethodDescription referent does not match authorization content"},
		{matches: !permissionFrom.Before(envelope.authorizationValidityWindow.from), detail: "permission validity starts before authorization content"},
		{matches: permissionFrom.Before(permissionUntil), detail: "permission validity is empty"},
		{matches: permissionUntil.Equal(envelope.authorizationValidityWindow.until), detail: "permission validity end does not match authorization content"},
		{matches: record.permissionSingleUseKey == envelope.singleUseKey.String(), detail: "permission single-use key does not match authorization content"},
		{matches: record.permissionContextPolicyRef == basis.contextPolicyRef.String(), detail: "permission context-policy ref does not match authority basis"},
	}
	mismatchIndex := slices.IndexFunc(checks, func(check struct {
		matches bool
		detail  string
	}) bool {
		return !check.matches
	})
	if mismatchIndex >= 0 {
		return fmt.Errorf("%s", checks[mismatchIndex].detail)
	}
	return nil
}

func parseAuthorizationEnvelope(record authorityRecordRow) (AuthorizationEnvelope, error) {
	actionKind, err := NewActionKind(record.actionKind)
	if err != nil {
		return AuthorizationEnvelope{}, err
	}
	projectRoot, err := NewProjectRoot(record.projectRoot)
	if err != nil {
		return AuthorizationEnvelope{}, err
	}
	profileAuthor, err := NewRoleAssignmentRef(record.profileAuthorRef)
	if err != nil {
		return AuthorizationEnvelope{}, err
	}
	profileAuthorDigest, err := NewDigest(record.profileAuthorDigest)
	if err != nil {
		return AuthorizationEnvelope{}, err
	}
	methodDescription, err := NewMethodDescriptionRef(record.methodDescriptionRef)
	if err != nil {
		return AuthorizationEnvelope{}, err
	}
	methodDescriptionDigest, err := NewDigest(record.methodDescriptionDigest)
	if err != nil {
		return AuthorizationEnvelope{}, err
	}
	methodContract, err := NewMethodContractRef(record.methodContractRef)
	if err != nil {
		return AuthorizationEnvelope{}, err
	}
	methodContractDigest, err := NewDigest(record.methodContractDigest)
	if err != nil {
		return AuthorizationEnvelope{}, err
	}
	classifierVersion, err := NewClassifierVersion(record.classifierVersion)
	if err != nil {
		return AuthorizationEnvelope{}, err
	}
	policyVersion, err := NewPolicyVersion(record.policyVersion)
	if err != nil {
		return AuthorizationEnvelope{}, err
	}
	sessionRef, err := NewSessionRef(record.sessionRef)
	if err != nil {
		return AuthorizationEnvelope{}, err
	}
	workWindow, err := parseTimeWindow(record.allowedWorkFrom, record.allowedWorkUntil)
	if err != nil {
		return AuthorizationEnvelope{}, err
	}
	basisWindow, err := parseTimeWindow(record.basisObservationFrom, record.basisObservationUntil)
	if err != nil {
		return AuthorizationEnvelope{}, err
	}
	validityWindow, err := parseTimeWindow(record.validFrom, record.validUntil)
	if err != nil {
		return AuthorizationEnvelope{}, err
	}
	singleUseKey, err := NewSingleUseKey(record.singleUseKey)
	if err != nil {
		return AuthorizationEnvelope{}, err
	}
	return NewAuthorizationEnvelopeBuilder(actionKind, projectRoot).
		ForProfileAuthor(profileAuthor, profileAuthorDigest).
		ForMethodDescription(methodDescription, methodDescriptionDigest).
		UnderMethodContract(methodContract, methodContractDigest).
		WithClassifier(classifierVersion, policyVersion).
		InSession(sessionRef).
		AllowWorkWithin(workWindow).
		AllowBasisObservationWithin(basisWindow).
		ValidWithin(validityWindow).
		SingleUse(singleUseKey).
		Build()
}

func parseCanonicalPresentation(
	record authorityRecordRow,
	basis AuthorityBasisExpectation,
	envelope AuthorizationEnvelope,
) (canonicalPresentation, error) {
	id, err := NewPresentationID(record.presentationID)
	if err != nil {
		return canonicalPresentation{}, err
	}
	envelopeDigest, err := NewDigest(record.envelopeDigest)
	if err != nil {
		return canonicalPresentation{}, err
	}
	expectedEnvelopeDigest, err := envelope.Digest()
	if err != nil || expectedEnvelopeDigest != envelopeDigest {
		return canonicalPresentation{}, fmt.Errorf("stored envelope digest does not match reconstructed envelope")
	}
	projectBindingDigest, err := NewDigest(record.projectBindingDigest)
	if err != nil {
		return canonicalPresentation{}, err
	}
	expectedProjectBindingDigest, err := ProjectBindingDigest(envelope.actionKind, envelope.projectRoot)
	if err != nil || expectedProjectBindingDigest != projectBindingDigest {
		return canonicalPresentation{}, fmt.Errorf("stored project-binding digest does not match reconstructed action and project")
	}
	digest, err := NewDigest(record.presentationDigest)
	if err != nil {
		return canonicalPresentation{}, err
	}
	presentation := canonicalPresentation{id: id, basis: basis, envelope: envelope, digest: digest}
	if err := validateCanonicalPresentation(presentation); err != nil {
		return canonicalPresentation{}, err
	}
	return presentation, nil
}

func parseCanonicalAuthorityResolution(
	record authorityRecordRow,
) (canonicalAuthorityResolution, error) {
	id, err := NewAuthorityResolutionID(record.authorityResolutionID)
	if err != nil {
		return canonicalAuthorityResolution{}, err
	}
	presentationID, err := NewPresentationID(record.presentationID)
	if err != nil {
		return canonicalAuthorityResolution{}, err
	}
	presentationDigest, err := NewDigest(record.resolutionPresentationDigest)
	if err != nil {
		return canonicalAuthorityResolution{}, err
	}
	profileAuthorRef, err := NewRoleAssignmentRef(record.resolutionProfileAuthorRef)
	if err != nil {
		return canonicalAuthorityResolution{}, err
	}
	profileAuthorDigest, err := NewDigest(record.resolutionProfileAuthorDigest)
	if err != nil {
		return canonicalAuthorityResolution{}, err
	}
	methodDescriptionRef, err := NewMethodDescriptionRef(record.resolutionMethodDescriptionRef)
	if err != nil {
		return canonicalAuthorityResolution{}, err
	}
	methodDescriptionDigest, err := NewDigest(record.resolutionMethodDescriptionDigest)
	if err != nil {
		return canonicalAuthorityResolution{}, err
	}
	methodContractRef, err := NewMethodContractRef(record.resolutionMethodContractRef)
	if err != nil {
		return canonicalAuthorityResolution{}, err
	}
	methodContractDigest, err := NewDigest(record.resolutionMethodContractDigest)
	if err != nil {
		return canonicalAuthorityResolution{}, err
	}
	verifierIdentity, err := NewVerifierIdentity(record.verifierIdentity)
	if err != nil {
		return canonicalAuthorityResolution{}, err
	}
	verifierVersion, err := NewVerifierVersion(record.verifierVersion)
	if err != nil {
		return canonicalAuthorityResolution{}, err
	}
	verificationPolicyRef, err := NewVerificationPolicyRef(record.verificationPolicyRef)
	if err != nil {
		return canonicalAuthorityResolution{}, err
	}
	verificationPolicyDigest, err := NewDigest(record.verificationPolicyDigest)
	if err != nil {
		return canonicalAuthorityResolution{}, err
	}
	resolvedAt, err := parseAuthorityTime(record.resolvedAt)
	if err != nil {
		return canonicalAuthorityResolution{}, err
	}
	validUntil, err := parseAuthorityTime(record.resolutionValidUntil)
	if err != nil {
		return canonicalAuthorityResolution{}, err
	}
	digest, err := NewDigest(record.authorityResolutionDigest)
	if err != nil {
		return canonicalAuthorityResolution{}, err
	}
	return canonicalAuthorityResolution{
		id:                       id,
		presentationID:           presentationID,
		presentationDigest:       presentationDigest,
		profileAuthorRef:         profileAuthorRef,
		profileAuthorDigest:      profileAuthorDigest,
		methodDescriptionRef:     methodDescriptionRef,
		methodDescriptionDigest:  methodDescriptionDigest,
		methodContractRef:        methodContractRef,
		methodContractDigest:     methodContractDigest,
		verifierIdentity:         verifierIdentity,
		verifierVersion:          verifierVersion,
		verificationPolicyRef:    verificationPolicyRef,
		verificationPolicyDigest: verificationPolicyDigest,
		resolvedAt:               resolvedAt,
		validUntil:               validUntil,
		digest:                   digest,
	}, nil
}

func validateAuthorityBasisExpectation(value AuthorityBasisExpectation) error {
	checks := []struct {
		valid  bool
		reason string
	}{
		{valid: value.speechActRef.valid(), reason: "SpeechAct ref is invalid"},
		{valid: value.speechActDigest.valid(), reason: "SpeechAct digest is invalid"},
		{valid: value.authorizationContentRef.valid(), reason: "authorization-content ref is invalid"},
		{valid: value.authorizationContentDigest.valid(), reason: "authorization-content digest is invalid"},
		{valid: value.permissionRef.valid(), reason: "permission ref is invalid"},
		{valid: value.permissionDigest.valid(), reason: "permission digest is invalid"},
		{valid: value.permissionPredicateRef.valid(), reason: "profile-admission predicate ref is invalid"},
		{valid: value.contextPolicyRef.valid(), reason: "context-policy ref is invalid"},
		{valid: value.contextPolicyDigest.valid(), reason: "context-policy digest is invalid"},
	}
	invalidIndex := slices.IndexFunc(checks, func(check struct {
		valid  bool
		reason string
	}) bool {
		return !check.valid
	})
	if invalidIndex >= 0 {
		return fmt.Errorf("%s", checks[invalidIndex].reason)
	}
	return nil
}

func validateResolveRequest(value ResolveRequest) error {
	if !value.presentationID.valid() || !value.authorityResolutionID.valid() {
		return fmt.Errorf("resolve request requires canonical presentation and authority-resolution IDs")
	}
	if err := validateAuthorityBasisExpectation(value.basis); err != nil {
		return err
	}
	return validateAuthorizationEnvelope(value.envelope)
}

func validateCanonicalPresentation(value canonicalPresentation) error {
	if !value.id.valid() {
		return fmt.Errorf("canonical presentation ID is invalid")
	}
	if err := validateAuthorityBasisExpectation(value.basis); err != nil {
		return err
	}
	if err := validateAuthorizationEnvelope(value.envelope); err != nil {
		return err
	}
	expectedDigest, err := presentationDigest(value.id, value.basis, value.envelope)
	if err != nil {
		return err
	}
	if expectedDigest != value.digest {
		return fmt.Errorf("presentation digest does not match canonical fields")
	}
	return nil
}

func validateCanonicalAuthorityResolution(
	value canonicalAuthorityResolution,
	presentation canonicalPresentation,
) error {
	identityValid := value.id.valid() &&
		value.presentationID.valid() &&
		value.profileAuthorRef.valid() &&
		value.profileAuthorDigest.valid() &&
		value.methodDescriptionRef.valid() &&
		value.methodDescriptionDigest.valid() &&
		value.methodContractRef.valid() &&
		value.methodContractDigest.valid() &&
		value.verifierIdentity.valid() &&
		value.verifierVersion.valid() &&
		value.verificationPolicyRef.valid() &&
		value.verificationPolicyDigest.valid()
	if !identityValid {
		return fmt.Errorf("canonical authority-resolution identity is invalid")
	}
	if value.presentationID != presentation.id || value.presentationDigest != presentation.digest {
		return fmt.Errorf("canonical authority resolution does not bind the exact presentation")
	}
	exactEnvelopeBinding := value.profileAuthorRef == presentation.envelope.profileAuthor &&
		value.profileAuthorDigest == presentation.envelope.profileAuthorDigest &&
		value.methodDescriptionRef == presentation.envelope.methodDescription &&
		value.methodDescriptionDigest == presentation.envelope.methodDescriptionDigest &&
		value.methodContractRef == presentation.envelope.methodContract &&
		value.methodContractDigest == presentation.envelope.methodContractDigest
	if !exactEnvelopeBinding {
		return fmt.Errorf("canonical authority resolution does not bind the exact assignment and method contract")
	}
	if value.resolvedAt.IsZero() || !value.validUntil.After(value.resolvedAt) {
		return fmt.Errorf("canonical authority-resolution validity interval is invalid")
	}
	if value.resolvedAt.Before(presentation.envelope.authorizationValidityWindow.from) {
		return fmt.Errorf("canonical authority resolution predates the authorization validity window")
	}
	if value.validUntil.After(presentation.envelope.authorizationValidityWindow.until) {
		return fmt.Errorf("canonical authority resolution exceeds the authorization validity window")
	}
	expectedDigest := authorityResolutionDigest(value)
	if expectedDigest != value.digest {
		return fmt.Errorf("authority-resolution digest does not match canonical fields")
	}
	return nil
}

func compareAuthorityRecord(
	request ResolveRequest,
	presentation canonicalPresentation,
	authorityResolution canonicalAuthorityResolution,
	record authorityRecordRow,
	now time.Time,
) []Denial {
	reasons := make([]Denial, 0, 8)
	reasons = appendMismatch(
		reasons,
		request.basis.speechActRef == presentation.basis.speechActRef &&
			request.basis.speechActDigest == presentation.basis.speechActDigest,
		DenialSpeechActMismatch,
		"SpeechAct ref or digest does not match the canonical presentation",
	)
	reasons = appendMismatch(
		reasons,
		request.basis.authorizationContentRef == presentation.basis.authorizationContentRef &&
			request.basis.authorizationContentDigest == presentation.basis.authorizationContentDigest,
		DenialAuthorizationContentMismatch,
		"authorization-content ref or digest does not match the canonical presentation",
	)
	reasons = appendMismatch(
		reasons,
		request.basis.permissionRef == presentation.basis.permissionRef &&
			request.basis.permissionDigest == presentation.basis.permissionDigest &&
			request.basis.permissionPredicateRef == presentation.basis.permissionPredicateRef,
		DenialPermissionMismatch,
		"permission ref or digest does not match the canonical presentation",
	)
	reasons = appendMismatch(
		reasons,
		request.basis.contextPolicyRef == presentation.basis.contextPolicyRef &&
			request.basis.contextPolicyDigest == presentation.basis.contextPolicyDigest,
		DenialContextPolicyMismatch,
		"context-policy ref or digest does not match the canonical presentation",
	)
	reasons = appendMismatch(
		reasons,
		request.envelope == presentation.envelope,
		DenialEnvelopeMismatch,
		"authorization envelope does not match every canonical field",
	)
	reasons = appendMismatch(
		reasons,
		presentation.envelope.authorizationValidityWindow.Contains(now),
		DenialOutsideAuthorizationWindow,
		"current kernel time is outside the authorization validity window",
	)
	reasons = appendMismatch(
		reasons,
		!now.Before(authorityResolution.resolvedAt),
		DenialResolutionNotEffective,
		"canonical authority resolution is not yet effective",
	)
	reasons = appendMismatch(
		reasons,
		now.Before(authorityResolution.validUntil),
		DenialResolutionExpired,
		"canonical authority resolution is expired",
	)
	reasons = appendMismatch(
		reasons,
		record.resolutionUsed == 0 && record.singleUseKeyUsed == 0,
		DenialSingleUseAlreadyConsumed,
		"authority resolution or its single-use key already has an authority-use record",
	)
	return reasons
}

func appendMismatch(
	reasons []Denial,
	matches bool,
	code DenialCode,
	detail string,
) []Denial {
	if matches {
		return reasons
	}
	return append(reasons, Denial{code: code, detail: detail})
}

func presentationDigest(
	id PresentationID,
	basis AuthorityBasisExpectation,
	envelope AuthorizationEnvelope,
) (Digest, error) {
	if !id.valid() {
		return Digest{}, fmt.Errorf("cannot digest an invalid presentation ID")
	}
	if err := validateAuthorityBasisExpectation(basis); err != nil {
		return Digest{}, err
	}
	envelopeDigest, err := envelope.Digest()
	if err != nil {
		return Digest{}, err
	}
	writer := newAuthorityDigestWriter(authorityPresentationDigestDomain)
	writer.add(id.String())
	writer.add(basis.speechActRef.String())
	writer.add(basis.speechActDigest.String())
	writer.add(basis.authorizationContentRef.String())
	writer.add(basis.authorizationContentDigest.String())
	writer.add(basis.permissionRef.String())
	writer.add(basis.permissionDigest.String())
	writer.add(permissionModalityMay)
	writer.add(basis.speechActRef.String())
	writer.add(envelope.profileAuthor.String())
	writer.add(envelope.profileAuthorDigest.String())
	writer.add(basis.authorizationContentRef.String())
	writer.add(envelope.actionKind.String())
	writer.add(envelope.projectRoot.String())
	writer.add(envelope.methodDescription.String())
	writer.add(envelope.methodDescriptionDigest.String())
	writer.add(envelope.methodContract.String())
	writer.add(envelope.methodContractDigest.String())
	writer.add(formatAuthorityTime(envelope.authorizationValidityWindow.from))
	writer.add(formatAuthorityTime(envelope.authorizationValidityWindow.until))
	writer.add(envelope.singleUseKey.String())
	writer.add(basis.permissionPredicateRef.String())
	writer.add(basis.contextPolicyRef.String())
	writer.add(basis.contextPolicyRef.String())
	writer.add(basis.contextPolicyDigest.String())
	writer.add(envelopeDigest.String())
	return writer.digest(), nil
}

func ProjectBindingDigest(actionKind ActionKind, projectRoot ProjectRoot) (Digest, error) {
	if !actionKind.valid() || !projectRoot.valid() {
		return Digest{}, fmt.Errorf("cannot digest an invalid action/project binding")
	}
	writer := newAuthorityDigestWriter(projectBindingDigestDomain)
	writer.add(actionKind.String())
	writer.add(projectRoot.String())
	return writer.digest(), nil
}

func authorityResolutionDigest(value canonicalAuthorityResolution) Digest {
	writer := newAuthorityDigestWriter(authorityResolutionDigestDomain)
	writer.add(value.id.String())
	writer.add(value.presentationID.String())
	writer.add(value.presentationDigest.String())
	writer.add(value.profileAuthorRef.String())
	writer.add(value.profileAuthorDigest.String())
	writer.add(value.methodDescriptionRef.String())
	writer.add(value.methodDescriptionDigest.String())
	writer.add(value.methodContractRef.String())
	writer.add(value.methodContractDigest.String())
	writer.add(value.verifierIdentity.String())
	writer.add(value.verifierVersion.String())
	writer.add(value.verificationPolicyRef.String())
	writer.add(value.verificationPolicyDigest.String())
	writer.add(formatAuthorityTime(value.resolvedAt))
	writer.add(formatAuthorityTime(value.validUntil))
	return writer.digest()
}

func parseTimeWindow(from string, until string) (TimeWindow, error) {
	parsedFrom, err := parseAuthorityTime(from)
	if err != nil {
		return TimeWindow{}, err
	}
	parsedUntil, err := parseAuthorityTime(until)
	if err != nil {
		return TimeWindow{}, err
	}
	return NewTimeWindow(parsedFrom, parsedUntil)
}

func denied(code DenialCode, detail string) Resolution {
	return deniedMany([]Denial{{code: code, detail: detail}})
}

func deniedMany(reasons []Denial) Resolution {
	copyOfReasons := append([]Denial{}, reasons...)
	value := NotAdmitted{reasons: copyOfReasons}
	return Resolution{notAdmitted: &value}
}

func verifyAuthoritySchema(database *sql.DB) error {
	const migrationQuery = `SELECT COUNT(*) FROM schema_version WHERE version = 34`
	var migrationCount int
	if err := database.QueryRow(migrationQuery).Scan(&migrationCount); err != nil {
		return fmt.Errorf("inspect authority migration version: %w", err)
	}
	if migrationCount != 1 {
		return fmt.Errorf("authority schema migration 34 is unavailable")
	}
	const tableQuery = `
		SELECT COUNT(*)
		FROM sqlite_master
		WHERE type = 'table'
		AND name IN (
			'authority_presentations',
			'authority_resolution_records',
			'authority_uses',
			'profile_onboarding_work_records',
			'project_profile_admissions',
			'project_profile_revisions',
			'project_profile_projection_debt'
		)`
	var tableCount int
	if err := database.QueryRow(tableQuery).Scan(&tableCount); err != nil {
		return fmt.Errorf("inspect authority schema: %w", err)
	}
	if tableCount != 7 {
		return fmt.Errorf("authority schema is unavailable: found %d of 7 canonical tables", tableCount)
	}
	const triggerQuery = `
		SELECT COUNT(*)
		FROM sqlite_master
		WHERE type = 'trigger'
		AND name IN (
			'authority_presentations_no_replace',
			'authority_presentations_exact_assignment_method',
			'authority_presentations_no_update',
			'authority_presentations_no_delete',
			'authority_resolution_records_no_replace',
			'authority_resolution_records_exact_presentation',
			'authority_resolution_records_no_update',
			'authority_resolution_records_no_delete',
			'authority_uses_no_replace',
			'authority_uses_exact_tuple',
			'authority_uses_no_update',
			'authority_uses_no_delete',
			'profile_onboarding_work_records_no_replace',
			'profile_onboarding_work_records_no_update',
			'profile_onboarding_work_records_no_delete',
			'project_profile_admissions_no_replace',
			'project_profile_admissions_revision_cas',
			'project_profile_admissions_exact_authority',
			'project_profile_admissions_no_update',
			'project_profile_admissions_no_delete',
			'project_profile_revisions_no_replace',
			'project_profile_revisions_exact_admission',
			'project_profile_revisions_no_update',
			'project_profile_revisions_no_delete',
			'project_profile_projection_debt_no_replace',
			'project_profile_projection_debt_no_update',
			'project_profile_projection_debt_no_delete'
		)`
	var triggerCount int
	if err := database.QueryRow(triggerQuery).Scan(&triggerCount); err != nil {
		return fmt.Errorf("inspect authority schema triggers: %w", err)
	}
	if triggerCount != 27 {
		return fmt.Errorf("authority schema is incomplete: found %d of 27 required triggers", triggerCount)
	}
	statement, err := database.Prepare(loadAuthorityRecordSQL)
	if err != nil {
		return fmt.Errorf("prepare canonical authority load: %w", err)
	}
	if err := statement.Close(); err != nil {
		return fmt.Errorf("close canonical authority load statement: %w", err)
	}
	return nil
}
