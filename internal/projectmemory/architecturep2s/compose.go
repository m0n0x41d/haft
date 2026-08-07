package architecturep2s

import (
	"fmt"
	"sort"
)

type ObservedClaim struct {
	assertionID string
	signature   string
	context     string
	typeEnv     string
	modality    ClaimModality
	provenance  string
	originEvent string
	references  []Reference
}

type ObservedClaimInput struct {
	AssertionID string
	Signature   string
	Context     string
	TypeEnv     string
	Modality    ClaimModality
	Provenance  string
	OriginEvent string
	References  []Reference
}

func NewObservedClaim(input ObservedClaimInput) (ObservedClaim, error) {
	references, err := canonicalReferences(input.References)
	if err != nil {
		return ObservedClaim{}, err
	}
	if input.AssertionID == "" ||
		input.Signature == "" ||
		input.Context == "" ||
		input.TypeEnv == "" ||
		!input.Modality.valid() ||
		input.Provenance == "" ||
		input.OriginEvent == "" ||
		len(references) == 0 {
		return ObservedClaim{}, fmt.Errorf(
			"architecture P2S observed claim is incomplete",
		)
	}
	return ObservedClaim{
		assertionID: input.AssertionID,
		signature:   input.Signature,
		context:     input.Context,
		typeEnv:     input.TypeEnv,
		modality:    input.Modality,
		provenance:  input.Provenance,
		originEvent: input.OriginEvent,
		references:  references,
	}, nil
}

func (claim ObservedClaim) AssertionID() string { return claim.assertionID }

func (claim ObservedClaim) Signature() string { return claim.signature }

func (claim ObservedClaim) Context() string { return claim.context }

func (claim ObservedClaim) TypeEnv() string { return claim.typeEnv }

func (claim ObservedClaim) Modality() ClaimModality { return claim.modality }

func (claim ObservedClaim) Provenance() string { return claim.provenance }

func (claim ObservedClaim) OriginEvent() string { return claim.originEvent }

func (claim ObservedClaim) References() []Reference {
	return append([]Reference(nil), claim.references...)
}

func (claim ObservedClaim) key() string {
	return claim.assertionID + "|" + claim.signature
}

type ClaimRule struct {
	position  PositionKind
	signature string
	patternID string
}

type ClaimRuleInput struct {
	Position  PositionKind
	Signature string
	PatternID string
}

func NewClaimRule(input ClaimRuleInput) (ClaimRule, error) {
	if !input.Position.valid() ||
		input.Signature == "" ||
		input.PatternID == "" {
		return ClaimRule{}, fmt.Errorf(
			"architecture P2S claim rule is incomplete",
		)
	}
	return ClaimRule{
		position:  input.Position,
		signature: input.Signature,
		patternID: input.PatternID,
	}, nil
}

type SourceDockRule struct {
	position  PositionKind
	signature string
}

type SourceDockRuleInput struct {
	Position  PositionKind
	Signature string
}

func NewSourceDockRule(input SourceDockRuleInput) (SourceDockRule, error) {
	if !input.Position.valid() || input.Signature == "" {
		return SourceDockRule{}, fmt.Errorf(
			"architecture P2S source-dock rule is incomplete",
		)
	}
	return SourceDockRule{
		position:  input.Position,
		signature: input.Signature,
	}, nil
}

type RuleSet struct {
	claimRules []ClaimRule
	dockRules  []SourceDockRule
}

func NewRuleSet(
	claimRules []ClaimRule,
	dockRules []SourceDockRule,
) (RuleSet, error) {
	canonicalClaims := append([]ClaimRule(nil), claimRules...)
	sort.Slice(canonicalClaims, func(left int, right int) bool {
		return claimRuleKey(canonicalClaims[left]) <
			claimRuleKey(canonicalClaims[right])
	})
	for index, rule := range canonicalClaims {
		if !rule.position.valid() ||
			rule.signature == "" ||
			rule.patternID == "" {
			return RuleSet{}, fmt.Errorf(
				"architecture P2S claim rule %d is invalid",
				index,
			)
		}
		if index > 0 &&
			claimRuleKey(canonicalClaims[index-1]) == claimRuleKey(rule) {
			return RuleSet{}, fmt.Errorf(
				"architecture P2S claim rules repeat %q",
				claimRuleKey(rule),
			)
		}
	}
	canonicalDocks := append([]SourceDockRule(nil), dockRules...)
	sort.Slice(canonicalDocks, func(left int, right int) bool {
		return sourceDockRuleKey(canonicalDocks[left]) <
			sourceDockRuleKey(canonicalDocks[right])
	})
	for index, rule := range canonicalDocks {
		if !rule.position.valid() || rule.signature == "" {
			return RuleSet{}, fmt.Errorf(
				"architecture P2S source-dock rule %d is invalid",
				index,
			)
		}
		if index > 0 &&
			sourceDockRuleKey(canonicalDocks[index-1]) ==
				sourceDockRuleKey(rule) {
			return RuleSet{}, fmt.Errorf(
				"architecture P2S source-dock rules repeat %q",
				sourceDockRuleKey(rule),
			)
		}
	}
	return RuleSet{
		claimRules: canonicalClaims,
		dockRules:  canonicalDocks,
	}, nil
}

type NotApplicableBasis struct {
	position PositionKind
	basisRef string
	reason   string
}

func NewNotApplicableBasis(
	position PositionKind,
	basisRef string,
	reason string,
) (NotApplicableBasis, error) {
	if !position.valid() || basisRef == "" || reason == "" {
		return NotApplicableBasis{}, fmt.Errorf(
			"architecture P2S not-applicable basis is incomplete",
		)
	}
	return NotApplicableBasis{
		position: position,
		basisRef: basisRef,
		reason:   reason,
	}, nil
}

type ComposeInput struct {
	Basis         ProjectionBasis
	Claims        []ObservedClaim
	NotApplicable []NotApplicableBasis
}

// Compose builds one deterministic ephemeral read model. Relation
// co-membership among only predeclared read signatures is used to bound the
// exact concern-connected component. Unmapped carrier or registry relations
// cannot bridge otherwise separate concerns. Co-membership creates no
// direction, causality, Work order, actuality, or semantic promotion. A
// position becomes direct only through a predeclared exact signature rule and
// an explicit positive v3 assertion.
func Compose(input ComposeInput, rules RuleSet) (ReadModel, error) {
	if !input.Basis.valid() {
		return ReadModel{}, fmt.Errorf(
			"compose architecture P2S: projection basis is invalid",
		)
	}
	claims, err := canonicalObservedClaims(input.Basis, input.Claims)
	if err != nil {
		return ReadModel{}, err
	}
	notApplicable, err := canonicalNotApplicable(input.NotApplicable)
	if err != nil {
		return ReadModel{}, err
	}
	relevant := claimsNamedByRules(claims, rules)
	reachable := concernConnectedClaims(
		input.Basis.EntityOfConcern(),
		relevant,
	)
	positions := make([]Position, 0, len(positionKinds))
	for _, kind := range positionKinds {
		position, positionErr := composePosition(
			kind,
			reachable,
			rules,
			notApplicable,
		)
		if positionErr != nil {
			return ReadModel{}, positionErr
		}
		positions = append(positions, position)
	}
	return NewReadModel(input.Basis, positions)
}

func claimsNamedByRules(
	claims []ObservedClaim,
	rules RuleSet,
) []ObservedClaim {
	result := make([]ObservedClaim, 0, len(claims))
	for _, claim := range claims {
		if !signatureNamedByRules(claim.Signature(), rules) {
			continue
		}
		result = append(result, claim)
	}
	return result
}

func signatureNamedByRules(signature string, rules RuleSet) bool {
	for _, rule := range rules.claimRules {
		if rule.signature == signature {
			return true
		}
	}
	for _, rule := range rules.dockRules {
		if rule.signature == signature {
			return true
		}
	}
	return false
}

func composePosition(
	kind PositionKind,
	claims []ObservedClaim,
	rules RuleSet,
	notApplicable map[PositionKind]NotApplicableBasis,
) (Position, error) {
	source, err := positionSourceReturn(kind)
	if err != nil {
		return nil, err
	}
	direct, err := claimWitnessesForPosition(kind, claims, rules.claimRules)
	if err != nil {
		return nil, err
	}
	docks, err := sourceDocksForPosition(kind, claims, rules.dockRules)
	if err != nil {
		return nil, err
	}
	if basis, found := notApplicable[kind]; found {
		if len(direct) != 0 {
			return nil, fmt.Errorf(
				"architecture P2S position %q has both direct claims and a not-applicable basis",
				kind,
			)
		}
		return NewNotApplicablePosition(
			kind,
			source,
			basis.basisRef,
			basis.reason,
		)
	}
	affirmed, unresolved := splitClaimWitnesses(direct)
	if len(unresolved) != 0 {
		reason := "direct claim posture is conflicting, unknown, denied, or legacy-unqualified"
		return NewUnderdeterminedPosition(
			kind,
			source,
			reason,
			direct,
			docks,
		)
	}
	if len(affirmed) != 0 {
		return NewDirectClaimPosition(kind, source, affirmed, docks)
	}
	return NewMissingPosition(kind, source, docks)
}

func claimWitnessesForPosition(
	kind PositionKind,
	claims []ObservedClaim,
	rules []ClaimRule,
) ([]ClaimWitness, error) {
	result := make([]ClaimWitness, 0)
	for _, claim := range claims {
		rule, found := claimRuleFor(kind, claim.Signature(), rules)
		if !found {
			continue
		}
		witness, err := NewClaimWitness(ClaimWitnessInput{
			AssertionID: claim.AssertionID(),
			Signature:   claim.Signature(),
			Modality:    claim.Modality(),
			PatternID:   rule.patternID,
			Provenance:  claim.Provenance(),
			OriginEvent: claim.OriginEvent(),
			References:  claim.References(),
		})
		if err != nil {
			return nil, err
		}
		result = append(result, witness)
	}
	return canonicalClaimWitnesses(result)
}

func sourceDocksForPosition(
	kind PositionKind,
	claims []ObservedClaim,
	rules []SourceDockRule,
) ([]SourceDock, error) {
	result := make([]SourceDock, 0)
	for _, claim := range claims {
		if !hasSourceDockRule(kind, claim.Signature(), rules) {
			continue
		}
		dock, err := NewSourceDock(SourceDockInput{
			AssertionID: claim.AssertionID(),
			Signature:   claim.Signature(),
			Provenance:  claim.Provenance(),
			OriginEvent: claim.OriginEvent(),
			References:  claim.References(),
		})
		if err != nil {
			return nil, err
		}
		result = append(result, dock)
	}
	return canonicalSourceDocks(result)
}

func splitClaimWitnesses(
	values []ClaimWitness,
) ([]ClaimWitness, []ClaimWitness) {
	affirmed := make([]ClaimWitness, 0, len(values))
	unresolved := make([]ClaimWitness, 0, len(values))
	for _, value := range values {
		if value.Modality() == ClaimAffirmsObtaining {
			affirmed = append(affirmed, value)
			continue
		}
		unresolved = append(unresolved, value)
	}
	return affirmed, unresolved
}

func concernConnectedClaims(
	concern Reference,
	claims []ObservedClaim,
) []ObservedClaim {
	claimsByReference := make(map[string][]int)
	for index, claim := range claims {
		for _, reference := range claim.References() {
			key := reference.Key()
			claimsByReference[key] = append(
				claimsByReference[key],
				index,
			)
		}
	}
	visitedReferences := map[string]struct{}{concern.Key(): {}}
	visitedClaims := make(map[int]struct{})
	queue := []string{concern.Key()}
	for len(queue) != 0 {
		current := queue[0]
		queue = queue[1:]
		indexes := claimsByReference[current]
		for _, index := range indexes {
			if _, found := visitedClaims[index]; found {
				continue
			}
			visitedClaims[index] = struct{}{}
			queue = appendUnvisitedReferences(
				queue,
				visitedReferences,
				claims[index].References(),
			)
		}
	}
	result := make([]ObservedClaim, 0, len(visitedClaims))
	for index, claim := range claims {
		if _, found := visitedClaims[index]; !found {
			continue
		}
		result = append(result, claim)
	}
	return result
}

func appendUnvisitedReferences(
	queue []string,
	visited map[string]struct{},
	references []Reference,
) []string {
	result := append([]string(nil), queue...)
	for _, reference := range references {
		key := reference.Key()
		if _, found := visited[key]; found {
			continue
		}
		visited[key] = struct{}{}
		result = append(result, key)
	}
	return result
}

func canonicalObservedClaims(
	basis ProjectionBasis,
	values []ObservedClaim,
) ([]ObservedClaim, error) {
	result := append([]ObservedClaim(nil), values...)
	sort.Slice(result, func(left int, right int) bool {
		return result[left].key() < result[right].key()
	})
	for index, value := range result {
		_, err := NewObservedClaim(ObservedClaimInput{
			AssertionID: value.assertionID,
			Signature:   value.signature,
			Context:     value.context,
			TypeEnv:     value.typeEnv,
			Modality:    value.modality,
			Provenance:  value.provenance,
			OriginEvent: value.originEvent,
			References:  value.references,
		})
		if err != nil {
			return nil, fmt.Errorf(
				"architecture P2S observed claim %d: %w",
				index,
				err,
			)
		}
		if value.Context() != basis.Context() ||
			value.TypeEnv() != basis.TypeEnv() {
			return nil, fmt.Errorf(
				"architecture P2S observed claim %q uses another context or TypeEnv",
				value.AssertionID(),
			)
		}
		if index > 0 && result[index-1].key() == value.key() {
			return nil, fmt.Errorf(
				"architecture P2S observed claims repeat %q",
				value.key(),
			)
		}
	}
	return result, nil
}

func canonicalNotApplicable(
	values []NotApplicableBasis,
) (map[PositionKind]NotApplicableBasis, error) {
	result := make(map[PositionKind]NotApplicableBasis)
	for _, value := range values {
		if !value.position.valid() ||
			value.basisRef == "" ||
			value.reason == "" {
			return nil, fmt.Errorf(
				"architecture P2S not-applicable basis is invalid",
			)
		}
		if _, found := result[value.position]; found {
			return nil, fmt.Errorf(
				"architecture P2S repeats not-applicable basis for %q",
				value.position,
			)
		}
		result[value.position] = value
	}
	return result, nil
}

func claimRuleFor(
	kind PositionKind,
	signature string,
	rules []ClaimRule,
) (ClaimRule, bool) {
	for _, rule := range rules {
		if rule.position == kind && rule.signature == signature {
			return rule, true
		}
	}
	return ClaimRule{}, false
}

func hasSourceDockRule(
	kind PositionKind,
	signature string,
	rules []SourceDockRule,
) bool {
	for _, rule := range rules {
		if rule.position == kind && rule.signature == signature {
			return true
		}
	}
	return false
}

func claimRuleKey(rule ClaimRule) string {
	return string(rule.position) + "|" + rule.signature
}

func sourceDockRuleKey(rule SourceDockRule) string {
	return string(rule.position) + "|" + rule.signature
}
