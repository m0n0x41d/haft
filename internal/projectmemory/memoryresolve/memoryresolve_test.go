package memoryresolve_test

import (
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectmemory/memoryresolve"
	"github.com/m0n0x41d/haft/internal/projectmemory/neighborhood"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestExactIdentifierAndContextResolveBeforeRanking(t *testing.T) {
	fixture := newResolutionFixture(t, "a", 11)
	auth := fixture.unit(
		t,
		"service:auth",
		"context:auth",
		"Authentication service",
		[]string{"auth-service"},
	)
	index := fixture.completeAllIndex(t, []memoryresolve.ResolutionUnit{auth})
	request := fixture.exactContextRequest(
		t,
		"service:auth",
		"context:auth",
		5,
	)

	result, err := memoryresolve.Resolve(request, index)
	if err != nil {
		t.Fatal(err)
	}
	exact, ok := result.(memoryresolve.ExactEntity)
	if !ok {
		t.Fatalf("result = %T, want ExactEntity", result)
	}
	context, ok := exact.ResolutionScope().Context().(memoryresolve.ExactContext)
	if !ok ||
		context.Context().String() != "context:auth" ||
		exact.Entity().Entity() != auth.Entity() ||
		exact.ResolutionWitnesses()[0].Kind() !=
			memoryresolve.WitnessExactIdentifier {
		t.Fatal("exact resolution lost identity, context, or witness")
	}
	if exact.Interpretation().Identity() != neighborhood.IdentityExact ||
		exact.Interpretation().RelationalRecords() !=
			neighborhood.RelationalRecordsUnavailable ||
		exact.Interpretation().Authority() !=
			neighborhood.AuthorityNotGranted {
		t.Fatal("exact identity resolution implied project structure or authority")
	}
}

func TestExactAliasConflictFailsUnsettled(t *testing.T) {
	fixture := newResolutionFixture(t, "b", 12)
	left := fixture.unit(
		t,
		"service:auth-primary",
		"context:auth",
		"Primary authentication service",
		[]string{"auth-service"},
	)
	right := fixture.unit(
		t,
		"service:auth-secondary",
		"context:auth",
		"Secondary authentication service",
		[]string{"auth-service"},
	)
	index := fixture.completeAllIndex(
		t,
		[]memoryresolve.ResolutionUnit{left, right},
	)
	request := fixture.exactContextRequest(
		t,
		"auth-service",
		"context:auth",
		5,
	)

	result, err := memoryresolve.Resolve(request, index)
	if err != nil {
		t.Fatal(err)
	}
	unsettled, ok := result.(memoryresolve.ResolutionUnsettled)
	if !ok {
		t.Fatalf("result = %T, want ResolutionUnsettled", result)
	}
	issues := unsettled.Issues()
	conflict, ok := issues[0].(memoryresolve.AliasConflictIssue)
	if !ok ||
		conflict.Alias().String() != "auth-service" ||
		len(conflict.CandidateEntityRefs()) != 2 {
		t.Fatal("alias conflict was auto-selected or lost its candidate refs")
	}
}

func TestAnyContextKeepsSameIdentityAsSeparateContextCandidates(t *testing.T) {
	fixture := newResolutionFixture(t, "c", 13)
	auth := fixture.unit(
		t,
		"service:identity",
		"context:auth",
		"Identity service in authentication",
		nil,
	)
	admin := fixture.unit(
		t,
		"service:identity",
		"context:admin",
		"Identity service in administration",
		nil,
	)
	index := fixture.completeAllIndex(
		t,
		[]memoryresolve.ResolutionUnit{auth, admin},
	)
	request := fixture.anyContextRequest(t, "service:identity", 5)

	result, err := memoryresolve.Resolve(request, index)
	if err != nil {
		t.Fatal(err)
	}
	candidates, ok := result.(memoryresolve.EntityCandidates)
	if !ok {
		t.Fatalf("result = %T, want EntityCandidates", result)
	}
	if len(candidates.Candidates()) != 2 ||
		candidates.Candidates()[0].Entity().Context() ==
			candidates.Candidates()[1].Entity().Context() {
		t.Fatal("cross-context identity candidates were merged")
	}
	if candidates.Interpretation().RelationalRecords() !=
		neighborhood.RelationalRecordsUnavailable {
		t.Fatal("identity candidates fabricated candidate project relations")
	}
}

func TestRussianLexicalCandidatesAreBudgetedAndScopeBound(t *testing.T) {
	fixture := newResolutionFixture(t, "d", 14)
	primary := fixture.unit(
		t,
		"service:auth",
		"context:auth",
		"Сервис авторизации токенов",
		nil,
	)
	secondary := fixture.unit(
		t,
		"service:external-auth",
		"context:auth",
		"Авторизация токенов внешнего клиента",
		nil,
	)
	index := fixture.completeAllIndex(
		t,
		[]memoryresolve.ResolutionUnit{primary, secondary},
	)
	request := fixture.exactContextRequest(
		t,
		"авторизация токенов",
		"context:auth",
		1,
	)

	result, err := memoryresolve.Resolve(request, index)
	if err != nil {
		t.Fatal(err)
	}
	candidates, ok := result.(memoryresolve.EntityCandidates)
	if !ok {
		t.Fatalf("result = %T, want EntityCandidates", result)
	}
	coverage := candidates.Coverage()
	cursor, hasCursor := coverage.Cursor()
	if len(candidates.Candidates()) != 1 ||
		coverage.InspectedCount() != 2 ||
		coverage.OmittedAtLeast() != 1 ||
		!hasCursor ||
		!cursor.Valid() ||
		cursor.IndexVersion() != index.Version() ||
		cursor.ResolutionScope().Query().Original() !=
			request.Query().Original() {
		t.Fatal("lexical candidate truncation lost exact scope or index basis")
	}
	applied := candidates.AppliedBudget()
	if applied.Requested() != 1 ||
		applied.Included() != 1 ||
		applied.OmittedAtLeast() != 1 {
		t.Fatal("resolution budget disagrees with candidate coverage")
	}
}

func TestNoMatchIsKnownAbsentOnlyUnderCompleteScope(t *testing.T) {
	fixture := newResolutionFixture(t, "e", 15)
	index := fixture.completeAllIndex(t, nil)
	request := fixture.anyContextRequest(t, "missing-service", 5)

	result, err := memoryresolve.Resolve(request, index)
	if err != nil {
		t.Fatal(err)
	}
	absent, ok := result.(memoryresolve.KnownAbsent)
	if !ok {
		t.Fatalf("result = %T, want KnownAbsent", result)
	}
	if absent.ResolutionScope().Query().Original() != "missing-service" ||
		absent.InspectedIndex() != index.Ref() ||
		absent.InspectedIndexVersion() != index.Version() ||
		absent.CompletenessBasis().String() == "" {
		t.Fatal("known absence lost its exact scope or completeness basis")
	}
}

func TestIncompleteAnyContextCannotClaimKnownAbsent(t *testing.T) {
	fixture := newResolutionFixture(t, "f", 16)
	authContext := mustValue(
		typedmemory.NewBoundedContextRef("context:auth"),
	)
	completeness := mustValue(
		memoryresolve.NewCompleteNamedContexts(
			[]typedmemory.BoundedContextRef{authContext},
			fixture.completenessBasis,
		),
	)
	index := fixture.index(t, completeness, nil)
	request := fixture.anyContextRequest(t, "missing-service", 5)

	result, err := memoryresolve.Resolve(request, index)
	if err != nil {
		t.Fatal(err)
	}
	unsettled, ok := result.(memoryresolve.ResolutionUnsettled)
	if !ok {
		t.Fatalf("result = %T, want ResolutionUnsettled", result)
	}
	if unsettled.Issues()[0].Kind() !=
		memoryresolve.IssueIncompleteResolutionIndex {
		t.Fatal("incomplete index fabricated known absence")
	}
}

func TestExactContextFiltersBeforeMatching(t *testing.T) {
	fixture := newResolutionFixture(t, "1", 17)
	billing := fixture.unit(
		t,
		"service:billing",
		"context:billing",
		"Shared gateway",
		[]string{"gateway"},
	)
	authContext := mustValue(
		typedmemory.NewBoundedContextRef("context:auth"),
	)
	completeness := mustValue(
		memoryresolve.NewCompleteNamedContexts(
			[]typedmemory.BoundedContextRef{authContext},
			fixture.completenessBasis,
		),
	)
	index := fixture.index(
		t,
		completeness,
		[]memoryresolve.ResolutionUnit{billing},
	)
	request := fixture.exactContextRequest(
		t,
		"gateway",
		"context:auth",
		5,
	)

	result, err := memoryresolve.Resolve(request, index)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := result.(memoryresolve.KnownAbsent); !ok {
		t.Fatalf("result = %T, cross-context match escaped scope filter", result)
	}
}

func TestSnapshotMismatchReturnsRetryRequired(t *testing.T) {
	fixture := newResolutionFixture(t, "2", 18)
	index := fixture.completeAllIndex(t, nil)
	stale := newResolutionFixture(t, "2", 17)
	request := stale.anyContextRequest(t, "service:auth", 5)

	result, err := memoryresolve.Resolve(request, index)
	if err != nil {
		t.Fatal(err)
	}
	retry, ok := result.(memoryresolve.ResolutionRetryRequired)
	if !ok {
		t.Fatalf("result = %T, want ResolutionRetryRequired", result)
	}
	if retry.ObservedSnapshot() != stale.snapshot ||
		retry.RequiredSnapshot() != fixture.snapshot ||
		retry.RetryOperation() != neighborhood.RetryReloadSnapshot {
		t.Fatal("snapshot mismatch did not retain observed and required bases")
	}
}

type resolutionFixture struct {
	typeEnv           typedmemory.TypeEnvRef
	snapshot          neighborhood.SnapshotBasis
	refKind           typedmemory.RefKindRef
	indexRef          memoryresolve.ResolutionIndexRef
	indexVersion      memoryresolve.ResolutionIndexVersion
	completenessBasis memoryresolve.ResolutionCompletenessBasisRef
}

func newResolutionFixture(
	t *testing.T,
	digestCharacter string,
	revision uint64,
) resolutionFixture {
	t.Helper()
	digest := mustValue(
		typedmemory.NewSHA256Digest(
			"sha256:" + strings.Repeat(digestCharacter, 64),
		),
	)
	typeEnv := mustValue(typedmemory.NewTypeEnvRef(digest))
	snapshot := mustValue(
		neighborhood.NewSnapshotBasis(
			typedmemory.NewGraphRevision(revision),
			typeEnv,
			digest,
		),
	)
	refKindID := mustValue(typedmemory.NewRefKindID("U.System"))
	refKind := mustValue(typedmemory.NewRefKindRef(typeEnv, refKindID))
	return resolutionFixture{
		typeEnv:      typeEnv,
		snapshot:     snapshot,
		refKind:      refKind,
		indexRef:     mustValue(memoryresolve.NewResolutionIndexRef("entities")),
		indexVersion: mustValue(memoryresolve.NewResolutionIndexVersion("v1")),
		completenessBasis: mustValue(
			memoryresolve.NewResolutionCompletenessBasisRef(
				"typed-entity-universe@revision",
			),
		),
	}
}

func (fixture resolutionFixture) unit(
	t *testing.T,
	entityID string,
	contextID string,
	label string,
	aliasValues []string,
) memoryresolve.ResolutionUnit {
	t.Helper()
	referenceID := mustValue(typedmemory.NewReferenceID(entityID))
	entity := mustValue(
		typedmemory.NewPersistedRef(fixture.refKind, referenceID),
	)
	context := mustValue(typedmemory.NewBoundedContextRef(contextID))
	readable := mustValue(neighborhood.NewReadableItemText(label))
	aliases := make([]typedmemory.EntityAlias, 0, len(aliasValues))
	for _, value := range aliasValues {
		aliases = append(
			aliases,
			mustValue(typedmemory.NewEntityAlias(value)),
		)
	}
	provenance := mustValue(
		typedmemory.NewProvenanceRef("entity-index:" + entityID),
	)
	basis := mustValue(
		typedmemory.NewResolutionBasisRef(
			"entity-index:" + entityID + "@" + contextID,
		),
	)
	return mustValue(
		memoryresolve.NewResolutionUnit(
			entity,
			context,
			readable,
			aliases,
			provenance,
			basis,
		),
	)
}

func (fixture resolutionFixture) completeAllIndex(
	t *testing.T,
	units []memoryresolve.ResolutionUnit,
) memoryresolve.ResolutionIndex {
	t.Helper()
	completeness := mustValue(
		memoryresolve.NewCompleteAllContexts(fixture.completenessBasis),
	)
	return fixture.index(t, completeness, units)
}

func (fixture resolutionFixture) index(
	t *testing.T,
	completeness memoryresolve.ResolutionIndexCompleteness,
	units []memoryresolve.ResolutionUnit,
) memoryresolve.ResolutionIndex {
	t.Helper()
	return mustValue(
		memoryresolve.NewResolutionIndex(
			fixture.indexRef,
			fixture.indexVersion,
			fixture.snapshot,
			completeness,
			units,
		),
	)
}

func (fixture resolutionFixture) exactContextRequest(
	t *testing.T,
	queryValue string,
	contextValue string,
	maxCandidates uint32,
) memoryresolve.ResolutionRequest {
	t.Helper()
	query := mustValue(memoryresolve.NewResolutionQuery(queryValue))
	contextRef := mustValue(
		typedmemory.NewBoundedContextRef(contextValue),
	)
	context := mustValue(memoryresolve.NewExactContext(contextRef))
	return mustValue(
		memoryresolve.NewResolutionRequest(
			query,
			context,
			fixture.snapshot,
			maxCandidates,
		),
	)
}

func (fixture resolutionFixture) anyContextRequest(
	t *testing.T,
	queryValue string,
	maxCandidates uint32,
) memoryresolve.ResolutionRequest {
	t.Helper()
	query := mustValue(memoryresolve.NewResolutionQuery(queryValue))
	return mustValue(
		memoryresolve.NewResolutionRequest(
			query,
			memoryresolve.AnyContext{},
			fixture.snapshot,
			maxCandidates,
		),
	)
}

func mustValue[T any](
	value T,
	err error,
) T {
	if err != nil {
		panic(err)
	}
	return value
}
