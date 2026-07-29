package profileauthority

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
)

func TestEvaluateNewAuthorityUseBindsExactSealedResolutionAndAdmission(t *testing.T) {
	_, closure := testResolutionClosure(t, "use-new")
	permission, _ := closure.Permission()
	checkedAt := permission.state.validity.From().Add(time.Minute)
	resolutionRef := mustParsed(
		t,
		"profile-authority-resolution:use-new",
		NewProfileDeclarationAuthorityResolutionRef,
	)
	resolution := requireNewResolution(
		t,
		EvaluateNewResolution(resolutionRef, closure, checkedAt),
	)
	resolutionRecord, _ := resolution.Record()
	consumedAt := checkedAt.Add(time.Minute)
	replayed := EvaluateReplayResolution(resolutionRecord, closure, consumedAt)
	replayedResolution, ok := replayed.Replay()
	if replayed.Kind() != ResolutionReplay || !ok {
		t.Fatalf("resolution replay kind = %q", replayed.Kind())
	}
	admittedUse, ok := replayedResolution.AdmittedUse()
	if !ok {
		t.Fatal("resolution replay omitted sealed admitted use")
	}
	useRef := mustParsed(
		t,
		"profile-authority-use:use-new",
		NewProfileDeclarationAuthorityUseRef,
	)
	requestDigest := testDigest(t, "use-new-request")
	admissionRef := mustParsed(
		t,
		"profile-admission:use-new",
		NewCommittedProfileAdmissionRef,
	)
	admissionDigest := testDigest(t, "use-new-admission")

	result := EvaluateNewAuthorityUse(
		useRef,
		admittedUse,
		requestDigest,
		admissionRef,
		admissionDigest,
		consumedAt,
	)
	if result.Kind() != AuthorityUseNew {
		t.Fatalf("authority-use kind = %q", result.Kind())
	}
	created, ok := result.New()
	if !ok {
		t.Fatal("new authority use is unavailable")
	}
	record, ok := created.Record()
	if !ok {
		t.Fatal("new authority use omitted canonical record")
	}
	assertAuthorityUseMatchesInputs(
		t,
		record,
		admittedUse,
		requestDigest,
		admissionRef,
		admissionDigest,
		consumedAt,
	)
}

func TestEvaluateRecordedAuthorityUseReplaysOnlyOriginalRequest(t *testing.T) {
	record, requestDigest := testAuthorityUseRecord(t, "use-replay")

	replayedResult := EvaluateRecordedAuthorityUse(record, requestDigest)
	if replayedResult.Kind() != AuthorityUseReplay {
		t.Fatalf("authority-use replay kind = %q", replayedResult.Kind())
	}
	replayed, ok := replayedResult.Replay()
	if !ok {
		t.Fatal("replayed authority use is unavailable")
	}
	replayedRecord, ok := replayed.Record()
	if !ok {
		t.Fatal("replayed authority use omitted original record")
	}
	originalDigest, _ := record.Digest()
	replayedDigest, _ := replayedRecord.Digest()
	if originalDigest.String() != replayedDigest.String() {
		t.Fatal("authority-use replay changed the durable record")
	}

	differentRequest := testDigest(t, "use-replay-different-request")
	deniedResult := EvaluateRecordedAuthorityUse(record, differentRequest)
	assertDeniedAuthorityUse(
		t,
		deniedResult,
		UseDenialSingleUseAlreadyConsumed,
	)
}

func TestEvaluateNewAuthorityUseRejectsUnsealedOrTimeShiftedUse(t *testing.T) {
	record, requestDigest := testAuthorityUseRecord(t, "use-denials")
	useRef, _ := record.Ref()
	admissionRef, admissionDigest, _ := record.CommittedAdmission()
	consumedAt, _ := record.ConsumedAt()
	resolutionRecord, _ := record.state.admittedUse.AuthorityResolutionRecord()
	resolutionUse, err := mintAdmittedUse(resolutionRecord, consumedAt)
	if err != nil {
		t.Fatalf("mint fixture admitted use: %v", err)
	}
	cases := []struct {
		name string
		want AuthorityUseDenialCode
		run  func() AuthorityUseResult
	}{
		{
			name: "zero admitted use",
			want: UseDenialInvalidAdmittedUse,
			run: func() AuthorityUseResult {
				return EvaluateNewAuthorityUse(
					useRef,
					AdmittedUse{},
					requestDigest,
					admissionRef,
					admissionDigest,
					consumedAt,
				)
			},
		},
		{
			name: "shifted consumption time",
			want: UseDenialInvalidConsumedAt,
			run: func() AuthorityUseResult {
				return EvaluateNewAuthorityUse(
					useRef,
					resolutionUse,
					requestDigest,
					admissionRef,
					admissionDigest,
					consumedAt.Add(time.Nanosecond),
				)
			},
		},
		{
			name: "zero use ref",
			want: UseDenialInvalidRef,
			run: func() AuthorityUseResult {
				return EvaluateNewAuthorityUse(
					ProfileDeclarationAuthorityUseRef{},
					resolutionUse,
					requestDigest,
					admissionRef,
					admissionDigest,
					consumedAt,
				)
			},
		},
	}
	for _, current := range cases {
		t.Run(current.name, func(t *testing.T) {
			assertDeniedAuthorityUse(t, current.run(), current.want)
		})
	}
}

func testAuthorityUseRecord(
	t *testing.T,
	identity string,
) (AuthorityUseRecord, authority.Digest) {
	t.Helper()
	_, closure := testResolutionClosure(t, identity)
	permission, _ := closure.Permission()
	checkedAt := permission.state.validity.From().Add(time.Minute)
	resolutionRef := mustParsed(
		t,
		"profile-authority-resolution:"+identity,
		NewProfileDeclarationAuthorityResolutionRef,
	)
	resolution := requireNewResolution(
		t,
		EvaluateNewResolution(resolutionRef, closure, checkedAt),
	)
	resolutionRecord, _ := resolution.Record()
	consumedAt := checkedAt.Add(time.Minute)
	replayed := EvaluateReplayResolution(resolutionRecord, closure, consumedAt)
	replayedResolution, ok := replayed.Replay()
	if replayed.Kind() != ResolutionReplay || !ok {
		t.Fatalf("resolution replay kind = %q", replayed.Kind())
	}
	use, ok := replayedResolution.AdmittedUse()
	if !ok {
		t.Fatal("resolution replay omitted sealed admitted use")
	}
	useRef := mustParsed(
		t,
		"profile-authority-use:"+identity,
		NewProfileDeclarationAuthorityUseRef,
	)
	requestDigest := testDigest(t, identity+"-request")
	admissionRef := mustParsed(
		t,
		"profile-admission:"+identity,
		NewCommittedProfileAdmissionRef,
	)
	admissionDigest := testDigest(t, identity+"-admission")
	result := EvaluateNewAuthorityUse(
		useRef,
		use,
		requestDigest,
		admissionRef,
		admissionDigest,
		consumedAt,
	)
	created, ok := result.New()
	if result.Kind() != AuthorityUseNew || !ok {
		t.Fatalf("authority-use creation = %q", result.Kind())
	}
	record, ok := created.Record()
	if !ok {
		t.Fatal("authority-use creation omitted record")
	}
	return record, requestDigest
}

func assertAuthorityUseMatchesInputs(
	t *testing.T,
	record AuthorityUseRecord,
	use AdmittedUse,
	requestDigest authority.Digest,
	admissionRef CommittedProfileAdmissionRef,
	admissionDigest authority.Digest,
	consumedAt time.Time,
) {
	t.Helper()
	canonical, canonicalOK := record.CanonicalBytes()
	digest, digestOK := record.Digest()
	if !canonicalOK || !digestOK || len(canonical) == 0 {
		t.Fatal("authority-use record omitted canonical identity")
	}
	dto := authorityUseJSONV2{}
	if err := json.Unmarshal(canonical, &dto); err != nil {
		t.Fatalf("decode authority-use canonical JSON: %v", err)
	}
	if dto.Schema != "haft.profile-authority.authority-use/v2" ||
		dto.UseRef != "profile-authority-use:use-new" ||
		dto.AdmissionRequestDigest != requestDigest.String() ||
		dto.CommittedAdmissionRef != admissionRef.String() ||
		dto.CommittedAdmissionDigest != admissionDigest.String() ||
		dto.ConsumedAt != formatTime(consumedAt) {
		t.Fatalf("authority-use canonical material = %#v", dto)
	}
	if dto.UseRef == digest.String() {
		t.Fatal("authority-use digest was self-included in canonical JSON")
	}
	resolutionRef, resolutionDigest, resolutionOK := record.Resolution()
	expectedResolutionRef, expectedResolutionDigest, _ := use.Resolution()
	basisRef, basisDigest, basisOK := record.Basis()
	expectedBasisRef, expectedBasisDigest, _ := use.Basis()
	permissionRef, permissionDigest, permissionOK := record.Permission()
	expectedPermissionRef, expectedPermissionDigest, _ := use.Permission()
	contentRef, contentDigest, contentOK := record.AuthorizationContent()
	expectedContentRef, expectedContentDigest, _ := use.AuthorizationContent()
	key, keyOK := record.SingleUseKey()
	expectedKey, _ := use.SingleUseKey()
	if !resolutionOK || resolutionRef.String() != expectedResolutionRef.String() ||
		resolutionDigest.String() != expectedResolutionDigest.String() ||
		!basisOK || basisRef.String() != expectedBasisRef.String() ||
		basisDigest.String() != expectedBasisDigest.String() ||
		!permissionOK || permissionRef.String() != expectedPermissionRef.String() ||
		permissionDigest.String() != expectedPermissionDigest.String() ||
		!contentOK || contentRef.String() != expectedContentRef.String() ||
		contentDigest.String() != expectedContentDigest.String() ||
		!keyOK || key.String() != expectedKey.String() {
		t.Fatal("authority-use record differs from sealed admitted-use bindings")
	}
}

func assertDeniedAuthorityUse(
	t *testing.T,
	result AuthorityUseResult,
	want AuthorityUseDenialCode,
) {
	t.Helper()
	if result.Kind() != AuthorityUseDenied {
		t.Fatalf("authority-use kind = %q, want denied", result.Kind())
	}
	denied, ok := result.Denied()
	if !ok {
		t.Fatal("denied authority-use result is unavailable")
	}
	reasons := denied.Reasons()
	if len(reasons) != 1 || reasons[0].Code() != want {
		t.Fatalf("authority-use denials = %#v, want %q", reasons, want)
	}
}
