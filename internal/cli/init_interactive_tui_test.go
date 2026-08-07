package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/m0n0x41d/haft/internal/initplanning"
)

func TestPublicInitRealPTYSpaceEnterAppliesSelectedHost(
	t *testing.T,
) {
	t.Parallel()

	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("real PTY init oracle requires the Unix script utility")
	}
	scriptPath, err := exec.LookPath("script")
	if err != nil {
		t.Fatalf("find Unix script utility: %v", err)
	}
	sourceRoot := initPTYSourceRoot(t)
	binaryPath := buildInitPTYSourceBinary(t, sourceRoot)
	projectRoot := t.TempDir()
	isolatedHome := t.TempDir()

	transcript, err := runInitPTYSpaceEnter(
		scriptPath,
		binaryPath,
		projectRoot,
		isolatedHome,
	)
	if err != nil {
		t.Fatalf("run source haft init through PTY: %v", err)
	}
	for _, expected := range []string{
		"> [ ] Air (experimental) — MCP + skills",
		"Haft initialization complete",
		"Project core initialized (schema ",
		"Air: ",
		"Reload: required",
		"Hosts: Air",
	} {
		if !strings.Contains(transcript, expected) {
			t.Fatalf(
				"PTY transcript omits %q:\n%s",
				expected,
				transcript,
			)
		}
	}
	if _, err := os.Stat(
		filepath.Join(projectRoot, ".haft", "config.yaml"),
	); !os.IsNotExist(err) {
		t.Fatalf("PTY init created obsolete project config: %v", err)
	}
	for _, path := range []string{
		filepath.Join(
			projectRoot,
			".haft",
			"host-installations",
			"air.project.json",
		),
		filepath.Join(projectRoot, ".codex", "config.toml"),
		filepath.Join(
			projectRoot,
			"skills",
			"h-reason",
			"SKILL.md",
		),
	} {
		info, statErr := os.Stat(path)
		if statErr != nil || !info.Mode().IsRegular() {
			t.Fatalf(
				"PTY init effect is not a regular file %s: %v",
				path,
				statErr,
			)
		}
	}
	databases, err := filepath.Glob(
		filepath.Join(
			isolatedHome,
			".haft",
			"projects",
			"*",
			"haft.db",
		),
	)
	if err != nil {
		t.Fatalf("locate isolated canonical database: %v", err)
	}
	if len(databases) != 1 {
		t.Fatalf(
			"isolated canonical databases = %v, want exactly one",
			databases,
		)
	}
	info, err := os.Stat(databases[0])
	if err != nil || !info.Mode().IsRegular() {
		t.Fatalf(
			"PTY init did not route canonical database to isolated home: %v",
			err,
		)
	}
}

func TestInitSelectionTUIMapsSpaceAndEnterToPureInteractiveIntent(
	t *testing.T,
) {
	t.Parallel()

	session := mustInitSelectionTUISession(t)
	model, err := newInitSelectionTUIModel(session)
	if err != nil {
		t.Fatalf("newInitSelectionTUIModel: %v", err)
	}
	initial := model.View().Content
	for _, expected := range []string{
		"Core project/ledger",
		"[ ] Claude Code (stable) — MCP + global skills + CLAUDE.md",
		"[ ] Codex (stable) — MCP + global skills + AGENTS.md",
		"suggested: binary_found_on_path",
		"Enter apply",
	} {
		if !strings.Contains(initial, expected) {
			t.Fatalf("initial TUI view omits %q:\n%s", expected, initial)
		}
	}

	updated, cmd := model.Update(initSelectionKey(tea.KeySpace, " "))
	if cmd != nil {
		t.Fatal("Space unexpectedly terminated the selection TUI")
	}
	selected := updated.(initSelectionTUIModel)
	editing := selected.session.Outcome().(initplanning.InteractiveEditingOutcome)
	if editing.Options[0].Host != initplanning.HostClaude ||
		editing.Options[0].Selection != initplanning.SelectionSelected {
		t.Fatalf("Space selected the wrong option: %+v", editing.Options)
	}
	if !strings.Contains(
		selected.View().Content,
		"[x] Claude Code (stable) — MCP + global skills + CLAUDE.md",
	) {
		t.Fatalf("selected TUI view omitted checked row:\n%s", selected.View().Content)
	}

	updated, cmd = selected.Update(initSelectionKey(tea.KeyEnter, ""))
	if cmd == nil {
		t.Fatal("Enter did not terminate the confirmed selection TUI")
	}
	confirmed := updated.(initSelectionTUIModel)
	outcome, ok := confirmed.session.Outcome().(initplanning.InteractiveConfirmedOutcome)
	if !ok {
		t.Fatalf("Enter outcome = %T", confirmed.session.Outcome())
	}
	if strings.Contains(
		confirmed.View().Content,
		"Selection unavailable",
	) {
		t.Fatalf(
			"terminal TUI rendered a stale selection error:\n%s",
			confirmed.View().Content,
		)
	}
	hosts := outcome.Intent.SelectedHosts().Values()
	if len(hosts) != 1 ||
		hosts[0].Host() != initplanning.HostClaude ||
		outcome.Intent.InvocationPolicy() !=
			initplanning.InvocationInteractive {
		t.Fatalf("confirmed interactive intent = %+v", outcome.Intent)
	}
}

func TestInitSelectionTUIMapsCancelAndEOFToDistinctNoWriteOutcomes(
	t *testing.T,
) {
	t.Parallel()

	model, err := newInitSelectionTUIModel(
		mustInitSelectionTUISession(t),
	)
	if err != nil {
		t.Fatalf("newInitSelectionTUIModel: %v", err)
	}
	updated, cmd := model.Update(
		initSelectionKey(tea.KeyEscape, ""),
	)
	if cmd == nil {
		t.Fatal("Escape did not terminate the selection TUI")
	}
	cancelled := updated.(initSelectionTUIModel)
	if _, ok := cancelled.session.Outcome().(initplanning.InteractiveCancelledOutcome); !ok {
		t.Fatalf("Escape outcome = %T", cancelled.session.Outcome())
	}

	eof, err := runInitSelectionTUI(
		mustInitSelectionTUISession(t),
		bytes.NewReader(nil),
		io.Discard,
	)
	if err != nil {
		t.Fatalf("runInitSelectionTUI(EOF): %v", err)
	}
	if _, ok := eof.(initplanning.InteractiveEOFOutcome); !ok {
		t.Fatalf("EOF outcome = %T", eof)
	}
}

func TestInitSelectionTUIConfirmWithoutToggleIsExplicitCoreOnly(
	t *testing.T,
) {
	t.Parallel()

	model, err := newInitSelectionTUIModel(
		mustInitSelectionTUISession(t),
	)
	if err != nil {
		t.Fatalf("newInitSelectionTUIModel: %v", err)
	}
	updated, cmd := model.Update(initSelectionKey(tea.KeyEnter, ""))
	if cmd == nil {
		t.Fatal("Enter did not terminate the core-only selection TUI")
	}
	confirmed := updated.(initSelectionTUIModel)
	outcome, ok := confirmed.session.Outcome().(initplanning.InteractiveConfirmedOutcome)
	if !ok {
		t.Fatalf("core-only outcome = %T", confirmed.session.Outcome())
	}
	if len(outcome.Intent.SelectedHosts().Values()) != 0 {
		t.Fatalf(
			"core-only TUI confirmation selected hosts: %+v",
			outcome.Intent.SelectedHosts().Values(),
		)
	}
}

func TestInitSelectionTUIKeepsTerminalFileVisibleToBubbleTea(
	t *testing.T,
) {
	t.Parallel()

	terminalInput := os.Stdin
	if got := initSelectionProgramInput(terminalInput); got != terminalInput {
		t.Fatalf("terminal input was wrapped as %T", got)
	}
	nonTerminalInput := bytes.NewReader(nil)
	if _, ok := initSelectionProgramInput(
		nonTerminalInput,
	).(*initSelectionEOFReader); !ok {
		t.Fatal("non-terminal input did not receive EOF adapter")
	}
}

func initSelectionKey(
	code rune,
	text string,
) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{
		Code: code,
		Text: text,
	})
}

func mustInitSelectionTUISession(
	t *testing.T,
) initplanning.InteractiveSession {
	t.Helper()
	components, err := initplanning.ParseComponentSet(
		[]string{
			string(initplanning.ComponentMCP),
			string(initplanning.ComponentSkills),
		},
	)
	if err != nil {
		t.Fatalf("ParseComponentSet: %v", err)
	}
	claude, err := initplanning.NewAdapterCapabilityBuilder(
		initplanning.HostClaude,
	).
		AtEdition("claude.coherent.v2").
		Allow(initplanning.ScopeProject, components).
		Build()
	if err != nil {
		t.Fatalf("build Claude capability: %v", err)
	}
	codex, err := initplanning.NewAdapterCapabilityBuilder(
		initplanning.HostCodex,
	).
		AtEdition("codex.coherent.v2").
		Allow(initplanning.ScopeProject, components).
		Build()
	if err != nil {
		t.Fatalf("build Codex capability: %v", err)
	}
	catalog, err := initplanning.NewAdapterCatalog(
		[]initplanning.AdapterCapability{claude, codex},
	)
	if err != nil {
		t.Fatalf("NewAdapterCatalog: %v", err)
	}
	discovery, err := initplanning.NewDiscoveryObservation(
		initplanning.HostCodex,
		"binary_found_on_path",
		initplanning.DiscoveryDetected,
	)
	if err != nil {
		t.Fatalf("NewDiscoveryObservation: %v", err)
	}
	root := filepath.Join(t.TempDir(), "project")
	session, err := initplanning.NewInteractiveSession(
		initplanning.InteractiveSessionInput{
			ProjectRoot: root,
			ProjectID:   "qnt_e3149c17",
			Choices: []initplanning.WeakHostSelection{
				{
					Host:  string(initplanning.HostClaude),
					Scope: string(initplanning.ScopeProject),
					Components: []string{
						string(initplanning.ComponentMCP),
						string(initplanning.ComponentSkills),
					},
				},
				{
					Host:  string(initplanning.HostCodex),
					Scope: string(initplanning.ScopeProject),
					Components: []string{
						string(initplanning.ComponentMCP),
						string(initplanning.ComponentSkills),
					},
				},
			},
			Discoveries: []initplanning.DiscoveryObservation{
				discovery,
			},
		},
		catalog,
	)
	if err != nil {
		t.Fatalf("NewInteractiveSession: %v", err)
	}
	return session
}

type initPTYTranscript struct {
	mu      sync.Mutex
	content bytes.Buffer
	updates chan struct{}
}

func newInitPTYTranscript() *initPTYTranscript {
	return &initPTYTranscript{
		updates: make(chan struct{}, 1),
	}
}

func (transcript *initPTYTranscript) Write(
	content []byte,
) (int, error) {
	transcript.mu.Lock()
	count, err := transcript.content.Write(content)
	transcript.mu.Unlock()
	select {
	case transcript.updates <- struct{}{}:
	default:
	}
	return count, err
}

func (transcript *initPTYTranscript) String() string {
	transcript.mu.Lock()
	defer transcript.mu.Unlock()
	return transcript.content.String()
}

func (transcript *initPTYTranscript) Len() int {
	transcript.mu.Lock()
	defer transcript.mu.Unlock()
	return transcript.content.Len()
}

func (transcript *initPTYTranscript) waitFor(
	ctx context.Context,
	expected string,
) error {
	for {
		current := transcript.String()
		if strings.Contains(current, expected) {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf(
				"wait for %q: %w\ntranscript:\n%s",
				expected,
				ctx.Err(),
				current,
			)
		case <-transcript.updates:
		}
	}
}

func (transcript *initPTYTranscript) waitForGrowth(
	ctx context.Context,
	previous int,
) error {
	for {
		current := transcript.Len()
		if current > previous {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf(
				"wait for terminal redraw after Space: %w\ntranscript:\n%s",
				ctx.Err(),
				transcript.String(),
			)
		case <-transcript.updates:
		}
	}
}

func initPTYSourceRoot(t *testing.T) string {
	t.Helper()
	current, err := os.Getwd()
	if err != nil {
		t.Fatalf("resolve CLI package directory: %v", err)
	}
	root := filepath.Clean(filepath.Join(current, "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("resolve source root %s: %v", root, err)
	}
	return root
}

func buildInitPTYSourceBinary(
	t *testing.T,
	sourceRoot string,
) string {
	t.Helper()
	binaryPath := filepath.Join(t.TempDir(), "haft")
	ctx, cancel := context.WithTimeout(
		context.Background(),
		2*time.Minute,
	)
	defer cancel()
	command := exec.CommandContext(
		ctx,
		"go",
		"build",
		"-o",
		binaryPath,
		"./cmd/haft",
	)
	command.Dir = sourceRoot
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf(
			"build source haft binary: %v\n%s",
			err,
			output,
		)
	}
	return binaryPath
}

func runInitPTYSpaceEnter(
	scriptPath string,
	binaryPath string,
	projectRoot string,
	isolatedHome string,
) (string, error) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		45*time.Second,
	)
	defer cancel()
	command := initPTYCommand(ctx, scriptPath, binaryPath)
	command.Dir = projectRoot
	command.Env = initPTYEnvironment(
		os.Environ(),
		isolatedHome,
	)
	input, err := command.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("open PTY input: %w", err)
	}
	transcript := newInitPTYTranscript()
	command.Stdout = transcript
	command.Stderr = transcript
	if err := command.Start(); err != nil {
		return "", fmt.Errorf("start PTY init: %w", err)
	}
	stop := func() {
		_ = input.Close()
		_ = command.Process.Kill()
		_ = command.Wait()
	}
	if err := transcript.waitFor(
		ctx,
		"> [ ] Air (experimental) — MCP + skills",
	); err != nil {
		stop()
		return transcript.String(), err
	}
	beforeSpace := transcript.Len()
	if _, err := input.Write([]byte{' '}); err != nil {
		stop()
		return transcript.String(),
			fmt.Errorf("send Space to PTY init: %w", err)
	}
	if err := transcript.waitForGrowth(
		ctx,
		beforeSpace,
	); err != nil {
		stop()
		return transcript.String(), err
	}
	if _, err := input.Write([]byte{'\r'}); err != nil {
		stop()
		return transcript.String(),
			fmt.Errorf("send Enter to PTY init: %w", err)
	}
	if err := transcript.waitFor(
		ctx,
		"Haft initialization complete",
	); err != nil {
		stop()
		return transcript.String(), err
	}
	_ = input.Close()
	if err := command.Wait(); err != nil {
		return transcript.String(),
			fmt.Errorf("wait for PTY init: %w", err)
	}
	return transcript.String(), nil
}

func initPTYCommand(
	ctx context.Context,
	scriptPath string,
	binaryPath string,
) *exec.Cmd {
	commandLine := "stty rows 24 cols 80; exec " +
		quoteInitPTYShellArgument(binaryPath) +
		" init"
	if runtime.GOOS == "linux" {
		return exec.CommandContext(
			ctx,
			scriptPath,
			"-q",
			"-e",
			"-c",
			commandLine,
			"/dev/null",
		)
	}
	return exec.CommandContext(
		ctx,
		scriptPath,
		"-q",
		"/dev/null",
		"/bin/sh",
		"-c",
		commandLine,
	)
}

func quoteInitPTYShellArgument(value string) string {
	escaped := strings.ReplaceAll(value, "'", "'\"'\"'")
	return "'" + escaped + "'"
}

func initPTYEnvironment(
	current []string,
	isolatedHome string,
) []string {
	result := append([]string{}, current...)
	result = replaceInitPTYEnvironment(
		result,
		"HOME",
		isolatedHome,
	)
	result = replaceInitPTYEnvironment(
		result,
		"XDG_CONFIG_HOME",
		filepath.Join(isolatedHome, ".config"),
	)
	result = replaceInitPTYEnvironment(
		result,
		"TERM",
		"xterm-256color",
	)
	result = replaceInitPTYEnvironment(
		result,
		"TERM_PROGRAM",
		"Apple_Terminal",
	)
	return result
}

func replaceInitPTYEnvironment(
	current []string,
	key string,
	value string,
) []string {
	prefix := key + "="
	result := make([]string, 0, len(current)+1)
	for _, entry := range current {
		if strings.HasPrefix(entry, prefix) {
			continue
		}
		result = append(result, entry)
	}
	return append(result, prefix+value)
}
