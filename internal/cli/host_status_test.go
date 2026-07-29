package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/initfs"
	"github.com/m0n0x41d/haft/internal/initplanning"
	"github.com/m0n0x41d/haft/internal/testsupport/profileadmissionfixture"
	"github.com/spf13/cobra"
)

func TestHostStatusCommandReadsManifestCurrentnessAndDuplicateRootsWithoutMutation(
	t *testing.T,
) {
	operatorHome := filepath.Clean(os.Getenv("HOME"))
	root := filepath.Join(t.TempDir(), "project")
	harness := profileadmissionfixture.New(t, root)
	projectRoot := harness.Root().String()
	projectID := harness.ProjectID()
	t.Setenv(envProjectRoot, projectRoot)
	t.Setenv(envExpectedProjectID, projectID)

	runtime, err := currentHostPublicationRuntimeFromProcess()
	if err != nil {
		t.Fatalf("currentHostPublicationRuntimeFromProcess: %v", err)
	}
	if runtime.userHomeRoot == operatorHome {
		t.Fatal("host-status fixture did not isolate the operator home")
	}
	bundle, err := currentSkillSourceBundle()
	if err != nil {
		t.Fatalf("currentSkillSourceBundle: %v", err)
	}
	publication, err := currentHostPublicationIdentity(runtime, bundle)
	if err != nil {
		t.Fatalf("currentHostPublicationIdentity: %v", err)
	}
	candidates, err := currentStandardSkillCandidates(
		projectRoot,
		bundle,
		runtime,
	)
	if err != nil {
		t.Fatalf("currentStandardSkillCandidates: %v", err)
	}
	candidate, found := findCurrentStandardSkillCandidate(
		candidates,
		initplanning.HostCodex,
		initplanning.ScopeProject,
	)
	if !found {
		t.Fatal("current Codex project skill candidate is missing")
	}
	projection, err := buildCurrentStandardSkillHostProjection(
		projectRoot,
		projectID,
		candidate,
		publication,
	)
	if err != nil {
		t.Fatalf("buildCurrentStandardSkillHostProjection: %v", err)
	}
	manifestPath := publishHostStatusFixture(
		t,
		projection,
		runtime.userHomeRoot,
	)
	globalReasonPath := filepath.Join(
		runtime.userHomeRoot,
		".agents",
		"skills",
		"h-reason",
		"SKILL.md",
	)
	writeHostStatusFixtureFile(
		t,
		globalReasonPath,
		[]byte("foreign duplicate h-reason\n"),
		0o644,
	)
	if err := harness.Close(); err != nil {
		t.Fatalf("close fixture ledger before public status: %v", err)
	}

	before := snapshotHostStatusFixture(t, []string{
		manifestPath,
		globalReasonPath,
	}, projection.Outputs())
	first := runHostStatusJSONForTest(t)
	after := snapshotHostStatusFixture(t, []string{
		manifestPath,
		globalReasonPath,
	}, projection.Outputs())
	if !slices.Equal(before, after) {
		t.Fatalf("read-only host status changed carrier bytes")
	}
	if first.Schema != hostStatusSchema ||
		first.Project.ProjectID != projectID ||
		first.Project.CorePosture != "current_read_only" ||
		first.MutationsPerformed {
		t.Fatalf("host status core envelope = %#v", first)
	}
	if len(first.Manifests) != 1 {
		t.Fatalf("manifest status count = %d, want 1", len(first.Manifests))
	}
	manifest := first.Manifests[0]
	if manifest.Path != manifestPath ||
		manifest.BindingPosture != "evaluated" ||
		manifest.Currentness == nil ||
		manifest.Currentness.Posture != initplanning.HostInstallationCurrent {
		t.Fatalf("current manifest status = %#v", manifest)
	}
	if first.FilesystemPresenceMeaning !=
		"discovery_evidence_only_without_valid_manifest" {
		t.Fatalf(
			"filesystem presence meaning = %q",
			first.FilesystemPresenceMeaning,
		)
	}
	assertHostStatusDuplicate(
		t,
		first.DuplicateSkillRoots,
		"h-reason",
		candidate.targetRoot,
		filepath.Dir(filepath.Dir(globalReasonPath)),
	)

	tamperedPath := projection.Outputs()[0].Path()
	writeHostStatusFixtureFile(
		t,
		tamperedPath,
		[]byte("locally modified owned carrier\n"),
		0o644,
	)
	tamperedBefore, err := os.ReadFile(tamperedPath)
	if err != nil {
		t.Fatalf("read tampered carrier: %v", err)
	}
	second := runHostStatusJSONForTest(t)
	tamperedAfter, err := os.ReadFile(tamperedPath)
	if err != nil {
		t.Fatalf("reread tampered carrier: %v", err)
	}
	if !bytes.Equal(tamperedBefore, tamperedAfter) {
		t.Fatal("host status overwrote a locally modified owned carrier")
	}
	if len(second.Manifests) != 1 ||
		second.Manifests[0].Currentness == nil ||
		second.Manifests[0].Currentness.Posture !=
			initplanning.HostInstallationBlocked {
		t.Fatalf(
			"tampered manifest currentness = %#v",
			second.Manifests,
		)
	}
	if !slices.Contains(
		second.Manifests[0].Currentness.Reasons,
		"path_locally_modified_owned",
	) {
		t.Fatalf(
			"tampered currentness reasons = %#v",
			second.Manifests[0].Currentness.Reasons,
		)
	}
}

func TestHostStatusCommandTreatsSharedUserSkillsAndProjectComponentsIndependently(
	t *testing.T,
) {
	secondRoot := filepath.Join(t.TempDir(), "second-project")
	harness := profileadmissionfixture.New(t, secondRoot)
	secondRoot = harness.Root().String()
	secondID := harness.ProjectID()
	t.Setenv(envProjectRoot, secondRoot)
	t.Setenv(envExpectedProjectID, secondID)

	runtime, err := currentHostPublicationRuntimeFromProcess()
	if err != nil {
		t.Fatalf("currentHostPublicationRuntimeFromProcess: %v", err)
	}
	bundle, err := currentSkillSourceBundle()
	if err != nil {
		t.Fatalf("currentSkillSourceBundle: %v", err)
	}
	publication, err := currentHostPublicationIdentity(runtime, bundle)
	if err != nil {
		t.Fatalf("currentHostPublicationIdentity: %v", err)
	}

	firstRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve first project root: %v", err)
	}
	firstID := "qnt_a3149c17"
	firstCandidates, err := currentStandardSkillCandidates(
		firstRoot,
		bundle,
		runtime,
	)
	if err != nil {
		t.Fatalf("first currentStandardSkillCandidates: %v", err)
	}
	userSkills, found := findCurrentStandardSkillCandidate(
		firstCandidates,
		initplanning.HostCodex,
		initplanning.ScopeUser,
	)
	if !found {
		t.Fatal("current Codex user skill candidate is missing")
	}
	sharedProjection, err := buildCurrentStandardSkillHostProjection(
		firstRoot,
		firstID,
		userSkills,
		publication,
	)
	if err != nil {
		t.Fatalf("build shared Codex skill projection: %v", err)
	}
	sharedManifestPath := publishHostStatusFixture(
		t,
		sharedProjection,
		runtime.userHomeRoot,
	)

	secondCandidates, err := currentStandardSkillCandidates(
		secondRoot,
		bundle,
		runtime,
	)
	if err != nil {
		t.Fatalf("second currentStandardSkillCandidates: %v", err)
	}
	projectComponents, err := initplanning.ParseComponentSet([]string{
		string(initplanning.ComponentInstructions),
		string(initplanning.ComponentMCP),
	})
	if err != nil {
		t.Fatalf("parse project components: %v", err)
	}
	projectProjection, err := buildSelectedCurrentCoherentHostProjection(
		secondRoot,
		secondID,
		initplanning.HostCodex,
		initplanning.ScopeProject,
		projectComponents,
		secondCandidates,
		bundle,
		publication,
		runtime,
	)
	if err != nil {
		t.Fatalf("build project Codex projection: %v", err)
	}
	projectManifestPath := publishCoherentHostStatusFixture(
		t,
		projectProjection,
		runtime.userHomeRoot,
	)
	if err := harness.Close(); err != nil {
		t.Fatalf("close second project ledger before status: %v", err)
	}

	extraPaths := []string{
		sharedManifestPath,
		projectManifestPath,
	}
	for _, fragment := range projectProjection.ManagedFragments() {
		extraPaths = append(
			extraPaths,
			fragment.Coordinate().CarrierPath(),
		)
	}
	allOutputs := append(
		slices.Clone(sharedProjection.Outputs()),
		projectProjection.Outputs()...,
	)
	before := snapshotHostStatusFixture(t, extraPaths, allOutputs)
	report := runHostStatusJSONForTest(t)
	after := snapshotHostStatusFixture(t, extraPaths, allOutputs)
	if !slices.Equal(before, after) || report.MutationsPerformed {
		t.Fatal("host status changed shared or project-scoped Codex carriers")
	}

	shared := findHostManifestStatus(
		t,
		report.Manifests,
		sharedManifestPath,
	)
	wantSharedComponents := []initplanning.Component{
		initplanning.ComponentSkills,
	}
	if shared.BindingPosture != "evaluated" ||
		shared.Currentness == nil ||
		shared.Currentness.Posture !=
			initplanning.HostInstallationCurrent ||
		shared.Currentness.ProjectRoot != firstRoot ||
		shared.Currentness.ProjectID != firstID ||
		!slices.Equal(
			shared.Currentness.InstalledComponents,
			wantSharedComponents,
		) ||
		!slices.Equal(
			shared.Currentness.DesiredComponents,
			wantSharedComponents,
		) {
		t.Fatalf("shared user skill status = %#v", shared)
	}

	project := findHostManifestStatus(
		t,
		report.Manifests,
		projectManifestPath,
	)
	wantProjectComponents := projectComponents.Values()
	if project.BindingPosture != "evaluated" ||
		project.Currentness == nil ||
		project.Currentness.Posture !=
			initplanning.HostInstallationCurrent ||
		project.Currentness.ProjectRoot != secondRoot ||
		project.Currentness.ProjectID != secondID ||
		!slices.Equal(
			project.Currentness.InstalledComponents,
			wantProjectComponents,
		) ||
		!slices.Equal(
			project.Currentness.DesiredComponents,
			wantProjectComponents,
		) {
		t.Fatalf("project component status = %#v", project)
	}

	sharedBytes, err := os.ReadFile(sharedManifestPath)
	if err != nil {
		t.Fatalf("read shared manifest after status: %v", err)
	}
	sharedManifest, err := initplanning.ParseInstallationManifest(
		sharedBytes,
	)
	if err != nil {
		t.Fatalf("parse shared manifest after status: %v", err)
	}
	if sharedManifest.ProjectRoot() != firstRoot ||
		sharedManifest.ProjectID() != firstID {
		t.Fatalf(
			"shared owner transferred to %s at %s",
			sharedManifest.ProjectID(),
			sharedManifest.ProjectRoot(),
		)
	}
}

func TestHostStatusCommandRejectsManifestDeclaredForAnotherLocation(
	t *testing.T,
) {
	root := filepath.Join(t.TempDir(), "project")
	harness := profileadmissionfixture.New(t, root)
	projectRoot := harness.Root().String()
	projectID := harness.ProjectID()
	t.Setenv(envProjectRoot, projectRoot)
	t.Setenv(envExpectedProjectID, projectID)

	runtime, err := currentHostPublicationRuntimeFromProcess()
	if err != nil {
		t.Fatalf("currentHostPublicationRuntimeFromProcess: %v", err)
	}
	bundle, err := currentSkillSourceBundle()
	if err != nil {
		t.Fatalf("currentSkillSourceBundle: %v", err)
	}
	publication, err := currentHostPublicationIdentity(runtime, bundle)
	if err != nil {
		t.Fatalf("currentHostPublicationIdentity: %v", err)
	}
	candidates, err := currentStandardSkillCandidates(
		projectRoot,
		bundle,
		runtime,
	)
	if err != nil {
		t.Fatalf("currentStandardSkillCandidates: %v", err)
	}
	claudeSkills, found := findCurrentStandardSkillCandidate(
		candidates,
		initplanning.HostClaude,
		initplanning.ScopeUser,
	)
	if !found {
		t.Fatal("current Claude user skill candidate is missing")
	}
	projection, err := buildCurrentStandardSkillHostProjection(
		projectRoot,
		projectID,
		claudeSkills,
		publication,
	)
	if err != nil {
		t.Fatalf("build Claude user skill projection: %v", err)
	}
	claudeManifestPath := publishHostStatusFixture(
		t,
		projection,
		runtime.userHomeRoot,
	)
	layout, err := initplanning.NewPublicationLayout(
		initplanning.PublicationLayoutInput{
			ProjectRoot:  projectRoot,
			ProjectID:    projectID,
			UserHomeRoot: runtime.userHomeRoot,
		},
	)
	if err != nil {
		t.Fatalf("build publication layout: %v", err)
	}
	codexLocation, err := layout.ManifestLocation(
		initplanning.HostCodex,
		initplanning.ScopeUser,
	)
	if err != nil {
		t.Fatalf("resolve Codex user manifest location: %v", err)
	}
	if err := os.MkdirAll(codexLocation.Root(), 0o755); err != nil {
		t.Fatalf("create Codex user manifest root: %v", err)
	}
	if err := os.Rename(
		claudeManifestPath,
		codexLocation.Path(),
	); err != nil {
		t.Fatalf("move Claude manifest to Codex location: %v", err)
	}
	if err := harness.Close(); err != nil {
		t.Fatalf("close fixture ledger before status: %v", err)
	}

	report := runHostStatusJSONForTest(t)
	status := findHostManifestStatus(
		t,
		report.Manifests,
		codexLocation.Path(),
	)
	if status.BindingPosture != "manifest_location_mismatch" ||
		status.Currentness != nil ||
		len(status.Reasons) != 1 ||
		!strings.Contains(
			status.Reasons[0],
			"manifest declares host claude scope user",
		) {
		t.Fatalf("wrong-location manifest status = %#v", status)
	}
}

func TestHostStatusCommandRejectsManifestDeclaredForAnotherScope(
	t *testing.T,
) {
	root := filepath.Join(t.TempDir(), "project")
	harness := profileadmissionfixture.New(t, root)
	projectRoot := harness.Root().String()
	projectID := harness.ProjectID()
	t.Setenv(envProjectRoot, projectRoot)
	t.Setenv(envExpectedProjectID, projectID)

	runtime, err := currentHostPublicationRuntimeFromProcess()
	if err != nil {
		t.Fatalf("currentHostPublicationRuntimeFromProcess: %v", err)
	}
	bundle, err := currentSkillSourceBundle()
	if err != nil {
		t.Fatalf("currentSkillSourceBundle: %v", err)
	}
	publication, err := currentHostPublicationIdentity(runtime, bundle)
	if err != nil {
		t.Fatalf("currentHostPublicationIdentity: %v", err)
	}
	candidates, err := currentStandardSkillCandidates(
		projectRoot,
		bundle,
		runtime,
	)
	if err != nil {
		t.Fatalf("currentStandardSkillCandidates: %v", err)
	}
	codexSkills, found := findCurrentStandardSkillCandidate(
		candidates,
		initplanning.HostCodex,
		initplanning.ScopeProject,
	)
	if !found {
		t.Fatal("current Codex project skill candidate is missing")
	}
	projection, err := buildCurrentStandardSkillHostProjection(
		projectRoot,
		projectID,
		codexSkills,
		publication,
	)
	if err != nil {
		t.Fatalf("build Codex project skill projection: %v", err)
	}
	projectManifestPath := publishHostStatusFixture(
		t,
		projection,
		runtime.userHomeRoot,
	)
	layout, err := initplanning.NewPublicationLayout(
		initplanning.PublicationLayoutInput{
			ProjectRoot:  projectRoot,
			ProjectID:    projectID,
			UserHomeRoot: runtime.userHomeRoot,
		},
	)
	if err != nil {
		t.Fatalf("build publication layout: %v", err)
	}
	codexUserLocation, err := layout.ManifestLocation(
		initplanning.HostCodex,
		initplanning.ScopeUser,
	)
	if err != nil {
		t.Fatalf("resolve Codex user manifest location: %v", err)
	}
	if err := os.MkdirAll(
		filepath.Dir(codexUserLocation.Path()),
		0o755,
	); err != nil {
		t.Fatalf("create Codex user manifest root: %v", err)
	}
	if err := os.Rename(
		projectManifestPath,
		codexUserLocation.Path(),
	); err != nil {
		t.Fatalf("move project manifest to user location: %v", err)
	}
	if err := harness.Close(); err != nil {
		t.Fatalf("close fixture ledger before status: %v", err)
	}

	report := runHostStatusJSONForTest(t)
	status := findHostManifestStatus(
		t,
		report.Manifests,
		codexUserLocation.Path(),
	)
	if status.BindingPosture != "manifest_location_mismatch" ||
		status.Currentness != nil ||
		len(status.Reasons) != 1 ||
		!strings.Contains(
			status.Reasons[0],
			"manifest declares host codex scope project",
		) {
		t.Fatalf("wrong-scope manifest status = %#v", status)
	}
}

func TestHostStatusCommandEvaluatesCoherentManagedFragmentWithoutClaimingCarrier(
	t *testing.T,
) {
	root := filepath.Join(t.TempDir(), "project")
	harness := profileadmissionfixture.New(t, root)
	projectRoot := harness.Root().String()
	projectID := harness.ProjectID()
	t.Setenv(envProjectRoot, projectRoot)
	t.Setenv(envExpectedProjectID, projectID)

	runtime, err := currentHostPublicationRuntimeFromProcess()
	if err != nil {
		t.Fatalf("currentHostPublicationRuntimeFromProcess: %v", err)
	}
	bundle, err := currentSkillSourceBundle()
	if err != nil {
		t.Fatalf("currentSkillSourceBundle: %v", err)
	}
	publication, err := currentHostPublicationIdentity(runtime, bundle)
	if err != nil {
		t.Fatalf("currentHostPublicationIdentity: %v", err)
	}
	candidates, err := currentStandardSkillCandidates(
		projectRoot,
		bundle,
		runtime,
	)
	if err != nil {
		t.Fatalf("currentStandardSkillCandidates: %v", err)
	}
	projection, err := buildCurrentCoherentHostProjection(
		projectRoot,
		projectID,
		initplanning.HostClaude,
		initplanning.ScopeProject,
		candidates,
		bundle,
		publication,
		runtime,
	)
	if err != nil {
		t.Fatalf("buildCurrentCoherentHostProjection: %v", err)
	}
	carrierPath := projection.ManagedFragments()[0].
		Coordinate().
		CarrierPath()
	writeHostStatusFixtureFile(
		t,
		carrierPath,
		[]byte(`{"theme":"dark"}`),
		0o640,
	)
	manifestPath := publishCoherentHostStatusFixture(
		t,
		projection,
		runtime.userHomeRoot,
	)
	setHostStatusCarrierTheme(t, carrierPath, "light")
	if err := harness.Close(); err != nil {
		t.Fatalf("close fixture ledger before coherent status: %v", err)
	}

	before, err := os.ReadFile(carrierPath)
	if err != nil {
		t.Fatalf("read coherent carrier before status: %v", err)
	}
	first := runHostStatusJSONForTest(t)
	after, err := os.ReadFile(carrierPath)
	if err != nil {
		t.Fatalf("read coherent carrier after status: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("coherent host status changed the shared carrier")
	}
	manifest := findHostManifestStatus(t, first.Manifests, manifestPath)
	if manifest.BindingPosture != "evaluated" ||
		manifest.Currentness == nil ||
		manifest.Currentness.Posture != initplanning.HostInstallationCurrent ||
		len(manifest.Currentness.ManagedFragments) != 2 {
		if manifest.Currentness == nil {
			t.Fatalf("coherent current manifest status = %#v", manifest)
		}
		t.Fatalf(
			"coherent current manifest status = %#v; currentness=%+v",
			manifest,
			*manifest.Currentness,
		)
	}
	for _, fragment := range manifest.Currentness.ManagedFragments {
		if fragment.State != initplanning.ManagedFragmentCurrentOwned {
			t.Fatalf(
				"coherent fragment %s/%s state = %s, want current_owned",
				fragment.CarrierPath,
				fragment.Selector,
				fragment.State,
			)
		}
	}

	setHostStatusHaftCommand(t, carrierPath, "/operator/haft")
	second := runHostStatusJSONForTest(t)
	tampered := findHostManifestStatus(t, second.Manifests, manifestPath)
	if tampered.Currentness == nil ||
		tampered.Currentness.Posture != initplanning.HostInstallationBlocked ||
		!slices.Contains(
			tampered.Currentness.Reasons,
			"fragment_locally_modified_owned",
		) {
		t.Fatalf("coherent tampered manifest status = %#v", tampered)
	}
}

func TestHostStatusCommandEvaluatesCoherentHermesYAMLWithoutClaimingCarrier(
	t *testing.T,
) {
	root := filepath.Join(t.TempDir(), "project")
	harness := profileadmissionfixture.New(t, root)
	projectRoot := harness.Root().String()
	projectID := harness.ProjectID()
	t.Setenv(envProjectRoot, projectRoot)
	t.Setenv(envExpectedProjectID, projectID)

	runtime, err := currentHostPublicationRuntimeFromProcess()
	if err != nil {
		t.Fatalf("currentHostPublicationRuntimeFromProcess: %v", err)
	}
	bundle, err := currentSkillSourceBundle()
	if err != nil {
		t.Fatalf("currentSkillSourceBundle: %v", err)
	}
	publication, err := currentHostPublicationIdentity(runtime, bundle)
	if err != nil {
		t.Fatalf("currentHostPublicationIdentity: %v", err)
	}
	candidates, err := currentStandardSkillCandidates(
		projectRoot,
		bundle,
		runtime,
	)
	if err != nil {
		t.Fatalf("currentStandardSkillCandidates: %v", err)
	}
	projection, err := buildCurrentCoherentHostProjection(
		projectRoot,
		projectID,
		initplanning.HostHermes,
		initplanning.ScopeUser,
		candidates,
		bundle,
		publication,
		runtime,
	)
	if err != nil {
		t.Fatalf("buildCurrentCoherentHostProjection: %v", err)
	}
	fragments := projection.ManagedFragments()
	if len(fragments) != 2 {
		t.Fatalf("Hermes managed fragments = %+v, want 2", fragments)
	}
	carrierPath := fragments[0].Coordinate().CarrierPath()
	writeHostStatusFixtureFile(
		t,
		carrierPath,
		[]byte(`# operator-owned header
theme: dark
skills:
  external_dirs:
    - /operator/skills
mcp_servers:
  other:
    command: other-server
telemetry: false
`),
		0o640,
	)
	manifestPath := publishCoherentHostStatusFixture(
		t,
		projection,
		runtime.userHomeRoot,
	)
	replaceHostStatusCarrierBytes(
		t,
		carrierPath,
		[]byte("theme: dark"),
		[]byte("theme: light"),
	)
	if err := harness.Close(); err != nil {
		t.Fatalf("close fixture ledger before coherent Hermes status: %v", err)
	}

	before, err := os.ReadFile(carrierPath)
	if err != nil {
		t.Fatalf("read coherent Hermes carrier before status: %v", err)
	}
	for _, exact := range [][]byte{
		[]byte("# operator-owned header\n"),
		[]byte("    - /operator/skills\n"),
		[]byte("  other:\n    command: other-server\n"),
		[]byte("telemetry: false\n"),
	} {
		if !bytes.Contains(before, exact) {
			t.Fatalf(
				"coherent Hermes publication changed unrelated bytes %q:\n%s",
				exact,
				before,
			)
		}
	}
	first := runHostStatusJSONForTest(t)
	after, err := os.ReadFile(carrierPath)
	if err != nil {
		t.Fatalf("read coherent Hermes carrier after status: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("coherent Hermes host status changed the shared YAML carrier")
	}
	manifest := findHostManifestStatus(t, first.Manifests, manifestPath)
	if manifest.BindingPosture != "evaluated" ||
		manifest.Currentness == nil ||
		manifest.Currentness.Posture != initplanning.HostInstallationCurrent ||
		len(manifest.Currentness.ManagedFragments) != 2 {
		t.Fatalf("coherent Hermes current manifest status = %#v", manifest)
	}
	for _, fragment := range manifest.Currentness.ManagedFragments {
		if fragment.State != initplanning.ManagedFragmentCurrentOwned {
			t.Fatalf(
				"coherent Hermes fragment state = %s, want current_owned",
				fragment.State,
			)
		}
	}

	replaceHostStatusCarrierBytes(
		t,
		carrierPath,
		[]byte("command: "+runtime.executablePath),
		[]byte("command: /operator/haft"),
	)
	second := runHostStatusJSONForTest(t)
	tampered := findHostManifestStatus(t, second.Manifests, manifestPath)
	if tampered.Currentness == nil ||
		tampered.Currentness.Posture != initplanning.HostInstallationBlocked ||
		!slices.Contains(
			tampered.Currentness.Reasons,
			"fragment_locally_modified_owned",
		) {
		t.Fatalf("coherent Hermes tampered status = %#v", tampered)
	}
}

func TestHostStatusCommandEvaluatesEveryRegisteredCoherentProjection(
	t *testing.T,
) {
	cases := []currentCoherentHostKey{
		{host: initplanning.HostAir, scope: initplanning.ScopeProject},
		{host: initplanning.HostAntigravity, scope: initplanning.ScopeUser},
		{host: initplanning.HostClaude, scope: initplanning.ScopeProject},
		{host: initplanning.HostCodex, scope: initplanning.ScopeProject},
		{host: initplanning.HostCursor, scope: initplanning.ScopeProject},
		{host: initplanning.HostGemini, scope: initplanning.ScopeUser},
		{host: initplanning.HostGrok, scope: initplanning.ScopeProject},
		{host: initplanning.HostHermes, scope: initplanning.ScopeUser},
		{host: initplanning.HostOpenCode, scope: initplanning.ScopeProject},
		{host: initplanning.HostPi, scope: initplanning.ScopeProject},
		{host: initplanning.HostZed, scope: initplanning.ScopeUser},
	}
	registry := currentCoherentHostFaceRegistry()
	if len(cases) != len(registry) {
		t.Fatalf(
			"coherent host E2E cases = %d, registry = %d",
			len(cases),
			len(registry),
		)
	}
	for _, test := range cases {
		t.Run(string(test.host)+"_"+string(test.scope), func(t *testing.T) {
			if _, covered := registry[test]; !covered {
				t.Fatalf("coherent host registry omitted %+v", test)
			}
			root := filepath.Join(t.TempDir(), "project")
			harness := profileadmissionfixture.New(t, root)
			projectRoot := harness.Root().String()
			projectID := harness.ProjectID()
			t.Setenv(envProjectRoot, projectRoot)
			t.Setenv(envExpectedProjectID, projectID)

			runtime, err := currentHostPublicationRuntimeFromProcess()
			if err != nil {
				t.Fatalf("currentHostPublicationRuntimeFromProcess: %v", err)
			}
			bundle, err := currentSkillSourceBundle()
			if err != nil {
				t.Fatalf("currentSkillSourceBundle: %v", err)
			}
			publication, err := currentHostPublicationIdentity(runtime, bundle)
			if err != nil {
				t.Fatalf("currentHostPublicationIdentity: %v", err)
			}
			candidates, err := currentStandardSkillCandidates(
				projectRoot,
				bundle,
				runtime,
			)
			if err != nil {
				t.Fatalf("currentStandardSkillCandidates: %v", err)
			}
			projection, err := buildCurrentCoherentHostProjection(
				projectRoot,
				projectID,
				test.host,
				test.scope,
				candidates,
				bundle,
				publication,
				runtime,
			)
			if err != nil {
				t.Fatalf("buildCurrentCoherentHostProjection: %v", err)
			}
			manifestPath := publishCoherentHostStatusFixture(
				t,
				projection,
				runtime.userHomeRoot,
			)
			if err := harness.Close(); err != nil {
				t.Fatalf("close fixture ledger before public status: %v", err)
			}

			extraPaths := []string{manifestPath}
			for _, fragment := range projection.ManagedFragments() {
				extraPaths = append(
					extraPaths,
					fragment.Coordinate().CarrierPath(),
				)
			}
			before := snapshotHostStatusFixture(
				t,
				extraPaths,
				projection.Outputs(),
			)
			report := runHostStatusJSONForTest(t)
			after := snapshotHostStatusFixture(
				t,
				extraPaths,
				projection.Outputs(),
			)
			if !slices.Equal(before, after) {
				t.Fatal("public host status changed a coherent installation")
			}
			if report.MutationsPerformed {
				t.Fatal("public host status claimed a mutation")
			}
			manifest := findHostManifestStatus(
				t,
				report.Manifests,
				manifestPath,
			)
			if manifest.BindingPosture != "evaluated" ||
				manifest.Host != test.host ||
				manifest.Scope != test.scope ||
				manifest.Currentness == nil ||
				manifest.Currentness.Posture !=
					initplanning.HostInstallationCurrent {
				t.Fatalf("coherent manifest status = %#v", manifest)
			}
			currentness := manifest.Currentness
			wantComponents := projection.Components().Values()
			if !slices.Equal(
				currentness.InstalledComponents,
				wantComponents,
			) || !slices.Equal(
				currentness.DesiredComponents,
				wantComponents,
			) {
				t.Fatalf(
					"coherent components installed=%v desired=%v want=%v",
					currentness.InstalledComponents,
					currentness.DesiredComponents,
					wantComponents,
				)
			}
			if len(currentness.VacantTargets) != 0 ||
				len(currentness.VacantManagedFragments) != 0 {
				t.Fatalf("coherent currentness has vacant effects: %#v", currentness)
			}
			for _, path := range currentness.Paths {
				if path.State != initplanning.PathCurrentOwned {
					t.Fatalf(
						"coherent path %s state = %s",
						path.Path,
						path.State,
					)
				}
			}
			for _, fragment := range currentness.ManagedFragments {
				if fragment.State != initplanning.ManagedFragmentCurrentOwned {
					t.Fatalf(
						"coherent fragment %s%s state = %s",
						fragment.Selector,
						fragment.MemberID,
						fragment.State,
					)
				}
			}
		})
	}
}

func TestHostStatusCommandExposesOnlyReadOnlyOutputControl(
	t *testing.T,
) {
	if hostStatusCmd.Flags().Lookup("json") == nil {
		t.Fatal("host status is missing --json")
	}
	for _, forbidden := range []string{
		"agents",
		"apply",
		"component",
		"host",
		"local",
		"migrate",
		"repair",
		"scope",
	} {
		if hostStatusCmd.Flags().Lookup(forbidden) != nil {
			t.Fatalf("host status exposes mutation or policy flag --%s", forbidden)
		}
	}
	if hostStatusCmd.Name() != "status" ||
		hostStatusCmd.Parent() != hostCmd {
		t.Fatal("host status is not registered under the host command")
	}
}

func TestHostStatusJSONUsesStableNestedFieldNames(
	t *testing.T,
) {
	report := hostStatusReport{
		Schema: hostStatusSchema,
		Manifests: []hostManifestStatus{{
			Path:           "/manifest.json",
			ReadPosture:    "valid_manifest",
			BindingPosture: "evaluated",
			Currentness: &hostInstallationStatus{
				InstalledPublication: hostPublicationStatus{
					HaftVersion: "v9",
				},
				VacantTargets: []hostVacantTargetStatus{{
					Path:          "/vacant",
					Component:     initplanning.ComponentSkills,
					DesiredDigest: "sha256:desired",
					DesiredMode:   0o644,
				}},
			},
		}},
	}
	content, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal host status: %v", err)
	}
	rendered := string(content)
	for _, expected := range []string{
		`"installed_publication"`,
		`"haft_version"`,
		`"vacant_targets"`,
		`"desired_digest"`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("host status JSON is missing %s: %s", expected, rendered)
		}
	}
	for _, leaked := range []string{
		`"HaftVersion"`,
		`"DesiredDigest"`,
	} {
		if strings.Contains(rendered, leaked) {
			t.Fatalf("host status JSON leaked Go field %s: %s", leaked, rendered)
		}
	}
}

func TestHostStatusTextSummarizesPerSkillDuplicatesByRootPair(
	t *testing.T,
) {
	first := hostDuplicateSkillRoot{
		SkillName:   "h-reason",
		LeftRoot:    "/project/.agents/skills",
		LeftHost:    initplanning.HostCodex,
		LeftScope:   initplanning.ScopeProject,
		LeftOrigin:  initplanning.SkillRootManifestOwned,
		RightRoot:   "/user/.agents/skills",
		RightHost:   initplanning.HostCodex,
		RightScope:  initplanning.ScopeUser,
		RightOrigin: initplanning.SkillRootDiscovered,
	}
	second := first
	second.SkillName = "h-status"
	second.LeftRoot = first.RightRoot
	second.LeftHost = first.RightHost
	second.LeftScope = first.RightScope
	second.LeftOrigin = first.RightOrigin
	second.RightRoot = first.LeftRoot
	second.RightHost = first.LeftHost
	second.RightScope = first.LeftScope
	second.RightOrigin = first.LeftOrigin
	report := hostStatusReport{
		Schema:              hostStatusSchema,
		DuplicateSkillRoots: []hostDuplicateSkillRoot{first, second},
	}
	output := &bytes.Buffer{}
	if err := writeHostStatusText(output, report); err != nil {
		t.Fatalf("write host status text: %v", err)
	}
	rendered := output.String()
	if !strings.Contains(
		rendered,
		"duplicate exposures: 2 across 1 distinct root pairs",
	) {
		t.Fatalf("duplicates were not summarized by root pair:\n%s", rendered)
	}
	if strings.Contains(rendered, "h-reason:") ||
		strings.Contains(rendered, "h-status:") {
		t.Fatalf("text output leaked noisy per-skill rows:\n%s", rendered)
	}
	if !strings.Contains(
		rendered,
		"Complete per-skill evidence: haft host status --json",
	) {
		t.Fatalf("text output omitted the complete-evidence route:\n%s", rendered)
	}
}

func publishHostStatusFixture(
	t *testing.T,
	projection initplanning.HostAdapterProjection,
	userHomeRoot string,
) string {
	t.Helper()
	builder := initplanning.NewHostAdapterInstallPlanBuilder(
		projection.Host(),
	)
	builder = builder.AtEdition(projection.Edition())
	builder = builder.PublishedFrom(projection.Publication())
	builder = builder.ForProject(
		projection.ProjectRoot(),
		projection.ProjectID().String(),
	)
	builder = builder.WithSelection(
		projection.Scope(),
		projection.Components(),
	)
	for _, root := range projection.TargetRoots() {
		builder = builder.AddTargetRoot(root)
	}
	for _, output := range projection.Outputs() {
		expectation, err := initplanning.ExpectMissing(output.Path())
		if err != nil {
			t.Fatalf("build missing expectation: %v", err)
		}
		builder = builder.AddOutput(expectation, output)
	}
	builder = builder.RecoverWith(projection.Recovery())
	plan, err := builder.Build()
	if err != nil {
		t.Fatalf("build host fixture plan: %v", err)
	}
	manifest, err := initplanning.BuildInstallationManifest(plan)
	if err != nil {
		t.Fatalf("build host fixture manifest: %v", err)
	}
	for _, output := range projection.Outputs() {
		writeHostStatusFixtureFile(
			t,
			output.Path(),
			output.Content(),
			output.Mode(),
		)
	}
	layout, err := initplanning.NewPublicationLayout(
		initplanning.PublicationLayoutInput{
			ProjectRoot:  projection.ProjectRoot(),
			ProjectID:    projection.ProjectID().String(),
			UserHomeRoot: userHomeRoot,
		},
	)
	if err != nil {
		t.Fatalf("build publication layout: %v", err)
	}
	location, err := layout.ManifestLocation(
		projection.Host(),
		projection.Scope(),
	)
	if err != nil {
		t.Fatalf("resolve manifest location: %v", err)
	}
	store, err := initfs.NewManifestStore(
		location.Root(),
		location.Path(),
		hostStatusMaxCarrierBytes,
	)
	if err != nil {
		t.Fatalf("build manifest store: %v", err)
	}
	outcome, err := store.Persist(
		manifest,
		initfs.ExpectManifestMissing(),
	)
	if err != nil {
		t.Fatalf("persist host fixture manifest: %v", err)
	}
	if outcome.Kind() != initfs.ManifestPersisted {
		t.Fatalf("manifest persist outcome = %s", outcome.Kind())
	}
	return location.Path()
}

func publishCoherentHostStatusFixture(
	t *testing.T,
	projection initplanning.HostAdapterProjection,
	userHomeRoot string,
) string {
	t.Helper()
	observer, err := initfs.NewFileObserver(hostStatusMaxCarrierBytes)
	if err != nil {
		t.Fatalf("build coherent fixture observer: %v", err)
	}
	wholePlan, err := initplanning.BuildFirstInstallationObservationPlan(
		projection,
		initplanning.WithoutKnownLegacyRegistry(),
	)
	if err != nil {
		t.Fatalf("build coherent fixture whole plan: %v", err)
	}
	wholeObservations := []initplanning.PathObservation{}
	if len(wholePlan.Targets()) != 0 {
		wholeObservations, err = observer.Observe(wholePlan)
		if err != nil {
			t.Fatalf("observe coherent fixture whole paths: %v", err)
		}
	}
	carrierPlans, err :=
		initplanning.BuildFirstCoherentManagedCarrierObservationPlans(
			projection,
			initplanning.NoManagedFragmentLegacyRegistry(),
		)
	if err != nil {
		t.Fatalf("build coherent fixture carrier plans: %v", err)
	}
	carrierInputs := make(
		[]initplanning.ManagedCarrierInput,
		0,
		len(carrierPlans),
	)
	for _, plan := range carrierPlans {
		input, err := observer.ObserveManagedCarrier(
			plan,
			projection.TargetRoots(),
		)
		if err != nil {
			t.Fatalf("observe coherent fixture carrier: %v", err)
		}
		carrierInputs = append(carrierInputs, input)
	}
	currentness, err :=
		initplanning.ClassifyFirstCoherentInstallationCurrentness(
			projection,
			wholeObservations,
			carrierInputs,
			initplanning.WithoutKnownLegacyRegistry(),
			initplanning.NoManagedFragmentLegacyRegistry(),
		)
	if err != nil {
		t.Fatalf("classify coherent fixture: %v", err)
	}
	plan, err := initplanning.CompileCoherentHostAdapterReconciliation(
		currentness,
	)
	if err != nil {
		t.Fatalf("compile coherent fixture: %v", err)
	}
	batch, err := initplanning.BuildHostPublicationBatch(plan)
	if err != nil {
		t.Fatalf("build coherent fixture publication batch: %v", err)
	}
	layout, err := initplanning.NewPublicationLayout(
		initplanning.PublicationLayoutInput{
			ProjectRoot:  projection.ProjectRoot(),
			ProjectID:    projection.ProjectID().String(),
			UserHomeRoot: userHomeRoot,
		},
	)
	if err != nil {
		t.Fatalf("build coherent fixture publication layout: %v", err)
	}
	location, err := layout.ManifestLocation(
		projection.Host(),
		projection.Scope(),
	)
	if err != nil {
		t.Fatalf("resolve coherent fixture manifest location: %v", err)
	}
	store, err := initfs.NewManifestStore(
		location.Root(),
		location.Path(),
		hostStatusMaxCarrierBytes,
	)
	if err != nil {
		t.Fatalf("build coherent fixture manifest store: %v", err)
	}
	publisher, err := initfs.NewHostPublisher(hostStatusMaxCarrierBytes)
	if err != nil {
		t.Fatalf("build coherent fixture publisher: %v", err)
	}
	outcome, err := publisher.Publish(batch, store)
	if err != nil {
		t.Fatalf("publish coherent fixture: %v", err)
	}
	if outcome.Kind() != initfs.HostPublicationApplied {
		t.Fatalf("coherent fixture publication outcome = %s", outcome.Kind())
	}
	return location.Path()
}

func findHostManifestStatus(
	t *testing.T,
	statuses []hostManifestStatus,
	path string,
) hostManifestStatus {
	t.Helper()
	for _, status := range statuses {
		if status.Path == path {
			return status
		}
	}
	t.Fatalf("host status omitted manifest %s", path)
	return hostManifestStatus{}
}

func setHostStatusCarrierTheme(
	t *testing.T,
	path string,
	theme string,
) {
	t.Helper()
	carrier := readHostStatusJSONCarrier(t, path)
	carrier["theme"] = theme
	writeHostStatusJSONCarrier(t, path, carrier)
}

func setHostStatusHaftCommand(
	t *testing.T,
	path string,
	command string,
) {
	t.Helper()
	carrier := readHostStatusJSONCarrier(t, path)
	servers, ok := carrier["mcp_servers"].(map[string]any)
	if !ok {
		servers, ok = carrier["mcpServers"].(map[string]any)
	}
	if !ok {
		t.Fatalf("shared carrier lacks MCP server map: %#v", carrier)
	}
	haft, ok := servers["haft"].(map[string]any)
	if !ok {
		t.Fatalf("shared carrier lacks Haft MCP entry: %#v", servers)
	}
	haft["command"] = command
	writeHostStatusJSONCarrier(t, path, carrier)
}

func readHostStatusJSONCarrier(
	t *testing.T,
	path string,
) map[string]any {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read shared JSON carrier: %v", err)
	}
	carrier := map[string]any{}
	if err := json.Unmarshal(content, &carrier); err != nil {
		t.Fatalf("decode shared JSON carrier: %v", err)
	}
	return carrier
}

func writeHostStatusJSONCarrier(
	t *testing.T,
	path string,
	carrier map[string]any,
) {
	t.Helper()
	content, err := json.Marshal(carrier)
	if err != nil {
		t.Fatalf("encode shared JSON carrier: %v", err)
	}
	if err := os.WriteFile(path, content, 0o640); err != nil {
		t.Fatalf("write shared JSON carrier: %v", err)
	}
}

func writeHostStatusFixtureFile(
	t *testing.T,
	path string,
	content []byte,
	mode os.FileMode,
) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create fixture directory: %v", err)
	}
	if err := os.WriteFile(path, content, mode); err != nil {
		t.Fatalf("write fixture carrier: %v", err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatalf("set fixture carrier mode: %v", err)
	}
}

func replaceHostStatusCarrierBytes(
	t *testing.T,
	path string,
	before []byte,
	after []byte,
) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture carrier for exact replacement: %v", err)
	}
	if bytes.Count(content, before) != 1 {
		t.Fatalf(
			"fixture carrier contains %q %d times, want once:\n%s",
			before,
			bytes.Count(content, before),
			content,
		)
	}
	content = bytes.Replace(content, before, after, 1)
	if err := os.WriteFile(path, content, 0o640); err != nil {
		t.Fatalf("write exact fixture carrier replacement: %v", err)
	}
}

func runHostStatusJSONForTest(t *testing.T) hostStatusReport {
	t.Helper()
	previousJSON := hostStatusJSON
	hostStatusJSON = true
	t.Cleanup(func() {
		hostStatusJSON = previousJSON
	})
	output := &bytes.Buffer{}
	command := &cobra.Command{}
	command.SetContext(context.Background())
	command.SetOut(output)
	if err := runHostStatus(command, nil); err != nil {
		t.Fatalf("runHostStatus: %v", err)
	}
	var report hostStatusReport
	decoder := json.NewDecoder(bytes.NewReader(output.Bytes()))
	if err := decoder.Decode(&report); err != nil {
		t.Fatalf("decode host status JSON: %v\n%s", err, output.String())
	}
	return report
}

func snapshotHostStatusFixture(
	t *testing.T,
	extra []string,
	outputs []initplanning.RenderedOutput,
) []string {
	t.Helper()
	paths := slices.Clone(extra)
	for _, output := range outputs {
		paths = append(paths, output.Path())
	}
	paths = sortedUniqueHostStatusStrings(paths)
	snapshot := make([]string, len(paths))
	for index, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read snapshot path %s: %v", path, err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat snapshot path %s: %v", path, err)
		}
		snapshot[index] = path + "\x00" +
			string(content) + "\x00" +
			info.Mode().Perm().String()
	}
	return snapshot
}

func sortedUniqueHostStatusStrings(values []string) []string {
	result := slices.Clone(values)
	slices.Sort(result)
	return slices.Compact(result)
}

func assertHostStatusDuplicate(
	t *testing.T,
	duplicates []hostDuplicateSkillRoot,
	skillName string,
	leftRoot string,
	rightRoot string,
) {
	t.Helper()
	for _, duplicate := range duplicates {
		if duplicate.SkillName != skillName {
			continue
		}
		roots := []string{duplicate.LeftRoot, duplicate.RightRoot}
		slices.Sort(roots)
		want := []string{leftRoot, rightRoot}
		slices.Sort(want)
		if slices.Equal(roots, want) {
			if duplicate.LeftEvidenceDigest == "" ||
				duplicate.RightEvidenceDigest == "" {
				t.Fatal("duplicate root evidence digest is missing")
			}
			return
		}
	}
	var rendered []string
	for _, duplicate := range duplicates {
		rendered = append(
			rendered,
			duplicate.SkillName+":"+duplicate.LeftRoot+"<->"+
				duplicate.RightRoot,
		)
	}
	t.Fatalf(
		"duplicate %s at %s and %s is missing: %s",
		skillName,
		leftRoot,
		rightRoot,
		strings.Join(rendered, ", "),
	)
}
