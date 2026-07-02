package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

const (
	sessionAuditKind      = "haft_session_graph_use_audit"
	sessionAuditAuthority = "read_only_session_audit_not_enforcement_gate"
)

var (
	sessionAuditRecommendedPatternRE = regexp.MustCompile(`"?recommended_pattern_use"?\s*:\s*\{[^}]*"?pattern_ref"?\s*:\s*"([^"]+)"`)
	sessionAuditSupportLevelRE       = regexp.MustCompile(`"?support_level"?\s*:\s*"([^"]+)"`)
	sessionAuditRouteStrategyRE      = regexp.MustCompile(`"?route_match_strategy"?\s*:\s*"([^"]+)"`)
)

var sessionAuditMutationBoundary = []string{
	"read_only_transcript_audit",
	"does_not_mutate_method_runs_decisions_evidence_or_carriers",
	"does_not_prove_plan_influence_without_closeout_or_review_evidence",
}

var (
	sessionAuditJSON             bool
	sessionAuditLimit            int
	sessionAuditExpectedPattern  string
	sessionAuditExpectedStrategy string
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
	sessionAuditCmd.Flags().StringVar(&sessionAuditExpectedPattern, "expect-pattern", "", "optional expected PatternUse pattern_ref for scenario audit; use none for mechanical controls")
	sessionAuditCmd.Flags().StringVar(&sessionAuditExpectedStrategy, "expect-strategy", "", "optional expected PatternUse route_match_strategy or support_level for scenario audit; use none for mechanical controls")
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
	Input                 string   `json:"input"`
	Files                 []string `json:"files"`
	ExpectedPatternRef    string   `json:"expected_pattern_ref,omitempty"`
	ExpectedRouteStrategy string   `json:"expected_route_strategy,omitempty"`
	EventPolicy           []string `json:"event_policy"`
}

type sessionGraphAuditSummary struct {
	FilesScanned                    int `json:"files_scanned"`
	SessionsWithEdits               int `json:"sessions_with_edits"`
	SessionsWithSubstantiveMoves    int `json:"sessions_with_substantive_moves"`
	EventsScanned                   int `json:"events_scanned"`
	ToolCalls                       int `json:"tool_calls"`
	EditToolCalls                   int `json:"edit_tool_calls"`
	StatusBeforeFirstEdit           int `json:"status_before_first_edit"`
	MethodPullBeforeFirstEdit       int `json:"method_pull_before_first_edit"`
	GraphBeforeFirstEdit            int `json:"graph_before_first_edit"`
	RichGraphBeforeFirstEdit        int `json:"rich_graph_before_first_edit"`
	PatternUseBeforeSubstantive     int `json:"pattern_use_before_substantive"`
	PatternUseBypasses              int `json:"pattern_use_bypasses"`
	PatternUseObserved              int `json:"pattern_use_observed"`
	ProgressiveDisclosureBypasses   int `json:"progressive_disclosure_bypasses"`
	ScenarioPass                    int `json:"scenario_pass"`
	ScenarioFail                    int `json:"scenario_fail"`
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
	Path                               string   `json:"path"`
	Format                             string   `json:"format"`
	EventsScanned                      int      `json:"events_scanned"`
	ToolCalls                          int      `json:"tool_calls"`
	EditToolCalls                      int      `json:"edit_tool_calls"`
	FirstEditOrdinal                   int      `json:"first_edit_ordinal,omitempty"`
	FirstSubstantiveOrdinal            int      `json:"first_substantive_ordinal,omitempty"`
	StatusBeforeFirstEdit              bool     `json:"status_before_first_edit"`
	MethodPullBeforeFirstEdit          bool     `json:"method_pull_before_first_edit"`
	GraphBeforeFirstEdit               bool     `json:"graph_before_first_edit"`
	RichGraphBeforeFirstEdit           bool     `json:"rich_graph_before_first_edit"`
	PatternUseBeforeSubstantive        bool     `json:"pattern_use_before_substantive"`
	PatternUseBypass                   bool     `json:"pattern_use_bypass"`
	CompactPatternUseSeen              bool     `json:"compact_pattern_use_seen"`
	RetrievedUncompiledSeen            bool     `json:"retrieved_uncompiled_seen"`
	FullPatternUseSeen                 bool     `json:"full_pattern_use_seen"`
	SourceCardSeen                     bool     `json:"source_card_seen"`
	FullBeforePatternApplication       bool     `json:"full_before_pattern_application"`
	ProgressiveDisclosureBypass        bool     `json:"progressive_disclosure_bypass"`
	ExpectedPatternRef                 string   `json:"expected_pattern_ref,omitempty"`
	ExpectedRouteStrategy              string   `json:"expected_route_strategy,omitempty"`
	ObservedPatternRefs                []string `json:"observed_pattern_refs,omitempty"`
	ObservedSupportLevels              []string `json:"observed_support_levels,omitempty"`
	ObservedRouteMatchStrategies       []string `json:"observed_route_match_strategies,omitempty"`
	ScenarioPass                       string   `json:"scenario_pass,omitempty"`
	FailureReason                      string   `json:"failure_reason,omitempty"`
	MethodCloseRecorded                bool     `json:"method_close_recorded"`
	GraphPlanInfluenceCloseoutHint     bool     `json:"graph_plan_influence_closeout_hint"`
	GraphActionsBeforeFirstEdit        []string `json:"graph_actions_before_first_edit,omitempty"`
	RichGraphActionsBeforeFirstEdit    []string `json:"rich_graph_actions_before_first_edit,omitempty"`
	SubstantiveActionsBeforePatternUse []string `json:"substantive_actions_before_pattern_use,omitempty"`
	ParseErrors                        int      `json:"parse_errors,omitempty"`
	Verdict                            string   `json:"verdict"`
	Rationale                          string   `json:"rationale"`
}

type sessionAuditToolEvent struct {
	Ordinal int
	Tool    string
	Action  string
	Payload string
}

type sessionAuditExpectation struct {
	PatternRef    string
	RouteStrategy string
}

func runSessionAudit(cmd *cobra.Command, args []string) error {
	if sessionAuditLimit < 0 {
		return fmt.Errorf("limit must be >= 0")
	}

	expectation := sessionAuditExpectation{
		PatternRef:    strings.TrimSpace(sessionAuditExpectedPattern),
		RouteStrategy: strings.TrimSpace(sessionAuditExpectedStrategy),
	}
	report, err := buildSessionGraphAuditReport(args[0], expectation)
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

func buildSessionGraphAuditReport(inputPath string, expectation sessionAuditExpectation) (sessionGraphAuditReport, error) {
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
			Input:                 filepath.Clean(inputPath),
			Files:                 append([]string(nil), files...),
			ExpectedPatternRef:    expectation.PatternRef,
			ExpectedRouteStrategy: expectation.RouteStrategy,
			EventPolicy: []string{
				"tool_use_and_assistant_text_events_counted",
				"session_metadata_and_system_prompts_ignored",
				"non_edit_substantive_reasoning_requires_pattern_use_gateway",
				"plan_influence_requires_method_closeout_hint_or_human_review",
			},
		},
	}

	for _, file := range files {
		session, err := auditSessionGraphUseFile(file, expectation)
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

func auditSessionGraphUseFile(path string, expectation sessionAuditExpectation) (sessionGraphAuditSessionFile, error) {
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

	return summarizeSessionGraphAudit(session, events, expectation), nil
}

func sessionAuditToolEvents(line []byte, startOrdinal int) ([]sessionAuditToolEvent, error) {
	var value any
	if err := json.Unmarshal(line, &value); err != nil {
		return nil, err
	}

	calls := sessionAuditToolCalls(value)
	events := make([]sessionAuditToolEvent, 0, len(calls)+2)
	for _, call := range calls {
		events = append(events, sessionAuditToolEvent{
			Ordinal: startOrdinal + len(events),
			Tool:    sessionAuditToolName(call),
			Action:  sessionAuditActionFromValue(call),
			Payload: sessionAuditJSONText(call),
		})
	}
	events = append(events, sessionAuditResultEvents(value, startOrdinal+len(events))...)
	events = append(events, sessionAuditTextEvents(value, startOrdinal+len(events))...)
	return events, nil
}

func sessionAuditToolCalls(value any) []map[string]any {
	root, ok := value.(map[string]any)
	if !ok {
		return nil
	}

	var calls []map[string]any
	calls = append(calls, sessionAuditCodexToolCalls(root)...)
	calls = append(calls, sessionAuditCodexExecToolCalls(root)...)
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

func sessionAuditCodexExecToolCalls(root map[string]any) []map[string]any {
	if sessionAuditStringField(root, "type") != "item.completed" {
		return nil
	}
	item, ok := root["item"].(map[string]any)
	if !ok {
		return nil
	}
	if sessionAuditStringField(item, "type") != "mcp_tool_call" {
		return nil
	}
	return []map[string]any{item}
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

func sessionAuditTextEvents(value any, startOrdinal int) []sessionAuditToolEvent {
	root, ok := value.(map[string]any)
	if !ok {
		return nil
	}

	texts := sessionAuditAgentTexts(root)
	events := make([]sessionAuditToolEvent, 0, len(texts))
	for index, text := range texts {
		events = append(events, sessionAuditToolEvent{
			Ordinal: startOrdinal + index,
			Tool:    "agent_message",
			Payload: text,
		})
	}
	return events
}

func sessionAuditResultEvents(value any, startOrdinal int) []sessionAuditToolEvent {
	root, ok := value.(map[string]any)
	if !ok {
		return nil
	}

	texts := sessionAuditResultTexts(root)
	events := make([]sessionAuditToolEvent, 0, len(texts))
	for index, text := range texts {
		events = append(events, sessionAuditToolEvent{
			Ordinal: startOrdinal + index,
			Tool:    "tool_result",
			Payload: text,
		})
	}
	return events
}

func sessionAuditAgentTexts(root map[string]any) []string {
	var texts []string
	texts = append(texts, sessionAuditCodexExecAgentTexts(root)...)
	texts = append(texts, sessionAuditCodexSessionAgentTexts(root)...)
	texts = append(texts, sessionAuditClaudeAgentTexts(root)...)
	return texts
}

func sessionAuditCodexExecAgentTexts(root map[string]any) []string {
	if sessionAuditStringField(root, "type") != "item.completed" {
		return nil
	}
	item, ok := root["item"].(map[string]any)
	if !ok {
		return nil
	}
	if sessionAuditStringField(item, "type") != "agent_message" {
		return nil
	}
	text := sessionAuditStringField(item, "text")
	if text == "" {
		return nil
	}
	return []string{text}
}

func sessionAuditCodexSessionAgentTexts(root map[string]any) []string {
	if sessionAuditStringField(root, "type") == "event_msg" {
		payload, ok := root["payload"].(map[string]any)
		if !ok || sessionAuditStringField(payload, "type") != "agent_message" {
			return nil
		}
		text := sessionAuditStringField(payload, "message")
		if text == "" {
			return nil
		}
		return []string{text}
	}
	if sessionAuditStringField(root, "type") != "response_item" {
		return nil
	}
	payload, ok := root["payload"].(map[string]any)
	if !ok || sessionAuditStringField(payload, "type") != "message" {
		return nil
	}
	return sessionAuditContentTexts(payload["content"])
}

func sessionAuditClaudeAgentTexts(root map[string]any) []string {
	message, ok := root["message"].(map[string]any)
	if !ok {
		return nil
	}
	return sessionAuditContentTexts(message["content"])
}

func sessionAuditContentTexts(value any) []string {
	content, ok := value.([]any)
	if !ok {
		return nil
	}

	var texts []string
	for _, item := range content {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		switch sessionAuditStringField(entry, "type") {
		case "output_text", "text":
			if text := sessionAuditStringField(entry, "text"); text != "" {
				texts = append(texts, text)
			}
		}
	}
	return texts
}

func sessionAuditResultTexts(root map[string]any) []string {
	var texts []string
	texts = append(texts, sessionAuditCodexExecResultTexts(root)...)
	texts = append(texts, sessionAuditCodexMCPResultTexts(root)...)
	texts = append(texts, sessionAuditCodexFunctionOutputTexts(root)...)
	return texts
}

func sessionAuditCodexExecResultTexts(root map[string]any) []string {
	if sessionAuditStringField(root, "type") != "item.completed" {
		return nil
	}
	item, ok := root["item"].(map[string]any)
	if !ok || sessionAuditStringField(item, "type") != "mcp_tool_call" {
		return nil
	}
	result, ok := item["result"].(map[string]any)
	if !ok {
		return nil
	}
	return sessionAuditMCPContentTexts(result["content"])
}

func sessionAuditCodexMCPResultTexts(root map[string]any) []string {
	if sessionAuditStringField(root, "type") != "event_msg" {
		return nil
	}
	payload, ok := root["payload"].(map[string]any)
	if !ok || sessionAuditStringField(payload, "type") != "mcp_tool_call_end" {
		return nil
	}
	result, ok := payload["result"].(map[string]any)
	if !ok {
		return nil
	}
	okResult, ok := result["Ok"].(map[string]any)
	if !ok {
		return nil
	}
	return sessionAuditMCPContentTexts(okResult["content"])
}

func sessionAuditCodexFunctionOutputTexts(root map[string]any) []string {
	if sessionAuditStringField(root, "type") != "response_item" {
		return nil
	}
	payload, ok := root["payload"].(map[string]any)
	if !ok || sessionAuditStringField(payload, "type") != "function_call_output" {
		return nil
	}
	output := sessionAuditStringField(payload, "output")
	if output == "" {
		return nil
	}
	return []string{output}
}

func sessionAuditMCPContentTexts(value any) []string {
	content, ok := value.([]any)
	if !ok {
		return nil
	}

	var texts []string
	for _, item := range content {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if sessionAuditStringField(entry, "type") != "text" {
			continue
		}
		if text := sessionAuditStringField(entry, "text"); text != "" {
			texts = append(texts, text)
		}
	}
	return texts
}

func summarizeSessionGraphAudit(
	session sessionGraphAuditSessionFile,
	events []sessionAuditToolEvent,
	expectation sessionAuditExpectation,
) sessionGraphAuditSessionFile {
	session.ExpectedPatternRef = expectation.PatternRef
	session.ExpectedRouteStrategy = expectation.RouteStrategy
	session.ToolCalls = sessionAuditToolEventCount(events)
	session.EditToolCalls = sessionAuditEditToolCallCount(events)
	session.FirstEditOrdinal = sessionAuditFirstEditOrdinal(events)
	session.FirstSubstantiveOrdinal = sessionAuditFirstSubstantiveOrdinal(events)
	session.ObservedPatternRefs = sessionAuditObservedPatternRefs(events)
	session.ObservedSupportLevels = sessionAuditObservedSupportLevels(events)
	session.ObservedRouteMatchStrategies = sessionAuditObservedRouteMatchStrategies(events)
	session.MethodCloseRecorded = sessionAuditMethodCloseRecorded(events)
	session.GraphPlanInfluenceCloseoutHint = sessionAuditGraphPlanInfluenceCloseoutHint(events)
	session.CompactPatternUseSeen = sessionAuditHasPatternUseMode(events, "compact")
	session.RetrievedUncompiledSeen = sessionAuditHasObservedValue(events, sessionAuditSupportLevelRE, "retrieved_uncompiled")
	session.FullPatternUseSeen = sessionAuditHasPatternUseMode(events, "full")
	session.SourceCardSeen = sessionAuditHasToolResultMarker(events, "source_card")

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
	if session.FirstSubstantiveOrdinal > 0 {
		beforeSubstantive := sessionAuditBeforeOrdinal(events, session.FirstSubstantiveOrdinal)
		session.PatternUseBeforeSubstantive = sessionAuditHasPatternUseAction(beforeSubstantive)
		session.PatternUseBypass = !session.PatternUseBeforeSubstantive
		session.FullBeforePatternApplication = sessionAuditFullPatternUseWithSourceCardBefore(events, session.FirstSubstantiveOrdinal)
		session.ProgressiveDisclosureBypass = sessionAuditProgressiveDisclosureBypass(session)
		session.SubstantiveActionsBeforePatternUse = sessionAuditSubstantiveActionsBeforePatternUse(events)
	}
	session.ScenarioPass, session.FailureReason = sessionAuditScenarioResult(session, expectation)
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
	if session.FirstSubstantiveOrdinal > 0 {
		summary.SessionsWithSubstantiveMoves++
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
	if session.PatternUseBeforeSubstantive {
		summary.PatternUseBeforeSubstantive++
	}
	if session.PatternUseBypass {
		summary.PatternUseBypasses++
	}
	if len(session.ObservedPatternRefs) > 0 ||
		len(session.ObservedSupportLevels) > 0 ||
		len(session.ObservedRouteMatchStrategies) > 0 {
		summary.PatternUseObserved++
	}
	if session.ProgressiveDisclosureBypass {
		summary.ProgressiveDisclosureBypasses++
	}
	switch session.ScenarioPass {
	case "pass":
		summary.ScenarioPass++
	case "fail":
		summary.ScenarioFail++
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

	if summary.PatternUseBypasses > 0 {
		diagnostics = append(diagnostics, sessionGraphAuditDiagnostic{
			Level:      "high",
			Code:       "pattern_use_bypass",
			Count:      summary.PatternUseBypasses,
			Message:    "session(s) made substantive Haft reasoning/work-shaping moves before calling haft_query(action=\"pattern_use\")",
			NextAction: "Add the PatternUse Gateway to the relevant carrier path and replay the listed sessions.",
			Examples:   sessionAuditPatternUseBypassExamples(report.Sessions, 3),
		})
	}

	if summary.ProgressiveDisclosureBypasses > 0 {
		diagnostics = append(diagnostics, sessionGraphAuditDiagnostic{
			Level:      "high",
			Code:       "progressive_disclosure_bypass",
			Count:      summary.ProgressiveDisclosureBypasses,
			Message:    "session(s) used compact retrieved_uncompiled PatternUse output for substantive application before a full recommendation with source_card",
			NextAction: "After compact retrieved_uncompiled, call haft_query(action=\"pattern_use\", mode=\"full\", ...) and inspect the source_card before applying the candidate.",
			Examples:   sessionAuditProgressiveDisclosureBypassExamples(report.Sessions, 3),
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

func sessionAuditPatternUseBypassExamples(
	sessions []sessionGraphAuditSessionFile,
	limit int,
) []string {
	var examples []string
	for _, session := range sessions {
		if !session.PatternUseBypass {
			continue
		}
		examples = append(examples, session.Path)
		if len(examples) >= limit {
			return examples
		}
	}
	return examples
}

func sessionAuditProgressiveDisclosureBypassExamples(
	sessions []sessionGraphAuditSessionFile,
	limit int,
) []string {
	var examples []string
	for _, session := range sessions {
		if !session.ProgressiveDisclosureBypass {
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
		"summary: files=%d sessions_with_edits=%d substantive_moves=%d pass=%d needs_review=%d fail=%d no_edit=%d graph_before_edit=%d rich_graph_before_edit=%d method_pull_before_edit=%d pattern_use_before_substantive=%d pattern_use_bypass=%d pattern_use_observed=%d progressive_disclosure_bypass=%d scenario_pass=%d scenario_fail=%d closeout_influence_hints=%d\n",
		report.Summary.FilesScanned,
		report.Summary.SessionsWithEdits,
		report.Summary.SessionsWithSubstantiveMoves,
		report.Summary.Pass,
		report.Summary.NeedsReview,
		report.Summary.Fail,
		report.Summary.NoEdit,
		report.Summary.GraphBeforeFirstEdit,
		report.Summary.RichGraphBeforeFirstEdit,
		report.Summary.MethodPullBeforeFirstEdit,
		report.Summary.PatternUseBeforeSubstantive,
		report.Summary.PatternUseBypasses,
		report.Summary.PatternUseObserved,
		report.Summary.ProgressiveDisclosureBypasses,
		report.Summary.ScenarioPass,
		report.Summary.ScenarioFail,
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
				"- %s verdict=%s first_edit=%d first_substantive=%d status=%t method_pull=%t graph=%t rich_graph=%t pattern_use=%t bypass=%t compact_pattern_use=%t retrieved_uncompiled=%t full_pattern_use=%t source_card=%t full_before_application=%t progressive_bypass=%t expected_pattern=%s expected_strategy=%s observed_patterns=%s observed_support=%s observed_strategy=%s scenario_pass=%s failure_reason=%s close=%t influence_hint=%t actions=%s substantive_before_pattern=%s\n",
				session.Path,
				session.Verdict,
				session.FirstEditOrdinal,
				session.FirstSubstantiveOrdinal,
				session.StatusBeforeFirstEdit,
				session.MethodPullBeforeFirstEdit,
				session.GraphBeforeFirstEdit,
				session.RichGraphBeforeFirstEdit,
				session.PatternUseBeforeSubstantive,
				session.PatternUseBypass,
				session.CompactPatternUseSeen,
				session.RetrievedUncompiledSeen,
				session.FullPatternUseSeen,
				session.SourceCardSeen,
				session.FullBeforePatternApplication,
				session.ProgressiveDisclosureBypass,
				session.ExpectedPatternRef,
				session.ExpectedRouteStrategy,
				strings.Join(session.ObservedPatternRefs, ","),
				strings.Join(session.ObservedSupportLevels, ","),
				strings.Join(session.ObservedRouteMatchStrategies, ","),
				session.ScenarioPass,
				session.FailureReason,
				session.MethodCloseRecorded,
				session.GraphPlanInfluenceCloseoutHint,
				strings.Join(session.GraphActionsBeforeFirstEdit, ","),
				strings.Join(session.SubstantiveActionsBeforePatternUse, ","),
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

func sessionAuditToolEventCount(events []sessionAuditToolEvent) int {
	count := 0
	for _, event := range events {
		if !sessionAuditIsToolCallEvent(event) {
			continue
		}
		count++
	}
	return count
}

func sessionAuditFirstSubstantiveOrdinal(events []sessionAuditToolEvent) int {
	for _, event := range events {
		if sessionAuditIsSubstantivePatternUseBoundary(event) {
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

func sessionAuditHasPatternUseAction(events []sessionAuditToolEvent) bool {
	return sessionAuditHasHaftQueryAction(events, "pattern_use")
}

func sessionAuditHasPatternUseMode(events []sessionAuditToolEvent, mode string) bool {
	for _, event := range events {
		if !sessionAuditIsHaftQuery(event) || event.Action != "pattern_use" {
			continue
		}
		if !sessionAuditPayloadHasJSONField(event.Payload, "mode", mode) {
			continue
		}
		return true
	}
	return false
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

func sessionAuditSubstantiveActionsBeforePatternUse(events []sessionAuditToolEvent) []string {
	firstPatternUse := sessionAuditFirstPatternUseOrdinal(events)
	limit := firstPatternUse
	if limit == 0 {
		limit = len(events) + 1
	}
	return sessionAuditSubstantiveActions(sessionAuditBeforeOrdinal(events, limit))
}

func sessionAuditFirstPatternUseOrdinal(events []sessionAuditToolEvent) int {
	for _, event := range events {
		if sessionAuditIsHaftQuery(event) && event.Action == "pattern_use" {
			return event.Ordinal
		}
	}
	return 0
}

func sessionAuditSubstantiveActions(events []sessionAuditToolEvent) []string {
	var actions []string
	for _, event := range events {
		if !sessionAuditIsSubstantivePatternUseBoundary(event) {
			continue
		}
		actions = append(actions, sessionAuditSubstantiveActionName(event))
	}
	return sessionAuditDedupeStrings(actions)
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

func sessionAuditIsSubstantivePatternUseBoundary(event sessionAuditToolEvent) bool {
	if sessionAuditIsSubstantiveAgentText(event) {
		return true
	}
	if sessionAuditIsHaftProblem(event) {
		switch event.Action {
		case "frame", "characterize":
			return true
		}
	}
	if sessionAuditIsHaftSolution(event) {
		switch event.Action {
		case "explore", "compare":
			return true
		}
	}
	if sessionAuditIsHaftDecision(event) {
		return true
	}
	if sessionAuditIsHaftMethod(event) && event.Action == "pull" {
		return true
	}
	return false
}

func sessionAuditSubstantiveActionName(event sessionAuditToolEvent) string {
	if sessionAuditIsSubstantiveAgentText(event) {
		return "agent_message.substantive_reasoning"
	}
	tool := strings.ToLower(event.Tool)
	for _, marker := range []string{"haft_problem", "haft_solution", "haft_decision", "haft_method"} {
		if strings.Contains(tool, marker) {
			return marker + "." + event.Action
		}
	}
	return event.Tool + "." + event.Action
}

func sessionAuditIsSubstantiveAgentText(event sessionAuditToolEvent) bool {
	if event.Tool != "agent_message" {
		return false
	}

	text := strings.TrimSpace(event.Payload)
	if len([]rune(text)) < 180 {
		return false
	}
	if sessionAuditTextLooksMechanicalListing(text) {
		return false
	}
	lowered := strings.ToLower(text)
	if !sessionAuditTextHasSubstantiveKeyword(lowered) {
		return false
	}
	return sessionAuditTextLooksStructuredAnswer(text)
}

func sessionAuditTextLooksMechanicalListing(text string) bool {
	lowered := strings.ToLower(text)
	if !strings.Contains(lowered, "```") {
		return false
	}
	if !strings.Contains(lowered, "files") && !strings.Contains(lowered, "файл") {
		return false
	}

	nonEmptyLines := 0
	pathLikeLines := 0
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "```") {
			continue
		}
		nonEmptyLines += 1
		if sessionAuditTextLineLooksPathLike(trimmed) {
			pathLikeLines += 1
		}
	}
	if pathLikeLines < 5 {
		return false
	}
	return pathLikeLines*2 >= nonEmptyLines
}

func sessionAuditTextLineLooksPathLike(line string) bool {
	if strings.HasPrefix(line, ".") {
		return true
	}
	if strings.Contains(line, "/") {
		return true
	}
	for _, suffix := range []string{".md", ".go", ".yaml", ".yml", ".toml", ".json", ".txt"} {
		if strings.HasSuffix(line, suffix) {
			return true
		}
	}
	return false
}

func sessionAuditTextHasSubstantiveKeyword(text string) bool {
	for _, marker := range []string{
		"architecture",
		"architectural",
		"selected structure",
		"boundary",
		"adr",
		"namecard",
		"candidate name",
		"debug",
		"diagnos",
		"hypothesis",
		"evidence",
		"proof",
		"public api",
		"what next",
		"next action",
		"test strategy",
		"архитектур",
		"структур",
		"границ",
		"назв",
		"имя",
		"диагност",
		"гипотез",
		"ули",
		"доказ",
		"план",
		"следующ",
		"вариант",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func sessionAuditTextLooksStructuredAnswer(text string) bool {
	for _, marker := range []string{"\n-", "\n1.", "\n2.", "\n##", "\n**", ":\n"} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	lowered := strings.ToLower(text)
	for _, marker := range []string{
		"entityofconcern:",
		"candidate name:",
		"usage sentence:",
		"почему это имя:",
		"ближайший отвергнутый вариант:",
	} {
		if strings.Contains(lowered, marker) {
			return true
		}
	}
	return false
}

func sessionAuditScenarioResult(
	session sessionGraphAuditSessionFile,
	expectation sessionAuditExpectation,
) (string, string) {
	pattern := strings.TrimSpace(expectation.PatternRef)
	strategy := strings.TrimSpace(expectation.RouteStrategy)
	if pattern == "" && strategy == "" {
		return "", ""
	}
	if strings.EqualFold(pattern, "none") {
		if session.PatternUseBeforeSubstantive || len(session.ObservedPatternRefs) > 0 {
			return "fail", "expected_no_pattern_use_but_pattern_use_was_observed"
		}
		return "pass", "mechanical_control_no_pattern_use"
	}
	if !session.PatternUseBeforeSubstantive {
		return "fail", "expected_pattern_use_before_substantive_but_none_was_detected"
	}
	if !sessionAuditContainsExpectedValue(session.ObservedPatternRefs, pattern) {
		return "fail", "expected_pattern_not_observed"
	}
	if strategy != "" &&
		!sessionAuditContainsExpectedValue(session.ObservedSupportLevels, strategy) &&
		!sessionAuditContainsExpectedValue(session.ObservedRouteMatchStrategies, strategy) {
		return "fail", "expected_support_or_strategy_not_observed"
	}
	return "pass", "matched_expected_pattern_and_support"
}

func sessionAuditContainsExpectedValue(values []string, expected string) bool {
	if expected == "" || strings.EqualFold(expected, "none") {
		return true
	}
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), strings.ToLower(expected)) {
			return true
		}
	}
	return false
}

func sessionAuditObservedPatternRefs(events []sessionAuditToolEvent) []string {
	return sessionAuditObservedValues(events, sessionAuditRecommendedPatternRE)
}

func sessionAuditObservedSupportLevels(events []sessionAuditToolEvent) []string {
	return sessionAuditObservedValues(events, sessionAuditSupportLevelRE)
}

func sessionAuditObservedRouteMatchStrategies(events []sessionAuditToolEvent) []string {
	return sessionAuditObservedValues(events, sessionAuditRouteStrategyRE)
}

func sessionAuditObservedValues(
	events []sessionAuditToolEvent,
	re *regexp.Regexp,
) []string {
	var values []string
	for _, event := range events {
		if event.Tool != "tool_result" {
			continue
		}
		text := sessionAuditNormalizeObservedPayload(event.Payload)
		matches := re.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}
			values = append(values, match[1])
		}
	}
	return sessionAuditDedupeStrings(values)
}

func sessionAuditHasObservedValue(
	events []sessionAuditToolEvent,
	re *regexp.Regexp,
	expected string,
) bool {
	return sessionAuditContainsExpectedValue(sessionAuditObservedValues(events, re), expected)
}

func sessionAuditHasToolResultMarker(events []sessionAuditToolEvent, marker string) bool {
	normalizedMarker := strings.ToLower(strings.TrimSpace(marker))
	if normalizedMarker == "" {
		return false
	}
	for _, event := range events {
		if event.Tool != "tool_result" {
			continue
		}
		text := strings.ToLower(sessionAuditNormalizeObservedPayload(event.Payload))
		if strings.Contains(text, normalizedMarker) {
			return true
		}
	}
	return false
}

func sessionAuditFullPatternUseWithSourceCardBefore(
	events []sessionAuditToolEvent,
	ordinal int,
) bool {
	if ordinal <= 0 {
		return false
	}
	before := sessionAuditBeforeOrdinal(events, ordinal)
	return sessionAuditHasPatternUseMode(before, "full") &&
		sessionAuditHasToolResultMarker(before, "source_card")
}

func sessionAuditProgressiveDisclosureBypass(session sessionGraphAuditSessionFile) bool {
	if session.FirstSubstantiveOrdinal == 0 {
		return false
	}
	if !session.CompactPatternUseSeen {
		return false
	}
	if !session.RetrievedUncompiledSeen {
		return false
	}
	return !session.FullBeforePatternApplication
}

func sessionAuditPayloadHasJSONField(payload string, field string, value string) bool {
	normalized := strings.ToLower(sessionAuditNormalizeObservedPayload(payload))
	field = strings.ToLower(strings.TrimSpace(field))
	value = strings.ToLower(strings.TrimSpace(value))
	if field == "" || value == "" {
		return false
	}
	quotedField := `"` + field + `"`
	quotedValue := `"` + value + `"`
	return strings.Contains(normalized, quotedField) &&
		strings.Contains(normalized, quotedValue)
}

func sessionAuditNormalizeObservedPayload(text string) string {
	text = strings.ReplaceAll(text, `\"`, `"`)
	text = strings.ReplaceAll(text, `\\n`, "\n")
	return text
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
		if session.FirstSubstantiveOrdinal > 0 && session.PatternUseBypass {
			return "fail", "Substantive non-edit reasoning happened before detectable PatternUse gateway use."
		}
		if session.FirstSubstantiveOrdinal > 0 && session.PatternUseBeforeSubstantive {
			return "pass", "Substantive non-edit reasoning was preceded by PatternUse gateway use."
		}
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

func sessionAuditIsToolCallEvent(event sessionAuditToolEvent) bool {
	if event.Tool == "agent_message" || event.Tool == "tool_result" {
		return false
	}
	return event.Tool != "" && event.Tool != "unknown"
}

func sessionAuditIsHaftQuery(event sessionAuditToolEvent) bool {
	return strings.Contains(strings.ToLower(event.Tool), "haft_query")
}

func sessionAuditIsHaftMethod(event sessionAuditToolEvent) bool {
	return strings.Contains(strings.ToLower(event.Tool), "haft_method")
}

func sessionAuditIsHaftProblem(event sessionAuditToolEvent) bool {
	return strings.Contains(strings.ToLower(event.Tool), "haft_problem")
}

func sessionAuditIsHaftSolution(event sessionAuditToolEvent) bool {
	return strings.Contains(strings.ToLower(event.Tool), "haft_solution")
}

func sessionAuditIsHaftDecision(event sessionAuditToolEvent) bool {
	return strings.Contains(strings.ToLower(event.Tool), "haft_decision")
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
	for _, key := range []string{"tool", "tool_name", "recipient_name"} {
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
		"pattern_use",
		"pull",
		"close",
		"frame",
		"characterize",
		"code_context",
		"impact",
		"node",
		"explore",
		"compare",
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
