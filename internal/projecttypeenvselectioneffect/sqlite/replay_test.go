package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/testsupport/kerneldbfixture"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestProbeReplayTxReturnsAbsentBeforeAnyCurrentnessRead(t *testing.T) {
	store, err := kerneldbfixture.OpenCurrentStore(
		filepath.Join(t.TempDir(), "haft.db"),
	)
	if err != nil {
		t.Fatalf("new migrated store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	ctx := context.Background()
	transaction, err := sqlitetransaction.BeginImmediate(ctx, store.GetRawDB())
	if err != nil {
		t.Fatalf("begin immediate: %v", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	probe, err := ProbeReplayTx(ctx, transaction, replayProbeFixture(t))
	if err != nil {
		t.Fatalf("probe absent replay: %v", err)
	}
	if _, absent := probe.Absent(); !absent {
		t.Fatalf("probe variant = %#v, want ReplayAbsent", probe)
	}
}

func TestProbeReplayTxRejectsAuthorityUseWithoutClosure(t *testing.T) {
	store, err := kerneldbfixture.OpenCurrentStore(
		filepath.Join(t.TempDir(), "haft.db"),
	)
	if err != nil {
		t.Fatalf("new migrated store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	ctx := context.Background()
	input := replayProbeFixture(t)
	insertOrphanReplayOwner(t, ctx, store.GetRawDB(), input)
	before := countReplayRows(t, ctx, store.GetRawDB())
	transaction, err := sqlitetransaction.BeginImmediate(ctx, store.GetRawDB())
	if err != nil {
		t.Fatalf("begin immediate: %v", err)
	}
	probe, err := ProbeReplayTx(ctx, transaction, input)
	if err == nil {
		t.Fatalf("probe corrupt replay = %#v, want stored-integrity error", probe)
	}
	if !strings.Contains(err.Error(), "corrupt TypeEnv head-selection replay closure footprint") {
		t.Fatalf("probe corrupt replay error = %v", err)
	}
	if rollback := transaction.Rollback(ctx); !rollback.Succeeded() {
		t.Fatalf("rollback replay probe: %v", rollback.Err())
	}
	after := countReplayRows(t, ctx, store.GetRawDB())
	if before != after {
		t.Fatalf("replay probe wrote rows: before=%v after=%v", before, after)
	}
}

type replayRowCounts struct {
	authorityUses int
	closures      int
}

func countReplayRows(
	t *testing.T,
	ctx context.Context,
	raw *sql.DB,
) replayRowCounts {
	t.Helper()
	count := func(table string) int {
		var value int
		if err := raw.QueryRowContext(
			ctx,
			"SELECT COUNT(*) FROM "+table,
		).Scan(&value); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		return value
	}
	return replayRowCounts{
		authorityUses: count("project_typeenv_head_selection_authority_uses"),
		closures:      count("project_typeenv_head_selection_closures"),
	}
}

func insertOrphanReplayOwner(
	t *testing.T,
	ctx context.Context,
	raw *sql.DB,
	input ReplayProbeInput,
) {
	t.Helper()
	connection, err := raw.Conn(ctx)
	if err != nil {
		t.Fatalf("acquire corrupt-fixture connection: %v", err)
	}
	defer connection.Close()
	if _, err := connection.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
		t.Fatalf("disable foreign keys for corrupt fixture: %v", err)
	}
	if _, err := connection.ExecContext(
		ctx,
		`DROP TRIGGER IF EXISTS
			project_typeenv_head_selection_authority_uses_v56_current_generation_only`,
	); err != nil {
		t.Fatalf("disable exact-source trigger for corrupt fixture: %v", err)
	}
	defer func() {
		if _, err := connection.ExecContext(
			ctx,
			"PRAGMA foreign_keys = ON",
		); err != nil {
			t.Fatalf("restore foreign keys after corrupt fixture: %v", err)
		}
	}()
	digest := "sha256:" + strings.Repeat("3", 64)
	_, err = connection.ExecContext(
		ctx,
		`INSERT INTO project_typeenv_head_selection_authority_uses (
			authority_use_ref,
			authority_use_digest,
			authority_generation,
			project_id,
			original_idempotency_key,
			authority_resolution_kind,
			authority_resolution_ref,
			authority_resolution_digest,
			content_ref,
			content_digest,
			request_ref,
			request_digest,
			work_ref,
			receipt_ref,
			predecessor_kind,
			base_type_env_ref,
			ordered_extension_refs_digest,
			canonical_ordered_extension_refs,
			runtime_evaluation_basis_ref,
			selected_composite_ref,
			stage_ref,
			stage_digest,
			expected_graph_revision,
			committed_head_revision,
			committed_graph_revision,
			verifier_ref,
			verifier_edition,
			canonical_bytes,
			recorded_at
		) VALUES (
			?, ?, 'legacy_unreproducible', ?, ?, 'historical_authority', ?, ?, ?, ?, ?, ?, ?, ?,
			'genesis', ?, ?, ?, ?, ?, ?, ?, 0, 1, 1, ?, 1, ?, ?
		)`,
		"authority-use:orphan",
		digest,
		input.Project.String(),
		input.IdempotencyKey.String(),
		"authority-resolution:orphan",
		digest,
		"authorization-content:orphan",
		input.ContentDigest.String(),
		"request:orphan",
		input.RequestDigest.String(),
		"work:orphan",
		"receipt:orphan",
		"type-env:base",
		digest,
		[]byte("[]"),
		"runtime-basis:orphan",
		"type-env:selected",
		"stage:orphan",
		digest,
		"verifier:orphan",
		[]byte("orphan-authority-use"),
		"2026-07-17T00:00:00Z",
	)
	if err != nil {
		t.Fatalf("insert orphan replay owner: %v", err)
	}
}

func replayProbeFixture(t *testing.T) ReplayProbeInput {
	t.Helper()
	project, err := projectidentity.ParseProjectID("qnt_1234abcd")
	if err != nil {
		t.Fatalf("parse project: %v", err)
	}
	key, err := projecttypeenvselection.NewProjectTypeEnvHeadSelectionIdempotencyKey(
		"replay-key",
	)
	if err != nil {
		t.Fatalf("new key: %v", err)
	}
	requestDigest, err := typedmemory.NewSHA256Digest(
		"sha256:" + strings.Repeat("1", 64),
	)
	if err != nil {
		t.Fatalf("new request digest: %v", err)
	}
	contentDigest, err := authority.NewDigest(
		"sha256:" + strings.Repeat("2", 64),
	)
	if err != nil {
		t.Fatalf("new content digest: %v", err)
	}
	return ReplayProbeInput{
		Project:        project,
		IdempotencyKey: key,
		RequestDigest:  requestDigest,
		ContentDigest:  contentDigest,
	}
}
