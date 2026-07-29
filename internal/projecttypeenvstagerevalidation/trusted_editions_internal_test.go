package projecttypeenvstagerevalidation

import (
	"bytes"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
)

func TestStaticEditionComparisonCoversEveryCoordinateInCanonicalOrder(t *testing.T) {
	catalog := CurrentTrustedStageEditionCatalog()
	assertTrustedStageEditionCatalogWellFormed(t, catalog)
	zero := TrustedStageEditionCatalog{}
	if len(zero.canonical) != 0 || zero.digest != (TrustedStageEditionCatalogDigest{}) {
		t.Fatal("zero catalog acquired package-owned canonical identity")
	}
	observation := staticStageEditionObservation{
		stageSchema:      "unsupported-stage-schema",
		stageCompiler:    "unsupported-stage-compiler",
		baseCompiler:     "unsupported-base-compiler",
		stageProducer:    "unsupported-stage-producer",
		stageRevalidator: "unsupported-stage-revalidator",
		compositeLowerer: "unsupported-composite-lowerer",
	}
	issues := compareStaticStageEditionObservation(catalog, observation)
	want := []TrustedStageEditionCoordinate{
		TrustedStageSchemaEdition,
		TrustedStageCompilerEdition,
		TrustedBaseCompilerSchemaEdition,
		TrustedStageProducerEdition,
		TrustedStageRevalidatorEdition,
		TrustedCompositeLowererEdition,
	}
	if len(issues) != len(want) {
		t.Fatalf("issue count = %d, want %d", len(issues), len(want))
	}
	for index, coordinate := range want {
		if issues[index].Coordinate() != coordinate {
			t.Fatalf(
				"issue[%d] coordinate = %q, want %q",
				index,
				issues[index].Coordinate(),
				coordinate,
			)
		}
		if issues[index].Expected() == "" || issues[index].Actual() == "" {
			t.Fatalf("issue[%d] lost expected or actual edition", index)
		}
		if issues[index].Repair() == "" {
			t.Fatalf("issue[%d] lost its rebuild return condition", index)
		}
	}
}

func TestPackageOwnedCatalogsKeepGenesisV2AndRouteFreshBaseV3Exactly(t *testing.T) {
	historicalGenesis := CurrentTrustedStageEditionCatalog()
	freshGenesis := currentBaseV3GenesisTrustedStageEditionCatalog()
	transition := currentTransitionTrustedStageEditionCatalog()
	if historicalGenesis.baseCompiler != typeenv.BaseTypeEnvCompilerSchemaV2 {
		t.Fatalf("historical Genesis base compiler = %q", historicalGenesis.baseCompiler)
	}
	if freshGenesis.baseCompiler != typeenv.BaseTypeEnvCompilerSchemaV3 ||
		transition.baseCompiler != typeenv.BaseTypeEnvCompilerSchemaV3 {
		t.Fatalf(
			"fresh catalog base compilers = %q/%q, want compiler-v3",
			freshGenesis.baseCompiler,
			transition.baseCompiler,
		)
	}
	if historicalGenesis.digest == freshGenesis.digest ||
		freshGenesis.digest == transition.digest {
		t.Fatal("distinct Genesis/Transition edition policies share one digest")
	}
	assertTrustedStageEditionCatalogWellFormed(t, historicalGenesis)
	assertTrustedStageEditionCatalogWellFormed(t, freshGenesis)
	assertTrustedStageEditionCatalogWellFormed(t, transition)
}

func assertTrustedStageEditionCatalogWellFormed(
	t *testing.T,
	catalog TrustedStageEditionCatalog,
) {
	t.Helper()
	canonical := trustedStageEditionCatalogCanonical(catalog)
	if !bytes.Equal(catalog.canonical, canonical) {
		t.Fatal("package-owned catalog canonical bytes do not match its coordinates")
	}
	if catalog.digest != digestTrustedStageEditionCatalog(canonical) {
		t.Fatal("package-owned catalog digest does not match its canonical bytes")
	}
}

func TestStaticEditionObservationDigestBindsEveryCoordinateAndTargetX(t *testing.T) {
	base := staticStageEditionObservation{
		stageSchema:      "stage-schema",
		stageCompiler:    "stage-compiler",
		baseCompiler:     "base-compiler",
		stageProducer:    "stage-producer",
		stageRevalidator: "stage-revalidator",
		compositeLowerer: "composite-lowerer",
	}
	baseCanonical := staticStageEditionObservationCanonical(base)
	baseDigest := digestTrustedStageEditionObservation(baseCanonical)
	variants := []staticStageEditionObservation{
		{
			stageSchema:      "other-stage-schema",
			stageCompiler:    base.stageCompiler,
			baseCompiler:     base.baseCompiler,
			stageProducer:    base.stageProducer,
			stageRevalidator: base.stageRevalidator,
			compositeLowerer: base.compositeLowerer,
		},
		{
			stageSchema:      base.stageSchema,
			stageCompiler:    "other-stage-compiler",
			baseCompiler:     base.baseCompiler,
			stageProducer:    base.stageProducer,
			stageRevalidator: base.stageRevalidator,
			compositeLowerer: base.compositeLowerer,
		},
		{
			stageSchema:      base.stageSchema,
			stageCompiler:    base.stageCompiler,
			baseCompiler:     "other-base-compiler",
			stageProducer:    base.stageProducer,
			stageRevalidator: base.stageRevalidator,
			compositeLowerer: base.compositeLowerer,
		},
		{
			stageSchema:      base.stageSchema,
			stageCompiler:    base.stageCompiler,
			baseCompiler:     base.baseCompiler,
			stageProducer:    "other-stage-producer",
			stageRevalidator: base.stageRevalidator,
			compositeLowerer: base.compositeLowerer,
		},
		{
			stageSchema:      base.stageSchema,
			stageCompiler:    base.stageCompiler,
			baseCompiler:     base.baseCompiler,
			stageProducer:    base.stageProducer,
			stageRevalidator: "other-stage-revalidator",
			compositeLowerer: base.compositeLowerer,
		},
		{
			stageSchema:      base.stageSchema,
			stageCompiler:    base.stageCompiler,
			baseCompiler:     base.baseCompiler,
			stageProducer:    base.stageProducer,
			stageRevalidator: base.stageRevalidator,
			compositeLowerer: "other-composite-lowerer",
		},
	}
	for index, variant := range variants {
		canonical := staticStageEditionObservationCanonical(variant)
		digest := digestTrustedStageEditionObservation(canonical)
		if digest == baseDigest {
			t.Fatalf("variant[%d] did not change observation digest", index)
		}
	}
	runtime, err := projecttypeenv.ParseRuntimeEvaluationBasisRef(
		"runtime-evaluation-basis:sha256:" + strings.Repeat("a", 64),
	)
	if err != nil {
		t.Fatalf("ParseRuntimeEvaluationBasisRef(): %v", err)
	}
	runtimeVariant := base
	runtimeVariant.targetRuntime = runtime
	runtimeCanonical := staticStageEditionObservationCanonical(runtimeVariant)
	runtimeDigest := digestTrustedStageEditionObservation(runtimeCanonical)
	if runtimeDigest == baseDigest {
		t.Fatal("target X did not change observation digest")
	}
}
