package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projectmemory"
	"github.com/m0n0x41d/haft/internal/projectmemory/decisionrecordadapter"
	"github.com/m0n0x41d/haft/internal/projectmemory/noteadapter"
	"github.com/m0n0x41d/haft/internal/projectmemory/portfoliocomparisonadapter"
	"github.com/m0n0x41d/haft/internal/projectmemory/recordatconcern"
	"github.com/m0n0x41d/haft/internal/projectmemory/solutionportfolioadapter"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
	"github.com/m0n0x41d/haft/internal/typedmemoryvalidation"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

func TestProductionPortfolioAndComparisonPreserveOptionsWithoutSelectingWinner(
	t *testing.T,
) {
	fixture := newProductionNoteSelectedFixture(t)
	ctx := context.Background()
	selection, err := fixture.service.SelectGenesis(
		ctx,
		genesisSelectionInput(fixture),
	)
	mustProductionNoteNoError(t, err)
	fresh, ok := selection.(projecttypeenvselectioneffect.FreshlyCommitted)
	if !ok {
		t.Fatalf(
			"production TypeEnv selection = %T, want FreshlyCommitted",
			selection,
		)
	}
	selected := fresh.Closure().CommittedGraphRevision()
	resolver := genesisE2EProjectRuntimeResolver(t, fixture)
	baseLoader, err :=
		typedmemorystore.NewProjectAwareSQLiteCurrentProjectSnapshotLoader(
			fixture.database,
			projectmemory.NewBaseTypeEnvLoader(),
			resolver,
		)
	mustProductionNoteNoError(t, err)
	clock := &genesisE2EClock{
		value: time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC),
	}
	contextRef, err := typedmemory.NewBoundedContextRef("haft-project")
	mustProductionNoteNoError(t, err)
	concern := productionNoteConcernDeclaration(t, contextRef)
	concernReceipt := admitProductionPortfolioConcern(
		t,
		ctx,
		fixture,
		resolver,
		baseLoader,
		clock,
		concern,
	)
	if concernReceipt.GraphRevision().Value() != selected.Value()+1 {
		t.Fatalf(
			"concern revision = %d, want %d",
			concernReceipt.GraphRevision().Value(),
			selected.Value()+1,
		)
	}

	current := loadProductionPortfolioSnapshot(
		t,
		ctx,
		baseLoader,
		fixture.project,
	)
	optionAInput := productionPortfolioRecordInput(
		t,
		fixture.project,
		current.Environment(),
		contextRef,
		"option-a",
		"Option A",
	)
	optionADraft, err := noteadapter.NewDraft(optionAInput)
	mustProductionNoteNoError(t, err)
	optionA := mustProductionRecordCandidate(
		t,
		noteadapter.Adapt(
			optionADraft,
			productionNoteExactRuntime(t, fixture, current),
			productionNoteConcernBinding(
				t,
				current,
				concern.Entity(),
				contextRef,
			),
		),
	)
	_, optionAReceipt := admitProductionPortfolioRecord(
		t,
		ctx,
		fixture,
		resolver,
		baseLoader,
		clock,
		optionA,
		"production-portfolio-option-a",
	)

	current = loadProductionPortfolioSnapshot(
		t,
		ctx,
		baseLoader,
		fixture.project,
	)
	optionBInput := productionPortfolioRecordInput(
		t,
		fixture.project,
		current.Environment(),
		contextRef,
		"option-b",
		"Option B",
	)
	optionBDraft, err := noteadapter.NewDraft(optionBInput)
	mustProductionNoteNoError(t, err)
	optionB := mustProductionRecordCandidate(
		t,
		noteadapter.Adapt(
			optionBDraft,
			productionNoteExactRuntime(t, fixture, current),
			productionNoteConcernBinding(
				t,
				current,
				concern.Entity(),
				contextRef,
			),
		),
	)
	_, optionBReceipt := admitProductionPortfolioRecord(
		t,
		ctx,
		fixture,
		resolver,
		baseLoader,
		clock,
		optionB,
		"production-portfolio-option-b",
	)
	if optionBReceipt.GraphRevision().Value() !=
		optionAReceipt.GraphRevision().Value()+1 {
		t.Fatal("option admissions did not advance one exact graph revision")
	}

	current = loadProductionPortfolioSnapshot(
		t,
		ctx,
		baseLoader,
		fixture.project,
	)
	optionARef := productionProjectRecordReference(
		t,
		current.Environment(),
		optionADraft.RecordEntity(),
	)
	optionBRef := productionProjectRecordReference(
		t,
		current.Environment(),
		optionBDraft.RecordEntity(),
	)
	portfolioRecord := productionPortfolioRecordInput(
		t,
		fixture.project,
		current.Environment(),
		contextRef,
		"portfolio",
		"Solution portfolio",
	)
	if _, err := solutionportfolioadapter.NewDraft(
		solutionportfolioadapter.DraftInput{
			Record:  portfolioRecord,
			Options: []typedmemory.PersistedRef{optionARef},
		},
	); err == nil {
		t.Fatal("SolutionPortfolio draft accepted fewer than two options")
	}
	portfolioDraft, err := solutionportfolioadapter.NewDraft(
		solutionportfolioadapter.DraftInput{
			Record:  portfolioRecord,
			Options: []typedmemory.PersistedRef{optionBRef, optionARef},
		},
	)
	mustProductionNoteNoError(t, err)
	portfolioReversed, err := solutionportfolioadapter.NewDraft(
		solutionportfolioadapter.DraftInput{
			Record:  portfolioRecord,
			Options: []typedmemory.PersistedRef{optionARef, optionBRef},
		},
	)
	mustProductionNoteNoError(t, err)
	exactRuntime := productionNoteExactRuntime(t, fixture, current)
	exactConcern := productionNoteConcernBinding(
		t,
		current,
		concern.Entity(),
		contextRef,
	)
	portfolio := mustProductionRecordCandidate(
		t,
		solutionportfolioadapter.Adapt(
			portfolioDraft,
			exactRuntime,
			exactConcern,
		),
	)
	reversedPortfolio := mustProductionRecordCandidate(
		t,
		solutionportfolioadapter.Adapt(
			portfolioReversed,
			exactRuntime,
			exactConcern,
		),
	)
	assertProductionCandidateIdentityEqual(
		t,
		portfolio,
		reversedPortfolio,
		"portfolio option order",
	)
	assertProductionReferenceSlot(
		t,
		portfolio,
		"Haft.SolutionPortfolioAtConcern.OptionSlot",
		[]typedmemory.PersistedRef{optionARef, optionBRef},
	)
	portfolioValid, portfolioReceipt := admitProductionPortfolioRecord(
		t,
		ctx,
		fixture,
		resolver,
		baseLoader,
		clock,
		portfolio,
		"production-solution-portfolio",
	)
	assertProductionSelectedConstraintPresent(
		t,
		portfolioValid,
		current.Environment(),
		"Haft.Constraint.SolutionPortfolioAtConcern.OptionSlot.CardinalityV1",
	)

	current = loadProductionPortfolioSnapshot(
		t,
		ctx,
		baseLoader,
		fixture.project,
	)
	portfolioRef := productionProjectRecordReference(
		t,
		current.Environment(),
		portfolioDraft.Record().RecordEntity(),
	)
	comparisonRecord := productionPortfolioRecordInput(
		t,
		fixture.project,
		current.Environment(),
		contextRef,
		"comparison",
		"Portfolio comparison",
	)
	outsideSubsetDraft, err := portfoliocomparisonadapter.NewDraft(
		portfoliocomparisonadapter.DraftInput{
			Record:              comparisonRecord,
			Portfolio:           portfolioRef,
			ComparedOptions:     []typedmemory.PersistedRef{optionARef, optionBRef},
			NonDominatedOptions: []typedmemory.PersistedRef{portfolioRef},
		},
	)
	mustProductionNoteNoError(t, err)
	outsideSubset := mustProductionRecordCandidate(
		t,
		portfoliocomparisonadapter.Adapt(
			outsideSubsetDraft,
			productionNoteExactRuntime(t, fixture, current),
			productionNoteConcernBinding(
				t,
				current,
				concern.Entity(),
				contextRef,
			),
		),
	)
	outsideEvaluation := evaluateProductionPortfolioRecord(
		t,
		ctx,
		fixture,
		resolver,
		baseLoader,
		clock,
		outsideSubset,
	)
	outsideInvalid, ok := outsideEvaluation.outcome.(typedmemoryvalidation.InvalidOutcome)
	if !ok {
		t.Fatalf(
			"outside-subset comparison validation = %T, want InvalidOutcome",
			outsideEvaluation.outcome,
		)
	}
	assertProductionDiagnosticCode(
		t,
		outsideInvalid.Diagnostics(),
		typedmemory.DiagnosticReferenceSubsetMismatch,
	)
	comparisonDraft, err := portfoliocomparisonadapter.NewDraft(
		portfoliocomparisonadapter.DraftInput{
			Record:              comparisonRecord,
			Portfolio:           portfolioRef,
			ComparedOptions:     []typedmemory.PersistedRef{optionBRef, optionARef},
			NonDominatedOptions: []typedmemory.PersistedRef{optionARef},
		},
	)
	mustProductionNoteNoError(t, err)
	comparisonReversed, err := portfoliocomparisonadapter.NewDraft(
		portfoliocomparisonadapter.DraftInput{
			Record:              comparisonRecord,
			Portfolio:           portfolioRef,
			ComparedOptions:     []typedmemory.PersistedRef{optionARef, optionBRef},
			NonDominatedOptions: []typedmemory.PersistedRef{optionARef},
		},
	)
	mustProductionNoteNoError(t, err)
	exactRuntime = productionNoteExactRuntime(t, fixture, current)
	exactConcern = productionNoteConcernBinding(
		t,
		current,
		concern.Entity(),
		contextRef,
	)
	comparison := mustProductionRecordCandidate(
		t,
		portfoliocomparisonadapter.Adapt(
			comparisonDraft,
			exactRuntime,
			exactConcern,
		),
	)
	reversedComparison := mustProductionRecordCandidate(
		t,
		portfoliocomparisonadapter.Adapt(
			comparisonReversed,
			exactRuntime,
			exactConcern,
		),
	)
	assertProductionCandidateIdentityEqual(
		t,
		comparison,
		reversedComparison,
		"comparison option order",
	)
	assertProductionReferenceSlot(
		t,
		comparison,
		"Haft.PortfolioComparison.ComparedOptionSlot",
		[]typedmemory.PersistedRef{optionARef, optionBRef},
	)
	assertProductionReferenceSlot(
		t,
		comparison,
		"Haft.PortfolioComparison.NonDominatedOptionSlot",
		[]typedmemory.PersistedRef{optionARef},
	)
	assertProductionComparisonHasNoWinnerSlot(t, comparison)
	comparisonValid, comparisonReceipt := admitProductionPortfolioRecord(
		t,
		ctx,
		fixture,
		resolver,
		baseLoader,
		clock,
		comparison,
		"production-portfolio-comparison",
	)
	assertProductionSelectedConstraintPresent(
		t,
		comparisonValid,
		current.Environment(),
		"Haft.Constraint.PortfolioComparison.NonDominatedSubsetV1",
	)
	if comparisonReceipt.GraphRevision().Value() !=
		portfolioReceipt.GraphRevision().Value()+1 {
		t.Fatal("comparison admission did not immediately follow the portfolio")
	}
	current = loadProductionPortfolioSnapshot(
		t,
		ctx,
		baseLoader,
		fixture.project,
	)
	comparisonRef := productionProjectRecordReference(
		t,
		current.Environment(),
		comparisonDraft.Record().RecordEntity(),
	)
	decisionCountBeforeSource := countProductionDecisionRecords(
		t,
		ctx,
		fixture,
	)
	decisionSource := seedProductionExistingDecisionChoice(
		t,
		ctx,
		fixture,
		contextRef,
	)
	decisionCountAfterSource := countProductionDecisionRecords(
		t,
		ctx,
		fixture,
	)
	if decisionCountAfterSource != decisionCountBeforeSource+1 {
		t.Fatal("DecisionRecord source fixture did not create one exact source record")
	}
	decisionDraft := productionDecisionChoiceDraft(
		t,
		fixture,
		current,
		contextRef,
		concern,
		decisionSource,
		optionARef,
		optionBRef,
		comparisonRef,
		false,
	)
	reversedDecisionDraft := productionDecisionChoiceDraft(
		t,
		fixture,
		current,
		contextRef,
		concern,
		decisionSource,
		optionARef,
		optionBRef,
		comparisonRef,
		true,
	)
	decision := mustProductionRecordCandidate(
		t,
		decisionrecordadapter.Adapt(
			decisionDraft,
			productionNoteExactRuntime(t, fixture, current),
		),
	)
	reversedDecision := mustProductionRecordCandidate(
		t,
		decisionrecordadapter.Adapt(
			reversedDecisionDraft,
			productionNoteExactRuntime(t, fixture, current),
		),
	)
	assertProductionCandidateIdentityEqual(
		t,
		decision,
		reversedDecision,
		"DecisionRecord option mapping order",
	)
	assertProductionReferenceSlot(
		t,
		decision,
		"Haft.DecisionChoiceAtConcern.OptionSlot",
		[]typedmemory.PersistedRef{optionARef, optionBRef},
	)
	assertProductionReferenceSlot(
		t,
		decision,
		"Haft.DecisionChoiceAtConcern.ChosenOptionSlot",
		[]typedmemory.PersistedRef{optionARef},
	)
	assertProductionReferenceSlot(
		t,
		decision,
		"Haft.DecisionChoiceAtConcern.RejectedOptionSlot",
		[]typedmemory.PersistedRef{optionBRef},
	)
	assertProductionReferenceSlot(
		t,
		decision,
		"Haft.DecisionChoiceAtConcern.ComparisonRecordSlot",
		[]typedmemory.PersistedRef{comparisonRef},
	)
	decisionValid, decisionReceipt := admitProductionPortfolioRecord(
		t,
		ctx,
		fixture,
		resolver,
		baseLoader,
		clock,
		decision,
		"production-decision-choice",
	)
	assertProductionSelectedConstraintPresent(
		t,
		decisionValid,
		current.Environment(),
		"Haft.Constraint.DecisionChoice.OptionPartitionV1",
	)
	if decisionReceipt.GraphRevision().Value() !=
		comparisonReceipt.GraphRevision().Value()+1 {
		t.Fatal("decision projection did not immediately follow the comparison")
	}
	if countProductionDecisionRecords(t, ctx, fixture) !=
		decisionCountAfterSource {
		t.Fatal("typed DecisionRecord projection created another legacy DecisionRecord")
	}
	assertProductionPortfolioRelationsReread(
		t,
		ctx,
		fixture,
	)
}

func seedProductionExistingDecisionChoice(
	t *testing.T,
	ctx context.Context,
	fixture genesisE2EFixture,
	contextRef typedmemory.BoundedContextRef,
) decisionrecordadapter.ExistingDecisionChoiceSource {
	t.Helper()
	choice := &artifact.ChoiceResult{
		SubjectRef:      "operator",
		OptionSet:       []string{"Option B", "Option A"},
		ComparisonBasis: []string{"Option A remains the selected reversible choice"},
		ChoiceRule:      "prefer the reversible option",
		NextMove:        artifact.ChoiceNextMoveChooseNow,
		VariantRef:      "Option A",
		Reason:          "Option A is reversible",
		ReopenCondition: "new evidence invalidates the comparison basis",
	}
	fields := artifact.DecisionFields{
		DecisionSubjectRef: "operator",
		ChoiceResult:       choice,
		SelectedTitle:      "Option A",
		WhySelected:        "Option A is reversible",
		SelectionPolicy:    "prefer the reversible option",
	}
	structured, err := json.Marshal(fields)
	mustProductionNoteNoError(t, err)
	record := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:      "dec-20260718-production-choice-12345678",
			Kind:    artifact.KindDecisionRecord,
			Version: 1,
			Status:  artifact.StatusActive,
			Context: contextRef.String(),
			Title:   "Option A",
		},
		Body:           "Existing manually bound DecisionRecord fixture",
		StructuredData: string(structured),
	}
	store := artifact.NewStore(fixture.database)
	mustProductionNoteNoError(t, store.Create(ctx, record))
	source, err := decisionrecordadapter.LoadExistingDecisionChoiceSource(
		ctx,
		store,
		record.Meta.ID,
	)
	mustProductionNoteNoError(t, err)
	return source
}

func productionDecisionChoiceDraft(
	t *testing.T,
	fixture genesisE2EFixture,
	current typedmemorystore.CurrentProjectSnapshot,
	contextRef typedmemory.BoundedContextRef,
	concern typedmemory.DeclareEntity,
	source decisionrecordadapter.ExistingDecisionChoiceSource,
	optionARef typedmemory.PersistedRef,
	optionBRef typedmemory.PersistedRef,
	comparisonRef typedmemory.PersistedRef,
	reversed bool,
) decisionrecordadapter.Draft {
	t.Helper()
	entity, err := typedmemory.NewEntityID(source.DecisionRecordRef())
	mustProductionNoteNoError(t, err)
	localRef, err := typedmemory.NewBatchLocalRef(source.DecisionRecordRef())
	mustProductionNoteNoError(t, err)
	label, err := typedmemory.NewEntityLabel(source.Title())
	mustProductionNoteNoError(t, err)
	assertion, err := typedmemory.NewAssertionID(
		"assertion:production-decision-choice",
	)
	mustProductionNoteNoError(t, err)
	gamma, err := typedmemory.NewGammaPoint(
		time.Date(2026, 7, 18, 10, 10, 0, 0, time.UTC),
	)
	mustProductionNoteNoError(t, err)
	contextSlice, err := typedmemory.NewContextSlice(
		typedmemory.ContextSliceInput{
			Context:   contextRef,
			GammaTime: gamma,
		},
	)
	mustProductionNoteNoError(t, err)
	optionA, err := decisionrecordadapter.NewDecisionOptionBinding(
		"Option A",
		optionARef,
	)
	mustProductionNoteNoError(t, err)
	optionB, err := decisionrecordadapter.NewDecisionOptionBinding(
		"Option B",
		optionBRef,
	)
	mustProductionNoteNoError(t, err)
	options := []decisionrecordadapter.DecisionOptionBinding{optionB, optionA}
	if reversed {
		options = []decisionrecordadapter.DecisionOptionBinding{optionA, optionB}
	}
	comparison, err := decisionrecordadapter.NewExactProjectRecordReference(
		"comparison:production",
		comparisonRef,
	)
	mustProductionNoteNoError(t, err)
	contextProjection, err := decisionrecordadapter.NewLegacyContextProjection(
		source,
		contextRef,
	)
	mustProductionNoteNoError(t, err)
	draft, err := decisionrecordadapter.NewDraft(
		decisionrecordadapter.ProjectionDraftInput{
			ProjectID:         fixture.project,
			RecordEntity:      entity,
			RecordLocalRef:    localRef,
			RecordLabel:       label,
			AssertionID:       assertion,
			ContextSlice:      contextSlice,
			Source:            source,
			ContextProjection: contextProjection,
			Concern: productionNoteConcernBinding(
				t,
				current,
				concern.Entity(),
				contextRef,
			),
			Problem:    decisionrecordadapter.NoProjectRecordReference(),
			Portfolio:  decisionrecordadapter.NoProjectRecordReference(),
			Options:    options,
			Comparison: comparison,
		},
	)
	mustProductionNoteNoError(t, err)
	return draft
}

func countProductionDecisionRecords(
	t *testing.T,
	ctx context.Context,
	fixture genesisE2EFixture,
) int {
	t.Helper()
	records, err := artifact.NewStore(fixture.database).ListByKind(
		ctx,
		artifact.KindDecisionRecord,
		0,
	)
	mustProductionNoteNoError(t, err)
	return len(records)
}

func admitProductionPortfolioConcern(
	t *testing.T,
	ctx context.Context,
	fixture genesisE2EFixture,
	resolver typedmemorystore.SelectedProjectTypeEnvRuntimeResolver,
	baseLoader typedmemorystore.CurrentProjectSnapshotLoader,
	clock typedmemorystore.Clock,
	concern typedmemory.DeclareEntity,
) typedmemorystore.CommitReceipt {
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
	candidate, err := typedmemory.NewMemoryChangeSet(
		[]typedmemory.MemoryChange{concern},
	)
	mustProductionNoteNoError(t, err)
	valid, err := runtime.PrepareCandidate(
		ctx,
		typedmemorywire.ProjectCurrentSelector{},
		candidate,
	)
	mustProductionNoteNoError(t, err)
	key, err := typedmemorystore.NewIdempotencyKey(
		"production-portfolio-concern",
	)
	mustProductionNoteNoError(t, err)
	receipt, err := runtime.AdmitValidated(
		ctx,
		valid,
		key,
		concern.Provenance(),
	)
	mustProductionNoteNoError(t, err)
	return receipt
}

func admitProductionPortfolioRecord(
	t *testing.T,
	ctx context.Context,
	fixture genesisE2EFixture,
	resolver typedmemorystore.SelectedProjectTypeEnvRuntimeResolver,
	baseLoader typedmemorystore.CurrentProjectSnapshotLoader,
	clock typedmemorystore.Clock,
	candidate recordatconcern.ValidCandidate,
	token string,
) (typedmemoryvalidation.ValidOutcome, typedmemorystore.CommitReceipt) {
	t.Helper()
	evaluation := evaluateProductionPortfolioRecord(
		t,
		ctx,
		fixture,
		resolver,
		baseLoader,
		clock,
		candidate,
	)
	valid, ok := evaluation.outcome.(typedmemoryvalidation.ValidOutcome)
	if !ok {
		t.Fatalf(
			"%s validation = %T/%s diagnostics=%#v",
			token,
			evaluation.outcome,
			evaluation.outcome.Verdict(),
			evaluation.outcome.Diagnostics(),
		)
	}
	runtime, err := projectmemory.NewAdmissionRuntime(
		fixture.project,
		evaluation.source,
		evaluation.adapter,
	)
	mustProductionNoteNoError(t, err)
	key, err := typedmemorystore.NewIdempotencyKey(token)
	mustProductionNoteNoError(t, err)
	provenance, err := typedmemory.NewProvenanceRef(
		"memory:test:" + token,
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
			"%s disposition = %s, want applied",
			token,
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
		t.Fatalf("%s replay changed the committed result", token)
	}
	return valid, receipt
}

type productionPortfolioCandidateEvaluation struct {
	outcome typedmemoryvalidation.Outcome
	source  projectmemory.ProjectBasisSource
	adapter *typedmemorystore.SQLiteAdapter
}

func evaluateProductionPortfolioRecord(
	t *testing.T,
	ctx context.Context,
	fixture genesisE2EFixture,
	resolver typedmemorystore.SelectedProjectTypeEnvRuntimeResolver,
	baseLoader typedmemorystore.CurrentProjectSnapshotLoader,
	clock typedmemorystore.Clock,
	candidate recordatconcern.ValidCandidate,
) productionPortfolioCandidateEvaluation {
	t.Helper()
	stage, err := recordatconcern.SealPreAdmissionSourceStage(candidate)
	mustProductionNoteNoError(t, err)
	overlayLoader, err :=
		typedmemorystore.NewCurrentProjectSnapshotLoaderWithObservableInputOverlay(
			baseLoader,
			stage,
		)
	mustProductionNoteNoError(t, err)
	source := newGenesisE2ECurrentProjectBasisSource(t, overlayLoader)
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
	return productionPortfolioCandidateEvaluation{
		outcome: outcome,
		source:  source,
		adapter: adapter,
	}
}

func productionPortfolioRecordInput(
	t *testing.T,
	project projectidentity.ProjectID,
	environment typedmemory.TypeEnv,
	contextRef typedmemory.BoundedContextRef,
	token string,
	labelValue string,
) recordatconcern.DraftInput {
	t.Helper()
	textKindID, err := typedmemory.NewKindID("Haft.Text")
	mustProductionNoteNoError(t, err)
	textKind, err := typedmemory.NewValueKindRef(
		environment.Ref(),
		textKindID,
	)
	mustProductionNoteNoError(t, err)
	claimID, err := typedmemory.NewClaimNodeID(
		"claim:production-portfolio-" + token,
	)
	mustProductionNoteNoError(t, err)
	claim, err := typedmemory.NewClaimNode(
		claimID,
		textKind,
		typedmemory.NewTextValue(labelValue+" remains explicitly addressable"),
	)
	mustProductionNoteNoError(t, err)
	graph, err := typedmemory.NewClaimGraphValue(
		[]typedmemory.ClaimNode{claim},
		nil,
	)
	mustProductionNoteNoError(t, err)
	exactGraph, err := recordatconcern.NewExactClaimGraph(graph)
	mustProductionNoteNoError(t, err)
	gamma, err := typedmemory.NewGammaPoint(
		time.Date(2026, 7, 18, 10, 5, 0, 0, time.UTC),
	)
	mustProductionNoteNoError(t, err)
	slice, err := typedmemory.NewContextSlice(
		typedmemory.ContextSliceInput{
			Context:   contextRef,
			GammaTime: gamma,
		},
	)
	mustProductionNoteNoError(t, err)
	entity, err := typedmemory.NewEntityID(
		"record:production-portfolio-" + token,
	)
	mustProductionNoteNoError(t, err)
	local, err := typedmemory.NewBatchLocalRef(
		"record:production-portfolio-" + token,
	)
	mustProductionNoteNoError(t, err)
	label, err := typedmemory.NewEntityLabel(labelValue)
	mustProductionNoteNoError(t, err)
	assertion, err := typedmemory.NewAssertionID(
		"assertion:production-portfolio-" + token,
	)
	mustProductionNoteNoError(t, err)
	provenance, err := typedmemory.NewProvenanceRef(
		"memory:test:production-portfolio-" + token,
	)
	mustProductionNoteNoError(t, err)
	return recordatconcern.DraftInput{
		ProjectID:      project,
		RecordEntity:   entity,
		RecordLocalRef: local,
		RecordLabel:    label,
		AssertionID:    assertion,
		ContextSlice:   slice,
		ClaimGraph:     exactGraph,
		Provenance:     provenance,
	}
}

func productionProjectRecordReference(
	t *testing.T,
	environment typedmemory.TypeEnv,
	entity typedmemory.EntityID,
) typedmemory.PersistedRef {
	t.Helper()
	refKindID, err := typedmemory.NewRefKindID("Haft.ProjectRecordRef")
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

func loadProductionPortfolioSnapshot(
	t *testing.T,
	ctx context.Context,
	loader typedmemorystore.CurrentProjectSnapshotLoader,
	project projectidentity.ProjectID,
) typedmemorystore.CurrentProjectSnapshot {
	t.Helper()
	current, err := loader.LoadCurrentProjectSnapshot(ctx, project)
	mustProductionNoteNoError(t, err)
	return current
}

func mustProductionRecordCandidate(
	t *testing.T,
	result recordatconcern.Result,
) recordatconcern.ValidCandidate {
	t.Helper()
	candidate, ok := result.(recordatconcern.ValidCandidate)
	if !ok {
		if underdetermined, missing := result.(recordatconcern.Underdetermined); missing {
			t.Fatalf(
				"project-record adapter result = %T, missing basis = %#v",
				result,
				productionMissingBasisDiagnostics(underdetermined.MissingBasis()),
			)
		}
		if invalid, rejected := result.(recordatconcern.Invalid); rejected {
			t.Fatalf(
				"project-record adapter result = %T, violations = %#v",
				result,
				productionViolationDiagnostics(invalid.Violations()),
			)
		}
		t.Fatalf(
			"project-record adapter result = %T, want ValidCandidate",
			result,
		)
	}
	return candidate
}

func productionMissingBasisDiagnostics(
	values []recordatconcern.MissingBasis,
) []string {
	diagnostics := make([]string, 0, len(values))
	for _, value := range values {
		diagnostics = append(
			diagnostics,
			value.Name()+" -> "+value.Repair().String(),
		)
	}
	return diagnostics
}

func productionViolationDiagnostics(
	values []recordatconcern.Violation,
) []string {
	diagnostics := make([]string, 0, len(values))
	for _, value := range values {
		diagnostics = append(
			diagnostics,
			value.Code()+" -> "+value.Message(),
		)
	}
	return diagnostics
}

func assertProductionCandidateIdentityEqual(
	t *testing.T,
	left recordatconcern.ValidCandidate,
	right recordatconcern.ValidCandidate,
	reordered string,
) {
	t.Helper()
	leftDigest, err := left.ChangeSet().Digest()
	mustProductionNoteNoError(t, err)
	rightDigest, err := right.ChangeSet().Digest()
	mustProductionNoteNoError(t, err)
	if leftDigest != rightDigest {
		t.Fatalf("%s changed canonical candidate identity", reordered)
	}
}

func assertProductionReferenceSlot(
	t *testing.T,
	candidate recordatconcern.ValidCandidate,
	slot string,
	want []typedmemory.PersistedRef,
) {
	t.Helper()
	relation := productionCandidateRelation(t, candidate)
	for _, binding := range relation.Bindings() {
		if binding.Name().String() != slot {
			continue
		}
		actual := make([]string, 0, len(binding.Fillers()))
		for _, filler := range binding.Fillers() {
			reference, ok := filler.(typedmemory.ByReferenceCandidate)
			if !ok {
				t.Fatalf("%s filler = %T, want ByReferenceCandidate", slot, filler)
			}
			persisted, ok := reference.Reference().(typedmemory.PersistedRef)
			if !ok {
				t.Fatalf("%s reference = %T, want PersistedRef", slot, reference.Reference())
			}
			actual = append(actual, persisted.ReferenceID().String())
		}
		expected := make([]string, 0, len(want))
		for _, reference := range want {
			expected = append(expected, reference.ReferenceID().String())
		}
		sort.Strings(actual)
		sort.Strings(expected)
		if fmt.Sprint(actual) != fmt.Sprint(expected) {
			t.Fatalf("%s references = %v, want %v", slot, actual, expected)
		}
		return
	}
	t.Fatalf("candidate omitted slot %s", slot)
}

func assertProductionComparisonHasNoWinnerSlot(
	t *testing.T,
	candidate recordatconcern.ValidCandidate,
) {
	t.Helper()
	relation := productionCandidateRelation(t, candidate)
	for _, binding := range relation.Bindings() {
		switch binding.Name().String() {
		case "Haft.PortfolioComparison.ChosenOptionSlot",
			"Haft.PortfolioComparison.WinnerSlot",
			"Haft.DecisionChoiceAtConcern.ChosenOptionSlot":
			t.Fatalf(
				"PortfolioComparison candidate smuggled selection slot %s",
				binding.Name().String(),
			)
		}
	}
}

func productionCandidateRelation(
	t *testing.T,
	candidate recordatconcern.ValidCandidate,
) typedmemory.RelationalAssertionCandidate {
	t.Helper()
	changes := candidate.ChangeSet().Changes()
	for _, change := range changes {
		assertion, ok := change.(typedmemory.AssertRelation)
		if ok {
			return assertion.Assertion()
		}
	}
	t.Fatal("project-record candidate omitted relational assertion")
	return typedmemory.RelationalAssertionCandidate{}
}

func assertProductionSelectedConstraintPresent(
	t *testing.T,
	valid typedmemoryvalidation.ValidOutcome,
	environment typedmemory.TypeEnv,
	constraint string,
) {
	t.Helper()
	basis, ok := valid.AdmissionBasis().(typedmemory.ContextSliceMembershipBasis)
	if !ok {
		t.Fatalf(
			"portfolio admission basis = %T, want ContextSliceMembershipBasis",
			valid.AdmissionBasis(),
		)
	}
	if basis.TypeEnv() != environment.Ref() {
		t.Fatal("portfolio admission basis used a different TypeEnv")
	}
	for _, checked := range environment.Constraints() {
		if checked.ID().String() == constraint {
			return
		}
	}
	t.Fatalf(
		"portfolio admission basis omitted checked constraint %s",
		constraint,
	)
}

func assertProductionPortfolioRelationsReread(
	t *testing.T,
	ctx context.Context,
	fixture genesisE2EFixture,
) {
	t.Helper()
	transaction, err := sqlitetransaction.BeginRead(
		ctx,
		fixture.database,
	)
	mustProductionNoteNoError(t, err)
	observation, err := typedmemorystore.LoadCurrentGraphRevalidationBasisTx(
		ctx,
		transaction,
		fixture.project,
	)
	mustProductionNoteNoError(t, err)
	finish := transaction.Commit(ctx)
	if !finish.Succeeded() {
		t.Fatalf("commit portfolio durable read: %v", finish.Err())
	}
	counts := make(map[string]int)
	for _, active := range observation.ActiveAssertions().Relations() {
		carrier := productionFreshCurrentAssertionCarrier(t, active)
		counts[carrier.Signature().ID().String()]++
	}
	if counts["Haft.NoteAtConcern"] != 2 ||
		counts["Haft.SolutionPortfolioAtConcern"] != 1 ||
		counts["Haft.PortfolioComparison"] != 1 ||
		counts["Haft.DecisionChoiceAtConcern"] != 1 {
		t.Fatalf(
			"durable portfolio relation counts = %#v, want 2 notes + portfolio + comparison + decision",
			counts,
		)
	}
}

func assertProductionDiagnosticCode(
	t *testing.T,
	diagnostics []typedmemoryvalidation.DiagnosticProjection,
	code typedmemory.DiagnosticCode,
) {
	t.Helper()
	for _, diagnostic := range diagnostics {
		if diagnostic.Code() == string(code) {
			return
		}
	}
	t.Fatalf("diagnostics = %#v, want code %s", diagnostics, code)
}
