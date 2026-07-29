package projectprofile

import "fmt"

// ScopedCapabilityApplicability is the immutable projection of one capability
// from one exact ScopeID in the canonical profile matrix. It is not a second
// applicability rule: kind and missing basis are copied byte-for-byte from the
// matrix entry, while the payload digest keeps the projection tied to the
// matrix edition that produced it.
//
// Callers must supply a ScopeID even when the matrix has only one scope. This
// keeps mixed profiles from being collapsed by entry order or repository-wide
// software heuristics.
type ScopedCapabilityApplicability struct {
	scopeID              ScopeID
	capability           Capability
	kind                 CapabilityApplicabilityKind
	missingBasis         CapabilityApplicabilityMissingBasis
	profilePayloadDigest ContentDigest
}

func (value ScopedCapabilityApplicability) Valid() bool {
	if !value.scopeID.valid() ||
		!knownCapability(value.capability) ||
		!value.profilePayloadDigest.valid() {
		return false
	}
	switch value.kind {
	case CapabilityRequired, CapabilityNotApplicable:
		return value.missingBasis == ""
	case CapabilityUnderdetermined:
		// The source matrix entry has already validated the concrete basis.
		// This projection must not become a second registry of allowed bases.
		return value.missingBasis != ""
	default:
		return false
	}
}

func (value ScopedCapabilityApplicability) ScopeID() ScopeID {
	if !value.Valid() {
		return ScopeID{}
	}
	return value.scopeID
}

func (value ScopedCapabilityApplicability) Capability() Capability {
	if !value.Valid() {
		return ""
	}
	return value.capability
}

func (value ScopedCapabilityApplicability) Kind() CapabilityApplicabilityKind {
	if !value.Valid() {
		return ""
	}
	return value.kind
}

func (value ScopedCapabilityApplicability) MissingBasis() (
	CapabilityApplicabilityMissingBasis,
	bool,
) {
	if !value.Valid() || value.kind != CapabilityUnderdetermined {
		return "", false
	}
	return value.missingBasis, true
}

func (value ScopedCapabilityApplicability) ProfilePayloadDigest() ContentDigest {
	if !value.Valid() {
		return ContentDigest{}
	}
	return value.profilePayloadDigest
}

// ResolveScopedCapabilityApplicability projects one exact matrix cell without
// IO, profile inference, automatic scope selection, or applicability scoring.
func ResolveScopedCapabilityApplicability(
	matrix CapabilityApplicabilityMatrix,
	scopeID ScopeID,
	capability Capability,
) (ScopedCapabilityApplicability, error) {
	if !matrix.Valid() {
		return ScopedCapabilityApplicability{}, fmt.Errorf(
			"capability applicability matrix is invalid",
		)
	}
	if !scopeID.valid() {
		return ScopedCapabilityApplicability{}, fmt.Errorf(
			"exact capability ScopeID is invalid",
		)
	}
	if !knownCapability(capability) {
		return ScopedCapabilityApplicability{}, fmt.Errorf(
			"capability %q is not part of the canonical matrix",
			capability,
		)
	}
	entry, found := matrix.Entry(scopeID, capability)
	if !found {
		return ScopedCapabilityApplicability{}, fmt.Errorf(
			"exact ScopeID %q is not present in the capability matrix",
			scopeID.String(),
		)
	}
	value := ScopedCapabilityApplicability{
		scopeID:              entry.ScopeID(),
		capability:           entry.Capability(),
		kind:                 entry.Kind(),
		profilePayloadDigest: matrix.ProfilePayloadDigest(),
	}
	if missingBasis, present := entry.MissingBasis(); present {
		value.missingBasis = missingBasis
	}
	if !value.Valid() {
		return ScopedCapabilityApplicability{}, fmt.Errorf(
			"scoped capability applicability projection is invalid",
		)
	}
	return value, nil
}
