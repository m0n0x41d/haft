package typedmemorywire

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	ContractVersionV1  = "haft.memory.v1"
	ContractVersionV2  = "haft.memory.v2"
	ContractVersion    = ContractVersionV1
	ActionValidate     = "validate"
	ActionAdmit        = "admit"
	ActionResolve      = "resolve"
	ActionNeighborhood = "neighborhood"
	ActionRecall       = "recall"

	MaximumChanges              = 64
	MaximumSlotBindings         = 64
	MaximumFillersPerSlot       = 64
	MaximumContextPins          = 64
	MaximumEnvironmentSelectors = 64
	MaximumTypedValueBytes      = 256 * 1024
	MaximumIdentifierBytes      = 4096
	MaximumTextBytes            = 16 * 1024
)

type BasisKind string

const (
	BasisBundledCandidateOpenWorld BasisKind = "bundled_candidate_open_world"
	BasisProjectCurrent            BasisKind = "project_current"
	BasisExactProject              BasisKind = "exact_project"
)

// BasisSelector is a caller request, not an observed active environment or
// snapshot. The service resolves it against project state before asking the
// candidate to bind its TypeEnv-dependent references.
type BasisSelector interface {
	Kind() BasisKind
	basisSelectorVariant()
}

// BundledCandidateOpenWorldSelector asks to inspect the bundled compiled
// candidate without claiming a project snapshot, active TypeEnv, or authority
// to persist. This selector alone cannot establish a project-level Valid
// verdict.
type BundledCandidateOpenWorldSelector struct{}

func (BundledCandidateOpenWorldSelector) Kind() BasisKind {
	return BasisBundledCandidateOpenWorld
}

func (BundledCandidateOpenWorldSelector) basisSelectorVariant() {}

type ProjectCurrentSelector struct{}

func (ProjectCurrentSelector) Kind() BasisKind { return BasisProjectCurrent }

func (ProjectCurrentSelector) basisSelectorVariant() {}

type ExactProjectSelector struct {
	requestedTypeEnvDigest typedmemory.SHA256Digest
	requestedGraphRevision typedmemory.GraphRevision
}

func (ExactProjectSelector) Kind() BasisKind { return BasisExactProject }

func (ExactProjectSelector) basisSelectorVariant() {}

func (selector ExactProjectSelector) RequestedTypeEnvDigest() typedmemory.SHA256Digest {
	return selector.requestedTypeEnvDigest
}

func (selector ExactProjectSelector) RequestedGraphRevision() typedmemory.GraphRevision {
	return selector.requestedGraphRevision
}

// ValidateRequest is an opaque, decoder-owned request. Its zero value is not a
// decoded request. Keeping this as an exact concrete type prevents a foreign
// wrapper from embedding a sealed interface and overriding lowering or resource
// accounting methods while retaining the promoted private seal.
type ValidateRequest struct {
	proof           *validateRequestProof
	contractVersion string
	basis           BasisSelector
	changeSet       memoryChangeSetCandidate
}

type validateRequestProof struct{}

var decodedValidateRequestProof = &validateRequestProof{}

func (request ValidateRequest) ContractVersion() string { return request.contractVersion }

func (ValidateRequest) Action() string { return ActionValidate }

func (ValidateRequest) requestVariant() {}

func (request ValidateRequest) Basis() BasisSelector { return request.basis }

func (request ValidateRequest) ChangeCount() int { return len(request.changeSet.changes) }

// DiagnosticCoordinates returns an immutable request-coordinate index captured
// by the strict decoder. It is presentation metadata only; semantic validation
// still uses BindChangeSet and the server-resolved TypeEnv.
func (request ValidateRequest) DiagnosticCoordinates() DiagnosticCoordinateIndex {
	if !IsDecodedValidateRequest(request) {
		return DiagnosticCoordinateIndex{}
	}
	return copyDiagnosticCoordinateIndex(request.changeSet.coordinates)
}

func (request ValidateRequest) BindChangeSet(
	typeEnv typedmemory.TypeEnvRef,
) (typedmemory.MemoryChangeSet, error) {
	if !IsDecodedValidateRequest(request) {
		return typedmemory.MemoryChangeSet{}, fmt.Errorf("decoded validation request is required")
	}
	return request.changeSet.bind(typeEnv)
}

// IsDecodedValidateRequest proves that a request came from this package's
// strict decoder. Action needs no second comparison because it is constant on
// this exact concrete type. ContractVersion is decoder-owned so v1 legacy and
// v2 assertion requests cannot silently widen one another.
func IsDecodedValidateRequest(request ValidateRequest) bool {
	if request.proof != decodedValidateRequestProof ||
		!validValidateContractVersion(request.contractVersion) ||
		request.basis == nil {
		return false
	}
	count := len(request.changeSet.changes)
	return count > 0 &&
		count <= MaximumChanges &&
		request.changeSet.coordinates.validFor(count)
}

func validValidateContractVersion(version string) bool {
	return version == ContractVersionV1 || version == ContractVersionV2
}

type memoryChangeCandidate interface {
	bind(typedmemory.TypeEnvRef) (typedmemory.MemoryChange, error)
	memoryChangeCandidateVariant()
}

type exactMemoryChangeCandidate struct {
	change typedmemory.MemoryChange
}

func (candidate exactMemoryChangeCandidate) bind(
	typedmemory.TypeEnvRef,
) (typedmemory.MemoryChange, error) {
	return candidate.change, nil
}

func (exactMemoryChangeCandidate) memoryChangeCandidateVariant() {}

type memoryChangeSetCandidate struct {
	changes     []memoryChangeCandidate
	coordinates DiagnosticCoordinateIndex
}

func (candidate memoryChangeSetCandidate) bind(
	typeEnv typedmemory.TypeEnvRef,
) (typedmemory.MemoryChangeSet, error) {
	changes := make([]typedmemory.MemoryChange, 0, len(candidate.changes))
	for index, changeCandidate := range candidate.changes {
		change, err := changeCandidate.bind(typeEnv)
		if err != nil {
			path := fmt.Sprintf("$.change_set.changes[%d]", index)
			return typedmemory.MemoryChangeSet{}, invalidValue(path, err)
		}
		changes = append(changes, change)
	}
	changeSet, err := typedmemory.NewMemoryChangeSet(changes)
	if err != nil {
		return typedmemory.MemoryChangeSet{}, invalidValue("$.change_set.changes", err)
	}
	return changeSet, nil
}

type requestWire struct {
	ContractVersion string          `json:"contract_version"`
	Action          string          `json:"action"`
	Basis           json.RawMessage `json:"basis"`
	ChangeSet       json.RawMessage `json:"change_set"`
}

type basisWire struct {
	Kind string `json:"kind"`
}

type exactProjectBasisWire struct {
	Kind          string  `json:"kind"`
	TypeEnvDigest string  `json:"type_env_digest"`
	GraphRevision *uint64 `json:"graph_revision"`
}

type changeSetWire struct {
	Changes []json.RawMessage `json:"changes"`
}

type discriminatorWire struct {
	Kind string `json:"kind"`
}

func DecodeValidateRequest(payload []byte) (ValidateRequest, error) {
	if err := scanStrictJSON(payload); err != nil {
		return ValidateRequest{}, err
	}

	wire := requestWire{}
	if err := decodeStrict(payload, &wire, "$", "request"); err != nil {
		return ValidateRequest{}, err
	}
	if err := requireIdentifier(wire.ContractVersion, "$.contract_version"); err != nil {
		return ValidateRequest{}, err
	}
	if !validValidateContractVersion(wire.ContractVersion) {
		message := fmt.Sprintf(
			"must equal %q or %q",
			ContractVersionV1,
			ContractVersionV2,
		)
		return ValidateRequest{}, invalidContract("$.contract_version", message)
	}
	if err := requireIdentifier(wire.Action, "$.action"); err != nil {
		return ValidateRequest{}, err
	}
	if wire.Action != ActionValidate {
		message := fmt.Sprintf("must equal %q", ActionValidate)
		return ValidateRequest{}, invalidContract("$.action", message)
	}
	if len(wire.Basis) == 0 {
		return ValidateRequest{}, invalidContract("$.basis", "basis is required")
	}
	if len(wire.ChangeSet) == 0 {
		return ValidateRequest{}, invalidContract("$.change_set", "change_set is required")
	}

	basis, err := decodeBasis(wire.Basis)
	if err != nil {
		return ValidateRequest{}, err
	}
	changeSet, err := decodeChangeSet(wire.ChangeSet, wire.ContractVersion)
	if err != nil {
		return ValidateRequest{}, err
	}
	return ValidateRequest{
		proof:           decodedValidateRequestProof,
		contractVersion: wire.ContractVersion,
		basis:           basis,
		changeSet:       changeSet,
	}, nil
}

func decodeBasis(raw []byte) (BasisSelector, error) {
	kind, err := decodeDiscriminator(raw, "$.basis")
	if err != nil {
		return nil, err
	}
	switch BasisKind(kind) {
	case BasisBundledCandidateOpenWorld:
		wire := basisWire{}
		if err := decodeStrict(raw, &wire, "$.basis", "bundled open-world candidate selector"); err != nil {
			return nil, err
		}
		return BundledCandidateOpenWorldSelector{}, nil
	case BasisProjectCurrent:
		wire := basisWire{}
		if err := decodeStrict(raw, &wire, "$.basis", "current project selector"); err != nil {
			return nil, err
		}
		return ProjectCurrentSelector{}, nil
	case BasisExactProject:
		return decodeExactProjectBasis(raw)
	default:
		message := fmt.Sprintf("unknown basis selector %q", kind)
		return nil, invalidContract("$.basis.kind", message)
	}
}

func decodeExactProjectBasis(raw []byte) (BasisSelector, error) {
	wire := exactProjectBasisWire{}
	if err := decodeStrict(raw, &wire, "$.basis", "exact project selector"); err != nil {
		return nil, err
	}
	if wire.GraphRevision == nil {
		return nil, invalidContract("$.basis.graph_revision", "graph_revision is required")
	}
	digest, err := typedmemory.NewSHA256Digest(wire.TypeEnvDigest)
	if err != nil {
		return nil, invalidValue("$.basis.type_env_digest", err)
	}
	revision := typedmemory.NewGraphRevision(*wire.GraphRevision)
	selector := ExactProjectSelector{
		requestedTypeEnvDigest: digest,
		requestedGraphRevision: revision,
	}
	return selector, nil
}

func decodeChangeSet(
	raw []byte,
	contractVersion string,
) (memoryChangeSetCandidate, error) {
	wire := changeSetWire{}
	if err := decodeStrict(raw, &wire, "$.change_set", "change_set"); err != nil {
		return memoryChangeSetCandidate{}, err
	}
	if len(wire.Changes) == 0 {
		return memoryChangeSetCandidate{}, invalidContract("$.change_set.changes", "changes must be non-empty")
	}
	if len(wire.Changes) > MaximumChanges {
		message := fmt.Sprintf("changes exceed %d items", MaximumChanges)
		return memoryChangeSetCandidate{}, resourceLimit("$.change_set.changes", message)
	}

	changes := make([]memoryChangeCandidate, 0, len(wire.Changes))
	coordinates := make([]diagnosticChangeCoordinate, 0, len(wire.Changes))
	for index, rawChange := range wire.Changes {
		path := fmt.Sprintf("$.change_set.changes[%d]", index)
		change, err := decodeMemoryChange(rawChange, path, contractVersion)
		if err != nil {
			return memoryChangeSetCandidate{}, err
		}
		coordinate, err := diagnosticCoordinateForCandidate(change)
		if err != nil {
			return memoryChangeSetCandidate{}, invalidValue(path, err)
		}
		changes = append(changes, change)
		coordinates = append(coordinates, coordinate)
	}
	if err := validateCandidateIdentity(changes); err != nil {
		return memoryChangeSetCandidate{}, invalidValue("$.change_set.changes", err)
	}
	index := DiagnosticCoordinateIndex{changes: coordinates}
	if !index.validFor(len(changes)) {
		return memoryChangeSetCandidate{}, invalidValue(
			"$.change_set.changes",
			fmt.Errorf("strict diagnostic coordinate index is incomplete"),
		)
	}
	return memoryChangeSetCandidate{
		changes:     changes,
		coordinates: index,
	}, nil
}

type candidateReservation struct {
	space string
	key   string
}

func validateCandidateIdentity(changes []memoryChangeCandidate) error {
	seen := make(map[string]struct{})
	for _, candidate := range changes {
		reservations := candidateReservations(candidate)
		for _, reservation := range reservations {
			key := lengthPrefixedKey(reservation.space, reservation.key)
			if _, duplicate := seen[key]; duplicate {
				return fmt.Errorf("MemoryChangeSet repeats %s %q", reservation.space, reservation.key)
			}
			seen[key] = struct{}{}
		}
	}
	return nil
}

func candidateReservations(candidate memoryChangeCandidate) []candidateReservation {
	switch typed := candidate.(type) {
	case relationMemoryChangeCandidate:
		return []candidateReservation{{space: "assertion", key: typed.assertion.String()}}
	case relationalAssertionMemoryChangeCandidate:
		return []candidateReservation{{space: "assertion", key: typed.assertion.String()}}
	case exactMemoryChangeCandidate:
		return exactChangeReservations(typed.change)
	default:
		return nil
	}
}

func exactChangeReservations(change typedmemory.MemoryChange) []candidateReservation {
	switch typed := change.(type) {
	case typedmemory.DeclareEntity:
		return []candidateReservation{
			{space: "entity", key: typed.Entity().String()},
			{space: "batch-local reference", key: typed.LocalRef().String()},
		}
	case typedmemory.RetractAssertion:
		return []candidateReservation{{space: "assertion", key: typed.Assertion().String()}}
	case typedmemory.ApplyIdentityChange:
		return identityChangeReservations(typed.Change())
	default:
		return nil
	}
}

func identityChangeReservations(change typedmemory.IdentityChange) []candidateReservation {
	switch typed := change.(type) {
	case typedmemory.AdmitAlias:
		key := lengthPrefixedKey(typed.Context().String(), typed.Alias().String())
		return []candidateReservation{{space: "alias subject", key: key}}
	case typedmemory.SupersedeAlias:
		oldKey := lengthPrefixedKey(typed.Context().String(), typed.OldAlias().String())
		newKey := lengthPrefixedKey(typed.Context().String(), typed.Replacement().String())
		return []candidateReservation{
			{space: "alias subject", key: oldKey},
			{space: "alias subject", key: newKey},
		}
	case typedmemory.MergeEntities:
		return reconciliationReservations(typed.Context(), typed.Survivor(), typed.Merged())
	case typedmemory.SplitEntity:
		return reconciliationReservations(typed.Context(), typed.Source(), typed.Targets())
	default:
		return nil
	}
}

func reconciliationReservations(
	context typedmemory.BoundedContextRef,
	primary typedmemory.EntityID,
	others []typedmemory.EntityID,
) []candidateReservation {
	entities := make([]typedmemory.EntityID, 0, len(others)+1)
	entities = append(entities, primary)
	entities = append(entities, others...)
	reservations := make([]candidateReservation, 0, len(entities))
	for _, entity := range entities {
		key := lengthPrefixedKey(context.String(), entity.String())
		reservations = append(reservations, candidateReservation{space: "reconciliation subject", key: key})
	}
	return reservations
}

func lengthPrefixedKey(parts ...string) string {
	builder := strings.Builder{}
	for _, part := range parts {
		length := fmt.Sprintf("%d:", len(part))
		builder.WriteString(length)
		builder.WriteString(part)
	}
	return builder.String()
}

func decodeMemoryChange(
	raw []byte,
	path string,
	contractVersion string,
) (memoryChangeCandidate, error) {
	kind, err := decodeDiscriminator(raw, path)
	if err != nil {
		return nil, err
	}
	switch kind {
	case "declare_entity":
		return decodeDeclareEntity(raw, path)
	case "identity_change":
		return decodeIdentityMemoryChange(raw, path)
	case "instantiate_relation":
		if contractVersion != ContractVersionV1 {
			return nil, invalidContract(
				path+".kind",
				"instantiate_relation belongs only to frozen haft.memory.v1 history",
			)
		}
		return decodeInstantiateRelation(raw, path)
	case "assert_relation":
		if contractVersion != ContractVersionV2 {
			return nil, invalidContract(
				path+".kind",
				"assert_relation requires haft.memory.v2",
			)
		}
		return decodeAssertRelation(raw, path)
	case "retract_assertion":
		return decodeRetractAssertion(raw, path)
	default:
		message := fmt.Sprintf("unknown MemoryChange kind %q", kind)
		return nil, invalidContract(path+".kind", message)
	}
}

type declareEntityWire struct {
	Kind       string `json:"kind"`
	EntityID   string `json:"entity_id"`
	LocalRef   string `json:"local_ref"`
	Context    string `json:"context"`
	Label      string `json:"label"`
	Provenance string `json:"provenance"`
}

func decodeDeclareEntity(raw []byte, path string) (memoryChangeCandidate, error) {
	wire := declareEntityWire{}
	if err := decodeStrict(raw, &wire, path, "declare_entity"); err != nil {
		return nil, err
	}
	entity, err := parseEntityID(wire.EntityID, path+".entity_id")
	if err != nil {
		return nil, err
	}
	localRef, err := parseBatchLocalRef(wire.LocalRef, path+".local_ref")
	if err != nil {
		return nil, err
	}
	context, err := parseContext(wire.Context, path+".context")
	if err != nil {
		return nil, err
	}
	if err := requireText(wire.Label, path+".label"); err != nil {
		return nil, err
	}
	label, err := typedmemory.NewEntityLabel(wire.Label)
	if err != nil {
		return nil, invalidValue(path+".label", err)
	}
	provenance, err := parseProvenance(wire.Provenance, path+".provenance")
	if err != nil {
		return nil, err
	}
	change, err := typedmemory.NewDeclareEntity(entity, localRef, context, label, provenance)
	if err != nil {
		return nil, invalidValue(path, err)
	}
	return exactMemoryChangeCandidate{change: change}, nil
}

type identityMemoryChangeWire struct {
	Kind   string          `json:"kind"`
	Change json.RawMessage `json:"change"`
}

func decodeIdentityMemoryChange(raw []byte, path string) (memoryChangeCandidate, error) {
	wire := identityMemoryChangeWire{}
	if err := decodeStrict(raw, &wire, path, "identity_change"); err != nil {
		return nil, err
	}
	if len(wire.Change) == 0 {
		return nil, invalidContract(path+".change", "identity change is required")
	}
	identity, err := decodeIdentityChange(wire.Change, path+".change")
	if err != nil {
		return nil, err
	}
	change, err := typedmemory.NewApplyIdentityChange(identity)
	if err != nil {
		return nil, invalidValue(path+".change", err)
	}
	return exactMemoryChangeCandidate{change: change}, nil
}

func decodeIdentityChange(raw []byte, path string) (typedmemory.IdentityChange, error) {
	kind, err := decodeDiscriminator(raw, path)
	if err != nil {
		return nil, err
	}
	switch kind {
	case "admit_alias":
		return decodeAdmitAlias(raw, path)
	case "supersede_alias":
		return decodeSupersedeAlias(raw, path)
	case "merge_entities":
		return decodeMergeEntities(raw, path)
	case "split_entity":
		return decodeSplitEntity(raw, path)
	default:
		message := fmt.Sprintf("unknown IdentityChange kind %q", kind)
		return nil, invalidContract(path+".kind", message)
	}
}

type admitAliasWire struct {
	Kind       string `json:"kind"`
	EntityID   string `json:"entity_id"`
	Alias      string `json:"alias"`
	Context    string `json:"context"`
	Provenance string `json:"provenance"`
}

func decodeAdmitAlias(raw []byte, path string) (typedmemory.IdentityChange, error) {
	wire := admitAliasWire{}
	if err := decodeStrict(raw, &wire, path, "admit_alias"); err != nil {
		return nil, err
	}
	entity, err := parseEntityID(wire.EntityID, path+".entity_id")
	if err != nil {
		return nil, err
	}
	alias, err := parseAlias(wire.Alias, path+".alias")
	if err != nil {
		return nil, err
	}
	context, err := parseContext(wire.Context, path+".context")
	if err != nil {
		return nil, err
	}
	provenance, err := parseProvenance(wire.Provenance, path+".provenance")
	if err != nil {
		return nil, err
	}
	change, err := typedmemory.NewAdmitAlias(entity, alias, context, provenance)
	if err != nil {
		return nil, invalidValue(path, err)
	}
	return change, nil
}

type supersedeAliasWire struct {
	Kind        string `json:"kind"`
	EntityID    string `json:"entity_id"`
	OldAlias    string `json:"old_alias"`
	Replacement string `json:"replacement"`
	Context     string `json:"context"`
	Provenance  string `json:"provenance"`
}

func decodeSupersedeAlias(raw []byte, path string) (typedmemory.IdentityChange, error) {
	wire := supersedeAliasWire{}
	if err := decodeStrict(raw, &wire, path, "supersede_alias"); err != nil {
		return nil, err
	}
	entity, err := parseEntityID(wire.EntityID, path+".entity_id")
	if err != nil {
		return nil, err
	}
	oldAlias, err := parseAlias(wire.OldAlias, path+".old_alias")
	if err != nil {
		return nil, err
	}
	replacement, err := parseAlias(wire.Replacement, path+".replacement")
	if err != nil {
		return nil, err
	}
	context, err := parseContext(wire.Context, path+".context")
	if err != nil {
		return nil, err
	}
	provenance, err := parseProvenance(wire.Provenance, path+".provenance")
	if err != nil {
		return nil, err
	}
	change, err := typedmemory.NewSupersedeAlias(entity, oldAlias, replacement, context, provenance)
	if err != nil {
		return nil, invalidValue(path, err)
	}
	return change, nil
}

type mergeEntitiesWire struct {
	Kind     string   `json:"kind"`
	Survivor string   `json:"survivor"`
	Merged   []string `json:"merged"`
	Context  string   `json:"context"`
	Basis    string   `json:"basis"`
}

func decodeMergeEntities(raw []byte, path string) (typedmemory.IdentityChange, error) {
	wire := mergeEntitiesWire{}
	if err := decodeStrict(raw, &wire, path, "merge_entities"); err != nil {
		return nil, err
	}
	survivor, err := parseEntityID(wire.Survivor, path+".survivor")
	if err != nil {
		return nil, err
	}
	merged, err := parseEntityIDs(wire.Merged, path+".merged")
	if err != nil {
		return nil, err
	}
	context, err := parseContext(wire.Context, path+".context")
	if err != nil {
		return nil, err
	}
	basis, err := parseReconciliationBasis(wire.Basis, path+".basis")
	if err != nil {
		return nil, err
	}
	change, err := typedmemory.NewMergeEntities(survivor, merged, context, basis)
	if err != nil {
		return nil, invalidValue(path, err)
	}
	return change, nil
}

type splitEntityWire struct {
	Kind    string   `json:"kind"`
	Source  string   `json:"source"`
	Targets []string `json:"targets"`
	Context string   `json:"context"`
	Basis   string   `json:"basis"`
}

func decodeSplitEntity(raw []byte, path string) (typedmemory.IdentityChange, error) {
	wire := splitEntityWire{}
	if err := decodeStrict(raw, &wire, path, "split_entity"); err != nil {
		return nil, err
	}
	source, err := parseEntityID(wire.Source, path+".source")
	if err != nil {
		return nil, err
	}
	targets, err := parseEntityIDs(wire.Targets, path+".targets")
	if err != nil {
		return nil, err
	}
	context, err := parseContext(wire.Context, path+".context")
	if err != nil {
		return nil, err
	}
	basis, err := parseReconciliationBasis(wire.Basis, path+".basis")
	if err != nil {
		return nil, err
	}
	change, err := typedmemory.NewSplitEntity(source, targets, context, basis)
	if err != nil {
		return nil, invalidValue(path, err)
	}
	return change, nil
}

type instantiateRelationWire struct {
	Kind         string            `json:"kind"`
	AssertionID  string            `json:"assertion_id"`
	SignatureID  string            `json:"signature_id"`
	ContextSlice json.RawMessage   `json:"context_slice"`
	Bindings     []json.RawMessage `json:"bindings"`
	Provenance   string            `json:"provenance"`
}

type assertRelationWire struct {
	Kind         string            `json:"kind"`
	AssertionID  string            `json:"assertion_id"`
	SignatureID  string            `json:"signature_id"`
	ContextSlice json.RawMessage   `json:"context_slice"`
	Modality     json.RawMessage   `json:"modality"`
	Bindings     []json.RawMessage `json:"bindings"`
	Provenance   string            `json:"provenance"`
}

type assertionModalityWire struct {
	Kind string `json:"kind"`
}

type contextSliceWire struct {
	Context              string                    `json:"context"`
	StandardPins         []versionedPinWire        `json:"standard_pins"`
	EnvironmentSelectors []environmentSelectorWire `json:"environment_selectors"`
	VocabularyPins       []versionedPinWire        `json:"vocabulary_pins"`
	RoleSetPins          []versionedPinWire        `json:"role_set_pins"`
	GammaTime            json.RawMessage           `json:"gamma_time"`
}

type versionedPinWire struct {
	Reference string `json:"reference"`
	Edition   string `json:"edition"`
	Digest    string `json:"digest"`
}

type environmentSelectorWire struct {
	Key          string `json:"key"`
	Value        string `json:"value"`
	SourceDigest string `json:"source_digest"`
}

type gammaPointWire struct {
	Kind string `json:"kind"`
	At   string `json:"at"`
}

type gammaWindowWire struct {
	Kind          string `json:"kind"`
	Start         string `json:"start"`
	End           string `json:"end"`
	StartBoundary string `json:"start_boundary"`
	EndBoundary   string `json:"end_boundary"`
}

type gammaPolicyApplicationWire struct {
	Kind             string          `json:"kind"`
	PolicyRef        string          `json:"policy_ref"`
	PolicyEdition    string          `json:"policy_edition"`
	PolicyDigest     string          `json:"policy_digest"`
	EvaluationAnchor json.RawMessage `json:"evaluation_anchor"`
	Resolved         json.RawMessage `json:"resolved"`
}

type slotBindingWire struct {
	SlotKind string            `json:"slot_kind"`
	Fillers  []json.RawMessage `json:"fillers"`
}

type relationMemoryChangeCandidate struct {
	assertion  typedmemory.AssertionID
	signature  typedmemory.SignatureID
	slice      typedmemory.ContextSlice
	bindings   []slotBindingCandidate
	provenance typedmemory.ProvenanceRef
}

func (candidate relationMemoryChangeCandidate) bind(
	typeEnv typedmemory.TypeEnvRef,
) (typedmemory.MemoryChange, error) {
	signature, err := typedmemory.NewRelationSignatureRef(typeEnv, candidate.signature)
	if err != nil {
		return nil, err
	}
	bindings, err := bindSlotBindingCandidates(typeEnv, candidate.bindings)
	if err != nil {
		return nil, err
	}
	relation, err := typedmemory.NewRelationInstantiation(
		candidate.assertion,
		signature,
		candidate.slice,
		bindings,
		candidate.provenance,
	)
	if err != nil {
		return nil, err
	}
	return typedmemory.NewInstantiateRelation(relation)
}

func (relationMemoryChangeCandidate) memoryChangeCandidateVariant() {}

type relationalAssertionMemoryChangeCandidate struct {
	assertion  typedmemory.AssertionID
	signature  typedmemory.SignatureID
	slice      typedmemory.ContextSlice
	modality   typedmemory.AssertionModality
	bindings   []slotBindingCandidate
	provenance typedmemory.ProvenanceRef
}

func (candidate relationalAssertionMemoryChangeCandidate) bind(
	typeEnv typedmemory.TypeEnvRef,
) (typedmemory.MemoryChange, error) {
	signature, err := typedmemory.NewRelationSignatureRef(typeEnv, candidate.signature)
	if err != nil {
		return nil, err
	}
	bindings, err := bindSlotBindingCandidates(typeEnv, candidate.bindings)
	if err != nil {
		return nil, err
	}
	assertion, err := typedmemory.NewRelationalAssertionCandidate(
		typedmemory.RelationalAssertionCandidateInput{
			Assertion:  candidate.assertion,
			Signature:  signature,
			Slice:      candidate.slice,
			Modality:   candidate.modality,
			Bindings:   bindings,
			Provenance: candidate.provenance,
		},
	)
	if err != nil {
		return nil, err
	}
	return typedmemory.NewAssertRelation(assertion)
}

func (relationalAssertionMemoryChangeCandidate) memoryChangeCandidateVariant() {}

func bindSlotBindingCandidates(
	typeEnv typedmemory.TypeEnvRef,
	candidates []slotBindingCandidate,
) ([]typedmemory.CandidateSlotBinding, error) {
	bindings := make([]typedmemory.CandidateSlotBinding, 0, len(candidates))
	for _, candidate := range candidates {
		binding, err := candidate.bind(typeEnv)
		if err != nil {
			return nil, err
		}
		bindings = append(bindings, binding)
	}
	return bindings, nil
}

type slotBindingCandidate struct {
	slotKind typedmemory.SlotKindID
	fillers  []slotFillerCandidate
}

func (candidate slotBindingCandidate) bind(
	typeEnv typedmemory.TypeEnvRef,
) (typedmemory.CandidateSlotBinding, error) {
	fillers := make([]typedmemory.CandidateSlotFiller, 0, len(candidate.fillers))
	for _, fillerCandidate := range candidate.fillers {
		filler, err := fillerCandidate.bind(typeEnv)
		if err != nil {
			return typedmemory.CandidateSlotBinding{}, err
		}
		fillers = append(fillers, filler)
	}
	return typedmemory.NewCandidateSlotBinding(candidate.slotKind, fillers)
}

type slotFillerCandidate interface {
	bind(typedmemory.TypeEnvRef) (typedmemory.CandidateSlotFiller, error)
	slotFillerCandidateVariant()
}

func decodeInstantiateRelation(
	raw []byte,
	path string,
) (memoryChangeCandidate, error) {
	wire := instantiateRelationWire{}
	if err := decodeStrict(raw, &wire, path, "instantiate_relation"); err != nil {
		return nil, err
	}
	assertion, err := parseAssertionID(wire.AssertionID, path+".assertion_id")
	if err != nil {
		return nil, err
	}
	signatureID, err := parseSignatureID(wire.SignatureID, path+".signature_id")
	if err != nil {
		return nil, err
	}
	slice, err := decodeContextSlice(wire.ContextSlice, path+".context_slice")
	if err != nil {
		return nil, err
	}
	bindings, err := decodeSlotBindings(wire.Bindings, path+".bindings")
	if err != nil {
		return nil, err
	}
	provenance, err := parseProvenance(wire.Provenance, path+".provenance")
	if err != nil {
		return nil, err
	}
	candidate := relationMemoryChangeCandidate{
		assertion:  assertion,
		signature:  signatureID,
		slice:      slice,
		bindings:   bindings,
		provenance: provenance,
	}
	return candidate, nil
}

func decodeAssertRelation(
	raw []byte,
	path string,
) (memoryChangeCandidate, error) {
	wire := assertRelationWire{}
	if err := decodeStrict(raw, &wire, path, "assert_relation"); err != nil {
		return nil, err
	}
	assertion, err := parseAssertionID(wire.AssertionID, path+".assertion_id")
	if err != nil {
		return nil, err
	}
	signatureID, err := parseSignatureID(wire.SignatureID, path+".signature_id")
	if err != nil {
		return nil, err
	}
	slice, err := decodeContextSlice(wire.ContextSlice, path+".context_slice")
	if err != nil {
		return nil, err
	}
	modality, err := decodeAssertionModality(wire.Modality, path+".modality")
	if err != nil {
		return nil, err
	}
	bindings, err := decodeSlotBindings(wire.Bindings, path+".bindings")
	if err != nil {
		return nil, err
	}
	provenance, err := parseProvenance(wire.Provenance, path+".provenance")
	if err != nil {
		return nil, err
	}
	return relationalAssertionMemoryChangeCandidate{
		assertion:  assertion,
		signature:  signatureID,
		slice:      slice,
		modality:   modality,
		bindings:   bindings,
		provenance: provenance,
	}, nil
}

func decodeAssertionModality(
	raw []byte,
	path string,
) (typedmemory.AssertionModality, error) {
	if len(raw) == 0 {
		return nil, invalidContract(path, "modality is required")
	}
	kind, err := decodeDiscriminator(raw, path)
	if err != nil {
		return nil, err
	}
	wire := assertionModalityWire{}
	if err := decodeStrict(raw, &wire, path, "assertion modality"); err != nil {
		return nil, err
	}
	switch typedmemory.AssertionModalityKind(kind) {
	case typedmemory.AssertionModalityAffirmsObtaining:
		return typedmemory.NewAffirmsObtaining(), nil
	case typedmemory.AssertionModalityDeniesObtaining:
		return typedmemory.NewDeniesObtaining(), nil
	case typedmemory.AssertionModalityObtainingUnknown:
		return typedmemory.NewObtainingUnknown(), nil
	default:
		message := fmt.Sprintf("unknown assertion modality %q", kind)
		return nil, invalidContract(path+".kind", message)
	}
}

func decodeContextSlice(raw []byte, path string) (typedmemory.ContextSlice, error) {
	if len(raw) == 0 {
		return typedmemory.ContextSlice{}, invalidContract(path, "context_slice is required")
	}
	wire := contextSliceWire{}
	if err := decodeStrict(raw, &wire, path, "context_slice"); err != nil {
		return typedmemory.ContextSlice{}, err
	}
	context, err := parseContext(wire.Context, path+".context")
	if err != nil {
		return typedmemory.ContextSlice{}, err
	}
	standards, err := decodeStandardPins(wire.StandardPins, path+".standard_pins")
	if err != nil {
		return typedmemory.ContextSlice{}, err
	}
	environment, err := decodeEnvironmentSelectors(
		wire.EnvironmentSelectors,
		path+".environment_selectors",
	)
	if err != nil {
		return typedmemory.ContextSlice{}, err
	}
	vocabularies, err := decodeVocabularyPins(wire.VocabularyPins, path+".vocabulary_pins")
	if err != nil {
		return typedmemory.ContextSlice{}, err
	}
	roleSets, err := decodeRoleSetPins(wire.RoleSetPins, path+".role_set_pins")
	if err != nil {
		return typedmemory.ContextSlice{}, err
	}
	gamma, err := decodeGammaTime(wire.GammaTime, path+".gamma_time")
	if err != nil {
		return typedmemory.ContextSlice{}, err
	}
	input := typedmemory.ContextSliceInput{
		Context:              context,
		StandardPins:         standards,
		EnvironmentSelectors: environment,
		VocabularyPins:       vocabularies,
		RoleSetPins:          roleSets,
		GammaTime:            gamma,
	}
	slice, err := typedmemory.NewContextSlice(input)
	if err != nil {
		return typedmemory.ContextSlice{}, invalidValue(path, err)
	}
	return slice, nil
}

func decodeStandardPins(
	wires []versionedPinWire,
	path string,
) ([]typedmemory.StandardPin, error) {
	if err := checkContextSliceItemCount(len(wires), path); err != nil {
		return nil, err
	}
	result := make([]typedmemory.StandardPin, 0, len(wires))
	for index, wire := range wires {
		itemPath := fmt.Sprintf("%s[%d]", path, index)
		reference, edition, digest, err := decodeVersionedPin(wire, itemPath)
		if err != nil {
			return nil, err
		}
		pin, err := typedmemory.NewStandardPin(reference, edition, digest)
		if err != nil {
			return nil, invalidValue(itemPath, err)
		}
		result = append(result, pin)
	}
	return result, nil
}

func decodeVocabularyPins(
	wires []versionedPinWire,
	path string,
) ([]typedmemory.VocabularyPin, error) {
	if err := checkContextSliceItemCount(len(wires), path); err != nil {
		return nil, err
	}
	result := make([]typedmemory.VocabularyPin, 0, len(wires))
	for index, wire := range wires {
		itemPath := fmt.Sprintf("%s[%d]", path, index)
		reference, edition, digest, err := decodeVersionedPin(wire, itemPath)
		if err != nil {
			return nil, err
		}
		pin, err := typedmemory.NewVocabularyPin(reference, edition, digest)
		if err != nil {
			return nil, invalidValue(itemPath, err)
		}
		result = append(result, pin)
	}
	return result, nil
}

func decodeRoleSetPins(
	wires []versionedPinWire,
	path string,
) ([]typedmemory.RoleSetPin, error) {
	if err := checkContextSliceItemCount(len(wires), path); err != nil {
		return nil, err
	}
	result := make([]typedmemory.RoleSetPin, 0, len(wires))
	for index, wire := range wires {
		itemPath := fmt.Sprintf("%s[%d]", path, index)
		reference, edition, digest, err := decodeVersionedPin(wire, itemPath)
		if err != nil {
			return nil, err
		}
		pin, err := typedmemory.NewRoleSetPin(reference, edition, digest)
		if err != nil {
			return nil, invalidValue(itemPath, err)
		}
		result = append(result, pin)
	}
	return result, nil
}

func decodeVersionedPin(
	wire versionedPinWire,
	path string,
) (typedmemory.CarrierRef, typedmemory.CarrierEdition, typedmemory.SHA256Digest, error) {
	reference, err := parseCarrierRef(wire.Reference, path+".reference")
	if err != nil {
		return typedmemory.CarrierRef{}, typedmemory.CarrierEdition{}, typedmemory.SHA256Digest{}, err
	}
	edition, err := parseCarrierEdition(wire.Edition, path+".edition")
	if err != nil {
		return typedmemory.CarrierRef{}, typedmemory.CarrierEdition{}, typedmemory.SHA256Digest{}, err
	}
	digest, err := parseSHA256Digest(wire.Digest, path+".digest")
	if err != nil {
		return typedmemory.CarrierRef{}, typedmemory.CarrierEdition{}, typedmemory.SHA256Digest{}, err
	}
	return reference, edition, digest, nil
}

func decodeEnvironmentSelectors(
	wires []environmentSelectorWire,
	path string,
) ([]typedmemory.EnvironmentSelector, error) {
	if len(wires) > MaximumEnvironmentSelectors {
		message := fmt.Sprintf("environment selectors exceed %d items", MaximumEnvironmentSelectors)
		return nil, resourceLimit(path, message)
	}
	result := make([]typedmemory.EnvironmentSelector, 0, len(wires))
	for index, wire := range wires {
		itemPath := fmt.Sprintf("%s[%d]", path, index)
		if err := requireIdentifier(wire.Key, itemPath+".key"); err != nil {
			return nil, err
		}
		key, err := typedmemory.NewEnvironmentSelectorKey(wire.Key)
		if err != nil {
			return nil, invalidValue(itemPath+".key", err)
		}
		if err := requireIdentifier(wire.Value, itemPath+".value"); err != nil {
			return nil, err
		}
		value, err := typedmemory.NewEnvironmentSelectorValue(wire.Value)
		if err != nil {
			return nil, invalidValue(itemPath+".value", err)
		}
		digest, err := parseSHA256Digest(wire.SourceDigest, itemPath+".source_digest")
		if err != nil {
			return nil, err
		}
		selector, err := typedmemory.NewEnvironmentSelector(key, value, digest)
		if err != nil {
			return nil, invalidValue(itemPath, err)
		}
		result = append(result, selector)
	}
	return result, nil
}

func checkContextSliceItemCount(count int, path string) error {
	if count <= MaximumContextPins {
		return nil
	}
	message := fmt.Sprintf("ContextSlice pins exceed %d items", MaximumContextPins)
	return resourceLimit(path, message)
}

func decodeGammaTime(raw []byte, path string) (typedmemory.GammaTimeSelector, error) {
	if len(raw) == 0 {
		return nil, invalidContract(path, "gamma_time is required")
	}
	kind, err := decodeDiscriminator(raw, path)
	if err != nil {
		return nil, err
	}
	switch kind {
	case "point":
		return decodeGammaPoint(raw, path)
	case "window":
		return decodeGammaWindow(raw, path)
	case "policy_application":
		return decodeGammaPolicyApplication(raw, path)
	default:
		message := fmt.Sprintf("unknown Gamma_time selector %q", kind)
		return nil, invalidContract(path+".kind", message)
	}
}

func decodeResolvedGammaTime(
	raw []byte,
	path string,
) (typedmemory.ResolvedGammaTimeSelector, error) {
	if len(raw) == 0 {
		return nil, invalidContract(path, "resolved Gamma_time is required")
	}
	kind, err := decodeDiscriminator(raw, path)
	if err != nil {
		return nil, err
	}
	switch kind {
	case "point":
		return decodeGammaPoint(raw, path)
	case "window":
		return decodeGammaWindow(raw, path)
	default:
		message := fmt.Sprintf("resolved Gamma_time cannot use %q", kind)
		return nil, invalidContract(path+".kind", message)
	}
}

func decodeGammaPoint(raw []byte, path string) (typedmemory.GammaPoint, error) {
	wire := gammaPointWire{}
	if err := decodeStrict(raw, &wire, path, "Gamma point"); err != nil {
		return typedmemory.GammaPoint{}, err
	}
	instant, err := parseGammaInstant(wire.At, path+".at")
	if err != nil {
		return typedmemory.GammaPoint{}, err
	}
	point, err := typedmemory.NewGammaPoint(instant)
	if err != nil {
		return typedmemory.GammaPoint{}, invalidValue(path, err)
	}
	return point, nil
}

func decodeGammaWindow(raw []byte, path string) (typedmemory.GammaWindow, error) {
	wire := gammaWindowWire{}
	if err := decodeStrict(raw, &wire, path, "Gamma window"); err != nil {
		return typedmemory.GammaWindow{}, err
	}
	start, err := parseGammaInstant(wire.Start, path+".start")
	if err != nil {
		return typedmemory.GammaWindow{}, err
	}
	end, err := parseGammaInstant(wire.End, path+".end")
	if err != nil {
		return typedmemory.GammaWindow{}, err
	}
	startBoundary, err := parseGammaBoundary(wire.StartBoundary, path+".start_boundary")
	if err != nil {
		return typedmemory.GammaWindow{}, err
	}
	endBoundary, err := parseGammaBoundary(wire.EndBoundary, path+".end_boundary")
	if err != nil {
		return typedmemory.GammaWindow{}, err
	}
	window, err := typedmemory.NewGammaWindow(start, end, startBoundary, endBoundary)
	if err != nil {
		return typedmemory.GammaWindow{}, invalidValue(path, err)
	}
	return window, nil
}

func decodeGammaPolicyApplication(
	raw []byte,
	path string,
) (typedmemory.GammaPolicyApplication, error) {
	wire := gammaPolicyApplicationWire{}
	if err := decodeStrict(raw, &wire, path, "Gamma policy application"); err != nil {
		return typedmemory.GammaPolicyApplication{}, err
	}
	policyRef, err := parseCarrierRef(wire.PolicyRef, path+".policy_ref")
	if err != nil {
		return typedmemory.GammaPolicyApplication{}, err
	}
	edition, err := parseCarrierEdition(wire.PolicyEdition, path+".policy_edition")
	if err != nil {
		return typedmemory.GammaPolicyApplication{}, err
	}
	digest, err := parseSHA256Digest(wire.PolicyDigest, path+".policy_digest")
	if err != nil {
		return typedmemory.GammaPolicyApplication{}, err
	}
	anchor, err := decodeGammaPoint(wire.EvaluationAnchor, path+".evaluation_anchor")
	if err != nil {
		return typedmemory.GammaPolicyApplication{}, err
	}
	resolved, err := decodeResolvedGammaTime(wire.Resolved, path+".resolved")
	if err != nil {
		return typedmemory.GammaPolicyApplication{}, err
	}
	application, err := typedmemory.NewGammaPolicyApplication(
		policyRef,
		edition,
		digest,
		anchor,
		resolved,
	)
	if err != nil {
		return typedmemory.GammaPolicyApplication{}, invalidValue(path, err)
	}
	return application, nil
}

func parseGammaInstant(raw string, path string) (time.Time, error) {
	if err := requireIdentifier(raw, path); err != nil {
		return time.Time{}, err
	}
	instant, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}, invalidContract(path, "must be an exact RFC3339 timestamp")
	}
	return instant, nil
}

func parseGammaBoundary(raw string, path string) (typedmemory.GammaBoundary, error) {
	switch raw {
	case typedmemory.GammaBoundaryInclusive.String():
		return typedmemory.GammaBoundaryInclusive, nil
	case typedmemory.GammaBoundaryExclusive.String():
		return typedmemory.GammaBoundaryExclusive, nil
	default:
		return 0, invalidContract(path, "must equal inclusive or exclusive")
	}
}

func decodeSlotBindings(
	rawBindings []json.RawMessage,
	path string,
) ([]slotBindingCandidate, error) {
	if len(rawBindings) == 0 {
		return nil, invalidContract(path, "bindings must be non-empty")
	}
	if len(rawBindings) > MaximumSlotBindings {
		message := fmt.Sprintf("bindings exceed %d items", MaximumSlotBindings)
		return nil, resourceLimit(path, message)
	}
	bindings := make([]slotBindingCandidate, 0, len(rawBindings))
	seen := make(map[string]struct{}, len(rawBindings))
	for index, rawBinding := range rawBindings {
		bindingPath := fmt.Sprintf("%s[%d]", path, index)
		binding, err := decodeSlotBinding(rawBinding, bindingPath)
		if err != nil {
			return nil, err
		}
		key := binding.slotKind.String()
		if _, duplicate := seen[key]; duplicate {
			message := fmt.Sprintf("duplicate slot binding %q", key)
			return nil, invalidContract(bindingPath+".slot_kind", message)
		}
		seen[key] = struct{}{}
		bindings = append(bindings, binding)
	}
	return bindings, nil
}

func decodeSlotBinding(
	raw []byte,
	path string,
) (slotBindingCandidate, error) {
	wire := slotBindingWire{}
	if err := decodeStrict(raw, &wire, path, "slot binding"); err != nil {
		return slotBindingCandidate{}, err
	}
	slotKind, err := parseSlotKindID(wire.SlotKind, path+".slot_kind")
	if err != nil {
		return slotBindingCandidate{}, err
	}
	if len(wire.Fillers) == 0 {
		return slotBindingCandidate{}, invalidContract(path+".fillers", "fillers must be non-empty")
	}
	if len(wire.Fillers) > MaximumFillersPerSlot {
		message := fmt.Sprintf("fillers exceed %d items", MaximumFillersPerSlot)
		return slotBindingCandidate{}, resourceLimit(path+".fillers", message)
	}
	fillers := make([]slotFillerCandidate, 0, len(wire.Fillers))
	for index, rawFiller := range wire.Fillers {
		fillerPath := fmt.Sprintf("%s.fillers[%d]", path, index)
		filler, err := decodeCandidateFiller(rawFiller, fillerPath)
		if err != nil {
			return slotBindingCandidate{}, err
		}
		fillers = append(fillers, filler)
	}
	return slotBindingCandidate{slotKind: slotKind, fillers: fillers}, nil
}

func decodeCandidateFiller(
	raw []byte,
	path string,
) (slotFillerCandidate, error) {
	kind, err := decodeDiscriminator(raw, path)
	if err != nil {
		return nil, err
	}
	switch kind {
	case "by_reference":
		return decodeReferenceFiller(raw, path)
	case "by_value":
		return decodeValueFiller(raw, path)
	default:
		message := fmt.Sprintf("unknown candidate filler kind %q", kind)
		return nil, invalidContract(path+".kind", message)
	}
}

type referenceFillerWire struct {
	Kind      string          `json:"kind"`
	Reference json.RawMessage `json:"reference"`
}

type referenceFillerCandidate struct {
	reference strongRefCandidate
}

func (candidate referenceFillerCandidate) bind(
	typeEnv typedmemory.TypeEnvRef,
) (typedmemory.CandidateSlotFiller, error) {
	reference, err := candidate.reference.bind(typeEnv)
	if err != nil {
		return nil, err
	}
	return typedmemory.NewByReferenceCandidate(reference)
}

func (referenceFillerCandidate) slotFillerCandidateVariant() {}

type strongRefCandidate interface {
	bind(typedmemory.TypeEnvRef) (typedmemory.StrongRef, error)
	strongRefCandidateVariant()
}

type persistedRefCandidate struct {
	refKind typedmemory.RefKindID
	id      typedmemory.ReferenceID
}

func (candidate persistedRefCandidate) bind(
	typeEnv typedmemory.TypeEnvRef,
) (typedmemory.StrongRef, error) {
	refKind, err := typedmemory.NewRefKindRef(typeEnv, candidate.refKind)
	if err != nil {
		return nil, err
	}
	return typedmemory.NewPersistedRef(refKind, candidate.id)
}

func (persistedRefCandidate) strongRefCandidateVariant() {}

type localRefCandidate struct {
	refKind  typedmemory.RefKindID
	localRef typedmemory.BatchLocalRef
}

func (candidate localRefCandidate) bind(
	typeEnv typedmemory.TypeEnvRef,
) (typedmemory.StrongRef, error) {
	refKind, err := typedmemory.NewRefKindRef(typeEnv, candidate.refKind)
	if err != nil {
		return nil, err
	}
	return typedmemory.NewLocalRef(refKind, candidate.localRef)
}

func (localRefCandidate) strongRefCandidateVariant() {}

func decodeReferenceFiller(
	raw []byte,
	path string,
) (slotFillerCandidate, error) {
	wire := referenceFillerWire{}
	if err := decodeStrict(raw, &wire, path, "by_reference filler"); err != nil {
		return nil, err
	}
	if len(wire.Reference) == 0 {
		return nil, invalidContract(path+".reference", "reference is required")
	}
	reference, err := decodeStrongRef(wire.Reference, path+".reference")
	if err != nil {
		return nil, err
	}
	return referenceFillerCandidate{reference: reference}, nil
}

type persistedRefWire struct {
	Kind    string `json:"kind"`
	RefKind string `json:"ref_kind"`
	ID      string `json:"id"`
}

type localRefWire struct {
	Kind     string `json:"kind"`
	RefKind  string `json:"ref_kind"`
	LocalRef string `json:"local_ref"`
}

func decodeStrongRef(
	raw []byte,
	path string,
) (strongRefCandidate, error) {
	kind, err := decodeDiscriminator(raw, path)
	if err != nil {
		return nil, err
	}
	if kind == "persisted" {
		wire := persistedRefWire{}
		if err := decodeStrict(raw, &wire, path, "persisted reference"); err != nil {
			return nil, err
		}
		refKind, err := parseRefKindID(wire.RefKind, path+".ref_kind")
		if err != nil {
			return nil, err
		}
		referenceID, err := parseReferenceID(wire.ID, path+".id")
		if err != nil {
			return nil, err
		}
		return persistedRefCandidate{refKind: refKind, id: referenceID}, nil
	}
	if kind == "local" {
		wire := localRefWire{}
		if err := decodeStrict(raw, &wire, path, "local reference"); err != nil {
			return nil, err
		}
		refKind, err := parseRefKindID(wire.RefKind, path+".ref_kind")
		if err != nil {
			return nil, err
		}
		localRef, err := parseBatchLocalRef(wire.LocalRef, path+".local_ref")
		if err != nil {
			return nil, err
		}
		return localRefCandidate{refKind: refKind, localRef: localRef}, nil
	}
	message := fmt.Sprintf("unknown strong-reference kind %q", kind)
	return nil, invalidContract(path+".kind", message)
}

type valueFillerWire struct {
	Kind  string          `json:"kind"`
	Value json.RawMessage `json:"value"`
}

type typedValueCandidateWire struct {
	ValueKind      string          `json:"value_kind"`
	ValueShape     json.RawMessage `json:"value_shape"`
	Codec          json.RawMessage `json:"codec"`
	InputBase64    string          `json:"input_base64"`
	AssertedDigest json.RawMessage `json:"asserted_digest"`
}

type valueShapeWire struct {
	ID     string `json:"id"`
	Digest string `json:"digest"`
}

type codecWire struct {
	ID                  string `json:"id"`
	Version             string `json:"version"`
	SpecificationDigest string `json:"specification_digest"`
}

type valueFillerCandidate struct {
	value typedValueCandidate
}

func (candidate valueFillerCandidate) bind(
	typeEnv typedmemory.TypeEnvRef,
) (typedmemory.CandidateSlotFiller, error) {
	value, err := candidate.value.bind(typeEnv)
	if err != nil {
		return nil, err
	}
	return typedmemory.NewByValueCandidate(value)
}

func (valueFillerCandidate) slotFillerCandidateVariant() {}

type typedValueCandidate struct {
	valueKind      typedmemory.KindID
	valueShape     typedmemory.ValueShapeRef
	codec          typedmemory.CodecRef
	inputBytes     []byte
	assertedDigest typedmemory.AssertedTypedValueDigest
}

func (candidate typedValueCandidate) bind(
	typeEnv typedmemory.TypeEnvRef,
) (typedmemory.TypedValueCandidate, error) {
	valueKind, err := typedmemory.NewValueKindRef(typeEnv, candidate.valueKind)
	if err != nil {
		return typedmemory.TypedValueCandidate{}, err
	}
	return typedmemory.NewTypedValueCandidate(
		valueKind,
		candidate.valueShape,
		candidate.codec,
		candidate.inputBytes,
		candidate.assertedDigest,
	)
}

func decodeValueFiller(
	raw []byte,
	path string,
) (slotFillerCandidate, error) {
	wire := valueFillerWire{}
	if err := decodeStrict(raw, &wire, path, "by_value filler"); err != nil {
		return nil, err
	}
	if len(wire.Value) == 0 {
		return nil, invalidContract(path+".value", "typed-value candidate is required")
	}
	candidate, err := decodeTypedValueCandidate(wire.Value, path+".value")
	if err != nil {
		return nil, err
	}
	return valueFillerCandidate{value: candidate}, nil
}

func decodeTypedValueCandidate(
	raw []byte,
	path string,
) (typedValueCandidate, error) {
	wire := typedValueCandidateWire{}
	if err := decodeStrict(raw, &wire, path, "typed-value candidate"); err != nil {
		return typedValueCandidate{}, err
	}
	valueKindID, err := parseKindID(wire.ValueKind, path+".value_kind")
	if err != nil {
		return typedValueCandidate{}, err
	}
	valueShape, err := decodeValueShape(wire.ValueShape, path+".value_shape")
	if err != nil {
		return typedValueCandidate{}, err
	}
	codec, err := decodeCodec(wire.Codec, path+".codec")
	if err != nil {
		return typedValueCandidate{}, err
	}
	input, err := decodeInputBytes(wire.InputBase64, path+".input_base64")
	if err != nil {
		return typedValueCandidate{}, err
	}
	asserted, err := decodeAssertedDigest(wire.AssertedDigest, path+".asserted_digest")
	if err != nil {
		return typedValueCandidate{}, err
	}
	candidate := typedValueCandidate{
		valueKind:      valueKindID,
		valueShape:     valueShape,
		codec:          codec,
		inputBytes:     input,
		assertedDigest: asserted,
	}
	return candidate, nil
}

func decodeValueShape(raw []byte, path string) (typedmemory.ValueShapeRef, error) {
	if len(raw) == 0 {
		return typedmemory.ValueShapeRef{}, invalidContract(path, "value_shape is required")
	}
	wire := valueShapeWire{}
	if err := decodeStrict(raw, &wire, path, "value_shape"); err != nil {
		return typedmemory.ValueShapeRef{}, err
	}
	if err := requireIdentifier(wire.ID, path+".id"); err != nil {
		return typedmemory.ValueShapeRef{}, err
	}
	id, err := typedmemory.NewShapeID(wire.ID)
	if err != nil {
		return typedmemory.ValueShapeRef{}, invalidValue(path+".id", err)
	}
	digest, err := typedmemory.NewSHA256Digest(wire.Digest)
	if err != nil {
		return typedmemory.ValueShapeRef{}, invalidValue(path+".digest", err)
	}
	ref, err := typedmemory.NewValueShapeRef(id, digest)
	if err != nil {
		return typedmemory.ValueShapeRef{}, invalidValue(path, err)
	}
	return ref, nil
}

func decodeCodec(raw []byte, path string) (typedmemory.CodecRef, error) {
	if len(raw) == 0 {
		return typedmemory.CodecRef{}, invalidContract(path, "codec is required")
	}
	wire := codecWire{}
	if err := decodeStrict(raw, &wire, path, "codec"); err != nil {
		return typedmemory.CodecRef{}, err
	}
	if err := requireIdentifier(wire.ID, path+".id"); err != nil {
		return typedmemory.CodecRef{}, err
	}
	id, err := typedmemory.NewCodecID(wire.ID)
	if err != nil {
		return typedmemory.CodecRef{}, invalidValue(path+".id", err)
	}
	if err := requireIdentifier(wire.Version, path+".version"); err != nil {
		return typedmemory.CodecRef{}, err
	}
	version, err := typedmemory.NewCanonicalizationVersion(wire.Version)
	if err != nil {
		return typedmemory.CodecRef{}, invalidValue(path+".version", err)
	}
	digest, err := typedmemory.NewSHA256Digest(wire.SpecificationDigest)
	if err != nil {
		return typedmemory.CodecRef{}, invalidValue(path+".specification_digest", err)
	}
	ref, err := typedmemory.NewCodecRef(id, version, digest)
	if err != nil {
		return typedmemory.CodecRef{}, invalidValue(path, err)
	}
	return ref, nil
}

type assertedDigestWire struct {
	Kind   string `json:"kind"`
	Digest string `json:"digest,omitempty"`
}

func decodeAssertedDigest(raw []byte, path string) (typedmemory.AssertedTypedValueDigest, error) {
	if len(raw) == 0 {
		return nil, invalidContract(path, "asserted_digest posture is required")
	}
	kind, err := decodeDiscriminator(raw, path)
	if err != nil {
		return nil, err
	}
	if kind == "none" {
		wire := struct {
			Kind string `json:"kind"`
		}{}
		if err := decodeStrict(raw, &wire, path, "no asserted digest"); err != nil {
			return nil, err
		}
		return typedmemory.NoAssertedDigest{}, nil
	}
	if kind == "exact" {
		wire := assertedDigestWire{}
		if err := decodeStrict(raw, &wire, path, "exact asserted digest"); err != nil {
			return nil, err
		}
		digest, err := typedmemory.NewSHA256Digest(wire.Digest)
		if err != nil {
			return nil, invalidValue(path+".digest", err)
		}
		return typedmemory.NewExactAssertedDigest(digest)
	}
	message := fmt.Sprintf("unknown asserted-digest posture %q", kind)
	return nil, invalidContract(path+".kind", message)
}

func decodeInputBytes(raw, path string) ([]byte, error) {
	if raw == "" {
		return nil, invalidContract(path, "input_base64 is required")
	}
	maximumEncodedLength := base64.StdEncoding.EncodedLen(MaximumTypedValueBytes)
	if len(raw) > maximumEncodedLength {
		message := fmt.Sprintf("decoded value exceeds %d bytes", MaximumTypedValueBytes)
		return nil, resourceLimit(path, message)
	}
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, invalidContract(path, "input_base64 is not canonical base64")
	}
	canonical := base64.StdEncoding.EncodeToString(decoded)
	if canonical != raw {
		return nil, invalidContract(path, "input_base64 is not canonical base64")
	}
	if len(decoded) > MaximumTypedValueBytes {
		message := fmt.Sprintf("decoded value exceeds %d bytes", MaximumTypedValueBytes)
		return nil, resourceLimit(path, message)
	}
	if len(decoded) == 0 {
		return nil, invalidContract(path, "decoded value must be non-empty")
	}
	return decoded, nil
}

type retractAssertionWire struct {
	Kind        string `json:"kind"`
	AssertionID string `json:"assertion_id"`
	Reason      string `json:"reason"`
	Provenance  string `json:"provenance"`
}

func decodeRetractAssertion(raw []byte, path string) (memoryChangeCandidate, error) {
	wire := retractAssertionWire{}
	if err := decodeStrict(raw, &wire, path, "retract_assertion"); err != nil {
		return nil, err
	}
	assertion, err := parseAssertionID(wire.AssertionID, path+".assertion_id")
	if err != nil {
		return nil, err
	}
	if err := requireText(wire.Reason, path+".reason"); err != nil {
		return nil, err
	}
	reason, err := typedmemory.NewRetractionReason(wire.Reason)
	if err != nil {
		return nil, invalidValue(path+".reason", err)
	}
	provenance, err := parseProvenance(wire.Provenance, path+".provenance")
	if err != nil {
		return nil, err
	}
	change, err := typedmemory.NewRetractAssertion(assertion, reason, provenance)
	if err != nil {
		return nil, invalidValue(path, err)
	}
	return exactMemoryChangeCandidate{change: change}, nil
}

func decodeDiscriminator(raw []byte, path string) (string, error) {
	wire := discriminatorWire{}
	if err := json.Unmarshal(raw, &wire); err != nil {
		message := fmt.Sprintf("invalid discriminator: %s", err)
		return "", invalidContract(path, message)
	}
	if err := requireIdentifier(wire.Kind, path+".kind"); err != nil {
		return "", err
	}
	return wire.Kind, nil
}

func decodeStrict[T any](raw []byte, target *T, path, label string) error {
	reader := bytes.NewReader(raw)
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		message := fmt.Sprintf("invalid %s: %s", label, err)
		return invalidContract(path, message)
	}
	var trailing json.RawMessage
	err := decoder.Decode(&trailing)
	if err != io.EOF {
		message := fmt.Sprintf("invalid %s: trailing JSON value", label)
		return invalidContract(path, message)
	}
	return nil
}

func invalidValue(path string, err error) error {
	return invalidContract(path, err.Error())
}

func requireIdentifier(value, path string) error {
	if strings.TrimSpace(value) == "" {
		return invalidContract(path, "value is required")
	}
	if len(value) > MaximumIdentifierBytes {
		message := fmt.Sprintf("identifier exceeds %d bytes", MaximumIdentifierBytes)
		return resourceLimit(path, message)
	}
	return nil
}

func requireText(value, path string) error {
	if strings.TrimSpace(value) == "" {
		return invalidContract(path, "text is required")
	}
	if len(value) > MaximumTextBytes {
		message := fmt.Sprintf("text exceeds %d bytes", MaximumTextBytes)
		return resourceLimit(path, message)
	}
	return nil
}

func parseEntityID(raw, path string) (typedmemory.EntityID, error) {
	if err := requireIdentifier(raw, path); err != nil {
		return typedmemory.EntityID{}, err
	}
	value, err := typedmemory.NewEntityID(raw)
	if err != nil {
		return typedmemory.EntityID{}, invalidValue(path, err)
	}
	return value, nil
}

func parseEntityIDs(values []string, path string) ([]typedmemory.EntityID, error) {
	if len(values) == 0 {
		return nil, invalidContract(path, "entity list must be non-empty")
	}
	result := make([]typedmemory.EntityID, 0, len(values))
	for index, raw := range values {
		itemPath := fmt.Sprintf("%s[%d]", path, index)
		value, err := parseEntityID(raw, itemPath)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, nil
}

func parseAssertionID(raw, path string) (typedmemory.AssertionID, error) {
	if err := requireIdentifier(raw, path); err != nil {
		return typedmemory.AssertionID{}, err
	}
	value, err := typedmemory.NewAssertionID(raw)
	if err != nil {
		return typedmemory.AssertionID{}, invalidValue(path, err)
	}
	return value, nil
}

func parseBatchLocalRef(raw, path string) (typedmemory.BatchLocalRef, error) {
	if err := requireIdentifier(raw, path); err != nil {
		return typedmemory.BatchLocalRef{}, err
	}
	value, err := typedmemory.NewBatchLocalRef(raw)
	if err != nil {
		return typedmemory.BatchLocalRef{}, invalidValue(path, err)
	}
	return value, nil
}

func parseContext(raw, path string) (typedmemory.BoundedContextRef, error) {
	if err := requireIdentifier(raw, path); err != nil {
		return typedmemory.BoundedContextRef{}, err
	}
	value, err := typedmemory.NewBoundedContextRef(raw)
	if err != nil {
		return typedmemory.BoundedContextRef{}, invalidValue(path, err)
	}
	return value, nil
}

func parseCarrierRef(raw, path string) (typedmemory.CarrierRef, error) {
	if err := requireIdentifier(raw, path); err != nil {
		return typedmemory.CarrierRef{}, err
	}
	value, err := typedmemory.NewCarrierRef(raw)
	if err != nil {
		return typedmemory.CarrierRef{}, invalidValue(path, err)
	}
	return value, nil
}

func parseCarrierEdition(raw, path string) (typedmemory.CarrierEdition, error) {
	if err := requireIdentifier(raw, path); err != nil {
		return typedmemory.CarrierEdition{}, err
	}
	value, err := typedmemory.NewCarrierEdition(raw)
	if err != nil {
		return typedmemory.CarrierEdition{}, invalidValue(path, err)
	}
	return value, nil
}

func parseSHA256Digest(raw, path string) (typedmemory.SHA256Digest, error) {
	if err := requireIdentifier(raw, path); err != nil {
		return typedmemory.SHA256Digest{}, err
	}
	value, err := typedmemory.NewSHA256Digest(raw)
	if err != nil {
		return typedmemory.SHA256Digest{}, invalidValue(path, err)
	}
	return value, nil
}

func parseProvenance(raw, path string) (typedmemory.ProvenanceRef, error) {
	if err := requireIdentifier(raw, path); err != nil {
		return typedmemory.ProvenanceRef{}, err
	}
	value, err := typedmemory.NewProvenanceRef(raw)
	if err != nil {
		return typedmemory.ProvenanceRef{}, invalidValue(path, err)
	}
	return value, nil
}

func parseAlias(raw, path string) (typedmemory.EntityAlias, error) {
	if err := requireIdentifier(raw, path); err != nil {
		return typedmemory.EntityAlias{}, err
	}
	value, err := typedmemory.NewEntityAlias(raw)
	if err != nil {
		return typedmemory.EntityAlias{}, invalidValue(path, err)
	}
	return value, nil
}

func parseReconciliationBasis(raw, path string) (typedmemory.ReconciliationBasisRef, error) {
	if err := requireIdentifier(raw, path); err != nil {
		return typedmemory.ReconciliationBasisRef{}, err
	}
	value, err := typedmemory.NewReconciliationBasisRef(raw)
	if err != nil {
		return typedmemory.ReconciliationBasisRef{}, invalidValue(path, err)
	}
	return value, nil
}

func parseSignatureID(raw, path string) (typedmemory.SignatureID, error) {
	if err := requireIdentifier(raw, path); err != nil {
		return typedmemory.SignatureID{}, err
	}
	value, err := typedmemory.NewSignatureID(raw)
	if err != nil {
		return typedmemory.SignatureID{}, invalidValue(path, err)
	}
	return value, nil
}

func parseSlotKindID(raw, path string) (typedmemory.SlotKindID, error) {
	if err := requireIdentifier(raw, path); err != nil {
		return typedmemory.SlotKindID{}, err
	}
	value, err := typedmemory.NewSlotKindID(raw)
	if err != nil {
		return typedmemory.SlotKindID{}, invalidValue(path, err)
	}
	return value, nil
}

func parseKindID(raw, path string) (typedmemory.KindID, error) {
	if err := requireIdentifier(raw, path); err != nil {
		return typedmemory.KindID{}, err
	}
	value, err := typedmemory.NewKindID(raw)
	if err != nil {
		return typedmemory.KindID{}, invalidValue(path, err)
	}
	return value, nil
}

func parseRefKindID(raw string, path string) (typedmemory.RefKindID, error) {
	if err := requireIdentifier(raw, path); err != nil {
		return typedmemory.RefKindID{}, err
	}
	id, err := typedmemory.NewRefKindID(raw)
	if err != nil {
		return typedmemory.RefKindID{}, invalidValue(path, err)
	}
	return id, nil
}

func parseReferenceID(raw, path string) (typedmemory.ReferenceID, error) {
	if err := requireIdentifier(raw, path); err != nil {
		return typedmemory.ReferenceID{}, err
	}
	value, err := typedmemory.NewReferenceID(raw)
	if err != nil {
		return typedmemory.ReferenceID{}, invalidValue(path, err)
	}
	return value, nil
}
