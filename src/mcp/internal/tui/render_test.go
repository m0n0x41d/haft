package tui

import (
	"strings"
	"testing"

	"github.com/m0n0x41d/quint-code/internal/agent"
)

func TestRenderBodyTextKeepsPlainTextPlain(t *testing.T) {
	got := renderBodyText("Hello", 40, DefaultStyles().AssistantText)

	if strings.Contains(got, "• ") || strings.Contains(got, ". Hello") {
		t.Fatalf("expected plain text rendering without markdown list/enumeration markers, got %q", got)
	}
}

func TestRenderAssistantMessageUsesHexagonMarker(t *testing.T) {
	model := Model{styles: DefaultStyles()}
	msg := viewMessage{
		Role: agent.RoleAssistant,
		Text: "Hello",
	}

	got := model.renderAssistantMessage(msg, 60)

	if !strings.Contains(got, "⏣") {
		t.Fatalf("expected assistant rendering to include hexagon marker, got %q", got)
	}
}

func TestRenderUserMessageUsesFullWidthAccentBlock(t *testing.T) {
	model := Model{styles: DefaultStyles()}
	msg := viewMessage{
		Role: agent.RoleUser,
		Text: "hello",
	}

	got := model.renderUserMessage(msg, 24)

	if strings.Contains(got, "You") {
		t.Fatalf("expected user label to be removed, got %q", got)
	}
	if !strings.Contains(got, strings.Repeat("━", 24)) {
		t.Fatalf("expected full-width accent borders, got %q", got)
	}
}

func TestRenderAllMessagesStreamingDoesNotShowBlockCursor(t *testing.T) {
	model := Model{
		state:     stateStreaming,
		styles:    DefaultStyles(),
		width:     80,
		streamBuf: &strings.Builder{},
		thinkBuf:  &strings.Builder{},
	}
	model.streamBuf.WriteString("hello")

	got := model.renderAllMessages()

	if strings.Contains(got, "█") {
		t.Fatalf("expected streaming content without block cursor, got %q", got)
	}
}
