package authority

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

type RecordedAuthorityBasis struct {
	state *recordedAuthorityBasisState
}

type recordedAuthorityBasisState struct {
	source                 RecordedSpeechActSource
	legacy                 recordedAuthority
	projectRoot            ProjectRoot
	presentationID         PresentationID
	presentationDigest     Digest
	authorityResolutionID  AuthorityResolutionID
	authorityResolutionDig Digest
	contentRef             AuthorizationContentRef
	contentDigest          Digest
	profileAuthorRef       RoleAssignmentRef
	permissionRef          PermissionRef
	permissionDigest       Digest
	institutedEffectDigest Digest
	resolvedAt             string
	validUntil             string
}

func (basis RecordedAuthorityBasis) Valid() bool {
	return basis.state != nil &&
		basis.state.source.Valid() &&
		basis.state.legacy.Valid() &&
		basis.state.projectRoot.valid() &&
		basis.state.presentationID.valid() &&
		basis.state.presentationDigest.valid() &&
		basis.state.authorityResolutionID.valid() &&
		basis.state.authorityResolutionDig.valid() &&
		basis.state.contentRef.valid() &&
		basis.state.contentDigest.valid() &&
		basis.state.profileAuthorRef.valid() &&
		basis.state.permissionRef.valid() &&
		basis.state.permissionDigest.valid() &&
		basis.state.institutedEffectDigest.valid()
}

func (basis RecordedAuthorityBasis) ProjectRoot() (ProjectRoot, bool) {
	if !basis.Valid() {
		return ProjectRoot{}, false
	}
	return basis.state.projectRoot, true
}

func (basis RecordedAuthorityBasis) PresentationID() (PresentationID, bool) {
	if !basis.Valid() {
		return PresentationID{}, false
	}
	return basis.state.presentationID, true
}

func (basis RecordedAuthorityBasis) PresentationDigest() (Digest, bool) {
	if !basis.Valid() {
		return Digest{}, false
	}
	return basis.state.presentationDigest, true
}

func (basis RecordedAuthorityBasis) AuthorityResolutionID() (AuthorityResolutionID, bool) {
	if !basis.Valid() {
		return AuthorityResolutionID{}, false
	}
	return basis.state.authorityResolutionID, true
}

func (basis RecordedAuthorityBasis) AuthorityResolutionDigest() (Digest, bool) {
	if !basis.Valid() {
		return Digest{}, false
	}
	return basis.state.authorityResolutionDig, true
}

func (basis RecordedAuthorityBasis) AuthorizationContentRef() (AuthorizationContentRef, bool) {
	if !basis.Valid() {
		return AuthorizationContentRef{}, false
	}
	return basis.state.contentRef, true
}

func (basis RecordedAuthorityBasis) AuthorizationContentDigest() (Digest, bool) {
	if !basis.Valid() {
		return Digest{}, false
	}
	return basis.state.contentDigest, true
}

func (basis RecordedAuthorityBasis) ProfileAuthorRoleAssignmentRef() (RoleAssignmentRef, bool) {
	if !basis.Valid() {
		return RoleAssignmentRef{}, false
	}
	return basis.state.profileAuthorRef, true
}

type authorityBasisRow struct {
	resolutionID              string
	resolutionDigest          string
	resolutionProjectRoot     string
	resolutionPresentationID  string
	resolutionPresentationDig string
	resolutionResolvedAt      string
	resolutionValidUntil      string
	resolutionLegacyDigest    string
	resolutionJSON            string
	presentationID            string
	presentationDigest        string
	presentationLegacyDigest  string
	presentationJSON          string
	contentRef                string
	contentDigest             string
	contentProfileAuthorRef   string
	contentJSON               string
	speechActRef              string
	speechActDigest           string
	permissionRef             string
	permissionDigest          string
	permissionJSON            string
	effectDigest              string
	effectJSON                string
}

func (row *authorityBasisRow) scanTargets() []any {
	return []any{
		&row.resolutionID,
		&row.resolutionDigest,
		&row.resolutionProjectRoot,
		&row.resolutionPresentationID,
		&row.resolutionPresentationDig,
		&row.resolutionResolvedAt,
		&row.resolutionValidUntil,
		&row.resolutionLegacyDigest,
		&row.resolutionJSON,
		&row.presentationID,
		&row.presentationDigest,
		&row.presentationLegacyDigest,
		&row.presentationJSON,
		&row.contentRef,
		&row.contentDigest,
		&row.contentProfileAuthorRef,
		&row.contentJSON,
		&row.speechActRef,
		&row.speechActDigest,
		&row.permissionRef,
		&row.permissionDigest,
		&row.permissionJSON,
		&row.effectDigest,
		&row.effectJSON,
	}
}

const loadAuthorityBasisSQL = `
SELECT
	resolution.authority_resolution_id,
	resolution.authority_resolution_digest,
	resolution.project_root,
	resolution.presentation_id,
	resolution.presentation_digest,
	resolution.resolved_at,
	resolution.valid_until,
	resolution.legacy_projection_digest,
	resolution.canonical_json,
	presentation.presentation_id,
	presentation.presentation_digest,
	presentation.legacy_projection_digest,
	presentation.canonical_json,
	content.authorization_content_ref,
	content.authorization_content_digest,
	content.profile_author_role_assignment_ref,
	content.canonical_json,
	act.speech_act_ref,
	act.speech_act_digest,
	permission.permission_ref,
	permission.permission_digest,
	permission.canonical_json,
	effect.instituted_effect_digest,
	effect.canonical_json
FROM authority_basis_resolutions resolution
JOIN authority_basis_presentations presentation
	ON presentation.presentation_id = resolution.presentation_id
	AND presentation.presentation_digest = resolution.presentation_digest
	AND presentation.project_root = resolution.project_root
JOIN profile_declaration_authorization_contents content
	ON content.authorization_content_ref = presentation.authorization_content_ref
	AND content.authorization_content_digest = presentation.authorization_content_digest
	AND content.project_root = resolution.project_root
JOIN speech_acts act
	ON act.speech_act_ref = presentation.speech_act_ref
	AND act.speech_act_digest = presentation.speech_act_digest
	AND act.project_root = resolution.project_root
JOIN profile_declaration_permissions permission
	ON permission.permission_ref = presentation.permission_ref
	AND permission.permission_digest = presentation.permission_digest
	AND permission.source_speech_act_ref = act.speech_act_ref
	AND permission.project_root = resolution.project_root
JOIN speech_act_instituted_effects effect
	ON effect.instituted_effect_digest = presentation.instituted_effect_digest
	AND effect.speech_act_ref = act.speech_act_ref
	AND effect.speech_act_digest = act.speech_act_digest
	AND effect.permission_ref = permission.permission_ref
	AND effect.permission_digest = permission.permission_digest
	AND effect.project_root = resolution.project_root
WHERE resolution.authority_resolution_id = ?
AND resolution.authority_resolution_digest = ?`

func LoadRecordedAuthorityBasisByResolution(
	ctx context.Context,
	database *sql.DB,
	id AuthorityResolutionID,
	digest Digest,
) (RecordedAuthorityBasis, error) {
	if ctx == nil || database == nil || !id.valid() || !digest.valid() {
		return RecordedAuthorityBasis{}, fmt.Errorf("authority basis load requires canonical arguments")
	}
	row := authorityBasisRow{}
	err := database.QueryRowContext(
		ctx,
		loadAuthorityBasisSQL,
		id.String(),
		digest.String(),
	).Scan(row.scanTargets()...)
	if err != nil {
		return RecordedAuthorityBasis{}, fmt.Errorf("load authority basis: %w", err)
	}
	return reconstructRecordedAuthorityBasis(ctx, database, row)
}

// LoadProfileDeclarationResolveRequest is the only durable-basis conversion
// into the profile-declaration gate request. The generic recorded basis stays
// audit-only; this function first performs the complete typed v38 replay and
// then verifies the sealed profile authority MethodDescription and effect
// policy before returning the compatibility request.
func LoadProfileDeclarationResolveRequest(
	ctx context.Context,
	database *sql.DB,
	id AuthorityResolutionID,
	digest Digest,
) (ResolveRequest, error) {
	basis, err := LoadRecordedAuthorityBasisByResolution(ctx, database, id, digest)
	if err != nil {
		return ResolveRequest{}, err
	}
	return profileDeclarationResolveRequestFromBasis(basis)
}

func profileDeclarationResolveRequestFromBasis(
	basis RecordedAuthorityBasis,
) (ResolveRequest, error) {
	if !basis.Valid() {
		return ResolveRequest{}, fmt.Errorf("profile declaration requires a strict recorded authority basis")
	}
	state := basis.state
	source := state.source.state
	method := source.intent.state.executionFrame.state.methodDescription
	manualMethod := ManualAuthorityIssueMethodDescription()
	policy := source.intent.state.contextPolicy.state
	content := state.legacy.state.presentation.envelope
	legacyUtterance := policy.effectRule.ref.String() == profileDeclarationEffectRuleValue &&
		policy.effectRule.utteranceRule == AuthorizeReviewedIntentUtteranceRule() &&
		policy.effectRule.utteranceDescription == profileAuthorityUtteranceDescription()
	humanUtterance := policy.effectRule.utteranceRule.binding == utteranceBindsLiteral &&
		policy.effectRule.utteranceRule.verb == "AUTHORIZE"
	exactProfileSemantics := method.valid() &&
		manualMethod.valid() &&
		method.state.digest == manualMethod.state.digest &&
		policy.boundedContext == manualMethod.state.boundedContext &&
		policy.recognizedActType.String() == "speech-act-type:authorize" &&
		policy.effectRule.institutedObjectKind.String() == "U.Commitment" &&
		policy.effectRule.modality.String() == permissionModalityMay &&
		policy.effectRule.scopedAction == ProfileDeclarationActionKind() &&
		(legacyUtterance || humanUtterance) &&
		content.actionKind == ProfileDeclarationActionKind()
	if !exactProfileSemantics {
		return ResolveRequest{}, fmt.Errorf("recorded authority basis is not the sealed profile-declaration protocol")
	}
	request, ok := state.legacy.resolveRequest()
	if !ok || request.presentationID != state.presentationID ||
		request.authorityResolutionID != state.authorityResolutionID {
		return ResolveRequest{}, fmt.Errorf("recorded authority basis omitted its exact profile gate request")
	}
	return request, nil
}

func loadProfileDeclarationResolveRequestInTransaction(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	id AuthorityResolutionID,
) (ResolveRequest, bool, error) {
	var rawDigest string
	err := transaction.ScanOne(
		ctx,
		"SELECT authority_resolution_digest FROM authority_basis_resolutions WHERE authority_resolution_id = ?",
		[]any{id.String()},
		[]any{&rawDigest},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ResolveRequest{}, false, nil
	}
	if err != nil {
		return ResolveRequest{}, false, err
	}
	digest, err := NewDigest(rawDigest)
	if err != nil {
		return ResolveRequest{}, true, err
	}
	basis, found, err := loadRecordedAuthorityBasisInTransaction(
		ctx,
		transaction,
		id,
		digest,
	)
	if err != nil || !found {
		return ResolveRequest{}, found, err
	}
	request, err := profileDeclarationResolveRequestFromBasis(basis)
	return request, true, err
}

func reconstructRecordedAuthorityBasis(
	ctx context.Context,
	database *sql.DB,
	row authorityBasisRow,
) (RecordedAuthorityBasis, error) {
	if err := validateAuthorityBasisRowDigests(row); err != nil {
		return RecordedAuthorityBasis{}, err
	}
	identity, err := parseAuthorityBasisRowIdentity(row)
	if err != nil {
		return RecordedAuthorityBasis{}, err
	}
	source, err := LoadRecordedSpeechActSource(
		ctx,
		database,
		identity.speechActRef,
		identity.speechActDigest,
	)
	if err != nil {
		return RecordedAuthorityBasis{}, err
	}
	legacy, err := loadRecordedAuthorityByResolution(
		ctx,
		database,
		identity.resolutionID,
		identity.resolutionLegacyDigest,
	)
	if err != nil {
		return RecordedAuthorityBasis{}, fmt.Errorf("load legacy compatibility projection: %w", err)
	}
	legacyPresentation, ok := legacy.Presentation()
	if !ok || legacyPresentation.value.digest.String() != row.presentationLegacyDigest {
		return RecordedAuthorityBasis{}, fmt.Errorf("legacy presentation projection does not match v38 basis")
	}
	return buildRecordedAuthorityBasis(row, source, legacy)
}

func loadRecordedAuthorityBasisInTransaction(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	id AuthorityResolutionID,
	digest Digest,
) (RecordedAuthorityBasis, bool, error) {
	row := authorityBasisRow{}
	err := transaction.ScanOne(
		ctx,
		loadAuthorityBasisSQL,
		[]any{id.String(), digest.String()},
		row.scanTargets(),
	)
	if errors.Is(err, sql.ErrNoRows) {
		return RecordedAuthorityBasis{}, false, nil
	}
	if err != nil {
		return RecordedAuthorityBasis{}, false, err
	}
	if err := validateAuthorityBasisRowDigests(row); err != nil {
		return RecordedAuthorityBasis{}, true, err
	}
	identity, err := parseAuthorityBasisRowIdentity(row)
	if err != nil {
		return RecordedAuthorityBasis{}, true, err
	}
	source, found, err := loadRecordedSpeechActSourceInTransaction(
		ctx,
		transaction,
		identity.speechActRef,
		identity.speechActDigest,
	)
	if err != nil || !found {
		return RecordedAuthorityBasis{}, true, errors.Join(
			fmt.Errorf("authority basis SpeechAct source is missing"),
			err,
		)
	}
	legacy, found, err := loadRecordedAuthorityInTransaction(
		ctx,
		transaction,
		identity.presentationID,
		identity.resolutionID,
	)
	if err != nil || !found {
		return RecordedAuthorityBasis{}, true, errors.Join(
			fmt.Errorf("authority basis legacy projection is missing"),
			err,
		)
	}
	basis, err := buildRecordedAuthorityBasis(row, source, legacy)
	return basis, true, err
}

func buildRecordedAuthorityBasis(
	row authorityBasisRow,
	source RecordedSpeechActSource,
	legacy recordedAuthority,
) (RecordedAuthorityBasis, error) {
	checked, err := rebuildCheckedAuthorityBasis(row, source, legacy)
	if err != nil {
		return RecordedAuthorityBasis{}, err
	}
	identity, err := parseAuthorityBasisRowIdentity(row)
	if err != nil {
		return RecordedAuthorityBasis{}, err
	}
	state := recordedAuthorityBasisState{
		source:                 source,
		legacy:                 legacy,
		projectRoot:            identity.projectRoot,
		presentationID:         identity.presentationID,
		presentationDigest:     identity.presentationDigest,
		authorityResolutionID:  identity.resolutionID,
		authorityResolutionDig: identity.resolutionDigest,
		contentRef:             identity.contentRef,
		contentDigest:          identity.contentDigest,
		profileAuthorRef:       identity.profileAuthorRef,
		permissionRef:          identity.permissionRef,
		permissionDigest:       identity.permissionDigest,
		institutedEffectDigest: identity.effectDigest,
		resolvedAt:             row.resolutionResolvedAt,
		validUntil:             row.resolutionValidUntil,
	}
	basis := RecordedAuthorityBasis{state: &state}
	if !basis.Valid() || !recordedAuthorityBasisMatchesChecked(basis, checked) {
		return RecordedAuthorityBasis{}, fmt.Errorf("reconstructed authority basis is invalid")
	}
	return basis, nil
}

type authorityBasisRowIdentity struct {
	projectRoot            ProjectRoot
	presentationID         PresentationID
	presentationDigest     Digest
	resolutionID           AuthorityResolutionID
	resolutionDigest       Digest
	resolutionLegacyDigest Digest
	contentRef             AuthorizationContentRef
	contentDigest          Digest
	profileAuthorRef       RoleAssignmentRef
	speechActRef           SpeechActRef
	speechActDigest        Digest
	permissionRef          PermissionRef
	permissionDigest       Digest
	effectDigest           Digest
}

func parseAuthorityBasisRowIdentity(row authorityBasisRow) (authorityBasisRowIdentity, error) {
	projectRoot, err := NewProjectRoot(row.resolutionProjectRoot)
	if err != nil {
		return authorityBasisRowIdentity{}, err
	}
	presentationID, err := NewPresentationID(row.presentationID)
	if err != nil {
		return authorityBasisRowIdentity{}, err
	}
	presentationDigest, err := NewDigest(row.presentationDigest)
	if err != nil {
		return authorityBasisRowIdentity{}, err
	}
	resolutionID, err := NewAuthorityResolutionID(row.resolutionID)
	if err != nil {
		return authorityBasisRowIdentity{}, err
	}
	resolutionDigest, err := NewDigest(row.resolutionDigest)
	if err != nil {
		return authorityBasisRowIdentity{}, err
	}
	resolutionLegacyDigest, err := NewDigest(row.resolutionLegacyDigest)
	if err != nil {
		return authorityBasisRowIdentity{}, err
	}
	contentRef, err := NewAuthorizationContentRef(row.contentRef)
	if err != nil {
		return authorityBasisRowIdentity{}, err
	}
	contentDigest, err := NewDigest(row.contentDigest)
	if err != nil {
		return authorityBasisRowIdentity{}, err
	}
	profileAuthorRef, err := NewRoleAssignmentRef(row.contentProfileAuthorRef)
	if err != nil {
		return authorityBasisRowIdentity{}, err
	}
	speechActRef, err := NewSpeechActRef(row.speechActRef)
	if err != nil {
		return authorityBasisRowIdentity{}, err
	}
	speechActDigest, err := NewDigest(row.speechActDigest)
	if err != nil {
		return authorityBasisRowIdentity{}, err
	}
	permissionRef, err := NewPermissionRef(row.permissionRef)
	if err != nil {
		return authorityBasisRowIdentity{}, err
	}
	permissionDigest, err := NewDigest(row.permissionDigest)
	if err != nil {
		return authorityBasisRowIdentity{}, err
	}
	effectDigest, err := NewDigest(row.effectDigest)
	if err != nil {
		return authorityBasisRowIdentity{}, err
	}
	return authorityBasisRowIdentity{
		projectRoot:            projectRoot,
		presentationID:         presentationID,
		presentationDigest:     presentationDigest,
		resolutionID:           resolutionID,
		resolutionDigest:       resolutionDigest,
		resolutionLegacyDigest: resolutionLegacyDigest,
		contentRef:             contentRef,
		contentDigest:          contentDigest,
		profileAuthorRef:       profileAuthorRef,
		speechActRef:           speechActRef,
		speechActDigest:        speechActDigest,
		permissionRef:          permissionRef,
		permissionDigest:       permissionDigest,
		effectDigest:           effectDigest,
	}, nil
}

type storedProfilePermissionProjection struct {
	Schema                   string   `json:"schema"`
	Ref                      string   `json:"permission_ref"`
	AdmissionPredicateRef    string   `json:"profile_admission_predicate_ref"`
	ClaimScopeRef            string   `json:"claim_scope_ref"`
	Referents                []string `json:"referents"`
	EvidenceClaimRefs        []string `json:"adjudication_evidence_claim_refs"`
	CarrierRefs              []string `json:"adjudication_carrier_refs"`
	VerifierIdentity         string   `json:"adjudication_verifier_identity"`
	VerifierVersion          string   `json:"adjudication_verifier_version"`
	VerificationPolicyRef    string   `json:"adjudication_verification_policy_ref"`
	VerificationPolicyDigest string   `json:"adjudication_verification_policy_digest"`
	EvaluationPolicyRef      string   `json:"adjudication_evaluation_policy_ref"`
	EvaluationPolicyDigest   string   `json:"adjudication_evaluation_policy_digest"`
	EvidenceRelationRef      string   `json:"adjudication_evidence_relation_ref"`
	CarrierExpectationRef    string   `json:"adjudication_carrier_expectation_ref"`
}

type storedBasisResolutionProjection struct {
	Schema                   string `json:"schema"`
	ResolutionID             string `json:"authority_resolution_id"`
	ProjectRoot              string `json:"project_root"`
	PresentationID           string `json:"presentation_id"`
	PresentationDigest       string `json:"presentation_digest"`
	VerifierIdentity         string `json:"verifier_identity"`
	VerifierVersion          string `json:"verifier_version"`
	VerificationPolicyRef    string `json:"verification_policy_ref"`
	VerificationPolicyDigest string `json:"verification_policy_digest"`
	ResolvedAt               string `json:"resolved_at"`
	AuthorityValidFrom       string `json:"authority_valid_from"`
	ValidUntil               string `json:"valid_until"`
	LegacyProjectionDigest   string `json:"legacy_projection_digest"`
}

func rebuildCheckedAuthorityBasis(
	row authorityBasisRow,
	source RecordedSpeechActSource,
	legacy recordedAuthority,
) (checkedAuthorityBasis, error) {
	permissionProjection := storedProfilePermissionProjection{}
	if err := json.Unmarshal([]byte(row.permissionJSON), &permissionProjection); err != nil {
		return checkedAuthorityBasis{}, fmt.Errorf("decode profile permission: %w", err)
	}
	resolutionProjection := storedBasisResolutionProjection{}
	if err := json.Unmarshal([]byte(row.resolutionJSON), &resolutionProjection); err != nil {
		return checkedAuthorityBasis{}, fmt.Errorf("decode authority basis resolution: %w", err)
	}
	legacyPresentation, ok := legacy.Presentation()
	if !ok {
		return checkedAuthorityBasis{}, fmt.Errorf("legacy presentation projection is invalid")
	}
	contentRef, err := NewAuthorizationContentRef(row.contentRef)
	if err != nil {
		return checkedAuthorityBasis{}, err
	}
	content, err := NewProfileDeclarationAuthorizationContent(
		contentRef,
		legacyPresentation.value.envelope,
	)
	if err != nil {
		return checkedAuthorityBasis{}, err
	}
	if content.state.digest.String() != row.contentDigest ||
		string(content.state.canonicalJSON) != row.contentJSON {
		return checkedAuthorityBasis{}, fmt.Errorf("authorization content is not exact canonical material")
	}
	authorityValidFrom, err := parseAuthorityTime(resolutionProjection.AuthorityValidFrom)
	if err != nil {
		return checkedAuthorityBasis{}, err
	}
	validUntil, err := parseAuthorityTime(resolutionProjection.ValidUntil)
	if err != nil {
		return checkedAuthorityBasis{}, err
	}
	resolutionWindow, err := NewTimeWindow(authorityValidFrom, validUntil)
	if err != nil {
		return checkedAuthorityBasis{}, err
	}
	intent, err := rebuildPreparedAuthorityIntent(
		row,
		source,
		content,
		permissionProjection,
		resolutionProjection,
		resolutionWindow,
	)
	if err != nil {
		return checkedAuthorityBasis{}, err
	}
	durableSource := VerifiedSpeechActSource(source)
	act, err := completeVerifiedAuthorityAct(intent, durableSource)
	if err != nil {
		return checkedAuthorityBasis{}, err
	}
	resolvedAt, err := parseAuthorityTime(resolutionProjection.ResolvedAt)
	if err != nil {
		return checkedAuthorityBasis{}, err
	}
	checked, err := newCheckedAuthorityBasis(act, resolvedAt)
	if err != nil {
		return checkedAuthorityBasis{}, err
	}
	exact := string(act.state.permission.state.canonicalJSON) == row.permissionJSON &&
		act.state.permission.state.digest.String() == row.permissionDigest &&
		string(act.state.effect.state.canonicalJSON) == row.effectJSON &&
		act.state.effect.state.digest.String() == row.effectDigest &&
		string(checked.presentation.canonicalJSON) == row.presentationJSON &&
		checked.presentation.digest.String() == row.presentationDigest &&
		string(checked.resolution.canonicalJSON) == row.resolutionJSON &&
		checked.resolution.digest.String() == row.resolutionDigest &&
		checked.legacyPresentation == legacy.state.presentation &&
		checked.legacyResolution == legacy.state.resolution
	if !exact {
		return checkedAuthorityBasis{}, fmt.Errorf("authority basis failed exact typed canonical reconstruction")
	}
	return checked, nil
}

func rebuildPreparedAuthorityIntent(
	row authorityBasisRow,
	source RecordedSpeechActSource,
	content ProfileDeclarationAuthorizationContent,
	permission storedProfilePermissionProjection,
	resolution storedBasisResolutionProjection,
	resolutionWindow TimeWindow,
) (PreparedAuthorityIntent, error) {
	presentationID, err := NewPresentationID(row.presentationID)
	if err != nil {
		return PreparedAuthorityIntent{}, err
	}
	resolutionID, err := NewAuthorityResolutionID(row.resolutionID)
	if err != nil {
		return PreparedAuthorityIntent{}, err
	}
	permissionRef, err := NewPermissionRef(permission.Ref)
	if err != nil {
		return PreparedAuthorityIntent{}, err
	}
	predicateRef, err := NewProfileAdmissionPredicateRef(permission.AdmissionPredicateRef)
	if err != nil {
		return PreparedAuthorityIntent{}, err
	}
	claimScopeRef, err := NewClaimScopeRef(permission.ClaimScopeRef)
	if err != nil {
		return PreparedAuthorityIntent{}, err
	}
	verifierIdentity, err := NewVerifierIdentity(resolution.VerifierIdentity)
	if err != nil {
		return PreparedAuthorityIntent{}, err
	}
	verifierVersion, err := NewVerifierVersion(resolution.VerifierVersion)
	if err != nil {
		return PreparedAuthorityIntent{}, err
	}
	verificationPolicyRef, err := NewVerificationPolicyRef(resolution.VerificationPolicyRef)
	if err != nil {
		return PreparedAuthorityIntent{}, err
	}
	verificationPolicyDigest, err := NewDigest(resolution.VerificationPolicyDigest)
	if err != nil {
		return PreparedAuthorityIntent{}, err
	}
	evidenceRelationRef, err := NewVerificationEvidenceRelationRef(permission.EvidenceRelationRef)
	if err != nil {
		return PreparedAuthorityIntent{}, err
	}
	carrierExpectationRef, err := NewVerificationCarrierExpectationRef(permission.CarrierExpectationRef)
	if err != nil {
		return PreparedAuthorityIntent{}, err
	}
	sourceState := source.state
	intent, err := NewPreparedAuthorityIntentBuilder(
		presentationID,
		resolutionID,
		sourceState.speechAct.state.ref,
		permissionRef,
		sourceState.capture.state.carrierRef,
	).
		WithAuthorizationContent(content).
		InSpeechActSession(sourceState.intent.state.sessionRef).
		UnderContextPolicy(sourceState.intent.state.contextPolicy).
		WithSpeechActExecutionFrame(sourceState.intent.state.executionFrame).
		ScopedBy(predicateRef).
		WithinClaimScope(claimScopeRef).
		VerifiedBy(verifierIdentity, verifierVersion).
		UnderVerificationPolicy(verificationPolicyRef, verificationPolicyDigest).
		WithAdjudicationEvidence(evidenceRelationRef, carrierExpectationRef).
		ResolutionEffectiveWithin(resolutionWindow).
		Build()
	if err != nil {
		return PreparedAuthorityIntent{}, err
	}
	if permission.VerifierIdentity != resolution.VerifierIdentity ||
		permission.VerifierVersion != resolution.VerifierVersion ||
		permission.VerificationPolicyRef != resolution.VerificationPolicyRef ||
		permission.VerificationPolicyDigest != resolution.VerificationPolicyDigest ||
		permission.EvaluationPolicyRef != resolution.VerificationPolicyRef ||
		permission.EvaluationPolicyDigest != resolution.VerificationPolicyDigest {
		return PreparedAuthorityIntent{}, fmt.Errorf("permission adjudication does not match basis resolution policy")
	}
	return intent, nil
}

func validateAuthorityBasisRowDigests(row authorityBasisRow) error {
	checks := []struct {
		domain string
		raw    string
		digest string
	}{
		{authorityBasisResolutionDigestDomain, row.resolutionJSON, row.resolutionDigest},
		{authorityBasisPresentationDigestDomain, row.presentationJSON, row.presentationDigest},
		{authorizationContentDigestDomain, row.contentJSON, row.contentDigest},
		{authorityPermissionDigestDomain, row.permissionJSON, row.permissionDigest},
		{institutedEffectDigestDomain, row.effectJSON, row.effectDigest},
	}
	return validateAuthorityBasisDigestChecks(checks, 0)
}

func validateAuthorityBasisDigestChecks(
	checks []struct {
		domain string
		raw    string
		digest string
	},
	index int,
) error {
	if index >= len(checks) {
		return nil
	}
	check := checks[index]
	if !json.Valid([]byte(check.raw)) {
		return fmt.Errorf("authority basis contains invalid canonical JSON")
	}
	writer := newAuthorityDigestWriter(check.domain)
	writer.add(check.raw)
	if writer.digest().String() != check.digest {
		return fmt.Errorf("authority basis canonical digest mismatch")
	}
	return validateAuthorityBasisDigestChecks(checks, index+1)
}
