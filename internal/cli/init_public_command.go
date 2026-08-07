package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/initexecution"
	"github.com/m0n0x41d/haft/internal/initfs"
	"github.com/m0n0x41d/haft/internal/initplanning"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/spf13/cobra"
)

const publicInitMaxCarrierBytes = int64(4 << 20)

func runTypedPublicInit(
	cmd *cobra.Command,
	_ []string,
	invocation initplanning.InvocationPolicy,
) error {
	request, err := currentPublicInitRequest(cmd, invocation)
	if err != nil {
		return err
	}
	runtime, err := currentHostPublicationRuntimeFromProcess()
	if err != nil {
		return err
	}
	output := initOutputWriter(cmd)
	prepared, err := prepareTypedPublicInitOperation(
		initCommandContext(cmd),
		request,
		runtime,
		output,
		publicInitMaxCarrierBytes,
	)
	if err != nil {
		return err
	}
	preview, err := prepared.Preview()
	if err != nil {
		return err
	}
	if preview.Base.Readiness == initplanning.PlanBlocked {
		return typedPublicInitBlockedError(preview)
	}
	confirmed, err := prepared.ConfirmPreview(preview)
	if err != nil {
		return err
	}
	executor, err := newTypedPublicInitExecutor(
		request,
		output,
		publicInitMaxCarrierBytes,
	)
	if err != nil {
		return err
	}
	outcome, applyErr := confirmed.Apply(
		initCommandContext(cmd),
		executor,
	)
	if err := renderTypedPublicInitOutcome(output, outcome); err != nil {
		return err
	}
	return applyErr
}

func typedPublicInitBlockedError(
	preview typedPublicInitPreview,
) error {
	for _, host := range preview.Base.Hosts {
		for _, effect := range host.Effects {
			if effect.Effect != initplanning.FileConflict {
				continue
			}
			return fmt.Errorf(
				"haft init cannot safely update %s: %s; no files were changed",
				effect.Path,
				effect.Reason,
			)
		}
	}
	return fmt.Errorf(
		"haft init cannot safely apply the selected configuration; no files were changed",
	)
}

func currentPublicInitRequest(
	cmd *cobra.Command,
	invocation initplanning.InvocationPolicy,
) (publicInitRequest, error) {
	projectRoot, err := os.Getwd()
	if err != nil {
		return publicInitRequest{},
			fmt.Errorf("resolve current project root: %w", err)
	}
	projectRoot, err = filepath.EvalSymlinks(projectRoot)
	if err != nil {
		return publicInitRequest{},
			fmt.Errorf("resolve physical project root: %w", err)
	}
	projectRoot = filepath.Clean(projectRoot)
	config, err := currentPublicProjectConfig(projectRoot)
	if err != nil {
		return publicInitRequest{}, err
	}
	overseerSelection := publicOverseerWeakDisabled()
	if publicOverseerRequested(cmd) {
		overseerSelection = publicOverseerWeakConfiguration{
			reviewer:     initOverseerReviewer,
			command:      initOverseerReviewerCommand,
			reviewOnHook: initOverseerReviewOnHook,
			timeout:      initOverseerReviewTimeout,
		}
	}
	if !initHermes &&
		(strings.TrimSpace(initHermesHome) != "" ||
			strings.TrimSpace(initHermesProfile) != "") {
		return publicInitRequest{}, fmt.Errorf(
			"--hermes-home and --profile require --hermes; no files were changed",
		)
	}
	return compilePublicInitRequest(
		weakPublicInitRequest{
			invocation:         invocation,
			projectRoot:        projectRoot,
			projectID:          config.ID,
			hosts:              requestedInitHostOptions(),
			local:              initLocal,
			agents:             initAgents,
			mcpOnly:            initMCPOnly,
			coreOnly:           initCoreOnly,
			omitInstructions:   initNoFileInstructions,
			profileScopeID:     initScopeID,
			overseer:           overseerSelection,
			hermesHomeInput:    initHermesHome,
			hermesProfileInput: initHermesProfile,
		},
	)
}

func currentPublicProjectConfig(
	projectRoot string,
) (project.Config, error) {
	current, err := project.Load(
		filepath.Join(projectRoot, ".haft"),
	)
	if err != nil {
		return project.Config{}, err
	}
	if current == nil {
		current, err = project.Load(
			filepath.Join(projectRoot, ".quint"),
		)
		if err != nil {
			return project.Config{}, err
		}
	}
	if current != nil {
		return project.Config{
			ID:   current.ID,
			Name: filepath.Base(projectRoot),
		}, nil
	}
	return project.ProposeConfig(projectRoot)
}

func publicOverseerRequested(
	cmd *cobra.Command,
) bool {
	if initOverseer {
		return true
	}
	return initFlagChanged(
		cmd,
		"overseer-reviewer",
		"overseer-reviewer-command",
		"overseer-review-on-hook",
		"overseer-review-timeout",
	)
}

func renderTypedPublicInitOutcome(
	output io.Writer,
	outcome typedPublicInitOutcome,
) error {
	if output == nil {
		return fmt.Errorf("typed public init outcome output is required")
	}
	if outcome.Kind() == publicInitApplied ||
		outcome.Kind() == publicInitAlreadyCurrent {
		return renderTypedPublicInitSuccess(output, outcome)
	}
	if _, err := fmt.Fprintf(
		output,
		"Haft initialization: %s\n",
		outcome.Kind(),
	); err != nil {
		return err
	}
	base := outcome.Base()
	if base.Reason() != "" {
		if _, err := fmt.Fprintf(
			output,
			"  base: %s\n",
			base.Reason(),
		); err != nil {
			return err
		}
	}
	if len(base.PendingBindings()) > 0 {
		pending := make(
			[]string,
			len(base.PendingBindings()),
		)
		for index, binding := range base.PendingBindings() {
			pending[index] = binding.String()
		}
		if _, err := fmt.Fprintf(
			output,
			"  pending hosts: %s\n",
			strings.Join(pending, ", "),
		); err != nil {
			return err
		}
	}
	for _, receipt := range base.HostReceipts() {
		recovery := receipt.Outcome().RecoveryArgv()
		if len(recovery) == 0 {
			continue
		}
		if _, err := fmt.Fprintf(
			output,
			"  %s recovery argv: %s\n",
			receipt.BindingID().String(),
			strings.Join(recovery, " "),
		); err != nil {
			return err
		}
	}
	if err := renderTypedPublicPartialReceipts(output, outcome); err != nil {
		return err
	}
	return renderPublicInitReloadReceipt(
		output,
		buildPublicInitReloadReceipt(outcome),
	)
}

func renderTypedPublicInitSuccess(
	output io.Writer,
	outcome typedPublicInitOutcome,
) error {
	headline := "Haft initialization complete"
	if outcome.Kind() == publicInitAlreadyCurrent {
		headline = "Haft is already initialized"
	}
	if _, err := fmt.Fprintln(output, headline); err != nil {
		return err
	}
	base := outcome.Base()
	core, hasCore := base.CoreReceipt()
	projectRoot := ""
	if hasCore {
		projectRoot = core.ProjectRoot()
		coreSummary := publicInitCoreSummary(core)
		if err := writePublicInitSuccessLine(
			output,
			coreSummary,
		); err != nil {
			return err
		}
		if err := writePublicInitSuccessLine(
			output,
			"Project memory ready",
		); err != nil {
			return err
		}
	}
	for _, receipt := range base.HostReceipts() {
		summary := publicInitHostSummary(
			receipt,
			projectRoot,
		)
		if err := writePublicInitSuccessLine(
			output,
			summary,
		); err != nil {
			return err
		}
	}
	if receipt, present := outcome.AgentSkills(); present {
		summary := publicInitAgentSkillsSummary(receipt)
		if err := writePublicInitSuccessLine(
			output,
			summary,
		); err != nil {
			return err
		}
	}
	if receipt, present := outcome.Hermes(); present {
		summary := publicInitExactFilesSummary(
			"Hermes integration",
			receipt,
			outcome.Kind(),
		)
		if err := writePublicInitSuccessLine(
			output,
			summary,
		); err != nil {
			return err
		}
	}
	if receipt, present := outcome.Overseer(); present {
		summary := publicInitExactFilesSummary(
			"Overseer",
			receipt,
			outcome.Kind(),
		)
		if err := writePublicInitSuccessLine(
			output,
			summary,
		); err != nil {
			return err
		}
	}
	if receipt, present := outcome.DeprecatedSkillCleanup(); present {
		summary := publicInitDeprecatedSkillCleanupSummary(receipt)
		if err := writePublicInitSuccessLine(
			output,
			summary,
		); err != nil {
			return err
		}
	}
	if receipt, present := outcome.LegacyCommandCleanup(); present {
		summary := publicInitLegacyCommandCleanupSummary(receipt)
		if err := writePublicInitSuccessLine(
			output,
			summary,
		); err != nil {
			return err
		}
	}
	if hasCore {
		if _, err := fmt.Fprintf(
			output,
			"  Project ID: %s\n",
			core.ProjectID(),
		); err != nil {
			return err
		}
	}
	if err := renderPublicInitOnboardingRecovery(output); err != nil {
		return err
	}
	return renderPublicInitReloadReceipt(
		output,
		buildPublicInitReloadReceipt(outcome),
	)
}

func renderPublicInitOnboardingRecovery(output io.Writer) error {
	if _, err := fmt.Fprintln(
		output,
		"Next setup surface: h-onboard status",
	); err != nil {
		return err
	}
	_, err := fmt.Fprintln(
		output,
		"Profile admission is separate from initialization and may still be required.",
	)
	return err
}

type publicInitReloadPosture string

const (
	publicInitReloadRequired      publicInitReloadPosture = "required"
	publicInitReloadNotRequired   publicInitReloadPosture = "not_required_by_this_run"
	publicInitReloadNotApplicable publicInitReloadPosture = "not_applicable"
)

type publicInitReloadReceipt struct {
	Posture publicInitReloadPosture `json:"posture"`
	Hosts   []string                `json:"hosts,omitempty"`
	Reasons []string                `json:"reasons,omitempty"`
}

func buildPublicInitReloadReceipt(
	outcome typedPublicInitOutcome,
) publicInitReloadReceipt {
	hostSet := map[string]struct{}{}
	reasonSet := map[string]struct{}{}
	hostSurfaceSelected := false
	for _, hostReceipt := range outcome.Base().HostReceipts() {
		hostSurfaceSelected = true
		for _, pathReceipt := range hostReceipt.Outcome().Receipts() {
			if !publicInitPathRequiresReload(pathReceipt.Step()) {
				continue
			}
			hostSet[publicInitHostLabel(hostReceipt.Host())] = struct{}{}
			for _, component := range pathReceipt.Components().Values() {
				reasonSet[publicInitReloadReason(component)] = struct{}{}
			}
		}
	}
	if agentReceipt, present := outcome.AgentSkills(); present {
		hostSurfaceSelected = true
		if agentReceipt.ChangedPaths() > 0 {
			hostSet["Codex-compatible agents"] = struct{}{}
			reasonSet["skills_changed"] = struct{}{}
		}
	}
	if commandReceipt, present := outcome.LegacyCommandCleanup(); present {
		hostSurfaceSelected = true
		completed := make(map[string]struct{})
		for _, path := range commandReceipt.Completed() {
			completed[path] = struct{}{}
		}
		for _, removal := range outcome.cleanupPlan.removals {
			if removal.kind != publicLegacyCommandFileRemoval {
				continue
			}
			if _, removed := completed[removal.path]; !removed {
				continue
			}
			hostSet[publicInitHostLabel(removal.host)] = struct{}{}
			reasonSet["commands_changed"] = struct{}{}
		}
	}
	if len(hostSet) == 0 {
		posture := publicInitReloadNotRequired
		if !hostSurfaceSelected {
			posture = publicInitReloadNotApplicable
		}
		return publicInitReloadReceipt{Posture: posture}
	}
	hosts := sortedPublicInitReceiptValues(hostSet)
	reasons := sortedPublicInitReceiptValues(reasonSet)
	return publicInitReloadReceipt{
		Posture: publicInitReloadRequired,
		Hosts:   hosts,
		Reasons: reasons,
	}
}

func publicInitPathRequiresReload(
	step initplanning.HostPublicationStepKind,
) bool {
	return step == initplanning.PublicationCreate ||
		step == initplanning.PublicationReplace ||
		step == initplanning.PublicationRemove
}

func publicInitReloadReason(
	component initplanning.Component,
) string {
	switch component {
	case initplanning.ComponentMCP:
		return "mcp_changed"
	case initplanning.ComponentSkills:
		return "skills_changed"
	case initplanning.ComponentInstructions:
		return "instructions_changed"
	case initplanning.ComponentPackage:
		return "package_changed"
	default:
		return string(component) + "_changed"
	}
}

func sortedPublicInitReceiptValues(
	values map[string]struct{},
) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func renderPublicInitReloadReceipt(
	output io.Writer,
	receipt publicInitReloadReceipt,
) error {
	if receipt.Posture != publicInitReloadRequired {
		return nil
	}
	if _, err := fmt.Fprintln(output, "  Reload: required"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(
		output,
		"    Hosts: %s\n",
		strings.Join(receipt.Hosts, ", "),
	); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(
		output,
		"    Changed: %s\n",
		strings.Join(receipt.Reasons, ", "),
	); err != nil {
		return err
	}
	_, err := fmt.Fprintln(
		output,
		"    Action: start a new host session or restart the current one.",
	)
	return err
}

func writePublicInitSuccessLine(
	output io.Writer,
	summary string,
) error {
	_, err := fmt.Fprintf(
		output,
		"  ✓ %s\n",
		summary,
	)
	return err
}

func publicInitCoreSummary(
	receipt initexecution.CoreEffectReceipt,
) string {
	switch receipt.Effect() {
	case initplanning.CoreInitialize:
		return fmt.Sprintf(
			"Project core initialized (schema %d)",
			receipt.AfterSchema(),
		)
	case initplanning.CoreMigrate:
		return fmt.Sprintf(
			"Project core migrated (schema %d → %d)",
			receipt.BeforeSchema(),
			receipt.AfterSchema(),
		)
	default:
		return fmt.Sprintf(
			"Project core already current (schema %d)",
			receipt.AfterSchema(),
		)
	}
}

func publicInitHostSummary(
	receipt initexecution.HostExecutionReceipt,
	projectRoot string,
) string {
	host := publicInitHostLabel(receipt.Host())
	if receipt.Scope() == initplanning.ScopeUser {
		host += " (user)"
	}
	publication := receipt.Outcome()
	paths := publication.Receipts()
	changed := publicInitChangedHostPaths(paths)
	preserved := publicInitPreservedHostPaths(paths)
	components := publicInitHostComponents(paths)
	if publication.Kind() == initfs.HostPublicationAlreadyCurrent {
		current := publicInitCurrentComponentSummary(components)
		return fmt.Sprintf("%s: %s already current", host, current)
	}
	if len(changed) == 0 {
		current := publicInitCurrentComponentSummary(components)
		if len(paths) == 1 {
			path := publicInitDisplayPath(
				projectRoot,
				paths[0].Path(),
			)
			return fmt.Sprintf(
				"%s: registered existing %s (%s)",
				host,
				current,
				path,
			)
		}
		return fmt.Sprintf("%s: registered existing %s", host, current)
	}
	if len(changed) == 1 {
		summary := publicInitSingleHostChange(
			host,
			changed[0],
			projectRoot,
		)
		return publicInitAppendPreservedHostSummary(
			summary,
			preserved,
			publication.ExpectedManifestDigest() == "",
			projectRoot,
		)
	}
	actions := publicInitHostActionSummary(changed)
	componentList := publicInitComponentList(
		publicInitHostComponents(changed),
	)
	summary := fmt.Sprintf(
		"%s: %s (%s)",
		host,
		actions,
		componentList,
	)
	return publicInitAppendPreservedHostSummary(
		summary,
		preserved,
		publication.ExpectedManifestDigest() == "",
		projectRoot,
	)
}

func publicInitChangedHostPaths(
	receipts []initfs.HostPathReceipt,
) []initfs.HostPathReceipt {
	changed := make(
		[]initfs.HostPathReceipt,
		0,
		len(receipts),
	)
	for _, receipt := range receipts {
		if receipt.Step() == initplanning.PublicationPreserve {
			continue
		}
		changed = append(changed, receipt)
	}
	return changed
}

func publicInitPreservedHostPaths(
	receipts []initfs.HostPathReceipt,
) []initfs.HostPathReceipt {
	preserved := make(
		[]initfs.HostPathReceipt,
		0,
		len(receipts),
	)
	for _, receipt := range receipts {
		if receipt.Step() != initplanning.PublicationPreserve {
			continue
		}
		preserved = append(preserved, receipt)
	}
	return preserved
}

func publicInitAppendPreservedHostSummary(
	summary string,
	preserved []initfs.HostPathReceipt,
	registered bool,
	projectRoot string,
) string {
	if len(preserved) == 0 {
		return summary
	}
	action := "verified existing"
	if registered {
		action = "registered existing"
	}
	components := publicInitCurrentComponentSummary(
		publicInitHostComponents(preserved),
	)
	if len(preserved) == 1 {
		path := publicInitDisplayPath(
			projectRoot,
			preserved[0].Path(),
		)
		return fmt.Sprintf(
			"%s; %s %s (%s)",
			summary,
			action,
			components,
			path,
		)
	}
	return fmt.Sprintf(
		"%s; %s %s",
		summary,
		action,
		components,
	)
}

func publicInitSingleHostChange(
	host string,
	receipt initfs.HostPathReceipt,
	projectRoot string,
) string {
	action := publicInitHostAction(receipt.Step())
	components := publicInitHostComponents(
		[]initfs.HostPathReceipt{receipt},
	)
	object := publicInitCurrentComponentSummary(components)
	path := publicInitDisplayPath(
		projectRoot,
		receipt.Path(),
	)
	return fmt.Sprintf(
		"%s: %s %s (%s)",
		host,
		action,
		object,
		path,
	)
}

func publicInitHostAction(
	step initplanning.HostPublicationStepKind,
) string {
	switch step {
	case initplanning.PublicationCreate:
		return "created"
	case initplanning.PublicationReplace:
		return "updated"
	case initplanning.PublicationAdoptLegacy:
		return "adopted existing"
	case initplanning.PublicationRemove:
		return "removed"
	default:
		return "verified"
	}
}

func publicInitHostActionSummary(
	receipts []initfs.HostPathReceipt,
) string {
	steps := []initplanning.HostPublicationStepKind{
		initplanning.PublicationAdoptLegacy,
		initplanning.PublicationCreate,
		initplanning.PublicationReplace,
		initplanning.PublicationRemove,
	}
	parts := make([]string, 0, len(steps))
	for _, step := range steps {
		count := 0
		for _, receipt := range receipts {
			if receipt.Step() == step {
				count++
			}
		}
		if count == 0 {
			continue
		}
		parts = append(
			parts,
			publicInitHostActionCount(step, count),
		)
	}
	return publicInitJoin(parts)
}

func publicInitHostActionCount(
	step initplanning.HostPublicationStepKind,
	count int,
) string {
	noun := "files"
	if count == 1 {
		noun = "file"
	}
	switch step {
	case initplanning.PublicationAdoptLegacy:
		return fmt.Sprintf(
			"adopted %d existing %s",
			count,
			noun,
		)
	case initplanning.PublicationCreate:
		return fmt.Sprintf("created %d %s", count, noun)
	case initplanning.PublicationReplace:
		return fmt.Sprintf("updated %d %s", count, noun)
	case initplanning.PublicationRemove:
		return fmt.Sprintf("removed %d %s", count, noun)
	default:
		return fmt.Sprintf("verified %d %s", count, noun)
	}
}

func publicInitHostComponents(
	receipts []initfs.HostPathReceipt,
) []initplanning.Component {
	ordered := []initplanning.Component{
		initplanning.ComponentMCP,
		initplanning.ComponentSkills,
		initplanning.ComponentInstructions,
		initplanning.ComponentHooks,
		initplanning.ComponentPackage,
	}
	present := map[initplanning.Component]bool{}
	for _, receipt := range receipts {
		for _, component := range receipt.Components().Values() {
			present[component] = true
		}
	}
	components := make(
		[]initplanning.Component,
		0,
		len(present),
	)
	for _, component := range ordered {
		if !present[component] {
			continue
		}
		components = append(components, component)
	}
	return components
}

func publicInitCurrentComponentSummary(
	components []initplanning.Component,
) string {
	if len(components) == 1 &&
		components[0] == initplanning.ComponentMCP {
		return "MCP configuration"
	}
	return publicInitComponentList(components)
}

func publicInitComponentList(
	components []initplanning.Component,
) string {
	labels := make([]string, 0, len(components))
	for _, component := range components {
		labels = append(
			labels,
			publicInitComponentLabel(component),
		)
	}
	if len(labels) == 0 {
		return "configuration"
	}
	return publicInitJoin(labels)
}

func publicInitComponentLabel(
	component initplanning.Component,
) string {
	switch component {
	case initplanning.ComponentMCP:
		return "MCP"
	case initplanning.ComponentSkills:
		return "skills"
	case initplanning.ComponentInstructions:
		return "instructions"
	case initplanning.ComponentHooks:
		return "hooks"
	case initplanning.ComponentPackage:
		return "package"
	default:
		return string(component)
	}
}

func publicInitJoin(values []string) string {
	switch len(values) {
	case 0:
		return ""
	case 1:
		return values[0]
	case 2:
		return values[0] + " and " + values[1]
	default:
		head := strings.Join(values[:len(values)-1], ", ")
		return head + ", and " + values[len(values)-1]
	}
}

func publicInitDisplayPath(
	projectRoot string,
	path string,
) string {
	if projectRoot == "" {
		return path
	}
	relative, err := filepath.Rel(projectRoot, path)
	if err != nil ||
		relative == ".." ||
		strings.HasPrefix(
			relative,
			".."+string(filepath.Separator),
		) {
		return path
	}
	return filepath.ToSlash(relative)
}

func publicInitHostLabel(
	host initplanning.HostID,
) string {
	switch host {
	case initplanning.HostClaude:
		return "Claude Code"
	case initplanning.HostCodex:
		return "Codex"
	case initplanning.HostCursor:
		return "Cursor"
	case initplanning.HostGemini:
		return "Gemini CLI"
	case initplanning.HostAntigravity:
		return "Antigravity"
	case initplanning.HostGrok:
		return "Grok"
	case initplanning.HostHermes:
		return "Hermes"
	case initplanning.HostZed:
		return "Zed"
	case initplanning.HostOpenCode:
		return "OpenCode"
	case initplanning.HostAir:
		return "Air"
	case initplanning.HostPi:
		return "Pi"
	default:
		return string(host)
	}
}

func publicInitAgentSkillsSummary(
	receipt publicAgentSkillsReceipt,
) string {
	if receipt.ChangedPaths() == 0 {
		return "Agent skills already current"
	}
	return fmt.Sprintf(
		"Agent skills: %d files installed or updated",
		receipt.ChangedPaths(),
	)
}

func publicInitExactFilesSummary(
	label string,
	receipt publicExactFileReceipt,
	kind publicInitOutcomeKind,
) string {
	count := len(receipt.Completed())
	if kind == publicInitAlreadyCurrent {
		return fmt.Sprintf("%s already current", label)
	}
	if count == 1 {
		return fmt.Sprintf("%s configured (1 file)", label)
	}
	return fmt.Sprintf("%s configured (%d files)", label, count)
}

func publicInitDeprecatedSkillCleanupSummary(
	receipt publicExactFileReceipt,
) string {
	count := len(receipt.Completed())
	if count == 1 {
		return "Removed 1 deprecated skill tree"
	}
	return fmt.Sprintf(
		"Removed %d deprecated skill trees",
		count,
	)
}

func publicInitLegacyCommandCleanupSummary(
	receipt publicExactFileReceipt,
) string {
	count := len(receipt.Completed())
	if count == 1 {
		return "Removed 1 legacy command file"
	}
	return fmt.Sprintf(
		"Removed %d legacy command files",
		count,
	)
}

func renderTypedPublicPartialReceipts(
	output io.Writer,
	outcome typedPublicInitOutcome,
) error {
	receipts := []publicExactFileReceipt{}
	if receipt, ok := outcome.CoreEffects(); ok {
		receipts = append(receipts, receipt)
	}
	if receipt, ok := outcome.Hermes(); ok {
		receipts = append(receipts, receipt)
	}
	if receipt, ok := outcome.Overseer(); ok {
		receipts = append(receipts, receipt)
	}
	if receipt, ok := outcome.LegacyCleanup(); ok {
		receipts = append(receipts, receipt)
	}
	for _, receipt := range receipts {
		if receipt.Failed() != "" {
			if _, err := fmt.Fprintf(
				output,
				"  failed: %s\n",
				receipt.Failed(),
			); err != nil {
				return err
			}
		}
		if len(receipt.Retry()) > 0 {
			if _, err := fmt.Fprintf(
				output,
				"  retry paths: %s\n",
				strings.Join(receipt.Retry(), ", "),
			); err != nil {
				return err
			}
		}
		if len(receipt.Recovery()) > 0 {
			if _, err := fmt.Fprintf(
				output,
				"  recovery argv: %s\n",
				strings.Join(receipt.Recovery(), " "),
			); err != nil {
				return err
			}
		}
	}
	if receipt, ok := outcome.AgentSkills(); ok {
		if receipt.Failed() != "" {
			if _, err := fmt.Fprintf(
				output,
				"  failed: %s\n",
				receipt.Failed(),
			); err != nil {
				return err
			}
		}
		if len(receipt.Retry()) > 0 {
			if _, err := fmt.Fprintf(
				output,
				"  retry paths: %s\n",
				strings.Join(receipt.Retry(), ", "),
			); err != nil {
				return err
			}
		}
		if len(receipt.Recovery()) > 0 {
			if _, err := fmt.Fprintf(
				output,
				"  recovery argv: %s\n",
				strings.Join(receipt.Recovery(), " "),
			); err != nil {
				return err
			}
		}
	}
	return nil
}
