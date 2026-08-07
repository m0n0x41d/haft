package projecttypeenv

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf/localpractice"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
)

func TestKindBridgeCompilesToExactSymbolicExtensionArtifact(t *testing.T) {
	t.Parallel()

	base := loadBaseArtifact(t)
	parsed := parseKindBridgeCarrierForBase(t, base, "one_way")
	bundle := acceptedManifestBundle(t, base, []localpractice.ParsedCarrier{parsed})
	node := bundle.Nodes()[0]
	artifact := compileAndSealExtension(t, node, nil)
	ir := artifact.IR()
	carrier := ir.Carrier()
	if carrier.ID().Value() != "haft.auth-bridge" ||
		carrier.ID().Span().Start() != 3 ||
		carrier.Edition().Value() != "1.0.0" ||
		carrier.Edition().Span().Start() != 4 ||
		carrier.Digest().String() != parsed.Digest().String() {
		t.Fatalf("bridge carrier provenance = %#v, source digest %q", carrier, parsed.Digest())
	}
	if ir.BaseSource().Value() != baseRef(t, base) ||
		ir.BaseSource().Span().Start() != 5 ||
		ir.BoundedContext().Value() != "frontend" ||
		ir.BoundedContext().Span().Start() != 6 {
		t.Fatalf("bridge base/context provenance = %#v / %#v", ir.BaseSource(), ir.BoundedContext())
	}
	declaration := declarationByKind(t, &ir, localpractice.DeclarationKindBridge)

	if declaration.Symbol().Value() != "Haft.Bridge.AuthenticatedRequestToFrontendRequest" {
		t.Fatalf("bridge symbol = %q", declaration.Symbol().Value())
	}
	if declaration.Span().Start() != 23 || declaration.Span().End() != 43 {
		t.Fatalf("bridge span = %d..%d", declaration.Span().Start(), declaration.Span().End())
	}
	if len(declaration.Exports()) != 1 || declaration.Exports()[0] != declaration.Symbol() {
		t.Fatalf("bridge exports = %#v, want exact own source symbol", declaration.Exports())
	}
	assertKindBridgeFact(
		t,
		declaration,
		"endpoints.source.bounded_context_ref",
		"auth-service",
		27,
	)
	assertKindBridgeFact(t, declaration, "endpoints.source.edition", "AuthStandard-v2.3", 28)
	assertKindBridgeFact(
		t,
		declaration,
		"endpoints.target.bounded_context_ref",
		"frontend",
		30,
	)
	assertKindBridgeFact(t, declaration, "endpoints.target.edition", "FrontendAuth-v1.4", 31)
	assertKindBridgeFact(t, declaration, "mapping.kind", "named_target", 33)
	assertKindBridgeFact(
		t,
		declaration,
		"mapping.source_kind",
		"Auth.AuthenticatedRequest",
		34,
	)
	assertKindBridgeFact(
		t,
		declaration,
		"mapping.target_kind",
		"Frontend.VerifiedRequest",
		35,
	)
	assertKindBridgeFact(t, declaration, "direction", "one_way", 36)
	assertKindBridgeFact(t, declaration, "order_preservation", "no_links_covered", 37)
	assertKindBridgeFact(t, declaration, "kind_congruence", "2", 38)
	assertKindBridgeFact(
		t,
		declaration,
		indexedPath("loss_notes", 0),
		"The x-auth header representation is not preserved.",
		40,
	)
	assertKindBridgeFact(
		t,
		declaration,
		indexedPath("definedness_area", 1),
		"Target membership is re-evaluated in the frontend ContextSlice.",
		43,
	)
	assertKindBridgeDependency(
		t,
		declaration,
		"mapping.source_kind",
		"Auth.AuthenticatedRequest",
	)
	assertKindBridgeDependency(
		t,
		declaration,
		"mapping.target_kind",
		"Frontend.VerifiedRequest",
	)
	if len(declaration.Dependencies()) != 2 {
		t.Fatalf("bridge dependencies = %#v, want exact source/target kind pair", declaration.Dependencies())
	}
	applicability := ir.Signature().Applicability()
	assertSignatureFact(t, applicability, "bounded_context_ref", "frontend", 50)
	assertSignatureFact(
		t,
		applicability,
		indexedPath("assumptions", 0),
		"The exact endpoint editions remain available.",
		52,
	)
	manifest := ir.Manifest()
	provides := manifest.Provides()
	if len(provides) != 1 ||
		provides[0].Value() != declaration.Symbol().Value() ||
		provides[0].Span().Start() != 14 ||
		provides[0].Span().End() != 14 {
		t.Fatalf("manifest provides = %#v, want exact bridge export", manifest.Provides())
	}
	if strings.Contains(string(artifact.CanonicalBytes()), "ContextKindAvailability") ||
		strings.Contains(string(artifact.CanonicalBytes()), "context_kind_availability") {
		t.Fatal("symbolic E artifact authored a derived ContextKindAvailability")
	}

	decoded, err := DecodeProjectTypeEnvExtensionArtifact(artifact.CanonicalBytes())
	if err != nil {
		t.Fatalf("DecodeProjectTypeEnvExtensionArtifact() error = %v", err)
	}
	if decoded.Ref() != artifact.Ref() ||
		!bytes.Equal(decoded.CanonicalBytes(), artifact.CanonicalBytes()) {
		t.Fatal("KindBridge artifact did not round-trip byte-identically")
	}
}

func TestKindBridgeExtensionIdentityTracksExplicitDirectionAndMapping(t *testing.T) {
	t.Parallel()

	base := loadBaseArtifact(t)
	oneWay := sealKindBridgeCarrierForBase(t, base, "one_way")
	twoWay := sealKindBridgeCarrierForBase(t, base, "two_way")
	if oneWay.Ref() == twoWay.Ref() || bytes.Equal(oneWay.CanonicalBytes(), twoWay.CanonicalBytes()) {
		t.Fatal("changing explicit bridge direction did not change E identity")
	}

	source := kindBridgeSourceForBase(t, base, "one_way")
	identityMapping := bytes.Replace(
		source,
		[]byte("target_kind: Frontend.VerifiedRequest"),
		[]byte("target_kind: Auth.AuthenticatedRequest"),
		1,
	)
	parsed := parseCarrier(t, identityMapping)
	bundle := acceptedManifestBundle(t, base, []localpractice.ParsedCarrier{parsed})
	identity := compileAndSealExtension(t, bundle.Nodes()[0], nil)
	if oneWay.Ref() == identity.Ref() || bytes.Equal(oneWay.CanonicalBytes(), identity.CanonicalBytes()) {
		t.Fatal("changing non-identity kind mapping to identity did not change E identity")
	}
}

func TestKindBridgeCanonicalDecodeFailsClosedOnUnknownDirectionOrFact(t *testing.T) {
	t.Parallel()

	base := loadBaseArtifact(t)
	artifact := sealKindBridgeCarrierForBase(t, base, "one_way")
	tests := []struct {
		name    string
		mutate  func(*symbolicDeclarationCanonicalV1)
		message string
	}{
		{
			name: "unknown direction",
			mutate: func(declaration *symbolicDeclarationCanonicalV1) {
				for index := range declaration.Facts {
					fact := &declaration.Facts[index]
					if fact.Path == "direction" {
						fact.Value.Value = "reverse"
						return
					}
				}
			},
			message: "direction",
		},
		{
			name: "unknown fact",
			mutate: func(declaration *symbolicDeclarationCanonicalV1) {
				declaration.Facts = append(
					declaration.Facts,
					sourceFactCanonicalV1{
						Path: "scope",
						Value: sourceScalarCanonicalV1{
							Value: "project-wide",
							Start: declaration.Span.Start,
							End:   declaration.Span.Start,
						},
					},
				)
			},
			message: "outside the kind_bridge schema",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoded := decodeProjectExtensionCanonicalForTest(t, artifact.CanonicalBytes())
			declaration := canonicalDeclarationByKindForTest(
				t,
				&encoded,
				localpractice.DeclarationKindBridge,
			)
			test.mutate(declaration)
			forged := encodeProjectExtensionCanonicalForTest(t, encoded)
			_, err := DecodeProjectTypeEnvExtensionArtifact(forged)
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("Decode error = %v, want substring %q", err, test.message)
			}
		})
	}
}

func TestKindBridgeExtensionCompilationIsDeterministic(t *testing.T) {
	t.Parallel()

	base := loadBaseArtifact(t)
	first := sealKindBridgeCarrierForBase(t, base, "one_way")
	second := sealKindBridgeCarrierForBase(t, base, "one_way")
	if first.Ref() != second.Ref() || !bytes.Equal(first.CanonicalBytes(), second.CanonicalBytes()) {
		t.Fatal("same exact KindBridge carrier produced different E identity")
	}
}

func sealKindBridgeCarrierForBase(
	t *testing.T,
	base typeenv.BaseTypeEnvArtifact,
	direction string,
) ProjectTypeEnvExtensionArtifact {
	t.Helper()
	parsed := parseKindBridgeCarrierForBase(t, base, direction)
	bundle := acceptedManifestBundle(t, base, []localpractice.ParsedCarrier{parsed})
	return compileAndSealExtension(t, bundle.Nodes()[0], nil)
}

func parseKindBridgeCarrierForBase(
	t *testing.T,
	base typeenv.BaseTypeEnvArtifact,
	direction string,
) localpractice.ParsedCarrier {
	t.Helper()
	return parseCarrier(t, kindBridgeSourceForBase(t, base, direction))
}

func kindBridgeSourceForBase(
	t *testing.T,
	base typeenv.BaseTypeEnvArtifact,
	direction string,
) []byte {
	t.Helper()
	source, err := os.ReadFile("../localpractice/testdata/valid_kind_bridge.yaml")
	if err != nil {
		t.Fatalf("read KindBridge fixture: %v", err)
	}
	baseRef, exists := base.TypeEnvRef()
	if !exists {
		t.Fatal("base artifact has no TypeEnvRef")
	}
	source = bytes.Replace(
		source,
		[]byte("typeenv:sha256:"+strings.Repeat("a", 64)),
		[]byte(baseRef.String()),
		1,
	)
	source = bytes.Replace(source, []byte("direction: one_way"), []byte("direction: "+direction), 1)
	return source
}

func assertKindBridgeFact(
	t *testing.T,
	declaration *SymbolicDeclaration,
	path string,
	value string,
	line uint64,
) {
	t.Helper()
	for _, fact := range declaration.Facts() {
		if fact.Path() != path {
			continue
		}
		actual := fact.Value()
		if actual.Value() != value || actual.Span().Start() != line || actual.Span().End() != line {
			t.Fatalf("fact %q = %#v, want %q at line %d", path, actual, value, line)
		}
		return
	}
	t.Fatalf("fact %q was not found", path)
}

func assertKindBridgeDependency(
	t *testing.T,
	declaration *SymbolicDeclaration,
	role string,
	target string,
) {
	t.Helper()
	for _, dependency := range declaration.Dependencies() {
		if dependency.Role() == role && dependency.Target().Value() == target {
			return
		}
	}
	t.Fatalf("dependency %q -> %q was not found", role, target)
}

func assertSignatureFact(
	t *testing.T,
	row SignatureRowIR,
	path string,
	value string,
	line uint64,
) {
	t.Helper()
	for _, fact := range row.Facts() {
		if fact.Path() == path && fact.Value().Value() == value &&
			fact.Value().Span().Start() == line && fact.Value().Span().End() == line {
			return
		}
	}
	t.Fatalf("signature fact %q = %q at line %d was not found", path, value, line)
}
