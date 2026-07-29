package cli

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
)

func TestMemoryBackfillDryRunThenApplyUsesSourceOwnedProjection(t *testing.T) {
	fixture := newTaskMemoryProjectionTestFixture(t)
	note := artifact.BuildNoteArtifact(
		"note-20260726-existing-backfill",
		time.Date(2026, 7, 26, 10, 0, 0, 0, time.UTC),
		artifact.NoteInput{
			Title: "Existing typed-memory source",
			Observations: []string{
				"Existing records remain byte-preserved during projection.",
			},
			Evidence: "test:memory-backfill",
		},
	)
	if err := fixture.store.Create(
		context.Background(),
		note,
	); err != nil {
		t.Fatalf("store existing Note: %v", err)
	}
	evidence := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:      "evid-20260726-existing-backfill",
			Kind:    artifact.KindEvidencePack,
			Version: 1,
			Title:   "Deferred evidence carrier",
		},
		Body: "Evidence meaning requires an exact Work source.",
	}
	if err := fixture.store.Create(
		context.Background(),
		evidence,
	); err != nil {
		t.Fatalf("store existing EvidencePack: %v", err)
	}
	beforeBody := note.Body
	beforeRevision := loadTaskMemoryProjectionRevision(t, fixture)

	dryRun := runMemoryBackfillRequest(
		t,
		fixture,
		memoryBackfillRequestFixture(memoryBackfillDryRun),
	)
	if dryRun.GraphRevisionBefore != beforeRevision ||
		dryRun.GraphRevisionAfter != beforeRevision {
		t.Fatalf(
			"dry-run revisions = %d -> %d, want %d",
			dryRun.GraphRevisionBefore,
			dryRun.GraphRevisionAfter,
			beforeRevision,
		)
	}
	if dryRun.Summary.ValidatedOnly != 1 ||
		dryRun.Summary.Committed != 0 ||
		dryRun.Summary.Deferred != 1 {
		t.Fatalf("dry-run summary = %#v", dryRun.Summary)
	}
	if len(dryRun.Routes) != 1 ||
		dryRun.Routes[0].Result != "validated_only" ||
		dryRun.Routes[0].ProjectionReport == nil ||
		dryRun.Routes[0].ProjectionReport.Persistence.Mode !=
			"validation_only_no_write" {
		t.Fatalf("dry-run route = %#v", dryRun.Routes)
	}
	if dryRun.Routes[0].ProjectionReport.RecordReference != nil {
		t.Fatalf(
			"dry-run invented durable record ref = %#v",
			dryRun.Routes[0].ProjectionReport.RecordReference,
		)
	}

	applied := runMemoryBackfillRequest(
		t,
		fixture,
		memoryBackfillRequestFixture(memoryBackfillApply),
	)
	if applied.Summary.Committed != 1 ||
		applied.Summary.Deferred != 1 {
		t.Fatalf("apply summary = %#v", applied.Summary)
	}
	if applied.GraphRevisionAfter <= applied.GraphRevisionBefore {
		t.Fatalf(
			"apply revisions = %d -> %d, want committed advance",
			applied.GraphRevisionBefore,
			applied.GraphRevisionAfter,
		)
	}
	if len(applied.Routes) != 1 ||
		applied.Routes[0].ProjectionReport == nil ||
		applied.Routes[0].ProjectionReport.RecordReference == nil {
		t.Fatalf("apply route = %#v", applied.Routes)
	}
	assertTaskMemoryProjectionIsObservable(
		t,
		fixture,
		*applied.Routes[0].ProjectionReport,
	)
	reloaded, err := fixture.store.Get(
		context.Background(),
		note.Meta.ID,
	)
	if err != nil {
		t.Fatalf("reload existing Note: %v", err)
	}
	if reloaded.Body != beforeBody {
		t.Fatal("backfill changed legacy Note carrier bytes")
	}

	replayedRevision := loadTaskMemoryProjectionRevision(t, fixture)
	replayed := runMemoryBackfillRequest(
		t,
		fixture,
		memoryBackfillRequestFixture(memoryBackfillDryRun),
	)
	if replayed.Summary.AlreadyProjected != 1 ||
		replayed.Summary.Unavailable != 0 ||
		replayed.GraphRevisionBefore != replayedRevision ||
		replayed.GraphRevisionAfter != replayedRevision {
		t.Fatalf(
			"already-projected dry-run = summary %#v revisions %d -> %d",
			replayed.Summary,
			replayed.GraphRevisionBefore,
			replayed.GraphRevisionAfter,
		)
	}
	if replayed.Routes[0].ProjectionReport == nil ||
		replayed.Routes[0].ProjectionReport.Persistence.Mode !=
			"existing_exact_projection_no_write" {
		t.Fatalf("already-projected route = %#v", replayed.Routes)
	}
}

func TestDecodeMemoryBackfillRequestFailsClosed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		payload string
		want    string
	}{
		{
			name:    "unknown field",
			payload: `{"contract_version":"haft.memory.backfill.v1","mode":"dry_run","request_provenance_ref":"operator:test","items":[{"artifact_ref":"note-a"}],"extra":true}`,
			want:    "unknown field",
		},
		{
			name:    "duplicate artifact",
			payload: `{"contract_version":"haft.memory.backfill.v1","mode":"dry_run","request_provenance_ref":"operator:test","items":[{"artifact_ref":"note-a"},{"artifact_ref":"note-a"}]}`,
			want:    "duplicate artifact_ref",
		},
		{
			name:    "partial concern",
			payload: `{"contract_version":"haft.memory.backfill.v1","mode":"dry_run","request_provenance_ref":"operator:test","items":[{"artifact_ref":"note-a","bounded_context_ref":"haft-project"}]}`,
			want:    "requires entity_ref",
		},
		{
			name:    "trailing value",
			payload: `{"contract_version":"haft.memory.backfill.v1","mode":"dry_run","request_provenance_ref":"operator:test","items":[{"artifact_ref":"note-a"}]} {}`,
			want:    "multiple JSON values",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := decodeMemoryBackfillRequest(
				[]byte(test.payload),
			)
			if err == nil ||
				!strings.Contains(err.Error(), test.want) {
				t.Fatalf(
					"decode error = %v, want %q",
					err,
					test.want,
				)
			}
		})
	}
}

func TestMemoryBackfillInterfaceIsCLIOnlyAndNamesAuthorityBoundary(
	t *testing.T,
) {
	t.Parallel()

	capability, found := findInterfaceCapability(
		haftInterfaceCatalog(),
		"memory.backfill",
	)
	if !found {
		t.Fatal("memory.backfill capability is absent")
	}
	if capability.CurrentExecution.MCPTool != "" ||
		capability.CurrentExecution.CLICommand !=
			"haft memory backfill --input-file request.json" {
		t.Fatalf(
			"memory.backfill execution = %#v",
			capability.CurrentExecution,
		)
	}
	invariants := strings.Join(capability.Invariants, "\n")
	for _, expected := range []string{
		"Dry-run writes zero rows",
		"never automatic background migration",
		"cannot declare schema",
		"Legacy carriers remain byte-preserved",
	} {
		if !strings.Contains(invariants, expected) {
			t.Fatalf(
				"memory.backfill invariants omit %q:\n%s",
				expected,
				invariants,
			)
		}
	}
}

func memoryBackfillRequestFixture(
	mode memoryBackfillMode,
) memoryBackfillRequest {
	return memoryBackfillRequest{
		ContractVersion:      memoryBackfillContractVersion,
		Mode:                 mode,
		RequestProvenanceRef: "operator:test-memory-backfill",
		Items: []memoryBackfillItem{
			{
				ArtifactRef: "note-20260726-existing-backfill",
				EntityRef: &memoryBackfillEntityRef{
					RefKindID:   "U.EntityRef",
					ReferenceID: taskMemoryTestConcern,
				},
				BoundedContextID: taskMemoryTestContext,
			},
			{
				ArtifactRef: "evid-20260726-existing-backfill",
			},
		},
	}
}

func runMemoryBackfillRequest(
	t *testing.T,
	fixture taskMemoryProjectionTestFixture,
	request memoryBackfillRequest,
) memoryBackfillReport {
	t.Helper()

	report, err := executeMemoryBackfill(
		context.Background(),
		fixture.projectID,
		fixture.store,
		fixture.projector,
		request,
	)
	if err != nil {
		t.Fatalf("executeMemoryBackfill() error = %v", err)
	}
	return report
}
