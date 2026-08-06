package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/m0n0x41d/haft/internal/initplanning"
)

func TestPublicAgentSkillsPlanIsIndependentExactAndIdempotent(
	t *testing.T,
) {
	t.Parallel()

	homeRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve home root: %v", err)
	}
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve project parent: %v", err)
	}
	projectRoot := filepath.Join(parent, "project")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("create project root: %v", err)
	}
	request, err := compilePublicInitRequest(
		weakPublicInitRequest{
			invocation:  initplanning.InvocationExplicit,
			projectRoot: projectRoot,
			projectID:   "qnt_e3149c17",
			agents:      true,
			overseer:    publicOverseerWeakDisabled(),
		},
	)
	if err != nil {
		t.Fatalf("compilePublicInitRequest: %v", err)
	}
	bundle, err := currentSkillSourceBundle()
	if err != nil {
		t.Fatalf("currentSkillSourceBundle: %v", err)
	}
	first, err := compilePublicAgentSkillsPlan(
		request,
		homeRoot,
		bundle,
	)
	if err != nil {
		t.Fatalf("compilePublicAgentSkillsPlan: %v", err)
	}
	if first.Scope() != initplanning.ScopeUser ||
		first.Root() != filepath.Join(homeRoot, ".agents", "skills") ||
		len(first.Effects()) == 0 {
		t.Fatalf("first plan = %#v", first)
	}
	for _, effect := range first.Effects() {
		if effect.Kind() != publicAgentSkillsCreate {
			t.Fatalf("first effect = %#v", effect)
		}
	}
	receipt, err := applyPublicAgentSkillsPlan(
		context.Background(),
		first,
	)
	if err != nil {
		t.Fatalf("applyPublicAgentSkillsPlan: %v", err)
	}
	if receipt.ChangedPaths() != len(first.Effects()) {
		t.Fatalf(
			"changed paths = %d, want %d",
			receipt.ChangedPaths(),
			len(first.Effects()),
		)
	}
	if _, err := os.Stat(
		filepath.Join(
			homeRoot,
			".agents",
			"skills",
			"h-reason",
			"SKILL.md",
		),
	); err != nil {
		t.Fatalf("agent skill missing: %v", err)
	}
	if _, err := os.Stat(receipt.ManifestPath()); err != nil {
		t.Fatalf("agent skill ownership manifest missing: %v", err)
	}
	assertPublicAgentManifestAdapterEdition(
		t,
		receipt.ManifestPath(),
		"codex.skills.v1",
	)

	second, err := compilePublicAgentSkillsPlan(
		request,
		homeRoot,
		bundle,
	)
	if err != nil {
		t.Fatalf("compile second agent plan: %v", err)
	}
	for _, effect := range second.Effects() {
		if effect.Kind() != publicAgentSkillsPreserve {
			t.Fatalf("second effect = %#v", effect)
		}
	}
	secondReceipt, err := applyPublicAgentSkillsPlan(
		context.Background(),
		second,
	)
	if err != nil {
		t.Fatalf("apply second agent plan: %v", err)
	}
	if secondReceipt.ChangedPaths() != 0 {
		t.Fatalf(
			"second apply changed %d paths",
			secondReceipt.ChangedPaths(),
		)
	}
}

func assertPublicAgentManifestAdapterEdition(
	t *testing.T,
	path string,
	want string,
) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read agent skill manifest: %v", err)
	}
	var manifest publicAgentSkillsManifestWire
	if err := json.Unmarshal(content, &manifest); err != nil {
		t.Fatalf("decode agent skill manifest: %v", err)
	}
	if manifest.AdapterEdition != want {
		t.Fatalf(
			"agent skill adapter_edition = %q, want %q\n%s",
			manifest.AdapterEdition,
			want,
			content,
		)
	}
}
