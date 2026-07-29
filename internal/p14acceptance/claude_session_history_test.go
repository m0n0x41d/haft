package p14acceptance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	p14ClaudeSessionHistorySchema = "haft.p14.claude-session-history/v1"
	p14ClaudeSessionHistoryLimit  = int64(512 << 20)
	p14ClaudeToolResultTimeout    = 30 * time.Second
)

const (
	p14ClaudeStartupTools        = "tools_list_delta"
	p14ClaudeStartupInstructions = "initialize_instructions_delta"
	p14ClaudeCallStatusBefore    = "status_before"
	p14ClaudeCallBoundedRead     = "bounded_onboard_status"
	p14ClaudeCallStatusAfter     = "status_after"
)

type p14ClaudeSessionHistoryEvidence struct {
	Schema                 string                           `json:"schema"`
	SessionID              string                           `json:"session_id"`
	ClaudeVersion          string                           `json:"claude_version"`
	ProjectRoot            string                           `json:"project_root"`
	SourcePath             string                           `json:"source_path"`
	SourcePrefixBytes      int64                            `json:"source_prefix_bytes"`
	SourcePrefixDigest     string                           `json:"source_prefix_digest"`
	ObservedAt             string                           `json:"observed_at"`
	ProtocolEvidenceDigest string                           `json:"protocol_evidence_digest"`
	StartupBindings        []p14ClaudeStartupHistoryBinding `json:"startup_bindings"`
	CallBindings           []p14ClaudeToolHistoryBinding    `json:"call_bindings"`
	EvidenceDigest         string                           `json:"evidence_digest"`
}

type p14ClaudeStartupHistoryBinding struct {
	Kind          string `json:"kind"`
	Line          int    `json:"line"`
	LineDigest    string `json:"line_digest"`
	OccurredAt    string `json:"occurred_at"`
	PayloadDigest string `json:"payload_digest"`
}

type p14ClaudeToolHistoryBinding struct {
	Role                 string                          `json:"role"`
	ToolUseID            string                          `json:"tool_use_id"`
	Tool                 string                          `json:"tool"`
	ArgumentsCanonical   string                          `json:"arguments_canonical"`
	ArgumentsDigest      string                          `json:"arguments_digest"`
	ToolUseLine          int                             `json:"tool_use_line"`
	ToolUseLineDigest    string                          `json:"tool_use_line_digest"`
	ToolUseAt            string                          `json:"tool_use_at"`
	ToolResultLine       int                             `json:"tool_result_line"`
	ToolResultLineDigest string                          `json:"tool_result_line_digest"`
	ToolResultAt         string                          `json:"tool_result_at"`
	DurationMilliseconds int64                           `json:"duration_milliseconds"`
	ResponseDigest       string                          `json:"response_digest"`
	RuntimeIdentity      *p14ClaudeRuntimeStatusIdentity `json:"runtime_identity,omitempty"`
}

type p14ClaudeRuntimeStatusIdentity struct {
	PID            int    `json:"pid"`
	StartedAt      string `json:"started_at"`
	ExecutablePath string `json:"executable_path"`
}

type p14ClaudeStartupEvent struct {
	Kind          string
	Line          int
	LineDigest    string
	OccurredAt    time.Time
	SessionID     string
	ProjectRoot   string
	PayloadDigest string
}

type p14ClaudeToolUseEvent struct {
	Line               int
	ItemIndex          int
	LineDigest         string
	OccurredAt         time.Time
	SessionID          string
	ClaudeVersion      string
	ProjectRoot        string
	UUID               string
	ToolUseID          string
	Tool               string
	ArgumentsCanonical string
}

type p14ClaudeToolResultEvent struct {
	Line           int
	ItemIndex      int
	LineDigest     string
	OccurredAt     time.Time
	SessionID      string
	ProjectRoot    string
	ParentUUID     string
	ToolUseID      string
	IsError        bool
	Response       []byte
	ResponseDigest string
}

type p14ClaudeExpectedToolCall struct {
	Role               string
	Tool               string
	ArgumentsCanonical string
}

var p14ClaudeExpectedToolCalls = []p14ClaudeExpectedToolCall{
	{
		Role:               p14ClaudeCallStatusBefore,
		Tool:               "mcp__haft__haft_query",
		ArgumentsCanonical: `{"action":"status","full":false}`,
	},
	{
		Role:               p14ClaudeCallBoundedRead,
		Tool:               "mcp__haft__haft_onboard",
		ArgumentsCanonical: `{"action":"status"}`,
	},
	{
		Role:               p14ClaudeCallStatusAfter,
		Tool:               "mcp__haft__haft_query",
		ArgumentsCanonical: `{"action":"status","full":false}`,
	},
}

func captureP14ClaudeSessionHistory(
	prepared preparedRequestOracleInput,
	runtime p14RuntimeObservationBinding,
	protocol p14MCPProtocolDiscovery,
	observedAt time.Time,
) (p14ClaudeSessionHistoryEvidence, error) {
	root, err := p14CanonicalClaudeProjectsRoot()
	if err != nil {
		return p14ClaudeSessionHistoryEvidence{}, err
	}
	checkpointAt, err := time.Parse(
		time.RFC3339Nano,
		runtime.RestartCheckpointCreatedAt,
	)
	if err != nil {
		return p14ClaudeSessionHistoryEvidence{}, err
	}
	matches := make([]p14ClaudeSessionHistoryEvidence, 0, 1)
	err = filepath.WalkDir(
		root,
		func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			if filepath.Ext(entry.Name()) != ".jsonl" {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() ||
				info.ModTime().Before(checkpointAt) {
				return nil
			}
			if err := validateP14CanonicalClaudeSessionPath(
				root,
				path,
			); err != nil {
				return nil
			}
			raw, err := readP14ClaudeSessionHistoryPrefix(path, 0)
			if err != nil {
				return err
			}
			if !p14LooksLikeClaudeProofTranscript(raw) {
				return nil
			}
			evidence, err := deriveP14ClaudeSessionHistoryEvidence(
				path,
				raw,
				prepared,
				runtime,
				protocol,
				observedAt,
			)
			if err != nil {
				return nil
			}
			matches = append(matches, evidence)
			return nil
		},
	)
	if err != nil {
		return p14ClaudeSessionHistoryEvidence{}, err
	}
	if len(matches) != 1 {
		return p14ClaudeSessionHistoryEvidence{}, fmt.Errorf(
			"P14 canonical Claude transcript match count is %d",
			len(matches),
		)
	}
	return matches[0], nil
}

func p14CanonicalClaudeProjectsRoot() (string, error) {
	account, err := user.Current()
	if err != nil {
		return "", err
	}
	claudeRoot := filepath.Join(account.HomeDir, ".claude")
	root := filepath.Join(claudeRoot, "projects")
	for _, directory := range []string{claudeRoot, root} {
		info, err := os.Lstat(directory)
		if err != nil {
			return "", err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf(
				"P14 canonical Claude history root is not a real directory",
			)
		}
	}
	return filepath.Clean(root), nil
}

func validateP14CanonicalClaudeSessionPath(
	root string,
	path string,
) error {
	if err := validateP14CanonicalClaudeSessionShape(
		root,
		path,
	); err != nil {
		return err
	}
	cleanRoot := filepath.Clean(root)
	cleanPath := filepath.Clean(path)
	relative, err := filepath.Rel(cleanRoot, cleanPath)
	if err != nil {
		return err
	}
	parts := strings.Split(filepath.ToSlash(relative), "/")
	if len(parts) != 2 ||
		parts[0] == "" ||
		parts[1] == "" ||
		filepath.Ext(parts[1]) != ".jsonl" {
		return fmt.Errorf(
			"P14 Claude transcript is outside the canonical main-session shape",
		)
	}
	for _, candidate := range []string{
		filepath.Join(cleanRoot, parts[0]),
		cleanPath,
	} {
		info, err := os.Lstat(candidate)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf(
				"P14 Claude transcript path contains a symlink",
			)
		}
	}
	return nil
}

func validateP14CanonicalClaudeSessionShape(
	root string,
	path string,
) error {
	cleanRoot := filepath.Clean(root)
	cleanPath := filepath.Clean(path)
	relative, err := filepath.Rel(cleanRoot, cleanPath)
	if err != nil {
		return err
	}
	parts := strings.Split(filepath.ToSlash(relative), "/")
	if len(parts) != 2 ||
		parts[0] == "" ||
		parts[1] == "" ||
		filepath.Ext(parts[1]) != ".jsonl" {
		return fmt.Errorf(
			"P14 Claude transcript is outside the canonical main-session shape",
		)
	}
	return nil
}

func readP14ClaudeSessionHistoryPrefix(
	sourcePath string,
	prefixBytes int64,
) ([]byte, error) {
	info, err := os.Lstat(sourcePath)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() ||
		info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf(
			"P14 Claude transcript must be a regular non-symlink file",
		)
	}
	size := info.Size()
	if prefixBytes == 0 {
		prefixBytes = size
	}
	if prefixBytes <= 0 ||
		prefixBytes > size ||
		prefixBytes > p14ClaudeSessionHistoryLimit {
		return nil, fmt.Errorf(
			"P14 Claude transcript prefix size is invalid",
		)
	}
	file, err := os.Open(sourcePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, prefixBytes))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) != prefixBytes ||
		!bytes.HasSuffix(raw, []byte{'\n'}) {
		return nil, fmt.Errorf(
			"P14 Claude transcript prefix is truncated",
		)
	}
	return raw, nil
}

func p14LooksLikeClaudeProofTranscript(raw []byte) bool {
	required := [][]byte{
		[]byte(`"mcp_instructions_delta"`),
		[]byte(`"deferred_tools_delta"`),
		[]byte(`"mcp__haft__haft_query"`),
		[]byte(`"mcp__haft__haft_onboard"`),
	}
	for _, marker := range required {
		if !bytes.Contains(raw, marker) {
			return false
		}
	}
	return true
}

func deriveP14ClaudeSessionHistoryEvidence(
	sourcePath string,
	raw []byte,
	prepared preparedRequestOracleInput,
	runtime p14RuntimeObservationBinding,
	protocol p14MCPProtocolDiscovery,
	observedAt time.Time,
) (p14ClaudeSessionHistoryEvidence, error) {
	canonicalRoot, err := p14CanonicalClaudeProjectsRoot()
	if err != nil {
		return p14ClaudeSessionHistoryEvidence{}, err
	}
	if err := validateP14CanonicalClaudeSessionShape(
		canonicalRoot,
		sourcePath,
	); err != nil {
		return p14ClaudeSessionHistoryEvidence{}, err
	}
	if int64(len(raw)) <= 0 ||
		int64(len(raw)) > p14ClaudeSessionHistoryLimit ||
		!bytes.HasSuffix(raw, []byte{'\n'}) {
		return p14ClaudeSessionHistoryEvidence{}, fmt.Errorf(
			"P14 Claude transcript prefix differs",
		)
	}
	checkpointAt, err := time.Parse(
		time.RFC3339Nano,
		runtime.RestartCheckpointCreatedAt,
	)
	if err != nil {
		return p14ClaudeSessionHistoryEvidence{}, err
	}
	liveMCPStartedAt, err := time.Parse(
		time.RFC3339Nano,
		runtime.LiveMCPStartedAt,
	)
	if err != nil {
		return p14ClaudeSessionHistoryEvidence{}, err
	}
	liveMCPFulfilledAt, err := time.Parse(
		time.RFC3339Nano,
		runtime.LiveMCPFulfilledAt,
	)
	if err != nil {
		return p14ClaudeSessionHistoryEvidence{}, err
	}
	if observedAt.Before(liveMCPFulfilledAt) {
		return p14ClaudeSessionHistoryEvidence{}, fmt.Errorf(
			"P14 Claude transcript observation predates live MCP fulfillment",
		)
	}
	expectedInstructions, expectedTools, err :=
		p14ClaudeStartupExpectations(protocol)
	if err != nil {
		return p14ClaudeSessionHistoryEvidence{}, err
	}
	startups := make([]p14ClaudeStartupEvent, 0, 2)
	uses := make([]p14ClaudeToolUseEvent, 0, 3)
	results := make(map[string][]p14ClaudeToolResultEvent)
	lines := bytes.Split(raw, []byte{'\n'})
	for index, line := range lines {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		root, object, err := decodeP14ClaudeSessionLine(line)
		if err != nil {
			return p14ClaudeSessionHistoryEvidence{}, fmt.Errorf(
				"P14 Claude transcript line %d: %w",
				index+1,
				err,
			)
		}
		if !object {
			continue
		}
		lineNumber := index + 1
		lineDigest := p14Digest(line)
		timestamp := p14JSONText(root["timestamp"])
		if timestamp == "" {
			continue
		}
		lineAt, err := time.Parse(time.RFC3339Nano, timestamp)
		if err != nil {
			return p14ClaudeSessionHistoryEvidence{}, fmt.Errorf(
				"P14 Claude transcript line %d timestamp: %w",
				lineNumber,
				err,
			)
		}
		if lineAt.Before(checkpointAt) || lineAt.After(observedAt) {
			continue
		}
		startup, present, err := p14ClaudeStartupFromLine(
			root,
			lineNumber,
			lineDigest,
			expectedInstructions,
			expectedTools,
		)
		if err != nil {
			return p14ClaudeSessionHistoryEvidence{}, err
		}
		if present {
			startups = append(startups, startup)
		}
		lineUses, err := p14ClaudeToolUsesFromLine(
			root,
			lineNumber,
			lineDigest,
		)
		if err != nil {
			return p14ClaudeSessionHistoryEvidence{}, err
		}
		for _, use := range lineUses {
			uses = append(uses, use)
		}
		lineResults, err := p14ClaudeToolResultsFromLine(
			root,
			lineNumber,
			lineDigest,
		)
		if err != nil {
			return p14ClaudeSessionHistoryEvidence{}, err
		}
		for _, result := range lineResults {
			results[result.ToolUseID] = append(
				results[result.ToolUseID],
				result,
			)
		}
	}
	if len(startups) != 2 {
		return p14ClaudeSessionHistoryEvidence{}, fmt.Errorf(
			"P14 Claude transcript startup proof count is %d",
			len(startups),
		)
	}
	slices.SortFunc(
		startups,
		func(left p14ClaudeStartupEvent, right p14ClaudeStartupEvent) int {
			return left.Line - right.Line
		},
	)
	if !p14ClaudeStartupKindsComplete(startups) {
		return p14ClaudeSessionHistoryEvidence{}, fmt.Errorf(
			"P14 Claude transcript startup proof differs",
		)
	}
	slices.SortFunc(
		uses,
		func(left p14ClaudeToolUseEvent, right p14ClaudeToolUseEvent) int {
			if left.Line != right.Line {
				return left.Line - right.Line
			}
			return left.ItemIndex - right.ItemIndex
		},
	)
	if len(uses) != len(p14ClaudeExpectedToolCalls) {
		return p14ClaudeSessionHistoryEvidence{}, fmt.Errorf(
			"P14 Claude transcript Haft tool count is %d",
			len(uses),
		)
	}
	sessionID := ""
	claudeVersion := ""
	projectRoot, err := p14ClaudeCanonicalProjectRoot(
		prepared.FrozenBasis.SelectedProject.ProjectRoot,
	)
	if err != nil {
		return p14ClaudeSessionHistoryEvidence{}, err
	}
	callBindings := make(
		[]p14ClaudeToolHistoryBinding,
		0,
		len(uses),
	)
	var previousResultAt time.Time
	for index, use := range uses {
		expected := p14ClaudeExpectedToolCalls[index]
		if use.Tool != expected.Tool ||
			use.ArgumentsCanonical != expected.ArgumentsCanonical {
			return p14ClaudeSessionHistoryEvidence{}, fmt.Errorf(
				"P14 Claude transcript call %d differs",
				index+1,
			)
		}
		if sessionID == "" {
			sessionID = use.SessionID
			claudeVersion = use.ClaudeVersion
		}
		if use.SessionID != sessionID ||
			use.ClaudeVersion != claudeVersion ||
			use.ProjectRoot != projectRoot ||
			(!previousResultAt.IsZero() &&
				use.OccurredAt.Before(previousResultAt)) {
			return p14ClaudeSessionHistoryEvidence{}, fmt.Errorf(
				"P14 Claude transcript session or chronology differs",
			)
		}
		matches := results[use.ToolUseID]
		if len(matches) != 1 {
			return p14ClaudeSessionHistoryEvidence{}, fmt.Errorf(
				"P14 Claude transcript result count for %q is %d",
				use.ToolUseID,
				len(matches),
			)
		}
		result := matches[0]
		duration := result.OccurredAt.Sub(use.OccurredAt)
		if result.SessionID != sessionID ||
			result.ProjectRoot != projectRoot ||
			result.ParentUUID != use.UUID ||
			result.IsError ||
			duration < 0 ||
			duration > p14ClaudeToolResultTimeout ||
			result.OccurredAt.After(observedAt) {
			return p14ClaudeSessionHistoryEvidence{}, fmt.Errorf(
				"P14 Claude transcript result for %q differs",
				use.ToolUseID,
			)
		}
		var runtimeIdentity *p14ClaudeRuntimeStatusIdentity
		if expected.Role == p14ClaudeCallStatusBefore ||
			expected.Role == p14ClaudeCallStatusAfter {
			parsed, err := parseP14ClaudeRuntimeStatusIdentity(
				result.Response,
			)
			if err != nil {
				return p14ClaudeSessionHistoryEvidence{}, fmt.Errorf(
					"P14 Claude transcript status runtime for %q differs: %w",
					use.ToolUseID,
					err,
				)
			}
			if err := validateP14ClaudeRuntimeStatusIdentity(
				runtime,
				parsed,
			); err != nil {
				return p14ClaudeSessionHistoryEvidence{}, err
			}
			runtimeIdentity = &parsed
		}
		previousResultAt = result.OccurredAt
		callBindings = append(callBindings, p14ClaudeToolHistoryBinding{
			Role:                 expected.Role,
			ToolUseID:            use.ToolUseID,
			Tool:                 use.Tool,
			ArgumentsCanonical:   use.ArgumentsCanonical,
			ArgumentsDigest:      p14Digest([]byte(use.ArgumentsCanonical)),
			ToolUseLine:          use.Line,
			ToolUseLineDigest:    use.LineDigest,
			ToolUseAt:            use.OccurredAt.Format(time.RFC3339Nano),
			ToolResultLine:       result.Line,
			ToolResultLineDigest: result.LineDigest,
			ToolResultAt:         result.OccurredAt.Format(time.RFC3339Nano),
			DurationMilliseconds: duration.Milliseconds(),
			ResponseDigest:       result.ResponseDigest,
			RuntimeIdentity:      runtimeIdentity,
		})
	}
	first := callBindings[0]
	firstUseAt, err := time.Parse(time.RFC3339Nano, first.ToolUseAt)
	if err != nil {
		return p14ClaudeSessionHistoryEvidence{}, err
	}
	firstResultAt, err := time.Parse(time.RFC3339Nano, first.ToolResultAt)
	if err != nil {
		return p14ClaudeSessionHistoryEvidence{}, err
	}
	if firstUseAt.Before(liveMCPStartedAt) ||
		liveMCPFulfilledAt.Before(firstUseAt) ||
		liveMCPFulfilledAt.After(firstResultAt) {
		return p14ClaudeSessionHistoryEvidence{}, fmt.Errorf(
			"P14 Claude first status does not bracket the live MCP challenge receipt",
		)
	}
	for _, startup := range startups {
		if startup.OccurredAt.Before(liveMCPStartedAt) ||
			startup.OccurredAt.After(firstUseAt) ||
			startup.SessionID != sessionID ||
			startup.ProjectRoot != projectRoot {
			return p14ClaudeSessionHistoryEvidence{}, fmt.Errorf(
				"P14 Claude startup proof is stale or follows tool use",
			)
		}
	}
	expectedSourceName := sessionID + ".jsonl"
	if filepath.Base(sourcePath) != expectedSourceName ||
		sessionID == "" ||
		claudeVersion == "" {
		return p14ClaudeSessionHistoryEvidence{}, fmt.Errorf(
			"P14 Claude transcript identity differs",
		)
	}
	startupBindings := make(
		[]p14ClaudeStartupHistoryBinding,
		0,
		len(startups),
	)
	for _, startup := range startups {
		startupBindings = append(
			startupBindings,
			p14ClaudeStartupHistoryBinding{
				Kind:          startup.Kind,
				Line:          startup.Line,
				LineDigest:    startup.LineDigest,
				OccurredAt:    startup.OccurredAt.Format(time.RFC3339Nano),
				PayloadDigest: startup.PayloadDigest,
			},
		)
	}
	evidence := p14ClaudeSessionHistoryEvidence{
		Schema:                 p14ClaudeSessionHistorySchema,
		SessionID:              sessionID,
		ClaudeVersion:          claudeVersion,
		ProjectRoot:            projectRoot,
		SourcePath:             filepath.Clean(sourcePath),
		SourcePrefixBytes:      int64(len(raw)),
		SourcePrefixDigest:     p14Digest(raw),
		ObservedAt:             observedAt.Format(time.RFC3339Nano),
		ProtocolEvidenceDigest: protocol.EvidenceDigest,
		StartupBindings:        startupBindings,
		CallBindings:           callBindings,
	}
	basis, err := p14ClaudeSessionHistoryDigestBasis(evidence)
	if err != nil {
		return p14ClaudeSessionHistoryEvidence{}, err
	}
	evidence.EvidenceDigest = p14Digest(basis)
	if err := validateP14ClaudeSessionHistoryEvidence(
		prepared,
		runtime,
		protocol,
		evidence,
	); err != nil {
		return p14ClaudeSessionHistoryEvidence{}, err
	}
	return evidence, nil
}

func decodeP14ClaudeSessionLine(
	line []byte,
) (map[string]any, bool, error) {
	decoder := json.NewDecoder(bytes.NewReader(line))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, false, err
	}
	root, ok := value.(map[string]any)
	return root, ok, nil
}

func p14ClaudeStartupFromLine(
	root map[string]any,
	line int,
	lineDigest string,
	expectedInstructions string,
	expectedTools []string,
) (p14ClaudeStartupEvent, bool, error) {
	if p14JSONText(root["type"]) != "attachment" {
		return p14ClaudeStartupEvent{}, false, nil
	}
	attachment := p14JSONMap(root["attachment"])
	kind := p14JSONText(attachment["type"])
	if kind != "mcp_instructions_delta" &&
		kind != "deferred_tools_delta" {
		return p14ClaudeStartupEvent{}, false, nil
	}
	occurredAt, err := p14ClaudeRelevantLineBasis(root)
	if err != nil {
		return p14ClaudeStartupEvent{}, false, err
	}
	switch kind {
	case "mcp_instructions_delta":
		blocks := p14ClaudeJSONStrings(attachment["addedBlocks"])
		haftBlocks := make([]string, 0, 1)
		for _, block := range blocks {
			if strings.HasPrefix(block, "## haft\n") {
				haftBlocks = append(haftBlocks, block)
			}
		}
		if len(haftBlocks) == 0 {
			return p14ClaudeStartupEvent{}, false, nil
		}
		added := p14ClaudeJSONStrings(attachment["addedNames"])
		removed := p14ClaudeJSONStrings(attachment["removedNames"])
		if len(haftBlocks) != 1 ||
			!slices.Contains(added, "haft") ||
			slices.Contains(removed, "haft") ||
			haftBlocks[0] != "## haft\n"+expectedInstructions {
			return p14ClaudeStartupEvent{}, false, fmt.Errorf(
				"P14 Claude initialize instructions delta differs",
			)
		}
		payload, err := marshalP14CanonicalJSON(map[string]any{
			"name":  "haft",
			"block": haftBlocks[0],
		})
		if err != nil {
			return p14ClaudeStartupEvent{}, false, err
		}
		projectRoot, err := p14ClaudeCanonicalProjectRoot(
			p14JSONText(root["cwd"]),
		)
		if err != nil {
			return p14ClaudeStartupEvent{}, false, err
		}
		return p14ClaudeStartupEvent{
			Kind:          p14ClaudeStartupInstructions,
			Line:          line,
			LineDigest:    lineDigest,
			OccurredAt:    occurredAt,
			SessionID:     p14JSONText(root["sessionId"]),
			ProjectRoot:   projectRoot,
			PayloadDigest: p14Digest(payload),
		}, true, nil
	default:
		added := p14ClaudeHaftToolNames(
			p14ClaudeJSONStrings(attachment["addedNames"]),
		)
		if len(added) == 0 {
			return p14ClaudeStartupEvent{}, false, nil
		}
		removed := p14ClaudeHaftToolNames(
			p14ClaudeJSONStrings(attachment["removedNames"]),
		)
		readded := p14ClaudeHaftToolNames(
			p14ClaudeJSONStrings(attachment["readdedNames"]),
		)
		pending := p14ClaudeJSONStrings(attachment["pendingMcpServers"])
		slices.Sort(added)
		expected := slices.Clone(expectedTools)
		slices.Sort(expected)
		if !slices.Equal(added, expected) ||
			len(removed) != 0 ||
			len(readded) != 0 ||
			slices.Contains(pending, "haft") {
			return p14ClaudeStartupEvent{}, false, fmt.Errorf(
				"P14 Claude tools/list delta differs",
			)
		}
		payload, err := marshalP14CanonicalJSON(added)
		if err != nil {
			return p14ClaudeStartupEvent{}, false, err
		}
		projectRoot, err := p14ClaudeCanonicalProjectRoot(
			p14JSONText(root["cwd"]),
		)
		if err != nil {
			return p14ClaudeStartupEvent{}, false, err
		}
		return p14ClaudeStartupEvent{
			Kind:          p14ClaudeStartupTools,
			Line:          line,
			LineDigest:    lineDigest,
			OccurredAt:    occurredAt,
			SessionID:     p14JSONText(root["sessionId"]),
			ProjectRoot:   projectRoot,
			PayloadDigest: p14Digest(payload),
		}, true, nil
	}
}

func p14ClaudeToolUsesFromLine(
	root map[string]any,
	line int,
	lineDigest string,
) ([]p14ClaudeToolUseEvent, error) {
	if p14JSONText(root["type"]) != "assistant" {
		return nil, nil
	}
	message := p14JSONMap(root["message"])
	sidechain, sidechainPresent := root["isSidechain"].(bool)
	if !sidechainPresent ||
		sidechain ||
		p14JSONText(root["entrypoint"]) != "cli" ||
		p14JSONText(message["role"]) != "assistant" {
		return nil, fmt.Errorf(
			"P14 Claude proof tool use is outside the canonical main session",
		)
	}
	content := p14JSONArray(message["content"])
	result := make([]p14ClaudeToolUseEvent, 0)
	for index, raw := range content {
		item := p14JSONMap(raw)
		if p14JSONText(item["type"]) != "tool_use" {
			continue
		}
		tool := p14JSONText(item["name"])
		if !strings.HasPrefix(tool, "mcp__haft__") {
			continue
		}
		occurredAt, err := p14ClaudeRelevantLineBasis(root)
		if err != nil {
			return nil, err
		}
		caller := p14JSONMap(item["caller"])
		if p14JSONText(caller["type"]) != "direct" {
			return nil, fmt.Errorf(
				"P14 Claude proof call is not direct",
			)
		}
		args, err := marshalP14CanonicalJSON(p14JSONMap(item["input"]))
		if err != nil {
			return nil, err
		}
		projectRoot, err := p14ClaudeCanonicalProjectRoot(
			p14JSONText(root["cwd"]),
		)
		if err != nil {
			return nil, err
		}
		uuid := p14JSONText(root["uuid"])
		toolUseID := p14JSONText(item["id"])
		version := p14JSONText(root["version"])
		if uuid == "" || toolUseID == "" || version == "" {
			return nil, fmt.Errorf(
				"P14 Claude proof tool identity differs",
			)
		}
		result = append(result, p14ClaudeToolUseEvent{
			Line:               line,
			ItemIndex:          index,
			LineDigest:         lineDigest,
			OccurredAt:         occurredAt,
			SessionID:          p14JSONText(root["sessionId"]),
			ClaudeVersion:      version,
			ProjectRoot:        projectRoot,
			UUID:               uuid,
			ToolUseID:          toolUseID,
			Tool:               tool,
			ArgumentsCanonical: string(args),
		})
	}
	return result, nil
}

func p14ClaudeToolResultsFromLine(
	root map[string]any,
	line int,
	lineDigest string,
) ([]p14ClaudeToolResultEvent, error) {
	if p14JSONText(root["type"]) != "user" {
		return nil, nil
	}
	message := p14JSONMap(root["message"])
	if p14JSONText(message["role"]) != "user" {
		return nil, fmt.Errorf(
			"P14 Claude proof tool result is outside a user message",
		)
	}
	content := p14JSONArray(message["content"])
	result := make([]p14ClaudeToolResultEvent, 0)
	for index, raw := range content {
		item := p14JSONMap(raw)
		if p14JSONText(item["type"]) != "tool_result" {
			continue
		}
		toolUseID := p14JSONText(item["tool_use_id"])
		if toolUseID == "" {
			continue
		}
		response, err := p14ClaudeToolResultText(item["content"])
		if err != nil {
			return nil, err
		}
		occurredAt, err := p14ClaudeRelevantLineBasis(root)
		if err != nil {
			return nil, err
		}
		isError := false
		if rawError, present := item["is_error"]; present {
			value, ok := rawError.(bool)
			if !ok {
				return nil, fmt.Errorf(
					"P14 Claude tool result error flag differs",
				)
			}
			isError = value
		}
		projectRoot, err := p14ClaudeCanonicalProjectRoot(
			p14JSONText(root["cwd"]),
		)
		if err != nil {
			return nil, err
		}
		result = append(result, p14ClaudeToolResultEvent{
			Line:           line,
			ItemIndex:      index,
			LineDigest:     lineDigest,
			OccurredAt:     occurredAt,
			SessionID:      p14JSONText(root["sessionId"]),
			ProjectRoot:    projectRoot,
			ParentUUID:     p14JSONText(root["parentUuid"]),
			ToolUseID:      toolUseID,
			IsError:        isError,
			Response:       response,
			ResponseDigest: p14Digest(response),
		})
	}
	return result, nil
}

func p14ClaudeRelevantLineBasis(
	root map[string]any,
) (time.Time, error) {
	sessionID := p14JSONText(root["sessionId"])
	projectRoot := p14JSONText(root["cwd"])
	timestamp := p14JSONText(root["timestamp"])
	if sessionID == "" || projectRoot == "" || timestamp == "" {
		return time.Time{}, fmt.Errorf(
			"P14 Claude relevant transcript line lacks identity",
		)
	}
	occurredAt, err := time.Parse(time.RFC3339Nano, timestamp)
	if err != nil {
		return time.Time{}, err
	}
	return occurredAt.UTC(), nil
}

func p14ClaudeCanonicalProjectRoot(raw string) (string, error) {
	if !filepath.IsAbs(raw) {
		return "", fmt.Errorf(
			"P14 Claude transcript project root is not absolute",
		)
	}
	resolved, err := filepath.EvalSymlinks(raw)
	if err != nil {
		return "", err
	}
	return filepath.Clean(resolved), nil
}

func p14ClaudeToolResultText(raw any) ([]byte, error) {
	switch value := raw.(type) {
	case string:
		text := []byte(strings.TrimSpace(value))
		if len(text) == 0 {
			return nil, fmt.Errorf("P14 Claude tool result is empty")
		}
		return text, nil
	case []any:
		texts := make([]string, 0, len(value))
		for _, entry := range value {
			item := p14JSONMap(entry)
			if p14JSONText(item["type"]) != "text" {
				return nil, fmt.Errorf(
					"P14 Claude tool result has non-text content",
				)
			}
			texts = append(texts, p14JSONText(item["text"]))
		}
		text := []byte(strings.TrimSpace(strings.Join(texts, "\n")))
		if len(text) == 0 {
			return nil, fmt.Errorf("P14 Claude tool result is empty")
		}
		return text, nil
	default:
		return nil, fmt.Errorf("P14 Claude tool result content differs")
	}
}

func parseP14ClaudeRuntimeStatusIdentity(
	raw []byte,
) (p14ClaudeRuntimeStatusIdentity, error) {
	const prefix = "- `haft serve`: pid="
	matches := make([]string, 0, 1)
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, prefix) {
			matches = append(matches, line)
		}
	}
	if len(matches) != 1 {
		return p14ClaudeRuntimeStatusIdentity{}, fmt.Errorf(
			"runtime status line count is %d",
			len(matches),
		)
	}
	remainder := strings.TrimPrefix(matches[0], prefix)
	startedMarker := " started="
	executableMarker := " executable=`"
	mtimeMarker := "` executable_mtime="
	startedIndex := strings.Index(remainder, startedMarker)
	executableIndex := strings.Index(remainder, executableMarker)
	mtimeIndex := strings.Index(remainder, mtimeMarker)
	if startedIndex <= 0 ||
		executableIndex <= startedIndex+len(startedMarker) ||
		mtimeIndex <= executableIndex+len(executableMarker) {
		return p14ClaudeRuntimeStatusIdentity{}, fmt.Errorf(
			"runtime status line shape differs",
		)
	}
	pid, err := strconv.Atoi(remainder[:startedIndex])
	if err != nil || pid <= 1 {
		return p14ClaudeRuntimeStatusIdentity{}, fmt.Errorf(
			"runtime status PID differs",
		)
	}
	startedAt := remainder[startedIndex+len(startedMarker) : executableIndex]
	if _, err := time.Parse(time.RFC3339Nano, startedAt); err != nil {
		return p14ClaudeRuntimeStatusIdentity{}, fmt.Errorf(
			"runtime status start time differs",
		)
	}
	executable := remainder[executableIndex+len(executableMarker) : mtimeIndex]
	if !filepath.IsAbs(executable) {
		return p14ClaudeRuntimeStatusIdentity{}, fmt.Errorf(
			"runtime status executable is not absolute",
		)
	}
	physical, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return p14ClaudeRuntimeStatusIdentity{}, err
	}
	return p14ClaudeRuntimeStatusIdentity{
		PID:            pid,
		StartedAt:      startedAt,
		ExecutablePath: filepath.Clean(physical),
	}, nil
}

func validateP14ClaudeRuntimeStatusIdentity(
	runtime p14RuntimeObservationBinding,
	identity p14ClaudeRuntimeStatusIdentity,
) error {
	expectedStartedAt, err := time.Parse(
		time.RFC3339Nano,
		runtime.LiveMCPStartedAt,
	)
	if err != nil {
		return err
	}
	actualStartedAt, err := time.Parse(
		time.RFC3339Nano,
		identity.StartedAt,
	)
	if err != nil {
		return err
	}
	expectedExecutable, err := filepath.EvalSymlinks(
		runtime.LiveMCPExecutablePath,
	)
	if err != nil {
		return err
	}
	if identity.PID != runtime.LiveMCPPID ||
		!actualStartedAt.Equal(expectedStartedAt) ||
		identity.ExecutablePath != filepath.Clean(expectedExecutable) {
		return fmt.Errorf(
			"P14 Claude status runtime identity differs from the live MCP receipt",
		)
	}
	return nil
}

func p14ClaudeStartupExpectations(
	protocol p14MCPProtocolDiscovery,
) (string, []string, error) {
	if len(protocol.Exchanges) != 2 {
		return "", nil, fmt.Errorf(
			"P14 Claude protocol proof exchange count differs",
		)
	}
	_, initializeResponse, err := validateP14MCPProtocolExchange(
		protocol.Exchanges[0],
		"initialize",
	)
	if err != nil {
		return "", nil, err
	}
	_, initializeResult, err := decodeP14MCPResponse(
		initializeResponse,
		"p14-initialize",
	)
	if err != nil {
		return "", nil, err
	}
	instructions := p14JSONText(initializeResult["instructions"])
	if instructions == "" {
		return "", nil, fmt.Errorf(
			"P14 Claude protocol proof has no initialize instructions",
		)
	}
	_, toolsResponse, err := validateP14MCPProtocolExchange(
		protocol.Exchanges[1],
		"tools/list",
	)
	if err != nil {
		return "", nil, err
	}
	_, toolsResult, err := decodeP14MCPResponse(
		toolsResponse,
		"p14-tools-list",
	)
	if err != nil {
		return "", nil, err
	}
	tools := make([]string, 0, len(p14ExpectedMCPToolOrder))
	for _, raw := range p14JSONArray(toolsResult["tools"]) {
		name := p14JSONText(p14JSONMap(raw)["name"])
		if name == "" {
			return "", nil, fmt.Errorf(
				"P14 Claude protocol proof has an unnamed tool",
			)
		}
		tools = append(tools, "mcp__haft__"+name)
	}
	return instructions, tools, nil
}

func p14ClaudeHaftToolNames(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if strings.HasPrefix(value, "mcp__haft__") {
			result = append(result, value)
		}
	}
	return result
}

func p14ClaudeJSONStrings(raw any) []string {
	values := p14JSONArray(raw)
	result := make([]string, 0, len(values))
	for _, value := range values {
		text, ok := value.(string)
		if ok {
			result = append(result, text)
		}
	}
	return result
}

func p14ClaudeStartupKindsComplete(
	startups []p14ClaudeStartupEvent,
) bool {
	kinds := make([]string, 0, len(startups))
	for _, startup := range startups {
		kinds = append(kinds, startup.Kind)
	}
	slices.Sort(kinds)
	return slices.Equal(kinds, []string{
		p14ClaudeStartupInstructions,
		p14ClaudeStartupTools,
	})
}

func validateP14ClaudeSessionHistoryEvidence(
	prepared preparedRequestOracleInput,
	runtime p14RuntimeObservationBinding,
	protocol p14MCPProtocolDiscovery,
	evidence p14ClaudeSessionHistoryEvidence,
) error {
	root, err := p14CanonicalClaudeProjectsRoot()
	if err != nil {
		return err
	}
	if err := validateP14CanonicalClaudeSessionShape(
		root,
		evidence.SourcePath,
	); err != nil {
		return err
	}
	projectRoot, err := p14ClaudeCanonicalProjectRoot(
		prepared.FrozenBasis.SelectedProject.ProjectRoot,
	)
	if err != nil {
		return err
	}
	if evidence.Schema != p14ClaudeSessionHistorySchema ||
		evidence.SessionID == "" ||
		evidence.ClaudeVersion == "" ||
		evidence.ProjectRoot != projectRoot ||
		filepath.Base(evidence.SourcePath) != evidence.SessionID+".jsonl" ||
		evidence.SourcePrefixBytes <= 0 ||
		evidence.SourcePrefixBytes > p14ClaudeSessionHistoryLimit ||
		!validP14Digest(evidence.SourcePrefixDigest) ||
		evidence.ProtocolEvidenceDigest != protocol.EvidenceDigest ||
		len(evidence.StartupBindings) != 2 ||
		len(evidence.CallBindings) != len(p14ClaudeExpectedToolCalls) ||
		!validP14Digest(evidence.EvidenceDigest) {
		return fmt.Errorf(
			"P14 Claude session history evidence basis differs",
		)
	}
	observedAt, err := time.Parse(time.RFC3339Nano, evidence.ObservedAt)
	if err != nil {
		return err
	}
	checkpointAt, err := time.Parse(
		time.RFC3339Nano,
		runtime.RestartCheckpointCreatedAt,
	)
	if err != nil || observedAt.Before(checkpointAt) {
		return fmt.Errorf(
			"P14 Claude session history observation is stale",
		)
	}
	startupKinds := make([]string, 0, 2)
	seenLines := make(map[int]struct{})
	for _, startup := range evidence.StartupBindings {
		occurredAt, err := time.Parse(time.RFC3339Nano, startup.OccurredAt)
		if err != nil ||
			occurredAt.Before(checkpointAt) ||
			occurredAt.After(observedAt) ||
			startup.Line <= 0 ||
			!validP14Digest(startup.LineDigest) ||
			!validP14Digest(startup.PayloadDigest) {
			return fmt.Errorf(
				"P14 Claude startup binding differs",
			)
		}
		if _, duplicate := seenLines[startup.Line]; duplicate {
			return fmt.Errorf(
				"P14 Claude transcript line is reused",
			)
		}
		seenLines[startup.Line] = struct{}{}
		startupKinds = append(startupKinds, startup.Kind)
	}
	slices.Sort(startupKinds)
	if !slices.Equal(startupKinds, []string{
		p14ClaudeStartupInstructions,
		p14ClaudeStartupTools,
	}) {
		return fmt.Errorf(
			"P14 Claude startup binding kinds differ",
		)
	}
	var previousResultAt time.Time
	for index, call := range evidence.CallBindings {
		expected := p14ClaudeExpectedToolCalls[index]
		runtimeIdentityExpected :=
			expected.Role == p14ClaudeCallStatusBefore ||
				expected.Role == p14ClaudeCallStatusAfter
		useAt, useErr := time.Parse(time.RFC3339Nano, call.ToolUseAt)
		resultAt, resultErr := time.Parse(
			time.RFC3339Nano,
			call.ToolResultAt,
		)
		if useErr != nil ||
			resultErr != nil ||
			call.Role != expected.Role ||
			call.ToolUseID == "" ||
			call.Tool != expected.Tool ||
			call.ArgumentsCanonical != expected.ArgumentsCanonical ||
			call.ArgumentsDigest !=
				p14Digest([]byte(call.ArgumentsCanonical)) ||
			call.ToolUseLine <= 0 ||
			call.ToolResultLine <= call.ToolUseLine ||
			!validP14Digest(call.ToolUseLineDigest) ||
			!validP14Digest(call.ToolResultLineDigest) ||
			!validP14Digest(call.ResponseDigest) ||
			(call.RuntimeIdentity != nil) != runtimeIdentityExpected ||
			call.DurationMilliseconds < 0 ||
			call.DurationMilliseconds >
				p14ClaudeToolResultTimeout.Milliseconds() ||
			call.DurationMilliseconds !=
				resultAt.Sub(useAt).Milliseconds() ||
			resultAt.Before(useAt) ||
			resultAt.After(observedAt) ||
			(!previousResultAt.IsZero() &&
				useAt.Before(previousResultAt)) {
			return fmt.Errorf(
				"P14 Claude call binding %d differs",
				index+1,
			)
		}
		if runtimeIdentityExpected {
			if err := validateP14ClaudeRuntimeStatusIdentity(
				runtime,
				*call.RuntimeIdentity,
			); err != nil {
				return err
			}
		}
		for _, line := range []int{
			call.ToolUseLine,
			call.ToolResultLine,
		} {
			if _, duplicate := seenLines[line]; duplicate {
				return fmt.Errorf(
					"P14 Claude transcript line is reused",
				)
			}
			seenLines[line] = struct{}{}
		}
		previousResultAt = resultAt
	}
	first := evidence.CallBindings[0]
	firstUseAt, err := time.Parse(time.RFC3339Nano, first.ToolUseAt)
	if err != nil {
		return err
	}
	firstResultAt, err := time.Parse(time.RFC3339Nano, first.ToolResultAt)
	if err != nil {
		return err
	}
	fulfilledAt, err := time.Parse(
		time.RFC3339Nano,
		runtime.LiveMCPFulfilledAt,
	)
	if err != nil ||
		fulfilledAt.Before(firstUseAt) ||
		fulfilledAt.After(firstResultAt) {
		return fmt.Errorf(
			"P14 Claude status bracket omits the live MCP receipt",
		)
	}
	basis, err := p14ClaudeSessionHistoryDigestBasis(evidence)
	if err != nil {
		return err
	}
	if p14Digest(basis) != evidence.EvidenceDigest {
		return fmt.Errorf(
			"P14 Claude session history evidence digest differs",
		)
	}
	return nil
}

func verifyP14ClaudeSessionHistorySource(
	prepared preparedRequestOracleInput,
	runtime p14RuntimeObservationBinding,
	protocol p14MCPProtocolDiscovery,
	evidence p14ClaudeSessionHistoryEvidence,
) error {
	if err := validateP14ClaudeSessionHistoryEvidence(
		prepared,
		runtime,
		protocol,
		evidence,
	); err != nil {
		return err
	}
	raw, err := readP14ClaudeSessionHistoryPrefix(
		evidence.SourcePath,
		evidence.SourcePrefixBytes,
	)
	if err != nil {
		return err
	}
	if p14Digest(raw) != evidence.SourcePrefixDigest {
		return fmt.Errorf(
			"P14 Claude transcript prefix changed",
		)
	}
	observedAt, err := time.Parse(
		time.RFC3339Nano,
		evidence.ObservedAt,
	)
	if err != nil {
		return err
	}
	reobserved, err := deriveP14ClaudeSessionHistoryEvidence(
		evidence.SourcePath,
		raw,
		prepared,
		runtime,
		protocol,
		observedAt,
	)
	if err != nil {
		return err
	}
	actual, err := marshalP14CanonicalJSON(evidence)
	if err != nil {
		return err
	}
	expected, err := marshalP14CanonicalJSON(reobserved)
	if err != nil {
		return err
	}
	if !bytes.Equal(actual, expected) {
		return fmt.Errorf(
			"P14 Claude transcript re-observation differs",
		)
	}
	return nil
}

func p14ClaudeSessionHistoryDigestBasis(
	evidence p14ClaudeSessionHistoryEvidence,
) ([]byte, error) {
	basis := evidence
	basis.EvidenceDigest = ""
	return marshalP14CanonicalJSON(basis)
}

func TestP14ClaudeSessionHistoryRejectsForgeryExtraAndHungCalls(
	t *testing.T,
) {
	prepared, runtime, protocol := syntheticP14ClaudeSessionBasis(t)
	observedAt := time.Date(2026, 7, 28, 10, 2, 0, 0, time.UTC)
	sourcePath := syntheticP14ClaudeSessionPath(t)
	raw := syntheticP14ClaudeSessionJSONL(
		t,
		prepared,
		runtime,
		protocol,
		"normal",
	)
	evidence, err := deriveP14ClaudeSessionHistoryEvidence(
		sourcePath,
		raw,
		prepared,
		runtime,
		protocol,
		observedAt,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(evidence.CallBindings) != 3 ||
		len(evidence.StartupBindings) != 2 {
		t.Fatal("P14 Claude transcript proof omitted required bindings")
	}
	for _, mode := range []string{
		"missing_result",
		"extra_haft_call",
		"stale_startup",
		"hung_result",
		"forged_instructions",
		"wrong_status_pid",
		"wrong_status_start",
		"wrong_status_path",
	} {
		t.Run(mode, func(t *testing.T) {
			mutated := syntheticP14ClaudeSessionJSONL(
				t,
				prepared,
				runtime,
				protocol,
				mode,
			)
			if _, err := deriveP14ClaudeSessionHistoryEvidence(
				sourcePath,
				mutated,
				prepared,
				runtime,
				protocol,
				observedAt,
			); err == nil {
				t.Fatalf(
					"P14 Claude transcript accepted %s",
					mode,
				)
			}
		})
	}
	for _, fulfilledAt := range []string{
		"2026-07-28T10:00:01.900Z",
		"2026-07-28T10:00:04.100Z",
	} {
		t.Run("receipt_outside_"+fulfilledAt, func(t *testing.T) {
			outside := runtime
			outside.LiveMCPFulfilledAt = fulfilledAt
			if _, err := deriveP14ClaudeSessionHistoryEvidence(
				sourcePath,
				raw,
				prepared,
				outside,
				protocol,
				observedAt,
			); err == nil {
				t.Fatal(
					"P14 Claude transcript accepted a receipt outside the first status pair",
				)
			}
		})
	}
}

func TestP14ClaudeHostProofRequestRejectsCallerHistoryAndCodexImports(
	t *testing.T,
) {
	for _, forbidden := range []string{
		"claude_history_root",
		"codex_mcp_capture_path",
	} {
		t.Run(forbidden, func(t *testing.T) {
			raw, err := json.MarshalIndent(map[string]any{
				"schema":                p14ClaudeHostProofRequestSchema,
				"prepared_carrier_path": ".context/p14/prepared.json",
				"claude_pid":            101,
				"mcp_pid":               102,
				forbidden:               "/caller/fabricated",
			}, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			raw = append(raw, '\n')
			request := p14ClaudeHostProofRequest{}
			if err := decodeP14CanonicalCarrier(
				raw,
				&request,
				"Claude host proof request",
			); err == nil {
				t.Fatalf(
					"P14 Claude request accepted %s",
					forbidden,
				)
			}
		})
	}
}

func TestP14ClaudeHistoryRootIgnoresCallerHomeAndPrefixIsAppendOnly(
	t *testing.T,
) {
	canonical, err := p14CanonicalClaudeProjectsRoot()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", t.TempDir())
	afterOverride, err := p14CanonicalClaudeProjectsRoot()
	if err != nil {
		t.Fatal(err)
	}
	if canonical != afterOverride {
		t.Fatal("P14 Claude history root followed caller HOME")
	}
	path := filepath.Join(t.TempDir(), "session.jsonl")
	prefix := []byte("{\"type\":\"one\"}\n")
	appended := []byte("{\"type\":\"two\"}\n")
	if err := os.WriteFile(
		path,
		append(slices.Clone(prefix), appended...),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	observed, err := readP14ClaudeSessionHistoryPrefix(
		path,
		int64(len(prefix)),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(observed, prefix) {
		t.Fatal("P14 Claude prefix reader included appended bytes")
	}
	tampered := slices.Clone(prefix)
	tampered[2] = 'X'
	if err := os.WriteFile(
		path,
		append(tampered, appended...),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	changed, err := readP14ClaudeSessionHistoryPrefix(
		path,
		int64(len(prefix)),
	)
	if err != nil {
		t.Fatal(err)
	}
	if p14Digest(changed) == p14Digest(prefix) {
		t.Fatal("P14 Claude prefix tamper retained the frozen digest")
	}
}

func TestP14ClaudeProcessSessionSelectorMustMatchTranscript(t *testing.T) {
	if err := p14ClaudeSessionMatchesProcessArguments(
		[]string{"claude", "--session-id", "session-a"},
		"session-b",
	); err == nil {
		t.Fatal("P14 Claude proof accepted another process session")
	}
	if err := p14ClaudeSessionMatchesProcessArguments(
		[]string{"claude", "--resume"},
		"session-b",
	); err != nil {
		t.Fatal(err)
	}
	if err := p14ClaudeSessionMatchesProcessArguments(
		[]string{"claude", "--resume", "session-b"},
		"session-b",
	); err != nil {
		t.Fatal(err)
	}
}

func syntheticP14ClaudeSessionBasis(
	t *testing.T,
) (
	preparedRequestOracleInput,
	p14RuntimeObservationBinding,
	p14MCPProtocolDiscovery,
) {
	t.Helper()
	root := t.TempDir()
	executable := filepath.Join(root, "haft")
	content := []byte("synthetic P14 Claude candidate")
	if err := os.WriteFile(executable, content, 0o755); err != nil {
		t.Fatal(err)
	}
	prepared := preparedRequestOracleInput{
		FrozenBasis: frozenP14Basis{
			Candidate: candidateP14Basis{
				ExecutableDigest: p14Digest(content),
			},
			SelectedProject: selectedProjectP14Basis{
				ProjectRoot: root,
			},
		},
	}
	runtime := p14RuntimeObservationBinding{
		RestartCheckpointCreatedAt: "2026-07-28T10:00:00Z",
		LiveMCPPID:                 410,
		LiveMCPStartedAt:           "2026-07-28T10:00:01Z",
		LiveMCPFulfilledAt:         "2026-07-28T10:00:03Z",
		LiveMCPExecutablePath:      executable,
		LiveMCPExecutableDigest:    p14Digest(content),
		LiveMCPProjectRoot:         root,
		LiveMCPReceiptDigest:       p14TestDigest("claude-session-receipt"),
	}
	protocol, err := syntheticP14MCPProtocolDiscovery(
		runtime,
		time.Date(2026, 7, 28, 10, 1, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatal(err)
	}
	return prepared, runtime, protocol
}

func syntheticP14ClaudeSessionPath(t *testing.T) string {
	t.Helper()
	root, err := p14CanonicalClaudeProjectsRoot()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(
		root,
		"-synthetic-p14-claude",
		"11111111-1111-4111-8111-111111111111.jsonl",
	)
}

func syntheticP14ClaudeSessionJSONL(
	t *testing.T,
	prepared preparedRequestOracleInput,
	runtime p14RuntimeObservationBinding,
	protocol p14MCPProtocolDiscovery,
	mode string,
) []byte {
	t.Helper()
	instructions, tools, err := p14ClaudeStartupExpectations(protocol)
	if err != nil {
		t.Fatal(err)
	}
	if mode == "forged_instructions" {
		instructions += "\nforged"
	}
	sessionID := "11111111-1111-4111-8111-111111111111"
	projectRoot := prepared.FrozenBasis.SelectedProject.ProjectRoot
	startupAt := "2026-07-28T10:00:01.500Z"
	if mode == "stale_startup" {
		startupAt = "2026-07-28T09:59:59Z"
	}
	lines := [][]byte{
		mustP14JSONLine(t, map[string]any{
			"type":       "attachment",
			"timestamp":  startupAt,
			"sessionId":  sessionID,
			"cwd":        projectRoot,
			"uuid":       "startup-tools",
			"parentUuid": "root",
			"attachment": map[string]any{
				"type":              "deferred_tools_delta",
				"addedNames":        tools,
				"addedLines":        tools,
				"pendingMcpServers": []string{},
				"readdedNames":      []string{},
				"removedNames":      []string{},
			},
		}),
		mustP14JSONLine(t, map[string]any{
			"type":       "attachment",
			"timestamp":  startupAt,
			"sessionId":  sessionID,
			"cwd":        projectRoot,
			"uuid":       "startup-instructions",
			"parentUuid": "startup-tools",
			"attachment": map[string]any{
				"type":         "mcp_instructions_delta",
				"addedNames":   []string{"haft"},
				"addedBlocks":  []string{"## haft\n" + instructions},
				"removedNames": []string{},
			},
		}),
	}
	useTimes := []time.Time{
		time.Date(2026, 7, 28, 10, 0, 2, 0, time.UTC),
		time.Date(2026, 7, 28, 10, 0, 5, 0, time.UTC),
		time.Date(2026, 7, 28, 10, 0, 7, 0, time.UTC),
	}
	resultTimes := []time.Time{
		time.Date(2026, 7, 28, 10, 0, 4, 0, time.UTC),
		time.Date(2026, 7, 28, 10, 0, 6, 0, time.UTC),
		time.Date(2026, 7, 28, 10, 0, 8, 0, time.UTC),
	}
	if mode == "hung_result" {
		resultTimes[1] = useTimes[1].Add(p14ClaudeToolResultTimeout + time.Second)
	}
	for index, expected := range p14ClaudeExpectedToolCalls {
		toolUseID := fmt.Sprintf("toolu_p14_claude_%d", index+1)
		useUUID := fmt.Sprintf("use-%d", index+1)
		args := map[string]any{}
		if err := json.Unmarshal(
			[]byte(expected.ArgumentsCanonical),
			&args,
		); err != nil {
			t.Fatal(err)
		}
		lines = append(lines, mustP14JSONLine(t, map[string]any{
			"type":        "assistant",
			"timestamp":   useTimes[index].Format(time.RFC3339Nano),
			"sessionId":   sessionID,
			"cwd":         projectRoot,
			"uuid":        useUUID,
			"parentUuid":  fmt.Sprintf("result-%d", index),
			"isSidechain": false,
			"version":     "9.0.0-p14",
			"entrypoint":  "cli",
			"message": map[string]any{
				"role": "assistant",
				"content": []any{
					map[string]any{
						"type":   "tool_use",
						"id":     toolUseID,
						"name":   expected.Tool,
						"input":  args,
						"caller": map[string]any{"type": "direct"},
					},
				},
			},
		}))
		if mode == "missing_result" && index == 1 {
			continue
		}
		response := `{"kind":"ok"}`
		if expected.Role == p14ClaudeCallStatusBefore ||
			expected.Role == p14ClaudeCallStatusAfter {
			statusPID := runtime.LiveMCPPID
			statusStartedAt := runtime.LiveMCPStartedAt
			statusExecutable := runtime.LiveMCPExecutablePath
			if index == 2 {
				switch mode {
				case "wrong_status_pid":
					statusPID++
				case "wrong_status_start":
					startedAt, err := time.Parse(
						time.RFC3339Nano,
						statusStartedAt,
					)
					if err != nil {
						t.Fatal(err)
					}
					statusStartedAt = startedAt.
						Add(time.Second).
						Format(time.RFC3339)
				case "wrong_status_path":
					statusExecutable = filepath.Join(
						runtime.LiveMCPProjectRoot,
						"other-haft",
					)
					if err := os.WriteFile(
						statusExecutable,
						[]byte("other candidate"),
						0o755,
					); err != nil {
						t.Fatal(err)
					}
				}
			}
			response = fmt.Sprintf(
				"## Haft Status\n\n### Runtime\n\n- `haft serve`: pid=%d started=%s executable=`%s` executable_mtime=2026-07-28T10:00:00Z\n",
				statusPID,
				statusStartedAt,
				statusExecutable,
			)
		}
		lines = append(lines, mustP14JSONLine(t, map[string]any{
			"type":       "user",
			"timestamp":  resultTimes[index].Format(time.RFC3339Nano),
			"sessionId":  sessionID,
			"cwd":        projectRoot,
			"uuid":       fmt.Sprintf("result-%d", index+1),
			"parentUuid": useUUID,
			"message": map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{
						"type":        "tool_result",
						"tool_use_id": toolUseID,
						"is_error":    false,
						"content":     response,
					},
				},
			},
		}))
	}
	if mode == "extra_haft_call" {
		lines = append(lines, mustP14JSONLine(t, map[string]any{
			"type":        "assistant",
			"timestamp":   "2026-07-28T10:00:09Z",
			"sessionId":   sessionID,
			"cwd":         projectRoot,
			"uuid":        "extra-use",
			"parentUuid":  "result-3",
			"isSidechain": false,
			"version":     "9.0.0-p14",
			"entrypoint":  "cli",
			"message": map[string]any{
				"role": "assistant",
				"content": []any{
					map[string]any{
						"type": "tool_use",
						"id":   "toolu_p14_extra",
						"name": "mcp__haft__haft_query",
						"input": map[string]any{
							"action": "status",
							"full":   false,
						},
						"caller": map[string]any{"type": "direct"},
					},
				},
			},
		}))
	}
	_ = runtime
	return append(bytes.Join(lines, []byte{'\n'}), '\n')
}
