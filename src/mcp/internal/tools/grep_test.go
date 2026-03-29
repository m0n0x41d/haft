package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGrepBasicMatch(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "hello.go", "package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n")
	writeTestFile(t, dir, "other.txt", "no match here\n")

	tool := &GrepTool{projectRoot: dir}
	args := mustJSON(t, map[string]any{"pattern": "main", "output_mode": "content"})
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.DisplayText, "hello.go") {
		t.Fatalf("expected match in hello.go, got: %s", result.DisplayText)
	}
	if strings.Contains(result.DisplayText, "other.txt") {
		t.Fatalf("unexpected match in other.txt")
	}
}

func TestGrepFilesWithMatches(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "a.go", "func foo() {}\n")
	writeTestFile(t, dir, "b.go", "func bar() {}\n")
	writeTestFile(t, dir, "c.txt", "nothing here\n")

	tool := &GrepTool{projectRoot: dir}
	args := mustJSON(t, map[string]any{"pattern": "func", "output_mode": "files_with_matches"})
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(result.DisplayText), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 files, got %d: %s", len(lines), result.DisplayText)
	}
}

func TestGrepCount(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "multi.go", "func a() {}\nfunc b() {}\nfunc c() {}\n")

	tool := &GrepTool{projectRoot: dir}
	args := mustJSON(t, map[string]any{"pattern": "func", "output_mode": "count"})
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.DisplayText, ":3") {
		t.Fatalf("expected count of 3, got: %s", result.DisplayText)
	}
}

func TestGrepGlobFilter(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "match.go", "func hello() {}\n")
	writeTestFile(t, dir, "match.txt", "func hello() {}\n")

	tool := &GrepTool{projectRoot: dir}
	args := mustJSON(t, map[string]any{"pattern": "func", "glob": "*.go", "output_mode": "files_with_matches"})
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result.DisplayText, ".txt") {
		t.Fatalf("glob filter should exclude .txt files: %s", result.DisplayText)
	}
}

func TestGrepNoMatch(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "empty.go", "package main\n")

	tool := &GrepTool{projectRoot: dir}
	args := mustJSON(t, map[string]any{"pattern": "nonexistent_xyz"})
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.DisplayText, "No matches") {
		t.Fatalf("expected no matches message, got: %s", result.DisplayText)
	}
}

func TestGrepCaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "test.go", "Hello World\nhello world\nHELLO WORLD\n")

	tool := &GrepTool{projectRoot: dir}
	args := mustJSON(t, map[string]any{"pattern": "hello", "-i": true, "output_mode": "count"})
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.DisplayText, ":3") {
		t.Fatalf("expected 3 case-insensitive matches, got: %s", result.DisplayText)
	}
}

func TestGlobBasic(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "a.go", "")
	writeTestFile(t, dir, "b.go", "")
	writeTestFile(t, dir, "c.txt", "")

	tool := &GlobTool{projectRoot: dir}
	args := mustJSON(t, map[string]any{"pattern": "*.go"})
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(result.DisplayText), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 .go files, got %d: %s", len(lines), result.DisplayText)
	}
}

func TestGlobNoMatch(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "a.txt", "")

	tool := &GlobTool{projectRoot: dir}
	args := mustJSON(t, map[string]any{"pattern": "*.rs"})
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.DisplayText, "No files matched") {
		t.Fatalf("expected no match message, got: %s", result.DisplayText)
	}
}

// --- helpers ---

func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
