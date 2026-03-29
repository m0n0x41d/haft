package tui

import (
	"context"
	"strings"
	"testing"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/m0n0x41d/quint-code/internal/agent"
)

func TestMouseWheelScrollsChatViewport(t *testing.T) {
	runFn := func(context.Context, *agent.Session, string) {}
	session := &agent.Session{ID: "ses-test", Model: "gpt-5.4"}
	model := New(session, runFn, NewBus(1), "", nil, nil)

	model.width = 80
	model.height = 12
	model.resizeComponents()
	model.chat.SetContent(strings.Repeat("line\n", 80))

	before := model.chat.YOffset()
	msg := tea.MouseWheelMsg(tea.Mouse{Button: tea.MouseWheelDown})

	updated, _ := model.Update(msg)
	next := updated.(Model)

	if next.chat.YOffset() <= before {
		t.Fatalf("expected mouse wheel to advance viewport offset, before=%d after=%d", before, next.chat.YOffset())
	}
}

func TestBottomAlignChatPadsShortContent(t *testing.T) {
	model := Model{chatReady: true}
	model.chat = viewport.New(viewport.WithWidth(80), viewport.WithHeight(5))

	got := model.bottomAlignChat("line 1\nline 2")

	want := "\n\n\nline 1\nline 2"
	if got != want {
		t.Fatalf("expected bottom aligned content %q, got %q", want, got)
	}
}

func TestBottomAlignChatLeavesTallContentUntouched(t *testing.T) {
	model := Model{chatReady: true}
	model.chat = viewport.New(viewport.WithWidth(80), viewport.WithHeight(3))

	content := "line 1\nline 2\nline 3\nline 4"
	got := model.bottomAlignChat(content)

	if got != content {
		t.Fatalf("expected tall content to remain unchanged, got %q", got)
	}
}
