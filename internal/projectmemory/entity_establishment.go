package projectmemory

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/m0n0x41d/haft/internal/projectmemory/entitycontract"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

const (
	EntityContractVersion = entitycontract.Version
	EntityActionEstablish = entitycontract.ActionEstablish
	EntityReferenceKindID = entitycontract.EntityReferenceKindID
	MaximumEntityAliases  = entitycontract.MaximumAliases
)

type EntityPersistenceReason string

const (
	EntityPersistenceExplicitOperatorRequest EntityPersistenceReason = entitycontract.ExplicitOperatorRequest
	EntityPersistenceNamedReceivingUse       EntityPersistenceReason = entitycontract.NamedReceivingUse
)

func (reason EntityPersistenceReason) valid() bool {
	switch reason {
	case EntityPersistenceExplicitOperatorRequest,
		EntityPersistenceNamedReceivingUse:
		return true
	default:
		return false
	}
}

// EntityEstablishmentRequest is the strong agent-facing intent. It contains no
// TypeEnv, graph revision, authority class, batch-local ref, or raw
// MemoryChangeSet; those are trusted implementation coordinates derived by the
// project runtime.
type EntityEstablishmentRequest struct {
	entityID          typedmemory.EntityID
	label             typedmemory.EntityLabel
	context           typedmemory.BoundedContextRef
	aliases           []typedmemory.EntityAlias
	persistenceReason EntityPersistenceReason
	requestProvenance typedmemory.ProvenanceRef
	idempotencyKey    typedmemorystore.IdempotencyKey
}

func (request EntityEstablishmentRequest) EntityID() typedmemory.EntityID {
	return request.entityID
}

func (request EntityEstablishmentRequest) Label() typedmemory.EntityLabel {
	return request.label
}

func (request EntityEstablishmentRequest) Context() typedmemory.BoundedContextRef {
	return request.context
}

func (request EntityEstablishmentRequest) Aliases() []typedmemory.EntityAlias {
	return append([]typedmemory.EntityAlias(nil), request.aliases...)
}

func (request EntityEstablishmentRequest) PersistenceReason() EntityPersistenceReason {
	return request.persistenceReason
}

func (request EntityEstablishmentRequest) RequestProvenance() typedmemory.ProvenanceRef {
	return request.requestProvenance
}

func (request EntityEstablishmentRequest) IdempotencyKey() typedmemorystore.IdempotencyKey {
	return request.idempotencyKey
}

func (request EntityEstablishmentRequest) Candidate() (
	typedmemory.MemoryChangeSet,
	error,
) {
	localRef, err := entityEstablishmentLocalRef(request.entityID, request.context)
	if err != nil {
		return typedmemory.MemoryChangeSet{}, err
	}
	declaration, err := typedmemory.NewDeclareEntity(
		request.entityID,
		localRef,
		request.context,
		request.label,
		request.requestProvenance,
	)
	if err != nil {
		return typedmemory.MemoryChangeSet{}, err
	}
	changes := make(
		[]typedmemory.MemoryChange,
		0,
		1+len(request.aliases),
	)
	changes = append(changes, declaration)
	for _, alias := range request.aliases {
		admission, admissionErr := typedmemory.NewAdmitAlias(
			request.entityID,
			alias,
			request.context,
			request.requestProvenance,
		)
		if admissionErr != nil {
			return typedmemory.MemoryChangeSet{}, admissionErr
		}
		effect, effectErr := typedmemory.NewApplyIdentityChange(admission)
		if effectErr != nil {
			return typedmemory.MemoryChangeSet{}, effectErr
		}
		changes = append(changes, effect)
	}
	return typedmemory.NewMemoryChangeSet(changes)
}

type entityEstablishmentRequestWire struct {
	Action               string    `json:"action"`
	EntityID             string    `json:"entity_id"`
	Label                string    `json:"label"`
	BoundedContextRef    string    `json:"bounded_context_ref"`
	Aliases              *[]string `json:"aliases"`
	PersistenceReason    string    `json:"persistence_reason"`
	RequestProvenanceRef string    `json:"request_provenance_ref"`
	IdempotencyKey       string    `json:"idempotency_key"`
}

func DecodeEntityEstablishmentRequest(
	payload []byte,
) (EntityEstablishmentRequest, error) {
	if err := typedmemorywire.ValidateStrictJSON(payload); err != nil {
		return EntityEstablishmentRequest{}, err
	}
	wire := entityEstablishmentRequestWire{}
	reader := bytes.NewReader(payload)
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return EntityEstablishmentRequest{}, fmt.Errorf(
			"invalid %s request: %w",
			EntityContractVersion,
			err,
		)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return EntityEstablishmentRequest{}, fmt.Errorf(
			"invalid %s request: trailing JSON value",
			EntityContractVersion,
		)
	}
	if wire.Action != EntityActionEstablish {
		return EntityEstablishmentRequest{}, fmt.Errorf(
			"action must be %q",
			EntityActionEstablish,
		)
	}
	if wire.Aliases == nil {
		return EntityEstablishmentRequest{}, fmt.Errorf("aliases is required")
	}
	if len(*wire.Aliases) > MaximumEntityAliases {
		return EntityEstablishmentRequest{}, fmt.Errorf(
			"aliases exceeds %d items",
			MaximumEntityAliases,
		)
	}
	entity, err := parseEntityEstablishmentIdentifier(
		wire.EntityID,
		"entity_id",
		typedmemory.NewEntityID,
	)
	if err != nil {
		return EntityEstablishmentRequest{}, err
	}
	label, err := parseEntityEstablishmentLabel(wire.Label)
	if err != nil {
		return EntityEstablishmentRequest{}, err
	}
	contextRef, err := parseEntityEstablishmentIdentifier(
		wire.BoundedContextRef,
		"bounded_context_ref",
		typedmemory.NewBoundedContextRef,
	)
	if err != nil {
		return EntityEstablishmentRequest{}, err
	}
	aliases, err := parseEntityEstablishmentAliases(*wire.Aliases)
	if err != nil {
		return EntityEstablishmentRequest{}, err
	}
	reason := EntityPersistenceReason(wire.PersistenceReason)
	if !reason.valid() {
		return EntityEstablishmentRequest{}, fmt.Errorf(
			"persistence_reason must be %q or %q",
			EntityPersistenceExplicitOperatorRequest,
			EntityPersistenceNamedReceivingUse,
		)
	}
	provenance, err := parseEntityEstablishmentIdentifier(
		wire.RequestProvenanceRef,
		"request_provenance_ref",
		typedmemory.NewProvenanceRef,
	)
	if err != nil {
		return EntityEstablishmentRequest{}, err
	}
	if err := requireEntityEstablishmentCanonicalText(
		wire.IdempotencyKey,
		"idempotency_key",
		typedmemorywire.MaximumAdmissionIdempotencyKeyBytes,
	); err != nil {
		return EntityEstablishmentRequest{}, err
	}
	key, err := typedmemorystore.NewIdempotencyKey(wire.IdempotencyKey)
	if err != nil {
		return EntityEstablishmentRequest{}, fmt.Errorf(
			"invalid idempotency_key: %w",
			err,
		)
	}
	return EntityEstablishmentRequest{
		entityID:          entity,
		label:             label,
		context:           contextRef,
		aliases:           aliases,
		persistenceReason: reason,
		requestProvenance: provenance,
		idempotencyKey:    key,
	}, nil
}

func parseEntityEstablishmentIdentifier[T any](
	raw string,
	field string,
	parse func(string) (T, error),
) (T, error) {
	var zero T
	if err := requireEntityEstablishmentCanonicalText(
		raw,
		field,
		typedmemorywire.MaximumIdentifierBytes,
	); err != nil {
		return zero, err
	}
	value, err := parse(raw)
	if err != nil {
		return zero, fmt.Errorf("invalid %s: %w", field, err)
	}
	return value, nil
}

func parseEntityEstablishmentLabel(
	raw string,
) (typedmemory.EntityLabel, error) {
	if err := requireEntityEstablishmentCanonicalText(
		raw,
		"label",
		typedmemorywire.MaximumTextBytes,
	); err != nil {
		return typedmemory.EntityLabel{}, err
	}
	label, err := typedmemory.NewEntityLabel(raw)
	if err != nil {
		return typedmemory.EntityLabel{}, fmt.Errorf("invalid label: %w", err)
	}
	return label, nil
}

func parseEntityEstablishmentAliases(
	raw []string,
) ([]typedmemory.EntityAlias, error) {
	aliases := make([]typedmemory.EntityAlias, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for index, value := range raw {
		field := fmt.Sprintf("aliases[%d]", index)
		alias, err := parseEntityEstablishmentIdentifier(
			value,
			field,
			typedmemory.NewEntityAlias,
		)
		if err != nil {
			return nil, err
		}
		if _, duplicate := seen[alias.String()]; duplicate {
			return nil, fmt.Errorf("%s duplicates alias %q", field, alias.String())
		}
		if index > 0 &&
			strings.Compare(aliases[index-1].String(), alias.String()) >= 0 {
			return nil, fmt.Errorf(
				"aliases must be supplied in strictly increasing canonical order",
			)
		}
		seen[alias.String()] = struct{}{}
		aliases = append(aliases, alias)
	}
	return aliases, nil
}

func requireEntityEstablishmentCanonicalText(
	raw string,
	field string,
	maximum int,
) error {
	if raw == "" || raw != strings.TrimSpace(raw) {
		return fmt.Errorf("%s must be non-empty canonical text", field)
	}
	if strings.ContainsAny(raw, "\r\n\t") {
		return fmt.Errorf("%s must be one line", field)
	}
	if len(raw) > maximum {
		return fmt.Errorf("%s exceeds %d bytes", field, maximum)
	}
	return nil
}

func entityEstablishmentLocalRef(
	entity typedmemory.EntityID,
	contextRef typedmemory.BoundedContextRef,
) (typedmemory.BatchLocalRef, error) {
	sum := sha256.Sum256(
		[]byte(entity.String() + "\x00" + contextRef.String()),
	)
	return typedmemory.NewBatchLocalRef(
		"local:haft-entity:" + hex.EncodeToString(sum[:16]),
	)
}

type EntityEstablishmentResultKind string

const (
	EntityOnboardingRequiredResult       EntityEstablishmentResultKind = "onboarding_required"
	EntityEnablementChoiceRequiredResult EntityEstablishmentResultKind = "enablement_choice_required"
	EntityRestartRequiredResult          EntityEstablishmentResultKind = "restart_required"
	EntityEstablishedResult              EntityEstablishmentResultKind = "established"
	EntityIdentityConflictResult         EntityEstablishmentResultKind = "identity_conflict"
	EntityAliasConflictResult            EntityEstablishmentResultKind = "alias_conflict"
	EntityIdempotencyConflictResult      EntityEstablishmentResultKind = "idempotency_conflict"
	EntityRejectedResult                 EntityEstablishmentResultKind = "rejected"
	EntityCommitOutcomeUnknownResult     EntityEstablishmentResultKind = "commit_outcome_unknown"
)

type EntityEstablishmentResult interface {
	Kind() EntityEstablishmentResultKind
	entityEstablishmentResultVariant()
}

type EntityReference struct {
	referenceID string
}

func NewEntityReference(entity typedmemory.EntityID) (EntityReference, error) {
	if entity.String() == "" {
		return EntityReference{}, fmt.Errorf("entity reference requires an EntityID")
	}
	return EntityReference{referenceID: entity.String()}, nil
}

func (reference EntityReference) RefKindID() string {
	return EntityReferenceKindID
}

func (reference EntityReference) ReferenceID() string {
	return reference.referenceID
}

func (reference EntityReference) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		RefKindID   string `json:"ref_kind_id"`
		ReferenceID string `json:"reference_id"`
	}{
		RefKindID:   EntityReferenceKindID,
		ReferenceID: reference.referenceID,
	})
}

type EntityEstablishmentRecovery struct {
	kind       EntityEstablishmentResultKind
	detail     string
	nextAction string
}

func NewEntityOnboardingRequired(
	detail string,
) (EntityEstablishmentRecovery, error) {
	return newEntityEstablishmentRecovery(
		EntityOnboardingRequiredResult,
		detail,
		"Run haft init, use h-onboard, restart the host MCP session, then retry the unchanged request.",
	)
}

func NewEntityEnablementChoiceRequired(
	detail string,
) (EntityEstablishmentRecovery, error) {
	return newEntityEstablishmentRecovery(
		EntityEnablementChoiceRequiredResult,
		detail,
		"Run haft init to repair the default project-memory installation, restart the host MCP session, then retry the unchanged request; no entity was written.",
	)
}

func NewEntityRestartRequired(
	detail string,
) (EntityEstablishmentRecovery, error) {
	return newEntityEstablishmentRecovery(
		EntityRestartRequiredResult,
		detail,
		"Restart the host MCP session, then retry this unchanged request with the same idempotency_key.",
	)
}

func newEntityEstablishmentRecovery(
	kind EntityEstablishmentResultKind,
	detail string,
	nextAction string,
) (EntityEstablishmentRecovery, error) {
	if strings.TrimSpace(detail) == "" || strings.TrimSpace(nextAction) == "" {
		return EntityEstablishmentRecovery{}, fmt.Errorf(
			"entity establishment recovery requires detail and next action",
		)
	}
	return EntityEstablishmentRecovery{
		kind:       kind,
		detail:     strings.TrimSpace(detail),
		nextAction: strings.TrimSpace(nextAction),
	}, nil
}

func (result EntityEstablishmentRecovery) Kind() EntityEstablishmentResultKind {
	return result.kind
}

func (EntityEstablishmentRecovery) entityEstablishmentResultVariant() {}

func (result EntityEstablishmentRecovery) Detail() string {
	return result.detail
}

func (result EntityEstablishmentRecovery) NextAction() string {
	return result.nextAction
}

type EntityEstablishmentDeliveryKind string

const (
	EntityFreshlyCommittedDelivery EntityEstablishmentDeliveryKind = "freshly_committed"
	EntityReplayedDelivery         EntityEstablishmentDeliveryKind = "replayed"
	EntityAlreadyExactDelivery     EntityEstablishmentDeliveryKind = "already_exact"
)

type EntityEstablished struct {
	delivery   EntityEstablishmentDeliveryKind
	entity     EntityReference
	label      string
	context    string
	aliases    []string
	receipt    typedmemorystore.CommitReceipt
	hasReceipt bool
}

func NewCommittedEntityEstablished(
	request EntityEstablishmentRequest,
	delivery EntityEstablishmentDeliveryKind,
	receipt typedmemorystore.CommitReceipt,
) (EntityEstablished, error) {
	if delivery != EntityFreshlyCommittedDelivery &&
		delivery != EntityReplayedDelivery {
		return EntityEstablished{}, fmt.Errorf(
			"committed entity delivery must be freshly_committed or replayed",
		)
	}
	if receipt.EventRef() == "" ||
		receipt.CommitRef() == "" ||
		receipt.ResultDigest().String() == "" {
		return EntityEstablished{}, fmt.Errorf(
			"committed entity delivery requires a durable receipt",
		)
	}
	return newEntityEstablished(request, delivery, receipt, true)
}

func NewAlreadyExactEntityEstablished(
	request EntityEstablishmentRequest,
) (EntityEstablished, error) {
	return newEntityEstablished(
		request,
		EntityAlreadyExactDelivery,
		typedmemorystore.CommitReceipt{},
		false,
	)
}

func newEntityEstablished(
	request EntityEstablishmentRequest,
	delivery EntityEstablishmentDeliveryKind,
	receipt typedmemorystore.CommitReceipt,
	hasReceipt bool,
) (EntityEstablished, error) {
	reference, err := NewEntityReference(request.entityID)
	if err != nil {
		return EntityEstablished{}, err
	}
	aliases := make([]string, 0, len(request.aliases))
	for _, alias := range request.aliases {
		aliases = append(aliases, alias.String())
	}
	return EntityEstablished{
		delivery:   delivery,
		entity:     reference,
		label:      request.label.String(),
		context:    request.context.String(),
		aliases:    aliases,
		receipt:    receipt,
		hasReceipt: hasReceipt,
	}, nil
}

func (EntityEstablished) Kind() EntityEstablishmentResultKind {
	return EntityEstablishedResult
}

func (EntityEstablished) entityEstablishmentResultVariant() {}

func (result EntityEstablished) DeliveryKind() EntityEstablishmentDeliveryKind {
	return result.delivery
}

func (result EntityEstablished) EntityRef() EntityReference {
	return result.entity
}

func (result EntityEstablished) Receipt() (
	typedmemorystore.CommitReceipt,
	bool,
) {
	return result.receipt, result.hasReceipt
}

type EntityIdentityConflict struct {
	entity EntityReference
	detail string
}

func NewEntityIdentityConflict(
	entity typedmemory.EntityID,
	detail string,
) (EntityIdentityConflict, error) {
	reference, err := NewEntityReference(entity)
	if err != nil {
		return EntityIdentityConflict{}, err
	}
	if strings.TrimSpace(detail) == "" {
		return EntityIdentityConflict{}, fmt.Errorf(
			"entity identity conflict requires detail",
		)
	}
	return EntityIdentityConflict{
		entity: reference,
		detail: strings.TrimSpace(detail),
	}, nil
}

func (EntityIdentityConflict) Kind() EntityEstablishmentResultKind {
	return EntityIdentityConflictResult
}

func (EntityIdentityConflict) entityEstablishmentResultVariant() {}

func (result EntityIdentityConflict) EntityRef() EntityReference {
	return result.entity
}

func (result EntityIdentityConflict) Detail() string {
	return result.detail
}

type EntityAliasConflict struct {
	alias          string
	existingEntity EntityReference
	detail         string
}

func NewEntityAliasConflict(
	alias typedmemory.EntityAlias,
	existingEntity typedmemory.EntityID,
	detail string,
) (EntityAliasConflict, error) {
	reference, err := NewEntityReference(existingEntity)
	if err != nil {
		return EntityAliasConflict{}, err
	}
	if alias.String() == "" || strings.TrimSpace(detail) == "" {
		return EntityAliasConflict{}, fmt.Errorf(
			"entity alias conflict requires alias, existing entity, and detail",
		)
	}
	return EntityAliasConflict{
		alias:          alias.String(),
		existingEntity: reference,
		detail:         strings.TrimSpace(detail),
	}, nil
}

func (EntityAliasConflict) Kind() EntityEstablishmentResultKind {
	return EntityAliasConflictResult
}

func (EntityAliasConflict) entityEstablishmentResultVariant() {}

type EntityIdempotencyConflict struct {
	detail string
}

func NewEntityIdempotencyConflict(
	detail string,
) (EntityIdempotencyConflict, error) {
	if strings.TrimSpace(detail) == "" {
		return EntityIdempotencyConflict{}, fmt.Errorf(
			"entity idempotency conflict requires detail",
		)
	}
	return EntityIdempotencyConflict{detail: strings.TrimSpace(detail)}, nil
}

func (EntityIdempotencyConflict) Kind() EntityEstablishmentResultKind {
	return EntityIdempotencyConflictResult
}

func (EntityIdempotencyConflict) entityEstablishmentResultVariant() {}

type EntityEstablishmentRejected struct {
	details []string
}

func NewEntityEstablishmentRejected(
	details []string,
) (EntityEstablishmentRejected, error) {
	canonical := canonicalEntityResultDetails(details)
	if len(canonical) == 0 {
		return EntityEstablishmentRejected{}, fmt.Errorf(
			"rejected entity establishment requires detail",
		)
	}
	return EntityEstablishmentRejected{details: canonical}, nil
}

func (EntityEstablishmentRejected) Kind() EntityEstablishmentResultKind {
	return EntityRejectedResult
}

func (EntityEstablishmentRejected) entityEstablishmentResultVariant() {}

type EntityEstablishmentCommitOutcomeUnknown struct {
	detail         string
	idempotencyKey string
}

func NewEntityEstablishmentCommitOutcomeUnknown(
	request EntityEstablishmentRequest,
	detail string,
) (EntityEstablishmentCommitOutcomeUnknown, error) {
	if request.idempotencyKey.String() == "" ||
		strings.TrimSpace(detail) == "" {
		return EntityEstablishmentCommitOutcomeUnknown{}, fmt.Errorf(
			"unknown entity commit outcome requires request key and detail",
		)
	}
	return EntityEstablishmentCommitOutcomeUnknown{
		detail:         strings.TrimSpace(detail),
		idempotencyKey: request.idempotencyKey.String(),
	}, nil
}

func (EntityEstablishmentCommitOutcomeUnknown) Kind() EntityEstablishmentResultKind {
	return EntityCommitOutcomeUnknownResult
}

func (EntityEstablishmentCommitOutcomeUnknown) entityEstablishmentResultVariant() {}

// EntityEstablishmentPort is the only effect capability needed by the MCP
// adapter. Closed results carry recovery policy; the transport shell neither
// infers setup state nor interprets store errors.
type EntityEstablishmentPort interface {
	Establish(
		context.Context,
		EntityEstablishmentRequest,
	) (EntityEstablishmentResult, error)
}

func MarshalEntityEstablishmentResult(
	result EntityEstablishmentResult,
) ([]byte, error) {
	if result == nil {
		return nil, fmt.Errorf("entity establishment result is required")
	}
	common := entityEstablishmentResponse{
		ContractVersion: EntityContractVersion,
		Action:          EntityActionEstablish,
		Result:          result.Kind(),
		Persistence: entityEstablishmentPersistenceResponse{
			AuthorityGranted: false,
		},
	}
	switch value := result.(type) {
	case EntityEstablishmentRecovery:
		common.Detail = value.detail
		common.NextAction = value.nextAction
	case EntityEstablished:
		common.DeliveryKind = value.delivery
		common.EntityRef = &value.entity
		common.Label = value.label
		common.BoundedContextRef = value.context
		common.Aliases = append([]string(nil), value.aliases...)
		common.Persistence.Performed =
			value.delivery != EntityAlreadyExactDelivery
		common.NextRead = newEntityNeighborhoodReadResponse(
			value.entity,
			value.context,
		)
	case EntityIdentityConflict:
		common.EntityRef = &value.entity
		common.Detail = value.detail
	case EntityAliasConflict:
		common.Alias = value.alias
		common.ExistingEntityRef = &value.existingEntity
		common.Detail = value.detail
	case EntityIdempotencyConflict:
		common.Detail = value.detail
	case EntityEstablishmentRejected:
		common.Details = append([]string(nil), value.details...)
	case EntityEstablishmentCommitOutcomeUnknown:
		common.Detail = value.detail
		common.Retry = &entityEstablishmentRetryResponse{
			IdempotencyKey: value.idempotencyKey,
			Instruction:    "Retry this unchanged request with the same idempotency_key.",
		}
	default:
		return nil, fmt.Errorf(
			"unsupported entity establishment result %T",
			result,
		)
	}
	return json.Marshal(common)
}

type entityEstablishmentResponse struct {
	ContractVersion   string                                 `json:"contract_version"`
	Action            string                                 `json:"action"`
	Result            EntityEstablishmentResultKind          `json:"result"`
	DeliveryKind      EntityEstablishmentDeliveryKind        `json:"delivery_kind,omitempty"`
	EntityRef         *EntityReference                       `json:"entity_ref,omitempty"`
	Label             string                                 `json:"label,omitempty"`
	BoundedContextRef string                                 `json:"bounded_context_ref,omitempty"`
	Aliases           []string                               `json:"aliases,omitempty"`
	Alias             string                                 `json:"alias,omitempty"`
	ExistingEntityRef *EntityReference                       `json:"existing_entity_ref,omitempty"`
	Detail            string                                 `json:"detail,omitempty"`
	Details           []string                               `json:"details,omitempty"`
	NextAction        string                                 `json:"next_action,omitempty"`
	Retry             *entityEstablishmentRetryResponse      `json:"retry,omitempty"`
	NextRead          *entityEstablishmentNextReadResponse   `json:"next_read,omitempty"`
	Persistence       entityEstablishmentPersistenceResponse `json:"persistence"`
}

type entityEstablishmentPersistenceResponse struct {
	Performed        bool `json:"performed"`
	AuthorityGranted bool `json:"authority_granted"`
}

type entityEstablishmentRetryResponse struct {
	IdempotencyKey string `json:"idempotency_key"`
	Instruction    string `json:"instruction"`
}

type entityEstablishmentNextReadResponse struct {
	Tool      string                             `json:"tool"`
	Arguments entityEstablishmentMemoryArguments `json:"arguments"`
}

type entityEstablishmentMemoryArguments struct {
	Action        string                                   `json:"action"`
	MemoryRequest entityEstablishmentNeighborhoodArguments `json:"memory_request"`
}

type entityEstablishmentNeighborhoodArguments struct {
	ContractVersion   string                                `json:"contract_version"`
	Mode              string                                `json:"mode"`
	Basis             entityEstablishmentCurrentBasis       `json:"basis"`
	EntityRef         EntityReference                       `json:"entity_ref"`
	BoundedContextRef string                                `json:"bounded_context_ref"`
	View              entityEstablishmentNeighborhoodView   `json:"view"`
	ReadBudget        entityEstablishmentNeighborhoodBudget `json:"read_budget"`
}

type entityEstablishmentCurrentBasis struct {
	Kind string `json:"kind"`
}

type entityEstablishmentNeighborhoodView struct {
	ProjectionProfileRef string   `json:"projection_profile_ref"`
	RequestedFacets      []string `json:"requested_facets"`
	Detail               string   `json:"detail"`
	IncludeHistory       bool     `json:"include_history"`
}

type entityEstablishmentNeighborhoodBudget struct {
	MaxFacets                   uint32 `json:"max_facets"`
	MaxItemsPerFacet            uint32 `json:"max_items_per_facet"`
	MaxRelationPathsPerItem     uint32 `json:"max_relation_paths_per_item"`
	MaxCarrierExcerptCharacters uint32 `json:"max_carrier_excerpt_characters"`
	MaxProvenanceDepth          uint32 `json:"max_provenance_depth"`
}

func newEntityNeighborhoodReadResponse(
	entity EntityReference,
	contextRef string,
) *entityEstablishmentNextReadResponse {
	return &entityEstablishmentNextReadResponse{
		Tool: "haft_query",
		Arguments: entityEstablishmentMemoryArguments{
			Action: typedmemorywire.QueryActionMemory,
			MemoryRequest: entityEstablishmentNeighborhoodArguments{
				ContractVersion: typedmemorywire.ContractVersion,
				Mode:            typedmemorywire.ActionNeighborhood,
				Basis: entityEstablishmentCurrentBasis{
					Kind: string(typedmemorywire.BasisProjectCurrent),
				},
				EntityRef:         entity,
				BoundedContextRef: contextRef,
				View: entityEstablishmentNeighborhoodView{
					ProjectionProfileRef: "agent_orientation.v2",
					RequestedFacets: []string{
						"epistemes",
						"problems",
						"alternatives",
						"decisions",
						"specifications",
						"evidence",
						"work",
						"implementation",
						"unresolved",
					},
					Detail:         "standard",
					IncludeHistory: false,
				},
				ReadBudget: entityEstablishmentNeighborhoodBudget{
					MaxFacets:                   9,
					MaxItemsPerFacet:            20,
					MaxRelationPathsPerItem:     8,
					MaxCarrierExcerptCharacters: 4096,
					MaxProvenanceDepth:          4,
				},
			},
		},
	}
}

func canonicalEntityResultDetails(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		canonical := strings.TrimSpace(value)
		if canonical == "" {
			continue
		}
		if _, duplicate := seen[canonical]; duplicate {
			continue
		}
		seen[canonical] = struct{}{}
		result = append(result, canonical)
	}
	slices.Sort(result)
	return result
}
