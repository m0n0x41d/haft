package projecttypeenvstagerevalidation

import (
	"bytes"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
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

func TestPackageOwnedCatalogsAddBaseV5WithoutRelabelingBaseV4(t *testing.T) {
	baseV4Genesis := currentBaseV4GenesisTrustedStageEditionCatalog()
	baseV5Genesis := currentBaseV5GenesisTrustedStageEditionCatalog()
	baseV4Transition := currentBaseV4TransitionTrustedStageEditionCatalog()
	baseV5Transition := currentBaseV5TransitionTrustedStageEditionCatalog()

	testCases := []struct {
		name    string
		catalog TrustedStageEditionCatalog
		want    trustedStageEditionCatalogCoordinates
	}{
		{
			name:    "historical Base-v4 Genesis",
			catalog: baseV4Genesis,
			want: trustedStageEditionCatalogCoordinates{
				stageSchema:      projecttypeenvselection.ProjectTypeEnvStageSchemaEditionV4,
				stageCompiler:    projecttypeenvselection.StageCompilerEditionV4(),
				baseCompiler:     typeenv.BaseTypeEnvCompilerSchemaV4,
				stageProducer:    projecttypeenvselection.StageProducerEditionV4(),
				stageRevalidator: projecttypeenvselection.StageRevalidatorEditionV4(),
				compositeLowerer: projecttypeenv.ProjectTypeEnvCompositeLowererSchemaV2,
			},
		},
		{
			name:    "current Base-v5 Genesis",
			catalog: baseV5Genesis,
			want: trustedStageEditionCatalogCoordinates{
				stageSchema:      projecttypeenvselection.ProjectTypeEnvStageSchemaEditionV4,
				stageCompiler:    projecttypeenvselection.StageCompilerEditionV4(),
				baseCompiler:     typeenv.BaseTypeEnvCompilerSchemaV5,
				stageProducer:    projecttypeenvselection.StageProducerEditionV4(),
				stageRevalidator: projecttypeenvselection.StageRevalidatorEditionV4(),
				compositeLowerer: projecttypeenv.ProjectTypeEnvCompositeLowererSchemaV2,
			},
		},
		{
			name:    "historical Base-v4 Transition",
			catalog: baseV4Transition,
			want: trustedStageEditionCatalogCoordinates{
				stageSchema:      projecttypeenvselection.ProjectTypeEnvStageSchemaEditionV5,
				stageCompiler:    projecttypeenvselection.StageCompilerEditionV5(),
				baseCompiler:     typeenv.BaseTypeEnvCompilerSchemaV4,
				stageProducer:    projecttypeenvselection.StageProducerEditionV5(),
				stageRevalidator: projecttypeenvselection.StageRevalidatorEditionV5(),
				compositeLowerer: projecttypeenv.ProjectTypeEnvCompositeLowererSchemaV2,
			},
		},
		{
			name:    "current Base-v5 Transition",
			catalog: baseV5Transition,
			want: trustedStageEditionCatalogCoordinates{
				stageSchema:      projecttypeenvselection.ProjectTypeEnvStageSchemaEditionV5,
				stageCompiler:    projecttypeenvselection.StageCompilerEditionV5(),
				baseCompiler:     typeenv.BaseTypeEnvCompilerSchemaV5,
				stageProducer:    projecttypeenvselection.StageProducerEditionV5(),
				stageRevalidator: projecttypeenvselection.StageRevalidatorEditionV5(),
				compositeLowerer: projecttypeenv.ProjectTypeEnvCompositeLowererSchemaV2,
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got := observeTrustedStageEditionCatalogCoordinates(testCase.catalog)
			if got != testCase.want {
				t.Fatalf("catalog coordinates = %#v, want %#v", got, testCase.want)
			}
			assertTrustedStageEditionCatalogWellFormed(t, testCase.catalog)
		})
	}
	if baseV4Genesis.digest == baseV5Genesis.digest ||
		baseV4Transition.digest == baseV5Transition.digest {
		t.Fatal("Base-v4 and Base-v5 catalog identities collapsed")
	}
}

func TestTrustedStageEditionCatalogRoutingSelectsBaseV5AndKeepsBaseV4Exact(
	t *testing.T,
) {
	genesis := projecttypeenvselection.NewGenesisStagePredecessor()
	transition := projecttypeenvselection.TransitionStagePredecessor{}

	testCases := []struct {
		name         string
		predecessor  projecttypeenvselection.ProjectTypeEnvStagePredecessor
		baseCompiler string
		want         TrustedStageEditionCatalogDigest
	}{
		{
			name:         "Base-v4 Genesis",
			predecessor:  genesis,
			baseCompiler: typeenv.BaseTypeEnvCompilerSchemaV4,
			want:         currentBaseV4GenesisTrustedStageEditionCatalog().digest,
		},
		{
			name:         "Base-v5 Genesis",
			predecessor:  genesis,
			baseCompiler: typeenv.BaseTypeEnvCompilerSchemaV5,
			want:         currentBaseV5GenesisTrustedStageEditionCatalog().digest,
		},
		{
			name:         "Base-v4 Transition",
			predecessor:  transition,
			baseCompiler: typeenv.BaseTypeEnvCompilerSchemaV4,
			want:         currentBaseV4TransitionTrustedStageEditionCatalog().digest,
		},
		{
			name:         "Base-v5 Transition",
			predecessor:  transition,
			baseCompiler: typeenv.BaseTypeEnvCompilerSchemaV5,
			want:         currentBaseV5TransitionTrustedStageEditionCatalog().digest,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			observation := staticStageEditionObservation{
				baseCompiler: testCase.baseCompiler,
			}
			got := trustedStageEditionCatalogForPredecessor(
				testCase.predecessor,
				observation,
			)
			if got.digest != testCase.want {
				t.Fatalf("routed catalog = %q, want %q", got.digest.String(), testCase.want.String())
			}
		})
	}
}

type trustedStageEditionCatalogCoordinates struct {
	stageSchema      string
	stageCompiler    projecttypeenvselection.StageCompilerEdition
	baseCompiler     string
	stageProducer    projecttypeenvselection.StageProducerEdition
	stageRevalidator projecttypeenvselection.StageRevalidatorEdition
	compositeLowerer string
}

func observeTrustedStageEditionCatalogCoordinates(
	catalog TrustedStageEditionCatalog,
) trustedStageEditionCatalogCoordinates {
	return trustedStageEditionCatalogCoordinates{
		stageSchema:      catalog.stageSchema,
		stageCompiler:    catalog.stageCompiler,
		baseCompiler:     catalog.baseCompiler,
		stageProducer:    catalog.stageProducer,
		stageRevalidator: catalog.stageRevalidator,
		compositeLowerer: catalog.compositeLowerer,
	}
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
