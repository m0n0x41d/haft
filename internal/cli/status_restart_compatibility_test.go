//go:build darwin || linux

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStatusReportsPredecessorRestartCheckpointWithoutFailing(t *testing.T) {
	root := t.TempDir()
	haftDirectory := filepath.Join(root, ".haft")
	restartDirectory := filepath.Join(haftDirectory, "restart")
	if err := os.Mkdir(haftDirectory, 0o700); err != nil {
		t.Fatalf("create .haft: %v", err)
	}
	if err := os.Mkdir(restartDirectory, 0o700); err != nil {
		t.Fatalf("create restart directory: %v", err)
	}
	writePrivateTestFile(
		t,
		filepath.Join(restartDirectory, ".gitignore"),
		[]byte("*\n!.gitignore\n"),
	)
	writePrivateTestFile(
		t,
		filepath.Join(restartDirectory, ".checkpoint.lock"),
		nil,
	)
	checkpointPath := filepath.Join(restartDirectory, "checkpoint.json")
	predecessor := []byte(`{"goal_objective_digest":"","goal_resume_count":0}`)
	writePrivateTestFile(t, checkpointPath, predecessor)

	response, err := statusWithLiveMCPReceipt(root, "Project status: available")
	if err != nil {
		t.Fatalf("statusWithLiveMCPReceipt: %v", err)
	}
	if !strings.Contains(response, "stale goal-coupled checkpoint ignored") {
		t.Fatalf("response did not expose stale checkpoint diagnostic: %q", response)
	}
	after, err := os.ReadFile(checkpointPath)
	if err != nil {
		t.Fatalf("read predecessor checkpoint: %v", err)
	}
	if string(after) != string(predecessor) {
		t.Fatal("status changed predecessor checkpoint bytes")
	}
}

func writePrivateTestFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write %s: %v", filepath.Base(path), err)
	}
}
