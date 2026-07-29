package projecttypeenvstore

import (
	"fmt"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf/localpractice"
	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/runtimemechanism"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	testSourceRevision = "44dd88188a07646ef23aca32627a3f670525853f"
	testCompilerSchema = "fpf-typeenv.v1"
)

type artifactClosureFixture struct {
	base      typeenv.BaseTypeEnvArtifact
	extension projecttypeenv.ProjectTypeEnvExtensionArtifact
	runtime   projecttypeenv.RuntimeEvaluationBasisArtifact
	composite projecttypeenv.ProjectTypeEnvCompositeArtifact
	closure   ArtifactClosure
}

type nonEmptyRuntimeFixture struct {
	basis     projecttypeenv.RuntimeEvaluationBasisArtifact
	mechanism runtimemechanism.RuntimeMechanismArtifactV1
}

func newNonEmptyRuntimeFixture(t *testing.T) nonEmptyRuntimeFixture {
	t.Helper()
	rule, err := typedmemory.NewRuleRef("haft.rule.store-fixture-member-of/v1")
	if err != nil {
		t.Fatalf("NewRuleRef(): %v", err)
	}
	entry, err := runtimemechanism.NewMemberOfEntry(rule)
	if err != nil {
		t.Fatalf("NewMemberOfEntry(): %v", err)
	}
	artifactRef, err := typedmemory.NewCarrierRef("artifact:store-fixture-member-of")
	if err != nil {
		t.Fatalf("NewCarrierRef(): %v", err)
	}
	edition, err := typedmemory.NewCarrierEdition("1.0.0")
	if err != nil {
		t.Fatalf("NewCarrierEdition(): %v", err)
	}
	mechanism, err := runtimemechanism.SealRuntimeMechanismArtifactV1(
		artifactRef,
		edition,
		[]runtimemechanism.RuntimeMechanismEntryV1{entry},
	)
	if err != nil {
		t.Fatalf("SealRuntimeMechanismArtifactV1(): %v", err)
	}
	mechanismPin, err := projecttypeenv.NewRuntimeMechanismArtifactPinFromArtifact(mechanism)
	if err != nil {
		t.Fatalf("NewRuntimeMechanismArtifactPinFromArtifact(): %v", err)
	}
	pin, err := projecttypeenv.NewEvaluatorRuntimeMechanismPin(
		projecttypeenv.EvaluatorRuntimeMechanismPinInput{
			Rule:             rule,
			Contract:         projecttypeenv.RuntimeMechanismContractMemberOf,
			Mechanism:        mechanismPin,
			ResolvedArtifact: &mechanism,
		},
	)
	if err != nil {
		t.Fatalf("NewEvaluatorRuntimeMechanismPin(): %v", err)
	}
	basis, err := projecttypeenv.SealRuntimeEvaluationBasis(
		[]projecttypeenv.RuntimeEvaluationMechanismPin{pin},
		mechanism,
	)
	if err != nil {
		t.Fatalf("SealRuntimeEvaluationBasis(non-empty): %v", err)
	}
	if err := basis.VerifyResolvedClosure(); err != nil {
		t.Fatalf("non-empty basis VerifyResolvedClosure(): %v", err)
	}
	return nonEmptyRuntimeFixture{basis: basis, mechanism: mechanism}
}

func newArtifactClosureFixture(t *testing.T) artifactClosureFixture {
	t.Helper()
	base := compiledBaseFixture(t)
	extension := extensionFixture(t, base)
	runtime, err := projecttypeenv.SealRuntimeEvaluationBasis(nil)
	if err != nil {
		t.Fatalf("SealRuntimeEvaluationBasis(): %v", err)
	}
	linked := acceptedLinkedFixture(
		t,
		projecttypeenv.LinkProjectTypeEnvCompositeIR(
			base,
			[]projecttypeenv.ProjectTypeEnvExtensionArtifact{extension},
		),
	)
	composite, err := projecttypeenv.SealProjectTypeEnvComposite(linked, runtime)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvComposite(): %v", err)
	}
	closure, err := PrepareArtifactClosure(
		base,
		[]projecttypeenv.ProjectTypeEnvExtensionArtifact{extension},
		runtime,
		composite,
	)
	if err != nil {
		t.Fatalf("PrepareArtifactClosure(): %v", err)
	}
	return artifactClosureFixture{
		base:      base,
		extension: extension,
		runtime:   runtime,
		composite: composite,
		closure:   closure,
	}
}

func compiledBaseFixture(t *testing.T) typeenv.BaseTypeEnvArtifact {
	t.Helper()
	revision := testSourceRevisionFixture(t)
	compiler := testCompilerSchemaFixture(t)
	contextDeclaration := testContextDeclaration(t, revision)
	entityDeclaration := testValueKindDeclaration(t, revision, "U.Entity", 10)
	declarations := []typeenv.LinkedDeclaration{
		contextDeclaration,
		entityDeclaration,
	}
	coverage := testCompiledCoverage(t, declarations)
	ir, err := typeenv.NewCompiledLinkedTypeEnvIR(
		revision,
		compiler,
		coverage,
		declarations,
	)
	if err != nil {
		t.Fatalf("NewCompiledLinkedTypeEnvIR(): %v", err)
	}
	artifact, err := typeenv.SealBaseTypeEnv(ir)
	if err != nil {
		t.Fatalf("SealBaseTypeEnv(): %v", err)
	}
	return artifact
}

func coverageOnlyBaseFixture(t *testing.T) typeenv.BaseTypeEnvArtifact {
	t.Helper()
	revision := testSourceRevisionFixture(t)
	compiler := testCompilerSchemaFixture(t)
	location := testSourceLocation(t, revision, "fixture:coverage-only", 1, 2)
	subject, err := typedmemory.SourceUnitCoverage(location.UnitID())
	if err != nil {
		t.Fatalf("SourceUnitCoverage(): %v", err)
	}
	entry, err := typedmemory.NewSourceOnlyCoverageEntry(
		subject,
		location,
		"fixture_has_no_closed_declaration",
	)
	if err != nil {
		t.Fatalf("NewSourceOnlyCoverageEntry(): %v", err)
	}
	coverage, err := typedmemory.NewCoverageManifest([]typedmemory.CoverageEntry{entry})
	if err != nil {
		t.Fatalf("NewCoverageManifest(): %v", err)
	}
	ir, err := typeenv.NewCoverageOnlyLinkedTypeEnvIR(
		revision,
		compiler,
		coverage,
		"fixture contains no executable declaration",
	)
	if err != nil {
		t.Fatalf("NewCoverageOnlyLinkedTypeEnvIR(): %v", err)
	}
	artifact, err := typeenv.SealBaseTypeEnv(ir)
	if err != nil {
		t.Fatalf("SealBaseTypeEnv(coverage-only): %v", err)
	}
	return artifact
}

func extensionFixture(
	t *testing.T,
	base typeenv.BaseTypeEnvArtifact,
) projecttypeenv.ProjectTypeEnvExtensionArtifact {
	return extensionFixtureNamed(
		t,
		base,
		"haft.store-fixture",
		"haft-store-fixture",
		"Haft.StoreFixture",
	)
}

func extensionFixtureNamed(
	t *testing.T,
	base typeenv.BaseTypeEnvArtifact,
	carrierID string,
	boundedContext string,
	valueKind string,
) projecttypeenv.ProjectTypeEnvExtensionArtifact {
	t.Helper()
	baseRef, exists := base.TypeEnvRef()
	if !exists {
		t.Fatal("compiled base fixture has no TypeEnvRef")
	}
	source := fmt.Sprintf(`schema_version: haft.local-practice/v1
carrier:
  id: %s
  edition: 1.0.0
base_type_env_ref: %s
bounded_context_ref: %s
compiler_version: haft.local-practice.compiler/v1
signature_manifest:
  id: %s
  version: 1.0.0
  publication_state: candidate
  imports: []
  provides:
    - %s
    - %s
signature:
  subject_block:
    subject_kind: %s
    ranged_value_kind: U.Entity
    slice_set: StoreFixtureSliceSet
    extent_rule: haft.store-fixture.extent/v1
  vocabulary:
    declarations:
      - kind: bounded_context
        symbol: %s
      - kind: value_kind
        symbol: %s
  laws:
    constraint_refs: []
    invariants:
      - Stored fixture values remain distinguishable.
  applicability:
    bounded_context_ref: %s
    assumptions:
      - The fixture is used only for artifact persistence tests.
`,
		carrierID,
		baseRef.String(),
		boundedContext,
		carrierID,
		boundedContext,
		valueKind,
		valueKind,
		boundedContext,
		valueKind,
		boundedContext,
	)
	parsed, err := localpractice.Parse([]byte(source))
	if err != nil {
		t.Fatalf("localpractice.Parse(): %v\n%s", err, source)
	}
	resolution := projecttypeenv.ResolveManifestGraph(
		base,
		[]localpractice.ParsedCarrier{parsed},
	)
	if resolution.Rejected() {
		t.Fatalf("ResolveManifestGraph() rejected: %s", fixtureLinkIssues(resolution.Issues()))
	}
	bundle, exists := resolution.Bundle()
	if !exists || len(bundle.Nodes()) != 1 {
		t.Fatalf("resolved bundle nodes = %d, exists = %v", len(bundle.Nodes()), exists)
	}
	ir, err := projecttypeenv.CompileProjectTypeEnvExtensionIR(
		bundle.Nodes()[0],
		nil,
	)
	if err != nil {
		t.Fatalf("CompileProjectTypeEnvExtensionIR(): %v", err)
	}
	artifact, err := projecttypeenv.SealProjectTypeEnvExtension(ir)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvExtension(): %v", err)
	}
	return artifact
}

func acceptedLinkedFixture(
	t *testing.T,
	resolution projecttypeenv.CompositeIRLinkResolution,
) projecttypeenv.LinkedProjectTypeEnvCompositeIR {
	t.Helper()
	if resolution.Rejected() {
		t.Fatalf("LinkProjectTypeEnvCompositeIR() rejected: %s", fixtureLinkIssues(resolution.Issues()))
	}
	linked, exists := resolution.CompositeIR()
	if !exists {
		t.Fatal("accepted composite link has no IR")
	}
	return linked
}

func fixtureLinkIssues(issues []projecttypeenv.LinkIssue) string {
	values := make([]string, 0, len(issues))
	for _, issue := range issues {
		values = append(values, fmt.Sprintf(
			"%s at %s: %s",
			issue.Code(),
			issue.Location().String(),
			issue.Detail(),
		))
	}
	return strings.Join(values, "; ")
}

func testSourceRevisionFixture(t *testing.T) typedmemory.SourceRevision {
	t.Helper()
	revision, err := typedmemory.NewSourceRevision(testSourceRevision)
	if err != nil {
		t.Fatalf("NewSourceRevision(): %v", err)
	}
	return revision
}

func testCompilerSchemaFixture(t *testing.T) typedmemory.CompilerSchemaVersion {
	t.Helper()
	compiler, err := typedmemory.NewCompilerSchemaVersion(testCompilerSchema)
	if err != nil {
		t.Fatalf("NewCompilerSchemaVersion(): %v", err)
	}
	return compiler
}

func testContextDeclaration(
	t *testing.T,
	revision typedmemory.SourceRevision,
) typeenv.LinkedDeclaration {
	t.Helper()
	contextRef, err := typedmemory.NewBoundedContextRef("fpf:publication")
	if err != nil {
		t.Fatalf("NewBoundedContextRef(): %v", err)
	}
	symbol, err := typedmemory.BoundedContextSymbolRef(contextRef)
	if err != nil {
		t.Fatalf("BoundedContextSymbolRef(): %v", err)
	}
	contextField, err := typeenv.NewDeclarationField(
		"context_ref",
		typeenv.NewTextValue(contextRef.String()),
	)
	if err != nil {
		t.Fatalf("NewDeclarationField(context_ref): %v", err)
	}
	revisionField, err := typeenv.NewDeclarationField(
		"source_revision",
		typeenv.NewTextValue(revision.String()),
	)
	if err != nil {
		t.Fatalf("NewDeclarationField(source_revision): %v", err)
	}
	body, err := typeenv.NewDeclarationBody([]typeenv.DeclarationField{
		contextField,
		revisionField,
	})
	if err != nil {
		t.Fatalf("NewDeclarationBody(context): %v", err)
	}
	rule := testCompilerRule(t, "fpf.publication-context.v1")
	location := testSourceLocation(t, revision, "fixture:publication-context", 1, 1)
	basis := testSourceBasis(t, rule, location, "fixture-context-provenance")
	declaration, err := typeenv.NewLinkedDeclaration(symbol, rule, body, basis)
	if err != nil {
		t.Fatalf("NewLinkedDeclaration(context): %v", err)
	}
	return declaration
}

func testValueKindDeclaration(
	t *testing.T,
	revision typedmemory.SourceRevision,
	kindName string,
	line uint64,
) typeenv.LinkedDeclaration {
	t.Helper()
	kindID, err := typedmemory.NewKindID(kindName)
	if err != nil {
		t.Fatalf("NewKindID(): %v", err)
	}
	symbol, err := typedmemory.KindSymbolRef(kindID)
	if err != nil {
		t.Fatalf("KindSymbolRef(): %v", err)
	}
	kindField, err := typeenv.NewDeclarationField(
		"kind_id",
		typeenv.NewTextValue(kindName),
	)
	if err != nil {
		t.Fatalf("NewDeclarationField(kind_id): %v", err)
	}
	roleField, err := typeenv.NewDeclarationField(
		"semantic_role",
		typeenv.NewTextValue("value_kind"),
	)
	if err != nil {
		t.Fatalf("NewDeclarationField(semantic_role): %v", err)
	}
	body, err := typeenv.NewDeclarationBody([]typeenv.DeclarationField{kindField, roleField})
	if err != nil {
		t.Fatalf("NewDeclarationBody(value kind): %v", err)
	}
	rule := testCompilerRule(t, "fpf.value-kind.declaration.v1")
	location := testSourceLocation(t, revision, "fixture:"+kindName, line, line+1)
	basis := testSourceBasis(t, rule, location, "fixture-kind-provenance")
	declaration, err := typeenv.NewLinkedDeclaration(symbol, rule, body, basis)
	if err != nil {
		t.Fatalf("NewLinkedDeclaration(value kind): %v", err)
	}
	return declaration
}

func testSourceBasis(
	t *testing.T,
	rule typedmemory.CompilerRuleID,
	location typedmemory.SourceLocation,
	provenanceText string,
) typeenv.SourceBasis {
	t.Helper()
	provenanceRef, err := typedmemory.NewProvenanceRef(provenanceText)
	if err != nil {
		t.Fatalf("NewProvenanceRef(): %v", err)
	}
	provenance, err := typedmemory.NewFPFSourceProvenance(
		provenanceRef,
		location,
		rule,
	)
	if err != nil {
		t.Fatalf("NewFPFSourceProvenance(): %v", err)
	}
	basis, err := typeenv.NewSourceDeclarationBasis(provenance)
	if err != nil {
		t.Fatalf("NewSourceDeclarationBasis(): %v", err)
	}
	return basis
}

func testCompiledCoverage(
	t *testing.T,
	declarations []typeenv.LinkedDeclaration,
) typedmemory.CoverageManifest {
	t.Helper()
	entries := make([]typedmemory.CoverageEntry, 0, len(declarations))
	for _, declaration := range declarations {
		subject, err := typedmemory.SchemaSymbolCoverage(declaration.Symbol())
		if err != nil {
			t.Fatalf("SchemaSymbolCoverage(): %v", err)
		}
		locations := declaration.Basis().SourceLocations()
		if len(locations) != 1 {
			t.Fatalf("declaration %s source count = %d", declaration.Symbol(), len(locations))
		}
		entry, err := typedmemory.NewCompiledCoverageEntry(subject, locations[0])
		if err != nil {
			t.Fatalf("NewCompiledCoverageEntry(): %v", err)
		}
		entries = append(entries, entry)
	}
	coverage, err := typedmemory.NewCoverageManifest(entries)
	if err != nil {
		t.Fatalf("NewCoverageManifest(): %v", err)
	}
	return coverage
}

func testSourceLocation(
	t *testing.T,
	revision typedmemory.SourceRevision,
	unitText string,
	start uint64,
	end uint64,
) typedmemory.SourceLocation {
	t.Helper()
	unit, err := typedmemory.NewSourceUnitID(unitText)
	if err != nil {
		t.Fatalf("NewSourceUnitID(): %v", err)
	}
	digest := testSHA256Digest(t, strings.Repeat("b", 64))
	lineRange, err := typedmemory.NewSourceLineRange(start, end)
	if err != nil {
		t.Fatalf("NewSourceLineRange(): %v", err)
	}
	location, err := typedmemory.NewUnpatternedSourceLocation(
		unit,
		revision,
		digest,
		lineRange,
	)
	if err != nil {
		t.Fatalf("NewUnpatternedSourceLocation(): %v", err)
	}
	return location
}

func testCompilerRule(t *testing.T, value string) typedmemory.CompilerRuleID {
	t.Helper()
	rule, err := typedmemory.NewCompilerRuleID(value)
	if err != nil {
		t.Fatalf("NewCompilerRuleID(): %v", err)
	}
	return rule
}

func testSHA256Digest(t *testing.T, hexText string) typedmemory.SHA256Digest {
	t.Helper()
	digest, err := typedmemory.NewSHA256Digest("sha256:" + hexText)
	if err != nil {
		t.Fatalf("NewSHA256Digest(): %v", err)
	}
	return digest
}
