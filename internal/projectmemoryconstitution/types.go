// Package projectmemoryconstitution evaluates whether one exact ClaimGraph,
// resolved EntityOfConcern, and effective project-memory ReferenceScheme
// constitute one project-memory episteme. It is a pure semantic core: it does
// not resolve references, select a TypeEnv, inspect a registry, read storage,
// or attach grounding, view, publication, carrier, or temporal coordinates.
package projectmemoryconstitution

import (
	"bytes"
	"cmp"
	"fmt"
	"slices"

	"github.com/m0n0x41d/haft/internal/projectmemoryreferencescheme"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// CanonicalClaimGraph is one exact ClaimGraph value together with the
// canonical bytes produced by ClaimGraphCodecV1. The codec and its shape are
// construction-time verification mechanisms; neither is retained as an
// episteme identity coordinate.
type CanonicalClaimGraph struct {
	value     typedmemory.ClaimGraphValue
	canonical []byte
}

func NewCanonicalClaimGraph(
	value typedmemory.ClaimGraphValue,
	codec typedmemory.ClaimGraphCodecV1,
) (CanonicalClaimGraph, error) {
	encoded := codec.EncodeInput(value)
	canonical, ok := encoded.(typedmemory.CanonicalizedCodecValue)
	if !ok {
		return CanonicalClaimGraph{}, fmt.Errorf(
			"canonical ClaimGraph requires the exact closed ClaimGraph value variant",
		)
	}
	roundTripResult := codec.Canonicalize(
		codec.Shape(),
		canonical.CanonicalBytes(),
	)
	roundTrip, ok := roundTripResult.(typedmemory.CanonicalizedCodecValue)
	if !ok {
		return CanonicalClaimGraph{}, fmt.Errorf(
			"canonical ClaimGraph bytes failed exact codec verification",
		)
	}
	if !bytes.Equal(canonical.CanonicalBytes(), roundTrip.CanonicalBytes()) {
		return CanonicalClaimGraph{}, fmt.Errorf(
			"canonical ClaimGraph bytes changed during exact codec verification",
		)
	}
	exactValue, ok := roundTrip.Value().(typedmemory.ClaimGraphValue)
	if !ok {
		return CanonicalClaimGraph{}, fmt.Errorf(
			"canonical ClaimGraph codec returned another value kind",
		)
	}
	return CanonicalClaimGraph{
		value:     exactValue,
		canonical: roundTrip.CanonicalBytes(),
	}, nil
}

func (graph CanonicalClaimGraph) Value() typedmemory.ClaimGraphValue {
	return graph.value
}

func (graph CanonicalClaimGraph) CanonicalBytes() []byte {
	return bytes.Clone(graph.canonical)
}

func (graph CanonicalClaimGraph) valid() bool {
	return graph.value != nil && len(graph.canonical) > 0
}

// ResolvedEntityOfConcern keeps only the exact resolved entity identity. The
// reference-resolution basis is provenance of resolution, not an episteme
// identity discriminator, and is deliberately not retained here.
type ResolvedEntityOfConcern struct {
	entity typedmemory.EntityID
}

func NewResolvedEntityOfConcern(
	entity typedmemory.EntityID,
) (ResolvedEntityOfConcern, error) {
	rebuilt, err := typedmemory.NewEntityID(entity.String())
	if err != nil || rebuilt != entity {
		return ResolvedEntityOfConcern{}, fmt.Errorf(
			"resolved EntityOfConcern requires an exact EntityID",
		)
	}
	return ResolvedEntityOfConcern{entity: rebuilt}, nil
}

func (concern ResolvedEntityOfConcern) EntityID() typedmemory.EntityID {
	return concern.entity
}

func (concern ResolvedEntityOfConcern) valid() bool {
	rebuilt, err := typedmemory.NewEntityID(concern.entity.String())
	return err == nil && rebuilt == concern.entity
}

// RoleOutcomeInput binds an explicit role evaluation to the exact scheme and
// exact rule pins it evaluated. Supplying a scheme or finding its mechanisms
// in a registry is not an outcome.
type RoleOutcomeInput struct {
	SchemeDigest      projectmemoryreferencescheme.ReferenceSchemeDigest
	Role              projectmemoryreferencescheme.SemanticRole
	Contract          projectmemoryreferencescheme.RuntimeContract
	EvaluatedRulePins []projectmemoryreferencescheme.ExactRulePin
}

type roleOutcomeState struct {
	schemeDigest projectmemoryreferencescheme.ReferenceSchemeDigest
	role         projectmemoryreferencescheme.SemanticRole
	contract     projectmemoryreferencescheme.RuntimeContract
	rulePins     []projectmemoryreferencescheme.ExactRulePin
}

// RoleOutcome is a closed role-evaluation result. Complete, contradicted, and
// incomplete execution are distinct; none is inferred from mechanism
// registration or from a RuntimeNotRequired rule pin.
type RoleOutcome interface {
	SchemeDigest() projectmemoryreferencescheme.ReferenceSchemeDigest
	Role() projectmemoryreferencescheme.SemanticRole
	Contract() projectmemoryreferencescheme.RuntimeContract
	EvaluatedRulePins() []projectmemoryreferencescheme.ExactRulePin
	roleOutcomeVariant()
}

type RoleSatisfied struct {
	state roleOutcomeState
}

func NewRoleSatisfied(input RoleOutcomeInput) (RoleSatisfied, error) {
	state, err := newRoleOutcomeState(input)
	if err != nil {
		return RoleSatisfied{}, err
	}
	return RoleSatisfied{state: state}, nil
}

func (outcome RoleSatisfied) SchemeDigest() projectmemoryreferencescheme.ReferenceSchemeDigest {
	return outcome.state.schemeDigest
}

func (outcome RoleSatisfied) Role() projectmemoryreferencescheme.SemanticRole {
	return outcome.state.role
}

func (outcome RoleSatisfied) Contract() projectmemoryreferencescheme.RuntimeContract {
	return outcome.state.contract
}

func (outcome RoleSatisfied) EvaluatedRulePins() []projectmemoryreferencescheme.ExactRulePin {
	return slices.Clone(outcome.state.rulePins)
}

func (RoleSatisfied) roleOutcomeVariant() {}

type RoleContradicted struct {
	state roleOutcomeState
}

func NewRoleContradicted(input RoleOutcomeInput) (RoleContradicted, error) {
	state, err := newRoleOutcomeState(input)
	if err != nil {
		return RoleContradicted{}, err
	}
	return RoleContradicted{state: state}, nil
}

func (outcome RoleContradicted) SchemeDigest() projectmemoryreferencescheme.ReferenceSchemeDigest {
	return outcome.state.schemeDigest
}

func (outcome RoleContradicted) Role() projectmemoryreferencescheme.SemanticRole {
	return outcome.state.role
}

func (outcome RoleContradicted) Contract() projectmemoryreferencescheme.RuntimeContract {
	return outcome.state.contract
}

func (outcome RoleContradicted) EvaluatedRulePins() []projectmemoryreferencescheme.ExactRulePin {
	return slices.Clone(outcome.state.rulePins)
}

func (RoleContradicted) roleOutcomeVariant() {}

type RoleUnderdetermined struct {
	state roleOutcomeState
}

func NewRoleUnderdetermined(
	input RoleOutcomeInput,
) (RoleUnderdetermined, error) {
	state, err := newRoleOutcomeState(input)
	if err != nil {
		return RoleUnderdetermined{}, err
	}
	return RoleUnderdetermined{state: state}, nil
}

func (outcome RoleUnderdetermined) SchemeDigest() projectmemoryreferencescheme.ReferenceSchemeDigest {
	return outcome.state.schemeDigest
}

func (outcome RoleUnderdetermined) Role() projectmemoryreferencescheme.SemanticRole {
	return outcome.state.role
}

func (outcome RoleUnderdetermined) Contract() projectmemoryreferencescheme.RuntimeContract {
	return outcome.state.contract
}

func (outcome RoleUnderdetermined) EvaluatedRulePins() []projectmemoryreferencescheme.ExactRulePin {
	return slices.Clone(outcome.state.rulePins)
}

func (RoleUnderdetermined) roleOutcomeVariant() {}

func newRoleOutcomeState(input RoleOutcomeInput) (roleOutcomeState, error) {
	contract, err := projectmemoryreferencescheme.RuntimeContractForRole(input.Role)
	if err != nil {
		return roleOutcomeState{}, fmt.Errorf("role evaluation: %w", err)
	}
	if input.Contract != contract {
		return roleOutcomeState{}, fmt.Errorf(
			"role evaluation contract %q does not match role %q contract %q",
			input.Contract.String(),
			input.Role,
			contract.String(),
		)
	}
	digest, err := projectmemoryreferencescheme.ParseReferenceSchemeDigest(
		input.SchemeDigest.String(),
	)
	if err != nil || digest != input.SchemeDigest {
		return roleOutcomeState{}, fmt.Errorf(
			"role evaluation requires an exact ReferenceScheme digest",
		)
	}
	normalized, err := normalizeOutcomeRulePins(
		input.Role,
		input.EvaluatedRulePins,
	)
	if err != nil {
		return roleOutcomeState{}, err
	}
	return roleOutcomeState{
		schemeDigest: digest,
		role:         input.Role,
		contract:     contract,
		rulePins:     normalized,
	}, nil
}

func normalizeOutcomeRulePins(
	role projectmemoryreferencescheme.SemanticRole,
	pins []projectmemoryreferencescheme.ExactRulePin,
) ([]projectmemoryreferencescheme.ExactRulePin, error) {
	if len(pins) == 0 {
		return nil, fmt.Errorf(
			"role %q evaluation requires at least one exact rule pin",
			role,
		)
	}
	normalized := slices.Clone(pins)
	invalidIndex := slices.IndexFunc(
		normalized,
		func(pin projectmemoryreferencescheme.ExactRulePin) bool {
			return !outcomeRulePinIsExactForRole(role, pin)
		},
	)
	if invalidIndex >= 0 {
		return nil, fmt.Errorf(
			"role %q evaluation rule pin %d is malformed or belongs to another role",
			role,
			invalidIndex,
		)
	}
	slices.SortFunc(
		normalized,
		func(
			left projectmemoryreferencescheme.ExactRulePin,
			right projectmemoryreferencescheme.ExactRulePin,
		) int {
			return cmp.Compare(left.Rule().String(), right.Rule().String())
		},
	)
	unique := slices.CompactFunc(
		slices.Clone(normalized),
		func(
			left projectmemoryreferencescheme.ExactRulePin,
			right projectmemoryreferencescheme.ExactRulePin,
		) bool {
			return left.Rule() == right.Rule()
		},
	)
	if len(unique) != len(normalized) {
		return nil, fmt.Errorf(
			"role %q evaluation contains duplicate RuleRef",
			role,
		)
	}
	return normalized, nil
}

func outcomeRulePinIsExactForRole(
	role projectmemoryreferencescheme.SemanticRole,
	pin projectmemoryreferencescheme.ExactRulePin,
) bool {
	rebuilt, err := projectmemoryreferencescheme.NewExactRulePin(
		projectmemoryreferencescheme.ExactRulePinInput{
			Role:    pin.Role(),
			Rule:    pin.Rule(),
			Source:  pin.Source(),
			Runtime: pin.Runtime(),
		},
	)
	return err == nil && rebuilt == pin && rebuilt.Role() == role
}

// RuntimeEvaluationBasis is the closed availability posture for aggregate
// role outcomes. A provided basis can still be incomplete or malformed; the
// evaluator classifies that state without converting absence into success.
type RuntimeEvaluationBasis interface {
	runtimeEvaluationBasisVariant()
}

type MissingRuntimeEvaluationBasis struct{}

func NewMissingRuntimeEvaluationBasis() MissingRuntimeEvaluationBasis {
	return MissingRuntimeEvaluationBasis{}
}

func (MissingRuntimeEvaluationBasis) runtimeEvaluationBasisVariant() {}

type RoleRuntimeEvaluationBasis struct {
	outcomes []RoleOutcome
}

// NewRoleRuntimeEvaluationBasis owns and canonicalizes supplied outcomes in
// semantic role order. Duplicate roles are retained so Evaluate can reject
// the malformed observation as Invalid rather than silently deduplicating it.
func NewRoleRuntimeEvaluationBasis(
	outcomes []RoleOutcome,
) RoleRuntimeEvaluationBasis {
	owned := slices.Clone(outcomes)
	slices.SortStableFunc(owned, compareRoleOutcomes)
	return RoleRuntimeEvaluationBasis{outcomes: owned}
}

func (basis RoleRuntimeEvaluationBasis) Outcomes() []RoleOutcome {
	return slices.Clone(basis.outcomes)
}

func (RoleRuntimeEvaluationBasis) runtimeEvaluationBasisVariant() {}

func compareRoleOutcomes(left RoleOutcome, right RoleOutcome) int {
	return roleRank(roleOf(left)) - roleRank(roleOf(right))
}

func roleOf(outcome RoleOutcome) projectmemoryreferencescheme.SemanticRole {
	if outcome == nil {
		return projectmemoryreferencescheme.SemanticRole("")
	}
	return outcome.Role()
}

func roleRank(role projectmemoryreferencescheme.SemanticRole) int {
	switch role {
	case projectmemoryreferencescheme.SemanticRoleDesignation:
		return 1
	case projectmemoryreferencescheme.SemanticRoleInterpretation:
		return 2
	case projectmemoryreferencescheme.SemanticRoleMeasurement:
		return 3
	case projectmemoryreferencescheme.SemanticRoleEvaluation:
		return 4
	default:
		return 5
	}
}

type Reason string

const (
	ReasonClaimGraphBasisInvalid             Reason = "claim_graph_basis_invalid"
	ReasonEntityOfConcernBasisInvalid        Reason = "entity_of_concern_basis_invalid"
	ReasonReferenceSchemeBasisInvalid        Reason = "reference_scheme_basis_invalid"
	ReasonReferenceSchemeRuntimeBasisInvalid Reason = "reference_scheme_runtime_basis_invalid"
	ReasonReferenceSchemeRuntimeBasisMissing Reason = "reference_scheme_runtime_basis_missing"
	ReasonEpistemeConstitutionNotSatisfied   Reason = "episteme_constitution_not_satisfied"
)

func (reason Reason) String() string { return string(reason) }

// Result is the closed Invalid | Underdetermined | Satisfied constitution
// algebra. Coordinate is intentionally absent from this common interface.
type Result interface {
	resultVariant()
}

type Invalid struct {
	reason Reason
}

func (result Invalid) Reason() Reason { return result.reason }

func (Invalid) resultVariant() {}

type Underdetermined struct {
	reason Reason
}

func (result Underdetermined) Reason() Reason { return result.reason }

func (Underdetermined) resultVariant() {}

type Satisfied struct {
	coordinate EpistemeCoordinate
}

func (result Satisfied) Coordinate() EpistemeCoordinate {
	return result.coordinate
}

func (Satisfied) resultVariant() {}

// EpistemeCoordinate is the immutable C.2.1 identity triple. Exact canonical
// ClaimGraph bytes are retained as a comparable string so equality is exact
// and collision-free. Grounding, view, publication, carrier, GammaTime,
// selected TypeEnv, graph revision, and reference-resolution basis have no
// coordinate here and cannot reidentify an episteme through this type.
type EpistemeCoordinate struct {
	claimGraphCanonical   string
	entityID              typedmemory.EntityID
	referenceSchemeDigest projectmemoryreferencescheme.ReferenceSchemeDigest
}

func newEpistemeCoordinate(
	graph CanonicalClaimGraph,
	concern ResolvedEntityOfConcern,
	scheme projectmemoryreferencescheme.ProjectMemoryReferenceSchemeV1,
) EpistemeCoordinate {
	return EpistemeCoordinate{
		claimGraphCanonical:   string(graph.CanonicalBytes()),
		entityID:              concern.EntityID(),
		referenceSchemeDigest: scheme.Digest(),
	}
}

func (coordinate EpistemeCoordinate) ClaimGraphCanonicalBytes() []byte {
	return []byte(coordinate.claimGraphCanonical)
}

func (coordinate EpistemeCoordinate) EntityID() typedmemory.EntityID {
	return coordinate.entityID
}

func (coordinate EpistemeCoordinate) ReferenceSchemeDigest() projectmemoryreferencescheme.ReferenceSchemeDigest {
	return coordinate.referenceSchemeDigest
}

// EvaluationInput contains only the three constitution participants and
// explicit role outcomes. Registry presence is intentionally inexpressible;
// a shell must execute mechanisms before constructing RoleOutcomes.
type EvaluationInput struct {
	ClaimGraph      CanonicalClaimGraph
	EntityOfConcern ResolvedEntityOfConcern
	ReferenceScheme projectmemoryreferencescheme.ProjectMemoryReferenceSchemeV1
	RuntimeBasis    RuntimeEvaluationBasis
}

// Evaluate returns the smallest honest C.2.1 constitution result. Satisfied
// means only that the exact constitution predicate was satisfied for this
// triple. It asserts no claim truth, evidence, authority, admission,
// publication, grounding, or world-side relation occurrence.
func Evaluate(input EvaluationInput) Result {
	staticFailure := evaluateStaticBasis(input)
	if staticFailure != nil {
		return staticFailure
	}
	return evaluateRuntimeBasis(
		input.ClaimGraph,
		input.EntityOfConcern,
		input.ReferenceScheme,
		input.RuntimeBasis,
	)
}

func evaluateStaticBasis(input EvaluationInput) Result {
	switch {
	case !input.ClaimGraph.valid():
		return Invalid{reason: ReasonClaimGraphBasisInvalid}
	case !input.EntityOfConcern.valid():
		return Invalid{reason: ReasonEntityOfConcernBasisInvalid}
	case input.ReferenceScheme.Verify() != nil:
		return Invalid{reason: ReasonReferenceSchemeBasisInvalid}
	default:
		return nil
	}
}

func evaluateRuntimeBasis(
	graph CanonicalClaimGraph,
	concern ResolvedEntityOfConcern,
	scheme projectmemoryreferencescheme.ProjectMemoryReferenceSchemeV1,
	basis RuntimeEvaluationBasis,
) Result {
	switch value := basis.(type) {
	case nil:
		return Underdetermined{reason: ReasonReferenceSchemeRuntimeBasisMissing}
	case MissingRuntimeEvaluationBasis:
		return Underdetermined{reason: ReasonReferenceSchemeRuntimeBasisMissing}
	case RoleRuntimeEvaluationBasis:
		return evaluateRoleOutcomes(graph, concern, scheme, value.Outcomes())
	default:
		return Invalid{reason: ReasonReferenceSchemeRuntimeBasisInvalid}
	}
}

func evaluateRoleOutcomes(
	graph CanonicalClaimGraph,
	concern ResolvedEntityOfConcern,
	scheme projectmemoryreferencescheme.ProjectMemoryReferenceSchemeV1,
	outcomes []RoleOutcome,
) Result {
	variantFailure := validateRoleOutcomeVariants(outcomes)
	if variantFailure != nil {
		return variantFailure
	}
	duplicateFailure := validateDistinctOutcomeRoles(outcomes)
	if duplicateFailure != nil {
		return duplicateFailure
	}
	matchingFailure := validateOutcomesMatchScheme(scheme, outcomes)
	if matchingFailure != nil {
		return matchingFailure
	}
	if len(outcomes) != len(projectmemoryreferencescheme.RoleRuntimeContracts()) {
		return Underdetermined{reason: ReasonReferenceSchemeRuntimeBasisMissing}
	}
	return aggregateRoleOutcomePostures(graph, concern, scheme, outcomes)
}

func validateRoleOutcomeVariants(outcomes []RoleOutcome) Result {
	invalidIndex := slices.IndexFunc(outcomes, func(outcome RoleOutcome) bool {
		switch outcome.(type) {
		case RoleSatisfied, RoleContradicted, RoleUnderdetermined:
			return false
		default:
			return true
		}
	})
	if invalidIndex >= 0 {
		return Invalid{reason: ReasonReferenceSchemeRuntimeBasisInvalid}
	}
	return nil
}

func validateDistinctOutcomeRoles(outcomes []RoleOutcome) Result {
	unique := slices.CompactFunc(
		slices.Clone(outcomes),
		func(left RoleOutcome, right RoleOutcome) bool {
			return roleOf(left) == roleOf(right)
		},
	)
	if len(unique) != len(outcomes) {
		return Invalid{reason: ReasonReferenceSchemeRuntimeBasisInvalid}
	}
	return nil
}

func validateOutcomesMatchScheme(
	scheme projectmemoryreferencescheme.ProjectMemoryReferenceSchemeV1,
	outcomes []RoleOutcome,
) Result {
	mismatchIndex := slices.IndexFunc(outcomes, func(outcome RoleOutcome) bool {
		return !outcomeMatchesScheme(scheme, outcome)
	})
	if mismatchIndex >= 0 {
		return Invalid{reason: ReasonReferenceSchemeRuntimeBasisInvalid}
	}
	return nil
}

func outcomeMatchesScheme(
	scheme projectmemoryreferencescheme.ProjectMemoryReferenceSchemeV1,
	outcome RoleOutcome,
) bool {
	expectedContract, err := projectmemoryreferencescheme.RuntimeContractForRole(
		outcome.Role(),
	)
	if err != nil || outcome.Contract() != expectedContract {
		return false
	}
	expectedPins, err := expectedRulePins(scheme, outcome.Role())
	if err != nil {
		return false
	}
	actualPins, err := normalizeOutcomeRulePins(
		outcome.Role(),
		outcome.EvaluatedRulePins(),
	)
	if err != nil {
		return false
	}
	return outcome.SchemeDigest() == scheme.Digest() &&
		slices.Equal(actualPins, expectedPins)
}

func expectedRulePins(
	scheme projectmemoryreferencescheme.ProjectMemoryReferenceSchemeV1,
	role projectmemoryreferencescheme.SemanticRole,
) ([]projectmemoryreferencescheme.ExactRulePin, error) {
	var pins []projectmemoryreferencescheme.ExactRulePin
	switch role {
	case projectmemoryreferencescheme.SemanticRoleDesignation:
		pins = scheme.Designation().Pins()
	case projectmemoryreferencescheme.SemanticRoleInterpretation:
		pins = scheme.Interpretation().Pins()
	case projectmemoryreferencescheme.SemanticRoleMeasurement:
		pins = measurementRulePins(scheme.Measurement())
	case projectmemoryreferencescheme.SemanticRoleEvaluation:
		pins = evaluationRulePins(scheme.Evaluation())
	default:
		return nil, fmt.Errorf("reference-scheme role %q is invalid", role)
	}
	return normalizeOutcomeRulePins(role, pins)
}

func measurementRulePins(
	branch projectmemoryreferencescheme.MeasurementBranch,
) []projectmemoryreferencescheme.ExactRulePin {
	switch value := branch.(type) {
	case projectmemoryreferencescheme.MeasurementRules:
		return value.Pins()
	case projectmemoryreferencescheme.MeasurementNotApplicable:
		return []projectmemoryreferencescheme.ExactRulePin{value.Rule()}
	default:
		return nil
	}
}

func evaluationRulePins(
	branch projectmemoryreferencescheme.EvaluationBranch,
) []projectmemoryreferencescheme.ExactRulePin {
	switch value := branch.(type) {
	case projectmemoryreferencescheme.EvaluationRules:
		return value.Pins()
	case projectmemoryreferencescheme.EvaluationNotApplicable:
		return []projectmemoryreferencescheme.ExactRulePin{value.Rule()}
	default:
		return nil
	}
}

func aggregateRoleOutcomePostures(
	graph CanonicalClaimGraph,
	concern ResolvedEntityOfConcern,
	scheme projectmemoryreferencescheme.ProjectMemoryReferenceSchemeV1,
	outcomes []RoleOutcome,
) Result {
	contradicted := slices.ContainsFunc(outcomes, func(outcome RoleOutcome) bool {
		_, ok := outcome.(RoleContradicted)
		return ok
	})
	if contradicted {
		return Invalid{reason: ReasonEpistemeConstitutionNotSatisfied}
	}
	underdetermined := slices.ContainsFunc(outcomes, func(outcome RoleOutcome) bool {
		_, ok := outcome.(RoleUnderdetermined)
		return ok
	})
	if underdetermined {
		return Underdetermined{reason: ReasonEpistemeConstitutionNotSatisfied}
	}
	coordinate := newEpistemeCoordinate(graph, concern, scheme)
	return Satisfied{coordinate: coordinate}
}
