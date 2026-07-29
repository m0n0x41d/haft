package scopedrecall_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectmemory/neighborhood"
	"github.com/m0n0x41d/haft/internal/projectmemory/scopedrecall"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestLexicalRecallScopesBeforeRankingAcrossRussianText(t *testing.T) {
	typeEnv := recallTestTypeEnv(t, "a")
	left := recallTestNeighborhood(
		t,
		typeEnv,
		"service:auth",
		"context:auth",
		11,
		[]string{"Авторизация токена"},
	)
	right := recallTestNeighborhood(
		t,
		typeEnv,
		"service:billing",
		"context:billing",
		11,
		[]string{
			"Авторизация токена авторизация токена авторизация токена",
		},
	)
	leftUnits, err := scopedrecall.BuildRecallUnits(left)
	if err != nil {
		t.Fatal(err)
	}
	rightUnits, err := scopedrecall.BuildRecallUnits(right)
	if err != nil {
		t.Fatal(err)
	}
	scope, err := scopedrecall.NewExactRecallScope(
		left.ViewContext().Entity(),
		left.ViewContext().Context(),
		left.ViewContext().ProfileRef(),
	)
	if err != nil {
		t.Fatal(err)
	}
	allUnits := append(leftUnits, rightUnits...)
	corpus, err := scopedrecall.NewScopedCorpus(
		scope,
		left.SnapshotBasis(),
		allUnits,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(corpus.Units()) != 1 {
		t.Fatal("cross-context candidate survived exact-scope filtering")
	}
	request := recallTestRequest(
		t,
		scope,
		left.SnapshotBasis(),
		"авторизация токена",
		10,
	)
	result, err := scopedrecall.NewLexicalProducer().Search(request, corpus)
	if err != nil {
		t.Fatal(err)
	}
	candidates, ok := result.(scopedrecall.ScopedMemoryCandidateSet)
	if !ok {
		t.Fatalf("result = %T, want ScopedMemoryCandidateSet", result)
	}
	if len(candidates.Candidates()) != 1 ||
		candidates.Candidates()[0].Unit().Scope() != scope {
		t.Fatal("ranking escaped exact EntityOfConcern/context scope")
	}
	if candidates.Interpretation().RelationalRecords() !=
		neighborhood.RelationalRecordsCandidateAssertions ||
		candidates.Interpretation().Authority() !=
			neighborhood.AuthorityNotGranted {
		t.Fatal("lexical relevance was promoted to relation or authority")
	}
}

func TestRecallUnitAndCandidateCursorAreSnapshotBoundAndDeterministic(t *testing.T) {
	typeEnv := recallTestTypeEnv(t, "b")
	exact := recallTestNeighborhood(
		t,
		typeEnv,
		"service:auth",
		"context:auth",
		20,
		[]string{
			"Auth token renewal",
			"Auth token validation",
			"Auth token revocation",
		},
	)
	left, err := scopedrecall.BuildRecallUnits(exact)
	if err != nil {
		t.Fatal(err)
	}
	right, err := scopedrecall.BuildRecallUnits(exact)
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 3 || len(right) != 3 {
		t.Fatal("RecallUnit projection lost exact items")
	}
	for index := range left {
		if left[index].ID() != right[index].ID() ||
			left[index].ContentDigest() != right[index].ContentDigest() {
			t.Fatal("fixed exact neighborhood changed RecallUnit identity")
		}
	}
	scope, err := scopedrecall.NewExactRecallScope(
		exact.ViewContext().Entity(),
		exact.ViewContext().Context(),
		exact.ViewContext().ProfileRef(),
	)
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := scopedrecall.NewScopedCorpus(
		scope,
		exact.SnapshotBasis(),
		left,
	)
	if err != nil {
		t.Fatal(err)
	}
	request := recallTestRequest(
		t,
		scope,
		exact.SnapshotBasis(),
		"auth token",
		2,
	)
	result, err := scopedrecall.NewLexicalProducer().Search(request, corpus)
	if err != nil {
		t.Fatal(err)
	}
	candidates, ok := result.(scopedrecall.ScopedMemoryCandidateSet)
	if !ok {
		t.Fatalf("result = %T, want candidate set", result)
	}
	coverage, ok := candidates.ProducerCoverage()[0].(scopedrecall.PartialProducerCoverage)
	if !ok ||
		coverage.InspectedCount() != 3 ||
		coverage.OmittedAtLeast() != 1 ||
		!coverage.Cursor().Valid() {
		t.Fatal("candidate truncation lost exact producer/cursor coverage")
	}
	if candidates.AppliedBudget().Included() != 2 ||
		candidates.AppliedBudget().OmittedAtLeast() != 1 {
		t.Fatal("candidate applied budget disagrees with coverage")
	}
}

func TestStaleScopedCorpusReturnsTypedBasisBeforeSearch(t *testing.T) {
	typeEnv := recallTestTypeEnv(t, "c")
	current := recallTestNeighborhood(
		t,
		typeEnv,
		"service:auth",
		"context:auth",
		31,
		[]string{"Auth token"},
	)
	stale := recallTestNeighborhood(
		t,
		typeEnv,
		"service:auth",
		"context:auth",
		30,
		[]string{"Auth token"},
	)
	currentUnits, err := scopedrecall.BuildRecallUnits(current)
	if err != nil {
		t.Fatal(err)
	}
	staleUnits, err := scopedrecall.BuildRecallUnits(stale)
	if err != nil {
		t.Fatal(err)
	}
	scope, err := scopedrecall.NewExactRecallScope(
		current.ViewContext().Entity(),
		current.ViewContext().Context(),
		current.ViewContext().ProfileRef(),
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = scopedrecall.NewScopedCorpus(
		scope,
		current.SnapshotBasis(),
		append(currentUnits, staleUnits...),
	)
	if _, ok := err.(scopedrecall.StaleCorpusBasis); !ok {
		t.Fatalf("stale corpus error = %T, want StaleCorpusBasis", err)
	}
}

func TestNoLexicalMatchAbstainsWithoutFabricatingCandidate(t *testing.T) {
	typeEnv := recallTestTypeEnv(t, "d")
	exact := recallTestNeighborhood(
		t,
		typeEnv,
		"service:auth",
		"context:auth",
		41,
		[]string{"Auth token renewal"},
	)
	units, err := scopedrecall.BuildRecallUnits(exact)
	if err != nil {
		t.Fatal(err)
	}
	scope, err := scopedrecall.NewExactRecallScope(
		exact.ViewContext().Entity(),
		exact.ViewContext().Context(),
		exact.ViewContext().ProfileRef(),
	)
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := scopedrecall.NewScopedCorpus(
		scope,
		exact.SnapshotBasis(),
		units,
	)
	if err != nil {
		t.Fatal(err)
	}
	request := recallTestRequest(
		t,
		scope,
		exact.SnapshotBasis(),
		"квантовая гравитация",
		10,
	)
	result, err := scopedrecall.NewLexicalProducer().Search(request, corpus)
	if err != nil {
		t.Fatal(err)
	}
	abstained, ok := result.(scopedrecall.ScopedRecallAbstained)
	if !ok {
		t.Fatalf("result = %T, want abstention", result)
	}
	if abstained.Basis().Kind() !=
		scopedrecall.AbstentionNoMatchingMemory ||
		abstained.Interpretation().Structure() !=
			neighborhood.StructureUnavailable {
		t.Fatal("empty lexical result fabricated a usable candidate set")
	}
}

func TestProducerCoverageAndNonCandidateResultsKeepClosedVariantsDistinct(
	t *testing.T,
) {
	observedTypeEnv := recallTestTypeEnv(t, "e")
	exact := recallTestNeighborhood(
		t,
		observedTypeEnv,
		"service:auth",
		"context:auth",
		51,
		[]string{"Auth token renewal"},
	)
	scope, err := scopedrecall.NewExactRecallScope(
		exact.ViewContext().Entity(),
		exact.ViewContext().Context(),
		exact.ViewContext().ProfileRef(),
	)
	if err != nil {
		t.Fatal(err)
	}
	request := recallTestRequest(
		t,
		scope,
		exact.SnapshotBasis(),
		"auth token",
		2,
	)
	producer, err := scopedrecall.NewProducerRef("producer:lexical:v1")
	if err != nil {
		t.Fatal(err)
	}
	complete, err := scopedrecall.NewCompleteProducerCoverage(producer, 1)
	if err != nil {
		t.Fatal(err)
	}
	cursor, err := scopedrecall.NewRecallCursor(
		scope,
		exact.SnapshotBasis(),
		producer,
		request.Query(),
		1,
	)
	if err != nil {
		t.Fatal(err)
	}
	partial, err := scopedrecall.NewPartialProducerCoverage(
		producer,
		2,
		1,
		cursor,
	)
	if err != nil {
		t.Fatal(err)
	}
	missing, err := neighborhood.NewMissingBasisRef(
		"producer:lexical:index-unavailable",
	)
	if err != nil {
		t.Fatal(err)
	}
	unavailable, err := scopedrecall.NewUnavailableProducerCoverage(
		producer,
		missing,
	)
	if err != nil {
		t.Fatal(err)
	}
	coverageKinds := []scopedrecall.ProducerCoverageKind{
		complete.Kind(),
		partial.Kind(),
		unavailable.Kind(),
	}
	if coverageKinds[0] != scopedrecall.ProducerCoverageComplete ||
		coverageKinds[1] != scopedrecall.ProducerCoveragePartial ||
		coverageKinds[2] != scopedrecall.ProducerCoverageUnavailable {
		t.Fatalf("producer coverage variants collapsed: %#v", coverageKinds)
	}

	noProducer, err := scopedrecall.NewNoUsableProducerBasis(
		[]scopedrecall.ProducerRef{producer},
		missing,
	)
	if err != nil {
		t.Fatal(err)
	}
	abstained, err := scopedrecall.NewScopedRecallAbstained(
		request,
		[]scopedrecall.ProducerRef{producer},
		noProducer,
	)
	if err != nil {
		t.Fatal(err)
	}
	if abstained.Kind() != scopedrecall.ScopedResultAbstained ||
		abstained.Basis().Kind() != scopedrecall.AbstentionNoUsableProducer ||
		abstained.Interpretation().Authority() !=
			neighborhood.AuthorityNotGranted {
		t.Fatal("unavailable producer was promoted to a candidate or authority")
	}

	requiredTypeEnv := recallTestTypeEnv(t, "f")
	required, err := neighborhood.NewSnapshotBasis(
		typedmemory.NewGraphRevision(52),
		requiredTypeEnv,
		requiredTypeEnv.Digest(),
	)
	if err != nil {
		t.Fatal(err)
	}
	cause, err := neighborhood.NewStaleSnapshotCause(
		exact.SnapshotBasis(),
		required,
	)
	if err != nil {
		t.Fatal(err)
	}
	retry, err := neighborhood.NewRetryRequiredResult(cause, required)
	if err != nil {
		t.Fatal(err)
	}
	scopedRetry, err := scopedrecall.NewScopedRetryRequired(request, retry)
	if err != nil {
		t.Fatal(err)
	}
	if scopedRetry.Kind() != scopedrecall.ScopedResultRetryRequired ||
		scopedRetry.RetryOperation() != neighborhood.RetryReloadSnapshot ||
		scopedRetry.Interpretation().WorkOrder() !=
			neighborhood.WorkOrderNotImplied {
		t.Fatal("stale scoped recall did not remain a non-authorizing retry")
	}
}

func recallTestNeighborhood(
	t *testing.T,
	typeEnv typedmemory.TypeEnvRef,
	entityID string,
	contextID string,
	revision uint64,
	texts []string,
) neighborhood.ExactNeighborhood {
	t.Helper()
	profileRef, err := neighborhood.ParseProjectionProfileRef(
		"agent_orientation.v1",
	)
	if err != nil {
		t.Fatal(err)
	}
	view, err := neighborhood.NewNeighborhoodViewSpec(
		profileRef,
		[]neighborhood.FacetKind{neighborhood.FacetProblems},
		neighborhood.DetailStandard,
		true,
	)
	if err != nil {
		t.Fatal(err)
	}
	context, err := typedmemory.NewBoundedContextRef(contextID)
	if err != nil {
		t.Fatal(err)
	}
	rootRef := recallTestRef(t, typeEnv, entityID)
	budget, err := neighborhood.NewReadBudgetBuilder().
		SetMaxFacets(1).
		SetMaxItemsPerFacet(100).
		SetMaxRelationPathsPerItem(10).
		SetMaxCarrierExcerptCharacters(1000).
		SetMaxProvenanceDepth(3).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	request, err := neighborhood.NewNeighborhoodRequestBuilder().
		SetEntity(rootRef).
		SetContext(context).
		SetTypeEnv(typeEnv).
		SetGraphRevision(typedmemory.NewGraphRevision(revision)).
		SetView(view).
		SetBudget(budget).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := neighborhood.NewSnapshotBasis(
		typedmemory.NewGraphRevision(revision),
		typeEnv,
		typeEnv.Digest(),
	)
	if err != nil {
		t.Fatal(err)
	}
	inputRef, err := neighborhood.NewProjectionInputRef(
		fmt.Sprintf("canonical:fixture:%s:%d", entityID, revision),
	)
	if err != nil {
		t.Fatal(err)
	}
	inputDigest := recallTestDigest(t, fmt.Sprintf("%x", revision%16))
	canonicalInput, err := neighborhood.NewCanonicalInputCoordinate(
		inputRef,
		inputDigest,
	)
	if err != nil {
		t.Fatal(err)
	}
	itemInput, err := neighborhood.NewProjectionInputCoordinate(
		inputRef,
		inputDigest,
	)
	if err != nil {
		t.Fatal(err)
	}
	postures := recallTestPostures(t)
	rootCoordinate, err := neighborhood.NewRootOutputCoordinate(rootRef)
	if err != nil {
		t.Fatal(err)
	}
	rootText, err := neighborhood.NewReadableItemText(
		"Entity " + entityID,
	)
	if err != nil {
		t.Fatal(err)
	}
	rootProvenance, err := typedmemory.NewProvenanceRef(
		"event:root:" + entityID,
	)
	if err != nil {
		t.Fatal(err)
	}
	root, err := neighborhood.NewProjectedRoot(
		rootCoordinate,
		rootText,
		postures,
		rootProvenance,
	)
	if err != nil {
		t.Fatal(err)
	}
	rootBasis, err := neighborhood.NewDirectProjectionItemBasis(
		rootCoordinate,
		[]neighborhood.ProjectionInputCoordinate{itemInput},
		neighborhood.TransformFieldSelection,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	rootSource, err := neighborhood.NewRootProjectionSource(root, rootBasis)
	if err != nil {
		t.Fatal(err)
	}
	items := make([]neighborhood.ItemProjectionSource, 0, len(texts))
	for index, text := range texts {
		items = append(
			items,
			recallTestItem(
				t,
				typeEnv,
				context,
				itemInput,
				entityID,
				index,
				text,
				postures,
			),
		)
	}
	facet, err := neighborhood.NewExactFacetInput(
		neighborhood.FacetProblems,
		itemInput,
		items,
	)
	if err != nil {
		t.Fatal(err)
	}
	pinned, err := neighborhood.NewPinnedNeighborhoodInputBuilder().
		SetRequest(request).
		SetSnapshot(snapshot).
		SetRoot(rootSource).
		AddCanonicalInput(canonicalInput).
		AddFacet(facet).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	result, err := neighborhood.Assemble(pinned)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func recallTestItem(
	t *testing.T,
	typeEnv typedmemory.TypeEnvRef,
	context typedmemory.BoundedContextRef,
	input neighborhood.ProjectionInputCoordinate,
	entityID string,
	index int,
	textValue string,
	postures neighborhood.ItemPostures,
) neighborhood.ItemProjectionSource {
	t.Helper()
	reference := recallTestRef(
		t,
		typeEnv,
		fmt.Sprintf("%s:problem:%d", entityID, index),
	)
	coordinate, err := neighborhood.NewFacetOutputCoordinate(
		neighborhood.FacetProblems,
		reference,
	)
	if err != nil {
		t.Fatal(err)
	}
	text, err := neighborhood.NewReadableItemText(textValue)
	if err != nil {
		t.Fatal(err)
	}
	provenance, err := typedmemory.NewProvenanceRef(
		fmt.Sprintf("event:%s:%d", entityID, index),
	)
	if err != nil {
		t.Fatal(err)
	}
	assertion, err := typedmemory.NewAssertionID(
		fmt.Sprintf("assertion:%s:%d", entityID, index),
	)
	if err != nil {
		t.Fatal(err)
	}
	signature, err := typedmemory.NewSignatureID("Haft.RecordAtConcern")
	if err != nil {
		t.Fatal(err)
	}
	slot, err := typedmemory.NewSlotKindID("Haft.RecordSlot")
	if err != nil {
		t.Fatal(err)
	}
	witness, err := neighborhood.NewRelationPathWitness(
		assertion,
		signature,
		context,
		slot,
		reference,
		provenance,
		fmt.Sprintf("admission:%s:%d", entityID, index),
	)
	if err != nil {
		t.Fatal(err)
	}
	item, err := neighborhood.NewNeighborhoodItem(
		coordinate,
		neighborhood.ItemProblemCard,
		text,
		postures,
		provenance,
		[]neighborhood.RelationPathWitness{witness},
	)
	if err != nil {
		t.Fatal(err)
	}
	basis, err := neighborhood.NewDirectProjectionItemBasis(
		coordinate,
		[]neighborhood.ProjectionInputCoordinate{input},
		neighborhood.TransformFieldSelection,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	source, err := neighborhood.NewItemProjectionSource(item, basis)
	if err != nil {
		t.Fatal(err)
	}
	return source
}

func recallTestRequest(
	t *testing.T,
	scope scopedrecall.ExactRecallScope,
	snapshot neighborhood.SnapshotBasis,
	queryValue string,
	limit uint32,
) scopedrecall.ScopedRecallRequest {
	t.Helper()
	query, err := scopedrecall.NewRecallQuery(queryValue)
	if err != nil {
		t.Fatal(err)
	}
	budget, err := scopedrecall.NewCandidateBudget(limit)
	if err != nil {
		t.Fatal(err)
	}
	request, err := scopedrecall.NewScopedRecallRequest(
		scope,
		snapshot,
		query,
		budget,
	)
	if err != nil {
		t.Fatal(err)
	}
	return request
}

func recallTestPostures(t *testing.T) neighborhood.ItemPostures {
	t.Helper()
	postures, valid := neighborhood.NewItemPostures(
		neighborhood.SemanticTypedActive,
		neighborhood.LifecycleActive,
		neighborhood.EvidenceUnknown,
		neighborhood.ProjectionCurrent,
	)
	if !valid {
		t.Fatal("recall test postures are invalid")
	}
	return postures
}

func recallTestTypeEnv(
	t *testing.T,
	fill string,
) typedmemory.TypeEnvRef {
	t.Helper()
	digest := recallTestDigest(t, fill)
	ref, err := typedmemory.NewTypeEnvRef(digest)
	if err != nil {
		t.Fatal(err)
	}
	return ref
}

func recallTestDigest(
	t *testing.T,
	fill string,
) typedmemory.SHA256Digest {
	t.Helper()
	value := ""
	for len(value) < 64 {
		value += fill
	}
	digest, err := typedmemory.NewSHA256Digest(
		"sha256:" + strings.ToLower(value[:64]),
	)
	if err != nil {
		t.Fatal(err)
	}
	return digest
}

func recallTestRef(
	t *testing.T,
	typeEnv typedmemory.TypeEnvRef,
	raw string,
) typedmemory.PersistedRef {
	t.Helper()
	kindID, err := typedmemory.NewRefKindID("U.EntityRef")
	if err != nil {
		t.Fatal(err)
	}
	kind, err := typedmemory.NewRefKindRef(typeEnv, kindID)
	if err != nil {
		t.Fatal(err)
	}
	id, err := typedmemory.NewReferenceID(raw)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := typedmemory.NewPersistedRef(kind, id)
	if err != nil {
		t.Fatal(err)
	}
	return ref
}
