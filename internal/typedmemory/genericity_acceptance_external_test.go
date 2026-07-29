package typedmemory_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectmemory/evidenceworkadapter"
	"github.com/m0n0x41d/haft/internal/projectmemory/problemcardadapter"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const haftModulePath = "github.com/m0n0x41d/haft"

var forbiddenKernelImportRoots = []string{
	haftModulePath + "/internal/decisionbinding",
	haftModulePath + "/internal/fpf",
	haftModulePath + "/internal/profileadmission",
	haftModulePath + "/internal/profileauthority",
	haftModulePath + "/internal/profiledetector",
	haftModulePath + "/internal/profileonboarding",
	haftModulePath + "/internal/profileprojection",
	haftModulePath + "/internal/projectmemory",
	haftModulePath + "/internal/projectprofile",
	haftModulePath + "/internal/projecttypeenvprofilebasis",
	haftModulePath + "/internal/projecttypeenvprofilecompatibility",
	haftModulePath + "/internal/projecttypeenvprofilefit",
	haftModulePath + "/internal/specmigrationv2",
	haftModulePath + "/internal/workcommission",
}

var forbiddenKernelLiteralTokens = []string{
	"ProblemCard",
	"SolutionPortfolio",
	"PortfolioComparison",
	"DecisionRecord",
	"WorkCommission",
	"SpecSection",
	"SoftwareSystemSpec",
	"TargetSystemSpec",
	"ProjectionProfile",
	"ProjectProfile",
	"problem_card",
	"solution_portfolio",
	"portfolio_comparison",
	"decision_record",
	"work_commission",
	"spec_section",
	"software_system_spec",
	"target_system_spec",
	"projection_profile",
	"project_profile",
}

var _ func(typedmemory.ClaimGraphValue) (
	problemcardadapter.ExactClaimGraph,
	error,
) = problemcardadapter.NewExactClaimGraph

var _ func(typedmemory.ClaimGraphValue) (
	evidenceworkadapter.ExactClaimGraph,
	error,
) = evidenceworkadapter.NewExactClaimGraph

func TestGenericKernelHasNoHaftProductCarrierDependency(t *testing.T) {
	kernelDirectory := typedMemoryKernelDirectory(t)
	entries, err := os.ReadDir(kernelDirectory)
	if err != nil {
		t.Fatalf("read typed-memory kernel directory: %v", err)
	}

	fileSet := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") ||
			strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(kernelDirectory, name)
		file, parseErr := parser.ParseFile(fileSet, path, nil, 0)
		if parseErr != nil {
			t.Fatalf("parse typed-memory kernel source %s: %v", name, parseErr)
		}
		assertNoProductImports(t, fileSet, file)
		assertNoProductCarrierLiterals(t, fileSet, file)
	}
}

func TestIndependentProductAdaptersConsumeGenericKernelPublicTypes(t *testing.T) {
	graph, err := typedmemory.NewClaimGraphValue(nil, nil)
	if err != nil {
		t.Fatalf("construct generic ClaimGraphValue: %v", err)
	}

	problemGraph, err := problemcardadapter.NewExactClaimGraph(graph)
	if err != nil {
		t.Fatalf("ProblemCard adapter consume generic ClaimGraphValue: %v", err)
	}
	evidenceGraph, err := evidenceworkadapter.NewExactClaimGraph(graph)
	if err != nil {
		t.Fatalf("Evidence/Work adapter consume generic ClaimGraphValue: %v", err)
	}

	assertSameClaimGraph(t, "ProblemCard", graph, problemGraph.Value())
	assertSameClaimGraph(t, "Evidence/Work", graph, evidenceGraph.Value())
}

func TestGenericityGuardTargetsExactProductVocabulary(t *testing.T) {
	for _, generic := range []string{
		"specification",
		"profile",
		"decision",
		"commission",
		"a generic problem card description",
		"haft.typedmemory.claim-graph-codec.v1",
	} {
		if forbiddenKernelLiteral(generic) {
			t.Errorf("generic vocabulary %q was treated as a product carrier literal", generic)
		}
	}
	for _, exact := range []string{
		"Haft.NoteAtConcern",
		"DecisionRecord",
		"payload.projection_profile_ref",
		"SoftwareSystemSpec carrier",
	} {
		if !forbiddenKernelLiteral(exact) {
			t.Errorf("exact product carrier vocabulary %q escaped the guard", exact)
		}
	}

	if forbiddenKernelImport(
		haftModulePath + "/internal/typedmemorycandidatecodec",
	) {
		t.Error("generic typed-memory collaborator import was treated as a product import")
	}
	if !forbiddenKernelImport(
		haftModulePath + "/internal/projectmemory/problemcardadapter",
	) {
		t.Error("exact product adapter import escaped the generic-kernel boundary")
	}
}

func typedMemoryKernelDirectory(t *testing.T) string {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate typed-memory genericity acceptance source")
	}
	return filepath.Dir(currentFile)
}

func assertNoProductImports(
	t *testing.T,
	fileSet *token.FileSet,
	file *ast.File,
) {
	t.Helper()
	for _, imported := range file.Imports {
		path, err := strconv.Unquote(imported.Path.Value)
		if err != nil {
			t.Fatalf("decode import at %s: %v", fileSet.Position(imported.Pos()), err)
		}
		if forbiddenKernelImport(path) {
			t.Errorf(
				"generic typed-memory kernel imports product layer %q at %s",
				path,
				fileSet.Position(imported.Pos()),
			)
		}
	}
}

func assertNoProductCarrierLiterals(
	t *testing.T,
	fileSet *token.FileSet,
	file *ast.File,
) {
	t.Helper()
	ast.Inspect(file, func(node ast.Node) bool {
		literal, ok := node.(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			return true
		}
		value, err := strconv.Unquote(literal.Value)
		if err != nil {
			t.Errorf("decode string literal at %s: %v", fileSet.Position(literal.Pos()), err)
			return true
		}
		if forbiddenKernelLiteral(value) {
			t.Errorf(
				"generic typed-memory kernel contains product carrier literal %q at %s",
				value,
				fileSet.Position(literal.Pos()),
			)
		}
		return true
	})
}

func forbiddenKernelImport(path string) bool {
	for _, root := range forbiddenKernelImportRoots {
		if path == root || strings.HasPrefix(path, root+"/") {
			return true
		}
	}
	return false
}

func forbiddenKernelLiteral(value string) bool {
	if strings.Contains(value, "Haft.") {
		return true
	}
	for _, token := range forbiddenKernelLiteralTokens {
		if strings.Contains(value, token) {
			return true
		}
	}
	return false
}

func assertSameClaimGraph(
	t *testing.T,
	consumer string,
	want typedmemory.ClaimGraphValue,
	got typedmemory.ClaimGraphValue,
) {
	t.Helper()
	if got.Kind() != want.Kind() ||
		len(got.Nodes()) != len(want.Nodes()) ||
		len(got.Edges()) != len(want.Edges()) {
		t.Fatalf("%s adapter changed the generic ClaimGraphValue", consumer)
	}
}
