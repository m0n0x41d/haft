package typedmemorystore

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestDisjointEntailmentWriteProjectsDedicatedUseWithoutCounterEvaluation(
	t *testing.T,
) {
	fixture := newDisjointEntailmentWriteFixture(t)
	builder := semanticMaterializationBuilder{
		request:  fixture.request,
		identity: genericEventIdentity{eventRef: "event:test-disjoint-entailment"},
	}

	err := builder.appendDisjointEntailmentUse(
		0,
		fixture.admissionUse,
		fixture.entailment,
		relationalAssertionStorageFamily,
	)
	if err != nil {
		t.Fatalf("appendDisjointEntailmentUse: %v", err)
	}
	if len(builder.statements) != 1 {
		t.Fatalf("statements = %d; want one dedicated entailment-use insert", len(builder.statements))
	}
	if builder.footprint.memberOfUseCount != 1 {
		t.Fatalf("MemberOf-use footprint = %d; want 1", builder.footprint.memberOfUseCount)
	}
	if builder.footprint.memberOfEvaluationCount != 0 {
		t.Fatalf("counter evaluations = %d; want 0", builder.footprint.memberOfEvaluationCount)
	}

	statement := builder.statements[0]
	if !strings.Contains(
		statement.query,
		"INSERT INTO typed_memory_relational_assertion_disjointness_uses_v3",
	) {
		t.Fatalf("write target = %q; want dedicated disjoint-entailment table", statement.query)
	}
	if len(statement.arguments) != 18 {
		t.Fatalf("insert arguments = %d; want 18 exact columns", len(statement.arguments))
	}
	coordinate := fixture.admissionUse.Coordinate()
	constraint := fixture.entailment.ConstraintRule()
	counter := fixture.entailment.CounterQuery()
	required := fixture.admissionUse.RequiredMembership()
	want := []any{
		fixture.request.project.String(),
		"event:test-disjoint-entailment",
		int64(coordinate.ChangeOrdinal()),
		coordinate.Assertion().String(),
		int64(0),
		int64(coordinate.FillerOrdinal()),
		coordinate.FillerDigest().String(),
		constraint.ID().String(),
		fixture.entailment.ConstraintDigest().String(),
		constraint.CanonicalBytes(),
		fixture.entailment.MatchedOperand().String(),
		fixture.entailment.ExcludedOperand().String(),
		counter.ValueKind().String(),
		counter.Digest().String(),
		counter.CanonicalBytes(),
		derivedRef("typed-memory-memberof-evaluation", required.Digest().String()),
		fixture.entailment.Digest().String(),
		fixture.entailment.CanonicalBytes(),
	}
	assertExactStatementArguments(t, statement.arguments, want)
	if len(builder.rowDigests) != 1 ||
		builder.rowDigests[0] != relationalAssertionStorageFamily.disjointnessDigestTag+
			fixture.entailment.Digest().String() {
		t.Fatalf("row digests = %#v; want exact disjoint-entailment-use identity", builder.rowDigests)
	}
}

func TestExpectedManifestProjectsEntailmentAsSemanticRowOnly(t *testing.T) {
	fixture := newDisjointEntailmentWriteFixture(t)
	direct, err := buildExpectedMaterializationManifest(fixture.directPrepared)
	if err != nil {
		t.Fatalf("build direct expected manifest: %v", err)
	}
	manifest, err := buildExpectedMaterializationManifest(fixture.entailedPrepared)
	if err != nil {
		t.Fatalf("build entailed expected manifest: %v", err)
	}

	if len(manifest.evaluations) != 1 {
		t.Fatalf("evaluations = %d; want only the required positive evaluation", len(manifest.evaluations))
	}
	if len(manifest.memberUses) != 1 {
		t.Fatalf("legacy MemberOf uses = %d; want only required_member", len(manifest.memberUses))
	}
	rows := semanticRowsByKind(
		manifest,
		relationalAssertionStorageFamily.disjointnessUseRowKind,
	)
	if len(rows) != 1 {
		t.Fatalf("disjoint-entailment semantic rows = %d; want 1", len(rows))
	}
	row := rows[0]
	coordinate := fixture.admissionUse.Coordinate()
	constraint := fixture.entailment.ConstraintRule()
	counter := fixture.entailment.CounterQuery()
	supporting := fixture.entailment.SupportingMembership()
	wantCoordinate := []string{
		"1",
		coordinate.Assertion().String(),
		"0",
		"0",
		coordinate.FillerDigest().String(),
		constraint.ID().String(),
		fixture.entailment.ConstraintDigest().String(),
		string(constraint.CanonicalBytes()),
		fixture.entailment.MatchedOperand().String(),
		fixture.entailment.ExcludedOperand().String(),
		counter.ValueKind().String(),
		counter.Digest().String(),
		string(counter.CanonicalBytes()),
		derivedRef(
			"typed-memory-memberof-evaluation",
			supporting.Digest().String(),
		),
	}
	if !equalStrings(row.coordinate, wantCoordinate) {
		t.Fatalf("entailment coordinate = %#v; want %#v", row.coordinate, wantCoordinate)
	}
	if row.semanticDigest != fixture.entailment.Digest() ||
		!bytes.Equal(row.semanticBytes, fixture.entailment.CanonicalBytes()) {
		t.Fatal("entailment semantic row lost the exact use carrier")
	}
	if row.conditional {
		t.Fatal("entailed use was projected as a conditional row")
	}

	decoded, err := decodeExpectedMaterializationManifest(
		manifest.CanonicalBytes(),
		manifest.Digest(),
		manifest.basisRevision,
	)
	if err != nil {
		t.Fatalf("decode entailed expected manifest: %v", err)
	}
	if !sameExpectedMaterializationManifest(manifest, decoded) {
		t.Fatal("entailed expected manifest changed on strict round trip")
	}
	rebuiltDirect, err := buildExpectedMaterializationManifest(fixture.directPrepared)
	if err != nil {
		t.Fatalf("rebuild direct expected manifest: %v", err)
	}
	if !sameExpectedMaterializationManifest(direct, rebuiltDirect) {
		t.Fatal("entailed projection changed the direct-only manifest identity")
	}
}

type disjointEntailmentWriteFixture struct {
	base             exactBasisStoreFixture
	environment      typedmemory.TypeEnv
	request          CommitRequest
	directPrepared   preparedAdmission
	entailedPrepared preparedAdmission
	admissionUse     typedmemory.ReferenceFillerAdmissionUse
	entailment       typedmemory.DisjointEntailmentUse
}

func newDisjointEntailmentWriteFixture(
	t *testing.T,
) disjointEntailmentWriteFixture {
	t.Helper()
	base := newExactBasisStoreFixture(t)
	request := base.request(t, "write-disjoint-entailment")
	return newDisjointEntailmentWriteFixtureFromRequest(t, base, request)
}

func newLegacyDisjointEntailmentWriteFixture(
	t *testing.T,
) disjointEntailmentWriteFixture {
	t.Helper()
	base := newExactBasisStoreFixture(t)
	request := base.legacyRequest(t, "write-legacy-disjoint-entailment")
	return newDisjointEntailmentWriteFixtureFromRequest(t, base, request)
}

func newDisjointEntailmentWriteFixtureFromRequest(
	t *testing.T,
	base exactBasisStoreFixture,
	request CommitRequest,
) disjointEntailmentWriteFixture {
	t.Helper()
	directPrepared, err := prepareGenericAdmission(request)
	if err != nil {
		t.Fatalf("prepare direct exact-basis admission: %v", err)
	}
	membership, ok := directPrepared.basis.(typedmemory.ContextSliceMembershipBasis)
	if !ok {
		t.Fatalf("direct basis = %T; want ContextSliceMembershipBasis", directPrepared.basis)
	}
	uses := membership.ReferenceFillerAdmissionUses()
	if len(uses) != 1 {
		t.Fatalf("direct reference uses = %d; want 1", len(uses))
	}
	environment, constraint := exactBasisDisjointEnvironment(t, base.environment)
	required := uses[0].RequiredMembership()
	entailment, err := typedmemory.NewDisjointEntailmentUse(
		typedmemory.DisjointEntailmentUseInput{
			TypeEnv:              environment,
			Constraint:           constraint,
			SupportingMembership: required,
			MatchedOperand:       base.entityKind.ID(),
			ExcludedOperand:      exactBasisExcludedKind(t),
		},
	)
	if err != nil {
		t.Fatalf("NewDisjointEntailmentUse: %v", err)
	}
	admissionUse, err := typedmemory.NewReferenceFillerAdmissionUse(
		typedmemory.ReferenceFillerAdmissionUseInput{
			TypeEnv:             environment,
			Coordinate:          uses[0].Coordinate(),
			Resolution:          uses[0].Resolution(),
			RequiredMembership:  required,
			DisjointMemberships: []typedmemory.DisjointCounterUse{entailment},
		},
	)
	if err != nil {
		t.Fatalf("NewReferenceFillerAdmissionUse(entailed): %v", err)
	}
	entailedBasis, err := typedmemory.NewContextSliceMembershipBasis(
		typedmemory.ContextSliceMembershipBasisInput{
			TypeEnv:                      membership.TypeEnv(),
			GraphRevision:                membership.GraphRevision(),
			Observations:                 membership.SnapshotObservations(),
			ReferenceFillerAdmissionUses: []typedmemory.ReferenceFillerAdmissionUse{admissionUse},
		},
	)
	if err != nil {
		t.Fatalf("NewContextSliceMembershipBasis(entailing): %v", err)
	}
	entailedPrepared := directPrepared
	entailedPrepared.basis = entailedBasis
	return disjointEntailmentWriteFixture{
		base:             base,
		environment:      environment,
		request:          request,
		directPrepared:   directPrepared,
		entailedPrepared: entailedPrepared,
		admissionUse:     admissionUse,
		entailment:       entailment,
	}
}

func commitDisjointEntailmentFixture(
	t *testing.T,
	key string,
) (disjointEntailmentWriteFixture, *SQLiteAdapter, CommitReceipt) {
	t.Helper()
	fixture := newDisjointEntailmentWriteFixture(t)
	membership, ok := fixture.directPrepared.basis.(typedmemory.ContextSliceMembershipBasis)
	if !ok {
		t.Fatalf(
			"direct basis = %T; want ContextSliceMembershipBasis",
			fixture.directPrepared.basis,
		)
	}
	resolutions := make([]typedmemory.StrongReferenceResolution, 0)
	judgements := make([]typedmemory.MemberOfJudgement, 0)
	for _, use := range membership.ReferenceFillerAdmissionUses() {
		judgements = append(judgements, use.RequiredMembership())
	}
	counterQuery := fixture.entailment.CounterQuery()
	missing, err := typedmemory.NoApplicableObservableSourceForMemberOf(
		counterQuery,
	)
	if err != nil {
		t.Fatalf("NoApplicableObservableSourceForMemberOf: %v", err)
	}
	repair, err := typedmemory.NewRepairPointer(
		"repair:test/disjoint-entailment-no-applicable-counter-source",
	)
	if err != nil {
		t.Fatalf("NewRepairPointer: %v", err)
	}
	counterRequest, err := typedmemory.NewMemberOfEvaluationRequest(
		counterQuery,
		fixture.entailment.EvaluationView(),
	)
	if err != nil {
		t.Fatalf("NewMemberOfEvaluationRequest: %v", err)
	}
	undefined, err := typedmemory.NewMemberOfUndefined(
		counterRequest,
		[]typedmemory.MemberOfMissingBasis{missing},
		repair,
	)
	if err != nil {
		t.Fatalf("NewMemberOfUndefined: %v", err)
	}
	observations := membership.SnapshotObservations()
	baseSnapshot, err := newTransactionAdmissionSnapshotWithClassifications(
		fixture.directPrepared.basis,
		observations,
		resolutions,
		judgements,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("newTransactionAdmissionSnapshot: %v", err)
	}
	snapshot := disjointEntailmentValidationSnapshot{
		transactionAdmissionSnapshot: baseSnapshot,
		undefinedCounter:             undefined,
	}
	registry := typedmemory.NewCodecRegistry()
	verdict := typedmemory.ValidateMemoryChangeSet(
		fixture.environment,
		registry,
		snapshot,
		fixture.request.candidate,
	)
	valid, ok := verdict.(typedmemory.Valid)
	if !ok {
		t.Fatalf(
			"ValidateMemoryChangeSet = %T (%s); want Valid",
			verdict,
			verdict.Kind(),
		)
	}
	request := fixture.request
	request.idempotencyKey, err = NewIdempotencyKey(key)
	if err != nil {
		t.Fatalf("NewIdempotencyKey: %v", err)
	}
	request.admissionBatch = valid.AdmissionBatch()
	blobCount := len(fixture.base.observableBlobs)
	blobs := make(map[string]ObservableInputBlob, blobCount)
	for _, blob := range fixture.base.observableBlobs {
		reference := blob.Reference()
		blobs[reference.String()] = blob
	}
	loader := staticTypeEnvLoader{
		reference:   fixture.environment.Ref(),
		environment: fixture.environment,
		registry:    registry,
	}
	engine := exactBasisMemberOfEngine{
		expectedProject: fixture.base.base.project,
		kindSignature:   fixture.base.kindSignature,
		entitySet:       fixture.base.entitySet,
		provenance:      fixture.base.evaluationSource,
	}
	provider := exactBasisObservableProvider{blobs: blobs}
	adapter, err := NewGenericSQLiteAdapter(
		fixture.base.base.database,
		loader,
		fixture.base.base.clock,
		engine,
		unexpectedReferenceEngine{},
		provider,
	)
	if err != nil {
		t.Fatalf("NewGenericSQLiteAdapter: %v", err)
	}
	receipt, err := adapter.CommitMemoryChangeSet(
		context.Background(),
		request,
	)
	if err != nil {
		t.Fatalf("CommitMemoryChangeSet: %v", err)
	}
	return fixture, adapter, receipt
}

type disjointEntailmentValidationSnapshot struct {
	transactionAdmissionSnapshot
	undefinedCounter typedmemory.MemberOfUndefined
}

func (snapshot disjointEntailmentValidationSnapshot) EvaluateMemberOf(
	request typedmemory.MemberOfEvaluationRequest,
) typedmemory.MemberOfJudgement {
	if request.Query().Digest() == snapshot.undefinedCounter.Query().Digest() {
		return snapshot.undefinedCounter
	}
	return snapshot.transactionAdmissionSnapshot.EvaluateMemberOf(request)
}

func exactBasisDisjointEnvironment(
	t *testing.T,
	source typedmemory.TypeEnv,
) (typedmemory.TypeEnv, typedmemory.KindDisjointConstraint) {
	t.Helper()
	entity, exists := source.KindDefinition(mustGenericKindID(t, "U.Entity"))
	if !exists {
		t.Fatal("exact-basis TypeEnv lacks U.Entity")
	}
	excluded, err := typedmemory.NewKindDefinition(
		exactBasisExcludedKind(t),
		entity.Provenance(),
	)
	if err != nil {
		t.Fatalf("NewKindDefinition(excluded): %v", err)
	}
	constraintID, err := typedmemory.NewConstraintID(
		"constraint:exact-basis.entity-episteme-disjoint",
	)
	if err != nil {
		t.Fatalf("NewConstraintID: %v", err)
	}
	constraint, err := typedmemory.NewKindDisjointConstraint(
		constraintID,
		[]typedmemory.KindID{entity.ID(), excluded.ID()},
		entity.Provenance(),
	)
	if err != nil {
		t.Fatalf("NewKindDisjointConstraint: %v", err)
	}
	builder := cloneExactTypeEnvBuilder(source)
	builder.AddKindDefinition(excluded)
	builder.AddConstraint(constraint)
	environment, err := builder.Build()
	if err != nil {
		t.Fatalf("build disjoint exact-basis TypeEnv: %v", err)
	}
	return environment, constraint
}

func cloneExactTypeEnvBuilder(source typedmemory.TypeEnv) *typedmemory.TypeEnvBuilder {
	builder := typedmemory.NewTypeEnvBuilder(source.Ref())
	builder.SetSourceRevision(source.SourceRevision())
	builder.SetCompilerSchemaVersion(source.CompilerSchemaVersion())
	builder.SetCoverageManifest(source.CoverageManifest())
	for _, value := range source.BoundedContexts() {
		builder.AddBoundedContext(value)
	}
	for _, value := range source.KindDefinitions() {
		builder.AddKindDefinition(value)
	}
	for _, value := range source.EntitySetDefinitions() {
		builder.AddEntitySetDefinition(value)
	}
	for _, value := range source.KindSignatureDefinitions() {
		builder.AddKindSignatureDefinition(value)
	}
	for _, value := range source.RefKindDefinitions() {
		builder.AddRefKindDefinition(value)
	}
	for _, value := range source.ContextKindAvailabilities() {
		builder.AddContextKindAvailability(value)
	}
	for _, value := range source.SubkindRelations() {
		builder.AddSubkindRelation(value)
	}
	for _, value := range source.ContextBridges() {
		builder.AddContextBridge(value)
	}
	for _, value := range source.RelationSignatures() {
		builder.AddRelationSignature(value)
	}
	for _, value := range source.ValueShapes() {
		builder.AddValueShape(value)
	}
	for _, value := range source.ValueBindings() {
		builder.AddValueBinding(value)
	}
	for _, value := range source.Constraints() {
		builder.AddConstraint(value)
	}
	return builder
}

func exactBasisExcludedKind(t *testing.T) typedmemory.KindID {
	t.Helper()
	return mustGenericKindID(t, "U.Episteme")
}

func assertExactStatementArguments(t *testing.T, got []any, want []any) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("argument count = %d; want %d", len(got), len(want))
	}
	for index := range got {
		gotBytes, gotIsBytes := got[index].([]byte)
		wantBytes, wantIsBytes := want[index].([]byte)
		if gotIsBytes || wantIsBytes {
			if !gotIsBytes || !wantIsBytes || !bytes.Equal(gotBytes, wantBytes) {
				t.Fatalf("argument %d = %#v; want %#v", index, got[index], want[index])
			}
			continue
		}
		if got[index] != want[index] {
			t.Fatalf("argument %d = %#v; want %#v", index, got[index], want[index])
		}
	}
}

func semanticRowsByKind(
	manifest expectedMaterializationManifest,
	want string,
) []expectedSemanticRowIdentity {
	rows := make([]expectedSemanticRowIdentity, 0)
	for _, row := range manifest.semanticRows {
		if row.rowKind == want {
			rows = append(rows, row)
		}
	}
	return rows
}

func equalStrings(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
