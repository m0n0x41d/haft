package typedmemory

import "fmt"

// EntitySetSymbolID is the source-declared identity exported through a
// SignatureManifest. It is deliberately not an EntitySetDefinitionRef: the
// latter identifies one concrete runtime definition at a context coordinate.
type EntitySetSymbolID struct {
	value string
}

func NewEntitySetSymbolID(raw string) (EntitySetSymbolID, error) {
	value, err := parseQualifiedIdentifier("EntitySet symbol ID", raw)
	if err != nil {
		return EntitySetSymbolID{}, err
	}
	return EntitySetSymbolID{value: value}, nil
}

func (id EntitySetSymbolID) String() string { return id.value }

func (id EntitySetSymbolID) valid() bool { return id.value != "" }

// KindSignatureSymbolID is the source-declared identity exported through a
// SignatureManifest. It must not be derived from a KindSignatureRef.
type KindSignatureSymbolID struct {
	value string
}

func NewKindSignatureSymbolID(raw string) (KindSignatureSymbolID, error) {
	value, err := parseQualifiedIdentifier("KindSignature symbol ID", raw)
	if err != nil {
		return KindSignatureSymbolID{}, err
	}
	return KindSignatureSymbolID{value: value}, nil
}

func (id KindSignatureSymbolID) String() string { return id.value }

func (id KindSignatureSymbolID) valid() bool { return id.value != "" }

func EntitySetSymbolRef(id EntitySetSymbolID) (SchemaSymbolRef, error) {
	if !id.valid() {
		return SchemaSymbolRef{}, fmt.Errorf("EntitySet symbol ID is required")
	}
	return SchemaSymbolRef{kind: EntitySetSymbol, key: id.String()}, nil
}

func KindSignatureSymbolRef(id KindSignatureSymbolID) (SchemaSymbolRef, error) {
	if !id.valid() {
		return SchemaSymbolRef{}, fmt.Errorf("KindSignature symbol ID is required")
	}
	return SchemaSymbolRef{kind: KindSignatureSymbol, key: id.String()}, nil
}

// DefineEntitySetSchemaChange binds one explicit exported source symbol to one
// concrete runtime EntitySet definition. The symbol and runtime coordinate are
// non-interchangeable identities.
type DefineEntitySetSchemaChange struct {
	exportedSymbol SchemaSymbolRef
	definition     EntitySetDefinition
}

func NewDefineEntitySetSchemaChange(
	exportedSymbol EntitySetSymbolID,
	definition EntitySetDefinition,
) (DefineEntitySetSchemaChange, error) {
	if !exportedSymbol.valid() {
		return DefineEntitySetSchemaChange{}, fmt.Errorf("EntitySet exported symbol is required")
	}
	if !definition.valid() {
		return DefineEntitySetSchemaChange{}, fmt.Errorf("EntitySet-definition schema change is invalid")
	}
	symbol, err := EntitySetSymbolRef(exportedSymbol)
	if err != nil {
		return DefineEntitySetSchemaChange{}, err
	}
	return DefineEntitySetSchemaChange{
		exportedSymbol: symbol,
		definition:     definition,
	}, nil
}

func (change DefineEntitySetSchemaChange) ExportedSymbolID() EntitySetSymbolID {
	id, _ := NewEntitySetSymbolID(change.exportedSymbol.Key())
	return id
}

func (change DefineEntitySetSchemaChange) ExportedSymbol() SchemaSymbolRef {
	return change.exportedSymbol
}

func (change DefineEntitySetSchemaChange) Definition() EntitySetDefinition {
	return change.definition
}

func (change DefineEntitySetSchemaChange) ChangeKey() string {
	return "define-entity-set:" + change.definition.Ref().Context().String()
}

func (change DefineEntitySetSchemaChange) ProvidedSymbols() []SchemaSymbolRef {
	return []SchemaSymbolRef{change.exportedSymbol}
}

func (change DefineEntitySetSchemaChange) Provenance() DeclarationProvenance {
	return change.definition.Provenance()
}

func (DefineEntitySetSchemaChange) schemaChangeVariant() {}

// DefineKindSignatureSchemaChange binds one explicit exported source symbol to
// one concrete runtime KindSignature definition. ChangeKey is intentionally
// derived from kind+context, never from the exported alias.
type DefineKindSignatureSchemaChange struct {
	exportedSymbol SchemaSymbolRef
	definition     KindSignatureDefinition
}

func NewDefineKindSignatureSchemaChange(
	exportedSymbol KindSignatureSymbolID,
	definition KindSignatureDefinition,
) (DefineKindSignatureSchemaChange, error) {
	if !exportedSymbol.valid() {
		return DefineKindSignatureSchemaChange{}, fmt.Errorf("KindSignature exported symbol is required")
	}
	if !definition.valid() {
		return DefineKindSignatureSchemaChange{}, fmt.Errorf("KindSignature-definition schema change is invalid")
	}
	symbol, err := KindSignatureSymbolRef(exportedSymbol)
	if err != nil {
		return DefineKindSignatureSchemaChange{}, err
	}
	return DefineKindSignatureSchemaChange{
		exportedSymbol: symbol,
		definition:     definition,
	}, nil
}

func (change DefineKindSignatureSchemaChange) ExportedSymbolID() KindSignatureSymbolID {
	id, _ := NewKindSignatureSymbolID(change.exportedSymbol.Key())
	return id
}

func (change DefineKindSignatureSchemaChange) ExportedSymbol() SchemaSymbolRef {
	return change.exportedSymbol
}

func (change DefineKindSignatureSchemaChange) Definition() KindSignatureDefinition {
	return change.definition
}

func (change DefineKindSignatureSchemaChange) ChangeKey() string {
	ref := change.definition.Ref()
	coordinate := exactTupleKey(
		"kind-signature",
		ref.ValueKind().ID().String(),
		ref.Context().String(),
	)
	return "define-kind-signature:" + coordinate
}

func (change DefineKindSignatureSchemaChange) ProvidedSymbols() []SchemaSymbolRef {
	return []SchemaSymbolRef{change.exportedSymbol}
}

func (change DefineKindSignatureSchemaChange) Provenance() DeclarationProvenance {
	return change.definition.Provenance()
}

func (DefineKindSignatureSchemaChange) schemaChangeVariant() {}
