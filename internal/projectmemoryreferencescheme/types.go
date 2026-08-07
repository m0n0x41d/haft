// Package projectmemoryreferencescheme defines the intrinsic, by-value
// ReferenceScheme used by Haft project-memory epistemes. Its identity is
// deliberately independent of a selected TypeEnv, context slice, time, and
// graph revision.
package projectmemoryreferencescheme

import (
	"bytes"
	"fmt"
	"slices"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// SemanticRole is the closed use made of one exact rule pin inside the
// project-memory ReferenceScheme.
type SemanticRole string

const (
	SemanticRoleDesignation    SemanticRole = "designation"
	SemanticRoleInterpretation SemanticRole = "interpretation"
	SemanticRoleMeasurement    SemanticRole = "measurement"
	SemanticRoleEvaluation     SemanticRole = "evaluation"
)

func (role SemanticRole) valid() bool {
	switch role {
	case SemanticRoleDesignation,
		SemanticRoleInterpretation,
		SemanticRoleMeasurement,
		SemanticRoleEvaluation:
		return true
	default:
		return false
	}
}

// ExactSourceCarrierPin identifies the immutable carrier bytes from which a
// rule is interpreted. RuleRef identifies the rule within this exact source.
type ExactSourceCarrierPin struct {
	revision typedmemory.SourceRevision
	carrier  typedmemory.CarrierRef
	edition  typedmemory.CarrierEdition
	digest   typedmemory.SHA256Digest
}

type ExactSourceCarrierPinInput struct {
	SourceRevision typedmemory.SourceRevision
	Carrier        typedmemory.CarrierRef
	Edition        typedmemory.CarrierEdition
	Digest         typedmemory.SHA256Digest
}

func NewExactSourceCarrierPin(
	input ExactSourceCarrierPinInput,
) (ExactSourceCarrierPin, error) {
	revision, err := validateSourceRevision(input.SourceRevision)
	if err != nil {
		return ExactSourceCarrierPin{}, err
	}
	carrier, err := validateCarrierRef("rule source carrier", input.Carrier)
	if err != nil {
		return ExactSourceCarrierPin{}, err
	}
	edition, err := validateCarrierEdition("rule source carrier", input.Edition)
	if err != nil {
		return ExactSourceCarrierPin{}, err
	}
	digest, err := validateDigest("rule source carrier", input.Digest)
	if err != nil {
		return ExactSourceCarrierPin{}, err
	}
	return ExactSourceCarrierPin{
		revision: revision,
		carrier:  carrier,
		edition:  edition,
		digest:   digest,
	}, nil
}

func (pin ExactSourceCarrierPin) SourceRevision() typedmemory.SourceRevision {
	return pin.revision
}

func (pin ExactSourceCarrierPin) Carrier() typedmemory.CarrierRef {
	return pin.carrier
}

func (pin ExactSourceCarrierPin) Edition() typedmemory.CarrierEdition {
	return pin.edition
}

func (pin ExactSourceCarrierPin) Digest() typedmemory.SHA256Digest {
	return pin.digest
}

func (pin ExactSourceCarrierPin) valid() bool {
	_, revisionErr := validateSourceRevision(pin.revision)
	_, carrierErr := validateCarrierRef("rule source carrier", pin.carrier)
	_, editionErr := validateCarrierEdition("rule source carrier", pin.edition)
	_, digestErr := validateDigest("rule source carrier", pin.digest)
	return revisionErr == nil &&
		carrierErr == nil &&
		editionErr == nil &&
		digestErr == nil
}

// ExactRuntimeMechanismPin identifies immutable mechanism bytes. It does not
// claim that the mechanism is loaded, registered, or successfully executed.
type ExactRuntimeMechanismPin struct {
	artifact typedmemory.CarrierRef
	edition  typedmemory.CarrierEdition
	digest   typedmemory.SHA256Digest
}

type ExactRuntimeMechanismPinInput struct {
	Artifact typedmemory.CarrierRef
	Edition  typedmemory.CarrierEdition
	Digest   typedmemory.SHA256Digest
}

func NewExactRuntimeMechanismPin(
	input ExactRuntimeMechanismPinInput,
) (ExactRuntimeMechanismPin, error) {
	artifact, err := validateCarrierRef("runtime mechanism artifact", input.Artifact)
	if err != nil {
		return ExactRuntimeMechanismPin{}, err
	}
	edition, err := validateCarrierEdition("runtime mechanism artifact", input.Edition)
	if err != nil {
		return ExactRuntimeMechanismPin{}, err
	}
	digest, err := validateDigest("runtime mechanism artifact", input.Digest)
	if err != nil {
		return ExactRuntimeMechanismPin{}, err
	}
	return ExactRuntimeMechanismPin{
		artifact: artifact,
		edition:  edition,
		digest:   digest,
	}, nil
}

func (pin ExactRuntimeMechanismPin) Artifact() typedmemory.CarrierRef {
	return pin.artifact
}

func (pin ExactRuntimeMechanismPin) Edition() typedmemory.CarrierEdition {
	return pin.edition
}

func (pin ExactRuntimeMechanismPin) Digest() typedmemory.SHA256Digest {
	return pin.digest
}

func (pin ExactRuntimeMechanismPin) valid() bool {
	_, artifactErr := validateCarrierRef("runtime mechanism artifact", pin.artifact)
	_, editionErr := validateCarrierEdition("runtime mechanism artifact", pin.edition)
	_, digestErr := validateDigest("runtime mechanism artifact", pin.digest)
	return artifactErr == nil && editionErr == nil && digestErr == nil
}

// RuntimeRequirement is an explicit sum. A caller must state whether a rule
// is declarative-only or pinned to an exact runtime mechanism.
type RuntimeRequirement interface {
	runtimeRequirementVariant()
}

type RuntimeNotRequired struct{}

func NewRuntimeNotRequired() RuntimeNotRequired { return RuntimeNotRequired{} }

func (RuntimeNotRequired) runtimeRequirementVariant() {}

type RuntimeRequired struct {
	mechanism ExactRuntimeMechanismPin
}

func NewRuntimeRequired(
	mechanism ExactRuntimeMechanismPin,
) (RuntimeRequired, error) {
	if !mechanism.valid() {
		return RuntimeRequired{}, fmt.Errorf("exact runtime mechanism pin is required")
	}
	return RuntimeRequired{mechanism: mechanism}, nil
}

func (requirement RuntimeRequired) Mechanism() ExactRuntimeMechanismPin {
	return requirement.mechanism
}

func (RuntimeRequired) runtimeRequirementVariant() {}

// ExactRulePin binds one semantic role and RuleRef to immutable source bytes
// and to an explicit runtime-requirement branch.
type ExactRulePin struct {
	role    SemanticRole
	rule    typedmemory.RuleRef
	source  ExactSourceCarrierPin
	runtime RuntimeRequirement
}

type ExactRulePinInput struct {
	Role    SemanticRole
	Rule    typedmemory.RuleRef
	Source  ExactSourceCarrierPin
	Runtime RuntimeRequirement
}

func NewExactRulePin(input ExactRulePinInput) (ExactRulePin, error) {
	if !input.Role.valid() {
		return ExactRulePin{}, fmt.Errorf("exact rule semantic role %q is invalid", input.Role)
	}
	rule, err := validateRuleRef(input.Rule)
	if err != nil {
		return ExactRulePin{}, err
	}
	if !input.Source.valid() {
		return ExactRulePin{}, fmt.Errorf("exact rule source/carrier pin is required")
	}
	runtime, err := validateRuntimeRequirement(input.Runtime)
	if err != nil {
		return ExactRulePin{}, err
	}
	return ExactRulePin{
		role:    input.Role,
		rule:    rule,
		source:  input.Source,
		runtime: runtime,
	}, nil
}

func (pin ExactRulePin) Role() SemanticRole { return pin.role }

func (pin ExactRulePin) Rule() typedmemory.RuleRef { return pin.rule }

func (pin ExactRulePin) Source() ExactSourceCarrierPin { return pin.source }

func (pin ExactRulePin) Runtime() RuntimeRequirement { return pin.runtime }

func (pin ExactRulePin) valid() bool {
	if !pin.role.valid() || !pin.source.valid() {
		return false
	}
	_, ruleErr := validateRuleRef(pin.rule)
	_, runtimeErr := validateRuntimeRequirement(pin.runtime)
	return ruleErr == nil && runtimeErr == nil
}

// DesignationRules and InterpretationRules are distinct non-empty groups;
// neither can be omitted from a sealed ReferenceScheme.
type DesignationRules struct {
	pins []ExactRulePin
}

func NewDesignationRules(pins []ExactRulePin) (DesignationRules, error) {
	normalized, err := normalizeRulePins(
		"designation rules",
		SemanticRoleDesignation,
		pins,
	)
	if err != nil {
		return DesignationRules{}, err
	}
	return DesignationRules{pins: normalized}, nil
}

func (rules DesignationRules) Pins() []ExactRulePin {
	return slices.Clone(rules.pins)
}

func (rules DesignationRules) valid() bool {
	_, err := normalizeRulePins(
		"designation rules",
		SemanticRoleDesignation,
		rules.pins,
	)
	return err == nil
}

type InterpretationRules struct {
	pins []ExactRulePin
}

func NewInterpretationRules(pins []ExactRulePin) (InterpretationRules, error) {
	normalized, err := normalizeRulePins(
		"interpretation rules",
		SemanticRoleInterpretation,
		pins,
	)
	if err != nil {
		return InterpretationRules{}, err
	}
	return InterpretationRules{pins: normalized}, nil
}

func (rules InterpretationRules) Pins() []ExactRulePin {
	return slices.Clone(rules.pins)
}

func (rules InterpretationRules) valid() bool {
	_, err := normalizeRulePins(
		"interpretation rules",
		SemanticRoleInterpretation,
		rules.pins,
	)
	return err == nil
}

// MeasurementBranch is an explicit Rules | NotApplicable sum.
type MeasurementBranch interface {
	measurementBranchVariant()
}

type MeasurementRules struct {
	pins []ExactRulePin
}

func NewMeasurementRules(pins []ExactRulePin) (MeasurementRules, error) {
	normalized, err := normalizeRulePins(
		"measurement rules",
		SemanticRoleMeasurement,
		pins,
	)
	if err != nil {
		return MeasurementRules{}, err
	}
	return MeasurementRules{pins: normalized}, nil
}

func (rules MeasurementRules) Pins() []ExactRulePin {
	return slices.Clone(rules.pins)
}

func (MeasurementRules) measurementBranchVariant() {}

type MeasurementNotApplicable struct {
	rule ExactRulePin
}

func NewMeasurementNotApplicable(
	rule ExactRulePin,
) (MeasurementNotApplicable, error) {
	if err := validateRuleForRole("measurement not-applicable rule", SemanticRoleMeasurement, rule); err != nil {
		return MeasurementNotApplicable{}, err
	}
	return MeasurementNotApplicable{rule: rule}, nil
}

func (branch MeasurementNotApplicable) Rule() ExactRulePin { return branch.rule }

func (MeasurementNotApplicable) measurementBranchVariant() {}

// EvaluationBranch is an explicit Rules | NotApplicable sum.
type EvaluationBranch interface {
	evaluationBranchVariant()
}

type EvaluationRules struct {
	pins []ExactRulePin
}

func NewEvaluationRules(pins []ExactRulePin) (EvaluationRules, error) {
	normalized, err := normalizeRulePins(
		"evaluation rules",
		SemanticRoleEvaluation,
		pins,
	)
	if err != nil {
		return EvaluationRules{}, err
	}
	return EvaluationRules{pins: normalized}, nil
}

func (rules EvaluationRules) Pins() []ExactRulePin {
	return slices.Clone(rules.pins)
}

func (EvaluationRules) evaluationBranchVariant() {}

type EvaluationNotApplicable struct {
	rule ExactRulePin
}

func NewEvaluationNotApplicable(
	rule ExactRulePin,
) (EvaluationNotApplicable, error) {
	if err := validateRuleForRole("evaluation not-applicable rule", SemanticRoleEvaluation, rule); err != nil {
		return EvaluationNotApplicable{}, err
	}
	return EvaluationNotApplicable{rule: rule}, nil
}

func (branch EvaluationNotApplicable) Rule() ExactRulePin { return branch.rule }

func (EvaluationNotApplicable) evaluationBranchVariant() {}

func normalizeRulePins(
	name string,
	role SemanticRole,
	pins []ExactRulePin,
) ([]ExactRulePin, error) {
	if len(pins) == 0 {
		return nil, fmt.Errorf("%s require at least one exact rule pin", name)
	}
	if len(pins) > maximumRulePins {
		return nil, fmt.Errorf("%s exceed %d exact rule pins", name, maximumRulePins)
	}
	normalized := slices.Clone(pins)
	for index, pin := range normalized {
		if err := validateRuleForRole(name, role, pin); err != nil {
			return nil, fmt.Errorf("%s pin %d: %w", name, index, err)
		}
	}
	slices.SortFunc(normalized, compareRulePins)
	for index := 1; index < len(normalized); index++ {
		previous := normalized[index-1]
		current := normalized[index]
		if previous.rule == current.rule {
			return nil, fmt.Errorf(
				"%s contain duplicate RuleRef %q",
				name,
				current.rule.String(),
			)
		}
	}
	return normalized, nil
}

func validateRuleForRole(
	name string,
	role SemanticRole,
	pin ExactRulePin,
) error {
	if !pin.valid() {
		return fmt.Errorf("%s is invalid", name)
	}
	if pin.role != role {
		return fmt.Errorf(
			"semantic role %q does not match required role %q",
			pin.role,
			role,
		)
	}
	return nil
}

func compareRulePins(left ExactRulePin, right ExactRulePin) int {
	leftCanonical := encodeRulePinCanonical(left)
	rightCanonical := encodeRulePinCanonical(right)
	return bytes.Compare(leftCanonical, rightCanonical)
}

func validateRuntimeRequirement(
	requirement RuntimeRequirement,
) (RuntimeRequirement, error) {
	switch value := requirement.(type) {
	case RuntimeNotRequired:
		return value, nil
	case RuntimeRequired:
		if !value.mechanism.valid() {
			return nil, fmt.Errorf("exact runtime mechanism pin is required")
		}
		return value, nil
	default:
		return nil, fmt.Errorf("explicit runtime requirement branch is required")
	}
}

func validateSourceRevision(
	revision typedmemory.SourceRevision,
) (typedmemory.SourceRevision, error) {
	rebuilt, err := typedmemory.NewSourceRevision(revision.String())
	if err != nil || rebuilt != revision {
		return typedmemory.SourceRevision{}, fmt.Errorf("source revision is invalid")
	}
	if err := validateCanonicalText("source revision", rebuilt.String()); err != nil {
		return typedmemory.SourceRevision{}, err
	}
	return rebuilt, nil
}

func validateCarrierRef(
	name string,
	carrier typedmemory.CarrierRef,
) (typedmemory.CarrierRef, error) {
	rebuilt, err := typedmemory.NewCarrierRef(carrier.String())
	if err != nil || rebuilt != carrier {
		return typedmemory.CarrierRef{}, fmt.Errorf("%s reference is invalid", name)
	}
	if err := validateCanonicalText(name+" reference", rebuilt.String()); err != nil {
		return typedmemory.CarrierRef{}, err
	}
	return rebuilt, nil
}

func validateCarrierEdition(
	name string,
	edition typedmemory.CarrierEdition,
) (typedmemory.CarrierEdition, error) {
	rebuilt, err := typedmemory.NewCarrierEdition(edition.String())
	if err != nil || rebuilt != edition {
		return typedmemory.CarrierEdition{}, fmt.Errorf("%s edition is invalid", name)
	}
	if err := validateCanonicalText(name+" edition", rebuilt.String()); err != nil {
		return typedmemory.CarrierEdition{}, err
	}
	return rebuilt, nil
}

func validateDigest(
	name string,
	digest typedmemory.SHA256Digest,
) (typedmemory.SHA256Digest, error) {
	rebuilt, err := typedmemory.NewSHA256Digest(digest.String())
	if err != nil || rebuilt != digest {
		return typedmemory.SHA256Digest{}, fmt.Errorf("%s digest is invalid", name)
	}
	return rebuilt, nil
}

func validateRuleRef(rule typedmemory.RuleRef) (typedmemory.RuleRef, error) {
	rebuilt, err := typedmemory.NewRuleRef(rule.String())
	if err != nil || rebuilt != rule {
		return typedmemory.RuleRef{}, fmt.Errorf("exact RuleRef is invalid")
	}
	if err := validateCanonicalText("exact RuleRef", rebuilt.String()); err != nil {
		return typedmemory.RuleRef{}, err
	}
	return rebuilt, nil
}
