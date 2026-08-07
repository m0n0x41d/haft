package typedmemorywire

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// ReadEntityReference carries the TypeEnv-independent portion of one exact
// persisted reference. The selected project TypeEnv supplies the RefKindRef;
// the caller cannot smuggle a foreign TypeEnv into a current-project read.
type ReadEntityReference struct {
	refKindID typedmemory.RefKindID
	reference typedmemory.ReferenceID
}

func newReadEntityReference(
	refKindID typedmemory.RefKindID,
	reference typedmemory.ReferenceID,
) (ReadEntityReference, error) {
	value := ReadEntityReference{
		refKindID: refKindID,
		reference: reference,
	}
	if value.refKindID.String() == "" || value.reference.String() == "" {
		return ReadEntityReference{}, fmt.Errorf(
			"read entity reference is incomplete",
		)
	}
	return value, nil
}

func (reference ReadEntityReference) RefKindID() typedmemory.RefKindID {
	return reference.refKindID
}

func (reference ReadEntityReference) ReferenceID() typedmemory.ReferenceID {
	return reference.reference
}

type ResolveReadRequest struct {
	proof         *resolveReadRequestProof
	basis         BasisSelector
	query         string
	context       *typedmemory.BoundedContextRef
	maxCandidates uint32
}

type resolveReadRequestProof struct{}

var decodedResolveReadRequestProof = &resolveReadRequestProof{}

func (ResolveReadRequest) ContractVersion() string { return ContractVersion }
func (ResolveReadRequest) Action() string          { return ActionResolve }
func (ResolveReadRequest) requestVariant()         {}

func (request ResolveReadRequest) Basis() BasisSelector {
	return request.basis
}

func (request ResolveReadRequest) Query() string {
	return request.query
}

func (request ResolveReadRequest) Context() (
	typedmemory.BoundedContextRef,
	bool,
) {
	if request.context == nil {
		return typedmemory.BoundedContextRef{}, false
	}
	return *request.context, true
}

func (request ResolveReadRequest) MaxCandidates() uint32 {
	return request.maxCandidates
}

func IsDecodedResolveReadRequest(request ResolveReadRequest) bool {
	return request.proof == decodedResolveReadRequestProof &&
		readBasisIsValid(request.basis) &&
		request.query != "" &&
		request.maxCandidates > 0
}

// ReadViewSpec is transport data only. The neighborhood package remains the
// authority for profile, facet, detail, and cross-field admissibility.
type ReadViewSpec struct {
	profile        string
	requested      []string
	detail         string
	includeHistory bool
}

func (view ReadViewSpec) ProjectionProfileRef() string {
	return view.profile
}

func (view ReadViewSpec) RequestedFacets() []string {
	return append([]string{}, view.requested...)
}

func (view ReadViewSpec) Detail() string {
	return view.detail
}

func (view ReadViewSpec) IncludeHistory() bool {
	return view.includeHistory
}

func (view ReadViewSpec) valid() bool {
	return view.profile != "" &&
		len(view.requested) > 0 &&
		view.detail != ""
}

// DimensionedReadBudget is the strict transport form. The neighborhood
// builder validates semantic limits after decoding.
type DimensionedReadBudget struct {
	maxFacets              uint32
	maxItemsPerFacet       uint32
	maxRelationPaths       uint32
	maxCarrierExcerptChars uint32
	maxProvenanceDepth     uint32
}

func (budget DimensionedReadBudget) MaxFacets() uint32 {
	return budget.maxFacets
}

func (budget DimensionedReadBudget) MaxItemsPerFacet() uint32 {
	return budget.maxItemsPerFacet
}

func (budget DimensionedReadBudget) MaxRelationPathsPerItem() uint32 {
	return budget.maxRelationPaths
}

func (budget DimensionedReadBudget) MaxCarrierExcerptCharacters() uint32 {
	return budget.maxCarrierExcerptChars
}

func (budget DimensionedReadBudget) MaxProvenanceDepth() uint32 {
	return budget.maxProvenanceDepth
}

func (budget DimensionedReadBudget) valid() bool {
	return budget.maxFacets > 0 &&
		budget.maxItemsPerFacet > 0 &&
		budget.maxRelationPaths > 0 &&
		budget.maxCarrierExcerptChars > 0 &&
		budget.maxProvenanceDepth > 0
}

type NeighborhoodReadRequest struct {
	proof      *neighborhoodReadRequestProof
	basis      BasisSelector
	entity     ReadEntityReference
	context    typedmemory.BoundedContextRef
	view       ReadViewSpec
	readBudget DimensionedReadBudget
}

type neighborhoodReadRequestProof struct{}

var decodedNeighborhoodReadRequestProof = &neighborhoodReadRequestProof{}

func (NeighborhoodReadRequest) ContractVersion() string {
	return ContractVersion
}

func (NeighborhoodReadRequest) Action() string  { return ActionNeighborhood }
func (NeighborhoodReadRequest) requestVariant() {}

func (request NeighborhoodReadRequest) Basis() BasisSelector {
	return request.basis
}

func (request NeighborhoodReadRequest) Entity() ReadEntityReference {
	return request.entity
}

func (request NeighborhoodReadRequest) Context() typedmemory.BoundedContextRef {
	return request.context
}

func (request NeighborhoodReadRequest) View() ReadViewSpec {
	return request.view
}

func (request NeighborhoodReadRequest) ReadBudget() DimensionedReadBudget {
	return request.readBudget
}

func IsDecodedNeighborhoodReadRequest(
	request NeighborhoodReadRequest,
) bool {
	return request.proof == decodedNeighborhoodReadRequestProof &&
		readBasisIsValid(request.basis) &&
		request.entity.RefKindID().String() != "" &&
		request.entity.ReferenceID().String() != "" &&
		request.context.String() != "" &&
		request.view.valid() &&
		request.readBudget.valid()
}

type RecallReadRequest struct {
	proof           *recallReadRequestProof
	basis           BasisSelector
	entity          ReadEntityReference
	context         typedmemory.BoundedContextRef
	view            ReadViewSpec
	readBudget      DimensionedReadBudget
	query           string
	candidateBudget uint32
}

type recallReadRequestProof struct{}

var decodedRecallReadRequestProof = &recallReadRequestProof{}

func (RecallReadRequest) ContractVersion() string { return ContractVersion }
func (RecallReadRequest) Action() string          { return ActionRecall }
func (RecallReadRequest) requestVariant()         {}

func (request RecallReadRequest) Basis() BasisSelector {
	return request.basis
}

func (request RecallReadRequest) Entity() ReadEntityReference {
	return request.entity
}

func (request RecallReadRequest) Context() typedmemory.BoundedContextRef {
	return request.context
}

func (request RecallReadRequest) View() ReadViewSpec {
	return request.view
}

func (request RecallReadRequest) ReadBudget() DimensionedReadBudget {
	return request.readBudget
}

func (request RecallReadRequest) Query() string {
	return request.query
}

func (request RecallReadRequest) CandidateBudget() uint32 {
	return request.candidateBudget
}

func IsDecodedRecallReadRequest(request RecallReadRequest) bool {
	return request.proof == decodedRecallReadRequestProof &&
		readBasisIsValid(request.basis) &&
		request.entity.RefKindID().String() != "" &&
		request.entity.ReferenceID().String() != "" &&
		request.context.String() != "" &&
		request.view.valid() &&
		request.readBudget.valid() &&
		request.query != "" &&
		request.candidateBudget > 0
}

type resolveReadRequestWire struct {
	ContractVersion string          `json:"contract_version"`
	Action          string          `json:"action"`
	Basis           json.RawMessage `json:"basis"`
	Query           string          `json:"query"`
	BoundedContext  *string         `json:"bounded_context_ref,omitempty"`
	MaxCandidates   *uint32         `json:"max_candidates"`
}

type neighborhoodReadRequestWire struct {
	ContractVersion string          `json:"contract_version"`
	Action          string          `json:"action"`
	Basis           json.RawMessage `json:"basis"`
	Entity          json.RawMessage `json:"entity_ref"`
	Context         string          `json:"bounded_context_ref"`
	View            json.RawMessage `json:"view"`
	ReadBudget      json.RawMessage `json:"read_budget"`
}

type recallReadRequestWire struct {
	ContractVersion string          `json:"contract_version"`
	Action          string          `json:"action"`
	Basis           json.RawMessage `json:"basis"`
	Entity          json.RawMessage `json:"entity_ref"`
	Context         string          `json:"bounded_context_ref"`
	View            json.RawMessage `json:"view"`
	ReadBudget      json.RawMessage `json:"read_budget"`
	Query           string          `json:"query"`
	CandidateBudget json.RawMessage `json:"candidate_budget"`
}

type readEntityReferenceWire struct {
	RefKindID   string `json:"ref_kind_id"`
	ReferenceID string `json:"reference_id"`
}

type neighborhoodViewWire struct {
	ProjectionProfileRef string   `json:"projection_profile_ref"`
	RequestedFacets      []string `json:"requested_facets"`
	Detail               string   `json:"detail"`
	IncludeHistory       *bool    `json:"include_history"`
}

type readBudgetWire struct {
	MaxFacets                   *uint32 `json:"max_facets"`
	MaxItemsPerFacet            *uint32 `json:"max_items_per_facet"`
	MaxRelationPathsPerItem     *uint32 `json:"max_relation_paths_per_item"`
	MaxCarrierExcerptCharacters *uint32 `json:"max_carrier_excerpt_characters"`
	MaxProvenanceDepth          *uint32 `json:"max_provenance_depth"`
}

type candidateBudgetWire struct {
	MaxCandidates *uint32 `json:"max_candidates"`
}

func DecodeResolveReadRequest(payload []byte) (ResolveReadRequest, error) {
	if err := scanStrictJSON(payload); err != nil {
		return ResolveReadRequest{}, err
	}
	wire := resolveReadRequestWire{}
	if err := decodeStrict(
		payload,
		&wire,
		"$",
		"EntityOfConcern resolution request",
	); err != nil {
		return ResolveReadRequest{}, err
	}
	if err := requireReadHeader(
		wire.ContractVersion,
		wire.Action,
		ActionResolve,
	); err != nil {
		return ResolveReadRequest{}, err
	}
	basis, err := decodeProjectReadBasis(wire.Basis)
	if err != nil {
		return ResolveReadRequest{}, err
	}
	if err := requireExactReadText(wire.Query, "$.query"); err != nil {
		return ResolveReadRequest{}, err
	}
	context, err := decodeResolutionContext(wire.BoundedContext)
	if err != nil {
		return ResolveReadRequest{}, err
	}
	maxCandidates, err := requirePositiveUint32(
		wire.MaxCandidates,
		"$.max_candidates",
	)
	if err != nil {
		return ResolveReadRequest{}, err
	}
	return ResolveReadRequest{
		proof:         decodedResolveReadRequestProof,
		basis:         basis,
		query:         wire.Query,
		context:       context,
		maxCandidates: maxCandidates,
	}, nil
}

func DecodeNeighborhoodReadRequest(
	payload []byte,
) (NeighborhoodReadRequest, error) {
	if err := scanStrictJSON(payload); err != nil {
		return NeighborhoodReadRequest{}, err
	}
	wire := neighborhoodReadRequestWire{}
	if err := decodeStrict(
		payload,
		&wire,
		"$",
		"exact memory-neighborhood request",
	); err != nil {
		return NeighborhoodReadRequest{}, err
	}
	if err := requireReadHeader(
		wire.ContractVersion,
		wire.Action,
		ActionNeighborhood,
	); err != nil {
		return NeighborhoodReadRequest{}, err
	}
	basis, err := decodeProjectReadBasis(wire.Basis)
	if err != nil {
		return NeighborhoodReadRequest{}, err
	}
	entity, err := decodeReadEntityReference(wire.Entity)
	if err != nil {
		return NeighborhoodReadRequest{}, err
	}
	context, err := parseContext(
		wire.Context,
		"$.bounded_context_ref",
	)
	if err != nil {
		return NeighborhoodReadRequest{}, err
	}
	view, err := decodeNeighborhoodView(wire.View)
	if err != nil {
		return NeighborhoodReadRequest{}, err
	}
	readBudget, err := decodeReadBudget(wire.ReadBudget)
	if err != nil {
		return NeighborhoodReadRequest{}, err
	}
	return NeighborhoodReadRequest{
		proof:      decodedNeighborhoodReadRequestProof,
		basis:      basis,
		entity:     entity,
		context:    context,
		view:       view,
		readBudget: readBudget,
	}, nil
}

func DecodeRecallReadRequest(payload []byte) (RecallReadRequest, error) {
	if err := scanStrictJSON(payload); err != nil {
		return RecallReadRequest{}, err
	}
	wire := recallReadRequestWire{}
	if err := decodeStrict(
		payload,
		&wire,
		"$",
		"scoped memory-recall request",
	); err != nil {
		return RecallReadRequest{}, err
	}
	if err := requireReadHeader(
		wire.ContractVersion,
		wire.Action,
		ActionRecall,
	); err != nil {
		return RecallReadRequest{}, err
	}
	basis, err := decodeProjectReadBasis(wire.Basis)
	if err != nil {
		return RecallReadRequest{}, err
	}
	entity, err := decodeReadEntityReference(wire.Entity)
	if err != nil {
		return RecallReadRequest{}, err
	}
	context, err := parseContext(
		wire.Context,
		"$.bounded_context_ref",
	)
	if err != nil {
		return RecallReadRequest{}, err
	}
	view, err := decodeNeighborhoodView(wire.View)
	if err != nil {
		return RecallReadRequest{}, err
	}
	readBudget, err := decodeReadBudget(wire.ReadBudget)
	if err != nil {
		return RecallReadRequest{}, err
	}
	if err := requireExactReadText(wire.Query, "$.query"); err != nil {
		return RecallReadRequest{}, err
	}
	candidateBudget, err := decodeCandidateBudget(wire.CandidateBudget)
	if err != nil {
		return RecallReadRequest{}, err
	}
	return RecallReadRequest{
		proof:           decodedRecallReadRequestProof,
		basis:           basis,
		entity:          entity,
		context:         context,
		view:            view,
		readBudget:      readBudget,
		query:           wire.Query,
		candidateBudget: candidateBudget,
	}, nil
}

func requireReadHeader(
	contractVersion string,
	action string,
	expectedAction string,
) error {
	if err := requireIdentifier(
		contractVersion,
		"$.contract_version",
	); err != nil {
		return err
	}
	if contractVersion != ContractVersion {
		message := fmt.Sprintf("must equal %q", ContractVersion)
		return invalidContract("$.contract_version", message)
	}
	if err := requireIdentifier(action, "$.action"); err != nil {
		return err
	}
	if action != expectedAction {
		message := fmt.Sprintf("must equal %q", expectedAction)
		return invalidContract("$.action", message)
	}
	return nil
}

func decodeProjectReadBasis(raw []byte) (BasisSelector, error) {
	if len(raw) == 0 {
		return nil, invalidContract("$.basis", "basis is required")
	}
	basis, err := decodeBasis(raw)
	if err != nil {
		return nil, err
	}
	switch exact := basis.(type) {
	case ProjectCurrentSelector:
		return exact, nil
	case ExactProjectSelector:
		if exact.RequestedGraphRevision().Value() == 0 {
			return nil, invalidContract(
				"$.basis.graph_revision",
				"exact read graph revision must be positive",
			)
		}
		return exact, nil
	default:
		return nil, invalidContract(
			"$.basis.kind",
			"memory read requires project_current or exact_project",
		)
	}
}

func readBasisIsValid(basis BasisSelector) bool {
	switch value := basis.(type) {
	case ProjectCurrentSelector:
		return true
	case ExactProjectSelector:
		return value.RequestedTypeEnvDigest().String() != "" &&
			value.RequestedGraphRevision().Value() > 0
	default:
		return false
	}
}

func decodeResolutionContext(
	raw *string,
) (*typedmemory.BoundedContextRef, error) {
	if raw == nil {
		return nil, nil
	}
	context, err := parseContext(*raw, "$.bounded_context_ref")
	if err != nil {
		return nil, err
	}
	return &context, nil
}

func decodeReadEntityReference(
	raw []byte,
) (ReadEntityReference, error) {
	if len(raw) == 0 {
		return ReadEntityReference{}, invalidContract(
			"$.entity_ref",
			"entity_ref is required",
		)
	}
	wire := readEntityReferenceWire{}
	if err := decodeStrict(
		raw,
		&wire,
		"$.entity_ref",
		"exact entity reference",
	); err != nil {
		return ReadEntityReference{}, err
	}
	refKind, err := parseRefKindID(
		wire.RefKindID,
		"$.entity_ref.ref_kind_id",
	)
	if err != nil {
		return ReadEntityReference{}, err
	}
	reference, err := parseReferenceID(
		wire.ReferenceID,
		"$.entity_ref.reference_id",
	)
	if err != nil {
		return ReadEntityReference{}, err
	}
	return newReadEntityReference(refKind, reference)
}

func decodeNeighborhoodView(
	raw []byte,
) (ReadViewSpec, error) {
	if len(raw) == 0 {
		return ReadViewSpec{}, invalidContract(
			"$.view",
			"view is required",
		)
	}
	wire := neighborhoodViewWire{}
	if err := decodeStrict(
		raw,
		&wire,
		"$.view",
		"memory neighborhood view",
	); err != nil {
		return ReadViewSpec{}, err
	}
	if err := requireIdentifier(
		wire.ProjectionProfileRef,
		"$.view.projection_profile_ref",
	); err != nil {
		return ReadViewSpec{}, err
	}
	facets, err := decodeRequestedFacets(wire.RequestedFacets)
	if err != nil {
		return ReadViewSpec{}, err
	}
	if err := requireIdentifier(wire.Detail, "$.view.detail"); err != nil {
		return ReadViewSpec{}, err
	}
	if wire.IncludeHistory == nil {
		return ReadViewSpec{}, invalidContract(
			"$.view.include_history",
			"include_history is required",
		)
	}
	view := ReadViewSpec{
		profile:        wire.ProjectionProfileRef,
		requested:      facets,
		detail:         wire.Detail,
		includeHistory: *wire.IncludeHistory,
	}
	if !view.valid() {
		return ReadViewSpec{}, invalidContract("$.view", "view is incomplete")
	}
	return view, nil
}

func decodeRequestedFacets(
	raw []string,
) ([]string, error) {
	if len(raw) == 0 {
		return nil, invalidContract(
			"$.view.requested_facets",
			"requested_facets must be non-empty",
		)
	}
	result := make([]string, 0, len(raw))
	for index, value := range raw {
		path := fmt.Sprintf("$.view.requested_facets[%d]", index)
		if err := requireIdentifier(value, path); err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, nil
}

func decodeReadBudget(
	raw []byte,
) (DimensionedReadBudget, error) {
	if len(raw) == 0 {
		return DimensionedReadBudget{}, invalidContract(
			"$.read_budget",
			"read_budget is required",
		)
	}
	wire := readBudgetWire{}
	if err := decodeStrict(
		raw,
		&wire,
		"$.read_budget",
		"dimensioned read budget",
	); err != nil {
		return DimensionedReadBudget{}, err
	}
	maxFacets, err := requirePositiveUint32(
		wire.MaxFacets,
		"$.read_budget.max_facets",
	)
	if err != nil {
		return DimensionedReadBudget{}, err
	}
	maxItems, err := requirePositiveUint32(
		wire.MaxItemsPerFacet,
		"$.read_budget.max_items_per_facet",
	)
	if err != nil {
		return DimensionedReadBudget{}, err
	}
	maxPaths, err := requirePositiveUint32(
		wire.MaxRelationPathsPerItem,
		"$.read_budget.max_relation_paths_per_item",
	)
	if err != nil {
		return DimensionedReadBudget{}, err
	}
	maxExcerpt, err := requirePositiveUint32(
		wire.MaxCarrierExcerptCharacters,
		"$.read_budget.max_carrier_excerpt_characters",
	)
	if err != nil {
		return DimensionedReadBudget{}, err
	}
	maxProvenance, err := requirePositiveUint32(
		wire.MaxProvenanceDepth,
		"$.read_budget.max_provenance_depth",
	)
	if err != nil {
		return DimensionedReadBudget{}, err
	}
	budget := DimensionedReadBudget{
		maxFacets:              maxFacets,
		maxItemsPerFacet:       maxItems,
		maxRelationPaths:       maxPaths,
		maxCarrierExcerptChars: maxExcerpt,
		maxProvenanceDepth:     maxProvenance,
	}
	if !budget.valid() {
		return DimensionedReadBudget{}, invalidContract(
			"$.read_budget",
			"read budget is incomplete",
		)
	}
	return budget, nil
}

func decodeCandidateBudget(
	raw []byte,
) (uint32, error) {
	if len(raw) == 0 {
		return 0, invalidContract(
			"$.candidate_budget",
			"candidate_budget is required",
		)
	}
	wire := candidateBudgetWire{}
	if err := decodeStrict(
		raw,
		&wire,
		"$.candidate_budget",
		"scoped recall candidate budget",
	); err != nil {
		return 0, err
	}
	maxCandidates, err := requirePositiveUint32(
		wire.MaxCandidates,
		"$.candidate_budget.max_candidates",
	)
	if err != nil {
		return 0, err
	}
	return maxCandidates, nil
}

func requirePositiveUint32(raw *uint32, path string) (uint32, error) {
	if raw == nil {
		return 0, invalidContract(path, "value is required")
	}
	if *raw == 0 {
		return 0, invalidContract(path, "value must be positive")
	}
	return *raw, nil
}

func requireExactReadText(value string, path string) error {
	if err := requireText(value, path); err != nil {
		return err
	}
	if strings.TrimSpace(value) != value {
		return invalidContract(
			path,
			"text must not have leading or trailing whitespace",
		)
	}
	return nil
}
