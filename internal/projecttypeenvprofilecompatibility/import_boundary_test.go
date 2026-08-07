package projecttypeenvprofilecompatibility_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

var forbiddenProductionImports = map[string]struct{}{
	"github.com/m0n0x41d/haft/internal/projectmemory/goldenconcernbundle": {},
	"github.com/m0n0x41d/haft/internal/projectmemory/neighborhood":        {},
	"github.com/m0n0x41d/haft/internal/typedmemorystore":                  {},
}

func TestProductionCompatibilityBoundaryDoesNotImportReadRuntime(t *testing.T) {
	directory := compatibilityPackageDirectory(t)
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("read compatibility package directory: %v", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(directory, name)
		file, parseErr := parser.ParseFile(
			token.NewFileSet(),
			path,
			nil,
			parser.ImportsOnly,
		)
		if parseErr != nil {
			t.Fatalf("parse production file %q: %v", name, parseErr)
		}
		for _, imported := range file.Imports {
			pathValue, unquoteErr := strconv.Unquote(imported.Path.Value)
			if unquoteErr != nil {
				t.Fatalf("decode import in %q: %v", name, unquoteErr)
			}
			if _, forbidden := forbiddenProductionImports[pathValue]; forbidden {
				t.Fatalf("production compatibility file %q imports upper read-runtime package %q", name, pathValue)
			}
		}
	}
}

func compatibilityPackageDirectory(t *testing.T) string {
	t.Helper()
	_, file, _, found := runtime.Caller(0)
	if !found {
		t.Fatal("resolve compatibility boundary test location")
	}
	return filepath.Dir(file)
}
