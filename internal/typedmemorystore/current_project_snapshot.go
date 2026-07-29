package typedmemorystore

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/projectmemory/identityreconciliation"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	memberOfSnapshotRepair  = "typed-memory-memberof-evaluator-unavailable"
	referenceSnapshotRepair = "typed-memory-reference-resolution-rule-unavailable"
)

// CurrentProjectSnapshotLoader owns the effectful read needed to correlate one
// graph head, its active TypeEnv, and an immutable in-memory fact view. Callers
// receive no database handle and cannot mutate or activate project state.
type CurrentProjectSnapshotLoader interface {
	LoadCurrentProjectSnapshot(
		context.Context,
		projectledger.ProjectID,
	) (CurrentProjectSnapshot, error)
}

// NewSQLiteCurrentProjectSnapshotLoader exposes only the coherent read port.
// Its private implementation cannot be type-asserted to SQLiteAdapter and has
// no clock, admission engine, transaction finisher, or write capability.
func NewSQLiteCurrentProjectSnapshotLoader(
	database *sql.DB,
	loader TypeEnvLoader,
) (CurrentProjectSnapshotLoader, error) {
	if database == nil {
		return nil, ErrDatabaseRequired
	}
	if !typeEnvLoaderIsPresent(loader) {
		return nil, ErrTypeEnvLoaderRequired
	}
	return &sqliteCurrentProjectSnapshotLoader{
		database: database,
		loader:   loader,
	}, nil
}

// NewProjectAwareSQLiteCurrentProjectSnapshotLoader composes the same
// read-only surface with exact selected-C reconstruction. The returned value
// still exposes no write, Stage, head-selection, or raw transaction port.
func NewProjectAwareSQLiteCurrentProjectSnapshotLoader(
	database *sql.DB,
	loader TypeEnvLoader,
	selectedProjectRuntime SelectedProjectTypeEnvRuntimeResolver,
) (CurrentProjectSnapshotLoader, error) {
	if !selectedProjectTypeEnvRuntimeResolverIsPresent(
		selectedProjectRuntime,
	) {
		return nil, ErrSelectedProjectTypeEnvRuntimeResolverRequired
	}
	basic, err := NewSQLiteCurrentProjectSnapshotLoader(database, loader)
	if err != nil {
		return nil, err
	}
	loaderValue, ok := basic.(*sqliteCurrentProjectSnapshotLoader)
	if !ok {
		return nil, fmt.Errorf(
			"construct project-aware current snapshot loader: internal loader type is invalid",
		)
	}
	loaderValue.selectedProjectRuntime = selectedProjectRuntime
	return loaderValue, nil
}

type sqliteCurrentProjectSnapshotLoader struct {
	database               *sql.DB
	loader                 TypeEnvLoader
	selectedProjectRuntime SelectedProjectTypeEnvRuntimeResolver
}

var _ CurrentProjectSnapshotLoader = (*sqliteCurrentProjectSnapshotLoader)(nil)

func (loader *sqliteCurrentProjectSnapshotLoader) LoadCurrentProjectSnapshot(
	ctx context.Context,
	project projectledger.ProjectID,
) (CurrentProjectSnapshot, error) {
	if loader == nil {
		return CurrentProjectSnapshot{}, ErrDatabaseRequired
	}
	adapter := &SQLiteAdapter{
		database:               loader.database,
		loader:                 loader.loader,
		selectedProjectRuntime: loader.selectedProjectRuntime,
	}
	return adapter.LoadCurrentProjectSnapshot(ctx, project)
}

// CurrentProjectSnapshot is the sealed output of one coherent project read.
// The contained MemorySnapshot owns private copied maps and performs no I/O.
type CurrentProjectSnapshot struct {
	project     projectledger.ProjectID
	environment typedmemory.TypeEnv
	codecs      typedmemory.CodecRegistry
	snapshot    typedmemory.MemorySnapshot
}

func NewCurrentProjectSnapshot(
	project projectledger.ProjectID,
	environment typedmemory.TypeEnv,
	codecs typedmemory.CodecRegistry,
	snapshot typedmemory.MemorySnapshot,
) (CurrentProjectSnapshot, error) {
	canonicalProject, err := projectledger.ParseProjectID(project.String())
	if err != nil || canonicalProject != project {
		return CurrentProjectSnapshot{}, fmt.Errorf("current project snapshot requires an exact project identity")
	}
	environmentRef := environment.Ref()
	environmentDigest := environmentRef.Digest()
	if environmentDigest.String() == "" {
		return CurrentProjectSnapshot{}, fmt.Errorf("current project snapshot requires an exact TypeEnv")
	}
	if !memorySnapshotIsPresent(snapshot) {
		return CurrentProjectSnapshot{}, fmt.Errorf("current project snapshot requires an immutable fact view")
	}
	if snapshot.TypeEnvRef() != environment.Ref() {
		return CurrentProjectSnapshot{}, fmt.Errorf("current project snapshot TypeEnv differs from its environment")
	}
	return CurrentProjectSnapshot{
		project:     project,
		environment: environment,
		codecs:      codecs,
		snapshot:    snapshot,
	}, nil
}

func (snapshot CurrentProjectSnapshot) ProjectID() projectledger.ProjectID {
	return snapshot.project
}

func (snapshot CurrentProjectSnapshot) Environment() typedmemory.TypeEnv {
	return snapshot.environment
}

func (snapshot CurrentProjectSnapshot) Codecs() typedmemory.CodecRegistry {
	return snapshot.codecs
}

func (snapshot CurrentProjectSnapshot) Snapshot() typedmemory.MemorySnapshot {
	return snapshot.snapshot
}

type entityContextKey struct {
	entity  typedmemory.EntityID
	context typedmemory.BoundedContextRef
}

type aliasContextKey struct {
	alias   typedmemory.EntityAlias
	context typedmemory.BoundedContextRef
}

type storedAssertionState uint8

const (
	storedAssertionActive storedAssertionState = iota + 1
	storedAssertionRetracted
)

type currentMemorySnapshot struct {
	project               projectledger.ProjectID
	revision              typedmemory.GraphRevision
	typeEnv               typedmemory.TypeEnvRef
	environment           typedmemory.TypeEnv
	codecs                typedmemory.CodecRegistry
	memberOfEngine        MemberOfEvaluationEngine
	classificationEngine  KindClassificationAdmissionEngine
	classificationSources immutableKindClassificationSourceCatalog
	memberOfSources       immutableObservableInputCatalog
	resolutionBasis       typedmemory.ResolutionBasisRef
	assertionRule         typedmemory.RuleRef
	memberOfRepair        typedmemory.RepairPointer
	referenceRepair       typedmemory.RepairPointer
	entityContexts        map[entityContextKey]struct{}
	activeAliases         map[aliasContextKey]typedmemory.EntityID
	assertionStates       map[typedmemory.AssertionID]storedAssertionState
}

func (snapshot *currentMemorySnapshot) GraphRevision() typedmemory.GraphRevision {
	return snapshot.revision
}

func (snapshot *currentMemorySnapshot) TypeEnvRef() typedmemory.TypeEnvRef {
	return snapshot.typeEnv
}

func (snapshot *currentMemorySnapshot) ResolveEntity(
	entity typedmemory.EntityID,
	contextRef typedmemory.BoundedContextRef,
) typedmemory.EntityResolution {
	if _, found := snapshot.environment.BoundedContext(contextRef); !found {
		missing, err := snapshot.missingContextBasis(contextRef)
		if err != nil {
			return nil
		}
		resolution, err := typedmemory.NewUnknownEntityResolution(
			entity,
			contextRef,
			[]typedmemory.MissingBasis{missing},
		)
		if err != nil {
			return nil
		}
		return resolution
	}
	key := entityContextKey{entity: entity, context: contextRef}
	if _, found := snapshot.entityContexts[key]; found {
		resolution, err := typedmemory.NewExactEntityResolution(
			entity,
			contextRef,
			snapshot.resolutionBasis,
		)
		if err == nil {
			return resolution
		}
		return nil
	}
	resolution, err := typedmemory.NewAbsentEntityResolution(
		entity,
		contextRef,
		snapshot.resolutionBasis,
	)
	if err != nil {
		return nil
	}
	return resolution
}

func (snapshot *currentMemorySnapshot) ResolveReference(
	reference typedmemory.StrongRef,
	contextRef typedmemory.BoundedContextRef,
) typedmemory.StrongReferenceResolution {
	if reference == nil {
		return nil
	}
	persisted, isPersisted := reference.(typedmemory.PersistedRef)
	_, contextKnown := snapshot.environment.BoundedContext(contextRef)
	_, referenceKindKnown := snapshot.environment.RefKindDefinition(
		reference.RefKind(),
	)
	if isPersisted && contextKnown && referenceKindKnown {
		entity, entityErr := typedmemory.NewEntityID(
			persisted.ReferenceID().String(),
		)
		_, entityKnown := snapshot.entityContexts[entityContextKey{
			entity:  entity,
			context: contextRef,
		}]
		if entityErr == nil && entityKnown {
			resolved, err := typedmemory.NewResolvedStrongReference(
				reference,
				entity,
				contextRef,
				snapshot.resolutionBasis,
			)
			if err == nil {
				return resolved
			}
		}
	}
	resolution, err := typedmemory.NewUnresolvedStrongReference(
		reference,
		contextRef,
		snapshot.referenceRepair,
	)
	if err != nil {
		return nil
	}
	return resolution
}

func (snapshot *currentMemorySnapshot) EvaluateMemberOf(
	request typedmemory.MemberOfEvaluationRequest,
) typedmemory.MemberOfJudgement {
	query := request.Query()
	preflight, err := snapshot.memberOfStructuralPreflight(query)
	if err != nil {
		return nil
	}
	if missing, ok := preflight.(memberOfSnapshotMissingBasis); ok {
		return snapshot.undefinedMemberOf(request, missing.basis)
	}
	if request.View().PreStateGraphRevision() != snapshot.revision {
		return snapshot.undefinedMemberOfForUnavailableSources(request)
	}
	if !memberOfEvaluationEngineIsPresent(snapshot.memberOfEngine) {
		signature, _ := snapshot.environment.KindSignatureDefinition(
			query.ValueKind(),
			query.ContextSlice().Context(),
		)
		missingEvaluator, missingErr := typedmemory.MissingEvaluatorForMemberOf(
			signature.Evaluator(),
		)
		if missingErr != nil {
			return nil
		}
		return snapshot.undefinedMemberOf(request, missingEvaluator)
	}
	selector, selectable := snapshot.memberOfEngine.(SnapshotObservableInputSelector)
	if !selectable {
		return snapshot.undefinedMemberOfForUnavailableSources(request)
	}
	universe, err := exactPersistedEntityUniverseFromSnapshot(
		snapshot.project,
		query.ContextSlice().Context(),
		snapshot.revision,
		snapshot.entityContexts,
	)
	if err != nil {
		return snapshot.undefinedMemberOfForUnavailableSources(request)
	}
	universeBlob, err := universe.ObservableBlob()
	if err != nil {
		return snapshot.undefinedMemberOfForUnavailableSources(request)
	}
	availableBlobs := snapshot.memberOfSources.Blobs()
	availableBlobs = append(availableBlobs, universeBlob)
	availableCatalog, err := newImmutableObservableInputCatalog(availableBlobs)
	if err != nil {
		return snapshot.undefinedMemberOfForUnavailableSources(request)
	}
	catalogInput, err := newMemberOfEvaluationInput(
		snapshot.project,
		snapshot.environment,
		request,
		availableCatalog.Blobs(),
		universe,
	)
	if err != nil {
		return snapshot.undefinedMemberOfForUnavailableSources(request)
	}
	selection := selector.SelectSnapshotObservableInputs(catalogInput)
	selectedBlobs := []ObservableInputBlob(nil)
	switch selected := selection.(type) {
	case SnapshotObservableInputsSelected:
		selectedBlobs = selected.ObservableInputs()
		if !availableCatalog.ContainsAll(selectedBlobs) {
			return snapshot.undefinedMemberOfForUnavailableSources(request)
		}
	case SnapshotObservableInputsNotApplicable:
		// The exact selected evaluator owns this posture. Preserve the empty
		// source set so it can return a typed no-applicable-source result;
		// Unavailable never reaches this branch.
	default:
		return snapshot.undefinedMemberOfForUnavailableSources(request)
	}
	evaluationInput, err := newMemberOfEvaluationInput(
		snapshot.project,
		snapshot.environment,
		request,
		selectedBlobs,
		universe,
	)
	if err != nil {
		return snapshot.undefinedMemberOfForUnavailableSources(request)
	}
	judgement, err := snapshot.memberOfEngine.EvaluateMemberOf(
		context.Background(),
		evaluationInput,
	)
	if err != nil || !typedmemory.MemberOfJudgementMatchesRequest(request, judgement) {
		return snapshot.undefinedMemberOfForFailedEvaluation(request)
	}
	return judgement
}

func (snapshot *currentMemorySnapshot) EvaluateKindClassification(
	request typedmemory.KindClassificationRequest,
) typedmemory.KindClassificationJudgement {
	candidate, valid := snapshot.kindClassificationCandidate(request)
	if !valid {
		return nil
	}
	key := entityContextKey{
		entity:  candidate.EntityID(),
		context: request.ContextSlice().Context(),
	}
	if _, visible := snapshot.entityContexts[key]; !visible {
		return snapshot.unknownKindClassification(
			request,
			typedmemory.KindUnknownFeatureSourceUnavailable,
			"repair:kind-classification/persist-or-declare-entity",
		)
	}
	visibility, err := NewSnapshotKindClassificationVisibility(
		snapshot.revision,
		candidate.EntityID(),
		request.ContextSlice().Context(),
		snapshot.resolutionBasis,
	)
	if err != nil {
		return snapshot.unknownKindClassification(
			request,
			typedmemory.KindUnknownFeatureSourceUnavailable,
			"repair:kind-classification/rebuild-snapshot-visibility",
		)
	}
	return snapshot.evaluateKindClassificationWithVisibility(request, visibility)
}

func (snapshot *currentMemorySnapshot) EvaluateKindClassificationForAdmission(
	request typedmemory.KindClassificationRequest,
	resolution typedmemory.AdmissionReferenceResolution,
	evaluationChangeOrdinal uint64,
	candidatePrefix typedmemory.OrderedCandidatePrefix,
) typedmemory.KindClassificationJudgement {
	candidate, valid := snapshot.kindClassificationCandidate(request)
	if !valid || resolution == nil ||
		resolution.Entity() != candidate.EntityID() ||
		resolution.Context() != request.ContextSlice().Context() {
		return nil
	}
	var visibility KindClassificationVisibility
	var err error
	switch exact := resolution.(type) {
	case typedmemory.SnapshotReferenceResolution:
		key := entityContextKey{
			entity:  exact.Entity(),
			context: exact.Context(),
		}
		if _, visible := snapshot.entityContexts[key]; !visible {
			return snapshot.unknownKindClassification(
				request,
				typedmemory.KindUnknownFeatureSourceUnavailable,
				"repair:kind-classification/reload-snapshot-entity",
			)
		}
		visibility, err = NewSnapshotKindClassificationVisibility(
			snapshot.revision,
			exact.Entity(),
			exact.Context(),
			exact.ResolutionBasis(),
		)
	case typedmemory.SameBatchDeclarationResolution:
		visibility, err = newProspectiveKindClassificationVisibilityFromPrefix(
			candidatePrefix,
			snapshot.revision,
			evaluationChangeOrdinal,
			exact,
		)
	default:
		return nil
	}
	if err != nil {
		return snapshot.unknownKindClassification(
			request,
			typedmemory.KindUnknownFeatureSourceUnavailable,
			"repair:kind-classification/rebuild-admission-visibility",
		)
	}
	return snapshot.evaluateKindClassificationWithVisibility(request, visibility)
}

func (snapshot *currentMemorySnapshot) kindClassificationCandidate(
	request typedmemory.KindClassificationRequest,
) (typedmemory.ExactKindEntityCandidate, bool) {
	if !request.Valid() ||
		request.LocalKind().TypeEnv() != snapshot.typeEnv ||
		request.LocalKind().Context() != request.ContextSlice().Context() {
		return typedmemory.ExactKindEntityCandidate{}, false
	}
	candidate, exact := request.Candidate().(typedmemory.ExactKindEntityCandidate)
	return candidate, exact
}

func (snapshot *currentMemorySnapshot) evaluateKindClassificationWithVisibility(
	request typedmemory.KindClassificationRequest,
	visibility KindClassificationVisibility,
) typedmemory.KindClassificationJudgement {
	signature, found := snapshot.environment.KindClassificationSignatureDefinition(
		request.LocalKind(),
	)
	if !found || signature.Ref() != request.SignatureEdition() {
		return snapshot.unknownKindClassification(
			request,
			typedmemory.KindUnknownCriterionUnavailable,
			"repair:kind-classification/compile-exact-signature",
		)
	}
	if !kindClassificationAdmissionEngineIsPresent(snapshot.classificationEngine) {
		return snapshot.unknownKindClassification(
			request,
			typedmemory.KindUnknownCriterionUnavailable,
			"repair:kind-classification/install-selected-runtime",
		)
	}
	input, err := NewKindClassificationAdmissionInput(
		snapshot.project,
		snapshot.environment,
		snapshot.codecs,
		request,
		visibility,
		snapshot.classificationSources.Blobs(),
	)
	if err != nil {
		return snapshot.unknownKindClassification(
			request,
			typedmemory.KindUnknownFeatureSourceMalformed,
			"repair:kind-classification/rebuild-snapshot-input",
		)
	}
	judgement, err := snapshot.classificationEngine.EvaluateKindClassification(
		context.Background(),
		input,
	)
	if err != nil || !typedmemory.KindClassificationJudgementMatchesRequest(
		request,
		judgement,
	) {
		return snapshot.unknownKindClassification(
			request,
			typedmemory.KindUnknownFeatureSourceUnavailable,
			"repair:kind-classification/reload-governed-features",
		)
	}
	return judgement
}

func (*currentMemorySnapshot) unknownKindClassification(
	request typedmemory.KindClassificationRequest,
	kind typedmemory.KindClassificationUnknownReasonKind,
	repairRaw string,
) typedmemory.KindClassificationJudgement {
	repair, err := typedmemory.NewRepairPointer(repairRaw)
	if err != nil {
		return nil
	}
	reason, err := typedmemory.NewKindClassificationUnknownReason(kind, repair)
	if err != nil {
		return nil
	}
	unknown, err := typedmemory.NewUnknownKindClassification(
		request,
		[]typedmemory.KindClassificationUnknownReason{reason},
	)
	if err != nil {
		return nil
	}
	return unknown
}

func (snapshot *currentMemorySnapshot) AssertionState(
	assertion typedmemory.AssertionID,
) typedmemory.AssertionState {
	state, found := snapshot.assertionStates[assertion]
	if !found {
		absent, err := typedmemory.NewAbsentAssertionState(assertion, snapshot.assertionRule)
		if err != nil {
			return nil
		}
		return absent
	}
	if state == storedAssertionRetracted {
		retracted, err := typedmemory.NewRetractedAssertionState(assertion, snapshot.assertionRule)
		if err != nil {
			return nil
		}
		return retracted
	}
	active, err := typedmemory.NewActiveAssertion(assertion, snapshot.assertionRule)
	if err != nil {
		return nil
	}
	return active
}

func (snapshot *currentMemorySnapshot) ResolveAlias(
	alias typedmemory.EntityAlias,
	contextRef typedmemory.BoundedContextRef,
) typedmemory.AliasAvailability {
	if _, found := snapshot.environment.BoundedContext(contextRef); !found {
		missing, err := snapshot.missingContextBasis(contextRef)
		if err != nil {
			return nil
		}
		resolution, err := typedmemory.NewUnsettledAliasResolution(
			alias,
			contextRef,
			[]typedmemory.MissingBasis{missing},
		)
		if err != nil {
			return nil
		}
		return resolution
	}
	key := aliasContextKey{alias: alias, context: contextRef}
	entity, found := snapshot.activeAliases[key]
	if found {
		bound, err := typedmemory.NewBoundAliasResolution(
			alias,
			entity,
			contextRef,
			snapshot.resolutionBasis,
		)
		if err != nil {
			return nil
		}
		return bound
	}
	unbound, err := typedmemory.NewUnboundAliasResolution(
		alias,
		contextRef,
		snapshot.resolutionBasis,
	)
	if err != nil {
		return nil
	}
	return unbound
}

func (snapshot *currentMemorySnapshot) ResolveReconciliationBasis(
	basis typedmemory.ReconciliationBasisRef,
	contextRef typedmemory.BoundedContextRef,
) typedmemory.ReconciliationBasisResolution {
	missing, err := typedmemory.NewMissingReconciliationBasis(basis, contextRef)
	if err != nil {
		return nil
	}
	return missing
}

type memberOfSnapshotPreflight interface {
	memberOfSnapshotPreflightVariant()
}

type memberOfSnapshotReady struct{}

func (memberOfSnapshotReady) memberOfSnapshotPreflightVariant() {}

type memberOfSnapshotMissingBasis struct {
	basis typedmemory.MemberOfMissingBasis
}

func (memberOfSnapshotMissingBasis) memberOfSnapshotPreflightVariant() {}

func (snapshot *currentMemorySnapshot) memberOfStructuralPreflight(
	query typedmemory.MemberOfQuery,
) (memberOfSnapshotPreflight, error) {
	valueKind := query.ValueKind()
	queryTypeEnv := valueKind.TypeEnv()
	if queryTypeEnv != snapshot.typeEnv {
		missing, err := typedmemory.MissingTypeEnvForMemberOf(queryTypeEnv)
		return memberOfSnapshotMissingBasis{basis: missing}, err
	}
	contextSlice := query.ContextSlice()
	contextRef := contextSlice.Context()
	signature, found := snapshot.environment.KindSignatureDefinition(
		valueKind,
		contextRef,
	)
	if !found {
		missing, err := typedmemory.MissingKindSignatureForMemberOf(query)
		return memberOfSnapshotMissingBasis{basis: missing}, err
	}
	if _, found := snapshot.environment.EntitySetDefinition(
		signature.EntitySet(),
	); !found {
		missing, err := typedmemory.MissingEntitySetForMemberOf(
			signature.EntitySet(),
		)
		return memberOfSnapshotMissingBasis{basis: missing}, err
	}
	return memberOfSnapshotReady{}, nil
}

func (snapshot *currentMemorySnapshot) undefinedMemberOfForUnavailableSources(
	request typedmemory.MemberOfEvaluationRequest,
) typedmemory.MemberOfJudgement {
	query := request.Query()
	missing, err := typedmemory.MissingObservableSourceForMemberOf(query)
	if err != nil {
		return nil
	}
	return snapshot.undefinedMemberOf(request, missing)
}

func (snapshot *currentMemorySnapshot) undefinedMemberOfForFailedEvaluation(
	request typedmemory.MemberOfEvaluationRequest,
) typedmemory.MemberOfJudgement {
	query := request.Query()
	signature, found := snapshot.environment.KindSignatureDefinition(
		query.ValueKind(),
		query.ContextSlice().Context(),
	)
	if !found {
		missing, err := typedmemory.MissingKindSignatureForMemberOf(query)
		if err != nil {
			return nil
		}
		return snapshot.undefinedMemberOf(request, missing)
	}
	missing, err := typedmemory.MissingEvaluationProvenanceForMemberOf(
		signature.Evaluator(),
	)
	if err != nil {
		return nil
	}
	return snapshot.undefinedMemberOf(request, missing)
}

func (snapshot *currentMemorySnapshot) undefinedMemberOf(
	request typedmemory.MemberOfEvaluationRequest,
	missing typedmemory.MemberOfMissingBasis,
) typedmemory.MemberOfJudgement {
	judgement, err := typedmemory.NewMemberOfUndefined(
		request,
		[]typedmemory.MemberOfMissingBasis{missing},
		snapshot.memberOfRepair,
	)
	if err != nil {
		return nil
	}
	return judgement
}

func (snapshot *currentMemorySnapshot) missingContextBasis(
	contextRef typedmemory.BoundedContextRef,
) (typedmemory.MissingBasis, error) {
	value := "active-typeenv-context:" + snapshot.typeEnv.String() + "/" + contextRef.String()
	return typedmemory.NewMissingBasis(value)
}

type storedEntityContextRow struct {
	EntityID   string `json:"entity_id"`
	ContextRef string `json:"context_ref"`
}

type storedAliasChangeRow struct {
	EventRevision int64   `json:"event_revision"`
	ChangeOrdinal int64   `json:"change_ordinal"`
	ChangeRef     string  `json:"change_ref"`
	ChangeKind    string  `json:"change_kind"`
	ContextRef    string  `json:"context_ref"`
	Alias         string  `json:"alias"`
	Replacement   *string `json:"replacement"`
	EntityID      string  `json:"entity_id"`
	SupersedesRef string  `json:"supersedes_ref"`
}

type storedAssertionRow struct {
	EventRevision          int64  `json:"event_revision"`
	EventExpectedRevision  int64  `json:"event_expected_revision"`
	ChangeOrdinal          int64  `json:"change_ordinal"`
	AssertionID            string `json:"assertion_id"`
	EventBasisTypeEnv      string `json:"event_basis_type_env"`
	AdmissionTypeEnv       string `json:"admission_type_env"`
	AdmissionBasisRevision int64  `json:"admission_basis_revision"`
	WriterGeneration       int64  `json:"writer_generation"`
	WriterProvenance       string `json:"writer_provenance"`
}

type storedObservableInputRow struct {
	EventRevision int64  `json:"event_revision"`
	EventRef      string `json:"event_ref"`
	Reference     string `json:"observable_input_ref"`
	Digest        string `json:"observable_input_digest"`
	CanonicalHex  string `json:"canonical_observable_input_hex"`
}

type storedKindClassificationSourceRow struct {
	EventRevision int64  `json:"event_revision"`
	EventRef      string `json:"event_ref"`
	Reference     string `json:"source_ref"`
	Digest        string `json:"source_digest"`
	CanonicalHex  string `json:"canonical_source_hex"`
}

type storedAdmissionCoordinate struct {
	EventRef             string `json:"event_ref"`
	IdempotencyKey       string `json:"idempotency_key"`
	WriterGeneration     int64  `json:"writer_generation"`
	GenerationProvenance string `json:"generation_provenance"`
}

type decodedAliasChange struct {
	eventRevision int64
	changeOrdinal int64
	changeRef     string
	changeKind    string
	context       typedmemory.BoundedContextRef
	alias         typedmemory.EntityAlias
	replacement   *typedmemory.EntityAlias
	entity        typedmemory.EntityID
	supersedesRef string
}

func (adapter *SQLiteAdapter) LoadCurrentProjectSnapshot(
	ctx context.Context,
	project projectledger.ProjectID,
) (CurrentProjectSnapshot, error) {
	if ctx == nil {
		return CurrentProjectSnapshot{}, fmt.Errorf("load current project snapshot: context is required")
	}
	if adapter == nil || adapter.database == nil {
		return CurrentProjectSnapshot{}, ErrDatabaseRequired
	}
	if adapter.loader == nil {
		return CurrentProjectSnapshot{}, ErrTypeEnvLoaderRequired
	}
	canonicalProject, projectErr := projectledger.ParseProjectID(project.String())
	if projectErr != nil || canonicalProject != project {
		return CurrentProjectSnapshot{}, fmt.Errorf("load current project snapshot: project identity is required")
	}
	transaction, err := sqlitetransaction.BeginRead(ctx, adapter.database)
	if err != nil {
		return CurrentProjectSnapshot{}, fmt.Errorf("begin current project snapshot read: %w", err)
	}
	loaded, err := adapter.loadCurrentProjectSnapshot(ctx, transaction, project)
	if err != nil {
		return CurrentProjectSnapshot{}, rollbackError(ctx, transaction, err)
	}
	if err := rollbackSuccess(ctx, transaction); err != nil {
		return CurrentProjectSnapshot{}, err
	}
	return loaded, nil
}

func (adapter *SQLiteAdapter) loadCurrentProjectSnapshot(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectledger.ProjectID,
) (CurrentProjectSnapshot, error) {
	if err := requireGenericStorageCapability(ctx, transaction); err != nil {
		return CurrentProjectSnapshot{}, err
	}
	head, err := loadHeadWithScanner(ctx, transaction, project)
	if err != nil {
		return CurrentProjectSnapshot{}, err
	}
	runtime, err := adapter.resolveTypeEnvRuntimeTx(
		ctx,
		transaction,
		project,
		head.Revision(),
		head.ActiveTypeEnv(),
	)
	if err != nil {
		return CurrentProjectSnapshot{}, err
	}
	if err := verifyCurrentHeadClosure(ctx, transaction, head); err != nil {
		return CurrentProjectSnapshot{}, err
	}
	if _, err := verifyExactV46AdmissionIntegrity(ctx, transaction, head); err != nil {
		return CurrentProjectSnapshot{}, err
	}
	entities, err := loadCurrentEntityContexts(ctx, transaction, head)
	if err != nil {
		return CurrentProjectSnapshot{}, err
	}
	aliases, err := loadCurrentAliases(ctx, transaction, head, entities)
	if err != nil {
		return CurrentProjectSnapshot{}, err
	}
	assertions, err := loadCurrentAssertionStates(ctx, transaction, head)
	if err != nil {
		return CurrentProjectSnapshot{}, err
	}
	memberOfSources, err := loadCurrentObservableInputCatalog(
		ctx,
		transaction,
		head,
	)
	if err != nil {
		return CurrentProjectSnapshot{}, err
	}
	classificationSources, err := loadCurrentKindClassificationSourceCatalog(
		ctx,
		transaction,
		head,
	)
	if err != nil {
		return CurrentProjectSnapshot{}, err
	}
	classificationSources, err = extendKindClassificationSourceCatalogWithSealedHistorical(
		project,
		runtime.classification,
		memberOfSources,
		classificationSources,
	)
	if err != nil {
		return CurrentProjectSnapshot{}, fmt.Errorf(
			"adapt sealed historical classification delivery sources: %w",
			err,
		)
	}
	resolutionBasis, err := snapshotResolutionBasis(project, head.Revision())
	if err != nil {
		return CurrentProjectSnapshot{}, err
	}
	assertionRule, err := snapshotAssertionRule(project, head.Revision())
	if err != nil {
		return CurrentProjectSnapshot{}, err
	}
	memberRepair, err := typedmemory.NewRepairPointer(memberOfSnapshotRepair)
	if err != nil {
		return CurrentProjectSnapshot{}, err
	}
	referenceRepair, err := typedmemory.NewRepairPointer(referenceSnapshotRepair)
	if err != nil {
		return CurrentProjectSnapshot{}, err
	}
	snapshot := &currentMemorySnapshot{
		project:               project,
		revision:              head.Revision(),
		typeEnv:               head.ActiveTypeEnv(),
		environment:           runtime.environment,
		codecs:                runtime.codecs,
		memberOfEngine:        runtime.memberOf,
		classificationEngine:  runtime.classification,
		classificationSources: classificationSources,
		memberOfSources:       memberOfSources,
		resolutionBasis:       resolutionBasis,
		assertionRule:         assertionRule,
		memberOfRepair:        memberRepair,
		referenceRepair:       referenceRepair,
		entityContexts:        entities,
		activeAliases:         aliases,
		assertionStates:       assertions,
	}
	return NewCurrentProjectSnapshot(
		project,
		runtime.environment,
		runtime.codecs,
		snapshot,
	)
}

func loadCurrentObservableInputCatalog(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	head GraphHead,
) (immutableObservableInputCatalog, error) {
	encoded, err := loadJSONAggregate(
		ctx,
		transaction,
		`SELECT COALESCE(json_group_array(json_object(
			'event_revision', graph_revision,
			'event_ref', event_ref,
			'observable_input_ref', observable_input_ref,
			'observable_input_digest', observable_input_digest,
			'canonical_observable_input_hex', canonical_observable_input_hex
		)), '[]')
		FROM (
			SELECT event.graph_revision, observable.event_ref,
				observable.observable_input_ref,
				observable.observable_input_digest,
				hex(observable.canonical_observable_input_bytes)
					AS canonical_observable_input_hex
			FROM typed_memory_observable_input_blobs observable
			JOIN typed_memory_graph_events event
				ON event.project_id = observable.project_id
				AND event.event_ref = observable.event_ref
			JOIN typed_memory_graph_commits commit_row
				ON commit_row.project_id = event.project_id
				AND commit_row.commit_ref = event.commit_ref
			WHERE observable.project_id = ? AND event.graph_revision <= ?
			ORDER BY event.graph_revision, observable.event_ref,
				observable.observable_input_ref
		)`,
		head,
	)
	if err != nil {
		return immutableObservableInputCatalog{}, err
	}
	rows := []storedObservableInputRow{}
	if err := json.Unmarshal([]byte(encoded), &rows); err != nil {
		return immutableObservableInputCatalog{}, storedAdmissionIntegrity(
			"decode current observable-input catalog",
			err,
		)
	}
	blobs := make([]ObservableInputBlob, 0, len(rows))
	for _, row := range rows {
		blob, err := decodeStoredObservableInputRow(row)
		if err != nil {
			return immutableObservableInputCatalog{}, err
		}
		blobs = append(blobs, blob)
	}
	catalog, err := newImmutableObservableInputCatalog(blobs)
	if err != nil {
		return immutableObservableInputCatalog{}, storedAdmissionIntegrity(
			"construct current observable-input catalog",
			err,
		)
	}
	return catalog, nil
}

func loadCurrentKindClassificationSourceCatalog(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	head GraphHead,
) (immutableKindClassificationSourceCatalog, error) {
	encoded, err := loadJSONAggregate(
		ctx,
		transaction,
		`SELECT COALESCE(json_group_array(json_object(
			'event_revision', graph_revision,
			'event_ref', event_ref,
			'source_ref', source_ref,
			'source_digest', source_digest,
			'canonical_source_hex', canonical_source_hex
		)), '[]')
		FROM (
			SELECT event.graph_revision, source.event_ref,
				source.source_ref, source.source_digest,
				hex(source.canonical_source_bytes) AS canonical_source_hex
			FROM typed_memory_kind_classification_source_blobs_v54 source
			JOIN typed_memory_graph_events event
				ON event.project_id = source.project_id
				AND event.event_ref = source.event_ref
			JOIN typed_memory_graph_commits commit_row
				ON commit_row.project_id = event.project_id
				AND commit_row.commit_ref = event.commit_ref
			WHERE source.project_id = ? AND event.graph_revision <= ?
			ORDER BY event.graph_revision, source.event_ref, source.source_ref
		)`,
		head,
	)
	if err != nil {
		return immutableKindClassificationSourceCatalog{}, err
	}
	rows := []storedKindClassificationSourceRow{}
	if err := json.Unmarshal([]byte(encoded), &rows); err != nil {
		return immutableKindClassificationSourceCatalog{}, storedAdmissionIntegrity(
			"decode current kind-classification source catalog",
			err,
		)
	}
	byReference := make(map[string]KindClassificationSourceBlob, len(rows))
	for _, row := range rows {
		blob, decodeErr := decodeStoredKindClassificationSourceRow(row)
		if decodeErr != nil {
			return immutableKindClassificationSourceCatalog{}, decodeErr
		}
		previous, repeated := byReference[blob.Reference().String()]
		if !repeated {
			byReference[blob.Reference().String()] = blob
			continue
		}
		if previous.Digest() != blob.Digest() ||
			!bytes.Equal(previous.Bytes(), blob.Bytes()) {
			return immutableKindClassificationSourceCatalog{}, storedAdmissionIntegrity(
				"kind-classification source reference has conflicting committed content",
				nil,
			)
		}
	}
	blobs := make([]KindClassificationSourceBlob, 0, len(byReference))
	for _, blob := range byReference {
		blobs = append(blobs, blob)
	}
	catalog, err := newImmutableKindClassificationSourceCatalog(blobs)
	if err != nil {
		return immutableKindClassificationSourceCatalog{}, storedAdmissionIntegrity(
			"construct current kind-classification source catalog",
			err,
		)
	}
	return catalog, nil
}

func decodeStoredKindClassificationSourceRow(
	row storedKindClassificationSourceRow,
) (KindClassificationSourceBlob, error) {
	if row.EventRevision <= 0 || row.EventRef == "" {
		return KindClassificationSourceBlob{}, storedAdmissionIntegrity(
			"stored kind-classification source coordinate is malformed",
			nil,
		)
	}
	reference, err := typedmemory.NewCarrierRef(row.Reference)
	if err != nil {
		return KindClassificationSourceBlob{}, storedAdmissionIntegrity(
			"stored kind-classification source reference is malformed",
			err,
		)
	}
	digest, err := typedmemory.NewSHA256Digest(row.Digest)
	if err != nil {
		return KindClassificationSourceBlob{}, storedAdmissionIntegrity(
			"stored kind-classification source digest is malformed",
			err,
		)
	}
	canonical, err := hex.DecodeString(row.CanonicalHex)
	if err != nil {
		return KindClassificationSourceBlob{}, storedAdmissionIntegrity(
			"stored kind-classification source bytes are malformed",
			err,
		)
	}
	blob, err := NewKindClassificationSourceBlob(reference, digest, canonical)
	if err != nil {
		return KindClassificationSourceBlob{}, storedAdmissionIntegrity(
			"stored kind-classification source content is corrupted",
			err,
		)
	}
	return blob, nil
}

func decodeStoredObservableInputRow(
	row storedObservableInputRow,
) (ObservableInputBlob, error) {
	if row.EventRevision <= 0 || row.EventRef == "" {
		return ObservableInputBlob{}, storedAdmissionIntegrity(
			"stored observable-input coordinate is malformed",
			nil,
		)
	}
	reference, err := typedmemory.NewObservableInputRef(row.Reference)
	if err != nil {
		return ObservableInputBlob{}, storedAdmissionIntegrity(
			"stored observable-input reference is malformed",
			err,
		)
	}
	digest, err := typedmemory.NewSHA256Digest(row.Digest)
	if err != nil {
		return ObservableInputBlob{}, storedAdmissionIntegrity(
			"stored observable-input digest is malformed",
			err,
		)
	}
	canonical, err := hex.DecodeString(row.CanonicalHex)
	if err != nil {
		return ObservableInputBlob{}, storedAdmissionIntegrity(
			"stored observable-input bytes are malformed",
			err,
		)
	}
	blob, err := NewObservableInputBlob(reference, digest, canonical)
	if err != nil {
		return ObservableInputBlob{}, storedAdmissionIntegrity(
			"stored observable-input content is corrupted",
			err,
		)
	}
	return blob, nil
}

func verifyCurrentHeadClosure(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	head GraphHead,
) error {
	var eventCount int64
	var maximumRevision sql.NullInt64
	project := head.Project()
	projectText := project.String()
	err := transaction.ScanOne(
		ctx,
		`SELECT COUNT(*), MAX(graph_revision)
		FROM typed_memory_graph_events
		WHERE project_id = ?`,
		[]any{projectText},
		[]any{&eventCount, &maximumRevision},
	)
	if err != nil {
		return fmt.Errorf("inspect current typed-memory event spine: %w", err)
	}
	headRevision := head.Revision()
	expectedCount, exact := sqliteIntegerFromUint64(headRevision.Value())
	if !exact {
		return storedAdmissionIntegrity("graph head revision exceeds SQLite INTEGER", nil)
	}
	if eventCount != expectedCount {
		return storedAdmissionIntegrity("graph head revision is not a contiguous event spine", nil)
	}
	if expectedCount == 0 {
		if maximumRevision.Valid || head.LastEventRef() != "" || head.LastCommitRef() != "" {
			return storedAdmissionIntegrity("empty graph head has non-empty closure", nil)
		}
		return nil
	}
	if !maximumRevision.Valid || maximumRevision.Int64 != expectedCount {
		return storedAdmissionIntegrity("graph head does not match the latest event revision", nil)
	}
	var eventRevision int64
	var eventResultTypeEnv string
	var eventCommitRef string
	var commitRevision int64
	var commitEventRef string
	err = transaction.ScanOne(
		ctx,
		`SELECT event.graph_revision, event.result_type_env_ref, event.commit_ref,
			commit_row.graph_revision, commit_row.event_ref
		FROM typed_memory_graph_events event
		JOIN typed_memory_graph_commits commit_row
			ON commit_row.project_id = event.project_id
			AND commit_row.commit_ref = event.commit_ref
		WHERE event.project_id = ? AND event.event_ref = ?
			AND commit_row.commit_ref = ?`,
		[]any{
			projectText,
			head.LastEventRef(),
			head.LastCommitRef(),
		},
		[]any{
			&eventRevision,
			&eventResultTypeEnv,
			&eventCommitRef,
			&commitRevision,
			&commitEventRef,
		},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return storedAdmissionIntegrity("graph head event/commit closure is missing", nil)
	}
	if err != nil {
		return fmt.Errorf("load current typed-memory head closure: %w", err)
	}
	activeTypeEnv := head.ActiveTypeEnv()
	activeTypeEnvText := activeTypeEnv.String()
	matches := eventRevision == expectedCount &&
		commitRevision == expectedCount &&
		eventResultTypeEnv == activeTypeEnvText &&
		eventCommitRef == head.LastCommitRef() &&
		commitEventRef == head.LastEventRef()
	if !matches {
		return storedAdmissionIntegrity("graph head event/commit closure is inconsistent", nil)
	}
	return nil
}

type verifiedMaterializationClosure struct {
	eventRef string
	commit   string
	digest   typedmemory.SHA256Digest
}

func verifyExactV46AdmissionIntegrity(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	head GraphHead,
) (*verifiedMaterializationClosure, error) {
	encoded, err := loadJSONAggregate(
		ctx,
		transaction,
		`SELECT COALESCE(json_group_array(json_object(
			'event_ref', event_ref,
			'idempotency_key', idempotency_key,
			'writer_generation', writer_generation,
			'generation_provenance', generation_provenance
		)), '[]')
		FROM (
			SELECT event.event_ref,
				COALESCE(history.idempotency_key, '') AS idempotency_key,
				COALESCE(generation.writer_generation, 0) AS writer_generation,
				COALESCE(generation.provenance_kind, '') AS generation_provenance
			FROM typed_memory_graph_events event
			LEFT JOIN typed_memory_idempotency_history history
				ON history.project_id = event.project_id
				AND history.event_ref = event.event_ref
			LEFT JOIN typed_memory_event_writer_generations generation
				ON generation.project_id = event.project_id
				AND generation.event_ref = event.event_ref
			WHERE event.project_id = ? AND event.graph_revision <= ?
			ORDER BY event.graph_revision
		)`,
		head,
	)
	if err != nil {
		return nil, fmt.Errorf("inspect exact v46 admission coordinates: %w", err)
	}
	coordinates := []storedAdmissionCoordinate{}
	if err := json.Unmarshal([]byte(encoded), &coordinates); err != nil {
		return nil, storedAdmissionIntegrity("decode exact v46 admission coordinates", err)
	}
	revision := head.Revision()
	if uint64(len(coordinates)) != revision.Value() {
		return nil, storedAdmissionIntegrity("v46 event/idempotency completeness", nil)
	}
	project := head.Project()
	var current *verifiedMaterializationClosure
	for _, coordinate := range coordinates {
		verified, err := verifyExactV46AdmissionCoordinate(
			ctx,
			transaction,
			project,
			coordinate,
		)
		if err != nil {
			return nil, err
		}
		if coordinate.EventRef == head.LastEventRef() {
			current = verified
		}
	}
	return current, nil
}

func verifyExactV46AdmissionCoordinate(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	project projectledger.ProjectID,
	coordinate storedAdmissionCoordinate,
) (*verifiedMaterializationClosure, error) {
	key, err := NewIdempotencyKey(coordinate.IdempotencyKey)
	if err != nil || coordinate.EventRef == "" {
		return nil, storedAdmissionIntegrity("stored v46 admission coordinate", err)
	}
	common, found, err := loadDurableGenericCommonRow(ctx, transaction, project, key)
	if err != nil {
		return nil, err
	}
	if !found || common.idempotencyEventRef != coordinate.EventRef {
		return nil, storedAdmissionIntegrity("v46 event/idempotency correlation", nil)
	}
	admission, admissionFound, err := loadDurableV46AdmissionRow(
		ctx,
		transaction,
		project,
		coordinate.EventRef,
	)
	if err != nil {
		return nil, err
	}
	closure, closureFound, err := loadDurableV46ClosureRow(
		ctx,
		transaction,
		project,
		coordinate.EventRef,
	)
	if err != nil {
		return nil, err
	}
	version := AdmissionContractV1()
	switch coordinate.WriterGeneration {
	case 0:
		if coordinate.GenerationProvenance != "" || admissionFound || closureFound {
			return nil, storedAdmissionIntegrity("reviewed identity-reconciliation generation boundary", nil)
		}
		verified, found, err := identityreconciliation.VerifyCommittedClosure(
			ctx,
			transaction,
			project,
			coordinate.EventRef,
		)
		if err != nil {
			return nil, storedAdmissionIntegrity("reviewed identity-reconciliation closure", err)
		}
		if !found {
			return nil, storedAdmissionIntegrity("event writer generation marker", nil)
		}
		return &verifiedMaterializationClosure{
			eventRef: verified.EventRef(),
			commit:   verified.CommitRef(),
			digest:   verified.MaterializationDigest(),
		}, nil
	case 45:
		if coordinate.GenerationProvenance != "migration_v45_backfill" ||
			admissionFound || closureFound {
			return nil, storedAdmissionIntegrity("legacy-v45 event generation boundary", nil)
		}
		detail := durableLegacyV45AdmissionDetail{common: common}
		if err := verifySnapshotLegacyV45Admission(detail); err != nil {
			return nil, err
		}
		return nil, nil
	case 46:
		if coordinate.GenerationProvenance != "writer_v46" ||
			!admissionFound || !closureFound {
			return nil, storedAdmissionIntegrity("exact-v46 event generation boundary", nil)
		}
	case 53:
		if coordinate.GenerationProvenance != "writer_v53" ||
			!admissionFound || !closureFound {
			return nil, storedAdmissionIntegrity("exact-v53 event generation boundary", nil)
		}
		availability, availabilityErr := loadRelationalAssertionStorageAvailability(
			ctx,
			transaction,
		)
		if availabilityErr != nil {
			return nil, availabilityErr
		}
		if availability != genericStorageExact {
			return nil, ErrStorageGenerationUnavailable
		}
		version = AdmissionContractV2()
	case 54:
		if coordinate.GenerationProvenance != "writer_v54" ||
			!admissionFound || !closureFound ||
			admission.basisKind != typedmemory.ContextSliceClassificationAdmissionBasis.String() {
			return nil, storedAdmissionIntegrity("exact-v54 event generation boundary", nil)
		}
		relationalAvailability, relationalErr := loadRelationalAssertionStorageAvailability(
			ctx,
			transaction,
		)
		if relationalErr != nil {
			return nil, relationalErr
		}
		classificationAvailability, classificationErr := loadKindClassificationStorageAvailability(
			ctx,
			transaction,
		)
		if classificationErr != nil {
			return nil, classificationErr
		}
		if relationalAvailability != genericStorageExact ||
			classificationAvailability != genericStorageExact {
			return nil, ErrStorageGenerationUnavailable
		}
		version = AdmissionContractV2()
	default:
		return nil, storedAdmissionIntegrity("event writer generation marker", nil)
	}
	if isStoredProjectTypeEnvActivation(common) {
		if !version.IsV1() {
			return nil, storedAdmissionIntegrity(
				"project TypeEnv activation must remain on writer-v46",
				nil,
			)
		}
		return verifyExactProjectTypeEnvActivationCoordinate(
			ctx,
			transaction,
			project,
			coordinate,
			common,
			admission,
			closure,
		)
	}
	if err := verifyStoredGenericEventIdentity(project, common); err != nil {
		return nil, err
	}
	if err := verifyStoredGenericAdmissionCarriers(admission); err != nil {
		return nil, err
	}
	if err := verifyStoredGenericAdmissionLinks(common, admission, closure); err != nil {
		return nil, err
	}
	if _, err := verifyStoredGenericCanonicalCarrier(
		"stored materialization-closure carrier",
		closure.materializationBytes,
		closure.materializationDigest,
	); err != nil {
		return nil, err
	}
	digest, err := verifySnapshotExactV46Materialization(
		ctx,
		transaction,
		project,
		common,
		admission,
		closure,
		version,
	)
	if err != nil {
		return nil, err
	}
	return &verifiedMaterializationClosure{
		eventRef: coordinate.EventRef,
		commit:   closure.commitRef,
		digest:   digest,
	}, nil
}

func verifySnapshotExactV46Materialization(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	common durableGenericCommonRow,
	admission durableV46AdmissionRow,
	closure durableV46ClosureRow,
	version AdmissionContractVersion,
) (typedmemory.SHA256Digest, error) {
	manifestDigest, err := typedmemory.NewSHA256Digest(admission.manifestDigest)
	if err != nil {
		return typedmemory.SHA256Digest{},
			storedAdmissionIntegrity("snapshot materialization manifest digest", err)
	}
	basisRevision, exact := uint64FromSQLiteInteger(admission.basisRevision)
	if !exact {
		return typedmemory.SHA256Digest{},
			storedAdmissionIntegrity("snapshot admission-basis revision", nil)
	}
	manifest, err := decodeExpectedMaterializationManifest(
		admission.manifestBytes,
		manifestDigest,
		basisRevision,
	)
	if err != nil {
		return typedmemory.SHA256Digest{}, err
	}
	if err := verifyActualMaterializationManifest(
		ctx,
		source,
		project,
		common.idempotencyEventRef,
		manifest,
	); err != nil {
		return typedmemory.SHA256Digest{}, err
	}
	actualFootprint, err := loadDurableV46FootprintRow(
		ctx,
		source,
		project,
		common.idempotencyEventRef,
	)
	if err != nil {
		return typedmemory.SHA256Digest{}, err
	}
	rowDigests, err := loadSnapshotV46RowDigests(
		ctx,
		source,
		project,
		common.idempotencyEventRef,
		manifest,
		version,
	)
	if err != nil {
		return typedmemory.SHA256Digest{}, err
	}
	input, err := snapshotMaterializationClosureInput(
		common,
		admission,
		manifestDigest,
		actualFootprint.footprint,
		rowDigests,
	)
	if err != nil {
		return typedmemory.SHA256Digest{}, err
	}
	expectedBytes := canonicalMaterializationClosureFromInput(input)
	expectedDigest, err := digestBytes(expectedBytes)
	if err != nil {
		return typedmemory.SHA256Digest{},
			storedAdmissionIntegrity("snapshot materialization closure digest", err)
	}
	closureFootprint := materializationFootprintFromClosure(closure)
	matches := closureFootprint == actualFootprint.footprint &&
		actualFootprint.topLevelChangeCount == common.eventChangeCount &&
		closure.entityCount == common.commitEntityCount &&
		closure.entityContextCount == common.commitEntityContextCount &&
		bytes.Equal(closure.materializationBytes, expectedBytes) &&
		closure.materializationDigest == expectedDigest.String()
	if !matches {
		return typedmemory.SHA256Digest{},
			storedAdmissionIntegrity("snapshot exact materialization closure", nil)
	}
	return expectedDigest, nil
}

func snapshotMaterializationClosureInput(
	common durableGenericCommonRow,
	admission durableV46AdmissionRow,
	manifestDigest typedmemory.SHA256Digest,
	footprint genericMaterializationFootprint,
	rowDigests []string,
) (materializationClosureInput, error) {
	graphRevision, err := graphRevisionFromSQLite(common.eventRevision)
	if err != nil {
		return materializationClosureInput{}, storedAdmissionIntegrity(
			"snapshot materialization graph revision",
			err,
		)
	}
	eventDigest, err := typedmemory.NewSHA256Digest(common.eventDigest)
	if err != nil {
		return materializationClosureInput{}, storedAdmissionIntegrity(
			"snapshot materialization event digest",
			err,
		)
	}
	basisKind, err := typedmemory.ParseAdmissionBasisKind(admission.basisKind)
	if err != nil {
		return materializationClosureInput{}, storedAdmissionIntegrity(
			"snapshot materialization basis kind",
			err,
		)
	}
	requestDigest, err := typedmemory.NewSHA256Digest(admission.requestDigest)
	if err != nil {
		return materializationClosureInput{}, storedAdmissionIntegrity(
			"snapshot materialization request digest",
			err,
		)
	}
	semanticDigest, err := typedmemory.NewSHA256Digest(admission.semanticDigest)
	if err != nil {
		return materializationClosureInput{}, storedAdmissionIntegrity(
			"snapshot materialization semantic digest",
			err,
		)
	}
	envelopeDigest, err := typedmemory.NewSHA256Digest(admission.envelopeDigest)
	if err != nil {
		return materializationClosureInput{}, storedAdmissionIntegrity(
			"snapshot materialization envelope digest",
			err,
		)
	}
	basisDigest, err := typedmemory.NewSHA256Digest(admission.basisDigest)
	if err != nil {
		return materializationClosureInput{}, storedAdmissionIntegrity(
			"snapshot materialization basis digest",
			err,
		)
	}
	identity := genericEventIdentity{
		nextRevision: graphRevision,
		commitRef:    common.eventCommitRef,
		eventRef:     common.idempotencyEventRef,
		eventDigest:  eventDigest,
	}
	return materializationClosureInput{
		identity:       identity,
		basisKind:      basisKind,
		requestDigest:  requestDigest,
		semanticDigest: semanticDigest,
		envelopeDigest: envelopeDigest,
		basisDigest:    basisDigest,
		manifestDigest: manifestDigest,
		footprint:      footprint,
		rowDigests:     rowDigests,
	}, nil
}

func loadSnapshotV46RowDigests(
	ctx context.Context,
	source scanner,
	project projectledger.ProjectID,
	eventRef string,
	manifest expectedMaterializationManifest,
	_ AdmissionContractVersion,
) ([]string, error) {
	rowDigests, err := loadStoredV46DigestRows(ctx, source, project, eventRef)
	if err != nil {
		return nil, err
	}
	valueDigests, err := loadStoredV46TypedValueDigests(
		ctx,
		source,
		project,
		eventRef,
		preparedAdmission{},
	)
	if err != nil {
		return nil, err
	}
	entityContextDigests, err := loadStoredV46EntityContextDigests(
		ctx,
		source,
		project,
		eventRef,
	)
	if err != nil {
		return nil, err
	}
	rowDigests = append(rowDigests, valueDigests...)
	rowDigests = append(rowDigests, entityContextDigests...)
	for _, declaration := range manifest.declarations {
		rowDigests = append(
			rowDigests,
			"entity-declaration:"+declaration.declarationDigest.String(),
		)
	}
	for _, prefix := range manifest.orderedPrefixes {
		rowDigests = append(
			rowDigests,
			"ordered-prefix:"+prefix.prefixDigest.String(),
		)
	}
	for _, rowDigest := range rowDigests {
		if err := validateDurableV46RowDigest(rowDigest); err != nil {
			return nil, storedAdmissionIntegrity("snapshot materialization row digest", err)
		}
	}
	sort.Strings(rowDigests)
	return rowDigests, nil
}

func verifySnapshotLegacyV45Admission(
	detail durableLegacyV45AdmissionDetail,
) error {
	common := detail.commonRow()
	basisTypeEnv, err := parseTypeEnvRef(common.eventBasisTypeEnv)
	if err != nil {
		return storedAdmissionIntegrity("legacy-v45 basis TypeEnv", err)
	}
	resultTypeEnv, err := parseTypeEnvRef(common.eventResultTypeEnv)
	if err != nil {
		return storedAdmissionIntegrity("legacy-v45 result TypeEnv", err)
	}
	matches := common.eventKind == "declare_entity" &&
		common.eventAuthorityClass == "non_binding_semantic_assertion" &&
		common.eventChangeCount == 1 &&
		basisTypeEnv == resultTypeEnv &&
		common.commitEntityCount == 1 &&
		common.commitEntityContextCount == 1
	if !matches {
		return storedAdmissionIntegrity("legacy-v45 durable declaration shape", nil)
	}
	return nil
}

func loadCurrentEntityContexts(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	head GraphHead,
) (map[entityContextKey]struct{}, error) {
	encoded, err := loadJSONAggregate(
		ctx,
		transaction,
		`SELECT COALESCE(json_group_array(json_object(
			'entity_id', entity_id,
			'context_ref', bounded_context_ref
		)), '[]')
		FROM (
			SELECT entity_context.entity_id, entity_context.bounded_context_ref
			FROM typed_memory_entity_contexts entity_context
			JOIN typed_memory_graph_events event
				ON event.project_id = entity_context.project_id
				AND event.event_ref = entity_context.declared_event_ref
			JOIN typed_memory_graph_commits commit_row
				ON commit_row.project_id = event.project_id
				AND commit_row.commit_ref = event.commit_ref
			WHERE entity_context.project_id = ? AND event.graph_revision <= ?
			ORDER BY entity_context.entity_id, entity_context.bounded_context_ref
		)`,
		head,
	)
	if err != nil {
		return nil, err
	}
	rows := []storedEntityContextRow{}
	if err := json.Unmarshal([]byte(encoded), &rows); err != nil {
		return nil, storedAdmissionIntegrity("decode stored entity/context projection", err)
	}
	result := make(map[entityContextKey]struct{}, len(rows))
	for _, row := range rows {
		entity, err := typedmemory.NewEntityID(row.EntityID)
		if err != nil {
			return nil, storedAdmissionIntegrity("stored entity ID is malformed", err)
		}
		contextRef, err := typedmemory.NewBoundedContextRef(row.ContextRef)
		if err != nil {
			return nil, storedAdmissionIntegrity("stored entity context is malformed", err)
		}
		key := entityContextKey{entity: entity, context: contextRef}
		if _, exists := result[key]; exists {
			return nil, storedAdmissionIntegrity("stored entity/context projection is duplicated", nil)
		}
		result[key] = struct{}{}
	}
	return result, nil
}

func loadCurrentAliases(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	head GraphHead,
	entities map[entityContextKey]struct{},
) (map[aliasContextKey]typedmemory.EntityID, error) {
	encoded, err := loadJSONAggregate(
		ctx,
		transaction,
		`SELECT COALESCE(json_group_array(json_object(
			'event_revision', graph_revision,
			'change_ordinal', change_ordinal,
			'change_ref', alias_change_ref,
			'change_kind', change_kind,
			'context_ref', bounded_context_ref,
			'alias', alias,
			'replacement', replacement_alias,
			'entity_id', entity_id,
			'supersedes_ref', supersedes_alias_change_ref
		)), '[]')
		FROM (
			SELECT event.graph_revision, alias_change.change_ordinal,
				alias_change.alias_change_ref, alias_change.change_kind,
				alias_change.bounded_context_ref, alias_change.alias,
				alias_change.replacement_alias, alias_change.entity_id,
				alias_change.supersedes_alias_change_ref
			FROM typed_memory_alias_changes alias_change
			JOIN typed_memory_graph_events event
				ON event.project_id = alias_change.project_id
				AND event.event_ref = alias_change.event_ref
			JOIN typed_memory_graph_commits commit_row
				ON commit_row.project_id = event.project_id
				AND commit_row.commit_ref = event.commit_ref
			WHERE alias_change.project_id = ? AND event.graph_revision <= ?
			ORDER BY event.graph_revision, alias_change.change_ordinal
		)`,
		head,
	)
	if err != nil {
		return nil, err
	}
	rows := []storedAliasChangeRow{}
	if err := json.Unmarshal([]byte(encoded), &rows); err != nil {
		return nil, storedAdmissionIntegrity("decode stored alias projection", err)
	}
	changes := make(map[string]decodedAliasChange, len(rows))
	for _, row := range rows {
		change, err := decodeAliasChange(row)
		if err != nil {
			return nil, err
		}
		if _, exists := changes[change.changeRef]; exists {
			return nil, storedAdmissionIntegrity("stored alias change ref is duplicated", nil)
		}
		entityKey := entityContextKey{entity: change.entity, context: change.context}
		if _, exists := entities[entityKey]; !exists {
			return nil, storedAdmissionIntegrity("stored alias refers to an absent entity/context", nil)
		}
		changes[change.changeRef] = change
	}
	superseded := make(map[string]struct{}, len(changes))
	for _, change := range changes {
		if change.changeKind != "supersede_alias" {
			continue
		}
		previous, found := changes[change.supersedesRef]
		if !found {
			return nil, storedAdmissionIntegrity("stored alias supersession predecessor is missing", nil)
		}
		if _, exists := superseded[change.supersedesRef]; exists {
			return nil, storedAdmissionIntegrity("stored alias lineage has multiple successors", nil)
		}
		precedes := previous.eventRevision < change.eventRevision ||
			(previous.eventRevision == change.eventRevision && previous.changeOrdinal < change.changeOrdinal)
		if !precedes || previous.entity != change.entity || previous.context != change.context {
			return nil, storedAdmissionIntegrity("stored alias supersession lineage is inconsistent", nil)
		}
		previousAlias := effectiveAlias(previous)
		if previousAlias != change.alias {
			return nil, storedAdmissionIntegrity("stored alias supersession does not continue the prior alias", nil)
		}
		superseded[change.supersedesRef] = struct{}{}
	}
	active := make(map[aliasContextKey]typedmemory.EntityID)
	for ref, change := range changes {
		if _, isSuperseded := superseded[ref]; isSuperseded {
			continue
		}
		alias := effectiveAlias(change)
		key := aliasContextKey{alias: alias, context: change.context}
		if _, exists := active[key]; exists {
			return nil, storedAdmissionIntegrity("stored active alias is ambiguous", nil)
		}
		active[key] = change.entity
	}
	return active, nil
}

func decodeAliasChange(row storedAliasChangeRow) (decodedAliasChange, error) {
	if row.EventRevision <= 0 || row.ChangeOrdinal < 0 || row.ChangeRef == "" {
		return decodedAliasChange{}, storedAdmissionIntegrity("stored alias coordinates are malformed", nil)
	}
	contextRef, err := typedmemory.NewBoundedContextRef(row.ContextRef)
	if err != nil {
		return decodedAliasChange{}, storedAdmissionIntegrity("stored alias context is malformed", err)
	}
	alias, err := typedmemory.NewEntityAlias(row.Alias)
	if err != nil {
		return decodedAliasChange{}, storedAdmissionIntegrity("stored alias is malformed", err)
	}
	entity, err := typedmemory.NewEntityID(row.EntityID)
	if err != nil {
		return decodedAliasChange{}, storedAdmissionIntegrity("stored alias entity is malformed", err)
	}
	change := decodedAliasChange{
		eventRevision: row.EventRevision,
		changeOrdinal: row.ChangeOrdinal,
		changeRef:     row.ChangeRef,
		changeKind:    row.ChangeKind,
		context:       contextRef,
		alias:         alias,
		entity:        entity,
		supersedesRef: row.SupersedesRef,
	}
	if row.ChangeKind == "admit_alias" {
		if row.Replacement != nil || row.SupersedesRef != "" {
			return decodedAliasChange{}, storedAdmissionIntegrity("stored alias admission has supersession fields", nil)
		}
		return change, nil
	}
	if row.ChangeKind != "supersede_alias" || row.Replacement == nil || row.SupersedesRef == "" {
		return decodedAliasChange{}, storedAdmissionIntegrity("stored alias change kind is malformed", nil)
	}
	replacement, err := typedmemory.NewEntityAlias(*row.Replacement)
	if err != nil {
		return decodedAliasChange{}, storedAdmissionIntegrity("stored replacement alias is malformed", err)
	}
	if replacement == alias {
		return decodedAliasChange{}, storedAdmissionIntegrity("stored alias supersession does not change alias", nil)
	}
	change.replacement = &replacement
	return change, nil
}

func effectiveAlias(change decodedAliasChange) typedmemory.EntityAlias {
	if change.replacement == nil {
		return change.alias
	}
	return *change.replacement
}

func loadCurrentAssertionStates(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	head GraphHead,
) (map[typedmemory.AssertionID]storedAssertionState, error) {
	relationRows, err := loadAssertionRows(
		ctx,
		transaction,
		head,
		`typed_memory_relation_instances`,
	)
	if err != nil {
		return nil, err
	}
	relationalAssertionRows, err := loadAssertionRows(
		ctx,
		transaction,
		head,
		`typed_memory_relational_assertions_v3`,
	)
	if err != nil {
		return nil, err
	}
	relationRows = append(relationRows, relationalAssertionRows...)
	retractionRows, err := loadAssertionRows(
		ctx,
		transaction,
		head,
		`typed_memory_assertion_retractions`,
	)
	if err != nil {
		return nil, err
	}
	states := make(map[typedmemory.AssertionID]storedAssertionState, len(relationRows))
	revisions := make(map[typedmemory.AssertionID]int64, len(relationRows))
	for _, row := range relationRows {
		assertion, err := typedmemory.NewAssertionID(row.AssertionID)
		if err != nil {
			return nil, storedAdmissionIntegrity("stored assertion ID is malformed", err)
		}
		if _, exists := states[assertion]; exists {
			return nil, storedAdmissionIntegrity("stored assertion has multiple relation origins", nil)
		}
		states[assertion] = storedAssertionActive
		revisions[assertion] = row.EventRevision
	}
	seenRetractions := make(map[typedmemory.AssertionID]struct{}, len(retractionRows))
	for _, row := range retractionRows {
		assertion, err := typedmemory.NewAssertionID(row.AssertionID)
		if err != nil {
			return nil, storedAdmissionIntegrity("stored retraction assertion ID is malformed", err)
		}
		if _, exists := seenRetractions[assertion]; exists {
			return nil, storedAdmissionIntegrity("stored assertion has multiple retractions", nil)
		}
		originRevision, exists := revisions[assertion]
		if !exists || originRevision >= row.EventRevision {
			return nil, storedAdmissionIntegrity("stored retraction has no prior assertion", nil)
		}
		seenRetractions[assertion] = struct{}{}
		states[assertion] = storedAssertionRetracted
	}
	return states, nil
}

func loadAssertionRows(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	head GraphHead,
	table string,
) ([]storedAssertionRow, error) {
	if table != "typed_memory_relation_instances" &&
		table != "typed_memory_relational_assertions_v3" &&
		table != "typed_memory_assertion_retractions" {
		return nil, fmt.Errorf("unsupported typed-memory assertion table")
	}
	statement := `SELECT COALESCE(json_group_array(json_object(
		'event_revision', graph_revision,
		'event_expected_revision', expected_revision,
		'change_ordinal', change_ordinal,
		'assertion_id', assertion_id,
		'event_basis_type_env', event_basis_type_env,
		'admission_type_env', admission_type_env,
		'admission_basis_revision', admission_basis_revision,
		'writer_generation', writer_generation,
		'writer_provenance', writer_provenance
	)), '[]')
	FROM (
		SELECT event.graph_revision, event.expected_revision,
			projection.change_ordinal, projection.assertion_id,
			event.basis_type_env_ref AS event_basis_type_env,
			basis.type_env_ref AS admission_type_env,
			basis.basis_graph_revision AS admission_basis_revision,
			generation.writer_generation,
			generation.provenance_kind AS writer_provenance
		FROM ` + table + ` projection
		JOIN typed_memory_graph_events event
			ON event.project_id = projection.project_id
			AND event.event_ref = projection.event_ref
		JOIN typed_memory_event_admission_bases basis
			ON basis.project_id = event.project_id
			AND basis.event_ref = event.event_ref
		JOIN typed_memory_event_writer_generations generation
			ON generation.project_id = event.project_id
			AND generation.event_ref = event.event_ref
		JOIN typed_memory_graph_commits commit_row
			ON commit_row.project_id = event.project_id
			AND commit_row.commit_ref = event.commit_ref
		WHERE projection.project_id = ? AND event.graph_revision <= ?
		ORDER BY event.graph_revision, projection.change_ordinal
	)`
	encoded, err := loadJSONAggregate(ctx, transaction, statement, head)
	if err != nil {
		return nil, err
	}
	rows := []storedAssertionRow{}
	if err := json.Unmarshal([]byte(encoded), &rows); err != nil {
		return nil, storedAdmissionIntegrity("decode stored assertion projection", err)
	}
	for _, row := range rows {
		if row.EventRevision <= 0 ||
			row.EventExpectedRevision < 0 ||
			row.ChangeOrdinal < 0 ||
			row.EventBasisTypeEnv == "" ||
			row.EventBasisTypeEnv != row.AdmissionTypeEnv ||
			row.EventExpectedRevision != row.AdmissionBasisRevision {
			return nil, storedAdmissionIntegrity("stored assertion coordinates are malformed", nil)
		}
		if !storedAssertionWriterMatchesTable(table, row) {
			return nil, storedAdmissionIntegrity(
				"stored assertion origin crosses its writer-generation lane",
				nil,
			)
		}
	}
	return rows, nil
}

func storedAssertionWriterMatchesTable(
	table string,
	row storedAssertionRow,
) bool {
	writer46 := row.WriterGeneration == genericStorageWriterGeneration &&
		row.WriterProvenance == "writer_v46"
	writer53 := row.WriterGeneration == relationalAssertionWriterGeneration &&
		row.WriterProvenance == "writer_v53"
	writer54 := row.WriterGeneration == kindClassificationWriterGeneration &&
		row.WriterProvenance == "writer_v54"
	if table == "typed_memory_relation_instances" {
		return writer46
	}
	if table == "typed_memory_relational_assertions_v3" {
		return writer53 || writer54
	}
	return writer46 || writer53 || writer54
}

func loadJSONAggregate(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	statement string,
	head GraphHead,
) (string, error) {
	var encoded string
	graphRevision := head.Revision()
	revision, exact := sqliteIntegerFromUint64(graphRevision.Value())
	if !exact {
		return "", storedAdmissionIntegrity(
			"current projection revision exceeds SQLite INTEGER",
			nil,
		)
	}
	project := head.Project()
	err := transaction.ScanOne(
		ctx,
		statement,
		[]any{project.String(), revision},
		[]any{&encoded},
	)
	if err != nil {
		return "", fmt.Errorf("load current typed-memory projection: %w", err)
	}
	return encoded, nil
}

func memorySnapshotIsPresent(snapshot typedmemory.MemorySnapshot) bool {
	if snapshot == nil {
		return false
	}
	value := reflect.ValueOf(snapshot)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return !value.IsNil()
	default:
		return true
	}
}

func typeEnvLoaderIsPresent(loader TypeEnvLoader) bool {
	if loader == nil {
		return false
	}
	value := reflect.ValueOf(loader)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return !value.IsNil()
	default:
		return true
	}
}
