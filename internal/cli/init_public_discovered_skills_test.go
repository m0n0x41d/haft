package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/initfs"
	"github.com/m0n0x41d/haft/internal/initplanning"
)

func TestPublicCodexInitRepairsExistingGeneratedProjectSkills(t *testing.T) {
	request, runtime, core := discoveredCodexFixture(t)
	legacy := readDiscoveredCodexFixture(t)
	projectSkill := filepath.Join(request.projectRoot, ".agents", "skills", "h-reason", "SKILL.md")
	writeDiscoveredCodexFixture(t, projectSkill, legacy)
	policy := filepath.Join(request.projectRoot, ".agents", "skills", "h-reason", "agents", "openai.yaml")
	writeDiscoveredCodexFixture(t, policy, []byte("policy:\n  allow_implicit_invocation: true\n"))
	unrelated := filepath.Join(request.projectRoot, ".claude", "skills", "h-reason", "SKILL.md")
	writeDiscoveredCodexFixture(t, unrelated, legacy)
	foreign := filepath.Join(request.projectRoot, ".agents", "skills", "h-private", "SKILL.md")
	writeDiscoveredCodexFixture(t, foreign, []byte("private skill\n"))

	config, err := currentCodexTOMLFragmentContent(currentCoherentHostContext{projectID: request.projectID})
	if err != nil {
		t.Fatal(err)
	}
	config = bytes.ReplaceAll(config, []byte("startup_timeout_sec = 20"), []byte("startup_timeout_sec = 10"))
	custom := []byte("\n[mcp_servers.haft.tools.haft_method]\napproval_mode = \"prompt\"\n")
	config = append(config, custom...)
	configPath := filepath.Join(request.projectRoot, ".codex", "config.toml")
	writeDiscoveredCodexFixture(t, configPath, config)

	plan, err := compilePublicHostInitPlan(request, core, runtime, publicInitMaxCarrierBytes)
	if err != nil {
		t.Fatal(err)
	}
	publishDiscoveredCodexFixture(t, plan, runtime, initfs.HostPublicationApplied)
	local, err := os.ReadFile(projectSkill)
	if err != nil {
		t.Fatal(err)
	}
	globalPath := filepath.Join(runtime.userHomeRoot, ".agents", "skills", "h-reason", "SKILL.md")
	global, err := os.ReadFile(globalPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(local, global) || bytes.Equal(local, legacy) {
		t.Fatal("project and global skills must equal the new rendering")
	}
	missing := filepath.Join(request.projectRoot, ".agents", "skills", "h-frame", "SKILL.md")
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("absent unowned project skill was installed: %v", err)
	}
	preserved, err := os.ReadFile(unrelated)
	if err != nil || !bytes.Equal(preserved, legacy) {
		t.Fatalf("unselected host changed: %v", err)
	}
	preserved, err = os.ReadFile(foreign)
	if err != nil || string(preserved) != "private skill\n" {
		t.Fatalf("unrelated skill changed: %v", err)
	}
	updatedConfig, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(updatedConfig, custom) ||
		!bytes.Contains(updatedConfig, []byte("startup_timeout_sec = 20")) ||
		bytes.Contains(updatedConfig, []byte("required =")) {
		t.Fatalf("startup allowance or custom approvals changed incorrectly: %s", updatedConfig)
	}
	second, err := compilePublicHostInitPlan(request, core, runtime, publicInitMaxCarrierBytes)
	if err != nil {
		t.Fatal(err)
	}
	publishDiscoveredCodexFixture(t, second, runtime, initfs.HostPublicationAlreadyCurrent)
	assertDiscoveredCodexStatus(t, request, runtime, initplanning.HostInstallationCurrent)
	modified := append(local, []byte("\nHuman edit retained.\n")...)
	writeDiscoveredCodexFixture(t, projectSkill, modified)
	_, err = compilePublicHostInitPlan(request, core, runtime, publicInitMaxCarrierBytes)
	if err == nil || !strings.Contains(err.Error(), projectSkill) || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("modified owned duplicate diagnostic = %v", err)
	}
}

func TestPublicCodexInitPreservesAmbiguousUnownedProjectSkill(t *testing.T) {
	request, runtime, core := discoveredCodexFixture(t)
	legacy := readDiscoveredCodexFixture(t)
	modified := append(legacy, []byte("\nHuman instructions: keep these edits.\n")...)
	path := filepath.Join(request.projectRoot, ".agents", "skills", "h-reason", "SKILL.md")
	writeDiscoveredCodexFixture(t, path, modified)
	_, err := compilePublicHostInitPlan(request, core, runtime, publicInitMaxCarrierBytes)
	if err == nil || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "exact known generated carrier") {
		t.Fatalf("ambiguous duplicate diagnostic = %v", err)
	}
	actual, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(actual, modified) {
		t.Fatalf("human edits were changed: %v", err)
	}
	globalRoot := filepath.Join(runtime.userHomeRoot, ".agents")
	if _, err := os.Stat(globalRoot); !os.IsNotExist(err) {
		t.Fatalf("blocked planning changed global fixture: %v", err)
	}
}

func TestPublicCodexProjectSkillPublicationRejectsConcurrentEdit(t *testing.T) {
	request, runtime, core := discoveredCodexFixture(t)
	legacy := readDiscoveredCodexFixture(t)
	path := filepath.Join(request.projectRoot, ".agents", "skills", "h-reason", "SKILL.md")
	writeDiscoveredCodexFixture(t, path, legacy)
	plan, err := compilePublicHostInitPlan(request, core, runtime, publicInitMaxCarrierBytes)
	if err != nil {
		t.Fatal(err)
	}
	modified := append(legacy, []byte("\nConcurrent human edit.\n")...)
	writeDiscoveredCodexFixture(t, path, modified)
	for _, host := range plan.Hosts() {
		if host.Scope() != initplanning.ScopeProject {
			continue
		}
		outcome := publishDiscoveredCodexHost(t, plan, host, runtime)
		if outcome.Kind() != initfs.HostPublicationPreconditionChanged {
			t.Fatalf("concurrent edit publication = %s", outcome.Kind())
		}
	}
	actual, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(actual, modified) {
		t.Fatalf("concurrent edits were changed: %v", err)
	}
}

func discoveredCodexFixture(t *testing.T) (publicInitRequest, currentHostPublicationRuntime, initplanning.CoreProjectPlan) {
	t.Helper()
	projectRoot := t.TempDir()
	request, err := compilePublicInitRequest(weakPublicInitRequest{
		invocation:  initplanning.InvocationExplicit,
		projectRoot: projectRoot,
		projectID:   "qnt_e3149c17",
		hosts:       initHostOptions{codex: true},
		overseer:    publicOverseerWeakDisabled(),
	})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := currentHostPublicationRuntimeFromProcess()
	if err != nil {
		t.Fatal(err)
	}
	runtime.userHomeRoot = t.TempDir()
	// The core is a planning fixture; no ledger is opened or core effect run.
	core := currentPreparedOperationCorePlan(t, projectRoot, request.projectID)
	return request, runtime, core
}

func readDiscoveredCodexFixture(t *testing.T) []byte {
	t.Helper()
	content, err := os.ReadFile("testdata/init/codex-h-reason-de0f1407.md")
	if err != nil {
		t.Fatal(err)
	}
	digest := publicContentDigest(content)
	if digest != publicLegacyCodexReasonDigest {
		t.Fatalf("historical fixture digest = %s", digest)
	}
	return content
}

func writeDiscoveredCodexFixture(t *testing.T, path string, content []byte) {
	t.Helper()
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func publishDiscoveredCodexFixture(t *testing.T, plan initplanning.InitPlan, runtime currentHostPublicationRuntime, want initfs.HostPublicationOutcomeKind) {
	t.Helper()
	if plan.Readiness() != initplanning.PlanReady {
		t.Fatalf("host plan blocked: %#v", plan.Preview())
	}
	for _, host := range plan.Hosts() {
		outcome := publishDiscoveredCodexHost(t, plan, host, runtime)
		if outcome.Kind() != want {
			t.Fatalf("%s publication = %s, want %s", host.BindingID().String(), outcome.Kind(), want)
		}
	}
}

func publishDiscoveredCodexHost(t *testing.T, plan initplanning.InitPlan, host initplanning.HostAdapterInstallPlan, runtime currentHostPublicationRuntime) initfs.HostPublicationOutcome {
	t.Helper()
	layout, err := initplanning.NewPublicationLayout(initplanning.PublicationLayoutInput{
		ProjectRoot:  plan.Core().ProjectRoot(),
		ProjectID:    plan.Core().ProjectID().String(),
		UserHomeRoot: runtime.userHomeRoot,
	})
	if err != nil {
		t.Fatal(err)
	}
	location, err := layout.ManifestLocation(host.Host(), host.Scope())
	if err != nil {
		t.Fatal(err)
	}
	store, err := initfs.NewManifestStore(location.Root(), location.Path(), publicInitMaxCarrierBytes)
	if err != nil {
		t.Fatal(err)
	}
	batch, err := initplanning.BuildHostPublicationBatch(host)
	if err != nil {
		t.Fatal(err)
	}
	publisher, err := initfs.NewHostPublisher(publicInitMaxCarrierBytes)
	if err != nil {
		t.Fatal(err)
	}
	outcome, err := publisher.Publish(batch, store)
	if err != nil {
		t.Fatal(err)
	}
	return outcome
}

func TestPublicCodexPartialProjectStatusRetainsForeignCollisions(t *testing.T) {
	request, runtime, core := discoveredCodexFixture(t)
	legacy := readDiscoveredCodexFixture(t)
	path := filepath.Join(request.projectRoot, ".agents", "skills", "h-reason", "SKILL.md")
	writeDiscoveredCodexFixture(t, path, legacy)
	plan, err := compilePublicHostInitPlan(request, core, runtime, publicInitMaxCarrierBytes)
	if err != nil {
		t.Fatal(err)
	}
	publishDiscoveredCodexFixture(t, plan, runtime, initfs.HostPublicationApplied)
	foreign := filepath.Join(request.projectRoot, ".agents", "skills", "h-frame", "SKILL.md")
	writeDiscoveredCodexFixture(t, foreign, []byte("Operator-owned h-frame\n"))
	assertDiscoveredCodexStatus(t, request, runtime, initplanning.HostInstallationBlocked)
}

func assertDiscoveredCodexStatus(t *testing.T, request publicInitRequest, runtime currentHostPublicationRuntime, want initplanning.HostInstallationPosture) {
	t.Helper()
	bundle, err := currentSkillSourceBundle()
	if err != nil {
		t.Fatal(err)
	}
	publication, err := currentHostPublicationIdentity(runtime, bundle)
	if err != nil {
		t.Fatal(err)
	}
	candidates, err := currentStandardSkillCandidates(request.projectRoot, bundle, runtime)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(request.projectRoot, ".haft", "host-installations", "codex.project.json")
	store, err := initfs.NewManifestStore(request.projectRoot, path, publicInitMaxCarrierBytes)
	if err != nil {
		t.Fatal(err)
	}
	read, err := store.Read()
	if err != nil {
		t.Fatal(err)
	}
	inspector, err := initfs.NewHostStatusInspector(publicInitMaxCarrierBytes)
	if err != nil {
		t.Fatal(err)
	}
	status := inspectOneHostManifest(store, read.Manifest(), request.projectRoot, request.projectID, candidates, bundle, publication, runtime, inspector)
	if status.Currentness == nil || status.Currentness.Posture != want {
		t.Fatalf("partial duplicate ownership status = %#v, want %s", status, want)
	}
}
