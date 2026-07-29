package cli

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/project/specflow"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projectmemory"
	"github.com/m0n0x41d/haft/internal/projectmemory/adaptersource"
	"github.com/m0n0x41d/haft/internal/projectmemory/decisionrecordadapter"
	"github.com/m0n0x41d/haft/internal/projectmemory/localpracticeruntime"
	"github.com/m0n0x41d/haft/internal/projectmemory/noteadapter"
	"github.com/m0n0x41d/haft/internal/projectmemory/portfoliocomparisonadapter"
	"github.com/m0n0x41d/haft/internal/projectmemory/problemcardadapter"
	"github.com/m0n0x41d/haft/internal/projectmemory/recordatconcern"
	"github.com/m0n0x41d/haft/internal/projectmemory/recordcarrier"
	"github.com/m0n0x41d/haft/internal/projectmemory/solutionportfolioadapter"
	"github.com/m0n0x41d/haft/internal/projectmemory/specsectionadapter"
	"github.com/m0n0x41d/haft/internal/recordmembershipregistration"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
	"github.com/m0n0x41d/haft/internal/typedmemoryvalidation"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

const taskMemoryProjectionContractVersion = "haft.task-memory-projection.v2"

type taskMemoryProjectionMode string

const (
	taskMemoryProjectionDryRun taskMemoryProjectionMode = "dry_run"
	taskMemoryProjectionApply  taskMemoryProjectionMode = "apply"
)

func (mode taskMemoryProjectionMode) valid() bool {
	return mode == taskMemoryProjectionDryRun ||
		mode == taskMemoryProjectionApply
}

type taskMemoryProjectionRequest struct {
	ToolName    string
	Action      string
	ArtifactRef string
	Arguments   map[string]any
	Mode        taskMemoryProjectionMode
}

type taskMemoryProjector interface {
	Project(
		context.Context,
		taskMemoryProjectionRequest,
	) (taskMemoryProjectionReport, bool)
}

type taskMemoryProjectionRuntime struct {
	projectID projectidentity.ProjectID
	database  *sql.DB
	store     *artifact.Store
	basis     projectMemoryRuntimeBasis
}

type unavailableTaskMemoryProjector struct {
	detail string
}

type taskMemoryProjectionReport struct {
	ContractVersion               string                       `json:"contract_version"`
	Artifact                      taskMemoryArtifactProjection `json:"artifact"`
	AdapterResult                 string                       `json:"adapter_result"`
	AdmissionResult               string                       `json:"admission_result"`
	AuthorityClass                string                       `json:"authority_class"`
	LegacyCarrierDisposition      string                       `json:"legacy_carrier_disposition"`
	SourceProjectionDisposition   string                       `json:"source_projection_disposition,omitempty"`
	SourceProjectionWarnings      []string                     `json:"source_projection_warnings,omitempty"`
	CandidateChangeCount          uint64                       `json:"candidate_change_count"`
	DurableChangeCount            uint64                       `json:"durable_change_count"`
	RelationDeclarationFragmentID string                       `json:"relation_declaration_fragment_id,omitempty"`
	RelationDeclarationPosture    string                       `json:"relation_declaration_posture,omitempty"`
	// RelationSignatureID is the v1 compatibility alias for the same fragment
	// coordinate. It does not claim a complete FPF RelationSignature.
	RelationSignatureID string                                `json:"relation_signature_id,omitempty"`
	RecordReference     *taskMemoryRecordReferenceProjection  `json:"record_reference,omitempty"`
	EntityOfConcern     *taskMemoryConcernProjection          `json:"entity_of_concern,omitempty"`
	MissingBasis        []taskMemoryMissingBasisProjection    `json:"missing_basis,omitempty"`
	Violations          []taskMemoryViolationProjection       `json:"violations,omitempty"`
	Validation          json.RawMessage                       `json:"validation,omitempty"`
	Persistence         taskMemoryPersistenceProjection       `json:"persistence"`
	Receipt             *taskMemoryAdmissionReceiptProjection `json:"receipt,omitempty"`
	Retry               *taskMemoryAdmissionRetryProjection   `json:"retry,omitempty"`
	OperationalDetail   string                                `json:"operational_detail,omitempty"`
	Interpretation      taskMemoryInterpretationProjection    `json:"interpretation"`
}

type taskMemoryArtifactProjection struct {
	Ref     string `json:"ref"`
	Kind    string `json:"kind"`
	Version int    `json:"version"`
	Title   string `json:"title"`
}

type taskMemoryConcernProjection struct {
	RefKindID       string `json:"ref_kind_id"`
	ReferenceID     string `json:"reference_id"`
	EntityID        string `json:"entity_id"`
	BoundedContext  string `json:"bounded_context_ref"`
	ResolutionBasis string `json:"resolution_basis_ref"`
}

type taskMemoryRecordReferenceProjection struct {
	RefKindID   string `json:"ref_kind_id"`
	ReferenceID string `json:"reference_id"`
	EntityID    string `json:"entity_id"`
}

type taskMemoryMissingBasisProjection struct {
	Name   string `json:"name"`
	Repair string `json:"repair"`
}

type taskMemoryViolationProjection struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type taskMemoryPersistenceProjection struct {
	Mode             string `json:"mode"`
	Disposition      string `json:"disposition,omitempty"`
	AuthorityGranted bool   `json:"authority_granted"`
}

type taskMemoryAdmissionReceiptProjection struct {
	EventRef      string `json:"event_ref"`
	CommitRef     string `json:"commit_ref"`
	GraphRevision uint64 `json:"graph_revision"`
	ResultDigest  string `json:"result_digest"`
}

type taskMemoryAdmissionRetryProjection struct {
	Kind            string `json:"kind"`
	ProjectID       string `json:"project_id"`
	ArtifactRef     string `json:"artifact_ref"`
	ArtifactVersion int    `json:"artifact_version"`
	IdempotencyKey  string `json:"idempotency_key"`
	CandidateDigest string `json:"candidate_digest"`
	Instruction     string `json:"instruction"`
}

type taskMemoryInterpretationProjection struct {
	Establishes      []string `json:"establishes"`
	Omits            []string `json:"omits"`
	DoesNotAuthorize []string `json:"does_not_authorize"`
}

type taskMemoryConcernInput struct {
	refKindID   string
	referenceID string
	context     string
}

type taskMemoryClaimInput struct {
	role string
	text string
}

type taskMemoryRecordSource struct {
	recordIdentity string
	claimIdentity  string
	label          string
	assertion      string
	provenance     string
	recordedAt     time.Time
	claims         []taskMemoryClaimInput
}

func newTaskMemoryProjectionRuntime(
	ctx context.Context,
	projectID projectidentity.ProjectID,
	database *sql.DB,
	store *artifact.Store,
) (*taskMemoryProjectionRuntime, error) {
	if store == nil {
		return nil, fmt.Errorf(
			"construct task-memory projection runtime: artifact store is required",
		)
	}
	basis, err := buildProjectMemoryRuntimeBasis(
		ctx,
		projectID,
		database,
	)
	if err != nil {
		return nil, err
	}
	return &taskMemoryProjectionRuntime{
		projectID: projectID,
		database:  database,
		store:     store,
		basis:     basis,
	}, nil
}

func projectExistingDecisionAfterBinding(
	ctx context.Context,
	projectID projectidentity.ProjectID,
	database *sql.DB,
	store *artifact.Store,
	decisionRef string,
) taskMemoryProjectionReport {
	artifactProjection := taskMemoryArtifactProjection{
		Ref: decisionRef,
	}
	runtime, err := newTaskMemoryProjectionRuntime(
		ctx,
		projectID,
		database,
		store,
	)
	if err != nil {
		return unavailableTaskMemoryProjectionReport(
			artifactProjection,
			fmt.Sprintf(
				"construct DecisionRecord typed projection runtime: %v",
				err,
			),
		)
	}
	report, applicable := runtime.Project(
		ctx,
		taskMemoryProjectionRequest{
			ToolName:    "haft_decision",
			Action:      "project_existing",
			ArtifactRef: decisionRef,
			Mode:        taskMemoryProjectionApply,
		},
	)
	if applicable {
		return report
	}
	return unavailableTaskMemoryProjectionReport(
		artifactProjection,
		"DecisionRecord typed projection route is unavailable",
	)
}

func newUnavailableTaskMemoryProjector(
	cause error,
) taskMemoryProjector {
	detail := "task-memory projection runtime is unavailable"
	if cause != nil {
		detail = cause.Error()
	}
	return unavailableTaskMemoryProjector{detail: detail}
}

func (projector unavailableTaskMemoryProjector) Project(
	_ context.Context,
	request taskMemoryProjectionRequest,
) (taskMemoryProjectionReport, bool) {
	if !taskMemoryProjectionApplicable(request) {
		return taskMemoryProjectionReport{}, false
	}
	artifactProjection := taskMemoryArtifactProjection{
		Ref: request.ArtifactRef,
	}
	report := unavailableTaskMemoryProjectionReport(
		artifactProjection,
		projector.detail,
	)
	return report, true
}

func (runtime *taskMemoryProjectionRuntime) Project(
	ctx context.Context,
	request taskMemoryProjectionRequest,
) (taskMemoryProjectionReport, bool) {
	if !taskMemoryProjectionApplicable(request) {
		return taskMemoryProjectionReport{}, false
	}
	if !request.Mode.valid() {
		report := unavailableTaskMemoryProjectionReport(
			taskMemoryArtifactProjection{Ref: request.ArtifactRef},
			fmt.Sprintf(
				"task-memory projection mode %q is invalid",
				request.Mode,
			),
		)
		return report, true
	}
	if runtime == nil || runtime.store == nil {
		report := unavailableTaskMemoryProjectionReport(
			taskMemoryArtifactProjection{Ref: request.ArtifactRef},
			"task-memory projection runtime is absent",
		)
		return report, true
	}
	if request.ToolName == "haft_spec_section" {
		return runtime.projectSpecSection(
			ctx,
			request,
			request.Mode,
		), true
	}
	record, err := runtime.store.Get(ctx, request.ArtifactRef)
	if err != nil {
		report := unavailableTaskMemoryProjectionReport(
			taskMemoryArtifactProjection{Ref: request.ArtifactRef},
			fmt.Sprintf("load persisted task carrier: %v", err),
		)
		return report, true
	}
	if request.ToolName == "haft_decision" {
		return runtime.projectDecisionRecord(
			ctx,
			record,
			request.Mode,
		), true
	}
	artifactProjection := projectTaskMemoryArtifact(record)
	concernInput, missing := parseTaskMemoryConcernInput(
		request.Arguments,
	)
	if len(missing) > 0 {
		report := underdeterminedTaskMemoryProjectionReport(
			artifactProjection,
			missing,
		)
		return report, true
	}
	switch request.ToolName {
	case "haft_note":
		return runtime.projectNote(
			ctx,
			record,
			request.Arguments,
			concernInput,
			request.Mode,
		), true
	case "haft_problem":
		return runtime.projectProblem(
			ctx,
			record,
			request.Arguments,
			concernInput,
			request.Mode,
		), true
	case "haft_solution":
		switch request.Action {
		case "explore":
			return runtime.projectSolutionPortfolio(
				ctx,
				record,
				concernInput,
				request.Mode,
			), true
		case "compare":
			return runtime.projectPortfolioComparison(
				ctx,
				record,
				concernInput,
				request.Mode,
			), true
		default:
			return taskMemoryProjectionReport{}, false
		}
	default:
		return taskMemoryProjectionReport{}, false
	}
}

func (runtime *taskMemoryProjectionRuntime) projectNote(
	ctx context.Context,
	record *artifact.Artifact,
	args map[string]any,
	concernInput taskMemoryConcernInput,
	mode taskMemoryProjectionMode,
) taskMemoryProjectionReport {
	artifactProjection := projectTaskMemoryArtifact(record)
	current, err := runtime.basis.snapshotLoader.LoadCurrentProjectSnapshot(
		ctx,
		runtime.projectID,
	)
	if err != nil {
		return unavailableTaskMemoryProjectionReport(
			artifactProjection,
			fmt.Sprintf("load selected project-memory snapshot: %v", err),
		)
	}
	concern, concernProjection, concernResult := resolveTaskMemoryConcern(
		current,
		concernInput,
	)
	if concernResult != nil {
		concernResult.Artifact = artifactProjection
		return *concernResult
	}
	exactRuntime, err := buildTaskMemoryExactRuntime(
		runtime.projectID,
		current,
		runtime.basis,
	)
	if err != nil {
		return unavailableTaskMemoryProjectionReport(
			artifactProjection,
			fmt.Sprintf("construct selected task-memory runtime: %v", err),
		)
	}
	draft, err := buildTaskMemoryNoteDraft(
		runtime.projectID,
		current,
		record,
		args,
		concernInput,
	)
	if err != nil {
		return invalidTaskMemoryProjectionReport(
			artifactProjection,
			[]taskMemoryViolationProjection{{
				Code:    "note_projection_input_invalid",
				Message: err.Error(),
			}},
		)
	}
	adapted := noteadapter.Adapt(
		draft,
		exactRuntime,
		concern,
	)
	return runtime.admitTaskMemoryCandidate(
		ctx,
		artifactProjection,
		concernProjection,
		adapted,
		exactRuntime.SourceMode(),
		mode,
	)
}

func (runtime *taskMemoryProjectionRuntime) projectProblem(
	ctx context.Context,
	record *artifact.Artifact,
	args map[string]any,
	concernInput taskMemoryConcernInput,
	mode taskMemoryProjectionMode,
) taskMemoryProjectionReport {
	artifactProjection := projectTaskMemoryArtifact(record)
	current, err := runtime.basis.snapshotLoader.LoadCurrentProjectSnapshot(
		ctx,
		runtime.projectID,
	)
	if err != nil {
		return unavailableTaskMemoryProjectionReport(
			artifactProjection,
			fmt.Sprintf("load selected project-memory snapshot: %v", err),
		)
	}
	concern, concernProjection, concernResult := resolveTaskMemoryConcern(
		current,
		concernInput,
	)
	if concernResult != nil {
		concernResult.Artifact = artifactProjection
		return *concernResult
	}
	exactRuntime, err := buildTaskMemoryExactRuntime(
		runtime.projectID,
		current,
		runtime.basis,
	)
	if err != nil {
		return unavailableTaskMemoryProjectionReport(
			artifactProjection,
			fmt.Sprintf("construct selected task-memory runtime: %v", err),
		)
	}
	draft, err := buildTaskMemoryProblemDraft(
		runtime.projectID,
		current,
		record,
		args,
		concernInput,
	)
	if err != nil {
		return invalidTaskMemoryProjectionReport(
			artifactProjection,
			[]taskMemoryViolationProjection{{
				Code:    "problem_projection_input_invalid",
				Message: err.Error(),
			}},
		)
	}
	adapted := problemcardadapter.Adapt(
		draft,
		exactRuntime,
		concern,
	)
	return runtime.admitTaskMemoryCandidate(
		ctx,
		artifactProjection,
		concernProjection,
		adapted,
		exactRuntime.SourceMode(),
		mode,
	)
}

func (runtime *taskMemoryProjectionRuntime) projectSolutionPortfolio(
	ctx context.Context,
	record *artifact.Artifact,
	concernInput taskMemoryConcernInput,
	mode taskMemoryProjectionMode,
) taskMemoryProjectionReport {
	artifactProjection := projectTaskMemoryArtifact(record)
	current, err := runtime.basis.snapshotLoader.LoadCurrentProjectSnapshot(
		ctx,
		runtime.projectID,
	)
	if err != nil {
		return unavailableTaskMemoryProjectionReport(
			artifactProjection,
			fmt.Sprintf("load selected project-memory snapshot: %v", err),
		)
	}
	concern, concernProjection, concernResult := resolveTaskMemoryConcern(
		current,
		concernInput,
	)
	if concernResult != nil {
		concernResult.Artifact = artifactProjection
		return *concernResult
	}
	exactRuntime, err := buildTaskMemoryExactRuntime(
		runtime.projectID,
		current,
		runtime.basis,
	)
	if err != nil {
		return unavailableTaskMemoryProjectionReport(
			artifactProjection,
			fmt.Sprintf("construct selected task-memory runtime: %v", err),
		)
	}
	draft, missing, err := buildTaskMemorySolutionPortfolioDraft(
		runtime.projectID,
		current,
		record,
		concernInput,
	)
	if len(missing) > 0 {
		report := underdeterminedTaskMemoryProjectionReport(
			artifactProjection,
			missing,
		)
		report.EntityOfConcern = &concernProjection
		return report
	}
	if err != nil {
		report := invalidTaskMemoryProjectionReport(
			artifactProjection,
			[]taskMemoryViolationProjection{{
				Code:    "solution_portfolio_projection_input_invalid",
				Message: err.Error(),
			}},
		)
		report.EntityOfConcern = &concernProjection
		return report
	}
	adapted := solutionportfolioadapter.Adapt(
		draft,
		exactRuntime,
		concern,
	)
	return runtime.admitTaskMemoryCandidate(
		ctx,
		artifactProjection,
		concernProjection,
		adapted,
		exactRuntime.SourceMode(),
		mode,
	)
}

func (runtime *taskMemoryProjectionRuntime) projectPortfolioComparison(
	ctx context.Context,
	record *artifact.Artifact,
	concernInput taskMemoryConcernInput,
	mode taskMemoryProjectionMode,
) taskMemoryProjectionReport {
	artifactProjection :=
		projectTaskMemoryPortfolioComparisonEdition(record)
	current, err := runtime.basis.snapshotLoader.LoadCurrentProjectSnapshot(
		ctx,
		runtime.projectID,
	)
	if err != nil {
		return unavailableTaskMemoryProjectionReport(
			artifactProjection,
			fmt.Sprintf("load selected project-memory snapshot: %v", err),
		)
	}
	concern, concernProjection, concernResult := resolveTaskMemoryConcern(
		current,
		concernInput,
	)
	if concernResult != nil {
		concernResult.Artifact = artifactProjection
		return *concernResult
	}
	exactRuntime, err := buildTaskMemoryExactRuntime(
		runtime.projectID,
		current,
		runtime.basis,
	)
	if err != nil {
		return unavailableTaskMemoryProjectionReport(
			artifactProjection,
			fmt.Sprintf("construct selected task-memory runtime: %v", err),
		)
	}
	draft, missing, err := buildTaskMemoryPortfolioComparisonDraft(
		runtime.projectID,
		current,
		record,
		concernInput,
	)
	if len(missing) > 0 {
		report := underdeterminedTaskMemoryProjectionReport(
			artifactProjection,
			missing,
		)
		report.EntityOfConcern = &concernProjection
		return report
	}
	if err != nil {
		report := invalidTaskMemoryProjectionReport(
			artifactProjection,
			[]taskMemoryViolationProjection{{
				Code:    "portfolio_comparison_projection_input_invalid",
				Message: err.Error(),
			}},
		)
		report.EntityOfConcern = &concernProjection
		return report
	}
	adapted := portfoliocomparisonadapter.Adapt(
		draft,
		exactRuntime,
		concern,
	)
	return runtime.admitTaskMemoryCandidate(
		ctx,
		artifactProjection,
		concernProjection,
		adapted,
		exactRuntime.SourceMode(),
		mode,
	)
}

func (runtime *taskMemoryProjectionRuntime) projectDecisionRecord(
	ctx context.Context,
	record *artifact.Artifact,
	mode taskMemoryProjectionMode,
) taskMemoryProjectionReport {
	artifactProjection := projectTaskMemoryArtifact(record)
	source, err := decisionrecordadapter.LoadExistingDecisionChoiceSource(
		ctx,
		runtime.store,
		record.Meta.ID,
	)
	if err != nil {
		return invalidTaskMemoryProjectionReport(
			artifactProjection,
			[]taskMemoryViolationProjection{{
				Code:    "decision_projection_source_invalid",
				Message: err.Error(),
			}},
		)
	}
	decorate := func(
		report taskMemoryProjectionReport,
	) taskMemoryProjectionReport {
		report.SourceProjectionDisposition =
			source.ChoiceFieldCorrelationMode()
		if source.ChoiceFieldCorrelationMode() ==
			"legacy_independent_choice_fields" {
			report.SourceProjectionWarnings =
				source.ChoiceFieldWarnings()
			report.Interpretation.Omits = append(
				report.Interpretation.Omits,
				"duplicated DecisionRecord rationale and problem fields remain independent carrier text; the projection uses only the explicit stored ChoiceResult and does not claim equivalence",
			)
		}
		return report
	}
	frame, err := runtime.loadCurrentProjectReadFrame(ctx)
	if err != nil {
		return decorate(unavailableTaskMemoryProjectionReport(
			artifactProjection,
			fmt.Sprintf(
				"load selected project-memory read frame: %v",
				err,
			),
		))
	}
	current := frame.Snapshot()
	exactRuntime, err := buildTaskMemoryExactRuntime(
		runtime.projectID,
		current,
		runtime.basis,
	)
	if err != nil {
		return decorate(unavailableTaskMemoryProjectionReport(
			artifactProjection,
			fmt.Sprintf("construct selected task-memory runtime: %v", err),
		))
	}
	draft, concernProjection, missing, err :=
		buildTaskMemoryDecisionProjectionDraft(
			ctx,
			runtime.projectID,
			current,
			frame,
			record,
			source,
			runtime.store,
		)
	if len(missing) > 0 {
		report := underdeterminedTaskMemoryProjectionReport(
			artifactProjection,
			missing,
		)
		if concernProjection != nil {
			report.EntityOfConcern = concernProjection
		}
		return decorate(report)
	}
	if err != nil {
		report := invalidTaskMemoryProjectionReport(
			artifactProjection,
			[]taskMemoryViolationProjection{{
				Code:    "decision_projection_input_invalid",
				Message: err.Error(),
			}},
		)
		if concernProjection != nil {
			report.EntityOfConcern = concernProjection
		}
		return decorate(report)
	}
	adapted := decisionrecordadapter.Adapt(
		draft,
		exactRuntime,
	)
	report := runtime.admitTaskMemoryCandidate(
		ctx,
		artifactProjection,
		*concernProjection,
		adapted,
		exactRuntime.SourceMode(),
		mode,
	)
	return decorate(report)
}

func (runtime *taskMemoryProjectionRuntime) loadCurrentProjectReadFrame(
	ctx context.Context,
) (typedmemorystore.CurrentProjectReadFrame, error) {
	loader, err :=
		typedmemorystore.NewProjectAwareSQLiteCurrentProjectReadFrameLoader(
			runtime.database,
			projectmemory.NewBaseTypeEnvLoader(),
			runtime.basis.selectedRuntime,
		)
	if err != nil {
		return typedmemorystore.CurrentProjectReadFrame{}, err
	}
	return loader.LoadCurrentProjectReadFrame(
		ctx,
		runtime.projectID,
	)
}

func (runtime *taskMemoryProjectionRuntime) projectSpecSection(
	ctx context.Context,
	request taskMemoryProjectionRequest,
	mode taskMemoryProjectionMode,
) taskMemoryProjectionReport {
	sectionID, semanticHash, err :=
		parseSpecSectionProjectionEditionRef(
			request.ArtifactRef,
		)
	if err != nil {
		return invalidTaskMemoryProjectionReport(
			taskMemoryArtifactProjection{Ref: request.ArtifactRef},
			[]taskMemoryViolationProjection{{
				Code:    "spec_section_edition_ref_invalid",
				Message: err.Error(),
			}},
		)
	}
	store := specflow.NewSQLiteSpecSectionEditionStore(
		runtime.database,
	)
	edition, err := store.GetCurrent(
		runtime.projectID.String(),
		sectionID,
	)
	if err != nil {
		return unavailableTaskMemoryProjectionReport(
			taskMemoryArtifactProjection{Ref: request.ArtifactRef},
			fmt.Sprintf(
				"load current SpecSection edition %q: %v",
				sectionID,
				err,
			),
		)
	}
	artifactProjection := projectTaskMemorySpecSectionEdition(
		edition,
		request.ArtifactRef,
	)
	if edition.SemanticHash != semanticHash {
		return underdeterminedTaskMemoryProjectionReport(
			artifactProjection,
			[]taskMemoryMissingBasisProjection{{
				Name:   "exact_spec_section_edition",
				Repair: "repair:reload-current-spec-section-edition",
			}},
		)
	}
	concernInput, missing := parseTaskMemoryConcernInput(
		request.Arguments,
	)
	if len(missing) > 0 {
		return underdeterminedTaskMemoryProjectionReport(
			artifactProjection,
			missing,
		)
	}
	current, err := runtime.basis.snapshotLoader.LoadCurrentProjectSnapshot(
		ctx,
		runtime.projectID,
	)
	if err != nil {
		return unavailableTaskMemoryProjectionReport(
			artifactProjection,
			fmt.Sprintf("load selected project-memory snapshot: %v", err),
		)
	}
	concern, concernProjection, concernResult := resolveTaskMemoryConcern(
		current,
		concernInput,
	)
	if concernResult != nil {
		concernResult.Artifact = artifactProjection
		return *concernResult
	}
	exactRuntime, err := buildTaskMemoryExactRuntime(
		runtime.projectID,
		current,
		runtime.basis,
	)
	if err != nil {
		return unavailableTaskMemoryProjectionReport(
			artifactProjection,
			fmt.Sprintf("construct selected task-memory runtime: %v", err),
		)
	}
	draft, err := buildTaskMemorySpecSectionDraft(
		runtime.projectID,
		current,
		edition,
		request.ArtifactRef,
		concernInput,
	)
	if err != nil {
		report := invalidTaskMemoryProjectionReport(
			artifactProjection,
			[]taskMemoryViolationProjection{{
				Code:    "spec_section_projection_input_invalid",
				Message: err.Error(),
			}},
		)
		report.EntityOfConcern = &concernProjection
		return report
	}
	adapted := specsectionadapter.Adapt(
		draft,
		exactRuntime,
		concern,
	)
	return runtime.admitTaskMemoryCandidate(
		ctx,
		artifactProjection,
		concernProjection,
		adapted,
		exactRuntime.SourceMode(),
		mode,
	)
}

func (runtime *taskMemoryProjectionRuntime) admitTaskMemoryCandidate(
	ctx context.Context,
	artifactProjection taskMemoryArtifactProjection,
	concern taskMemoryConcernProjection,
	adapted recordatconcern.Result,
	sourceMode adaptersource.Mode,
	mode taskMemoryProjectionMode,
) taskMemoryProjectionReport {
	switch result := adapted.(type) {
	case recordatconcern.Invalid:
		return invalidTaskMemoryProjectionReport(
			artifactProjection,
			projectTaskMemoryViolations(result.Violations()),
		)
	case recordatconcern.Underdetermined:
		return underdeterminedTaskMemoryProjectionReport(
			artifactProjection,
			projectTaskMemoryMissingBasis(result.MissingBasis()),
		)
	case recordatconcern.ValidCandidate:
		return runtime.validateAndAdmitTaskMemoryCandidate(
			ctx,
			artifactProjection,
			concern,
			result,
			sourceMode,
			mode,
		)
	default:
		return unavailableTaskMemoryProjectionReport(
			artifactProjection,
			fmt.Sprintf(
				"task adapter returned unsupported result %T",
				adapted,
			),
		)
	}
}

func (runtime *taskMemoryProjectionRuntime) validateAndAdmitTaskMemoryCandidate(
	ctx context.Context,
	artifactProjection taskMemoryArtifactProjection,
	concern taskMemoryConcernProjection,
	candidate recordatconcern.ValidCandidate,
	sourceMode adaptersource.Mode,
	mode taskMemoryProjectionMode,
) taskMemoryProjectionReport {
	current, err := runtime.basis.snapshotLoader.LoadCurrentProjectSnapshot(
		ctx,
		runtime.projectID,
	)
	if err != nil {
		return unavailableTaskMemoryProjectionReport(
			artifactProjection,
			fmt.Sprintf(
				"load current task-memory snapshot before projection: %v",
				err,
			),
		)
	}
	if taskMemoryCandidateAlreadyProjected(
		current.Snapshot(),
		candidate,
	) {
		return alreadyProjectedTaskMemoryProjectionReport(
			artifactProjection,
			concern,
			candidate,
		)
	}
	stage, err := recordatconcern.SealPreAdmissionSourceStage(candidate)
	if err != nil {
		return unavailableTaskMemoryProjectionReport(
			artifactProjection,
			fmt.Sprintf("seal task-memory source stage: %v", err),
		)
	}
	if err := sourceMode.Verify(); err != nil {
		return unavailableTaskMemoryProjectionReport(
			artifactProjection,
			fmt.Sprintf("verify task-memory adapter source mode: %v", err),
		)
	}
	overlayLoader, err := taskMemoryValidationOverlayLoader(
		runtime.basis.snapshotLoader,
		stage,
		sourceMode,
	)
	if err != nil {
		return unavailableTaskMemoryProjectionReport(
			artifactProjection,
			fmt.Sprintf("compose task-memory validation overlay: %v", err),
		)
	}
	source, err := projectmemory.NewCurrentProjectBasisSource(
		overlayLoader,
	)
	if err != nil {
		return unavailableTaskMemoryProjectionReport(
			artifactProjection,
			fmt.Sprintf("construct task-memory project basis: %v", err),
		)
	}
	validation, err := projectmemory.NewValidationRuntime(
		runtime.projectID,
		source,
	)
	if err != nil {
		return unavailableTaskMemoryProjectionReport(
			artifactProjection,
			fmt.Sprintf("construct task-memory validation runtime: %v", err),
		)
	}
	outcome, err := validation.EvaluateCandidate(
		ctx,
		typedmemorywire.ProjectCurrentSelector{},
		candidate.ChangeSet(),
	)
	if err != nil {
		return unavailableTaskMemoryProjectionReport(
			artifactProjection,
			fmt.Sprintf("validate task-memory candidate: %v", err),
		)
	}
	valid, accepted := outcome.(typedmemoryvalidation.ValidOutcome)
	if !accepted {
		return notAdmittedTaskMemoryProjectionReport(
			artifactProjection,
			concern,
			candidate,
			outcome,
		)
	}
	if mode == taskMemoryProjectionDryRun {
		return validatedOnlyTaskMemoryProjectionReport(
			artifactProjection,
			concern,
			candidate,
			valid,
		)
	}
	adapterBuilder :=
		typedmemorystore.NewProjectExecutableGenericSQLiteAdapterBuilder(
			runtime.database,
		).
			SetTypeEnvLoader(projectmemory.NewBaseTypeEnvLoader()).
			SetClock(typedmemorystore.SystemClock{}).
			SetReferenceEngine(
				typedmemorystore.NewExactPersistedStrongReferenceEngine(),
			).
			SetObservableInputs(stage).
			SetSelectedProjectRuntime(runtime.basis.selectedRuntime)
	if sourceMode.IsCurrentKindClassification() {
		adapterBuilder = adapterBuilder.SetKindClassificationSources(stage)
	}
	adapter, err := adapterBuilder.Build()
	if err != nil {
		return unavailableTaskMemoryProjectionReport(
			artifactProjection,
			fmt.Sprintf("construct task-memory commit adapter: %v", err),
		)
	}
	admission, err := projectmemory.NewAdmissionRuntime(
		runtime.projectID,
		source,
		adapter,
	)
	if err != nil {
		return unavailableTaskMemoryProjectionReport(
			artifactProjection,
			fmt.Sprintf("construct task-memory admission runtime: %v", err),
		)
	}
	candidateDigest, err := candidate.ChangeSet().Digest()
	if err != nil {
		return unavailableTaskMemoryProjectionReport(
			artifactProjection,
			fmt.Sprintf("digest task-memory candidate: %v", err),
		)
	}
	idempotencyKey, err := taskMemoryIdempotencyKey(
		artifactProjection,
		candidateDigest,
	)
	if err != nil {
		return unavailableTaskMemoryProjectionReport(
			artifactProjection,
			fmt.Sprintf("construct task-memory idempotency key: %v", err),
		)
	}
	provenance, err := taskMemoryArtifactProvenance(
		artifactProjection,
	)
	if err != nil {
		return unavailableTaskMemoryProjectionReport(
			artifactProjection,
			fmt.Sprintf("construct task-memory provenance: %v", err),
		)
	}
	receipt, err := admission.AdmitValidated(
		ctx,
		valid,
		idempotencyKey,
		provenance,
	)
	if errors.Is(err, typedmemorystore.ErrCommitOutcomeUnknown) {
		return unknownTaskMemoryProjectionReport(
			runtime.projectID,
			artifactProjection,
			concern,
			candidate,
			idempotencyKey,
			candidateDigest,
			err,
		)
	}
	if err != nil {
		return unavailableValidTaskMemoryProjectionReport(
			artifactProjection,
			concern,
			candidate,
			fmt.Sprintf("commit task-memory candidate: %v", err),
		)
	}
	return committedTaskMemoryProjectionReport(
		artifactProjection,
		concern,
		candidate,
		receipt,
	)
}

func taskMemoryCandidateAlreadyProjected(
	snapshot typedmemory.MemorySnapshot,
	candidate recordatconcern.ValidCandidate,
) bool {
	assertions := 0
	for _, change := range candidate.ChangeSet().Changes() {
		assertionChange, isAssertion :=
			change.(typedmemory.AssertRelation)
		if !isAssertion {
			continue
		}
		assertions++
		state := snapshot.AssertionState(
			assertionChange.Assertion().Assertion(),
		)
		if _, active := state.(typedmemory.ActiveAssertion); !active {
			return false
		}
	}
	return assertions > 0
}

func buildTaskMemoryExactRuntime(
	projectID projectidentity.ProjectID,
	current typedmemorystore.CurrentProjectSnapshot,
	basis projectMemoryRuntimeBasis,
) (recordatconcern.ExactRuntimeBasis, error) {
	target, err := basis.selectedTargetFor(current)
	if err != nil {
		return recordatconcern.ExactRuntimeBasis{}, err
	}
	runtimeDigest, err := typedmemorystore.NewSelectedRuntimeBasisDigest(
		target.RuntimeBasis().Digest(),
	)
	if err != nil {
		return recordatconcern.ExactRuntimeBasis{}, err
	}
	registry, present := target.ExactRuntimeRegistry()
	if !present {
		return recordatconcern.ExactRuntimeBasis{}, fmt.Errorf(
			"installed target has no exact runtime registry",
		)
	}
	coordinate, present := registry.CoordinateDigest()
	if !present {
		return recordatconcern.ExactRuntimeBasis{}, fmt.Errorf(
			"installed target registry has no exact coordinate digest",
		)
	}
	registryDigest, err :=
		typedmemorystore.NewExactTargetRegistryCoordinateDigest(
			coordinate,
		)
	if err != nil {
		return recordatconcern.ExactRuntimeBasis{}, err
	}
	sourceMode, err := selectTaskMemoryAdapterSourceMode(target)
	if err != nil {
		return recordatconcern.ExactRuntimeBasis{}, err
	}
	coordinates := recordatconcern.NewExactRuntimeBasisBuilder(projectID).
		SetGraphRevision(current.Snapshot().GraphRevision()).
		SetEnvironment(current.Environment()).
		SetCodecs(current.Codecs()).
		SetSelectedRuntimeCoordinates(
			runtimeDigest,
			registryDigest,
		)
	if sourceMode.IsCurrentKindClassification() {
		return coordinates.
			SetCurrentKindClassification().
			Build()
	}
	registration, err := selectTaskMemoryRegistrationPolicy(
		target.RegistrationPolicies(),
		recordcarrier.NewRecordMembershipEvaluatorV1().RuleRef(),
	)
	if err != nil {
		return recordatconcern.ExactRuntimeBasis{}, err
	}
	return coordinates.
		SetRegistrationPolicy(registration).
		Build()
}

func (basis projectMemoryRuntimeBasis) selectedTargetFor(
	current typedmemorystore.CurrentProjectSnapshot,
) (localpracticeruntime.Target, error) {
	selected := current.Environment().Ref()
	target, present := basis.targetsByTypeEnv[selected.String()]
	if !present {
		return localpracticeruntime.Target{}, fmt.Errorf(
			"selected project TypeEnv %s has no exact installed Local-Practice target",
			selected.String(),
		)
	}
	if target.Composite().Ref() != selected ||
		target.RuntimeBasis().Ref() != target.Composite().RuntimeEvaluationBasisRef() {
		return localpracticeruntime.Target{}, fmt.Errorf(
			"installed Local-Practice target is uncorrelated with selected project TypeEnv %s",
			selected.String(),
		)
	}
	return target, nil
}

func selectTaskMemoryAdapterSourceMode(
	target localpracticeruntime.Target,
) (adaptersource.Mode, error) {
	registry, present := target.ExactRuntimeRegistry()
	if !present {
		return adaptersource.Mode{}, fmt.Errorf(
			"installed target has no exact runtime registry",
		)
	}
	classification, classificationPresent := registry.KindClassificationRegistry()
	membership, membershipPresent := registry.MemberOfRegistry()
	classificationReady := classificationPresent && classification.Len() > 0
	membershipReady := membershipPresent && membership.Len() > 0
	policies := target.RegistrationPolicies()
	if classificationReady && !membershipReady && len(policies) == 0 {
		return adaptersource.CurrentKindClassification(), nil
	}
	if membershipReady && !classificationReady {
		return adaptersource.HistoricalMembership(), nil
	}
	return adaptersource.Mode{}, fmt.Errorf(
		"installed target source posture is ambiguous: kind_classification=%t member_of=%t historical_policies=%d",
		classificationReady,
		membershipReady,
		len(policies),
	)
}

func taskMemoryValidationOverlayLoader(
	base typedmemorystore.CurrentProjectSnapshotLoader,
	stage recordatconcern.PreAdmissionSourceStage,
	mode adaptersource.Mode,
) (typedmemorystore.CurrentProjectSnapshotLoader, error) {
	if err := mode.Verify(); err != nil {
		return nil, err
	}
	if mode.IsCurrentKindClassification() {
		return typedmemorystore.NewCurrentProjectSnapshotLoaderWithKindClassificationSourceOverlay(
			base,
			stage,
		)
	}
	return typedmemorystore.NewCurrentProjectSnapshotLoaderWithObservableInputOverlay(
		base,
		stage,
	)
}

func selectTaskMemoryRegistrationPolicy(
	policies []recordmembershipregistration.RegistrationArtifactV1,
	rule typedmemory.RuleRef,
) (recordmembershipregistration.RegistrationArtifactV1, error) {
	matches := make(
		[]recordmembershipregistration.RegistrationArtifactV1,
		0,
		1,
	)
	for _, policy := range policies {
		if policy.Evaluator().Rule() == rule {
			matches = append(matches, policy)
		}
	}
	if len(matches) != 1 {
		return recordmembershipregistration.RegistrationArtifactV1{}, fmt.Errorf(
			"selected task-memory runtime has %d record-membership policies; want exactly one",
			len(matches),
		)
	}
	if err := matches[0].Verify(); err != nil {
		return recordmembershipregistration.RegistrationArtifactV1{}, err
	}
	return matches[0], nil
}

func buildTaskMemoryNoteDraft(
	projectID projectidentity.ProjectID,
	current typedmemorystore.CurrentProjectSnapshot,
	record *artifact.Artifact,
	args map[string]any,
	concern taskMemoryConcernInput,
) (recordatconcern.Draft, error) {
	return buildTaskMemoryRecordDraft(
		projectID,
		current,
		record,
		concern,
		noteTaskMemoryClaims(args),
	)
}

func buildTaskMemoryProblemDraft(
	projectID projectidentity.ProjectID,
	current typedmemorystore.CurrentProjectSnapshot,
	record *artifact.Artifact,
	args map[string]any,
	concern taskMemoryConcernInput,
) (recordatconcern.Draft, error) {
	return buildTaskMemoryRecordDraft(
		projectID,
		current,
		record,
		concern,
		problemTaskMemoryClaims(args),
	)
}

func buildTaskMemorySolutionPortfolioDraft(
	projectID projectidentity.ProjectID,
	current typedmemorystore.CurrentProjectSnapshot,
	record *artifact.Artifact,
	concern taskMemoryConcernInput,
) (
	solutionportfolioadapter.Draft,
	[]taskMemoryMissingBasisProjection,
	error,
) {
	options, missing, err := resolveTaskMemorySolutionOptionRefs(
		current,
		record,
		concern,
	)
	if len(missing) > 0 || err != nil {
		return solutionportfolioadapter.Draft{}, missing, err
	}
	recordDraft, err := buildTaskMemoryRecordDraftInput(
		projectID,
		current,
		record,
		concern,
		solutionPortfolioTaskMemoryClaims(record),
	)
	if err != nil {
		return solutionportfolioadapter.Draft{}, nil, err
	}
	draft, err := solutionportfolioadapter.NewDraft(
		solutionportfolioadapter.DraftInput{
			Record:  recordDraft,
			Options: options,
		},
	)
	if err != nil {
		return solutionportfolioadapter.Draft{}, nil, err
	}
	return draft, nil, nil
}

func buildTaskMemoryPortfolioComparisonDraft(
	projectID projectidentity.ProjectID,
	current typedmemorystore.CurrentProjectSnapshot,
	record *artifact.Artifact,
	concern taskMemoryConcernInput,
) (
	portfoliocomparisonadapter.Draft,
	[]taskMemoryMissingBasisProjection,
	error,
) {
	fields := record.UnmarshalPortfolioFields()
	comparison := fields.Comparison
	if comparison == nil {
		return portfoliocomparisonadapter.Draft{},
			nil,
			fmt.Errorf(
				"portfolio %s has no persisted comparison result",
				record.Meta.ID,
			)
	}
	portfolioReferenceID := "record:" + record.Meta.ID
	portfolio, portfolioMissing, err :=
		resolveTaskMemoryProjectRecordRef(
			current,
			concern,
			portfolioReferenceID,
			"solution_portfolio_project_record_ref",
		)
	if err != nil {
		return portfoliocomparisonadapter.Draft{}, nil, err
	}
	if portfolioMissing != nil {
		return portfoliocomparisonadapter.Draft{},
			[]taskMemoryMissingBasisProjection{*portfolioMissing},
			nil
	}
	comparedIDs := taskMemorySortedMapKeys(comparison.Scores)
	compared, comparedMissing, err :=
		resolveTaskMemoryComparisonOptionRefs(
			current,
			record,
			concern,
			comparedIDs,
			"compared_option_project_record_ref",
		)
	if err != nil {
		return portfoliocomparisonadapter.Draft{}, nil, err
	}
	nonDominated, nonDominatedMissing, err :=
		resolveTaskMemoryComparisonOptionRefs(
			current,
			record,
			concern,
			comparison.NonDominatedSet,
			"non_dominated_option_project_record_ref",
		)
	if err != nil {
		return portfoliocomparisonadapter.Draft{}, nil, err
	}
	missing := append(
		[]taskMemoryMissingBasisProjection(nil),
		comparedMissing...,
	)
	missing = append(missing, nonDominatedMissing...)
	if len(missing) > 0 {
		return portfoliocomparisonadapter.Draft{}, missing, nil
	}
	recordDraft, err := buildTaskMemoryPortfolioComparisonRecordDraftInput(
		projectID,
		current,
		record,
		concern,
	)
	if err != nil {
		return portfoliocomparisonadapter.Draft{}, nil, err
	}
	draft, err := portfoliocomparisonadapter.NewDraft(
		portfoliocomparisonadapter.DraftInput{
			Record:              recordDraft,
			Portfolio:           portfolio,
			ComparedOptions:     compared,
			NonDominatedOptions: nonDominated,
		},
	)
	if err != nil {
		return portfoliocomparisonadapter.Draft{}, nil, err
	}
	return draft, nil, nil
}

type taskMemoryDecisionPortfolioBasis struct {
	portfolio typedmemory.PersistedRef
	concern   typedmemory.PersistedRef
	options   []typedmemory.PersistedRef
	context   typedmemory.BoundedContextRef
}

func buildTaskMemoryDecisionProjectionDraft(
	ctx context.Context,
	projectID projectidentity.ProjectID,
	current typedmemorystore.CurrentProjectSnapshot,
	frame typedmemorystore.CurrentProjectReadFrame,
	record *artifact.Artifact,
	source decisionrecordadapter.ExistingDecisionChoiceSource,
	store *artifact.Store,
) (
	decisionrecordadapter.Draft,
	*taskMemoryConcernProjection,
	[]taskMemoryMissingBasisProjection,
	error,
) {
	fields := record.UnmarshalDecisionFields()
	choice := artifact.NormalizeChoiceResult(fields.ChoiceResult)
	if choice == nil {
		return decisionrecordadapter.Draft{},
			nil,
			nil,
			fmt.Errorf(
				"DecisionRecord %s has no exact ChoiceResult",
				record.Meta.ID,
			)
	}
	portfolioSourceRef := strings.TrimSpace(choice.PortfolioRef)
	if portfolioSourceRef == "" {
		return decisionrecordadapter.Draft{},
			nil,
			[]taskMemoryMissingBasisProjection{{
				Name:   "decision_option_record_mapping",
				Repair: "repair:bind-choice-to-typed-solution-portfolio",
			}},
			nil
	}
	portfolioRecord, err := store.Get(
		ctx,
		portfolioSourceRef,
	)
	if err != nil {
		return decisionrecordadapter.Draft{},
			nil,
			nil,
			fmt.Errorf(
				"load DecisionRecord portfolio %s: %w",
				portfolioSourceRef,
				err,
			)
	}
	if portfolioRecord.Meta.Kind != artifact.KindSolutionPortfolio {
		return decisionrecordadapter.Draft{},
			nil,
			nil,
			fmt.Errorf(
				"DecisionRecord portfolio %s is %s, want SolutionPortfolio",
				portfolioSourceRef,
				portfolioRecord.Meta.Kind,
			)
	}
	portfolioBasis, missing, err :=
		resolveTaskMemoryDecisionPortfolioBasis(
			frame,
			"record:"+portfolioSourceRef,
		)
	if missing != nil {
		return decisionrecordadapter.Draft{},
			nil,
			[]taskMemoryMissingBasisProjection{*missing},
			nil
	}
	if err != nil {
		return decisionrecordadapter.Draft{}, nil, nil, err
	}
	resolution := current.Snapshot().ResolveReference(
		portfolioBasis.concern,
		portfolioBasis.context,
	)
	resolvedConcern, present :=
		resolution.(typedmemory.ResolvedStrongReference)
	if !present {
		return decisionrecordadapter.Draft{},
			nil,
			[]taskMemoryMissingBasisProjection{{
				Name:   "decision_entity_of_concern_resolution",
				Repair: taskMemoryResolutionRepair(resolution),
			}},
			nil
	}
	concernBinding, err := decisionrecordadapter.NewExactConcernBinding(
		resolvedConcern,
	)
	if err != nil {
		return decisionrecordadapter.Draft{}, nil, nil, err
	}
	concernProjection := &taskMemoryConcernProjection{
		RefKindID: portfolioBasis.concern.
			RefKind().
			ID().
			String(),
		ReferenceID: portfolioBasis.concern.
			ReferenceID().
			String(),
		EntityID:        resolvedConcern.Entity().String(),
		BoundedContext:  resolvedConcern.Context().String(),
		ResolutionBasis: resolvedConcern.Basis().String(),
	}
	concernInput := taskMemoryConcernInput{
		refKindID:   concernProjection.RefKindID,
		referenceID: concernProjection.ReferenceID,
		context:     concernProjection.BoundedContext,
	}
	optionBindings, optionMissing, err :=
		buildTaskMemoryDecisionOptionBindings(
			current,
			portfolioRecord,
			choice.OptionSet,
			portfolioBasis.options,
			concernInput,
		)
	if err != nil {
		return decisionrecordadapter.Draft{},
			concernProjection,
			nil,
			err
	}
	if len(optionMissing) > 0 {
		return decisionrecordadapter.Draft{},
			concernProjection,
			optionMissing,
			nil
	}
	problem, problemMissing, err :=
		buildTaskMemoryDecisionProblemReference(
			current,
			choice.ProblemRefs,
			concernInput,
		)
	if err != nil {
		return decisionrecordadapter.Draft{},
			concernProjection,
			nil,
			err
	}
	if problemMissing != nil {
		return decisionrecordadapter.Draft{},
			concernProjection,
			[]taskMemoryMissingBasisProjection{*problemMissing},
			nil
	}
	portfolio, err :=
		decisionrecordadapter.NewExactProjectRecordReference(
			portfolioSourceRef,
			portfolioBasis.portfolio,
		)
	if err != nil {
		return decisionrecordadapter.Draft{},
			concernProjection,
			nil,
			err
	}
	draft, err := newTaskMemoryDecisionProjectionDraft(
		projectID,
		record,
		source,
		portfolioBasis.context,
		concernBinding,
		problem,
		portfolio,
		optionBindings,
	)
	if err != nil {
		return decisionrecordadapter.Draft{},
			concernProjection,
			nil,
			err
	}
	return draft, concernProjection, nil, nil
}

func resolveTaskMemoryDecisionPortfolioBasis(
	frame typedmemorystore.CurrentProjectReadFrame,
	portfolioReferenceID string,
) (
	taskMemoryDecisionPortfolioBasis,
	*taskMemoryMissingBasisProjection,
	error,
) {
	matches := make(
		[]typedmemorystore.CurrentAssertionCarrier,
		0,
		1,
	)
	for _, active := range frame.GraphObservation().
		ActiveAssertions().
		Relations() {
		relation, usable := taskMemoryAssertionCarrier(active)
		if !usable {
			continue
		}
		if relation.Signature().ID().String() !=
			"Haft.SolutionPortfolioAtConcern" {
			continue
		}
		portfolioRefs, err := taskMemoryRelationReferences(
			relation,
			"Haft.SolutionPortfolioAtConcern.PortfolioSlot",
		)
		if err != nil {
			return taskMemoryDecisionPortfolioBasis{}, nil, err
		}
		if len(portfolioRefs) == 1 &&
			portfolioRefs[0].ReferenceID().String() ==
				portfolioReferenceID {
			matches = append(matches, relation)
		}
	}
	if len(matches) == 0 {
		missing := taskMemoryMissingBasisProjection{
			Name: "decision_portfolio_typed_relation:" +
				portfolioReferenceID,
			Repair: "repair:project-solution-portfolio-at-concern",
		}
		return taskMemoryDecisionPortfolioBasis{}, &missing, nil
	}
	if len(matches) != 1 {
		return taskMemoryDecisionPortfolioBasis{}, nil, fmt.Errorf(
			"typed portfolio %s has %d active SolutionPortfolioAtConcern relations; want exactly one",
			portfolioReferenceID,
			len(matches),
		)
	}
	relation := matches[0]
	portfolio, err := taskMemorySingleRelationReference(
		relation,
		"Haft.SolutionPortfolioAtConcern.PortfolioSlot",
	)
	if err != nil {
		return taskMemoryDecisionPortfolioBasis{}, nil, err
	}
	concern, err := taskMemorySingleRelationReference(
		relation,
		"Haft.SolutionPortfolioAtConcern.EntityOfConcernSlot",
	)
	if err != nil {
		return taskMemoryDecisionPortfolioBasis{}, nil, err
	}
	options, err := taskMemoryRelationReferences(
		relation,
		"Haft.SolutionPortfolioAtConcern.OptionSlot",
	)
	if err != nil {
		return taskMemoryDecisionPortfolioBasis{}, nil, err
	}
	if len(options) < 2 {
		return taskMemoryDecisionPortfolioBasis{}, nil, fmt.Errorf(
			"typed portfolio %s has %d option references; want at least two",
			portfolioReferenceID,
			len(options),
		)
	}
	return taskMemoryDecisionPortfolioBasis{
		portfolio: portfolio,
		concern:   concern,
		options:   options,
		context:   relation.Context(),
	}, nil, nil
}

func taskMemoryRelationReferences(
	relation typedmemorystore.CurrentAssertionCarrier,
	slotName string,
) ([]typedmemory.PersistedRef, error) {
	for _, binding := range relation.Bindings() {
		if binding.Name().String() != slotName {
			continue
		}
		references := make(
			[]typedmemory.PersistedRef,
			0,
			len(binding.Fillers()),
		)
		for _, filler := range binding.Fillers() {
			reference, ok := filler.(typedmemory.ReferenceFiller)
			if !ok {
				return nil, fmt.Errorf(
					"relation %s slot %s contains a non-reference filler",
					relation.Signature().ID(),
					slotName,
				)
			}
			references = append(
				references,
				reference.Reference(),
			)
		}
		return references, nil
	}
	return nil, fmt.Errorf(
		"relation %s omits required slot %s",
		relation.Signature().ID(),
		slotName,
	)
}

func taskMemorySingleRelationReference(
	relation typedmemorystore.CurrentAssertionCarrier,
	slotName string,
) (typedmemory.PersistedRef, error) {
	references, err := taskMemoryRelationReferences(
		relation,
		slotName,
	)
	if err != nil {
		return typedmemory.PersistedRef{}, err
	}
	if len(references) != 1 {
		return typedmemory.PersistedRef{}, fmt.Errorf(
			"relation %s slot %s has %d references; want exactly one",
			relation.Signature().ID(),
			slotName,
			len(references),
		)
	}
	return references[0], nil
}

func taskMemoryAssertionCarrier(
	active typedmemorystore.CurrentActiveAssertion,
) (typedmemorystore.CurrentAssertionCarrier, bool) {
	carrier := active.Carrier()
	switch value := carrier.(type) {
	case typedmemorystore.CurrentLegacyRelation:
		return carrier, true
	case typedmemorystore.CurrentRelationalAssertion:
		// This receiving use consumes a positive typed assertion as its
		// binding basis. A denial or unknown posture cannot satisfy that use;
		// AffirmsObtaining still remains assertion content, not an occurrence.
		usable := value.Assertion().Modality().Kind() ==
			typedmemory.AssertionModalityAffirmsObtaining
		return carrier, usable
	default:
		return nil, false
	}
}

func buildTaskMemoryDecisionOptionBindings(
	current typedmemorystore.CurrentProjectSnapshot,
	portfolio *artifact.Artifact,
	optionLabels []string,
	typedOptions []typedmemory.PersistedRef,
	concern taskMemoryConcernInput,
) (
	[]decisionrecordadapter.DecisionOptionBinding,
	[]taskMemoryMissingBasisProjection,
	error,
) {
	fields := portfolio.UnmarshalPortfolioFields()
	bindings := make(
		[]decisionrecordadapter.DecisionOptionBinding,
		0,
		len(optionLabels),
	)
	missing := make(
		[]taskMemoryMissingBasisProjection,
		0,
		len(optionLabels),
	)
	typedOptionSet := make(
		map[string]struct{},
		len(typedOptions),
	)
	for _, reference := range typedOptions {
		typedOptionSet[reference.ReferenceID().String()] = struct{}{}
	}
	for _, label := range optionLabels {
		variant, err := taskMemoryPortfolioVariantForChoiceLabel(
			fields.Variants,
			label,
		)
		if err != nil {
			return nil, nil, err
		}
		if variant.ProjectRecordRef == nil {
			missing = append(
				missing,
				taskMemoryMissingBasisProjection{
					Name: "decision_option_project_record_ref:" +
						strings.TrimSpace(label),
					Repair: "repair:provide-exact-project-record-ref",
				},
			)
			continue
		}
		if strings.TrimSpace(variant.ProjectRecordRef.RefKindID) !=
			"Haft.ProjectRecordRef" {
			return nil, nil, fmt.Errorf(
				"decision option %q project_record_ref has ref_kind_id %q; want %q",
				label,
				variant.ProjectRecordRef.RefKindID,
				"Haft.ProjectRecordRef",
			)
		}
		persisted, unresolved, err :=
			resolveTaskMemoryProjectRecordRef(
				current,
				concern,
				variant.ProjectRecordRef.ReferenceID,
				"decision_option_project_record_ref:"+
					strings.TrimSpace(label),
			)
		if err != nil {
			return nil, nil, err
		}
		if unresolved != nil {
			missing = append(missing, *unresolved)
			continue
		}
		if _, partOfPortfolio :=
			typedOptionSet[persisted.ReferenceID().String()]; !partOfPortfolio {
			return nil, nil, fmt.Errorf(
				"decision option %q reference %s is not in the typed portfolio option set",
				label,
				persisted.ReferenceID(),
			)
		}
		binding, err :=
			decisionrecordadapter.NewDecisionOptionBinding(
				label,
				persisted,
			)
		if err != nil {
			return nil, nil, err
		}
		bindings = append(bindings, binding)
	}
	if len(missing) > 0 {
		return nil, missing, nil
	}
	return bindings, nil, nil
}

func taskMemoryPortfolioVariantForChoiceLabel(
	variants []artifact.Variant,
	rawLabel string,
) (artifact.Variant, error) {
	label := strings.TrimSpace(rawLabel)
	matches := make([]artifact.Variant, 0, 1)
	for _, variant := range variants {
		if strings.TrimSpace(variant.ID) == label ||
			strings.TrimSpace(variant.Title) == label {
			matches = append(matches, variant)
		}
	}
	if len(matches) != 1 {
		return artifact.Variant{}, fmt.Errorf(
			"DecisionRecord option label %q matches %d portfolio variants; want exactly one exact ID or title",
			rawLabel,
			len(matches),
		)
	}
	return matches[0], nil
}

func buildTaskMemoryDecisionProblemReference(
	current typedmemorystore.CurrentProjectSnapshot,
	problemRefs []string,
	concern taskMemoryConcernInput,
) (
	decisionrecordadapter.OptionalProjectRecordReference,
	*taskMemoryMissingBasisProjection,
	error,
) {
	if len(problemRefs) == 0 {
		return decisionrecordadapter.NoProjectRecordReference(),
			nil,
			nil
	}
	if len(problemRefs) != 1 {
		return decisionrecordadapter.OptionalProjectRecordReference{},
			nil,
			fmt.Errorf(
				"DecisionChoiceAtConcern v1 supports one problem record; source names %d",
				len(problemRefs),
			)
	}
	sourceRef := strings.TrimSpace(problemRefs[0])
	persisted, unresolved, err := resolveTaskMemoryProjectRecordRef(
		current,
		concern,
		"record:"+sourceRef,
		"decision_problem_project_record_ref:"+sourceRef,
	)
	if err != nil {
		return decisionrecordadapter.OptionalProjectRecordReference{},
			nil,
			err
	}
	if unresolved != nil {
		return decisionrecordadapter.OptionalProjectRecordReference{},
			unresolved,
			nil
	}
	exact, err :=
		decisionrecordadapter.NewExactProjectRecordReference(
			sourceRef,
			persisted,
		)
	if err != nil {
		return decisionrecordadapter.OptionalProjectRecordReference{},
			nil,
			err
	}
	return exact, nil, nil
}

func newTaskMemoryDecisionProjectionDraft(
	projectID projectidentity.ProjectID,
	record *artifact.Artifact,
	source decisionrecordadapter.ExistingDecisionChoiceSource,
	contextRef typedmemory.BoundedContextRef,
	concern decisionrecordadapter.ExactConcernBinding,
	problem decisionrecordadapter.OptionalProjectRecordReference,
	portfolio decisionrecordadapter.OptionalProjectRecordReference,
	options []decisionrecordadapter.DecisionOptionBinding,
) (decisionrecordadapter.Draft, error) {
	entity, err := typedmemory.NewEntityID(
		source.DecisionRecordRef(),
	)
	if err != nil {
		return decisionrecordadapter.Draft{}, err
	}
	localRef, err := typedmemory.NewBatchLocalRef(
		source.DecisionRecordRef(),
	)
	if err != nil {
		return decisionrecordadapter.Draft{}, err
	}
	label, err := typedmemory.NewEntityLabel(
		source.Title(),
	)
	if err != nil {
		return decisionrecordadapter.Draft{}, err
	}
	assertion, err := typedmemory.NewAssertionID(
		fmt.Sprintf(
			"assertion:%s:choice:v%d:at-concern",
			record.Meta.ID,
			record.Meta.Version,
		),
	)
	if err != nil {
		return decisionrecordadapter.Draft{}, err
	}
	gamma, err := typedmemory.NewGammaPoint(
		record.Meta.CreatedAt,
	)
	if err != nil {
		return decisionrecordadapter.Draft{}, err
	}
	contextSlice, err := typedmemory.NewContextSlice(
		typedmemory.ContextSliceInput{
			Context:   contextRef,
			GammaTime: gamma,
		},
	)
	if err != nil {
		return decisionrecordadapter.Draft{}, err
	}
	contextProjection, err :=
		decisionrecordadapter.NewLegacyContextProjection(
			source,
			contextRef,
		)
	if err != nil {
		return decisionrecordadapter.Draft{}, err
	}
	return decisionrecordadapter.NewDraft(
		decisionrecordadapter.ProjectionDraftInput{
			ProjectID:         projectID,
			RecordEntity:      entity,
			RecordLocalRef:    localRef,
			RecordLabel:       label,
			AssertionID:       assertion,
			ContextSlice:      contextSlice,
			Source:            source,
			ContextProjection: contextProjection,
			Concern:           concern,
			Problem:           problem,
			Portfolio:         portfolio,
			Options:           options,
			Comparison: decisionrecordadapter.
				NoProjectRecordReference(),
		},
	)
}

func resolveTaskMemorySolutionOptionRefs(
	current typedmemorystore.CurrentProjectSnapshot,
	record *artifact.Artifact,
	concern taskMemoryConcernInput,
) (
	[]typedmemory.PersistedRef,
	[]taskMemoryMissingBasisProjection,
	error,
) {
	contextRef, err := typedmemory.NewBoundedContextRef(
		concern.context,
	)
	if err != nil {
		return nil, nil, err
	}
	refKindID, err := typedmemory.NewRefKindID(
		"Haft.ProjectRecordRef",
	)
	if err != nil {
		return nil, nil, err
	}
	refKind, err := typedmemory.NewRefKindRef(
		current.Environment().Ref(),
		refKindID,
	)
	if err != nil {
		return nil, nil, err
	}
	fields := record.UnmarshalPortfolioFields()
	options := make(
		[]typedmemory.PersistedRef,
		0,
		len(fields.Variants),
	)
	missing := make(
		[]taskMemoryMissingBasisProjection,
		0,
		len(fields.Variants),
	)
	for _, variant := range fields.Variants {
		reference := variant.ProjectRecordRef
		if reference == nil {
			missing = append(
				missing,
				missingTaskMemorySolutionOptionRef(variant.ID),
			)
			continue
		}
		if strings.TrimSpace(reference.RefKindID) != refKindID.String() {
			return nil, nil, fmt.Errorf(
				"variant %s project_record_ref has ref_kind_id %q; want %q",
				variant.ID,
				reference.RefKindID,
				refKindID.String(),
			)
		}
		referenceID, err := typedmemory.NewReferenceID(
			strings.TrimSpace(reference.ReferenceID),
		)
		if err != nil {
			return nil, nil, fmt.Errorf(
				"variant %s project_record_ref: %w",
				variant.ID,
				err,
			)
		}
		persisted, err := typedmemory.NewPersistedRef(
			refKind,
			referenceID,
		)
		if err != nil {
			return nil, nil, fmt.Errorf(
				"variant %s project_record_ref: %w",
				variant.ID,
				err,
			)
		}
		resolution := current.Snapshot().ResolveReference(
			persisted,
			contextRef,
		)
		if _, resolved := resolution.(typedmemory.ResolvedStrongReference); !resolved {
			basis := missingTaskMemorySolutionOptionRef(variant.ID)
			basis.Repair = taskMemoryResolutionRepair(resolution)
			missing = append(missing, basis)
			continue
		}
		options = append(options, persisted)
	}
	if len(missing) > 0 {
		return nil, missing, nil
	}
	return options, nil, nil
}

func resolveTaskMemoryComparisonOptionRefs(
	current typedmemorystore.CurrentProjectSnapshot,
	record *artifact.Artifact,
	concern taskMemoryConcernInput,
	variantIDs []string,
	missingName string,
) (
	[]typedmemory.PersistedRef,
	[]taskMemoryMissingBasisProjection,
	error,
) {
	fields := record.UnmarshalPortfolioFields()
	references := make(
		map[string]*artifact.VariantProjectRecordRef,
		len(fields.Variants),
	)
	for _, variant := range fields.Variants {
		variantID := strings.TrimSpace(variant.ID)
		if variantID == "" {
			return nil, nil, fmt.Errorf(
				"portfolio contains a variant without an ID",
			)
		}
		if _, duplicate := references[variantID]; duplicate {
			return nil, nil, fmt.Errorf(
				"portfolio contains duplicate variant ID %q",
				variantID,
			)
		}
		references[variantID] = variant.ProjectRecordRef
	}
	result := make(
		[]typedmemory.PersistedRef,
		0,
		len(variantIDs),
	)
	missing := make(
		[]taskMemoryMissingBasisProjection,
		0,
		len(variantIDs),
	)
	for _, rawVariantID := range variantIDs {
		variantID := strings.TrimSpace(rawVariantID)
		reference, present := references[variantID]
		if !present {
			return nil, nil, fmt.Errorf(
				"comparison references unknown variant %q",
				variantID,
			)
		}
		if reference == nil {
			missing = append(
				missing,
				taskMemoryMissingBasisProjection{
					Name:   missingName + ":" + variantID,
					Repair: "repair:provide-exact-project-record-ref",
				},
			)
			continue
		}
		if strings.TrimSpace(reference.RefKindID) !=
			"Haft.ProjectRecordRef" {
			return nil, nil, fmt.Errorf(
				"variant %s project_record_ref has ref_kind_id %q; want %q",
				variantID,
				reference.RefKindID,
				"Haft.ProjectRecordRef",
			)
		}
		persisted, unresolved, err := resolveTaskMemoryProjectRecordRef(
			current,
			concern,
			reference.ReferenceID,
			missingName+":"+variantID,
		)
		if err != nil {
			return nil, nil, err
		}
		if unresolved != nil {
			missing = append(missing, *unresolved)
			continue
		}
		result = append(result, persisted)
	}
	if len(missing) > 0 {
		return nil, missing, nil
	}
	return result, nil, nil
}

func resolveTaskMemoryProjectRecordRef(
	current typedmemorystore.CurrentProjectSnapshot,
	concern taskMemoryConcernInput,
	rawReferenceID string,
	missingName string,
) (
	typedmemory.PersistedRef,
	*taskMemoryMissingBasisProjection,
	error,
) {
	contextRef, err := typedmemory.NewBoundedContextRef(
		concern.context,
	)
	if err != nil {
		return typedmemory.PersistedRef{}, nil, err
	}
	refKindID, err := typedmemory.NewRefKindID(
		"Haft.ProjectRecordRef",
	)
	if err != nil {
		return typedmemory.PersistedRef{}, nil, err
	}
	refKind, err := typedmemory.NewRefKindRef(
		current.Environment().Ref(),
		refKindID,
	)
	if err != nil {
		return typedmemory.PersistedRef{}, nil, err
	}
	referenceID, err := typedmemory.NewReferenceID(
		strings.TrimSpace(rawReferenceID),
	)
	if err != nil {
		return typedmemory.PersistedRef{}, nil, err
	}
	persisted, err := typedmemory.NewPersistedRef(
		refKind,
		referenceID,
	)
	if err != nil {
		return typedmemory.PersistedRef{}, nil, err
	}
	resolution := current.Snapshot().ResolveReference(
		persisted,
		contextRef,
	)
	if _, resolved := resolution.(typedmemory.ResolvedStrongReference); resolved {
		return persisted, nil, nil
	}
	missing := taskMemoryMissingBasisProjection{
		Name:   strings.TrimSpace(missingName),
		Repair: taskMemoryResolutionRepair(resolution),
	}
	return typedmemory.PersistedRef{}, &missing, nil
}

func taskMemorySortedMapKeys(
	values map[string]map[string]string,
) []string {
	keys := make(
		[]string,
		0,
		len(values),
	)
	for key := range values {
		keys = append(keys, strings.TrimSpace(key))
	}
	sort.Strings(keys)
	return keys
}

func missingTaskMemorySolutionOptionRef(
	variantID string,
) taskMemoryMissingBasisProjection {
	return taskMemoryMissingBasisProjection{
		Name: "solution_option_project_record_ref:" +
			strings.TrimSpace(variantID),
		Repair: "repair:provide-exact-project-record-ref",
	}
}

func buildTaskMemoryRecordDraft(
	projectID projectidentity.ProjectID,
	current typedmemorystore.CurrentProjectSnapshot,
	record *artifact.Artifact,
	concern taskMemoryConcernInput,
	claims []taskMemoryClaimInput,
) (recordatconcern.Draft, error) {
	input, err := buildTaskMemoryRecordDraftInput(
		projectID,
		current,
		record,
		concern,
		claims,
	)
	if err != nil {
		return recordatconcern.Draft{}, err
	}
	return recordatconcern.NewDraft(input)
}

func buildTaskMemoryRecordDraftInput(
	projectID projectidentity.ProjectID,
	current typedmemorystore.CurrentProjectSnapshot,
	record *artifact.Artifact,
	concern taskMemoryConcernInput,
	claims []taskMemoryClaimInput,
) (recordatconcern.DraftInput, error) {
	provenance, err := taskMemoryArtifactProvenance(
		projectTaskMemoryArtifact(record),
	)
	if err != nil {
		return recordatconcern.DraftInput{}, err
	}
	identity := "record:" + record.Meta.ID
	source := taskMemoryRecordSource{
		recordIdentity: identity,
		claimIdentity:  record.Meta.ID,
		label:          record.Meta.Title,
		assertion: fmt.Sprintf(
			"assertion:%s:v%d:at-concern",
			record.Meta.ID,
			record.Meta.Version,
		),
		provenance: provenance.String(),
		recordedAt: record.Meta.CreatedAt,
		claims:     claims,
	}
	return buildTaskMemoryRecordDraftInputFromSource(
		projectID,
		current,
		concern,
		source,
	)
}

func buildTaskMemoryRecordDraftInputFromSource(
	projectID projectidentity.ProjectID,
	current typedmemorystore.CurrentProjectSnapshot,
	concern taskMemoryConcernInput,
	source taskMemoryRecordSource,
) (recordatconcern.DraftInput, error) {
	claimGraph, err := buildTaskMemoryClaimGraph(
		current.Environment(),
		source.claimIdentity,
		source.claims,
	)
	if err != nil {
		return recordatconcern.DraftInput{}, err
	}
	exactClaimGraph, err := recordatconcern.NewExactClaimGraph(
		claimGraph,
	)
	if err != nil {
		return recordatconcern.DraftInput{}, err
	}
	contextRef, err := typedmemory.NewBoundedContextRef(
		concern.context,
	)
	if err != nil {
		return recordatconcern.DraftInput{}, err
	}
	gamma, err := typedmemory.NewGammaPoint(
		source.recordedAt,
	)
	if err != nil {
		return recordatconcern.DraftInput{}, err
	}
	contextSlice, err := typedmemory.NewContextSlice(
		typedmemory.ContextSliceInput{
			Context:   contextRef,
			GammaTime: gamma,
		},
	)
	if err != nil {
		return recordatconcern.DraftInput{}, err
	}
	recordEntity, err := typedmemory.NewEntityID(
		source.recordIdentity,
	)
	if err != nil {
		return recordatconcern.DraftInput{}, err
	}
	localRef, err := typedmemory.NewBatchLocalRef(
		source.recordIdentity,
	)
	if err != nil {
		return recordatconcern.DraftInput{}, err
	}
	label, err := typedmemory.NewEntityLabel(
		source.label,
	)
	if err != nil {
		return recordatconcern.DraftInput{}, err
	}
	assertionID, err := typedmemory.NewAssertionID(
		source.assertion,
	)
	if err != nil {
		return recordatconcern.DraftInput{}, err
	}
	provenance, err := typedmemory.NewProvenanceRef(
		source.provenance,
	)
	if err != nil {
		return recordatconcern.DraftInput{}, err
	}
	return recordatconcern.DraftInput{
		ProjectID:      projectID,
		RecordEntity:   recordEntity,
		RecordLocalRef: localRef,
		RecordLabel:    label,
		AssertionID:    assertionID,
		ContextSlice:   contextSlice,
		ClaimGraph:     exactClaimGraph,
		Provenance:     provenance,
	}, nil
}

func buildTaskMemorySpecSectionDraft(
	projectID projectidentity.ProjectID,
	current typedmemorystore.CurrentProjectSnapshot,
	edition specflow.SpecSectionEdition,
	editionRef string,
	concern taskMemoryConcernInput,
) (specsectionadapter.Draft, error) {
	label := strings.TrimSpace(edition.Section.Title)
	if label == "" {
		label = edition.SectionID
	}
	source := taskMemoryRecordSource{
		recordIdentity: "record:" + editionRef,
		claimIdentity:  editionRef,
		label:          label,
		assertion:      "assertion:" + editionRef + ":v1:at-concern",
		provenance: "spec-section-edition:" +
			projectID.String() +
			":" +
			edition.SectionID +
			":" +
			edition.SemanticHash,
		recordedAt: edition.UpdatedAt,
		claims:     specSectionTaskMemoryClaims(edition),
	}
	input, err := buildTaskMemoryRecordDraftInputFromSource(
		projectID,
		current,
		concern,
		source,
	)
	if err != nil {
		return specsectionadapter.Draft{}, err
	}
	return specsectionadapter.NewDraft(input)
}

func buildTaskMemoryPortfolioComparisonRecordDraftInput(
	projectID projectidentity.ProjectID,
	current typedmemorystore.CurrentProjectSnapshot,
	record *artifact.Artifact,
	concern taskMemoryConcernInput,
) (recordatconcern.DraftInput, error) {
	artifactProjection :=
		projectTaskMemoryPortfolioComparisonEdition(record)
	provenance, err := taskMemoryArtifactProvenance(
		artifactProjection,
	)
	if err != nil {
		return recordatconcern.DraftInput{}, err
	}
	coordinate := fmt.Sprintf(
		"%s:comparison:v%d",
		record.Meta.ID,
		record.Meta.Version,
	)
	source := taskMemoryRecordSource{
		recordIdentity: "record:" + coordinate,
		claimIdentity:  coordinate,
		label:          artifactProjection.Title,
		assertion:      "assertion:" + coordinate + ":at-concern",
		provenance:     provenance.String(),
		recordedAt:     record.Meta.UpdatedAt,
		claims:         portfolioComparisonTaskMemoryClaims(record),
	}
	return buildTaskMemoryRecordDraftInputFromSource(
		projectID,
		current,
		concern,
		source,
	)
}

func buildTaskMemoryClaimGraph(
	environment typedmemory.TypeEnv,
	artifactRef string,
	claims []taskMemoryClaimInput,
) (typedmemory.ClaimGraphValue, error) {
	textKindID, err := typedmemory.NewKindID("Haft.Text")
	if err != nil {
		return nil, err
	}
	textKind, err := typedmemory.NewValueKindRef(
		environment.Ref(),
		textKindID,
	)
	if err != nil {
		return nil, err
	}
	normalized := normalizeTaskMemoryClaims(claims)
	if len(normalized) == 0 {
		return nil, fmt.Errorf(
			"task carrier has no claim-bearing observation, rationale, or evidence text",
		)
	}
	nodes := make(
		[]typedmemory.ClaimNode,
		0,
		len(normalized),
	)
	for _, claim := range normalized {
		nodeID, err := typedmemory.NewClaimNodeID(
			taskMemoryClaimNodeID(artifactRef, claim),
		)
		if err != nil {
			return nil, err
		}
		node, err := typedmemory.NewClaimNode(
			nodeID,
			textKind,
			typedmemory.NewTextValue(claim.text),
		)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}
	return typedmemory.NewClaimGraphValue(nodes, nil)
}

func noteTaskMemoryClaims(
	args map[string]any,
) []taskMemoryClaimInput {
	claims := make([]taskMemoryClaimInput, 0)
	for _, observation := range parseStringArrayFromArgs(
		args,
		"observations",
	) {
		claims = append(claims, taskMemoryClaimInput{
			role: "observation",
			text: observation,
		})
	}
	for _, field := range []string{"rationale", "evidence"} {
		value, _ := args[field].(string)
		claims = append(claims, taskMemoryClaimInput{
			role: field,
			text: value,
		})
	}
	return claims
}

func problemTaskMemoryClaims(
	args map[string]any,
) []taskMemoryClaimInput {
	scalars := []struct {
		role string
		key  string
	}{
		{role: "problem_type", key: "problem_type"},
		{role: "problem_profile", key: "problem_profile"},
		{role: "source_kind", key: "source_kind"},
		{role: "signal", key: "signal"},
		{role: "why_now", key: "why_now"},
		{role: "scope", key: "scope"},
		{role: "acceptance_probe", key: "acceptance_probe"},
		{role: "freshness_disposition", key: "freshness_disposition"},
		{role: "acceptance", key: "acceptance"},
		{role: "blast_radius", key: "blast_radius"},
		{role: "reversibility", key: "reversibility"},
	}
	claims := make(
		[]taskMemoryClaimInput,
		0,
		len(scalars),
	)
	for _, scalar := range scalars {
		value, _ := args[scalar.key].(string)
		claims = append(claims, taskMemoryClaimInput{
			role: scalar.role,
			text: value,
		})
	}
	lists := []struct {
		role string
		key  string
	}{
		{role: "constraint", key: "constraints"},
		{role: "optimization_target", key: "optimization_targets"},
		{role: "observation_indicator", key: "observation_indicators"},
	}
	for _, list := range lists {
		for _, value := range parseStringArrayFromArgs(
			args,
			list.key,
		) {
			claims = append(claims, taskMemoryClaimInput{
				role: list.role,
				text: value,
			})
		}
	}
	return claims
}

func solutionPortfolioTaskMemoryClaims(
	record *artifact.Artifact,
) []taskMemoryClaimInput {
	fields := record.UnmarshalPortfolioFields()
	claims := make(
		[]taskMemoryClaimInput,
		0,
		len(fields.Variants)*8,
	)
	if strings.TrimSpace(fields.ProblemRef) != "" {
		claims = append(claims, taskMemoryClaimInput{
			role: "problem_reference",
			text: fields.ProblemRef,
		})
	}
	if strings.TrimSpace(fields.NoSteppingStoneRationale) != "" {
		claims = append(claims, taskMemoryClaimInput{
			role: "no_stepping_stone_rationale",
			text: fields.NoSteppingStoneRationale,
		})
	}
	for _, variant := range fields.Variants {
		prefix := strings.TrimSpace(variant.ID) + ": "
		scalars := []struct {
			role string
			text string
		}{
			{role: "option_title", text: prefix + variant.Title},
			{role: "option_description", text: prefix + variant.Description},
			{role: "option_weakest_link", text: prefix + variant.WeakestLink},
			{role: "option_novelty", text: prefix + variant.NoveltyMarker},
			{role: "option_diversity_role", text: prefix + variant.DiversityRole},
			{role: "option_assumption", text: prefix + variant.AssumptionNotes},
			{role: "option_rollback", text: prefix + variant.RollbackNotes},
			{
				role: "option_stepping_stone_basis",
				text: prefix + variant.SteppingStoneBasis,
			},
		}
		for _, scalar := range scalars {
			claims = append(claims, taskMemoryClaimInput{
				role: scalar.role,
				text: scalar.text,
			})
		}
		lists := []struct {
			role   string
			values []string
		}{
			{role: "option_strength", values: variant.Strengths},
			{role: "option_risk", values: variant.Risks},
			{role: "option_evidence_reference", values: variant.EvidenceRefs},
		}
		for _, list := range lists {
			for _, value := range list.values {
				claims = append(claims, taskMemoryClaimInput{
					role: list.role,
					text: prefix + value,
				})
			}
		}
	}
	return claims
}

func portfolioComparisonTaskMemoryClaims(
	record *artifact.Artifact,
) []taskMemoryClaimInput {
	fields := record.UnmarshalPortfolioFields()
	if fields.Comparison == nil {
		return nil
	}
	comparison := fields.Comparison
	claims := make(
		[]taskMemoryClaimInput,
		0,
		len(comparison.Dimensions)+
			len(comparison.Scores)*2+
			len(comparison.NonDominatedSet),
	)
	for _, dimension := range comparison.Dimensions {
		claims = append(claims, taskMemoryClaimInput{
			role: "comparison_dimension",
			text: dimension,
		})
	}
	for _, variantID := range taskMemorySortedMapKeys(
		comparison.Scores,
	) {
		dimensionScores := comparison.Scores[variantID]
		dimensions := make(
			[]string,
			0,
			len(dimensionScores),
		)
		for dimension := range dimensionScores {
			dimensions = append(
				dimensions,
				strings.TrimSpace(dimension),
			)
		}
		sort.Strings(dimensions)
		for _, dimension := range dimensions {
			claims = append(claims, taskMemoryClaimInput{
				role: "comparison_score",
				text: strings.TrimSpace(variantID) +
					":" +
					dimension +
					"=" +
					strings.TrimSpace(
						dimensionScores[dimension],
					),
			})
		}
	}
	for _, variantID := range comparison.NonDominatedSet {
		claims = append(claims, taskMemoryClaimInput{
			role: "non_dominated_option",
			text: variantID,
		})
	}
	for _, pair := range comparison.Incomparable {
		normalizedPair := append([]string(nil), pair...)
		sort.Strings(normalizedPair)
		claims = append(claims, taskMemoryClaimInput{
			role: "incomparable_pair",
			text: strings.Join(normalizedPair, " | "),
		})
	}
	for _, explanation := range comparison.DominatedVariants {
		dominators := append(
			[]string(nil),
			explanation.DominatedBy...,
		)
		sort.Strings(dominators)
		claims = append(claims, taskMemoryClaimInput{
			role: "dominated_option",
			text: strings.TrimSpace(explanation.Variant) +
				" <- [" +
				strings.Join(dominators, ", ") +
				"]: " +
				strings.TrimSpace(explanation.Summary),
		})
	}
	for _, tradeoff := range comparison.ParetoTradeoffs {
		claims = append(claims, taskMemoryClaimInput{
			role: "pareto_tradeoff",
			text: strings.TrimSpace(tradeoff.Variant) +
				": " +
				strings.TrimSpace(tradeoff.Summary),
		})
	}
	if strings.TrimSpace(comparison.PolicyApplied) != "" {
		claims = append(claims, taskMemoryClaimInput{
			role: "comparison_policy",
			text: comparison.PolicyApplied,
		})
	}
	claims = append(
		claims,
		portfolioComparisonParityClaims(comparison.ParityPlan)...,
	)
	return claims
}

func portfolioComparisonParityClaims(
	plan *artifact.ParityPlan,
) []taskMemoryClaimInput {
	if plan == nil {
		return nil
	}
	claims := make(
		[]taskMemoryClaimInput,
		0,
		len(plan.BaselineSet)+
			len(plan.Normalization)+
			len(plan.PinnedConditions)+
			4,
	)
	for _, baseline := range plan.BaselineSet {
		claims = append(claims, taskMemoryClaimInput{
			role: "parity_baseline",
			text: baseline,
		})
	}
	for _, rule := range plan.Normalization {
		dimension := strings.TrimSpace(rule.Dimension)
		method := strings.TrimSpace(rule.Method)
		if dimension == "" && method == "" {
			continue
		}
		text := fmt.Sprintf(
			"dimension=%q method=%q",
			dimension,
			method,
		)
		claims = append(claims, taskMemoryClaimInput{
			role: "parity_normalization",
			text: text,
		})
	}
	for _, condition := range plan.PinnedConditions {
		claims = append(claims, taskMemoryClaimInput{
			role: "parity_pinned_condition",
			text: condition,
		})
	}
	scalars := []taskMemoryClaimInput{
		{role: "parity_window", text: plan.Window},
		{role: "parity_budget", text: plan.Budget},
		{
			role: "parity_missing_data_policy",
			text: plan.MissingDataPolicy,
		},
	}
	return append(claims, scalars...)
}

func specSectionTaskMemoryClaims(
	edition specflow.SpecSectionEdition,
) []taskMemoryClaimInput {
	section := edition.Section
	scalars := []struct {
		role string
		text string
	}{
		{role: "section_id", text: section.ID},
		{role: "section_title", text: section.Title},
		{role: "section_kind", text: section.Kind},
		{role: "section_statement_type", text: section.StatementType},
		{role: "section_claim_layer", text: section.ClaimLayer},
		{role: "section_owner", text: section.Owner},
		{role: "section_status", text: section.Status},
		{role: "section_valid_until", text: section.ValidUntil},
		{role: "section_document_kind", text: section.DocumentKind},
		{role: "system_frame_id", text: section.SystemFrame.ID},
		{role: "system_frame_kind", text: section.SystemFrame.Kind},
		{role: "system_frame_source", text: section.SystemFrame.Source},
	}
	claims := make(
		[]taskMemoryClaimInput,
		0,
		len(scalars)+len(section.Claims)*5,
	)
	for _, scalar := range scalars {
		claims = append(claims, taskMemoryClaimInput{
			role: scalar.role,
			text: scalar.text,
		})
	}
	lists := []struct {
		role   string
		values []string
	}{
		{role: "section_term", values: section.Terms},
		{role: "section_dependency", values: section.DependsOn},
		{role: "section_target_reference", values: section.TargetRefs},
	}
	for _, list := range lists {
		for _, value := range list.values {
			claims = append(claims, taskMemoryClaimInput{
				role: list.role,
				text: value,
			})
		}
	}
	for _, requirement := range section.EvidenceRequired {
		claims = append(claims, taskMemoryClaimInput{
			role: "section_evidence_requirement",
			text: strings.TrimSpace(requirement.Kind) +
				": " +
				strings.TrimSpace(requirement.Description),
		})
	}
	for _, claim := range section.Claims {
		prefix := strings.TrimSpace(claim.ID) + ": "
		claimScalars := []struct {
			role string
			text string
		}{
			{role: "spec_claim", text: prefix + claim.Statement},
			{role: "spec_claim_class", text: prefix + claim.Class},
			{role: "spec_claim_valid_until", text: prefix + claim.ValidUntil},
		}
		for _, scalar := range claimScalars {
			claims = append(claims, taskMemoryClaimInput{
				role: scalar.role,
				text: scalar.text,
			})
		}
		claimLists := []struct {
			role   string
			values []string
		}{
			{role: "spec_claim_scope", values: claim.Scope},
			{role: "spec_claim_support_reference", values: claim.SupportRefs},
			{role: "spec_claim_evidence_reference", values: claim.EvidenceRefs},
			{
				role:   "spec_claim_governing_pattern_reference",
				values: claim.GoverningPatternRefs,
			},
		}
		for _, list := range claimLists {
			for _, value := range list.values {
				claims = append(claims, taskMemoryClaimInput{
					role: list.role,
					text: prefix + value,
				})
			}
		}
	}
	return claims
}

func normalizeTaskMemoryClaims(
	claims []taskMemoryClaimInput,
) []taskMemoryClaimInput {
	unique := make(map[string]taskMemoryClaimInput)
	for _, claim := range claims {
		role := strings.TrimSpace(claim.role)
		text := strings.TrimSpace(claim.text)
		if role == "" || text == "" {
			continue
		}
		key := role + "\x00" + text
		unique[key] = taskMemoryClaimInput{
			role: role,
			text: text,
		}
	}
	keys := make([]string, 0, len(unique))
	for key := range unique {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	normalized := make(
		[]taskMemoryClaimInput,
		0,
		len(keys),
	)
	for _, key := range keys {
		normalized = append(normalized, unique[key])
	}
	return normalized
}

func taskMemoryClaimNodeID(
	artifactRef string,
	claim taskMemoryClaimInput,
) string {
	sum := sha256.Sum256(
		[]byte(claim.role + "\x00" + claim.text),
	)
	return "claim:" +
		artifactRef +
		":" +
		hex.EncodeToString(sum[:])
}

func resolveTaskMemoryConcern(
	current typedmemorystore.CurrentProjectSnapshot,
	input taskMemoryConcernInput,
) (
	recordatconcern.ExactConcernBinding,
	taskMemoryConcernProjection,
	*taskMemoryProjectionReport,
) {
	contextRef, err := typedmemory.NewBoundedContextRef(
		input.context,
	)
	if err != nil {
		report := invalidTaskMemoryProjectionReport(
			taskMemoryArtifactProjection{},
			[]taskMemoryViolationProjection{{
				Code:    "bounded_context_ref_invalid",
				Message: err.Error(),
			}},
		)
		return recordatconcern.ExactConcernBinding{}, taskMemoryConcernProjection{}, &report
	}
	refKindID, err := typedmemory.NewRefKindID(
		input.refKindID,
	)
	if err != nil {
		report := invalidTaskMemoryProjectionReport(
			taskMemoryArtifactProjection{},
			[]taskMemoryViolationProjection{{
				Code:    "entity_ref_kind_invalid",
				Message: err.Error(),
			}},
		)
		return recordatconcern.ExactConcernBinding{}, taskMemoryConcernProjection{}, &report
	}
	refKind, err := typedmemory.NewRefKindRef(
		current.Environment().Ref(),
		refKindID,
	)
	if err != nil {
		report := invalidTaskMemoryProjectionReport(
			taskMemoryArtifactProjection{},
			[]taskMemoryViolationProjection{{
				Code:    "entity_ref_kind_invalid",
				Message: err.Error(),
			}},
		)
		return recordatconcern.ExactConcernBinding{}, taskMemoryConcernProjection{}, &report
	}
	referenceID, err := typedmemory.NewReferenceID(
		input.referenceID,
	)
	if err != nil {
		report := invalidTaskMemoryProjectionReport(
			taskMemoryArtifactProjection{},
			[]taskMemoryViolationProjection{{
				Code:    "entity_reference_id_invalid",
				Message: err.Error(),
			}},
		)
		return recordatconcern.ExactConcernBinding{}, taskMemoryConcernProjection{}, &report
	}
	reference, err := typedmemory.NewPersistedRef(
		refKind,
		referenceID,
	)
	if err != nil {
		report := invalidTaskMemoryProjectionReport(
			taskMemoryArtifactProjection{},
			[]taskMemoryViolationProjection{{
				Code:    "entity_reference_invalid",
				Message: err.Error(),
			}},
		)
		return recordatconcern.ExactConcernBinding{}, taskMemoryConcernProjection{}, &report
	}
	resolution := current.Snapshot().ResolveReference(
		reference,
		contextRef,
	)
	resolved, present := resolution.(typedmemory.ResolvedStrongReference)
	if !present {
		report := underdeterminedTaskMemoryProjectionReport(
			taskMemoryArtifactProjection{},
			[]taskMemoryMissingBasisProjection{{
				Name:   "entity_of_concern_resolution",
				Repair: taskMemoryResolutionRepair(resolution),
			}},
		)
		return recordatconcern.ExactConcernBinding{}, taskMemoryConcernProjection{}, &report
	}
	binding, err := recordatconcern.NewExactConcernBinding(
		resolved,
	)
	if err != nil {
		report := invalidTaskMemoryProjectionReport(
			taskMemoryArtifactProjection{},
			[]taskMemoryViolationProjection{{
				Code:    "entity_of_concern_binding_invalid",
				Message: err.Error(),
			}},
		)
		return recordatconcern.ExactConcernBinding{}, taskMemoryConcernProjection{}, &report
	}
	projection := taskMemoryConcernProjection{
		RefKindID:       input.refKindID,
		ReferenceID:     input.referenceID,
		EntityID:        resolved.Entity().String(),
		BoundedContext:  resolved.Context().String(),
		ResolutionBasis: resolved.Basis().String(),
	}
	return binding, projection, nil
}

func taskMemoryResolutionRepair(
	resolution typedmemory.StrongReferenceResolution,
) string {
	switch exact := resolution.(type) {
	case typedmemory.UnresolvedStrongReference:
		return exact.Repair().String()
	case typedmemory.MissingContextBridgeResolution:
		return exact.Repair().String()
	default:
		return "repair:resolve-entity-of-concern"
	}
}

func parseTaskMemoryConcernInput(
	args map[string]any,
) (taskMemoryConcernInput, []taskMemoryMissingBasisProjection) {
	contextRef, _ := args["bounded_context_ref"].(string)
	contextRef = strings.TrimSpace(contextRef)
	refKindID, referenceID := parseTaskMemoryEntityReference(
		args["entity_ref"],
	)
	missing := make(
		[]taskMemoryMissingBasisProjection,
		0,
		2,
	)
	if contextRef == "" {
		missing = append(missing, taskMemoryMissingBasisProjection{
			Name:   "bounded_context_ref",
			Repair: "repair:provide-exact-bounded-context-ref",
		})
	}
	if refKindID == "" || referenceID == "" {
		missing = append(missing, taskMemoryMissingBasisProjection{
			Name:   "entity_of_concern_resolution",
			Repair: "repair:resolve-entity-of-concern",
		})
	}
	return taskMemoryConcernInput{
		refKindID:   refKindID,
		referenceID: referenceID,
		context:     contextRef,
	}, missing
}

func parseTaskMemoryEntityReference(
	raw any,
) (string, string) {
	switch value := raw.(type) {
	case map[string]any:
		refKindID, _ := value["ref_kind_id"].(string)
		referenceID, _ := value["reference_id"].(string)
		return strings.TrimSpace(refKindID), strings.TrimSpace(referenceID)
	case map[string]string:
		return strings.TrimSpace(value["ref_kind_id"]),
			strings.TrimSpace(value["reference_id"])
	default:
		return "", ""
	}
}

func taskMemoryProjectionApplicable(
	request taskMemoryProjectionRequest,
) bool {
	switch request.ToolName {
	case "haft_note":
		return strings.TrimSpace(request.ArtifactRef) != ""
	case "haft_problem":
		return request.Action == "frame" &&
			strings.TrimSpace(request.ArtifactRef) != ""
	case "haft_solution":
		return (request.Action == "explore" ||
			request.Action == "compare") &&
			strings.TrimSpace(request.ArtifactRef) != ""
	case "haft_spec_section":
		return request.Action == "project" &&
			strings.TrimSpace(request.ArtifactRef) != ""
	case "haft_decision":
		return request.Action == "project_existing" &&
			strings.TrimSpace(request.ArtifactRef) != ""
	default:
		return false
	}
}

func projectTaskMemoryArtifact(
	record *artifact.Artifact,
) taskMemoryArtifactProjection {
	if record == nil {
		return taskMemoryArtifactProjection{}
	}
	return taskMemoryArtifactProjection{
		Ref:     record.Meta.ID,
		Kind:    string(record.Meta.Kind),
		Version: record.Meta.Version,
		Title:   record.Meta.Title,
	}
}

func projectTaskMemoryPortfolioComparisonEdition(
	record *artifact.Artifact,
) taskMemoryArtifactProjection {
	if record == nil {
		return taskMemoryArtifactProjection{}
	}
	title := strings.TrimSpace(record.Meta.Title)
	if title == "" {
		title = record.Meta.ID
	}
	return taskMemoryArtifactProjection{
		Ref:     record.Meta.ID,
		Kind:    "PortfolioComparisonEdition",
		Version: record.Meta.Version,
		Title:   "Comparison: " + title,
	}
}

func projectTaskMemorySpecSectionEdition(
	edition specflow.SpecSectionEdition,
	editionRef string,
) taskMemoryArtifactProjection {
	title := strings.TrimSpace(edition.Section.Title)
	if title == "" {
		title = edition.SectionID
	}
	return taskMemoryArtifactProjection{
		Ref:     editionRef,
		Kind:    "SpecSectionEdition",
		Version: 1,
		Title:   title,
	}
}

func projectTaskMemoryMissingBasis(
	missing []recordatconcern.MissingBasis,
) []taskMemoryMissingBasisProjection {
	result := make(
		[]taskMemoryMissingBasisProjection,
		0,
		len(missing),
	)
	for _, basis := range missing {
		result = append(result, taskMemoryMissingBasisProjection{
			Name:   basis.Name(),
			Repair: basis.Repair().String(),
		})
	}
	return result
}

func projectTaskMemoryViolations(
	violations []recordatconcern.Violation,
) []taskMemoryViolationProjection {
	result := make(
		[]taskMemoryViolationProjection,
		0,
		len(violations),
	)
	for _, violation := range violations {
		result = append(result, taskMemoryViolationProjection{
			Code:    violation.Code(),
			Message: violation.Message(),
		})
	}
	return result
}

func underdeterminedTaskMemoryProjectionReport(
	artifactProjection taskMemoryArtifactProjection,
	missing []taskMemoryMissingBasisProjection,
) taskMemoryProjectionReport {
	return taskMemoryProjectionReport{
		ContractVersion:          taskMemoryProjectionContractVersion,
		Artifact:                 artifactProjection,
		AdapterResult:            "underdetermined",
		AdmissionResult:          "not_attempted",
		AuthorityClass:           typedmemorywire.AuthorityClassNonBindingSemanticAssertion,
		LegacyCarrierDisposition: "retained_unsettled",
		MissingBasis:             append([]taskMemoryMissingBasisProjection(nil), missing...),
		Persistence: taskMemoryPersistenceProjection{
			Mode:             "not_attempted_no_write",
			AuthorityGranted: false,
		},
		Interpretation: taskMemoryInterpretation(
			[]string{
				"the legacy project carrier remains durable",
			},
			[]string{
				"no typed project-memory relation was created because exact basis is missing",
			},
		),
	}
}

func invalidTaskMemoryProjectionReport(
	artifactProjection taskMemoryArtifactProjection,
	violations []taskMemoryViolationProjection,
) taskMemoryProjectionReport {
	return taskMemoryProjectionReport{
		ContractVersion:          taskMemoryProjectionContractVersion,
		Artifact:                 artifactProjection,
		AdapterResult:            "invalid",
		AdmissionResult:          "not_attempted",
		AuthorityClass:           typedmemorywire.AuthorityClassNonBindingSemanticAssertion,
		LegacyCarrierDisposition: "retained_unsettled",
		Violations:               append([]taskMemoryViolationProjection(nil), violations...),
		Persistence: taskMemoryPersistenceProjection{
			Mode:             "not_attempted_no_write",
			AuthorityGranted: false,
		},
		Interpretation: taskMemoryInterpretation(
			[]string{
				"the legacy project carrier remains durable",
			},
			[]string{
				"no typed project-memory relation was created because the task projection is invalid",
			},
		),
	}
}

func notAdmittedTaskMemoryProjectionReport(
	artifactProjection taskMemoryArtifactProjection,
	concern taskMemoryConcernProjection,
	candidate recordatconcern.ValidCandidate,
	outcome typedmemoryvalidation.Outcome,
) taskMemoryProjectionReport {
	validation := typedmemoryvalidation.PresentOutcome(outcome)
	encoded, _ := json.Marshal(validation)
	return taskMemoryProjectionReport{
		ContractVersion:               taskMemoryProjectionContractVersion,
		Artifact:                      artifactProjection,
		AdapterResult:                 "valid",
		AdmissionResult:               "not_admitted",
		AuthorityClass:                typedmemorywire.AuthorityClassNonBindingSemanticAssertion,
		LegacyCarrierDisposition:      "retained_unsettled",
		CandidateChangeCount:          uint64(len(candidate.ChangeSet().Changes())),
		RelationDeclarationFragmentID: candidate.RelationDeclarationFragmentID().String(),
		RelationDeclarationPosture:    typedmemory.RelationDeclarationTypedFragment.String(),
		RelationSignatureID:           candidate.RelationDeclarationFragmentID().String(),
		EntityOfConcern:               &concern,
		Validation:                    encoded,
		Persistence: taskMemoryPersistenceProjection{
			Mode:             "not_admitted_no_write",
			AuthorityGranted: false,
		},
		Interpretation: taskMemoryInterpretation(
			[]string{
				"the task adapter produced one exact typed candidate",
				"the selected project basis did not admit that candidate and no typed change was committed",
			},
			nil,
		),
	}
}

func validatedOnlyTaskMemoryProjectionReport(
	artifactProjection taskMemoryArtifactProjection,
	concern taskMemoryConcernProjection,
	candidate recordatconcern.ValidCandidate,
	outcome typedmemoryvalidation.ValidOutcome,
) taskMemoryProjectionReport {
	validation := typedmemoryvalidation.PresentOutcome(outcome)
	encoded, _ := json.Marshal(validation)
	return taskMemoryProjectionReport{
		ContractVersion:               taskMemoryProjectionContractVersion,
		Artifact:                      artifactProjection,
		AdapterResult:                 "valid",
		AdmissionResult:               "validated_only",
		AuthorityClass:                typedmemorywire.AuthorityClassNonBindingSemanticAssertion,
		LegacyCarrierDisposition:      "retained_unsettled",
		CandidateChangeCount:          uint64(len(candidate.ChangeSet().Changes())),
		RelationDeclarationFragmentID: candidate.RelationDeclarationFragmentID().String(),
		RelationDeclarationPosture:    typedmemory.RelationDeclarationTypedFragment.String(),
		RelationSignatureID:           candidate.RelationDeclarationFragmentID().String(),
		EntityOfConcern:               &concern,
		Validation:                    encoded,
		Persistence: taskMemoryPersistenceProjection{
			Mode:             "validation_only_no_write",
			AuthorityGranted: false,
		},
		Interpretation: taskMemoryInterpretation(
			[]string{
				"the source-owned task adapter produced one exact typed candidate",
				"the candidate is valid under the exact selected project basis",
			},
			[]string{
				"dry-run committed no typed project-memory relation",
				"the candidate has no durable project-record reference until apply commits it",
			},
		),
	}
}

func alreadyProjectedTaskMemoryProjectionReport(
	artifactProjection taskMemoryArtifactProjection,
	concern taskMemoryConcernProjection,
	candidate recordatconcern.ValidCandidate,
) taskMemoryProjectionReport {
	return taskMemoryProjectionReport{
		ContractVersion:               taskMemoryProjectionContractVersion,
		Artifact:                      artifactProjection,
		AdapterResult:                 "valid",
		AdmissionResult:               "already_projected",
		AuthorityClass:                typedmemorywire.AuthorityClassNonBindingSemanticAssertion,
		LegacyCarrierDisposition:      "retained_with_typed_projection",
		CandidateChangeCount:          uint64(len(candidate.ChangeSet().Changes())),
		DurableChangeCount:            0,
		RelationDeclarationFragmentID: candidate.RelationDeclarationFragmentID().String(),
		RelationDeclarationPosture:    typedmemory.RelationDeclarationTypedFragment.String(),
		RelationSignatureID:           candidate.RelationDeclarationFragmentID().String(),
		RecordReference:               projectTaskMemoryRecordReference(candidate),
		EntityOfConcern:               &concern,
		Persistence: taskMemoryPersistenceProjection{
			Mode:             "existing_exact_projection_no_write",
			Disposition:      "already_projected",
			AuthorityGranted: false,
		},
		Interpretation: taskMemoryInterpretation(
			[]string{
				"the exact artifact-edition assertion is already active in typed project memory",
				"the legacy carrier and typed projection remain distinct correlated records",
			},
			[]string{
				"this projection request wrote no semantic rows",
				"an active projection does not reperform or broaden the source artifact's meaning",
			},
		),
	}
}

func committedTaskMemoryProjectionReport(
	artifactProjection taskMemoryArtifactProjection,
	concern taskMemoryConcernProjection,
	candidate recordatconcern.ValidCandidate,
	receipt typedmemorystore.CommitReceipt,
) taskMemoryProjectionReport {
	changeCount := uint64(len(candidate.ChangeSet().Changes()))
	establishes, omissions :=
		committedTaskMemoryInterpretationInputs(candidate)
	return taskMemoryProjectionReport{
		ContractVersion:               taskMemoryProjectionContractVersion,
		Artifact:                      artifactProjection,
		AdapterResult:                 "valid",
		AdmissionResult:               "committed",
		AuthorityClass:                typedmemorywire.AuthorityClassNonBindingSemanticAssertion,
		LegacyCarrierDisposition:      "retained_with_typed_projection",
		CandidateChangeCount:          changeCount,
		DurableChangeCount:            changeCount,
		RelationDeclarationFragmentID: candidate.RelationDeclarationFragmentID().String(),
		RelationDeclarationPosture:    typedmemory.RelationDeclarationTypedFragment.String(),
		RelationSignatureID:           candidate.RelationDeclarationFragmentID().String(),
		RecordReference:               projectTaskMemoryRecordReference(candidate),
		EntityOfConcern:               &concern,
		Persistence: taskMemoryPersistenceProjection{
			Mode:             "transactional_project_memory_commit",
			Disposition:      string(receipt.Disposition()),
			AuthorityGranted: false,
		},
		Receipt: &taskMemoryAdmissionReceiptProjection{
			EventRef:      receipt.EventRef(),
			CommitRef:     receipt.CommitRef(),
			GraphRevision: receipt.GraphRevision().Value(),
			ResultDigest:  receipt.ResultDigest().String(),
		},
		Interpretation: taskMemoryInterpretation(
			establishes,
			omissions,
		),
	}
}

func committedTaskMemoryInterpretationInputs(
	candidate recordatconcern.ValidCandidate,
) ([]string, []string) {
	establishes := []string{
		"the exact non-binding record-at-concern candidate is durable at the receipt coordinates",
		"the legacy carrier and typed projection remain distinct correlated records",
	}
	fragmentID := candidate.RelationDeclarationFragmentID().String()
	if fragmentID == "Haft.DecisionChoiceAtConcern" {
		establishes = append(
			establishes,
			"the already-existing DecisionRecord ChoiceResult maps to exact chosen and rejected project records at the EntityOfConcern",
		)
		omissions := []string{
			"the typed projection did not perform, repeat, approve, or supersede the human binding choice",
			"no comparison record is inferred from recency, graph proximity, or a legacy recommendation",
		}
		return establishes, omissions
	}
	if fragmentID !=
		"Haft.PortfolioComparison" {
		return establishes, nil
	}
	establishes = append(
		establishes,
		"the compared option set and its non-dominated subset remain exactly addressable",
	)
	omissions := []string{
		"the non-dominated subset is not a winner, recommendation, ChoiceResult, or DecisionRecord",
		"legacy advisory selected_ref and recommendation fields are not projected into the typed comparison relation",
	}
	return establishes, omissions
}

func projectTaskMemoryRecordReference(
	candidate recordatconcern.ValidCandidate,
) *taskMemoryRecordReferenceProjection {
	refKindID := "Haft.ProjectRecordRef"
	variant := candidate.CarrierBinding().RecordVariant().Token()
	if variant ==
		(recordcarrier.SpecSectionRecordVariantV1{}).Token() {
		refKindID = "Haft.SpecSectionRecordRef"
	}
	if variant ==
		(recordcarrier.DecisionRecordVariantV1{}).Token() {
		refKindID = "Haft.DecisionRecordRef"
	}
	entityID := candidate.CarrierBinding().EntityID().String()
	return &taskMemoryRecordReferenceProjection{
		RefKindID:   refKindID,
		ReferenceID: entityID,
		EntityID:    entityID,
	}
}

func unknownTaskMemoryProjectionReport(
	projectID projectidentity.ProjectID,
	artifactProjection taskMemoryArtifactProjection,
	concern taskMemoryConcernProjection,
	candidate recordatconcern.ValidCandidate,
	key typedmemorystore.IdempotencyKey,
	digest typedmemory.SHA256Digest,
	cause error,
) taskMemoryProjectionReport {
	return taskMemoryProjectionReport{
		ContractVersion:               taskMemoryProjectionContractVersion,
		Artifact:                      artifactProjection,
		AdapterResult:                 "valid",
		AdmissionResult:               "commit_outcome_unknown",
		AuthorityClass:                typedmemorywire.AuthorityClassNonBindingSemanticAssertion,
		LegacyCarrierDisposition:      "retained_typed_outcome_unknown",
		CandidateChangeCount:          uint64(len(candidate.ChangeSet().Changes())),
		RelationDeclarationFragmentID: candidate.RelationDeclarationFragmentID().String(),
		RelationDeclarationPosture:    typedmemory.RelationDeclarationTypedFragment.String(),
		RelationSignatureID:           candidate.RelationDeclarationFragmentID().String(),
		EntityOfConcern:               &concern,
		Persistence: taskMemoryPersistenceProjection{
			Mode:             "commit_outcome_unknown",
			AuthorityGranted: false,
		},
		Retry: &taskMemoryAdmissionRetryProjection{
			Kind:            "replay_exact_task_projection",
			ProjectID:       projectID.String(),
			ArtifactRef:     artifactProjection.Ref,
			ArtifactVersion: artifactProjection.Version,
			IdempotencyKey:  key.String(),
			CandidateDigest: digest.String(),
			Instruction:     "replay the unchanged artifact edition and exact EntityOfConcern coordinates",
		},
		OperationalDetail: cause.Error(),
		Interpretation: taskMemoryInterpretation(
			[]string{
				"the legacy project carrier remains durable",
			},
			[]string{
				"the typed-memory transaction may have committed, but neither commit nor rollback is established",
			},
		),
	}
}

func unavailableValidTaskMemoryProjectionReport(
	artifactProjection taskMemoryArtifactProjection,
	concern taskMemoryConcernProjection,
	candidate recordatconcern.ValidCandidate,
	detail string,
) taskMemoryProjectionReport {
	report := unavailableTaskMemoryProjectionReport(
		artifactProjection,
		detail,
	)
	report.AdapterResult = "valid"
	report.CandidateChangeCount = uint64(
		len(candidate.ChangeSet().Changes()),
	)
	report.RelationDeclarationFragmentID = candidate.RelationDeclarationFragmentID().String()
	report.RelationDeclarationPosture = typedmemory.RelationDeclarationTypedFragment.String()
	report.RelationSignatureID = candidate.RelationDeclarationFragmentID().String()
	report.EntityOfConcern = &concern
	return report
}

func unavailableTaskMemoryProjectionReport(
	artifactProjection taskMemoryArtifactProjection,
	detail string,
) taskMemoryProjectionReport {
	return taskMemoryProjectionReport{
		ContractVersion:          taskMemoryProjectionContractVersion,
		Artifact:                 artifactProjection,
		AdapterResult:            "not_evaluated",
		AdmissionResult:          "unavailable",
		AuthorityClass:           typedmemorywire.AuthorityClassNonBindingSemanticAssertion,
		LegacyCarrierDisposition: "retained_unsettled",
		Persistence: taskMemoryPersistenceProjection{
			Mode:             "projection_unavailable",
			AuthorityGranted: false,
		},
		OperationalDetail: detail,
		Interpretation: taskMemoryInterpretation(
			[]string{
				"the legacy project carrier remains durable",
			},
			[]string{
				"typed projection availability, validation, and persistence are not established",
			},
		),
	}
}

func taskMemoryInterpretation(
	establishes []string,
	stateSpecificOmissions []string,
) taskMemoryInterpretationProjection {
	commonOmissions := []string{
		"no truth, evidence-quality, applicability-outside-the-exact-context, completion, recommendation, causal-order, or work-order claim is established",
		"the carrier, its ClaimGraph, and its EntityOfConcern remain distinct",
	}
	omissions := append(
		[]string(nil),
		stateSpecificOmissions...,
	)
	omissions = append(
		omissions,
		commonOmissions...,
	)
	return taskMemoryInterpretationProjection{
		Establishes: append([]string(nil), establishes...),
		Omits:       omissions,
		DoesNotAuthorize: []string{
			"binding a DecisionRecord or WorkCommission",
			"approving, reopening, or rebaselining a SpecSection",
			"selecting or evolving a ProjectTypeEnvHead",
		},
	}
}

func taskMemoryIdempotencyKey(
	artifactProjection taskMemoryArtifactProjection,
	digest typedmemory.SHA256Digest,
) (typedmemorystore.IdempotencyKey, error) {
	return typedmemorystore.NewIdempotencyKey(
		fmt.Sprintf(
			"task-memory:%s:v%d:%s",
			artifactProjection.Ref,
			artifactProjection.Version,
			digest.String(),
		),
	)
}

func taskMemoryArtifactProvenance(
	artifactProjection taskMemoryArtifactProjection,
) (typedmemory.ProvenanceRef, error) {
	return typedmemory.NewProvenanceRef(
		fmt.Sprintf(
			"artifact:%s:v%d",
			artifactProjection.Ref,
			artifactProjection.Version,
		),
	)
}

func appendTaskMemoryProjection(
	rendered string,
	report taskMemoryProjectionReport,
) string {
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return rendered +
			"\n\n## Typed project-memory projection\n\n" +
			"Projection result could not be encoded: " +
			err.Error() +
			"\n"
	}
	return rendered +
		"\n\n## Typed project-memory projection\n\n```json\n" +
		string(encoded) +
		"\n```\n"
}

func applyTaskMemoryProjection(
	ctx context.Context,
	rendered string,
	request taskMemoryProjectionRequest,
	projector taskMemoryProjector,
) string {
	if projector == nil {
		return rendered
	}
	report, applicable := projector.Project(
		ctx,
		request,
	)
	if !applicable {
		return rendered
	}
	return appendTaskMemoryProjection(rendered, report)
}
