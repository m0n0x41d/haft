package projecttypeenv

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/fpf/localpractice"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const compositeAvailabilitySourceCompilerRule = "haft.projecttypeenv.context-kind-availability-source.v1"

type compositeAvailabilityLowering struct {
	linked     LinkedProjectTypeEnvCompositeIR
	base       typedmemory.TypeEnv
	byRef      map[string]LinkedCompositeExtension
	bySymbol   map[string]compositeSourceDeclaration
	contextDec map[string]compositeSourceDeclaration
}

func lowerCompositeContextKindAvailabilities(
	linked LinkedProjectTypeEnvCompositeIR,
) ([]typedmemory.ContextKindAvailability, error) {
	resolution := DeriveContextKindAvailabilityPlan(linked)
	if resolution.Rejected() {
		issues := resolution.Issues()
		if len(issues) == 0 {
			return nil, fmt.Errorf("context-kind availability plan was rejected without an issue")
		}
		issue := issues[0]
		return nil, fmt.Errorf(
			"derive context-kind availability plan: %s at %s: %s; repair: %s",
			issue.Code(),
			issue.Location().String(),
			issue.Detail(),
			issue.Repair(),
		)
	}
	plan, exists := resolution.Plan()
	if !exists {
		return nil, fmt.Errorf("accepted context-kind availability plan is absent")
	}
	base, err := typeenv.LowerBaseTypeEnvArtifact(linked.BaseArtifact())
	if err != nil {
		return nil, fmt.Errorf("lower exact B for availability provider provenance: %w", err)
	}
	context, err := newCompositeAvailabilityLowering(linked, base)
	if err != nil {
		return nil, err
	}
	result := make([]typedmemory.ContextKindAvailability, 0, len(plan.Inputs()))
	for _, input := range plan.Inputs() {
		grounds := make([]typedmemory.ContextKindAvailabilityGround, 0, len(input.Grounds()))
		for _, sourceGround := range input.Grounds() {
			ground, err := context.lowerGround(input, sourceGround)
			if err != nil {
				return nil, fmt.Errorf(
					"lower availability %s/%s ground %s: %w",
					input.Context().String(),
					input.KindID().String(),
					sourceGround.GroundKind(),
					err,
				)
			}
			grounds = append(grounds, ground)
		}
		set, err := typedmemory.NewContextKindAvailabilityGroundSet(grounds)
		if err != nil {
			return nil, err
		}
		availability, err := typedmemory.NewContextKindAvailability(
			input.Context(),
			input.KindID(),
			set,
		)
		if err != nil {
			return nil, err
		}
		result = append(result, availability)
	}
	return result, nil
}

func newCompositeAvailabilityLowering(
	linked LinkedProjectTypeEnvCompositeIR,
	base typedmemory.TypeEnv,
) (compositeAvailabilityLowering, error) {
	result := compositeAvailabilityLowering{
		linked:     linked,
		base:       base,
		byRef:      make(map[string]LinkedCompositeExtension),
		bySymbol:   make(map[string]compositeSourceDeclaration),
		contextDec: make(map[string]compositeSourceDeclaration),
	}
	for _, extension := range linked.Extensions() {
		ref := extension.Ref().String()
		result.byRef[ref] = extension
		for _, declaration := range extension.Artifact().IR().Signature().Vocabulary().Declarations() {
			source := compositeSourceDeclaration{extension: extension, value: declaration}
			result.bySymbol[ref+"\x00"+declaration.Symbol().Value()] = source
			if declaration.Kind() == localpractice.DeclarationBoundedContext {
				result.contextDec[ref] = source
			}
		}
		if _, exists := result.contextDec[ref]; !exists {
			return compositeAvailabilityLowering{}, fmt.Errorf(
				"extension %q lacks the explicit bounded_context declaration required for availability provenance",
				ref,
			)
		}
	}
	return result, nil
}

func (lowering compositeAvailabilityLowering) lowerGround(
	input ContextKindAvailabilityInput,
	ground ContextKindAvailabilityGround,
) (typedmemory.ContextKindAvailabilityGround, error) {
	switch value := ground.(type) {
	case LocalKindDeclarationGround:
		return lowering.lowerLocalGround(input, value)
	case DirectKindUseGround:
		return lowering.lowerDirectGround(input, value)
	case BridgedKindUseGround:
		return lowering.lowerBridgedGround(input, value)
	default:
		return nil, fmt.Errorf("unsupported availability ground %T", ground)
	}
}

func (lowering compositeAvailabilityLowering) lowerLocalGround(
	input ContextKindAvailabilityInput,
	ground LocalKindDeclarationGround,
) (typedmemory.ContextKindAvailabilityGround, error) {
	providerValue, ok := ground.Provider().(ExtensionCompositeSymbolProvider)
	if !ok {
		return nil, fmt.Errorf("local ground provider is %T, want extension provider", ground.Provider())
	}
	provider, err := lowering.extensionProvider(providerValue)
	if err != nil {
		return nil, err
	}
	contextSource := provider.ContextSource()
	applicability, err := lowering.extensionContextSource(
		ground.ExtensionRef(),
		ground.ApplicabilitySource(),
		"local-applicability",
	)
	if err != nil {
		return nil, err
	}
	return typedmemory.NewLocalContextKindAvailabilityGround(
		typedmemory.LocalContextKindAvailabilityGroundInput{
			Context:             input.Context(),
			KindID:              input.KindID(),
			ContextSource:       contextSource,
			ApplicabilitySource: applicability,
			Provider:            provider,
		},
	)
}

func (lowering compositeAvailabilityLowering) lowerDirectGround(
	input ContextKindAvailabilityInput,
	ground DirectKindUseGround,
) (typedmemory.ContextKindAvailabilityGround, error) {
	contextSource, err := lowering.extensionContextSource(
		ground.ConsumerRef(),
		ground.ContextSource(),
		"direct-context",
	)
	if err != nil {
		return nil, err
	}
	applicability, err := lowering.extensionContextSource(
		ground.ConsumerRef(),
		ground.ApplicabilitySource(),
		"direct-applicability",
	)
	if err != nil {
		return nil, err
	}
	useSource, err := lowering.extensionUseSource(
		ground.ConsumerRef(),
		ground.Origin(),
		ground.Role(),
		ground.EvidenceSource(),
		"direct-use:"+ground.Role(),
	)
	if err != nil {
		return nil, err
	}
	provider, err := lowering.provider(ground.Provider())
	if err != nil {
		return nil, err
	}
	scope, err := typedAvailabilityScope(ground.DependencyScope())
	if err != nil {
		return nil, err
	}
	return typedmemory.NewDirectContextKindAvailabilityGround(
		typedmemory.DirectContextKindAvailabilityGroundInput{
			ConsumerExtension:   ground.ConsumerRef(),
			Context:             input.Context(),
			KindID:              input.KindID(),
			ContextSource:       contextSource,
			ApplicabilitySource: applicability,
			UseSource:           useSource,
			Origin:              ground.Origin(),
			Role:                ground.Role(),
			Scope:               scope,
			Provider:            provider,
		},
	)
}

func (lowering compositeAvailabilityLowering) lowerBridgedGround(
	input ContextKindAvailabilityInput,
	ground BridgedKindUseGround,
) (typedmemory.ContextKindAvailabilityGround, error) {
	contextSource, err := lowering.extensionContextSource(
		ground.ConsumerRef(),
		ground.ContextSource(),
		"bridged-context",
	)
	if err != nil {
		return nil, err
	}
	applicability, err := lowering.extensionContextSource(
		ground.ConsumerRef(),
		ground.ApplicabilitySource(),
		"bridged-applicability",
	)
	if err != nil {
		return nil, err
	}
	useSource, err := lowering.extensionUseSource(
		ground.ConsumerRef(),
		ground.Origin(),
		ground.Role(),
		ground.EvidenceSource(),
		"bridged-use:"+ground.Role(),
	)
	if err != nil {
		return nil, err
	}
	providerValue, ok := ground.Provider().(ExtensionCompositeSymbolProvider)
	if !ok {
		return nil, fmt.Errorf("bridged ground provider is %T, want extension provider", ground.Provider())
	}
	provider, err := lowering.extensionProvider(providerValue)
	if err != nil {
		return nil, err
	}
	basis, err := lowering.bridgeBasis(ground.Bridge())
	if err != nil {
		return nil, err
	}
	return typedmemory.NewBridgedContextKindAvailabilityGround(
		typedmemory.BridgedContextKindAvailabilityGroundInput{
			ConsumerExtension:   ground.ConsumerRef(),
			Context:             input.Context(),
			KindID:              input.KindID(),
			ContextSource:       contextSource,
			ApplicabilitySource: applicability,
			UseSource:           useSource,
			Origin:              ground.Origin(),
			Role:                ground.Role(),
			Provider:            provider,
			BridgeBasis:         basis,
		},
	)
}

func (lowering compositeAvailabilityLowering) provider(
	provider CompositeSymbolProvider,
) (typedmemory.ContextKindAvailabilityProvider, error) {
	switch value := provider.(type) {
	case BaseCompositeSymbolProvider:
		return lowering.baseProvider(value)
	case ExtensionCompositeSymbolProvider:
		return lowering.extensionProvider(value)
	default:
		return nil, fmt.Errorf("unsupported provider %T", provider)
	}
}

func (lowering compositeAvailabilityLowering) baseProvider(
	provider BaseCompositeSymbolProvider,
) (typedmemory.ContextKindAvailabilityProvider, error) {
	kindID, err := typedmemory.NewKindID(provider.Symbol().Key())
	if err != nil {
		return nil, err
	}
	definition, exists := lowering.base.KindDefinition(kindID)
	if !exists {
		return nil, fmt.Errorf("base provider kind %q is absent from exact B", kindID.String())
	}
	source, err := typedmemory.NewContextKindAvailabilitySource(
		provider.Symbol().Key(),
		definition.Provenance(),
	)
	if err != nil {
		return nil, err
	}
	return typedmemory.NewBaseKindAvailabilityProvider(
		lowering.linked.BaseTypeEnvRef(),
		provider.Symbol(),
		source,
	)
}

func (lowering compositeAvailabilityLowering) extensionProvider(
	provider ExtensionCompositeSymbolProvider,
) (typedmemory.ExtensionKindAvailabilityProvider, error) {
	extension, exists := lowering.byRef[provider.Ref().String()]
	if !exists {
		return typedmemory.ExtensionKindAvailabilityProvider{}, fmt.Errorf(
			"extension provider %q is outside linked E",
			provider.Ref().String(),
		)
	}
	ir := extension.Artifact().IR()
	context, err := typedmemory.NewBoundedContextRef(ir.BoundedContext().Value())
	if err != nil {
		return typedmemory.ExtensionKindAvailabilityProvider{}, err
	}
	contextSource, err := lowering.extensionContextSource(
		provider.Ref(),
		ir.BoundedContext(),
		"provider-context",
	)
	if err != nil {
		return typedmemory.ExtensionKindAvailabilityProvider{}, err
	}
	key := provider.Ref().String() + "\x00" + provider.RawSymbol()
	declaration, exists := lowering.bySymbol[key]
	if !exists || declaration.value.Kind() != localpractice.DeclarationValueKind {
		return typedmemory.ExtensionKindAvailabilityProvider{}, fmt.Errorf(
			"extension provider declaration %q is not an exact value_kind",
			provider.RawSymbol(),
		)
	}
	declarationSource, err := lowering.projectSource(
		declaration,
		declaration.value.Symbol(),
		"provider-kind",
	)
	if err != nil {
		return typedmemory.ExtensionKindAvailabilityProvider{}, err
	}
	return typedmemory.NewExtensionKindAvailabilityProvider(
		typedmemory.ExtensionKindAvailabilityProviderInput{
			ExtensionRef:      provider.Ref(),
			Context:           context,
			ContextSource:     contextSource,
			Symbol:            provider.Symbol(),
			DeclarationSource: declarationSource,
		},
	)
}

func (lowering compositeAvailabilityLowering) extensionContextSource(
	extensionRef typedmemory.TypeEnvExtensionRef,
	scalar SourceScalar,
	semantic string,
) (typedmemory.ContextKindAvailabilitySource, error) {
	declaration, exists := lowering.contextDec[extensionRef.String()]
	if !exists {
		return typedmemory.ContextKindAvailabilitySource{}, fmt.Errorf(
			"extension %q has no explicit bounded_context declaration",
			extensionRef.String(),
		)
	}
	return lowering.projectSource(declaration, scalar, semantic)
}

func (lowering compositeAvailabilityLowering) extensionUseSource(
	extensionRef typedmemory.TypeEnvExtensionRef,
	origin string,
	role string,
	scalar SourceScalar,
	semantic string,
) (typedmemory.ContextKindAvailabilitySource, error) {
	symbol := strings.TrimPrefix(origin, "declaration:")
	if symbol != origin && symbol != "" {
		declaration, exists := lowering.bySymbol[extensionRef.String()+"\x00"+symbol]
		if !exists {
			return typedmemory.ContextKindAvailabilitySource{}, fmt.Errorf(
				"availability use declaration %q is absent from consumer E",
				symbol,
			)
		}
		return lowering.projectSource(declaration, scalar, semantic)
	}

	extension, exists := lowering.byRef[extensionRef.String()]
	if !exists {
		return typedmemory.ContextKindAvailabilitySource{}, fmt.Errorf(
			"availability use consumer %q is outside linked E",
			extensionRef.String(),
		)
	}
	basis, err := lowering.dependencyManifestBasis(extensionRef, origin, role)
	if err != nil {
		return typedmemory.ContextKindAvailabilitySource{}, err
	}
	return lowering.projectSourceWithBasis(extension, scalar, semantic, basis)
}

func (lowering compositeAvailabilityLowering) projectSource(
	declaration compositeSourceDeclaration,
	scalar SourceScalar,
	semantic string,
) (typedmemory.ContextKindAvailabilitySource, error) {
	ir := declaration.extension.Artifact().IR()
	manifest, err := typedmemory.NewSignatureManifestRef(
		ir.Manifest().ID().Value(),
		ir.Manifest().Version().Value(),
	)
	if err != nil {
		return typedmemory.ContextKindAvailabilitySource{}, err
	}
	basis, err := compositeDeclarationManifestBasis(lowering.linked, declaration, manifest)
	if err != nil {
		return typedmemory.ContextKindAvailabilitySource{}, err
	}
	return lowering.projectSourceWithBasis(declaration.extension, scalar, semantic, basis)
}

func (lowering compositeAvailabilityLowering) projectSourceWithBasis(
	extension LinkedCompositeExtension,
	scalar SourceScalar,
	semantic string,
	basis typedmemory.ManifestSymbolBasis,
) (typedmemory.ContextKindAvailabilitySource, error) {
	ir := extension.Artifact().IR()
	reference, err := typedmemory.NewProvenanceRef(fmt.Sprintf(
		"%s#availability-source:%d:%d:%s",
		extension.Ref().String(),
		scalar.Span().Start(),
		scalar.Span().End(),
		semantic,
	))
	if err != nil {
		return typedmemory.ContextKindAvailabilitySource{}, err
	}
	carrier, err := typedmemory.NewCarrierRef(ir.Carrier().ID().Value())
	if err != nil {
		return typedmemory.ContextKindAvailabilitySource{}, err
	}
	edition, err := typedmemory.NewCarrierEdition(ir.Carrier().Edition().Value())
	if err != nil {
		return typedmemory.ContextKindAvailabilitySource{}, err
	}
	lineRange, err := typedmemory.NewSourceLineRange(scalar.Span().Start(), scalar.Span().End())
	if err != nil {
		return typedmemory.ContextKindAvailabilitySource{}, err
	}
	rule, err := typedmemory.NewCompilerRuleID(compositeAvailabilitySourceCompilerRule)
	if err != nil {
		return typedmemory.ContextKindAvailabilitySource{}, err
	}
	context, err := typedmemory.NewBoundedContextRef(ir.BoundedContext().Value())
	if err != nil {
		return typedmemory.ContextKindAvailabilitySource{}, err
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
	provenance, err := builder.Build()
	if err != nil {
		return typedmemory.ContextKindAvailabilitySource{}, err
	}
	return typedmemory.NewContextKindAvailabilitySource(scalar.Value(), provenance)
}

func (lowering compositeAvailabilityLowering) dependencyManifestBasis(
	extensionRef typedmemory.TypeEnvExtensionRef,
	origin string,
	role string,
) (typedmemory.ManifestSymbolBasis, error) {
	extension, exists := lowering.byRef[extensionRef.String()]
	if !exists {
		return typedmemory.ManifestSymbolBasis{}, fmt.Errorf(
			"availability dependency consumer %q is outside linked E",
			extensionRef.String(),
		)
	}
	ir := extension.Artifact().IR()
	manifest, err := typedmemory.NewSignatureManifestRef(
		ir.Manifest().ID().Value(),
		ir.Manifest().Version().Value(),
	)
	if err != nil {
		return typedmemory.ManifestSymbolBasis{}, err
	}
	for _, resolution := range lowering.linked.DependencyResolutions() {
		if resolution.ConsumerRef() != extensionRef ||
			resolution.Origin() != origin ||
			resolution.Role() != role {
			continue
		}
		direction := typedmemory.ManifestImport
		if resolution.Scope() == CompositeDependencyOwn {
			direction = typedmemory.ManifestProvide
		}
		return typedmemory.NewManifestSymbolBasis(manifest, direction, resolution.Target())
	}
	return typedmemory.ManifestSymbolBasis{}, fmt.Errorf(
		"availability use %q/%q has no exact linked dependency resolution",
		origin,
		role,
	)
}

func (lowering compositeAvailabilityLowering) bridgeBasis(
	bridge typedmemory.ContextBridge,
) (typedmemory.ContextKindAvailabilityBridgeBasis, error) {
	if _, projectSource := bridge.Provenance().(typedmemory.ProjectSourceProvenance); projectSource {
		extensions := lowering.linked.Extensions()
		sort.Slice(extensions, func(left, right int) bool {
			return extensions[left].Ref().String() < extensions[right].Ref().String()
		})
		for _, extension := range extensions {
			basis, err := typedmemory.NewExtensionKindAvailabilityBridgeBasis(
				extension.Ref(),
				bridge,
			)
			if err == nil {
				return basis, nil
			}
		}
		return nil, fmt.Errorf("project-source KindBridge does not match any linked E artifact")
	}
	basis, err := typedmemory.NewBaseKindAvailabilityBridgeBasis(
		lowering.linked.BaseTypeEnvRef(),
		bridge,
	)
	if err != nil {
		return nil, err
	}
	return basis, nil
}

func typedAvailabilityScope(
	scope CompositeDependencyScope,
) (typedmemory.ContextKindUseScope, error) {
	switch scope {
	case CompositeDependencyBase:
		return typedmemory.BaseContextKindUse, nil
	case CompositeDependencyOwn:
		return typedmemory.OwnExtensionContextKindUse, nil
	case CompositeDependencyImported:
		return typedmemory.ImportedExtensionContextKindUse, nil
	default:
		return 0, fmt.Errorf("unsupported dependency scope %q", scope)
	}
}

func mergeCompositeContextKindAvailabilities(
	base []typedmemory.ContextKindAvailability,
	derived []typedmemory.ContextKindAvailability,
) ([]typedmemory.ContextKindAvailability, error) {
	byCoordinate := make(map[string]typedmemory.ContextKindAvailability)
	for _, availability := range append(
		append([]typedmemory.ContextKindAvailability(nil), base...),
		derived...,
	) {
		key := availability.Context().String() + "\x00" + availability.KindID().String()
		previous, exists := byCoordinate[key]
		if !exists {
			byCoordinate[key] = availability
			continue
		}
		if !bytes.Equal(previous.CanonicalBytes(), availability.CanonicalBytes()) {
			return nil, fmt.Errorf(
				"base and extension availability grounds conflict at %s/%s",
				availability.Context().String(),
				availability.KindID().String(),
			)
		}
	}
	result := make([]typedmemory.ContextKindAvailability, 0, len(byCoordinate))
	for _, availability := range byCoordinate {
		result = append(result, availability)
	}
	sort.Slice(result, func(left, right int) bool {
		leftKey := result[left].Context().String() + "\x00" + result[left].KindID().String()
		rightKey := result[right].Context().String() + "\x00" + result[right].KindID().String()
		return leftKey < rightKey
	})
	return result, nil
}
