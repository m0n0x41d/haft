package typedmemorystore

import (
	"bytes"
	"fmt"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

type transactionAdmissionSnapshot struct {
	revision                 typedmemory.GraphRevision
	typeEnv                  typedmemory.TypeEnvRef
	entities                 map[string]typedmemory.EntityResolution
	aliases                  map[string]typedmemory.AliasAvailability
	assertions               map[string]typedmemory.AssertionState
	references               map[string]typedmemory.StrongReferenceResolution
	membershipJudgements     map[string]typedmemory.MemberOfJudgement
	classificationJudgements map[string]typedmemory.KindClassificationJudgement
	disjointEntailments      map[string]typedmemory.DisjointEntailmentUse
}

func newTransactionAdmissionSnapshotWithClassifications(
	basis typedmemory.AdmissionBasis,
	observations []typedmemory.AdmissionSnapshotObservation,
	referenceResolutions []typedmemory.StrongReferenceResolution,
	membershipJudgements []typedmemory.MemberOfJudgement,
	classificationJudgements []typedmemory.KindClassificationJudgement,
	entailments []typedmemory.DisjointEntailmentUse,
) (transactionAdmissionSnapshot, error) {
	if basis == nil {
		return transactionAdmissionSnapshot{}, ErrInvalidAdmissionBatch
	}
	snapshot := transactionAdmissionSnapshot{
		revision:                 basis.GraphRevision(),
		typeEnv:                  basis.TypeEnv(),
		entities:                 make(map[string]typedmemory.EntityResolution),
		aliases:                  make(map[string]typedmemory.AliasAvailability),
		assertions:               make(map[string]typedmemory.AssertionState),
		references:               make(map[string]typedmemory.StrongReferenceResolution),
		membershipJudgements:     make(map[string]typedmemory.MemberOfJudgement),
		classificationJudgements: make(map[string]typedmemory.KindClassificationJudgement),
		disjointEntailments:      make(map[string]typedmemory.DisjointEntailmentUse),
	}
	if err := snapshot.addObservations(observations); err != nil {
		return transactionAdmissionSnapshot{}, err
	}
	if err := snapshot.addReferenceResolutions(referenceResolutions); err != nil {
		return transactionAdmissionSnapshot{}, err
	}
	if err := snapshot.addMembershipJudgements(membershipJudgements); err != nil {
		return transactionAdmissionSnapshot{}, err
	}
	if err := snapshot.addClassificationJudgements(classificationJudgements); err != nil {
		return transactionAdmissionSnapshot{}, err
	}
	if err := snapshot.addDisjointEntailments(entailments); err != nil {
		return transactionAdmissionSnapshot{}, err
	}
	return snapshot, nil
}

func (snapshot *transactionAdmissionSnapshot) addClassificationJudgements(
	judgements []typedmemory.KindClassificationJudgement,
) error {
	for _, judgement := range judgements {
		if !typedmemory.KindClassificationJudgementValid(judgement) ||
			judgement.Kind() == typedmemory.KindClassificationUnknown {
			return fmt.Errorf(
				"transaction admission snapshot requires a direct true or false classification",
			)
		}
		key := judgement.Request().Digest().String()
		previous, exists := snapshot.classificationJudgements[key]
		if !exists {
			snapshot.classificationJudgements[key] = judgement
			continue
		}
		if previous.Digest() == judgement.Digest() &&
			bytes.Equal(previous.CanonicalBytes(), judgement.CanonicalBytes()) {
			continue
		}
		return fmt.Errorf(
			"transaction admission snapshot contains conflicting kind classifications",
		)
	}
	return nil
}

func (snapshot *transactionAdmissionSnapshot) addObservations(
	observations []typedmemory.AdmissionSnapshotObservation,
) error {
	for _, observation := range observations {
		switch value := observation.(type) {
		case typedmemory.EntityAbsentObservation:
			resolution := value.Resolution()
			snapshot.entities[entityObservationKey(
				resolution.Entity(),
				resolution.Context(),
			)] = resolution
		case typedmemory.EntityExactObservation:
			resolution := value.Resolution()
			snapshot.entities[entityObservationKey(
				resolution.Entity(),
				resolution.Context(),
			)] = resolution
		case typedmemory.AliasUnboundObservation:
			resolution := value.Resolution()
			snapshot.aliases[aliasObservationKey(
				resolution.Alias(),
				resolution.Context(),
			)] = resolution
		case typedmemory.AliasBoundObservation:
			resolution := value.Resolution()
			snapshot.aliases[aliasObservationKey(
				resolution.Alias(),
				resolution.Context(),
			)] = resolution
		case typedmemory.AssertionAbsentObservation:
			state := value.State()
			snapshot.assertions[state.Assertion().String()] = state
		case typedmemory.AssertionActiveObservation:
			state := value.State()
			snapshot.assertions[state.Assertion().String()] = state
		default:
			return fmt.Errorf("unsupported transaction admission observation %T", observation)
		}
	}
	return nil
}

func (snapshot *transactionAdmissionSnapshot) addReferenceResolutions(
	resolutions []typedmemory.StrongReferenceResolution,
) error {
	for _, resolved := range resolutions {
		if resolved == nil {
			return fmt.Errorf("transaction admission snapshot contains an empty reference resolution")
		}
		queryReference := resolved.Reference()
		snapshot.references[referenceObservationKey(
			queryReference,
			resolved.Context(),
		)] = resolved
	}
	return nil
}

func (snapshot *transactionAdmissionSnapshot) addMembershipJudgements(
	judgements []typedmemory.MemberOfJudgement,
) error {
	for _, judgement := range judgements {
		var view typedmemory.MemberOfEvaluationView
		switch value := judgement.(type) {
		case typedmemory.MemberOfMember:
			view = value.EvaluationView()
		case typedmemory.MemberOfNotMember:
			view = value.EvaluationView()
		default:
			return fmt.Errorf("transaction admission snapshot requires defined MemberOf judgements")
		}
		request, err := typedmemory.NewMemberOfEvaluationRequest(judgement.Query(), view)
		if err != nil {
			return err
		}
		snapshot.membershipJudgements[request.Digest().String()] = judgement
	}
	return nil
}

func (snapshot *transactionAdmissionSnapshot) addDisjointEntailments(
	entailments []typedmemory.DisjointEntailmentUse,
) error {
	for _, entailment := range entailments {
		if entailment == nil {
			return fmt.Errorf("transaction admission snapshot contains an empty disjoint entailment")
		}
		request, err := typedmemory.NewMemberOfEvaluationRequest(
			entailment.CounterQuery(),
			entailment.EvaluationView(),
		)
		if err != nil {
			return fmt.Errorf("transaction admission snapshot contains an invalid disjoint entailment: %w", err)
		}
		key := disjointEntailmentObservationKey(
			request,
			entailment.Constraint(),
			entailment.SupportingMembership(),
		)
		previous, exists := snapshot.disjointEntailments[key]
		if !exists {
			snapshot.disjointEntailments[key] = entailment
			continue
		}
		if previous.Digest() == entailment.Digest() &&
			bytes.Equal(previous.CanonicalBytes(), entailment.CanonicalBytes()) {
			continue
		}
		return fmt.Errorf("transaction admission snapshot contains conflicting disjoint entailments")
	}
	return nil
}

func (snapshot transactionAdmissionSnapshot) GraphRevision() typedmemory.GraphRevision {
	return snapshot.revision
}

func (snapshot transactionAdmissionSnapshot) TypeEnvRef() typedmemory.TypeEnvRef {
	return snapshot.typeEnv
}

func (snapshot transactionAdmissionSnapshot) ResolveEntity(
	entity typedmemory.EntityID,
	contextRef typedmemory.BoundedContextRef,
) typedmemory.EntityResolution {
	return snapshot.entities[entityObservationKey(entity, contextRef)]
}

func (snapshot transactionAdmissionSnapshot) ResolveReference(
	reference typedmemory.StrongRef,
	contextRef typedmemory.BoundedContextRef,
) typedmemory.StrongReferenceResolution {
	return snapshot.references[referenceObservationKey(reference, contextRef)]
}

func (snapshot transactionAdmissionSnapshot) EvaluateMemberOf(
	request typedmemory.MemberOfEvaluationRequest,
) typedmemory.MemberOfJudgement {
	return snapshot.membershipJudgements[request.Digest().String()]
}

func (snapshot transactionAdmissionSnapshot) EvaluateKindClassification(
	request typedmemory.KindClassificationRequest,
) typedmemory.KindClassificationJudgement {
	return snapshot.classificationJudgements[request.Digest().String()]
}

func (snapshot transactionAdmissionSnapshot) ResolveDisjointEntailment(
	request typedmemory.MemberOfEvaluationRequest,
	constraint typedmemory.ConstraintID,
	supporting typedmemory.MemberOfMember,
) (typedmemory.DisjointEntailmentUse, bool) {
	key := disjointEntailmentObservationKey(request, constraint, supporting)
	use, exists := snapshot.disjointEntailments[key]
	if !exists || use == nil {
		return nil, false
	}
	actualSupport := use.SupportingMembership()
	if actualSupport.Digest() != supporting.Digest() ||
		!bytes.Equal(actualSupport.CanonicalBytes(), supporting.CanonicalBytes()) {
		return nil, false
	}
	return use, true
}

func (snapshot transactionAdmissionSnapshot) AssertionState(
	assertion typedmemory.AssertionID,
) typedmemory.AssertionState {
	return snapshot.assertions[assertion.String()]
}

func (snapshot transactionAdmissionSnapshot) ResolveAlias(
	alias typedmemory.EntityAlias,
	contextRef typedmemory.BoundedContextRef,
) typedmemory.AliasAvailability {
	return snapshot.aliases[aliasObservationKey(alias, contextRef)]
}

func (snapshot transactionAdmissionSnapshot) ResolveReconciliationBasis(
	basis typedmemory.ReconciliationBasisRef,
	contextRef typedmemory.BoundedContextRef,
) typedmemory.ReconciliationBasisResolution {
	resolution, err := typedmemory.NewMissingReconciliationBasis(basis, contextRef)
	if err != nil {
		return nil
	}
	return resolution
}

func entityObservationKey(
	entity typedmemory.EntityID,
	contextRef typedmemory.BoundedContextRef,
) string {
	return entity.String() + "\x00" + contextRef.String()
}

func aliasObservationKey(
	alias typedmemory.EntityAlias,
	contextRef typedmemory.BoundedContextRef,
) string {
	return alias.String() + "\x00" + contextRef.String()
}

func referenceObservationKey(
	reference typedmemory.StrongRef,
	contextRef typedmemory.BoundedContextRef,
) string {
	return reference.RefKind().String() + "\x00" + reference.ReferenceKey() + "\x00" + contextRef.String()
}

func disjointEntailmentObservationKey(
	request typedmemory.MemberOfEvaluationRequest,
	constraint typedmemory.ConstraintID,
	supporting typedmemory.MemberOfMember,
) string {
	return constraint.String() + "\x00" +
		request.Digest().String() + "\x00" +
		supporting.Digest().String()
}

var _ typedmemory.MemorySnapshot = transactionAdmissionSnapshot{}
var _ typedmemory.KindClassificationSnapshot = transactionAdmissionSnapshot{}
var _ typedmemory.DisjointEntailmentSnapshot = transactionAdmissionSnapshot{}
