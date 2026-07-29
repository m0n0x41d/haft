package authority

import "fmt"

// WorkRef is a strong reference to one dated performed U.Work occurrence. It
// is deliberately distinct from SpeechActRef even when both references name
// the same communicative occurrence.
type WorkRef struct{ value string }

func NewWorkRef(raw string) (WorkRef, error) {
	value, err := parseAuthorityReference("Work ref", raw)
	return WorkRef{value: value}, err
}

func (value WorkRef) String() string { return value.value }
func (value WorkRef) valid() bool    { return validAuthorityReference(value.value) }

type RoleRef struct{ value string }

func NewRoleRef(raw string) (RoleRef, error) {
	value, err := parseAuthorityReference("Role ref", raw)
	return RoleRef{value: value}, err
}

func (value RoleRef) String() string { return value.value }

type DescriptionRefKind string

const (
	DescriptionRefClaimID  DescriptionRefKind = "claim_id"
	DescriptionRefEpisteme DescriptionRefKind = "episteme"
)

// DescriptionRef is the closed ClaimIdRef | EpistemeRef representation
// authorized by the active TermMap. The variant is explicit so an arbitrary
// carrier, SpeechAct, or Work reference cannot be parsed as a description.
type DescriptionRef struct {
	kind  DescriptionRefKind
	value string
}

func NewClaimIDDescriptionRef(raw string) (DescriptionRef, error) {
	return newDescriptionRef(DescriptionRefClaimID, raw)
}

func NewEpistemeDescriptionRef(raw string) (DescriptionRef, error) {
	return newDescriptionRef(DescriptionRefEpisteme, raw)
}

func newDescriptionRef(kind DescriptionRefKind, raw string) (DescriptionRef, error) {
	if kind != DescriptionRefClaimID && kind != DescriptionRefEpisteme {
		return DescriptionRef{}, fmt.Errorf("DescriptionRef kind is not admitted")
	}
	value, err := parseAuthorityReference("Description ref", raw)
	if err != nil {
		return DescriptionRef{}, err
	}
	return DescriptionRef{kind: kind, value: value}, nil
}

func (value DescriptionRef) Kind() DescriptionRefKind { return value.kind }
func (value DescriptionRef) String() string           { return value.value }
func (value DescriptionRef) valid() bool {
	return (value.kind == DescriptionRefClaimID || value.kind == DescriptionRefEpisteme) &&
		validAuthorityReference(value.value)
}

type ResourceLedgerRef struct{ value string }

func NewResourceLedgerRef(raw string) (ResourceLedgerRef, error) {
	value, err := parseAuthorityReference("resource-ledger ref", raw)
	return ResourceLedgerRef{value: value}, err
}

func (value ResourceLedgerRef) String() string { return value.value }
func (value ResourceLedgerRef) valid() bool    { return validAuthorityReference(value.value) }

type AcceptancePostureRef struct{ value string }

func NewAcceptancePostureRef(raw string) (AcceptancePostureRef, error) {
	value, err := parseAuthorityReference("acceptance-posture ref", raw)
	return AcceptancePostureRef{value: value}, err
}

func (value AcceptancePostureRef) String() string { return value.value }
func (value AcceptancePostureRef) valid() bool {
	return validAuthorityReference(value.value)
}

type AuditTraceRef struct{ value string }

func NewAuditTraceRef(raw string) (AuditTraceRef, error) {
	value, err := parseAuthorityReference("audit-trace ref", raw)
	return AuditTraceRef{value: value}, err
}

func (value AuditTraceRef) String() string { return value.value }
func (value AuditTraceRef) valid() bool    { return validAuthorityReference(value.value) }

// ObservableCarrierBinding binds one observable content carrier to exact
// bytes. It does not say that the carrier is the description it bears.
type ObservableCarrierBinding struct {
	ref    CarrierRef
	digest Digest
}

func NewObservableCarrierBinding(
	ref CarrierRef,
	digest Digest,
) (ObservableCarrierBinding, error) {
	binding := ObservableCarrierBinding{ref: ref, digest: digest}
	if !binding.valid() {
		return ObservableCarrierBinding{}, fmt.Errorf(
			"observable carrier binding requires a canonical ref and digest",
		)
	}
	return binding, nil
}

func (binding ObservableCarrierBinding) Ref() CarrierRef { return binding.ref }
func (binding ObservableCarrierBinding) Digest() Digest  { return binding.digest }
func (binding ObservableCarrierBinding) valid() bool {
	return binding.ref.valid() && binding.digest.valid()
}
