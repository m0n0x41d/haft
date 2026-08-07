package projecttypeenv

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"

	"github.com/m0n0x41d/haft/internal/fpf/localpractice"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const contextKindAvailabilityPlanDomain = "haft.fpf.projecttypeenv.context-kind-availability-plan.v1"

const contextKindAvailabilityBridgeCompilerRule = "haft.projecttypeenv.context-kind-availability-bridge.v1"

const (
	IssueContextKindAvailabilityInputInvalid LinkIssueCode = "context_kind_availability_input_invalid"
	IssueContextKindAvailabilityBaseInvalid  LinkIssueCode = "context_kind_availability_base_invalid"
	IssueContextKindBridgeMissing            LinkIssueCode = "context_kind_bridge_missing"
)

// ContextKindAvailabilityGroundKind identifies why one exact kind can be used
// in one exact bounded context. It is neither C.3 MemberOf nor E.24.UK durable
// U-kind admission.
type ContextKindAvailabilityGroundKind string

const (
	LocalKindDeclarationGroundKind ContextKindAvailabilityGroundKind = "local_kind_declaration"
	DirectKindUseGroundKind        ContextKindAvailabilityGroundKind = "direct_kind_use"
	BridgedKindUseGroundKind       ContextKindAvailabilityGroundKind = "bridged_kind_use"
)

// ContextKindAvailabilityGround is a closed, immutable provenance union. The
// plan retains every exact ground instead of selecting an arbitrary first use.
type ContextKindAvailabilityGround interface {
	GroundKind() ContextKindAvailabilityGroundKind
	ContextSource() SourceScalar
	ApplicabilitySource() SourceScalar
	EvidenceSource() SourceScalar
	Provider() CompositeSymbolProvider
	contextKindAvailabilityGroundVariant()
}

// LocalKindDeclarationGround proves availability from an exact value_kind
// declaration in the same Local-Practice extension and context.
type LocalKindDeclarationGround struct {
	extensionRef  typedmemory.TypeEnvExtensionRef
	coordinate    ManifestCoordinate
	context       SourceScalar
	applicability SourceScalar
	declaration   SourceScalar
	provider      ExtensionCompositeSymbolProvider
}

func (LocalKindDeclarationGround) GroundKind() ContextKindAvailabilityGroundKind {
	return LocalKindDeclarationGroundKind
}

func (ground LocalKindDeclarationGround) ExtensionRef() typedmemory.TypeEnvExtensionRef {
	return ground.extensionRef
}

func (ground LocalKindDeclarationGround) Coordinate() ManifestCoordinate {
	return ground.coordinate
}

func (ground LocalKindDeclarationGround) ContextSource() SourceScalar {
	return ground.context
}

func (ground LocalKindDeclarationGround) ApplicabilitySource() SourceScalar {
	return ground.applicability
}

func (ground LocalKindDeclarationGround) EvidenceSource() SourceScalar {
	return ground.declaration
}

func (ground LocalKindDeclarationGround) Provider() CompositeSymbolProvider {
	return cloneCompositeProvider(ground.provider)
}

func (LocalKindDeclarationGround) contextKindAvailabilityGroundVariant() {}

// DirectKindUseGround proves availability from an exact use whose provider is
// the compiled base or an imported/local extension in the same context.
type DirectKindUseGround struct {
	consumerRef     typedmemory.TypeEnvExtensionRef
	consumer        ManifestCoordinate
	context         SourceScalar
	applicability   SourceScalar
	origin          string
	role            string
	use             SourceScalar
	dependencyScope CompositeDependencyScope
	provider        CompositeSymbolProvider
}

func (DirectKindUseGround) GroundKind() ContextKindAvailabilityGroundKind {
	return DirectKindUseGroundKind
}

func (ground DirectKindUseGround) ConsumerRef() typedmemory.TypeEnvExtensionRef {
	return ground.consumerRef
}

func (ground DirectKindUseGround) ConsumerCoordinate() ManifestCoordinate {
	return ground.consumer
}

func (ground DirectKindUseGround) ContextSource() SourceScalar { return ground.context }

func (ground DirectKindUseGround) ApplicabilitySource() SourceScalar {
	return ground.applicability
}

func (ground DirectKindUseGround) EvidenceSource() SourceScalar { return ground.use }

func (ground DirectKindUseGround) Origin() string { return ground.origin }

func (ground DirectKindUseGround) Role() string { return ground.role }

func (ground DirectKindUseGround) DependencyScope() CompositeDependencyScope {
	return ground.dependencyScope
}

func (ground DirectKindUseGround) Provider() CompositeSymbolProvider {
	return cloneCompositeProvider(ground.provider)
}

func (DirectKindUseGround) contextKindAvailabilityGroundVariant() {}

// BridgedKindUseGround proves a cross-context use through one exact KindBridge.
// Several matching bridges yield several retained grounds.
type BridgedKindUseGround struct {
	consumerRef     typedmemory.TypeEnvExtensionRef
	consumer        ManifestCoordinate
	context         SourceScalar
	applicability   SourceScalar
	origin          string
	role            string
	use             SourceScalar
	dependencyScope CompositeDependencyScope
	providerContext SourceScalar
	provider        ExtensionCompositeSymbolProvider
	bridge          typedmemory.ContextBridge
}

func (BridgedKindUseGround) GroundKind() ContextKindAvailabilityGroundKind {
	return BridgedKindUseGroundKind
}

func (ground BridgedKindUseGround) ConsumerRef() typedmemory.TypeEnvExtensionRef {
	return ground.consumerRef
}

func (ground BridgedKindUseGround) ConsumerCoordinate() ManifestCoordinate {
	return ground.consumer
}

func (ground BridgedKindUseGround) ContextSource() SourceScalar { return ground.context }

func (ground BridgedKindUseGround) ApplicabilitySource() SourceScalar {
	return ground.applicability
}

func (ground BridgedKindUseGround) EvidenceSource() SourceScalar { return ground.use }

func (ground BridgedKindUseGround) Origin() string { return ground.origin }

func (ground BridgedKindUseGround) Role() string { return ground.role }

func (ground BridgedKindUseGround) DependencyScope() CompositeDependencyScope {
	return ground.dependencyScope
}

func (ground BridgedKindUseGround) ProviderContextSource() SourceScalar {
	return ground.providerContext
}

func (ground BridgedKindUseGround) Provider() CompositeSymbolProvider {
	return cloneCompositeProvider(ground.provider)
}

func (ground BridgedKindUseGround) Bridge() typedmemory.ContextBridge {
	return ground.bridge
}

func (BridgedKindUseGround) contextKindAvailabilityGroundVariant() {}

// ContextKindAvailabilityInput is a derivation result for final
// lowering. It is not a Local-Practice field, public caller fact, persisted E
// SchemaChange, MemberOf judgement, authority receipt, or E.24.UK admission.
type ContextKindAvailabilityInput struct {
	context typedmemory.BoundedContextRef
	kindID  typedmemory.KindID
	grounds []ContextKindAvailabilityGround
}

func (input ContextKindAvailabilityInput) Context() typedmemory.BoundedContextRef {
	return input.context
}

func (input ContextKindAvailabilityInput) KindID() typedmemory.KindID {
	return input.kindID
}

func (input ContextKindAvailabilityInput) Grounds() []ContextKindAvailabilityGround {
	return cloneContextKindAvailabilityGrounds(input.grounds)
}

// ContextKindAvailabilityPlan is a pure, immutable lowering plan. It has no C
// TypeEnvRef, active project head, persistence operation, or activation effect.
type ContextKindAvailabilityPlan struct {
	inputs    []ContextKindAvailabilityInput
	canonical []byte
}

func (plan ContextKindAvailabilityPlan) Inputs() []ContextKindAvailabilityInput {
	return cloneContextKindAvailabilityInputs(plan.inputs)
}

func (plan ContextKindAvailabilityPlan) CanonicalBytes() []byte {
	return append([]byte(nil), plan.canonical...)
}

type ContextKindAvailabilityPlanResolution interface {
	Rejected() bool
	Issues() []LinkIssue
	Plan() (ContextKindAvailabilityPlan, bool)
	contextKindAvailabilityPlanResolutionVariant()
}

type acceptedContextKindAvailabilityPlan struct {
	plan ContextKindAvailabilityPlan
}

func (acceptedContextKindAvailabilityPlan) Rejected() bool { return false }

func (acceptedContextKindAvailabilityPlan) Issues() []LinkIssue { return nil }

func (resolution acceptedContextKindAvailabilityPlan) Plan() (
	ContextKindAvailabilityPlan,
	bool,
) {
	return cloneContextKindAvailabilityPlan(resolution.plan), true
}

func (acceptedContextKindAvailabilityPlan) contextKindAvailabilityPlanResolutionVariant() {}

type rejectedContextKindAvailabilityPlan struct {
	issues []LinkIssue
}

func (rejectedContextKindAvailabilityPlan) Rejected() bool { return true }

func (resolution rejectedContextKindAvailabilityPlan) Issues() []LinkIssue {
	return cloneIssues(resolution.issues)
}

func (rejectedContextKindAvailabilityPlan) Plan() (ContextKindAvailabilityPlan, bool) {
	return ContextKindAvailabilityPlan{}, false
}

func (rejectedContextKindAvailabilityPlan) contextKindAvailabilityPlanResolutionVariant() {}

type contextKindAvailabilityExtension struct {
	ref           typedmemory.TypeEnvExtensionRef
	coordinate    ManifestCoordinate
	context       typedmemory.BoundedContextRef
	contextSource SourceScalar
	applicability SourceScalar
	artifact      ProjectTypeEnvExtensionArtifact
}

type contextKindAvailabilityAccumulator struct {
	context typedmemory.BoundedContextRef
	kindID  typedmemory.KindID
	grounds map[string]ContextKindAvailabilityGround
}

type contextKindBridgeMatch struct {
	bridge       typedmemory.ContextBridge
	consumerKind typedmemory.KindID
}

// DeriveContextKindAvailabilityPlan derives context-local kind availability
// only from a verified linked B+E IR. Exact declarations, uses, providers,
// applicability and bridges are the grounds; no caller-authored availability
// field is accepted.
func DeriveContextKindAvailabilityPlan(
	linked LinkedProjectTypeEnvCompositeIR,
) ContextKindAvailabilityPlanResolution {
	extensions, inputIssues := validateContextKindAvailabilityInput(linked)
	if len(inputIssues) > 0 {
		return rejectContextKindAvailabilityPlan(inputIssues)
	}

	baseEnvironment, err := typeenv.LowerBaseTypeEnvArtifact(linked.BaseArtifact())
	if err != nil {
		return rejectContextKindAvailabilityPlan([]LinkIssue{newLinkIssue(
			IssueContextKindAvailabilityBaseInvalid,
			BaseArtifactLocation{},
			linked.BaseTypeEnvRef().String(),
			"verified base artifact cannot supply the exact bridge environment: "+err.Error(),
			"repair and recompile the exact FPF base artifact",
		)})
	}

	byRef := make(map[string]contextKindAvailabilityExtension, len(extensions))
	for _, extension := range extensions {
		byRef[extension.ref.String()] = extension
	}
	accumulators := make(map[string]*contextKindAvailabilityAccumulator)
	issues := make([]LinkIssue, 0)

	for _, extension := range extensions {
		for _, declaration := range extension.artifact.IR().Signature().Vocabulary().Declarations() {
			if declaration.Kind() != localpractice.DeclarationValueKind {
				continue
			}
			symbol, _, _, symbolErr := schemaKindSymbol(declaration.Symbol().Value())
			if symbolErr != nil {
				issues = append(issues, contextKindAvailabilityInputIssue(
					extension,
					declaration.Symbol(),
					symbolErr.Error(),
				))
				continue
			}
			kindID, kindErr := typedmemory.NewKindID(symbol.Key())
			if kindErr != nil {
				issues = append(issues, contextKindAvailabilityInputIssue(
					extension,
					declaration.Symbol(),
					kindErr.Error(),
				))
				continue
			}
			provider := ExtensionCompositeSymbolProvider{
				symbol:     symbol,
				raw:        declaration.Symbol().Value(),
				coordinate: extension.coordinate,
				ref:        extension.ref,
			}
			ground := LocalKindDeclarationGround{
				extensionRef:  extension.ref,
				coordinate:    extension.coordinate,
				context:       extension.contextSource,
				applicability: extension.applicability,
				declaration:   declaration.Symbol(),
				provider:      provider,
			}
			addContextKindAvailabilityGround(
				accumulators,
				extension.context,
				kindID,
				ground,
			)
		}
	}

	bridges, bridgeIssues := contextKindAvailabilityBridges(
		baseEnvironment.ContextBridges(),
		extensions,
	)
	issues = append(issues, bridgeIssues...)
	for _, dependency := range linked.DependencyResolutions() {
		if dependency.Target().Kind() != typedmemory.KindSymbol {
			continue
		}
		consumer, exists := byRef[dependency.ConsumerRef().String()]
		if !exists {
			issues = append(issues, newLinkIssue(
				IssueContextKindAvailabilityInputInvalid,
				CompositeLinkInputLocation{},
				dependency.ConsumerRef().String(),
				"kind dependency names a consumer outside the linked extension set",
				"re-link the exact B+E artifacts before deriving availability",
			))
			continue
		}
		kindID, kindErr := typedmemory.NewKindID(dependency.Target().Key())
		if kindErr != nil {
			issues = append(issues, contextKindAvailabilityInputIssue(
				consumer,
				dependency.Source(),
				kindErr.Error(),
			))
			continue
		}

		switch provider := dependency.Provider().(type) {
		case BaseCompositeSymbolProvider:
			ground := newDirectKindUseGround(consumer, dependency, provider)
			addContextKindAvailabilityGround(accumulators, consumer.context, kindID, ground)
		case ExtensionCompositeSymbolProvider:
			providerExtension, providerExists := byRef[provider.Ref().String()]
			if !providerExists {
				issues = append(issues, contextKindAvailabilityInputIssue(
					consumer,
					dependency.Source(),
					"extension provider is outside the linked extension set",
				))
				continue
			}
			if providerExtension.context == consumer.context {
				ground := newDirectKindUseGround(consumer, dependency, provider)
				addContextKindAvailabilityGround(accumulators, consumer.context, kindID, ground)
				continue
			}
			matching := matchingContextKindBridges(
				bridges,
				providerExtension.context,
				consumer.context,
				kindID,
			)
			if len(matching) == 0 {
				issues = append(issues, newLinkIssue(
					IssueContextKindBridgeMissing,
					contextKindAvailabilityExtensionLocation(consumer),
					dependency.Target().String(),
					fmt.Sprintf(
						"cross-context kind use from %q to %q has no exact KindBridge for %q",
						providerExtension.context.String(),
						consumer.context.String(),
						kindID.String(),
					),
					"declare and import an exact KindBridge with explicit direction and mapping, or keep the use inside one bounded context",
				))
				continue
			}
			for _, match := range matching {
				ground := BridgedKindUseGround{
					consumerRef:     consumer.ref,
					consumer:        consumer.coordinate,
					context:         consumer.contextSource,
					applicability:   consumer.applicability,
					origin:          dependency.Origin(),
					role:            dependency.Role(),
					use:             dependency.Source(),
					dependencyScope: dependency.Scope(),
					providerContext: providerExtension.contextSource,
					provider:        provider,
					bridge:          match.bridge,
				}
				addContextKindAvailabilityGround(
					accumulators,
					consumer.context,
					match.consumerKind,
					ground,
				)
			}
		default:
			issues = append(issues, contextKindAvailabilityInputIssue(
				consumer,
				dependency.Source(),
				"kind dependency has an unsupported provider variant",
			))
		}
	}

	if len(issues) > 0 {
		return rejectContextKindAvailabilityPlan(issues)
	}
	inputs := canonicalContextKindAvailabilityInputs(accumulators)
	plan := ContextKindAvailabilityPlan{inputs: inputs}
	plan.canonical = canonicalContextKindAvailabilityPlan(linked, inputs)
	return acceptedContextKindAvailabilityPlan{plan: cloneContextKindAvailabilityPlan(plan)}
}

func contextKindAvailabilityBridges(
	base []typedmemory.ContextBridge,
	extensions []contextKindAvailabilityExtension,
) ([]typedmemory.ContextBridge, []LinkIssue) {
	bridges := make([]typedmemory.ContextBridge, len(base))
	copy(bridges, base)
	issues := make([]LinkIssue, 0)
	for _, extension := range extensions {
		declarations := extension.artifact.IR().Signature().Vocabulary().Declarations()
		for _, declaration := range declarations {
			if declaration.Kind() != localpractice.DeclarationKindBridge {
				continue
			}
			bridge, err := lowerContextKindAvailabilityBridge(extension, declaration)
			if err != nil {
				issues = append(issues, contextKindAvailabilityInputIssue(
					extension,
					declaration.Symbol(),
					"lower exact KindBridge: "+err.Error(),
				))
				continue
			}
			bridges = append(bridges, bridge)
		}
	}
	sort.Slice(bridges, func(left, right int) bool {
		leftID := bridges[left].ID().String()
		rightID := bridges[right].ID().String()
		if leftID != rightID {
			return leftID < rightID
		}
		leftCanonical := bridges[left].CanonicalBytes()
		rightCanonical := bridges[right].CanonicalBytes()
		return bytes.Compare(leftCanonical, rightCanonical) < 0
	})
	return bridges, issues
}

func lowerContextKindAvailabilityBridge(
	extension contextKindAvailabilityExtension,
	declaration SymbolicDeclaration,
) (typedmemory.ContextBridge, error) {
	bridgeID, err := typedmemory.NewContextBridgeID(declaration.Symbol().Value())
	if err != nil {
		return typedmemory.ContextBridge{}, fmt.Errorf("bridge ID: %w", err)
	}
	source, err := contextKindAvailabilityBridgeEndpoint(
		declaration,
		"endpoints.source.bounded_context_ref",
		"endpoints.source.edition",
	)
	if err != nil {
		return typedmemory.ContextBridge{}, fmt.Errorf("source endpoint: %w", err)
	}
	target, err := contextKindAvailabilityBridgeEndpoint(
		declaration,
		"endpoints.target.bounded_context_ref",
		"endpoints.target.edition",
	)
	if err != nil {
		return typedmemory.ContextBridge{}, fmt.Errorf("target endpoint: %w", err)
	}
	mapping, err := contextKindAvailabilityBridgeMapping(declaration)
	if err != nil {
		return typedmemory.ContextBridge{}, err
	}
	direction, err := contextKindAvailabilityBridgeDirection(declaration)
	if err != nil {
		return typedmemory.ContextBridge{}, err
	}
	orderCoverage, err := contextKindAvailabilityBridgeOrderCoverage(declaration)
	if err != nil {
		return typedmemory.ContextBridge{}, err
	}
	kindCongruence, err := contextKindAvailabilityBridgeCongruence(declaration)
	if err != nil {
		return typedmemory.ContextBridge{}, err
	}
	lossNotes, err := contextKindAvailabilityBridgeLossNotes(declaration)
	if err != nil {
		return typedmemory.ContextBridge{}, err
	}
	definednessArea, err := contextKindAvailabilityBridgeDefinednessArea(declaration)
	if err != nil {
		return typedmemory.ContextBridge{}, err
	}
	provenance, err := contextKindAvailabilityBridgeProvenance(
		extension,
		declaration,
		bridgeID,
	)
	if err != nil {
		return typedmemory.ContextBridge{}, err
	}
	bridge, err := typedmemory.NewContextBridge(typedmemory.ContextBridgeInput{
		ID:              bridgeID,
		Source:          source,
		Target:          target,
		Mapping:         mapping,
		Direction:       direction,
		OrderCoverage:   orderCoverage,
		KindCongruence:  kindCongruence,
		LossNotes:       lossNotes,
		DefinednessArea: definednessArea,
		Provenance:      provenance,
	})
	if err != nil {
		return typedmemory.ContextBridge{}, err
	}
	return bridge, nil
}

func contextKindAvailabilityBridgeEndpoint(
	declaration SymbolicDeclaration,
	contextPath string,
	editionPath string,
) (typedmemory.ContextBridgeEndpoint, error) {
	contextSource, err := requiredDeclarationFact(declaration, contextPath)
	if err != nil {
		return typedmemory.ContextBridgeEndpoint{}, err
	}
	context, err := typedmemory.NewBoundedContextRef(contextSource.Value())
	if err != nil {
		return typedmemory.ContextBridgeEndpoint{}, err
	}
	editionSource, err := requiredDeclarationFact(declaration, editionPath)
	if err != nil {
		return typedmemory.ContextBridgeEndpoint{}, err
	}
	edition, err := typedmemory.NewContextEdition(editionSource.Value())
	if err != nil {
		return typedmemory.ContextBridgeEndpoint{}, err
	}
	return typedmemory.NewContextBridgeEndpoint(context, edition)
}

func contextKindAvailabilityBridgeMapping(
	declaration SymbolicDeclaration,
) (typedmemory.NamedTargetKindMapping, error) {
	mappingKind, err := requiredDeclarationFact(declaration, "mapping.kind")
	if err != nil {
		return typedmemory.NamedTargetKindMapping{}, err
	}
	if localpractice.KindBridgeMappingKind(mappingKind.Value()) != localpractice.KindBridgeNamedTarget {
		return typedmemory.NamedTargetKindMapping{}, fmt.Errorf(
			"unsupported mapping kind %q",
			mappingKind.Value(),
		)
	}
	sourceKindSource, err := requiredDeclarationFact(declaration, "mapping.source_kind")
	if err != nil {
		return typedmemory.NamedTargetKindMapping{}, err
	}
	sourceKind, err := typedmemory.NewKindID(sourceKindSource.Value())
	if err != nil {
		return typedmemory.NamedTargetKindMapping{}, err
	}
	targetKindSource, err := requiredDeclarationFact(declaration, "mapping.target_kind")
	if err != nil {
		return typedmemory.NamedTargetKindMapping{}, err
	}
	targetKind, err := typedmemory.NewKindID(targetKindSource.Value())
	if err != nil {
		return typedmemory.NamedTargetKindMapping{}, err
	}
	return typedmemory.NewNamedTargetKindMapping(sourceKind, targetKind)
}

func contextKindAvailabilityBridgeDirection(
	declaration SymbolicDeclaration,
) (typedmemory.BridgeDirection, error) {
	direction, err := requiredDeclarationFact(declaration, "direction")
	if err != nil {
		return 0, err
	}
	switch localpractice.KindBridgeDirectionKind(direction.Value()) {
	case localpractice.KindBridgeOneWay:
		return typedmemory.OneWayBridge, nil
	case localpractice.KindBridgeTwoWay:
		return typedmemory.TwoWayBridge, nil
	default:
		return 0, fmt.Errorf("unsupported direction %q", direction.Value())
	}
}

func contextKindAvailabilityBridgeOrderCoverage(
	declaration SymbolicDeclaration,
) (typedmemory.KindBridgeOrderCoverage, error) {
	coverage, err := requiredDeclarationFact(declaration, "order_preservation")
	if err != nil {
		return 0, err
	}
	if localpractice.KindBridgeOrderPreservationKind(coverage.Value()) !=
		localpractice.KindBridgeNoOrderLinksCovered {
		return 0, fmt.Errorf("unsupported order coverage %q", coverage.Value())
	}
	return typedmemory.NoOrderLinksCovered, nil
}

func contextKindAvailabilityBridgeCongruence(
	declaration SymbolicDeclaration,
) (typedmemory.KindCongruenceLevel, error) {
	congruence, err := requiredDeclarationFact(declaration, "kind_congruence")
	if err != nil {
		return 0, err
	}
	value, err := strconv.ParseUint(congruence.Value(), 10, 8)
	if err != nil {
		return 0, err
	}
	return typedmemory.NewKindCongruenceLevel(uint8(value))
}

func contextKindAvailabilityBridgeLossNotes(
	declaration SymbolicDeclaration,
) (typedmemory.KindBridgeLossNotes, error) {
	sources, err := indexedDeclarationFacts(declaration, "loss_notes")
	if err != nil {
		return typedmemory.KindBridgeLossNotes{}, err
	}
	values := contextKindAvailabilitySourceValues(sources)
	return typedmemory.NewKindBridgeLossNotes(values)
}

func contextKindAvailabilityBridgeDefinednessArea(
	declaration SymbolicDeclaration,
) (typedmemory.KindBridgeDefinednessArea, error) {
	sources, err := indexedDeclarationFacts(declaration, "definedness_area")
	if err != nil {
		return typedmemory.KindBridgeDefinednessArea{}, err
	}
	values := contextKindAvailabilitySourceValues(sources)
	return typedmemory.NewKindBridgeDefinednessArea(values)
}

func contextKindAvailabilitySourceValues(sources []SourceScalar) []string {
	values := make([]string, 0, len(sources))
	for _, source := range sources {
		values = append(values, source.Value())
	}
	return values
}

func contextKindAvailabilityBridgeProvenance(
	extension contextKindAvailabilityExtension,
	declaration SymbolicDeclaration,
	bridgeID typedmemory.ContextBridgeID,
) (typedmemory.ProjectSourceProvenance, error) {
	ir := extension.artifact.IR()
	reference, err := typedmemory.NewProvenanceRef(
		extension.ref.String() + "#bridge:" + bridgeID.String(),
	)
	if err != nil {
		return typedmemory.ProjectSourceProvenance{}, err
	}
	carrier, err := typedmemory.NewCarrierRef(ir.Carrier().ID().Value())
	if err != nil {
		return typedmemory.ProjectSourceProvenance{}, err
	}
	edition, err := typedmemory.NewCarrierEdition(ir.Carrier().Edition().Value())
	if err != nil {
		return typedmemory.ProjectSourceProvenance{}, err
	}
	lineRange, err := typedmemory.NewSourceLineRange(
		declaration.Span().Start(),
		declaration.Span().End(),
	)
	if err != nil {
		return typedmemory.ProjectSourceProvenance{}, err
	}
	rule, err := typedmemory.NewCompilerRuleID(contextKindAvailabilityBridgeCompilerRule)
	if err != nil {
		return typedmemory.ProjectSourceProvenance{}, err
	}
	context, err := typedmemory.NewBoundedContextRef(ir.BoundedContext().Value())
	if err != nil {
		return typedmemory.ProjectSourceProvenance{}, err
	}
	manifest := ir.Manifest()
	manifestRef, err := typedmemory.NewSignatureManifestRef(
		manifest.ID().Value(),
		manifest.Version().Value(),
	)
	if err != nil {
		return typedmemory.ProjectSourceProvenance{}, err
	}
	symbol, err := typedmemory.ContextBridgeSymbolRef(bridgeID)
	if err != nil {
		return typedmemory.ProjectSourceProvenance{}, err
	}
	basis, err := typedmemory.NewManifestSymbolBasis(
		manifestRef,
		typedmemory.ManifestProvide,
		symbol,
	)
	if err != nil {
		return typedmemory.ProjectSourceProvenance{}, err
	}
	builder := typedmemory.NewProjectSourceProvenanceBuilder(
		reference,
		carrier,
		edition,
		ir.Carrier().Digest(),
	)
	builder = builder.SetDeclarationRange(lineRange)
	builder = builder.SetCompilerRule(rule)
	builder = builder.SetBoundedContext(context)
	builder = builder.SetBaseTypeEnv(ir.BaseTypeEnvRef())
	builder = builder.SetSignatureBlockRow(typedmemory.VocabularyRow)
	builder = builder.SetManifestBasis(basis)
	return builder.Build()
}

func validateContextKindAvailabilityInput(
	linked LinkedProjectTypeEnvCompositeIR,
) ([]contextKindAvailabilityExtension, []LinkIssue) {
	if err := linked.base.Verify(); err != nil {
		return nil, []LinkIssue{newLinkIssue(
			IssueContextKindAvailabilityInputInvalid,
			BaseArtifactLocation{},
			"compiled-fpf-base",
			err.Error(),
			"supply a verified linked composite IR",
		)}
	}
	baseRef, exists := linked.base.TypeEnvRef()
	if !exists || baseRef != linked.baseRef {
		return nil, []LinkIssue{newLinkIssue(
			IssueContextKindAvailabilityInputInvalid,
			CompositeLinkInputLocation{},
			linked.baseRef.String(),
			"linked base reference does not equal the verified base artifact reference",
			"re-link the exact B+E artifacts before deriving availability",
		)}
	}
	if len(linked.canonical) == 0 || !bytes.Equal(linked.canonical, canonicalCompositeLinkIR(linked)) {
		return nil, []LinkIssue{newLinkIssue(
			IssueContextKindAvailabilityInputInvalid,
			CompositeLinkInputLocation{},
			"linked-composite-ir",
			"linked composite canonical evidence is empty or differs from its semantic fields",
			"re-link the exact B+E artifacts before deriving availability",
		)}
	}

	extensions := make([]contextKindAvailabilityExtension, 0, len(linked.extensions))
	issues := make([]LinkIssue, 0)
	for _, extension := range linked.extensions {
		artifact := extension.Artifact()
		if err := artifact.Verify(); err != nil {
			issues = append(issues, newLinkIssue(
				IssueContextKindAvailabilityInputInvalid,
				CompositeLinkInputLocation{},
				extension.Ref().String(),
				err.Error(),
				"re-link the exact verified extension artifact",
			))
			continue
		}
		compiled, contextErr := contextKindAvailabilityExtensionFromLinked(extension)
		if contextErr != nil {
			issues = append(issues, newLinkIssue(
				IssueContextKindAvailabilityInputInvalid,
				newCompositeExtensionLocation(
					extension.Coordinate(),
					extension.Ref(),
					artifact.IR().Signature().Applicability().Span(),
				),
				extension.Ref().String(),
				contextErr.Error(),
				"repair the exact bounded-context and Applicability declarations, then reseal and relink",
			))
			continue
		}
		extensions = append(extensions, compiled)
	}
	sortIssues(issues)
	return extensions, issues
}

func contextKindAvailabilityExtensionFromLinked(
	extension LinkedCompositeExtension,
) (contextKindAvailabilityExtension, error) {
	ir := extension.Artifact().IR()
	contextSource := ir.BoundedContext()
	context, err := typedmemory.NewBoundedContextRef(contextSource.Value())
	if err != nil {
		return contextKindAvailabilityExtension{}, err
	}
	applicability, err := exactApplicabilityContext(ir.Signature().Applicability())
	if err != nil {
		return contextKindAvailabilityExtension{}, err
	}
	if applicability.Value() != contextSource.Value() {
		return contextKindAvailabilityExtension{}, fmt.Errorf(
			"Applicability context %q differs from carrier context %q",
			applicability.Value(),
			contextSource.Value(),
		)
	}
	return contextKindAvailabilityExtension{
		ref:           extension.Ref(),
		coordinate:    extension.Coordinate(),
		context:       context,
		contextSource: contextSource,
		applicability: applicability,
		artifact:      extension.Artifact(),
	}, nil
}

func exactApplicabilityContext(row SignatureRowIR) (SourceScalar, error) {
	var result SourceScalar
	count := 0
	for _, fact := range row.Facts() {
		if fact.Path() != "bounded_context_ref" {
			continue
		}
		result = fact.Value()
		count++
	}
	if count != 1 {
		return SourceScalar{}, fmt.Errorf(
			"Signature Applicability has %d bounded_context_ref facts; want exactly one",
			count,
		)
	}
	return result, nil
}

func newDirectKindUseGround(
	consumer contextKindAvailabilityExtension,
	dependency CompositeDependencyResolution,
	provider CompositeSymbolProvider,
) DirectKindUseGround {
	return DirectKindUseGround{
		consumerRef:     consumer.ref,
		consumer:        consumer.coordinate,
		context:         consumer.contextSource,
		applicability:   consumer.applicability,
		origin:          dependency.Origin(),
		role:            dependency.Role(),
		use:             dependency.Source(),
		dependencyScope: dependency.Scope(),
		provider:        cloneCompositeProvider(provider),
	}
}

func matchingContextKindBridges(
	bridges []typedmemory.ContextBridge,
	providerContext typedmemory.BoundedContextRef,
	consumerContext typedmemory.BoundedContextRef,
	providerKind typedmemory.KindID,
) []contextKindBridgeMatch {
	result := make([]contextKindBridgeMatch, 0)
	for _, bridge := range bridges {
		source := bridge.Source()
		target := bridge.Target()
		mapping := bridge.Mapping()
		forward := source.Context() == providerContext &&
			target.Context() == consumerContext &&
			mapping.SourceKind() == providerKind
		reverse := bridge.Direction() == typedmemory.TwoWayBridge &&
			target.Context() == providerContext &&
			source.Context() == consumerContext &&
			mapping.TargetKind() == providerKind
		if forward {
			result = append(result, contextKindBridgeMatch{
				bridge:       bridge,
				consumerKind: mapping.TargetKind(),
			})
		}
		if reverse {
			result = append(result, contextKindBridgeMatch{
				bridge:       bridge,
				consumerKind: mapping.SourceKind(),
			})
		}
	}
	sort.Slice(result, func(left, right int) bool {
		leftKey := result[left].bridge.ID().String() + "\x00" + result[left].consumerKind.String()
		rightKey := result[right].bridge.ID().String() + "\x00" + result[right].consumerKind.String()
		return leftKey < rightKey
	})
	return result
}

func addContextKindAvailabilityGround(
	accumulators map[string]*contextKindAvailabilityAccumulator,
	context typedmemory.BoundedContextRef,
	kindID typedmemory.KindID,
	ground ContextKindAvailabilityGround,
) {
	key := context.String() + "\x00" + kindID.String()
	accumulator, exists := accumulators[key]
	if !exists {
		accumulator = &contextKindAvailabilityAccumulator{
			context: context,
			kindID:  kindID,
			grounds: make(map[string]ContextKindAvailabilityGround),
		}
		accumulators[key] = accumulator
	}
	groundKey := string(canonicalContextKindAvailabilityGround(ground))
	accumulator.grounds[groundKey] = cloneContextKindAvailabilityGround(ground)
}

func canonicalContextKindAvailabilityInputs(
	accumulators map[string]*contextKindAvailabilityAccumulator,
) []ContextKindAvailabilityInput {
	keys := make([]string, 0, len(accumulators))
	for key := range accumulators {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	inputs := make([]ContextKindAvailabilityInput, 0, len(keys))
	for _, key := range keys {
		accumulator := accumulators[key]
		groundKeys := make([]string, 0, len(accumulator.grounds))
		for groundKey := range accumulator.grounds {
			groundKeys = append(groundKeys, groundKey)
		}
		sort.Strings(groundKeys)
		grounds := make([]ContextKindAvailabilityGround, 0, len(groundKeys))
		for _, groundKey := range groundKeys {
			grounds = append(grounds, cloneContextKindAvailabilityGround(accumulator.grounds[groundKey]))
		}
		inputs = append(inputs, ContextKindAvailabilityInput{
			context: accumulator.context,
			kindID:  accumulator.kindID,
			grounds: grounds,
		})
	}
	return inputs
}

func canonicalContextKindAvailabilityPlan(
	linked LinkedProjectTypeEnvCompositeIR,
	inputs []ContextKindAvailabilityInput,
) []byte {
	writer := compositeLinkWriter{}
	writer.addString(contextKindAvailabilityPlanDomain)
	writer.addString(linked.BaseTypeEnvRef().String())
	writer.addString(linked.BaseArtifact().Digest().String())
	writer.addString(string(linked.CanonicalBytes()))
	writer.addUint(uint64(len(inputs)))
	for _, input := range inputs {
		writer.addString(input.Context().String())
		writer.addString(input.KindID().String())
		writer.addUint(uint64(len(input.grounds)))
		for _, ground := range input.grounds {
			writer.addString(string(canonicalContextKindAvailabilityGround(ground)))
		}
	}
	return writer.bytes()
}

func canonicalContextKindAvailabilityGround(
	ground ContextKindAvailabilityGround,
) []byte {
	writer := compositeLinkWriter{}
	writer.addString(string(ground.GroundKind()))
	writer.addString(compositeSourceScalarKey(ground.ContextSource()))
	writer.addString(compositeSourceScalarKey(ground.ApplicabilitySource()))
	writer.addString(compositeSourceScalarKey(ground.EvidenceSource()))
	writer.addString(compositeProviderKey(ground.Provider()))
	switch value := ground.(type) {
	case LocalKindDeclarationGround:
		writer.addString(value.extensionRef.String())
		writer.addString(value.coordinate.String())
	case DirectKindUseGround:
		writer.addString(value.consumerRef.String())
		writer.addString(value.consumer.String())
		writer.addString(value.origin)
		writer.addString(value.role)
		writer.addString(string(value.dependencyScope))
	case BridgedKindUseGround:
		writer.addString(value.consumerRef.String())
		writer.addString(value.consumer.String())
		writer.addString(value.origin)
		writer.addString(value.role)
		writer.addString(string(value.dependencyScope))
		writer.addString(compositeSourceScalarKey(value.providerContext))
		writer.addString(string(value.bridge.CanonicalBytes()))
	default:
		writer.addString("invalid")
	}
	return writer.bytes()
}

func contextKindAvailabilityInputIssue(
	extension contextKindAvailabilityExtension,
	source SourceScalar,
	detail string,
) LinkIssue {
	return newLinkIssue(
		IssueContextKindAvailabilityInputInvalid,
		newCompositeExtensionLocation(extension.coordinate, extension.ref, source.Span()),
		source.Value(),
		detail,
		"repair the exact source declaration, then reseal and relink",
	)
}

func contextKindAvailabilityExtensionLocation(
	extension contextKindAvailabilityExtension,
) CompositeExtensionLocation {
	return newCompositeExtensionLocation(
		extension.coordinate,
		extension.ref,
		extension.applicability.Span(),
	)
}

func rejectContextKindAvailabilityPlan(
	issues []LinkIssue,
) ContextKindAvailabilityPlanResolution {
	owned := cloneIssues(issues)
	sortIssues(owned)
	return rejectedContextKindAvailabilityPlan{issues: owned}
}

func cloneContextKindAvailabilityGround(
	ground ContextKindAvailabilityGround,
) ContextKindAvailabilityGround {
	switch value := ground.(type) {
	case LocalKindDeclarationGround:
		value.provider = cloneCompositeProvider(value.provider).(ExtensionCompositeSymbolProvider)
		return value
	case DirectKindUseGround:
		value.provider = cloneCompositeProvider(value.provider)
		return value
	case BridgedKindUseGround:
		value.provider = cloneCompositeProvider(value.provider).(ExtensionCompositeSymbolProvider)
		return value
	default:
		return nil
	}
}

func cloneContextKindAvailabilityGrounds(
	grounds []ContextKindAvailabilityGround,
) []ContextKindAvailabilityGround {
	result := make([]ContextKindAvailabilityGround, 0, len(grounds))
	for _, ground := range grounds {
		result = append(result, cloneContextKindAvailabilityGround(ground))
	}
	return result
}

func cloneContextKindAvailabilityInputs(
	inputs []ContextKindAvailabilityInput,
) []ContextKindAvailabilityInput {
	result := append([]ContextKindAvailabilityInput(nil), inputs...)
	for index := range result {
		result[index].grounds = cloneContextKindAvailabilityGrounds(result[index].grounds)
	}
	return result
}

func cloneContextKindAvailabilityPlan(
	plan ContextKindAvailabilityPlan,
) ContextKindAvailabilityPlan {
	plan.inputs = cloneContextKindAvailabilityInputs(plan.inputs)
	plan.canonical = append([]byte(nil), plan.canonical...)
	return plan
}
