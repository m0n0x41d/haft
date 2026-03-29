package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/m0n0x41d/quint-code/internal/agent"
)

func TestMouseWheelScrollsChatList(t *testing.T) {
	runFn := func(context.Context, *agent.Session, string) {}
	session := &agent.Session{ID: "ses-test", Model: "gpt-5.4"}
	model := New(session, runFn, NewBus(1), "", nil, nil)

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
