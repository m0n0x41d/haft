package cli

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/initplanning"
	"github.com/m0n0x41d/haft/internal/overseer"
)

func TestTypedPublicOverseerPlanIsExactPreservesHookAndBecomesCurrent(
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
	hookPath := filepath.Join(
		projectRoot,
		".git",
		"hooks",
		"post-commit",
	)
	if err := os.MkdirAll(filepath.Dir(hookPath), 0o755); err != nil {
		t.Fatalf("create hook parent: %v", err)
	}
	existingHook := "#!/bin/sh\n\nprintf 'project hook\\n'\n"
	if err := os.WriteFile(
		hookPath,
		[]byte(existingHook),
		0o755,
	); err != nil {
		t.Fatalf("write existing hook: %v", err)
	}
	t.Setenv("HOME", homeRoot)
	request, err := compilePublicInitRequest(
		weakPublicInitRequest{
			invocation:  initplanning.InvocationExplicit,
			projectRoot: projectRoot,
			projectID:   "qnt_e3149c17",
			hosts:       initHostOptions{codex: true},
			overseer: publicOverseerWeakConfiguration{
				reviewer: "auto",
				timeout:  90,
			},
		},
	)
	if err != nil {
		t.Fatalf("compilePublicInitRequest: %v", err)
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
	if _, err := os.Stat(
		filepath.Join(projectRoot, ".haft"),
	); !os.IsNotExist(err) {
		t.Fatalf("planning wrote project core: %v", err)
	}
	preview, err := prepared.Preview()
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if len(preview.Overseer) != 1 ||
		len(preview.Overseer[0].Effects) != 3 {
		t.Fatalf("overseer preview = %#v", preview.Overseer)
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
	config, err := overseer.LoadConfig(projectRoot)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if config.ReviewerAgent != overseer.ReviewerCodex ||
		config.ReviewTimeoutSeconds != 90 {
		t.Fatalf("overseer config = %#v", config)
	}
	hook, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("read installed hook: %v", err)
	}
	if !strings.Contains(string(hook), "printf 'project hook") ||
		!strings.Contains(string(hook), "# BEGIN HAFT OVERSEER") {
		t.Fatalf("installed hook lost content:\n%s", string(hook))
	}
	manifestPath := filepath.Join(
		projectRoot,
		".haft",
		"ancillary-installations",
		"overseer.json",
	)
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("overseer manifest missing: %v", err)
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
	for _, effect := range secondPreview.Overseer[0].Effects {
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

func TestPublicExactFileEffectsRejectStalePreviewBeforeAnyWrite(
	t *testing.T,
) {
	t.Parallel()

	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve root: %v", err)
	}
	firstPath := filepath.Join(root, "first")
	secondPath := filepath.Join(root, "second")
	first, err := planPublicExactFile(
		firstPath,
		[]byte("first\n"),
		0o644,
	)
	if err != nil {
		t.Fatalf("plan first: %v", err)
	}
	second, err := planPublicExactFile(
		secondPath,
		[]byte("second\n"),
		0o644,
	)
	if err != nil {
		t.Fatalf("plan second: %v", err)
	}
	if err := os.WriteFile(secondPath, []byte("foreign\n"), 0o644); err != nil {
		t.Fatalf("change second path: %v", err)
	}
	receipt, err := applyPublicExactFileEffects(
		context.Background(),
		[]publicExactFileEffect{first, second},
		[]string{"haft", "init", "--overseer"},
	)
	if err == nil {
		t.Fatal("stale preview was applied")
	}
	if _, err := os.Stat(firstPath); !os.IsNotExist(err) {
		t.Fatalf("precondition failure wrote first path: %v", err)
	}
	if len(receipt.Completed()) != 0 ||
		len(receipt.Untouched()) != 2 ||
		len(receipt.Retry()) != 2 {
		t.Fatalf("stale receipt = %#v", receipt)
	}
}
