package kindclassificationevaluation

import (
	"bytes"
	"context"
	"fmt"
	"sort"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

type GovernedFeatureResolution interface {
	governedFeatureResolutionVariant()
}

type GovernedFeaturesAvailable struct {
	features typedmemory.GovernedCandidateFeatureSet
}

func NewGovernedFeaturesAvailable(
	request typedmemory.KindClassificationRequest,
	features typedmemory.GovernedCandidateFeatureSet,
) (GovernedFeaturesAvailable, error) {
	if !request.Valid() || !features.ValidFor(request) {
		return GovernedFeaturesAvailable{}, fmt.Errorf(
			"available governed features must match the exact classification request",
		)
	}
	return GovernedFeaturesAvailable{features: features}, nil
}

func (resolution GovernedFeaturesAvailable) Features() typedmemory.GovernedCandidateFeatureSet {
	return resolution.features
}

func (GovernedFeaturesAvailable) governedFeatureResolutionVariant() {}

type GovernedFeaturesUnavailable struct {
	reasons []typedmemory.KindClassificationUnknownReason
}

func NewGovernedFeaturesUnavailable(
	request typedmemory.KindClassificationRequest,
	reasons []typedmemory.KindClassificationUnknownReason,
) (GovernedFeaturesUnavailable, error) {
	unknown, err := typedmemory.NewUnknownKindClassification(request, reasons)
	if err != nil {
		return GovernedFeaturesUnavailable{}, fmt.Errorf(
			"unavailable governed features require exact reasons: %w",
			err,
		)
	}
	return GovernedFeaturesUnavailable{reasons: unknown.Reasons()}, nil
}

func (resolution GovernedFeaturesUnavailable) Reasons() []typedmemory.KindClassificationUnknownReason {
	return append([]typedmemory.KindClassificationUnknownReason(nil), resolution.reasons...)
}

func (GovernedFeaturesUnavailable) governedFeatureResolutionVariant() {}

type EvaluationInput struct {
	request               typedmemory.KindClassificationRequest
	signature             typedmemory.KindClassificationSignatureDefinition
	features              GovernedFeatureResolution
	availableDependencies []typedmemory.KindSignatureDependencyPin
}

func NewEvaluationInput(
	request typedmemory.KindClassificationRequest,
	signature typedmemory.KindClassificationSignatureDefinition,
	features GovernedFeatureResolution,
	availableDependencies []typedmemory.KindSignatureDependencyPin,
) (EvaluationInput, error) {
	if !request.Valid() {
		return EvaluationInput{}, fmt.Errorf("classification evaluation requires an exact request")
	}
	if !signature.Valid() || signature.Ref() != request.SignatureEdition() {
		return EvaluationInput{}, fmt.Errorf(
			"classification evaluation requires the request's exact KindSignature edition",
		)
	}
	if !validFeatureResolution(request, features) {
		return EvaluationInput{}, fmt.Errorf(
			"classification evaluation requires an exact governed-feature posture",
		)
	}
	dependencies, err := normalizeDependencies(availableDependencies)
	if err != nil {
		return EvaluationInput{}, err
	}
	return EvaluationInput{
		request:               request,
		signature:             signature,
		features:              features,
		availableDependencies: dependencies,
	}, nil
}

func (input EvaluationInput) Request() typedmemory.KindClassificationRequest {
	return input.request
}

func (input EvaluationInput) Signature() typedmemory.KindClassificationSignatureDefinition {
	return input.signature
}

func (input EvaluationInput) FeatureResolution() GovernedFeatureResolution {
	return input.features
}

func (input EvaluationInput) AvailableDependencies() []typedmemory.KindSignatureDependencyPin {
	return append(
		[]typedmemory.KindSignatureDependencyPin(nil),
		input.availableDependencies...,
	)
}

func (input EvaluationInput) Valid() bool {
	rebuilt, err := NewEvaluationInput(
		input.request,
		input.signature,
		input.features,
		input.availableDependencies,
	)
	return err == nil && sameEvaluationInput(rebuilt, input)
}

func (input EvaluationInput) MissingDependencies() []typedmemory.KindSignatureDependencyPin {
	declared := input.signature.Dependencies()
	missing := make([]typedmemory.KindSignatureDependencyPin, 0)
	for _, dependency := range declared {
		if !containsDependency(input.availableDependencies, dependency) {
			missing = append(missing, dependency)
		}
	}
	return missing
}

func validFeatureResolution(
	request typedmemory.KindClassificationRequest,
	resolution GovernedFeatureResolution,
) bool {
	switch value := resolution.(type) {
	case GovernedFeaturesAvailable:
		return value.features.ValidFor(request)
	case GovernedFeaturesUnavailable:
		_, err := typedmemory.NewUnknownKindClassification(request, value.reasons)
		return err == nil
	default:
		return false
	}
}

func normalizeDependencies(
	dependencies []typedmemory.KindSignatureDependencyPin,
) ([]typedmemory.KindSignatureDependencyPin, error) {
	result := append([]typedmemory.KindSignatureDependencyPin(nil), dependencies...)
	sort.Slice(result, func(left, right int) bool {
		return bytes.Compare(
			result[left].CanonicalBytes(),
			result[right].CanonicalBytes(),
		) < 0
	})
	for index, dependency := range result {
		if !dependency.Valid() {
			return nil, fmt.Errorf("available KindSignature dependency %d is invalid", index)
		}
		if index > 0 && bytes.Equal(
			result[index-1].CanonicalBytes(),
			dependency.CanonicalBytes(),
		) {
			return nil, fmt.Errorf("available KindSignature dependency is repeated")
		}
	}
	return result, nil
}

func containsDependency(
	dependencies []typedmemory.KindSignatureDependencyPin,
	want typedmemory.KindSignatureDependencyPin,
) bool {
	index := sort.Search(len(dependencies), func(index int) bool {
		return bytes.Compare(
			dependencies[index].CanonicalBytes(),
			want.CanonicalBytes(),
		) >= 0
	})
	return index < len(dependencies) && bytes.Equal(
		dependencies[index].CanonicalBytes(),
		want.CanonicalBytes(),
	)
}

func sameEvaluationInput(left EvaluationInput, right EvaluationInput) bool {
	if left.request.Digest() != right.request.Digest() ||
		left.signature.Ref() != right.signature.Ref() ||
		len(left.availableDependencies) != len(right.availableDependencies) {
		return false
	}
	for index := range left.availableDependencies {
		if !bytes.Equal(
			left.availableDependencies[index].CanonicalBytes(),
			right.availableDependencies[index].CanonicalBytes(),
		) {
			return false
		}
	}
	switch leftFeatures := left.features.(type) {
	case GovernedFeaturesAvailable:
		rightFeatures, ok := right.features.(GovernedFeaturesAvailable)
		return ok && leftFeatures.features.Digest() == rightFeatures.features.Digest()
	case GovernedFeaturesUnavailable:
		rightFeatures, ok := right.features.(GovernedFeaturesUnavailable)
		if !ok || len(leftFeatures.reasons) != len(rightFeatures.reasons) {
			return false
		}
		for index := range leftFeatures.reasons {
			if !bytes.Equal(
				leftFeatures.reasons[index].CanonicalBytes(),
				rightFeatures.reasons[index].CanonicalBytes(),
			) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

type Engine interface {
	EvaluateKindClassification(
		context.Context,
		EvaluationInput,
	) (typedmemory.KindClassificationJudgement, error)
}
