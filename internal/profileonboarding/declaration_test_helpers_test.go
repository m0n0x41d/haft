package profileonboarding

import (
	"path/filepath"
	"testing"

	"github.com/m0n0x41d/haft/internal/profiledetector"
)

func workInputTestSuggestion(
	t testing.TB,
	root string,
	files []string,
) profiledetector.Suggestion {
	t.Helper()
	snapshot, err := profiledetector.NewSnapshot(root, files, len(files), false)
	if err != nil {
		t.Fatal(err)
	}
	return profiledetector.Detect(snapshot)
}

func canonicalWorkInputTestRoot(t testing.TB) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}
