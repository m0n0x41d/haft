package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/profileauthority"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

const selectPermissionSQL = `SELECT
	permission_ref, permission_digest, prepared_authorization_digest,
	project_root, permission_kind, subject_ref, subject_digest, modality,
	action_kind, claim_scope_ref, bounded_context_ref, valid_from, valid_until,
	referents_json, authorization_content_ref, authorization_content_digest,
	method_description_ref, method_description_digest, enactability_predicate_ref,
	adjudication_evidence_claim_refs_json,
	adjudication_carrier_class_refs_json, adjudication_verifier_identity,
	adjudication_verifier_version, adjudication_evaluation_policy_ref,
	adjudication_evaluation_policy_digest, source_speech_act_ref,
	source_speech_act_digest, context_policy_ref, context_policy_digest,
	capture_carrier_ref, capture_carrier_digest, canonical_json, recorded_at
FROM profile_declaration_permissions_v2`

const selectEffectSQL = `SELECT
	effect_digest, project_root, speech_act_ref, speech_act_digest,
	permission_ref, permission_digest, canonical_json, recorded_at
FROM profile_declaration_instituted_effects_v2`

const selectBasisSQL = `SELECT
	basis_ref, basis_digest, project_root, speech_act_ref, speech_act_digest,
	authorization_content_ref, authorization_content_digest, permission_ref,
	permission_digest, context_policy_ref, context_policy_digest,
	instituted_effect_digest, canonical_json, recorded_at
FROM profile_declaration_authority_bases_v2`

type exactClosureSources struct {
	prepared profileauthority.PreparedAuthorization
	source   authority.RecordedSpeechActSource
	policy   authority.SpeechActContextPolicy
}

func LoadPermission(
	ctx context.Context,
	database *sql.DB,
	ref authority.PermissionRef,
	digest authority.Digest,
) (profileauthority.Permission, error) {
	row, err := readPermissionRow(ctx, database, ref, digest)
	if err != nil {
		return profileauthority.Permission{}, err
	}
	sources, err := resolveExactClosureSources(
		ctx,
		database,
		row.preparedDigest,
		row.sourceSpeechActRef,
		row.sourceSpeechActDigest,
		row.contextPolicyRef,
		row.contextPolicyDigest,
	)
	if err != nil {
		return profileauthority.Permission{}, err
	}
	return reconstructPermission(row, sources)
}

func LoadInstitutedEffect(
	ctx context.Context,
	database *sql.DB,
	digest authority.Digest,
) (profileauthority.InstitutedEffect, error) {
	row, err := readEffectRow(ctx, database, digest)
	if err != nil {
		return profileauthority.InstitutedEffect{}, err
	}
	permissionRef, err := authority.NewPermissionRef(row.permissionRef)
	if err != nil {
		return profileauthority.InstitutedEffect{}, fmt.Errorf("parse effect Permission ref: %w", err)
	}
	permissionDigest, err := authority.NewDigest(row.permissionDigest)
	if err != nil {
		return profileauthority.InstitutedEffect{}, fmt.Errorf("parse effect Permission digest: %w", err)
	}
	permission, err := LoadPermission(ctx, database, permissionRef, permissionDigest)
	if err != nil {
		return profileauthority.InstitutedEffect{}, fmt.Errorf("load effect Permission: %w", err)
	}
	return reconstructEffect(row, permission)
}

func LoadFourRefBasis(
	ctx context.Context,
	database *sql.DB,
	ref profileauthority.BasisRef,
	digest authority.Digest,
) (profileauthority.FourRefBasis, error) {
	row, err := readBasisRow(ctx, database, ref, digest)
	if err != nil {
		return profileauthority.FourRefBasis{}, err
	}
	bundle, err := loadClosureBundle(ctx, database, row)
	if err != nil {
		return profileauthority.FourRefBasis{}, err
	}
	return bundle.basis, nil
}

func LoadClosure(
	ctx context.Context,
	database *sql.DB,
	ref profileauthority.BasisRef,
	digest authority.Digest,
) (profileauthority.Closure, error) {
	row, err := readBasisRow(ctx, database, ref, digest)
	if err != nil {
		return profileauthority.Closure{}, err
	}
	bundle, err := loadClosureBundle(ctx, database, row)
	if err != nil {
		return profileauthority.Closure{}, err
	}
	return bundle.closure, nil
}

type closureBundle struct {
	closure profileauthority.Closure
	basis   profileauthority.FourRefBasis
	sources exactClosureSources
}

func loadClosureBundle(
	ctx context.Context,
	database *sql.DB,
	basisRow basisRow,
) (closureBundle, error) {
	prepared, err := loadPreparedForContent(ctx, database, basisRow.contentRef, basisRow.contentDigest)
	if err != nil {
		return closureBundle{}, err
	}
	sources, err := resolveExactClosureSources(
		ctx,
		database,
		mustDigestString(prepared),
		basisRow.speechActRef,
		basisRow.speechActDigest,
		basisRow.contextPolicyRef,
		basisRow.contextPolicyDigest,
	)
	if err != nil {
		return closureBundle{}, err
	}
	permissionRef, err := authority.NewPermissionRef(basisRow.permissionRef)
	if err != nil {
		return closureBundle{}, fmt.Errorf("parse basis Permission ref: %w", err)
	}
	permissionDigest, err := authority.NewDigest(basisRow.permissionDigest)
	if err != nil {
		return closureBundle{}, fmt.Errorf("parse basis Permission digest: %w", err)
	}
	permissionRow, err := readPermissionRow(ctx, database, permissionRef, permissionDigest)
	if err != nil {
		return closureBundle{}, err
	}
	permission, err := reconstructPermission(permissionRow, sources)
	if err != nil {
		return closureBundle{}, err
	}
	effectDigest, err := authority.NewDigest(basisRow.institutedEffectDigest)
	if err != nil {
		return closureBundle{}, fmt.Errorf("parse basis instituted-effect digest: %w", err)
	}
	effectRow, err := readEffectRow(ctx, database, effectDigest)
	if err != nil {
		return closureBundle{}, err
	}
	effect, err := reconstructEffect(effectRow, permission)
	if err != nil {
		return closureBundle{}, err
	}
	basis, err := reconstructBasis(basisRow, sources, permission, effect)
	if err != nil {
		return closureBundle{}, err
	}
	closure, err := profileauthority.NewClosure(sources.prepared, sources.source)
	if err != nil {
		return closureBundle{}, fmt.Errorf("rebuild profile authority closure: %w", err)
	}
	if err := compareClosureMembers(closure, permission, effect, basis); err != nil {
		return closureBundle{}, err
	}
	return closureBundle{closure: closure, basis: basis, sources: sources}, nil
}

func resolveExactClosureSources(
	ctx context.Context,
	database *sql.DB,
	preparedDigestRaw string,
	speechActRefRaw string,
	speechActDigestRaw string,
	policyRefRaw string,
	policyDigestRaw string,
) (exactClosureSources, error) {
	preparedDigest, err := authority.NewDigest(preparedDigestRaw)
	if err != nil {
		return exactClosureSources{}, fmt.Errorf("parse prepared authorization digest: %w", err)
	}
	prepared, err := LoadPreparedAuthorization(ctx, database, preparedDigest)
	if err != nil {
		return exactClosureSources{}, fmt.Errorf("load exact prepared authorization: %w", err)
	}
	speechActRef, err := authority.NewSpeechActRef(speechActRefRaw)
	if err != nil {
		return exactClosureSources{}, fmt.Errorf("parse source SpeechAct ref: %w", err)
	}
	speechActDigest, err := authority.NewDigest(speechActDigestRaw)
	if err != nil {
		return exactClosureSources{}, fmt.Errorf("parse source SpeechAct digest: %w", err)
	}
	source, err := authority.LoadRecordedSpeechActSource(
		ctx,
		database,
		speechActRef,
		speechActDigest,
	)
	if err != nil {
		return exactClosureSources{}, fmt.Errorf("load exact generic SpeechAct source: %w", err)
	}
	if err := profileauthority.ValidateRecordedSource(prepared, source); err != nil {
		return exactClosureSources{}, err
	}
	policyRef, err := authority.NewContextPolicyRef(policyRefRaw)
	if err != nil {
		return exactClosureSources{}, fmt.Errorf("parse context-policy ref: %w", err)
	}
	policyDigest, err := authority.NewDigest(policyDigestRaw)
	if err != nil {
		return exactClosureSources{}, fmt.Errorf("parse context-policy digest: %w", err)
	}
	policy, err := authority.LoadSpeechActContextPolicy(
		ctx,
		database,
		policyRef,
		policyDigest,
	)
	if err != nil {
		return exactClosureSources{}, fmt.Errorf("load independent context-policy member: %w", err)
	}
	if err := validatePreparedPolicy(prepared, policy); err != nil {
		return exactClosureSources{}, err
	}
	return exactClosureSources{
		prepared: prepared,
		source:   source,
		policy:   policy,
	}, nil
}

func reconstructPermission(
	row permissionRow,
	sources exactClosureSources,
) (profileauthority.Permission, error) {
	permission, err := profileauthority.NewPermission(sources.prepared, sources.source)
	if err != nil {
		return profileauthority.Permission{}, fmt.Errorf("rebuild profile Permission: %w", err)
	}
	if err := validatePermissionRow(row, permission, sources); err != nil {
		return profileauthority.Permission{}, err
	}
	return permission, nil
}

func reconstructEffect(
	row effectRow,
	permission profileauthority.Permission,
) (profileauthority.InstitutedEffect, error) {
	effect, err := profileauthority.NewInstitutedEffect(permission)
	if err != nil {
		return profileauthority.InstitutedEffect{}, fmt.Errorf("rebuild instituted profile effect: %w", err)
	}
	digest, digestOK := effect.Digest()
	canonical, canonicalOK := effect.CanonicalBytes()
	_, recordedAtErr := parseCanonicalTime(row.recordedAt)
	valid := digestOK && canonicalOK && recordedAtErr == nil
	valid = valid && digest.String() == row.digest
	valid = valid && slices.Equal(canonical, []byte(row.canonical))
	if !valid {
		return profileauthority.InstitutedEffect{}, fmt.Errorf(
			"stored instituted profile effect failed canonical rehash",
		)
	}
	expected := effectJSON{}
	if err := json.Unmarshal(canonical, &expected); err != nil {
		return profileauthority.InstitutedEffect{}, err
	}
	actual := effectJSON{
		Schema:           expected.Schema,
		ProjectRoot:      row.projectRoot,
		SpeechActRef:     row.speechActRef,
		SpeechActDigest:  row.speechActDigest,
		PermissionRef:    row.permissionRef,
		PermissionDigest: row.permissionDigest,
	}
	if actual != expected {
		return profileauthority.InstitutedEffect{}, fmt.Errorf(
			"stored instituted profile effect columns differ from canonical content",
		)
	}
	return effect, nil
}

func reconstructBasis(
	row basisRow,
	sources exactClosureSources,
	permission profileauthority.Permission,
	effect profileauthority.InstitutedEffect,
) (profileauthority.FourRefBasis, error) {
	basis, err := profileauthority.NewFourRefBasis(sources.prepared, permission)
	if err != nil {
		return profileauthority.FourRefBasis{}, fmt.Errorf("rebuild four-ref basis: %w", err)
	}
	digest, digestOK := basis.Digest()
	canonical, canonicalOK := basis.CanonicalBytes()
	effectDigest, effectDigestOK := effect.Digest()
	_, recordedAtErr := parseCanonicalTime(row.recordedAt)
	valid := digestOK && canonicalOK && effectDigestOK && recordedAtErr == nil
	valid = valid && digest.String() == row.digest
	valid = valid && effectDigest.String() == row.institutedEffectDigest
	valid = valid && slices.Equal(canonical, []byte(row.canonical))
	if !valid {
		return profileauthority.FourRefBasis{}, fmt.Errorf(
			"stored profile four-ref basis failed canonical rehash",
		)
	}
	expected := basisJSON{}
	if err := json.Unmarshal(canonical, &expected); err != nil {
		return profileauthority.FourRefBasis{}, err
	}
	actual := basisJSON{
		Schema:              expected.Schema,
		BasisRef:            row.ref,
		ProjectRoot:         row.projectRoot,
		SpeechActRef:        row.speechActRef,
		SpeechActDigest:     row.speechActDigest,
		ContentRef:          row.contentRef,
		ContentDigest:       row.contentDigest,
		PermissionRef:       row.permissionRef,
		PermissionDigest:    row.permissionDigest,
		ContextPolicyRef:    row.contextPolicyRef,
		ContextPolicyDigest: row.contextPolicyDigest,
	}
	if actual != expected {
		return profileauthority.FourRefBasis{}, fmt.Errorf(
			"stored profile four-ref basis columns differ from canonical content",
		)
	}
	return basis, nil
}

func validatePermissionRow(
	row permissionRow,
	permission profileauthority.Permission,
	sources exactClosureSources,
) error {
	digest, digestOK := permission.Digest()
	canonical, canonicalOK := permission.CanonicalBytes()
	preparedDigest, preparedDigestOK := sources.prepared.Digest()
	_, recordedAtErr := parseCanonicalTime(row.recordedAt)
	valid := digestOK && canonicalOK && preparedDigestOK && recordedAtErr == nil
	valid = valid && digest.String() == row.digest
	valid = valid && preparedDigest.String() == row.preparedDigest
	valid = valid && row.permissionKind == "U.Commitment"
	valid = valid && slices.Equal(canonical, []byte(row.canonical))
	if !valid {
		return fmt.Errorf("stored profile Permission failed canonical rehash")
	}
	expected := permissionJSON{}
	if err := json.Unmarshal(canonical, &expected); err != nil {
		return err
	}
	referents, err := parseStringArray(row.referentsJSON)
	if err != nil {
		return err
	}
	evidence, err := parseStringArray(row.evidenceClaimRefsJSON)
	if err != nil {
		return err
	}
	carriers, err := parseStringArray(row.carrierClassRefsJSON)
	if err != nil {
		return err
	}
	actual := permissionJSON{
		Schema:                   expected.Schema,
		PermissionRef:            row.ref,
		ProjectRoot:              row.projectRoot,
		SubjectRef:               row.subjectRef,
		SubjectDigest:            row.subjectDigest,
		Modality:                 row.modality,
		ActionKind:               row.actionKind,
		ClaimScopeRef:            row.claimScopeRef,
		BoundedContextRef:        row.boundedContextRef,
		ValidFrom:                row.validFrom,
		ValidUntil:               row.validUntil,
		Referents:                referents,
		AuthorizationContentRef:  row.contentRef,
		AuthorizationContentDig:  row.contentDigest,
		MethodDescriptionRef:     row.methodDescriptionRef,
		MethodDescriptionDigest:  row.methodDescriptionDigest,
		EnactabilityPredicateRef: row.enactabilityPredicateRef,
		EvidenceClaimRefs:        evidence,
		CarrierClassRefs:         carriers,
		VerifierIdentity:         row.verifierIdentity,
		VerifierVersion:          row.verifierVersion,
		EvaluationPolicyRef:      row.evaluationPolicyRef,
		EvaluationPolicyDigest:   row.evaluationPolicyDigest,
		SourceSpeechActRef:       row.sourceSpeechActRef,
		SourceSpeechActDigest:    row.sourceSpeechActDigest,
		ContextPolicyRef:         row.contextPolicyRef,
		ContextPolicyDigest:      row.contextPolicyDigest,
		CaptureCarrierRef:        row.captureRef,
		CaptureCarrierDigest:     row.captureDigest,
	}
	if !samePermissionJSON(actual, expected) {
		return fmt.Errorf("stored profile Permission columns differ from canonical content")
	}
	return validatePreparedPolicy(sources.prepared, sources.policy)
}

func samePermissionJSON(left permissionJSON, right permissionJSON) bool {
	leftReferents := left.Referents
	rightReferents := right.Referents
	leftEvidence := left.EvidenceClaimRefs
	rightEvidence := right.EvidenceClaimRefs
	leftCarriers := left.CarrierClassRefs
	rightCarriers := right.CarrierClassRefs
	left.Referents = nil
	right.Referents = nil
	left.EvidenceClaimRefs = nil
	right.EvidenceClaimRefs = nil
	left.CarrierClassRefs = nil
	right.CarrierClassRefs = nil
	return reflect.DeepEqual(left, right) &&
		slices.Equal(leftReferents, rightReferents) &&
		slices.Equal(leftEvidence, rightEvidence) &&
		slices.Equal(leftCarriers, rightCarriers)
}

func validatePreparedPolicy(
	prepared profileauthority.PreparedAuthorization,
	policy authority.SpeechActContextPolicy,
) error {
	expected, expectedOK := prepared.ContextPolicy()
	expectedRef, expectedRefOK := expected.Ref()
	expectedDigest, expectedDigestOK := expected.Digest()
	actualRef, actualRefOK := policy.Ref()
	actualDigest, actualDigestOK := policy.Digest()
	complete := expectedOK && expectedRefOK && expectedDigestOK
	complete = complete && actualRefOK && actualDigestOK
	exact := complete && expectedRef.String() == actualRef.String()
	exact = exact && expectedDigest.String() == actualDigest.String()
	if !exact {
		return fmt.Errorf("independent context-policy member differs from preparation")
	}
	return nil
}

func compareClosureMembers(
	closure profileauthority.Closure,
	permission profileauthority.Permission,
	effect profileauthority.InstitutedEffect,
	basis profileauthority.FourRefBasis,
) error {
	actualPermission, permissionOK := closure.Permission()
	actualEffect, effectOK := closure.Effect()
	actualBasis, basisOK := closure.Basis()
	permissionExact := sameCanonicalPermission(actualPermission, permission)
	effectExact := sameCanonicalEffect(actualEffect, effect)
	basisExact := sameCanonicalBasis(actualBasis, basis)
	if !permissionOK || !effectOK || !basisOK ||
		!permissionExact || !effectExact || !basisExact {
		return fmt.Errorf("stored profile closure differs from reconstructed members")
	}
	return nil
}

func sameCanonicalPermission(
	left profileauthority.Permission,
	right profileauthority.Permission,
) bool {
	leftDigest, leftDigestOK := left.Digest()
	rightDigest, rightDigestOK := right.Digest()
	leftBytes, leftBytesOK := left.CanonicalBytes()
	rightBytes, rightBytesOK := right.CanonicalBytes()
	return leftDigestOK && rightDigestOK && leftBytesOK && rightBytesOK &&
		leftDigest.String() == rightDigest.String() && slices.Equal(leftBytes, rightBytes)
}

func sameCanonicalEffect(
	left profileauthority.InstitutedEffect,
	right profileauthority.InstitutedEffect,
) bool {
	leftDigest, leftDigestOK := left.Digest()
	rightDigest, rightDigestOK := right.Digest()
	leftBytes, leftBytesOK := left.CanonicalBytes()
	rightBytes, rightBytesOK := right.CanonicalBytes()
	return leftDigestOK && rightDigestOK && leftBytesOK && rightBytesOK &&
		leftDigest.String() == rightDigest.String() && slices.Equal(leftBytes, rightBytes)
}

func sameCanonicalBasis(
	left profileauthority.FourRefBasis,
	right profileauthority.FourRefBasis,
) bool {
	leftDigest, leftDigestOK := left.Digest()
	rightDigest, rightDigestOK := right.Digest()
	leftBytes, leftBytesOK := left.CanonicalBytes()
	rightBytes, rightBytesOK := right.CanonicalBytes()
	return leftDigestOK && rightDigestOK && leftBytesOK && rightBytesOK &&
		leftDigest.String() == rightDigest.String() && slices.Equal(leftBytes, rightBytes)
}

func parseStringArray(raw string) ([]string, error) {
	values := []string{}
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil, fmt.Errorf("decode typed reference array: %w", err)
	}
	return values, nil
}

func readPermissionRow(
	ctx context.Context,
	database *sql.DB,
	ref authority.PermissionRef,
	digest authority.Digest,
) (permissionRow, error) {
	transaction, err := beginRead(ctx, database)
	if err != nil {
		return permissionRow{}, err
	}
	row, found, scanErr := scanPermissionByRef(ctx, transaction, ref.String())
	if scanErr == nil && !found {
		scanErr = sql.ErrNoRows
	}
	if scanErr == nil && row.digest != digest.String() {
		scanErr = fmt.Errorf("profile Permission digest differs from requested identity")
	}
	return finishReadValue(ctx, transaction, row, scanErr)
}

func readEffectRow(
	ctx context.Context,
	database *sql.DB,
	digest authority.Digest,
) (effectRow, error) {
	transaction, err := beginRead(ctx, database)
	if err != nil {
		return effectRow{}, err
	}
	row, found, scanErr := scanEffectByDigest(ctx, transaction, digest.String())
	if scanErr == nil && !found {
		scanErr = sql.ErrNoRows
	}
	return finishReadValue(ctx, transaction, row, scanErr)
}

func readBasisRow(
	ctx context.Context,
	database *sql.DB,
	ref profileauthority.BasisRef,
	digest authority.Digest,
) (basisRow, error) {
	transaction, err := beginRead(ctx, database)
	if err != nil {
		return basisRow{}, err
	}
	row, found, scanErr := scanBasisByRef(ctx, transaction, ref.String())
	if scanErr == nil && !found {
		scanErr = sql.ErrNoRows
	}
	if scanErr == nil && row.digest != digest.String() {
		scanErr = fmt.Errorf("profile four-ref basis digest differs from requested identity")
	}
	return finishReadValue(ctx, transaction, row, scanErr)
}

func loadPreparedForContent(
	ctx context.Context,
	database *sql.DB,
	contentRef string,
	contentDigest string,
) (profileauthority.PreparedAuthorization, error) {
	transaction, err := beginRead(ctx, database)
	if err != nil {
		return profileauthority.PreparedAuthorization{}, err
	}
	row := preparationRow{}
	scanErr := transaction.ScanOne(
		ctx,
		selectPreparationSQL+" WHERE authorization_content_ref = ? AND authorization_content_digest = ?",
		[]any{contentRef, contentDigest},
		row.scanTargets(),
	)
	if scanErr == nil {
		var prepared profileauthority.PreparedAuthorization
		prepared, scanErr = reconstructPrepared(ctx, transaction, row)
		return finishReadValue(ctx, transaction, prepared, scanErr)
	}
	return finishReadValue(
		ctx,
		transaction,
		profileauthority.PreparedAuthorization{},
		scanErr,
	)
}

func scanPermissionByRef(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	ref string,
) (permissionRow, bool, error) {
	row := permissionRow{}
	err := transaction.ScanOne(
		ctx,
		selectPermissionSQL+" WHERE permission_ref = ?",
		[]any{ref},
		row.scanTargets(),
	)
	if errors.Is(err, sql.ErrNoRows) {
		return permissionRow{}, false, nil
	}
	if err != nil {
		return permissionRow{}, false, fmt.Errorf("load profile Permission: %w", err)
	}
	return row, true, nil
}

func scanEffectByDigest(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	digest string,
) (effectRow, bool, error) {
	row := effectRow{}
	err := transaction.ScanOne(
		ctx,
		selectEffectSQL+" WHERE effect_digest = ?",
		[]any{digest},
		row.scanTargets(),
	)
	if errors.Is(err, sql.ErrNoRows) {
		return effectRow{}, false, nil
	}
	if err != nil {
		return effectRow{}, false, fmt.Errorf("load instituted profile effect: %w", err)
	}
	return row, true, nil
}

func scanBasisByRef(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	ref string,
) (basisRow, bool, error) {
	row := basisRow{}
	err := transaction.ScanOne(
		ctx,
		selectBasisSQL+" WHERE basis_ref = ?",
		[]any{ref},
		row.scanTargets(),
	)
	if errors.Is(err, sql.ErrNoRows) {
		return basisRow{}, false, nil
	}
	if err != nil {
		return basisRow{}, false, fmt.Errorf("load profile four-ref basis: %w", err)
	}
	return row, true, nil
}

func mustDigestString(prepared profileauthority.PreparedAuthorization) string {
	digest, _ := prepared.Digest()
	return digest.String()
}
