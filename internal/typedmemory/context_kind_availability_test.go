package typedmemory

import (
	"bytes"
	"testing"
)

type contextKindAvailabilityFixture struct {
	context        BoundedContextRef
	kind           KindID
	base           TypeEnvRef
	local          LocalContextKindAvailabilityGround
	directBase     DirectContextKindAvailabilityGround
	directImported DirectContextKindAvailabilityGround
	bridged        BridgedContextKindAvailabilityGround
	grounds        ContextKindAvailabilityGroundSet
}

func TestContextKindAvailabilityRetainsEveryCanonicalGround(t *testing.T) {
	fixture := newContextKindAvailabilityFixture(t)
	availability := mustTypedMemoryValue(NewContextKindAvailability(
		fixture.context,
		fixture.kind,
		fixture.grounds,
	))

	grounds := availability.Grounds()
	if len(grounds) != 4 {
		t.Fatalf("ground count = %d, want 4", len(grounds))
	}
	seenLocal := false
	seenBase := false
	seenImported := false
	seenBridge := false
	for _, ground := range grounds {
		if ground.Context() != fixture.context || ground.KindID() != fixture.kind {
			t.Fatal("ground lost the aggregate context-kind coordinate")
		}
		switch value := ground.(type) {
		case LocalContextKindAvailabilityGround:
			seenLocal = value.Provider().ExtensionRef().valid()
		case DirectContextKindAvailabilityGround:
			switch value.Scope() {
			case BaseContextKindUse:
				provider, ok := value.Provider().(BaseKindAvailabilityProvider)
				seenBase = ok && provider.BaseTypeEnvRef() == fixture.base
			case ImportedExtensionContextKindUse:
				provider, ok := value.Provider().(ExtensionKindAvailabilityProvider)
				seenImported = ok && provider.ExtensionRef().valid()
			}
		case BridgedContextKindAvailabilityGround:
			basis, ok := value.BridgeBasis().(BaseKindAvailabilityBridgeBasis)
			mapping := value.Bridge().Mapping()
			seenBridge = ok &&
				basis.BaseTypeEnvRef() == fixture.base &&
				mapping.TargetKind() == fixture.kind
		}
	}
	if !seenLocal || !seenBase || !seenImported || !seenBridge {
		t.Fatalf(
			"retained grounds local=%t base=%t imported=%t bridged=%t",
			seenLocal,
			seenBase,
			seenImported,
			seenBridge,
		)
	}
}

func TestContextKindAvailabilityGroundSetIsPermutationInvariantAndRejectsLoss(t *testing.T) {
	fixture := newContextKindAvailabilityFixture(t)
	permuted := mustTypedMemoryValue(NewContextKindAvailabilityGroundSet(
		[]ContextKindAvailabilityGround{
			fixture.bridged,
			fixture.directImported,
			fixture.local,
			fixture.directBase,
		},
	))
	if !bytes.Equal(fixture.grounds.CanonicalBytes(), permuted.CanonicalBytes()) {
		t.Fatal("ground permutation changed canonical aggregate identity")
	}

	if _, err := NewContextKindAvailabilityGroundSet(nil); err == nil {
		t.Fatal("empty ground set was accepted")
	}
	if _, err := NewContextKindAvailabilityGroundSet(
		[]ContextKindAvailabilityGround{fixture.local, fixture.local},
	); err == nil {
		t.Fatal("duplicate ground was accepted")
	}

	typeEnvFixture := newTypeEnvFixture(t)
	other := typeEnvTestKindAvailability(
		typeEnvFixture.secondaryContext.Ref(),
		typeEnvFixture.systemKind.ID(),
		typeEnvFixture.provenance,
	).Grounds()[0]
	if _, err := NewContextKindAvailabilityGroundSet(
		[]ContextKindAvailabilityGround{fixture.local, other},
	); err == nil {
		t.Fatal("mixed context-kind coordinates were accepted")
	}
}

func TestContextKindAvailabilityRejectsCanonicalDrift(t *testing.T) {
	fixture := newContextKindAvailabilityFixture(t)

	canonicalDrift := cloneContextKindAvailabilityGroundSet(fixture.grounds)
	canonicalDrift.canonical[0] ^= 0xff
	if canonicalDrift.valid() {
		t.Fatal("ground set accepted stored canonical drift")
	}
	if _, err := NewContextKindAvailability(
		fixture.context,
		fixture.kind,
		canonicalDrift,
	); err == nil {
		t.Fatal("availability accepted a drifted ground set")
	}

	orderingDrift := cloneContextKindAvailabilityGroundSet(fixture.grounds)
	orderingDrift.grounds[0], orderingDrift.grounds[1] =
		orderingDrift.grounds[1], orderingDrift.grounds[0]
	if orderingDrift.valid() {
		t.Fatal("ground set accepted noncanonical ground order")
	}

	groundDrift := cloneContextKindAvailabilityGroundSet(fixture.grounds)
	localIndex := contextKindAvailabilityGroundIndex(
		groundDrift.grounds,
		LocalContextKindAvailabilityGroundKind,
	)
	local := groundDrift.grounds[localIndex].(LocalContextKindAvailabilityGround)
	local.canonical[0] ^= 0xff
	groundDrift.grounds[localIndex] = local
	if groundDrift.valid() {
		t.Fatal("ground set accepted a drifted member ground")
	}
}

func TestContextKindAvailabilityCoordinatesChangeCanonicalIdentity(t *testing.T) {
	fixture := newContextKindAvailabilityFixture(t)
	baseProvider := fixture.directBase.Provider().(BaseKindAvailabilityProvider)
	otherBase := typeEnvTestTypeEnvRef(t, 0xe2)
	otherProvider := mustTypedMemoryValue(NewBaseKindAvailabilityProvider(
		otherBase,
		baseProvider.Symbol(),
		baseProvider.DeclarationSource(),
	))
	if bytes.Equal(baseProvider.CanonicalBytes(), otherProvider.CanonicalBytes()) {
		t.Fatal("provider TypeEnv coordinate did not affect canonical identity")
	}

	bridge := fixture.bridged.Bridge()
	bridgeBasis := fixture.bridged.BridgeBasis()
	if !bytes.Contains(bridgeBasis.CanonicalBytes(), bridge.CanonicalBytes()) {
		t.Fatal("bridge basis did not authenticate the full ContextBridge contract")
	}
	otherBasis := mustTypedMemoryValue(NewBaseKindAvailabilityBridgeBasis(otherBase, bridge))
	if bytes.Equal(
		bridgeBasis.CanonicalBytes(),
		otherBasis.CanonicalBytes(),
	) {
		t.Fatal("bridge owner coordinate did not affect canonical identity")
	}

	symbol := mustTypedMemoryValue(KindSymbolRef(fixture.kind))
	first, _ := contextKindAvailabilityProjectSource(
		t,
		"availability.source-lines",
		fixture.context,
		symbol,
		21,
	)
	second, _ := contextKindAvailabilityProjectSource(
		t,
		"availability.source-lines",
		fixture.context,
		symbol,
		22,
	)
	firstSource := mustTypedMemoryValue(NewContextKindAvailabilitySource(
		fixture.kind.String(),
		first,
	))
	secondSource := mustTypedMemoryValue(NewContextKindAvailabilitySource(
		fixture.kind.String(),
		second,
	))
	if bytes.Equal(firstSource.CanonicalBytes(), secondSource.CanonicalBytes()) {
		t.Fatal("exact source line coordinate did not affect canonical identity")
	}
}

func TestContextKindAvailabilityDefensivelyCopiesAggregateState(t *testing.T) {
	fixture := newContextKindAvailabilityFixture(t)
	original := fixture.grounds.CanonicalBytes()

	exposedCanonical := fixture.grounds.CanonicalBytes()
	exposedCanonical[0] ^= 0xff
	if !bytes.Equal(original, fixture.grounds.CanonicalBytes()) {
		t.Fatal("CanonicalBytes exposed mutable ground-set storage")
	}

	exposedGrounds := fixture.grounds.Grounds()
	localIndex := contextKindAvailabilityGroundIndex(
		exposedGrounds,
		LocalContextKindAvailabilityGroundKind,
	)
	local := exposedGrounds[localIndex].(LocalContextKindAvailabilityGround)
	local.canonical[0] ^= 0xff
	exposedGrounds[localIndex] = local
	if !fixture.grounds.valid() || !bytes.Equal(original, fixture.grounds.CanonicalBytes()) {
		t.Fatal("Grounds exposed mutable aggregate storage")
	}

	typeEnvFixture := newTypeEnvFixture(t)
	environment := typeEnvFixture.build(t)
	before := environment.ContextKindAvailabilities()[0].CanonicalBytes()
	exposed := environment.ContextKindAvailabilities()
	exposed[0].grounds.canonical[0] ^= 0xff
	after := environment.ContextKindAvailabilities()[0].CanonicalBytes()
	if !bytes.Equal(before, after) {
		t.Fatal("TypeEnv ContextKindAvailabilities exposed mutable aggregate storage")
	}

	lookedUp, found := environment.ContextKindAvailability(
		exposed[0].Context(),
		exposed[0].KindID(),
	)
	if !found {
		t.Fatal("ContextKindAvailability lookup lost the stored coordinate")
	}
	lookedUp.grounds.canonical[0] ^= 0xff
	lookedUpAgain, _ := environment.ContextKindAvailability(
		exposed[0].Context(),
		exposed[0].KindID(),
	)
	if !bytes.Equal(before, lookedUpAgain.CanonicalBytes()) {
		t.Fatal("ContextKindAvailability lookup exposed mutable aggregate storage")
	}

	firstInput := typeEnvTestSourceLocation(t, 0xd1)
	secondInput := typeEnvTestSourceLocation(t, 0xd2)
	derived := mustTypedMemoryValue(NewCompilerDerivedProvenance(
		typeEnvTestProvenanceRef(t, "prov:availability:defensive-copy"),
		[]SourceLocation{firstInput, secondInput},
		typeEnvTestCompilerRuleID(t, "availability.defensive-copy.v1"),
	))
	source := mustTypedMemoryValue(NewContextKindAvailabilitySource(
		fixture.kind.String(),
		derived,
	))
	sourceCanonical := source.CanonicalBytes()
	derived.inputs[0] = typeEnvTestSourceLocation(t, 0xd3)
	exposedDerived := source.Provenance().(CompilerDerivedProvenance)
	exposedDerived.inputs[0] = typeEnvTestSourceLocation(t, 0xd4)
	if !source.valid() || !bytes.Equal(sourceCanonical, source.CanonicalBytes()) {
		t.Fatal("availability source shared compiler-derived provenance storage")
	}
}

func TestContextKindAvailabilityProviderAndBridgeBasisAreClosed(t *testing.T) {
	fixture := newContextKindAvailabilityFixture(t)
	projectSource := fixture.local.Provider().DeclarationSource()
	baseSymbol := fixture.directBase.Provider().Symbol()
	if _, err := NewBaseKindAvailabilityProvider(
		fixture.base,
		baseSymbol,
		projectSource,
	); err == nil {
		t.Fatal("base provider accepted project-extension provenance")
	}

	fpfSource := fixture.directBase.Provider().DeclarationSource()
	localProvider := fixture.local.Provider()
	if _, err := NewExtensionKindAvailabilityProvider(
		ExtensionKindAvailabilityProviderInput{
			ExtensionRef:      localProvider.ExtensionRef(),
			Context:           localProvider.Context(),
			ContextSource:     localProvider.ContextSource(),
			Symbol:            localProvider.Symbol(),
			DeclarationSource: fpfSource,
		},
	); err == nil {
		t.Fatal("extension provider accepted base FPF provenance")
	}

	bridge := fixture.bridged.Bridge()
	if _, err := NewExtensionKindAvailabilityBridgeBasis(
		fixture.local.Provider().ExtensionRef(),
		bridge,
	); err == nil {
		t.Fatal("extension bridge basis accepted an FPF-owned bridge")
	}

	bridgeSymbol := mustTypedMemoryValue(ContextBridgeSymbolRef(bridge.ID()))
	projectProvenance, extension := contextKindAvailabilityProjectSource(
		t,
		"availability.bridge-owner",
		fixture.context,
		bridgeSymbol,
		50,
	)
	projectBridgeInput := contextKindAvailabilityBridgeInput(bridge)
	projectBridgeInput.Provenance = projectProvenance
	projectBridge := mustTypedMemoryValue(NewContextBridge(projectBridgeInput))
	if _, err := NewExtensionKindAvailabilityBridgeBasis(extension, projectBridge); err != nil {
		t.Fatalf("valid extension-owned bridge basis rejected: %v", err)
	}
}

func TestBridgedContextKindAvailabilityHonorsNonIdentityDirection(t *testing.T) {
	fixture := newContextKindAvailabilityFixture(t)
	provider := fixture.bridged.Provider()
	consumerContext := fixture.context
	providerContext := provider.Context()
	consumerKind := fixture.kind
	providerKind := provider.KindID()
	baseFixture := newTypeEnvFixture(t)
	twoWay := typeEnvTestContextBridge(
		t,
		typeEnvTestBridgeID(t, "Haft.Bridge.ConsumerToProvider"),
		consumerContext,
		providerContext,
		consumerKind,
		providerKind,
		TwoWayBridge,
		baseFixture.provenance,
	)
	twoWayBasis := mustTypedMemoryValue(NewBaseKindAvailabilityBridgeBasis(
		fixture.base,
		twoWay,
	))
	input := BridgedContextKindAvailabilityGroundInput{
		ConsumerExtension:   fixture.bridged.ConsumerExtension(),
		Context:             consumerContext,
		KindID:              consumerKind,
		ContextSource:       fixture.bridged.ContextSource(),
		ApplicabilitySource: fixture.bridged.ApplicabilitySource(),
		UseSource:           fixture.bridged.EvidenceSource(),
		Origin:              fixture.bridged.Origin(),
		Role:                fixture.bridged.Role(),
		Provider:            provider,
		BridgeBasis:         twoWayBasis,
	}
	if _, err := NewBridgedContextKindAvailabilityGround(input); err != nil {
		t.Fatalf("two-way reverse nonidentity bridge rejected: %v", err)
	}

	oneWayInput := contextKindAvailabilityBridgeInput(twoWay)
	oneWayInput.Direction = OneWayBridge
	oneWay := mustTypedMemoryValue(NewContextBridge(oneWayInput))
	input.BridgeBasis = mustTypedMemoryValue(NewBaseKindAvailabilityBridgeBasis(
		fixture.base,
		oneWay,
	))
	if _, err := NewBridgedContextKindAvailabilityGround(input); err == nil {
		t.Fatal("one-way bridge was incorrectly traversed in reverse")
	}
}

func contextKindAvailabilityGroundIndex(
	grounds []ContextKindAvailabilityGround,
	kind ContextKindAvailabilityGroundKind,
) int {
	for index, ground := range grounds {
		if ground.GroundKind() == kind {
			return index
		}
	}
	panic("context-kind availability test ground is missing")
}

func newContextKindAvailabilityFixture(t *testing.T) contextKindAvailabilityFixture {
	t.Helper()
	baseFixture := newTypeEnvFixture(t)
	consumerContext := typeEnvTestContextRef(t, "ctx:consumer")
	providerContext := typeEnvTestContextRef(t, "ctx:provider")
	consumerKind := typeEnvTestKindID(t, "Haft.ConsumerConcern")
	providerKind := typeEnvTestKindID(t, "Haft.ProviderConcern")
	consumerSymbol := mustTypedMemoryValue(KindSymbolRef(consumerKind))

	consumerProvenance, consumerRef := contextKindAvailabilityProjectSource(
		t,
		"availability.consumer",
		consumerContext,
		consumerSymbol,
		10,
	)
	consumerContextSource := mustTypedMemoryValue(NewContextKindAvailabilitySource(
		consumerContext.String(),
		consumerProvenance,
	))
	consumerKindSource := mustTypedMemoryValue(NewContextKindAvailabilitySource(
		consumerKind.String(),
		consumerProvenance,
	))

	localProvider := contextKindAvailabilityExtensionProvider(
		t,
		"availability.local",
		consumerContext,
		consumerKind,
		20,
	)
	local := mustTypedMemoryValue(NewLocalContextKindAvailabilityGround(
		LocalContextKindAvailabilityGroundInput{
			Context:             consumerContext,
			KindID:              consumerKind,
			ContextSource:       localProvider.ContextSource(),
			ApplicabilitySource: localProvider.ContextSource(),
			Provider:            localProvider,
		},
	))

	baseDeclaration := mustTypedMemoryValue(NewContextKindAvailabilitySource(
		consumerKind.String(),
		baseFixture.provenance,
	))
	baseProvider := mustTypedMemoryValue(NewBaseKindAvailabilityProvider(
		baseFixture.ref,
		consumerSymbol,
		baseDeclaration,
	))
	directBase := mustTypedMemoryValue(NewDirectContextKindAvailabilityGround(
		DirectContextKindAvailabilityGroundInput{
			ConsumerExtension:   consumerRef,
			Context:             consumerContext,
			KindID:              consumerKind,
			ContextSource:       consumerContextSource,
			ApplicabilitySource: consumerContextSource,
			UseSource:           consumerKindSource,
			Origin:              "signature.subject",
			Role:                "ranged_value_kind",
			Scope:               BaseContextKindUse,
			Provider:            baseProvider,
		},
	))

	importedProvider := contextKindAvailabilityExtensionProvider(
		t,
		"availability.imported",
		consumerContext,
		consumerKind,
		30,
	)
	directImported := mustTypedMemoryValue(NewDirectContextKindAvailabilityGround(
		DirectContextKindAvailabilityGroundInput{
			ConsumerExtension:   consumerRef,
			Context:             consumerContext,
			KindID:              consumerKind,
			ContextSource:       consumerContextSource,
			ApplicabilitySource: consumerContextSource,
			UseSource:           consumerKindSource,
			Origin:              "signature.vocabulary",
			Role:                "value_kind",
			Scope:               ImportedExtensionContextKindUse,
			Provider:            importedProvider,
		},
	))

	bridgedProvider := contextKindAvailabilityExtensionProvider(
		t,
		"availability.provider",
		providerContext,
		providerKind,
		40,
	)
	providerUseSource := mustTypedMemoryValue(NewContextKindAvailabilitySource(
		providerKind.String(),
		consumerProvenance,
	))
	bridge := typeEnvTestContextBridge(
		t,
		typeEnvTestBridgeID(t, "Haft.Bridge.ProviderToConsumer"),
		providerContext,
		consumerContext,
		providerKind,
		consumerKind,
		OneWayBridge,
		baseFixture.provenance,
	)
	bridgeBasis := mustTypedMemoryValue(NewBaseKindAvailabilityBridgeBasis(
		baseFixture.ref,
		bridge,
	))
	bridged := mustTypedMemoryValue(NewBridgedContextKindAvailabilityGround(
		BridgedContextKindAvailabilityGroundInput{
			ConsumerExtension:   consumerRef,
			Context:             consumerContext,
			KindID:              consumerKind,
			ContextSource:       consumerContextSource,
			ApplicabilitySource: consumerContextSource,
			UseSource:           providerUseSource,
			Origin:              "signature.vocabulary",
			Role:                "value_kind",
			Provider:            bridgedProvider,
			BridgeBasis:         bridgeBasis,
		},
	))

	grounds := mustTypedMemoryValue(NewContextKindAvailabilityGroundSet(
		[]ContextKindAvailabilityGround{local, directBase, directImported, bridged},
	))
	return contextKindAvailabilityFixture{
		context:        consumerContext,
		kind:           consumerKind,
		base:           baseFixture.ref,
		local:          local,
		directBase:     directBase,
		directImported: directImported,
		bridged:        bridged,
		grounds:        grounds,
	}
}

func contextKindAvailabilityExtensionProvider(
	t *testing.T,
	manifestID string,
	context BoundedContextRef,
	kind KindID,
	line uint64,
) ExtensionKindAvailabilityProvider {
	t.Helper()
	symbol := mustTypedMemoryValue(KindSymbolRef(kind))
	provenance, extension := contextKindAvailabilityProjectSource(
		t,
		manifestID,
		context,
		symbol,
		line,
	)
	contextSource := mustTypedMemoryValue(NewContextKindAvailabilitySource(
		context.String(),
		provenance,
	))
	declarationSource := mustTypedMemoryValue(NewContextKindAvailabilitySource(
		kind.String(),
		provenance,
	))
	return mustTypedMemoryValue(NewExtensionKindAvailabilityProvider(
		ExtensionKindAvailabilityProviderInput{
			ExtensionRef:      extension,
			Context:           context,
			ContextSource:     contextSource,
			Symbol:            symbol,
			DeclarationSource: declarationSource,
		},
	))
}

func contextKindAvailabilityProjectSource(
	t *testing.T,
	manifestID string,
	context BoundedContextRef,
	symbol SchemaSymbolRef,
	line uint64,
) (ProjectSourceProvenance, TypeEnvExtensionRef) {
	t.Helper()
	writer := newCanonicalWriter("test.context-kind-availability-project-source.v1")
	writer.addString(manifestID)
	writer.addString(context.String())
	writer.addString(symbol.String())
	digest := writer.digest()
	manifest := mustTypedMemoryValue(NewSignatureManifestRef(manifestID, "1.0.0"))
	basis := mustTypedMemoryValue(NewManifestSymbolBasis(
		manifest,
		ManifestProvide,
		symbol,
	))
	provenance := mustTypedMemoryValue(
		NewProjectSourceProvenanceBuilder(
			mustTypedMemoryValue(NewProvenanceRef(manifestID+"#"+symbol.Key())),
			mustTypedMemoryValue(NewCarrierRef("carrier:"+manifestID)),
			mustTypedMemoryValue(NewCarrierEdition("1.0.0")),
			digest,
		).
			SetDeclarationRange(mustTypedMemoryValue(NewSourceLineRange(line, line))).
			SetCompilerRule(mustTypedMemoryValue(NewCompilerRuleID("test.context-kind-availability.v1"))).
			SetBoundedContext(context).
			SetBaseTypeEnv(mustTypedMemoryValue(NewTypeEnvRef(digest))).
			SetSignatureBlockRow(VocabularyRow).
			SetManifestBasis(basis).
			Build(),
	)
	extensionID := mustTypedMemoryValue(NewExtensionID(manifest.ID()))
	extension := mustTypedMemoryValue(newTypeEnvExtensionRef(extensionID, digest))
	return provenance, extension
}

func contextKindAvailabilityBridgeInput(bridge ContextBridge) ContextBridgeInput {
	return ContextBridgeInput{
		ID:              bridge.ID(),
		Source:          bridge.Source(),
		Target:          bridge.Target(),
		Mapping:         bridge.Mapping(),
		Direction:       bridge.Direction(),
		OrderCoverage:   bridge.OrderCoverage(),
		KindCongruence:  bridge.KindCongruence(),
		LossNotes:       bridge.LossNotes(),
		DefinednessArea: bridge.DefinednessArea(),
		Provenance:      bridge.Provenance(),
	}
}
