package method

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/artifact"
)

func TestCheckpointOpenCloseTraceIsAppendOnly(t *testing.T) {
	ctx := context.Background()
	store, haftDir := newCheckpointTestStore(t)
	_, _, run, err := CreateRun(ctx, store, haftDir, PullInput{
		Task:             "checkpoint test",
		DeclaredTaskKind: "feature",
		ChangeIntent:     "test checkpoint open close trace behavior",
	})
	if err != nil {
		t.Fatal(err)
	}

	opened, err := OpenCheckpoint(ctx, store, haftDir, CheckpointOpenInput{
		RunID:        run.ID,
		TargetRef:    "internal/method/checkpoint.go::OpenCheckpoint",
		CheckRef:     "method:checkpoint-pilot",
		TargetDigest: "sha256:open",
		TTL:          time.Minute,
	})
	if err != nil {
		t.Fatalf("OpenCheckpoint: %v", err)
	}
	if opened.CloseToken == "" {
		t.Fatal("open result missing close token")
	}
	if opened.Record.CloseToken != "" {
		t.Fatalf("open record persisted raw close token: %#v", opened.Record)
	}
	if opened.Record.CloseTokenHash == "" || opened.Record.CloseTokenHash == opened.CloseToken {
		t.Fatalf("open record missing token hash: %#v", opened.Record)
	}
	_, storedOpenRun, err := loadRunArtifact(ctx, store, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if storedOpenRun.Checkpoints[0].CloseToken != "" {
		t.Fatalf("stored checkpoint persisted raw close token: %#v", storedOpenRun.Checkpoints[0])
	}
	if storedOpenRun.Checkpoints[0].CloseTokenHash == "" {
		t.Fatalf("stored checkpoint missing close token hash: %#v", storedOpenRun.Checkpoints[0])
	}

	trace, err := TraceCheckpoints(ctx, store, run.ID)
	if err != nil {
		t.Fatalf("TraceCheckpoints(open): %v", err)
	}
	if trace.Summary.Records != 1 || trace.Summary.Open != 1 || trace.Summary.Closed != 0 {
		t.Fatalf("open trace summary = %+v", trace.Summary)
	}
	traceBytes, err := json.Marshal(trace)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(traceBytes), opened.CloseToken) {
		t.Fatalf("trace leaked raw close token: %s", string(traceBytes))
	}

	closed, err := CloseCheckpoint(ctx, store, haftDir, CheckpointCloseInput{
		CloseToken:      opened.CloseToken,
		Outcome:         "reviewed",
		ObservationRefs: []string{"go test ./internal/method"},
		ResultingDigest: "sha256:closed",
		NextTargetRef:   "internal/method/checkpoint_test.go",
	})
	if err != nil {
		t.Fatalf("CloseCheckpoint: %v", err)
	}
	if closed.CheckpointID != opened.CheckpointID {
		t.Fatalf("closed checkpoint = %q, want %q", closed.CheckpointID, opened.CheckpointID)
	}
	if closed.Record.CloseToken != "" {
		t.Fatalf("close record persisted raw close token: %#v", closed.Record)
	}
	if closed.Record.CloseTokenHash == "" {
		t.Fatalf("close record missing token hash: %#v", closed.Record)
	}

	trace, err = TraceCheckpoints(ctx, store, run.ID)
	if err != nil {
		t.Fatalf("TraceCheckpoints(closed): %v", err)
	}
	if trace.Summary.Records != 2 || trace.Summary.Open != 0 || trace.Summary.Closed != 1 {
		t.Fatalf("closed trace summary = %+v", trace.Summary)
	}
	if len(trace.States) != 1 || trace.States[0].Outcome != "reviewed" {
		t.Fatalf("trace states = %+v", trace.States)
	}
	if trace.States[0].CloseToken != "" || trace.States[0].CloseTokenHash == "" {
		t.Fatalf("trace state should expose hash, not raw token: %+v", trace.States[0])
	}

	_, err = CloseCheckpoint(ctx, store, haftDir, CheckpointCloseInput{
		CloseToken: opened.CloseToken,
		Outcome:    "duplicate close",
	})
	if err == nil {
		t.Fatal("expected duplicate close to fail")
	}
	if !strings.Contains(err.Error(), "already closed") {
		t.Fatalf("duplicate close error = %v", err)
	}
}

func TestCheckpointCloseAcceptsLegacyRawTokenRecord(t *testing.T) {
	ctx := context.Background()
	store, haftDir := newCheckpointTestStore(t)
	_, _, run, err := CreateRun(ctx, store, haftDir, PullInput{
		Task:             "legacy checkpoint token test",
		DeclaredTaskKind: "feature",
		ChangeIntent:     "test legacy raw close token compatibility",
	})
	if err != nil {
		t.Fatal(err)
	}

	opened, err := OpenCheckpoint(ctx, store, haftDir, CheckpointOpenInput{
		RunID:        run.ID,
		TargetRef:    "internal/method/checkpoint.go::openCheckpointByToken",
		CheckRef:     "method:checkpoint-legacy",
		TargetDigest: "sha256:open",
		TTL:          time.Minute,
	})
	if err != nil {
		t.Fatalf("OpenCheckpoint: %v", err)
	}

	runArtifact, legacyRun, err := loadRunArtifact(ctx, store, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	legacyRun.Checkpoints[0].CloseToken = opened.CloseToken
	legacyRun.Checkpoints[0].CloseTokenHash = ""
	persistCheckpointTestRun(t, ctx, store, runArtifact, legacyRun)

	closed, err := CloseCheckpoint(ctx, store, haftDir, CheckpointCloseInput{
		CloseToken: opened.CloseToken,
		Outcome:    "legacy raw token closed",
	})
	if err != nil {
		t.Fatalf("CloseCheckpoint legacy raw token: %v", err)
	}
	if closed.CheckpointID != opened.CheckpointID {
		t.Fatalf("closed checkpoint = %q, want %q", closed.CheckpointID, opened.CheckpointID)
	}
	if closed.Record.CloseToken != "" || closed.Record.CloseTokenHash == "" {
		t.Fatalf("legacy close should persist hash, not raw token: %#v", closed.Record)
	}
}

func TestCheckpointCloseRejectsExpiredToken(t *testing.T) {
	ctx := context.Background()
	store, haftDir := newCheckpointTestStore(t)
	_, _, run, err := CreateRun(ctx, store, haftDir, PullInput{
		Task:             "expired checkpoint test",
		DeclaredTaskKind: "feature",
		ChangeIntent:     "test expired checkpoint rejection",
	})
	if err != nil {
		t.Fatal(err)
	}

	opened, err := OpenCheckpoint(ctx, store, haftDir, CheckpointOpenInput{
		RunID:        run.ID,
		TargetRef:    "internal/method/checkpoint.go::CloseCheckpoint",
		CheckRef:     "method:checkpoint-expiry",
		TargetDigest: "sha256:open",
		TTL:          time.Minute,
	})
	if err != nil {
		t.Fatalf("OpenCheckpoint: %v", err)
	}
	forceCheckpointExpired(t, ctx, store, run.ID)

	_, err = CloseCheckpoint(ctx, store, haftDir, CheckpointCloseInput{
		CloseToken: opened.CloseToken,
		Outcome:    "late",
	})
	if err == nil {
		t.Fatal("expected expired checkpoint close to fail")
	}
	if !strings.Contains(err.Error(), "expired") {
		t.Fatalf("expired close error = %v", err)
	}
}

func TestCloseRunPreservesUnknownStructuredDataFields(t *testing.T) {
	ctx := context.Background()
	store, haftDir := newCheckpointTestStore(t)
	_, _, run, err := CreateRun(ctx, store, haftDir, PullInput{
		Task:             "future method run field preservation",
		DeclaredTaskKind: "mechanical",
		ChangeIntent:     "test method run forward compatibility",
		CeremonyRequest:  "low",
	})
	if err != nil {
		t.Fatal(err)
	}

	runArtifact, err := store.Get(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	structured := map[string]json.RawMessage{}
	if err := json.Unmarshal([]byte(runArtifact.StructuredData), &structured); err != nil {
		t.Fatal(err)
	}
	structured["future_methodrun_extension"] = json.RawMessage(`{"records":[{"kind":"future_checkpoint"}]}`)
	updated, err := json.Marshal(structured)
	if err != nil {
		t.Fatal(err)
	}
	runArtifact.StructuredData = string(updated)
	if err := store.Update(ctx, runArtifact); err != nil {
		t.Fatal(err)
	}

	_, _, _, err = CloseRun(ctx, store, haftDir, CloseInput{
		PullID:       run.ID,
		ChangedFiles: []string{"internal/method/run.go"},
		GateResults:  satisfiedGateResults(run),
		Verification: Verification{
			Commands: []string{"go test ./internal/method"},
			Result:   "pass",
		},
	})
	if err != nil {
		t.Fatalf("CloseRun: %v", err)
	}

	closedArtifact, err := store.Get(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	closedStructured := map[string]json.RawMessage{}
	if err := json.Unmarshal([]byte(closedArtifact.StructuredData), &closedStructured); err != nil {
		t.Fatal(err)
	}
	if _, ok := closedStructured["future_methodrun_extension"]; !ok {
		t.Fatalf("future extension was dropped: %s", closedArtifact.StructuredData)
	}
	if string(closedStructured["status"]) != `"closed"` {
		t.Fatalf("status was not updated in merged structured data: %s", closedStructured["status"])
	}
}

func satisfiedGateResults(run MethodRun) []GateResult {
	var results []GateResult
	for _, card := range run.Methods {
		for _, gate := range card.HardGates {
			results = append(results, GateResult{
				GateID:       gate.ID,
				Status:       "satisfied",
				EvidenceRefs: []string{"test:evidence"},
			})
		}
	}
	return results
}

func newCheckpointTestStore(t *testing.T) (*artifact.Store, string) {
	t.Helper()
	database, err := db.NewStore(filepath.Join(t.TempDir(), "haft.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = database.Close()
	})
	haftDir := filepath.Join(t.TempDir(), ".haft")
	if err := os.MkdirAll(filepath.Join(haftDir, artifact.KindMethodRun.Dir()), 0o755); err != nil {
		t.Fatal(err)
	}
	return artifact.NewStore(database.GetRawDB()), haftDir
}

func forceCheckpointExpired(t *testing.T, ctx context.Context, store *artifact.Store, runID string) {
	t.Helper()
	runArtifact, err := store.Get(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	run, err := DecodeRun(runArtifact)
	if err != nil {
		t.Fatal(err)
	}
	run.Checkpoints[0].ExpiresAt = nowRFC3339(time.Now().UTC().Add(-time.Minute))
	persistCheckpointTestRun(t, ctx, store, runArtifact, run)
}

func persistCheckpointTestRun(t *testing.T, ctx context.Context, store *artifact.Store, runArtifact *artifact.Artifact, run MethodRun) {
	t.Helper()
	encoded, err := json.Marshal(run)
	if err != nil {
		t.Fatal(err)
	}
	runArtifact.StructuredData = string(encoded)
	runArtifact.Body = RenderRunBody(run)
	if err := store.Update(ctx, runArtifact); err != nil {
		t.Fatal(err)
	}
}
