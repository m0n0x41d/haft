package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestExactRefreshToolRevisionTracksCanonicalInputClosure(t *testing.T) {
	root := newRefreshToolRevisionFixture(t)
	runRefreshToolRevisionGit(t, root, "init", "--quiet")
	runRefreshToolRevisionGit(t, root, "config", "user.name", "Tool Revision Test")
	runRefreshToolRevisionGit(t, root, "config", "user.email", "tool-revision@example.invalid")
	runRefreshToolRevisionGit(t, root, "add", ".")
	runRefreshToolRevisionGit(t, root, "commit", "--quiet", "-m", "initial")

	initial := mustExactRefreshToolRevision(t, root)
	if !strings.HasPrefix(initial, refreshToolRevisionPrefix) ||
		strings.Contains(initial, "git:") {
		t.Fatalf("tool revision = %q, want content-only revision prefix", initial)
	}
	if dirty := runRefreshToolRevisionGit(
		t,
		root,
		"status",
		"--porcelain",
		"--",
		"go.mod",
		"go.sum",
	); dirty != "" {
		t.Fatalf("read-only identity derivation changed module files: %s", dirty)
	}

	runRefreshToolRevisionGit(t, root, "commit", "--quiet", "--allow-empty", "-m", "move HEAD only")
	if afterHEAD := mustExactRefreshToolRevision(t, root); afterHEAD != initial {
		t.Fatalf("HEAD-only change altered revision: before=%s after=%s", initial, afterHEAD)
	}

	writeRefreshToolRevisionFixtureFile(
		t,
		root,
		"internal/dependency/value_test.go",
		"package dependency\n\nconst testOnly = \"changed\"\n",
	)
	writeRefreshToolRevisionFixtureFile(
		t,
		root,
		"data/haft/fpf-integration.lock.json",
		"derived lock bytes\n",
	)
	writeRefreshToolRevisionFixtureFile(
		t,
		root,
		"internal/cli/fpf.db",
		"derived database bytes\n",
	)
	writeRefreshToolRevisionFixtureFile(
		t,
		root,
		"data/FPF/FPF-Spec.md",
		"source publication bytes\n",
	)
	if afterExcluded := mustExactRefreshToolRevision(t, root); afterExcluded != initial {
		t.Fatalf("test/source/derived inputs altered revision: before=%s after=%s", initial, afterExcluded)
	}

	writeRefreshToolRevisionFixtureFile(
		t,
		root,
		"internal/dependency/value.go",
		"package dependency\n\nimport _ \"embed\"\n\n//go:embed payload.txt\nvar payload string\n\nconst Value = \"two\"\n",
	)
	dependencyChanged := mustExactRefreshToolRevision(t, root)
	if dependencyChanged == initial {
		t.Fatal("transitive dependency content did not alter tool revision")
	}

	writeRefreshToolRevisionFixtureFile(
		t,
		root,
		"internal/dependency/payload.txt",
		"embedded payload two\n",
	)
	embedChanged := mustExactRefreshToolRevision(t, root)
	if embedChanged == dependencyChanged {
		t.Fatal("transitive embedded content did not alter tool revision")
	}

	writeRefreshToolRevisionFixtureFile(
		t,
		root,
		"internal/cdependency/fixture.c",
		"int fixture_value(void) { return 2; }\n",
	)
	if cChanged := mustExactRefreshToolRevision(t, root); cChanged == embedChanged {
		t.Fatal("transitive C content did not alter tool revision")
	}
}

func TestExactRefreshToolRevisionIsLocationAndAmbientPlatformIndependent(t *testing.T) {
	firstRoot := newRefreshToolRevisionFixture(t)
	secondRoot := newRefreshToolRevisionFixture(t)
	first := mustExactRefreshToolRevision(t, firstRoot)
	second := mustExactRefreshToolRevision(t, secondRoot)
	if first != second {
		t.Fatalf("absolute fixture location altered revision: first=%s second=%s", first, second)
	}

	t.Setenv("GOOS", "windows")
	t.Setenv("GOARCH", "386")
	t.Setenv("GOFLAGS", "-tags=ambient_must_not_participate")
	t.Setenv("GOWORK", filepath.Join(t.TempDir(), "ambient.work"))
	if ambient := mustExactRefreshToolRevision(t, firstRoot); ambient != first {
		t.Fatalf("ambient Go build context altered revision: before=%s after=%s", first, ambient)
	}
}

func TestAddRefreshToolInputRejectsConflictingLogicalBytes(t *testing.T) {
	directory := t.TempDir()
	firstPath := filepath.Join(directory, "first.go")
	secondPath := filepath.Join(directory, "second.go")
	if err := os.WriteFile(firstPath, []byte("first"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secondPath, []byte("second"), 0o600); err != nil {
		t.Fatal(err)
	}
	inputs := make(map[string]refreshToolInput)
	const logicalName = "go/example.test/dependency/value.go"
	if err := addRefreshToolInput(inputs, logicalName, firstPath); err != nil {
		t.Fatal(err)
	}
	err := addRefreshToolInput(inputs, logicalName, secondPath)
	if err == nil || !strings.Contains(err.Error(), "different bytes") {
		t.Fatalf("duplicate logical input error = %v", err)
	}
}

func newRefreshToolRevisionFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"go.mod":                                         "module example.test/refreshfixture\n\ngo 1.25.8\n",
		"go.sum":                                         "",
		"cmd/fpf-refresh/main.go":                        "package main\n\nimport (\n\t_ \"example.test/refreshfixture/internal/cdependency\"\n\t_ \"example.test/refreshfixture/internal/dependency\"\n)\n\nfunc main() {}\n",
		"cmd/indexer/main.go":                            "package main\n\nimport _ \"example.test/refreshfixture/internal/dependency\"\n\nfunc main() {}\n",
		"internal/dependency/value.go":                   "package dependency\n\nimport _ \"embed\"\n\n//go:embed payload.txt\nvar payload string\n\nconst Value = \"one\"\n",
		"internal/dependency/value_test.go":              "package dependency\n\nconst testOnly = \"one\"\n",
		"internal/dependency/payload.txt":                "embedded payload one\n",
		"internal/cdependency/value.go":                  "package cdependency\n\n/*\nint fixture_value(void);\n*/\nimport \"C\"\n\nvar Value = C.fixture_value()\n",
		"internal/cdependency/fixture.c":                 "int fixture_value(void) { return 1; }\n",
		"scripts/fpf_query_token_gate.sh":                "#!/bin/sh\nexit 0\n",
		"scripts/fpf_query_token_count.py":               "print(0)\n",
		"scripts/fpf_query_token_count.requirements.txt": "tokenizer==1.0\n",
	}
	for relative, content := range files {
		writeRefreshToolRevisionFixtureFile(t, root, relative, content)
	}
	return root
}

func writeRefreshToolRevisionFixtureFile(
	t *testing.T,
	root string,
	relative string,
	content string,
) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustExactRefreshToolRevision(t *testing.T, root string) string {
	t.Helper()
	revision, err := exactRefreshToolRevision(context.Background(), root)
	if err != nil {
		t.Fatalf("exactRefreshToolRevision() error = %v", err)
	}
	return revision
}

func runRefreshToolRevisionGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}
