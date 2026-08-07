package typedmemorykindruntime

import (
	"bytes"
	"fmt"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// C32CandidateEvaluationEvidence is the closed performed-evaluation posture
// used by certificate conversion. Persisted evaluation has no candidate
// invocation; prospective evaluation must provide its exact request and
// successful result.
type C32CandidateEvaluationEvidence interface {
	c32CandidateEvaluationEvidenceVariant()
}

type PersistedC32CandidateEvidence struct{}

func NewPersistedC32CandidateEvidence() PersistedC32CandidateEvidence {
	return PersistedC32CandidateEvidence{}
}

func (PersistedC32CandidateEvidence) c32CandidateEvaluationEvidenceVariant() {}

type ProspectiveC32CandidateEvidence struct {
	request CandidateVisibilityRequest
	result  CandidateVisible
}

func NewProspectiveC32CandidateEvidence(
	request CandidateVisibilityRequest,
	result CandidateVisible,
) (ProspectiveC32CandidateEvidence, error) {
	if !request.valid() || !validCandidateVisibilityResult(result) {
		return ProspectiveC32CandidateEvidence{}, fmt.Errorf(
			"prospective C.3.2 candidate evidence requires exact request and visible result",
		)
	}
	basis := result.Basis()
	if result.DefinitionRef() != request.Definition().Ref() ||
		result.EvaluationViewDigest() != request.View().Digest() ||
		basis.Rule() != request.Definition().CandidatePolicy().(typedmemory.PriorBatchDeclarationsVisible).EvaluationRule() {
		return ProspectiveC32CandidateEvidence{}, fmt.Errorf(
			"prospective C.3.2 candidate result does not match its request",
		)
	}
	return ProspectiveC32CandidateEvidence{
		request: request,
		result:  result,
	}, nil
}

func (ProspectiveC32CandidateEvidence) c32CandidateEvaluationEvidenceVariant() {}

type C32PrerequisiteCertificateFromResultsInput struct {
	EnumerationRequest EntitySetEnumerationRequest
	EnumerationResult  EntitySetEnumerated
	DefinednessRequest KindDefinednessRequest
	DefinednessResult  KindDefined
	CandidateEvidence  C32CandidateEvaluationEvidence
}

// NewC32PrerequisiteCertificateFromResults converts actual sealed successful
// results into the cycle-free typedmemory coordinate consumed by MemberOfBasis
// v3. Digest-shaped caller input alone cannot satisfy this factory.
func NewC32PrerequisiteCertificateFromResults(
	input C32PrerequisiteCertificateFromResultsInput,
) (typedmemory.C32PrerequisiteCertificate, error) {
	if !input.EnumerationRequest.valid() ||
		!validEntitySetEnumerationResult(input.EnumerationResult) ||
		!input.DefinednessRequest.valid() ||
		!validKindDefinednessResult(input.DefinednessResult) {
		return typedmemory.C32PrerequisiteCertificate{}, fmt.Errorf(
			"C.3.2 certificate conversion requires exact sealed requests and successful results",
		)
	}
	if err := correlateEnumerationResult(
		input.EnumerationRequest,
		input.EnumerationResult,
	); err != nil {
		return typedmemory.C32PrerequisiteCertificate{}, err
	}
	if err := correlateDefinednessResult(
		input.DefinednessRequest,
		input.DefinednessResult,
		input.EnumerationResult,
	); err != nil {
		return typedmemory.C32PrerequisiteCertificate{}, err
	}
	visibility, err := c32CandidateVisibilityCoordinate(
		input.EnumerationRequest,
		input.CandidateEvidence,
	)
	if err != nil {
		return typedmemory.C32PrerequisiteCertificate{}, err
	}
	enumerationMechanism, err := c32MechanismIdentity(
		input.EnumerationResult.Basis().Mechanism(),
	)
	if err != nil {
		return typedmemory.C32PrerequisiteCertificate{}, err
	}
	definednessMechanism, err := c32MechanismIdentity(
		input.DefinednessResult.Basis().Mechanism(),
	)
	if err != nil {
		return typedmemory.C32PrerequisiteCertificate{}, err
	}
	memberOfRequest := input.DefinednessRequest.MemberOfRequest()
	signature := input.DefinednessRequest.Signature()
	return typedmemory.NewC32PrerequisiteCertificate(
		typedmemory.C32PrerequisiteCertificateInput{
			TypeEnv:                  memberOfRequest.View().TypeEnv(),
			KindSignature:            signature.Ref(),
			EntitySet:                signature.EntitySet(),
			ContextSlice:             memberOfRequest.Query().ContextSlice().Ref(),
			EvaluationView:           memberOfRequest.View(),
			MemberOfRequestDigest:    memberOfRequest.Digest(),
			EnumerationRequestDigest: input.EnumerationRequest.Digest(),
			EnumerationResultDigest:  input.EnumerationResult.Digest(),
			EnumerationBasisDigest:   input.EnumerationResult.Basis().Digest(),
			EnumerationRule:          input.EnumerationResult.Basis().Rule(),
			EnumerationMechanism:     enumerationMechanism,
			DefinednessRequestDigest: input.DefinednessRequest.Digest(),
			DefinednessResultDigest:  input.DefinednessResult.Digest(),
			DefinednessBasisDigest:   input.DefinednessResult.Basis().Digest(),
			DefinednessRule:          input.DefinednessResult.Basis().Rule(),
			DefinednessMechanism:     definednessMechanism,
			CandidateVisibility:      visibility,
		},
	)
}

func correlateEnumerationResult(
	request EntitySetEnumerationRequest,
	result EntitySetEnumerated,
) error {
	basis := result.Basis()
	observation, exact := request.Observation().(ExactEntitySetObservation)
	if !exact ||
		result.DefinitionRef() != request.Definition().Ref() ||
		result.ContextSliceRef() != request.ContextSlice().Ref() ||
		result.EvaluationViewDigest() != request.View().Digest() ||
		basis.Rule() != request.Definition().EnumerationRule() ||
		!bytes.Equal(
			basis.CandidateBasis().CanonicalBytes(),
			request.Candidates().CanonicalBytes(),
		) ||
		!exactEntityIDs(result.Entities(), observation.Entities()) ||
		!exactObservableInputs(
			basis.ObservableInputs(),
			observation.ObservableInputs(),
		) {
		return fmt.Errorf(
			"C.3.2 EntitySet result does not match its exact request",
		)
	}
	return nil
}

func correlateDefinednessResult(
	request KindDefinednessRequest,
	result KindDefined,
	enumeration EntitySetEnumerated,
) error {
	basis := result.Basis()
	observation, exact := request.Observation().(ExactKindDefinednessObservation)
	if !exact ||
		request.Enumeration().Digest() != enumeration.Digest() ||
		result.MemberOfRequestDigest() != request.MemberOfRequest().Digest() ||
		result.SignatureRef() != request.Signature().Ref() ||
		basis.EntitySetRef() != request.Signature().EntitySet() ||
		basis.ContextSliceRef() != request.MemberOfRequest().Query().ContextSlice().Ref() ||
		basis.EvaluationViewDigest() != request.MemberOfRequest().View().Digest() ||
		basis.EnumerationDigest() != enumeration.Digest() ||
		basis.Rule() != request.Signature().DefinednessRule() ||
		!exactObservableInputs(
			basis.ObservableInputs(),
			observation.ObservableInputs(),
		) ||
		!exactKindAssumptionPins(
			basis.MatchedAssumptions(),
			request.Signature().Assumptions(),
		) {
		return fmt.Errorf(
			"C.3.2 KindDefined result does not match its exact request and EntitySet result",
		)
	}
	return nil
}

func c32CandidateVisibilityCoordinate(
	enumeration EntitySetEnumerationRequest,
	evidence C32CandidateEvaluationEvidence,
) (typedmemory.C32CandidateVisibilityCoordinate, error) {
	switch exactView := enumeration.View().(type) {
	case typedmemory.PersistedSnapshotView:
		_, persisted := evidence.(PersistedC32CandidateEvidence)
		_, persistedBasis := enumeration.Candidates().(PersistedEntitySetCandidateBasis)
		if !persisted || !persistedBasis {
			return nil, fmt.Errorf(
				"persisted C.3.2 enumeration requires persisted candidate evidence",
			)
		}
		if exactView.Digest() != enumeration.Candidates().(PersistedEntitySetCandidateBasis).EvaluationViewDigest() {
			return nil, fmt.Errorf(
				"persisted C.3.2 candidate evidence belongs to another view",
			)
		}
		return typedmemory.NewC32PersistedVisibilityCoordinate(), nil
	case typedmemory.ProspectiveBatchView:
		prospective, ok := evidence.(ProspectiveC32CandidateEvidence)
		if !ok {
			return nil, fmt.Errorf(
				"prospective C.3.2 enumeration requires exact candidate request and result",
			)
		}
		if prospective.request.Definition().Ref() != enumeration.Definition().Ref() ||
			prospective.request.View().Digest() != exactView.Digest() {
			return nil, fmt.Errorf(
				"prospective C.3.2 candidate evidence does not match enumeration",
			)
		}
		basis := prospective.result.Basis()
		mechanism, err := c32MechanismIdentity(basis.Mechanism())
		if err != nil {
			return nil, err
		}
		return typedmemory.NewC32ProspectiveVisibilityCoordinate(
			typedmemory.C32ProspectiveVisibilityCoordinateInput{
				RequestDigest: prospective.request.Digest(),
				ResultDigest:  prospective.result.Digest(),
				BasisDigest:   basis.Digest(),
				Rule:          basis.Rule(),
				Mechanism:     mechanism,
			},
		)
	default:
		return nil, fmt.Errorf("C.3.2 evaluation view is unsupported")
	}
}

func c32MechanismIdentity(
	mechanism EvaluationMechanism,
) (typedmemory.C32EvaluationMechanismIdentity, error) {
	return typedmemory.NewC32EvaluationMechanismIdentity(
		mechanism.Artifact(),
		mechanism.Edition(),
		mechanism.Digest(),
	)
}
