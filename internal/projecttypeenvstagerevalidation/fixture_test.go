package projecttypeenvstagerevalidation_test

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/fpf/typeenvsql"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/projecttypeenvassertionreport"
	"github.com/m0n0x41d/haft/internal/projecttypeenvcompatibility"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilebasis"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilecompatibility"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilefit"
	"github.com/m0n0x41d/haft/internal/projecttypeenvruntime"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/projecttypeenvtransitioncompatibility"
	"github.com/m0n0x41d/haft/internal/runtimemechanism"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	_ "modernc.org/sqlite"
)

type targetClosureFixture struct {
	verification projecttypeenv.ProjectTypeEnvCompositeVerification
	snapshot     projecttypeenv.ProjectTypeEnvExecutableSnapshot
	runtimeBasis projecttypeenv.RuntimeEvaluationBasisArtifact
	catalog      runtimemechanism.RuntimeMechanismArtifactV1
}

type targetClosureFixtureSet struct {
	alpha targetClosureFixture
	beta  targetClosureFixture
}

type rejectingStageFixtureCodec struct{}

func (rejectingStageFixtureCodec) Canonicalize(
	typedmemory.ValueShapeRef,
	[]byte,
) typedmemory.CodecCanonicalization {
	return typedmemory.RejectedCodecValue{}
}

var (
	targetClosureFixturesOnce sync.Once
	targetClosureFixtures     targetClosureFixtureSet
	targetClosureFixturesErr  error
)

func targetFixtures(t *testing.T) targetClosureFixtureSet {
	t.Helper()
	targetClosureFixturesOnce.Do(func() {
		targetClosureFixtures, targetClosureFixturesErr = buildTargetClosureFixtures()
	})
	if targetClosureFixturesErr != nil {
		t.Fatalf("build target-closure fixtures: %v", targetClosureFixturesErr)
	}
	return targetClosureFixtures
}

func buildTargetClosureFixtures() (targetClosureFixtureSet, error) {
	base, err := loadBaseArtifact()
	if err != nil {
		return targetClosureFixtureSet{}, err
	}
	resolution := projecttypeenv.LinkProjectTypeEnvCompositeIR(base, nil)
	if resolution.Rejected() {
		return targetClosureFixtureSet{}, fmt.Errorf(
			"link empty extension DAG: %#v",
			resolution.Issues(),
		)
	}
	linked, exists := resolution.CompositeIR()
	if !exists {
		return targetClosureFixtureSet{}, fmt.Errorf(
			"accepted empty extension DAG produced no linked IR",
		)
	}
	alpha, err := buildTargetClosure(base, linked, "alpha")
	if err != nil {
		return targetClosureFixtureSet{}, err
	}
	beta, err := buildTargetClosure(base, linked, "beta")
	if err != nil {
		return targetClosureFixtureSet{}, err
	}
	return targetClosureFixtureSet{alpha: alpha, beta: beta}, nil
}

func loadBaseArtifact() (typeenv.BaseTypeEnvArtifact, error) {
	databasePath, err := filepath.Abs(filepath.Join("..", "cli", "fpf.db"))
	if err != nil {
		return typeenv.BaseTypeEnvArtifact{}, err
	}
	database, err := sql.Open(
		"sqlite",
		"file:"+filepath.ToSlash(databasePath)+"?mode=ro&immutable=1",
	)
	if err != nil {
		return typeenv.BaseTypeEnvArtifact{}, err
	}
	database.SetMaxOpenConns(1)
	defer func() { _ = database.Close() }()
	return typeenvsql.LoadArtifactReadOnlyDB(context.Background(), database)
}

func buildTargetClosure(
	base typeenv.BaseTypeEnvArtifact,
	linked projecttypeenv.LinkedProjectTypeEnvCompositeIR,
	suffix string,
) (targetClosureFixture, error) {
	runtimeBasis, catalog, err := buildRuntimeEvaluationBasis(base, linked, suffix)
	if err != nil {
		return targetClosureFixture{}, err
	}
	composite, err := projecttypeenv.SealProjectTypeEnvComposite(linked, runtimeBasis)
	if err != nil {
		return targetClosureFixture{}, err
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
		return targetClosureFixture{}, fmt.Errorf(
			"prepare target closure %q: %#v",
			suffix,
			preparation.Issues(),
		)
	}
	verification, verified := preparation.Verification()
	if !verified {
		return targetClosureFixture{}, fmt.Errorf(
			"target closure %q has no final verification",
			suffix,
		)
	}
	snapshot, executable := preparation.ExecutableSnapshot()
	if !executable {
		return targetClosureFixture{}, fmt.Errorf(
			"target closure %q has no executable snapshot",
			suffix,
		)
	}
	return targetClosureFixture{
		verification: verification,
		snapshot:     snapshot,
		runtimeBasis: runtimeBasis,
		catalog:      catalog,
	}, nil
}

func buildRuntimeEvaluationBasis(
	base typeenv.BaseTypeEnvArtifact,
	linked projecttypeenv.LinkedProjectTypeEnvCompositeIR,
	suffix string,
) (
	projecttypeenv.RuntimeEvaluationBasisArtifact,
	runtimemechanism.RuntimeMechanismArtifactV1,
	error,
) {
	emptyBasis, err := projecttypeenv.SealRuntimeEvaluationBasis(nil)
	if err != nil {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{},
			runtimemechanism.RuntimeMechanismArtifactV1{},
			err
	}
	provisionalComposite, err := projecttypeenv.SealProjectTypeEnvComposite(
		linked,
		emptyBasis,
	)
	if err != nil {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{},
			runtimemechanism.RuntimeMechanismArtifactV1{},
			err
	}
	candidate, _, err := typeenv.LowerBaseTypeEnvArtifactWithCodecsAtRef(
		base,
		provisionalComposite.Ref(),
	)
	if err != nil {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{},
			runtimemechanism.RuntimeMechanismArtifactV1{},
			err
	}
	resolution := projecttypeenv.ResolveProjectTypeEnvCompositeRuntimeRequirements(
		provisionalComposite,
		candidate,
		linked,
		emptyBasis,
	)
	requirements := resolution.RequiredSet().Requirements()
	if len(requirements) == 0 {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{},
			runtimemechanism.RuntimeMechanismArtifactV1{},
			fmt.Errorf(
				"provisional composite has no runtime requirements",
			)
	}
	return sealRuntimeEvaluationBasis(requirements, suffix)
}

func sealRuntimeEvaluationBasis(
	requirements []projecttypeenv.CompositeRuntimeRequirement,
	suffix string,
) (
	projecttypeenv.RuntimeEvaluationBasisArtifact,
	runtimemechanism.RuntimeMechanismArtifactV1,
	error,
) {
	entries := make([]runtimemechanism.RuntimeMechanismEntryV1, 0, len(requirements))
	for _, requirement := range requirements {
		entry, err := runtimeMechanismEntry(requirement)
		if err != nil {
			return projecttypeenv.RuntimeEvaluationBasisArtifact{},
				runtimemechanism.RuntimeMechanismArtifactV1{},
				err
		}
		entries = append(entries, entry)
	}
	artifactRef, err := typedmemory.NewCarrierRef(
		"artifact:stage-revalidator-runtime-" + suffix,
	)
	if err != nil {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{},
			runtimemechanism.RuntimeMechanismArtifactV1{},
			err
	}
	edition, err := typedmemory.NewCarrierEdition("1.0.0")
	if err != nil {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{},
			runtimemechanism.RuntimeMechanismArtifactV1{},
			err
	}
	artifact, err := runtimemechanism.SealRuntimeMechanismArtifactV1(
		artifactRef,
		edition,
		entries,
	)
	if err != nil {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{},
			runtimemechanism.RuntimeMechanismArtifactV1{},
			err
	}
	mechanism, err := projecttypeenv.NewRuntimeMechanismArtifactPinFromArtifact(artifact)
	if err != nil {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{},
			runtimemechanism.RuntimeMechanismArtifactV1{},
			err
	}
	pins := make([]projecttypeenv.RuntimeEvaluationMechanismPin, 0, len(requirements))
	for _, requirement := range requirements {
		pin, err := runtimeMechanismPin(requirement, mechanism, artifact)
		if err != nil {
			return projecttypeenv.RuntimeEvaluationBasisArtifact{},
				runtimemechanism.RuntimeMechanismArtifactV1{},
				err
		}
		pins = append(pins, pin)
	}
	basis, err := projecttypeenv.SealRuntimeEvaluationBasis(pins, artifact)
	if err != nil {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{},
			runtimemechanism.RuntimeMechanismArtifactV1{},
			err
	}
	return basis, artifact, nil
}

func runtimeMechanismEntry(
	requirement projecttypeenv.CompositeRuntimeRequirement,
) (runtimemechanism.RuntimeMechanismEntryV1, error) {
	codec, hasCodec := requirement.Codec()
	if hasCodec {
		return runtimemechanism.NewCodecCanonicalizationEntry(codec)
	}
	rule, hasRule := requirement.Rule()
	if !hasRule {
		return runtimemechanism.RuntimeMechanismEntryV1{}, fmt.Errorf(
			"runtime requirement %q has no semantic reference",
			requirement.SemanticReference(),
		)
	}
	switch requirement.InvocationContract() {
	case projecttypeenv.RuntimeMechanismContractEntitySetEnumeration:
		return runtimemechanism.NewEntitySetEnumerationEntry(rule)
	case projecttypeenv.RuntimeMechanismContractCandidateVisibility:
		return runtimemechanism.NewCandidateVisibilityEntry(rule)
	case projecttypeenv.RuntimeMechanismContractKindDefinedness:
		return runtimemechanism.NewKindDefinednessEntry(rule)
	case projecttypeenv.RuntimeMechanismContractMemberOf:
		return runtimemechanism.NewMemberOfEntry(rule)
	case projecttypeenv.RuntimeMechanismContractCarrierMembershipDelivery:
		return runtimemechanism.NewCarrierMembershipDeliveryEntry(rule)
	default:
		return runtimemechanism.RuntimeMechanismEntryV1{}, fmt.Errorf(
			"unsupported runtime invocation contract %q",
			requirement.InvocationContract(),
		)
	}
}

func runtimeMechanismPin(
	requirement projecttypeenv.CompositeRuntimeRequirement,
	mechanism projecttypeenv.RuntimeMechanismArtifactPin,
	artifact runtimemechanism.RuntimeMechanismArtifactV1,
) (projecttypeenv.RuntimeEvaluationMechanismPin, error) {
	codec, hasCodec := requirement.Codec()
	if hasCodec {
		return projecttypeenv.NewCodecRuntimeMechanismPin(
			projecttypeenv.CodecRuntimeMechanismPinInput{
				Codec:            codec,
				Mechanism:        mechanism,
				ResolvedArtifact: &artifact,
			},
		)
	}
	rule, hasRule := requirement.Rule()
	if !hasRule {
		return nil, fmt.Errorf(
			"runtime requirement %q has no semantic reference",
			requirement.SemanticReference(),
		)
	}
	if requirement.Role() == projecttypeenv.RuntimeMechanismRoleCarrierMembership {
		return projecttypeenv.NewCarrierMembershipRuntimeMechanismPin(
			projecttypeenv.CarrierMembershipRuntimeMechanismPinInput{
				Rule:             rule,
				Mechanism:        mechanism,
				ResolvedArtifact: &artifact,
			},
		)
	}
	return projecttypeenv.NewEvaluatorRuntimeMechanismPin(
		projecttypeenv.EvaluatorRuntimeMechanismPinInput{
			Rule:             rule,
			Contract:         requirement.InvocationContract(),
			Mechanism:        mechanism,
			ResolvedArtifact: &artifact,
		},
	)
}

func exactRuntimeRegistryForTarget(
	t *testing.T,
	target targetClosureFixture,
) projecttypeenvruntime.ExactTargetRuntimeRegistry {
	t.Helper()
	codecs := typedmemory.NewCodecRegistry()
	for _, pin := range target.runtimeBasis.Pins() {
		codecPin, isCodec := pin.(projecttypeenv.CodecRuntimeMechanismPin)
		if !isCodec {
			t.Fatalf(
				"Stage fixture X contains unsupported runtime pin %T",
				pin,
			)
		}
		updated, err := codecs.Register(
			codecPin.Codec(),
			rejectingStageFixtureCodec{},
		)
		if err != nil {
			t.Fatalf("register Stage fixture codec %s: %v", codecPin.Codec(), err)
		}
		codecs = updated
	}
	resolution := projecttypeenvruntime.ObserveCurrentTargetRuntime(
		projecttypeenvruntime.ObservationInput{
			RuntimeBasis: target.runtimeBasis,
			Installed: projecttypeenvruntime.InstalledRuntimeRegistryInput{
				Codecs:            codecs,
				MechanismCatalogs: []runtimemechanism.RuntimeMechanismArtifactV1{target.catalog},
			},
		},
	)
	matched, ok := resolution.(projecttypeenvruntime.Matched)
	if !ok {
		t.Fatalf(
			"Stage fixture runtime observation = %T (%s), want Matched",
			resolution,
			resolution.Kind(),
		)
	}
	registry, ok := matched.Registry()
	if !ok || !registry.Valid() {
		t.Fatal("Stage fixture runtime observation exposed no exact registry")
	}
	return registry
}

func testProject(t *testing.T, value string) projectidentity.ProjectID {
	t.Helper()
	project, err := projectidentity.ParseProjectID(value)
	if err != nil {
		t.Fatalf("ParseProjectID(%q): %v", value, err)
	}
	return project
}

func genesisStage(
	t *testing.T,
	project projectidentity.ProjectID,
	target targetClosureFixture,
) projecttypeenvselection.ProjectTypeEnvStage {
	t.Helper()
	predecessor := projecttypeenvselection.NewGenesisStagePredecessor()
	compatibility, err := projecttypeenvselection.NewInitialStageCompatibility(
		target.verification.CompositeRef(),
	)
	if err != nil {
		t.Fatalf("NewInitialStageCompatibility(): %v", err)
	}
	return stageWithPredecessor(
		t,
		project,
		target,
		predecessor,
		compatibility,
		projecttypeenvtransitioncompatibility.Set{},
	)
}

func transitionStage(
	t *testing.T,
	project projectidentity.ProjectID,
	target targetClosureFixture,
	prior projecttypeenvselection.ProjectTypeEnvHeadState,
) projecttypeenvselection.ProjectTypeEnvStage {
	t.Helper()
	predecessor, err := prior.ExactPriorHead()
	if err != nil {
		t.Fatalf("ExactPriorHead(): %v", err)
	}
	previous := target.snapshot.Environment()
	if prior.SelectedComposite() != target.snapshot.TypeEnvRef() {
		previous = stageFixtureTypeEnvAtRef(
			t,
			prior.SelectedComposite(),
			target.snapshot.Environment(),
		)
	}
	diff, err := projecttypeenvcompatibility.Compare(
		previous,
		target.snapshot.Environment(),
	)
	if err != nil {
		t.Fatalf("projecttypeenvcompatibility.Compare(): %v", err)
	}
	compatibility, err := projecttypeenvselection.NewComparedStageCompatibility(diff)
	if err != nil {
		t.Fatalf("NewComparedStageCompatibility(): %v", err)
	}
	successor, err := projecttypeenvcompatibility.CompareSuccessor(
		previous,
		target.snapshot.Environment(),
	)
	if err != nil {
		t.Fatalf("projecttypeenvcompatibility.CompareSuccessor(): %v", err)
	}
	projectionProfiles, err := projecttypeenvprofilecompatibility.AssessTransitionProjectionProfiles(
		successor,
	)
	if err != nil {
		t.Fatalf("AssessTransitionProjectionProfiles(): %v", err)
	}
	return stageWithPredecessor(
		t,
		project,
		target,
		predecessor,
		compatibility,
		projectionProfiles,
	)
}

func transitionStageFromExecutable(
	t *testing.T,
	project projectidentity.ProjectID,
	target targetClosureFixture,
	prior projecttypeenvselection.ProjectTypeEnvHeadState,
	priorExecutable projecttypeenv.ProjectTypeEnvExecutableSnapshot,
) projecttypeenvselection.ProjectTypeEnvStage {
	t.Helper()
	predecessor, err := prior.ExactPriorHead()
	if err != nil {
		t.Fatalf("ExactPriorHead(): %v", err)
	}
	if priorExecutable.TypeEnvRef() != prior.SelectedComposite() {
		t.Fatal("prior executable does not match prior selected C")
	}
	diff, err := projecttypeenvcompatibility.Compare(
		priorExecutable.Environment(),
		target.snapshot.Environment(),
	)
	if err != nil {
		t.Fatalf("projecttypeenvcompatibility.Compare(): %v", err)
	}
	compatibility, err := projecttypeenvselection.NewComparedStageCompatibility(diff)
	if err != nil {
		t.Fatalf("NewComparedStageCompatibility(): %v", err)
	}
	successor, err := projecttypeenvcompatibility.CompareSuccessor(
		priorExecutable.Environment(),
		target.snapshot.Environment(),
	)
	if err != nil {
		t.Fatalf("projecttypeenvcompatibility.CompareSuccessor(): %v", err)
	}
	projectionProfiles, err := projecttypeenvprofilecompatibility.AssessTransitionProjectionProfiles(
		successor,
	)
	if err != nil {
		t.Fatalf("AssessTransitionProjectionProfiles(): %v", err)
	}
	return stageWithPredecessor(
		t,
		project,
		target,
		predecessor,
		compatibility,
		projectionProfiles,
	)
}

func stageWithPredecessor(
	t *testing.T,
	project projectidentity.ProjectID,
	target targetClosureFixture,
	predecessor projecttypeenvselection.ProjectTypeEnvStagePredecessor,
	compatibility projecttypeenvselection.ProjectTypeEnvStageCompatibility,
	transitionProjectionProfiles projecttypeenvtransitioncompatibility.Set,
) projecttypeenvselection.ProjectTypeEnvStage {
	t.Helper()
	graphBasis, err := projecttypeenvselection.SealProjectGraphSnapshotBasis(
		projecttypeenvselection.ProjectGraphSnapshotBasisInput{
			Project:       project,
			GraphRevision: typedmemory.NewGraphRevision(0),
			Closure:       projecttypeenvselection.EmptyProjectGraphClosure{},
		},
	)
	if err != nil {
		t.Fatalf("SealProjectGraphSnapshotBasis(): %v", err)
	}
	graphRef, err := projecttypeenvassertionreport.ParseGraphSnapshotRef(
		graphBasis.Ref().String(),
	)
	if err != nil {
		t.Fatalf("ParseGraphSnapshotRef(): %v", err)
	}
	graphCoordinate, err := projecttypeenvassertionreport.NewGraphSnapshotCoordinate(
		graphRef,
		graphBasis.GraphRevision(),
		graphBasis.Ref().Digest(),
	)
	if err != nil {
		t.Fatalf("NewGraphSnapshotCoordinate(): %v", err)
	}
	revalidation, err := projecttypeenvassertionreport.NewReport(
		target.verification.CompositeRef(),
		graphCoordinate,
		target.verification.RuntimeEvaluationBasisRef(),
		target.verification.RuntimeEvaluationBasisRef().Digest(),
		nil,
	)
	if err != nil {
		t.Fatalf("projecttypeenvassertionreport.NewReport(): %v", err)
	}
	profileRoot, err := projectprofile.NewProjectRootV1(
		"/tmp/haft-stage-revalidation-" + project.String(),
	)
	if err != nil {
		t.Fatalf("NewProjectRootV1(): %v", err)
	}
	profileBasis, err := projecttypeenvprofilebasis.NewNoCanonicalProjectProfile(
		profileRoot,
	)
	if err != nil {
		t.Fatalf("NewNoCanonicalProjectProfile(): %v", err)
	}
	profile, err := projecttypeenvprofilefit.AssessProjectTypeEnvProfileFit(
		profileBasis,
		target.snapshot,
	)
	if err != nil {
		t.Fatalf("AssessProjectTypeEnvProfileFit(): %v", err)
	}
	verification := target.verification
	stage, err := projecttypeenvselection.SealProjectTypeEnvStage(
		projecttypeenvselection.ProjectTypeEnvStageInput{
			Project:                                  project,
			Predecessor:                              predecessor,
			Base:                                     verification.BaseTypeEnvRef(),
			OrderedExtensions:                        verification.ExtensionRefs(),
			RuntimeBasis:                             verification.RuntimeEvaluationBasisRef(),
			VerifiedComposite:                        verification,
			Composite:                                verification.CompositeRef(),
			GraphSnapshotBasis:                       graphBasis,
			GraphSnapshotBasisRef:                    graphBasis.Ref(),
			GraphSnapshotBasisDigest:                 graphBasis.Ref().Digest(),
			GraphRevision:                            typedmemory.NewGraphRevision(0),
			ProfileLedgerRevision:                    profileBasis.LedgerRevision(),
			ProfileLedgerDigest:                      profileBasis.ProfileLedgerDigest(),
			Compatibility:                            compatibility,
			ExistingAssertionRevalidation:            revalidation,
			ProfileCompatibility:                     profile,
			TransitionProjectionProfileCompatibility: transitionProjectionProfiles,
		},
	)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvStage(): %v", err)
	}
	return stage
}

func stageFixtureTypeEnvAtRef(
	t *testing.T,
	ref typedmemory.TypeEnvRef,
	source typedmemory.TypeEnv,
) typedmemory.TypeEnv {
	t.Helper()
	contexts := source.BoundedContexts()
	if len(contexts) == 0 {
		t.Fatal("source executable TypeEnv has no bounded context")
	}
	environment, err := typedmemory.NewTypeEnvBuilder(ref).
		SetSourceRevision(source.SourceRevision()).
		SetCompilerSchemaVersion(source.CompilerSchemaVersion()).
		SetCoverageManifest(source.CoverageManifest()).
		AddBoundedContext(contexts[0]).
		Build()
	if err != nil {
		t.Fatalf("build prior executable TypeEnv fixture: %v", err)
	}
	return environment
}

func headState(
	t *testing.T,
	project projectidentity.ProjectID,
	composite typedmemory.TypeEnvRef,
	revision uint64,
) projecttypeenvselection.ProjectTypeEnvHeadState {
	t.Helper()
	headRevision, err := projecttypeenvselection.NewHeadRevision(revision)
	if err != nil {
		t.Fatalf("NewHeadRevision(): %v", err)
	}
	state, err := projecttypeenvselection.SealProjectTypeEnvHeadState(
		projecttypeenvselection.ProjectTypeEnvHeadStateInput{
			Project:           project,
			SelectedComposite: composite,
			Revision:          headRevision,
		},
	)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvHeadState(): %v", err)
	}
	return state
}

func syntheticTypeEnvRef(t *testing.T, digit string) typedmemory.TypeEnvRef {
	t.Helper()
	ref, err := typedmemory.ParseTypeEnvRef(
		"typeenv:sha256:" + strings.Repeat(digit, 64),
	)
	if err != nil {
		t.Fatalf("ParseTypeEnvRef(): %v", err)
	}
	return ref
}

func testDigest(t *testing.T, digit string) typedmemory.SHA256Digest {
	t.Helper()
	digest, err := typedmemory.NewSHA256Digest(
		"sha256:" + strings.Repeat(digit, 64),
	)
	if err != nil {
		t.Fatalf("NewSHA256Digest(): %v", err)
	}
	return digest
}
