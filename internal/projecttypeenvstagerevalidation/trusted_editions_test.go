package projecttypeenvstagerevalidation_test

import (
	"bytes"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/projecttypeenvstagerevalidation"
)

func TestCurrentTrustedStageEditionsMatchStaticCoordinatesAndRequireExactTargetRuntime(
	t *testing.T,
) {
	project := testProject(t, "qnt_a11ce001")
	target := targetFixtures(t).alpha
	stage := genesisStage(t, project, target)

	result := projecttypeenvstagerevalidation.CompareCurrentTrustedStageEditions(
		stage,
		target.snapshot,
	)
	matched, ok := result.(projecttypeenvstagerevalidation.StaticTrustedStageEditionsMatched)
	if !ok {
		t.Fatalf("comparison result = %T, want StaticTrustedStageEditionsMatched", result)
	}
	if matched.ObservationDigest().String() == "" {
		t.Fatal("matched result has no exact observation digest")
	}
	requirement := matched.RuntimeRegistryRequirement()
	wantRuntime := target.snapshot.Record().RuntimeEvaluationBasisRef()
	if requirement.TargetRuntimeBasis() != wantRuntime {
		t.Fatal("runtime requirement does not bind exact target X")
	}
	if requirement.CatalogDigest() != matched.CatalogDigest() {
		t.Fatal("runtime requirement does not bind static catalog digest")
	}
}

func TestCurrentTrustedStageEditionsRejectUnsupportedStageEditionsDeterministically(
	t *testing.T,
) {
	project := testProject(t, "qnt_a11ce002")
	target := targetFixtures(t).alpha
	stage := genesisStage(t, project, target)
	unsupported := stageWithUnsupportedEditions(t, stage)

	first := projecttypeenvstagerevalidation.CompareCurrentTrustedStageEditions(
		unsupported,
		target.snapshot,
	)
	second := projecttypeenvstagerevalidation.CompareCurrentTrustedStageEditions(
		unsupported,
		target.snapshot,
	)
	firstUnsupported, ok := first.(projecttypeenvstagerevalidation.UnsupportedTrustedStageEditions)
	if !ok {
		t.Fatalf("comparison result = %T, want UnsupportedTrustedStageEditions", first)
	}
	secondUnsupported, ok := second.(projecttypeenvstagerevalidation.UnsupportedTrustedStageEditions)
	if !ok {
		t.Fatalf("second comparison result = %T, want UnsupportedTrustedStageEditions", second)
	}
	firstIssues := firstUnsupported.Issues()
	secondIssues := secondUnsupported.Issues()
	if len(firstIssues) != 3 || len(secondIssues) != 3 {
		t.Fatalf("unsupported issue counts = %d/%d, want 3/3", len(firstIssues), len(secondIssues))
	}
	wantCoordinates := []projecttypeenvstagerevalidation.TrustedStageEditionCoordinate{
		projecttypeenvstagerevalidation.TrustedStageCompilerEdition,
		projecttypeenvstagerevalidation.TrustedStageProducerEdition,
		projecttypeenvstagerevalidation.TrustedStageRevalidatorEdition,
	}
	for index, want := range wantCoordinates {
		if firstIssues[index].Coordinate() != want ||
			secondIssues[index].Coordinate() != want {
			t.Fatalf(
				"issue[%d] coordinates = %q/%q, want %q",
				index,
				firstIssues[index].Coordinate(),
				secondIssues[index].Coordinate(),
				want,
			)
		}
		if firstIssues[index].Expected() != secondIssues[index].Expected() ||
			firstIssues[index].Actual() != secondIssues[index].Actual() {
			t.Fatalf("issue[%d] is not deterministic", index)
		}
	}
	if firstUnsupported.CatalogDigest() != secondUnsupported.CatalogDigest() ||
		firstUnsupported.ObservationDigest() != secondUnsupported.ObservationDigest() {
		t.Fatal("unsupported comparison identities are not deterministic")
	}
}

func TestCurrentTrustedStageEditionsRejectInvalidStrongInputs(t *testing.T) {
	result := projecttypeenvstagerevalidation.CompareCurrentTrustedStageEditions(
		projecttypeenvselection.ProjectTypeEnvStage{},
		targetClosureFixture{}.snapshot,
	)
	invalid, ok := result.(projecttypeenvstagerevalidation.InvalidTrustedStageEditionInput)
	if !ok {
		t.Fatalf("comparison result = %T, want InvalidTrustedStageEditionInput", result)
	}
	issues := invalid.Issues()
	if len(issues) != 2 {
		t.Fatalf("invalid issue count = %d, want 2", len(issues))
	}
	if issues[0].Code() != projecttypeenvstagerevalidation.TrustedEditionStageInvalid ||
		issues[1].Code() != projecttypeenvstagerevalidation.TrustedEditionExecutableSnapshotInvalid {
		t.Fatalf("invalid issues are not in canonical code order: %#v", issues)
	}
}

func TestCurrentTrustedStageEditionsRejectCrossTargetRuntimeBasis(t *testing.T) {
	project := testProject(t, "qnt_a11ce003")
	targets := targetFixtures(t)
	stage := genesisStage(t, project, targets.beta)
	result := projecttypeenvstagerevalidation.CompareCurrentTrustedStageEditions(
		stage,
		targets.alpha.snapshot,
	)
	invalid, ok := result.(projecttypeenvstagerevalidation.InvalidTrustedStageEditionInput)
	if !ok {
		t.Fatalf("comparison result = %T, want InvalidTrustedStageEditionInput", result)
	}
	issues := invalid.Issues()
	if len(issues) != 1 {
		t.Fatalf("invalid issue count = %d, want 1", len(issues))
	}
	if issues[0].Code() !=
		projecttypeenvstagerevalidation.TrustedEditionTargetRuntimeBasisMismatch {
		t.Fatalf("issue code = %q, want target runtime-basis mismatch", issues[0].Code())
	}
}

func TestTrustedStageEditionCatalogIsPackageOwnedAndReturnsByteCopies(t *testing.T) {
	first := projecttypeenvstagerevalidation.CurrentTrustedStageEditionCatalog()
	second := projecttypeenvstagerevalidation.CurrentTrustedStageEditionCatalog()
	if first.Digest() != second.Digest() {
		t.Fatal("current catalog digest is not deterministic")
	}
	firstBytes := first.CanonicalBytes()
	secondBytes := second.CanonicalBytes()
	if len(firstBytes) == 0 || !bytes.Equal(firstBytes, secondBytes) {
		t.Fatal("current catalog canonical bytes are empty or unstable")
	}
	firstBytes[0] ^= 0xff
	if bytes.Equal(firstBytes, first.CanonicalBytes()) {
		t.Fatal("catalog canonical bytes leaked mutable backing storage")
	}
	if first.StageCompilerEdition().String() !=
		projecttypeenvselection.ProjectTypeEnvStageCompilerEditionV4 {
		t.Fatal("catalog does not use the production Stage compiler edition")
	}
	if first.StageProducerEdition().String() !=
		projecttypeenvselection.ProjectTypeEnvStageProducerEditionV4 {
		t.Fatal("catalog does not use the production Stage producer edition")
	}
	if first.StageRevalidatorEdition().String() !=
		projecttypeenvselection.ProjectTypeEnvStageRevalidatorEditionV4 {
		t.Fatal("catalog does not use the production Stage revalidator edition")
	}
	if first.BaseCompilerSchemaEdition() != typeenv.BaseTypeEnvCompilerSchemaV2 {
		t.Fatal("catalog does not use the production Base-TypeEnv compiler schema")
	}
	if first.CompositeLowererEdition() !=
		projecttypeenv.ProjectTypeEnvCompositeLowererSchemaV1 {
		t.Fatal("catalog does not use the production composite lowerer edition")
	}
}

func stageWithUnsupportedEditions(
	t *testing.T,
	stage projecttypeenvselection.ProjectTypeEnvStage,
) projecttypeenvselection.ProjectTypeEnvStage {
	t.Helper()
	canonical := stage.CanonicalBytes()
	replacements := [][2][]byte{
		{
			[]byte(projecttypeenvselection.ProjectTypeEnvStageCompilerEditionV4),
			[]byte("project-typeenv-stage-compiler/x4"),
		},
		{
			[]byte(projecttypeenvselection.ProjectTypeEnvStageProducerEditionV4),
			[]byte("project-typeenv-stage-producer/x4"),
		},
		{
			[]byte(projecttypeenvselection.ProjectTypeEnvStageRevalidatorEditionV4),
			[]byte("project-typeenv-stage-revalidator/x4"),
		},
	}
	for _, replacement := range replacements {
		if !bytes.Contains(canonical, replacement[0]) {
			t.Fatalf("Stage canonical bytes do not contain %q", replacement[0])
		}
		canonical = bytes.ReplaceAll(canonical, replacement[0], replacement[1])
	}
	decoded, err := projecttypeenvselection.DecodeProjectTypeEnvStage(canonical)
	if err != nil {
		t.Fatalf("DecodeProjectTypeEnvStage(unsupported editions): %v", err)
	}
	return decoded
}
