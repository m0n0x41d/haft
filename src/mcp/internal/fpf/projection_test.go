package fpf

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"quint-mcp/db"
)

var ctx = context.Background()

func TestComputeContentHash(t *testing.T) {
	body := "test content"
	hash := ComputeContentHash(body)
	if hash == "" {
		t.Error("ComputeContentHash returned empty hash")
	}
	if len(hash) != 32 {
		t.Errorf("Expected 32 char hash, got %d", len(hash))
	}

	hash2 := ComputeContentHash(body)
	if hash != hash2 {
		t.Error("Same content should produce same hash")
	}

	hash3 := ComputeContentHash("different content")
	if hash == hash3 {
		t.Error("Different content should produce different hash")
	}
}

func TestWriteWithHash(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "test.md")

	fields := map[string]string{
		"scope": "global",
		"kind":  "system",
	}
	body := "\n# Test\n\nContent here"

	err := WriteWithHash(path, fields, body)
	if err != nil {
		t.Fatalf("WriteWithHash failed: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	contentStr := string(content)
	if !containsString(contentStr, "content_hash:") {
		t.Error("File should contain content_hash in frontmatter")
	}
	if !containsString(contentStr, "scope: global") {
		t.Error("File should contain scope field")
	}
	if !containsString(contentStr, "kind: system") {
		t.Error("File should contain kind field")
	}
	if !containsString(contentStr, "# Test") {
		t.Error("File should contain body content")
	}
}

func TestValidateFile_Valid(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "valid.md")

	fields := map[string]string{"scope": "test"}
	body := "\n# Valid Content\n\nThis is valid."

	if err := WriteWithHash(path, fields, body); err != nil {
		t.Fatalf("WriteWithHash failed: %v", err)
	}

	content, tampered, _, _, err := ValidateFile(path)
	if err != nil {
		t.Fatalf("ValidateFile failed: %v", err)
	}
	if tampered {
		t.Error("Untampered file should not be detected as tampered")
	}
	if content == "" {
		t.Error("ValidateFile should return content")
	}
}

func TestValidateFile_Tampered(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "tampered.md")

	fields := map[string]string{"scope": "test"}
	body := "\n# Original Content"

	if err := WriteWithHash(path, fields, body); err != nil {
		t.Fatalf("WriteWithHash failed: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	tamperedContent := string(content) + "\n\n## TAMPERED ADDITION"
	if err := os.WriteFile(path, []byte(tamperedContent), 0644); err != nil {
		t.Fatalf("Failed to write tampered content: %v", err)
	}

	_, tampered, expectedHash, actualHash, err := ValidateFile(path)
	if err != nil {
		t.Fatalf("ValidateFile failed: %v", err)
	}
	if !tampered {
		t.Error("Tampered file should be detected")
	}
	if expectedHash == actualHash {
		t.Error("Hashes should be different for tampered file")
	}
}

func TestValidateFile_NoFrontmatter(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "no-frontmatter.md")

	if err := os.WriteFile(path, []byte("# Just content\n\nNo frontmatter here."), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	content, tampered, _, _, err := ValidateFile(path)
	if err != nil {
		t.Fatalf("ValidateFile failed: %v", err)
	}
	if tampered {
		t.Error("File without frontmatter should not be detected as tampered")
	}
	if content == "" {
		t.Error("Should return content")
	}
}

func TestValidateFile_NoHash(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "no-hash.md")

	content := "---\nscope: test\n---\n\n# Content"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	_, tampered, _, _, err := ValidateFile(path)
	if err != nil {
		t.Fatalf("ValidateFile failed: %v", err)
	}
	if tampered {
		t.Error("File without hash should not be detected as tampered (legacy file)")
	}
}

func TestReadWithValidation_Tampered(t *testing.T) {
	tempDir := t.TempDir()
	quintDir := filepath.Join(tempDir, ".quint")
	l0Dir := filepath.Join(quintDir, "knowledge", "L0")
	os.MkdirAll(l0Dir, 0755)

	dbPath := filepath.Join(quintDir, "quint.db")
	store, err := db.NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	fsm := &FSM{State: State{Phase: PhaseAbduction}}
	tools := NewTools(fsm, tempDir, store)

	path := filepath.Join(l0Dir, "test-hypo.md")
	body := "\n# Hypothesis: Test\n\nOriginal content"
	fields := map[string]string{"scope": "test", "kind": "system"}
	if err := WriteWithHash(path, fields, body); err != nil {
		t.Fatalf("WriteWithHash failed: %v", err)
	}

	if err := store.CreateHolon(ctx, "test-hypo", "hypothesis", "system", "L0", "Test", body, "default", "test", ""); err != nil {
		t.Fatalf("CreateHolon failed: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	tamperedContent := string(content) + "\n\n## TAMPERED"
	if err := os.WriteFile(path, []byte(tamperedContent), 0644); err != nil {
		t.Fatalf("Failed to tamper file: %v", err)
	}

	_, event, err := tools.ReadWithValidation(path)
	if err != nil {
		t.Fatalf("ReadWithValidation failed: %v", err)
	}

	if event == nil {
		t.Fatal("Should have detected tampering event")
	}
	if event.FilePath != path {
		t.Errorf("Expected path %s, got %s", path, event.FilePath)
	}
	if event.ExpectedHash == event.ActualHash {
		t.Error("Hashes should be different")
	}
}

func TestExtractHolonIDFromPath(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/foo/.quint/knowledge/L0/test-hypo.md", "test-hypo"},
		{"/foo/.quint/knowledge/L1/another.md", "another"},
		{"/foo/.quint/knowledge/L2/final.md", "final"},
		{"/foo/.quint/knowledge/invalid/bad.md", "bad"},
		{"/foo/.quint/decisions/DRR-123.md", ""},
		{"/foo/other/file.md", ""},
	}

	for _, tt := range tests {
		result := extractHolonIDFromPath(tt.path)
		if result != tt.expected {
			t.Errorf("extractHolonIDFromPath(%q) = %q, want %q", tt.path, result, tt.expected)
		}
	}
}

func TestExtractLayerFromPath(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/foo/.quint/knowledge/L0/test.md", "L0"},
		{"/foo/.quint/knowledge/L1/test.md", "L1"},
		{"/foo/.quint/knowledge/L2/test.md", "L2"},
		{"/foo/.quint/knowledge/invalid/test.md", "invalid"},
		{"/foo/.quint/decisions/DRR.md", ""},
	}

	for _, tt := range tests {
		result := extractLayerFromPath(tt.path)
		if result != tt.expected {
			t.Errorf("extractLayerFromPath(%q) = %q, want %q", tt.path, result, tt.expected)
		}
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
