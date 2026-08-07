package projectprofile

import (
	"cmp"
	"fmt"
	"slices"
)

// RealizationClass is the closed profile-local classification used to derive
// capability applicability. It is not an FPF U.Kind and does not classify the
// repository itself.
type RealizationClass string

const (
	SoftwareRealizationClass    RealizationClass = "software"
	NonSoftwareRealizationClass RealizationClass = "non_software"
)

const (
	CodeDoctrineAndIndexCapability Capability = "code_doctrine_and_index"
	FPFQueryCapability             Capability = "fpf_query"
	ProcessChecksCapability        Capability = "process_checks"
	ProjectStatusCapability        Capability = "project_status"
	SoftwareSystemSpecCapability   Capability = "software_system_spec"
	SWEMethodPackCapability        Capability = "swe_methodpack"
	TargetSystemSpecCapability     Capability = "target_system_spec"
	TermMapCapability              Capability = "term_map"
	TypedProjectMemoryCapability   Capability = "typed_project_memory"
)

// CapabilityApplicabilityKind is a resolved per-scope result. Underdetermined
// belongs to the outer canonical-profile resolver: a matrix exists only after
// one integrity-valid declared profile has supplied exact scopes.
type CapabilityApplicabilityKind string

const (
	CapabilityRequired        CapabilityApplicabilityKind = "required"
	CapabilityNotApplicable   CapabilityApplicabilityKind = "not_applicable"
	CapabilityUnderdetermined CapabilityApplicabilityKind = "underdetermined"
)

// CapabilityApplicabilityMissingBasis names a basis that the admitted profile
// payload does not carry. The missing basis remains capability-local; it does
// not demote the canonical profile admission itself.
type CapabilityApplicabilityMissingBasis string

const (
	MissingAdmittedTargetSystemRelation CapabilityApplicabilityMissingBasis = "admitted_target_system_relation"
)

type capabilityPolicyOutcome struct {
	kind         CapabilityApplicabilityKind
	missingBasis CapabilityApplicabilityMissingBasis
}

type capabilityPolicy struct {
	capability  Capability
	software    capabilityPolicyOutcome
	nonSoftware capabilityPolicyOutcome
}

// capabilityPolicies is the single v1 applicability table. Its lexical
// capability order is part of the deterministic matrix representation.
var capabilityPolicies = []capabilityPolicy{
	{
		capability:  CodeDoctrineAndIndexCapability,
		software:    requiredCapability(),
		nonSoftware: notApplicableCapability(),
	},
	{
		capability:  FPFQueryCapability,
		software:    requiredCapability(),
		nonSoftware: requiredCapability(),
	},
	{
		capability:  ProcessChecksCapability,
		software:    requiredCapability(),
		nonSoftware: notApplicableCapability(),
	},
	{
		capability:  ProjectStatusCapability,
		software:    requiredCapability(),
		nonSoftware: requiredCapability(),
	},
	{
		capability:  SoftwareSystemSpecCapability,
		software:    requiredCapability(),
		nonSoftware: notApplicableCapability(),
	},
	{
		capability:  SWEMethodPackCapability,
		software:    requiredCapability(),
		nonSoftware: notApplicableCapability(),
	},
	{
		capability:  TargetSystemSpecCapability,
		software:    requiredCapability(),
		nonSoftware: requiredCapability(),
	},
	{
		capability:  TermMapCapability,
		software:    requiredCapability(),
		nonSoftware: requiredCapability(),
	},
	{
		capability:  TypedProjectMemoryCapability,
		software:    requiredCapability(),
		nonSoftware: requiredCapability(),
	},
}

// CapabilityApplicabilityEntry is one exact ScopeID/capability result. Fields
// are private so callers cannot combine a scope class with another policy
// result.
type CapabilityApplicabilityEntry struct {
	scopeID          ScopeID
	realizationClass RealizationClass
	capability       Capability
	kind             CapabilityApplicabilityKind
	missingBasis     CapabilityApplicabilityMissingBasis
}

func (entry CapabilityApplicabilityEntry) ScopeID() ScopeID {
	return entry.scopeID
}

func (entry CapabilityApplicabilityEntry) RealizationClass() RealizationClass {
	return entry.realizationClass
}

func (entry CapabilityApplicabilityEntry) Capability() Capability {
	return entry.capability
}

func (entry CapabilityApplicabilityEntry) Kind() CapabilityApplicabilityKind {
	return entry.kind
}

func (entry CapabilityApplicabilityEntry) MissingBasis() (CapabilityApplicabilityMissingBasis, bool) {
	if !entry.Valid() || entry.kind != CapabilityUnderdetermined {
		return "", false
	}
	return entry.missingBasis, true
}

func (entry CapabilityApplicabilityEntry) Valid() bool {
	return validateCapabilityApplicabilityEntry(entry) == nil
}

// CapabilityApplicabilityMatrix is the immutable pure projection of one
// canonical profile payload. Admission identity and ledger revision stay in
// the effect-boundary result that carries this matrix.
type CapabilityApplicabilityMatrix struct {
	payload       ProfileDeclarationPayload
	payloadDigest ContentDigest
	entries       []CapabilityApplicabilityEntry
}

func (matrix CapabilityApplicabilityMatrix) Valid() bool {
	return validateCapabilityApplicabilityMatrix(matrix) == nil
}

func (matrix CapabilityApplicabilityMatrix) ProfilePayloadDigest() ContentDigest {
	if !matrix.Valid() {
		return ContentDigest{}
	}
	return matrix.payloadDigest
}

func (matrix CapabilityApplicabilityMatrix) ScopeIDs() []ScopeID {
	if !matrix.Valid() {
		return nil
	}
	values := matrix.payload.Scopes().Values()
	return mapSliceV1Pure(values, func(scope RealizationScope) ScopeID {
		return scope.ScopeID()
	})
}

func (matrix CapabilityApplicabilityMatrix) Entries() []CapabilityApplicabilityEntry {
	if !matrix.Valid() {
		return nil
	}
	return append([]CapabilityApplicabilityEntry{}, matrix.entries...)
}

func (matrix CapabilityApplicabilityMatrix) Entry(
	scopeID ScopeID,
	capability Capability,
) (CapabilityApplicabilityEntry, bool) {
	if !matrix.Valid() || !scopeID.valid() || !knownCapability(capability) {
		return CapabilityApplicabilityEntry{}, false
	}
	probe := CapabilityApplicabilityEntry{
		scopeID:    scopeID,
		capability: capability,
	}
	index, found := slices.BinarySearchFunc(
		matrix.entries,
		probe,
		compareCapabilityApplicabilityEntryKey,
	)
	if !found {
		return CapabilityApplicabilityEntry{}, false
	}
	return matrix.entries[index], true
}

func KnownCapabilities() []Capability {
	return mapSliceV1Pure(capabilityPolicies, func(policy capabilityPolicy) Capability {
		return policy.capability
	})
}

// ResolveCapabilityApplicabilityMatrix performs no IO and grants no effect
// capability. Its input must already be the payload recovered from the
// canonical admission boundary.
func ResolveCapabilityApplicabilityMatrix(
	payload ProfileDeclarationPayload,
) (CapabilityApplicabilityMatrix, error) {
	canonical, err := NewProfileDeclarationPayload(payload.Scopes())
	if err != nil {
		return CapabilityApplicabilityMatrix{}, err
	}
	digest, err := DigestProfileDeclarationPayload(canonical)
	if err != nil {
		return CapabilityApplicabilityMatrix{}, err
	}
	entries, err := capabilityApplicabilityEntries(canonical)
	if err != nil {
		return CapabilityApplicabilityMatrix{}, err
	}
	matrix := CapabilityApplicabilityMatrix{
		payload:       canonical,
		payloadDigest: digest,
		entries:       entries,
	}
	if err := validateCapabilityApplicabilityMatrix(matrix); err != nil {
		return CapabilityApplicabilityMatrix{}, err
	}
	return matrix, nil
}

func capabilityApplicabilityEntries(
	payload ProfileDeclarationPayload,
) ([]CapabilityApplicabilityEntry, error) {
	scopes := payload.Scopes().Values()
	grouped, err := mapSliceV1(
		scopes,
		func(_ int, scope RealizationScope) ([]CapabilityApplicabilityEntry, error) {
			return capabilityApplicabilityEntriesForScope(scope)
		},
	)
	if err != nil {
		return nil, err
	}
	return slices.Concat(grouped...), nil
}

func capabilityApplicabilityEntriesForScope(
	scope RealizationScope,
) ([]CapabilityApplicabilityEntry, error) {
	realizationClass, err := realizationClassOf(scope)
	if err != nil {
		return nil, err
	}
	return mapSliceV1(
		capabilityPolicies,
		func(_ int, policy capabilityPolicy) (CapabilityApplicabilityEntry, error) {
			outcome, err := policy.applicabilityFor(realizationClass)
			if err != nil {
				return CapabilityApplicabilityEntry{}, err
			}
			entry := CapabilityApplicabilityEntry{
				scopeID:          scope.ScopeID(),
				realizationClass: realizationClass,
				capability:       policy.capability,
				kind:             outcome.kind,
				missingBasis:     outcome.missingBasis,
			}
			if err := validateCapabilityApplicabilityEntry(entry); err != nil {
				return CapabilityApplicabilityEntry{}, err
			}
			return entry, nil
		},
	)
}

func (policy capabilityPolicy) applicabilityFor(
	realizationClass RealizationClass,
) (capabilityPolicyOutcome, error) {
	switch realizationClass {
	case SoftwareRealizationClass:
		return policy.software, nil
	case NonSoftwareRealizationClass:
		return policy.nonSoftware, nil
	default:
		return capabilityPolicyOutcome{}, fmt.Errorf("unknown realization class %q", realizationClass)
	}
}

func requiredCapability() capabilityPolicyOutcome {
	return capabilityPolicyOutcome{kind: CapabilityRequired}
}

func notApplicableCapability() capabilityPolicyOutcome {
	return capabilityPolicyOutcome{kind: CapabilityNotApplicable}
}

func realizationClassOf(scope RealizationScope) (RealizationClass, error) {
	switch scope.(type) {
	case SoftwareRealization:
		return SoftwareRealizationClass, nil
	case NonSoftwareRealization:
		return NonSoftwareRealizationClass, nil
	default:
		return "", fmt.Errorf("unknown realization scope variant")
	}
}

func validateCapabilityApplicabilityMatrix(
	matrix CapabilityApplicabilityMatrix,
) error {
	canonical, err := NewProfileDeclarationPayload(matrix.payload.Scopes())
	if err != nil {
		return fmt.Errorf("capability matrix profile payload is invalid: %w", err)
	}
	digest, err := DigestProfileDeclarationPayload(canonical)
	if err != nil {
		return err
	}
	if matrix.payloadDigest != digest {
		return fmt.Errorf("capability matrix profile-payload digest is not canonical")
	}
	expected, err := capabilityApplicabilityEntries(canonical)
	if err != nil {
		return err
	}
	if !slices.Equal(matrix.entries, expected) {
		return fmt.Errorf("capability matrix entries are not canonical")
	}
	return nil
}

func validateCapabilityApplicabilityEntry(
	entry CapabilityApplicabilityEntry,
) error {
	if !entry.scopeID.valid() {
		return fmt.Errorf("capability applicability ScopeID is invalid")
	}
	policy, found := policyForCapability(entry.capability)
	if !found {
		return fmt.Errorf("capability applicability names an unknown capability")
	}
	expected, err := policy.applicabilityFor(entry.realizationClass)
	if err != nil {
		return err
	}
	actual := capabilityPolicyOutcome{
		kind:         entry.kind,
		missingBasis: entry.missingBasis,
	}
	if actual != expected {
		return fmt.Errorf("capability applicability kind does not match its realization class")
	}
	if entry.kind == CapabilityUnderdetermined &&
		entry.missingBasis != MissingAdmittedTargetSystemRelation {
		return fmt.Errorf("capability applicability missing basis is unknown")
	}
	return nil
}

func policyForCapability(capability Capability) (capabilityPolicy, bool) {
	index, found := slices.BinarySearchFunc(
		capabilityPolicies,
		capability,
		func(policy capabilityPolicy, target Capability) int {
			return cmp.Compare(string(policy.capability), string(target))
		},
	)
	if !found {
		return capabilityPolicy{}, false
	}
	return capabilityPolicies[index], true
}

func knownCapability(capability Capability) bool {
	_, found := policyForCapability(capability)
	return found
}

func compareCapabilityApplicabilityEntryKey(
	left CapabilityApplicabilityEntry,
	right CapabilityApplicabilityEntry,
) int {
	scopeOrder := cmp.Compare(left.scopeID.String(), right.scopeID.String())
	if scopeOrder != 0 {
		return scopeOrder
	}
	return cmp.Compare(string(left.capability), string(right.capability))
}
