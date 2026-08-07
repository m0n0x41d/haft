package projecttypeenv

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf/localpractice"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestProjectTypeEnvCompositeSealUsesCanonicalLinkedRecipe(t *testing.T) {
	base, extensions := projectTypeEnvCompositeArtifactFixture(t)
	basis := emptyRuntimeEvaluationBasisFixture(t)
	leftLinked := acceptedCompositeIR(t, LinkProjectTypeEnvCompositeIR(
		base,
		[]ProjectTypeEnvExtensionArtifact{extensions[1], extensions[2], extensions[0]},
	))
	rightLinked := acceptedCompositeIR(t, LinkProjectTypeEnvCompositeIR(
		base,
		[]ProjectTypeEnvExtensionArtifact{extensions[2], extensions[0], extensions[1]},
	))
	left, err := SealProjectTypeEnvComposite(leftLinked, basis)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvComposite(left): %v", err)
	}
	right, err := SealProjectTypeEnvComposite(rightLinked, basis)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvComposite(right): %v", err)
	}
	if left.Ref() != right.Ref() {
		t.Fatalf("caller permutation changed C: %s != %s", left.Ref(), right.Ref())
	}
	if left.Digest() != left.Ref().Digest() {
		t.Fatal("composite digest is not the digest carried by C")
	}
	if !bytes.Equal(left.CanonicalBytes(), right.CanonicalBytes()) {
		t.Fatal("caller permutation changed composite canonical bytes")
	}
	expectedRefs := projectTypeEnvCompositeExtensionRefs(leftLinked.Extensions())
	if !projectTypeEnvExtensionRefsEqual(left.ExtensionRefs(), expectedRefs) {
		t.Fatalf("extension order = %v, want linked topological order %v", left.ExtensionRefs(), expectedRefs)
	}
	if left.BaseTypeEnvRef() != leftLinked.BaseTypeEnvRef() {
		t.Fatal("composite recipe lost exact B")
	}
	if left.RuntimeEvaluationBasisRef() != basis.Ref() {
		t.Fatal("composite recipe lost exact X")
	}
	if left.LowererSchemaVersion() != ProjectTypeEnvCompositeLowererSchemaV2 {
		t.Fatalf("lowerer schema = %q", left.LowererSchemaVersion())
	}
	if bytes.Contains(left.CanonicalBytes(), []byte(left.Ref().String())) {
		t.Fatal("composite canonical recipe contains its own C reference")
	}
	if err := left.Verify(); err != nil {
		t.Fatalf("Verify(): %v", err)
	}
	assertProjectTypeEnvCompositeRecipeOnly(t, left)
}

func TestProjectTypeEnvCompositeIdentityIsSensitiveToRecipeCoordinates(t *testing.T) {
	base, extensions := projectTypeEnvCompositeArtifactFixture(t)
	basis := emptyRuntimeEvaluationBasisFixture(t)
	linked := acceptedCompositeIR(t, LinkProjectTypeEnvCompositeIR(base, extensions))
	artifact, err := SealProjectTypeEnvComposite(linked, basis)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvComposite(): %v", err)
	}
	recipe := projectTypeEnvCompositeRecipe{
		base:          artifact.BaseTypeEnvRef(),
		extensions:    artifact.ExtensionRefs(),
		runtimeBasis:  artifact.RuntimeEvaluationBasisRef(),
		lowererSchema: artifact.LowererSchemaVersion(),
	}

	tests := []struct {
		name   string
		mutate func(projectTypeEnvCompositeRecipe) projectTypeEnvCompositeRecipe
	}{
		{
			name: "base",
			mutate: func(value projectTypeEnvCompositeRecipe) projectTypeEnvCompositeRecipe {
				value.base = projectTypeEnvRefFixture(t, "1")
				return value
			},
		},
		{
			name: "extension",
			mutate: func(value projectTypeEnvCompositeRecipe) projectTypeEnvCompositeRecipe {
				value.extensions = append([]typedmemory.TypeEnvExtensionRef(nil), value.extensions...)
				value.extensions[0] = projectTypeEnvExtensionRefFixture(t, "replacement.signature", "2")
				return value
			},
		},
		{
			name: "extension-order",
			mutate: func(value projectTypeEnvCompositeRecipe) projectTypeEnvCompositeRecipe {
				value.extensions = append([]typedmemory.TypeEnvExtensionRef(nil), value.extensions...)
				value.extensions[0], value.extensions[1] = value.extensions[1], value.extensions[0]
				return value
			},
		},
		{
			name: "runtime-basis",
			mutate: func(value projectTypeEnvCompositeRecipe) projectTypeEnvCompositeRecipe {
				value.runtimeBasis = runtimeEvaluationBasisRefFixture(t, "3")
				return value
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			changed := test.mutate(recipe)
			canonical, encodeErr := encodeProjectTypeEnvCompositeRecipe(changed)
			if encodeErr != nil {
				t.Fatalf("encode changed recipe: %v", encodeErr)
			}
			ref, refErr := projectTypeEnvCompositeRef(canonical)
			if refErr != nil {
				t.Fatalf("derive changed ref: %v", refErr)
			}
			if ref == artifact.Ref() {
				t.Fatalf("changing %s did not change C", test.name)
			}
		})
	}

	t.Run("lowerer-schema", func(t *testing.T) {
		encoded := projectTypeEnvCompositeCanonicalV1{
			BaseTypeEnvRef:            recipe.base.String(),
			ExtensionRefs:             projectTypeEnvCompositeRefStrings(recipe.extensions),
			RuntimeEvaluationBasisRef: recipe.runtimeBasis.String(),
			LowererSchemaVersion:      ProjectTypeEnvCompositeLowererSchemaV1,
		}
		canonical := projectTypeEnvCompositeRawCanonicalFixture(t, encoded)
		ref, refErr := projectTypeEnvCompositeRef(canonical)
		if refErr != nil {
			t.Fatalf("derive changed ref: %v", refErr)
		}
		if ref == artifact.Ref() {
			t.Fatal("changing lowerer schema did not change C")
		}
		if _, decodeErr := DecodeProjectTypeEnvCompositeArtifact(canonical); decodeErr != nil {
			t.Fatalf("historical lowerer decode error = %v", decodeErr)
		}
	})

	t.Run("unsupported-lowerer-schema", func(t *testing.T) {
		encoded := projectTypeEnvCompositeCanonicalV1{
			BaseTypeEnvRef:            recipe.base.String(),
			ExtensionRefs:             projectTypeEnvCompositeRefStrings(recipe.extensions),
			RuntimeEvaluationBasisRef: recipe.runtimeBasis.String(),
			LowererSchemaVersion:      "haft.fpf.projecttypeenv.composite-lowerer/v3",
		}
		canonical := projectTypeEnvCompositeRawCanonicalFixture(t, encoded)
		if _, decodeErr := DecodeProjectTypeEnvCompositeArtifact(canonical); decodeErr == nil ||
			!strings.Contains(decodeErr.Error(), "unsupported") {
			t.Fatalf("unsupported lowerer decode error = %v", decodeErr)
		}
	})
}

func TestProjectTypeEnvCompositeDecodeRoundTripAndDeepCopy(t *testing.T) {
	artifact := sealedProjectTypeEnvCompositeArtifactFixture(t)
	canonical := artifact.CanonicalBytes()
	decoded, err := DecodeProjectTypeEnvCompositeArtifact(canonical)
	if err != nil {
		t.Fatalf("DecodeProjectTypeEnvCompositeArtifact(): %v", err)
	}
	if decoded.Ref() != artifact.Ref() {
		t.Fatalf("round-trip C = %s, want %s", decoded.Ref(), artifact.Ref())
	}
	canonical[0] ^= 0xff
	if bytes.Equal(canonical, artifact.CanonicalBytes()) {
		t.Fatal("CanonicalBytes returned shared storage")
	}
	refs := decoded.ExtensionRefs()
	refs[0] = typedmemory.TypeEnvExtensionRef{}
	if decoded.ExtensionRefs()[0] == (typedmemory.TypeEnvExtensionRef{}) {
		t.Fatal("ExtensionRefs returned shared storage")
	}
	verified, err := VerifyProjectTypeEnvCompositeArtifact(
		artifact.Ref(),
		artifact.CanonicalBytes(),
	)
	if err != nil {
		t.Fatalf("VerifyProjectTypeEnvCompositeArtifact(): %v", err)
	}
	if verified.Ref() != artifact.Ref() {
		t.Fatal("verified artifact changed C")
	}
	wrong := projectTypeEnvRefFixture(t, "4")
	if _, err := VerifyProjectTypeEnvCompositeArtifact(wrong, artifact.CanonicalBytes()); err == nil ||
		!strings.Contains(err.Error(), "does not match") {
		t.Fatalf("wrong expected ref error = %v", err)
	}
}

func TestProjectTypeEnvCompositeRejectsMalformedTrailingAndNoncanonicalBytes(t *testing.T) {
	artifact := sealedProjectTypeEnvCompositeArtifactFixture(t)
	valid := artifact.CanonicalBytes()
	encoded := projectTypeEnvCompositeEncodedFixture(t, artifact)

	tests := []struct {
		name      string
		canonical []byte
		contains  string
	}{
		{name: "empty", canonical: nil, contains: "required"},
		{name: "truncated", canonical: valid[:len(valid)-1], contains: "exceeds remaining"},
		{name: "trailing-envelope-byte", canonical: append(append([]byte(nil), valid...), 0), contains: "trailing bytes"},
		{name: "wrong-root", canonical: projectTypeEnvCompositeEnvelopeFixture("wrong.root", projectTypeEnvCompositeArtifactDomain, []byte(`{}`)), contains: "unexpected"},
		{name: "wrong-domain", canonical: projectTypeEnvCompositeEnvelopeFixture(projectTypeEnvCompositeCanonicalDomain, "wrong.artifact", []byte(`{}`)), contains: "unexpected"},
		{name: "invalid-utf8", canonical: projectTypeEnvCompositeEnvelopeFixture(projectTypeEnvCompositeCanonicalDomain, projectTypeEnvCompositeArtifactDomain, []byte{0xff}), contains: "UTF-8"},
		{name: "unknown-field", canonical: projectTypeEnvCompositeEnvelopeFixture(projectTypeEnvCompositeCanonicalDomain, projectTypeEnvCompositeArtifactDomain, projectTypeEnvCompositeJSONWithSuffix(t, encoded, `,"unknown":true}`)), contains: "unknown field"},
		{name: "trailing-json", canonical: projectTypeEnvCompositeEnvelopeFixture(projectTypeEnvCompositeCanonicalDomain, projectTypeEnvCompositeArtifactDomain, append(projectTypeEnvCompositePayloadFixture(t, encoded), []byte(` {}`)...)), contains: "trailing value"},
		{name: "noncanonical-whitespace", canonical: projectTypeEnvCompositeEnvelopeFixture(projectTypeEnvCompositeCanonicalDomain, projectTypeEnvCompositeArtifactDomain, append([]byte(" "), projectTypeEnvCompositePayloadFixture(t, encoded)...)), contains: "not canonical"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := DecodeProjectTypeEnvCompositeArtifact(test.canonical)
			if err == nil || !strings.Contains(err.Error(), test.contains) {
				t.Fatalf("DecodeProjectTypeEnvCompositeArtifact() error = %v, want %q", err, test.contains)
			}
		})
	}

	overflow := make([]byte, 8)
	binary.BigEndian.PutUint64(overflow, ^uint64(0))
	if _, err := DecodeProjectTypeEnvCompositeArtifact(overflow); err == nil ||
		!strings.Contains(err.Error(), "exceeds remaining") {
		t.Fatalf("overflow length error = %v", err)
	}
}

func TestProjectTypeEnvCompositeRejectsInvalidRecipeCoordinates(t *testing.T) {
	artifact := sealedProjectTypeEnvCompositeArtifactFixture(t)
	valid := projectTypeEnvCompositeEncodedFixture(t, artifact)
	duplicate := valid
	duplicate.ExtensionRefs = []string{valid.ExtensionRefs[0], valid.ExtensionRefs[0]}

	tests := []struct {
		name     string
		encoded  projectTypeEnvCompositeCanonicalV1
		contains string
	}{
		{
			name: "base",
			encoded: func() projectTypeEnvCompositeCanonicalV1 {
				value := valid
				value.BaseTypeEnvRef = "not-a-TypeEnv-ref"
				return value
			}(),
			contains: "base reference",
		},
		{
			name: "extension",
			encoded: func() projectTypeEnvCompositeCanonicalV1 {
				value := valid
				value.ExtensionRefs = []string{"not-an-extension-ref"}
				return value
			}(),
			contains: "extension_refs[0]",
		},
		{name: "duplicate-extension", encoded: duplicate, contains: "repeats extension"},
		{
			name: "runtime-basis",
			encoded: func() projectTypeEnvCompositeCanonicalV1 {
				value := valid
				value.RuntimeEvaluationBasisRef = "not-a-runtime-basis-ref"
				return value
			}(),
			contains: "runtime basis reference",
		},
		{
			name: "lowerer",
			encoded: func() projectTypeEnvCompositeCanonicalV1 {
				value := valid
				value.LowererSchemaVersion = "unknown"
				return value
			}(),
			contains: "unsupported",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			canonical := projectTypeEnvCompositeRawCanonicalFixture(t, test.encoded)
			_, err := DecodeProjectTypeEnvCompositeArtifact(canonical)
			if err == nil || !strings.Contains(err.Error(), test.contains) {
				t.Fatalf("DecodeProjectTypeEnvCompositeArtifact() error = %v, want %q", err, test.contains)
			}
		})
	}
}

func TestProjectTypeEnvCompositeEnforcesResourceLimits(t *testing.T) {
	oversized := make([]byte, maximumProjectTypeEnvCompositeArtifactBytes+1)
	if _, err := DecodeProjectTypeEnvCompositeArtifact(oversized); err == nil ||
		!strings.Contains(err.Error(), "limit") {
		t.Fatalf("byte-limit error = %v", err)
	}

	artifact := sealedProjectTypeEnvCompositeArtifactFixture(t)
	encoded := projectTypeEnvCompositeEncodedFixture(t, artifact)
	encoded.ExtensionRefs = make([]string, 0, maximumCompositeExtensionArtifacts+1)
	for index := 0; index <= maximumCompositeExtensionArtifacts; index++ {
		id := fmt.Sprintf("limit.extension.%d", index)
		ref := projectTypeEnvExtensionRefFixture(t, id, "5")
		encoded.ExtensionRefs = append(encoded.ExtensionRefs, ref.String())
	}
	canonical := projectTypeEnvCompositeRawCanonicalFixture(t, encoded)
	if _, err := DecodeProjectTypeEnvCompositeArtifact(canonical); err == nil ||
		!strings.Contains(err.Error(), "extension refs; limit") {
		t.Fatalf("extension-limit error = %v", err)
	}
}

func TestProjectTypeEnvCompositeSealRejectsForgedLinkedProofAndRuntimeBasis(t *testing.T) {
	base, extensions := projectTypeEnvCompositeArtifactFixture(t)
	linked := acceptedCompositeIR(t, LinkProjectTypeEnvCompositeIR(base, extensions))
	basis := emptyRuntimeEvaluationBasisFixture(t)
	linked.canonical[0] ^= 0xff
	if _, err := SealProjectTypeEnvComposite(linked, basis); err == nil ||
		!strings.Contains(err.Error(), "canonical verified") {
		t.Fatalf("forged linked proof error = %v", err)
	}

	linked = acceptedCompositeIR(t, LinkProjectTypeEnvCompositeIR(base, extensions))
	basis.canonical[0] ^= 0xff
	if _, err := SealProjectTypeEnvComposite(linked, basis); err == nil ||
		!strings.Contains(err.Error(), "runtime basis") {
		t.Fatalf("forged runtime basis error = %v", err)
	}
}

func projectTypeEnvCompositeArtifactFixture(
	t *testing.T,
) (typeenv.BaseTypeEnvArtifact, []ProjectTypeEnvExtensionArtifact) {
	t.Helper()
	base := loadBaseArtifact(t)
	alphaCarrier := parseCarrier(t, carrierFixtureInContext(
		t,
		base,
		"alpha.signature",
		"1.0.0",
		"Alpha",
		nil,
		"alpha-project",
	))
	betaCarrier := parseCarrier(t, carrierFixtureInContext(
		t,
		base,
		"beta.signature",
		"1.0.0",
		"Beta",
		[]string{"alpha.signature"},
		"beta-project",
	))
	gammaCarrier := parseCarrier(t, carrierFixtureInContext(
		t,
		base,
		"gamma.signature",
		"1.0.0",
		"Gamma",
		nil,
		"gamma-project",
	))
	bundle := acceptedManifestBundle(
		t,
		base,
		[]localpractice.ParsedCarrier{betaCarrier, gammaCarrier, alphaCarrier},
	)
	nodes := nodesByCoordinate(bundle.Nodes())
	alpha := compileAndSealExtension(t, nodes["alpha.signature@1.0.0"], nil)
	beta := compileAndSealExtension(
		t,
		nodes["beta.signature@1.0.0"],
		[]ProjectTypeEnvExtensionArtifact{alpha},
	)
	gamma := compileAndSealExtension(t, nodes["gamma.signature@1.0.0"], nil)
	return base, []ProjectTypeEnvExtensionArtifact{alpha, beta, gamma}
}

func sealedProjectTypeEnvCompositeArtifactFixture(
	t *testing.T,
) ProjectTypeEnvCompositeArtifact {
	t.Helper()
	base, extensions := projectTypeEnvCompositeArtifactFixture(t)
	linked := acceptedCompositeIR(t, LinkProjectTypeEnvCompositeIR(base, extensions))
	basis := emptyRuntimeEvaluationBasisFixture(t)
	artifact, err := SealProjectTypeEnvComposite(linked, basis)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvComposite(): %v", err)
	}
	return artifact
}

func emptyRuntimeEvaluationBasisFixture(t *testing.T) RuntimeEvaluationBasisArtifact {
	t.Helper()
	basis, err := SealRuntimeEvaluationBasis(nil)
	if err != nil {
		t.Fatalf("SealRuntimeEvaluationBasis(nil): %v", err)
	}
	return basis
}

func projectTypeEnvCompositeEncodedFixture(
	t *testing.T,
	artifact ProjectTypeEnvCompositeArtifact,
) projectTypeEnvCompositeCanonicalV1 {
	t.Helper()
	payload, err := decodeProjectTypeEnvCompositeEnvelope(artifact.CanonicalBytes())
	if err != nil {
		t.Fatalf("decode composite envelope: %v", err)
	}
	encoded := projectTypeEnvCompositeCanonicalV1{}
	if err := json.Unmarshal(payload, &encoded); err != nil {
		t.Fatalf("decode composite test payload: %v", err)
	}
	return encoded
}

func assertProjectTypeEnvCompositeRecipeOnly(
	t *testing.T,
	artifact ProjectTypeEnvCompositeArtifact,
) {
	t.Helper()
	payload, err := decodeProjectTypeEnvCompositeEnvelope(artifact.CanonicalBytes())
	if err != nil {
		t.Fatalf("decode composite envelope: %v", err)
	}
	fields := make(map[string]json.RawMessage)
	if err := json.Unmarshal(payload, &fields); err != nil {
		t.Fatalf("decode composite fields: %v", err)
	}
	want := []string{
		"base_type_env_ref",
		"extension_refs",
		"runtime_evaluation_basis_ref",
		"lowerer_schema_version",
	}
	if len(fields) != len(want) {
		t.Fatalf("composite recipe fields = %v, want exactly %v", fields, want)
	}
	for _, field := range want {
		if _, exists := fields[field]; !exists {
			t.Fatalf("composite recipe is missing %q", field)
		}
	}
}

func projectTypeEnvCompositePayloadFixture(
	t *testing.T,
	encoded projectTypeEnvCompositeCanonicalV1,
) []byte {
	t.Helper()
	payload, err := json.Marshal(encoded)
	if err != nil {
		t.Fatalf("json.Marshal(composite): %v", err)
	}
	return payload
}

func projectTypeEnvCompositeRawCanonicalFixture(
	t *testing.T,
	encoded projectTypeEnvCompositeCanonicalV1,
) []byte {
	t.Helper()
	payload := projectTypeEnvCompositePayloadFixture(t, encoded)
	return projectTypeEnvCompositeEnvelopeFixture(
		projectTypeEnvCompositeCanonicalDomain,
		projectTypeEnvCompositeArtifactDomain,
		payload,
	)
}

func projectTypeEnvCompositeEnvelopeFixture(
	root string,
	domain string,
	payload []byte,
) []byte {
	writer := projectTypeEnvCompositeWriter{}
	writer.addString(root)
	writer.addString(domain)
	writer.addBytes(payload)
	return writer.bytes()
}

func projectTypeEnvCompositeJSONWithSuffix(
	t *testing.T,
	encoded projectTypeEnvCompositeCanonicalV1,
	suffix string,
) []byte {
	t.Helper()
	payload := projectTypeEnvCompositePayloadFixture(t, encoded)
	if len(payload) == 0 || payload[len(payload)-1] != '}' {
		t.Fatal("canonical JSON object terminator is missing")
	}
	return append(payload[:len(payload)-1], []byte(suffix)...)
}

func projectTypeEnvCompositeRefStrings(
	refs []typedmemory.TypeEnvExtensionRef,
) []string {
	result := make([]string, 0, len(refs))
	for _, ref := range refs {
		result = append(result, ref.String())
	}
	return result
}

func projectTypeEnvRefFixture(t *testing.T, fill string) typedmemory.TypeEnvRef {
	t.Helper()
	ref, err := typedmemory.ParseTypeEnvRef("typeenv:sha256:" + strings.Repeat(fill, 64))
	if err != nil {
		t.Fatalf("ParseTypeEnvRef(): %v", err)
	}
	return ref
}

func projectTypeEnvExtensionRefFixture(
	t *testing.T,
	id string,
	fill string,
) typedmemory.TypeEnvExtensionRef {
	t.Helper()
	ref, err := typedmemory.ParseTypeEnvExtensionRef(
		"typeenv-extension:" + id + "@sha256:" + strings.Repeat(fill, 64),
	)
	if err != nil {
		t.Fatalf("ParseTypeEnvExtensionRef(): %v", err)
	}
	return ref
}

func runtimeEvaluationBasisRefFixture(t *testing.T, fill string) RuntimeEvaluationBasisRef {
	t.Helper()
	ref, err := ParseRuntimeEvaluationBasisRef(
		"runtime-evaluation-basis:sha256:" + strings.Repeat(fill, 64),
	)
	if err != nil {
		t.Fatalf("ParseRuntimeEvaluationBasisRef(): %v", err)
	}
	return ref
}
