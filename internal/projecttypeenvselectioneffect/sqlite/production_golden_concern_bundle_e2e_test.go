package sqlite

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projectmemory"
	"github.com/m0n0x41d/haft/internal/projectmemory/architecturep2s"
	"github.com/m0n0x41d/haft/internal/projectmemory/codeanchoradapter"
	"github.com/m0n0x41d/haft/internal/projectmemory/decisionrecordadapter"
	"github.com/m0n0x41d/haft/internal/projectmemory/evidenceworkadapter"
	"github.com/m0n0x41d/haft/internal/projectmemory/goldenconcernbundle"
	"github.com/m0n0x41d/haft/internal/projectmemory/memoryresolve"
	"github.com/m0n0x41d/haft/internal/projectmemory/neighborhood"
	"github.com/m0n0x41d/haft/internal/projectmemory/noteadapter"
	"github.com/m0n0x41d/haft/internal/projectmemory/portfoliocomparisonadapter"
	"github.com/m0n0x41d/haft/internal/projectmemory/problemcardadapter"
	"github.com/m0n0x41d/haft/internal/projectmemory/scopedrecall"
	"github.com/m0n0x41d/haft/internal/projectmemory/solutionportfolioadapter"
	"github.com/m0n0x41d/haft/internal/projectmemory/specsectionadapter"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorycandidatecodec"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

func TestProductionGoldenConcernBundleRecoversExactHaftSoftwareSystemMemory(
	t *testing.T,
) {
	fixture := newProductionNoteSelectedFixture(t)
	ctx := context.Background()
	selectionInput := genesisSelectionInput(fixture)
	selection, err := fixture.service.SelectGenesis(ctx, selectionInput)
	mustProductionNoteNoError(t, err)
	if _, ok := selection.(projecttypeenvselectioneffect.FreshlyCommitted); !ok {
		t.Fatalf(
			"production TypeEnv selection = %T, want FreshlyCommitted",
			selection,
		)
	}

	resolver := genesisE2EProjectRuntimeResolver(t, fixture)
	baseTypeEnv := projectmemory.NewBaseTypeEnvLoader()
	baseLoader, err :=
		typedmemorystore.NewProjectAwareSQLiteCurrentProjectSnapshotLoader(
			fixture.database,
			baseTypeEnv,
			resolver,
		)
	mustProductionNoteNoError(t, err)
	clock := &genesisE2EClock{
		value: time.Date(2026, 7, 18, 15, 0, 0, 0, time.UTC),
	}
	contextRef, err := typedmemory.NewBoundedContextRef("haft-project")
	mustProductionNoteNoError(t, err)

	concern := goldenHaftSoftwareSystemDeclaration(t, contextRef)
	concernReceipt := admitProductionPortfolioConcern(
		t,
		ctx,
		fixture,
		resolver,
		baseLoader,
		clock,
		concern,
	)
	current := loadProductionPortfolioSnapshot(
		t,
		ctx,
		baseLoader,
		fixture.project,
	)
	concernRef := goldenPersistedReference(
		t,
		current.Environment(),
		"U.EntityRef",
		concern.Entity(),
	)
	concernAdmission, err := goldenconcernbundle.NewConcernAdmission(
		fixture.project,
		concern,
		concernRef,
		concernReceipt,
	)
	mustProductionNoteNoError(t, err)

	problemDraft, problemEntity, _ := productionProblemCardDraft(
		t,
		fixture,
		current,
		contextRef,
	)
	problemRuntime := productionNoteExactRuntime(t, fixture, current)
	problemConcern := productionNoteConcernBinding(
		t,
		current,
		concern.Entity(),
		contextRef,
	)
	problemResult := problemcardadapter.Adapt(
		problemDraft,
		problemRuntime,
		problemConcern,
	)
	problemCandidate := mustProductionRecordCandidate(t, problemResult)
	_, problemReceipt := admitProductionPortfolioRecord(
		t,
		ctx,
		fixture,
		resolver,
		baseLoader,
		clock,
		problemCandidate,
		"golden-concern-problem-card",
	)
	problemAdmission, err :=
		goldenconcernbundle.NewRecordAdapterAdmission(
			fixture.project,
			problemCandidate,
			problemReceipt,
		)
	mustProductionNoteNoError(t, err)

	frozen := loadGoldenP4DecisionFixture(t)
	optionEntities := make(
		map[string]typedmemory.EntityID,
		len(frozen.ChoiceResult.OptionSet),
	)
	optionRefs := make(
		map[string]typedmemory.PersistedRef,
		len(frozen.ChoiceResult.OptionSet),
	)
	optionAdmissions := make(
		[]goldenconcernbundle.AdapterAdmission,
		0,
		len(frozen.ChoiceResult.OptionSet),
	)
	optionReceipts := make(
		map[string]typedmemorystore.CommitReceipt,
		len(frozen.ChoiceResult.OptionSet),
	)
	for index, label := range frozen.ChoiceResult.OptionSet {
		current = loadProductionPortfolioSnapshot(
			t,
			ctx,
			baseLoader,
			fixture.project,
		)
		token := goldenOptionToken(index)
		input := productionPortfolioRecordInput(
			t,
			fixture.project,
			current.Environment(),
			contextRef,
			token,
			label,
		)
		draft, draftErr := noteadapter.NewDraft(input)
		mustProductionNoteNoError(t, draftErr)
		runtime := productionNoteExactRuntime(t, fixture, current)
		concernBinding := productionNoteConcernBinding(
			t,
			current,
			concern.Entity(),
			contextRef,
		)
		result := noteadapter.Adapt(draft, runtime, concernBinding)
		candidate := mustProductionRecordCandidate(t, result)
		admissionToken := "golden-concern-" + token
		_, receipt := admitProductionPortfolioRecord(
			t,
			ctx,
			fixture,
			resolver,
			baseLoader,
			clock,
			candidate,
			admissionToken,
		)
		admission, admissionErr :=
			goldenconcernbundle.NewRecordAdapterAdmission(
				fixture.project,
				candidate,
				receipt,
			)
		mustProductionNoteNoError(t, admissionErr)
		optionEntities[label] = input.RecordEntity
		optionAdmissions = append(optionAdmissions, admission)
		optionReceipts[label] = receipt
	}

	current = loadProductionPortfolioSnapshot(
		t,
		ctx,
		baseLoader,
		fixture.project,
	)
	orderedOptionRefs := make(
		[]typedmemory.PersistedRef,
		0,
		len(frozen.ChoiceResult.OptionSet),
	)
	for _, label := range frozen.ChoiceResult.OptionSet {
		reference := productionProjectRecordReference(
			t,
			current.Environment(),
			optionEntities[label],
		)
		optionRefs[label] = reference
		orderedOptionRefs = append(orderedOptionRefs, reference)
	}
	portfolioInput := productionPortfolioRecordInput(
		t,
		fixture.project,
		current.Environment(),
		contextRef,
		"golden-p4-portfolio",
		"P4 typed-memory architecture option portfolio",
	)
	portfolioDraft, err := solutionportfolioadapter.NewDraft(
		solutionportfolioadapter.DraftInput{
			Record:  portfolioInput,
			Options: orderedOptionRefs,
		},
	)
	mustProductionNoteNoError(t, err)
	portfolioRuntime := productionNoteExactRuntime(t, fixture, current)
	portfolioConcern := productionNoteConcernBinding(
		t,
		current,
		concern.Entity(),
		contextRef,
	)
	portfolioResult := solutionportfolioadapter.Adapt(
		portfolioDraft,
		portfolioRuntime,
		portfolioConcern,
	)
	portfolioCandidate := mustProductionRecordCandidate(t, portfolioResult)
	_, portfolioReceipt := admitProductionPortfolioRecord(
		t,
		ctx,
		fixture,
		resolver,
		baseLoader,
		clock,
		portfolioCandidate,
		"golden-concern-p4-portfolio",
	)
	portfolioAdmission, err :=
		goldenconcernbundle.NewRecordAdapterAdmission(
			fixture.project,
			portfolioCandidate,
			portfolioReceipt,
		)
	mustProductionNoteNoError(t, err)

	current = loadProductionPortfolioSnapshot(
		t,
		ctx,
		baseLoader,
		fixture.project,
	)
	portfolioRef := productionProjectRecordReference(
		t,
		current.Environment(),
		portfolioInput.RecordEntity,
	)
	comparisonInput := productionPortfolioRecordInput(
		t,
		fixture.project,
		current.Environment(),
		contextRef,
		"golden-p4-comparison",
		"P4 typed-memory architecture comparison",
	)
	selectedOptionRef := optionRefs[frozen.ChoiceResult.VariantRef]
	comparisonDraft, err := portfoliocomparisonadapter.NewDraft(
		portfoliocomparisonadapter.DraftInput{
			Record:              comparisonInput,
			Portfolio:           portfolioRef,
			ComparedOptions:     orderedOptionRefs,
			NonDominatedOptions: []typedmemory.PersistedRef{selectedOptionRef},
		},
	)
	mustProductionNoteNoError(t, err)
	comparisonRuntime := productionNoteExactRuntime(t, fixture, current)
	comparisonConcern := productionNoteConcernBinding(
		t,
		current,
		concern.Entity(),
		contextRef,
	)
	comparisonResult := portfoliocomparisonadapter.Adapt(
		comparisonDraft,
		comparisonRuntime,
		comparisonConcern,
	)
	comparisonCandidate := mustProductionRecordCandidate(t, comparisonResult)
	_, comparisonReceipt := admitProductionPortfolioRecord(
		t,
		ctx,
		fixture,
		resolver,
		baseLoader,
		clock,
		comparisonCandidate,
		"golden-concern-p4-comparison",
	)
	comparisonAdmission, err :=
		goldenconcernbundle.NewRecordAdapterAdmission(
			fixture.project,
			comparisonCandidate,
			comparisonReceipt,
		)
	mustProductionNoteNoError(t, err)

	current = loadProductionPortfolioSnapshot(
		t,
		ctx,
		baseLoader,
		fixture.project,
	)
	comparisonRef := productionProjectRecordReference(
		t,
		current.Environment(),
		comparisonInput.RecordEntity,
	)
	decisionSource := seedGoldenP4DecisionChoice(
		t,
		ctx,
		fixture,
		frozen,
	)
	problemRef := productionProjectRecordReference(
		t,
		current.Environment(),
		problemEntity,
	)
	decisionDraft := goldenP4DecisionChoiceDraft(
		t,
		fixture,
		current,
		contextRef,
		concern,
		decisionSource,
		frozen,
		problemRef,
		optionRefs,
		comparisonRef,
	)
	decisionRuntime := productionNoteExactRuntime(t, fixture, current)
	decisionResult := decisionrecordadapter.Adapt(
		decisionDraft,
		decisionRuntime,
	)
	decisionCandidate := mustProductionRecordCandidate(t, decisionResult)
	_, decisionReceipt := admitProductionPortfolioRecord(
		t,
		ctx,
		fixture,
		resolver,
		baseLoader,
		clock,
		decisionCandidate,
		"golden-concern-p4-decision",
	)
	decisionAdmission, err :=
		goldenconcernbundle.NewRecordAdapterAdmission(
			fixture.project,
			decisionCandidate,
			decisionReceipt,
		)
	mustProductionNoteNoError(t, err)

	current = loadProductionPortfolioSnapshot(
		t,
		ctx,
		baseLoader,
		fixture.project,
	)
	specInput := productionPortfolioRecordInput(
		t,
		fixture.project,
		current.Environment(),
		contextRef,
		"golden-spec-section",
		"SS.constraints.typed-memory.001.D1",
	)
	specDraft, err := specsectionadapter.NewDraft(specInput)
	mustProductionNoteNoError(t, err)
	specRuntime := productionNoteExactRuntime(t, fixture, current)
	specConcern := productionNoteConcernBinding(
		t,
		current,
		concern.Entity(),
		contextRef,
	)
	specResult := specsectionadapter.Adapt(
		specDraft,
		specRuntime,
		specConcern,
	)
	specCandidate := mustProductionRecordCandidate(t, specResult)
	_, specReceipt := admitProductionPortfolioRecord(
		t,
		ctx,
		fixture,
		resolver,
		baseLoader,
		clock,
		specCandidate,
		"golden-concern-spec-section",
	)
	specAdmission, err :=
		goldenconcernbundle.NewRecordAdapterAdmission(
			fixture.project,
			specCandidate,
			specReceipt,
		)
	mustProductionNoteNoError(t, err)

	current = loadProductionPortfolioSnapshot(
		t,
		ctx,
		baseLoader,
		fixture.project,
	)
	specRecordRef := goldenPersistedReference(
		t,
		current.Environment(),
		"Haft.ProjectRecordRef",
		specInput.RecordEntity,
	)
	claim := goldenProjectClaimStatesSpecCandidate(
		t,
		fixture.project,
		current,
		concern,
		contextRef,
		specRecordRef,
	)
	claimReceipt := admitProductionCarrierCandidate(
		t,
		ctx,
		fixture,
		resolver,
		baseLoader,
		clock,
		claim.changeSet,
		claim.stage,
		"golden-concern-project-claim",
	)
	claimAdmission, err :=
		goldenconcernbundle.NewGovernedProjectClaimAdmission(
			fixture.project,
			claim.changeSet,
			claim.source,
			claimReceipt,
		)
	mustProductionNoteNoError(t, err)

	carrierEdition := productionCarrierEditionDeclaration(t, contextRef)
	admitProductionSingleDeclaration(
		t,
		ctx,
		fixture,
		resolver,
		baseLoader,
		clock,
		carrierEdition,
		"golden-concern-evidence-carrier",
	)
	current = loadProductionPortfolioSnapshot(
		t,
		ctx,
		baseLoader,
		fixture.project,
	)
	evidenceDraft := productionEvidenceWorkDraft(
		t,
		fixture.project,
		current,
		concern.Entity(),
		claim.entity,
		carrierEdition.Entity(),
		contextRef,
	)
	evidenceRuntime := productionEvidenceWorkExactRuntime(
		t,
		fixture,
		current,
	)
	evidenceResult := evidenceworkadapter.Adapt(
		evidenceDraft,
		evidenceRuntime,
	)
	evidenceCandidate, ok :=
		evidenceResult.(evidenceworkadapter.ValidCandidate)
	if !ok {
		t.Fatalf(
			"Golden Evidence/Work adapter result = %T, want ValidCandidate",
			evidenceResult,
		)
	}
	evidenceStage, err :=
		evidenceworkadapter.SealPreAdmissionSourceStage(evidenceCandidate)
	mustProductionNoteNoError(t, err)
	carrierSource := productionCarrierEditionSource(
		t,
		fixture.project,
		carrierEdition.Entity(),
		contextRef,
	)
	carrierStage := newProductionCarrierFamilySourceStage(
		t,
		carrierSource,
	)
	compositeStage := newProductionCompositeObservableStage(
		t,
		ctx,
		fixture.project,
		evidenceStage,
		carrierStage,
	)
	evidenceReceipt := admitProductionEvidenceWorkCandidate(
		t,
		ctx,
		fixture,
		resolver,
		baseLoader,
		clock,
		evidenceCandidate,
		compositeStage,
	)
	evidenceAdmission, err :=
		goldenconcernbundle.NewEvidenceWorkAdapterAdmission(
			fixture.project,
			evidenceCandidate,
			evidenceReceipt,
		)
	mustProductionNoteNoError(t, err)

	current = loadProductionPortfolioSnapshot(
		t,
		ctx,
		baseLoader,
		fixture.project,
	)
	claimResolution := productionResolveReference(
		t,
		current,
		"Haft.ProjectClaimRef",
		claim.entity,
		contextRef,
	)
	claimLinkTarget, err :=
		codeanchoradapter.NewExactReferenceBinding(claimResolution)
	mustProductionNoteNoError(t, err)
	occurrenceEntity, err := typedmemory.NewEntityID(
		"evidence-work:production-occurrence",
	)
	mustProductionNoteNoError(t, err)
	workResolution := productionResolveReference(
		t,
		current,
		"Haft.PerformedWorkOccurrenceRef",
		occurrenceEntity,
		contextRef,
	)
	workLinkTarget, err :=
		codeanchoradapter.NewExactReferenceBinding(workResolution)
	mustProductionNoteNoError(t, err)
	codeDraft := goldenCodeAnchorDraft(
		t,
		fixture.project,
		contextRef,
		claimLinkTarget,
		workLinkTarget,
	)
	codeRuntime := productionCodeAnchorExactRuntime(t, fixture, current)
	codeResult := codeanchoradapter.Adapt(codeDraft, codeRuntime)
	codeCandidate, ok := codeResult.(codeanchoradapter.ValidCandidate)
	if !ok {
		t.Fatalf(
			"Golden CodeAnchor adapter result = %T, want ValidCandidate",
			codeResult,
		)
	}
	codeReceipt := admitProductionCodeAnchorCandidate(
		t,
		ctx,
		fixture,
		resolver,
		baseLoader,
		clock,
		codeCandidate,
	)
	codeAdmission, err :=
		goldenconcernbundle.NewCodeAnchorAdapterAdmission(
			fixture.project,
			codeCandidate,
			codeReceipt,
		)
	mustProductionNoteNoError(t, err)

	current = loadProductionPortfolioSnapshot(
		t,
		ctx,
		baseLoader,
		fixture.project,
	)
	coordinate, err := goldenconcernbundle.NewSnapshotCoordinate(
		contextRef,
		current.Environment().Ref(),
		current.Snapshot().GraphRevision(),
		clock.Now(),
	)
	mustProductionNoteNoError(t, err)
	if coordinate.GraphRevision() != codeReceipt.GraphRevision() {
		t.Fatal("GoldenConcernBundle snapshot did not observe the final admission")
	}

	builder := goldenconcernbundle.NewBuilder(fixture.project)
	builder.SetConcern(concernAdmission)
	builder.SetSnapshot(coordinate)
	builder.AddAdapterAdmission(problemAdmission)
	for _, admission := range optionAdmissions {
		builder.AddAdapterAdmission(admission)
	}
	builder.AddAdapterAdmission(portfolioAdmission)
	builder.AddAdapterAdmission(comparisonAdmission)
	builder.AddAdapterAdmission(decisionAdmission)
	builder.AddAdapterAdmission(specAdmission)
	builder.AddAdapterAdmission(claimAdmission)
	builder.AddAdapterAdmission(evidenceAdmission)
	builder.AddAdapterAdmission(codeAdmission)

	addGoldenItem(
		t,
		builder,
		goldenconcernbundle.ItemProblemCard,
		current.Environment(),
		"Haft.ProjectRecordRef",
		problemEntity,
		problemReceipt,
	)
	for _, label := range frozen.ChoiceResult.OptionSet {
		addGoldenItem(
			t,
			builder,
			goldenconcernbundle.ItemSolutionOption,
			current.Environment(),
			"Haft.ProjectRecordRef",
			optionEntities[label],
			optionReceipts[label],
		)
	}
	addGoldenItem(
		t,
		builder,
		goldenconcernbundle.ItemSolutionPortfolio,
		current.Environment(),
		"Haft.ProjectRecordRef",
		portfolioInput.RecordEntity,
		portfolioReceipt,
	)
	addGoldenItem(
		t,
		builder,
		goldenconcernbundle.ItemPortfolioComparison,
		current.Environment(),
		"Haft.ProjectRecordRef",
		comparisonInput.RecordEntity,
		comparisonReceipt,
	)
	decisionEntity, err := typedmemory.NewEntityID(frozen.ID)
	mustProductionNoteNoError(t, err)
	addGoldenItem(
		t,
		builder,
		goldenconcernbundle.ItemDecisionRecord,
		current.Environment(),
		"Haft.DecisionRecordRef",
		decisionEntity,
		decisionReceipt,
	)
	addGoldenItem(
		t,
		builder,
		goldenconcernbundle.ItemSpecSection,
		current.Environment(),
		"Haft.SpecSectionRecordRef",
		specInput.RecordEntity,
		specReceipt,
	)
	addGoldenItem(
		t,
		builder,
		goldenconcernbundle.ItemProjectClaim,
		current.Environment(),
		"Haft.ProjectClaimRef",
		claim.entity,
		claimReceipt,
	)
	addGoldenEvidenceWorkItems(
		t,
		builder,
		current.Environment(),
		evidenceReceipt,
	)
	codeEntity, err := typedmemory.NewEntityID(
		"code-anchor:golden-concern-bundle",
	)
	mustProductionNoteNoError(t, err)
	addGoldenItem(
		t,
		builder,
		goldenconcernbundle.ItemCodeAnchor,
		current.Environment(),
		"Haft.CodeAnchorRef",
		codeEntity,
		codeReceipt,
	)

	bundle, err := builder.Build()
	mustProductionNoteNoError(t, err)
	if err := bundle.Verify(); err != nil {
		t.Fatalf("verify GoldenConcernBundle: %v", err)
	}
	reordered := rebuildGoldenBundleInReverseOrder(t, bundle)
	if !bytes.Equal(
		bundle.CanonicalBytes(),
		reordered.CanonicalBytes(),
	) ||
		bundle.Digest() != reordered.Digest() {
		t.Fatal(
			"GoldenConcernBundle identity changed with explanatory input order",
		)
	}
	if len(bundle.Items()) != 18 {
		t.Fatalf(
			"GoldenConcernBundle items = %d, want 18",
			len(bundle.Items()),
		)
	}
	if len(bundle.CanonicalBytes()) == 0 {
		t.Fatal("GoldenConcernBundle canonical bytes are empty")
	}
	digest := productionSHA256Digest(t, bundle.CanonicalBytes())
	if bundle.Digest() != digest {
		t.Fatal("GoldenConcernBundle digest does not commit to canonical bytes")
	}
	decoded, err := goldenconcernbundle.DecodeCanonical(
		bundle.CanonicalBytes(),
	)
	mustProductionNoteNoError(t, err)
	if decoded.Digest() != bundle.Digest() ||
		!bytes.Equal(
			decoded.CanonicalBytes(),
			bundle.CanonicalBytes(),
		) {
		t.Fatal(
			"GoldenConcernBundle changed across canonical export and rebuild",
		)
	}
	assertGoldenConcernCurrentResolution(
		t,
		ctx,
		fixture,
		resolver,
		baseTypeEnv,
		bundle,
	)
	beforeNeighborhoodRead := current.Snapshot().GraphRevision()
	assertGoldenBundleNeighborhoodProjection(t, bundle)
	afterNeighborhoodRead := loadProductionPortfolioSnapshot(
		t,
		ctx,
		baseLoader,
		fixture.project,
	)
	if afterNeighborhoodRead.Snapshot().GraphRevision() !=
		beforeNeighborhoodRead {
		t.Fatal("pure neighborhood projection changed canonical graph revision")
	}
	assertGoldenBundleDurableReread(
		t,
		ctx,
		fixture,
		bundle,
	)
}

func assertGoldenConcernCurrentResolution(
	t *testing.T,
	ctx context.Context,
	fixture genesisE2EFixture,
	resolver typedmemorystore.SelectedProjectTypeEnvRuntimeResolver,
	baseTypeEnv projectmemory.BaseTypeEnvLoader,
	bundle goldenconcernbundle.Bundle,
) {
	t.Helper()
	loader, err :=
		typedmemorystore.NewProjectAwareSQLiteCurrentProjectReadFrameLoader(
			fixture.database,
			baseTypeEnv,
			resolver,
		)
	mustProductionNoteNoError(t, err)
	frame, err := loader.LoadCurrentProjectReadFrame(
		ctx,
		fixture.project,
	)
	mustProductionNoteNoError(t, err)
	readRuntime, err := projectmemory.NewCurrentMemoryReadRuntime(
		fixture.project,
		loader,
	)
	mustProductionNoteNoError(t, err)
	index, err := projectmemory.BuildCurrentResolutionIndex(frame)
	mustProductionNoteNoError(t, err)
	query, err := memoryresolve.NewResolutionQuery(
		bundle.Concern().Reference().ReferenceID().String(),
	)
	mustProductionNoteNoError(t, err)
	contextScope, err := memoryresolve.NewExactContext(
		bundle.Snapshot().Context(),
	)
	mustProductionNoteNoError(t, err)
	request, err := memoryresolve.NewResolutionRequest(
		query,
		contextScope,
		index.SnapshotBasis(),
		8,
	)
	mustProductionNoteNoError(t, err)
	result, err := readRuntime.Resolve(ctx, request)
	mustProductionNoteNoError(t, err)
	exact, ok := result.(memoryresolve.ExactEntity)
	if !ok {
		t.Fatalf(
			"current concern resolution = %T, want ExactEntity",
			result,
		)
	}
	if exact.Entity().Entity() != bundle.Concern().Reference() ||
		exact.Entity().Context() != bundle.Snapshot().Context() ||
		exact.Interpretation().Relations() !=
			neighborhood.RelationsUnavailable ||
		exact.Interpretation().Authority() !=
			neighborhood.AuthorityNotGranted {
		t.Fatal(
			"current concern resolution lost identity/scope or implied relations/authority",
		)
	}
	resolutionResponse, err := projectmemory.EncodeResolutionReadResponse(
		exact,
	)
	mustProductionNoteNoError(t, err)
	assertGoldenMemoryReadEnvelope(
		t,
		resolutionResponse,
		typedmemorywire.ActionResolve,
		string(memoryresolve.ResultExactEntity),
		"",
		string(neighborhood.RelationsUnavailable),
	)
	profileRef, err := neighborhood.ParseProjectionProfileRef(
		"agent_orientation.v1",
	)
	mustProductionNoteNoError(t, err)
	view, err := neighborhood.NewNeighborhoodViewSpec(
		profileRef,
		neighborhood.KnownFacetKinds(),
		neighborhood.DetailEvidence,
		true,
	)
	mustProductionNoteNoError(t, err)
	readBudget, err := neighborhood.NewReadBudgetBuilder().
		SetMaxFacets(uint32(len(neighborhood.KnownFacetKinds()))).
		SetMaxItemsPerFacet(64).
		SetMaxRelationPathsPerItem(64).
		SetMaxCarrierExcerptCharacters(4096).
		SetMaxProvenanceDepth(4).
		Build()
	mustProductionNoteNoError(t, err)
	neighborhoodRequest, err :=
		neighborhood.NewNeighborhoodRequestBuilder().
			SetEntity(bundle.Concern().Reference()).
			SetContext(bundle.Snapshot().Context()).
			SetTypeEnv(index.SnapshotBasis().TypeEnv()).
			SetGraphRevision(index.SnapshotBasis().GraphRevision()).
			SetView(view).
			SetBudget(readBudget).
			Build()
	mustProductionNoteNoError(t, err)
	neighborhoodResult, err := readRuntime.Neighborhood(
		ctx,
		neighborhoodRequest,
	)
	mustProductionNoteNoError(t, err)
	exactNeighborhood, found :=
		neighborhoodResult.(neighborhood.ExactNeighborhood)
	if !found {
		t.Fatalf(
			"current neighborhood result = %T, want ExactNeighborhood",
			neighborhoodResult,
		)
	}
	if exactNeighborhood.ViewContext().Entity() !=
		bundle.Concern().Reference() ||
		exactNeighborhood.SnapshotBasis() != index.SnapshotBasis() {
		t.Fatal("current neighborhood changed exact concern or snapshot")
	}
	itemCount := 0
	codeAnchorFound := false
	for _, facet := range exactNeighborhood.Facets() {
		itemCount += len(facet.Items())
		for _, item := range facet.Items() {
			if item.ItemKind() == neighborhood.ItemCodeAnchor {
				codeAnchorFound = true
			}
		}
	}
	if itemCount != len(bundle.Items())-1 || !codeAnchorFound {
		t.Fatalf(
			"current neighborhood items = %d, code anchor = %t; want %d and true",
			itemCount,
			codeAnchorFound,
			len(bundle.Items())-1,
		)
	}
	neighborhoodResponse, err :=
		projectmemory.EncodeNeighborhoodReadResponse(exactNeighborhood)
	mustProductionNoteNoError(t, err)
	assertGoldenMemoryReadEnvelope(
		t,
		neighborhoodResponse,
		typedmemorywire.ActionNeighborhood,
		string(neighborhood.ResultExactNeighborhood),
		exactNeighborhood.Digest().String(),
		string(neighborhood.RelationsExactAtSnapshot),
	)
	assertGoldenArchitectureP2SProjection(
		t,
		ctx,
		frame,
		bundle,
		readRuntime,
		neighborhoodRequest,
		neighborhoodResponse,
	)
	recallQuery, err := scopedrecall.NewRecallQuery(
		"SS.constraints.typed-memory.001.D1",
	)
	mustProductionNoteNoError(t, err)
	candidateBudget, err := scopedrecall.NewCandidateBudget(8)
	mustProductionNoteNoError(t, err)
	recallResult, err := readRuntime.Recall(
		ctx,
		neighborhoodRequest,
		recallQuery,
		candidateBudget,
	)
	mustProductionNoteNoError(t, err)
	candidates, ok := recallResult.(scopedrecall.ScopedMemoryCandidateSet)
	if !ok {
		t.Fatalf(
			"current scoped recall result = %T, want ScopedMemoryCandidateSet",
			recallResult,
		)
	}
	specFound := false
	for _, candidate := range candidates.Candidates() {
		if candidate.Unit().ItemKind() == neighborhood.ItemSpecSection {
			specFound = true
		}
	}
	if !specFound ||
		candidates.Scope().Entity() != bundle.Concern().Reference() ||
		candidates.Interpretation().Relations() !=
			neighborhood.RelationsCandidateOnly {
		t.Fatal(
			"current scoped recall lost exact scope, spec match, or candidate-only relation posture",
		)
	}
	recallResponse, err := projectmemory.EncodeScopedRecallReadResponse(
		candidates,
	)
	mustProductionNoteNoError(t, err)
	assertGoldenMemoryReadEnvelope(
		t,
		recallResponse,
		typedmemorywire.ActionRecall,
		string(scopedrecall.ScopedResultCandidateSet),
		"",
		string(neighborhood.RelationsCandidateOnly),
	)

	staleRevision := typedmemory.NewGraphRevision(
		index.SnapshotBasis().GraphRevision().Value() - 1,
	)
	staleNeighborhoodRequest, err :=
		neighborhood.NewNeighborhoodRequestBuilder().
			SetEntity(bundle.Concern().Reference()).
			SetContext(bundle.Snapshot().Context()).
			SetTypeEnv(index.SnapshotBasis().TypeEnv()).
			SetGraphRevision(staleRevision).
			SetView(view).
			SetBudget(readBudget).
			Build()
	mustProductionNoteNoError(t, err)
	staleNeighborhood, err := readRuntime.Neighborhood(
		ctx,
		staleNeighborhoodRequest,
	)
	mustProductionNoteNoError(t, err)
	if _, ok := staleNeighborhood.(neighborhood.RetryRequiredResult); !ok {
		t.Fatalf(
			"stale neighborhood = %T, want RetryRequiredResult",
			staleNeighborhood,
		)
	}
	staleRecall, err := readRuntime.Recall(
		ctx,
		staleNeighborhoodRequest,
		recallQuery,
		candidateBudget,
	)
	mustProductionNoteNoError(t, err)
	if _, ok := staleRecall.(scopedrecall.ScopedRetryRequired); !ok {
		t.Fatalf(
			"stale scoped recall = %T, want ScopedRetryRequired",
			staleRecall,
		)
	}
}

func assertGoldenArchitectureP2SProjection(
	t *testing.T,
	ctx context.Context,
	frame typedmemorystore.CurrentProjectReadFrame,
	bundle goldenconcernbundle.Bundle,
	readRuntime projectmemory.CurrentMemoryReadRuntime,
	neighborhoodRequest neighborhood.NeighborhoodRequest,
	publicResponseBefore []byte,
) {
	t.Helper()
	model, found, err := projectmemory.BuildCurrentArchitectureP2S(
		frame,
		bundle.Concern().Reference(),
		bundle.Snapshot().Context(),
	)
	mustProductionNoteNoError(t, err)
	if !found {
		t.Fatal("GoldenConcernBundle architecture P2S concern was not found")
	}
	if model.Basis().GraphRevision() !=
		bundle.Snapshot().GraphRevision().Value() ||
		len(model.CanonicalBytes()) == 0 ||
		model.Digest() == "" {
		t.Fatal("architecture P2S projection lost its exact snapshot identity")
	}
	direct := map[architecturep2s.PositionKind]struct{}{
		architecturep2s.PositionAlternatives: {},
		architecturep2s.PositionComparison:   {},
		architecturep2s.PositionDecision:     {},
		architecturep2s.PositionWorkRecord:   {},
		architecturep2s.PositionWorkToChange: {},
		architecturep2s.PositionEvidence:     {},
	}
	for _, kind := range architecturep2s.PositionKinds() {
		position, present := model.Position(kind)
		if !present {
			t.Fatalf("architecture P2S position %q is absent", kind)
		}
		_, wantDirect := direct[kind]
		if wantDirect && position.Resolution() !=
			architecturep2s.ResolutionDirectClaim {
			t.Fatalf(
				"architecture P2S position %q = %q, want direct claim",
				kind,
				position.Resolution(),
			)
		}
		if !wantDirect && position.Resolution() !=
			architecturep2s.ResolutionMissing {
			t.Fatalf(
				"architecture P2S position %q = %q, want explicit missing",
				kind,
				position.Resolution(),
			)
		}
	}
	assertGoldenArchitectureSourceDock(
		t,
		model,
		architecturep2s.PositionPerformedWork,
	)
	assertGoldenArchitectureSourceDock(
		t,
		model,
		architecturep2s.PositionActualChange,
	)
	assertGoldenArchitectureSourceDock(
		t,
		model,
		architecturep2s.PositionActualStructure,
	)
	assertGoldenArchitectureSourceDock(
		t,
		model,
		architecturep2s.PositionTargetEffect,
	)
	repeated, repeatedFound, err := projectmemory.BuildCurrentArchitectureP2S(
		frame,
		bundle.Concern().Reference(),
		bundle.Snapshot().Context(),
	)
	mustProductionNoteNoError(t, err)
	if !repeatedFound || repeated.Digest() != model.Digest() ||
		!bytes.Equal(repeated.CanonicalBytes(), model.CanonicalBytes()) {
		t.Fatal("architecture P2S projection is not deterministic")
	}
	publicResultAfter, err := readRuntime.Neighborhood(
		ctx,
		neighborhoodRequest,
	)
	mustProductionNoteNoError(t, err)
	publicExactAfter, ok := publicResultAfter.(neighborhood.ExactNeighborhood)
	if !ok {
		t.Fatalf(
			"post-P2S neighborhood = %T, want ExactNeighborhood",
			publicResultAfter,
		)
	}
	publicResponseAfter, err :=
		projectmemory.EncodeNeighborhoodReadResponse(publicExactAfter)
	mustProductionNoteNoError(t, err)
	if !bytes.Equal(publicResponseBefore, publicResponseAfter) {
		t.Fatal("internal architecture P2S projection changed public memory bytes")
	}
}

func assertGoldenArchitectureSourceDock(
	t *testing.T,
	model architecturep2s.ReadModel,
	kind architecturep2s.PositionKind,
) {
	t.Helper()
	position, found := model.Position(kind)
	if !found {
		t.Fatalf("architecture P2S source-dock position %q is absent", kind)
	}
	missing, ok := position.(architecturep2s.MissingPosition)
	if !ok || len(missing.SourceDocks()) == 0 {
		t.Fatalf(
			"architecture P2S position %q promoted its source carrier or lost its dock",
			kind,
		)
	}
}

func assertGoldenMemoryReadEnvelope(
	t *testing.T,
	encoded []byte,
	action string,
	resultKind string,
	resultDigest string,
	relations string,
) {
	t.Helper()
	var envelope struct {
		ContractVersion string         `json:"contract_version"`
		Action          string         `json:"action"`
		ResultKind      string         `json:"result_kind"`
		ResultDigest    string         `json:"result_digest"`
		Result          map[string]any `json:"result"`
	}
	if err := json.Unmarshal(encoded, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.ContractVersion != typedmemorywire.ContractVersion ||
		envelope.Action != action ||
		envelope.ResultKind != resultKind ||
		envelope.ResultDigest != resultDigest {
		t.Fatalf("public memory-read envelope = %#v", envelope)
	}
	interpretation, ok :=
		envelope.Result["interpretation_contract"].(map[string]any)
	if !ok {
		t.Fatalf(
			"public memory-read interpretation = %#v",
			envelope.Result["interpretation_contract"],
		)
	}
	if interpretation["relational_records"] != relations ||
		interpretation["authority"] !=
			string(neighborhood.AuthorityNotGranted) ||
		interpretation["work_order"] !=
			string(neighborhood.WorkOrderNotImplied) {
		t.Fatalf(
			"public memory-read interpretation = %#v",
			interpretation,
		)
	}
}

func assertGoldenBundleNeighborhoodProjection(
	t *testing.T,
	bundle goldenconcernbundle.Bundle,
) {
	t.Helper()
	profileRef, err := neighborhood.ParseProjectionProfileRef(
		"agent_orientation.v1",
	)
	mustProductionNoteNoError(t, err)
	view, err := neighborhood.NewNeighborhoodViewSpec(
		profileRef,
		neighborhood.KnownFacetKinds(),
		neighborhood.DetailEvidence,
		true,
	)
	mustProductionNoteNoError(t, err)
	budget, err := neighborhood.NewReadBudgetBuilder().
		SetMaxFacets(uint32(len(neighborhood.KnownFacetKinds()))).
		SetMaxItemsPerFacet(64).
		SetMaxRelationPathsPerItem(64).
		SetMaxCarrierExcerptCharacters(4096).
		SetMaxProvenanceDepth(4).
		Build()
	mustProductionNoteNoError(t, err)
	request, err := neighborhood.NewNeighborhoodRequestBuilder().
		SetEntity(bundle.Concern().Reference()).
		SetContext(bundle.Snapshot().Context()).
		SetTypeEnv(bundle.Snapshot().TypeEnv()).
		SetGraphRevision(bundle.Snapshot().GraphRevision()).
		SetView(view).
		SetBudget(budget).
		Build()
	mustProductionNoteNoError(t, err)
	posture, valid := neighborhood.NewItemPostures(
		neighborhood.SemanticTypedActive,
		neighborhood.LifecycleActive,
		neighborhood.EvidenceUnknown,
		neighborhood.ProjectionCurrent,
	)
	if !valid {
		t.Fatal("GoldenConcernBundle explicit acceptance posture is invalid")
	}
	bindings := make(
		[]neighborhood.ExactPostureBinding,
		0,
		len(bundle.Items()),
	)
	for _, item := range bundle.Items() {
		binding, bindingErr := neighborhood.NewExactPostureBinding(
			item.Reference(),
			posture,
		)
		mustProductionNoteNoError(t, bindingErr)
		bindings = append(bindings, binding)
	}
	postureSet, err := neighborhood.NewExactPostureSet(bindings)
	mustProductionNoteNoError(t, err)
	input, err := neighborhood.AdaptGoldenConcernBundleAcceptance(
		bundle,
		request,
		postureSet,
	)
	mustProductionNoteNoError(t, err)
	left, err := neighborhood.Assemble(input)
	mustProductionNoteNoError(t, err)
	right, err := neighborhood.Assemble(input)
	mustProductionNoteNoError(t, err)
	if !bytes.Equal(left.CanonicalBytes(), right.CanonicalBytes()) ||
		left.Digest() != right.Digest() {
		t.Fatal("GoldenConcernBundle neighborhood projection is not byte-stable")
	}
	if left.ViewContext().Entity() != bundle.Concern().Reference() ||
		left.SnapshotBasis().GraphRevision() !=
			bundle.Snapshot().GraphRevision() {
		t.Fatal("GoldenConcernBundle neighborhood retargeted its concern")
	}
	if len(left.Facets()) != len(neighborhood.KnownFacetKinds()) ||
		len(left.ProjectionBasis().ItemBases()) != len(bundle.Items()) {
		t.Fatal("GoldenConcernBundle neighborhood lost a requested typed item")
	}
	if !left.Interpretation().HydrateBeforeReliance() {
		t.Fatal("unknown evidence posture was presented as reliance-ready")
	}
	incomplete, err := neighborhood.NewExactPostureSet(bindings[1:])
	mustProductionNoteNoError(t, err)
	if _, err := neighborhood.AdaptGoldenConcernBundleAcceptance(
		bundle,
		request,
		incomplete,
	); err == nil {
		t.Fatal("GoldenConcernBundle adapter inferred a missing item posture")
	}
}

func rebuildGoldenBundleInReverseOrder(
	t *testing.T,
	source goldenconcernbundle.Bundle,
) goldenconcernbundle.Bundle {
	t.Helper()
	builder := goldenconcernbundle.NewBuilder(source.ProjectID())
	builder.SetConcern(source.Concern())
	builder.SetSnapshot(source.Snapshot())
	admissions := source.AdapterAdmissions()
	for index := len(admissions) - 1; index >= 0; index-- {
		builder.AddAdapterAdmission(admissions[index])
	}
	items := source.Items()
	for index := len(items) - 1; index >= 0; index-- {
		item := items[index]
		if item.Role() == goldenconcernbundle.ItemEntityOfConcern {
			continue
		}
		spec, err := goldenconcernbundle.NewItemSpec(
			item.Role(),
			item.Reference(),
			item.AdmissionEventRef(),
		)
		mustProductionNoteNoError(t, err)
		builder.AddItem(spec)
	}
	bundle, err := builder.Build()
	mustProductionNoteNoError(t, err)
	return bundle
}

type goldenP4DecisionFixture struct {
	Schema             string                `json:"schema"`
	ID                 string                `json:"id"`
	Version            int                   `json:"version"`
	Status             artifact.Status       `json:"status"`
	Title              string                `json:"title"`
	Context            string                `json:"context"`
	DecisionSubjectRef string                `json:"decision_subject_ref"`
	ProblemRefs        []string              `json:"problem_refs"`
	SelectedTitle      string                `json:"selected_title"`
	SelectionPolicy    string                `json:"selection_policy"`
	WhySelected        string                `json:"why_selected"`
	ChoiceResult       artifact.ChoiceResult `json:"choice_result"`
}

func loadGoldenP4DecisionFixture(
	t *testing.T,
) goldenP4DecisionFixture {
	t.Helper()
	canonical, err := os.ReadFile(
		"testdata/p4_decision_choice_source.json",
	)
	mustProductionNoteNoError(t, err)
	decoder := json.NewDecoder(bytes.NewReader(canonical))
	decoder.DisallowUnknownFields()
	fixture := goldenP4DecisionFixture{}
	mustProductionNoteNoError(t, decoder.Decode(&fixture))
	if fixture.Schema !=
		"haft.test-fixture.p4-decision-choice-source/v1" {
		t.Fatalf("P4 fixture schema = %q", fixture.Schema)
	}
	trailing := json.RawMessage{}
	if err := decoder.Decode(&trailing); err == nil {
		t.Fatal("P4 fixture contains more than one JSON value")
	}
	if fixture.ID != "dec-20260716-11f33e36" ||
		len(fixture.ChoiceResult.OptionSet) != 6 ||
		fixture.ChoiceResult.NextMove != artifact.ChoiceNextMoveChooseNow {
		t.Fatal("P4 fixture no longer identifies the manually bound six-option choice")
	}
	return fixture
}

func seedGoldenP4DecisionChoice(
	t *testing.T,
	ctx context.Context,
	fixture genesisE2EFixture,
	frozen goldenP4DecisionFixture,
) decisionrecordadapter.ExistingDecisionChoiceSource {
	t.Helper()
	choice := frozen.ChoiceResult
	fields := artifact.DecisionFields{
		ProblemRefs:        append([]string(nil), frozen.ProblemRefs...),
		DecisionSubjectRef: frozen.DecisionSubjectRef,
		ChoiceResult:       &choice,
		SelectedTitle:      frozen.SelectedTitle,
		WhySelected:        frozen.WhySelected,
		SelectionPolicy:    frozen.SelectionPolicy,
	}
	structured, err := json.Marshal(fields)
	mustProductionNoteNoError(t, err)
	record := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:      frozen.ID,
			Kind:    artifact.KindDecisionRecord,
			Version: frozen.Version,
			Status:  frozen.Status,
			Context: frozen.Context,
			Title:   frozen.Title,
		},
		Body:           "Frozen real P4 typed-memory architecture decision",
		StructuredData: string(structured),
	}
	store := artifact.NewStore(fixture.database)
	mustProductionNoteNoError(t, store.Create(ctx, record))
	source, err := decisionrecordadapter.LoadExistingDecisionChoiceSource(
		ctx,
		store,
		frozen.ID,
	)
	mustProductionNoteNoError(t, err)
	return source
}

func goldenP4DecisionChoiceDraft(
	t *testing.T,
	fixture genesisE2EFixture,
	current typedmemorystore.CurrentProjectSnapshot,
	contextRef typedmemory.BoundedContextRef,
	concern typedmemory.DeclareEntity,
	source decisionrecordadapter.ExistingDecisionChoiceSource,
	frozen goldenP4DecisionFixture,
	problemRef typedmemory.PersistedRef,
	optionRefs map[string]typedmemory.PersistedRef,
	comparisonRef typedmemory.PersistedRef,
) decisionrecordadapter.Draft {
	t.Helper()
	entity, err := typedmemory.NewEntityID(source.DecisionRecordRef())
	mustProductionNoteNoError(t, err)
	local, err := typedmemory.NewBatchLocalRef(source.DecisionRecordRef())
	mustProductionNoteNoError(t, err)
	label, err := typedmemory.NewEntityLabel(source.Title())
	mustProductionNoteNoError(t, err)
	assertion, err := typedmemory.NewAssertionID(
		"assertion:golden-p4-decision-choice",
	)
	mustProductionNoteNoError(t, err)
	gamma, err := typedmemory.NewGammaPoint(
		time.Date(2026, 7, 18, 15, 10, 0, 0, time.UTC),
	)
	mustProductionNoteNoError(t, err)
	contextSlice, err := typedmemory.NewContextSlice(
		typedmemory.ContextSliceInput{
			Context:   contextRef,
			GammaTime: gamma,
		},
	)
	mustProductionNoteNoError(t, err)
	options := make(
		[]decisionrecordadapter.DecisionOptionBinding,
		0,
		len(frozen.ChoiceResult.OptionSet),
	)
	for _, option := range frozen.ChoiceResult.OptionSet {
		binding, bindingErr :=
			decisionrecordadapter.NewDecisionOptionBinding(
				option,
				optionRefs[option],
			)
		mustProductionNoteNoError(t, bindingErr)
		options = append(options, binding)
	}
	problem, err := decisionrecordadapter.NewExactProjectRecordReference(
		frozen.ProblemRefs[0],
		problemRef,
	)
	mustProductionNoteNoError(t, err)
	comparison, err :=
		decisionrecordadapter.NewExactProjectRecordReference(
			"comparison:golden-p4",
			comparisonRef,
		)
	mustProductionNoteNoError(t, err)
	contextProjection, err :=
		decisionrecordadapter.NewLegacyContextProjection(
			source,
			contextRef,
		)
	mustProductionNoteNoError(t, err)
	concernBinding := productionNoteConcernBinding(
		t,
		current,
		concern.Entity(),
		contextRef,
	)
	input := decisionrecordadapter.ProjectionDraftInput{
		ProjectID:         fixture.project,
		RecordEntity:      entity,
		RecordLocalRef:    local,
		RecordLabel:       label,
		AssertionID:       assertion,
		ContextSlice:      contextSlice,
		Source:            source,
		ContextProjection: contextProjection,
		Concern:           concernBinding,
		Problem:           problem,
		Portfolio:         decisionrecordadapter.NoProjectRecordReference(),
		Options:           options,
		Comparison:        comparison,
	}
	draft, err := decisionrecordadapter.NewDraft(input)
	mustProductionNoteNoError(t, err)
	return draft
}

func goldenHaftSoftwareSystemDeclaration(
	t *testing.T,
	contextRef typedmemory.BoundedContextRef,
) typedmemory.DeclareEntity {
	t.Helper()
	entity, err := typedmemory.NewEntityID("HaftSoftwareSystem")
	mustProductionNoteNoError(t, err)
	local, err := typedmemory.NewBatchLocalRef("HaftSoftwareSystem")
	mustProductionNoteNoError(t, err)
	label, err := typedmemory.NewEntityLabel("Haft software system")
	mustProductionNoteNoError(t, err)
	provenance, err := typedmemory.NewProvenanceRef(
		"memory:test:golden-haft-software-system",
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

func goldenOptionToken(index int) string {
	return "golden-p4-option-" + string(rune('a'+index))
}

func goldenPersistedReference(
	t *testing.T,
	environment typedmemory.TypeEnv,
	refKindRaw string,
	entity typedmemory.EntityID,
) typedmemory.PersistedRef {
	t.Helper()
	refKindID, err := typedmemory.NewRefKindID(refKindRaw)
	mustProductionNoteNoError(t, err)
	refKind, err := typedmemory.NewRefKindRef(
		environment.Ref(),
		refKindID,
	)
	mustProductionNoteNoError(t, err)
	referenceID, err := typedmemory.NewReferenceID(entity.String())
	mustProductionNoteNoError(t, err)
	reference, err := typedmemory.NewPersistedRef(
		refKind,
		referenceID,
	)
	mustProductionNoteNoError(t, err)
	return reference
}

func goldenProjectClaimStatesSpecCandidate(
	t *testing.T,
	project projectidentity.ProjectID,
	current typedmemorystore.CurrentProjectSnapshot,
	concern typedmemory.DeclareEntity,
	contextRef typedmemory.BoundedContextRef,
	specRef typedmemory.PersistedRef,
) productionProjectClaimCandidate {
	t.Helper()
	candidate := productionProjectClaimAtConcernCandidate(
		t,
		project,
		current,
		concern,
		contextRef,
	)
	signatureID, err := typedmemory.NewSignatureID(
		"Haft.RecordStatesClaim",
	)
	mustProductionNoteNoError(t, err)
	signatureRef, err := typedmemory.NewRelationSignatureRef(
		current.Environment().Ref(),
		signatureID,
	)
	mustProductionNoteNoError(t, err)
	signature, found :=
		current.Environment().RelationSignature(signatureRef)
	if !found {
		t.Fatal("selected TypeEnv has no Haft.RecordStatesClaim")
	}
	claimLocal, err := typedmemory.NewBatchLocalRef(
		candidate.entity.String(),
	)
	mustProductionNoteNoError(t, err)
	claimRef := productionLocalReference(
		t,
		current.Environment(),
		"Haft.ProjectClaimRef",
		claimLocal,
	)
	bindings := []typedmemory.CandidateSlotBinding{
		productionReferenceBinding(
			t,
			"Haft.RecordStatesClaim.ProjectRecordSlot",
			specRef,
		),
		productionReferenceBinding(
			t,
			"Haft.RecordStatesClaim.ProjectClaimSlot",
			claimRef,
		),
	}
	assertion, err := typedmemory.NewAssertionID(
		"assertion:golden-spec-states-project-claim",
	)
	mustProductionNoteNoError(t, err)
	provenance, err := typedmemory.NewProvenanceRef(
		"memory:test:golden-spec-states-project-claim",
	)
	mustProductionNoteNoError(t, err)
	contextSlice := productionCodeAnchorContextSlice(t, contextRef)
	modality := typedmemory.NewAffirmsObtaining()
	relation, err := typedmemory.NewRelationalAssertionCandidate(
		typedmemory.RelationalAssertionCandidateInput{
			Assertion:  assertion,
			Signature:  signature.Ref(),
			Slice:      contextSlice,
			Modality:   modality,
			Bindings:   bindings,
			Provenance: provenance,
		},
	)
	mustProductionNoteNoError(t, err)
	change, err := typedmemory.NewAssertRelation(relation)
	mustProductionNoteNoError(t, err)
	changes := candidate.changeSet.Changes()
	changes = append(changes, change)
	changeSet, err := typedmemory.NewMemoryChangeSet(changes)
	mustProductionNoteNoError(t, err)
	candidate.changeSet = changeSet
	return candidate
}

func goldenCodeAnchorDraft(
	t *testing.T,
	project projectidentity.ProjectID,
	contextRef typedmemory.BoundedContextRef,
	claim codeanchoradapter.ExactReferenceBinding,
	work codeanchoradapter.ExactReferenceBinding,
) codeanchoradapter.Draft {
	t.Helper()
	target, err := typedmemorycandidatecodec.NewSymbolCodeAnchorTarget(
		"internal/projectmemory/goldenconcernbundle/model.go",
		"Builder.Build",
	)
	mustProductionNoteNoError(t, err)
	locator, err := typedmemorycandidatecodec.NewCodeAnchorLocator(
		"github.com/m0n0x41d/haft",
		"0f9c64ef",
		target,
	)
	mustProductionNoteNoError(t, err)
	claimAssertion, err := typedmemory.NewAssertionID(
		"assertion:golden-code-realizes-project-claim",
	)
	mustProductionNoteNoError(t, err)
	claimLink, err := codeanchoradapter.NewClaimLink(
		claimAssertion,
		claim,
	)
	mustProductionNoteNoError(t, err)
	workAssertion, err := typedmemory.NewAssertionID(
		"assertion:golden-code-changed-by-work",
	)
	mustProductionNoteNoError(t, err)
	workLink, err := codeanchoradapter.NewWorkLink(
		workAssertion,
		work,
	)
	mustProductionNoteNoError(t, err)
	entity, err := typedmemory.NewEntityID(
		"code-anchor:golden-concern-bundle",
	)
	mustProductionNoteNoError(t, err)
	local, err := typedmemory.NewBatchLocalRef(
		"code-anchor:golden-concern-bundle",
	)
	mustProductionNoteNoError(t, err)
	label, err := typedmemory.NewEntityLabel(
		"GoldenConcernBundle builder at exact repository revision",
	)
	mustProductionNoteNoError(t, err)
	definition, err := typedmemory.NewAssertionID(
		"assertion:golden-code-anchor-definition",
	)
	mustProductionNoteNoError(t, err)
	provenance, err := typedmemory.NewProvenanceRef(
		"memory:test:golden-code-anchor",
	)
	mustProductionNoteNoError(t, err)
	links := []codeanchoradapter.SemanticLink{
		claimLink,
		workLink,
	}
	input := codeanchoradapter.DraftInput{
		ProjectID:             project,
		AnchorEntity:          entity,
		AnchorLocalRef:        local,
		AnchorLabel:           label,
		DefinitionAssertionID: definition,
		ContextSlice:          productionCodeAnchorContextSlice(t, contextRef),
		Locator:               codeanchoradapter.NewExactLocator(locator),
		Links:                 links,
		Provenance:            provenance,
	}
	draft, err := codeanchoradapter.NewDraft(input)
	mustProductionNoteNoError(t, err)
	return draft
}

func addGoldenItem(
	t *testing.T,
	builder *goldenconcernbundle.Builder,
	role goldenconcernbundle.ItemRole,
	environment typedmemory.TypeEnv,
	refKindRaw string,
	entity typedmemory.EntityID,
	receipt typedmemorystore.CommitReceipt,
) {
	t.Helper()
	reference := goldenPersistedReference(
		t,
		environment,
		refKindRaw,
		entity,
	)
	item, err := goldenconcernbundle.NewItemSpec(
		role,
		reference,
		receipt.EventRef(),
	)
	mustProductionNoteNoError(t, err)
	builder.AddItem(item)
}

func addGoldenEvidenceWorkItems(
	t *testing.T,
	builder *goldenconcernbundle.Builder,
	environment typedmemory.TypeEnv,
	receipt typedmemorystore.CommitReceipt,
) {
	t.Helper()
	specs := []struct {
		role    goldenconcernbundle.ItemRole
		refKind string
		entity  string
	}{
		{
			role:    goldenconcernbundle.ItemEvidenceRecord,
			refKind: "Haft.EvidenceRecordRef",
			entity:  "evidence-work:production-evidence",
		},
		{
			role:    goldenconcernbundle.ItemSupportingEpistemeRecord,
			refKind: "Haft.SupportingEpistemeRecordRef",
			entity:  "evidence-work:production-supporting",
		},
		{
			role:    goldenconcernbundle.ItemWorkRecord,
			refKind: "Haft.WorkRecordRef",
			entity:  "evidence-work:production-work",
		},
		{
			role:    goldenconcernbundle.ItemPerformedWorkOccurrence,
			refKind: "Haft.PerformedWorkOccurrenceRef",
			entity:  "evidence-work:production-occurrence",
		},
	}
	for _, spec := range specs {
		entity, err := typedmemory.NewEntityID(spec.entity)
		mustProductionNoteNoError(t, err)
		addGoldenItem(
			t,
			builder,
			spec.role,
			environment,
			spec.refKind,
			entity,
			receipt,
		)
	}
}

func assertGoldenBundleDurableReread(
	t *testing.T,
	ctx context.Context,
	fixture genesisE2EFixture,
	bundle goldenconcernbundle.Bundle,
) {
	t.Helper()
	transaction, err := sqlitetransaction.BeginRead(
		ctx,
		fixture.database,
	)
	mustProductionNoteNoError(t, err)
	observation, err :=
		typedmemorystore.LoadCurrentGraphRevalidationBasisTx(
			ctx,
			transaction,
			fixture.project,
		)
	mustProductionNoteNoError(t, err)
	finish := transaction.Commit(ctx)
	if !finish.Succeeded() {
		t.Fatalf("commit GoldenConcernBundle durable reread: %v", finish.Err())
	}
	if observation.GraphSnapshotBasis().GraphRevision() !=
		bundle.Snapshot().GraphRevision() {
		t.Fatal("GoldenConcernBundle reread observed another graph revision")
	}
	required := map[string]bool{
		"Haft.ProblemCardAtConcern":              false,
		"Haft.NoteAtConcern":                     false,
		"Haft.SolutionPortfolioAtConcern":        false,
		"Haft.PortfolioComparison":               false,
		"Haft.DecisionChoiceAtConcern":           false,
		"Haft.SpecSectionAtConcern":              false,
		"Haft.ProjectClaimAtConcern":             false,
		"Haft.RecordStatesClaim":                 false,
		"Haft.SupportingEpistemeRecordAtConcern": false,
		"Haft.WorkOccurrenceRecord":              false,
		"Haft.EvidenceUse":                       false,
		"Haft.CodeAnchorDefinition":              false,
		"Haft.CodeRealizesClaim":                 false,
		"Haft.CodeChangedByWork":                 false,
	}
	for _, active := range observation.ActiveAssertions().Relations() {
		carrier := productionFreshCurrentAssertionCarrier(t, active)
		signature := carrier.Signature().ID().String()
		if _, found := required[signature]; found {
			required[signature] = true
		}
	}
	for signature, found := range required {
		if !found {
			t.Fatalf(
				"GoldenConcernBundle durable graph omitted %s",
				signature,
			)
		}
	}
}
