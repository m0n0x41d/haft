package profileauthority

import (
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
)

type ResolutionResultKind string

const (
	ResolutionNew     ResolutionResultKind = "new"
	ResolutionReplay  ResolutionResultKind = "replay"
	ResolutionDenied  ResolutionResultKind = "denied"
	ResolutionInvalid ResolutionResultKind = "invalid"
)

type ResolutionDenialCode string

const (
	DenialInvalidResolutionRef   ResolutionDenialCode = "invalid_resolution_ref"
	DenialInvalidClosure         ResolutionDenialCode = "invalid_source_closure"
	DenialInvalidCheckedAt       ResolutionDenialCode = "invalid_checked_at"
	DenialPermissionNotCurrent   ResolutionDenialCode = "permission_not_current"
	DenialReplayRecordInvalid    ResolutionDenialCode = "replay_record_invalid"
	DenialReplayBindingMismatch  ResolutionDenialCode = "replay_binding_mismatch"
	DenialReplayBeforeResolution ResolutionDenialCode = "replay_before_resolution"
)

// ResolutionDenial explains why no new resolution or replay capability was
// produced. A denial is not an AuthorityResolutionRecord and must not be
// persisted as an admitted evaluation.
type ResolutionDenial struct {
	code   ResolutionDenialCode
	detail string
}

func (denial ResolutionDenial) Code() ResolutionDenialCode {
	return denial.code
}

func (denial ResolutionDenial) Detail() string {
	return denial.detail
}

func (denial ResolutionDenial) valid() bool {
	return denial.code != "" && denial.detail != ""
}

type DeniedResolution struct {
	reasons []ResolutionDenial
}

func (denied DeniedResolution) Reasons() []ResolutionDenial {
	return slices.Clone(denied.reasons)
}

func (denied DeniedResolution) valid() bool {
	return len(denied.reasons) > 0 &&
		slices.IndexFunc(denied.reasons, func(reason ResolutionDenial) bool {
			return !reason.valid()
		}) == -1
}

type admittedUseSeal struct{}

var profileAuthorityGateSeal = &admittedUseSeal{}

type admittedUseState struct {
	record   AuthorityResolutionRecord
	judgedAt time.Time
	gateSeal *admittedUseSeal
}

// AdmittedUse is an opaque, action-specific, in-process capability minted only
// by exact replay of a persisted pre-Work resolution at the later transaction
// judgement time. It is a current evaluation snapshot, not a durable permission
// or reusable token. The SQLite transaction must exact-revalidate the
// resolution and enforce the single-use key.
type AdmittedUse struct {
	state *admittedUseState
}

func mintAdmittedUse(
	record AuthorityResolutionRecord,
	judgedAt time.Time,
) (AdmittedUse, error) {
	state := admittedUseState{
		record:   record,
		judgedAt: canonicalTime(judgedAt),
		gateSeal: profileAuthorityGateSeal,
	}
	use := AdmittedUse{state: &state}
	if !use.valid() {
		return AdmittedUse{}, fmt.Errorf(
			"profile authority gate refused an inconsistent admitted use",
		)
	}
	return use, nil
}

func (use AdmittedUse) valid() bool {
	if use.state == nil || use.state.gateSeal != profileAuthorityGateSeal {
		return false
	}
	snapshot, ok := use.state.record.snapshot()
	if !ok {
		return false
	}
	if use.state.judgedAt.Before(snapshot.checkedAt) {
		return false
	}
	return snapshot.permissionValidity.Contains(use.state.judgedAt)
}

// MarshalJSON rejects accidental persistence of the sealed in-process value.
// Persist the AuthorityResolutionRecord and the later AuthorityUseRecord, not
// this capability.
func (use AdmittedUse) MarshalJSON() ([]byte, error) {
	return nil, fmt.Errorf("AdmittedUse is an in-process capability and has no wire form")
}

var _ json.Marshaler = AdmittedUse{}

func (use AdmittedUse) Resolution() (
	ProfileDeclarationAuthorityResolutionRef,
	authority.Digest,
	bool,
) {
	if !use.valid() {
		return ProfileDeclarationAuthorityResolutionRef{}, authority.Digest{}, false
	}
	ref, _ := use.state.record.Ref()
	digest, _ := use.state.record.Digest()
	return ref, digest, true
}

// AuthorityResolutionRecord returns the exact immutable record bound inside
// this sealed use. A consumer should accept AdmittedUse and obtain the record
// through this method instead of accepting a separately supplied record/use
// pair that could be cross-bound.
func (use AdmittedUse) AuthorityResolutionRecord() (
	AuthorityResolutionRecord,
	bool,
) {
	if !use.valid() {
		return AuthorityResolutionRecord{}, false
	}
	return use.state.record, true
}

func (use AdmittedUse) Basis() (BasisRef, authority.Digest, bool) {
	if !use.valid() {
		return BasisRef{}, authority.Digest{}, false
	}
	return use.state.record.Basis()
}

func (use AdmittedUse) ProjectBinding() (
	authority.ProjectRoot,
	authority.ActionKind,
	authority.Digest,
	bool,
) {
	if !use.valid() {
		return authority.ProjectRoot{}, authority.ActionKind{}, authority.Digest{}, false
	}
	return use.state.record.ProjectBinding()
}

func (use AdmittedUse) ActionEnvelopeDigest() (authority.Digest, bool) {
	if !use.valid() {
		return authority.Digest{}, false
	}
	return use.state.record.ActionEnvelopeDigest()
}

func (use AdmittedUse) Permission() (
	authority.PermissionRef,
	authority.Digest,
	bool,
) {
	if !use.valid() {
		return authority.PermissionRef{}, authority.Digest{}, false
	}
	return use.state.record.Permission()
}

func (use AdmittedUse) AuthorizationContent() (
	authority.AuthorizationContentRef,
	authority.Digest,
	bool,
) {
	if !use.valid() {
		return authority.AuthorizationContentRef{}, authority.Digest{}, false
	}
	return use.state.record.AuthorizationContent()
}

func (use AdmittedUse) SingleUseKey() (authority.SingleUseKey, bool) {
	if !use.valid() {
		return authority.SingleUseKey{}, false
	}
	return use.state.record.SingleUseKey()
}

func (use AdmittedUse) JudgedAt() (time.Time, bool) {
	if !use.valid() {
		return time.Time{}, false
	}
	return use.state.judgedAt, true
}

type NewResolution struct {
	record AuthorityResolutionRecord
}

func (result NewResolution) Record() (AuthorityResolutionRecord, bool) {
	return result.record, result.valid()
}

func (result NewResolution) valid() bool {
	_, ok := result.record.snapshot()
	return ok
}

type ReplayedResolution struct {
	record AuthorityResolutionRecord
	use    AdmittedUse
}

func (result ReplayedResolution) Record() (AuthorityResolutionRecord, bool) {
	return result.record, result.valid()
}

func (result ReplayedResolution) AdmittedUse() (AdmittedUse, bool) {
	return result.use, result.valid()
}

func (result ReplayedResolution) valid() bool {
	if !result.use.valid() {
		return false
	}
	recordRef, recordRefOK := result.record.Ref()
	recordDigest, recordDigestOK := result.record.Digest()
	useRef, useDigest, useOK := result.use.Resolution()
	return recordRefOK && recordDigestOK && useOK &&
		recordRef.String() == useRef.String() &&
		recordDigest.String() == useDigest.String()
}

// ResolutionResult is the closed New | Replay | Denied algebra. New means the
// caller may persist the returned immutable pre-Work resolution record, but it
// carries no consumable use. Replay means an exact existing record was
// re-evaluated at the later transaction judgement time without renewing its
// checked_at or digest and may carry the sealed admitted use. Denied carries no
// capability and no admitted record.
type ResolutionResult struct {
	newResult    *NewResolution
	replayResult *ReplayedResolution
	deniedResult *DeniedResolution
}

func (result ResolutionResult) Kind() ResolutionResultKind {
	newValid := result.newResult != nil && result.newResult.valid()
	replayValid := result.replayResult != nil && result.replayResult.valid()
	deniedValid := result.deniedResult != nil && result.deniedResult.valid()
	variants := 0
	variants += boolAsInt(newValid)
	variants += 2 * boolAsInt(replayValid)
	variants += 4 * boolAsInt(deniedValid)
	kinds := map[int]ResolutionResultKind{
		1: ResolutionNew,
		2: ResolutionReplay,
		4: ResolutionDenied,
	}
	kind, ok := kinds[variants]
	if !ok {
		return ResolutionInvalid
	}
	return kind
}

func boolAsInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func (result ResolutionResult) New() (NewResolution, bool) {
	if result.Kind() != ResolutionNew {
		return NewResolution{}, false
	}
	return *result.newResult, true
}

func (result ResolutionResult) Replay() (ReplayedResolution, bool) {
	if result.Kind() != ResolutionReplay {
		return ReplayedResolution{}, false
	}
	return *result.replayResult, true
}

func (result ResolutionResult) Denied() (DeniedResolution, bool) {
	if result.Kind() != ResolutionDenied {
		return DeniedResolution{}, false
	}
	return *result.deniedResult, true
}

// EvaluateNewResolution performs the package-owned pre-Work A.2.5-style
// evaluation that may mint a new immutable resolution. It deliberately does
// not mint AdmittedUse: a final authority use must be judged after performed
// Work by exact replay of the persisted resolution. It performs no I/O and does
// not consume the single-use key.
func EvaluateNewResolution(
	ref ProfileDeclarationAuthorityResolutionRef,
	closure Closure,
	checkedAt time.Time,
) ResolutionResult {
	if !ref.valid() {
		return deniedResolution(
			DenialInvalidResolutionRef,
			"profile authority resolution ref is invalid",
		)
	}
	if !closure.valid() {
		return deniedResolution(
			DenialInvalidClosure,
			"exact source-native profile authority closure is unavailable",
		)
	}
	canonicalCheckedAt := canonicalTime(checkedAt)
	if canonicalCheckedAt.IsZero() {
		return deniedResolution(
			DenialInvalidCheckedAt,
			"authority evaluation requires a non-zero checked_at",
		)
	}
	if !closure.permission.state.validity.Contains(canonicalCheckedAt) {
		return deniedResolution(
			DenialPermissionNotCurrent,
			"profile declaration MAY permission is not current at checked_at",
		)
	}
	record, err := newAuthorityResolutionRecord(ref, closure, canonicalCheckedAt)
	if err != nil {
		return deniedResolution(DenialInvalidClosure, err.Error())
	}
	newResult := NewResolution{record: record}
	return ResolutionResult{newResult: &newResult}
}

// EvaluateReplayResolution re-evaluates an exact immutable resolution at the
// caller's current transaction judgement time. It preserves the original
// checked_at and digest and never renews authority. Row existence, transaction
// freshness, and single-use consumption remain persistence responsibilities.
func EvaluateReplayResolution(
	record AuthorityResolutionRecord,
	closure Closure,
	judgedAt time.Time,
) ResolutionResult {
	snapshot, recordOK := record.snapshot()
	if !recordOK {
		return deniedResolution(
			DenialReplayRecordInvalid,
			"exact immutable profile authority resolution is unavailable",
		)
	}
	if !closure.valid() {
		return deniedResolution(
			DenialInvalidClosure,
			"exact source-native profile authority closure is unavailable",
		)
	}
	expected, err := canonicalAuthorityResolution(
		snapshot.ref,
		closure,
		snapshot.checkedAt,
	)
	if err != nil || !sameAuthorityResolution(expected, snapshot) {
		return deniedResolution(
			DenialReplayBindingMismatch,
			"stored resolution and source-native closure have different exact bindings",
		)
	}
	canonicalJudgedAt := canonicalTime(judgedAt)
	if canonicalJudgedAt.IsZero() {
		return deniedResolution(
			DenialInvalidCheckedAt,
			"replay evaluation requires a non-zero judgement time",
		)
	}
	if canonicalJudgedAt.Before(snapshot.checkedAt) {
		return deniedResolution(
			DenialReplayBeforeResolution,
			"replay judgement cannot precede the original checked_at",
		)
	}
	if !snapshot.permissionValidity.Contains(canonicalJudgedAt) {
		return deniedResolution(
			DenialPermissionNotCurrent,
			"profile declaration MAY permission is not current at replay judgement",
		)
	}
	use, err := mintAdmittedUse(record, canonicalJudgedAt)
	if err != nil {
		return deniedResolution(DenialReplayRecordInvalid, err.Error())
	}
	replay := ReplayedResolution{record: record, use: use}
	return ResolutionResult{replayResult: &replay}
}

func sameAuthorityResolution(
	left authorityResolutionSnapshot,
	right authorityResolutionSnapshot,
) bool {
	return left.digest.String() == right.digest.String() &&
		slices.Equal(left.canonical, right.canonical)
}

func deniedResolution(
	code ResolutionDenialCode,
	detail string,
) ResolutionResult {
	denied := DeniedResolution{
		reasons: []ResolutionDenial{{code: code, detail: detail}},
	}
	return ResolutionResult{deniedResult: &denied}
}
