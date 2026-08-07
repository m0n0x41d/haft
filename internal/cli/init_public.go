package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/m0n0x41d/haft/internal/initplanning"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

const publicInitPlaceholderProjectID = "qnt_00000000"

type publicInitTerminalProbe func(*cobra.Command) bool

type publicInitSelectionRunner func(
	initplanning.InteractiveSession,
	io.Reader,
	io.Writer,
) (initplanning.InteractiveOutcome, error)

type typedPublicInitEffectRunner func(
	*cobra.Command,
	[]string,
	initplanning.InvocationPolicy,
) error

func runPublicInit(cmd *cobra.Command, args []string) error {
	return runPublicInitWithTypedEffect(
		cmd,
		args,
		publicInitHasTerminal,
		runInitSelectionTUI,
		runTypedPublicInit,
	)
}

func runPublicInitWithTypedEffect(
	cmd *cobra.Command,
	args []string,
	hasTerminal publicInitTerminalProbe,
	selectHosts publicInitSelectionRunner,
	apply typedPublicInitEffectRunner,
) error {
	if cmd == nil {
		return fmt.Errorf("init command is required")
	}
	if hasExplicitInitFlags(cmd) {
		if err := validateExplicitInitSelection(cmd); err != nil {
			return err
		}
		return apply(
			cmd,
			args,
			initplanning.InvocationExplicit,
		)
	}
	if !hasTerminal(cmd) {
		return fmt.Errorf(
			"bare 'haft init' requires an interactive terminal; no files were changed; use 'haft init --core-only' or explicit host flags such as '--codex' or '--claude'",
		)
	}

	session, err := newPublicInitSelectionSession()
	if err != nil {
		return fmt.Errorf("prepare initialization selection: %w", err)
	}
	outcome, err := selectHosts(
		session,
		cmd.InOrStdin(),
		cmd.OutOrStdout(),
	)
	if err != nil {
		return fmt.Errorf("select initialization hosts: %w", err)
	}

	switch typed := outcome.(type) {
	case initplanning.InteractiveConfirmedOutcome:
		restore := captureInitHostFlagState()
		defer restore.apply()
		if err := applyInteractiveInitIntent(typed.Intent); err != nil {
			return err
		}
		return apply(
			cmd,
			args,
			initplanning.InvocationInteractive,
		)
	case initplanning.InteractiveCancelledOutcome:
		_, _ = fmt.Fprintln(
			cmd.OutOrStdout(),
			"Initialization cancelled; no files were changed.",
		)
		return nil
	case initplanning.InteractiveEOFOutcome:
		_, _ = fmt.Fprintln(
			cmd.OutOrStdout(),
			"Initialization input ended; no files were changed.",
		)
		return nil
	default:
		return fmt.Errorf(
			"initialization selection returned a non-terminal outcome",
		)
	}
}

func hasExplicitInitFlags(cmd *cobra.Command) bool {
	return cmd.Flags().NFlag() > 0
}

func validateExplicitInitSelection(cmd *cobra.Command) error {
	if initCoreOnly && initMCPOnly {
		return fmt.Errorf(
			"--core-only cannot be combined with --mcp-only; no files were changed",
		)
	}
	if initMCPOnly && !hasRequestedInitHost(requestedInitHostOptions()) {
		return fmt.Errorf(
			"--mcp-only requires an explicit host flag or --all; no files were changed",
		)
	}
	if !hasExplicitInitTarget(cmd) {
		return fmt.Errorf(
			"init modifiers require an explicit target such as --claude, --codex, --agents, --overseer, or --core-only; no files were changed",
		)
	}
	if !initCoreOnly {
		return nil
	}
	if initAgents {
		return fmt.Errorf(
			"--core-only cannot be combined with --agents; no files were changed",
		)
	}
	if hasRequestedInitHost(requestedInitHostOptions()) {
		return fmt.Errorf(
			"--core-only cannot be combined with host flags or --all; no files were changed",
		)
	}
	if initOverseer ||
		initFlagChanged(
			cmd,
			"overseer",
			"overseer-reviewer",
			"overseer-reviewer-command",
			"overseer-review-on-hook",
			"overseer-review-timeout",
		) {
		return fmt.Errorf(
			"--core-only cannot be combined with overseer installation flags; no files were changed",
		)
	}
	return nil
}

func hasExplicitInitTarget(
	cmd *cobra.Command,
) bool {
	if initCoreOnly ||
		initAgents ||
		hasRequestedInitHost(requestedInitHostOptions()) ||
		publicOverseerRequested(cmd) {
		return true
	}
	return initFlagChanged(cmd, "scope-id")
}

func initFlagChanged(
	cmd *cobra.Command,
	names ...string,
) bool {
	for _, name := range names {
		if cmd.Flags().Changed(name) {
			return true
		}
	}
	return false
}

func requestedInitHostOptions() initHostOptions {
	return initHostOptions{
		claude:   initClaude,
		cursor:   initCursor,
		gemini:   initGemini,
		codex:    initCodex,
		air:      initAir,
		opencode: initOpencode,
		hermes:   initHermes,
		zed:      initZed,
		agy:      initAgy,
		pi:       initPi,
		grok:     initGrok,
		all:      initAll,
	}
}

func hasRequestedInitHost(options initHostOptions) bool {
	return hasInitHost(options)
}

func publicInitHasTerminal(cmd *cobra.Command) bool {
	input, inputOK := cmd.InOrStdin().(*os.File)
	output, outputOK := cmd.OutOrStdout().(*os.File)
	if !inputOK || !outputOK {
		return false
	}
	return publicInitFileIsTerminal(input) &&
		publicInitFileIsTerminal(output)
}

func publicInitFileIsTerminal(file *os.File) bool {
	descriptor := file.Fd()
	return isatty.IsTerminal(descriptor) ||
		isatty.IsCygwinTerminal(descriptor)
}

func newPublicInitSelectionSession() (
	initplanning.InteractiveSession,
	error,
) {
	projectRoot, err := os.Getwd()
	if err != nil {
		return initplanning.InteractiveSession{}, err
	}
	projectRoot, err = filepath.Abs(projectRoot)
	if err != nil {
		return initplanning.InteractiveSession{}, err
	}
	projectRoot = filepath.Clean(projectRoot)

	projectID, err := publicInitProjectID(projectRoot)
	if err != nil {
		return initplanning.InteractiveSession{}, err
	}
	catalog, choices, err := publicInitAdapterCatalog()
	if err != nil {
		return initplanning.InteractiveSession{}, err
	}
	discoveries, err := publicInitDiscoveries(choices)
	if err != nil {
		return initplanning.InteractiveSession{}, err
	}
	input := initplanning.InteractiveSessionInput{
		ProjectRoot: projectRoot,
		ProjectID:   projectID,
		Choices:     choices,
		Discoveries: discoveries,
	}
	return initplanning.NewInteractiveSession(input, catalog)
}

func publicInitProjectID(projectRoot string) (string, error) {
	haftDir := filepath.Join(projectRoot, ".haft")
	config, err := project.Load(haftDir)
	if err != nil {
		return "", err
	}
	if config == nil {
		return publicInitPlaceholderProjectID, nil
	}
	return config.ID, nil
}

func publicInitAdapterCatalog() (
	initplanning.AdapterCatalog,
	[]initplanning.WeakHostSelection,
	error,
) {
	registry, err := currentCoherentHostApplicabilityRegistry()
	if err != nil {
		return initplanning.AdapterCatalog{}, nil, err
	}
	keys := make([]currentCoherentHostKey, 0, len(registry))
	for key := range registry {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(left int, right int) bool {
		if keys[left].host == keys[right].host {
			return keys[left].scope < keys[right].scope
		}
		return keys[left].host < keys[right].host
	})

	capabilities := make(
		[]initplanning.AdapterCapability,
		0,
		len(keys),
	)
	choices := make(
		[]initplanning.WeakHostSelection,
		0,
		len(keys),
	)
	for _, key := range keys {
		applicability := registry[key]
		components := includedInitComponents(applicability)
		componentSet, err := initplanning.ParseComponentSet(components)
		if err != nil {
			return initplanning.AdapterCatalog{}, nil, err
		}
		capability, err := initplanning.NewAdapterCapabilityBuilder(key.host).
			AtEdition("public-init-menu.v1").
			Allow(key.scope, componentSet).
			Build()
		if err != nil {
			return initplanning.AdapterCatalog{}, nil, err
		}
		capabilities = append(capabilities, capability)
		choices = append(
			choices,
			initplanning.WeakHostSelection{
				Host:       string(key.host),
				Scope:      string(key.scope),
				Components: components,
			},
		)
	}
	catalog, err := initplanning.NewAdapterCatalog(capabilities)
	if err != nil {
		return initplanning.AdapterCatalog{}, nil, err
	}
	return catalog, choices, nil
}

func includedInitComponents(
	applicability initplanning.HostComponentApplicability,
) []string {
	components := make([]string, 0)
	for _, record := range applicability.Records() {
		if record.Disposition != initplanning.ComponentIncluded {
			continue
		}
		components = append(components, string(record.Component))
	}
	return components
}

func publicInitDiscoveries(
	choices []initplanning.WeakHostSelection,
) ([]initplanning.DiscoveryObservation, error) {
	result := make(
		[]initplanning.DiscoveryObservation,
		0,
		len(choices),
	)
	for _, choice := range choices {
		host, err := initplanning.ParseHostID(choice.Host)
		if err != nil {
			return nil, err
		}
		binary := publicInitHostBinary(host)
		if binary == "" {
			continue
		}
		if _, err := exec.LookPath(binary); err != nil {
			continue
		}
		observation, err := initplanning.NewDiscoveryObservation(
			host,
			"binary_found_on_path",
			initplanning.DiscoveryDetected,
		)
		if err != nil {
			return nil, err
		}
		result = append(result, observation)
	}
	return result, nil
}

func publicInitHostBinary(host initplanning.HostID) string {
	binaries := map[initplanning.HostID]string{
		initplanning.HostClaude:      "claude",
		initplanning.HostCodex:       "codex",
		initplanning.HostCursor:      "cursor",
		initplanning.HostGemini:      "gemini",
		initplanning.HostAntigravity: "agy",
		initplanning.HostGrok:        "grok",
		initplanning.HostHermes:      "hermes",
		initplanning.HostZed:         "zed",
		initplanning.HostOpenCode:    "opencode",
		initplanning.HostAir:         "air",
		initplanning.HostPi:          "pi",
	}
	return binaries[host]
}

type initHostFlagState struct {
	claude             bool
	cursor             bool
	gemini             bool
	codex              bool
	air                bool
	opencode           bool
	hermes             bool
	zed                bool
	agy                bool
	pi                 bool
	grok               bool
	all                bool
	coreOnly           bool
	agents             bool
	mcpOnly            bool
	local              bool
	noFileInstructions bool
	scopeID            string
}

func captureInitHostFlagState() initHostFlagState {
	return initHostFlagState{
		claude:             initClaude,
		cursor:             initCursor,
		gemini:             initGemini,
		codex:              initCodex,
		air:                initAir,
		opencode:           initOpencode,
		hermes:             initHermes,
		zed:                initZed,
		agy:                initAgy,
		pi:                 initPi,
		grok:               initGrok,
		all:                initAll,
		coreOnly:           initCoreOnly,
		agents:             initAgents,
		mcpOnly:            initMCPOnly,
		local:              initLocal,
		noFileInstructions: initNoFileInstructions,
		scopeID:            initScopeID,
	}
}

func (state initHostFlagState) apply() {
	initClaude = state.claude
	initCursor = state.cursor
	initGemini = state.gemini
	initCodex = state.codex
	initAir = state.air
	initOpencode = state.opencode
	initHermes = state.hermes
	initZed = state.zed
	initAgy = state.agy
	initPi = state.pi
	initGrok = state.grok
	initAll = state.all
	initCoreOnly = state.coreOnly
	initAgents = state.agents
	initMCPOnly = state.mcpOnly
	initLocal = state.local
	initNoFileInstructions = state.noFileInstructions
	initScopeID = state.scopeID
}

func applyInteractiveInitIntent(
	intent initplanning.InitIntent,
) error {
	clearInitHostFlags()
	hosts := intent.SelectedHosts().Values()
	if len(hosts) == 0 {
		initCoreOnly = true
		return nil
	}
	for _, selection := range hosts {
		if err := selectInitHostFlag(selection.Host()); err != nil {
			return err
		}
	}
	return nil
}

func clearInitHostFlags() {
	initClaude = false
	initCursor = false
	initGemini = false
	initCodex = false
	initAir = false
	initOpencode = false
	initHermes = false
	initZed = false
	initAgy = false
	initPi = false
	initGrok = false
	initAll = false
	initCoreOnly = false
	initAgents = false
	initMCPOnly = false
	initLocal = false
	initNoFileInstructions = false
	initScopeID = ""
}

func selectInitHostFlag(host initplanning.HostID) error {
	switch host {
	case initplanning.HostClaude:
		initClaude = true
	case initplanning.HostCodex:
		initCodex = true
	case initplanning.HostCursor:
		initCursor = true
	case initplanning.HostGemini:
		initGemini = true
	case initplanning.HostAntigravity:
		initAgy = true
	case initplanning.HostGrok:
		initGrok = true
	case initplanning.HostHermes:
		initHermes = true
	case initplanning.HostZed:
		initZed = true
	case initplanning.HostOpenCode:
		initOpencode = true
	case initplanning.HostAir:
		initAir = true
	case initplanning.HostPi:
		initPi = true
	default:
		return fmt.Errorf(
			"interactive initialization selected unsupported host %q",
			host,
		)
	}
	return nil
}
