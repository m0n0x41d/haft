package authority

import (
	"bytes"
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"
)

type authorityBasisWriterFixture struct {
	database *sql.DB
	writer   *AuthorityBasisWriter
	act      VerifiedAuthorityAct
}

func TestAuthorityBasisWriterStoresLoadsAndReplaysExactGraph(t *testing.T) {
	fixture := newAuthorityBasisWriterFixture(t)

	stored, err := fixture.writer.Record(context.Background(), fixture.act)
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	if stored.Kind() != AuthorityBasisWriteStored {
		t.Fatalf("first write kind = %q, want %q", stored.Kind(), AuthorityBasisWriteStored)
	}
	recorded, ok := stored.RecordedAuthorityBasis()
	if !ok {
		t.Fatal("stored authority basis is unavailable")
	}
	id, ok := recorded.AuthorityResolutionID()
	if !ok {
		t.Fatal("stored authority basis omitted resolution ID")
	}
	digest, ok := recorded.AuthorityResolutionDigest()
	if !ok {
		t.Fatal("stored authority basis omitted resolution digest")
	}
	loaded, err := LoadRecordedAuthorityBasisByResolution(
		context.Background(),
		fixture.database,
		id,
		digest,
	)
	if err != nil {
		t.Fatalf("LoadRecordedAuthorityBasisByResolution: %v", err)
	}
	if !loaded.Valid() {
		t.Fatal("strict loader returned an invalid authority basis")
	}
	assertLegacyPermissionWindowStartsAtSpeechAct(t, fixture.database, id)

	replay, err := fixture.writer.Record(context.Background(), fixture.act)
	if err != nil {
		t.Fatalf("exact replay: %v", err)
	}
	if replay.Kind() != AuthorityBasisWriteExactReplay {
		t.Fatalf("replay kind = %q, want %q", replay.Kind(), AuthorityBasisWriteExactReplay)
	}
	replayed, ok := replay.RecordedAuthorityBasis()
	if !ok || !recordedAuthorityBasisMatchesAct(replayed, fixture.act) {
		t.Fatal("exact replay did not return the original durable graph")
	}
}

func assertLegacyPermissionWindowStartsAtSpeechAct(
	t *testing.T,
	database *sql.DB,
	resolutionID AuthorityResolutionID,
) {
	t.Helper()
	const statement = `SELECT
		legacy.permission_valid_from,
		permission.valid_from,
		capture.ended_at,
		legacy.valid_from
	FROM authority_resolution_records legacy_resolution
	JOIN authority_presentations legacy
		ON legacy.presentation_id = legacy_resolution.presentation_id
	JOIN authority_basis_presentations basis
		ON basis.presentation_id = legacy.presentation_id
	JOIN profile_declaration_permissions permission
		ON permission.permission_ref = basis.permission_ref
	JOIN terminal_capture_records capture
		ON capture.capture_carrier_ref = basis.capture_carrier_ref
	WHERE legacy_resolution.authority_resolution_id = ?`
	var legacyPermissionFrom string
	var permissionFrom string
	var workEndedAt string
	var contentValidFrom string
	err := database.QueryRow(statement, resolutionID.String()).Scan(
		&legacyPermissionFrom,
		&permissionFrom,
		&workEndedAt,
		&contentValidFrom,
	)
	if err != nil {
		t.Fatalf("load compatibility permission window: %v", err)
	}
	if legacyPermissionFrom != permissionFrom || permissionFrom != workEndedAt {
		t.Fatalf(
			"permission starts = legacy %q, v38 %q, capture %q",
			legacyPermissionFrom,
			permissionFrom,
			workEndedAt,
		)
	}
	if legacyPermissionFrom == contentValidFrom {
		t.Fatal("compatibility permission validity was widened before the SpeechAct")
	}
}

func TestAuthorityBasisStrictLoaderRejectsTamperedPermissionProjection(t *testing.T) {
	fixture := newAuthorityBasisWriterFixture(t)
	stored, err := fixture.writer.Record(context.Background(), fixture.act)
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	recorded, ok := stored.RecordedAuthorityBasis()
	if !ok {
		t.Fatal("stored authority basis is unavailable")
	}
	id, _ := recorded.AuthorityResolutionID()
	digest, _ := recorded.AuthorityResolutionDigest()

	_, err = fixture.database.Exec("DROP TRIGGER profile_declaration_permissions_no_update")
	if err != nil {
		t.Fatalf("drop immutable-row guard for corruption fixture: %v", err)
	}
	_, err = fixture.database.Exec(`UPDATE profile_declaration_permissions
		SET canonical_json = json_set(
			canonical_json,
			'$.adjudication_evidence_relation_ref',
			'evidence-relation:tampered'
		)`)
	if err != nil {
		t.Fatalf("inject malformed stored permission: %v", err)
	}

	_, err = LoadRecordedAuthorityBasisByResolution(
		context.Background(),
		fixture.database,
		id,
		digest,
	)
	if err == nil || !strings.Contains(err.Error(), "digest") {
		t.Fatalf("tampered permission load error = %v", err)
	}
}

func TestProfileDeclarationResolveRequestRejectsAlternateRecognizedActPolicy(t *testing.T) {
	original, capturedAt := testPreparedAuthorityIntent(t)
	state := original.state
	alternatePolicy, err := NewSpeechActContextPolicy(
		state.contextPolicy.state.ref,
		state.contextPolicy.state.boundedContext,
		mustParse(t, NewSpeechActTypeRef, "speech-act-type:endorse"),
		state.contextPolicy.state.effectRule,
	)
	if err != nil {
		t.Fatalf("NewSpeechActContextPolicy: %v", err)
	}
	alternate, err := NewPreparedAuthorityIntentBuilder(
		state.presentationID,
		state.authorityResolutionID,
		state.speechActRef,
		state.permissionRef,
		state.captureCarrierRef,
	).
		WithAuthorizationContent(state.content).
		InSpeechActSession(state.speechActSessionRef).
		UnderContextPolicy(alternatePolicy).
		WithSpeechActExecutionFrame(state.executionFrame).
		ScopedBy(state.permissionPredicateRef).
		WithinClaimScope(state.claimScopeRef).
		VerifiedBy(state.verifierIdentity, state.verifierVersion).
		UnderVerificationPolicy(state.verificationPolicyRef, state.verificationPolicyDigest).
		WithAdjudicationEvidence(state.evidenceRelationRef, state.carrierExpectationRef).
		ResolutionEffectiveWithin(state.resolutionWindow).
		Build()
	if err != nil {
		t.Fatalf("build alternate prepared authority intent: %v", err)
	}
	reviewText := "Authorize the exact profile declaration and no other action."
	reviewDigest, err := AuthorityIntentReviewDigest(alternate, reviewText)
	if err != nil {
		t.Fatalf("AuthorityIntentReviewDigest: %v", err)
	}
	terminal := newAuthorityIssueTerminalFixture(
		"AUTHORIZE "+reviewDigest.String()+"\n",
		true,
	)
	act, err := captureVerifiedAuthorityAct(
		context.Background(),
		alternate,
		reviewText,
		reviewDigest,
		terminal,
		authorityWorkClock(capturedAt),
		bytes.NewReader(bytes.Repeat([]byte{0x3c}, 16)),
	)
	if err != nil {
		t.Fatalf("capture alternate authority act: %v", err)
	}
	checked, err := newCheckedAuthorityBasis(act, act.state.capture.state.endedAt)
	if err != nil {
		t.Fatalf("newCheckedAuthorityBasis: %v", err)
	}
	legacy, err := newRecordedAuthority(checked.legacyPresentation, checked.legacyResolution)
	if err != nil {
		t.Fatalf("newRecordedAuthority: %v", err)
	}
	basis := RecordedAuthorityBasis{state: &recordedAuthorityBasisState{
		source:                 RecordedSpeechActSource{state: act.state.source.state},
		legacy:                 legacy,
		projectRoot:            checked.presentation.projectRoot,
		presentationID:         checked.presentation.id,
		presentationDigest:     checked.presentation.digest,
		authorityResolutionID:  checked.resolution.id,
		authorityResolutionDig: checked.resolution.digest,
		contentRef:             act.state.intent.state.content.state.ref,
		contentDigest:          act.state.intent.state.content.state.digest,
		profileAuthorRef:       act.state.intent.state.content.state.envelope.profileAuthor,
		permissionRef:          act.state.permission.state.ref,
		permissionDigest:       act.state.permission.state.digest,
		institutedEffectDigest: act.state.effect.state.digest,
		resolvedAt:             formatAuthorityTime(checked.resolution.resolvedAt),
		validUntil:             formatAuthorityTime(checked.resolution.validUntil),
	}}

	_, err = profileDeclarationResolveRequestFromBasis(basis)
	if err == nil || !strings.Contains(err.Error(), "sealed profile-declaration protocol") {
		t.Fatalf("alternate recognized-act policy error = %v", err)
	}
}

func newAuthorityBasisWriterFixture(t *testing.T) authorityBasisWriterFixture {
	t.Helper()
	database := openFrozenLegacyAuthoritySchema43(t)
	act := testVerifiedAuthorityAct(t)
	capturedAt := canonicalAuthorityTime(act.state.capture.state.endedAt)
	insertExactAuthoritySupport(t, authorityFixture{
		database: database,
		now:      capturedAt,
		envelope: act.state.intent.state.content.state.envelope,
	})
	writer, err := OpenAuthorityBasisWriter(database)
	if err != nil {
		t.Fatalf("OpenAuthorityBasisWriter: %v", err)
	}
	writer.now = func() time.Time { return capturedAt }
	return authorityBasisWriterFixture{
		database: database,
		writer:   writer,
		act:      act,
	}
}
