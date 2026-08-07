package typedmemory

import (
	"reflect"
	"strings"
	"testing"
)

func TestTypeEnvExtensionProposalIsNonBindingAndDisjointFromMemoryChange(t *testing.T) {
	fixture := newExtensionFixture(t)
	proposal := fixture.build(t)

	if _, isMemoryChange := any(fixture.change).(MemoryChange); isMemoryChange {
		t.Fatal("SchemaChange is also a MemoryChange")
	}
	proposalType := reflect.TypeOf(proposal)
	if _, hasActivateMethod := proposalType.MethodByName("Activate"); hasActivateMethod {
		t.Fatal("non-binding TypeEnvExtensionProposal exposes an Activate method")
	}
	if proposal.BaseTypeEnv() != fixture.base {
		t.Fatal("extension proposal lost the exact base TypeEnv")
	}
	if proposal.Ref().ID() != fixture.id {
		t.Fatal("extension proposal lost its extension ID")
	}
	if proposal.Ref().Digest() != digestCanonicalBytes(proposal.CanonicalBytes()) {
		t.Fatal("extension proposal ref was not derived from its canonical bytes")
	}
	if len(proposal.SchemaChanges().Changes()) != 1 {
		t.Fatalf("schema changes = %d; want 1", len(proposal.SchemaChanges().Changes()))
	}
}

func TestTypeEnvExtensionProposalDefensivelyCopiesManifestAndChanges(t *testing.T) {
	fixture := newExtensionFixture(t)
	proposal := fixture.build(t)

	provides := proposal.SignatureManifest().Provides()
	provides[0] = SchemaSymbolRef{}
	if !proposal.SignatureManifest().Provides()[0].valid() {
		t.Fatal("mutating manifest accessor changed the proposal")
	}

	changes := proposal.SchemaChanges().Changes()
	changes[0] = nil
	if proposal.SchemaChanges().Changes()[0] == nil {
		t.Fatal("mutating SchemaChangeSet accessor changed the proposal")
	}
}

func TestTypeEnvExtensionProposalRejectsWrongSourceAndUnrealizedProvide(t *testing.T) {
	fixture := newExtensionFixture(t)
	wrongCarrier := typeEnvTestCarrierRef(t, "carrier:wrong.md")
	wrongProvenance := extensionTestProjectProvenance(
		t,
		fixture,
		wrongCarrier,
		fixture.symbol,
	)
	wrongDefinition, err := NewKindDefinition(fixture.kindID, wrongProvenance)
	if err != nil {
		t.Fatalf("NewKindDefinition() error = %v", err)
	}
	wrongChange, _ := NewDefineKindSchemaChange(wrongDefinition)
	wrongSet, _ := NewSchemaChangeSet([]SchemaChange{wrongChange})

	_, err = fixture.builder().SetSchemaChanges(wrongSet).Build()
	if err == nil || !strings.Contains(err.Error(), "does not match proposal source") {
		t.Fatalf("wrong-source error = %v; want source mismatch", err)
	}

	undeclaredKind := typeEnvTestKindID(t, "Haft.Undeclared")
	undeclaredSymbol, _ := KindSymbolRef(undeclaredKind)
	undeclaredProvenance := extensionTestProjectProvenance(
		t,
		fixture,
		fixture.carrier,
		undeclaredSymbol,
	)
	undeclaredDefinition, _ := NewKindDefinition(undeclaredKind, undeclaredProvenance)
	undeclaredChange, _ := NewDefineKindSchemaChange(undeclaredDefinition)
	undeclaredSet, _ := NewSchemaChangeSet([]SchemaChange{undeclaredChange})

	_, err = fixture.builder().SetSchemaChanges(undeclaredSet).Build()
	if err == nil || !strings.Contains(err.Error(), "no exact SchemaChange realization") {
		t.Fatalf("unrealized-provide error = %v; want exact manifest rejection", err)
	}
}

func TestTypeEnvExtensionProposalRejectsPhantomManifestProvide(t *testing.T) {
	fixture := newExtensionFixture(t)
	phantom := mustExtensionValue(KindSymbolRef(typeEnvTestKindID(t, "Haft.Phantom")))
	manifest := mustExtensionValue(NewSignatureManifest(
		fixture.manifest.Ref(),
		fixture.manifest.Imports(),
		[]SchemaSymbolRef{fixture.symbol, phantom},
	))

	_, err := fixture.builder().SetSignatureManifest(manifest).Build()
	if err == nil || !strings.Contains(err.Error(), "SchemaChanges realize") {
		t.Fatalf("phantom-provide error = %v; want exact manifest-set rejection", err)
	}
}

func TestTypeEnvExtensionProposalRejectsManifestImportForExportingDeclaration(t *testing.T) {
	fixture := newExtensionFixture(t)
	provenance := extensionTestProjectProvenanceWithBasis(
		t,
		fixture,
		fixture.carrier,
		fixture.symbol,
		ManifestImport,
	)
	definition := mustExtensionValue(NewKindDefinition(fixture.kindID, provenance))
	change := mustExtensionValue(NewDefineKindSchemaChange(definition))
	changes := mustExtensionValue(NewSchemaChangeSet([]SchemaChange{change}))

	_, err := fixture.builder().SetSchemaChanges(changes).Build()
	if err == nil || !strings.Contains(err.Error(), "requires a manifest provide basis") {
		t.Fatalf("import-basis error = %v; want exporting-direction rejection", err)
	}
}

func TestTypeEnvExtensionProposalRejectsUnassociatedManifestBasis(t *testing.T) {
	fixture := newExtensionFixture(t)
	contextSymbol := mustExtensionValue(BoundedContextSymbolRef(fixture.context))
	manifest := mustExtensionValue(NewSignatureManifest(
		fixture.manifest.Ref(),
		nil,
		[]SchemaSymbolRef{fixture.symbol, contextSymbol},
	))
	badKindProvenance := extensionTestProjectProvenanceWithBasis(
		t,
		fixture,
		fixture.carrier,
		contextSymbol,
		ManifestProvide,
	)
	definition := mustExtensionValue(NewKindDefinition(fixture.kindID, badKindProvenance))
	defineKind := mustExtensionValue(NewDefineKindSchemaChange(definition))
	contextProvenance := extensionTestProjectProvenanceWithBasis(
		t,
		fixture,
		fixture.carrier,
		contextSymbol,
		ManifestProvide,
	)
	context := mustExtensionValue(NewBoundedContext(fixture.context, contextProvenance))
	addContext := mustExtensionValue(NewAddBoundedContextSchemaChange(context))
	changes := mustExtensionValue(NewSchemaChangeSet([]SchemaChange{defineKind, addContext}))

	_, err := fixture.builder().
		SetSignatureManifest(manifest).
		SetSchemaChanges(changes).
		Build()
	if err == nil || !strings.Contains(err.Error(), "is not associated with the declaration") {
		t.Fatalf("unassociated-basis error = %v; want declaration-association rejection", err)
	}
}

func TestTypeEnvExtensionProposalRejectsUnassociatedRelationSlotBasis(t *testing.T) {
	factory := newExhaustiveExtensionFactory(t)
	kindID := typeEnvTestKindID(t, "Haft.ProjectMemory")
	refKindID := typeEnvTestRefKindID(t, "Haft.ProjectMemoryRef")
	refKind := typeEnvTestRefKindRef(t, factory.base, refKindID)
	change := factory.relationSignatureChange(kindID, refKind, false).(DefineTypedRelationDeclarationFragmentSchemaChange)
	fragment := change.Fragment()
	if change.Posture() != RelationDeclarationTypedFragment {
		t.Fatalf("relation schema-change posture = %q", change.Posture())
	}
	slots := fragment.Slots()
	relationSymbol := mustExtensionValue(RelationSymbolRef(fragment.Ref().ID()))
	for index, slot := range slots {
		if _, isProjectSource := slot.Provenance().(ProjectSourceProvenance); !isProjectSource {
			continue
		}
		slots[index] = mustExtensionValue(NewSlotSpec(
			slot.SlotKind(),
			slot.Target(),
			slot.Cardinality(),
			factory.projectProvenance(relationSymbol, VocabularyRow, ManifestProvide),
		))
	}
	invalidFragment := mustExtensionValue(NewTypedRelationDeclarationFragment(
		fragment.Ref(),
		fragment.Contexts(),
		slots,
		fragment.Provenance(),
	))
	invalidChange := mustExtensionValue(
		NewDefineTypedRelationDeclarationFragmentSchemaChange(invalidFragment),
	)
	manifest := mustExtensionValue(NewSignatureManifest(factory.manifest, nil, invalidChange.ProvidedSymbols()))
	changes := mustExtensionValue(NewSchemaChangeSet([]SchemaChange{invalidChange}))
	compatibility := mustExtensionValue(NewTypeEnvCompatibilityDiff(factory.base, nil))
	revalidation := mustExtensionValue(NewExistingAssertionRevalidationReport(
		RevalidationClean,
		NewGraphRevision(1),
		nil,
		typeEnvTestDigest(t, 0xdd),
	))
	id := mustExtensionValue(NewExtensionID("haft.typed-memory.invalid-slot.v1"))

	_, err := NewLoweredTypeEnvExtensionProposalBuilder(id).
		SetSourceCarrier(factory.carrier, factory.edition, factory.carrierHash).
		SetBaseTypeEnv(factory.base).
		SetBoundedContext(factory.context).
		SetSignatureManifest(manifest).
		SetSchemaChanges(changes).
		SetCompilerSchemaVersion(typeEnvTestCompilerVersion(t, "local-practice.invalid-slot.v1")).
		SetCompatibilityDiff(compatibility).
		SetRevalidationReport(revalidation).
		Build()
	if err == nil || !strings.Contains(err.Error(), "is not associated with the declaration") {
		t.Fatalf("slot-basis error = %v; want slot-association rejection", err)
	}
}

func TestSchemaChangeSetRejectsDuplicateSchemaEffects(t *testing.T) {
	fixture := newExtensionFixture(t)
	_, err := NewSchemaChangeSet([]SchemaChange{fixture.change, fixture.change})
	if err == nil || !strings.Contains(err.Error(), "duplicate schema change") {
		t.Fatalf("duplicate SchemaChangeSet error = %v; want duplicate rejection", err)
	}
}

func TestSchemaChangeSetRejectsNilBeforeCanonicalSorting(t *testing.T) {
	if _, err := NewSchemaChangeSet([]SchemaChange{nil}); err == nil {
		t.Fatal("SchemaChangeSet accepted nil SchemaChange")
	}
}

type extensionFixture struct {
	id            ExtensionID
	base          TypeEnvRef
	context       BoundedContextRef
	carrier       CarrierRef
	edition       CarrierEdition
	carrierHash   SHA256Digest
	manifest      SignatureManifest
	symbol        SchemaSymbolRef
	kindID        KindID
	change        SchemaChange
	changes       SchemaChangeSet
	compiler      CompilerSchemaVersion
	compatibility TypeEnvCompatibilityDiff
	revalidation  ExistingAssertionRevalidationReport
}

func newExtensionFixture(t *testing.T) extensionFixture {
	t.Helper()
	extensionID, err := NewExtensionID("haft.typed-memory.v1")
	if err != nil {
		t.Fatalf("NewExtensionID() error = %v", err)
	}
	base := typeEnvTestTypeEnvRef(t, 0x42)
	context := typeEnvTestContextRef(t, "ctx:haft")
	carrier := typeEnvTestCarrierRef(t, "carrier:.haft/typed-memory-local-practice.md")
	edition := typeEnvTestCarrierEdition(t, "2026-07-16")
	carrierHash := typeEnvTestDigest(t, 0x43)
	kindID := typeEnvTestKindID(t, "Haft.ProjectMemory")
	symbol, _ := KindSymbolRef(kindID)
	manifestRef := typeEnvTestManifestRef(t)
	manifest, err := NewSignatureManifest(manifestRef, nil, []SchemaSymbolRef{symbol})
	if err != nil {
		t.Fatalf("NewSignatureManifest() error = %v", err)
	}
	partial := extensionFixture{
		id:          extensionID,
		base:        base,
		context:     context,
		carrier:     carrier,
		edition:     edition,
		carrierHash: carrierHash,
		manifest:    manifest,
		symbol:      symbol,
		kindID:      kindID,
	}
	provenance := extensionTestProjectProvenance(t, partial, carrier, symbol)
	definition, err := NewKindDefinition(kindID, provenance)
	if err != nil {
		t.Fatalf("NewKindDefinition() error = %v", err)
	}
	change, err := NewDefineKindSchemaChange(definition)
	if err != nil {
		t.Fatalf("NewDefineKindSchemaChange() error = %v", err)
	}
	changes, err := NewSchemaChangeSet([]SchemaChange{change})
	if err != nil {
		t.Fatalf("NewSchemaChangeSet() error = %v", err)
	}
	compatibilityChange, err := NewCompatibilityChange(
		symbol,
		CompatibilityAdded,
		"new project-local kind",
	)
	if err != nil {
		t.Fatalf("NewCompatibilityChange() error = %v", err)
	}
	compatibility, err := NewTypeEnvCompatibilityDiff(
		base,
		[]CompatibilityChange{compatibilityChange},
	)
	if err != nil {
		t.Fatalf("NewTypeEnvCompatibilityDiff() error = %v", err)
	}
	revalidation, err := NewExistingAssertionRevalidationReport(
		RevalidationClean,
		NewGraphRevision(7),
		nil,
		typeEnvTestDigest(t, 0x44),
	)
	if err != nil {
		t.Fatalf("NewExistingAssertionRevalidationReport() error = %v", err)
	}
	partial.change = change
	partial.changes = changes
	partial.compiler = typeEnvTestCompilerVersion(t, "local-practice.v1")
	partial.compatibility = compatibility
	partial.revalidation = revalidation
	return partial
}

func (fixture extensionFixture) builder() *LoweredTypeEnvExtensionProposalBuilder {
	return NewLoweredTypeEnvExtensionProposalBuilder(fixture.id).
		SetSourceCarrier(fixture.carrier, fixture.edition, fixture.carrierHash).
		SetBaseTypeEnv(fixture.base).
		SetBoundedContext(fixture.context).
		SetSignatureManifest(fixture.manifest).
		SetSchemaChanges(fixture.changes).
		SetCompilerSchemaVersion(fixture.compiler).
		SetCompatibilityDiff(fixture.compatibility).
		SetRevalidationReport(fixture.revalidation)
}

func (fixture extensionFixture) build(t *testing.T) TypeEnvExtensionProposal {
	t.Helper()
	proposal, err := fixture.builder().Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	return proposal
}

func extensionTestProjectProvenance(
	t *testing.T,
	fixture extensionFixture,
	carrier CarrierRef,
	symbol SchemaSymbolRef,
) ProjectSourceProvenance {
	return extensionTestProjectProvenanceWithBasis(
		t,
		fixture,
		carrier,
		symbol,
		ManifestProvide,
	)
}

func extensionTestProjectProvenanceWithBasis(
	t *testing.T,
	fixture extensionFixture,
	carrier CarrierRef,
	symbol SchemaSymbolRef,
	direction ManifestDirection,
) ProjectSourceProvenance {
	t.Helper()
	basis, err := NewManifestSymbolBasis(
		fixture.manifest.Ref(),
		direction,
		symbol,
	)
	if err != nil {
		t.Fatalf("NewManifestSymbolBasis() error = %v", err)
	}
	provenance, err := NewProjectSourceProvenanceBuilder(
		typeEnvTestProvenanceRef(t, "prov:project:"+symbol.Key()),
		carrier,
		fixture.edition,
		fixture.carrierHash,
	).
		SetDeclarationRange(typeEnvTestLineRange(t, 20, 28)).
		SetCompilerRule(typeEnvTestCompilerRuleID(t, "local-practice.kind.v1")).
		SetBoundedContext(fixture.context).
		SetBaseTypeEnv(fixture.base).
		SetSignatureBlockRow(VocabularyRow).
		SetManifestBasis(basis).
		Build()
	if err != nil {
		t.Fatalf("project provenance Build() error = %v", err)
	}
	return provenance
}
