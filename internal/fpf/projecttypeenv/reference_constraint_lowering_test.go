package projecttypeenv

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf/localpractice"
)

func TestReferenceConstraintsCompileSealDecodeAndRetainExactSource(t *testing.T) {
	t.Parallel()

	artifact := referenceConstraintArtifact(t, referenceConstraintSource(t))
	if err := artifact.Verify(); err != nil {
		t.Fatalf("artifact.Verify() error = %v", err)
	}
	decoded, err := DecodeProjectTypeEnvExtensionArtifact(artifact.CanonicalBytes())
	if err != nil {
		t.Fatalf("DecodeProjectTypeEnvExtensionArtifact() error = %v", err)
	}
	if decoded.Ref() != artifact.Ref() {
		t.Fatal("decoded artifact changed the source E identity")
	}
	if !bytes.Equal(decoded.CanonicalBytes(), artifact.CanonicalBytes()) {
		t.Fatal("decoded artifact changed canonical E bytes")
	}

	ir := decoded.IR()
	subset := declarationBySymbolForTest(t, &ir, "Haft.Constraint.SelectedSubset")
	assertDeclarationSpanForTest(t, subset, 45, 51)
	assertFactForTest(t, subset, "rule.kind", "reference_slot_subset", 48, 51)
	assertFactForTest(t, subset, "rule.relation", "Haft.ReferencePartition", 49, 49)
	assertFactForTest(t, subset, "rule.subset", "SelectedSlot", 50, 50)
	assertFactForTest(t, subset, "rule.superset", "WholeSlot", 51, 51)
	assertDependencyForTest(t, subset, "constraint.relation", "Haft.ReferencePartition", 49, 49)
	assertDependencyForTest(t, subset, "constraint.subset", "SelectedSlot", 50, 50)
	assertDependencyForTest(t, subset, "constraint.superset", "WholeSlot", 51, 51)
	assertExactOwnExportForTest(t, subset)

	partition := declarationBySymbolForTest(t, &ir, "Haft.Constraint.WholePartition")
	assertDeclarationSpanForTest(t, partition, 52, 60)
	assertFactForTest(t, partition, "rule.kind", "reference_slot_partition", 55, 60)
	assertFactForTest(t, partition, "rule.relation", "Haft.ReferencePartition", 56, 56)
	assertFactForTest(t, partition, "rule.whole", "WholeSlot", 57, 57)
	assertFactForTest(t, partition, indexedPath("rule.parts", 0), "SelectedSlot", 59, 59)
	assertFactForTest(t, partition, indexedPath("rule.parts", 1), "RejectedSlot", 60, 60)
	assertDependencyForTest(t, partition, "constraint.relation", "Haft.ReferencePartition", 56, 56)
	assertDependencyForTest(t, partition, "constraint.whole", "WholeSlot", 57, 57)
	assertDependencyForTest(t, partition, indexedPath("rule.parts", 0), "SelectedSlot", 59, 59)
	assertDependencyForTest(t, partition, indexedPath("rule.parts", 1), "RejectedSlot", 60, 60)
	assertExactOwnExportForTest(t, partition)
}

func TestReferenceConstraintArtifactRejectsMissingDependenciesAndExportDrift(t *testing.T) {
	t.Parallel()

	artifact := referenceConstraintArtifact(t, referenceConstraintSource(t))
	tests := []struct {
		name   string
		mutate func(*ProjectTypeEnvExtensionIR)
		want   string
	}{
		{
			name: "missing subset relation dependency",
			mutate: func(ir *ProjectTypeEnvExtensionIR) {
				declaration := declarationBySymbolForTest(t, ir, "Haft.Constraint.SelectedSubset")
				declaration.dependencies = removeDependencyForTest(
					declaration.dependencies,
					"constraint.relation",
				)
			},
			want: "requires dependency role \"constraint.relation\"",
		},
		{
			name: "missing partition slot dependency",
			mutate: func(ir *ProjectTypeEnvExtensionIR) {
				declaration := declarationBySymbolForTest(t, ir, "Haft.Constraint.WholePartition")
				declaration.dependencies = removeDependencyForTest(
					declaration.dependencies,
					indexedPath("rule.parts", 1),
				)
			},
			want: "requires dependency role \"rule.parts[000001]\"",
		},
		{
			name: "manifest provide missing",
			mutate: func(ir *ProjectTypeEnvExtensionIR) {
				ir.manifest.provides = ir.manifest.provides[:len(ir.manifest.provides)-1]
			},
			want: "source manifest provides",
		},
		{
			name: "declaration export missing",
			mutate: func(ir *ProjectTypeEnvExtensionIR) {
				declaration := declarationBySymbolForTest(t, ir, "Haft.Constraint.SelectedSubset")
				declaration.exports = nil
			},
			want: "has no explicit export",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ir := artifact.IR()
			test.mutate(&ir)
			_, err := SealProjectTypeEnvExtension(ir)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("SealProjectTypeEnvExtension() error = %v; want %q", err, test.want)
			}
		})
	}
}

func TestReferenceConstraintDecodeRejectsForgedClosedVariant(t *testing.T) {
	t.Parallel()

	artifact := referenceConstraintArtifact(t, referenceConstraintSource(t))
	encoded := decodeProjectExtensionCanonicalForTest(t, artifact.CanonicalBytes())
	declaration := canonicalDeclarationBySymbolForTest(
		t,
		&encoded,
		"Haft.Constraint.SelectedSubset",
	)
	for index := range declaration.Facts {
		fact := &declaration.Facts[index]
		if fact.Path == "rule.kind" {
			fact.Value.Value = "reference_slot_partition"
		}
	}
	forged := encodeProjectExtensionCanonicalForTest(t, encoded)
	_, err := DecodeProjectTypeEnvExtensionArtifact(forged)
	if err == nil || !strings.Contains(err.Error(), "requires source fact \"rule.whole\"") {
		t.Fatalf("DecodeProjectTypeEnvExtensionArtifact() error = %v; want closed-variant rejection", err)
	}
}

func referenceConstraintSource(t *testing.T) []byte {
	t.Helper()
	path := filepath.Join("..", "localpractice", "testdata", "valid_reference_constraints.yaml")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read reference constraint fixture: %v", err)
	}
	base := loadBaseArtifact(t)
	placeholder := "typeenv:sha256:" + strings.Repeat("a", 64)
	resolved := strings.Replace(string(source), placeholder, baseRef(t, base), 1)
	if resolved == string(source) {
		t.Fatal("reference constraint fixture base placeholder was not replaced")
	}
	provide := "    - Haft.ReferencePartition"
	completeProvides := strings.Join([]string{
		provide,
		"    - WholeSlot",
		"    - SelectedSlot",
		"    - RejectedSlot",
	}, "\n")
	completed := strings.Replace(resolved, provide, completeProvides, 1)
	if completed == resolved {
		t.Fatal("reference constraint fixture nested SlotKind provides were not inserted")
	}
	return []byte(completed)
}

func referenceConstraintArtifact(
	t *testing.T,
	source []byte,
) ProjectTypeEnvExtensionArtifact {
	t.Helper()
	base := loadBaseArtifact(t)
	carrier := parseCarrier(t, source)
	bundle := acceptedManifestBundle(t, base, []localpractice.ParsedCarrier{carrier})
	nodes := bundle.Nodes()
	if len(nodes) != 1 {
		t.Fatalf("resolved nodes = %d, want one", len(nodes))
	}
	return compileAndSealExtension(t, nodes[0], nil)
}

func declarationBySymbolForTest(
	t *testing.T,
	ir *ProjectTypeEnvExtensionIR,
	symbol string,
) *SymbolicDeclaration {
	t.Helper()
	for index := range ir.signature.vocabulary.declarations {
		declaration := &ir.signature.vocabulary.declarations[index]
		if declaration.symbol.value == symbol {
			return declaration
		}
	}
	t.Fatalf("symbolic declaration %q was not found", symbol)
	return nil
}

func assertDeclarationSpanForTest(
	t *testing.T,
	declaration *SymbolicDeclaration,
	start uint64,
	end uint64,
) {
	t.Helper()
	if declaration.span.start != start || declaration.span.end != end {
		t.Fatalf("%q span = %d-%d, want %d-%d", declaration.symbol.value, declaration.span.start, declaration.span.end, start, end)
	}
}

func assertFactForTest(
	t *testing.T,
	declaration *SymbolicDeclaration,
	path string,
	value string,
	start uint64,
	end uint64,
) {
	t.Helper()
	for _, fact := range declaration.facts {
		if fact.path == path {
			if fact.value.value != value || fact.value.span.start != start || fact.value.span.end != end {
				t.Fatalf("%q fact %q = %#v, want %q at %d-%d", declaration.symbol.value, path, fact.value, value, start, end)
			}
			return
		}
	}
	t.Fatalf("%q fact %q was not found", declaration.symbol.value, path)
}

func assertDependencyForTest(
	t *testing.T,
	declaration *SymbolicDeclaration,
	role string,
	target string,
	start uint64,
	end uint64,
) {
	t.Helper()
	for _, dependency := range declaration.dependencies {
		if dependency.role == role {
			if dependency.target.value != target || dependency.target.span.start != start || dependency.target.span.end != end {
				t.Fatalf("%q dependency %q = %#v, want %q at %d-%d", declaration.symbol.value, role, dependency.target, target, start, end)
			}
			return
		}
	}
	t.Fatalf("%q dependency %q was not found", declaration.symbol.value, role)
}

func assertExactOwnExportForTest(t *testing.T, declaration *SymbolicDeclaration) {
	t.Helper()
	if len(declaration.exports) != 1 || declaration.exports[0] != declaration.symbol {
		t.Fatalf("%q exports = %#v, want exact own source scalar", declaration.symbol.value, declaration.exports)
	}
}

func removeDependencyForTest(
	dependencies []SymbolicDependency,
	role string,
) []SymbolicDependency {
	result := make([]SymbolicDependency, 0, len(dependencies))
	for _, dependency := range dependencies {
		if dependency.role != role {
			result = append(result, dependency)
		}
	}
	return result
}

func canonicalDeclarationBySymbolForTest(
	t *testing.T,
	encoded *projectExtensionCanonicalV1,
	symbol string,
) *symbolicDeclarationCanonicalV1 {
	t.Helper()
	for index := range encoded.Signature.Vocabulary.Declarations {
		declaration := &encoded.Signature.Vocabulary.Declarations[index]
		if declaration.Symbol.Value == symbol {
			return declaration
		}
	}
	t.Fatalf("canonical declaration %q was not found", symbol)
	return nil
}
