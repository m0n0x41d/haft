package projectmemory

import (
	"slices"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectmemory/neighborhood"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestCurrentNeighborhoodClassifiesNoteAtConcernRecordExactly(t *testing.T) {
	roles := currentNeighborhoodRoles(
		"Haft.NoteAtConcern",
		"Haft.NoteAtConcern.NoteSlot",
		"Haft.ProjectRecordRef",
	)

	if !slices.Equal(roles, []neighborhood.ItemKind{
		neighborhood.ItemNoteRecord,
	}) {
		t.Fatalf("NoteAtConcern record roles = %v", roles)
	}

	otherSlot := currentNeighborhoodRoles(
		"Haft.NoteAtConcern",
		"Haft.NoteAtConcern.EntityOfConcernSlot",
		"Haft.ProjectRecordRef",
	)
	if len(otherSlot) != 0 {
		t.Fatalf("non-note ProjectRecord slot roles = %v", otherSlot)
	}
}

func TestCurrentNeighborhoodTraversalKeyBridgesOnlyStableRefKindIdentity(
	t *testing.T,
) {
	priorEntity := currentNeighborhoodTestReference(
		t,
		"sha256:1111111111111111111111111111111111111111111111111111111111111111",
		"U.EntityRef",
		"entity:haft-v9-typed-memory",
	)
	currentEntity := currentNeighborhoodTestReference(
		t,
		"sha256:2222222222222222222222222222222222222222222222222222222222222222",
		"U.EntityRef",
		"entity:haft-v9-typed-memory",
	)
	otherKind := currentNeighborhoodTestReference(
		t,
		"sha256:2222222222222222222222222222222222222222222222222222222222222222",
		"Haft.ProjectRecordRef",
		"entity:haft-v9-typed-memory",
	)

	priorTraversal := currentNeighborhoodTraversalKey(priorEntity)
	currentTraversal := currentNeighborhoodTraversalKey(currentEntity)
	if priorTraversal != currentTraversal {
		t.Fatal("stable RefKind identity split across TypeEnv editions")
	}
	if priorTraversal == currentNeighborhoodTraversalKey(otherKind) {
		t.Fatal("traversal key collapsed distinct RefKind identities")
	}
	if currentPersistedReferenceKey(priorEntity) ==
		currentPersistedReferenceKey(currentEntity) {
		t.Fatal("exact persisted coordinates lost their TypeEnv editions")
	}
	projected, err := currentNeighborhoodProjectionReference(
		priorEntity,
		currentEntity.RefKind().TypeEnv(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if currentPersistedReferenceKey(projected) !=
		currentPersistedReferenceKey(currentEntity) {
		t.Fatal("legacy output reference was not projected into the selected TypeEnv")
	}
	if currentPersistedReferenceKey(priorEntity) ==
		currentPersistedReferenceKey(projected) {
		t.Fatal("legacy witness coordinate was rewritten in place")
	}
}

func currentNeighborhoodTestReference(
	t *testing.T,
	typeEnvDigest string,
	refKindIDValue string,
	referenceIDValue string,
) typedmemory.PersistedRef {
	t.Helper()
	digest, err := typedmemory.NewSHA256Digest(typeEnvDigest)
	if err != nil {
		t.Fatal(err)
	}
	typeEnv, err := typedmemory.NewTypeEnvRef(digest)
	if err != nil {
		t.Fatal(err)
	}
	refKindID, err := typedmemory.NewRefKindID(refKindIDValue)
	if err != nil {
		t.Fatal(err)
	}
	refKind, err := typedmemory.NewRefKindRef(typeEnv, refKindID)
	if err != nil {
		t.Fatal(err)
	}
	referenceID, err := typedmemory.NewReferenceID(referenceIDValue)
	if err != nil {
		t.Fatal(err)
	}
	reference, err := typedmemory.NewPersistedRef(refKind, referenceID)
	if err != nil {
		t.Fatal(err)
	}
	return reference
}
