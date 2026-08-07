package p13acceptance

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	p13ChildEnvironmentKey           = "HAFT_P13_CHILD"
	p13RunConsolidatedEnvironmentKey = "HAFT_P13_RUN_CONSOLIDATED"
	acceptanceEvidenceSchema         = "haft.p13.acceptance-evidence/v3"
	suiteProvenanceExecuted          = "executed"
	suiteProvenanceImported          = "imported"
	outputTailLimit                  = 16 * 1024
	gofmtChunkSize                   = 160
)

type consolidatedRunDisposition string

const (
	consolidatedRunChildSkip    consolidatedRunDisposition = "child_skip"
	consolidatedRunOrdinarySkip consolidatedRunDisposition = "ordinary_skip"
	consolidatedRunExecute      consolidatedRunDisposition = "execute"
	consolidatedRunInvalid      consolidatedRunDisposition = "invalid"
)

type suiteRuntimeInputs struct {
	GoPackages              []string
	GoPackageDigest         string
	ExcludedGoPackages      []string
	ExcludedGoPackageDigest string
	GoFiles                 []string
	GoFileDigest            string
}

type goPackagePartition struct {
	Included []string
	Excluded []string
}

type suiteExecution struct {
	Evidence      suiteEvidence
	PassedAnchors map[string]struct{}
}

type suiteEvidence struct {
	ID                        string              `json:"id"`
	Kind                      string              `json:"kind"`
	Status                    string              `json:"status"`
	Provenance                string              `json:"provenance"`
	DependencyDigest          string              `json:"dependency_digest"`
	DurationMillis            int64               `json:"duration_millis"`
	InvocationCount           int                 `json:"invocation_count"`
	InvocationDigest          string              `json:"invocation_digest"`
	InputDigest               string              `json:"input_digest,omitempty"`
	OutputDigest              string              `json:"output_digest"`
	OutputBytes               int64               `json:"output_bytes"`
	PassedAnchors             []string            `json:"passed_anchors,omitempty"`
	Invocations               []commandInvocation `json:"invocations"`
	PreIdentityDigest         string              `json:"pre_identity_digest"`
	PostIdentityDigest        string              `json:"post_identity_digest"`
	IdentityUnchanged         bool                `json:"identity_unchanged"`
	ImportedFromCarrierPath   string              `json:"imported_from_carrier_path,omitempty"`
	ImportedFromCarrierDigest string              `json:"imported_from_carrier_digest,omitempty"`
	Failure                   string              `json:"failure,omitempty"`
	FailureOutputTail         string              `json:"failure_output_tail,omitempty"`
}

type gateEvidence struct {
	ID      string           `json:"id"`
	Status  string           `json:"status"`
	Suites  []string         `json:"suites"`
	Anchors []anchorEvidence `json:"anchors"`
}

type anchorEvidence struct {
	Key           string   `json:"key"`
	PassedSuites  []string `json:"passed_suites"`
	MissingSuites []string `json:"missing_suites,omitempty"`
}

type acceptanceEvidence struct {
	Schema                  string             `json:"schema"`
	Status                  string             `json:"status"`
	ReleaseClaim            bool               `json:"release_claim"`
	ResultSemantics         string             `json:"result_semantics"`
	ConsolidatedCommand     string             `json:"consolidated_command"`
	StartedAt               string             `json:"started_at"`
	FinishedAt              string             `json:"finished_at"`
	FreshnessBoundary       string             `json:"freshness_boundary"`
	CarrierPath             string             `json:"carrier_path"`
	ManifestDigest          string             `json:"manifest_digest"`
	IdentityDigest          string             `json:"identity_digest"`
	IdentityUnchanged       bool               `json:"identity_unchanged"`
	ExcludedGoPackages      []string           `json:"excluded_go_packages"`
	ExcludedGoPackageDigest string             `json:"excluded_go_package_digest"`
	StartIdentity           acceptanceIdentity `json:"start_identity"`
	EndIdentity             acceptanceIdentity `json:"end_identity"`
	Suites                  []suiteEvidence    `json:"suites"`
	Gates                   []gateEvidence     `json:"gates"`
	Waivers                 []manifestWaiver   `json:"waivers"`
	Failure                 string             `json:"failure,omitempty"`
}

type commandInvocation struct {
	Program          string   `json:"program"`
	Args             []string `json:"args"`
	WorkingDirectory string   `json:"working_directory"`
}

type suiteCapture struct {
	stdout      *outputObserver
	stderr      *outputObserver
	invocations []commandInvocation
}

type outputObserver struct {
	mu              sync.Mutex
	hash            hash.Hash
	bytes           int64
	tail            []byte
	pending         []byte
	requiredAnchors map[string]struct{}
	passedAnchors   map[string]struct{}
	failedEvents    []string
	failedEventSet  map[string]struct{}
}

type goTestEvent struct {
	Action  string `json:"Action"`
	Package string `json:"Package"`
	Test    string `json:"Test"`
}

func TestPendingFinalSourceBlocksConsolidatedExecution(t *testing.T) {
	manifest := acceptanceManifest{
		Status: manifestStatusPendingFinalSource,
		FreezeInput: freezeInputSpec{
			Posture: freezePosturePendingFinalSource,
		},
	}
	err := validateManifestExecutionState(manifest)
	if err == nil {
		t.Fatal("missing final source unexpectedly permits P13 execution")
	}
}

func TestGoPackagePartitionAccountsForEveryExcludedPackage(t *testing.T) {
	packages := []string{
		modulePath + "/internal/typedmemory",
		modulePath + "/desktop",
		modulePath + "/internal/desktop/bridge",
	}
	partition := partitionGoPackages(packages)
	wantIncluded := []string{modulePath + "/internal/typedmemory"}
	wantExcluded := []string{
		modulePath + "/desktop",
		modulePath + "/internal/desktop/bridge",
	}
	if !slices.Equal(partition.Included, wantIncluded) {
		t.Fatalf("included packages = %v, want %v", partition.Included, wantIncluded)
	}
	if !slices.Equal(partition.Excluded, wantExcluded) {
		t.Fatalf("excluded packages = %v, want %v", partition.Excluded, wantExcluded)
	}
	if err := validateExcludedGoPackages(partition.Excluded, []string{}); err == nil {
		t.Fatal("non-empty excluded package accounting unexpectedly passed")
	}
}

func TestCriticalRaceInvocationIsExplicitAndResourceBounded(t *testing.T) {
	suite := suiteSpec{
		ID:   "go_race",
		Kind: "go_test_race_critical",
		GoRaceCases: []goRaceCase{
			{
				Package: modulePath + "/internal/typedmemorystore",
				Tests:   []string{"TestConcurrentWriters"},
			},
		},
		GoPackageParallelism: 1,
		GoTestProcs:          2,
		TimeoutSeconds:       10800,
	}
	got := criticalGoRaceTestArgs(suite, suite.GoRaceCases[0])
	want := []string{
		"test",
		"-json",
		"-count=1",
		"-timeout=10800s",
		"-race",
		"-p=1",
		"-cpu=2",
		"-run=^(TestConcurrentWriters)$",
		modulePath + "/internal/typedmemorystore",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("critical race args = %v, want %v", got, want)
	}
	if slices.Contains(got, "./...") {
		t.Fatal("critical race invocation unexpectedly expands to the repository")
	}
	normal := normalGoTestArgs(
		[]string{modulePath + "/internal/typedmemory"},
		7200,
	)
	wantNormal := []string{
		"test",
		"-json",
		"-count=1",
		"-timeout=7200s",
		modulePath + "/internal/typedmemory",
	}
	if !slices.Equal(normal, wantNormal) {
		t.Fatalf("normal Go args = %v, want %v", normal, wantNormal)
	}
}

func TestFailureTailRetainsGoTestFailureBeyondOutputTail(t *testing.T) {
	capture := newSuiteCapture(nil)
	failedTest := modulePath + "/internal/cli::TestBrokenInterface"
	failureEvent := []byte(
		`{"Action":"fail","Package":"` +
			modulePath +
			`/internal/cli","Test":"TestBrokenInterface"}` +
			"\n",
	)
	if _, err := capture.stdout.Write(failureEvent); err != nil {
		t.Fatal(err)
	}
	noise := bytes.Repeat([]byte("x"), outputTailLimit+1)
	if _, err := capture.stdout.Write(noise); err != nil {
		t.Fatal(err)
	}
	capture.finish()
	tail := capture.failureTail(errors.New("go test failed"))
	if !strings.Contains(tail, failedTest) {
		t.Fatalf("failure tail lost exact failed test:\n%s", tail)
	}
}

func TestManifestSuiteAnchorSetsDoNotBroadenRaceCoverage(t *testing.T) {
	raceAnchor := testAnchor{
		Package: modulePath + "/internal/typedmemorystore",
		Test:    "TestConcurrentWriters",
	}
	normalOnlyAnchor := testAnchor{
		Package: modulePath + "/internal/fpf",
		Test:    "TestSourceProjection",
	}
	gates := []gateSpec{
		{
			ID:       "G5",
			SuiteIDs: []string{"go_normal", "go_race"},
			Anchors:  []testAnchor{raceAnchor, normalOnlyAnchor},
		},
		{
			ID:       "G1",
			SuiteIDs: []string{"go_normal"},
			Anchors:  []testAnchor{normalOnlyAnchor},
		},
	}
	suites := []suiteSpec{
		{ID: "go_normal", Kind: "go_test_all_non_desktop"},
		{
			ID:   "go_race",
			Kind: "go_test_race_critical",
			GoRaceCases: []goRaceCase{
				{
					Package: raceAnchor.Package,
					Tests:   []string{"TestConcurrentWriters"},
				},
			},
		},
	}
	anchors := manifestSuiteAnchorSets(suites, gates)
	if _, found := anchors["go_race"][anchorKey(raceAnchor)]; !found {
		t.Fatal("critical race anchor is absent from its owning suite")
	}
	if _, found := anchors["go_race"][anchorKey(normalOnlyAnchor)]; found {
		t.Fatal("normal-only anchor leaked into critical race coverage")
	}
}

func TestCriticalRaceSuiteDoesNotClaimOutOfScopeAnchor(t *testing.T) {
	suite := suiteSpec{
		Kind: "go_test_race_critical",
		GoRaceCases: []goRaceCase{
			{
				Package: modulePath + "/internal/typedmemorystore",
				Tests:   []string{"TestConcurrentWriters"},
			},
		},
	}
	inScope := testAnchor{
		Package: modulePath + "/internal/typedmemorystore",
		Test:    "TestConcurrentWriters",
	}
	outOfScope := testAnchor{
		Package: modulePath + "/internal/fpf",
		Test:    "TestSourceProjection",
	}
	if !suiteCoversAnchor(suite, inScope) {
		t.Fatal("critical race suite rejected an in-scope anchor")
	}
	if suiteCoversAnchor(suite, outOfScope) {
		t.Fatal("critical race suite claimed an out-of-scope anchor")
	}
}

func TestCriticalRaceRuntimeScopeRejectsUnavailablePackage(t *testing.T) {
	available := []string{modulePath + "/internal/typedmemorystore"}
	selected := []goRaceCase{
		{
			Package: modulePath + "/internal/missing",
			Tests:   []string{"TestMissing"},
		},
	}
	if err := validateCriticalRaceRuntimeScope(available, selected); err == nil {
		t.Fatal("critical race scope accepted a package outside the current Go closure")
	}
}

func TestStableExecutionBasisRejectsManifestOrIdentityDrift(t *testing.T) {
	identity := acceptanceIdentity{Digest: "sha256:stable"}
	if err := validateStableExecutionBasis(
		[]byte("manifest"),
		[]byte("manifest"),
		identity,
		identity,
	); err != nil {
		t.Fatalf("stable execution basis rejected: %v", err)
	}
	changedIdentity := identity
	changedIdentity.Digest = "sha256:changed"
	if err := validateStableExecutionBasis(
		[]byte("manifest"),
		[]byte("manifest"),
		identity,
		changedIdentity,
	); err == nil {
		t.Fatal("identity drift during suite-input discovery unexpectedly passed")
	}
	if err := validateStableExecutionBasis(
		[]byte("manifest"),
		[]byte("changed-manifest"),
		identity,
		identity,
	); err == nil {
		t.Fatal("manifest drift during suite-input discovery unexpectedly passed")
	}
}

func TestSuiteIdentityBoundaryRejectsDrift(t *testing.T) {
	want := "sha256:stable"
	stable := acceptanceIdentity{Digest: want}
	if !suiteIdentityMatches(want, stable, nil) {
		t.Fatal("stable suite identity boundary rejected")
	}
	changed := acceptanceIdentity{Digest: "sha256:changed"}
	if suiteIdentityMatches(want, changed, nil) {
		t.Fatal("suite identity drift unexpectedly passed")
	}
	if suiteIdentityMatches(want, stable, errors.New("capture failed")) {
		t.Fatal("suite identity capture failure unexpectedly passed")
	}
}

func TestAtomicEvidenceCarrierIsNoClobberAndRereadVerified(t *testing.T) {
	root := t.TempDir()
	evidence := acceptanceEvidence{
		Schema:              acceptanceEvidenceSchema,
		Status:              "passed",
		ConsolidatedCommand: consolidatedCommand,
		StartedAt:           "2026-07-21T20:00:00Z",
		FinishedAt:          "2026-07-21T20:00:01Z",
		IdentityDigest:      "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}
	carrierPath, err := acceptanceEvidenceCarrierPath(evidence)
	if err != nil {
		t.Fatal(err)
	}
	evidence.CarrierPath = carrierPath
	encoded, err := json.Marshal(evidence)
	if err != nil {
		t.Fatal(err)
	}
	path, digest, err := persistAcceptanceEvidence(root, evidence, encoded)
	if err != nil {
		t.Fatalf("persist evidence carrier: %v", err)
	}
	if path != carrierPath || digest != sha256Prefixed(encoded) {
		t.Fatalf("carrier result = %q %q", path, digest)
	}
	if _, _, err := persistAcceptanceEvidence(root, evidence, encoded); err == nil {
		t.Fatal("second evidence publication replaced an existing carrier")
	}
}

func TestP13ConsolidatedRunDisposition(t *testing.T) {
	cases := []struct {
		name      string
		child     string
		requested string
		want      consolidatedRunDisposition
	}{
		{
			name: "ordinary test skips",
			want: consolidatedRunOrdinarySkip,
		},
		{
			name:      "explicit capability executes",
			requested: "1",
			want:      consolidatedRunExecute,
		},
		{
			name:      "child marker takes precedence",
			child:     "1",
			requested: "1",
			want:      consolidatedRunChildSkip,
		},
		{
			name:      "invalid capability fails closed",
			requested: "true",
			want:      consolidatedRunInvalid,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := p13ConsolidatedRunDisposition(
				testCase.child,
				testCase.requested,
			)
			if got != testCase.want {
				t.Fatalf("disposition = %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestP13ChildEnvironmentDoesNotInheritConsolidatedCapability(t *testing.T) {
	t.Setenv(p13RunConsolidatedEnvironmentKey, "1")
	environment := acceptanceEnvironment(true)
	childValues := environmentValues(environment, p13ChildEnvironmentKey)
	requestValues := environmentValues(
		environment,
		p13RunConsolidatedEnvironmentKey,
	)
	if !slices.Equal(childValues, []string{"1"}) {
		t.Fatalf("P13 child environment values = %v, want [1]", childValues)
	}
	if len(requestValues) != 0 {
		t.Fatalf(
			"P13 child inherited consolidated capability values %v",
			requestValues,
		)
	}
}

func TestP13ConsolidatedAcceptance(t *testing.T) {
	disposition := p13ConsolidatedRunDisposition(
		os.Getenv(p13ChildEnvironmentKey),
		os.Getenv(p13RunConsolidatedEnvironmentKey),
	)
	switch disposition {
	case consolidatedRunChildSkip:
		t.Skip("P13 child suite does not recursively launch consolidation")
	case consolidatedRunOrdinarySkip:
		t.Skip("set HAFT_P13_RUN_CONSOLIDATED=1 for the explicit P13 run")
	case consolidatedRunInvalid:
		t.Fatalf(
			"%s must be empty or exactly 1",
			p13RunConsolidatedEnvironmentKey,
		)
	case consolidatedRunExecute:
	default:
		t.Fatalf("unknown P13 consolidated disposition %q", disposition)
	}
	runStarted := time.Now().UTC()
	root, err := repositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	manifest, manifestBytes, err := loadAcceptanceManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateAcceptanceManifest(root, manifest); err != nil {
		t.Fatal(err)
	}
	if err := validateManifestExecutionState(manifest); err != nil {
		t.Fatal(err)
	}
	identityBeforeDiscovery, err := captureAcceptanceIdentity(root, manifest.Identity)
	if err != nil {
		t.Fatalf("capture P13 identity before suite-input discovery: %v", err)
	}
	inputs, err := prepareSuiteRuntimeInputs(root, manifest.Identity)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateExcludedGoPackages(
		inputs.ExcludedGoPackages,
		manifest.Identity.RequiredExcludedGoPackages,
	); err != nil {
		t.Fatal(err)
	}
	stableManifest, stableManifestBytes, err := loadAcceptanceManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateAcceptanceManifest(root, stableManifest); err != nil {
		t.Fatal(err)
	}
	if err := validateManifestExecutionState(stableManifest); err != nil {
		t.Fatal(err)
	}
	startIdentity, err := captureAcceptanceIdentity(root, stableManifest.Identity)
	if err != nil {
		t.Fatalf("capture frozen P13 start identity: %v", err)
	}
	if err := validateStableExecutionBasis(
		manifestBytes,
		stableManifestBytes,
		identityBeforeDiscovery,
		startIdentity,
	); err != nil {
		t.Fatal(err)
	}
	manifest = stableManifest
	manifestBytes = stableManifestBytes
	if err := validateCapturedFrozenBasis(manifest.FreezeInput, startIdentity); err != nil {
		t.Fatal(err)
	}
	dependencyDigests, err := captureSuiteDependencyDigests(
		root,
		manifest,
		inputs,
		startIdentity,
	)
	if err != nil {
		t.Fatalf("capture P13 suite dependency closures: %v", err)
	}
	reuseSource, err := loadOptionalSuiteReuseSource(root, manifest)
	if err != nil {
		t.Fatalf("load optional P13 suite reuse source: %v", err)
	}
	requiredAnchors := manifestSuiteAnchorSets(manifest.Suites, manifest.Gates)
	executions := runAcceptanceSuites(
		root,
		manifest.Suites,
		inputs,
		startIdentity,
		requiredAnchors,
		dependencyDigests,
		reuseSource,
	)
	endIdentity, endErr := captureAcceptanceIdentity(root, manifest.Identity)
	identityUnchanged := endErr == nil && startIdentity.Digest == endIdentity.Digest
	finalizeExecutedSuiteIdentity(
		executions,
		startIdentity,
		endIdentity,
		endErr,
	)
	gates := evaluateGateEvidence(manifest, executions)
	runFinished := time.Now().UTC()
	evidence := buildAcceptanceEvidence(
		manifest,
		manifestBytes,
		startIdentity,
		endIdentity,
		endErr,
		identityUnchanged,
		inputs,
		executions,
		gates,
		runStarted,
		runFinished,
	)
	carrierPath, err := acceptanceEvidenceCarrierPath(evidence)
	if err != nil {
		t.Fatalf("derive P13 acceptance evidence path: %v", err)
	}
	evidence.CarrierPath = carrierPath
	encoded, err := json.Marshal(evidence)
	if err != nil {
		t.Fatalf("encode P13 acceptance evidence: %v", err)
	}
	persistedPath, evidenceDigest, err := persistAcceptanceEvidence(
		root,
		evidence,
		encoded,
	)
	if err != nil {
		t.Fatalf("persist P13 acceptance evidence: %v", err)
	}
	t.Logf(
		"P13_ACCEPTANCE_EVIDENCE path=%s digest=%s json=%s",
		persistedPath,
		evidenceDigest,
		encoded,
	)
	if evidence.Status != "passed" {
		t.FailNow()
	}
}

func validateStableExecutionBasis(
	initialManifest []byte,
	stableManifest []byte,
	before acceptanceIdentity,
	after acceptanceIdentity,
) error {
	if !bytes.Equal(initialManifest, stableManifest) {
		return fmt.Errorf("P13 manifest changed during suite-input discovery")
	}
	if before.Digest == "" || after.Digest == "" || before.Digest != after.Digest {
		return fmt.Errorf("P13 acceptance identity changed during suite-input discovery")
	}
	return nil
}

func prepareSuiteRuntimeInputs(
	root string,
	spec identitySpec,
) (suiteRuntimeInputs, error) {
	goProgram, err := resolveExecutable("go")
	if err != nil {
		return suiteRuntimeInputs{}, err
	}
	packagesRaw, err := runIdentityCommand(root, goProgram, "list", "./...")
	if err != nil {
		return suiteRuntimeInputs{}, err
	}
	packages := splitNonEmptyLines(packagesRaw)
	partition := partitionGoPackages(packages)
	if len(partition.Included) == 0 {
		return suiteRuntimeInputs{}, fmt.Errorf("Go package set is empty")
	}
	relevantPaths, err := collectRelevantPaths(root, spec)
	if err != nil {
		return suiteRuntimeInputs{}, err
	}
	goFiles := make([]string, 0)
	for _, relative := range relevantPaths {
		if filepath.Ext(relative) == ".go" {
			goFiles = append(goFiles, filepath.ToSlash(relative))
		}
	}
	if len(goFiles) == 0 {
		return suiteRuntimeInputs{}, fmt.Errorf("gofmt input set is empty")
	}
	return suiteRuntimeInputs{
		GoPackages:              partition.Included,
		GoPackageDigest:         digestStringList(partition.Included),
		ExcludedGoPackages:      partition.Excluded,
		ExcludedGoPackageDigest: digestStringList(partition.Excluded),
		GoFiles:                 goFiles,
		GoFileDigest:            digestStringList(goFiles),
	}, nil
}

func splitNonEmptyLines(raw []byte) []string {
	lines := strings.Split(normalizedCommandText(raw), "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func partitionGoPackages(packages []string) goPackagePartition {
	partition := goPackagePartition{
		Included: make([]string, 0, len(packages)),
		Excluded: make([]string, 0),
	}
	for _, packagePath := range packages {
		if strings.Contains(packagePath, "/desktop") {
			partition.Excluded = append(partition.Excluded, packagePath)
			continue
		}
		partition.Included = append(partition.Included, packagePath)
	}
	slices.Sort(partition.Included)
	slices.Sort(partition.Excluded)
	return partition
}

func validateExcludedGoPackages(observed []string, required []string) error {
	if !slices.Equal(observed, required) {
		return fmt.Errorf(
			"P13 excluded Go packages = %v, want exact empty accounting %v",
			observed,
			required,
		)
	}
	return nil
}

func manifestSuiteAnchorSets(
	suites []suiteSpec,
	gates []gateSpec,
) map[string]map[string]struct{} {
	suiteByID := make(map[string]suiteSpec, len(suites))
	for _, suite := range suites {
		suiteByID[suite.ID] = suite
	}
	anchorsBySuite := make(map[string]map[string]struct{})
	for _, gate := range gates {
		for _, suiteID := range gate.SuiteIDs {
			suite := suiteByID[suiteID]
			if !isGoTestSuite(suite.Kind) {
				continue
			}
			anchors := anchorsBySuite[suiteID]
			if anchors == nil {
				anchors = make(map[string]struct{})
				anchorsBySuite[suiteID] = anchors
			}
			for _, anchor := range gate.Anchors {
				if !suiteCoversAnchor(suite, anchor) {
					continue
				}
				anchors[anchorKey(anchor)] = struct{}{}
			}
		}
	}
	return anchorsBySuite
}

func runAcceptanceSuites(
	root string,
	suites []suiteSpec,
	inputs suiteRuntimeInputs,
	identity acceptanceIdentity,
	requiredAnchors map[string]map[string]struct{},
	dependencyDigests map[string]string,
	reuseSource suiteReuseSource,
) []suiteExecution {
	executions := make([]suiteExecution, 0, len(suites))
	for _, suite := range suites {
		dependencyDigest := dependencyDigests[suite.ID]
		suiteAnchors := requiredAnchors[suite.ID]
		imported, reusable := importSuiteExecution(
			suite,
			dependencyDigest,
			suiteAnchors,
			reuseSource,
		)
		if reusable {
			executions = append(executions, imported)
			continue
		}
		execution := runAcceptanceSuite(
			root,
			suite,
			inputs,
			identity,
			suiteAnchors,
		)
		execution.Evidence.Provenance = suiteProvenanceExecuted
		execution.Evidence.DependencyDigest = dependencyDigest
		execution.Evidence.PreIdentityDigest = identity.Digest
		executions = append(executions, execution)
	}
	return executions
}

func finalizeExecutedSuiteIdentity(
	executions []suiteExecution,
	start acceptanceIdentity,
	end acceptanceIdentity,
	endErr error,
) {
	for index := range executions {
		evidence := &executions[index].Evidence
		if evidence.Provenance != suiteProvenanceExecuted {
			continue
		}
		evidence.PostIdentityDigest = end.Digest
		evidence.IdentityUnchanged = suiteIdentityMatches(
			start.Digest,
			end,
			endErr,
		)
		if evidence.IdentityUnchanged {
			continue
		}
		evidence.Status = "fail"
		evidence.Failure = appendFailure(
			evidence.Failure,
			identityBoundaryFailure(
				"final",
				start.Digest,
				end.Digest,
				endErr,
			),
		)
	}
}

func suiteIdentityMatches(
	want string,
	observed acceptanceIdentity,
	captureErr error,
) bool {
	return captureErr == nil && want != "" && observed.Digest == want
}

func identityBlockedSuiteExecution(
	suite suiteSpec,
	boundary string,
	want string,
	observed string,
	err error,
) suiteExecution {
	failure := identityBoundaryFailure(boundary, want, observed, err)
	return suiteExecution{
		Evidence: suiteEvidence{
			ID:                 suite.ID,
			Kind:               suite.Kind,
			Status:             "fail",
			InvocationCount:    0,
			InvocationDigest:   digestStringList([]string{}),
			OutputDigest:       sha256Prefixed(nil),
			Invocations:        []commandInvocation{},
			PreIdentityDigest:  observed,
			PostIdentityDigest: observed,
			IdentityUnchanged:  false,
			Failure:            failure,
		},
		PassedAnchors: map[string]struct{}{},
	}
}

func identityBoundaryFailure(
	boundary string,
	want string,
	observed string,
	err error,
) string {
	if err != nil {
		return fmt.Sprintf(
			"capture P13 %s-suite identity: %v",
			boundary,
			err,
		)
	}
	return fmt.Sprintf(
		"P13 %s-suite identity = %q, want %q",
		boundary,
		observed,
		want,
	)
}

func appendFailure(current string, added string) string {
	parts := []string{current, added}
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			result = append(result, part)
		}
	}
	return strings.Join(result, "; ")
}

func runAcceptanceSuite(
	root string,
	suite suiteSpec,
	inputs suiteRuntimeInputs,
	identity acceptanceIdentity,
	requiredAnchors map[string]struct{},
) suiteExecution {
	capture := newSuiteCapture(requiredAnchors)
	ctx, cancel := context.WithTimeout(
		context.Background(),
		time.Duration(suite.TimeoutSeconds)*time.Second,
	)
	defer cancel()
	started := time.Now()
	inputDigest, err := executeSuite(
		ctx,
		root,
		suite,
		inputs,
		identity,
		capture,
	)
	duration := time.Since(started)
	capture.finish()
	passedAnchors := capture.passedAnchorSet()
	status := "pass"
	failure := ""
	if err != nil {
		status = "fail"
		failure = err.Error()
	}
	evidence := suiteEvidence{
		ID:                suite.ID,
		Kind:              suite.Kind,
		Status:            status,
		DurationMillis:    duration.Milliseconds(),
		InvocationCount:   len(capture.invocations),
		InvocationDigest:  capture.invocationDigest(),
		InputDigest:       inputDigest,
		OutputDigest:      capture.outputDigest(),
		OutputBytes:       capture.outputBytes(),
		PassedAnchors:     sortedSetKeys(passedAnchors),
		Invocations:       cloneCommandInvocations(capture.invocations),
		Failure:           failure,
		FailureOutputTail: capture.failureTail(err),
	}
	return suiteExecution{
		Evidence:      evidence,
		PassedAnchors: passedAnchors,
	}
}

func cloneCommandInvocations(
	values []commandInvocation,
) []commandInvocation {
	cloned := make([]commandInvocation, 0, len(values))
	for _, value := range values {
		cloned = append(cloned, commandInvocation{
			Program:          value.Program,
			Args:             slices.Clone(value.Args),
			WorkingDirectory: value.WorkingDirectory,
		})
	}
	return cloned
}

func executeSuite(
	ctx context.Context,
	root string,
	suite suiteSpec,
	inputs suiteRuntimeInputs,
	identity acceptanceIdentity,
	capture *suiteCapture,
) (string, error) {
	switch suite.Kind {
	case "exec":
		return "", executeManifestSuite(ctx, root, suite, capture)
	case "fpf_index_verify":
		return identity.FPF.Source.Digest, executeFPFVerification(
			ctx,
			root,
			identity.FPF.Head,
			capture,
		)
	case "go_test_all_non_desktop":
		return inputs.GoPackageDigest, executeNormalGoTest(
			ctx,
			root,
			inputs.GoPackages,
			suite.TimeoutSeconds,
			capture,
		)
	case "go_test_race_critical":
		if err := validateCriticalRaceRuntimeScope(
			inputs.GoPackages,
			suite.GoRaceCases,
		); err != nil {
			return "", err
		}
		inputDigest, err := digestCanonicalJSON(suite.GoRaceCases)
		if err != nil {
			return "", fmt.Errorf("digest P13 critical-race cases: %w", err)
		}
		return inputDigest, executeCriticalGoRaceTest(
			ctx,
			root,
			suite,
			capture,
		)
	case "go_vet_all_non_desktop":
		return inputs.GoPackageDigest, executeGoVet(
			ctx,
			root,
			inputs.GoPackages,
			capture,
		)
	case "gofmt_check":
		return inputs.GoFileDigest, executeGofmtCheck(
			ctx,
			root,
			inputs.GoFiles,
			capture,
		)
	default:
		return "", fmt.Errorf("unsupported suite kind %q", suite.Kind)
	}
}

func executeManifestSuite(
	ctx context.Context,
	root string,
	suite suiteSpec,
	capture *suiteCapture,
) error {
	program, err := resolveExecutable(suite.Program)
	if err != nil {
		return err
	}
	directory := filepath.Join(root, filepath.FromSlash(suite.WorkingDirectory))
	return capture.run(ctx, directory, program, suite.Args)
}

func executeFPFVerification(
	ctx context.Context,
	root string,
	fpfHead string,
	capture *suiteCapture,
) error {
	goProgram, err := resolveExecutable("go")
	if err != nil {
		return err
	}
	args := []string{
		"run",
		"./cmd/indexer",
		"-verify",
		"internal/cli/fpf.db",
		"data/FPF/FPF-Spec.md",
		fpfHead,
	}
	return capture.run(ctx, root, goProgram, args)
}

func executeNormalGoTest(
	ctx context.Context,
	root string,
	packages []string,
	timeoutSeconds int,
	capture *suiteCapture,
) error {
	goProgram, err := resolveExecutable("go")
	if err != nil {
		return err
	}
	args := normalGoTestArgs(packages, timeoutSeconds)
	return capture.run(ctx, root, goProgram, args)
}

func executeCriticalGoRaceTest(
	ctx context.Context,
	root string,
	suite suiteSpec,
	capture *suiteCapture,
) error {
	goProgram, err := resolveExecutable("go")
	if err != nil {
		return err
	}
	for _, raceCase := range suite.GoRaceCases {
		args := criticalGoRaceTestArgs(suite, raceCase)
		if err := capture.run(ctx, root, goProgram, args); err != nil {
			return err
		}
	}
	return nil
}

func normalGoTestArgs(packages []string, timeoutSeconds int) []string {
	args := []string{
		"test",
		"-json",
		"-count=1",
		fmt.Sprintf("-timeout=%ds", timeoutSeconds),
	}
	args = append(args, packages...)
	return args
}

func criticalGoRaceTestArgs(
	suite suiteSpec,
	raceCase goRaceCase,
) []string {
	testPattern := exactGoTestPattern(raceCase.Tests)
	args := []string{
		"test",
		"-json",
		"-count=1",
		fmt.Sprintf("-timeout=%ds", suite.TimeoutSeconds),
		"-race",
		fmt.Sprintf("-p=%d", suite.GoPackageParallelism),
		fmt.Sprintf("-cpu=%d", suite.GoTestProcs),
		"-run=" + testPattern,
		raceCase.Package,
	}
	return args
}

func exactGoTestPattern(testNames []string) string {
	escaped := make([]string, 0, len(testNames))
	for _, testName := range testNames {
		escaped = append(escaped, regexp.QuoteMeta(testName))
	}
	return "^(" + strings.Join(escaped, "|") + ")$"
}

func validateCriticalRaceRuntimeScope(
	available []string,
	selected []goRaceCase,
) error {
	availableSet := make(map[string]struct{}, len(available))
	for _, packagePath := range available {
		availableSet[packagePath] = struct{}{}
	}
	for _, raceCase := range selected {
		if _, found := availableSet[raceCase.Package]; !found {
			return fmt.Errorf(
				"P13 critical-race package %q is absent from the current Go closure",
				raceCase.Package,
			)
		}
	}
	return nil
}

func executeGoVet(
	ctx context.Context,
	root string,
	packages []string,
	capture *suiteCapture,
) error {
	goProgram, err := resolveExecutable("go")
	if err != nil {
		return err
	}
	args := []string{"vet"}
	args = append(args, packages...)
	return capture.run(ctx, root, goProgram, args)
}

func executeGofmtCheck(
	ctx context.Context,
	root string,
	goFiles []string,
	capture *suiteCapture,
) error {
	goProgram, err := resolveExecutable("go")
	if err != nil {
		return err
	}
	gofmtProgram := filepath.Join(filepath.Dir(goProgram), "gofmt")
	if !isExecutableFile(gofmtProgram) {
		return fmt.Errorf("resolve gofmt executable %q", gofmtProgram)
	}
	for offset := 0; offset < len(goFiles); offset += gofmtChunkSize {
		limit := min(offset+gofmtChunkSize, len(goFiles))
		args := []string{"-l"}
		args = append(args, goFiles[offset:limit]...)
		if err := capture.run(ctx, root, gofmtProgram, args); err != nil {
			return err
		}
	}
	if capture.stdout.byteCount() != 0 {
		return fmt.Errorf("gofmt reported unformatted Go files")
	}
	return nil
}

func newSuiteCapture(requiredAnchors map[string]struct{}) *suiteCapture {
	return &suiteCapture{
		stdout: newOutputObserver(requiredAnchors),
		stderr: newOutputObserver(nil),
	}
}

func (capture *suiteCapture) run(
	ctx context.Context,
	workingDirectory string,
	program string,
	args []string,
) error {
	invocation := commandInvocation{
		Program:          program,
		Args:             slices.Clone(args),
		WorkingDirectory: workingDirectory,
	}
	capture.invocations = append(capture.invocations, invocation)
	command := exec.CommandContext(ctx, program, args...)
	command.Dir = workingDirectory
	command.Env = acceptanceEnvironment(true)
	command.Stdout = capture.stdout
	command.Stderr = capture.stderr
	err := command.Run()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("suite timed out: %w", ctx.Err())
	}
	if err != nil {
		return fmt.Errorf("command %s failed: %w", filepath.Base(program), err)
	}
	return nil
}

func (capture *suiteCapture) finish() {
	capture.stdout.finish()
	capture.stderr.finish()
}

func (capture *suiteCapture) passedAnchorSet() map[string]struct{} {
	return capture.stdout.passedAnchorSet()
}

func (capture *suiteCapture) invocationDigest() string {
	digest, err := digestCanonicalJSON(capture.invocations)
	if err != nil {
		return "sha256:invalid"
	}
	return digest
}

func (capture *suiteCapture) outputDigest() string {
	coordinates := struct {
		Stdout string `json:"stdout"`
		Stderr string `json:"stderr"`
	}{
		Stdout: capture.stdout.digest(),
		Stderr: capture.stderr.digest(),
	}
	digest, err := digestCanonicalJSON(coordinates)
	if err != nil {
		return "sha256:invalid"
	}
	return digest
}

func (capture *suiteCapture) outputBytes() int64 {
	return capture.stdout.byteCount() + capture.stderr.byteCount()
}

func (capture *suiteCapture) failureTail(err error) string {
	if err == nil {
		return ""
	}
	failures := capture.stdout.failureSummary()
	stdout := capture.stdout.tailText()
	stderr := capture.stderr.tailText()
	parts := make([]string, 0, 3)
	if failures != "" {
		parts = append(parts, "go test failures:\n"+failures)
	}
	parts = append(parts,
		"stdout:\n"+stdout,
		"stderr:\n"+stderr,
	)
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func newOutputObserver(requiredAnchors map[string]struct{}) *outputObserver {
	cloned := make(map[string]struct{}, len(requiredAnchors))
	for key := range requiredAnchors {
		cloned[key] = struct{}{}
	}
	return &outputObserver{
		hash:            sha256.New(),
		requiredAnchors: cloned,
		passedAnchors:   make(map[string]struct{}),
		failedEventSet:  make(map[string]struct{}),
	}
}

func (observer *outputObserver) Write(content []byte) (int, error) {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	written, err := observer.hash.Write(content)
	observer.bytes += int64(written)
	observer.appendTail(content)
	observer.pending = append(observer.pending, content...)
	observer.consumeCompleteLines()
	return written, err
}

func (observer *outputObserver) appendTail(content []byte) {
	observer.tail = append(observer.tail, content...)
	if len(observer.tail) <= outputTailLimit {
		return
	}
	offset := len(observer.tail) - outputTailLimit
	observer.tail = slices.Clone(observer.tail[offset:])
}

func (observer *outputObserver) consumeCompleteLines() {
	for {
		index := bytes.IndexByte(observer.pending, '\n')
		if index < 0 {
			return
		}
		line := slices.Clone(observer.pending[:index])
		observer.pending = observer.pending[index+1:]
		observer.observeGoTestLine(line)
	}
}

func (observer *outputObserver) observeGoTestLine(line []byte) {
	if len(line) == 0 {
		return
	}
	event := goTestEvent{}
	if err := json.Unmarshal(line, &event); err != nil {
		return
	}
	if event.Action == "fail" && event.Package != "" {
		observer.recordGoTestFailure(event)
	}
	if event.Action != "pass" || event.Package == "" || event.Test == "" {
		return
	}
	key := event.Package + "::" + event.Test
	if _, required := observer.requiredAnchors[key]; required {
		observer.passedAnchors[key] = struct{}{}
	}
}

func (observer *outputObserver) recordGoTestFailure(event goTestEvent) {
	key := event.Package
	if event.Test != "" {
		key += "::" + event.Test
	}
	if _, exists := observer.failedEventSet[key]; exists {
		return
	}
	observer.failedEventSet[key] = struct{}{}
	observer.failedEvents = append(observer.failedEvents, key)
}

func (observer *outputObserver) finish() {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	if len(observer.pending) != 0 {
		observer.observeGoTestLine(observer.pending)
		observer.pending = nil
	}
}

func (observer *outputObserver) digest() string {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	return "sha256:" + hex.EncodeToString(observer.hash.Sum(nil))
}

func (observer *outputObserver) byteCount() int64 {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	return observer.bytes
}

func (observer *outputObserver) tailText() string {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	return strings.TrimSpace(string(observer.tail))
}

func (observer *outputObserver) failureSummary() string {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	return strings.Join(observer.failedEvents, "\n")
}

func (observer *outputObserver) passedAnchorSet() map[string]struct{} {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	result := make(map[string]struct{}, len(observer.passedAnchors))
	for key := range observer.passedAnchors {
		result[key] = struct{}{}
	}
	return result
}

func evaluateGateEvidence(
	manifest acceptanceManifest,
	executions []suiteExecution,
) []gateEvidence {
	executionByID := make(map[string]suiteExecution, len(executions))
	for _, execution := range executions {
		executionByID[execution.Evidence.ID] = execution
	}
	suiteByID := make(map[string]suiteSpec, len(manifest.Suites))
	for _, suite := range manifest.Suites {
		suiteByID[suite.ID] = suite
	}
	result := make([]gateEvidence, 0, len(manifest.Gates))
	for _, gate := range manifest.Gates {
		result = append(
			result,
			evaluateOneGate(gate, suiteByID, executionByID),
		)
	}
	return result
}

func evaluateOneGate(
	gate gateSpec,
	suiteByID map[string]suiteSpec,
	executionByID map[string]suiteExecution,
) gateEvidence {
	status := "pass"
	for _, suiteID := range gate.SuiteIDs {
		execution := executionByID[suiteID]
		if execution.Evidence.Status != "pass" {
			status = "fail"
		}
	}
	anchors := make([]anchorEvidence, 0, len(gate.Anchors))
	for _, anchor := range gate.Anchors {
		anchorResult := evaluateOneAnchor(
			anchor,
			gate.SuiteIDs,
			suiteByID,
			executionByID,
		)
		if len(anchorResult.MissingSuites) != 0 {
			status = "fail"
		}
		anchors = append(anchors, anchorResult)
	}
	return gateEvidence{
		ID:      gate.ID,
		Status:  status,
		Suites:  slices.Clone(gate.SuiteIDs),
		Anchors: anchors,
	}
}

func evaluateOneAnchor(
	anchor testAnchor,
	suiteIDs []string,
	suiteByID map[string]suiteSpec,
	executionByID map[string]suiteExecution,
) anchorEvidence {
	key := anchorKey(anchor)
	passed := make([]string, 0)
	missing := make([]string, 0)
	for _, suiteID := range suiteIDs {
		suite := suiteByID[suiteID]
		if !isGoTestSuite(suite.Kind) {
			continue
		}
		if !suiteCoversAnchor(suite, anchor) {
			continue
		}
		execution := executionByID[suiteID]
		if _, found := execution.PassedAnchors[key]; found {
			passed = append(passed, suiteID)
			continue
		}
		missing = append(missing, suiteID)
	}
	return anchorEvidence{
		Key:           key,
		PassedSuites:  passed,
		MissingSuites: missing,
	}
}

func isGoTestSuite(kind string) bool {
	return kind == "go_test_all_non_desktop" ||
		kind == "go_test_race_critical"
}

func suiteCoversAnchor(suite suiteSpec, anchor testAnchor) bool {
	if suite.Kind == "go_test_all_non_desktop" {
		return true
	}
	if suite.Kind != "go_test_race_critical" {
		return false
	}
	for _, raceCase := range suite.GoRaceCases {
		if raceCase.Package != anchor.Package {
			continue
		}
		return slices.Contains(raceCase.Tests, anchor.Test)
	}
	return false
}

func buildAcceptanceEvidence(
	manifest acceptanceManifest,
	manifestBytes []byte,
	startIdentity acceptanceIdentity,
	endIdentity acceptanceIdentity,
	endErr error,
	identityUnchanged bool,
	inputs suiteRuntimeInputs,
	executions []suiteExecution,
	gates []gateEvidence,
	startedAt time.Time,
	finishedAt time.Time,
) acceptanceEvidence {
	status := "passed"
	failureParts := make([]string, 0)
	if endErr != nil {
		status = "failed"
		failureParts = append(
			failureParts,
			"capture frozen P13 end identity: "+endErr.Error(),
		)
	}
	if !identityUnchanged {
		status = "failed"
		failureParts = append(failureParts, "acceptance identity changed during the run")
	}
	suites := make([]suiteEvidence, 0, len(executions))
	for _, execution := range executions {
		suites = append(suites, execution.Evidence)
		if execution.Evidence.Status != "pass" {
			status = "failed"
		}
	}
	for _, gate := range gates {
		if gate.Status != "pass" {
			status = "failed"
		}
	}
	return acceptanceEvidence{
		Schema:                  acceptanceEvidenceSchema,
		Status:                  status,
		ReleaseClaim:            false,
		ResultSemantics:         manifest.ResultSemantics,
		ConsolidatedCommand:     manifest.SingleCommand,
		StartedAt:               startedAt.UTC().Format(time.RFC3339Nano),
		FinishedAt:              finishedAt.UTC().Format(time.RFC3339Nano),
		FreshnessBoundary:       "valid only while the recorded acceptance identity and every suite dependency closure remain unchanged",
		ManifestDigest:          sha256Prefixed(manifestBytes),
		IdentityDigest:          startIdentity.Digest,
		IdentityUnchanged:       identityUnchanged,
		ExcludedGoPackages:      slices.Clone(inputs.ExcludedGoPackages),
		ExcludedGoPackageDigest: inputs.ExcludedGoPackageDigest,
		StartIdentity:           startIdentity,
		EndIdentity:             endIdentity,
		Suites:                  suites,
		Gates:                   gates,
		Waivers:                 slices.Clone(manifest.Waivers),
		Failure:                 strings.Join(failureParts, "; "),
	}
}

func acceptanceEvidenceCarrierPath(
	evidence acceptanceEvidence,
) (string, error) {
	finished, err := time.Parse(time.RFC3339Nano, evidence.FinishedAt)
	if err != nil || finished.UTC().Format(time.RFC3339Nano) != evidence.FinishedAt {
		return "", fmt.Errorf("P13 evidence finish time is not canonical UTC RFC3339Nano")
	}
	if !strings.HasPrefix(evidence.IdentityDigest, "sha256:") {
		return "", fmt.Errorf("P13 evidence identity digest is invalid")
	}
	identity := strings.TrimPrefix(evidence.IdentityDigest, "sha256:")
	decodedIdentity, decodeErr := hex.DecodeString(identity)
	if decodeErr != nil || len(decodedIdentity) != sha256.Size {
		return "", fmt.Errorf("P13 evidence identity digest is invalid")
	}
	name := fmt.Sprintf(
		"p13-acceptance-%s-%s.json",
		finished.UTC().Format("20060102T150405.000000000Z"),
		identity[:16],
	)
	return filepath.ToSlash(filepath.Join(".context", "p13", name)), nil
}

func persistAcceptanceEvidence(
	root string,
	evidence acceptanceEvidence,
	canonical []byte,
) (string, string, error) {
	wantPath, err := acceptanceEvidenceCarrierPath(evidence)
	if err != nil {
		return "", "", err
	}
	if evidence.CarrierPath != wantPath {
		return "", "", fmt.Errorf(
			"P13 evidence carrier path = %q, want %q",
			evidence.CarrierPath,
			wantPath,
		)
	}
	reencoded, err := json.Marshal(evidence)
	if err != nil {
		return "", "", fmt.Errorf("encode canonical P13 evidence: %w", err)
	}
	if !bytes.Equal(reencoded, canonical) {
		return "", "", fmt.Errorf("P13 evidence bytes are not canonical")
	}
	rootHandle, err := os.OpenRoot(root)
	if err != nil {
		return "", "", fmt.Errorf("open P13 evidence repository root: %w", err)
	}
	defer rootHandle.Close()
	directory := filepath.ToSlash(filepath.Join(".context", "p13"))
	if err := rootHandle.MkdirAll(directory, 0o700); err != nil {
		return "", "", fmt.Errorf("create P13 evidence directory: %w", err)
	}
	if err := syncEvidenceDirectory(rootHandle, ".", "repository root"); err != nil {
		return "", "", err
	}
	if err := syncEvidenceDirectory(rootHandle, ".context", "context directory"); err != nil {
		return "", "", err
	}
	evidenceRoot, err := rootHandle.OpenRoot(directory)
	if err != nil {
		return "", "", fmt.Errorf("open P13 evidence directory: %w", err)
	}
	defer evidenceRoot.Close()
	finalName := filepath.Base(filepath.FromSlash(wantPath))
	temporaryName, temporary, err := createEvidenceTemporary(evidenceRoot, finalName)
	if err != nil {
		return "", "", err
	}
	temporaryPresent := true
	defer func() {
		if temporaryPresent {
			_ = evidenceRoot.Remove(temporaryName)
		}
	}()
	if _, err := temporary.Write(canonical); err != nil {
		temporary.Close()
		return "", "", fmt.Errorf("write P13 evidence temporary: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return "", "", fmt.Errorf("sync P13 evidence temporary: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return "", "", fmt.Errorf("close P13 evidence temporary: %w", err)
	}
	if err := evidenceRoot.Link(temporaryName, finalName); err != nil {
		return "", "", fmt.Errorf("publish P13 evidence without replacement: %w", err)
	}
	if err := evidenceRoot.Remove(temporaryName); err != nil {
		return "", "", fmt.Errorf("remove published P13 evidence temporary: %w", err)
	}
	temporaryPresent = false
	if err := syncEvidenceDirectory(evidenceRoot, ".", "evidence directory"); err != nil {
		return "", "", err
	}
	observed, err := evidenceRoot.ReadFile(finalName)
	if err != nil {
		return "", "", fmt.Errorf("reread P13 evidence carrier: %w", err)
	}
	if !bytes.Equal(observed, canonical) {
		return "", "", fmt.Errorf("reread P13 evidence carrier changed bytes")
	}
	return wantPath, sha256Prefixed(canonical), nil
}

func syncEvidenceDirectory(root *os.Root, path string, label string) error {
	directory, err := root.Open(path)
	if err != nil {
		return fmt.Errorf("open P13 %s for sync: %w", label, err)
	}
	syncErr := directory.Sync()
	closeErr := directory.Close()
	if syncErr != nil {
		return fmt.Errorf("sync P13 %s: %w", label, syncErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close P13 %s: %w", label, closeErr)
	}
	return nil
}

func createEvidenceTemporary(
	root *os.Root,
	finalName string,
) (string, *os.File, error) {
	for range 16 {
		random := make([]byte, 8)
		if _, err := rand.Read(random); err != nil {
			return "", nil, fmt.Errorf("generate P13 evidence temporary name: %w", err)
		}
		name := "." + finalName + "." + hex.EncodeToString(random) + ".tmp"
		file, err := root.OpenFile(
			name,
			os.O_WRONLY|os.O_CREATE|os.O_EXCL,
			0o600,
		)
		if err == nil {
			return name, file, nil
		}
		if os.IsExist(err) {
			continue
		}
		return "", nil, fmt.Errorf("create P13 evidence temporary: %w", err)
	}
	return "", nil, fmt.Errorf("create unique P13 evidence temporary")
}

func acceptanceEnvironment(child bool) []string {
	environment := slices.Clone(os.Environ())
	environment = replaceEnvironmentValue(
		environment,
		"PATH",
		acceptanceSearchPath(),
	)
	if child {
		environment = removeEnvironmentValue(
			environment,
			p13RunConsolidatedEnvironmentKey,
		)
		environment = replaceEnvironmentValue(
			environment,
			p13ChildEnvironmentKey,
			"1",
		)
	}
	return environment
}

func replaceEnvironmentValue(
	environment []string,
	key string,
	value string,
) []string {
	filtered := removeEnvironmentValue(environment, key)
	return append(filtered, key+"="+value)
}

func removeEnvironmentValue(
	environment []string,
	key string,
) []string {
	prefix := key + "="
	filtered := make([]string, 0, len(environment))
	for _, entry := range environment {
		if !strings.HasPrefix(entry, prefix) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func environmentValues(environment []string, key string) []string {
	prefix := key + "="
	values := make([]string, 0, 1)
	for _, entry := range environment {
		if strings.HasPrefix(entry, prefix) {
			values = append(values, strings.TrimPrefix(entry, prefix))
		}
	}
	return values
}

func p13ConsolidatedRunDisposition(
	child string,
	requested string,
) consolidatedRunDisposition {
	if child == "1" {
		return consolidatedRunChildSkip
	}
	dispositions := map[string]consolidatedRunDisposition{
		"":  consolidatedRunOrdinarySkip,
		"1": consolidatedRunExecute,
	}
	disposition, ok := dispositions[requested]
	if !ok {
		return consolidatedRunInvalid
	}
	return disposition
}

func digestStringList(values []string) string {
	encoded, err := json.Marshal(values)
	if err != nil {
		return "sha256:invalid"
	}
	return sha256Prefixed(encoded)
}

func sortedSetKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}
