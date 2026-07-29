package codeintel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/codebase"
	"github.com/m0n0x41d/haft/internal/contextgraph"
)

const (
	exploreContractVersion         = "haft.code_explore.v1"
	exploreTraceRefPrefix          = "code-explore-trace:v1"
	workingExploreCandidateMax     = 5
	workingExploreContextRefMax    = 5
	workingExploreSourceByteMax    = 6000
	workingExplorePayloadByteMax   = 12000
	workingExploreTextByteMax      = 512
	workingExploreCompactHopMax    = 5
	workingExploreCompactReasonMax = 3
	exploreRequestKindSymbol       = "symbol"
	exploreRequestKindConcern      = "concern"
	exploreCanonicalResultExact    = "exact"
	exploreCanonicalResultBag      = "bag"
	exploreCanonicalResultConcern  = "concern"
	exploreReplayMismatchIndex     = "index_snapshot"
	exploreReplayMismatchRequest   = "request"
	exploreReplayMismatchResult    = "result"
	exploreSourceReturnView        = "trace"
	exploreWorkingProjectionReason = "working_projection_budget"
)

// ExploreExecutionRequest is a closed exact-one-of request. Weak strings are
// parsed at construction; downstream execution cannot represent both a symbol
// and a concern query.
type ExploreExecutionRequest struct {
	kind          string
	symbol        string
	file          string
	line          int
	concern       codebase.ConcernQuery
	maxCandidates int
}

func NewExploreExecutionRequest(
	symbol string,
	file string,
	line int,
	rawConcern string,
	maxCandidates int,
) (ExploreExecutionRequest, error) {
	trimmedSymbol := strings.TrimSpace(symbol)
	hasSymbol := trimmedSymbol != ""
	hasConcern := rawConcern != ""
	if hasSymbol == hasConcern {
		return ExploreExecutionRequest{}, fmt.Errorf(
			"explore requires exactly one of symbol or query",
		)
	}
	if line < 0 {
		return ExploreExecutionRequest{}, fmt.Errorf(
			"explore line must be >= 0",
		)
	}
	if hasSymbol {
		return ExploreExecutionRequest{
			kind:   exploreRequestKindSymbol,
			symbol: trimmedSymbol,
			file:   strings.TrimSpace(file),
			line:   line,
		}, nil
	}
	concern, err := codebase.NewConcernQuery(rawConcern)
	if err != nil {
		return ExploreExecutionRequest{}, err
	}
	budget := maxCandidates
	if budget == 0 {
		budget = DefaultConcernCandidateBudget
	}
	if _, err := codebase.NewDiscoveryBudget(budget); err != nil {
		return ExploreExecutionRequest{}, err
	}
	if strings.TrimSpace(file) != "" || line > 0 {
		return ExploreExecutionRequest{}, fmt.Errorf(
			"explore concern query cannot include symbol coordinates",
		)
	}
	return ExploreExecutionRequest{
		kind:          exploreRequestKindConcern,
		concern:       concern,
		maxCandidates: budget,
	}, nil
}

func (request ExploreExecutionRequest) Kind() string {
	return request.kind
}

func (request ExploreExecutionRequest) Symbol() string {
	return request.symbol
}

func (request ExploreExecutionRequest) File() string {
	return request.file
}

func (request ExploreExecutionRequest) Line() int {
	return request.line
}

func (request ExploreExecutionRequest) Concern() string {
	return request.concern.Raw()
}

func (request ExploreExecutionRequest) MaxCandidates() int {
	return request.maxCandidates
}

type ExplorePublicationView string

const (
	ExplorePublicationViewWorking    ExplorePublicationView = "working"
	ExplorePublicationViewTrace      ExplorePublicationView = "trace"
	ExplorePublicationViewDiagnostic ExplorePublicationView = "diagnostic"
)

type ExplorePublicationRequest struct {
	view     ExplorePublicationView
	traceRef string
}

func NewExplorePublicationRequest(
	rawView string,
	rawTraceRef string,
) (ExplorePublicationRequest, error) {
	view := ExplorePublicationView(strings.TrimSpace(rawView))
	if view == "" {
		view = ExplorePublicationViewWorking
	}
	if view != ExplorePublicationViewWorking &&
		view != ExplorePublicationViewTrace &&
		view != ExplorePublicationViewDiagnostic {
		return ExplorePublicationRequest{}, fmt.Errorf(
			"unsupported explore view %q; expected working, trace, or diagnostic",
			rawView,
		)
	}
	traceRef := strings.TrimSpace(rawTraceRef)
	if traceRef != "" && view == ExplorePublicationViewWorking {
		return ExplorePublicationRequest{}, fmt.Errorf(
			"explore trace_ref requires trace or diagnostic view",
		)
	}
	if traceRef != "" {
		if _, err := parseExploreTraceRef(traceRef); err != nil {
			return ExplorePublicationRequest{}, err
		}
	}
	return ExplorePublicationRequest{
		view:     view,
		traceRef: traceRef,
	}, nil
}

func (request ExplorePublicationRequest) View() ExplorePublicationView {
	return request.view
}

func (request ExplorePublicationRequest) TraceRef() string {
	return request.traceRef
}

// ExploreEnvelope is the one full semantic result before presentation. Exactly
// one canonical variant is populated and the index basis is shared by every
// later projection.
type ExploreEnvelope struct {
	request ExploreExecutionRequest
	kind    string
	index   codebase.IndexState
	exact   ExploreResult
	bag     ExploreBagResult
	concern ConcernDiscoveryResult
}

func (envelope ExploreEnvelope) Request() ExploreExecutionRequest {
	return envelope.request
}

func (envelope ExploreEnvelope) Index() codebase.IndexState {
	return envelope.index
}

func (s *Service) ExecuteExplore(
	ctx context.Context,
	projectRoot string,
	request ExploreExecutionRequest,
) (ExploreEnvelope, error) {
	if request.kind == exploreRequestKindConcern {
		result, err := s.DiscoverConcern(
			ctx,
			projectRoot,
			request.concern.Raw(),
			request.maxCandidates,
		)
		if err != nil {
			return ExploreEnvelope{}, err
		}
		return ExploreEnvelope{
			request: request,
			kind:    exploreCanonicalResultConcern,
			index:   result.Index,
			concern: result,
		}, nil
	}
	if request.kind != exploreRequestKindSymbol {
		return ExploreEnvelope{}, fmt.Errorf(
			"explore execution request is invalid",
		)
	}
	seeds := splitExploreSymbolBag(request.symbol)
	if len(seeds) >= 2 {
		result, err := s.ExploreBag(ctx, projectRoot, seeds)
		if err != nil {
			return ExploreEnvelope{}, err
		}
		return ExploreEnvelope{
			request: request,
			kind:    exploreCanonicalResultBag,
			index:   result.Index,
			bag:     result,
		}, nil
	}
	result, err := s.Explore(
		ctx,
		projectRoot,
		request.symbol,
		request.file,
		request.line,
	)
	if err != nil {
		return ExploreEnvelope{}, err
	}
	return ExploreEnvelope{
		request: request,
		kind:    exploreCanonicalResultExact,
		index:   result.Index,
		exact:   result,
	}, nil
}

func splitExploreSymbolBag(raw string) []string {
	normalized := strings.ReplaceAll(raw, ",", " ")
	fields := strings.Fields(normalized)
	seen := make(map[string]bool, len(fields))
	result := make([]string, 0, len(fields))
	for _, field := range fields {
		if seen[field] {
			continue
		}
		seen[field] = true
		result = append(result, field)
	}
	return result
}

type PublishedExploreKind string

const (
	PublishedExploreKindResolved       PublishedExploreKind = "resolved"
	PublishedExploreKindCandidateSet   PublishedExploreKind = "candidate_set"
	PublishedExploreKindDisconnected   PublishedExploreKind = "disconnected"
	PublishedExploreKindUnresolved     PublishedExploreKind = "unresolved"
	PublishedExploreKindIncomplete     PublishedExploreKind = "incomplete"
	PublishedExploreKindUnavailable    PublishedExploreKind = "unavailable"
	PublishedExploreKindReplayMismatch PublishedExploreKind = "replay_mismatch"
)

type ExploreTraceRef string

func (ref ExploreTraceRef) String() string {
	return string(ref)
}

type PublishedExploreResult interface {
	PublicationView() ExplorePublicationView
	PublishedKind() PublishedExploreKind
	TraceReference() ExploreTraceRef
	isPublishedExploreResult()
}

type PublishedExploreJSONStyle string

const (
	PublishedExploreJSONCompact  PublishedExploreJSONStyle = "compact"
	PublishedExploreJSONIndented PublishedExploreJSONStyle = "indented"
)

func EncodePublishedExplore(
	result PublishedExploreResult,
	style PublishedExploreJSONStyle,
) ([]byte, error) {
	if result == nil {
		return nil, fmt.Errorf("published explore result is required")
	}
	if err := validatePublishedExploreResult(result); err != nil {
		return nil, err
	}
	var (
		wire []byte
		err  error
	)
	if style == PublishedExploreJSONCompact {
		wire, err = json.Marshal(result)
	}
	if style == PublishedExploreJSONIndented {
		wire, err = json.MarshalIndent(result, "", "  ")
	}
	if style != PublishedExploreJSONCompact &&
		style != PublishedExploreJSONIndented {
		return nil, fmt.Errorf(
			"unsupported published explore JSON style %q",
			style,
		)
	}
	if err != nil {
		return nil, err
	}
	if result.PublicationView() == ExplorePublicationViewWorking &&
		len(wire) > workingExplorePayloadByteMax {
		return nil, fmt.Errorf(
			"working explore payload uses %d bytes; limit is %d; request trace or diagnostic view",
			len(wire),
			workingExplorePayloadByteMax,
		)
	}
	return wire, nil
}

type publishedExplore struct {
	ContractVersion       string                          `json:"contract_version"`
	View                  ExplorePublicationView          `json:"view"`
	Kind                  PublishedExploreKind            `json:"kind"`
	TraceRef              ExploreTraceRef                 `json:"trace_ref,omitempty"`
	RequestBasis          publishedExploreRequestBasis    `json:"request_basis"`
	IndexCoverage         publishedExploreIndexCoverage   `json:"index_coverage"`
	SeedResolution        *publishedExploreSeedResolution `json:"seed_resolution,omitempty"`
	TraversalOutcome      *publishedExploreTraversal      `json:"traversal_outcome,omitempty"`
	Candidates            []publishedExploreCandidate     `json:"candidates,omitempty"`
	SourceHops            []publishedExploreHop           `json:"source_hops,omitempty"`
	ReasoningContext      []publishedExploreReasoning     `json:"reasoning_context,omitempty"`
	Source                *publishedExploreSource         `json:"source,omitempty"`
	LimitingReason        string                          `json:"limiting_reason,omitempty"`
	ReturnView            string                          `json:"return_view,omitempty"`
	ResolutionDiagnostics any                             `json:"resolution_diagnostics,omitempty"`
	RetrievalDiagnostics  any                             `json:"retrieval_diagnostics,omitempty"`
	TraceBasis            *publishedExploreTraceBasis     `json:"trace_basis,omitempty"`
	ReplayMismatch        *publishedExploreReplayMismatch `json:"replay_mismatch,omitempty"`
}

func (publishedExplore) isPublishedExploreResult() {}

func (result publishedExplore) PublicationView() ExplorePublicationView {
	return result.View
}

func (result publishedExplore) PublishedKind() PublishedExploreKind {
	return result.Kind
}

func (result publishedExplore) TraceReference() ExploreTraceRef {
	return result.TraceRef
}

type publishedExploreRequestBasis struct {
	Kind          string `json:"kind"`
	Symbol        string `json:"symbol,omitempty"`
	File          string `json:"file,omitempty"`
	Line          int    `json:"line,omitempty"`
	Query         string `json:"query,omitempty"`
	MaxCandidates int    `json:"max_candidates,omitempty"`
}

type publishedExploreIndexCoverage struct {
	Epoch                 int64                             `json:"epoch"`
	CoverageRef           string                            `json:"coverage_ref"`
	Posture               string                            `json:"posture"`
	KnownAbsenceSupported bool                              `json:"known_absence_supported"`
	DiscoveredFiles       int64                             `json:"discovered_files"`
	AdmittedFiles         int64                             `json:"admitted_files"`
	IndexedFiles          int64                             `json:"indexed_files"`
	EmptyFiles            int64                             `json:"empty_files"`
	SkippedFiles          int64                             `json:"skipped_files"`
	Degraded              bool                              `json:"degraded"`
	DegradedReason        string                            `json:"degraded_reason,omitempty"`
	Exclusions            []codebase.IndexExclusionSnapshot `json:"exclusions,omitempty"`
}

type publishedExploreSeedResolution struct {
	Kind   string `json:"kind"`
	Detail string `json:"detail,omitempty"`
}

type publishedExploreSymbol struct {
	AnchorID      string `json:"anchor_id"`
	Name          string `json:"name"`
	QualifiedName string `json:"qualified_name,omitempty"`
	Kind          string `json:"symbol_kind"`
	Receiver      string `json:"receiver,omitempty"`
	File          string `json:"file"`
	StartLine     int    `json:"start_line"`
	EndLine       int    `json:"end_line"`
	Language      string `json:"language,omitempty"`
	Epoch         int64  `json:"epoch"`
}

type publishedExploreCandidate struct {
	Rank                 int                      `json:"rank"`
	Symbol               publishedExploreSymbol   `json:"symbol"`
	SourceLane           string                   `json:"source_lane,omitempty"`
	DirectBridge         string                   `json:"direct_bridge,omitempty"`
	OriginLanes          []string                 `json:"origin_lanes,omitempty"`
	ReasoningArtifacts   []ConcernArtifactSupport `json:"reasoning_artifacts,omitempty"`
	Governance           *ConcernGovernance       `json:"governance,omitempty"`
	CallEvidence         *ConcernCallEvidence     `json:"call_evidence,omitempty"`
	RankingIsAdvisory    bool                     `json:"ranking_is_advisory"`
	IdentityAutoSelected bool                     `json:"identity_auto_selected"`
}

type publishedExploreTraversal struct {
	Kind           string `json:"kind"`
	Detail         string `json:"detail,omitempty"`
	Termination    string `json:"termination,omitempty"`
	MaxHops        int64  `json:"max_hops,omitempty"`
	VisitBudget    int64  `json:"visit_budget,omitempty"`
	VisitedNodes   int64  `json:"visited_nodes,omitempty"`
	InspectedEdges int64  `json:"inspected_edges,omitempty"`
	HopDepth       int64  `json:"hop_depth,omitempty"`
	BridgeCount    int64  `json:"bridge_count,omitempty"`
}

type publishedExploreHop struct {
	Symbol     publishedExploreSymbol `json:"symbol"`
	Distance   int                    `json:"distance"`
	ViaKind    codebase.EdgeKind      `json:"via_kind,omitempty"`
	Provenance *codebase.Provenance   `json:"provenance,omitempty"`
}

type publishedExploreArtifactRef struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Kind  string `json:"kind"`
}

type publishedExploreSpecRef struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	Resolution    string `json:"resolution"`
	BaselineState string `json:"baseline_state"`
}

type publishedExploreInvariant struct {
	Text        string `json:"text"`
	DecisionRef string `json:"decision_ref,omitempty"`
	ContextOnly bool   `json:"context_only"`
}

type publishedExploreReasoning struct {
	SymbolAnchor                    string                        `json:"symbol_anchor"`
	Granularity                     string                        `json:"granularity,omitempty"`
	Decisions                       []publishedExploreArtifactRef `json:"decisions,omitempty"`
	ExactBindingDecisionRefs        []string                      `json:"exact_binding_decision_refs"`
	AffectedPathContextDecisionRefs []string                      `json:"affected_path_context_decision_refs"`
	ModuleDecisionRefs              []string                      `json:"module_decision_refs"`
	Problems                        []publishedExploreArtifactRef `json:"problems,omitempty"`
	Alternatives                    []publishedExploreArtifactRef `json:"alternatives,omitempty"`
	Notes                           []publishedExploreArtifactRef `json:"notes,omitempty"`
	Specs                           []publishedExploreSpecRef     `json:"specs,omitempty"`
	Invariants                      []publishedExploreInvariant   `json:"invariants,omitempty"`
}

type publishedExploreSource struct {
	Available      bool   `json:"available"`
	Content        string `json:"content,omitempty"`
	OriginalBytes  int    `json:"original_bytes"`
	IncludedBytes  int    `json:"included_bytes"`
	Truncated      bool   `json:"truncated"`
	TruncationRule string `json:"truncation_rule,omitempty"`
}

type publishedExploreTraceBasis struct {
	Schema             string `json:"schema"`
	IndexDigest        string `json:"index_digest"`
	RequestDigest      string `json:"request_digest"`
	ResultDigest       string `json:"result_digest"`
	ConcernGraphRef    string `json:"concern_graph_ref,omitempty"`
	ConcernGraphDigest string `json:"concern_graph_digest,omitempty"`
}

type publishedExploreReplayMismatch struct {
	Mismatch string `json:"mismatch"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
}

type parsedExploreTraceRef struct {
	indexDigest   string
	requestDigest string
	resultDigest  string
}

func PublishExplore(
	ctx context.Context,
	service *Service,
	projectRoot string,
	request ExploreExecutionRequest,
	publication ExplorePublicationRequest,
) (PublishedExploreResult, error) {
	if service == nil {
		return nil, fmt.Errorf("explore service is required")
	}
	envelope, err := service.ExecuteExplore(ctx, projectRoot, request)
	if err != nil {
		return nil, err
	}
	return ProjectExplore(envelope, publication)
}

func ProjectExplore(
	envelope ExploreEnvelope,
	publication ExplorePublicationRequest,
) (PublishedExploreResult, error) {
	requestBasis := projectExploreRequestBasis(envelope.request)
	indexCoverage := projectExploreIndexCoverage(envelope.index, false)
	traceBasis, err := buildExploreTraceBasis(envelope, requestBasis)
	if err != nil {
		return nil, err
	}
	traceRef := buildExploreTraceRef(traceBasis)
	if publication.traceRef != "" {
		expected, err := parseExploreTraceRef(publication.traceRef)
		if err != nil {
			return nil, err
		}
		mismatch := compareExploreTrace(expected, traceBasis)
		if mismatch != nil {
			return publishedExplore{
				ContractVersion: exploreContractVersion,
				View:            publication.view,
				Kind:            PublishedExploreKindReplayMismatch,
				RequestBasis:    requestBasis,
				IndexCoverage:   indexCoverage,
				ReplayMismatch:  mismatch,
				TraceBasis:      &traceBasis,
			}, nil
		}
	}
	result, err := projectExploreWorking(
		envelope,
		requestBasis,
		indexCoverage,
		traceRef,
	)
	if err != nil {
		return nil, err
	}
	result.View = publication.view
	if publication.view == ExplorePublicationViewWorking {
		if err := fitWorkingExplorePayload(&result); err != nil {
			return nil, err
		}
	}
	if publication.view == ExplorePublicationViewTrace {
		projectExploreTrace(envelope, &result)
		result.TraceBasis = &traceBasis
	}
	if publication.view == ExplorePublicationViewDiagnostic {
		projectExploreTrace(envelope, &result)
		result.TraceBasis = &traceBasis
		projectExploreDiagnostics(envelope, &result)
	}
	if err := validatePublishedExploreResult(result); err != nil {
		return nil, err
	}
	return result, nil
}

func projectExploreWorking(
	envelope ExploreEnvelope,
	requestBasis publishedExploreRequestBasis,
	indexCoverage publishedExploreIndexCoverage,
	traceRef ExploreTraceRef,
) (publishedExplore, error) {
	result := publishedExplore{
		ContractVersion: exploreContractVersion,
		View:            ExplorePublicationViewWorking,
		TraceRef:        traceRef,
		RequestBasis:    requestBasis,
		IndexCoverage:   indexCoverage,
	}
	if envelope.kind == exploreCanonicalResultExact {
		projectExactExplore(envelope.exact, &result)
		return result, nil
	}
	if envelope.kind == exploreCanonicalResultBag {
		projectBagExplore(envelope.bag, &result)
		return result, nil
	}
	if envelope.kind == exploreCanonicalResultConcern {
		projectConcernExplore(envelope.concern, &result)
		return result, nil
	}
	return publishedExplore{}, fmt.Errorf(
		"canonical explore envelope has unsupported result %q",
		envelope.kind,
	)
}

func projectExactExplore(
	exact ExploreResult,
	result *publishedExplore,
) {
	kind := ""
	detail := ""
	if exact.SeedResolution != nil {
		kind = exact.SeedResolution.Kind().String()
		detail = exact.SeedResolution.DetailCode()
	}
	result.SeedResolution = &publishedExploreSeedResolution{
		Kind:   kind,
		Detail: detail,
	}
	switch kind {
	case "resolved_seed":
		result.Kind = PublishedExploreKindResolved
		result.TraversalOutcome = projectChainTraversal(exact.ChainOutcome)
		result.SourceHops = projectExploreHops(exact.Chain, false)
		result.ReasoningContext = projectChainReasoning(exact.Chain)
		result.Source = projectExploreSource(
			exact.SeedBody,
			exact.SeedBodyOK,
		)
		if result.Source.Truncated {
			result.LimitingReason = exploreWorkingProjectionReason
			result.ReturnView = exploreSourceReturnView
		}
	case "candidate_set":
		result.Kind = PublishedExploreKindCandidateSet
		result.Candidates = projectExactCandidates(exact.Candidates)
	case "seed_not_found":
		if exact.Index.SupportsKnownAbsence() {
			result.Kind = PublishedExploreKindUnresolved
		} else {
			result.Kind = PublishedExploreKindIncomplete
			result.LimitingReason = exact.Index.Basis.Coverage.Posture
			result.ReturnView = exploreSourceReturnView
		}
	case "seed_unavailable":
		result.Kind = PublishedExploreKindUnavailable
		result.LimitingReason = detail
	default:
		result.Kind = PublishedExploreKindUnavailable
		result.LimitingReason = "invalid_seed_resolution"
	}
}

func projectBagExplore(
	bag ExploreBagResult,
	result *publishedExplore,
) {
	result.SeedResolution = &publishedExploreSeedResolution{
		Kind:   "bag",
		Detail: bagSeedResolutionDetail(bag),
	}
	result.Candidates = projectExactCandidates(bag.Seeds)
	result.SourceHops = projectBagHops(bag, false)
	result.ReasoningContext = projectBagReasoning(bag)
	kind, reason := classifyBagPublishedKind(bag)
	result.Kind = kind
	result.LimitingReason = reason
	if kind == PublishedExploreKindIncomplete ||
		kind == PublishedExploreKindUnavailable {
		result.ReturnView = exploreSourceReturnView
	}
}

func projectConcernExplore(
	concern ConcernDiscoveryResult,
	result *publishedExplore,
) {
	result.SeedResolution = &publishedExploreSeedResolution{
		Kind:   concern.Outcome.String(),
		Detail: concern.Outcome.DetailCode(),
	}
	result.Candidates = projectConcernWorkingCandidates(concern.Candidates())
	switch concern.Outcome.String() {
	case ConcernCandidates, ConcernResolvedExactIdentity:
		result.Kind = PublishedExploreKindCandidateSet
	case ConcernNoCandidates:
		result.Kind = PublishedExploreKindUnresolved
	case ConcernIncompleteBasis:
		result.Kind = PublishedExploreKindIncomplete
		result.LimitingReason = concern.Outcome.DetailCode()
		result.ReturnView = exploreSourceReturnView
	case ConcernIndexUnavailable:
		result.Kind = PublishedExploreKindUnavailable
		result.LimitingReason = concern.Outcome.DetailCode()
		result.ReturnView = exploreSourceReturnView
	default:
		result.Kind = PublishedExploreKindUnavailable
		result.LimitingReason = "invalid_concern_outcome"
	}
}

func projectExploreTrace(
	envelope ExploreEnvelope,
	result *publishedExplore,
) {
	result.IndexCoverage = projectExploreIndexCoverage(envelope.index, true)
	if envelope.kind == exploreCanonicalResultExact {
		result.SourceHops = projectExploreHops(envelope.exact.Chain, true)
		return
	}
	if envelope.kind == exploreCanonicalResultBag {
		result.SourceHops = projectBagHops(envelope.bag, true)
		return
	}
	if envelope.kind == exploreCanonicalResultConcern {
		result.Candidates = projectConcernTraceCandidates(
			envelope.concern.Candidates(),
		)
	}
}

func projectExploreDiagnostics(
	envelope ExploreEnvelope,
	result *publishedExplore,
) {
	if envelope.kind == exploreCanonicalResultExact {
		result.ResolutionDiagnostics = map[string]any{
			"seed_resolution":   envelope.exact.SeedResolution,
			"resolution_counts": envelope.exact.Resolution,
		}
		retrieval := map[string]any{
			"blast_radius": envelope.exact.BlastRadius,
		}
		if envelope.exact.SeedResolution != nil &&
			envelope.exact.SeedResolution.Kind().String() ==
				"resolved_seed" {
			retrieval["chain_outcome"] = envelope.exact.ChainOutcome
		}
		result.RetrievalDiagnostics = retrieval
		return
	}
	if envelope.kind == exploreCanonicalResultBag {
		result.ResolutionDiagnostics = map[string]any{
			"seed_resolutions":  envelope.bag.SeedResolutions,
			"unresolved":        envelope.bag.Unresolved,
			"resolution_counts": envelope.bag.Resolution,
		}
		result.RetrievalDiagnostics = map[string]any{
			"legs": envelope.bag.Legs,
		}
		return
	}
	result.ResolutionDiagnostics = map[string]any{
		"outcome":       envelope.concern.Outcome.String(),
		"detail":        envelope.concern.Outcome.DetailCode(),
		"lexical_batch": envelope.concern.Batch,
	}
	result.RetrievalDiagnostics = map[string]any{
		"fusion_basis":     envelope.concern.Basis,
		"fused_candidates": envelope.concern.Candidates(),
	}
}

func projectExploreRequestBasis(
	request ExploreExecutionRequest,
) publishedExploreRequestBasis {
	if request.kind == exploreRequestKindConcern {
		return publishedExploreRequestBasis{
			Kind:          request.kind,
			Query:         request.concern.Raw(),
			MaxCandidates: request.maxCandidates,
		}
	}
	return publishedExploreRequestBasis{
		Kind:   request.kind,
		Symbol: request.symbol,
		File:   request.file,
		Line:   request.line,
	}
}

func projectExploreIndexCoverage(
	index codebase.IndexState,
	includeExclusions bool,
) publishedExploreIndexCoverage {
	coverage := index.Basis.Coverage
	result := publishedExploreIndexCoverage{
		Epoch:                 index.Epoch,
		CoverageRef:           index.Basis.CoverageRef(),
		Posture:               coverage.Posture,
		KnownAbsenceSupported: index.SupportsKnownAbsence(),
		DiscoveredFiles:       coverage.DiscoveredFiles,
		AdmittedFiles:         coverage.AdmittedFiles,
		IndexedFiles:          coverage.IndexedFiles,
		EmptyFiles:            coverage.EmptyFiles,
		SkippedFiles:          coverage.SkippedFiles,
		Degraded:              index.Degraded,
		DegradedReason:        index.DegradedReason,
	}
	if includeExclusions {
		result.Exclusions = slices.Clone(index.Basis.Exclusions)
	}
	return result
}

func projectPublishedSymbol(
	symbol codebase.CodeSymbol,
) publishedExploreSymbol {
	return publishedExploreSymbol{
		AnchorID:      symbol.AnchorID,
		Name:          symbol.Name,
		QualifiedName: symbol.QualifiedName,
		Kind:          symbol.Kind,
		Receiver:      symbol.Receiver,
		File:          symbol.FilePath,
		StartLine:     symbol.StartLine,
		EndLine:       symbol.EndLine,
		Language:      symbol.Lang,
		Epoch:         symbol.IndexEpoch,
	}
}

func projectExactCandidates(
	candidates []codebase.CodeSymbol,
) []publishedExploreCandidate {
	limit := min(len(candidates), workingExploreCandidateMax)
	result := make([]publishedExploreCandidate, 0, limit)
	for index, candidate := range candidates[:limit] {
		result = append(result, publishedExploreCandidate{
			Rank:                 index + 1,
			Symbol:               projectPublishedSymbol(candidate),
			RankingIsAdvisory:    true,
			IdentityAutoSelected: false,
		})
	}
	return result
}

func projectConcernWorkingCandidates(
	candidates []ConcernCandidate,
) []publishedExploreCandidate {
	limit := min(len(candidates), workingExploreCandidateMax)
	result := make([]publishedExploreCandidate, 0, limit)
	for index, candidate := range candidates[:limit] {
		result = append(result, publishedExploreCandidate{
			Rank:                 index + 1,
			Symbol:               projectPublishedSymbol(candidate.Symbol()),
			RankingIsAdvisory:    true,
			IdentityAutoSelected: false,
		})
	}
	return result
}

func projectConcernTraceCandidates(
	candidates []ConcernCandidate,
) []publishedExploreCandidate {
	limit := min(len(candidates), workingExploreCandidateMax)
	result := make([]publishedExploreCandidate, 0, limit)
	for index, candidate := range candidates[:limit] {
		artifacts := candidate.Artifacts()
		artifacts = artifacts[:min(
			len(artifacts),
			workingExploreContextRefMax,
		)]
		projected := publishedExploreCandidate{
			Rank:               index + 1,
			Symbol:             projectPublishedSymbol(candidate.Symbol()),
			SourceLane:         candidate.SourceLane(),
			DirectBridge:       candidate.DirectBridge(),
			OriginLanes:        candidate.OriginLanes(),
			ReasoningArtifacts: artifacts,
			Governance: pointerTo(
				boundConcernGovernance(candidate.Governance()),
			),
			CallEvidence: pointerTo(
				boundConcernCallEvidence(candidate.Calls()),
			),
			RankingIsAdvisory:    true,
			IdentityAutoSelected: false,
		}
		result = append(result, projected)
	}
	return result
}

func pointerTo[T any](value T) *T {
	return &value
}

func boundConcernGovernance(
	governance ConcernGovernance,
) ConcernGovernance {
	return ConcernGovernance{
		Decisions: truncateSlice(
			governance.Decisions,
			workingExploreContextRefMax,
		),
		ExactBindingDecisionRefs: truncateSlice(
			governance.ExactBindingDecisionRefs,
			workingExploreContextRefMax,
		),
		AffectedPathContextDecisionRefs: truncateSlice(
			governance.AffectedPathContextDecisionRefs,
			workingExploreContextRefMax,
		),
		Problems: truncateSlice(
			governance.Problems,
			workingExploreContextRefMax,
		),
		Alternatives: truncateSlice(
			governance.Alternatives,
			workingExploreContextRefMax,
		),
		Notes: truncateSlice(
			governance.Notes,
			workingExploreContextRefMax,
		),
		Specs: truncateSlice(
			governance.Specs,
			workingExploreContextRefMax,
		),
		Invariants: truncateSlice(
			governance.Invariants,
			workingExploreContextRefMax,
		),
		ModuleDecisionRefs: truncateSlice(
			governance.ModuleDecisionRefs,
			workingExploreContextRefMax,
		),
		SymbolGranularity: governance.SymbolGranularity,
	}
}

func boundConcernCallEvidence(
	evidence ConcernCallEvidence,
) ConcernCallEvidence {
	return ConcernCallEvidence{
		Incoming: truncateSlice(
			evidence.Incoming,
			workingExploreContextRefMax,
		),
		Outgoing: truncateSlice(
			evidence.Outgoing,
			workingExploreContextRefMax,
		),
		OutgoingCoverage: evidence.OutgoingCoverage,
	}
}

func truncateSlice[T any](values []T, limit int) []T {
	count := min(len(values), limit)
	return slices.Clone(values[:count])
}

func projectChainTraversal(
	outcome ChainOutcome,
) *publishedExploreTraversal {
	stats := outcome.Stats()
	budget := stats.Budget()
	return &publishedExploreTraversal{
		Kind:           "chain",
		Termination:    outcome.Termination().String(),
		MaxHops:        budget.MaxHops(),
		VisitBudget:    budget.VisitBudget(),
		VisitedNodes:   stats.VisitedNodes(),
		InspectedEdges: stats.InspectedEdges(),
		HopDepth:       stats.HopDepth(),
		BridgeCount:    stats.BridgeCount(),
	}
}

func projectExploreHops(
	chain []ChainStep,
	includeProvenance bool,
) []publishedExploreHop {
	result := make([]publishedExploreHop, 0, len(chain))
	for _, step := range chain {
		projected := publishedExploreHop{
			Symbol:   projectPublishedSymbol(step.Symbol),
			Distance: step.Distance,
			ViaKind:  step.ViaKind,
		}
		if includeProvenance {
			projected.Provenance = pointerTo(step.Provenance)
		}
		result = append(result, projected)
	}
	return result
}

func projectBagHops(
	bag ExploreBagResult,
	includeProvenance bool,
) []publishedExploreHop {
	result := make([]publishedExploreHop, 0)
	seen := make(map[string]bool)
	for _, leg := range bag.Legs {
		for _, step := range leg.Steps {
			anchor := step.Symbol.AnchorID
			if anchor == "" {
				anchor = step.Symbol.ID
			}
			if seen[anchor] {
				continue
			}
			seen[anchor] = true
			projected := publishedExploreHop{
				Symbol:   projectPublishedSymbol(step.Symbol),
				Distance: step.Distance,
				ViaKind:  step.ViaKind,
			}
			if includeProvenance {
				projected.Provenance = pointerTo(step.Provenance)
			}
			result = append(result, projected)
		}
	}
	return result
}

func projectChainReasoning(
	chain []ChainStep,
) []publishedExploreReasoning {
	result := make([]publishedExploreReasoning, 0, len(chain))
	for _, step := range chain {
		projected := projectCodeContext(step.Context)
		if projected.SymbolAnchor == "" {
			projected.SymbolAnchor = step.Symbol.AnchorID
		}
		result = append(result, projected)
	}
	return result
}

func projectBagReasoning(
	bag ExploreBagResult,
) []publishedExploreReasoning {
	result := make([]publishedExploreReasoning, 0)
	seen := make(map[string]bool)
	for _, leg := range bag.Legs {
		for _, step := range leg.Steps {
			anchor := step.Symbol.AnchorID
			if seen[anchor] {
				continue
			}
			seen[anchor] = true
			projected := projectCodeContext(step.Context)
			if projected.SymbolAnchor == "" {
				projected.SymbolAnchor = anchor
			}
			result = append(result, projected)
		}
	}
	return result
}

func projectCodeContext(
	codeContext contextgraph.CodeContext,
) publishedExploreReasoning {
	result := publishedExploreReasoning{
		SymbolAnchor:       codeContext.Target.AnchorID,
		Granularity:        codeContext.SymbolGranularity,
		ModuleDecisionRefs: make([]string, 0),
	}
	result.Decisions = projectArtifactRefs(codeContext.Decisions)
	result.ExactBindingDecisionRefs = projectArtifactIDs(
		codeContext.ExactBindingDecisions,
	)
	result.AffectedPathContextDecisionRefs = projectArtifactIDs(
		codeContext.AffectedPathContextDecisions,
	)
	for _, decision := range codeContext.ModuleDecisions {
		result.ModuleDecisionRefs = append(
			result.ModuleDecisionRefs,
			decision.ID,
		)
	}
	result.Problems = projectArtifactRefs(codeContext.Problems)
	result.Alternatives = projectArtifactRefs(codeContext.Portfolios)
	result.Notes = projectArtifactRefs(codeContext.Notes)
	for _, spec := range codeContext.Specs[:min(
		len(codeContext.Specs),
		workingExploreContextRefMax,
	)] {
		result.Specs = append(result.Specs, publishedExploreSpecRef{
			ID:            spec.ID,
			Title:         spec.Title,
			Resolution:    string(spec.Resolution),
			BaselineState: string(spec.BaselineState),
		})
	}
	for _, invariant := range codeContext.Invariants[:min(
		len(codeContext.Invariants),
		workingExploreContextRefMax,
	)] {
		result.Invariants = append(
			result.Invariants,
			publishedExploreInvariant{
				Text:        invariant.Text,
				DecisionRef: invariant.DecisionID,
			},
		)
	}
	for _, invariant := range codeContext.ContextInvariants[:min(
		len(codeContext.ContextInvariants),
		workingExploreContextRefMax,
	)] {
		result.Invariants = append(
			result.Invariants,
			publishedExploreInvariant{
				Text:        invariant.Text,
				DecisionRef: invariant.DecisionID,
				ContextOnly: true,
			},
		)
	}
	return result
}

func projectArtifactIDs(items []*artifact.Artifact) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		result = append(result, item.Meta.ID)
	}
	return stableUniqueStrings(result)
}

func projectArtifactRefs(
	artifacts []*artifact.Artifact,
) []publishedExploreArtifactRef {
	limit := min(len(artifacts), workingExploreContextRefMax)
	result := make([]publishedExploreArtifactRef, 0, limit)
	for _, item := range artifacts[:limit] {
		if item == nil {
			continue
		}
		result = append(result, publishedExploreArtifactRef{
			ID:    item.Meta.ID,
			Title: item.Meta.Title,
			Kind:  string(item.Meta.Kind),
		})
	}
	return result
}

func projectExploreSource(
	source string,
	available bool,
) *publishedExploreSource {
	if !available {
		return &publishedExploreSource{Available: false}
	}
	content, truncated := truncateUTF8Bytes(
		source,
		workingExploreSourceByteMax,
	)
	result := &publishedExploreSource{
		Available:     true,
		Content:       content,
		OriginalBytes: len(source),
		IncludedBytes: len(content),
		Truncated:     truncated,
	}
	if truncated {
		result.TruncationRule = fmt.Sprintf(
			"utf8_prefix_max_%d_bytes",
			workingExploreSourceByteMax,
		)
	}
	return result
}

func truncateUTF8Bytes(value string, limit int) (string, bool) {
	if len(value) <= limit {
		return value, false
	}
	end := limit
	for end > 0 && !utf8.RuneStart(value[end]) {
		end--
	}
	return value[:end], true
}

func fitWorkingExplorePayload(
	result *publishedExplore,
) error {
	wire, err := json.Marshal(*result)
	if err != nil {
		return err
	}
	if len(wire) <= workingExplorePayloadByteMax {
		return nil
	}
	result.LimitingReason = exploreWorkingProjectionReason
	result.ReturnView = exploreSourceReturnView
	sourceContent := detachWorkingExploreSource(result)
	wire, err = json.Marshal(*result)
	if err != nil {
		return err
	}
	if len(wire) > workingExplorePayloadByteMax {
		compactWorkingExploreContext(result)
		wire, err = json.Marshal(*result)
		if err != nil {
			return err
		}
	}
	if len(wire) > workingExplorePayloadByteMax {
		minimizeWorkingExploreContext(result)
		wire, err = json.Marshal(*result)
		if err != nil {
			return err
		}
	}
	if len(wire) > workingExplorePayloadByteMax {
		return fmt.Errorf(
			"working explore semantic basis uses %d bytes; limit is %d; narrow the request or use trace view",
			len(wire),
			workingExplorePayloadByteMax,
		)
	}
	if sourceContent == "" {
		return nil
	}
	fitWorkingExploreSource(result, sourceContent)
	return nil
}

func detachWorkingExploreSource(
	result *publishedExplore,
) string {
	if result.Source == nil || !result.Source.Available {
		return ""
	}
	content := result.Source.Content
	result.Source.Content = ""
	result.Source.IncludedBytes = 0
	result.Source.Truncated = true
	result.Source.TruncationRule = fmt.Sprintf(
		"utf8_prefix_fit_%d_byte_payload",
		workingExplorePayloadByteMax,
	)
	return content
}

func fitWorkingExploreSource(
	result *publishedExplore,
	content string,
) {
	lower := 0
	upper := len(content)
	best := ""
	for lower <= upper {
		middle := lower + (upper-lower)/2
		candidate, _ := truncateUTF8Bytes(content, middle)
		result.Source.Content = candidate
		result.Source.IncludedBytes = len(candidate)
		wire, err := json.Marshal(*result)
		fits := err == nil && len(wire) <= workingExplorePayloadByteMax
		if fits {
			best = candidate
			lower = middle + 1
			continue
		}
		upper = middle - 1
	}
	result.Source.Content = best
	result.Source.IncludedBytes = len(best)
}

func compactWorkingExploreContext(
	result *publishedExplore,
) {
	result.Candidates = compactWorkingCandidates(
		result.Candidates,
		workingExploreCandidateMax,
		workingExploreTextByteMax,
	)
	result.SourceHops = compactWorkingHops(
		result.SourceHops,
		workingExploreCompactHopMax,
		workingExploreTextByteMax,
	)
	result.ReasoningContext = compactWorkingReasoning(
		result.ReasoningContext,
		workingExploreCompactReasonMax,
		workingExploreContextRefMax,
		workingExploreTextByteMax,
	)
	result.IndexCoverage.DegradedReason = boundExploreText(
		result.IndexCoverage.DegradedReason,
		workingExploreTextByteMax,
	)
}

func minimizeWorkingExploreContext(
	result *publishedExplore,
) {
	result.Candidates = compactWorkingCandidates(
		result.Candidates,
		1,
		workingExploreTextByteMax/2,
	)
	result.SourceHops = compactWorkingHops(
		result.SourceHops,
		2,
		workingExploreTextByteMax/2,
	)
	result.ReasoningContext = compactWorkingReasoning(
		result.ReasoningContext,
		1,
		1,
		workingExploreTextByteMax/2,
	)
}

func compactWorkingCandidates(
	candidates []publishedExploreCandidate,
	limit int,
	textLimit int,
) []publishedExploreCandidate {
	count := min(len(candidates), limit)
	result := slices.Clone(candidates[:count])
	for index := range result {
		result[index].Symbol = compactWorkingSymbol(
			result[index].Symbol,
			textLimit,
		)
	}
	return result
}

func compactWorkingHops(
	hops []publishedExploreHop,
	limit int,
	textLimit int,
) []publishedExploreHop {
	selected := selectWorkingHops(hops, limit)
	for index := range selected {
		selected[index].Symbol = compactWorkingSymbol(
			selected[index].Symbol,
			textLimit,
		)
	}
	return selected
}

func selectWorkingHops(
	hops []publishedExploreHop,
	limit int,
) []publishedExploreHop {
	if len(hops) <= limit {
		return slices.Clone(hops)
	}
	if limit <= 1 {
		return slices.Clone(hops[:limit])
	}
	headCount := limit - 1
	result := slices.Clone(hops[:headCount])
	result = append(result, hops[len(hops)-1])
	return result
}

func compactWorkingReasoning(
	contexts []publishedExploreReasoning,
	contextLimit int,
	refLimit int,
	textLimit int,
) []publishedExploreReasoning {
	count := min(len(contexts), contextLimit)
	result := slices.Clone(contexts[:count])
	for index := range result {
		result[index] = compactWorkingReasoningItem(
			result[index],
			refLimit,
			textLimit,
		)
	}
	return result
}

func compactWorkingReasoningItem(
	item publishedExploreReasoning,
	refLimit int,
	textLimit int,
) publishedExploreReasoning {
	item.SymbolAnchor = boundExploreText(item.SymbolAnchor, textLimit)
	item.Granularity = boundExploreText(item.Granularity, textLimit)
	item.Decisions = compactWorkingArtifactRefs(
		item.Decisions,
		refLimit,
		textLimit,
	)
	item.ExactBindingDecisionRefs = truncateSlice(
		item.ExactBindingDecisionRefs,
		refLimit,
	)
	item.AffectedPathContextDecisionRefs = truncateSlice(
		item.AffectedPathContextDecisionRefs,
		refLimit,
	)
	item.ModuleDecisionRefs = truncateSlice(
		item.ModuleDecisionRefs,
		refLimit,
	)
	item.Problems = compactWorkingArtifactRefs(
		item.Problems,
		refLimit,
		textLimit,
	)
	item.Alternatives = compactWorkingArtifactRefs(
		item.Alternatives,
		refLimit,
		textLimit,
	)
	item.Notes = compactWorkingArtifactRefs(
		item.Notes,
		refLimit,
		textLimit,
	)
	item.Specs = compactWorkingSpecRefs(
		item.Specs,
		refLimit,
		textLimit,
	)
	item.Invariants = compactWorkingInvariants(
		item.Invariants,
		refLimit,
		textLimit,
	)
	return item
}

func compactWorkingArtifactRefs(
	refs []publishedExploreArtifactRef,
	limit int,
	textLimit int,
) []publishedExploreArtifactRef {
	count := min(len(refs), limit)
	result := slices.Clone(refs[:count])
	for index := range result {
		result[index].ID = boundExploreText(result[index].ID, textLimit)
		result[index].Title = boundExploreText(
			result[index].Title,
			textLimit,
		)
		result[index].Kind = boundExploreText(
			result[index].Kind,
			textLimit,
		)
	}
	return result
}

func compactWorkingSpecRefs(
	refs []publishedExploreSpecRef,
	limit int,
	textLimit int,
) []publishedExploreSpecRef {
	count := min(len(refs), limit)
	result := slices.Clone(refs[:count])
	for index := range result {
		result[index].ID = boundExploreText(result[index].ID, textLimit)
		result[index].Title = boundExploreText(
			result[index].Title,
			textLimit,
		)
		result[index].Resolution = boundExploreText(
			result[index].Resolution,
			textLimit,
		)
		result[index].BaselineState = boundExploreText(
			result[index].BaselineState,
			textLimit,
		)
	}
	return result
}

func compactWorkingInvariants(
	invariants []publishedExploreInvariant,
	limit int,
	textLimit int,
) []publishedExploreInvariant {
	count := min(len(invariants), limit)
	result := slices.Clone(invariants[:count])
	for index := range result {
		result[index].Text = boundExploreText(
			result[index].Text,
			textLimit,
		)
		result[index].DecisionRef = boundExploreText(
			result[index].DecisionRef,
			textLimit,
		)
	}
	return result
}

func compactWorkingSymbol(
	symbol publishedExploreSymbol,
	textLimit int,
) publishedExploreSymbol {
	symbol.AnchorID = boundExploreText(symbol.AnchorID, textLimit)
	symbol.Name = boundExploreText(symbol.Name, textLimit)
	symbol.QualifiedName = boundExploreText(
		symbol.QualifiedName,
		textLimit,
	)
	symbol.Kind = boundExploreText(symbol.Kind, textLimit)
	symbol.Receiver = boundExploreText(symbol.Receiver, textLimit)
	symbol.File = boundExploreText(symbol.File, textLimit)
	symbol.Language = boundExploreText(symbol.Language, textLimit)
	return symbol
}

func boundExploreText(
	value string,
	limit int,
) string {
	bounded, _ := truncateUTF8Bytes(value, limit)
	return bounded
}

func bagSeedResolutionDetail(bag ExploreBagResult) string {
	if len(bag.Unresolved) > 0 {
		return "some_seeds_unresolved"
	}
	return "all_seeds_resolved"
}

func classifyBagPublishedKind(
	bag ExploreBagResult,
) (PublishedExploreKind, string) {
	if len(bag.Seeds) < 2 {
		if bag.Index.SupportsKnownAbsence() {
			return PublishedExploreKindUnresolved, "fewer_than_two_resolved_seeds"
		}
		return PublishedExploreKindIncomplete, bag.Index.Basis.Coverage.Posture
	}
	hasPath := false
	hasIncomplete := false
	hasUnavailable := false
	for _, leg := range bag.Legs {
		if leg.Forward.Kind().String() == "path_found" ||
			leg.Reverse.Kind().String() == "path_found" {
			hasPath = true
		}
		for _, outcome := range []PathOutcome{leg.Forward, leg.Reverse} {
			if outcome == nil {
				hasUnavailable = true
				continue
			}
			if outcome.Kind().String() == "path_truncated" {
				hasIncomplete = true
			}
			if outcome.Kind().String() == "path_unavailable" {
				hasUnavailable = true
			}
		}
	}
	if hasUnavailable {
		return PublishedExploreKindUnavailable, "path_capability_unavailable"
	}
	if hasIncomplete {
		return PublishedExploreKindIncomplete, "path_search_budget_reached"
	}
	if hasPath {
		return PublishedExploreKindResolved, ""
	}
	return PublishedExploreKindDisconnected, "no_static_path_within_indexed_graph"
}

func buildExploreTraceBasis(
	envelope ExploreEnvelope,
	request publishedExploreRequestBasis,
) (publishedExploreTraceBasis, error) {
	indexCoverage := projectExploreIndexCoverage(envelope.index, true)
	indexDigest, err := digestJSON(indexCoverage)
	if err != nil {
		return publishedExploreTraceBasis{}, err
	}
	requestDigest, err := digestJSON(request)
	if err != nil {
		return publishedExploreTraceBasis{}, err
	}
	resultDigest, err := digestExploreEnvelope(envelope)
	if err != nil {
		return publishedExploreTraceBasis{}, err
	}
	basis := publishedExploreTraceBasis{
		Schema:        "code-explore-trace-basis.v1",
		IndexDigest:   indexDigest,
		RequestDigest: requestDigest,
		ResultDigest:  resultDigest,
	}
	if envelope.kind == exploreCanonicalResultConcern {
		basis.ConcernGraphRef = envelope.concern.Basis.ReplayRef
		basis.ConcernGraphDigest = envelope.concern.Basis.GraphDigest
	}
	return basis, nil
}

func digestExploreEnvelope(
	envelope ExploreEnvelope,
) (string, error) {
	if envelope.kind == exploreCanonicalResultExact {
		payload := map[string]any{
			"kind":            envelope.kind,
			"seed_resolution": envelope.exact.SeedResolution,
			"candidates":      envelope.exact.Candidates,
			"chain":           envelope.exact.Chain,
			"chain_outcome":   envelope.exact.ChainOutcome,
			"blast_radius":    envelope.exact.BlastRadius,
			"seed_body":       envelope.exact.SeedBody,
			"seed_body_ok":    envelope.exact.SeedBodyOK,
			"resolution":      envelope.exact.Resolution,
		}
		if envelope.exact.SeedResolution == nil ||
			envelope.exact.SeedResolution.Kind().String() != "resolved_seed" {
			delete(payload, "chain_outcome")
		}
		return digestJSON(payload)
	}
	if envelope.kind == exploreCanonicalResultBag {
		return digestJSON(map[string]any{
			"kind":             envelope.kind,
			"seed_resolutions": envelope.bag.SeedResolutions,
			"unresolved":       envelope.bag.Unresolved,
			"legs":             envelope.bag.Legs,
			"resolution":       envelope.bag.Resolution,
		})
	}
	if envelope.kind == exploreCanonicalResultConcern {
		return digestJSON(map[string]any{
			"kind":       envelope.kind,
			"outcome":    envelope.concern.Outcome.String(),
			"detail":     envelope.concern.Outcome.DetailCode(),
			"candidates": envelope.concern.Candidates(),
			"basis":      envelope.concern.Basis,
		})
	}
	return "", fmt.Errorf(
		"cannot digest unsupported explore result %q",
		envelope.kind,
	)
}

func digestJSON(value any) (string, error) {
	wire, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(wire)
	return hex.EncodeToString(sum[:]), nil
}

func buildExploreTraceRef(
	basis publishedExploreTraceBasis,
) ExploreTraceRef {
	return ExploreTraceRef(strings.Join([]string{
		exploreTraceRefPrefix,
		basis.IndexDigest,
		basis.RequestDigest,
		basis.ResultDigest,
	}, ":"))
}

func parseExploreTraceRef(
	raw string,
) (parsedExploreTraceRef, error) {
	parts := strings.Split(strings.TrimSpace(raw), ":")
	if len(parts) != 5 ||
		parts[0]+":"+parts[1] != exploreTraceRefPrefix {
		return parsedExploreTraceRef{}, fmt.Errorf(
			"invalid explore trace_ref",
		)
	}
	for _, digest := range parts[2:] {
		if !isSHA256Hex(digest) {
			return parsedExploreTraceRef{}, fmt.Errorf(
				"invalid explore trace_ref digest",
			)
		}
	}
	return parsedExploreTraceRef{
		indexDigest:   parts[2],
		requestDigest: parts[3],
		resultDigest:  parts[4],
	}, nil
}

func isSHA256Hex(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func compareExploreTrace(
	expected parsedExploreTraceRef,
	actual publishedExploreTraceBasis,
) *publishedExploreReplayMismatch {
	if expected.indexDigest != actual.IndexDigest {
		return &publishedExploreReplayMismatch{
			Mismatch: exploreReplayMismatchIndex,
			Expected: expected.indexDigest,
			Actual:   actual.IndexDigest,
		}
	}
	if expected.requestDigest != actual.RequestDigest {
		return &publishedExploreReplayMismatch{
			Mismatch: exploreReplayMismatchRequest,
			Expected: expected.requestDigest,
			Actual:   actual.RequestDigest,
		}
	}
	if expected.resultDigest != actual.ResultDigest {
		return &publishedExploreReplayMismatch{
			Mismatch: exploreReplayMismatchResult,
			Expected: expected.resultDigest,
			Actual:   actual.ResultDigest,
		}
	}
	return nil
}

func validatePublishedExploreResult(
	result PublishedExploreResult,
) error {
	published, ok := result.(publishedExplore)
	if !ok {
		return fmt.Errorf(
			"unsupported published explore result %T",
			result,
		)
	}
	if published.ContractVersion != exploreContractVersion {
		return fmt.Errorf("published explore contract version is invalid")
	}
	if published.View != ExplorePublicationViewWorking &&
		published.View != ExplorePublicationViewTrace &&
		published.View != ExplorePublicationViewDiagnostic {
		return fmt.Errorf("published explore view %q is invalid", published.View)
	}
	if published.Kind == PublishedExploreKindReplayMismatch {
		if published.View == ExplorePublicationViewWorking ||
			published.ReplayMismatch == nil {
			return fmt.Errorf("published explore replay mismatch is invalid")
		}
		return nil
	}
	if _, err := parseExploreTraceRef(published.TraceRef.String()); err != nil {
		return err
	}
	if published.Kind == PublishedExploreKindCandidateSet &&
		len(published.Candidates) == 0 {
		return fmt.Errorf("published explore candidate set is empty")
	}
	if published.SeedResolution == nil {
		return fmt.Errorf("published explore seed resolution is required")
	}
	if published.Kind != PublishedExploreKindResolved &&
		published.TraversalOutcome != nil {
		return fmt.Errorf(
			"non-resolved explore result cannot contain traversal outcome",
		)
	}
	if published.View == ExplorePublicationViewWorking &&
		(published.TraceBasis != nil ||
			published.ResolutionDiagnostics != nil ||
			published.RetrievalDiagnostics != nil) {
		return fmt.Errorf("working explore projection contains diagnostic fields")
	}
	if published.View == ExplorePublicationViewTrace &&
		published.TraceBasis == nil {
		return fmt.Errorf("trace explore projection lacks trace basis")
	}
	if published.View == ExplorePublicationViewDiagnostic &&
		(published.TraceBasis == nil ||
			published.ResolutionDiagnostics == nil ||
			published.RetrievalDiagnostics == nil) {
		return fmt.Errorf("diagnostic explore projection is incomplete")
	}
	return nil
}
