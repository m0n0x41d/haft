package typedmemory

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestMemoryChangeSetCanonicalBytesMatchCandidateDigest(t *testing.T) {
	entity, err := NewEntityID("entity:canonical-request")
	if err != nil {
		t.Fatalf("entity: %v", err)
	}
	localRef, err := NewBatchLocalRef("canonical-request")
	if err != nil {
		t.Fatalf("local ref: %v", err)
	}
	contextRef, err := NewBoundedContextRef("context:canonical-request")
	if err != nil {
		t.Fatalf("context: %v", err)
	}
	label, err := NewEntityLabel("Canonical request")
	if err != nil {
		t.Fatalf("label: %v", err)
	}
	provenance, err := NewProvenanceRef("provenance:canonical-request")
	if err != nil {
		t.Fatalf("provenance: %v", err)
	}
	declaration, err := NewDeclareEntity(entity, localRef, contextRef, label, provenance)
	if err != nil {
		t.Fatalf("declaration: %v", err)
	}
	changeSet, err := NewMemoryChangeSet([]MemoryChange{declaration})
	if err != nil {
		t.Fatalf("change set: %v", err)
	}

	canonical, err := changeSet.CanonicalBytes()
	if err != nil {
		t.Fatalf("canonical bytes: %v", err)
	}
	digest, err := changeSet.Digest()
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	sum := sha256.Sum256(canonical)
	want, err := NewSHA256Digest("sha256:" + hex.EncodeToString(sum[:]))
	if err != nil {
		t.Fatalf("expected digest: %v", err)
	}
	if digest != want {
		t.Fatalf("digest = %s, hash(canonical) = %s", digest.String(), want.String())
	}

	copyBytes := changeSetCanonicalCopy(canonical)
	copyBytes[0] ^= 0xff
	replayed, err := changeSet.CanonicalBytes()
	if err != nil {
		t.Fatalf("replayed canonical bytes: %v", err)
	}
	if replayed[0] == copyBytes[0] {
		t.Fatal("MemoryChangeSet canonical bytes leaked mutable storage")
	}
}

func changeSetCanonicalCopy(value []byte) []byte {
	return append([]byte(nil), value...)
}
