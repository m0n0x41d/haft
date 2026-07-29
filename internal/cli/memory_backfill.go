package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/projectmemory/existingrecordprojection"
	"github.com/spf13/cobra"
)

const (
	memoryBackfillContractVersion = "haft.memory.backfill.v1"
	memoryBackfillMaximumItems    = 512
	memoryBackfillMaximumBytes    = 1 << 20
)

type memoryBackfillMode string

const (
	memoryBackfillDryRun memoryBackfillMode = "dry_run"
	memoryBackfillApply  memoryBackfillMode = "apply"
)

type memoryBackfillRequest struct {
	ContractVersion      string               `json:"contract_version"`
	Mode                 memoryBackfillMode   `json:"mode"`
	RequestProvenanceRef string               `json:"request_provenance_ref"`
	Items                []memoryBackfillItem `json:"items"`
}

type memoryBackfillItem struct {
	ArtifactRef      string                   `json:"artifact_ref"`
	EntityRef        *memoryBackfillEntityRef `json:"entity_ref,omitempty"`
	BoundedContextID string                   `json:"bounded_context_ref,omitempty"`
}

type memoryBackfillEntityRef struct {
	RefKindID   string `json:"ref_kind_id"`
	ReferenceID string `json:"reference_id"`
}

type memoryBackfillReport struct {
	ContractVersion      string                          `json:"contract_version"`
	Mode                 memoryBackfillMode              `json:"mode"`
	ProjectID            string                          `json:"project_id"`
	RequestProvenanceRef string                          `json:"request_provenance_ref"`
	GraphRevisionBefore  uint64                          `json:"graph_revision_before"`
	GraphRevisionAfter   uint64                          `json:"graph_revision_after"`
	Routes               []memoryBackfillRouteResult     `json:"routes"`
	Deferred             []memoryBackfillDeferredResult  `json:"deferred"`
	Summary              memoryBackfillSummary           `json:"summary"`
	AuthorityBoundary    memoryBackfillAuthorityBoundary `json:"authority_boundary"`
}

type memoryBackfillRouteResult struct {
	ArtifactRef      string                      `json:"artifact_ref"`
	ArtifactKind     string                      `json:"artifact_kind"`
	Version          int                         `json:"version"`
	Projection       string                      `json:"projection"`
	Requirements     []string                    `json:"requirements"`
	Result           string                      `json:"result"`
	Detail           string                      `json:"detail,omitempty"`
	ProjectionReport *taskMemoryProjectionReport `json:"projection_report,omitempty"`
}

type memoryBackfillDeferredResult struct {
	ArtifactRef  string `json:"artifact_ref"`
	ArtifactKind string `json:"artifact_kind"`
	Version      int    `json:"version"`
	Reason       string `json:"reason"`
}

type memoryBackfillSummary struct {
	SelectedArtifacts int `json:"selected_artifacts"`
	PlannedRoutes     int `json:"planned_routes"`
	ValidatedOnly     int `json:"validated_only"`
	AlreadyProjected  int `json:"already_projected"`
	Committed         int `json:"committed"`
	Unresolved        int `json:"unresolved"`
	Invalid           int `json:"invalid"`
	Unavailable       int `json:"unavailable"`
	OutcomeUnknown    int `json:"outcome_unknown"`
	Deferred          int `json:"deferred"`
}

type memoryBackfillAuthorityBoundary struct {
	Mutation      string `json:"mutation"`
	Schema        string `json:"schema"`
	TypeEnvHead   string `json:"type_env_head"`
	Decision      string `json:"decision"`
	Specification string `json:"specification"`
	EvidenceTruth string `json:"evidence_truth"`
	PerformedWork string `json:"performed_work"`
	Publication   string `json:"publication"`
}

type memoryBackfillRouteExecution struct {
	toolName string
	action   string
}

var memoryBackfillExecutions = map[existingrecordprojection.Projection]memoryBackfillRouteExecution{
	existingrecordprojection.ProjectionNoteAtConcern: {
		toolName: "haft_note",
		action:   "record",
	},
	existingrecordprojection.ProjectionProblemCardAtConcern: {
		toolName: "haft_problem",
		action:   "frame",
	},
	existingrecordprojection.ProjectionSolutionPortfolioAtConcern: {
		toolName: "haft_solution",
		action:   "explore",
	},
	existingrecordprojection.ProjectionPortfolioComparisonAtConcern: {
		toolName: "haft_solution",
		action:   "compare",
	},
	existingrecordprojection.ProjectionDecisionChoiceAtConcern: {
		toolName: "haft_decision",
		action:   "project_existing",
	},
}

var memoryBackfillInputFile string

var memoryBackfillCmd = &cobra.Command{
	Use:   "backfill",
	Short: "Dry-run or apply source-owned typed projections for existing records",
	Long: `Project an explicit inventory of existing Haft records into typed project memory.

The strict input file selects dry_run or apply, names the request provenance,
and supplies exact EntityOfConcern coordinates per artifact where required.
Dry-run opens the project ledger read-only and writes zero rows. Apply uses the
same task adapters and AdmissionService as creation-time projection. Unsupported
carriers remain deferred; missing source meaning remains unresolved. The
operation never guesses identity, declares schema, changes the TypeEnv head, or
reinterprets evidence, Work, decisions, or specifications.`,
	Args: cobra.NoArgs,
	RunE: runMemoryBackfill,
}

func init() {
	memoryBackfillCmd.Flags().StringVar(
		&memoryBackfillInputFile,
		"input-file",
		"",
		"Strict haft.memory.backfill.v1 JSON file, or - for stdin",
	)
	memoryCmd.AddCommand(memoryBackfillCmd)
}

func runMemoryBackfill(
	cmd *cobra.Command,
	_ []string,
) (runErr error) {
	if memoryBackfillInputFile == "" {
		return fmt.Errorf(
			"--input-file is required; run `haft interface memory.backfill --json` for the exact contract",
		)
	}
	payload, err := readMemoryValidationInput(
		cmd,
		memoryBackfillInputFile,
	)
	if err != nil {
		return err
	}
	request, err := decodeMemoryBackfillRequest(payload)
	if err != nil {
		return err
	}
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a haft project: %w", err)
	}
	access := projectledger.ReadOnly
	if request.Mode == memoryBackfillApply {
		access = projectledger.ReadWrite
	}
	ledger, err := openCurrentProjectLedger(
		cmd.Context(),
		projectRoot,
		access,
		"typed-memory existing-record backfill",
	)
	if err != nil {
		return err
	}
	defer func() {
		runErr = errors.Join(runErr, ledger.Close())
	}()
	if err := ledger.Revalidate(cmd.Context()); err != nil {
		return fmt.Errorf(
			"revalidate checked project ledger before typed-memory backfill: %w",
			err,
		)
	}
	store := artifact.NewStore(ledger.Database())
	projector, err := newTaskMemoryProjectionRuntime(
		cmd.Context(),
		ledger.ProjectID(),
		ledger.Database(),
		store,
	)
	if err != nil {
		return fmt.Errorf(
			"construct existing-record typed-memory projector: %w",
			err,
		)
	}
	report, operationErr := executeMemoryBackfill(
		cmd.Context(),
		ledger.ProjectID(),
		store,
		projector,
		request,
	)
	revalidationErr := ledger.Revalidate(cmd.Context())
	if revalidationErr != nil {
		revalidationErr = fmt.Errorf(
			"revalidate checked project ledger after typed-memory backfill: %w",
			revalidationErr,
		)
	}
	if operationErr != nil || revalidationErr != nil {
		return errors.Join(operationErr, revalidationErr)
	}
	return writeJSON(cmd.OutOrStdout(), report)
}

func decodeMemoryBackfillRequest(
	payload []byte,
) (memoryBackfillRequest, error) {
	if len(payload) > memoryBackfillMaximumBytes {
		return memoryBackfillRequest{}, fmt.Errorf(
			"typed-memory backfill request exceeds %d bytes",
			memoryBackfillMaximumBytes,
		)
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	request := memoryBackfillRequest{}
	if err := decoder.Decode(&request); err != nil {
		return memoryBackfillRequest{}, fmt.Errorf(
			"decode typed-memory backfill request: %w",
			err,
		)
	}
	if err := requireMemoryBackfillEOF(decoder); err != nil {
		return memoryBackfillRequest{}, err
	}
	if err := request.verify(); err != nil {
		return memoryBackfillRequest{}, err
	}
	return request, nil
}

func requireMemoryBackfillEOF(
	decoder *json.Decoder,
) error {
	var trailing any
	err := decoder.Decode(&trailing)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil {
		return fmt.Errorf(
			"decode typed-memory backfill trailing data: %w",
			err,
		)
	}
	return fmt.Errorf(
		"decode typed-memory backfill request: multiple JSON values are not allowed",
	)
}

func (request memoryBackfillRequest) verify() error {
	if request.ContractVersion != memoryBackfillContractVersion {
		return fmt.Errorf(
			"contract_version must be %q",
			memoryBackfillContractVersion,
		)
	}
	if request.Mode != memoryBackfillDryRun &&
		request.Mode != memoryBackfillApply {
		return fmt.Errorf(
			"mode must be %q or %q",
			memoryBackfillDryRun,
			memoryBackfillApply,
		)
	}
	if !exactMemoryBackfillString(request.RequestProvenanceRef) {
		return fmt.Errorf(
			"request_provenance_ref must be one exact non-empty reference",
		)
	}
	if len(request.Items) == 0 ||
		len(request.Items) > memoryBackfillMaximumItems {
		return fmt.Errorf(
			"items must contain 1..%d exact artifact selections",
			memoryBackfillMaximumItems,
		)
	}
	seen := make(map[string]struct{}, len(request.Items))
	for index, item := range request.Items {
		if err := item.verify(); err != nil {
			return fmt.Errorf("items[%d]: %w", index, err)
		}
		if _, duplicate := seen[item.ArtifactRef]; duplicate {
			return fmt.Errorf(
				"items[%d]: duplicate artifact_ref %q",
				index,
				item.ArtifactRef,
			)
		}
		seen[item.ArtifactRef] = struct{}{}
	}
	return nil
}

func (item memoryBackfillItem) verify() error {
	if !exactMemoryBackfillString(item.ArtifactRef) {
		return fmt.Errorf(
			"artifact_ref must be one exact non-empty artifact identity",
		)
	}
	if item.EntityRef == nil {
		if strings.TrimSpace(item.BoundedContextID) != "" {
			return fmt.Errorf(
				"bounded_context_ref requires entity_ref",
			)
		}
		return nil
	}
	if !exactMemoryBackfillString(item.EntityRef.RefKindID) ||
		!exactMemoryBackfillString(item.EntityRef.ReferenceID) ||
		!exactMemoryBackfillString(item.BoundedContextID) {
		return fmt.Errorf(
			"entity_ref and bounded_context_ref must be exact non-empty coordinates",
		)
	}
	return nil
}

func exactMemoryBackfillString(value string) bool {
	return value != "" &&
		value == strings.TrimSpace(value)
}

func executeMemoryBackfill(
	ctx context.Context,
	projectID projectidentity.ProjectID,
	store *artifact.Store,
	projector *taskMemoryProjectionRuntime,
	request memoryBackfillRequest,
) (memoryBackfillReport, error) {
	if store == nil || projector == nil {
		return memoryBackfillReport{}, fmt.Errorf(
			"execute typed-memory backfill: store and projector are required",
		)
	}
	before, err := memoryBackfillGraphRevision(ctx, projector)
	if err != nil {
		return memoryBackfillReport{}, err
	}
	records, itemsByRef, err := loadMemoryBackfillInventory(
		ctx,
		store,
		request.Items,
	)
	if err != nil {
		return memoryBackfillReport{}, err
	}
	plan, err := existingrecordprojection.Build(records)
	if err != nil {
		return memoryBackfillReport{}, err
	}
	recordsByRef := indexMemoryBackfillRecords(records)
	routes := executeMemoryBackfillRoutes(
		ctx,
		projector,
		plan.Routes(),
		recordsByRef,
		itemsByRef,
		request.Mode,
	)
	deferred := projectMemoryBackfillDeferred(plan.Deferred())
	after, err := memoryBackfillGraphRevision(ctx, projector)
	if err != nil {
		return memoryBackfillReport{}, err
	}
	if request.Mode == memoryBackfillDryRun &&
		before != after {
		return memoryBackfillReport{}, fmt.Errorf(
			"typed-memory dry-run changed graph revision from %d to %d",
			before,
			after,
		)
	}
	report := memoryBackfillReport{
		ContractVersion:      memoryBackfillContractVersion,
		Mode:                 request.Mode,
		ProjectID:            projectID.String(),
		RequestProvenanceRef: request.RequestProvenanceRef,
		GraphRevisionBefore:  before,
		GraphRevisionAfter:   after,
		Routes:               routes,
		Deferred:             deferred,
		AuthorityBoundary:    memoryBackfillBoundary(request.Mode),
	}
	report.Summary = summarizeMemoryBackfill(
		len(records),
		routes,
		deferred,
	)
	return report, nil
}

func loadMemoryBackfillInventory(
	ctx context.Context,
	store *artifact.Store,
	items []memoryBackfillItem,
) (
	[]*artifact.Artifact,
	map[string]memoryBackfillItem,
	error,
) {
	records := make([]*artifact.Artifact, 0, len(items))
	itemsByRef := make(
		map[string]memoryBackfillItem,
		len(items),
	)
	for _, item := range items {
		record, err := store.Get(ctx, item.ArtifactRef)
		if err != nil {
			return nil, nil, fmt.Errorf(
				"load existing record %s: %w",
				item.ArtifactRef,
				err,
			)
		}
		records = append(records, record)
		itemsByRef[item.ArtifactRef] = item
	}
	return records, itemsByRef, nil
}

func indexMemoryBackfillRecords(
	records []*artifact.Artifact,
) map[string]*artifact.Artifact {
	indexed := make(
		map[string]*artifact.Artifact,
		len(records),
	)
	for _, record := range records {
		indexed[record.Meta.ID] = record
	}
	return indexed
}

func executeMemoryBackfillRoutes(
	ctx context.Context,
	projector *taskMemoryProjectionRuntime,
	routes []existingrecordprojection.Route,
	recordsByRef map[string]*artifact.Artifact,
	itemsByRef map[string]memoryBackfillItem,
	mode memoryBackfillMode,
) []memoryBackfillRouteResult {
	results := make(
		[]memoryBackfillRouteResult,
		0,
		len(routes),
	)
	for _, route := range routes {
		result := executeMemoryBackfillRoute(
			ctx,
			projector,
			route,
			recordsByRef[route.ArtifactRef()],
			itemsByRef[route.ArtifactRef()],
			mode,
		)
		results = append(results, result)
	}
	return results
}

func executeMemoryBackfillRoute(
	ctx context.Context,
	projector *taskMemoryProjectionRuntime,
	route existingrecordprojection.Route,
	record *artifact.Artifact,
	item memoryBackfillItem,
	mode memoryBackfillMode,
) memoryBackfillRouteResult {
	result := newMemoryBackfillRouteResult(route)
	arguments, err := existingrecordprojection.SourceArguments(
		route,
		record,
		memoryBackfillConcernCoordinates(item),
	)
	if err != nil {
		result.Result = "unresolved_source"
		result.Detail = err.Error()
		return result
	}
	execution, present := memoryBackfillExecutions[route.Projection()]
	if !present {
		result.Result = "unavailable"
		result.Detail = "no source-owned task-memory execution route"
		return result
	}
	report, applicable := projector.Project(
		ctx,
		taskMemoryProjectionRequest{
			ToolName:    execution.toolName,
			Action:      execution.action,
			ArtifactRef: route.ArtifactRef(),
			Arguments:   arguments,
			Mode:        taskMemoryProjectionMode(mode),
		},
	)
	if !applicable {
		result.Result = "unavailable"
		result.Detail = "selected task-memory projection route is not applicable"
		return result
	}
	result.Result = memoryBackfillProjectionResult(report)
	result.ProjectionReport = &report
	return result
}

func newMemoryBackfillRouteResult(
	route existingrecordprojection.Route,
) memoryBackfillRouteResult {
	requirements := route.Requirements()
	renderedRequirements := make(
		[]string,
		0,
		len(requirements),
	)
	for _, requirement := range requirements {
		renderedRequirements = append(
			renderedRequirements,
			string(requirement),
		)
	}
	return memoryBackfillRouteResult{
		ArtifactRef:  route.ArtifactRef(),
		ArtifactKind: string(route.ArtifactKind()),
		Version:      route.ArtifactVersion(),
		Projection:   string(route.Projection()),
		Requirements: renderedRequirements,
	}
}

func memoryBackfillConcernCoordinates(
	item memoryBackfillItem,
) existingrecordprojection.ConcernCoordinates {
	if item.EntityRef == nil {
		return existingrecordprojection.ConcernCoordinates{}
	}
	return existingrecordprojection.ConcernCoordinates{
		RefKindID:        item.EntityRef.RefKindID,
		ReferenceID:      item.EntityRef.ReferenceID,
		BoundedContextID: item.BoundedContextID,
	}
}

func memoryBackfillProjectionResult(
	report taskMemoryProjectionReport,
) string {
	switch report.AdmissionResult {
	case "validated_only":
		return "validated_only"
	case "already_projected":
		return "already_projected"
	case "committed":
		return "committed"
	case "commit_outcome_unknown":
		return "outcome_unknown"
	case "unavailable":
		return "unavailable"
	}
	switch report.AdapterResult {
	case "invalid":
		return "invalid"
	case "underdetermined":
		return "unresolved"
	default:
		return "unresolved"
	}
}

func projectMemoryBackfillDeferred(
	deferred []existingrecordprojection.Deferred,
) []memoryBackfillDeferredResult {
	results := make(
		[]memoryBackfillDeferredResult,
		0,
		len(deferred),
	)
	for _, item := range deferred {
		results = append(results, memoryBackfillDeferredResult{
			ArtifactRef:  item.ArtifactRef(),
			ArtifactKind: string(item.ArtifactKind()),
			Version:      item.ArtifactVersion(),
			Reason:       string(item.Reason()),
		})
	}
	return results
}

func memoryBackfillGraphRevision(
	ctx context.Context,
	projector *taskMemoryProjectionRuntime,
) (uint64, error) {
	current, err := projector.basis.snapshotLoader.LoadCurrentProjectSnapshot(
		ctx,
		projector.projectID,
	)
	if err != nil {
		return 0, fmt.Errorf(
			"load typed-memory backfill graph revision: %w",
			err,
		)
	}
	return current.Snapshot().GraphRevision().Value(), nil
}

func summarizeMemoryBackfill(
	selected int,
	routes []memoryBackfillRouteResult,
	deferred []memoryBackfillDeferredResult,
) memoryBackfillSummary {
	summary := memoryBackfillSummary{
		SelectedArtifacts: selected,
		PlannedRoutes:     len(routes),
		Deferred:          len(deferred),
	}
	for _, route := range routes {
		switch route.Result {
		case "validated_only":
			summary.ValidatedOnly++
		case "already_projected":
			summary.AlreadyProjected++
		case "committed":
			summary.Committed++
		case "invalid":
			summary.Invalid++
		case "unavailable":
			summary.Unavailable++
		case "outcome_unknown":
			summary.OutcomeUnknown++
		default:
			summary.Unresolved++
		}
	}
	return summary
}

func memoryBackfillBoundary(
	mode memoryBackfillMode,
) memoryBackfillAuthorityBoundary {
	mutation := "validation_only_zero_write"
	if mode == memoryBackfillApply {
		mutation = "explicit_selected_non_binding_projection_admission"
	}
	return memoryBackfillAuthorityBoundary{
		Mutation:      mutation,
		Schema:        "not_schema_declaration_or_activation",
		TypeEnvHead:   "not_typeenv_head_selection_or_mutation",
		Decision:      "not_decision_binding_or_supersession",
		Specification: "not_specification_approval_reopen_or_rebaseline",
		EvidenceTruth: "not_evidence_truth_or_quality",
		PerformedWork: "not_performed_work_or_completion",
		Publication:   "not_publication_or_release",
	}
}
