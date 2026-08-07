package initialprofilebootstrap

import (
	"path/filepath"
	"testing"

	"github.com/m0n0x41d/haft/internal/profiledetector"
)

func TestDecideAppliesOnlySupportedSingletonWithoutCurrentProfile(t *testing.T) {
	suggestion := detectorSuggestion(t, []string{"go.mod", "internal/core.go"}, false)
	decision, err := Decide(false, ReviewAbsent, suggestion)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Kind() != ApplySupportedSingleton {
		t.Fatalf("kind = %q", decision.Kind())
	}
	scope, ok := decision.SupportedSingleton()
	if !ok || scope.Orientation() != "software" {
		t.Fatalf("supported singleton = %#v, %v", scope, ok)
	}
}

func TestDecideKeepsExistingProfileBeforeConsideringDetector(t *testing.T) {
	suggestion := detectorSuggestion(t, []string{"go.mod", "internal/core.go"}, false)
	decision, err := Decide(true, ReviewHumanOrForeign, suggestion)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Kind() != KeepExisting {
		t.Fatalf("kind = %q", decision.Kind())
	}
}

func TestDecideRoutesAmbiguousAndHumanReviewCasesToHuman(t *testing.T) {
	tests := []struct {
		name      string
		files     []string
		truncated bool
		review    ReviewDisposition
		reason    ReviewReason
	}{
		{
			name:   "mixed",
			files:  []string{"go.mod", "internal/core.go", "models/current.onnx"},
			review: ReviewAbsent,
			reason: ReviewReasonUnsupportedConfidence,
		},
		{
			name:      "truncated",
			files:     []string{"go.mod", "internal/core.go"},
			truncated: true,
			review:    ReviewAbsent,
			reason:    ReviewReasonTruncatedObservation,
		},
		{
			name:   "human review",
			files:  []string{"go.mod", "internal/core.go"},
			review: ReviewHumanOrForeign,
			reason: ReviewReasonHumanOrForeignReview,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			suggestion := detectorSuggestion(t, test.files, test.truncated)
			decision, err := Decide(false, test.review, suggestion)
			if err != nil {
				t.Fatal(err)
			}
			if decision.Kind() != HumanReviewRequired {
				t.Fatalf("kind = %q", decision.Kind())
			}
			reason, ok := decision.ReviewReason()
			if !ok || reason != test.reason {
				t.Fatalf("reason = %q, %v", reason, ok)
			}
		})
	}
}

func detectorSuggestion(
	t *testing.T,
	files []string,
	truncated bool,
) profiledetector.Suggestion {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	scanned := len(files)
	if truncated {
		scanned++
	}
	snapshot, err := profiledetector.NewSnapshot(root, files, scanned, truncated)
	if err != nil {
		t.Fatal(err)
	}
	return profiledetector.Detect(snapshot)
}
