package typedmemorystore

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

type revalidatedAdmission struct {
	prepared              preparedAdmission
	observableBlobs       []ObservableInputBlob
	classificationSources []KindClassificationSourceBlob
}

func (adapter *SQLiteAdapter) revalidateGenericAdmission(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	request CommitRequest,
	prepared preparedAdmission,
	environment typedmemory.TypeEnv,
	registry typedmemory.CodecRegistry,
	memberOfEngine MemberOfEvaluationEngine,
	classificationEngine KindClassificationAdmissionEngine,
) (revalidatedAdmission, error) {
	observations, err := rebuildAdmissionObservations(
		ctx,
		transaction,
		request.project,
		request.expectedRevision,
		prepared.basis.SnapshotObservations(),
	)
	if err != nil {
		return revalidatedAdmission{}, err
	}

	resolutions, judgements, entailments, observables, err := adapter.recomputeAdmissionUses(
		ctx,
		transaction,
		request.project,
		request,
		prepared.basis,
		environment,
		memberOfEngine,
	)
	if err != nil {
		return revalidatedAdmission{}, err
	}
	classificationResolutions, classifications, classificationSources, err := adapter.recomputeClassificationAdmissionUses(
		ctx,
		transaction,
		request.project,
		request,
		prepared.basis,
		environment,
		registry,
		classificationEngine,
	)
	if err != nil {
		return revalidatedAdmission{}, err
	}
	resolutions = append(resolutions, classificationResolutions...)

	snapshot, err := newTransactionAdmissionSnapshotWithClassifications(
		prepared.basis,
		observations,
		resolutions,
		judgements,
		classifications,
		entailments,
	)
	if err != nil {
		return revalidatedAdmission{}, err
	}
	verdict := typedmemory.ValidateMemoryChangeSet(
		environment,
		registry,
		snapshot,
		request.candidate,
	)
	valid, admitted := verdict.(typedmemory.Valid)
	if !admitted {
		return revalidatedAdmission{}, ErrRevalidationRejected
	}
	freshRequest := request
	freshRequest.admissionBatch = valid.AdmissionBatch()
	fresh, err := prepareGenericAdmission(freshRequest)
	if err != nil {
		return revalidatedAdmission{}, err
	}
	if !samePreparedAdmissionEnvelope(prepared, fresh) {
		return revalidatedAdmission{}, fmt.Errorf(
			"%w: fresh semantic validation differs from the prepared admission envelope",
			ErrAdmissionEnvelopeMismatch,
		)
	}
	return revalidatedAdmission{
		prepared:              fresh,
		observableBlobs:       observables,
		classificationSources: classificationSources,
	}, nil
}

func (adapter *SQLiteAdapter) recomputeClassificationAdmissionUses(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectledger.ProjectID,
	request CommitRequest,
	basis typedmemory.AdmissionBasis,
	environment typedmemory.TypeEnv,
	registry typedmemory.CodecRegistry,
	engine KindClassificationAdmissionEngine,
) (
	[]typedmemory.StrongReferenceResolution,
	[]typedmemory.KindClassificationJudgement,
	[]KindClassificationSourceBlob,
	error,
) {
	classification, required := basis.(typedmemory.ContextSliceClassificationBasis)
	if !required {
		return nil, nil, nil, nil
	}
	if !kindClassificationAdmissionEngineIsPresent(engine) ||
		adapter.referenceEngine == nil {
		return nil, nil, nil, ErrStorageGenerationUnavailable
	}
	resolutions := make([]typedmemory.StrongReferenceResolution, 0)
	judgements := make([]typedmemory.KindClassificationJudgement, 0)
	sources := make([]KindClassificationSourceBlob, 0)
	for _, use := range classification.ClassificationReferenceFillerAdmissionUses() {
		resolution, err := adapter.recomputeAdmissionResolution(
			ctx,
			transaction,
			project,
			request,
			use.Coordinate(),
			use.Resolution(),
			environment,
		)
		if err != nil {
			return nil, nil, nil, err
		}
		if resolution != nil {
			resolutions = append(resolutions, resolution)
		}
		requiredJudgement, requiredSources, err := adapter.recomputeKindClassificationJudgement(
			ctx,
			transaction,
			project,
			request,
			environment,
			registry,
			use.Coordinate().ChangeOrdinal(),
			use.Resolution(),
			use.RequiredClassification(),
			engine,
		)
		if err != nil {
			return nil, nil, nil, err
		}
		judgements = append(judgements, requiredJudgement)
		sources = append(sources, requiredSources...)
		for _, disjoint := range use.DisjointClassifications() {
			counter, counterSources, counterErr := adapter.recomputeKindClassificationJudgement(
				ctx,
				transaction,
				project,
				request,
				environment,
				registry,
				use.Coordinate().ChangeOrdinal(),
				use.Resolution(),
				disjoint.Judgement(),
				engine,
			)
			if counterErr != nil {
				return nil, nil, nil, counterErr
			}
			judgements = append(judgements, counter)
			sources = append(sources, counterSources...)
		}
	}
	normalizedSources, err := coalesceKindClassificationSourceBlobs(sources)
	if err != nil {
		return nil, nil, nil, err
	}
	return resolutions, judgements, normalizedSources, nil
}

func (adapter *SQLiteAdapter) recomputeKindClassificationJudgement(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectledger.ProjectID,
	request CommitRequest,
	environment typedmemory.TypeEnv,
	registry typedmemory.CodecRegistry,
	evaluationChangeOrdinal uint64,
	resolution typedmemory.AdmissionReferenceResolution,
	expected typedmemory.KindClassificationJudgement,
	engine KindClassificationAdmissionEngine,
) (
	typedmemory.KindClassificationJudgement,
	[]KindClassificationSourceBlob,
	error,
) {
	if !typedmemory.KindClassificationJudgementValid(expected) ||
		expected.Kind() == typedmemory.KindClassificationUnknown {
		return nil, nil, ErrInvalidAdmissionBatch
	}
	visibility, err := classificationVisibilityForResolution(
		request.candidate,
		request.expectedRevision,
		evaluationChangeOrdinal,
		resolution,
	)
	if err != nil {
		return nil, nil, err
	}
	sources, err := adapter.loadKindClassificationSources(
		ctx,
		transaction,
		project,
		request.expectedRevision,
		expected,
	)
	if err != nil {
		return nil, nil, err
	}
	input, err := NewKindClassificationAdmissionInput(
		project,
		environment,
		registry,
		expected.Request(),
		visibility,
		sources,
	)
	if err != nil {
		return nil, nil, err
	}
	recomputed, err := engine.EvaluateKindClassification(ctx, input)
	if err != nil {
		return nil, nil, fmt.Errorf("evaluate exact kind classification: %w", err)
	}
	if !sameKindClassificationJudgement(expected, recomputed) {
		return nil, nil, fmt.Errorf(
			"%w: kind-classification judgement %s changed during commit revalidation (expected %s/%s, recomputed %s/%s)",
			ErrAdmissionEnvelopeMismatch,
			expected.Request().Digest().String(),
			expected.Kind().String(),
			expected.Digest().String(),
			recomputed.Kind().String(),
			recomputed.Digest().String(),
		)
	}
	return recomputed, sources, nil
}

func (adapter *SQLiteAdapter) loadKindClassificationSources(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectledger.ProjectID,
	revision typedmemory.GraphRevision,
	judgement typedmemory.KindClassificationJudgement,
) ([]KindClassificationSourceBlob, error) {
	if transaction == nil {
		return nil, sqlitetransaction.ErrTransactionInvalid
	}
	if err := transaction.RequireActive(); err != nil {
		return nil, err
	}
	coordinates, err := kindClassificationSourceCoordinates(judgement)
	if err != nil {
		return nil, err
	}
	result := make([]KindClassificationSourceBlob, 0, len(coordinates))
	for _, coordinate := range coordinates {
		persisted, found, loadErr := loadPersistedKindClassificationSourceTx(
			ctx,
			transaction,
			project,
			revision,
			coordinate.reference,
			coordinate.digest,
		)
		if loadErr != nil {
			return nil, loadErr
		}
		if found {
			result = append(result, persisted)
			continue
		}
		if !kindClassificationSourceProviderIsPresent(adapter.kindClassificationSources) {
			return nil, ErrStorageGenerationUnavailable
		}
		blob, loadErr := adapter.kindClassificationSources.LoadKindClassificationSource(
			ctx,
			project,
			coordinate.reference,
			coordinate.digest,
		)
		if loadErr != nil {
			return nil, fmt.Errorf(
				"load exact kind-classification source %q: %w",
				coordinate.reference.String(),
				loadErr,
			)
		}
		verified, verifyErr := NewKindClassificationSourceBlob(
			blob.Reference(),
			blob.Digest(),
			blob.Bytes(),
		)
		if verifyErr != nil ||
			verified.Reference() != coordinate.reference ||
			verified.Digest() != coordinate.digest {
			return nil, ErrAdmissionEnvelopeMismatch
		}
		result = append(result, verified)
	}
	return normalizeKindClassificationSourceBlobs(result)
}

type kindClassificationSourceCoordinate struct {
	reference typedmemory.CarrierRef
	digest    typedmemory.SHA256Digest
}

func kindClassificationSourceCoordinates(
	judgement typedmemory.KindClassificationJudgement,
) ([]kindClassificationSourceCoordinate, error) {
	var features []typedmemory.GovernedCandidateFeature
	switch value := judgement.(type) {
	case typedmemory.TrueKindClassification:
		features = value.Basis().FeatureSet().Features()
	case typedmemory.FalseKindClassification:
		features = value.Basis().FeatureSet().Features()
	default:
		return nil, ErrInvalidAdmissionBatch
	}
	byReference := make(map[string]kindClassificationSourceCoordinate)
	for _, feature := range features {
		if strings.HasPrefix(
			feature.Source().String(),
			"kind-classification-visibility:",
		) {
			continue
		}
		coordinate := kindClassificationSourceCoordinate{
			reference: feature.Source(),
			digest:    feature.SourceDigest(),
		}
		key := coordinate.reference.String()
		observed, found := byReference[key]
		if found && observed.digest != coordinate.digest {
			return nil, ErrInvalidAdmissionBatch
		}
		byReference[key] = coordinate
	}
	keys := make([]string, 0, len(byReference))
	for key := range byReference {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]kindClassificationSourceCoordinate, 0, len(keys))
	for _, key := range keys {
		result = append(result, byReference[key])
	}
	return result, nil
}

func classificationVisibilityForResolution(
	candidate typedmemory.MemoryChangeSet,
	revision typedmemory.GraphRevision,
	evaluationChangeOrdinal uint64,
	resolution typedmemory.AdmissionReferenceResolution,
) (KindClassificationVisibility, error) {
	switch value := resolution.(type) {
	case typedmemory.SnapshotReferenceResolution:
		return NewSnapshotKindClassificationVisibility(
			revision,
			value.Entity(),
			value.Context(),
			value.ResolutionBasis(),
		)
	case typedmemory.SameBatchDeclarationResolution:
		return newProspectiveKindClassificationVisibility(
			candidate,
			revision,
			evaluationChangeOrdinal,
			value,
		)
	default:
		return nil, ErrInvalidAdmissionBatch
	}
}

func sameKindClassificationJudgement(
	expected typedmemory.KindClassificationJudgement,
	actual typedmemory.KindClassificationJudgement,
) bool {
	return typedmemory.KindClassificationJudgementValid(expected) &&
		typedmemory.KindClassificationJudgementValid(actual) &&
		expected.Kind() == actual.Kind() &&
		expected.Request().Digest() == actual.Request().Digest() &&
		bytes.Equal(
			expected.Request().CanonicalBytes(),
			actual.Request().CanonicalBytes(),
		) &&
		expected.Digest() == actual.Digest() &&
		bytes.Equal(expected.CanonicalBytes(), actual.CanonicalBytes())
}

func (adapter *SQLiteAdapter) recomputeAdmissionUses(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectledger.ProjectID,
	request CommitRequest,
	basis typedmemory.AdmissionBasis,
	environment typedmemory.TypeEnv,
	memberOfEngine MemberOfEvaluationEngine,
) ([]typedmemory.StrongReferenceResolution, []typedmemory.MemberOfJudgement, []typedmemory.DisjointEntailmentUse, []ObservableInputBlob, error) {
	membership, requiresMembership := basis.(typedmemory.ContextSliceMembershipBasis)
	if !requiresMembership {
		return nil, nil, nil, nil, nil
	}
	if !memberOfEvaluationEngineIsPresent(memberOfEngine) ||
		adapter.referenceEngine == nil ||
		adapter.observableInputs == nil {
		return nil, nil, nil, nil, ErrStorageGenerationUnavailable
	}

	resolutions := make([]typedmemory.StrongReferenceResolution, 0)
	judgements := make([]typedmemory.MemberOfJudgement, 0)
	entailments := make([]typedmemory.DisjointEntailmentUse, 0)
	observableByIdentity := make(map[string]ObservableInputBlob)
	observableDigestByRef := make(map[string]typedmemory.SHA256Digest)

	for _, use := range membership.ReferenceFillerAdmissionUses() {
		resolution, err := adapter.recomputeAdmissionResolution(
			ctx,
			transaction,
			project,
			request,
			use.Coordinate(),
			use.Resolution(),
			environment,
		)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		if resolution != nil {
			resolutions = append(resolutions, resolution)
		}

		required, blobs, err := adapter.recomputeMemberOfJudgement(
			ctx,
			transaction,
			project,
			request,
			use.Resolution(),
			use.Coordinate().ChangeOrdinal(),
			use.RequiredMembership(),
			environment,
			memberOfEngine,
		)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		recomputedRequired, ok := required.(typedmemory.MemberOfMember)
		if !ok {
			return nil, nil, nil, nil, ErrAdmissionEnvelopeMismatch
		}
		judgements = append(judgements, recomputedRequired)
		if err := mergeObservableBlobs(observableByIdentity, observableDigestByRef, blobs); err != nil {
			return nil, nil, nil, nil, err
		}

		for _, disjoint := range use.DisjointMemberships() {
			switch expected := disjoint.(type) {
			case typedmemory.DirectNotMemberUse:
				judgement, counterBlobs, err := adapter.recomputeMemberOfJudgement(
					ctx,
					transaction,
					project,
					request,
					use.Resolution(),
					use.Coordinate().ChangeOrdinal(),
					expected.Judgement(),
					environment,
					memberOfEngine,
				)
				if err != nil {
					return nil, nil, nil, nil, err
				}
				judgements = append(judgements, judgement)
				if err := mergeObservableBlobs(observableByIdentity, observableDigestByRef, counterBlobs); err != nil {
					return nil, nil, nil, nil, err
				}
			case typedmemory.DisjointEntailmentUse:
				recomputed, err := rebuildDisjointEntailment(
					environment,
					recomputedRequired,
					expected,
				)
				if err != nil {
					return nil, nil, nil, nil, err
				}
				entailments = append(entailments, recomputed)
			default:
				return nil, nil, nil, nil, ErrInvalidAdmissionBatch
			}
		}
	}

	observables := make([]ObservableInputBlob, 0, len(observableByIdentity))
	for _, blob := range observableByIdentity {
		observables = append(observables, blob)
	}
	return resolutions, judgements, entailments, observables, nil
}

func rebuildDisjointEntailment(
	environment typedmemory.TypeEnv,
	recomputedRequired typedmemory.MemberOfMember,
	expected typedmemory.DisjointEntailmentUse,
) (typedmemory.DisjointEntailmentUse, error) {
	constraint, err := exactCurrentDisjointConstraint(environment, expected.Constraint())
	if err != nil {
		return nil, err
	}
	recomputed, err := typedmemory.NewDisjointEntailmentUse(typedmemory.DisjointEntailmentUseInput{
		TypeEnv:              environment,
		Constraint:           constraint,
		SupportingMembership: recomputedRequired,
		MatchedOperand:       expected.MatchedOperand(),
		ExcludedOperand:      expected.ExcludedOperand(),
	})
	if err != nil {
		return nil, ErrAdmissionEnvelopeMismatch
	}
	if !sameDisjointEntailmentUse(expected, recomputed) {
		return nil, ErrAdmissionEnvelopeMismatch
	}
	return recomputed, nil
}

func exactCurrentDisjointConstraint(
	environment typedmemory.TypeEnv,
	constraintID typedmemory.ConstraintID,
) (typedmemory.KindDisjointConstraint, error) {
	matches := make([]typedmemory.ConstraintRule, 0, 1)
	for _, rule := range environment.Constraints() {
		if rule.ID() == constraintID {
			matches = append(matches, rule)
		}
	}
	if len(matches) != 1 {
		return typedmemory.KindDisjointConstraint{}, ErrAdmissionEnvelopeMismatch
	}
	constraint, ok := matches[0].(typedmemory.KindDisjointConstraint)
	if !ok {
		return typedmemory.KindDisjointConstraint{}, ErrAdmissionEnvelopeMismatch
	}
	return constraint, nil
}

func sameDisjointEntailmentUse(
	expected typedmemory.DisjointEntailmentUse,
	actual typedmemory.DisjointEntailmentUse,
) bool {
	return expected != nil &&
		actual != nil &&
		expected.Constraint() == actual.Constraint() &&
		expected.ConstraintDigest() == actual.ConstraintDigest() &&
		bytes.Equal(expected.ConstraintRule().CanonicalBytes(), actual.ConstraintRule().CanonicalBytes()) &&
		expected.SupportingMembership().Digest() == actual.SupportingMembership().Digest() &&
		bytes.Equal(expected.SupportingMembership().CanonicalBytes(), actual.SupportingMembership().CanonicalBytes()) &&
		expected.CounterQuery().Digest() == actual.CounterQuery().Digest() &&
		bytes.Equal(expected.CounterQuery().CanonicalBytes(), actual.CounterQuery().CanonicalBytes()) &&
		expected.EvaluationView().Digest() == actual.EvaluationView().Digest() &&
		bytes.Equal(expected.EvaluationView().CanonicalBytes(), actual.EvaluationView().CanonicalBytes()) &&
		expected.MatchedOperand() == actual.MatchedOperand() &&
		expected.ExcludedOperand() == actual.ExcludedOperand() &&
		expected.Digest() == actual.Digest() &&
		bytes.Equal(expected.CanonicalBytes(), actual.CanonicalBytes())
}

func (adapter *SQLiteAdapter) recomputeAdmissionResolution(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectledger.ProjectID,
	request CommitRequest,
	coordinate typedmemory.RelationFillerCoordinate,
	expected typedmemory.AdmissionReferenceResolution,
	environment typedmemory.TypeEnv,
) (typedmemory.StrongReferenceResolution, error) {
	switch value := expected.(type) {
	case typedmemory.SameBatchDeclarationResolution:
		_, err := rebuildProspectiveView(
			request.candidate,
			request.expectedTypeEnv,
			request.expectedRevision,
			coordinate.ChangeOrdinal(),
			value,
		)
		return nil, err
	case typedmemory.SnapshotReferenceResolution:
		universe, err := loadExactPersistedEntityUniverseTx(
			ctx,
			transaction,
			project,
			value.Context(),
			request.expectedRevision,
		)
		if err != nil {
			return nil, err
		}
		input := newStrongReferenceResolutionInput(
			project,
			environment,
			request.expectedRevision,
			value.PersistedReference(),
			value.Context(),
			universe,
		)
		resolved, err := adapter.referenceEngine.ResolveStrongReference(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("resolve exact persisted reference: %w", err)
		}
		exact, ok := resolved.(typedmemory.ResolvedStrongReference)
		if !ok {
			return nil, ErrRevalidationRejected
		}
		recomputed, err := typedmemory.NewSnapshotReferenceResolution(exact)
		if err != nil {
			return nil, err
		}
		if !sameCanonicalAdmissionResolution(value, recomputed) {
			return nil, ErrAdmissionEnvelopeMismatch
		}
		return exact, nil
	default:
		return nil, ErrInvalidAdmissionBatch
	}
}

func (adapter *SQLiteAdapter) recomputeMemberOfJudgement(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectledger.ProjectID,
	request CommitRequest,
	resolution typedmemory.AdmissionReferenceResolution,
	evaluationChangeOrdinal uint64,
	expected typedmemory.MemberOfJudgement,
	environment typedmemory.TypeEnv,
	memberOfEngine MemberOfEvaluationEngine,
) (typedmemory.MemberOfJudgement, []ObservableInputBlob, error) {
	defined, ok := expected.(typedmemory.DefinedMemberOfJudgement)
	if !ok {
		return nil, nil, ErrInvalidAdmissionBatch
	}
	view, err := rebuildEvaluationView(
		request.candidate,
		request.expectedTypeEnv,
		request.expectedRevision,
		evaluationChangeOrdinal,
		resolution,
	)
	if err != nil {
		return nil, nil, err
	}
	evaluationRequest, err := typedmemory.NewMemberOfEvaluationRequest(expected.Query(), view)
	if err != nil {
		return nil, nil, err
	}
	universe, err := loadExactPersistedEntityUniverseTx(
		ctx,
		transaction,
		project,
		expected.Query().ContextSlice().Context(),
		request.expectedRevision,
	)
	if err != nil {
		return nil, nil, err
	}
	blobs, err := adapter.loadObservableInputs(
		ctx,
		transaction,
		project,
		request.expectedRevision,
		defined.Basis().ObservableInputs(),
		universe,
	)
	if err != nil {
		return nil, nil, err
	}
	input, err := newMemberOfEvaluationInput(
		project,
		environment,
		evaluationRequest,
		blobs,
		universe,
	)
	if err != nil {
		return nil, nil, err
	}
	recomputed, err := memberOfEngine.EvaluateMemberOf(ctx, input)
	if err != nil {
		return nil, nil, fmt.Errorf("evaluate exact MemberOf request: %w", err)
	}
	if !sameDefinedMemberOfJudgement(expected, recomputed) {
		return nil, nil, ErrAdmissionEnvelopeMismatch
	}
	return recomputed, blobs, nil
}

func (adapter *SQLiteAdapter) loadObservableInputs(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectledger.ProjectID,
	revision typedmemory.GraphRevision,
	inputs []typedmemory.MemberOfObservableInput,
	universe ExactPersistedEntityUniverse,
) ([]ObservableInputBlob, error) {
	blobs := make([]ObservableInputBlob, 0, len(inputs))
	universeInput, err := universe.ObservableInput()
	if err != nil {
		return nil, err
	}
	for _, input := range inputs {
		if input.Reference() == universeInput.Reference() {
			if input.Digest() != universeInput.Digest() {
				return nil, newObservableInputBlobUnavailableError(
					input.Reference(),
					input.Digest(),
				)
			}
			blob, err := universe.ObservableBlob()
			if err != nil {
				return nil, err
			}
			blobs = append(blobs, blob)
			continue
		}
		persisted, found, err := loadPersistedObservableInputTx(
			ctx,
			transaction,
			project,
			revision,
			input,
		)
		if err != nil {
			return nil, err
		}
		if found {
			blobs = append(blobs, persisted)
			continue
		}
		blob, err := adapter.observableInputs.LoadObservableInput(
			ctx,
			project,
			input.Reference(),
			input.Digest(),
		)
		if err != nil {
			return nil, newObservableInputBlobUnavailableError(
				input.Reference(),
				input.Digest(),
			)
		}
		verified, err := NewObservableInputBlob(
			blob.Reference(),
			blob.Digest(),
			blob.Bytes(),
		)
		if err != nil || verified.Reference() != input.Reference() || verified.Digest() != input.Digest() {
			return nil, newObservableInputBlobUnavailableError(
				input.Reference(),
				input.Digest(),
			)
		}
		blobs = append(blobs, verified)
	}
	return blobs, nil
}

func rebuildEvaluationView(
	candidate typedmemory.MemoryChangeSet,
	typeEnv typedmemory.TypeEnvRef,
	revision typedmemory.GraphRevision,
	evaluationChangeOrdinal uint64,
	resolution typedmemory.AdmissionReferenceResolution,
) (typedmemory.MemberOfEvaluationView, error) {
	switch value := resolution.(type) {
	case typedmemory.SnapshotReferenceResolution:
		return typedmemory.NewPersistedSnapshotView(typeEnv, revision)
	case typedmemory.SameBatchDeclarationResolution:
		return rebuildProspectiveView(
			candidate,
			typeEnv,
			revision,
			evaluationChangeOrdinal,
			value,
		)
	default:
		return nil, ErrInvalidAdmissionBatch
	}
}

func rebuildProspectiveView(
	candidate typedmemory.MemoryChangeSet,
	typeEnv typedmemory.TypeEnvRef,
	revision typedmemory.GraphRevision,
	evaluationChangeOrdinal uint64,
	resolution typedmemory.SameBatchDeclarationResolution,
) (typedmemory.ProspectiveBatchView, error) {
	prefix, err := typedmemory.ComputeOrderedCandidatePrefix(
		candidate,
		evaluationChangeOrdinal,
	)
	if err != nil {
		return typedmemory.ProspectiveBatchView{}, err
	}
	return typedmemory.NewProspectiveBatchView(typedmemory.ProspectiveBatchViewInput{
		TypeEnv:                  typeEnv,
		PreStateGraphRevision:    revision,
		EvaluationChangeOrdinal:  evaluationChangeOrdinal,
		DeclarationChangeOrdinal: resolution.DeclarationChangeOrdinal(),
		Declaration:              resolution.Declaration(),
		LocalReference:           resolution.LocalReference(),
		PersistedReference:       resolution.PersistedReference(),
		OrderedCandidatePrefix:   prefix,
	})
}

func mergeObservableBlobs(
	byIdentity map[string]ObservableInputBlob,
	digestByRef map[string]typedmemory.SHA256Digest,
	blobs []ObservableInputBlob,
) error {
	for _, blob := range blobs {
		ref := blob.Reference().String()
		if digest, exists := digestByRef[ref]; exists && digest != blob.Digest() {
			return fmt.Errorf("observable input reference %q has conflicting exact digests", ref)
		}
		digestByRef[ref] = blob.Digest()
		key := ref + "\x00" + blob.Digest().String()
		byIdentity[key] = blob
	}
	return nil
}

func samePreparedAdmissionEnvelope(left preparedAdmission, right preparedAdmission) bool {
	return left.requestDigest == right.requestDigest &&
		bytes.Equal(left.requestBytes, right.requestBytes) &&
		left.semanticDigest == right.semanticDigest &&
		bytes.Equal(left.semanticBytes, right.semanticBytes) &&
		left.envelopeDigest == right.envelopeDigest &&
		bytes.Equal(left.envelopeBytes, right.envelopeBytes) &&
		left.basis.Kind() == right.basis.Kind() &&
		left.basis.Digest() == right.basis.Digest() &&
		bytes.Equal(left.basis.CanonicalBytes(), right.basis.CanonicalBytes())
}

func sameCanonicalAdmissionResolution(
	left typedmemory.AdmissionReferenceResolution,
	right typedmemory.AdmissionReferenceResolution,
) bool {
	return left.Kind() == right.Kind() &&
		left.Digest() == right.Digest() &&
		bytes.Equal(left.CanonicalBytes(), right.CanonicalBytes())
}

func sameDefinedMemberOfJudgement(
	expected typedmemory.MemberOfJudgement,
	actual typedmemory.MemberOfJudgement,
) bool {
	return actual != nil &&
		expected.Kind() == actual.Kind() &&
		expected.Digest() == actual.Digest() &&
		bytes.Equal(expected.CanonicalBytes(), actual.CanonicalBytes())
}
