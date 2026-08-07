package typedmemory

import (
	"bytes"
	"encoding/binary"
	"testing"
)

type errorCanonicalMaterializer interface {
	CanonicalBytes() ([]byte, error)
	Digest() (SHA256Digest, error)
}

func TestCanonicalMaterializationDeclareEntityMatchesProspectiveConsumers(t *testing.T) {
	fixture := newLocalReferenceAdmissionFixture(t, "local:declaration-materialization", true)
	declaration, localReference, prefix := prospectiveViewTestBasis(t, fixture)
	canonical, err := declaration.CanonicalBytes()
	if err != nil {
		t.Fatalf("DeclareEntity.CanonicalBytes(): %v", err)
	}
	if !bytes.Equal(canonical, mustCanonicalMemoryChange(t, declaration)) {
		t.Fatal("DeclareEntity.CanonicalBytes differs from candidate MemoryChange encoding")
	}
	digest, err := declaration.Digest()
	if err != nil {
		t.Fatalf("DeclareEntity.Digest(): %v", err)
	}

	resolution, err := NewSameBatchDeclarationResolution(SameBatchDeclarationResolutionInput{
		LocalReference:           localReference,
		DeclarationChangeOrdinal: 0,
		Declaration:              declaration,
		PersistedReference:       fixture.stableReference,
	})
	if err != nil {
		t.Fatalf("NewSameBatchDeclarationResolution(): %v", err)
	}
	view, err := NewProspectiveBatchView(ProspectiveBatchViewInput{
		TypeEnv:                  fixture.environment.Ref(),
		PreStateGraphRevision:    fixture.snapshot.GraphRevision(),
		EvaluationChangeOrdinal:  1,
		DeclarationChangeOrdinal: 0,
		Declaration:              declaration,
		LocalReference:           localReference,
		PersistedReference:       fixture.stableReference,
		OrderedCandidatePrefix:   prefix,
	})
	if err != nil {
		t.Fatalf("NewProspectiveBatchView(): %v", err)
	}
	if !bytes.Equal(resolution.DeclarationCanonicalBytes(), canonical) ||
		!bytes.Equal(view.DeclarationCanonicalBytes(), canonical) {
		t.Fatal("prospective consumers changed exact DeclareEntity candidate bytes")
	}
	if resolution.DeclarationDigest() != digest || view.DeclarationDigest() != digest {
		t.Fatal("prospective consumers diverged from DeclareEntity candidate digest")
	}
}

func TestCanonicalMaterializationAccessorsMatchValidatedRelationEnvelope(t *testing.T) {
	fixture := newValidationFixture(t)
	verdict := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		fixture.changeSet,
	)
	valid, ok := verdict.(Valid)
	if !ok {
		t.Fatalf("verdict = %T (%s); want Valid", verdict, verdict.Kind())
	}

	changeSet := valid.ChangeSet()
	changes := changeSet.Changes()
	validated, ok := changes[0].(ValidatedRelationInstance)
	if !ok {
		t.Fatalf("validated change = %T; want ValidatedRelationInstance", changes[0])
	}
	relation := validated.Relation()
	relationBytes := assertErrorCanonicalMaterialization(
		t,
		relation,
		mustCanonicalRelationInstance(t, relation),
	)
	changeSetBytes, err := changeSet.CanonicalBytes()
	if err != nil {
		t.Fatalf("ValidatedMemoryChangeSet.CanonicalBytes(): %v", err)
	}
	assertContainsFramedCanonicalValue(t, changeSetBytes, relationBytes)

	foundReference := false
	foundValue := false
	for _, binding := range relation.Bindings() {
		bindingBytes := binding.CanonicalBytes()
		if !bytes.Equal(bindingBytes, canonicalSlotBinding(binding)) {
			t.Fatal("SlotBinding.CanonicalBytes differs from the canonical relation encoding")
		}
		if binding.Digest() != digestCanonicalBytes(bindingBytes) {
			t.Fatal("SlotBinding.Digest does not hash SlotBinding.CanonicalBytes")
		}
		assertContainsFramedCanonicalValue(t, relationBytes, bindingBytes)

		for _, filler := range binding.Fillers() {
			if value, ok := filler.(ValueFiller); ok {
				foundValue = true
				valueBytes := value.CanonicalBytes()
				if !bytes.Equal(valueBytes, canonicalSlotFiller(value)) {
					t.Fatal("ValueFiller.CanonicalBytes differs from the canonical slot encoding")
				}
				if value.Digest() != digestCanonicalBytes(valueBytes) {
					t.Fatal("ValueFiller.Digest does not hash ValueFiller.CanonicalBytes")
				}
				assertContainsFramedCanonicalValue(t, bindingBytes, valueBytes)
			}
			reference, ok := filler.(ReferenceFiller)
			if !ok {
				continue
			}
			foundReference = true
			referenceBytes := reference.CanonicalBytes()
			if !bytes.Equal(referenceBytes, canonicalSlotFiller(reference)) {
				t.Fatal("ReferenceFiller.CanonicalBytes differs from the canonical slot encoding")
			}
			if reference.Digest() != digestCanonicalBytes(referenceBytes) {
				t.Fatal("ReferenceFiller.Digest does not hash ReferenceFiller.CanonicalBytes")
			}
			assertContainsFramedCanonicalValue(t, bindingBytes, referenceBytes)
		}
	}
	if !foundReference {
		t.Fatal("validated fixture did not contain a ReferenceFiller")
	}
	if !foundValue {
		t.Fatal("validated fixture did not contain a ValueFiller")
	}
}

func TestCanonicalMaterializationAccessorsMatchValidatedIdentityEnvelope(t *testing.T) {
	entity := mustEntityID(t, "entity:canonical-materialization")
	context := mustBoundedContextRef(t, "context:canonical-materialization")
	provenance := typeEnvTestProvenanceRef(t, "memory:canonical-materialization")
	admittedAlias := mustEntityAlias(t, "canonical materialization")
	replacementAlias := mustEntityAlias(t, "canonical materialization service")

	admit, err := NewAdmitAlias(entity, admittedAlias, context, provenance)
	if err != nil {
		t.Fatalf("NewAdmitAlias(): %v", err)
	}
	supersede, err := NewSupersedeAlias(
		entity,
		admittedAlias,
		replacementAlias,
		context,
		provenance,
	)
	if err != nil {
		t.Fatalf("NewSupersedeAlias(): %v", err)
	}
	assertion, err := NewAssertionID("assertion:canonical-materialization")
	if err != nil {
		t.Fatalf("NewAssertionID(): %v", err)
	}
	reason, err := NewRetractionReason("superseded source evidence")
	if err != nil {
		t.Fatalf("NewRetractionReason(): %v", err)
	}
	retraction, err := NewRetractAssertion(assertion, reason, provenance)
	if err != nil {
		t.Fatalf("NewRetractAssertion(): %v", err)
	}

	changeSet := newValidatedMemoryChangeSet([]ValidatedMemoryChange{
		ValidatedIdentityChange{change: admit},
		ValidatedIdentityChange{change: supersede},
		ValidatedRetraction{change: retraction},
	})
	changeSetBytes, err := changeSet.CanonicalBytes()
	if err != nil {
		t.Fatalf("ValidatedMemoryChangeSet.CanonicalBytes(): %v", err)
	}

	admitBytes := assertErrorCanonicalMaterialization(
		t,
		admit,
		mustCanonicalIdentityChange(t, admit),
	)
	assertContainsFramedCanonicalValue(t, changeSetBytes, admitBytes)

	supersedeBytes := assertErrorCanonicalMaterialization(
		t,
		supersede,
		mustCanonicalIdentityChange(t, supersede),
	)
	assertContainsFramedCanonicalValue(t, changeSetBytes, supersedeBytes)

	retractionBytes := assertErrorCanonicalMaterialization(
		t,
		retraction,
		mustCanonicalMemoryChange(t, retraction),
	)
	assertContainsFramedCanonicalValue(t, changeSetBytes, retractionBytes)
}

func assertErrorCanonicalMaterialization(
	t *testing.T,
	value errorCanonicalMaterializer,
	want []byte,
) []byte {
	t.Helper()
	got, err := value.CanonicalBytes()
	if err != nil {
		t.Fatalf("CanonicalBytes(): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("CanonicalBytes accessor differs from the existing canonical encoder")
	}
	digest, err := value.Digest()
	if err != nil {
		t.Fatalf("Digest(): %v", err)
	}
	if digest != digestCanonicalBytes(got) {
		t.Fatal("Digest does not hash the exact CanonicalBytes result")
	}
	return got
}

func assertContainsFramedCanonicalValue(t *testing.T, parent []byte, child []byte) {
	t.Helper()
	framed := make([]byte, 8, 8+len(child))
	binary.BigEndian.PutUint64(framed, uint64(len(child)))
	framed = append(framed, child...)
	if !bytes.Contains(parent, framed) {
		t.Fatal("parent canonical envelope does not embed the exact length-framed child bytes")
	}
}

func mustCanonicalRelationInstance(t *testing.T, relation RelationInstance) []byte {
	t.Helper()
	canonical, err := canonicalRelationInstance(relation)
	if err != nil {
		t.Fatalf("canonicalRelationInstance(): %v", err)
	}
	return canonical
}

func mustCanonicalIdentityChange(t *testing.T, change IdentityChange) []byte {
	t.Helper()
	canonical, err := canonicalIdentityChange(change)
	if err != nil {
		t.Fatalf("canonicalIdentityChange(): %v", err)
	}
	return canonical
}

func mustCanonicalMemoryChange(t *testing.T, change MemoryChange) []byte {
	t.Helper()
	canonical, err := canonicalMemoryChange(change)
	if err != nil {
		t.Fatalf("canonicalMemoryChange(): %v", err)
	}
	return canonical
}
