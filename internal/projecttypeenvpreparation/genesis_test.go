package projecttypeenvpreparation

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	typedmemorycandidates "github.com/m0n0x41d/haft/data/haft/local-practice/typed-memory/candidates"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/fpf/typeenvsql"
	"github.com/m0n0x41d/haft/internal/projectgraphobservation"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projectmemory/localpracticeruntime"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilebasis"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	_ "modernc.org/sqlite"
)

func TestPrepareGenesisCandidateDerivesExactNonBindingClosureAndStage(
	t *testing.T,
) {
	t.Parallel()

	input := genesisCandidateTestInput(t)
	candidate, err := PrepareGenesisCandidate(input)
	if err != nil {
		t.Fatalf("PrepareGenesisCandidate() error = %v", err)
	}
	stage := candidate.Stage()
	target := candidate.Target()
	baseRef, executable := input.Target.Base().TypeEnvRef()
	if !executable {
		t.Fatal("test base is not executable")
	}
	if candidate.BaseSnapshot().Ref() != baseRef ||
		candidate.ArtifactClosure().Base().Digest() != input.Target.Base().Digest() ||
		candidate.ArtifactClosure().Composite().Ref() != target.Composite().Ref() {
		t.Fatal("Genesis candidate lost exact B/E/X/C closure coordinates")
	}
	if stage.Project() != input.Project ||
		stage.Base() != baseRef ||
		stage.VerifiedComposite() != target.Composite().Ref() ||
		stage.RuntimeBasis() != target.RuntimeBasis().Ref() ||
		stage.GraphSnapshotBasis() != input.CurrentGraph.GraphSnapshotBasis().Ref() ||
		stage.GraphRevision() != input.CurrentGraph.GraphSnapshotBasis().GraphRevision() ||
		stage.ProfileLedgerRevision() != input.CurrentProfile.LedgerRevision() ||
		stage.ProfileLedgerDigest() != input.CurrentProfile.ProfileLedgerDigest() ||
		stage.SchemaEdition() != projecttypeenvselection.ProjectTypeEnvStageSchemaEditionV4 {
		t.Fatal("Genesis Stage differs from exact current observations or target")
	}
	if _, ok := stage.Predecessor().(projecttypeenvselection.GenesisStagePredecessor); !ok {
		t.Fatalf("Genesis Stage predecessor = %T", stage.Predecessor())
	}
	if _, historical := stage.HistoricalProvenance(); historical {
		t.Fatal("current Genesis Stage retained historical caller provenance")
	}
	if err := stage.Verify(); err != nil {
		t.Fatalf("Genesis Stage Verify() error = %v", err)
	}
	if err := candidate.Verification().Verify(); err != nil {
		t.Fatalf("Genesis verification Verify() error = %v", err)
	}
	if err := candidate.ExecutableSnapshot().Verify(); err != nil {
		t.Fatalf("Genesis executable snapshot Verify() error = %v", err)
	}
	if !candidate.ExactRuntime().Valid() {
		t.Fatal("Genesis candidate has no exact non-serializable runtime")
	}
}

func TestPrepareGenesisCandidateRejectsGraphNotAtExactBase(t *testing.T) {
	t.Parallel()

	input := genesisCandidateTestInput(t)
	otherDigest, err := typedmemory.NewSHA256Digest(
		"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	)
	if err != nil {
		t.Fatal(err)
	}
	otherRef, err := typedmemory.NewTypeEnvRef(otherDigest)
	if err != nil {
		t.Fatal(err)
	}
	input.CurrentGraph = genesisCandidateGraphObservation(
		t,
		input.Project,
		otherRef,
	)

	_, err = PrepareGenesisCandidate(input)
	if err == nil || !strings.Contains(err.Error(), "is not exact base") {
		t.Fatalf("PrepareGenesisCandidate(mismatched active TypeEnv) error = %v", err)
	}
}

func TestPrepareGenesisCandidateRejectsNonZeroGraphRevision(t *testing.T) {
	t.Parallel()

	input := genesisCandidateTestInput(t)
	event, err := projecttypeenvselection.ParseGraphEventRef(
		"typed-memory-event:" + strings.Repeat("a", 64),
	)
	if err != nil {
		t.Fatal(err)
	}
	commit, err := projecttypeenvselection.ParseGraphCommitRef(
		"typed-memory-commit:" + strings.Repeat("b", 64),
	)
	if err != nil {
		t.Fatal(err)
	}
	materialization, err := typedmemory.NewSHA256Digest(
		"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
	)
	if err != nil {
		t.Fatal(err)
	}
	closure, err := projecttypeenvselection.NewCommittedProjectGraphClosure(
		projecttypeenvselection.CommittedProjectGraphClosureInput{
			Event:                 event,
			Commit:                commit,
			MaterializationDigest: materialization,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	revision := typedmemory.NewGraphRevision(1)
	basis, err := projecttypeenvselection.SealProjectGraphSnapshotBasis(
		projecttypeenvselection.ProjectGraphSnapshotBasisInput{
			Project:       input.Project,
			GraphRevision: revision,
			Closure:       closure,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	active, err := projectgraphobservation.NewCurrentActiveAssertionSet(
		input.Project,
		revision,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	input.CurrentGraph, err = projectgraphobservation.NewCurrentProjectGraphObservation(
		basis,
		input.CurrentGraph.ActiveTypeEnv(),
		active,
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = PrepareGenesisCandidate(input)
	if err == nil || !strings.Contains(err.Error(), "revision zero") {
		t.Fatalf("PrepareGenesisCandidate(non-zero graph) error = %v", err)
	}
}

func TestPrepareGenesisCandidateRejectsForeignProfileRoot(t *testing.T) {
	t.Parallel()

	input := genesisCandidateTestInput(t)
	foreignRoot, err := projectprofile.NewProjectRootV1(
		filepath.Clean(t.TempDir()),
	)
	if err != nil {
		t.Fatal(err)
	}
	input.ProjectRoot = foreignRoot

	_, err = PrepareGenesisCandidate(input)
	if err == nil || !strings.Contains(err.Error(), "profile project root mismatch") {
		t.Fatalf("PrepareGenesisCandidate(foreign profile root) error = %v", err)
	}
}

func genesisCandidateTestInput(t *testing.T) GenesisCandidateInput {
	t.Helper()
	base := genesisCandidateBaseArtifact(t)
	baseRef, executable := base.TypeEnvRef()
	if !executable {
		t.Fatal("bundled base artifact is not executable")
	}
	project, err := projectidentity.ParseProjectID("qnt_8eadbeef")
	if err != nil {
		t.Fatal(err)
	}
	root, err := projectprofile.NewProjectRootV1(filepath.Clean(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	profile, err := projecttypeenvprofilebasis.NewNoCanonicalProjectProfile(root)
	if err != nil {
		t.Fatal(err)
	}
	return GenesisCandidateInput{
		Project:     project,
		ProjectRoot: root,
		Target: genesisCandidateTarget(
			t,
			base,
		),
		CurrentGraph: genesisCandidateGraphObservation(
			t,
			project,
			baseRef,
		),
		CurrentProfile: profile,
	}
}

func genesisCandidateTarget(
	t *testing.T,
	base typeenv.BaseTypeEnvArtifact,
) localpracticeruntime.Target {
	t.Helper()
	target, err := localpracticeruntime.Build(
		base,
		typedmemorycandidates.SourceV1_5(),
	)
	if err != nil {
		t.Fatal(err)
	}
	return target
}

func genesisCandidateGraphObservation(
	t *testing.T,
	project projectidentity.ProjectID,
	activeTypeEnv typedmemory.TypeEnvRef,
) projectgraphobservation.CurrentProjectGraphObservation {
	t.Helper()
	revision := typedmemory.NewGraphRevision(0)
	basis, err := projecttypeenvselection.SealProjectGraphSnapshotBasis(
		projecttypeenvselection.ProjectGraphSnapshotBasisInput{
			Project:       project,
			GraphRevision: revision,
			Closure:       projecttypeenvselection.EmptyProjectGraphClosure{},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	active, err := projectgraphobservation.NewCurrentActiveAssertionSet(
		project,
		revision,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	observation, err := projectgraphobservation.NewCurrentProjectGraphObservation(
		basis,
		activeTypeEnv,
		active,
	)
	if err != nil {
		t.Fatal(err)
	}
	return observation
}

func genesisCandidateBaseArtifact(t *testing.T) typeenv.BaseTypeEnvArtifact {
	t.Helper()
	path := filepath.Join("..", "cli", "fpf.db")
	database, err := sql.Open(
		"sqlite",
		"file:"+filepath.ToSlash(path)+"?mode=ro&immutable=1",
	)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = database.Close() }()
	artifact, err := typeenvsql.LoadArtifactReadOnlyDB(
		context.Background(),
		database,
	)
	if err != nil {
		t.Fatal(err)
	}
	return artifact
}
