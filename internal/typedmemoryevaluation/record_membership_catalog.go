package typedmemoryevaluation

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/projectmemory/recordcarrier"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// NewRecordMembershipRegistry is the production registration factory for the
// record-carrier MemberOf contract. The sibling kind-runtime factories own the
// preceding EntitySetEnumeration, CandidateVisibility, and KindDefinedness
// contracts. Each binds a reviewed typed evaluator rather than accepting an
// arbitrary caller closure.
//
// The supplied identity is the independently pinned deployed-artifact
// coordinate later compared with X. This factory neither constructs nor
// verifies X and grants no TypeEnv, admission, storage, or authority effect.
// Its role is X evaluator. The distinct X carrier-membership pin governs the
// trusted source-delivery/adapter boundary and is deliberately absent here.
func NewRecordMembershipRegistry(
	identity MechanismIdentity,
) (
	Registry[
		recordcarrier.RecordMembershipEvaluationRequestV1,
		typedmemory.MemberOfJudgement,
	],
	error,
) {
	if !identity.valid() {
		return Registry[
			recordcarrier.RecordMembershipEvaluationRequestV1,
			typedmemory.MemberOfJudgement,
		]{}, fmt.Errorf("record membership evaluator mechanism identity is invalid")
	}
	if identity.Role() != EvaluatorRole {
		return Registry[
			recordcarrier.RecordMembershipEvaluationRequestV1,
			typedmemory.MemberOfJudgement,
		]{}, fmt.Errorf("record membership evaluator requires evaluator mechanism role")
	}
	evaluator := recordcarrier.NewRecordMembershipEvaluatorV1()
	mechanism, err := newPureEvaluator(evaluator.Evaluate)
	if err != nil {
		return Registry[
			recordcarrier.RecordMembershipEvaluationRequestV1,
			typedmemory.MemberOfJudgement,
		]{}, err
	}
	registration, err := newRegistration(
		evaluator.RuleRef(),
		identity,
		mechanism,
	)
	if err != nil {
		return Registry[
			recordcarrier.RecordMembershipEvaluationRequestV1,
			typedmemory.MemberOfJudgement,
		]{}, err
	}
	registrations := []Registration[
		recordcarrier.RecordMembershipEvaluationRequestV1,
		typedmemory.MemberOfJudgement,
	]{registration}
	return NewRegistry(registrations)
}
