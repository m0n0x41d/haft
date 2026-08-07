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

	sessionAuditUseNotApplicable      = "not_applicable"
	sessionAuditUseUsed               = "used"
	sessionAuditUseUnavailable        = "unavailable"
	sessionAuditUseIncorrectlySkipped = "incorrectly_skipped"
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
	Long: `Audit Codex or Claude session JSONL for conditional Haft context use.

The audit is read-only. It classifies code-graph and typed-memory orientation
separately as not_applicable, used, unavailable, or incorrectly_skipped. It
also checks MethodPack ordering for non-mechanical work, unauthorized
typed-memory persistence, MethodPack close, and closeout text that ties graph
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
	FilesScanned                               int `json:"files_scanned"`
	SessionsWithEdits                          int `json:"sessions_with_edits"`
	SessionsWithSubstantiveMoves               int `json:"sessions_with_substantive_moves"`
	EventsScanned                              int `json:"events_scanned"`
	ToolCalls                                  int `json:"tool_calls"`
	EditToolCalls                              int `json:"edit_tool_calls"`
	StatusBeforeFirstEdit                      int `json:"status_before_first_edit"`
	MethodPullBeforeFirstEdit                  int `json:"method_pull_before_first_edit"`
	GraphBeforeFirstEdit                       int `json:"graph_before_first_edit"`
	RichGraphBeforeFirstEdit                   int `json:"rich_graph_before_first_edit"`
	GraphResultBeforeFirstEdit                 int `json:"graph_result_before_first_edit"`
	GraphTruthBeforeFirstEdit                  int `json:"graph_truth_before_first_edit"`
	TypeScriptSessionsWithEdits                int `json:"typescript_sessions_with_edits"`
	TypeScriptGraphBeforeFirstEdit             int `json:"typescript_graph_before_first_edit"`
	BatchGraphBeforeFirstEdit                  int `json:"batch_graph_before_first_edit"`
	AnchorGraphBeforeFirstEdit                 int `json:"anchor_graph_before_first_edit"`
	DegradedGraphBeforeFirstEdit               int `json:"degraded_graph_before_first_edit"`
	MethodCloseRecorded                        int `json:"method_close_recorded"`
	GraphPlanInfluenceCloseoutHints            int `json:"graph_plan_influence_closeout_hints"`
	ContextHeavyMemoryUseDetected              int `json:"context_heavy_memory_use_detected"`
	TypedMemoryResolveBeforeFirstEdit          int `json:"typed_memory_resolve_before_first_edit"`
	TypedMemoryHydrationBeforeFirstEdit        int `json:"typed_memory_hydration_before_first_edit"`
	TypedMemoryBasisUnavailableBeforeFirstEdit int `json:"typed_memory_basis_unavailable_before_first_edit"`
	TypedMemoryAdmissionAttempted              int `json:"typed_memory_admission_attempted"`
	UnauthorizedTypedMemoryAdmission           int `json:"unauthorized_typed_memory_admission"`
	CodeGraphNotApplicable                     int `json:"code_graph_not_applicable"`
	CodeGraphUsed                              int `json:"code_graph_used"`
	CodeGraphUnavailable                       int `json:"code_graph_unavailable"`
	CodeGraphIncorrectlySkipped                int `json:"code_graph_incorrectly_skipped"`
	TypedMemoryNotApplicable                   int `json:"typed_memory_not_applicable"`
	TypedMemoryUsed                            int `json:"typed_memory_used"`
	TypedMemoryUnavailable                     int `json:"typed_memory_unavailable"`
	TypedMemoryIncorrectlySkipped              int `json:"typed_memory_incorrectly_skipped"`
	Pass                                       int `json:"pass"`
	NeedsReview                                int `json:"needs_review"`
	Fail                                       int `json:"fail"`
	NoEdit                                     int `json:"no_edit"`
	ParseErrors                                int `json:"parse_errors"`
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
	Path                                         string   `json:"path"`
	Format                                       string   `json:"format"`
	EventsScanned                                int      `json:"events_scanned"`
	ToolCalls                                    int      `json:"tool_calls"`
	EditToolCalls                                int      `json:"edit_tool_calls"`
	FirstEditOrdinal                             int      `json:"first_edit_ordinal,omitempty"`
	FirstSubstantiveOrdinal                      int      `json:"first_substantive_ordinal,omitempty"`
	StatusBeforeFirstEdit                        bool     `json:"status_before_first_edit"`
	MethodPullBeforeFirstEdit                    bool     `json:"method_pull_before_first_edit"`
	GraphBeforeFirstEdit                         bool     `json:"graph_before_first_edit"`
	RichGraphBeforeFirstEdit                     bool     `json:"rich_graph_before_first_edit"`
	GraphResultBeforeFirstEdit                   bool     `json:"graph_result_before_first_edit"`
	GraphTruthBeforeFirstEdit                    bool     `json:"graph_truth_before_first_edit"`
	TypeScriptEditDetected                       bool     `json:"typescript_edit_detected"`
	TypeScriptGraphBeforeFirstEdit               bool     `json:"typescript_graph_before_first_edit"`
	BatchGraphBeforeFirstEdit                    bool     `json:"batch_graph_before_first_edit"`
	AnchorGraphBeforeFirstEdit                   bool     `json:"anchor_graph_before_first_edit"`
	DegradedGraphBeforeFirstEdit                 bool     `json:"degraded_graph_before_first_edit"`
	IndexEpochObservedBeforeFirstEdit            bool     `json:"index_epoch_observed_before_first_edit"`
	ResolutionObservedBeforeFirstEdit            bool     `json:"resolution_observed_before_first_edit"`
	MethodCloseRecorded                          bool     `json:"method_close_recorded"`
	GraphPlanInfluenceCloseoutHint               bool     `json:"graph_plan_influence_closeout_hint"`
	ContextHeavyMemoryUseDetected                bool     `json:"context_heavy_memory_use_detected"`
	TypedMemoryResolveBeforeFirstEdit            bool     `json:"typed_memory_resolve_before_first_edit"`
	TypedMemoryHydrationAttemptedBeforeFirstEdit bool     `json:"typed_memory_hydration_attempted_before_first_edit"`
	TypedMemoryHydrationBeforeFirstEdit          bool     `json:"typed_memory_hydration_before_first_edit"`
	TypedMemoryBasisUnavailableBeforeFirstEdit   bool     `json:"typed_memory_basis_unavailable_before_first_edit"`
	TypedMemoryAdmissionAttempted                bool     `json:"typed_memory_admission_attempted"`
	UnauthorizedTypedMemoryAdmission             bool     `json:"unauthorized_typed_memory_admission"`
	CodeGraphOrientation                         string   `json:"code_graph_orientation"`
	TypedMemoryOrientation                       string   `json:"typed_memory_orientation"`
	GraphActionsBeforeFirstEdit                  []string `json:"graph_actions_before_first_edit,omitempty"`
	RichGraphActionsBeforeFirstEdit              []string `json:"rich_graph_actions_before_first_edit,omitempty"`
	ParseErrors                                  int      `json:"parse_errors,omitempty"`
	Verdict                                      string   `json:"verdict"`
	Rationale                                    string   `json:"rationale"`
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
		SchemaVersion:    4,
		Authority:        sessionAuditAuthority,
		MutationBoundary: append([]string(nil), sessionAuditMutationBoundary...),
		ScanPolicy: sessionGraphAuditScanPolicy{
			Input: filepath.Clean(inputPath),
			Files: append([]string(nil), files...),
			EventPolicy: []string{
				"tool_use_assistant_text_and_user_request_events_counted",
				"session_metadata_and_system_prompts_ignored",
				"plan_influence_requires_method_closeout_hint_or_human_review",
				"graph_tool_call_attempt_is_not_a_successful_graph_result",
				"typescript_vue_edits_require_a_typescript_targeted_graph_preflight",
				"epoch_resolution_and_degraded_markers_are_observability_not_authority",
				"code_graph_and_typed_memory_orientation_are_classified_separately",
				"mechanical_edits_are_explicitly_not_applicable_to_graph_preflight",
				"context_heavy_work_orients_typed_memory_when_available_without_becoming_a_global_gate",
				"typed_memory_admission_requires_an_explicit_named_receiving_use",
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

	agentTexts := sessionAuditAgentTexts(root)
	userTexts := sessionAuditUserTexts(root)
	events := make(
		[]sessionAuditToolEvent,
		0,
		len(agentTexts)+len(userTexts),
	)
	for _, text := range userTexts {
		events = append(events, sessionAuditToolEvent{
			Ordinal: startOrdinal + len(events),
			Tool:    "user_message",
			Payload: text,
		})
	}
	for _, text := range agentTexts {
		events = append(events, sessionAuditToolEvent{
			Ordinal: startOrdinal + len(events),
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

func sessionAuditUserTexts(root map[string]any) []string {
	var texts []string
	texts = append(texts, sessionAuditCodexExecUserTexts(root)...)
	texts = append(texts, sessionAuditCodexSessionUserTexts(root)...)
	texts = append(texts, sessionAuditClaudeUserTexts(root)...)
	return texts
}

func sessionAuditCodexExecUserTexts(root map[string]any) []string {
	if sessionAuditStringField(root, "type") != "item.completed" {
		return nil
	}
	item, ok := root["item"].(map[string]any)
	if !ok {
		return nil
	}
	if sessionAuditStringField(item, "type") != "user_message" {
		return nil
	}
	text := sessionAuditStringField(item, "text")
	if text == "" {
		return nil
	}
	return []string{text}
}

func sessionAuditCodexSessionUserTexts(root map[string]any) []string {
	if sessionAuditStringField(root, "type") == "event_msg" {
		payload, ok := root["payload"].(map[string]any)
		if !ok || sessionAuditStringField(payload, "type") != "user_message" {
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
	if !ok ||
		sessionAuditStringField(payload, "type") != "message" ||
		sessionAuditStringField(payload, "role") != "user" {
		return nil
	}
	return sessionAuditContentTexts(payload["content"])
}

func sessionAuditClaudeUserTexts(root map[string]any) []string {
	if sessionAuditStringField(root, "type") != "user" {
		return nil
	}
	message, ok := root["message"].(map[string]any)
	if !ok {
		return nil
	}
	return sessionAuditContentTexts(message["content"])
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
	if !ok ||
		sessionAuditStringField(payload, "type") != "message" ||
		sessionAuditStringField(payload, "role") == "user" {
		return nil
	}
	return sessionAuditContentTexts(payload["content"])
}

func sessionAuditClaudeAgentTexts(root map[string]any) []string {
	message, ok := root["message"].(map[string]any)
	if !ok || sessionAuditStringField(message, "role") == "user" {
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
		case "input_text", "output_text", "text":
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
) sessionGraphAuditSessionFile {
	session.ToolCalls = sessionAuditToolEventCount(events)
	session.EditToolCalls = sessionAuditEditToolCallCount(events)
	session.FirstEditOrdinal = sessionAuditFirstEditOrdinal(events)
	session.FirstSubstantiveOrdinal = sessionAuditFirstSubstantiveOrdinal(events)
	session.MethodCloseRecorded = sessionAuditMethodCloseRecorded(events)
	session.GraphPlanInfluenceCloseoutHint = sessionAuditGraphPlanInfluenceCloseoutHint(events)
	session.ContextHeavyMemoryUseDetected =
		sessionAuditHasContextHeavyMemoryUse(events)
	session.TypedMemoryAdmissionAttempted =
		sessionAuditTypedMemoryAdmissionAttempted(events)
	session.UnauthorizedTypedMemoryAdmission =
		sessionAuditUnauthorizedTypedMemoryAdmission(events)

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
	if session.GraphBeforeFirstEdit {
		session.GraphResultBeforeFirstEdit = sessionAuditHasGraphResult(beforeEdit)
		session.IndexEpochObservedBeforeFirstEdit = sessionAuditHasToolResultMarker(beforeEdit, "index epoch:")
		session.ResolutionObservedBeforeFirstEdit = sessionAuditHasToolResultMarker(beforeEdit, "resolution:")
		session.GraphTruthBeforeFirstEdit = session.IndexEpochObservedBeforeFirstEdit || session.ResolutionObservedBeforeFirstEdit
		session.DegradedGraphBeforeFirstEdit = sessionAuditHasGraphDegradedResult(beforeEdit)
	}
	session.TypeScriptEditDetected = sessionAuditHasTypeScriptEdit(events)
	session.TypeScriptGraphBeforeFirstEdit = sessionAuditHasTypeScriptGraphQuery(beforeEdit)
	session.BatchGraphBeforeFirstEdit = sessionAuditHasGraphJSONField(beforeEdit, "files")
	session.AnchorGraphBeforeFirstEdit = sessionAuditHasGraphJSONField(beforeEdit, "anchor_id")
	session.TypedMemoryResolveBeforeFirstEdit =
		sessionAuditHasTypedMemoryMode(beforeEdit, "resolve")
	session.TypedMemoryHydrationAttemptedBeforeFirstEdit =
		sessionAuditHasTypedMemoryMode(beforeEdit, "neighborhood")
	session.TypedMemoryHydrationBeforeFirstEdit =
		session.TypedMemoryHydrationAttemptedBeforeFirstEdit &&
			sessionAuditHasToolResultMarker(
				beforeEdit,
				`"result_kind":"exact_neighborhood"`,
			)
	session.TypedMemoryBasisUnavailableBeforeFirstEdit =
		sessionAuditHasTypedMemoryBasisUnavailable(beforeEdit)
	session.CodeGraphOrientation =
		sessionAuditCodeGraphOrientation(session, beforeEdit)
	session.TypedMemoryOrientation =
		sessionAuditTypedMemoryOrientation(session)
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
	if session.GraphResultBeforeFirstEdit {
		summary.GraphResultBeforeFirstEdit++
	}
	if session.GraphTruthBeforeFirstEdit {
		summary.GraphTruthBeforeFirstEdit++
	}
	if session.TypeScriptEditDetected {
		summary.TypeScriptSessionsWithEdits++
	}
	if session.TypeScriptGraphBeforeFirstEdit {
		summary.TypeScriptGraphBeforeFirstEdit++
	}
	if session.BatchGraphBeforeFirstEdit {
		summary.BatchGraphBeforeFirstEdit++
	}
	if session.AnchorGraphBeforeFirstEdit {
		summary.AnchorGraphBeforeFirstEdit++
	}
	if session.DegradedGraphBeforeFirstEdit {
		summary.DegradedGraphBeforeFirstEdit++
	}
	if session.MethodCloseRecorded {
		summary.MethodCloseRecorded++
	}
	if session.GraphPlanInfluenceCloseoutHint {
		summary.GraphPlanInfluenceCloseoutHints++
	}
	if session.ContextHeavyMemoryUseDetected {
		summary.ContextHeavyMemoryUseDetected++
	}
	if session.TypedMemoryResolveBeforeFirstEdit {
		summary.TypedMemoryResolveBeforeFirstEdit++
	}
	if session.TypedMemoryHydrationBeforeFirstEdit {
		summary.TypedMemoryHydrationBeforeFirstEdit++
	}
	if session.TypedMemoryBasisUnavailableBeforeFirstEdit {
		summary.TypedMemoryBasisUnavailableBeforeFirstEdit++
	}
	if session.TypedMemoryAdmissionAttempted {
		summary.TypedMemoryAdmissionAttempted++
	}
	if session.UnauthorizedTypedMemoryAdmission {
		summary.UnauthorizedTypedMemoryAdmission++
	}
	switch session.CodeGraphOrientation {
	case sessionAuditUseNotApplicable:
		summary.CodeGraphNotApplicable++
	case sessionAuditUseUsed:
		summary.CodeGraphUsed++
	case sessionAuditUseUnavailable:
		summary.CodeGraphUnavailable++
	case sessionAuditUseIncorrectlySkipped:
		summary.CodeGraphIncorrectlySkipped++
	}
	switch session.TypedMemoryOrientation {
	case sessionAuditUseNotApplicable:
		summary.TypedMemoryNotApplicable++
	case sessionAuditUseUsed:
		summary.TypedMemoryUsed++
	case sessionAuditUseUnavailable:
		summary.TypedMemoryUnavailable++
	case sessionAuditUseIncorrectlySkipped:
		summary.TypedMemoryIncorrectlySkipped++
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

	if summary.UnauthorizedTypedMemoryAdmission > 0 {
		diagnostics = append(diagnostics, sessionGraphAuditDiagnostic{
			Level:      "high",
			Code:       "unauthorized_typed_memory_persistence",
			Count:      summary.UnauthorizedTypedMemoryAdmission,
			Message:    "session(s) attempted typed-memory admission without a prior explicit named receiving use plus request provenance",
			NextAction: "Do not admit memory automatically; require an explicit operator save/record request or another named receiving use and preserve its provenance.",
		})
	}

	missingMemoryOrientation := summary.TypedMemoryIncorrectlySkipped
	if missingMemoryOrientation > 0 {
		diagnostics = append(diagnostics, sessionGraphAuditDiagnostic{
			Level:      "medium",
			Code:       "context_heavy_without_typed_memory_orientation",
			Count:      missingMemoryOrientation,
			Message:    "context-heavy, multi-session, or reliance-bearing session(s) lacked exact typed-memory hydration and lacked an explicit unavailable-basis result",
			NextAction: "Resolve the EntityOfConcern, hydrate the smallest exact neighborhood, and continue without a memory gate when the project basis is unavailable.",
		})
	}

	if summary.TypedMemoryBasisUnavailableBeforeFirstEdit > 0 {
		diagnostics = append(diagnostics, sessionGraphAuditDiagnostic{
			Level:      "low",
			Code:       "typed_memory_basis_unavailable_non_blocking",
			Count:      summary.TypedMemoryBasisUnavailableBeforeFirstEdit,
			Message:    "typed-memory orientation reported an unavailable project basis before edit",
			NextAction: "Keep the unavailable-basis result visible and continue otherwise-authorized Work; do not request profile admission merely to satisfy orientation.",
		})
	}

	missingGraph := summary.CodeGraphIncorrectlySkipped
	if missingGraph > 0 {
		diagnostics = append(diagnostics, sessionGraphAuditDiagnostic{
			Level:      "high",
			Code:       "edit_before_graph_preflight",
			Count:      missingGraph,
			Message:    "non-mechanical session(s) edited files before any usable code_context/impact/node/explore/callers/callees graph preflight",
			NextAction: "Inspect the governed code target before relying on graph facts; keep explicitly mechanical work classified as not_applicable.",
			Examples:   sessionAuditExamples(report.Sessions, "fail", 3),
		})
	}

	missingGraphResult := 0
	for _, session := range report.Sessions {
		if session.CodeGraphOrientation != sessionAuditUseUnavailable ||
			session.DegradedGraphBeforeFirstEdit {
			continue
		}
		missingGraphResult++
	}
	if missingGraphResult > 0 {
		diagnostics = append(diagnostics, sessionGraphAuditDiagnostic{
			Level:      "medium",
			Code:       "graph_preflight_result_not_observed",
			Count:      missingGraphResult,
			Message:    "session(s) called a graph action before editing, but no usable graph result was present in the transcript",
			NextAction: "Inspect transport failures or truncated transcripts; a tool-call attempt is not graph evidence.",
		})
	}

	missingTypeScriptGraph := 0
	for _, session := range report.Sessions {
		if !session.TypeScriptEditDetected ||
			session.TypeScriptGraphBeforeFirstEdit ||
			session.CodeGraphOrientation == sessionAuditUseNotApplicable {
			continue
		}
		missingTypeScriptGraph++
	}
	if missingTypeScriptGraph > 0 {
		diagnostics = append(diagnostics, sessionGraphAuditDiagnostic{
			Level:      "high",
			Code:       "typescript_edit_without_typescript_graph_preflight",
			Count:      missingTypeScriptGraph,
			Message:    "session(s) edited TypeScript/Vue paths without a TypeScript/Vue-targeted graph query before the first edit",
			NextAction: "Run code_context/impact/node/explore against the actual .ts/.tsx/.vue target before editing.",
		})
	}

	if summary.DegradedGraphBeforeFirstEdit > 0 {
		diagnostics = append(diagnostics, sessionGraphAuditDiagnostic{
			Level:      "high",
			Code:       "degraded_graph_used_before_edit",
			Count:      summary.DegradedGraphBeforeFirstEdit,
			Message:    "session(s) proceeded toward editing after the graph reported a degraded index",
			NextAction: "Repair or recover the index epoch before relying on graph absence or blast radius.",
		})
	}

	missingInfluence := 0
	for _, session := range report.Sessions {
		if session.CodeGraphOrientation != sessionAuditUseUsed ||
			session.GraphPlanInfluenceCloseoutHint {
			continue
		}
		missingInfluence++
	}
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
		"summary: files=%d sessions_with_edits=%d substantive_moves=%d pass=%d needs_review=%d fail=%d no_edit=%d graph_before_edit=%d rich_graph_before_edit=%d method_pull_before_edit=%d closeout_influence_hints=%d\n",
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
		report.Summary.GraphPlanInfluenceCloseoutHints,
	); err != nil {
		return err
	}
	if err := printf(
		"graph_v2: successful_result_before_edit=%d truth_markers_before_edit=%d typescript_edits=%d typescript_graph_before_edit=%d batch_graph_before_edit=%d anchor_graph_before_edit=%d degraded_graph_before_edit=%d\n",
		report.Summary.GraphResultBeforeFirstEdit,
		report.Summary.GraphTruthBeforeFirstEdit,
		report.Summary.TypeScriptSessionsWithEdits,
		report.Summary.TypeScriptGraphBeforeFirstEdit,
		report.Summary.BatchGraphBeforeFirstEdit,
		report.Summary.AnchorGraphBeforeFirstEdit,
		report.Summary.DegradedGraphBeforeFirstEdit,
	); err != nil {
		return err
	}
	if err := printf(
		"orientation_v1: code_graph={not_applicable:%d used:%d unavailable:%d incorrectly_skipped:%d} typed_memory={not_applicable:%d used:%d unavailable:%d incorrectly_skipped:%d}\n",
		report.Summary.CodeGraphNotApplicable,
		report.Summary.CodeGraphUsed,
		report.Summary.CodeGraphUnavailable,
		report.Summary.CodeGraphIncorrectlySkipped,
		report.Summary.TypedMemoryNotApplicable,
		report.Summary.TypedMemoryUsed,
		report.Summary.TypedMemoryUnavailable,
		report.Summary.TypedMemoryIncorrectlySkipped,
	); err != nil {
		return err
	}
	if err := printf(
		"memory_v2: context_heavy=%d resolve_before_edit=%d hydrated_before_edit=%d basis_unavailable_before_edit=%d admission_attempted=%d unauthorized_admission=%d\n",
		report.Summary.ContextHeavyMemoryUseDetected,
		report.Summary.TypedMemoryResolveBeforeFirstEdit,
		report.Summary.TypedMemoryHydrationBeforeFirstEdit,
		report.Summary.TypedMemoryBasisUnavailableBeforeFirstEdit,
		report.Summary.TypedMemoryAdmissionAttempted,
		report.Summary.UnauthorizedTypedMemoryAdmission,
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
				"- %s verdict=%s first_edit=%d first_substantive=%d status=%t method_pull=%t graph=%t graph_orientation=%s memory_orientation=%s rich_graph=%t close=%t influence_hint=%t actions=%s\n",
				session.Path,
				session.Verdict,
				session.FirstEditOrdinal,
				session.FirstSubstantiveOrdinal,
				session.StatusBeforeFirstEdit,
				session.MethodPullBeforeFirstEdit,
				session.GraphBeforeFirstEdit,
				session.CodeGraphOrientation,
				session.TypedMemoryOrientation,
				session.RichGraphBeforeFirstEdit,
				session.MethodCloseRecorded,
				session.GraphPlanInfluenceCloseoutHint,
				strings.Join(session.GraphActionsBeforeFirstEdit, ","),
			); err != nil {
				return err
			}
			if err := printf(
				"  graph_v2 result=%t truth=%t ts_edit=%t ts_graph=%t batch=%t anchor=%t degraded=%t epoch=%t resolution=%t\n",
				session.GraphResultBeforeFirstEdit,
				session.GraphTruthBeforeFirstEdit,
				session.TypeScriptEditDetected,
				session.TypeScriptGraphBeforeFirstEdit,
				session.BatchGraphBeforeFirstEdit,
				session.AnchorGraphBeforeFirstEdit,
				session.DegradedGraphBeforeFirstEdit,
				session.IndexEpochObservedBeforeFirstEdit,
				session.ResolutionObservedBeforeFirstEdit,
			); err != nil {
				return err
			}
			if err := printf(
				"  memory_v2 context_heavy=%t resolve=%t hydration_attempt=%t hydrated=%t basis_unavailable=%t admission=%t unauthorized_admission=%t\n",
				session.ContextHeavyMemoryUseDetected,
				session.TypedMemoryResolveBeforeFirstEdit,
				session.TypedMemoryHydrationAttemptedBeforeFirstEdit,
				session.TypedMemoryHydrationBeforeFirstEdit,
				session.TypedMemoryBasisUnavailableBeforeFirstEdit,
				session.TypedMemoryAdmissionAttempted,
				session.UnauthorizedTypedMemoryAdmission,
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
		if sessionAuditIsSubstantiveMove(event) {
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
		if sessionAuditEventHasHaftQueryAction(event, action) {
			return true
		}
	}
	return false
}

func sessionAuditEventHasHaftQueryAction(
	event sessionAuditToolEvent,
	action string,
) bool {
	if !sessionAuditIsHaftQuery(event) {
		return false
	}
	if event.Action == action {
		return true
	}
	return sessionAuditPayloadHasFieldValue(
		event.Payload,
		"action",
		action,
	)
}

func sessionAuditHasTypedMemoryMode(
	events []sessionAuditToolEvent,
	mode string,
) bool {
	for _, event := range events {
		if !sessionAuditEventHasHaftQueryAction(event, "memory") {
			continue
		}
		if sessionAuditPayloadHasFieldValue(event.Payload, "mode", mode) {
			return true
		}
	}
	return false
}

func sessionAuditHasTypedMemoryBasisUnavailable(
	events []sessionAuditToolEvent,
) bool {
	if !sessionAuditHasTypedMemoryMode(events, "resolve") &&
		!sessionAuditHasTypedMemoryMode(events, "neighborhood") &&
		!sessionAuditHasTypedMemoryMode(events, "recall") {
		return false
	}
	for _, marker := range []string{
		"project_basis_unavailable",
		"project_typeenv_unavailable",
		"typeenv_head_unavailable",
		`"result_kind":"known_absent"`,
		`"result_kind":"abstained"`,
	} {
		if sessionAuditHasToolResultMarker(events, marker) {
			return true
		}
	}
	return false
}

func sessionAuditHasContextHeavyMemoryUse(
	events []sessionAuditToolEvent,
) bool {
	for _, event := range events {
		if event.Tool != "user_message" &&
			event.Tool != "agent_message" {
			continue
		}
		text := strings.ToLower(strings.TrimSpace(event.Payload))
		for _, marker := range []string{
			"context-heavy",
			"multi-session",
			"prior session",
			"previous session",
			"earlier session",
			"other session",
			"reliance-bearing",
			"cross-session",
			"предыдущ",
			"прошл",
			"той сесс",
			"другой сесс",
			"межсессион",
			"контекст из",
		} {
			if strings.Contains(text, marker) {
				return true
			}
		}
		if strings.Contains(text, "session") &&
			(strings.Contains(text, "previous") ||
				strings.Contains(text, "prior") ||
				strings.Contains(text, "earlier")) {
			return true
		}
	}
	return false
}

func sessionAuditTypedMemoryAdmissionAttempted(
	events []sessionAuditToolEvent,
) bool {
	for _, event := range events {
		if sessionAuditEventHasHaftMemoryAction(event, "admit") {
			return true
		}
	}
	return false
}

func sessionAuditUnauthorizedTypedMemoryAdmission(
	events []sessionAuditToolEvent,
) bool {
	for _, event := range events {
		if !sessionAuditEventHasHaftMemoryAction(event, "admit") {
			continue
		}
		if !sessionAuditHasExplicitMemoryReceivingUseBefore(
			events,
			event.Ordinal,
		) {
			return true
		}
		if !sessionAuditPayloadHasNonEmptyField(
			event.Payload,
			"request_provenance_ref",
		) {
			return true
		}
	}
	return false
}

func sessionAuditHasExplicitMemoryReceivingUseBefore(
	events []sessionAuditToolEvent,
	ordinal int,
) bool {
	for _, event := range events {
		if event.Ordinal >= ordinal || event.Tool != "user_message" {
			continue
		}
		if sessionAuditTextRequestsMemoryPersistence(event.Payload) {
			return true
		}
	}
	return false
}

func sessionAuditTextRequestsMemoryPersistence(text string) bool {
	normalized := strings.ToLower(strings.TrimSpace(text))
	for _, marker := range []string{
		"/h-note",
		"$h-note",
		"remember this",
		"save this to project memory",
		"record this in project memory",
		"persist this for the next session",
		"write this to project memory",
		"запомни",
		"запиши это",
		"сохрани это",
		"зафиксируй это",
	} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	memoryMarker := false
	for _, marker := range []string{
		"project memory",
		"haft memory",
		"next session",
		"проектную память",
		"память haft",
		"следующей сесс",
		"на будущее",
	} {
		memoryMarker = memoryMarker || strings.Contains(normalized, marker)
	}
	if !memoryMarker {
		return false
	}
	for _, verb := range []string{
		"save",
		"record",
		"persist",
		"write",
		"admit",
		"запиши",
		"сохрани",
		"зафиксируй",
		"добавь",
	} {
		if strings.Contains(normalized, verb) {
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

func sessionAuditMethodPullLooksMechanical(event sessionAuditToolEvent) bool {
	for _, field := range []string{"declared_task_kind", "change_intent"} {
		for _, value := range []string{"mechanical", "mechanical_edit", "formatting_only"} {
			if sessionAuditPayloadHasJSONField(event.Payload, field, value) {
				return true
			}
		}
	}
	return sessionAuditPayloadHasJSONField(event.Payload, "ceremony_request", "none")
}

func sessionAuditHasMechanicalMethodPull(
	events []sessionAuditToolEvent,
) bool {
	for _, event := range events {
		if !sessionAuditIsHaftMethod(event) || event.Action != "pull" {
			continue
		}
		if sessionAuditMethodPullLooksMechanical(event) {
			return true
		}
	}
	return false
}

func sessionAuditCodeGraphOrientation(
	session sessionGraphAuditSessionFile,
	beforeEdit []sessionAuditToolEvent,
) string {
	if session.FirstEditOrdinal == 0 ||
		sessionAuditHasMechanicalMethodPull(beforeEdit) {
		return sessionAuditUseNotApplicable
	}
	if session.DegradedGraphBeforeFirstEdit ||
		(session.GraphBeforeFirstEdit &&
			!session.GraphResultBeforeFirstEdit) {
		return sessionAuditUseUnavailable
	}
	if session.GraphResultBeforeFirstEdit {
		return sessionAuditUseUsed
	}
	return sessionAuditUseIncorrectlySkipped
}

func sessionAuditTypedMemoryOrientation(
	session sessionGraphAuditSessionFile,
) string {
	if !session.ContextHeavyMemoryUseDetected {
		return sessionAuditUseNotApplicable
	}
	if session.TypedMemoryHydrationBeforeFirstEdit {
		return sessionAuditUseUsed
	}
	if session.TypedMemoryBasisUnavailableBeforeFirstEdit {
		return sessionAuditUseUnavailable
	}
	return sessionAuditUseIncorrectlySkipped
}

func sessionAuditGraphActions(events []sessionAuditToolEvent) []string {
	var actions []string
	for _, event := range events {
		for _, action := range sessionAuditGraphActionNames() {
			if sessionAuditEventHasHaftQueryAction(event, action) {
				actions = append(actions, action)
			}
		}
	}
	return sessionAuditDedupeStrings(actions)
}

func sessionAuditRichGraphActions(events []sessionAuditToolEvent) []string {
	var actions []string
	for _, event := range events {
		for _, action := range sessionAuditRichGraphActionNames() {
			if sessionAuditEventHasHaftQueryAction(event, action) {
				actions = append(actions, action)
			}
		}
	}
	return sessionAuditDedupeStrings(actions)
}

func sessionAuditHasGraphResult(events []sessionAuditToolEvent) bool {
	for _, marker := range []string{
		"## code context",
		"## node",
		"## explore",
		"## callers",
		"## callees",
		"## impact",
		"index epoch:",
		"resolution:",
	} {
		if sessionAuditHasToolResultMarker(events, marker) {
			return true
		}
	}
	return false
}

func sessionAuditHasTypeScriptEdit(events []sessionAuditToolEvent) bool {
	for _, event := range events {
		if sessionAuditIsEditTool(event) && sessionAuditMentionsTypeScriptPath(event.Payload) {
			return true
		}
	}
	return false
}

func sessionAuditHasTypeScriptGraphQuery(events []sessionAuditToolEvent) bool {
	for _, event := range events {
		if !sessionAuditMentionsTypeScriptPath(event.Payload) {
			continue
		}
		for _, action := range sessionAuditGraphActionNames() {
			if sessionAuditEventHasHaftQueryAction(event, action) {
				return true
			}
		}
	}
	return false
}

func sessionAuditMentionsTypeScriptPath(payload string) bool {
	normalized := strings.ToLower(sessionAuditNormalizeObservedPayload(payload))
	for _, extension := range []string{".ts", ".tsx", ".mts", ".cts", ".vue"} {
		if strings.Contains(normalized, extension) {
			return true
		}
	}
	return false
}

func sessionAuditHasGraphJSONField(events []sessionAuditToolEvent, field string) bool {
	marker := `"` + strings.ToLower(strings.TrimSpace(field)) + `"`
	for _, event := range events {
		graphQuery := false
		for _, action := range sessionAuditGraphActionNames() {
			graphQuery = graphQuery ||
				sessionAuditEventHasHaftQueryAction(event, action)
		}
		if !graphQuery {
			continue
		}
		payload := strings.ToLower(sessionAuditNormalizeObservedPayload(event.Payload))
		if strings.Contains(payload, marker) {
			return true
		}
	}
	return false
}

func sessionAuditHasGraphDegradedResult(events []sessionAuditToolEvent) bool {
	for _, event := range events {
		if event.Tool != "tool_result" {
			continue
		}
		text := strings.ToLower(sessionAuditNormalizeObservedPayload(event.Payload))
		graphResult := false
		for _, marker := range []string{"## code context", "## node", "## explore", "## callers", "## callees", "## impact", "index epoch:", "resolution:"} {
			graphResult = graphResult || strings.Contains(text, marker)
		}
		if graphResult && strings.Contains(text, "degraded") {
			return true
		}
	}
	return false
}

func sessionAuditIsSubstantiveMove(event sessionAuditToolEvent) bool {
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
		return !sessionAuditMethodPullLooksMechanical(event)
	}
	return false
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

func sessionAuditPayloadHasFieldValue(
	payload string,
	field string,
	value string,
) bool {
	normalized := strings.ToLower(
		sessionAuditNormalizeObservedPayload(payload),
	)
	normalized = strings.NewReplacer(
		" ", "",
		"\t", "",
		"\n", "",
		"\r", "",
	).Replace(normalized)
	field = strings.ToLower(strings.TrimSpace(field))
	value = strings.ToLower(strings.TrimSpace(value))
	if field == "" || value == "" {
		return false
	}
	for _, marker := range []string{
		`"` + field + `":"` + value + `"`,
		field + `:"` + value + `"`,
		`"` + field + `:'` + value + `'`,
		field + `:'` + value + `'`,
	} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

func sessionAuditPayloadHasNonEmptyField(
	payload string,
	field string,
) bool {
	normalized := sessionAuditNormalizeObservedPayload(payload)
	normalized = strings.NewReplacer(
		" ", "",
		"\t", "",
		"\n", "",
		"\r", "",
	).Replace(normalized)
	field = strings.TrimSpace(field)
	if field == "" {
		return false
	}
	for _, prefix := range []string{
		`"` + field + `":"`,
		field + `:"`,
		`"` + field + `:'`,
		field + `:'`,
	} {
		index := strings.Index(normalized, prefix)
		if index < 0 {
			continue
		}
		valueStart := index + len(prefix)
		if valueStart < len(normalized) {
			return normalized[valueStart] != '"' &&
				normalized[valueStart] != '\''
		}
	}
	return false
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
	if session.UnauthorizedTypedMemoryAdmission {
		return "fail", "Typed-memory admission was attempted without a prior explicit named receiving use and request provenance."
	}
	if session.TypedMemoryOrientation == sessionAuditUseIncorrectlySkipped {
		return "needs_review", "Context-heavy or multi-session reliance was detected without exact typed-memory hydration or an explicit unavailable-basis result."
	}
	if session.FirstEditOrdinal == 0 {
		return "no_edit", "No edit tool call was detected, so graph-before-edit enforcement is not applicable."
	}
	if session.CodeGraphOrientation == sessionAuditUseNotApplicable {
		return "pass", "The MethodPack classified the edit as mechanical, so code-graph orientation was not applicable."
	}
	if session.TypeScriptEditDetected &&
		!session.TypeScriptGraphBeforeFirstEdit {
		return "fail", "A TypeScript/Vue edit happened without a target-specific TypeScript/Vue graph preflight."
	}
	if session.CodeGraphOrientation == sessionAuditUseUnavailable {
		if !session.DegradedGraphBeforeFirstEdit {
			return "needs_review", "A graph call happened before the edit, but the transcript contains no usable graph result."
		}
		return "needs_review", "The graph returned a degraded index before the edit; absence and blast-radius results are not reliable enough to pass."
	}
	if session.CodeGraphOrientation == sessionAuditUseIncorrectlySkipped {
		if session.TypeScriptEditDetected {
			return "fail", "A TypeScript/Vue edit happened without a target-specific TypeScript/Vue graph preflight."
		}
		return "fail", "A non-mechanical edit happened before detectable code-graph orientation."
	}
	if session.MethodPullBeforeFirstEdit &&
		session.GraphPlanInfluenceCloseoutHint {
		return "pass", "MethodPack pull, usable graph orientation, and closeout influence evidence were present."
	}
	return "needs_review", "Usable graph orientation happened before edit, but MethodPack pull or closeout influence evidence is missing."
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
	if event.Tool == "agent_message" ||
		event.Tool == "user_message" ||
		event.Tool == "tool_result" {
		return false
	}
	return event.Tool != "" && event.Tool != "unknown"
}

func sessionAuditIsHaftQuery(event sessionAuditToolEvent) bool {
	tool := strings.ToLower(event.Tool)
	if strings.Contains(tool, "haft_query") {
		return true
	}
	payload := strings.ToLower(
		sessionAuditNormalizeObservedPayload(event.Payload),
	)
	for _, marker := range []string{
		"mcp__haft__haft_query",
		"tools.haft_query",
		"tools.mcp__haft__haft_query",
	} {
		if strings.Contains(payload, marker) {
			return true
		}
	}
	return false
}

func sessionAuditIsHaftMethod(event sessionAuditToolEvent) bool {
	return strings.Contains(strings.ToLower(event.Tool), "haft_method")
}

func sessionAuditIsHaftMemory(event sessionAuditToolEvent) bool {
	tool := strings.ToLower(event.Tool)
	if strings.Contains(tool, "haft_memory") {
		return true
	}
	payload := strings.ToLower(
		sessionAuditNormalizeObservedPayload(event.Payload),
	)
	for _, marker := range []string{
		"mcp__haft__haft_memory",
		"tools.haft_memory",
		"tools.mcp__haft__haft_memory",
		`"name":"haft_memory"`,
	} {
		if strings.Contains(payload, marker) {
			return true
		}
	}
	return false
}

func sessionAuditEventHasHaftMemoryAction(
	event sessionAuditToolEvent,
	action string,
) bool {
	if !sessionAuditIsHaftMemory(event) {
		return false
	}
	if event.Action == action {
		return true
	}
	return sessionAuditPayloadHasFieldValue(
		event.Payload,
		"action",
		action,
	)
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

func sessionAuditGraphActionNames() []string {
	return []string{
		"code_context",
		"impact",
		"node",
		"explore",
		"callers",
		"callees",
	}
}

func sessionAuditRichGraphActionNames() []string {
	return []string{
		"impact",
		"node",
		"explore",
		"callers",
		"callees",
	}
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
		"fpf",
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
