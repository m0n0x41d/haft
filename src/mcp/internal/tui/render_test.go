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

	// Use buildAssistantItems which replaced renderAssistantMessage
	msg := viewMessage{
		Role: agent.RoleAssistant,
		Text: "Hello",
	}
	items := model.buildAssistantItems(msg, 60)
	if len(items) == 0 {
		t.Fatal("expected at least one item from buildAssistantItems")
	}

	got := items[0].Render(60)
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

	// Use buildChatItems which replaced renderAllMessages
	items := model.buildChatItems()
	var got string
	for _, item := range items {
		got += item.Render(80) + "\n"
	}

	if strings.Contains(got, "█") {
		t.Fatalf("expected streaming content without block cursor, got %q", got)
	}
}

func TestWrapPlainLineMixedLanguageOrderStable(t *testing.T) {
	line := "Ладно, ещё один примерно на 1k symbols. Иногда тест интерфейса лучше делать не на hello world, а на тексте ближе к реальному хаосу."
	got := wrapPlainLine(line, 24)
	normalized := strings.Join(strings.Fields(got), " ")
	if normalized != strings.Join(strings.Fields(line), " ") {
		t.Fatalf("expected wrapped text to preserve token order\nwant: %q\n got: %q", line, got)
	}
}

func TestRenderBodyTextMixedLanguageNoReordering(t *testing.T) {
	text := "Ладно, ещё один примерно на 1k symbols. Иногда тест интерфейса лучше делать не на hello world, а на тексте ближе к реальному хаосу."
	got := renderBodyText(text, 24, DefaultStyles().AssistantText)
	normalized := strings.Join(strings.Fields(got), " ")
	if normalized != strings.Join(strings.Fields(text), " ") {
		t.Fatalf("expected assistant body text to preserve token order\nwant: %q\n got: %q", text, got)
	}
}
