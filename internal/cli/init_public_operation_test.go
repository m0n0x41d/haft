package cli

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/m0n0x41d/haft/internal/initexecution"
	"github.com/m0n0x41d/haft/internal/initfs"
	"github.com/m0n0x41d/haft/internal/initplanning"
)

func TestTypedPublicCodexOperationInitializesFullCodexIntegration(
	t *testing.T,
) {
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
	t.Setenv("HOME", homeRoot)
	request, err := compilePublicInitRequest(
		weakPublicInitRequest{
			invocation:  initplanning.InvocationExplicit,
			projectRoot: projectRoot,
			projectID:   "qnt_e3149c17",
			hosts:       initHostOptions{codex: true},
			overseer:    publicOverseerWeakDisabled(),
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
	preview, err := prepared.Preview()
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if len(preview.Base.Hosts) != 2 {
		t.Fatalf("preview hosts = %#v", preview.Base.Hosts)
	}
	previewComponents := make(map[initplanning.InstallScope][]initplanning.Component)
	for _, host := range preview.Base.Hosts {
		if host.Host != initplanning.HostCodex {
			t.Fatalf("preview host = %#v", host)
		}
		previewComponents[host.Scope] = host.Components
	}
	if !slices.Equal(
		previewComponents[initplanning.ScopeProject],
		[]initplanning.Component{
			initplanning.ComponentInstructions,
			initplanning.ComponentMCP,
		},
	) {
		t.Fatalf("project components = %v", previewComponents[initplanning.ScopeProject])
	}
	if !slices.Equal(
		previewComponents[initplanning.ScopeUser],
		[]initplanning.Component{initplanning.ComponentSkills},
	) {
		t.Fatalf("user components = %v", previewComponents[initplanning.ScopeUser])
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
	outcome, err := confirmed.Apply(context.Background(), executor)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if outcome.Kind() != publicInitApplied {
		t.Fatalf("outcome = %#v", outcome)
	}
	if _, err := os.Stat(
		filepath.Join(projectRoot, ".codex", "config.toml"),
	); err != nil {
		t.Fatalf("Codex config missing: %v", err)
	}
	for _, path := range []string{
		filepath.Join(projectRoot, "AGENTS.md"),
		filepath.Join(
			homeRoot,
			".agents",
			"skills",
			"h-reason",
			"SKILL.md",
		),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("Codex integration missing %s: %v", path, err)
		}
	}
	if _, err := os.Stat(
		filepath.Join(
			projectRoot,
			".haft",
			"host-installations",
			"codex.project.json",
		),
	); err != nil {
		t.Fatalf("Codex ownership manifest missing: %v", err)
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
	secondConfirmed, err := secondPrepared.ConfirmPreview(
		secondPreview,
	)
	if err != nil {
		t.Fatalf("second ConfirmPreview: %v", err)
	}
	secondOutcome, err := secondConfirmed.Apply(
		context.Background(),
		executor,
	)
	if err != nil {
		t.Fatalf("second Apply: %v", err)
	}
	if secondOutcome.Kind() != publicInitAlreadyCurrent {
		t.Fatalf("second outcome = %#v", secondOutcome)
	}
}

func TestTypedPublicOperationAppliesEveryExplicitHostFlag(
	t *testing.T,
) {
	testCases := []struct {
		name  string
		hosts initHostOptions
	}{
		{name: "claude", hosts: initHostOptions{claude: true}},
		{name: "cursor", hosts: initHostOptions{cursor: true}},
		{name: "gemini", hosts: initHostOptions{gemini: true}},
		{name: "codex", hosts: initHostOptions{codex: true}},
		{name: "air", hosts: initHostOptions{air: true}},
		{name: "opencode", hosts: initHostOptions{opencode: true}},
		{name: "zed", hosts: initHostOptions{zed: true}},
		{name: "antigravity", hosts: initHostOptions{agy: true}},
		{name: "pi", hosts: initHostOptions{pi: true}},
		{name: "grok", hosts: initHostOptions{grok: true}},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			homeRoot, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatalf("resolve home root: %v", err)
			}
			projectRoot, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatalf("resolve project root: %v", err)
			}
			t.Setenv("HOME", homeRoot)
			request, err := compilePublicInitRequest(
				weakPublicInitRequest{
					invocation:  initplanning.InvocationExplicit,
					projectRoot: projectRoot,
					projectID:   "qnt_e3149c17",
					hosts:       testCase.hosts,
					overseer:    publicOverseerWeakDisabled(),
				},
			)
			if err != nil {
				t.Fatalf("compilePublicInitRequest: %v", err)
			}
			runtime, err := currentHostPublicationRuntimeFromProcess()
			if err != nil {
				t.Fatalf(
					"currentHostPublicationRuntimeFromProcess: %v",
					err,
				)
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
			if len(preview.Base.Hosts) == 0 {
				t.Fatal("explicit host produced no host publication")
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
		})
	}
}

func TestTypedPublicMultiHostFailureReturnsPendingBindingsAndRetries(
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
	t.Setenv("HOME", homeRoot)
	request, err := compilePublicInitRequest(
		weakPublicInitRequest{
			invocation:  initplanning.InvocationExplicit,
			projectRoot: projectRoot,
			projectID:   "qnt_e3149c17",
			hosts:       initHostOptions{all: true},
			overseer:    publicOverseerWeakDisabled(),
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
	preview, err := prepared.Preview()
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	confirmed, err := prepared.ConfirmPreview(preview)
	if err != nil {
		t.Fatalf("ConfirmPreview: %v", err)
	}
	publisher, err := initfs.NewHostPublisher(1 << 20)
	if err != nil {
		t.Fatalf("NewHostPublisher: %v", err)
	}
	publications := 0
	failure := errors.New("injected second host failure")
	host := initexecution.HostPublicationFunc(func(
		batch initplanning.HostPublicationBatch,
		store initfs.ManifestStore,
	) (initfs.HostPublicationOutcome, error) {
		publications++
		if publications == 2 {
			return initfs.HostPublicationOutcome{}, failure
		}
		return publisher.Publish(batch, store)
	})
	baseExecutor, err := initexecution.NewExecutor(
		newPublicProjectCoreEffect(request, io.Discard),
		host,
	)
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}
	outcome, err := confirmed.Apply(
		context.Background(),
		typedPublicInitExecutor{base: baseExecutor},
	)
	if !errors.Is(err, failure) {
		t.Fatalf("Apply error = %v", err)
	}
	if outcome.Kind() != publicInitPublicationIncomplete ||
		outcome.Base().Kind() !=
			initexecution.InitExecutionHostIncomplete ||
		len(outcome.Base().PendingBindings()) != 3 {
		t.Fatalf("partial outcome = %#v", outcome)
	}

	retryPrepared, err := prepareTypedPublicInitOperation(
		context.Background(),
		request,
		runtime,
		io.Discard,
		1<<20,
	)
	if err != nil {
		t.Fatalf("prepare retry: %v", err)
	}
	retryPreview, err := retryPrepared.Preview()
	if err != nil {
		t.Fatalf("retry Preview: %v", err)
	}
	retryConfirmed, err := retryPrepared.ConfirmPreview(
		retryPreview,
	)
	if err != nil {
		t.Fatalf("confirm retry: %v", err)
	}
	retryExecutor, err := newTypedPublicInitExecutor(
		request,
		io.Discard,
		1<<20,
	)
	if err != nil {
		t.Fatalf("new retry executor: %v", err)
	}
	retryOutcome, err := retryConfirmed.Apply(
		context.Background(),
		retryExecutor,
	)
	if err != nil {
		t.Fatalf("retry Apply: %v", err)
	}
	if retryOutcome.Kind() != publicInitApplied {
		t.Fatalf("retry outcome = %#v", retryOutcome)
	}
}

func TestTypedPublicHostFailureAfterCoreReturnsAllPending(
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
	t.Setenv("HOME", homeRoot)
	request, err := compilePublicInitRequest(
		weakPublicInitRequest{
			invocation:  initplanning.InvocationExplicit,
			projectRoot: projectRoot,
			projectID:   "qnt_e3149c17",
			hosts:       initHostOptions{codex: true},
			overseer:    publicOverseerWeakDisabled(),
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
	preview, err := prepared.Preview()
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	confirmed, err := prepared.ConfirmPreview(preview)
	if err != nil {
		t.Fatalf("ConfirmPreview: %v", err)
	}
	failure := errors.New("injected first host failure")
	host := initexecution.HostPublicationFunc(func(
		initplanning.HostPublicationBatch,
		initfs.ManifestStore,
	) (initfs.HostPublicationOutcome, error) {
		return initfs.HostPublicationOutcome{}, failure
	})
	baseExecutor, err := initexecution.NewExecutor(
		newPublicProjectCoreEffect(request, io.Discard),
		host,
	)
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}
	outcome, err := confirmed.Apply(
		context.Background(),
		typedPublicInitExecutor{base: baseExecutor},
	)
	if !errors.Is(err, failure) {
		t.Fatalf("Apply error = %v", err)
	}
	if outcome.Kind() != publicInitPublicationIncomplete ||
		outcome.Base().Kind() !=
			initexecution.InitExecutionHostIncomplete ||
		len(outcome.Base().PendingBindings()) != 2 {
		t.Fatalf("partial outcome = %#v", outcome)
	}
	if _, present := outcome.Base().CoreReceipt(); !present {
		t.Fatal("host failure omitted the completed core receipt")
	}
	if _, err := os.Stat(
		filepath.Join(projectRoot, ".haft", "project.yaml"),
	); err != nil {
		t.Fatalf("completed core is missing: %v", err)
	}
	if _, err := os.Stat(
		filepath.Join(projectRoot, ".codex", "config.toml"),
	); !os.IsNotExist(err) {
		t.Fatalf("failed host was reported as published: %v", err)
	}
}

func TestTypedPublicCodexAndAgentsOperationCoalescesDuplicateSkillPublication(
	t *testing.T,
) {
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
	t.Setenv("HOME", homeRoot)
	request, err := compilePublicInitRequest(
		weakPublicInitRequest{
			invocation:  initplanning.InvocationExplicit,
			projectRoot: projectRoot,
			projectID:   "qnt_e3149c17",
			hosts:       initHostOptions{codex: true},
			agents:      true,
			overseer:    publicOverseerWeakDisabled(),
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
	preview, err := prepared.Preview()
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if len(preview.Base.Hosts) != 2 ||
		len(preview.AgentSkills) != 0 {
		t.Fatalf("combined preview = %#v", preview)
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
	outcome, err := confirmed.Apply(context.Background(), executor)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if outcome.Kind() != publicInitApplied {
		t.Fatalf("outcome = %#v", outcome)
	}
	if _, present := outcome.AgentSkills(); present {
		t.Fatal("combined outcome duplicated the Codex skill publication")
	}
	for _, path := range []string{
		filepath.Join(projectRoot, ".codex", "config.toml"),
		filepath.Join(
			homeRoot,
			".agents",
			"skills",
			"h-reason",
			"SKILL.md",
		),
		filepath.Join(
			homeRoot,
			".haft",
			"host-installations",
			"codex.user.json",
		),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected publication %s: %v", path, err)
		}
	}
}

func TestTypedPublicCorePreconditionFailureReturnsExactNoWriteReceipt(
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
	t.Setenv("HOME", homeRoot)
	request, err := compilePublicInitRequest(
		weakPublicInitRequest{
			invocation:  initplanning.InvocationExplicit,
			projectRoot: projectRoot,
			projectID:   "qnt_e3149c17",
			coreOnly:    true,
			overseer:    publicOverseerWeakDisabled(),
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
	preview, err := prepared.Preview()
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	confirmed, err := prepared.ConfirmPreview(preview)
	if err != nil {
		t.Fatalf("ConfirmPreview: %v", err)
	}
	changedPath := filepath.Join(
		projectRoot,
		".haft",
		"config.yaml",
	)
	if err := os.MkdirAll(filepath.Dir(changedPath), 0o755); err != nil {
		t.Fatalf("create concurrent core root: %v", err)
	}
	if err := os.WriteFile(
		changedPath,
		[]byte("concurrent: true\n"),
		0o644,
	); err != nil {
		t.Fatalf("write concurrent core carrier: %v", err)
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
	if err == nil {
		t.Fatal("changed core carrier was applied")
	}
	if outcome.Kind() != publicInitCoreUnconfirmed {
		t.Fatalf("outcome kind = %s", outcome.Kind())
	}
	receipt, present := outcome.CoreEffects()
	if !present {
		t.Fatal("core failure omitted its exact receipt")
	}
	if len(receipt.Completed()) != 0 ||
		receipt.Failed() != changedPath ||
		len(receipt.Untouched()) != len(preview.Base.Core.FileEffects)+1 ||
		len(receipt.Retry()) != len(preview.Base.Core.FileEffects)+1 ||
		!slices.Equal(
			receipt.Recovery(),
			[]string{"haft", "init", "--core-only"},
		) {
		t.Fatalf("core failure receipt = %#v", receipt)
	}
	for _, file := range preview.Base.Core.FileEffects {
		if file.Path == changedPath {
			continue
		}
		if _, err := os.Stat(file.Path); !os.IsNotExist(err) {
			t.Fatalf("precondition failure wrote %s: %v", file.Path, err)
		}
	}
	if _, err := os.Stat(
		preview.Base.Core.DatabasePath,
	); !os.IsNotExist(err) {
		t.Fatalf("precondition failure wrote database: %v", err)
	}
}

func TestTypedPublicCoordinationCoversCoreAndAncillaryPublications(
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
	t.Setenv("HOME", homeRoot)
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
	confirmed, err := prepared.ConfirmPreview(preview)
	if err != nil {
		t.Fatalf("ConfirmPreview: %v", err)
	}
	held, err := prepared.coordinator.TryAcquire(
		prepared.resources,
	)
	if err != nil {
		t.Fatalf("acquire competing operation: %v", err)
	}
	lease, acquired := held.Lease()
	if !acquired {
		t.Fatal("competing operation did not acquire coordination")
	}
	executor, err := newTypedPublicInitExecutor(
		request,
		io.Discard,
		1<<20,
	)
	if err != nil {
		t.Fatalf("newTypedPublicInitExecutor: %v", err)
	}
	busy, err := confirmed.Apply(
		context.Background(),
		executor,
	)
	if err != nil {
		t.Fatalf("busy Apply: %v", err)
	}
	if busy.Kind() != publicInitBusy {
		t.Fatalf("busy outcome = %#v", busy)
	}
	if _, err := os.Stat(
		filepath.Join(projectRoot, ".haft"),
	); !os.IsNotExist(err) {
		t.Fatalf("busy operation wrote project core: %v", err)
	}
	if _, err := os.Stat(
		filepath.Join(homeRoot, ".agents", "skills"),
	); !os.IsNotExist(err) {
		t.Fatalf("busy operation wrote agent skills: %v", err)
	}
	if err := lease.Release(); err != nil {
		t.Fatalf("release competing operation: %v", err)
	}
	applied, err := confirmed.Apply(
		context.Background(),
		executor,
	)
	if err != nil {
		t.Fatalf("retry Apply: %v", err)
	}
	if applied.Kind() != publicInitApplied {
		t.Fatalf("retry outcome = %#v", applied)
	}
}
