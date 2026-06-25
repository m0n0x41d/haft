package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

const (
	sessionAuditKind      = "haft_session_graph_use_audit"
	sessionAuditAuthority = "read_only_session_audit_not_enforcement_gate"
)

var sessionAuditMutationBoundary = []string{
	"read_only_transcript_audit",
	"does_not_mutate_method_runs_decisions_evidence_or_carriers",
	"does_not_prove_plan_influence_without_closeout_or_review_evidence",
}

var (
	sessionAuditJSON  bool
	sessionAuditLimit int
)

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Inspect agent session transcript behavior",
}

var sessionAuditCmd = &cobra.Command{
	Use:   "audit PATH",
	Short: "Audit Haft graph-use behavior in Codex or Claude JSONL sessions",
	Long: `Audit Codex or Claude session JSONL for Haft graph-use behavior.

The audit is read-only. It checks ordering signals such as status before edit,
MethodPack pull before edit, code graph preflight before edit, richer graph
actions before edit, MethodPack close, and closeout text that ties graph
evidence to plan/risk/blast-radius changes.`,
	Args: cobra.ExactArgs(1),
	RunE: runSessionAudit,
}

func init() {
	sessionAuditCmd.Flags().BoolVar(&sessionAuditJSON, "json", false, "print the full audit as JSON")
	sessionAuditCmd.Flags().IntVar(&sessionAuditLimit, "limit", 20, "limit rendered sessions; set 0 for all")
	sessionCmd.AddCommand(sessionAuditCmd)
	rootCmd.AddCommand(sessionCmd)
}

type sessionGraphAuditReport struct {
	Kind             string                         `json:"kind"`
	SchemaVersion    int                            `json:"schema_version"`
	Authority        string                         `json:"authority"`
	MutationBoundary []string                       `json:"mutation_boundary"`
	ScanPolicy       sessionGraphAuditScanPolicy    `json:"scan_policy"`
	Summary          sessionGraphAuditSummary       `json:"summary"`
	Projection       *sessionGraphAuditProjection   `json:"projection,omitempty"`
	Diagnostics      []sessionGraphAuditDiagnostic  `json:"diagnostics,omitempty"`
	Sessions         []sessionGraphAuditSessionFile `json:"sessions,omitempty"`
}

type sessionGraphAuditScanPolicy struct {
	Input       string   `json:"input"`
	Files       []string `json:"files"`
	EventPolicy []string `json:"event_policy"`
}

type sessionGraphAuditSummary struct {
	FilesScanned                    int `json:"files_scanned"`
	SessionsWithEdits               int `json:"sessions_with_edits"`
	EventsScanned                   int `json:"events_scanned"`
	ToolCalls                       int `json:"tool_calls"`
	EditToolCalls                   int `json:"edit_tool_calls"`
	StatusBeforeFirstEdit           int `json:"status_before_first_edit"`
	MethodPullBeforeFirstEdit       int `json:"method_pull_before_first_edit"`
	GraphBeforeFirstEdit            int `json:"graph_before_first_edit"`
	RichGraphBeforeFirstEdit        int `json:"rich_graph_before_first_edit"`
	MethodCloseRecorded             int `json:"method_close_recorded"`
	GraphPlanInfluenceCloseoutHints int `json:"graph_plan_influence_closeout_hints"`
	Pass                            int `json:"pass"`
	NeedsReview                     int `json:"needs_review"`
	Fail                            int `json:"fail"`
	NoEdit                          int `json:"no_edit"`
	ParseErrors                     int `json:"parse_errors"`
}

type sessionGraphAuditProjection struct {
	View             string `json:"view"`
	Limit            int    `json:"limit"`
	OmittedSessions  int    `json:"omitted_sessions"`
	FullAuditCommand string `json:"full_audit_command"`
}

type sessionGraphAuditDiagnostic struct {
	Level      string   `json:"level"`
	Code       string   `json:"code"`
	Count      int      `json:"count"`
	Message    string   `json:"message"`
	NextAction string   `json:"next_action"`
	Examples   []string `json:"examples,omitempty"`
}

type sessionGraphAuditSessionFile struct {
	Path                            string   `json:"path"`
	Format                          string   `json:"format"`
	EventsScanned                   int      `json:"events_scanned"`
	ToolCalls                       int      `json:"tool_calls"`
	EditToolCalls                   int      `json:"edit_tool_calls"`
	FirstEditOrdinal                int      `json:"first_edit_ordinal,omitempty"`
	StatusBeforeFirstEdit           bool     `json:"status_before_first_edit"`
	MethodPullBeforeFirstEdit       bool     `json:"method_pull_before_first_edit"`
	GraphBeforeFirstEdit            bool     `json:"graph_before_first_edit"`
	RichGraphBeforeFirstEdit        bool     `json:"rich_graph_before_first_edit"`
	MethodCloseRecorded             bool     `json:"method_close_recorded"`
	GraphPlanInfluenceCloseoutHint  bool     `json:"graph_plan_influence_closeout_hint"`
	GraphActionsBeforeFirstEdit     []string `json:"graph_actions_before_first_edit,omitempty"`
	RichGraphActionsBeforeFirstEdit []string `json:"rich_graph_actions_before_first_edit,omitempty"`
	ParseErrors                     int      `json:"parse_errors,omitempty"`
	Verdict                         string   `json:"verdict"`
	Rationale                       string   `json:"rationale"`
}

type sessionAuditToolEvent struct {
	Ordinal int
	Tool    string
	Action  string
	Payload string
}

func runSessionAudit(cmd *cobra.Command, args []string) error {
	if sessionAuditLimit < 0 {
		return fmt.Errorf("limit must be >= 0")
	}

	report, err := buildSessionGraphAuditReport(args[0])
	if err != nil {
		return err
	}
	if sessionAuditLimit > 0 {
		report = limitSessionGraphAuditReport(report, sessionAuditLimit)
	}
	if sessionAuditJSON {
		return writeJSON(cmd.OutOrStdout(), report)
	}

	return writeSessionGraphAuditText(cmd.OutOrStdout(), report)
}

func buildSessionGraphAuditReport(inputPath string) (sessionGraphAuditReport, error) {
	files, err := sessionAuditFiles(inputPath)
	if err != nil {
		return sessionGraphAuditReport{}, err
	}

	report := sessionGraphAuditReport{
		Kind:             sessionAuditKind,
		SchemaVersion:    1,
		Authority:        sessionAuditAuthority,
		MutationBoundary: append([]string(nil), sessionAuditMutationBoundary...),
		ScanPolicy: sessionGraphAuditScanPolicy{
			Input: filepath.Clean(inputPath),
			Files: append([]string(nil), files...),
			EventPolicy: []string{
				"only_tool_use_events_counted",
				"session_metadata_and_system_prompts_ignored",
				"plan_influence_requires_method_closeout_hint_or_human_review",
			},
		},
	}

	for _, file := range files {
		session, err := auditSessionGraphUseFile(file)
		if err != nil {
			return sessionGraphAuditReport{}, err
		}
		report.Sessions = append(report.Sessions, session)
		report.Summary = addSessionGraphAuditSummary(report.Summary, session)
	}
	report.Diagnostics = sessionAuditDiagnostics(report)

	return report, nil
}

func sessionAuditFiles(inputPath string) ([]string, error) {
	info, err := os.Stat(inputPath)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return []string{filepath.Clean(inputPath)}, nil
	}

	var files []string
	err = filepath.WalkDir(inputPath, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".jsonl" {
			return nil
		}
		files = append(files, filepath.Clean(path))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	if len(files) == 0 {
		return nil, fmt.Errorf("no .jsonl session files found under %s", inputPath)
	}
	return files, nil
}

func auditSessionGraphUseFile(path string) (sessionGraphAuditSessionFile, error) {
	file, err := os.Open(path)
	if err != nil {
		return sessionGraphAuditSessionFile{}, err
	}
	defer file.Close()

	session := sessionGraphAuditSessionFile{
		Path:   filepath.Clean(path),
		Format: "jsonl",
	}
	events := []sessionAuditToolEvent{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024*32)
	for scanner.Scan() {
		session.EventsScanned++
		nextEvents, parseErr := sessionAuditToolEvents(scanner.Bytes(), len(events)+1)
		if parseErr != nil {
			session.ParseErrors++
			continue
		}
		events = append(events, nextEvents...)
	}
	if err := scanner.Err(); err != nil {
		return sessionGraphAuditSessionFile{}, err
	}

	return summarizeSessionGraphAudit(session, events), nil
}

func sessionAuditToolEvents(line []byte, startOrdinal int) ([]sessionAuditToolEvent, error) {
	var value any
	if err := json.Unmarshal(line, &value); err != nil {
		return nil, err
	}

	calls := sessionAuditToolCalls(value)
	events := make([]sessionAuditToolEvent, 0, len(calls))
	for index, call := range calls {
		events = append(events, sessionAuditToolEvent{
			Ordinal: startOrdinal + index,
			Tool:    sessionAuditToolName(call),
			Action:  sessionAuditActionFromValue(call),
			Payload: sessionAuditJSONText(call),
		})
	}
	return events, nil
}

func sessionAuditToolCalls(value any) []map[string]any {
	root, ok := value.(map[string]any)
	if !ok {
		return nil
	}

	var calls []map[string]any
	calls = append(calls, sessionAuditCodexToolCalls(root)...)
	calls = append(calls, sessionAuditClaudeToolCalls(root)...)
	return calls
}

func sessionAuditCodexToolCalls(root map[string]any) []map[string]any {
	if sessionAuditStringField(root, "type") != "response_item" {
		return nil
	}
	payload, ok := root["payload"].(map[string]any)
	if !ok {
		return nil
	}
	if sessionAuditStringField(payload, "type") != "function_call" {
		return nil
	}
	return []map[string]any{payload}
}

func sessionAuditClaudeToolCalls(root map[string]any) []map[string]any {
	message, ok := root["message"].(map[string]any)
	if !ok {
		return nil
	}
	content, ok := message["content"].([]any)
	if !ok {
		return nil
	}

	var calls []map[string]any
	for _, item := range content {
		call, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if sessionAuditStringField(call, "type") != "tool_use" {
			continue
		}
		calls = append(calls, call)
	}
	return calls
}

func summarizeSessionGraphAudit(
	session sessionGraphAuditSessionFile,
	events []sessionAuditToolEvent,
) sessionGraphAuditSessionFile {
	session.ToolCalls = len(events)
	session.EditToolCalls = sessionAuditEditToolCallCount(events)
	session.FirstEditOrdinal = sessionAuditFirstEditOrdinal(events)
	session.MethodCloseRecorded = sessionAuditMethodCloseRecorded(events)
	session.GraphPlanInfluenceCloseoutHint = sessionAuditGraphPlanInfluenceCloseoutHint(events)

	limit := session.FirstEditOrdinal
	if limit == 0 {
		limit = len(events) + 1
	}

	beforeEdit := sessionAuditBeforeOrdinal(events, limit)
	session.StatusBeforeFirstEdit = sessionAuditHasHaftQueryAction(beforeEdit, "status")
	session.MethodPullBeforeFirstEdit = sessionAuditHasHaftMethodAction(beforeEdit, "pull")
	session.GraphActionsBeforeFirstEdit = sessionAuditGraphActions(beforeEdit)
	session.RichGraphActionsBeforeFirstEdit = sessionAuditRichGraphActions(beforeEdit)
	session.GraphBeforeFirstEdit = len(session.GraphActionsBeforeFirstEdit) > 0
	session.RichGraphBeforeFirstEdit = len(session.RichGraphActionsBeforeFirstEdit) > 0
	session.Verdict, session.Rationale = sessionAuditVerdict(session)
	return session
}

func addSessionGraphAuditSummary(
	summary sessionGraphAuditSummary,
	session sessionGraphAuditSessionFile,
) sessionGraphAuditSummary {
	summary.FilesScanned++
	summary.EventsScanned += session.EventsScanned
	summary.ToolCalls += session.ToolCalls
	summary.EditToolCalls += session.EditToolCalls
	summary.ParseErrors += session.ParseErrors
	if session.FirstEditOrdinal > 0 {
		summary.SessionsWithEdits++
	}
	if session.StatusBeforeFirstEdit {
		summary.StatusBeforeFirstEdit++
	}
	if session.MethodPullBeforeFirstEdit {
		summary.MethodPullBeforeFirstEdit++
	}
	if session.GraphBeforeFirstEdit {
		summary.GraphBeforeFirstEdit++
	}
	if session.RichGraphBeforeFirstEdit {
		summary.RichGraphBeforeFirstEdit++
	}
	if session.MethodCloseRecorded {
		summary.MethodCloseRecorded++
	}
	if session.GraphPlanInfluenceCloseoutHint {
		summary.GraphPlanInfluenceCloseoutHints++
	}
	switch session.Verdict {
	case "pass":
		summary.Pass++
	case "needs_review":
		summary.NeedsReview++
	case "fail":
		summary.Fail++
	case "no_edit":
		summary.NoEdit++
	}
	return summary
}

func sessionAuditDiagnostics(report sessionGraphAuditReport) []sessionGraphAuditDiagnostic {
	summary := report.Summary
	var diagnostics []sessionGraphAuditDiagnostic

	missingGraph := summary.SessionsWithEdits - summary.GraphBeforeFirstEdit
	if missingGraph > 0 {
		diagnostics = append(diagnostics, sessionGraphAuditDiagnostic{
			Level:      "high",
			Code:       "edit_before_graph_preflight",
			Count:      missingGraph,
			Message:    "session(s) edited files before any code_context/impact/node/explore/callers/callees graph preflight",
			NextAction: "Add MethodPack graph-preflight evidence for governed code work and inspect the listed sessions.",
			Examples:   sessionAuditExamples(report.Sessions, "fail", 3),
		})
	}

	missingRich := summary.SessionsWithEdits - summary.RichGraphBeforeFirstEdit
	if missingRich > 0 {
		diagnostics = append(diagnostics, sessionGraphAuditDiagnostic{
			Level:      "medium",
			Code:       "missing_rich_graph_traversal_before_edit",
			Count:      missingRich,
			Message:    "session(s) did not use impact/node/explore/callers/callees before first edit",
			NextAction: "Use task-specific graph traversal when changing governed symbols, not only status or broad search.",
		})
	}

	missingInfluence := summary.GraphBeforeFirstEdit - summary.GraphPlanInfluenceCloseoutHints
	if missingInfluence > 0 {
		diagnostics = append(diagnostics, sessionGraphAuditDiagnostic{
			Level:      "medium",
			Code:       "plan_influence_not_proven",
			Count:      missingInfluence,
			Message:    "graph preflight happened, but deterministic transcript audit did not find closeout text tying graph output to plan/risk/blast-radius changes",
			NextAction: "Record graph evidence refs plus the plan influence note in haft_method closeout.",
		})
	}

	if summary.ParseErrors > 0 {
		diagnostics = append(diagnostics, sessionGraphAuditDiagnostic{
			Level:      "low",
			Code:       "jsonl_parse_errors",
			Count:      summary.ParseErrors,
			Message:    "line(s) could not be decoded as JSONL and were skipped",
			NextAction: "Inspect malformed transcript lines if counts look unexpectedly low.",
		})
	}

	return diagnostics
}

func sessionAuditExamples(
	sessions []sessionGraphAuditSessionFile,
	verdict string,
	limit int,
) []string {
	var examples []string
	for _, session := range sessions {
		if session.Verdict != verdict {
			continue
		}
		examples = append(examples, session.Path)
		if len(examples) >= limit {
			return examples
		}
	}
	return examples
}

func limitSessionGraphAuditReport(
	report sessionGraphAuditReport,
	limit int,
) sessionGraphAuditReport {
	if limit <= 0 {
		return report
	}

	total := len(report.Sessions)
	kept := total
	if kept > limit {
		kept = limit
	}

	limited := report
	limited.Sessions = append([]sessionGraphAuditSessionFile(nil), report.Sessions[:kept]...)
	limited.Projection = &sessionGraphAuditProjection{
		View:             "compact",
		Limit:            limit,
		OmittedSessions:  total - kept,
		FullAuditCommand: "haft session audit " + report.ScanPolicy.Input + " --json --limit 0",
	}
	return limited
}

func writeSessionGraphAuditText(
	output io.Writer,
	report sessionGraphAuditReport,
) error {
	println := func(args ...any) error {
		_, err := fmt.Fprintln(output, args...)
		return err
	}
	printf := func(format string, args ...any) error {
		_, err := fmt.Fprintf(output, format, args...)
		return err
	}

	if err := println("Haft session graph-use audit"); err != nil {
		return err
	}
	if err := printf("authority: %s\n", report.Authority); err != nil {
		return err
	}
	if err := printf(
		"summary: files=%d sessions_with_edits=%d pass=%d needs_review=%d fail=%d no_edit=%d graph_before_edit=%d rich_graph_before_edit=%d method_pull_before_edit=%d closeout_influence_hints=%d\n",
		report.Summary.FilesScanned,
		report.Summary.SessionsWithEdits,
		report.Summary.Pass,
		report.Summary.NeedsReview,
		report.Summary.Fail,
		report.Summary.NoEdit,
		report.Summary.GraphBeforeFirstEdit,
		report.Summary.RichGraphBeforeFirstEdit,
		report.Summary.MethodPullBeforeFirstEdit,
		report.Summary.GraphPlanInfluenceCloseoutHints,
	); err != nil {
		return err
	}
	if report.Projection != nil && report.Projection.OmittedSessions > 0 {
		if err := printf("projection: omitted_sessions=%d full=%s\n", report.Projection.OmittedSessions, report.Projection.FullAuditCommand); err != nil {
			return err
		}
	}
	if len(report.Diagnostics) > 0 {
		if err := println(); err != nil {
			return err
		}
		if err := println("Diagnostics:"); err != nil {
			return err
		}
		for _, diagnostic := range report.Diagnostics {
			if err := printf("- %s %s: %s", diagnostic.Level, diagnostic.Code, diagnostic.Message); err != nil {
				return err
			}
			if diagnostic.Count > 0 {
				if err := printf(" (%d)", diagnostic.Count); err != nil {
					return err
				}
			}
			if err := println(); err != nil {
				return err
			}
			if err := printf("  next: %s\n", diagnostic.NextAction); err != nil {
				return err
			}
		}
	}
	if len(report.Sessions) > 0 {
		if err := println(); err != nil {
			return err
		}
		if err := println("Sessions:"); err != nil {
			return err
		}
		for _, session := range report.Sessions {
			if err := printf(
				"- %s verdict=%s first_edit=%d status=%t method_pull=%t graph=%t rich_graph=%t close=%t influence_hint=%t actions=%s\n",
				session.Path,
				session.Verdict,
				session.FirstEditOrdinal,
				session.StatusBeforeFirstEdit,
				session.MethodPullBeforeFirstEdit,
				session.GraphBeforeFirstEdit,
				session.RichGraphBeforeFirstEdit,
				session.MethodCloseRecorded,
				session.GraphPlanInfluenceCloseoutHint,
				strings.Join(session.GraphActionsBeforeFirstEdit, ","),
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func sessionAuditFirstEditOrdinal(events []sessionAuditToolEvent) int {
	for _, event := range events {
		if sessionAuditIsEditTool(event) {
			return event.Ordinal
		}
	}
	return 0
}

func sessionAuditEditToolCallCount(events []sessionAuditToolEvent) int {
	count := 0
	for _, event := range events {
		if sessionAuditIsEditTool(event) {
			count++
		}
	}
	return count
}

func sessionAuditBeforeOrdinal(
	events []sessionAuditToolEvent,
	ordinal int,
) []sessionAuditToolEvent {
	var out []sessionAuditToolEvent
	for _, event := range events {
		if event.Ordinal >= ordinal {
			continue
		}
		out = append(out, event)
	}
	return out
}

func sessionAuditHasHaftQueryAction(
	events []sessionAuditToolEvent,
	action string,
) bool {
	for _, event := range events {
		if !sessionAuditIsHaftQuery(event) {
			continue
		}
		if event.Action == action {
			return true
		}
	}
	return false
}

func sessionAuditHasHaftMethodAction(
	events []sessionAuditToolEvent,
	action string,
) bool {
	for _, event := range events {
		if !sessionAuditIsHaftMethod(event) {
			continue
		}
		if event.Action == action {
			return true
		}
	}
	return false
}

func sessionAuditGraphActions(events []sessionAuditToolEvent) []string {
	var actions []string
	for _, event := range events {
		if !sessionAuditIsHaftQuery(event) {
			continue
		}
		if !sessionAuditIsGraphAction(event.Action) {
			continue
		}
		actions = append(actions, event.Action)
	}
	return sessionAuditDedupeStrings(actions)
}

func sessionAuditRichGraphActions(events []sessionAuditToolEvent) []string {
	var actions []string
	for _, event := range events {
		if !sessionAuditIsHaftQuery(event) {
			continue
		}
		if !sessionAuditIsRichGraphAction(event.Action) {
			continue
		}
		actions = append(actions, event.Action)
	}
	return sessionAuditDedupeStrings(actions)
}

func sessionAuditMethodCloseRecorded(events []sessionAuditToolEvent) bool {
	for _, event := range events {
		if sessionAuditIsHaftMethod(event) && event.Action == "close" {
			return true
		}
	}
	return false
}

func sessionAuditGraphPlanInfluenceCloseoutHint(events []sessionAuditToolEvent) bool {
	for _, event := range events {
		if !sessionAuditIsHaftMethod(event) || event.Action != "close" {
			continue
		}
		lowered := strings.ToLower(event.Payload)
		if !sessionAuditTextMentionsGraphEvidence(lowered) {
			continue
		}
		if sessionAuditTextMentionsPlanInfluence(lowered) {
			return true
		}
	}
	return false
}

func sessionAuditVerdict(session sessionGraphAuditSessionFile) (string, string) {
	if session.FirstEditOrdinal == 0 {
		return "no_edit", "No edit tool call was detected, so graph-before-edit enforcement is not applicable."
	}
	if session.StatusBeforeFirstEdit &&
		session.MethodPullBeforeFirstEdit &&
		session.GraphBeforeFirstEdit &&
		session.GraphPlanInfluenceCloseoutHint {
		return "pass", "Status, MethodPack pull, graph preflight, and closeout influence evidence were present."
	}
	if session.GraphBeforeFirstEdit {
		return "needs_review", "Graph preflight happened before edit, but status, MethodPack pull, or closeout influence evidence is missing."
	}
	return "fail", "An edit happened before detectable code graph preflight."
}

func sessionAuditIsEditTool(event sessionAuditToolEvent) bool {
	tool := strings.ToLower(event.Tool)
	payload := strings.ToLower(event.Payload)
	for _, marker := range []string{"apply_patch", "notebookedit", "multiedit", ".edit", "edit", ".write", "write"} {
		if strings.Contains(tool, marker) {
			return true
		}
	}
	for _, marker := range []string{"apply_patch", "cat >", "tee ", "sed -i", "perl -pi"} {
		if strings.Contains(payload, marker) {
			return true
		}
	}
	return false
}

func sessionAuditIsHaftQuery(event sessionAuditToolEvent) bool {
	return strings.Contains(strings.ToLower(event.Tool), "haft_query")
}

func sessionAuditIsHaftMethod(event sessionAuditToolEvent) bool {
	return strings.Contains(strings.ToLower(event.Tool), "haft_method")
}

func sessionAuditIsGraphAction(action string) bool {
	switch action {
	case "code_context", "impact", "node", "explore", "callers", "callees":
		return true
	}
	return false
}

func sessionAuditIsRichGraphAction(action string) bool {
	switch action {
	case "impact", "node", "explore", "callers", "callees":
		return true
	}
	return false
}

func sessionAuditTextMentionsGraphEvidence(text string) bool {
	for _, marker := range []string{"code_context", "impact", "node", "explore", "callers", "callees"} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func sessionAuditTextMentionsPlanInfluence(text string) bool {
	for _, marker := range []string{"plan", "risk", "blast", "changed", "file choice", "scope", "influence"} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func sessionAuditToolName(call map[string]any) string {
	name := sessionAuditStringField(call, "name")
	namespace := sessionAuditStringField(call, "namespace")
	if namespace != "" && name != "" {
		return namespace + "." + name
	}
	if name != "" {
		return name
	}
	for _, key := range []string{"tool_name", "recipient_name"} {
		if value := sessionAuditStringField(call, key); value != "" {
			return value
		}
	}
	return "unknown"
}

func sessionAuditActionFromValue(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		if action := sessionAuditStringField(typed, "action"); action != "" {
			return sessionAuditNormalizeToken(action)
		}
		for _, key := range []string{"arguments", "input"} {
			if action := sessionAuditActionFromValue(typed[key]); action != "" {
				return action
			}
		}
		for _, nested := range typed {
			if action := sessionAuditActionFromValue(nested); action != "" {
				return action
			}
		}
	case []any:
		for _, nested := range typed {
			if action := sessionAuditActionFromValue(nested); action != "" {
				return action
			}
		}
	case string:
		return sessionAuditActionFromString(typed)
	}
	return ""
}

func sessionAuditActionFromString(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "{") {
		var decoded any
		if err := json.Unmarshal([]byte(trimmed), &decoded); err == nil {
			return sessionAuditActionFromValue(decoded)
		}
	}
	lowered := strings.ToLower(trimmed)
	for _, action := range []string{
		"status",
		"pull",
		"close",
		"code_context",
		"impact",
		"node",
		"explore",
		"callers",
		"callees",
	} {
		quoted := `"action":"` + action + `"`
		spaced := `"action": "` + action + `"`
		if strings.Contains(lowered, quoted) || strings.Contains(lowered, spaced) {
			return action
		}
	}
	return ""
}

func sessionAuditStringField(
	value map[string]any,
	key string,
) string {
	raw, ok := value[key]
	if !ok {
		return ""
	}
	text, ok := raw.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func sessionAuditNormalizeToken(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, "-", "_")
	value = strings.ReplaceAll(value, " ", "_")
	return value
}

func sessionAuditJSONText(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(data)
}

func sessionAuditDedupeStrings(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		value = sessionAuditNormalizeToken(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
