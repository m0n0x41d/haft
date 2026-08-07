package typedmemorystore

import (
	"strings"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	kindClassificationSourceBlobTable54 = "typed_memory_kind_classification_source_blobs_v54"
	kindClassificationEvaluationTable54 = "typed_memory_kind_classification_evaluations_v54"
	kindClassificationFeatureTable54    = "typed_memory_kind_classification_features_v54"
	kindClassificationUseTable54        = "typed_memory_relational_assertion_classification_uses_v54"
)

const (
	kindClassificationSourceBlobRowKind54 = "kind_classification_source_blob_v54"
	kindClassificationEvaluationRowKind54 = "kind_classification_evaluation_v54"
	kindClassificationFeatureRowKind54    = "kind_classification_feature_v54"
	kindClassificationUseRowKind54        = "relational_assertion_classification_use_v54"
)

const (
	kindClassificationSourceBlobDigestTag54 = "kind-classification-source-v54:"
	kindClassificationEvaluationDigestTag54 = "kind-classification-evaluation-v54:"
	kindClassificationFeatureDigestTag54    = "kind-classification-feature-v54:"
	kindClassificationUseDigestTag54        = "relational-assertion-classification-use-v54:"
)

type settledKindClassification struct {
	judgement typedmemory.KindClassificationJudgement
	basis     typedmemory.KindClassificationEvaluationBasis
}

func requireSettledKindClassification(
	judgement typedmemory.KindClassificationJudgement,
) (settledKindClassification, error) {
	switch value := judgement.(type) {
	case typedmemory.TrueKindClassification:
		return settledKindClassification{judgement: value, basis: value.Basis()}, nil
	case typedmemory.FalseKindClassification:
		return settledKindClassification{judgement: value, basis: value.Basis()}, nil
	default:
		return settledKindClassification{}, ErrInvalidAdmissionBatch
	}
}

func kindClassificationEvaluationRef(
	judgement typedmemory.KindClassificationJudgement,
) string {
	return derivedRef(
		"typed-memory-kind-classification-evaluation",
		judgement.Digest().String(),
	)
}

func kindClassificationFeatureSourceKind(
	feature typedmemory.GovernedCandidateFeature,
) string {
	if strings.HasPrefix(
		feature.Source().String(),
		"kind-classification-visibility:",
	) {
		return "internal_visibility"
	}
	return "external_blob"
}
