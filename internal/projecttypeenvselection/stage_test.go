package projecttypeenvselection

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/fpf/typeenvsql"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/projecttypeenvassertionreport"
	"github.com/m0n0x41d/haft/internal/projecttypeenvcompatibility"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilebasis"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilefit"
	"github.com/m0n0x41d/haft/internal/projecttypeenvtransitioncompatibility"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	_ "modernc.org/sqlite"
)

var (
	stageExecutableFixtureOnce sync.Once
	stageExecutableFixture     stageExecutableTypeEnvFixture
	stageExecutableFixtureErr  error
)

type stageExecutableTypeEnvFixture struct {
	verification projecttypeenv.ProjectTypeEnvCompositeVerification
	snapshot     projecttypeenv.ProjectTypeEnvExecutableSnapshot
}

func TestProjectTypeEnvStageGenesisCanonicalRoundTrip(t *testing.T) {
	input := stageGenesisInput(t, 17)
	stage := sealStageFixture(t, input)
	if err := stage.Verify(); err != nil {
		t.Fatalf("Verify(): %v", err)
	}
	decoded, err := VerifyProjectTypeEnvStage(stage.Ref(), stage.CanonicalBytes())
	if err != nil {
		t.Fatalf("VerifyProjectTypeEnvStage(): %v", err)
	}
	if decoded.Project() != input.Project ||
		decoded.Base() != input.Base ||
		decoded.RuntimeBasis() != input.RuntimeBasis ||
		decoded.VerifiedComposite() != input.Composite ||
		decoded.GraphSnapshotBasis() != input.GraphSnapshotBasisRef ||
		decoded.GraphRevision() != input.GraphRevision {
		t.Fatalf("decoded Stage lost exact coordinates")
	}
	if decoded.GraphSnapshotBasisDigest() != input.GraphSnapshotBasisDigest ||
		decoded.ProfileLedgerRevision() != input.ProfileLedgerRevision ||
		decoded.ProfileLedgerDigest() != input.ProfileLedgerDigest {
		t.Fatalf("decoded Stage lost mutable-basis revision or digest")
	}
	if decoded.CompositeVerificationRef() != input.VerifiedComposite.Ref() ||
		decoded.CompositeVerificationDigest() != input.VerifiedComposite.Digest() ||
		decoded.CompositeVerificationRef().Digest() != decoded.CompositeVerificationDigest() {
		t.Fatalf("decoded Stage lost exact composite-verification record identity")
	}
	if decoded.CompatibilityRef().Digest() != decoded.CompatibilityDigest() ||
		decoded.ExistingAssertionRevalidationRef().Digest() != decoded.ExistingAssertionRevalidationDigest() ||
		decoded.ProfileFitRef().Digest() != decoded.ProfileFitDigest() {
		t.Fatalf("decoded Stage projection ref/digest pairs diverged")
	}
	report := decoded.ExistingAssertionRevalidation()
	if report.Digest() != input.ExistingAssertionRevalidation.Digest() ||
		report.TargetTypeEnv() != decoded.VerifiedComposite() ||
		report.GraphSnapshotRef().String() != decoded.GraphSnapshotBasis().String() ||
		report.GraphRevision() != decoded.GraphRevision() ||
		report.RuntimeBasisRef() != decoded.RuntimeBasis() ||
		!bytes.Equal(
			report.CanonicalBytes(),
			input.ExistingAssertionRevalidation.CanonicalBytes(),
		) {
		t.Fatalf("decoded Stage lost the exact full assertion-revalidation report")
	}
	decodedReport, err := projecttypeenvassertionreport.DecodeCanonicalReport(
		report.CanonicalBytes(),
	)
	if err != nil {
		t.Fatalf("DecodeCanonicalReport(): %v", err)
	}
	if decodedReport.Digest() != report.Digest() ||
		!bytes.Equal(decodedReport.CanonicalBytes(), report.CanonicalBytes()) {
		t.Fatalf("full assertion report changed on strict round-trip")
	}
	reorderedReport := mustStageAssertionReport(
		t,
		stageAssertionReportInput{
			target:            input.Composite,
			graphSnapshot:     input.GraphSnapshotBasis,
			graphRevision:     input.GraphRevision,
			runtimeBasis:      input.RuntimeBasis,
			runtimeCoordinate: report.RuntimeCoordinateDigest(),
			includeOutcomes:   true,
			reverseOutcomes:   true,
		},
	)
	if reorderedReport.Digest() != report.Digest() ||
		!bytes.Equal(reorderedReport.CanonicalBytes(), report.CanonicalBytes()) {
		t.Fatalf("assertion outcome permutation changed canonical report identity")
	}
	trailingReport := append(report.CanonicalBytes(), 0x00)
	if _, err := projecttypeenvassertionreport.DecodeCanonicalReport(
		trailingReport,
	); err == nil {
		t.Fatalf("assertion-report codec accepted trailing bytes")
	}
	profile := decoded.ProfileCompatibility()
	executable := mustStageExecutableTypeEnvFixture(t)
	if profile.Digest() != input.ProfileCompatibility.Digest() ||
		profile.FitRef() != decoded.ProfileFitRef() ||
		profile.TargetTypeEnvRef() != decoded.VerifiedComposite() ||
		profile.TargetSnapshotDigest() != executable.snapshot.Digest() ||
		!bytes.Equal(
			profile.CanonicalBytes(),
			input.ProfileCompatibility.CanonicalBytes(),
		) {
		t.Fatalf("decoded Stage lost the exact full profile-fit assessment")
	}
	if decoded.SchemaEdition() != ProjectTypeEnvStageSchemaEditionV4 ||
		decoded.ProducerEdition() != StageProducerEditionV4() {
		t.Fatalf("decoded Stage lost schema or producer edition")
	}
	reader := stageReader{value: decoded.CanonicalBytes()}
	domain, err := reader.readString("domain")
	if err != nil || domain != projectTypeEnvStageDomain {
		t.Fatalf("current Stage domain = %q, %v; want v4", domain, err)
	}
	if _, ok := decoded.HistoricalProvenance(); ok {
		t.Fatal("current Stage exposed historical caller-supplied provenance")
	}
	if bytes.Contains(decoded.CanonicalBytes(), []byte(projectTypeEnvStageProvenancePrefix)) {
		t.Fatal("current Stage encoded historical caller-supplied provenance")
	}
	_, ok := decoded.Predecessor().(GenesisStagePredecessor)
	if !ok {
		t.Fatalf("decoded predecessor = %T; want Genesis", decoded.Predecessor())
	}
	if _, ok := LegacyGenesisNoPriorHeadProof(decoded.Predecessor()); ok {
		t.Fatal("current Genesis exposed a legacy no-prior-head proof")
	}
	if bytes.Contains(
		decoded.CanonicalBytes(),
		[]byte(noPriorHeadProofRefPrefix),
	) {
		t.Fatal("current Genesis Stage leaked a no-prior-head proof identity")
	}
	initial, ok := decoded.Compatibility().(InitialStageCompatibility)
	if !ok {
		t.Fatalf("decoded compatibility = %T; want initial", decoded.Compatibility())
	}
	if initial.Target() != decoded.VerifiedComposite() {
		t.Fatalf("decoded initial compatibility lost exact target C")
	}
	if _, ok := profile.(projecttypeenvprofilefit.Underdetermined); !ok {
		t.Fatalf("decoded profile = %T; want underdetermined", profile)
	}
	if !bytes.Equal(stage.CanonicalBytes(), decoded.CanonicalBytes()) {
		t.Fatalf("canonical roundtrip changed bytes")
	}
}

func TestProjectTypeEnvStageV2GenesisDecodesReadOnly(t *testing.T) {
	proof := mustNoPriorProofRef(t, "7")
	stage := mustLoadReconstructedStageV2Genesis(t)

	if err := stage.Verify(); err != nil {
		t.Fatalf("Verify legacy Stage: %v", err)
	}
	if stage.SchemaEdition() != ProjectTypeEnvStageSchemaEditionV2 {
		t.Fatalf("legacy schema = %q; want v2", stage.SchemaEdition())
	}
	predecessor, ok := stage.Predecessor().(legacyGenesisStagePredecessor)
	if !ok {
		t.Fatalf("legacy predecessor = %T; want private legacy Genesis", stage.Predecessor())
	}
	if predecessor.noPriorHeadProof != proof {
		t.Fatal("legacy Stage lost its historical proof reference")
	}
	exposedProof, ok := LegacyGenesisNoPriorHeadProof(stage.Predecessor())
	if !ok || exposedProof != proof {
		t.Fatal("legacy proof inspection lost the historical coordinate")
	}
	if provenance, ok := stage.HistoricalProvenance(); !ok ||
		provenance != mustStageProvenanceRef(t, "b") {
		t.Fatal("legacy v2 Stage lost its historical provenance coordinate")
	}

	input := stageGenesisInput(t, 17)
	input.Predecessor = stage.Predecessor()
	if _, err := SealProjectTypeEnvStage(input); err == nil ||
		!strings.Contains(err.Error(), "Stage v4 predecessor posture is invalid") {
		t.Fatalf("new-write admission of legacy predecessor error = %v", err)
	}
}

func TestProjectTypeEnvStageV3DecodesForHistoricalInspection(t *testing.T) {
	stage := mustLoadFrozenHistoricalStageV3Genesis(t)

	if err := stage.Verify(); err != nil {
		t.Fatalf("Verify historical v3 Stage: %v", err)
	}
	if stage.SchemaEdition() != ProjectTypeEnvStageSchemaEditionV3 {
		t.Fatalf("historical schema = %q; want v3", stage.SchemaEdition())
	}
	provenance, ok := stage.HistoricalProvenance()
	if !ok || provenance != mustHistoricalStageV3ProvenanceRef(t) {
		t.Fatal("historical v3 Stage lost its uninterpreted provenance coordinate")
	}
	decoded, err := DecodeProjectTypeEnvStage(stage.CanonicalBytes())
	if err != nil {
		t.Fatalf("DecodeProjectTypeEnvStage(v3): %v", err)
	}
	if decoded.Ref() != stage.Ref() ||
		!bytes.Equal(decoded.CanonicalBytes(), stage.CanonicalBytes()) {
		t.Fatal("historical v3 Stage changed during exact decode")
	}
}

func TestProjectTypeEnvStageGenesisSupportsAnEmptyGraphWithoutSelectedTypeEnv(t *testing.T) {
	input := stageGenesisInput(t, 1)
	emptySnapshot, err := SealProjectGraphSnapshotBasis(ProjectGraphSnapshotBasisInput{
		Project:       input.Project,
		GraphRevision: typedmemory.NewGraphRevision(0),
		Closure:       EmptyProjectGraphClosure{},
	})
	if err != nil {
		t.Fatalf("SealProjectGraphSnapshotBasis(empty): %v", err)
	}
	input.GraphSnapshotBasis = emptySnapshot
	input.GraphSnapshotBasisRef = emptySnapshot.Ref()
	input.GraphSnapshotBasisDigest = emptySnapshot.Ref().Digest()
	input.GraphRevision = typedmemory.NewGraphRevision(0)
	input.ExistingAssertionRevalidation = mustStageAssertionReport(
		t,
		stageAssertionReportInput{
			target:            input.Composite,
			graphSnapshot:     emptySnapshot,
			graphRevision:     input.GraphRevision,
			runtimeBasis:      input.RuntimeBasis,
			runtimeCoordinate: testDigest(t, "a"),
		},
	)
	stage := sealStageFixture(t, input)
	if stage.GraphRevision().Value() != 0 {
		t.Fatalf("Genesis graph revision = %d; want zero", stage.GraphRevision().Value())
	}
	if _, ok := stage.Predecessor().(GenesisStagePredecessor); !ok {
		t.Fatalf("empty-graph Stage predecessor = %T; want Genesis", stage.Predecessor())
	}
}

func TestProjectTypeEnvStageTransitionBindsExactPriorHeadAndCompatibility(t *testing.T) {
	input := stageTransitionInput(t, 29)
	stage := sealStageFixture(t, input)
	if stage.SchemaEdition() != ProjectTypeEnvStageSchemaEditionV5 ||
		stage.CompilerEdition() != StageCompilerEditionV5() ||
		stage.ProducerEdition() != StageProducerEditionV5() ||
		stage.RevalidatorEdition() != StageRevalidatorEditionV5() {
		t.Fatal("Transition Stage did not use the v5 edition family")
	}
	predecessor, ok := stage.Predecessor().(TransitionStagePredecessor)
	if !ok {
		t.Fatalf("predecessor = %T; want Transition", stage.Predecessor())
	}
	if predecessor.HeadRevision().Value() != 4 {
		t.Fatalf("head revision = %d", predecessor.HeadRevision().Value())
	}
	compatibility, ok := stage.Compatibility().(ComparedStageCompatibility)
	if !ok {
		t.Fatalf("compatibility = %T; want compared", stage.Compatibility())
	}
	if compatibility.Base() != predecessor.SelectedComposite() {
		t.Fatalf("compatibility base does not bind exact prior selected C")
	}
	if compatibility.Target() != stage.VerifiedComposite() {
		t.Fatalf("compatibility target does not bind exact selected C")
	}
	diff := compatibility.Diff()
	if diff.Base() != predecessor.SelectedComposite() ||
		diff.Target() != stage.VerifiedComposite() {
		t.Fatalf("compatibility diff lost exact base or target")
	}
	if stage.CompatibilityDigest() != diff.Digest() ||
		stage.CompatibilityRef().Digest() != diff.Digest() {
		t.Fatalf("Stage compatibility identity does not bind exact canonical diff")
	}
	if len(diff.Changes()) != 2 {
		t.Fatalf("compatibility changes = %d; want 2", len(diff.Changes()))
	}
	projectionProfiles, exists := stage.TransitionProjectionProfileCompatibility()
	if !exists {
		t.Fatal("Transition Stage lost projection-profile compatibility")
	}
	if projectionProfiles.Ref() != input.TransitionProjectionProfileCompatibility.Ref() ||
		projectionProfiles.SuccessorDiff().Base() != predecessor.SelectedComposite() ||
		projectionProfiles.SuccessorDiff().Target() != stage.VerifiedComposite() {
		t.Fatal("Transition Stage projection-profile compatibility lost exact identity")
	}
	projectionProfilesRef, exists := stage.TransitionProjectionProfileCompatibilityRef()
	if !exists || projectionProfilesRef.Digest() != projectionProfiles.Digest() {
		t.Fatal("Transition Stage projection-profile compatibility ref diverged")
	}
	projectionProfilesDigest, exists := stage.TransitionProjectionProfileCompatibilityDigest()
	if !exists || projectionProfilesDigest != projectionProfiles.Digest() {
		t.Fatal("Transition Stage projection-profile compatibility digest diverged")
	}
}

func TestProjectTypeEnvStageTransitionV5RequiresProjectionProfileCompatibility(
	t *testing.T,
) {
	input := stageTransitionInput(t, 30)
	input.TransitionProjectionProfileCompatibility = projecttypeenvtransitioncompatibility.Set{}
	_, err := SealProjectTypeEnvStage(input)
	if err == nil || !strings.Contains(
		err.Error(),
		"requires projection-profile compatibility",
	) {
		t.Fatalf("Transition Stage without projection-profile compatibility: %v", err)
	}
}

func TestProjectTypeEnvStageTransitionV5RejectsTransitiveArtifactTampering(
	t *testing.T,
) {
	stage := sealStageFixture(t, stageTransitionInput(t, 31))
	artifact, exists := stage.TransitionProjectionProfileCompatibility()
	if !exists {
		t.Fatal("Transition Stage lost its transitive compatibility artifact")
	}
	artifactBytes := artifact.CanonicalBytes()
	tamperedArtifact := append([]byte(nil), artifactBytes...)
	tamperedArtifact[len(tamperedArtifact)-1] ^= 0x01
	canonical := bytes.Replace(
		stage.CanonicalBytes(),
		artifactBytes,
		tamperedArtifact,
		1,
	)
	if bytes.Equal(canonical, stage.CanonicalBytes()) {
		t.Fatal("Transition artifact tamper fixture did not change Stage bytes")
	}
	if _, err := DecodeProjectTypeEnvStage(canonical); err == nil {
		t.Fatal("Transition Stage accepted a changed transitive artifact under the old identity")
	}
}

func TestProjectTypeEnvStageRejectsPredecessorVariantConfusion(t *testing.T) {
	genesis := stageGenesisInput(t, 3)
	prior := testTypeEnvRef(t, "9")
	diff := mustStageCompatibilityDiff(t, prior, genesis.Composite, false)
	compared, err := NewComparedStageCompatibility(diff)
	if err != nil {
		t.Fatalf("NewComparedStageCompatibility(): %v", err)
	}
	genesis.Compatibility = compared
	if _, err := SealProjectTypeEnvStage(genesis); err == nil ||
		!strings.Contains(err.Error(), "genesis Stage requires initial") {
		t.Fatalf("Genesis accepted compared compatibility: %v", err)
	}

	transition := stageTransitionInput(t, 4)
	transition.Compatibility = mustInitialStageCompatibility(t, transition.Composite)
	if _, err := SealProjectTypeEnvStage(transition); err == nil ||
		!strings.Contains(err.Error(), "transition Stage requires compared") {
		t.Fatalf("Transition accepted initial compatibility: %v", err)
	}

	stage := sealStageFixture(t, stageGenesisInput(t, 5))
	forged := replaceCanonicalText(t, stage.CanonicalBytes(), "genesis", "bogus!!")
	if _, err := DecodeProjectTypeEnvStage(forged); err == nil ||
		!strings.Contains(err.Error(), "predecessor") {
		t.Fatalf("decode accepted unknown predecessor variant: %v", err)
	}
}

func TestProjectTypeEnvStageStrongCoordinatesCannotAlias(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	if _, err := ParseNoPriorHeadProofRef("project-typeenv-head:" + digest); err == nil {
		t.Fatalf("absence-proof parser accepted head ref")
	}
	if _, err := ParseProjectTypeEnvHeadRef("project-typeenv-no-prior-head-proof:" + digest); err == nil {
		t.Fatalf("head parser accepted absence-proof ref")
	}
	if _, err := ParseProjectTypeEnvStageRef("project-graph-snapshot-basis:" + digest); err == nil {
		t.Fatalf("Stage parser accepted graph-snapshot ref")
	}
	project := mustProjectID(t, "qnt_1234abcd")
	head, err := ProjectTypeEnvHeadRefForProject(project)
	if err != nil {
		t.Fatalf("ProjectTypeEnvHeadRefForProject(): %v", err)
	}
	if head.Project() != project || head.String() != "project-typeenv-head:qnt_1234abcd" {
		t.Fatalf("stable head ref = %q for project %q", head.String(), head.Project().String())
	}
	if _, err := ParseProjectTypeEnvHeadRef("project-typeenv-head:" + digest); err == nil {
		t.Fatalf("head parser accepted content digest instead of stable project slot")
	}
	if _, err := NewHeadRevision(0); err == nil {
		t.Fatalf("HeadRevision accepted zero sentinel")
	}
}

func TestProjectTypeEnvStageRejectsEveryExactBasisMismatch(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ProjectTypeEnvStageInput)
		want   string
	}{
		{
			name: "project",
			mutate: func(input *ProjectTypeEnvStageInput) {
				input.Project = mustProjectID(t, "qnt_89abcdef")
			},
			want: "snapshot project mismatch",
		},
		{
			name: "snapshot ref",
			mutate: func(input *ProjectTypeEnvStageInput) {
				input.GraphSnapshotBasisRef = mustSnapshotRef(t, "a")
			},
			want: "snapshot reference mismatch",
		},
		{
			name: "graph revision",
			mutate: func(input *ProjectTypeEnvStageInput) {
				input.GraphRevision = typedmemory.NewGraphRevision(44)
			},
			want: "snapshot revision mismatch",
		},
		{
			name: "snapshot digest",
			mutate: func(input *ProjectTypeEnvStageInput) {
				input.GraphSnapshotBasisDigest = testDigest(t, "e")
			},
			want: "snapshot digest mismatch",
		},
		{
			name: "B",
			mutate: func(input *ProjectTypeEnvStageInput) {
				input.Base = testTypeEnvRef(t, "d")
			},
			want: "Stage B does not match",
		},
		{
			name: "E",
			mutate: func(input *ProjectTypeEnvStageInput) {
				input.OrderedExtensions = append(
					input.OrderedExtensions,
					mustExtensionRef(t, "haft.local.unverified", "2"),
				)
			},
			want: "ordered E DAG does not match",
		},
		{
			name: "X",
			mutate: func(input *ProjectTypeEnvStageInput) {
				input.RuntimeBasis = mustRuntimeBasisRef(t, "e")
				input.ExistingAssertionRevalidation = mustRebindStageReportRuntime(
					t,
					input.ExistingAssertionRevalidation,
					input.RuntimeBasis,
				)
			},
			want: "Stage X does not match",
		},
		{
			name: "C",
			mutate: func(input *ProjectTypeEnvStageInput) {
				input.Composite = testTypeEnvRef(t, "f")
				input.Compatibility = mustInitialStageCompatibility(t, input.Composite)
				input.ExistingAssertionRevalidation = mustRebindStageReportTarget(
					t,
					input.ExistingAssertionRevalidation,
					input.Composite,
				)
				input.ProfileCompatibility = mustRebindStageAssessmentTarget(
					t,
					input.ProfileCompatibility,
					input.Composite,
				)
			},
			want: "Stage C does not match",
		},
		{
			name: "revalidation target",
			mutate: func(input *ProjectTypeEnvStageInput) {
				input.ExistingAssertionRevalidation = mustRebindStageReportTarget(
					t,
					input.ExistingAssertionRevalidation,
					testTypeEnvRef(t, "4"),
				)
			},
			want: "revalidation target does not match Stage C",
		},
		{
			name: "revalidation graph snapshot",
			mutate: func(input *ProjectTypeEnvStageInput) {
				other := testCommittedSnapshotBasis(t, input.GraphRevision.Value(), "5")
				input.ExistingAssertionRevalidation = mustStageAssertionReport(
					t,
					stageAssertionReportInput{
						target:            input.Composite,
						graphSnapshot:     other,
						graphRevision:     input.GraphRevision,
						runtimeBasis:      input.RuntimeBasis,
						runtimeCoordinate: testDigest(t, "a"),
						includeOutcomes:   true,
					},
				)
			},
			want: "revalidation graph snapshot does not match Stage graph basis",
		},
		{
			name: "revalidation revision",
			mutate: func(input *ProjectTypeEnvStageInput) {
				input.ExistingAssertionRevalidation = mustStageAssertionReport(
					t,
					stageAssertionReportInput{
						target:            input.Composite,
						graphSnapshot:     input.GraphSnapshotBasis,
						graphRevision:     typedmemory.NewGraphRevision(99),
						runtimeBasis:      input.RuntimeBasis,
						runtimeCoordinate: testDigest(t, "a"),
						includeOutcomes:   true,
					},
				)
			},
			want: "revalidation graph revision mismatch",
		},
		{
			name: "revalidation runtime",
			mutate: func(input *ProjectTypeEnvStageInput) {
				input.ExistingAssertionRevalidation = mustRebindStageReportRuntime(
					t,
					input.ExistingAssertionRevalidation,
					mustRuntimeBasisRef(t, "4"),
				)
			},
			want: "revalidation runtime basis does not match Stage X",
		},
		{
			name: "profile-fit target",
			mutate: func(input *ProjectTypeEnvStageInput) {
				input.ProfileCompatibility = mustRebindStageAssessmentTarget(
					t,
					input.ProfileCompatibility,
					testTypeEnvRef(t, "4"),
				)
			},
			want: "profile-fit target does not match Stage C",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := stageGenesisInput(t, 11)
			test.mutate(&input)
			_, err := SealProjectTypeEnvStage(input)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("SealProjectTypeEnvStage() error = %v; want %q", err, test.want)
			}
		})
	}

	transition := stageTransitionInput(t, 12)
	wrongBaseDiff := mustStageCompatibilityDiff(
		t,
		testTypeEnvRef(t, "8"),
		transition.Composite,
		false,
	)
	wrong, err := NewComparedStageCompatibility(wrongBaseDiff)
	if err != nil {
		t.Fatalf("NewComparedStageCompatibility(): %v", err)
	}
	transition.Compatibility = wrong
	if _, err := SealProjectTypeEnvStage(transition); err == nil ||
		!strings.Contains(err.Error(), "base does not match prior selected C") {
		t.Fatalf("Transition accepted wrong compatibility base: %v", err)
	}

	transition = stageTransitionInput(t, 12)
	prior := transition.Predecessor.(TransitionStagePredecessor)
	wrongTargetDiff := mustStageCompatibilityDiff(
		t,
		prior.SelectedComposite(),
		testTypeEnvRef(t, "8"),
		false,
	)
	wrong, err = NewComparedStageCompatibility(wrongTargetDiff)
	if err != nil {
		t.Fatalf("NewComparedStageCompatibility(wrong target): %v", err)
	}
	transition.Compatibility = wrong
	if _, err := SealProjectTypeEnvStage(transition); err == nil ||
		!strings.Contains(err.Error(), "target does not match Stage C") {
		t.Fatalf("Transition accepted wrong compatibility target: %v", err)
	}

	transition = stageTransitionInput(t, 12)
	prior = transition.Predecessor.(TransitionStagePredecessor)
	wrongProjectPredecessor, err := NewTransitionStagePredecessor(
		TransitionStagePredecessorInput{
			Project:           mustProjectID(t, "qnt_89abcdef"),
			Head:              mustHeadRef(t, mustProjectID(t, "qnt_89abcdef")),
			HeadRevision:      prior.HeadRevision(),
			SelectedComposite: prior.SelectedComposite(),
		},
	)
	if err != nil {
		t.Fatalf("NewTransitionStagePredecessor(wrong project): %v", err)
	}
	transition.Predecessor = wrongProjectPredecessor
	if _, err := SealProjectTypeEnvStage(transition); err == nil ||
		!strings.Contains(err.Error(), "prior-head project mismatch") {
		t.Fatalf("Transition accepted cross-project prior head: %v", err)
	}
}

func TestProjectTypeEnvStageRejectsDuplicateAndOverBoundEDAG(t *testing.T) {
	input := stageGenesisInput(t, 13)
	duplicate := mustExtensionRef(t, "haft.local.duplicate", "2")
	input.OrderedExtensions = []typedmemory.TypeEnvExtensionRef{duplicate, duplicate}
	if _, err := SealProjectTypeEnvStage(input); err == nil ||
		!strings.Contains(err.Error(), "duplicate extension") {
		t.Fatalf("Stage accepted duplicate E: %v", err)
	}

	overBound := make([]typedmemory.TypeEnvExtensionRef, 0, maximumStageExtensions+1)
	for index := 0; index <= maximumStageExtensions; index++ {
		id := "haft.local.bound" + strings.Repeat("x", index%7) + string(rune('a'+index%26))
		digit := "0123456789abcdef"[index%16 : index%16+1]
		ref := mustExtensionRef(t, id, digit)
		overBound = append(overBound, ref)
	}
	if _, err := normalizeOrderedExtensionRefs(overBound); err == nil ||
		!strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("ordered E DAG accepted over-bound input: %v", err)
	}
}

func TestProjectTypeEnvStagePreservesEDAGOrderButCanonicalizesSets(t *testing.T) {
	leftInput := stageTransitionInput(t, 21)
	rightInput := stageTransitionInput(t, 21)

	leftCompatibility := leftInput.Compatibility.(ComparedStageCompatibility)
	rightDiff := mustStageCompatibilityDiff(
		t,
		leftCompatibility.Base(),
		leftCompatibility.Target(),
		true,
	)
	rightCompatibility, err := NewComparedStageCompatibility(
		rightDiff,
	)
	if err != nil {
		t.Fatalf("NewComparedStageCompatibility(): %v", err)
	}
	if !bytes.Equal(
		leftCompatibility.Diff().CanonicalBytes(),
		rightCompatibility.Diff().CanonicalBytes(),
	) {
		t.Fatalf("builder permutation changed canonical compatibility diff")
	}
	if leftCompatibility.Diff().Digest() != rightCompatibility.Diff().Digest() {
		t.Fatalf("builder permutation changed compatibility diff identity")
	}
	rightInput.Compatibility = rightCompatibility
	left := sealStageFixture(t, leftInput)
	right := sealStageFixture(t, rightInput)
	if left.Ref() != right.Ref() {
		t.Fatalf("compatibility-set permutation changed Stage identity")
	}

	alpha := mustExtensionRef(t, "haft.local.alpha", "2")
	beta := mustExtensionRef(t, "haft.local.beta", "3")
	forward, err := normalizeOrderedExtensionRefs(
		[]typedmemory.TypeEnvExtensionRef{alpha, beta},
	)
	if err != nil {
		t.Fatalf("normalize forward E DAG: %v", err)
	}
	reverse, err := normalizeOrderedExtensionRefs(
		[]typedmemory.TypeEnvExtensionRef{beta, alpha},
	)
	if err != nil {
		t.Fatalf("normalize reverse E DAG: %v", err)
	}
	if orderedExtensionRefsEqual(forward, reverse) ||
		forward[0] != alpha || reverse[0] != beta {
		t.Fatalf("ordered E DAG was treated as an unordered set")
	}
}

func TestProjectTypeEnvStageIdentityTracksProfileLedgerBasis(t *testing.T) {
	baseInput := stageGenesisInput(t, 23)
	changedRevisionInput := stageGenesisInput(t, 23)
	changedRevisionInput.ProfileLedgerRevision = projectprofile.NewLedgerRevision(1)
	changedBasisInput := stageGenesisInput(t, 23)
	executable := mustStageExecutableTypeEnvFixture(t)
	changedBasis, changedAssessment := mustStageProfileAssessment(
		t,
		executable.snapshot,
		"/tmp/haft-stage-profile-other",
		projecttypeenvprofilefit.CurrentRuleEdition(),
	)
	changedBasisInput.ProfileLedgerRevision = changedBasis.LedgerRevision()
	changedBasisInput.ProfileLedgerDigest = changedBasis.ProfileLedgerDigest()
	changedBasisInput.ProfileCompatibility = changedAssessment
	base := sealStageFixture(t, baseInput)
	changedRevision := sealStageFixture(t, changedRevisionInput)
	changedBasisStage := sealStageFixture(t, changedBasisInput)
	if base.Ref() == changedRevision.Ref() {
		t.Fatalf("profile-ledger revision did not affect Stage identity")
	}
	if base.ProfileFitRef() != changedRevision.ProfileFitRef() {
		t.Fatalf("outer ledger revision unexpectedly rewrote full profile-fit identity")
	}
	if base.Ref() == changedBasisStage.Ref() {
		t.Fatalf("exact profile basis did not affect Stage identity")
	}
	if base.ProfileFitRef() == changedBasisStage.ProfileFitRef() {
		t.Fatalf("full profile-fit identity did not bind exact profile basis")
	}
}

func TestProjectTypeEnvStageProfilePostureIsClosedAndCanonical(t *testing.T) {
	executable := mustStageExecutableTypeEnvFixture(t)
	basis, leftProfile := mustStageProfileAssessment(
		t,
		executable.snapshot,
		"/tmp/haft-stage-profile-canonical",
		projecttypeenvprofilefit.CurrentRuleEdition(),
	)
	_, rightProfile := mustStageProfileAssessment(
		t,
		executable.snapshot,
		"/tmp/haft-stage-profile-canonical",
		projecttypeenvprofilefit.CurrentRuleEdition(),
	)
	leftInput := stageGenesisInput(t, 31)
	leftInput.ProfileLedgerRevision = basis.LedgerRevision()
	leftInput.ProfileLedgerDigest = basis.ProfileLedgerDigest()
	leftInput.ProfileCompatibility = leftProfile
	rightInput := stageGenesisInput(t, 31)
	rightInput.ProfileLedgerRevision = basis.LedgerRevision()
	rightInput.ProfileLedgerDigest = basis.ProfileLedgerDigest()
	rightInput.ProfileCompatibility = rightProfile
	left := sealStageFixture(t, leftInput)
	right := sealStageFixture(t, rightInput)
	if left.Ref() != right.Ref() {
		t.Fatalf("repeated deterministic profile assessment changed Stage identity")
	}
	if _, ok := left.ProfileCompatibility().(projecttypeenvprofilefit.Underdetermined); !ok {
		t.Fatalf("profile posture = %T", left.ProfileCompatibility())
	}
	decoded, err := projecttypeenvprofilefit.DecodeCanonicalAssessment(
		leftProfile.CanonicalBytes(),
	)
	if err != nil {
		t.Fatalf("DecodeCanonicalAssessment(): %v", err)
	}
	if decoded.Digest() != leftProfile.Digest() ||
		decoded.FitRef() != leftProfile.FitRef() ||
		!bytes.Equal(decoded.CanonicalBytes(), leftProfile.CanonicalBytes()) {
		t.Fatalf("full profile-fit assessment changed on strict round-trip")
	}
	trailing := append(leftProfile.CanonicalBytes(), 0x00)
	if _, err := projecttypeenvprofilefit.DecodeCanonicalAssessment(trailing); err == nil {
		t.Fatalf("profile-fit codec accepted trailing bytes")
	}
	futureEdition, err := projecttypeenvprofilefit.NewRuleEdition(
		"haft.project-typeenv.profile-fit-rules/future",
	)
	if err != nil {
		t.Fatalf("NewRuleEdition(): %v", err)
	}
	_, unavailable := mustStageProfileAssessment(
		t,
		executable.snapshot,
		"/tmp/haft-stage-profile-canonical",
		futureEdition,
	)
	if _, ok := unavailable.(projecttypeenvprofilefit.Unavailable); !ok {
		t.Fatalf("future-edition profile posture = %T; want unavailable", unavailable)
	}
	unavailableInput := stageGenesisInput(t, 31)
	unavailableInput.ProfileLedgerRevision = basis.LedgerRevision()
	unavailableInput.ProfileLedgerDigest = basis.ProfileLedgerDigest()
	unavailableInput.ProfileCompatibility = unavailable
	unavailableStage := sealStageFixture(t, unavailableInput)
	if left.Ref() == unavailableStage.Ref() ||
		left.ProfileFitRef() == unavailableStage.ProfileFitRef() {
		t.Fatalf("profile-fit edition and grounds did not affect exact identity")
	}

	forged := replaceCanonicalText(t, left.CanonicalBytes(), "underdetermined", "unsupported____")
	if _, err := DecodeProjectTypeEnvStage(forged); err == nil ||
		!strings.Contains(err.Error(), "unsupported profile-fit assessment variant") {
		t.Fatalf("decode accepted unknown profile posture: %v", err)
	}
}

func TestProjectTypeEnvStageRejectsForgeryTrailingAndStoredMutation(t *testing.T) {
	stage := sealStageFixture(t, stageGenesisInput(t, 41))
	other := sealStageFixture(t, stageGenesisInput(t, 42))
	if _, err := VerifyProjectTypeEnvStage(other.Ref(), stage.CanonicalBytes()); err == nil {
		t.Fatalf("Verify accepted forged expected Stage ref")
	}
	trailing := append(stage.CanonicalBytes(), 0x00)
	if _, err := DecodeProjectTypeEnvStage(trailing); err == nil ||
		!strings.Contains(err.Error(), "trailing bytes") {
		t.Fatalf("Decode accepted trailing bytes: %v", err)
	}
	forgedCompatibilityRef := "project-typeenv-compatibility-diff:" + testDigest(t, "f").String()
	forgedCanonical := replaceCanonicalText(
		t,
		stage.CanonicalBytes(),
		stage.CompatibilityRef().String(),
		forgedCompatibilityRef,
	)
	if _, err := DecodeProjectTypeEnvStage(forgedCanonical); err == nil ||
		!strings.Contains(err.Error(), "compatibility ref or digest mismatch") {
		t.Fatalf("Decode accepted forged compatibility ref: %v", err)
	}
	forgedVerificationRef := "project-typeenv-composite-verification:" + testDigest(t, "d").String()
	forgedCanonical = replaceCanonicalText(
		t,
		stage.CanonicalBytes(),
		stage.CompositeVerificationRef().String(),
		forgedVerificationRef,
	)
	if _, err := DecodeProjectTypeEnvStage(forgedCanonical); err == nil ||
		!strings.Contains(err.Error(), "composite verification ref and digest mismatch") {
		t.Fatalf("Decode accepted forged composite-verification ref: %v", err)
	}
	mutated := stage
	mutated.base = testTypeEnvRef(t, "d")
	if err := mutated.Verify(); err == nil || !strings.Contains(err.Error(), "stored state") {
		t.Fatalf("Verify accepted mutated stored state: %v", err)
	}
	mutatedIdentity := stage
	mutatedIdentity.compatibilityDigest = testDigest(t, "e")
	if err := mutatedIdentity.Verify(); err == nil ||
		!strings.Contains(err.Error(), "compatibility ref or digest mismatch") {
		t.Fatalf("Verify accepted mutated compatibility identity: %v", err)
	}
	oversized := make([]byte, maximumProjectTypeEnvStageBytes+1)
	if _, err := DecodeProjectTypeEnvStage(oversized); err == nil ||
		!strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("Decode accepted oversized Stage: %v", err)
	}
}

func TestProjectTypeEnvStageRequiresFinalLowererCapability(t *testing.T) {
	input := stageGenesisInput(t, 43)
	record := input.VerifiedComposite.Record()
	if err := record.Verify(); err != nil {
		t.Fatalf("persistable verification record is invalid: %v", err)
	}
	input.VerifiedComposite = projecttypeenv.ProjectTypeEnvCompositeVerification{}
	if _, err := SealProjectTypeEnvStage(input); err == nil ||
		!strings.Contains(err.Error(), "capability") {
		t.Fatalf("Stage accepted an unminted final-lowerer capability: %v", err)
	}
}

func TestProjectTypeEnvStageOwnsAllCallerSlices(t *testing.T) {
	input := stageTransitionInput(t, 51)
	extensions := input.OrderedExtensions
	stage := sealStageFixture(t, input)

	extensions = append(extensions, mustExtensionRef(t, "changed", "9"))
	returnedExtensions := stage.OrderedExtensions()
	returnedExtensions = append(
		returnedExtensions,
		mustExtensionRef(t, "returned.changed", "7"),
	)
	compatibility := stage.Compatibility().(ComparedStageCompatibility)
	expectedDiffCanonical := compatibility.Diff().CanonicalBytes()
	returnedDiff := compatibility.Diff()
	returnedChanges := returnedDiff.Changes()
	returnedChanges[0] = nil
	returnedDiffCanonical := returnedDiff.CanonicalBytes()
	returnedDiffCanonical[0] ^= 0xff
	report := stage.ExistingAssertionRevalidation()
	expectedReportCanonical := report.CanonicalBytes()
	returnedOutcomes := report.Outcomes()
	returnedOutcomes[0] = nil
	returnedReportCanonical := report.CanonicalBytes()
	returnedReportCanonical[0] ^= 0xff
	profile := stage.ProfileCompatibility()
	expectedProfileCanonical := profile.CanonicalBytes()
	returnedGrounds := profile.Grounds()
	returnedGrounds[0] = projecttypeenvprofilefit.Ground{}
	returnedProfileCanonical := profile.CanonicalBytes()
	returnedProfileCanonical[0] ^= 0xff
	canonical := stage.CanonicalBytes()
	canonical[0] ^= 0xff

	if err := stage.Verify(); err != nil {
		t.Fatalf("caller mutation changed sealed Stage: %v", err)
	}
	if len(stage.OrderedExtensions()) != 0 || len(extensions) != 1 || len(returnedExtensions) != 1 {
		t.Fatalf("Stage retained caller-owned extension slice")
	}
	if bytes.Equal(canonical, stage.CanonicalBytes()) {
		t.Fatalf("CanonicalBytes exposed mutable storage")
	}
	storedCompatibility := stage.Compatibility().(ComparedStageCompatibility)
	storedDiff := storedCompatibility.Diff()
	if !bytes.Equal(storedDiff.CanonicalBytes(), expectedDiffCanonical) {
		t.Fatalf("Compatibility exposed mutable diff canonical storage")
	}
	storedChanges := storedDiff.Changes()
	if len(storedChanges) != 2 || storedChanges[0] == nil {
		t.Fatalf("Compatibility exposed mutable change slice")
	}
	storedReport := stage.ExistingAssertionRevalidation()
	if !bytes.Equal(storedReport.CanonicalBytes(), expectedReportCanonical) {
		t.Fatalf("Stage exposed mutable assertion-report canonical storage")
	}
	storedOutcomes := storedReport.Outcomes()
	if len(storedOutcomes) != 2 || storedOutcomes[0] == nil {
		t.Fatalf("Stage exposed mutable assertion-report outcome slice")
	}
	storedProfile := stage.ProfileCompatibility()
	if !bytes.Equal(storedProfile.CanonicalBytes(), expectedProfileCanonical) {
		t.Fatalf("Stage exposed mutable profile-assessment canonical storage")
	}
	storedGrounds := storedProfile.Grounds()
	if len(storedGrounds) != 1 ||
		storedGrounds[0].Kind() != projecttypeenvprofilefit.GroundNoCanonicalProfile {
		t.Fatalf("Stage exposed mutable profile-assessment ground slice")
	}
}

func stageGenesisInput(t *testing.T, revision uint64) ProjectTypeEnvStageInput {
	t.Helper()
	project := testProjectID(t)
	snapshot := testCommittedSnapshotBasis(t, revision, "3")
	executable := mustStageExecutableTypeEnvFixture(t)
	verified := executable.verification
	base := verified.BaseTypeEnvRef()
	extensions := verified.ExtensionRefs()
	runtimeBasis := verified.RuntimeEvaluationBasisRef()
	composite := verified.CompositeRef()
	predecessor := NewGenesisStagePredecessor()
	profileBasis, profile := mustStageProfileAssessment(
		t,
		executable.snapshot,
		"/tmp/haft-stage-profile",
		projecttypeenvprofilefit.CurrentRuleEdition(),
	)
	compatibility := mustInitialStageCompatibility(t, composite)
	report := mustStageAssertionReport(
		t,
		stageAssertionReportInput{
			target:            composite,
			graphSnapshot:     snapshot,
			graphRevision:     typedmemory.NewGraphRevision(revision),
			runtimeBasis:      runtimeBasis,
			runtimeCoordinate: testDigest(t, "a"),
			includeOutcomes:   true,
		},
	)
	return ProjectTypeEnvStageInput{
		Project:                       project,
		Predecessor:                   predecessor,
		Base:                          base,
		OrderedExtensions:             extensions,
		RuntimeBasis:                  runtimeBasis,
		VerifiedComposite:             verified,
		Composite:                     composite,
		GraphSnapshotBasis:            snapshot,
		GraphSnapshotBasisRef:         snapshot.Ref(),
		GraphSnapshotBasisDigest:      snapshot.Ref().Digest(),
		GraphRevision:                 typedmemory.NewGraphRevision(revision),
		ProfileLedgerRevision:         profileBasis.LedgerRevision(),
		ProfileLedgerDigest:           profileBasis.ProfileLedgerDigest(),
		Compatibility:                 compatibility,
		ExistingAssertionRevalidation: report,
		ProfileCompatibility:          profile,
	}
}

func stageTransitionInput(t *testing.T, revision uint64) ProjectTypeEnvStageInput {
	t.Helper()
	input := stageGenesisInput(t, revision)
	priorComposite := testTypeEnvRef(t, "c")
	predecessor, err := NewTransitionStagePredecessor(TransitionStagePredecessorInput{
		Project:           input.Project,
		Head:              mustHeadRef(t, input.Project),
		HeadRevision:      mustHeadRevision(t, 4),
		SelectedComposite: priorComposite,
	})
	if err != nil {
		t.Fatalf("NewTransitionStagePredecessor(): %v", err)
	}
	diff := mustStageCompatibilityDiff(t, priorComposite, input.Composite, false)
	compatibility, err := NewComparedStageCompatibility(diff)
	if err != nil {
		t.Fatalf("NewComparedStageCompatibility(): %v", err)
	}
	input.Predecessor = predecessor
	input.Compatibility = compatibility
	baseEnvironment := mustStageCompatibilityTypeEnv(t, priorComposite, false, false)
	targetEnvironment := mustStageCompatibilityTypeEnv(t, input.Composite, true, false)
	successor, err := projecttypeenvcompatibility.CompareSuccessor(
		baseEnvironment,
		targetEnvironment,
	)
	if err != nil {
		t.Fatalf("projecttypeenvcompatibility.CompareSuccessor(): %v", err)
	}
	transitionProfiles, err := projecttypeenvtransitioncompatibility.New(
		successor,
		[]byte("stage-transition-profile-compatibility-fixture"),
	)
	if err != nil {
		t.Fatalf("projecttypeenvtransitioncompatibility.New(): %v", err)
	}
	input.TransitionProjectionProfileCompatibility = transitionProfiles
	return input
}

type stageAssertionReportInput struct {
	target            typedmemory.TypeEnvRef
	graphSnapshot     ProjectGraphSnapshotBasis
	graphRevision     typedmemory.GraphRevision
	runtimeBasis      projecttypeenv.RuntimeEvaluationBasisRef
	runtimeCoordinate typedmemory.SHA256Digest
	includeOutcomes   bool
	reverseOutcomes   bool
}

func mustStageExecutableTypeEnvFixture(
	t *testing.T,
) stageExecutableTypeEnvFixture {
	t.Helper()
	stageExecutableFixtureOnce.Do(func() {
		stageExecutableFixture, stageExecutableFixtureErr =
			buildStageExecutableTypeEnvFixture()
	})
	if stageExecutableFixtureErr != nil {
		t.Fatalf("build executable Stage fixture: %v", stageExecutableFixtureErr)
	}
	if err := stageExecutableFixture.verification.Verify(); err != nil {
		t.Fatalf("verify executable Stage fixture capability: %v", err)
	}
	if err := stageExecutableFixture.snapshot.Verify(); err != nil {
		t.Fatalf("verify executable Stage fixture snapshot: %v", err)
	}
	return stageExecutableFixture
}

func buildStageExecutableTypeEnvFixture() (
	stageExecutableTypeEnvFixture,
	error,
) {
	databasePath, err := filepath.Abs(filepath.Join("..", "cli", "fpf.db"))
	if err != nil {
		return stageExecutableTypeEnvFixture{}, err
	}
	database, err := sql.Open(
		"sqlite",
		"file:"+filepath.ToSlash(databasePath)+"?mode=ro&immutable=1",
	)
	if err != nil {
		return stageExecutableTypeEnvFixture{}, err
	}
	database.SetMaxOpenConns(1)
	defer func() { _ = database.Close() }()

	base, err := typeenvsql.LoadArtifactReadOnlyDB(context.Background(), database)
	if err != nil {
		return stageExecutableTypeEnvFixture{}, err
	}
	resolution := projecttypeenv.LinkProjectTypeEnvCompositeIR(base, nil)
	if resolution.Rejected() {
		return stageExecutableTypeEnvFixture{}, fmt.Errorf(
			"link empty project extension DAG: %#v",
			resolution.Issues(),
		)
	}
	linked, exists := resolution.CompositeIR()
	if !exists {
		return stageExecutableTypeEnvFixture{}, fmt.Errorf(
			"accepted empty project extension DAG has no linked IR",
		)
	}
	runtimeBasis, err := stageRuntimeEvaluationBasis(base, linked)
	if err != nil {
		return stageExecutableTypeEnvFixture{}, err
	}
	composite, err := projecttypeenv.SealProjectTypeEnvComposite(
		linked,
		runtimeBasis,
	)
	if err != nil {
		return stageExecutableTypeEnvFixture{}, err
	}
	preparation := projecttypeenv.PrepareProjectTypeEnvComposite(
		projecttypeenv.ProjectTypeEnvCompositePreparationInput{
			Base:         base,
			Linked:       linked,
			RuntimeBasis: runtimeBasis,
			Composite:    composite,
		},
	)
	if preparation.Rejected() {
		return stageExecutableTypeEnvFixture{}, fmt.Errorf(
			"prepare executable Stage fixture: %#v",
			preparation.Issues(),
		)
	}
	verification, verified := preparation.Verification()
	if !verified {
		return stageExecutableTypeEnvFixture{}, fmt.Errorf(
			"accepted Stage fixture has no verification capability",
		)
	}
	snapshot, executable := preparation.ExecutableSnapshot()
	if !executable {
		return stageExecutableTypeEnvFixture{}, fmt.Errorf(
			"accepted Stage fixture has no executable snapshot",
		)
	}
	return stageExecutableTypeEnvFixture{
		verification: verification,
		snapshot:     snapshot,
	}, nil
}

func mustStageAssertionReport(
	t *testing.T,
	input stageAssertionReportInput,
) projecttypeenvassertionreport.Report {
	t.Helper()
	graphRef, err := projecttypeenvassertionreport.ParseGraphSnapshotRef(
		input.graphSnapshot.Ref().String(),
	)
	if err != nil {
		t.Fatalf("ParseGraphSnapshotRef(): %v", err)
	}
	graph, err := projecttypeenvassertionreport.NewGraphSnapshotCoordinate(
		graphRef,
		input.graphRevision,
		input.graphSnapshot.Ref().Digest(),
	)
	if err != nil {
		t.Fatalf("NewGraphSnapshotCoordinate(): %v", err)
	}
	outcomes := []projecttypeenvassertionreport.AssertionOutcome{}
	if input.includeOutcomes {
		alpha := mustStageAssertionOutcome(t, "assertion:alpha", "1")
		zeta := mustStageAssertionOutcome(t, "assertion:zeta", "2")
		outcomes = []projecttypeenvassertionreport.AssertionOutcome{alpha, zeta}
		if input.reverseOutcomes {
			outcomes = []projecttypeenvassertionreport.AssertionOutcome{zeta, alpha}
		}
	}
	report, err := projecttypeenvassertionreport.NewReport(
		input.target,
		graph,
		input.runtimeBasis,
		input.runtimeCoordinate,
		outcomes,
	)
	if err != nil {
		t.Fatalf("NewReport(): %v", err)
	}
	if err := report.Verify(); err != nil {
		t.Fatalf("verify assertion-revalidation report: %v", err)
	}
	return report
}

func mustStageAssertionOutcome(
	t *testing.T,
	assertionRaw string,
	digestDigit string,
) projecttypeenvassertionreport.AssertionOutcome {
	t.Helper()
	assertion, err := typedmemory.NewAssertionID(assertionRaw)
	if err != nil {
		t.Fatalf("NewAssertionID(): %v", err)
	}
	outcome, err := projecttypeenvassertionreport.NewAssertionOutcome(
		assertion,
		testDigest(t, digestDigit),
		nil,
	)
	if err != nil {
		t.Fatalf("NewAssertionOutcome(): %v", err)
	}
	return outcome
}

func mustRebindStageReportTarget(
	t *testing.T,
	report projecttypeenvassertionreport.Report,
	target typedmemory.TypeEnvRef,
) projecttypeenvassertionreport.Report {
	t.Helper()
	rebound, err := projecttypeenvassertionreport.NewReport(
		target,
		report.GraphSnapshot(),
		report.RuntimeBasisRef(),
		report.RuntimeCoordinateDigest(),
		report.Outcomes(),
	)
	if err != nil {
		t.Fatalf("rebind assertion report target: %v", err)
	}
	return rebound
}

func mustRebindStageReportRuntime(
	t *testing.T,
	report projecttypeenvassertionreport.Report,
	runtime projecttypeenv.RuntimeEvaluationBasisRef,
) projecttypeenvassertionreport.Report {
	t.Helper()
	rebound, err := projecttypeenvassertionreport.NewReport(
		report.TargetTypeEnv(),
		report.GraphSnapshot(),
		runtime,
		report.RuntimeCoordinateDigest(),
		report.Outcomes(),
	)
	if err != nil {
		t.Fatalf("rebind assertion report runtime: %v", err)
	}
	return rebound
}

func mustStageProfileAssessment(
	t *testing.T,
	snapshot projecttypeenv.ProjectTypeEnvExecutableSnapshot,
	rootRaw string,
	edition projecttypeenvprofilefit.RuleEdition,
) (
	projecttypeenvprofilebasis.CurrentProjectProfileBasis,
	projecttypeenvprofilefit.Assessment,
) {
	t.Helper()
	root, err := projectprofile.NewProjectRootV1(rootRaw)
	if err != nil {
		t.Fatalf("NewProjectRootV1(): %v", err)
	}
	basis, err := projecttypeenvprofilebasis.NewNoCanonicalProjectProfile(root)
	if err != nil {
		t.Fatalf("NewNoCanonicalProjectProfile(): %v", err)
	}
	assessment, err := projecttypeenvprofilefit.AssessProjectTypeEnvProfileFitWithEdition(
		basis,
		snapshot,
		edition,
	)
	if err != nil {
		t.Fatalf("AssessProjectTypeEnvProfileFitWithEdition(): %v", err)
	}
	if err := assessment.Verify(); err != nil {
		t.Fatalf("verify profile-fit assessment: %v", err)
	}
	return basis, assessment
}

func mustRebindStageAssessmentTarget(
	t *testing.T,
	assessment projecttypeenvprofilefit.Assessment,
	target typedmemory.TypeEnvRef,
) projecttypeenvprofilefit.Assessment {
	t.Helper()
	canonical := replaceCanonicalText(
		t,
		assessment.CanonicalBytes(),
		assessment.TargetTypeEnvRef().String(),
		target.String(),
	)
	rebound, err := projecttypeenvprofilefit.DecodeCanonicalAssessment(canonical)
	if err != nil {
		t.Fatalf("rebind profile-fit assessment target: %v", err)
	}
	return rebound
}

func mustInitialStageCompatibility(
	t *testing.T,
	target typedmemory.TypeEnvRef,
) InitialStageCompatibility {
	t.Helper()
	compatibility, err := NewInitialStageCompatibility(target)
	if err != nil {
		t.Fatalf("NewInitialStageCompatibility(): %v", err)
	}
	return compatibility
}

func mustStageCompatibilityDiff(
	t *testing.T,
	baseRef typedmemory.TypeEnvRef,
	targetRef typedmemory.TypeEnvRef,
	reverseTargetDeclarations bool,
) projecttypeenvcompatibility.Diff {
	t.Helper()
	base := mustStageCompatibilityTypeEnv(t, baseRef, false, false)
	target := mustStageCompatibilityTypeEnv(
		t,
		targetRef,
		true,
		reverseTargetDeclarations,
	)
	diff, err := projecttypeenvcompatibility.Compare(base, target)
	if err != nil {
		t.Fatalf("projecttypeenvcompatibility.Compare(): %v", err)
	}
	if err := diff.Verify(); err != nil {
		t.Fatalf("verify compatibility diff: %v", err)
	}
	return diff
}

func mustStageCompatibilityTypeEnv(
	t *testing.T,
	ref typedmemory.TypeEnvRef,
	includeTargetDeclarations bool,
	reverseDeclarations bool,
) typedmemory.TypeEnv {
	t.Helper()
	provenance, coverage := stageCompatibilitySource(t)
	commonRefValue, commonRefErr := typedmemory.NewBoundedContextRef(
		"ctx:stage-compatibility-common",
	)
	commonRef := mustStageValue(t, commonRefValue, commonRefErr)
	commonValue, commonErr := typedmemory.NewBoundedContext(commonRef, provenance)
	common := mustStageValue(t, commonValue, commonErr)
	optionalRefValue, optionalRefErr := typedmemory.NewBoundedContextRef(
		"ctx:stage-compatibility-optional",
	)
	optionalRef := mustStageValue(t, optionalRefValue, optionalRefErr)
	optionalValue, optionalErr := typedmemory.NewBoundedContext(optionalRef, provenance)
	optional := mustStageValue(t, optionalValue, optionalErr)
	kindIDValue, kindIDErr := typedmemory.NewKindID("Haft.StageCompatibilityFixture")
	kindID := mustStageValue(t, kindIDValue, kindIDErr)
	kindValue, kindErr := typedmemory.NewKindDefinition(kindID, provenance)
	kind := mustStageValue(t, kindValue, kindErr)
	sourceRevisionValue, sourceRevisionErr := typedmemory.NewSourceRevision(
		"stage-compatibility-source-v1",
	)
	sourceRevision := mustStageValue(t, sourceRevisionValue, sourceRevisionErr)
	compilerVersionValue, compilerVersionErr := typedmemory.NewCompilerSchemaVersion(
		"stage-compatibility-compiler/v1",
	)
	compilerVersion := mustStageValue(t, compilerVersionValue, compilerVersionErr)
	builder := typedmemory.NewTypeEnvBuilder(ref).
		SetSourceRevision(sourceRevision).
		SetCompilerSchemaVersion(compilerVersion).
		SetCoverageManifest(coverage)
	if !includeTargetDeclarations {
		value, err := builder.
			AddBoundedContext(common).
			Build()
		return mustStageValue(t, value, err)
	}
	if reverseDeclarations {
		builder.
			AddKindDefinition(kind).
			AddBoundedContext(optional).
			AddBoundedContext(common)
	} else {
		builder.
			AddBoundedContext(common).
			AddBoundedContext(optional).
			AddKindDefinition(kind)
	}
	value, err := builder.Build()
	return mustStageValue(t, value, err)
}

func stageCompatibilitySource(
	t *testing.T,
) (typedmemory.FPFSourceProvenance, typedmemory.CoverageManifest) {
	t.Helper()
	unitValue, unitErr := typedmemory.NewSourceUnitID(
		"spec:stage-compatibility-fixture",
	)
	unit := mustStageValue(t, unitValue, unitErr)
	revisionValue, revisionErr := typedmemory.NewSourceRevision(
		"stage-compatibility-source-v1",
	)
	revision := mustStageValue(t, revisionValue, revisionErr)
	lineRangeValue, lineRangeErr := typedmemory.NewSourceLineRange(1, 10)
	lineRange := mustStageValue(t, lineRangeValue, lineRangeErr)
	patternIDValue, patternIDErr := typedmemory.NewPatternID("A.6.0")
	patternID := mustStageValue(t, patternIDValue, patternIDErr)
	locationValue, locationErr := typedmemory.NewPatternedSourceLocation(
		unit,
		revision,
		testDigest(t, "6"),
		lineRange,
		patternID,
	)
	location := mustStageValue(t, locationValue, locationErr)
	provenanceRefValue, provenanceRefErr := typedmemory.NewProvenanceRef(
		"prov:stage-compatibility-fixture",
	)
	provenanceRef := mustStageValue(t, provenanceRefValue, provenanceRefErr)
	ruleIDValue, ruleIDErr := typedmemory.NewCompilerRuleID(
		"stage.compatibility.fixture.v1",
	)
	ruleID := mustStageValue(t, ruleIDValue, ruleIDErr)
	provenanceValue, provenanceErr := typedmemory.NewFPFSourceProvenance(
		provenanceRef,
		location,
		ruleID,
	)
	provenance := mustStageValue(t, provenanceValue, provenanceErr)
	subjectValue, subjectErr := typedmemory.SourceUnitCoverage(unit)
	subject := mustStageValue(t, subjectValue, subjectErr)
	entryValue, entryErr := typedmemory.NewCompiledCoverageEntry(subject, location)
	entry := mustStageValue(t, entryValue, entryErr)
	coverageValue, coverageErr := typedmemory.NewCoverageManifest(
		[]typedmemory.CoverageEntry{entry},
	)
	coverage := mustStageValue(t, coverageValue, coverageErr)
	return provenance, coverage
}

func mustStageValue[T any](t *testing.T, value T, err error) T {
	t.Helper()
	if err != nil {
		t.Fatalf("build Stage compatibility fixture: %v", err)
	}
	return value
}

func sealStageFixture(t *testing.T, input ProjectTypeEnvStageInput) ProjectTypeEnvStage {
	t.Helper()
	stage, err := SealProjectTypeEnvStage(input)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvStage(): %v", err)
	}
	return stage
}

func mustExtensionRef(t *testing.T, id string, digit string) typedmemory.TypeEnvExtensionRef {
	t.Helper()
	raw := "typeenv-extension:" + id + "@sha256:" + strings.Repeat(digit, 64)
	ref, err := typedmemory.ParseTypeEnvExtensionRef(raw)
	if err != nil {
		t.Fatalf("ParseTypeEnvExtensionRef(%q): %v", raw, err)
	}
	return ref
}

func mustRuntimeBasisRef(t *testing.T, digit string) projecttypeenv.RuntimeEvaluationBasisRef {
	t.Helper()
	raw := "runtime-evaluation-basis:sha256:" + strings.Repeat(digit, 64)
	ref, err := projecttypeenv.ParseRuntimeEvaluationBasisRef(raw)
	if err != nil {
		t.Fatalf("ParseRuntimeEvaluationBasisRef(): %v", err)
	}
	return ref
}

func mustNoPriorProofRef(t *testing.T, digit string) NoPriorHeadProofRef {
	t.Helper()
	ref, err := ParseNoPriorHeadProofRef(
		"project-typeenv-no-prior-head-proof:sha256:" + strings.Repeat(digit, 64),
	)
	if err != nil {
		t.Fatalf("ParseNoPriorHeadProofRef(): %v", err)
	}
	return ref
}

func mustHeadRef(t *testing.T, project projectidentity.ProjectID) ProjectTypeEnvHeadRef {
	t.Helper()
	ref, err := ProjectTypeEnvHeadRefForProject(project)
	if err != nil {
		t.Fatalf("ProjectTypeEnvHeadRefForProject(): %v", err)
	}
	return ref
}

func mustHeadRevision(t *testing.T, value uint64) HeadRevision {
	t.Helper()
	revision, err := NewHeadRevision(value)
	if err != nil {
		t.Fatalf("NewHeadRevision(): %v", err)
	}
	return revision
}

func mustStageProvenanceRef(t *testing.T, digit string) ProjectTypeEnvStageProvenanceRef {
	t.Helper()
	ref, err := ParseProjectTypeEnvStageProvenanceRef(
		"project-typeenv-stage-provenance:sha256:" + strings.Repeat(digit, 64),
	)
	if err != nil {
		t.Fatalf("ParseProjectTypeEnvStageProvenanceRef(): %v", err)
	}
	return ref
}

func mustSnapshotRef(t *testing.T, digit string) ProjectGraphSnapshotBasisRef {
	t.Helper()
	ref, err := ParseProjectGraphSnapshotBasisRef(
		"project-graph-snapshot-basis:sha256:" + strings.Repeat(digit, 64),
	)
	if err != nil {
		t.Fatalf("ParseProjectGraphSnapshotBasisRef(): %v", err)
	}
	return ref
}

func testDigest(t *testing.T, digit string) typedmemory.SHA256Digest {
	t.Helper()
	digest, err := typedmemory.NewSHA256Digest("sha256:" + strings.Repeat(digit, 64))
	if err != nil {
		t.Fatalf("NewSHA256Digest(): %v", err)
	}
	return digest
}

func mustProjectID(t *testing.T, raw string) projectidentity.ProjectID {
	t.Helper()
	project, err := projectidentity.ParseProjectID(raw)
	if err != nil {
		t.Fatalf("ParseProjectID(): %v", err)
	}
	return project
}

func replaceCanonicalText(
	t *testing.T,
	canonical []byte,
	old string,
	replacement string,
) []byte {
	t.Helper()
	if len(old) != len(replacement) {
		t.Fatalf("replacement lengths differ: %q vs %q", old, replacement)
	}
	needle := []byte(old)
	if bytes.Count(canonical, needle) != 1 {
		t.Fatalf("canonical occurrence count for %q = %d", old, bytes.Count(canonical, needle))
	}
	return bytes.Replace(canonical, needle, []byte(replacement), 1)
}
