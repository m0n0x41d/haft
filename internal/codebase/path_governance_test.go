package codebase

import (
	"context"
	"database/sql"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectpath"
)

func TestCodeGovernancePathResolutionAndCoverageMatrix(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := "2026-07-28T00:00:00Z"
	mustCodebaseExec(
		t,
		db,
		`CREATE TABLE code_files (file_path TEXT PRIMARY KEY)`,
	)

	insertModule := func(id, path string) {
		t.Helper()
		mustCodebaseExec(
			t,
			db,
			`INSERT INTO codebase_modules
				(module_id, path, name, lang, file_count, last_scanned)
			 VALUES (?, ?, ?, 'go', 1, ?)`,
			id,
			path,
			id,
			now,
		)
	}
	insertModule("mod-root", "")
	insertModule("mod-cli", "internal/cli")
	insertModule("mod-cli-nested", "internal/cli/nested")
	insertModule("mod-client", "internal/client")
	insertModule("mod-literal", "pkg/a_%")

	scanner := NewScanner(db)
	assertModule := func(filePath, expected string) {
		t.Helper()
		actual, err := scanner.ResolveFileToModule(ctx, filePath)
		if err != nil {
			t.Fatal(err)
		}
		if actual != expected {
			t.Fatalf(
				"ResolveFileToModule(%q) = %q, want %q",
				filePath,
				actual,
				expected,
			)
		}
	}
	assertModule("internal/cli", "mod-cli")
	assertModule(`internal\cli\main.go`, "mod-cli")
	assertModule("internal/client/main.go", "mod-client")
	assertModule("pkg/a_%/main.go", "mod-literal")
	assertModule("pkg/abx/main.go", "mod-root")
	if _, err := scanner.ResolveFileToModule(ctx, "../outside.go"); err == nil {
		t.Fatal("project escape must fail before module lookup")
	}
	for _, filePath := range []string{
		"internal/cli/main.go",
		"internal/cli/nested/main.go",
		"internal/client/a.go",
		"internal/client/b.go",
		"pkg/a_%/a.go",
	} {
		mustCodebaseExec(
			t,
			db,
			`INSERT INTO code_files (file_path) VALUES (?)`,
			filePath,
		)
	}

	insertDecision := func(id, status, affectedPath, structuredData string) {
		t.Helper()
		mustCodebaseExec(
			t,
			db,
			`INSERT INTO artifacts
				(id, kind, version, status, context, mode, title, content,
				 valid_until, created_at, updated_at, structured_data)
			 VALUES (?, 'DecisionRecord', 1, ?, '', '', ?, 'body', '', ?, ?, ?)`,
			id,
			status,
			id,
			now,
			now,
			structuredData,
		)
		if affectedPath != "" {
			mustCodebaseExec(
				t,
				db,
				`INSERT INTO affected_files (artifact_id, file_path, file_hash)
				 VALUES (?, ?, '')`,
				id,
				affectedPath,
			)
		}
	}
	insertDecision(
		"dec-cli",
		"refresh_due",
		"internal/cli",
		`{"governance_mode":"module"}`,
	)
	insertDecision(
		"dec-cli-backslash",
		"active",
		`internal\cli\main.go`,
		`{"governance_mode":"module"}`,
	)
	insertDecision(
		"dec-cli-dot-cleaned",
		"active",
		"internal/cli/./main.go",
		`{"governance_mode":"module"}`,
	)
	insertDecision(
		"dec-cli-explicit-module",
		"active",
		"",
		`{"binding_targets":[{"kind":"module","module_path":"internal/cli"}]}`,
	)
	insertDecision(
		"dec-20260716-11f33e36",
		"active",
		"internal/cli",
		`{"binding_targets":[{"kind":"symbol","file_path":"db/migrations.go"}]}`,
	)
	insertDecision(
		"dec-cli-nested",
		"active",
		"internal/cli/nested/main.go",
		`{"governance_mode":"module"}`,
	)
	insertDecision(
		"dec-cli-nonmodule-dir",
		"active",
		"internal/cli/generated",
		`{"governance_mode":"module"}`,
	)
	insertDecision(
		"dec-client-exact",
		"active",
		"internal/client/a.go",
		`{"governance_mode":"exact"}`,
	)
	insertDecision(
		"dec-literal-footprint",
		"active",
		"pkg/a_%/a.go",
		`{"implementation_footprint":{"files":["pkg/a_%/a.go"]}}`,
	)
	insertDecision(
		"dec-client-terminal",
		"superseded",
		"internal/client/b.go",
		`{"governance_mode":"module"}`,
	)

	report, err := ComputeCoverage(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	statusByModule := make(map[string]CoverageStatus)
	decisionsByModule := make(map[string][]string)
	for _, module := range report.Modules {
		statusByModule[module.Module.ID] = module.Status
		decisionsByModule[module.Module.ID] = module.DecisionIDs
	}
	if statusByModule["mod-cli"] != CoverageCovered {
		t.Fatalf("refresh_due module decision must cover mod-cli: %+v", report)
	}
	if statusByModule["mod-cli-nested"] != CoverageCovered {
		t.Fatalf("nested decision must cover only nested module: %+v", report)
	}
	cliDecisionSet := make(map[string]bool)
	for _, decisionID := range decisionsByModule["mod-cli"] {
		cliDecisionSet[decisionID] = true
	}
	for _, expected := range []string{
		"dec-20260716-11f33e36",
		"dec-cli",
		"dec-cli-explicit-module",
	} {
		if !cliDecisionSet[expected] {
			t.Fatalf("%s missing from CLI coverage: %+v", expected, report)
		}
	}
	for _, excluded := range []string{
		"dec-cli-backslash",
		"dec-cli-dot-cleaned",
		"dec-cli-nested",
		"dec-cli-nonmodule-dir",
	} {
		if cliDecisionSet[excluded] {
			t.Fatalf("%s leaked into CLI coverage: %+v", excluded, report)
		}
	}
	for _, moduleID := range []string{"mod-root", "mod-client", "mod-literal"} {
		if statusByModule[moduleID] != CoverageBlind {
			t.Fatalf(
				"%s = %s, want blind; decisions=%v",
				moduleID,
				statusByModule[moduleID],
				decisionsByModule[moduleID],
			)
		}
	}

	linked, err := activeDecisionLinkedFiles(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"internal/cli",
		"internal/cli/main.go",
		"internal/client/a.go",
		"pkg/a_%/a.go",
	} {
		if !linked[expected] {
			t.Fatalf("current exact association %q is missing: %v", expected, linked)
		}
	}
	for _, excluded := range []string{"internal/client/b.go"} {
		if linked[excluded] {
			t.Fatalf("non-authoritative association %q leaked: %v", excluded, linked)
		}
	}

	literalModule, err := projectpath.ParseModule("pkg/a_%")
	if err != nil {
		t.Fatal(err)
	}
	literalChild, err := projectpath.Parse("pkg/a_%/child.go")
	if err != nil {
		t.Fatal(err)
	}
	if !literalModule.Contains(literalChild) {
		t.Fatal("literal wildcard module must contain its own child")
	}
	sibling, err := projectpath.Parse("pkg/abx/child.go")
	if err != nil {
		t.Fatal(err)
	}
	if literalModule.Contains(sibling) {
		t.Fatal("SQL wildcard characters must remain literal path characters")
	}
}

func mustCodebaseExec(
	t *testing.T,
	db *sql.DB,
	statement string,
	args ...any,
) {
	t.Helper()
	if _, err := db.Exec(statement, args...); err != nil {
		t.Fatal(err)
	}
}
