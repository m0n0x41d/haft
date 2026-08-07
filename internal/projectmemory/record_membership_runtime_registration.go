package projectmemory

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/memberofruntime"
	"github.com/m0n0x41d/haft/internal/projectmemory/recordcarrier"
	"github.com/m0n0x41d/haft/internal/projecttypeenvruntime"
	"github.com/m0n0x41d/haft/internal/recordmembershipregistration"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemoryevaluation"
)

// RecordMembershipAdmissionEngineBuilder composes the one implemented
// project-record MemberOf family from installed immutable components. The
// result is not yet selected by X; ObserveCurrentTargetRuntime separately
// filters every callable registry to the exact target pins.
type RecordMembershipAdmissionEngineBuilder struct {
	entitySetEnumeration projecttypeenvruntime.EntitySetEnumerationEvaluatorRegistry
	candidateVisibility  projecttypeenvruntime.CandidateVisibilityEvaluatorRegistry
	kindDefinedness      projecttypeenvruntime.KindDefinednessEvaluatorRegistry
	recordCarrier        typedmemoryevaluation.Registry[
		recordcarrier.RecordMembershipEvaluationRequestV1,
		typedmemory.MemberOfJudgement,
	]
	policy recordmembershipregistration.RegistrationArtifactV1
}

func NewRecordMembershipAdmissionEngineBuilder() RecordMembershipAdmissionEngineBuilder {
	return RecordMembershipAdmissionEngineBuilder{}
}

func (builder RecordMembershipAdmissionEngineBuilder) SetEntitySetEnumeration(
	registry projecttypeenvruntime.EntitySetEnumerationEvaluatorRegistry,
) RecordMembershipAdmissionEngineBuilder {
	builder.entitySetEnumeration = registry.Clone()
	return builder
}

func (builder RecordMembershipAdmissionEngineBuilder) SetCandidateVisibility(
	registry projecttypeenvruntime.CandidateVisibilityEvaluatorRegistry,
) RecordMembershipAdmissionEngineBuilder {
	builder.candidateVisibility = registry.Clone()
	return builder
}

func (builder RecordMembershipAdmissionEngineBuilder) SetKindDefinedness(
	registry projecttypeenvruntime.KindDefinednessEvaluatorRegistry,
) RecordMembershipAdmissionEngineBuilder {
	builder.kindDefinedness = registry.Clone()
	return builder
}

func (builder RecordMembershipAdmissionEngineBuilder) SetRecordCarrierMembership(
	registry typedmemoryevaluation.Registry[
		recordcarrier.RecordMembershipEvaluationRequestV1,
		typedmemory.MemberOfJudgement,
	],
) RecordMembershipAdmissionEngineBuilder {
	builder.recordCarrier = registry.Clone()
	return builder
}

func (builder RecordMembershipAdmissionEngineBuilder) SetRegistrationPolicy(
	policy recordmembershipregistration.RegistrationArtifactV1,
) RecordMembershipAdmissionEngineBuilder {
	builder.policy = policy
	return builder
}

func (builder RecordMembershipAdmissionEngineBuilder) Build() (
	RecordMembershipAdmissionEngine,
	error,
) {
	engine := RecordMembershipAdmissionEngine{
		entitySetEnumeration: builder.entitySetEnumeration.Clone(),
		candidateVisibility:  builder.candidateVisibility.Clone(),
		kindDefinedness:      builder.kindDefinedness.Clone(),
		recordMembership:     builder.recordCarrier.Clone(),
		policy:               builder.policy,
	}
	if err := engine.validate(); err != nil {
		return RecordMembershipAdmissionEngine{}, fmt.Errorf(
			"build record MemberOf admission engine: %w",
			err,
		)
	}
	return engine, nil
}

// NewRecordMembershipEvaluatorRegistry exposes the reviewed record-carrier
// family through the generic store-facing MemberOf boundary. The exact rule
// and mechanism identity come from the verified registration policy; callers
// cannot relabel this engine as another MemberOf family.
func NewRecordMembershipEvaluatorRegistry(
	engine RecordMembershipAdmissionEngine,
) (memberofruntime.Registry, error) {
	if err := engine.validate(); err != nil {
		return memberofruntime.Registry{}, err
	}
	coordinate := engine.policy.Evaluator()
	identity, err := recordMembershipMechanismIdentity(coordinate)
	if err != nil {
		return memberofruntime.Registry{}, err
	}
	registration, err := memberofruntime.NewRegistration(
		coordinate.Rule(),
		identity,
		engine,
	)
	if err != nil {
		return memberofruntime.Registry{}, err
	}
	return memberofruntime.NewRegistry(
		[]memberofruntime.Registration{registration},
	)
}
