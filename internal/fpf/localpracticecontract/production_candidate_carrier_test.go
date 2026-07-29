package localpracticecontract_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf/localpractice"
	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/fpf/typeenvsql"
	_ "modernc.org/sqlite"
)

const typedMemoryCandidateCarrierPath = "../../../data/haft/local-practice/typed-memory/candidates/1.4.0.yaml"

func TestTypedMemoryCandidateCarrierParsesCompilesAndSeals(t *testing.T) {
	source := readTypedMemoryCandidateCarrier(t)
	parsed, err := localpractice.Parse(source)
	if err != nil {
		t.Fatalf("parse production candidate carrier: %v", err)
	}
	assertTypedMemoryCandidateSourceContract(t, parsed)

	base := loadCurrentBaseTypeEnv(t)
	baseRef, exists := base.TypeEnvRef()
	if !exists {
		t.Fatal("current source-derived FPF base has no executable TypeEnvRef")
	}
	if got := parsed.Carrier().BaseTypeEnvRef().Value(); got != baseRef.String() {
		t.Fatalf("candidate base_type_env_ref = %q, current embedded B = %q", got, baseRef.String())
	}

	resolution := projecttypeenv.ResolveManifestGraph(
		base,
		[]localpractice.ParsedCarrier{parsed},
	)
	if resolution.Rejected() {
		t.Fatalf("resolve candidate manifest: %#v", resolution.Issues())
	}
	bundle, accepted := resolution.Bundle()
	if !accepted {
		t.Fatal("accepted candidate manifest has no resolved bundle")
	}
	nodes := bundle.Nodes()
	if len(nodes) != 1 {
		t.Fatalf("candidate manifest nodes = %d, want 1", len(nodes))
	}
	ir, err := projecttypeenv.CompileProjectTypeEnvExtensionIR(nodes[0], nil)
	if err != nil {
		t.Fatalf("compile candidate E IR: %v", err)
	}
	artifact, err := projecttypeenv.SealProjectTypeEnvExtension(ir)
	if err != nil {
		t.Fatalf("seal candidate E artifact: %v", err)
	}
	if err := artifact.Verify(); err != nil {
		t.Fatalf("verify candidate E artifact: %v", err)
	}
	if artifact.ManifestCoordinate().String() != "haft.typed-memory@1.4.0" {
		t.Fatalf("candidate E coordinate = %q", artifact.ManifestCoordinate().String())
	}
	if artifact.IR().BaseTypeEnvRef() != baseRef {
		t.Fatal("sealed candidate E lost its exact B reference")
	}
	linked := projecttypeenv.LinkProjectTypeEnvCompositeIR(
		base,
		[]projecttypeenv.ProjectTypeEnvExtensionArtifact{artifact},
	)
	if linked.Rejected() {
		t.Fatalf("link candidate E against exact B: %#v", linked.Issues())
	}
	compositeIR, accepted := linked.CompositeIR()
	if !accepted || len(compositeIR.Extensions()) != 1 {
		t.Fatal("accepted candidate E did not produce one linked extension")
	}
	if len(compositeIR.CoverageGaps()) != 1 ||
		compositeIR.CoverageGaps()[0].Code() != projecttypeenv.CompositeGapStratumDirectionUnresolved {
		t.Fatalf("candidate coverage gaps = %#v, want only explicit stratum-direction gap", compositeIR.CoverageGaps())
	}
	assertReferenceSchemeRuntimeRequirements(t, base, compositeIR)
}

func assertTypedMemoryCandidateSourceContract(
	t *testing.T,
	parsed localpractice.ParsedCarrier,
) {
	t.Helper()
	carrier := parsed.Carrier()
	manifest := carrier.Manifest()
	state, exists := manifest.PublicationState()
	if !exists || state != localpractice.PublicationCandidate {
		t.Fatalf("candidate publication state = %q, present = %v", state, exists)
	}
	if len(manifest.Imports()) != 0 {
		t.Fatal("candidate fabricated a source manifest import for the compiled FPF base")
	}
	for _, provided := range manifest.Provides() {
		if strings.HasPrefix(provided.Symbol().Value(), "U.") {
			t.Fatalf("candidate exports forbidden FPF Core symbol %q", provided.Symbol().Value())
		}
	}

	declarations := carrier.Signature().Vocabulary().Declarations()
	wantCounts := map[localpractice.DeclarationKind]int{
		localpractice.DeclarationBoundedContext:              1,
		localpractice.DeclarationValueKind:                   17,
		localpractice.DeclarationSubkind:                     11,
		localpractice.DeclarationRefKind:                     11,
		localpractice.DeclarationKindClassificationSignature: 12,
		localpractice.DeclarationRelationSignature:           16,
		localpractice.DeclarationRuntimeEvaluatorInput:       1,
		localpractice.DeclarationValueShape:                  20,
		localpractice.DeclarationCodecBinding:                7,
		localpractice.DeclarationRuntimeEvaluatorRequirement: 5,
		localpractice.DeclarationConstraint:                  67,
	}
	gotCounts := make(map[localpractice.DeclarationKind]int, len(wantCounts))
	cardinalities := make(map[string]int)
	relationSlots := make(map[string]struct{})
	shapeSymbols := make(map[string]struct{})
	codecSymbols := make(map[string]struct{})
	runtimeRequirements := make(map[string]string)
	constitutionInputFound := false
	for _, declaration := range declarations {
		gotCounts[declaration.Kind()]++
		switch value := declaration.(type) {
		case localpractice.RelationSignatureDeclaration:
			for _, slot := range value.Slots() {
				coordinate := value.Symbol().Value() + "\x00" + slot.SlotKind().Value()
				relationSlots[coordinate] = struct{}{}
			}
		case localpractice.ConstraintDeclaration:
			cardinality, isCardinality := value.Rule().(localpractice.SlotCardinalityConstraint)
			if isCardinality {
				coordinate := cardinality.Relation().Value() + "\x00" + cardinality.Slot().Value()
				cardinalities[coordinate]++
			}
		case localpractice.ValueShapeDeclaration:
			shapeSymbols[value.Symbol().Value()] = struct{}{}
		case localpractice.CodecBindingDeclaration:
			codecSymbols[value.Symbol().Value()] = struct{}{}
		case localpractice.RuntimeEvaluatorRequirementDeclaration:
			coordinate := value.InvocationContract().Value() + "\x00" + value.RuleRef().Value()
			runtimeRequirements[coordinate] = value.Symbol().Value()
		case localpractice.RuntimeEvaluatorInputDeclaration:
			if value.Symbol().Value() != "Haft.ProjectEpistemeConstitutionBasis" {
				t.Fatalf("unexpected runtime evaluator input %q", value.Symbol().Value())
			}
			if value.EvaluatorRequirement().Value() != "Haft.RuntimeRequirement.ProjectEpistemeConstitutionEvaluationV1" {
				t.Fatalf("constitution input evaluator requirement = %q", value.EvaluatorRequirement().Value())
			}
			if len(value.Slots()) != 3 {
				t.Fatalf("constitution input slots = %d, want 3", len(value.Slots()))
			}
			constitutionInputFound = true
		}
	}
	for kind, want := range wantCounts {
		if got := gotCounts[kind]; got != want {
			t.Fatalf("candidate %s declarations = %d, want %d", kind, got, want)
		}
	}
	for coordinate := range relationSlots {
		if cardinalities[coordinate] != 1 {
			t.Fatalf("relation slot %q has %d cardinality constraints, want exactly 1", coordinate, cardinalities[coordinate])
		}
	}
	if len(cardinalities) != len(relationSlots) {
		t.Fatalf("cardinality coordinates = %d, relation slots = %d", len(cardinalities), len(relationSlots))
	}
	if !constitutionInputFound {
		t.Fatal("candidate is missing ProjectEpistemeConstitutionBasis runtime input")
	}

	for _, symbol := range []string{
		"Haft.Shape.EvidenceUseQualifierV1",
		"Haft.Shape.PerformedIntervalV1",
		"Haft.Shape.CanonicalInstantV1",
		"Haft.Shape.CodeAnchorLocatorV1",
		"Haft.Shape.ProjectMemoryReferenceSchemeExactSourceCarrierPinV1",
		"Haft.Shape.ProjectMemoryReferenceSchemeExactRuntimeMechanismPinV1",
		"Haft.Shape.ProjectMemoryReferenceSchemeRuntimeNotRequiredRulePinV1",
		"Haft.Shape.ProjectMemoryReferenceSchemeRuntimeRequiredRulePinV1",
		"Haft.Shape.ProjectMemoryReferenceSchemeExactRulePinV1",
		"Haft.Shape.ProjectMemoryReferenceSchemeExactRulePinSetV1",
		"Haft.Shape.ProjectMemoryReferenceSchemeRuleBranchV1",
		"Haft.Shape.ProjectMemoryReferenceSchemeV1",
	} {
		if _, exists := shapeSymbols[symbol]; !exists {
			t.Fatalf("candidate is missing required shape %q", symbol)
		}
	}
	for _, symbol := range []string{
		"Haft.Codec.EvidenceUseQualifierV1",
		"Haft.Codec.PerformedIntervalV1",
		"Haft.Codec.CanonicalInstantV1",
		"Haft.Codec.CodeAnchorLocatorV1",
		"Haft.Codec.ProjectMemoryReferenceSchemeV1",
	} {
		if _, exists := codecSymbols[symbol]; !exists {
			t.Fatalf("candidate is missing required conceptual codec %q", symbol)
		}
	}
	wantRequirements := map[string]string{
		"reference_designation_resolution\x00haft.reference-scheme.project-memory/v1/designation-resolution": "Haft.RuntimeRequirement.ProjectMemoryReferenceDesignationResolutionV1",
		"claim_interpretation\x00haft.reference-scheme.project-memory/v1/claim-interpretation":               "Haft.RuntimeRequirement.ProjectMemoryClaimInterpretationV1",
		"claim_measurement\x00haft.reference-scheme.project-memory/v1/claim-measurement":                     "Haft.RuntimeRequirement.ProjectMemoryClaimMeasurementV1",
		"claim_evaluation\x00haft.reference-scheme.project-memory/v1/claim-evaluation":                       "Haft.RuntimeRequirement.ProjectMemoryClaimEvaluationV1",
		"episteme_constitution_evaluation\x00haft.episteme-constitution.project-memory/v1/evaluate":          "Haft.RuntimeRequirement.ProjectEpistemeConstitutionEvaluationV1",
	}
	if len(runtimeRequirements) != len(wantRequirements) {
		t.Fatalf("runtime evaluator requirements = %d, want %d", len(runtimeRequirements), len(wantRequirements))
	}
	for coordinate, wantSymbol := range wantRequirements {
		if gotSymbol := runtimeRequirements[coordinate]; gotSymbol != wantSymbol {
			t.Fatalf("runtime requirement %q symbol = %q, want %q", coordinate, gotSymbol, wantSymbol)
		}
	}
}

func assertReferenceSchemeRuntimeRequirements(
	t *testing.T,
	base typeenv.BaseTypeEnvArtifact,
	linked projecttypeenv.LinkedProjectTypeEnvCompositeIR,
) {
	t.Helper()
	discovery := projecttypeenv.DiscoverProjectTypeEnvCompositeRuntimeRequirements(base, linked)
	if discovery.Rejected() {
		t.Fatalf("discover candidate runtime requirements: %#v", discovery.Issues())
	}
	required, accepted := discovery.RequiredSet()
	if !accepted {
		t.Fatal("accepted runtime-requirement discovery has no required set")
	}
	want := map[string]struct{}{
		"reference_designation_resolution\x00haft.reference-scheme.project-memory/v1/designation-resolution": {},
		"claim_interpretation\x00haft.reference-scheme.project-memory/v1/claim-interpretation":               {},
		"claim_measurement\x00haft.reference-scheme.project-memory/v1/claim-measurement":                     {},
		"claim_evaluation\x00haft.reference-scheme.project-memory/v1/claim-evaluation":                       {},
		"episteme_constitution_evaluation\x00haft.episteme-constitution.project-memory/v1/evaluate":          {},
	}
	got := make(map[string]struct{})
	for _, requirement := range required.Requirements() {
		rule, hasRule := requirement.Rule()
		if !hasRule {
			continue
		}
		coordinate := requirement.InvocationContract().String() + "\x00" + rule.String()
		if _, expected := want[coordinate]; expected {
			got[coordinate] = struct{}{}
		}
	}
	if len(got) != len(want) {
		t.Fatalf("source-derived ReferenceScheme runtime requirements = %#v, want %#v", got, want)
	}
}

func readTypedMemoryCandidateCarrier(t *testing.T) []byte {
	t.Helper()
	source, err := os.ReadFile(typedMemoryCandidateCarrierPath)
	if err != nil {
		t.Fatalf("read %s: %v", typedMemoryCandidateCarrierPath, err)
	}
	return source
}

func loadCurrentBaseTypeEnv(t *testing.T) typeenv.BaseTypeEnvArtifact {
	t.Helper()
	path, err := filepath.Abs("../../cli/fpf.db")
	if err != nil {
		t.Fatalf("resolve embedded FPF index: %v", err)
	}
	database, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?mode=ro&immutable=1")
	if err != nil {
		t.Fatalf("open embedded FPF index read-only: %v", err)
	}
	database.SetMaxOpenConns(1)
	t.Cleanup(func() {
		if closeErr := database.Close(); closeErr != nil {
			t.Errorf("close embedded FPF index: %v", closeErr)
		}
	})
	artifact, err := typeenvsql.LoadArtifactReadOnlyDB(context.Background(), database)
	if err != nil {
		t.Fatalf("load exact source-derived FPF base: %v", err)
	}
	return artifact
}
