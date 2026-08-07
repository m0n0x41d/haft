package typedmemory

import (
	"bytes"
	"testing"
)

func TestRelationInstantiationRequiresContextSliceAndDerivesContextFromIt(t *testing.T) {
	fixture := newRelationContextSliceFixture(t)
	_, err := NewRelationInstantiation(
		fixture.assertion,
		fixture.signature,
		ContextSlice{},
		[]CandidateSlotBinding{fixture.candidateBinding},
		fixture.provenance,
	)
	if err == nil {
		t.Fatal("NewRelationInstantiation() accepted a missing ContextSlice")
	}

	slice := mustContextSliceBuild(t, ContextSliceInput{
		Context:   fixture.context,
		GammaTime: mustContextSlicePoint(t, "2026-07-16T08:00:00Z"),
	})
	relation := fixture.candidate(t, slice)
	if relation.Context() != relation.Slice().Context() {
		t.Fatal("RelationInstantiation.Context() was not derived from Slice()")
	}
	instance := fixture.instance(t, relation)
	if instance.Context() != instance.Slice().Context() {
		t.Fatal("RelationInstance.Context() was not derived from Slice()")
	}
}

func TestRelationCanonicalDigestsCommitToEntireContextSlice(t *testing.T) {
	fixture := newRelationContextSliceFixture(t)
	basePoint := mustContextSlicePoint(t, "2026-07-16T08:00:00Z")
	base := mustContextSliceBuild(t, ContextSliceInput{
		Context:              fixture.context,
		StandardPins:         []StandardPin{mustContextSliceStandardPin(t, "standard:api", "v1", "api-v1")},
		EnvironmentSelectors: []EnvironmentSelector{mustContextSliceEnvironment(t, "platform", "linux", "platform-linux")},
		GammaTime:            basePoint,
	})
	variants := map[string]ContextSlice{
		"standard edition": mustContextSliceBuild(t, ContextSliceInput{
			Context:              fixture.context,
			StandardPins:         []StandardPin{mustContextSliceStandardPin(t, "standard:api", "v2", "api-v2")},
			EnvironmentSelectors: []EnvironmentSelector{mustContextSliceEnvironment(t, "platform", "linux", "platform-linux")},
			GammaTime:            basePoint,
		}),
		"environment selector": mustContextSliceBuild(t, ContextSliceInput{
			Context:              fixture.context,
			StandardPins:         []StandardPin{mustContextSliceStandardPin(t, "standard:api", "v1", "api-v1")},
			EnvironmentSelectors: []EnvironmentSelector{mustContextSliceEnvironment(t, "platform", "darwin", "platform-darwin")},
			GammaTime:            basePoint,
		}),
		"Gamma_time": mustContextSliceBuild(t, ContextSliceInput{
			Context:              fixture.context,
			StandardPins:         []StandardPin{mustContextSliceStandardPin(t, "standard:api", "v1", "api-v1")},
			EnvironmentSelectors: []EnvironmentSelector{mustContextSliceEnvironment(t, "platform", "linux", "platform-linux")},
			GammaTime:            mustContextSlicePoint(t, "2026-07-16T08:00:01Z"),
		}),
	}

	baseCandidate := fixture.candidate(t, base)
	baseCandidateDigest := relationCandidateDigest(t, baseCandidate)
	baseAdmittedBytes := relationAdmittedBytes(t, fixture.instance(t, baseCandidate))
	for name, slice := range variants {
		t.Run(name, func(t *testing.T) {
			candidate := fixture.candidate(t, slice)
			if relationCandidateDigest(t, candidate) == baseCandidateDigest {
				t.Fatal("ContextSlice change did not change candidate relation digest")
			}
			instance := fixture.instance(t, candidate)
			if relationAdmittedBytes(t, instance) == baseAdmittedBytes {
				t.Fatal("ContextSlice change did not change admitted relation digest")
			}
		})
	}
}

func TestRelationCanonicalDomainsWereBumpedForContextSlice(t *testing.T) {
	fixture := newRelationContextSliceFixture(t)
	slice := mustContextSliceBuild(t, ContextSliceInput{
		Context:   fixture.context,
		GammaTime: mustContextSlicePoint(t, "2026-07-16T08:00:00Z"),
	})
	candidate := fixture.candidate(t, slice)
	candidateBytes, err := canonicalRelationInstantiation(candidate)
	if err != nil {
		t.Fatalf("canonicalRelationInstantiation() error = %v", err)
	}
	if !bytes.Contains(candidateBytes, []byte("relation-instantiation.v2")) {
		t.Fatal("candidate relation did not use the ContextSlice domain version")
	}
	instance := fixture.instance(t, candidate)
	instanceBytes, err := canonicalRelationInstance(instance)
	if err != nil {
		t.Fatalf("canonicalRelationInstance() error = %v", err)
	}
	if !bytes.Contains(instanceBytes, []byte("validated-relation-instance.v2")) {
		t.Fatal("admitted relation did not use the ContextSlice domain version")
	}
	if !bytes.Contains(candidateBytes, []byte(slice.Ref().String())) ||
		!bytes.Contains(candidateBytes, slice.CanonicalBytes()) {
		t.Fatal("candidate relation bytes omitted ContextSlice ref or canonical bytes")
	}
	if !bytes.Contains(instanceBytes, []byte(slice.Ref().String())) ||
		!bytes.Contains(instanceBytes, slice.CanonicalBytes()) {
		t.Fatal("admitted relation bytes omitted ContextSlice ref or canonical bytes")
	}
	filler := instance.Bindings()[0].Fillers()[0].(ReferenceFiller)
	if filler.Entity() != fixture.entity {
		t.Fatal("admitted reference filler lost its stable EntityID")
	}
	if !bytes.Contains(instanceBytes, []byte(fixture.entity.String())) {
		t.Fatal("admitted relation bytes omitted stable EntityID")
	}
	otherEntity, err := NewEntityID("entity:other-resolution")
	if err != nil {
		t.Fatalf("NewEntityID() error = %v", err)
	}
	otherBinding := newSlotBinding(
		fixture.candidateBinding.Name(),
		[]SlotFiller{newReferenceFiller(fixture.persisted, otherEntity)},
	)
	otherInstance := newRelationInstance(candidate, []SlotBinding{otherBinding})
	otherBytes, err := canonicalRelationInstance(otherInstance)
	if err != nil {
		t.Fatalf("canonicalRelationInstance(other entity) error = %v", err)
	}
	if bytes.Equal(instanceBytes, otherBytes) {
		t.Fatal("stable EntityID change did not change admitted relation bytes")
	}
}

type relationContextSliceFixture struct {
	assertion        AssertionID
	signature        RelationSignatureRef
	context          BoundedContextRef
	candidateBinding CandidateSlotBinding
	persisted        PersistedRef
	entity           EntityID
	provenance       ProvenanceRef
}

func newRelationContextSliceFixture(t *testing.T) relationContextSliceFixture {
	t.Helper()
	digest := mustContextSliceDigest(t, "relation-context-slice-typeenv")
	typeEnv, err := NewTypeEnvRef(digest)
	if err != nil {
		t.Fatalf("NewTypeEnvRef() error = %v", err)
	}
	signatureID, err := NewSignatureID("Local.Relation")
	if err != nil {
		t.Fatalf("NewSignatureID() error = %v", err)
	}
	signature, err := NewRelationSignatureRef(typeEnv, signatureID)
	if err != nil {
		t.Fatalf("NewRelationSignatureRef() error = %v", err)
	}
	refKindID, err := NewRefKindID("U.EntityRef")
	if err != nil {
		t.Fatalf("NewRefKindID() error = %v", err)
	}
	refKind, err := NewRefKindRef(typeEnv, refKindID)
	if err != nil {
		t.Fatalf("NewRefKindRef() error = %v", err)
	}
	referenceID, err := NewReferenceID("entity:relation-context-slice")
	if err != nil {
		t.Fatalf("NewReferenceID() error = %v", err)
	}
	persisted, err := NewPersistedRef(refKind, referenceID)
	if err != nil {
		t.Fatalf("NewPersistedRef() error = %v", err)
	}
	entity, err := NewEntityID("entity:relation-context-slice")
	if err != nil {
		t.Fatalf("NewEntityID() error = %v", err)
	}
	filler, err := NewByReferenceCandidate(persisted)
	if err != nil {
		t.Fatalf("NewByReferenceCandidate() error = %v", err)
	}
	slot, err := NewSlotKindID("Local.EntitySlot")
	if err != nil {
		t.Fatalf("NewSlotKindID() error = %v", err)
	}
	binding, err := NewCandidateSlotBinding(slot, []CandidateSlotFiller{filler})
	if err != nil {
		t.Fatalf("NewCandidateSlotBinding() error = %v", err)
	}
	assertion, err := NewAssertionID("assertion:relation-context-slice")
	if err != nil {
		t.Fatalf("NewAssertionID() error = %v", err)
	}
	context, err := NewBoundedContextRef("context:relation-context-slice")
	if err != nil {
		t.Fatalf("NewBoundedContextRef() error = %v", err)
	}
	provenance, err := NewProvenanceRef("memory:relation-context-slice")
	if err != nil {
		t.Fatalf("NewProvenanceRef() error = %v", err)
	}
	return relationContextSliceFixture{
		assertion:        assertion,
		signature:        signature,
		context:          context,
		candidateBinding: binding,
		persisted:        persisted,
		entity:           entity,
		provenance:       provenance,
	}
}

func (fixture relationContextSliceFixture) candidate(
	t *testing.T,
	slice ContextSlice,
) RelationInstantiation {
	t.Helper()
	relation, err := NewRelationInstantiation(
		fixture.assertion,
		fixture.signature,
		slice,
		[]CandidateSlotBinding{fixture.candidateBinding},
		fixture.provenance,
	)
	if err != nil {
		t.Fatalf("NewRelationInstantiation() error = %v", err)
	}
	return relation
}

func (fixture relationContextSliceFixture) instance(
	t *testing.T,
	candidate RelationInstantiation,
) RelationInstance {
	t.Helper()
	binding := newSlotBinding(
		fixture.candidateBinding.Name(),
		[]SlotFiller{newReferenceFiller(fixture.persisted, fixture.entity)},
	)
	instance := newRelationInstance(candidate, []SlotBinding{binding})
	if !instance.valid() {
		t.Fatal("newRelationInstance() produced an invalid relation")
	}
	return instance
}

func relationCandidateDigest(t *testing.T, relation RelationInstantiation) SHA256Digest {
	t.Helper()
	change, err := NewInstantiateRelation(relation)
	if err != nil {
		t.Fatalf("NewInstantiateRelation() error = %v", err)
	}
	changeSet, err := NewMemoryChangeSet([]MemoryChange{change})
	if err != nil {
		t.Fatalf("NewMemoryChangeSet() error = %v", err)
	}
	digest, err := changeSet.Digest()
	if err != nil {
		t.Fatalf("MemoryChangeSet.Digest() error = %v", err)
	}
	return digest
}

func relationAdmittedBytes(t *testing.T, relation RelationInstance) string {
	t.Helper()
	encoded, err := canonicalRelationInstance(relation)
	if err != nil {
		t.Fatalf("canonicalRelationInstance() error = %v", err)
	}
	return string(encoded)
}
