package legacydualread

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectgraphobservation"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projectmemory/legacyimport"
	"github.com/m0n0x41d/haft/internal/projectmemory/legacyimportsqlite"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
	_ "modernc.org/sqlite"
)

func TestCoalesceRequiresExactBridgeAndKeepsLegacyRelationsUnbound(
	t *testing.T,
) {
	report := legacyReport(t, "exact", []string{
		`INSERT INTO artifacts (id, title, content) VALUES ('legacy-1', 'current carrier', 'artifact body')`,
		`INSERT INTO holons (id, title, content) VALUES ('legacy-1', 'historical carrier', 'different holon body')`,
		`INSERT INTO artifact_links (source_id, target_id, link_type, created_at) VALUES ('legacy-1', 'legacy-2', 'Haft.DecisionChoiceAtConcern', '2026-07-18T00:00:00Z')`,
	})
	directory, graph := typedSources(
		t,
		[]typedEntityFixture{
			{id: "typed-1", label: "Typed One"},
			{id: "typed-2", label: "Typed Two"},
		},
	)
	source := firstCarrierLegacyIdentity(t, report)
	target := firstAssociation(t, report).Target()
	bridges := []IdentityBridge{
		identityBridge(t, source, "typed-1", "a"),
		identityBridge(t, target, "typed-2", "b"),
	}

	view, err := Coalesce(directory, graph, report, bridges)
	if err != nil {
		t.Fatalf("Coalesce() error = %v", err)
	}

	if view.TypedEntityDirectory().Digest() != directory.Digest() {
		t.Fatal("dual-read changed the typed entity directory")
	}
	observedGraph := view.TypedGraphObservation()
	if observedGraph.GraphSnapshotBasis().Ref() !=
		graph.GraphSnapshotBasis().Ref() {
		t.Fatal("dual-read changed the typed graph basis")
	}
	if got := len(observedGraph.ActiveAssertions().Relations()); got != 0 {
		t.Fatalf(
			"legacy label fabricated %d typed current assertions",
			got,
		)
	}
	if got := len(view.LegacyCarriers()); got != 2 {
		t.Fatalf("legacy carrier reads = %d, want artifact + holon", got)
	}
	for _, read := range view.LegacyCarriers() {
		resolution, ok := read.IdentityResolution().(ExactIdentityResolution)
		if !ok {
			t.Fatalf(
				"carrier identity resolution = %T, want exact",
				read.IdentityResolution(),
			)
		}
		if resolution.Target().EntityID().String() != "typed-1" {
			t.Fatalf(
				"carrier target = %q, want typed-1",
				resolution.Target().EntityID().String(),
			)
		}
	}
	if got := len(view.LegacyAssociations()); got != 1 {
		t.Fatalf("legacy association reads = %d, want 1", got)
	}
	association := view.LegacyAssociations()[0]
	if association.SemanticPosture() != SemanticPostureLegacyUnbound {
		t.Fatalf(
			"legacy association posture = %q",
			association.SemanticPosture(),
		)
	}
	if association.Observation().Label().String() ==
		"Haft.DecisionChoiceAtConcern" {
		t.Fatal("raw legacy label escaped its opaque source namespace")
	}
	assertExactTarget(t, association.SourceResolution(), "typed-1")
	assertExactTarget(t, association.TargetResolution(), "typed-2")
	if len(view.Issues()) != 0 {
		t.Fatalf("exact view issues = %v, want none", view.Issues())
	}
	if !bytes.Contains(
		view.CanonicalBytes(),
		[]byte(`"typed_current_assertions":[]`),
	) {
		t.Fatal("canonical view does not preserve the empty typed assertion set")
	}
}

func TestCoalesceExposesIdentityCollisionWithoutChoosingWinner(
	t *testing.T,
) {
	report := legacyReport(t, "collision", []string{
		`INSERT INTO artifacts (id, title, content) VALUES ('legacy-1', 'carrier', 'body')`,
	})
	directory, graph := typedSources(
		t,
		[]typedEntityFixture{
			{id: "typed-1", label: "Typed One"},
			{id: "typed-2", label: "Typed Two"},
		},
	)
	legacy := firstCarrierLegacyIdentity(t, report)
	bridges := []IdentityBridge{
		identityBridge(t, legacy, "typed-1", "a"),
		identityBridge(t, legacy, "typed-2", "b"),
	}

	view, err := Coalesce(directory, graph, report, bridges)
	if err != nil {
		t.Fatalf("Coalesce() error = %v", err)
	}

	if got := len(view.Issues()); got != 1 {
		t.Fatalf("collision issues = %d, want 1", got)
	}
	collision, ok := view.Issues()[0].(IdentityCollisionIssue)
	if !ok {
		t.Fatalf(
			"collision issue = %T, want IdentityCollisionIssue",
			view.Issues()[0],
		)
	}
	if got := len(collision.Candidates()); got != 2 {
		t.Fatalf("collision candidates = %d, want 2", got)
	}
	resolution, ok := view.LegacyCarriers()[0].IdentityResolution().(AmbiguousIdentityResolution)
	if !ok {
		t.Fatalf(
			"colliding carrier resolution = %T, want ambiguous",
			view.LegacyCarriers()[0].IdentityResolution(),
		)
	}
	if got := len(resolution.Candidates()); got != 2 {
		t.Fatalf("ambiguous resolution candidates = %d, want 2", got)
	}
}

func TestCoalesceReportsAbsentBridgeSourceAndTarget(t *testing.T) {
	report := legacyReport(t, "absence", []string{
		`INSERT INTO artifacts (id, title, content) VALUES ('legacy-1', 'carrier', 'body')`,
	})
	directory, graph := typedSources(
		t,
		[]typedEntityFixture{{id: "typed-1", label: "Typed One"}},
	)
	presentLegacy := firstCarrierLegacyIdentity(t, report)
	absentLegacy, err := legacyimport.NewLegacyIdentityRef(
		"legacy-object:absent",
	)
	if err != nil {
		t.Fatalf("NewLegacyIdentityRef() error = %v", err)
	}
	bridges := []IdentityBridge{
		identityBridge(t, absentLegacy, "typed-1", "a"),
		identityBridge(t, presentLegacy, "typed-absent", "b"),
	}

	view, err := Coalesce(directory, graph, report, bridges)
	if err != nil {
		t.Fatalf("Coalesce() error = %v", err)
	}

	issueKinds := make(map[IssueKind]int)
	for _, issue := range view.Issues() {
		issueKinds[issue.Kind()]++
	}
	if issueKinds[IssueBridgeSourceAbsent] != 1 {
		t.Fatalf("source-absent issues = %d, want 1", issueKinds[IssueBridgeSourceAbsent])
	}
	if issueKinds[IssueBridgeTargetAbsent] != 1 {
		t.Fatalf("target-absent issues = %d, want 1", issueKinds[IssueBridgeTargetAbsent])
	}
	resolution := view.LegacyCarriers()[0].IdentityResolution()
	if resolution.Kind() != ResolutionUnbound {
		t.Fatalf(
			"carrier with absent typed target resolution = %q, want unbound",
			resolution.Kind(),
		)
	}
}

func TestCoalesceIsPermutationStableAcrossRowsAndBridges(t *testing.T) {
	firstReport := legacyReport(t, "permutation-first", []string{
		`INSERT INTO artifacts (id, title, content) VALUES ('legacy-1', 'carrier', 'artifact')`,
		`INSERT INTO holons (id, title, content) VALUES ('legacy-1', 'historical', 'holon')`,
		`INSERT INTO artifact_links (source_id, target_id, link_type, created_at) VALUES ('legacy-1', 'legacy-2', 'informs', '2026-07-18T00:00:00Z')`,
	})
	secondReport := legacyReport(t, "permutation-second", []string{
		`INSERT INTO artifact_links (source_id, target_id, link_type, created_at) VALUES ('legacy-1', 'legacy-2', 'informs', '2026-07-18T00:00:00Z')`,
		`INSERT INTO holons (id, title, content) VALUES ('legacy-1', 'historical', 'holon')`,
		`INSERT INTO artifacts (id, title, content) VALUES ('legacy-1', 'carrier', 'artifact')`,
	})
	directory, graph := typedSources(
		t,
		[]typedEntityFixture{
			{id: "typed-1", label: "Typed One"},
			{id: "typed-2", label: "Typed Two"},
		},
	)
	source := firstCarrierLegacyIdentity(t, firstReport)
	target := firstAssociation(t, firstReport).Target()
	firstBridge := identityBridge(t, source, "typed-1", "a")
	secondBridge := identityBridge(t, source, "typed-1", "b")
	targetBridge := identityBridge(t, target, "typed-2", "c")

	first, err := Coalesce(
		directory,
		graph,
		firstReport,
		[]IdentityBridge{firstBridge, secondBridge, targetBridge},
	)
	if err != nil {
		t.Fatalf("first Coalesce() error = %v", err)
	}
	second, err := Coalesce(
		directory,
		graph,
		secondReport,
		[]IdentityBridge{
			targetBridge,
			secondBridge,
			firstBridge,
			firstBridge,
		},
	)
	if err != nil {
		t.Fatalf("second Coalesce() error = %v", err)
	}

	if first.Digest() != second.Digest() ||
		!bytes.Equal(first.CanonicalBytes(), second.CanonicalBytes()) {
		t.Fatalf(
			"permutation changed dual-read view\nfirst:  %s\nsecond: %s",
			first.CanonicalBytes(),
			second.CanonicalBytes(),
		)
	}
}

func TestCoalesceRejectsCrossProjectSourcesAndBridges(t *testing.T) {
	report := legacyReport(t, "cross-project", []string{
		`INSERT INTO artifacts (id, title, content) VALUES ('legacy-1', 'carrier', 'body')`,
	})
	directory, graph := typedSources(
		t,
		[]typedEntityFixture{{id: "typed-1", label: "Typed One"}},
	)
	otherProject, err := projectidentity.ParseProjectID("qnt_aabbccdd")
	if err != nil {
		t.Fatalf("ParseProjectID(other) error = %v", err)
	}
	legacy := firstCarrierLegacyIdentity(t, report)
	bridge := identityBridgeForProject(
		t,
		otherProject,
		legacy,
		"typed-1",
		"a",
	)

	_, err = Coalesce(directory, graph, report, []IdentityBridge{bridge})
	if err == nil || !strings.Contains(err.Error(), "another project") {
		t.Fatalf("Coalesce(cross-project bridge) error = %v", err)
	}
}

func assertExactTarget(
	t *testing.T,
	resolution IdentityResolution,
	entity string,
) {
	t.Helper()
	exact, ok := resolution.(ExactIdentityResolution)
	if !ok {
		t.Fatalf("identity resolution = %T, want exact", resolution)
	}
	if exact.Target().EntityID().String() != entity {
		t.Fatalf(
			"identity target = %q, want %q",
			exact.Target().EntityID().String(),
			entity,
		)
	}
}

type typedEntityFixture struct {
	id    string
	label string
}

func typedSources(
	t *testing.T,
	fixtures []typedEntityFixture,
) (
	typedmemorystore.CurrentEntityDirectory,
	typedmemorystore.CurrentProjectGraphObservation,
) {
	t.Helper()
	project := testProjectID(t)
	revision := typedmemory.NewGraphRevision(1)
	event := mustGraphEventRef(t, "a")
	commit := mustGraphCommitRef(t, "b")
	materialization := mustDigest(t, "c")
	closure, err := projecttypeenvselection.NewCommittedProjectGraphClosure(
		projecttypeenvselection.CommittedProjectGraphClosureInput{
			Event:                 event,
			Commit:                commit,
			MaterializationDigest: materialization,
		},
	)
	if err != nil {
		t.Fatalf("NewCommittedProjectGraphClosure() error = %v", err)
	}
	basis, err := projecttypeenvselection.SealProjectGraphSnapshotBasis(
		projecttypeenvselection.ProjectGraphSnapshotBasisInput{
			Project:       project,
			GraphRevision: revision,
			Closure:       closure,
		},
	)
	if err != nil {
		t.Fatalf("SealProjectGraphSnapshotBasis() error = %v", err)
	}
	typeEnv, err := typedmemory.NewTypeEnvRef(mustDigest(t, "d"))
	if err != nil {
		t.Fatalf("NewTypeEnvRef() error = %v", err)
	}
	contextRef := mustContextRef(t)
	entries := make(
		[]typedmemorystore.CurrentEntityDirectoryEntry,
		0,
		len(fixtures),
	)
	for _, fixture := range fixtures {
		entry, entryErr := typedmemorystore.NewCurrentEntityDirectoryEntry(
			typedmemorystore.CurrentEntityDirectoryEntryInput{
				Entity:           mustEntityID(t, fixture.id),
				Context:          contextRef,
				Label:            mustEntityLabel(t, fixture.label),
				Provenance:       mustProvenance(t, "typed:"+fixture.id),
				DeclaredEvent:    event,
				DeclaredRevision: revision,
			},
		)
		if entryErr != nil {
			t.Fatalf("NewCurrentEntityDirectoryEntry() error = %v", entryErr)
		}
		entries = append(entries, entry)
	}
	directory, err := typedmemorystore.NewCurrentEntityDirectory(
		project,
		basis,
		typeEnv,
		entries,
	)
	if err != nil {
		t.Fatalf("NewCurrentEntityDirectory() error = %v", err)
	}
	assertions, err := projectgraphobservation.NewCurrentActiveAssertionSet(
		project,
		revision,
		nil,
	)
	if err != nil {
		t.Fatalf("NewCurrentActiveAssertionSet() error = %v", err)
	}
	graph, err := projectgraphobservation.NewCurrentProjectGraphObservation(
		basis,
		typeEnv,
		assertions,
	)
	if err != nil {
		t.Fatalf("NewCurrentProjectGraphObservation() error = %v", err)
	}
	return directory, graph
}

func identityBridge(
	t *testing.T,
	legacy legacyimport.LegacyIdentityRef,
	entity string,
	suffix string,
) IdentityBridge {
	t.Helper()
	return identityBridgeForProject(
		t,
		testProjectID(t),
		legacy,
		entity,
		suffix,
	)
}

func identityBridgeForProject(
	t *testing.T,
	project projectidentity.ProjectID,
	legacy legacyimport.LegacyIdentityRef,
	entity string,
	suffix string,
) IdentityBridge {
	t.Helper()
	carrierRef, err := typedmemory.NewCarrierRef(
		"legacy-identity-map:" + suffix,
	)
	if err != nil {
		t.Fatalf("NewCarrierRef() error = %v", err)
	}
	edition, err := typedmemory.NewCarrierEdition("edition:" + suffix)
	if err != nil {
		t.Fatalf("NewCarrierEdition() error = %v", err)
	}
	basis, err := NewMappingCarrierBasis(
		carrierRef,
		edition,
		mustDigest(t, suffix),
	)
	if err != nil {
		t.Fatalf("NewMappingCarrierBasis() error = %v", err)
	}
	bridge, err := NewIdentityBridge(IdentityBridgeInput{
		Project: project,
		Legacy:  legacy,
		Entity:  mustEntityID(t, entity),
		Context: mustContextRef(t),
		Basis:   basis,
	})
	if err != nil {
		t.Fatalf("NewIdentityBridge() error = %v", err)
	}
	return bridge
}

func legacyReport(
	t *testing.T,
	name string,
	statements []string,
) legacyimport.DryRunReport {
	t.Helper()
	database, err := sql.Open(
		"sqlite",
		"file:legacydualread-"+name+"?mode=memory&cache=shared",
	)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	database.SetMaxOpenConns(1)
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Errorf("database.Close() error = %v", err)
		}
	})
	schema := []string{
		`CREATE TABLE artifacts (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			content TEXT NOT NULL
		)`,
		`CREATE TABLE artifact_links (
			source_id TEXT NOT NULL,
			target_id TEXT NOT NULL,
			link_type TEXT NOT NULL,
			created_at TEXT NOT NULL,
			PRIMARY KEY (source_id, target_id, link_type)
		)`,
		`CREATE TABLE holons (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			content TEXT NOT NULL
		)`,
		`CREATE TABLE relations (
			source_id TEXT NOT NULL,
			target_id TEXT NOT NULL,
			relation_type TEXT NOT NULL,
			congruence_level INTEGER NOT NULL,
			PRIMARY KEY (source_id, target_id, relation_type)
		)`,
	}
	for _, statement := range append(schema, statements...) {
		if _, err := database.Exec(statement); err != nil {
			t.Fatalf("Exec(%q) error = %v", statement, err)
		}
	}
	loader, err := legacyimportsqlite.NewCoreSnapshotLoader(database)
	if err != nil {
		t.Fatalf("NewCoreSnapshotLoader() error = %v", err)
	}
	report, err := loader.DryRun(context.Background(), testProjectID(t))
	if err != nil {
		t.Fatalf("DryRun() error = %v", err)
	}
	return report
}

func firstCarrierLegacyIdentity(
	t *testing.T,
	report legacyimport.DryRunReport,
) legacyimport.LegacyIdentityRef {
	t.Helper()
	for _, carrier := range report.CarrierCatalog().Snapshots() {
		identified, ok := carrier.LegacyIdentity().(legacyimport.IdentifiedLegacyCarrier)
		if ok {
			return identified.Ref()
		}
	}
	t.Fatal("legacy carrier identity was not found")
	return legacyimport.LegacyIdentityRef{}
}

func firstAssociation(
	t *testing.T,
	report legacyimport.DryRunReport,
) legacyimport.AssociationObservation {
	t.Helper()
	for _, item := range report.Items() {
		for _, observation := range item.Observations() {
			association, ok := observation.(legacyimport.AssociationObservation)
			if ok {
				return association
			}
		}
	}
	t.Fatal("legacy association was not found")
	return legacyimport.AssociationObservation{}
}

func testProjectID(t *testing.T) projectidentity.ProjectID {
	t.Helper()
	project, err := projectidentity.ParseProjectID("qnt_e3149c17")
	if err != nil {
		t.Fatalf("ParseProjectID() error = %v", err)
	}
	return project
}

func mustGraphEventRef(
	t *testing.T,
	character string,
) projecttypeenvselection.GraphEventRef {
	t.Helper()
	ref, err := projecttypeenvselection.ParseGraphEventRef(
		"typed-memory-event:" + strings.Repeat(character, 64),
	)
	if err != nil {
		t.Fatalf("ParseGraphEventRef() error = %v", err)
	}
	return ref
}

func mustGraphCommitRef(
	t *testing.T,
	character string,
) projecttypeenvselection.GraphCommitRef {
	t.Helper()
	ref, err := projecttypeenvselection.ParseGraphCommitRef(
		"typed-memory-commit:" + strings.Repeat(character, 64),
	)
	if err != nil {
		t.Fatalf("ParseGraphCommitRef() error = %v", err)
	}
	return ref
}

func mustDigest(t *testing.T, character string) typedmemory.SHA256Digest {
	t.Helper()
	value := character
	if len(value) != 1 ||
		!strings.Contains("0123456789abcdef", value) {
		value = fmt.Sprintf("%x", []byte(value))[0:1]
	}
	digest, err := typedmemory.NewSHA256Digest(
		"sha256:" + strings.Repeat(value, 64),
	)
	if err != nil {
		t.Fatalf("NewSHA256Digest() error = %v", err)
	}
	return digest
}

func mustContextRef(t *testing.T) typedmemory.BoundedContextRef {
	t.Helper()
	contextRef, err := typedmemory.NewBoundedContextRef("project")
	if err != nil {
		t.Fatalf("NewBoundedContextRef() error = %v", err)
	}
	return contextRef
}

func mustEntityID(t *testing.T, raw string) typedmemory.EntityID {
	t.Helper()
	entity, err := typedmemory.NewEntityID(raw)
	if err != nil {
		t.Fatalf("NewEntityID(%q) error = %v", raw, err)
	}
	return entity
}

func mustEntityLabel(t *testing.T, raw string) typedmemory.EntityLabel {
	t.Helper()
	label, err := typedmemory.NewEntityLabel(raw)
	if err != nil {
		t.Fatalf("NewEntityLabel(%q) error = %v", raw, err)
	}
	return label
}

func mustProvenance(t *testing.T, raw string) typedmemory.ProvenanceRef {
	t.Helper()
	provenance, err := typedmemory.NewProvenanceRef(raw)
	if err != nil {
		t.Fatalf("NewProvenanceRef(%q) error = %v", raw, err)
	}
	return provenance
}
