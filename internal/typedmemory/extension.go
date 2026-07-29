package typedmemory

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
)

type ExtensionID struct {
	value string
}

func NewExtensionID(raw string) (ExtensionID, error) {
	value, err := parseQualifiedIdentifier("TypeEnv extension ID", raw)
	if err != nil {
		return ExtensionID{}, err
	}
	return ExtensionID{value: value}, nil
}

func (id ExtensionID) String() string { return id.value }

func (id ExtensionID) valid() bool { return id.value != "" }

type TypeEnvExtensionRef struct {
	id     ExtensionID
	digest SHA256Digest
}

func newTypeEnvExtensionRef(
	id ExtensionID,
	digest SHA256Digest,
) (TypeEnvExtensionRef, error) {
	if !id.valid() {
		return TypeEnvExtensionRef{}, fmt.Errorf("TypeEnv extension ID is required")
	}
	if !digest.valid() {
		return TypeEnvExtensionRef{}, fmt.Errorf("TypeEnv extension digest is required")
	}
	return TypeEnvExtensionRef{id: id, digest: digest}, nil
}

// ParseTypeEnvExtensionRef parses an external coordinate. Parsing proves only
// that the coordinate is well formed. VerifyLoweredTypeEnvExtensionProposal
// must be used before it is trusted as the identity of proposal bytes.
func ParseTypeEnvExtensionRef(raw string) (TypeEnvExtensionRef, error) {
	value, found := strings.CutPrefix(raw, "typeenv-extension:")
	if !found {
		return TypeEnvExtensionRef{}, fmt.Errorf("TypeEnv extension reference is malformed")
	}
	separator := strings.LastIndex(value, "@"+sha256Prefix)
	if separator < 1 {
		return TypeEnvExtensionRef{}, fmt.Errorf("TypeEnv extension reference is malformed")
	}
	id, err := NewExtensionID(value[:separator])
	if err != nil {
		return TypeEnvExtensionRef{}, fmt.Errorf("TypeEnv extension reference ID: %w", err)
	}
	digest, err := NewSHA256Digest(value[separator+1:])
	if err != nil {
		return TypeEnvExtensionRef{}, fmt.Errorf("TypeEnv extension reference digest: %w", err)
	}
	ref, err := newTypeEnvExtensionRef(id, digest)
	if err != nil {
		return TypeEnvExtensionRef{}, err
	}
	if ref.String() != raw {
		return TypeEnvExtensionRef{}, fmt.Errorf("TypeEnv extension reference is not canonical")
	}
	return ref, nil
}

func (ref TypeEnvExtensionRef) ID() ExtensionID { return ref.id }

func (ref TypeEnvExtensionRef) Digest() SHA256Digest { return ref.digest }

func (ref TypeEnvExtensionRef) String() string {
	return "typeenv-extension:" + ref.id.String() + "@" + ref.digest.String()
}

func (ref TypeEnvExtensionRef) valid() bool { return ref.id.valid() && ref.digest.valid() }

type SignatureManifest struct {
	ref      SignatureManifestRef
	imports  []SchemaSymbolRef
	provides []SchemaSymbolRef
}

func NewSignatureManifest(
	ref SignatureManifestRef,
	imports []SchemaSymbolRef,
	provides []SchemaSymbolRef,
) (SignatureManifest, error) {
	if !ref.valid() {
		return SignatureManifest{}, fmt.Errorf("signature manifest reference is required")
	}
	if len(provides) == 0 {
		return SignatureManifest{}, fmt.Errorf("signature manifest requires at least one provided symbol")
	}
	ownedImports, err := canonicalSchemaSymbols("manifest import", imports)
	if err != nil {
		return SignatureManifest{}, err
	}
	ownedProvides, err := canonicalSchemaSymbols("manifest provide", provides)
	if err != nil {
		return SignatureManifest{}, err
	}
	provided := make(map[string]struct{}, len(ownedProvides))
	for _, symbol := range ownedProvides {
		provided[symbol.String()] = struct{}{}
	}
	for _, symbol := range ownedImports {
		if _, overlaps := provided[symbol.String()]; overlaps {
			return SignatureManifest{}, fmt.Errorf("manifest symbol %q cannot be both imported and provided", symbol.String())
		}
	}
	return SignatureManifest{
		ref:      ref,
		imports:  ownedImports,
		provides: ownedProvides,
	}, nil
}

func (manifest SignatureManifest) Ref() SignatureManifestRef { return manifest.ref }

func (manifest SignatureManifest) Imports() []SchemaSymbolRef {
	return append([]SchemaSymbolRef(nil), manifest.imports...)
}

func (manifest SignatureManifest) Provides() []SchemaSymbolRef {
	return append([]SchemaSymbolRef(nil), manifest.provides...)
}

func (manifest SignatureManifest) valid() bool {
	return manifest.ref.valid() && len(manifest.provides) > 0
}

func (manifest SignatureManifest) providesSymbol(symbol SchemaSymbolRef) bool {
	index := sort.Search(len(manifest.provides), func(index int) bool {
		return manifest.provides[index].String() >= symbol.String()
	})
	return index < len(manifest.provides) && manifest.provides[index] == symbol
}

func canonicalSchemaSymbols(
	label string,
	symbols []SchemaSymbolRef,
) ([]SchemaSymbolRef, error) {
	owned := append([]SchemaSymbolRef(nil), symbols...)
	sort.Slice(owned, func(left, right int) bool {
		return owned[left].String() < owned[right].String()
	})
	for index, symbol := range owned {
		if !symbol.valid() {
			return nil, fmt.Errorf("%s at index %d is invalid", label, index)
		}
		if index > 0 && symbol == owned[index-1] {
			return nil, fmt.Errorf("duplicate %s %q", label, symbol.String())
		}
	}
	return owned, nil
}

// SchemaChange is a sealed union disjoint from MemoryChange. None of these
// values implements memoryChangeVariant, so a generic MemoryChangeSet cannot
// carry a TypeEnv mutation.
type SchemaChange interface {
	ChangeKey() string
	ProvidedSymbols() []SchemaSymbolRef
	Provenance() DeclarationProvenance
	schemaChangeVariant()
}

type AddBoundedContextSchemaChange struct {
	context BoundedContext
}

func NewAddBoundedContextSchemaChange(
	context BoundedContext,
) (AddBoundedContextSchemaChange, error) {
	if !context.valid() {
		return AddBoundedContextSchemaChange{}, fmt.Errorf("bounded-context schema change is invalid")
	}
	return AddBoundedContextSchemaChange{context: context}, nil
}

func (change AddBoundedContextSchemaChange) Context() BoundedContext { return change.context }

func (change AddBoundedContextSchemaChange) ChangeKey() string {
	return "add-context:" + change.context.ref.String()
}

func (change AddBoundedContextSchemaChange) ProvidedSymbols() []SchemaSymbolRef {
	symbol, _ := BoundedContextSymbolRef(change.context.ref)
	return []SchemaSymbolRef{symbol}
}

func (change AddBoundedContextSchemaChange) Provenance() DeclarationProvenance {
	return change.context.provenance
}

func (AddBoundedContextSchemaChange) schemaChangeVariant() {}

type DefineKindSchemaChange struct {
	definition KindDefinition
}

func NewDefineKindSchemaChange(
	definition KindDefinition,
) (DefineKindSchemaChange, error) {
	if !definition.valid() {
		return DefineKindSchemaChange{}, fmt.Errorf("kind-definition schema change is invalid")
	}
	return DefineKindSchemaChange{definition: definition}, nil
}

func (change DefineKindSchemaChange) Definition() KindDefinition { return change.definition }

func (change DefineKindSchemaChange) ChangeKey() string {
	return "define-kind:" + change.definition.id.String()
}

func (change DefineKindSchemaChange) ProvidedSymbols() []SchemaSymbolRef {
	symbol, _ := KindSymbolRef(change.definition.id)
	return []SchemaSymbolRef{symbol}
}

func (change DefineKindSchemaChange) Provenance() DeclarationProvenance {
	return change.definition.provenance
}

func (DefineKindSchemaChange) schemaChangeVariant() {}

type DefineRefKindSchemaChange struct {
	definition RefKindDefinition
}

func NewDefineRefKindSchemaChange(
	definition RefKindDefinition,
) (DefineRefKindSchemaChange, error) {
	if !definition.valid() {
		return DefineRefKindSchemaChange{}, fmt.Errorf("RefKind-definition schema change is invalid")
	}
	return DefineRefKindSchemaChange{definition: definition}, nil
}

func (change DefineRefKindSchemaChange) Definition() RefKindDefinition {
	return change.definition
}

func (change DefineRefKindSchemaChange) ChangeKey() string {
	return "define-ref-kind:" + change.definition.ref.String()
}

func (change DefineRefKindSchemaChange) ProvidedSymbols() []SchemaSymbolRef {
	symbol, _ := RefKindSymbolRef(change.definition.ref.ID())
	return []SchemaSymbolRef{symbol}
}

func (change DefineRefKindSchemaChange) Provenance() DeclarationProvenance {
	return change.definition.provenance
}

func (DefineRefKindSchemaChange) schemaChangeVariant() {}

type DefineSubkindSchemaChange struct {
	relation SubkindRelation
}

func NewDefineSubkindSchemaChange(
	relation SubkindRelation,
) (DefineSubkindSchemaChange, error) {
	if !relation.valid() {
		return DefineSubkindSchemaChange{}, fmt.Errorf("subkind schema change is invalid")
	}
	return DefineSubkindSchemaChange{relation: relation}, nil
}

func (change DefineSubkindSchemaChange) Relation() SubkindRelation { return change.relation }

func (change DefineSubkindSchemaChange) ChangeKey() string {
	return "define-subkind:" + change.relation.key()
}

func (DefineSubkindSchemaChange) ProvidedSymbols() []SchemaSymbolRef { return nil }

func (change DefineSubkindSchemaChange) Provenance() DeclarationProvenance {
	return change.relation.provenance
}

func (DefineSubkindSchemaChange) schemaChangeVariant() {}

type AddContextBridgeSchemaChange struct {
	bridge ContextBridge
}

func NewAddContextBridgeSchemaChange(
	bridge ContextBridge,
) (AddContextBridgeSchemaChange, error) {
	if !bridge.valid() {
		return AddContextBridgeSchemaChange{}, fmt.Errorf("context-bridge schema change is invalid")
	}
	return AddContextBridgeSchemaChange{bridge: cloneContextBridge(bridge)}, nil
}

func (change AddContextBridgeSchemaChange) Bridge() ContextBridge {
	return cloneContextBridge(change.bridge)
}

func (change AddContextBridgeSchemaChange) ChangeKey() string {
	return "add-bridge:" + change.bridge.id.String()
}

func (change AddContextBridgeSchemaChange) ProvidedSymbols() []SchemaSymbolRef {
	symbol, _ := ContextBridgeSymbolRef(change.bridge.id)
	return []SchemaSymbolRef{symbol}
}

func (change AddContextBridgeSchemaChange) Provenance() DeclarationProvenance {
	return cloneDeclarationProvenance(change.bridge.provenance)
}

func (AddContextBridgeSchemaChange) schemaChangeVariant() {}

type DefineTypedRelationDeclarationFragmentSchemaChange struct {
	fragment TypedRelationDeclarationFragment
}

func NewDefineTypedRelationDeclarationFragmentSchemaChange(
	fragment TypedRelationDeclarationFragment,
) (DefineTypedRelationDeclarationFragmentSchemaChange, error) {
	if !fragment.valid() {
		return DefineTypedRelationDeclarationFragmentSchemaChange{}, fmt.Errorf(
			"typed relation declaration fragment schema change is invalid",
		)
	}
	return DefineTypedRelationDeclarationFragmentSchemaChange{fragment: fragment}, nil
}

func (change DefineTypedRelationDeclarationFragmentSchemaChange) Fragment() TypedRelationDeclarationFragment {
	return change.fragment
}

// Signature preserves the historical accessor spelling used by sealed
// extension editions. It returns the same structurally limited fragment.
func (change DefineTypedRelationDeclarationFragmentSchemaChange) Signature() RelationSignature {
	return change.fragment
}

func (change DefineTypedRelationDeclarationFragmentSchemaChange) Posture() RelationDeclarationPosture {
	return change.fragment.Posture()
}

func (change DefineTypedRelationDeclarationFragmentSchemaChange) ChangeKey() string {
	// Preserve the edition-bound canonical key.
	return "define-signature:" + change.fragment.ref.ID().String()
}

func (change DefineTypedRelationDeclarationFragmentSchemaChange) ProvidedSymbols() []SchemaSymbolRef {
	relationSymbol, _ := RelationSymbolRef(change.fragment.ref.ID())
	symbols := []SchemaSymbolRef{relationSymbol}
	for _, slot := range change.fragment.slots {
		slotSymbol, _ := SlotKindSymbolRef(change.fragment.ref.ID(), slot.slotKind)
		symbols = append(symbols, slotSymbol)
	}
	result, _ := canonicalSchemaSymbols("relation-provided symbol", symbols)
	return result
}

func (change DefineTypedRelationDeclarationFragmentSchemaChange) Provenance() DeclarationProvenance {
	return change.fragment.provenance
}

func (DefineTypedRelationDeclarationFragmentSchemaChange) schemaChangeVariant() {}

// DefineRelationSignatureSchemaChange is the historical Go spelling retained
// for sealed edition replay. It aliases the fragment-only current type.
type DefineRelationSignatureSchemaChange = DefineTypedRelationDeclarationFragmentSchemaChange

// NewDefineRelationSignatureSchemaChange preserves the historical constructor
// spelling for exact extension decode/replay.
func NewDefineRelationSignatureSchemaChange(
	signature RelationSignature,
) (DefineRelationSignatureSchemaChange, error) {
	return NewDefineTypedRelationDeclarationFragmentSchemaChange(signature)
}

type DeclareValueShapeSchemaChange struct {
	declaration ValueShapeDeclaration
}

func NewDeclareValueShapeSchemaChange(
	declaration ValueShapeDeclaration,
) (DeclareValueShapeSchemaChange, error) {
	if !declaration.valid() {
		return DeclareValueShapeSchemaChange{}, fmt.Errorf("value-shape schema change is invalid")
	}
	return DeclareValueShapeSchemaChange{declaration: declaration}, nil
}

func (change DeclareValueShapeSchemaChange) Declaration() ValueShapeDeclaration {
	return change.declaration
}

func (change DeclareValueShapeSchemaChange) ChangeKey() string {
	return "declare-shape:" + change.declaration.ref.String()
}

func (change DeclareValueShapeSchemaChange) ProvidedSymbols() []SchemaSymbolRef {
	symbol, _ := ValueShapeSymbolRef(change.declaration.ref.ID())
	return []SchemaSymbolRef{symbol}
}

func (change DeclareValueShapeSchemaChange) Provenance() DeclarationProvenance {
	return change.declaration.provenance
}

func (DeclareValueShapeSchemaChange) schemaChangeVariant() {}

type BindValueKindSchemaChange struct {
	binding ValueBinding
}

func NewBindValueKindSchemaChange(
	binding ValueBinding,
) (BindValueKindSchemaChange, error) {
	if !binding.valid() {
		return BindValueKindSchemaChange{}, fmt.Errorf("value-binding schema change is invalid")
	}
	return BindValueKindSchemaChange{binding: binding}, nil
}

func (change BindValueKindSchemaChange) Binding() ValueBinding { return change.binding }

func (change BindValueKindSchemaChange) ChangeKey() string {
	return "bind-value-kind:" + change.binding.valueKind.String()
}

func (change BindValueKindSchemaChange) ProvidedSymbols() []SchemaSymbolRef {
	codecSymbol, _ := CodecSymbolRef(change.binding.codec.ID())
	return []SchemaSymbolRef{codecSymbol}
}

func (change BindValueKindSchemaChange) Provenance() DeclarationProvenance {
	return change.binding.provenance
}

func (BindValueKindSchemaChange) schemaChangeVariant() {}

type AddConstraintSchemaChange struct {
	rule ConstraintRule
}

func NewAddConstraintSchemaChange(
	rule ConstraintRule,
) (AddConstraintSchemaChange, error) {
	if !validConstraintRule(rule) {
		return AddConstraintSchemaChange{}, fmt.Errorf("constraint schema change is invalid")
	}
	return AddConstraintSchemaChange{rule: rule}, nil
}

func (change AddConstraintSchemaChange) Rule() ConstraintRule { return change.rule }

func (change AddConstraintSchemaChange) ChangeKey() string {
	return "add-constraint:" + change.rule.ID().String()
}

func (change AddConstraintSchemaChange) ProvidedSymbols() []SchemaSymbolRef {
	symbol, _ := ConstraintSymbolRef(change.rule.ID())
	return []SchemaSymbolRef{symbol}
}

func (change AddConstraintSchemaChange) Provenance() DeclarationProvenance {
	return change.rule.Provenance()
}

func (AddConstraintSchemaChange) schemaChangeVariant() {}

func validSchemaChange(change SchemaChange) bool {
	switch value := change.(type) {
	case AddBoundedContextSchemaChange:
		return value.context.valid()
	case DefineKindSchemaChange:
		return value.definition.valid()
	case DefineRefKindSchemaChange:
		return value.definition.valid()
	case DefineSubkindSchemaChange:
		return value.relation.valid()
	case AddContextBridgeSchemaChange:
		return value.bridge.valid()
	case DefineTypedRelationDeclarationFragmentSchemaChange:
		return value.fragment.valid()
	case DeclareValueShapeSchemaChange:
		return value.declaration.valid()
	case BindValueKindSchemaChange:
		return value.binding.valid()
	case AddConstraintSchemaChange:
		return validConstraintRule(value.rule)
	case DefineEntitySetSchemaChange:
		return value.exportedSymbol.valid() &&
			value.exportedSymbol.Kind() == EntitySetSymbol &&
			value.definition.valid()
	case DefineKindSignatureSchemaChange:
		return value.exportedSymbol.valid() &&
			value.exportedSymbol.Kind() == KindSignatureSymbol &&
			value.definition.valid()
	default:
		return false
	}
}

type SchemaChangeSet struct {
	changes []SchemaChange
}

func NewSchemaChangeSet(changes []SchemaChange) (SchemaChangeSet, error) {
	if len(changes) == 0 {
		return SchemaChangeSet{}, fmt.Errorf("SchemaChangeSet must be non-empty")
	}
	owned := cloneSchemaChanges(changes)
	for index, change := range owned {
		if !validSchemaChange(change) {
			return SchemaChangeSet{}, fmt.Errorf("schema change %d is invalid", index)
		}
	}
	sort.Slice(owned, func(left, right int) bool {
		return owned[left].ChangeKey() < owned[right].ChangeKey()
	})
	for index, change := range owned {
		if index > 0 && change.ChangeKey() == owned[index-1].ChangeKey() {
			return SchemaChangeSet{}, fmt.Errorf("duplicate schema change %q", change.ChangeKey())
		}
	}
	exportedCoordinates := make(map[string]string)
	for _, change := range owned {
		for _, symbol := range change.ProvidedSymbols() {
			coordinate, exists := exportedCoordinates[symbol.String()]
			if exists && coordinate != change.ChangeKey() {
				return SchemaChangeSet{}, fmt.Errorf(
					"schema symbol %q cannot bind both %q and %q",
					symbol.String(),
					coordinate,
					change.ChangeKey(),
				)
			}
			exportedCoordinates[symbol.String()] = change.ChangeKey()
		}
	}
	return SchemaChangeSet{changes: owned}, nil
}

func (set SchemaChangeSet) Changes() []SchemaChange {
	return cloneSchemaChanges(set.changes)
}

func (set SchemaChangeSet) valid() bool { return len(set.changes) > 0 }

func cloneSchemaChange(change SchemaChange) SchemaChange {
	switch value := change.(type) {
	case AddContextBridgeSchemaChange:
		value.bridge = cloneContextBridge(value.bridge)
		return value
	default:
		return change
	}
}

func cloneSchemaChanges(changes []SchemaChange) []SchemaChange {
	result := make([]SchemaChange, 0, len(changes))
	for _, change := range changes {
		result = append(result, cloneSchemaChange(change))
	}
	return result
}

type CompatibilityChangeKind uint8

const (
	CompatibilityAdded CompatibilityChangeKind = iota + 1
	CompatibilityChanged
	CompatibilityRemoved
)

func (kind CompatibilityChangeKind) String() string {
	switch kind {
	case CompatibilityAdded:
		return "added"
	case CompatibilityChanged:
		return "changed"
	case CompatibilityRemoved:
		return "removed"
	default:
		return ""
	}
}

func (kind CompatibilityChangeKind) valid() bool { return kind.String() != "" }

type CompatibilityChange struct {
	symbol    SchemaSymbolRef
	kind      CompatibilityChangeKind
	rationale string
}

func NewCompatibilityChange(
	symbol SchemaSymbolRef,
	kind CompatibilityChangeKind,
	rationale string,
) (CompatibilityChange, error) {
	if !symbol.valid() {
		return CompatibilityChange{}, fmt.Errorf("compatibility symbol is required")
	}
	if !kind.valid() {
		return CompatibilityChange{}, fmt.Errorf("compatibility change kind is required")
	}
	value, err := parseOpaqueIdentifier("compatibility rationale", rationale)
	if err != nil {
		return CompatibilityChange{}, err
	}
	return CompatibilityChange{symbol: symbol, kind: kind, rationale: value}, nil
}

func (change CompatibilityChange) Symbol() SchemaSymbolRef { return change.symbol }

func (change CompatibilityChange) Kind() CompatibilityChangeKind { return change.kind }

func (change CompatibilityChange) Rationale() string { return change.rationale }

func (change CompatibilityChange) valid() bool {
	return change.symbol.valid() && change.kind.valid() && change.rationale != ""
}

type TypeEnvCompatibilityDiff struct {
	base    TypeEnvRef
	changes []CompatibilityChange
}

func NewTypeEnvCompatibilityDiff(
	base TypeEnvRef,
	changes []CompatibilityChange,
) (TypeEnvCompatibilityDiff, error) {
	if !base.valid() {
		return TypeEnvCompatibilityDiff{}, fmt.Errorf("compatibility base TypeEnv is required")
	}
	owned := append([]CompatibilityChange(nil), changes...)
	sort.Slice(owned, func(left, right int) bool {
		return owned[left].symbol.String() < owned[right].symbol.String()
	})
	for index, change := range owned {
		if !change.valid() {
			return TypeEnvCompatibilityDiff{}, fmt.Errorf("compatibility change %d is invalid", index)
		}
		if index > 0 && change.symbol == owned[index-1].symbol {
			return TypeEnvCompatibilityDiff{}, fmt.Errorf("duplicate compatibility symbol %q", change.symbol.String())
		}
	}
	return TypeEnvCompatibilityDiff{base: base, changes: owned}, nil
}

func (diff TypeEnvCompatibilityDiff) Base() TypeEnvRef { return diff.base }

func (diff TypeEnvCompatibilityDiff) Changes() []CompatibilityChange {
	return append([]CompatibilityChange(nil), diff.changes...)
}

func (diff TypeEnvCompatibilityDiff) valid() bool { return diff.base.valid() }

type RevalidationPosture uint8

const (
	RevalidationClean RevalidationPosture = iota + 1
	RevalidationConflict
	RevalidationUnderdetermined
)

func (posture RevalidationPosture) String() string {
	switch posture {
	case RevalidationClean:
		return "clean"
	case RevalidationConflict:
		return "conflict"
	case RevalidationUnderdetermined:
		return "underdetermined"
	default:
		return ""
	}
}

func (posture RevalidationPosture) valid() bool { return posture.String() != "" }

type ExistingAssertionRevalidationReport struct {
	posture            RevalidationPosture
	graphRevision      GraphRevision
	affectedAssertions []AssertionID
	reportDigest       SHA256Digest
}

func NewExistingAssertionRevalidationReport(
	posture RevalidationPosture,
	graphRevision GraphRevision,
	affectedAssertions []AssertionID,
	reportDigest SHA256Digest,
) (ExistingAssertionRevalidationReport, error) {
	if !posture.valid() {
		return ExistingAssertionRevalidationReport{}, fmt.Errorf("revalidation posture is required")
	}
	if !reportDigest.valid() {
		return ExistingAssertionRevalidationReport{}, fmt.Errorf("revalidation report digest is required")
	}
	owned := append([]AssertionID(nil), affectedAssertions...)
	sort.Slice(owned, func(left, right int) bool {
		return owned[left].String() < owned[right].String()
	})
	for index, assertionID := range owned {
		if !assertionID.valid() {
			return ExistingAssertionRevalidationReport{}, fmt.Errorf("revalidation assertion %d is invalid", index)
		}
		if index > 0 && assertionID == owned[index-1] {
			return ExistingAssertionRevalidationReport{}, fmt.Errorf("duplicate revalidation assertion %q", assertionID.String())
		}
	}
	return ExistingAssertionRevalidationReport{
		posture:            posture,
		graphRevision:      graphRevision,
		affectedAssertions: owned,
		reportDigest:       reportDigest,
	}, nil
}

func (report ExistingAssertionRevalidationReport) Posture() RevalidationPosture {
	return report.posture
}

func (report ExistingAssertionRevalidationReport) GraphRevision() GraphRevision {
	return report.graphRevision
}

func (report ExistingAssertionRevalidationReport) AffectedAssertions() []AssertionID {
	return append([]AssertionID(nil), report.affectedAssertions...)
}

func (report ExistingAssertionRevalidationReport) Digest() SHA256Digest {
	return report.reportDigest
}

func (report ExistingAssertionRevalidationReport) valid() bool {
	return report.posture.valid() && report.reportDigest.valid()
}

// TypeEnvExtensionProposal is an immutable, non-binding lowered artifact.
// Its SchemaChange values already contain concrete TypeEnvRefs. It is not the
// self-reference-free LocalPractice source IR and cannot derive or activate a
// composite TypeEnv. It intentionally exposes no Activate method; activation
// belongs to a later manually authorized institutional-effect service. Its
// outer ref authenticates the complete lowered artifact; it does not assert
// that nested ValueShapeRef or revalidation-report digests were independently
// derived from their adjacent descriptive fields.
type TypeEnvExtensionProposal struct {
	ref           TypeEnvExtensionRef
	canonical     []byte
	edition       CarrierEdition
	baseTypeEnv   TypeEnvRef
	context       BoundedContextRef
	carrier       CarrierRef
	carrierHash   SHA256Digest
	manifest      SignatureManifest
	changes       SchemaChangeSet
	compiler      CompilerSchemaVersion
	compatibility TypeEnvCompatibilityDiff
	revalidation  ExistingAssertionRevalidationReport
}

// loweredTypeEnvExtensionCandidate deliberately has no proposal digest. It is
// the lowered semantic input from which the canonical artifact and its
// TypeEnvExtensionRef are derived together. Concrete TypeEnvRefs inside its
// SchemaChanges are preserved exactly and are never rewritten to the base.
type loweredTypeEnvExtensionCandidate struct {
	id            ExtensionID
	edition       CarrierEdition
	baseTypeEnv   TypeEnvRef
	context       BoundedContextRef
	carrier       CarrierRef
	carrierHash   SHA256Digest
	manifest      SignatureManifest
	changes       SchemaChangeSet
	compiler      CompilerSchemaVersion
	compatibility TypeEnvCompatibilityDiff
	revalidation  ExistingAssertionRevalidationReport
}

type LoweredTypeEnvExtensionProposalBuilder struct {
	value loweredTypeEnvExtensionCandidate
}

func NewLoweredTypeEnvExtensionProposalBuilder(
	id ExtensionID,
) *LoweredTypeEnvExtensionProposalBuilder {
	return &LoweredTypeEnvExtensionProposalBuilder{value: loweredTypeEnvExtensionCandidate{id: id}}
}

func (builder *LoweredTypeEnvExtensionProposalBuilder) SetSourceCarrier(
	carrier CarrierRef,
	edition CarrierEdition,
	contentHash SHA256Digest,
) *LoweredTypeEnvExtensionProposalBuilder {
	builder.value.carrier = carrier
	builder.value.edition = edition
	builder.value.carrierHash = contentHash
	return builder
}

func (builder *LoweredTypeEnvExtensionProposalBuilder) SetBaseTypeEnv(
	base TypeEnvRef,
) *LoweredTypeEnvExtensionProposalBuilder {
	builder.value.baseTypeEnv = base
	return builder
}

func (builder *LoweredTypeEnvExtensionProposalBuilder) SetBoundedContext(
	context BoundedContextRef,
) *LoweredTypeEnvExtensionProposalBuilder {
	builder.value.context = context
	return builder
}

func (builder *LoweredTypeEnvExtensionProposalBuilder) SetSignatureManifest(
	manifest SignatureManifest,
) *LoweredTypeEnvExtensionProposalBuilder {
	builder.value.manifest = manifest
	return builder
}

func (builder *LoweredTypeEnvExtensionProposalBuilder) SetSchemaChanges(
	changes SchemaChangeSet,
) *LoweredTypeEnvExtensionProposalBuilder {
	builder.value.changes = changes
	return builder
}

func (builder *LoweredTypeEnvExtensionProposalBuilder) SetCompilerSchemaVersion(
	version CompilerSchemaVersion,
) *LoweredTypeEnvExtensionProposalBuilder {
	builder.value.compiler = version
	return builder
}

func (builder *LoweredTypeEnvExtensionProposalBuilder) SetCompatibilityDiff(
	diff TypeEnvCompatibilityDiff,
) *LoweredTypeEnvExtensionProposalBuilder {
	builder.value.compatibility = diff
	return builder
}

func (builder *LoweredTypeEnvExtensionProposalBuilder) SetRevalidationReport(
	report ExistingAssertionRevalidationReport,
) *LoweredTypeEnvExtensionProposalBuilder {
	builder.value.revalidation = report
	return builder
}

func (builder *LoweredTypeEnvExtensionProposalBuilder) Build() (TypeEnvExtensionProposal, error) {
	if builder == nil {
		return TypeEnvExtensionProposal{}, fmt.Errorf("TypeEnv extension proposal builder is required")
	}
	if err := validateLoweredTypeEnvExtensionCandidate(builder.value); err != nil {
		return TypeEnvExtensionProposal{}, err
	}
	canonical, err := encodeLoweredTypeEnvExtensionCandidate(builder.value)
	if err != nil {
		return TypeEnvExtensionProposal{}, err
	}
	ref, err := newTypeEnvExtensionRef(builder.value.id, digestCanonicalBytes(canonical))
	if err != nil {
		return TypeEnvExtensionProposal{}, err
	}
	proposal := proposalFromCandidate(builder.value, ref, canonical)
	return cloneTypeEnvExtensionProposal(proposal), nil
}

func (proposal TypeEnvExtensionProposal) Ref() TypeEnvExtensionRef { return proposal.ref }

// CanonicalBytes returns an owned copy of the exact bytes whose SHA-256 digest
// forms the proposal reference.
func (proposal TypeEnvExtensionProposal) CanonicalBytes() []byte {
	return append([]byte(nil), proposal.canonical...)
}

func (proposal TypeEnvExtensionProposal) Edition() CarrierEdition { return proposal.edition }

func (proposal TypeEnvExtensionProposal) BaseTypeEnv() TypeEnvRef { return proposal.baseTypeEnv }

func (proposal TypeEnvExtensionProposal) BoundedContext() BoundedContextRef {
	return proposal.context
}

func (proposal TypeEnvExtensionProposal) SourceCarrier() CarrierRef { return proposal.carrier }

func (proposal TypeEnvExtensionProposal) SourceCarrierHash() SHA256Digest {
	return proposal.carrierHash
}

func (proposal TypeEnvExtensionProposal) SignatureManifest() SignatureManifest {
	manifest, _ := NewSignatureManifest(
		proposal.manifest.ref,
		proposal.manifest.imports,
		proposal.manifest.provides,
	)
	return manifest
}

func (proposal TypeEnvExtensionProposal) SchemaChanges() SchemaChangeSet {
	set, _ := NewSchemaChangeSet(proposal.changes.changes)
	return set
}

func (proposal TypeEnvExtensionProposal) CompilerSchemaVersion() CompilerSchemaVersion {
	return proposal.compiler
}

func (proposal TypeEnvExtensionProposal) CompatibilityDiff() TypeEnvCompatibilityDiff {
	diff, _ := NewTypeEnvCompatibilityDiff(
		proposal.compatibility.base,
		proposal.compatibility.changes,
	)
	return diff
}

func (proposal TypeEnvExtensionProposal) RevalidationReport() ExistingAssertionRevalidationReport {
	report, _ := NewExistingAssertionRevalidationReport(
		proposal.revalidation.posture,
		proposal.revalidation.graphRevision,
		proposal.revalidation.affectedAssertions,
		proposal.revalidation.reportDigest,
	)
	return report
}

func validateLoweredTypeEnvExtensionCandidate(candidate loweredTypeEnvExtensionCandidate) error {
	checks := []struct {
		valid   bool
		message string
	}{
		{candidate.id.valid(), "TypeEnv extension ID is required"},
		{candidate.edition.valid(), "TypeEnv extension edition is required"},
		{candidate.baseTypeEnv.valid(), "TypeEnv extension base is required"},
		{candidate.context.valid(), "TypeEnv extension bounded context is required"},
		{candidate.carrier.valid(), "TypeEnv extension source carrier is required"},
		{candidate.carrierHash.valid(), "TypeEnv extension source digest is required"},
		{candidate.manifest.valid(), "TypeEnv extension signature manifest is required"},
		{candidate.changes.valid(), "TypeEnv extension SchemaChangeSet is required"},
		{candidate.compiler.valid(), "TypeEnv extension compiler schema version is required"},
		{candidate.compatibility.valid(), "TypeEnv extension compatibility diff is required"},
		{candidate.revalidation.valid(), "TypeEnv extension revalidation report is required"},
		{candidate.compatibility.base == candidate.baseTypeEnv, "compatibility diff base does not match proposal base"},
	}
	for _, check := range checks {
		if !check.valid {
			return fmt.Errorf("%s", check.message)
		}
	}
	return validateCandidateChanges(candidate)
}

func validateCandidateChanges(candidate loweredTypeEnvExtensionCandidate) error {
	if err := validateManifestExportClosure(candidate); err != nil {
		return err
	}
	for _, change := range candidate.changes.changes {
		provenance, ok := change.Provenance().(ProjectSourceProvenance)
		if !ok {
			return fmt.Errorf("schema change %q requires project-source provenance", change.ChangeKey())
		}
		associated, exporting := schemaChangeBasisSymbols(change)
		label := "schema change " + change.ChangeKey()
		if err := validateProjectSourceBasis(candidate, label, provenance, associated, exporting); err != nil {
			return err
		}
		if err := validateNestedProjectSourceBases(candidate, change); err != nil {
			return err
		}
	}
	return nil
}

func validateManifestExportClosure(candidate loweredTypeEnvExtensionCandidate) error {
	realized := make([]SchemaSymbolRef, 0)
	seen := make(map[string]string)
	for _, change := range candidate.changes.changes {
		for _, symbol := range change.ProvidedSymbols() {
			if previous, exists := seen[symbol.String()]; exists {
				return fmt.Errorf(
					"schema symbol %q is provided by both %q and %q",
					symbol.String(),
					previous,
					change.ChangeKey(),
				)
			}
			seen[symbol.String()] = change.ChangeKey()
			realized = append(realized, symbol)
		}
	}
	canonical, err := canonicalSchemaSymbols("realized manifest provide", realized)
	if err != nil {
		return err
	}
	declared := candidate.manifest.provides
	if len(canonical) != len(declared) {
		return fmt.Errorf(
			"signature manifest provides %d symbols but SchemaChanges realize %d",
			len(declared),
			len(canonical),
		)
	}
	for index, symbol := range canonical {
		if symbol != declared[index] {
			return fmt.Errorf(
				"signature manifest provide %q has no exact SchemaChange realization",
				declared[index].String(),
			)
		}
	}
	return nil
}

func schemaChangeBasisSymbols(change SchemaChange) ([]SchemaSymbolRef, bool) {
	switch value := change.(type) {
	case DefineSubkindSchemaChange:
		subkind, _ := KindSymbolRef(value.relation.subkind)
		superkind, _ := KindSymbolRef(value.relation.superkind)
		return []SchemaSymbolRef{subkind, superkind}, false
	case DefineTypedRelationDeclarationFragmentSchemaChange:
		symbol, _ := RelationSymbolRef(value.fragment.ref.ID())
		return []SchemaSymbolRef{symbol}, true
	default:
		provided := change.ProvidedSymbols()
		return provided, len(provided) > 0
	}
}

func validateNestedProjectSourceBases(
	candidate loweredTypeEnvExtensionCandidate,
	change SchemaChange,
) error {
	fragmentChange, ok := change.(DefineTypedRelationDeclarationFragmentSchemaChange)
	if !ok {
		return nil
	}
	for _, slot := range fragmentChange.fragment.slots {
		provenance, isProjectSource := slot.provenance.(ProjectSourceProvenance)
		if !isProjectSource {
			continue
		}
		symbol, _ := SlotKindSymbolRef(fragmentChange.fragment.ref.ID(), slot.slotKind)
		label := "relation fragment slot " + fragmentChange.fragment.ref.ID().String() + "/" + slot.slotKind.String()
		if err := validateProjectSourceBasis(
			candidate,
			label,
			provenance,
			[]SchemaSymbolRef{symbol},
			true,
		); err != nil {
			return err
		}
	}
	return nil
}

func validateProjectSourceBasis(
	candidate loweredTypeEnvExtensionCandidate,
	label string,
	provenance ProjectSourceProvenance,
	associated []SchemaSymbolRef,
	exporting bool,
) error {
	if provenance.carrier != candidate.carrier ||
		provenance.edition != candidate.edition ||
		provenance.contentHash != candidate.carrierHash {
		return fmt.Errorf("%s provenance does not match proposal source", label)
	}
	if provenance.context != candidate.context || provenance.baseTypeEnv != candidate.baseTypeEnv {
		return fmt.Errorf("%s provenance does not match proposal context/base", label)
	}
	basis := provenance.manifestBasis
	if basis.manifest != candidate.manifest.ref {
		return fmt.Errorf("%s provenance names another signature manifest", label)
	}
	if !containsSchemaSymbol(associated, basis.symbol) {
		return fmt.Errorf(
			"%s provenance basis symbol %q is not associated with the declaration",
			label,
			basis.symbol.String(),
		)
	}
	if exporting {
		if basis.direction != ManifestProvide {
			return fmt.Errorf("%s exporting declaration requires a manifest provide basis", label)
		}
		if !candidate.manifest.providesSymbol(basis.symbol) {
			return fmt.Errorf("%s provenance basis symbol is absent from manifest provides", label)
		}
		return nil
	}
	if basis.direction == ManifestProvide && candidate.manifest.providesSymbol(basis.symbol) {
		return nil
	}
	if basis.direction == ManifestImport && containsSchemaSymbol(candidate.manifest.imports, basis.symbol) {
		return nil
	}
	return fmt.Errorf("%s provenance basis direction does not match the exact manifest set", label)
}

func containsSchemaSymbol(symbols []SchemaSymbolRef, expected SchemaSymbolRef) bool {
	for _, symbol := range symbols {
		if symbol == expected {
			return true
		}
	}
	return false
}

func candidateFromProposal(proposal TypeEnvExtensionProposal) loweredTypeEnvExtensionCandidate {
	return loweredTypeEnvExtensionCandidate{
		id:            proposal.ref.id,
		edition:       proposal.edition,
		baseTypeEnv:   proposal.baseTypeEnv,
		context:       proposal.context,
		carrier:       proposal.carrier,
		carrierHash:   proposal.carrierHash,
		manifest:      proposal.manifest,
		changes:       proposal.changes,
		compiler:      proposal.compiler,
		compatibility: proposal.compatibility,
		revalidation:  proposal.revalidation,
	}
}

func proposalFromCandidate(
	candidate loweredTypeEnvExtensionCandidate,
	ref TypeEnvExtensionRef,
	canonical []byte,
) TypeEnvExtensionProposal {
	return TypeEnvExtensionProposal{
		ref:           ref,
		canonical:     append([]byte(nil), canonical...),
		edition:       candidate.edition,
		baseTypeEnv:   candidate.baseTypeEnv,
		context:       candidate.context,
		carrier:       candidate.carrier,
		carrierHash:   candidate.carrierHash,
		manifest:      candidate.manifest,
		changes:       candidate.changes,
		compiler:      candidate.compiler,
		compatibility: candidate.compatibility,
		revalidation:  candidate.revalidation,
	}
}

func verifyTypeEnvExtensionProposal(proposal TypeEnvExtensionProposal) error {
	if !proposal.ref.valid() {
		return fmt.Errorf("TypeEnv extension reference is required")
	}
	if len(proposal.canonical) == 0 {
		return fmt.Errorf("TypeEnv extension canonical bytes are required")
	}
	candidate := candidateFromProposal(proposal)
	if err := validateLoweredTypeEnvExtensionCandidate(candidate); err != nil {
		return err
	}
	canonical, err := encodeLoweredTypeEnvExtensionCandidate(candidate)
	if err != nil {
		return err
	}
	if !bytes.Equal(canonical, proposal.canonical) {
		return fmt.Errorf("TypeEnv extension canonical bytes do not match semantic content")
	}
	expected, err := newTypeEnvExtensionRef(candidate.id, digestCanonicalBytes(canonical))
	if err != nil {
		return err
	}
	if expected != proposal.ref {
		return fmt.Errorf("TypeEnv extension reference does not match canonical bytes")
	}
	return nil
}

func cloneTypeEnvExtensionProposal(source TypeEnvExtensionProposal) TypeEnvExtensionProposal {
	manifest, _ := NewSignatureManifest(
		source.manifest.ref,
		source.manifest.imports,
		source.manifest.provides,
	)
	changes, _ := NewSchemaChangeSet(source.changes.changes)
	compatibility, _ := NewTypeEnvCompatibilityDiff(
		source.compatibility.base,
		source.compatibility.changes,
	)
	revalidation, _ := NewExistingAssertionRevalidationReport(
		source.revalidation.posture,
		source.revalidation.graphRevision,
		source.revalidation.affectedAssertions,
		source.revalidation.reportDigest,
	)
	return TypeEnvExtensionProposal{
		ref:           source.ref,
		canonical:     append([]byte(nil), source.canonical...),
		edition:       source.edition,
		baseTypeEnv:   source.baseTypeEnv,
		context:       source.context,
		carrier:       source.carrier,
		carrierHash:   source.carrierHash,
		manifest:      manifest,
		changes:       changes,
		compiler:      source.compiler,
		compatibility: compatibility,
		revalidation:  revalidation,
	}
}
