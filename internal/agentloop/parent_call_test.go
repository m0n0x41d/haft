package agentloop

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/m0n0x41d/haft/internal/agent"
	"github.com/m0n0x41d/haft/internal/jsonrpc"
	"github.com/m0n0x41d/haft/internal/protocol"
	"github.com/m0n0x41d/haft/internal/provider"
	"github.com/m0n0x41d/haft/internal/tools"
)

func TestExecuteToolCallsParallelPropagatesActiveToolCallID(t *testing.T) {
	t.Parallel()

	var recorded sync.Map
	registry := tools.NewRegistry(t.TempDir())
	registry.Register(toolStub{
		name: "echo",
		execute: func(ctx context.Context, _ string) (agent.ToolResult, error) {
			recorded.Store(tools.ActiveToolCallID(ctx), true)
			return agent.PlainResult("ok"), nil
		},
	})

	coordinator := &Coordinator{
		Tools:    registry,
		Messages: &messageStoreStub{},
		Bus:      newProtocolBus(),
	}

	sess := &agent.Session{ID: "sess_1"}
	history := []agent.Message{}
	toolCalls := []agent.ToolCallPart{
		{ToolCallID: "call_1", ToolName: "echo", Arguments: `{"value":"a"}`},
		{ToolCallID: "call_2", ToolName: "echo", Arguments: `{"value":"b"}`},
	}

	results := coordinator.executeToolCallsParallel(context.Background(), sess, toolCalls, &history)

	if len(results) != len(toolCalls) {
		t.Fatalf("result count = %d, want %d", len(results), len(toolCalls))
	}

	for _, toolCall := range toolCalls {
		if _, ok := recorded.Load(toolCall.ToolCallID); !ok {
			t.Fatalf("tool call %q was not propagated into context", toolCall.ToolCallID)
		}
	}
}

func TestSpawnSubagentEmitsParentCallID(t *testing.T) {
	t.Parallel()

	writer := &lockedBuffer{}
	sessions := &sessionStoreStub{}
	coordinator := &Coordinator{
		Provider:  providerStub{},
		Tools:     tools.NewRegistry(t.TempDir()),
		Sessions:  sessions,
		Messages:  &messageStoreStub{},
		Bus:       newProtocolBusWithWriter(writer),
		Subagents: NewSubagentTracker(),
	}

	parentSession := &agent.Session{ID: "sess_parent", Model: "test-model"}
	subagentDef := agent.SubagentDef{Name: "explore", MaxSteps: 1}
	ctx := tools.WithActiveToolCallID(context.Background(), "call_parent")

	handle, err := coordinator.SpawnSubagent(ctx, parentSession, subagentDef, "inspect the repo")
	if err != nil {
		t.Fatalf("SpawnSubagent() error = %v", err)
	}

	result := <-handle.Result
	if result.Error != nil {
		t.Fatalf("subagent result error = %v", result.Error)
	}

	event := findSubagentStartEvent(t, writer.String())
	if event.ParentCallID != "call_parent" {
		t.Fatalf("parentCallId = %q, want %q", event.ParentCallID, "call_parent")
	}
	if event.SubagentID == "" {
		t.Fatal("subagentId should not be empty")
	}
	if sessions.createCalls != 1 {
		t.Fatalf("createCalls = %d, want 1", sessions.createCalls)
	}
}

func TestSpawnSubagentRequiresParentCallID(t *testing.T) {
	t.Parallel()

	writer := &lockedBuffer{}
	sessions := &sessionStoreStub{}
	coordinator := &Coordinator{
		Provider:  providerStub{},
		Tools:     tools.NewRegistry(t.TempDir()),
		Sessions:  sessions,
		Messages:  &messageStoreStub{},
		Bus:       newProtocolBusWithWriter(writer),
		Subagents: NewSubagentTracker(),
	}

	parentSession := &agent.Session{ID: "sess_parent", Model: "test-model"}
	subagentDef := agent.SubagentDef{Name: "explore", MaxSteps: 1}

	handle, err := coordinator.SpawnSubagent(context.Background(), parentSession, subagentDef, "inspect the repo")

	if err == nil {
		t.Fatal("SpawnSubagent() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "active parent tool call ID") {
		t.Fatalf("error = %q, want active parent tool call ID", err.Error())
	}
	if handle != nil {
		t.Fatal("handle should be nil when parent call ID is missing")
	}
	if sessions.createCalls != 0 {
		t.Fatalf("createCalls = %d, want 0", sessions.createCalls)
	}
	if coordinator.Subagents.ActiveCount() != 0 {
		t.Fatalf("active subagents = %d, want 0", coordinator.Subagents.ActiveCount())
	}
	if strings.TrimSpace(writer.String()) != "" {
		t.Fatalf("unexpected bus output: %q", writer.String())
	}
}

type toolStub struct {
	name    string
	execute func(ctx context.Context, argsJSON string) (agent.ToolResult, error)
}

func (t toolStub) Name() string { return t.name }

func (t toolStub) Schema() agent.ToolSchema { return agent.ToolSchema{Name: t.name} }

func (t toolStub) Execute(ctx context.Context, argsJSON string) (agent.ToolResult, error) {
	return t.execute(ctx, argsJSON)
}

type providerStub struct{}

func (providerStub) Stream(
	_ context.Context,
	_ []agent.Message,
	_ []agent.ToolSchema,
	_ func(provider.StreamDelta),
) (*agent.Message, error) {
	return &agent.Message{
		Role:  agent.RoleAssistant,
		Parts: []agent.Part{agent.TextPart{Text: "summary"}},
	}, nil
}

func (providerStub) ModelID() string { return "test-model" }

type sessionStoreStub struct {
	createCalls int
}

func (s *sessionStoreStub) Create(_ context.Context, _ *agent.Session) error {
	s.createCalls++
	return nil
}
func (*sessionStoreStub) Get(_ context.Context, _ string) (*agent.Session, error) {
	return nil, nil
}
func (*sessionStoreStub) Update(_ context.Context, _ *agent.Session) error { return nil }
func (*sessionStoreStub) ListRecent(_ context.Context, _ int) ([]agent.Session, error) {
	return nil, nil
}

type messageStoreStub struct {
	mu       sync.Mutex
	messages []agent.Message
}

func (s *messageStoreStub) Save(_ context.Context, msg *agent.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.messages = append(s.messages, *msg)
	return nil
}

func (s *messageStoreStub) UpdateMessage(_ context.Context, msg *agent.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for index := range s.messages {
		if s.messages[index].ID == msg.ID {
			s.messages[index] = *msg
			return nil
		}
	}

	s.messages = append(s.messages, *msg)
	return nil
}

func (s *messageStoreStub) ListBySession(_ context.Context, sessionID string) ([]agent.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var result []agent.Message
	for _, msg := range s.messages {
		if msg.SessionID == sessionID {
			result = append(result, msg)
		}
	}

	return result, nil
}

func (*messageStoreStub) LastUserMessage(_ context.Context, _ string) (string, error) { return "", nil }
func (*messageStoreStub) DeleteOlderThan(_ context.Context, _ string, _ int) (int, error) {
	return 0, nil
}

func newProtocolBus() *protocol.Bus {
	return newProtocolBusWithWriter(&lockedBuffer{})
}

func newProtocolBusWithWriter(writer io.Writer) *protocol.Bus {
	return protocol.NewBus(jsonrpc.NewServer(strings.NewReader(""), writer))
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}

func findSubagentStartEvent(t *testing.T, output string) protocol.SubagentStart {
	t.Helper()

	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		var message jsonrpc.Message
		if err := json.Unmarshal([]byte(line), &message); err != nil {
			t.Fatalf("unmarshal jsonrpc message: %v", err)
		}

		if message.Method != protocol.MethodSubagentStart {
			continue
		}

		var event protocol.SubagentStart
		if err := json.Unmarshal(message.Params, &event); err != nil {
			t.Fatalf("unmarshal subagent.start params: %v", err)
		}
		return event
	}

	t.Fatalf("subagent.start event not found in %q", output)
	return protocol.SubagentStart{}
}
