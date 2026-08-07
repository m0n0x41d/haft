package p14acceptance

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"
)

const (
	p14CodexSessionHistorySchema = "haft.p14.codex-session-history/v2"
	p14CodexSessionHistoryLimit  = int64(512 << 20)
)

type p14CodexSessionHistoryEvidence struct {
	Schema                string                              `json:"schema"`
	ThreadID              string                              `json:"thread_id"`
	SourcePath            string                              `json:"source_path"`
	SourceDevice          uint64                              `json:"source_device"`
	SourceInode           uint64                              `json:"source_inode"`
	SourcePrefixBytes     int64                               `json:"source_prefix_bytes"`
	SourcePrefixDigest    string                              `json:"source_prefix_digest"`
	ObservedAt            string                              `json:"observed_at"`
	SessionMetaLine       int                                 `json:"session_meta_line"`
	SessionMetaLineDigest string                              `json:"session_meta_line_digest"`
	CallBindings          []p14CodexSessionHistoryCallBinding `json:"call_bindings"`
	ExchangeBindings      []p14CodexMCPExchangeBinding        `json:"exchange_bindings"`
	EvidenceDigest        string                              `json:"evidence_digest"`
}

type p14CodexSessionHistoryCallBinding struct {
	Sequence               int    `json:"sequence"`
	ToolCallID             string `json:"tool_call_id"`
	TurnID                 string `json:"turn_id"`
	BeginLine              int    `json:"begin_line"`
	BeginLineDigest        string `json:"begin_line_digest"`
	EndLine                int    `json:"end_line"`
	EndLineDigest          string `json:"end_line_digest"`
	PromptLine             int    `json:"prompt_line,omitempty"`
	PromptLineDigest       string `json:"prompt_line_digest,omitempty"`
	ExchangeEvidenceDigest string `json:"exchange_evidence_digest"`
}

type p14CodexMCPExchangeBinding struct {
	ExchangeID             string                    `json:"exchange_id"`
	ScenarioID             string                    `json:"scenario_id"`
	ParallelGroup          string                    `json:"parallel_group,omitempty"`
	IdentityBeforeSequence int                       `json:"identity_before_sequence"`
	IdentityAfterSequence  int                       `json:"identity_after_sequence"`
	TargetSequences        []int                     `json:"target_sequences"`
	BasisBeforeSequence    int                       `json:"basis_before_sequence,omitempty"`
	BasisAfterSequence     int                       `json:"basis_after_sequence,omitempty"`
	BasisBefore            *p14CodexMemoryBasisProof `json:"basis_before,omitempty"`
	BasisAfter             *p14CodexMemoryBasisProof `json:"basis_after,omitempty"`
	RuntimeIdentityDigest  string                    `json:"runtime_identity_digest"`
	EvidenceDigest         string                    `json:"evidence_digest"`
}

type p14CodexMemoryBasisProof struct {
	TypeEnvRef     string `json:"type_env_ref"`
	TypeEnvDigest  string `json:"type_env_digest"`
	GraphRevision  int64  `json:"graph_revision"`
	ResponseDigest string `json:"response_digest"`
}

type p14CodexSessionUserEvent struct {
	Line       int
	LineDigest string
	TurnID     string
	Text       string
}

type p14CodexSessionToolBegin struct {
	Line          int
	LineDigest    string
	ThreadID      string
	TurnID        string
	CallID        string
	Server        string
	Tool          string
	ArgsCanonical string
	OccurredAt    time.Time
}

type p14CodexSessionToolEvent struct {
	Line                 int
	LineDigest           string
	ThreadID             string
	TurnID               string
	CallID               string
	Server               string
	Tool                 string
	ArgsCanonical        string
	Status               string
	DurationMilliseconds int64
	TurnToolCallOrdinal  int
	TurnToolCallCount    int
	OccurredAt           time.Time
	IsError              bool
	Body                 []byte
}

type p14CodexSessionFileIdentity struct {
	Device uint64
	Inode  uint64
}

func p14CodexSessionIdentityFromFileInfo(
	info os.FileInfo,
) (p14CodexSessionFileIdentity, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Dev == 0 || stat.Ino == 0 {
		return p14CodexSessionFileIdentity{}, fmt.Errorf(
			"P14 Codex session history has no stable device/inode identity",
		)
	}
	return p14CodexSessionFileIdentity{
		Device: uint64(stat.Dev),
		Inode:  uint64(stat.Ino),
	}, nil
}

type p14CodexSessionToolCounter struct {
	TurnID string
	CallID string
	Line   int
}

func captureP14CodexSessionHistory(
	packet p14CodexMCPRequestCarrier,
	input p14CodexMCPCaptureInput,
	observedAt time.Time,
) (p14CodexSessionHistoryEvidence, error) {
	sourcePath, err := locateP14CodexSessionHistory(
		packet.Packet.Runtime,
	)
	if err != nil {
		return p14CodexSessionHistoryEvidence{}, err
	}
	raw, sourceIdentity, err := readP14CodexSessionHistoryPrefix(
		sourcePath,
		0,
	)
	if err != nil {
		return p14CodexSessionHistoryEvidence{}, err
	}
	return deriveP14CodexSessionHistoryEvidence(
		sourcePath,
		raw,
		sourceIdentity,
		packet,
		input,
		observedAt,
	)
}

func locateP14CodexSessionHistory(
	runtime p14RuntimeObservationBinding,
) (string, error) {
	threadID := runtime.ThreadID
	if strings.TrimSpace(threadID) == "" {
		return "", fmt.Errorf("P14 Codex session thread ID is absent")
	}
	expectedStateRoot, expectedSessionRoot, err :=
		p14CanonicalCodexRuntimeRoots()
	if err != nil {
		return "", err
	}
	if runtime.CodexStateRoot != expectedStateRoot ||
		runtime.CodexSessionRoot != expectedSessionRoot {
		return "", fmt.Errorf(
			"P14 Codex session roots differ from verified runtime binding",
		)
	}
	sessionRoot := filepath.Clean(runtime.CodexSessionRoot)
	rootInfo, err := os.Lstat(sessionRoot)
	if err != nil {
		return "", err
	}
	if !rootInfo.IsDir() ||
		rootInfo.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf(
			"P14 Codex session root is not a physical directory",
		)
	}
	matches := make([]string, 0, 1)
	err = filepath.WalkDir(
		sessionRoot,
		func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			name := entry.Name()
			if filepath.Ext(name) != ".jsonl" ||
				!strings.HasSuffix(
					name,
					"-"+threadID+".jsonl",
				) {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf(
					"P14 Codex session history is not a regular file: %s",
					path,
				)
			}
			clean := filepath.Clean(path)
			if !p14PathIsWithin(sessionRoot, clean) {
				return fmt.Errorf(
					"P14 Codex session history escapes session root",
				)
			}
			matches = append(matches, clean)
			return nil
		},
	)
	if err != nil {
		return "", err
	}
	slices.Sort(matches)
	if len(matches) != 1 {
		return "", fmt.Errorf(
			"P14 Codex session history match count for %q is %d",
			threadID,
			len(matches),
		)
	}
	return matches[0], nil
}

func readP14CodexSessionHistoryPrefix(
	sourcePath string,
	prefixBytes int64,
) ([]byte, p14CodexSessionFileIdentity, error) {
	info, err := os.Lstat(sourcePath)
	if err != nil {
		return nil, p14CodexSessionFileIdentity{}, err
	}
	if !info.Mode().IsRegular() ||
		info.Mode()&os.ModeSymlink != 0 {
		return nil, p14CodexSessionFileIdentity{}, fmt.Errorf(
			"P14 Codex session history must be a regular non-symlink file",
		)
	}
	sourceIdentity, err := p14CodexSessionIdentityFromFileInfo(info)
	if err != nil {
		return nil, p14CodexSessionFileIdentity{}, err
	}
	size := info.Size()
	if prefixBytes == 0 {
		prefixBytes = size
	}
	if prefixBytes <= 0 ||
		prefixBytes > size ||
		prefixBytes > p14CodexSessionHistoryLimit {
		return nil, p14CodexSessionFileIdentity{}, fmt.Errorf(
			"P14 Codex session history prefix size is invalid",
		)
	}
	file, err := os.Open(sourcePath)
	if err != nil {
		return nil, p14CodexSessionFileIdentity{}, err
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return nil, p14CodexSessionFileIdentity{}, err
	}
	openedIdentity, err := p14CodexSessionIdentityFromFileInfo(openedInfo)
	if err != nil {
		return nil, p14CodexSessionFileIdentity{}, err
	}
	if openedIdentity != sourceIdentity {
		return nil, p14CodexSessionFileIdentity{}, fmt.Errorf(
			"P14 Codex session history changed identity before read",
		)
	}
	raw, err := io.ReadAll(io.LimitReader(file, prefixBytes))
	if err != nil {
		return nil, p14CodexSessionFileIdentity{}, err
	}
	if int64(len(raw)) != prefixBytes {
		return nil, p14CodexSessionFileIdentity{}, fmt.Errorf(
			"P14 Codex session history prefix was truncated",
		)
	}
	afterInfo, err := file.Stat()
	if err != nil {
		return nil, p14CodexSessionFileIdentity{}, err
	}
	afterIdentity, err := p14CodexSessionIdentityFromFileInfo(afterInfo)
	if err != nil {
		return nil, p14CodexSessionFileIdentity{}, err
	}
	if afterIdentity != sourceIdentity ||
		afterInfo.Size() < prefixBytes {
		return nil, p14CodexSessionFileIdentity{}, fmt.Errorf(
			"P14 Codex session history changed identity during read",
		)
	}
	return raw, sourceIdentity, nil
}

func deriveP14CodexSessionHistoryEvidence(
	sourcePath string,
	raw []byte,
	sourceIdentity p14CodexSessionFileIdentity,
	packet p14CodexMCPRequestCarrier,
	input p14CodexMCPCaptureInput,
	observedAt time.Time,
) (p14CodexSessionHistoryEvidence, error) {
	capturedAt, err := time.Parse(time.RFC3339Nano, input.CapturedAt)
	if err != nil {
		return p14CodexSessionHistoryEvidence{}, err
	}
	if observedAt.Before(capturedAt) {
		return p14CodexSessionHistoryEvidence{}, fmt.Errorf(
			"P14 Codex session history was observed before capture",
		)
	}
	windowStart, err := p14CodexCaptureWindowStart(packet.Packet)
	if err != nil {
		return p14CodexSessionHistoryEvidence{}, err
	}
	if observedAt.Before(windowStart) {
		return p14CodexSessionHistoryEvidence{}, fmt.Errorf(
			"P14 Codex session history predates verified restart evidence",
		)
	}
	lines := bytes.Split(raw, []byte{'\n'})
	users := make(map[string][]p14CodexSessionUserEvent)
	begins := make(map[string]p14CodexSessionToolBegin)
	ends := make(map[string]p14CodexSessionToolEvent)
	candidateBegins := make([]string, 0)
	candidateEnds := make([]string, 0)
	turnCounters := make([]p14CodexSessionToolCounter, 0)
	currentTurnID := ""
	sessionMetaLine := 0
	sessionMetaLineDigest := ""
	threadID := packet.Packet.Runtime.ThreadID
	for index, line := range lines {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		lineNumber := index + 1
		root, err := decodeP14CodexSessionLine(line)
		if err != nil {
			return p14CodexSessionHistoryEvidence{}, fmt.Errorf(
				"P14 Codex session line %d: %w",
				lineNumber,
				err,
			)
		}
		lineDigest := p14Digest(line)
		eventType := p14JSONText(root["type"])
		payload := p14JSONMap(root["payload"])
		if eventType == "session_meta" {
			if sessionMetaLine != 0 ||
				p14JSONText(payload["id"]) != threadID {
				return p14CodexSessionHistoryEvidence{}, fmt.Errorf(
					"P14 Codex session metadata differs",
				)
			}
			sessionMetaLine = lineNumber
			sessionMetaLineDigest = lineDigest
			continue
		}
		user, present := p14CodexSessionUserFromLine(
			root,
			lineNumber,
			lineDigest,
		)
		if present {
			currentTurnID = user.TurnID
			users[user.TurnID] = append(users[user.TurnID], user)
			continue
		}
		counter, present := p14CodexSessionFunctionCallCounter(
			root,
			currentTurnID,
			lineNumber,
		)
		if present {
			turnCounters = append(turnCounters, counter)
		}
		begin, present, err := p14CodexSessionMCPBeginFromLine(
			root,
			currentTurnID,
			lineNumber,
			lineDigest,
		)
		if err != nil {
			return p14CodexSessionHistoryEvidence{}, err
		}
		if present {
			if _, duplicate := begins[begin.CallID]; duplicate {
				return p14CodexSessionHistoryEvidence{}, fmt.Errorf(
					"P14 Codex session repeats tool-call begin %q",
					begin.CallID,
				)
			}
			begins[begin.CallID] = begin
			if begin.Server == "haft" &&
				!begin.OccurredAt.Before(windowStart) &&
				!begin.OccurredAt.After(observedAt) {
				candidateBegins = append(
					candidateBegins,
					begin.CallID,
				)
			}
		}
		tool, present, err := p14CodexSessionMCPToolFromLine(
			root,
			currentTurnID,
			lineNumber,
			lineDigest,
		)
		if err != nil {
			return p14CodexSessionHistoryEvidence{}, err
		}
		if !present {
			continue
		}
		if _, duplicate := ends[tool.CallID]; duplicate {
			return p14CodexSessionHistoryEvidence{}, fmt.Errorf(
				"P14 Codex session repeats tool-call end %q",
				tool.CallID,
			)
		}
		ends[tool.CallID] = tool
		if tool.Server == "haft" &&
			!tool.OccurredAt.Before(windowStart) &&
			!tool.OccurredAt.After(observedAt) {
			candidateEnds = append(candidateEnds, tool.CallID)
		}
	}
	if sessionMetaLine == 0 {
		return p14CodexSessionHistoryEvidence{}, fmt.Errorf(
			"P14 Codex session metadata is absent",
		)
	}
	if len(candidateBegins) != len(input.Calls) ||
		len(candidateEnds) != len(input.Calls) {
		return p14CodexSessionHistoryEvidence{}, fmt.Errorf(
			"P14 Codex session contains %d Haft begins and %d ends in the capture window, want %d each",
			len(candidateBegins),
			len(candidateEnds),
			len(input.Calls),
		)
	}
	turnCounts, turnOrdinals := p14CodexSessionTurnCallPositions(
		turnCounters,
	)
	candidateBeginSet := make(map[string]struct{}, len(candidateBegins))
	for _, callID := range candidateBegins {
		candidateBeginSet[callID] = struct{}{}
	}
	candidateEndSet := make(map[string]struct{}, len(candidateEnds))
	for _, callID := range candidateEnds {
		candidateEndSet[callID] = struct{}{}
	}
	bindings := make(
		[]p14CodexSessionHistoryCallBinding,
		0,
		len(input.Calls),
	)
	for index, evidence := range input.Calls {
		planned := packet.Packet.Calls[index]
		callID := evidence.Transcript.ToolCallID
		if _, present := candidateBeginSet[callID]; !present {
			return p14CodexSessionHistoryEvidence{}, fmt.Errorf(
				"P14 Codex session omits selected call begin %q",
				callID,
			)
		}
		if _, present := candidateEndSet[callID]; !present {
			return p14CodexSessionHistoryEvidence{}, fmt.Errorf(
				"P14 Codex session omits selected call end %q",
				callID,
			)
		}
		delete(candidateBeginSet, callID)
		delete(candidateEndSet, callID)
		begin := begins[callID]
		tool := ends[callID]
		if err := validateP14CodexSessionToolProjection(
			planned,
			evidence,
			begin,
			tool,
			turnCounts[tool.TurnID],
			turnOrdinals[tool.TurnID+"/"+tool.CallID],
		); err != nil {
			return p14CodexSessionHistoryEvidence{}, err
		}
		binding := p14CodexSessionHistoryCallBinding{
			Sequence:        evidence.Sequence,
			ToolCallID:      callID,
			TurnID:          tool.TurnID,
			BeginLine:       begin.Line,
			BeginLineDigest: begin.LineDigest,
			EndLine:         tool.Line,
			EndLineDigest:   tool.LineDigest,
		}
		if planned.AgentPrompt != nil {
			prompt, err := selectP14CodexSessionPrompt(
				users[tool.TurnID],
				planned.AgentPrompt,
			)
			if err != nil {
				return p14CodexSessionHistoryEvidence{}, err
			}
			binding.PromptLine = prompt.Line
			binding.PromptLineDigest = prompt.LineDigest
		}
		bindings = append(bindings, binding)
	}
	if len(candidateBeginSet) != 0 ||
		len(candidateEndSet) != 0 {
		return p14CodexSessionHistoryEvidence{}, fmt.Errorf(
			"P14 Codex session contains unbound Haft begin/end events",
		)
	}
	exchangeBindings, exchangeDigests, err :=
		deriveP14CodexMCPExchangeBindings(
			packet,
			input,
			bindings,
			begins,
			ends,
		)
	if err != nil {
		return p14CodexSessionHistoryEvidence{}, err
	}
	for index := range bindings {
		exchangeDigest := exchangeDigests[packet.Packet.Calls[index].ExchangeID]
		bindings[index].ExchangeEvidenceDigest = exchangeDigest
	}
	evidence := p14CodexSessionHistoryEvidence{
		Schema:                p14CodexSessionHistorySchema,
		ThreadID:              threadID,
		SourcePath:            filepath.Clean(sourcePath),
		SourceDevice:          sourceIdentity.Device,
		SourceInode:           sourceIdentity.Inode,
		SourcePrefixBytes:     int64(len(raw)),
		SourcePrefixDigest:    p14Digest(raw),
		ObservedAt:            observedAt.UTC().Format(time.RFC3339Nano),
		SessionMetaLine:       sessionMetaLine,
		SessionMetaLineDigest: sessionMetaLineDigest,
		CallBindings:          bindings,
		ExchangeBindings:      exchangeBindings,
	}
	digestBasis, err := p14CodexSessionHistoryDigestBasis(evidence)
	if err != nil {
		return p14CodexSessionHistoryEvidence{}, err
	}
	evidence.EvidenceDigest = p14Digest(digestBasis)
	if err := validateP14CodexSessionHistoryEvidence(
		packet,
		input,
		evidence,
	); err != nil {
		return p14CodexSessionHistoryEvidence{}, err
	}
	return evidence, nil
}

func p14CodexCaptureWindowStart(
	packet p14CodexMCPRequestInput,
) (time.Time, error) {
	generatedAt, err := time.Parse(
		time.RFC3339Nano,
		packet.GeneratedAt,
	)
	if err != nil {
		return time.Time{}, err
	}
	checkpointAt, err := time.Parse(
		time.RFC3339Nano,
		packet.Runtime.RestartCheckpointCreatedAt,
	)
	if err != nil {
		return time.Time{}, err
	}
	fulfilledAt, err := time.Parse(
		time.RFC3339Nano,
		packet.Runtime.LiveMCPFulfilledAt,
	)
	if err != nil {
		return time.Time{}, err
	}
	if generatedAt.Before(checkpointAt) ||
		generatedAt.Before(fulfilledAt) {
		return time.Time{}, fmt.Errorf(
			"P14 Codex request predates verified restart or live MCP receipt",
		)
	}
	return generatedAt.UTC(), nil
}

func decodeP14CodexSessionLine(
	line []byte,
) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(line))
	decoder.UseNumber()
	root := map[string]any{}
	if err := decoder.Decode(&root); err != nil {
		return nil, err
	}
	return root, nil
}

func p14CodexSessionUserFromLine(
	root map[string]any,
	line int,
	lineDigest string,
) (p14CodexSessionUserEvent, bool) {
	if p14JSONText(root["type"]) != "response_item" {
		return p14CodexSessionUserEvent{}, false
	}
	payload := p14JSONMap(root["payload"])
	if p14JSONText(payload["type"]) != "message" ||
		p14JSONText(payload["role"]) != "user" {
		return p14CodexSessionUserEvent{}, false
	}
	metadata := p14JSONMap(
		payload["internal_chat_message_metadata_passthrough"],
	)
	turnID := p14JSONText(metadata["turn_id"])
	content := p14JSONArray(payload["content"])
	texts := make([]string, 0, 1)
	for _, raw := range content {
		item := p14JSONMap(raw)
		if p14JSONText(item["type"]) != "input_text" {
			continue
		}
		texts = append(texts, p14JSONText(item["text"]))
	}
	if turnID == "" || len(texts) != 1 {
		return p14CodexSessionUserEvent{}, false
	}
	return p14CodexSessionUserEvent{
		Line:       line,
		LineDigest: lineDigest,
		TurnID:     turnID,
		Text:       texts[0],
	}, true
}

func p14CodexSessionFunctionCallCounter(
	root map[string]any,
	turnID string,
	line int,
) (p14CodexSessionToolCounter, bool) {
	if p14JSONText(root["type"]) != "response_item" ||
		turnID == "" {
		return p14CodexSessionToolCounter{}, false
	}
	payload := p14JSONMap(root["payload"])
	if p14JSONText(payload["type"]) != "function_call" {
		return p14CodexSessionToolCounter{}, false
	}
	metadata := p14JSONMap(
		payload["internal_chat_message_metadata_passthrough"],
	)
	recordedTurnID := p14JSONText(metadata["turn_id"])
	if recordedTurnID != "" {
		turnID = recordedTurnID
	}
	callID := p14JSONText(payload["call_id"])
	if callID == "" || turnID == "" {
		return p14CodexSessionToolCounter{}, false
	}
	return p14CodexSessionToolCounter{
		TurnID: turnID,
		CallID: callID,
		Line:   line,
	}, true
}

func p14CodexSessionMCPBeginFromLine(
	root map[string]any,
	currentTurnID string,
	line int,
	lineDigest string,
) (p14CodexSessionToolBegin, bool, error) {
	if p14JSONText(root["type"]) != "response_item" {
		return p14CodexSessionToolBegin{}, false, nil
	}
	payload := p14JSONMap(root["payload"])
	if p14JSONText(payload["type"]) != "function_call" {
		return p14CodexSessionToolBegin{}, false, nil
	}
	namespace := p14JSONText(payload["namespace"])
	if !strings.HasPrefix(namespace, "mcp__") {
		return p14CodexSessionToolBegin{}, false, nil
	}
	server := strings.TrimPrefix(namespace, "mcp__")
	tool := p14JSONText(payload["name"])
	callID := p14JSONText(payload["call_id"])
	metadata := p14JSONMap(
		payload["internal_chat_message_metadata_passthrough"],
	)
	turnID := p14JSONText(metadata["turn_id"])
	if turnID == "" {
		turnID = currentTurnID
	}
	if server == "" || tool == "" || callID == "" || turnID == "" {
		return p14CodexSessionToolBegin{}, false, fmt.Errorf(
			"P14 Codex MCP begin identity differs",
		)
	}
	argsCanonical, err := p14CodexSessionCanonicalArguments(
		payload["arguments"],
	)
	if err != nil {
		return p14CodexSessionToolBegin{}, false, err
	}
	occurredAt, err := time.Parse(
		time.RFC3339Nano,
		p14JSONText(root["timestamp"]),
	)
	if err != nil {
		return p14CodexSessionToolBegin{}, false, err
	}
	return p14CodexSessionToolBegin{
		Line:          line,
		LineDigest:    lineDigest,
		TurnID:        turnID,
		CallID:        callID,
		Server:        server,
		Tool:          tool,
		ArgsCanonical: string(argsCanonical),
		OccurredAt:    occurredAt.UTC(),
	}, true, nil
}

func p14CodexSessionCanonicalArguments(raw any) ([]byte, error) {
	text, ok := raw.(string)
	if !ok || strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf(
			"P14 Codex MCP begin arguments are not exact JSON text",
		)
	}
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	args := map[string]any{}
	if err := decoder.Decode(&args); err != nil {
		return nil, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf(
			"P14 Codex MCP begin arguments have trailing JSON",
		)
	}
	return marshalP14CanonicalJSON(args)
}

func p14CodexSessionMCPToolFromLine(
	root map[string]any,
	turnID string,
	line int,
	lineDigest string,
) (p14CodexSessionToolEvent, bool, error) {
	if p14JSONText(root["type"]) != "event_msg" {
		return p14CodexSessionToolEvent{}, false, nil
	}
	payload := p14JSONMap(root["payload"])
	if p14JSONText(payload["type"]) != "mcp_tool_call_end" {
		return p14CodexSessionToolEvent{}, false, nil
	}
	if turnID == "" {
		return p14CodexSessionToolEvent{}, false, fmt.Errorf(
			"P14 Codex MCP call has no current user turn",
		)
	}
	invocation := p14JSONMap(payload["invocation"])
	argsCanonical, err := marshalP14CanonicalJSON(
		p14JSONMap(invocation["arguments"]),
	)
	if err != nil {
		return p14CodexSessionToolEvent{}, false, err
	}
	status, isError, body, err :=
		p14CodexSessionMCPResult(payload["result"])
	if err != nil {
		return p14CodexSessionToolEvent{}, false, err
	}
	occurredAt, err := time.Parse(
		time.RFC3339Nano,
		p14JSONText(root["timestamp"]),
	)
	if err != nil {
		return p14CodexSessionToolEvent{}, false, err
	}
	duration := p14JSONMap(payload["duration"])
	seconds, secondsOK := p14JSONInt64(duration["secs"])
	nanoseconds, nanosecondsOK := p14JSONInt64(duration["nanos"])
	if !secondsOK ||
		!nanosecondsOK ||
		seconds < 0 ||
		nanoseconds < 0 ||
		nanoseconds >= int64(time.Second) {
		return p14CodexSessionToolEvent{}, false, fmt.Errorf(
			"P14 Codex MCP duration differs",
		)
	}
	callID := p14JSONText(payload["call_id"])
	server := p14JSONText(invocation["server"])
	tool := p14JSONText(invocation["tool"])
	if callID == "" || server == "" || tool == "" {
		return p14CodexSessionToolEvent{}, false, fmt.Errorf(
			"P14 Codex MCP identity differs",
		)
	}
	return p14CodexSessionToolEvent{
		Line:                 line,
		LineDigest:           lineDigest,
		TurnID:               turnID,
		CallID:               callID,
		Server:               server,
		Tool:                 tool,
		ArgsCanonical:        string(argsCanonical),
		Status:               status,
		DurationMilliseconds: seconds*1000 + nanoseconds/1_000_000,
		OccurredAt:           occurredAt.UTC(),
		IsError:              isError,
		Body:                 body,
	}, true, nil
}

func p14CodexSessionMCPResult(
	raw any,
) (string, bool, []byte, error) {
	result := p14JSONMap(raw)
	if rawErr, present := result["Err"]; present {
		text, ok := rawErr.(string)
		if !ok || text == "" {
			return "", false, nil, fmt.Errorf(
				"P14 Codex MCP error result differs",
			)
		}
		lower := strings.ToLower(text)
		for _, interrupted := range []string{
			"cancelled",
			"canceled",
			"interrupted",
			"aborted",
		} {
			if strings.Contains(lower, interrupted) {
				return "interrupted", true, []byte(text), nil
			}
		}
		return "failed", true, []byte(text), nil
	}
	okResult := p14JSONMap(result["Ok"])
	content := p14JSONArray(okResult["content"])
	texts := make([]string, 0, 1)
	for _, rawContent := range content {
		item := p14JSONMap(rawContent)
		if p14JSONText(item["type"]) != "text" {
			continue
		}
		text, ok := item["text"].(string)
		if !ok {
			return "", false, nil, fmt.Errorf(
				"P14 Codex MCP text response is not exact text",
			)
		}
		texts = append(texts, text)
	}
	if len(texts) != 1 || texts[0] == "" {
		return "", false, nil, fmt.Errorf(
			"P14 Codex MCP text response differs",
		)
	}
	isError := p14JSONBool(okResult["isError"])
	status := "completed"
	if isError {
		status = "failed"
	}
	return status, isError, []byte(texts[0]), nil
}

func p14JSONInt64(raw any) (int64, bool) {
	switch value := raw.(type) {
	case json.Number:
		result, err := value.Int64()
		return result, err == nil
	case float64:
		result := int64(value)
		return result, float64(result) == value
	case int64:
		return value, true
	case int:
		return int64(value), true
	default:
		return 0, false
	}
}

func p14CodexSessionTurnCallPositions(
	counters []p14CodexSessionToolCounter,
) (map[string]int, map[string]int) {
	slices.SortFunc(
		counters,
		func(
			left p14CodexSessionToolCounter,
			right p14CodexSessionToolCounter,
		) int {
			if left.Line != right.Line {
				return left.Line - right.Line
			}
			return strings.Compare(left.CallID, right.CallID)
		},
	)
	counts := make(map[string]int)
	ordinals := make(map[string]int)
	seen := make(map[string]struct{})
	for _, counter := range counters {
		key := counter.TurnID + "/" + counter.CallID
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		counts[counter.TurnID]++
		ordinals[key] = counts[counter.TurnID]
	}
	return counts, ordinals
}

func validateP14CodexSessionToolProjection(
	planned p14CodexMCPPlannedCall,
	evidence p14CodexMCPCallEvidence,
	begin p14CodexSessionToolBegin,
	tool p14CodexSessionToolEvent,
	turnCallCount int,
	turnCallOrdinal int,
) error {
	responseBody, err := base64.StdEncoding.DecodeString(
		evidence.Response.BodyBase64,
	)
	if err != nil {
		return err
	}
	responseAt, err := time.Parse(
		time.RFC3339Nano,
		evidence.Response.CapturedAt,
	)
	if err != nil {
		return err
	}
	if begin.Server != "haft" ||
		begin.Tool != planned.Tool ||
		begin.ArgsCanonical != planned.ArgsCanonical ||
		tool.Server != "haft" ||
		tool.Tool != planned.Tool ||
		tool.ArgsCanonical != planned.ArgsCanonical {
		return fmt.Errorf(
			"P14 Codex session tool identity differs for %s/%s",
			planned.ScenarioID,
			planned.CaseID,
		)
	}
	if begin.TurnID != tool.TurnID ||
		begin.CallID != tool.CallID ||
		begin.OccurredAt.After(tool.OccurredAt) ||
		begin.Line >= tool.Line ||
		tool.TurnID != evidence.Transcript.TurnID {
		return fmt.Errorf(
			"P14 Codex session begin/end differs for %s/%s",
			planned.ScenarioID,
			planned.CaseID,
		)
	}
	if tool.Status != evidence.Transcript.Status ||
		tool.DurationMilliseconds !=
			evidence.Transcript.DurationMilliseconds ||
		tool.IsError != evidence.Response.IsError ||
		!bytes.Equal(tool.Body, responseBody) ||
		!tool.OccurredAt.Equal(responseAt) {
		return fmt.Errorf(
			"P14 Codex session response differs for %s/%s: status=%s/%s duration=%d/%d body_equal=%t time_equal=%t",
			planned.ScenarioID,
			planned.CaseID,
			tool.Status,
			evidence.Transcript.Status,
			tool.DurationMilliseconds,
			evidence.Transcript.DurationMilliseconds,
			bytes.Equal(tool.Body, responseBody),
			tool.OccurredAt.Equal(responseAt),
		)
	}
	if turnCallCount != evidence.Transcript.TurnToolCallCount ||
		turnCallOrdinal != evidence.Transcript.TurnToolCallOrdinal {
		return fmt.Errorf(
			"P14 Codex session turn projection differs for %s/%s",
			planned.ScenarioID,
			planned.CaseID,
		)
	}
	return nil
}

func selectP14CodexSessionPrompt(
	users []p14CodexSessionUserEvent,
	planned *p14CodexMCPPlannedAgentPrompt,
) (p14CodexSessionUserEvent, error) {
	matches := make([]p14CodexSessionUserEvent, 0, 1)
	for _, user := range users {
		if user.Text == planned.TextCanonical {
			matches = append(matches, user)
		}
	}
	if len(matches) != 1 {
		return p14CodexSessionUserEvent{}, fmt.Errorf(
			"P14 Codex prompt history match count for %q is %d",
			planned.ID,
			len(matches),
		)
	}
	return matches[0], nil
}

func validateP14CodexSessionHistoryEvidence(
	packet p14CodexMCPRequestCarrier,
	input p14CodexMCPCaptureInput,
	evidence p14CodexSessionHistoryEvidence,
) error {
	if evidence.Schema != p14CodexSessionHistorySchema ||
		evidence.ThreadID != packet.Packet.Runtime.ThreadID ||
		!filepath.IsAbs(evidence.SourcePath) ||
		!p14PathIsWithin(
			packet.Packet.Runtime.CodexSessionRoot,
			evidence.SourcePath,
		) ||
		evidence.SourceDevice == 0 ||
		evidence.SourceInode == 0 ||
		evidence.SourcePrefixBytes <= 0 ||
		evidence.SourcePrefixBytes > p14CodexSessionHistoryLimit ||
		!validP14Digest(evidence.SourcePrefixDigest) ||
		evidence.SessionMetaLine <= 0 ||
		!validP14Digest(evidence.SessionMetaLineDigest) ||
		len(evidence.CallBindings) != len(input.Calls) ||
		len(evidence.ExchangeBindings) !=
			p14CodexMCPExchangeCount(packet.Packet.Calls) ||
		!validP14Digest(evidence.EvidenceDigest) {
		return fmt.Errorf("P14 Codex session history evidence basis differs")
	}
	if _, err := time.Parse(time.RFC3339Nano, evidence.ObservedAt); err != nil {
		return fmt.Errorf(
			"P14 Codex session history observation time differs",
		)
	}
	seenLines := make(map[int]struct{})
	for index, binding := range evidence.CallBindings {
		call := input.Calls[index]
		promptExpected := call.AgentPrompt != nil
		if binding.Sequence != call.Sequence ||
			binding.ToolCallID != call.Transcript.ToolCallID ||
			binding.TurnID != call.Transcript.TurnID ||
			binding.BeginLine <= 0 ||
			!validP14Digest(binding.BeginLineDigest) ||
			binding.EndLine <= binding.BeginLine ||
			!validP14Digest(binding.EndLineDigest) ||
			!validP14Digest(binding.ExchangeEvidenceDigest) ||
			(binding.PromptLine > 0) != promptExpected ||
			(binding.PromptLineDigest != "") != promptExpected {
			return fmt.Errorf(
				"P14 Codex session history call binding differs",
			)
		}
		if promptExpected &&
			!validP14Digest(binding.PromptLineDigest) {
			return fmt.Errorf(
				"P14 Codex session prompt binding differs",
			)
		}
		for _, line := range []int{
			binding.BeginLine,
			binding.EndLine,
			binding.PromptLine,
		} {
			if line == 0 {
				continue
			}
			if _, duplicate := seenLines[line]; duplicate {
				return fmt.Errorf(
					"P14 Codex session history line is reused",
				)
			}
			seenLines[line] = struct{}{}
		}
	}
	if err := validateP14CodexMCPExchangeEvidenceShape(
		packet,
		input,
		evidence.CallBindings,
		evidence.ExchangeBindings,
	); err != nil {
		return err
	}
	digestBasis, err := p14CodexSessionHistoryDigestBasis(evidence)
	if err != nil {
		return err
	}
	if p14Digest(digestBasis) != evidence.EvidenceDigest {
		return fmt.Errorf(
			"P14 Codex session history evidence digest differs",
		)
	}
	return nil
}

func verifyP14CodexSessionHistorySource(
	packet p14CodexMCPRequestCarrier,
	input p14CodexMCPCaptureInput,
	evidence p14CodexSessionHistoryEvidence,
) error {
	if err := validateP14CodexSessionHistoryEvidence(
		packet,
		input,
		evidence,
	); err != nil {
		return err
	}
	raw, sourceIdentity, err := readP14CodexSessionHistoryPrefix(
		evidence.SourcePath,
		evidence.SourcePrefixBytes,
	)
	if err != nil {
		return err
	}
	if sourceIdentity.Device != evidence.SourceDevice ||
		sourceIdentity.Inode != evidence.SourceInode ||
		p14Digest(raw) != evidence.SourcePrefixDigest {
		return fmt.Errorf(
			"P14 Codex session history prefix changed",
		)
	}
	observedAt, err := time.Parse(
		time.RFC3339Nano,
		evidence.ObservedAt,
	)
	if err != nil {
		return err
	}
	reobserved, err := deriveP14CodexSessionHistoryEvidence(
		evidence.SourcePath,
		raw,
		sourceIdentity,
		packet,
		input,
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
			"P14 Codex session history re-observation differs",
		)
	}
	return nil
}

func p14CodexSessionHistoryDigestBasis(
	evidence p14CodexSessionHistoryEvidence,
) ([]byte, error) {
	basis := struct {
		Schema                string                              `json:"schema"`
		ThreadID              string                              `json:"thread_id"`
		SourcePath            string                              `json:"source_path"`
		SourceDevice          uint64                              `json:"source_device"`
		SourceInode           uint64                              `json:"source_inode"`
		SourcePrefixBytes     int64                               `json:"source_prefix_bytes"`
		SourcePrefixDigest    string                              `json:"source_prefix_digest"`
		ObservedAt            string                              `json:"observed_at"`
		SessionMetaLine       int                                 `json:"session_meta_line"`
		SessionMetaLineDigest string                              `json:"session_meta_line_digest"`
		CallBindings          []p14CodexSessionHistoryCallBinding `json:"call_bindings"`
		ExchangeBindings      []p14CodexMCPExchangeBinding        `json:"exchange_bindings"`
	}{
		Schema:                evidence.Schema,
		ThreadID:              evidence.ThreadID,
		SourcePath:            evidence.SourcePath,
		SourceDevice:          evidence.SourceDevice,
		SourceInode:           evidence.SourceInode,
		SourcePrefixBytes:     evidence.SourcePrefixBytes,
		SourcePrefixDigest:    evidence.SourcePrefixDigest,
		ObservedAt:            evidence.ObservedAt,
		SessionMetaLine:       evidence.SessionMetaLine,
		SessionMetaLineDigest: evidence.SessionMetaLineDigest,
		CallBindings:          evidence.CallBindings,
		ExchangeBindings:      evidence.ExchangeBindings,
	}
	return marshalP14CanonicalJSON(basis)
}

func syntheticP14CodexSessionHistoryEvidence(
	t *testing.T,
	packet p14CodexMCPRequestCarrier,
	input p14CodexMCPCaptureInput,
) p14CodexSessionHistoryEvidence {
	t.Helper()
	raw := syntheticP14CodexSessionJSONL(t, packet, input)
	capturedAt := mustP14Time(t, input.CapturedAt)
	sourcePath := filepath.Join(
		packet.Packet.Runtime.CodexSessionRoot,
		"synthetic-"+packet.Packet.Runtime.ThreadID+".jsonl",
	)
	evidence, err := deriveP14CodexSessionHistoryEvidence(
		sourcePath,
		raw,
		p14CodexSessionFileIdentity{
			Device: 1,
			Inode:  1,
		},
		packet,
		input,
		capturedAt.Add(time.Minute),
	)
	if err != nil {
		t.Fatal(err)
	}
	return evidence
}

func TestP14CodexSessionHistoryRejectsProjectionAndExtraCallForgery(
	t *testing.T,
) {
	root, err := p14RepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	contract, rawContract, err := loadRequestOracleContract(root)
	if err != nil {
		t.Fatal(err)
	}
	preparedInput, err := completePreparedInputForTest(
		contract,
		p14Digest(rawContract),
	)
	if err != nil {
		t.Fatal(err)
	}
	preparationBytes, err := json.Marshal(preparedInput)
	if err != nil {
		t.Fatal(err)
	}
	preparationDigest := p14Digest(preparationBytes)
	digestBody := strings.TrimPrefix(preparationDigest, "sha256:")
	prepared := preparedRequestOracleCarrier{
		Schema: p14PreparedCarrierSchema,
		Status: p14ContractStatus,
		CarrierPath: filepath.ToSlash(filepath.Join(
			".context",
			"p14",
			"p14-prepared-request-oracle-"+digestBody[:16]+".json",
		)),
		PreparationDigest: preparationDigest,
		Preparation:       preparedInput,
	}
	preparedBytes, err := json.MarshalIndent(prepared, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	preparedBytes = append(preparedBytes, '\n')
	runtime := syntheticP14RuntimeObservationBinding(preparedInput)
	packet, err := buildP14CodexMCPRequestCarrier(
		contract,
		prepared.CarrierPath,
		p14Digest(preparedBytes),
		prepared,
		runtime,
		time.Now().UTC().Add(-time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}
	capturedAt := time.Now().UTC().Add(-time.Minute)
	input := syntheticP14CodexMCPCaptureInput(
		packet,
		packet.CarrierPath,
		p14TestDigest("session-history-packet-carrier"),
		capturedAt,
	)
	raw := syntheticP14CodexSessionJSONL(t, packet, input)
	syntheticSource := filepath.Join(
		packet.Packet.Runtime.CodexSessionRoot,
		"synthetic-"+packet.Packet.Runtime.ThreadID+".jsonl",
	)
	syntheticIdentity := p14CodexSessionFileIdentity{
		Device: 1,
		Inode:  1,
	}
	evidence, err := deriveP14CodexSessionHistoryEvidence(
		syntheticSource,
		raw,
		syntheticIdentity,
		packet,
		input,
		capturedAt.Add(time.Minute),
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(evidence.CallBindings) != len(input.Calls) {
		t.Fatal("P14 Codex session history omitted call bindings")
	}
	statusIndex := slices.IndexFunc(
		input.Calls,
		func(call p14CodexMCPCallEvidence) bool {
			return call.ExchangeRole ==
				p14CodexMCPExchangeIdentityBefore
		},
	)
	if statusIndex < 0 {
		t.Fatal("P14 Codex session fixture omits status bracket")
	}
	statusBody, err := p14CodexMCPResponseBody(
		input.Calls[statusIndex],
	)
	if err != nil {
		t.Fatal(err)
	}
	startedAt, err := time.Parse(
		time.RFC3339Nano,
		packet.Packet.Runtime.LiveMCPStartedAt,
	)
	if err != nil {
		t.Fatal(err)
	}
	statusTamperCases := []struct {
		Name        string
		OldFragment string
		NewFragment string
	}{
		{
			Name: "wrong runtime PID",
			OldFragment: fmt.Sprintf(
				"pid=%d",
				packet.Packet.Runtime.LiveMCPPID,
			),
			NewFragment: fmt.Sprintf(
				"pid=%d",
				packet.Packet.Runtime.LiveMCPPID+1,
			),
		},
		{
			Name: "wrong runtime start",
			OldFragment: "started=" +
				startedAt.UTC().Format(time.RFC3339),
			NewFragment: "started=" +
				startedAt.
					Add(time.Second).
					UTC().
					Format(time.RFC3339),
		},
		{
			Name: "wrong runtime executable",
			OldFragment: "executable=`" +
				packet.Packet.Runtime.LiveMCPExecutablePath +
				"`",
			NewFragment: "executable=`" +
				packet.Packet.Runtime.LiveMCPExecutablePath +
				"-forged`",
		},
	}
	for _, testCase := range statusTamperCases {
		t.Run(testCase.Name, func(t *testing.T) {
			tampered := input
			tampered.Calls = slices.Clone(input.Calls)
			body := bytes.Replace(
				statusBody,
				[]byte(testCase.OldFragment),
				[]byte(testCase.NewFragment),
				1,
			)
			if bytes.Equal(body, statusBody) {
				t.Fatal("status runtime tamper fixture did not change")
			}
			call := tampered.Calls[statusIndex]
			call.Response.BodyBase64 =
				base64.StdEncoding.EncodeToString(body)
			call.Response.BodyDigest = p14Digest(body)
			tampered.Calls[statusIndex] = call
			assertP14CodexSessionHistoryRejected(
				t,
				syntheticSource,
				syntheticP14CodexSessionJSONL(
					t,
					packet,
					tampered,
				),
				syntheticIdentity,
				packet,
				tampered,
				capturedAt.Add(time.Minute),
				testCase.Name,
			)
		})
	}
	basisAfterIndex := slices.IndexFunc(
		input.Calls,
		func(call p14CodexMCPCallEvidence) bool {
			return call.ExchangeRole ==
				p14CodexMCPExchangeBasisAfter
		},
	)
	if basisAfterIndex < 0 {
		t.Fatal("P14 Codex session fixture omits no-write basis guard")
	}
	storeReadTampered := input
	storeReadTampered.Calls = slices.Clone(input.Calls)
	basisCall := storeReadTampered.Calls[basisAfterIndex]
	basisBody, err := p14CodexMCPResponseBody(basisCall)
	if err != nil {
		t.Fatal(err)
	}
	basisBody = append(slices.Clone(basisBody), '\n')
	basisCall.Response.BodyBase64 =
		base64.StdEncoding.EncodeToString(basisBody)
	basisCall.Response.BodyDigest = p14Digest(basisBody)
	storeReadTampered.Calls[basisAfterIndex] = basisCall
	assertP14CodexSessionHistoryRejected(
		t,
		syntheticSource,
		syntheticP14CodexSessionJSONL(
			t,
			packet,
			storeReadTampered,
		),
		syntheticIdentity,
		packet,
		storeReadTampered,
		capturedAt.Add(time.Minute),
		"changed semantic store-read projection",
	)
	bodyTampered := input
	bodyTampered.Calls = slices.Clone(input.Calls)
	call := bodyTampered.Calls[0]
	call.Response.BodyBase64 = base64.StdEncoding.EncodeToString(
		[]byte(`{"forged":true}`),
	)
	call.Response.BodyDigest = p14Digest([]byte(`{"forged":true}`))
	bodyTampered.Calls[0] = call
	if _, err := deriveP14CodexSessionHistoryEvidence(
		syntheticSource,
		raw,
		syntheticIdentity,
		packet,
		bodyTampered,
		capturedAt.Add(time.Minute),
	); err == nil {
		t.Fatal("P14 Codex session history accepted forged response body")
	}
	extra := syntheticP14CodexSessionMCPLine(
		t,
		input.Calls[0],
		"call-p14-extra",
		capturedAt.Add(-time.Second),
	)
	extraRaw := append(slices.Clone(raw), extra...)
	extraRaw = append(extraRaw, '\n')
	if _, err := deriveP14CodexSessionHistoryEvidence(
		syntheticSource,
		extraRaw,
		syntheticIdentity,
		packet,
		input,
		capturedAt.Add(time.Minute),
	); err == nil {
		t.Fatal("P14 Codex session history accepted an extra Haft call")
	}
	first := input.Calls[0]
	beginLine := syntheticP14CodexSessionMCPBeginLine(t, first)
	endLine := syntheticP14CodexSessionMCPLine(
		t,
		first,
		first.Transcript.ToolCallID,
		mustP14Time(t, first.Response.CapturedAt),
	)
	missingBegin := bytes.Replace(
		raw,
		append(slices.Clone(beginLine), '\n'),
		nil,
		1,
	)
	assertP14CodexSessionHistoryRejected(
		t,
		syntheticSource,
		missingBegin,
		syntheticIdentity,
		packet,
		input,
		capturedAt.Add(time.Minute),
		"missing begin",
	)
	missingEnd := bytes.Replace(
		raw,
		append(slices.Clone(endLine), '\n'),
		nil,
		1,
	)
	assertP14CodexSessionHistoryRejected(
		t,
		syntheticSource,
		missingEnd,
		syntheticIdentity,
		packet,
		input,
		capturedAt.Add(time.Minute),
		"missing end",
	)
	duplicateBegin := append(slices.Clone(raw), beginLine...)
	duplicateBegin = append(duplicateBegin, '\n')
	assertP14CodexSessionHistoryRejected(
		t,
		syntheticSource,
		duplicateBegin,
		syntheticIdentity,
		packet,
		input,
		capturedAt.Add(time.Minute),
		"duplicate begin",
	)
	duplicateEnd := append(slices.Clone(raw), endLine...)
	duplicateEnd = append(duplicateEnd, '\n')
	assertP14CodexSessionHistoryRejected(
		t,
		syntheticSource,
		duplicateEnd,
		syntheticIdentity,
		packet,
		input,
		capturedAt.Add(time.Minute),
		"duplicate end",
	)
	unmatched := first
	unmatched.Transcript.ToolCallID = "call-p14-unmatched"
	unmatchedBegin := syntheticP14CodexSessionMCPBeginLine(t, unmatched)
	unmatchedRaw := append(slices.Clone(raw), unmatchedBegin...)
	unmatchedRaw = append(unmatchedRaw, '\n')
	assertP14CodexSessionHistoryRejected(
		t,
		syntheticSource,
		unmatchedRaw,
		syntheticIdentity,
		packet,
		input,
		capturedAt.Add(time.Minute),
		"unmatched begin",
	)
	ambiguous := first
	ambiguous.Transcript.ArgsCanonical = `{"action":"forged"}`
	ambiguousBegin := syntheticP14CodexSessionMCPBeginLine(t, ambiguous)
	ambiguousRaw := bytes.Replace(
		raw,
		beginLine,
		ambiguousBegin,
		1,
	)
	assertP14CodexSessionHistoryRejected(
		t,
		syntheticSource,
		ambiguousRaw,
		syntheticIdentity,
		packet,
		input,
		capturedAt.Add(time.Minute),
		"ambiguous begin arguments",
	)
	interruptedEnd := syntheticP14CodexSessionInterruptedMCPLine(
		t,
		first,
	)
	interruptedRaw := bytes.Replace(
		raw,
		endLine,
		interruptedEnd,
		1,
	)
	assertP14CodexSessionHistoryRejected(
		t,
		syntheticSource,
		interruptedRaw,
		syntheticIdentity,
		packet,
		input,
		capturedAt.Add(time.Minute),
		"interrupted end",
	)
	assertP14CodexSessionHistoryRejected(
		t,
		filepath.Join(t.TempDir(), filepath.Base(syntheticSource)),
		raw,
		syntheticIdentity,
		packet,
		input,
		capturedAt.Add(time.Minute),
		"session-root escape",
	)
}

func assertP14CodexSessionHistoryRejected(
	t *testing.T,
	sourcePath string,
	raw []byte,
	sourceIdentity p14CodexSessionFileIdentity,
	packet p14CodexMCPRequestCarrier,
	input p14CodexMCPCaptureInput,
	observedAt time.Time,
	name string,
) {
	t.Helper()
	if _, err := deriveP14CodexSessionHistoryEvidence(
		sourcePath,
		raw,
		sourceIdentity,
		packet,
		input,
		observedAt,
	); err == nil {
		t.Fatalf("P14 Codex session history accepted %s", name)
	}
}

func syntheticP14CodexSessionJSONL(
	t *testing.T,
	packet p14CodexMCPRequestCarrier,
	input p14CodexMCPCaptureInput,
) []byte {
	t.Helper()
	lines := make([][]byte, 0, len(input.Calls)*3+1)
	lines = append(lines, mustP14JSONLine(t, map[string]any{
		"timestamp": packet.Packet.GeneratedAt,
		"type":      "session_meta",
		"payload": map[string]any{
			"id": packet.Packet.Runtime.ThreadID,
		},
	}))
	for index := 0; index < len(input.Calls); {
		call := input.Calls[index]
		if call.ExchangeRole == p14CodexMCPExchangeTarget &&
			call.ParallelGroup != "" {
			end := index + 1
			for end < len(input.Calls) &&
				input.Calls[end].ExchangeID == call.ExchangeID &&
				input.Calls[end].ExchangeRole ==
					p14CodexMCPExchangeTarget {
				end++
			}
			group := input.Calls[index:end]
			lines = append(
				lines,
				syntheticP14CodexSessionUserLine(
					t,
					group[0],
					"P14 scripted concurrent capture "+
						group[0].ExchangeID,
				),
			)
			for _, member := range group {
				lines = append(
					lines,
					syntheticP14CodexSessionMCPBeginLine(
						t,
						member,
					),
				)
			}
			for _, member := range group {
				lines = append(
					lines,
					syntheticP14CodexSessionMCPLine(
						t,
						member,
						member.Transcript.ToolCallID,
						mustP14Time(
							t,
							member.Response.CapturedAt,
						),
					),
				)
			}
			index = end
			continue
		}
		promptText := "P14 scripted capture " + call.CaseID
		if call.AgentPrompt != nil {
			promptText = call.AgentPrompt.TextCanonical
		}
		lines = append(
			lines,
			syntheticP14CodexSessionUserLine(
				t,
				call,
				promptText,
			),
			syntheticP14CodexSessionMCPBeginLine(t, call),
			syntheticP14CodexSessionMCPLine(
				t,
				call,
				call.Transcript.ToolCallID,
				mustP14Time(t, call.Response.CapturedAt),
			),
		)
		index++
	}
	return append(bytes.Join(lines, []byte{'\n'}), '\n')
}

func syntheticP14CodexSessionUserLine(
	t *testing.T,
	call p14CodexMCPCallEvidence,
	text string,
) []byte {
	t.Helper()
	endAt := mustP14Time(t, call.Response.CapturedAt)
	duration := time.Duration(
		call.Transcript.DurationMilliseconds,
	) * time.Millisecond
	if duration <= 0 {
		duration = time.Nanosecond
	}
	return mustP14JSONLine(t, map[string]any{
		"timestamp": endAt.
			Add(-duration).
			Add(-time.Nanosecond).
			UTC().
			Format(time.RFC3339Nano),
		"type": "response_item",
		"payload": map[string]any{
			"type": "message",
			"role": "user",
			"content": []any{
				map[string]any{
					"type": "input_text",
					"text": text,
				},
			},
			"internal_chat_message_metadata_passthrough": map[string]any{
				"turn_id": call.Transcript.TurnID,
			},
		},
	})
}

func syntheticP14CodexSessionMCPBeginLine(
	t *testing.T,
	call p14CodexMCPCallEvidence,
) []byte {
	t.Helper()
	endAt := mustP14Time(t, call.Response.CapturedAt)
	duration := time.Duration(
		call.Transcript.DurationMilliseconds,
	) * time.Millisecond
	if duration <= 0 {
		duration = time.Nanosecond
	}
	return mustP14JSONLine(t, map[string]any{
		"timestamp": endAt.Add(-duration).UTC().Format(time.RFC3339Nano),
		"type":      "response_item",
		"payload": map[string]any{
			"type":      "function_call",
			"call_id":   call.Transcript.ToolCallID,
			"namespace": "mcp__haft",
			"name":      call.Transcript.Tool,
			"arguments": call.Transcript.ArgsCanonical,
			"internal_chat_message_metadata_passthrough": map[string]any{
				"turn_id": call.Transcript.TurnID,
			},
		},
	})
}

func syntheticP14CodexSessionMCPLine(
	t *testing.T,
	call p14CodexMCPCallEvidence,
	callID string,
	occurredAt time.Time,
) []byte {
	t.Helper()
	args := map[string]any{}
	if err := json.Unmarshal(
		[]byte(call.Transcript.ArgsCanonical),
		&args,
	); err != nil {
		t.Fatal(err)
	}
	body, err := base64.StdEncoding.DecodeString(
		call.Response.BodyBase64,
	)
	if err != nil {
		t.Fatal(err)
	}
	result := map[string]any{
		"Ok": map[string]any{
			"content": []any{
				map[string]any{
					"type": "text",
					"text": string(body),
				},
			},
			"isError": call.Response.IsError,
		},
	}
	return mustP14JSONLine(t, map[string]any{
		"timestamp": occurredAt.UTC().Format(time.RFC3339Nano),
		"type":      "event_msg",
		"payload": map[string]any{
			"type":    "mcp_tool_call_end",
			"call_id": callID,
			"invocation": map[string]any{
				"server":    "haft",
				"tool":      call.Transcript.Tool,
				"arguments": args,
			},
			"duration": map[string]any{
				"secs":  0,
				"nanos": call.Transcript.DurationMilliseconds * 1_000_000,
			},
			"result": result,
		},
	})
}

func syntheticP14CodexSessionInterruptedMCPLine(
	t *testing.T,
	call p14CodexMCPCallEvidence,
) []byte {
	t.Helper()
	args := map[string]any{}
	if err := json.Unmarshal(
		[]byte(call.Transcript.ArgsCanonical),
		&args,
	); err != nil {
		t.Fatal(err)
	}
	return mustP14JSONLine(t, map[string]any{
		"timestamp": call.Response.CapturedAt,
		"type":      "event_msg",
		"payload": map[string]any{
			"type":    "mcp_tool_call_end",
			"call_id": call.Transcript.ToolCallID,
			"invocation": map[string]any{
				"server":    "haft",
				"tool":      call.Transcript.Tool,
				"arguments": args,
			},
			"duration": map[string]any{
				"secs": 0,
				"nanos": call.Transcript.DurationMilliseconds *
					1_000_000,
			},
			"result": map[string]any{
				"Err": "interrupted by actual host",
			},
		},
	})
}

func mustP14JSONLine(
	t *testing.T,
	value any,
) []byte {
	t.Helper()
	raw, err := marshalP14CanonicalJSON(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func mustP14Time(
	t *testing.T,
	value string,
) time.Time {
	t.Helper()
	result, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		t.Fatal(err)
	}
	return result
}
