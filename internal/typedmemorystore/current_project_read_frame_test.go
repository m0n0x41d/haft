package typedmemorystore

import (
	"bytes"
	"context"
	"testing"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestCurrentProjectReadFrameLoadsOneCorrelatedReadOnlySnapshot(
	t *testing.T,
) {
	fixture := newGenericMixedStoreFixture(t)
	loader := staticTypeEnvLoader{
		reference:   fixture.environment.Ref(),
		environment: fixture.environment,
		registry:    fixture.registry,
	}
	reader, err := NewSQLiteCurrentProjectReadFrameLoader(
		fixture.base.database,
		loader,
	)
	if err != nil {
		t.Fatalf("NewSQLiteCurrentProjectReadFrameLoader: %v", err)
	}
	if _, writable := reader.(CommitPort); writable {
		t.Fatal("current project read frame loader exposes CommitPort")
	}
	if _, writable := reader.(SnapshotPort); writable {
		t.Fatal("current project read frame loader exposes SnapshotPort")
	}

	beforeEvents := countStoredGraphEvents(t, fixture)
	frame, err := reader.LoadCurrentProjectReadFrame(
		context.Background(),
		fixture.base.project,
	)
	if err != nil {
		t.Fatalf("LoadCurrentProjectReadFrame: %v", err)
	}
	afterEvents := countStoredGraphEvents(t, fixture)
	if afterEvents != beforeEvents {
		t.Fatalf(
			"read frame changed graph event count from %d to %d",
			beforeEvents,
			afterEvents,
		)
	}

	snapshot := frame.Snapshot()
	directory := frame.EntityDirectory()
	graph := frame.GraphObservation()
	if snapshot.ProjectID() != fixture.base.project ||
		directory.ProjectID() != fixture.base.project ||
		graph.GraphSnapshotBasis().Project() != fixture.base.project {
		t.Fatal("read frame lost the selected project identity")
	}
	if snapshot.Snapshot().GraphRevision().Value() != 2 ||
		directory.GraphSnapshotBasis().GraphRevision().Value() != 2 ||
		graph.GraphSnapshotBasis().GraphRevision().Value() != 2 {
		t.Fatal("read frame components do not share graph revision 2")
	}
	if snapshot.Environment().Ref() != fixture.environment.Ref() ||
		directory.ActiveTypeEnv() != fixture.environment.Ref() ||
		graph.ActiveTypeEnv() != fixture.environment.Ref() {
		t.Fatal("read frame components do not share the active TypeEnv")
	}

	entries := directory.Entries()
	if len(entries) != 1 {
		t.Fatalf("directory entries = %d; want 1", len(entries))
	}
	entry := entries[0]
	aliases := entry.Aliases()
	if entry.Entity() != fixture.anchor ||
		entry.Context() != fixture.primary ||
		entry.Label().String() != "Anchor" ||
		entry.Provenance().String() != "memory:test:anchor" ||
		entry.DeclaredRevision().Value() != 1 ||
		len(aliases) != 1 ||
		aliases[0] != fixture.oldAlias {
		t.Fatal("directory did not preserve exact entity declaration coordinates")
	}
	relations := graph.ActiveAssertions().Relations()
	if len(relations) != 1 ||
		relations[0].AssertionID() != fixture.oldAssertion {
		t.Fatal("read frame did not preserve the active relation")
	}
}

func TestCurrentEntityDirectoryIsCanonicalAndDefensivelyOwned(t *testing.T) {
	fixture := newGenericMixedStoreFixture(t)
	frame := loadGenericCurrentProjectReadFrame(t, fixture)
	directory := frame.EntityDirectory()
	entry := directory.Entries()[0]

	copyEntries := directory.Entries()
	copyEntries[0] = CurrentEntityDirectoryEntry{}
	copyCanonical := directory.CanonicalBytes()
	copyCanonical[0] ^= 0xff
	copyAliases := entry.Aliases()
	copyAliases[0] = typedmemory.EntityAlias{}

	reloaded := directory.Entries()[0]
	if reloaded.Entity() != fixture.anchor ||
		reloaded.Aliases()[0] != fixture.oldAlias ||
		bytes.Equal(copyCanonical, directory.CanonicalBytes()) {
		t.Fatal("current entity directory leaked mutable backing storage")
	}
	if err := directory.Verify(); err != nil {
		t.Fatalf("CurrentEntityDirectory.Verify: %v", err)
	}
}

func TestCurrentEntityDirectoryRejectsDuplicateEntityContext(t *testing.T) {
	fixture := newGenericMixedStoreFixture(t)
	frame := loadGenericCurrentProjectReadFrame(t, fixture)
	directory := frame.EntityDirectory()
	entry := directory.Entries()[0]

	_, err := NewCurrentEntityDirectory(
		directory.ProjectID(),
		directory.GraphSnapshotBasis(),
		directory.ActiveTypeEnv(),
		[]CurrentEntityDirectoryEntry{entry, entry},
	)
	if err == nil {
		t.Fatal("duplicate entity/context was accepted")
	}
}

func TestCurrentEntityDirectoryRejectsAmbiguousAliasInOneContext(t *testing.T) {
	fixture := newGenericMixedStoreFixture(t)
	frame := loadGenericCurrentProjectReadFrame(t, fixture)
	directory := frame.EntityDirectory()
	entry := directory.Entries()[0]
	otherEntity := mustGenericEntityID(t, "entity:other")
	other, err := NewCurrentEntityDirectoryEntry(
		CurrentEntityDirectoryEntryInput{
			Entity:           otherEntity,
			Context:          entry.Context(),
			Label:            mustEntityLabel(t, "Other"),
			Provenance:       entry.Provenance(),
			DeclaredEvent:    entry.DeclaredEvent(),
			DeclaredRevision: entry.DeclaredRevision(),
			Aliases:          entry.Aliases(),
		},
	)
	if err != nil {
		t.Fatalf("NewCurrentEntityDirectoryEntry(other): %v", err)
	}

	_, err = NewCurrentEntityDirectory(
		directory.ProjectID(),
		directory.GraphSnapshotBasis(),
		directory.ActiveTypeEnv(),
		[]CurrentEntityDirectoryEntry{entry, other},
	)
	if err == nil {
		t.Fatal("ambiguous active alias in one context was accepted")
	}
}

func TestCurrentProjectReadFrameRejectsComponentsFromDifferentRevisions(
	t *testing.T,
) {
	fixture := newGenericMixedStoreFixture(t)
	before := loadGenericCurrentProjectReadFrame(t, fixture)
	candidate := fixture.finalCandidate(
		t,
		"Replacement entity",
		"replacement payload",
	)
	request := fixture.finalRequest(
		t,
		"read-frame-correlation",
		candidate,
	)
	if _, err := fixture.adapter.CommitMemoryChangeSet(
		context.Background(),
		request,
	); err != nil {
		t.Fatalf("CommitMemoryChangeSet: %v", err)
	}
	after := loadGenericCurrentProjectReadFrame(t, fixture)

	_, err := NewCurrentProjectReadFrame(
		before.Snapshot(),
		before.EntityDirectory(),
		after.GraphObservation(),
	)
	if err == nil {
		t.Fatal("uncorrelated read-frame components were accepted")
	}
}

func loadGenericCurrentProjectReadFrame(
	t *testing.T,
	fixture genericMixedStoreFixture,
) CurrentProjectReadFrame {
	t.Helper()
	loader := staticTypeEnvLoader{
		reference:   fixture.environment.Ref(),
		environment: fixture.environment,
		registry:    fixture.registry,
	}
	reader, err := NewSQLiteCurrentProjectReadFrameLoader(
		fixture.base.database,
		loader,
	)
	if err != nil {
		t.Fatalf("NewSQLiteCurrentProjectReadFrameLoader: %v", err)
	}
	frame, err := reader.LoadCurrentProjectReadFrame(
		context.Background(),
		fixture.base.project,
	)
	if err != nil {
		t.Fatalf("LoadCurrentProjectReadFrame: %v", err)
	}
	return frame
}

func countStoredGraphEvents(
	t *testing.T,
	fixture genericMixedStoreFixture,
) int64 {
	t.Helper()
	var count int64
	err := fixture.base.database.QueryRow(
		`SELECT COUNT(*) FROM typed_memory_graph_events
		WHERE project_id = ?`,
		fixture.base.project.String(),
	).Scan(&count)
	if err != nil {
		t.Fatalf("count graph events: %v", err)
	}
	return count
}

func mustEntityLabel(t *testing.T, raw string) typedmemory.EntityLabel {
	t.Helper()
	label, err := typedmemory.NewEntityLabel(raw)
	if err != nil {
		t.Fatalf("NewEntityLabel(%q): %v", raw, err)
	}
	return label
}
