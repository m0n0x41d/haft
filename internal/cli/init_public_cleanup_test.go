package cli

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/initplanning"
)

func TestTypedPublicAgentInitPreviewsAndRemovesDeprecatedSkills(
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
	skillsRoot := filepath.Join(homeRoot, ".agents", "skills")
	deprecatedPath := filepath.Join(skillsRoot, "q-reason")
	unrelatedPath := filepath.Join(skillsRoot, "custom-skill")
	writePublicCleanupTestFile(
		t,
		filepath.Join(deprecatedPath, "SKILL.md"),
		[]byte("deprecated\n"),
	)
	writePublicCleanupTestFile(
		t,
		filepath.Join(unrelatedPath, "SKILL.md"),
		[]byte("operator owned\n"),
	)
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
	if len(preview.DeprecatedSkillCleanup) != 1 ||
		!slices.Equal(
			preview.DeprecatedSkillCleanup[0].Paths,
			[]string{deprecatedPath},
		) {
		t.Fatalf(
			"deprecated cleanup preview = %#v",
			preview.DeprecatedSkillCleanup,
		)
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
		t.Fatalf("outcome = %#v", outcome)
	}
	if _, err := os.Stat(deprecatedPath); !os.IsNotExist(err) {
		t.Fatalf("deprecated skill remains: %v", err)
	}
	if _, err := os.Stat(unrelatedPath); err != nil {
		t.Fatalf("unrelated skill was removed: %v", err)
	}
	second, err := prepareTypedPublicInitOperation(
		context.Background(),
		request,
		runtime,
		io.Discard,
		1<<20,
	)
	if err != nil {
		t.Fatalf("prepare second operation: %v", err)
	}
	secondPreview, err := second.Preview()
	if err != nil {
		t.Fatalf("second Preview: %v", err)
	}
	if len(secondPreview.DeprecatedSkillCleanup) != 0 {
		t.Fatalf(
			"second cleanup preview = %#v",
			secondPreview.DeprecatedSkillCleanup,
		)
	}
}

func TestDeprecatedSkillCleanupRejectsChangedTreeBeforeRemoval(
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
	deprecatedPath := filepath.Join(
		homeRoot,
		".agents",
		"skills",
		"h-abduct",
	)
	skillPath := filepath.Join(deprecatedPath, "SKILL.md")
	writePublicCleanupTestFile(t, skillPath, []byte("before\n"))
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
	plan, err := compilePublicDeprecatedSkillCleanupPlan(
		request,
		runtime,
		publicHermesPlan{},
		false,
	)
	if err != nil {
		t.Fatalf("compile cleanup plan: %v", err)
	}
	if err := os.WriteFile(
		skillPath,
		[]byte("after\n"),
		0o644,
	); err != nil {
		t.Fatalf("mutate deprecated skill: %v", err)
	}
	receipt, err := applyPublicDeprecatedSkillCleanupPlan(
		context.Background(),
		plan,
	)
	if err == nil ||
		!strings.Contains(err.Error(), "changed after preview") {
		t.Fatalf("apply error = %v", err)
	}
	if len(receipt.Completed()) != 0 ||
		receipt.Failed() != deprecatedPath ||
		!slices.Equal(receipt.Retry(), []string{deprecatedPath}) {
		t.Fatalf("failure receipt = %#v", receipt)
	}
	content, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("changed tree was removed: %v", err)
	}
	if string(content) != "after\n" {
		t.Fatalf("changed content = %q", content)
	}
}

func TestDeprecatedSkillCleanupRejectsSymlinkSwapBeforeRemoval(
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
	deprecatedPath := filepath.Join(
		homeRoot,
		".agents",
		"skills",
		"h-abduct",
	)
	skillPath := filepath.Join(deprecatedPath, "SKILL.md")
	writePublicCleanupTestFile(t, skillPath, []byte("before\n"))
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
	plan, err := compilePublicDeprecatedSkillCleanupPlan(
		request,
		runtime,
		publicHermesPlan{},
		false,
	)
	if err != nil {
		t.Fatalf("compile cleanup plan: %v", err)
	}
	foreignPath := filepath.Join(homeRoot, "foreign-skill.md")
	writePublicCleanupTestFile(t, foreignPath, []byte("foreign\n"))
	if err := os.Remove(skillPath); err != nil {
		t.Fatalf("remove previewed skill: %v", err)
	}
	if err := os.Symlink(foreignPath, skillPath); err != nil {
		t.Skipf("create symlink fixture: %v", err)
	}

	receipt, err := applyPublicDeprecatedSkillCleanupPlan(
		context.Background(),
		plan,
	)
	if err == nil ||
		!strings.Contains(err.Error(), "contains a symlink") {
		t.Fatalf("apply error = %v", err)
	}
	if len(receipt.Completed()) != 0 ||
		receipt.Failed() != deprecatedPath ||
		!slices.Equal(receipt.Retry(), []string{deprecatedPath}) {
		t.Fatalf("failure receipt = %#v", receipt)
	}
	foreignContent, err := os.ReadFile(foreignPath)
	if err != nil {
		t.Fatalf("read foreign target: %v", err)
	}
	if string(foreignContent) != "foreign\n" {
		t.Fatalf("foreign target content = %q", foreignContent)
	}
	if _, err := os.Lstat(skillPath); err != nil {
		t.Fatalf("symlink was removed: %v", err)
	}
}

func TestDeprecatedSkillCleanupIncludesSelectedHostWithoutAgents(
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
	deprecatedPath := filepath.Join(
		projectRoot,
		".claude",
		"skills",
		"h-boundary-unpack",
	)
	writePublicCleanupTestFile(
		t,
		filepath.Join(deprecatedPath, "SKILL.md"),
		[]byte("deprecated\n"),
	)
	request, err := compilePublicInitRequest(
		weakPublicInitRequest{
			invocation:  initplanning.InvocationExplicit,
			projectRoot: projectRoot,
			projectID:   "qnt_e3149c17",
			local:       true,
			hosts:       initHostOptions{claude: true},
			overseer:    publicOverseerWeakDisabled(),
		},
	)
	if err != nil {
		t.Fatalf("compilePublicInitRequest: %v", err)
	}
	if request.agentSkills != publicAgentSkillsNone {
		t.Fatalf("request unexpectedly selected agents: %#v", request)
	}
	runtime, err := currentHostPublicationRuntimeFromProcess()
	if err != nil {
		t.Fatalf("currentHostPublicationRuntimeFromProcess: %v", err)
	}
	runtime.userHomeRoot = homeRoot
	plan, err := compilePublicDeprecatedSkillCleanupPlan(
		request,
		runtime,
		publicHermesPlan{},
		false,
	)
	if err != nil {
		t.Fatalf("compile cleanup plan: %v", err)
	}
	preview := previewPublicDeprecatedSkillCleanup(plan)
	if !slices.Equal(preview.Paths, []string{deprecatedPath}) {
		t.Fatalf("cleanup paths = %#v", preview.Paths)
	}
}

func TestTypedPublicLegacyCommandCleanupSelectsOnlyStableReplacementSurfaces(
	t *testing.T,
) {
	tests := []struct {
		name    string
		hosts   initHostOptions
		local   bool
		agents  bool
		mcpOnly bool
		want    []string
	}{
		{
			name:  "claude user",
			hosts: initHostOptions{claude: true},
			want:  []string{"claude_user"},
		},
		{
			name:  "claude project",
			hosts: initHostOptions{claude: true},
			local: true,
			want:  []string{"claude_project"},
		},
		{
			name:  "codex user",
			hosts: initHostOptions{codex: true},
			want:  []string{"codex_user"},
		},
		{
			name:  "codex local still replaces user prompts",
			hosts: initHostOptions{codex: true},
			local: true,
			want:  []string{"codex_user"},
		},
		{
			name:  "all stable hosts",
			hosts: initHostOptions{all: true},
			want:  []string{"claude_user", "codex_user"},
		},
		{
			name:  "all stable hosts local",
			hosts: initHostOptions{all: true},
			local: true,
			want:  []string{"claude_project", "codex_user"},
		},
		{
			name:    "mcp only has no replacement skills",
			hosts:   initHostOptions{codex: true},
			mcpOnly: true,
		},
		{
			name:   "independent agents do not own codex prompts",
			agents: true,
		},
		{
			name:  "experimental host does not take command ownership",
			hosts: initHostOptions{cursor: true},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			homeRoot := publicCleanupPhysicalTempDir(t)
			projectRoot := publicCleanupPhysicalTempDir(t)
			commandPaths := map[string]string{
				"claude_user": filepath.Join(
					homeRoot,
					".claude",
					"commands",
					"h-frame.md",
				),
				"claude_project": filepath.Join(
					projectRoot,
					".claude",
					"commands",
					"h-frame.md",
				),
				"codex_user": filepath.Join(
					homeRoot,
					".codex",
					"prompts",
					"h-frame.md",
				),
			}
			for _, path := range commandPaths {
				writePublicCleanupTestFile(
					t,
					path,
					[]byte("legacy command\n"),
				)
			}
			_, _, plan := compilePublicCleanupTestPlan(
				t,
				homeRoot,
				projectRoot,
				weakPublicInitRequest{
					hosts:   test.hosts,
					local:   test.local,
					agents:  test.agents,
					mcpOnly: test.mcpOnly,
				},
			)
			got := previewPublicLegacyCommandCleanup(plan).Paths
			want := make([]string, 0, len(test.want))
			for _, key := range test.want {
				want = append(want, commandPaths[key])
			}
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Fatalf("legacy command cleanup paths = %v, want %v", got, want)
			}
		})
	}
}

func TestTypedPublicLegacyCommandCleanupRemovesExactFilesAndPreservesForeignEntries(
	t *testing.T,
) {
	homeRoot := publicCleanupPhysicalTempDir(t)
	projectRoot := publicCleanupPhysicalTempDir(t)
	t.Setenv("HOME", homeRoot)
	claudeRoot := filepath.Join(homeRoot, ".claude", "commands")
	codexRoot := filepath.Join(homeRoot, ".codex", "prompts")
	legacyPaths := []string{
		filepath.Join(claudeRoot, "h-frame.md"),
		filepath.Join(codexRoot, "h-frame.md"),
	}
	for _, path := range legacyPaths {
		writePublicCleanupTestFile(t, path, []byte("legacy command\n"))
	}
	preserved := map[string][]byte{
		filepath.Join(claudeRoot, "custom.md"):           []byte("custom\n"),
		filepath.Join(claudeRoot, "h-frame.txt"):         []byte("text\n"),
		filepath.Join(codexRoot, "h-frame.md.bak"):       []byte("backup\n"),
		filepath.Join(codexRoot, "nested", "h-frame.md"): []byte("nested\n"),
	}
	for path, content := range preserved {
		writePublicCleanupTestFile(t, path, content)
	}
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
	if len(preview.LegacyCommandCleanup) != 1 ||
		!slices.Equal(
			preview.LegacyCommandCleanup[0].Paths,
			legacyPaths,
		) {
		t.Fatalf(
			"legacy command cleanup preview = %#v",
			preview.LegacyCommandCleanup,
		)
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
	receipt, present := outcome.LegacyCommandCleanup()
	if !present || !slices.Equal(receipt.Completed(), legacyPaths) {
		t.Fatalf("legacy command cleanup receipt = %#v, present = %t", receipt, present)
	}
	var rendered strings.Builder
	if err := renderTypedPublicInitOutcome(&rendered, outcome); err != nil {
		t.Fatalf("renderTypedPublicInitOutcome: %v", err)
	}
	for _, want := range []string{
		"Removed 2 legacy command files",
		"Reload: required",
		"Hosts: Claude Code, Codex",
		"Changed: commands_changed",
	} {
		if !strings.Contains(rendered.String(), want) {
			t.Fatalf("rendered outcome lacks %q:\n%s", want, rendered.String())
		}
	}
	for _, path := range legacyPaths {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("legacy command %s remains: %v", path, err)
		}
	}
	for path, want := range preserved {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read preserved path %s: %v", path, err)
		}
		if string(got) != string(want) {
			t.Fatalf("preserved path %s = %q, want %q", path, got, want)
		}
	}
	second, err := prepareTypedPublicInitOperation(
		context.Background(),
		request,
		runtime,
		io.Discard,
		1<<20,
	)
	if err != nil {
		t.Fatalf("prepare second operation: %v", err)
	}
	secondPreview, err := second.Preview()
	if err != nil {
		t.Fatalf("second Preview: %v", err)
	}
	if len(secondPreview.LegacyCommandCleanup) != 0 {
		t.Fatalf(
			"second legacy command cleanup preview = %#v",
			secondPreview.LegacyCommandCleanup,
		)
	}
}

func TestTypedPublicLegacyCommandCleanupRejectsChangedFileBeforeAnyRemoval(
	t *testing.T,
) {
	homeRoot := publicCleanupPhysicalTempDir(t)
	projectRoot := publicCleanupPhysicalTempDir(t)
	claudePath := filepath.Join(
		homeRoot,
		".claude",
		"commands",
		"h-frame.md",
	)
	codexPath := filepath.Join(
		homeRoot,
		".codex",
		"prompts",
		"h-frame.md",
	)
	for _, path := range []string{claudePath, codexPath} {
		writePublicCleanupTestFile(t, path, []byte("before\n"))
	}
	_, _, plan := compilePublicCleanupTestPlan(
		t,
		homeRoot,
		projectRoot,
		weakPublicInitRequest{hosts: initHostOptions{all: true}},
	)
	if err := os.WriteFile(codexPath, []byte("after\n"), 0o644); err != nil {
		t.Fatalf("mutate legacy command: %v", err)
	}
	receipt, err := applyPublicDeprecatedSkillCleanupPlan(
		context.Background(),
		plan,
	)
	if err == nil ||
		!strings.Contains(err.Error(), "changed after preview") {
		t.Fatalf("apply error = %v", err)
	}
	wantPaths := []string{claudePath, codexPath}
	slices.Sort(wantPaths)
	if len(receipt.Completed()) != 0 ||
		receipt.Failed() != codexPath ||
		!slices.Equal(receipt.Untouched(), wantPaths) ||
		!slices.Equal(receipt.Retry(), wantPaths) {
		t.Fatalf("failure receipt = %#v", receipt)
	}
	if !slices.Equal(
		receipt.Recovery(),
		[]string{"haft", "init", "--claude", "--codex"},
	) {
		t.Fatalf("recovery argv = %v", receipt.Recovery())
	}
	for _, path := range wantPaths {
		if _, err := os.Lstat(path); err != nil {
			t.Fatalf("preflight failure removed %s: %v", path, err)
		}
	}
}

func TestTypedPublicLegacyCommandCleanupRejectsSymlinkAndNonRegularTargets(
	t *testing.T,
) {
	t.Run("symlink swap", func(t *testing.T) {
		homeRoot := publicCleanupPhysicalTempDir(t)
		projectRoot := publicCleanupPhysicalTempDir(t)
		commandPath := filepath.Join(
			homeRoot,
			".codex",
			"prompts",
			"h-frame.md",
		)
		writePublicCleanupTestFile(t, commandPath, []byte("before\n"))
		_, _, plan := compilePublicCleanupTestPlan(
			t,
			homeRoot,
			projectRoot,
			weakPublicInitRequest{
				hosts: initHostOptions{codex: true},
			},
		)
		foreignPath := filepath.Join(homeRoot, "foreign.md")
		writePublicCleanupTestFile(t, foreignPath, []byte("foreign\n"))
		if err := os.Remove(commandPath); err != nil {
			t.Fatalf("remove previewed command: %v", err)
		}
		if err := os.Symlink(foreignPath, commandPath); err != nil {
			t.Skipf("create symlink fixture: %v", err)
		}
		receipt, err := applyPublicDeprecatedSkillCleanupPlan(
			context.Background(),
			plan,
		)
		if err == nil || !strings.Contains(err.Error(), "is a symlink") {
			t.Fatalf("apply error = %v", err)
		}
		if len(receipt.Completed()) != 0 ||
			receipt.Failed() != commandPath ||
			!slices.Equal(receipt.Retry(), []string{commandPath}) {
			t.Fatalf("failure receipt = %#v", receipt)
		}
		got, err := os.ReadFile(foreignPath)
		if err != nil || string(got) != "foreign\n" {
			t.Fatalf("foreign target = %q, %v", got, err)
		}
	})
	t.Run("directory", func(t *testing.T) {
		homeRoot := publicCleanupPhysicalTempDir(t)
		projectRoot := publicCleanupPhysicalTempDir(t)
		t.Setenv("HOME", homeRoot)
		commandPath := filepath.Join(
			homeRoot,
			".codex",
			"prompts",
			"h-frame.md",
		)
		if err := os.MkdirAll(commandPath, 0o755); err != nil {
			t.Fatalf("create command-shaped directory: %v", err)
		}
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
		_, err = compilePublicDeprecatedSkillCleanupPlan(
			request,
			currentHostPublicationRuntime{userHomeRoot: homeRoot},
			publicHermesPlan{},
			false,
		)
		if err == nil ||
			!strings.Contains(err.Error(), "is not a regular file") {
			t.Fatalf("compile cleanup plan error = %v", err)
		}
		if info, statErr := os.Lstat(commandPath); statErr != nil ||
			!info.IsDir() {
			t.Fatalf("nonregular target changed: %v, %v", info, statErr)
		}
	})
}

func TestTypedPublicLegacyCommandCleanupRejectsModeDriftBeforeRemoval(
	t *testing.T,
) {
	homeRoot := publicCleanupPhysicalTempDir(t)
	projectRoot := publicCleanupPhysicalTempDir(t)
	commandPath := filepath.Join(
		homeRoot,
		".codex",
		"prompts",
		"h-frame.md",
	)
	writePublicCleanupTestFile(t, commandPath, []byte("legacy command\n"))
	_, _, plan := compilePublicCleanupTestPlan(
		t,
		homeRoot,
		projectRoot,
		weakPublicInitRequest{
			hosts: initHostOptions{codex: true},
		},
	)
	if err := os.Chmod(commandPath, 0o600); err != nil {
		t.Fatalf("change legacy command mode: %v", err)
	}
	receipt, err := applyPublicDeprecatedSkillCleanupPlan(
		context.Background(),
		plan,
	)
	if err == nil ||
		!strings.Contains(err.Error(), "changed after preview") {
		t.Fatalf("apply error = %v", err)
	}
	if len(receipt.Completed()) != 0 ||
		receipt.Failed() != commandPath ||
		!slices.Equal(receipt.Retry(), []string{commandPath}) {
		t.Fatalf("failure receipt = %#v", receipt)
	}
	info, err := os.Lstat(commandPath)
	if err != nil {
		t.Fatalf("mode-drifted command was removed: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode-drifted command mode = %o", info.Mode().Perm())
	}
}

func TestTypedPublicLegacyCommandCleanupRejectsParentSymlinkEscape(
	t *testing.T,
) {
	homeRoot := publicCleanupPhysicalTempDir(t)
	projectRoot := publicCleanupPhysicalTempDir(t)
	commandPath := filepath.Join(
		homeRoot,
		".codex",
		"prompts",
		"h-frame.md",
	)
	writePublicCleanupTestFile(t, commandPath, []byte("inside\n"))
	_, _, plan := compilePublicCleanupTestPlan(
		t,
		homeRoot,
		projectRoot,
		weakPublicInitRequest{
			hosts: initHostOptions{codex: true},
		},
	)
	originalRoot := filepath.Join(homeRoot, ".codex-original")
	if err := os.Rename(
		filepath.Join(homeRoot, ".codex"),
		originalRoot,
	); err != nil {
		t.Fatalf("move previewed Codex root: %v", err)
	}
	outsideRoot := publicCleanupPhysicalTempDir(t)
	outsidePath := filepath.Join(outsideRoot, "prompts", "h-frame.md")
	writePublicCleanupTestFile(t, outsidePath, []byte("outside\n"))
	if err := os.Symlink(
		outsideRoot,
		filepath.Join(homeRoot, ".codex"),
	); err != nil {
		t.Skipf("create parent symlink fixture: %v", err)
	}
	receipt, err := applyPublicDeprecatedSkillCleanupPlan(
		context.Background(),
		plan,
	)
	if err == nil ||
		!strings.Contains(err.Error(), "unsafe ancestor") {
		t.Fatalf("apply error = %v", err)
	}
	if len(receipt.Completed()) != 0 ||
		receipt.Failed() != commandPath ||
		!slices.Equal(receipt.Retry(), []string{commandPath}) {
		t.Fatalf("failure receipt = %#v", receipt)
	}
	outside, err := os.ReadFile(outsidePath)
	if err != nil || string(outside) != "outside\n" {
		t.Fatalf("outside target = %q, %v", outside, err)
	}
	original, err := os.ReadFile(
		filepath.Join(originalRoot, "prompts", "h-frame.md"),
	)
	if err != nil || string(original) != "inside\n" {
		t.Fatalf("original target = %q, %v", original, err)
	}
}

func TestTypedPublicLegacyCleanupHonorsCancellationBeforeRemoval(
	t *testing.T,
) {
	homeRoot := publicCleanupPhysicalTempDir(t)
	projectRoot := publicCleanupPhysicalTempDir(t)
	commandPath := filepath.Join(
		homeRoot,
		".codex",
		"prompts",
		"h-frame.md",
	)
	writePublicCleanupTestFile(t, commandPath, []byte("legacy command\n"))
	_, _, plan := compilePublicCleanupTestPlan(
		t,
		homeRoot,
		projectRoot,
		weakPublicInitRequest{
			hosts: initHostOptions{codex: true},
		},
	)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	receipt, err := applyPublicDeprecatedSkillCleanupPlan(ctx, plan)
	if err == nil || !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("apply error = %v", err)
	}
	if len(receipt.Completed()) != 0 ||
		receipt.Failed() != commandPath ||
		!slices.Equal(receipt.Untouched(), []string{commandPath}) ||
		!slices.Equal(receipt.Retry(), []string{commandPath}) {
		t.Fatalf("cancellation receipt = %#v", receipt)
	}
	if _, err := os.Lstat(commandPath); err != nil {
		t.Fatalf("cancellation removed command: %v", err)
	}
}

func TestTypedPublicLegacyCleanupRecoveryPreservesMCPOnlyScope(
	t *testing.T,
) {
	homeRoot := publicCleanupPhysicalTempDir(t)
	projectRoot := publicCleanupPhysicalTempDir(t)
	deprecatedPath := filepath.Join(
		homeRoot,
		".agents",
		"skills",
		"q-reason",
		"SKILL.md",
	)
	writePublicCleanupTestFile(t, deprecatedPath, []byte("deprecated\n"))
	_, _, plan := compilePublicCleanupTestPlan(
		t,
		homeRoot,
		projectRoot,
		weakPublicInitRequest{
			hosts:   initHostOptions{codex: true},
			agents:  true,
			mcpOnly: true,
		},
	)
	if !slices.Equal(
		plan.recovery,
		[]string{
			"haft",
			"init",
			"--codex",
			"--mcp-only",
			"--agents",
		},
	) {
		t.Fatalf("cleanup recovery argv = %v", plan.recovery)
	}
}

func TestTypedPublicPartialLegacyCleanupRendersReloadRequirement(
	t *testing.T,
) {
	t.Parallel()

	claudePath := "/cleanup/.claude/commands/h-frame.md"
	codexPath := "/cleanup/.codex/prompts/h-frame.md"
	plan := publicDeprecatedSkillCleanupPlan{
		removals: []publicDeprecatedSkillRemoval{
			{
				kind: publicLegacyCommandFileRemoval,
				host: initplanning.HostClaude,
				path: claudePath,
			},
			{
				kind: publicLegacyCommandFileRemoval,
				host: initplanning.HostCodex,
				path: codexPath,
			},
		},
		recovery: []string{"haft", "init", "--claude", "--codex"},
	}
	outcome := typedPublicInitOutcome{
		kind: publicInitPublicationIncomplete,
		cleanup: publicExactFileReceipt{
			completed: []string{claudePath},
			failed:    codexPath,
			retry:     []string{codexPath},
			recovery:  slices.Clone(plan.recovery),
		},
		cleanupPlan: plan,
		hasCleanup:  true,
	}
	var output strings.Builder
	if err := renderTypedPublicInitOutcome(&output, outcome); err != nil {
		t.Fatalf("renderTypedPublicInitOutcome: %v", err)
	}
	for _, want := range []string{
		"failed: " + codexPath,
		"Reload: required",
		"Hosts: Claude Code",
		"Changed: commands_changed",
	} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("partial outcome lacks %q:\n%s", want, output.String())
		}
	}
}

func compilePublicCleanupTestPlan(
	t *testing.T,
	homeRoot string,
	projectRoot string,
	input weakPublicInitRequest,
) (
	publicInitRequest,
	currentHostPublicationRuntime,
	publicDeprecatedSkillCleanupPlan,
) {
	t.Helper()
	t.Setenv("HOME", homeRoot)
	input.invocation = initplanning.InvocationExplicit
	input.projectRoot = projectRoot
	input.projectID = "qnt_e3149c17"
	input.overseer = publicOverseerWeakDisabled()
	request, err := compilePublicInitRequest(input)
	if err != nil {
		t.Fatalf("compilePublicInitRequest: %v", err)
	}
	runtime := currentHostPublicationRuntime{userHomeRoot: homeRoot}
	plan, err := compilePublicDeprecatedSkillCleanupPlan(
		request,
		runtime,
		publicHermesPlan{},
		false,
	)
	if err != nil {
		t.Fatalf("compile cleanup plan: %v", err)
	}
	return request, runtime, plan
}

func publicCleanupPhysicalTempDir(t *testing.T) string {
	t.Helper()
	path, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve temp directory: %v", err)
	}
	return path
}

func writePublicCleanupTestFile(
	t *testing.T,
	path string,
	content []byte,
) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create test directory: %v", err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
}
