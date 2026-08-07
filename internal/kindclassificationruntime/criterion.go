package kindclassificationruntime

import (
	"context"
	"fmt"
	"sort"

	"github.com/m0n0x41d/haft/internal/kindclassificationevaluation"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

type DirectFeaturePredicateInput struct {
	Key                 typedmemory.KindFeatureKey
	Governor            typedmemory.RuleRef
	ExpectedValueKind   typedmemory.ValueKindRef
	ExpectedValueDigest typedmemory.SHA256Digest
}

type DirectFeaturePredicate struct {
	key                 typedmemory.KindFeatureKey
	governor            typedmemory.RuleRef
	expectedValueKind   typedmemory.ValueKindRef
	expectedValueDigest typedmemory.SHA256Digest
}

func NewDirectFeaturePredicate(
	input DirectFeaturePredicateInput,
) (DirectFeaturePredicate, error) {
	key, err := typedmemory.NewKindFeatureKey(input.Key.String())
	if err != nil || key != input.Key {
		return DirectFeaturePredicate{}, fmt.Errorf("classification predicate feature key is invalid")
	}
	governor, err := typedmemory.NewRuleRef(input.Governor.String())
	if err != nil || governor != input.Governor {
		return DirectFeaturePredicate{}, fmt.Errorf("classification predicate governor is invalid")
	}
	if input.ExpectedValueKind.String() == "" {
		return DirectFeaturePredicate{}, fmt.Errorf("classification predicate ValueKind is required")
	}
	digest, err := typedmemory.NewSHA256Digest(input.ExpectedValueDigest.String())
	if err != nil || digest != input.ExpectedValueDigest {
		return DirectFeaturePredicate{}, fmt.Errorf("classification predicate value digest is invalid")
	}
	return DirectFeaturePredicate{
		key:                 key,
		governor:            governor,
		expectedValueKind:   input.ExpectedValueKind,
		expectedValueDigest: digest,
	}, nil
}

func (predicate DirectFeaturePredicate) Key() typedmemory.KindFeatureKey {
	return predicate.key
}

func (predicate DirectFeaturePredicate) Governor() typedmemory.RuleRef {
	return predicate.governor
}

func (predicate DirectFeaturePredicate) ExpectedValueKind() typedmemory.ValueKindRef {
	return predicate.expectedValueKind
}

func (predicate DirectFeaturePredicate) ExpectedValueDigest() typedmemory.SHA256Digest {
	return predicate.expectedValueDigest
}

func (predicate DirectFeaturePredicate) valid() bool {
	rebuilt, err := NewDirectFeaturePredicate(DirectFeaturePredicateInput{
		Key:                 predicate.key,
		Governor:            predicate.governor,
		ExpectedValueKind:   predicate.expectedValueKind,
		ExpectedValueDigest: predicate.expectedValueDigest,
	})
	return err == nil && rebuilt == predicate
}

type DirectFeatureCriterion struct {
	rule       typedmemory.RuleRef
	predicates []DirectFeaturePredicate
}

func NewDirectFeatureCriterion(
	rule typedmemory.RuleRef,
	predicates []DirectFeaturePredicate,
) (DirectFeatureCriterion, error) {
	parsed, err := typedmemory.NewRuleRef(rule.String())
	if err != nil || parsed != rule {
		return DirectFeatureCriterion{}, fmt.Errorf("classification criterion RuleRef is invalid")
	}
	if len(predicates) == 0 {
		return DirectFeatureCriterion{}, fmt.Errorf("classification criterion requires a direct feature predicate")
	}
	normalized := append([]DirectFeaturePredicate(nil), predicates...)
	sort.Slice(normalized, func(left, right int) bool {
		return normalized[left].key.String() < normalized[right].key.String()
	})
	for index, predicate := range normalized {
		if !predicate.valid() {
			return DirectFeatureCriterion{}, fmt.Errorf("classification criterion predicate %d is invalid", index)
		}
		if index > 0 && normalized[index-1].key == predicate.key {
			return DirectFeatureCriterion{}, fmt.Errorf(
				"classification criterion repeats feature key %q",
				predicate.key.String(),
			)
		}
	}
	return DirectFeatureCriterion{rule: rule, predicates: normalized}, nil
}

func (criterion DirectFeatureCriterion) RuleRef() typedmemory.RuleRef {
	return criterion.rule
}

func (criterion DirectFeatureCriterion) Predicates() []DirectFeaturePredicate {
	return append([]DirectFeaturePredicate(nil), criterion.predicates...)
}

func (criterion DirectFeatureCriterion) valid() bool {
	rebuilt, err := NewDirectFeatureCriterion(criterion.rule, criterion.predicates)
	return err == nil && len(rebuilt.predicates) == len(criterion.predicates)
}

func (criterion DirectFeatureCriterion) EvaluateKindClassification(
	ctx context.Context,
	input kindclassificationevaluation.EvaluationInput,
) (typedmemory.KindClassificationJudgement, error) {
	if ctx == nil {
		return nil, fmt.Errorf("classification evaluation context is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("classification evaluation context: %w", err)
	}
	if !criterion.valid() || !input.Valid() {
		return nil, fmt.Errorf("classification criterion or input is invalid")
	}
	request := input.Request()
	signature := input.Signature()
	if signature.Criterion() != criterion.rule {
		return unknownJudgement(
			request,
			typedmemory.KindUnknownCriterionUnavailable,
			"repair:kind-classification/install-exact-criterion",
		)
	}
	if request.Candidate().ValueKind() != signature.CandidateValueKind() {
		return unknownJudgement(
			request,
			typedmemory.KindUnknownCandidateOutsideDomain,
			"repair:kind-classification/candidate-value-kind",
		)
	}
	if len(input.MissingDependencies()) > 0 {
		return unknownJudgement(
			request,
			typedmemory.KindUnknownDependencyUnavailable,
			"repair:kind-classification/restore-signature-dependencies",
		)
	}
	resolution := input.FeatureResolution()
	switch value := resolution.(type) {
	case kindclassificationevaluation.GovernedFeaturesUnavailable:
		return typedmemory.NewUnknownKindClassification(request, value.Reasons())
	case kindclassificationevaluation.GovernedFeaturesAvailable:
		return criterion.evaluateAvailable(request, signature, value.Features())
	default:
		return nil, fmt.Errorf("classification feature-resolution variant is invalid")
	}
}

func (criterion DirectFeatureCriterion) evaluateAvailable(
	request typedmemory.KindClassificationRequest,
	signature typedmemory.KindClassificationSignatureDefinition,
	features typedmemory.GovernedCandidateFeatureSet,
) (typedmemory.KindClassificationJudgement, error) {
	reasons := make([]typedmemory.KindClassificationUnknownReason, 0)
	contradiction := false
	for _, predicate := range criterion.predicates {
		feature, found := features.Feature(predicate.key)
		if !found {
			reason, err := unknownReason(
				typedmemory.KindUnknownMissingGovernedFeature,
				"repair:kind-classification/feature/"+predicate.key.String(),
			)
			if err != nil {
				return nil, err
			}
			reasons = append(reasons, reason)
			continue
		}
		if feature.Governor() != predicate.governor ||
			feature.Value().ValueKind() != predicate.expectedValueKind {
			reason, err := unknownReason(
				typedmemory.KindUnknownFeatureSourceMismatch,
				"repair:kind-classification/feature-source/"+predicate.key.String(),
			)
			if err != nil {
				return nil, err
			}
			reasons = append(reasons, reason)
			continue
		}
		if feature.Value().Digest() != predicate.expectedValueDigest {
			contradiction = true
		}
	}
	if len(reasons) > 0 {
		return typedmemory.NewUnknownKindClassification(request, reasons)
	}
	basis, err := typedmemory.NewKindClassificationEvaluationBasis(
		request,
		signature,
		features,
	)
	if err != nil {
		return nil, err
	}
	if contradiction {
		return typedmemory.NewFalseKindClassification(request, basis)
	}
	return typedmemory.NewTrueKindClassification(request, basis)
}

func unknownJudgement(
	request typedmemory.KindClassificationRequest,
	kind typedmemory.KindClassificationUnknownReasonKind,
	repairRaw string,
) (typedmemory.KindClassificationJudgement, error) {
	reason, err := unknownReason(kind, repairRaw)
	if err != nil {
		return nil, err
	}
	return typedmemory.NewUnknownKindClassification(
		request,
		[]typedmemory.KindClassificationUnknownReason{reason},
	)
}

func unknownReason(
	kind typedmemory.KindClassificationUnknownReasonKind,
	repairRaw string,
) (typedmemory.KindClassificationUnknownReason, error) {
	repair, err := typedmemory.NewRepairPointer(repairRaw)
	if err != nil {
		return typedmemory.KindClassificationUnknownReason{}, err
	}
	return typedmemory.NewKindClassificationUnknownReason(kind, repair)
}

var _ kindclassificationevaluation.Engine = DirectFeatureCriterion{}
