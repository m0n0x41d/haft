package tui

import (
	"strings"

	"github.com/m0n0x41d/quint-code/internal/agent"
)

// ---------------------------------------------------------------------------
// L2: Chat Items — domain rendering units with highlight support.
//
// Each item holds pre-rendered content (built by model from viewMessage data).
// Highlight is applied as a post-processing step via L1.ApplyHighlight.
//
// Depends on: L1 (selection geometry)
// ---------------------------------------------------------------------------

// ChatItem is one renderable, independently selectable unit in the chat list.
type ChatItem interface {
	// Render returns the rendered string. If a highlight is active,
	// it applies reverse-video via L1.ApplyHighlight.
	Render(width int) string

	// Height returns the number of display lines.
	Height() int

	// Selectable reports whether this item participates in text selection.
	Selectable() bool

	// SetHighlight sets the highlight range within this item.
	// Pass (-1,-1,-1,-1) to clear.
	SetHighlight(startLine, startCol, endLine, endCol int)

	// Highlight returns the current highlight range.
	Highlight() (startLine, startCol, endLine, endCol int)

	// PlainText returns ANSI-stripped text from the highlighted region
	// for clipboard extraction. Returns "" if no highlight.
	PlainText(width int) string
}

// ---------------------------------------------------------------------------
// Highlight state (embedded by concrete items)
// ---------------------------------------------------------------------------

type highlightState struct {
	hlStartLine, hlStartCol int
	hlEndLine, hlEndCol     int
}

func (h *highlightState) SetHighlight(sl, sc, el, ec int) {
	h.hlStartLine = sl
	h.hlStartCol = sc
	h.hlEndLine = el
	h.hlEndCol = ec
}

func (h *highlightState) Highlight() (int, int, int, int) {
	return h.hlStartLine, h.hlStartCol, h.hlEndLine, h.hlEndCol
}

func (h *highlightState) hasHighlight() bool {
	// -1 means "to end of item" — still a valid highlight
	if h.hlStartLine < 0 && h.hlEndLine < 0 {
		return false // both unset = no highlight
	}
	startLine := h.hlStartLine
	if startLine < 0 {
		startLine = 0
	}
	endLine := h.hlEndLine
	if endLine < 0 {
		return true // -1 end = "to end of item" = always has content
	}
	return startLine != endLine || h.hlStartCol != h.hlEndCol
}

// resolvedRange converts -1 sentinels to actual dimensions using item height.
func (h *highlightState) resolvedRange(height, width int) SelectionRange {
	sl, sc := h.hlStartLine, h.hlStartCol
	el, ec := h.hlEndLine, h.hlEndCol
	if sl < 0 {
		sl = 0
	}
	if sc < 0 {
		sc = 0
	}
	if el < 0 {
		el = height - 1
	}
	if ec < 0 {
		ec = width
	}
	return NormalizeRange(Coord{Line: sl, Col: sc}, Coord{Line: el, Col: ec})
}

func noHighlight() highlightState {
	return highlightState{-1, -1, -1, -1}
}

// ---------------------------------------------------------------------------
// Base item: pre-rendered content + highlight
// ---------------------------------------------------------------------------

type baseItem struct {
	rendered string
	height   int
	highlightState
}

func newBaseItem(rendered string) baseItem {
	h := 1
	if rendered != "" {
		h = strings.Count(rendered, "\n") + 1
	}
	return baseItem{
		rendered:       rendered,
		height:         h,
		highlightState: noHighlight(),
	}
}

func (b *baseItem) Render(width int) string {
	if !b.hasHighlight() {
		return b.rendered
	}
	return ApplyHighlight(b.rendered, width, b.resolvedRange(b.height, width))
}

func (b *baseItem) Height() int { return b.height }

func (b *baseItem) PlainText(width int) string {
	if !b.hasHighlight() {
		return ""
	}
	return ExtractText(b.rendered, width, b.resolvedRange(b.height, width))
}

// ---------------------------------------------------------------------------
// Concrete item types
// ---------------------------------------------------------------------------

// userItem — user message with border decoration.
type userItem struct{ baseItem }

func (*userItem) Selectable() bool { return true }

// assistantTextItem — assistant text body (thinking + markdown/plain text).
type assistantTextItem struct{ baseItem }

func (*assistantTextItem) Selectable() bool { return true }

// toolItem — a single tool call (header + output).
type toolItem struct{ baseItem }

func (*toolItem) Selectable() bool { return true }

// streamingItem — live streaming text (rebuilt every spinner tick).
type streamingItem struct{ baseItem }

func (*streamingItem) Selectable() bool { return true }

// errorItem — error message display.
type errorItem struct{ baseItem }

func (*errorItem) Selectable() bool { return true }

// dividerItem — visual separator between messages. Not selectable.
type dividerItem struct{ baseItem }

func (*dividerItem) Selectable() bool                { return false }
func (*dividerItem) SetHighlight(_, _, _, _ int)     {}
func (*dividerItem) Highlight() (int, int, int, int) { return -1, -1, -1, -1 }
func (*dividerItem) PlainText(_ int) string          { return "" }

// permissionItem — permission prompt. Not selectable (interactive element).
type permissionItem struct{ baseItem }

func (*permissionItem) Selectable() bool                { return false }
func (*permissionItem) SetHighlight(_, _, _, _ int)     {}
func (*permissionItem) Highlight() (int, int, int, int) { return -1, -1, -1, -1 }
func (*permissionItem) PlainText(_ int) string          { return "" }

// ---------------------------------------------------------------------------
// Builder: viewMessage slice → ChatItem slice
// ---------------------------------------------------------------------------

// buildChatItems converts the model's viewMessages (+ transient state)
// into a flat list of ChatItems for the ChatList.
//
// Called from refreshChat() — rebuilds on every content change.
func (m Model) buildChatItems() []ChatItem {
	fullWidth := max(20, m.width-2)
	bodyWidth := max(20, fullWidth-4)

	var items []ChatItem

	for i, msg := range m.messages {
		// Divider between messages
		if i > 0 {
			items = append(items, &dividerItem{
				baseItem: newBaseItem(m.renderSectionDivider(fullWidth)),
			})
		}

		switch msg.Role {
		case agent.RoleUser:
			items = append(items, &userItem{
				baseItem: newBaseItem(m.renderUserMessage(msg, fullWidth)),
			})

		case agent.RoleAssistant:
			items = append(items, m.buildAssistantItems(msg, bodyWidth)...)
		}
	}

	// Streaming content (transient)
	if m.state == stateStreaming || m.state == statePermission {
		items = append(items, m.buildStreamingItems(bodyWidth)...)
	}

	// Error (transient)
	if m.errMsg != "" {
		errBlock := m.styles.ErrorText.Render(" Error: " + truncate(m.errMsg, bodyWidth))
		items = append(items, &errorItem{baseItem: newBaseItem(errBlock)})
	}

	// Permission (transient, not selectable)
	if m.state == statePermission {
		items = append(items, &permissionItem{
			baseItem: newBaseItem(m.renderPermission(bodyWidth)),
		})
	}

	return items
}

// buildAssistantItems splits an assistant viewMessage into separate items:
// one for text (if present) and one per tool call.
func (m Model) buildAssistantItems(msg viewMessage, w int) []ChatItem {
	var items []ChatItem

	// Text block: thinking + body
	var textParts []string
	if msg.Thinking != "" {
		textParts = append(textParts, m.renderThinkingBox(msg.Thinking, w))
	}
	if msg.Text != "" {
		textParts = append(textParts, renderBodyText(msg.Text, w, m.styles.AssistantText))
	}

	label := ""
	if msg.Phase != "" {
		label = m.findPhaseName(msg.Phase)
	}

	textBody := strings.Join(textParts, "\n\n")
	if textBody != "" || len(msg.Tools) == 0 {
		rendered := m.renderAssistantBlock(label, textBody)
		items = append(items, &assistantTextItem{baseItem: newBaseItem(rendered)})
	}

	// Each tool as a separate item
	for _, tool := range msg.Tools {
		rendered := m.renderTool(tool, w)
		items = append(items, &toolItem{baseItem: newBaseItem(rendered)})
	}

	return items
}

// buildStreamingItems creates items for the currently streaming content.
func (m Model) buildStreamingItems(w int) []ChatItem {
	var items []ChatItem

	thinking := m.thinkBuf.String()
	if thinking != "" {
		thinkBox := m.renderThinkingBox(thinking, w)
		rendered := m.renderAssistantBlock("", thinkBox)
		items = append(items, &streamingItem{baseItem: newBaseItem(rendered)})
	}

	s := m.streamBuf.String()
	if s != "" {
		body := renderBodyText(s, w, m.styles.AssistantText)
		rendered := m.renderAssistantBlock("", body)
		items = append(items, &streamingItem{baseItem: newBaseItem(rendered)})
	}

	return items
}
