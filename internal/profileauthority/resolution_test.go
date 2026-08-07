package profileauthority

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
)

func TestEvaluateNewResolutionBindsExactSourceAndActionEnvelope(t *testing.T) {
	prepared, closure := testResolutionClosure(t, "new-exact")
	permission, ok := closure.Permission()
	if !ok {
		t.Fatal("closure omitted permission")
	}
	checkedAt := permission.state.validity.From().Add(time.Minute)
	ref := mustParsed(
		t,
		"profile-authority-resolution:new-exact",
		NewProfileDeclarationAuthorityResolutionRef,
	)

	result := EvaluateNewResolution(ref, closure, checkedAt)
	if result.Kind() != ResolutionNew {
		t.Fatalf("resolution kind = %q", result.Kind())
	}
	created, ok := result.New()
	if !ok {
		t.Fatal("new result is unavailable")
	}
	record, ok := created.Record()
	if !ok {
		t.Fatal("new result omitted canonical record")
	}
	assertResolutionMatchesSourceClosure(t, record, prepared, closure, checkedAt)
}

func TestEvaluateReplayResolutionPreservesOriginalRecord(t *testing.T) {
	_, closure := testResolutionClosure(t, "replay")
	permission, _ := closure.Permission()
	checkedAt := permission.state.validity.From().Add(time.Minute)
	judgedAt := checkedAt.Add(time.Minute)
	ref := mustParsed(
		t,
		"profile-authority-resolution:replay",
		NewProfileDeclarationAuthorityResolutionRef,
	)
	created := requireNewResolution(t, EvaluateNewResolution(ref, closure, checkedAt))
	original, _ := created.Record()
	originalDigest, _ := original.Digest()
	originalBytes, _ := original.CanonicalBytes()

	replayedResult := EvaluateReplayResolution(original, closure, judgedAt)
	if replayedResult.Kind() != ResolutionReplay {
		t.Fatalf("replay kind = %q", replayedResult.Kind())
	}
	replayed, ok := replayedResult.Replay()
	if !ok {
		t.Fatal("replayed result is unavailable")
	}
	replayedRecord, _ := replayed.Record()
	replayedDigest, _ := replayedRecord.Digest()
	replayedBytes, _ := replayedRecord.CanonicalBytes()
	replayedCheckedAt, _ := replayedRecord.CheckedAt()
	if replayedDigest.String() != originalDigest.String() ||
		string(replayedBytes) != string(originalBytes) ||
		!replayedCheckedAt.Equal(checkedAt) {
		t.Fatal("replay renewed or changed the original resolution")
	}
	use, _ := replayed.AdmittedUse()
	replayJudgedAt, ok := use.JudgedAt()
	if !ok || !replayJudgedAt.Equal(judgedAt) {
		t.Fatalf("replay judgement = %s", replayJudgedAt)
	}
	if _, err := json.Marshal(use); err == nil ||
		!strings.Contains(err.Error(), "no wire form") {
		t.Fatalf("AdmittedUse JSON error = %v", err)
	}
}

func TestResolutionGateDeniesForeignExpiredAndCorruptReplay(t *testing.T) {
	_, closure := testResolutionClosure(t, "denials")
	permission, _ := closure.Permission()
	checkedAt := permission.state.validity.From().Add(time.Minute)
	ref := mustParsed(
		t,
		"profile-authority-resolution:denials",
		NewProfileDeclarationAuthorityResolutionRef,
	)
	created := requireNewResolution(t, EvaluateNewResolution(ref, closure, checkedAt))
	record, _ := created.Record()
	_, foreignClosure := testResolutionClosure(t, "foreign-denials")

	cases := []struct {
		name string
		want ResolutionDenialCode
		run  func() ResolutionResult
	}{
		{
			name: "foreign closure",
			want: DenialReplayBindingMismatch,
			run: func() ResolutionResult {
				return EvaluateReplayResolution(record, foreignClosure, checkedAt)
			},
		},
		{
			name: "expired permission",
			want: DenialPermissionNotCurrent,
			run: func() ResolutionResult {
				return EvaluateReplayResolution(
					record,
					closure,
					permission.state.validity.Until(),
				)
			},
		},
		{
			name: "time before original resolution",
			want: DenialReplayBeforeResolution,
			run: func() ResolutionResult {
				return EvaluateReplayResolution(
					record,
					closure,
					checkedAt.Add(-time.Nanosecond),
				)
			},
		},
		{
			name: "zero record",
			want: DenialReplayRecordInvalid,
			run: func() ResolutionResult {
				return EvaluateReplayResolution(
					AuthorityResolutionRecord{},
					closure,
					checkedAt,
				)
			},
		},
		{
			name: "corrupt canonical record",
			want: DenialReplayRecordInvalid,
			run: func() ResolutionResult {
				state := *record.state
				state.canonical = append([]byte{}, state.canonical...)
				state.canonical[0] = '['
				corrupt := AuthorityResolutionRecord{state: &state}
				return EvaluateReplayResolution(corrupt, closure, checkedAt)
			},
		},
	}
	for _, current := range cases {
		t.Run(current.name, func(t *testing.T) {
			assertDeniedResolution(t, current.run(), current.want)
		})
	}
}

func TestEvaluateNewResolutionDeniesInvalidOrNonCurrentInputs(t *testing.T) {
	_, closure := testResolutionClosure(t, "new-denials")
	permission, _ := closure.Permission()
	validRef := mustParsed(
		t,
		"profile-authority-resolution:new-denials",
		NewProfileDeclarationAuthorityResolutionRef,
	)
	cases := []struct {
		name string
		want ResolutionDenialCode
		run  func() ResolutionResult
	}{
		{
			name: "zero ref",
			want: DenialInvalidResolutionRef,
			run: func() ResolutionResult {
				return EvaluateNewResolution(
					ProfileDeclarationAuthorityResolutionRef{},
					closure,
					permission.state.validity.From(),
				)
			},
		},
		{
			name: "zero closure",
			want: DenialInvalidClosure,
			run: func() ResolutionResult {
				return EvaluateNewResolution(
					validRef,
					Closure{},
					permission.state.validity.From(),
				)
			},
		},
		{
			name: "zero checked at",
			want: DenialInvalidCheckedAt,
			run: func() ResolutionResult {
				return EvaluateNewResolution(validRef, closure, time.Time{})
			},
		},
		{
			name: "before permission",
			want: DenialPermissionNotCurrent,
			run: func() ResolutionResult {
				return EvaluateNewResolution(
					validRef,
					closure,
					permission.state.validity.From().Add(-time.Nanosecond),
				)
			},
		},
	}
	for _, current := range cases {
		t.Run(current.name, func(t *testing.T) {
			assertDeniedResolution(t, current.run(), current.want)
		})
	}
}

func testResolutionClosure(
	t *testing.T,
	identity string,
) (PreparedAuthorization, Closure) {
	t.Helper()
	prepared := testPreparedAuthorization(t, testPreparedInput{
		root:          "/tmp/haft-profile-resolution-" + identity,
		profileAuthor: "role-assignment:profile-author:" + identity,
		identity:      identity,
	})
	validity := prepared.state.content.state.authorizationValidity
	recipe := exactSourceRecipeForPrepared(
		t,
		prepared,
		validity.From().Add(time.Minute),
		validity.From().Add(2*time.Minute),
		validity.From().Add(3*time.Minute),
	)
	source := recordSourceFromRecipe(t, prepared, recipe)
	closure, err := NewClosure(prepared, source)
	if err != nil {
		t.Fatalf("NewClosure: %v", err)
	}
	return prepared, closure
}

func requireNewResolution(
	t *testing.T,
	result ResolutionResult,
) NewResolution {
	t.Helper()
	created, ok := result.New()
	if result.Kind() != ResolutionNew || !ok {
		t.Fatalf("resolution result = %q", result.Kind())
	}
	return created
}

func assertDeniedResolution(
	t *testing.T,
	result ResolutionResult,
	want ResolutionDenialCode,
) {
	t.Helper()
	if result.Kind() != ResolutionDenied {
		t.Fatalf("resolution kind = %q, want denied", result.Kind())
	}
	denied, ok := result.Denied()
	if !ok {
		t.Fatal("denied result is unavailable")
	}
	reasons := denied.Reasons()
	if len(reasons) != 1 || reasons[0].Code() != want {
		t.Fatalf("denial reasons = %#v, want %q", reasons, want)
	}
}

func assertResolutionMatchesSourceClosure(
	t *testing.T,
	record AuthorityResolutionRecord,
	prepared PreparedAuthorization,
	closure Closure,
	checkedAt time.Time,
) {
	t.Helper()
	ref, refOK := record.Ref()
	digest, digestOK := record.Digest()
	canonical, canonicalOK := record.CanonicalBytes()
	if !refOK || !digestOK || !canonicalOK || len(canonical) == 0 {
		t.Fatal("resolution omitted canonical identity")
	}
	if ref.String() != "profile-authority-resolution:new-exact" ||
		!strings.HasPrefix(digest.String(), "sha256:") {
		t.Fatalf("resolution identity = %s %s", ref.String(), digest.String())
	}
	dto := authorityResolutionJSONV2{}
	if err := json.Unmarshal(canonical, &dto); err != nil {
		t.Fatalf("decode resolution canonical JSON: %v", err)
	}
	if dto.Schema != "haft.profile-authority.authority-resolution/v2" ||
		dto.EnactabilityPredicateRef == "" ||
		dto.RoleStateRelation != roleStateRelationValue ||
		dto.EnactableState != enactableStateValue ||
		dto.CurrentnessResult != currentnessResultValue ||
		dto.PredicateResult != predicateResultValue ||
		dto.AdmissionResult != admissionResultValue {
		t.Fatalf("resolution admission semantics = %#v", dto)
	}
	basis, _ := closure.Basis()
	basisRef, _ := basis.Ref()
	basisDigest, _ := basis.Digest()
	actualBasisRef, actualBasisDigest, basisOK := record.Basis()
	if !basisOK || actualBasisRef.String() != basisRef.String() ||
		actualBasisDigest.String() != basisDigest.String() {
		t.Fatal("resolution does not bind the exact four-ref basis")
	}
	assertResolutionMemberPair(
		t,
		"SpeechAct",
		record.SpeechAct,
		basis.SpeechAct,
	)
	assertResolutionMemberPair(
		t,
		"authorization content",
		record.AuthorizationContent,
		basis.AuthorizationContent,
	)
	assertResolutionMemberPair(
		t,
		"permission",
		record.Permission,
		basis.Permission,
	)
	assertResolutionMemberPair(
		t,
		"context policy",
		record.ContextPolicy,
		basis.ContextPolicy,
	)
	root, action, projectDigest, projectOK := record.ProjectBinding()
	actionDigest, actionOK := record.ActionEnvelopeDigest()
	contentRoot, _ := prepared.state.content.ProjectRoot()
	expectedAction, _ := ActionKind()
	if !projectOK || !actionOK || root.String() != contentRoot.String() ||
		action.String() != expectedAction.String() ||
		projectDigest.String() == actionDigest.String() {
		t.Fatal("resolution collapsed or lost project/action envelope bindings")
	}
	key, keyOK := record.SingleUseKey()
	expectedKey, _ := prepared.state.content.SingleUseKey()
	workWindow, workOK := record.AllowedWorkWindow()
	expectedWork, _ := prepared.state.content.AllowedWorkWindow()
	basisWindow, basisWindowOK := record.AllowedBasisObservationWindow()
	expectedBasisWindow, _ := prepared.state.content.AllowedBasisObservationWindow()
	authorizationWindow, authorizationOK := record.AuthorizationValidity()
	expectedAuthorization, _ := prepared.state.content.AuthorizationValidity()
	permissionWindow, permissionOK := record.PermissionValidity()
	if !keyOK || key.String() != expectedKey.String() ||
		!workOK || workWindow != expectedWork ||
		!basisWindowOK || basisWindow != expectedBasisWindow ||
		!authorizationOK || authorizationWindow != expectedAuthorization ||
		!permissionOK || !permissionWindow.Contains(checkedAt) {
		t.Fatal("resolution did not preserve single-use/content/currentness windows")
	}
	recordedAt, checkedOK := record.CheckedAt()
	if !checkedOK || !recordedAt.Equal(checkedAt) ||
		!record.CurrentAtCheckedAt() ||
		!record.PredicateSatisfied() ||
		!record.Admitted() {
		t.Fatal("resolution does not represent admitted current predicate evaluation")
	}
}

func assertResolutionMemberPair[T interface{ String() string }](
	t *testing.T,
	name string,
	actual func() (T, authorityDigestAlias, bool),
	expected func() (T, authorityDigestAlias, bool),
) {
	t.Helper()
	actualRef, actualDigest, actualOK := actual()
	expectedRef, expectedDigest, expectedOK := expected()
	if !actualOK || !expectedOK || actualRef.String() != expectedRef.String() ||
		actualDigest.String() != expectedDigest.String() {
		t.Fatalf("resolution %s pair differs from basis", name)
	}
}

// authorityDigestAlias keeps the pair assertion generic without weakening the
// production API to strings.
type authorityDigestAlias = authority.Digest
