package typedmemory

import (
	"bytes"
	"fmt"
	"sort"
)

// KindClassificationAssertionPin addresses one separate C.2.1 claim-bearing
// classification assertion. The assertion may cite a judgement; it is not the
// candidate, the KindSignature, or the judgement itself.
type KindClassificationAssertionPin struct {
	versioned VersionedPin
}

func NewKindClassificationAssertionPin(
	reference CarrierRef,
	edition CarrierEdition,
	digest SHA256Digest,
) (KindClassificationAssertionPin, error) {
	pin, err := newVersionedPin(reference, edition, digest)
	if err != nil {
		return KindClassificationAssertionPin{}, fmt.Errorf("classification assertion: %w", err)
	}
	return KindClassificationAssertionPin{versioned: pin}, nil
}

func (pin KindClassificationAssertionPin) Reference() CarrierRef {
	return pin.versioned.Reference()
}

func (pin KindClassificationAssertionPin) Edition() CarrierEdition {
	return pin.versioned.Edition()
}

func (pin KindClassificationAssertionPin) Digest() SHA256Digest {
	return pin.versioned.Digest()
}

func (pin KindClassificationAssertionPin) CanonicalBytes() []byte {
	return pin.versioned.canonicalBytes("kind-classification-assertion-pin.v1")
}

func (pin KindClassificationAssertionPin) valid() bool { return pin.versioned.valid() }

// KindClassificationEvidencePin addresses supporting Evidence separately
// from the candidate-side features evaluated by the criterion. Evidence may
// support the assertion but its presence cannot construct true or false.
type KindClassificationEvidencePin struct {
	versioned VersionedPin
}

func NewKindClassificationEvidencePin(
	reference CarrierRef,
	edition CarrierEdition,
	digest SHA256Digest,
) (KindClassificationEvidencePin, error) {
	pin, err := newVersionedPin(reference, edition, digest)
	if err != nil {
		return KindClassificationEvidencePin{}, fmt.Errorf("classification Evidence: %w", err)
	}
	return KindClassificationEvidencePin{versioned: pin}, nil
}

func (pin KindClassificationEvidencePin) Reference() CarrierRef {
	return pin.versioned.Reference()
}

func (pin KindClassificationEvidencePin) Edition() CarrierEdition {
	return pin.versioned.Edition()
}

func (pin KindClassificationEvidencePin) Digest() SHA256Digest {
	return pin.versioned.Digest()
}

func (pin KindClassificationEvidencePin) CanonicalBytes() []byte {
	return pin.versioned.canonicalBytes("kind-classification-evidence-pin.v1")
}

func (pin KindClassificationEvidencePin) valid() bool { return pin.versioned.valid() }

type KindClassificationSupport struct {
	judgementDigest SHA256Digest
	assertion       KindClassificationAssertionPin
	evidence        []KindClassificationEvidencePin
	canonicalBytes  []byte
	digest          SHA256Digest
}

func NewKindClassificationSupport(
	judgement KindClassificationJudgement,
	assertion KindClassificationAssertionPin,
	evidence []KindClassificationEvidencePin,
) (KindClassificationSupport, error) {
	if !validKindClassificationJudgement(judgement) {
		return KindClassificationSupport{}, fmt.Errorf("classification support requires an existing exact judgement")
	}
	if !assertion.valid() {
		return KindClassificationSupport{}, fmt.Errorf("classification support requires a separate assertion")
	}
	normalized, err := normalizeKindClassificationEvidence(evidence)
	if err != nil {
		return KindClassificationSupport{}, err
	}
	writer := newCanonicalWriter("kind-classification-support.v1")
	writer.addString(judgement.Digest().String())
	writer.addBytes(assertion.CanonicalBytes())
	writer.addUint64(uint64(len(normalized)))
	for _, pin := range normalized {
		writer.addBytes(pin.CanonicalBytes())
	}
	return KindClassificationSupport{
		judgementDigest: judgement.Digest(),
		assertion:       assertion,
		evidence:        normalized,
		canonicalBytes:  writer.bytes(),
		digest:          writer.digest(),
	}, nil
}

func (support KindClassificationSupport) JudgementDigest() SHA256Digest {
	return support.judgementDigest
}

func (support KindClassificationSupport) Assertion() KindClassificationAssertionPin {
	return support.assertion
}

func (support KindClassificationSupport) Evidence() []KindClassificationEvidencePin {
	return append([]KindClassificationEvidencePin(nil), support.evidence...)
}

func (support KindClassificationSupport) CanonicalBytes() []byte {
	return append([]byte(nil), support.canonicalBytes...)
}

func (support KindClassificationSupport) Digest() SHA256Digest { return support.digest }

func normalizeKindClassificationEvidence(
	values []KindClassificationEvidencePin,
) ([]KindClassificationEvidencePin, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("classification support requires at least one exact Evidence pin")
	}
	result := append([]KindClassificationEvidencePin(nil), values...)
	sort.Slice(result, func(left, right int) bool {
		return bytes.Compare(result[left].CanonicalBytes(), result[right].CanonicalBytes()) < 0
	})
	normalized := make([]KindClassificationEvidencePin, 0, len(result))
	for _, pin := range result {
		if !pin.valid() {
			return nil, fmt.Errorf("classification support contains an invalid Evidence pin")
		}
		if len(normalized) == 0 {
			normalized = append(normalized, pin)
			continue
		}
		previous := normalized[len(normalized)-1]
		if previous.Reference() != pin.Reference() {
			normalized = append(normalized, pin)
			continue
		}
		if bytes.Equal(previous.CanonicalBytes(), pin.CanonicalBytes()) {
			continue
		}
		return nil, fmt.Errorf(
			"classification Evidence %q has conflicting exact pins",
			pin.Reference().String(),
		)
	}
	return normalized, nil
}

type KindGuardScopeCoverage uint8

const (
	KindGuardScopeCovered KindGuardScopeCoverage = iota + 1
	KindGuardScopeNotCovered
	KindGuardScopeUnknown
)

func (coverage KindGuardScopeCoverage) String() string {
	switch coverage {
	case KindGuardScopeCovered:
		return "covered"
	case KindGuardScopeNotCovered:
		return "not_covered"
	case KindGuardScopeUnknown:
		return "unknown"
	default:
		return ""
	}
}

func (coverage KindGuardScopeCoverage) valid() bool { return coverage.String() != "" }

type KindGuardDispositionKind uint8

const (
	KindGuardAllowed KindGuardDispositionKind = iota + 1
	KindGuardRefused
)

func (kind KindGuardDispositionKind) String() string {
	switch kind {
	case KindGuardAllowed:
		return "allowed"
	case KindGuardRefused:
		return "refused"
	default:
		return ""
	}
}

type KindGuardRefusalReason uint8

const (
	KindGuardClassificationFalse KindGuardRefusalReason = iota + 1
	KindGuardClassificationUnknown
	KindGuardScopeOutsideCoverage
	KindGuardScopeCoverageUnknown
)

func (reason KindGuardRefusalReason) String() string {
	switch reason {
	case KindGuardClassificationFalse:
		return "classification_false"
	case KindGuardClassificationUnknown:
		return "classification_unknown"
	case KindGuardScopeOutsideCoverage:
		return "scope_not_covered"
	case KindGuardScopeCoverageUnknown:
		return "scope_coverage_unknown"
	default:
		return ""
	}
}

type KindClassificationGuardDisposition struct {
	kind               KindGuardDispositionKind
	classificationKind KindClassificationJudgementKind
	judgementDigest    SHA256Digest
	scopeCoverage      KindGuardScopeCoverage
	reasons            []KindGuardRefusalReason
	canonicalBytes     []byte
	digest             SHA256Digest
}

type kindGuardRuleKey struct {
	classification KindClassificationJudgementKind
	scope          KindGuardScopeCoverage
}

type kindGuardRuleResult struct {
	disposition KindGuardDispositionKind
	reasons     []KindGuardRefusalReason
}

var failClosedKindGuardRules = map[kindGuardRuleKey]kindGuardRuleResult{
	{classification: KindClassificationTrue, scope: KindGuardScopeCovered}: {
		disposition: KindGuardAllowed,
	},
	{classification: KindClassificationTrue, scope: KindGuardScopeNotCovered}: {
		disposition: KindGuardRefused,
		reasons:     []KindGuardRefusalReason{KindGuardScopeOutsideCoverage},
	},
	{classification: KindClassificationTrue, scope: KindGuardScopeUnknown}: {
		disposition: KindGuardRefused,
		reasons:     []KindGuardRefusalReason{KindGuardScopeCoverageUnknown},
	},
	{classification: KindClassificationFalse, scope: KindGuardScopeCovered}: {
		disposition: KindGuardRefused,
		reasons:     []KindGuardRefusalReason{KindGuardClassificationFalse},
	},
	{classification: KindClassificationFalse, scope: KindGuardScopeNotCovered}: {
		disposition: KindGuardRefused,
		reasons: []KindGuardRefusalReason{
			KindGuardClassificationFalse,
			KindGuardScopeOutsideCoverage,
		},
	},
	{classification: KindClassificationFalse, scope: KindGuardScopeUnknown}: {
		disposition: KindGuardRefused,
		reasons: []KindGuardRefusalReason{
			KindGuardClassificationFalse,
			KindGuardScopeCoverageUnknown,
		},
	},
	{classification: KindClassificationUnknown, scope: KindGuardScopeCovered}: {
		disposition: KindGuardRefused,
		reasons:     []KindGuardRefusalReason{KindGuardClassificationUnknown},
	},
	{classification: KindClassificationUnknown, scope: KindGuardScopeNotCovered}: {
		disposition: KindGuardRefused,
		reasons: []KindGuardRefusalReason{
			KindGuardClassificationUnknown,
			KindGuardScopeOutsideCoverage,
		},
	},
	{classification: KindClassificationUnknown, scope: KindGuardScopeUnknown}: {
		disposition: KindGuardRefused,
		reasons: []KindGuardRefusalReason{
			KindGuardClassificationUnknown,
			KindGuardScopeCoverageUnknown,
		},
	},
}

// EvaluateFailClosedKindGuard consumes an existing judgement and a separate
// scope result. A refusal records the original three-valued classification;
// it cannot rewrite unknown to false.
func EvaluateFailClosedKindGuard(
	judgement KindClassificationJudgement,
	coverage KindGuardScopeCoverage,
) (KindClassificationGuardDisposition, error) {
	if !validKindClassificationJudgement(judgement) {
		return KindClassificationGuardDisposition{}, fmt.Errorf("kind guard requires an exact classification judgement")
	}
	if !coverage.valid() {
		return KindClassificationGuardDisposition{}, fmt.Errorf("kind guard requires a separate scope-coverage result")
	}
	result, exists := failClosedKindGuardRules[kindGuardRuleKey{
		classification: judgement.Kind(),
		scope:          coverage,
	}]
	if !exists {
		return KindClassificationGuardDisposition{}, fmt.Errorf("kind guard has no rule for the exact classification and scope pair")
	}
	writer := newCanonicalWriter("kind-classification-guard-disposition.v1")
	writer.addString(result.disposition.String())
	writer.addString(judgement.Kind().String())
	writer.addString(judgement.Digest().String())
	writer.addString(coverage.String())
	writer.addUint64(uint64(len(result.reasons)))
	for _, reason := range result.reasons {
		writer.addString(reason.String())
	}
	return KindClassificationGuardDisposition{
		kind:               result.disposition,
		classificationKind: judgement.Kind(),
		judgementDigest:    judgement.Digest(),
		scopeCoverage:      coverage,
		reasons:            append([]KindGuardRefusalReason(nil), result.reasons...),
		canonicalBytes:     writer.bytes(),
		digest:             writer.digest(),
	}, nil
}

func (disposition KindClassificationGuardDisposition) Kind() KindGuardDispositionKind {
	return disposition.kind
}

func (disposition KindClassificationGuardDisposition) ClassificationKind() KindClassificationJudgementKind {
	return disposition.classificationKind
}

func (disposition KindClassificationGuardDisposition) JudgementDigest() SHA256Digest {
	return disposition.judgementDigest
}

func (disposition KindClassificationGuardDisposition) ScopeCoverage() KindGuardScopeCoverage {
	return disposition.scopeCoverage
}

func (disposition KindClassificationGuardDisposition) Reasons() []KindGuardRefusalReason {
	return append([]KindGuardRefusalReason(nil), disposition.reasons...)
}

func (disposition KindClassificationGuardDisposition) CanonicalBytes() []byte {
	return append([]byte(nil), disposition.canonicalBytes...)
}

func (disposition KindClassificationGuardDisposition) Digest() SHA256Digest {
	return disposition.digest
}

type KindExtensionReceivingUseRef struct {
	value string
}

func NewKindExtensionReceivingUseRef(raw string) (KindExtensionReceivingUseRef, error) {
	value, err := parseOpaqueIdentifier("KindExtension receiving use", raw)
	if err != nil {
		return KindExtensionReceivingUseRef{}, err
	}
	return KindExtensionReceivingUseRef{value: value}, nil
}

func (ref KindExtensionReceivingUseRef) String() string { return ref.value }

func (ref KindExtensionReceivingUseRef) valid() bool { return ref.value != "" }

type KindExtensionProjectionInput struct {
	Signature      KindClassificationSignatureDefinition
	ContextSlice   ContextSlice
	ReceivingUse   KindExtensionReceivingUseRef
	TrueJudgements []TrueKindClassification
}

// KindExtensionProjection is an optional set-valued representation for one
// named receiving use. It is not U.EntitySet, a collection holon, an A.14
// membership occurrence, a direct classification relation, or a truth source.
type KindExtensionProjection struct {
	signatureEdition KindClassificationSignatureRef
	contextSlice     ContextSlice
	receivingUse     KindExtensionReceivingUseRef
	extentRule       RuleRef
	trueJudgements   []TrueKindClassification
	canonicalBytes   []byte
	digest           SHA256Digest
}

func NewKindExtensionProjection(
	input KindExtensionProjectionInput,
) (KindExtensionProjection, error) {
	if !input.Signature.valid() {
		return KindExtensionProjection{}, fmt.Errorf("KindExtension requires an exact KindSignature")
	}
	if !validCompleteContextSlice(input.ContextSlice) {
		return KindExtensionProjection{}, fmt.Errorf("KindExtension requires an exact ContextSlice")
	}
	if input.ContextSlice.Context() != input.Signature.LocalKind().Context() {
		return KindExtensionProjection{}, fmt.Errorf("KindExtension ContextSlice belongs to another local kind context")
	}
	if !input.ReceivingUse.valid() {
		return KindExtensionProjection{}, fmt.Errorf("KindExtension requires a named receiving use")
	}
	extentRule, declared := input.Signature.ExtentRule().(DeclaredKindExtentRule)
	if !declared || !extentRule.rule.valid() {
		return KindExtensionProjection{}, fmt.Errorf("KindExtension requires a declared ExtentRule")
	}
	judgements, err := normalizeTrueKindClassifications(
		input.Signature.Ref(),
		input.ContextSlice,
		input.TrueJudgements,
	)
	if err != nil {
		return KindExtensionProjection{}, err
	}
	writer := newCanonicalWriter("kind-extension-projection.v1")
	writer.addString(input.Signature.Ref().String())
	writer.addBytes(input.ContextSlice.CanonicalBytes())
	writer.addString(input.ReceivingUse.String())
	writer.addString(extentRule.RuleRef().String())
	writer.addUint64(uint64(len(judgements)))
	for _, judgement := range judgements {
		writer.addBytes(judgement.CanonicalBytes())
	}
	return KindExtensionProjection{
		signatureEdition: input.Signature.Ref(),
		contextSlice:     input.ContextSlice,
		receivingUse:     input.ReceivingUse,
		extentRule:       extentRule.RuleRef(),
		trueJudgements:   judgements,
		canonicalBytes:   writer.bytes(),
		digest:           writer.digest(),
	}, nil
}

func (projection KindExtensionProjection) SignatureEdition() KindClassificationSignatureRef {
	return projection.signatureEdition
}

func (projection KindExtensionProjection) ContextSlice() ContextSlice {
	return projection.contextSlice
}

func (projection KindExtensionProjection) ReceivingUse() KindExtensionReceivingUseRef {
	return projection.receivingUse
}

func (projection KindExtensionProjection) ExtentRule() RuleRef { return projection.extentRule }

func (projection KindExtensionProjection) TrueJudgements() []TrueKindClassification {
	return append([]TrueKindClassification(nil), projection.trueJudgements...)
}

func (projection KindExtensionProjection) CanonicalBytes() []byte {
	return append([]byte(nil), projection.canonicalBytes...)
}

func (projection KindExtensionProjection) Digest() SHA256Digest { return projection.digest }

func normalizeTrueKindClassifications(
	signature KindClassificationSignatureRef,
	contextSlice ContextSlice,
	values []TrueKindClassification,
) ([]TrueKindClassification, error) {
	result := append([]TrueKindClassification(nil), values...)
	sort.Slice(result, func(left, right int) bool {
		leftCandidate := result[left].Request().Candidate().Digest().String()
		rightCandidate := result[right].Request().Candidate().Digest().String()
		return leftCandidate < rightCandidate
	})
	for index, judgement := range result {
		if !validKindClassificationJudgement(judgement) {
			return nil, fmt.Errorf("KindExtension contains an invalid true judgement")
		}
		request := judgement.Request()
		if request.SignatureEdition() != signature {
			return nil, fmt.Errorf("KindExtension judgement belongs to another KindSignature edition")
		}
		if request.ContextSlice().Ref() != contextSlice.Ref() ||
			!bytes.Equal(request.ContextSlice().CanonicalBytes(), contextSlice.CanonicalBytes()) {
			return nil, fmt.Errorf("KindExtension judgement belongs to another ContextSlice")
		}
		if index > 0 && result[index-1].Request().Candidate().Digest() == request.Candidate().Digest() {
			return nil, fmt.Errorf("KindExtension repeats one exact candidate")
		}
	}
	return result, nil
}
