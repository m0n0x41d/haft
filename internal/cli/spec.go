package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/project"
)

var (
	specCheckJSON         bool
	specCoverageJSON      bool
	specPlanJSON          bool
	specPlanAcceptID      string
	specOnboardJSON       bool
	specOnboardApproveID  string
	specOnboardReopenID   string
	specOnboardRebaseline string
	specOnboardReason     string
	specOnboardApprovedBy string
	specCheckExit         = os.Exit
	specCoverageExit      = os.Exit
)

var specCmd = &cobra.Command{
	Use:   "spec",
	Short: "Inspect project specification carriers",
}

var specCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Run L0/L1/L1.5 structural checks for spec carriers",
	Long: `Run deterministic L0/L1/L1.5 checks for project specification carriers.

The check parses fenced YAML spec-section blocks, validates required structural
fields and L1.5 carrier shapes, and verifies that the term-map carrier has
parseable term entries. It does not perform L2 semantic validation or L3
runtime/evidence validation.`,
	RunE: runSpecCheck,
}

var specCoverageCmd = &cobra.Command{
	Use:   "coverage",
	Short: "Show derived spec coverage by section",
	Long: `Show derived SpecCoverage for active spec sections.

Coverage is computed from spec-section refs, artifact links, WorkCommissions,
affected files, and attached evidence. It does not read or store manual
coverage status fields, and it does not report coverage percentages as the
primary truth.`,
	RunE: runSpecCoverage,
}

var specOnboardCmd = &cobra.Command{
	Use:   "onboard",
	Short: "Drive the spec onboarding method one step at a time",
	Long: `Return the next typed onboarding action for the current project.

The command derives state from .haft/specs/* carriers, runs the canonical
SpecOnboardingMethod phase registry from internal/project/specflow, and
prints a WorkflowIntent: which phase is next, what the human should
decide, what context the host agent needs, and which structural Checks
the resulting section must satisfy.

The command does not write spec carriers or DB rows; surfaces (Claude
Code via MCP plugin, Desktop wizard, this CLI) all read the same intent
and dispatch their own UX.`,
	RunE: runSpecOnboard,
}

var specPlanCmd = &cobra.Command{
	Use:   "plan",
	Short: "Show DecisionRecord draft proposals for uncovered or stale spec sections",
	Long: `Show SpecPlan proposals for active spec sections whose coverage is uncovered or stale.

The command groups sections by document kind, spec kind, dependency signature,
and affected area. Listing output is a human-review draft surface only:
proposals are not authority. No DecisionRecords are created by listing, and
no WorkCommissions are created or scheduled.

Use --accept <proposal-id> to create one DecisionRecord from a reviewed
proposal. Merge, split, and discard are typed non-executable actions in this
slice and are reported with command gaps.`,
	RunE: runSpecPlan,
}

func init() {
	specCheckCmd.Flags().BoolVar(&specCheckJSON, "json", false, "print structured JSON output")
	specCoverageCmd.Flags().BoolVar(&specCoverageJSON, "json", false, "print structured JSON output")
	specPlanCmd.Flags().BoolVar(&specPlanJSON, "json", false, "print structured JSON output")
	specPlanCmd.Flags().StringVar(&specPlanAcceptID, "accept", "", "accept proposal id and create one DecisionRecord")
	specOnboardCmd.Flags().BoolVar(&specOnboardJSON, "json", false, "print structured JSON output")
	specOnboardCmd.Flags().StringVar(&specOnboardApproveID, "approve", "", "record a SpecSectionBaseline for the given active section id")
	specOnboardCmd.Flags().StringVar(&specOnboardRebaseline, "rebaseline", "", "overwrite an existing SpecSectionBaseline for the given section id (requires --reason)")
	specOnboardCmd.Flags().StringVar(&specOnboardReopenID, "reopen", "", "delete the SpecSectionBaseline for the given section id so it re-enters the onboarding loop")
	specOnboardCmd.Flags().StringVar(&specOnboardReason, "reason", "", "audit-trail rationale recorded with --rebaseline / --reopen")
	specOnboardCmd.Flags().StringVar(&specOnboardApprovedBy, "approved-by", "", "identifier of who approved the baseline (default: human)")
	specCmd.AddCommand(specCheckCmd)
	specCmd.AddCommand(specCoverageCmd)
	specCmd.AddCommand(specPlanCmd)
	specCmd.AddCommand(specOnboardCmd)
	rootCmd.AddCommand(specCmd)
}

func runSpecCheck(cmd *cobra.Command, _ []string) error {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a haft project: %w", err)
	}

	report, err := project.CheckSpecificationSet(projectRoot)
	if err != nil {
		return err
	}

	report = appendSpecBaselineFindings(report, projectRoot)

	output := cmd.OutOrStdout()
	if specCheckJSON {
		err = writeSpecCheckJSON(output, report)
	} else {
		err = writeSpecCheckSummary(output, report)
	}
	if err != nil {
		return err
	}

	if report.HasFindings() {
		specCheckExit(1)
	}

	return nil
}

func runSpecCoverage(cmd *cobra.Command, _ []string) error {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a haft project: %w", err)
	}

	report, err := buildSpecCoverageReport(context.Background(), projectRoot)
	if err != nil {
		blocked := &specCoverageBlockedError{}
		if specCoverageJSON && errors.As(err, &blocked) {
			if writeErr := writeSpecCoverageBlockedJSON(cmd.OutOrStdout(), blocked.report); writeErr != nil {
				return writeErr
			}

			specCoverageExit(1)
			return nil
		}

		return err
	}

	output := cmd.OutOrStdout()
	if specCoverageJSON {
		return writeSpecCoverageJSON(output, report)
	}

	return writeSpecCoverageSummary(output, report)
}

func runSpecPlan(cmd *cobra.Command, _ []string) error {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a haft project: %w", err)
	}

	report, err := buildSpecPlanReport(context.Background(), projectRoot)
	if err != nil {
		return err
	}

	if strings.TrimSpace(specPlanAcceptID) != "" {
		result, err := acceptSpecPlanProposal(context.Background(), projectRoot, report, specPlanAcceptID)
		if err != nil {
			return err
		}

		output := cmd.OutOrStdout()
		if specPlanJSON {
			return writeSpecPlanAcceptJSON(output, result)
		}

		return writeSpecPlanAcceptSummary(output, result)
	}

	output := cmd.OutOrStdout()
	if specPlanJSON {
		return writeSpecPlanJSON(output, report)
	}

	return writeSpecPlanSummary(output, report)
}

func writeSpecCheckJSON(w io.Writer, report project.SpecCheckReport) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")

	return encoder.Encode(report)
}

func writeSpecCheckSummary(w io.Writer, report project.SpecCheckReport) error {
	builder := strings.Builder{}

	if report.HasFindings() {
		builder.WriteString(fmt.Sprintf("haft spec check: L0/L1/L1.5 findings found (%d finding(s))\n", report.Summary.TotalFindings))
	} else {
		builder.WriteString("haft spec check: clean (L0/L1/L1.5)\n")
	}

	builder.WriteString(fmt.Sprintf("spec_sections: %d\n", report.Summary.SpecSections))
	builder.WriteString(fmt.Sprintf("active_spec_sections: %d\n", report.Summary.ActiveSpecSections))
	builder.WriteString(fmt.Sprintf("term_map_entries: %d\n", report.Summary.TermMapEntries))

	if len(report.Findings) > 0 {
		builder.WriteString("\nFindings:\n")
	}

	for _, finding := range report.Findings {
		builder.WriteString(formatSpecCheckFinding(finding))
	}

	_, err := io.WriteString(w, builder.String())

	return err
}

func formatSpecCheckFinding(finding project.SpecCheckFinding) string {
	location := finding.Path
	if finding.Line > 0 {
		location = fmt.Sprintf("%s:%d", finding.Path, finding.Line)
	}

	section := ""
	if finding.SectionID != "" {
		section = " section=" + finding.SectionID
	}

	line := fmt.Sprintf("- [%s] %s %s%s - %s\n",
		finding.Level,
		finding.Code,
		location,
		section,
		finding.Message,
	)
	if finding.NextAction != "" {
		line += fmt.Sprintf("  next_action: %s\n", finding.NextAction)
	}

	return line
}

type specCoverageBlockedError struct {
	report project.SpecCheckReport
}

func (err *specCoverageBlockedError) Error() string {
	return fmt.Sprintf(
		"spec coverage blocked: spec check has %d finding(s); run `haft spec check` first",
		err.report.Summary.TotalFindings,
	)
}

type specCoverageBlockedJSONReport struct {
	Status     string                     `json:"status"`
	Reason     string                     `json:"reason"`
	NextAction string                     `json:"next_action"`
	SpecCheck  project.SpecCheckReport    `json:"spec_check"`
	Coverage   project.SpecCoverageReport `json:"coverage"`
}

func buildSpecCoverageReport(
	ctx context.Context,
	projectRoot string,
) (project.SpecCoverageReport, error) {
	specCheck, err := project.CheckSpecificationSet(projectRoot)
	if err != nil {
		return project.SpecCoverageReport{}, err
	}
	if specCheck.HasFindings() {
		return project.SpecCoverageReport{}, &specCoverageBlockedError{report: specCheck}
	}

	sections, err := project.LoadSpecSections(projectRoot)
	if err != nil {
		return project.SpecCoverageReport{}, err
	}

	store, closeStore, err := openSpecCoverageStore(projectRoot)
	if err != nil {
		return project.SpecCoverageReport{}, err
	}
	defer closeStore()

	input := project.SpecCoverageInput{
		Sections: sections,
	}
	if store == nil {
		return project.DeriveSpecCoverage(input), nil
	}

	sectionIDs := specCoverageSectionIDSet(sections)
	input.Problems, err = specCoverageProblems(ctx, store, sectionIDs)
	if err != nil {
		return project.SpecCoverageReport{}, err
	}
	input.Decisions, err = specCoverageDecisions(ctx, store, projectRoot, sectionIDs)
	if err != nil {
		return project.SpecCoverageReport{}, err
	}
	input.Commissions, err = specCoverageCommissions(ctx, store, sectionIDs)
	if err != nil {
		return project.SpecCoverageReport{}, err
	}
	input.RuntimeRuns, err = specCoverageRuntimeRuns(ctx, store, sectionIDs)
	if err != nil {
		return project.SpecCoverageReport{}, err
	}
	input.Evidence, err = specCoverageEvidence(
		ctx,
		store,
		input.Problems,
		input.Decisions,
		input.Commissions,
		input.RuntimeRuns,
		sectionIDs,
	)
	if err != nil {
		return project.SpecCoverageReport{}, err
	}

	return project.DeriveSpecCoverage(input), nil
}

func buildSpecPlanReport(
	ctx context.Context,
	projectRoot string,
) (project.SpecPlanReport, error) {
	coverage, err := buildSpecCoverageReport(ctx, projectRoot)
	if err != nil {
		return project.SpecPlanReport{}, err
	}

	return project.BuildSpecPlan(coverage), nil
}

type specPlanAcceptResult struct {
	Action      string   `json:"action"`
	ProposalID  string   `json:"proposal_id"`
	DecisionRef string   `json:"decision_ref"`
	DecisionMD  string   `json:"decision_md,omitempty"`
	SectionRefs []string `json:"section_refs"`
}

func acceptSpecPlanProposal(
	ctx context.Context,
	projectRoot string,
	report project.SpecPlanReport,
	proposalID string,
) (specPlanAcceptResult, error) {
	proposal, ok := project.FindSpecPlanProposal(report, proposalID)
	if !ok {
		return specPlanAcceptResult{}, fmt.Errorf("spec plan proposal %q not found; rerun `haft spec plan`", strings.TrimSpace(proposalID))
	}

	input, err := specPlanDecisionInput(proposal)
	if err != nil {
		return specPlanAcceptResult{}, err
	}

	store, closeStore, err := openSpecCoverageStore(projectRoot)
	if err != nil {
		return specPlanAcceptResult{}, err
	}
	defer closeStore()

	if store == nil {
		return specPlanAcceptResult{}, fmt.Errorf("spec plan accept requires an initialized Haft project database")
	}

	decision, decisionMD, err := artifact.Decide(ctx, store, filepath.Join(projectRoot, ".haft"), input)
	if err != nil {
		return specPlanAcceptResult{}, err
	}

	return specPlanAcceptResult{
		Action:      string(project.SpecPlanActionAccept),
		ProposalID:  proposal.ID,
		DecisionRef: decision.Meta.ID,
		DecisionMD:  decisionMD,
		SectionRefs: input.SectionRefs,
	}, nil
}

func specPlanDecisionInput(proposal project.SpecPlanProposal) (artifact.DecideInput, error) {
	proposal = normalizeCLISpecPlanProposal(proposal)
	draft := proposal.DecisionRecordDraft
	if len(draft.SectionRefs) == 0 {
		return artifact.DecideInput{}, fmt.Errorf("spec plan proposal %s has no section refs", proposal.ID)
	}

	rejections := make([]artifact.RejectionReason, 0, len(draft.WhyNotOthers))
	for _, rejection := range draft.WhyNotOthers {
		rejections = append(rejections, artifact.RejectionReason{
			Variant: rejection.Variant,
			Reason:  rejection.Reason,
		})
	}

	return artifact.DecideInput{
		SelectedTitle:   draft.SelectedTitle,
		WhySelected:     draft.WhySelected,
		SelectionPolicy: draft.SelectionPolicy,
		CounterArgument: draft.CounterArgument,
		WhyNotOthers:    rejections,
		WeakestLink:     draft.WeakestLink,
		Rollback: &artifact.RollbackSpec{
			Triggers: draft.RollbackTriggers,
		},
		EvidenceReqs:    draft.EvidenceRequirements,
		RefreshTriggers: draft.RefreshTriggers,
		SectionRefs:     draft.SectionRefs,
		TaskContext:     proposal.ID,
		SearchKeywords:  strings.Join(draft.SectionRefs, " "),
	}, nil
}

func normalizeCLISpecPlanProposal(proposal project.SpecPlanProposal) project.SpecPlanProposal {
	report := project.SpecPlanReport{
		Proposals: []project.SpecPlanProposal{proposal},
	}
	found, ok := project.FindSpecPlanProposal(report, proposal.ID)
	if ok {
		return found
	}

	return proposal
}

func openSpecCoverageStore(projectRoot string) (*artifact.Store, func(), error) {
	haftDir := filepath.Join(projectRoot, ".haft")
	cfg, err := project.Load(haftDir)
	if err != nil {
		return nil, func() {}, fmt.Errorf("load project config: %w", err)
	}
	if cfg == nil {
		return nil, func() {}, nil
	}

	dbPath, err := cfg.DBPath()
	if err != nil {
		return nil, func() {}, fmt.Errorf("get DB path: %w", err)
	}

	dsn := dbPath + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(3000)"
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, func() {}, fmt.Errorf("open DB: %w", err)
	}

	closeStore := func() {
		_ = sqlDB.Close()
	}

	return artifact.NewStore(sqlDB), closeStore, nil
}

func specCoverageProblems(
	ctx context.Context,
	store *artifact.Store,
	sectionIDs map[string]struct{},
) ([]project.SpecCoverageProblem, error) {
	items, err := loadSpecCoverageArtifacts(ctx, store, artifact.KindProblemCard)
	if err != nil {
		return nil, err
	}

	problems := make([]project.SpecCoverageProblem, 0, len(items))
	for _, item := range items {
		problems = append(problems, project.SpecCoverageProblem{
			ID:          item.Meta.ID,
			Title:       item.Meta.Title,
			Status:      string(item.Meta.Status),
			ValidUntil:  item.Meta.ValidUntil,
			SectionRefs: explicitSpecCoverageRefs(item, sectionIDs),
		})
	}

	return problems, nil
}

func specCoverageDecisions(
	ctx context.Context,
	store *artifact.Store,
	projectRoot string,
	sectionIDs map[string]struct{},
) ([]project.SpecCoverageDecision, error) {
	items, err := loadSpecCoverageArtifacts(ctx, store, artifact.KindDecisionRecord)
	if err != nil {
		return nil, err
	}

	drifted, err := specCoverageDriftedDecisionSet(ctx, store, projectRoot)
	if err != nil {
		return nil, err
	}

	decisions := make([]project.SpecCoverageDecision, 0, len(items))
	for _, item := range items {
		fields := item.UnmarshalDecisionFields()
		affectedFiles, err := store.GetAffectedFiles(ctx, item.Meta.ID)
		if err != nil {
			return nil, fmt.Errorf("load affected files for %s: %w", item.Meta.ID, err)
		}

		decisions = append(decisions, project.SpecCoverageDecision{
			ID:            item.Meta.ID,
			Title:         item.Meta.Title,
			Status:        string(item.Meta.Status),
			ValidUntil:    item.Meta.ValidUntil,
			ProblemRefs:   specCoverageProblemRefs(item, fields),
			SectionRefs:   explicitSpecCoverageRefs(item, sectionIDs),
			AffectedFiles: specCoverageAffectedFilePaths(affectedFiles),
			Drifted:       drifted[item.Meta.ID],
		})
	}

	return decisions, nil
}

func specCoverageCommissions(
	ctx context.Context,
	store *artifact.Store,
	sectionIDs map[string]struct{},
) ([]project.SpecCoverageCommission, error) {
	items, err := loadSpecCoverageArtifacts(ctx, store, artifact.KindWorkCommission)
	if err != nil {
		return nil, err
	}

	commissions := make([]project.SpecCoverageCommission, 0, len(items))
	for _, item := range items {
		payload, err := decodeWorkCommissionPayload(item.Meta.ID, item.StructuredData)
		if err != nil {
			return nil, err
		}

		validUntil := stringField(payload, "valid_until")
		if validUntil == "" {
			validUntil = item.Meta.ValidUntil
		}

		commissions = append(commissions, project.SpecCoverageCommission{
			ID:          item.Meta.ID,
			DecisionRef: stringField(payload, "decision_ref"),
			State:       stringField(payload, "state"),
			Status:      string(item.Meta.Status),
			ValidUntil:  validUntil,
			SectionRefs: specCoverageRefsFromMap(payload, sectionIDs),
		})
	}

	return commissions, nil
}

func specCoverageRuntimeRuns(
	ctx context.Context,
	store *artifact.Store,
	sectionIDs map[string]struct{},
) ([]project.SpecCoverageRuntimeRun, error) {
	items, err := loadSpecCoverageArtifacts(ctx, store, artifact.KindWorkCommission)
	if err != nil {
		return nil, err
	}

	runtimeRuns := make([]project.SpecCoverageRuntimeRun, 0)
	for _, item := range items {
		payload, err := decodeWorkCommissionPayload(item.Meta.ID, item.StructuredData)
		if err != nil {
			return nil, err
		}

		runtimeRuns = append(runtimeRuns, specCoverageRuntimeRunsFromCommission(payload, sectionIDs)...)
	}

	return runtimeRuns, nil
}

func specCoverageRuntimeRunsFromCommission(
	commission map[string]any,
	sectionIDs map[string]struct{},
) []project.SpecCoverageRuntimeRun {
	events := mapSliceField(commission, "events")
	runtimeRuns := make([]project.SpecCoverageRuntimeRun, 0, len(events))
	var runtimeRun *project.SpecCoverageRuntimeRun
	runtimeRunClosed := false
	runtimeRunOrdinal := 0

	for _, event := range events {
		if !specCoverageRuntimeRunEventCandidate(event) {
			continue
		}

		payload, _ := mapArg(event, "payload")
		eventRunID := specCoverageRuntimeRunExplicitID(event, payload)
		if specCoverageRuntimeRunNeedsNewAttempt(runtimeRun, runtimeRunClosed, eventRunID) {
			if runtimeRun != nil {
				runtimeRuns = append(runtimeRuns, *runtimeRun)
			}

			runtimeRunOrdinal++
			runtimeRun = specCoverageRuntimeRunStart(commission, eventRunID, runtimeRunOrdinal, sectionIDs)
			runtimeRunClosed = false
		}

		*runtimeRun = specCoverageRuntimeRunWithEvent(*runtimeRun, event, payload, sectionIDs)
		if specCoverageRuntimeRunEventTerminal(event) {
			runtimeRunClosed = true
		}
	}

	if runtimeRun != nil {
		runtimeRuns = append(runtimeRuns, *runtimeRun)
	}

	return runtimeRuns
}

func specCoverageRuntimeRunEventCandidate(event map[string]any) bool {
	switch stringField(event, "action") {
	case "record_run_event", "record_preflight", "start_after_preflight", "complete_or_block":
		return true
	case "":
		return stringField(event, "event") == "phase_outcome"
	default:
		return false
	}
}

func specCoverageRuntimeRunNeedsNewAttempt(
	runtimeRun *project.SpecCoverageRuntimeRun,
	runtimeRunClosed bool,
	eventRunID string,
) bool {
	if runtimeRun == nil {
		return true
	}
	if runtimeRunClosed {
		return true
	}
	if eventRunID == "" {
		return false
	}

	return runtimeRun.ID != eventRunID
}

func specCoverageRuntimeRunStart(
	commission map[string]any,
	eventRunID string,
	ordinal int,
	sectionIDs map[string]struct{},
) *project.SpecCoverageRuntimeRun {
	runtimeRunID := eventRunID
	if runtimeRunID == "" {
		runtimeRunID = specCoverageRuntimeRunOrdinalID(commission, ordinal)
	}

	return &project.SpecCoverageRuntimeRun{
		ID:             runtimeRunID,
		CommissionRef:  stringField(commission, "id"),
		ValidUntil:     stringField(commission, "valid_until"),
		SectionRefs:    specCoverageRuntimeRunSectionRefs(commission, nil, nil, sectionIDs),
		EvidenceStatus: project.RuntimeEvidenceMissing,
	}
}

func specCoverageRuntimeRunWithEvent(
	runtimeRun project.SpecCoverageRuntimeRun,
	event map[string]any,
	payload map[string]any,
	sectionIDs map[string]struct{},
) project.SpecCoverageRuntimeRun {
	outcome := specCoverageRuntimePhaseOutcome(event, payload)

	if runtimeRun.RunnerID == "" {
		runtimeRun.RunnerID = stringField(event, "runner_id")
	}
	if outcome.Event != "" {
		runtimeRun.Event = outcome.Event
	}
	if outcome.Verdict != "" {
		runtimeRun.Verdict = outcome.Verdict
	}
	if outcome.Phase != "" {
		runtimeRun.Phase = outcome.Phase
	}
	if outcome.Reason != "" {
		runtimeRun.Reason = outcome.Reason
	}
	if outcome.RecordedAt != "" {
		runtimeRun.RecordedAt = outcome.RecordedAt
	}
	if runtimeRun.StartedAt == "" {
		runtimeRun.StartedAt = outcome.RecordedAt
	}
	if specCoverageRuntimeRunEventTerminal(event) {
		runtimeRun.CompletedAt = outcome.RecordedAt
	}
	if validUntil := firstStringField("valid_until", event, payload); validUntil != "" {
		runtimeRun.ValidUntil = validUntil
	}

	runtimeRun.SectionRefs = append(
		runtimeRun.SectionRefs,
		specCoverageRuntimeRunSectionRefs(nil, event, payload, sectionIDs)...,
	)
	runtimeRun.PhaseOutcomes = append(runtimeRun.PhaseOutcomes, outcome)
	if reason := specCoverageRuntimeRunUnsupportedReason(event); reason != "" {
		runtimeRun.UnsupportedReason = reason
	}

	return runtimeRun
}

func specCoverageRuntimeRunExplicitID(
	event map[string]any,
	payload map[string]any,
) string {
	if id := firstStringField("runtime_run_id", event, payload); id != "" {
		return id
	}
	if id := firstStringField("run_id", event, payload); id != "" {
		return id
	}
	return firstStringField("carrier_ref", event, payload)
}

func specCoverageRuntimeRunOrdinalID(
	commission map[string]any,
	ordinal int,
) string {
	return fmt.Sprintf("%s#runtime-run-%03d", stringField(commission, "id"), ordinal)
}

func specCoverageRuntimePhaseOutcome(
	event map[string]any,
	payload map[string]any,
) project.SpecCoverageRuntimePhaseOutcome {
	return project.SpecCoverageRuntimePhaseOutcome{
		Action:     stringField(event, "action"),
		Phase:      specCoverageRuntimeRunPhase(event, payload),
		Event:      stringField(event, "event"),
		Verdict:    stringField(event, "verdict"),
		Reason:     stringField(event, "reason"),
		RecordedAt: stringField(event, "recorded_at"),
	}
}

func specCoverageRuntimeRunPhase(
	event map[string]any,
	payload map[string]any,
) string {
	if phase := firstStringField("phase", event, payload); phase != "" {
		return phase
	}

	phaseByEvent := map[string]string{
		"preflight_checked": "preflight",
		"preflight_passed":  "preflight",
		"workflow_terminal": "terminal",
		"phase_blocked":     "terminal",
		"freshness_blocked": "terminal",
	}
	if phase := phaseByEvent[stringField(event, "event")]; phase != "" {
		return phase
	}
	if stringField(event, "action") == "complete_or_block" {
		return "terminal"
	}

	return ""
}

func specCoverageRuntimeRunEventTerminal(event map[string]any) bool {
	if stringField(event, "action") == "complete_or_block" {
		return true
	}
	if stringField(event, "event") == "freshness_blocked" {
		return true
	}
	if stringField(event, "action") == "record_preflight" && stringField(event, "verdict") == "blocked" {
		return true
	}

	return false
}

func specCoverageRuntimeRunSectionRefs(
	commission map[string]any,
	event map[string]any,
	payload map[string]any,
	sectionIDs map[string]struct{},
) []string {
	refs := make([]string, 0)
	refs = append(refs, specCoverageRefsFromMap(commission, sectionIDs)...)
	refs = append(refs, specCoverageRefsFromMap(event, sectionIDs)...)
	refs = append(refs, specCoverageRefsFromMap(payload, sectionIDs)...)

	return cleanStringSlice(refs)
}

func specCoverageRuntimeRunUnsupportedReason(event map[string]any) string {
	if stringField(event, "event") == "" {
		return "RuntimeRun lifecycle event is missing the event field"
	}
	if stringField(event, "verdict") == "" {
		return "RuntimeRun lifecycle event is missing the verdict field"
	}

	return ""
}

func specCoverageEvidence(
	ctx context.Context,
	store *artifact.Store,
	problems []project.SpecCoverageProblem,
	decisions []project.SpecCoverageDecision,
	commissions []project.SpecCoverageCommission,
	runtimeRuns []project.SpecCoverageRuntimeRun,
	sectionIDs map[string]struct{},
) ([]project.SpecCoverageEvidence, error) {
	artifactRefs := specCoverageEvidenceArtifactRefs(problems, decisions, commissions, runtimeRuns)
	evidence := make([]project.SpecCoverageEvidence, 0)

	for _, artifactRef := range artifactRefs {
		items, err := store.GetEvidenceItems(ctx, artifactRef)
		if err != nil {
			return nil, fmt.Errorf("load evidence for %s: %w", artifactRef, err)
		}

		for _, item := range items {
			evidence = append(evidence, project.SpecCoverageEvidence{
				ID:          item.ID,
				ArtifactRef: artifactRef,
				Type:        item.Type,
				Verdict:     item.Verdict,
				CarrierRef:  item.CarrierRef,
				ValidUntil:  item.ValidUntil,
				SectionRefs: specCoverageRefsFromEvidence(item, sectionIDs),
				CodeRefs:    specCoverageCodeRefsFromEvidence(item, sectionIDs),
				TestRefs:    specCoverageTestRefsFromEvidence(item, sectionIDs),
			})
		}
	}

	return evidence, nil
}

func loadSpecCoverageArtifacts(
	ctx context.Context,
	store *artifact.Store,
	kind artifact.Kind,
) ([]*artifact.Artifact, error) {
	items, err := store.ListByKind(ctx, kind, 0)
	if err != nil {
		return nil, fmt.Errorf("list %s artifacts: %w", kind, err)
	}

	loaded := make([]*artifact.Artifact, 0, len(items))
	for _, item := range items {
		fullItem, err := store.Get(ctx, item.Meta.ID)
		if err != nil {
			return nil, fmt.Errorf("load artifact %s: %w", item.Meta.ID, err)
		}

		loaded = append(loaded, fullItem)
	}

	return loaded, nil
}

func specCoverageDriftedDecisionSet(
	ctx context.Context,
	store *artifact.Store,
	projectRoot string,
) (map[string]bool, error) {
	reports, err := artifact.CheckDrift(ctx, store, projectRoot)
	if err != nil {
		return nil, fmt.Errorf("scan drift for spec coverage: %w", err)
	}

	drifted := map[string]bool{}
	for _, report := range reports {
		if !report.HasBaseline {
			continue
		}
		if len(report.Files) == 0 {
			continue
		}

		drifted[report.DecisionID] = true
	}

	return drifted, nil
}

func specCoverageProblemRefs(
	item *artifact.Artifact,
	fields artifact.DecisionFields,
) []string {
	refs := append([]string(nil), fields.ProblemRefs...)

	for _, link := range item.Meta.Links {
		if !strings.HasPrefix(link.Ref, artifact.KindProblemCard.IDPrefix()+"-") {
			continue
		}

		refs = append(refs, link.Ref)
	}

	return cleanStringSlice(refs)
}

func specCoverageAffectedFilePaths(files []artifact.AffectedFile) []string {
	paths := make([]string, 0, len(files))

	for _, file := range files {
		paths = append(paths, filepath.ToSlash(file.Path))
	}

	return cleanStringSlice(paths)
}

func specCoverageEvidenceArtifactRefs(
	problems []project.SpecCoverageProblem,
	decisions []project.SpecCoverageDecision,
	commissions []project.SpecCoverageCommission,
	runtimeRuns []project.SpecCoverageRuntimeRun,
) []string {
	refs := make([]string, 0, len(problems)+len(decisions)+len(commissions)+len(runtimeRuns))

	for _, problem := range problems {
		refs = append(refs, problem.ID)
	}
	for _, decision := range decisions {
		refs = append(refs, decision.ID)
	}
	for _, commission := range commissions {
		refs = append(refs, commission.ID)
	}
	for _, runtimeRun := range runtimeRuns {
		refs = append(refs, runtimeRun.ID)
	}

	return cleanStringSlice(refs)
}

func specCoverageSectionIDSet(sections []project.SpecSection) map[string]struct{} {
	ids := make(map[string]struct{}, len(sections))

	for _, section := range sections {
		ids[section.ID] = struct{}{}
	}

	return ids
}

func explicitSpecCoverageRefs(
	item *artifact.Artifact,
	sectionIDs map[string]struct{},
) []string {
	refs := make([]string, 0)

	for _, link := range item.Meta.Links {
		if _, ok := sectionIDs[link.Ref]; !ok {
			continue
		}

		refs = append(refs, link.Ref)
	}

	refs = append(refs, specCoverageRefsFromStructuredData(item.StructuredData, sectionIDs)...)

	return cleanStringSlice(refs)
}

func specCoverageRefsFromStructuredData(
	data string,
	sectionIDs map[string]struct{},
) []string {
	trimmed := strings.TrimSpace(data)
	if trimmed == "" {
		return nil
	}

	var payload any
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return nil
	}

	return specCoverageRefsFromValue(payload, sectionIDs)
}

func specCoverageRefsFromMap(
	payload map[string]any,
	sectionIDs map[string]struct{},
) []string {
	return specCoverageRefsFromValue(payload, sectionIDs)
}

func specCoverageRefsFromValue(
	value any,
	sectionIDs map[string]struct{},
) []string {
	switch typed := value.(type) {
	case map[string]any:
		return specCoverageRefsFromObject(typed, sectionIDs)
	case []any:
		return specCoverageRefsFromNestedList(typed, sectionIDs)
	default:
		return nil
	}
}

func specCoverageRefsFromObject(
	payload map[string]any,
	sectionIDs map[string]struct{},
) []string {
	refs := make([]string, 0)

	for key, value := range payload {
		if specCoverageRefKey(key) {
			refs = append(refs, specCoverageRefsFromExplicitValue(value, sectionIDs)...)
			continue
		}

		refs = append(refs, specCoverageRefsFromValue(value, sectionIDs)...)
	}

	return cleanStringSlice(refs)
}

func specCoverageRefsFromNestedList(
	values []any,
	sectionIDs map[string]struct{},
) []string {
	refs := make([]string, 0)

	for _, value := range values {
		refs = append(refs, specCoverageRefsFromValue(value, sectionIDs)...)
	}

	return cleanStringSlice(refs)
}

func specCoverageRefsFromExplicitValue(
	value any,
	sectionIDs map[string]struct{},
) []string {
	switch typed := value.(type) {
	case string:
		return specCoverageRefsFromStrings([]string{typed}, sectionIDs)
	case []string:
		return specCoverageRefsFromStrings(typed, sectionIDs)
	case []any:
		return specCoverageRefsFromExplicitList(typed, sectionIDs)
	default:
		return nil
	}
}

func specCoverageRefsFromExplicitList(
	values []any,
	sectionIDs map[string]struct{},
) []string {
	refs := make([]string, 0, len(values))

	for _, value := range values {
		text, ok := value.(string)
		if !ok {
			continue
		}

		refs = append(refs, text)
	}

	return specCoverageRefsFromStrings(refs, sectionIDs)
}

func specCoverageRefsFromEvidence(
	item artifact.EvidenceItem,
	sectionIDs map[string]struct{},
) []string {
	refs := make([]string, 0)
	refs = append(refs, specCoverageRefsFromStrings(item.ClaimRefs, sectionIDs)...)
	refs = append(refs, specCoverageRefsFromStrings(item.ClaimScope, sectionIDs)...)
	refs = append(refs, specCoverageRefsFromStrings([]string{item.CarrierRef}, sectionIDs)...)

	return cleanStringSlice(refs)
}

func specCoverageRefsFromStrings(
	values []string,
	sectionIDs map[string]struct{},
) []string {
	refs := make([]string, 0, len(values))

	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if _, ok := sectionIDs[trimmed]; !ok {
			continue
		}

		refs = append(refs, trimmed)
	}

	return cleanStringSlice(refs)
}

func specCoverageCodeRefsFromEvidence(
	item artifact.EvidenceItem,
	sectionIDs map[string]struct{},
) []string {
	return specCoveragePathRefsFromEvidence(item, sectionIDs, specCoverageCodePath)
}

func specCoverageTestRefsFromEvidence(
	item artifact.EvidenceItem,
	sectionIDs map[string]struct{},
) []string {
	return specCoveragePathRefsFromEvidence(item, sectionIDs, specCoverageTestPath)
}

func specCoveragePathRefsFromEvidence(
	item artifact.EvidenceItem,
	sectionIDs map[string]struct{},
	predicate func(string) bool,
) []string {
	candidates := make([]string, 0, len(item.ClaimScope)+2)
	candidates = append(candidates, item.CarrierRef)
	candidates = append(candidates, item.ClaimScope...)

	refs := make([]string, 0)
	for _, candidate := range candidates {
		trimmed := strings.TrimSpace(candidate)
		if _, ok := sectionIDs[trimmed]; ok {
			continue
		}
		if !predicate(trimmed) {
			continue
		}

		refs = append(refs, filepath.ToSlash(trimmed))
	}

	return cleanStringSlice(refs)
}

func specCoverageCodePath(value string) bool {
	if value == "" {
		return false
	}
	if specCoverageTestPath(value) {
		return false
	}

	switch strings.ToLower(filepath.Ext(value)) {
	case ".go", ".rs", ".ts", ".tsx", ".js", ".jsx", ".py", ".java", ".kt", ".rb", ".php", ".c", ".cc", ".cpp", ".h", ".hpp":
		return true
	default:
		return false
	}
}

func specCoverageTestPath(value string) bool {
	normalized := strings.ToLower(filepath.ToSlash(value))
	if normalized == "" {
		return false
	}
	if strings.Contains(normalized, "/test/") || strings.Contains(normalized, "/tests/") {
		return true
	}
	if strings.Contains(normalized, "_test.") {
		return true
	}
	if strings.Contains(normalized, ".test.") || strings.Contains(normalized, ".spec.") {
		return true
	}

	return false
}

func specCoverageRefKey(key string) bool {
	switch strings.TrimSpace(key) {
	case "spec_ref", "spec_refs", "spec_section_ref", "spec_section_refs", "section_ref", "section_refs", "target_ref", "target_refs":
		return true
	default:
		return false
	}
}

func writeSpecCoverageJSON(w io.Writer, report project.SpecCoverageReport) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")

	return encoder.Encode(report)
}

func writeSpecCoverageBlockedJSON(w io.Writer, specCheck project.SpecCheckReport) error {
	report := specCoverageBlockedJSONReport{
		Status:     "blocked",
		Reason:     fmt.Sprintf("spec check has %d finding(s)", specCheck.Summary.TotalFindings),
		NextAction: "resolve spec_check.findings, then rerun `haft spec coverage --json`",
		SpecCheck:  specCheck,
		Coverage: project.SpecCoverageReport{
			Sections: []project.SpecCoverageSection{},
			Gaps: []project.SpecCoverageGap{{
				Kind:       "spec_check_blocked",
				Detail:     "spec coverage is derived only after deterministic spec check passes",
				NextAction: "resolve spec_check.findings, then rerun `haft spec coverage --json`",
			}},
			Summary: project.SpecCoverageSummary{
				TotalSections: 0,
				StateCounts:   map[string]int{},
			},
		},
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")

	return encoder.Encode(report)
}

func writeSpecCoverageSummary(w io.Writer, report project.SpecCoverageReport) error {
	builder := strings.Builder{}
	builder.WriteString(fmt.Sprintf("haft spec coverage: %d active section(s)\n", report.Summary.TotalSections))
	builder.WriteString("states:\n")

	for _, state := range specCoverageStateOrder() {
		count := report.Summary.StateCounts[string(state)]
		builder.WriteString(fmt.Sprintf("  %s: %d\n", state, count))
	}

	if len(report.Gaps) > 0 {
		builder.WriteString("\nGaps:\n")
		for _, gap := range report.Gaps {
			builder.WriteString(fmt.Sprintf("- %s: %s\n", gap.Kind, gap.Detail))
		}
	}

	if len(report.Sections) > 0 {
		builder.WriteString("\nSections:\n")
	}

	for _, section := range report.Sections {
		builder.WriteString(fmt.Sprintf("- %s [%s]\n", section.SectionID, section.State))
		builder.WriteString(fmt.Sprintf("  why: %s\n", strings.Join(section.Why, "; ")))
		builder.WriteString(fmt.Sprintf("  next_action: %s\n", section.NextAction))
		if len(section.Gaps) > 0 {
			builder.WriteString(fmt.Sprintf("  gaps: %s\n", formatSpecCoverageGapKinds(section.Gaps)))
		}
	}

	_, err := io.WriteString(w, builder.String())

	return err
}

func writeSpecPlanJSON(w io.Writer, report project.SpecPlanReport) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")

	return encoder.Encode(report)
}

func writeSpecPlanAcceptJSON(w io.Writer, result specPlanAcceptResult) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")

	return encoder.Encode(result)
}

func writeSpecPlanAcceptSummary(w io.Writer, result specPlanAcceptResult) error {
	builder := strings.Builder{}
	builder.WriteString(fmt.Sprintf("haft spec plan: accepted %s\n", result.ProposalID))
	builder.WriteString(fmt.Sprintf("decision_ref: %s\n", result.DecisionRef))
	builder.WriteString(fmt.Sprintf("sections: %s\n", strings.Join(result.SectionRefs, ", ")))
	builder.WriteString("WorkCommissions: none created\n")

	_, err := io.WriteString(w, builder.String())

	return err
}

func writeSpecPlanSummary(w io.Writer, report project.SpecPlanReport) error {
	builder := strings.Builder{}
	builder.WriteString(fmt.Sprintf(
		"haft spec plan: %d proposal(s) from %d uncovered/stale section(s)\n",
		report.Summary.TotalProposals,
		report.Summary.TotalCandidates,
	))
	builder.WriteString(fmt.Sprintf("authority: %s\n", report.Authority))
	builder.WriteString(fmt.Sprintf("review_actions: %s\n", formatSpecPlanReviewActions(report.ReviewActions)))

	if len(report.Proposals) > 0 {
		builder.WriteString("\nProposals:\n")
	}

	for _, proposal := range report.Proposals {
		builder.WriteString(fmt.Sprintf("- %s: %s\n", proposal.ID, proposal.Title))
		builder.WriteString(fmt.Sprintf("  group: document_kind=%s spec_kind=%s affected_area=%s\n", proposal.DocumentKind, proposal.SpecKind, proposal.AffectedArea))
		builder.WriteString(fmt.Sprintf("  dependencies: %s\n", formatSpecPlanValues(proposal.DependencyRefs)))
		builder.WriteString(fmt.Sprintf("  sections: %s\n", strings.Join(proposal.SectionRefs, ", ")))
		builder.WriteString(fmt.Sprintf("  states: %s\n", formatSpecPlanStates(proposal.States)))
		builder.WriteString("  decision_record_draft:\n")
		builder.WriteString(fmt.Sprintf("    selected_title: %s\n", proposal.DecisionRecordDraft.SelectedTitle))
		builder.WriteString(fmt.Sprintf("    section_refs: %s\n", strings.Join(proposal.DecisionRecordDraft.SectionRefs, ", ")))
		builder.WriteString(fmt.Sprintf("    weakest_link: %s\n", proposal.DecisionRecordDraft.WeakestLink))
		if len(proposal.Reasons) > 0 {
			builder.WriteString(fmt.Sprintf("  reasons: %s\n", strings.Join(proposal.Reasons, " | ")))
		}
	}

	_, err := io.WriteString(w, builder.String())

	return err
}

func formatSpecPlanReviewActions(actions []project.SpecPlanReviewAction) string {
	kinds := make([]string, 0, len(actions))

	for _, action := range actions {
		kinds = append(kinds, string(action.Kind))
	}

	return strings.Join(cleanStringSlice(kinds), ", ")
}

func formatSpecPlanValues(values []string) string {
	if len(values) == 0 {
		return "(none)"
	}

	return strings.Join(values, ", ")
}

func formatSpecPlanStates(states []project.SpecCoverageState) string {
	values := make([]string, 0, len(states))

	for _, state := range states {
		values = append(values, string(state))
	}

	return strings.Join(cleanStringSlice(values), ", ")
}

func specCoverageStateOrder() []project.SpecCoverageState {
	return []project.SpecCoverageState{
		project.SpecCoverageUncovered,
		project.SpecCoverageReasoned,
		project.SpecCoverageCommissioned,
		project.SpecCoverageImplemented,
		project.SpecCoverageVerified,
		project.SpecCoverageStale,
	}
}

func formatSpecCoverageGapKinds(gaps []project.SpecCoverageGap) string {
	kinds := make([]string, 0, len(gaps))

	for _, gap := range gaps {
		kinds = append(kinds, gap.Kind)
	}

	return strings.Join(cleanStringSlice(kinds), ", ")
}
