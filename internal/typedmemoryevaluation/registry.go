package typedmemoryevaluation

import (
	"cmp"
	"fmt"
	"regexp"
	"slices"
	"unicode"
	"unicode/utf8"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	// MaxRegistryRegistrations bounds construction and lookup memory. A larger
	// runtime basis requires a new reviewed version rather than an unbounded
	// allocation from an external carrier.
	MaxRegistryRegistrations = 4 << 10

	maxRuleRefBytes            = 1 << 10
	maxIdentityCoordinateBytes = 4 << 10
)

var (
	exactSemanticVersion = regexp.MustCompile(
		`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-(?:0|[1-9][0-9]*|[0-9A-Za-z-]*[A-Za-z-][0-9A-Za-z-]*)(?:\.(?:0|[1-9][0-9]*|[0-9A-Za-z-]*[A-Za-z-][0-9A-Za-z-]*))*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`,
	)
	exactBuildEdition = regexp.MustCompile(
		`^build-[0-9]{8}\.(0|[1-9][0-9]*)(?:\.[0-9A-Za-z-]+)*$`,
	)
)

// MechanismRole is a closed evaluator-mechanism role. New roles are added only
// with a reviewed contract; arbitrary strings cannot silently widen X later.
type MechanismRole uint8

const (
	EvaluatorRole MechanismRole = iota + 1
)

func (role MechanismRole) String() string {
	switch role {
	case EvaluatorRole:
		return "evaluator"
	default:
		return ""
	}
}

func (role MechanismRole) valid() bool {
	return role.String() != ""
}

// MechanismIdentity is the exact, explicit coordinate of an evaluator
// implementation. It is not inferred from a Go type, function name, build ID,
// or reflection. CarrierRef and CarrierEdition are reused because they are the
// strong artifact coordinates already carried by MemberOf evaluation
// provenance.
type MechanismIdentity struct {
	artifact typedmemory.CarrierRef
	edition  typedmemory.CarrierEdition
	digest   typedmemory.SHA256Digest
	role     MechanismRole
}

func NewMechanismIdentity(
	artifact typedmemory.CarrierRef,
	edition typedmemory.CarrierEdition,
	digest typedmemory.SHA256Digest,
	role MechanismRole,
) (MechanismIdentity, error) {
	if err := validateArtifactRef(artifact); err != nil {
		return MechanismIdentity{}, err
	}
	if err := validateExactEdition(edition); err != nil {
		return MechanismIdentity{}, err
	}
	if err := validateDigest(digest); err != nil {
		return MechanismIdentity{}, err
	}
	if !role.valid() {
		return MechanismIdentity{}, fmt.Errorf("evaluator mechanism role is required or unsupported")
	}
	return MechanismIdentity{
		artifact: artifact,
		edition:  edition,
		digest:   digest,
		role:     role,
	}, nil
}

func (identity MechanismIdentity) ArtifactRef() typedmemory.CarrierRef {
	return identity.artifact
}

func (identity MechanismIdentity) Edition() typedmemory.CarrierEdition {
	return identity.edition
}

func (identity MechanismIdentity) Digest() typedmemory.SHA256Digest {
	return identity.digest
}

func (identity MechanismIdentity) Role() MechanismRole {
	return identity.role
}

func (identity MechanismIdentity) valid() bool {
	rebuilt, err := NewMechanismIdentity(
		identity.artifact,
		identity.edition,
		identity.digest,
		identity.role,
	)
	return err == nil && rebuilt == identity
}

// PureEvaluator is a typed, immutable callable. Production construction stays
// package-private so an arbitrary caller closure cannot claim a reviewed pure
// mechanism identity. Vetted package factories bind the supported evaluators.
type PureEvaluator[Input, Output any] struct {
	evaluate func(Input) (Output, error)
}

func newPureEvaluator[Input, Output any](
	evaluate func(Input) (Output, error),
) (PureEvaluator[Input, Output], error) {
	if evaluate == nil {
		return PureEvaluator[Input, Output]{}, fmt.Errorf("pure evaluator function is required")
	}
	return PureEvaluator[Input, Output]{evaluate: evaluate}, nil
}

func (evaluator PureEvaluator[Input, Output]) Evaluate(
	input Input,
) (Output, error) {
	if !evaluator.valid() {
		var zero Output
		return zero, fmt.Errorf("pure evaluator mechanism is invalid")
	}
	return evaluator.evaluate(input)
}

func (evaluator PureEvaluator[Input, Output]) valid() bool {
	return evaluator.evaluate != nil
}

// Registration binds one exact RuleRef to one typed mechanism and its
// separately supplied implementation identity. It carries no schema,
// ContextKindAvailability, project-head, or authority state. Registry presence
// supplies executable evaluation only; it does not make a kind available.
type Registration[Input, Output any] struct {
	rule      typedmemory.RuleRef
	identity  MechanismIdentity
	evaluator PureEvaluator[Input, Output]
}

func newRegistration[Input, Output any](
	rule typedmemory.RuleRef,
	identity MechanismIdentity,
	evaluator PureEvaluator[Input, Output],
) (Registration[Input, Output], error) {
	if err := validateRuleRef(rule); err != nil {
		return Registration[Input, Output]{}, err
	}
	if !identity.valid() {
		return Registration[Input, Output]{}, fmt.Errorf("evaluator mechanism identity is invalid")
	}
	if !evaluator.valid() {
		return Registration[Input, Output]{}, fmt.Errorf("pure evaluator mechanism is required")
	}
	return Registration[Input, Output]{
		rule:      rule,
		identity:  identity,
		evaluator: evaluator,
	}, nil
}

func (registration Registration[Input, Output]) RuleRef() typedmemory.RuleRef {
	return registration.rule
}

func (registration Registration[Input, Output]) Identity() MechanismIdentity {
	return registration.identity
}

func (registration Registration[Input, Output]) Evaluator() PureEvaluator[Input, Output] {
	return registration.evaluator
}

func (registration Registration[Input, Output]) valid() bool {
	rebuilt, err := newRegistration(
		registration.rule,
		registration.identity,
		registration.evaluator,
	)
	return err == nil && sameRegistrationCoordinates(rebuilt, registration)
}

type ConstructionConflictKind uint8

const (
	DuplicateRuleRefRegistration ConstructionConflictKind = iota + 1
	ConflictingMechanismIdentity
)

func (kind ConstructionConflictKind) String() string {
	switch kind {
	case DuplicateRuleRefRegistration:
		return "duplicate_rule_ref_registration"
	case ConflictingMechanismIdentity:
		return "conflicting_mechanism_identity"
	default:
		return ""
	}
}

// ConstructionConflict reports the deterministic first conflicting RuleRef in
// canonical RuleRef order. Identities are returned in canonical coordinate
// order and are never selected by caller input order.
type ConstructionConflict struct {
	kind       ConstructionConflictKind
	rule       typedmemory.RuleRef
	identities []MechanismIdentity
}

func (conflict ConstructionConflict) Error() string {
	return fmt.Sprintf(
		"evaluator registry %s for RuleRef %q",
		conflict.kind.String(),
		conflict.rule.String(),
	)
}

func (conflict ConstructionConflict) Kind() ConstructionConflictKind {
	return conflict.kind
}

func (conflict ConstructionConflict) RuleRef() typedmemory.RuleRef {
	return conflict.rule
}

func (conflict ConstructionConflict) Identities() []MechanismIdentity {
	return append([]MechanismIdentity(nil), conflict.identities...)
}

// Registry is an immutable, canonically ordered registry. It owns a defensive
// copy of construction input and exposes no post-construction registration
// method. A copied Registry remains read-only; Clone additionally creates an
// independent backing slice.
type Registry[Input, Output any] struct {
	registrations []Registration[Input, Output]
}

func NewRegistry[Input, Output any](
	registrations []Registration[Input, Output],
) (Registry[Input, Output], error) {
	if len(registrations) > MaxRegistryRegistrations {
		return Registry[Input, Output]{}, fmt.Errorf(
			"evaluator registry has %d registrations; maximum is %d",
			len(registrations),
			MaxRegistryRegistrations,
		)
	}
	normalized := append([]Registration[Input, Output](nil), registrations...)
	for index, registration := range normalized {
		if !registration.valid() {
			return Registry[Input, Output]{}, fmt.Errorf(
				"evaluator registry registration %d is invalid",
				index,
			)
		}
	}
	slices.SortFunc(normalized, compareRegistrations)
	if conflict := firstConstructionConflict(normalized); conflict != nil {
		return Registry[Input, Output]{}, *conflict
	}
	return Registry[Input, Output]{registrations: normalized}, nil
}

func (registry Registry[Input, Output]) Len() int {
	return len(registry.registrations)
}

func (registry Registry[Input, Output]) Registrations() []Registration[Input, Output] {
	return append([]Registration[Input, Output](nil), registry.registrations...)
}

func (registry Registry[Input, Output]) Clone() Registry[Input, Output] {
	return Registry[Input, Output]{registrations: registry.Registrations()}
}

type LookupResultKind uint8

const (
	FoundResult LookupResultKind = iota + 1
	MissingResult
	MismatchResult
)

func (kind LookupResultKind) String() string {
	switch kind {
	case FoundResult:
		return "found"
	case MissingResult:
		return "missing"
	case MismatchResult:
		return "mismatch"
	default:
		return ""
	}
}

// LookupResult is the closed exact-lookup result. Missing and Mismatch remain
// distinct so later composition can map either to an explicit
// Underdetermined basis instead of treating registry presence as admission.
type LookupResult[Input, Output any] interface {
	Kind() LookupResultKind
	RuleRef() typedmemory.RuleRef
	lookupResultVariant()
}

// Found is the validated exact-hit refinement. Its concrete implementation is
// private, so external callers cannot forge a zero value that claims Found.
type Found[Input, Output any] interface {
	LookupResult[Input, Output]
	Registration() Registration[Input, Output]
}

type found[Input, Output any] struct {
	registration Registration[Input, Output]
}

func (found[Input, Output]) Kind() LookupResultKind { return FoundResult }

func (result found[Input, Output]) RuleRef() typedmemory.RuleRef {
	return result.registration.rule
}

func (result found[Input, Output]) Registration() Registration[Input, Output] {
	return result.registration
}

func (found[Input, Output]) lookupResultVariant() {}

// Missing is the validated absent-RuleRef refinement. Its concrete
// implementation is private and always preserves the exact expected identity.
type Missing[Input, Output any] interface {
	LookupResult[Input, Output]
	ExpectedIdentity() MechanismIdentity
	missingLookupResult()
}

type missing[Input, Output any] struct {
	rule     typedmemory.RuleRef
	expected MechanismIdentity
}

func (missing[Input, Output]) Kind() LookupResultKind { return MissingResult }

func (result missing[Input, Output]) RuleRef() typedmemory.RuleRef {
	return result.rule
}

func (result missing[Input, Output]) ExpectedIdentity() MechanismIdentity {
	return result.expected
}

func (missing[Input, Output]) lookupResultVariant() {}

func (missing[Input, Output]) missingLookupResult() {}

// Mismatch is the validated exact-RuleRef/wrong-identity refinement. Its
// concrete implementation is private and preserves both coordinates.
type Mismatch[Input, Output any] interface {
	LookupResult[Input, Output]
	RegisteredIdentity() MechanismIdentity
	ExpectedIdentity() MechanismIdentity
	mismatchLookupResult()
}

type mismatch[Input, Output any] struct {
	registration Registration[Input, Output]
	expected     MechanismIdentity
}

func (mismatch[Input, Output]) Kind() LookupResultKind { return MismatchResult }

func (result mismatch[Input, Output]) RuleRef() typedmemory.RuleRef {
	return result.registration.rule
}

func (result mismatch[Input, Output]) RegisteredIdentity() MechanismIdentity {
	return result.registration.identity
}

func (result mismatch[Input, Output]) ExpectedIdentity() MechanismIdentity {
	return result.expected
}

func (mismatch[Input, Output]) lookupResultVariant() {}

func (mismatch[Input, Output]) mismatchLookupResult() {}

// Lookup resolves only the exact RuleRef and exact expected mechanism
// identity. Invalid weak-boundary inputs are construction errors; every valid
// lookup returns one non-nil Found, Missing, or Mismatch variant.
func (registry Registry[Input, Output]) Lookup(
	rule typedmemory.RuleRef,
	expected MechanismIdentity,
) (LookupResult[Input, Output], error) {
	if err := validateRuleRef(rule); err != nil {
		return nil, err
	}
	if !expected.valid() {
		return nil, fmt.Errorf("expected evaluator mechanism identity is invalid")
	}
	index, exactRuleFound := slices.BinarySearchFunc(
		registry.registrations,
		rule,
		compareRegistrationRule,
	)
	if !exactRuleFound {
		return missing[Input, Output]{rule: rule, expected: expected}, nil
	}
	registration := registry.registrations[index]
	if registration.identity != expected {
		return mismatch[Input, Output]{
			registration: registration,
			expected:     expected,
		}, nil
	}
	return found[Input, Output]{registration: registration}, nil
}

func compareRegistrations[Input, Output any](
	left Registration[Input, Output],
	right Registration[Input, Output],
) int {
	if order := cmp.Compare(left.rule.String(), right.rule.String()); order != 0 {
		return order
	}
	return compareMechanismIdentities(left.identity, right.identity)
}

func compareRegistrationRule[Input, Output any](
	registration Registration[Input, Output],
	rule typedmemory.RuleRef,
) int {
	return cmp.Compare(registration.rule.String(), rule.String())
}

func compareMechanismIdentities(left, right MechanismIdentity) int {
	if order := cmp.Compare(left.artifact.String(), right.artifact.String()); order != 0 {
		return order
	}
	if order := cmp.Compare(left.edition.String(), right.edition.String()); order != 0 {
		return order
	}
	if order := cmp.Compare(left.digest.String(), right.digest.String()); order != 0 {
		return order
	}
	return cmp.Compare(left.role.String(), right.role.String())
}

func sameRegistrationCoordinates[Input, Output any](
	left Registration[Input, Output],
	right Registration[Input, Output],
) bool {
	return left.rule == right.rule && left.identity == right.identity
}

func firstConstructionConflict[Input, Output any](
	registrations []Registration[Input, Output],
) *ConstructionConflict {
	for start := 0; start < len(registrations); {
		end := registrationGroupEnd(registrations, start)
		if end-start > 1 {
			return buildConstructionConflict(registrations[start:end])
		}
		start = end
	}
	return nil
}

func registrationGroupEnd[Input, Output any](
	registrations []Registration[Input, Output],
	start int,
) int {
	rule := registrations[start].rule
	end := start + 1
	for end < len(registrations) && registrations[end].rule == rule {
		end++
	}
	return end
}

func buildConstructionConflict[Input, Output any](
	group []Registration[Input, Output],
) *ConstructionConflict {
	identities := make([]MechanismIdentity, 0, len(group))
	for _, registration := range group {
		if len(identities) == 0 || identities[len(identities)-1] != registration.identity {
			identities = append(identities, registration.identity)
		}
	}
	kind := DuplicateRuleRefRegistration
	if len(identities) > 1 {
		kind = ConflictingMechanismIdentity
	}
	return &ConstructionConflict{
		kind:       kind,
		rule:       group[0].rule,
		identities: identities,
	}
}

func validateRuleRef(rule typedmemory.RuleRef) error {
	value := rule.String()
	if err := validateBoundedUTF8("evaluator RuleRef", value, maxRuleRefBytes); err != nil {
		return err
	}
	rebuilt, err := typedmemory.NewRuleRef(value)
	if err != nil || rebuilt != rule {
		return fmt.Errorf("evaluator RuleRef is invalid")
	}
	return nil
}

func validateArtifactRef(artifact typedmemory.CarrierRef) error {
	value := artifact.String()
	if err := validateBoundedUTF8(
		"evaluator artifact reference",
		value,
		maxIdentityCoordinateBytes,
	); err != nil {
		return err
	}
	rebuilt, err := typedmemory.NewCarrierRef(value)
	if err != nil || rebuilt != artifact {
		return fmt.Errorf("evaluator artifact reference is invalid")
	}
	return nil
}

func validateExactEdition(edition typedmemory.CarrierEdition) error {
	value := edition.String()
	if err := validateBoundedUTF8(
		"evaluator artifact edition",
		value,
		maxIdentityCoordinateBytes,
	); err != nil {
		return err
	}
	rebuilt, err := typedmemory.NewCarrierEdition(value)
	if err != nil || rebuilt != edition {
		return fmt.Errorf("evaluator artifact edition is invalid")
	}
	if !exactMechanismEdition(value) {
		return fmt.Errorf(
			"evaluator artifact edition must be an exact semantic version or immutable build edition",
		)
	}
	return nil
}

func validateDigest(digest typedmemory.SHA256Digest) error {
	rebuilt, err := typedmemory.NewSHA256Digest(digest.String())
	if err != nil || rebuilt != digest {
		return fmt.Errorf("evaluator artifact digest is invalid")
	}
	return nil
}

func validateBoundedUTF8(name, value string, maximum int) error {
	if value == "" {
		return fmt.Errorf("%s is required", name)
	}
	if !utf8.ValidString(value) {
		return fmt.Errorf("%s must be valid UTF-8", name)
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return fmt.Errorf("%s contains a control character", name)
		}
	}
	if len(value) > maximum {
		return fmt.Errorf("%s exceeds %d bytes", name, maximum)
	}
	return nil
}

func exactMechanismEdition(value string) bool {
	return exactSemanticVersion.MatchString(value) || exactBuildEdition.MatchString(value)
}
