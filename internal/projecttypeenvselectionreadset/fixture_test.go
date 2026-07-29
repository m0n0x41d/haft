package projecttypeenvselectionreadset

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/fpf/typeenvsql"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/projecttypeenvassertionreport"
	"github.com/m0n0x41d/haft/internal/projecttypeenvheadstore"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilebasis"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilefit"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/runtimemechanism"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	_ "modernc.org/sqlite"
)

type genesisReadSetFixture struct {
	database      *sql.DB
	store         *projecttypeenvheadstore.Store
	project       projectidentity.ProjectID
	graph         projecttypeenvselection.ProjectGraphSnapshotBasis
	stage         projecttypeenvselection.ProjectTypeEnvStage
	request       projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest
	observedAt    time.Time
	observedInput GenesisHeadObservationInput
}

type readSetTargetFixture struct {
	verification projecttypeenv.ProjectTypeEnvCompositeVerification
	snapshot     projecttypeenv.ProjectTypeEnvExecutableSnapshot
}

var (
	readSetTargetOnce sync.Once
	readSetTarget     readSetTargetFixture
	readSetTargetErr  error
)

func newGenesisReadSetFixture(
	t *testing.T,
	graphRevision uint64,
) genesisReadSetFixture {
	t.Helper()
	path := filepath.Join(t.TempDir(), "project-typeenv-genesis-readset.db")
	database, err := sql.Open(
		"sqlite",
		"file:"+filepath.ToSlash(path)+"?_pragma=foreign_keys(1)",
	)
	if err != nil {
		t.Fatalf("open Genesis read-set database: %v", err)
	}
	database.SetMaxOpenConns(4)
	store, err := projecttypeenvheadstore.New(context.Background(), database)
	if err != nil {
		_ = database.Close()
		t.Fatalf("projecttypeenvheadstore.New(): %v", err)
	}
	project := readSetProject(t, "qnt_89abcdef")
	graph := readSetGraphBasis(t, project, graphRevision, "a")
	stage := readSetGenesisStage(t, project, graph)
	key, err := projecttypeenvselection.NewProjectTypeEnvHeadSelectionIdempotencyKey(
		fmt.Sprintf("genesis-readset-%d", graphRevision),
	)
	if err != nil {
		_ = database.Close()
		t.Fatalf("NewProjectTypeEnvHeadSelectionIdempotencyKey(): %v", err)
	}
	request, err := projecttypeenvselection.SealGenesisProjectTypeEnvHeadSelectionRequest(
		projecttypeenvselection.GenesisProjectTypeEnvHeadSelectionRequestInput{
			Project:               project,
			Stage:                 stage,
			ExpectedGraphRevision: graph.GraphRevision(),
			IdempotencyKey:        key,
		},
	)
	if err != nil {
		_ = database.Close()
		t.Fatalf("SealGenesisProjectTypeEnvHeadSelectionRequest(): %v", err)
	}
	observedAt := time.Date(
		2026,
		time.July,
		17,
		11,
		23,
		45,
		123456789,
		time.FixedZone("fixture", 4*60*60),
	)
	t.Cleanup(func() { _ = database.Close() })
	input := GenesisHeadObservationInput{
		Request:      request,
		Stage:        stage,
		CurrentGraph: graph,
		ObservedAt:   observedAt,
	}
	return genesisReadSetFixture{
		database:      database,
		store:         store,
		project:       project,
		graph:         graph,
		stage:         stage,
		request:       request,
		observedAt:    observedAt,
		observedInput: input,
	}
}

func readSetGenesisStage(
	t *testing.T,
	project projectidentity.ProjectID,
	graph projecttypeenvselection.ProjectGraphSnapshotBasis,
) projecttypeenvselection.ProjectTypeEnvStage {
	t.Helper()
	target := readSetTargetClosure(t)
	predecessor := projecttypeenvselection.NewGenesisStagePredecessor()
	graphRef, err := projecttypeenvassertionreport.ParseGraphSnapshotRef(
		graph.Ref().String(),
	)
	if err != nil {
		t.Fatalf("ParseGraphSnapshotRef(): %v", err)
	}
	graphCoordinate, err := projecttypeenvassertionreport.NewGraphSnapshotCoordinate(
		graphRef,
		graph.GraphRevision(),
		graph.Ref().Digest(),
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
		"/tmp/haft-genesis-readset-" + project.String(),
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
	profileCompatibility, err := projecttypeenvprofilefit.AssessProjectTypeEnvProfileFit(
		profileBasis,
		target.snapshot,
	)
	if err != nil {
		t.Fatalf("AssessProjectTypeEnvProfileFit(): %v", err)
	}
	initialCompatibility, err := projecttypeenvselection.NewInitialStageCompatibility(
		target.verification.CompositeRef(),
	)
	if err != nil {
		t.Fatalf("NewInitialStageCompatibility(): %v", err)
	}
	verification := target.verification
	stage, err := projecttypeenvselection.SealProjectTypeEnvStage(
		projecttypeenvselection.ProjectTypeEnvStageInput{
			Project:                       project,
			Predecessor:                   predecessor,
			Base:                          verification.BaseTypeEnvRef(),
			OrderedExtensions:             verification.ExtensionRefs(),
			RuntimeBasis:                  verification.RuntimeEvaluationBasisRef(),
			VerifiedComposite:             verification,
			Composite:                     verification.CompositeRef(),
			GraphSnapshotBasis:            graph,
			GraphSnapshotBasisRef:         graph.Ref(),
			GraphSnapshotBasisDigest:      graph.Ref().Digest(),
			GraphRevision:                 graph.GraphRevision(),
			ProfileLedgerRevision:         profileBasis.LedgerRevision(),
			ProfileLedgerDigest:           profileBasis.ProfileLedgerDigest(),
			Compatibility:                 initialCompatibility,
			ExistingAssertionRevalidation: revalidation,
			ProfileCompatibility:          profileCompatibility,
		},
	)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvStage(): %v", err)
	}
	return stage
}

func readSetTargetClosure(t *testing.T) readSetTargetFixture {
	t.Helper()
	readSetTargetOnce.Do(func() {
		readSetTarget, readSetTargetErr = buildReadSetTargetClosure()
	})
	if readSetTargetErr != nil {
		t.Fatalf("build read-set target closure: %v", readSetTargetErr)
	}
	return readSetTarget
}

func buildReadSetTargetClosure() (readSetTargetFixture, error) {
	base, err := loadReadSetBaseArtifact()
	if err != nil {
		return readSetTargetFixture{}, err
	}
	resolution := projecttypeenv.LinkProjectTypeEnvCompositeIR(base, nil)
	if resolution.Rejected() {
		return readSetTargetFixture{}, fmt.Errorf(
			"link empty extension DAG: %#v",
			resolution.Issues(),
		)
	}
	linked, exists := resolution.CompositeIR()
	if !exists {
		return readSetTargetFixture{}, fmt.Errorf(
			"accepted empty extension DAG produced no linked IR",
		)
	}
	runtimeBasis, err := buildReadSetRuntimeEvaluationBasis(base, linked)
	if err != nil {
		return readSetTargetFixture{}, err
	}
	composite, err := projecttypeenv.SealProjectTypeEnvComposite(linked, runtimeBasis)
	if err != nil {
		return readSetTargetFixture{}, err
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
		return readSetTargetFixture{}, fmt.Errorf(
			"prepare read-set target closure: %#v",
			preparation.Issues(),
		)
	}
	verification, verified := preparation.Verification()
	if !verified {
		return readSetTargetFixture{}, fmt.Errorf(
			"read-set target closure has no final verification",
		)
	}
	snapshot, executable := preparation.ExecutableSnapshot()
	if !executable {
		return readSetTargetFixture{}, fmt.Errorf(
			"read-set target closure has no executable snapshot",
		)
	}
	return readSetTargetFixture{
		verification: verification,
		snapshot:     snapshot,
	}, nil
}

func loadReadSetBaseArtifact() (typeenv.BaseTypeEnvArtifact, error) {
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

func buildReadSetRuntimeEvaluationBasis(
	base typeenv.BaseTypeEnvArtifact,
	linked projecttypeenv.LinkedProjectTypeEnvCompositeIR,
) (projecttypeenv.RuntimeEvaluationBasisArtifact, error) {
	emptyBasis, err := projecttypeenv.SealRuntimeEvaluationBasis(nil)
	if err != nil {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{}, err
	}
	provisionalComposite, err := projecttypeenv.SealProjectTypeEnvComposite(
		linked,
		emptyBasis,
	)
	if err != nil {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{}, err
	}
	candidate, _, err := typeenv.LowerBaseTypeEnvArtifactWithCodecsAtRef(
		base,
		provisionalComposite.Ref(),
	)
	if err != nil {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{}, err
	}
	resolution := projecttypeenv.ResolveProjectTypeEnvCompositeRuntimeRequirements(
		provisionalComposite,
		candidate,
		linked,
		emptyBasis,
	)
	requirements := resolution.RequiredSet().Requirements()
	if len(requirements) == 0 {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{}, fmt.Errorf(
			"provisional composite has no runtime requirements",
		)
	}
	entries := make([]runtimemechanism.RuntimeMechanismEntryV1, 0, len(requirements))
	for _, requirement := range requirements {
		entry, entryErr := readSetRuntimeMechanismEntry(requirement)
		if entryErr != nil {
			return projecttypeenv.RuntimeEvaluationBasisArtifact{}, entryErr
		}
		entries = append(entries, entry)
	}
	artifactRef, err := typedmemory.NewCarrierRef(
		"artifact:genesis-readset-runtime",
	)
	if err != nil {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{}, err
	}
	edition, err := typedmemory.NewCarrierEdition("1.0.0")
	if err != nil {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{}, err
	}
	artifact, err := runtimemechanism.SealRuntimeMechanismArtifactV1(
		artifactRef,
		edition,
		entries,
	)
	if err != nil {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{}, err
	}
	mechanism, err := projecttypeenv.NewRuntimeMechanismArtifactPinFromArtifact(artifact)
	if err != nil {
		return projecttypeenv.RuntimeEvaluationBasisArtifact{}, err
	}
	pins := make([]projecttypeenv.RuntimeEvaluationMechanismPin, 0, len(requirements))
	for _, requirement := range requirements {
		pin, pinErr := readSetRuntimeMechanismPin(
			requirement,
			mechanism,
			artifact,
		)
		if pinErr != nil {
			return projecttypeenv.RuntimeEvaluationBasisArtifact{}, pinErr
		}
		pins = append(pins, pin)
	}
	return projecttypeenv.SealRuntimeEvaluationBasis(pins, artifact)
}

func readSetRuntimeMechanismEntry(
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

func readSetRuntimeMechanismPin(
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

func readSetGraphBasis(
	t *testing.T,
	project projectidentity.ProjectID,
	revision uint64,
	digit string,
) projecttypeenvselection.ProjectGraphSnapshotBasis {
	t.Helper()
	var closure projecttypeenvselection.ProjectGraphClosure
	if revision == 0 {
		closure = projecttypeenvselection.EmptyProjectGraphClosure{}
	} else {
		event, err := projecttypeenvselection.ParseGraphEventRef(
			"typed-memory-event:" + strings.Repeat(digit, 64),
		)
		if err != nil {
			t.Fatalf("ParseGraphEventRef(): %v", err)
		}
		commit, err := projecttypeenvselection.ParseGraphCommitRef(
			"typed-memory-commit:" + strings.Repeat(digit, 64),
		)
		if err != nil {
			t.Fatalf("ParseGraphCommitRef(): %v", err)
		}
		committed, err := projecttypeenvselection.NewCommittedProjectGraphClosure(
			projecttypeenvselection.CommittedProjectGraphClosureInput{
				Event:                 event,
				Commit:                commit,
				MaterializationDigest: readSetDigest(t, digit),
			},
		)
		if err != nil {
			t.Fatalf("NewCommittedProjectGraphClosure(): %v", err)
		}
		closure = committed
	}
	basis, err := projecttypeenvselection.SealProjectGraphSnapshotBasis(
		projecttypeenvselection.ProjectGraphSnapshotBasisInput{
			Project:       project,
			GraphRevision: typedmemory.NewGraphRevision(revision),
			Closure:       closure,
		},
	)
	if err != nil {
		t.Fatalf("SealProjectGraphSnapshotBasis(): %v", err)
	}
	return basis
}

func readSetHeadState(
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

func readSetProject(t *testing.T, raw string) projectidentity.ProjectID {
	t.Helper()
	project, err := projectidentity.ParseProjectID(raw)
	if err != nil {
		t.Fatalf("ParseProjectID(%q): %v", raw, err)
	}
	return project
}

func readSetDigest(t *testing.T, digit string) typedmemory.SHA256Digest {
	t.Helper()
	digest, err := typedmemory.NewSHA256Digest(
		"sha256:" + strings.Repeat(digit, 64),
	)
	if err != nil {
		t.Fatalf("NewSHA256Digest(): %v", err)
	}
	return digest
}
