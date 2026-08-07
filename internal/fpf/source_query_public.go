package fpf

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// QueryPublicationView selects a public description of one canonical query
// result. It is independent from QueryMode: retrieval happens first, then this
// effect-free projection chooses the carrier appropriate to the current use.
type QueryPublicationView string

const (
	QueryPublicationViewWorking    QueryPublicationView = "working"
	QueryPublicationViewTrace      QueryPublicationView = "trace"
	QueryPublicationViewDiagnostic QueryPublicationView = "diagnostic"
)

const (
	queryTraceRefPrefix       = "fpf-query-trace:v1"
	workingDirectReferenceMax = 32
	workingMissingBasisMax    = 4
	workingMissingBasisRunes  = 240
)

// QueryPublicationRequest is a closed public-view request. TraceRef is an
// opaque replay coordinate returned by an earlier response; callers never need
// to copy source revisions, hashes, or repository-local paths.
type QueryPublicationRequest struct {
	view     QueryPublicationView
	traceRef string
}

func NewQueryPublicationRequest(rawView, rawTraceRef string) (QueryPublicationRequest, error) {
	view, err := parseQueryPublicationView(rawView)
	if err != nil {
		return QueryPublicationRequest{}, err
	}
	traceRef := strings.TrimSpace(rawTraceRef)
	if traceRef != "" && view == QueryPublicationViewWorking {
		return QueryPublicationRequest{}, fmt.Errorf("trace_ref requires trace or diagnostic view")
	}
	if traceRef != "" {
		if _, err := parseQueryTraceRef(traceRef); err != nil {
			return QueryPublicationRequest{}, err
		}
	}
	return QueryPublicationRequest{view: view, traceRef: traceRef}, nil
}

func (request QueryPublicationRequest) View() QueryPublicationView { return request.view }
func (request QueryPublicationRequest) TraceRef() string           { return request.traceRef }

func parseQueryPublicationView(raw string) (QueryPublicationView, error) {
	value := QueryPublicationView(strings.TrimSpace(raw))
	if value == "" {
		return QueryPublicationViewWorking, nil
	}
	if value == QueryPublicationViewWorking ||
		value == QueryPublicationViewTrace ||
		value == QueryPublicationViewDiagnostic {
		return value, nil
	}
	return "", fmt.Errorf(
		"unsupported FPF query view %q; expected working, trace, or diagnostic",
		raw,
	)
}

// QuerySourceSnapshot identifies the complete source publication from which a
// canonical QueryResult was retrieved. Its fields remain internal until an
// explicit trace or diagnostic projection is selected.
type QuerySourceSnapshot struct {
	indexSchemaVersion string
	revision           string
	readmeDigest       string
	specDigest         string
}

func NewQuerySourceSnapshot(
	indexSchemaVersion string,
	revision string,
	readmeDigest string,
	specDigest string,
) (QuerySourceSnapshot, error) {
	snapshot := QuerySourceSnapshot{
		indexSchemaVersion: strings.TrimSpace(indexSchemaVersion),
		revision:           strings.TrimSpace(revision),
		readmeDigest:       strings.TrimSpace(readmeDigest),
		specDigest:         strings.TrimSpace(specDigest),
	}
	if err := validateQuerySourceSnapshot(snapshot); err != nil {
		return QuerySourceSnapshot{}, err
	}
	return snapshot, nil
}

func (snapshot QuerySourceSnapshot) IndexSchemaVersion() string {
	return snapshot.indexSchemaVersion
}
func (snapshot QuerySourceSnapshot) Revision() string     { return snapshot.revision }
func (snapshot QuerySourceSnapshot) ReadmeDigest() string { return snapshot.readmeDigest }
func (snapshot QuerySourceSnapshot) SpecDigest() string   { return snapshot.specDigest }

// CanonicalQueryExecution pairs retrieval output with the exact publication
// snapshot that produced it. Construction validates provenance before any
// public projection can be encoded.
type CanonicalQueryExecution struct {
	request    QueryRequest
	evaluation QueryEvaluation
	snapshot   QuerySourceSnapshot
}

func NewCanonicalQueryExecution(
	request QueryRequest,
	evaluation QueryEvaluation,
	snapshot QuerySourceSnapshot,
) (CanonicalQueryExecution, error) {
	ownedRequest, err := cloneQueryRequest(request)
	if err != nil {
		return CanonicalQueryExecution{}, err
	}
	ownedEvaluation := newQueryEvaluation(
		evaluation.canonicalResult(),
		evaluation.ProducerIDs(),
	)
	execution := CanonicalQueryExecution{
		request:    ownedRequest,
		evaluation: ownedEvaluation,
		snapshot:   snapshot,
	}
	if err := validateCanonicalQueryExecution(execution); err != nil {
		return CanonicalQueryExecution{}, err
	}
	return execution, nil
}

func (execution CanonicalQueryExecution) Request() QueryRequest {
	request, _ := cloneQueryRequest(execution.request)
	return request
}

func (execution CanonicalQueryExecution) canonicalRequest() QueryRequest {
	return execution.request
}

func (execution CanonicalQueryExecution) Mode() QueryMode {
	request := execution.canonicalRequest()
	if request == nil {
		return ""
	}
	return request.Mode()
}

func (execution CanonicalQueryExecution) Result() QueryResult {
	return cloneQueryResult(execution.canonicalResult())
}

func (execution CanonicalQueryExecution) canonicalResult() QueryResult {
	return execution.evaluation.canonicalResult()
}

func (execution CanonicalQueryExecution) ProducerIDs() []string {
	return execution.evaluation.ProducerIDs()
}

func (execution CanonicalQueryExecution) Snapshot() QuerySourceSnapshot {
	return execution.snapshot
}

type PublishedQueryResultKind string

const (
	PublishedQueryResultKindExactHit       PublishedQueryResultKind = "exact_hit"
	PublishedQueryResultKindCandidateSet   PublishedQueryResultKind = "candidate_set"
	PublishedQueryResultKindAbstained      PublishedQueryResultKind = "abstained"
	PublishedQueryResultKindReplayMismatch PublishedQueryResultKind = "replay_mismatch"
)

// PublishedQueryResult is the closed public response family. Canonical
// QueryResult values never implement this interface and therefore cannot be
// passed accidentally to the shared public encoder.
type PublishedQueryResult interface {
	PublicationView() QueryPublicationView
	PublishedKind() PublishedQueryResultKind
	TraceReference() QueryTraceRef
	isPublishedQueryResult()
}

type PublishedQueryJSONStyle string

const (
	PublishedQueryJSONCompact  PublishedQueryJSONStyle = "compact"
	PublishedQueryJSONIndented PublishedQueryJSONStyle = "indented"
)

// EncodePublishedQuery is the only public FPF Query JSON encoder. Its input
// type prevents canonical QueryResult values from bypassing publication.
func EncodePublishedQuery(
	result PublishedQueryResult,
	style PublishedQueryJSONStyle,
) ([]byte, error) {
	if result == nil {
		return nil, fmt.Errorf("published FPF query result is required")
	}
	if err := validatePublishedQueryResult(result); err != nil {
		return nil, err
	}
	if style == PublishedQueryJSONCompact {
		return json.Marshal(result)
	}
	if style == PublishedQueryJSONIndented {
		return json.MarshalIndent(result, "", "  ")
	}
	return nil, fmt.Errorf("unsupported published FPF query JSON style %q", style)
}

func validatePublishedQueryResult(result PublishedQueryResult) error {
	view := result.PublicationView()
	if view != QueryPublicationViewWorking &&
		view != QueryPublicationViewTrace &&
		view != QueryPublicationViewDiagnostic {
		return fmt.Errorf("published FPF query result has unsupported view %q", view)
	}
	if result.PublishedKind() != PublishedQueryResultKindReplayMismatch {
		if _, err := parseQueryTraceRef(result.TraceReference().String()); err != nil {
			return fmt.Errorf("published FPF query result trace reference: %w", err)
		}
	}
	switch typed := result.(type) {
	case workingExactHit:
		return validatePublishedEnvelope(typed.View, QueryPublicationViewWorking)
	case workingCandidateSet:
		return validatePublishedEnvelope(typed.View, QueryPublicationViewWorking)
	case workingAbstained:
		return validatePublishedEnvelope(typed.View, QueryPublicationViewWorking)
	case traceExactHit:
		return validatePublishedEnvelope(typed.View, QueryPublicationViewTrace)
	case traceCandidateSet:
		return validatePublishedEnvelope(typed.View, QueryPublicationViewTrace)
	case traceAbstained:
		return validatePublishedEnvelope(typed.View, QueryPublicationViewTrace)
	case diagnosticExactHit:
		return validatePublishedEnvelope(typed.View, QueryPublicationViewDiagnostic)
	case diagnosticCandidateSet:
		return validatePublishedEnvelope(typed.View, QueryPublicationViewDiagnostic)
	case diagnosticAbstained:
		return validatePublishedEnvelope(typed.View, QueryPublicationViewDiagnostic)
	case queryReplayMismatch:
		if typed.View == QueryPublicationViewWorking {
			return fmt.Errorf("FPF query replay mismatch cannot use working view")
		}
		return validatePublishedEnvelope(typed.View, typed.PublicationView())
	default:
		return fmt.Errorf("unsupported published FPF query result %T", result)
	}
}

func validatePublishedEnvelope(got, want QueryPublicationView) error {
	if got != want {
		return fmt.Errorf("published FPF query wire view %q differs from carrier view %q", got, want)
	}
	return nil
}

type QueryTraceRef string

func (ref QueryTraceRef) String() string { return string(ref) }

type PublishedSourceRelation struct {
	Kind            SourceRelationKind `json:"kind"`
	TargetPatternID string             `json:"target_pattern_id"`
}

type PublishedSourceUseCues struct {
	ConditionText   string `json:"condition_text,omitempty"`
	FirstResultText string `json:"first_result_text,omitempty"`
	StopReturnText  string `json:"stop_return_text,omitempty"`
}

type PublishedResponseBudget struct {
	MaxCandidatesPerRole     int `json:"max_candidates_per_role,omitempty"`
	MaxTotalCandidates       int `json:"max_total_candidates,omitempty"`
	MaxExcerptCharacters     int `json:"max_excerpt_characters,omitempty"`
	MaxRelationsPerCandidate int `json:"max_relations_per_candidate,omitempty"`
}

type publishedExactSourceUnit interface {
	isPublishedExactSourceUnit()
}

type publishedLookupSourceUnit struct {
	UnitID                   string                            `json:"unit_id"`
	SourceID                 string                            `json:"source_id,omitempty"`
	SourceRole               SourceUnitRole                    `json:"source_role"`
	Title                    string                            `json:"title"`
	PatternID                string                            `json:"pattern_id,omitempty"`
	ParentPatternID          string                            `json:"parent_pattern_id,omitempty"`
	PublicationStatus        string                            `json:"publication_status,omitempty"`
	DirectRefs               []string                          `json:"direct_refs,omitempty"`
	DirectRefsTruncated      bool                              `json:"direct_refs_truncated,omitempty"`
	DirectRefsOmittedAtLeast int                               `json:"direct_refs_omitted_at_least,omitempty"`
	RelationProjection       *PublishedExactRelationProjection `json:"relation_projection,omitempty"`
	UseCues                  *PublishedSourceUseCues           `json:"use_cues,omitempty"`
	UseCuesTruncated         bool                              `json:"use_cues_truncated,omitempty"`
}

func (publishedLookupSourceUnit) isPublishedExactSourceUnit() {}

type publishedInspectSourceUnit struct {
	UnitID            string                    `json:"unit_id"`
	SourceID          string                    `json:"source_id,omitempty"`
	SourceRole        SourceUnitRole            `json:"source_role"`
	Title             string                    `json:"title"`
	Body              string                    `json:"body"`
	PatternID         string                    `json:"pattern_id,omitempty"`
	ParentPatternID   string                    `json:"parent_pattern_id,omitempty"`
	PublicationStatus string                    `json:"publication_status,omitempty"`
	DirectRefs        []string                  `json:"direct_refs,omitempty"`
	Relations         []PublishedSourceRelation `json:"relations,omitempty"`
	UseCues           *PublishedSourceUseCues   `json:"use_cues,omitempty"`
}

func (publishedInspectSourceUnit) isPublishedExactSourceUnit() {}

type PublishedExactRelationProjection struct {
	Relations      []PublishedSourceRelation `json:"relations"`
	Truncated      bool                      `json:"truncated"`
	OmittedAtLeast int                       `json:"omitted_at_least"`
}

type PublishedCandidateSourceUnit struct {
	UnitID                   string                                `json:"unit_id"`
	SourceID                 string                                `json:"source_id,omitempty"`
	SourceRole               SourceUnitRole                        `json:"source_role"`
	Title                    string                                `json:"title"`
	Excerpt                  string                                `json:"excerpt,omitempty"`
	ExcerptTruncated         bool                                  `json:"excerpt_truncated"`
	UseCues                  *PublishedSourceUseCues               `json:"use_cues,omitempty"`
	UseCuesTruncated         bool                                  `json:"use_cues_truncated,omitempty"`
	PatternID                string                                `json:"pattern_id,omitempty"`
	ParentPatternID          string                                `json:"parent_pattern_id,omitempty"`
	PublicationStatus        string                                `json:"publication_status,omitempty"`
	DirectRefs               []string                              `json:"direct_refs,omitempty"`
	DirectRefsTruncated      bool                                  `json:"direct_refs_truncated,omitempty"`
	DirectRefsOmittedAtLeast int                                   `json:"direct_refs_omitted_at_least,omitempty"`
	RelationProjection       *PublishedCandidateRelationProjection `json:"relation_projection,omitempty"`
}

type PublishedCandidateRelationProjection struct {
	Relations      []PublishedSourceRelation `json:"relations"`
	Truncated      bool                      `json:"truncated"`
	OmittedAtLeast int                       `json:"omitted_at_least"`
}

type PublishedSourceCandidate struct {
	Source PublishedCandidateSourceUnit `json:"source"`
}

type PublishedSourceCandidateGroup struct {
	Role       SourceUnitRole             `json:"source_role"`
	Candidates []PublishedSourceCandidate `json:"candidates"`
}

type PublishedCandidateTruncation struct {
	Applied            bool                    `json:"applied"`
	Budget             PublishedResponseBudget `json:"budget"`
	IncludedCandidates int                     `json:"included_candidates"`
	OmittedAtLeast     int                     `json:"omitted_at_least"`
}

type PublishedExactHitFields struct {
	Kind       QueryResultKind          `json:"kind"`
	Identifier string                   `json:"identifier"`
	Unit       publishedExactSourceUnit `json:"unit"`
}

type PublishedCandidateSetFields struct {
	Kind       QueryResultKind                 `json:"kind"`
	Groups     []PublishedSourceCandidateGroup `json:"groups"`
	Truncation PublishedCandidateTruncation    `json:"truncation"`
}

type PublishedAbstainedFields struct {
	Kind                  QueryResultKind `json:"kind"`
	Reason                string          `json:"reason"`
	MissingBasis          []string        `json:"missing_basis"`
	MissingBasisTruncated bool            `json:"missing_basis_truncated,omitempty"`
}

type workingExactHit struct {
	View     QueryPublicationView `json:"view"`
	TraceRef QueryTraceRef        `json:"trace_ref"`
	PublishedExactHitFields
}

func (workingExactHit) PublicationView() QueryPublicationView { return QueryPublicationViewWorking }
func (result workingExactHit) TraceReference() QueryTraceRef  { return result.TraceRef }
func (workingExactHit) PublishedKind() PublishedQueryResultKind {
	return PublishedQueryResultKindExactHit
}
func (workingExactHit) isPublishedQueryResult() {}

type workingCandidateSet struct {
	View     QueryPublicationView `json:"view"`
	TraceRef QueryTraceRef        `json:"trace_ref"`
	PublishedCandidateSetFields
}

func (workingCandidateSet) PublicationView() QueryPublicationView {
	return QueryPublicationViewWorking
}
func (result workingCandidateSet) TraceReference() QueryTraceRef { return result.TraceRef }
func (workingCandidateSet) PublishedKind() PublishedQueryResultKind {
	return PublishedQueryResultKindCandidateSet
}
func (workingCandidateSet) isPublishedQueryResult() {}

type workingAbstained struct {
	View     QueryPublicationView `json:"view"`
	TraceRef QueryTraceRef        `json:"trace_ref"`
	PublishedAbstainedFields
}

func (workingAbstained) PublicationView() QueryPublicationView { return QueryPublicationViewWorking }
func (result workingAbstained) TraceReference() QueryTraceRef  { return result.TraceRef }
func (workingAbstained) PublishedKind() PublishedQueryResultKind {
	return PublishedQueryResultKindAbstained
}
func (workingAbstained) isPublishedQueryResult() {}

type traceExactHit struct {
	View     QueryPublicationView `json:"view"`
	TraceRef QueryTraceRef        `json:"trace_ref"`
	PublishedExactHitFields
	Trace QueryResultTrace `json:"trace"`
}

func (traceExactHit) PublicationView() QueryPublicationView { return QueryPublicationViewTrace }
func (result traceExactHit) TraceReference() QueryTraceRef  { return result.TraceRef }
func (traceExactHit) PublishedKind() PublishedQueryResultKind {
	return PublishedQueryResultKindExactHit
}
func (traceExactHit) isPublishedQueryResult() {}

type traceCandidateSet struct {
	View     QueryPublicationView `json:"view"`
	TraceRef QueryTraceRef        `json:"trace_ref"`
	PublishedCandidateSetFields
	Trace QueryResultTrace `json:"trace"`
}

func (traceCandidateSet) PublicationView() QueryPublicationView { return QueryPublicationViewTrace }
func (result traceCandidateSet) TraceReference() QueryTraceRef  { return result.TraceRef }
func (traceCandidateSet) PublishedKind() PublishedQueryResultKind {
	return PublishedQueryResultKindCandidateSet
}
func (traceCandidateSet) isPublishedQueryResult() {}

type traceAbstained struct {
	View     QueryPublicationView `json:"view"`
	TraceRef QueryTraceRef        `json:"trace_ref"`
	PublishedAbstainedFields
	Trace QueryResultTrace `json:"trace"`
}

func (traceAbstained) PublicationView() QueryPublicationView { return QueryPublicationViewTrace }
func (result traceAbstained) TraceReference() QueryTraceRef  { return result.TraceRef }
func (traceAbstained) PublishedKind() PublishedQueryResultKind {
	return PublishedQueryResultKindAbstained
}
func (traceAbstained) isPublishedQueryResult() {}

type diagnosticExactHit struct {
	View     QueryPublicationView `json:"view"`
	TraceRef QueryTraceRef        `json:"trace_ref"`
	ExactHit
	Diagnostic QueryDiagnostics `json:"diagnostic"`
}

func (diagnosticExactHit) PublicationView() QueryPublicationView {
	return QueryPublicationViewDiagnostic
}
func (result diagnosticExactHit) TraceReference() QueryTraceRef { return result.TraceRef }
func (diagnosticExactHit) PublishedKind() PublishedQueryResultKind {
	return PublishedQueryResultKindExactHit
}
func (diagnosticExactHit) isPublishedQueryResult() {}

type diagnosticCandidateSet struct {
	View     QueryPublicationView `json:"view"`
	TraceRef QueryTraceRef        `json:"trace_ref"`
	CandidateSet
	Diagnostic QueryDiagnostics `json:"diagnostic"`
}

func (diagnosticCandidateSet) PublicationView() QueryPublicationView {
	return QueryPublicationViewDiagnostic
}
func (result diagnosticCandidateSet) TraceReference() QueryTraceRef { return result.TraceRef }
func (diagnosticCandidateSet) PublishedKind() PublishedQueryResultKind {
	return PublishedQueryResultKindCandidateSet
}
func (diagnosticCandidateSet) isPublishedQueryResult() {}

type diagnosticAbstained struct {
	View     QueryPublicationView `json:"view"`
	TraceRef QueryTraceRef        `json:"trace_ref"`
	Abstained
	Diagnostic QueryDiagnostics `json:"diagnostic"`
}

func (diagnosticAbstained) PublicationView() QueryPublicationView {
	return QueryPublicationViewDiagnostic
}
func (result diagnosticAbstained) TraceReference() QueryTraceRef { return result.TraceRef }
func (diagnosticAbstained) PublishedKind() PublishedQueryResultKind {
	return PublishedQueryResultKindAbstained
}
func (diagnosticAbstained) isPublishedQueryResult() {}

type QueryDiagnostics struct {
	RetrievalMode QueryMode `json:"retrieval_mode"`
	ProducerIDs   []string  `json:"producer_ids"`
}

type QueryReplayMismatchCode string

const (
	QueryReplayMismatchSourceSnapshot QueryReplayMismatchCode = "source_snapshot_mismatch"
	QueryReplayMismatchRequest        QueryReplayMismatchCode = "query_request_mismatch"
	QueryReplayMismatchResult         QueryReplayMismatchCode = "query_result_mismatch"
)

type queryReplayMismatch struct {
	View             QueryPublicationView     `json:"view"`
	Kind             PublishedQueryResultKind `json:"kind"`
	Code             QueryReplayMismatchCode  `json:"code"`
	ExpectedTraceRef QueryTraceRef            `json:"expected_trace_ref"`
	CurrentTraceRef  QueryTraceRef            `json:"current_trace_ref,omitempty"`
	CurrentBasisRef  string                   `json:"current_replay_basis_ref,omitempty"`
}

func (mismatch queryReplayMismatch) PublicationView() QueryPublicationView { return mismatch.View }
func (mismatch queryReplayMismatch) TraceReference() QueryTraceRef {
	if mismatch.CurrentTraceRef != "" {
		return mismatch.CurrentTraceRef
	}
	return mismatch.ExpectedTraceRef
}
func (queryReplayMismatch) PublishedKind() PublishedQueryResultKind {
	return PublishedQueryResultKindReplayMismatch
}
func (queryReplayMismatch) isPublishedQueryResult() {}

type TraceSourceSnapshot struct {
	IndexSchemaVersion   string `json:"index_schema_version"`
	SourceRevision       string `json:"source_revision"`
	ReadmeDocumentDigest string `json:"readme_document_digest"`
	SpecificationDigest  string `json:"specification_document_digest"`
}

type TraceProvenanceEntry struct {
	Ref         string `json:"ref"`
	SourcePath  string `json:"source_path"`
	StartLine   int    `json:"start_line"`
	EndLine     int    `json:"end_line"`
	ContentHash string `json:"content_hash"`
}

type TraceUnitBinding struct {
	UnitID        string `json:"unit_id"`
	ProvenanceRef string `json:"provenance_ref"`
}

type TraceRelationBinding struct {
	SubjectUnitID   string             `json:"subject_unit_id"`
	Ordinal         int                `json:"ordinal"`
	Kind            SourceRelationKind `json:"kind"`
	TargetPatternID string             `json:"target_pattern_id"`
	ProvenanceRef   string             `json:"provenance_ref"`
}

type TraceRetrievalEvidenceBinding struct {
	CandidateUnitID string `json:"candidate_unit_id"`
	GroundOrdinal   int    `json:"ground_ordinal"`
	EvidenceUnitID  string `json:"evidence_unit_id"`
	ProvenanceRef   string `json:"provenance_ref"`
}

// QueryResultTrace stores each SourceProvenance once. Combining a binding's
// entry with SourceSnapshot.SourceRevision reconstructs the canonical value.
type QueryResultTrace struct {
	SourceSnapshot            TraceSourceSnapshot             `json:"source_snapshot"`
	Provenance                []TraceProvenanceEntry          `json:"provenance"`
	UnitBindings              []TraceUnitBinding              `json:"unit_bindings,omitempty"`
	RelationBindings          []TraceRelationBinding          `json:"relation_bindings,omitempty"`
	RetrievalEvidenceBindings []TraceRetrievalEvidenceBinding `json:"retrieval_evidence_bindings,omitempty"`
}

type queryTraceCoordinates struct {
	snapshotDigest string
	requestDigest  string
	resultDigest   string
}

// QueryReplayPreflight binds the typed retrieval request to the source
// snapshot before retrieval executes. The CLI/MCP shell must call Check before
// Query whenever a replay ref is present.
type QueryReplayPreflight struct {
	request     QueryRequest
	snapshot    QuerySourceSnapshot
	coordinates queryTraceCoordinates
}

func NewQueryReplayPreflight(
	request QueryRequest,
	snapshot QuerySourceSnapshot,
) (QueryReplayPreflight, error) {
	ownedRequest, err := cloneQueryRequest(request)
	if err != nil {
		return QueryReplayPreflight{}, err
	}
	if err := validateQuerySourceSnapshot(snapshot); err != nil {
		return QueryReplayPreflight{}, err
	}
	coordinates, err := buildQueryReplayCoordinates(ownedRequest, snapshot, nil)
	if err != nil {
		return QueryReplayPreflight{}, err
	}
	return QueryReplayPreflight{
		request:     ownedRequest,
		snapshot:    snapshot,
		coordinates: coordinates,
	}, nil
}

func (preflight QueryReplayPreflight) Complete(
	evaluation QueryEvaluation,
) (CanonicalQueryExecution, error) {
	return NewCanonicalQueryExecution(preflight.request, evaluation, preflight.snapshot)
}

// Check returns proceed=false with a typed mismatch when the source snapshot
// or typed request changed. In that case retrieval must not run.
func (preflight QueryReplayPreflight) Check(
	request QueryPublicationRequest,
) (PublishedQueryResult, bool, error) {
	if request.TraceRef() == "" {
		return nil, true, nil
	}
	expected, err := parseQueryTraceRef(request.TraceRef())
	if err != nil {
		return nil, false, err
	}
	if expected.snapshotDigest != preflight.coordinates.snapshotDigest {
		return newQueryReplayPreflightMismatch(
			request,
			QueryReplayMismatchSourceSnapshot,
			preflight.coordinates,
		), false, nil
	}
	if expected.requestDigest != preflight.coordinates.requestDigest {
		return newQueryReplayPreflightMismatch(
			request,
			QueryReplayMismatchRequest,
			preflight.coordinates,
		), false, nil
	}
	return nil, true, nil
}

func newQueryReplayPreflightMismatch(
	request QueryPublicationRequest,
	code QueryReplayMismatchCode,
	current queryTraceCoordinates,
) queryReplayMismatch {
	return queryReplayMismatch{
		View:             request.View(),
		Kind:             PublishedQueryResultKindReplayMismatch,
		Code:             code,
		ExpectedTraceRef: QueryTraceRef(request.TraceRef()),
		CurrentBasisRef:  buildQueryReplayBasisRef(current),
	}
}

func buildQueryReplayBasisRef(coordinates queryTraceCoordinates) string {
	return strings.Join([]string{
		"fpf-query-replay-basis:v1",
		coordinates.snapshotDigest,
		coordinates.requestDigest,
	}, ":")
}

// ProjectQueryResult performs the sole canonical-result to public-carrier
// conversion used by CLI and MCP. Replay drift is a typed result, not an error
// and never silently falls through to current source.
func ProjectQueryResult(
	execution CanonicalQueryExecution,
	request QueryPublicationRequest,
) (PublishedQueryResult, error) {
	if err := validateCanonicalQueryExecution(execution); err != nil {
		return nil, err
	}
	preflight, err := NewQueryReplayPreflight(execution.canonicalRequest(), execution.Snapshot())
	if err != nil {
		return nil, err
	}
	mismatch, proceed, err := preflight.Check(request)
	if err != nil {
		return nil, err
	}
	if !proceed {
		return mismatch, nil
	}
	coordinates, ref, err := buildQueryTraceRef(execution)
	if err != nil {
		return nil, err
	}
	if request.TraceRef() != "" {
		expected, err := parseQueryTraceRef(request.TraceRef())
		if err != nil {
			return nil, err
		}
		if expected.resultDigest != coordinates.resultDigest {
			return newQueryReplayMismatch(
				request,
				QueryReplayMismatchResult,
				ref,
			), nil
		}
	}
	return projectCurrentQueryResult(execution, request.View(), ref)
}

func newQueryReplayMismatch(
	request QueryPublicationRequest,
	code QueryReplayMismatchCode,
	current QueryTraceRef,
) queryReplayMismatch {
	return queryReplayMismatch{
		View:             request.View(),
		Kind:             PublishedQueryResultKindReplayMismatch,
		Code:             code,
		ExpectedTraceRef: QueryTraceRef(request.TraceRef()),
		CurrentTraceRef:  current,
	}
}

func projectCurrentQueryResult(
	execution CanonicalQueryExecution,
	view QueryPublicationView,
	traceRef QueryTraceRef,
) (PublishedQueryResult, error) {
	projectors := map[QueryPublicationView]func(CanonicalQueryExecution, QueryTraceRef) (PublishedQueryResult, error){
		QueryPublicationViewWorking:    projectWorkingQueryResult,
		QueryPublicationViewTrace:      projectTraceQueryResult,
		QueryPublicationViewDiagnostic: projectDiagnosticQueryResult,
	}
	projector, exists := projectors[view]
	if !exists {
		return nil, fmt.Errorf("unsupported FPF query view %q", view)
	}
	return projector(execution, traceRef)
}

func projectWorkingQueryResult(
	execution CanonicalQueryExecution,
	traceRef QueryTraceRef,
) (PublishedQueryResult, error) {
	switch result := execution.canonicalResult().(type) {
	case ExactHit:
		fields, err := projectExactHitFields(result, execution.canonicalRequest())
		if err != nil {
			return nil, err
		}
		return workingExactHit{
			View:                    QueryPublicationViewWorking,
			TraceRef:                traceRef,
			PublishedExactHitFields: fields,
		}, nil
	case CandidateSet:
		return workingCandidateSet{
			View:                        QueryPublicationViewWorking,
			TraceRef:                    traceRef,
			PublishedCandidateSetFields: projectCandidateSetFields(result),
		}, nil
	case Abstained:
		return workingAbstained{
			View:                     QueryPublicationViewWorking,
			TraceRef:                 traceRef,
			PublishedAbstainedFields: projectAbstainedFields(result),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported canonical FPF query result %T", execution.canonicalResult())
	}
}

func projectTraceQueryResult(
	execution CanonicalQueryExecution,
	traceRef QueryTraceRef,
) (PublishedQueryResult, error) {
	trace := buildQueryResultTrace(execution)
	switch result := execution.canonicalResult().(type) {
	case ExactHit:
		fields, err := projectExactHitFields(result, execution.canonicalRequest())
		if err != nil {
			return nil, err
		}
		return traceExactHit{
			View:                    QueryPublicationViewTrace,
			TraceRef:                traceRef,
			PublishedExactHitFields: fields,
			Trace:                   trace,
		}, nil
	case CandidateSet:
		return traceCandidateSet{
			View:                        QueryPublicationViewTrace,
			TraceRef:                    traceRef,
			PublishedCandidateSetFields: projectCandidateSetFields(result),
			Trace:                       trace,
		}, nil
	case Abstained:
		return traceAbstained{
			View:                     QueryPublicationViewTrace,
			TraceRef:                 traceRef,
			PublishedAbstainedFields: projectAbstainedFields(result),
			Trace:                    trace,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported canonical FPF query result %T", execution.canonicalResult())
	}
}

func projectDiagnosticQueryResult(
	execution CanonicalQueryExecution,
	traceRef QueryTraceRef,
) (PublishedQueryResult, error) {
	switch result := execution.canonicalResult().(type) {
	case ExactHit:
		return diagnosticExactHit{
			View:       QueryPublicationViewDiagnostic,
			TraceRef:   traceRef,
			ExactHit:   result,
			Diagnostic: diagnosticsForQueryExecution(execution),
		}, nil
	case CandidateSet:
		return diagnosticCandidateSet{
			View:         QueryPublicationViewDiagnostic,
			TraceRef:     traceRef,
			CandidateSet: result,
			Diagnostic:   diagnosticsForQueryExecution(execution),
		}, nil
	case Abstained:
		return diagnosticAbstained{
			View:       QueryPublicationViewDiagnostic,
			TraceRef:   traceRef,
			Abstained:  result,
			Diagnostic: diagnosticsForQueryExecution(execution),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported canonical FPF query result %T", execution.canonicalResult())
	}
}

func diagnosticsForQueryExecution(execution CanonicalQueryExecution) QueryDiagnostics {
	return QueryDiagnostics{
		RetrievalMode: execution.Mode(),
		ProducerIDs:   execution.ProducerIDs(),
	}
}

func projectExactHitFields(
	result ExactHit,
	request QueryRequest,
) (PublishedExactHitFields, error) {
	unit, err := projectExactSourceUnit(result.Unit, request)
	if err != nil {
		return PublishedExactHitFields{}, err
	}
	return PublishedExactHitFields{
		Kind:       result.Kind,
		Identifier: result.Identifier,
		Unit:       unit,
	}, nil
}

func projectExactSourceUnit(
	unit SourceUnit,
	request QueryRequest,
) (publishedExactSourceUnit, error) {
	switch typed := request.(type) {
	case LookupQuery:
		return projectLookupSourceUnit(unit, normalizeResponseBudget(typed.ResponseBudget)), nil
	case InspectQuery:
		return projectInspectSourceUnit(unit), nil
	default:
		return nil, fmt.Errorf("exact FPF query hit is not valid for %q mode", request.Mode())
	}
}

func projectLookupSourceUnit(
	unit SourceUnit,
	budget ResponseBudget,
) publishedLookupSourceUnit {
	directRefs, directRefsTruncated, directRefsOmitted := projectDirectRefs(unit.DirectRefs)
	useCues, useCuesTruncated := projectBoundedUseCues(
		unit.UseCues,
		budget.MaxExcerptCharacters,
	)
	return publishedLookupSourceUnit{
		UnitID:                   unit.UnitID,
		SourceID:                 unit.SourceID,
		SourceRole:               unit.Role,
		Title:                    unit.Title,
		PatternID:                unit.PatternID,
		ParentPatternID:          unit.ParentPatternID,
		PublicationStatus:        unit.PublicationStatus,
		DirectRefs:               directRefs,
		DirectRefsTruncated:      directRefsTruncated,
		DirectRefsOmittedAtLeast: directRefsOmitted,
		RelationProjection: projectExactRelationProjection(
			unit.Relations,
			budget.MaxRelationsPerCandidate,
		),
		UseCues:          useCues,
		UseCuesTruncated: useCuesTruncated,
	}
}

func projectInspectSourceUnit(unit SourceUnit) publishedInspectSourceUnit {
	return publishedInspectSourceUnit{
		UnitID:            unit.UnitID,
		SourceID:          unit.SourceID,
		SourceRole:        unit.Role,
		Title:             unit.Title,
		Body:              unit.Body,
		PatternID:         unit.PatternID,
		ParentPatternID:   unit.ParentPatternID,
		PublicationStatus: unit.PublicationStatus,
		DirectRefs:        append([]string(nil), unit.DirectRefs...),
		Relations:         projectSourceRelations(unit.Relations),
		UseCues:           projectUseCues(unit.UseCues),
	}
}

func projectExactRelationProjection(
	relations []SourceRelation,
	limit int,
) *PublishedExactRelationProjection {
	if len(relations) == 0 {
		return nil
	}
	visible := min(len(relations), max(0, limit))
	return &PublishedExactRelationProjection{
		Relations:      projectSourceRelations(relations[:visible]),
		Truncated:      visible < len(relations),
		OmittedAtLeast: len(relations) - visible,
	}
}

func projectSourceRelations(relations []SourceRelation) []PublishedSourceRelation {
	result := make([]PublishedSourceRelation, 0, len(relations))
	for _, relation := range relations {
		result = append(result, PublishedSourceRelation{
			Kind:            relation.Kind,
			TargetPatternID: relation.TargetPatternID,
		})
	}
	return result
}

func projectCandidateSetFields(result CandidateSet) PublishedCandidateSetFields {
	groups := make([]PublishedSourceCandidateGroup, 0, len(result.Groups))
	for _, group := range result.Groups {
		groups = append(groups, projectSourceCandidateGroup(group))
	}
	return PublishedCandidateSetFields{
		Kind:   result.Kind,
		Groups: groups,
		Truncation: PublishedCandidateTruncation{
			Applied:            result.Truncation.Applied,
			Budget:             projectResponseBudget(result.Truncation.Budget),
			IncludedCandidates: result.Truncation.IncludedCandidates,
			OmittedAtLeast:     result.Truncation.OmittedAtLeast,
		},
	}
}

func projectSourceCandidateGroup(group SourceCandidateGroup) PublishedSourceCandidateGroup {
	candidates := make([]PublishedSourceCandidate, 0, len(group.Candidates))
	for _, candidate := range group.Candidates {
		candidates = append(candidates, PublishedSourceCandidate{
			Source: projectCandidateSourceUnit(candidate.Source),
		})
	}
	return PublishedSourceCandidateGroup{
		Role:       group.Role,
		Candidates: candidates,
	}
}

func projectCandidateSourceUnit(unit CandidateSourceUnit) PublishedCandidateSourceUnit {
	directRefs, directRefsTruncated, directRefsOmitted := projectDirectRefs(unit.DirectRefs)
	return PublishedCandidateSourceUnit{
		UnitID:                   unit.UnitID,
		SourceID:                 unit.SourceID,
		SourceRole:               unit.SourceRole,
		Title:                    unit.Title,
		Excerpt:                  unit.Excerpt,
		ExcerptTruncated:         unit.ExcerptTruncated,
		UseCues:                  cloneOptionalUseCues(unit.UseCues),
		UseCuesTruncated:         unit.UseCuesTruncated,
		PatternID:                unit.PatternID,
		ParentPatternID:          unit.ParentPatternID,
		PublicationStatus:        unit.PublicationStatus,
		DirectRefs:               directRefs,
		DirectRefsTruncated:      directRefsTruncated,
		DirectRefsOmittedAtLeast: directRefsOmitted,
		RelationProjection:       projectPublishedRelationProjection(unit.RelationProjection),
	}
}

func projectPublishedRelationProjection(
	projection *CandidateRelationProjection,
) *PublishedCandidateRelationProjection {
	if projection == nil {
		return nil
	}
	return &PublishedCandidateRelationProjection{
		Relations:      projectSourceRelations(projection.Relations),
		Truncated:      projection.Truncated,
		OmittedAtLeast: projection.OmittedAtLeast,
	}
}

func projectDirectRefs(values []string) ([]string, bool, int) {
	visible := min(len(values), workingDirectReferenceMax)
	result := append([]string(nil), values[:visible]...)
	omitted := len(values) - visible
	return result, omitted > 0, omitted
}

func projectUseCues(value SourceUseCues) *PublishedSourceUseCues {
	if value == (SourceUseCues{}) {
		return nil
	}
	return &PublishedSourceUseCues{
		ConditionText:   value.ConditionText,
		FirstResultText: value.FirstResultText,
		StopReturnText:  value.StopReturnText,
	}
}

func cloneOptionalUseCues(value *SourceUseCues) *PublishedSourceUseCues {
	if value == nil {
		return nil
	}
	return projectUseCues(*value)
}

func projectBoundedUseCues(
	value SourceUseCues,
	totalLimit int,
) (*PublishedSourceUseCues, bool) {
	if value == (SourceUseCues{}) {
		return nil, false
	}
	values := []string{
		value.ConditionText,
		value.FirstResultText,
		value.StopReturnText,
	}
	allocations := splitSourceTextBudget(values, totalLimit)
	condition, conditionTruncated := boundedSourceText(values[0], allocations[0])
	firstResult, firstResultTruncated := boundedSourceText(values[1], allocations[1])
	stopReturn, stopReturnTruncated := boundedSourceText(values[2], allocations[2])
	return &PublishedSourceUseCues{
		ConditionText:   condition,
		FirstResultText: firstResult,
		StopReturnText:  stopReturn,
	}, conditionTruncated || firstResultTruncated || stopReturnTruncated
}

func projectResponseBudget(value ResponseBudget) PublishedResponseBudget {
	return PublishedResponseBudget(value)
}

func projectAbstainedFields(result Abstained) PublishedAbstainedFields {
	missingBasis := make([]string, 0, workingMissingBasisMax)
	truncated := len(result.MissingBasis) > workingMissingBasisMax
	for _, basis := range result.MissingBasis[:min(len(result.MissingBasis), workingMissingBasisMax)] {
		projected, itemTruncated := boundedSourceText(basis, workingMissingBasisRunes)
		missingBasis = append(missingBasis, projected)
		truncated = truncated || itemTruncated
	}
	return PublishedAbstainedFields{
		Kind:                  result.Kind,
		Reason:                result.Reason,
		MissingBasis:          missingBasis,
		MissingBasisTruncated: truncated,
	}
}

func buildQueryTraceRef(
	execution CanonicalQueryExecution,
) (queryTraceCoordinates, QueryTraceRef, error) {
	coordinates, err := buildQueryReplayCoordinates(
		execution.canonicalRequest(),
		execution.Snapshot(),
		execution.canonicalResult(),
	)
	if err != nil {
		return queryTraceCoordinates{}, "", err
	}
	ref := QueryTraceRef(strings.Join([]string{
		queryTraceRefPrefix,
		coordinates.snapshotDigest,
		coordinates.requestDigest,
		coordinates.resultDigest,
	}, ":"))
	return coordinates, ref, nil
}

func buildQueryReplayCoordinates(
	request QueryRequest,
	snapshot QuerySourceSnapshot,
	result QueryResult,
) (queryTraceCoordinates, error) {
	snapshotDigest, err := digestQueryTraceValue("source-snapshot", struct {
		IndexSchemaVersion string `json:"index_schema_version"`
		Revision           string `json:"revision"`
		ReadmeDigest       string `json:"readme_digest"`
		SpecDigest         string `json:"spec_digest"`
	}{
		IndexSchemaVersion: snapshot.IndexSchemaVersion(),
		Revision:           snapshot.Revision(),
		ReadmeDigest:       snapshot.ReadmeDigest(),
		SpecDigest:         snapshot.SpecDigest(),
	})
	if err != nil {
		return queryTraceCoordinates{}, err
	}
	requestDigest, err := digestQueryTraceValue("typed-query-request", struct {
		Mode    QueryMode    `json:"mode"`
		Request QueryRequest `json:"request"`
	}{
		Mode:    request.Mode(),
		Request: request,
	})
	if err != nil {
		return queryTraceCoordinates{}, err
	}
	coordinates := queryTraceCoordinates{
		snapshotDigest: snapshotDigest,
		requestDigest:  requestDigest,
	}
	if result == nil {
		return coordinates, nil
	}
	resultDigest, err := digestQueryTraceValue("canonical-query-result", result)
	if err != nil {
		return queryTraceCoordinates{}, err
	}
	coordinates.resultDigest = resultDigest
	return coordinates, nil
}

func digestQueryTraceValue(domain string, value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode %s for FPF query trace: %w", domain, err)
	}
	hash := sha256.New()
	_, _ = hash.Write([]byte("haft/fpf-query-trace/v1/"))
	_, _ = hash.Write([]byte(domain))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write(encoded)
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func parseQueryTraceRef(raw string) (queryTraceCoordinates, error) {
	parts := strings.Split(strings.TrimSpace(raw), ":")
	if len(parts) != 5 || strings.Join(parts[:2], ":") != queryTraceRefPrefix {
		return queryTraceCoordinates{}, fmt.Errorf("trace_ref is not a canonical %s reference", queryTraceRefPrefix)
	}
	if !isSHA256Hex(parts[2]) || !isSHA256Hex(parts[3]) || !isSHA256Hex(parts[4]) {
		return queryTraceCoordinates{}, fmt.Errorf("trace_ref contains an invalid digest")
	}
	return queryTraceCoordinates{
		snapshotDigest: parts[2],
		requestDigest:  parts[3],
		resultDigest:   parts[4],
	}, nil
}

func isSHA256Hex(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}

func buildQueryResultTrace(execution CanonicalQueryExecution) QueryResultTrace {
	collector := newQueryTraceCollector(execution.Snapshot())
	collector.collectResult(execution.canonicalResult())
	return collector.result()
}

type queryTraceCollector struct {
	snapshot                  QuerySourceSnapshot
	provenanceByRef           map[string]TraceProvenanceEntry
	unitBindings              map[string]TraceUnitBinding
	relationBindings          map[string]TraceRelationBinding
	retrievalEvidenceBindings map[string]TraceRetrievalEvidenceBinding
}

func newQueryTraceCollector(snapshot QuerySourceSnapshot) *queryTraceCollector {
	return &queryTraceCollector{
		snapshot:                  snapshot,
		provenanceByRef:           make(map[string]TraceProvenanceEntry),
		unitBindings:              make(map[string]TraceUnitBinding),
		relationBindings:          make(map[string]TraceRelationBinding),
		retrievalEvidenceBindings: make(map[string]TraceRetrievalEvidenceBinding),
	}
}

func (collector *queryTraceCollector) collectResult(result QueryResult) {
	switch typed := result.(type) {
	case ExactHit:
		collector.collectUnit(typed.Unit)
	case CandidateSet:
		collector.collectGroups(typed.Groups)
	case Abstained:
		return
	}
}

func (collector *queryTraceCollector) collectGroups(groups []SourceCandidateGroup) {
	for _, group := range groups {
		for _, candidate := range group.Candidates {
			collector.collectCandidate(candidate)
		}
	}
}

func (collector *queryTraceCollector) collectCandidate(candidate SourceCandidate) {
	collector.bindUnit(candidate.Source.UnitID, candidate.Source.Provenance)
	collector.bindRetrievalEvidence(candidate.Source.UnitID, candidate.MatchGrounds)
	projection := candidate.Source.RelationProjection
	if projection == nil {
		return
	}
	for ordinal, relation := range projection.Relations {
		collector.bindRelation(projection.CanonicalUnitID, ordinal, relation)
	}
}

func (collector *queryTraceCollector) bindRetrievalEvidence(
	candidateUnitID string,
	grounds []MatchGround,
) {
	for ordinal, ground := range grounds {
		if ground.Evidence == nil {
			continue
		}
		ref := collector.addProvenance(ground.Evidence.Provenance)
		binding := TraceRetrievalEvidenceBinding{
			CandidateUnitID: candidateUnitID,
			GroundOrdinal:   ordinal,
			EvidenceUnitID:  ground.Evidence.UnitID,
			ProvenanceRef:   ref,
		}
		key := fmt.Sprintf("%s\x00%08d\x00%s\x00%s", candidateUnitID, ordinal, binding.EvidenceUnitID, ref)
		collector.retrievalEvidenceBindings[key] = binding
	}
}

func (collector *queryTraceCollector) collectUnit(unit SourceUnit) {
	collector.bindUnit(unit.UnitID, unit.Provenance)
	for ordinal, relation := range unit.Relations {
		collector.bindRelation(unit.UnitID, ordinal, relation)
	}
}

func (collector *queryTraceCollector) bindUnit(unitID string, provenance SourceProvenance) {
	ref := collector.addProvenance(provenance)
	binding := TraceUnitBinding{UnitID: unitID, ProvenanceRef: ref}
	collector.unitBindings[unitID+"\x00"+ref] = binding
}

func (collector *queryTraceCollector) bindRelation(
	subjectUnitID string,
	ordinal int,
	relation SourceRelation,
) {
	ref := collector.addProvenance(relation.Provenance)
	binding := TraceRelationBinding{
		SubjectUnitID:   subjectUnitID,
		Ordinal:         ordinal,
		Kind:            relation.Kind,
		TargetPatternID: relation.TargetPatternID,
		ProvenanceRef:   ref,
	}
	key := strings.Join([]string{
		subjectUnitID,
		fmt.Sprintf("%d", ordinal),
		string(relation.Kind),
		relation.TargetPatternID,
		ref,
	}, "\x00")
	collector.relationBindings[key] = binding
}

func (collector *queryTraceCollector) addProvenance(provenance SourceProvenance) string {
	ref := provenanceTraceRef(provenance)
	collector.provenanceByRef[ref] = TraceProvenanceEntry{
		Ref:         ref,
		SourcePath:  provenance.SourcePath,
		StartLine:   provenance.StartLine,
		EndLine:     provenance.EndLine,
		ContentHash: provenance.ContentHash,
	}
	return ref
}

func provenanceTraceRef(provenance SourceProvenance) string {
	encoded, _ := json.Marshal(provenance)
	hash := sha256.Sum256(append([]byte("haft/fpf-source-provenance/v1\x00"), encoded...))
	return "fpf-source-provenance:sha256:" + hex.EncodeToString(hash[:])
}

func (collector *queryTraceCollector) result() QueryResultTrace {
	provenance := mapValues(collector.provenanceByRef)
	slices.SortFunc(provenance, func(left, right TraceProvenanceEntry) int {
		return strings.Compare(left.Ref, right.Ref)
	})
	unitBindings := mapValues(collector.unitBindings)
	slices.SortFunc(unitBindings, func(left, right TraceUnitBinding) int {
		return strings.Compare(left.UnitID+left.ProvenanceRef, right.UnitID+right.ProvenanceRef)
	})
	relationBindings := mapValues(collector.relationBindings)
	slices.SortFunc(relationBindings, func(left, right TraceRelationBinding) int {
		leftKey := fmt.Sprintf("%s\x00%08d\x00%s\x00%s", left.SubjectUnitID, left.Ordinal, left.Kind, left.TargetPatternID)
		rightKey := fmt.Sprintf("%s\x00%08d\x00%s\x00%s", right.SubjectUnitID, right.Ordinal, right.Kind, right.TargetPatternID)
		return strings.Compare(leftKey, rightKey)
	})
	retrievalEvidenceBindings := mapValues(collector.retrievalEvidenceBindings)
	slices.SortFunc(retrievalEvidenceBindings, func(left, right TraceRetrievalEvidenceBinding) int {
		leftKey := fmt.Sprintf("%s\x00%08d\x00%s", left.CandidateUnitID, left.GroundOrdinal, left.EvidenceUnitID)
		rightKey := fmt.Sprintf("%s\x00%08d\x00%s", right.CandidateUnitID, right.GroundOrdinal, right.EvidenceUnitID)
		return strings.Compare(leftKey, rightKey)
	})
	return QueryResultTrace{
		SourceSnapshot: TraceSourceSnapshot{
			IndexSchemaVersion:   collector.snapshot.IndexSchemaVersion(),
			SourceRevision:       collector.snapshot.Revision(),
			ReadmeDocumentDigest: collector.snapshot.ReadmeDigest(),
			SpecificationDigest:  collector.snapshot.SpecDigest(),
		},
		Provenance:                provenance,
		UnitBindings:              unitBindings,
		RelationBindings:          relationBindings,
		RetrievalEvidenceBindings: retrievalEvidenceBindings,
	}
}

func mapValues[K comparable, V any](values map[K]V) []V {
	result := make([]V, 0, len(values))
	for _, value := range values {
		result = append(result, value)
	}
	return result
}

func validateCanonicalQueryExecution(execution CanonicalQueryExecution) error {
	if execution.canonicalRequest() == nil {
		return fmt.Errorf("canonical FPF query request is required")
	}
	if execution.canonicalResult() == nil {
		return fmt.Errorf("canonical FPF query result is required")
	}
	producerIDs := execution.ProducerIDs()
	if len(producerIDs) == 0 {
		return fmt.Errorf("canonical FPF query producer identity is required")
	}
	seenProducerIDs := make(map[string]struct{}, len(producerIDs))
	for _, producerID := range producerIDs {
		trimmed := strings.TrimSpace(producerID)
		if trimmed == "" || trimmed != producerID {
			return fmt.Errorf("canonical FPF query producer identity is invalid")
		}
		if _, exists := seenProducerIDs[producerID]; exists {
			return fmt.Errorf("canonical FPF query producer identity %q is duplicated", producerID)
		}
		seenProducerIDs[producerID] = struct{}{}
	}
	snapshot := execution.Snapshot()
	if err := validateQuerySourceSnapshot(snapshot); err != nil {
		return err
	}
	if err := validateQueryModeResult(execution.Mode(), execution.canonicalResult().ResultKind()); err != nil {
		return err
	}
	switch result := execution.canonicalResult().(type) {
	case ExactHit:
		if result.Kind != QueryResultKindExactHit {
			return fmt.Errorf("canonical ExactHit kind is %q", result.Kind)
		}
		if err := validateQuerySourceUnit(result.Unit, snapshot); err != nil {
			return err
		}
	case CandidateSet:
		if result.Kind != QueryResultKindCandidateSet {
			return fmt.Errorf("canonical CandidateSet kind is %q", result.Kind)
		}
		if err := validateQueryCandidateGroups(result.Groups, snapshot); err != nil {
			return err
		}
	case Abstained:
		if result.Kind != QueryResultKindAbstained {
			return fmt.Errorf("canonical Abstained kind is %q", result.Kind)
		}
	default:
		return fmt.Errorf("unsupported canonical FPF query result %T", execution.canonicalResult())
	}
	return nil
}

func validateQueryModeResult(mode QueryMode, kind QueryResultKind) error {
	allowed := map[QueryMode]map[QueryResultKind]struct{}{
		QueryModeConcern: {
			QueryResultKindCandidateSet: {},
			QueryResultKindAbstained:    {},
		},
		QueryModeLookup: {
			QueryResultKindExactHit:     {},
			QueryResultKindCandidateSet: {},
			QueryResultKindAbstained:    {},
		},
		QueryModeInspect: {
			QueryResultKindExactHit:  {},
			QueryResultKindAbstained: {},
		},
	}
	allowedKinds, exists := allowed[mode]
	if !exists {
		return fmt.Errorf("canonical FPF query mode %q is unsupported", mode)
	}
	if _, exists := allowedKinds[kind]; !exists {
		return fmt.Errorf("canonical FPF query result %q is invalid for %q mode", kind, mode)
	}
	return nil
}

func validateQuerySourceSnapshot(snapshot QuerySourceSnapshot) error {
	if snapshot.IndexSchemaVersion() != SpecIndexSchemaVersion {
		return fmt.Errorf(
			"canonical FPF query source index schema is %q, want %q",
			snapshot.IndexSchemaVersion(),
			SpecIndexSchemaVersion,
		)
	}
	if !isCanonicalSourceRevision(snapshot.Revision()) {
		return fmt.Errorf("canonical FPF query source revision is malformed")
	}
	if !isCanonicalSHA256Digest(snapshot.ReadmeDigest()) {
		return fmt.Errorf("canonical FPF query README digest is malformed")
	}
	if !isCanonicalSHA256Digest(snapshot.SpecDigest()) {
		return fmt.Errorf("canonical FPF query specification digest is malformed")
	}
	return nil
}

func isCanonicalSourceRevision(value string) bool {
	if len(value) == 40 {
		decoded, err := hex.DecodeString(value)
		return err == nil && len(decoded) == 20 && strings.ToLower(value) == value
	}
	return isCanonicalSHA256Digest(value)
}

func isCanonicalSHA256Digest(value string) bool {
	const prefix = "sha256:"
	if !strings.HasPrefix(value, prefix) {
		return false
	}
	digest := strings.TrimPrefix(value, prefix)
	return isSHA256Hex(digest) && strings.ToLower(digest) == digest
}

func validateQueryCandidateGroups(
	groups []SourceCandidateGroup,
	snapshot QuerySourceSnapshot,
) error {
	for _, group := range groups {
		for _, candidate := range group.Candidates {
			if candidate.Source.SourceRole != group.Role {
				return fmt.Errorf(
					"canonical candidate %q role %q differs from group %q",
					candidate.Source.UnitID,
					candidate.Source.SourceRole,
					group.Role,
				)
			}
			if err := validateCandidateSourceUnit(candidate.Source, snapshot); err != nil {
				return err
			}
			if err := validateMatchGrounds(candidate.MatchGrounds, snapshot); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateQuerySourceUnit(unit SourceUnit, snapshot QuerySourceSnapshot) error {
	if strings.TrimSpace(unit.UnitID) == "" || strings.TrimSpace(string(unit.Role)) == "" {
		return fmt.Errorf("canonical exact source unit identity is incomplete")
	}
	if err := validateQuerySourceProvenance(unit.Provenance, snapshot); err != nil {
		return fmt.Errorf("canonical source unit %s: %w", unit.UnitID, err)
	}
	if unit.Provenance.ContentHash != sourceContentHash(unit.Body) {
		return fmt.Errorf("canonical source unit %s content hash mismatch", unit.UnitID)
	}
	for _, relation := range unit.Relations {
		if err := validateQuerySourceRelation(relation, snapshot); err != nil {
			return fmt.Errorf("canonical source unit %s relation: %w", unit.UnitID, err)
		}
	}
	return nil
}

func validateCandidateSourceUnit(
	unit CandidateSourceUnit,
	snapshot QuerySourceSnapshot,
) error {
	if strings.TrimSpace(unit.UnitID) == "" || strings.TrimSpace(string(unit.SourceRole)) == "" {
		return fmt.Errorf("canonical candidate source identity is incomplete")
	}
	if err := validateQuerySourceProvenance(unit.Provenance, snapshot); err != nil {
		return fmt.Errorf("canonical candidate %s: %w", unit.UnitID, err)
	}
	projection := unit.RelationProjection
	if projection == nil {
		return nil
	}
	for _, relation := range projection.Relations {
		if err := validateQuerySourceRelation(relation, snapshot); err != nil {
			return fmt.Errorf("canonical candidate %s relation: %w", unit.UnitID, err)
		}
	}
	return nil
}

func validateQuerySourceRelation(
	relation SourceRelation,
	snapshot QuerySourceSnapshot,
) error {
	if !isSourceRelationKind(relation.Kind) || strings.TrimSpace(relation.TargetPatternID) == "" {
		return fmt.Errorf("source relation identity is incomplete")
	}
	return validateQuerySourceProvenance(relation.Provenance, snapshot)
}

func validateMatchGrounds(
	grounds []MatchGround,
	snapshot QuerySourceSnapshot,
) error {
	for _, ground := range grounds {
		if strings.TrimSpace(string(ground.Tier)) == "" ||
			strings.TrimSpace(ground.ProbeField) == "" ||
			strings.TrimSpace(ground.SourceField) == "" {
			return fmt.Errorf("canonical candidate match ground is incomplete")
		}
		if ground.Evidence == nil {
			continue
		}
		if err := validateQuerySourceProvenance(ground.Evidence.Provenance, snapshot); err != nil {
			return fmt.Errorf("canonical match-ground evidence: %w", err)
		}
	}
	return nil
}

func validateQuerySourceProvenance(
	provenance SourceProvenance,
	snapshot QuerySourceSnapshot,
) error {
	if strings.TrimSpace(provenance.SourcePath) == "" ||
		provenance.StartLine <= 0 ||
		provenance.EndLine < provenance.StartLine ||
		strings.TrimSpace(provenance.ContentHash) == "" ||
		strings.TrimSpace(provenance.SourceRevision) == "" {
		return fmt.Errorf("source provenance is incomplete")
	}
	if provenance.SourceRevision != snapshot.Revision() {
		return fmt.Errorf(
			"source revision %q differs from snapshot %q",
			provenance.SourceRevision,
			snapshot.Revision(),
		)
	}
	return nil
}
