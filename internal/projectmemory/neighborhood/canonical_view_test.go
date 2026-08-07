package neighborhood

import (
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestMemoryViewContextPayloadPublishesComposableEntityRef(t *testing.T) {
	t.Parallel()

	digest, err := typedmemory.NewSHA256Digest(
		"sha256:" + strings.Repeat("a", 64),
	)
	if err != nil {
		t.Fatal(err)
	}
	typeEnv, err := typedmemory.NewTypeEnvRef(digest)
	if err != nil {
		t.Fatal(err)
	}
	refKindID, err := typedmemory.NewRefKindID("U.EntityRef")
	if err != nil {
		t.Fatal(err)
	}
	refKind, err := typedmemory.NewRefKindRef(typeEnv, refKindID)
	if err != nil {
		t.Fatal(err)
	}
	referenceID, err := typedmemory.NewReferenceID("service:auth")
	if err != nil {
		t.Fatal(err)
	}
	entity, err := typedmemory.NewPersistedRef(refKind, referenceID)
	if err != nil {
		t.Fatal(err)
	}
	context, err := typedmemory.NewBoundedContextRef("context:project")
	if err != nil {
		t.Fatal(err)
	}
	profile, err := ParseProjectionProfileRef("agent_orientation.v2")
	if err != nil {
		t.Fatal(err)
	}
	view, err := NewMemoryViewContext(entity, context, profile)
	if err != nil {
		t.Fatal(err)
	}

	payload := memoryViewContextPayload(view)
	entityRef, ok := payload["entity_ref"].(map[string]any)
	if !ok {
		t.Fatalf("entity_ref = %#v", payload["entity_ref"])
	}
	if entityRef["ref_kind_id"] != "U.EntityRef" ||
		entityRef["reference_id"] != "service:auth" {
		t.Fatalf("entity_ref = %#v", entityRef)
	}
	for _, legacy := range []string{
		"entity_reference_kind",
		"entity_reference_id",
	} {
		if _, exists := payload[legacy]; exists {
			t.Fatalf("memory_view_context leaked legacy key %q", legacy)
		}
	}
}
