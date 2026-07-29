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
	"github.com/m0n0x41d/haft/internal/project/specflow"
)

var (
	specCheckJSON           bool
	specCheckScopeID        string
	specCoverageJSON        bool
	specCoverageScopeID     string
	specPlanJSON            bool
	specPlanAcceptID        string
	specPlanScopeID         string
	specStatusJSON          bool
	specStatusScopeID       string
	specNextJSON            bool
	specNextScopeID         string
	specUseJSON             bool
	specUseContext          string
	specUsePolicy           string
	specUseWaiverExpiresAt  string
	specUseGateFile         string
	specSyncJSON            bool
	specSyncSection         string
	specRepairEditionsJSON  bool
	specRepairEditionsApply bool
	specExportJSON          bool
	specExportMarkdown      bool
	specApplyChangeJSON     bool
	specApplyDryRun         bool
	specApplyBefore         string
	specApplyAfter          string
	specApplySection        string
	specApplyKind           string
	specClassifyChangeJSON  bool
	specClassifyBefore      string
	specClassifyAfter       string
	specClassifySection     string
	specClassifyKind        string
	specOnboardJSON         bool
	specOnboardScopeID      string
	specOnboardApproveID    string
	specOnboardReopenID     string
	specOnboardRebaseline   string
	specOnboardReason       string
	specOnboardApprovedBy   string
	specApproveJSON         bool
	specApproveApprovedBy   string
	specRebaselineJSON      bool
	specRebaselineReason    string
	specRebaselineBy        string
	specReopenJSON          bool
	specReopenReason        string
	specMigrateJSON         bool
	specCheckExit           = os.Exit
	specCoverageExit        = os.Exit
	specPlanExit            = os.Exit
)

var specCmd = &cobra.Command{
	Use:   "spec",
	Short: "Inspect project specification carriers",
}

var specMigrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Continue the project's current specification migration",
	Long: `Continue the one exact specification migration registered for this
project. Haft resolves its internal migration carrier and current state; the
operator never supplies packet paths, hashes, refs, targets, or recovery modes.

The same command is state-driven and preserves one boundary per invocation:
it first presents and records a human semantic review, on a later invocation
applies that exact reviewed migration, and resumes a sealed interrupted journal
when recovery is required. --json is always read-only and never opens /dev/tty
or performs a migration effect.`,
	Args: cobra.NoArgs,
	RunE: runSpecMigrate,
}

var specCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Run L0/L1/L1.5 structural checks for spec carriers",
	Long: `Run deterministic L0/L1/L1.5 checks for project specification carriers.

The check parses fenced YAML spec-section blocks, validates required structural
fields and L1.5 carrier shapes, and verifies that the term-map carrier has
parseable term entries. It does not perform L2 semantic validation or L3
runtime/evidence validation.

The public command first resolves the canonical project profile from SQLite.
A singleton scope is selected automatically; a mixed profile requires an exact
--scope-id. Non-applicable specification members are omitted before carrier or
SQL-edition parsing. If profile applicability is underdetermined, the command
returns one neutral cue and does not report a clean specification check.`,
	RunE: runSpecCheck,
}

var specStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the next spec lifecycle action",
	Long: `Show the next typed spec lifecycle action.

The status projection is a UX layer over the canonical WorkflowIntent, spec
carrier checks, and SpecSectionBaseline state. It does not mutate carriers,
approve sections, or rebaseline drift.

The public command resolves canonical profile applicability before reading the
SQL-first specification projection. A singleton scope is selected
automatically; a mixed profile requires an exact --scope-id. Structured output
retains the selected scope and canonical admission provenance.`,
	RunE: runSpecStatus,
}

var specNextCmd = &cobra.Command{
	Use:   "next",
	Short: "Print the next typed spec lifecycle projection",
	Long: `Print the next typed spec lifecycle projection.

Use --json for agent/MCP-style consumption. The payload keeps the underlying
WorkflowIntent recoverable so surfaces do not invent their own lifecycle
semantics.

The public command resolves canonical profile applicability before reading the
SQL-first specification projection. A singleton scope is selected
automatically; a mixed profile requires an exact --scope-id. A non-applicable
target or software carrier contributes no phase, and unresolved applicability
is returned as one neutral cue.`,
	RunE: runSpecNext,
}

var specCoverageCmd = &cobra.Command{
	Use:   "coverage",
	Short: "Show derived spec coverage by section",
	Long: `Show derived SpecCoverage for active spec sections.

Coverage is computed from spec-section refs, artifact links, WorkCommissions,
affected files, and attached evidence. It does not read or store manual
coverage status fields, and it does not report coverage percentages as the
primary truth.

The public command resolves canonical project-profile applicability before
reading the SQL-first specification projection. A singleton scope is selected
automatically; a mixed profile requires an exact --scope-id. Non-applicable
specification members are omitted normally. Unresolved applicability blocks
only this coverage operation and is returned as one neutral cue.`,
	RunE: runSpecCoverage,
}

var specSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync typed spec carriers into the project SQL edition store",
	Long: `Sync parsed .haft/specs/* SpecSection carrier blocks into the project
SQL edition store.

Only typed fenced yaml spec-section blocks are imported. Surrounding Markdown
prose remains carrier text, not authority. The command does not approve,
rebaseline, reopen, create evidence, pass gates, create claim truth or global
truth, create prose authority, or mutate SpecSectionApprovalBaseline rows.

By default the command reconciles the full carrier set, including removal of
SQL editions no longer present in the carriers. --section imports exactly one
named carrier section and never updates or removes any other SQL edition.`,
	RunE: runSpecSync,
}

var specRepairEditionsCmd = &cobra.Command{
	Use:   "repair-editions",
	Short: "Repair stale SQL SpecSection edition semantic hashes",
	Long: `Repair stale semantic_hash cache values in the SQL SpecSection edition
store.

By default this command is a dry-run and only reports rows where the stored
semantic_hash differs from HashSection(section_json). Use --apply to update
only the semantic_hash cache column. The command does not approve, rebaseline,
reopen, create evidence, pass gates, create claim truth or global truth, or
create prose authority.`,
	RunE: runSpecRepairEditions,
}

var specExportCmd = &cobra.Command{
	Use:   "export SECTION_ID",
	Short: "Render one SQL SpecSection edition as a Markdown carrier projection",
	Long: `Render one current SQL SpecSection edition as a deterministic Markdown
carrier projection.

SQL remains the source of truth. The rendered Markdown is a publication
projection for carrier synchronization only; it is not approval, rebaseline,
evidence, GateDecision, claim truth, global truth, or prose authority.`,
	Args: cobra.ExactArgs(1),
	RunE: runSpecExport,
}

var specApplyChangeCmd = &cobra.Command{
	Use:   "apply-change",
	Short: "Apply a reviewed SpecSection carrier change to SQL editions",
	Long: `Apply one explicit before/after SpecSection carrier change to the
project SQL edition store.

Only changes classified as recognized semantic scalar, relationship, or mixed
updates are written. Carrier-only changes are reported as no-op. Unknown or
high-risk changes block. The command does not approve, rebaseline, reopen, or
mutate SpecSectionApprovalBaseline rows, and it does not create evidence, pass
gates, create claim truth or global truth, or create prose authority.

Use --dry-run to run the same typed carrier parser, SQL conflict check, and
planned-edition projection without writing the SQL edition store.`,
	RunE: runSpecApplyChange,
}

var specClassifyChangeCmd = &cobra.Command{
	Use:   "classify-change",
	Short: "Classify an explicit before/after SpecSection carrier change",
	Long: `Classify one explicit before/after SpecSection carrier change.

The command parses the given carrier files, compares the requested section, and
reports whether the change is carrier-only, a recognized semantic scalar
update, a relationship update, mixed, or unknown/high-risk.

This is read-only review input for the future sync-back path. It does not write
SQLite, approve sections, rebaseline drift, create evidence, pass gates, create
claim truth or global truth, or create prose authority.`,
	RunE: runSpecClassifyChange,
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

var specApproveCmd = &cobra.Command{
	Use:   "approve SECTION_ID",
	Short: "Record a SpecSectionBaseline for an active section",
	Args:  cobra.ExactArgs(1),
	RunE:  runSpecApprove,
}

var specRebaselineCmd = &cobra.Command{
	Use:   "rebaseline SECTION_ID",
	Short: "Overwrite a SpecSectionBaseline after intentional carrier evolution",
	Args:  cobra.ExactArgs(1),
	RunE:  runSpecRebaseline,
}

var specReopenCmd = &cobra.Command{
	Use:   "reopen SECTION_ID",
	Short: "Delete a SpecSectionBaseline so the section re-enters review",
	Args:  cobra.ExactArgs(1),
	RunE:  runSpecReopen,
}

var specPlanCmd = &cobra.Command{
	Use:   "plan",
	Short: "Show DecisionRecord draft proposals for uncovered or stale spec sections",
	Long: `Show SpecPlan proposals for active spec sections whose coverage is uncovered or stale.

The command groups sections by document kind, spec kind, dependency signature,
and affected area. Listing output is a human-review draft surface only:
proposals are not authority. No DecisionRecords are created by listing, and
no WorkCommissions are created or scheduled.

Use --accept <proposal-id> to bind one exact reviewed proposal through the same
project-local decision policy as decision.decide. The default
explicit_h_decide mode treats the operator's explicit invocation as the human
gate and does not ask for a second phrase; opt-in strict_cli_speech_act adds the
readable /dev/tty review. Merge, split, and discard are typed non-executable
actions in this slice and are reported with command gaps.

The public command resolves canonical project-profile applicability before
deriving proposals. A singleton scope is selected automatically; a mixed
profile requires an exact --scope-id. No proposal is derived or accepted when
that exact applicability basis is unresolved.`,
	RunE: runSpecPlan,
}

func init() {
	specCheckCmd.Flags().BoolVar(&specCheckJSON, "json", false, "print structured JSON output")
	specCheckCmd.Flags().StringVar(
		&specCheckScopeID,
		"scope-id",
		"",
		"exact canonical project-profile ScopeID; required when several scopes exist",
	)
	specStatusCmd.Flags().BoolVar(&specStatusJSON, "json", false, "print structured JSON output")
	specStatusCmd.Flags().StringVar(
		&specStatusScopeID,
		"scope-id",
		"",
		"exact canonical project-profile ScopeID; required when several scopes exist",
	)
	specNextCmd.Flags().BoolVar(&specNextJSON, "json", false, "print structured JSON output")
	specNextCmd.Flags().StringVar(
		&specNextScopeID,
		"scope-id",
		"",
		"exact canonical project-profile ScopeID; required when several scopes exist",
	)
	specCoverageCmd.Flags().BoolVar(&specCoverageJSON, "json", false, "print structured JSON output")
	specCoverageCmd.Flags().StringVar(
		&specCoverageScopeID,
		"scope-id",
		"",
		"exact canonical project-profile ScopeID; required when several scopes exist",
	)
	specReviewCmd.Flags().BoolVar(&specReviewJSON, "json", false, "print structured JSON output")
	specUseCmd.Flags().BoolVar(&specUseJSON, "json", false, "print structured JSON output")
	specUseCmd.Flags().StringVar(&specUseContext, "context", "", "declared use context for the SpecificationUseRecord")
	specUseCmd.Flags().StringVar(&specUsePolicy, "policy", "", "admission policy: documentary_only, stronger_use_requires_current_source, or temporary_waiver")
	specUseCmd.Flags().StringVar(&specUseWaiverExpiresAt, "waiver-expires-at", "", "expiry for temporary_waiver policy (RFC3339 or YYYY-MM-DD)")
	specUseCmd.Flags().StringVar(&specUseGateFile, "gate-file", "", "JSON OperationalGate profile for local read-only gate evaluation")
	specSyncCmd.Flags().BoolVar(&specSyncJSON, "json", false, "print structured JSON output")
	specSyncCmd.Flags().StringVar(
		&specSyncSection,
		"section",
		"",
		"import exactly one SpecSection without reconciling any other edition",
	)
	specRepairEditionsCmd.Flags().BoolVar(&specRepairEditionsJSON, "json", false, "print structured JSON output")
	specRepairEditionsCmd.Flags().BoolVar(&specRepairEditionsApply, "apply", false, "write repaired semantic_hash cache values instead of dry-run")
	specExportCmd.Flags().BoolVar(&specExportJSON, "json", false, "print structured JSON output")
	specExportCmd.Flags().BoolVar(&specExportMarkdown, "markdown", false, "print only the generated Markdown carrier projection")
	specApplyChangeCmd.Flags().BoolVar(&specApplyChangeJSON, "json", false, "print structured JSON output")
	specApplyChangeCmd.Flags().BoolVar(&specApplyDryRun, "dry-run", false, "preview the SQL sync-back without writing the edition store")
	specApplyChangeCmd.Flags().StringVar(&specApplyBefore, "before", "", "path to the before spec carrier")
	specApplyChangeCmd.Flags().StringVar(&specApplyAfter, "after", "", "path to the after spec carrier")
	specApplyChangeCmd.Flags().StringVar(&specApplySection, "section", "", "SpecSection id to apply")
	specApplyChangeCmd.Flags().StringVar(&specApplyKind, "kind", "", "carrier kind override: target-system, software-system, or term-map")
	specClassifyChangeCmd.Flags().BoolVar(&specClassifyChangeJSON, "json", false, "print structured JSON output")
	specClassifyChangeCmd.Flags().StringVar(&specClassifyBefore, "before", "", "path to the before spec carrier")
	specClassifyChangeCmd.Flags().StringVar(&specClassifyAfter, "after", "", "path to the after spec carrier")
	specClassifyChangeCmd.Flags().StringVar(&specClassifySection, "section", "", "SpecSection id to compare")
	specClassifyChangeCmd.Flags().StringVar(&specClassifyKind, "kind", "", "carrier kind override: target-system, software-system, or term-map")
	specPlanCmd.Flags().BoolVar(&specPlanJSON, "json", false, "print structured JSON output")
	specPlanCmd.Flags().StringVar(&specPlanAcceptID, "accept", "", "review proposal id and manually bind one DecisionRecord")
	specPlanCmd.Flags().StringVar(
		&specPlanScopeID,
		"scope-id",
		"",
		"exact canonical project-profile ScopeID; required when several scopes exist",
	)
	specOnboardCmd.Flags().BoolVar(&specOnboardJSON, "json", false, "print structured JSON output")
	specOnboardCmd.Flags().StringVar(
		&specOnboardScopeID,
		"scope-id",
		"",
		"exact canonical project-profile ScopeID; required when several scopes exist",
	)
	specOnboardCmd.Flags().StringVar(&specOnboardApproveID, "approve", "", "record a SpecSectionBaseline for the given active section id")
	specOnboardCmd.Flags().StringVar(&specOnboardRebaseline, "rebaseline", "", "overwrite an existing SpecSectionBaseline for the given section id (requires --reason)")
	specOnboardCmd.Flags().StringVar(&specOnboardReopenID, "reopen", "", "delete the SpecSectionBaseline for the given section id so it re-enters the onboarding loop")
	specOnboardCmd.Flags().StringVar(&specOnboardReason, "reason", "", "audit-trail rationale recorded with --rebaseline / --reopen")
	specOnboardCmd.Flags().StringVar(&specOnboardApprovedBy, "approved-by", "", "identifier of who approved the baseline (default: human)")
	specApproveCmd.Flags().BoolVar(&specApproveJSON, "json", false, "print structured JSON output")
	specApproveCmd.Flags().StringVar(&specApproveApprovedBy, "approved-by", "", "identifier of who approved the baseline (default: human)")
	specRebaselineCmd.Flags().BoolVar(&specRebaselineJSON, "json", false, "print structured JSON output")
	specRebaselineCmd.Flags().StringVar(&specRebaselineReason, "reason", "", "audit-trail rationale explaining the baseline change")
	specRebaselineCmd.Flags().StringVar(&specRebaselineBy, "approved-by", "", "identifier of who approved the rebaseline (default: human)")
	specReopenCmd.Flags().BoolVar(&specReopenJSON, "json", false, "print structured JSON output")
	specReopenCmd.Flags().StringVar(&specReopenReason, "reason", "", "audit-trail rationale explaining why the baseline is reopened")
	specMigrateCmd.Flags().BoolVar(&specMigrateJSON, "json", false, "inspect current migration state as JSON without review or mutation")
	specCmd.AddCommand(specCheckCmd)
	specCmd.AddCommand(specStatusCmd)
	specCmd.AddCommand(specNextCmd)
	specCmd.AddCommand(specCoverageCmd)
	specCmd.AddCommand(specReviewCmd)
	specCmd.AddCommand(specUseCmd)
	specCmd.AddCommand(specSyncCmd)
	specCmd.AddCommand(specRepairEditionsCmd)
	specCmd.AddCommand(specExportCmd)
	specCmd.AddCommand(specApplyChangeCmd)
	specCmd.AddCommand(specClassifyChangeCmd)
	specCmd.AddCommand(specPlanCmd)
	specCmd.AddCommand(specOnboardCmd)
	specCmd.AddCommand(specApproveCmd)
	specCmd.AddCommand(specRebaselineCmd)
	specCmd.AddCommand(specReopenCmd)
	specCmd.AddCommand(specMigrateCmd)
	rootCmd.AddCommand(specCmd)
}

func runSpecCheck(cmd *cobra.Command, _ []string) error {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a haft project: %w", err)
	}

	request, err := projectSpecificationScopeRequestFromFlag(specCheckScopeID)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	result, err := buildPublicSpecCheck(ctx, projectRoot, request)
	if err != nil {
		return err
	}

	output := cmd.OutOrStdout()
	if specCheckJSON {
		err = writePublicSpecCheckJSON(output, result)
	} else if result.SpecCheckReport != nil {
		err = writeSpecCheckSummary(output, *result.SpecCheckReport)
	} else {
		err = writeProjectSpecificationApplicabilityCue(
			output,
			"haft spec check",
			result.ProfileApplicability,
		)
	}
	if err != nil {
		return err
	}

	if result.SpecCheckReport == nil || result.HasFindings() {
		specCheckExit(1)
	}

	return nil
}

func runSpecCoverage(cmd *cobra.Command, _ []string) error {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a haft project: %w", err)
	}

	request, err := projectSpecificationScopeRequestFromFlag(specCoverageScopeID)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	result, err := buildPublicSpecCoverage(ctx, projectRoot, request)
	if err != nil {
		blocked := &specCoverageBlockedError{}
		if specCoverageJSON && errors.As(err, &blocked) {
			writeErr := writeSpecCoverageBlockedJSON(
				cmd.OutOrStdout(),
				blocked.report,
				result.ProfileApplicability,
			)
			if writeErr != nil {
				return writeErr
			}

			specCoverageExit(1)
			return nil
		}

		return err
	}

	output := cmd.OutOrStdout()
	if specCoverageJSON {
		err = writePublicSpecCoverageJSON(output, result)
	} else if result.SpecCoverageReport != nil {
		err = writeSpecCoverageSummary(output, *result.SpecCoverageReport)
	} else {
		err = writeProjectSpecificationApplicabilityCue(
			output,
			"haft spec coverage",
			result.ProfileApplicability,
		)
	}
	if err != nil {
		return err
	}
	if result.SpecCoverageReport == nil {
		specCoverageExit(1)
	}
	return nil
}

func runSpecPlan(cmd *cobra.Command, _ []string) error {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a haft project: %w", err)
	}

	request, err := projectSpecificationScopeRequestFromFlag(specPlanScopeID)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	result, err := buildPublicSpecPlan(ctx, projectRoot, request)
	if err != nil {
		return err
	}
	if result.SpecPlanReport == nil {
		output := cmd.OutOrStdout()
		if specPlanJSON {
			err = writePublicSpecPlanJSON(output, result)
		} else {
			err = writeProjectSpecificationApplicabilityCue(
				output,
				"haft spec plan",
				result.ProfileApplicability,
			)
		}
		if err != nil {
			return err
		}
		specPlanExit(1)
		return nil
	}

	if strings.TrimSpace(specPlanAcceptID) != "" {
		accepted, err := acceptSpecPlanProposalForSpecificationSet(
			ctx,
			projectRoot,
			*result.SpecPlanReport,
			specPlanAcceptID,
			result.specificationSet,
		)
		if err != nil {
			return err
		}
		accepted.ProfileApplicability = result.ProfileApplicability

		output := cmd.OutOrStdout()
		if specPlanJSON {
			return writeSpecPlanAcceptJSON(output, accepted)
		}

		return writeSpecPlanAcceptSummary(output, accepted)
	}

	output := cmd.OutOrStdout()
	if specPlanJSON {
		return writePublicSpecPlanJSON(output, result)
	}

	return writeSpecPlanSummary(output, *result.SpecPlanReport)
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
	Status               string                                  `json:"status"`
	Reason               string                                  `json:"reason"`
	NextAction           string                                  `json:"next_action"`
	SpecCheck            project.SpecCheckReport                 `json:"spec_check"`
	Coverage             project.SpecCoverageReport              `json:"coverage"`
	ProfileApplicability publicProjectSpecificationApplicability `json:"profile_applicability"`
}

type publicSpecCoverageResult struct {
	*project.SpecCoverageReport
	ProfileApplicability publicProjectSpecificationApplicability `json:"profile_applicability"`
	specificationSet     project.ProjectSpecificationSet
}

type publicSpecPlanResult struct {
	*project.SpecPlanReport
	ProfileApplicability publicProjectSpecificationApplicability `json:"profile_applicability"`
	specificationSet     project.ProjectSpecificationSet
}

func buildPublicSpecCoverage(
	ctx context.Context,
	projectRoot string,
	request projectSpecificationScopeRequest,
) (publicSpecCoverageResult, error) {
	specSet, resolution, err := loadProjectSpecificationSetSQLFirstFromCanonicalProfile(
		ctx,
		projectRoot,
		request,
	)
	if err != nil {
		return publicSpecCoverageResult{}, err
	}
	applicability, err := publicProjectSpecificationApplicabilityFrom(
		resolution,
		request,
	)
	if err != nil {
		return publicSpecCoverageResult{}, err
	}
	result := publicSpecCoverageResult{
		ProfileApplicability: applicability,
		specificationSet:     specSet,
	}
	if _, _, resolved := resolution.Resolved(); !resolved {
		return result, nil
	}
	report, err := buildSpecCoverageReportFromSpecificationSet(
		ctx,
		applicability.ProjectRoot,
		specSet,
	)
	if err != nil {
		return result, err
	}
	result.SpecCoverageReport = &report
	return result, nil
}

func buildPublicSpecPlan(
	ctx context.Context,
	projectRoot string,
	request projectSpecificationScopeRequest,
) (publicSpecPlanResult, error) {
	coverage, err := buildPublicSpecCoverage(
		ctx,
		projectRoot,
		request,
	)
	result := publicSpecPlanResult{
		ProfileApplicability: coverage.ProfileApplicability,
		specificationSet:     coverage.specificationSet,
	}
	if err != nil {
		return result, err
	}
	if coverage.SpecCoverageReport == nil {
		return result, nil
	}
	report := project.BuildSpecPlan(*coverage.SpecCoverageReport)
	result.SpecPlanReport = &report
	return result, nil
}

func buildSpecCoverageReportFromSpecificationSet(
	ctx context.Context,
	projectRoot string,
	specSet project.ProjectSpecificationSet,
) (project.SpecCoverageReport, error) {
	specCheck := project.SpecCheckReportFromSpecificationSet(specSet)
	if specCheck.HasFindings() {
		return project.SpecCoverageReport{}, &specCoverageBlockedError{report: specCheck}
	}
	sections := specSet.Sections

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

type specPlanAcceptResult struct {
	Action               string                                  `json:"action"`
	ProposalID           string                                  `json:"proposal_id"`
	DecisionRef          string                                  `json:"decision_ref"`
	DecisionTitle        string                                  `json:"decision_title"`
	DecisionMD           string                                  `json:"decision_md,omitempty"`
	SectionRefs          []string                                `json:"section_refs"`
	ExactReplay          bool                                    `json:"exact_replay,omitempty"`
	Warnings             []string                                `json:"warnings,omitempty"`
	TaskMemoryProjection *taskMemoryProjectionReport             `json:"task_memory_projection,omitempty"`
	ProfileApplicability publicProjectSpecificationApplicability `json:"profile_applicability"`
}

func acceptSpecPlanProposalForSpecificationSet(
	ctx context.Context,
	projectRoot string,
	report project.SpecPlanReport,
	proposalID string,
	specificationSet project.ProjectSpecificationSet,
) (specPlanAcceptResult, error) {
	proposal, ok := project.FindSpecPlanProposal(report, proposalID)
	if !ok {
		return specPlanAcceptResult{}, fmt.Errorf(
			"spec plan proposal %q not found; rerun `haft spec plan`",
			strings.TrimSpace(proposalID),
		)
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
		return specPlanAcceptResult{}, fmt.Errorf(
			"spec plan accept requires an initialized Haft project database",
		)
	}
	draft := specBindingDecisionDraftFromDecideInput(input)
	draft = enrichSpecBindingDecisionDraft(ctx, store, draft)
	preflight := specflow.BuildSpecBindingPreflight(
		specificationSet,
		specflow.SpecBindingPreflightInput{
			DecisionDraft: draft,
		},
	)
	input.SpecBindingPreflight = specBindingPreflightReceiptFromSpecflow(preflight)
	input.SpecBindingRequired = true
	return bindSpecPlanDecision(ctx, projectRoot, proposal, input)
}

func bindSpecPlanDecision(
	ctx context.Context,
	projectRoot string,
	proposal project.SpecPlanProposal,
	input artifact.DecideInput,
) (specPlanAcceptResult, error) {
	bound, err := bindDecisionByProjectPolicy(
		ctx,
		projectRoot,
		input,
	)
	if err != nil {
		return specPlanAcceptResult{}, err
	}
	return specPlanAcceptResult{
		Action:               string(project.SpecPlanActionAccept),
		ProposalID:           proposal.ID,
		DecisionRef:          bound.DecisionRef,
		DecisionTitle:        bound.Title,
		DecisionMD:           bound.FilePath,
		SectionRefs:          append([]string(nil), input.SectionRefs...),
		ExactReplay:          bound.ExactReplay,
		Warnings:             append([]string(nil), bound.Warnings...),
		TaskMemoryProjection: bound.TaskMemoryProjection,
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
		SelectedTitle: draft.SelectedTitle,
		ProblemStatement: fmt.Sprintf(
			"%s. Affected SpecSections: %s. Current basis: %s",
			proposal.Title,
			strings.Join(proposal.SectionRefs, ", "),
			strings.Join(proposal.Reasons, "; "),
		),
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

func writePublicSpecCoverageJSON(
	w io.Writer,
	result publicSpecCoverageResult,
) error {
	return writeIndentedJSON(w, result)
}

func writeSpecCoverageBlockedJSON(
	w io.Writer,
	specCheck project.SpecCheckReport,
	applicability publicProjectSpecificationApplicability,
) error {
	report := specCoverageBlockedJSONReport{
		Status:               "blocked",
		Reason:               fmt.Sprintf("spec check has %d finding(s)", specCheck.Summary.TotalFindings),
		NextAction:           "resolve spec_check.findings, then rerun `haft spec coverage --json`",
		SpecCheck:            specCheck,
		ProfileApplicability: applicability,
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

func writePublicSpecPlanJSON(
	w io.Writer,
	result publicSpecPlanResult,
) error {
	return writeIndentedJSON(w, result)
}

func writeSpecPlanAcceptJSON(w io.Writer, result specPlanAcceptResult) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")

	return encoder.Encode(result)
}

func writeSpecPlanAcceptSummary(w io.Writer, result specPlanAcceptResult) error {
	builder := strings.Builder{}
	builder.WriteString(fmt.Sprintf("haft spec plan: accepted %s\n", result.ProposalID))
	builder.WriteString(fmt.Sprintf("decision: %s — %s\n", result.DecisionRef, result.DecisionTitle))
	builder.WriteString(fmt.Sprintf("sections: %s\n", strings.Join(result.SectionRefs, ", ")))
	builder.WriteString("WorkCommissions: none created\n")
	for _, warning := range result.Warnings {
		builder.WriteString(fmt.Sprintf("warning: %s\n", warning))
	}
	if result.TaskMemoryProjection != nil {
		builder.WriteString(fmt.Sprintf(
			"typed memory: %s",
			result.TaskMemoryProjection.AdmissionResult,
		))
		if result.TaskMemoryProjection.RelationDeclarationFragmentID != "" {
			builder.WriteString(fmt.Sprintf(
				" (%s)",
				result.TaskMemoryProjection.RelationDeclarationFragmentID,
			))
		}
		builder.WriteString("\n")
	}

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
