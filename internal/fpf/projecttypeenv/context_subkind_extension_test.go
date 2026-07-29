package projecttypeenv

import (
	"bytes"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf/localpractice"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
)

func TestContextAndSubkindCompileToClosedSourceDeclarations(t *testing.T) {
	base := loadBaseArtifact(t)
	parsed := parseCarrier(t, contextSubkindExtensionSource(t, base, "Alpha"))
	bundle := acceptedManifestBundle(t, base, []localpractice.ParsedCarrier{parsed})
	ir := compileExtensionIR(t, bundle.Nodes()[0], nil)

	context := declarationByKind(t, &ir, localpractice.DeclarationBoundedContext)
	if context.Symbol().Value() != "haft-project" ||
		len(context.Exports()) != 1 ||
		context.Exports()[0].Value() != "haft-project" ||
		len(context.Facts()) != 0 ||
		len(context.Dependencies()) != 0 {
		t.Fatalf("bounded-context IR = %#v", context)
	}
	subkind := declarationByKind(t, &ir, localpractice.DeclarationSubkind)
	if subkind.Symbol().Value() != "Alpha.Subkind.ProjectConcernEntity" {
		t.Fatalf("subkind symbol = %q", subkind.Symbol().Value())
	}
	if len(subkind.Exports()) != 0 {
		t.Fatalf("subkind exports = %#v, want none", subkind.Exports())
	}
	assertExactSourceFact(t, subkind, "child_kind", "Alpha.ProjectConcern")
	assertExactSourceFact(t, subkind, "super_kind", "U.Entity")
	if !hasExactDependency(subkind, "child_kind", "Alpha.ProjectConcern") ||
		!hasExactDependency(subkind, "super_kind", "U.Entity") {
		t.Fatalf("subkind dependencies = %#v", subkind.Dependencies())
	}

	artifact := sealExtension(t, ir)
	decoded, err := DecodeProjectTypeEnvExtensionArtifact(artifact.CanonicalBytes())
	if err != nil {
		t.Fatalf("DecodeProjectTypeEnvExtensionArtifact() error = %v", err)
	}
	if decoded.Ref() != artifact.Ref() ||
		!bytes.Equal(decoded.CanonicalBytes(), artifact.CanonicalBytes()) {
		t.Fatal("context/subkind extension changed identity across round trip")
	}
}

func TestBoundedContextDeclarationMustMatchCarrierAndApplicability(t *testing.T) {
	base := loadBaseArtifact(t)
	source := string(contextSubkindExtensionSource(t, base, "Alpha"))
	source = strings.Replace(source, "    - haft-project\n", "    - other-context\n", 1)
	source = strings.Replace(source, "        symbol: haft-project\n", "        symbol: other-context\n", 1)
	parsed := parseCarrier(t, []byte(source))
	bundle := acceptedManifestBundle(t, base, []localpractice.ParsedCarrier{parsed})

	_, err := CompileProjectTypeEnvExtensionIR(bundle.Nodes()[0], nil)
	if err == nil || !strings.Contains(err.Error(), "does not match carrier root and Applicability") {
		t.Fatalf("CompileProjectTypeEnvExtensionIR() error = %v", err)
	}
}

func TestContextAndSubkindCanonicalSchemaRejectsForgedVariants(t *testing.T) {
	base := loadBaseArtifact(t)
	parsed := parseCarrier(t, contextSubkindExtensionSource(t, base, "Alpha"))
	bundle := acceptedManifestBundle(t, base, []localpractice.ParsedCarrier{parsed})
	artifact := compileAndSealExtension(t, bundle.Nodes()[0], nil)
	tests := []struct {
		name   string
		mutate func(*testing.T, *ProjectTypeEnvExtensionIR)
		want   string
	}{
		{
			name: "bounded context fact",
			mutate: func(t *testing.T, ir *ProjectTypeEnvExtensionIR) {
				declaration := declarationByKind(t, ir, localpractice.DeclarationBoundedContext)
				declaration.facts = append(declaration.facts, SourceFact{
					path:  "invented",
					value: declaration.symbol,
				})
			},
			want: "outside the bounded_context schema",
		},
		{
			name: "subkind export",
			mutate: func(t *testing.T, ir *ProjectTypeEnvExtensionIR) {
				declaration := declarationByKind(t, ir, localpractice.DeclarationSubkind)
				declaration.exports = append(declaration.exports, declaration.symbol)
			},
			want: "must not export a schema symbol",
		},
		{
			name: "subkind dependency divergence",
			mutate: func(t *testing.T, ir *ProjectTypeEnvExtensionIR) {
				declaration := declarationByKind(t, ir, localpractice.DeclarationSubkind)
				for index := range declaration.dependencies {
					if declaration.dependencies[index].role == "super_kind" {
						declaration.dependencies[index].target.value = "U.System"
					}
				}
			},
			want: "does not exactly match its source fact",
		},
		{
			name: "subkind equal endpoints",
			mutate: func(t *testing.T, ir *ProjectTypeEnvExtensionIR) {
				declaration := declarationByKind(t, ir, localpractice.DeclarationSubkind)
				child := sourceFactByPath(t, declaration, "child_kind")
				for index := range declaration.facts {
					if declaration.facts[index].path == "super_kind" {
						declaration.facts[index].value = child
					}
				}
				for index := range declaration.dependencies {
					if declaration.dependencies[index].role == "super_kind" {
						declaration.dependencies[index].target = child
					}
				}
			},
			want: "child_kind and super_kind must be distinct",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ir := artifact.IR()
			test.mutate(t, &ir)
			_, err := SealProjectTypeEnvExtension(ir)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("SealProjectTypeEnvExtension() error = %v, want %q", err, test.want)
			}
		})
	}
}

func contextSubkindExtensionSource(
	t *testing.T,
	base typeenv.BaseTypeEnvArtifact,
	prefix string,
) []byte {
	t.Helper()
	source := string(carrierFixture(t, base, "context.signature", "1.0.0", prefix, nil))
	source = strings.Replace(
		source,
		"    - "+prefix+".ProjectConcern\n",
		"    - haft-project\n    - "+prefix+".ProjectConcern\n",
		1,
	)
	source = strings.Replace(
		source,
		"      - kind: value_kind\n",
		"      - kind: bounded_context\n"+
			"        symbol: haft-project\n"+
			"      - kind: subkind\n"+
			"        symbol: "+prefix+".Subkind.ProjectConcernEntity\n"+
			"        child_kind: "+prefix+".ProjectConcern\n"+
			"        super_kind: U.Entity\n"+
			"      - kind: value_kind\n",
		1,
	)
	return []byte(source)
}

func assertExactSourceFact(
	t *testing.T,
	declaration *SymbolicDeclaration,
	path string,
	want string,
) {
	t.Helper()
	if got := sourceFactByPath(t, declaration, path).Value(); got != want {
		t.Fatalf("%s = %q, want %q", path, got, want)
	}
}

func sourceFactByPath(
	t *testing.T,
	declaration *SymbolicDeclaration,
	path string,
) SourceScalar {
	t.Helper()
	for _, fact := range declaration.Facts() {
		if fact.Path() == path {
			return fact.Value()
		}
	}
	t.Fatalf("declaration %q has no source fact %q", declaration.Symbol().Value(), path)
	return SourceScalar{}
}

func hasExactDependency(
	declaration *SymbolicDeclaration,
	role string,
	target string,
) bool {
	for _, dependency := range declaration.Dependencies() {
		if dependency.Role() == role && dependency.Target().Value() == target {
			return true
		}
	}
	return false
}
