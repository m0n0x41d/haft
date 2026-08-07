package localpractice

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestParseGoldenCarrierPreservesSourceIdentityAndRows(t *testing.T) {
	source := readGoldenCarrier(t)
	parsed, err := Parse(source)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	digest := sha256.Sum256(source)
	wantDigest := "sha256:" + hex.EncodeToString(digest[:])
	if parsed.Digest().String() != wantDigest {
		t.Fatalf("Digest() = %q, want %q", parsed.Digest().String(), wantDigest)
	}

	carrier := parsed.Carrier()
	if carrier.SchemaVersion().Value() != SchemaVersion {
		t.Fatalf("SchemaVersion() = %q", carrier.SchemaVersion().Value())
	}
	if carrier.Identity().ID().Value() != "haft.typed-memory" {
		t.Fatalf("carrier ID = %q", carrier.Identity().ID().Value())
	}
	if carrier.Identity().Edition().Value() != "1.0.0" {
		t.Fatalf("carrier edition = %q", carrier.Identity().Edition().Value())
	}
	assertRange(t, "carrier", carrier.Span(), 1, 116)
	assertRange(t, "manifest", carrier.Manifest().Span(), 8, 26)

	block := carrier.Signature()
	assertRange(t, "SubjectBlock", block.SubjectBlock().Span(), 28, 32)
	assertRange(t, "Vocabulary", block.Vocabulary().Span(), 33, 105)
	assertRange(t, "Laws", block.Laws().Span(), 106, 112)
	assertRange(t, "Applicability", block.Applicability().Span(), 113, 116)

	manifest := carrier.Manifest()
	if len(manifest.Imports()) != 0 {
		t.Fatalf("manifest imports = %#v", manifest.Imports())
	}
	if len(manifest.Provides()) != 12 {
		t.Fatalf("manifest provides = %d, want 12", len(manifest.Provides()))
	}
	assertRange(t, "first provided symbol", manifest.Provides()[0].Symbol().Span(), 15, 15)
	state, present := manifest.PublicationState()
	if !present || state != PublicationCandidate {
		t.Fatalf("publication state = %q, %v", state, present)
	}

	declarations := block.Vocabulary().Declarations()
	if len(declarations) != 10 {
		t.Fatalf("declarations = %d, want 10", len(declarations))
	}
	assertDeclaration(t, declarations[0], DeclarationValueKind, "Haft.ProjectConcern", 35, 36)
	assertDeclaration(t, declarations[1], DeclarationRefKind, "Haft.ProjectConcernRef", 37, 39)
	assertDeclaration(t, declarations[2], DeclarationEntitySet, "Haft.ProjectEntities", 40, 44)
	assertDeclaration(
		t,
		declarations[3],
		DeclarationKindSignature,
		"Haft.ProjectConcern.Signature",
		45,
		53,
	)
	assertDeclaration(t, declarations[4], DeclarationRelationSignature, "Haft.ConcernMemory", 54, 65)
	fragment, ok := declarations[4].(TypedRelationDeclarationFragmentDeclaration)
	if !ok {
		t.Fatalf("legacy relation_signature declaration type = %T", declarations[4])
	}
	if fragment.Posture() != RelationDeclarationTypedFragment {
		t.Fatalf("relation declaration posture = %q", fragment.Posture())
	}
	assertDeclaration(t, declarations[5], DeclarationValueShape, "Haft.Shape.Text", 66, 70)
	assertDeclaration(t, declarations[6], DeclarationCodecBinding, "Haft.Codec.Text", 71, 78)
	assertDeclaration(
		t,
		declarations[7],
		DeclarationConstraint,
		"Haft.Constraint.RequiredConcern",
		79,
		87,
	)
	assertDeclaration(
		t,
		declarations[8],
		DeclarationConstraint,
		"Haft.Constraint.OptionalEvidence",
		88,
		96,
	)
	assertDeclaration(
		t,
		declarations[9],
		DeclarationConstraint,
		"Haft.Constraint.ConcernOrEvidence",
		97,
		105,
	)

	entitySet, ok := declarations[2].(EntitySetDefinitionDeclaration)
	if !ok {
		t.Fatalf("entity-set declaration type = %T", declarations[2])
	}
	if entitySet.EnumerationRule().Value() != "haft.rule.project-entities/v1" {
		t.Fatalf("enumeration rule = %q", entitySet.EnumerationRule().Value())
	}
	if entitySet.CandidatePolicy().Kind() != EntitySetPersistedOnly {
		t.Fatalf("candidate policy = %q", entitySet.CandidatePolicy().Kind())
	}

	kindSignature, ok := declarations[3].(KindSignatureDefinitionDeclaration)
	if !ok {
		t.Fatalf("kind-signature declaration type = %T", declarations[3])
	}
	if kindSignature.Formality() != SignatureF3 {
		t.Fatalf("kind signature formality = %q", kindSignature.Formality().String())
	}
	if len(kindSignature.Assumptions()) != 0 {
		t.Fatalf("kind signature assumptions = %d", len(kindSignature.Assumptions()))
	}
	membershipBasis, ok := kindSignature.MembershipBasis().(CarrierFirstMembershipBasis)
	if !ok {
		t.Fatalf("membership basis = %T, want CarrierFirstMembershipBasis", kindSignature.MembershipBasis())
	}
	if membershipBasis.Kind() != KindSignatureCarrierFirst ||
		membershipBasis.KindSource().Value() != "carrier_first" ||
		membershipBasis.AdapterRule().Value() != "haft.member-of.project-record-carrier/v1" {
		t.Fatalf("carrier-first membership basis = %#v", membershipBasis)
	}
	assertRange(t, "carrier-first membership basis", membershipBasis.Span(), 52, 52)
	assertRange(t, "carrier-first adapter rule", membershipBasis.AdapterRule().Span(), 52, 52)

	relation, ok := declarations[4].(RelationSignatureDeclaration)
	if !ok {
		t.Fatalf("relation declaration type = %T", declarations[4])
	}
	slots := relation.Slots()
	if len(slots) != 2 {
		t.Fatalf("relation slots = %d", len(slots))
	}
	assertRange(t, "ConcernSlot", slots[0].Span(), 57, 61)
	if slots[0].ReferenceMode().Kind() != ReferenceByKind {
		t.Fatalf("ConcernSlot ref mode = %q", slots[0].ReferenceMode().Kind())
	}
	byReference, ok := slots[0].ReferenceMode().(RefKindReferenceMode)
	if !ok || byReference.RefKind().Value() != "Haft.ProjectConcernRef" {
		t.Fatalf("ConcernSlot reference = %#v", slots[0].ReferenceMode())
	}
	assertRange(t, "EvidenceSlot", slots[1].Span(), 62, 65)
	if slots[1].ReferenceMode().Kind() != ReferenceByValue {
		t.Fatalf("EvidenceSlot ref mode = %q", slots[1].ReferenceMode().Kind())
	}
	codec, ok := declarations[6].(CodecBindingDeclaration)
	if !ok {
		t.Fatalf("codec declaration type = %T", declarations[6])
	}
	if len(codec.Contract()) != 2 {
		t.Fatalf("codec conceptual contract entries = %d", len(codec.Contract()))
	}
	cardinality, ok := declarations[7].(ConstraintDeclaration)
	if !ok {
		t.Fatalf("cardinality declaration type = %T", declarations[7])
	}
	cardinalityRule, ok := cardinality.Rule().(SlotCardinalityConstraint)
	if !ok {
		t.Fatalf("cardinality rule type = %T", cardinality.Rule())
	}
	if cardinalityRule.Slot().Value() != "ConcernSlot" {
		t.Fatalf("cardinality slot = %q", cardinalityRule.Slot().Value())
	}

	copyDeclarations := block.Vocabulary().Declarations()
	copyDeclarations[0] = declarations[1]
	if parsed.Carrier().Signature().Vocabulary().Declarations()[0].Kind() != DeclarationValueKind {
		t.Fatal("ParsedCarrier leaked its declaration slice")
	}
}

func TestKindSignatureMembershipBasisIsAClosedRequiredSum(t *testing.T) {
	valid := string(readGoldenCarrier(t))
	carrierFirstLine := "        membership_basis: {kind: carrier_first, adapter_rule: haft.member-of.project-record-carrier/v1}\n"
	directLine := "        membership_basis: {kind: direct_observable_inputs}\n"
	directSource := strings.Replace(valid, carrierFirstLine, directLine, 1)
	if directSource == valid {
		t.Fatal("direct-observable fixture replacement did not run")
	}
	parsed, err := Parse([]byte(directSource))
	if err != nil {
		t.Fatalf("Parse(direct_observable_inputs) error = %v", err)
	}
	declarations := parsed.Carrier().Signature().Vocabulary().Declarations()
	kindSignature := declarations[3].(KindSignatureDefinitionDeclaration)
	basis, ok := kindSignature.MembershipBasis().(DirectObservableInputsMembershipBasis)
	if !ok {
		t.Fatalf("direct membership basis = %T", kindSignature.MembershipBasis())
	}
	if basis.Kind() != KindSignatureDirectObservableInputs ||
		basis.KindSource().Value() != "direct_observable_inputs" {
		t.Fatalf("direct membership basis = %#v", basis)
	}
	assertRange(t, "direct membership basis", basis.Span(), 52, 52)

	tests := []struct {
		name        string
		replacement string
		message     string
	}{
		{
			name:        "basis is absent",
			replacement: "",
			message:     "missing required field \"membership_basis\"",
		},
		{
			name:        "basis kind is unknown",
			replacement: "        membership_basis: {kind: inferred_from_evaluator}\n",
			message:     "want direct_observable_inputs or carrier_first",
		},
		{
			name:        "direct basis smuggles an adapter",
			replacement: "        membership_basis: {kind: direct_observable_inputs, adapter_rule: haft.member-of.project-record-carrier/v1}\n",
			message:     "unknown field \"adapter_rule\"",
		},
		{
			name:        "carrier-first basis omits its adapter",
			replacement: "        membership_basis: {kind: carrier_first}\n",
			message:     "missing required field \"adapter_rule\"",
		},
		{
			name:        "basis is not an algebraic mapping",
			replacement: "        membership_basis: carrier_first\n",
			message:     "must be a mapping",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := strings.Replace(valid, carrierFirstLine, test.replacement, 1)
			if source == valid {
				t.Fatal("membership-basis fixture replacement did not run")
			}
			_, parseErr := Parse([]byte(source))
			if parseErr == nil || !strings.Contains(parseErr.Error(), test.message) {
				t.Fatalf("Parse() error = %v, want substring %q", parseErr, test.message)
			}
		})
	}
}

func TestParseRejectsUnsafeOrStructurallyInvalidYAML(t *testing.T) {
	valid := string(readGoldenCarrier(t))
	cases := []struct {
		name    string
		source  []byte
		message string
	}{
		{
			name:    "unknown field",
			source:  []byte("unexpected: value\n" + valid),
			message: "unknown field",
		},
		{
			name: "duplicate field",
			source: []byte(strings.Replace(
				valid,
				"schema_version: haft.local-practice/v1",
				"schema_version: haft.local-practice/v1\nschema_version: haft.local-practice/v1",
				1,
			)),
			message: "duplicate field",
		},
		{
			name: "duplicate nested field",
			source: []byte(strings.Replace(
				valid,
				"    subject_kind: Haft.ProjectConcern",
				"    subject_kind: Haft.ProjectConcern\n    subject_kind: Haft.ProjectConcern",
				1,
			)),
			message: "duplicate field \"subject_kind\"",
		},
		{
			name:    "missing field",
			source:  []byte(strings.Replace(valid, "compiler_version: haft.local-practice.compiler/v1\n", "", 1)),
			message: "missing required field \"compiler_version\"",
		},
		{
			name: "missing conceptual row",
			source: []byte(strings.Replace(
				valid,
				"  laws:\n    constraint_refs:\n      - Haft.Constraint.RequiredConcern\n      - Haft.Constraint.OptionalEvidence\n      - Haft.Constraint.ConcernOrEvidence\n    invariants:\n      - Schema declarations remain outside MemoryChangeSet.\n",
				"",
				1,
			)),
			message: "missing required field \"laws\"",
		},
		{
			name:    "trailing document",
			source:  []byte(valid + "---\nextra: document\n"),
			message: "exactly one YAML document",
		},
		{
			name: "alias",
			source: []byte(strings.NewReplacer(
				"bounded_context_ref: haft-project",
				"bounded_context_ref: &ctx haft-project",
				"    bounded_context_ref: haft-project",
				"    bounded_context_ref: *ctx",
			).Replace(valid)),
			message: "aliases and anchors",
		},
		{
			name: "explicit tag",
			source: []byte(strings.Replace(
				valid,
				"schema_version: haft.local-practice/v1",
				"schema_version: !!str haft.local-practice/v1",
				1,
			)),
			message: "explicit tags",
		},
		{
			name: "folded block scalar cannot bypass one-line semantics",
			source: []byte(strings.Replace(
				valid,
				"compiler_version: haft.local-practice.compiler/v1",
				"compiler_version: >-\n  haft.local-practice.compiler/v1",
				1,
			)),
			message: "block scalar styles",
		},
		{
			name: "literal multi-line scalar cannot forge a wider source span",
			source: []byte(strings.Replace(
				valid,
				"      - Schema declarations remain outside MemoryChangeSet.",
				"      - |-\n        Schema declarations remain outside\n        MemoryChangeSet.",
				1,
			)),
			message: "block scalar styles",
		},
		{
			name: "double-quoted multi-line scalar cannot forge a shorter source span",
			source: []byte(strings.Replace(
				valid,
				"compiler_version: haft.local-practice.compiler/v1",
				"compiler_version: \"haft.local-practice.\n  compiler/v1\"",
				1,
			)),
			message: "one physical line",
		},
		{
			name: "plain multi-line scalar cannot forge a shorter source span",
			source: []byte(strings.Replace(
				valid,
				"compiler_version: haft.local-practice.compiler/v1",
				"compiler_version: haft.local-practice.\n  compiler/v1",
				1,
			)),
			message: "one physical line",
		},
		{
			name: "escaped NUL semantic value",
			source: []byte(strings.Replace(
				valid,
				"compiler_version: haft.local-practice.compiler/v1",
				`compiler_version: "haft\0compiler"`,
				1,
			)),
			message: "control characters",
		},
		{
			name: "escaped control character in key",
			source: []byte(strings.Replace(
				valid,
				"compiler_version: haft.local-practice.compiler/v1",
				`"compiler\u007fversion": haft.local-practice.compiler/v1`,
				1,
			)),
			message: "control character in a field name",
		},
		{
			name:    "null",
			source:  []byte(strings.Replace(valid, "compiler_version: haft.local-practice.compiler/v1", "compiler_version: null", 1)),
			message: "null values",
		},
		{
			name:    "bounded context mismatch",
			source:  []byte(strings.Replace(valid, "    bounded_context_ref: haft-project", "    bounded_context_ref: other-project", 1)),
			message: "does not match carrier bounded_context_ref",
		},
		{
			name:    "noncanonical base ref",
			source:  []byte(strings.Replace(valid, strings.Repeat("a", 64), strings.Repeat("A", 64), 1)),
			message: "lowercase SHA-256",
		},
	}

	invalidUTF8 := append([]byte(nil), readGoldenCarrier(t)...)
	invalidUTF8 = append(invalidUTF8, 0xff)
	cases = append(cases, struct {
		name    string
		source  []byte
		message string
	}{
		name:    "invalid UTF-8 cannot normalize to replacement text",
		source:  invalidUTF8,
		message: "valid UTF-8",
	})

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, err := Parse(test.source)
			if err == nil {
				t.Fatal("Parse() accepted invalid carrier")
			}
			if !strings.Contains(err.Error(), test.message) {
				t.Fatalf("Parse() error = %q, want substring %q", err, test.message)
			}
		})
	}
}

func TestDigestCommitsToExactCarrierBytesRatherThanOnlyTheAST(t *testing.T) {
	source := readGoldenCarrier(t)
	first, err := Parse(source)
	if err != nil {
		t.Fatalf("Parse(first) error = %v", err)
	}
	withComment := append([]byte("# same declaration, different carrier bytes\n"), source...)
	second, err := Parse(withComment)
	if err != nil {
		t.Fatalf("Parse(second) error = %v", err)
	}
	if first.Digest() == second.Digest() {
		t.Fatal("exact source-byte change did not change carrier digest")
	}
	if first.Carrier().Identity().ID().Value() != second.Carrier().Identity().ID().Value() {
		t.Fatal("comment-only mutation unexpectedly changed the parsed carrier identity")
	}
}

func TestParseAcceptsSingleLineQuotedScalars(t *testing.T) {
	source := string(readGoldenCarrier(t))
	source = strings.Replace(
		source,
		"compiler_version: haft.local-practice.compiler/v1",
		`compiler_version: "haft.local-practice.compiler/v1"`,
		1,
	)
	source = strings.Replace(
		source,
		"      - Schema declarations remain outside MemoryChangeSet.",
		"      - 'Schema declarations aren''t runtime implementations.'",
		1,
	)
	parsed, err := Parse([]byte(source))
	if err != nil {
		t.Fatalf("Parse() rejected single-line quoted scalars: %v", err)
	}
	if parsed.Carrier().CompilerVersion().Value() != "haft.local-practice.compiler/v1" {
		t.Fatalf("compiler version = %q", parsed.Carrier().CompilerVersion().Value())
	}
	assertRange(t, "quoted compiler version", parsed.Carrier().CompilerVersion().Span(), 7, 7)
	invariants := parsed.Carrier().Signature().Laws().Invariants()
	if invariants[0].Value() != "Schema declarations aren't runtime implementations." {
		t.Fatalf("invariant = %q", invariants[0].Value())
	}
}

func TestParseRejectsInvalidSlotSpecs(t *testing.T) {
	valid := string(readGoldenCarrier(t))
	cases := []struct {
		name    string
		source  string
		message string
	}{
		{
			name:    "slot kind lacks Slot suffix",
			source:  strings.Replace(valid, "slot_kind: ConcernSlot", "slot_kind: ConcernPosition", 1),
			message: "must end with Slot",
		},
		{
			name:    "value kind uses Ref suffix",
			source:  strings.Replace(valid, "value_kind: Haft.ProjectConcern\n            ref_mode", "value_kind: Haft.ProjectConcernRef\n            ref_mode", 1),
			message: "ValueKind suffix discipline",
		},
		{
			name:    "reference mode lacks RefKind",
			source:  strings.Replace(valid, "              ref_kind: Haft.ProjectConcernRef\n", "", 1),
			message: "missing required field \"ref_kind\"",
		},
		{
			name: "ByValue smuggles RefKind",
			source: strings.Replace(
				valid,
				"              kind: by_value\n",
				"              kind: by_value\n              ref_kind: Haft.ProjectConcernRef\n",
				1,
			),
			message: "unknown field \"ref_kind\"",
		},
		{
			name:    "RefKind lacks Ref suffix",
			source:  strings.Replace(valid, "ref_kind: Haft.ProjectConcernRef", "ref_kind: Haft.ProjectConcernPointer", 1),
			message: "must end with Ref",
		},
		{
			name: "SlotSpec cannot smuggle cardinality",
			source: strings.Replace(
				valid,
				"              ref_kind: Haft.ProjectConcernRef\n",
				"              ref_kind: Haft.ProjectConcernRef\n            cardinality:\n              minimum: 1\n              maximum: 1\n",
				1,
			),
			message: "unknown field \"cardinality\"",
		},
		{
			name: "duplicate SlotKind",
			source: strings.Replace(
				valid,
				"          - slot_kind: EvidenceSlot",
				"          - slot_kind: ConcernSlot",
				1,
			),
			message: "duplicate SlotKind",
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, err := Parse([]byte(test.source))
			if err == nil {
				t.Fatal("Parse() accepted invalid SlotSpec")
			}
			if !strings.Contains(err.Error(), test.message) {
				t.Fatalf("Parse() error = %q, want substring %q", err, test.message)
			}
		})
	}
}

func TestParseRejectsInvalidSlotCardinalityConstraints(t *testing.T) {
	valid := string(readGoldenCarrier(t))
	cases := []struct {
		name    string
		source  string
		message string
	}{
		{
			name:    "maximum precedes minimum",
			source:  strings.Replace(valid, "minimum: 1\n            maximum: 1", "minimum: 2\n            maximum: 1", 1),
			message: "precedes minimum",
		},
		{
			name:    "constraint slot lacks Slot suffix",
			source:  strings.Replace(valid, "          slot: ConcernSlot", "          slot: ConcernPosition", 1),
			message: "must end with Slot",
		},
		{
			name: "missing cardinality",
			source: strings.Replace(
				valid,
				"          cardinality:\n            minimum: 1\n            maximum: 1\n",
				"",
				1,
			),
			message: "missing required field \"cardinality\"",
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, err := Parse([]byte(test.source))
			if err == nil {
				t.Fatal("Parse() accepted invalid slot-cardinality constraint")
			}
			if !strings.Contains(err.Error(), test.message) {
				t.Fatalf("Parse() error = %q, want substring %q", err, test.message)
			}
		})
	}
}

func TestClosedUnionsAndDuplicateSetsFailClosed(t *testing.T) {
	valid := string(readGoldenCarrier(t))
	duplicateAssumptions := "        assumptions:\n" +
		"          - carrier_ref: carrier:domain-model\n" +
		"            edition: 2.0.0\n" +
		"            digest: sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\n" +
		"          - carrier_ref: carrier:domain-model\n" +
		"            edition: 2.0.0\n" +
		"            digest: sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	cases := []struct {
		name    string
		source  string
		message string
	}{
		{
			name:    "unknown declaration variant",
			source:  strings.Replace(valid, "      - kind: value_kind", "      - kind: executable_handler", 1),
			message: "kind \"executable_handler\" is not supported",
		},
		{
			name:    "unknown entity-set policy variant",
			source:  strings.Replace(valid, "kind: persisted_entities_only", "kind: latest_entities", 1),
			message: "kind \"latest_entities\" is not supported",
		},
		{
			name:    "unknown reference-mode variant",
			source:  strings.Replace(valid, "kind: reference", "kind: pointer", 1),
			message: "kind \"pointer\" is not supported",
		},
		{
			name:    "unknown value-shape variant",
			source:  strings.Replace(valid, "kind: scalar", "kind: arbitrary_json", 1),
			message: "kind \"arbitrary_json\" is not supported",
		},
		{
			name:    "unknown constraint variant",
			source:  strings.Replace(valid, "kind: slot_cardinality", "kind: runtime_guard", 1),
			message: "kind \"runtime_guard\" is not supported",
		},
		{
			name:    "duplicate declaration symbol",
			source:  strings.Replace(valid, "symbol: Haft.Shape.Text", "symbol: Haft.ProjectConcern", 1),
			message: "duplicate symbol \"Haft.ProjectConcern\"",
		},
		{
			name: "duplicate shape member",
			source: strings.Replace(
				valid,
				"          kind: scalar\n          scalar_kind: text",
				"          kind: record\n"+
					"          fields:\n"+
					"            - name: title\n"+
					"              shape: Haft.Shape.Text\n"+
					"            - name: title\n"+
					"              shape: Haft.Shape.Text",
				1,
			),
			message: "duplicate member \"title\"",
		},
		{
			name: "duplicate codec contract entry",
			source: strings.Replace(
				valid,
				"          - Decode then encode preserves canonical bytes.",
				"          - Equal conceptual values produce equal canonical bytes.",
				1,
			),
			message: "duplicate value",
		},
		{
			name:    "duplicate kind-signature assumption coordinate",
			source:  strings.Replace(valid, "        assumptions: []", duplicateAssumptions, 1),
			message: "duplicate carrier edition",
		},
		{
			name: "duplicate slot-group operand",
			source: strings.Replace(
				valid,
				"            - EvidenceSlot\n          mode: at_least_one",
				"            - ConcernSlot\n          mode: at_least_one",
				1,
			),
			message: "duplicate value \"ConcernSlot\"",
		},
		{
			name: "runtime codec implementation is outside Vocabulary",
			source: strings.Replace(
				valid,
				"        value_shape: Haft.Shape.Text",
				"        value_shape: Haft.Shape.Text\n        implementation: haft.codec.text/v1",
				1,
			),
			message: "unknown field \"implementation\"",
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, err := Parse([]byte(test.source))
			if err == nil {
				t.Fatal("Parse() accepted an open union or duplicate set member")
			}
			if !strings.Contains(err.Error(), test.message) {
				t.Fatalf("Parse() error = %q, want substring %q", err, test.message)
			}
		})
	}
}

func TestNestedASTCollectionsAreDefensiveCopies(t *testing.T) {
	parsed, err := Parse(readGoldenCarrier(t))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	carrier := parsed.Carrier()
	manifestProvides := carrier.Manifest().Provides()
	manifestProvides[0] = ManifestProvide{}
	if carrier.Manifest().Provides()[0].Symbol().Value() == "" {
		t.Fatal("SignatureManifest leaked its provides slice")
	}

	block := carrier.Signature()
	declarations := block.Vocabulary().Declarations()
	relation := declarations[4].(RelationSignatureDeclaration)
	slots := relation.Slots()
	slots[0] = SlotSpec{}
	if relation.Slots()[0].SlotKind().Value() == "" {
		t.Fatal("RelationSignatureDeclaration leaked its slots slice")
	}
	codec := declarations[6].(CodecBindingDeclaration)
	contract := codec.Contract()
	contract[0] = SourceText{}
	if codec.Contract()[0].Value() == "" {
		t.Fatal("CodecBindingDeclaration leaked its conceptual contract slice")
	}
	slotGroup := declarations[9].(ConstraintDeclaration).Rule().(SlotGroupConstraint)
	groupSlots := slotGroup.Slots()
	groupSlots[0] = SourceText{}
	if slotGroup.Slots()[0].Value() == "" {
		t.Fatal("SlotGroupConstraint leaked its slots slice")
	}
	lawRefs := block.Laws().ConstraintRefs()
	lawRefs[0] = SourceText{}
	if block.Laws().ConstraintRefs()[0].Value() == "" {
		t.Fatal("Laws leaked its constraint-ref slice")
	}
	assumptions := block.Applicability().Assumptions()
	assumptions[0] = SourceText{}
	if block.Applicability().Assumptions()[0].Value() == "" {
		t.Fatal("Applicability leaked its assumptions slice")
	}

	withPin := strings.Replace(
		string(readGoldenCarrier(t)),
		"        assumptions: []",
		"        assumptions:\n"+
			"          - carrier_ref: carrier:domain-model\n"+
			"            edition: 2.0.0\n"+
			"            digest: sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		1,
	)
	parsedWithPin, err := Parse([]byte(withPin))
	if err != nil {
		t.Fatalf("Parse(with pin) error = %v", err)
	}
	kindSignature := parsedWithPin.Carrier().Signature().Vocabulary().Declarations()[3].(KindSignatureDefinitionDeclaration)
	pins := kindSignature.Assumptions()
	pins[0] = KindSignatureAssumption{}
	if kindSignature.Assumptions()[0].CarrierRef().Value() == "" {
		t.Fatal("KindSignatureDefinitionDeclaration leaked its assumptions slice")
	}

	recordSource := strings.Replace(
		string(readGoldenCarrier(t)),
		"          kind: scalar\n          scalar_kind: text",
		"          kind: record\n"+
			"          fields:\n"+
			"            - name: title\n"+
			"              shape: Haft.Shape.Text",
		1,
	)
	parsedRecord, err := Parse([]byte(recordSource))
	if err != nil {
		t.Fatalf("Parse(record) error = %v", err)
	}
	shapeDeclaration := parsedRecord.Carrier().Signature().Vocabulary().Declarations()[5].(ValueShapeDeclaration)
	record := shapeDeclaration.Shape().(RecordValueShape)
	fields := record.Fields()
	fields[0] = ShapeMember{}
	if record.Fields()[0].Name().Value() == "" {
		t.Fatal("RecordValueShape leaked its fields slice")
	}
}

func TestAssumptionCoordinatesCannotCollideByDelimiterPlacement(t *testing.T) {
	left := kindSignatureAssumptionCoordinate{
		carrierRef: "carrier:a",
		edition:    "b\x00c",
	}
	right := kindSignatureAssumptionCoordinate{
		carrierRef: "carrier:a\x00b",
		edition:    "c",
	}
	if left == right {
		t.Fatal("structural assumption coordinates unexpectedly compare equal")
	}
	coordinates := map[kindSignatureAssumptionCoordinate]struct{}{
		left:  {},
		right: {},
	}
	if len(coordinates) != 2 {
		t.Fatal("structural assumption coordinate map collapsed delimiter placement")
	}
}

func TestVersionPolicyAndCarrierManifestCoherenceRemainLinkerOwned(t *testing.T) {
	source := string(readGoldenCarrier(t))
	source = strings.Replace(source, "  id: haft.typed-memory", "  id: carrier.identity", 1)
	source = strings.Replace(source, "  edition: 1.0.0", "  edition: edition-alpha", 1)
	source = strings.Replace(source, "  id: haft.typed-memory", "  id: distinct.signature", 1)
	source = strings.Replace(source, "  version: 1.0.0", "  version: release-label", 1)
	parsed, err := Parse([]byte(source))
	if err != nil {
		t.Fatalf("Parse() imposed linker-owned identity or SemVer policy: %v", err)
	}
	carrier := parsed.Carrier()
	if carrier.Identity().ID().Value() == carrier.Manifest().ID().Value() {
		t.Fatal("test fixture failed to preserve distinct carrier and manifest identities")
	}
	if carrier.Identity().Edition().Value() != "edition-alpha" {
		t.Fatalf("carrier edition = %q", carrier.Identity().Edition().Value())
	}
	if carrier.Manifest().Version().Value() != "release-label" {
		t.Fatalf("manifest version = %q", carrier.Manifest().Version().Value())
	}
	codecType := reflect.TypeOf(CodecBindingDeclaration{})
	if _, exists := codecType.MethodByName("Implementation"); exists {
		t.Fatal("conceptual CodecBindingDeclaration exposes runtime implementation selection")
	}
	slotType := reflect.TypeOf(SlotSpec{})
	if _, exists := slotType.MethodByName("Cardinality"); exists {
		t.Fatal("A.6.5 SlotSpec exposes non-triple cardinality state")
	}
}

func TestParsePreservesSyntheticManifestImportsWithoutInferringBaseDependency(t *testing.T) {
	valid := string(readGoldenCarrier(t))
	source := strings.Replace(
		valid,
		"  imports: []",
		"  imports:\n    - example.signature",
		1,
	)
	if source == valid {
		t.Fatal("test replacement did not add a synthetic manifest import")
	}
	parsed, err := Parse([]byte(source))
	if err != nil {
		t.Fatalf("Parse() rejected synthetic manifest import: %v", err)
	}
	imports := parsed.Carrier().Manifest().Imports()
	if len(imports) != 1 || imports[0].SignatureID().Value() != "example.signature" {
		t.Fatalf("synthetic manifest imports = %#v", imports)
	}
}

func TestParseClosedShapeConstraintAndCandidatePolicyVariants(t *testing.T) {
	valid := string(readGoldenCarrier(t))
	tests := []struct {
		name        string
		old         string
		replacement string
	}{
		{
			name: "record shape",
			old:  "          kind: scalar\n          scalar_kind: text",
			replacement: "          kind: record\n" +
				"          fields:\n" +
				"            - name: title\n" +
				"              shape: Haft.Shape.Text",
		},
		{
			name: "sum shape",
			old:  "          kind: scalar\n          scalar_kind: text",
			replacement: "          kind: sum\n" +
				"          variants:\n" +
				"            - name: known\n" +
				"              shape: Haft.Shape.Text",
		},
		{
			name: "ordered sequence shape",
			old:  "          kind: scalar\n          scalar_kind: text",
			replacement: "          kind: ordered_sequence\n" +
				"          element_shape: Haft.Shape.Text",
		},
		{
			name: "unordered set shape",
			old:  "          kind: scalar\n          scalar_kind: text",
			replacement: "          kind: unordered_set\n" +
				"          element_shape: Haft.Shape.Text",
		},
		{
			name:        "claim graph shape",
			old:         "          kind: scalar\n          scalar_kind: text",
			replacement: "          kind: claim_graph",
		},
		{
			name: "kind disjoint constraint",
			old: "          kind: slot_group\n" +
				"          relation: Haft.ConcernMemory\n" +
				"          slots:\n" +
				"            - ConcernSlot\n" +
				"            - EvidenceSlot\n" +
				"          mode: at_least_one",
			replacement: "          kind: kind_disjoint\n" +
				"          kinds:\n" +
				"            - Haft.ProjectConcern\n" +
				"            - U.Entity",
		},
		{
			name: "prior-batch entity-set policy",
			old:  "          kind: persisted_entities_only",
			replacement: "          kind: prior_batch_declarations_visible\n" +
				"          evaluation_rule: haft.rule.project-entities-prior-batch/v1",
		},
		{
			name: "exact kind-signature assumption pin",
			old:  "        assumptions: []",
			replacement: "        assumptions:\n" +
				"          - carrier_ref: carrier:domain-model\n" +
				"            edition: 2.0.0\n" +
				"            digest: sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := strings.Replace(valid, test.old, test.replacement, 1)
			if source == valid {
				t.Fatal("test replacement did not change fixture")
			}
			if _, err := Parse([]byte(source)); err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
		})
	}
}

func TestSourceLineRangeRejectsInvalidCoordinates(t *testing.T) {
	if _, err := newSourceLineRange(0, 1); err == nil {
		t.Fatal("newSourceLineRange accepted zero start")
	}
	if _, err := newSourceLineRange(3, 2); err == nil {
		t.Fatal("newSourceLineRange accepted reversed range")
	}
	lineRange, err := newSourceLineRange(2, 2)
	if err != nil {
		t.Fatalf("newSourceLineRange() error = %v", err)
	}
	assertRange(t, "valid singleton", lineRange, 2, 2)
}

func readGoldenCarrier(t *testing.T) []byte {
	t.Helper()
	source, err := os.ReadFile("testdata/valid.yaml")
	if err != nil {
		t.Fatalf("read golden carrier: %v", err)
	}
	return source
}

func assertDeclaration(
	t *testing.T,
	declaration Declaration,
	kind DeclarationKind,
	symbol string,
	start uint64,
	end uint64,
) {
	t.Helper()
	if declaration.Kind() != kind {
		t.Fatalf("declaration kind = %q, want %q", declaration.Kind(), kind)
	}
	if declaration.Symbol().Value() != symbol {
		t.Fatalf("declaration symbol = %q, want %q", declaration.Symbol().Value(), symbol)
	}
	assertRange(t, symbol, declaration.Span(), start, end)
}

func assertRange(t *testing.T, label string, lineRange SourceLineRange, start, end uint64) {
	t.Helper()
	if lineRange.Start() != start || lineRange.End() != end {
		t.Fatalf(
			"%s range = %d..%d, want %d..%d",
			label,
			lineRange.Start(),
			lineRange.End(),
			start,
			end,
		)
	}
}
