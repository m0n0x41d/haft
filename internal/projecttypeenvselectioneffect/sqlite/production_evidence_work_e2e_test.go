package sqlite

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projectmemory"
	"github.com/m0n0x41d/haft/internal/projectmemory/carrierfamily"
	"github.com/m0n0x41d/haft/internal/projectmemory/evidenceworkadapter"
	"github.com/m0n0x41d/haft/internal/projectmemory/recordcarrier"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorycandidatecodec"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
	"github.com/m0n0x41d/haft/internal/typedmemoryvalidation"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

func TestProductionEvidenceWorkAdapterAdmitsClaimBoundLocalRelations(
	t *testing.T,
) {
	fixture := newProductionNoteSelectedFixture(t)
	ctx := context.Background()
	selection, err := fixture.service.SelectGenesis(
		ctx,
		genesisSelectionInput(fixture),
	)
	mustProductionNoteNoError(t, err)
	if _, ok := selection.(projecttypeenvselectioneffect.FreshlyCommitted); !ok {
		t.Fatalf(
			"production TypeEnv selection = %T, want FreshlyCommitted",
			selection,
		)
	}
	resolver := genesisE2EProjectRuntimeResolver(t, fixture)
	baseLoader, err :=
		typedmemorystore.NewProjectAwareSQLiteCurrentProjectSnapshotLoader(
			fixture.database,
			projectmemory.NewBaseTypeEnvLoader(),
			resolver,
		)
	mustProductionNoteNoError(t, err)
	clock := &genesisE2EClock{
		value: time.Date(2026, 7, 18, 14, 0, 0, 0, time.UTC),
	}
	contextRef, err := typedmemory.NewBoundedContextRef("haft-project")
	mustProductionNoteNoError(t, err)
	concern := productionNoteConcernDeclaration(t, contextRef)
	admitProductionPortfolioConcern(
		t,
		ctx,
		fixture,
		resolver,
		baseLoader,
		clock,
		concern,
	)
	carrierEdition := productionCarrierEditionDeclaration(t, contextRef)
	admitProductionSingleDeclaration(
		t,
		ctx,
		fixture,
		resolver,
		baseLoader,
		clock,
		carrierEdition,
		"production-evidence-carrier-edition-declaration",
	)

	current := loadProductionPortfolioSnapshot(
		t,
		ctx,
		baseLoader,
		fixture.project,
	)
	claim := productionProjectClaimAtConcernCandidate(
		t,
		fixture.project,
		current,
		concern,
		contextRef,
	)
	admitProductionCarrierCandidate(
		t,
		ctx,
		fixture,
		resolver,
		baseLoader,
		clock,
		claim.changeSet,
		claim.stage,
		"production-evidence-target-claim",
	)
	current = loadProductionPortfolioSnapshot(
		t,
		ctx,
		baseLoader,
		fixture.project,
	)
	draft := productionEvidenceWorkDraft(
		t,
		fixture.project,
		current,
		concern.Entity(),
		claim.entity,
		carrierEdition.Entity(),
		contextRef,
	)
	result := evidenceworkadapter.Adapt(
		draft,
		productionEvidenceWorkExactRuntime(t, fixture, current),
	)
	candidate, ok := result.(evidenceworkadapter.ValidCandidate)
	if !ok {
		t.Fatalf(
			"Evidence/Work adapter result = %T, want ValidCandidate",
			result,
		)
	}
	assertProductionEvidenceWorkCandidate(t, candidate)
	candidateStage, err :=
		evidenceworkadapter.SealPreAdmissionSourceStage(candidate)
	mustProductionNoteNoError(t, err)
	carrierSource := productionCarrierEditionSource(
		t,
		fixture.project,
		carrierEdition.Entity(),
		contextRef,
	)
	carrierStage := newProductionCarrierFamilySourceStage(t, carrierSource)
	stage := newProductionCompositeObservableStage(
		t,
		ctx,
		fixture.project,
		candidateStage,
		carrierStage,
	)
	receipt := admitProductionEvidenceWorkCandidate(
		t,
		ctx,
		fixture,
		resolver,
		baseLoader,
		clock,
		candidate,
		stage,
	)
	assertProductionEvidenceWorkReread(
		t,
		ctx,
		fixture,
		receipt.GraphRevision(),
	)
}

func productionCarrierEditionDeclaration(
	t *testing.T,
	contextRef typedmemory.BoundedContextRef,
) typedmemory.DeclareEntity {
	t.Helper()
	entity, err := typedmemory.NewEntityID(
		"carrier-edition:production-evidence-source",
	)
	mustProductionNoteNoError(t, err)
	local, err := typedmemory.NewBatchLocalRef(
		"carrier-edition:production-evidence-source",
	)
	mustProductionNoteNoError(t, err)
	label, err := typedmemory.NewEntityLabel(
		"Production evidence source carrier edition",
	)
	mustProductionNoteNoError(t, err)
	provenance, err := typedmemory.NewProvenanceRef(
		"memory:test:production-evidence-source-carrier-edition",
	)
	mustProductionNoteNoError(t, err)
	declaration, err := typedmemory.NewDeclareEntity(
		entity,
		local,
		contextRef,
		label,
		provenance,
	)
	mustProductionNoteNoError(t, err)
	return declaration
}

func admitProductionSingleDeclaration(
	t *testing.T,
	ctx context.Context,
	fixture genesisE2EFixture,
	resolver typedmemorystore.SelectedProjectTypeEnvRuntimeResolver,
	baseLoader typedmemorystore.CurrentProjectSnapshotLoader,
	clock typedmemorystore.Clock,
	declaration typedmemory.DeclareEntity,
	token string,
) {
	t.Helper()
	adapter := newProductionNoteCommitAdapter(
		t,
		fixture,
		resolver,
		clock,
		productionNoteUnavailableObservableProvider{},
	)
	source := newGenesisE2ECurrentProjectBasisSource(t, baseLoader)
	runtime, err := projectmemory.NewAdmissionRuntime(
		fixture.project,
		source,
		adapter,
	)
	mustProductionNoteNoError(t, err)
	changeSet, err := typedmemory.NewMemoryChangeSet(
		[]typedmemory.MemoryChange{declaration},
	)
	mustProductionNoteNoError(t, err)
	valid, err := runtime.PrepareCandidate(
		ctx,
		typedmemorywire.ProjectCurrentSelector{},
		changeSet,
	)
	mustProductionNoteNoError(t, err)
	key, err := typedmemorystore.NewIdempotencyKey(token)
	mustProductionNoteNoError(t, err)
	receipt, err := runtime.AdmitValidated(
		ctx,
		valid,
		key,
		declaration.Provenance(),
	)
	mustProductionNoteNoError(t, err)
	if receipt.Disposition() != typedmemorystore.CommitApplied {
		t.Fatalf("%s disposition = %s, want applied", token, receipt.Disposition())
	}
}

func productionEvidenceWorkDraft(
	t *testing.T,
	project projectidentity.ProjectID,
	current typedmemorystore.CurrentProjectSnapshot,
	concernEntity typedmemory.EntityID,
	claimEntity typedmemory.EntityID,
	carrierEditionEntity typedmemory.EntityID,
	contextRef typedmemory.BoundedContextRef,
) evidenceworkadapter.Draft {
	t.Helper()
	concernResolution := productionResolveReference(
		t,
		current,
		"U.EntityRef",
		concernEntity,
		contextRef,
	)
	concern, err :=
		evidenceworkadapter.NewExactConcernReference(concernResolution)
	mustProductionNoteNoError(t, err)
	performer, err :=
		evidenceworkadapter.NewExactPerformerReference(concernResolution)
	mustProductionNoteNoError(t, err)
	claimResolution := productionResolveReference(
		t,
		current,
		"Haft.ProjectClaimRef",
		claimEntity,
		contextRef,
	)
	claim, err :=
		evidenceworkadapter.NewExactProjectClaimReference(claimResolution)
	mustProductionNoteNoError(t, err)
	carrierResolution := productionResolveReference(
		t,
		current,
		"Haft.CarrierEditionRef",
		carrierEditionEntity,
		contextRef,
	)
	carrierEdition, err :=
		evidenceworkadapter.NewExactCarrierEditionReference(carrierResolution)
	mustProductionNoteNoError(t, err)
	supportingGraph, _ := productionProjectClaimGraph(
		t,
		current,
		"Test output was interpreted for one explicit project claim",
	)
	exactSupporting, err :=
		evidenceworkadapter.NewExactClaimGraph(supportingGraph)
	mustProductionNoteNoError(t, err)
	workGraph, _ := productionProjectClaimGraph(
		t,
		current,
		"The named test execution occurred in the bounded interval",
	)
	exactWork, err := evidenceworkadapter.NewExactClaimGraph(workGraph)
	mustProductionNoteNoError(t, err)
	qualifier, err := typedmemorycandidatecodec.NewEvidenceUseQualifier(
		typedmemorycandidatecodec.EvidenceConfirming,
	)
	mustProductionNoteNoError(t, err)
	start, err := typedmemorycandidatecodec.ParseCanonicalInstant(
		"2026-07-18T13:55:00Z",
	)
	mustProductionNoteNoError(t, err)
	end, err := typedmemorycandidatecodec.ParseCanonicalInstant(
		"2026-07-18T13:56:00Z",
	)
	mustProductionNoteNoError(t, err)
	interval, err := typedmemorycandidatecodec.NewCompletedPerformedInterval(
		start,
		end,
	)
	mustProductionNoteNoError(t, err)
	provenance, err := typedmemory.NewProvenanceRef(
		"memory:test:production-evidence-work",
	)
	mustProductionNoteNoError(t, err)
	draft, err := evidenceworkadapter.NewDraft(
		evidenceworkadapter.DraftInput{
			ProjectID:                project,
			EvidenceRecord:           productionEvidenceWorkIdentity(t, "evidence"),
			SupportingEpistemeRecord: productionEvidenceWorkIdentity(t, "supporting"),
			WorkRecord:               productionEvidenceWorkIdentity(t, "work"),
			PerformedWorkOccurrence:  productionEvidenceWorkIdentity(t, "occurrence"),
			SupportingAssertionID:    productionAssertionID(t, "supporting-episteme"),
			WorkAssertionID:          productionAssertionID(t, "work-occurrence"),
			EvidenceUseAssertionID:   productionAssertionID(t, "evidence-use"),
			ContextSlice:             productionCodeAnchorContextSlice(t, contextRef),
			Concern:                  concern,
			Performer:                performer,
			TargetClaim:              claim,
			ProvenanceCarrierEdition: carrierEdition,
			Qualifier:                qualifier,
			Interval:                 interval,
			SupportingClaimGraph:     exactSupporting,
			WorkClaimGraph:           exactWork,
			Provenance:               provenance,
		},
	)
	mustProductionNoteNoError(t, err)
	return draft
}

func productionEvidenceWorkIdentity(
	t *testing.T,
	token string,
) evidenceworkadapter.NewEntityIdentity {
	t.Helper()
	entity, err := typedmemory.NewEntityID(
		"evidence-work:production-" + token,
	)
	mustProductionNoteNoError(t, err)
	local, err := typedmemory.NewBatchLocalRef(
		"evidence-work:production-" + token,
	)
	mustProductionNoteNoError(t, err)
	label, err := typedmemory.NewEntityLabel(
		"Production Evidence/Work " + token,
	)
	mustProductionNoteNoError(t, err)
	identity, err := evidenceworkadapter.NewEntityIdentityValue(
		entity,
		local,
		label,
	)
	mustProductionNoteNoError(t, err)
	return identity
}

func productionAssertionID(
	t *testing.T,
	token string,
) typedmemory.AssertionID {
	t.Helper()
	assertion, err := typedmemory.NewAssertionID(
		"assertion:production-evidence-work-" + token,
	)
	mustProductionNoteNoError(t, err)
	return assertion
}

func productionResolveReference(
	t *testing.T,
	current typedmemorystore.CurrentProjectSnapshot,
	refKindRaw string,
	entity typedmemory.EntityID,
	contextRef typedmemory.BoundedContextRef,
) typedmemory.ResolvedStrongReference {
	t.Helper()
	refKindID, err := typedmemory.NewRefKindID(refKindRaw)
	mustProductionNoteNoError(t, err)
	refKind, err := typedmemory.NewRefKindRef(
		current.Environment().Ref(),
		refKindID,
	)
	mustProductionNoteNoError(t, err)
	referenceID, err := typedmemory.NewReferenceID(entity.String())
	mustProductionNoteNoError(t, err)
	reference, err := typedmemory.NewPersistedRef(refKind, referenceID)
	mustProductionNoteNoError(t, err)
	resolution := current.Snapshot().ResolveReference(reference, contextRef)
	resolved, ok := resolution.(typedmemory.ResolvedStrongReference)
	if !ok {
		t.Fatalf(
			"%s resolution = %T, want ResolvedStrongReference",
			refKindRaw,
			resolution,
		)
	}
	return resolved
}

func productionEvidenceWorkExactRuntime(
	t *testing.T,
	fixture genesisE2EFixture,
	current typedmemorystore.CurrentProjectSnapshot,
) evidenceworkadapter.ExactRuntimeBasis {
	t.Helper()
	runtimeDigest, err := typedmemorystore.NewSelectedRuntimeBasisDigest(
		fixture.target.runtime.Digest(),
	)
	mustProductionNoteNoError(t, err)
	coordinate, found := fixture.target.registry.CoordinateDigest()
	if !found {
		t.Fatal("production Evidence/Work runtime has no registry coordinate")
	}
	registryDigest, err :=
		typedmemorystore.NewExactTargetRegistryCoordinateDigest(coordinate)
	mustProductionNoteNoError(t, err)
	recordPolicy := productionPolicyForRule(
		t,
		fixture.target.installed.RegistrationPolicies,
		recordcarrier.NewRecordMembershipEvaluatorV1().RuleRef(),
	)
	workPolicy := productionPolicyForRule(
		t,
		fixture.target.installed.RegistrationPolicies,
		carrierfamily.PerformedWorkOccurrenceEvaluatorRuleV1(),
	)
	runtime, err := evidenceworkadapter.NewExactRuntimeBasisBuilder(
		fixture.project,
	).
		SetGraphRevision(current.Snapshot().GraphRevision()).
		SetEnvironment(current.Environment()).
		SetCodecs(current.Codecs()).
		SetSelectedRuntimeCoordinates(runtimeDigest, registryDigest).
		SetRecordRegistrationPolicy(recordPolicy).
		SetPerformedWorkRegistrationPolicy(workPolicy).
		Build()
	mustProductionNoteNoError(t, err)
	return runtime
}

func productionCarrierEditionSource(
	t *testing.T,
	project projectidentity.ProjectID,
	entity typedmemory.EntityID,
	contextRef typedmemory.BoundedContextRef,
) carrierfamily.MembershipSourceV1 {
	t.Helper()
	canonical := []byte(
		`{"carrier":"production-evidence-source","edition":"1.0.0"}`,
	)
	digest := productionSHA256Digest(t, canonical)
	ref, err := typedmemory.NewCarrierRef(
		"production-evidence-source:" + digest.String(),
	)
	mustProductionNoteNoError(t, err)
	edition, err := typedmemory.NewCarrierEdition("1.0.0")
	mustProductionNoteNoError(t, err)
	payload, err := carrierfamily.NewSourcePayloadV1(
		ref,
		edition,
		digest,
		"haft.production-evidence-source/v1",
		canonical,
	)
	mustProductionNoteNoError(t, err)
	carrier, err := carrierfamily.SealCarrierEditionCarrierV1(
		entity,
		contextRef,
		payload,
	)
	mustProductionNoteNoError(t, err)
	manifest, err := carrierfamily.CurrentCarrierEditionMappingManifestV1()
	mustProductionNoteNoError(t, err)
	binding, err := carrierfamily.SealEntityCarrierBindingV1(
		project,
		carrier,
		manifest.Ref(),
		manifest.AdapterVersion(),
	)
	mustProductionNoteNoError(t, err)
	source, err := carrierfamily.SealMembershipSourceV1(
		project,
		entity,
		contextRef,
		carrier,
		binding,
	)
	mustProductionNoteNoError(t, err)
	return source
}

type productionCompositeObservableStage struct {
	project projectidentity.ProjectID
	blobs   map[string]typedmemorystore.ObservableInputBlob
	order   []string
}

var _ typedmemorystore.ObservableInputContentProvider = productionCompositeObservableStage{}
var _ typedmemorystore.SnapshotObservableInputOverlay = productionCompositeObservableStage{}

func newProductionCompositeObservableStage(
	t *testing.T,
	ctx context.Context,
	project projectidentity.ProjectID,
	overlays ...typedmemorystore.SnapshotObservableInputOverlay,
) productionCompositeObservableStage {
	t.Helper()
	blobs := make(map[string]typedmemorystore.ObservableInputBlob)
	for _, overlay := range overlays {
		values, err := overlay.LoadSnapshotObservableInputs(ctx, project)
		mustProductionNoteNoError(t, err)
		for _, blob := range values {
			key := blob.Reference().String()
			if _, exists := blobs[key]; exists {
				t.Fatalf("duplicate composite observable %s", key)
			}
			blobs[key] = blob
		}
	}
	order := make([]string, 0, len(blobs))
	for raw := range blobs {
		order = append(order, raw)
	}
	sort.Strings(order)
	return productionCompositeObservableStage{
		project: project,
		blobs:   blobs,
		order:   order,
	}
}

func (stage productionCompositeObservableStage) LoadObservableInput(
	ctx context.Context,
	project projectidentity.ProjectID,
	reference typedmemory.ObservableInputRef,
	digest typedmemory.SHA256Digest,
) (typedmemorystore.ObservableInputBlob, error) {
	if err := ctx.Err(); err != nil {
		return typedmemorystore.ObservableInputBlob{}, err
	}
	blob, found := stage.blobs[reference.String()]
	if project != stage.project || !found || blob.Digest() != digest {
		return typedmemorystore.ObservableInputBlob{}, fmt.Errorf(
			"production composite observable is unavailable",
		)
	}
	return typedmemorystore.NewObservableInputBlob(
		blob.Reference(),
		blob.Digest(),
		blob.Bytes(),
	)
}

func (stage productionCompositeObservableStage) LoadSnapshotObservableInputs(
	ctx context.Context,
	project projectidentity.ProjectID,
) ([]typedmemorystore.ObservableInputBlob, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if project != stage.project {
		return nil, fmt.Errorf("production composite observable project mismatch")
	}
	result := make(
		[]typedmemorystore.ObservableInputBlob,
		0,
		len(stage.order),
	)
	for _, raw := range stage.order {
		result = append(result, stage.blobs[raw])
	}
	return result, nil
}

func assertProductionEvidenceWorkCandidate(
	t *testing.T,
	candidate evidenceworkadapter.ValidCandidate,
) {
	t.Helper()
	changes := candidate.ChangeSet().Changes()
	if len(changes) != 7 {
		t.Fatalf(
			"Evidence/Work changes = %d, want four declarations plus three relations",
			len(changes),
		)
	}
	counts := map[string]int{}
	for _, change := range changes {
		relation, ok := change.(typedmemory.AssertRelation)
		if ok {
			counts[relation.Assertion().Signature().ID().String()]++
		}
	}
	for _, signature := range []string{
		"Haft.SupportingEpistemeRecordAtConcern",
		"Haft.WorkOccurrenceRecord",
		"Haft.EvidenceUse",
	} {
		if counts[signature] != 1 {
			t.Fatalf(
				"Evidence/Work relation counts = %#v, want one %s",
				counts,
				signature,
			)
		}
	}
	for raw := range counts {
		if raw == "U.Work" || raw == "A.10.Evidence" {
			t.Fatalf("Evidence/Work candidate fabricated exact FPF relation %s", raw)
		}
	}
}

func admitProductionEvidenceWorkCandidate(
	t *testing.T,
	ctx context.Context,
	fixture genesisE2EFixture,
	resolver typedmemorystore.SelectedProjectTypeEnvRuntimeResolver,
	baseLoader typedmemorystore.CurrentProjectSnapshotLoader,
	clock typedmemorystore.Clock,
	candidate evidenceworkadapter.ValidCandidate,
	stage productionCompositeObservableStage,
) typedmemorystore.CommitReceipt {
	t.Helper()
	overlay, err :=
		typedmemorystore.NewCurrentProjectSnapshotLoaderWithObservableInputOverlay(
			baseLoader,
			stage,
		)
	mustProductionNoteNoError(t, err)
	source := newGenesisE2ECurrentProjectBasisSource(t, overlay)
	adapter := newProductionNoteCommitAdapter(
		t,
		fixture,
		resolver,
		clock,
		stage,
	)
	validation, err := projectmemory.NewValidationRuntime(
		fixture.project,
		source,
	)
	mustProductionNoteNoError(t, err)
	outcome, err := validation.EvaluateCandidate(
		ctx,
		typedmemorywire.ProjectCurrentSelector{},
		candidate.ChangeSet(),
	)
	mustProductionNoteNoError(t, err)
	valid, ok := outcome.(typedmemoryvalidation.ValidOutcome)
	if !ok {
		t.Fatalf(
			"Evidence/Work validation = %T/%s diagnostics=%#v",
			outcome,
			outcome.Verdict(),
			outcome.Diagnostics(),
		)
	}
	runtime, err := projectmemory.NewAdmissionRuntime(
		fixture.project,
		source,
		adapter,
	)
	mustProductionNoteNoError(t, err)
	key, err := typedmemorystore.NewIdempotencyKey(
		"production-evidence-work-admission",
	)
	mustProductionNoteNoError(t, err)
	provenance, err := typedmemory.NewProvenanceRef(
		"memory:test:production-evidence-work-admission",
	)
	mustProductionNoteNoError(t, err)
	receipt, err := runtime.AdmitValidated(
		ctx,
		valid,
		key,
		provenance,
	)
	mustProductionNoteNoError(t, err)
	if receipt.Disposition() != typedmemorystore.CommitApplied {
		t.Fatalf(
			"Evidence/Work admission = %s, want applied",
			receipt.Disposition(),
		)
	}
	replay, err := runtime.AdmitValidated(
		ctx,
		valid,
		key,
		provenance,
	)
	mustProductionNoteNoError(t, err)
	if replay.Disposition() != typedmemorystore.CommitReplay ||
		replay.EventRef() != receipt.EventRef() ||
		replay.ResultDigest() != receipt.ResultDigest() {
		t.Fatal("Evidence/Work replay changed the committed result")
	}
	return receipt
}

func assertProductionEvidenceWorkReread(
	t *testing.T,
	ctx context.Context,
	fixture genesisE2EFixture,
	revision typedmemory.GraphRevision,
) {
	t.Helper()
	transaction, err := sqlitetransaction.BeginRead(ctx, fixture.database)
	mustProductionNoteNoError(t, err)
	observation, err := typedmemorystore.LoadCurrentGraphRevalidationBasisTx(
		ctx,
		transaction,
		fixture.project,
	)
	mustProductionNoteNoError(t, err)
	finish := transaction.Commit(ctx)
	if !finish.Succeeded() {
		t.Fatalf("commit Evidence/Work durable read: %v", finish.Err())
	}
	if observation.GraphSnapshotBasis().GraphRevision() != revision {
		t.Fatal("Evidence/Work durable reread observed another graph revision")
	}
	counts := map[string]int{}
	for _, active := range observation.ActiveAssertions().Relations() {
		carrier := productionFreshCurrentAssertionCarrier(t, active)
		counts[carrier.Signature().ID().String()]++
	}
	for _, signature := range []string{
		"Haft.ProjectClaimAtConcern",
		"Haft.SupportingEpistemeRecordAtConcern",
		"Haft.WorkOccurrenceRecord",
		"Haft.EvidenceUse",
	} {
		if counts[signature] != 1 {
			t.Fatalf(
				"durable Evidence/Work counts = %#v, want one %s",
				counts,
				signature,
			)
		}
	}
}
