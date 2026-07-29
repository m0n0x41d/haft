package kindclassificationengine

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/kindclassificationevaluation"
	"github.com/m0n0x41d/haft/internal/kindclassificationruntime"
	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/projectmemory/carrierfamily"
	"github.com/m0n0x41d/haft/internal/projectmemory/recordcarrier"
	"github.com/m0n0x41d/haft/internal/projecttypeenvruntime"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorycandidatecodec"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
)

const (
	projectObjectFamilyFeatureKey  = "haft.project-object.family"
	projectRecordVariantFeatureKey = "haft.project-record.variant"
	projectObjectFamilyGovernor    = "haft.feature.project-object-family/v1"
	projectRecordVariantGovernor   = "haft.feature.project-record-carrier/v1"
	projectRecordFamilyToken       = "project_record"
	entityPresenceFeatureKey       = "haft.entity.identity-present"
	entityPresenceFeatureGovernor  = "haft.feature.entity-visibility/v1"
	entityPresenceFeatureToken     = "present"
)

var (
	ErrRecordKindClassificationRuntimeMissing = errors.New(
		"project-memory record kind-classification runtime is missing",
	)
	ErrRecordKindClassificationRuntimeInvalid = errors.New(
		"project-memory record kind-classification runtime is invalid",
	)
)

// ProjectKindClassificationAdmissionEngine converts one exact, trusted
// project-record or carrier-family delivery into direct governed candidate
// features and then delegates only the criterion decision to the exact RuleRef
// registry selected by X. Source delivery is neither Evidence nor truth by
// construction.
type ProjectKindClassificationAdmissionEngine struct {
	registry kindclassificationruntime.Registry
}

var _ typedmemorystore.KindClassificationAdmissionEngine = ProjectKindClassificationAdmissionEngine{}
var _ typedmemorystore.ExactKindClassificationAdmissionEngine = ProjectKindClassificationAdmissionEngine{}
var _ typedmemorystore.SealedHistoricalKindClassificationSourceAdapter = ProjectKindClassificationAdmissionEngine{}

func NewProjectKindClassificationAdmissionEngine(
	registry kindclassificationruntime.Registry,
) (ProjectKindClassificationAdmissionEngine, error) {
	if registry.Len() == 0 {
		return ProjectKindClassificationAdmissionEngine{},
			ErrRecordKindClassificationRuntimeMissing
	}
	return ProjectKindClassificationAdmissionEngine{
		registry: registry.Clone(),
	}, nil
}

// ForExactTargetRuntime constructs the current engine only when the selected
// target X declares direct KindClassification. Historical MemberOf targets
// return an absent engine; the closed reference-kind loader selects their
// separate compatibility variant.
func ForExactTargetRuntime(
	runtime projecttypeenvruntime.ExactTargetRuntimeRegistry,
) (typedmemorystore.ExactKindClassificationAdmissionEngine, error) {
	if !runtime.Valid() {
		return nil, ErrRecordKindClassificationRuntimeInvalid
	}
	registry, available := runtime.KindClassificationRegistry()
	if !available {
		return nil, ErrRecordKindClassificationRuntimeInvalid
	}
	if registry.Len() == 0 {
		return nil, nil
	}
	engine, err := NewProjectKindClassificationAdmissionEngine(registry)
	if err != nil {
		return nil, err
	}
	return engine, nil
}

// ExactKindClassificationRegistry returns the immutable evaluator identity set
// from which this admission engine was built. Storage adapters use it only to
// correlate the callable engine with the exact target X; it grants no source,
// graph, admission, or persistence capability.
func (engine ProjectKindClassificationAdmissionEngine) ExactKindClassificationRegistry() kindclassificationruntime.Registry {
	return engine.registry.Clone()
}

// AdaptSealedHistoricalKindClassificationSources recovers only the exact
// carrier and project binding from a sealed RecordMembership delivery source.
// It neither consumes the historical MemberOf judgement nor writes the derived
// current source. Classification is evaluated afresh by the selected target X.
func (engine ProjectKindClassificationAdmissionEngine) AdaptSealedHistoricalKindClassificationSources(
	project projectledger.ProjectID,
	observables []typedmemorystore.ObservableInputBlob,
) ([]typedmemorystore.KindClassificationSourceBlob, error) {
	if engine.registry.Len() == 0 {
		return nil, ErrRecordKindClassificationRuntimeInvalid
	}
	adapted := make([]typedmemorystore.KindClassificationSourceBlob, 0)
	for _, observable := range observables {
		if !strings.HasPrefix(
			observable.Reference().String(),
			"record-membership-source:",
		) {
			continue
		}
		expected, err := typedmemory.NewMemberOfObservableInput(
			observable.Reference(),
			observable.Digest(),
		)
		if err != nil {
			return nil, fmt.Errorf(
				"adapt sealed record delivery observable: %w",
				err,
			)
		}
		historical, err := recordcarrier.VerifyRecordMembershipSourceV1(
			expected,
			observable.Bytes(),
		)
		if err != nil {
			return nil, fmt.Errorf(
				"adapt sealed record delivery source: %w",
				err,
			)
		}
		if historical.ProjectID() != project {
			return nil, fmt.Errorf(
				"adapt sealed record delivery source: project mismatch",
			)
		}
		current, err := recordcarrier.SealRecordClassificationSourceV1(
			historical.ProjectID(),
			historical.EntityID(),
			historical.BoundedContext(),
			historical.Carrier(),
			historical.Binding(),
		)
		if err != nil {
			return nil, fmt.Errorf(
				"derive current record classification delivery source: %w",
				err,
			)
		}
		blob, err := typedmemorystore.NewKindClassificationSourceBlob(
			current.Ref(),
			current.Digest(),
			current.CanonicalBytes(),
		)
		if err != nil {
			return nil, fmt.Errorf(
				"seal adapted record classification delivery blob: %w",
				err,
			)
		}
		adapted = append(adapted, blob)
	}
	return adapted, nil
}

// RecordKindClassificationAdmissionEngine is retained as a source-compatible
// internal alias while historical call sites move to the project-wide current
// engine. It does not select the historical MemberOf runtime.
type RecordKindClassificationAdmissionEngine = ProjectKindClassificationAdmissionEngine

func NewRecordKindClassificationAdmissionEngine(
	registry kindclassificationruntime.Registry,
) (RecordKindClassificationAdmissionEngine, error) {
	return NewProjectKindClassificationAdmissionEngine(registry)
}

func (engine ProjectKindClassificationAdmissionEngine) EvaluateKindClassification(
	ctx context.Context,
	input typedmemorystore.KindClassificationAdmissionInput,
) (typedmemory.KindClassificationJudgement, error) {
	if ctx == nil {
		return nil, fmt.Errorf("record kind classification requires context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if engine.registry.Len() == 0 {
		return nil, ErrRecordKindClassificationRuntimeInvalid
	}
	request := input.Request()
	signature, found := input.Environment().KindClassificationSignatureDefinition(
		request.LocalKind(),
	)
	if !found || signature.Ref() != request.SignatureEdition() {
		return nil, ErrRecordKindClassificationRuntimeInvalid
	}
	features := resolveProjectClassificationFeatures(input)
	evaluation, err := kindclassificationevaluation.NewEvaluationInput(
		request,
		signature,
		features,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"construct record kind-classification evaluation: %w",
			err,
		)
	}
	return engine.registry.EvaluateKindClassification(ctx, evaluation)
}

func resolveProjectClassificationFeatures(
	input typedmemorystore.KindClassificationAdmissionInput,
) kindclassificationevaluation.GovernedFeatureResolution {
	request := input.Request()
	candidate, exactEntity := request.Candidate().(typedmemory.ExactKindEntityCandidate)
	if !exactEntity {
		return unavailableRecordClassificationFeatures(
			request,
			typedmemory.KindUnknownCandidateOutsideDomain,
			"repair:kind-classification/use-entity-candidate",
		)
	}
	identityFeature, err := governedEntityPresenceFeature(input)
	if err != nil {
		return unavailableRecordClassificationFeatures(
			request,
			typedmemory.KindUnknownFeatureSourceMalformed,
			"repair:kind-classification/rebuild-visibility-feature",
		)
	}
	recordSources := make([]recordcarrier.RecordClassificationSourceV1, 0, 1)
	carrierSources := make([]carrierfamily.ClassificationSourceV1, 0, 1)
	for _, blob := range input.Sources() {
		if strings.HasPrefix(
			blob.Reference().String(),
			"record-classification-source:",
		) {
			source, err := recordcarrier.VerifyRecordClassificationSourceV1(
				blob.Reference(),
				blob.Digest(),
				blob.Bytes(),
			)
			if err != nil {
				return unavailableRecordClassificationFeatures(
					request,
					typedmemory.KindUnknownFeatureSourceMalformed,
					"repair:kind-classification/replace-malformed-record-source",
				)
			}
			if source.EntityID() != candidate.EntityID() {
				continue
			}
			if source.ProjectID() != input.ProjectID() ||
				source.BoundedContext() != request.ContextSlice().Context() {
				return unavailableRecordClassificationFeatures(
					request,
					typedmemory.KindUnknownFeatureSourceMismatch,
					"repair:kind-classification/correlate-record-source",
				)
			}
			recordSources = append(recordSources, source)
			continue
		}
		if !carrierfamily.IsClassificationSourceReference(blob.Reference()) {
			continue
		}
		source, err := carrierfamily.VerifyClassificationSourceV1(
			blob.Reference(),
			blob.Digest(),
			blob.Bytes(),
		)
		if err != nil {
			return unavailableRecordClassificationFeatures(
				request,
				typedmemory.KindUnknownFeatureSourceMalformed,
				"repair:kind-classification/replace-malformed-carrier-source",
			)
		}
		if source.EntityID() != candidate.EntityID() {
			continue
		}
		if source.ProjectID() != input.ProjectID() ||
			source.BoundedContext() != request.ContextSlice().Context() {
			return unavailableRecordClassificationFeatures(
				request,
				typedmemory.KindUnknownFeatureSourceMismatch,
				"repair:kind-classification/correlate-carrier-source",
			)
		}
		carrierSources = append(carrierSources, source)
	}
	sourceCount := len(recordSources) + len(carrierSources)
	if sourceCount == 0 {
		return availableRecordClassificationFeatures(
			request,
			[]typedmemory.GovernedCandidateFeature{identityFeature},
		)
	}
	if sourceCount != 1 {
		return unavailableRecordClassificationFeatures(
			request,
			typedmemory.KindUnknownFeatureSourceUntrusted,
			"repair:kind-classification/select-one-record-source",
		)
	}
	features := []typedmemory.GovernedCandidateFeature{identityFeature}
	if len(recordSources) == 1 {
		recordFeatures, err := governedRecordClassificationFeatures(
			input,
			recordSources[0],
		)
		if err != nil {
			return unavailableRecordClassificationFeatures(
				request,
				typedmemory.KindUnknownFeatureSourceMalformed,
				"repair:kind-classification/rebuild-record-features",
			)
		}
		features = append(features, recordFeatures...)
	}
	if len(carrierSources) == 1 {
		carrierFeatures, err := governedCarrierFamilyClassificationFeatures(
			input,
			carrierSources[0],
		)
		if err != nil {
			return unavailableRecordClassificationFeatures(
				request,
				typedmemory.KindUnknownFeatureSourceMalformed,
				"repair:kind-classification/rebuild-carrier-features",
			)
		}
		features = append(features, carrierFeatures...)
	}
	return availableRecordClassificationFeatures(request, features)
}

func governedCarrierFamilyClassificationFeatures(
	input typedmemorystore.KindClassificationAdmissionInput,
	source carrierfamily.ClassificationSourceV1,
) ([]typedmemory.GovernedCandidateFeature, error) {
	familyGovernor, err := typedmemory.NewRuleRef(projectObjectFamilyGovernor)
	if err != nil {
		return nil, err
	}
	familyKey, err := typedmemory.NewKindFeatureKey(projectObjectFamilyFeatureKey)
	if err != nil {
		return nil, err
	}
	familyValue, err := verifiedRecordFeatureText(
		input.Environment(),
		input.Codecs(),
		source.FamilyToken(),
	)
	if err != nil {
		return nil, err
	}
	family, err := typedmemory.NewGovernedCandidateFeature(
		typedmemory.GovernedCandidateFeatureInput{
			Key:          familyKey,
			Value:        familyValue,
			Governor:     familyGovernor,
			Source:       source.Ref(),
			SourceDigest: source.Digest(),
		},
	)
	if err != nil {
		return nil, err
	}
	return []typedmemory.GovernedCandidateFeature{family}, nil
}

func governedRecordClassificationFeatures(
	input typedmemorystore.KindClassificationAdmissionInput,
	source recordcarrier.RecordClassificationSourceV1,
) ([]typedmemory.GovernedCandidateFeature, error) {
	familyGovernor, err := typedmemory.NewRuleRef(projectObjectFamilyGovernor)
	if err != nil {
		return nil, err
	}
	variantGovernor, err := typedmemory.NewRuleRef(projectRecordVariantGovernor)
	if err != nil {
		return nil, err
	}
	familyKey, err := typedmemory.NewKindFeatureKey(projectObjectFamilyFeatureKey)
	if err != nil {
		return nil, err
	}
	variantKey, err := typedmemory.NewKindFeatureKey(projectRecordVariantFeatureKey)
	if err != nil {
		return nil, err
	}
	familyValue, err := verifiedRecordFeatureText(
		input.Environment(),
		input.Codecs(),
		projectRecordFamilyToken,
	)
	if err != nil {
		return nil, err
	}
	variantValue, err := verifiedRecordFeatureText(
		input.Environment(),
		input.Codecs(),
		source.RecordVariant().Token(),
	)
	if err != nil {
		return nil, err
	}
	family, err := typedmemory.NewGovernedCandidateFeature(
		typedmemory.GovernedCandidateFeatureInput{
			Key:          familyKey,
			Value:        familyValue,
			Governor:     familyGovernor,
			Source:       source.Ref(),
			SourceDigest: source.Digest(),
		},
	)
	if err != nil {
		return nil, err
	}
	variant, err := typedmemory.NewGovernedCandidateFeature(
		typedmemory.GovernedCandidateFeatureInput{
			Key:          variantKey,
			Value:        variantValue,
			Governor:     variantGovernor,
			Source:       source.Ref(),
			SourceDigest: source.Digest(),
		},
	)
	if err != nil {
		return nil, err
	}
	return []typedmemory.GovernedCandidateFeature{family, variant}, nil
}

func governedEntityPresenceFeature(
	input typedmemorystore.KindClassificationAdmissionInput,
) (typedmemory.GovernedCandidateFeature, error) {
	reference, digest, found :=
		typedmemorystore.KindClassificationVisibilitySourceCoordinate(
			input.Visibility(),
		)
	if !found {
		return typedmemory.GovernedCandidateFeature{}, fmt.Errorf(
			"entity-presence visibility coordinate is unavailable",
		)
	}
	key, err := typedmemory.NewKindFeatureKey(entityPresenceFeatureKey)
	if err != nil {
		return typedmemory.GovernedCandidateFeature{}, err
	}
	governor, err := typedmemory.NewRuleRef(entityPresenceFeatureGovernor)
	if err != nil {
		return typedmemory.GovernedCandidateFeature{}, err
	}
	value, err := verifiedRecordFeatureText(
		input.Environment(),
		input.Codecs(),
		entityPresenceFeatureToken,
	)
	if err != nil {
		return typedmemory.GovernedCandidateFeature{}, err
	}
	return typedmemory.NewGovernedCandidateFeature(
		typedmemory.GovernedCandidateFeatureInput{
			Key:          key,
			Value:        value,
			Governor:     governor,
			Source:       reference,
			SourceDigest: digest,
		},
	)
}

func availableRecordClassificationFeatures(
	request typedmemory.KindClassificationRequest,
	features []typedmemory.GovernedCandidateFeature,
) kindclassificationevaluation.GovernedFeatureResolution {
	set, err := typedmemory.NewGovernedCandidateFeatureSet(request, features)
	if err != nil {
		return nil
	}
	available, err := kindclassificationevaluation.NewGovernedFeaturesAvailable(
		request,
		set,
	)
	if err != nil {
		return nil
	}
	return available
}

func verifiedRecordFeatureText(
	environment typedmemory.TypeEnv,
	registry typedmemory.CodecRegistry,
	raw string,
) (typedmemory.VerifiedTypedValue, error) {
	kindID, err := typedmemory.NewKindID("Haft.Text")
	if err != nil {
		return nil, err
	}
	valueKind, err := typedmemory.NewValueKindRef(environment.Ref(), kindID)
	if err != nil {
		return nil, err
	}
	binding, found := environment.ValueBinding(valueKind)
	if !found {
		return nil, fmt.Errorf("record feature Text binding is unavailable")
	}
	suite, err := typedmemorycandidatecodec.NewSuite(environment.ValueShapes())
	if err != nil {
		return nil, err
	}
	encoded := suite.Text().EncodeInput(raw)
	canonical, ok := encoded.(typedmemory.CanonicalizedCodecValue)
	if !ok {
		return nil, fmt.Errorf("record feature Text codec rejected the token")
	}
	candidate, err := typedmemory.NewTypedValueCandidate(
		binding.ValueKind(),
		binding.ValueShape(),
		binding.Codec(),
		canonical.CanonicalBytes(),
		typedmemory.NoAssertedDigest{},
	)
	if err != nil {
		return nil, err
	}
	verification := typedmemory.VerifyTypedValue(registry, binding, candidate)
	valid, ok := verification.(typedmemory.ValidTypedValue)
	if !ok {
		return nil, fmt.Errorf("record feature Text value did not verify")
	}
	return valid.Value(), nil
}

// VerifyProjectFeatureText constructs the exact Haft.Text feature value used
// by both the criterion registry and the admission engine. Keeping one codec
// path prevents registry predicates and extracted features from drifting.
func VerifyProjectFeatureText(
	environment typedmemory.TypeEnv,
	registry typedmemory.CodecRegistry,
	raw string,
) (typedmemory.VerifiedTypedValue, error) {
	return verifiedRecordFeatureText(environment, registry, raw)
}

func unavailableRecordClassificationFeatures(
	request typedmemory.KindClassificationRequest,
	kind typedmemory.KindClassificationUnknownReasonKind,
	repairRaw string,
) kindclassificationevaluation.GovernedFeatureResolution {
	repair, err := typedmemory.NewRepairPointer(repairRaw)
	if err != nil {
		return nil
	}
	reason, err := typedmemory.NewKindClassificationUnknownReason(kind, repair)
	if err != nil {
		return nil
	}
	unavailable, err := kindclassificationevaluation.NewGovernedFeaturesUnavailable(
		request,
		[]typedmemory.KindClassificationUnknownReason{reason},
	)
	if err != nil {
		return nil
	}
	return unavailable
}
