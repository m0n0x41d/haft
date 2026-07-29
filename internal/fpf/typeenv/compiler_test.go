package typeenv

import (
	"bytes"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestPinnedPublicationCompilesArtifactAndRuntimeTypeEnv(t *testing.T) {
	snapshot := loadPinnedGrammarSnapshot(t)
	result, err := CompileBaseTypeEnv(snapshot)
	if err != nil {
		t.Fatalf("CompileBaseTypeEnv() error = %v", err)
	}
	accepted, ok := result.(compilationAccepted)
	if !ok {
		t.Fatalf("CompileBaseTypeEnv() = %T, diagnostics = %v", result, result.Diagnostics())
	}
	if result.CompilerSchemaVersion().String() != BaseTypeEnvCompilerSchemaV4 {
		t.Fatalf("compiler schema = %q, want v4", result.CompilerSchemaVersion().String())
	}
	if BaseTypeEnvCompilerSchemaV3 == BaseTypeEnvCompilerSchemaV4 {
		t.Fatal("v4 compiler silently relabelled the historical v3 edition")
	}
	artifact, exists := accepted.Artifact()
	if !exists {
		t.Fatal("accepted compilation has no artifact")
	}
	if err := artifact.Verify(); err != nil {
		t.Fatalf("artifact.Verify() error = %v", err)
	}
	ref, exists := artifact.TypeEnvRef()
	if !exists {
		t.Fatal("compiled artifact did not mint TypeEnvRef")
	}
	environment, exists := accepted.Environment()
	if !exists {
		t.Fatal("accepted compilation has no environment")
	}
	if environment.Ref().String() != ref.String() {
		t.Fatalf("environment ref = %q, artifact ref = %q", environment.Ref().String(), ref.String())
	}
	if environment.SourceRevision().String() != snapshot.Revision() {
		t.Fatalf("environment revision = %q, snapshot = %q", environment.SourceRevision().String(), snapshot.Revision())
	}
	if len(environment.BoundedContexts()) != 1 {
		t.Fatalf("bounded context count = %d, want compiler-derived publication context", len(environment.BoundedContexts()))
	}
	if len(environment.ContextKindAvailabilities()) != 0 {
		t.Fatalf(
			"compiler invented %d context-kind availabilities",
			len(environment.ContextKindAvailabilities()),
		)
	}
	if len(environment.SubkindRelations()) != 0 {
		t.Fatalf("compiler invented %d concrete U.SubkindOf edges", len(environment.SubkindRelations()))
	}
	if len(environment.TypedRelationDeclarationFragments()) != 0 {
		t.Fatalf(
			"compiler lowered %d partial relation fragments",
			len(environment.TypedRelationDeclarationFragments()),
		)
	}
	for _, declaration := range artifact.Declarations() {
		subject, err := typedmemory.SchemaSymbolCoverage(declaration.Symbol())
		if err != nil {
			t.Fatalf("SchemaSymbolCoverage(%s): %v", declaration.Symbol().String(), err)
		}
		coverage, exists := artifact.CoverageManifest().Entry(subject)
		if !exists {
			t.Fatalf("declaration %s has no coverage", declaration.Symbol().String())
		}
		if coverage.Posture() == typedmemory.CoverageSourceOnly {
			sourceOnlyRule := declaration.RuleID().String()
			if sourceOnlyRule != typedRelationFragmentRule &&
				sourceOnlyRule != c3SourceContractCompilerRule {
				t.Fatalf(
					"declaration %s is unexpectedly source-only under %s",
					declaration.Symbol().String(),
					sourceOnlyRule,
				)
			}
			continue
		}
		switch declaration.Symbol().Kind() {
		case typedmemory.ContextSymbol,
			typedmemory.KindSymbol,
			typedmemory.RefKindSymbol,
			typedmemory.ShapeSymbol,
			typedmemory.CodecSymbol:
			continue
		default:
			t.Fatalf("artifact marks non-materialized symbol compiled: %s", declaration.Symbol().String())
		}
	}

	wantKinds := []string{
		"U.ClaimGraph",
		"U.Entity",
		"U.Holon",
		"U.Episteme",
		"U.ReferenceScheme",
	}
	assertKindDefinitions(t, environment, wantKinds)
	wantRefs := []string{"U.EntityRef", "U.EpistemeRef", "U.HolonRef"}
	assertRefKindDefinitions(t, environment, wantRefs)
}

func TestPinnedPublicationPreservesThreeSymbolicC21DeclarationsWithoutLowering(t *testing.T) {
	snapshot := loadPinnedGrammarSnapshot(t)
	result, err := CompileBaseTypeEnv(snapshot)
	if err != nil {
		t.Fatalf("CompileBaseTypeEnv() error = %v", err)
	}
	artifact, exists := result.Artifact()
	if !exists {
		t.Fatalf("CompileBaseTypeEnv() rejected: %v", result.Diagnostics())
	}
	environment, exists := result.Environment()
	if !exists {
		t.Fatal("accepted compilation has no runtime environment")
	}
	if len(environment.TypedRelationDeclarationFragments()) != 0 {
		t.Fatalf(
			"runtime lowered %d source-only fragments",
			len(environment.TypedRelationDeclarationFragments()),
		)
	}

	wantRelations := []string{
		"EpistemeConstitutionRelation",
		"EpistemeEditionRelation",
		"EpistemeEmpiricalGroundingRelation",
	}
	for _, relationName := range wantRelations {
		relationID, _ := typedmemory.NewSignatureID(relationName)
		relationSymbol, _ := typedmemory.RelationSymbolRef(relationID)
		declaration, exists := artifactDeclaration(artifact, relationSymbol)
		if !exists {
			t.Fatalf("symbolic declaration %s is missing", relationName)
		}
		if declaration.RuleID().String() != typedRelationFragmentRule {
			t.Fatalf("%s compiler rule = %q", relationName, declaration.RuleID().String())
		}
		carrier, err := declarationTextField(declaration, "carrier_kind")
		if err != nil || carrier != "typed_relation_declaration_fragment" {
			t.Fatalf("%s carrier = %q, err=%v", relationName, carrier, err)
		}
		relationSubject, _ := typedmemory.SchemaSymbolCoverage(relationSymbol)
		entry, exists := artifact.CoverageManifest().Entry(relationSubject)
		if !exists || entry.Posture() != typedmemory.CoverageSourceOnly {
			t.Fatalf("%s coverage = %#v, want source_only", relationName, entry)
		}
		wantReason := "typed_relation_declaration_fragment_not_executable:applicability_dependency_closure_and_runtime_evaluator_unavailable"
		if entry.Rationale() != wantReason {
			t.Fatalf("%s gap = %q, want %q", relationName, entry.Rationale(), wantReason)
		}
	}
	decoded, err := DecodeBaseTypeEnvArtifact(artifact.CanonicalBytes())
	if err != nil {
		t.Fatalf("DecodeBaseTypeEnvArtifact(symbolic v3): %v", err)
	}
	decodedEnvironment, err := LowerBaseTypeEnvArtifact(decoded)
	if err != nil {
		t.Fatalf("LowerBaseTypeEnvArtifact(decoded symbolic v3): %v", err)
	}
	if len(decodedEnvironment.TypedRelationDeclarationFragments()) != 0 {
		t.Fatalf(
			"decoded artifact lowered %d symbolic fragments",
			len(decodedEnvironment.TypedRelationDeclarationFragments()),
		)
	}
	symbols := artifactSymbolStrings(artifact)
	assertNotContainsString(t, symbols, "signature:U.EpistemeSlotRelation")
}

func TestPinnedPublicationReportsHeterogeneousProseCoverage(t *testing.T) {
	snapshot := loadPinnedGrammarSnapshot(t)
	result, err := CompileBaseTypeEnv(snapshot)
	if err != nil {
		t.Fatalf("CompileBaseTypeEnv() error = %v", err)
	}
	artifact, exists := result.Artifact()
	if !exists {
		t.Fatalf("CompileBaseTypeEnv() rejected: %v", result.Diagnostics())
	}
	coverage := artifact.CoverageManifest()
	wantOwners := map[string]bool{
		"A.14":      false,
		"A.15":      false,
		"A.22.CGUS": false,
	}
	for _, entry := range coverage.Entries() {
		unitID, isUnit := entry.Subject().SourceUnitID()
		if !isUnit {
			continue
		}
		unit, exists := snapshot.ResolveSourceUnit(unitID.String())
		if !exists {
			t.Fatalf("coverage references unknown source unit %q", unitID.String())
		}
		owner := unit.PatternID
		if unit.Role == fpf.SourceUnitRolePatternSection {
			owner = unit.ParentPatternID
		}
		if _, tracked := wantOwners[owner]; !tracked {
			continue
		}
		if entry.Posture() != typedmemory.CoverageSourceOnly {
			t.Fatalf("%s coverage = %s, want source_only", owner, entry.Posture())
		}
		if entry.Rationale() != "heterogeneous_normative_prose_outside_cov2_grammar" {
			continue
		}
		wantOwners[owner] = true
	}
	for owner, found := range wantOwners {
		if !found {
			t.Fatalf("coverage has no exact source units for %s", owner)
		}
	}
}

func TestPinnedPublicationQuarantinesConflictingRelationOntology(t *testing.T) {
	snapshot := loadPinnedGrammarSnapshot(t)
	result, err := CompileBaseTypeEnv(snapshot)
	if err != nil {
		t.Fatalf("CompileBaseTypeEnv() error = %v", err)
	}
	artifact, exists := result.Artifact()
	if !exists {
		t.Fatalf("CompileBaseTypeEnv() rejected: %v", result.Diagnostics())
	}
	wantReasons := map[string]bool{
		"source_conflict_with_direct_governor:retired_relation_ontology":  false,
		"source_conflict_with_direct_governor:role_assignment_slot_model": false,
	}
	for _, entry := range artifact.CoverageManifest().Entries() {
		if _, tracked := wantReasons[entry.Rationale()]; tracked {
			wantReasons[entry.Rationale()] = true
		}
	}
	for reason, found := range wantReasons {
		if !found {
			t.Fatalf("coverage has no exact source-only conflict for %s", reason)
		}
	}
}

func TestRecognizedPublicationGrammarMutationRejectsWithoutArtifact(t *testing.T) {
	snapshot := loadPinnedGrammarSnapshot(t)
	unit := resolveGrammarSourceID(t, snapshot, "A.6.5:4.2")
	mutated := mutatePinnedStructuralSource(
		t,
		snapshot,
		"A.6.5:4.2",
		"SlotSpec := <SlotKind, ValueKind, refMode>",
		"SlotSpec := <SlotKind, ValueKind>",
	)
	result, err := CompileBaseTypeEnv(mutated)
	if err != nil {
		t.Fatalf("CompileBaseTypeEnv(mutated) error = %v", err)
	}
	rejected, ok := result.(compilationRejected)
	if !ok {
		t.Fatalf("mutated compilation = %T, want rejected result", result)
	}
	diagnostics := rejected.Diagnostics()
	if len(diagnostics) != 1 {
		t.Fatalf("diagnostic count = %d, want 1: %#v", len(diagnostics), diagnostics)
	}
	if diagnostics[0].UnitID() != unit.UnitID {
		t.Fatalf("diagnostic unit = %q", diagnostics[0].UnitID())
	}
	if diagnostics[0].Code() != "slot_spec_production_malformed" {
		t.Fatalf("diagnostic code = %q", diagnostics[0].Code())
	}
}

func TestDeletedStructuralCueFailsCompletenessAudit(t *testing.T) {
	snapshot := loadPinnedGrammarSnapshot(t)
	mutated := mutatePinnedStructuralSource(
		t,
		snapshot,
		"A.6.5:4.2",
		"SlotSpec := <SlotKind, ValueKind, refMode>",
		"PositionSpec := <SlotKind, ValueKind, refMode>",
	)
	result, err := CompileBaseTypeEnv(mutated)
	if err != nil {
		t.Fatalf("CompileBaseTypeEnv(mutated) error = %v", err)
	}
	if !result.Rejected() {
		t.Fatal("deleted structural cue silently compiled")
	}
	found := false
	for _, diagnostic := range result.Diagnostics() {
		if diagnostic.Code() != "structural_declaration_family_count_mismatch" {
			continue
		}
		if strings.Contains(diagnostic.Message(), "slot_spec_production") {
			found = true
		}
	}
	if !found {
		t.Fatalf("completeness diagnostics = %#v", result.Diagnostics())
	}
}

func TestSourceUnitPermutationDoesNotChangeCompilation(t *testing.T) {
	snapshot := loadPinnedGrammarSnapshot(t)
	revision, _ := typedmemory.NewSourceRevision(snapshot.Revision())
	compiler, _ := typedmemory.NewCompilerSchemaVersion(baseTypeEnvCompilerSchema)
	forwardUnits := snapshot.SourceUnits()
	reverseUnits := append([]fpf.SourceUnit(nil), forwardUnits...)
	for left, right := 0, len(reverseUnits)-1; left < right; left, right = left+1, right-1 {
		reverseUnits[left], reverseUnits[right] = reverseUnits[right], reverseUnits[left]
	}
	forward, err := compileSourceUnits(revision, compiler, forwardUnits)
	if err != nil {
		t.Fatalf("compileSourceUnits(forward) error = %v", err)
	}
	reverse, err := compileSourceUnits(revision, compiler, reverseUnits)
	if err != nil {
		t.Fatalf("compileSourceUnits(reverse) error = %v", err)
	}
	forwardArtifact, forwardExists := forward.Artifact()
	reverseArtifact, reverseExists := reverse.Artifact()
	if !forwardExists || !reverseExists {
		t.Fatalf("permuted compilation rejected: %v / %v", forward.Diagnostics(), reverse.Diagnostics())
	}
	if forwardArtifact.Digest().String() != reverseArtifact.Digest().String() {
		t.Fatalf("permutation changed digest: %s != %s", forwardArtifact.Digest().String(), reverseArtifact.Digest().String())
	}
	if !bytes.Equal(forwardArtifact.CanonicalBytes(), reverseArtifact.CanonicalBytes()) {
		t.Fatal("permutation changed canonical artifact bytes")
	}
}

func assertKindDefinitions(
	t *testing.T,
	environment typedmemory.TypeEnv,
	want []string,
) {
	t.Helper()
	got := make([]string, 0, len(environment.KindDefinitions()))
	for _, definition := range environment.KindDefinitions() {
		got = append(got, definition.ID().String())
	}
	for _, value := range want {
		assertContainsString(t, got, value)
	}
}

func assertRefKindDefinitions(
	t *testing.T,
	environment typedmemory.TypeEnv,
	want []string,
) {
	t.Helper()
	got := make([]string, 0, len(environment.RefKindDefinitions()))
	for _, definition := range environment.RefKindDefinitions() {
		got = append(got, definition.Ref().ID().String())
	}
	for _, value := range want {
		assertContainsString(t, got, value)
	}
}

func artifactSymbolStrings(artifact BaseTypeEnvArtifact) []string {
	values := make([]string, 0, len(artifact.Declarations()))
	for _, declaration := range artifact.Declarations() {
		values = append(values, declaration.Symbol().String())
	}
	return values
}

func mutatePinnedStructuralSource(
	t *testing.T,
	snapshot fpf.PublicationSnapshot,
	sourceID string,
	before string,
	after string,
) fpf.PublicationSnapshot {
	t.Helper()
	unit := resolveGrammarSourceID(t, snapshot, sourceID)
	if strings.Count(unit.Body, before) != 1 {
		t.Fatalf("source %s contains %d mutation anchors, want 1", sourceID, strings.Count(unit.Body, before))
	}
	mutatedBody := strings.Replace(unit.Body, before, after, 1)
	bundle := snapshot.SourceBundle()
	if bytes.Count(bundle.Spec.Markdown, []byte(unit.Body)) != 1 {
		t.Fatalf("source %s body is not one exact publication span", sourceID)
	}
	bundle.Spec.Markdown = bytes.Replace(
		bundle.Spec.Markdown,
		[]byte(unit.Body),
		[]byte(mutatedBody),
		1,
	)
	mutated, err := fpf.BuildPublicationSnapshot(bundle)
	if err != nil {
		t.Fatalf("BuildPublicationSnapshot(%s mutation): %v", sourceID, err)
	}
	return mutated
}

func assertContainsString(t *testing.T, values []string, want string) {
	t.Helper()
	for _, value := range values {
		if value == want {
			return
		}
	}
	t.Fatalf("%q not found in %s", want, strings.Join(values, ", "))
}

func assertNotContainsString(t *testing.T, values []string, unwanted string) {
	t.Helper()
	for _, value := range values {
		if value == unwanted {
			t.Fatalf("unexpected %q in %s", unwanted, strings.Join(values, ", "))
		}
	}
}
