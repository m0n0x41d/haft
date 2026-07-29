package typedmemory

import (
	"bytes"
	"fmt"
	"sort"
)

// SubkindOfRequest carries the direct U.SubkindOf participants and exact
// effective ReferenceScheme edition. Its digest is the participant-determined
// relation-occurrence identity; it is not a claim that the relation obtains.
type SubkindOfRequest struct {
	narrower        LocalKindRef
	broader         LocalKindRef
	referenceScheme KindReferenceSchemePin
	canonicalBytes  []byte
	digest          SHA256Digest
}

func NewSubkindOfRequest(
	narrower LocalKindRef,
	broader LocalKindRef,
	referenceScheme KindReferenceSchemePin,
) (SubkindOfRequest, error) {
	if !narrower.valid() || !broader.valid() {
		return SubkindOfRequest{}, fmt.Errorf("U.SubkindOf requires exact narrower and broader local kinds")
	}
	if narrower.TypeEnv() != broader.TypeEnv() || narrower.Context() != broader.Context() {
		return SubkindOfRequest{}, fmt.Errorf("U.SubkindOf participants must use one TypeEnv and bounded context")
	}
	if !referenceScheme.valid() {
		return SubkindOfRequest{}, fmt.Errorf("U.SubkindOf requires an exact effective ReferenceScheme edition")
	}
	writer := newCanonicalWriter("subkind-of-request.v1")
	writer.addString(narrower.String())
	writer.addString(broader.String())
	writer.addBytes(referenceScheme.CanonicalBytes())
	return SubkindOfRequest{
		narrower:        narrower,
		broader:         broader,
		referenceScheme: referenceScheme,
		canonicalBytes:  writer.bytes(),
		digest:          writer.digest(),
	}, nil
}

func (request SubkindOfRequest) NarrowerKind() LocalKindRef { return request.narrower }

func (request SubkindOfRequest) BroaderKind() LocalKindRef { return request.broader }

func (request SubkindOfRequest) ReferenceScheme() KindReferenceSchemePin {
	return request.referenceScheme
}

func (request SubkindOfRequest) CanonicalBytes() []byte {
	return append([]byte(nil), request.canonicalBytes...)
}

func (request SubkindOfRequest) Digest() SHA256Digest { return request.digest }

func (request SubkindOfRequest) valid() bool {
	rebuilt, err := NewSubkindOfRequest(
		request.narrower,
		request.broader,
		request.referenceScheme,
	)
	return err == nil && rebuilt.digest == request.digest &&
		bytes.Equal(rebuilt.canonicalBytes, request.canonicalBytes)
}

// SubkindOfObtainingFact is the settled direct-relation fact used by order
// checks. A separate assertion may designate it, but the assertion does not
// create this fact.
type SubkindOfObtainingFact struct {
	request        SubkindOfRequest
	governingRule  RuleRef
	canonicalBytes []byte
	digest         SHA256Digest
}

func NewSubkindOfObtainingFact(
	request SubkindOfRequest,
	governingRule RuleRef,
) (SubkindOfObtainingFact, error) {
	if !request.valid() {
		return SubkindOfObtainingFact{}, fmt.Errorf("SubkindOfObtains requires an exact direct-relation request")
	}
	if !governingRule.valid() {
		return SubkindOfObtainingFact{}, fmt.Errorf("SubkindOfObtains requires a direct governing rule")
	}
	writer := newCanonicalWriter("subkind-of-obtaining-fact.v1")
	writer.addBytes(request.CanonicalBytes())
	writer.addString(governingRule.String())
	return SubkindOfObtainingFact{
		request:        request,
		governingRule:  governingRule,
		canonicalBytes: writer.bytes(),
		digest:         writer.digest(),
	}, nil
}

func (fact SubkindOfObtainingFact) Request() SubkindOfRequest { return fact.request }

func (fact SubkindOfObtainingFact) GoverningRule() RuleRef { return fact.governingRule }

func (fact SubkindOfObtainingFact) CanonicalBytes() []byte {
	return append([]byte(nil), fact.canonicalBytes...)
}

func (fact SubkindOfObtainingFact) Digest() SHA256Digest { return fact.digest }

func (fact SubkindOfObtainingFact) valid() bool {
	rebuilt, err := NewSubkindOfObtainingFact(fact.request, fact.governingRule)
	return err == nil && rebuilt.digest == fact.digest &&
		bytes.Equal(rebuilt.canonicalBytes, fact.canonicalBytes)
}

// SubkindOfAssertionLink keeps the C.2.1 assertion episteme separate from the
// obtaining relation fact it designates.
type SubkindOfAssertionLink struct {
	factDigest     SHA256Digest
	assertion      KindClassificationAssertionPin
	canonicalBytes []byte
	digest         SHA256Digest
}

func NewSubkindOfAssertionLink(
	fact SubkindOfObtainingFact,
	assertion KindClassificationAssertionPin,
) (SubkindOfAssertionLink, error) {
	if !fact.valid() {
		return SubkindOfAssertionLink{}, fmt.Errorf("subkind assertion link requires an exact obtaining fact")
	}
	if !assertion.valid() {
		return SubkindOfAssertionLink{}, fmt.Errorf("subkind assertion link requires a separate C.2.1 assertion")
	}
	writer := newCanonicalWriter("subkind-of-assertion-link.v1")
	writer.addString(fact.Digest().String())
	writer.addBytes(assertion.CanonicalBytes())
	return SubkindOfAssertionLink{
		factDigest:     fact.Digest(),
		assertion:      assertion,
		canonicalBytes: writer.bytes(),
		digest:         writer.digest(),
	}, nil
}

func (link SubkindOfAssertionLink) FactDigest() SHA256Digest { return link.factDigest }

func (link SubkindOfAssertionLink) Assertion() KindClassificationAssertionPin {
	return link.assertion
}

func (link SubkindOfAssertionLink) CanonicalBytes() []byte {
	return append([]byte(nil), link.canonicalBytes...)
}

func (link SubkindOfAssertionLink) Digest() SHA256Digest { return link.digest }

type SubkindOrderViolationKind uint8

const (
	SubkindOrderMissingReflexiveFact SubkindOrderViolationKind = iota + 1
	SubkindOrderMissingTransitiveFact
	SubkindOrderAntisymmetryViolation
)

func (kind SubkindOrderViolationKind) String() string {
	switch kind {
	case SubkindOrderMissingReflexiveFact:
		return "missing_reflexive_fact"
	case SubkindOrderMissingTransitiveFact:
		return "missing_transitive_fact"
	case SubkindOrderAntisymmetryViolation:
		return "antisymmetry_violation"
	default:
		return ""
	}
}

type SubkindOrderViolation struct {
	kind   SubkindOrderViolationKind
	first  LocalKindRef
	second LocalKindRef
	third  LocalKindRef
}

func (violation SubkindOrderViolation) Kind() SubkindOrderViolationKind {
	return violation.kind
}

func (violation SubkindOrderViolation) First() LocalKindRef { return violation.first }

func (violation SubkindOrderViolation) Second() LocalKindRef { return violation.second }

func (violation SubkindOrderViolation) Third() (LocalKindRef, bool) {
	return violation.third, violation.third.valid()
}

func (violation SubkindOrderViolation) CanonicalBytes() []byte {
	writer := newCanonicalWriter("subkind-order-violation.v1")
	writer.addString(violation.kind.String())
	writer.addString(violation.first.String())
	writer.addString(violation.second.String())
	writer.addString(violation.third.String())
	return writer.bytes()
}

type SubkindOrderAssessment struct {
	valid          bool
	violations     []SubkindOrderViolation
	canonicalBytes []byte
	digest         SHA256Digest
}

// AssessSubkindOrder checks only explicit obtaining facts for one exact
// ReferenceScheme edition. It does not infer a missing edge or turn absence
// into a false relation judgement.
func AssessSubkindOrder(
	kinds []LocalKindRef,
	facts []SubkindOfObtainingFact,
) (SubkindOrderAssessment, error) {
	normalizedKinds, err := normalizeSubkindOrderKinds(kinds)
	if err != nil {
		return SubkindOrderAssessment{}, err
	}
	normalizedFacts, scheme, err := normalizeSubkindOrderFacts(normalizedKinds, facts)
	if err != nil {
		return SubkindOrderAssessment{}, err
	}
	edges := subkindOrderEdges(normalizedFacts)
	violations := subkindOrderViolations(normalizedKinds, edges)
	writer := newCanonicalWriter("subkind-order-assessment.v1")
	writer.addBytes(scheme.CanonicalBytes())
	writer.addUint64(uint64(len(normalizedKinds)))
	for _, kind := range normalizedKinds {
		writer.addString(kind.String())
	}
	writer.addUint64(uint64(len(normalizedFacts)))
	for _, fact := range normalizedFacts {
		writer.addBytes(fact.CanonicalBytes())
	}
	writer.addUint64(uint64(len(violations)))
	for _, violation := range violations {
		writer.addBytes(violation.CanonicalBytes())
	}
	return SubkindOrderAssessment{
		valid:          len(violations) == 0,
		violations:     violations,
		canonicalBytes: writer.bytes(),
		digest:         writer.digest(),
	}, nil
}

func (assessment SubkindOrderAssessment) Valid() bool { return assessment.valid }

func (assessment SubkindOrderAssessment) Violations() []SubkindOrderViolation {
	return append([]SubkindOrderViolation(nil), assessment.violations...)
}

func (assessment SubkindOrderAssessment) CanonicalBytes() []byte {
	return append([]byte(nil), assessment.canonicalBytes...)
}

func (assessment SubkindOrderAssessment) Digest() SHA256Digest { return assessment.digest }

func normalizeSubkindOrderKinds(values []LocalKindRef) ([]LocalKindRef, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("subkind order assessment requires an exact non-empty kind domain")
	}
	result := append([]LocalKindRef(nil), values...)
	sort.Slice(result, func(left, right int) bool {
		return result[left].String() < result[right].String()
	})
	normalized := make([]LocalKindRef, 0, len(result))
	for _, kind := range result {
		if !kind.valid() {
			return nil, fmt.Errorf("subkind order domain contains an invalid local kind")
		}
		if len(normalized) > 0 && normalized[len(normalized)-1] == kind {
			continue
		}
		if len(normalized) > 0 &&
			(normalized[0].TypeEnv() != kind.TypeEnv() || normalized[0].Context() != kind.Context()) {
			return nil, fmt.Errorf("subkind order domain crosses TypeEnv or bounded-context boundaries")
		}
		normalized = append(normalized, kind)
	}
	return normalized, nil
}

func normalizeSubkindOrderFacts(
	kinds []LocalKindRef,
	values []SubkindOfObtainingFact,
) ([]SubkindOfObtainingFact, KindReferenceSchemePin, error) {
	if len(values) == 0 {
		return nil, KindReferenceSchemePin{}, fmt.Errorf("subkind order assessment requires explicit obtaining facts")
	}
	domain := make(map[LocalKindRef]struct{}, len(kinds))
	for _, kind := range kinds {
		domain[kind] = struct{}{}
	}
	result := append([]SubkindOfObtainingFact(nil), values...)
	sort.Slice(result, func(left, right int) bool {
		return bytes.Compare(result[left].CanonicalBytes(), result[right].CanonicalBytes()) < 0
	})
	scheme := result[0].Request().ReferenceScheme()
	normalized := make([]SubkindOfObtainingFact, 0, len(result))
	for _, fact := range result {
		if !fact.valid() {
			return nil, KindReferenceSchemePin{}, fmt.Errorf("subkind order assessment contains an invalid obtaining fact")
		}
		request := fact.Request()
		if _, exists := domain[request.NarrowerKind()]; !exists {
			return nil, KindReferenceSchemePin{}, fmt.Errorf("subkind obtaining fact has a narrower kind outside the declared domain")
		}
		if _, exists := domain[request.BroaderKind()]; !exists {
			return nil, KindReferenceSchemePin{}, fmt.Errorf("subkind obtaining fact has a broader kind outside the declared domain")
		}
		if !bytes.Equal(scheme.CanonicalBytes(), request.ReferenceScheme().CanonicalBytes()) {
			return nil, KindReferenceSchemePin{}, fmt.Errorf("subkind order assessment mixes ReferenceScheme editions")
		}
		if len(normalized) > 0 && normalized[len(normalized)-1].Request().Digest() == request.Digest() {
			continue
		}
		normalized = append(normalized, fact)
	}
	return normalized, scheme, nil
}

type subkindOrderEdge struct {
	narrower LocalKindRef
	broader  LocalKindRef
}

func subkindOrderEdges(facts []SubkindOfObtainingFact) map[subkindOrderEdge]struct{} {
	edges := make(map[subkindOrderEdge]struct{}, len(facts))
	for _, fact := range facts {
		request := fact.Request()
		edges[subkindOrderEdge{
			narrower: request.NarrowerKind(),
			broader:  request.BroaderKind(),
		}] = struct{}{}
	}
	return edges
}

func subkindOrderViolations(
	kinds []LocalKindRef,
	edges map[subkindOrderEdge]struct{},
) []SubkindOrderViolation {
	violations := make([]SubkindOrderViolation, 0)
	for _, kind := range kinds {
		if _, exists := edges[subkindOrderEdge{narrower: kind, broader: kind}]; !exists {
			violations = append(violations, SubkindOrderViolation{
				kind:   SubkindOrderMissingReflexiveFact,
				first:  kind,
				second: kind,
			})
		}
	}
	pairCount := len(kinds) * len(kinds)
	for index := range pairCount {
		first, second := subkindPairAt(kinds, index)
		if first == second || first.String() > second.String() {
			continue
		}
		if _, forward := edges[subkindOrderEdge{narrower: first, broader: second}]; !forward {
			continue
		}
		if _, reverse := edges[subkindOrderEdge{narrower: second, broader: first}]; !reverse {
			continue
		}
		violations = append(violations, SubkindOrderViolation{
			kind:   SubkindOrderAntisymmetryViolation,
			first:  first,
			second: second,
		})
	}
	tripleCount := pairCount * len(kinds)
	for index := range tripleCount {
		first, second, third := subkindTripleAt(kinds, index)
		if _, firstFact := edges[subkindOrderEdge{narrower: first, broader: second}]; !firstFact {
			continue
		}
		if _, secondFact := edges[subkindOrderEdge{narrower: second, broader: third}]; !secondFact {
			continue
		}
		if _, closure := edges[subkindOrderEdge{narrower: first, broader: third}]; closure {
			continue
		}
		violations = append(violations, SubkindOrderViolation{
			kind:   SubkindOrderMissingTransitiveFact,
			first:  first,
			second: second,
			third:  third,
		})
	}
	sort.Slice(violations, func(left, right int) bool {
		return bytes.Compare(
			violations[left].CanonicalBytes(),
			violations[right].CanonicalBytes(),
		) < 0
	})
	return deduplicateSubkindOrderViolations(violations)
}

func subkindPairAt(kinds []LocalKindRef, index int) (LocalKindRef, LocalKindRef) {
	width := len(kinds)
	return kinds[index/width], kinds[index%width]
}

func subkindTripleAt(
	kinds []LocalKindRef,
	index int,
) (LocalKindRef, LocalKindRef, LocalKindRef) {
	width := len(kinds)
	pairWidth := width * width
	first := kinds[index/pairWidth]
	second := kinds[(index/width)%width]
	third := kinds[index%width]
	return first, second, third
}

func deduplicateSubkindOrderViolations(
	values []SubkindOrderViolation,
) []SubkindOrderViolation {
	result := make([]SubkindOrderViolation, 0, len(values))
	for _, violation := range values {
		if len(result) > 0 && bytes.Equal(
			result[len(result)-1].CanonicalBytes(),
			violation.CanonicalBytes(),
		) {
			continue
		}
		result = append(result, violation)
	}
	return result
}

type KindBridgeDirection uint8

const (
	KindBridgeForwardOnly KindBridgeDirection = iota + 1
	KindBridgeBidirectional
)

func (direction KindBridgeDirection) String() string {
	switch direction {
	case KindBridgeForwardOnly:
		return "forward_only"
	case KindBridgeBidirectional:
		return "bidirectional"
	default:
		return ""
	}
}

func (direction KindBridgeDirection) valid() bool { return direction.String() != "" }

type KindBridgeInput struct {
	SourceKind            LocalKindRef
	TargetKind            LocalKindRef
	SourceReferenceScheme KindReferenceSchemePin
	TargetReferenceScheme KindReferenceSchemePin
	Direction             KindBridgeDirection
	DefinednessRule       RuleRef
	Provenance            DeclarationProvenance
}

// KindBridge is one obtaining direct relation between exact source and target
// local kinds. It is separate from any bridge-assertion episteme and never
// transports a source classification result as target truth.
type KindBridge struct {
	sourceKind            LocalKindRef
	targetKind            LocalKindRef
	sourceReferenceScheme KindReferenceSchemePin
	targetReferenceScheme KindReferenceSchemePin
	direction             KindBridgeDirection
	definednessRule       RuleRef
	provenance            DeclarationProvenance
	canonicalBytes        []byte
	digest                SHA256Digest
}

func NewKindBridge(input KindBridgeInput) (KindBridge, error) {
	if !input.SourceKind.valid() || !input.TargetKind.valid() {
		return KindBridge{}, fmt.Errorf("KindBridge requires exact source and target local kinds")
	}
	if !input.SourceReferenceScheme.valid() || !input.TargetReferenceScheme.valid() {
		return KindBridge{}, fmt.Errorf("KindBridge requires exact source and target ReferenceScheme editions")
	}
	if !input.Direction.valid() {
		return KindBridge{}, fmt.Errorf("KindBridge direction is required")
	}
	if !input.DefinednessRule.valid() {
		return KindBridge{}, fmt.Errorf("KindBridge definedness rule is required")
	}
	if !validDeclarationProvenance(input.Provenance) {
		return KindBridge{}, fmt.Errorf("KindBridge provenance is required")
	}
	writer := newCanonicalWriter("kind-bridge.v1")
	writer.addString(input.SourceKind.String())
	writer.addString(input.TargetKind.String())
	writer.addBytes(input.SourceReferenceScheme.CanonicalBytes())
	writer.addBytes(input.TargetReferenceScheme.CanonicalBytes())
	writer.addString(input.Direction.String())
	writer.addString(input.DefinednessRule.String())
	writer.addBytes(input.Provenance.CanonicalBytes())
	return KindBridge{
		sourceKind:            input.SourceKind,
		targetKind:            input.TargetKind,
		sourceReferenceScheme: input.SourceReferenceScheme,
		targetReferenceScheme: input.TargetReferenceScheme,
		direction:             input.Direction,
		definednessRule:       input.DefinednessRule,
		provenance:            input.Provenance,
		canonicalBytes:        writer.bytes(),
		digest:                writer.digest(),
	}, nil
}

func (bridge KindBridge) SourceKind() LocalKindRef { return bridge.sourceKind }

func (bridge KindBridge) TargetKind() LocalKindRef { return bridge.targetKind }

func (bridge KindBridge) SourceReferenceScheme() KindReferenceSchemePin {
	return bridge.sourceReferenceScheme
}

func (bridge KindBridge) TargetReferenceScheme() KindReferenceSchemePin {
	return bridge.targetReferenceScheme
}

func (bridge KindBridge) Direction() KindBridgeDirection { return bridge.direction }

func (bridge KindBridge) DefinednessRule() RuleRef { return bridge.definednessRule }

func (bridge KindBridge) Provenance() DeclarationProvenance { return bridge.provenance }

func (bridge KindBridge) CanonicalBytes() []byte {
	return append([]byte(nil), bridge.canonicalBytes...)
}

func (bridge KindBridge) Digest() SHA256Digest { return bridge.digest }

func (bridge KindBridge) valid() bool {
	rebuilt, err := NewKindBridge(KindBridgeInput{
		SourceKind:            bridge.sourceKind,
		TargetKind:            bridge.targetKind,
		SourceReferenceScheme: bridge.sourceReferenceScheme,
		TargetReferenceScheme: bridge.targetReferenceScheme,
		Direction:             bridge.direction,
		DefinednessRule:       bridge.definednessRule,
		Provenance:            bridge.provenance,
	})
	return err == nil && rebuilt.digest == bridge.digest &&
		bytes.Equal(rebuilt.canonicalBytes, bridge.canonicalBytes)
}

type BridgedKindClassificationRequestInput struct {
	Bridge          KindBridge
	SourceRequest   KindClassificationRequest
	SourceSignature KindClassificationSignatureDefinition
	TargetSignature KindClassificationSignatureDefinition
	TargetSlice     ContextSlice
}

// NewBridgedKindClassificationRequest creates a fresh target-side four-input
// request. It intentionally accepts no source judgement, so no source result
// can be reused as target truth.
func NewBridgedKindClassificationRequest(
	input BridgedKindClassificationRequestInput,
) (KindClassificationRequest, error) {
	if !input.Bridge.valid() {
		return KindClassificationRequest{}, fmt.Errorf("bridged classification requires an exact KindBridge")
	}
	if !input.SourceRequest.valid() || !input.SourceSignature.valid() || !input.TargetSignature.valid() {
		return KindClassificationRequest{}, fmt.Errorf("bridged classification requires exact source request and signatures")
	}
	if input.SourceRequest.SignatureEdition() != input.SourceSignature.Ref() {
		return KindClassificationRequest{}, fmt.Errorf("bridged source request and signature edition do not match")
	}
	if input.SourceSignature.LocalKind() != input.Bridge.SourceKind() ||
		input.TargetSignature.LocalKind() != input.Bridge.TargetKind() {
		return KindClassificationRequest{}, fmt.Errorf("bridged classification signatures do not match bridge participants")
	}
	if !bytes.Equal(
		input.SourceSignature.ReferenceScheme().CanonicalBytes(),
		input.Bridge.SourceReferenceScheme().CanonicalBytes(),
	) || !bytes.Equal(
		input.TargetSignature.ReferenceScheme().CanonicalBytes(),
		input.Bridge.TargetReferenceScheme().CanonicalBytes(),
	) {
		return KindClassificationRequest{}, fmt.Errorf("bridged classification ReferenceScheme editions do not match the bridge")
	}
	if input.SourceRequest.Candidate().ValueKind() != input.TargetSignature.CandidateValueKind() {
		return KindClassificationRequest{}, fmt.Errorf("bridged target KindSignature cannot evaluate the exact source candidate ValueKind")
	}
	return NewKindClassificationRequest(KindClassificationRequestInput{
		Candidate:        input.SourceRequest.Candidate(),
		LocalKind:        input.TargetSignature.LocalKind(),
		SignatureEdition: input.TargetSignature.Ref(),
		ContextSlice:     input.TargetSlice,
	})
}
