package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"

	"github.com/m0n0x41d/quint-code/internal/agent"
)

func TestMouseWheelScrollsChatList(t *testing.T) {
	runFn := func(context.Context, *agent.Session, string) {}
	session := &agent.Session{ID: "ses-test", Model: "gpt-5.4"}
	model := New(session, runFn, NewBus(1), "", nil, nil, nil, nil)

	model.width = 80
	model.height = 12
	model.resizeComponents()

	// Add enough content to make scrolling possible
	items := make([]ChatItem, 20)
	for i := range items {
		items[i] = newTestItem("line content "+strings.Repeat("x", i), true)
	}
	model.chatList.SetItems(items)
	model.chatList.setYOffset(0) // start at top

	before := model.chatList.YOffset()
	msg := tea.MouseWheelMsg(tea.Mouse{Button: tea.MouseWheelDown})

	updated, _ := model.Update(msg)
	next := updated.(Model)

	if next.chatList.YOffset() <= before {
		t.Fatalf("expected mouse wheel to advance offset, before=%d after=%d",
			before, next.chatList.YOffset())
	}
}

func TestChatListBottomAlignsShortContent(t *testing.T) {
	cl := NewChatList(80, 5)
	cl.SetItems([]ChatItem{
		newTestItem("line 1", true),
		newTestItem("line 2", true),
	})
	// Total = 3 lines (2 items + 1 gap), viewport = 5
	view := cl.View()
	lines := strings.Split(view, "\n")
	if len(lines) != 5 {
		t.Fatalf("expected 5 lines, got %d", len(lines))
	}
	// First 2 lines should be empty (padding)
	if strings.TrimSpace(lines[0]) != "" || strings.TrimSpace(lines[1]) != "" {
		t.Fatal("short content should be bottom-aligned (top lines empty)")
	}
	// Content should be at the bottom
	if !strings.Contains(lines[2], "line 1") {
		t.Errorf("expected 'line 1' at line 2, got %q", lines[2])
	}
}

func TestChatListTallContentNotPadded(t *testing.T) {
	cl := NewChatList(80, 3)
	cl.SetItems([]ChatItem{
		newTestItem("a", true),
		newTestItem("b", true),
		newTestItem("c", true),
		newTestItem("d", true),
	})
	// Total = 7 lines (4 items + 3 gaps), viewport = 3
	// Should show last 3 lines (at bottom due to follow mode)
	view := cl.View()
	lines := strings.Split(view, "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 visible lines, got %d", len(lines))
	}
}

func TestViewUsesUVCompositionForMixedText(t *testing.T) {
	runFn := func(context.Context, *agent.Session, string) {}
	session := &agent.Session{ID: "ses-test", Model: "gpt-5.4"}
	model := New(session, runFn, NewBus(1), "", nil, nil, nil, nil)
	model.width = 80
	model.height = 18
	model.resizeComponents()
	model.messages = []viewMessage{{
		Role: agent.RoleAssistant,
		Text: "Ладно, вот нормальный mixed-text sample примерно на 1k symbols для проверки рендера в TUI.",
	}}
	model.refreshChat()

	view := model.View()
	normalized := strings.Join(strings.Fields(ansi.Strip(view.Content)), " ")
	want := strings.Join(strings.Fields("Ладно, вот нормальный mixed-text sample примерно на 1k symbols для проверки рендера в TUI."), " ")
	if !strings.Contains(normalized, want) {
		t.Fatalf("expected UV-composed view to preserve mixed text order\nwant substring: %q\n got: %q", want, normalized)
	}
}

func TestLayoutBlocksReserveSeparatorRows(t *testing.T) {
	runFn := func(context.Context, *agent.Session, string) {}
	session := &agent.Session{ID: "ses-test", Model: "gpt-5.4"}
	model := New(session, runFn, NewBus(1), "", nil, nil, nil, nil)
	model.width = 80
	model.height = 10
	model.resizeComponents()

	inputBlock, statusBlock, chatH := model.layoutBlocks()
	total := chatH + lipgloss.Height(inputBlock) + lipgloss.Height(statusBlock) + 2
	if total != model.height {
		t.Fatalf("expected layout to reserve two separator rows, total=%d height=%d", total, model.height)
	}
}

func TestDrawBlockWrapsLongMixedText(t *testing.T) {
	canvas := uv.NewScreenBuffer(24, 6)
	canvas.Method = ansi.GraphemeWidth
	text := "Сейчас как раз нужен не красивый text, а честный stress sample для TUI"
	drawBlock(&canvas, 0, 0, 24, 6, text)
	got := ansi.Strip(canvas.Render())
	normalized := strings.Join(strings.Fields(got), " ")
	want := strings.Join(strings.Fields(text), " ")
	if !strings.Contains(normalized, want) {
		t.Fatalf("expected wrapped UV block to preserve mixed text order\nwant substring: %q\n got: %q", want, normalized)
	}
}

func TestStreamDoneFinalizesFromAuthoritativeMessage(t *testing.T) {
	runFn := func(context.Context, *agent.Session, string) {}
	session := &agent.Session{ID: "ses-test", Model: "gpt-5.4"}
	model := New(session, runFn, NewBus(1), "", nil, nil, nil, nil)
	model.currentPhase = agent.PhaseWorker
	model.streamBuf.WriteString("broken live preview text")
	model.thinkBuf.WriteString("thinking")

	final := agent.Message{
		Role:  agent.RoleAssistant,
		Parts: []agent.Part{agent.TextPart{Text: "authoritative final text"}},
	}
	updated, _ := model.Update(StreamDoneMsg{Message: final})
	next := updated.(Model)
	if len(next.messages) == 0 {
		t.Fatal("expected finalized assistant message")
	}
	got := next.messages[len(next.messages)-1]
	if got.Text != "authoritative final text" {
		t.Fatalf("expected final text from StreamDone message, got %q", got.Text)
	}
}

func TestStreamingDeltaUpdatesCanonicalAssistantMessage(t *testing.T) {
	runFn := func(context.Context, *agent.Session, string) {}
	session := &agent.Session{ID: "ses-test", Model: "gpt-5.4"}
	model := New(session, runFn, NewBus(1), "", nil, nil, nil, nil)
	model.currentPhase = agent.PhaseWorker

	updated, _ := model.Update(StreamDeltaMsg{Text: "hello "})
	next := updated.(Model)
	updated, _ = next.Update(StreamDeltaMsg{Text: "world"})
	next = updated.(Model)

	if len(next.messages) == 0 {
		t.Fatal("expected assistant message during streaming")
	}
	got := next.messages[len(next.messages)-1]
	if got.Text != "hello world" {
		t.Fatalf("expected canonical assistant text to accumulate deltas, got %q", got.Text)
	}
}
