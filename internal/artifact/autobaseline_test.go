package artifact

import "testing"

// helpers mirroring symbol_drift_test.go style
func tdAdded(name string) SymbolDriftItem {
	return SymbolDriftItem{SymbolName: name, SymbolKind: "func", Status: "added"}
}
func tdModified(name string) SymbolDriftItem {
	return SymbolDriftItem{SymbolName: name, SymbolKind: "func", Status: "modified"}
}
func tdRemoved(name string) SymbolDriftItem {
	return SymbolDriftItem{SymbolName: name, SymbolKind: "func", Status: "removed"}
}

// TestClassifyAutoBaseline_VerdictMapping pins the table: each kernel verdict
// maps to exactly one disposition, and the safety-critical one (governed_modified)
// never becomes AutoResolveSilent.
func TestClassifyAutoBaseline_VerdictMapping(t *testing.T) {
	cases := []struct {
		name  string
		files []DriftItem
		want  AutoBaselineAction
	}{
		{
			name:  "additive-only resolves silently",
			files: []DriftItem{{Path: "a.go", Status: DriftModified, Symbols: []SymbolDriftItem{tdAdded("Foo")}}},
			want:  AutoResolveSilent,
		},
		{
			name:  "added file resolves silently",
			files: []DriftItem{{Path: "new.go", Status: DriftAdded}},
			want:  AutoResolveSilent,
		},
		{
			name:  "modified governed symbol stages for confirm",
			files: []DriftItem{{Path: "a.go", Status: DriftModified, Symbols: []SymbolDriftItem{tdModified("Bar")}}},
			want:  StageForConfirm,
		},
		{
			name:  "removed governed symbol stages for confirm",
			files: []DriftItem{{Path: "a.go", Status: DriftModified, Symbols: []SymbolDriftItem{tdRemoved("Bar")}}},
			want:  StageForConfirm,
		},
		{
			name:  "deleted file stages for confirm",
			files: []DriftItem{{Path: "gone.go", Status: DriftMissing}},
			want:  StageForConfirm,
		},
		{
			name:  "unanalyzable change surfaces for review",
			files: []DriftItem{{Path: "a.bin", Status: DriftModified}},
			want:  SurfaceForReview,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := DriftReport{HasBaseline: true, Files: tc.files}
			got := ClassifyAutoBaseline([]DriftReport{r})
			if len(got) != 1 {
				t.Fatalf("expected 1 disposition, got %d", len(got))
			}
			if got[0].Action != tc.want {
				t.Fatalf("Action = %q, want %q (reason: %s)", got[0].Action, tc.want, got[0].Reason)
			}
		})
	}
}

// TestClassifyAutoBaseline_SeededHarnessDrainSurfaces is the regression test
// named in dec-20260606-9b4a4c52 prediction #2: the harness-drain decision's
// governed func canApplyHarnessWorkspaceDiff was REMOVED; that drift must stage
// for confirm and must NEVER be silently auto-baselined.
func TestClassifyAutoBaseline_SeededHarnessDrainSurfaces(t *testing.T) {
	harnessDrain := DriftReport{
		DecisionID:    "dec-20260428-harness-drain-v3-16bf21f3",
		DecisionTitle: "Per-WorkCommission delivery_policy as the apply-authority gate for batch drain mode",
		HasBaseline:   true,
		Files: []DriftItem{
			{
				Path:    "internal/cli/harness.go",
				Status:  DriftModified,
				Symbols: []SymbolDriftItem{tdRemoved("canApplyHarnessWorkspaceDiff")},
			},
		},
	}
	got := ClassifyAutoBaseline([]DriftReport{harnessDrain})
	if got[0].Action == AutoResolveSilent {
		t.Fatalf("SAFETY VIOLATION: removed governed func was silently auto-baselined")
	}
	if got[0].Action != StageForConfirm {
		t.Fatalf("Action = %q, want %q", got[0].Action, StageForConfirm)
	}
}

// TestClassifyAutoBaseline_NeverSilentlyResolvesGoverned is the property behind
// prediction #3: across any report whose kernel verdict is governed_modified,
// the disposition is never AutoResolveSilent.
func TestClassifyAutoBaseline_NeverSilentlyResolvesGoverned(t *testing.T) {
	governed := []DriftReport{
		{HasBaseline: true, Files: []DriftItem{{Path: "a.go", Status: DriftModified, Symbols: []SymbolDriftItem{tdModified("X")}}}},
		{HasBaseline: true, Files: []DriftItem{{Path: "b.go", Status: DriftModified, Symbols: []SymbolDriftItem{tdRemoved("Y")}}}},
		{HasBaseline: true, Files: []DriftItem{{Path: "c.go", Status: DriftMissing}}},
		{HasBaseline: true, Files: []DriftItem{
			{Path: "a.go", Status: DriftAdded},
			{Path: "b.go", Status: DriftModified, Symbols: []SymbolDriftItem{tdModified("Z")}},
		}},
	}
	for _, d := range ClassifyAutoBaseline(governed) {
		if d.Action == AutoResolveSilent {
			t.Fatalf("SAFETY VIOLATION: governed_modified report mapped to AutoResolveSilent: %+v", d.Report)
		}
	}
}

// TestClassifyAutoBaseline_NoBaselineFailsSafe: no baseline => cannot prove
// benign => surface, regardless of file shape.
func TestClassifyAutoBaseline_NoBaselineFailsSafe(t *testing.T) {
	r := DriftReport{HasBaseline: false, Files: []DriftItem{{Path: "a.go", Status: DriftAdded}}}
	got := ClassifyAutoBaseline([]DriftReport{r})
	if got[0].Action != SurfaceForReview {
		t.Fatalf("Action = %q, want %q", got[0].Action, SurfaceForReview)
	}
}

func TestClassifyAutoBaseline_MaterialityMapping(t *testing.T) {
	cases := []struct {
		name string
		file DriftItem
		want AutoBaselineAction
	}{
		{
			name: "adjacent churn resolves silently",
			file: DriftItem{Path: "shared.go", Status: DriftModified, Materiality: DriftMaterialityAdjacentFileChurn},
			want: AutoResolveSilent,
		},
		{
			name: "carrier churn resolves silently",
			file: DriftItem{Path: "CHANGELOG.md", Status: DriftModified, Materiality: DriftMaterialityCarrierOnly},
			want: AutoResolveSilent,
		},
		{
			name: "generated churn resolves silently",
			file: DriftItem{Path: "internal/cli/fpf.db", Status: DriftModified, Materiality: DriftMaterialityGeneratedOrIgnored},
			want: AutoResolveSilent,
		},
		{
			name: "unknown legacy still reviews",
			file: DriftItem{Path: "shared.go", Status: DriftModified, Materiality: DriftMaterialityUnknownLegacyFileScope},
			want: SurfaceForReview,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report := DriftReport{HasBaseline: true, Files: []DriftItem{tc.file}}
			got := ClassifyAutoBaseline([]DriftReport{report})
			if got[0].Action != tc.want {
				t.Fatalf("Action = %q, want %q", got[0].Action, tc.want)
			}
		})
	}
}

// TestClassifyAutoBaseline_CorpusReplay models this session's drift corpus shape
// (a mix of additive churn, freshly-shipped governed changes, and one seeded
// breakage) and asserts the safety invariant holds across the whole set: no
// governed change is ever in the silent bucket.
func TestClassifyAutoBaseline_CorpusReplay(t *testing.T) {
	corpus := []DriftReport{}
	// 11 "additive / shared-file churn" decisions -> silent
	for range 11 {
		corpus = append(corpus, DriftReport{
			HasBaseline: true,
			Files:       []DriftItem{{Path: "shared.go", Status: DriftModified, Symbols: []SymbolDriftItem{tdAdded("New")}}},
		})
	}
	// 13 "freshly-shipped governed" decisions -> stage for confirm
	for range 13 {
		corpus = append(corpus, DriftReport{
			HasBaseline: true,
			Files:       []DriftItem{{Path: "impl.go", Status: DriftModified, Symbols: []SymbolDriftItem{tdModified("Impl")}}},
		})
	}
	// 1 seeded true breakage (removed governed func) -> stage for confirm
	corpus = append(corpus, DriftReport{
		HasBaseline: true,
		Files:       []DriftItem{{Path: "harness.go", Status: DriftModified, Symbols: []SymbolDriftItem{tdRemoved("canApplyHarnessWorkspaceDiff")}}},
	})

	silent, confirm, review := PartitionDispositions(ClassifyAutoBaseline(corpus))

	if len(silent) != 11 {
		t.Fatalf("silent bucket = %d, want 11", len(silent))
	}
	if len(confirm) != 14 {
		t.Fatalf("confirm bucket = %d, want 14", len(confirm))
	}
	if len(review) != 0 {
		t.Fatalf("review bucket = %d, want 0", len(review))
	}
	// Safety invariant: nothing in the silent bucket is a governed change.
	for _, d := range silent {
		if d.Report.SymbolVerdict() == SymbolVerdictGovernedModified {
			t.Fatalf("SAFETY VIOLATION: governed change in silent bucket")
		}
	}
}
