package artifact

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSetAffectedFilesRejectsInvalidPathWithoutReplacingExistingRows(
	t *testing.T,
) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	item := &Artifact{
		Meta: Meta{
			ID:        "note-path-boundary",
			Kind:      KindNote,
			Status:    StatusActive,
			Title:     "Path boundary",
			CreatedAt: now,
			UpdatedAt: now,
		},
		Body: "fixture",
	}
	if err := store.Create(ctx, item); err != nil {
		t.Fatal(err)
	}
	if err := store.SetAffectedFiles(
		ctx,
		item.Meta.ID,
		[]AffectedFile{{Path: `internal\cli\main.go`}},
	); err != nil {
		t.Fatal(err)
	}
	if err := store.SetAffectedFiles(
		ctx,
		item.Meta.ID,
		[]AffectedFile{{Path: "../outside.go"}},
	); err == nil {
		t.Fatal("traversal path was accepted")
	}

	files, err := store.GetAffectedFiles(ctx, item.Meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Path != "internal/cli/main.go" {
		t.Fatalf("existing rows changed after rejection: %+v", files)
	}

	if err := store.SetAffectedSymbols(
		ctx,
		item.Meta.ID,
		[]AffectedSymbol{{
			FilePath:   `internal\cli\main.go`,
			SymbolName: "Run",
			SymbolKind: "func",
			Line:       10,
			EndLine:    12,
		}},
	); err != nil {
		t.Fatal(err)
	}
	if err := store.SetAffectedSymbols(
		ctx,
		item.Meta.ID,
		[]AffectedSymbol{{
			FilePath:   `C:relative.go`,
			SymbolName: "Escape",
			SymbolKind: "func",
		}},
	); err == nil {
		t.Fatal("drive-relative affected symbol path was accepted")
	}
	symbols, err := store.GetAffectedSymbols(ctx, item.Meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(symbols) != 1 ||
		symbols[0].FilePath != "internal/cli/main.go" {
		t.Fatalf(
			"symbol rows changed after path rejection: %+v",
			symbols,
		)
	}
}

func TestAffectedFileReadersCanonicalizeLegacyRowsWithoutMigration(
	t *testing.T,
) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	item := &Artifact{
		Meta: Meta{
			ID:        "note-legacy-path-reader",
			Kind:      KindNote,
			Status:    StatusActive,
			Title:     "Legacy path reader",
			CreatedAt: now,
			UpdatedAt: now,
		},
		Body: "fixture",
	}
	if err := store.Create(ctx, item); err != nil {
		t.Fatal(err)
	}
	for _, rawPath := range []string{
		`pkg\a_%\main.go`,
		"pkg/a_%/main.go",
	} {
		if _, err := store.DB().ExecContext(
			ctx,
			`INSERT INTO affected_files
				(artifact_id, file_path, file_hash)
			 VALUES (?, ?, '')`,
			item.Meta.ID,
			rawPath,
		); err != nil {
			t.Fatal(err)
		}
	}

	files, err := store.GetAffectedFiles(ctx, item.Meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Path != "pkg/a_%/main.go" {
		t.Fatalf("canonical affected files = %+v", files)
	}
	projection, err := store.AllAffectedFiles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(projection.Rows) != 1 ||
		projection.Rows[0].FilePath != "pkg/a_%/main.go" {
		t.Fatalf("canonical affected-file refs = %+v", projection.Rows)
	}
	if !projection.Complete() {
		t.Fatalf("canonical rows reported as skipped: %+v", projection.Skipped)
	}
	linked, err := store.SearchByAffectedFile(
		ctx,
		"pkg/a_%/main.go",
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(linked) != 1 || linked[0].Meta.ID != item.Meta.ID {
		t.Fatalf("literal legacy path lookup = %+v", linked)
	}
}

func TestBaselineRejectsPhysicalSymlinkEscapeWithoutReplacingBaseline(
	t *testing.T,
) {
	store := setupTestDB(t)
	ctx := context.Background()
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(outside, "secret.go"),
		[]byte("package secret\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	item := &Artifact{
		Meta: Meta{
			ID:        "dec-symlink-escape",
			Kind:      KindDecisionRecord,
			Status:    StatusActive,
			Title:     "Do not escape",
			CreatedAt: now,
			UpdatedAt: now,
		},
		Body: "fixture",
		StructuredData: `{
			"governance_mode":"exact",
			"binding_targets":[{
				"kind":"whole_file_fallback",
				"file_path":"escape/secret.go"
			}]
		}`,
	}
	if err := store.Create(ctx, item); err != nil {
		t.Fatal(err)
	}
	if err := store.SetAffectedFiles(
		ctx,
		item.Meta.ID,
		[]AffectedFile{{
			Path: "escape/secret.go",
			Hash: "original-baseline",
		}},
	); err != nil {
		t.Fatal(err)
	}

	_, err := Baseline(ctx, store, root, BaselineInput{
		DecisionRef: item.Meta.ID,
	})
	if err == nil || !strings.Contains(err.Error(), "outside project root") {
		t.Fatalf("baseline escape error = %v", err)
	}
	files, getErr := store.GetAffectedFiles(ctx, item.Meta.ID)
	if getErr != nil {
		t.Fatal(getErr)
	}
	if len(files) != 1 || files[0].Hash != "original-baseline" {
		t.Fatalf("rejected baseline replaced stored rows: %+v", files)
	}
}

func TestBuildDecisionArtifactRejectsInvalidAuthorityBearingPaths(
	t *testing.T,
) {
	tests := []struct {
		name   string
		field  string
		mutate func(*DecideInput)
	}{
		{
			name:  "binding target traversal",
			field: "binding_targets[0].file_path",
			mutate: func(input *DecideInput) {
				input.BindingTargets = []BindingTarget{{
					Kind:     BindingTargetSymbol,
					FilePath: "../outside.go",
				}}
			},
		},
		{
			name:  "binding target absolute module",
			field: "binding_targets[0].module_path",
			mutate: func(input *DecideInput) {
				input.BindingTargets = []BindingTarget{{
					Kind:       BindingTargetModule,
					ModulePath: "/tmp/outside",
				}}
			},
		},
		{
			name:  "module target without module path",
			field: "binding_targets[0].module_path is required",
			mutate: func(input *DecideInput) {
				input.BindingTargets = []BindingTarget{{
					Kind:     BindingTargetModule,
					FilePath: "internal/cli/main.go",
				}}
			},
		},
		{
			name:  "governance target UNC path",
			field: "governance_targets[0].binding_target.file_path",
			mutate: func(input *DecideInput) {
				input.GovernanceTargets = []GovernanceTarget{{
					Kind: "code",
					BindingTarget: &BindingTarget{
						Kind:     BindingTargetWholeFileFallback,
						FilePath: `\\server\share\outside.go`,
					},
				}}
			},
		},
		{
			name:  "drift watch drive path",
			field: "drift_watch_targets[0].binding_target.file_path",
			mutate: func(input *DecideInput) {
				input.DriftWatchTargets = []DriftWatchTarget{{
					TargetRef: "code",
					Trigger:   "source_changed",
					BindingTarget: &BindingTarget{
						Kind:     BindingTargetWholeFileFallback,
						FilePath: `C:outside.go`,
					},
				}}
			},
		},
		{
			name:  "implementation footprint control character",
			field: "implementation_footprint.files[0]",
			mutate: func(input *DecideInput) {
				input.ImplementationFootprint = ImplementationFootprint{
					Files: []string{"internal/cli/\x00outside.go"},
				}
			},
		},
		{
			name:  "affected file absolute path",
			field: "affected_files[0]",
			mutate: func(input *DecideInput) {
				input.AffectedFiles = []string{"/tmp/outside.go"}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := decisionPathContractInput()
			test.mutate(&input)

			_, err := BuildDecisionArtifact(
				DecideContext{ID: "dec-path-contract"},
				input,
			)
			if err == nil || !strings.Contains(err.Error(), test.field) {
				t.Fatalf(
					"BuildDecisionArtifact error = %v, want field %q",
					err,
					test.field,
				)
			}
		})
	}
}

func TestNormalizeDecisionInputCanonicalizesEveryTypedPathCarrier(
	t *testing.T,
) {
	input := decisionPathContractInput()
	input.ImplementationFootprint = ImplementationFootprint{
		Files: []string{`internal\cli\.\worker.go`},
	}
	input.BindingTargets = []BindingTarget{{
		Kind:       BindingTargetModule,
		ModulePath: `internal\cli`,
	}}
	input.GovernanceTargets = []GovernanceTarget{{
		Kind: "code",
		BindingTarget: &BindingTarget{
			Kind:     BindingTargetWholeFileFallback,
			FilePath: `internal\cli\main.go`,
		},
	}}
	input.DriftWatchTargets = []DriftWatchTarget{{
		TargetRef: "code",
		Trigger:   "source_changed",
		BindingTarget: &BindingTarget{
			Kind:     BindingTargetRange,
			FilePath: `internal\cli\worker.go`,
		},
	}}

	normalized := normalizeDecisionInput(input)
	if err := validateDecisionProjectPaths(normalized); err != nil {
		t.Fatal(err)
	}
	if len(normalized.ImplementationFootprint.Files) != 1 ||
		normalized.ImplementationFootprint.Files[0] !=
			"internal/cli/worker.go" {
		t.Fatalf(
			"implementation footprint = %+v",
			normalized.ImplementationFootprint,
		)
	}
	if len(normalized.BindingTargets) != 1 ||
		normalized.BindingTargets[0].ModulePath != "internal/cli" {
		t.Fatalf("module binding = %+v", normalized.BindingTargets)
	}
	if len(normalized.GovernanceTargets) != 1 ||
		normalized.GovernanceTargets[0].BindingTarget == nil ||
		normalized.GovernanceTargets[0].BindingTarget.FilePath !=
			"internal/cli/main.go" {
		t.Fatalf(
			"governance targets = %+v",
			normalized.GovernanceTargets,
		)
	}
	if len(normalized.DriftWatchTargets) != 1 ||
		normalized.DriftWatchTargets[0].BindingTarget == nil ||
		normalized.DriftWatchTargets[0].BindingTarget.FilePath !=
			"internal/cli/worker.go" {
		t.Fatalf(
			"drift watch targets = %+v",
			normalized.DriftWatchTargets,
		)
	}
}

func decisionPathContractInput() DecideInput {
	const (
		selected = "Keep every decision path project-relative"
		reason   = "One parser keeps all authority-bearing paths canonical."
		policy   = "Reject paths that cannot be represented by projectpath."
	)
	return completeDecision(DecideInput{
		SelectedTitle:   selected,
		WhySelected:     reason,
		SelectionPolicy: policy,
		ChoiceResult: &ChoiceResult{
			SubjectRef: "operator",
			OptionSet: []string{
				selected,
				"Keep per-field path normalization",
			},
			ChoiceRule: policy,
			NextMove:   ChoiceNextMoveChooseNow,
			VariantRef: selected,
			Reason:     reason,
		},
	})
}

// A row stored before canonicalAffectedFile existed must not deny every reader
// the rest of the table. The projection excludes it and names it; it never
// fails the read.
func TestAffectedFileReadsDegradeOnPreInvariantStoredRow(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	item := &Artifact{
		Meta: Meta{
			ID:        "note-pre-invariant-path",
			Kind:      KindNote,
			Status:    StatusActive,
			Title:     "Pre-invariant affected path",
			CreatedAt: now,
			UpdatedAt: now,
		},
		Body: "fixture",
	}
	if err := store.Create(ctx, item); err != nil {
		t.Fatal(err)
	}
	if err := store.SetAffectedFiles(
		ctx,
		item.Meta.ID,
		[]AffectedFile{{Path: "internal/cli/main.go"}},
	); err != nil {
		t.Fatal(err)
	}

	// SetAffectedFiles rejects this path today, so the fixture reaches the
	// table directly — exactly how the row was admitted before the invariant.
	const legacyPath = "/Users/someone/.agent/attachments/pasted-text.txt"
	if _, err := store.db.ExecContext(
		ctx,
		`INSERT INTO affected_files (artifact_id, file_path, file_hash)
		 VALUES (?, ?, '')`,
		item.Meta.ID,
		legacyPath,
	); err != nil {
		t.Fatal(err)
	}

	projection, err := store.AllAffectedFiles(ctx)
	if err != nil {
		t.Fatalf("one pre-invariant row failed the whole projection: %v", err)
	}
	if len(projection.Rows) != 1 ||
		projection.Rows[0].FilePath != "internal/cli/main.go" {
		t.Fatalf("expressible rows = %+v", projection.Rows)
	}
	if projection.Complete() {
		t.Fatal("excluded row was not named in the projection")
	}
	if len(projection.Skipped) != 1 ||
		projection.Skipped[0].RawPath != legacyPath ||
		projection.Skipped[0].ArtifactID != item.Meta.ID {
		t.Fatalf("skipped rows = %+v", projection.Skipped)
	}
	if projection.Skipped[0].Reason == "" {
		t.Fatal("skipped row carries no reason")
	}

	files, err := store.GetAffectedFiles(ctx, item.Meta.ID)
	if err != nil {
		t.Fatalf("per-artifact read failed on a pre-invariant row: %v", err)
	}
	if len(files) != 1 || files[0].Path != "internal/cli/main.go" {
		t.Fatalf("per-artifact affected files = %+v", files)
	}
}
