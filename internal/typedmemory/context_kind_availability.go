package typedmemory

import (
	"bytes"
	"fmt"
	"sort"
	"unicode/utf8"
)

const (
	maximumContextKindAvailabilityGrounds = 1 << 12
	maximumContextKindGroundSetBytes      = 16 << 20
)

// ContextKindAvailabilitySource preserves one exact source scalar together
// with the declaration provenance that locates it. The value is retained
// because several distinct facts can share one carrier row and provenance.
type ContextKindAvailabilitySource struct {
	value      string
	provenance DeclarationProvenance
	canonical  []byte
}

func NewContextKindAvailabilitySource(
	value string,
	provenance DeclarationProvenance,
) (ContextKindAvailabilitySource, error) {
	parsed, err := parseOpaqueIdentifier("context-kind availability source value", value)
	if err != nil {
		return ContextKindAvailabilitySource{}, err
	}
	if !utf8.ValidString(value) || parsed != value {
		return ContextKindAvailabilitySource{}, fmt.Errorf(
			"context-kind availability source value must be exact canonical UTF-8",
		)
	}
	if !validDeclarationProvenance(provenance) {
		return ContextKindAvailabilitySource{}, fmt.Errorf(
			"context-kind availability source provenance is required",
		)
	}
	source := ContextKindAvailabilitySource{
		value:      value,
		provenance: cloneDeclarationProvenance(provenance),
	}
	source.canonical = canonicalContextKindAvailabilitySource(source)
	return cloneContextKindAvailabilitySource(source), nil
}

func (source ContextKindAvailabilitySource) Value() string { return source.value }

func (source ContextKindAvailabilitySource) Provenance() DeclarationProvenance {
	return cloneDeclarationProvenance(source.provenance)
}

func (source ContextKindAvailabilitySource) CanonicalBytes() []byte {
	return append([]byte(nil), source.canonical...)
}

func (source ContextKindAvailabilitySource) valid() bool {
	parsed, err := parseOpaqueIdentifier("context-kind availability source value", source.value)
	if err != nil || parsed != source.value || !utf8.ValidString(source.value) {
		return false
	}
	if !validDeclarationProvenance(source.provenance) {
		return false
	}
	expected := canonicalContextKindAvailabilitySource(source)
	return bytes.Equal(source.canonical, expected)
}

func canonicalContextKindAvailabilitySource(
	source ContextKindAvailabilitySource,
) []byte {
	writer := newCanonicalWriter("context-kind-availability-source.v1")
	writer.addString(source.value)
	writer.addBytes(source.provenance.CanonicalBytes())
	return writer.bytes()
}

func cloneContextKindAvailabilitySource(
	source ContextKindAvailabilitySource,
) ContextKindAvailabilitySource {
	source.provenance = cloneDeclarationProvenance(source.provenance)
	source.canonical = append([]byte(nil), source.canonical...)
	return source
}

type ContextKindAvailabilityProviderKind uint8

const (
	BaseContextKindAvailabilityProvider ContextKindAvailabilityProviderKind = iota + 1
	ExtensionContextKindAvailabilityProvider
)

func (kind ContextKindAvailabilityProviderKind) String() string {
	switch kind {
	case BaseContextKindAvailabilityProvider:
		return "base"
	case ExtensionContextKindAvailabilityProvider:
		return "extension"
	default:
		return ""
	}
}

// ContextKindAvailabilityProvider is a closed provider-coordinate union. A
// provider identifies where a kind symbol came from; it does not establish
// availability, membership, durable U-kind admission, or project authority.
type ContextKindAvailabilityProvider interface {
	ProviderKind() ContextKindAvailabilityProviderKind
	Symbol() SchemaSymbolRef
	KindID() KindID
	DeclarationSource() ContextKindAvailabilitySource
	CanonicalBytes() []byte
	contextKindAvailabilityProviderVariant()
}

type BaseKindAvailabilityProvider struct {
	base        TypeEnvRef
	symbol      SchemaSymbolRef
	kindID      KindID
	declaration ContextKindAvailabilitySource
	canonical   []byte
}

func NewBaseKindAvailabilityProvider(
	base TypeEnvRef,
	symbol SchemaSymbolRef,
	declaration ContextKindAvailabilitySource,
) (BaseKindAvailabilityProvider, error) {
	kindID, err := contextKindAvailabilityProviderKindID(symbol)
	if err != nil {
		return BaseKindAvailabilityProvider{}, err
	}
	if !base.valid() {
		return BaseKindAvailabilityProvider{}, fmt.Errorf("base provider TypeEnvRef is required")
	}
	if !declaration.valid() || declaration.Value() != symbol.Key() {
		return BaseKindAvailabilityProvider{}, fmt.Errorf(
			"base provider declaration must locate the exact kind symbol",
		)
	}
	if !baseAvailabilityProvenance(declaration.Provenance()) {
		return BaseKindAvailabilityProvider{}, fmt.Errorf(
			"base provider declaration must come from FPF source or compiler-derived FPF inputs",
		)
	}
	provider := BaseKindAvailabilityProvider{
		base:        base,
		symbol:      symbol,
		kindID:      kindID,
		declaration: cloneContextKindAvailabilitySource(declaration),
	}
	provider.canonical = canonicalBaseKindAvailabilityProvider(provider)
	return cloneBaseKindAvailabilityProvider(provider), nil
}

func (BaseKindAvailabilityProvider) ProviderKind() ContextKindAvailabilityProviderKind {
	return BaseContextKindAvailabilityProvider
}

func (provider BaseKindAvailabilityProvider) BaseTypeEnvRef() TypeEnvRef { return provider.base }

func (provider BaseKindAvailabilityProvider) Symbol() SchemaSymbolRef { return provider.symbol }

func (provider BaseKindAvailabilityProvider) KindID() KindID { return provider.kindID }

func (provider BaseKindAvailabilityProvider) DeclarationSource() ContextKindAvailabilitySource {
	return cloneContextKindAvailabilitySource(provider.declaration)
}

func (provider BaseKindAvailabilityProvider) CanonicalBytes() []byte {
	return append([]byte(nil), provider.canonical...)
}

func (BaseKindAvailabilityProvider) contextKindAvailabilityProviderVariant() {}

func (provider BaseKindAvailabilityProvider) valid() bool {
	if !provider.base.valid() || !provider.symbol.valid() || !provider.kindID.valid() {
		return false
	}
	if provider.symbol.Kind() != KindSymbol || provider.symbol.Key() != provider.kindID.String() {
		return false
	}
	if !provider.declaration.valid() || provider.declaration.Value() != provider.symbol.Key() {
		return false
	}
	if !baseAvailabilityProvenance(provider.declaration.Provenance()) {
		return false
	}
	expected := canonicalBaseKindAvailabilityProvider(provider)
	return bytes.Equal(provider.canonical, expected)
}

func canonicalBaseKindAvailabilityProvider(provider BaseKindAvailabilityProvider) []byte {
	writer := newCanonicalWriter("context-kind-availability-provider.base.v1")
	writer.addString(provider.base.String())
	writer.addString(provider.symbol.String())
	writer.addBytes(provider.declaration.canonical)
	return writer.bytes()
}

func cloneBaseKindAvailabilityProvider(
	provider BaseKindAvailabilityProvider,
) BaseKindAvailabilityProvider {
	provider.declaration = cloneContextKindAvailabilitySource(provider.declaration)
	provider.canonical = append([]byte(nil), provider.canonical...)
	return provider
}

type ExtensionKindAvailabilityProvider struct {
	extension     TypeEnvExtensionRef
	context       BoundedContextRef
	contextSource ContextKindAvailabilitySource
	symbol        SchemaSymbolRef
	kindID        KindID
	declaration   ContextKindAvailabilitySource
	canonical     []byte
}

type ExtensionKindAvailabilityProviderInput struct {
	ExtensionRef      TypeEnvExtensionRef
	Context           BoundedContextRef
	ContextSource     ContextKindAvailabilitySource
	Symbol            SchemaSymbolRef
	DeclarationSource ContextKindAvailabilitySource
}

func NewExtensionKindAvailabilityProvider(
	input ExtensionKindAvailabilityProviderInput,
) (ExtensionKindAvailabilityProvider, error) {
	kindID, err := contextKindAvailabilityProviderKindID(input.Symbol)
	if err != nil {
		return ExtensionKindAvailabilityProvider{}, err
	}
	if !input.ExtensionRef.valid() {
		return ExtensionKindAvailabilityProvider{}, fmt.Errorf(
			"extension provider exact TypeEnvExtensionRef is required",
		)
	}
	if !input.Context.valid() {
		return ExtensionKindAvailabilityProvider{}, fmt.Errorf(
			"extension provider bounded context is required",
		)
	}
	if !sourceNamesContext(input.ContextSource, input.Context) {
		return ExtensionKindAvailabilityProvider{}, fmt.Errorf(
			"extension provider context source must locate the exact bounded context",
		)
	}
	if !input.DeclarationSource.valid() || input.DeclarationSource.Value() != input.Symbol.Key() {
		return ExtensionKindAvailabilityProvider{}, fmt.Errorf(
			"extension provider declaration must locate the exact kind symbol",
		)
	}
	if err := validateExtensionKindAvailabilityProviderSources(input); err != nil {
		return ExtensionKindAvailabilityProvider{}, err
	}
	provider := ExtensionKindAvailabilityProvider{
		extension:     input.ExtensionRef,
		context:       input.Context,
		contextSource: cloneContextKindAvailabilitySource(input.ContextSource),
		symbol:        input.Symbol,
		kindID:        kindID,
		declaration:   cloneContextKindAvailabilitySource(input.DeclarationSource),
	}
	provider.canonical = canonicalExtensionKindAvailabilityProvider(provider)
	return cloneExtensionKindAvailabilityProvider(provider), nil
}

func (ExtensionKindAvailabilityProvider) ProviderKind() ContextKindAvailabilityProviderKind {
	return ExtensionContextKindAvailabilityProvider
}

func (provider ExtensionKindAvailabilityProvider) ExtensionRef() TypeEnvExtensionRef {
	return provider.extension
}

func (provider ExtensionKindAvailabilityProvider) Context() BoundedContextRef {
	return provider.context
}

func (provider ExtensionKindAvailabilityProvider) ContextSource() ContextKindAvailabilitySource {
	return cloneContextKindAvailabilitySource(provider.contextSource)
}

func (provider ExtensionKindAvailabilityProvider) Symbol() SchemaSymbolRef { return provider.symbol }

func (provider ExtensionKindAvailabilityProvider) KindID() KindID { return provider.kindID }

func (provider ExtensionKindAvailabilityProvider) DeclarationSource() ContextKindAvailabilitySource {
	return cloneContextKindAvailabilitySource(provider.declaration)
}

func (provider ExtensionKindAvailabilityProvider) CanonicalBytes() []byte {
	return append([]byte(nil), provider.canonical...)
}

func (ExtensionKindAvailabilityProvider) contextKindAvailabilityProviderVariant() {}

func (provider ExtensionKindAvailabilityProvider) valid() bool {
	if !provider.extension.valid() || !provider.context.valid() || !provider.symbol.valid() {
		return false
	}
	if !provider.kindID.valid() || provider.symbol.Kind() != KindSymbol {
		return false
	}
	if provider.symbol.Key() != provider.kindID.String() {
		return false
	}
	if !sourceNamesContext(provider.contextSource, provider.context) {
		return false
	}
	if !provider.declaration.valid() || provider.declaration.Value() != provider.symbol.Key() {
		return false
	}
	input := ExtensionKindAvailabilityProviderInput{
		ExtensionRef:      provider.extension,
		Context:           provider.context,
		ContextSource:     provider.contextSource,
		Symbol:            provider.symbol,
		DeclarationSource: provider.declaration,
	}
	if validateExtensionKindAvailabilityProviderSources(input) != nil {
		return false
	}
	expected := canonicalExtensionKindAvailabilityProvider(provider)
	return bytes.Equal(provider.canonical, expected)
}

func canonicalExtensionKindAvailabilityProvider(
	provider ExtensionKindAvailabilityProvider,
) []byte {
	writer := newCanonicalWriter("context-kind-availability-provider.extension.v1")
	writer.addString(provider.extension.String())
	writer.addString(provider.context.String())
	writer.addBytes(provider.contextSource.canonical)
	writer.addString(provider.symbol.String())
	writer.addBytes(provider.declaration.canonical)
	return writer.bytes()
}

func cloneExtensionKindAvailabilityProvider(
	provider ExtensionKindAvailabilityProvider,
) ExtensionKindAvailabilityProvider {
	provider.contextSource = cloneContextKindAvailabilitySource(provider.contextSource)
	provider.declaration = cloneContextKindAvailabilitySource(provider.declaration)
	provider.canonical = append([]byte(nil), provider.canonical...)
	return provider
}

func baseAvailabilityProvenance(provenance DeclarationProvenance) bool {
	switch provenance.(type) {
	case FPFSourceProvenance, CompilerDerivedProvenance:
		return true
	default:
		return false
	}
}

func validateExtensionKindAvailabilityProviderSources(
	input ExtensionKindAvailabilityProviderInput,
) error {
	contextProvenance, contextIsProject := projectSourceProvenance(
		input.ContextSource,
	)
	declarationProvenance, declarationIsProject := projectSourceProvenance(
		input.DeclarationSource,
	)
	if !contextIsProject || !declarationIsProject {
		return fmt.Errorf(
			"extension provider sources must retain project-source provenance",
		)
	}
	if contextProvenance.BoundedContext() != input.Context ||
		declarationProvenance.BoundedContext() != input.Context {
		return fmt.Errorf(
			"extension provider sources must name the exact bounded context",
		)
	}
	if !sameProjectSourceCarrier(contextProvenance, declarationProvenance) {
		return fmt.Errorf(
			"extension provider context and declaration must come from one exact carrier edition",
		)
	}
	if !extensionRefMatchesProjectSource(input.ExtensionRef, declarationProvenance) ||
		!extensionRefMatchesProjectSource(input.ExtensionRef, contextProvenance) {
		return fmt.Errorf(
			"extension provider reference must match the exact source manifest",
		)
	}
	basis := declarationProvenance.ManifestBasis()
	if basis.Direction() != ManifestProvide || basis.Symbol() != input.Symbol {
		return fmt.Errorf(
			"extension provider declaration must be an exact manifest provide for the kind symbol",
		)
	}
	return nil
}

func projectSourceProvenance(
	source ContextKindAvailabilitySource,
) (ProjectSourceProvenance, bool) {
	provenance, ok := source.Provenance().(ProjectSourceProvenance)
	return provenance, ok && provenance.validate() == nil
}

func extensionRefMatchesProjectSource(
	extension TypeEnvExtensionRef,
	provenance ProjectSourceProvenance,
) bool {
	return extension.valid() &&
		provenance.validate() == nil &&
		extension.ID().String() == provenance.ManifestBasis().Manifest().ID()
}

func sameProjectSourceCarrier(
	left ProjectSourceProvenance,
	right ProjectSourceProvenance,
) bool {
	return left.Carrier() == right.Carrier() &&
		left.Edition() == right.Edition() &&
		left.ContentHash() == right.ContentHash() &&
		left.BoundedContext() == right.BoundedContext() &&
		left.BaseTypeEnv() == right.BaseTypeEnv() &&
		left.ManifestBasis().Manifest() == right.ManifestBasis().Manifest()
}

func contextKindAvailabilityProviderKindID(symbol SchemaSymbolRef) (KindID, error) {
	if !symbol.valid() || symbol.Kind() != KindSymbol {
		return KindID{}, fmt.Errorf("context-kind availability provider must be a kind symbol")
	}
	kindID, err := NewKindID(symbol.Key())
	if err != nil {
		return KindID{}, fmt.Errorf("context-kind availability provider: %w", err)
	}
	return kindID, nil
}

func validContextKindAvailabilityProvider(provider ContextKindAvailabilityProvider) bool {
	switch value := provider.(type) {
	case BaseKindAvailabilityProvider:
		return value.valid()
	case ExtensionKindAvailabilityProvider:
		return value.valid()
	default:
		return false
	}
}

func cloneContextKindAvailabilityProvider(
	provider ContextKindAvailabilityProvider,
) ContextKindAvailabilityProvider {
	switch value := provider.(type) {
	case BaseKindAvailabilityProvider:
		return cloneBaseKindAvailabilityProvider(value)
	case ExtensionKindAvailabilityProvider:
		return cloneExtensionKindAvailabilityProvider(value)
	default:
		return nil
	}
}

type ContextKindAvailabilityBridgeBasisKind uint8

const (
	BaseContextKindAvailabilityBridgeBasis ContextKindAvailabilityBridgeBasisKind = iota + 1
	ExtensionContextKindAvailabilityBridgeBasis
)

func (kind ContextKindAvailabilityBridgeBasisKind) String() string {
	switch kind {
	case BaseContextKindAvailabilityBridgeBasis:
		return "base"
	case ExtensionContextKindAvailabilityBridgeBasis:
		return "extension"
	default:
		return ""
	}
}

// ContextKindAvailabilityBridgeBasis retains both the bridge declaration and
// the exact TypeEnv owner which supplied it. A naked ContextBridge is not a
// complete coordinate because the same declaration bytes can occur under a
// different base or extension identity.
type ContextKindAvailabilityBridgeBasis interface {
	BasisKind() ContextKindAvailabilityBridgeBasisKind
	Bridge() ContextBridge
	CanonicalBytes() []byte
	contextKindAvailabilityBridgeBasisVariant()
}

type BaseKindAvailabilityBridgeBasis struct {
	base      TypeEnvRef
	bridge    ContextBridge
	canonical []byte
}

func NewBaseKindAvailabilityBridgeBasis(
	base TypeEnvRef,
	bridge ContextBridge,
) (BaseKindAvailabilityBridgeBasis, error) {
	if !base.valid() {
		return BaseKindAvailabilityBridgeBasis{}, fmt.Errorf(
			"base bridge basis TypeEnvRef is required",
		)
	}
	if !bridge.valid() || !baseAvailabilityProvenance(bridge.Provenance()) {
		return BaseKindAvailabilityBridgeBasis{}, fmt.Errorf(
			"base bridge basis requires a valid FPF-derived ContextBridge",
		)
	}
	basis := BaseKindAvailabilityBridgeBasis{
		base:   base,
		bridge: cloneContextKindAvailabilityBridge(bridge),
	}
	basis.canonical = canonicalBaseKindAvailabilityBridgeBasis(basis)
	return cloneBaseKindAvailabilityBridgeBasis(basis), nil
}

func (BaseKindAvailabilityBridgeBasis) BasisKind() ContextKindAvailabilityBridgeBasisKind {
	return BaseContextKindAvailabilityBridgeBasis
}

func (basis BaseKindAvailabilityBridgeBasis) BaseTypeEnvRef() TypeEnvRef { return basis.base }

func (basis BaseKindAvailabilityBridgeBasis) Bridge() ContextBridge {
	return cloneContextKindAvailabilityBridge(basis.bridge)
}

func (basis BaseKindAvailabilityBridgeBasis) CanonicalBytes() []byte {
	return append([]byte(nil), basis.canonical...)
}

func (BaseKindAvailabilityBridgeBasis) contextKindAvailabilityBridgeBasisVariant() {}

func (basis BaseKindAvailabilityBridgeBasis) valid() bool {
	if !basis.base.valid() || !basis.bridge.valid() {
		return false
	}
	if !baseAvailabilityProvenance(basis.bridge.Provenance()) {
		return false
	}
	expected := canonicalBaseKindAvailabilityBridgeBasis(basis)
	return bytes.Equal(basis.canonical, expected)
}

func canonicalBaseKindAvailabilityBridgeBasis(
	basis BaseKindAvailabilityBridgeBasis,
) []byte {
	writer := newCanonicalWriter("context-kind-availability-bridge-basis.base.v1")
	writer.addString(basis.base.String())
	writer.addBytes(canonicalContextKindAvailabilityBridge(basis.bridge))
	return writer.bytes()
}

func cloneBaseKindAvailabilityBridgeBasis(
	basis BaseKindAvailabilityBridgeBasis,
) BaseKindAvailabilityBridgeBasis {
	basis.bridge = cloneContextKindAvailabilityBridge(basis.bridge)
	basis.canonical = append([]byte(nil), basis.canonical...)
	return basis
}

type ExtensionKindAvailabilityBridgeBasis struct {
	extension TypeEnvExtensionRef
	bridge    ContextBridge
	canonical []byte
}

func NewExtensionKindAvailabilityBridgeBasis(
	extension TypeEnvExtensionRef,
	bridge ContextBridge,
) (ExtensionKindAvailabilityBridgeBasis, error) {
	if !extension.valid() {
		return ExtensionKindAvailabilityBridgeBasis{}, fmt.Errorf(
			"extension bridge basis TypeEnvExtensionRef is required",
		)
	}
	if !bridge.valid() {
		return ExtensionKindAvailabilityBridgeBasis{}, fmt.Errorf(
			"extension bridge basis ContextBridge is required",
		)
	}
	if err := validateExtensionKindAvailabilityBridgeBasis(extension, bridge); err != nil {
		return ExtensionKindAvailabilityBridgeBasis{}, err
	}
	basis := ExtensionKindAvailabilityBridgeBasis{
		extension: extension,
		bridge:    cloneContextKindAvailabilityBridge(bridge),
	}
	basis.canonical = canonicalExtensionKindAvailabilityBridgeBasis(basis)
	return cloneExtensionKindAvailabilityBridgeBasis(basis), nil
}

func (ExtensionKindAvailabilityBridgeBasis) BasisKind() ContextKindAvailabilityBridgeBasisKind {
	return ExtensionContextKindAvailabilityBridgeBasis
}

func (basis ExtensionKindAvailabilityBridgeBasis) ExtensionRef() TypeEnvExtensionRef {
	return basis.extension
}

func (basis ExtensionKindAvailabilityBridgeBasis) Bridge() ContextBridge {
	return cloneContextKindAvailabilityBridge(basis.bridge)
}

func (basis ExtensionKindAvailabilityBridgeBasis) CanonicalBytes() []byte {
	return append([]byte(nil), basis.canonical...)
}

func (ExtensionKindAvailabilityBridgeBasis) contextKindAvailabilityBridgeBasisVariant() {}

func (basis ExtensionKindAvailabilityBridgeBasis) valid() bool {
	if !basis.extension.valid() || !basis.bridge.valid() {
		return false
	}
	if validateExtensionKindAvailabilityBridgeBasis(basis.extension, basis.bridge) != nil {
		return false
	}
	expected := canonicalExtensionKindAvailabilityBridgeBasis(basis)
	return bytes.Equal(basis.canonical, expected)
}

func validateExtensionKindAvailabilityBridgeBasis(
	extension TypeEnvExtensionRef,
	bridge ContextBridge,
) error {
	provenance, ok := bridge.Provenance().(ProjectSourceProvenance)
	if !ok || provenance.validate() != nil {
		return fmt.Errorf(
			"extension bridge basis must retain project-source provenance",
		)
	}
	if !extensionRefMatchesProjectSource(extension, provenance) {
		return fmt.Errorf(
			"extension bridge basis reference must match the exact source manifest",
		)
	}
	symbol, err := ContextBridgeSymbolRef(bridge.ID())
	if err != nil {
		return err
	}
	basis := provenance.ManifestBasis()
	if basis.Direction() != ManifestProvide || basis.Symbol() != symbol {
		return fmt.Errorf(
			"extension bridge must be an exact manifest provide for its bridge symbol",
		)
	}
	return nil
}

func canonicalExtensionKindAvailabilityBridgeBasis(
	basis ExtensionKindAvailabilityBridgeBasis,
) []byte {
	writer := newCanonicalWriter("context-kind-availability-bridge-basis.extension.v1")
	writer.addString(basis.extension.String())
	writer.addBytes(canonicalContextKindAvailabilityBridge(basis.bridge))
	return writer.bytes()
}

func cloneExtensionKindAvailabilityBridgeBasis(
	basis ExtensionKindAvailabilityBridgeBasis,
) ExtensionKindAvailabilityBridgeBasis {
	basis.bridge = cloneContextKindAvailabilityBridge(basis.bridge)
	basis.canonical = append([]byte(nil), basis.canonical...)
	return basis
}

func cloneContextKindAvailabilityBridge(bridge ContextBridge) ContextBridge {
	return cloneContextBridge(bridge)
}

func canonicalContextKindAvailabilityBridge(bridge ContextBridge) []byte {
	return bridge.CanonicalBytes()
}

func validContextKindAvailabilityBridgeBasis(
	basis ContextKindAvailabilityBridgeBasis,
) bool {
	switch value := basis.(type) {
	case BaseKindAvailabilityBridgeBasis:
		return value.valid()
	case ExtensionKindAvailabilityBridgeBasis:
		return value.valid()
	default:
		return false
	}
}

func cloneContextKindAvailabilityBridgeBasis(
	basis ContextKindAvailabilityBridgeBasis,
) ContextKindAvailabilityBridgeBasis {
	switch value := basis.(type) {
	case BaseKindAvailabilityBridgeBasis:
		return cloneBaseKindAvailabilityBridgeBasis(value)
	case ExtensionKindAvailabilityBridgeBasis:
		return cloneExtensionKindAvailabilityBridgeBasis(value)
	default:
		return nil
	}
}

type ContextKindAvailabilityGroundKind uint8

const (
	LocalContextKindAvailabilityGroundKind ContextKindAvailabilityGroundKind = iota + 1
	DirectContextKindAvailabilityGroundKind
	BridgedContextKindAvailabilityGroundKind
)

func (kind ContextKindAvailabilityGroundKind) String() string {
	switch kind {
	case LocalContextKindAvailabilityGroundKind:
		return "local_declaration"
	case DirectContextKindAvailabilityGroundKind:
		return "direct_use"
	case BridgedContextKindAvailabilityGroundKind:
		return "bridged_use"
	default:
		return ""
	}
}

type ContextKindAvailabilityGround interface {
	GroundKind() ContextKindAvailabilityGroundKind
	Context() BoundedContextRef
	KindID() KindID
	ContextSource() ContextKindAvailabilitySource
	ApplicabilitySource() ContextKindAvailabilitySource
	EvidenceSource() ContextKindAvailabilitySource
	CanonicalBytes() []byte
	contextKindAvailabilityGroundVariant()
}

type LocalContextKindAvailabilityGround struct {
	context       BoundedContextRef
	kindID        KindID
	contextSource ContextKindAvailabilitySource
	applicability ContextKindAvailabilitySource
	provider      ExtensionKindAvailabilityProvider
	canonical     []byte
}

type LocalContextKindAvailabilityGroundInput struct {
	Context             BoundedContextRef
	KindID              KindID
	ContextSource       ContextKindAvailabilitySource
	ApplicabilitySource ContextKindAvailabilitySource
	Provider            ExtensionKindAvailabilityProvider
}

func NewLocalContextKindAvailabilityGround(
	input LocalContextKindAvailabilityGroundInput,
) (LocalContextKindAvailabilityGround, error) {
	if err := validateContextKindAvailabilityCommon(
		input.Context,
		input.KindID,
		input.ContextSource,
		input.ApplicabilitySource,
	); err != nil {
		return LocalContextKindAvailabilityGround{}, err
	}
	if !input.Provider.valid() {
		return LocalContextKindAvailabilityGround{}, fmt.Errorf(
			"local availability ground provider is invalid",
		)
	}
	if input.Provider.Context() != input.Context || input.Provider.KindID() != input.KindID {
		return LocalContextKindAvailabilityGround{}, fmt.Errorf(
			"local availability ground provider must declare the exact context-kind coordinate",
		)
	}
	if !bytes.Equal(
		input.ContextSource.CanonicalBytes(),
		input.Provider.ContextSource().CanonicalBytes(),
	) {
		return LocalContextKindAvailabilityGround{}, fmt.Errorf(
			"local availability ground must use the provider's exact context source",
		)
	}
	if err := validateContextKindAvailabilityExtensionSources(
		input.Provider.ExtensionRef(),
		input.Context,
		[]ContextKindAvailabilitySource{
			input.ContextSource,
			input.ApplicabilitySource,
			input.Provider.DeclarationSource(),
		},
	); err != nil {
		return LocalContextKindAvailabilityGround{}, err
	}
	ground := LocalContextKindAvailabilityGround{
		context:       input.Context,
		kindID:        input.KindID,
		contextSource: cloneContextKindAvailabilitySource(input.ContextSource),
		applicability: cloneContextKindAvailabilitySource(input.ApplicabilitySource),
		provider:      cloneExtensionKindAvailabilityProvider(input.Provider),
	}
	ground.canonical = canonicalLocalContextKindAvailabilityGround(ground)
	return cloneLocalContextKindAvailabilityGround(ground), nil
}

func (LocalContextKindAvailabilityGround) GroundKind() ContextKindAvailabilityGroundKind {
	return LocalContextKindAvailabilityGroundKind
}

func (ground LocalContextKindAvailabilityGround) Context() BoundedContextRef {
	return ground.context
}

func (ground LocalContextKindAvailabilityGround) KindID() KindID { return ground.kindID }

func (ground LocalContextKindAvailabilityGround) ContextSource() ContextKindAvailabilitySource {
	return cloneContextKindAvailabilitySource(ground.contextSource)
}

func (ground LocalContextKindAvailabilityGround) ApplicabilitySource() ContextKindAvailabilitySource {
	return cloneContextKindAvailabilitySource(ground.applicability)
}

func (ground LocalContextKindAvailabilityGround) EvidenceSource() ContextKindAvailabilitySource {
	return ground.provider.DeclarationSource()
}

func (ground LocalContextKindAvailabilityGround) Provider() ExtensionKindAvailabilityProvider {
	return cloneExtensionKindAvailabilityProvider(ground.provider)
}

func (ground LocalContextKindAvailabilityGround) CanonicalBytes() []byte {
	return append([]byte(nil), ground.canonical...)
}

func (LocalContextKindAvailabilityGround) contextKindAvailabilityGroundVariant() {}

func (ground LocalContextKindAvailabilityGround) valid() bool {
	if validateContextKindAvailabilityCommon(
		ground.context,
		ground.kindID,
		ground.contextSource,
		ground.applicability,
	) != nil {
		return false
	}
	if !ground.provider.valid() || ground.provider.Context() != ground.context {
		return false
	}
	if ground.provider.KindID() != ground.kindID {
		return false
	}
	if !bytes.Equal(
		ground.contextSource.CanonicalBytes(),
		ground.provider.ContextSource().CanonicalBytes(),
	) {
		return false
	}
	if validateContextKindAvailabilityExtensionSources(
		ground.provider.ExtensionRef(),
		ground.context,
		[]ContextKindAvailabilitySource{
			ground.contextSource,
			ground.applicability,
			ground.provider.DeclarationSource(),
		},
	) != nil {
		return false
	}
	expected := canonicalLocalContextKindAvailabilityGround(ground)
	return bytes.Equal(ground.canonical, expected)
}

func canonicalLocalContextKindAvailabilityGround(
	ground LocalContextKindAvailabilityGround,
) []byte {
	writer := newCanonicalWriter("context-kind-availability-ground.local.v1")
	writeContextKindAvailabilityCommon(
		&writer,
		ground.context,
		ground.kindID,
		ground.contextSource,
		ground.applicability,
	)
	writer.addBytes(ground.provider.canonical)
	return writer.bytes()
}

func cloneLocalContextKindAvailabilityGround(
	ground LocalContextKindAvailabilityGround,
) LocalContextKindAvailabilityGround {
	ground.contextSource = cloneContextKindAvailabilitySource(ground.contextSource)
	ground.applicability = cloneContextKindAvailabilitySource(ground.applicability)
	ground.provider = cloneExtensionKindAvailabilityProvider(ground.provider)
	ground.canonical = append([]byte(nil), ground.canonical...)
	return ground
}

type ContextKindUseScope uint8

const (
	BaseContextKindUse ContextKindUseScope = iota + 1
	OwnExtensionContextKindUse
	ImportedExtensionContextKindUse
)

func (scope ContextKindUseScope) String() string {
	switch scope {
	case BaseContextKindUse:
		return "base"
	case OwnExtensionContextKindUse:
		return "own_extension"
	case ImportedExtensionContextKindUse:
		return "imported_extension"
	default:
		return ""
	}
}

func (scope ContextKindUseScope) valid() bool { return scope.String() != "" }

type DirectContextKindAvailabilityGround struct {
	consumer      TypeEnvExtensionRef
	context       BoundedContextRef
	kindID        KindID
	contextSource ContextKindAvailabilitySource
	applicability ContextKindAvailabilitySource
	useSource     ContextKindAvailabilitySource
	origin        string
	role          string
	scope         ContextKindUseScope
	provider      ContextKindAvailabilityProvider
	canonical     []byte
}

type DirectContextKindAvailabilityGroundInput struct {
	ConsumerExtension   TypeEnvExtensionRef
	Context             BoundedContextRef
	KindID              KindID
	ContextSource       ContextKindAvailabilitySource
	ApplicabilitySource ContextKindAvailabilitySource
	UseSource           ContextKindAvailabilitySource
	Origin              string
	Role                string
	Scope               ContextKindUseScope
	Provider            ContextKindAvailabilityProvider
}

func NewDirectContextKindAvailabilityGround(
	input DirectContextKindAvailabilityGroundInput,
) (DirectContextKindAvailabilityGround, error) {
	if err := validateContextKindAvailabilityCommon(
		input.Context,
		input.KindID,
		input.ContextSource,
		input.ApplicabilitySource,
	); err != nil {
		return DirectContextKindAvailabilityGround{}, err
	}
	if !input.ConsumerExtension.valid() {
		return DirectContextKindAvailabilityGround{}, fmt.Errorf(
			"direct availability ground consumer extension is required",
		)
	}
	if !input.UseSource.valid() {
		return DirectContextKindAvailabilityGround{}, fmt.Errorf(
			"direct availability ground use source is required",
		)
	}
	origin, err := exactContextKindAvailabilityLabel("use origin", input.Origin)
	if err != nil {
		return DirectContextKindAvailabilityGround{}, err
	}
	role, err := exactContextKindAvailabilityLabel("use role", input.Role)
	if err != nil {
		return DirectContextKindAvailabilityGround{}, err
	}
	if !input.Scope.valid() || !validContextKindAvailabilityProvider(input.Provider) {
		return DirectContextKindAvailabilityGround{}, fmt.Errorf(
			"direct availability ground scope and provider are required",
		)
	}
	if input.Provider.KindID() != input.KindID || input.UseSource.Value() != input.Provider.Symbol().Key() {
		return DirectContextKindAvailabilityGround{}, fmt.Errorf(
			"direct availability ground must use the exact provider kind",
		)
	}
	if err := validateDirectContextKindAvailabilityProvider(input); err != nil {
		return DirectContextKindAvailabilityGround{}, err
	}
	if err := validateContextKindAvailabilityExtensionSources(
		input.ConsumerExtension,
		input.Context,
		[]ContextKindAvailabilitySource{
			input.ContextSource,
			input.ApplicabilitySource,
			input.UseSource,
		},
	); err != nil {
		return DirectContextKindAvailabilityGround{}, err
	}
	ground := DirectContextKindAvailabilityGround{
		consumer:      input.ConsumerExtension,
		context:       input.Context,
		kindID:        input.KindID,
		contextSource: cloneContextKindAvailabilitySource(input.ContextSource),
		applicability: cloneContextKindAvailabilitySource(input.ApplicabilitySource),
		useSource:     cloneContextKindAvailabilitySource(input.UseSource),
		origin:        origin,
		role:          role,
		scope:         input.Scope,
		provider:      cloneContextKindAvailabilityProvider(input.Provider),
	}
	ground.canonical = canonicalDirectContextKindAvailabilityGround(ground)
	return cloneDirectContextKindAvailabilityGround(ground), nil
}

func (DirectContextKindAvailabilityGround) GroundKind() ContextKindAvailabilityGroundKind {
	return DirectContextKindAvailabilityGroundKind
}

func (ground DirectContextKindAvailabilityGround) Context() BoundedContextRef {
	return ground.context
}

func (ground DirectContextKindAvailabilityGround) KindID() KindID { return ground.kindID }

func (ground DirectContextKindAvailabilityGround) ConsumerExtension() TypeEnvExtensionRef {
	return ground.consumer
}

func (ground DirectContextKindAvailabilityGround) ContextSource() ContextKindAvailabilitySource {
	return cloneContextKindAvailabilitySource(ground.contextSource)
}

func (ground DirectContextKindAvailabilityGround) ApplicabilitySource() ContextKindAvailabilitySource {
	return cloneContextKindAvailabilitySource(ground.applicability)
}

func (ground DirectContextKindAvailabilityGround) EvidenceSource() ContextKindAvailabilitySource {
	return cloneContextKindAvailabilitySource(ground.useSource)
}

func (ground DirectContextKindAvailabilityGround) Origin() string { return ground.origin }

func (ground DirectContextKindAvailabilityGround) Role() string { return ground.role }

func (ground DirectContextKindAvailabilityGround) Scope() ContextKindUseScope { return ground.scope }

func (ground DirectContextKindAvailabilityGround) Provider() ContextKindAvailabilityProvider {
	return cloneContextKindAvailabilityProvider(ground.provider)
}

func (ground DirectContextKindAvailabilityGround) CanonicalBytes() []byte {
	return append([]byte(nil), ground.canonical...)
}

func (DirectContextKindAvailabilityGround) contextKindAvailabilityGroundVariant() {}

func (ground DirectContextKindAvailabilityGround) valid() bool {
	if validateContextKindAvailabilityCommon(
		ground.context,
		ground.kindID,
		ground.contextSource,
		ground.applicability,
	) != nil {
		return false
	}
	if !ground.consumer.valid() || !ground.useSource.valid() || !ground.scope.valid() {
		return false
	}
	if !validContextKindAvailabilityProvider(ground.provider) {
		return false
	}
	if ground.provider.KindID() != ground.kindID || ground.useSource.Value() != ground.provider.Symbol().Key() {
		return false
	}
	input := DirectContextKindAvailabilityGroundInput{
		ConsumerExtension: ground.consumer,
		Context:           ground.context,
		KindID:            ground.kindID,
		Scope:             ground.scope,
		Provider:          ground.provider,
	}
	if validateDirectContextKindAvailabilityProvider(input) != nil {
		return false
	}
	if validateContextKindAvailabilityExtensionSources(
		ground.consumer,
		ground.context,
		[]ContextKindAvailabilitySource{
			ground.contextSource,
			ground.applicability,
			ground.useSource,
		},
	) != nil {
		return false
	}
	if _, err := exactContextKindAvailabilityLabel("use origin", ground.origin); err != nil {
		return false
	}
	if _, err := exactContextKindAvailabilityLabel("use role", ground.role); err != nil {
		return false
	}
	expected := canonicalDirectContextKindAvailabilityGround(ground)
	return bytes.Equal(ground.canonical, expected)
}

func validateDirectContextKindAvailabilityProvider(
	input DirectContextKindAvailabilityGroundInput,
) error {
	switch provider := input.Provider.(type) {
	case BaseKindAvailabilityProvider:
		if input.Scope != BaseContextKindUse {
			return fmt.Errorf("base kind provider requires base use scope")
		}
		return nil
	case ExtensionKindAvailabilityProvider:
		if provider.Context() != input.Context {
			return fmt.Errorf("direct extension kind use must stay in one bounded context")
		}
		if input.Scope == OwnExtensionContextKindUse && provider.ExtensionRef() == input.ConsumerExtension {
			return nil
		}
		if input.Scope == ImportedExtensionContextKindUse && provider.ExtensionRef() != input.ConsumerExtension {
			return nil
		}
		return fmt.Errorf("extension provider does not match the declared direct-use scope")
	default:
		return fmt.Errorf("direct availability ground provider variant is unsupported")
	}
}

func canonicalDirectContextKindAvailabilityGround(
	ground DirectContextKindAvailabilityGround,
) []byte {
	writer := newCanonicalWriter("context-kind-availability-ground.direct.v1")
	writeContextKindAvailabilityCommon(
		&writer,
		ground.context,
		ground.kindID,
		ground.contextSource,
		ground.applicability,
	)
	writer.addString(ground.consumer.String())
	writer.addBytes(ground.useSource.canonical)
	writer.addString(ground.origin)
	writer.addString(ground.role)
	writer.addString(ground.scope.String())
	writer.addBytes(ground.provider.CanonicalBytes())
	return writer.bytes()
}

func cloneDirectContextKindAvailabilityGround(
	ground DirectContextKindAvailabilityGround,
) DirectContextKindAvailabilityGround {
	ground.contextSource = cloneContextKindAvailabilitySource(ground.contextSource)
	ground.applicability = cloneContextKindAvailabilitySource(ground.applicability)
	ground.useSource = cloneContextKindAvailabilitySource(ground.useSource)
	ground.provider = cloneContextKindAvailabilityProvider(ground.provider)
	ground.canonical = append([]byte(nil), ground.canonical...)
	return ground
}

type BridgedContextKindAvailabilityGround struct {
	consumer      TypeEnvExtensionRef
	context       BoundedContextRef
	kindID        KindID
	contextSource ContextKindAvailabilitySource
	applicability ContextKindAvailabilitySource
	useSource     ContextKindAvailabilitySource
	origin        string
	role          string
	provider      ExtensionKindAvailabilityProvider
	bridgeBasis   ContextKindAvailabilityBridgeBasis
	canonical     []byte
}

type BridgedContextKindAvailabilityGroundInput struct {
	ConsumerExtension   TypeEnvExtensionRef
	Context             BoundedContextRef
	KindID              KindID
	ContextSource       ContextKindAvailabilitySource
	ApplicabilitySource ContextKindAvailabilitySource
	UseSource           ContextKindAvailabilitySource
	Origin              string
	Role                string
	Provider            ExtensionKindAvailabilityProvider
	BridgeBasis         ContextKindAvailabilityBridgeBasis
}

func NewBridgedContextKindAvailabilityGround(
	input BridgedContextKindAvailabilityGroundInput,
) (BridgedContextKindAvailabilityGround, error) {
	if err := validateContextKindAvailabilityCommon(
		input.Context,
		input.KindID,
		input.ContextSource,
		input.ApplicabilitySource,
	); err != nil {
		return BridgedContextKindAvailabilityGround{}, err
	}
	if !input.ConsumerExtension.valid() || !input.UseSource.valid() {
		return BridgedContextKindAvailabilityGround{}, fmt.Errorf(
			"bridged availability ground consumer and use source are required",
		)
	}
	if !input.Provider.valid() || !validContextKindAvailabilityBridgeBasis(input.BridgeBasis) {
		return BridgedContextKindAvailabilityGround{}, fmt.Errorf(
			"bridged availability ground provider and exact bridge basis are required",
		)
	}
	if input.Provider.ExtensionRef() == input.ConsumerExtension {
		return BridgedContextKindAvailabilityGround{}, fmt.Errorf(
			"bridged availability ground requires distinct provider and consumer extensions",
		)
	}
	if input.UseSource.Value() != input.Provider.Symbol().Key() {
		return BridgedContextKindAvailabilityGround{}, fmt.Errorf(
			"bridged availability ground must use the exact provider kind symbol",
		)
	}
	if !bridgeMapsProviderKindToConsumer(
		input.BridgeBasis.Bridge(),
		input.Provider.Context(),
		input.Provider.KindID(),
		input.Context,
		input.KindID,
	) {
		return BridgedContextKindAvailabilityGround{}, fmt.Errorf(
			"KindBridge does not map the exact provider context-kind to the consumer context-kind",
		)
	}
	if err := validateContextKindAvailabilityExtensionSources(
		input.ConsumerExtension,
		input.Context,
		[]ContextKindAvailabilitySource{
			input.ContextSource,
			input.ApplicabilitySource,
			input.UseSource,
		},
	); err != nil {
		return BridgedContextKindAvailabilityGround{}, err
	}
	origin, err := exactContextKindAvailabilityLabel("use origin", input.Origin)
	if err != nil {
		return BridgedContextKindAvailabilityGround{}, err
	}
	role, err := exactContextKindAvailabilityLabel("use role", input.Role)
	if err != nil {
		return BridgedContextKindAvailabilityGround{}, err
	}
	ground := BridgedContextKindAvailabilityGround{
		consumer:      input.ConsumerExtension,
		context:       input.Context,
		kindID:        input.KindID,
		contextSource: cloneContextKindAvailabilitySource(input.ContextSource),
		applicability: cloneContextKindAvailabilitySource(input.ApplicabilitySource),
		useSource:     cloneContextKindAvailabilitySource(input.UseSource),
		origin:        origin,
		role:          role,
		provider:      cloneExtensionKindAvailabilityProvider(input.Provider),
		bridgeBasis:   cloneContextKindAvailabilityBridgeBasis(input.BridgeBasis),
	}
	ground.canonical = canonicalBridgedContextKindAvailabilityGround(ground)
	return cloneBridgedContextKindAvailabilityGround(ground), nil
}

func (BridgedContextKindAvailabilityGround) GroundKind() ContextKindAvailabilityGroundKind {
	return BridgedContextKindAvailabilityGroundKind
}

func (ground BridgedContextKindAvailabilityGround) Context() BoundedContextRef {
	return ground.context
}

func (ground BridgedContextKindAvailabilityGround) KindID() KindID { return ground.kindID }

func (ground BridgedContextKindAvailabilityGround) ConsumerExtension() TypeEnvExtensionRef {
	return ground.consumer
}

func (ground BridgedContextKindAvailabilityGround) ContextSource() ContextKindAvailabilitySource {
	return cloneContextKindAvailabilitySource(ground.contextSource)
}

func (ground BridgedContextKindAvailabilityGround) ApplicabilitySource() ContextKindAvailabilitySource {
	return cloneContextKindAvailabilitySource(ground.applicability)
}

func (ground BridgedContextKindAvailabilityGround) EvidenceSource() ContextKindAvailabilitySource {
	return cloneContextKindAvailabilitySource(ground.useSource)
}

func (ground BridgedContextKindAvailabilityGround) Origin() string { return ground.origin }

func (ground BridgedContextKindAvailabilityGround) Role() string { return ground.role }

func (ground BridgedContextKindAvailabilityGround) Provider() ExtensionKindAvailabilityProvider {
	return cloneExtensionKindAvailabilityProvider(ground.provider)
}

func (ground BridgedContextKindAvailabilityGround) BridgeBasis() ContextKindAvailabilityBridgeBasis {
	return cloneContextKindAvailabilityBridgeBasis(ground.bridgeBasis)
}

func (ground BridgedContextKindAvailabilityGround) Bridge() ContextBridge {
	return ground.bridgeBasis.Bridge()
}

func (ground BridgedContextKindAvailabilityGround) CanonicalBytes() []byte {
	return append([]byte(nil), ground.canonical...)
}

func (BridgedContextKindAvailabilityGround) contextKindAvailabilityGroundVariant() {}

func (ground BridgedContextKindAvailabilityGround) valid() bool {
	if validateContextKindAvailabilityCommon(
		ground.context,
		ground.kindID,
		ground.contextSource,
		ground.applicability,
	) != nil {
		return false
	}
	if !ground.consumer.valid() || !ground.useSource.valid() {
		return false
	}
	if !ground.provider.valid() || !validContextKindAvailabilityBridgeBasis(ground.bridgeBasis) {
		return false
	}
	if ground.provider.ExtensionRef() == ground.consumer {
		return false
	}
	if ground.useSource.Value() != ground.provider.Symbol().Key() {
		return false
	}
	if !bridgeMapsProviderKindToConsumer(
		ground.bridgeBasis.Bridge(),
		ground.provider.Context(),
		ground.provider.KindID(),
		ground.context,
		ground.kindID,
	) {
		return false
	}
	if validateContextKindAvailabilityExtensionSources(
		ground.consumer,
		ground.context,
		[]ContextKindAvailabilitySource{
			ground.contextSource,
			ground.applicability,
			ground.useSource,
		},
	) != nil {
		return false
	}
	if _, err := exactContextKindAvailabilityLabel("use origin", ground.origin); err != nil {
		return false
	}
	if _, err := exactContextKindAvailabilityLabel("use role", ground.role); err != nil {
		return false
	}
	expected := canonicalBridgedContextKindAvailabilityGround(ground)
	return bytes.Equal(ground.canonical, expected)
}

func bridgeMapsProviderKindToConsumer(
	bridge ContextBridge,
	providerContext BoundedContextRef,
	providerKind KindID,
	consumerContext BoundedContextRef,
	consumerKind KindID,
) bool {
	mapping := bridge.Mapping()
	forward := bridge.Source().Context() == providerContext &&
		bridge.Target().Context() == consumerContext &&
		mapping.SourceKind() == providerKind &&
		mapping.TargetKind() == consumerKind
	reverse := bridge.Direction() == TwoWayBridge &&
		bridge.Target().Context() == providerContext &&
		bridge.Source().Context() == consumerContext &&
		mapping.TargetKind() == providerKind &&
		mapping.SourceKind() == consumerKind
	return forward || reverse
}

func canonicalBridgedContextKindAvailabilityGround(
	ground BridgedContextKindAvailabilityGround,
) []byte {
	writer := newCanonicalWriter("context-kind-availability-ground.bridged.v1")
	writeContextKindAvailabilityCommon(
		&writer,
		ground.context,
		ground.kindID,
		ground.contextSource,
		ground.applicability,
	)
	writer.addString(ground.consumer.String())
	writer.addBytes(ground.useSource.canonical)
	writer.addString(ground.origin)
	writer.addString(ground.role)
	writer.addBytes(ground.provider.canonical)
	writer.addBytes(ground.bridgeBasis.CanonicalBytes())
	return writer.bytes()
}

func cloneBridgedContextKindAvailabilityGround(
	ground BridgedContextKindAvailabilityGround,
) BridgedContextKindAvailabilityGround {
	ground.contextSource = cloneContextKindAvailabilitySource(ground.contextSource)
	ground.applicability = cloneContextKindAvailabilitySource(ground.applicability)
	ground.useSource = cloneContextKindAvailabilitySource(ground.useSource)
	ground.provider = cloneExtensionKindAvailabilityProvider(ground.provider)
	ground.bridgeBasis = cloneContextKindAvailabilityBridgeBasis(ground.bridgeBasis)
	ground.canonical = append([]byte(nil), ground.canonical...)
	return ground
}

func validateContextKindAvailabilityCommon(
	context BoundedContextRef,
	kindID KindID,
	contextSource ContextKindAvailabilitySource,
	applicability ContextKindAvailabilitySource,
) error {
	if !context.valid() || !kindID.valid() {
		return fmt.Errorf("context-kind availability ground coordinate is required")
	}
	if !sourceNamesContext(contextSource, context) {
		return fmt.Errorf("context source must locate the exact availability context")
	}
	if !sourceNamesContext(applicability, context) {
		return fmt.Errorf(
			"%s source must locate the exact availability context",
			"Applicability",
		)
	}
	return nil
}

func sourceNamesContext(
	source ContextKindAvailabilitySource,
	context BoundedContextRef,
) bool {
	if !source.valid() || source.Value() != context.String() {
		return false
	}
	project, isProjectSource := source.Provenance().(ProjectSourceProvenance)
	return !isProjectSource || project.BoundedContext() == context
}

func validateContextKindAvailabilityExtensionSources(
	extension TypeEnvExtensionRef,
	context BoundedContextRef,
	sources []ContextKindAvailabilitySource,
) error {
	if !extension.valid() || !context.valid() || len(sources) == 0 {
		return fmt.Errorf(
			"context-kind availability extension source coordinate is incomplete",
		)
	}
	var carrier ProjectSourceProvenance
	for index, source := range sources {
		provenance, ok := projectSourceProvenance(source)
		if !ok {
			return fmt.Errorf(
				"context-kind availability extension source %d must retain project-source provenance",
				index,
			)
		}
		if provenance.BoundedContext() != context ||
			!extensionRefMatchesProjectSource(extension, provenance) {
			return fmt.Errorf(
				"context-kind availability extension source %d belongs to another extension coordinate",
				index,
			)
		}
		if index > 0 && !sameProjectSourceCarrier(carrier, provenance) {
			return fmt.Errorf(
				"context-kind availability extension sources must come from one exact carrier edition",
			)
		}
		carrier = provenance
	}
	return nil
}

func exactContextKindAvailabilityLabel(label string, value string) (string, error) {
	parsed, err := parseOpaqueIdentifier("context-kind availability "+label, value)
	if err != nil {
		return "", err
	}
	if !utf8.ValidString(value) || parsed != value {
		return "", fmt.Errorf("context-kind availability %s must be exact canonical UTF-8", label)
	}
	return parsed, nil
}

func writeContextKindAvailabilityCommon(
	writer *canonicalWriter,
	context BoundedContextRef,
	kindID KindID,
	contextSource ContextKindAvailabilitySource,
	applicability ContextKindAvailabilitySource,
) {
	writer.addString(context.String())
	writer.addString(kindID.String())
	writer.addBytes(contextSource.canonical)
	writer.addBytes(applicability.canonical)
}

func validContextKindAvailabilityGround(ground ContextKindAvailabilityGround) bool {
	switch value := ground.(type) {
	case LocalContextKindAvailabilityGround:
		return value.valid()
	case DirectContextKindAvailabilityGround:
		return value.valid()
	case BridgedContextKindAvailabilityGround:
		return value.valid()
	default:
		return false
	}
}

func cloneContextKindAvailabilityGround(
	ground ContextKindAvailabilityGround,
) ContextKindAvailabilityGround {
	switch value := ground.(type) {
	case LocalContextKindAvailabilityGround:
		return cloneLocalContextKindAvailabilityGround(value)
	case DirectContextKindAvailabilityGround:
		return cloneDirectContextKindAvailabilityGround(value)
	case BridgedContextKindAvailabilityGround:
		return cloneBridgedContextKindAvailabilityGround(value)
	default:
		return nil
	}
}

type ContextKindAvailabilityGroundSet struct {
	context   BoundedContextRef
	kindID    KindID
	grounds   []ContextKindAvailabilityGround
	canonical []byte
}

func NewContextKindAvailabilityGroundSet(
	grounds []ContextKindAvailabilityGround,
) (ContextKindAvailabilityGroundSet, error) {
	canonicalGrounds, context, kindID, err := canonicalContextKindAvailabilityGrounds(grounds)
	if err != nil {
		return ContextKindAvailabilityGroundSet{}, err
	}
	set := ContextKindAvailabilityGroundSet{
		context: context,
		kindID:  kindID,
		grounds: canonicalGrounds,
	}
	set.canonical = canonicalContextKindAvailabilityGroundSet(set.grounds)
	if len(set.canonical) > maximumContextKindGroundSetBytes {
		return ContextKindAvailabilityGroundSet{}, fmt.Errorf(
			"context-kind availability ground set exceeds %d canonical bytes",
			maximumContextKindGroundSetBytes,
		)
	}
	return cloneContextKindAvailabilityGroundSet(set), nil
}

func (set ContextKindAvailabilityGroundSet) Context() BoundedContextRef { return set.context }

func (set ContextKindAvailabilityGroundSet) KindID() KindID { return set.kindID }

func (set ContextKindAvailabilityGroundSet) Grounds() []ContextKindAvailabilityGround {
	return cloneContextKindAvailabilityGrounds(set.grounds)
}

func (set ContextKindAvailabilityGroundSet) CanonicalBytes() []byte {
	return append([]byte(nil), set.canonical...)
}

func (set ContextKindAvailabilityGroundSet) Digest() SHA256Digest {
	return digestCanonicalBytes(set.canonical)
}

func (set ContextKindAvailabilityGroundSet) valid() bool {
	if len(set.grounds) == 0 || len(set.grounds) > maximumContextKindAvailabilityGrounds {
		return false
	}
	if !set.context.valid() || !set.kindID.valid() {
		return false
	}
	for index, ground := range set.grounds {
		if !validContextKindAvailabilityGround(ground) {
			return false
		}
		if ground.Context() != set.context || ground.KindID() != set.kindID {
			return false
		}
		if index == 0 {
			continue
		}
		previous := set.grounds[index-1].CanonicalBytes()
		current := ground.CanonicalBytes()
		if bytes.Compare(previous, current) >= 0 {
			return false
		}
	}
	expected := canonicalContextKindAvailabilityGroundSet(set.grounds)
	return len(expected) <= maximumContextKindGroundSetBytes && bytes.Equal(set.canonical, expected)
}

func canonicalContextKindAvailabilityGrounds(
	grounds []ContextKindAvailabilityGround,
) ([]ContextKindAvailabilityGround, BoundedContextRef, KindID, error) {
	if len(grounds) == 0 {
		return nil, BoundedContextRef{}, KindID{}, fmt.Errorf(
			"context-kind availability requires a nonempty ground set",
		)
	}
	if len(grounds) > maximumContextKindAvailabilityGrounds {
		return nil, BoundedContextRef{}, KindID{}, fmt.Errorf(
			"context-kind availability ground count exceeds %d",
			maximumContextKindAvailabilityGrounds,
		)
	}
	owned := cloneContextKindAvailabilityGrounds(grounds)
	for index, ground := range owned {
		if !validContextKindAvailabilityGround(ground) {
			return nil, BoundedContextRef{}, KindID{}, fmt.Errorf(
				"context-kind availability ground %d is invalid",
				index,
			)
		}
	}
	sort.Slice(owned, func(left, right int) bool {
		return bytes.Compare(owned[left].CanonicalBytes(), owned[right].CanonicalBytes()) < 0
	})
	context := owned[0].Context()
	kindID := owned[0].KindID()
	for index, ground := range owned {
		if ground.Context() != context || ground.KindID() != kindID {
			return nil, BoundedContextRef{}, KindID{}, fmt.Errorf(
				"context-kind availability ground %d belongs to another context-kind coordinate",
				index,
			)
		}
		if index == 0 {
			continue
		}
		if bytes.Equal(ground.CanonicalBytes(), owned[index-1].CanonicalBytes()) {
			return nil, BoundedContextRef{}, KindID{}, fmt.Errorf(
				"duplicate context-kind availability ground",
			)
		}
	}
	return owned, context, kindID, nil
}

func canonicalContextKindAvailabilityGroundSet(
	grounds []ContextKindAvailabilityGround,
) []byte {
	writer := newCanonicalWriter("context-kind-availability-ground-set.v1")
	writer.addUint64(uint64(len(grounds)))
	for _, ground := range grounds {
		writer.addBytes(ground.CanonicalBytes())
	}
	return writer.bytes()
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

func cloneContextKindAvailabilityGroundSet(
	set ContextKindAvailabilityGroundSet,
) ContextKindAvailabilityGroundSet {
	set.grounds = cloneContextKindAvailabilityGrounds(set.grounds)
	set.canonical = append([]byte(nil), set.canonical...)
	return set
}

// ContextKindAvailability is an internal compiler-derived TypeEnv projection.
// It says only that one kind vocabulary is available in one bounded context.
// Its exact ground set is not MemberOf, E.24.UK admission, a SchemaChange,
// project authority, truth, evidence, or an authored Local-Practice field.
type ContextKindAvailability struct {
	context   BoundedContextRef
	kindID    KindID
	grounds   ContextKindAvailabilityGroundSet
	canonical []byte
}

func NewContextKindAvailability(
	context BoundedContextRef,
	kindID KindID,
	grounds ContextKindAvailabilityGroundSet,
) (ContextKindAvailability, error) {
	if !context.valid() {
		return ContextKindAvailability{}, fmt.Errorf("kind-availability context is required")
	}
	if !kindID.valid() {
		return ContextKindAvailability{}, fmt.Errorf("available kind ID is required")
	}
	if !grounds.valid() {
		return ContextKindAvailability{}, fmt.Errorf(
			"kind-availability canonical ground set is required",
		)
	}
	if grounds.Context() != context || grounds.KindID() != kindID {
		return ContextKindAvailability{}, fmt.Errorf(
			"kind-availability ground set belongs to another context-kind coordinate",
		)
	}
	availability := ContextKindAvailability{
		context: context,
		kindID:  kindID,
		grounds: cloneContextKindAvailabilityGroundSet(grounds),
	}
	availability.canonical = canonicalContextKindAvailability(availability)
	return cloneContextKindAvailability(availability), nil
}

func (availability ContextKindAvailability) Context() BoundedContextRef {
	return availability.context
}

func (availability ContextKindAvailability) KindID() KindID { return availability.kindID }

func (availability ContextKindAvailability) GroundSet() ContextKindAvailabilityGroundSet {
	return cloneContextKindAvailabilityGroundSet(availability.grounds)
}

func (availability ContextKindAvailability) Grounds() []ContextKindAvailabilityGround {
	return availability.grounds.Grounds()
}

func (availability ContextKindAvailability) CanonicalBytes() []byte {
	return append([]byte(nil), availability.canonical...)
}

func (availability ContextKindAvailability) Digest() SHA256Digest {
	return digestCanonicalBytes(availability.canonical)
}

func (availability ContextKindAvailability) key() string {
	return availability.context.String() + "/kind/" + availability.kindID.String()
}

func (availability ContextKindAvailability) valid() bool {
	if !availability.context.valid() || !availability.kindID.valid() {
		return false
	}
	if !availability.grounds.valid() {
		return false
	}
	if availability.grounds.Context() != availability.context ||
		availability.grounds.KindID() != availability.kindID {
		return false
	}
	expected := canonicalContextKindAvailability(availability)
	return bytes.Equal(availability.canonical, expected)
}

func canonicalContextKindAvailability(availability ContextKindAvailability) []byte {
	writer := newCanonicalWriter("context-kind-availability.v1")
	writer.addString(availability.context.String())
	writer.addString(availability.kindID.String())
	writer.addBytes(availability.grounds.canonical)
	return writer.bytes()
}

func cloneContextKindAvailability(
	availability ContextKindAvailability,
) ContextKindAvailability {
	availability.grounds = cloneContextKindAvailabilityGroundSet(availability.grounds)
	availability.canonical = append([]byte(nil), availability.canonical...)
	return availability
}

func cloneContextKindAvailabilities(
	values []ContextKindAvailability,
) []ContextKindAvailability {
	result := make([]ContextKindAvailability, 0, len(values))
	for _, value := range values {
		result = append(result, cloneContextKindAvailability(value))
	}
	return result
}
