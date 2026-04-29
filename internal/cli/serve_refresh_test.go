package cli

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
)

func TestApplyRefreshReminderSkipsCommissionProtocol(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()

	seedStaleRefreshScan(t, ctx, store)

	result := `{"commissions":[]}`
	got := applyRefreshReminder(ctx, result, "haft_commission", store)

	if got != result {
		t.Fatalf("commission response was modified:\n%s", got)
	}
}

func TestApplyRefreshReminderSkipsMachineJSON(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()

	seedStaleRefreshScan(t, ctx, store)

	result := `{"problem_card":{"id":"prob-1","kind":"ProblemCard"}}`
	got := applyRefreshReminder(ctx, result, "haft_query", store)

	if got != result {
		t.Fatalf("JSON response was modified:\n%s", got)
	}
}

func TestApplyRefreshReminderKeepsHumanReadableReminder(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()

	seedStaleRefreshScan(t, ctx, store)

	got := applyRefreshReminder(ctx, "Frame recorded.", "haft_problem", store)

	if !strings.Contains(got, "Refresh reminder") {
		t.Fatalf("expected refresh reminder, got:\n%s", got)
	}
}

func seedStaleRefreshScan(t *testing.T, ctx context.Context, store *artifact.Store) {
	t.Helper()

	_, err := store.DB().ExecContext(ctx, `CREATE TABLE IF NOT EXISTS audit_log (
		id TEXT PRIMARY KEY,
		timestamp TEXT NOT NULL,
		tool_name TEXT NOT NULL DEFAULT '',
		operation TEXT NOT NULL,
		actor TEXT NOT NULL DEFAULT '',
		target_id TEXT,
		input_hash TEXT,
		result TEXT NOT NULL DEFAULT '',
		details TEXT,
		context_id TEXT NOT NULL DEFAULT 'default'
	)`)
	if err != nil {
		t.Fatal(err)
	}

	timestamp := time.Now().
		UTC().
		Add(-6 * 24 * time.Hour).
		Format(time.RFC3339)

	_, err = store.DB().ExecContext(
		ctx,
		`INSERT INTO audit_log (id, timestamp, operation) VALUES (?, ?, ?)`,
		"audit-refresh-scan",
		timestamp,
		"haft_refresh:scan",
	)
	if err != nil {
		t.Fatal(err)
	}
}
