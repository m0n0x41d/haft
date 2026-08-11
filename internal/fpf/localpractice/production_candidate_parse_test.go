package localpractice

import (
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
	"testing"
)

const (
	productionCandidateCarrierV1Path     = "../../../data/haft/local-practice/typed-memory/candidates/1.0.0.yaml"
	productionCandidateCarrierV1_1Path   = "../../../data/haft/local-practice/typed-memory/candidates/1.1.0.yaml"
	productionCandidateCarrierV1_2Path   = "../../../data/haft/local-practice/typed-memory/candidates/1.2.0.yaml"
	productionCandidateCarrierV1_3Path   = "../../../data/haft/local-practice/typed-memory/candidates/1.3.0.yaml"
	productionCandidateCarrierV1_4Path   = "../../../data/haft/local-practice/typed-memory/candidates/1.4.0.yaml"
	productionCandidateCarrierV1_5Path   = "../../../data/haft/local-practice/typed-memory/candidates/1.5.0.yaml"
	productionCandidateCarrierV1_6Path   = "../../../data/haft/local-practice/typed-memory/candidates/1.6.0.yaml"
	productionCandidateCarrierV1Digest   = "4f80253dedf46d40ca63662bb0e48c39991a36e1554028b00ed87ad242b4a7f7"
	productionCandidateCarrierV1_1Digest = "bf0c00131ac84cca8dc62a3e3631b56415b4946fe868f1c286df1253c397393c"
	productionCandidateCarrierV1_2Digest = "3d4cfaef710daf2ec70a43970ef4ebff2a0cdeac4d26da0486947a4a15ed4d2e"
	productionCandidateCarrierV1_3Digest = "cffa1363f6b0b5cd1bc701f48c117826a802e640205a1f76e0957f78b5897327"
	productionCandidateCarrierV1_4Digest = "8c4544660446dff9de3edb8ac93b6ccd9378e69e2909bb57c47d107a0900a2b7"
	productionCandidateCarrierV1_5Digest = "a0c633b8d088ae2acddfd367bab572a94c7209d834ecb364ac38665ced74ced3"
	productionCandidateCarrierV1_6Digest = "9d43be568fff986c3ab664b88f85e4533de68d2147faa4a60b85d5acd3c9a8c2"
	productionCandidateBaseV1_1          = "typeenv:sha256:a5223d5018230095652543f0378a1fc3f64175f21d01309e6f4084088d5d2804"
	productionCandidateBaseV1_2          = "typeenv:sha256:973eeeed8e234b4ff0194662d80e204fe27ad5ba92c87840a6d1ed3a9d5d742d"
	productionCandidateBaseV1_3          = "typeenv:sha256:28c7650b8933cbf6feb5d87965d48b4a8c7b80ae71c9c0ca4990d8ae7b6a36b6"
	productionCandidateBaseV1_4          = "typeenv:sha256:effff65cae9eaf1aba287245df79c460fbeaee5f666dcaa7992bfeb251c1e35e"
	productionCandidateBaseV1_5          = "typeenv:sha256:1b6b04c14aa43bea396aafdbd810eb0345f7f9e9be37a5aee874a328c3b26efc"
	productionCandidateBaseV1_6          = "typeenv:sha256:dffe960ad95df0f16c66c4040dfcb3c20ea19dc1aa1a4d506bb1dae77e514565"
)

func TestProductionCandidateCarrierV1RemainsByteStableAndReplayable(t *testing.T) {
	source, parsed := parseProductionCandidate(t, productionCandidateCarrierV1Path)
	digest := sha256.Sum256(source)
	gotDigest := fmt.Sprintf("%x", digest)
	if gotDigest != productionCandidateCarrierV1Digest {
		t.Fatalf("1.0.0 source digest = %q, want %q", gotDigest, productionCandidateCarrierV1Digest)
	}
	assertNonCoreCandidate(t, parsed)

	runtimeRequirements := 0
	for _, declaration := range parsed.Carrier().Signature().Vocabulary().Declarations() {
		if declaration.Kind() == DeclarationRuntimeEvaluatorRequirement {
			runtimeRequirements++
		}
		if declaration.Kind() == DeclarationRuntimeEvaluatorInput {
			t.Fatal("historical 1.0.0 unexpectedly contains a runtime evaluator input")
		}
	}
	if runtimeRequirements != 0 {
		t.Fatalf("historical 1.0.0 runtime evaluator requirements = %d, want 0", runtimeRequirements)
	}
}

func TestProductionCandidateCarrierV1_1RemainsByteStableAndReplayable(t *testing.T) {
	source, parsed := parseProductionCandidate(t, productionCandidateCarrierV1_1Path)
	digest := sha256.Sum256(source)
	gotDigest := fmt.Sprintf("%x", digest)
	if gotDigest != productionCandidateCarrierV1_1Digest {
		t.Fatalf("1.1.0 source digest = %q, want %q", gotDigest, productionCandidateCarrierV1_1Digest)
	}
	assertReferenceSchemeRuntimeContract(
		t,
		parsed,
		"1.1.0",
		productionCandidateBaseV1_1,
	)
}

func TestHistoricalCandidateCarrierV1_2RemainsByteStableAndReplayable(t *testing.T) {
	source, parsed := parseProductionCandidate(t, productionCandidateCarrierV1_2Path)
	digest := sha256.Sum256(source)
	if got := fmt.Sprintf("%x", digest); got != productionCandidateCarrierV1_2Digest {
		t.Fatalf("1.2.0 source digest = %q, want %q", got, productionCandidateCarrierV1_2Digest)
	}
	assertReferenceSchemeRuntimeContract(
		t,
		parsed,
		"1.2.0",
		productionCandidateBaseV1_2,
	)
}

func TestHistoricalCandidateCarrierV1_3RemainsByteStableAndUsesKindClassification(
	t *testing.T,
) {
	assertKindClassificationCandidate(
		t,
		"historical",
		productionCandidateCarrierV1_3Path,
		productionCandidateCarrierV1_3Digest,
		"1.3.0",
		productionCandidateBaseV1_3,
	)
}

// TestCurrentCandidateCarrierV1_4RemainsByteStableAndUsesKindClassification
// retains its historical P13 anchor name. The assertions now state the
// carrier's current posture explicitly: 1.4.0 is byte-stable replay input,
// while later editions are separate non-binding successor candidates below.
func TestCurrentCandidateCarrierV1_4RemainsByteStableAndUsesKindClassification(
	t *testing.T,
) {
	assertKindClassificationCandidate(
		t,
		"historical",
		productionCandidateCarrierV1_4Path,
		productionCandidateCarrierV1_4Digest,
		"1.4.0",
		productionCandidateBaseV1_4,
	)
}

func TestHistoricalCandidateCarrierV1_5RemainsByteStableAndUsesKindClassification(
	t *testing.T,
) {
	assertKindClassificationCandidate(
		t,
		"historical",
		productionCandidateCarrierV1_5Path,
		productionCandidateCarrierV1_5Digest,
		"1.5.0",
		productionCandidateBaseV1_5,
	)
}

func TestCurrentCandidateCarrierV1_6RemainsByteStableAndUsesKindClassification(
	t *testing.T,
) {
	assertKindClassificationCandidate(
		t,
		"current",
		productionCandidateCarrierV1_6Path,
		productionCandidateCarrierV1_6Digest,
		"1.6.0",
		productionCandidateBaseV1_6,
	)
}

func assertKindClassificationCandidate(
	t *testing.T,
	posture string,
	path string,
	wantDigest string,
	wantEdition string,
	wantBase string,
) {
	t.Helper()
	source, parsed := parseProductionCandidate(t, path)
	digest := sha256.Sum256(source)
	if got := fmt.Sprintf("%x", digest); got != wantDigest {
		t.Fatalf("%s source digest = %q, want %q", wantEdition, got, wantDigest)
	}
	assertReferenceSchemeRuntimeContract(
		t,
		parsed,
		wantEdition,
		wantBase,
	)
	classificationSignatures := 0
	for _, declaration := range parsed.Carrier().Signature().Vocabulary().Declarations() {
		switch declaration.Kind() {
		case DeclarationKindClassificationSignature:
			classificationSignatures++
		case DeclarationEntitySet, DeclarationKindSignature:
			t.Fatalf(
				"%s %s contains sealed historical declaration %q",
				posture,
				wantEdition,
				declaration.Kind(),
			)
		}
	}
	if classificationSignatures != 12 {
		t.Fatalf(
			"%s %s classification signatures = %d, want 12",
			posture,
			wantEdition,
			classificationSignatures,
		)
	}
}

func assertReferenceSchemeRuntimeContract(
	t *testing.T,
	parsed ParsedCarrier,
	wantEdition string,
	wantBase string,
) {
	t.Helper()
	assertNonCoreCandidate(t, parsed)
	carrier := parsed.Carrier()
	if got := carrier.Identity().Edition().Value(); got != wantEdition {
		t.Fatalf("carrier edition = %q, want %s", got, wantEdition)
	}
	if got := carrier.Manifest().Version().Value(); got != wantEdition {
		t.Fatalf("manifest version = %q, want %s", got, wantEdition)
	}
	if got := carrier.BaseTypeEnvRef().Value(); got != wantBase {
		t.Fatalf("base_type_env_ref = %q, want %q", got, wantBase)
	}

	declarations := carrier.Signature().Vocabulary().Declarations()
	bySymbol := make(map[string]Declaration, len(declarations))
	for _, declaration := range declarations {
		bySymbol[declaration.Symbol().Value()] = declaration
	}
	assertProjectEpistemeConstitutionInput(t, bySymbol)
	assertProjectMemoryReferenceSchemeShapes(t, bySymbol)
	assertProjectMemoryReferenceSchemeCodec(t, bySymbol)
	assertProjectMemoryRuntimeRequirements(t, bySymbol)
}

func assertNonCoreCandidate(t *testing.T, parsed ParsedCarrier) {
	t.Helper()
	manifest := parsed.Carrier().Manifest()
	state, present := manifest.PublicationState()
	if !present || state != PublicationCandidate {
		t.Fatalf("publication state = %q, present = %v", state, present)
	}
	if len(manifest.Imports()) != 0 {
		t.Fatal("candidate fabricated a SignatureManifest import for the compiled FPF base")
	}
	for _, provided := range manifest.Provides() {
		if strings.HasPrefix(provided.Symbol().Value(), "U.") {
			t.Fatalf("candidate exports forbidden FPF Core symbol %q", provided.Symbol().Value())
		}
	}
}

func assertProjectEpistemeConstitutionInput(
	t *testing.T,
	bySymbol map[string]Declaration,
) {
	t.Helper()
	const symbol = "Haft.ProjectEpistemeConstitutionBasis"
	declaration, exists := bySymbol[symbol]
	if !exists {
		t.Fatalf("candidate is missing %q", symbol)
	}
	input, ok := declaration.(RuntimeEvaluatorInputDeclaration)
	if !ok {
		t.Fatalf("%s declaration = %T, want RuntimeEvaluatorInputDeclaration", symbol, declaration)
	}
	const requirement = "Haft.RuntimeRequirement.ProjectEpistemeConstitutionEvaluationV1"
	if got := input.EvaluatorRequirement().Value(); got != requirement {
		t.Fatalf("%s evaluator requirement = %q, want %q", symbol, got, requirement)
	}

	slots := input.Slots()
	if len(slots) != 3 {
		t.Fatalf("%s slots = %d, want 3", symbol, len(slots))
	}
	assertInputSlot(t, slots[0], symbol+".ClaimGraphSlot", "U.ClaimGraph", ReferenceByValue, "")
	assertInputSlot(t, slots[1], symbol+".EntityOfConcernSlot", "U.Entity", ReferenceByKind, "U.EntityRef")
	assertInputSlot(t, slots[2], symbol+".ReferenceSchemeSlot", "U.ReferenceScheme", ReferenceByValue, "")
}

func assertInputSlot(
	t *testing.T,
	slot SlotSpec,
	wantSlot string,
	wantValue string,
	wantMode ReferenceModeKind,
	wantRefKind string,
) {
	t.Helper()
	if got := slot.SlotKind().Value(); got != wantSlot {
		t.Fatalf("slot kind = %q, want %q", got, wantSlot)
	}
	if got := slot.ValueKind().Value(); got != wantValue {
		t.Fatalf("%s value kind = %q, want %q", wantSlot, got, wantValue)
	}
	mode := slot.ReferenceMode()
	if got := mode.Kind(); got != wantMode {
		t.Fatalf("%s reference mode = %q, want %q", wantSlot, got, wantMode)
	}
	if wantMode == ReferenceByValue {
		return
	}
	reference, ok := mode.(RefKindReferenceMode)
	if !ok {
		t.Fatalf("%s reference mode = %T, want RefKindReferenceMode", wantSlot, mode)
	}
	if got := reference.RefKind().Value(); got != wantRefKind {
		t.Fatalf("%s RefKind = %q, want %q", wantSlot, got, wantRefKind)
	}
}

func assertProjectMemoryReferenceSchemeShapes(
	t *testing.T,
	bySymbol map[string]Declaration,
) {
	t.Helper()
	wantKinds := map[string]ValueShapeKind{
		"Haft.Shape.ProjectMemoryReferenceSchemeExactSourceCarrierPinV1":     ValueShapeRecord,
		"Haft.Shape.ProjectMemoryReferenceSchemeExactRuntimeMechanismPinV1":  ValueShapeRecord,
		"Haft.Shape.ProjectMemoryReferenceSchemeRuntimeNotRequiredRulePinV1": ValueShapeRecord,
		"Haft.Shape.ProjectMemoryReferenceSchemeRuntimeRequiredRulePinV1":    ValueShapeRecord,
		"Haft.Shape.ProjectMemoryReferenceSchemeExactRulePinV1":              ValueShapeSum,
		"Haft.Shape.ProjectMemoryReferenceSchemeExactRulePinSetV1":           ValueShapeUnorderedSet,
		"Haft.Shape.ProjectMemoryReferenceSchemeRuleBranchV1":                ValueShapeSum,
		"Haft.Shape.ProjectMemoryReferenceSchemeV1":                          ValueShapeRecord,
	}
	for symbol, wantKind := range wantKinds {
		declaration, exists := bySymbol[symbol]
		if !exists {
			t.Fatalf("candidate is missing shape %q", symbol)
		}
		shapeDeclaration, ok := declaration.(ValueShapeDeclaration)
		if !ok {
			t.Fatalf("%s declaration = %T, want ValueShapeDeclaration", symbol, declaration)
		}
		if got := shapeDeclaration.Shape().Kind(); got != wantKind {
			t.Fatalf("%s shape kind = %q, want %q", symbol, got, wantKind)
		}
	}

	rootDeclaration := bySymbol["Haft.Shape.ProjectMemoryReferenceSchemeV1"]
	rootShape := rootDeclaration.(ValueShapeDeclaration).Shape()
	record, ok := rootShape.(RecordValueShape)
	if !ok {
		t.Fatalf("root ReferenceScheme shape = %T, want RecordValueShape", rootShape)
	}
	wantFields := []string{"designation", "interpretation", "measurement", "evaluation"}
	fields := record.Fields()
	if len(fields) != len(wantFields) {
		t.Fatalf("root ReferenceScheme fields = %d, want %d", len(fields), len(wantFields))
	}
	for index, want := range wantFields {
		if got := fields[index].Name().Value(); got != want {
			t.Fatalf("root ReferenceScheme field[%d] = %q, want %q", index, got, want)
		}
	}
}

func assertProjectMemoryReferenceSchemeCodec(
	t *testing.T,
	bySymbol map[string]Declaration,
) {
	t.Helper()
	const symbol = "Haft.Codec.ProjectMemoryReferenceSchemeV1"
	declaration, exists := bySymbol[symbol]
	if !exists {
		t.Fatalf("candidate is missing codec %q", symbol)
	}
	codec, ok := declaration.(CodecBindingDeclaration)
	if !ok {
		t.Fatalf("%s declaration = %T, want CodecBindingDeclaration", symbol, declaration)
	}
	if got := codec.ValueKind().Value(); got != "U.ReferenceScheme" {
		t.Fatalf("%s value kind = %q, want U.ReferenceScheme", symbol, got)
	}
	if got := codec.ValueShape().Value(); got != "Haft.Shape.ProjectMemoryReferenceSchemeV1" {
		t.Fatalf("%s value shape = %q", symbol, got)
	}
}

type runtimeRequirementExpectation struct {
	ruleRef  string
	contract string
}

func assertProjectMemoryRuntimeRequirements(
	t *testing.T,
	bySymbol map[string]Declaration,
) {
	t.Helper()
	want := map[string]runtimeRequirementExpectation{
		"Haft.RuntimeRequirement.ProjectMemoryReferenceDesignationResolutionV1": {
			ruleRef:  "haft.reference-scheme.project-memory/v1/designation-resolution",
			contract: "reference_designation_resolution",
		},
		"Haft.RuntimeRequirement.ProjectMemoryClaimInterpretationV1": {
			ruleRef:  "haft.reference-scheme.project-memory/v1/claim-interpretation",
			contract: "claim_interpretation",
		},
		"Haft.RuntimeRequirement.ProjectMemoryClaimMeasurementV1": {
			ruleRef:  "haft.reference-scheme.project-memory/v1/claim-measurement",
			contract: "claim_measurement",
		},
		"Haft.RuntimeRequirement.ProjectMemoryClaimEvaluationV1": {
			ruleRef:  "haft.reference-scheme.project-memory/v1/claim-evaluation",
			contract: "claim_evaluation",
		},
		"Haft.RuntimeRequirement.ProjectEpistemeConstitutionEvaluationV1": {
			ruleRef:  "haft.episteme-constitution.project-memory/v1/evaluate",
			contract: "episteme_constitution_evaluation",
		},
	}
	for symbol, expected := range want {
		declaration, exists := bySymbol[symbol]
		if !exists {
			t.Fatalf("candidate is missing runtime requirement %q", symbol)
		}
		requirement, ok := declaration.(RuntimeEvaluatorRequirementDeclaration)
		if !ok {
			t.Fatalf("%s declaration = %T, want RuntimeEvaluatorRequirementDeclaration", symbol, declaration)
		}
		if got := requirement.RuleRef().Value(); got != expected.ruleRef {
			t.Fatalf("%s RuleRef = %q, want %q", symbol, got, expected.ruleRef)
		}
		if got := requirement.InvocationContract().Value(); got != expected.contract {
			t.Fatalf("%s invocation contract = %q, want %q", symbol, got, expected.contract)
		}
	}
}

func parseProductionCandidate(t *testing.T, path string) ([]byte, ParsedCarrier) {
	t.Helper()
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read production candidate carrier %s: %v", path, err)
	}
	parsed, err := Parse(source)
	if err != nil {
		t.Fatalf("Parse(production candidate %s): %v", path, err)
	}
	return source, parsed
}
