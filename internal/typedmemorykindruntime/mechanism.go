package typedmemorykindruntime

import (
	"bytes"
	"fmt"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// EvaluationMechanism is the exact deployed-artifact coordinate committed to
// an evaluation basis. It identifies configured callable content; it is not
// executable-byte attestation or evidence that evaluation occurred.
type EvaluationMechanism struct {
	artifact typedmemory.CarrierRef
	edition  typedmemory.CarrierEdition
	digest   typedmemory.SHA256Digest
}

type EvaluationMechanismInput struct {
	Artifact typedmemory.CarrierRef
	Edition  typedmemory.CarrierEdition
	Digest   typedmemory.SHA256Digest
}

func NewEvaluationMechanism(
	input EvaluationMechanismInput,
) (EvaluationMechanism, error) {
	artifact, err := typedmemory.NewCarrierRef(input.Artifact.String())
	if err != nil || artifact != input.Artifact {
		return EvaluationMechanism{}, fmt.Errorf("evaluation mechanism artifact is invalid")
	}
	edition, err := typedmemory.NewCarrierEdition(input.Edition.String())
	if err != nil || edition != input.Edition {
		return EvaluationMechanism{}, fmt.Errorf("evaluation mechanism edition is invalid")
	}
	digest, err := typedmemory.NewSHA256Digest(input.Digest.String())
	if err != nil || digest != input.Digest {
		return EvaluationMechanism{}, fmt.Errorf("evaluation mechanism digest is invalid")
	}
	return EvaluationMechanism{
		artifact: artifact,
		edition:  edition,
		digest:   digest,
	}, nil
}

func (mechanism EvaluationMechanism) Artifact() typedmemory.CarrierRef {
	return mechanism.artifact
}

func (mechanism EvaluationMechanism) Edition() typedmemory.CarrierEdition {
	return mechanism.edition
}

func (mechanism EvaluationMechanism) Digest() typedmemory.SHA256Digest {
	return mechanism.digest
}

func (mechanism EvaluationMechanism) CanonicalBytes() []byte {
	writer := newCanonicalWriter("kind-runtime-evaluation-mechanism.v1")
	writer.addString(mechanism.artifact.String())
	writer.addString(mechanism.edition.String())
	writer.addString(mechanism.digest.String())
	return writer.bytes()
}

func validEvaluationMechanism(mechanism EvaluationMechanism) bool {
	rebuilt, err := NewEvaluationMechanism(EvaluationMechanismInput{
		Artifact: mechanism.artifact,
		Edition:  mechanism.edition,
		Digest:   mechanism.digest,
	})
	return err == nil && bytes.Equal(rebuilt.CanonicalBytes(), mechanism.CanonicalBytes())
}

func validRuleRef(rule typedmemory.RuleRef) bool {
	rebuilt, err := typedmemory.NewRuleRef(rule.String())
	return err == nil && rebuilt == rule
}

func validDigest(digest typedmemory.SHA256Digest) bool {
	rebuilt, err := typedmemory.NewSHA256Digest(digest.String())
	return err == nil && rebuilt == digest
}

func validEntitySetDefinitionRef(
	ref typedmemory.EntitySetDefinitionRef,
) bool {
	rebuilt, err := typedmemory.NewEntitySetDefinitionRef(
		ref.TypeEnv(),
		ref.Context(),
		ref.Digest(),
	)
	return err == nil && rebuilt == ref
}

func validKindSignatureRef(ref typedmemory.KindSignatureRef) bool {
	rebuilt, err := typedmemory.NewKindSignatureRef(
		ref.ValueKind(),
		ref.Context(),
		ref.Digest(),
	)
	return err == nil && rebuilt == ref
}

func validContextSliceRef(ref typedmemory.ContextSliceRef) bool {
	rebuilt, err := typedmemory.NewContextSliceRef(ref.Digest())
	return err == nil && rebuilt == ref
}

func validRepairPointer(repair typedmemory.RepairPointer) bool {
	rebuilt, err := typedmemory.NewRepairPointer(repair.String())
	return err == nil && rebuilt == repair
}

func validMemberOfEvaluationRequest(
	request typedmemory.MemberOfEvaluationRequest,
) bool {
	rebuilt, err := typedmemory.NewMemberOfEvaluationRequest(
		request.Query(),
		request.View(),
	)
	return err == nil &&
		rebuilt.Digest() == request.Digest() &&
		bytes.Equal(rebuilt.CanonicalBytes(), request.CanonicalBytes())
}

func validContextSlice(slice typedmemory.ContextSlice) bool {
	decoded, err := typedmemory.DecodeCanonicalContextSlice(slice.CanonicalBytes())
	return err == nil &&
		decoded.Ref() == slice.Ref() &&
		bytes.Equal(decoded.CanonicalBytes(), slice.CanonicalBytes())
}

func validEvaluationView(view typedmemory.MemberOfEvaluationView) bool {
	switch value := view.(type) {
	case typedmemory.PersistedSnapshotView:
		rebuilt, err := typedmemory.NewPersistedSnapshotView(
			value.TypeEnv(),
			value.PreStateGraphRevision(),
		)
		return err == nil &&
			rebuilt.Digest() == value.Digest() &&
			bytes.Equal(rebuilt.CanonicalBytes(), value.CanonicalBytes())
	case typedmemory.ProspectiveBatchView:
		rebuilt, err := typedmemory.NewProspectiveBatchView(
			typedmemory.ProspectiveBatchViewInput{
				TypeEnv:                  value.TypeEnv(),
				PreStateGraphRevision:    value.PreStateGraphRevision(),
				EvaluationChangeOrdinal:  value.EvaluationChangeOrdinal(),
				DeclarationChangeOrdinal: value.DeclarationChangeOrdinal(),
				Declaration:              value.Declaration(),
				LocalReference:           value.LocalReference(),
				PersistedReference:       value.PersistedReference(),
				OrderedCandidatePrefix:   value.OrderedCandidatePrefix(),
			},
		)
		return err == nil &&
			rebuilt.Digest() == value.Digest() &&
			bytes.Equal(rebuilt.CanonicalBytes(), value.CanonicalBytes())
	default:
		return false
	}
}

func validEntitySetDefinition(
	definition typedmemory.EntitySetDefinition,
) bool {
	rebuilt, err := typedmemory.NewEntitySetDefinition(
		typedmemory.EntitySetDefinitionInput{
			TypeEnv:         definition.Ref().TypeEnv(),
			Context:         definition.Ref().Context(),
			EnumerationRule: definition.EnumerationRule(),
			CandidatePolicy: definition.CandidatePolicy(),
			Provenance:      definition.Provenance(),
		},
	)
	return err == nil &&
		rebuilt.Ref() == definition.Ref() &&
		bytes.Equal(rebuilt.CanonicalBytes(), definition.CanonicalBytes())
}

func validKindSignatureDefinition(
	definition typedmemory.KindSignatureDefinition,
) bool {
	rebuilt, err := typedmemory.NewKindSignatureDefinition(
		typedmemory.KindSignatureDefinitionInput{
			ValueKind:       definition.ValueKind(),
			Formality:       definition.Formality(),
			Assumptions:     definition.Assumptions(),
			DefinednessRule: definition.DefinednessRule(),
			Evaluator:       definition.Evaluator(),
			EntitySet:       definition.EntitySet(),
			Provenance:      definition.Provenance(),
		},
	)
	return err == nil &&
		rebuilt.Ref() == definition.Ref() &&
		bytes.Equal(rebuilt.CanonicalBytes(), definition.CanonicalBytes())
}
