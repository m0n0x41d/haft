package cli

import (
	"context"
	"fmt"
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

func TestHandleQuintRefreshReviewBuildsReadOnlyJudgmentPacket(t *testing.T) {
	fixture := newCheckTestProject(t)
	seedGovernanceDebt(t, fixture)

	before := countArtifacts(t, fixture)
	result, err := handleQuintRefresh(context.Background(), fixture.store, fixture.haftDir, map[string]any{
		"action": "review",
	})
	if err != nil {
		t.Fatalf("handleQuintRefresh(review) returned error: %v", err)
	}
	after := countArtifacts(t, fixture)
	if after != before {
		t.Fatalf("artifact count changed: before=%d after=%d", before, after)
	}

	for _, want := range []string{
		"Maintenance Judgment Review",
		"not_mutation",
		"operator_approval_required",
		"review_material_drift",
	} {
		if !strings.Contains(result, want) {
			t.Fatalf("review response missing %q:\n%s", want, result)
		}
	}
}

func TestHandleQuintRefreshDrainDryRunBuildsSafePreview(t *testing.T) {
	fixture := newCheckTestProject(t)
	seedGovernanceDebt(t, fixture)

	before := countArtifacts(t, fixture)
	result, err := handleQuintRefresh(context.Background(), fixture.store, fixture.haftDir, map[string]any{
		"action":  "drain",
		"dry_run": true,
	})
	if err != nil {
		t.Fatalf("handleQuintRefresh(drain dry-run) returned error: %v", err)
	}
	after := countArtifacts(t, fixture)
	if after != before {
		t.Fatalf("dry-run artifact count changed: before=%d after=%d", before, after)
	}

	for _, want := range []string{
		"Maintenance Drain (dry-run)",
		"not_mutation",
		"Needs operator:",
	} {
		if !strings.Contains(result, want) {
			t.Fatalf("drain dry-run response missing %q:\n%s", want, result)
		}
	}

	staleItems, err := artifact.ScanStale(context.Background(), fixture.store)
	if err != nil {
		t.Fatalf("ScanStale returned error: %v", err)
	}
	wantFooter := fmt.Sprintf("Stale: %d decision(s) need refresh", len(staleItems))
	if len(staleItems) > 0 && !strings.Contains(result, wantFooter) {
		t.Fatalf("drain footer should use typed stale lane %q:\n%s", wantFooter, result)
	}
}

func TestHandleQuintRefreshDrainFooterUsesTypedStaleSnapshot(t *testing.T) {
	fixture := newCheckTestProject(t)
	decision := mustCreateDecision(t, fixture, artifact.DecideInput{
		SelectedTitle:   "Deprecated expired decision",
		WhySelected:     "Need a terminal decision that raw nav would count as stale.",
		SelectionPolicy: "Prefer a single decision with no active operator work.",
		CounterArgument: "Terminal decisions should not surface as current stale debt.",
		WeakestLink:     "Footer stale count must follow the typed status snapshot.",
		WhyNotOthers: []artifact.RejectionReason{{
			Variant: "Active expired decision",
			Reason:  "Would be legitimate stale debt and would not prove footer filtering.",
		}},
		Rollback: &artifact.RollbackSpec{
			Triggers: []string{"Deprecated decisions resurface in compact status footers."},
		},
	})
	mustSetValidUntil(t, fixture, decision.Meta.ID, time.Now().Add(-72*time.Hour).Format("2006-01-02"))
	mustSetArtifactStatus(t, fixture, decision.Meta.ID, artifact.StatusDeprecated)

	result, err := handleQuintRefresh(context.Background(), fixture.store, fixture.haftDir, map[string]any{
		"action":  "drain",
		"dry_run": true,
	})
	if err != nil {
		t.Fatalf("handleQuintRefresh(drain dry-run) returned error: %v", err)
	}
	if strings.Contains(result, "Stale: 1 decision(s) need refresh") {
		t.Fatalf("drain footer used raw stale decision count instead of typed snapshot:\n%s", result)
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
