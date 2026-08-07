package projecttypeenvstage

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf/localpractice"
	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/fpf/typeenvsql"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/projecttypeenvassertionreport"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilebasis"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilefit"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/projecttypeenvstore"
	"github.com/m0n0x41d/haft/internal/runtimemechanism"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	_ "modernc.org/sqlite"
)

var (
	stageStoreBaseOnce sync.Once
	stageStoreBase     typeenv.BaseTypeEnvArtifact
	stageStoreBaseErr  error
)

type stageStoreDomainFixture struct {
	closure      projecttypeenvstore.ArtifactClosure
	stage        projecttypeenvselection.ProjectTypeEnvStage
	verification projecttypeenv.ProjectTypeEnvCompositeVerificationRecord
	snapshot     projecttypeenv.ProjectTypeEnvExecutableSnapshotRecord
}

type stageStoreFixture struct {
	database *sql.DB
	store    *Store
	domain   stageStoreDomainFixture
}

func newStageStoreFixture(t *testing.T, suffix string) stageStoreFixture {
	t.Helper()
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open Stage store database: %v", err)
	}
	database.SetMaxOpenConns(1)
	store, err := New(context.Background(), database)
	if err != nil {
		_ = database.Close()
		t.Fatalf("New(): %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return stageStoreFixture{
		database: database,
		store:    store,
		domain:   newStageStoreDomainFixture(t, suffix),
	}
}

func newStageStoreDomainFixture(t *testing.T, suffix string) stageStoreDomainFixture {
	t.Helper()
	base := stageStoreBaseFixture(t)
	extension := stageStoreExtensionFixture(t, base, suffix)
	extensions := []projecttypeenv.ProjectTypeEnvExtensionArtifact{extension}
	resolution := projecttypeenv.LinkProjectTypeEnvCompositeIR(base, extensions)
	if resolution.Rejected() {
		t.Fatalf("LinkProjectTypeEnvCompositeIR(): %#v", resolution.Issues())
	}
	linked, exists := resolution.CompositeIR()
	if !exists {
		t.Fatal("accepted B/E link has no linked IR")
	}
	runtimeBasis, mechanism := stageStoreRuntimeBasis(t, base, linked, suffix)
	composite, err := projecttypeenv.SealProjectTypeEnvComposite(linked, runtimeBasis)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvComposite(): %v", err)
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
		t.Fatalf("PrepareProjectTypeEnvComposite(): %#v", preparation.Issues())
	}
	verification, exists := preparation.Verification()
	if !exists {
		t.Fatal("accepted final lowering has no verification capability")
	}
	snapshot, exists := preparation.ExecutableSnapshot()
	if !exists {
		t.Fatal("accepted final lowering has no executable snapshot")
	}
	closure, err := projecttypeenvstore.PrepareArtifactClosureWithRuntimeMechanisms(
		base,
		extensions,
		runtimeBasis,
		composite,
		[]runtimemechanism.RuntimeMechanismArtifactV1{mechanism},
	)
	if err != nil {
		t.Fatalf("PrepareArtifactClosureWithRuntimeMechanisms(): %v", err)
	}
	stage := stageStoreStageFixture(t, verification, snapshot)
	return stageStoreDomainFixture{
		closure:      closure,
		stage:        stage,
		verification: verification.Record(),
		snapshot:     snapshot.Record(),
	}
}

func stageStoreBaseFixture(t *testing.T) typeenv.BaseTypeEnvArtifact {
	t.Helper()
	stageStoreBaseOnce.Do(func() {
		databasePath, err := filepath.Abs(filepath.Join("..", "cli", "fpf.db"))
		if err != nil {
			stageStoreBaseErr = err
			return
		}
		database, err := sql.Open(
			"sqlite",
			"file:"+filepath.ToSlash(databasePath)+"?mode=ro&immutable=1",
		)
		if err != nil {
			stageStoreBaseErr = err
			return
		}
		database.SetMaxOpenConns(1)
		defer func() { _ = database.Close() }()
		stageStoreBase, stageStoreBaseErr = typeenvsql.LoadArtifactReadOnlyDB(
			context.Background(),
			database,
		)
	})
	if stageStoreBaseErr != nil {
		t.Fatalf("load Stage store B: %v", stageStoreBaseErr)
	}
	return stageStoreBase
}

func stageStoreExtensionFixture(
	t *testing.T,
	base typeenv.BaseTypeEnvArtifact,
	suffix string,
) projecttypeenv.ProjectTypeEnvExtensionArtifact {
	t.Helper()
	baseRef, exists := base.TypeEnvRef()
	if !exists {
		t.Fatal("compiled FPF base has no TypeEnvRef")
	}
	carrierID := "haft.stage-store-fixture-" + suffix
	boundedContext := "haft-stage-store-" + suffix
	valueKind := "Haft.StageStoreFixture" + strings.ToUpper(suffix)
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
    slice_set: StageStoreFixtureSliceSet
    extent_rule: haft.stage-store-fixture.extent/v1
  vocabulary:
    declarations:
      - kind: bounded_context
        symbol: %s
      - kind: value_kind
        symbol: %s
  laws:
    constraint_refs: []
    invariants:
      - Stage store fixture values remain distinguishable.
  applicability:
    bounded_context_ref: %s
    assumptions:
      - The carrier is used only for immutable Stage persistence tests.
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
	manifestResolution := projecttypeenv.ResolveManifestGraph(
		base,
		[]localpractice.ParsedCarrier{parsed},
	)
	if manifestResolution.Rejected() {
		t.Fatalf("ResolveManifestGraph(): %#v", manifestResolution.Issues())
	}
	bundle, exists := manifestResolution.Bundle()
	if !exists || len(bundle.Nodes()) != 1 {
		t.Fatalf("resolved manifest bundle nodes = %d, exists = %v", len(bundle.Nodes()), exists)
	}
	ir, err := projecttypeenv.CompileProjectTypeEnvExtensionIR(bundle.Nodes()[0], nil)
	if err != nil {
		t.Fatalf("CompileProjectTypeEnvExtensionIR(): %v", err)
	}
	artifact, err := projecttypeenv.SealProjectTypeEnvExtension(ir)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvExtension(): %v", err)
	}
	return artifact
}

func stageStoreRuntimeBasis(
	t *testing.T,
	base typeenv.BaseTypeEnvArtifact,
	linked projecttypeenv.LinkedProjectTypeEnvCompositeIR,
	suffix string,
) (
	projecttypeenv.RuntimeEvaluationBasisArtifact,
	runtimemechanism.RuntimeMechanismArtifactV1,
) {
	t.Helper()
	emptyBasis, err := projecttypeenv.SealRuntimeEvaluationBasis(nil)
	if err != nil {
		t.Fatalf("SealRuntimeEvaluationBasis(empty): %v", err)
	}
	provisionalComposite, err := projecttypeenv.SealProjectTypeEnvComposite(
		linked,
		emptyBasis,
	)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvComposite(provisional): %v", err)
	}
	candidate, _, err := typeenv.LowerBaseTypeEnvArtifactWithCodecsAtRef(
		base,
		provisionalComposite.Ref(),
	)
	if err != nil {
		t.Fatalf("LowerBaseTypeEnvArtifactWithCodecsAtRef(): %v", err)
	}
	requirementResolution := projecttypeenv.ResolveProjectTypeEnvCompositeRuntimeRequirements(
		provisionalComposite,
		candidate,
		linked,
		emptyBasis,
	)
	requirements := requirementResolution.RequiredSet().Requirements()
	if len(requirements) == 0 {
		t.Fatal("provisional composite has no runtime requirements")
	}
	entries := make([]runtimemechanism.RuntimeMechanismEntryV1, 0, len(requirements))
	for _, requirement := range requirements {
		entry, entryErr := stageStoreMechanismEntry(requirement)
		if entryErr != nil {
			t.Fatalf("build runtime mechanism entry: %v", entryErr)
		}
		entries = append(entries, entry)
	}
	artifactRef, err := typedmemory.NewCarrierRef("artifact:stage-store-runtime-" + suffix)
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
		entries,
	)
	if err != nil {
		t.Fatalf("SealRuntimeMechanismArtifactV1(): %v", err)
	}
	mechanismPin, err := projecttypeenv.NewRuntimeMechanismArtifactPinFromArtifact(mechanism)
	if err != nil {
		t.Fatalf("NewRuntimeMechanismArtifactPinFromArtifact(): %v", err)
	}
	pins := make([]projecttypeenv.RuntimeEvaluationMechanismPin, 0, len(requirements))
	for _, requirement := range requirements {
		pin, pinErr := stageStoreMechanismPin(requirement, mechanismPin, mechanism)
		if pinErr != nil {
			t.Fatalf("build runtime mechanism pin: %v", pinErr)
		}
		pins = append(pins, pin)
	}
	basis, err := projecttypeenv.SealRuntimeEvaluationBasis(pins, mechanism)
	if err != nil {
		t.Fatalf("SealRuntimeEvaluationBasis(): %v", err)
	}
	return basis, mechanism
}

func stageStoreMechanismEntry(
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

func stageStoreMechanismPin(
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

func stageStoreStageFixture(
	t *testing.T,
	verification projecttypeenv.ProjectTypeEnvCompositeVerification,
	executable projecttypeenv.ProjectTypeEnvExecutableSnapshot,
) projecttypeenvselection.ProjectTypeEnvStage {
	t.Helper()
	project, err := projectidentity.ParseProjectID("qnt_1234abcd")
	if err != nil {
		t.Fatalf("ParseProjectID(): %v", err)
	}
	snapshot, err := projecttypeenvselection.SealProjectGraphSnapshotBasis(
		projecttypeenvselection.ProjectGraphSnapshotBasisInput{
			Project:       project,
			GraphRevision: typedmemory.NewGraphRevision(0),
			Closure:       projecttypeenvselection.EmptyProjectGraphClosure{},
		},
	)
	if err != nil {
		t.Fatalf("SealProjectGraphSnapshotBasis(): %v", err)
	}
	predecessor := projecttypeenvselection.NewGenesisStagePredecessor()
	graphRef, err := projecttypeenvassertionreport.ParseGraphSnapshotRef(
		snapshot.Ref().String(),
	)
	if err != nil {
		t.Fatalf("ParseGraphSnapshotRef(): %v", err)
	}
	graphCoordinate, err := projecttypeenvassertionreport.NewGraphSnapshotCoordinate(
		graphRef,
		snapshot.GraphRevision(),
		snapshot.Ref().Digest(),
	)
	if err != nil {
		t.Fatalf("NewGraphSnapshotCoordinate(): %v", err)
	}
	revalidation, err := projecttypeenvassertionreport.NewReport(
		verification.CompositeRef(),
		graphCoordinate,
		verification.RuntimeEvaluationBasisRef(),
		verification.RuntimeEvaluationBasisRef().Digest(),
		nil,
	)
	if err != nil {
		t.Fatalf("projecttypeenvassertionreport.NewReport(): %v", err)
	}
	profileRoot, err := projectprofile.NewProjectRootV1(
		"/tmp/haft-stage-store-fixture",
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
		executable,
	)
	if err != nil {
		t.Fatalf("AssessProjectTypeEnvProfileFit(): %v", err)
	}
	initialCompatibility, err := projecttypeenvselection.NewInitialStageCompatibility(
		verification.CompositeRef(),
	)
	if err != nil {
		t.Fatalf("NewInitialStageCompatibility(): %v", err)
	}
	stage, err := projecttypeenvselection.SealProjectTypeEnvStage(
		projecttypeenvselection.ProjectTypeEnvStageInput{
			Project:                       project,
			Predecessor:                   predecessor,
			Base:                          verification.BaseTypeEnvRef(),
			OrderedExtensions:             verification.ExtensionRefs(),
			RuntimeBasis:                  verification.RuntimeEvaluationBasisRef(),
			VerifiedComposite:             verification,
			Composite:                     verification.CompositeRef(),
			GraphSnapshotBasis:            snapshot,
			GraphSnapshotBasisRef:         snapshot.Ref(),
			GraphSnapshotBasisDigest:      snapshot.Ref().Digest(),
			GraphRevision:                 typedmemory.NewGraphRevision(0),
			ProfileLedgerRevision:         profileBasis.LedgerRevision(),
			ProfileLedgerDigest:           profileBasis.ProfileLedgerDigest(),
			Compatibility:                 initialCompatibility,
			ExistingAssertionRevalidation: revalidation,
			ProfileCompatibility:          profile,
		},
	)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvStage(): %v", err)
	}
	return stage
}

func stageStoreDigest(t *testing.T, digit string) typedmemory.SHA256Digest {
	t.Helper()
	digest, err := typedmemory.NewSHA256Digest("sha256:" + strings.Repeat(digit, 64))
	if err != nil {
		t.Fatalf("NewSHA256Digest(): %v", err)
	}
	return digest
}
