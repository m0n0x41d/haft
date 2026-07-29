package profiledetector

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDetectorClassifiesRequiredRepositoryFixturesWithoutBinding(t *testing.T) {
	tests := []struct {
		name           string
		files          []string
		classification Classification
		confidence     ConfidencePosture
		orientations   []string
	}{
		{
			name:           "software",
			files:          []string{"go.mod", "internal/kernel.go", "README.md"},
			classification: SoftwareSignals,
			confidence:     SupportedConfidence,
			orientations:   []string{"software"},
		},
		{
			name:           "documents",
			files:          []string{"mkdocs.yml", "docs/one.md", "docs/two.md"},
			classification: NonSoftwareSignals,
			confidence:     SupportedConfidence,
			orientations:   []string{"documents"},
		},
		{
			name:           "documents_with_helper_code",
			files:          []string{"mkdocs.yml", "docs/one.md", "scripts/build.py"},
			classification: NonSoftwareSignals,
			confidence:     SupportedConfidence,
			orientations:   []string{"documents"},
		},
		{
			name:           "mixed_software_and_model",
			files:          []string{"go.mod", "internal/kernel.go", "models/current.onnx"},
			classification: MixedSignals,
			confidence:     ConflictingConfidence,
			orientations:   []string{"models", "software"},
		},
		{
			name:           "insufficient",
			files:          []string{"README.md", "scripts/check.py"},
			classification: InsufficientDetectorBasis,
			confidence:     InsufficientConfidence,
			orientations:   []string{},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := mustPhysicalTempDir(t)
			snapshot, err := NewSnapshot(root, test.files, len(test.files), false)
			if err != nil {
				t.Fatal(err)
			}
			result := Detect(snapshot)
			if result.Classification() != test.classification {
				t.Fatalf("classification = %q, want %q", result.Classification(), test.classification)
			}
			if result.ConfidencePosture() != test.confidence {
				t.Fatalf("confidence = %q, want %q", result.ConfidencePosture(), test.confidence)
			}
			gotOrientations := orientations(result.SuggestedScopes())
			if !reflect.DeepEqual(gotOrientations, test.orientations) {
				t.Fatalf("orientations = %#v, want %#v", gotOrientations, test.orientations)
			}
		})
	}
}

func TestDetectorTruncationCannotProduceAProfileSuggestion(t *testing.T) {
	root := mustPhysicalTempDir(t)
	files := []string{"go.mod", "internal/kernel.go"}
	snapshot, err := NewSnapshot(root, files, len(files)+1, true)
	if err != nil {
		t.Fatal(err)
	}
	result := Detect(snapshot)
	if result.Classification() != InsufficientDetectorBasis {
		t.Fatalf("truncated classification = %q", result.Classification())
	}
	if len(result.SuggestedScopes()) != 0 {
		t.Fatalf("truncated scan suggested scopes: %#v", result.SuggestedScopes())
	}
}

func TestInspectSkipsHaftGitDependenciesAndSymlinks(t *testing.T) {
	root := mustPhysicalTempDir(t)
	writeDetectorFile(t, root, "go.mod")
	writeDetectorFile(t, root, "internal/kernel.go")
	writeDetectorFile(t, root, ".haft/specs/software-system.md")
	writeDetectorFile(t, root, "node_modules/tool/index.js")
	outside := filepath.Join(mustPhysicalTempDir(t), "outside.onnx")
	if err := os.WriteFile(outside, []byte("model"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "model.onnx")); err != nil {
		t.Fatal(err)
	}
	result, err := Inspect(root)
	if err != nil {
		t.Fatal(err)
	}
	if result.Classification() != SoftwareSignals {
		t.Fatalf("classification = %q", result.Classification())
	}
	files := result.Snapshot().RelativeFiles()
	want := []string{"go.mod", "internal/kernel.go"}
	if !reflect.DeepEqual(files, want) {
		t.Fatalf("observed files = %#v, want %#v", files, want)
	}
}

func orientations(scopes []SuggestedScope) []string {
	result := make([]string, len(scopes))
	for index, scope := range scopes {
		result[index] = scope.Orientation()
	}
	return result
}

func mustPhysicalTempDir(t *testing.T) string {
	t.Helper()
	path := t.TempDir()
	physical, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return physical
}

func writeDetectorFile(t *testing.T, root string, relative string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("fixture"), 0o644); err != nil {
		t.Fatal(err)
	}
}
