package profileauthority

import (
	"slices"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
)

type AuthorityUseResultKind string

const (
	AuthorityUseNew     AuthorityUseResultKind = "new"
	AuthorityUseReplay  AuthorityUseResultKind = "replay"
	AuthorityUseDenied  AuthorityUseResultKind = "denied"
	AuthorityUseInvalid AuthorityUseResultKind = "invalid"
)

type AuthorityUseDenialCode string

const (
	UseDenialInvalidRef               AuthorityUseDenialCode = "invalid_use_ref"
	UseDenialInvalidAdmittedUse       AuthorityUseDenialCode = "invalid_admitted_use"
	UseDenialInvalidRequestDigest     AuthorityUseDenialCode = "invalid_admission_request_digest"
	UseDenialInvalidCommittedResult   AuthorityUseDenialCode = "invalid_committed_admission"
	UseDenialInvalidConsumedAt        AuthorityUseDenialCode = "invalid_consumed_at"
	UseDenialRecordedUseInvalid       AuthorityUseDenialCode = "recorded_use_invalid"
	UseDenialSingleUseAlreadyConsumed AuthorityUseDenialCode = "single_use_already_consumed"
)

type AuthorityUseDenial struct {
	code   AuthorityUseDenialCode
	detail string
}

func (denial AuthorityUseDenial) Code() AuthorityUseDenialCode {
	return denial.code
}

func (denial AuthorityUseDenial) Detail() string {
	return denial.detail
}

func (denial AuthorityUseDenial) valid() bool {
	return denial.code != "" && denial.detail != ""
}

type DeniedAuthorityUse struct {
	reasons []AuthorityUseDenial
}

func (denied DeniedAuthorityUse) Reasons() []AuthorityUseDenial {
	return slices.Clone(denied.reasons)
}

func (denied DeniedAuthorityUse) valid() bool {
	return len(denied.reasons) > 0 &&
		slices.IndexFunc(denied.reasons, func(reason AuthorityUseDenial) bool {
			return !reason.valid()
		}) == -1
}

type NewAuthorityUse struct {
	record AuthorityUseRecord
}

func (result NewAuthorityUse) Record() (AuthorityUseRecord, bool) {
	_, ok := result.record.snapshot()
	return result.record, ok
}

func (result NewAuthorityUse) valid() bool {
	_, ok := result.record.snapshot()
	return ok
}

type ReplayedAuthorityUse struct {
	record AuthorityUseRecord
}

func (result ReplayedAuthorityUse) Record() (AuthorityUseRecord, bool) {
	_, ok := result.record.snapshot()
	return result.record, ok
}

func (result ReplayedAuthorityUse) valid() bool {
	_, ok := result.record.snapshot()
	return ok
}

type AuthorityUseResult struct {
	newResult    *NewAuthorityUse
	replayResult *ReplayedAuthorityUse
	deniedResult *DeniedAuthorityUse
}

func (result AuthorityUseResult) Kind() AuthorityUseResultKind {
	newValid := result.newResult != nil && result.newResult.valid()
	replayValid := result.replayResult != nil && result.replayResult.valid()
	deniedValid := result.deniedResult != nil && result.deniedResult.valid()
	variants := boolAsInt(newValid)
	variants += 2 * boolAsInt(replayValid)
	variants += 4 * boolAsInt(deniedValid)
	kinds := map[int]AuthorityUseResultKind{
		1: AuthorityUseNew,
		2: AuthorityUseReplay,
		4: AuthorityUseDenied,
	}
	kind, ok := kinds[variants]
	if !ok {
		return AuthorityUseInvalid
	}
	return kind
}

func (result AuthorityUseResult) New() (NewAuthorityUse, bool) {
	if result.Kind() != AuthorityUseNew {
		return NewAuthorityUse{}, false
	}
	return *result.newResult, true
}

func (result AuthorityUseResult) Replay() (ReplayedAuthorityUse, bool) {
	if result.Kind() != AuthorityUseReplay {
		return ReplayedAuthorityUse{}, false
	}
	return *result.replayResult, true
}

func (result AuthorityUseResult) Denied() (DeniedAuthorityUse, bool) {
	if result.Kind() != AuthorityUseDenied {
		return DeniedAuthorityUse{}, false
	}
	return *result.deniedResult, true
}

// EvaluateNewAuthorityUse canonicalizes a durable use fact from a sealed gate
// result and an already-canonical committed admission pair. It performs no I/O;
// the transaction adapter must insert the admission, use, and revision and
// exact-reread them atomically.
func EvaluateNewAuthorityUse(
	ref ProfileDeclarationAuthorityUseRef,
	use AdmittedUse,
	admissionRequestDigest authority.Digest,
	committedAdmissionRef CommittedProfileAdmissionRef,
	committedAdmissionDigest authority.Digest,
	consumedAt time.Time,
) AuthorityUseResult {
	if !ref.valid() {
		return deniedAuthorityUse(
			UseDenialInvalidRef,
			"profile authority use ref is invalid",
		)
	}
	if !use.valid() {
		return deniedAuthorityUse(
			UseDenialInvalidAdmittedUse,
			"sealed admitted use is unavailable",
		)
	}
	if !validDigest(admissionRequestDigest) {
		return deniedAuthorityUse(
			UseDenialInvalidRequestDigest,
			"admission-request digest is invalid",
		)
	}
	if !committedAdmissionRef.valid() || !validDigest(committedAdmissionDigest) {
		return deniedAuthorityUse(
			UseDenialInvalidCommittedResult,
			"committed admission pair is invalid",
		)
	}
	judgedAt, _ := use.JudgedAt()
	canonicalConsumedAt := canonicalTime(consumedAt)
	if canonicalConsumedAt.IsZero() || !canonicalConsumedAt.Equal(judgedAt) {
		return deniedAuthorityUse(
			UseDenialInvalidConsumedAt,
			"consumed_at must equal the sealed gate judgement time",
		)
	}
	record, err := newAuthorityUseRecord(
		ref,
		use,
		admissionRequestDigest,
		committedAdmissionRef,
		committedAdmissionDigest,
		canonicalConsumedAt,
	)
	if err != nil {
		return deniedAuthorityUse(UseDenialInvalidAdmittedUse, err.Error())
	}
	created := NewAuthorityUse{record: record}
	return AuthorityUseResult{newResult: &created}
}

// EvaluateRecordedAuthorityUse implements historical exact replay. The same
// admission-request digest returns the original committed use without asking
// an expired permission to become current again. A different digest is denied
// as an attempted second use of the same single-use authority.
func EvaluateRecordedAuthorityUse(
	record AuthorityUseRecord,
	admissionRequestDigest authority.Digest,
) AuthorityUseResult {
	snapshot, ok := record.snapshot()
	if !ok {
		return deniedAuthorityUse(
			UseDenialRecordedUseInvalid,
			"exact recorded authority use is unavailable",
		)
	}
	if !validDigest(admissionRequestDigest) {
		return deniedAuthorityUse(
			UseDenialInvalidRequestDigest,
			"admission-request digest is invalid",
		)
	}
	if snapshot.admissionRequestDigest.String() != admissionRequestDigest.String() {
		return deniedAuthorityUse(
			UseDenialSingleUseAlreadyConsumed,
			"single-use authority is bound to another admission-request digest",
		)
	}
	replayed := ReplayedAuthorityUse{record: record}
	return AuthorityUseResult{replayResult: &replayed}
}

func deniedAuthorityUse(
	code AuthorityUseDenialCode,
	detail string,
) AuthorityUseResult {
	denied := DeniedAuthorityUse{
		reasons: []AuthorityUseDenial{{code: code, detail: detail}},
	}
	return AuthorityUseResult{deniedResult: &denied}
}
