package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateDirectoryStructure_CreatesWorkflowExample(t *testing.T) {
	haftDir := filepath.Join(t.TempDir(), ".haft")

	if err := createDirectoryStructure(haftDir); err != nil {
		t.Fatalf("createDirectoryStructure returned error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(haftDir, "workflow.md"))
	if err != nil {
		t.Fatalf("read workflow example: %v", err)
	}

	if !strings.Contains(string(data), "## Defaults") {
		t.Fatalf("workflow example missing Defaults section:\n%s", string(data))
	}
}
