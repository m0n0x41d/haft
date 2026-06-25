package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSpecRuntimeReadPathsDoNotBypassSQLEditionSource(t *testing.T) {
	root := testProjectRoot(t)
	cliDir := filepath.Join(root, "internal", "cli")

	allowed := map[string]map[string]string{
		"internal/cli/spec_read.go": {
			"project.LoadProjectSpecificationSet(": "SQL-first helper may read carriers only for compatibility fallback and term-map support",
		},
		"internal/cli/spec_sync.go": {
			"project.LoadProjectSpecificationSet(": "spec sync is the explicit carrier import path into SQL editions",
		},
		"internal/cli/spec_classify_change.go": {
			"project.SpecSectionsFromDocuments(": "spec classify-change parses explicit before/after carrier files as read-only review input",
		},
		"internal/cli/overseer.go": {
			"project.SpecSectionsFromDocuments(": "overseer parses the changed carrier file as review-scoping input, not as the runtime spec source of truth",
		},
	}
	tokens := []string{
		"project.LoadProjectSpecificationSet(",
		"project.ProjectSpecificationSetFromDocuments(",
		"project.SpecSectionsFromDocuments(",
	}

	entries, err := os.ReadDir(cliDir)
	if err != nil {
		t.Fatalf("read %s: %v", cliDir, err)
	}
	seenAllowed := map[string]map[string]bool{}
	for path, pathAllowed := range allowed {
		seenAllowed[path] = map[string]bool{}
		for token := range pathAllowed {
			seenAllowed[path][token] = false
		}
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}

		absPath := filepath.Join(cliDir, entry.Name())
		relPath := filepath.ToSlash(filepath.Join("internal", "cli", entry.Name()))
		content := readSpecRuntimeReadPathSource(t, absPath)
		for _, token := range tokens {
			if !strings.Contains(content, token) {
				continue
			}

			reason, ok := allowed[relPath][token]
			if !ok {
				t.Fatalf("%s bypasses SQL-first SpecSection reads via %q; use loadProjectSpecificationSetSQLFirst or add an explicit carrier-only exception", relPath, token)
			}
			if strings.TrimSpace(reason) == "" {
				t.Fatalf("%s allowlist for %q must explain the carrier boundary", relPath, token)
			}
			seenAllowed[relPath][token] = true
		}
	}

	for relPath, tokenSeen := range seenAllowed {
		for token, seen := range tokenSeen {
			if !seen {
				t.Fatalf("stale SQL-first read-path allowlist: %s no longer contains %q", relPath, token)
			}
		}
	}
}

func TestLoadProjectSpecificationSetFromSQLEditionsPropagatesStoreOpenFailure(t *testing.T) {
	root := setupSpecSyncProject(t)
	homeFile := filepath.Join(t.TempDir(), "home-file")
	if err := os.WriteFile(homeFile, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("write home file: %v", err)
	}
	t.Setenv("HOME", homeFile)

	_, _, err := loadProjectSpecificationSetFromSQLEditions(root)
	if err == nil {
		t.Fatal("expected SQL edition store open failure")
	}
	if !strings.Contains(err.Error(), "create project DB dir") {
		t.Fatalf("error = %v, want DB dir failure", err)
	}
}

func readSpecRuntimeReadPathSource(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
