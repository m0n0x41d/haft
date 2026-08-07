package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

const (
	processReconcileKind      = "haft_process_reconcile"
	processReconcileAuthority = "read_only_process_reconciliation_report_not_processpattern_or_apply_authority"
)

const maxProcessReconcileTextFindings = 20

var processReconcileJSON bool

var processReconcileCmd = &cobra.Command{
	Use:   "reconcile",
	Short: "Read-only process authority reconciliation report",
	Long: `Inspect the derived process-authority index for duplicate current authority,
non-current history, and carrier hygiene.

This report is review input only. It does not create ProcessPattern objects,
does not mutate MethodPack definitions, and does not authorize apply actions.`,
	RunE: runProcessReconcile,
}

type processReconcileReport struct {
	Kind              string                    `json:"kind"`
	SchemaVersion     int                       `json:"schema_version"`
	Authority         string                    `json:"authority"`
	AuthorityBoundary string                    `json:"authority_boundary"`
	SourceAuthority   string                    `json:"source_authority"`
	Summary           processReconcileSummary   `json:"summary"`
	Findings          []ProcessReconcileFinding `json:"findings,omitempty"`
	MutationBoundary  []string                  `json:"mutation_boundary"`
	Notes             []string                  `json:"notes,omitempty"`
}

type processReconcileSummary struct {
	Entries                  int `json:"entries"`
	Findings                 int `json:"findings"`
	DuplicateCurrentTargets  int `json:"duplicate_current_targets"`
	NonCurrentHistoryEntries int `json:"non_current_history_entries"`
	MissingCarrierRefs       int `json:"missing_carrier_refs"`
	ApplyReadyMutations      int `json:"apply_ready_mutations"`
}

type ProcessReconcileFinding struct {
	FindingID         string   `json:"finding_id"`
	FindingKind       string   `json:"finding_kind"`
	Severity          string   `json:"severity"`
	TargetRef         string   `json:"target_ref,omitempty"`
	ClaimKind         string   `json:"claim_kind,omitempty"`
	BoundedContext    string   `json:"bounded_context,omitempty"`
	LifecycleStatuses []string `json:"lifecycle_statuses,omitempty"`
	AuthorityKeys     []string `json:"authority_keys,omitempty"`
	SourceRefs        []string `json:"source_refs,omitempty"`
	NextAction        string   `json:"next_action"`
	AuthorityBoundary string   `json:"authority_boundary"`
}

func init() {
	processReconcileCmd.Flags().BoolVar(&processReconcileJSON, "json", false, "print structured JSON output")
	processCmd.AddCommand(processReconcileCmd)
}

func runProcessReconcile(cmd *cobra.Command, args []string) error {
	authorityReport, err := buildProcessAuthorityReport()
	if err != nil {
		return err
	}
	report := buildProcessReconcileReport(authorityReport)
	if processReconcileJSON {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return err
	}
	return writeProcessReconcileText(cmd.OutOrStdout(), report)
}

func buildProcessReconcileReport(authorityReport processAuthorityReport) processReconcileReport {
	findings := processReconcileFindings(authorityReport.Entries)
	sort.SliceStable(findings, func(i, j int) bool {
		return findings[i].FindingID < findings[j].FindingID
	})

	return processReconcileReport{
		Kind:              processReconcileKind,
		SchemaVersion:     1,
		Authority:         processReconcileAuthority,
		AuthorityBoundary: "report_only_not_processpattern_not_approval_not_evidence_truth_not_gate_passage_not_enforcement_not_apply_authority",
		SourceAuthority:   authorityReport.Authority,
		Summary:           summarizeProcessReconcile(authorityReport.Entries, findings),
		Findings:          findings,
		MutationBoundary: []string{
			"read_only_reconciliation_report",
			"does_not_mutate_methodpack_definitions_methodruns_interfaces_or_carriers",
			"does_not_create_processpattern_objects",
			"does_not_authorize_apply_or_operator_approval",
		},
		Notes: []string{
			"Canonical process authority remains in MethodPack definitions and kernel interface contracts.",
			"Use this report to decide whether MethodPack lifecycle metadata needs cleanup; do not treat it as apply authority.",
		},
	}
}

func processReconcileFindings(entries []ProcessAuthorityEntry) []ProcessReconcileFinding {
	findings := duplicateCurrentProcessAuthorityFindings(entries)
	findings = append(findings, nonCurrentProcessAuthorityFindings(entries)...)
	findings = append(findings, missingCarrierProcessAuthorityFindings(entries)...)
	return findings
}

func duplicateCurrentProcessAuthorityFindings(entries []ProcessAuthorityEntry) []ProcessReconcileFinding {
	groups := map[string][]ProcessAuthorityEntry{}
	for _, entry := range entries {
		if entry.LifecycleStatus != "current" {
			continue
		}
		key := processAuthorityTargetGroupKey(entry)
		if key == "" {
			continue
		}
		groups[key] = append(groups[key], entry)
	}

	var findings []ProcessReconcileFinding
	for _, group := range groups {
		if len(group) < 2 {
			continue
		}
		first := group[0]
		findings = append(findings, ProcessReconcileFinding{
			FindingID:         "duplicate_current_authority:" + processReconcileSlug(processAuthorityTargetGroupKey(first)),
			FindingKind:       "duplicate_current_authority",
			Severity:          "needs_review",
			TargetRef:         first.TargetRef,
			ClaimKind:         first.ClaimKind,
			BoundedContext:    first.BoundedContext,
			LifecycleStatuses: processAuthorityLifecycleStatuses(group),
			AuthorityKeys:     processAuthorityKeys(group),
			SourceRefs:        processAuthoritySourceRefs(group),
			NextAction:        "review whether these current authority entries are intentionally co-governing; otherwise supersede/deprecate one MethodPack/interface source",
			AuthorityBoundary: "finding_is_review_input_not_apply_authority",
		})
	}
	return findings
}

func nonCurrentProcessAuthorityFindings(entries []ProcessAuthorityEntry) []ProcessReconcileFinding {
	var findings []ProcessReconcileFinding
	for _, entry := range entries {
		if entry.LifecycleStatus == "" || entry.LifecycleStatus == "current" {
			continue
		}
		findings = append(findings, ProcessReconcileFinding{
			FindingID:         "non_current_history:" + processReconcileSlug(entry.AuthorityKey),
			FindingKind:       "non_current_history",
			Severity:          "info",
			TargetRef:         entry.TargetRef,
			ClaimKind:         entry.ClaimKind,
			BoundedContext:    entry.BoundedContext,
			LifecycleStatuses: []string{entry.LifecycleStatus},
			AuthorityKeys:     []string{entry.AuthorityKey},
			SourceRefs:        []string{entry.SourceRef},
			NextAction:        "keep as history unless it still appears in current guidance; if it does, update carrier refs or successor metadata",
			AuthorityBoundary: "history_finding_not_current_authority_not_apply_authority",
		})
	}
	return findings
}

func missingCarrierProcessAuthorityFindings(entries []ProcessAuthorityEntry) []ProcessReconcileFinding {
	var findings []ProcessReconcileFinding
	for _, entry := range entries {
		if len(entry.CarrierRefs) > 0 {
			continue
		}
		findings = append(findings, ProcessReconcileFinding{
			FindingID:         "missing_carrier_refs:" + processReconcileSlug(entry.AuthorityKey),
			FindingKind:       "missing_carrier_refs",
			Severity:          "needs_review",
			TargetRef:         entry.TargetRef,
			ClaimKind:         entry.ClaimKind,
			BoundedContext:    entry.BoundedContext,
			LifecycleStatuses: []string{entry.LifecycleStatus},
			AuthorityKeys:     []string{entry.AuthorityKey},
			SourceRefs:        []string{entry.SourceRef},
			NextAction:        "attach a MethodPack, generated-contract, or interface carrier ref so agents can inspect the governing source",
			AuthorityBoundary: "carrier_hygiene_finding_not_apply_authority",
		})
	}
	return findings
}

func processAuthorityTargetGroupKey(entry ProcessAuthorityEntry) string {
	parts := []string{
		strings.TrimSpace(entry.BoundedContext),
		strings.TrimSpace(entry.ClaimKind),
		strings.TrimSpace(entry.TargetRef),
	}
	if parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return ""
	}
	return strings.Join(parts, "\x00")
}

func processAuthorityKeys(entries []ProcessAuthorityEntry) []string {
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		out = append(out, strings.TrimSpace(entry.AuthorityKey))
	}
	return sortedUniqueProcessStrings(out)
}

func processAuthoritySourceRefs(entries []ProcessAuthorityEntry) []string {
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		out = append(out, strings.TrimSpace(entry.SourceRef))
	}
	return sortedUniqueProcessStrings(out)
}

func processAuthorityLifecycleStatuses(entries []ProcessAuthorityEntry) []string {
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		out = append(out, strings.TrimSpace(entry.LifecycleStatus))
	}
	return sortedUniqueProcessStrings(out)
}

func sortedUniqueProcessStrings(values []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func summarizeProcessReconcile(entries []ProcessAuthorityEntry, findings []ProcessReconcileFinding) processReconcileSummary {
	summary := processReconcileSummary{
		Entries:             len(entries),
		Findings:            len(findings),
		ApplyReadyMutations: 0,
	}
	for _, finding := range findings {
		switch finding.FindingKind {
		case "duplicate_current_authority":
			summary.DuplicateCurrentTargets++
		case "non_current_history":
			summary.NonCurrentHistoryEntries++
		case "missing_carrier_refs":
			summary.MissingCarrierRefs++
		}
	}
	return summary
}

func writeProcessReconcileText(w io.Writer, report processReconcileReport) error {
	if _, err := fmt.Fprintln(w, "Haft process reconciliation report"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "findings=%d duplicate_current=%d non_current_history=%d missing_carriers=%d authority=%s\n",
		report.Summary.Findings,
		report.Summary.DuplicateCurrentTargets,
		report.Summary.NonCurrentHistoryEntries,
		report.Summary.MissingCarrierRefs,
		report.Authority,
	); err != nil {
		return err
	}
	findings := report.Findings
	if len(findings) > maxProcessReconcileTextFindings {
		findings = findings[:maxProcessReconcileTextFindings]
	}
	for _, finding := range findings {
		if _, err := fmt.Fprintf(w, "- %s %s target=%s action=%s\n",
			finding.Severity,
			finding.FindingKind,
			finding.TargetRef,
			finding.NextAction,
		); err != nil {
			return err
		}
	}
	if omitted := len(report.Findings) - len(findings); omitted > 0 {
		if _, err := fmt.Fprintf(w, "... and %d more; run `haft process reconcile --json` for full findings\n", omitted); err != nil {
			return err
		}
	}
	if len(report.Findings) == 0 {
		if _, err := fmt.Fprintln(w, "No process-authority reconciliation findings."); err != nil {
			return err
		}
	}
	return nil
}

func processReconcileSlug(value string) string {
	value = strings.ReplaceAll(value, "\x00", "-")
	var b strings.Builder
	for _, r := range strings.ToLower(value) {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}
