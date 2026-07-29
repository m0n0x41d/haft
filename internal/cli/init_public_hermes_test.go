package cli

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/m0n0x41d/haft/internal/initplanning"
	"gopkg.in/yaml.v3"
)

func TestTypedPublicHermesOperationSupportsExactExternalHome(
	t *testing.T,
) {
	homeRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve home root: %v", err)
	}
	projectRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}
	externalParent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve Hermes parent: %v", err)
	}
	hermesHome := filepath.Join(externalParent, "custom-hermes")
	if err := os.MkdirAll(hermesHome, 0o755); err != nil {
		t.Fatalf("create Hermes home: %v", err)
	}
	configPath := filepath.Join(hermesHome, "config.yaml")
	if err := os.WriteFile(
		configPath,
		[]byte(
			"unrelated:\n"+
				"  keep: true\n"+
				"mcp_servers:\n"+
				"  quint-code:\n"+
				"    command: operator-wrapper\n"+
				"    args: [custom]\n",
		),
		0o644,
	); err != nil {
		t.Fatalf("write existing Hermes config: %v", err)
	}
	t.Setenv("HOME", homeRoot)
	request, err := compilePublicInitRequest(
		weakPublicInitRequest{
			invocation:      initplanning.InvocationExplicit,
			projectRoot:     projectRoot,
			projectID:       "qnt_e3149c17",
			hosts:           initHostOptions{hermes: true},
			overseer:        publicOverseerWeakDisabled(),
			hermesHomeInput: hermesHome,
		},
	)
	if err != nil {
		t.Fatalf("compilePublicInitRequest: %v", err)
	}
	if len(request.hostBindings) != 0 ||
		request.hermes.kind != publicHermesConfigure {
		t.Fatalf("Hermes was not isolated from host bindings: %#v", request)
	}
	runtime, err := currentHostPublicationRuntimeFromProcess()
	if err != nil {
		t.Fatalf("currentHostPublicationRuntimeFromProcess: %v", err)
	}
	runtime.userHomeRoot = homeRoot
	prepared, err := prepareTypedPublicInitOperation(
		context.Background(),
		request,
		runtime,
		io.Discard,
		1<<20,
	)
	if err != nil {
		t.Fatalf("prepareTypedPublicInitOperation: %v", err)
	}
	preview, err := prepared.Preview()
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if len(preview.Base.Hosts) != 0 ||
		len(preview.Hermes) != 1 ||
		preview.Hermes[0].ConfigPath != configPath {
		t.Fatalf("Hermes preview = %#v", preview)
	}
	confirmed, err := prepared.ConfirmPreview(preview)
	if err != nil {
		t.Fatalf("ConfirmPreview: %v", err)
	}
	executor, err := newTypedPublicInitExecutor(
		request,
		io.Discard,
		1<<20,
	)
	if err != nil {
		t.Fatalf("newTypedPublicInitExecutor: %v", err)
	}
	outcome, err := confirmed.Apply(
		context.Background(),
		executor,
	)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if outcome.Kind() != publicInitApplied {
		t.Fatalf("outcome kind = %s", outcome.Kind())
	}
	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read Hermes config: %v", err)
	}
	var config map[string]any
	if err := yaml.Unmarshal(configBytes, &config); err != nil {
		t.Fatalf("parse Hermes config: %v", err)
	}
	if _, ok := config["unrelated"]; !ok {
		t.Fatalf("Hermes config lost unrelated content: %s", configBytes)
	}
	mcpServers, ok := config["mcp_servers"].(map[string]any)
	if !ok ||
		mcpServers["haft"] == nil ||
		mcpServers["quint-code"] == nil {
		t.Fatalf(
			"Hermes config claimed a foreign MCP alias: %s",
			configBytes,
		)
	}
	skillPath := filepath.Join(
		homeRoot,
		".haft",
		"hermes",
		"skills",
		"haft",
		"h-reason",
		"SKILL.md",
	)
	if _, err := os.Stat(skillPath); err != nil {
		t.Fatalf("Hermes skill missing: %v", err)
	}
	manifestPath := filepath.Join(
		projectRoot,
		".haft",
		"ancillary-installations",
		"hermes.json",
	)
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("Hermes manifest missing: %v", err)
	}

	secondPrepared, err := prepareTypedPublicInitOperation(
		context.Background(),
		request,
		runtime,
		io.Discard,
		1<<20,
	)
	if err != nil {
		t.Fatalf("prepare second operation: %v", err)
	}
	secondPreview, err := secondPrepared.Preview()
	if err != nil {
		t.Fatalf("second Preview: %v", err)
	}
	for _, effect := range secondPreview.Hermes[0].Effects {
		if effect.Kind != publicExactFilePreserve {
			t.Fatalf("second effect is not current: %#v", effect)
		}
	}
	secondConfirmed, err := secondPrepared.ConfirmPreview(
		secondPreview,
	)
	if err != nil {
		t.Fatalf("confirm second operation: %v", err)
	}
	secondOutcome, err := secondConfirmed.Apply(
		context.Background(),
		executor,
	)
	if err != nil {
		t.Fatalf("apply second operation: %v", err)
	}
	if secondOutcome.Kind() != publicInitAlreadyCurrent {
		t.Fatalf("second outcome kind = %s", secondOutcome.Kind())
	}
}

func TestResolvePublicHermesHomeAppliesProfileOnlyToImplicitHome(
	t *testing.T,
) {
	userHome := t.TempDir()
	envHome := filepath.Join(t.TempDir(), "env-hermes")
	explicitHome := filepath.Join(t.TempDir(), "explicit-hermes")
	t.Setenv("HERMES_HOME", "")

	implicit, err := resolvePublicHermesHome(
		publicHermesOptions{profileInput: "engineering"},
		userHome,
	)
	if err != nil {
		t.Fatalf("resolve implicit profile home: %v", err)
	}
	wantImplicit := filepath.Join(
		userHome,
		".hermes",
		"profiles",
		"engineering",
	)
	if implicit != wantImplicit {
		t.Fatalf(
			"implicit profile home = %q, want %q",
			implicit,
			wantImplicit,
		)
	}

	t.Setenv("HERMES_HOME", envHome)
	fromEnvironment, err := resolvePublicHermesHome(
		publicHermesOptions{profileInput: "engineering"},
		userHome,
	)
	if err != nil {
		t.Fatalf("resolve environment profile home: %v", err)
	}
	wantEnvironment := filepath.Join(
		envHome,
		"profiles",
		"engineering",
	)
	if fromEnvironment != wantEnvironment {
		t.Fatalf(
			"environment profile home = %q, want %q",
			fromEnvironment,
			wantEnvironment,
		)
	}

	explicit, err := resolvePublicHermesHome(
		publicHermesOptions{
			homeInput:    explicitHome,
			profileInput: "engineering",
		},
		userHome,
	)
	if err != nil {
		t.Fatalf("resolve explicit Hermes home: %v", err)
	}
	if explicit != explicitHome {
		t.Fatalf(
			"explicit Hermes home = %q, want %q",
			explicit,
			explicitHome,
		)
	}

	if _, err := resolvePublicHermesHome(
		publicHermesOptions{profileInput: "../engineering"},
		userHome,
	); err == nil {
		t.Fatal("invalid Hermes profile was accepted")
	}
}
