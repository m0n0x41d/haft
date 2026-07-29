package projecttypeenv

import (
	"bytes"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf/localpractice"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
)

const carrierFirstMembershipBasisFixture = "        membership_basis: {kind: carrier_first, adapter_rule: haft.member-of.project-record-carrier/v1}\n"

const directObservableMembershipBasisFixture = "        membership_basis: {kind: direct_observable_inputs}\n"

func TestKindSignatureCarrierFirstMembershipBasisCompilesToExactSymbolicDependency(
	t *testing.T,
) {
	t.Parallel()

	base := loadBaseArtifact(t)
	artifact := sealMembershipBasisFixture(t, base, carrierFirstMembershipBasisFixture)
	ir := artifact.IR()
	declaration := declarationByKind(t, &ir, localpractice.DeclarationKindSignature)
	assertMembershipBasisFact(t, declaration, "membership_basis.kind", "carrier_first", 52)
	assertMembershipBasisFact(
		t,
		declaration,
		"membership_basis.adapter_rule",
		"haft.member-of.project-record-carrier/v1",
		52,
	)
	if !hasSymbolicDependency(
		[]SymbolicDeclaration{*declaration},
		declaration.Symbol().Value(),
		"membership_basis.adapter_rule",
		"haft.member-of.project-record-carrier/v1",
	) {
		t.Fatal("carrier-first membership adapter is not an exact symbolic dependency")
	}
	if strings.Contains(string(artifact.CanonicalBytes()), "ContextKindAvailability") ||
		strings.Contains(string(artifact.CanonicalBytes()), "context_kind_availability") {
		t.Fatal("membership basis authored a derived ContextKindAvailability")
	}
}

func TestKindSignatureDirectObservableBasisHasNoAdapterDependency(t *testing.T) {
	t.Parallel()

	base := loadBaseArtifact(t)
	carrierFirst := sealMembershipBasisFixture(t, base, carrierFirstMembershipBasisFixture)
	direct := sealMembershipBasisFixture(t, base, directObservableMembershipBasisFixture)
	if carrierFirst.Ref() == direct.Ref() ||
		bytes.Equal(carrierFirst.CanonicalBytes(), direct.CanonicalBytes()) {
		t.Fatal("changing membership-basis variant did not change E identity")
	}
	ir := direct.IR()
	declaration := declarationByKind(t, &ir, localpractice.DeclarationKindSignature)
	assertMembershipBasisFact(
		t,
		declaration,
		"membership_basis.kind",
		"direct_observable_inputs",
		52,
	)
	for _, fact := range declaration.Facts() {
		if fact.Path() == "membership_basis.adapter_rule" {
			t.Fatal("direct-observable membership basis contains an adapter fact")
		}
	}
	for _, dependency := range declaration.Dependencies() {
		if dependency.Role() == "membership_basis.adapter_rule" {
			t.Fatal("direct-observable membership basis contains an adapter dependency")
		}
	}
}

func TestKindSignatureMembershipBasisCanonicalDecodeFailsClosed(t *testing.T) {
	t.Parallel()

	base := loadBaseArtifact(t)
	artifact := sealMembershipBasisFixture(t, base, carrierFirstMembershipBasisFixture)
	tests := []struct {
		name    string
		mutate  func(*symbolicDeclarationCanonicalV1)
		message string
	}{
		{
			name: "unknown basis kind",
			mutate: func(declaration *symbolicDeclarationCanonicalV1) {
				setMembershipBasisCanonicalFact(
					declaration,
					"membership_basis.kind",
					"inferred",
				)
			},
			message: "membership basis",
		},
		{
			name: "direct basis retains carrier adapter",
			mutate: func(declaration *symbolicDeclarationCanonicalV1) {
				setMembershipBasisCanonicalFact(
					declaration,
					"membership_basis.kind",
					"direct_observable_inputs",
				)
			},
			message: "outside the kind_signature_definition schema",
		},
		{
			name: "carrier-first adapter absent",
			mutate: func(declaration *symbolicDeclarationCanonicalV1) {
				declaration.Facts = removeMembershipBasisCanonicalFact(
					declaration.Facts,
					"membership_basis.adapter_rule",
				)
				declaration.Dependencies = removeMembershipBasisCanonicalDependency(
					declaration.Dependencies,
					"membership_basis.adapter_rule",
				)
			},
			message: "requires source fact \"membership_basis.adapter_rule\"",
		},
		{
			name: "carrier-first adapter dependency absent",
			mutate: func(declaration *symbolicDeclarationCanonicalV1) {
				declaration.Dependencies = removeMembershipBasisCanonicalDependency(
					declaration.Dependencies,
					"membership_basis.adapter_rule",
				)
			},
			message: "requires dependency role \"membership_basis.adapter_rule\"",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoded := decodeProjectExtensionCanonicalForTest(t, artifact.CanonicalBytes())
			declaration := canonicalDeclarationByKindForTest(
				t,
				&encoded,
				localpractice.DeclarationKindSignature,
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

func sealMembershipBasisFixture(
	t *testing.T,
	base typeenv.BaseTypeEnvArtifact,
	basisLine string,
) ProjectTypeEnvExtensionArtifact {
	t.Helper()
	source := carrierFixture(t, base, "alpha.signature", "1.0.0", "Alpha", nil)
	if basisLine != carrierFirstMembershipBasisFixture {
		replaced := bytes.Replace(
			source,
			[]byte(carrierFirstMembershipBasisFixture),
			[]byte(basisLine),
			1,
		)
		if bytes.Equal(source, replaced) {
			t.Fatal("membership-basis fixture replacement did not run")
		}
		source = replaced
	}
	parsed := parseCarrier(t, source)
	bundle := acceptedManifestBundle(t, base, []localpractice.ParsedCarrier{parsed})
	return compileAndSealExtension(t, bundle.Nodes()[0], nil)
}

func assertMembershipBasisFact(
	t *testing.T,
	declaration *SymbolicDeclaration,
	path string,
	value string,
	line uint64,
) {
	t.Helper()
	for _, fact := range declaration.Facts() {
		if fact.Path() == path &&
			fact.Value().Value() == value &&
			fact.Value().Span().Start() == line &&
			fact.Value().Span().End() == line {
			return
		}
	}
	t.Fatalf("membership-basis fact %q = %q at line %d was not found", path, value, line)
}

func setMembershipBasisCanonicalFact(
	declaration *symbolicDeclarationCanonicalV1,
	path string,
	value string,
) {
	for index := range declaration.Facts {
		fact := &declaration.Facts[index]
		if fact.Path == path {
			fact.Value.Value = value
			return
		}
	}
}

func removeMembershipBasisCanonicalFact(
	facts []sourceFactCanonicalV1,
	path string,
) []sourceFactCanonicalV1 {
	result := make([]sourceFactCanonicalV1, 0, len(facts))
	for _, fact := range facts {
		if fact.Path != path {
			result = append(result, fact)
		}
	}
	return result
}

func removeMembershipBasisCanonicalDependency(
	dependencies []symbolicDependencyCanonicalV1,
	role string,
) []symbolicDependencyCanonicalV1 {
	result := make([]symbolicDependencyCanonicalV1, 0, len(dependencies))
	for _, dependency := range dependencies {
		if dependency.Role != role {
			result = append(result, dependency)
		}
	}
	return result
}
