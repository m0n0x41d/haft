package memberofevaluation

import (
	"bytes"
	"slices"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestObservableInputBlobIsContentAddressedAndDefensive(t *testing.T) {
	content := []byte("exact observable bytes")
	reference := mustObservableRef(t, "observable:test:exact")
	digest := mustDigest(t, content)
	blob := mustObservableBlob(t, reference, digest, content)
	content[0] ^= 0xff

	if !blob.Valid() || string(blob.Bytes()) != "exact observable bytes" {
		t.Fatalf("blob lost exact immutable bytes")
	}
	copy := blob.Bytes()
	copy[0] ^= 0xff
	if bytes.Equal(copy, blob.Bytes()) {
		t.Fatalf("Bytes leaked mutable backing storage")
	}
}

func TestExactPersistedEntityUniverseKeepsCanonicalEmptyDistinctFromUnavailable(
	t *testing.T,
) {
	project := mustProject(t, "qnt_1234abcd")
	contextRef := mustContext(t, "context:member-of-evaluation")
	second := mustEntity(t, "entity:second")
	first := mustEntity(t, "entity:first")
	input := []typedmemory.EntityID{second, first}
	universe := mustUniverse(
		t,
		project,
		contextRef,
		typedmemory.NewGraphRevision(7),
		input,
	)
	input[0] = first

	if !universe.Valid() || !slices.Equal(
		universe.Members(),
		[]typedmemory.EntityID{first, second},
	) {
		t.Fatalf("exact universe is not canonical")
	}
	wantCanonical := `{"schema_version":"haft.typed-memory.persisted-entity-universe/v1","project_id":"qnt_1234abcd","bounded_context":"context:member-of-evaluation","graph_revision":7,"entity_ids":["entity:first","entity:second"]}`
	if string(universe.CanonicalBytes()) != wantCanonical {
		t.Fatalf("canonical bytes changed:\n%s", universe.CanonicalBytes())
	}
	blobValue, blobErr := universe.ObservableBlob()
	blob := requireValue(t, blobValue, blobErr)
	if !blob.Valid() || !bytes.Equal(blob.Bytes(), universe.CanonicalBytes()) {
		t.Fatalf("universe observable blob lost exact canonical content")
	}

	empty := mustUniverse(
		t,
		project,
		contextRef,
		typedmemory.NewGraphRevision(7),
		nil,
	)
	var exact PersistedEntityUniverse = empty
	var unavailable PersistedEntityUniverse = NewPersistedEntityUniverseUnavailable()
	if _, ok := exact.(ExactPersistedEntityUniverse); !ok || len(empty.Members()) != 0 {
		t.Fatalf("empty exact universe was not retained")
	}
	if _, ok := unavailable.(PersistedEntityUniverseUnavailable); !ok {
		t.Fatalf("unavailable posture = %T", unavailable)
	}
}

func TestMemberOfEvaluationInputAndSelectionDefensivelyCopyObservables(t *testing.T) {
	project := mustProject(t, "qnt_2345bcde")
	blob := testObservable(t, "observable:test:input", "input bytes")
	blobs := []ObservableInputBlob{blob}
	inputValue, inputErr := NewMemberOfEvaluationInput(
		project,
		typedmemory.TypeEnv{},
		typedmemory.MemberOfEvaluationRequest{},
		blobs,
		NewPersistedEntityUniverseUnavailable(),
	)
	input := requireValue(t, inputValue, inputErr)
	blobs[0] = ObservableInputBlob{}

	if !input.Valid() || len(input.ObservableInputs()) != 1 {
		t.Fatalf("evaluation input lost immutable observable")
	}
	returned := input.ObservableInputs()
	returned[0] = ObservableInputBlob{}
	if !input.ObservableInputs()[0].Valid() {
		t.Fatalf("ObservableInputs leaked mutable backing storage")
	}

	second := testObservable(t, "observable:test:a", "a bytes")
	selectedValue, selectedErr := NewSnapshotObservableInputsSelected(
		[]ObservableInputBlob{blob, second, blob},
	)
	selected := requireValue(t, selectedValue, selectedErr)
	if !selected.Valid() || len(selected.ObservableInputs()) != 2 {
		t.Fatalf("selected exact catalog was not canonicalized")
	}
	selectedCopy := selected.ObservableInputs()
	selectedCopy[0] = ObservableInputBlob{}
	if !selected.ObservableInputs()[0].Valid() {
		t.Fatalf("selected observable inputs leaked mutable backing storage")
	}
}

func TestSnapshotObservableInputSelectionKeepsNotApplicableDistinctFromUnavailable(
	t *testing.T,
) {
	notApplicable := NewSnapshotObservableInputsNotApplicable()
	unavailable := NewSnapshotObservableInputsUnavailable()
	if !notApplicable.Valid() || !unavailable.Valid() {
		t.Fatal("closed snapshot source-selection postures must be valid")
	}
	var absent SnapshotObservableInputSelection = notApplicable
	var missing SnapshotObservableInputSelection = unavailable
	if _, ok := absent.(SnapshotObservableInputsNotApplicable); !ok {
		t.Fatalf("clean source absence = %T, want NotApplicable", absent)
	}
	if _, ok := missing.(SnapshotObservableInputsUnavailable); !ok {
		t.Fatalf("source failure = %T, want Unavailable", missing)
	}
	if _, err := NewSnapshotObservableInputsSelected(nil); err == nil {
		t.Fatal("Selected accepted an empty source set instead of explicit NotApplicable")
	}
}

func TestExactPersistedEntityUniverseRejectsDuplicateIdentity(t *testing.T) {
	project := mustProject(t, "qnt_3456cdef")
	contextRef := mustContext(t, "context:member-of-duplicate")
	entity := mustEntity(t, "entity:duplicate")
	_, err := NewExactPersistedEntityUniverse(
		project,
		contextRef,
		typedmemory.NewGraphRevision(0),
		[]typedmemory.EntityID{entity, entity},
	)
	if err == nil {
		t.Fatalf("duplicate EntityID was accepted")
	}
}

func testObservable(
	t *testing.T,
	referenceRaw string,
	contentRaw string,
) ObservableInputBlob {
	t.Helper()
	content := []byte(contentRaw)
	reference := mustObservableRef(t, referenceRaw)
	digest := mustDigest(t, content)
	return mustObservableBlob(t, reference, digest, content)
}

func mustProject(t *testing.T, raw string) projectledger.ProjectID {
	t.Helper()
	value, err := projectledger.ParseProjectID(raw)
	return requireValue(t, value, err)
}

func mustContext(t *testing.T, raw string) typedmemory.BoundedContextRef {
	t.Helper()
	value, err := typedmemory.NewBoundedContextRef(raw)
	return requireValue(t, value, err)
}

func mustEntity(t *testing.T, raw string) typedmemory.EntityID {
	t.Helper()
	value, err := typedmemory.NewEntityID(raw)
	return requireValue(t, value, err)
}

func mustObservableRef(t *testing.T, raw string) typedmemory.ObservableInputRef {
	t.Helper()
	value, err := typedmemory.NewObservableInputRef(raw)
	return requireValue(t, value, err)
}

func mustDigest(t *testing.T, content []byte) typedmemory.SHA256Digest {
	t.Helper()
	value, err := digestBytes(content)
	return requireValue(t, value, err)
}

func mustObservableBlob(
	t *testing.T,
	reference typedmemory.ObservableInputRef,
	digest typedmemory.SHA256Digest,
	content []byte,
) ObservableInputBlob {
	t.Helper()
	value, err := NewObservableInputBlob(reference, digest, content)
	return requireValue(t, value, err)
}

func mustUniverse(
	t *testing.T,
	project projectledger.ProjectID,
	contextRef typedmemory.BoundedContextRef,
	revision typedmemory.GraphRevision,
	members []typedmemory.EntityID,
) ExactPersistedEntityUniverse {
	t.Helper()
	value, err := NewExactPersistedEntityUniverse(
		project,
		contextRef,
		revision,
		members,
	)
	return requireValue(t, value, err)
}

func requireValue[T any](t *testing.T, value T, err error) T {
	t.Helper()
	if err != nil {
		t.Fatalf("fixture construction: %v", err)
	}
	return value
}
