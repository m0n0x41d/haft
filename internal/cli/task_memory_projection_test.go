package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	typedmemorycandidates "github.com/m0n0x41d/haft/data/haft/local-practice/typed-memory/candidates"
	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/project/specflow"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projectmemory"
	"github.com/m0n0x41d/haft/internal/projectmemory/localpracticeruntime"
	"github.com/m0n0x41d/haft/internal/testsupport/profileadmissionfixture"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

const (
	taskMemoryTestContext = "haft-project"
	taskMemoryTestConcern = "entity:authorization-service"
)

type taskMemoryProjectionTestFixture struct {
	root       string
	projectID  projectidentity.ProjectID
	store      *artifact.Store
	projector  *taskMemoryProjectionRuntime
	concernRev uint64
}

func TestAppendTaskMemoryProjectionSeparatesCarrierAndTypedEffects(
	t *testing.T,
) {
	t.Parallel()

	report := underdeterminedTaskMemoryProjectionReport(
		taskMemoryArtifactProjection{
			Ref:  "note-example",
			Kind: string(artifact.KindNote),
		},
		[]taskMemoryMissingBasisProjection{
			{Name: "entity_ref", Repair: "supply exact current concern"},
		},
	)

	rendered := appendTaskMemoryProjection("Note recorded.", report)

	for _, want := range []string{
		"## Persistence effects",
		"Carrier: `note-example` remains durable (`retained_unsettled`)",
		"Typed projection: admission `not_attempted`; write mode `not_attempted_no_write`; durable changes `0`",
		"## Typed project-memory projection",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf(
				"rendered task-memory projection missing %q:\n%s",
				want,
				rendered,
			)
		}
	}
}

func TestTaskMemoryAdapterSourceModeRecognizesCurrentClassificationRuntime(t *testing.T) {
	t.Parallel()

	base, err := loadEmbeddedMemoryRuntime(context.Background())
	if err != nil {
		t.Fatalf("loadEmbeddedMemoryRuntime() error = %v", err)
	}
	target, err := localpracticeruntime.Build(
		base.Artifact(),
		typedmemorycandidates.SourceV1_6(),
	)
	if err != nil {
		t.Fatalf("localpracticeruntime.Build() error = %v", err)
	}
	mode, err := selectTaskMemoryAdapterSourceMode(target)
	if err != nil {
		t.Fatalf("selectTaskMemoryAdapterSourceMode() error = %v", err)
	}
	if !mode.IsCurrentKindClassification() ||
		mode.IsHistoricalMembership() {
		t.Fatalf(
			"source mode = current %t historical %t",
			mode.IsCurrentKindClassification(),
			mode.IsHistoricalMembership(),
		)
	}
}

func TestTaskMemoryNoteProjectsOneExactConcernRelationAndCommitsIt(
	t *testing.T,
) {
	fixture := newTaskMemoryProjectionTestFixture(t)
	args := map[string]any{
		"title":        "Authorization service token boundary",
		"observations": []string{"Refresh tokens remain inside the authorization service."},
		"entity_ref": map[string]any{
			"ref_kind_id":  "U.EntityRef",
			"reference_id": taskMemoryTestConcern,
		},
		"bounded_context_ref": taskMemoryTestContext,
	}
	rendered, report := callTaskMemoryTool(
		t,
		fixture,
		"haft_note",
		args,
	)

	if !strings.Contains(rendered, report.Artifact.Ref) {
		t.Fatalf(
			"note response omitted persisted artifact ref %q",
			report.Artifact.Ref,
		)
	}
	if report.ContractVersion != taskMemoryProjectionContractVersion ||
		report.AdapterResult != "valid" ||
		report.AdmissionResult != "committed" ||
		report.RelationDeclarationFragmentID != "Haft.NoteAtConcern" ||
		report.RelationDeclarationPosture !=
			typedmemory.RelationDeclarationTypedFragment.String() ||
		report.RelationSignatureID != "Haft.NoteAtConcern" ||
		report.LegacyCarrierDisposition != "retained_with_typed_projection" {
		t.Fatalf("task-memory projection report = %#v", report)
	}
	if report.EntityOfConcern == nil ||
		report.EntityOfConcern.EntityID != taskMemoryTestConcern ||
		report.EntityOfConcern.BoundedContext != taskMemoryTestContext {
		t.Fatalf(
			"task-memory concern projection = %#v",
			report.EntityOfConcern,
		)
	}
	if report.RecordReference == nil ||
		report.RecordReference.RefKindID != "Haft.ProjectRecordRef" ||
		report.RecordReference.ReferenceID !=
			"record:"+report.Artifact.Ref ||
		report.RecordReference.EntityID !=
			report.RecordReference.ReferenceID {
		t.Fatalf(
			"task-memory record reference = %#v",
			report.RecordReference,
		)
	}
	if report.Receipt == nil ||
		report.Receipt.GraphRevision <= fixture.concernRev ||
		report.CandidateChangeCount == 0 ||
		report.DurableChangeCount != report.CandidateChangeCount {
		t.Fatalf("task-memory admission receipt = %#v", report)
	}
	if report.Persistence.AuthorityGranted ||
		len(report.Interpretation.Omits) == 0 ||
		len(report.Interpretation.DoesNotAuthorize) == 0 {
		t.Fatalf(
			"task-memory interpretation contract = %#v",
			report.Interpretation,
		)
	}
	if _, err := fixture.store.Get(
		context.Background(),
		report.Artifact.Ref,
	); err != nil {
		t.Fatalf("load retained legacy note: %v", err)
	}
	assertTaskMemoryProjectionIsObservable(
		t,
		fixture,
		report,
	)
}

func TestTaskMemoryNoteWithoutExactConcernRetainsCarrierWithoutGraphWrite(
	t *testing.T,
) {
	fixture := newTaskMemoryProjectionTestFixture(t)
	before := loadTaskMemoryProjectionRevision(
		t,
		fixture,
	)
	args := map[string]any{
		"title":        "Authorization service unresolved observation",
		"observations": []string{"A caller reported an unresolved token boundary."},
	}
	_, report := callTaskMemoryTool(
		t,
		fixture,
		"haft_note",
		args,
	)
	after := loadTaskMemoryProjectionRevision(
		t,
		fixture,
	)

	if report.AdapterResult != "underdetermined" ||
		report.AdmissionResult != "not_attempted" ||
		report.LegacyCarrierDisposition != "retained_unsettled" ||
		report.Receipt != nil {
		t.Fatalf("underdetermined task-memory report = %#v", report)
	}
	if len(report.MissingBasis) != 2 {
		t.Fatalf(
			"missing basis = %#v, want exact context and EntityOfConcern",
			report.MissingBasis,
		)
	}
	if before != after {
		t.Fatalf(
			"typed graph revision changed without exact concern: before=%d after=%d",
			before,
			after,
		)
	}
	if _, err := fixture.store.Get(
		context.Background(),
		report.Artifact.Ref,
	); err != nil {
		t.Fatalf("load retained unsettled note: %v", err)
	}
	if containsTaskMemoryStatement(
		report.Interpretation.Establishes,
		"no typed project-memory relation was created",
	) {
		t.Fatalf(
			"non-result was presented as an established semantic claim: %#v",
			report.Interpretation,
		)
	}
	if !containsTaskMemoryStatement(
		report.Interpretation.Omits,
		"no typed project-memory relation was created",
	) {
		t.Fatalf(
			"underdetermined omission is absent: %#v",
			report.Interpretation,
		)
	}
}

func TestTaskMemoryProblemProjectsOneExactConcernRelationAndCommitsIt(
	t *testing.T,
) {
	fixture := newTaskMemoryProjectionTestFixture(t)
	args := map[string]any{
		"action":                "frame",
		"title":                 "Authorization boundary is not explicit",
		"signal":                "Refresh-token ownership differs across callers.",
		"problem_profile":       "deep",
		"why_now":               "A new caller depends on the boundary.",
		"scope":                 "Authorization service token lifecycle.",
		"acceptance_probe":      "Every token transition has one named owner.",
		"freshness_disposition": "Recheck after the caller integration.",
		"entity_ref": map[string]any{
			"ref_kind_id":  "U.EntityRef",
			"reference_id": taskMemoryTestConcern,
		},
		"bounded_context_ref": taskMemoryTestContext,
	}
	_, report := callTaskMemoryTool(
		t,
		fixture,
		"haft_problem",
		args,
	)

	if report.AdapterResult != "valid" ||
		report.AdmissionResult != "committed" ||
		report.RelationSignatureID != "Haft.ProblemCardAtConcern" ||
		report.Artifact.Kind != string(artifact.KindProblemCard) {
		t.Fatalf("ProblemCard task-memory projection = %#v", report)
	}
	if report.EntityOfConcern == nil ||
		report.EntityOfConcern.EntityID != taskMemoryTestConcern {
		t.Fatalf(
			"ProblemCard concern projection = %#v",
			report.EntityOfConcern,
		)
	}
	assertTaskMemoryProjectionIsObservable(
		t,
		fixture,
		report,
	)
}

func TestTaskMemorySolutionPortfolioPreservesExactOptionRecordsAndCommitsIt(
	t *testing.T,
) {
	fixture := newTaskMemoryProjectionTestFixture(t)
	firstOption := admitTaskMemoryOptionRecord(
		t,
		fixture,
		"Keep refresh tokens in the authorization service",
		"The authorization service remains the sole refresh-token owner.",
	)
	secondOption := admitTaskMemoryOptionRecord(
		t,
		fixture,
		"Delegate refresh tokens to caller sessions",
		"Each caller session owns and rotates its refresh token.",
	)
	args := taskMemorySolutionArgs(
		firstOption,
		secondOption,
	)
	_, report := callTaskMemoryTool(
		t,
		fixture,
		"haft_solution",
		args,
	)

	if report.AdapterResult != "valid" ||
		report.AdmissionResult != "committed" ||
		report.RelationSignatureID != "Haft.SolutionPortfolioAtConcern" ||
		report.Artifact.Kind != string(artifact.KindSolutionPortfolio) {
		t.Fatalf(
			"SolutionPortfolio task-memory projection = %#v",
			report,
		)
	}
	if report.EntityOfConcern == nil ||
		report.EntityOfConcern.EntityID != taskMemoryTestConcern {
		t.Fatalf(
			"SolutionPortfolio concern projection = %#v",
			report.EntityOfConcern,
		)
	}
	assertTaskMemoryProjectionIsObservable(
		t,
		fixture,
		report,
	)
	stored, err := fixture.store.Get(
		context.Background(),
		report.Artifact.Ref,
	)
	if err != nil {
		t.Fatalf("load retained solution portfolio: %v", err)
	}
	fields := stored.UnmarshalPortfolioFields()
	if len(fields.Variants) != 2 ||
		fields.Variants[0].ProjectRecordRef == nil ||
		fields.Variants[1].ProjectRecordRef == nil {
		t.Fatalf(
			"persisted option references = %#v",
			fields.Variants,
		)
	}
	if fields.Variants[0].ProjectRecordRef.ReferenceID != firstOption ||
		fields.Variants[1].ProjectRecordRef.ReferenceID != secondOption {
		t.Fatalf(
			"persisted option references = %#v, want %q and %q",
			fields.Variants,
			firstOption,
			secondOption,
		)
	}

	comparisonArgs := taskMemoryComparisonArgs(
		report.Artifact.Ref,
	)
	_, comparisonReport := callTaskMemoryTool(
		t,
		fixture,
		"haft_solution",
		comparisonArgs,
	)
	if comparisonReport.AdapterResult != "valid" ||
		comparisonReport.AdmissionResult != "committed" ||
		comparisonReport.RelationSignatureID !=
			"Haft.PortfolioComparison" ||
		comparisonReport.Artifact.Kind !=
			"PortfolioComparisonEdition" ||
		comparisonReport.Artifact.Version != 2 {
		t.Fatalf(
			"PortfolioComparison task-memory projection = %#v",
			comparisonReport,
		)
	}
	wantComparisonRef := "record:" +
		report.Artifact.Ref +
		":comparison:v2"
	if comparisonReport.RecordReference == nil ||
		comparisonReport.RecordReference.RefKindID !=
			"Haft.ProjectRecordRef" ||
		comparisonReport.RecordReference.ReferenceID !=
			wantComparisonRef {
		t.Fatalf(
			"PortfolioComparison record reference = %#v, want %q",
			comparisonReport.RecordReference,
			wantComparisonRef,
		)
	}
	if !containsTaskMemoryStatement(
		comparisonReport.Interpretation.Omits,
		"selected_ref",
	) ||
		!containsTaskMemoryStatement(
			comparisonReport.Interpretation.Omits,
			"not a winner",
		) {
		t.Fatalf(
			"PortfolioComparison interpretation = %#v",
			comparisonReport.Interpretation,
		)
	}
	assertTaskMemoryComparisonRelation(
		t,
		fixture,
		comparisonReport,
		"record:"+report.Artifact.Ref,
		[]string{firstOption, secondOption},
		[]string{firstOption},
		[]taskMemoryClaimInput{
			{role: "parity_baseline", text: "V1"},
			{role: "parity_baseline", text: "V2"},
			{
				role: "parity_normalization",
				text: `dimension="latency" method="linear 0..1"`,
			},
			{role: "parity_window", text: "5 minutes"},
			{role: "parity_budget", text: "100 requests"},
			{
				role: "parity_missing_data_policy",
				text: artifact.MissingDataPolicyExplicitAbstain,
			},
			{
				role: "parity_pinned_condition",
				text: "same host and request mix",
			},
		},
	)
	decision := createTaskMemoryDecisionRecord(
		t,
		fixture,
		"dec-20260719-task-memory-choice",
		report.Artifact.Ref,
	)
	decisionReport, applicable := fixture.projector.Project(
		context.Background(),
		taskMemoryProjectionRequest{
			ToolName:    "haft_decision",
			Action:      "project_existing",
			ArtifactRef: decision.Meta.ID,
			Mode:        taskMemoryProjectionApply,
		},
	)
	if !applicable ||
		decisionReport.AdapterResult != "valid" ||
		decisionReport.AdmissionResult != "committed" ||
		decisionReport.RelationSignatureID !=
			"Haft.DecisionChoiceAtConcern" {
		t.Fatalf(
			"DecisionRecord task-memory projection = %#v",
			decisionReport,
		)
	}
	if decisionReport.RecordReference == nil ||
		decisionReport.RecordReference.RefKindID !=
			"Haft.DecisionRecordRef" ||
		decisionReport.RecordReference.ReferenceID !=
			decision.Meta.ID {
		t.Fatalf(
			"DecisionRecord reference = %#v",
			decisionReport.RecordReference,
		)
	}
	assertTaskMemoryDecisionRelation(
		t,
		fixture,
		decisionReport,
		"record:"+report.Artifact.Ref,
		[]string{firstOption, secondOption},
		firstOption,
		[]string{secondOption},
	)
	legacyDecision := createTaskMemoryDecisionRecord(
		t,
		fixture,
		"dec-20260719-legacy-independent-choice-rule",
		report.Artifact.Ref,
	)
	legacyFields := legacyDecision.UnmarshalDecisionFields()
	legacyFields.SelectionPolicy =
		"Apply the detailed maximin policy before using the stored choice rule."
	legacyStructuredData, err := json.Marshal(legacyFields)
	if err != nil {
		t.Fatalf("encode legacy DecisionRecord fixture: %v", err)
	}
	legacyDecision.StructuredData = string(legacyStructuredData)
	if err := fixture.store.Update(
		context.Background(),
		legacyDecision,
	); err != nil {
		t.Fatalf("persist legacy DecisionRecord fixture: %v", err)
	}
	legacyReport, legacyApplicable := fixture.projector.Project(
		context.Background(),
		taskMemoryProjectionRequest{
			ToolName:    "haft_decision",
			Action:      "project_existing",
			ArtifactRef: legacyDecision.Meta.ID,
			Mode:        taskMemoryProjectionDryRun,
		},
	)
	if !legacyApplicable ||
		legacyReport.AdapterResult != "valid" ||
		legacyReport.AdmissionResult != "validated_only" ||
		legacyReport.SourceProjectionDisposition !=
			"legacy_independent_choice_fields" ||
		len(legacyReport.SourceProjectionWarnings) != 1 ||
		!containsTaskMemoryStatement(
			legacyReport.Interpretation.Omits,
			"does not claim",
		) {
		t.Fatalf(
			"legacy DecisionRecord task-memory projection = %#v",
			legacyReport,
		)
	}
	beforeDirectDecision := loadTaskMemoryProjectionRevision(
		t,
		fixture,
	)
	directDecision := createTaskMemoryDecisionRecord(
		t,
		fixture,
		"dec-20260719-direct-task-memory-choice",
		"",
	)
	directReport, directApplicable := fixture.projector.Project(
		context.Background(),
		taskMemoryProjectionRequest{
			ToolName:    "haft_decision",
			Action:      "project_existing",
			ArtifactRef: directDecision.Meta.ID,
			Mode:        taskMemoryProjectionApply,
		},
	)
	afterDirectDecision := loadTaskMemoryProjectionRevision(
		t,
		fixture,
	)
	if !directApplicable ||
		directReport.AdapterResult != "underdetermined" ||
		directReport.AdmissionResult != "not_attempted" ||
		len(directReport.MissingBasis) != 1 ||
		directReport.MissingBasis[0].Repair !=
			"repair:bind-choice-to-typed-solution-portfolio" {
		t.Fatalf(
			"direct DecisionRecord typed projection = %#v",
			directReport,
		)
	}
	if beforeDirectDecision != afterDirectDecision {
		t.Fatalf(
			"direct DecisionRecord changed typed graph without exact option mapping: before=%d after=%d",
			beforeDirectDecision,
			afterDirectDecision,
		)
	}

	beforeUnsettledCompare := loadTaskMemoryProjectionRevision(
		t,
		fixture,
	)
	storedAfterCompare, err := fixture.store.Get(
		context.Background(),
		report.Artifact.Ref,
	)
	if err != nil {
		t.Fatalf("reload compared portfolio: %v", err)
	}
	unsettledFields := storedAfterCompare.UnmarshalPortfolioFields()
	unsettledFields.Variants[1].ProjectRecordRef = nil
	structuredData, err := json.Marshal(unsettledFields)
	if err != nil {
		t.Fatalf("encode unsettled compared portfolio: %v", err)
	}
	storedAfterCompare.StructuredData = string(structuredData)
	if err := fixture.store.Update(
		context.Background(),
		storedAfterCompare,
	); err != nil {
		t.Fatalf("persist unsettled compared portfolio fixture: %v", err)
	}
	_, unsettledReport := callTaskMemoryTool(
		t,
		fixture,
		"haft_solution",
		comparisonArgs,
	)
	afterUnsettledCompare := loadTaskMemoryProjectionRevision(
		t,
		fixture,
	)
	if unsettledReport.AdapterResult != "underdetermined" ||
		unsettledReport.AdmissionResult != "not_attempted" ||
		unsettledReport.Receipt != nil ||
		len(unsettledReport.MissingBasis) == 0 {
		t.Fatalf(
			"unsettled PortfolioComparison projection = %#v",
			unsettledReport,
		)
	}
	if beforeUnsettledCompare != afterUnsettledCompare {
		t.Fatalf(
			"typed graph changed for comparison with a missing option record: before=%d after=%d",
			beforeUnsettledCompare,
			afterUnsettledCompare,
		)
	}
}

func TestTaskMemorySolutionPortfolioWithoutOptionRecordsRemainsUnsettled(
	t *testing.T,
) {
	fixture := newTaskMemoryProjectionTestFixture(t)
	before := loadTaskMemoryProjectionRevision(
		t,
		fixture,
	)
	args := taskMemorySolutionArgs(
		"",
		"",
	)
	_, report := callTaskMemoryTool(
		t,
		fixture,
		"haft_solution",
		args,
	)
	after := loadTaskMemoryProjectionRevision(
		t,
		fixture,
	)

	if report.AdapterResult != "underdetermined" ||
		report.AdmissionResult != "not_attempted" ||
		report.LegacyCarrierDisposition != "retained_unsettled" ||
		report.Receipt != nil {
		t.Fatalf(
			"unsettled SolutionPortfolio projection = %#v",
			report,
		)
	}
	if len(report.MissingBasis) != 2 {
		t.Fatalf(
			"missing option-record basis = %#v, want one per variant",
			report.MissingBasis,
		)
	}
	if before != after {
		t.Fatalf(
			"typed graph changed without exact option records: before=%d after=%d",
			before,
			after,
		)
	}
	if _, err := fixture.store.Get(
		context.Background(),
		report.Artifact.Ref,
	); err != nil {
		t.Fatalf("load retained unsettled portfolio: %v", err)
	}
}

func TestPortfolioComparisonParityClaimsPreserveNormalizationInCanonicalGraph(
	t *testing.T,
) {
	fixture := newTaskMemoryProjectionTestFixture(t)
	current, err := fixture.projector.basis.snapshotLoader.LoadCurrentProjectSnapshot(
		context.Background(),
		fixture.projectID,
	)
	if err != nil {
		t.Fatalf("load current project snapshot: %v", err)
	}

	first := &artifact.ParityPlan{
		BaselineSet: []string{"V1", "V2"},
		Window:      "5 minutes",
		Budget:      "100 requests",
		Normalization: []artifact.NormRule{
			{Dimension: " latency ", Method: " linear 0..1 "},
			{Dimension: " ", Method: ""},
			{Dimension: "cost", Method: "USD per request"},
		},
		MissingDataPolicy: artifact.MissingDataPolicyExplicitAbstain,
	}
	permuted := &artifact.ParityPlan{
		BaselineSet: []string{"V2", "V1"},
		Window:      "5 minutes",
		Budget:      "100 requests",
		Normalization: []artifact.NormRule{
			{Dimension: "cost", Method: "USD per request"},
			{Dimension: "latency", Method: "linear 0..1"},
		},
		MissingDataPolicy: artifact.MissingDataPolicyExplicitAbstain,
	}
	changed := &artifact.ParityPlan{
		BaselineSet: []string{"V1", "V2"},
		Window:      "5 minutes",
		Budget:      "100 requests",
		Normalization: []artifact.NormRule{
			{Dimension: "latency", Method: "z-score"},
			{Dimension: "cost", Method: "USD per request"},
		},
		MissingDataPolicy: artifact.MissingDataPolicyExplicitAbstain,
	}

	firstClaims := portfolioComparisonParityClaims(first)
	firstBytes := canonicalTaskMemoryClaimGraphBytes(
		t,
		current.Environment(),
		"record:portfolio:comparison:v2",
		firstClaims,
	)
	permutedClaims := portfolioComparisonParityClaims(permuted)
	permutedBytes := canonicalTaskMemoryClaimGraphBytes(
		t,
		current.Environment(),
		"record:portfolio:comparison:v2",
		permutedClaims,
	)
	changedClaims := portfolioComparisonParityClaims(changed)
	changedBytes := canonicalTaskMemoryClaimGraphBytes(
		t,
		current.Environment(),
		"record:portfolio:comparison:v2",
		changedClaims,
	)

	if !bytes.Equal(firstBytes, permutedBytes) {
		t.Fatal("canonical comparison graph changed under parity-rule permutation")
	}
	if bytes.Equal(firstBytes, changedBytes) {
		t.Fatal("canonical comparison graph ignored a changed normalization method")
	}
	if !containsTaskMemoryClaim(
		firstClaims,
		"parity_normalization",
		`dimension="latency" method="linear 0..1"`,
	) {
		t.Fatalf("normalization claims = %#v", firstClaims)
	}
}

func TestTaskMemorySpecSectionProjectsCurrentEditionWithoutLifecycleEffect(
	t *testing.T,
) {
	fixture := newTaskMemoryProjectionTestFixture(t)
	section := project.SpecSection{
		ID:            "SS.constraints.authorization.001",
		Spec:          "software-system",
		DocumentKind:  "SoftwareSystemSpec",
		Kind:          "constraint",
		Title:         "Authorization token ownership",
		StatementType: "normative",
		ClaimLayer:    "L2",
		Owner:         "human",
		Status:        "active",
		SystemFrame: project.SystemReferenceFrame{
			ID:   "HaftSoftwareSystem",
			Kind: "software_system",
		},
		Claims: []project.SpecClaim{{
			ID:        "SS.constraints.authorization.001.C1",
			Class:     "constraint",
			Statement: "The authorization service is the sole refresh-token owner.",
		}},
	}
	editionStore := specflow.NewSQLiteSpecSectionEditionStore(
		fixture.projector.database,
	)
	edition := specflow.NewSpecSectionEdition(
		fixture.projectID.String(),
		section,
		specflow.SpecSectionSourceSQL,
		time.Now().UTC(),
	)
	if err := editionStore.PutCurrent(edition); err != nil {
		t.Fatalf("seed current SpecSection edition: %v", err)
	}
	before := countTaskMemorySpecSectionBaselines(
		t,
		fixture,
		section.ID,
	)
	args := map[string]any{
		"action":     "project",
		"section_id": section.ID,
		"entity_ref": map[string]any{
			"ref_kind_id":  "U.EntityRef",
			"reference_id": taskMemoryTestConcern,
		},
		"bounded_context_ref": taskMemoryTestContext,
	}
	_, report := callTaskMemoryTool(
		t,
		fixture,
		"haft_spec_section",
		args,
	)
	after := countTaskMemorySpecSectionBaselines(
		t,
		fixture,
		section.ID,
	)

	if report.AdapterResult != "valid" ||
		report.AdmissionResult != "committed" ||
		report.RelationSignatureID != "Haft.SpecSectionAtConcern" ||
		report.Artifact.Kind != "SpecSectionEdition" {
		t.Fatalf(
			"SpecSection task-memory projection = %#v",
			report,
		)
	}
	if report.EntityOfConcern == nil ||
		report.EntityOfConcern.EntityID != taskMemoryTestConcern {
		t.Fatalf(
			"SpecSection concern projection = %#v",
			report.EntityOfConcern,
		)
	}
	if report.RecordReference == nil ||
		report.RecordReference.RefKindID != "Haft.SpecSectionRecordRef" ||
		!strings.HasPrefix(
			report.RecordReference.ReferenceID,
			"record:spec-section-edition:",
		) ||
		report.RecordReference.EntityID !=
			report.RecordReference.ReferenceID {
		t.Fatalf(
			"SpecSection record reference = %#v",
			report.RecordReference,
		)
	}
	if before != after {
		t.Fatalf(
			"SpecSection projection changed lifecycle baselines: before=%d after=%d",
			before,
			after,
		)
	}
	assertTaskMemoryProjectionIsObservable(
		t,
		fixture,
		report,
	)
}

func countTaskMemorySpecSectionBaselines(
	t *testing.T,
	fixture taskMemoryProjectionTestFixture,
	sectionID string,
) int {
	t.Helper()

	row := fixture.projector.database.QueryRow(
		`SELECT COUNT(*)
		   FROM spec_section_baselines
		  WHERE project_id = ?
		    AND section_id = ?`,
		fixture.projectID.String(),
		sectionID,
	)
	var count int
	if err := row.Scan(&count); err != nil {
		t.Fatalf("count SpecSection lifecycle baselines: %v", err)
	}
	return count
}

func admitTaskMemoryOptionRecord(
	t *testing.T,
	fixture taskMemoryProjectionTestFixture,
	title string,
	observation string,
) string {
	t.Helper()

	args := map[string]any{
		"title":        title,
		"observations": []string{observation},
		"entity_ref": map[string]any{
			"ref_kind_id":  "U.EntityRef",
			"reference_id": taskMemoryTestConcern,
		},
		"bounded_context_ref": taskMemoryTestContext,
	}
	_, report := callTaskMemoryTool(
		t,
		fixture,
		"haft_note",
		args,
	)
	if report.AdmissionResult != "committed" {
		t.Fatalf(
			"admit option record %q: %#v",
			title,
			report,
		)
	}
	if report.RecordReference == nil ||
		report.RecordReference.RefKindID != "Haft.ProjectRecordRef" {
		t.Fatalf(
			"option record reference %q = %#v",
			title,
			report.RecordReference,
		)
	}
	return report.RecordReference.ReferenceID
}

func taskMemorySolutionArgs(
	firstOption string,
	secondOption string,
) map[string]any {
	first := map[string]any{
		"id":                   "V1",
		"title":                "Central token ownership",
		"description":          "Keep refresh tokens in the authorization service.",
		"weakest_link":         "The service remains a scaling bottleneck.",
		"novelty_marker":       "One owner preserves a narrow security boundary.",
		"stepping_stone":       true,
		"stepping_stone_basis": "Central ownership leaves a later delegation seam.",
	}
	second := map[string]any{
		"id":             "V2",
		"title":          "Caller session ownership",
		"description":    "Delegate refresh-token ownership to caller sessions.",
		"weakest_link":   "Revocation becomes a distributed responsibility.",
		"novelty_marker": "Ownership follows each caller session.",
	}
	if firstOption != "" {
		first["project_record_ref"] = map[string]any{
			"ref_kind_id":  "Haft.ProjectRecordRef",
			"reference_id": firstOption,
		}
	}
	if secondOption != "" {
		second["project_record_ref"] = map[string]any{
			"ref_kind_id":  "Haft.ProjectRecordRef",
			"reference_id": secondOption,
		}
	}
	return map[string]any{
		"action":   "explore",
		"variants": []any{first, second},
		"entity_ref": map[string]any{
			"ref_kind_id":  "U.EntityRef",
			"reference_id": taskMemoryTestConcern,
		},
		"bounded_context_ref": taskMemoryTestContext,
	}
}

func taskMemoryComparisonArgs(
	portfolioRef string,
) map[string]any {
	return map[string]any{
		"action":        "compare",
		"portfolio_ref": portfolioRef,
		"dimensions":    []string{"latency"},
		"scores": map[string]map[string]string{
			"V1": {"latency": "5ms"},
			"V2": {"latency": "10ms"},
		},
		"non_dominated_set": []string{"V1"},
		"dominated_variants": []any{map[string]any{
			"variant":      "V2",
			"dominated_by": []string{"V1"},
			"summary":      "V1 has lower latency under the same comparison window.",
		}},
		"pareto_tradeoffs": []any{map[string]any{
			"variant": "V1",
			"summary": "V1 has the lowest observed latency.",
		}},
		"policy_applied": "Minimize latency under the same observation window.",
		"parity_plan": map[string]any{
			"baseline_set": []string{"V1", "V2"},
			"window":       "5 minutes",
			"budget":       "100 requests",
			"normalization": []any{map[string]any{
				"dimension": "latency",
				"method":    "linear 0..1",
			}},
			"missing_data_policy": artifact.MissingDataPolicyExplicitAbstain,
			"pinned_conditions":   []string{"same host and request mix"},
		},
		"selected_ref":             "V2",
		"recommendation_rationale": "Legacy advisory content must not become a typed winner.",
		"entity_ref": map[string]any{
			"ref_kind_id":  "U.EntityRef",
			"reference_id": taskMemoryTestConcern,
		},
		"bounded_context_ref": taskMemoryTestContext,
	}
}

func canonicalTaskMemoryClaimGraphBytes(
	t *testing.T,
	environment typedmemory.TypeEnv,
	artifactRef string,
	claims []taskMemoryClaimInput,
) []byte {
	t.Helper()

	graph, err := buildTaskMemoryClaimGraph(
		environment,
		artifactRef,
		claims,
	)
	if err != nil {
		t.Fatalf("build task-memory ClaimGraph: %v", err)
	}
	kindID, err := typedmemory.NewKindID("U.ClaimGraph")
	if err != nil {
		t.Fatalf("construct U.ClaimGraph kind ID: %v", err)
	}
	kindRef, err := typedmemory.NewValueKindRef(
		environment.Ref(),
		kindID,
	)
	if err != nil {
		t.Fatalf("construct U.ClaimGraph kind ref: %v", err)
	}
	binding, found := environment.ValueBinding(kindRef)
	if !found {
		t.Fatal("selected TypeEnv has no U.ClaimGraph value binding")
	}
	codec, err := typedmemory.NewClaimGraphCodecV1(
		binding.ValueShape(),
	)
	if err != nil {
		t.Fatalf("construct ClaimGraph codec: %v", err)
	}
	encoded := codec.EncodeInput(graph)
	canonical, ok := encoded.(typedmemory.CanonicalizedCodecValue)
	if !ok {
		t.Fatalf("encode task-memory ClaimGraph = %T", encoded)
	}
	return canonical.CanonicalBytes()
}

func containsTaskMemoryClaim(
	claims []taskMemoryClaimInput,
	role string,
	text string,
) bool {
	normalized := normalizeTaskMemoryClaims(claims)
	for _, claim := range normalized {
		if claim.role == role && claim.text == text {
			return true
		}
	}
	return false
}

func createTaskMemoryDecisionRecord(
	t *testing.T,
	fixture taskMemoryProjectionTestFixture,
	decisionID string,
	portfolioRef string,
) *artifact.Artifact {
	t.Helper()

	choice := &artifact.ChoiceResult{
		SubjectRef:      "operator",
		OptionSet:       []string{"V1", "V2"},
		ComparisonBasis: []string{"V1 has lower observed latency than V2."},
		ChoiceRule:      "Minimize latency under the same observation window.",
		NextMove:        artifact.ChoiceNextMoveChooseNow,
		VariantRef:      "V1",
		PortfolioRef:    portfolioRef,
		Reason:          "V1 satisfies the declared comparison policy.",
		ReopenCondition: "Reopen when the observation window changes.",
	}
	fields := artifact.DecisionFields{
		SelectedTitle:   "Central token ownership selected",
		WhySelected:     choice.Reason,
		SelectionPolicy: choice.ChoiceRule,
		ChoiceResult:    choice,
	}
	encoded, err := json.Marshal(fields)
	if err != nil {
		t.Fatalf("encode DecisionRecord fixture: %v", err)
	}
	record := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:        decisionID,
			Kind:      artifact.KindDecisionRecord,
			Version:   1,
			Status:    artifact.StatusActive,
			Context:   taskMemoryTestContext,
			Title:     fields.SelectedTitle,
			CreatedAt: time.Date(2026, 7, 19, 8, 0, 0, 0, time.UTC),
		},
		StructuredData: string(encoded),
	}
	if err := fixture.store.Create(
		context.Background(),
		record,
	); err != nil {
		t.Fatalf("store DecisionRecord fixture: %v", err)
	}
	return record
}

func newTaskMemoryProjectionTestFixture(
	t *testing.T,
) taskMemoryProjectionTestFixture {
	t.Helper()

	ctx := context.Background()
	root := filepath.Join(
		t.TempDir(),
		"project",
	)
	harness := profileadmissionfixture.New(
		t,
		root,
	)
	harness.AdmitSoftwareRevision(
		t,
		"task-memory-projection",
	)
	t.Setenv(
		envProjectRoot,
		harness.Root().String(),
	)
	t.Setenv(
		envExpectedProjectID,
		harness.ProjectID(),
	)
	if err := runMemoryTypeEnvPrepare(
		genesisTestCommand(&bytes.Buffer{}),
		nil,
	); err != nil {
		t.Fatalf("prepare task-memory TypeEnv genesis: %v", err)
	}
	if err := runMemoryTypeEnvSelect(
		genesisTestCommand(&bytes.Buffer{}),
		nil,
	); err != nil {
		t.Fatalf("select task-memory TypeEnv genesis: %v", err)
	}
	projectIDValue, err := projectidentity.ParseProjectID(
		harness.ProjectID(),
	)
	projectID := mustTaskMemoryProjectionValue(
		t,
		projectIDValue,
		err,
	)
	runtimeValue, err := newProjectMemoryRuntime(
		ctx,
		projectID,
		harness.Database(),
	)
	runtime := mustTaskMemoryProjectionValue(
		t,
		runtimeValue,
		err,
	)
	concernReceipt := admitTaskMemoryTestConcern(
		t,
		runtime,
	)
	store := artifact.NewStore(
		harness.Database(),
	)
	projectorValue, err := newTaskMemoryProjectionRuntime(
		ctx,
		projectID,
		harness.Database(),
		store,
	)
	projector := mustTaskMemoryProjectionValue(
		t,
		projectorValue,
		err,
	)
	return taskMemoryProjectionTestFixture{
		root:       root,
		projectID:  projectID,
		store:      store,
		projector:  projector,
		concernRev: concernReceipt.GraphRevision().Value(),
	}
}

func admitTaskMemoryTestConcern(
	t *testing.T,
	runtime projectMemoryRuntime,
) typedmemorystore.CommitReceipt {
	t.Helper()

	ctx := context.Background()
	entityValue, err := typedmemory.NewEntityID(
		taskMemoryTestConcern,
	)
	entity := mustTaskMemoryProjectionValue(
		t,
		entityValue,
		err,
	)
	localRefValue, err := typedmemory.NewBatchLocalRef(
		taskMemoryTestConcern,
	)
	localRef := mustTaskMemoryProjectionValue(
		t,
		localRefValue,
		err,
	)
	contextValue, err := typedmemory.NewBoundedContextRef(
		taskMemoryTestContext,
	)
	contextRef := mustTaskMemoryProjectionValue(
		t,
		contextValue,
		err,
	)
	labelValue, err := typedmemory.NewEntityLabel(
		"Authorization service",
	)
	label := mustTaskMemoryProjectionValue(
		t,
		labelValue,
		err,
	)
	changeProvenanceValue, err := typedmemory.NewProvenanceRef(
		"provenance:task-memory-projection:concern",
	)
	changeProvenance := mustTaskMemoryProjectionValue(
		t,
		changeProvenanceValue,
		err,
	)
	declarationValue, err := typedmemory.NewDeclareEntity(
		entity,
		localRef,
		contextRef,
		label,
		changeProvenance,
	)
	declaration := mustTaskMemoryProjectionValue(
		t,
		declarationValue,
		err,
	)
	candidateValue, err := typedmemory.NewMemoryChangeSet(
		[]typedmemory.MemoryChange{declaration},
	)
	candidate := mustTaskMemoryProjectionValue(
		t,
		candidateValue,
		err,
	)
	validValue, err := runtime.admission.PrepareCandidate(
		ctx,
		typedmemorywire.ProjectCurrentSelector{},
		candidate,
	)
	valid := mustTaskMemoryProjectionValue(
		t,
		validValue,
		err,
	)
	keyValue, err := typedmemorystore.NewIdempotencyKey(
		"task-memory-projection-test-concern",
	)
	key := mustTaskMemoryProjectionValue(
		t,
		keyValue,
		err,
	)
	requestProvenanceValue, err := typedmemory.NewProvenanceRef(
		"provenance:task-memory-projection:concern-request",
	)
	requestProvenance := mustTaskMemoryProjectionValue(
		t,
		requestProvenanceValue,
		err,
	)
	receiptValue, err := runtime.admission.AdmitValidated(
		ctx,
		valid,
		key,
		requestProvenance,
	)
	return mustTaskMemoryProjectionValue(
		t,
		receiptValue,
		err,
	)
}

func callTaskMemoryTool(
	t *testing.T,
	fixture taskMemoryProjectionTestFixture,
	toolName string,
	args map[string]any,
) (string, taskMemoryProjectionReport) {
	t.Helper()

	handler := makeV5HandlerWithTaskMemoryProjection(
		fixture.store,
		nil,
		nil,
		filepath.Join(fixture.root, ".haft"),
		nil,
		nil,
		fixture.projector,
	)
	request := map[string]any{
		"name":      toolName,
		"arguments": args,
	}
	rawValue, err := json.Marshal(
		request,
	)
	raw := mustTaskMemoryProjectionValue(
		t,
		rawValue,
		err,
	)
	rendered, err := handler(
		context.Background(),
		toolName,
		raw,
	)
	if err != nil {
		t.Fatalf("%s with task-memory projection: %v", toolName, err)
	}
	return rendered, decodeTaskMemoryProjectionReport(
		t,
		rendered,
	)
}

func decodeTaskMemoryProjectionReport(
	t *testing.T,
	rendered string,
) taskMemoryProjectionReport {
	t.Helper()

	const prefix = "## Typed project-memory projection\n\n```json\n"
	start := strings.Index(
		rendered,
		prefix,
	)
	if start < 0 {
		t.Fatalf("task-memory projection block is absent:\n%s", rendered)
	}
	payload := rendered[start+len(prefix):]
	end := strings.Index(
		payload,
		"\n```",
	)
	if end < 0 {
		t.Fatalf("task-memory projection block is unterminated:\n%s", rendered)
	}
	report := taskMemoryProjectionReport{}
	if err := json.Unmarshal(
		[]byte(payload[:end]),
		&report,
	); err != nil {
		t.Fatalf("decode task-memory projection report: %v", err)
	}
	return report
}

func loadTaskMemoryProjectionRevision(
	t *testing.T,
	fixture taskMemoryProjectionTestFixture,
) uint64 {
	t.Helper()

	currentValue, err :=
		fixture.projector.basis.snapshotLoader.LoadCurrentProjectSnapshot(
			context.Background(),
			fixture.projectID,
		)
	current := mustTaskMemoryProjectionValue(
		t,
		currentValue,
		err,
	)
	return current.Snapshot().GraphRevision().Value()
}

func assertTaskMemoryProjectionIsObservable(
	t *testing.T,
	fixture taskMemoryProjectionTestFixture,
	report taskMemoryProjectionReport,
) {
	t.Helper()

	currentValue, err :=
		fixture.projector.basis.snapshotLoader.LoadCurrentProjectSnapshot(
			context.Background(),
			fixture.projectID,
		)
	current := mustTaskMemoryProjectionValue(
		t,
		currentValue,
		err,
	)
	assertionValue, err := typedmemory.NewAssertionID(
		"assertion:" +
			report.Artifact.Ref +
			":v1:at-concern",
	)
	assertion := mustTaskMemoryProjectionValue(
		t,
		assertionValue,
		err,
	)
	state := current.Snapshot().AssertionState(
		assertion,
	)
	if _, active := state.(typedmemory.ActiveAssertion); !active {
		t.Fatalf(
			"task-memory assertion state = %T, want ActiveAssertion",
			state,
		)
	}
}

func mustTaskMemoryAffirmedV3Carrier(
	t *testing.T,
	assertions []typedmemorystore.CurrentActiveAssertion,
	assertionID string,
	label string,
) typedmemorystore.CurrentAssertionCarrier {
	t.Helper()

	for _, active := range assertions {
		carrier := active.Carrier()
		if carrier.AssertionID().String() != assertionID {
			continue
		}
		if carrier.Kind() !=
			typedmemorystore.CurrentRelationalAssertionV3Carrier {
			t.Fatalf(
				"%s carrier kind = %q, want exact v3 relational assertion",
				label,
				carrier.Kind(),
			)
		}
		if _, legacy := active.LegacyRelation(); legacy {
			t.Fatalf(
				"%s v3 assertion was coerced into a legacy relation occurrence",
				label,
			)
		}
		exact, present := active.RelationalAssertion()
		if !present {
			t.Fatalf(
				"%s lost its exact v3 relational assertion carrier",
				label,
			)
		}
		modality, explicit := active.Posture().ExplicitModality()
		if !explicit ||
			modality != typedmemory.AssertionModalityAffirmsObtaining {
			t.Fatalf(
				"%s explicit modality = (%q, %v), want affirms_obtaining",
				label,
				modality,
				explicit,
			)
		}
		if exact.Modality().Kind() != modality {
			t.Fatalf(
				"%s exact carrier modality = %q, posture = %q",
				label,
				exact.Modality().Kind(),
				modality,
			)
		}
		affirms, exactAffirms := exact.Modality().(typedmemory.AffirmsObtaining)
		if !exactAffirms || affirms.HasOccurrenceDesignation() {
			t.Fatalf(
				"%s affirms posture acquired occurrence semantics",
				label,
			)
		}
		return carrier
	}
	t.Fatalf(
		"%s assertion %q is absent",
		label,
		assertionID,
	)
	return nil
}

func assertTaskMemoryComparisonRelation(
	t *testing.T,
	fixture taskMemoryProjectionTestFixture,
	report taskMemoryProjectionReport,
	portfolioRef string,
	comparedRefs []string,
	nonDominatedRefs []string,
	wantClaims []taskMemoryClaimInput,
) {
	t.Helper()

	loaderValue, err :=
		typedmemorystore.NewProjectAwareSQLiteCurrentProjectReadFrameLoader(
			fixture.projector.database,
			projectmemory.NewBaseTypeEnvLoader(),
			fixture.projector.basis.selectedRuntime,
		)
	loader := mustTaskMemoryProjectionValue(
		t,
		loaderValue,
		err,
	)
	frameValue, err := loader.LoadCurrentProjectReadFrame(
		context.Background(),
		fixture.projectID,
	)
	frame := mustTaskMemoryProjectionValue(
		t,
		frameValue,
		err,
	)
	assertionID := "assertion:" +
		strings.TrimPrefix(
			report.RecordReference.ReferenceID,
			"record:",
		) +
		":at-concern"
	comparison := mustTaskMemoryAffirmedV3Carrier(
		t,
		frame.GraphObservation().
			ActiveAssertions().
			Relations(),
		assertionID,
		"PortfolioComparison",
	)
	if comparison.Signature().ID().String() !=
		"Haft.PortfolioComparison" {
		t.Fatalf(
			"comparison signature = %q",
			comparison.Signature().ID().String(),
		)
	}
	wantRefs := map[string][]string{
		"Haft.PortfolioComparison.ComparisonSlot": {
			report.RecordReference.ReferenceID,
		},
		"Haft.PortfolioComparison.PortfolioSlot": {
			portfolioRef,
		},
		"Haft.PortfolioComparison.ComparedOptionSlot":     append([]string(nil), comparedRefs...),
		"Haft.PortfolioComparison.NonDominatedOptionSlot": append([]string(nil), nonDominatedRefs...),
	}
	for _, binding := range comparison.Bindings() {
		name := binding.Name().String()
		if strings.Contains(name, "Chosen") ||
			strings.Contains(name, "Winner") {
			t.Fatalf(
				"PortfolioComparison smuggled selection slot %q",
				name,
			)
		}
		expected, checked := wantRefs[name]
		if !checked {
			continue
		}
		actual := taskMemoryReferenceFillerIDs(binding)
		sort.Strings(actual)
		sort.Strings(expected)
		if strings.Join(actual, "\x00") !=
			strings.Join(expected, "\x00") {
			t.Fatalf(
				"%s references = %#v, want %#v",
				name,
				actual,
				expected,
			)
		}
		delete(wantRefs, name)
	}
	if len(wantRefs) != 0 {
		t.Fatalf(
			"PortfolioComparison is missing checked slots %#v",
			wantRefs,
		)
	}
	claimIdentity := strings.TrimPrefix(
		report.RecordReference.ReferenceID,
		"record:",
	)
	assertTaskMemoryClaimGraphContains(
		t,
		comparison.Bindings(),
		"Haft.PortfolioComparison.ClaimGraphSlot",
		claimIdentity,
		wantClaims,
	)
}

func assertTaskMemoryClaimGraphContains(
	t *testing.T,
	bindings []typedmemory.SlotBinding,
	slotName string,
	claimIdentity string,
	wantClaims []taskMemoryClaimInput,
) {
	t.Helper()

	for _, binding := range bindings {
		if binding.Name().String() != slotName {
			continue
		}
		fillers := binding.Fillers()
		if len(fillers) != 1 {
			t.Fatalf("%s fillers = %d, want 1", slotName, len(fillers))
		}
		valueFiller, exact := fillers[0].(typedmemory.ValueFiller)
		if !exact {
			t.Fatalf("%s filler = %T, want ValueFiller", slotName, fillers[0])
		}
		verified := valueFiller.Value()
		codec, err := typedmemory.NewClaimGraphCodecV1(
			verified.ValueShape(),
		)
		if err != nil {
			t.Fatalf("construct stored ClaimGraph codec: %v", err)
		}
		decoded := codec.Canonicalize(
			verified.ValueShape(),
			verified.CanonicalBytes(),
		)
		canonical, ok := decoded.(typedmemory.CanonicalizedCodecValue)
		if !ok {
			t.Fatalf("decode stored ClaimGraph = %T", decoded)
		}
		graph, ok := canonical.Value().(typedmemory.ClaimGraphValue)
		if !ok {
			t.Fatalf("stored value = %T, want ClaimGraphValue", canonical.Value())
		}
		nodes := make(map[string]typedmemory.ClaimNode)
		for _, node := range graph.Nodes() {
			nodes[node.ID().String()] = node
		}
		for _, claim := range normalizeTaskMemoryClaims(wantClaims) {
			nodeID := taskMemoryClaimNodeID(claimIdentity, claim)
			node, found := nodes[nodeID]
			if !found {
				t.Fatalf("stored ClaimGraph omitted %s claim %q", claim.role, claim.text)
			}
			scalar, exact := node.Value().(typedmemory.ScalarTypedValue)
			if !exact {
				t.Fatalf("stored claim %s value = %T", nodeID, node.Value())
			}
			text, isText := scalar.Text()
			if !isText || text != claim.text {
				t.Fatalf(
					"stored claim %s text = (%q, %v), want %q",
					nodeID,
					text,
					isText,
					claim.text,
				)
			}
		}
		return
	}
	t.Fatalf("typed-memory relation omitted %s", slotName)
}

func assertTaskMemoryDecisionRelation(
	t *testing.T,
	fixture taskMemoryProjectionTestFixture,
	report taskMemoryProjectionReport,
	portfolioRef string,
	optionRefs []string,
	chosenRef string,
	rejectedRefs []string,
) {
	t.Helper()

	loaderValue, err :=
		typedmemorystore.NewProjectAwareSQLiteCurrentProjectReadFrameLoader(
			fixture.projector.database,
			projectmemory.NewBaseTypeEnvLoader(),
			fixture.projector.basis.selectedRuntime,
		)
	loader := mustTaskMemoryProjectionValue(
		t,
		loaderValue,
		err,
	)
	frameValue, err := loader.LoadCurrentProjectReadFrame(
		context.Background(),
		fixture.projectID,
	)
	frame := mustTaskMemoryProjectionValue(
		t,
		frameValue,
		err,
	)
	assertionID := "assertion:" +
		report.Artifact.Ref +
		":choice:v1:at-concern"
	decision := mustTaskMemoryAffirmedV3Carrier(
		t,
		frame.GraphObservation().
			ActiveAssertions().
			Relations(),
		assertionID,
		"DecisionChoiceAtConcern",
	)
	if decision.Signature().ID().String() !=
		"Haft.DecisionChoiceAtConcern" {
		t.Fatalf(
			"decision signature = %q",
			decision.Signature().ID().String(),
		)
	}
	wantRefs := map[string][]string{
		"Haft.DecisionChoiceAtConcern.DecisionRecordSlot": {
			report.Artifact.Ref,
		},
		"Haft.DecisionChoiceAtConcern.PortfolioRecordSlot": {
			portfolioRef,
		},
		"Haft.DecisionChoiceAtConcern.OptionSlot": append(
			[]string(nil),
			optionRefs...,
		),
		"Haft.DecisionChoiceAtConcern.ChosenOptionSlot": {
			chosenRef,
		},
		"Haft.DecisionChoiceAtConcern.RejectedOptionSlot": append(
			[]string(nil),
			rejectedRefs...,
		),
	}
	for _, binding := range decision.Bindings() {
		name := binding.Name().String()
		if name ==
			"Haft.DecisionChoiceAtConcern.ComparisonRecordSlot" {
			if len(binding.Fillers()) != 0 {
				t.Fatalf(
					"DecisionChoiceAtConcern invented comparison reference %#v",
					taskMemoryReferenceFillerIDs(binding),
				)
			}
			continue
		}
		expected, checked := wantRefs[name]
		if !checked {
			continue
		}
		actual := taskMemoryReferenceFillerIDs(binding)
		sort.Strings(actual)
		sort.Strings(expected)
		if strings.Join(actual, "\x00") !=
			strings.Join(expected, "\x00") {
			t.Fatalf(
				"%s references = %#v, want %#v",
				name,
				actual,
				expected,
			)
		}
		delete(wantRefs, name)
	}
	if len(wantRefs) != 0 {
		t.Fatalf(
			"DecisionChoiceAtConcern is missing checked slots %#v",
			wantRefs,
		)
	}
}

func taskMemoryReferenceFillerIDs(
	binding typedmemory.SlotBinding,
) []string {
	references := make([]string, 0)
	for _, filler := range binding.Fillers() {
		reference, ok := filler.(typedmemory.ReferenceFiller)
		if !ok {
			continue
		}
		references = append(
			references,
			reference.Reference().ReferenceID().String(),
		)
	}
	return references
}

func containsTaskMemoryStatement(
	values []string,
	substring string,
) bool {
	for _, value := range values {
		if strings.Contains(
			value,
			substring,
		) {
			return true
		}
	}
	return false
}

func mustTaskMemoryProjectionValue[T any](
	t *testing.T,
	value T,
	err error,
) T {
	t.Helper()

	if err != nil {
		t.Fatal(err)
	}
	return value
}
