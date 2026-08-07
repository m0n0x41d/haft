package projecttypeenv

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf/localpractice"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestProjectTypeEnvExtensionArtifactRetainsSymbolicSourceAndRoundTrips(t *testing.T) {
	t.Parallel()

	base := loadBaseArtifact(t)
	alpha := parseCarrier(t, carrierFixture(t, base, "alpha.signature", "1.0.0", "Alpha", nil))
	beta := parseCarrier(t, carrierFixture(
		t,
		base,
		"beta.signature",
		"1.1.0",
		"Beta",
		[]string{"alpha.signature"},
	))
	bundle := acceptedManifestBundle(t, base, []localpractice.ParsedCarrier{beta, alpha})
	nodes := nodesByCoordinate(bundle.Nodes())
	alphaArtifact := compileAndSealExtension(t, nodes["alpha.signature@1.0.0"], nil)
	betaArtifact := compileAndSealExtension(
		t,
		nodes["beta.signature@1.1.0"],
		[]ProjectTypeEnvExtensionArtifact{alphaArtifact},
	)

	baseRef, exists := base.TypeEnvRef()
	if !exists {
		t.Fatal("compiled base artifact has no TypeEnvRef")
	}
	ir := betaArtifact.IR()
	if ir.BaseTypeEnvRef() != baseRef || ir.BaseSource().Value() != baseRef.String() {
		t.Fatal("symbolic extension lost its exact base B reference")
	}
	if ir.Carrier().Digest().String() != beta.Digest().String() {
		t.Fatal("symbolic extension lost the exact raw carrier digest")
	}
	if ir.Carrier().ID().Value() != "beta.signature" || ir.Carrier().Edition().Value() != "1.1.0" {
		t.Fatal("symbolic extension lost carrier identity/edition")
	}
	predecessors := ir.Manifest().DirectPredecessors()
	if len(predecessors) != 1 || predecessors[0].Ref() != alphaArtifact.Ref() {
		t.Fatalf("direct predecessors = %#v; want exact alpha E-ref", predecessors)
	}
	rows := ir.Signature()
	if rows.SubjectBlock().Name() != "subject_block" || rows.Laws().Name() != "laws" || rows.Applicability().Name() != "applicability" {
		t.Fatal("artifact did not retain the exact four-row Signature structure")
	}
	if !rows.Span().valid() || !rows.Vocabulary().Span().valid() {
		t.Fatal("artifact did not retain Signature source coordinates")
	}
	declarations := rows.Vocabulary().Declarations()
	if len(declarations) != 10 {
		t.Fatalf("symbolic declarations = %d; want complete fixture vocabulary", len(declarations))
	}
	legacyRelation := declarationByKind(
		t,
		&ir,
		localpractice.DeclarationRelationSignature,
	)
	posture, classified := legacyRelation.RelationDeclarationPosture()
	if !classified || posture != localpractice.RelationDeclarationTypedFragment {
		t.Fatalf("legacy relation_signature semantic posture = %q, %v", posture, classified)
	}
	if !hasSymbolicDependency(declarations, "Beta.ProjectConcernRef", "value_kind", "Beta.ProjectConcern") {
		t.Fatal("RefKind symbolic dependency was not retained")
	}
	if betaArtifact.Ref().ID().String() != "beta.signature" {
		t.Fatalf("derived E-ref ID = %q; want manifest ID", betaArtifact.Ref().ID().String())
	}
	digest, err := projectExtensionDigest(betaArtifact.CanonicalBytes())
	if err != nil {
		t.Fatalf("projectExtensionDigest() error = %v", err)
	}
	if betaArtifact.Ref().Digest() != digest {
		t.Fatal("derived E-ref digest is not SHA-256 of exact canonical bytes")
	}

	decoded, err := DecodeProjectTypeEnvExtensionArtifact(betaArtifact.CanonicalBytes())
	if err != nil {
		t.Fatalf("DecodeProjectTypeEnvExtensionArtifact() error = %v", err)
	}
	if decoded.Ref() != betaArtifact.Ref() || !bytes.Equal(decoded.CanonicalBytes(), betaArtifact.CanonicalBytes()) {
		t.Fatal("decode/reseal changed exact artifact identity")
	}
	if err := decoded.Verify(); err != nil {
		t.Fatalf("decoded.Verify() error = %v", err)
	}
	if _, hasActivate := reflect.TypeOf(betaArtifact).MethodByName("Activate"); hasActivate {
		t.Fatal("pure E artifact exposes activation")
	}
}

func TestProjectTypeEnvExtensionSealIsPermutationInvariant(t *testing.T) {
	t.Parallel()

	base := loadBaseArtifact(t)
	alpha := parseCarrier(t, carrierFixture(t, base, "alpha.signature", "1.0.0", "Alpha", nil))
	gamma := parseCarrier(t, carrierFixture(t, base, "gamma.signature", "1.0.0", "Gamma", nil))
	delta := parseCarrier(t, carrierFixture(
		t,
		base,
		"delta.signature",
		"2.0.0",
		"Delta",
		[]string{"gamma.signature", "alpha.signature"},
	))
	bundle := acceptedManifestBundle(
		t,
		base,
		[]localpractice.ParsedCarrier{delta, gamma, alpha},
	)
	nodes := nodesByCoordinate(bundle.Nodes())
	alphaArtifact := compileAndSealExtension(t, nodes["alpha.signature@1.0.0"], nil)
	gammaArtifact := compileAndSealExtension(t, nodes["gamma.signature@1.0.0"], nil)
	deltaNode := nodes["delta.signature@2.0.0"]
	forwardIR := compileExtensionIR(
		t,
		deltaNode,
		[]ProjectTypeEnvExtensionArtifact{alphaArtifact, gammaArtifact},
	)
	reverseIR := compileExtensionIR(
		t,
		deltaNode,
		[]ProjectTypeEnvExtensionArtifact{gammaArtifact, alphaArtifact},
	)
	reverseSlice(reverseIR.manifest.predecessors)
	reverseSlice(reverseIR.manifest.provides)
	reverseSlice(reverseIR.signature.vocabulary.declarations)
	for index := range reverseIR.signature.vocabulary.declarations {
		declaration := &reverseIR.signature.vocabulary.declarations[index]
		reverseSlice(declaration.exports)
		reverseSlice(declaration.facts)
		reverseSlice(declaration.dependencies)
	}
	forward := sealExtension(t, forwardIR)
	reverse := sealExtension(t, reverseIR)

	if forward.Ref() != reverse.Ref() || !bytes.Equal(forward.CanonicalBytes(), reverse.CanonicalBytes()) {
		t.Fatal("permuting unordered IR collections changed E artifact identity")
	}
}

func TestProjectTypeEnvExtensionIdentityIsSensitiveToExactPredecessor(t *testing.T) {
	t.Parallel()

	base := loadBaseArtifact(t)
	alphaSource := carrierFixture(t, base, "alpha.signature", "1.0.0", "Alpha", nil)
	changedAlphaSource := bytes.Replace(
		alphaSource,
		[]byte("Schema declarations remain outside MemoryChangeSet."),
		[]byte("Schema declarations remain outside every MemoryChangeSet."),
		1,
	)
	alpha := parseCarrier(t, alphaSource)
	changedAlpha := parseCarrier(t, changedAlphaSource)
	beta := parseCarrier(t, carrierFixture(
		t,
		base,
		"beta.signature",
		"1.0.0",
		"Beta",
		[]string{"alpha.signature"},
	))
	firstBundle := acceptedManifestBundle(t, base, []localpractice.ParsedCarrier{alpha, beta})
	changedBundle := acceptedManifestBundle(t, base, []localpractice.ParsedCarrier{changedAlpha})
	firstNodes := nodesByCoordinate(firstBundle.Nodes())
	changedNodes := nodesByCoordinate(changedBundle.Nodes())
	alphaArtifact := compileAndSealExtension(t, firstNodes["alpha.signature@1.0.0"], nil)
	changedAlphaArtifact := compileAndSealExtension(t, changedNodes["alpha.signature@1.0.0"], nil)
	if alphaArtifact.Ref() == changedAlphaArtifact.Ref() {
		t.Fatal("changing exact predecessor source did not change its E-ref")
	}
	betaNode := firstNodes["beta.signature@1.0.0"]
	firstBeta := compileAndSealExtension(
		t,
		betaNode,
		[]ProjectTypeEnvExtensionArtifact{alphaArtifact},
	)
	changedBeta := compileAndSealExtension(
		t,
		betaNode,
		[]ProjectTypeEnvExtensionArtifact{changedAlphaArtifact},
	)
	if firstBeta.Ref() == changedBeta.Ref() {
		t.Fatal("child E-ref is insensitive to exact predecessor E-ref")
	}
	if firstBeta.IR().Carrier().Digest() != changedBeta.IR().Carrier().Digest() {
		t.Fatal("predecessor sensitivity test unexpectedly changed child source")
	}
}

func TestProjectTypeEnvExtensionRejectsPredecessorAndManifestClosureErrors(t *testing.T) {
	t.Parallel()

	base := loadBaseArtifact(t)
	alpha := parseCarrier(t, carrierFixture(t, base, "alpha.signature", "1.0.0", "Alpha", nil))
	beta := parseCarrier(t, carrierFixture(
		t,
		base,
		"beta.signature",
		"1.0.0",
		"Beta",
		[]string{"alpha.signature"},
	))
	bundle := acceptedManifestBundle(t, base, []localpractice.ParsedCarrier{alpha, beta})
	nodes := nodesByCoordinate(bundle.Nodes())
	alphaArtifact := compileAndSealExtension(t, nodes["alpha.signature@1.0.0"], nil)

	if _, err := CompileProjectTypeEnvExtensionIR(nodes["beta.signature@1.0.0"], nil); err == nil || !strings.Contains(err.Error(), "missing exact predecessor") {
		t.Fatalf("missing predecessor error = %v", err)
	}
	if _, err := CompileProjectTypeEnvExtensionIR(
		nodes["alpha.signature@1.0.0"],
		[]ProjectTypeEnvExtensionArtifact{alphaArtifact},
	); err == nil || !strings.Contains(err.Error(), "not imported") {
		t.Fatalf("extra predecessor error = %v", err)
	}

	invalidSource := carrierFixture(t, base, "invalid.signature", "1.0.0", "Invalid", nil)
	invalidSource = bytes.Replace(
		invalidSource,
		[]byte("    - Invalid.ProjectConcernRef\n"),
		nil,
		1,
	)
	invalid := parseCarrier(t, invalidSource)
	invalidBundle := acceptedManifestBundle(t, base, []localpractice.ParsedCarrier{invalid})
	invalidNode := invalidBundle.Nodes()[0]
	_, err := CompileProjectTypeEnvExtensionIR(invalidNode, nil)
	if err == nil || !strings.Contains(err.Error(), "symbolic declarations export") {
		t.Fatalf("manifest closure error = %v; want exact export equality rejection", err)
	}
}

func TestProjectTypeEnvExtensionRejectsPredecessorFromDifferentBase(t *testing.T) {
	t.Parallel()

	base := loadBaseArtifact(t)
	alpha := parseCarrier(t, carrierFixture(t, base, "alpha.signature", "1.0.0", "Alpha", nil))
	beta := parseCarrier(t, carrierFixture(
		t,
		base,
		"beta.signature",
		"1.0.0",
		"Beta",
		[]string{"alpha.signature"},
	))
	bundle := acceptedManifestBundle(t, base, []localpractice.ParsedCarrier{alpha, beta})
	nodes := nodesByCoordinate(bundle.Nodes())
	alphaArtifact := compileAndSealExtension(t, nodes["alpha.signature@1.0.0"], nil)

	mismatchedIR := alphaArtifact.IR()
	mismatchedBase, err := typedmemory.ParseTypeEnvRef(
		"typeenv:sha256:" + strings.Repeat("b", 64),
	)
	if err != nil {
		t.Fatalf("ParseTypeEnvRef() error = %v", err)
	}
	mismatchedIR.baseTypeEnv = mismatchedBase
	mismatchedIR.baseSource.value = mismatchedBase.String()
	mismatchedArtifact := sealExtension(t, mismatchedIR)

	_, err = CompileProjectTypeEnvExtensionIR(
		nodes["beta.signature@1.0.0"],
		[]ProjectTypeEnvExtensionArtifact{mismatchedArtifact},
	)
	if err == nil || !strings.Contains(err.Error(), "does not match child base") {
		t.Fatalf("predecessor base mismatch error = %v", err)
	}
}

func TestProjectTypeEnvExtensionRejectsClosedSymbolicSchemaViolations(t *testing.T) {
	t.Parallel()

	artifact := standaloneExtensionArtifact(t, "alpha.signature", "1.0.0", "Alpha")
	cases := []struct {
		name   string
		mutate func(*testing.T, *ProjectTypeEnvExtensionIR)
		want   string
	}{
		{
			name: "unknown source fact",
			mutate: func(t *testing.T, ir *ProjectTypeEnvExtensionIR) {
				declaration := declarationByKind(t, ir, localpractice.DeclarationValueKind)
				declaration.facts = append(declaration.facts, SourceFact{
					path: "invented_path",
					value: SourceScalar{
						value: "invented",
						span:  declaration.span,
					},
				})
			},
			want: "outside the value_kind schema",
		},
		{
			name: "fact dependency divergence",
			mutate: func(t *testing.T, ir *ProjectTypeEnvExtensionIR) {
				declaration := declarationByKind(t, ir, localpractice.DeclarationRefKind)
				declaration.dependencies[0].target.value = "Alpha.OtherConcern"
			},
			want: "does not exactly match its source fact",
		},
		{
			name: "unexpected export",
			mutate: func(t *testing.T, ir *ProjectTypeEnvExtensionIR) {
				declaration := declarationByKind(t, ir, localpractice.DeclarationValueKind)
				declaration.exports = append(declaration.exports, SourceScalar{
					value: "Alpha.InventedExport",
					span:  declaration.span,
				})
			},
			want: "outside the value_kind schema",
		},
		{
			name: "by-value slot with ref kind",
			mutate: func(t *testing.T, ir *ProjectTypeEnvExtensionIR) {
				declaration := declarationByKind(t, ir, localpractice.DeclarationRelationSignature)
				prefix := keyedPath("slots", "Alpha.EvidenceSlot")
				refKind := SourceScalar{
					value: "Alpha.ProjectConcernRef",
					span:  declaration.span,
				}
				declaration.facts = append(declaration.facts, SourceFact{
					path:  prefix + ".ref_mode.ref_kind",
					value: refKind,
				})
				declaration.dependencies = append(
					declaration.dependencies,
					SymbolicDependency{
						role:   prefix + ".ref_mode.ref_kind",
						target: refKind,
					},
				)
			},
			want: "by_value slot",
		},
	}
	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			ir := artifact.IR()
			testCase.mutate(t, &ir)
			_, err := SealProjectTypeEnvExtension(ir)
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("SealProjectTypeEnvExtension() error = %v, want %q", err, testCase.want)
			}
		})
	}
}

func TestProjectTypeEnvExtensionRoundTripsEveryClosedSourceVariant(t *testing.T) {
	t.Parallel()

	base := loadBaseArtifact(t)
	baseline := string(carrierFixture(
		t,
		base,
		"variant.signature",
		"1.0.0",
		"Variant",
		nil,
	))
	cases := []struct {
		name        string
		old         string
		replacement string
	}{
		{
			name: "record value shape",
			old:  "          kind: scalar\n          scalar_kind: text",
			replacement: "          kind: record\n" +
				"          fields:\n" +
				"            - name: title\n" +
				"              shape: Variant.Shape.Text",
		},
		{
			name: "sum value shape",
			old:  "          kind: scalar\n          scalar_kind: text",
			replacement: "          kind: sum\n" +
				"          variants:\n" +
				"            - name: known\n" +
				"              shape: Variant.Shape.Text",
		},
		{
			name: "ordered sequence value shape",
			old:  "          kind: scalar\n          scalar_kind: text",
			replacement: "          kind: ordered_sequence\n" +
				"          element_shape: Variant.Shape.Text",
		},
		{
			name: "unordered set value shape",
			old:  "          kind: scalar\n          scalar_kind: text",
			replacement: "          kind: unordered_set\n" +
				"          element_shape: Variant.Shape.Text",
		},
		{
			name:        "claim graph value shape",
			old:         "          kind: scalar\n          scalar_kind: text",
			replacement: "          kind: claim_graph",
		},
		{
			name: "kind-disjoint constraint",
			old: "          kind: slot_group\n" +
				"          relation: Variant.ConcernMemory\n" +
				"          slots:\n" +
				"            - Variant.ConcernSlot\n" +
				"            - Variant.EvidenceSlot\n" +
				"          mode: at_least_one",
			replacement: "          kind: kind_disjoint\n" +
				"          kinds:\n" +
				"            - Variant.ProjectConcern\n" +
				"            - U.Entity",
		},
		{
			name: "prior-batch entity-set policy",
			old:  "          kind: persisted_entities_only",
			replacement: "          kind: prior_batch_declarations_visible\n" +
				"          evaluation_rule: haft.rule.project-entities-prior-batch/v1",
		},
		{
			name: "kind-signature assumption",
			old:  "        assumptions: []",
			replacement: "        assumptions:\n" +
				"          - carrier_ref: carrier:domain-model\n" +
				"            edition: 2.0.0\n" +
				"            digest: sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		},
	}
	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			source := strings.Replace(baseline, testCase.old, testCase.replacement, 1)
			if source == baseline {
				t.Fatal("test replacement did not change Local-Practice carrier")
			}
			carrier := parseCarrier(t, []byte(source))
			bundle := acceptedManifestBundle(t, base, []localpractice.ParsedCarrier{carrier})
			artifact := compileAndSealExtension(t, bundle.Nodes()[0], nil)
			if err := artifact.Verify(); err != nil {
				t.Fatalf("artifact.Verify() error = %v", err)
			}
		})
	}
}

func TestProjectTypeEnvExtensionCodecRejectsForgeryMutationTrailingAndNoncanonical(t *testing.T) {
	t.Parallel()

	artifact := standaloneExtensionArtifact(t, "alpha.signature", "1.0.0", "Alpha")
	forgedDigest, err := typedmemory.NewSHA256Digest(
		"sha256:" + strings.Repeat("f", 64),
	)
	if err != nil {
		t.Fatalf("NewSHA256Digest() error = %v", err)
	}
	forged, err := deriveProjectExtensionRef("alpha.signature", forgedDigest)
	if err != nil {
		t.Fatalf("deriveProjectExtensionRef() error = %v", err)
	}
	if _, err := VerifyProjectTypeEnvExtensionArtifact(forged, artifact.CanonicalBytes()); err == nil {
		t.Fatal("forged asserted E-ref was accepted")
	}

	mutated := artifact.CanonicalBytes()
	needle := []byte("Alpha.ProjectConcern")
	index := bytes.Index(mutated, needle)
	if index < 0 {
		t.Fatalf("canonical artifact does not contain %q", needle)
	}
	mutated[index] = 'Z'
	if _, err := VerifyProjectTypeEnvExtensionArtifact(artifact.Ref(), mutated); err == nil {
		t.Fatal("mutated artifact was accepted under its old E-ref")
	}

	trailing := append(artifact.CanonicalBytes(), 0)
	if _, err := DecodeProjectTypeEnvExtensionArtifact(trailing); err == nil || !strings.Contains(err.Error(), "trailing") {
		t.Fatalf("trailing error = %v; want explicit rejection", err)
	}

	payload := decodeProjectExtensionPayloadForTest(t, artifact.CanonicalBytes())
	var encoded projectExtensionCanonicalV1
	if err := json.Unmarshal(payload, &encoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	reverseSlice(encoded.Signature.Vocabulary.Declarations)
	noncanonicalPayload, err := json.Marshal(encoded)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	noncanonical := wrapProjectExtensionPayloadForTest(noncanonicalPayload)
	if _, err := DecodeProjectTypeEnvExtensionArtifact(noncanonical); err == nil || !strings.Contains(err.Error(), "not canonical") {
		t.Fatalf("noncanonical-order error = %v; want exact reseal rejection", err)
	}

	invalidUTF8Payload := append([]byte(nil), payload...)
	utf8Needle := []byte(localpractice.SchemaVersion)
	index = bytes.Index(invalidUTF8Payload, utf8Needle)
	if index < 0 {
		t.Fatalf("canonical payload does not contain %q", utf8Needle)
	}
	invalidUTF8Payload[index] = 0xff
	invalidUTF8 := wrapProjectExtensionPayloadForTest(invalidUTF8Payload)
	if _, err := DecodeProjectTypeEnvExtensionArtifact(invalidUTF8); err == nil || !strings.Contains(err.Error(), "invalid UTF-8") {
		t.Fatalf("invalid UTF-8 error = %v; want decode-side rejection", err)
	}
}

func TestProjectTypeEnvExtensionDecodeRejectsSourceImpossibleSemanticScalars(t *testing.T) {
	t.Parallel()

	artifact := standaloneExtensionArtifact(t, "alpha.signature", "1.0.0", "Alpha")
	cases := []struct {
		name  string
		value string
	}{
		{name: "Unicode control", value: "unsafe\x00contract"},
		{name: "surrounding whitespace", value: " contract with leading space"},
	}
	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			encoded := decodeProjectExtensionCanonicalForTest(t, artifact.CanonicalBytes())
			declaration := canonicalDeclarationByKindForTest(
				t,
				&encoded,
				localpractice.DeclarationCodecBinding,
			)
			changed := false
			for index := range declaration.Facts {
				fact := &declaration.Facts[index]
				if strings.HasPrefix(fact.Path, "contract[") {
					fact.Value.Value = testCase.value
					changed = true
					break
				}
			}
			if !changed {
				t.Fatal("codec declaration has no contract fact")
			}
			canonical := encodeProjectExtensionCanonicalForTest(t, encoded)
			if _, err := DecodeProjectTypeEnvExtensionArtifact(canonical); err == nil {
				t.Fatalf("DecodeProjectTypeEnvExtensionArtifact() accepted %s", testCase.name)
			}
		})
	}
}

func TestProjectTypeEnvExtensionDecodeRejectsNoncanonicalPredecessorCoordinates(t *testing.T) {
	t.Parallel()

	base := loadBaseArtifact(t)
	alpha := parseCarrier(t, carrierFixture(t, base, "alpha.signature", "1.0.0", "Alpha", nil))
	beta := parseCarrier(t, carrierFixture(
		t,
		base,
		"beta.signature",
		"1.0.0",
		"Beta",
		[]string{"alpha.signature"},
	))
	bundle := acceptedManifestBundle(t, base, []localpractice.ParsedCarrier{alpha, beta})
	nodes := nodesByCoordinate(bundle.Nodes())
	alphaArtifact := compileAndSealExtension(t, nodes["alpha.signature@1.0.0"], nil)
	betaArtifact := compileAndSealExtension(
		t,
		nodes["beta.signature@1.0.0"],
		[]ProjectTypeEnvExtensionArtifact{alphaArtifact},
	)
	cases := []struct {
		name    string
		version string
	}{
		{name: "empty version", version: ""},
		{name: "noncanonical SemVer", version: "01.0.0"},
	}
	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			encoded := decodeProjectExtensionCanonicalForTest(t, betaArtifact.CanonicalBytes())
			if len(encoded.Manifest.Predecessors) != 1 {
				t.Fatalf("predecessors = %d, want 1", len(encoded.Manifest.Predecessors))
			}
			encoded.Manifest.Predecessors[0].ManifestVersion = testCase.version
			canonical := encodeProjectExtensionCanonicalForTest(t, encoded)
			_, err := DecodeProjectTypeEnvExtensionArtifact(canonical)
			if err == nil || !strings.Contains(err.Error(), "predecessor coordinate is invalid") {
				t.Fatalf("DecodeProjectTypeEnvExtensionArtifact() error = %v", err)
			}
		})
	}
}

func TestProjectTypeEnvExtensionDecodeRejectsNoncanonicalSignatureRowIndices(t *testing.T) {
	t.Parallel()

	artifact := standaloneExtensionArtifact(t, "alpha.signature", "1.0.0", "Alpha")
	cases := []struct {
		name   string
		mutate func(*projectExtensionCanonicalV1) bool
	}{
		{
			name: "nonnumeric laws index",
			mutate: func(encoded *projectExtensionCanonicalV1) bool {
				for index := range encoded.Signature.Laws.Facts {
					fact := &encoded.Signature.Laws.Facts[index]
					if strings.HasPrefix(fact.Path, "constraint_refs[") {
						fact.Path = "constraint_refs[x]"
						return true
					}
				}
				return false
			},
		},
		{
			name: "non-dense laws index",
			mutate: func(encoded *projectExtensionCanonicalV1) bool {
				for index := range encoded.Signature.Laws.Facts {
					fact := &encoded.Signature.Laws.Facts[index]
					if fact.Path == "constraint_refs[000000]" {
						fact.Path = "constraint_refs[000099]"
						return true
					}
				}
				return false
			},
		},
		{
			name: "non-dense applicability index",
			mutate: func(encoded *projectExtensionCanonicalV1) bool {
				for index := range encoded.Signature.Applicability.Facts {
					fact := &encoded.Signature.Applicability.Facts[index]
					if fact.Path == "assumptions[000000]" {
						fact.Path = "assumptions[000001]"
						return true
					}
				}
				return false
			},
		},
	}
	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			encoded := decodeProjectExtensionCanonicalForTest(t, artifact.CanonicalBytes())
			if !testCase.mutate(&encoded) {
				t.Fatal("test mutation did not find the target row fact")
			}
			canonical := encodeProjectExtensionCanonicalForTest(t, encoded)
			if _, err := DecodeProjectTypeEnvExtensionArtifact(canonical); err == nil {
				t.Fatalf("DecodeProjectTypeEnvExtensionArtifact() accepted %s", testCase.name)
			}
		})
	}
}

func TestProjectTypeEnvExtensionDecodeRejectsSourceImpossibleSignatureRowValues(t *testing.T) {
	t.Parallel()

	artifact := standaloneExtensionArtifact(t, "alpha.signature", "1.0.0", "Alpha")
	cases := []struct {
		name   string
		mutate func(*projectExtensionCanonicalV1) bool
	}{
		{
			name: "subject ValueKind is not qualified",
			mutate: func(encoded *projectExtensionCanonicalV1) bool {
				for index := range encoded.Signature.Subject.Facts {
					fact := &encoded.Signature.Subject.Facts[index]
					if fact.Path == "subject_kind" {
						fact.Value.Value = "bad kind"
						return true
					}
				}
				return false
			},
		},
		{
			name: "constraint reference is not qualified",
			mutate: func(encoded *projectExtensionCanonicalV1) bool {
				for index := range encoded.Signature.Laws.Facts {
					fact := &encoded.Signature.Laws.Facts[index]
					if fact.Path == "constraint_refs[000000]" {
						fact.Value.Value = "bad ref"
						return true
					}
				}
				return false
			},
		},
	}
	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			encoded := decodeProjectExtensionCanonicalForTest(t, artifact.CanonicalBytes())
			if !testCase.mutate(&encoded) {
				t.Fatal("test mutation did not find the target row fact")
			}
			canonical := encodeProjectExtensionCanonicalForTest(t, encoded)
			if _, err := DecodeProjectTypeEnvExtensionArtifact(canonical); err == nil {
				t.Fatalf("DecodeProjectTypeEnvExtensionArtifact() accepted %s", testCase.name)
			}
		})
	}
}

func TestProjectTypeEnvExtensionArtifactDefensivelyCopiesState(t *testing.T) {
	t.Parallel()

	artifact := standaloneExtensionArtifact(t, "alpha.signature", "1.0.0", "Alpha")
	canonical := artifact.CanonicalBytes()
	canonical[0] ^= 0xff
	if bytes.Equal(canonical, artifact.CanonicalBytes()) {
		t.Fatal("CanonicalBytes() leaked mutable artifact storage")
	}
	ir := artifact.IR()
	ir.manifest.provides[0] = SourceScalar{}
	ir.signature.vocabulary.declarations[0].facts = nil
	if !artifact.IR().manifest.provides[0].valid() {
		t.Fatal("IR() leaked mutable manifest storage")
	}
	if len(artifact.IR().signature.vocabulary.declarations[0].facts) == 0 {
		t.Fatal("IR() leaked mutable declaration storage")
	}
}

func TestProjectTypeEnvExtensionArtifactVerifyRejectsDivergentStoredState(t *testing.T) {
	t.Parallel()

	artifact := standaloneExtensionArtifact(t, "alpha.signature", "1.0.0", "Alpha")
	t.Run("stored IR differs from canonical bytes", func(t *testing.T) {
		forged := artifact
		forged.ir = artifact.IR()
		forged.ir.boundedContext.value = "different-project"
		for index := range forged.ir.signature.applicability.facts {
			fact := &forged.ir.signature.applicability.facts[index]
			if fact.path == "bounded_context_ref" {
				fact.value.value = "different-project"
			}
		}
		if err := forged.Verify(); err == nil || !strings.Contains(err.Error(), "stored IR does not exactly encode") {
			t.Fatalf("Verify() error = %v; want stored IR divergence", err)
		}
	})
	t.Run("stored reference differs from canonical bytes", func(t *testing.T) {
		forged := artifact
		forgedDigest, err := typedmemory.NewSHA256Digest(
			"sha256:" + strings.Repeat("f", 64),
		)
		if err != nil {
			t.Fatalf("NewSHA256Digest() error = %v", err)
		}
		forged.ref, err = deriveProjectExtensionRef("alpha.signature", forgedDigest)
		if err != nil {
			t.Fatalf("deriveProjectExtensionRef() error = %v", err)
		}
		if err := forged.Verify(); err == nil || !strings.Contains(err.Error(), "reference is not derived") {
			t.Fatalf("Verify() error = %v; want reference divergence", err)
		}
	})
	t.Run("stored canonical bytes differ from IR and reference", func(t *testing.T) {
		other := standaloneExtensionArtifact(t, "other.signature", "1.0.0", "Other")
		forged := artifact
		forged.canonical = other.CanonicalBytes()
		if err := forged.Verify(); err == nil || !strings.Contains(err.Error(), "reference is not derived") {
			t.Fatalf("Verify() error = %v; want canonical divergence", err)
		}
	})
}

func standaloneExtensionArtifact(
	t *testing.T,
	id string,
	version string,
	prefix string,
) ProjectTypeEnvExtensionArtifact {
	t.Helper()
	base := loadBaseArtifact(t)
	carrier := parseCarrier(t, carrierFixture(t, base, id, version, prefix, nil))
	bundle := acceptedManifestBundle(t, base, []localpractice.ParsedCarrier{carrier})
	return compileAndSealExtension(t, bundle.Nodes()[0], nil)
}

func acceptedManifestBundle(
	t *testing.T,
	base typeenv.BaseTypeEnvArtifact,
	carriers []localpractice.ParsedCarrier,
) ResolvedManifestBundle {
	t.Helper()
	resolution := ResolveManifestGraph(base, carriers)
	if resolution.Rejected() {
		t.Fatalf("ResolveManifestGraph() rejected: %#v", resolution.Issues())
	}
	bundle, exists := resolution.Bundle()
	if !exists {
		t.Fatal("accepted manifest resolution has no bundle")
	}
	return bundle
}

func nodesByCoordinate(nodes []ResolvedManifestNode) map[string]ResolvedManifestNode {
	result := make(map[string]ResolvedManifestNode, len(nodes))
	for _, node := range nodes {
		result[node.Coordinate().String()] = node
	}
	return result
}

func compileExtensionIR(
	t *testing.T,
	node ResolvedManifestNode,
	predecessors []ProjectTypeEnvExtensionArtifact,
) ProjectTypeEnvExtensionIR {
	t.Helper()
	ir, err := CompileProjectTypeEnvExtensionIR(node, predecessors)
	if err != nil {
		t.Fatalf("CompileProjectTypeEnvExtensionIR() error = %v", err)
	}
	return ir
}

func sealExtension(
	t *testing.T,
	ir ProjectTypeEnvExtensionIR,
) ProjectTypeEnvExtensionArtifact {
	t.Helper()
	artifact, err := SealProjectTypeEnvExtension(ir)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvExtension() error = %v", err)
	}
	return artifact
}

func compileAndSealExtension(
	t *testing.T,
	node ResolvedManifestNode,
	predecessors []ProjectTypeEnvExtensionArtifact,
) ProjectTypeEnvExtensionArtifact {
	t.Helper()
	ir := compileExtensionIR(t, node, predecessors)
	return sealExtension(t, ir)
}

func hasSymbolicDependency(
	declarations []SymbolicDeclaration,
	symbol string,
	role string,
	target string,
) bool {
	for _, declaration := range declarations {
		if declaration.Symbol().Value() != symbol {
			continue
		}
		for _, dependency := range declaration.Dependencies() {
			if dependency.Role() == role && dependency.Target().Value() == target {
				return true
			}
		}
	}
	return false
}

func declarationByKind(
	t *testing.T,
	ir *ProjectTypeEnvExtensionIR,
	kind localpractice.DeclarationKind,
) *SymbolicDeclaration {
	t.Helper()
	for index := range ir.signature.vocabulary.declarations {
		declaration := &ir.signature.vocabulary.declarations[index]
		if declaration.kind == kind {
			return declaration
		}
	}
	t.Fatalf("symbolic declaration kind %q was not found", kind)
	return nil
}

func decodeProjectExtensionPayloadForTest(t *testing.T, canonical []byte) []byte {
	t.Helper()
	payload, err := decodeProjectExtensionEnvelope(canonical)
	if err != nil {
		t.Fatalf("decodeProjectExtensionEnvelope() error = %v", err)
	}
	return payload
}

func decodeProjectExtensionCanonicalForTest(
	t *testing.T,
	canonical []byte,
) projectExtensionCanonicalV1 {
	t.Helper()
	payload := decodeProjectExtensionPayloadForTest(t, canonical)
	var encoded projectExtensionCanonicalV1
	if err := json.Unmarshal(payload, &encoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	return encoded
}

func encodeProjectExtensionCanonicalForTest(
	t *testing.T,
	encoded projectExtensionCanonicalV1,
) []byte {
	t.Helper()
	payload, err := json.Marshal(encoded)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return wrapProjectExtensionPayloadForTest(payload)
}

func canonicalDeclarationByKindForTest(
	t *testing.T,
	encoded *projectExtensionCanonicalV1,
	kind localpractice.DeclarationKind,
) *symbolicDeclarationCanonicalV1 {
	t.Helper()
	for index := range encoded.Signature.Vocabulary.Declarations {
		declaration := &encoded.Signature.Vocabulary.Declarations[index]
		if declaration.Kind == string(kind) {
			return declaration
		}
	}
	t.Fatalf("canonical declaration kind %q was not found", kind)
	return nil
}

func wrapProjectExtensionPayloadForTest(payload []byte) []byte {
	writer := newProjectExtensionWriter(projectExtensionArtifactDomain)
	writer.addBytes(payload)
	return writer.bytes()
}

func reverseSlice[T any](values []T) {
	for left, right := 0, len(values)-1; left < right; left, right = left+1, right-1 {
		values[left], values[right] = values[right], values[left]
	}
}
